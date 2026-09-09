package benchmarks

import (
	"strings"
	"testing"
)

// known stubs the task-set face LoadBatch validates task ids against —
// the embedded registry's Has (the registry lands with the task set).
func known(id string) bool { return id == "demo-01" }

const validBatch = `{
  "model": "gpt-demo",
  "task": "demo-01",
  "attempts": [
    {"round": 1, "files": {"src/main.we": "fn a() {}"}},
    {"round": 2, "files": {"src/main.we": "fn a() {}", "src/util/help.we": "fn b() {}"}}
  ]
}`

func TestLoadBatchValid(t *testing.T) {
	b, err := LoadBatch([]byte(validBatch), known)
	if err != nil {
		t.Fatalf("valid batch rejected: %v", err)
	}
	if b.Model != "gpt-demo" || b.Task != "demo-01" || len(b.Attempts) != 2 {
		t.Fatalf("fields: %+v", b)
	}
	a1 := b.Attempts[0]
	if a1.Round != 1 || len(a1.Files) != 1 || a1.Files["src/main.we"] != "fn a() {}" {
		t.Errorf("attempt 1: %+v", a1)
	}
	a2 := b.Attempts[1]
	if a2.Round != 2 || len(a2.Files) != 2 || a2.Files["src/util/help.we"] != "fn b() {}" {
		t.Errorf("attempt 2: %+v", a2)
	}
}

func TestLoadBatchMalformed(t *testing.T) {
	cases := []struct {
		name    string
		batch   string
		wantErr string
	}{
		{"invalid JSON", `{"model": `, "invalid JSON"},
		{"empty model", `{"model": "", "task": "demo-01", "attempts": [{"round": 1, "files": {"src/main.we": "x"}}]}`, "model is empty"},
		{"unknown task", `{"model": "m", "task": "no-such", "attempts": [{"round": 1, "files": {"src/main.we": "x"}}]}`, "unknown task"},
		{"no attempts", `{"model": "m", "task": "demo-01", "attempts": []}`, "no attempts"},
		{"missing attempts", `{"model": "m", "task": "demo-01"}`, "no attempts"},
		{"round gap", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {"src/main.we": "x"}}, {"round": 3, "files": {"src/main.we": "x"}}]}`, "round 3 where 2 goes"},
		{"round not starting at 1", `{"model": "m", "task": "demo-01", "attempts": [{"round": 2, "files": {"src/main.we": "x"}}]}`, "round 2 where 1 goes"},
		{"repeated round", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {"src/main.we": "x"}}, {"round": 1, "files": {"src/main.we": "x"}}]}`, "round 1 where 2 goes"},
		{"empty files", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {}}]}`, "files is empty"},
		{"path touches tests/", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {"tests/x_test.we": "x"}}]}`, "outside src/"},
		{"path touches we.toml", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {"we.toml": "x"}}]}`, "outside src/"},
		{"bare src", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {"src": "x"}}]}`, "outside src/"},
		{"absolute path", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {"/etc/passwd": "x"}}]}`, "outside src/"},
		{"unclean path", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {"src/../src/main.we": "x"}}]}`, "not in clean slash form"},
		{"dot-prefixed path", `{"model": "m", "task": "demo-01", "attempts": [{"round": 1, "files": {"./src/main.we": "x"}}]}`, "not in clean slash form"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadBatch([]byte(tc.batch), known)
			if err == nil {
				t.Fatalf("malformed batch accepted: %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not carry %q", err.Error(), tc.wantErr)
			}
		})
	}
}
