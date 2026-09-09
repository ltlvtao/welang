// Package lex implements the lexical stage of chapter 1
// (docs/spec/0100-lexical.md): the closed token inventory, maximal munch,
// identifiers and the fully-reserved keyword set, numeric/string/rune
// literals, comments, the mechanical BOM and line-break normalization, and
// the attribute unit. Lexing is context-free — zero soft keywords, zero
// contextual interpretations — and stops at the first lexical error:
// chapter 1 fixes that an error produces no token stream, and chapter 21
// that an E-severity diagnostic stops the pipeline.
package lex

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ltlvtao/welang/internal/diag"
)

// Token kinds for words and literals. An operator or punctuation token's
// kind is its own text ("+", "==", "..", "#[", "_") — the closed inventory
// of chapter 1's Operators and punctuation requirement is the kind set.
const (
	KindIdent   = "ident"
	KindKeyword = "keyword"
	KindInt     = "int"
	KindFloat   = "float"
	KindString  = "string"
	KindRune    = "rune"
	KindAttr    = "attr"
	KindComment = "comment"
	KindEOF     = "eof"
)

// keywords is chapter 1's fully-reserved closed set: a keyword token is
// never an identifier, in any syntactic position. Adding a word is a
// spec-layer change that amends the chapter's list.
var keywords = map[string]bool{
	"fn": true, "let": true, "var": true, "pub": true, "import": true, "as": true, "mut": true,
	"if": true, "else": true, "return": true, "match": true, "for": true, "in": true,
	"while": true, "loop": true, "break": true, "continue": true, "defer": true,
	"true": true, "false": true, "foreign": true,
	"record": true, "byval": true, "byres": true, "newtype": true, "with": true,
	"type":      true,
	"interface": true, "impl": true, "where": true, "derives": true,
	"scope": true, "resource": true,
	"effect": true,
	"task":   true, "select": true, "case": true, "timeout": true, "collectAll": true,
	"test": true, "mock": true,
}

// attrWhitelist is the closed attribute namespace assembled from ratified
// chapters. No chapter has ratified an attribute yet — a grep of docs/spec
// finds `#[` only in chapter 1's own definition and chapter 2's
// line-joining note — so the set is empty and every attribute name reports
// E0008 until a spec-layer change amends it.
var attrWhitelist = map[string]bool{}

// helps carries each code's remediation, compressed from its registry
// entry (docs/spec/diagnostics.toml). One home until the registry embed
// lands (roadmap follow-up 2).
var helps = map[string]string{
	"E0001": "Use an ASCII letter, digit, or underscore in identifiers; non-ASCII text belongs in comments and literals.",
	"E0002": `Close the literal with " on the same line; multiline strings do not exist in We.`,
	"E0003": "Close the literal with ' on the same line, holding exactly one character or one escape.",
	"E0004": "Add the closing */; block comments do not nest — the first */ closes.",
	"E0005": `Permitted escapes: \n \t \r \0 \\ \" \' \u{h..} (1-6 hex digits).`,
	"E0006": "_ only between two digits; no leading decimal zero; a digit on each side of the dot; suffixes i8-u64, f32, f64.",
	"E0007": "Balance the braces inside the ${...} hole; a $ not followed by { is an ordinary character.",
	"E0008": "Remove the attribute or check its spelling; names join the whitelist only through spec changes.",
	"E0009": "Replace the argument with a compile-time literal (string, number, boolean, literal list).",
}

// attrFormMsg is the canonical E0001 detail for a malformed attribute
// unit (form violations fire before the E0009/E0008 checks).
const attrFormMsg = `invalid character — the attribute unit expects a name, optional (arguments), then "]" on one line`

// Token is one lexical unit. Line and Col are 1-based; Col counts runes
// since the line start (chapter 1 fixes no unit — this is the toolchain's
// mechanism, recorded in the lexical change's design).
type Token struct {
	Kind string
	Text string
	Line int
	Col  int

	// AttrName and AttrArgs are set on attr tokens only: the attribute is
	// one lexical unit, so its name and raw argument text ride the token
	// instead of a sub-token stream. AttrArgs is the text between the
	// parentheses, trimmed, empty when there are no arguments.
	AttrName string
	AttrArgs string
}

// DocUnit is one /// documentation unit (chapter 6): a maximal run of
// consecutive /// lines. StartLine and EndLine are the unit's 1-based line
// span; Lines holds each line's text after the /// (one leading space
// trimmed). Units are collected as a side channel — comments still produce
// no tokens (chapter 1) — and attachment is the parser's to resolve.
type DocUnit struct {
	StartLine int
	EndLine   int
	Lines     []string
}

// stop unwinds the scan at the first lexical error (first-error-stop,
// chapter 1: no token stream is produced).
type stop struct{ d diag.Diagnostic }

// lexer holds the scan state. col is the 1-based rune column of the next
// unconsumed rune; a line break resets it. docs/docOpen track the ///
// side channel: docOpen indexes the unit that consecutive /// lines still
// extend, -1 when a blank line or ordinary comment has broken the run.
// keep switches the comment branches of the main loop from skipping to
// emitting: comments surface as KindComment trivia tokens (the formatter's
// face, M11) instead of vanishing.
type lexer struct {
	name string
	src  string
	off  int
	line int
	col  int

	docs    []DocUnit
	docOpen int
	docLast int
	keep    bool
}

// File lexes one source file. It returns the token stream (ending in an
// eof token) and nil on success, or nil and the first diagnostic on any
// lexical error. name is carried verbatim into diagnostics. Callers that
// also need the /// documentation units use Scan.
func File(name string, src []byte) (toks []Token, first *diag.Diagnostic) {
	toks, _, first = Scan(name, src)
	return toks, first
}

// Scan lexes one source file, returning the token stream (ending in an
// eof token) and the /// documentation units alongside it, or the first
// lexical diagnostic with no tokens and no units.
func Scan(name string, src []byte) (toks []Token, docs []DocUnit, first *diag.Diagnostic) {
	toks, docs, first = run(name, src, false)
	return toks, docs, first
}

// Keep lexes one source file with comments kept as trivia: each //, ///,
// or /* */ comment surfaces as a KindComment token (text verbatim, slashes
// included, the line break excluded) at its own position, and every other
// token is exactly the face Scan produces. Chapter 1 fixes that comments
// produce no tokens on the compile pipeline's face; this second entry is
// the formatter's (M11 design D1) — one scanner, two faces.
func Keep(name string, src []byte) (toks []Token, first *diag.Diagnostic) {
	toks, _, first = run(name, src, true)
	return toks, first
}

// run is the shared scan body under both faces.
func run(name string, src []byte, keep bool) (toks []Token, docs []DocUnit, first *diag.Diagnostic) {
	l := &lexer{name: name, src: string(src), line: 1, col: 1, docOpen: -1, keep: keep}
	if d := l.checkEncoding(); d != nil {
		return nil, nil, d
	}
	defer func() {
		if r := recover(); r != nil {
			s, ok := r.(stop)
			if !ok {
				panic(r)
			}
			toks, docs, first = nil, nil, &s.d
		}
	}()
	toks = l.scanAll()
	return toks, l.docs, nil
}

// fail reports one lexical diagnostic and stops the scan. The message
// starts with the code's registry title.
func (l *lexer) fail(line, col int, code, message string) {
	d := diag.Error(code, message).At(l.name, line, col)
	if h, ok := helps[code]; ok {
		d = d.WithHelp(h)
	}
	panic(stop{d})
}

// failAt is fail with a position computed from a byte offset.
func (l *lexer) failAt(off int, code, message string) {
	line, col := l.posAt(off)
	l.fail(line, col, code, message)
}

// posAt computes the 1-based line and rune column of the rune at byte
// offset off (error paths only — it walks from the start).
func (l *lexer) posAt(off int) (line, col int) {
	line, col = 1, 1
	for i, r := range l.src {
		if i >= off {
			break
		}
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return line, col
}

func (l *lexer) pos() (int, int) { return l.line, l.col }

func (l *lexer) eof() bool { return l.off >= len(l.src) }

// peek returns the current byte; call only when !eof.
func (l *lexer) peek() byte { return l.src[l.off] }

// peek2 returns the byte after the current one, 0 at end of input.
func (l *lexer) peek2() byte {
	if l.off+1 < len(l.src) {
		return l.src[l.off+1]
	}
	return 0
}

// advance consumes one rune, maintaining the line and column.
func (l *lexer) advance() rune {
	r, size := utf8.DecodeRuneInString(l.src[l.off:])
	l.off += size
	if r == '\n' {
		l.line, l.col = l.line+1, 1
	} else {
		l.col++
	}
	return r
}

// checkEncoding enforces chapter 1's Source files requirement before any
// token scanning: the whole file is valid UTF-8, a byte-order mark is
// permitted only as a single leading one, and CR appears only as part of
// CRLF. On success the leading BOM is stripped (mechanically ignored).
func (l *lexer) checkEncoding() *diag.Diagnostic {
	if !utf8.ValidString(l.src) {
		off := 0
		for off < len(l.src) {
			r, size := utf8.DecodeRuneInString(l.src[off:])
			if r == utf8.RuneError && size == 1 {
				break
			}
			off += size
		}
		line, col := l.posAt(off)
		d := diag.Error("E0001",
			fmt.Sprintf("invalid character — byte 0x%02X at offset %d is not valid UTF-8", l.src[off], off)).
			At(l.name, line, col).WithHelp(helps["E0001"])
		return &d
	}
	stripped := strings.TrimPrefix(l.src, "\uFEFF")
	adjust := len(l.src) - len(stripped)
	if i := strings.Index(stripped, "\uFEFF"); i >= 0 {
		off := i + adjust
		line, col := l.posAt(off)
		d := diag.Error("E0001",
			fmt.Sprintf("invalid character — byte-order mark at offset %d; a single leading U+FEFF is ignored, any other appearance is an error", off)).
			At(l.name, line, col).WithHelp(helps["E0001"])
		return &d
	}
	for i := 0; i < len(stripped); i++ {
		if stripped[i] == '\r' && (i+1 >= len(stripped) || stripped[i+1] != '\n') {
			line, col := l.posAt(i)
			d := diag.Error("E0001",
				fmt.Sprintf("invalid character — bare carriage return at offset %d; line breaks are LF and CRLF counts as one break", i)).
				At(l.name, line, col).WithHelp(helps["E0001"])
			return &d
		}
	}
	l.src = stripped
	return nil
}

// scanAll is the main token loop: skip whitespace and comments (both only
// separate tokens), then scan one token, until end of input.
func (l *lexer) scanAll() []Token {
	var toks []Token
	for {
		for !l.eof() {
			c := l.peek()
			switch {
			case c == ' ' || c == '\t' || c == '\n' || c == '\r':
				l.advance()
			case c == '/' && l.peek2() == '/':
				if l.keep {
					toks = append(toks, l.keepLineComment())
				} else {
					l.skipLineComment()
				}
			case c == '/' && l.peek2() == '*':
				if l.keep {
					toks = append(toks, l.keepBlockComment())
				} else {
					l.docOpen = -1
					l.skipBlockComment()
				}
			default:
				goto token
			}
		}
	token:
		if l.eof() {
			line, col := l.pos()
			return append(toks, Token{Kind: KindEOF, Line: line, Col: col})
		}
		var t Token
		switch c := l.peek(); {
		case c == '"':
			t = l.scanStringToken()
		case c == '\'':
			t = l.scanRuneToken()
		case c == '#':
			t = l.scanAttr()
		case isDigit(c):
			t = l.scanNumber()
		case isIdentStart(c):
			t = l.scanWord()
		default:
			t = l.scanOperator() // '/' not starting a comment lands here too
		}
		toks = append(toks, t)
	}
}

// skipLineComment consumes one // or /// comment to the end of the line. A
// /// comment feeds the documentation side channel: consecutive /// lines
// extend one DocUnit; an ordinary // comment breaks the run.
func (l *lexer) skipLineComment() {
	start := l.off
	line := l.line
	for !l.eof() && l.peek() != '\n' {
		l.advance()
	}
	text := strings.TrimRight(l.src[start:l.off], "\r")
	switch {
	case strings.HasPrefix(text, "///"):
		text = strings.TrimPrefix(text[3:], " ")
		if l.docOpen >= 0 && l.docLast == line-1 {
			u := &l.docs[l.docOpen]
			u.Lines = append(u.Lines, text)
			u.EndLine = line
		} else {
			l.docs = append(l.docs, DocUnit{StartLine: line, EndLine: line, Lines: []string{text}})
			l.docOpen = len(l.docs) - 1
		}
		l.docLast = line
	default:
		l.docOpen = -1
	}
}

// keepLineComment consumes one // or /// comment and returns it as a
// KindComment token: text verbatim through the slashes, the line break
// (and its CR) excluded. The /// side channel stays off in keep mode —
// attachment semantics belong to the parser's face.
func (l *lexer) keepLineComment() Token {
	line, col := l.pos()
	start := l.off
	for !l.eof() && l.peek() != '\n' {
		l.advance()
	}
	return Token{Kind: KindComment, Text: strings.TrimRight(l.src[start:l.off], "\r"), Line: line, Col: col}
}

// keepBlockComment consumes one /* */ comment and returns it as one
// KindComment token, text verbatim (its line span rides the opening line).
func (l *lexer) keepBlockComment() Token {
	line, col := l.pos()
	start := l.off
	l.skipBlockComment()
	return Token{Kind: KindComment, Text: l.src[start:l.off], Line: line, Col: col}
}

// skipBlockComment consumes one /* */ comment. Block comments do not nest:
// the first */ closes the comment, an inner /* is ordinary text.
func (l *lexer) skipBlockComment() {
	sl, sc := l.pos()
	l.advance() // '/'
	l.advance() // '*'
	for {
		if l.eof() {
			l.fail(sl, sc, "E0004", "unterminated block comment — /* is not closed by */ before end of file")
		}
		if l.peek() == '*' && l.peek2() == '/' {
			l.advance()
			l.advance()
			return
		}
		l.advance()
	}
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isHexDigit(c byte) bool   { return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') }
func isLetter(c byte) bool     { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentStart(c byte) bool { return isLetter(c) || c == '_' }
func isIdentChar(c byte) bool  { return isIdentStart(c) || isDigit(c) }

// scanWord consumes an identifier-shaped run and classifies it: keyword
// (the closed reserved set), the bare underscore (chapter 1 lists `_` as
// punctuation), or identifier.
func (l *lexer) scanWord() Token {
	line, col := l.pos()
	start := l.off
	for !l.eof() && isIdentChar(l.peek()) {
		l.advance()
	}
	text := l.src[start:l.off]
	switch {
	case keywords[text]:
		return Token{Kind: KindKeyword, Text: text, Line: line, Col: col}
	case text == "_":
		return Token{Kind: "_", Text: text, Line: line, Col: col}
	default:
		return Token{Kind: KindIdent, Text: text, Line: line, Col: col}
	}
}

// twoCharOps and oneCharOps are the closed operator and punctuation
// inventory. Maximal munch tries the two-byte forms first.
var twoCharOps = []string{"==", "!=", "<=", ">=", "&&", "||", "<<", ">>", "->", "=>", ".."}

const oneCharOps = "+-*/%<>!&|^~=.,;:?(){}[]"

// scanOperator consumes one operator or punctuation token, or reports
// E0001 for a character that matches nothing in the inventory.
func (l *lexer) scanOperator() Token {
	line, col := l.pos()
	rest := l.src[l.off:]
	for _, op := range twoCharOps {
		if strings.HasPrefix(rest, op) {
			l.off += 2
			l.col += 2
			return Token{Kind: op, Text: op, Line: line, Col: col}
		}
	}
	c := l.peek()
	if strings.IndexByte(oneCharOps, c) >= 0 {
		l.advance()
		return Token{Kind: string(c), Text: string(c), Line: line, Col: col}
	}
	r, _ := utf8.DecodeRuneInString(rest)
	switch {
	case r >= 0x80 && unicode.IsLetter(r):
		l.fail(line, col, "E0001",
			fmt.Sprintf("invalid character — %q in an identifier: identifiers are ASCII-only (homoglyph confusability is a security surface)", string(r)))
	case r >= 0x80:
		l.fail(line, col, "E0001",
			fmt.Sprintf("invalid character — %q matches no token in the lexical inventory; non-ASCII is permitted only inside comments and literals", string(r)))
	default:
		l.fail(line, col, "E0001",
			fmt.Sprintf("invalid character — %q matches no token in the lexical inventory", string(r)))
	}
	panic("unreachable")
}

// scanStringToken consumes one string literal token, validating escapes
// and interpolation balance (E0002/E0005/E0007).
func (l *lexer) scanStringToken() Token {
	line, col := l.pos()
	start := l.off
	l.consumeStringLit(false)
	return Token{Kind: KindString, Text: l.src[start:l.off], Line: line, Col: col}
}

// consumeStringLit consumes one string literal starting at its opening
// quote. soft=true reports an end-of-line run by returning false with the
// position rewound — used where the quote may be the enclosing literal's
// own close (an interpolation hole that never balanced); otherwise the
// run is E0002 at the opening quote.
func (l *lexer) consumeStringLit(soft bool) bool {
	quoteLine, quoteCol := l.pos()
	l.advance() // opening "
	for {
		if l.eof() || l.peek() == '\n' || l.peek() == '\r' {
			if soft {
				return false
			}
			l.fail(quoteLine, quoteCol, "E0002", "unterminated string literal — not closed before the end of the line")
		}
		c := l.peek()
		switch {
		case c == '"':
			l.advance()
			return true
		case c == '\\':
			if l.peek2() == '\n' || l.peek2() == '\r' || l.off+1 >= len(l.src) {
				// The literal ends with a dangling backslash: it never
				// closed, which is the primary fact.
				if soft {
					return false
				}
				l.fail(quoteLine, quoteCol, "E0002", "unterminated string literal — not closed before the end of the line")
			}
			l.consumeEscape()
		case c == '$' && l.peek2() == '{':
			holeLine, holeCol := l.pos()
			l.advance() // '$'
			l.advance() // '{'
			l.consumeHole(holeLine, holeCol, quoteLine, quoteCol)
		default:
			l.advance()
		}
	}
}

// consumeHole consumes an interpolation hole after "${", stopping after
// the "}" that balances it. A hole is a balanced-brace expression region:
// parentheses and brackets opened inside it hold it open — a "}" inside
// open parens is ordinary region text, which is what makes chapter 1's
// own example "${f(}" unbalanced (the quote then falls inside the still
// open hole). Nested literals inside the hole are consumed with their
// own rules, and their braces do not count. A quote that cannot close a
// nested literal is the enclosing literal's own close — with the hole
// still open that is E0007 at the opening ${.
func (l *lexer) consumeHole(holeLine, holeCol, quoteLine, quoteCol int) {
	depth := 1  // braces; the hole's own ${ counts as one
	parens := 0 // open ( or [ within the hole so far
	for depth > 0 {
		if l.eof() || l.peek() == '\n' || l.peek() == '\r' {
			l.fail(quoteLine, quoteCol, "E0002", "unterminated string literal — not closed before the end of the line")
		}
		switch l.peek() {
		case '{':
			depth++
			l.advance()
		case '}':
			if parens == 0 {
				depth--
			}
			l.advance()
		case '(', '[':
			parens++
			l.advance()
		case ')', ']':
			if parens > 0 {
				parens--
			}
			l.advance()
		case '"':
			if !l.consumeStringLit(true) {
				l.fail(holeLine, holeCol, "E0007", "unbalanced interpolation braces — the ${ hole is not closed inside the literal")
			}
		case '\'':
			l.consumeRuneLit()
		default:
			l.advance()
		}
	}
}

// consumeEscape validates one escape sequence at the current backslash:
// the closed set n t r 0 \ " ' and \u{h..} (1-6 hex digits, value at most
// U+10FFFF, surrogates forbidden). The cursor lands after the escape.
func (l *lexer) consumeEscape() {
	escLine, escCol := l.pos()
	l.advance() // backslash
	if l.eof() {
		// A trailing backslash at end of input: defensive (the string and
		// rune scanners check line ends first), kept total.
		l.fail(escLine, escCol, "E0005", `invalid escape sequence — a trailing "\" is not in the closed set`)
	}
	c := l.peek()
	switch c {
	case 'n', 't', 'r', '0', '\\', '"', '\'':
		l.advance()
		return
	case 'u':
		l.advance()
		if l.peek() != '{' {
			l.fail(escLine, escCol, "E0005", `invalid escape sequence — "\u" wants {h..} with 1-6 hex digits`)
		}
		l.advance() // '{'
		n := 0
		val := uint64(0)
		for !l.eof() && isHexDigit(l.peek()) {
			n++
			val = val*16 + uint64(hexVal(l.peek()))
			l.advance()
		}
		if n < 1 || n > 6 || l.eof() || l.peek() != '}' || val > 0x10FFFF || (val >= 0xD800 && val <= 0xDFFF) {
			l.fail(escLine, escCol, "E0005",
				`invalid escape sequence — "\u{h..}" wants 1-6 hex digits, a value at most U+10FFFF, and no surrogates`)
		}
		l.advance() // '}'
	default:
		r, _ := utf8.DecodeRuneInString(l.src[l.off:])
		// Render the offending escape as source shows it (`"\e"`), with the
		// character itself quoted so control characters stay printable.
		q := strconv.Quote(string(r))
		l.fail(escLine, escCol, "E0005",
			fmt.Sprintf(`invalid escape sequence — "\%s" is not in the closed set`, q[1:len(q)-1]))
	}
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return int(c-'A') + 10
	}
}

// scanRuneToken consumes one rune literal token: exactly one character or
// one escape between single quotes on a single line (E0003/E0005).
func (l *lexer) scanRuneToken() Token {
	line, col := l.pos()
	start := l.off
	l.consumeRuneLit()
	return Token{Kind: KindRune, Text: l.src[start:l.off], Line: line, Col: col}
}

// consumeRuneLit consumes one rune literal starting at its opening quote.
// The registry folds the content rule into E0003: a rune literal holds
// exactly one character or one escape, and it must close on the same line.
func (l *lexer) consumeRuneLit() {
	quoteLine, quoteCol := l.pos()
	l.advance() // opening '
	const unterminated = "unterminated rune literal — not closed before the end of the line"
	const shape = "unterminated rune literal — a rune holds exactly one character or one escape"
	if l.eof() || l.peek() == '\n' || l.peek() == '\r' {
		l.fail(quoteLine, quoteCol, "E0003", unterminated)
	}
	switch {
	case l.peek() == '\\':
		l.consumeEscape()
	case l.peek() == '\'':
		l.fail(quoteLine, quoteCol, "E0003", shape) // '' holds nothing
	default:
		l.advance() // exactly one character
	}
	if l.eof() || l.peek() == '\n' || l.peek() == '\r' {
		l.fail(quoteLine, quoteCol, "E0003", unterminated)
	}
	if l.peek() != '\'' {
		l.fail(quoteLine, quoteCol, "E0003", shape)
	}
	l.advance() // closing '
}

// scanNumber consumes one numeric literal: a greedy numeric-shaped span
// (digits, letters, underscores; a dot unless it opens a `..` token; a
// sign only right after an exponent marker of a dotted decimal form),
// then validates the span against chapter 1's forms. A violation reports
// E0006 at the span start with the clause that failed. One form cuts the
// token short: an undotted decimal followed by an identifier-shaped tail
// (as in `1e5` — no dot means no float attempt) emits the integer and the
// tail re-lexes as a separate identifier token.
func (l *lexer) scanNumber() Token {
	line, col := l.pos()
	start := l.off
	end := numericSpan(l.src, start)
	text := l.src[start:end]
	consume := len(text)
	kind := KindInt

	switch {
	case len(text) >= 2 && text[0] == '0' && (text[1] == 'x' || text[1] == 'o' || text[1] == 'b'):
		if detail := validateBased(text); detail != "" {
			l.fail(line, col, "E0006", fmt.Sprintf("invalid numeric literal — %q: %s", text, detail))
		}
	case strings.IndexByte(text, '.') >= 0:
		kind = KindFloat
		if detail := validateFloat(text); detail != "" {
			l.fail(line, col, "E0006", fmt.Sprintf("invalid numeric literal — %q: %s", text, detail))
		}
	default:
		i := digitRunEnd(text, 0)
		digits, tail := text[:i], text[i:]
		if detail := checkDigitRun(digits, true); detail != "" {
			l.fail(line, col, "E0006", fmt.Sprintf("invalid numeric literal — %q: %s", text, detail))
		}
		switch {
		case tail == "":
		case knownIntSuffix(tail):
			// an integer suffix rides the token
		case knownFloatSuffix(tail) || allLetters(tail):
			// `42f32` (a float suffix on an integer form) and `42int` (a
			// letter-shaped suffix attempt) both violate the closed set
			l.fail(line, col, "E0006", fmt.Sprintf("invalid numeric literal — %q: %s", text, clauseSuffix))
		default:
			consume = len(digits) // cut: the tail re-lexes as an identifier
		}
	}

	l.off = start + consume
	l.col += consume // numeric spans are ASCII
	return Token{Kind: kind, Text: text[:consume], Line: line, Col: col}
}

// numericSpan returns the end offset of the numeric-shaped run starting
// at i: [0-9a-zA-Z_] freely; '.' consumed unless the next byte opens a
// `..` token (chapter 1: a numeric literal's maximal munch never absorbs
// a dot that begins `..`); '+'/'-' consumed only immediately after an
// exponent marker of a dotted decimal form. s[i] must be a digit.
func numericSpan(s string, i int) int {
	dot := false
	for i < len(s) {
		c := s[i]
		switch {
		case isDigit(c) || isLetter(c) || c == '_':
			i++
		case c == '.':
			if i+1 < len(s) && s[i+1] == '.' {
				return i // the range token `..` begins here
			}
			dot = true
			i++
		case c == '+' || c == '-':
			if dot && i > 0 && (s[i-1] == 'e' || s[i-1] == 'E') {
				i++
			} else {
				return i
			}
		default:
			return i
		}
	}
	return i
}

// The E0006 clauses, one string per violated form rule.
const (
	clauseSep      = "_ separates digit groups only between two digits"
	clausePrefix   = "the base prefix wants at least one digit of its base"
	clauseBaseDig  = "only digits of the literal's base may follow its prefix"
	clauseLeadZero = "a decimal literal has no leading zero (0 alone is allowed)"
	clauseDotSides = "a float wants a digit on each side of the dot"
	clauseExponent = "an exponent wants at least one digit"
	clauseSuffix   = "the suffix set is closed (i8 i16 i32 i64 u8 u16 u32 u64, floats f32 f64)"
	clauseBased    = "based literals (0x/0o/0b) are integer forms: no dot, no exponent"
)

var intSuffixes = []string{"i8", "u8", "i16", "u16", "i32", "u32", "i64", "u64"}
var floatSuffixes = []string{"f32", "f64"}

func knownIntSuffix(tail string) bool {
	for _, suf := range intSuffixes {
		if tail == suf {
			return true
		}
	}
	return false
}

// knownFloatSuffix reports whether tail is one of the float suffixes.
func knownFloatSuffix(tail string) bool {
	for _, suf := range floatSuffixes {
		if tail == suf {
			return true
		}
	}
	return false
}

func allLetters(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isLetter(s[i]) {
			return false
		}
	}
	return len(s) > 0
}

// digitRunEnd returns the end of the leading [0-9_]* run of s at i.
func digitRunEnd(s string, i int) int {
	for i < len(s) && (isDigit(s[i]) || s[i] == '_') {
		i++
	}
	return i
}

// checkDigitRun validates one [0-9_]* run: underscores strictly between
// two digits, and — when leadZero is set (integer and float integer
// parts) — no leading decimal zero on the underscore-stripped digits.
func checkDigitRun(run string, leadZero bool) string {
	if leadZero {
		bare := strings.ReplaceAll(run, "_", "")
		if len(bare) > 1 && bare[0] == '0' {
			return clauseLeadZero
		}
	}
	for j := 0; j < len(run); j++ {
		if run[j] == '_' {
			if j == 0 || j == len(run)-1 || !isDigit(run[j-1]) || !isDigit(run[j+1]) {
				return clauseSep
			}
		}
	}
	return ""
}

// validateBased checks 0x/0o/0b forms: integer forms only — digits of the
// base with separators, an optional integer suffix, nothing else.
func validateBased(s string) string {
	base := s[1]
	digitOfBase := func(c byte) bool {
		switch base {
		case 'x':
			return isHexDigit(c)
		case 'o':
			return c >= '0' && c <= '7'
		default:
			return c == '0' || c == '1'
		}
	}
	rest := s[2:]
	if strings.IndexByte(rest, '.') >= 0 {
		return clauseBased
	}
	body := rest
	for _, suf := range intSuffixes {
		if strings.HasSuffix(rest, suf) && len(rest) > len(suf) && digitOfBase(rest[len(rest)-len(suf)-1]) {
			body = rest[:len(rest)-len(suf)]
			break
		}
	}
	if body == "" {
		return clausePrefix
	}
	for j := 0; j < len(body); j++ {
		c := body[j]
		switch {
		case digitOfBase(c):
		case c == '_':
			if j == 0 || j == len(body)-1 || !digitOfBase(body[j-1]) || !digitOfBase(body[j+1]) {
				return clauseSep
			}
		default:
			return clauseBaseDig
		}
	}
	return ""
}

// validateFloat checks dotted decimal forms: digits '.' digits, an
// optional exponent (e/E, optional sign, at least one digit), an optional
// float suffix, and nothing else.
func validateFloat(s string) string {
	body := s
	for _, suf := range floatSuffixes {
		if strings.HasSuffix(body, suf) && len(body) > len(suf) && isDigit(body[len(body)-len(suf)-1]) {
			body = body[:len(body)-len(suf)]
			break
		}
	}
	j := digitRunEnd(body, 0)
	if detail := checkDigitRun(body[:j], true); detail != "" {
		return detail
	}
	if j >= len(body) || body[j] != '.' || j == 0 {
		return clauseDotSides
	}
	j++ // the dot
	if j >= len(body) || !isDigit(body[j]) {
		return clauseDotSides
	}
	k := digitRunEnd(body, j)
	if detail := checkDigitRun(body[j:k], false); detail != "" {
		return detail
	}
	j = k
	if j < len(body) && (body[j] == 'e' || body[j] == 'E') {
		j++
		if j < len(body) && (body[j] == '+' || body[j] == '-') {
			j++
		}
		if j >= len(body) || !isDigit(body[j]) {
			return clauseExponent
		}
		k := digitRunEnd(body, j)
		if detail := checkDigitRun(body[j:k], false); detail != "" {
			return detail
		}
		j = k
	}
	if j < len(body) {
		// Leftover after a complete form: an unknown suffix attempt
		// (`1.5i8`, `1.5e3x`) or stray material — the closed-suffix rule.
		return clauseSuffix
	}
	return ""
}

// scanAttr consumes one attribute unit: the token `#[`, a name, optional
// parenthesized arguments, and the closing `]` — one lexical structure.
// Validation order (design D7): form (E0001), then the literal-only
// argument rule (E0009), then the closed whitelist (E0008) — with an
// empty whitelist, checking the name first would leave E0009 unreachable.
func (l *lexer) scanAttr() Token {
	line, col := l.pos()
	start := l.off
	if l.peek2() != '[' {
		l.fail(line, col, "E0001", `invalid character — "#" must be followed by "[" to open an attribute unit`)
	}
	l.advance() // '#'
	l.advance() // '['
	l.skipHoriz()
	if l.eof() || !isIdentStart(l.peek()) {
		nl, nc := l.pos()
		l.fail(nl, nc, "E0001", attrFormMsg)
	}
	nameLine, nameCol := l.pos()
	nameStart := l.off
	for !l.eof() && isIdentChar(l.peek()) {
		l.advance()
	}
	name := l.src[nameStart:l.off]
	if keywords[name] {
		l.fail(nameLine, nameCol, "E0001", attrFormMsg) // a keyword is not a name
	}
	l.skipHoriz()
	argsRaw, argsOff := "", 0
	if !l.eof() && l.peek() == '(' {
		argsOff = l.off + 1
		l.consumeAttrArgs()
		argsRaw = strings.TrimSpace(l.src[argsOff : l.off-1])
	}
	l.skipHoriz()
	if l.eof() || l.peek() != ']' {
		nl, nc := l.pos()
		l.fail(nl, nc, "E0001", attrFormMsg)
	}
	l.advance() // ']'
	if argsRaw != "" {
		l.validateAttrArgs(argsRaw, argsOff)
	}
	if !attrWhitelist[name] {
		l.fail(line, col, "E0008", fmt.Sprintf("unknown attribute — %q is not in the closed whitelist", name))
	}
	return Token{
		Kind: KindAttr, Text: l.src[start:l.off], Line: line, Col: col,
		AttrName: name, AttrArgs: argsRaw,
	}
}

func (l *lexer) skipHoriz() {
	for !l.eof() && (l.peek() == ' ' || l.peek() == '\t') {
		l.advance()
	}
}

// consumeAttrArgs consumes a balanced parenthesized argument region,
// starting at '(' and stopping after its matching ')'. Strings and runes
// inside are consumed with their own literal rules; a newline before the
// unit closes is a form violation.
func (l *lexer) consumeAttrArgs() {
	stack := []byte{')'}
	l.advance() // '('
	for len(stack) > 0 {
		if l.eof() || l.peek() == '\n' || l.peek() == '\r' {
			nl, nc := l.pos()
			l.fail(nl, nc, "E0001", attrFormMsg)
		}
		switch c := l.peek(); c {
		case '(':
			stack = append(stack, ')')
			l.advance()
		case '[':
			stack = append(stack, ']')
			l.advance()
		case ')', ']':
			if c != stack[len(stack)-1] {
				nl, nc := l.pos()
				l.fail(nl, nc, "E0001", attrFormMsg)
			}
			stack = stack[:len(stack)-1]
			l.advance()
		case '"':
			sl, sc := l.pos()
			if !l.consumeStringLit(true) {
				l.fail(sl, sc, "E0002", "unterminated string literal — not closed before the end of the line")
			}
		case '\'':
			l.consumeRuneLit()
		default:
			l.advance()
		}
	}
}

// validateAttrArgs checks the literal-only argument rule (E0009) against
// the raw argument text: each argument is a string, number, boolean, or a
// literal list of those; anything else — an expression, call, or
// reference — fails at the argument's start.
func (l *lexer) validateAttrArgs(raw string, base int) {
	i, n := 0, len(raw)
	isWS := func(c byte) bool { return c == ' ' || c == '\t' }
	for {
		for i < n && isWS(raw[i]) {
			i++
		}
		if i >= n {
			return
		}
		argStart := i
		if !l.parseArgLiteral(raw, &i, base) {
			l.failAt(base+argStart, "E0009",
				fmt.Sprintf("non-literal attribute argument — %q: arguments are compile-time literals only", collectArg(raw, argStart)))
		}
		for i < n && isWS(raw[i]) {
			i++
		}
		if i >= n {
			return
		}
		if raw[i] == ',' {
			i++
			continue
		}
		l.failAt(base+argStart, "E0009",
			fmt.Sprintf("non-literal attribute argument — %q: arguments are compile-time literals only", collectArg(raw, argStart)))
	}
}

// parseArgLiteral parses one literal at raw[*i], advancing *i past it.
// It reports false when the argument is not a literal; a malformed
// numeric inside the region reports E0006 directly (a bad literal is a
// literal-form error, not a non-literal argument).
func (l *lexer) parseArgLiteral(raw string, i *int, base int) bool {
	n := len(raw)
	switch raw[*i] {
	case '"':
		end, interp := skipArgString(raw, *i)
		if end < 0 || interp {
			return false // an interpolating string embeds a reference
		}
		*i = end
		return true
	case '[':
		*i++
		for {
			for *i < n && (raw[*i] == ' ' || raw[*i] == '\t') {
				*i++
			}
			if *i >= n {
				return false
			}
			if raw[*i] == ']' {
				*i++
				return true
			}
			if !l.parseArgLiteral(raw, i, base) {
				return false
			}
			for *i < n && (raw[*i] == ' ' || raw[*i] == '\t') {
				*i++
			}
			if *i < n && raw[*i] == ',' {
				*i++
				continue
			}
			if *i >= n || raw[*i] != ']' {
				return false
			}
		}
	case 't':
		if strings.HasPrefix(raw[*i:], "true") && (*i+4 >= n || !isIdentChar(raw[*i+4])) {
			*i += 4
			return true
		}
		return false
	case 'f':
		if strings.HasPrefix(raw[*i:], "false") && (*i+5 >= n || !isIdentChar(raw[*i+5])) {
			*i += 5
			return true
		}
		return false
	default:
		if !isDigit(raw[*i]) {
			return false
		}
		end := numericSpan(raw, *i)
		text := raw[*i:end]
		switch {
		case len(text) >= 2 && text[0] == '0' && (text[1] == 'x' || text[1] == 'o' || text[1] == 'b'):
			if detail := validateBased(text); detail != "" {
				l.failAt(base+*i, "E0006", fmt.Sprintf("invalid numeric literal — %q: %s", text, detail))
			}
			*i = end
			return true
		case strings.IndexByte(text, '.') >= 0:
			if detail := validateFloat(text); detail != "" {
				l.failAt(base+*i, "E0006", fmt.Sprintf("invalid numeric literal — %q: %s", text, detail))
			}
			*i = end
			return true
		}
		j := digitRunEnd(text, 0)
		digits, tail := text[:j], text[j:]
		if detail := checkDigitRun(digits, true); detail != "" {
			l.failAt(base+*i, "E0006", fmt.Sprintf("invalid numeric literal — %q: %s", text, detail))
		}
		switch {
		case tail == "" || knownIntSuffix(tail):
			*i = end
			return true
		case knownFloatSuffix(tail) || allLetters(tail):
			l.failAt(base+*i, "E0006", fmt.Sprintf("invalid numeric literal — %q: %s", text, clauseSuffix))
		}
		return false // not one literal token (as in `1e5`): the argument is non-literal
	}
}

// skipArgString returns the offset just past a string literal starting at
// raw[i] == '"', and whether it contains an interpolation hole (such a
// string embeds an expression, so it is not a compile-time literal).
// Input is already validated; -1 is defensive only.
func skipArgString(raw string, i int) (end int, interp bool) {
	j := i + 1
	for j < len(raw) {
		switch raw[j] {
		case '\\':
			j += 2
		case '"':
			return j + 1, interp
		case '$':
			if j+1 < len(raw) && raw[j+1] == '{' {
				return skipArgHole(raw, j+2)
			}
			j++
		default:
			j++
		}
	}
	return -1, interp
}

// skipArgHole continues skipArgString from inside an interpolation hole.
func skipArgHole(raw string, j int) (end int, interp bool) {
	depth := 1
	for j < len(raw) && depth > 0 {
		switch raw[j] {
		case '{':
			depth++
			j++
		case '}':
			depth--
			j++
		case '"':
			e, _ := skipArgString(raw, j)
			if e < 0 {
				return -1, true
			}
			j = e
		default:
			j++
		}
	}
	if depth > 0 {
		return -1, true
	}
	// continue the enclosing string after the hole
	for j < len(raw) {
		switch raw[j] {
		case '\\':
			j += 2
		case '"':
			return j + 1, true
		case '$':
			if j+1 < len(raw) && raw[j+1] == '{' {
				return skipArgHole(raw, j+2)
			}
			j++
		default:
			j++
		}
	}
	return -1, true
}

// collectArg gathers the argument text from a start offset to the next
// top-level comma (or the end), for the E0009 message.
func collectArg(raw string, from int) string {
	j, depth := from, 0
	for j < len(raw) {
		switch raw[j] {
		case '"':
			e, _ := skipArgString(raw, j)
			if e < 0 {
				j++
				continue
			}
			j = e
			continue
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ',':
			if depth == 0 {
				return strings.TrimSpace(raw[from:j])
			}
		}
		j++
	}
	return strings.TrimSpace(raw[from:])
}
