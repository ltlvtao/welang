package benchmarks

import (
	"encoding/json"
	"strings"
	"testing"
)

// The five synthetic shapes of design D5: every batch text below is a
// hand-written attempt against a real seed task, simulating what the
// feedback protocol would produce. Each asserts exact Report values —
// the verdict machine is judged by numbers, not by spot checks.

// control-01 attempt texts.

const (
	// attemptSeriesE0501 declares the accumulator Int32 against Int64
	// literals — check rejects with E0501.
	attemptSeriesE0501 = `pub type AppError = Failed(String)

pub fn seriesSum(n: Int64) -> Int64 {
    var total: Int32 = 0
    var i: Int64 = 1
    while i <= n {
        total = total + i
        i = i + 1
    }
    return total
}

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`
	// attemptSeriesE0012 names the function snake_case — E0012.
	attemptSeriesE0012 = `pub type AppError = Failed(String)

pub fn series_sum(n: Int64) -> Int64 {
    var total: Int64 = 0
    var i: Int64 = 1
    while i <= n {
        total = total + i
        i = i + 1
    }
    return total
}

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`
	// attemptSeriesE0605 drops a non-unit call — E0605.
	attemptSeriesE0605 = `pub type AppError = Failed(String)

pub fn seriesSum(n: Int64) -> Int64 {
    seriesSum(1)
    return 1
}

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`
	// attemptSeriesE1304 calls an undeclared helper — E1304.
	attemptSeriesE1304 = `pub type AppError = Failed(String)

pub fn seriesSum(n: Int64) -> Int64 {
    return helper(n)
}

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`
	// attemptSeriesE0827 constructs Some outside an expected position — E0827.
	attemptSeriesE0827 = `pub type AppError = Failed(String)

pub fn seriesSum(n: Int64) -> Int64 {
    let v = Some(3)
    return n
}

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`
	// attemptSeriesLatent compiles clean but adds 1 per step, not i —
	// the reference test catches the wrong total (check 0, test 1).
	attemptSeriesLatent = `pub type AppError = Failed(String)

pub fn seriesSum(n: Int64) -> Int64 {
    var total: Int64 = 0
    var i: Int64 = 1
    while i <= n {
        total = total + 1
        i = i + 1
    }
    return total
}

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`
	// attemptSeriesBoundary uses a top-level destructuring binding — a
	// chapter 8 form the reference build honestly declines (check 70).
	attemptSeriesBoundary = `pub type AppError = Failed(String)

let (a, b) = (1, 2)

pub fn seriesSum(n: Int64) -> Int64 {
    var total: Int64 = 0
    var i: Int64 = 1
    while i <= n {
        total = total + i
        i = i + 1
    }
    return total
}

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`
)

// referenceFiles is the task's full reference project (assembly input).
func referenceFiles(t *testing.T, id string) map[string]string {
	t.Helper()
	for _, task := range Tasks() {
		if task.ID == id {
			return task.Files
		}
	}
	t.Fatalf("no task %q", id)
	return nil
}

// attemptSrc is the reference solution as an attempt's submission face —
// src/ only, the way a batch carries it.
func attemptSrc(t *testing.T, id string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for p, c := range referenceFiles(t, id) {
		if strings.HasPrefix(p, "src/") && len(p) > len("src/") {
			out[p] = c
		}
	}
	return out
}

// attemptFiles mutates the reference's src/main.we and returns it as an
// attempt's submission face — src/ only.
func attemptFiles(t *testing.T, id string, mutate func(src string) string) map[string]string {
	t.Helper()
	src := mutate(referenceFiles(t, id)["src/main.we"])
	return map[string]string{"src/main.we": src}
}

func TestRunnerFirstPassShape(t *testing.T) {
	b := &Batch{Model: "synthetic", Task: "control-01", Attempts: []Attempt{
		{Round: 1, Files: attemptSrc(t, "control-01")},
	}}
	rep := Evaluate([]*Batch{b})
	v := rep.Tasks[0].Attempts[0]
	if v.Bucket != BucketClean {
		t.Fatalf("bucket %q, want clean", v.Bucket)
	}
	if rep.Metrics.FPCR.Numerator != 1 || rep.Metrics.FPCR.Denominator != 1 || rep.Metrics.FPCR.Rate != 1 {
		t.Fatalf("FPCR %+v, want 1/1", rep.Metrics.FPCR)
	}
	if rep.Metrics.LBRAttempt.Numerator != 0 || rep.Metrics.LBRAttempt.Denominator != 1 {
		t.Fatalf("LBR attempt %+v, want 0/1", rep.Metrics.LBRAttempt)
	}
	if rep.Tasks[0].ConvergedRound != 1 {
		t.Fatalf("converged round %d, want 1", rep.Tasks[0].ConvergedRound)
	}
}

func TestRunnerConvergeShape(t *testing.T) {
	b := &Batch{Model: "synthetic", Task: "control-01", Attempts: []Attempt{
		{Round: 1, Files: map[string]string{"src/main.we": attemptSeriesE0501}},
		{Round: 2, Files: attemptSrc(t, "control-01")},
	}}
	rep := Evaluate([]*Batch{b})
	if got := rep.Tasks[0].Attempts[0].Bucket; got != BucketRejected {
		t.Fatalf("round 1 bucket %q, want rejected", got)
	}
	if got := rep.Tasks[0].Attempts[1].Bucket; got != BucketClean {
		t.Fatalf("round 2 bucket %q, want clean", got)
	}
	if rep.Metrics.FPCR.Numerator != 0 || rep.Metrics.FPCR.Denominator != 1 {
		t.Fatalf("FPCR %+v, want 0/1 (first round rejected)", rep.Metrics.FPCR)
	}
	if rep.Tasks[0].ConvergedRound != 2 {
		t.Fatalf("converged round %d, want 2", rep.Tasks[0].ConvergedRound)
	}
	if rep.Metrics.FLC.NotConverged != 0 {
		t.Fatalf("not-converged %d, want 0", rep.Metrics.FLC.NotConverged)
	}
	// Both rounds count in the attempt-level LBR denominator; the latent
	// numerator stays 0.
	if rep.Metrics.LBRAttempt.Denominator != 1 || rep.Metrics.LBRAttempt.Numerator != 0 {
		t.Fatalf("LBR attempt %+v, want 0/1 (only the check-pass round counts)", rep.Metrics.LBRAttempt)
	}
}

func TestRunnerNotConvergedShape(t *testing.T) {
	b := &Batch{Model: "synthetic", Task: "control-01", Attempts: []Attempt{
		{Round: 1, Files: map[string]string{"src/main.we": attemptSeriesE0501}},
		{Round: 2, Files: map[string]string{"src/main.we": attemptSeriesE0012}},
		{Round: 3, Files: map[string]string{"src/main.we": attemptSeriesE0605}},
		{Round: 4, Files: map[string]string{"src/main.we": attemptSeriesE1304}},
		{Round: 5, Files: map[string]string{"src/main.we": attemptSeriesE0827}},
	}}
	rep := Evaluate([]*Batch{b})
	for i, want := range []Bucket{BucketRejected, BucketRejected, BucketRejected, BucketRejected, BucketRejected} {
		if got := rep.Tasks[0].Attempts[i].Bucket; got != want {
			t.Fatalf("round %d bucket %q, want %q", i+1, got, want)
		}
	}
	if rep.Metrics.FLC.NotConverged != 1 {
		t.Fatalf("not-converged %d, want 1", rep.Metrics.FLC.NotConverged)
	}
	if rep.Tasks[0].ConvergedRound != 0 {
		t.Fatalf("converged round %d, want 0 (censored)", rep.Tasks[0].ConvergedRound)
	}
	if rep.Metrics.FLC.Max != ConvergenceCap {
		t.Fatalf("FLC max %d, want the cap %d", rep.Metrics.FLC.Max, ConvergenceCap)
	}
	if rep.Metrics.FPCR.Denominator != 1 || rep.Metrics.FPCR.Numerator != 0 {
		t.Fatalf("FPCR %+v, want 0/1", rep.Metrics.FPCR)
	}
}

func TestRunnerLatentShape(t *testing.T) {
	b := &Batch{Model: "synthetic", Task: "control-01", Attempts: []Attempt{
		{Round: 1, Files: map[string]string{"src/main.we": attemptSeriesLatent}},
	}}
	rep := Evaluate([]*Batch{b})
	if got := rep.Tasks[0].Attempts[0].Bucket; got != BucketLatent {
		t.Fatalf("bucket %q, want latent", got)
	}
	if rep.Metrics.FPCR.Numerator != 1 || rep.Metrics.FPCR.Denominator != 1 {
		t.Fatalf("FPCR %+v, want 1/1 (latent counts in the numerator)", rep.Metrics.FPCR)
	}
	if rep.Metrics.LBRAttempt.Numerator != 1 || rep.Metrics.LBRAttempt.Denominator != 1 {
		t.Fatalf("LBR attempt %+v, want 1/1", rep.Metrics.LBRAttempt)
	}
	if rep.Metrics.LBRTask.Numerator != 1 || rep.Metrics.LBRTask.Denominator != 1 {
		t.Fatalf("LBR task %+v, want 1/1", rep.Metrics.LBRTask)
	}
}

func TestRunnerBoundaryShape(t *testing.T) {
	b := &Batch{Model: "synthetic", Task: "control-01", Attempts: []Attempt{
		{Round: 1, Files: map[string]string{"src/main.we": attemptSeriesBoundary}},
	}}
	rep := Evaluate([]*Batch{b})
	if got := rep.Tasks[0].Attempts[0].Bucket; got != BucketBoundary {
		t.Fatalf("bucket %q, want boundary", got)
	}
	if rep.Metrics.FPCR.Numerator != 0 || rep.Metrics.FPCR.Denominator != 0 {
		t.Fatalf("FPCR %+v, want 0/0 (boundary excluded from both sides)", rep.Metrics.FPCR)
	}
	if rep.Metrics.BoundaryAttempts != 1 {
		t.Fatalf("boundary count %d, want 1", rep.Metrics.BoundaryAttempts)
	}
}

func TestRunnerAggregatesTasks(t *testing.T) {
	clean := &Batch{Model: "syn", Task: "control-01", Attempts: []Attempt{
		{Round: 1, Files: attemptSrc(t, "control-01")},
	}}
	// Off-by-one parity loop against control-04: check-green, tests fail.
	latent := &Batch{Model: "syn", Task: "control-04", Attempts: []Attempt{
		{Round: 1, Files: attemptFiles(t, "control-04", func(src string) string {
			return strings.Replace(src, "while m >= 2 {", "while m > 2 {", 1)
		})},
	}}
	rep := Evaluate([]*Batch{clean, latent})
	if len(rep.Tasks) != 2 {
		t.Fatalf("task views %d, want 2", len(rep.Tasks))
	}
	if rep.Metrics.SampleSize.Tasks != 2 || rep.Metrics.SampleSize.Attempts != 2 {
		t.Fatalf("sample size %+v, want 2 tasks / 2 attempts", rep.Metrics.SampleSize)
	}
	if rep.Metrics.FPCR.Numerator != 2 || rep.Metrics.FPCR.Denominator != 2 {
		t.Fatalf("FPCR %+v, want 2/2", rep.Metrics.FPCR)
	}
	if rep.Metrics.LBRAttempt.Numerator != 1 || rep.Metrics.LBRAttempt.Denominator != 2 {
		t.Fatalf("LBR attempt %+v, want 1/2", rep.Metrics.LBRAttempt)
	}
	if rep.Metrics.LBRTask.Numerator != 1 || rep.Metrics.LBRTask.Denominator != 2 {
		t.Fatalf("LBR task %+v, want 1/2", rep.Metrics.LBRTask)
	}
}

func TestReportRendersJSON(t *testing.T) {
	b := &Batch{Model: "synthetic", Task: "control-01", Attempts: []Attempt{
		{Round: 1, Files: attemptSrc(t, "control-01")},
	}}
	data, err := json.Marshal(Evaluate([]*Batch{b}))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"model", "tasks", "metrics"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("report JSON lacks %q", key)
		}
	}
	metrics := doc["metrics"].(map[string]any)
	for _, key := range []string{"sample_size", "fpcr", "flc", "lbr_attempt", "lbr_task", "boundary_attempts", "test_malformed_attempts"} {
		if _, ok := metrics[key]; !ok {
			t.Errorf("metrics JSON lacks %q", key)
		}
	}
	if !strings.Contains(string(data), `"bucket": "clean"`) && !strings.Contains(string(data), `"bucket":"clean"`) {
		t.Errorf("report JSON lacks attempt bucket field")
	}
}
