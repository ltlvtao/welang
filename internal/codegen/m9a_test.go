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
		}}, bndOtherFns, ""},
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
