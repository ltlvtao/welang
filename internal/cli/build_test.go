package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A fake clang whose --version output is scripted; the pinned-version gate
// must accept the pin and reject everything else, parsing the x.y.z out of
// the `clang version ...` line without the distro suffix.
func TestClangVersionGate(t *testing.T) {
	write := func(body string) string {
		dir := t.TempDir()
		p := filepath.Join(dir, "clang")
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name    string
		version string
		ok      bool
	}{
		{"exact pin", "echo 'Ubuntu clang version 21.1.8 (6ubuntu1)'\n", true},
		{"bare pin", "echo 'clang version 21.1.8'\n", true},
		{"newer major", "echo 'clang version 22.0.0'\n", false},
		{"older patch", "echo 'clang version 21.1.7'\n", false},
		{"no version line", "echo 'some other tool'\n", false},
		{"garbage", "exit 1\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkClangVersion(write(c.version))
			if c.ok && err != nil {
				t.Fatalf("want accept, got: %v", err)
			}
			if !c.ok && err == nil {
				t.Fatalf("want reject, got accept")
			}
		})
	}
	// A missing path is a rejection, not a panic.
	if err := checkClangVersion(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatalf("want reject for missing clang, got accept")
	}
}

// we clean is idempotent and silent on an absent build/, and under
// --verbose lists what it removes in a fixed order.
func TestCleanFaces(t *testing.T) {
	proj := func(tb testing.TB) string {
		dir := tb.TempDir()
		writeTree(tb, dir, map[string]string{
			"we.toml":     "name = \"demo\"\nversion = \"0.1.0\"\ntype = \"executable\"\n",
			"src/main.we": "pub fn main() -> Result<(), E> { return Ok(()) }\n",
		})
		return dir
	}
	t.Run("idempotent silent", func(t *testing.T) {
		dir := proj(t)
		var out, errb bytes.Buffer
		chdir(t, dir)
		if code := Run([]string{"clean", "."}, &out, &errb); code != exitOK {
			t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
		}
		if out.Len() != 0 || errb.Len() != 0 {
			t.Fatalf("want silence, got out %q err %q", out.String(), errb.String())
		}
	})
	t.Run("verbose lists removed paths", func(t *testing.T) {
		dir := proj(t)
		writeTree(t, dir, map[string]string{
			"build/demo":    "binary stub\n",
			"build/demo.ll": "ir stub\n",
		})
		var out, errb bytes.Buffer
		chdir(t, dir)
		if code := Run([]string{"clean", ".", "--verbose"}, &out, &errb); code != exitOK {
			t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
		}
		want := "we: removed build/demo\nwe: removed build/demo.ll\nwe: removed build/\n"
		if out.String() != want {
			t.Fatalf("stdout mismatch:\nwant %q\ngot  %q", want, out.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "build")); !os.IsNotExist(err) {
			t.Fatalf("build/ still present: %v", err)
		}
	})
	// An absent build/ removes an empty set: success, and under --verbose
	// not one line (design D1's pinned face).
	t.Run("verbose silent when absent", func(t *testing.T) {
		dir := proj(t)
		var out, errb bytes.Buffer
		chdir(t, dir)
		if code := Run([]string{"clean", ".", "--verbose"}, &out, &errb); code != exitOK {
			t.Fatalf("want exit 0, got %d (stderr %q)", code, errb.String())
		}
		if out.Len() != 0 || errb.Len() != 0 {
			t.Fatalf("want silence, got out %q err %q", out.String(), errb.String())
		}
	})
}

// Same project built into two different directories yields byte-identical
// binaries on the pinned toolchain (chapter 21's same-input-same-output
// made executable).
func TestBuildDeterministic(t *testing.T) {
	if err := checkClangVersion("clang"); err != nil {
		t.Skipf("pinned clang unavailable: %v", err)
	}
	srcs := map[string]string{
		"we.toml":     "name = \"demo\"\nversion = \"0.1.0\"\ntype = \"executable\"\n",
		"src/main.we": "pub type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    return Err(Failed(\"boom\"))\n}\n",
	}
	var bins [][]byte
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		writeTree(t, dir, srcs)
		var out, errb bytes.Buffer
		chdir(t, dir)
		if code := Run([]string{"build", "."}, &out, &errb); code != exitOK {
			t.Fatalf("build %d failed: code %d stderr %q", i, code, errb.String())
		}
		b, err := os.ReadFile(filepath.Join(dir, "build", "demo"))
		if err != nil {
			t.Fatal(err)
		}
		bins = append(bins, b)
	}
	if !bytes.Equal(bins[0], bins[1]) {
		t.Fatalf("two builds of the same project differ (%d vs %d bytes)", len(bins[0]), len(bins[1]))
	}
}

func writeTree(tb testing.TB, dir string, files map[string]string) {
	tb.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
}

func chdir(tb testing.TB, dir string) {
	tb.Helper()
	old, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { os.Chdir(old) })
}
