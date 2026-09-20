package wec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ltlvtao/welang/internal/cli"
	"github.com/ltlvtao/welang/internal/lex"
)

// This file is the B3b reconciliation tower (the wec-lexer change, design
// D4/D5): it builds the wec project — a We program, the bootstrap
// compiler's lexical seed at the repository top level — with the
// Go-hosted toolchain in-process, then drives the artifact over a corpus
// and requires its serialized lexical output to be byte-identical to what
// internal/lex computes live. Hand-written lexer expectations do not exist
// here by design: the reference side is computed on every run, so port
// drift has nowhere to hide.

// esc is the serialization spec's escaping (design D4): backslash doubles,
// the printable ASCII range 0x21-0x7E passes verbatim, and every other
// byte — space, the controls, 0x7F, and anything non-ASCII — renders as a
// backslash plus two uppercase hex digits. The wec side implements the
// same table; the format pin below catches a rewrite on either side.
func esc(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			b.WriteString("\\\\")
		case c >= 0x21 && c <= 0x7E:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "\\%02X", c)
		}
	}
	return b.String()
}

// serialize renders internal/lex's scan of one source under the shared
// serialization spec (design D4): a "tokens" header and one T line per
// token — attr tokens carry their name and raw argument text as two more
// fields — then an optional docs section, or a single E line replacing
// everything when the scan stops at the first lexical error (chapter 1's
// first-error-stop). The diagnostic's file name is not serialized (the
// driver contract fixes it) and neither is the registry help (a static
// property, not lexer behavior).
func serialize(src []byte) string {
	toks, docs, d := lex.Scan("wec-input.we", src)
	if d != nil {
		return fmt.Sprintf("E %d %d %s %s\n", d.Line(), d.Column(), d.Code(), esc(d.Message()))
	}
	var b strings.Builder
	b.WriteString("tokens\n")
	for _, tk := range toks {
		line := fmt.Sprintf("T %d %d %s %s", tk.Line, tk.Col, tk.Kind, esc(tk.Text))
		if tk.Kind == "attr" {
			line += " " + esc(tk.AttrName) + " " + esc(tk.AttrArgs)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(docs) > 0 {
		b.WriteString("docs\n")
		for _, du := range docs {
			fmt.Fprintf(&b, "D %d %d\n", du.StartLine, du.EndLine)
			for _, ln := range du.Lines {
				b.WriteString("L ")
				b.WriteString(esc(ln))
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// unnameable reports whether byte value b is one of the thirteen values a
// We string literal cannot construct (C0, C1, F5-FF — design D2-3's
// observational-invariance corner): the reference side's E0001 message
// carries the byte's value, and the wec side has no way to name it. Such
// entries are excluded with a logged disclosure, never silently.
func unnameable(b byte) bool {
	return b == 0xC0 || b == 0xC1 || b >= 0xF5
}

// firstBadByte walks src to its first ill-formed byte (the reference
// lexer's own walk) and reports whether that byte's value is nameable in
// We. Entries whose first bad byte is not nameable stay out until the
// byte-exposure face lands (follow-up #30).
func firstBadByteNameable(src string) bool {
	for off := 0; off < len(src); {
		r, size := utf8.DecodeRuneInString(src[off:])
		if r == utf8.RuneError && size == 1 {
			return !unnameable(src[off])
		}
		off += size
	}
	return true
}

// entry is one corpus source under its corpus-qualified name.
type entry struct {
	name string
	src  string
}

// loadCorpus gathers the reconciliation corpus (design D7): the stdlib's
// own sources, the wec project's own sources (the bootstrap's earliest
// self-consistency signal), every conformance case's setup sources
// (read-only), the Go lex package's own test literals, and the in-tower
// literals. The E0001-E0009 error matrix grows with its tasks (T4); T2
// adds the encoding faces' shapes (the five nameable invalid-UTF-8 forms,
// the BOM pair, the bare CRs, the unterminated block comment).
func loadCorpus(t *testing.T) []entry {
	t.Helper()
	var cs []entry
	cs = append(cs,
		entry{"literal:empty", ""},
		entry{"literal:let-one", "let x = 1\n"},
		// T2's encoding and comment faces. The five invalid-UTF-8 forms
		// are the nameable shapes: a stray lead, a truncated tail, a
		// surrogate, a past-range sequence, an overlong form. The BOM
		// pair covers the leading strip and the interior error; the
		// bare-CR entries cover both the plain and the after-BOM-strip
		// walk (the reference's CR loop indexes the stripped string while
		// positions come from the original — the port carries that
		// quirk). The 0xFF shape lives only in the lex-test corpus and is
		// excluded as one of the thirteen unnameable values.
		entry{"literal:utf8-stray-lead", "\xc3("},
		entry{"literal:utf8-truncated", "\xe2\x82"},
		entry{"literal:utf8-surrogate", "\xed\xa0\x80"},
		entry{"literal:utf8-past-range", "\xf4\x90\x80\x80"},
		entry{"literal:utf8-overlong", "\xe0\x80\x80"},
		entry{"literal:bom-leading", "\uFEFFlet x\r\n"},
		entry{"literal:bom-interior", "a\uFEFFb"},
		entry{"literal:cr-bare", "let x = 1\rlet y = 2"},
		entry{"literal:cr-after-bom", "\uFEFFlet x\ry"},
		entry{"literal:crlf", "let x\r\nlet y\r\n"},
		entry{"literal:block-comment-open", "/* never closed"},
		entry{"literal:docs-run", "/// one\n/// two\nlet x\n"},
		entry{"literal:docs-broken", "/// one\n// plain\n/// two\nlet x\n"},
		// T3's numeric face: one positive and one negative per E0006
		// clause (separator, base prefix, base digits, leading zero, dot
		// sides, exponent, closed suffix, based-is-integer), plus the
		// maximal-munch boundaries — the `..` range token never joins a
		// span, and an undotted decimal with an identifier-shaped tail
		// (`1e5`) cuts after the integer.
		entry{"literal:num-sep-pos", "1_000_000"},
		entry{"literal:num-sep-neg", "1__0"},
		entry{"literal:num-sep-edge", "1_"},
		entry{"literal:num-based-hex", "0xFF"},
		entry{"literal:num-based-bin", "0b1010_1011"},
		entry{"literal:num-based-oct", "0o777"},
		entry{"literal:num-prefix-neg", "0x"},
		entry{"literal:num-based-dig-neg", "0o8"},
		entry{"literal:num-hex-letter-neg", "0xAG"},
		entry{"literal:num-leadzero-neg", "01"},
		entry{"literal:num-leadzero-sep", "0_1"},
		entry{"literal:num-zero", "0"},
		entry{"literal:num-float-pos", "1.5"},
		entry{"literal:num-float-exp", "1.5e3"},
		entry{"literal:num-float-exp-neg-sign", "1.5E-3"},
		entry{"literal:num-float-exp-sep", "1.5e3_3"},
		entry{"literal:num-dot-sides-neg", "1."},
		entry{"literal:num-exp-neg", "1.5e"},
		entry{"literal:num-exp-sign-neg", "1.5e+"},
		entry{"literal:num-based-dot-neg", "0x1.5"},
		entry{"literal:num-int-suffix", "42i8"},
		entry{"literal:num-suffix-float-on-int", "42f32"},
		entry{"literal:num-suffix-unknown", "42int"},
		entry{"literal:num-suffix-on-float", "1.5i8"},
		entry{"literal:num-suffix-after-exp", "1.5e3x"},
		entry{"literal:num-cut-ident-tail", "1e5"},
		entry{"literal:num-range-dotdot", "1..5"},
		entry{"literal:num-based-range", "0x1..2"},
		// T3's rune face: the plain character forms and E0003's two
		// shapes (unterminated, and empty-or-multi-character). The escape
		// forms ('\n', '\u{41}') ride T4's consumeEscape; they live with
		// the string block below.
		entry{"literal:rune-pos", "'a'"},
		entry{"literal:rune-pos-nonascii", "'中'"},
		entry{"literal:rune-empty-neg", "''"},
		entry{"literal:rune-multi-neg", "'ab'"},
		entry{"literal:rune-eof-neg", "'a"},
		entry{"literal:rune-newline-neg", "'a\n'"},
		// T4's string and escape face: the closed escape set, the \u
		// domain rule (at most U+10FFFF, no surrogates), the dangling
		// backslash as an unterminated literal, and the interpolation
		// hole's brace/paren balance — including chapter 1's own `${f(}`
		// shape, where the closing quote falls inside the still-open
		// hole.
		entry{"literal:str-plain", "\"abc\""},
		entry{"literal:str-dollar-plain", "\"cost: $5\""},
		entry{"literal:str-escapes", "\"\\n\\t\\r\\0\\\\\\\"\\'\""},
		entry{"literal:str-u-escape", "\"\\u{41}\""},
		entry{"literal:str-u-max", "\"\\u{10FFFF}\""},
		entry{"literal:str-nonascii", "\"héllo — 中\""},
		entry{"literal:str-esc-neg", "\"\\e\""},
		entry{"literal:str-u-surrogate", "\"\\u{D800}\""},
		entry{"literal:str-u-past-max", "\"\\u{110000}\""},
		entry{"literal:str-u-empty", "\"\\u{}\""},
		entry{"literal:str-u-seven", "\"\\u{1234567}\""},
		entry{"literal:str-u-nobrace", "\"\\u41\""},
		entry{"literal:str-dangling", "\"abc\\"},
		entry{"literal:str-unterminated", "\"abc"},
		entry{"literal:hole-balanced", "\"a${1 + 1}b\""},
		entry{"literal:hole-nested-string", "\"${\"x\"}\""},
		entry{"literal:hole-unbalanced", "\"${f(}\""},
		entry{"literal:hole-unclosed", "\"${1\""},
		// T4's attribute face: the validation order (form E0001, then
		// the literal-only rule E0009, then the empty whitelist E0008),
		// the balanced-argument walk with its string/rune literals, and
		// a bad number dying inside the arguments (E0006 at the
		// argument's own position).
		entry{"literal:attr-whitelist", "#[test]"},
		entry{"literal:attr-nonliteral", "#[x(y)]"},
		entry{"literal:attr-open-eof", "#["},
		entry{"literal:attr-hash-alone", "# x"},
		entry{"literal:attr-digit-name", "#[1x]"},
		entry{"literal:attr-keyword-name", "#[fn]"},
		entry{"literal:attr-with-args", "#[cfg(1)]"},
		entry{"literal:attr-arg-literals", "#[x(\"a\", 2, true)]"},
		entry{"literal:attr-arg-list", "#[x([1, 2])]"},
		entry{"literal:attr-args-mismatch", "#[x(]"},
		entry{"literal:attr-args-badnum", "#[x(1__0)]"},
		entry{"literal:attr-spaced", "#[x ( 1 )]"},
		entry{"literal:rune-escape", "'\\n'"},
		entry{"literal:rune-escape-u", "'\\u{41}'"},
	)
	lib, err := filepath.Glob(filepath.Join("..", "..", "stdlib", "src", "*.we"))
	if err != nil {
		t.Fatalf("glob stdlib: %v", err)
	}
	for _, f := range lib {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		cs = append(cs, entry{"stdlib:" + filepath.Base(f), string(data)})
	}
	own, err := filepath.Glob(filepath.Join("..", "..", "wec", "src", "*.we"))
	if err != nil {
		t.Fatalf("glob wec: %v", err)
	}
	for _, f := range own {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		cs = append(cs, entry{"wec:" + filepath.Base(f), string(data)})
	}
	cs = append(cs, lexTestLiterals(t)...)
	cases, err := filepath.Glob(filepath.Join("..", "conformance", "testdata", "cases", "*.json"))
	if err != nil {
		t.Fatalf("glob cases: %v", err)
	}
	for _, cf := range cases {
		var c struct {
			Setup struct {
				Files map[string]string `json:"files"`
			} `json:"setup"`
		}
		data, err := os.ReadFile(cf)
		if err != nil {
			t.Fatalf("read %s: %v", cf, err)
		}
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatalf("decode %s: %v", cf, err)
		}
		paths := make([]string, 0, len(c.Setup.Files))
		for p := range c.Setup.Files {
			paths = append(paths, p)
		}
		sortStrings(paths)
		base := strings.TrimSuffix(filepath.Base(cf), ".json")
		for _, p := range paths {
			if strings.HasSuffix(p, ".we") {
				cs = append(cs, entry{"case:" + base + ":" + p, c.Setup.Files[p]})
			}
		}
	}
	return cs
}

// lexSampleRe matches the sample sources the lex package's own tests hand
// to lexMust or lexFail — backquoted or double-quoted Go string literals.
var lexSampleRe = regexp.MustCompile(`(?:lexMust|lexFail)\(t, (` + "`[^`]*`" + `|"(?:[^"\\\n]|\\.)*")`)

// lexTestLiterals extracts those samples verbatim (corpus item ③ of
// design D7): strconv decodes the Go literal, so \r, \uFEFF, and \xff
// reach the corpus as the exact bytes the reference lexer sees. The
// samples span every face and all of them run (the T2-T4 stage fence
// retired at T5); the unnameable-byte corner (the "\xff" sample) is
// excluded with a logged disclosure.
func lexTestLiterals(t *testing.T) []entry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "lex", "lex_test.go"))
	if err != nil {
		t.Fatalf("read lex_test.go: %v", err)
	}
	var cs []entry
	for _, m := range lexSampleRe.FindAllStringSubmatch(string(data), -1) {
		src, err := strconv.Unquote(m[1])
		if err != nil {
			t.Fatalf("unquote %s: %v", m[1], err)
		}
		cs = append(cs, entry{"lextest:" + m[1], src})
	}
	return cs
}

// sortStrings is sort.Strings, kept local so the tower's imports stay the
// contract it exercises.
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

// buildWec copies the wec project into a fresh temp directory and builds
// it once with the in-process CLI (the conformance runner's posture), so
// the artifact's provenance never depends on this repository's working
// directory. A clean build is the full pipeline through code generation,
// so it subsumes `we check`'s verdict on the project.
func buildWec(t *testing.T) (bin, dir string) {
	t.Helper()
	dir = t.TempDir()
	root := filepath.Join(dir, "wec")
	if err := copyTree(filepath.Join("..", "..", "wec"), root); err != nil {
		t.Fatalf("copy wec: %v", err)
	}
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"build", root}, &out, &errb); code != 0 {
		t.Fatalf("we build: exit %d\n%s%s", code, out.String(), errb.String())
	}
	bin = filepath.Join(root, "build", "wec")
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("artifact missing: %v", err)
	}
	return bin, dir
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// runWec executes the artifact once with src as wec-input.we in the
// artifact's working directory (the driver contract: a fixed path, since
// the language has no argv face) and returns its stdout. The contract
// fixes the exit code at 0 — diagnostics ride the output, not the process
// status — so any nonzero exit is itself a failure.
func runWec(t *testing.T, bin, dir, src string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "wec-input.we"), []byte(src), 0o644); err != nil {
		t.Fatalf("write wec-input.we: %v", err)
	}
	var out, errb bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("wec artifact: %v\n%s", err, errb.String())
	}
	return out.String()
}

// TestLexReconciliation is B3b's differential bar at its lexical layer:
// for every corpus entry — the full corpus; the T2-T4 stage fence is
// gone, retired at T5 when its vocabulary reached the reference's whole
// face — the wec artifact's serialized output must equal internal/lex's
// live serialization byte for byte. The single remaining exclusion is the
// thirteen-unnameable-byte corner (follow-up #30), disclosed per entry.
func TestLexReconciliation(t *testing.T) {
	bin, dir := buildWec(t)
	ran := 0
	for _, e := range loadCorpus(t) {
		_, _, d := lex.Scan("wec-input.we", []byte(e.src))
		if d != nil && d.Code() == "E0001" && strings.Contains(d.Message(), "is not valid UTF-8") && !firstBadByteNameable(e.src) {
			t.Logf("corpus: %s excluded — its first bad byte is one of the thirteen unnameable values (follow-up #30)", e.name)
			continue
		}
		ran++
		got := runWec(t, bin, dir, e.src)
		want := serialize([]byte(e.src))
		if !bytes.Equal([]byte(got), []byte(want)) {
			t.Errorf("%s: output diverges\nwant: %q\ngot:  %q", e.name, want, got)
		}
	}
	if ran == 0 {
		t.Fatal("the corpus admitted no entries — the extractor is broken")
	}
	t.Logf("reconciled %d corpus entries", ran)
}

// TestSerializationContractPinsTheFormat anchors the shared serialization
// spec's own shape with hand-written bytes — the one place hand-written
// expectations are legitimate (design D5): they pin the FORMAT, not lexer
// behavior, so a spec rewrite on either side of the tower fails here
// first. The empty source is the minimal shape: the header, one eof token
// with empty text (the trailing space is the empty text field's
// separator), no docs section.
func TestSerializationContractPinsTheFormat(t *testing.T) {
	if got := serialize([]byte("")); got != "tokens\nT 1 1 eof \n" {
		t.Fatalf("empty source serializes to %q", got)
	}
}

// TestCorpusIsNotEmpty sanity-checks the extractor: the corpus must carry
// the stdlib sources and the conformance case sources it claims, or the
// reconciliation above would be reconciling a fraction of what it names.
func TestCorpusIsNotEmpty(t *testing.T) {
	cs := loadCorpus(t)
	var stdlib, own, lextest, conformance int
	for _, e := range cs {
		switch {
		case strings.HasPrefix(e.name, "stdlib:"):
			stdlib++
		case strings.HasPrefix(e.name, "wec:"):
			own++
		case strings.HasPrefix(e.name, "lextest:"):
			lextest++
		case strings.HasPrefix(e.name, "case:"):
			conformance++
		}
	}
	if stdlib == 0 || own == 0 || lextest == 0 || conformance == 0 {
		t.Fatalf("corpus incomplete: stdlib=%d wec=%d lextest=%d conformance=%d of %d", stdlib, own, lextest, conformance, len(cs))
	}
	t.Logf("corpus: %d entries (stdlib=%d, wec=%d, lextest=%d, conformance=%d)", len(cs), stdlib, own, lextest, conformance)
}
