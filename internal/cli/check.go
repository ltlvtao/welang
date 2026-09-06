package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/parser"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// runCheck runs the check subcommand: the lexical, parsing, and type
// stages over one source file, or over a project directory (its we.toml
// manifest, then the source root's root module src/main.we). The first
// diagnostic exits 1 on the protocol faces; a form a ratified chapter
// owns that this build has not implemented yet stops at its boundary —
// a plain stderr line without a diagnostic event, under --json too:
// implementation-transient, not a spec surface (chapter 21's exit 70).
func (e *env) runCheck(path string, info os.FileInfo) int {
	if info.IsDir() {
		return e.runCheckProject(path)
	}
	if !strings.HasSuffix(path, ".we") {
		return e.usageErr("check wants a .we file, got %q", path)
	}
	if file, code := e.loadFile(path); file == nil {
		return code
	}
	return e.checkPassed(1)
}

// runCheckProject checks a project directory through the shared loader
// and stops at the type stage — check produces no artifact (chapter 21
// R2), so there is no artifact face to stop at.
func (e *env) runCheckProject(dir string) int {
	_, _, modules, code := e.loadProject(dir, false)
	if code != exitOK {
		return code
	}
	return e.checkPassed(modules)
}

// loadFile runs the single-file pipeline: read, parse, and the type stage
// in single-file mode. A nil file means the failure is already reported,
// with its exit code. check and build/run share it; where a clean file
// goes next is each subcommand's own business.
func (e *env) loadFile(path string) (*ast.File, int) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, e.fsError(err)
	}
	file, d, ni := parser.Parse(path, src)
	if d != nil {
		e.report(*d)
		return nil, exitDiagnostic
	}
	if ni != nil {
		return nil, e.boundary(ni.What)
	}
	td, tni := typecheck.Check(file, path, typecheck.SingleFile)
	if td != nil {
		e.report(*td)
		return nil, exitDiagnostic
	}
	if tni != nil {
		return nil, e.boundary(tni.What)
	}
	return file, exitOK
}

// loadProject runs the project pipeline up through the type stage
// (chapter 21 R2's shared prefix): the manifest first (the three keys,
// each with its validation), the dependency face, then the root module
// src/main.we and its import graph — every dependency module read and
// parsed in depth-first source order (E1302/E1301 are the loader's
// faces), then all modules type-checked in the graph's post-order (the
// imported before the importing — chapter 15's deterministic
// initialization order; the main convention binds the root module).
// It returns the root module, the manifest name, and the module count; a
// nil file means the failure is already reported, with its exit code.
//
// artifact marks a caller with an artifact face (build/run): a library
// manifest stops at the library boundary after the manifest validations
// and before any source work (design D2). check passes false and keeps
// its check-through behavior. The dependency boundary applies to every
// pipeline command alike (chapter 22 R5) and fires first when both are
// present.
func (e *env) loadProject(dir string, artifact bool) (*ast.File, string, int, int) {
	raw, err := os.ReadFile(filepath.Join(dir, "we.toml"))
	if err != nil {
		e.report(diag.Error("E1905", "project manifest missing or incomplete — no we.toml in the project directory; create we.toml with the name, version, and type keys, or run we new to write the skeleton").
			At("we.toml", 1, 1).
			WithHelp("Create we.toml with the name, version, and type keys, or run we new to write the skeleton."))
		return nil, "", 0, exitDiagnostic
	}
	manifest, deps := parseManifest(string(raw))
	for _, key := range []string{"name", "version", "type"} {
		if manifest[key] == "" {
			e.report(diag.Error("E1905", fmt.Sprintf(
				"project manifest missing or incomplete — the manifest's %q key is missing; create we.toml with the name, version, and type keys, or run we new to write the skeleton", key)).
				At("we.toml", 1, 1).
				WithHelp("Create we.toml with the name, version, and type keys, or run we new to write the skeleton."))
			return nil, "", 0, exitDiagnostic
		}
	}
	if !validProjectName(manifest["name"]) {
		e.report(diag.Error("E1904", fmt.Sprintf(
			"invalid project name — %q is not lowercase letters, digits, and hyphens; use chapter 1's naming convention", manifest["name"])).
			At("we.toml", 1, 1).
			WithHelp("Use lowercase letters, digits, and hyphens per chapter 1's naming convention."))
		return nil, "", 0, exitDiagnostic
	}
	if !validVersion(manifest["version"]) {
		e.report(diag.Error("E2004", fmt.Sprintf(
			"invalid version value — %q is not three dot-separated non-negative integers without leading zeros; write the version as major.minor.patch", manifest["version"])).
			At("we.toml", 1, 1).
			WithHelp("Write the version as three integers, major.minor.patch, without leading zeros."))
		return nil, "", 0, exitDiagnostic
	}
	if manifest["type"] != "executable" && manifest["type"] != "library" {
		e.report(diag.Error("E1903", fmt.Sprintf(
			"invalid toolchain configuration value — the \"type\" key holds %q; its legal values are executable and library", manifest["type"])).
			At("we.toml", 1, 1).
			WithHelp("Set the named key to one of the legal values the diagnostic lists."))
		return nil, "", 0, exitDiagnostic
	}
	// Chapter 22 R5: acquisition precedes module resolution, so a non-empty
	// [dependencies] set stops every pipeline command here — an empty or
	// absent section is trivially satisfied.
	if deps > 0 {
		return nil, "", 0, e.boundary(whatNonEmptyDeps)
	}
	// The artifact kind is the manifest's word: build/run stop before any
	// source work for a library project (design D2's pinned order).
	if artifact && manifest["type"] == "library" {
		return nil, "", 0, e.boundary(whatLibraryArtifacts)
	}
	root := filepath.Join(dir, "src", "main.we")
	src, err := os.ReadFile(root)
	if err != nil {
		e.report(diag.Error("E1305", "main function signature violation — the root module "+root+" does not exist; declare exactly one pub fn main() -> Result<(), E> in src/main.we with E a named sum type").
			At(root, 1, 1).
			WithHelp("Declare exactly one pub fn main() -> Result<(), E> in src/main.we with E a named sum type."))
		return nil, "", 0, exitDiagnostic
	}
	file, d, ni := parser.Parse(root, src)
	if d != nil {
		e.report(*d)
		return nil, "", 0, exitDiagnostic
	}
	if ni != nil {
		return nil, "", 0, e.boundary(ni.What)
	}
	// The module graph from the root (chapter 15): depth-first over the
	// imports in source order, then the post-order the type stage takes.
	mods, code := e.loadGraph(dir, root, file)
	if code != exitOK {
		return nil, "", 0, code
	}
	td, tni := typecheck.CheckProject(file, root, mods)
	if td != nil {
		e.report(*td)
		return nil, "", 0, exitDiagnostic
	}
	if tni != nil {
		return nil, "", 0, e.boundary(tni.What)
	}
	return file, manifest["name"], len(mods) + 1, exitOK
}

// The loader's diagnostic helps — the registry's remediations, quoted per
// the type stage's tHelps embed.
const (
	helpE1301 = "Extract the code both modules need into a third module and import it from each."
	helpE1302 = "Create the file at the expected path, fix the path spelling, or add the dependency to the cache."
)

// loadGraph walks the project's import graph depth-first from the root
// (design D9): every import resolves by the path mapping (a.b.c to
// src/a/b/c.we) or, for the std segment, from the compiler-provided
// registry (never the file system — chapter 15 R1); a missing file or
// unknown std path is E1302 (the message names the expected path, or the
// std form when nothing maps), a back edge is E1301 (three-color marking;
// the message renders the cycle), and each first-visited module is read
// and parsed in discovery order.
// It returns the dependency modules in post-order — the imported before
// the importing, chapter 15's deterministic initialization order — with
// the root excluded (the caller checks it last); a non-zero code means
// the failure is already reported.
func (e *env) loadGraph(dir, rootPath string, root *ast.File) ([]typecheck.Module, int) {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	var order []typecheck.Module
	color := map[string]int{}
	var stack []string
	var visit func(key, path string, file *ast.File) int
	visit = func(key, path string, file *ast.File) int {
		color[key] = gray
		stack = append(stack, key)
		defer func() {
			stack = stack[:len(stack)-1]
			color[key] = black
		}()
		for _, it := range file.Items {
			imp, ok := it.(*ast.Import)
			if !ok {
				continue
			}
			// The std segment resolves from the compiler-provided
			// registry, never the file system (chapter 15 R1): the
			// module rides the graph like any dependency — provided
			// before the importing — and an unknown path is E1302's
			// std form (no expected-path clause: nothing maps it).
			if imp.Path[0] == "std" {
				key := strings.Join(imp.Path, ".")
				if color[key] == black {
					continue
				}
				stdFile, ok := typecheck.StdModule(key)
				if !ok {
					e.report(diag.Error("E1302", typecheck.StdModuleNotFound(key)).
						At(path, imp.PathLine, imp.PathCol).
						WithHelp(helpE1302))
					return exitDiagnostic
				}
				if code := visit(key, key, stdFile); code != exitOK {
					return code
				}
				continue
			}
			depKey := strings.Join(imp.Path, ".")
			if color[depKey] == gray {
				cycle := depKey
				for i := len(stack) - 1; i >= 0; i-- {
					cycle = stack[i] + " -> " + cycle
					if stack[i] == depKey {
						break
					}
				}
				e.report(diag.Error("E1301", fmt.Sprintf(
					"circular module dependency — the import closes the cycle %s; there are no forward declarations, and the fix is extracting the shared code into a third module both import",
					cycle)).
					At(path, imp.PathLine, imp.PathCol).
					WithHelp(helpE1301))
				return exitDiagnostic
			}
			if color[depKey] == black {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(imp.Path...) + ".we")
			depPath := filepath.Join(dir, "src", rel)
			if _, err := os.Stat(depPath); err != nil {
				e.report(diag.Error("E1302", fmt.Sprintf(
					"module not found — the import %q expects the module at src/%s and no file is there; create the file at the expected path, fix the path spelling, or add the dependency to the cache",
					depKey, rel)).
					At(path, imp.PathLine, imp.PathCol).
					WithHelp(helpE1302))
				return exitDiagnostic
			}
			src, err := os.ReadFile(depPath)
			if err != nil {
				return e.fsError(err)
			}
			depFile, d, ni := parser.Parse(depPath, src)
			if d != nil {
				e.report(*d)
				return exitDiagnostic
			}
			if ni != nil {
				return e.boundary(ni.What)
			}
			if code := visit(depKey, depPath, depFile); code != exitOK {
				return code
			}
		}
		if key != "main" {
			order = append(order, typecheck.Module{Key: key, Path: path, File: file})
		}
		return exitOK
	}
	return order, visit("main", rootPath, root)
}

// parseManifest reads the manifest's flat string keys — the minimal
// subset this milestone's validations need: `key = "value"` lines, with
// blank and # lines skipped. It also counts the keys inside a
// [dependencies] section (chapter 22 R5's face: any key there is a
// non-empty dependency set). Richer TOML shapes arrive with the project
// chapter's own milestone.
func parseManifest(raw string) (map[string]string, int) {
	m := map[string]string{}
	deps := 0
	section := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if i := strings.Index(line, "="); i >= 0 {
			key := strings.TrimSpace(line[:i])
			val := strings.Trim(strings.TrimSpace(line[i+1:]), `"`)
			m[key] = val
			if section == "dependencies" {
				deps++
			}
		}
	}
	return m, deps
}

// validVersion reports whether v is three dot-separated non-negative
// integers without leading zeros (registry entry E2004).
func validVersion(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return false
		}
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return false
			}
		}
	}
	return true
}

// boundary prints one not-implemented boundary line (stderr in both
// faces — no diagnostic event, chapter 21's exit 70).
func (e *env) boundary(what string) int {
	fmt.Fprintf(e.stderr, "we: %s are not implemented in this reference build yet\n", what)
	return exitNotImplemented
}

// checkPassed renders the check's success faces: silent exit 0, zero
// events under --json, one line under --verbose.
func (e *env) checkPassed(modules int) int {
	if e.verbose {
		noun := "modules"
		if modules == 1 {
			noun = "module"
		}
		fmt.Fprintf(e.stdout, "we: check passed (%d %s)\n", modules, noun)
	}
	return exitOK
}
