package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/codegen"
	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/parser"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// The we test runner (chapter 21 R5, M10b design D6). The pure faces —
// the discovery order, the --filter read, the subprocess exit mapping —
// carry no state and were pinned test-first; the run chain rides them:
// discovery, --filter, each test module its own root over the shared
// import-graph machine, one synthesized harness program, the pinned
// clang build, and one subprocess whose report is its own render on the
// streams we were given.

// discoveredTest is one test block the runner will run: the module file
// that declares it (slash-separated, relative to the project root) and
// the test's description string.
type discoveredTest struct {
	File string
	Name string
}

// testFile is one discovered test module: its path relative to the
// project root (slash form — the report lines and the module key both
// derive from it) and its parsed file.
type testFile struct {
	Path string
	File *ast.File
}

// collectTests walks the default test set: every *_test.we under the
// project's tests/, files in the walk's path order, declarations in
// source order within (chapter 21 R5's default set; the cross-file
// order is the implementation's reading — sorted paths, design D6's
// disclosure). Both faces of the runner ride this one walk — the pure
// discovery face and the run chain — so the order rule lives in exactly
// one place. A failure rides out for the caller to report: a parse
// diagnostic, a parse boundary What, or the walk's own error; a project
// without tests/ is an empty set.
func collectTests(dir string) (files []testFile, stop *diag.Diagnostic, what string, walkErr error) {
	root := filepath.Join(dir, "tests")
	if _, err := os.Stat(root); err != nil {
		return nil, nil, "", nil
	}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, "_test.we") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		file, pd, ni := parser.Parse(p, src)
		if pd != nil {
			stop = pd
			return errStopWalk
		}
		if ni != nil {
			what = ni.What
			return errStopWalk
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		files = append(files, testFile{Path: filepath.ToSlash(rel), File: file})
		return nil
	})
	if err != nil {
		if errors.Is(err, errStopWalk) {
			return nil, stop, what, nil
		}
		return nil, nil, "", err
	}
	return files, nil, "", nil
}

// errStopWalk aborts the discovery walk after a parse failure decided
// the outcome (WalkDir's contract carries it as the walk error; the
// caller maps every abort alike).
var errStopWalk = errors.New("stop walk")

// discoverTests collects the default test set as its run list: the pure
// discovery face over collectTests' walk. A parse failure is the
// loader's failure class — the run chain reports it; this face returns
// the exit code.
func discoverTests(dir string) ([]discoveredTest, int) {
	files, d, what, err := collectTests(dir)
	if d != nil || what != "" || err != nil {
		return nil, exitDiagnostic
	}
	return testNames(files), exitOK
}

// testNames lists one walk's test blocks in run order — the discovery
// face's mapping and the run chain's --filter input (one walk, one
// order rule, one name mapping).
func testNames(files []testFile) []discoveredTest {
	var ts []discoveredTest
	for _, f := range files {
		for _, it := range f.File.Items {
			if td, ok := it.(*ast.TestDecl); ok {
				ts = append(ts, discoveredTest{File: f.Path, Name: td.Desc})
			}
		}
	}
	return ts
}

// filterTests applies --filter: a Go regexp, unanchored, matched against
// the test name (the file path is not part of the match — the flag
// filters tests, not files). An invalid pattern is a usage error; an
// empty match is an empty run, not an error.
func filterTests(pattern string, ts []discoveredTest) ([]discoveredTest, int) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, exitUsage
	}
	var kept []discoveredTest
	for _, t := range ts {
		if re.MatchString(t.Name) {
			kept = append(kept, t)
		}
	}
	return kept, exitOK
}

// mapTestExit maps one test subprocess's outcome to the runner's exit:
// a clean run passes 0 through; every other outcome — a failure exit or
// a runtime abort (the panic path's SIGABRT) — is 1, the run's own
// failure class. The subprocess's stderr already carries the story.
func mapTestExit(err error) int {
	if err == nil {
		return exitOK
	}
	return exitDiagnostic
}

// runTest dispatches the test runner (chapter 21 R5): a directory is
// the project run, a file the single-file run. The toolchain gate rides
// first — this subcommand builds.
func (e *env) runTest(path string, info os.FileInfo) int {
	if code := e.toolchainGate(); code != exitOK {
		return code
	}
	if info.IsDir() {
		return e.runTestProject(path)
	}
	if !strings.HasSuffix(path, ".we") {
		return e.usageErr("test wants a .we file, got %q", path)
	}
	return e.runTestSingle(path)
}

// runTestProject runs a project's default test set (design D6): the
// manifest's shared validations first — a library project keeps going,
// its executable is the synthesized harness, not the artifact kind's —
// then the discovery walk, --filter, and the compile chain: each test
// module its own root over the shared import-graph machine, checked
// root by root (no main convention at a test root — the harness owns
// the entry), the union program assembled deps-first with each module
// once across the roots. Any diagnostic the compile chain reports is
// the test compile's own exit 2 — a compile failure never runs a test.
func (e *env) runTestProject(dir string) int {
	manifestKeys, code := e.loadManifest(dir, false)
	if code != exitOK {
		if code == exitDiagnostic {
			return exitCompileFailure
		}
		return code // the dependency boundary (chapter 22 R5) passes through
	}
	// The exploration count resolves once the manifest face has spoken
	// (M10c design D9): flag > [test].explore-iterations > 100.
	e.exploreIters = resolveExploreIters(e.itersSet, e.itersFlag, manifestKeys["test.explore-iterations"])
	files, d, what, werr := collectTests(dir)
	if d != nil {
		e.report(*d)
		return exitCompileFailure
	}
	if what != "" {
		return e.boundary(what)
	}
	if werr != nil {
		return e.fsError(werr)
	}
	files, code = e.applyFilter(files)
	if code != exitOK {
		return code
	}
	name := manifestKeys["name"]
	prog := make([]codegen.ProgModule, 0, len(files))
	depSeen := map[string]bool{}
	// The advisory findings of the roots this run compiles (M11 design
	// D5): collected per test module beside its own check — the graph is
	// already in hand — and rendered after the whole set checks clean, a
	// promoted finding mapping to the compile-failure exit before any
	// harness is built.
	var advisories []diag.Diagnostic
	for _, f := range files {
		key := strings.ReplaceAll(strings.TrimSuffix(f.Path, ".we"), "/", ".")
		rootPath := filepath.Join(dir, filepath.FromSlash(f.Path))
		deps, code := e.loadGraph(dir, key, rootPath, f.File)
		if code != exitOK {
			if code == exitDiagnostic {
				return exitCompileFailure
			}
			return code
		}
		td, tni := typecheck.CheckTestRoot(f.File, rootPath, key, deps)
		if td != nil {
			e.report(*td)
			return exitCompileFailure
		}
		if tni != nil {
			return e.boundary(tni.What)
		}
		advisories = append(advisories, typecheck.AdvisoriesTestRoot(f.File, rootPath, key, deps)...)
		for _, m := range deps {
			// The std modules ride the graph for the type stage; the
			// program face filters them — their call faces are the
			// emitter's own std table (the build face's rule).
			if m.Key == "std" || strings.HasPrefix(m.Key, "std.") {
				continue
			}
			if depSeen[m.Key] {
				continue // one define per module across the test roots
			}
			depSeen[m.Key] = true
			prog = append(prog, codegen.ProgModule{Key: m.Key, ID: m.Key, File: m.File})
		}
		// The test module's ID carries the manifest name: the program
		// this run builds is the project's own.
		prog = append(prog, codegen.ProgModule{Key: key, ID: name, Path: f.Path, File: f.File})
	}
	if promoted, _ := e.advise(manifestKeys, advisories); promoted {
		return exitCompileFailure
	}
	if len(prog) == 0 {
		// The empty default set still runs: a zero-test driver reports
		// total 0 and exits 0 (chapter 21) — the program stays non-empty
		// with a bare root.
		prog = append(prog, codegen.ProgModule{Key: "main", ID: name, File: &ast.File{}})
	}
	ir, ni := codegen.EmitProgram(codegen.ModeTest, prog)
	if ni != nil {
		return e.boundary(ni.What)
	}
	buildDir := filepath.Join(dir, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return e.fsError(err)
	}
	artifact, code := e.compileProgram(buildDir, name+".test", ir)
	if code != exitOK {
		return code
	}
	return e.runTestBinary(artifact, dir)
}

// runTestSingle runs one .we file as its own single-file compilation
// (chapter 21): the named file is the whole test set — std imports
// resolve from the built-in modules and every other import the check
// face already rejected — so a file without test blocks is the empty
// set's empty run (design D6's literal reading: total 0, exit 0). The
// harness builds in a throwaway directory a single-file run deletes on
// the way out — it owns no build/ (there is no manifest to name one).
func (e *env) runTestSingle(path string) int {
	file, code := e.loadFile(path)
	if file == nil {
		if code == exitDiagnostic {
			return exitCompileFailure
		}
		return code
	}
	files, code := e.applyFilter([]testFile{{Path: filepath.ToSlash(path), File: file}})
	if code != exitOK {
		return code
	}
	// The advisory layer over the filtered face — the set this run
	// compiles (Q4); no manifest here, so nothing can promote and the
	// findings report without stopping the run.
	e.advise(nil, typecheck.Advisories(files[0].File, path, typecheck.SingleFile))
	base := strings.TrimSuffix(filepath.Base(path), ".we")
	prog := []codegen.ProgModule{{
		Key:  "main", // the single-file face's module key (Emit's own)
		ID:   base,
		Path: filepath.ToSlash(path),
		File: files[0].File,
	}}
	ir, ni := codegen.EmitProgram(codegen.ModeTest, prog)
	if ni != nil {
		return e.boundary(ni.What)
	}
	tmp, err := os.MkdirTemp("", "we-test-")
	if err != nil {
		return e.fsError(err)
	}
	defer os.RemoveAll(tmp)
	artifact, code := e.compileProgram(tmp, base+".test", ir)
	if code != exitOK {
		return code
	}
	// A single-file run owns no manifest, so the count's chain stops at
	// the flag or the built-in (M10c design D9).
	e.exploreIters = resolveExploreIters(e.itersSet, e.itersFlag, "")
	return e.runTestBinary(artifact, "")
}

// applyFilter bakes --filter into the synthesis (design D6): the match
// runs CLI-side over the walk's names, and the files strip down to the
// kept tests — the unmatched TestDecls never reach the compile, so a
// broken block outside the filter does not fail the filtered run. An
// empty filter keeps everything; an invalid pattern is a usage error.
func (e *env) applyFilter(files []testFile) ([]testFile, int) {
	if e.filter == "" {
		return files, exitOK
	}
	keptList, code := filterTests(e.filter, testNames(files))
	if code != exitOK {
		return nil, e.usageErr("invalid --filter pattern %q", e.filter)
	}
	kept := make(map[string]bool, len(keptList))
	for _, t := range keptList {
		kept[t.Name] = true
	}
	for i := range files {
		var items []ast.Item
		for _, it := range files[i].File.Items {
			if td, ok := it.(*ast.TestDecl); ok && !kept[td.Desc] {
				continue
			}
			items = append(items, it)
		}
		if len(items) != len(files[i].File.Items) {
			// A shallow copy with the stripped items — the docs index may
			// dangle past a removed decl, and no later stage reads it
			// (the check stage already ran on nothing here; codegen
			// never opens Docs).
			nf := *files[i].File
			nf.Items = items
			files[i].File = &nf
		}
	}
	return files, exitOK
}

// runTestBinary executes the harness once: the child's stdio rides the
// streams we itself were given — the report is the child's render on
// both protocol faces, --json and the exploration face through the argv
// where the child reads them — and the exit maps per the runner's
// protocol (chapter 21): the child's 0 passes through, its 1 is the
// run's 1, an abort is 1 again (mapTestExit; a runtime failure, not a
// compile one).
func (e *env) runTestBinary(artifact, workDir string) int {
	argv := append([]string{artifact}, e.testBinaryArgs()...)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = workDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return mapTestExit(err)
		}
		return e.fsError(err)
	}
	return exitOK
}

// resolveExploreIters is the exploration count's default chain (M10c
// design D9): an explicit --iterations wins, the manifest's
// [test].explore-iterations next, the built-in 100 last. The manifest
// value arrives already E1903-validated on the project face; the
// defensive re-check keeps the single-file face (no manifest) and any
// future caller honest.
func resolveExploreIters(flagSet bool, flag int, manifest string) int {
	if flagSet {
		return flag
	}
	if n, err := strconv.Atoi(manifest); err == nil && n > 0 {
		return n
	}
	return 100
}

// testBinaryArgs renders the harness argv beyond the binary's own name:
// --json first (the M10b order), then the exploration face as a unit —
// --explore and the resolved --iterations ride together (the manifest
// default is resolved CLI-side before the child exists), --no-reduce
// when given. A normal run adds nothing.
func (e *env) testBinaryArgs() []string {
	var args []string
	if e.json {
		args = append(args, "--json")
	}
	if e.explore {
		args = append(args, "--explore", "--iterations", strconv.Itoa(e.exploreIters))
		if e.noReduce {
			args = append(args, "--no-reduce")
		}
	}
	return args
}
