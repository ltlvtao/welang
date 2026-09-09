// Package deps implements chapter 22's dependency machinery: the version
// and constraint grammars, the manifest's dependency table, maximal-version
// resolution, the lockfile, and cache acquisition. Everything here is pure
// or filesystem-shaped — the CLI renders the diagnostics.
package deps

import "strings"

// Version is a semantic version's three numeric components (chapter 22 R2).
// Prerelease and build tags are deliberately absent: the spec's grammar is
// exactly three dot-separated integers.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion reads three dot-separated non-negative integers without
// leading zeros — the same predicate the manifest's own version key
// validates (E2004).
func ParseVersion(s string) (Version, bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var v Version
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return Version{}, false
		}
		n := 0
		for j := 0; j < len(p); j++ {
			if p[j] < '0' || p[j] > '9' {
				return Version{}, false
			}
			n = n*10 + int(p[j]-'0')
		}
		switch i {
		case 0:
			v.Major = n
		case 1:
			v.Minor = n
		case 2:
			v.Patch = n
		}
	}
	return v, true
}

// Compare orders versions fully numerically, component by component.
func Compare(a, b Version) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// String renders the version back to major.minor.patch.
func (v Version) String() string {
	return itoa(v.Major) + "." + itoa(v.Minor) + "." + itoa(v.Patch)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
