package parser

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M9a concurrency-check (design D1): the three keyword-led productions —
// task effect blocks, the scope compound forms, select — and their
// parse-time diagnostics E1601, E1611, E1612. Written test-first: the
// AST nodes and productions land in T3.

func bodyOf(t *testing.T, src string) ast.Block {
	t.Helper()
	f := wantClean(t, src)
	return f.Items[0].(*ast.FnDecl).Body
}

func TestTaskProduction(t *testing.T) {
	// Expression position: a let initializer carries the task.
	b := bodyOf(t, "fn f() {\n    let t = task effect net io { 1 }\n}\n")
	bind := b.Items[0].(*ast.Binding)
	task, ok := bind.Init.(*ast.TaskExpr)
	if !ok {
		t.Fatalf("let init: want *ast.TaskExpr, got %T", bind.Init)
	}
	if len(task.EffectTags) != 2 || task.EffectTags[0] != "net" || task.EffectTags[1] != "io" {
		t.Fatalf("task tags: %+v", task.EffectTags)
	}
	if len(task.Body.Items) != 1 {
		t.Fatalf("task body: %+v", task.Body)
	}
	// Statement position: a bare task is an expression statement.
	b2 := bodyOf(t, "fn f() {\n    task effect net { 1 }\n}\n")
	if len(b2.Items) != 1 {
		t.Fatalf("one statement wanted, got %d", len(b2.Items))
	}
	es, ok := b2.Items[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("statement task: want *ast.ExprStmt, got %T", b2.Items[0])
	}
	if _, ok := es.Expr.(*ast.TaskExpr); !ok {
		t.Fatalf("statement task: want *ast.TaskExpr, got %T", es.Expr)
	}
	// Omission of the effect segment is E1601 at the task keyword.
	wantDiag(t, "fn f() {\n    scope {\n        task { 1 }\n    }\n}\n", "E1601", "task block without an effect segment", 3, 9)
	wantDiag(t, "fn f() {\n    scope {\n        task net { 1 }\n    }\n}\n", "E1601", "task block without an effect segment", 3, 9)
}

func TestScopeCompoundProduction(t *testing.T) {
	// The four forms, in the expression position that carries their value.
	b := bodyOf(t, "fn f() {\n    let v = scope { 5 }\n}\n")
	sc := b.Items[0].(*ast.Binding).Init.(*ast.ScopeExpr)
	if sc.Timeout != nil || sc.CollectAll {
		t.Fatalf("bare scope: %+v", sc)
	}
	b = bodyOf(t, "fn f() {\n    let v = scope timeout(500) { 5 }\n}\n")
	sc = b.Items[0].(*ast.Binding).Init.(*ast.ScopeExpr)
	if sc.Timeout == nil || exprStr(sc.Timeout) != "500" || sc.CollectAll {
		t.Fatalf("timeout scope: %+v", sc)
	}
	b = bodyOf(t, "fn f() {\n    let v = scope collectAll { 5 }\n}\n")
	sc = b.Items[0].(*ast.Binding).Init.(*ast.ScopeExpr)
	if sc.Timeout != nil || !sc.CollectAll {
		t.Fatalf("collectAll scope: %+v", sc)
	}
	b = bodyOf(t, "fn f() {\n    let v = scope timeout(500) collectAll { 5 }\n}\n")
	sc = b.Items[0].(*ast.Binding).Init.(*ast.ScopeExpr)
	if sc.Timeout == nil || !sc.CollectAll {
		t.Fatalf("composite scope: %+v", sc)
	}
	// Statement position: the bare form is an expression statement.
	b2 := bodyOf(t, "fn f() {\n    scope {\n        let n = 1\n    }\n}\n")
	es, ok := b2.Items[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("statement scope: want *ast.ExprStmt, got %T", b2.Items[0])
	}
	if _, ok := es.Expr.(*ast.ScopeExpr); !ok {
		t.Fatalf("statement scope: want *ast.ScopeExpr, got %T", es.Expr)
	}
	// The timeout clause precedes collectAll; the reverse order fits no
	// production. A non-clause token after scope lands the same way.
	wantDiag(t, "fn f() {\n    let v = scope collectAll timeout(500) { 5 }\n}\n", "E0105", "where the scope body opens", 2, 30)
	wantDiag(t, "fn f() {\n    scope x {}\n}\n", "E0105", "where the scope body opens", 2, 11)
	// An empty body is a clean parse; the value face is the checker's.
	wantClean(t, "fn f() {\n    scope timeout(1) {}\n}\n")
	// The resource form (M6b) still parses on its own path.
	wantClean(t, "fn open() -> Int64 {\n    return 1\n}\nfn f() {\n    scope resource(x = open()) {\n        let n = x\n    }\n}\n")
}

func TestSelectProduction(t *testing.T) {
	b := bodyOf(t, "fn f() {\n    let v = select {\n        case x = a.receive() => x\n        case _ = b.receive() => 0\n    }\n}\n")
	sel, ok := b.Items[0].(*ast.Binding).Init.(*ast.SelectExpr)
	if !ok {
		t.Fatalf("let init: want *ast.SelectExpr, got %T", b.Items[0].(*ast.Binding).Init)
	}
	if len(sel.Cases) != 2 {
		t.Fatalf("two cases wanted, got %d", len(sel.Cases))
	}
	c0, c1 := sel.Cases[0], sel.Cases[1]
	if c0.Wildcard || c0.Name != "x" || exprStr(c0.Source) != "a.receive()" || exprStr(c0.Body) != "x" {
		t.Fatalf("case 0: %+v", c0)
	}
	if !c1.Wildcard || c1.Name != "" || exprStr(c1.Body) != "0" {
		t.Fatalf("case 1: %+v", c1)
	}
	// Patterns beyond a binding name or the wildcard are E1611 at the
	// case keyword.
	wantDiag(t, "fn f() {\n    let v = select {\n        case Some(x) = a.receive() => x\n        case y = a.receive() => y\n    }\n}\n", "E1611", "select case pattern is not a binding or the wildcard", 3, 9)
	wantDiag(t, "fn f() {\n    let v = select {\n        case 1 = a.receive() => 0\n        case y = a.receive() => y\n    }\n}\n", "E1611", "select case pattern is not a binding or the wildcard", 3, 9)
	// Fewer than two cases is E1612 at the select keyword.
	wantDiag(t, "fn f() {\n    let v = select {\n        case x = a.receive() => x\n    }\n}\n", "E1612", "select holds fewer than two cases", 2, 13)
	wantDiag(t, "fn f() {\n    let v = select {\n    }\n}\n", "E1612", "select holds fewer than two cases", 2, 13)
}
