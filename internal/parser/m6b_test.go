package parser

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M6b (modules-errors) parser tests: the chapter 13 scope resource
// statement and the chapter 14 `?` propagation suffix (design D1).
// Structural cases pin the tree shapes T3 must produce — ScopeRes with
// its binding list, Prop as a level-1 postfix — and diagnostic cases
// mirror the T1 golden table's parse-level contract (E0012/E0404 in the
// scope head, the E0202 value-position family, the E0105 locks) plus the
// bndScope split: chapter 13 forms land, chapter 18 forms keep their own
// boundary row.

// --- D1: the scope resource statement ----------------------------------------

func TestScopeResStmt(t *testing.T) {
	// Single binding: name = expr, then a block.
	ss := stmtsOf(t, "scope resource(f = openFile()) {\n        let n = f.fd\n    }\n")
	sr, ok := ss[0].(*ast.ScopeRes)
	if !ok {
		t.Fatalf("ScopeRes wanted, got %T", ss[0])
	}
	if len(sr.Binds) != 1 {
		t.Fatalf("one binding wanted, got %+v", sr.Binds)
	}
	b := sr.Binds[0]
	if b.Name != "f" {
		t.Fatalf("binding f wanted, got %q", b.Name)
	}
	if c, ok := b.Val.(*ast.Call); !ok {
		t.Fatalf("head call wanted, got %#v", b.Val)
	} else if id, ok := c.Fn.(*ast.Ident); !ok || id.Name != "openFile" {
		t.Fatalf("head openFile() wanted, got %#v", c.Fn)
	}
	if len(sr.Body.Items) != 1 {
		t.Fatalf("one-item body wanted, got %d", len(sr.Body.Items))
	}

	// Multiple bindings fold comma-separated; the binding carries no
	// annotation slot (the head expression's type is the binding's type).
	ms := stmtsOf(t, "scope resource(a = one(), b = two()) {\n        let _ = a\n    }\n")
	mr := ms[0].(*ast.ScopeRes)
	if len(mr.Binds) != 2 || mr.Binds[1].Name != "b" {
		t.Fatalf("two bindings [a b] wanted, got %+v", mr.Binds)
	}

	// A binding name is a block binding: camelCase (E0012) and duplicate
	// names (E0404 block namespace) keep their existing codes.
	wantDiag(t, "fn f() {\n    scope resource(Bad = openFile()) {\n        let _ = 1\n    }\n}\n",
		"E0012", `"Bad" is not camelCase`, 2, 20)
	wantDiag(t, "fn f() {\n    scope resource(f = one(), f = two()) {\n        let _ = 1\n    }\n}\n",
		"E0404", "the scope resource head binds \"f\" twice", 2, 31)

	// Value position keeps the E0202 family (golden e0202-scope-value).
	wantDiag(t, "fn f() {\n    let x = scope resource(y = 1) {\n        return\n    }\n}\n",
		"E0202", `"scope" produces no value and cannot stand in a let initializer`, 2, 13)

	// The binding list is a paren region: a construction head opens its
	// braces there (golden e1103-gc parses; the checker owns the head
	// judgment).
	wantClean(t, "fn f() {\n    scope resource(u = User { name: \"n\" }) {\n        let _ = u\n    }\n}\n")

	// A defer inside the scope block is not at the function body's top
	// level: E0204's existing placement rule (golden e0204-defer-in-scope).
	wantDiag(t, "fn use() {\n    scope resource(f = openFile()) {\n        defer {\n            let n = 1\n        }\n    }\n}\n",
		"E0204", "defer is not a direct item of the function body's block", 3, 9)
}

// --- D1: the bndScope split ---------------------------------------------------

func TestScopeSplit(t *testing.T) {
	// A bare `scope` or any non-resource scope form is chapter 18's: it
	// keeps its own honest boundary row after the split.
	wantBnd(t, "fn f() {\n    scope {\n        let _ = 1\n    }\n}\n",
		"chapter 18 (scope) forms")
	wantBnd(t, "fn f() {\n    scope timeout(5) {\n        let _ = 1\n    }\n}\n",
		"chapter 18 (scope) forms")
	// `scope` must be followed literally by `resource` + binding list; a
	// resource head without parentheses is not a production.
	wantDiag(t, "fn f() {\n    scope resource {\n        let _ = 1\n    }\n}\n",
		"E0105", `"(" opens the scope resource binding list`, 2, 20)
}

// --- D1: the `?` propagation suffix --------------------------------------------

func TestPropSuffix(t *testing.T) {
	// `g()?` is a true postfix node anchored at the `?` token, level 1 in
	// the postfix chain — same slot as call and field access.
	e := exprOf(t, "g()?")
	p, ok := e.(*ast.Prop)
	if !ok {
		t.Fatalf("Prop wanted, got %T", e)
	}
	if c, ok := p.X.(*ast.Call); !ok {
		t.Fatalf("call operand wanted, got %#v", p.X)
	} else if id, ok := c.Fn.(*ast.Ident); !ok || id.Name != "g" {
		t.Fatalf("operand g() wanted, got %#v", c.Fn)
	}

	// Chains left-associate through the postfix loop: `f()?.name` reads
	// the field on the unwrapped Ok payload.
	f := wantClean(t, "fn probe() {\n    let r = g()?.name\n}\n")
	init := f.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.Binding).Init
	fld, ok := init.(*ast.Member)
	if !ok {
		t.Fatalf("Member wanted, got %T", init)
	}
	if _, ok := fld.Recv.(*ast.Prop); !ok {
		t.Fatalf("Prop under the member access wanted, got %#v", fld.Recv)
	}

	// Statement position takes the suffix too (value discarded).
	ss := stmtsOf(t, "g()?\n")
	if _, ok := ss[0].(*ast.ExprStmt).Expr.(*ast.Prop); !ok {
		t.Fatalf("statement-position Prop wanted, got %#v", ss[0])
	}

	// `!` keeps its permanent rejection (golden e0105 lock).
	wantDiag(t, "fn g() -> Int64 {\n    return 1\n}\n\nfn f() -> Int64 {\n    let x = g()!\n    return 1\n}\n",
		"E0105", `"!" after a complete statement`, 6, 16)
}
