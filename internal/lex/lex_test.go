package lex

import (
	"strings"
	"testing"
)

// lexMust lexes src expecting success and returns the token stream
// without the trailing eof token.
func lexMust(t *testing.T, src string) []Token {
	t.Helper()
	toks, d := File("test.we", []byte(src))
	if d != nil {
		t.Fatalf("unexpected diagnostic: %s", d.Human())
	}
	if toks == nil || len(toks) == 0 || toks[len(toks)-1].Kind != KindEOF {
		t.Fatalf("stream must end in an eof token, got %v", toks)
	}
	return toks[:len(toks)-1]
}

// lexFail lexes src expecting exactly one diagnostic and checks code,
// position, and a message fragment.
func lexFail(t *testing.T, src string, code string, line, col int, msgPart string) {
	t.Helper()
	toks, d := File("test.we", []byte(src))
	if d == nil {
		t.Fatalf("expected %s, got a clean stream: %v", code, toks)
	}
	if toks != nil {
		t.Fatalf("an error must produce no token stream, got %v", toks)
	}
	if d.Code() != code {
		t.Fatalf("expected code %s, got %s (%s)", code, d.Code(), d.Message())
	}
	if !strings.Contains(d.Message(), msgPart) {
		t.Fatalf("message %q missing %q", d.Message(), msgPart)
	}
	pos := `"line":` + itoa(line) + `,"column":` + itoa(col)
	if !strings.Contains(d.JSON(), pos) {
		t.Fatalf("expected position %d:%d, got %s", line, col, d.JSON())
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestTokenStream pins the token inventory, classification, and positions
// over one kitchen-sink source.
func TestTokenStream(t *testing.T) {
	src := "fn let_ok 42 3.14\n  records 0x1F 'z' \"s\"\n-> => .. == != <= >= << >> && ||\n"
	got := lexMust(t, src)
	want := []Token{
		{Kind: KindKeyword, Text: "fn", Line: 1, Col: 1},
		{Kind: KindIdent, Text: "let_ok", Line: 1, Col: 4},
		{Kind: KindInt, Text: "42", Line: 1, Col: 11},
		{Kind: KindFloat, Text: "3.14", Line: 1, Col: 14},
		{Kind: KindIdent, Text: "records", Line: 2, Col: 3}, // near-miss of a keyword
		{Kind: KindInt, Text: "0x1F", Line: 2, Col: 11},
		{Kind: KindRune, Text: "'z'", Line: 2, Col: 16},
		{Kind: KindString, Text: `"s"`, Line: 2, Col: 20},
		{Kind: "->", Text: "->", Line: 3, Col: 1},
		{Kind: "=>", Text: "=>", Line: 3, Col: 4},
		{Kind: "..", Text: "..", Line: 3, Col: 7},
		{Kind: "==", Text: "==", Line: 3, Col: 10},
		{Kind: "!=", Text: "!=", Line: 3, Col: 13},
		{Kind: "<=", Text: "<=", Line: 3, Col: 16},
		{Kind: ">=", Text: ">=", Line: 3, Col: 19},
		{Kind: "<<", Text: "<<", Line: 3, Col: 22},
		{Kind: ">>", Text: ">>", Line: 3, Col: 25},
		{Kind: "&&", Text: "&&", Line: 3, Col: 28},
		{Kind: "||", Text: "||", Line: 3, Col: 31},
	}
	if len(got) != len(want) {
		t.Fatalf("token count %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestKeywordSet pins the reserved set against chapter 1's enumerated block:
// every listed word is a keyword, the map holds nothing else, and near-misses
// stay identifiers (case sensitivity is total).
func TestKeywordSet(t *testing.T) {
	spec := []string{
		"fn", "let", "var", "pub", "import", "as", "mut",
		"if", "else", "return", "match", "for", "in", "while", "loop", "break", "continue", "defer",
		"true", "false", "foreign",
		"record", "byval", "byres", "newtype", "with",
		"type",
		"interface", "impl", "where", "derives",
		"scope", "resource",
		"effect",
		"task", "select", "case", "timeout", "collectAll",
		"test", "mock",
	}
	if len(keywords) != len(spec) {
		t.Fatalf("keyword map holds %d words, chapter 1 enumerates %d", len(keywords), len(spec))
	}
	for _, w := range spec {
		if !keywords[w] {
			t.Errorf("%q must be reserved", w)
		}
	}
	for _, w := range []string{"records", "types", "Type", "CollectAll", "collectall", "effectful"} {
		tok := lexMust(t, w)
		if len(tok) != 1 || tok[0].Kind != KindIdent {
			t.Errorf("%q must lex as one identifier, got %+v", w, tok)
		}
	}
	// The bare underscore is punctuation (chapter 1's inventory), `_x` an identifier.
	toks := lexMust(t, "_ _x")
	if toks[0].Kind != "_" || toks[1].Kind != KindIdent || toks[1].Text != "_x" {
		t.Fatalf("underscore classification: %+v", toks)
	}
}

// TestNumericValid pins every ratified form.
func TestNumericValid(t *testing.T) {
	cases := []struct {
		src  string
		kind string
	}{
		{"123", KindInt}, {"0", KindInt}, {"1_000", KindInt}, {"1_000_000", KindInt},
		{"0x1F", KindInt}, {"0xF_F", KindInt}, {"0xFFu8", KindInt}, {"0o17", KindInt}, {"0b1010_0001", KindInt},
		{"42i8", KindInt}, {"0u8", KindInt}, {"1i64", KindInt},
		{"3.14", KindFloat}, {"0.5", KindFloat}, {"3.14f32", KindFloat}, {"1.5e3", KindFloat},
		{"1.5E-3", KindFloat}, {"1.5e+3", KindFloat}, {"1.5e1_0f64", KindFloat},
	}
	for _, c := range cases {
		toks := lexMust(t, c.src)
		if len(toks) != 1 || toks[0].Kind != c.kind || toks[0].Text != c.src {
			t.Errorf("%q: got %+v, want one %s", c.src, toks, c.kind)
		}
	}
}

// TestNumericInvalid pins each E0006 clause with chapter 1's own failure
// shapes. `1.5.3` lands on the suffix clause as the leftover after one
// complete form.
func TestNumericInvalid(t *testing.T) {
	cases := []struct {
		src     string
		msgPart string
	}{
		{"1__0", clauseSep},
		{"100_", clauseSep},
		{"0x_FF", clauseSep},
		{"100_i64", clauseSep},
		{"1_.5", clauseSep},
		{"1.5_", clauseSep},
		{"0x", clausePrefix},
		{"0b2", clauseBaseDig},
		{"0b12", clauseBaseDig},
		{"0o8", clauseBaseDig},
		{"0xG1", clauseBaseDig},
		{"01", clauseLeadZero},
		{"0_1", clauseLeadZero},
		{"00.5", clauseLeadZero},
		{"1.", clauseDotSides},
		{"1.x", clauseDotSides},
		{"1.e5", clauseDotSides},
		{"1._5", clauseDotSides},
		{"1.5.3", clauseSuffix},
		{"0x1.5", clauseBased},
		{"1.5e", clauseExponent},
		{"1.5e+", clauseExponent},
		{"42int", clauseSuffix},
		{"42f32", clauseSuffix}, // a float suffix on an integer form
		{"3.14i8", clauseSuffix},
		{"1.5e3x", clauseSuffix},
	}
	for _, c := range cases {
		lexFail(t, c.src, "E0006", 1, 1, c.msgPart)
	}
}

// TestNumericSplit pins the maximal-munch interactions chapter 1 fixes:
// the range token never merges with numerals, and an undotted `1e5` is an
// integer followed by an identifier (no dot means no float attempt, and a
// tail with non-letters re-lexes rather than failing as a suffix).
func TestNumericSplit(t *testing.T) {
	cases := []struct {
		src  string
		want []Token
	}{
		{"0..9", []Token{
			{Kind: KindInt, Text: "0", Line: 1, Col: 1},
			{Kind: "..", Text: "..", Line: 1, Col: 2},
			{Kind: KindInt, Text: "9", Line: 1, Col: 4},
		}},
		{"1.5..2.5", []Token{
			{Kind: KindFloat, Text: "1.5", Line: 1, Col: 1},
			{Kind: "..", Text: "..", Line: 1, Col: 4},
			{Kind: KindFloat, Text: "2.5", Line: 1, Col: 6},
		}},
		{"1e5", []Token{
			{Kind: KindInt, Text: "1", Line: 1, Col: 1},
			{Kind: KindIdent, Text: "e5", Line: 1, Col: 2},
		}},
	}
	for _, c := range cases {
		got := lexMust(t, c.src)
		if len(got) != len(c.want) {
			t.Fatalf("%q: got %+v, want %+v", c.src, got, c.want)
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Fatalf("%q token %d: got %+v, want %+v", c.src, i, got[i], c.want[i])
			}
		}
	}
}

// TestStrings pins the closed escape set, interpolation, and the E0002 /
// E0005 / E0007 failures.
func TestStrings(t *testing.T) {
	valid := []string{
		`"plain"`,
		`"with $ alone"`,
		`"esc \n\t\r\0 \\ \" \' ok"`,
		`"\u{41}\u{1F600}"`,
		`"interp ${x + 1} tail"`,
		`"nested ${m("x")}"`,
		`"braces ${ {"k": 1} } inside"`,
		`"bare } brace"`,
		`"中文 freely"`,
		`"rune in hole ${f('a')}"`,
	}
	for _, src := range valid {
		toks := lexMust(t, src)
		if len(toks) != 1 || toks[0].Kind != KindString || toks[0].Text != src {
			t.Errorf("%q must lex as one string token, got %+v", src, toks)
		}
	}
	lexFail(t, `"abc`, "E0002", 1, 1, "not closed before the end of the line")
	lexFail(t, "let s = \"abc\n", "E0002", 1, 9, "not closed")
	lexFail(t, `"\e"`, "E0005", 1, 2, `"\e" is not in the closed set`) // at the backslash opening the escape
	lexFail(t, `"\x41"`, "E0005", 1, 2, "closed set")
	lexFail(t, `"\u{GG}"`, "E0005", 1, 2, "1-6 hex digits")
	lexFail(t, `"\u{110000}"`, "E0005", 1, 2, "at most U+10FFFF")
	lexFail(t, `"\u{D800}"`, "E0005", 1, 2, "no surrogates")
	// Chapter 1's own scenario: the quote inside the hole cannot close a
	// nested string, so it is the outer literal's own close with the hole
	// still open — E0007 at the opening ${.
	lexFail(t, `"${f(}"`, "E0007", 1, 2, "hole is not closed inside the literal")
	// The same rule one level deeper: the inner hole's own close attempt
	// fails first, so E0007 names the inner ${ at column 9.
	lexFail(t, `"${ {} "${ x"`, "E0007", 1, 9, "hole is not closed inside the literal")
}

// TestRunes pins the one-character-or-one-escape rule (the registry folds
// content violations into E0003).
func TestRunes(t *testing.T) {
	for _, src := range []string{`'a'`, `'\n'`, `'\u{1F600}'`, `'中'`} {
		toks := lexMust(t, src)
		if len(toks) != 1 || toks[0].Kind != KindRune || toks[0].Text != src {
			t.Errorf("%q must lex as one rune token, got %+v", src, toks)
		}
	}
	lexFail(t, `'a`, "E0003", 1, 1, "not closed before the end of the line")
	lexFail(t, `''`, "E0003", 1, 1, "exactly one character or one escape")
	lexFail(t, `'ab'`, "E0003", 1, 1, "exactly one character or one escape")
	lexFail(t, `'\n x'`, "E0003", 1, 1, "exactly one character or one escape")
	lexFail(t, `'\e'`, "E0005", 1, 2, "closed set")
}

// TestComments pins the three forms, non-nesting blocks, and E0004. Doc
// comments (`///`) are a third form of line comment at this stage.
func TestComments(t *testing.T) {
	src := "// line comment 中文\n/// doc comment\nlet x = 1 /* inline /* still same */ + 2\n"
	toks := lexMust(t, src)
	want := []Token{
		{Kind: KindKeyword, Text: "let", Line: 3, Col: 1},
		{Kind: KindIdent, Text: "x", Line: 3, Col: 5},
		{Kind: "=", Text: "=", Line: 3, Col: 7},
		{Kind: KindInt, Text: "1", Line: 3, Col: 9},
		{Kind: "+", Text: "+", Line: 3, Col: 38},
		{Kind: KindInt, Text: "2", Line: 3, Col: 40},
	}
	if len(toks) != len(want) {
		t.Fatalf("got %+v, want %+v", toks, want)
	}
	for i := range want {
		if toks[i] != want[i] {
			t.Fatalf("token %d: got %+v, want %+v", i, toks[i], want[i])
		}
	}
	lexFail(t, "/* never", "E0004", 1, 1, "not closed by */ before end of file")
}

// TestEncoding pins the mechanical normalization and the file-level E0001
// rules: BOM handling, invalid UTF-8, bare CR, CRLF counting. (The BOM in
// these sources is written as an escape — a literal one is illegal inside
// Go source.)
func TestEncoding(t *testing.T) {
	// Leading BOM ignored; CRLF counts as one break; positions hold.
	toks := lexMust(t, "\uFEFFlet x\r\nlet y\r\n")
	want := []Token{
		{Kind: KindKeyword, Text: "let", Line: 1, Col: 1},
		{Kind: KindIdent, Text: "x", Line: 1, Col: 5},
		{Kind: KindKeyword, Text: "let", Line: 2, Col: 1},
		{Kind: KindIdent, Text: "y", Line: 2, Col: 5},
	}
	if len(toks) != len(want) {
		t.Fatalf("got %+v, want %+v", toks, want)
	}
	for i := range want {
		if toks[i] != want[i] {
			t.Fatalf("token %d: got %+v, want %+v", i, toks[i], want[i])
		}
	}
	lexFail(t, "a\uFEFFb", "E0001", 1, 2, "byte-order mark")
	lexFail(t, "let a = \"\xff\"", "E0001", 1, 10, "not valid UTF-8")
	lexFail(t, "let a = 1\rlet b = 2", "E0001", 1, 10, "bare carriage return")
}

// TestAttr pins the attribute unit: form violations (E0001), the
// literal-only argument rule (E0009), and the E0009-before-E0008 order
// (with an empty whitelist, E0008-first would leave E0009 unreachable).
func TestAttr(t *testing.T) {
	lexFail(t, "# [x]", "E0001", 1, 1, `"#" must be followed by "["`)
	lexFail(t, "#", "E0001", 1, 1, `"#" must be followed by "["`)
	lexFail(t, "#[", "E0001", 1, 3, `expects a name`)
	lexFail(t, "#[x", "E0001", 1, 4, `then "]"`) // end of input before the unit closes
	lexFail(t, "#[type]", "E0001", 1, 3, `expects a name`)
	lexFail(t, "#[x(]", "E0001", 1, 5, `expects a name, optional (arguments), then "]"`)
	lexFail(t, "#[x\n]", "E0001", 1, 4, `then "]"`) // the unit must close on one line
	lexFail(t, "#[inline]", "E0008", 1, 1, `unknown attribute — "inline" is not in the closed whitelist`)
	// `limit`, not a reserved word: a keyword name is the form error pinned
	// above (`#[type]`).
	lexFail(t, "#[limit(5000 + n)]", "E0009", 1, 9, `"5000 + n": arguments are compile-time literals only`)
	lexFail(t, `#[cfg("x", 1, true, [1, 2.5])]`, "E0008", 1, 1, "closed whitelist")
	lexFail(t, `#[cfg(f(n))]`, "E0009", 1, 7, `"f(n)"`)
	lexFail(t, `#[cfg("${x}")]`, "E0009", 1, 7, "compile-time literals only")
	lexFail(t, `#[cfg(1e5)]`, "E0009", 1, 7, "compile-time literals only")
	lexFail(t, `#[cfg(1__0)]`, "E0006", 1, 7, clauseSep)
	lexFail(t, "#[cfg ()]", "E0008", 1, 1, "closed whitelist") // horiz ws tolerated

	// The token shape the parser milestone consumes — name and raw args
	// riding one token — is observable only with a whitelisted name (the
	// real list is empty until a chapter ratifies one), so the test lends
	// the map an entry and restores it.
	attrWhitelist["cfg"] = true
	defer delete(attrWhitelist, "cfg")
	toks := lexMust(t, "#[cfg  (  1 , \"s\" )]\n")
	if len(toks) != 1 || toks[0].Kind != KindAttr || toks[0].AttrName != "cfg" ||
		toks[0].AttrArgs != `1 , "s"` || toks[0].Text != `#[cfg  (  1 , "s" )]` {
		t.Fatalf("attr token shape: %+v", toks)
	}
}

// TestFirstErrorStop pins that the first lexical error wins and no token
// stream is produced.
func TestFirstErrorStop(t *testing.T) {
	toks, d := File("test.we", []byte("let $ x = @ y"))
	if d == nil || d.Code() != "E0001" || toks != nil {
		t.Fatalf("expected the first E0001 and no stream, got %+v / %v", toks, d)
	}
	if !strings.Contains(d.Message(), `"$"`) {
		t.Fatalf("the first offender must be named, got %q", d.Message())
	}
}

// TestDocUnits pins the /// side channel: consecutive /// lines form one
// unit, blank lines and ordinary comments break the run, one leading space
// of text is trimmed, and — with no /// present — Scan returns no units
// while File stays byte-for-byte equivalent over the same input.
func TestDocUnits(t *testing.T) {
	src := "/// One unit: first line.\n" +
		"/// Second line.\n" +
		"\n" +
		"/// New unit after a blank line.\n" +
		"fn f() {}\n" +
		"// ordinary comment breaks\n" +
		"/// Third unit.\n" +
		"/* block */ /// the block comment breaks: new unit.\n"
	toks, docs, d := Scan("test.we", []byte(src))
	if d != nil {
		t.Fatalf("unexpected diagnostic: %s", d.Human())
	}
	if toks == nil || toks[len(toks)-1].Kind != KindEOF {
		t.Fatalf("stream must end in eof, got %v", toks)
	}
	if len(toks) != 7 { // fn f ( ) { } + eof
		t.Fatalf("token count %d, want 7: %+v", len(toks), toks)
	}
	want := []DocUnit{
		{StartLine: 1, EndLine: 2, Lines: []string{"One unit: first line.", "Second line."}},
		{StartLine: 4, EndLine: 4, Lines: []string{"New unit after a blank line."}},
		{StartLine: 7, EndLine: 7, Lines: []string{"Third unit."}},
		{StartLine: 8, EndLine: 8, Lines: []string{"the block comment breaks: new unit."}},
	}
	if len(docs) != len(want) {
		t.Fatalf("unit count %d, want %d: %+v", len(docs), len(want), docs)
	}
	for i := range want {
		if docs[i].StartLine != want[i].StartLine || docs[i].EndLine != want[i].EndLine ||
			strings.Join(docs[i].Lines, "|") != strings.Join(want[i].Lines, "|") {
			t.Fatalf("unit %d: got %+v, want %+v", i, docs[i], want[i])
		}
	}

	toks2, d2 := File("test.we", []byte(src))
	if d2 != nil || len(toks2) != len(toks) {
		t.Fatalf("File must remain equivalent over the same input")
	}
	toks3, docs3, d3 := Scan("test.we", []byte("let x = 1\n// note\n"))
	if d3 != nil || docs3 != nil || len(toks3) != 5 { // let x = 1 + eof
		t.Fatalf("no /// means no units: toks=%v docs=%v d=%v", toks3, docs3, d3)
	}
}
