package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M10b (design D1-D3, D5, D8): the program-level emitter. Written
// test-first — red today on the undefined symbols alone (EmitProgram,
// the ProgModule input, ModeTest, the two new boundary Whats). The
// fn-ABI matrix is the D2 formality: the design names the families,
// these tests freeze the exact IR shapes as the contract the bodies
// then satisfy. Slot symbols carry the module key verbatim (test module
// keys are path-derived with '.' for '/', so the whole name stays in
// the LLVM identifier set).

// --- local builders -------------------------------------------------------

func pubFn(name string, params []ast.Param, ret ast.TypeRef, body ...ast.Stmt) *ast.FnDecl {
	return &ast.FnDecl{Pub: true, Name: name, Params: params, Ret: ret, Body: ast.Block{Items: body}}
}

func named(name string) *ast.NamedType { return &ast.NamedType{Name: name} }

func i64Param(name string) ast.Param { return ast.Param{Name: name, Type: named("Int64")} }

func retValue(v ast.Expr) *ast.Return { return &ast.Return{HasValue: true, Value: v} }

func letBind(name string, x ast.Expr) *ast.Binding {
	return &ast.Binding{Kw: "let", Name: name, Init: x}
}

func letDiscard(x ast.Expr) *ast.Binding {
	return &ast.Binding{Kw: "let", Name: "_", Init: x}
}

func qualCall(qual, name string, args ...ast.Expr) *ast.Call {
	return &ast.Call{Fn: &ast.Member{Recv: ident(qual), Name: name}, Args: args}
}

func boolLit(text string) *ast.Literal { return &ast.Literal{Kind: "bool", Text: text} }

// progBuild wraps main-shaped statements into the root build module
// (the we new skeleton: AppError + main).
func progBuild(stmts ...ast.Stmt) ProgModule {
	return ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		mainDecl(stmts...),
	}}}
}

func wantIR(t *testing.T, ir, fragment, what string) {
	t.Helper()
	if !strings.Contains(ir, fragment) {
		t.Fatalf("%s: IR missing %q\n--\n%s", what, fragment, ir)
	}
}

func wantNoIR(t *testing.T, ir, fragment, what string) {
	t.Helper()
	if strings.Contains(ir, fragment) {
		t.Fatalf("%s: IR unexpectedly contains %q\n--\n%s", what, fragment, ir)
	}
}

// --- slots and the entry exception (D3) -----------------------------------

// Build mode: every We fn is defined under its qualified symbol with a
// slot global pointing at the real body, all call sites load the slot,
// and the root main alone is the direct entry (__we_main, no slot).
func TestM10bBuildSlotsAndEntry(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		pubFn("three", nil, named("Int64"), retValue(intLit("3"))),
	}}}
	root := progBuild(
		letDiscard(qualCall("util", "three")),
		okReturn(),
	)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{util, root})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define i64 @util.three()", "real body")
	wantIR(t, ir, "@slot.util.three = global ptr @util.three", "slot global")
	wantIR(t, ir, "load ptr, ptr @slot.util.three", "call via slot")
	wantNoIR(t, ir, "call i64 @util.three", "no direct call")
	wantIR(t, ir, "define i32 @__we_main()", "build entry")
	wantNoIR(t, ir, "@slot.main.main", "entry exception: no slot for root main")
}

// Test mode: main is an ordinary fn — slotted like any other, mockable
// through the slot; the synthesized driver owns __we_main.
func TestM10bMainDowngrade(t *testing.T) {
	withMain := ProgModule{Key: "helper", File: &ast.File{Items: []ast.Item{
		appError(),
		mainDecl(okReturn()),
	}}}
	tm := ProgModule{Key: "tests.m_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		&ast.Import{Path: []string{"helper"}},
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		&ast.TestDecl{Desc: "noop", Body: ast.Block{Items: []ast.Stmt{
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
	}}}
	ir, ni := EmitProgram(ModeTest, []ProgModule{withMain, tm})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@slot.helper.main = global ptr @helper.main", "downgraded main is slotted")
	wantIR(t, ir, "call void @__we_test_begin()", "driver owns the entry")

	bir, ni := EmitProgram(ModeBuild, []ProgModule{withMain, progBuild(okReturn())})
	if ni != nil {
		t.Fatalf("build boundary: %s", ni.What)
	}
	// The root main of the build is the entry — no slot (D3's exception);
	// the helper module's own main is an ordinary fn there too (slot, not
	// entry).
	wantNoIR(t, bir, "@slot.main.main", "build mode: the root main is the entry, unslotted")
	wantIR(t, bir, "@slot.helper.main = global ptr @helper.main", "helper main slotted in build too")
	wantIR(t, bir, "define i32 @__we_main()", "build entry")
}

// --- std entry slots and the channel exception (D3) -----------------------

// Imported std modules load their fn entries as slots at each entry's
// first reference: io println, test assertTrue, time now/sleep — the
// check face accepts mocks on these literals, so the run face must be
// able to intercept. The concurrent channel constructor is the disclosed
// exception: its runtime face is a primCtor with expectation-derived
// arguments, never a slottable fn call.
func TestM10bStdEntrySlots(t *testing.T) {
	tm := ProgModule{Key: "tests.t_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		stdIoImport(""),
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		&ast.Import{Path: []string{"std", "time"}},
		&ast.TestDecl{Desc: "entries", Body: ast.Block{Items: []ast.Stmt{
			letBind("a", qualCall("time", "now")),
			letDiscard(qualCall("time", "sleep", intLit("1"))),
			ioCall("io", "println", strLit(`"x"`)),
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
	}}}
	ir, ni := EmitProgram(ModeTest, []ProgModule{tm})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@slot.io.println = global ptr @__we_println", "io slot")
	wantIR(t, ir, "load ptr, ptr @slot.io.println", "io call via slot")
	wantIR(t, ir, "@slot.test.assertTrue = global ptr @__we_assert_true", "test slot")
	wantIR(t, ir, "@slot.time.now = global ptr @__we_time_now", "time now slot")
	wantIR(t, ir, "@slot.time.sleep = global ptr @__we_time_sleep", "time sleep slot")

	chanMod := ProgModule{Key: "tests.c_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		&ast.TestDecl{Desc: "chan", Body: ast.Block{Items: []ast.Stmt{
			&ast.Binding{Kw: "let", Name: "ch",
				Typ:  &ast.NamedType{Qual: "conc", Name: "Channel", Args: []ast.TypeRef{named("Int64")}},
				Init: qualCall("conc", "channel", intLit("1")),
			},
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
	}}}
	cir, ni := EmitProgram(ModeTest, []ProgModule{chanMod})
	if ni != nil {
		t.Fatalf("chan boundary: %s", ni.What)
	}
	wantNoIR(t, cir, "@slot.concurrent.channel", "channel constructor is not slotted")
	wantIR(t, cir, "@__we_prim_new_chan", "channel stays a primCtor call")
}

// --- the fn ABI matrix (D2, T2 formality) ---------------------------------

// One subtest per family of the D2 table; the define line and the call
// argument shape are the contract. Value records ride the same ptr ABI
// as gc records in this reference build (one heap representation), so
// the sret row of the design table stays unused — pinned here as ptr.
func TestM10bFnAbiMatrix(t *testing.T) {
	cases := []struct {
		name    string
		mod     ProgModule
		defines []string
		calls   []string
	}{{
		name: "i64",
		mod: ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
			pubFn("add", []ast.Param{i64Param("a"), i64Param("b")}, named("Int64"),
				retValue(binOp("+", ident("a"), ident("b")))),
			appError(),
			mainDecl(
				letBind("n", &ast.Call{Fn: ident("add"), Args: []ast.Expr{intLit("2"), intLit("3")}}),
				okReturn(),
			),
		}}},
		defines: []string{"define i64 @main.add(i64 %a, i64 %b)"},
		calls:   []string{"(i64 2, i64 3)"},
	}, {
		name: "float",
		mod: ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
			pubFn("half", []ast.Param{{Name: "x", Type: named("Float64")}}, named("Float64"),
				retValue(ident("x"))),
			appError(),
			mainDecl(
				letBind("h", &ast.Call{Fn: ident("half"), Args: []ast.Expr{&ast.Literal{Kind: "float", Text: "1.5"}}}),
				okReturn(),
			),
		}}},
		defines: []string{"define double @main.half(double %x)"},
		// The literal renders as its exact double bits (numImmediate's
		// hex form — the operand is bare, the call site spells the type).
		calls: []string{"(double 0x3FF8000000000000)"},
	}, {
		name: "string",
		mod: ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
			pubFn("ident", []ast.Param{{Name: "s", Type: named("String")}}, named("String"),
				retValue(ident("s"))),
			appError(),
			mainDecl(
				letBind("s", strLit(`"we"`)),
				letBind("t", &ast.Call{Fn: ident("ident"), Args: []ast.Expr{ident("s")}}),
				okReturn(),
			),
		}}},
		defines: []string{"define { ptr, i64 } @main.ident(ptr %s0, i64 %s1)"},
		calls:   []string{"call { ptr, i64 } %"},
	}, {
		name: "gc record",
		mod: ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
			&ast.RecordDecl{Cat: "gc", Name: "User", Fields: []ast.FieldDecl{{Name: "name", Typ: named("String")}}},
			pubFn("same", []ast.Param{{Name: "u", Type: named("User")}}, named("User"),
				retValue(ident("u"))),
			appError(),
			mainDecl(
				letBind("a", &ast.Call{Fn: ident("User"), Args: []ast.Expr{strLit(`"we"`)}}),
				letBind("b", &ast.Call{Fn: ident("same"), Args: []ast.Expr{ident("a")}}),
				okReturn(),
			),
		}}},
		defines: []string{"define ptr @main.same(ptr %u)"},
		calls:   []string{"call ptr %"},
	}, {
		name: "sum",
		mod: ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
			&ast.SumDecl{Name: "Opt", Variants: []ast.Variant{{Name: "A"}, {Name: "B"}}},
			pubFn("pick", []ast.Param{i64Param("i")}, named("Opt"),
				retValue(&ast.Call{Fn: ident("B")})),
			pubFn("use", []ast.Param{{Name: "r", Type: named("Opt")}}, named("Int64"),
				retValue(intLit("3"))),
			appError(),
			mainDecl(
				letBind("r", &ast.Call{Fn: ident("pick"), Args: []ast.Expr{intLit("1")}}),
				letDiscard(&ast.Call{Fn: ident("use"), Args: []ast.Expr{ident("r")}}),
				okReturn(),
			),
		}}},
		defines: []string{
			// A valueless variant return widens with the family, and the
			// variant's own index is the tag — the whole define is pinned,
			// so a two-word regression anywhere in it shows.
			"define { i64, i64, i64 } @main.pick(i64 %i) {\nentry:\n  ret { i64, i64, i64 } { i64 1, i64 0, i64 0 }\n}",
			"define i64 @main.use(i64 %r0, i64 %r1, i64 %r2)",
		},
		calls: []string{
			"call { i64, i64, i64 } %",
			// All three words cross back out of the aggregate, through the
			// binding's slots, and into the callee's three parameters. The
			// loads are pinned by their slot so a consumer reading the wrong
			// word — both hold zero here — cannot pass unnoticed.
			"%v5 = extractvalue { i64, i64, i64 } %v1, 0",
			"%v7 = extractvalue { i64, i64, i64 } %v1, 2",
			"%v9 = load i64, ptr %v2\n  %v10 = load i64, ptr %v3\n  %v11 = load i64, ptr %v4",
			"%v12 = call i64 %v8(i64 %v9, i64 %v10, i64 %v11)",
		},
	}, {
		// The prelude Result's own return face: `Ok(())` is the fused tag
		// space's zero, and it is the other half of fnRetOperand's sum
		// arm — a declared fn, not a tail, is where it is reachable. The
		// value type is chapter 8's, so the one payload position is the
		// unit position and the aggregate is the zero constant.
		name: "result ok unit",
		mod: ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
			appError(),
			pubFn("go", nil, &ast.NamedType{Name: "Result", Args: []ast.TypeRef{
				&ast.UnitType{}, named("AppError")}}, okReturn()),
			mainDecl(okReturn()),
		}}},
		defines: []string{
			"define { i64, i64, i64 } @main.go() {\nentry:\n  ret { i64, i64, i64 } { i64 0, i64 0, i64 0 }\n}",
		},
	}, {
		// The same sum-parameter signature, reached through a fn value:
		// the adapter's word-per-parameter expansion reads the family's
		// own word list (abiWordTypes), which the direct call above never
		// touches.
		name: "sum fn value",
		mod: ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
			&ast.SumDecl{Name: "Opt", Variants: []ast.Variant{{Name: "A"}, {Name: "B"}}},
			pubFn("use", []ast.Param{{Name: "r", Type: named("Opt")}}, named("Int64"),
				retValue(intLit("3"))),
			appError(),
			mainDecl(
				letBind("f", ident("use")),
				okReturn(),
			),
		}}},
		defines: []string{
			"define internal i64 @.cb0(ptr %env, i64 %a0, i64 %a1, i64 %a2)",
			"%r = call i64 @main.use(i64 %a0, i64 %a1, i64 %a2)",
		},
	}, {
		name: "valueless",
		mod: ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
			pubFn("note", nil, nil, &ast.Return{}),
			appError(),
			mainDecl(
				letDiscard(&ast.Call{Fn: ident("note")}),
				okReturn(),
			),
		}}},
		defines: []string{"define void @main.note()"},
		calls:   []string{"call void %"},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ir, ni := EmitProgram(ModeBuild, []ProgModule{tc.mod})
			if ni != nil {
				t.Fatalf("boundary: %s", ni.What)
			}
			for _, d := range tc.defines {
				wantIR(t, ir, d, tc.name+" define")
			}
			for _, c := range tc.calls {
				wantIR(t, ir, c, tc.name+" call")
			}
		})
	}
}

// --- the synthesized harness (D5) ------------------------------------------

// The test driver: per test — mocks stored into the target slots in
// source order, begin, one spawned wrapper task awaited at the task
// boundary, end, slots restored to the real bodies, one report; the
// summary closes. Test and mock fns are defined under the test
// module's key; wrappers spawn through the M9b task ABI.
func TestM10bHarnessSynthesis(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		pubFn("three", nil, named("Int64"), retValue(intLit("3"))),
	}}}
	tm := ProgModule{Key: "tests.m_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		&ast.TestDecl{Desc: "plain", Body: ast.Block{Items: []ast.Stmt{
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
		&ast.TestDecl{Desc: "mocked", Body: ast.Block{Items: []ast.Stmt{
			&ast.MockDecl{Target: "three", TargetQual: "util", HasRet: true, Ret: named("Int64"),
				Body: ast.Block{Items: []ast.Stmt{retValue(intLit("7"))}}},
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
	}}}
	ir, ni := EmitProgram(ModeTest, []ProgModule{util, tm})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define void @tests.m_test.test.0()", "test fn 0")
	wantIR(t, ir, "define void @tests.m_test.test.1()", "test fn 1")
	wantIR(t, ir, "define i64 @tests.m_test.mock.0()", "mock fn carries the target ABI")
	wantIR(t, ir, "define i32 @__we_main()", "driver is the entry")
	wantIR(t, ir, "store ptr @tests.m_test.mock.0, ptr @slot.util.three", "mock install")
	wantIR(t, ir, "store ptr @util.three, ptr @slot.util.three", "slot restore")
	wantIR(t, ir, "call void @__we_test_begin()", "begin")
	wantIR(t, ir, "call void @__we_test_end()", "end")
	wantIR(t, ir, "__we_task_new(ptr @tests.m_test.wrap.0, ptr null)", "wrapper spawn, zero captures")
	wantIR(t, ir, "call i64 @__we_handle_await(", "await at the task boundary")
	wantIR(t, ir, "call void @__we_test_report(", "report")
	wantIR(t, ir, "call void @__we_test_summary()", "summary")
}

// --- the two new stops (D8) -------------------------------------------------

func TestM10bBndStops(t *testing.T) {
	generic := ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
		&ast.FnDecl{Name: "id", TypeParams: []*ast.TypeParam{{Name: "T"}},
			Params: []ast.Param{{Name: "x", Type: named("T")}}, Ret: named("T"),
			Body: ast.Block{Items: []ast.Stmt{retValue(ident("x"))}}},
		appError(),
		mainDecl(
			letDiscard(&ast.Call{Fn: ident("id"), Args: []ast.Expr{intLit("1")}}),
			okReturn(),
		),
	}}}
	if _, ni := EmitProgram(ModeBuild, []ProgModule{generic}); ni == nil || ni.What != bndGenericFns {
		t.Fatalf("generic: want %q, got %+v", bndGenericFns, ni)
	}

	// The fn body pin re-anchored at T7-2: the Range, String, and List
	// sources are all emitted now — the List literal that anchored this
	// pin through T4-3 and T6 is the T7-2 widening itself — so the stop
	// is the source the build still refuses. The listing below is what
	// that source is at the check face: a user impl of Iterable, whose
	// protocol runs through the interface's own machinery. At the
	// emission face the rule the pin states is the one that holds here —
	// the source is an Ident this build has no List binding for, so the
	// walk it would need does not exist.
	forBody := ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
		recDecl("CountIter", "gc", fld("n", "Int64")),
		recDecl("Range2", "gc", fld("n", "Int64")),
		implDecl("Iterator<Int64>", "CountIter",
			method("next", ast.RecvMutSelf, "Option<Int64>", retValue(ident("None")))),
		implDecl("Iterable<Int64>", "Range2",
			method("iterator", ast.RecvSelf, "CountIter", retValue(construct("CountIter", init1("n", intLit("0")))))),
		pubFn("looped", nil, nil,
			&ast.ForStmt{Pat: &ast.PatBinding{Name: "c"}, Iter: ident("r")},
			&ast.Return{}),
		appError(),
		mainDecl(
			letDiscard(&ast.Call{Fn: ident("looped")}),
			okReturn(),
		),
	}}}
	if _, ni := EmitProgram(ModeBuild, []ProgModule{forBody}); ni == nil || ni.What != bndFnBody {
		t.Fatalf("fn body: want %q, got %+v", bndFnBody, ni)
	}
}

// --- the call faces the fn protocol widened for (M10b goldens) -------------
//
// Four positions take a direct fn call where the M9b expression emitters
// stay call-free (a call enters those sets through a let binding): the
// fn tail return's value, a call's argument of the family's own ABI, the
// assertEqual comparands, and — the valueless early exit — a return
// inside a test body's scope block. The resolution lives at each
// position; the expression sets stay pinned.

// A String parameter rides back out through insertvalue — an SSA pair
// cannot sit inside a ret's aggregate constant (the probe-verified fix:
// the ident tail had never compiled before).
func TestM10bStrParamRet(t *testing.T) {
	m := ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
		pubFn("ident", []ast.Param{{Name: "s", Type: named("String")}}, named("String"),
			retValue(ident("s"))),
		appError(),
		mainDecl(
			letBind("t", &ast.Call{Fn: ident("ident"), Args: []ast.Expr{strLit(`"we"`)}}),
			okReturn(),
		),
	}}}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{m})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "insertvalue { ptr, i64 } undef, ptr %s0, 0", "pair builds through insertvalue")
	wantIR(t, ir, "ret { ptr, i64 } %v", "ret takes the built register")
}

// double(double(n)): the inner call's result is the outer's argument —
// every i64 call site rides its slot (three across the module: quad's
// two, main's one).
func TestM10bNestedCallArg(t *testing.T) {
	m := ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
		pubFn("double", []ast.Param{i64Param("n")}, named("Int64"),
			retValue(binOp("*", ident("n"), intLit("2")))),
		pubFn("quad", []ast.Param{i64Param("n")}, named("Int64"),
			retValue(&ast.Call{Fn: ident("double"), Args: []ast.Expr{
				&ast.Call{Fn: ident("double"), Args: []ast.Expr{ident("n")}}}})),
		appError(),
		mainDecl(
			letDiscard(&ast.Call{Fn: ident("quad"), Args: []ast.Expr{intLit("2")}}),
			okReturn(),
		),
	}}}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{m})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	if got := strings.Count(ir, "= call i64 %"); got != 3 {
		t.Fatalf("want 3 i64 slot calls (nested + outer + main), got %d\n%s", got, ir)
	}
}

// A call comparand contributes its result's face: the Int64-returning
// call rides the numeric helper, the String-returning one the string
// helper.
func TestM10bAssertEqualCallComparands(t *testing.T) {
	tm := ProgModule{Key: "tests.m_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		pubFn("add", []ast.Param{i64Param("a"), i64Param("b")}, named("Int64"),
			retValue(binOp("+", ident("a"), ident("b")))),
		pubFn("tag", []ast.Param{{Name: "s", Type: named("String")}}, named("String"),
			retValue(ident("s"))),
		&ast.TestDecl{Desc: "nums", Body: ast.Block{Items: []ast.Stmt{
			&ast.ExprStmt{Expr: qualCall("st", "assertEqual",
				&ast.Call{Fn: ident("add"), Args: []ast.Expr{intLit("2"), intLit("3")}}, intLit("5"))},
		}}},
		&ast.TestDecl{Desc: "strs", Body: ast.Block{Items: []ast.Stmt{
			&ast.ExprStmt{Expr: qualCall("st", "assertEqual",
				&ast.Call{Fn: ident("tag"), Args: []ast.Expr{strLit(`"we"`)}}, strLit(`"we"`))},
		}}},
	}}}
	ir, ni := EmitProgram(ModeTest, []ProgModule{tm})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "call void @__we_assert_eq_i64(i64 %", "numeric route takes the call's result")
	wantIR(t, ir, "call void @__we_assert_eq_str(ptr %", "string route takes the call's pair")
}

// The test context's valueless early exit works from inside a scope
// block (a failed test's source discharges its handle through a return
// the runtime never reaches past the task-fail tail).
func TestM10bReturnInScope(t *testing.T) {
	tm := ProgModule{Key: "tests.m_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		&ast.TestDecl{Desc: "quick", Body: ast.Block{Items: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{
				&ast.ExprStmt{Expr: &ast.Call{Fn: ident("assert"),
					Args: []ast.Expr{boolLit("false"), strLit(`"boom"`)}}},
				&ast.Return{},
			}}}},
		}}},
	}}}
	ir, ni := EmitProgram(ModeTest, []ProgModule{tm})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@__we_scope_enter", "the scope entered")
	wantIR(t, ir, "ret void", "the early exit rets from inside")
}
