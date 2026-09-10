package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ltlvtao/welang/internal/codegen"
	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/typecheck"
	"github.com/ltlvtao/welang/internal/version"
	"github.com/ltlvtao/welang/runtime"
)

// Boundary Whats owned by the build surface (the codegen table lives in
// internal/codegen). Each completes `we: %s are not implemented in this
// reference build yet`.
const (
	whatSingleFileBuild  = "single-file builds (spec gap; roadmap follow-up)"
	whatLibraryArtifacts = "library artifacts (chapter 21)"
)

// runBuild builds one project directory into build/<name> (chapter 21 R2:
// the pipeline continues through code generation), or honest-stops at the
// artifact-naming boundary for a single file (Q3: no manifest, no name).
func (e *env) runBuild(path string, info os.FileInfo) int {
	if code := e.toolchainGate(); code != exitOK {
		return code
	}
	if !info.IsDir() {
		if file, code := e.loadFile(path); file == nil {
			return code
		}
		// A single-file build has no manifest and so no artifact name —
		// the naming boundary is the executable's own face, a test module
		// included (its run tower is `we test`, M10b design D6).
		return e.boundary(whatSingleFileBuild)
	}
	_, code := e.buildProject(path)
	return code
}

// runRun builds (same faces as build), then executes the artifact with the
// child's stdio passed straight through to the streams we itself was given
// — the program's output is the output — and propagates its exit code
// (chapter 15 R6's Err-path 1 arrives as exactly that).
func (e *env) runRun(path string, info os.FileInfo) int {
	if code := e.toolchainGate(); code != exitOK {
		return code
	}
	if !info.IsDir() {
		if file, code := e.loadFile(path); file == nil {
			return code
		}
		return e.boundary(whatSingleFileBuild)
	}
	artifact, code := e.buildProject(path)
	if code != exitOK {
		return code
	}
	// The artifact path rides the project path the caller named, while the
	// child's Dir is the project itself — exec resolves a relative Path
	// inside Dir, so `we run proj` from outside doubles the prefix and the
	// binary is never found. Resolve absolute first. The conformance run
	// goldens ride absolute temp dirs and never crossed it; the M12
	// black-box battery's relative-path form caught it.
	abs, err := filepath.Abs(artifact)
	if err != nil {
		return e.fsError(err)
	}
	cmd := exec.Command(abs)
	cmd.Dir = path
	cmd.Stdin = os.Stdin
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			if code := ee.ExitCode(); code >= 0 {
				return code
			}
			fmt.Fprintf(e.stderr, "we: %s terminated by signal\n", filepath.ToSlash(artifact))
			return exitDiagnostic
		}
		return e.fsError(err)
	}
	return exitOK
}

// runClean removes the project's build/ directory outright (chapter 21 R1):
// idempotent when absent, source and manifest untouched. It reads no
// manifest — ch22 R5 exempts clean from acquisition — so a directory
// without we.toml cleans just the same. A file path is a usage error: clean
// has no single-file face to point at (design D1).
func (e *env) runClean(path string, info os.FileInfo) int {
	if !info.IsDir() {
		return e.usageErr("clean wants a project directory, got %q", path)
	}
	buildDir := filepath.Join(path, "build")
	if _, err := os.Stat(buildDir); err == nil {
		if e.verbose {
			// List what the removal touches, lexically, files only, before
			// it happens — the directory line comes last.
			var files []string
			filepath.WalkDir(buildDir, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					files = append(files, p)
				}
				return nil
			})
			for _, f := range files {
				rel, err := filepath.Rel(path, f)
				if err != nil {
					return e.fsError(err)
				}
				fmt.Fprintf(e.stdout, "we: removed %s\n", filepath.ToSlash(rel))
			}
		}
		if err := os.RemoveAll(buildDir); err != nil {
			return e.fsError(err)
		}
		if e.verbose {
			rel, err := filepath.Rel(path, buildDir)
			if err != nil {
				return e.fsError(err)
			}
			// The directory line keeps its trailing slash — it names the
			// directory, not a file inside it (design D1).
			fmt.Fprintf(e.stdout, "we: removed %s/\n", filepath.ToSlash(rel))
		}
	}
	// An absent build/ removes an empty set: still success, and under
	// --verbose not one line (design D1).
	return exitOK
}

// toolchainGate verifies the pinned LLVM toolchain before any project
// loading (design D7): the clang on PATH must report exactly the pin, or
// the build surface refuses to start — a toolchain failure, not a
// diagnostic and not a boundary. M12 (design D5) widens the gate to the
// nm face the foreign binding's verification rides on.
func (e *env) toolchainGate() int {
	if err := checkClangVersion("clang"); err != nil {
		fmt.Fprintf(e.stderr, "we: LLVM toolchain not available (want clang %s): %v\n", version.LLVMPin, err)
		return exitDiagnostic
	}
	if err := checkNmVersion("llvm-nm"); err != nil {
		fmt.Fprintf(e.stderr, "we: LLVM toolchain not available (want llvm-nm %s): %v\n", version.LLVMPin, err)
		return exitDiagnostic
	}
	return exitOK
}

// checkNmVersion parses `LLVM version x.y.z` out of llvm-nm's --version
// output (the Ubuntu toolchain's second line reads `Ubuntu LLVM version
// 21.1.8`) and requires the exact pin — the clang gate's own discipline.
// The path is a parameter so tests can fake the output.
func checkNmVersion(path string) error {
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("running %s --version: %w", path, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for i := 0; i+2 < len(fields); i++ {
			if fields[i] == "LLVM" && fields[i+1] == "version" {
				if fields[i+2] == version.LLVMPin {
					return nil
				}
				return fmt.Errorf("%s --version reports %q", path, fields[i+2])
			}
		}
	}
	return fmt.Errorf("%s --version output carries no version line", path)
}

// checkClangVersion parses `clang version x.y.z` out of the --version
// output and requires the exact pin (the distro suffix, like (6ubuntu1),
// is not part of the comparison). The path is a parameter so tests can
// fake the output; production always passes "clang" on PATH.
func checkClangVersion(path string) error {
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("running %s --version: %w", path, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for i := 0; i+2 < len(fields); i++ {
			if fields[i] == "clang" && fields[i+1] == "version" {
				if fields[i+2] == version.LLVMPin {
					return nil
				}
				return fmt.Errorf("%s --version reports %q", path, fields[i+2])
			}
		}
	}
	return fmt.Errorf("%s --version output carries no version line", path)
}

// buildProject runs the shared pipeline through code generation and the
// clang driver (design D6), leaving the intermediates in build/ for
// inspection. On success it returns the artifact path and prints the
// success faces; a failure is already reported, with its exit code.
func (e *env) buildProject(dir string) (string, int) {
	manifest, file, name, mods, depRoots, code := e.loadProject(dir, true)
	if file == nil {
		return "", code
	}
	// The advisory layer rides before code generation (M11 design D5):
	// warnings render and the build continues; a promoted finding stops
	// exactly as an error does — no artifact, the run's exit 1.
	found, code := e.projectAdvisories(dir, filepath.Join(dir, "src", "main.we"), file, mods, depRoots)
	if code != exitOK {
		return "", code
	}
	if promoted, _ := e.advise(manifest, found); promoted {
		return "", exitDiagnostic
	}
	// The program face of design D1: dependency modules in graph order,
	// the root last. The std modules drop out here — their call faces
	// ride the emitter's own std table, their module bodies are the
	// check stage's material, never IR.
	prog := append(e.programModules(mods), codegen.ProgModule{Key: "main", ID: name, File: file})
	ir, ni := codegen.EmitProgram(codegen.ModeBuild, prog)
	if ni != nil {
		return "", e.boundary(ni.What)
	}
	buildDir := filepath.Join(dir, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", e.fsError(err)
	}
	// The native face (M12 design D5): discover and compile native/,
	// then verify every declared foreign name against the objects'
	// symbols — E1906 renders every missing name and no artifact links
	// with an unresolved binding.
	nativeObjs, code := e.nativeObjects(dir, buildDir, mods, prog)
	if code != exitOK {
		return "", code
	}
	artifact, code := e.compileProgram(buildDir, name, ir, nativeObjs)
	if code != exitOK {
		return "", code
	}
	if e.verbose {
		rel, err := filepath.Rel(dir, artifact)
		if err != nil {
			return "", e.fsError(err)
		}
		fmt.Fprintf(e.stdout, "we: built %s\n", filepath.ToSlash(rel))
	}
	return artifact, exitOK
}

// nativeSources lists the project's native/ C sources, base names in
// byte order (os.ReadDir's own order); an absent directory is an empty
// set, not an error — ok reports the directory's presence so the build
// log can distinguish an empty native/ from none at all.
func nativeSources(dir string) ([]string, bool) {
	entries, err := os.ReadDir(filepath.Join(dir, "native"))
	if err != nil {
		return nil, false
	}
	var srcs []string
	for _, en := range entries {
		if en.IsDir() || filepath.Ext(en.Name()) != ".c" {
			continue
		}
		srcs = append(srcs, en.Name())
	}
	return srcs, true
}

// definedSymbols lists the names llvm-nm reports as defined across the
// objects: one `address type name` line per symbol, the name the last
// field (the exact line shape the pinned toolchain prints).
func definedSymbols(nm string, objs []string) (map[string]bool, error) {
	defined := make(map[string]bool)
	if len(objs) == 0 {
		return defined, nil
	}
	args := append([]string{"--defined-only"}, objs...)
	out, err := exec.Command(nm, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running %s: %w", nm, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		defined[fields[len(fields)-1]] = true
	}
	return defined, nil
}

// missingForeignNames is the difference the E1906 report renders: every
// declared foreign name no native object defines, in declaration order.
func missingForeignNames(declared []codegen.ForeignName, defined map[string]bool) []codegen.ForeignName {
	var missing []codegen.ForeignName
	for _, n := range declared {
		if !defined[n.Name] {
			missing = append(missing, n)
		}
	}
	return missing
}

// helpE1906 is the registry's remediation for the link face, quoted per
// the loader's helps discipline.
const helpE1906 = "Provide the native library or symbol the declaration names, or correct the declaration to the symbol that exists."

// nativeObjects runs the native face of design D5: compile every
// native/*.c into build/, list the objects' defined symbols, and verify
// the program's declared foreign names against them — each missing name
// is E1906 (anchored at its declaration), all rendered before the exit,
// and the caller links nothing. prog is the whole program face (deps in
// graph order plus the root keyed "main"); the mods side carries the
// source paths the diagnostics anchor against.
func (e *env) nativeObjects(dir, buildDir string, mods []typecheck.Module, prog []codegen.ProgModule) ([]string, int) {
	declared := codegen.ForeignNames(prog)
	srcs, _ := nativeSources(dir)
	var objs []string
	for _, f := range srcs {
		obj := filepath.Join(buildDir, "native-"+strings.TrimSuffix(f, ".c")+".o")
		if code := e.runClang("clang", "-c", filepath.Join(dir, "native", f), "-o", obj); code != exitOK {
			return nil, code
		}
		objs = append(objs, obj)
	}
	if len(declared) == 0 {
		return objs, exitOK // nothing binds: no verification face
	}
	defined, err := definedSymbols("llvm-nm", objs)
	if err != nil {
		fmt.Fprintf(e.stderr, "we: %v\n", err)
		return nil, exitDiagnostic
	}
	modPaths := make(map[string]string, len(mods)+1)
	for _, m := range mods {
		modPaths[m.Key] = m.Path
	}
	modPaths["main"] = filepath.Join(dir, "src", "main.we")
	missing := missingForeignNames(declared, defined)
	for _, n := range missing {
		path := modPaths[n.Module]
		e.report(diag.Error("E1906",
			fmt.Sprintf("unresolved native symbol — %q (declared at %s:%d)", n.Name, path, n.Line)).
			At(path, n.Line, n.Col).
			WithHelp(helpE1906))
	}
	if len(missing) > 0 {
		return nil, exitDiagnostic
	}
	return objs, exitOK
}

// programModules lifts the loader's modules into the program face
// buildProject assembles — the non-std dependency modules in graph
// order. The root module joins at the call site, keyed "main".
func (e *env) programModules(mods []typecheck.Module) []codegen.ProgModule {
	prog := make([]codegen.ProgModule, 0, len(mods))
	for _, m := range mods {
		if m.Key == "std" || strings.HasPrefix(m.Key, "std.") {
			continue
		}
		prog = append(prog, codegen.ProgModule{Key: m.Key, ID: m.Key, File: m.File})
	}
	return prog
}

// compileProgram writes one program's IR and the runtime sources into
// build/ and runs the pinned clang sequence (design D6's three-step
// order), leaving the intermediates for inspection. base names both the
// IR file (<base>.ll) and the linked artifact (<base>); the native
// objects join the final link's argument list (M12 design D5). A failure
// is already reported, with its exit code.
func (e *env) compileProgram(buildDir, base, ir string, nativeObjs []string) (string, int) {
	for _, f := range []struct{ path, content string }{
		{filepath.Join(buildDir, base+".ll"), ir},
		{filepath.Join(buildDir, "rt-startup.c"), weruntime.StartupSource},
		{filepath.Join(buildDir, "sched.h"), weruntime.SchedHeader},
		{filepath.Join(buildDir, "rt-sched.c"), weruntime.SchedSource},
		{filepath.Join(buildDir, "rt-conc.c"), weruntime.ConcSource},
		{filepath.Join(buildDir, "rt-test.c"), weruntime.TestSource},
		{filepath.Join(buildDir, "rt-gc.c"), weruntime.GCSource},
		{filepath.Join(buildDir, "rt-io.c"), weruntime.IOSource},
		{filepath.Join(buildDir, "str.h"), weruntime.StrHeader},
		{filepath.Join(buildDir, "rt-str.c"), weruntime.StrSource},
		{filepath.Join(buildDir, "list.h"), weruntime.ListHeader},
		{filepath.Join(buildDir, "rt-list.c"), weruntime.ListSource},
	} {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			return "", e.fsError(err)
		}
	}
	// The clang driver sequence of design D6: the runtime sources to
	// objects, then one driver call that compiles the .ll and links.
	// M9b inserts the scheduler and the wait-machine sources ahead of
	// the collector — startup enters __we_sched_boot, so the link needs
	// them even for a plain M8 body. M10b adds the clock face test.c owns
	// (the scheduler reads it, so every sched link carries it). M12
	// appends the native objects after the runtime objects (design D5).
	// -Wno-override-module keeps the no-triple IR (D4) from warning on
	// stderr — the success faces are silent, and a warning is output.
	linkArgs := []string{"-Wno-override-module",
		filepath.Join(buildDir, base+".ll"),
		filepath.Join(buildDir, "rt-startup.o"),
		filepath.Join(buildDir, "rt-sched.o"),
		filepath.Join(buildDir, "rt-conc.o"),
		filepath.Join(buildDir, "rt-test.o"),
		filepath.Join(buildDir, "rt-gc.o"),
		filepath.Join(buildDir, "rt-io.o"),
		filepath.Join(buildDir, "rt-str.o"),
		filepath.Join(buildDir, "rt-list.o"),
	}
	linkArgs = append(linkArgs, nativeObjs...)
	linkArgs = append(linkArgs, "-o", filepath.Join(buildDir, base))
	for _, c := range []struct {
		name string
		args []string
	}{
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-startup.c"), "-o", filepath.Join(buildDir, "rt-startup.o")}},
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-sched.c"), "-o", filepath.Join(buildDir, "rt-sched.o")}},
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-conc.c"), "-o", filepath.Join(buildDir, "rt-conc.o")}},
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-test.c"), "-o", filepath.Join(buildDir, "rt-test.o")}},
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-gc.c"), "-o", filepath.Join(buildDir, "rt-gc.o")}},
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-io.c"), "-o", filepath.Join(buildDir, "rt-io.o")}},
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-str.c"), "-o", filepath.Join(buildDir, "rt-str.o")}},
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-list.c"), "-o", filepath.Join(buildDir, "rt-list.o")}},
		{"clang", linkArgs},
	} {
		if code := e.runClang(c.name, c.args...); code != exitOK {
			return "", code
		}
	}
	return filepath.Join(buildDir, base), exitOK
}

// runClang runs one clang invocation; a non-zero exit reports one line —
// the command's short name and the first line of its output — exit 1, the
// toolchain's own failure class (design D6).
func (e *env) runClang(name string, args ...string) int {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		first := strings.SplitN(string(out), "\n", 2)[0]
		if first == "" {
			first = err.Error()
		}
		fmt.Fprintf(e.stderr, "we: %s failed: %s\n", name, first)
		return exitDiagnostic
	}
	return exitOK
}
