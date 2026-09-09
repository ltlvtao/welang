package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ltlvtao/welang/internal/deps"
	"github.com/ltlvtao/welang/internal/diag"
)

// The dependency face's wiring (chapter 22 R3–R5, design D6): the loader's
// orchestrator — lock first, resolution and acquisition when the lock
// cannot witness the graph — and the loader's cache leg. fmt and clean
// never call in: fmt reads only the manifest's declared form (loadManifest
// validates it for every reader alike), clean reads no manifest at all.

// Dependency-face helps — the registry's remediations, quoted verbatim.
const (
	helpE2001 = "Check the spelling of the dependency name, or declare a package that exists."
	helpE2002 = "Loosen or align the conflicting constraint - typically raise or drop an exact pin - so the graph's floors and ceilings admit one common version."
	helpE2005 = "Reacquire the package (clear the corrupted cache entry) or regenerate the lock if the dependency was legitimately replaced."
)

// prepareDeps runs the dependency face for every pipeline command (R5): a
// we.lock that satisfies the graph against a warm cache is the build's
// truth — no source is touched, the offline read; otherwise resolution
// picks, acquisition copies each pick into the cache, and the lock
// rewrites. The displaced lock still testifies on regeneration: a package
// resolved to the same version it recorded must hash to the same digest —
// E2005 fires before the rewrite, so the lock keeps its old bytes. It
// returns the acquired roots (package name -> cache directory) the
// loader's cache leg resolves against; nil with exitOK means the manifest
// declared no dependencies.
func (e *env) prepareDeps(dir string, table deps.Table) (map[string]string, int) {
	if len(table) == 0 {
		return nil, exitOK
	}
	cacheRoot, err := deps.CacheRoot()
	if err != nil {
		return nil, e.fsError(err)
	}
	var displaced []deps.LockEntry // the lock a regeneration replaces
	if entries, ok := deps.ReadLock(filepath.Join(dir, "we.lock")); ok {
		state, roots, flaw := deps.LockStatus(table, entries, cacheRoot)
		switch state {
		case deps.LockWins:
			return roots, exitOK
		case deps.LockTampered:
			return nil, e.e2005(flaw.Name, flaw.Version, flaw.Got, flaw.Want)
		}
		displaced = entries // LockStale: the lock cannot witness the graph
	}
	regRoot := deps.RegistryRoot()
	picks, conflict := deps.Resolve(table, deps.NewDirRegistry(regRoot))
	if conflict != nil {
		return nil, e.reportConflict(conflict)
	}
	names := make([]string, 0, len(picks))
	for n := range picks {
		names = append(names, n)
	}
	sort.Strings(names)
	entries := make([]deps.LockEntry, 0, len(names))
	roots := map[string]string{}
	for _, n := range names {
		v := picks[n]
		if err := deps.Acquire(regRoot, cacheRoot, n, v); err != nil {
			return nil, e.fsError(err)
		}
		pkgDir := deps.PackageDir(cacheRoot, n, v)
		digest, err := deps.Digest(pkgDir)
		if err != nil {
			return nil, e.fsError(err)
		}
		entries = append(entries, deps.LockEntry{Name: n, Version: v, Digest: digest})
		roots[n] = pkgDir
	}
	if len(displaced) > 0 {
		prev := map[string]deps.LockEntry{}
		for _, old := range displaced {
			prev[old.Name] = old
		}
		for _, entry := range entries {
			if old, ok := prev[entry.Name]; ok && old.Version == entry.Version && old.Digest != entry.Digest {
				return nil, e.e2005(entry.Name, entry.Version, entry.Digest, old.Digest)
			}
		}
	}
	if err := deps.WriteLock(filepath.Join(dir, "we.lock"), entries); err != nil {
		return nil, e.fsError(err)
	}
	return roots, exitOK
}

// e2005 reports the integrity mismatch at the lock's anchor: the package,
// its version, what the content hashes to, and what the lock recorded.
func (e *env) e2005(name string, v deps.Version, got, want string) int {
	e.report(diag.Error("E2005", fmt.Sprintf(
		"lockfile integrity mismatch — cached %q %s hashes to %s, we.lock records %s",
		name, v, got, want)).
		At("we.lock", 1, 1).
		WithHelp(helpE2005))
	return exitDiagnostic
}

// reportConflict renders resolution's failure at the manifest's anchor:
// E2001 names the package no source answers; E2002 renders in two forms —
// a violated ceiling carries the violated constraint, its dependent chain
// to root, and the raising constraint with its author, while a demanded
// version no source holds carries the demand and its author (design D3).
func (e *env) reportConflict(c *deps.Conflict) int {
	if c.Code == "E2001" {
		e.report(diag.Error("E2001", fmt.Sprintf(
			"unknown dependency — %q answers to no package in any source", c.Name)).
			At("we.toml", 1, 1).
			WithHelp(helpE2001))
		return exitDiagnostic
	}
	msg := fmt.Sprintf(
		"unsatisfiable dependency constraint — package %q resolves to %s, which no source holds (demanded by %q from %s)",
		c.Name, c.Pick, c.Demand, c.Dep)
	if c.Con.Op != "" {
		msg = fmt.Sprintf(
			"unsatisfiable dependency constraint — package %q resolves to %s, violating %q required along %s; the pick was raised by %q from %s",
			c.Name, c.Pick, c.Con, c.Chain, c.Raised, c.Raiser)
	}
	e.report(diag.Error("E2002", msg).
		At("we.toml", 1, 1).
		WithHelp(helpE2002))
	return exitDiagnostic
}

// depModulePath maps an import path onto a cached dependency's module
// file: the first segment names the package (an acquired root from
// prepareDeps), the rest the module under its src/ — some.lib.util
// answers <root>/src/lib/util.we. A std segment or a name the project did
// not declare answers false: those legs belong to the std registry and
// the local src/ tree.
func depModulePath(dirs map[string]string, imp string) (string, bool) {
	first, rest, found := strings.Cut(imp, ".")
	if !found {
		return "", false // a package name alone names no module
	}
	root, ok := dirs[first]
	if !ok {
		return "", false
	}
	return filepath.Join(root, "src", strings.ReplaceAll(rest, ".", "/")+".we"), true
}
