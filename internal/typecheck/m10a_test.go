package typecheck

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/parser"
)

// M10a testing-check (design D3-D8): the test body's valueless context
// with E1401 suppression, the mock checking chain and its judgment order,
// advanceTime's typing and E1806 positions, std.test loading with the
// assertEqual domain face, and cross-module mock targets through
// CheckProject (the project loading face itself is the M10b run tower).
// Written test-first: the checks land in T4-T6.

// runCheckAs runs the checker under an explicit file name — the shared
// helpers pin "test.we", while module identity is a name fact. A parse
// diagnostic surfaces as the diagnostic (the pipeline's own order): the
// test-body return discipline is the parse stage's face under the
// "(test)" context, E0402 included.
func runCheckAs(t *testing.T, name, src string) (*diag.Diagnostic, *NotImplemented) {
	t.Helper()
	f, d, ni := parser.Parse(name, []byte(src))
	if d != nil {
		return d, nil
	}
	if ni != nil {
		t.Fatalf("parse boundary: %s", ni.What)
	}
	return Check(f, name, SingleFile)
}

func wantOKAs(t *testing.T, name, src string) {
	t.Helper()
	if d, ni := runCheckAs(t, name, src); d != nil || ni != nil {
		t.Fatalf("want clean, got d=%v ni=%+v", d, ni)
	}
}

func wantDiagAs(t *testing.T, name, src, code, part string, line, col int) {
	t.Helper()
	d, ni := runCheckAs(t, name, src)
	if ni != nil {
		t.Fatalf("expected %s, got boundary %q", code, ni.What)
	}
	if d == nil {
		t.Fatalf("expected %s, got a clean check: %q", code, src)
	}
	if d.Code() != code || !strings.Contains(d.Message(), part) {
		t.Fatalf("expected %s (%q), got %s (%s)", code, part, d.Code(), d.Message())
	}
	pos := `"line":` + itoa(line) + `,"column":` + itoa(col)
	if !strings.Contains(d.JSON(), pos) {
		t.Fatalf("expected position %d:%d, got %s", line, col, d.JSON())
	}
}

func wantBndAs(t *testing.T, name, src, what string) {
	t.Helper()
	d, ni := runCheckAs(t, name, src)
	if d != nil {
		t.Fatalf("expected boundary %q, got diagnostic: %s", what, d.Human())
	}
	if ni == nil {
		t.Fatalf("expected boundary %q, got a clean check: %q", what, src)
	}
	if ni.What != what {
		t.Fatalf("boundary What %q, want %q", ni.What, what)
	}
}

// runCrossModule checks one test module against a helper module through
// CheckProject: util is ingested first (its bucket serves the qualified
// targets), the test module rides as a dependency, and a minimal clean
// root main satisfies the project convention. The loader's import-graph
// walk cannot reach a _test.we stem (follow-up #12) — this pins the
// checker's cross-module machinery itself (design D11).
func runCrossModule(t *testing.T, testSrc string) (*diag.Diagnostic, *NotImplemented) {
	t.Helper()
	utilSrc := "pub fn read(n: String) -> String {\n    return n\n}\n\nfn hidden() {\n    return\n}\n"
	rootSrc := "pub type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n"
	utilF, d, ni := parser.Parse("src/util.we", []byte(utilSrc))
	if d != nil || ni != nil {
		t.Fatalf("util parse failed: %v %+v", d, ni)
	}
	testF, d, ni := parser.Parse("src/t_test.we", []byte(testSrc))
	if d != nil || ni != nil {
		t.Fatalf("test module parse failed: %v %+v", d, ni)
	}
	rootF, d, ni := parser.Parse("src/main.we", []byte(rootSrc))
	if d != nil || ni != nil {
		t.Fatalf("root parse failed: %v %+v", d, ni)
	}
	return CheckProject(rootF, "src/main.we", []Module{
		{Key: "util", Path: "src/util.we", File: utilF},
		{Key: "t", Path: "src/t_test.we", File: testF},
	})
}

// D3: the test body is a valueless fn context ("(test)") whose direct
// calls skip E1401 (a test is driver code) — the suppression penetrates
// control-flow nesting but not task bodies (own segment), mock bodies
// (target segment), or the module's plain fns; E1405 and E1402 hold.
func TestTestBodyContext(t *testing.T) {
	n := "tests/t_test.we"
	wantOKAs(t, n, "test \"t\" {\n    return\n}\n")
	wantDiagAs(t, n, "test \"t\" {\n    return 5\n}\n",
		"E0402", `"(test)" declares no return type`, 2, 5)
	// Suppression: the body performs io with no segment of its own.
	wantOKAs(t, n, "fn work() effect io {\n    return\n}\n\ntest \"driver\" {\n    work()\n}\n")
	wantOKAs(t, n, "fn work() effect io {\n    return\n}\n\ntest \"driver\" {\n    if true {\n        work()\n    }\n    while false {\n        work()\n    }\n}\n")
	// A task body inside answers its own declared segment (E1401 holds,
	// addressing the task block).
	wantDiagAs(t, n, "fn work() effect net {\n    return\n}\n\ntest \"extent\" {\n    scope {\n        let t = task effect io {\n            work()\n            1\n        }\n        let _ = t.await()\n    }\n}\n",
		"E1401", `which the task block does not declare`, 8, 13)
	// E1405: a test module is a normal module — top-level purity holds.
	wantDiagAs(t, n, "fn work() effect io {\n    return\n}\n\nlet x = work()\n\ntest \"t\" {\n    return\n}\n",
		"E1405", `the initializer of "x" calls "work"`, 5, 9)
	// E1402: closure agreement inside a test body holds.
	wantDiagAs(t, n, "fn touch() effect net {\n    return\n}\n\ntest \"slot\" {\n    let g: fn(Int64) io -> () = |x: Int64| {\n        touch()\n    }\n    let _ = g\n}\n",
		"E1402", `the value performs effect "net"`, 6, 9)
	// Plain fns in a test module keep the full judgment.
	wantDiagAs(t, n, "fn work() effect io {\n    return\n}\n\nfn driver() {\n    work()\n}\n\ntest \"t\" {\n    return\n}\n",
		"E1401", `which "driver" does not declare`, 6, 5)
}

// D4-D6: the mock chain — target resolution, mockability, verbatim
// signature, one-target-one-mock per block — in judgment order, with
// the mock body checked against the target's own segment.
func TestMockChecking(t *testing.T) {
	n := "tests/t_test.we"
	wantOKAs(t, n, "fn read(n: String) effect io -> String {\n    return n\n}\n\ntest \"m\" {\n    mock read(n: String) -> String effect io {\n        return \"x\"\n    }\n    assert(true, \"done\")\n}\n")
	// The mock body is a fn context of the target's shape (E0402 reused,
	// naming the target).
	wantDiagAs(t, n, "fn work() effect io {\n    return\n}\n\ntest \"w\" {\n    mock work() effect io {\n        return 5\n    }\n}\n",
		"E0402", `"work" declares no return type`, 7, 9)
	// Judgment order: resolution (E1304) before signature, mockability
	// (E1804) before signature, signature (E1803) before duplication.
	wantDiagAs(t, n, "test \"nope\" {\n    mock missing(n: String) -> String {\n        return \"x\"\n    }\n}\n",
		"E1304", `"missing" is held by no scope`, 2, 10)
	wantDiagAs(t, n, "fn pair<T>(a: T, b: T) -> T {\n    return a\n}\n\ntest \"g\" {\n    mock pair(a: Int64) -> Int64 {\n        return 1\n    }\n}\n",
		"E1804", `pair is a generic fn`, 6, 10)
	wantDiagAs(t, n, "fn read(n: String) effect io -> String {\n    return n\n}\n\ntest \"order\" {\n    mock read(n: Int64) -> String effect io {\n        return \"x\"\n    }\n    mock read(n: String) -> String effect io {\n        return \"y\"\n    }\n}\n",
		"E1803", "mock read(n: Int64) -> String effect io, target read(n: String) -> String effect io", 6, 5)
	// E1804's non-fn categories: a constructor and a top-level binding.
	wantDiagAs(t, n, "record Point { x: Int64, y: Int64 }\n\ntest \"c\" {\n    mock Point(x: Int64, y: Int64) {\n        return\n    }\n}\n",
		"E1804", `Point is a constructor`, 4, 10)
	wantDiagAs(t, n, "let size: Int64 = 3\n\ntest \"l\" {\n    mock size(a: Int64) {\n        return\n    }\n}\n",
		"E1804", `size is a top-level binding`, 4, 10)
	// E1803's six mismatch categories, anchored at the mock keyword, the
	// message naming the first differing category and both signatures.
	read := "fn read(n: String) effect io -> String {\n    return n\n}\n\n"
	wantDiagAs(t, n, read+"test \"a\" {\n    mock read(k: String) -> String effect io {\n        return \"x\"\n    }\n}\n",
		"E1803", "the parameter list differs: mock read(k: String) -> String effect io, target read(n: String) -> String effect io", 6, 5)
	wantDiagAs(t, n, read+"test \"b\" {\n    mock read(n: Int64) -> String effect io {\n        return \"x\"\n    }\n}\n",
		"E1803", "the parameter list differs", 6, 5)
	wantDiagAs(t, n, "fn work() effect io {\n    return\n}\n\ntest \"c\" {\n    mock work() -> String effect io {\n        return \"x\"\n    }\n}\n",
		"E1803", "the declared return differs: mock work() -> String effect io, target work() effect io", 6, 5)
	wantDiagAs(t, n, read+"test \"d\" {\n    mock read(n: String) -> Int64 effect io {\n        return 1\n    }\n}\n",
		"E1803", "the declared return differs", 6, 5)
	wantDiagAs(t, n, read+"test \"e\" {\n    mock read(n: String) -> String {\n        return \"x\"\n    }\n}\n",
		"E1803", "the effect segment differs: mock read(n: String) -> String, target read(n: String) -> String effect io", 6, 5)
	wantDiagAs(t, n, "fn read(n: String) effect io net -> String {\n    return n\n}\n\ntest \"f\" {\n    mock read(n: String) -> String effect net io {\n        return \"x\"\n    }\n}\n",
		"E1803", "the effect segment differs: mock read(n: String) -> String effect net io, target read(n: String) -> String effect io net", 6, 5)
	// E1805: one target mocked twice in one block, anchored at the second.
	wantDiagAs(t, n, read+"test \"dup\" {\n    mock read(n: String) -> String effect io {\n        return \"a\"\n    }\n    mock read(n: String) -> String effect io {\n        return \"b\"\n    }\n}\n",
		"E1805", `read is mocked twice in this block`, 9, 5)
	// The mock body checks against the target's segment: a net call under
	// an io target is E1401 naming the target; an io call is green.
	wantDiagAs(t, n, "fn fetch(n: String) effect net -> String {\n    return n\n}\n\nfn read(n: String) effect io -> String {\n    return n\n}\n\ntest \"segment\" {\n    mock read(n: String) -> String effect io {\n        return fetch(\"k\")\n    }\n}\n",
		"E1401", `which "read" does not declare`, 11, 16)
	wantOKAs(t, n, "fn touch(n: String) effect io {\n    return\n}\n\nfn read(n: String) effect io -> String {\n    return n\n}\n\ntest \"segment\" {\n    mock read(n: String) -> String effect io {\n        touch(\"k\")\n        return \"x\"\n    }\n}\n")
	// A synthetic std module fn is a module-level monomorphic pub fn by
	// the chapter's literal words — mockable with no special case (the
	// design D4 disclosure, pinned here rather than in goldens).
	wantOKAs(t, n, "import std.concurrent as conc\n\ntest \"synthetic\" {\n    mock conc.channel(n: Int64) {\n        return\n    }\n}\n")
}

// D7: advanceTime types (Int64) -> () and is legal exactly in the test
// extent — the test body and the task/scope bodies inside it; the name
// and call positions judge alike (E1806 at the name token).
func TestAdvanceTimePosition(t *testing.T) {
	n := "tests/t_test.we"
	wantOKAs(t, n, "test \"clock\" {\n    advanceTime(100)\n}\n")
	wantOKAs(t, n, "test \"extent\" {\n    scope {\n        advanceTime(5)\n        let t = task effect io {\n            advanceTime(5)\n            1\n        }\n        let _ = t.await()\n    }\n    advanceTime(1)\n}\n")
	wantDiagAs(t, n, "fn tick() {\n    advanceTime(5)\n    return\n}\n",
		"E1806", "advanceTime called outside a test block", 2, 5)
	wantDiagAs(t, "src/main.we", "pub type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    advanceTime(5)\n    return Ok(())\n}\n",
		"E1806", "advanceTime called outside a test block", 4, 5)
	wantDiagAs(t, n, "test \"closure\" {\n    let f = fn() {\n        advanceTime(5)\n        return\n    }\n    let _ = f\n}\n",
		"E1806", "advanceTime called outside a test block", 3, 9)
	wantDiagAs(t, n, "fn read(n: String) effect io -> String {\n    return n\n}\n\ntest \"mock body\" {\n    mock read(n: String) -> String effect io {\n        advanceTime(5)\n        return \"x\"\n    }\n}\n",
		"E1806", "advanceTime called outside a test block", 7, 9)
	// Typing: the argument judges against Int64 (E0501), the name is a
	// fn value inside the extent and E1806 outside it.
	wantDiagAs(t, n, "test \"t\" {\n    advanceTime(true)\n}\n",
		"E0501", "the argument is Bool, the parameter is Int64", 2, 17)
	wantOKAs(t, n, "test \"t\" {\n    let f = advanceTime\n    let _ = f\n}\n")
	wantDiagAs(t, n, "fn f() {\n    let g = advanceTime\n}\n",
		"E1806", "advanceTime called outside a test block", 2, 13)
}

// D8: std.test loads as the third synthetic module — the two Bool faces
// type through the ordinary import surface, assertEqual is the
// same-type-pair special face over the scalar/Bool/String domain (an
// honest boundary beyond it), the member closure is E1304's, and the
// bare import binds the keyword name inertly.
func TestStdTestLoading(t *testing.T) {
	n := "tests/t_test.we"
	wantOKAs(t, n, "import std.test as st\n\ntest \"faces\" {\n    st.assertTrue(true)\n    st.assertFalse(false)\n}\n")
	wantDiagAs(t, n, "import std.test as st\n\ntest \"t\" {\n    st.assertTrue(\"x\")\n}\n",
		"E0501", "the argument is String, the parameter is Bool", 4, 19)
	wantOKAs(t, n, "import std.test as st\n\ntest \"equal\" {\n    st.assertEqual(1, 2)\n    st.assertEqual(true, false)\n    st.assertEqual(\"a\", \"b\")\n}\n")
	wantDiagAs(t, n, "import std.test as st\n\ntest \"m\" {\n    st.assertEqual(1, true)\n}\n",
		"E0501", "the argument is Bool, the parameter is Int64", 4, 23)
	// The record this row used to stop on now declares Eq and is in the
	// domain; T10's TestAssertEqualEqDomain carries the residual rows.
	wantOKAs(t, n, "import std.test as st\n\nrecord Point { x: Int64, y: Int64 } derives Eq\n\ntest \"domain\" {\n    let a = Point { x: 1, y: 2 }\n    let b = Point { x: 1, y: 2 }\n    st.assertEqual(a, b)\n}\n")
	wantDiagAs(t, n, "import std.test as st\n\ntest \"u\" {\n    st.assertLength(1)\n}\n",
		"E1304", `the module "std.test" declares no "assertLength"`, 4, 5)
	// The bare import is legal and inert (the keyword name binds nothing
	// usable); loading itself is not test-gated; an unknown sibling
	// stays E1302's std form.
	wantOKAs(t, n, "import std.test\n\ntest \"bare\" {\n    assert(true, \"bare\")\n}\n")
	// A user's own same-named fns coexist with the std.test faces: the
	// bare name is the local declaration, the qualified name the face —
	// the gate is the module key, never the fn name (a user module named
	// std.test is structurally unreachable: the std segment never
	// resolves against the file system).
	wantOKAs(t, n, "import std.test as st\n\nfn assertTrue(n: Int64) {\n    return\n}\n\ntest \"coexist\" {\n    assertTrue(3)\n    st.assertTrue(true)\n}\n")
	wantOKAs(t, "src/x.we", "import std.test\n\nfn f() {\n    return\n}\n")
	wantDiagAs(t, "src/x.we", "import std.testing\n\nfn f() {\n    return\n}\n",
		"E1302", `no standard-library module "std.testing"`, 1, 8)
}

// T10 D9: assertEqual's domain is the scalar set closed recursively over
// derives-Eq composites. A record, sum, or newtype carries its comparands
// only when it declares Eq and every component is in the domain; a tuple
// has nowhere to write a clause and follows its elements. A composite
// that declares nothing, or that reaches a leaf outside the scalar set,
// is the residual boundary — which the domain word retires into, so the
// stop reads as the generic-fn word rather than a word of its own.
func TestAssertEqualEqDomain(t *testing.T) {
	n := "tests/t_test.we"
	// The accept set: each row is one composite shape whose leaves are all
	// in the scalar set.
	wantOKAs(t, n, "import std.test as st\n\nrecord Point { x: Int64, y: Int64 } derives Eq\n\ntest \"t\" {\n    let a = Point { x: 1, y: 2 }\n    let b = Point { x: 2, y: 2 }\n    st.assertEqual(a, b)\n}\n")
	wantOKAs(t, n, "import std.test as st\n\nrecord Named { name: String, n: UInt8 } derives Eq\n\ntest \"t\" {\n    let a = Named { name: \"a\", n: 1u8 }\n    let b = Named { name: \"b\", n: 1u8 }\n    st.assertEqual(a, b)\n}\n")
	// A nested composite is in the domain through its own clause.
	wantOKAs(t, n, "import std.test as st\n\nrecord Inner { x: Int64 } derives Eq\nrecord Outer { i: Inner, b: Bool } derives Eq\n\ntest \"t\" {\n    let a = Outer { i: Inner { x: 1 }, b: true }\n    let b = Outer { i: Inner { x: 2 }, b: true }\n    st.assertEqual(a, b)\n}\n")
	// The clause's other targets ride along; only Eq gates the domain.
	wantOKAs(t, n, "import std.test as st\n\nrecord Point { x: Int64 } derives Eq, Hash, Show\n\ntest \"t\" {\n    let a = Point { x: 1 }\n    let b = Point { x: 2 }\n    st.assertEqual(a, b)\n}\n")
	wantOKAs(t, n, "import std.test as st\n\ntype Shape = Circle(Int64) | Dot derives Eq\n\ntest \"t\" {\n    let a = Circle(1)\n    let b = Dot\n    st.assertEqual(a, b)\n}\n")
	wantOKAs(t, n, "import std.test as st\n\nnewtype UserId(Int64) derives Eq\n\ntest \"t\" {\n    let a = UserId(1)\n    let b = UserId(2)\n    st.assertEqual(a, b)\n}\n")
	// A tuple needs no clause: there is no declaration to carry one, so it
	// follows its elements — including a record element that has its own.
	wantOKAs(t, n, "import std.test as st\n\ntest \"t\" {\n    let a = (1, \"x\")\n    let b = (2, \"x\")\n    st.assertEqual(a, b)\n}\n")
	wantOKAs(t, n, "import std.test as st\n\nrecord Point { x: Int64 } derives Eq\n\ntest \"t\" {\n    let a = (Point { x: 1 }, true)\n    let b = (Point { x: 2 }, true)\n    st.assertEqual(a, b)\n}\n")
	// A newtype over a record is transparent to the domain's walk.
	wantOKAs(t, n, "import std.test as st\n\nrecord Point { x: Int64 } derives Eq\nnewtype Wrapped(Point) derives Eq\n\ntest \"t\" {\n    let a = Wrapped(Point { x: 1 })\n    let b = Wrapped(Point { x: 2 })\n    st.assertEqual(a, b)\n}\n")

	// The reject set: each row stops at the residual boundary.
	bnd := func(src string) {
		t.Helper()
		wantBndAs(t, n, src, "generic functions in code generation (monomorphization is the B-track codegen-full widening)")
	}
	// No clause: chapter 10's composite equality is the generated
	// .equals(), so a composite that generates none has none to compare.
	bnd("import std.test as st\n\nrecord Point { x: Int64, y: Int64 }\n\ntest \"t\" {\n    let a = Point { x: 1, y: 2 }\n    let b = Point { x: 2, y: 2 }\n    st.assertEqual(a, b)\n}\n")
	// The clause is present but a leaf is outside the scalar set: Float64
	// and Rune both carry the Eq capability (E0823 admits them) without
	// being comparanda the assertion fixes.
	bnd("import std.test as st\n\nrecord P { f: Float64 } derives Eq\n\ntest \"t\" {\n    let a = P { f: 1.0 }\n    let b = P { f: 2.0 }\n    st.assertEqual(a, b)\n}\n")
	bnd("import std.test as st\n\nrecord P { r: Rune } derives Eq\n\ntest \"t\" {\n    let a = P { r: 'a' }\n    let b = P { r: 'b' }\n    st.assertEqual(a, b)\n}\n")
	// The recursion the domain gate owns is the leaf set's, not the
	// clause's: E0823 already refuses a component that declares nothing
	// (carries and this predicate agree there), so a component that
	// declares Eq while reaching a leaf the assertion does not fix is
	// what the second level is for — the shell passes E0823 and the walk
	// still has to descend.
	bnd("import std.test as st\n\nrecord Inner { f: Float64 } derives Eq\nrecord Outer { i: Inner } derives Eq\n\ntest \"t\" {\n    let a = Outer { i: Inner { f: 1.0 } }\n    let b = Outer { i: Inner { f: 2.0 } }\n    st.assertEqual(a, b)\n}\n")
	bnd("import std.test as st\n\ntype Shape = Circle(Int64) | Dot\n\ntest \"t\" {\n    let a = Circle(1)\n    let b = Dot\n    st.assertEqual(a, b)\n}\n")
	// A payload outside the scalar set stops the sum's walk.
	bnd("import std.test as st\n\ntype Box = Hold(Float64) | Empty derives Eq\n\ntest \"t\" {\n    let a = Hold(1.0)\n    let b = Empty\n    st.assertEqual(a, b)\n}\n")
	// A tuple element that declares nothing cuts the tuple off: the
	// clause requirement is per component, and a tuple has none to lend.
	bnd("import std.test as st\n\nrecord Point { x: Int64 }\n\ntest \"t\" {\n    let a = (Point { x: 1 }, true)\n    let b = (Point { x: 2 }, true)\n    st.assertEqual(a, b)\n}\n")
	// A List is not a comparand the assertion fixes.
	bnd("import std.test as st\n\ntest \"t\" {\n    let a: List<Int64> = []\n    st.assertEqual(a, a)\n}\n")
	// A nested tuple is in the domain — its leaves are — even though code
	// generation's aggregate layout stops one level down (the boundary
	// lives in the build stage, not in this predicate).
	wantOKAs(t, n, "import std.test as st\n\ntest \"t\" {\n    let a = (1, (2, 3))\n    let b = (1, (2, 4))\n    st.assertEqual(a, b)\n}\n")
}

// D6/D11: cross-module mock targets resolve through the import face —
// pub green, non-pub E1303, and an alias the same identity as the bare
// name (one module one key, so the second mock is E1805's).
func TestMockCrossModule(t *testing.T) {
	d, ni := runCrossModule(t, "import util\n\ntest \"cross\" {\n    mock util.read(n: String) -> String {\n        return \"x\"\n    }\n}\n")
	if d != nil || ni != nil {
		t.Fatalf("pub target: want clean, got d=%v ni=%+v", d, ni)
	}
	testSrc := "import util\n\ntest \"secret\" {\n    mock util.hidden() {\n        return\n    }\n}\n"
	d, ni = runCrossModule(t, testSrc)
	if ni != nil {
		t.Fatalf("expected E1303, got boundary %q", ni.What)
	}
	if d == nil || d.Code() != "E1303" || !strings.Contains(d.Message(), `"hidden" is declared in "util" without pub`) {
		t.Fatalf("expected E1303, got %v", d)
	}
	if pos := `"line":4,"column":15`; !strings.Contains(d.JSON(), pos) {
		t.Fatalf("expected position 3:15, got %s", d.JSON())
	}
	testSrc = "import util\nimport util as u\n\ntest \"alias\" {\n    mock util.read(n: String) -> String {\n        return \"a\"\n    }\n    mock u.read(n: String) -> String {\n        return \"b\"\n    }\n}\n"
	d, ni = runCrossModule(t, testSrc)
	if ni != nil {
		t.Fatalf("expected E1805, got boundary %q", ni.What)
	}
	if d == nil || d.Code() != "E1805" || !strings.Contains(d.Message(), `read is mocked twice in this block`) {
		t.Fatalf("expected E1805, got %v", d)
	}
	if pos := `"line":8,"column":5`; !strings.Contains(d.JSON(), pos) {
		t.Fatalf("expected position 8:5, got %s", d.JSON())
	}
}
