package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/deps"
)

// M13 (dependencies) wiring tests (design D6): the prepareDeps
// orchestrator — acquire-and-lock, the offline lock-win path, the E2005
// and E2002 report faces — the loader's cache-leg mapping, and the two
// command-level exemptions: fmt validates the declared form but never
// resolves, and the test face maps dependency diagnostics to its exit 2.
// The digests and message texts pinned here are the same ones the T1
// conformance goldens assert. Written test-first: red today on the
// undefined symbols alone.

// The golden fixture family: some@1.2.0 answers 42, its tampered twin 99.
const (
	m13SomeSrc        = "pub fn answer() -> Int64 {\n    return 42\n}\n"
	m13TamperedSrc    = "pub fn answer() -> Int64 {\n    return 99\n}\n"
	m13DigestSome     = "sha256-c941aa369df38186d84ac9515bbf67a7c2694ca29594e111778254a5618413b3"
	m13DigestTampered = "sha256-433688e99b4c77269e4dad54c9720cec73a3b189af9ed5582e64461a9d91d85f"
)

const m13LockSome = "# We lockfile — machine-written by resolution; commit it, do not edit.\n\n" +
	"[[package]]\nname = \"some\"\nversion = \"1.2.0\"\ndigest = \"" + m13DigestSome + "\"\n"

// m13Registry lays the registry fixture family (design D8): some 1.2.0 and
// 1.3.0, so a passing pick of 1.2.0 proves the resolver does not simply
// take the newest source version.
func m13Registry(t *testing.T) string {
	t.Helper()
	reg := t.TempDir()
	writeTree(t, reg, map[string]string{
		"some/1.2.0/we.toml":         "name = \"some\"\nversion = \"1.2.0\"\ntype = \"library\"\n",
		"some/1.2.0/src/lib/util.we": m13SomeSrc,
		"some/1.3.0/we.toml":         "name = \"some\"\nversion = \"1.3.0\"\ntype = \"library\"\n",
		"some/1.3.0/src/lib/util.we": "pub fn answer() -> Int64 {\n    return 43\n}\n",
	})
	return reg
}

func m13Table(t *testing.T, entries map[string]string) deps.Table {
	t.Helper()
	table, fault := deps.ReadDeps(entries)
	if fault != nil {
		t.Fatalf("fixture table: %+v", fault)
	}
	return table
}

func TestPrepareDepsAcquiresAndLocks(t *testing.T) {
	reg, cache, proj := m13Registry(t), t.TempDir(), t.TempDir()
	t.Setenv("WE_REGISTRY", reg)
	t.Setenv("WE_CACHE", cache)
	var out, errb bytes.Buffer
	e := &env{stdout: &out, stderr: &errb}
	roots, code := e.prepareDeps(proj, m13Table(t, map[string]string{"dependencies.some": "^1.2.0"}))
	if code != exitOK {
		t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
	}
	if want := filepath.Join(cache, "some", "1.2.0"); roots["some"] != want {
		t.Fatalf("roots = %v; want some -> %s", roots, want)
	}
	// The pick is 1.2.0 even though 1.3.0 sits in the source; the lock
	// records it with the digest the goldens pin.
	raw, err := os.ReadFile(filepath.Join(proj, "we.lock"))
	if err != nil || string(raw) != m13LockSome {
		t.Fatalf("we.lock = %q %v; want the golden bytes", raw, err)
	}
	src, err := os.ReadFile(filepath.Join(cache, "some", "1.2.0", "src", "lib", "util.we"))
	if err != nil || string(src) != m13SomeSrc {
		t.Fatalf("cache copy: %v", err)
	}
}

func TestPrepareDepsLockWinsOffline(t *testing.T) {
	// A satisfying lock plus a warm cache answers without touching any
	// source: the registry points at a directory that does not exist.
	cache, proj := t.TempDir(), t.TempDir()
	writeTree(t, cache, map[string]string{
		"some/1.2.0/we.toml":         "name = \"some\"\nversion = \"1.2.0\"\ntype = \"library\"\n",
		"some/1.2.0/src/lib/util.we": m13SomeSrc,
	})
	writeTree(t, proj, map[string]string{"we.lock": m13LockSome})
	t.Setenv("WE_REGISTRY", filepath.Join(t.TempDir(), "registry-absent"))
	t.Setenv("WE_CACHE", cache)
	var out, errb bytes.Buffer
	e := &env{stdout: &out, stderr: &errb}
	roots, code := e.prepareDeps(proj, m13Table(t, map[string]string{"dependencies.some": "^1.2.0"}))
	if code != exitOK {
		t.Fatalf("want offline lock-win exit 0, got %d (stderr %q)", code, errb.String())
	}
	if want := filepath.Join(cache, "some", "1.2.0"); roots["some"] != want {
		t.Fatalf("roots = %v; want some -> %s", roots, want)
	}
}

func TestPrepareDepsE2005(t *testing.T) {
	// The lock wins the race but the cached content hashes away from it.
	reg, cache, proj := m13Registry(t), t.TempDir(), t.TempDir()
	writeTree(t, cache, map[string]string{
		"some/1.2.0/we.toml":         "name = \"some\"\nversion = \"1.2.0\"\ntype = \"library\"\n",
		"some/1.2.0/src/lib/util.we": m13TamperedSrc,
	})
	writeTree(t, proj, map[string]string{"we.lock": m13LockSome})
	t.Setenv("WE_REGISTRY", reg)
	t.Setenv("WE_CACHE", cache)
	var out, errb bytes.Buffer
	e := &env{stdout: &out, stderr: &errb}
	_, code := e.prepareDeps(proj, m13Table(t, map[string]string{"dependencies.some": "^1.2.0"}))
	if code != exitDiagnostic {
		t.Fatalf("want exit 1, got %d", code)
	}
	want := "we.lock:1:1: error[E2005]: lockfile integrity mismatch — " +
		"cached \"some\" 1.2.0 hashes to " + m13DigestTampered +
		", we.lock records " + m13DigestSome
	if errb.String() != want+"\n" {
		t.Fatalf("stderr:\nwant %q\ngot  %q", want, errb.String())
	}
}

func TestPrepareDepsE2002Rendering(t *testing.T) {
	// The chapter's ceiling scenario, message and all: the pick, the
	// violated constraint, its chain to root, and the raiser.
	reg, cache, proj := t.TempDir(), t.TempDir(), t.TempDir()
	writeTree(t, reg, map[string]string{
		"a/1.0.0/we.toml":      "name = \"a\"\nversion = \"1.0.0\"\ntype = \"library\"\n\n[dependencies]\nb = \"^2.0.0\"\n",
		"a/1.0.0/src/lib/a.we": "pub fn mark() -> Int64 {\n    return 1\n}\n",
		"b/2.0.0/we.toml":      "name = \"b\"\nversion = \"2.0.0\"\ntype = \"library\"\n",
		"b/2.0.0/src/lib/b.we": "pub fn mark() -> Int64 {\n    return 20\n}\n",
	})
	t.Setenv("WE_REGISTRY", reg)
	t.Setenv("WE_CACHE", cache)
	var out, errb bytes.Buffer
	e := &env{stdout: &out, stderr: &errb}
	_, code := e.prepareDeps(proj, m13Table(t, map[string]string{
		"dependencies.a": "^1.0.0",
		"dependencies.b": "=1.0.0",
	}))
	if code != exitDiagnostic {
		t.Fatalf("want exit 1, got %d", code)
	}
	want := "we.toml:1:1: error[E2002]: unsatisfiable dependency constraint — " +
		"package \"b\" resolves to 2.0.0, violating \"=1.0.0\" required along root; " +
		"the pick was raised by \"^2.0.0\" from a@1.0.0"
	if errb.String() != want+"\n" {
		t.Fatalf("stderr:\nwant %q\ngot  %q", want, errb.String())
	}
}

func TestDepModulePath(t *testing.T) {
	dirs := map[string]string{"some": filepath.Join("cache", "some", "1.2.0")}
	if p, ok := depModulePath(dirs, "some.lib.util"); !ok ||
		p != filepath.Join("cache", "some", "1.2.0", "src", "lib", "util.we") {
		t.Fatalf("cache leg: %q %v", p, ok)
	}
	// Undeclared first segments and std never take the cache leg.
	for _, imp := range []string{"other.lib.util", "std.io", "some"} {
		if _, ok := depModulePath(dirs, imp); ok {
			t.Fatalf("%q took the cache leg", imp)
		}
	}
}

// fmt reads the manifest's declared form (the validations hold for every
// reader) but never resolves: an empty source universe cannot make it
// fail, and the source still reformats.
func TestFmtSkipsResolution(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":     "name = \"demo\"\nversion = \"0.1.0\"\ntype = \"executable\"\n\n[dependencies]\nsome = \"^1.2.0\"\n",
		"src/main.we": "pub type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n",
	})
	t.Setenv("WE_REGISTRY", t.TempDir()) // an empty universe
	t.Setenv("WE_CACHE", t.TempDir())
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"fmt", "."}, &out, &errb); code != exitOK {
		t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "src", "main.we"))
	if strings.Contains(string(raw), "    return") {
		t.Fatalf("source not reformatted: %q", raw)
	}
}

// The test face keeps its M10b gate: a dependency diagnostic from the
// shared loader is the compile failure's exit 2, not the diagnostic face's 1.
func TestTestFaceMapsDepsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":         "name = \"demo\"\nversion = \"0.1.0\"\ntype = \"executable\"\n\n[dependencies]\nfoo = \"1.2.3\"\n",
		"tests/t_test.we": "import std.test as st\n\ntest \"t\" {\n    st.assertTrue(true)\n}\n",
	})
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"test", "."}, &out, &errb); code != exitCompileFailure {
		t.Fatalf("want exit 2, got %d (stderr %q)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "error[E2003]") {
		t.Fatalf("want the E2003 face, got %q", errb.String())
	}
}
