// Package benchmarks is the machine face of the We evaluation suite
// (docs/benchmarks.md): the batch schema, the embedded task set, and the
// runner that classifies attempts into the five buckets and computes the
// three metrics. The document is the single authority; this package is its
// consumption face — where the two disagree, the document governs and this
// package carries the bug.
package benchmarks

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Batch is one task's attempt sequence (docs/benchmarks.md, "Batch schema
// and feedback protocol"): the model that produced it, the task id, and
// the rounds in order. Files map slash paths under src/ to source text;
// the runner overlays them onto the reference project.
type Batch struct {
	Model    string    `json:"model"`
	Task     string    `json:"task"`
	Attempts []Attempt `json:"attempts"`
}

// Attempt is one round's submission — the files face of that round's
// generate step.
type Attempt struct {
	Round int               `json:"round"`
	Files map[string]string `json:"files"`
}

// LoadBatch parses and validates a batch file. known is the task-set face
// the task id is validated against (the embedded registry's Has). A batch
// LoadBatch rejects never reaches the runner: the verdict pipeline sees
// only well-formed batches, so every rejection here is the schema's own
// error face, never a bucket.
func LoadBatch(data []byte, known func(task string) bool) (*Batch, error) {
	var b Batch
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("batch: invalid JSON — %v", err)
	}
	if b.Model == "" {
		return nil, fmt.Errorf("batch: model is empty — the model field identifies the batch's producer")
	}
	if !known(b.Task) {
		return nil, fmt.Errorf("batch: unknown task %q — the task must exist in the task set", b.Task)
	}
	if len(b.Attempts) == 0 {
		return nil, fmt.Errorf("batch: no attempts — a batch carries at least one round")
	}
	for i, a := range b.Attempts {
		if a.Round != i+1 {
			return nil, fmt.Errorf("batch: attempt %d carries round %d where %d goes — rounds start at 1 and continue without gaps or repeats", i+1, a.Round, i+1)
		}
		if len(a.Files) == 0 {
			return nil, fmt.Errorf("batch: attempt %d: files is empty — a round submits at least one source file", i+1)
		}
		for p := range a.Files {
			if filepath.ToSlash(filepath.Clean(p)) != p {
				return nil, fmt.Errorf("batch: attempt %d: file path %q is not in clean slash form — write src/main.we, not src/./main.we or src/../src/main.we", i+1, p)
			}
			if !strings.HasPrefix(p, "src/") || len(p) <= len("src/") {
				return nil, fmt.Errorf("batch: attempt %d: file path %q is outside src/ — src/ is the attempt's whole surface; tests/ and we.toml are the scoring face", i+1, p)
			}
		}
	}
	return &b, nil
}
