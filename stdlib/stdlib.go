// Package stdlib holds the standard library's real We sources, embedded
// into the compiler at build time (B2a design D1): the library is genuine
// .we files — the same parser and the same checker a user module rides,
// no longer a Go-synthesized AST registry. The go:embed mirrors
// runtime/runtime.go's c/*.c: the sources must live inside this package's
// tree, and the loader (typecheck.StdModule) reads them through Sources().
// This package imports nothing of the compiler; the dependency runs one
// way, typecheck -> stdlib, so no cycle can form.
package stdlib

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed src/*.we
var srcFS embed.FS

// sources maps a bare std segment ("io", "test", ...) to its source text.
// Built once at init: the embedded set is fixed for the process lifetime.
var sources = func() map[string]string {
	paths, err := fs.Glob(srcFS, "src/*.we")
	if err != nil {
		panic("stdlib: glob embedded sources: " + err.Error())
	}
	sort.Strings(paths)
	m := make(map[string]string, len(paths))
	for _, p := range paths {
		b, err := srcFS.ReadFile(p)
		if err != nil {
			panic("stdlib: read embedded source " + p + ": " + err.Error())
		}
		m[strings.TrimSuffix(strings.TrimPrefix(p, "src/"), ".we")] = string(b)
	}
	return m
}()

// Sources returns the embedded standard-library sources keyed by their
// bare std segment ("io", "test", ...). The map is the loader's single
// authority for which std modules exist as real sources; callers must
// not mutate it.
func Sources() map[string]string { return sources }
