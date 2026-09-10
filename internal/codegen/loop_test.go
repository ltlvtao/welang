package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T2 statement set (design D1): the loop forms and their exits. while
// re-evaluates its condition at the head, loop branches back to its body,
// break leaves the innermost loop, continue restarts it — and an exit
// that crosses an open scope discharges that scope first (chapter
// 18:231): the fail-fast forms cancel then join, collectAll joins alone.
// Red before the statement set: every shape stopped at bndMainBody on the
// real toolchain (probe battery; loop-in-fn and loop-in-task stopped at
// bndFnBody/bndTaskBody).

// whileShape builds `while cond { body }`.
func whileShape(cond ast.Expr, body ...ast.Stmt) *ast.While {
	return &ast.While{Cond: cond, Body: ast.Block{Items: body}}
}

// order checks that the needles appear in the IR in the given order.
func order(t *testing.T, ir string, needles ...string) {
	t.Helper()
	at := 0
	for _, n := range needles {
		i := strings.Index(ir[at:], n)
		if i < 0 {
			t.Fatalf("IR missing %q after byte %d:\n%s", n, at, ir)
		}
		at += i + len(n)
	}
}

// TestWhileBreakContinue pins the two targets: break branches to the exit
// label, continue branches back to the head label.
func TestWhileBreakContinue(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "i", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("0")},
		whileShape(binOp("<", ident("i"), intLit("3")),
			&ast.ExprStmt{Expr: &ast.If{Cond: binOp("==", ident("i"), intLit("1")), Then: ast.Block{Items: []ast.Stmt{&ast.Continue{}}}}},
			&ast.ExprStmt{Expr: &ast.If{Cond: binOp("==", ident("i"), intLit("2")), Then: ast.Block{Items: []ast.Stmt{&ast.Break{}}}}},
		),
		okReturn(),
	), "br label %whhead", "whbody", "whexit")
	if strings.Contains(ir, "unreachable\n  br") {
		t.Fatalf("an instruction lands after a terminator:\n%s", ir)
	}
	order(t, ir, "br label %whexit", "br label %whhead")
}

// TestLoopInfinite pins `loop block`: break exits, continue restarts the
// body (the frame's two targets are the body label).
func TestLoopInfinite(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Loop{Body: ast.Block{Items: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.If{Cond: binOp("==", intLit("1"), intLit("1")), Then: ast.Block{Items: []ast.Stmt{&ast.Break{}}}}},
		}}},
		okReturn(),
	), "br label %lpbody", "lpexit")
	if strings.Contains(ir, "unreachable\n  br") {
		t.Fatalf("an instruction lands after a terminator:\n%s", ir)
	}
}

// TestBreakPierceScope: a break inside a scope inside a loop discharges
// the scope at the break site — cancel (fail-fast) then join — before
// branching to the loop's exit.
func TestBreakPierceScope(t *testing.T) {
	ir := assertClean(t, m9bModule(
		whileShape(binOp("<", intLit("0"), intLit("3")),
			&ast.ExprStmt{Expr: &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{
				&ast.Break{},
			}}}},
		),
		okReturn(),
	))
	order(t, ir,
		"call void @__we_scope_cancel(",
		"call i64 @__we_scope_leave(",
		"br label %whexit",
	)
	// The scope's own tail must not join again: the body diverged, so its
	// leave exists only on the pierce path (one cancel, one leave).
	if got := strings.Count(ir, "call i64 @__we_scope_leave("); got != 1 {
		t.Fatalf("want exactly one leave (the pierce path), got %d:\n%s", got, ir)
	}
	if strings.Contains(ir, "unreachable\n  br") {
		t.Fatalf("an instruction lands after a terminator:\n%s", ir)
	}
}

// TestContinuePierceCollect: the collectAll form joins without
// cancelling — one leave, no cancel — before branching back to the head.
func TestContinuePierceCollect(t *testing.T) {
	ir := assertClean(t, m9bModule(
		whileShape(binOp("<", intLit("0"), intLit("3")),
			&ast.ExprStmt{Expr: &ast.ScopeExpr{CollectAll: true, Body: ast.Block{Items: []ast.Stmt{
				&ast.Continue{},
			}}}},
		),
		okReturn(),
	))
	order(t, ir,
		"call i64 @__we_scope_leave(",
		"br label %whhead",
	)
	if strings.Contains(ir, "__we_scope_cancel") {
		t.Fatalf("a collectAll pierce must not cancel:\n%s", ir)
	}
}

// TestBreakInsideScopeStays: a loop inside a scope does not pierce it —
// the break branches to the loop's exit inside the scope body, and the
// scope joins normally at its own end.
func TestBreakInsideScopeStays(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.ExprStmt{Expr: &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{
			whileShape(binOp("<", intLit("0"), intLit("3")), &ast.Break{}),
		}}}},
		okReturn(),
	))
	order(t, ir,
		"br label %whexit",
		"call i64 @__we_scope_leave(",
	)
	if strings.Contains(ir, "__we_scope_cancel") {
		t.Fatalf("the scope encloses the loop; the exit does not cross it:\n%s", ir)
	}
}
