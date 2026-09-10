package benchmarks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Black-box battery (design D6, tasks.md T6): the in-process verdict
// machine injects streams into cli.Run; this battery re-derives the same
// verdicts through the real `we` binary as a subprocess and asserts the
// two faces agree on every bucket. One shape per bucket, plus the
// check-gated shape (rejected never reaches the test stage).
func TestBlackboxBucketParity(t *testing.T) {
	if testing.Short() {
		t.Skip("blackbox battery builds the binary and shells out")
	}
	bin := filepath.Join(t.TempDir(), "we")
	build := exec.Command("go", "build", "-o", bin, "github.com/ltlvtao/welang/cmd/we")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build cmd/we: %v\n%s", err, out)
	}

	shapes := []struct {
		name  string
		task  string
		files map[string]string
	}{
		{"clean", "control-01", attemptSrc(t, "control-01")},
		{"latent", "control-01", map[string]string{"src/main.we": attemptSeriesLatent}},
		{"rejected", "control-01", map[string]string{"src/main.we": attemptSeriesE0501}},
		{"boundary", "control-01", map[string]string{"src/main.we": attemptSeriesBoundary}},
		// The test-malformed shape needs an attempt that checks clean and
		// then fails the test stage's compile. control-04's modulo carrier
		// died with the codegen-mono expression set (T3: `%` and `/` are
		// checked arithmetic now, so `return n % 2` is a clean program),
		// leaving control-03's string-equality decline — a boundary until
		// the string chain lands — as the corpus's live carrier.
		{"test-malformed", "control-03", attemptFiles(t, "control-03", func(src string) string {
			return strings.Replace(src, `pub fn echo(s: String) -> String {
    return s
}`, `pub fn echo(s: String) -> String {
    var r: String = s
    if s == "mirror" {
        r = "mirror-x"
    }
    return r
}`, 1)
		})},
	}

	for _, sh := range shapes {
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			// Assemble the same project the verdict machine assembles.
			dir := t.TempDir()
			for p, content := range referenceFiles(t, sh.task) {
				full := filepath.Join(dir, filepath.FromSlash(p))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for p, content := range sh.files {
				full := filepath.Join(dir, filepath.FromSlash(p))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// Subprocess face: exit codes of the real binary.
			checkExit := runBinary(t, bin, dir, "check")
			sub := bucketFromExits(checkExit, func() int {
				if checkExit != 0 {
					return -1
				}
				return runBinary(t, bin, dir, "test")
			})
			// In-process face: the verdict machine.
			rep := Evaluate([]*Batch{{
				Model:    "blackbox",
				Task:     sh.task,
				Attempts: []Attempt{{Round: 1, Files: sh.files}},
			}})
			inproc := rep.Tasks[0].Attempts[0].Bucket
			if sub != inproc {
				t.Fatalf("bucket drift: subprocess %q vs in-process %q", sub, inproc)
			}
			if inproc != Bucket(sh.name) {
				t.Fatalf("bucket %q, want the %q shape", inproc, sh.name)
			}
		})
	}
}

// runBinary runs the real we binary in dir and returns its exit code,
// failing the test on abnormal signals.
func runBinary(t *testing.T, bin, dir, stage string) int {
	t.Helper()
	cmd := exec.Command(bin, stage, ".", "--json")
	cmd.Dir = dir
	err := cmd.Run()
	exit := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("%s: %v", stage, err)
		}
		exit = ee.ExitCode()
	}
	return exit
}

// bucketFromExits mirrors the verdict order over subprocess exit codes.
func bucketFromExits(checkExit int, testExit func() int) Bucket {
	switch checkExit {
	case 0:
	case 1:
		return BucketRejected
	default:
		return BucketBoundary
	}
	switch testExit() {
	case 0:
		return BucketClean
	case 1:
		return BucketLatent
	default:
		return BucketTestMalformed
	}
}
