package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M9a's new expression nodes stop at the M8 body boundary (design D9):
// statement position, let-initializer position, the io-call argument, and
// the containing-fn face are each pinned per-form here, so a future
// emitter case cannot silently swallow one ("explicit case, never
// vanish", the M4 discipline). The M9b flips ride the T6/T7 records'
// disclosures: the two constructor forms turned green with T6, the task
// and scope value forms with T7 (they pin their ABI symbols now), and
// the bare task statement stopped inside the task body with T7 — its
// unit-valued body reads under the task word. The forms still stopping
// at the main body carry the v2 vocabulary. The conformance goldens pin
// the CLI face of the same stops; these pin the emitter's own.

// taskExpr builds `task effect net { body }`.
func taskExpr(body ast.Expr) *ast.TaskExpr {
	return &ast.TaskExpr{
		EffectTags: []string{"net"},
		EffectLine: 1, EffectCol: 6,
		Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: body}}},
	}
}

// scopeExpr builds a scope form around one body expression.
func scopeExpr(timeout bool, body ast.Expr) *ast.ScopeExpr {
	s := &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: body}}}}
	if timeout {
		s.Timeout = &ast.Literal{Kind: "int", Text: "5"}
	}
	return s
}

// selectUnit builds a two-case select of channel receives with unit
// bodies — the statement-position shape.
func selectUnit() *ast.SelectExpr {
	recv := func(ch string) ast.Expr {
		return &ast.Call{Fn: &ast.Member{Recv: &ast.Ident{Name: ch}, Name: "receive"}}
	}
	return &ast.SelectExpr{Cases: []ast.SelectCase{
		{Wildcard: true, Source: recv("a"), Body: &ast.Unit{}, Line: 1, Col: 5},
		{Wildcard: true, Source: recv("b"), Body: &ast.Unit{}, Line: 2, Col: 5},
	}}
}

// mutexCtor builds `conc.Mutex(0)`.
func mutexCtor() *ast.Call {
	return &ast.Call{
		Fn:   &ast.Member{Recv: &ast.Ident{Name: "conc"}, Name: "Mutex"},
		Args: []ast.Expr{&ast.Literal{Kind: "int", Text: "0"}},
	}
}

// m9aModule wraps main-shaped statements with the concurrent import and
// the skeleton error sum.
func m9aModule(stmts ...ast.Stmt) *ast.File {
	return &ast.File{Items: []ast.Item{
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		appError(),
		mainDecl(stmts...),
	}}
}

func TestM9aBodyBoundaryWhats(t *testing.T) {
	unit := &ast.Unit{}
	cases := []struct {
		name string
		file *ast.File
		what string
		abi  string // the flipped-green forms' ABI symbol
	}{
		{"scope statement", m9aModule(
			&ast.ExprStmt{Expr: scopeExpr(false, unit)},
			okReturn()), bndMainBody, ""},
		{"bare task statement", m9aModule(
			&ast.ExprStmt{Expr: taskExpr(unit)},
			okReturn()), bndTaskBody, ""},
		{"select statement", m9aModule(
			&ast.ExprStmt{Expr: selectUnit()},
			okReturn()), bndMainBody, ""},
		{"let init task", m9aModule(
			&ast.Binding{Kw: "let", Name: "t", Init: taskExpr(&ast.Literal{Kind: "int", Text: "1"})},
			okReturn()), "", "__we_task_new"},
		{"let init scope value", m9aModule(
			&ast.Binding{Kw: "let", Name: "r", Init: scopeExpr(true, &ast.Literal{Kind: "int", Text: "5"})},
			okReturn()), "", "__we_scope_enter"},
		{"let init select", m9aModule(
			&ast.Binding{Kw: "let", Name: "v", Init: selectUnit()},
			okReturn()), bndMainBody, ""},
		{"conc constructor binding", m9aModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
			okReturn()), "", "__we_prim_new_mutex"},
		{"conc constructor discarded", m9aModule(
			&ast.Binding{Kw: "let", Name: "_", Init: mutexCtor()},
			okReturn()), "", "__we_prim_new_mutex"},
		{"io call argument concurrent expression", &ast.File{Items: []ast.Item{
			stdIoImport("io"),
			&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
			appError(),
			mainDecl(ioCall("io", "println", taskExpr(&ast.Literal{Kind: "string", Text: `"s"`})), okReturn()),
		}}, bndMainBody, ""},
		{"fn containing concurrent forms", &ast.File{Items: []ast.Item{
			&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
			appError(),
			&ast.FnDecl{
				Pub: true, Name: "go",
				Ret:  &ast.NamedType{Name: "Result", Args: []ast.TypeRef{&ast.UnitType{}, &ast.NamedType{Name: "AppError"}}},
				Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: scopeExpr(false, unit)}}},
			},
			mainDecl(okReturn()),
		}}, bndFnBody, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ir, ni := Emit(c.file, "demo")
			if c.what == "" {
				// The flipped-green forms: clean emission carrying the
				// form's ABI symbol.
				if ni != nil {
					t.Fatalf("expected clean emission, got boundary %q", ni.What)
				}
				if !strings.Contains(ir, c.abi) {
					t.Fatalf("IR missing %q:\n%s", c.abi, ir)
				}
				return
			}
			if ni == nil {
				t.Fatalf("expected boundary %q, got none", c.what)
			}
			if ni.What != c.what {
				t.Fatalf("boundary What %q, want %q", ni.What, c.what)
			}
		})
	}
}

// The std.concurrent import rides the M8 std-erasure path unchanged: it
// leaves no IR trace, and the plain hello body emits byte-identically.
func TestM9aStdConcurrentImportErases(t *testing.T) {
	f := helloModule("io")
	f.Items = append([]ast.Item{&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"}}, f.Items...)
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("expected clean emission, got boundary %q", ni.What)
	}
	if ir != m8HelloIR {
		t.Fatalf("IR changed under the concurrent import:\n%s", ir)
	}
}

// syncMutex is `conc.Mutex<Int64>` — a chapter 18 object written at a
// parameter position.
func syncMutex() ast.TypeRef {
	return &ast.NamedType{Name: "Mutex", Qual: "conc", Args: []ast.TypeRef{named("Int64")}}
}

// A chapter 18 object is a primitive, and a primitive at a parameter
// position crosses as its own pointer: the callee's define takes the ptr
// directly, and a method call on the parameter resolves through the prim
// environment exactly as it does on an object the frame constructed
// itself. The object stays rooted across the call — the caller pushed it
// when it made it, and the caller's frame is live while the callee runs —
// so the callee owes no root work of its own.
func TestM9aSyncTypedParameterBindsThePointer(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		appError(),
		pubFn("bump", []ast.Param{{Name: "m", Type: syncMutex()}}, named("Int64"),
			retValue(qualCall("m", "get"))),
		mainDecl(
			letBind("c", mutexCtor()),
			letDiscard(call(ident("bump"), ident("c"))),
			letDiscard(call(ident("bump"), mutexCtor())),
			okReturn()),
	}}
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// The parameter face is the pointer itself, and the body's method call
	// takes that name — no load, because the parameter IS the object.
	wantIR(t, ir, "define i64 @main.bump(ptr %m) {", "the sync-typed parameter's face")
	wantIR(t, ir, "  %v6 = call i64 @__we_prim_get(ptr %m)", "the method call on the parameter")
	// Both argument spellings cross as a pointer: a bound object hands
	// over the one it is holding, and one constructed inline hands over
	// what the construction just returned.
	wantIR(t, ir, "  %v2 = call i64 %v1(ptr %v0)", "the bound-object argument")
	wantIR(t, ir, "  %v4 = call ptr @__we_prim_new_mutex(i64 0)\n  call void @__we_root_push(ptr %v4)\n"+
		"  %v5 = call i64 %v3(ptr %v4)", "the inline-construction argument")
}

// The prim face is a parameter face only. A chapter 18 object coming back
// would need a callee-side operand the result face has not got, and the
// caller would read the value it hands over as a scalar; fitAbi refuses
// the return face rather than guess, and the refusal carries the body word
// because the return is written in the body.
func TestM9aPrimReturnStaysAtTheBoundary(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		appError(),
		pubFn("make", nil, syncMutex(), retValue(mutexCtor())),
		mainDecl(okReturn()),
	}}
	_, ni := Emit(f, "demo")
	if ni == nil {
		t.Fatal("expected a boundary for a prim-returning fn, got clean emission")
	}
	if ni.What != bndFnBody {
		t.Fatalf("boundary What %q, want %q", ni.What, bndFnBody)
	}
}

// The adapter a fn value crosses as is built from the same word list the
// direct call uses, so a prim parameter is a `ptr` in the adapter's
// parameter list too. The adapter is the face where the word list is the
// only thing that spells the parameter — the direct call descends from
// the define's own list — so an adapter spelling it `i64` would declare a
// callee whose type does not match the fn it forwards to.
func TestM9aPrimParameterTakenAsAValue(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		appError(),
		pubFn("use", []ast.Param{{Name: "m", Type: syncMutex()}}, named("Int64"),
			retValue(qualCall("m", "get"))),
		mainDecl(letBind("f", ident("use")), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define internal i64 @.cb0(ptr %env, ptr %a0) {", "the adapter's prim word")
	wantIR(t, ir, "  %r = call i64 @main.use(ptr %a0)", "the forwarded argument")
}

// A custom-effect fn's gate is a same-ABI clone of the real define, and
// it builds its parameter list the same way — so a prim parameter is a
// `ptr` in the gate too, and the forwarded argument keeps that spelling.
func TestM9aPrimParameterUnderAnEffectGate(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		&ast.EffectDecl{Name: "db"},
		appError(),
		&ast.FnDecl{Pub: true, Name: "peek", EffectTags: []string{"db"},
			Params: []ast.Param{{Name: "m", Type: syncMutex()}}, Ret: named("Int64"),
			Body: ast.Block{Items: []ast.Stmt{retValue(qualCall("m", "get"))}}},
		mainDecl(letBind("c", mutexCtor()), letDiscard(call(ident("peek"), ident("c"))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define internal i64 @main.peek.fxgate(ptr %m) {", "the gate's prim parameter")
	wantIR(t, ir, "  %r = call i64 @main.peek(ptr %m)", "the forwarded argument")
}

// The family is std.concurrent's, identified by the import the name
// arrives through and not by the name alone. A module that aliases
// something else as `conc` and exports a type called `Mutex` is not a
// chapter 18 object: a real one ignores its type argument because each of
// them is one pointer, while this one is the user's own type wearing the
// same name — so it is refused rather than classified as a pointer.
func TestM9aPrimNameFromANonConcurrentImportStops(t *testing.T) {
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		&ast.Import{Path: []string{"my", "helpers"}, Alias: "conc"},
		appError(),
		pubFn("take", []ast.Param{{Name: "m", Type: syncMutex()}}, named("Int64"),
			retValue(intLit("0"))),
		mainDecl(okReturn()),
	}}}})
	if ni == nil {
		t.Fatal("expected a boundary for a conc-qualified name that is not std.concurrent's")
	}
	if ni.What != bndFnBody {
		t.Fatalf("boundary What %q, want %q", ni.What, bndFnBody)
	}
}
