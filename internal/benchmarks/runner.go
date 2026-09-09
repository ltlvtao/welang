package benchmarks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ltlvtao/welang/internal/cli"
)

// The verdict machine and metric face of docs/benchmarks.md ("Runner
// verdict order", "The three metrics"). Judgment only: the runner never
// produces feedback and never calls a model — batch production is a
// separate face. Every attempt is assembled as a fresh copy of the
// reference project with the attempt's files overlaid, judged by
// `we check --json` then (on check pass) `we test --json` through the
// same in-process CLI injection the conformance suite uses. Determinism:
// no clock, no network, no temp paths in the report — re-running the
// same batches renders byte-identical JSON.
//
// Known limit (disclosed): evaluation is in-process, so an attempt whose
// compiled code never terminates (a busy loop with no wait source) halts
// its evaluation; the real-batch production face must guard against that
// before feeding untrusted attempts.

// ConvergenceCap is the FLC truncation point: a task whose rounds never
// check-pass within this many attempts counts as not-converged, carried
// at the cap in the max.
const ConvergenceCap = 5

// Bucket is the closed five-bucket classification of one attempt.
type Bucket string

const (
	// BucketClean: check exit 0 and test exit 0.
	BucketClean Bucket = "clean"
	// BucketLatent: check exit 0 and test exit 1.
	BucketLatent Bucket = "latent"
	// BucketTestMalformed: check exit 0 and test exit 2 or 70.
	BucketTestMalformed Bucket = "test-malformed"
	// BucketRejected: check exit 1 (error diagnostics).
	BucketRejected Bucket = "rejected"
	// BucketBoundary: check exit 70 or 2 (honest boundary / usage
	// malformed) — excluded from every metric's sides, disclosed alone.
	BucketBoundary Bucket = "boundary"
)

// checkPass reports whether the bucket clears the check gate.
func checkPass(b Bucket) bool {
	return b == BucketClean || b == BucketLatent || b == BucketTestMalformed
}

// AttemptVerdict is one attempt's classification.
type AttemptVerdict struct {
	Round     int    `json:"round"`
	Bucket    Bucket `json:"bucket"`
	CheckExit int    `json:"check_exit"`
	// TestExit is nil when the test stage never ran (check did not pass).
	TestExit *int `json:"test_exit,omitempty"`
}

// TaskReport is one task's view across its batch's rounds.
type TaskReport struct {
	Task           string            `json:"task"`
	Attempts       []*AttemptVerdict `json:"attempts"`
	ConvergedRound int               `json:"converged_round"`
}

// Ratio is a rate with its denominator always shown — no bare
// percentages (statistical discipline).
type Ratio struct {
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Rate        float64 `json:"rate"`
}

func ratio(n, d int) Ratio {
	r := Ratio{Numerator: n, Denominator: d}
	if d > 0 {
		r.Rate = float64(n) / float64(d)
	}
	return r
}

// FLCStats is the convergence-round distribution across tasks: rounds of
// converged tasks, the not-converged census count, and median/p90/max
// with not-converged carried at the cap in max.
type FLCStats struct {
	ConvergenceRounds []int   `json:"convergence_rounds"`
	NotConverged      int     `json:"not_converged"`
	Median            float64 `json:"median"`
	P90               float64 `json:"p90"`
	Max               int     `json:"max"`
}

// Metrics is the report's aggregate face.
type Metrics struct {
	SampleSize struct {
		Tasks    int `json:"tasks"`
		Attempts int `json:"attempts"`
	} `json:"sample_size"`
	FPCR                  Ratio    `json:"fpcr"`
	FLC                   FLCStats `json:"flc"`
	LBRAttempt            Ratio    `json:"lbr_attempt"`
	LBRTask               Ratio    `json:"lbr_task"`
	BoundaryAttempts      int      `json:"boundary_attempts"`
	TestMalformedAttempts int      `json:"test_malformed_attempts"`
}

// Report is the machine face of one evaluation run.
type Report struct {
	Model   string        `json:"model"`
	Tasks   []*TaskReport `json:"tasks"`
	Metrics Metrics       `json:"metrics"`
}

// Evaluate runs every batch's attempts through the verdict order and
// aggregates the three metrics. Batches must have cleared LoadBatch.
func Evaluate(batches []*Batch) *Report {
	rep := &Report{}
	for _, b := range batches {
		task := taskByID(b.Task)
		tr := &TaskReport{Task: b.Task}
		for _, a := range b.Attempts {
			verdict := judgeAttempt(task, a)
			tr.Attempts = append(tr.Attempts, verdict)
			if tr.ConvergedRound == 0 && checkPass(verdict.Bucket) {
				tr.ConvergedRound = verdict.Round
			}
			rep.Metrics.SampleSize.Attempts++
			switch verdict.Bucket {
			case BucketBoundary:
				rep.Metrics.BoundaryAttempts++
			case BucketTestMalformed:
				rep.Metrics.TestMalformedAttempts++
			}
		}
		rep.Tasks = append(rep.Tasks, tr)
		rep.Metrics.SampleSize.Tasks++
		if rep.Model == "" {
			rep.Model = b.Model
		}
	}
	computeMetrics(rep)
	return rep
}

func taskByID(id string) Task {
	parseTasks()
	return taskIndex[id]
}

// judgeAttempt assembles the attempt over a fresh reference copy and runs
// the two-stage verdict order: check classifies or gates, test finishes
// the classification.
func judgeAttempt(task Task, a Attempt) *AttemptVerdict {
	dir, err := os.MkdirTemp("", "we-bench-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	for p, content := range task.Files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			panic(err)
		}
	}
	// Defense in depth: only src/ overlays land — LoadBatch already
	// rejects paths outside it, but Evaluate trusts its caller only so
	// far; the scoring face (tests/, we.toml) is never overwritable.
	for p, content := range a.Files {
		if !strings.HasPrefix(p, "src/") || len(p) <= len("src/") {
			panic("benchmarks: attempt file outside src/ reached the runner: " + p)
		}
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			panic(err)
		}
	}
	prev, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	var out, errb bytes.Buffer
	checkExit := runCLI([]string{"check", ".", "--json"}, &out, &errb)
	v := &AttemptVerdict{Round: a.Round, CheckExit: checkExit}
	switch checkExit {
	case 0:
	case 1:
		v.Bucket = BucketRejected
	default:
		// 70 (honest boundary) and 2 (usage malformed) share the bucket.
		v.Bucket = BucketBoundary
	}
	if checkExit == 0 {
		testExit := runCLI([]string{"test", ".", "--json"}, &out, &errb)
		v.TestExit = &testExit
		switch testExit {
		case 0:
			v.Bucket = BucketClean
		case 1:
			v.Bucket = BucketLatent
		default:
			v.Bucket = BucketTestMalformed
		}
	}
	if err := os.Chdir(prev); err != nil {
		panic(err)
	}
	return v
}

// runCLI runs one argv inside the process working directory with
// injected streams — the conformance runner's discipline without the
// golden plumbing.
func runCLI(args []string, out, errb *bytes.Buffer) int {
	out.Reset()
	errb.Reset()
	return cli.Run(args, out, errb)
}

func computeMetrics(rep *Report) {
	m := &rep.Metrics
	// FPCR: first-round attempts only; boundary excluded from both sides.
	var num, den int
	for _, tr := range rep.Tasks {
		v := tr.Attempts[0]
		if v.Bucket == BucketBoundary {
			continue
		}
		den++
		if checkPass(v.Bucket) {
			num++
		}
	}
	m.FPCR = ratio(num, den)

	// FLC: convergence rounds per task; not-converged carried at the cap.
	rounds := make([]int, 0, len(rep.Tasks))
	for _, tr := range rep.Tasks {
		if tr.ConvergedRound == 0 {
			m.FLC.NotConverged++
			m.FLC.Max = ConvergenceCap
			rounds = append(rounds, 0)
			continue
		}
		rounds = append(rounds, tr.ConvergedRound)
		if tr.ConvergedRound > m.FLC.Max {
			m.FLC.Max = tr.ConvergedRound
		}
	}
	m.FLC.ConvergenceRounds = rounds
	converged := make([]int, 0, len(rounds))
	for _, r := range rounds {
		if r > 0 {
			converged = append(converged, r)
		}
	}
	sort.Ints(converged)
	if len(converged) > 0 {
		m.FLC.Median = median(converged)
		m.FLC.P90 = percentileNearestRank(converged, 90)
	}

	// LBR, attempt level: every check-pass attempt of every round.
	var lat, latDen int
	for _, tr := range rep.Tasks {
		for _, v := range tr.Attempts {
			if checkPass(v.Bucket) {
				latDen++
				if v.Bucket == BucketLatent {
					lat++
				}
			}
		}
	}
	m.LBRAttempt = ratio(lat, latDen)

	// LBR, task level (derived view): of the tasks with at least one
	// check-pass attempt, those whose last check-pass attempt is latent.
	var tlat, tden int
	for _, tr := range rep.Tasks {
		var last *AttemptVerdict
		for _, v := range tr.Attempts {
			if checkPass(v.Bucket) {
				last = v
			}
		}
		if last == nil {
			continue
		}
		tden++
		if last.Bucket == BucketLatent {
			tlat++
		}
	}
	m.LBRTask = ratio(tlat, tden)
}

func median(sorted []int) float64 {
	n := len(sorted)
	if n%2 == 1 {
		return float64(sorted[n/2])
	}
	return float64(sorted[n/2-1]+sorted[n/2]) / 2
}

func percentileNearestRank(sorted []int, p int) float64 {
	n := len(sorted)
	rank := (p * n) / 100
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return float64(sorted[rank-1])
}

// RenderJSON serializes the report — the reproducible artifact face
// (reports are re-producible, so they do not enter the repository).
func (r *Report) RenderJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
