// Package parser implements the parsing stage of chapters 2–5, 6–8, 10, 12,
// and 17 (docs/spec/0200-grammar.md, docs/spec/0300-control-flow.md,
// docs/spec/0400-match.md, docs/spec/0500-iteration.md,
// docs/spec/0600-declarations.md, docs/spec/0800-composites.md,
// docs/spec/1000-interfaces.md, docs/spec/1100-iterables.md,
// docs/spec/1200-fn-types.md, docs/spec/1700-collections.md) over chapter
// 1's token stream: the line-joining rule with its two decision points,
// blocks as expressions, the statement families, the expression skeleton
// with the closed 12-level precedence table, the module's top-level
// items with their naming, name-space, and documentation rules, chapter
// 3's control statements, chapter 4's match arms and pattern grammar,
// chapter 5's for statement with its irrefutable head, chapter
// 8's composite declarations, construction, update, and tuples,
// chapter 10's interface and impl declarations with their generic, where,
// and derives clauses, chapter 12's two closure forms, and chapter 17's
// list literals. Chapter 7's
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
	bndConcScope = "chapter 18 (scope) forms"
	bndConc      = "chapter 18 (concurrency) forms"
	bndFFI       = "chapter 19 (ffi) forms"
	bndTest      = "chapter 20 (testing) forms"
	bndMutPar    = "mut parameters"
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
	"E0011": "Rename the type to PascalCase (for example UserRecord) and update references; conventions are enforced at the declaration.",
	"E1403": "Choose a camelCase name that is not one of the built-in tags.",
	"E0701": "Declare the variant with one record payload, which names its fields: `Moved(Point)`.",
	"E0201": "Remove the stray break/continue, or move it inside a loop body; to exit a block or function use the value/return forms instead of loop-control words.",
	"E0202": "Supply a value: add an else arm producing a value, or restructure so the valueless form is a statement item and the value comes from an expression, for example an accumulator binding.",
	"E0203": "Wrap the calls in a block: defer { cleanup() }; multiple statements and ordering belong inside the block.",
	"E0204": "Move the defer to the top level of the enclosing function body, or release the resource through `scope resource` instead of a nested defer.",
	"E0301": "Add at least one arm; when only a default outcome is needed, a single wildcard arm `_ => ...` covers all values.",
	"E0302": "Give every branch the same binding names, or drop the bindings from the or-pattern (for example `200 | 404`); re-express per-branch bindings as separate arms.",
	"E0602": "Declare a record with named fields instead of the long tuple.",
}

// NotImplemented reports a ratified-but-unimplemented form. What names the
// form group; the CLI prints the boundary line and exits 70.
type NotImplemented struct{ What string }

// stop and bstop unwind the parse at the first diagnostic or boundary.
type stop struct{ d diag.Diagnostic }
type bstop struct{ what string }

// fnCtx is the enclosing function body: its name anchors E0402's message
// and its declared-return flag decides whether `return expr` is legal. A
// deferBody context is the body of a defer — returns are illegal there
// (E0401) whatever function encloses the defer.
type fnCtx struct {
	name      string
	hasRet    bool
	deferBody bool
}

// parser holds the parse state. last is the most recently consumed token
// (Line 0 before the first) — the line-joining rule compares against it,
// and a short closure's `|` reads it back to tell an operand slot from a
// fresh value position. depth counts the line-insensitive regions: (
// groups and call argument lists. fns stacks the function-body contexts
// (fn declarations, closures, defer bodies). loopDepth counts the enclosing
// while/loop bodies — break and continue need one (E0201); a closure body
// resets it (it is a function body, not a loop body). direct marks the
// function-body top level — the one block level where defer is a legal item
// (E0204). noBrace suspends the postfix `{` inside a control-flow head (the
// brace opens the body, never a construction). shortBody marks a short
// closure's maximal body — break and continue there report E0201, the
// closure-body loop reset, ahead of E0202. fnBrace hands the next primary
// `{` (the body's own leading brace) the function-block reading.
// valSite names the enclosing value position for E0202's
// message. names is the module name space (chapter 6); itemStarts feeds the
// documentation attachment pass.
type parser struct {
	name       string
	toks       []lex.Token
	pos        int
	last       lex.Token
	depth      int
	fns        []fnCtx
	loopDepth  int
	direct     bool
	noBrace    int
	shortBody  int
	fnBrace    bool
	valSite    string
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

// checkPascal enforces E0011 at a type or variant name token (sum type
// names and variant names follow the one rule).
func (p *parser) checkPascal(t lex.Token) {
	if !isPascal(t.Text) {
		p.failTok(t, "E0011",
			fmt.Sprintf("type name must be PascalCase — %q is not PascalCase", t.Text))
	}
}

// dupCheck rejects a name already held by the module name space. Used at
// declaration tokens whose registration completes later (fn bodies), so
// the collision (E0404) outranks the naming convention (E0012) at the
// same token — the chapter 9 scenario reports E0404 for a variant name
// that collides with a PascalCase fn name.
func (p *parser) dupCheck(t lex.Token) {
	if line, dup := p.names[t.Text]; dup {
		p.failTok(t, "E0404",
			fmt.Sprintf("duplicate name in one module — %q is already declared at line %d; one module has one name space", t.Text, line))
	}
}

// declare enters a name into the module name space; the later of two
// colliding declarations is rejected (chapter 6).
func (p *parser) declare(t lex.Token) {
	p.dupCheck(t)
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
			if isKw(p.cur(), "type") {
				return p.parseSumDecl(true, false, t.Line, t.Col)
			}
			if isKw(p.cur(), "byval") {
				bv := p.cur()
				p.next()
				if isKw(p.cur(), "type") {
					return p.parseSumDecl(true, true, t.Line, t.Col)
				}
				if isKw(p.cur(), "record") {
					return p.parseRecordDecl(true, "value", t.Line, t.Col)
				}
				p.failTok(bv, "E0105",
					"unexpected token — byval prefixes type or record here; the prefix order is pub, then byval, then the declaration")
			}
			if isKw(p.cur(), "record") {
				return p.parseRecordDecl(true, "gc", t.Line, t.Col)
			}
			if isKw(p.cur(), "effect") {
				return p.parseEffectDecl(true, t.Line, t.Col)
			}
			if isKw(p.cur(), "byres") {
				p.next()
				if isKw(p.cur(), "record") {
					return p.parseRecordDecl(true, "resource", t.Line, t.Col)
				}
				if p.atEnd() {
					p.fail(t.Line, t.Col, "E0105", "unexpected end of file — pub byres prefixes a record declaration")
				}
				p.failTok(p.cur(), "E0105",
					fmt.Sprintf("unexpected token — %q after byres: byres prefixes record", p.cur().Text))
			}
			if isKw(p.cur(), "newtype") {
				return p.parseNewtypeDecl(true, t.Line, t.Col)
			}
			if isKw(p.cur(), "interface") {
				return p.parseInterfaceDecl(true, t.Line, t.Col)
			}
			if p.atEnd() {
				p.fail(t.Line, t.Col, "E0105", "unexpected end of file — pub prefixes a fn, let, type, chapter 8 declaration, or interface")
			}
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q after pub: pub prefixes fn, let, type, the chapter 8 declarations, and interface (an impl block carries no pub)", p.cur().Text))
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
		case "record":
			return p.parseRecordDecl(false, "gc", t.Line, t.Col)
		case "byres":
			p.next()
			if isKw(p.cur(), "record") {
				return p.parseRecordDecl(false, "resource", t.Line, t.Col)
			}
			if p.atEnd() {
				p.failTok(p.cur(), "E0105", "unexpected end of file — byres prefixes a record declaration")
			}
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q after byres: byres prefixes record", p.cur().Text))
		case "newtype":
			return p.parseNewtypeDecl(false, t.Line, t.Col)
		case "byval":
			if isKw(p.peek(), "type") {
				p.next() // byval
				return p.parseSumDecl(false, true, t.Line, t.Col)
			}
			if isKw(p.peek(), "pub") {
				p.failTok(p.peek(), "E0105",
					`unexpected token — "pub" after byval: the prefix order is pub, then byval, then the declaration`)
			}
			if isKw(p.peek(), "record") {
				p.next() // byval
				return p.parseRecordDecl(false, "value", t.Line, t.Col)
			}
			if p.peek().Kind == lex.KindEOF {
				p.failTok(p.peek(), "E0105", "unexpected end of file — byval prefixes type or record")
			}
			p.failTok(p.peek(), "E0105",
				fmt.Sprintf("unexpected token — %q after byval: byval prefixes type or record; the prefix order is pub, then byval", p.peek().Text))
		case "type":
			return p.parseSumDecl(false, false, t.Line, t.Col)
		case "interface":
			return p.parseInterfaceDecl(false, t.Line, t.Col)
		case "impl":
			return p.parseImplDecl(t.Line, t.Col)
		case "effect":
			return p.parseEffectDecl(false, t.Line, t.Col)
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

// parseEffectDecl parses chapter 16's `effect name` (the pub bit already
// consumed by the caller). The name joins the module's single name space
// (E0404), refuses the built-in tags io/net/time (E1403 — language-level
// names, not module items), and obeys camelCase (E0012); a collision
// outranks the built-in conflict, which outranks the naming convention.
func (p *parser) parseEffectDecl(pub bool, line, col int) *ast.EffectDecl {
	p.next() // effect
	nt := p.cur()
	if nt.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(nt, "E0105", "unexpected end of file — an effect declaration is effect name")
		}
		p.failTok(nt, "E0105",
			fmt.Sprintf("unexpected token — %q where an effect declaration names its name", nt.Text))
	}
	p.dupCheck(nt)
	if nt.Text == "io" || nt.Text == "net" || nt.Text == "time" {
		p.failTok(nt, "E1403",
			fmt.Sprintf("effect name conflicts with a built-in effect — %q is one of the built-in tags (io, net, time), language-level names no module item may take; choose a camelCase name that is not one of the built-in tags", nt.Text))
	}
	if !isCamel(nt.Text) {
		p.failTok(nt, "E0012",
			fmt.Sprintf("effect names must be camelCase — %q is not camelCase", nt.Text))
	}
	p.names[nt.Text] = nt.Line
	p.next()
	return &ast.EffectDecl{Pub: pub, Name: nt.Text, Line: line, Col: col, NameLine: nt.Line, NameCol: nt.Col}
}

// parseEffectSegment parses chapter 16's declaration segment — the
// `effect` keyword (already verified by the caller) followed by one or
// more effect tags. The tags reach the checker verbatim (resolution is
// the type stage's own face, E1304); the first tag's position is the
// anchor for those diagnostics.
func (p *parser) parseEffectSegment() ([]string, int, int) {
	p.next() // effect
	t := p.cur()
	if t.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(t, "E0105", "unexpected end of file — an effect segment names at least one effect")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q where an effect segment names at least one effect", t.Text))
	}
	line, col := t.Line, t.Col
	return p.effectTagList(), line, col
}

// effectTagList parses the tags of an effect segment: one or more of
// them, each the bare name or the qualified module.name (chapter 15's
// cross-module reach — the importing module spells a pub effect of
// another module qualified). The joined spelling reaches the checker
// verbatim. The dot continuation holds only within one line (chapter 2's
// joining rule). The caller has verified the first identifier is next.
func (p *parser) effectTagList() []string {
	var tags []string
	for p.cur().Kind == lex.KindIdent {
		t := p.cur()
		p.next()
		name := t.Text
		if p.cur().Kind == "." && p.cur().Line == t.Line {
			dot := p.cur()
			p.next()
			nt := p.cur()
			if nt.Kind != lex.KindIdent || nt.Line != dot.Line {
				if p.atEnd() {
					p.failTok(nt, "E0105", "unexpected end of file — a qualified effect tag is module.name")
				}
				p.failTok(nt, "E0105",
					fmt.Sprintf("unexpected token — %q where a qualified effect tag names its effect: a tag is name or module.name", nt.Text))
			}
			name += "." + nt.Text
			p.next()
		}
		tags = append(tags, name)
	}
	return tags
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
	imp.PathLine, imp.PathCol = last.Line, last.Col
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

// parseFnDecl parses `[pub] fn name<T…>(params) [-> type] [where …] block`:
// the chapter 10 generic clause sits between the name and the parameter
// list, the where clause between the return annotation and the body.
// Later-chapter clauses (effect segments, mut parameters) stop at their
// boundaries before any of their own grammar runs.
func (p *parser) parseFnDecl(pub bool, line, col int) *ast.FnDecl {
	p.next() // fn
	d := &ast.FnDecl{Pub: pub, Line: line, Col: col}
	t := p.cur()
	if t.Kind != lex.KindIdent {
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q where a fn name goes: a fn declaration is fn name(params) [-> type] { body }", t.Text))
	}
	p.dupCheck(t)
	p.checkCamel(t)
	d.Name, d.NameLine, d.NameCol = t.Text, t.Line, t.Col
	p.next()
	if p.cur().Kind == "<" {
		d.TypeParams = p.parseTypeParams()
	}
	if p.cur().Kind != "(" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a fn wants its parameter list")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a fn's parameter list opens", p.cur().Text))
	}
	d.Params = p.parseParamList()
	if isKw(p.cur(), "effect") {
		d.EffectTags, d.EffectLine, d.EffectCol = p.parseEffectSegment()
	}
	if p.cur().Kind == "->" {
		p.next()
		d.Ret = p.parseTypeRef()
	}
	if isKw(p.cur(), "where") && !p.brokeLine() {
		d.Where = p.parseWhereClause()
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a fn wants its body block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a fn body block opens", p.cur().Text))
	}
	d.Body = p.parseFnBlock(&fnCtx{name: d.Name, hasRet: d.Ret != nil})
	p.names[d.Name] = d.NameLine
	return d
}

// parseParamList parses `(name: type, …)` with a uniform trailing comma —
// the one parameter-list shape a fn declaration and a full closure share
// (parameters are annotated at every site; they are never inferred). The
// caller has verified the ( is next; this consumes it.
func (p *parser) parseParamList() []ast.Param {
	p.next() // (
	return p.annotatedParams()
}

// annotatedParams parses the annotated parameter entries of an open
// parameter list — everything after its leading ( (or a receiver's
// separating comma) through the closing ).
func (p *parser) annotatedParams() []ast.Param {
	var params []ast.Param
	for {
		if p.cur().Kind == ")" {
			p.next()
			return params
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
		params = append(params, ast.Param{Name: nt.Text, Type: ty, NameLine: nt.Line, NameCol: nt.Col})
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == ")" {
			p.next()
			return params
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a parameter list closes with )")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a parameter list: parameters are name: type pairs", p.cur().Text))
	}
}

// recvParamList parses a method's parameter list — the caller has verified
// the ( is next and passed the method's name for E0801's message. The first
// entry is the receiver: the bare `self` or `mut self` (chapter 10). Zero
// parameters anchor the ( (E0801), an annotated first parameter anchors the
// parameter name (E0801), and any other bare name anchors that name
// (E0802). The receiver lands in the returned RecvKind — Params carries
// only the annotated remainder.
func (p *parser) recvParamList(method string) (ast.RecvKind, []ast.Param) {
	open := p.cur()
	p.next() // (
	if p.cur().Kind == ")" {
		p.failTok(open, "E0801",
			fmt.Sprintf("method declares no receiver — %q declares no parameters; the receiver is the bare parameter \"self\" or \"mut self\"", method))
	}
	var recv ast.RecvKind
	if isKw(p.cur(), "mut") {
		p.next()
		st := p.cur()
		if st.Kind != lex.KindIdent {
			if p.atEnd() {
				p.failTok(st, "E0105", "unexpected end of file — a receiver is the bare parameter self or mut self")
			}
			p.failTok(st, "E0105",
				fmt.Sprintf("unexpected token — %q where the receiver goes: a receiver is the bare parameter self or mut self", st.Text))
		}
		if st.Text != "self" {
			p.failTok(st, "E0802",
				fmt.Sprintf("method receiver must be named self — the receiver is spelled %q; a receiver is \"self\" or \"mut self\"", st.Text))
		}
		recv = ast.RecvMutSelf
		p.next()
	} else {
		st := p.cur()
		if st.Kind != lex.KindIdent {
			if p.atEnd() {
				p.failTok(st, "E0105", "unexpected end of file — a method's parameter list opens with the receiver self or mut self")
			}
			p.failTok(st, "E0105",
				fmt.Sprintf("unexpected token — %q where a method's receiver goes: the first parameter is the bare receiver self or mut self", st.Text))
		}
		if p.peek().Kind == ":" {
			p.failTok(st, "E0801",
				fmt.Sprintf("method declares no receiver — the first parameter of %q carries an annotation; the receiver is the bare parameter \"self\" or \"mut self\"", method))
		}
		if st.Text != "self" {
			p.failTok(st, "E0802",
				fmt.Sprintf("method receiver must be named self — the receiver is spelled %q; a receiver is \"self\" or \"mut self\"", st.Text))
		}
		recv = ast.RecvSelf
		p.next()
	}
	if p.cur().Kind == "," {
		p.next()
		return recv, p.annotatedParams()
	}
	if p.cur().Kind != ")" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a parameter list closes with )")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a parameter list: parameters are name: type pairs", p.cur().Text))
	}
	p.next() // )
	return recv, nil
}

// parseFullClosure parses chapter 12's full closure form in expression
// position: `fn(params) [-> type] block`. Parameters share the fn
// declaration's shape, and the body is a function body block (defer at its
// top level, the loop-depth reset). A closure carries no effect segment
// (chapter 16 R4: its set is inferred from the body, not declared) — the
// `effect` keyword in the body position falls to the block check and is
// refused as E0105 like any other stray token.
func (p *parser) parseFullClosure(t lex.Token) ast.Expr {
	p.next() // fn
	c := &ast.Closure{Line: t.Line, Col: t.Col}
	c.Params = p.parseParamList()
	if p.cur().Kind == "->" {
		p.next()
		c.Ret = p.parseTypeRef()
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a closure wants its body block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a closure's body block opens", p.cur().Text))
	}
	c.Body = p.parseFnBlock(&fnCtx{name: "(closure)", hasRet: c.Ret != nil})
	return c
}

// parseShortClosure parses the short form `|p1, …, pk| body` (k at least
// one — no zero-parameter short closure exists; `||` is the logical-or
// token). Parameters are bare names or name: type pairs; the body is one
// maximal expression normalized to a one-item block. A brace-led body gets
// the function-block reading (fnBrace), the closure-body loop reset holds,
// and shortBody makes break/continue in the maximal body report E0201 —
// the closure reset — ahead of E0202.
func (p *parser) parseShortClosure(t lex.Token) ast.Expr {
	p.next() // |
	c := &ast.Closure{Short: true, Line: t.Line, Col: t.Col}
	if p.cur().Kind == "|" {
		p.failTok(p.cur(), "E0105",
			`unexpected token — "|" fits no production here: no zero-parameter short closure exists; a zero-parameter closure is written in the full form fn() { ... }`)
	}
	for {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a short closure's parameter list closes with |")
		}
		nt := p.cur()
		if nt.Kind != lex.KindIdent {
			p.failTok(nt, "E0105",
				fmt.Sprintf("unexpected token — %q in a short closure's parameter list: parameters are name or name: type pairs", nt.Text))
		}
		p.checkCamel(nt)
		p.next()
		prm := ast.Param{Name: nt.Text, NameLine: nt.Line, NameCol: nt.Col}
		if p.cur().Kind == ":" {
			p.next()
			prm.Type = p.parseTypeRef()
		}
		c.Params = append(c.Params, prm)
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == "|" {
			p.next()
			break
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a short closure's parameter list closes with |")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a short closure's parameter list: parameters are name or name: type pairs", p.cur().Text))
	}
	savedLoop, savedDirect := p.loopDepth, p.direct
	p.loopDepth, p.direct = 0, true
	p.shortBody++
	p.fnBrace = true
	bt := p.cur()
	body := p.parseExpr(valueCtx)
	p.shortBody--
	p.fnBrace = false
	p.loopDepth, p.direct = savedLoop, savedDirect
	c.Body = ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: body, Line: bt.Line, Col: bt.Col}}, Line: t.Line, Col: t.Col}
	return c
}

// operandSlot reports whether a token kind places the next position in an
// operator's operand slot: a binary or prefix operator immediately before
// `|` means the short-closure form is unreachable there — chapter 12 pins
// the parenthesized spelling (as in 1 + (|x| x)).
func operandSlot(kind string) bool {
	if kind == "!" || kind == "~" {
		return true
	}
	_, ok := binLevels[kind]
	return ok
}

// parseSumDecl parses chapter 9's sum declaration: `[pub] [byval] type
// Name<T…> = V1 | … | Vn [derives …]`, each variant bare (a unit variant) or
// carrying 1..8 payload type references. The chapter 10 clauses attach: the
// generic clause after the name, derives after the last variant on the
// declaration's last line. The prefixes are consumed by the caller; line/col
// anchor at the outermost one.
func (p *parser) parseSumDecl(pub, byval bool, line, col int) *ast.SumDecl {
	p.next() // type
	d := &ast.SumDecl{Pub: pub, Byval: byval, Line: line, Col: col}
	t := p.cur()
	if t.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(t, "E0105", "unexpected end of file — a sum declaration is type Name = V1 | … | Vn")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q where the type name goes: a sum declaration is type Name = V1 | … | Vn", t.Text))
	}
	p.dupCheck(t)
	p.checkPascal(t)
	d.Name, d.NameLine, d.NameCol = t.Text, t.Line, t.Col
	p.names[d.Name] = d.NameLine
	p.next()
	if p.cur().Kind == "<" {
		d.TypeParams = p.parseTypeParams()
	}
	if p.cur().Kind != "=" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a sum declaration is type Name = V1 | … | Vn")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where the = of a sum declaration goes", p.cur().Text))
	}
	p.next() // = (a trailing = or | continues per chapter 2's set)
	for {
		vt := p.cur()
		if vt.Kind != lex.KindIdent {
			if p.atEnd() {
				p.failTok(vt, "E0105", "unexpected end of file — a variant list wants at least one variant")
			}
			p.failTok(vt, "E0105",
				fmt.Sprintf("unexpected token — %q where a variant name goes: variants are Name or Name(payload types)", vt.Text))
		}
		p.dupCheck(vt)
		p.checkPascal(vt)
		p.names[vt.Text] = vt.Line
		v := ast.Variant{Name: vt.Text, Line: vt.Line, Col: vt.Col}
		p.next()
		if p.cur().Kind == "(" {
			p.next()
			for {
				if p.cur().Kind == ")" {
					if len(v.Payload) == 0 {
						p.failTok(p.cur(), "E0105",
							"unexpected token — an empty payload list fits no production: a variant is bare, or carries one to eight payload types")
					}
					p.next()
					break
				}
				if p.atEnd() {
					p.failTok(p.cur(), "E0105", "unexpected end of file — a payload list closes with )")
				}
				v.Payload = append(v.Payload, p.parseTypeRef())
				if p.cur().Kind == "," {
					p.next()
					continue
				}
				if p.cur().Kind == ")" {
					p.next()
					break
				}
				p.failTok(p.cur(), "E0105",
					fmt.Sprintf("unexpected token — %q in a payload list: payloads are comma-separated type references and the list closes with )", p.cur().Text))
			}
			if len(v.Payload) > 8 {
				p.failTok(vt, "E0701",
					fmt.Sprintf("variant payload arity above eight — %q declares %d payload types; a record payload names its fields", vt.Text, len(v.Payload)))
			}
		}
		d.Variants = append(d.Variants, v)
		if p.cur().Kind == "|" {
			// A line-starting | after a complete variant ends the
			// declaration; checkStart reports it (E0102).
			if p.brokeLine() {
				return d
			}
			p.next()
			continue
		}
		d.Derives = p.maybeDerives()
		return d
	}
}

// parseRecordDecl parses chapter 8's record declaration: `[pub] [byval |
// byres] record Name<T…> { fields } [derives …]`. The prefixes are consumed
// by the caller; line/col anchor at the outermost one. The field list is
// comma- or newline-separated with a uniform trailing comma — braces are
// brackets, so line breaks inside fold. The chapter 10 clauses attach: the
// generic clause after the name, derives after the closing brace on the
// declaration's last line.
func (p *parser) parseRecordDecl(pub bool, cat string, line, col int) *ast.RecordDecl {
	p.next() // record
	d := &ast.RecordDecl{Pub: pub, Cat: cat, Line: line, Col: col}
	t := p.cur()
	if t.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(t, "E0105", "unexpected end of file — a record declaration is record Name { fields }")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q where the record name goes: a record declaration is record Name { fields }", t.Text))
	}
	p.dupCheck(t)
	p.checkPascal(t)
	d.Name, d.NameLine, d.NameCol = t.Text, t.Line, t.Col
	p.names[d.Name] = d.NameLine
	p.next()
	if p.cur().Kind == "<" {
		d.TypeParams = p.parseTypeParams()
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a record wants its field block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a record's field block opens", p.cur().Text))
	}
	p.next() // {
	p.depth++
	defer func() { p.depth-- }()
	for {
		if p.cur().Kind == "}" {
			p.next()
			d.Derives = p.maybeDerives()
			return d
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a record's field block closes with }")
		}
		ft := p.cur()
		if ft.Kind != lex.KindIdent {
			p.failTok(ft, "E0105",
				fmt.Sprintf("unexpected token — %q where a field name goes: fields are name: type pairs", ft.Text))
		}
		p.checkCamel(ft)
		p.next()
		if p.cur().Kind != ":" {
			p.failTok(ft, "E0105",
				fmt.Sprintf("unexpected token — field %q carries no annotation: fields are name: type; field types are never inferred", ft.Text))
		}
		p.next() // :
		d.Fields = append(d.Fields, ast.FieldDecl{Name: ft.Text, Typ: p.parseTypeRef(), Line: ft.Line, Col: ft.Col})
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == "}" {
			p.next()
			d.Derives = p.maybeDerives()
			return d
		}
		// a newline before the next field name continues the list
		if p.cur().Kind == lex.KindIdent && p.brokeLine() {
			continue
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a record's field block closes with }")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a record field list: fields are name: type pairs separated by commas or line breaks", p.cur().Text))
	}
}

// parseNewtypeDecl parses `newtype Name<T…>(Underlying) [derives …]`
// (chapter 8): a zero-cost wrapper over exactly one type. The chapter 10
// clauses attach: the generic clause after the name, derives after the
// closing paren on the declaration's last line. The prefix is consumed by
// the caller; line/col anchor at the outermost prefix token.
func (p *parser) parseNewtypeDecl(pub bool, line, col int) *ast.NewtypeDecl {
	p.next() // newtype
	d := &ast.NewtypeDecl{Pub: pub, Line: line, Col: col}
	t := p.cur()
	if t.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(t, "E0105", "unexpected end of file — a newtype declaration is newtype Name(Type)")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q where the newtype name goes: a newtype declaration is newtype Name(Type)", t.Text))
	}
	p.dupCheck(t)
	p.checkPascal(t)
	d.Name, d.NameLine, d.NameCol = t.Text, t.Line, t.Col
	p.names[d.Name] = d.NameLine
	p.next()
	if p.cur().Kind == "<" {
		d.TypeParams = p.parseTypeParams()
	}
	if p.cur().Kind != "(" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a newtype wraps its underlying type in (…)")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a newtype's underlying type opens: a newtype wraps exactly one underlying type", p.cur().Text))
	}
	p.next() // (
	d.Underlying = p.parseTypeRef()
	if p.cur().Kind != ")" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a newtype closes with )")
		}
		p.failTok(p.cur(), "E0105", "a newtype wraps exactly one underlying type — the wrapped list takes no comma and closes with )")
	}
	p.next() // )
	d.Derives = p.maybeDerives()
	return d
}

// --- chapter 10: interfaces, impls, and their clauses -------------------------

// parseTypeParams parses a declaration's generic parameter clause
// `<T1, …, Tk>` (chapter 10), k one to eight, each parameter a PascalCase
// name (E0011 — the clause is the one place a lowercase type name never
// reaches). The caller has verified the < is next; this consumes it. Above
// eight parameters the clause's own < anchors E0825 — checked after the
// parse so the count can be named.
func (p *parser) parseTypeParams() []*ast.TypeParam {
	open := p.cur() // <
	p.next()
	if p.cur().Kind == ">" {
		p.failTok(p.cur(), "E0105",
			`unexpected token — ">" opens an empty generic parameter clause: a clause holds one to eight names`)
	}
	var tps []*ast.TypeParam
	for {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a generic parameter clause closes with >")
		}
		nt := p.cur()
		if nt.Kind != lex.KindIdent {
			p.failTok(nt, "E0105",
				fmt.Sprintf("unexpected token — %q in a generic parameter clause: the parameters are PascalCase names", nt.Text))
		}
		p.checkPascal(nt)
		tps = append(tps, &ast.TypeParam{Name: nt.Text, Line: nt.Line, Col: nt.Col})
		p.next()
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.closeAngle() {
			break
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a generic parameter clause closes with >")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a generic parameter clause: the clause closes with >", p.cur().Text))
	}
	if len(tps) > 8 {
		p.failTok(open, "E0825",
			fmt.Sprintf("generic parameter count above eight — the clause declares %d parameters; a clause carries at most eight", len(tps)))
	}
	return tps
}

// parseWhereClause parses `where b1, b2, …` (chapter 10): comma-separated
// constraints, one WhereBound each — either the bound form `T: Iface [+ …]`
// or the equality form `T.Assoc == Type`. The caller has verified the where
// keyword trails its declaration on the same line; this consumes it.
func (p *parser) parseWhereClause() []*ast.WhereBound {
	p.next() // where
	var wbs []*ast.WhereBound
	for {
		wbs = append(wbs, p.parseBoundRef())
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		return wbs
	}
}

// parseBoundRef parses one where constraint: `Subject: Iface [+ Iface2 …]`
// or `Subject.Assoc == TypeRef`. One WhereBound holds one form — the bound
// list or the equality, never both.
func (p *parser) parseBoundRef() *ast.WhereBound {
	st := p.cur()
	if st.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(st, "E0105", "unexpected end of file — a where constraint is T: Iface or T.Assoc == Type")
		}
		p.failTok(st, "E0105",
			fmt.Sprintf("unexpected token — %q where a where subject goes: a constraint is T: Iface or T.Assoc == Type", st.Text))
	}
	p.next()
	wb := &ast.WhereBound{Subject: st.Text, Line: st.Line, Col: st.Col}
	if p.cur().Kind == "." {
		p.next() // .
		at := p.cur()
		if at.Kind != lex.KindIdent {
			if p.atEnd() {
				p.failTok(at, "E0105", "unexpected end of file — an equality constraint is T.Assoc == Type")
			}
			p.failTok(at, "E0105",
				fmt.Sprintf("unexpected token — %q where the associated type goes: an equality constraint is T.Assoc == Type", at.Text))
		}
		p.next()
		if p.cur().Kind != "==" {
			if p.atEnd() {
				p.failTok(p.cur(), "E0105", "unexpected end of file — an equality constraint is T.Assoc == Type")
			}
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q where == goes: an equality constraint is T.Assoc == Type", p.cur().Text))
		}
		eq := p.cur()
		p.next() // ==
		wb.Eq = append(wb.Eq, &ast.TypeEq{Assoc: at.Text, RHS: p.parseTypeRef(), Line: eq.Line, Col: eq.Col})
		return wb
	}
	if p.cur().Kind != ":" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a where constraint is T: Iface or T.Assoc == Type")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a constraint's : or == goes: a constraint is T: Iface or T.Assoc == Type", p.cur().Text))
	}
	p.next() // :
	for {
		it := p.cur()
		if it.Kind != lex.KindIdent || !isPascal(it.Text) {
			if p.atEnd() {
				p.failTok(it, "E0105", "unexpected end of file — a bound names an interface: T: Iface [+ Iface2 …]")
			}
			p.failTok(it, "E0105",
				fmt.Sprintf("unexpected token — %q where a bound goes: a bound names an interface, T: Iface [+ Iface2 …]", it.Text))
		}
		p.next()
		nt := &ast.NamedType{Name: it.Text, Line: it.Line, Col: it.Col}
		nt.Args = p.genericArgs()
		wb.Ifaces = append(wb.Ifaces, nt)
		if p.cur().Kind == "+" {
			p.next()
			continue
		}
		return wb
	}
}

// maybeDerives consumes a derives clause when one trails a declaration's
// last line: `derives Eq[, Hash][, Show]` — the closed target set,
// comma-separated, duplicate-free (chapter 10). A line-starting derives
// after a complete declaration is no clause; the keyword stays for the next
// item position (E0105 there). Each violation anchors its target name.
func (p *parser) maybeDerives() *ast.DerivesClause {
	if !isKw(p.cur(), "derives") || p.brokeLine() {
		return nil
	}
	t := p.cur()
	p.next() // derives
	dc := &ast.DerivesClause{Line: t.Line, Col: t.Col}
	seen := map[string]bool{}
	for {
		nt := p.cur()
		if nt.Kind != lex.KindIdent {
			if p.atEnd() {
				p.failTok(nt, "E0105", "unexpected end of file — a derives clause is derives Eq, Hash, Show")
			}
			p.failTok(nt, "E0105",
				fmt.Sprintf("unexpected token — %q in a derives clause: the clause is derives followed by the bare names Eq, Hash, Show", nt.Text))
		}
		p.next()
		switch {
		case nt.Text == "Eq" || nt.Text == "Hash" || nt.Text == "Show":
			if seen[nt.Text] {
				p.failTok(nt, "E0824",
					fmt.Sprintf("unknown or duplicate derive target — %q appears twice in the clause; each target appears at most once", nt.Text))
			}
			seen[nt.Text] = true
		case nt.Text == "Shareable":
			// the one named non-target: a where bound the clause shape
			// invites but the chapter does not take
			p.failTok(nt, "E0824",
				`unknown or duplicate derive target — "Shareable" is not a derive target; it is a where bound, not a clause target`)
		default:
			p.failTok(nt, "E0824",
				fmt.Sprintf("unknown or duplicate derive target — %q is not a derive target; the clause takes \"Eq\", \"Hash\", and \"Show\"", nt.Text))
		}
		dc.Targets = append(dc.Targets, nt.Text)
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		return dc
	}
}

// parseInterfaceDecl parses `[pub] interface Name<T…> { items }` (chapter
// 10). Items stand one per line with no separator token: associated-type
// holes and method signatures, the latter optionally with a default body.
// Interface members carry no pub of their own, and a where clause trails no
// interface declaration — only a fn declaration's signature and an impl
// head. The prefixes are consumed by the caller; line/col anchor at the
// outermost one.
func (p *parser) parseInterfaceDecl(pub bool, line, col int) *ast.InterfaceDecl {
	p.next() // interface
	d := &ast.InterfaceDecl{Pub: pub, Line: line, Col: col}
	t := p.cur()
	if t.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(t, "E0105", "unexpected end of file — an interface declaration is interface Name { items }")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q where the interface name goes: an interface declaration is interface Name { items }", t.Text))
	}
	p.dupCheck(t)
	p.checkPascal(t)
	d.Name, d.NameLine, d.NameCol = t.Text, t.Line, t.Col
	p.names[d.Name] = d.NameLine
	p.next()
	if p.cur().Kind == "<" {
		d.TypeParams = p.parseTypeParams()
	}
	if isKw(p.cur(), "where") {
		p.failTok(p.cur(), "E0105",
			`unexpected token — "where" fits no production here: a where clause trails a fn signature or an impl head, not an interface declaration`)
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — an interface wants its item block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where an interface's item block opens", p.cur().Text))
	}
	p.next() // {
	p.depth++
	defer func() { p.depth-- }()
	// An interface's method names share the interface's own member space,
	// not the module's (chapter 10: only the interface's name joins the
	// module's one name space) — two interfaces may declare one method
	// name (their collision is E0814's, at the impl); a second declaration
	// inside one interface is E0404.
	memberNames := map[string]int{}
	for {
		if p.cur().Kind == "}" {
			p.next()
			return d
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — an interface's item block closes with }")
		}
		switch {
		case isKw(p.cur(), "type"):
			kw := p.cur()
			a := p.parseAssocDecl()
			// E0803 after the parse so the count and the name can both be
			// named; the anchor stays the hole's own type keyword.
			if len(d.Assocs) >= 4 {
				p.failTok(kw, "E0803",
					fmt.Sprintf("associated type count above four — %q declares a fifth associated type %q; an interface declares at most four", d.Name, a.Name))
			}
			d.Assocs = append(d.Assocs, a)
		case isKw(p.cur(), "fn"):
			d.Methods = append(d.Methods, p.parseMethodSig(memberNames))
		default:
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q fits no interface item production: an interface declares associated types and methods only", p.cur().Text))
		}
		if p.cur().Kind == "}" {
			p.next()
			return d
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — an interface's item block closes with }")
		}
		if !p.brokeLine() {
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q fits no production: interface items are separated by newlines and carry no separator token", p.cur().Text))
		}
	}
}

// parseAssocDecl parses one associated-type hole of an interface: `type
// Name` (chapter 10). A hole carries no bound on its declaration — bounds
// live in where clauses (E0804, at the bound's first token).
func (p *parser) parseAssocDecl() *ast.AssocDecl {
	p.next() // type
	nt := p.cur()
	if nt.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(nt, "E0105", "unexpected end of file — an associated type is type Name")
		}
		p.failTok(nt, "E0105",
			fmt.Sprintf("unexpected token — %q where the associated type's name goes: a hole is type Name", nt.Text))
	}
	p.checkPascal(nt)
	p.next()
	if p.cur().Kind == ":" {
		p.next() // :
		p.failTok(p.cur(), "E0804",
			fmt.Sprintf("associated type declares an upper bound — the associated type %q carries an upper bound; bounds belong in where clauses, not on the hole's declaration", nt.Text))
	}
	return &ast.AssocDecl{Name: nt.Text, Line: nt.Line, Col: nt.Col}
}

// parseMethodSig parses one interface method item: `fn name<T…>(recv,
// params…) [-> type]` or the same with a default body block (chapter 10).
// Signature names share the enclosing interface's member space (E0404 at
// the second of two in one interface); a where clause trails no method
// signature — only a fn declaration's signature and an impl head.
func (p *parser) parseMethodSig(memberNames map[string]int) ast.MethodSig {
	p.next() // fn
	var m ast.MethodSig
	t := p.cur()
	if t.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(t, "E0105", "unexpected end of file — a method signature is fn name(self, params…) [-> type]")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q where the method name goes: a method signature is fn name(self, params…) [-> type]", t.Text))
	}
	if line, dup := memberNames[t.Text]; dup {
		p.failTok(t, "E0404",
			fmt.Sprintf("duplicate name in one module — %q is already declared at line %d; one module has one name space", t.Text, line))
	}
	memberNames[t.Text] = t.Line
	p.checkCamel(t)
	m.Name, m.NameLine, m.NameCol = t.Text, t.Line, t.Col
	p.next()
	if p.cur().Kind == "<" {
		m.TypeParams = p.parseTypeParams()
	}
	if p.cur().Kind != "(" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a method's parameter list opens with the receiver")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a method's parameter list opens: the first parameter is the receiver self or mut self", p.cur().Text))
	}
	m.Recv, m.Params = p.recvParamList(m.Name)
	if isKw(p.cur(), "effect") {
		m.EffectTags, m.EffectLine, m.EffectCol = p.parseEffectSegment()
	}
	if p.cur().Kind == "->" {
		p.next()
		m.Ret = p.parseTypeRef()
		m.HasRet = true
	}
	if isKw(p.cur(), "where") {
		p.failTok(p.cur(), "E0105",
			`unexpected token — "where" fits no production here: a where clause trails a fn declaration's signature or an impl head, not a method signature`)
	}
	if p.cur().Kind == "{" {
		b := p.parseFnBlock(&fnCtx{name: m.Name, hasRet: m.Ret != nil})
		m.Body = &b
	}
	return m
}

// parseImplDecl parses `impl<T…> [Iface for] Head [where …] { items }`
// (chapter 10): the for-form implements an interface for a head type, the
// bare form is an inherent impl (Iface nil). Iface and Head are full type
// references — a tuple head and a bare generic-parameter head parse here;
// head nominality is the checker's E0811. Items are associated-type
// bindings (all before any method — E0806) and method definitions, one per
// line; an impl block itself carries no pub (the pub dispatch rejects it).
func (p *parser) parseImplDecl(line, col int) *ast.ImplDecl {
	p.next() // impl
	d := &ast.ImplDecl{Line: line, Col: col}
	if p.cur().Kind == "<" {
		d.TypeParams = p.parseTypeParams()
	}
	d.Head = p.parseTypeRef()
	if isKw(p.cur(), "for") {
		d.Iface = d.Head
		p.next() // for
		d.Head = p.parseTypeRef()
	}
	if isKw(p.cur(), "where") && !p.brokeLine() {
		d.Where = p.parseWhereClause()
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — an impl wants its item block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where an impl's item block opens", p.cur().Text))
	}
	p.next() // {
	p.depth++
	defer func() { p.depth-- }()
	for {
		if p.cur().Kind == "}" {
			p.next()
			return d
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — an impl's item block closes with }")
		}
		switch {
		case isKw(p.cur(), "type"):
			kw := p.cur()
			b := p.parseAssocBinding()
			// E0806 after the parse so the binding's name can be named; the
			// anchor stays the binding's own type keyword, and the message
			// names the method the binding follows (the last one).
			if len(d.Methods) > 0 {
				p.failTok(kw, "E0806",
					fmt.Sprintf("associated type binding after a method definition — the binding of %q follows the method %q; associated-type bindings precede methods in an impl", b.Name, d.Methods[len(d.Methods)-1].Name))
			}
			d.Assocs = append(d.Assocs, b)
		case isKw(p.cur(), "fn"), isKw(p.cur(), "pub"):
			d.Methods = append(d.Methods, p.parseImplMethod())
		default:
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q fits no impl item production: an impl defines associated-type bindings and methods", p.cur().Text))
		}
		if p.cur().Kind == "}" {
			p.next()
			return d
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — an impl's item block closes with }")
		}
		if !p.brokeLine() {
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q fits no production: impl items are separated by newlines and carry no separator token", p.cur().Text))
		}
	}
}

// parseAssocBinding parses one `type Name = TypeRef` of an impl: its
// binding of an interface associated type (chapter 10). Bindings precede
// every method definition (E0806 is parseImplDecl's, at the keyword).
func (p *parser) parseAssocBinding() *ast.AssocBinding {
	p.next() // type
	nt := p.cur()
	if nt.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(nt, "E0105", "unexpected end of file — an associated-type binding is type Name = Type")
		}
		p.failTok(nt, "E0105",
			fmt.Sprintf("unexpected token — %q where the binding's name goes: a binding is type Name = Type", nt.Text))
	}
	p.checkPascal(nt)
	p.next()
	if p.cur().Kind != "=" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — an associated-type binding is type Name = Type")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a binding's = goes: a binding is type Name = Type", p.cur().Text))
	}
	p.next() // =
	return &ast.AssocBinding{Name: nt.Text, Type: p.parseTypeRef(), Line: nt.Line, Col: nt.Col}
}

// parseImplMethod parses one method definition of an impl body: `[pub] fn
// name<T…>(recv, params…) [-> type] block` (chapter 10) — the node is the
// fn declaration's, with the receiver carried by Recv. The optional pub is
// the method's own visibility (an impl block carries no pub; its methods
// may). Method names stay out of the module's one name space — they live in
// the impl's member set — and a where clause trails no method definition.
func (p *parser) parseImplMethod() *ast.FnDecl {
	t := p.cur()
	pub := false
	if isKw(t, "pub") {
		pub = true
		p.next()
		if !isKw(p.cur(), "fn") {
			if p.atEnd() {
				p.failTok(p.cur(), "E0105", "unexpected end of file — an impl method definition is [pub] fn name(self, params…) block")
			}
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q after pub: an impl method definition is [pub] fn name(self, params…) block", p.cur().Text))
		}
	}
	line, col := t.Line, t.Col
	p.next() // fn
	d := &ast.FnDecl{Pub: pub, Line: line, Col: col}
	nt := p.cur()
	if nt.Kind != lex.KindIdent {
		if p.atEnd() {
			p.failTok(nt, "E0105", "unexpected end of file — an impl method definition is [pub] fn name(self, params…) block")
		}
		p.failTok(nt, "E0105",
			fmt.Sprintf("unexpected token — %q where the method name goes: an impl method definition is [pub] fn name(self, params…) block", nt.Text))
	}
	p.checkCamel(nt)
	d.Name, d.NameLine, d.NameCol = nt.Text, nt.Line, nt.Col
	p.next()
	if p.cur().Kind == "<" {
		d.TypeParams = p.parseTypeParams()
	}
	if p.cur().Kind != "(" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a method's parameter list opens with the receiver")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a method's parameter list opens: the first parameter is the receiver self or mut self", p.cur().Text))
	}
	d.Recv, d.Params = p.recvParamList(d.Name)
	if isKw(p.cur(), "effect") {
		d.EffectTags, d.EffectLine, d.EffectCol = p.parseEffectSegment()
	}
	if p.cur().Kind == "->" {
		p.next()
		d.Ret = p.parseTypeRef()
	}
	if isKw(p.cur(), "where") {
		p.failTok(p.cur(), "E0105",
			`unexpected token — "where" fits no production here: a where clause trails a fn declaration's signature or an impl head, not a method definition`)
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a method definition wants its body block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a method definition's body block opens", p.cur().Text))
	}
	d.Body = p.parseFnBlock(&fnCtx{name: d.Name, hasRet: d.Ret != nil})
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

// parseBlock parses `{ items }` — a plain block (a nested block, a
// control-form body, an expression block, a defer body): not the function
// body's top level, so defer is not a legal item here. fctx non-nil marks a
// fn context for return checking. Block braces are not brackets — line
// breaks inside stay significant; only the parenthesis regions go
// insensitive.
func (p *parser) parseBlock(fctx *fnCtx) ast.Block {
	return p.block(fctx, false)
}

// parseFnBlock parses a function body's top-level block (a fn declaration's
// or a closure's): defer is legal at its top level (E0204's one level) and
// the loop depth starts at zero — a closure inside a loop is a function
// body, not a loop body.
func (p *parser) parseFnBlock(fctx *fnCtx) ast.Block {
	savedLoop, savedDirect := p.loopDepth, p.direct
	p.loopDepth, p.direct = 0, true
	b := p.block(fctx, true)
	p.loopDepth, p.direct = savedLoop, savedDirect
	return b
}

// block is the shared `{ items }` loop; direct names whether this block is
// a function body's top level.
func (p *parser) block(fctx *fnCtx, direct bool) ast.Block {
	open := p.cur()
	p.next()
	if fctx != nil {
		p.fns = append(p.fns, *fctx)
		defer func() { p.fns = p.fns[:len(p.fns)-1] }()
	}
	savedDirect := p.direct
	p.direct = direct
	b := ast.Block{Line: open.Line, Col: open.Col}
	for {
		if p.cur().Kind == "}" {
			p.next()
			p.direct = savedDirect
			return b
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a block wants its closing }")
		}
		p.checkStart(false)
		b.Items = append(b.Items, p.parseStmt())
		if p.cur().Kind == "}" {
			p.next()
			p.direct = savedDirect
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
		case "while":
			return p.parseWhile()
		case "loop":
			return p.parseLoop()
		case "break", "continue":
			return p.parseBreakCont()
		case "defer":
			return p.parseDefer()
		case "if", "match", "fn", "true", "false":
			// keyword-led expression forms; in statement position they
			// wrap in an expression statement (chapter 2)
		case "for":
			return p.parseFor()
		case "scope":
			return p.parseScopeRes()
		case "task", "select":
			p.bnd(bndConc)
		case "mock":
			p.bnd(bndTest)
		case "pub", "import", "as", "mut", "else", "in", "where", "derives",
			"with", "resource", "effect", "case", "timeout", "collectAll",
			"record", "byval", "byres", "newtype", "type", "interface",
			"impl", "foreign", "test":
			p.failTok(t, "E0105",
				fmt.Sprintf("unexpected token — %q fits no statement production: statements are let|var bindings, name assignments, return, chapter 3's control statements, and expression statements", t.Text))
		}
	}
	// `self.field = expr` — chapter 10's one field-write form. The receiver
	// self is an ordinary binding (no keyword), so the shape is probed at the
	// token level ahead of the plain-name assignment below: a member target
	// is otherwise not an assignment head at all. The probe bounds-checks
	// because the token slice ends at its single eof token.
	if t.Kind == lex.KindIdent && t.Text == "self" && p.pos+3 < len(p.toks) &&
		p.toks[p.pos+1].Kind == "." && p.toks[p.pos+2].Kind == lex.KindIdent &&
		p.toks[p.pos+3].Kind == "=" && p.toks[p.pos+3].Line == t.Line {
		p.next() // self
		p.next() // .
		ft := p.cur()
		p.next() // field
		op := p.cur()
		p.next() // =
		v := p.parseExpr(valueCtx)
		return &ast.Assign{Name: "self", Field: ft.Text, Value: v, Line: t.Line, Col: t.Col, OpLine: op.Line, OpCol: op.Col}
	}
	if t.Kind == lex.KindIdent && p.peek().Kind == "=" && p.peek().Line == t.Line {
		p.next() // name
		op := p.cur()
		p.next() // =
		v := p.parseExpr(valueCtx)
		return &ast.Assign{Name: t.Text, Value: v, Line: t.Line, Col: t.Col, OpLine: op.Line, OpCol: op.Col}
	}
	e := p.parseExpr(stmtCtx)
	return &ast.ExprStmt{Expr: e, Line: t.Line, Col: t.Col}
}

// parseBinding parses `let|var name [: type] = expr` (name `_` discards)
// or `let|var (p1, …, pn) [: type] = expr` with an irrefutable tuple
// pattern (chapter 8's let destructure).
func (p *parser) parseBinding() *ast.Binding {
	kw := p.cur()
	p.next()
	b := &ast.Binding{Kw: kw.Text, Line: kw.Line, Col: kw.Col}
	t := p.cur()
	switch {
	case t.Kind == lex.KindIdent && isPascal(t.Text) && p.peek().Kind == "(":
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q fits no binding name production: refutable patterns are match-only; the let name position takes an identifier or an irrefutable tuple pattern", t.Text))
	case t.Kind == lex.KindIdent:
		p.checkCamel(t)
		b.Name, b.NameLine, b.NameCol = t.Text, t.Line, t.Col
		p.next()
	case t.Kind == "_":
		b.Name, b.NameLine, b.NameCol = "_", t.Line, t.Col
		p.next()
	case t.Kind == "(":
		b.Pat = p.parseLetTuple(t)
	default:
		if t.Kind == lex.KindEOF {
			p.failTok(t, "E0105", "unexpected end of file — a binding wants let|var name [: type] = expr")
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q fits no binding name production: the let name position takes an identifier or an irrefutable tuple pattern", t.Text))
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
	saved := p.valSite
	p.valSite = "a let initializer"
	b.Init = p.parseExpr(valueCtx)
	p.valSite = saved
	return b
}

// parseLetTuple parses a let head's irrefutable tuple pattern: two or more
// elements of bindings, wildcards, and nested tuples — variant, literal,
// and or-patterns are match-only (chapter 4).
func (p *parser) parseLetTuple(open lex.Token) ast.Pattern {
	p.next()
	if p.cur().Kind == ")" {
		p.failTok(open, "E0105",
			`unexpected token — "()" fits no binding name production: the let position takes an identifier or an irrefutable tuple pattern`)
	}
	tt := &ast.PatTuple{Line: open.Line, Col: open.Col}
	for {
		tt.Elems = append(tt.Elems, p.parseIrrefutable())
		if p.cur().Kind == "," {
			p.next()
			if p.cur().Kind == ")" {
				p.next()
				break
			}
			continue
		}
		if p.cur().Kind == ")" {
			if len(tt.Elems) < 2 {
				p.failTok(open, "E0105",
					`unexpected token — "(" fits no binding name production: a let tuple pattern holds two or more elements`)
			}
			p.next()
			break
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a let tuple pattern closes with )")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q fits no irrefutable pattern production: the let position takes bindings, wildcards, and tuples of them", p.cur().Text))
	}
	p.checkDupBinds(tt)
	return tt
}

// parseIrrefutable parses one element of a let tuple pattern.
func (p *parser) parseIrrefutable() ast.Pattern {
	t := p.cur()
	switch {
	case t.Kind == "_":
		p.next()
		return &ast.PatWildcard{Line: t.Line, Col: t.Col}
	case t.Kind == lex.KindIdent && !isPascal(t.Text):
		p.checkCamel(t)
		p.next()
		return &ast.PatBinding{Name: t.Text, Line: t.Line, Col: t.Col}
	case t.Kind == "(":
		return p.parseLetTuple(t)
	case t.Kind == lex.KindIdent:
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q fits no irrefutable pattern production: refutable patterns are match-only; the let position takes bindings, wildcards, and tuples of them", t.Text))
	}
	if t.Kind == lex.KindEOF {
		p.failTok(t, "E0105", "unexpected end of file — a let tuple pattern holds bindings, wildcards, and nested tuples")
	}
	p.failTok(t, "E0105",
		fmt.Sprintf("unexpected token — %q fits no irrefutable pattern production: the let position takes bindings, wildcards, and tuples of them", t.Text))
	panic("unreachable")
}

// parseReturn parses `return` / `return expr`. Outside any fn body it is
// E0401; a value where the fn declares none is E0402 (a bare return in a
// fn with a declared type parses — exhaustiveness is the types stage's).
func (p *parser) parseReturn() *ast.Return {
	t := p.cur()
	p.next()
	hasValue := !p.atEnd() && p.cur().Kind != "}" && !(p.depth == 0 && p.brokeLine())
	if len(p.fns) == 0 || p.fns[len(p.fns)-1].deferBody {
		p.failTok(t, "E0401",
			"return outside a function body — no function context encloses this return; a defer body runs at function exit and carries no return of its own")
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

// --- chapter 3 statements -----------------------------------------------------

// headExpr parses a control-flow head (a condition or scrutinee): a value
// expression under chapter 2's line rules, with the postfix `{` suspended —
// the brace that follows a head opens the body, never a construction.
func (p *parser) headExpr() ast.Expr {
	p.noBrace++
	e := p.parseExpr(valueCtx)
	p.noBrace--
	return e
}

// parseWhile parses `while cond block`; the body is a loop body for the
// break/continue depth.
func (p *parser) parseWhile() *ast.While {
	t := p.cur()
	p.next()
	cond := p.headExpr()
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a while wants its body block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a while body block opens", p.cur().Text))
	}
	p.loopDepth++
	body := p.parseBlock(nil)
	p.loopDepth--
	return &ast.While{Cond: cond, Body: body, Line: t.Line, Col: t.Col}
}

// parseLoop parses `loop block`.
func (p *parser) parseLoop() *ast.Loop {
	t := p.cur()
	p.next()
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a loop wants its body block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a loop body block opens", p.cur().Text))
	}
	p.loopDepth++
	body := p.parseBlock(nil)
	p.loopDepth--
	return &ast.Loop{Body: body, Line: t.Line, Col: t.Col}
}

// --- chapter 5: the for statement (over chapter 11's iterables) ----------------

// parseFor parses `for pat in expr block` (chapter 5): the head pattern is
// irrefutable — a binding, the wildcard, or a tuple of them, the same
// grammar a let head takes — and the iterated expression is chapter 11's
// Iterable value (a control-flow head: the postfix `{` opens the body).
// The body is a loop body for the break/continue depth.
func (p *parser) parseFor() *ast.ForStmt {
	t := p.cur()
	p.next() // for
	pat := p.forHead()
	if !isKw(p.cur(), "in") {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a for head is pat in iterable")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a for head's in goes: a for head is pat in iterable", p.cur().Text))
	}
	p.next() // in
	iter := p.headExpr()
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a for wants its body block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a for body block opens", p.cur().Text))
	}
	p.loopDepth++
	body := p.parseBlock(nil)
	p.loopDepth--
	return &ast.ForStmt{Pat: pat, Iter: iter, Body: body, Line: t.Line, Col: t.Col}
}

// forHead parses a for head's irrefutable pattern: a binding name, the
// wildcard, or a tuple of them — chapter 8's let-head grammar is the one
// authority for irrefutable patterns, so tuple heads reuse it. A refutable
// shape anchors its own first token.
func (p *parser) forHead() ast.Pattern {
	t := p.cur()
	switch {
	case t.Kind == "_":
		p.next()
		return &ast.PatWildcard{Line: t.Line, Col: t.Col}
	case t.Kind == lex.KindIdent && !isPascal(t.Text):
		p.checkCamel(t)
		p.next()
		return &ast.PatBinding{Name: t.Text, Line: t.Line, Col: t.Col}
	case t.Kind == "(":
		return p.parseLetTuple(t)
	}
	if t.Kind == lex.KindEOF {
		p.failTok(t, "E0105", "unexpected end of file — the \"for\" head takes an irrefutable pattern: a binding, the wildcard, or a tuple of them")
	}
	p.failTok(t, "E0105",
		fmt.Sprintf("unexpected token — %q fits no production here: the \"for\" head takes an irrefutable pattern, a binding, the wildcard, or a tuple of them; refutable patterns are match-only", t.Text))
	panic("unreachable")
}

// parseBreakCont parses bare break/continue; outside any loop body they are
// E0201 (loop depth is a pure position fact — a closure body resets it).
func (p *parser) parseBreakCont() ast.Stmt {
	t := p.cur()
	p.next()
	if p.loopDepth == 0 {
		p.failLoopCtrl(t)
	}
	if t.Text == "break" {
		return &ast.Break{Line: t.Line, Col: t.Col}
	}
	return &ast.Continue{Line: t.Line, Col: t.Col}
}

// failLoopCtrl reports E0201 at a break/continue keyword — the shared
// report for the statement form and the value-position form inside a short
// closure's maximal body (the closure-body loop reset makes that position
// E0201's, not E0202's).
func (p *parser) failLoopCtrl(t lex.Token) {
	p.failTok(t, "E0201",
		fmt.Sprintf("break or continue outside a loop — %q stands in a block that is not inside a loop body; bare break and continue control the innermost enclosing while or loop", t.Text))
}

// parseDefer parses `defer block` (E0203) as a direct item of a function
// body's block (E0204). The body is a plain block in a no-return context:
// returns inside it are E0401, whatever function encloses the defer.
func (p *parser) parseDefer() *ast.Defer {
	t := p.cur()
	p.next()
	if !p.direct {
		p.failTok(t, "E0204",
			"defer placement — defer is not a direct item of the function body's block; defer runs at function exit in reverse order and needs the single exit-point placement")
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — defer takes a block: defer { … }")
		}
		p.failTok(t, "E0203",
			"defer body must be a block — the operand is an expression, not a block; the single ratified shape is defer { ... }")
	}
	blk := p.parseBlock(&fnCtx{deferBody: true})
	return &ast.Defer{Block: blk, Line: t.Line, Col: t.Col}
}

// parseScopeRes parses `scope resource(name = expr, …) block` (chapter 13).
// Any other `scope` shape — bare, timeout, collectAll — is chapter 18's and
// keeps its own honest boundary row. Head names are block bindings:
// camelCase (E0012) and duplicates (E0404, the pattern-binding variant's
// wording) keep their existing codes. The body is a plain nested block:
// loop depth persists (break/continue punch through scope blocks) and defer
// inside is not at a function body's top level (E0204's existing check).
func (p *parser) parseScopeRes() ast.Stmt {
	t := p.cur()
	p.next() // scope
	if !isKw(p.cur(), "resource") {
		p.bnd(bndConcScope)
	}
	p.next() // resource
	if p.cur().Kind != "(" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a scope resource head is scope resource(name = expr, …) { … }")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where \"(\" opens the scope resource binding list: a scope resource head is scope resource(name = expr, …) { … }", p.cur().Text))
	}
	p.next() // (
	// The binding list is a paren region — line breaks fold inside and
	// a head expression's construction braces open (the noBrace
	// suspension binds only at depth zero).
	p.depth++
	defer func() { p.depth-- }()
	var binds []ast.ScopeBind
	seen := map[string]int{}
	for {
		nt := p.cur()
		if nt.Kind != lex.KindIdent && nt.Kind != "_" {
			if p.atEnd() {
				p.failTok(nt, "E0105", "unexpected end of file — a scope resource binding is name = expr")
			}
			p.failTok(nt, "E0105",
				fmt.Sprintf("unexpected token — %q where a scope resource binding's name goes: a binding is name = expr", nt.Text))
		}
		if nt.Kind == "_" {
			p.failTok(nt, "E0105",
				`unexpected token — "_" fits no scope resource binding: each head binding takes over a handle and names it`)
		}
		p.checkCamel(nt)
		if line, dup := seen[nt.Text]; dup {
			p.failTok(nt, "E0404",
				fmt.Sprintf("duplicate name in one module — the scope resource head binds %q twice, first at line %d; a head binds each name at most once", nt.Text, line))
		}
		seen[nt.Text] = nt.Line
		p.next()
		if p.cur().Kind != "=" {
			if p.atEnd() {
				p.failTok(p.cur(), "E0105", "unexpected end of file — a scope resource binding is name = expr")
			}
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q where a scope resource binding's = goes: a binding is name = expr", p.cur().Text))
		}
		p.next() // =
		val := p.headExpr()
		binds = append(binds, ast.ScopeBind{Name: nt.Text, Val: val, Line: nt.Line, Col: nt.Col})
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == ")" {
			p.next()
			break
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — the scope resource binding list closes with )")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where the scope resource binding list closes with )", p.cur().Text))
	}
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a scope resource wants its body block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a scope resource body block opens", p.cur().Text))
	}
	body := p.parseBlock(nil)
	return &ast.ScopeRes{Binds: binds, Body: body, Line: t.Line, Col: t.Col}
}

// --- chapter 3's if and chapter 4's match (expressions) ------------------------

// valueSite names the enclosing value position for E0202's message; the
// default covers every value position the parser does not name specially.
func (p *parser) valueSite() string {
	if p.valSite == "" {
		return "this value position"
	}
	return p.valSite
}

// parseIf parses `if cond block [else block | if]` (chapter 3). An else-if
// chain nests in the else position; `else { if … }` folds to the same
// shape. Without else the form produces no value — in a value position it
// is E0202 (the statement position is legal).
func (p *parser) parseIf(ctx exprCtx) *ast.If {
	t := p.cur()
	p.next()
	cond := p.headExpr()
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — an if wants its then block")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where an if's then block opens", p.cur().Text))
	}
	node := &ast.If{Cond: cond, Then: p.parseBlock(nil), Line: t.Line, Col: t.Col}
	if isKw(p.cur(), "else") && !p.brokeLine() {
		p.next()
		if isKw(p.cur(), "if") {
			node.Else = p.parseIf(valueCtx)
			return node
		}
		if p.cur().Kind != "{" {
			if p.atEnd() {
				p.failTok(p.cur(), "E0105", "unexpected end of file — else wants its block")
			}
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q where an else block opens (else if continues the chain)", p.cur().Text))
		}
		be := &ast.BlockExpr{Block: p.parseBlock(nil), Line: p.cur().Line, Col: p.cur().Col}
		// `else { if … }` is the else-if shape with the block spelled out.
		if len(be.Block.Items) == 1 {
			if es, ok := be.Block.Items[0].(*ast.ExprStmt); ok {
				if inner, ok := es.Expr.(*ast.If); ok {
					node.Else = inner
					return node
				}
			}
		}
		node.Else = be
	}
	if ctx == valueCtx && node.Else == nil {
		p.failTok(t, "E0202",
			fmt.Sprintf("valueless form in value position — an if without else produces no value and cannot stand in %s; valueless forms are statements", p.valueSite()))
	}
	return node
}

// parseMatch parses `match scrutinee { arms }` (chapter 4): at least one
// arm, arms separated by line breaks with no separator token.
func (p *parser) parseMatch() *ast.Match {
	t := p.cur()
	p.next()
	scrut := p.headExpr()
	if p.cur().Kind != "{" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a match opens its arms with {")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where a match's arm group opens", p.cur().Text))
	}
	open := p.cur()
	p.next()
	if p.cur().Kind == "}" {
		p.failTok(open, "E0301",
			"match requires at least one arm — the brace group holds no arms; with zero arms no arm can be taken and the match could never produce a value")
	}
	m := &ast.Match{Scrutinee: scrut, Line: t.Line, Col: t.Col}
	for {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a match closes with }")
		}
		m.Arms = append(m.Arms, p.parseMatchArm())
		if p.cur().Kind == "}" {
			p.next()
			return m
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a match closes with }")
		}
		if !p.brokeLine() {
			p.failTok(p.cur(), "E0105",
				fmt.Sprintf("unexpected token — %q fits no production: match arms are separated by newlines and carry no separator token", p.cur().Text))
		}
	}
}

// parseMatchArm parses `pattern [if cond] => body` — the body is one
// expression (a block body arrives as a block expression).
func (p *parser) parseMatchArm() ast.MatchArm {
	pat := p.parsePattern()
	ln, col := patPos(pat)
	arm := ast.MatchArm{Pat: pat, Line: ln, Col: col}
	if isKw(p.cur(), "if") {
		p.next()
		arm.Guard = p.headExpr()
	}
	if p.cur().Kind != "=>" {
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a match arm is pattern [if cond] => body")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q fits no production: an arm is pattern [if cond] => body", p.cur().Text))
	}
	p.next() // =>
	arm.Body = p.parseExpr(valueCtx)
	return arm
}

// patPos reports a pattern's first-token position.
func patPos(p ast.Pattern) (int, int) {
	switch x := p.(type) {
	case *ast.PatLiteral:
		return x.Line, x.Col
	case *ast.PatWildcard:
		return x.Line, x.Col
	case *ast.PatBinding:
		return x.Line, x.Col
	case *ast.PatOr:
		return x.Line, x.Col
	case *ast.PatTuple:
		return x.Line, x.Col
	case *ast.PatVariant:
		return x.Line, x.Col
	}
	return 0, 0
}

// --- the pattern grammar (chapter 4) -------------------------------------------

// parsePattern parses one full pattern: an atom, or a flat or-chain of
// atoms. The name-set checks are syntactic (chapter 4): one pattern binds a
// name at most once (E0404, per branch), and or-branches bind one name set
// (E0302).
func (p *parser) parsePattern() ast.Pattern {
	first := p.patternAtom()
	if p.cur().Kind != "|" {
		p.checkDupBinds(first)
		return first
	}
	or := &ast.PatOr{Branches: []ast.Pattern{first}, Line: p.cur().Line, Col: p.cur().Col}
	for p.cur().Kind == "|" {
		p.next()
		or.Branches = append(or.Branches, p.patternAtom())
	}
	for _, br := range or.Branches {
		p.checkDupBinds(br)
	}
	p.checkOrNames(or)
	return or
}

// patternAtom parses one non-or pattern: a literal, the wildcard, a binding
// name, a (possibly qualified) variant with optional payload sub-patterns,
// or a tuple.
func (p *parser) patternAtom() ast.Pattern {
	t := p.cur()
	switch {
	case t.Kind == lex.KindInt || t.Kind == lex.KindFloat ||
		t.Kind == lex.KindString || t.Kind == lex.KindRune:
		p.next()
		return &ast.PatLiteral{Kind: t.Kind, Text: t.Text, Line: t.Line, Col: t.Col}
	case isKw(t, "true") || isKw(t, "false"):
		p.next()
		return &ast.PatLiteral{Kind: "bool", Text: t.Text, Line: t.Line, Col: t.Col}
	case t.Kind == "_":
		p.next()
		return &ast.PatWildcard{Line: t.Line, Col: t.Col}
	case t.Kind == lex.KindIdent && !isPascal(t.Text):
		// a lowercase identifier followed by `.` qualifies a variant
		if p.peek().Kind == "." {
			qual := t.Text
			p.next() // module
			p.next() // .
			nt := p.cur()
			if nt.Kind != lex.KindIdent || !isPascal(nt.Text) {
				p.failTok(nt, "E0105",
					fmt.Sprintf("unexpected token — %q where the variant name goes: a qualified pattern is module.Variant or module.Variant(sub-patterns)", nt.Text))
			}
			return p.patVariant(nt, true, qual)
		}
		p.checkCamel(t)
		p.next()
		return &ast.PatBinding{Name: t.Text, Line: t.Line, Col: t.Col}
	case t.Kind == lex.KindIdent && isPascal(t.Text):
		return p.patVariant(t, false, "")
	case t.Kind == "(":
		return p.patTuple(t)
	case t.Kind == "-":
		p.failTok(t, "E0105",
			`unexpected token — "-" fits no pattern production: patterns take chapter-1 literals, which carry no sign`)
	}
	if t.Kind == lex.KindEOF {
		p.failTok(t, "E0105", "unexpected end of file — a match arm is pattern [if cond] => body")
	}
	p.failTok(t, "E0105",
		fmt.Sprintf("unexpected token — %q fits no pattern production: patterns are chapter-1 literals, _, binding names, variants, and tuples of patterns", t.Text))
	panic("unreachable")
}

// patVariant parses a variant pattern from its name token: bare (a unit
// variant), or with a parenthesized payload sub-pattern list. The
// qualified form carries its module qualifier's name.
func (p *parser) patVariant(t lex.Token, qualified bool, qual string) ast.Pattern {
	v := &ast.PatVariant{Name: t.Text, Qualified: qualified, Qual: qual, Line: t.Line, Col: t.Col}
	p.next()
	if p.cur().Kind != "(" {
		return v
	}
	p.next()
	for {
		if p.cur().Kind == ")" {
			p.next()
			return v
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a payload sub-pattern list closes with )")
		}
		v.Args = append(v.Args, p.parsePattern())
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == ")" {
			p.next()
			return v
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a payload sub-pattern list: sub-patterns are comma-separated and the list closes with )", p.cur().Text))
	}
}

// patTuple parses a tuple pattern of two or more sub-patterns: `()` is no
// pattern (the set has no unit pattern) and a one-element group is no
// pattern either.
func (p *parser) patTuple(open lex.Token) ast.Pattern {
	p.next()
	if p.cur().Kind == ")" {
		p.failTok(open, "E0105",
			`unexpected token — "()" fits no pattern production: the pattern set has no unit pattern; use the wildcard _ for unit values`)
	}
	tt := &ast.PatTuple{Line: open.Line, Col: open.Col}
	for {
		tt.Elems = append(tt.Elems, p.parsePattern())
		if p.cur().Kind == "," {
			p.next()
			if p.cur().Kind == ")" {
				p.next()
				return tt
			}
			continue
		}
		if p.cur().Kind == ")" {
			if len(tt.Elems) < 2 {
				p.failTok(open, "E0105",
					`unexpected token — "(" fits no pattern production: a tuple pattern holds two or more sub-patterns`)
			}
			p.next()
			return tt
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a tuple pattern closes with )")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a tuple pattern: sub-patterns are comma-separated and the pattern closes with )", p.cur().Text))
	}
}

// checkOrNames enforces E0302: every branch of an or-pattern binds the same
// name set, whichever branch matches, the arm body sees one binding set.
func (p *parser) checkOrNames(or *ast.PatOr) {
	base := patNames(or.Branches[0], nil)
	for _, br := range or.Branches[1:] {
		names := patNames(br, nil)
		if sameNameSet(base, names) {
			continue
		}
		a, b := firstOnly(base, names), firstOnly(names, base)
		p.fail(or.Line, or.Col, "E0302",
			fmt.Sprintf("or-pattern branches must bind the same names — the branches bind different names (%q and %q); whichever branch matches, the arm body must see one binding set", a, b))
	}
}

// patNames collects a pattern's binding names in source order.
func patNames(p ast.Pattern, into []string) []string {
	switch x := p.(type) {
	case *ast.PatLiteral, *ast.PatWildcard:
	case *ast.PatBinding:
		into = append(into, x.Name)
	case *ast.PatOr:
		for _, br := range x.Branches {
			into = patNames(br, into)
		}
	case *ast.PatTuple:
		for _, e := range x.Elems {
			into = patNames(e, into)
		}
	case *ast.PatVariant:
		for _, a := range x.Args {
			into = patNames(a, into)
		}
	}
	return into
}

func sameNameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if !seen[s] {
			return false
		}
	}
	return true
}

// firstOnly names the first element of a absent from b — E0302's report
// names one differing name per side.
func firstOnly(a, b []string) string {
	seen := map[string]bool{}
	for _, s := range b {
		seen[s] = true
	}
	for _, s := range a {
		if !seen[s] {
			return s
		}
	}
	return ""
}

// checkDupBinds enforces E0404 within one pattern (per or-branch): a name
// bound twice in one pattern is rejected at its second binding.
func (p *parser) checkDupBinds(pat ast.Pattern) {
	seen := map[string]bool{}
	var walk func(x ast.Pattern)
	walk = func(x ast.Pattern) {
		switch v := x.(type) {
		case *ast.PatBinding:
			if seen[v.Name] {
				p.fail(v.Line, v.Col, "E0404",
					fmt.Sprintf("duplicate name in one module — one pattern binds %q twice; a pattern binds each name at most once", v.Name))
			}
			seen[v.Name] = true
		case *ast.PatOr:
			return // branch sets are E0302's, not duplicates
		case *ast.PatTuple:
			for _, e := range v.Elems {
				walk(e)
			}
		case *ast.PatVariant:
			for _, a := range v.Args {
				walk(a)
			}
		}
	}
	walk(pat)
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
				p.assignAfterExpr()
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
// one ratified field form self.field is parsed by parseStmt's probe, so any
// member target reaching here is a plain non-name target — E0105.
func (p *parser) assignAfterExpr() {
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
			node = &ast.Member{Recv: node, Name: nt.Text, Line: t.Line, Col: t.Col, NameLine: nt.Line, NameCol: nt.Col}
			p.next()
		case "(":
			node = &ast.Call{Fn: node, Args: p.parseCallArgs(), Line: t.Line, Col: t.Col}
		case "?":
			// `?` is the propagation suffix (chapter 14): level-1 postfix,
			// chained with call and member access, anchored at the token.
			node = &ast.Prop{X: node, Line: t.Line, Col: t.Col}
			p.next()
		case "<":
			// Explicit type arguments (chapter 10) reach a bare-name head
			// only — obj.m<T>() keeps the comparison reading (the method
			// rule: no explicit form). The attempt is speculative: a shape
			// that does not commit restores the stream, and the comparison
			// levels take the tokens (`if a < b {` stays a comparison).
			id, bare := node.(*ast.Ident)
			if !bare {
				return node
			}
			if next := p.tryTypeArgs(id); next != nil {
				node = next
			} else {
				return node
			}
		case "[":
			p.failTok(t, "E0105",
				`unexpected token — "[" fits no postfix production: postfix is .name or (args); indexing is by named methods (the collections chapter)`)
		case "{":
			// A control-flow head ends at its body brace — never a
			// construction (headExpr suspends this case at depth zero).
			if p.noBrace > 0 && p.depth == 0 {
				return node
			}
			// Construction (chapter 8): a bare name/member chain whose
			// final name is PascalCase — the head is a type reference.
			// Anything else cannot be a construction head.
			path, bare := chainPath(node)
			if !bare || !isPascal(path[len(path)-1]) {
				p.failTok(t, "E0105",
					`unexpected token — "{" after a complete expression: construction heads are PascalCase type references (chapter 8)`)
			}
			node = p.parseConstruct(t, path, nil, p.last.Line, p.last.Col)
		default:
			return node
		}
	}
}

// tryTypeArgs speculatively parses an explicit generic clause on a
// bare-name postfix head (chapter 10): `name<T…>(args)` — a call — or
// `Name<T…> { … }` — a construction. The attempt commits only when the
// clause's closer is followed by ( or { under the postfix continuation
// rule (a construction brace never inside a control-flow head — the
// noBrace suspension), and an empty clause is no explicit form at all.
// Anything else — a non-committed shape, or a diagnostic raised inside
// the attempt — restores the stream and reports nil, leaving the tokens
// to the comparison levels: `if a < b {` stays a comparison, and a
// a < b > c shape keeps its non-associative diagnostic. Diagnostics a
// committed shape raises are real and stand.
func (p *parser) tryTypeArgs(id *ast.Ident) (node ast.Expr) {
	savedPos, savedLast := p.pos, p.last
	committed := false
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(stop); ok && !committed {
				p.pos, p.last = savedPos, savedLast
				node = nil
				return
			}
			panic(r)
		}
	}()
	args := p.genericArgs()
	if len(args) == 0 {
		p.pos, p.last = savedPos, savedLast
		return nil
	}
	// The postfix continuation rule holds after the clause too: at depth
	// zero a token on a new line ends the expression (the caller's loop
	// head enforced it for the <; this enforces it for what follows).
	if p.depth == 0 && p.brokeLine() {
		p.pos, p.last = savedPos, savedLast
		return nil
	}
	cl := p.last // the clause's closing `>` — the explicit application's anchor
	switch p.cur().Kind {
	case "(":
		committed = true
		op := p.cur()
		return &ast.Call{Fn: id, Args: p.parseCallArgs(), TypeArgs: args,
			Line: op.Line, Col: op.Col, ArgLine: cl.Line, ArgCol: cl.Col}
	case "{":
		if p.noBrace > 0 && p.depth == 0 {
			p.pos, p.last = savedPos, savedLast
			return nil
		}
		committed = true
		if !isPascal(id.Name) {
			p.failTok(p.cur(), "E0105",
				`unexpected token — "{" after a complete expression: construction heads are PascalCase type references (chapter 8)`)
		}
		node := p.parseConstruct(p.cur(), []string{id.Name}, args, id.Line, id.Col).(*ast.Construct)
		node.ArgLine, node.ArgCol = cl.Line, cl.Col
		return node
	}
	p.pos, p.last = savedPos, savedLast
	return nil
}

// chainPath reports whether e is a bare identifier or member chain (no
// calls or other postfixes) and its dotted path — a construction head is
// one of these with a PascalCase final name.
func chainPath(e ast.Expr) ([]string, bool) {
	switch x := e.(type) {
	case *ast.Ident:
		return []string{x.Name}, true
	case *ast.Member:
		if path, ok := chainPath(x.Recv); ok {
			return append(path, x.Name), true
		}
	}
	return nil, false
}

// parseConstruct parses chapter 8's construction and update braces: `Path {
// field: value, … [,] [with & base] }`, or with an explicit generic clause
// `Path<T…> { … }` (chapter 10's postfix angle-bracket lookahead hands the
// shape over). open is the `{`; path is the dotted head with the type name
// last; typeArgs is the explicit clause (nil without one); hl/hc anchor the
// node at the head's own name token — the last consumed token on the plain
// path, the head identifier on the lookahead path (the closer `>` sits in
// p.last there). Construction braces are brackets — line
// breaks inside fold — and the field list is comma- or newline-separated
// with a uniform trailing comma. The update base is one postfix expression
// after `with &`.
func (p *parser) parseConstruct(open lex.Token, path []string, typeArgs []ast.TypeRef, hl, hc int) ast.Expr {
	node := &ast.Construct{Name: path[len(path)-1], TypeArgs: typeArgs, Line: hl, Col: hc}
	if len(path) > 1 {
		node.Qual = joinDot(path[:len(path)-1])
	}
	p.next() // {
	p.depth++
	defer func() { p.depth-- }()
	for {
		if p.cur().Kind == "}" {
			p.next()
			return node
		}
		if isKw(p.cur(), "with") {
			p.next()
			if p.cur().Kind != "&" {
				if p.atEnd() {
					p.failTok(p.cur(), "E0105", "unexpected end of file — an update base is with & followed by one postfix expression")
				}
				p.failTok(p.cur(), "E0105",
					fmt.Sprintf("unexpected token — %q fits no production here: an update base is with & followed by one postfix expression", p.cur().Text))
			}
			p.next() // &
			node.Base = p.parsePostfix(valueCtx)
			if p.cur().Kind != "}" {
				if p.atEnd() {
					p.failTok(p.cur(), "E0105", "unexpected end of file — a construction closes with }")
				}
				p.failTok(p.cur(), "E0105",
					fmt.Sprintf("unexpected token — %q after the update base: a construction closes with }", p.cur().Text))
			}
			p.next()
			return node
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a construction closes with }")
		}
		ft := p.cur()
		if ft.Kind != lex.KindIdent {
			p.failTok(ft, "E0105",
				fmt.Sprintf("unexpected token — %q where a field name goes: field initializers are name: value pairs", ft.Text))
		}
		p.next()
		if p.cur().Kind != ":" {
			p.failTok(ft, "E0105",
				fmt.Sprintf("unexpected token — field %q wants its value: field initializers are name: value pairs", ft.Text))
		}
		p.next() // :
		node.Fields = append(node.Fields, ast.FieldInit{Name: ft.Text, Value: p.parseExpr(valueCtx), Line: ft.Line, Col: ft.Col})
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == "}" || isKw(p.cur(), "with") {
			continue
		}
		// a newline before the next field name continues the list
		if p.cur().Kind == lex.KindIdent && p.brokeLine() {
			continue
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a construction closes with }")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a field initializer list: fields are name: value pairs separated by commas or line breaks", p.cur().Text))
	}
}

// --- chapter 17: list literals -------------------------------------------------

// parseListLit parses `[e1, …, en]` (chapter 17), n zero or more, with the
// uniform trailing comma. The brackets are a paren region — line breaks
// inside fold. The caller passes the opening `[`; this consumes it.
func (p *parser) parseListLit(open lex.Token) ast.Expr {
	l := &ast.ListLit{Line: open.Line, Col: open.Col}
	p.next() // [
	p.depth++
	defer func() { p.depth-- }()
	for {
		if p.cur().Kind == "]" {
			p.next()
			return l
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a list literal closes with ]")
		}
		l.Elems = append(l.Elems, p.parseExpr(valueCtx))
		if p.cur().Kind == "," {
			p.next()
			continue
		}
		if p.cur().Kind == "]" {
			p.next()
			return l
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a list literal closes with ]")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q in a list literal: elements are comma-separated expressions and the literal closes with ]", p.cur().Text))
	}
}

// tupleTail parses a tuple expression's remainder after the first comma and
// closes the paren group (the caller opened the depth; this closes it).
// open is the group's `(` — the tuple node and the E0602 arity report both
// anchor there.
func (p *parser) tupleTail(open lex.Token, first ast.Expr) ast.Expr {
	tu := &ast.Tuple{Elems: []ast.Expr{first}, Line: open.Line, Col: open.Col}
	for {
		p.next() // the comma (or ) after a trailing comma)
		if p.cur().Kind == ")" {
			p.next()
			p.depth--
			p.tupleArity(open, len(tu.Elems), "expression")
			return tu
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a tuple expression closes with )")
		}
		tu.Elems = append(tu.Elems, p.parseExpr(valueCtx))
		if p.cur().Kind == "," {
			continue
		}
		if p.cur().Kind == ")" {
			p.next()
			p.depth--
			p.tupleArity(open, len(tu.Elems), "expression")
			return tu
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a tuple expression closes with )")
		}
		p.failTok(p.cur(), "E0105",
			fmt.Sprintf("unexpected token — %q where ) closes the tuple", p.cur().Text))
	}
}

// tupleArity enforces E0602's shared bound at a tuple's two sites: noun is
// "expression" or "type".
func (p *parser) tupleArity(open lex.Token, n int, noun string) {
	if n > 8 {
		p.failTok(open, "E0602", fmt.Sprintf(
			"tuple arity above eight — the tuple %s has %d elements; the arity bound forces naming: the longer the tuple, the more it wants to be a record",
			noun, n))
	}
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
	// fnBrace is one-shot: only this primary — the short closure body's own
	// leading brace — gets the function-block reading. Every later brace
	// (nested, in operands, in groups) is a plain block.
	fnBrace := p.fnBrace
	p.fnBrace = false
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
			return p.parseIf(ctx)
		case "match":
			return p.parseMatch()
		case "while", "loop", "break", "continue", "defer", "for", "scope":
			// ratified valueless statement forms — in any expression
			// position they produce no value (E0202). Inside a short
			// closure's maximal body break/continue hit the closure-body
			// loop reset first: E0201's position fact outranks E0202's.
			if (t.Text == "break" || t.Text == "continue") && p.shortBody > 0 {
				p.failLoopCtrl(t)
			}
			p.failTok(t, "E0202",
				fmt.Sprintf("valueless form in value position — %q produces no value and cannot stand in %s; valueless forms are statements", t.Text, p.valSite))
		case "fn":
			// an expression fn is a closure: the full form opens its
			// parameter list right after the keyword
			if p.peek().Kind == "(" {
				return p.parseFullClosure(t)
			}
			if p.peek().Kind == lex.KindEOF {
				p.failTok(p.peek(), "E0105", "unexpected end of file — an expression fn is a closure, fn(name: type, …) [-> type] { body }")
			}
			p.failTok(t, "E0105",
				fmt.Sprintf("unexpected token — %q where a closure's parameter list opens: an expression fn is a closure, fn(name: type, …) [-> type] { body }", p.peek().Text))
		case "task", "select":
			p.bnd(bndConc)
		}
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q cannot begin an expression: expressions are identifiers, literals, (e), { block }, and unary ! - ~", t.Text))
	case "(":
		p.next()
		p.depth++
		if p.cur().Kind == ")" {
			// the unit value: the one value of the unit type (chapter 8's
			// sliver this milestone implements)
			u := &ast.Unit{Line: t.Line, Col: t.Col}
			p.next()
			p.depth--
			return u
		}
		e := p.parseExpr(valueCtx)
		if p.cur().Kind == "," {
			return p.tupleTail(t, e) // consumes the ) and closes the depth
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
		if fnBrace {
			// a short closure's brace-led body is a function body block
			return &ast.BlockExpr{Block: p.parseFnBlock(&fnCtx{name: "(closure)"}), Line: t.Line, Col: t.Col}
		}
		return &ast.BlockExpr{Block: p.parseBlock(nil), Line: t.Line, Col: t.Col}
	case "[":
		return p.parseListLit(t)
	case "|":
		// a short closure in an operator's operand slot is unreachable —
		// the parenthesized spelling is the ratified one
		if operandSlot(p.last.Kind) {
			p.failTok(t, "E0105",
				`unexpected token — "|" fits no production here: a short closure in operand position must be parenthesized, as in 1 + (|x| x)`)
		}
		return p.parseShortClosure(t)
	case "||":
		// maximal munch leaves no zero-parameter short closure token
		p.failTok(t, "E0105",
			`unexpected token — "||" is the logical-or operator: no zero-parameter short closure exists; a zero-parameter closure is written in the full form fn() { ... }`)
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
			al, ac := p.anglePos()
			return &ast.NamedType{Name: t.Text, Line: t.Line, Col: t.Col, Args: p.genericArgs(), ArgLine: al, ArgCol: ac}
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
				al, ac := p.anglePos()
				return &ast.NamedType{Qual: joinDot(quals), Name: nt.Text, Line: t.Line, Col: t.Col, Args: p.genericArgs(), ArgLine: al, ArgCol: ac}
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
// parenthesized type fits no production — grouping is not a type form);
// E0602's arity bound holds here as at the expression site.
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
			break
		}
		tt.Elems = append(tt.Elems, p.parseTypeRef())
		if p.cur().Kind == "," {
			continue
		}
		if p.cur().Kind == ")" {
			break
		}
		if p.atEnd() {
			p.failTok(p.cur(), "E0105", "unexpected end of file — a tuple type closes with )")
		}
		p.failTok(p.cur(), "E0105", typeRefMsg(p.cur()))
	}
	p.next() // )
	p.tupleArity(open, len(tt.Elems), "type")
	return tt
}

// anglePos reports the position of a generic clause's `<` when one is
// next, else the zero position — the application's anchor.
func (p *parser) anglePos() (int, int) {
	if p.cur().Kind == "<" {
		return p.cur().Line, p.cur().Col
	}
	return 0, 0
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

// closeAngle consumes the closing `>` of a generic application and reports
// whether it did. Only the bare `>` closes: maximal munch makes `>>` and
// `>=` the shift and ge operators, so a two-byte `>`-led token at a closer
// position means the nested closers were not separated — chapter 10
// rejects that form under E0105 with the separation remediation.
func (p *parser) closeAngle() bool {
	switch t := p.cur(); t.Kind {
	case ">":
		p.next()
		return true
	case ">>", ">=":
		p.failTok(t, "E0105",
			fmt.Sprintf("unexpected token — %q merges the nested closers: nested closers are separated — write Name<Name<Int64> >; %q is the shift operator (chapter 1)", t.Text, t.Text))
	}
	return false
}

// parseFnType parses `fn(params) [tags] -> T`; the effect tags — bare or
// qualified — sit between the parameter list and the arrow, and the
// arrow itself is part of the form.
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
	if p.cur().Kind == lex.KindIdent {
		ft.TagLine, ft.TagCol = p.cur().Line, p.cur().Col
	}
	ft.EffectTags = p.effectTagList()
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
