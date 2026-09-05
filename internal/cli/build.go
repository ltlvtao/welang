package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ltlvtao/welang/internal/codegen"
	"github.com/ltlvtao/welang/internal/version"
	"github.com/ltlvtao/welang/runtime"
)

// Boundary Whats owned by the build surface (the codegen table lives in
// internal/codegen). Each completes `we: %s are not implemented in this
// reference build yet`.
const (
	whatSingleFileBuild  = "single-file builds (spec gap; roadmap follow-up)"
	whatLibraryArtifacts = "library artifacts (chapter 21)"
	whatNonEmptyDeps     = "pipeline commands with a non-empty dependency set (chapter 22)"
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
	cmd := exec.Command(artifact)
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
// diagnostic and not a boundary.
func (e *env) toolchainGate() int {
	if err := checkClangVersion("clang"); err != nil {
		fmt.Fprintf(e.stderr, "we: LLVM toolchain not available (want clang %s): %v\n", version.LLVMPin, err)
		return exitDiagnostic
	}
	return exitOK
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
	file, name, code := e.loadProject(dir, true)
	if file == nil {
		return "", code
	}
	ir, ni := codegen.Emit(file, name)
	if ni != nil {
		return "", e.boundary(ni.What)
	}
	buildDir := filepath.Join(dir, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", e.fsError(err)
	}
	for _, f := range []struct{ path, content string }{
		{filepath.Join(buildDir, name+".ll"), ir},
		{filepath.Join(buildDir, "rt-startup.c"), weruntime.StartupSource},
		{filepath.Join(buildDir, "rt-alloc.c"), weruntime.AllocSource},
	} {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			return "", e.fsError(err)
		}
	}
	// The clang driver sequence of design D6: both runtime sources to
	// objects, then one driver call that compiles the .ll and links.
	// -Wno-override-module keeps the no-triple IR (D4) from warning on
	// stderr — the success faces are silent, and a warning is output.
	for _, c := range []struct {
		name string
		args []string
	}{
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-startup.c"), "-o", filepath.Join(buildDir, "rt-startup.o")}},
		{"clang", []string{"-c", filepath.Join(buildDir, "rt-alloc.c"), "-o", filepath.Join(buildDir, "rt-alloc.o")}},
		{"clang", []string{"-Wno-override-module",
			filepath.Join(buildDir, name+".ll"),
			filepath.Join(buildDir, "rt-startup.o"),
			filepath.Join(buildDir, "rt-alloc.o"),
			"-o", filepath.Join(buildDir, name)}},
	} {
		if code := e.runClang(c.name, c.args...); code != exitOK {
			return "", code
		}
	}
	artifact := filepath.Join(buildDir, name)
	if e.verbose {
		rel, err := filepath.Rel(dir, artifact)
		if err != nil {
			return "", e.fsError(err)
		}
		fmt.Fprintf(e.stdout, "we: built %s\n", filepath.ToSlash(rel))
	}
	return artifact, exitOK
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
