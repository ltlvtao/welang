package typecheck

import (
	"reflect"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// TestStdModuleLoadsParsedSources: the migrated three come back as real
// parses of the embedded sources (B2a design D1-2) — pub fn decls with
// the names the synthetic registry used to hand out; the bodies are the
// fictions the sources write (empty blocks; now's tail 0). assertEqual
// is among them by design D1-4 option A: declared for reach, its calls
// still riding importCall's dispatch ahead of the gate. std.string's two
// are the first real bodies (B2a design D5): the row checks the names
// the same way — the body's realness is the pipeline's business, not the
// loader's.
func TestStdModuleLoadsParsedSources(t *testing.T) {
	cases := []struct {
		key   string
		names []string
	}{
		{"std.io", []string{"println", "print"}},
		{"std.test", []string{"assertTrue", "assertFalse", "assertEqual"}},
		{"std.time", []string{"now", "sleep"}},
		{"std.fs", []string{"readFile", "writeFile", "appendFile", "removeFile", "makeDir", "removeDir", "listDir"}},
		{"std.process", []string{"run"}},
		{"std.string", []string{"join", "repeat"}},
	}
	for _, tc := range cases {
		file, ok := StdModule(tc.key)
		if !ok || file == nil {
			t.Fatalf("%s not registered", tc.key)
		}
		var got []string
		for _, it := range file.Items {
			fn, isFn := it.(*ast.FnDecl)
			if !isFn {
				continue // fs and process carry their sums and records beside the fns
			}
			if !fn.Pub {
				t.Fatalf("%s: %s is not pub", tc.key, fn.Name)
			}
			got = append(got, fn.Name)
		}
		if !reflect.DeepEqual(got, tc.names) {
			t.Fatalf("%s items = %v, want %v", tc.key, got, tc.names)
		}
	}
}

// TestStdModuleParseCacheHoldsOneFile: the cache hands every caller the
// same pointer — the buckets' sumInfo identity and the cross-module AST
// sharing both ride pointer stability (the M6b precedent).
func TestStdModuleParseCacheHoldsOneFile(t *testing.T) {
	a, _ := StdModule("std.io")
	b, _ := StdModule("std.io")
	if a != b {
		t.Fatal(`StdModule("std.io") returned two distinct files`)
	}
}

// TestStdConcurrentStaysSynthetic: the D1-5 exception — the same package
// singleton as before the flip, with the same item shapes (four pub sums
// plus the channel marker fn).
func TestStdConcurrentStaysSynthetic(t *testing.T) {
	f1, ok1 := StdModule("std.concurrent")
	f2, ok2 := StdModule("std.concurrent")
	if !ok1 || !ok2 || f1 == nil || f1 != f2 || f1 != stdConcurrentFile {
		t.Fatal("std.concurrent is not the synthetic singleton")
	}
	var sums, fns int
	for _, it := range f1.Items {
		switch it.(type) {
		case *ast.SumDecl:
			sums++
		case *ast.FnDecl:
			fns++
		}
	}
	if sums != 4 || fns != 1 {
		t.Fatalf("std.concurrent items = %d sums + %d fns, want 4 + 1", sums, fns)
	}
}

// TestStdFsIsNotYetAStandardModule was T2's red-first anchor — fs landed
// with T2, so the anchor retires here: the loader answers with the parsed
// source, and the module's own FsError sum rides beside the seven fns
// (a caller names it qualified — Result<_, fs.FsError>).
func TestStdFsIsAStandardModule(t *testing.T) {
	file, ok := StdModule("std.fs")
	if !ok || file == nil {
		t.Fatal("std.fs not registered")
	}
	var sums int
	for _, it := range file.Items {
		if _, isSum := it.(*ast.SumDecl); isSum {
			sums++
		}
	}
	if sums != 1 {
		t.Fatalf("std.fs carries %d sum decls, want 1 (FsError)", sums)
	}
}
