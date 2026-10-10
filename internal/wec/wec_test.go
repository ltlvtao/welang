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

// serializeOutput is the B3c full-output reference (design D5): the
// lexical serialization above, then whichever section internal/parser's
// three-return dispatch appends — nothing more on a lexical error (the E
// line is the whole output), the diagnostic E line or the NotImplemented N
// line behind the lexical sections when the parse stops, or the AST
// serializer's success section (astser_test.go, the spec's authority).
func serializeOutput(src []byte) string {
	lexPart := serialize(src)
	if strings.HasPrefix(lexPart, "E ") {
		return lexPart
	}
	return lexPart + parseSection(src)
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
		// T2's file-structure face: the item dispatch, the naming matrix
		// (E0011 rides T3's type declarations; E0012/E0013 land here), the
		// module name-space collisions, the pub/byval/byres prefix order,
		// and the top-level error codes. Positives stay inside the T2 faces
		// — empty fn bodies, literal and identifier initializers, named and
		// qualified type references without tuples; everything wider (list
		// literals, calls, statements) keeps diverging until its own task.
		entry{"t2:import-basic", "import std.io\n"},
		entry{"t2:import-alias", "import std.string as s\n"},
		entry{"t2:import-two", "import std.io\nimport std.string\n"},
		entry{"t2:effect-decls", "effect db\npub effect audit\n"},
		entry{"t2:fn-empty", "fn f() {}\n"},
		entry{"t2:fn-param", "fn id(x: Int64) -> Int64 {}\n"},
		entry{"t2:fn-two-params", "fn add(a: Int64, b: Int64) {}\n"},
		entry{"t2:fn-effect", "fn ping() effect io {}\n"},
		entry{"t2:fn-effect-qualified", "fn f() effect std.db {}\n"},
		entry{"t2:fn-qual-type", "fn f(x: std.string.String) {}\n"},
		entry{"t2:fn-generic-type", "fn f(xs: List<Int64>) {}\n"},
		entry{"t2:let-annot", "let n: Int64 = 7\n"},
		entry{"t2:let-string", "let s = \"abc\"\n"},
		entry{"t2:let-bools", "let a = true\nlet b = false\n"},
		entry{"t2:let-rune", "let r = 'a'\n"},
		entry{"t2:let-underscore", "let _ = 1\n"},
		entry{"t2:let-ident-init", "let x = y\n"},
		entry{"t2:file-mixed", "import std.io\neffect db\nfn go(x: Int64) effect io {}\nlet n = 3\n"},
		entry{"t2:err-e0012-fn", "fn F() {}\n"},
		entry{"t2:err-e0012-let", "let X = 1\n"},
		entry{"t2:err-e0012-effect", "effect DB\n"},
		entry{"t2:err-e1403", "effect io\n"},
		entry{"t2:err-e0404-fn", "fn f() {}\nfn f() {}\n"},
		entry{"t2:err-e0404-let", "let x = 1\nlet x = 2\n"},
		entry{"t2:err-e0404-import", "import std.io\nlet io = 1\n"},
		entry{"t2:err-e0404-over-e1403", "let io = 1\neffect io\n"},
		entry{"t2:err-e0013-import", "import std.Io\n"},
		entry{"t2:err-import-eof", "import\n"},
		entry{"t2:err-import-int", "import 1\n"},
		entry{"t2:err-var-top", "var x = 1\n"},
		entry{"t2:err-return-top", "return 1\n"},
		entry{"t2:err-mock-top", "mock std.io println\n"},
		entry{"t2:err-pub-eof", "pub\n"},
		entry{"t2:err-pub-ident", "pub x\n"},
		entry{"t2:err-byval-pub", "byval pub type T {}\n"},
		entry{"t2:err-byval-eq", "byval = 1\n"},
		entry{"t2:err-byval-eof", "byval\n"},
		entry{"t2:err-byres-ident", "byres x\n"},
		entry{"t2:err-pub-byres-eof", "pub byres\n"},
		entry{"t2:err-e0102-first", "+ 1\n"},
		entry{"t2:err-e0102-after", "let x = 1\n+ 1\n"},
		entry{"t2:err-item-fallthrough", "x = 1\n"},
		entry{"t2:err-fn-name-int", "fn 1() {}\n"},
		entry{"t2:err-fn-noparen", "fn f {}\n"},
		entry{"t2:err-fn-nobody", "fn f()\n"},
		entry{"t2:err-fn-nobody-params", "fn f(x: Int64)\n"},
		entry{"t2:err-param-noannot", "fn f(x) {}\n"},
		entry{"t2:err-param-int", "fn f(1) {}\n"},
		entry{"t2:err-param-eof", "fn f(\n"},
		entry{"t2:err-param-tail", "fn f(x: Int64 2) {}\n"},
		entry{"t2:err-effect-seg-tok", "fn f() effect {}\n"},
		entry{"t2:err-typeref-lower", "fn f(x: int) {}\n"},
		entry{"t2:err-typeref-eof", "let x:\n"},
		entry{"t2:err-binding-eq-eof", "let x\n"},
		entry{"t2:err-binding-tok", "let x 1\n"},
		entry{"t2:n-mut-param", "fn f(mut x: Int64) {}\n"},
		// T3's declaration face: the composite declaration family — record
		// in its three categories, sum with bare and payload variants,
		// newtype, interface with associated types and method signatures,
		// impl with associated-type bindings and methods, the foreign
		// block's production side — and the type-reference family beyond
		// the named forms: the unit and tuple forms, the fn type with its
		// effect tags, and the generic clause and where clause on every
		// declaration that takes one. The E17xx matrix here is the
		// parser-production side (E1701's ABI literal through descText's
		// split decode, E1703's required effect segment, E1704's
		// monomorphic boundary); E1702 is the statement-position rejection
		// and lands with T4's statement dispatch. Dyn<I> rides T2's named
		// application face — the entry pins the spelling, not new parser
		// behavior.
		entry{"t3:sum-bare", "type Shape = Circle | Square\n"},
		entry{"t3:sum-payload", "type Opt = Some(Int64) | None\n"},
		entry{"t3:sum-multi-payload", "type R = P(Int64, Int64, String)\n"},
		entry{"t3:sum-pub", "pub type T = A | B\n"},
		entry{"t3:sum-byval", "byval type T = A | B\n"},
		entry{"t3:sum-pub-byval", "pub byval type T = A | B\n"},
		entry{"t3:sum-generic", "type Tree<T> = Leaf(T) | Node(Tree<T>, Tree<T>)\n"},
		entry{"t3:sum-derives", "type C = Red | Green derives Eq, Hash\n"},
		entry{"t3:err-sum-name-int", "type 1 = A\n"},
		entry{"t3:err-sum-name-camel", "type shape = A\n"},
		entry{"t3:err-sum-noeq", "type Shape A\n"},
		entry{"t3:err-sum-noeq-eof", "type Shape\n"},
		entry{"t3:err-sum-variant-eof", "type S =\n"},
		entry{"t3:err-sum-variant-tok", "type S = 1\n"},
		entry{"t3:err-sum-payload-empty", "type S = A()\n"},
		entry{"t3:err-sum-payload-eof", "type S = A(Int64\n"},
		entry{"t3:err-sum-payload-tail", "type S = A(Int64 2)\n"},
		entry{"t3:err-sum-payload-arity", "type S = A(Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64)\n"},
		entry{"t3:err-sum-variant-dup", "type S = A | A\n"},
		entry{"t3:err-sum-line-pipe", "type S = A\n| B\n"},
		entry{"t3:err-sum-derives-dup", "type S = A derives Eq, Eq\n"},
		entry{"t3:err-sum-derives-shareable", "type S = A derives Shareable\n"},
		entry{"t3:err-sum-derives-eof", "type S = A derives\n"},
		entry{"t3:err-derives-item-pos", "type S = A\nderives Eq\n"},
		entry{"t3:record-gc", "record P { x: Int64, y: Int64 }\n"},
		entry{"t3:record-byval", "byval record P { x: Int64 }\n"},
		entry{"t3:record-byres", "byres record R { h: Int64 }\n"},
		entry{"t3:record-pub", "pub record P { x: Int64 }\n"},
		entry{"t3:record-pub-byval", "pub byval record P { x: Int64 }\n"},
		entry{"t3:record-pub-byres", "pub byres record R { h: Int64 }\n"},
		entry{"t3:record-empty", "record P { }\n"},
		entry{"t3:record-generic", "record Box<T> { v: T }\n"},
		entry{"t3:record-newline-sep", "record P { x: Int64\ny: Int64 }\n"},
		entry{"t3:record-trailing-comma", "record P { x: Int64, }\n"},
		entry{"t3:record-tuple-field", "record P { pair: (Int64, Int64) }\n"},
		entry{"t3:record-derives", "record P { x: Int64 } derives Eq, Hash, Show\n"},
		entry{"t3:err-record-name-int", "record 1 { }\n"},
		entry{"t3:err-record-name-camel", "record p { }\n"},
		entry{"t3:err-record-noblock", "record P x\n"},
		entry{"t3:err-record-noblock-eof", "record P\n"},
		entry{"t3:err-record-field-int", "record P { 1: Int64 }\n"},
		entry{"t3:err-record-field-noannot", "record P { x }\n"},
		entry{"t3:err-record-field-eof", "record P {\n"},
		entry{"t3:err-record-field-tail", "record P { x: Int64 2 }\n"},
		entry{"t3:err-record-dup", "record P { x: Int64 }\nrecord P { y: Int64 }\n"},
		entry{"t3:newtype-basic", "newtype Meters(Int64)\n"},
		entry{"t3:newtype-pub", "pub newtype Meters(Int64)\n"},
		entry{"t3:newtype-generic", "newtype Wrap<T>(T)\n"},
		entry{"t3:newtype-derives", "newtype M(Int64) derives Eq\n"},
		entry{"t3:err-newtype-name", "newtype 1(Int64)\n"},
		entry{"t3:err-newtype-noopen", "newtype M Int64)\n"},
		entry{"t3:err-newtype-eof-open", "newtype M\n"},
		entry{"t3:err-newtype-noclose", "newtype M(Int64\n"},
		entry{"t3:err-newtype-comma", "newtype M(Int64, Int64)\n"},
		entry{"t3:err-tparam-empty", "record B<> { }\n"},
		entry{"t3:err-tparam-tok", "record B<1> { }\n"},
		entry{"t3:err-tparam-lower", "record B<t> { }\n"},
		entry{"t3:err-tparam-eof", "record B<\n"},
		entry{"t3:err-tparam-tail", "record B<T1 T2> { }\n"},
		entry{"t3:err-tparam-count", "record B<T1, T2, T3, T4, T5, T6, T7, T8, T9> { }\n"},
		entry{"t3:iface-basic", "interface Shape { fn area(self) -> Int64 }\n"},
		entry{"t3:iface-pub", "pub interface Shape { fn area(self) -> Int64 }\n"},
		entry{"t3:iface-assoc", "interface It { type Item\nfn next(self) -> Item }\n"},
		entry{"t3:iface-method-before-assoc", "interface It { fn size(self) -> Int64\ntype Item }\n"},
		entry{"t3:iface-default-body", "interface I { fn f(self) { } }\n"},
		entry{"t3:iface-mut-recv", "interface S { fn push(mut self, v: Int64) }\n"},
		entry{"t3:iface-generic", "interface C<T> { fn get(self) -> T }\n"},
		entry{"t3:iface-generic-method", "interface I { fn m<T>(self, x: T) -> T }\n"},
		entry{"t3:iface-method-effect", "interface I { fn m(self) effect io }\n"},
		entry{"t3:err-iface-name", "interface 1 { }\n"},
		entry{"t3:err-iface-noblock", "interface I x\n"},
		entry{"t3:err-iface-where", "interface I where T: Eq { }\n"},
		entry{"t3:err-iface-item", "interface I { let x = 1 }\n"},
		entry{"t3:err-iface-item-sep", "interface I { fn a(self) fn b(self) }\n"},
		entry{"t3:err-assoc-name", "interface I { type 1 }\n"},
		entry{"t3:err-assoc-bound", "interface I { type Item: Eq }\n"},
		entry{"t3:err-assoc-count", "interface I { type A\ntype B\ntype C\ntype D\ntype E }\n"},
		entry{"t3:err-msig-name", "interface I { fn 1(self) }\n"},
		entry{"t3:err-msig-noparen", "interface I { fn f }\n"},
		entry{"t3:err-msig-where", "interface I { fn f(self) where T: Eq }\n"},
		entry{"t3:err-msig-dup", "interface I { fn f(self)\nfn f(self) }\n"},
		entry{"t3:err-recv-none", "interface I { fn f() }\n"},
		entry{"t3:err-recv-annotated", "interface I { fn f(x: Int64) }\n"},
		entry{"t3:err-recv-spell", "interface I { fn f(this) }\n"},
		entry{"t3:err-recv-mut-spell", "interface I { fn f(mut this) }\n"},
		entry{"t3:impl-inherent", "impl P { fn get(self) -> Int64 { } }\n"},
		entry{"t3:impl-for", "impl Show for P { fn show(self) { } }\n"},
		entry{"t3:impl-generic-for", "impl<T> Store<T> for Box<T> { fn get(self) -> T { } }\n"},
		entry{"t3:impl-assoc", "impl It for P { type Item = Int64\nfn next(self) -> Item { } }\n"},
		entry{"t3:impl-where", "impl<T> Eq for Box<T> where T: Eq { fn eq(self) { } }\n"},
		entry{"t3:impl-pub-method", "impl P { pub fn get(self) { } }\n"},
		entry{"t3:impl-method-params", "impl P { fn set(self, v: Int64) { } }\n"},
		entry{"t3:impl-method-effect", "impl P { fn m(self) effect io { } }\n"},
		entry{"t3:err-impl-head-lower", "impl x { }\n"},
		entry{"t3:err-impl-noblock", "impl P x\n"},
		entry{"t3:err-impl-assoc-after-method", "impl P { fn m(self) { }\ntype T = Int64 }\n"},
		entry{"t3:err-impl-assoc-noeq", "impl P { type T Int64 }\n"},
		entry{"t3:err-impl-item", "impl P { let x = 1 }\n"},
		entry{"t3:err-impl-method-nobody", "impl P { fn m(self) }\n"},
		entry{"t3:err-impl-method-pub-tok", "impl P { pub x() { } }\n"},
		entry{"t3:fn-generic", "fn id<T>(x: T) -> T { }\n"},
		entry{"t3:fn-where", "fn f<T>(x: T) where T: Eq { }\n"},
		entry{"t3:where-bound-plus", "fn f<T>(x: T) where T: Eq + Hash { }\n"},
		entry{"t3:where-eq", "fn f<T>(x: T) where T.Item == Int64 { }\n"},
		entry{"t3:where-multi", "fn f<T, U>(x: T, y: U) where T: Eq, U: Hash { }\n"},
		entry{"t3:err-where-subject", "fn f<T>(x: T) where 1: Eq { }\n"},
		entry{"t3:err-where-bound-lower", "fn f<T>(x: T) where T: eq { }\n"},
		entry{"t3:err-where-eq-noarrow", "fn f<T>(x: T) where T.Item = Int64 { }\n"},
		entry{"t3:foreign-fn", "foreign \"c\" { fn abs(x: Int64) effect net -> Int64 }\n"},
		entry{"t3:foreign-pure-fn", "foreign \"c\" { fn f(x: Int64) effect }\n"},
		entry{"t3:foreign-fn-pub", "foreign \"c\" { pub fn abs(x: Int64) effect io -> Int64 }\n"},
		entry{"t3:foreign-record", "foreign \"c\" { record File }\n"},
		entry{"t3:foreign-byval-record", "foreign \"c\" { byval record P }\n"},
		entry{"t3:foreign-byres-record", "foreign \"c\" { byres record P }\n"},
		entry{"t3:foreign-pub-record", "foreign \"c\" { pub record File }\n"},
		entry{"t3:foreign-two", "foreign \"c\" { fn a(x: Int64) effect\nrecord F }\n"},
		entry{"t3:foreign-abi-uescape", "foreign \"\\u{63}\" { record F }\n"},
		entry{"t3:err-e1701-string", "foreign \"rust\" { }\n"},
		entry{"t3:err-e1701-nonstring", "foreign c { }\n"},
		entry{"t3:err-e1701-hole", "foreign \"a${1}c\" { }\n"},
		entry{"t3:err-foreign-open", "foreign \"c\" x\n"},
		entry{"t3:err-foreign-open-eof", "foreign \"c\"\n"},
		entry{"t3:err-e1703", "foreign \"c\" { fn f(x: Int64) }\n"},
		entry{"t3:err-e1704", "foreign \"c\" { fn f<T>(x: T) effect }\n"},
		entry{"t3:err-foreign-body", "foreign \"c\" { fn f(x: Int64) effect { } }\n"},
		entry{"t3:err-foreign-fields", "foreign \"c\" { record F { x: Int64 } }\n"},
		entry{"t3:err-foreign-item", "foreign \"c\" { let x = 1 }\n"},
		entry{"t3:err-foreign-item-pub-eof", "foreign \"c\" { pub\n"},
		entry{"t3:ty-unit", "fn f(u: ()) { }\n"},
		entry{"t3:ty-tuple", "fn f(p: (Int64, Int64)) { }\n"},
		entry{"t3:ty-tuple-trailing", "fn f(p: (Int64, Int64,)) { }\n"},
		entry{"t3:ty-tuple-nested", "fn f(p: (Int64, (Int64, String))) { }\n"},
		entry{"t3:ty-fn", "fn f(cb: fn(Int64) -> Int64) { }\n"},
		entry{"t3:ty-fn-two", "fn f(cb: fn(Int64, String) -> Bool) { }\n"},
		entry{"t3:ty-fn-tags", "fn f(cb: fn(Int64) effect io -> Int64) { }\n"},
		entry{"t3:ty-fn-unit-ret", "fn f(cb: fn(Int64) -> ()) { }\n"},
		entry{"t3:ty-dyn", "fn f(d: Dyn<Show>) { }\n"},
		entry{"t3:err-ty-single-paren", "fn f(x: (Int64)) { }\n"},
		entry{"t3:err-ty-tuple-arity", "fn f(x: (Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64)) { }\n"},
		entry{"t3:err-ty-tuple-eof", "fn f(x: (Int64,\n"},
		entry{"t3:err-ty-fn-noparen", "fn f(x: fn -> Int64) { }\n"},
		entry{"t3:err-ty-fn-noarrow", "fn f(x: fn(Int64)) { }\n"},
		entry{"t3:err-ty-fn-eof", "fn f(x: fn(\n"},
		entry{"t3:err-ty-fn-arrow-eof", "fn f(x: fn(Int64) ->\n"},
		// T4's statement face: the block loop and statement dispatch — the
		// direct flag with defer's one-layer law (E0204/E0203), the mock
		// depth law's failure arm (E1802 at depth 0; the depth-2 positive
		// rides T4-B's test face), let/var bindings and both assignment
		// heads, return's hasRet check (E0402), the if/else chain with the
		// valueless-if rule (E0202), loopDepth on break/continue (E0201),
		// for, scope resource with its own E0404 wording, the statement
		// separation law (E0105), foreign's statement-position code
		// (E1702), and the tail-expression form. Operands stay inside the
		// faces already landed — literals, idents, true/false — since
		// richer expressions ride T5.
		entry{"t4:stmt-let", "fn f() { let x = 1\n }\n"},
		entry{"t4:stmt-var-annot", "fn f() { var n: Int64 = 7\n }\n"},
		entry{"t4:stmt-two", "fn f() { let x = 1\nlet y = 2\n }\n"},
		entry{"t4:stmt-tail-expr", "fn f() -> Int64 { 1\n }\n"},
		entry{"t4:stmt-assign-name", "fn f() { x = 2\n }\n"},
		entry{"t4:stmt-assign-self", "impl P { fn set(self) { self.x = 1\n }\n }\n"},
		entry{"t4:err-stmt-nosep", "fn f() { let x = 1 let y = 2 }\n"},
		entry{"t4:err-stmt-pub", "fn f() { pub x\n }\n"},
		entry{"t4:err-foreign-stmt", "fn f() { foreign \"c\" { }\n }\n"},
		entry{"t4:err-mock-fnbody", "fn f() { mock std.io println\n }\n"},
		entry{"t4:return-bare", "fn f() { return\n }\n"},
		entry{"t4:return-value", "fn f() -> Int64 { return 1\n }\n"},
		entry{"t4:err-return-value-noret", "fn f() { return 1\n }\n"},
		entry{"t4:err-return-eof", "fn f() { return"},
		entry{"t4:if-basic", "fn f(c: Bool) { if c { let x = 1\n }\n }\n"},
		entry{"t4:if-else", "fn f(c: Bool) { if c { let x = 1\n } else { let y = 2\n }\n }\n"},
		entry{"t4:if-else-if", "fn f(a: Bool, b: Bool) { if a { let x = 1\n } else if b { let y = 2\n } else { let z = 3\n }\n }\n"},
		entry{"t4:if-else-block-if", "fn f(a: Bool) { if a { let x = 1\n } else { if a { let y = 2\n }\n }\n }\n"},
		entry{"t4:if-value-tail", "fn f(c: Bool) -> Int64 { if c { 1\n } else { 2\n }\n }\n"},
		entry{"t4:if-tail-noelse", "fn f(c: Bool) -> Int64 { if c { 1\n }\n }\n"},
		entry{"t4:err-if-valueless-init", "fn f(c: Bool) { let x = if c { 1\n }\n }\n"},
		entry{"t4:err-if-nothing", "fn f(c: Bool) { if c let x = 1\n }\n"},
		entry{"t4:while-basic", "fn f(c: Bool) { while c { let x = 1\n }\n }\n"},
		entry{"t4:loop-break", "fn f() { loop { break\n }\n }\n"},
		entry{"t4:while-continue", "fn f(c: Bool) { while c { continue\n }\n }\n"},
		entry{"t4:err-break-noloop", "fn f() { break\n }\n"},
		entry{"t4:err-continue-noloop", "fn f() { continue\n }\n"},
		entry{"t4:err-break-top", "break\n"},
		entry{"t4:defer-block", "fn f() { defer { let x = 1\n }\n }\n"},
		entry{"t4:err-defer-nested", "fn f(c: Bool) { while c { defer { }\n }\n }\n"},
		entry{"t4:err-defer-nonblock", "fn f() { defer g\n }\n"},
		entry{"t4:for-binding", "fn f(xs: List<Int64>) { for x in xs { let y = x\n }\n }\n"},
		entry{"t4:for-wildcard", "fn f(xs: List<Int64>) { for _ in xs { }\n }\n"},
		entry{"t4:err-for-noin", "fn f(xs: List<Int64>) { for x xs { }\n }\n"},
		entry{"t4:scope-resource", "fn f() { scope resource(h = g) { let x = 1\n }\n }\n"},
		entry{"t4:scope-resource-two", "fn f() { scope resource(h = g, i = h) { }\n }\n"},
		entry{"t4:err-scope-underscore", "fn f() { scope resource(_ = g) { }\n }\n"},
		entry{"t4:err-scope-dup", "fn f() { scope resource(h = g, h = g) { }\n }\n"},
		entry{"t4:err-scope-camel", "fn f() { scope resource(H = g) { }\n }\n"},
		// T4-B's pattern and test face: the let/for tuple pattern
		// (irrefutable elements only), the match expression with its arm
		// family (patterns, guards, or-chains under the two name-set laws
		// E0302/E0404), and the test block with mock declarations. The
		// tower's fixed input name never ends _test.we, so a green test
		// block parses fully and then both sides report E1801
		// identically — the P test/mock rows themselves are unobservable
		// here (a disclosed corner; their spellings are pinned against
		// the authority table by hand), while every diagnostic inside the
		// block is observable because it latches before the module check.
		entry{"t4b:let-tuple", "fn f() { let (a, b) = g\n }\n"},
		entry{"t4b:let-tuple-three", "fn f() { let (a, b, c) = g\n }\n"},
		entry{"t4b:let-tuple-nested", "fn f() { let (a, (b, _)) = g\n }\n"},
		entry{"t4b:let-tuple-wild", "fn f() { let (_, x) = g\n }\n"},
		entry{"t4b:let-tuple-trailing", "fn f() { let (a, b,) = g\n }\n"},
		entry{"t4b:for-tuple", "fn f(m: Map<Int64, Int64>) { for (k, v) in m { }\n }\n"},
		entry{"t4b:err-let-tuple-empty", "fn f() { let () = g\n }\n"},
		entry{"t4b:err-let-tuple-single", "fn f() { let (a) = g\n }\n"},
		entry{"t4b:err-let-tuple-pascal", "fn f() { let (A) = g\n }\n"},
		entry{"t4b:err-let-tuple-dup", "fn f() { let (a, a) = g\n }\n"},
		entry{"t4b:err-let-tuple-tail", "fn f() { let (a b) = g\n }\n"},
		// Two top-level tuple lets: the tuple form declares the empty name
		// at line 0, so the second binding's declare finds it — E0404 at
		// the synthesized 0:0 position quoting line 0. Pins the absent
		// sentinel of the We-side name lookup (-1, not 0).
		entry{"t4b:err-topletpat-dup", "let (a, b) = f\nlet (c, d) = g\n"},
		entry{"t4b:match-literal", "fn f(x: Int64) -> Int64 { match x { 1 => 2\n_ => 3\n }\n }\n"},
		entry{"t4b:match-string", "fn f(s: String) -> Int64 { match s { \"a\" => 1\n_ => 0\n }\n }\n"},
		entry{"t4b:match-rune", "fn f(r: Rune) -> Int64 { match r { 'a' => 1\n_ => 0\n }\n }\n"},
		entry{"t4b:match-bool", "fn f(b: Bool) -> Int64 { match b { true => 1\nfalse => 0\n }\n }\n"},
		entry{"t4b:match-binding", "fn f(x: Int64) -> Int64 { match x { n => n\n }\n }\n"},
		entry{"t4b:match-variant", "fn f(o: Opt) -> Int64 { match o { Some(n) => n\nNone => 0\n }\n }\n"},
		entry{"t4b:match-variant-empty-args", "fn f(o: Opt) -> Int64 { match o { Some() => 1\nNone => 0\n }\n }\n"},
		entry{"t4b:match-variant-qual", "fn f(o: Q) -> Int64 { match o { std.Q(n) => n\n }\n }\n"},
		entry{"t4b:match-tuple-pat", "fn f(p: (Int64, Int64)) -> Int64 { match p { (a, b) => a\n }\n }\n"},
		entry{"t4b:match-or", "fn f(x: Int64) -> Int64 { match x { 1 | 2 => 0\n_ => 9\n }\n }\n"},
		entry{"t4b:match-or-names-ok", "fn f(o: Opt) -> Int64 { match o { Some(n) | Query(n) => n\n }\n }\n"},
		entry{"t4b:match-guard", "fn f(x: Int64, c: Bool) -> Int64 { match x { n if c => n\n_ => 0\n }\n }\n"},
		entry{"t4b:match-stmt", "fn f(x: Int64) { match x { _ => g\n }\n }\n"},
		entry{"t4b:err-match-empty", "fn f(x: Int64) -> Int64 { match x { }\n }\n"},
		entry{"t4b:err-match-nosep", "fn f(x: Int64) -> Int64 { match x { 1 => 2 _ => 3\n }\n }\n"},
		entry{"t4b:err-match-noarrow", "fn f(x: Int64) -> Int64 { match x { 1 2\n }\n }\n"},
		entry{"t4b:err-match-nobrace", "fn f(x: Int64) -> Int64 { match x 1\n }\n"},
		entry{"t4b:err-pat-negative", "fn f(x: Int64) -> Int64 { match x { -1 => 0\n_ => 1\n }\n }\n"},
		entry{"t4b:err-pat-or-dup", "fn f(p: (Int64, Int64)) -> Int64 { match p { (a, a) | b => a\n }\n }\n"},
		entry{"t4b:err-pat-or-names", "fn f(o: Opt) -> Int64 { match o { Some(n) | None => n\n }\n }\n"},
		// An or nested inside another or's branch atom (via tuple
		// elements): the inner or's binds already carry the or-ancestor
		// mark, and the outer or's name law must strip exactly one mark —
		// a second marking would leak "!n" into the E0302 message. The
		// inner ors keep their own name sets legal, so the error the
		// reference reports is the outer violation.
		entry{"t4b:err-pat-or-nested-mark", "fn f(x: Int64) -> Int64 { match x { ((Some(n) | Query(n), 5) | n, 6) | q => n\n_ => 0\n }\n }\n"},
		entry{"t4b:err-pat-variant-tail", "fn f(o: Opt) -> Int64 { match o { Some(n => n\n }\n }\n"},
		entry{"t4b:test-block", "test \"d\" { let x = 1\n }\n"},
		entry{"t4b:test-mock", "test \"d\" { mock g() { }\n }\n"},
		entry{"t4b:test-mock-full", "test \"d\" { mock io greet(x: Int64) -> Int64 effect io { return x\n }\n }\n"},
		entry{"t4b:err-test-nodesc", "test 1 { }\n"},
		entry{"t4b:err-test-nobody", "test \"d\" 1\n"},
		entry{"t4b:err-test-eof", "test"},
		entry{"t4b:err-mock-return", "test \"d\" { mock g() { return 1\n }\n }\n"},
		entry{"t4b:err-mock-noparams", "test \"d\" { mock g { }\n }\n"},
		entry{"t4b:err-mock-badqual", "test \"d\" { mock std.io. { }\n }\n"},
		entry{"t4b:err-mock-nobody", "test \"d\" { mock g() 1\n }\n"},
		// T5's expression core: the twelve-level precedence climb with its
		// two non-associative levels (E0104), the `=` residue rule in both
		// contexts (E0103 in a value position, the assignment-attempt E0105
		// in statement position), unary, the postfix chain (member, call,
		// the propagation suffix, the speculative explicit-application `<`),
		// and the primary family — groups, the unit value, tuple and list
		// literals, construction braces with the update form, both closure
		// forms, the block expression, and the value-carrying keyword forms
		// (task, the scope compounds, select). The valueless keyword family
		// pins E0202's raw site word too: a let initializer names it, every
		// other value position leaves it empty (the reference's asymmetry
		// against the if-form's defaulted word).
		entry{"t5:prec-mul-add", "let x = 1 + 2 * 3\n"},
		entry{"t5:prec-group", "let x = (1 + 2) * 3\n"},
		entry{"t5:assoc-left", "let x = 1 - 2 - 3\n"},
		entry{"t5:prec-shift-add", "let x = 1 << 2 + 3\n"},
		entry{"t5:prec-bit-mix", "let x = 1 & 2 ^ 3 | 4\n"},
		entry{"t5:prec-cmp-logic", "let x = 1 < 2 && 3 > 4\n"},
		entry{"t5:prec-logic", "let x = a || b && c\n"},
		entry{"t5:range", "let x = 1 .. 5\n"},
		entry{"t5:unary-mix", "let x = -1 * 2\n"},
		entry{"t5:unary-postfix", "let x = -a.b\n"},
		entry{"t5:unary-not", "fn f(b: Bool) -> Bool { return !b\n }\n"},
		entry{"t5:unary-tilde", "let x = ~mask\n"},
		entry{"t5:unary-chain", "let x = - -1\n"},
		entry{"t5:line-continue", "let x = 1 +\n2\n"},
		entry{"t5:nonassoc-split", "let x = (1 < 2) && (2 < 3)\n"},
		entry{"t5:err-nonaloc-cmp", "let x = 1 < 2 < 3\n"},
		entry{"t5:err-nonaloc-eq", "let x = 1 == 2 != 3\n"},
		entry{"t5:err-nonaloc-shape", "let x = a < b > c\n"},
		entry{"t5:err-nonaloc-range", "let x = 1 .. 2 .. 3\n"},
		entry{"t5:err-e0103-value", "let x = y = 1\n"},
		entry{"t5:err-e0103-arg", "let x = f(y = 1)\n"},
		entry{"t5:err-assign-after-call", "fn f() { g() = 1\n }\n"},
		entry{"t5:member-chain", "let x = a.b.c\n"},
		entry{"t5:call-basic", "let x = f(1, 2)\n"},
		entry{"t5:call-empty", "let x = g()\n"},
		entry{"t5:call-trailing", "let x = f(1,)\n"},
		entry{"t5:call-multiline", "let x = f(1,\n2)\n"},
		entry{"t5:postfix-mix", "let x = a.b(1).c\n"},
		entry{"t5:prop", "let x = g()?\n"},
		entry{"t5:call-generic", "let x = id<Int64>(1)\n"},
		entry{"t5:spec-rollback-eof", "let x = a < b\n"},
		entry{"t5:spec-rollback-brace", "fn f() { if a < b { }\n }\n"},
		entry{"t5:err-member-eof", "let x = a.\n"},
		entry{"t5:err-member-int", "let x = a.1\n"},
		entry{"t5:err-index", "fn f() { let x = a[0]\n }\n"},
		entry{"t5:err-call-eof", "let x = f(1\n"},
		entry{"t5:err-call-tail", "let x = f(1 2)\n"},
		entry{"t5:group-fold", "let x = (1)\n"},
		entry{"t5:unit", "let u = ()\n"},
		entry{"t5:tuple", "let p = (1, 2)\n"},
		entry{"t5:tuple-trailing", "let p = (1, 2,)\n"},
		entry{"t5:err-group-eof", "let x = (1\n"},
		entry{"t5:err-group-tail", "let x = (1 2)\n"},
		entry{"t5:err-tuple-tail", "let x = (1, 2 3)\n"},
		entry{"t5:err-tuple-arity", "let x = (1, 2, 3, 4, 5, 6, 7, 8, 9)\n"},
		entry{"t5:list", "let xs = [1, 2, 3]\n"},
		entry{"t5:list-empty", "let xs = []\n"},
		entry{"t5:err-list-eof", "let xs = [1\n"},
		entry{"t5:err-list-tail", "let xs = [1 2]\n"},
		entry{"t5:construct", "let p = Point { x: 1, y: 2 }\n"},
		entry{"t5:construct-newline", "let p = Point { x: 1\ny: 2 }\n"},
		entry{"t5:construct-with", "let q = Point { x: 1 with & p }\n"},
		entry{"t5:construct-with-only", "let q = Point { with & p }\n"},
		entry{"t5:construct-qual", "let p = std.Point { x: 1 }\n"},
		entry{"t5:construct-generic", "let b = Box<Int64> { v: 1 }\n"},
		entry{"t5:construct-group", "let p = (Point { x: 1 })\n"},
		entry{"t5:err-construct-lower", "let p = point { x: 1 }\n"},
		entry{"t5:err-construct-field-int", "let p = Point { 1: 2 }\n"},
		entry{"t5:err-construct-field-noval", "let p = Point { x }\n"},
		entry{"t5:err-construct-field-tail", "let p = Point { x: 1 2 }\n"},
		entry{"t5:err-construct-with-nobase", "let p = Point { x: 1 with x }\n"},
		entry{"t5:closure-full", "let f = fn(x: Int64) -> Int64 { return x\n }\n"},
		entry{"t5:closure-full-noret", "let f = fn(x: Int64) { g\n }\n"},
		entry{"t5:closure-short", "let f = |x| x + 1\n"},
		entry{"t5:closure-short-typed", "let f = |x: Int64, y: Int64| x\n"},
		entry{"t5:closure-operand-paren", "let f = 1 + (|x| x)\n"},
		entry{"t5:err-closure-operand", "let f = 1 + |x| x\n"},
		entry{"t5:err-short-zero", "let f = || x\n"},
		entry{"t5:err-fn-expr-eof", "let f = fn\n"},
		entry{"t5:err-fn-expr-tail", "let f = fn x\n"},
		entry{"t5:err-short-param-eof", "let f = |x\n"},
		entry{"t5:err-short-param-int", "let f = |1| x\n"},
		entry{"t5:err-short-param-pascal", "let f = |X| x\n"},
		entry{"t5:blockexpr", "let x = { 1\n }\n"},
		entry{"t5:task", "fn f() -> Int64 { let x = task effect io { return 1\n }\n }\n"},
		entry{"t5:err-task-noeffect", "let x = task { }\n"},
		entry{"t5:scope-bare", "let x = scope { 1\n }\n"},
		entry{"t5:scope-timeout", "let x = scope timeout(100) { 1\n }\n"},
		entry{"t5:scope-timeout-collect", "let x = scope timeout(100) collectAll { 1\n }\n"},
		entry{"t5:err-scope-resource-value", "let x = scope resource(h = g) { }\n"},
		entry{"t5:err-scope-timeout-noparen", "let x = scope timeout 100 { }\n"},
		entry{"t5:select", "let x = select { case a = f => a\ncase b = g => b\n }\n"},
		entry{"t5:select-wild", "let x = select { case _ = f => 1\ncase b = g => b\n }\n"},
		entry{"t5:err-select-one", "let x = select { case a = f => a\n }\n"},
		entry{"t5:err-select-case-pattern", "let x = select { case 1 = f => 1\ncase b = g => b\n }\n"},
		entry{"t5:err-select-nosep", "let x = select { case a = f => a case b = g => b\n }\n"},
		entry{"t5:err-valueless-while", "let x = while c { }\n"},
		entry{"t5:err-valueless-loop", "let x = loop { }\n"},
		entry{"t5:err-valueless-for", "let x = for y in ys { }\n"},
		entry{"t5:err-valueless-defer", "let x = defer { }\n"},
		entry{"t5:err-valueless-break", "let x = break\n"},
		entry{"t5:err-valueless-return", "fn f() -> Int64 { return while c { }\n }\n"},
		entry{"t5:err-shortclosure-break", "let f = |x| break\n"},
		entry{"t5:err-keyword-expr", "let x = import\n"},
		entry{"t5:err-rparen-begin", "let x = )\n"},
		entry{"t5:deep-args-binary", "let x = f(1 + 2, g(3 * 4))\n"},
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

// firstDiff reports the first byte offset where want and got disagree
// (their common-prefix length when one is a prefix of the other).
func firstDiff(want, got string) int {
	n := min(len(want), len(got))
	for i := range n {
		if want[i] != got[i] {
			return i
		}
	}
	return n
}

// window returns a short excerpt of s around offset i, so a divergence
// report shows the neighborhood instead of a whole serialized file.
func window(s string, i int) string {
	return s[max(0, i-48):min(len(s), i+48)]
}

// reconciledFloor is the honest ratchet that replaces B3b's stage fence
// (design D5): the count of corpus entries the wec side has reconciled,
// which only rises as faces land — every entry green at a task's close
// stays green, so a regression anywhere in the port drops the count below
// this line and fails the test. It reaches the full corpus when the last
// face lands (T6). B3b's lexical-only TestLexReconciliation is retired
// into this test: the lexical comparison is the parse-section-empty prefix
// of the full output, so asserting both would assert a fraction twice.
const reconciledFloor = 1356

// TestOutputReconciliation is the B3c differential bar (design D5): for
// every admitted corpus entry, the wec artifact's FULL serialized output —
// lexical sections plus the parse section this change appends — must equal
// the live reference byte for byte, the reference being internal/lex's
// serialization followed by internal/parser's three-return dispatch through
// the AST serializer. There is no stage fence: every entry runs from T1 on,
// entries whose parse faces have not landed diverge (logged, with the first
// differing byte's neighborhood, for the face-by-face record), and the
// floor constant is the assertion — the count of entries ever reconciled,
// which only rises. The single remaining exclusion is the
// thirteen-unnameable-byte corner (follow-up #30), disclosed per entry.
func TestOutputReconciliation(t *testing.T) {
	bin, dir := buildWec(t)
	ran, reconciled := 0, 0
	for _, e := range loadCorpus(t) {
		_, _, d := lex.Scan("wec-input.we", []byte(e.src))
		if d != nil && d.Code() == "E0001" && strings.Contains(d.Message(), "is not valid UTF-8") && !firstBadByteNameable(e.src) {
			t.Logf("corpus: %s excluded — its first bad byte is one of the thirteen unnameable values (follow-up #30)", e.name)
			continue
		}
		ran++
		got := runWec(t, bin, dir, e.src)
		want := serializeOutput([]byte(e.src))
		if !bytes.Equal([]byte(got), []byte(want)) {
			// A divergence is recorded, not failed: with the stage fence
			// gone every entry runs from T1 on, so the entries whose parse
			// faces have not landed yet diverge by design and the count
			// shrinks as the tasks close. The floor below is what fails —
			// a face that was green regressing drops the reconciled count
			// under it — and at the change's close the floor is the full
			// corpus, so nothing diverges silently at any point.
			i := firstDiff(want, got)
			t.Logf("%s: first divergence at byte %d\nwant: %q\ngot:  %q", e.name, i, window(want, i), window(got, i))
			continue
		}
		reconciled++
	}
	if ran == 0 {
		t.Fatal("the corpus admitted no entries — the extractor is broken")
	}
	t.Logf("reconciled %d of %d corpus entries (full-output reconciliation; subsumes B3b's retired TestLexReconciliation)", reconciled, ran)
	if reconciled < reconciledFloor {
		t.Errorf("reconciled %d of %d entries — below the floor %d; a face that was green has regressed (the floor only rises as tasks land)", reconciled, ran, reconciledFloor)
	}
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
