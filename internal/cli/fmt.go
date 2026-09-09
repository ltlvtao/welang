package cli

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	wefmt "github.com/ltlvtao/welang/internal/fmt"
	"github.com/ltlvtao/welang/internal/parser"
)

// runFmt runs the fmt subcommand (design D2): a directory is a project —
// the manifest gate first, the same E1905 face as check/build/run/test —
// then every .we file under it except the build/ subtree; a .we file is
// the single-file face with no manifest gate. Each file formats
// independently (adjudication Q1): one file's failure neither stops nor
// rolls back another's rewrite. fmt runs no module resolution and no type
// stage — R3's rule face is pure layout; a bad parse blocks, a bad type
// does not.
func (e *env) runFmt(path string, info os.FileInfo) int {
	var files []string
	if info.IsDir() {
		// The manifest gate carries the dependency face's declared-form
		// validations (they hold for every reader) — but fmt never
		// resolves: the table comes back unread (design D6).
		if _, _, code := e.loadManifest(path, false); code != exitOK {
			return code
		}
		var err error
		files, err = collectWeFiles(path)
		if err != nil {
			return e.fsError(err)
		}
	} else {
		if !strings.HasSuffix(path, ".we") {
			return e.usageErr("fmt wants a .we file, got %q", path)
		}
		files = []string{path}
	}
	return e.fmtFiles(files)
}

// collectWeFiles walks a project directory for every .we file, excluding
// the build/ subtree (adjudication Q2 — generated output), in lexical
// order (WalkDir's own).
func collectWeFiles(dir string) ([]string, error) {
	buildDir := filepath.Join(dir, "build")
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == buildDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".we") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// fmtFiles formats each path independently (Q1): the parser runs first as
// the boundary pre-gate — Format has no not-implemented face of its own,
// so this caller owns that class (exit 70 stops the run; a ratified form
// this build cannot yet parse is not reformatted) — then Format, and a
// changed output writes back 0o644. Success is silent; --verbose lists
// each rewritten file (`we: formatted {path}`, the `we: built` precedent
// shape). A parse failure reports and continues to the next file; the
// run's exit is 1 if any file failed. The second parse inside Format is
// accepted — correctness face over performance (R11 posture).
func (e *env) fmtFiles(paths []string) int {
	code := exitOK
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return e.fsError(err)
		}
		_, d, ni := parser.Parse(p, src)
		if d != nil {
			e.report(*d)
			code = exitDiagnostic
			continue
		}
		if ni != nil {
			return e.boundary(ni.What)
		}
		out, ds := wefmt.Format(p, src)
		if len(ds) > 0 {
			e.report(ds[0])
			code = exitDiagnostic
			continue
		}
		if !bytes.Equal(out, src) {
			if err := os.WriteFile(p, out, 0o644); err != nil {
				return e.fsError(err)
			}
			if e.verbose {
				fmt.Fprintf(e.stdout, "we: formatted %s\n", p)
			}
		}
	}
	return code
}

// vetPosture reads one advisory code's [vet] posture from the manifest;
// an absent table or key is the default warning (the registry's default
// severity). The value's legality was already the loader's gate.
func vetPosture(manifest map[string]string, code string) string {
	if v, ok := manifest["vet."+code]; ok {
		return v
	}
	return "warning"
}
