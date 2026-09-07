package codegen

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M9a's new expression nodes stop at the M8 body boundary (design D9):
// statement position, let-initializer position, the concurrent
// constructors, the io-call argument, and the containing-fn face are each
// pinned per-form here, so a future emitter case cannot silently swallow
// one ("explicit case, never vanish", the M4 discipline). The
// conformance goldens pin the CLI face of the same stops; these pin the
// emitter's own.

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
	}{
		{"scope statement", m9aModule(
			&ast.ExprStmt{Expr: scopeExpr(false, unit)},
			okReturn()), bndMainBody},
		{"bare task statement", m9aModule(
			&ast.ExprStmt{Expr: taskExpr(unit)},
			okReturn()), bndMainBody},
		{"select statement", m9aModule(
			&ast.ExprStmt{Expr: selectUnit()},
			okReturn()), bndMainBody},
		{"let init task", m9aModule(
			&ast.Binding{Kw: "let", Name: "t", Init: taskExpr(&ast.Literal{Kind: "int", Text: "1"})},
			okReturn()), bndMainBody},
		{"let init scope value", m9aModule(
			&ast.Binding{Kw: "let", Name: "r", Init: scopeExpr(true, &ast.Literal{Kind: "int", Text: "5"})},
			okReturn()), bndMainBody},
		{"let init select", m9aModule(
			&ast.Binding{Kw: "let", Name: "v", Init: selectUnit()},
			okReturn()), bndMainBody},
		{"conc constructor binding", m9aModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
			okReturn()), bndMainBody},
		{"conc constructor discarded", m9aModule(
			&ast.Binding{Kw: "let", Name: "_", Init: mutexCtor()},
			okReturn()), bndMainBody},
		{"io call argument concurrent expression", &ast.File{Items: []ast.Item{
			stdIoImport("io"),
			&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
			appError(),
			mainDecl(ioCall("io", "println", taskExpr(&ast.Literal{Kind: "string", Text: `"s"`})), okReturn()),
		}}, bndMainBody},
		{"fn containing concurrent forms", &ast.File{Items: []ast.Item{
			&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
			appError(),
			&ast.FnDecl{
				Pub: true, Name: "go",
				Ret:  &ast.NamedType{Name: "Result", Args: []ast.TypeRef{&ast.UnitType{}, &ast.NamedType{Name: "AppError"}}},
				Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: scopeExpr(false, unit)}}},
			},
			mainDecl(okReturn()),
		}}, bndOtherFns},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ni := Emit(c.file, "demo")
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
