package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	src, err := os.ReadFile(path)
	if err != nil {
		return e.fsError(err)
	}
	file, d, ni := parser.Parse(path, src)
	if d != nil {
		e.report(*d)
		return exitDiagnostic
	}
	if ni != nil {
		return e.boundary(ni.What)
	}
	td, tni := typecheck.Check(file, path, typecheck.SingleFile)
	if td != nil {
		e.report(*td)
		return exitDiagnostic
	}
	if tni != nil {
		return e.boundary(tni.What)
	}
	return e.checkPassed(1)
}

// runCheckProject checks a project directory: the manifest first (the
// three keys, each with its validation), then the root module
// src/main.we through the type stage in project mode (the main
// convention binds the root module).
func (e *env) runCheckProject(dir string) int {
	raw, err := os.ReadFile(filepath.Join(dir, "we.toml"))
	if err != nil {
		e.report(diag.Error("E1905", "project manifest missing or incomplete — no we.toml in the project directory; create we.toml with the name, version, and type keys, or run we new to write the skeleton").
			At("we.toml", 1, 1).
			WithHelp("Create we.toml with the name, version, and type keys, or run we new to write the skeleton."))
		return exitDiagnostic
	}
	manifest := parseManifest(string(raw))
	for _, key := range []string{"name", "version", "type"} {
		if manifest[key] == "" {
			e.report(diag.Error("E1905", fmt.Sprintf(
				"project manifest missing or incomplete — the manifest's %q key is missing; create we.toml with the name, version, and type keys, or run we new to write the skeleton", key)).
				At("we.toml", 1, 1).
				WithHelp("Create we.toml with the name, version, and type keys, or run we new to write the skeleton."))
			return exitDiagnostic
		}
	}
	if !validProjectName(manifest["name"]) {
		e.report(diag.Error("E1904", fmt.Sprintf(
			"invalid project name — %q is not lowercase letters, digits, and hyphens; use chapter 1's naming convention", manifest["name"])).
			At("we.toml", 1, 1).
			WithHelp("Use lowercase letters, digits, and hyphens per chapter 1's naming convention."))
		return exitDiagnostic
	}
	if !validVersion(manifest["version"]) {
		e.report(diag.Error("E2004", fmt.Sprintf(
			"invalid version value — %q is not three dot-separated non-negative integers without leading zeros; write the version as major.minor.patch", manifest["version"])).
			At("we.toml", 1, 1).
			WithHelp("Write the version as three integers, major.minor.patch, without leading zeros."))
		return exitDiagnostic
	}
	if manifest["type"] != "executable" && manifest["type"] != "library" {
		e.report(diag.Error("E1903", fmt.Sprintf(
			"invalid toolchain configuration value — the \"type\" key holds %q; its legal values are executable and library", manifest["type"])).
			At("we.toml", 1, 1).
			WithHelp("Set the named key to one of the legal values the diagnostic lists."))
		return exitDiagnostic
	}
	root := filepath.Join(dir, "src", "main.we")
	src, err := os.ReadFile(root)
	if err != nil {
		e.report(diag.Error("E1305", "main function signature violation — the root module "+root+" does not exist; declare exactly one pub fn main() -> Result<(), E> in src/main.we with E a named sum type").
			At(root, 1, 1).
			WithHelp("Declare exactly one pub fn main() -> Result<(), E> in src/main.we with E a named sum type."))
		return exitDiagnostic
	}
	file, d, ni := parser.Parse(root, src)
	if d != nil {
		e.report(*d)
		return exitDiagnostic
	}
	if ni != nil {
		return e.boundary(ni.What)
	}
	td, tni := typecheck.Check(file, root, typecheck.Project)
	if td != nil {
		e.report(*td)
		return exitDiagnostic
	}
	if tni != nil {
		return e.boundary(tni.What)
	}
	return e.checkPassed(1)
}

// parseManifest reads the manifest's flat string keys — the minimal
// subset this milestone's validations need: `key = "value"` lines, with
// blank and # lines skipped. Richer TOML shapes arrive with the project
// chapter's own milestone.
func parseManifest(raw string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "="); i >= 0 {
			key := strings.TrimSpace(line[:i])
			val := strings.Trim(strings.TrimSpace(line[i+1:]), `"`)
			m[key] = val
		}
	}
	return m
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
