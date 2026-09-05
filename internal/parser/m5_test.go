package parser

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M5 (control-and-composites) form tests: chapter 3 statements, chapter 4
// match arms and the pattern grammar, chapter 8 declarations, construction
// and update expressions, chapter 12 closures. Tree shapes pin design D1's
// node inventory; diagnostics pin D2–D5's parse-layer readings.

// --- D2: chapter 3 statements and value-position rules -----------------------

func TestCh3Statements(t *testing.T) {
	stmts := stmtsOf(t, "while flag {\n        n = n + 1\n    }\n")
	w, ok := stmts[0].(*ast.While)
	if !ok {
		t.Fatalf("while wanted, got %T", stmts[0])
	}
	if _, ok := w.Cond.(*ast.Ident); !ok {
		t.Fatalf("while cond: got %T", w.Cond)
	}
	if len(w.Body.Items) != 1 {
		t.Fatalf("while body: %d items", len(w.Body.Items))
	}

	stmts = stmtsOf(t, "loop {\n        break\n    }\n")
	if _, ok := stmts[0].(*ast.Loop); !ok {
		t.Fatalf("loop wanted, got %T", stmts[0])
	}
	if _, ok := stmts[0].(*ast.Loop).Body.Items[0].(*ast.Break); !ok {
		t.Fatalf("break wanted, got %T", stmts[0].(*ast.Loop).Body.Items[0])
	}

	stmts = stmtsOf(t, "loop {\n        break\n        continue\n    }\n")
	l, ok := stmts[0].(*ast.Loop)
	if !ok {
		t.Fatalf("loop wanted, got %T", stmts[0])
	}
	if _, ok := l.Body.Items[0].(*ast.Break); !ok {
		t.Fatalf("break: got %T", l.Body.Items[0])
	}
	if _, ok := l.Body.Items[1].(*ast.Continue); !ok {
		t.Fatalf("continue: got %T", l.Body.Items[1])
	}

	stmts = stmtsOf(t, "defer { cleanup() }\n")
	d, ok := stmts[0].(*ast.Defer)
	if !ok {
		t.Fatalf("defer wanted, got %T", stmts[0])
	}
	if len(d.Block.Items) != 1 {
		t.Fatalf("defer block: %d items", len(d.Block.Items))
	}

	// if, match, and closures are expressions: statement position wraps
	// them in ExprStmt (chapter 2's keyword-led expression forms).
	stmts = stmtsOf(t, "if flag { () }\n")
	es, ok := stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expression statement wanted, got %T", stmts[0])
	}
	if _, ok := es.Expr.(*ast.If); !ok {
		t.Fatalf("if expression wanted, got %T", es.Expr)
	}
}

func TestCh3IfTree(t *testing.T) {
	e := exprOf(t, "if a { 1 } else { 2 }")
	x, ok := e.(*ast.If)
	if !ok {
		t.Fatalf("if wanted, got %T", e)
	}
	if x.Else == nil {
		t.Fatalf("else arm wanted")
	}
	if _, ok := x.Else.(*ast.BlockExpr); !ok {
		t.Fatalf("block else: got %T", x.Else)
	}
	// else-if is a nested if in the else position (chapter 3).
	e = exprOf(t, "if a { 1 } else if b { 2 } else { 3 }")
	x = e.(*ast.If)
	nested, ok := x.Else.(*ast.If)
	if !ok {
		t.Fatalf("nested if in else: got %T", x.Else)
	}
	if _, ok := nested.Else.(*ast.BlockExpr); !ok {
		t.Fatalf("inner else block: got %T", nested.Else)
	}
}

func TestCh3ValuePosition(t *testing.T) {
	wantDiag(t, "fn f(flag: Bool) -> Int64 {\n    let x = if flag { 1 }\n    return x\n}\n",
		"E0202", "an if without else", 2, 13)
	wantDiag(t, "fn f() -> Int64 {\n    let x = while true { break }\n    return x\n}\n",
		"E0202", `"while" produces no value`, 2, 13)
	wantDiag(t, "fn f() -> Int64 {\n    let x = loop { break }\n    return x\n}\n",
		"E0202", `"loop" produces no value`, 2, 13)
	wantDiag(t, "fn f() -> Int64 {\n    let x = break\n    return x\n}\n",
		"E0202", `"break" produces no value`, 2, 13)
	wantDiag(t, "fn f() {\n    g(defer { () })\n}\n",
		"E0202", `"defer" produces no value`, 2, 7)
}

func TestCh3LoopDepth(t *testing.T) {
	wantClean(t, "fn f() {\n    while true {\n        break\n    }\n}\n")
	wantClean(t, "fn f() {\n    loop {\n        while true {\n            continue\n        }\n        break\n    }\n}\n")
	wantDiag(t, "fn f() {\n    break\n}\n", "E0201", `"break" stands in a block`, 2, 5)
	wantDiag(t, "fn f() {\n    continue\n}\n", "E0201", `"continue" stands in a block`, 2, 5)
}

func TestDeferPlacement(t *testing.T) {
	wantDiag(t, "fn cleanup() {\n}\n\nfn f() {\n    defer cleanup()\n}\n",
		"E0203", "the operand is an expression", 5, 5)
	wantDiag(t, "fn f() {\n    if true {\n        defer { () }\n    }\n}\n",
		"E0204", "not a direct item of the function body's block", 3, 9)
	wantDiag(t, "fn f() {\n    while true {\n        defer { () }\n        break\n    }\n}\n",
		"E0204", "not a direct item of the function body's block", 3, 9)
	wantClean(t, "fn f() {\n    defer { () }\n}\n")
	// The defer body is not a function context: return inside is E0401
	// through the existing empty context stack (chapter 3 pins this).
	wantDiag(t, "fn f() {\n    defer { return }\n}\n",
		"E0401", "return outside a function body", 2, 13)
}

// --- D3: match arms and the pattern grammar ----------------------------------

func TestMatchArms(t *testing.T) {
	e := exprOf(t, "match s {\n        Circle(r) => r\n        _ => 0.0\n    }")
	m, ok := e.(*ast.Match)
	if !ok {
		t.Fatalf("match wanted, got %T", e)
	}
	if _, ok := m.Scrutinee.(*ast.Ident); !ok {
		t.Fatalf("scrutinee: got %T", m.Scrutinee)
	}
	if len(m.Arms) != 2 {
		t.Fatalf("two arms wanted, got %d", len(m.Arms))
	}
	vp, ok := m.Arms[0].Pat.(*ast.PatVariant)
	if !ok || vp.Name != "Circle" || len(vp.Args) != 1 {
		t.Fatalf("arm 0 pattern: %+v", m.Arms[0].Pat)
	}
	if m.Arms[0].Guard != nil {
		t.Fatalf("arm 0 unguarded")
	}
	// An arm body is one expression; a block body arrives as BlockExpr.
	if _, ok := m.Arms[0].Body.(*ast.Ident); !ok {
		t.Fatalf("arm 0 body: got %T", m.Arms[0].Body)
	}
	if _, ok := m.Arms[1].Pat.(*ast.PatWildcard); !ok {
		t.Fatalf("arm 1 wildcard: got %T", m.Arms[1].Pat)
	}

	e = exprOf(t, "match s {\n        Circle(r) if r > 1.0 => r\n        _ => { 0.0 }\n    }")
	m = e.(*ast.Match)
	if _, ok := m.Arms[0].Guard.(*ast.Binary); !ok {
		t.Fatalf("guard: got %T", m.Arms[0].Guard)
	}
	if _, ok := m.Arms[1].Body.(*ast.BlockExpr); !ok {
		t.Fatalf("block body: got %T", m.Arms[1].Body)
	}

	wantDiag(t, "fn f(c: Bool) -> Int64 {\n    return match c { }\n}\n",
		"E0301", "the brace group holds no arms", 2, 20)
	wantDiag(t, "fn f(c: Bool) -> Int64 {\n    return match c {\n        true => 1,\n        false => 2\n    }\n}\n",
		"E0105", "match arms are separated by newlines", 3, 18)
}

func patOf(t *testing.T, src string) ast.Pattern {
	t.Helper()
	f := wantClean(t, "fn p(s: Shape) {\n    let _ = match s {\n        "+src+" => ()\n        _ => ()\n    }\n}\n")
	body := f.Items[0].(*ast.FnDecl).Body
	b := body.Items[0].(*ast.Binding)
	m := b.Init.(*ast.Match)
	return m.Arms[0].Pat
}

func TestPatternGrammar(t *testing.T) {
	if p, ok := patOf(t, "1").(*ast.PatLiteral); !ok || p.Text != "1" {
		t.Fatalf("literal pattern: %+v", patOf(t, "1"))
	}
	if _, ok := patOf(t, `_`).(*ast.PatWildcard); !ok {
		t.Fatalf("wildcard pattern")
	}
	if p, ok := patOf(t, "w").(*ast.PatBinding); !ok || p.Name != "w" {
		t.Fatalf("binding pattern: %+v", patOf(t, "w"))
	}
	// Or-patterns collect flat: one node, three branches (wildcards bind
	// nothing, so the name sets agree).
	po, ok := patOf(t, "_ | _ | _").(*ast.PatOr)
	if !ok || len(po.Branches) != 3 {
		t.Fatalf("flat or-pattern: %+v", patOf(t, "_ | _ | _"))
	}
	// Tuple patterns nest recursively; a guard after an or-pattern covers
	// the whole chain.
	pt, ok := patOf(t, "(Hit(a), (b, c))").(*ast.PatTuple)
	if !ok || len(pt.Elems) != 2 {
		t.Fatalf("tuple pattern: %+v", patOf(t, "(Hit(a), (b, c))"))
	}
	if _, ok := pt.Elems[0].(*ast.PatVariant); !ok {
		t.Fatalf("nested variant: got %T", pt.Elems[0])
	}
	if _, ok := pt.Elems[1].(*ast.PatTuple); !ok {
		t.Fatalf("nested tuple: got %T", pt.Elems[1])
	}
	// Qualified variant patterns parse; their typing is multi-module
	// territory (the boundary fires later, at name resolution).
	pv, ok := patOf(t, "shapes.Circle(r)").(*ast.PatVariant)
	if !ok || !pv.Qualified || pv.Name != "Circle" {
		t.Fatalf("qualified variant: %+v", patOf(t, "shapes.Circle(r)"))
	}
}

func TestPatternRejections(t *testing.T) {
	probe := func(pat string, code, part string, col int) {
		t.Helper()
		wantDiag(t, "fn p(s: Shape) {\n    let _ = match s {\n        "+pat+" => ()\n        _ => ()\n    }\n}\n",
			code, part, 3, col)
	}
	probe(`()`, "E0105", "no unit pattern", 9)
	probe(`-1`, "E0105", "carry no sign", 9)
	probe(`(a)`, "E0105", "fit", 9)
	probe(`1 2`, "E0105", "fit", 11)
	// The syntactic name-set checks (chapter 4).
	probe(`a | b`, "E0302", `bind different names ("a" and "b")`, 11)
	probe(`Hit(a) | Miss(b)`, "E0302", "bind different names", 16)
	probe(`(a, a)`, "E0404", "one pattern binds", 13)
}

func TestLetPatternPosition(t *testing.T) {
	// Irrefutable tuple patterns of bindings and wildcards bind.
	f := wantClean(t, "fn f(t: (Int64, Int64)) -> Int64 {\n    let (a, _) = t\n    return a\n}\n")
	b := f.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.Binding)
	pt, ok := b.Pat.(*ast.PatTuple)
	if !ok || len(pt.Elems) != 2 {
		t.Fatalf("let tuple pattern: %+v", b.Pat)
	}
	wantClean(t, "fn f(t: (Int64, (Int64, Int64))) -> Int64 {\n    let (a, (_, b)) = t\n    return b\n}\n")
	// Refutable patterns are match-only.
	wantDiag(t, "fn f(s: Shape) -> Float64 {\n    let Circle(r) = s\n    return r\n}\n",
		"E0105", "refutable patterns are match-only", 2, 9)
	wantDiag(t, "fn f(n: Int64) -> Int64 {\n    let 1 = n\n    return n\n}\n",
		"E0105", "fit", 2, 9)
}

// --- D4: chapter 8 declarations, construction, update, tuples -----------------

func TestCh8Decls(t *testing.T) {
	f := wantClean(t, "record User { name: String, age: Int64, }\n")
	rd, ok := f.Items[0].(*ast.RecordDecl)
	if !ok || rd.Cat != "gc" || rd.Name != "User" || rd.Pub {
		t.Fatalf("record decl: %+v", f.Items[0])
	}
	if len(rd.Fields) != 2 || rd.Fields[0].Name != "name" {
		t.Fatalf("record fields: %+v", rd.Fields)
	}
	f = wantClean(t, "pub byval record P { x: Int64\n    y: Int64 }\n")
	rd = f.Items[0].(*ast.RecordDecl)
	if !rd.Pub || rd.Cat != "value" {
		t.Fatalf("byval record: %+v", rd)
	}
	f = wantClean(t, "byres record Conn { host: String }\n")
	if rd = f.Items[0].(*ast.RecordDecl); rd.Cat != "resource" {
		t.Fatalf("byres record: %+v", rd)
	}
	f = wantClean(t, "record E { }\n")
	if rd = f.Items[0].(*ast.RecordDecl); len(rd.Fields) != 0 {
		t.Fatalf("zero-field record: %+v", rd)
	}
	f = wantClean(t, "newtype UserId(Int64)\n")
	nd, ok := f.Items[0].(*ast.NewtypeDecl)
	if !ok || nd.Name != "UserId" {
		t.Fatalf("newtype decl: %+v", f.Items[0])
	}
	f = wantClean(t, "pub newtype Email(String)\n")
	if nd = f.Items[0].(*ast.NewtypeDecl); !nd.Pub {
		t.Fatalf("pub newtype: %+v", nd)
	}
}

func TestConstructUpdate(t *testing.T) {
	e := exprOf(t, "User { name: \"a\", age: 1 }")
	c, ok := e.(*ast.Construct)
	if !ok || c.Qual != "" || c.Name != "User" || c.Base != nil {
		t.Fatalf("construct: %+v", e)
	}
	if len(c.Fields) != 2 || c.Fields[1].Name != "age" {
		t.Fatalf("construct fields: %+v", c.Fields)
	}
	// A dotted head is a named type reference (module.Name).
	e = exprOf(t, "shapes.User { name: \"a\" }")
	c = e.(*ast.Construct)
	if c.Qual != "shapes" || c.Name != "User" {
		t.Fatalf("dotted head: %+v", c)
	}
	// Update: `with &base` carries one postfix expression.
	e = exprOf(t, "User { age: 2 with &u }")
	c = e.(*ast.Construct)
	if c.Base == nil {
		t.Fatalf("update base missing")
	}
	if _, ok := c.Base.(*ast.Ident); !ok {
		t.Fatalf("update base: got %T", c.Base)
	}
	// Construction braces are brackets: field initializers fold across
	// lines without continuation tokens.
	wantClean(t, "fn f() -> Int64 {\n    let u = User {\n        name: \"a\",\n        age: 1\n    }\n    return u.age\n}\n")
	wantDiag(t, "fn f() {\n    let u = User { age: 1 with u }\n}\n",
		"E0105", "fit", 2, 32)
}

func TestTupleExprs(t *testing.T) {
	e := exprOf(t, "(a, b)")
	tu, ok := e.(*ast.Tuple)
	if !ok || len(tu.Elems) != 2 {
		t.Fatalf("tuple: %+v", e)
	}
	// (e) stays grouping — it folds to the operand, never a 1-tuple.
	if got := exprStr(exprOf(t, "(a)")); got != "a" {
		t.Fatalf("grouping folds: %s", got)
	}
	if _, ok := exprOf(t, "()").(*ast.Unit); !ok {
		t.Fatalf("unit value")
	}
	// Nine elements exceed the arity bound at the two registry-named
	// sites: the type slot and the expression.
	wantDiag(t, "fn f() {\n    let t = (1, 2, 3, 4, 5, 6, 7, 8, 9)\n}\n",
		"E0602", "the tuple expression has 9 elements", 2, 13)
	wantDiag(t, "fn f(t: (Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64)) {\n}\n",
		"E0602", "the tuple type has 9 elements", 1, 9)
	// A >8-element PATTERN parses clean: the registry description covers
	// only types and expressions, so the rejection is E0501 at the types
	// stage (design D3's erratum).
	wantClean(t, "fn f(n: Int64) {\n    let _ = match n {\n        (a, b, c, d, e, f, g, h, i) => ()\n        _ => ()\n    }\n}\n")
}

// --- D5: chapter 12 closures --------------------------------------------------

func TestClosures(t *testing.T) {
	e := exprOf(t, "fn(a: Int64, b: Int64) -> Int64 { return a + b }")
	c, ok := e.(*ast.Closure)
	if !ok || c.Short || len(c.Params) != 2 || c.Ret == nil {
		t.Fatalf("full closure: %+v", e)
	}
	if len(c.Body.Items) != 1 {
		t.Fatalf("closure body: %d items", len(c.Body.Items))
	}
	// The full form without a return type declares valuelessness.
	e = exprOf(t, "fn() { }")
	if c = e.(*ast.Closure); c.Ret != nil {
		t.Fatalf("valueless closure: %+v", c)
	}

	e = exprOf(t, "|x| x + 1")
	c, ok = e.(*ast.Closure)
	if !ok || !c.Short || len(c.Params) != 1 || c.Params[0].Type != nil {
		t.Fatalf("short closure: %+v", e)
	}
	// The body is maximal: postfixes and binary operators fold into it.
	e = exprOf(t, "|x| x + 1")
	if b, ok := e.(*ast.Closure).Body.Items[0].(*ast.ExprStmt).Expr.(*ast.Binary); !ok || b.Op != "+" {
		t.Fatalf("maximal body: got %T", e.(*ast.Closure).Body.Items[0])
	}
	// Annotated short-form parameters.
	e = exprOf(t, "|x: Int64, y: Int64| x")
	c = e.(*ast.Closure)
	if c.Params[1].Type == nil {
		t.Fatalf("annotated short params: %+v", c.Params)
	}
	// A short closure takes no postfix: calling one needs parentheses.
	e = exprOf(t, "(|x| x + 1)(2)")
	call, ok := e.(*ast.Call)
	if !ok {
		t.Fatalf("call through parens: got %T", e)
	}
	if _, ok := call.Fn.(*ast.Closure); !ok {
		t.Fatalf("parenthesized closure callee: got %T", call.Fn)
	}

	// Zero-parameter closures exist only in the full form: || is the
	// logical-or token by maximal munch (chapter 12 pins the report).
	wantDiag(t, "fn f() -> Int64 {\n    let g = || 1\n    return 1\n}\n",
		"E0105", "no zero-parameter short closure", 2, 13)
	// Operand position: | is unreachable in the binary loop's operand
	// slot, so the parenthesized form is the only spelling.
	wantDiag(t, "fn f() -> Int64 {\n    let n = 1 + |x| x\n    return n\n}\n",
		"E0105", "must be parenthesized", 2, 17)
	// Statement-initial | stays chapter 2's continuation-token rule.
	wantDiag(t, "fn f() {\n    |x| x\n}\n",
		"E0102", `cannot begin a statement`, 2, 5)
}

func TestClosureFunctionContext(t *testing.T) {
	// A closure body is a function-body context: return, defer, and the
	// loop-depth reset all hold inside it.
	wantClean(t, "fn f() -> Int64 {\n    let g = fn(c: Bool) -> Int64 { return if c { 1 } else { 2 } }\n    return g(true)\n}\n")
	wantDiag(t, "fn f() {\n    let g = fn() { return 1 }\n}\n",
		"E0402", "declares no return type", 2, 20)
	// The loop-depth reset: a closure body is a function body, not the
	// loop body it lexically sits in.
	wantDiag(t, "fn f() {\n    while true {\n        let g = |x: Int64| break\n    }\n}\n",
		"E0201", `"break" stands in a block`, 3, 28)
	wantClean(t, "fn f() {\n    let g = |x: Int64| {\n        while true {\n            break\n        }\n        x\n    }\n}\n")
	// And defer is legal at a closure body's top level.
	wantClean(t, "fn f() {\n    let g = |x: Int64| {\n        defer { () }\n        x\n    }\n}\n")
}
