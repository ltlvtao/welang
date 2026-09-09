package deps

import (
	"os"
	"path/filepath"
	"testing"
)

// M13 (dependencies) package tests (design D1–D5): the version and
// constraint grammars, the manifest dependency-table lift, MVS resolution
// with its two failure codes, the lock round-trip and win/stale/tamper
// states, and the digest the lock records. The exact digest and lock bytes
// pinned here are the same ones the T1 conformance goldens assert — one
// algorithm, two witnesses. Written test-first: red today because the
// package does not exist yet.

// --- version (D1) ---

func TestVersionParse(t *testing.T) {
	good := map[string]Version{
		"0.0.0": {0, 0, 0}, "1.2.3": {1, 2, 3}, "10.20.30": {10, 20, 30},
	}
	for s, want := range good {
		v, ok := ParseVersion(s)
		if !ok || v != want {
			t.Fatalf("ParseVersion(%q) = %v,%v; want %v,true", s, v, ok, want)
		}
		if v.String() != s {
			t.Fatalf("String round-trip: %q -> %q", s, v.String())
		}
	}
	for _, s := range []string{"", "1.2", "1.2.3.4", "01.2.3", "1.02.3", "v1.2.3", "1.2.x", " 1.2.3", "-1.2.3"} {
		if _, ok := ParseVersion(s); ok {
			t.Fatalf("ParseVersion(%q) accepted", s)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	// Full numeric order across every component, no string-order traps.
	seq := []Version{{0, 9, 9}, {1, 0, 0}, {1, 0, 1}, {1, 2, 0}, {1, 10, 0}, {2, 0, 0}, {10, 0, 0}}
	for i := 1; i < len(seq); i++ {
		if Compare(seq[i-1], seq[i]) >= 0 {
			t.Fatalf("want %v < %v", seq[i-1], seq[i])
		}
		if Compare(seq[i], seq[i-1]) <= 0 {
			t.Fatalf("want antisymmetry at %v/%v", seq[i], seq[i-1])
		}
	}
	if Compare(Version{1, 2, 3}, Version{1, 2, 3}) != 0 {
		t.Fatal("equal versions must compare 0")
	}
}

// --- constraint (D1): exactly four forms ---

func TestConstraintParse(t *testing.T) {
	for _, s := range []string{"^1.2.0", "~1.2.0", ">=1.1.0", "=1.0.0"} {
		c, ok := ParseConstraint(s)
		if !ok || c.String() != s {
			t.Fatalf("ParseConstraint(%q) = %v,%v; want ok with round-trip", s, c, ok)
		}
	}
	// Everything else is E2003's grammar: a bare version, an unknown
	// operator, a short operand, a leading zero, a prefix decoration.
	for _, s := range []string{"", "1.2.3", ">1.2.3", "<1.2.3", "^1.2", ">=1", "=01.2.3", "^01.2.3", "v=1.2.3", "^ 1.2.3", "^1.2.3.4"} {
		if _, ok := ParseConstraint(s); ok {
			t.Fatalf("ParseConstraint(%q) accepted", s)
		}
	}
}

func TestConstraintFloorsCeilings(t *testing.T) {
	cases := []struct {
		c       string
		floor   string
		ceil    string
		hasCeil bool
	}{
		{"^1.2.0", "1.2.0", "2.0.0", true},
		{"^0.1.0", "0.1.0", "1.0.0", true}, // no zero-major special case: same major uniformly (R2)
		{"~1.2.0", "1.2.0", "1.3.0", true},
		{">=1.1.0", "1.1.0", "", false},
		{"=1.0.0", "1.0.0", "1.0.0", true},
	}
	for _, tc := range cases {
		c, _ := ParseConstraint(tc.c)
		if got := c.Floor().String(); got != tc.floor {
			t.Fatalf("%s floor = %s; want %s", tc.c, got, tc.floor)
		}
		ceil, ok := c.Ceiling()
		if ok != tc.hasCeil || (ok && ceil.String() != tc.ceil) {
			t.Fatalf("%s ceiling = %s,%v; want %s,%v", tc.c, ceil, ok, tc.ceil, tc.hasCeil)
		}
	}
}

func TestConstraintAllows(t *testing.T) {
	must := func(s string) Constraint {
		c, ok := ParseConstraint(s)
		if !ok {
			t.Fatalf("fixture %q", s)
		}
		return c
	}
	// The ceiling is exclusive; the floor inclusive; >= has no top.
	check := func(c Constraint, v string, want bool) {
		t.Helper()
		ver, _ := ParseVersion(v)
		if got := c.Allows(ver); got != want {
			t.Fatalf("%s.Allows(%s) = %v", c.String(), v, got)
		}
	}
	caret := must("^1.2.0")
	check(caret, "1.2.0", true)
	check(caret, "1.9.9", true)
	check(caret, "2.0.0", false)
	check(caret, "1.1.9", false)
	tilde := must("~1.2.0")
	check(tilde, "1.2.9", true)
	check(tilde, "1.3.0", false)
	ge := must(">=1.1.0")
	check(ge, "1.1.0", true)
	check(ge, "99.0.0", true)
	check(ge, "1.0.9", false)
	eq := must("=1.0.0")
	check(eq, "1.0.0", true)
	check(eq, "1.0.1", false)
}

// --- ReadDeps (D1): E2006 keys, E2003 values ---

func TestReadDeps(t *testing.T) {
	m := map[string]string{
		"name":              "demo",
		"dependencies.some": "^1.2.0",
		"dependencies.left": "~0.3.1",
	}
	table, fault := ReadDeps(m)
	if fault != nil {
		t.Fatalf("clean table faulted: %+v", fault)
	}
	if len(table) != 2 || table["some"].String() != "^1.2.0" || table["left"].String() != "~0.3.1" {
		t.Fatalf("table lift: %v", table)
	}
	// Non-dependency keys never surface.
	if _, ok := table["name"]; ok {
		t.Fatal("section leakage")
	}
}

func TestReadDepsE2006(t *testing.T) {
	for _, key := range []string{"MyLib", "my_lib", "my.lib", "", "std"} {
		m := map[string]string{"dependencies." + key: "^1.0.0"}
		_, fault := ReadDeps(m)
		if fault == nil || fault.Code != "E2006" || fault.Key != key {
			t.Fatalf("key %q: got %+v; want E2006 naming the key", key, fault)
		}
	}
	// Digits and hyphens are legal package-name characters.
	for _, key := range []string{"left-pad", "x9", "a"} {
		if _, fault := ReadDeps(map[string]string{"dependencies." + key: "^1.0.0"}); fault != nil {
			t.Fatalf("legal key %q faulted: %+v", key, fault)
		}
	}
}

func TestReadDepsE2003(t *testing.T) {
	for _, val := range []string{"1.2.3", ">1.2.3", "^1.2", "^01.2.3", "", "latest"} {
		m := map[string]string{"dependencies.foo": val}
		_, fault := ReadDeps(m)
		if fault == nil || fault.Code != "E2003" || fault.Key != "foo" || fault.Val != val {
			t.Fatalf("value %q: got %+v; want E2003 naming key and value", val, fault)
		}
	}
}

func TestReadDepsFirstFaultSorted(t *testing.T) {
	// Two faults in one table: the first by dependency-name order reports,
	// deterministically (the E-face renders one diagnostic, not a scatter).
	m := map[string]string{
		"dependencies.zzz": "1.2.3",
		"dependencies.aaa": "bogus",
	}
	_, fault := ReadDeps(m)
	if fault == nil || fault.Key != "aaa" {
		t.Fatalf("want sorted-first fault aaa, got %+v", fault)
	}
}

// --- resolution (D2/D3) ---

// fakeRegistry answers from maps, standing in for the directory tree.
type fakeRegistry struct {
	versions map[string][]string
	deps     map[string]Table
}

func (f fakeRegistry) Versions(name string) []Version {
	var out []Version
	for _, s := range f.versions[name] {
		v, _ := ParseVersion(s)
		out = append(out, v)
	}
	return out
}

func (f fakeRegistry) Deps(name string, v Version) (Table, bool) {
	table, ok := f.deps[name+"@"+v.String()]
	return table, ok
}

func mustTable(t *testing.T, entries map[string]string) Table {
	t.Helper()
	table, fault := ReadDeps(entries)
	if fault != nil {
		t.Fatalf("fixture table: %+v", fault)
	}
	return table
}

func TestResolvePicksMaxFloorNotLatest(t *testing.T) {
	// The chapter's property: picks are a function of the manifests alone.
	// Nothing demands more than 1.1.0, so a published 1.2.0 changes no
	// pick — registry-state independence.
	reg := fakeRegistry{
		versions: map[string][]string{"a": {"1.0.0"}, "b": {"1.1.0", "1.2.0"}},
		deps: map[string]Table{
			"a@1.0.0": mustTable(t, map[string]string{"dependencies.b": ">=1.1.0"}),
			"b@1.1.0": {},
			"b@1.2.0": {},
		},
	}
	root := mustTable(t, map[string]string{
		"dependencies.a": "^1.0.0",
		"dependencies.b": ">=1.1.0",
	})
	picks, conflict := Resolve(root, reg)
	if conflict != nil {
		t.Fatalf("conflict: %+v", conflict)
	}
	if picks["a"].String() != "1.0.0" || picks["b"].String() != "1.1.0" {
		t.Fatalf("picks = %v; want a=1.0.0 b=1.1.0 (not the 1.2.0 in source)", picks)
	}
}

func TestResolveTransitiveRaise(t *testing.T) {
	// The chapter's example: root holds a ^1.0.0 and b ~1.2.0, a@1.0.0
	// demands b >=1.1.0 — the floors meet at b 1.2.0.
	reg := fakeRegistry{
		versions: map[string][]string{"a": {"1.0.0"}, "b": {"1.1.0", "1.2.0"}},
		deps: map[string]Table{
			"a@1.0.0": mustTable(t, map[string]string{"dependencies.b": ">=1.1.0"}),
			"b@1.1.0": {},
			"b@1.2.0": {},
		},
	}
	root := mustTable(t, map[string]string{
		"dependencies.a": "^1.0.0",
		"dependencies.b": "~1.2.0",
	})
	picks, conflict := Resolve(root, reg)
	if conflict != nil {
		t.Fatalf("conflict: %+v", conflict)
	}
	if picks["b"].String() != "1.2.0" {
		t.Fatalf("b pick = %v; want 1.2.0", picks["b"])
	}
}

func TestResolveDeterministic(t *testing.T) {
	// A conflict graph resolved repeatedly must name the same violation
	// every time — the report is the first fault in package-name order,
	// never an artifact of map iteration.
	reg := fakeRegistry{
		versions: map[string][]string{"a": {"1.0.0"}, "b": {"2.0.0"}, "c": {"1.0.0"}},
		deps: map[string]Table{
			"a@1.0.0": mustTable(t, map[string]string{"dependencies.b": "^2.0.0"}),
			"b@2.0.0": {},
			"c@1.0.0": mustTable(t, map[string]string{"dependencies.b": "^2.0.0"}),
		},
	}
	root := mustTable(t, map[string]string{
		"dependencies.a": "^1.0.0",
		"dependencies.b": "=1.0.0",
		"dependencies.c": "^1.0.0",
	})
	var first *Conflict
	for i := 0; i < 100; i++ {
		_, conflict := Resolve(root, reg)
		if conflict == nil {
			t.Fatal("want a conflict")
		}
		if first == nil {
			first = conflict
			continue
		}
		if *conflict != *first {
			t.Fatalf("nondeterministic conflict: %+v vs %+v", conflict, first)
		}
	}
}

func TestResolveE2001(t *testing.T) {
	reg := fakeRegistry{versions: map[string][]string{}, deps: map[string]Table{}}
	root := mustTable(t, map[string]string{"dependencies.left-pad": "^1.0.0"})
	_, conflict := Resolve(root, reg)
	if conflict == nil || conflict.Code != "E2001" || conflict.Name != "left-pad" {
		t.Fatalf("got %+v; want E2001 naming left-pad", conflict)
	}
}

func TestResolveE2002Ceiling(t *testing.T) {
	// The chapter's scenario: root pins b =1.0.0 while a@1.0.0 demands
	// ^2.0.0 — the pick rises to 2.0.0 and violates the pin. The report
	// carries the violated constraint, its chain to root, and the raiser.
	reg := fakeRegistry{
		versions: map[string][]string{"a": {"1.0.0"}, "b": {"1.0.0", "2.0.0"}},
		deps: map[string]Table{
			"a@1.0.0": mustTable(t, map[string]string{"dependencies.b": "^2.0.0"}),
			"b@2.0.0": {},
		},
	}
	root := mustTable(t, map[string]string{
		"dependencies.a": "^1.0.0",
		"dependencies.b": "=1.0.0",
	})
	picks, conflict := Resolve(root, reg)
	if conflict == nil {
		t.Fatalf("no conflict; picks %v", picks)
	}
	if conflict.Code != "E2002" || conflict.Name != "b" || conflict.Pick.String() != "2.0.0" {
		t.Fatalf("conflict identity: %+v", conflict)
	}
	if conflict.Con.String() != "=1.0.0" || conflict.Chain != "root" {
		t.Fatalf("violated constraint/chain: %+v", conflict)
	}
	if conflict.Raised.String() != "^2.0.0" || conflict.Raiser != "a@1.0.0" {
		t.Fatalf("raiser: %+v", conflict)
	}
}

func TestResolveE2002CeilingChain(t *testing.T) {
	// One level deeper: the violated constraint sits on b@1.0.0, reached
	// root -> a@1.0.0 -> b@1.0.0; the chain renders with every hop, and
	// d@1.0.0 is the raiser.
	reg := fakeRegistry{
		versions: map[string][]string{"a": {"1.0.0"}, "b": {"1.0.0"}, "c": {"1.0.0", "2.0.0"}, "d": {"1.0.0"}},
		deps: map[string]Table{
			"a@1.0.0": mustTable(t, map[string]string{"dependencies.b": "^1.0.0"}),
			"b@1.0.0": mustTable(t, map[string]string{"dependencies.c": "=1.0.0"}),
			"c@2.0.0": {},
			"d@1.0.0": mustTable(t, map[string]string{"dependencies.c": "^2.0.0"}),
		},
	}
	root := mustTable(t, map[string]string{
		"dependencies.a": "^1.0.0",
		"dependencies.d": "^1.0.0",
	})
	_, conflict := Resolve(root, reg)
	if conflict == nil || conflict.Name != "c" {
		t.Fatalf("got %+v; want the c violation", conflict)
	}
	if conflict.Con.String() != "=1.0.0" || conflict.Chain != "root -> a@1.0.0 -> b@1.0.0" {
		t.Fatalf("violated constraint/chain: %q / %q", conflict.Con.String(), conflict.Chain)
	}
	if conflict.Raised.String() != "^2.0.0" || conflict.Raiser != "d@1.0.0" {
		t.Fatalf("raiser: %q from %s", conflict.Raised.String(), conflict.Raiser)
	}
}

func TestResolveE2002Missing(t *testing.T) {
	// The floor's maximum is a version no source holds.
	reg := fakeRegistry{
		versions: map[string][]string{"c": {"1.4.0"}},
		deps:     map[string]Table{"c@1.4.0": {}},
	}
	root := mustTable(t, map[string]string{"dependencies.c": ">=1.5.0"})
	_, conflict := Resolve(root, reg)
	if conflict == nil || conflict.Code != "E2002" || conflict.Name != "c" {
		t.Fatalf("got %+v; want E2002 naming c", conflict)
	}
	if conflict.Pick.String() != "1.5.0" || conflict.Demand.String() != ">=1.5.0" || conflict.Dep != "root" {
		t.Fatalf("missing-version fields: %+v", conflict)
	}
}

// --- directory registry (D2) ---

// writePkg lays one package version into root the registry shape pins:
// <root>/<name>/<version>/{we.toml, src/...}.
func writePkg(t *testing.T, root, name, ver, dep string, srcs map[string]string) string {
	t.Helper()
	pkgDir := filepath.Join(root, name, ver)
	toml := "name = \"" + name + "\"\nversion = \"" + ver + "\"\ntype = \"library\"\n"
	if dep != "" {
		toml += "\n[dependencies]\n" + dep + "\n"
	}
	files := map[string]string{"we.toml": toml}
	for k, v := range srcs {
		files[k] = v
	}
	for rel, content := range files {
		p := filepath.Join(pkgDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return pkgDir
}

const someSrc = "pub fn answer() -> Int64 {\n    return 42\n}\n"

// The T1 goldens' fixture digest — one algorithm, two witnesses: this
// constant must equal the we.lock digests the conformance cases assert.
const someDigest = "sha256-c941aa369df38186d84ac9515bbf67a7c2694ca29594e111778254a5618413b3"

func TestDirRegistry(t *testing.T) {
	reg := NewDirRegistry(filepath.Join(t.TempDir(), "registry-absent"))
	if vs := reg.Versions("some"); len(vs) != 0 {
		t.Fatalf("absent registry must answer empty, got %v", vs)
	}
	root := t.TempDir()
	writePkg(t, root, "some", "1.2.0", "", map[string]string{"src/lib/util.we": someSrc})
	writePkg(t, root, "some", "1.3.0", "", nil)
	writePkg(t, root, "some", "not-a-version", "", nil) // skipped, not a version
	reg = NewDirRegistry(root)
	vs := reg.Versions("some")
	if len(vs) != 2 || vs[0].String() != "1.2.0" || vs[1].String() != "1.3.0" {
		t.Fatalf("versions = %v; want ascending [1.2.0 1.3.0]", vs)
	}
	table, ok := reg.Deps("some", vs[0])
	if !ok || len(table) != 0 {
		t.Fatalf("deps of a dep-free package: %v %v", table, ok)
	}
	writePkg(t, root, "a", "1.0.0", "b = \">=1.1.0\"", nil)
	av, _ := ParseVersion("1.0.0")
	table, ok = reg.Deps("a", av)
	if !ok || table["b"].String() != ">=1.1.0" {
		t.Fatalf("deps of a@1.0.0: %v %v", table, ok)
	}
}

// --- digest and acquisition (D5) ---

func TestDigest(t *testing.T) {
	pkgDir := writePkg(t, t.TempDir(), "some", "1.2.0", "", map[string]string{"src/lib/util.we": someSrc})
	got, err := Digest(pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != someDigest {
		t.Fatalf("digest = %s; want the golden fixture value %s", got, someDigest)
	}
}

func TestAcquire(t *testing.T) {
	regRoot, cacheRoot := t.TempDir(), t.TempDir()
	src := writePkg(t, regRoot, "some", "1.2.0", "", map[string]string{"src/lib/util.we": someSrc})
	v, _ := ParseVersion("1.2.0")
	if err := Acquire(regRoot, cacheRoot, "some", v); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(cacheRoot, "some", "1.2.0")
	for _, rel := range []string{"we.toml", "src/lib/util.we"} {
		want, _ := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil || string(got) != string(want) {
			t.Fatalf("acquired %s differs: %v", rel, err)
		}
	}
	// Idempotent: a second acquisition of the same version succeeds.
	if err := Acquire(regRoot, cacheRoot, "some", v); err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	d, err := Digest(dst)
	if err != nil || d != someDigest {
		t.Fatalf("cache digest = %s %v; want %s", d, err, someDigest)
	}
}

// --- lock (D4) ---

func TestLockRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "we.lock")
	entries := []LockEntry{
		{Name: "b", Version: Version{1, 2, 0}, Digest: "sha256-bb"},
		{Name: "a", Version: Version{1, 0, 0}, Digest: "sha256-aa"},
	}
	if err := WriteLock(path, entries); err != nil {
		t.Fatal(err)
	}
	// Machine-written bytes: the comment line, name-sorted [[package]]
	// blocks, one blank line between sections, single trailing newline.
	want := "# We lockfile — machine-written by resolution; commit it, do not edit.\n\n" +
		"[[package]]\nname = \"a\"\nversion = \"1.0.0\"\ndigest = \"sha256-aa\"\n\n" +
		"[[package]]\nname = \"b\"\nversion = \"1.2.0\"\ndigest = \"sha256-bb\"\n"
	raw, _ := os.ReadFile(path)
	if string(raw) != want {
		t.Fatalf("lock bytes:\n got %q\nwant %q", raw, want)
	}
	got, ok := ReadLock(path)
	if !ok || len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" ||
		got[0].Version.String() != "1.0.0" || got[0].Digest != "sha256-aa" {
		t.Fatalf("round trip: %v %v", got, ok)
	}
	// A malformed lock is unreadable — the caller regenerates silently,
	// no diagnostic (chapter 22 R4).
	if err := os.WriteFile(path, []byte("this is not [[toml\nbroken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadLock(path); ok {
		t.Fatal("malformed lock must not read")
	}
}

func TestLockStatusWins(t *testing.T) {
	cacheRoot := t.TempDir()
	pkgDir := writePkg(t, cacheRoot, "some", "1.2.0", "", map[string]string{"src/lib/util.we": someSrc})
	root := mustTable(t, map[string]string{"dependencies.some": "^1.2.0"})
	entries := []LockEntry{{Name: "some", Version: Version{1, 2, 0}, Digest: someDigest}}
	state, roots, flaw := LockStatus(root, entries, cacheRoot)
	if state != LockWins || flaw != nil {
		t.Fatalf("state = %v flaw = %+v; want LockWins", state, flaw)
	}
	if roots["some"] != pkgDir {
		t.Fatalf("roots = %v; want some -> %s", roots, pkgDir)
	}
}

func TestLockStatusWinsTransitive(t *testing.T) {
	// The closure walks cached manifests: a's locked manifest drags b in,
	// and both cache entries must be present with matching digests.
	cacheRoot := t.TempDir()
	writePkg(t, cacheRoot, "a", "1.0.0", "b = \">=1.1.0\"", nil)
	writePkg(t, cacheRoot, "b", "1.2.0", "", nil)
	root := mustTable(t, map[string]string{"dependencies.a": "^1.0.0"})
	da, _ := Digest(filepath.Join(cacheRoot, "a", "1.0.0"))
	db, _ := Digest(filepath.Join(cacheRoot, "b", "1.2.0"))
	entries := []LockEntry{
		{Name: "a", Version: Version{1, 0, 0}, Digest: da},
		{Name: "b", Version: Version{1, 2, 0}, Digest: db},
	}
	state, _, flaw := LockStatus(root, entries, cacheRoot)
	if state != LockWins || flaw != nil {
		t.Fatalf("state = %v flaw = %+v; want LockWins", state, flaw)
	}
}

func TestLockStatusStale(t *testing.T) {
	cacheRoot := t.TempDir()
	writePkg(t, cacheRoot, "some", "1.2.0", "", map[string]string{"src/lib/util.we": someSrc})
	root := mustTable(t, map[string]string{"dependencies.some": "^2.0.0"}) // constraint drifted
	entries := []LockEntry{{Name: "some", Version: Version{1, 2, 0}, Digest: someDigest}}
	if state, _, flaw := LockStatus(root, entries, cacheRoot); state != LockStale || flaw != nil {
		t.Fatalf("constraint drift: state %v flaw %+v; want LockStale", state, flaw)
	}
	// A cold cache cannot witness the lock either — regenerate.
	entries[0].Version = Version{2, 0, 0}
	if state, _, _ := LockStatus(root, entries, t.TempDir()); state != LockStale {
		t.Fatalf("cold cache: state %v; want LockStale", state)
	}
}

func TestLockStatusTampered(t *testing.T) {
	cacheRoot := t.TempDir()
	writePkg(t, cacheRoot, "some", "1.2.0", "", map[string]string{
		"src/lib/util.we": "pub fn answer() -> Int64 {\n    return 99\n}\n",
	})
	root := mustTable(t, map[string]string{"dependencies.some": "^1.2.0"})
	entries := []LockEntry{{Name: "some", Version: Version{1, 2, 0}, Digest: someDigest}}
	state, _, flaw := LockStatus(root, entries, cacheRoot)
	if state != LockTampered || flaw == nil {
		t.Fatalf("state %v flaw %+v; want LockTampered with the mismatch", state, flaw)
	}
	want := "sha256-433688e99b4c77269e4dad54c9720cec73a3b189af9ed5582e64461a9d91d85f"
	if flaw.Name != "some" || flaw.Version.String() != "1.2.0" || flaw.Got != want || flaw.Want != someDigest {
		t.Fatalf("integrity fields: %+v", flaw)
	}
}
