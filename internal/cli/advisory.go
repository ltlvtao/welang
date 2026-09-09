// The advisory layer's cli face (M11 design D5): the render-and-gate
// stage every pipeline command shares, and the project's whole advisory
// surface in one walk order. The collection itself lives in typecheck
// (advisory.go, design D4); this file owns postures, rendering, and the
// two exit axes — the promoted stop and the finding count.

package cli

import (
	"path/filepath"
	"strings"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// advise renders the collected findings under their [vet] postures and
// reports the run's two outcomes: promoted — some finding maps to
// "error" and stops the pipeline after rendering — and shown, the count
// of unignored findings (we vet's exit axis). The postures read the
// manifest the loader already validated; a nil map is the warning
// default everywhere (the single-file faces own no manifest, so nothing
// can promote there). Rendering is exhaustive before any stop: a
// promoted finding never suppresses the findings after it.
func (e *env) advise(manifest map[string]string, found []diag.Diagnostic) (promoted bool, shown int) {
	for _, a := range found {
		switch vetPosture(manifest, a.Code()) {
		case "ignore":
		case "error":
			e.report(a.AsError())
			promoted = true
			shown++
		default:
			e.report(a)
			shown++
		}
	}
	return promoted, shown
}

// projectAdvisories gathers the advisory surface the pipeline commands
// share (design D5, adjudication Q4): the source graph's root module —
// already loaded and checked by the caller's own pipeline — then every
// test module the test runner's discovery finds, each as its own root
// over its own import graph, in the walk's path order. The source root's
// findings come first, the test modules' after (the mixed order no
// golden pins; disclosed). A test module that fails to parse or load
// reports and stops the stage: the advisory layer never widens a
// project's green face silently.
func (e *env) projectAdvisories(dir, rootPath string, root *ast.File, mods []typecheck.Module, depDirs map[string]string) ([]diag.Diagnostic, int) {
	found := typecheck.AdvisoriesProject(root, rootPath, mods)
	files, d, what, werr := collectTests(dir)
	if d != nil {
		e.report(*d)
		return nil, exitDiagnostic
	}
	if what != "" {
		return nil, e.boundary(what)
	}
	if werr != nil {
		return nil, e.fsError(werr)
	}
	for _, f := range files {
		key := strings.ReplaceAll(strings.TrimSuffix(f.Path, ".we"), "/", ".")
		testPath := filepath.Join(dir, filepath.FromSlash(f.Path))
		deps, code := e.loadGraph(dir, key, testPath, f.File, depDirs)
		if code != exitOK {
			return nil, code
		}
		found = append(found, typecheck.AdvisoriesTestRoot(f.File, testPath, key, deps)...)
	}
	return found, exitOK
}
