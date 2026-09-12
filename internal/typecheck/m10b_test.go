package typecheck

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/parser"
)

// M10b (design D4): std.time is the fourth synthetic module — now and
// sleep as Pub FnDecls with the time effect segment, loaded through the
// same import face as io/concurrent/test. Written test-first: the
// module is not registered yet, so the clean faces below fail at the
// load gate (E1302 std form) and the diagnostic faces fail to reach
// their codes. The loading gate keys on the module key, so a user
// module named time never disturbs the std entry — the M9a precedent.

func parseModule(t *testing.T, name, src string) *ast.File {
	t.Helper()
	f, d, ni := parser.Parse(name, []byte(src))
	if d != nil || ni != nil {
		t.Fatalf("%s parse failed: %v %+v", name, d, ni)
	}
	return f
}

// TestStdTimeSignatures pins the two entries' faces: now yields Int64,
// sleep takes Int64 and produces no value, both carry the time effect
// (a declaring fn is clean, a pure fn reports E1401), and the test
// body's suppression covers both.
func TestStdTimeSignatures(t *testing.T) {
	n := "tests/t_test.we"
	// Declaring fn: both calls clean, now's Int64 feeds the return.
	wantOKAs(t, n, "import std.time\n\nfn probe() effect time -> Int64 {\n    let _ = time.sleep(1)\n    return time.now()\n}\n\ntest \"probe\" {\n    let a = time.now()\n    let _ = time.sleep(100)\n    assert(true, \"ok\")\n}\n")
	// Pure fn: the segment is missing, E1401 names the callee's set.
	wantDiagAs(t, n, "import std.time\n\nfn pure() -> Int64 {\n    return time.now()\n}\n\ntest \"pure\" {\n    assert(true, \"ok\")\n}\n",
		"E1401", `time`, 4, 12)
	// Argument typing is the ordinary agreement face, anchored at the
	// argument expression (the checker's standing convention, exprPos).
	wantDiagAs(t, n, "import std.time\n\ntest \"args\" {\n    let _ = time.sleep(\"x\")\n}\n",
		"E0501", ``, 4, 24)
	// Alias arrival; the bare name is not prelude.
	wantOKAs(t, n, "import std.time as clock\n\nfn probe() effect time -> Int64 {\n    return clock.now()\n}\n\ntest \"alias\" {\n    assert(true, \"ok\")\n}\n")
	wantDiagAs(t, n, "import std.time\n\ntest \"bare\" {\n    let _ = now()\n}\n",
		"E1304", ``, 4, 13)
	// A task declaring time may sleep; the body's own segment governs. The
	// scope wrap is chapter 18's own rule — a task block sits inside some
	// scope block's body (E1618), the test body itself not being one.
	wantOKAs(t, n, "import std.time\n\ntest \"task sleep\" {\n    scope {\n        let t = task effect time {\n            let _ = time.sleep(5)\n        }\n        let _ = t.await()\n    }\n}\n")
	// The mock face: a std entry is a literal mock target (chapter 20's
	// own example), signature restated verbatim — the check face
	// accepts, so the run face must intercept (review F1's premise).
	wantOKAs(t, n, "import std.time\n\ntest \"mocked clock\" {\n    mock time.sleep(ms: Int64) effect time {\n        return\n    }\n    let _ = time.sleep(1000)\n}\n")
}

// TestStdTimeUserSameName: a user module keyed time coexists with the
// std entry — the import paths differ (std.time vs time), each name
// arrives from its own import. The deps list carries the std module the
// way the loader's graph walk places it (loadGraph asks StdModule per
// import; CheckProject itself takes the graph as given).
func TestStdTimeUserSameName(t *testing.T) {
	userTime := parseModule(t, "src/time.we", "pub fn helper() -> Int64 {\n    return 1\n}\n")
	stdTime, ok := StdModule("std.time")
	if !ok {
		t.Fatal("std.time not registered")
	}
	root := parseModule(t, "src/main.we", "import time\nimport std.time as clock\n\npub type AppError = Failed(String)\n\nfn probe() effect time -> Int64 {\n    return clock.now()\n}\n\npub fn main() effect time -> Result<(), AppError> {\n    let _ = time.helper()\n    let _ = probe()\n    return Ok(())\n}\n")
	d, ni, _ := CheckProject(root, "src/main.we", []Module{
		{Key: "std.time", Path: "std.time", File: stdTime},
		{Key: "time", Path: "src/time.we", File: userTime},
	})
	if d != nil || ni != nil {
		t.Fatalf("want clean, got d=%v ni=%+v", d, ni)
	}
}

// TestStdTimeEffectSegmentOrder: the segment sits between the parameter
// list and the arrow (chapter 6's order) — the swapped spelling stays
// E0105, not a new face.
func TestStdTimeEffectSegmentOrder(t *testing.T) {
	n := "tests/t_test.we"
	wantDiagAs(t, n, "import std.time\n\nfn probe() -> Int64 effect time {\n    return time.now()\n}\n\ntest \"order\" {\n    assert(true, \"ok\")\n}\n",
		"E0105", ``, 3, 21)
}
