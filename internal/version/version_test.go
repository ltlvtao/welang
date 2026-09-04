package version

import (
	"strings"
	"testing"
)

// TestManifestConstants checks the version manifest's invariants: every
// constant non-empty, and CompilerVersion a semantic version under chapter
// 22's shape — three components, ASCII digits only, no leading zeros.
func TestManifestConstants(t *testing.T) {
	if CompilerVersion == "" || SpecVersion == "" || LLVMPin == "" {
		t.Fatalf("version manifest constants must be non-empty: %q %q %q",
			CompilerVersion, SpecVersion, LLVMPin)
	}
	parts := strings.Split(CompilerVersion, ".")
	if len(parts) != 3 {
		t.Fatalf("CompilerVersion %q: semantic versions have exactly three components", CompilerVersion)
	}
	for _, p := range parts {
		if p == "" {
			t.Fatalf("CompilerVersion %q: empty component", CompilerVersion)
		}
		if len(p) > 1 && p[0] == '0' {
			t.Fatalf("CompilerVersion %q: component %q has a leading zero", CompilerVersion, p)
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				t.Fatalf("CompilerVersion %q: component %q has a non-digit %q", CompilerVersion, p, c)
			}
		}
	}
}
