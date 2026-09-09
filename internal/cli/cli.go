// Package cli implements the we command surface (chapter 21, the we command
// surface requirement): argument parsing, the closed subcommand table, the
// global options, and the exit-code protocol. Streams are injected so the
// conformance runner stays in-process and deterministic.
package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/version"
)

// Exit codes. 0/1/2 carry the observable protocol of this slice: success,
// an E-severity diagnostic, and usage errors (the same class as chapter 21's
// "a subcommand outside the set is the shell's error" — no diagnostic, no
// JSON event). exitNotImplemented (70, sysexits EX_SOFTWARE) marks
// subcommands this reference build has not implemented yet — an
// implementation-transient boundary, not a spec surface; it disappears as
// implementation slices land.
const (
	exitOK             = 0
	exitDiagnostic     = 1
	exitUsage          = 2
	exitNotImplemented = 70
	// exitCompileFailure is we test's own third exit (chapter 21 R5):
	// the test compilation itself failed — the diagnostic is already
	// rendered and no test ran. The number is exitUsage's 2 (both are
	// never-ran classes); the runner's protocol earns it its own name.
	exitCompileFailure = 2
)

// subcommands is the closed set chapter 21 fixes. takesPath marks the
// subcommands whose [path] argument is resolved before dispatch (E1907).
// implemented lists the subcommands this reference build runs today.
var subcommands = map[string]struct {
	takesPath   bool
	implemented bool
}{
	"new":     {takesPath: false, implemented: true},
	"build":   {takesPath: true, implemented: true},
	"check":   {takesPath: true, implemented: true},
	"run":     {takesPath: true, implemented: true},
	"test":    {takesPath: true, implemented: true},
	"fmt":     {takesPath: true, implemented: true},
	"vet":     {takesPath: true, implemented: true},
	"doc":     {takesPath: true, implemented: true},
	"clean":   {takesPath: true, implemented: true},
	"version": {takesPath: false, implemented: true},
	"lsp":     {takesPath: true},
}

// env carries one run's parsed global options and output streams.
type env struct {
	stdout  io.Writer
	stderr  io.Writer
	json    bool
	color   string
	verbose bool
	// filter is we test's --filter pattern (M10b design D6): matched
	// CLI-side, the kept tests alone ride into the harness synthesis.
	filter string
	// The doc face (M11 design D6), doc-scoped like test's own trio:
	// --output redirects the pages into a directory, --check reports
	// documentation gaps without writing anything.
	docOutput string
	docCheck  bool
	// The exploration face (M10c design D9), test-scoped like --filter:
	// --explore arms the harness's exploration mode, --no-reduce disables
	// the POR dedup, --iterations N (both forms) pins the count. itersFlag
	// holds the explicit value only while itersSet says one was given;
	// exploreIters carries the resolved chain (flag > manifest > 100) once
	// the run's manifest face has decided it.
	explore      bool
	noReduce     bool
	itersFlag    int
	itersSet     bool
	exploreIters int
}

// Run parses args (the argv after the program name), dispatches one
// subcommand, and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	e := &env{stdout: stdout, stderr: stderr, color: "auto"}
	if len(args) == 0 {
		printUsage(e.stderr)
		return exitUsage
	}
	name, rest := args[0], args[1:]
	sub, known := subcommands[name]
	if !known {
		return e.usageErr("unknown subcommand %q", name)
	}
	positional, code := e.parseOptions(name, rest)
	if code != exitOK {
		return code
	}
	var info os.FileInfo // the resolved [path], for takesPath subcommands
	if sub.takesPath {
		// The [path] argument is resolved before dispatch, so E1907 fires
		// on not-yet-implemented subcommands too (R1: `we build nosuchdir`).
		if len(positional) > 1 {
			return e.usageErr("%s takes at most one [path] argument", name)
		}
		path := "."
		if len(positional) == 1 {
			path = positional[0]
		}
		var err error
		info, err = os.Stat(path)
		if err != nil {
			e.report(diag.Error("E1907", fmt.Sprintf("command path not found — %q", path)).
				WithHelp("Pass a project directory, a .we file, or nothing (the working directory)."))
			return exitDiagnostic
		}
	}
	if !sub.implemented {
		fmt.Fprintf(e.stderr, "we: %s is not implemented in this reference build yet\n", name)
		return exitNotImplemented
	}
	switch name {
	case "version":
		return e.runVersion(positional)
	case "new":
		return e.runNew(positional)
	case "check", "build", "run", "clean", "fmt", "vet", "doc":
		path := "."
		if len(positional) == 1 {
			path = positional[0]
		}
		switch name {
		case "check":
			return e.runCheck(path, info)
		case "build":
			return e.runBuild(path, info)
		case "run":
			return e.runRun(path, info)
		case "fmt":
			return e.runFmt(path, info)
		case "vet":
			return e.runVet(path, info)
		case "doc":
			return e.runDoc(path, info)
		default:
			return e.runClean(path, info)
		}
	case "test":
		path := "."
		if len(positional) == 1 {
			path = positional[0]
		}
		return e.runTest(path, info)
	}
	// Unreachable: every implemented subcommand is handled above.
	return e.usageErr("unknown subcommand %q", name)
}

// parseOptions splits flags from positional arguments. Global options are
// accepted anywhere after the subcommand, per `we <subcommand> [path]
// [options]`. The subcommand's own flags — we test's --filter (M10b
// design D6) and the exploration trio --explore/--iterations/--no-reduce
// (M10c design D9), we doc's --output/--check (M11 design D6) — parse
// only under their subcommand; elsewhere they stay unknown options.
func (e *env) parseOptions(sub string, args []string) ([]string, int) {
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			e.json = true
		case a == "--verbose":
			e.verbose = true
		case a == "--color":
			if i+1 >= len(args) {
				return nil, e.usageErr("--color wants a value (auto, always, or never)")
			}
			i++
			if code := e.setColor(args[i]); code != exitOK {
				return nil, code
			}
		case strings.HasPrefix(a, "--color="):
			if code := e.setColor(strings.TrimPrefix(a, "--color=")); code != exitOK {
				return nil, code
			}
		case sub == "test" && a == "--filter":
			if i+1 >= len(args) {
				return nil, e.usageErr("--filter wants a pattern")
			}
			i++
			e.filter = args[i]
		case sub == "test" && strings.HasPrefix(a, "--filter="):
			e.filter = strings.TrimPrefix(a, "--filter=")
		case sub == "test" && a == "--explore":
			e.explore = true
		case sub == "test" && a == "--no-reduce":
			e.noReduce = true
		case sub == "test" && a == "--iterations":
			if i+1 >= len(args) {
				return nil, e.usageErr("invalid --iterations value (want a positive integer)")
			}
			i++
			if code := e.setIterations(args[i]); code != exitOK {
				return nil, code
			}
		case sub == "test" && strings.HasPrefix(a, "--iterations="):
			if code := e.setIterations(strings.TrimPrefix(a, "--iterations=")); code != exitOK {
				return nil, code
			}
		case sub == "doc" && a == "--output":
			if i+1 >= len(args) {
				return nil, e.usageErr("--output wants a directory")
			}
			i++
			e.docOutput = args[i]
		case sub == "doc" && strings.HasPrefix(a, "--output="):
			e.docOutput = strings.TrimPrefix(a, "--output=")
		case sub == "doc" && a == "--check":
			e.docCheck = true
		case strings.HasPrefix(a, "--"):
			return nil, e.usageErr("unknown option %q", a)
		default:
			positional = append(positional, a)
		}
	}
	return positional, exitOK
}

func (e *env) setColor(v string) int {
	switch v {
	case "auto", "always", "never":
		e.color = v
		return exitOK
	}
	return e.usageErr("invalid --color value %q (want auto, always, or never)", v)
}

// setIterations reads one --iterations value: a decimal positive integer
// (M10c design D9). Anything else is the one usage-error shape — the
// golden's pinned message.
func (e *env) setIterations(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return e.usageErr("invalid --iterations value %q (want a positive integer)", v)
	}
	e.itersFlag, e.itersSet = n, true
	return exitOK
}

// report renders one diagnostic on the protocol faces: under --json as an
// event line on stdout only; otherwise as the human one-liner on stderr.
func (e *env) report(d diag.Diagnostic) {
	if e.json {
		fmt.Fprintln(e.stdout, d.JSON())
		return
	}
	fmt.Fprintln(e.stderr, d.Human())
}

// runVersion prints the compiler and specification versions in the fixed
// shape; the numbers themselves live in internal/version, never in the
// spec's text (chapter 0, mechanism neutrality).
func (e *env) runVersion(positional []string) int {
	if len(positional) > 0 {
		return e.usageErr("version takes no arguments")
	}
	if e.json {
		line := fmt.Sprintf(`{"type":"version","compiler":%q,"spec":%q`, version.CompilerVersion, version.SpecVersion)
		if e.verbose {
			line += fmt.Sprintf(`,"go":%q,"llvm":%q`, runtime.Version(), version.LLVMPin)
		}
		fmt.Fprintln(e.stdout, line+"}")
		return exitOK
	}
	fmt.Fprintf(e.stdout, "we %s\n", version.CompilerVersion)
	fmt.Fprintf(e.stdout, "spec %s\n", version.SpecVersion)
	if e.verbose {
		fmt.Fprintf(e.stdout, "go %s\n", runtime.Version())
		fmt.Fprintf(e.stdout, "llvm %s\n", version.LLVMPin)
	}
	return exitOK
}

// runNew creates the project skeleton: a manifest, src/main.we with a
// `pub fn main` of chapter 15's exact shape, and one empty test module
// under tests/. The name is validated first — an illegal name creates
// nothing (R1).
func (e *env) runNew(positional []string) int {
	if len(positional) != 1 {
		return e.usageErr("usage: we new <name>")
	}
	name := positional[0]
	if !validProjectName(name) {
		e.report(diag.Error("E1904", fmt.Sprintf("invalid project name — %q", name)).
			WithHelp("Use lowercase letters, digits, and hyphens per chapter 1's naming convention."))
		return exitDiagnostic
	}
	if _, err := os.Stat(name); err == nil {
		return e.usageErr("target directory %q already exists", name)
	}
	files := []struct{ path, content string }{
		{name + "/we.toml", manifestSkeleton},
		{name + "/src/main.we", mainSkeleton},
		{name + "/tests/main_test.we", testSkeleton},
	}
	for _, f := range files {
		if err := os.MkdirAll(dirOf(f.path), 0o755); err != nil {
			return e.fsError(err)
		}
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			return e.fsError(err)
		}
		if e.verbose {
			fmt.Fprintf(e.stdout, "created %s\n", f.path)
		}
	}
	return exitOK
}

func (e *env) fsError(err error) int {
	fmt.Fprintf(e.stderr, "we: %v\n", err)
	return exitDiagnostic
}

// validProjectName reports whether name is a legal project name: non-empty,
// lowercase letters, digits, and hyphens only — chapter 22's concrete
// spelling of chapter 1's convention (registry entry E1904). Leading or
// trailing hyphens are not restricted: the spec fixes no such rule, and none
// is invented here (roadmap follow-up 1).
func validProjectName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
		default:
			return false
		}
	}
	return true
}

func dirOf(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return "."
}

func (e *env) usageErr(format string, a ...any) int {
	fmt.Fprintf(e.stderr, "we: "+format+"\n", a...)
	printUsage(e.stderr)
	return exitUsage
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: we <subcommand> [path] [options]")
	fmt.Fprintln(w, "subcommands: new build check run test fmt vet doc clean version lsp")
	fmt.Fprintln(w, "options: --json --color auto|always|never --verbose")
}
