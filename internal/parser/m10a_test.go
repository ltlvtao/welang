package parser

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M10a testing-check (design D1/D2): the two keyword-led productions —
// test blocks as top-level items, mock declarations as direct items of a
// test block body — module identity from the file name (E1801), the
// three E1802 positions, and the E0105 family faces. Written test-first:
// the AST nodes and productions land in T3.

// parseAs runs Parse under an explicit file name — the shared helpers
// pin "test.we", while module identity is a name fact (design D2).
func parseAs(t *testing.T, name, src string) (*ast.File, *NotImplemented) {
	t.Helper()
	f, d, ni := Parse(name, []byte(src))
	if d != nil {
		t.Fatalf("unexpected diagnostic: %s", d.Human())
	}
	return f, ni
}

func wantCleanAs(t *testing.T, name, src string) *ast.File {
	t.Helper()
	f, ni := parseAs(t, name, src)
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if f == nil {
		t.Fatalf("no tree for %q", src)
	}
	return f
}

func wantDiagAs(t *testing.T, name, src, code, part string, line, col int) {
	t.Helper()
	f, d, ni := Parse(name, []byte(src))
	if ni != nil {
		t.Fatalf("expected %s, got boundary %q", code, ni.What)
	}
	if d == nil {
		t.Fatalf("expected %s, got a clean parse: %q -> %+v", code, src, f)
	}
	if f != nil {
		t.Fatalf("a rejected input must produce no tree, got %+v", f)
	}
	if d.Code() != code || !strings.Contains(d.Message(), part) {
		t.Fatalf("expected %s (%q), got %s (%s)", code, part, d.Code(), d.Message())
	}
	pos := `"line":` + itoa(line) + `,"column":` + itoa(col)
	if !strings.Contains(d.JSON(), pos) {
		t.Fatalf("expected position %d:%d, got %s", line, col, d.JSON())
	}
}

// The test-block production: description literal, body block, positions.
func TestTestBlockProduction(t *testing.T) {
	src := "fn helper() -> Int64 {\n    return 1\n}\n\ntest \"absolute value\" {\n    assert(helper() == 1, \"one\")\n}\n"
	f := wantCleanAs(t, "tests/math_test.we", src)
	td, ok := f.Items[1].(*ast.TestDecl)
	if !ok {
		t.Fatalf("items[1]: want *ast.TestDecl, got %T", f.Items[1])
	}
	if td.Desc != "absolute value" {
		t.Fatalf("desc: %q", td.Desc)
	}
	if td.Line != 5 || td.Col != 1 {
		t.Fatalf("test keyword position: %d:%d", td.Line, td.Col)
	}
	if td.DescLine != 5 || td.DescCol != 6 {
		t.Fatalf("desc position: %d:%d", td.DescLine, td.DescCol)
	}
	if len(td.Body.Items) != 1 {
		t.Fatalf("body: %+v", td.Body.Items)
	}
	// An interpolated description rides verbatim: the check tower carries
	// the raw text (decoding is the run tower's report face, design D1).
	f = wantCleanAs(t, "tests/t_test.we", "test \"count: ${n}\" {\n}\n")
	td = f.Items[0].(*ast.TestDecl)
	if td.Desc != "count: ${n}" {
		t.Fatalf("interpolated desc: %q", td.Desc)
	}
	// A non-string description token is E0105's family face at the token.
	wantDiagAs(t, "tests/t_test.we", "test 5 {\n}\n",
		"E0105", `where a test block names its description string`, 1, 6)
	// pub never reaches the production: the pub-tail dispatch reports.
	wantDiagAs(t, "tests/t_test.we", "pub test \"p\" {\n}\n",
		"E0105", `"test" after pub`, 1, 5)
	// Statement position stays E0105: a test block is not a statement.
	wantDiagAs(t, "tests/t_test.we", "test \"outer\" {\n    test \"inner\" { }\n}\n",
		"E0105", `fits no statement production`, 2, 5)
}

// The mock production: target (bare and qualified), restated signature
// fields, and the three E1802 positions (design D1's depth-1 context).
func TestMockProduction(t *testing.T) {
	src := "fn read(n: String) effect io -> String {\n    return n\n}\n\ntest \"mock\" {\n    mock read(n: String) -> String effect io {\n        return \"x\"\n    }\n}\n"
	f := wantCleanAs(t, "tests/cache_test.we", src)
	td := f.Items[1].(*ast.TestDecl)
	md, ok := td.Body.Items[0].(*ast.MockDecl)
	if !ok {
		t.Fatalf("body[0]: want *ast.MockDecl, got %T", td.Body.Items[0])
	}
	if md.Target != "read" || md.TargetQual != "" {
		t.Fatalf("target: %q qual %q", md.Target, md.TargetQual)
	}
	if md.Line != 6 || md.Col != 5 || md.TargetLine != 6 || md.TargetCol != 10 {
		t.Fatalf("positions: mock %d:%d target %d:%d", md.Line, md.Col, md.TargetLine, md.TargetCol)
	}
	if len(md.Params) != 1 || md.Params[0].Name != "n" {
		t.Fatalf("params: %+v", md.Params)
	}
	if nt, ok := md.Params[0].Type.(*ast.NamedType); !ok || nt.Name != "String" {
		t.Fatalf("param type: %T", md.Params[0].Type)
	}
	if !md.HasRet {
		t.Fatalf("HasRet: %+v", md)
	}
	if nt, ok := md.Ret.(*ast.NamedType); !ok || nt.Name != "String" {
		t.Fatalf("ret: %T", md.Ret)
	}
	if len(md.EffectTags) != 1 || md.EffectTags[0] != "io" {
		t.Fatalf("tags: %+v", md.EffectTags)
	}
	if md.EffectLine != 6 || md.EffectCol != 43 {
		t.Fatalf("effect position: %d:%d", md.EffectLine, md.EffectCol)
	}
	if len(md.Body.Items) != 1 {
		t.Fatalf("body: %+v", md.Body.Items)
	}
	// A qualified target splits into qualifier and name.
	f = wantCleanAs(t, "tests/t_test.we", "test \"q\" {\n    mock util.read(n: String) -> String {\n        return \"x\"\n    }\n}\n")
	md = f.Items[0].(*ast.TestDecl).Body.Items[0].(*ast.MockDecl)
	if md.TargetQual != "util" || md.Target != "read" || md.TargetCol != 15 {
		t.Fatalf("qualified target: %+v", md)
	}
	// The valueless, segment-free form: no return slot, no tags.
	f = wantCleanAs(t, "tests/t_test.we", "test \"v\" {\n    mock work() {\n        return\n    }\n}\n")
	md = f.Items[0].(*ast.TestDecl).Body.Items[0].(*ast.MockDecl)
	if md.HasRet || md.Ret != nil || len(md.EffectTags) != 0 {
		t.Fatalf("valueless mock: %+v", md)
	}
	// E1802's three positions, all anchored at the mock keyword.
	wantDiagAs(t, "tests/t_test.we", "fn g() {\n    return\n}\n\nmock g() {\n    return\n}\n",
		"E1802", "mock declaration outside a test block", 5, 1)
	wantDiagAs(t, "tests/t_test.we", "fn g() {\n    return\n}\n\nfn f() {\n    mock g() {\n        return\n    }\n    return\n}\n",
		"E1802", "mock declaration outside a test block", 6, 5)
	wantDiagAs(t, "tests/t_test.we", "fn g() {\n    return\n}\n\ntest \"nested\" {\n    if true {\n        mock g() {\n            return\n        }\n    }\n}\n",
		"E1802", "mock declaration outside a test block", 7, 9)
	// A non-name target token is E0105's family face at the token.
	wantDiagAs(t, "tests/t_test.we", "test \"t\" {\n    mock 5() {\n        return\n    }\n}\n",
		"E0105", `where a mock declaration names its target function`, 2, 10)
}

// Module identity is the file-name suffix (design D2): the suffix alone
// decides, and E1801 rides the same fact at the test keyword.
func TestTestModuleIdentity(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"tests/x_test.we", true},
		{"x_test.we", true},
		{"_test.we", true},
		{"src/x.we", false},
		{"test.we", false},
		{"x_test.wel", false},
		{"xtest.we", false},
	}
	for _, c := range cases {
		src := "fn f() {\n    return\n}\n"
		if c.want {
			src = "test \"a\" {\n}\n"
		}
		f := wantCleanAs(t, c.name, src)
		if f.IsTestModule != c.want {
			t.Fatalf("%s: IsTestModule %v, want %v", c.name, f.IsTestModule, c.want)
		}
	}
	// The same test block is E1801 in a plain module, clean in a test one.
	block := "fn work() {\n    return\n}\n\ntest \"outside\" {\n    work()\n}\n"
	wantDiagAs(t, "src/plain.we", block, "E1801",
		"test block outside a test module", 5, 1)
	wantCleanAs(t, "src/plain_test.we", block)
}
