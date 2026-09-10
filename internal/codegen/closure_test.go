package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T6 closures and fn values (design D5, chapter 12). The plan's carrier is
// the `{fnptr, env}` pair behind one gc pointer; the spellings below are
// the emitter's:
//
//   - a closure is emitted as an internal thunk whose first parameter is
//     its environment block, and whose body IS a function body (its tail
//     expression the implicit value, its returns the thunk's own);
//   - the block is built at the creation site, where the captured bindings
//     still live, and its header carries a trace bitmap — one bit per
//     payload word — so the collector keeps what the block points at;
//   - a captured word is an i64: a scalar's value, a pointer's address, a
//     String's pair;
//   - one convention at every call site — `<fnptr>(env, args...)` — which
//     is why a declared fn in value position is fronted by an adapter
//     thunk that supplies the environment parameter its own signature
//     does not have.

// TestClosureScalarCaptureFreezesAWord: a scalar capture copies its word
// into the block at the creation site (chapter 12: the value is frozen
// there), and the thunk reads that word back at its entry.
func TestClosureScalarCaptureFreezesAWord(t *testing.T) {
	ir := assertClean(t, mutexModule(nil,
		&ast.Binding{Kw: "let", Name: "k", Init: intLit("3")},
		&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
		varLit("n", "0"),
		assignTo("n", captureOn("m", binOp("+", ident("v"), ident("k")))),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define internal i64 @.cb0(ptr %env, i64 %v) {")
	// The block holds one word and traces none of it: a scalar is not a
	// gc reference, and saying it is would send the collector after an
	// integer.
	if !strings.Contains(ir, "@.emap0 = private unnamed_addr constant [1 x i64] [i64 0]") {
		t.Fatalf("one untraced word:\n%s", ir)
	}
	if !strings.Contains(ir, "= call ptr @__we_alloc(i64 24)") {
		t.Fatalf("the block is the header plus one word:\n%s", ir)
	}
	// The freeze: the word is the scalar's value read at the creation
	// site, not the name it was bound to.
	if !regexp.MustCompile(`getelementptr i8, ptr %v\d+, i64 16\n\s+store i64 3, ptr %v\d+`).MatchString(ir) {
		t.Fatalf("the frozen word is stored at the block's first slot:\n%s", ir)
	}
	if !regexp.MustCompile(`getelementptr i8, ptr %env, i64 16\n\s+%v\d+ = load i64, ptr %v\d+`).MatchString(ir) {
		t.Fatalf("the thunk reads the captured word back at 16:\n%s", ir)
	}
}

// TestClosureGcCaptureTracesItsPointer: a gc binding is captured by
// reference — the block carries its address word, the bitmap marks that
// word, and the thunk converts it back to a pointer. The record key
// survives the trip, so a field read through the capture lands on the
// record's own offsets.
func TestClosureGcCaptureTracesItsPointer(t *testing.T) {
	ir := assertClean(t, mutexModule(
		[]ast.Item{recDecl("Cell", "gc", fld("n", "Int64"))},
		&ast.Binding{Kw: "let", Name: "c", Init: construct("Cell", init1("n", intLit("5")))},
		&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
		varLit("n", "0"),
		assignTo("n", captureOn("m", binOp("+", ident("v"), memberOf(ident("c"), "n")))),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define internal i64 @.cb0(ptr %env, i64 %v) {")
	if !strings.Contains(ir, "@.emap0 = private unnamed_addr constant [1 x i64] [i64 1]") {
		t.Fatalf("the pointer's word is traced:\n%s", ir)
	}
	if !regexp.MustCompile(`%v\d+ = ptrtoint ptr %v\d+ to i64`).MatchString(ir) {
		t.Fatalf("the creation site stores the address as a word:\n%s", ir)
	}
	if !regexp.MustCompile(`%v\d+ = inttoptr i64 %v\d+ to ptr`).MatchString(ir) {
		t.Fatalf("the thunk converts the word back to a pointer:\n%s", ir)
	}
	if !strings.Contains(ir, "@.map.main.Cell") {
		t.Fatalf("the capture keeps the record it points at:\n%s", ir)
	}
}

// TestClosureStringCaptureCopiesThePairUntraced: a String capture copies
// the header's two words and traces neither. The bytes are not this
// collector's: the runtime carves them with malloc and never frees them,
// and the literal form points into the module's read-only pool — a mark
// bit written through either is a write into memory this collector does
// not own.
func TestClosureStringCaptureCopiesThePairUntraced(t *testing.T) {
	ir := assertClean(t, mutexModule(nil,
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
		varLit("n", "0"),
		assignTo("n", captureOn("m", binOp("+", ident("v"), callOn(ident("s"), "byteLength")))),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define internal i64 @.cb0(ptr %env, i64 %v) {")
	if !strings.Contains(ir, "@.emap0 = private unnamed_addr constant [1 x i64] [i64 0]") {
		t.Fatalf("neither word of the pair is traced:\n%s", ir)
	}
	if !strings.Contains(ir, "= call ptr @__we_alloc(i64 32)") {
		t.Fatalf("the pair takes two words (16 + 2*8):\n%s", ir)
	}
	// The pair is read exactly as any String read is: the literal interns
	// into the module's constants, and both words are stored.
	if !regexp.MustCompile(`%v\d+ = ptrtoint ptr @\.s0 to i64`).MatchString(ir) {
		t.Fatalf("the data word comes from the interned literal:\n%s", ir)
	}
	if !regexp.MustCompile(`getelementptr i8, ptr %v\d+, i64 24\n\s+store i64 3, ptr %v\d+`).MatchString(ir) {
		t.Fatalf("the length rides the pair's second word:\n%s", ir)
	}
}

// TestZeroCaptureClosurePassesANullEnv: a closure that reads nothing from
// its creation site has no block to build, and its thunk's environment is
// the null pointer. This is the shape the goldens already carried before
// the task, which is why they do not move.
func TestZeroCaptureClosurePassesANullEnv(t *testing.T) {
	ir := assertClean(t, mutexModule(nil,
		&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
		varLit("n", "0"),
		assignTo("n", captureOn("m", binOp("+", ident("v"), intLit("1")))),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define internal i64 @.cb0(ptr %env, i64 %v) {")
	if strings.Contains(ir, "@.emap") {
		t.Fatalf("no captures, no block:\n%s", ir)
	}
	if !regexp.MustCompile(`call i64 @__we_prim_update\(ptr %v\d+, ptr @\.cb0, ptr null\)`).MatchString(ir) {
		t.Fatalf("the callback ABI takes (fn, env) and the env is null:\n%s", ir)
	}
}

// TestCallbackAbiTakesFnAndEnv: the primitive callback ABI is the pair,
// not a packed struct — a captured closure's call site hands the thunk
// and the block over as two arguments (design D5's compatibility rule).
func TestCallbackAbiTakesFnAndEnv(t *testing.T) {
	ir := assertClean(t, mutexModule(
		[]ast.Item{recDecl("Cell", "gc", fld("n", "Int64"))},
		&ast.Binding{Kw: "let", Name: "c", Init: construct("Cell", init1("n", intLit("5")))},
		&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
		varLit("n", "0"),
		assignTo("n", captureOn("m", binOp("+", ident("v"), memberOf(ident("c"), "n")))),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "")
	if !regexp.MustCompile(`call i64 @__we_prim_update\(ptr %v\d+, ptr @\.cb0, ptr %v\d+\)`).MatchString(ir) {
		t.Fatalf("(fn, env) as two pointer words:\n%s", ir)
	}
}

// TestFnValueIsOneGcCarrier: a fn value is one pointer to a gc block
// holding the pair. Its trace descriptor marks the second word only — the
// first is a code pointer, and marking it would send the collector at
// text.
func TestFnValueIsOneGcCarrier(t *testing.T) {
	ir := assertClean(t, fnValueModule(
		&ast.Binding{Kw: "let", Name: "f", Typ: &ast.FnType{Params: []ast.TypeRef{named("Int64")}, Ret: named("Int64")},
			Init: &ast.Closure{Params: []ast.Param{{Name: "x"}}, Short: true,
				Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: binOp("+", ident("x"), intLit("1"))}}}}},
		letBind("y", &ast.Call{Fn: ident("f"), Args: []ast.Expr{intLit("2")}}),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("y"))),
		okReturn(),
	))
	if !strings.Contains(ir, "@.fnmap0 = private unnamed_addr constant [1 x i64] [i64 2]") {
		t.Fatalf("the carrier's descriptor traces its second word only:\n%s", ir)
	}
	if !strings.Contains(ir, "= call ptr @__we_alloc(i64 32)") {
		t.Fatalf("the carrier is the header plus both words:\n%s", ir)
	}
	if !regexp.MustCompile(`getelementptr i8, ptr %v\d+, i64 16\n\s+store ptr @\.cb0, ptr`).MatchString(ir) {
		t.Fatalf("the thunk is the carrier's first word:\n%s", ir)
	}
	if !regexp.MustCompile(`getelementptr i8, ptr %v\d+, i64 24\n\s+store ptr null, ptr`).MatchString(ir) {
		t.Fatalf("the environment is the carrier's second word (null, no captures):\n%s", ir)
	}
}

// TestFnValueCallLoadsThePairBackOut: a call through a bound fn value goes
// through the pair the carrier holds — the code pointer as the callee, the
// environment as the first argument. No signature is re-derived at the
// call site: the value carries the one it was built with.
func TestFnValueCallLoadsThePairBackOut(t *testing.T) {
	ir := assertClean(t, fnValueModule(
		&ast.Binding{Kw: "let", Name: "f", Typ: &ast.FnType{Params: []ast.TypeRef{named("Int64")}, Ret: named("Int64")},
			Init: &ast.Closure{Params: []ast.Param{{Name: "x"}}, Short: true,
				Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: binOp("+", ident("x"), intLit("1"))}}}}},
		letBind("y", &ast.Call{Fn: ident("f"), Args: []ast.Expr{intLit("2")}}),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("y"))),
		okReturn(),
	))
	if !regexp.MustCompile(`getelementptr i8, ptr %v\d+, i64 16\n\s+%v\d+ = load ptr, ptr %v\d+\n\s+%v\d+ = getelementptr i8, ptr %v\d+, i64 24\n\s+%v\d+ = load ptr, ptr %v\d+\n\s+%v\d+ = call i64 %v\d+\(ptr %v\d+, i64 2\)`).MatchString(ir) {
		t.Fatalf("the call loads both words and passes env first:\n%s", ir)
	}
}

// TestDeclaredFnInValuePositionTakesAnAdapter: a declared fn's own
// signature has no environment parameter, but every fn value is called as
// `<fnptr>(env, args...)`. The value is therefore an adapter thunk's
// pointer beside a null environment — one convention at the call site is
// what lets a body call through a parameter without knowing which kind of
// fn value arrived.
func TestDeclaredFnInValuePositionTakesAnAdapter(t *testing.T) {
	ir := assertClean(t, fnApplyModule(
		letBind("a", &ast.Call{Fn: ident("apply"), Args: []ast.Expr{ident("square"), intLit("2")}}),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("a"))),
		okReturn(),
	))
	if !strings.Contains(ir, "define internal i64 @.cb0(ptr %env, i64 %a0) {") {
		t.Fatalf("the adapter takes the environment the fn does not:\n%s", ir)
	}
	// The adapter forwards the words as they arrive: no environment in,
	// none out.
	if !strings.Contains(ir, "call i64 @main.square(i64 %a0)") {
		t.Fatalf("the adapter forwards to the fn's own symbol:\n%s", ir)
	}
	if !regexp.MustCompile(`store ptr @\.cb0, ptr %v\d+`).MatchString(ir) {
		t.Fatalf("the value's code pointer is the adapter's:\n%s", ir)
	}
	if !regexp.MustCompile(`store ptr null, ptr %v\d+`).MatchString(ir) {
		t.Fatalf("a declared fn captures nothing:\n%s", ir)
	}
}

// TestFnTypedParameterCrossesAsOnePointer: a fn-typed parameter crosses
// the boundary as the one carrier pointer it is — the callee reads the
// pair out of it and calls through.
func TestFnTypedParameterCrossesAsOnePointer(t *testing.T) {
	ir := assertClean(t, fnApplyModule(
		letBind("a", &ast.Call{Fn: ident("apply"), Args: []ast.Expr{ident("square"), intLit("2")}}),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("a"))),
		okReturn(),
	))
	if !strings.Contains(ir, "define i64 @main.apply(ptr %f, i64 %v) {") {
		t.Fatalf("the fn-typed parameter is one pointer:\n%s", ir)
	}
	if !regexp.MustCompile(`%v\d+ = call i64 %v\d+\(ptr %v\d+, i64 %v\)`).MatchString(ir) {
		t.Fatalf("the callee calls through the carrier with env first:\n%s", ir)
	}
}

// TestBareParameterClosureTakesThePositionsType: a closure written with
// bare parameters in a fn-typed argument position takes its signature
// from the parameter's declaration (chapter 12: the expected type is what
// a bare parameter reads).
func TestBareParameterClosureTakesThePositionsType(t *testing.T) {
	ir := assertClean(t, fnApplyModule(
		letBind("a", &ast.Call{Fn: ident("apply"), Args: []ast.Expr{
			&ast.Closure{Params: []ast.Param{{Name: "x"}}, Short: true,
				Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: binOp("*", ident("x"), intLit("2"))}}}},
			intLit("5"),
		}}),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("a"))),
		okReturn(),
	))
	if !strings.Contains(ir, "define internal i64 @.cb0(ptr %env, i64 %x) {") {
		t.Fatalf("the bare parameter takes the position's Int64:\n%s", ir)
	}
	if !regexp.MustCompile(`call i64 %v\d+\(ptr \S+, i64 5\)`).MatchString(ir) {
		t.Fatalf("the closure's carrier is called at the declared signature:\n%s", ir)
	}
}

// mutexModule wraps a concurrent program: the imports, the error sum, the
// declarations, and the main-shaped body.
func mutexModule(decls []ast.Item, stmts ...ast.Stmt) *ast.File {
	items := []ast.Item{
		stdIoImport("io"),
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		appError(),
	}
	items = append(items, decls...)
	items = append(items, mainDecl(stmts...))
	return &ast.File{Items: items}
}

// captureOn builds `m.update(|v| body)` — the callback position whose
// closure is the one under test.
func captureOn(cell string, body ast.Expr) *ast.Call {
	return callOn(ident(cell), "update", &ast.Closure{
		Params: []ast.Param{{Name: "v"}}, Short: true,
		Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: body}}},
	})
}

// fnApplyModule is the chapter 12 pair: a declared fn `square`, a fn-typed
// `apply`, and one main-shaped caller.
func fnApplyModule(extra ...ast.Stmt) *ast.File {
	stmts := append([]ast.Stmt(nil), extra...)
	return &ast.File{Items: []ast.Item{stdIoImport("io"), appError(),
		pubFn("square", []ast.Param{{Name: "n", Type: named("Int64")}}, named("Int64"),
			retValue(binOp("*", ident("n"), ident("n")))),
		pubFn("apply", []ast.Param{
			{Name: "f", Type: &ast.FnType{Params: []ast.TypeRef{named("Int64")}, Ret: named("Int64")}},
			{Name: "v", Type: named("Int64")}}, named("Int64"),
			retValue(&ast.Call{Fn: ident("f"), Args: []ast.Expr{ident("v")}})),
		mainDecl(stmts...)}}
}

// fnValueModule is the smallest module that binds a closure as a value.
func fnValueModule(stmts ...ast.Stmt) *ast.File {
	return &ast.File{Items: []ast.Item{stdIoImport("io"), appError(), mainDecl(stmts...)}}
}

// TestHigherOrderFnInValuePositionTakesAnAdapter: an adapter's own
// parameters cross at the declared words, so a fn-typed parameter is one
// carrier pointer there too — the shape that lets a higher-order fn be
// passed as a value like any other.
func TestHigherOrderFnInValuePositionTakesAnAdapter(t *testing.T) {
	sig := &ast.FnType{Params: []ast.TypeRef{named("Int64")}, Ret: named("Int64")}
	ir := assertClean(t, fnApplyModule(
		&ast.Binding{Kw: "let", Name: "g", Typ: &ast.FnType{Params: []ast.TypeRef{sig, named("Int64")}, Ret: named("Int64")},
			Init: ident("apply")},
		letBind("b", &ast.Call{Fn: ident("g"), Args: []ast.Expr{ident("square"), intLit("3")}}),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("b"))),
		okReturn(),
	))
	if !strings.Contains(ir, "define internal i64 @.cb0(ptr %env, ptr %a0, i64 %a1) {") {
		t.Fatalf("the adapter takes the carrier as one pointer word:\n%s", ir)
	}
	if !strings.Contains(ir, "call i64 @main.apply(ptr %a0, i64 %a1)") {
		t.Fatalf("and forwards it whole to the fn's own symbol:\n%s", ir)
	}
}
