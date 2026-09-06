package parser

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/diag"
)

// --- helpers ---------------------------------------------------------------

func runParse(t *testing.T, src string) (*ast.File, *diag.Diagnostic, *NotImplemented) {
	t.Helper()
	return Parse("test.we", []byte(src))
}

// wantClean asserts a clean parse and returns the tree.
func wantClean(t *testing.T, src string) *ast.File {
	t.Helper()
	f, d, ni := runParse(t, src)
	if d != nil {
		t.Fatalf("unexpected diagnostic: %s", d.Human())
	}
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if f == nil {
		t.Fatalf("no tree for %q", src)
	}
	return f
}

// wantDiag asserts exactly one diagnostic with code, message fragment, and
// position (checked through the JSON rendering, whose field order the
// protocol fixes), and that no tree is produced.
func wantDiag(t *testing.T, src, code, part string, line, col int) *diag.Diagnostic {
	t.Helper()
	f, d, ni := runParse(t, src)
	if ni != nil {
		t.Fatalf("expected %s, got boundary %q", code, ni.What)
	}
	if d == nil {
		t.Fatalf("expected %s, got a clean parse: %q -> %+v", code, src, f)
	}
	if f != nil {
		t.Fatalf("a rejected input must produce no tree, got %+v", f)
	}
	if d.Code() != code {
		t.Fatalf("expected code %s, got %s (%s)", code, d.Code(), d.Message())
	}
	if !strings.Contains(d.Message(), part) {
		t.Fatalf("message %q missing %q", d.Message(), part)
	}
	pos := `"line":` + itoa(line) + `,"column":` + itoa(col)
	if !strings.Contains(d.JSON(), pos) {
		t.Fatalf("expected position %d:%d, got %s", line, col, d.JSON())
	}
	return d
}

// wantBnd asserts the NotImplemented boundary with its exact form-group
// string (design D6's closed table).
func wantBnd(t *testing.T, src, what string) {
	t.Helper()
	f, d, ni := runParse(t, src)
	if d != nil {
		t.Fatalf("expected boundary %q, got diagnostic: %s", what, d.Human())
	}
	if ni == nil {
		t.Fatalf("expected boundary %q, got a clean parse: %q -> %+v", what, src, f)
	}
	if ni.What != what {
		t.Fatalf("boundary What %q, want %q", ni.What, what)
	}
	if f != nil {
		t.Fatalf("a boundary must produce no tree, got %+v", f)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// exprOf wraps expr in `let r = expr` inside a fn body and returns the
// initializer expression.
func exprOf(t *testing.T, expr string) ast.Expr {
	t.Helper()
	f := wantClean(t, "fn probe() {\n    let r = "+expr+"\n}\n")
	body := f.Items[0].(*ast.FnDecl).Body
	if len(body.Items) != 1 {
		t.Fatalf("one statement wanted, got %d", len(body.Items))
	}
	b, ok := body.Items[0].(*ast.Binding)
	if !ok {
		t.Fatalf("binding wanted, got %T", body.Items[0])
	}
	return b.Init
}

// stmtOf wraps src lines in a fn body and returns the parsed statements.
func stmtsOf(t *testing.T, lines string) []ast.Stmt {
	t.Helper()
	f := wantClean(t, "fn probe() {\n"+lines+"}\n")
	return f.Items[0].(*ast.FnDecl).Body.Items
}

// exprStr renders an expression canonically — grouping parentheses fold
// away, so the string pins the tree shape the precedence table produces.
func exprStr(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.Literal:
		return x.Text
	case *ast.Unary:
		return "(" + x.Op + exprStr(x.X) + ")"
	case *ast.Binary:
		return "(" + exprStr(x.L) + x.Op + exprStr(x.R) + ")"
	case *ast.Call:
		args := make([]string, len(x.Args))
		for i, a := range x.Args {
			args[i] = exprStr(a)
		}
		return exprStr(x.Fn) + "(" + strings.Join(args, ",") + ")"
	case *ast.Member:
		return exprStr(x.Recv) + "." + x.Name
	case *ast.Unit:
		return "()"
	case *ast.BlockExpr:
		return "{...}"
	}
	return "?"
}

// typStr renders a type reference canonically.
func typStr(ty ast.TypeRef) string {
	switch x := ty.(type) {
	case *ast.NamedType:
		s := x.Name
		if x.Qual != "" {
			s = x.Qual + "." + s
		}
		if len(x.Args) > 0 {
			args := make([]string, len(x.Args))
			for i, a := range x.Args {
				args[i] = typStr(a)
			}
			s += "<" + strings.Join(args, ",") + ">"
		}
		return s
	case *ast.TupleType:
		elems := make([]string, len(x.Elems))
		for i, e := range x.Elems {
			elems[i] = typStr(e)
		}
		return "(" + strings.Join(elems, ",") + ")"
	case *ast.UnitType:
		return "()"
	case *ast.FnType:
		params := make([]string, len(x.Params))
		for i, p := range x.Params {
			params[i] = typStr(p)
		}
		s := "fn(" + strings.Join(params, ",") + ")"
		if len(x.EffectTags) > 0 {
			s += " " + strings.Join(x.EffectTags, " ")
		}
		return s + "->" + typStr(x.Ret)
	}
	return "?"
}

// --- D6: the 41-keyword dispatch table, exhaustive over three positions ---

// outcome is one cell of the dispatch table: clean, a diagnostic
// (code + message fragment, positioned by the wrapper), or a boundary.
type outcome struct {
	clean bool
	code  string
	part  string
	what  string
}

func okRes() outcome               { return outcome{clean: true} }
func dg(code, part string) outcome { return outcome{code: code, part: part} }
func bd(what string) outcome       { return outcome{what: what} }

const (
	topStmtPart = "fits no top-level item production"
	stmtPart    = "fits no statement production"
	exprPart    = "cannot begin an expression"
	scopeForms  = "chapter 13 and 18 (scope) forms"
	effectForms = "chapter 16 (effects) forms"
	concurForms = "chapter 18 (concurrency) forms"
	ffiForms    = "chapter 19 (ffi) forms"
	testForms   = "chapter 20 (testing) forms"
)

// dispatchTable is design D6 verbatim: keyword, its completion text, and
// the three position outcomes (top level / statement / expression).
var dispatchTable = []struct {
	kw      string
	rest    string
	top     outcome
	stmt    outcome
	express outcome
}{
	{"let", "a = 1", okRes(), okRes(), dg("E0105", exprPart)},
	{"var", "v = 1", dg("E0403", "var at the top level"), okRes(), dg("E0105", exprPart)},
	{"pub", "fn f() {}", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"import", "a.b", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"as", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"mut", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"fn", "f() {}", okRes(), dg("E0105", "where a closure's parameter list opens"), dg("E0105", "where a closure's parameter list opens")},
	{"if", "true {}", dg("E0105", topStmtPart), okRes(), dg("E0202", "an if without else")},
	{"else", "", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"return", "", dg("E0401", "return outside a function body"), okRes(), dg("E0105", exprPart)},
	{"match", "x { _ => 1 }", dg("E0105", topStmtPart), okRes(), okRes()},
	{"for", "x in y {}", dg("E0105", topStmtPart), okRes(), dg("E0202", `"for" produces no value`)},
	{"in", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"while", "true {}", dg("E0105", topStmtPart), okRes(), dg("E0202", `"while" produces no value`)},
	{"loop", "{}", dg("E0105", topStmtPart), okRes(), dg("E0202", `"loop" produces no value`)},
	{"break", "", dg("E0105", topStmtPart), dg("E0201", `"break" stands in a block`), dg("E0202", `"break" produces no value`)},
	{"continue", "", dg("E0105", topStmtPart), dg("E0201", `"continue" stands in a block`), dg("E0202", `"continue" produces no value`)},
	{"defer", "{ () }", dg("E0105", topStmtPart), okRes(), dg("E0202", `"defer" produces no value`)},
	{"true", "", dg("E0105", topStmtPart), okRes(), okRes()},
	{"false", "", dg("E0105", topStmtPart), okRes(), okRes()},
	{"foreign", "", bd(ffiForms), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"record", "User {}", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"byval", "record B {}", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"byres", "record B {}", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"newtype", "N(Int64)", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"with", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"type", "T = Int64", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"interface", "I {}", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"impl", "I {}", okRes(), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"where", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"derives", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"scope", "x {}", dg("E0105", topStmtPart), bd(scopeForms), bd(scopeForms)},
	{"resource", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"effect", "io {}", bd(effectForms), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"task", "f() {}", dg("E0105", topStmtPart), bd(concurForms), bd(concurForms)},
	{"select", "{}", dg("E0105", topStmtPart), bd(concurForms), bd(concurForms)},
	{"case", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"timeout", "1", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"collectAll", "x", dg("E0105", topStmtPart), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"test", `"t" {}`, bd(testForms), dg("E0105", stmtPart), dg("E0105", exprPart)},
	{"mock", "m {}", dg("E0105", topStmtPart), bd(testForms), dg("E0105", exprPart)},
}

// TestKeywordDispatchExhaustive runs every one of chapter 1's 41 keywords
// through the three positions of design D6's table and asserts the pinned
// outcome for each cell.
func TestKeywordDispatchExhaustive(t *testing.T) {
	if len(dispatchTable) != 41 {
		t.Fatalf("dispatch table holds %d rows, chapter 1 enumerates 41 keywords", len(dispatchTable))
	}
	seen := map[string]bool{}
	for _, row := range dispatchTable {
		if seen[row.kw] {
			t.Fatalf("keyword %q listed twice", row.kw)
		}
		seen[row.kw] = true

		check := func(where, src string, want outcome, line, col int) {
			t.Helper()
			f, d, ni := runParse(t, src)
			switch {
			case want.clean:
				if d != nil || ni != nil || f == nil {
					t.Fatalf("%s %q: want clean, got d=%v ni=%+v", where, row.kw, d, ni)
				}
			case want.what != "":
				if d != nil {
					t.Fatalf("%s %q: want boundary %q, got %s", where, row.kw, want.what, d.Human())
				}
				if ni == nil || ni.What != want.what {
					t.Fatalf("%s %q: want boundary %q, got %+v", where, row.kw, want.what, ni)
				}
			default:
				if ni != nil {
					t.Fatalf("%s %q: want %s, got boundary %q", where, row.kw, want.code, ni.What)
				}
				if d == nil {
					t.Fatalf("%s %q: want %s, got a clean parse", where, row.kw, want.code)
				}
				if d.Code() != want.code || !strings.Contains(d.Message(), want.part) {
					t.Fatalf("%s %q: want %s (%q), got %s (%s)", where, row.kw, want.code, want.part, d.Code(), d.Message())
				}
				pos := `"line":` + itoa(line) + `,"column":` + itoa(col)
				if !strings.Contains(d.JSON(), pos) {
					t.Fatalf("%s %q: want position %d:%d, got %s", where, row.kw, line, col, d.JSON())
				}
			}
		}
		check("top", row.kw+" "+row.rest+"\n", row.top, 1, 1)
		check("stmt", "fn probe() {\n    "+row.kw+" "+row.rest+"\n}\n", row.stmt, 2, 5)
		check("expr", "fn probe() {\n    let _ = "+row.kw+" "+row.rest+"\n}\n", row.express, 2, 13)
	}
}

// --- D3: line-joining, the two decision points -----------------------------

func TestLineJoining(t *testing.T) {
	// Operator at end of line continues; operand ends the statement.
	if got := exprOf(t, "b +\n        c"); exprStr(got) != "(b+c)" {
		t.Fatalf("trailing +: got %s", exprStr(got))
	}
	wantDiag(t, "fn f() {\n    let a = b\n        * c\n}\n",
		"E0102", `cannot begin a statement`, 3, 9)
	wantDiag(t, "fn f() {\n    let ok = a < b < c\n}\n",
		"E0104", "non-associative", 2, 20)
	// Trailing dot continues (chapter 2's own svc example); leading dot
	// does not — the boundary fires after the statement ends.
	if got := exprOf(t, "svc.\n        query()"); exprStr(got) != "svc.query()" {
		t.Fatalf("trailing dot: got %s", exprStr(got))
	}
	wantDiag(t, "fn f() {\n    svc\n        .query()\n}\n",
		"E0102", `ends at line 2`, 3, 9)
	// Inside parentheses line breaks are insignificant, and trailing
	// commas are uniform across comma lists.
	if got := exprOf(t, "f(\n        a,\n        b,\n    )"); exprStr(got) != "f(a,b)" {
		t.Fatalf("parenthesized call: got %s", exprStr(got))
	}
	// One statement per line at depth zero.
	if got := stmtsOf(t, "    let a = 1\n    let b = 2\n"); len(got) != 2 {
		t.Fatalf("two statements wanted, got %d", len(got))
	}
	// Same-line residue after a complete statement is E0105.
	wantDiag(t, "fn f() {\n    let a = 1 2\n}\n",
		"E0105", "after a complete statement", 2, 15)
	// An assignment needs the = on the same line as its name.
	wantDiag(t, "fn f() {\n    x\n        = 1\n}\n",
		"E0102", `ends at line 2`, 3, 9)
	// The statement-start class is closed: `[` is not in it (a list
	// literal is a primary expression, reached in expression position).
	wantDiag(t, "fn f() {\n    [1, 2]\n}\n",
		"E0102", "continuation token", 2, 5)
	// In expression position the brackets are chapter 17's list literal.
	lf := wantClean(t, "fn f() {\n    let xs = [1, 2]\n}\n")
	if ll, ok := lf.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.Binding).Init.(*ast.ListLit); !ok || len(ll.Elems) != 2 {
		t.Fatalf("two-element list literal wanted, got %#v", lf.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.Binding).Init)
	}
	// Continuation token with no preceding statement anywhere in the file.
	wantDiag(t, "+ 1\n", "E0102", "no statement precedes it", 1, 1)
}

// --- D5: precedence and associativity ---------------------------------------

func TestPrecedence(t *testing.T) {
	cases := []struct{ expr, want string }{
		{"a + b * c", "(a+(b*c))"},
		{"a * b + c", "((a*b)+c)"},
		{"a - b - c", "((a-b)-c)"},
		{"a << b << c", "((a<<b)<<c)"},
		{"a & b | c ^ d", "((a&b)|(c^d))"},
		{"a == b || c && d", "((a==b)||(c&&d))"},
		{"a + b .. c", "((a+b)..c)"},
		{"!a && b", "((!a)&&b)"},
		{"-a + b", "((-a)+b)"},
		{"- -a", "(-(-a))"},
		{"-a.f", "(-a.f)"},
		{"f(a, b)(c)", "f(a,b)(c)"},
		{"a.b.c", "a.b.c"},
		{"(a + b) * c", "((a+b)*c)"},
		{"a % b / c", "((a%b)/c)"},
	}
	for _, c := range cases {
		if got := exprStr(exprOf(t, c.expr)); got != c.want {
			t.Errorf("%s: got %s, want %s", c.expr, got, c.want)
		}
	}
	// The non-associative levels: 9 (comparisons/equality) and 12 (`..`).
	wantDiag(t, "fn f() {\n    let r = a == b == c\n}\n", "E0104", "non-associative", 2, 20)
	wantDiag(t, "fn f() {\n    let r = a < b == c\n}\n", "E0104", "non-associative", 2, 19)
	wantDiag(t, "fn f() {\n    let r = a..b..c\n}\n", "E0104", "each range must be a single explicit group", 2, 17)
}

// --- D4: statement families --------------------------------------------------

func TestStatements(t *testing.T) {
	// Binding forms, including the discard and annotations.
	b := stmtsOf(t, "    let a = 1\n    var v: Int64 = 2\n    let _: String = f()\n")
	if len(b) != 3 {
		t.Fatalf("three statements wanted, got %d", len(b))
	}
	b0 := b[0].(*ast.Binding)
	b1 := b[1].(*ast.Binding)
	b2 := b[2].(*ast.Binding)
	if b0.Kw != "let" || b0.Name != "a" || b0.Typ != nil || exprStr(b0.Init) != "1" {
		t.Fatalf("binding 0: %+v", b0)
	}
	if b1.Kw != "var" || b1.Name != "v" || b1.Typ == nil || typStr(b1.Typ) != "Int64" {
		t.Fatalf("binding 1: %+v", b1)
	}
	if b2.Name != "_" || typStr(b2.Typ) != "String" {
		t.Fatalf("binding 2: %+v", b2)
	}
	// Assignment: bare names only, and never an expression.
	a := stmtsOf(t, "    x = y + 1\n")[0].(*ast.Assign)
	if a.Name != "x" || exprStr(a.Value) != "(y+1)" {
		t.Fatalf("assignment: %+v", a)
	}
	wantDiag(t, "fn f() {\n    let x = y = 1\n}\n", "E0103", "assignment is not an expression", 2, 15)
	wantDiag(t, "fn g() {\n    work(a = 1)\n}\n", "E0103", "assignment is not an expression", 2, 12)
	wantDiag(t, "fn set(u: Int64) {\n    u.name = \"bob\"\n}\n", "E0105", "assignment targets are bare names", 2, 12)
	// The one field-write form self.field = expr parses (chapter 10); the
	// receiver placement (mut self) is the checker's E0813.
	sa := stmtsOf(t, "    self.name = \"bob\"\n")[0].(*ast.Assign)
	if sa.Name != "self" || sa.Field != "name" || exprStr(sa.Value) != `"bob"` {
		t.Fatalf("self field write: %+v", sa)
	}
	// return forms and their two diagnostics.
	r := stmtsOf(t, "    return\n")[0].(*ast.Return)
	if r.HasValue {
		t.Fatalf("bare return: %+v", r)
	}
	fv := wantClean(t, "fn probe() -> Int64 {\n    return f()\n}\n")
	r2 := fv.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.Return)
	if !r2.HasValue || exprStr(r2.Value) != "f()" {
		t.Fatalf("valued return: %+v", r2)
	}
	wantDiag(t, "fn noValue() { return 1 }\n", "E0402", `"noValue" declares no return type`, 1, 16)
	wantDiag(t, "return 1\n", "E0401", "return outside a function body", 1, 1)
	wantDiag(t, "let x = {\n    return 1\n}\n", "E0401", "return outside a function body", 2, 5)
	// Top-level var and pub var.
	wantDiag(t, "var count = 0\n", "E0403", "var at the top level", 1, 1)
	wantDiag(t, "pub var v = 1\n", "E0105", "after pub", 1, 5)
	// Binding head violations.
	wantDiag(t, "fn f() {\n    let fn = 1\n}\n", "E0105", "fits no binding name production", 2, 9)
	wantDiag(t, "fn f() {\n    let a 1\n}\n", "E0105", "in a binding head", 2, 11)
	// The bare underscore is punctuation, not in the statement-start class.
	wantDiag(t, "fn f() {\n    _ = 5\n}\n", "E0102", "continuation token", 2, 5)
}

// --- D5: the expression skeleton ---------------------------------------------

func TestExpressions(t *testing.T) {
	cases := []struct{ expr, want string }{
		{"42", "42"}, {"3.14", "3.14"}, {`"s"`, `"s"`}, {"'c'", "'c'"},
		{"true", "true"}, {"false", "false"},
		{"!x", "(!x)"}, {"~m", "(~m)"}, {"!!x", "(!(!x))"},
		{"a.b(c).d(e, f)", "a.b(c).d(e,f)"},
		{"(a)", "a"},
		{"()", "()"},
	}
	for _, c := range cases {
		if got := exprStr(exprOf(t, c.expr)); got != c.want {
			t.Errorf("%s: got %s, want %s", c.expr, got, c.want)
		}
	}
	// `?` is chapter 14's; indexing is ratified never — a diagnostic, not
	// a boundary.
	wantBnd(t, "fn f() {\n    let y = x?\n}\n", "chapter 14 (errors) forms")
	wantDiag(t, "fn first(list: Int64) {\n    let first = list[0]\n}\n",
		"E0105", `postfix is .name or (args)`, 2, 21)
	// Tuple expressions, construction braces (chapter 8), and the two
	// closure forms (chapter 12) parse since M5; their typing lands with
	// the typecheck passes.
	wantClean(t, "fn f() {\n    let t = (a, b)\n}\n")
	wantClean(t, "fn build(id: Int64) -> Int64 {\n    let u = User { id: id }\n    id\n}\n")
	wantClean(t, "fn f() {\n    let u = net.User { }\n}\n")
	wantDiag(t, "fn f() {\n    let u = b { }\n}\n", "E0105", "construction heads are PascalCase", 2, 15)
	wantDiag(t, "fn f() {\n    f() { }\n}\n", "E0105", "construction heads are PascalCase", 2, 9)
	wantClean(t, "fn f() {\n    let g = |x| x\n}\n")
	wantClean(t, "fn f() {\n    let g = fn(x: Int64) { x }\n}\n")
	// `..` has no prefix form.
	wantDiag(t, "fn f() {\n    let r = ..a\n}\n", "E0105", exprPart, 2, 13)
	// A block is an expression whose value is its final expression item.
	f := wantClean(t, "fn f() {\n    let v = {\n        let t = 1\n        t + 1\n    }\n}\n")
	body := f.Items[0].(*ast.FnDecl).Body
	bind := body.Items[0].(*ast.Binding)
	be, ok := bind.Init.(*ast.BlockExpr)
	if !ok || len(be.Block.Items) != 2 {
		t.Fatalf("block expression: %+v", bind.Init)
	}
	if _, isBinding := be.Block.Items[0].(*ast.Binding); !isBinding {
		t.Fatalf("block item 0 must be a binding, got %T", be.Block.Items[0])
	}
	if exprStr(be.Block.Items[1].(*ast.ExprStmt).Expr) != "(t+1)" {
		t.Fatalf("block value must be the final expression")
	}
	// Call lists close; trailing commas allowed.
	if got := exprStr(exprOf(t, "f(a,)")); got != "f(a)" {
		t.Fatalf("trailing comma: got %s", got)
	}
	wantDiag(t, "fn f() {\n    g(1\n}\n", "E0105", "in a call argument list", 3, 1)
	// Member names are identifiers.
	wantDiag(t, "fn f() {\n    a.type\n}\n", "E0105", `member names are identifiers`, 2, 7)
}

// --- D7/D8/D9/D10: declarations ----------------------------------------------

func TestDeclarations(t *testing.T) {
	// Full fn form.
	f := wantClean(t, "fn add(a: Int64, b: Int64,) -> Int64 {\n    return a\n}\n")
	fn := f.Items[0].(*ast.FnDecl)
	if fn.Pub || fn.Name != "add" || len(fn.Params) != 2 || typStr(fn.Ret) != "Int64" || len(fn.Body.Items) != 1 {
		t.Fatalf("fn decl: %+v", fn)
	}
	if fn.Params[1].Name != "b" || typStr(fn.Params[1].Type) != "Int64" {
		t.Fatalf("params: %+v", fn.Params)
	}
	// pub, and valueless fn.
	f = wantClean(t, "pub fn run() {\n    return\n}\n")
	fn = f.Items[0].(*ast.FnDecl)
	if !fn.Pub || fn.Ret != nil {
		t.Fatalf("pub valueless fn: %+v", fn)
	}
	// Signature violations and later-chapter clauses.
	wantDiag(t, "fn bad(x) { }\n", "E0105", `parameter "x" carries no annotation`, 1, 8)
	wantClean(t, "fn f<T>(x: T) { }\n")
	wantBnd(t, "fn f(mut x: Int64) { }\n", "mut parameters")
	wantBnd(t, "fn read(p: String) effect io -> String { return p }\n", effectForms)
	wantDiag(t, "fn f()\n", "E0105", "a fn wants its body block", 2, 1)
	wantDiag(t, "fn f() 1\n", "E0105", "where a fn body block opens", 1, 8)
	// Imports: forms, aliases, and E0013.
	f = wantClean(t, "import std.io\nimport net.http.client as http\n")
	imp0 := f.Items[0].(*ast.Import)
	imp1 := f.Items[1].(*ast.Import)
	if len(imp0.Path) != 2 || imp0.Alias != "" || len(imp1.Path) != 3 || imp1.Alias != "http" {
		t.Fatalf("imports: %+v %+v", imp0, imp1)
	}
	wantDiag(t, "import Models.User\n", "E0013", `"Models" is not a lowercase dotted path segment`, 1, 8)
	wantDiag(t, "import std.io as IO\n", "E0013", `"IO" is not a lowercase dotted path segment`, 1, 18)
	wantDiag(t, "import std.io as\n", "E0105", "an import wants a lowercase dotted path", 2, 1)
	// E0012 over the three name families.
	wantDiag(t, "fn Bad_Name() { }\n", "E0012", `"Bad_Name" is not camelCase`, 1, 4)
	wantDiag(t, "fn f(BadParam: Int64) { }\n", "E0012", `"BadParam" is not camelCase`, 1, 6)
	wantDiag(t, "let BadName = 1\n", "E0012", `"BadName" is not camelCase`, 1, 5)
	wantDiag(t, "fn f() {\n    let _x = 1\n}\n", "E0012", `"_x" is not camelCase`, 2, 9)
	f = wantClean(t, "let camel1 = 1\npub let alsoFine = 2\n")
	if len(f.Items) != 2 {
		t.Fatalf("camelCase bindings must parse: %+v", f.Items)
	}
	// E0404: one module, one name space — the later one is rejected.
	wantDiag(t, "fn f() {\n}\n\nfn f() {\n}\n", "E0404", `"f" is already declared at line 1`, 4, 4)
	wantDiag(t, "import std.io as io\nfn io() { }\n", "E0404", `"io" is already declared at line 1`, 2, 4)
	wantDiag(t, "let a = 1\nfn a() { }\n", "E0404", `"a" is already declared at line 1`, 2, 4)
	// Documentation units attach to the next top-level item; a trailing
	// orphan is E0405 at the unit's first line.
	f = wantClean(t, "/// One.\nfn f() { }\n\n/// Two.\n/// Two more.\nlet a = 1\n")
	if len(f.Docs) != 2 {
		t.Fatalf("two doc units wanted, got %+v", f.Docs)
	}
	if f.Docs[0].Item != 0 || f.Docs[0].StartLine != 1 || f.Docs[0].EndLine != 1 || len(f.Docs[0].Lines) != 1 {
		t.Fatalf("doc 0: %+v", f.Docs[0])
	}
	if f.Docs[1].Item != 1 || f.Docs[1].StartLine != 4 || f.Docs[1].EndLine != 5 || len(f.Docs[1].Lines) != 2 {
		t.Fatalf("doc 1: %+v", f.Docs[1])
	}
	wantDiag(t, "fn f() {\n}\n\n/// Orphan documentation.\n", "E0405", "orphan documentation comment", 4, 1)
	// Statements do not exist at the top level.
	wantDiag(t, "fn run() {\n    compute()\n}\n\ncompute()\n",
		"E0105", topStmtPart, 5, 1)
}

// --- chapter 9: sum declarations (D7) -----------------------------------------

func TestSumDecls(t *testing.T) {
	// Full form: pub and byval prefixes, unit and payloaded variants.
	f := wantClean(t, "pub byval type Shape = Dot | Circle(Float64) | Rect(Float64, Float64)\n")
	d := f.Items[0].(*ast.SumDecl)
	if !d.Pub || !d.Byval || d.Name != "Shape" || len(d.Variants) != 3 {
		t.Fatalf("sum decl: %+v", d)
	}
	if d.Variants[0].Name != "Dot" || len(d.Variants[0].Payload) != 0 {
		t.Fatalf("unit variant: %+v", d.Variants[0])
	}
	if d.Variants[1].Name != "Circle" || len(d.Variants[1].Payload) != 1 ||
		typStr(d.Variants[1].Payload[0]) != "Float64" {
		t.Fatalf("payloaded variant: %+v", d.Variants[1])
	}
	if len(d.Variants[2].Payload) != 2 || typStr(d.Variants[2].Payload[1]) != "Float64" {
		t.Fatalf("two payloads: %+v", d.Variants[2])
	}
	// Positions: the decl anchors at its outermost prefix token, the name
	// and each variant at their own tokens.
	if d.Line != 1 || d.Col != 1 || d.NameCol != 16 || d.Variants[1].Col != 30 {
		t.Fatalf("positions: %+v", d)
	}
	// Bare type, and payload type references reuse the full grammar.
	f = wantClean(t, "type R = Wrap(Result<Int64, AppError>)\n")
	d = f.Items[0].(*ast.SumDecl)
	if d.Pub || d.Byval || typStr(d.Variants[0].Payload[0]) != "Result<Int64,AppError>" {
		t.Fatalf("bare sum: %+v", d)
	}
	// `=` and trailing `|` continue per chapter 2's continuation set; a
	// variant list may span lines that way.
	f = wantClean(t, "type Shape =\n    Dot |\n    Circle(Float64)\n")
	if len(f.Items) != 1 || len(f.Items[0].(*ast.SumDecl).Variants) != 2 {
		t.Fatalf("line continuation: %+v", f.Items)
	}
	// A line-STARTING `|` after a complete variant ends the declaration;
	// checkStart reports it as a statement that cannot begin (E0102).
	wantDiag(t, "type Shape = Dot\n| Circle(Float64)\n",
		"E0102", "cannot begin a statement", 2, 1)
	// `type T = Int64` is a one-variant sum whose variant is named Int64 —
	// legal syntax; the shadowing is chapter 15's to resolve.
	f = wantClean(t, "type T = Int64\n")
	if d := f.Items[0].(*ast.SumDecl); d.Name != "T" || d.Variants[0].Name != "Int64" {
		t.Fatalf("shadowing variant: %+v", d)
	}
	// Naming: sum and variant names are PascalCase (E0011), and all names
	// share the one module name space (E0404 names the earlier line).
	wantDiag(t, "type bad = A | B\n", "E0011", `"bad" is not PascalCase`, 1, 6)
	wantDiag(t, "type Good = ok\n", "E0011", `"ok" is not PascalCase`, 1, 13)
	wantDiag(t, "type Shape = Circle(Float64)\nfn Circle(r: Float64) { }\n",
		"E0404", `"Circle" is already declared at line 1`, 2, 4)
	// Payload arity above eight is E0701 at the variant name.
	wantDiag(t, "type Big = V(Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64)\n",
		"E0701", "variant payload arity above eight", 1, 12)
	// The chapter 10 clauses parse: the generic clause after the name,
	// derives after the last variant on the declaration's last line — and
	// derives takes bare names, so the call-shaped form is E0105 at its (.
	wantClean(t, "type Pair<A, B> = P(A, B)\n")
	wantClean(t, "type Shape = Dot derives Eq\n")
	wantDiag(t, "type Shape = Dot derives(Eq)\n", "E0105", "in a derives clause", 1, 25)
	// The prefix order is fixed: pub outside byval, byval outside type.
	wantDiag(t, "byval pub type B = V\n", "E0105", "after byval", 1, 7)
	// The declaration closes at its line break; a stray token after the
	// last variant is E0105.
	wantDiag(t, "type Shape = Dot Circle\n", "E0105", "after a top-level item", 1, 18)
}

// --- D7/Q3: type references ---------------------------------------------------

func TestTypeRefs(t *testing.T) {
	typeOf := func(t *testing.T, src string) ast.TypeRef {
		t.Helper()
		f := wantClean(t, "let slot: "+src+" = never\n")
		return f.Items[0].(*ast.TopLet).Binding.Typ
	}
	cases := []struct{ src, want string }{
		{"Int64", "Int64"},
		{"String", "String"},
		{"client.Reader", "client.Reader"},
		{"net.http.Client", "net.http.Client"},
		{"Vec<Int64>", "Vec<Int64>"},
		// F3 (chapter 10): nested closers are separated — the canonical
		// render keeps them adjacent, the source must not.
		{"Map<String, Vec<Int64> >", "Map<String,Vec<Int64>>"},
		{"Vec<Vec<Int64> >", "Vec<Vec<Int64>>"},
		{"Dyn<I>", "Dyn<I>"},
		{"(Int64, String)", "(Int64,String)"},
		{"(Int64, String,)", "(Int64,String)"},
		{"()", "()"},
		{"fn(Int64) -> String", "fn(Int64)->String"},
		{"fn() -> ()", "fn()->()"},
		{"fn(Int64, String) io db -> Bool", "fn(Int64,String) io db->Bool"},
	}
	for _, c := range cases {
		if got := typStr(typeOf(t, c.src)); got != c.want {
			t.Errorf("%s: got %s, want %s", c.src, got, c.want)
		}
	}
	// F3 (chapter 10): at a closer position only the bare `>` is the closer.
	// `>>` and `>=` there are the shift and ge operators maximal munch made,
	// so the unseparated form is E0105 with the separation remediation; the
	// separated form needs its own space before `=`.
	wantDiag(t, "let m: Map<String, Vec<Int64>>= never\n", "E0105", "nested closers are separated", 1, 29)
	wantDiag(t, "let n: Map<String, Int64>= never\n", "E0105", "nested closers are separated", 1, 25)
	wantDiag(t, "let v: Vec<Vec<Int64>> = 1\n", "E0105", "nested closers are separated", 1, 21)
	wantClean(t, "let m: Map<String, Vec<Int64> > = never\n")
	wantClean(t, "let n: Map<String, Int64> = never\n")
	// Slot violations are E0105 naming the productions considered.
	wantDiag(t, "let x: int64 = 1\n", "E0105", "fits no type reference production", 1, 8)
	wantDiag(t, "let x: (Int64) = 1\n", "E0105", "fits no type reference production", 1, 8)
	wantDiag(t, "let x: mod.name = 1\n", "E0105", "fits no type reference production", 1, 12)
	wantDiag(t, "let x: Mod.Name = 1\n", "E0105", "fits no type reference production", 1, 11)
	wantDiag(t, "let x: Name.T = 1\n", "E0105", "fits no type reference production", 1, 12)
	wantDiag(t, "let x: fn(Int64) = 1\n", "E0105", "in a function type reference", 1, 18)
	// The generic construction head was a disclosed M2 gap (F1): `<` and
	// `>` parsed as one non-associative comparison level, so `Box<Int64> {`
	// was E0104 at the `>`. M6a's postfix angle-bracket lookahead (design
	// D2) consumes the shape as a construction with explicit type
	// arguments; the pin flips with the feature, red until T3/T4 land.
	{
		f := wantClean(t, "fn f() {\n    let b = Box<Int64> { v: 1 }\n}\n")
		b := f.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.Binding)
		ct, ok := b.Init.(*ast.Construct)
		if !ok {
			t.Fatalf("Construct wanted, got %T", b.Init)
		}
		if len(ct.TypeArgs) != 1 || ct.TypeArgs[0].(*ast.NamedType).Name != "Int64" {
			t.Fatalf("Box<Int64> type arguments wanted, got %+v", ct.TypeArgs)
		}
	}
}

// --- D6: the boundary form groups, one probe each -----------------------------

func TestBoundaryForms(t *testing.T) {
	probes := []struct {
		src  string
		what string
	}{
		{"fn f() {\n    scope x {}\n}\n", scopeForms},
		{"fn f() {\n    let y = x?\n}\n", "chapter 14 (errors) forms"},
		{"effect io {}\n", effectForms},
		{"fn f(x: Int64) effect io -> Int64 {\n    return x\n}\n", effectForms},
		{"fn f() {\n    task g() {}\n}\n", concurForms},
		{"fn f() {\n    select {}\n}\n", concurForms},
		{"foreign fn f() {}\n", ffiForms},
		{"test \"t\" {\n}\n", testForms},
		{"fn f() {\n    mock m {}\n}\n", testForms},
		{"fn f(mut x: Int64) {}\n", "mut parameters"},
	}
	for _, p := range probes {
		wantBnd(t, p.src, p.what)
	}
}

// --- D12: message shape against the registry obligations ----------------------

func TestDiagnosticShape(t *testing.T) {
	d := wantDiag(t, "fn f() {\n    let x = y = 1\n}\n", "E0103", "assignment is not an expression", 2, 15)
	if !strings.HasPrefix(d.Message(), "assignment is not an expression — ") {
		t.Fatalf("message must start with the registry title: %q", d.Message())
	}
	j := d.JSON()
	if !strings.Contains(j, `"help":`) {
		t.Fatalf("a diagnostic must carry its registry remediation: %s", j)
	}
	if !strings.HasPrefix(j, `{"type":"diagnostic","severity":"error"`) {
		t.Fatalf("JSON must open with the fixed field order: %s", j)
	}
	if !strings.Contains(j, `"help":`) {
		t.Fatalf("JSON must carry the help field: %s", j)
	}
	// A lexical diagnostic floats up unchanged, no tree.
	wantDiag(t, "let a = \"unterminated\n", "E0002", "unterminated string literal", 1, 9)
}
