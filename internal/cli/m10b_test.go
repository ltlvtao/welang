package cli

import (
	"os/exec"
	"testing"
)

// M10b (design D6): the we test runner's three pure faces — discovery
// order (path-sorted files, source order within a file, tests/ only),
// the --filter read (Go regexp, unanchored, matched against the test
// name; an invalid pattern is a usage error), and the subprocess exit
// mapping (0 passes through; every other outcome — failure exit or a
// runtime abort — reports 1, stderr already carrying the story).
// Written test-first: red today on the undefined symbols alone.

func TestDiscoverTestsOrder(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":              manifest,
		"tests/m_test.we":      "test \"one\" {\n    assert(true, \"1\")\n}\n\ntest \"two\" {\n    assert(true, \"2\")\n}\n",
		"tests/unit/b_test.we": "test \"deep\" {\n    assert(true, \"d\")\n}\n",
		"tests/plain.we":       "fn helper() {\n    return\n}\n",
		"src/helpers_test.we":  "test \"src side\" {\n    assert(true, \"s\")\n}\n",
	})
	chdir(t, dir)
	got, code := discoverTests(".")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	want := []string{
		"tests/m_test.we\x00one",
		"tests/m_test.we\x00two",
		"tests/unit/b_test.we\x00deep",
	}
	if len(got) != len(want) {
		t.Fatalf("want %d tests, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		file, name := got[i].File, got[i].Name
		if file+"\x00"+name != w {
			t.Fatalf("test %d: want %q, got %s / %s", i, w, file, name)
		}
	}
}

// A project without tests/ is an empty default set, not an error.
func TestDiscoverTestsNoDir(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":     manifest,
		"src/main.we": "pub type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n",
	})
	chdir(t, dir)
	got, code := discoverTests(".")
	if code != exitOK || len(got) != 0 {
		t.Fatalf("want clean empty set, got %d tests, code %d", len(got), code)
	}
}

func TestFilterMatching(t *testing.T) {
	ts := []discoveredTest{
		{File: "tests/m_test.we", Name: "alpha"},
		{File: "tests/m_test.we", Name: "beta"},
		{File: "tests/m_test.we", Name: "gamma beta"},
	}
	got, code := filterTests("beta", ts)
	if code != exitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	if len(got) != 2 || got[0].Name != "beta" || got[1].Name != "gamma beta" {
		t.Fatalf("unanchored substring semantics: got %+v", got)
	}
	got, code = filterTests("^be", ts)
	if code != exitOK || len(got) != 1 || got[0].Name != "beta" {
		t.Fatalf("anchor is the pattern's business: got %+v code %d", got, code)
	}
	got, code = filterTests("nosuch", ts)
	if code != exitOK || len(got) != 0 {
		t.Fatalf("empty match is an empty run: got %+v code %d", got, code)
	}
	if _, code := filterTests("a(b", ts); code != exitUsage {
		t.Fatalf("invalid pattern is a usage error, got code %d", code)
	}
}

func TestMapTestExit(t *testing.T) {
	if code := mapTestExit(nil); code != exitOK {
		t.Fatalf("clean run: want 0, got %d", code)
	}
	if err := exec.Command("sh", "-c", "exit 1").Run(); err == nil {
		t.Fatal("exit 1 probe did not fail")
	} else if code := mapTestExit(err); code != 1 {
		t.Fatalf("failure exit: want 1, got %d", code)
	}
	if err := exec.Command("sh", "-c", "kill -ABRT $$").Run(); err == nil {
		t.Fatal("abort probe did not fail")
	} else if code := mapTestExit(err); code != 1 {
		t.Fatalf("abort: want 1, got %d", code)
	}
}
