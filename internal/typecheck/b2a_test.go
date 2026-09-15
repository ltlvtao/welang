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
// still riding importCall's dispatch ahead of the gate.
func TestStdModuleLoadsParsedSources(t *testing.T) {
	cases := []struct {
		key   string
		names []string
	}{
		{"std.io", []string{"println", "print"}},
		{"std.test", []string{"assertTrue", "assertFalse", "assertEqual"}},
		{"std.time", []string{"now", "sleep"}},
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
				t.Fatalf("%s: item %T is not a fn decl", tc.key, it)
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

// TestStdFsIsNotYetAStandardModule: T2's red-first anchor — no fs source
// is embedded yet, so the loader answers not-found and E1302's std form
// stays the observable face for import std.fs.
func TestStdFsIsNotYetAStandardModule(t *testing.T) {
	if f, ok := StdModule("std.fs"); ok {
		t.Fatalf("std.fs registered with %d items", len(f.Items))
	}
}
