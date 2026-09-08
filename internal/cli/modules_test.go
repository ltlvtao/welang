package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The loader's project faces (design D9): the graph walk's own diagnostics —
// module resolution precedes type checking — and the multi-module check's
// clean faces. The conformance goldens pin the same behavior end-to-end;
// these pin the loader against live project directories.

const manifest = "name = \"demo\"\nversion = \"0.1.0\"\ntype = \"executable\"\n"

// A clean two-module project checks silently and counts both modules.
func TestLoadGraphGreenProject(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":     manifest,
		"src/main.we": "import util\n\npub fn main() -> Result<(), AppError> {\n    util.helper()\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
		"src/util.we": "pub fn helper() {\n    return\n}\n",
	})
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"check", "."}, &out, &errb); code != exitOK {
		t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
	}
	if out.Len() != 0 || errb.Len() != 0 {
		t.Fatalf("want silence, got out %q err %q", out.String(), errb.String())
	}
	// --verbose names the module count: the root plus its imports.
	var vout, verr bytes.Buffer
	if code := Run([]string{"check", ".", "--verbose"}, &vout, &verr); code != exitOK {
		t.Fatalf("verbose: want exit 0, got %d (stderr %q)", code, verr.String())
	}
	if got, want := vout.String(), "we: check passed (2 modules)\n"; got != want {
		t.Fatalf("verbose mismatch:\nwant %q\ngot  %q", want, got)
	}
}

// The chain main -> u2 -> u1 checks clean with the cross-module top-level
// initializers — chapter 15's deterministic initialization order is the
// loader's post-order (the imported before the importing).
func TestLoadGraphInitOrder(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":     manifest,
		"src/main.we": "import u1\nimport u2\n\npub fn main() -> Result<(), AppError> {\n    let x = u2.two()\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n\nlet y = u2.two()\n",
		"src/u1.we":   "pub fn one() -> Int64 {\n    return 1\n}\n",
		"src/u2.we":   "import u1\n\npub fn two() -> Int64 {\n    return u1.one() + 1\n}\n\nlet base = u1.one()\n",
	})
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"check", "."}, &out, &errb); code != exitOK {
		t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
	}
	if out.Len() != 0 || errb.Len() != 0 {
		t.Fatalf("want silence, got out %q err %q", out.String(), errb.String())
	}
	var vout, verr bytes.Buffer
	if code := Run([]string{"check", ".", "--verbose"}, &vout, &verr); code != exitOK {
		t.Fatalf("verbose: want exit 0, got %d", code)
	}
	if got, want := vout.String(), "we: check passed (3 modules)\n"; got != want {
		t.Fatalf("verbose mismatch:\nwant %q\ngot  %q", want, got)
	}
}

// A missing dependency module is E1302 naming the expected path, anchored
// at the import's first path segment in whichever module holds the import —
// the root's own imports and a dependency's alike (discovery order).
func TestLoadGraphModuleNotFound(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			"root import",
			map[string]string{
				"we.toml":     manifest,
				"src/main.we": "import util\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
			},
			"src/main.we:1:8: error[E1302]: module not found — the import \"util\" expects the module at src/util.we and no file is there; create the file at the expected path, fix the path spelling, or add the dependency to the cache\n",
		},
		{
			"dependency's import",
			map[string]string{
				"we.toml":     manifest,
				"src/main.we": "import a\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
				"src/a.we":    "import missing\n\npub fn afn() {\n    return\n}\n",
			},
			"src/a.we:1:8: error[E1302]: module not found — the import \"missing\" expects the module at src/missing.we and no file is there; create the file at the expected path, fix the path spelling, or add the dependency to the cache\n",
		},
		{
			"nested path mapping",
			map[string]string{
				"we.toml":     manifest,
				"src/main.we": "import net.http.client\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
			},
			"src/main.we:1:8: error[E1302]: module not found — the import \"net.http.client\" expects the module at src/net/http/client.we and no file is there; create the file at the expected path, fix the path spelling, or add the dependency to the cache\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, c.files)
			var out, errb bytes.Buffer
			chdir(t, dir)
			if code := Run([]string{"check", "."}, &out, &errb); code != exitDiagnostic {
				t.Fatalf("want exit 1, got %d (stderr %q)", code, errb.String())
			}
			if out.Len() != 0 {
				t.Fatalf("want silent stdout, got %q", out.String())
			}
			if errb.String() != c.want {
				t.Fatalf("stderr mismatch:\nwant %q\ngot  %q", c.want, errb.String())
			}
		})
	}
}

// A deep import whose file exists at the mapped path resolves — the path
// mapping a.b.c to src/a/b/c.we, alias and all.
func TestLoadGraphDeepPathGreen(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":                manifest,
		"src/main.we":            "import net.http.client as client\n\npub fn main() -> Result<(), AppError> {\n    client.serve()\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
		"src/net/http/client.we": "pub fn serve() {\n    return\n}\n",
	})
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"check", "."}, &out, &errb); code != exitOK {
		t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
	}
	if out.Len() != 0 || errb.Len() != 0 {
		t.Fatalf("want silence, got out %q err %q", out.String(), errb.String())
	}
}

// An import that closes a cycle is E1301 rendering the cycle, anchored at
// the closing import's first path segment.
func TestLoadGraphCycle(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":     manifest,
		"src/main.we": "import a\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
		"src/a.we":    "import b\n\npub fn afn() {\n    return\n}\n",
		"src/b.we":    "import a\n\npub fn bfn() {\n    return\n}\n",
	})
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"check", "."}, &out, &errb); code != exitDiagnostic {
		t.Fatalf("want exit 1, got %d (stderr %q)", code, errb.String())
	}
	want := "src/b.we:1:8: error[E1301]: circular module dependency — the import closes the cycle a -> b -> a; there are no forward declarations, and the fix is extracting the shared code into a third module both import\n"
	if errb.String() != want {
		t.Fatalf("stderr mismatch:\nwant %q\ngot  %q", want, errb.String())
	}
}

// A std import rides the compiler-provided registry into the graph
// (chapter 15 R1 — the std segment never touches the file system): std.io
// loads as a dependency module, and an unused import checks clean.
func TestLoadGraphStdLoading(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":     manifest,
		"src/main.we": "import std.io\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
	})
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"check", "."}, &out, &errb); code != exitOK {
		t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
	}
	if out.String() != "" || errb.String() != "" {
		t.Fatalf("want a quiet pass, got stdout %q stderr %q", out.String(), errb.String())
	}
}

// An unknown std path is E1302's std form from the loader itself — no
// expected-path clause, the registry is the only mapping.
func TestLoadGraphStdUnknown(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":     manifest,
		"src/main.we": "import std.json\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
	})
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"check", "."}, &out, &errb); code != exitDiagnostic {
		t.Fatalf("want exit 1, got %d (stderr %q)", code, errb.String())
	}
	want := "src/main.we:1:8: error[E1302]: module not found — no standard-library module \"std.json\" exists in this build; the std segment is compiler-provided, so the name is misspelled or the module is not implemented yet\n"
	if errb.String() != want {
		t.Fatalf("stderr mismatch:\nwant %q\ngot  %q", want, errb.String())
	}
}

// The cross-module visibility gate: a non-pub item reached through a
// qualified form is E1303; an undeclared one is E1304 (the type stage's
// own face, riding the graph the loader built).
func TestLoadGraphVisibilityGate(t *testing.T) {
	cases := []struct {
		name  string
		main  string
		dep   string
		first string // the first stderr line's prefix match
	}{
		{
			"non-pub fn through a call head",
			"import b\n\npub fn main() -> Result<(), AppError> {\n    b.helper()\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
			"fn helper() {\n    return\n}\n",
			"src/main.we:4:5: error[E1303]: cross-module use of a module-local item — \"helper\" is declared in \"b\" without pub",
		},
		{
			"undeclared item",
			"import b\n\npub fn main() -> Result<(), AppError> {\n    b.nosuch()\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
			"pub fn helper() {\n    return\n}\n",
			"src/main.we:4:5: error[E1304]: unresolved name — the module \"b\" declares no \"nosuch\"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, map[string]string{
				"we.toml":     manifest,
				"src/main.we": c.main,
				"src/b.we":    c.dep,
			})
			var out, errb bytes.Buffer
			chdir(t, dir)
			if code := Run([]string{"check", "."}, &out, &errb); code != exitDiagnostic {
				t.Fatalf("want exit 1, got %d (stderr %q)", code, errb.String())
			}
			if !strings.HasPrefix(errb.String(), c.first) {
				t.Fatalf("stderr mismatch:\nwant prefix %q\ngot  %q", c.first, errb.String())
			}
		})
	}
}

// A multi-module build rides the M10b program face end to end: the
// dependency's helper fn defines under its module-qualified symbol with
// its slot, the root main is the entry, and the artifact lands — the
// other-functions row this pinned retired with the widening.
func TestLoadGraphBuild(t *testing.T) {
	if err := checkClangVersion("clang"); err != nil {
		t.Skipf("pinned clang unavailable: %v", err)
	}
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":     manifest,
		"src/main.we": "import util\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
		"src/util.we": "pub fn helper() {\n    return\n}\n",
	})
	var out, errb bytes.Buffer
	chdir(t, dir)
	if code := Run([]string{"build", "."}, &out, &errb); code != exitOK {
		t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
	}
	if errb.String() != "" {
		t.Fatalf("clean build carries no stderr, got %q", errb.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "build", "demo")); err != nil {
		t.Fatalf("artifact missing: %v", err)
	}
	ll, err := os.ReadFile(filepath.Join(dir, "build", "demo.ll"))
	if err != nil {
		t.Fatalf("intermediate .ll missing: %v", err)
	}
	for _, want := range []string{
		"define void @util.helper()",
		"@slot.util.helper = global ptr @util.helper",
		"define i32 @__we_main()",
	} {
		if !strings.Contains(string(ll), want) {
			t.Fatalf("IR missing %q:\n%s", want, ll)
		}
	}
}
