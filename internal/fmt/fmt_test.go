// Package fmt is the line-structure-preserving token reformatter of M11
// design D1–D3: lex.Keep keeps comments as trivia, the parser classifies
// the `|` ambiguity, and the reformatter re-spaces tokens without ever
// moving one across a line (imports reorder as whole declarations, the
// one spec-named exception).
package fmt

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/diag"
)

// oneCase is a dirty source and its hand-pinned clean output; every case
// also asserts the fixed point Format(Format(x)) == Format(x) and the
// line-count invariant of chapter 2's newline-carries-meaning discipline.
type oneCase struct {
	name  string
	dirty string
	clean string
}

var cases = []oneCase{
	{"spacing", "pub fn add(a:Int64,b:Int64)->Int64{\n    return a+b\n}\n",
		"pub fn add(a: Int64, b: Int64) -> Int64 {\n  return a + b\n}\n"},
	{"blank collapse", "\n\npub fn main() -> Int64 {\n\n\n    return 0\n}\n",
		"pub fn main() -> Int64 {\n\n  return 0\n}\n"},
	{"empty block", "pub fn noop() { }\n", "pub fn noop() {}\n"},
	{"closure pipe", "let n = m.update(|v|v+1)\nlet flag = match n {\n    1|2=>true\n    _=>false\n}\n",
		"let n = m.update(|v| v + 1)\nlet flag = match n {\n  1 | 2 => true\n  _ => false\n}\n"},
	{"import groups", "import util\nimport std.io\n\npub fn main() {\n    return\n}\n",
		"import std.io\n\nimport util\n\npub fn main() {\n  return\n}\n"},
	{"comment attach", "import util\n\n// note\nimport std.io\n\npub fn main() {\n    return\n}\n",
		"// note\nimport std.io\n\nimport util\n\npub fn main() {\n  return\n}\n"},
	{"tabs crlf trailing", "\tpub fn main() -> Int64 {\r\n    return 0   \r\n}\r\n",
		"pub fn main() -> Int64 {\n  return 0\n}\n"},
	{"string verbatim", "let s = \"a  b:c,d\"\n", "let s = \"a  b:c,d\"\n"},
}

func TestFormatCases(t *testing.T) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, ds := Format("f.we", []byte(tc.dirty))
			if len(ds) != 0 {
				t.Fatalf("diagnostics: %v", ds[0].Human())
			}
			if string(out) != tc.clean {
				t.Fatalf("output:\nwant %q\ngot  %q", tc.clean, string(out))
			}
		})
	}
}

// The fixed point: formatting twice changes nothing the second time.
func TestFormatFixpoint(t *testing.T) {
	for _, tc := range cases {
		once, ds := Format("f.we", []byte(tc.dirty))
		if len(ds) != 0 {
			t.Fatalf("%s: diagnostics: %s", tc.name, ds[0].Human())
		}
		twice, ds := Format("f.we", once)
		if len(ds) != 0 {
			t.Fatalf("%s: second pass diagnostics: %s", tc.name, ds[0].Human())
		}
		if string(once) != string(twice) {
			t.Fatalf("%s: not a fixed point:\nfirst  %q\nsecond %q", tc.name, once, twice)
		}
	}
}

// Chapter 2's derived invariant: the formatter never moves a token across
// a line, so the count of token-bearing (non-blank) lines is preserved —
// imports permute whole declarations, and blank lines are spacing policy
// (D3), not tokens, so only they may be inserted or collapsed.
func TestFormatPreservesTokenLineCount(t *testing.T) {
	count := func(s string) int {
		n := 0
		for _, ln := range strings.Split(s, "\n") {
			if strings.TrimSpace(ln) != "" {
				n++
			}
		}
		return n
	}
	for _, tc := range cases {
		out, ds := Format("f.we", []byte(tc.dirty))
		if len(ds) != 0 {
			t.Fatalf("%s: diagnostics: %s", tc.name, ds[0].Human())
		}
		if in, got := count(tc.dirty), count(string(out)); in != got {
			t.Fatalf("%s: token line count: want %d, got %d", tc.name, in, got)
		}
	}
}

// A parse failure is the formatter's only gate (design D2): the source is
// returned unchanged alongside the parser's diagnostic, code and position
// intact.
func TestFormatParseError(t *testing.T) {
	src := []byte("pub fn (")
	out, ds := Format("f.we", src)
	if len(ds) == 0 {
		t.Fatalf("want diagnostics, got none")
	}
	if ds[0].Code() != "E0105" {
		t.Fatalf("code: want E0105, got %s", ds[0].Code())
	}
	if !strings.Contains(ds[0].JSON(), `"line":1,"column":8`) {
		t.Fatalf("position: %s", ds[0].JSON())
	}
	if string(out) != string(src) {
		t.Fatalf("source must be unchanged, got %q", string(out))
	}
	var _ = diag.Error // import honesty
}

// An empty file stays empty — no trailing newline is invented for zero
// content (design D3's edge).
func TestFormatEmptyStaysEmpty(t *testing.T) {
	out, ds := Format("f.we", []byte(""))
	if len(ds) != 0 {
		t.Fatalf("diagnostics: %s", ds[0].Human())
	}
	if len(out) != 0 {
		t.Fatalf("want empty output, got %q", string(out))
	}
}

// Comments ride the reformat verbatim: the text inside a line comment is
// never re-spaced, only its position within the line is.
func TestFormatCommentVerbatim(t *testing.T) {
	src := "pub fn main() {\n    let x = 1   //  keep   these\n    return\n}\n"
	want := "pub fn main() {\n  let x = 1 //  keep   these\n  return\n}\n"
	out, ds := Format("f.we", []byte(src))
	if len(ds) != 0 {
		t.Fatalf("diagnostics: %s", ds[0].Human())
	}
	if string(out) != want {
		t.Fatalf("output:\nwant %q\ngot  %q", want, string(out))
	}
}
