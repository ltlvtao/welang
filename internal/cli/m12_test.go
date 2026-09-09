package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ltlvtao/welang/internal/codegen"
)

// M12 (ffi) link-tower tests (design D5): native/ discovery in byte
// order, the llvm-nm pinned-version gate (the clang gate's precedent),
// the defined-symbol listing, and the declared-vs-defined difference
// the E1906 report renders. The end-to-end faces (build/run exit 1 with
// no artifact, check never linking) are pinned by the T1 conformance
// goldens and the T7 black-box battery. Written test-first: red today
// on the undefined symbols alone.

// native/: .c files in byte order; an absent directory is an empty
// set, not an error.
func TestNativeSources(t *testing.T) {
	dir := t.TempDir()
	if srcs, ok := nativeSources(dir); ok || len(srcs) != 0 {
		t.Fatalf("absent native/: want empty+false, got %v (ok=%v)", srcs, ok)
	}
	write := func(rel string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("int x;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("native/math.c")
	write("native/aaa.c")
	write("native/header.h")
	write("src/main.we")
	srcs, ok := nativeSources(dir)
	if !ok || len(srcs) != 2 {
		t.Fatalf("want 2 .c sources, got %v (ok=%v)", srcs, ok)
	}
	if srcs[0] != "aaa.c" || srcs[1] != "math.c" {
		t.Fatalf("byte order: %v", srcs)
	}
}

// The llvm-nm gate mirrors the clang gate: the pin exactly, everything
// else rejected, a missing tool rejected without a panic. This
// toolchain's real shape is `Ubuntu LLVM version 21.1.8`.
func TestCheckNmVersion(t *testing.T) {
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "llvm-nm")
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := checkNmVersion(write("echo 'Ubuntu LLVM version 21.1.8'\n")); err != nil {
		t.Fatalf("want accept, got: %v", err)
	}
	if err := checkNmVersion(write("echo 'Ubuntu LLVM version 22.0.0'\n")); err == nil {
		t.Fatalf("want reject for a newer major")
	}
	if err := checkNmVersion(write("echo 'some other tool'\n")); err == nil {
		t.Fatalf("want reject without a version line")
	}
	if err := checkNmVersion(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatalf("want reject for a missing tool")
	}
}

// definedSymbols lists the defined names llvm-nm reports for the .o
// files: one `address type name` line per symbol, the name in the last
// field.
func TestDefinedSymbols(t *testing.T) {
	p := filepath.Join(t.TempDir(), "llvm-nm")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nprintf '0000000000000000 T abs\\n0000000000000000 T dial\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	obj := filepath.Join(t.TempDir(), "native-math.o")
	if err := os.WriteFile(obj, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	defined, err := definedSymbols(p, []string{obj})
	if err != nil {
		t.Fatal(err)
	}
	if !defined["abs"] || !defined["dial"] || defined["missing"] {
		t.Fatalf("defined set: %v", defined)
	}
}

// The difference set the E1906 report renders: every declared foreign
// name no native object defines.
func TestMissingForeignNames(t *testing.T) {
	declared := []codegen.ForeignName{
		{Module: "main", Name: "abs", Line: 2, Col: 8},
		{Module: "main", Name: "dial", Line: 3, Col: 8},
	}
	missing := missingForeignNames(declared, map[string]bool{"abs": true})
	if len(missing) != 1 || missing[0].Name != "dial" {
		t.Fatalf("want dial missing, got %+v", missing)
	}
	if got := missingForeignNames(declared, map[string]bool{"abs": true, "dial": true}); len(got) != 0 {
		t.Fatalf("want none missing, got %+v", got)
	}
}
