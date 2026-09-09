package deps

import "strings"

// Constraint is one dependency requirement in exactly one of the four
// forms (chapter 22 R2): ^, ~, >=, or = over a three-part version.
type Constraint struct {
	Op string
	V  Version
}

// ParseConstraint reads an operator prefix followed by a well-formed
// version. Everything else — a bare version, an unknown operator, a short
// or zero-padded operand — is outside the grammar (E2003's face).
func ParseConstraint(s string) (Constraint, bool) {
	for _, op := range []string{"^", "~", ">=", "="} {
		if strings.HasPrefix(s, op) {
			v, ok := ParseVersion(s[len(op):])
			if !ok {
				return Constraint{}, false
			}
			return Constraint{Op: op, V: v}, true
		}
	}
	return Constraint{}, false
}

// Floor is the least version the constraint admits.
func (c Constraint) Floor() Version {
	return c.V
}

// Ceiling is the exclusive upper bound, when the form imposes one: ^ stops
// below the next major, ~ below the next minor, = at the pick itself; >=
// has no top. There is no cargo-style zero special case — ^0.1.0 stops
// below 0.2.0 like any other minor.
func (c Constraint) Ceiling() (Version, bool) {
	switch c.Op {
	case "^":
		return Version{Major: c.V.Major + 1}, true
	case "~":
		return Version{Major: c.V.Major, Minor: c.V.Minor + 1}, true
	case "=":
		return c.V, true
	}
	return Version{}, false
}

// Allows reports whether v satisfies the constraint: at or above the
// floor, below any exclusive ceiling, and — for the exact form — the pick
// itself (its ceiling names the pick, so equality is the rule, not an
// open range).
func (c Constraint) Allows(v Version) bool {
	if c.Op == "=" {
		return Compare(v, c.V) == 0
	}
	if Compare(v, c.Floor()) < 0 {
		return false
	}
	if ceil, ok := c.Ceiling(); ok && Compare(v, ceil) >= 0 {
		return false
	}
	return true
}

// String renders the constraint back to operator + version.
func (c Constraint) String() string {
	return c.Op + c.V.String()
}
