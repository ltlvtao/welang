package deps

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Registry answers versions and manifests from a source universe. The
// directory tree is the reference shape (design D2): WE_REGISTRY names a
// root holding <name>/<version>/{we.toml, src/...}.
type Registry interface {
	// Versions lists a package's versions ascending; an empty answer
	// means no source holds the package at all.
	Versions(name string) []Version
	// Deps reads a package's [dependencies] at one version; false means
	// no manifest answers.
	Deps(name string, v Version) (Table, bool)
}

// DirRegistry is the local-directory source: version enumeration is a
// directory listing (non-version directory names are not versions), and a
// package's manifest is read straight from its tree. An absent root is an
// empty universe, not an error.
type DirRegistry struct {
	root string
}

// NewDirRegistry answers from root, if it is there.
func NewDirRegistry(root string) Registry {
	return DirRegistry{root: root}
}

// Versions implements Registry.
func (r DirRegistry) Versions(name string) []Version {
	entries, err := os.ReadDir(filepath.Join(r.root, name))
	if err != nil {
		return nil
	}
	var vs []Version
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if v, ok := ParseVersion(e.Name()); ok {
			vs = append(vs, v)
		}
	}
	sort.Slice(vs, func(i, j int) bool { return Compare(vs[i], vs[j]) < 0 })
	return vs
}

// Deps implements Registry. Only the [dependencies] table is read — a
// registry package is acquired goods, not a user face, so its own name,
// version, and type keys are not validated here (design D2).
func (r DirRegistry) Deps(name string, v Version) (Table, bool) {
	raw, err := os.ReadFile(filepath.Join(r.root, name, v.String(), "we.toml"))
	if err != nil {
		return nil, false
	}
	table, fault := ReadDeps(parseDepsSection(string(raw)))
	if fault != nil {
		return nil, false
	}
	return table, true
}

// parseDepsSection reads just the [dependencies] section of a package
// manifest, in the flat `key = "value"` shape the project manifest reader
// pins, surfacing entries under their dependencies. prefix so ReadDeps
// consumes them directly.
func parseDepsSection(raw string) map[string]string {
	m := map[string]string{}
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
		if section != "dependencies" {
			continue
		}
		if i := strings.Index(line, "="); i >= 0 {
			key := strings.TrimSpace(line[:i])
			val := strings.TrimSpace(line[i+1:])
			if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
				val = val[1 : len(val)-1]
			}
			m["dependencies."+key] = val
		}
	}
	return m
}
