package stdlib

import (
	"reflect"
	"sort"
	"testing"

	"github.com/ltlvtao/welang/internal/parser"
)

// TestEmbeddedSourceSet: the embedded key set is exactly the migrated
// three (B2a T1) — fs, process, and string join in T2–T4; concurrent
// never does (design D1-5 keeps it synthetic). A key landing here
// unannounced is a slice violation the gate should catch early.
func TestEmbeddedSourceSet(t *testing.T) {
	want := []string{"io", "test", "time"}
	got := make([]string, 0, len(Sources()))
	for k := range Sources() {
		got = append(got, k)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("embedded std sources = %v, want %v", got, want)
	}
}

// TestEmbeddedSourcesParseClean: every embedded source parses clean — the
// first diagnostic and the not-implemented boundary both nil. This gate
// is what lets StdModule panic on a parse failure instead of diagnosing:
// the gate and the sources cannot drift apart without this test failing
// first.
func TestEmbeddedSourcesParseClean(t *testing.T) {
	for key, src := range Sources() {
		file, d, ni := parser.Parse("std/"+key+".we", []byte(src))
		if d != nil || ni != nil || file == nil {
			t.Errorf("std/%s.we does not parse: d=%v ni=%+v file=%v", key, d, ni, file)
		}
	}
}
