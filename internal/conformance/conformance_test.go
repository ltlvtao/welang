package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const casesDir = "testdata/cases"

// TestGoldenCases runs every case under testdata/cases against the CLI and
// compares exit code, both streams, and (when Files is present) the exact
// created tree. WE_UPDATE_GOLDEN=1 rewrites expectations from actual
// behavior instead of failing — a maintenance tool, not the initial source
// of truth (expectations are hand-pinned from the spec).
func TestGoldenCases(t *testing.T) {
	cases, err := LoadCases(casesDir)
	if err != nil {
		t.Fatalf("load cases: %v", err)
	}
	if len(cases) == 0 {
		t.Fatalf("no golden cases found in %s", casesDir)
	}
	update := os.Getenv("WE_UPDATE_GOLDEN") == "1"
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			workdir := t.TempDir()
			got := Execute(&c, workdir)
			mismatches := diff(&c, got)
			if len(mismatches) == 0 {
				return
			}
			if update {
				c.Exit, c.Stdout, c.Stderr = got.Exit, got.Stdout, got.Stderr
				if c.Files != nil {
					c.Files = got.Files
				}
				if err := WriteCase(filepath.Join(casesDir, c.Name+".json"), c); err != nil {
					t.Fatalf("update golden: %v", err)
				}
				return
			}
			t.Errorf("golden mismatch:\n%s", strings.Join(mismatches, "\n"))
		})
	}
}

func diff(want *Case, got Result) []string {
	var ms []string
	if want.Exit != got.Exit {
		ms = append(ms, fmt.Sprintf("exit: want %d, got %d", want.Exit, got.Exit))
	}
	if want.Stdout != got.Stdout {
		ms = append(ms, fmt.Sprintf("stdout:\n  want: %q\n  got:  %q", want.Stdout, got.Stdout))
	}
	if want.Stderr != got.Stderr {
		ms = append(ms, fmt.Sprintf("stderr:\n  want: %q\n  got:  %q", want.Stderr, got.Stderr))
	}
	if want.Files != nil && !reflect.DeepEqual(want.Files, got.Files) {
		ms = append(ms, fmt.Sprintf("files:\n  want: %v\n  got:  %v", sortedKeys(want.Files), sortedKeys(got.Files)))
		for k, v := range want.Files {
			if got.Files[k] != v {
				ms = append(ms, fmt.Sprintf("file %s content:\n  want: %q\n  got:  %q", k, v, got.Files[k]))
			}
		}
	}
	return ms
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
