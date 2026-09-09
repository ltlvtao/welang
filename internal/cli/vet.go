// The we vet subcommand (chapter 21 R4, M11 design D5): the check
// pipeline's own faces — the same path resolution, manifest gates, and
// type stage as check, no toolchain gate and no artifact — with the
// advisory layer as the run's own output. The [vet] postures decide what
// renders; an unignored finding of any severity is the run's exit 1 (the
// finding axis the spec pins — a promoted error's stop folds into the
// same 1; the mechanism's disclosed reading).

package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ltlvtao/welang/internal/typecheck"
)

// runVet dispatches like check: a directory is the project face, a .we
// file the single-file face (no manifest — every posture is the warning
// default), anything else the usage error.
func (e *env) runVet(path string, info os.FileInfo) int {
	if info.IsDir() {
		return e.runVetProject(path)
	}
	if !strings.HasSuffix(path, ".we") {
		return e.usageErr("vet wants a .we file, got %q", path)
	}
	file, code := e.loadFile(path)
	if file == nil {
		return code
	}
	if _, shown := e.advise(nil, typecheck.Advisories(file, path, typecheck.SingleFile)); shown > 0 {
		return exitDiagnostic
	}
	return exitOK
}

// runVetProject vets one project directory: the shared loader and check
// first (a diagnostic there is already the run's own exit 1), then the
// whole advisory surface — the source graph's root, then every test
// module — under the manifest's postures.
func (e *env) runVetProject(dir string) int {
	manifest, file, _, mods, code := e.loadProject(dir, false)
	if code != exitOK {
		return code
	}
	found, code := e.projectAdvisories(dir, filepath.Join(dir, "src", "main.we"), file, mods)
	if code != exitOK {
		return code
	}
	if _, shown := e.advise(manifest, found); shown > 0 {
		return exitDiagnostic
	}
	return exitOK
}
