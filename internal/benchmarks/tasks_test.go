package benchmarks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/cli"
)

// The seed task set is itself an inspected artifact: this file pins its
// shape (docs/benchmarks.md, "Task taxonomy") and self-certifies every
// reference solution — check exit 0 and test exit 0, in-process, the same
// injection face the runner uses. A task whose reference cannot run green
// has no business defining what a correct submission looks like.

var wantTaskIDs = []string{
	"control-01", "control-02", "control-03", "control-04",
	"ctxpass-01", "ctxpass-02",
	"dangling-01", "dangling-02",
	"errpath-01", "errpath-02",
	"implconv-01", "implconv-02",
	"nullbnd-01", "nullbnd-02",
	"race-01", "race-02",
	"resleak-01", "resleak-02",
}

func TestTaskSetShape(t *testing.T) {
	got := Tasks()
	if len(got) != len(wantTaskIDs) {
		t.Fatalf("task set carries %d tasks, want %d", len(got), len(wantTaskIDs))
	}
	for i, id := range wantTaskIDs {
		if got[i].ID != id {
			t.Errorf("task %d is %q, want %q (IDs sort in set order)", i, got[i].ID, id)
		}
	}
	for _, task := range got {
		if !Has(task.ID) {
			t.Errorf("Has(%q) is false for a listed task", task.ID)
		}
		if strings.TrimSpace(task.Prompt) == "" {
			t.Errorf("%s: prompt is empty", task.ID)
		}
		if strings.TrimSpace(task.Traps) == "" {
			t.Errorf("%s: traps are empty", task.ID)
		}
		if task.Files["we.toml"] == "" || task.Files["src/main.we"] == "" {
			t.Errorf("%s: reference lacks we.toml or src/main.we", task.ID)
		}
		var tests int
		for p := range task.Files {
			if strings.HasPrefix(p, "tests/") && strings.HasSuffix(p, ".we") {
				tests++
			}
		}
		if tests == 0 {
			t.Errorf("%s: reference carries no test file", task.ID)
		}
	}
	if Has("no-such-task") {
		t.Errorf(`Has("no-such-task") is true`)
	}
}

// runInProcess mirrors the conformance runner's discipline: chdir into the
// assembled project, run the argv with injected streams, restore the
// process directory.
func runInProcess(t *testing.T, dir string, args ...string) int {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := cli.Run(args, &out, &errb)
	if err := os.Chdir(prev); err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Logf("%v exit %d\nstdout:\n%s\nstderr:\n%s", args, code, out.String(), errb.String())
	}
	return code
}

func TestReferenceSolutionsGreen(t *testing.T) {
	for _, task := range Tasks() {
		t.Run(task.ID, func(t *testing.T) {
			dir := t.TempDir()
			for p, content := range task.Files {
				full := filepath.Join(dir, filepath.FromSlash(p))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if code := runInProcess(t, dir, "check", ".", "--json"); code != 0 {
				t.Fatalf("reference check exit %d, want 0", code)
			}
			if code := runInProcess(t, dir, "test", ".", "--json"); code != 0 {
				t.Fatalf("reference test exit %d, want 0", code)
			}
		})
	}
}
