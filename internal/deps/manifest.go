package deps

import (
	"sort"
	"strings"
)

// Table is one manifest's [dependencies]: package name to constraint.
type Table map[string]Constraint

// DeclError names one rejected entry — E2006 for a key that is not a
// package name, E2003 for a value outside the four constraint forms. The
// caller renders the diagnostic.
type DeclError struct {
	Code string
	Key  string
	Val  string
}

// ReadDeps lifts and validates the dependencies.* keys out of a parsed
// manifest. Keys must be legal package names under chapter 1's convention
// — lowercase letters, digits, and hyphens — and never the reserved std;
// values must parse as one of the four constraint forms. A bare unquoted
// value needs no quote-sensitivity of its own: a bare `1.2.3` fails the
// constraint grammar naturally. The first fault reports, in dependency
// name order, so the face is one diagnostic, deterministic.
func ReadDeps(manifest map[string]string) (Table, *DeclError) {
	var names []string
	for k := range manifest {
		if strings.HasPrefix(k, "dependencies.") {
			names = append(names, strings.TrimPrefix(k, "dependencies."))
		}
	}
	sort.Strings(names)
	table := Table{}
	for _, name := range names {
		if !validDepName(name) {
			return nil, &DeclError{Code: "E2006", Key: name}
		}
		val := manifest["dependencies."+name]
		con, ok := ParseConstraint(val)
		if !ok {
			return nil, &DeclError{Code: "E2003", Key: name, Val: val}
		}
		table[name] = con
	}
	return table, nil
}

// validDepName is the package-name predicate: non-empty, lowercase
// letters, digits, and hyphens — the same character set as project names
// — with std reserved for the built-in standard library.
func validDepName(name string) bool {
	if name == "" || name == "std" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}
