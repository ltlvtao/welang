// Package parser implements the parsing stage of chapters 2 and 6
// (docs/spec/0200-grammar.md, docs/spec/0600-declarations.md) over chapter
// 1's token stream: the line-joining rule with its two decision points,
// blocks as expressions, the statement families, the expression skeleton
// with the closed 12-level precedence table, and the module's top-level
// items with their naming, name-space, and documentation rules. Chapter 7's
// type references fill the annotation slots (syntax only — no checking).
// Parsing is deterministic recursive descent — exactly one tree or
// rejected — and stops at the first diagnostic (chapter 21: an E-severity
// diagnostic stops the pipeline). A NotImplemented result is not a
// diagnostic: it reports a form a ratified chapter owns that this
// reference build has not implemented yet, one boundary per form group.
package parser

import (
	"fmt"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/lex"
)

// The boundary form groups (design D6's closed table). Each later
// milestone deletes its rows; the list shrinks to zero with the roadmap.
const (
	bndCtl    = "chapter 3 (control flow) forms"
	bndMatch  = "chapter 4 (match) forms"
	bndIter   = "chapter 5 (iteration) forms"
	bndTypes  = "chapter 7 (types) forms"
	bndComp   = "chapter 8 (composites) forms"
	bndGener  = "chapter 10 (interfaces and generics) forms"
	bndClosur = "chapter 12 (fn types and closures) forms"
	bndScope  = "chapter 13 and 18 (scope) forms"
	bndErr    = "chapter 14 (errors) forms"
	bndEffect = "chapter 16 (effects) forms"
	bndColl   = "chapter 17 (collections) forms"
	bndConc   = "chapter 18 (concurrency) forms"
	bndFFI    = "chapter 19 (ffi) forms"
	bndTest   = "chapter 20 (testing) forms"
	bndMutPar = "mut parameters"
)

// helps carries each code's remediation, compressed from its registry
// entry (docs/spec/diagnostics.toml), matching the conformance goldens.
// One home until the registry embed lands (roadmap follow-up 2).
var helps = map[string]string{
	"E0101": "Rewrite the construct with explicit parentheses to force a single grouping, or split it across lines so an inferred statement boundary separates the two intended items.",
	"E0102": "Move the operator or dot to the end of the previous line so it trails (for example svc. then query() on the next line), or keep the construct on one line.",
	"E0103": "Split into separate statements: assign to each target on its own line; for a call argument pass the value directly instead of assigning inside the call.",
	"E0104": "Write the split form with explicit grouping, for example (a < b) && (b < c); each comparison and each range must be a single explicit group.",
	"E0105": "Check the construct against the ratified surface forms of the grammar chapter; a construct no chapter ratifies — bracketed indexing among them — is re-expressed with ratified forms.",
	"E0401": "Move the return inside the function whose value it carries; for module-level logic wrap it in a fn.",
	"E0402": "Either declare the return type (fn f() -> T) so the value is the function's product, or drop the expression from the return.",
	"E0403": "Declare the top-level binding with let; move the mutable state inside a function, or model shared mutable state through the concurrency chapter's mechanism.",
	"E0404": "Rename one of the declarations, or give the import an alias (or drop an existing alias) so the introduced names differ.",
	"E0405": "Add the declaration the documentation describes, convert the lines to ordinary // comments, or remove them.",
	"E0012": "Rename the binding to camelCase (for example userId) and update references; constants also use camelCase.",
	"E0013": "Rename the module to lowercase with dot separation (for example net.http.client) and update imports.",
}

// NotImplemented reports a ratified-but-unimplemented form. What names the
// form group; the CLI prints the boundary line and exits 70.
type NotImplemented struct{ What string }

// stop and bstop unwind the parse at the first diagnostic or boundary.
type stop struct{ d diag.Diagnostic }
type bstop struct{ what string }

// fnCtx is the enclosing function body: its name anchors E0402's message
// and its declared-return flag decides whether `return expr` is legal.
type fnCtx struct {
	name   string
	hasRet bool
}

// parser holds the parse state. last is the most recently consumed token
// (Line 0 before the first) — the line-joining rule compares against it.
// depth counts the line-insensitive regions: ( groups and call argument
// lists. names is the module name space (chapter 6); itemStarts feeds the
// documentation attachment pass.
type parser struct {
	name       string
	toks       []lex.Token
	pos        int
	last       lex.Token
	depth      int
	fns        []fnCtx
	names      map[string]int
	docs       []lex.DocUnit
	itemStarts []int
}

// Parse parses one source file. It returns the tree on a clean parse, or
// the first diagnostic (lexical or syntactic) with no tree, or a
// NotImplemented boundary with no tree.
func Parse(name string, src []byte) (file *ast.File, d *diag.Diagnostic, ni *NotImplemented) {
	toks, docs, lexErr := lex.Scan(name, src)
	if lexErr != nil {
		return nil, lexErr, nil
	}
	p := &parser{name: name, toks: toks, docs: docs, names: map[string]int{}}
	defer func() {
		if r := recover(); r != nil {
			switch s := r.(type) {
			case stop:
				file, d, ni = nil, &s.d, nil
			case bstop:
				file, d, ni = nil, nil, &NotImplemented{What: s.what}
			default:
				panic(r)
			}
		}
	}()
	return p.parseFile(), nil, nil
}

// --- token access -----------------------------------------------------------

func (p *parser) cur() lex.Token { return p.toks[p.pos] }

func (p *parser) atEnd() bool { return p.cur().Kind == lex.KindEOF }

func (p *parser) next() {
	p.last = p.toks[p.pos]
	p.pos++
}

// peek is the token after the current one (the eof token bounds the slice).
func (p *parser) peek() lex.Token { return p.toks[p.pos+1] }

// brokeLine reports whether the current token starts a new line relative
// to the last consumed one — the depth-zero statement boundary fact.
func (p *parser) brokeLine() bool { return p.cur().Line > p.last.Line }

func isKw(t lex.Token, word string) bool {
	return t.Kind == lex.KindKeyword && t.Text == word
}

func (p *parser) fail(line, col int, code, message string) {
	d := diag.Error(code, message).At(p.name, line, col)
	if h, ok := helps[code]; ok {
		d = d.WithHelp(h)
	}
	panic(stop{d})
}

func (p *parser) failTok(t lex.Token, code, message string) {
	p.fail(t.Line, t.Col, code, message)
}

func (p *parser) bnd(what string) { panic(bstop{what}) }

// --- charsets (chapter 1 naming, checked at the declarations that bind) -----

func isCamel(s string) bool {
	if len(s) == 0 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func isModuleSeg(s string) bool {
	if len(s) == 0 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func isPascal(s string) bool { return len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z' }

// checkCamel enforces E0012 at a binding token.
func (p *parser) checkCamel(t lex.Token) {
	if !isCamel(t.Text) {
		p.failTok(t, "E0012",
			fmt.Sprintf("variable, function, and method names must be camelCase — %q is not camelCase", t.Text))
	}
}

// declare enters a name into the module name space; the later of two
// colliding declarations is rejected (chapter 6).
func (p *parser) declare(t lex.Token) {
	if line, dup := p.names[t.Text]; dup {
		p.failTok(t, "E0404",
			fmt.Sprintf("duplicate name in one module — %q is already declared at line %d; one module has one name space", t.Text, line))
	}
	p.names[t.Text] = t.Line
}

// --- file structure (chapter 6) ----------------------------------------------

// parseFile parses the module: top-level items separated by line breaks,
// then the documentation attachment pass.
func (p *parser) parseFile() *ast.File {
	f := &ast.File{}
	for !p.atEnd() {
		p.checkStart(true)
		f.Items = append(f.Items, p.parseItem())
		if p.atEnd() {
			break
		}
		if !p.brokeLine() {
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q after a top-level item: items are separated by line breaks", p.cur().Text))
		}
	}
	p.attachDocs(f)
	return f
}

// checkStart enforces the closed statement-start class at an item or
// statement position: a token outside it is E0102 naming the end of the
// previous statement (design D3).
func (p *parser) checkStart(top bool) {
	t := p.cur()
	if t.Kind == lex.KindAttr {
		if top {
			p.failTok(t, "E0105",
				fmt.Sprintf("unexpected token — %q fits no top-level item production: no attribute target is ratified yet", t.Text))
		} else {
			p.failTok(t, "E0105",
				fmt.Sprintf("unexpected token — %q fits no statement production: no attribute target is ratified yet", t.Text))
		}
	}
	switch t.Kind {
	case lex.KindIdent, lex.KindInt, lex.KindFloat, lex.KindString, lex.KindRune,
		lex.KindKeyword, "(", "{", "!", "-", "~":
		return
	}
	if p.last.Line == 0 {
		p.failTok(t, "E0102",
			fmt.Sprintf("statement begins with a continuation token — %q cannot begin a statement; no statement precedes it", t.Text))
	}
	p.failTok(t, "E0102",
		fmt.Sprintf("statement begins with a continuation token — %q cannot begin a statement; the previous statement ends at line %d", t.Text, p.last.Line))
}

// parseItem dispatches one top-level item by its keyword (design D6's
// top-level column); a non-item statement that passed the start class is
// E0105 naming the item productions.
func (p *parser) parseItem() ast.Item {
	t := p.cur()
	p.itemStarts = append(p.itemStarts, t.Line)
	if t.Kind == lex.KindKeyword {
		switch t.Text {
		case "import":
			return p.parseImport()
		case "pub":
			p.next()
			if isKw(p.cur(), "fn") {
				return p.parseFnDecl(true, t.Line, t.Col)
			}
			if isKw(p.cur(), "let") {
				return p.parseTopLet(true, t.Line, t.Col)
			}
			if p.atEnd() {
				p.fail(t.Line, t.Col, "E0105", "unexpected end of file — pub prefixes a fn or let declaration")
			}
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q after pub: pub prefixes fn and let", p.cur().Text))
		case "fn":
			return p.parseFnDecl(false, t.Line, t.Col)
		case "let":
			return p.parseTopLet(false, t.Line, t.Col)
		case "var":
			p.failTok(t, "E0403",
				"var at the top level — module-level mutable state does not exist; declare the top-level binding with let (var is legal inside blocks)")
		case "return":
			p.failTok(t, "E0401",
				"return outside a function body — no function context encloses this return; module-level logic belongs in a fn")
		case "record", "byval", "byres", "newtype":
			p.bnd(bndComp)
		case "type":
			p.bnd(bndTypes)
		case "interface", "impl":
			p.bnd(bndGener)
		case "effect":
			p.bnd(bndEffect)
		case "foreign":
			p.bnd(bndFFI)
		case "test":
			p.bnd(bndTest)
		}
	}
	p.failTok(t, "E0105",
		fmt.Sprintf("unexpected token — %q fits no top-level item production: top-level items are import, fn, and let declarations (optionally pub); statements exist only inside blocks", t.Text))
	panic("unreachable")
}

// parseImport parses `import path [as alias]`; each path segment and the
// alias obey the module charset (E0013), and the introduced name joins the
// module name space.
func (p *parser) parseImport() *ast.Import {
	kw := p.cur()
	p.next()
	imp := &ast.Import{Line: kw.Line, Col: kw.Col}
	last := p.moduleSeg()
	imp.Path = append(imp.Path, last.Text)
	for p.cur().Kind == "." {
		p.next()
		last = p.moduleSeg()
		imp.Path = append(imp.Path, last.Text)
	}
	nameTok := last
	if isKw(p.cur(), "as") {
		p.next()
		nameTok = p.moduleSeg()
		imp.Alias = nameTok.Text
	}
	p.declare(nameTok)
	return imp
}

// moduleSeg expects one lowercase dotted-path segment.
func (p *parser) moduleSeg() lex.Token {
	t := p.cur()
	if t.Kind != lex.KindIdent {
		if t.Kind == lex.KindEOF {
			p.failTok(t, "E0105", "unexpected end of file — an import wants a lowercase dotted path (and its alias after as)")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q in an import: imports are import path [as name]", t.Text))
	}
	if !isModuleSeg(t.Text) {
		p.failTok(t, "E0013",
			fmt.Sprintf("module names must be lowercase — %q is not a lowercase dotted path segment", t.Text))
	}
	p.next()
	return t
}

// parseTopLet parses `[pub] let name [: type] = expr`.
func (p *parser) parseTopLet(pub bool, line, col int) *ast.TopLet {
	b := p.parseBinding() // consumes the let keyword itself
	if b.Name != "_" {
		// The binding-name token is the one before any annotation: rebuild
		// its position from the recorded fields.
		p.declare(lex.Token{Kind: lex.KindIdent, Text: b.Name, Line: b.NameLine, Col: b.NameCol})
	}
	return &ast.TopLet{Pub: pub, Binding: *b, Line: line, Col: col}
}

// parseFnDecl parses `[pub] fn name(params) [-> type] block`. Later-
// chapter clauses (generics, effect segments, mut parameters) stop at
// their boundaries before any of their own grammar runs.
func (p *parser) parseFnDecl(pub bool, line, col int) *ast.FnDecl {
	p.next() // fn
	d := &ast.FnDecl{Pub: pub, Line: line, Col: col}
	t := p.cur()
	if t.Kind != lex.KindIdent {
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q where a fn name goes: a fn declaration is fn name(params) [-> type] { body }", t.Text))
	}
	p.checkCamel(t)
	d.Name, d.NameLine, d.NameCol = t.Text, t.Line, t.Col
	p.next()
	if p.cur().Kind == "<" {
		p.bnd(bndGener)
	}
	if p.cur().Kind != "(" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a fn wants its parameter list")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a fn's parameter list opens", p.cur().Text))
	}
	p.next() // (
	for {
		if p.cur().Kind == ")" {
			p.next()
			break
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a parameter list closes with )")
		}
		nt := p.cur()
		if isKw(nt, "mut") {
			p.bnd(bndMutPar)
		}
		if nt.Kind != lex.KindIdent {
			p.failTok(nt, "E0105",
				fmt.Sprintf("unexpected token — %q in a parameter list: parameters are name: type pairs", nt.Text))
		}
		p.next()
		if p.cur().Kind != ":" {
			p.failTok(nt, "E0105",
				fmt.Sprintf("unexpected token — parameter %q carries no annotation: parameters are name: type; parameter types are never inferred", nt.Text))
		}
		p.checkCamel(nt)
		p.next() // :
		ty := p.parseTypeRef()
		d.Params = append(d.Params, ast.Param{Name: nt.Text, Type: ty, NameLine: nt.Line, NameCol: nt.Col})
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == ")" {
			p.next()
			break
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a parameter list closes with )")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a parameter list: parameters are name: type pairs", p.cur().Text))
	}
	if isKw(p.cur(), "effect") {
		p.bnd(bndEffect)
	}
	if p.cur().Kind == "->" {
		p.next()
		d.Ret = p.parseTypeRef()
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a fn wants its body block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a fn body block opens", p.cur().Text))
	}
	d.Body = p.parseBlock(&fnCtx{name: d.Name, hasRet: d.Ret != nil})
	p.declare(lex.Token{Kind: lex.KindIdent, Text: d.Name, Line: d.NameLine, Col: d.NameCol})
	return d
}

// attachDocs resolves each /// unit against the next top-level item; a
// unit with no following item is an orphan (E0405) at its first line.
func (p *parser) attachDocs(f *ast.File) {
	for _, u := range p.docs {
		idx := -1
		for i, ln := range p.itemStarts {
			if ln > u.EndLine {
				idx = i
				break
			}
		}
		if idx < 0 {
			p.fail(u.StartLine, 1, "E0405",
				"orphan documentation comment — the /// unit documents no following top-level item; add the declaration or use ordinary // comments")
		}
		f.Docs = append(f.Docs, ast.DocAttach{StartLine: u.StartLine, EndLine: u.EndLine, Lines: u.Lines, Item: idx})
	}
}

// --- blocks and statements (chapter 2) ---------------------------------------

// parseBlock parses `{ items }`. fctx non-nil marks a fn body: returns
// inside it are legal and checked against the declared return type. Block
// braces are not brackets — line breaks inside stay significant; only the
// parenthesis regions go insensitive.
func (p *parser) parseBlock(fctx *fnCtx) ast.Block {
	open := p.cur()
	p.next()
	if fctx != nil {
		p.fns = append(p.fns, *fctx)
		defer func() { p.fns = p.fns[:len(p.fns)-1] }()
	}
	b := ast.Block{Line: open.Line, Col: open.Col}
	for {
		if p.cur().Kind == "}" {
			p.next()
			return b
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a block wants its closing }")
		}
		p.checkStart(false)
		b.Items = append(b.Items, p.parseStmt())
		if p.cur().Kind == "}" {
			p.next()
			return b
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a block wants its closing }")
		}
		if !p.brokeLine() {
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q after a complete statement: a statement ends at its line break; only } or a new statement follows", p.cur().Text))
		}
	}
}

// parseStmt dispatches one statement by keyword (design D6's statement
// column); an identifier followed by `=` on the same line is an
// assignment; anything else in the class is an expression statement.
func (p *parser) parseStmt() ast.Stmt {
	t := p.cur()
	if t.Kind == lex.KindKeyword {
		switch t.Text {
		case "let", "var":
			b := p.parseBinding()
			return b
		case "return":
			return p.parseReturn()
		case "if", "while", "loop", "break", "continue", "defer":
			p.bnd(bndCtl)
		case "match":
			p.bnd(bndMatch)
		case "for":
			p.bnd(bndIter)
		case "scope":
			p.bnd(bndScope)
		case "task", "select":
			p.bnd(bndConc)
		case "mock":
			p.bnd(bndTest)
		case "fn":
			p.bnd(bndClosur)
		case "true", "false":
			// a literal expression statement
		case "pub", "import", "as", "mut", "else", "in", "where", "derives",
			"with", "resource", "effect", "case", "timeout", "collectAll",
			"record", "byval", "byres", "newtype", "type", "interface",
			"impl", "foreign", "test":
			p.failTok(t, "E0105",
				fmt.Sprintf("unexpected token — %q fits no statement production: statements are let|var bindings, name assignments, return, and expression statements", t.Text))
		}
	}
	if t.Kind == lex.KindIdent && p.peek().Kind == "=" && p.peek().Line == t.Line {
		p.next() // name
		p.next() // =
		v := p.parseExpr(valueCtx)
		return &ast.Assign{Name: t.Text, Value: v, Line: t.Line, Col: t.Col}
	}
	e := p.parseExpr(stmtCtx)
	return &ast.ExprStmt{Expr: e, Line: t.Line, Col: t.Col}
}

// parseBinding parses `let|var name [: type] = expr` (name `_` discards).
func (p *parser) parseBinding() *ast.Binding {
	kw := p.cur()
	p.next()
	b := &ast.Binding{Kw: kw.Text, Line: kw.Line, Col: kw.Col}
	t := p.cur()
	switch {
	case t.Kind == lex.KindIdent:
		p.checkCamel(t)
		b.Name, b.NameLine, b.NameCol = t.Text, t.Line, t.Col
		p.next()
	case t.Kind == "_":
		b.Name, b.NameLine, b.NameCol = "_", t.Line, t.Col
		p.next()
	case t.Kind == "(":
		p.bnd(bndComp) // tuple patterns are chapter 8's
	default:
		if t.Kind == lex.KindEOF {
			p.failTok(t, "E0105", "unexpected end of file — a binding wants let|var name [: type] = expr")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q in a binding head: bindings are let|var name [: type] = expr", t.Text))
	}
	if p.cur().Kind == ":" {
		p.next()
		b.Typ = p.parseTypeRef()
	}
	if p.cur().Kind != "=" {
		t := p.cur()
		if t.Kind == lex.KindEOF {
			p.failTok(t, "E0105", "unexpected end of file — a binding wants = and its initializer")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q in a binding head: bindings are let|var name [: type] = expr", t.Text))
	}
	p.next() // =
	b.Init = p.parseExpr(valueCtx)
	return b
}

// parseReturn parses `return` / `return expr`. Outside any fn body it is
// E0401; a value where the fn declares none is E0402 (a bare return in a
// fn with a declared type parses — exhaustiveness is the types stage's).
func (p *parser) parseReturn() *ast.Return {
	t := p.cur()
	p.next()
	hasValue := !p.atEnd() && p.cur().Kind != "}" && !(p.depth == 0 && p.brokeLine())
	if len(p.fns) == 0 {
		p.failTok(t, "E0401",
			"return outside a function body — no function context encloses this return; module-level logic belongs in a fn")
	}
	if hasValue && !p.fns[len(p.fns)-1].hasRet {
		fn := p.fns[len(p.fns)-1]
		p.failTok(t, "E0402",
			fmt.Sprintf("return with a value in a function that declares none — %q declares no return type; bare return exits a valueless function legally", fn.name))
	}
	r := &ast.Return{HasValue: hasValue, Line: t.Line, Col: t.Col}
	if hasValue {
		r.Value = p.parseExpr(valueCtx)
	}
	return r
}

// --- expressions (chapter 2's skeleton and precedence table) ------------------

// exprCtx distinguishes the two `=`-residue readings (design D4): in a
// value context `=` after a complete expression is E0103; in a statement
// context it is an assignment attempt on a non-name target (E0105, or
// chapter 10's boundary for the one self.field form).
type exprCtx int

const (
	valueCtx exprCtx = iota
	stmtCtx
)

// binLevels is chapter 2's closed 12-level table; higher is looser. 9 and
// 12 are the non-associative levels.
var binLevels = map[string]int{
	"*": 3, "/": 3, "%": 3,
	"+": 4, "-": 4,
	"<<": 5, ">>": 5,
	"&": 6, "^": 7, "|": 8,
	"<": 9, "<=": 9, ">": 9, ">=": 9, "==": 9, "!=": 9,
	"&&": 10, "||": 11, "..": 12,
}

// parseExpr parses one full expression.
func (p *parser) parseExpr(ctx exprCtx) ast.Expr { return p.parseBinary(12, ctx) }

// parseBinary is precedence climbing over the closed table. The loop head
// carries the two line-joining decision points: at depth zero a token on
// a new line ends the expression (the operator then E0102s as a statement
// start), and a `=` after a complete expression is the assignment rule.
func (p *parser) parseBinary(level int, ctx exprCtx) ast.Expr {
	if level == 2 {
		return p.parseUnary(ctx)
	}
	lhs := p.parseBinary(level-1, ctx)
	for {
		if p.atEnd() {
			return lhs
		}
		if p.depth == 0 && p.brokeLine() {
			return lhs
		}
		if p.cur().Kind == "=" {
			if ctx == stmtCtx {
				p.assignAfterExpr(lhs)
			}
			p.failTok(p.cur(), "E0103",
				`assignment is not an expression — "=" cannot appear in an expression position: assignment is a statement, not an expression`)
		}
		op := p.cur()
		if binLevels[op.Kind] != level {
			return lhs
		}
		p.next()
		rhs := p.parseBinary(level-1, ctx)
		if level == 9 || level == 12 {
			// Non-associative: one operator per group. A same-level
			// operator chaining it (after a legal continuation check)
			// is E0104 at the second operator.
			if !p.atEnd() {
				if p.depth == 0 && p.brokeLine() {
					return p.binary(op, lhs, rhs)
				}
				if binLevels[p.cur().Kind] == level {
					p.failNonAssoc(p.cur())
				}
			}
			return p.binary(op, lhs, rhs)
		}
		lhs = p.binary(op, lhs, rhs)
	}
}

func (p *parser) binary(op lex.Token, l, r ast.Expr) ast.Expr {
	return &ast.Binary{Op: op.Text, L: l, R: r, Line: op.Line, Col: op.Col}
}

// failNonAssoc reports a chained non-associative operator at the second
// operator of the chain.
func (p *parser) failNonAssoc(op lex.Token) {
	if op.Kind == ".." {
		p.failTok(op, "E0104",
			`chained non-associative operator — ".." chains a non-associative level; each range must be a single explicit group`)
	}
	p.failTok(op, "E0104",
		fmt.Sprintf("chained non-associative operator — %q chains a non-associative level; write the split form, for example (a < b) && (b < c)", op.Text))
}

// assignAfterExpr handles `=` after a complete expression statement: the
// one ratified field form self.field is chapter 10's boundary; any other
// non-name target is E0105.
func (p *parser) assignAfterExpr(lhs ast.Expr) {
	if m, ok := lhs.(*ast.Member); ok {
		if id, ok := m.Recv.(*ast.Ident); ok && id.Name == "self" {
			p.bnd(bndGener)
		}
	}
	p.failTok(p.cur(), "E0105",
		`unexpected token — "=" after a complete expression statement: assignment targets are bare names (the one field form, self.field, belongs to a method receiver)`)
}

// parseUnary parses prefix `!`, `-`, `~` (level 2); prefix operators bind
// looser than postfix, so their operand carries the whole chain.
func (p *parser) parseUnary(ctx exprCtx) ast.Expr {
	t := p.cur()
	switch t.Kind {
	case "!", "-", "~":
		p.next()
		x := p.parseUnary(ctx)
		return &ast.Unary{Op: t.Text, X: x, Line: t.Line, Col: t.Col}
	}
	return p.parsePostfix(ctx)
}

// parsePostfix parses the left-associative postfix chain (level 1):
// .name, (args). `?` and construction braces belong to later chapters;
// `[` is ratified never — indexing is a diagnostic, not a boundary.
func (p *parser) parsePostfix(ctx exprCtx) ast.Expr {
	node := p.parsePrimary(ctx)
	for {
		if p.atEnd() {
			return node
		}
		if p.depth == 0 && p.brokeLine() {
			return node
		}
		t := p.cur()
		switch t.Kind {
		case ".":
			p.next()
			nt := p.cur()
			if nt.Kind != lex.KindIdent {
				p.failTok(nt, "E0105",
					fmt.Sprintf("unexpected token — %q after \".\": member names are identifiers", nt.Text))
			}
			node = &ast.Member{Recv: node, Name: nt.Text, Line: t.Line, Col: t.Col}
			p.next()
		case "(":
			node = &ast.Call{Fn: node, Args: p.parseCallArgs(), Line: t.Line, Col: t.Col}
		case "?":
			p.bnd(bndErr)
		case "[":
			p.failTok(t, "E0105",
				`unexpected token — "[" fits no postfix production: postfix is .name or (args); indexing is by named methods (the collections chapter)`)
		case "{":
			// Construction: a bare name/member chain whose final name is
			// PascalCase, with the brace on the same line (chapter 8's
			// boundary). Anything else cannot be a construction head.
			if name, bare := chainHead(node); bare && isPascal(name) {
				p.bnd(bndComp)
			}
			p.failTok(t, "E0105",
				`unexpected token — "{" after a complete expression: construction heads are PascalCase type references (chapter 8)`)
		default:
			return node
		}
	}
}

// chainHead reports whether e is a bare identifier or member chain (no
// calls or other postfixes) and its final name.
func chainHead(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name, true
	case *ast.Member:
		if _, ok := chainHead(x.Recv); ok {
			return x.Name, true
		}
	}
	return "", false
}

// parseCallArgs consumes `( ... )` with comma-separated expressions and a
// uniform trailing comma; inside, line breaks are insignificant.
func (p *parser) parseCallArgs() []ast.Expr {
	p.next() // (
	p.depth++
	defer func() { p.depth-- }()
	var args []ast.Expr
	for {
		if p.cur().Kind == ")" {
			p.next()
			return args
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a call argument list closes with )")
		}
		args = append(args, p.parseExpr(valueCtx))
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == ")" {
			p.next()
			return args
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a call argument list closes with )")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a call argument list: arguments are comma-separated expressions and the list closes with )", p.cur().Text))
	}
}

// parsePrimary parses one primary expression; later-chapter primaries
// (lists, closures, fn expressions, control-flow forms) stop at their
// boundaries or reject with the productions considered.
func (p *parser) parsePrimary(ctx exprCtx) ast.Expr {
	t := p.cur()
	switch t.Kind {
	case lex.KindIdent:
		p.next()
		return &ast.Ident{Name: t.Text, Line: t.Line, Col: t.Col}
	case lex.KindInt, lex.KindFloat, lex.KindString, lex.KindRune:
		p.next()
		return &ast.Literal{Kind: t.Kind, Text: t.Text, Line: t.Line, Col: t.Col}
	case lex.KindKeyword:
		switch t.Text {
		case "true", "false":
			p.next()
			return &ast.Literal{Kind: "bool", Text: t.Text, Line: t.Line, Col: t.Col}
		case "if":
			p.bnd(bndCtl)
		case "match":
			p.bnd(bndMatch)
		case "fn":
			p.bnd(bndClosur)
		case "scope":
			p.bnd(bndScope)
		case "task", "select":
			p.bnd(bndConc)
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q cannot begin an expression: expressions are identifiers, literals, (e), { block }, and unary ! - ~", t.Text))
	case "(":
		p.next()
		p.depth++
		if p.cur().Kind == ")" {
			p.bnd(bndComp) // the unit value
		}
		e := p.parseExpr(valueCtx)
		if p.cur().Kind == "," {
			p.bnd(bndComp) // tuples
		}
		if p.cur().Kind != ")" {
			if p.atEnd() {
				p.failTok(p.cur(), "E0105", "unexpected end of file — the group closes with )")
			}
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q where ) closes the group", p.cur().Text))
		}
		p.next()
		p.depth--
		return e // grouping folds away: grouping is binding
	case "{":
		return &ast.BlockExpr{Block: p.parseBlock(nil), Line: t.Line, Col: t.Col}
	case "[":
		p.bnd(bndColl)
	case "|":
		p.bnd(bndClosur)
	}
	p.failTok(t, "E0105",
		fmt.Sprintf("unexpected token — %q cannot begin an expression: expressions are identifiers, literals, (e), { block }, and unary ! - ~", t.Text))
	panic("unreachable")
}

// --- type references (chapter 7, syntax only) ---------------------------------

// parseTypeRef parses one type reference into an annotation slot: Name,
// module.Name (dotted lowercase qualifier), a generic application
// (Dyn<I> is the same shape), a tuple of two or more, the unit type, or a
// fn type with optional bare effect tags before the arrow.
func (p *parser) parseTypeRef() ast.TypeRef {
	t := p.cur()
	switch {
	case isKw(t, "fn"):
		return p.parseFnType()
	case t.Kind == "(":
		return p.parseParenType()
	case t.Kind == lex.KindIdent:
		p.next()
		if isPascal(t.Text) {
			if p.cur().Kind == "." {
				p.failTok(p.cur(), "E0105", typeRefMsg(p.cur()))
			}
			return &ast.NamedType{Name: t.Text, Line: t.Line, Col: t.Col, Args: p.genericArgs()}
		}
		// lowercase head: a module qualifier path ending in the type name
		quals := []string{t.Text}
		last := t
		for p.cur().Kind == "." {
			p.next()
			nt := p.cur()
			if nt.Kind != lex.KindIdent {
				p.failTok(nt, "E0105", typeRefMsg(nt))
			}
			p.next()
			if isPascal(nt.Text) {
				if p.cur().Kind == "." {
					p.failTok(p.cur(), "E0105", typeRefMsg(p.cur()))
				}
				return &ast.NamedType{Qual: joinDot(quals), Name: nt.Text, Line: t.Line, Col: t.Col, Args: p.genericArgs()}
			}
			quals = append(quals, nt.Text)
			last = nt
		}
		// a lowercase run that never reaches a type name
		p.failTok(last, "E0105", typeRefMsg(last))
	}
	if t.Kind == lex.KindEOF {
		p.failTok(t, "E0105", "unexpected end of file — an annotation slot wants a type reference")
	}
	p.failTok(t, "E0105", typeRefMsg(t))
	panic("unreachable")
}

func typeRefMsg(t lex.Token) string {
	return fmt.Sprintf("unexpected token — %q fits no type reference production: type references are Name, module.Name, Name<T…>, (T1, …, Tn), (), Dyn<I>, and fn(…) -> T", t.Text)
}

func joinDot(parts []string) string {
	out := parts[0]
	for _, s := range parts[1:] {
		out += "." + s
	}
	return out
}

// parseParenType parses `()` or a tuple of two or more (a single
// parenthesized type fits no production — grouping is not a type form).
func (p *parser) parseParenType() ast.TypeRef {
	open := p.cur()
	p.next()
	if p.cur().Kind == ")" {
		p.next()
		return &ast.UnitType{Line: open.Line, Col: open.Col}
	}
	first := p.parseTypeRef()
	if p.cur().Kind != "," {
		p.failTok(open, "E0105", typeRefMsg(open))
	}
	tt := &ast.TupleType{Elems: []ast.TypeRef{first}, Line: open.Line, Col: open.Col}
	for {
		p.next() // the comma (or ) after a trailing comma)
		if p.cur().Kind == ")" {
			p.next()
			return tt
		}
		tt.Elems = append(tt.Elems, p.parseTypeRef())
		if p.cur().Kind == "," {
			continue
		}
		if p.cur().Kind == ")" {
			p.next()
			return tt
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a tuple type closes with )")
		}
		p.failTok(p.cur(), "E0105", typeRefMsg(p.cur()))
	}
}

// genericArgs consumes a `<T1, …, Tk>` application when present. Type
// slots are unambiguous contexts, so `<` here is never a comparison.
func (p *parser) genericArgs() []ast.TypeRef {
	if p.cur().Kind != "<" {
		return nil
	}
	p.next()
	var args []ast.TypeRef
	for {
		if p.closeAngle() {
			return args
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a generic argument list closes with >")
		}
		args = append(args, p.parseTypeRef())
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.closeAngle() {
			return args
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a generic argument list closes with >")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a generic argument list: the list closes with >", p.cur().Text))
	}
}

// closeAngle consumes one closing `>` of a generic application and reports
// whether it did. The lexer's maximal munch merges adjacent bytes, so a
// two-byte token starting with `>` splits here: the consumed closer is the
// first byte, and the remainder — `>` for a nested closer, `=` before an
// initializer written without a space — stays as the current token, shifted
// one column. Type slots are unambiguous contexts, so a `>`-led token at a
// closer position is always a closer.
func (p *parser) closeAngle() bool {
	switch t := p.cur(); t.Kind {
	case ">":
		p.next()
		return true
	case ">>":
		p.toks[p.pos] = lex.Token{Kind: ">", Text: ">", Line: t.Line, Col: t.Col + 1}
		return true
	case ">=":
		p.toks[p.pos] = lex.Token{Kind: "=", Text: "=", Line: t.Line, Col: t.Col + 1}
		return true
	}
	return false
}

// parseFnType parses `fn(params) [tags] -> T`; the effect tags are bare
// identifiers between the parameter list and the arrow, and the arrow
// itself is part of the form.
func (p *parser) parseFnType() ast.TypeRef {
	t := p.cur()
	p.next() // fn
	if p.cur().Kind != "(" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a fn type opens its parameter list with (")
		}
		p.failTok(p.cur(), "E0105", typeRefMsg(p.cur()))
	}
	p.next()
	ft := &ast.FnType{Line: t.Line, Col: t.Col}
	for {
		if p.cur().Kind == ")" {
			p.next()
			break
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a fn type's parameter list closes with )")
		}
		ft.Params = append(ft.Params, p.parseTypeRef())
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == ")" {
			p.next()
			break
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a function type reference: fn(T1, …, Tn) [tags] -> T", p.cur().Text))
	}
	for p.cur().Kind == lex.KindIdent {
		ft.EffectTags = append(ft.EffectTags, p.cur().Text)
		p.next()
	}
	if p.cur().Kind != "->" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a fn type declares its result with ->")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a function type reference: fn(T1, …, Tn) [tags] -> T", p.cur().Text))
	}
	p.next()
	ft.Ret = p.parseTypeRef()
	return ft
}
