// Package fmt is the line-structure-preserving token reformatter of M11
// design D1–D3: lex.Keep keeps comments as trivia, the parser classifies
// the two token-adjacency ambiguities no local rule can settle (the `|`
// delimiter vs or-pipe, and type-application angle brackets vs comparison
// operators), and the reformatter re-spaces tokens without ever moving one
// across a line — the one spec-named exception is the import block, which
// reorders as whole declarations. Never wraps, never aligns: a long line
// stays long.
package fmt

import (
	"sort"
	"strconv"
	"strings"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/lex"
	"github.com/ltlvtao/welang/internal/parser"
)

// Format reformats one We source file (chapter 21 R3's rule set). A lexical
// or parse failure is the only gate — chapter 2's newline-carries-meaning
// discipline makes the token-line structure semantic, so a file that does
// not parse is not reformatted: the source returns unchanged alongside the
// stage's diagnostic. A ratified-but-unimplemented form returns the source
// unchanged with no diagnostic — that class is the caller's own boundary
// gate (the cli parses once before formatting); the package has no
// boundary face of its own.
func Format(name string, src []byte) ([]byte, []diag.Diagnostic) {
	toks, d := lex.Keep(name, src)
	if d != nil {
		return src, []diag.Diagnostic{*d}
	}
	file, pd, ni := parser.Parse(name, src)
	if pd != nil {
		return src, []diag.Diagnostic{*pd}
	}
	if ni != nil {
		return src, nil
	}
	r := newReformer(toks, file)
	return []byte(r.reformat()), nil
}

// line is one token-bearing source line (comments included, eof excluded).
type line struct {
	no   int // 1-based source line number — the blank-run ledger
	toks []lex.Token
}

// tokinfo carries the per-token classifications that need more than the
// token pair at hand (design D1/D3).
type tokinfo struct {
	unary        bool // prefix !, ~, or a minus no value precedes
	closureOpen  bool // the `|` opening a short closure's parameter list
	closureClose bool // the `|` closing one
	typeAngle    bool // <, >, >> in type-application position (not a comparison)
}

type reformer struct {
	lines []line
	eff   []int // per line: the depth its indent follows (leading closers applied)
	// marks collected from the AST walk, keyed "line:col"
	closureOpen  map[string]bool
	closureClose map[string]bool // paired by pairClosurePipes
	binaryOps    map[string]bool // every Binary comparison/shift operator token
}

func newReformer(toks []lex.Token, file *ast.File) *reformer {
	r := &reformer{
		closureOpen:  map[string]bool{},
		closureClose: map[string]bool{},
		binaryOps:    map[string]bool{},
	}
	var cur *line
	for _, t := range toks {
		if t.Kind == lex.KindEOF {
			break
		}
		if cur == nil || t.Line != cur.no {
			r.lines = append(r.lines, line{no: t.Line})
			cur = &r.lines[len(r.lines)-1]
		}
		cur.toks = append(cur.toks, t)
	}
	r.computeDepths()
	r.walkFile(file)
	r.pairClosurePipes(toks)
	return r
}

// posKey is the mark key of a token position.
func posKey(line, col int) string {
	return strconv.Itoa(line) + ":" + strconv.Itoa(col)
}

// computeDepths walks the token stream once: the depth of a line's indent
// applies every leading closer first (design D3 — a closing line indents by
// the depth it closes to), then the whole line's openers and closers run
// into the next line's starting depth. Depth counts {, (, and [ alike;
// string and comment interiors never enter the account (the Keep face
// makes that natural).
func (r *reformer) computeDepths() {
	r.eff = make([]int, len(r.lines))
	d := 0
	for i := range r.lines {
		toks := r.lines[i].toks
		j := 0
		for ; j < len(toks) && isCloser(toks[j].Kind); j++ {
			d--
		}
		r.eff[i] = d
		for ; j < len(toks); j++ {
			switch {
			case isOpener(toks[j].Kind):
				d++
			case isCloser(toks[j].Kind):
				d--
			}
		}
	}
}

func isOpener(k string) bool { return k == "{" || k == "(" || k == "[" }
func isCloser(k string) bool { return k == "}" || k == ")" || k == "]" }

// --- the AST classification walk (design D1) --------------------------------
//
// Two ambiguities have no token-local answer. A `|` is a closure delimiter
// or the or-operator by syntactic position, and <, >, >> are comparison
// operators exactly when the parser made them Binary operators — every
// other angle token is a type-application bracket, closed by default. The
// walk collects the opening pipes of short closures and the positions of
// comparison/shift Binary operators; nothing else is re-derived here.

func (r *reformer) walkFile(f *ast.File) {
	for _, it := range f.Items {
		r.walkItem(it)
	}
}

func (r *reformer) walkItem(it ast.Item) {
	switch n := it.(type) {
	case *ast.Import:
	case *ast.FnDecl:
		for _, p := range n.Params {
			r.walkType(p.Type)
		}
		r.walkType(n.Ret)
		for _, w := range n.Where {
			r.walkWhere(w)
		}
		r.walkBlock(n.Body)
	case *ast.TopLet:
		r.walkBinding(n.Binding)
	case *ast.SumDecl:
		for _, v := range n.Variants {
			for _, t := range v.Payload {
				r.walkType(t)
			}
		}
	case *ast.RecordDecl:
		for _, f := range n.Fields {
			r.walkType(f.Typ)
		}
	case *ast.NewtypeDecl:
		r.walkType(n.Underlying)
	case *ast.InterfaceDecl:
		for _, m := range n.Methods {
			for _, p := range m.Params {
				r.walkType(p.Type)
			}
			r.walkType(m.Ret)
			if m.Body != nil {
				r.walkBlock(*m.Body)
			}
		}
	case *ast.ImplDecl:
		r.walkType(n.Iface)
		r.walkType(n.Head)
		for _, w := range n.Where {
			r.walkWhere(w)
		}
		for _, a := range n.Assocs {
			r.walkType(a.Type)
		}
		for _, m := range n.Methods {
			r.walkItem(m)
		}
	case *ast.EffectDecl:
	case *ast.TestDecl:
		r.walkBlock(n.Body)
	}
}

func (r *reformer) walkWhere(w *ast.WhereBound) {
	for _, i := range w.Ifaces {
		r.walkType(i)
	}
	for _, e := range w.Eq {
		r.walkType(e.RHS)
	}
}

func (r *reformer) walkStmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.Binding:
		r.walkBinding(*n)
	case *ast.MockDecl:
		for _, p := range n.Params {
			r.walkType(p.Type)
		}
		r.walkType(n.Ret)
		r.walkBlock(n.Body)
	case *ast.Assign:
		r.walkExpr(n.Value)
	case *ast.Return:
		r.walkExpr(n.Value)
	case *ast.ExprStmt:
		r.walkExpr(n.Expr)
	case *ast.While:
		r.walkExpr(n.Cond)
		r.walkBlock(n.Body)
	case *ast.Loop:
		r.walkBlock(n.Body)
	case *ast.Defer:
		r.walkBlock(n.Block)
	case *ast.ForStmt:
		r.walkPattern(n.Pat)
		r.walkExpr(n.Iter)
		r.walkBlock(n.Body)
	case *ast.ScopeRes:
		for _, b := range n.Binds {
			r.walkExpr(b.Val)
		}
		r.walkBlock(n.Body)
	}
}

func (r *reformer) walkBlock(b ast.Block) {
	for _, s := range b.Items {
		r.walkStmt(s)
	}
}

func (r *reformer) walkBinding(b ast.Binding) {
	r.walkPattern(b.Pat)
	r.walkType(b.Typ)
	r.walkExpr(b.Init)
}

func (r *reformer) walkExpr(e ast.Expr) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.Unary:
		r.walkExpr(n.X)
	case *ast.Binary:
		if isAngleOp(n.Op) {
			r.binaryOps[posKey(n.Line, n.Col)] = true
		}
		r.walkExpr(n.L)
		r.walkExpr(n.R)
	case *ast.Call:
		r.walkExpr(n.Fn)
		for _, a := range n.Args {
			r.walkExpr(a)
		}
		for _, t := range n.TypeArgs {
			r.walkType(t)
		}
	case *ast.Member:
		r.walkExpr(n.Recv)
	case *ast.Prop:
		r.walkExpr(n.X)
	case *ast.BlockExpr:
		r.walkBlock(n.Block)
	case *ast.If:
		r.walkExpr(n.Cond)
		r.walkBlock(n.Then)
		r.walkExpr(n.Else)
	case *ast.Match:
		r.walkExpr(n.Scrutinee)
		for _, a := range n.Arms {
			r.walkPattern(a.Pat)
			r.walkExpr(a.Guard)
			r.walkExpr(a.Body)
		}
	case *ast.TaskExpr:
		r.walkBlock(n.Body)
	case *ast.ScopeExpr:
		r.walkExpr(n.Timeout)
		r.walkBlock(n.Body)
	case *ast.SelectExpr:
		for _, c := range n.Cases {
			r.walkExpr(c.Source)
			r.walkExpr(c.Body)
		}
	case *ast.Construct:
		for _, f := range n.Fields {
			r.walkExpr(f.Value)
		}
		r.walkExpr(n.Base)
		for _, t := range n.TypeArgs {
			r.walkType(t)
		}
	case *ast.Tuple:
		for _, el := range n.Elems {
			r.walkExpr(el)
		}
	case *ast.Closure:
		if n.Short {
			r.closureOpen[posKey(n.Line, n.Col)] = true
		}
		for _, p := range n.Params {
			r.walkType(p.Type)
		}
		r.walkType(n.Ret)
		r.walkBlock(n.Body)
	case *ast.ListLit:
		for _, el := range n.Elems {
			r.walkExpr(el)
		}
	}
}

func (r *reformer) walkPattern(p ast.Pattern) {
	if p == nil {
		return
	}
	switch n := p.(type) {
	case *ast.PatOr:
		for _, b := range n.Branches {
			r.walkPattern(b)
		}
	case *ast.PatTuple:
		for _, e := range n.Elems {
			r.walkPattern(e)
		}
	case *ast.PatVariant:
		for _, a := range n.Args {
			r.walkPattern(a)
		}
	}
}

func (r *reformer) walkType(t ast.TypeRef) {
	if t == nil {
		return
	}
	switch n := t.(type) {
	case *ast.NamedType:
		for _, a := range n.Args {
			r.walkType(a)
		}
	case *ast.TupleType:
		for _, e := range n.Elems {
			r.walkType(e)
		}
	case *ast.FnType:
		for _, p := range n.Params {
			r.walkType(p)
		}
		r.walkType(n.Ret)
	}
}

// isAngleOp reports whether op is one of the operators that also spell a
// type-application bracket; only their Binary positions are comparisons.
func isAngleOp(op string) bool {
	switch op {
	case "<", ">", "<=", ">=", "<<", ">>":
		return true
	}
	return false
}

// pairClosurePipes finds each short closure's closing pipe: from the
// opening `|` token, the next `|` token in the stream is the closer — a
// parameter list holds no pipe of its own, and the body cannot start
// before the delimiter closes.
func (r *reformer) pairClosurePipes(toks []lex.Token) {
	for i, t := range toks {
		if t.Kind != "|" || !r.closureOpen[posKey(t.Line, t.Col)] {
			continue
		}
		for j := i + 1; j < len(toks); j++ {
			if toks[j].Kind == "|" {
				r.closureClose[posKey(toks[j].Line, toks[j].Col)] = true
				break
			}
		}
	}
}

// --- assembly (design D3: imports, blank-line policy, indentation) ----------

// importUnit is one import declaration lifted for the block reorder: its
// path segments, alias, and source lines — the attached comment run that
// sits directly above with no blank line between rides along (the one
// cross-line move the spec names; R3's alphabetical order).
type importUnit struct {
	std   bool
	segs  []string
	alias string
	lines []int // indices into r.lines: attached comments, then the import
}

// reformat assembles the whole file: the import block first (std group in
// path order, one blank, the rest in path order), then every remaining
// line in source order — one blank between top-level units, runs of blanks
// collapsed to one, no leading blank, exactly one trailing newline.
func (r *reformer) reformat() string {
	imports, kept := r.extractImports()
	var out []string
	groups := [2][]importUnit{} // 0: std.*, 1: the rest
	for _, u := range imports {
		if u.std {
			groups[0] = append(groups[0], u)
		} else {
			groups[1] = append(groups[1], u)
		}
	}
	for g, group := range groups {
		for _, u := range group {
			for _, idx := range u.lines {
				out = append(out, r.renderLine(idx))
			}
		}
		if g == 0 && len(groups[0]) > 0 && len(groups[1]) > 0 {
			out = append(out, "")
		}
	}
	if len(imports) > 0 && len(kept) > 0 {
		out = append(out, "")
	}
	unit := r.unitStarts(kept)
	for i, idx := range kept {
		if i > 0 {
			if r.lines[idx].no-r.lines[kept[i-1]].no-1 > 0 || unit[i] {
				out = append(out, "")
			}
		}
		out = append(out, r.renderLine(idx))
	}
	if len(out) == 0 {
		return "" // zero content stays zero bytes (design D3's edge)
	}
	return strings.Join(out, "\n") + "\n"
}

// extractImports lifts every top-level import declaration (with its
// attached comment run) out of the line sequence and sorts both groups by
// path segments, alias breaking ties. kept holds the remaining line
// indices in source order.
func (r *reformer) extractImports() (imports []importUnit, kept []int) {
	consumed := make([]bool, len(r.lines))
	for i, ln := range r.lines {
		if r.eff[i] != 0 || len(ln.toks) == 0 {
			continue
		}
		t0 := ln.toks[0]
		if t0.Kind != lex.KindKeyword || t0.Text != "import" {
			continue
		}
		u := importUnit{lines: []int{i}}
		for k := 1; k < len(ln.toks); k++ {
			t := ln.toks[k]
			if t.Kind == lex.KindComment {
				break
			}
			if t.Kind == lex.KindKeyword && t.Text == "as" {
				if k+1 < len(ln.toks) && ln.toks[k+1].Kind == lex.KindIdent {
					u.alias = ln.toks[k+1].Text
				}
				break
			}
			if t.Kind == lex.KindIdent {
				u.segs = append(u.segs, t.Text)
			}
		}
		u.std = len(u.segs) > 0 && u.segs[0] == "std"
		consumed[i] = true
		// The attached run: comment-only lines directly above, no blank
		// line between, not already claimed by an import above them.
		for j := i - 1; j >= 0; j-- {
			if consumed[j] || r.eff[j] != 0 || !r.isCommentOnly(j) || r.lines[j+1].no-r.lines[j].no-1 != 0 {
				break
			}
			u.lines = append([]int{j}, u.lines...)
			consumed[j] = true
		}
		imports = append(imports, u)
	}
	for i := range r.lines {
		if !consumed[i] {
			kept = append(kept, i)
		}
	}
	sort.SliceStable(imports, func(x, y int) bool {
		a, b := imports[x], imports[y]
		if a.std != b.std {
			return a.std
		}
		n := len(a.segs)
		if len(b.segs) < n {
			n = len(b.segs)
		}
		for k := 0; k < n; k++ {
			if a.segs[k] != b.segs[k] {
				return a.segs[k] < b.segs[k]
			}
		}
		if len(a.segs) != len(b.segs) {
			return len(a.segs) < len(b.segs)
		}
		return a.alias < b.alias
	})
	return imports, kept
}

// declStart is the keyword set that opens a top-level item (chapter 6's
// items; pub and the category keywords prefix the head keyword).
var declStart = map[string]bool{
	"pub": true, "fn": true, "let": true, "var": true, "type": true,
	"byval": true, "byres": true, "record": true, "newtype": true,
	"interface": true, "impl": true, "effect": true, "test": true,
	"foreign": true,
}

// unitStarts marks the lines that begin a blank-separated top-level unit.
// Declarations separate (fmt-pass pins the insert: effect→type, fn→fn);
// a run of let/var bindings clusters like the import block (the closure
// pipe case pins no insert between adjacent top-level lets). A comment
// run's boundary falls above the run — never between a comment and the
// item it documents.
func (r *reformer) unitStarts(kept []int) []bool {
	n := len(kept)
	u := make([]bool, n)
	prevKind := "" // the head kind of the last item seen ("let" or "decl")
	for i := 0; i < n; i++ {
		idx := kept[i]
		adjacentComment := i > 0 && r.eff[kept[i-1]] == 0 && r.isCommentOnly(kept[i-1]) &&
			r.lines[idx].no-r.lines[kept[i-1]].no-1 == 0
		switch {
		case r.eff[idx] == 0 && r.isItemStart(r.lines[idx]):
			kind := r.headKind(r.lines[idx])
			u[i] = (kind == "decl" || prevKind != "let") && !adjacentComment
			prevKind = kind
		case r.eff[idx] == 0 && r.isCommentOnly(idx):
			if adjacentComment {
				break // a run continuation: the run's top owns the decision
			}
			// Walk down the run to its anchor, the first non-comment line.
			j := i
			for j+1 < n && r.isCommentOnly(kept[j+1]) && r.lines[kept[j+1]].no-r.lines[kept[j]].no-1 == 0 {
				j++
			}
			anchor := -1
			if j+1 < n && r.lines[kept[j+1]].no-r.lines[kept[j]].no-1 == 0 {
				anchor = kept[j+1]
			}
			if anchor < 0 || r.eff[anchor] != 0 || !r.isItemStart(r.lines[anchor]) {
				u[i] = true // free-standing, or moored to a non-item: its own unit
				break
			}
			kind := r.headKind(r.lines[anchor])
			u[i] = kind == "decl" || prevKind != "let"
			// The anchor's own item-start line is an adjacent-comment case:
			// it marks no boundary, and updates prevKind when reached.
		}
	}
	return u
}

// headKind classifies an item-start line by its head keyword, skipping the
// prefix keywords: let/var bindings are "let", every other item "decl".
func (r *reformer) headKind(ln line) string {
	for _, t := range ln.toks {
		if t.Kind != lex.KindKeyword {
			return "decl"
		}
		switch t.Text {
		case "pub", "byval", "byres":
			continue
		case "let", "var":
			return "let"
		}
		return "decl"
	}
	return "decl"
}

func (r *reformer) isItemStart(ln line) bool {
	if len(ln.toks) == 0 {
		return false
	}
	t0 := ln.toks[0]
	return t0.Kind == lex.KindKeyword && declStart[t0.Text]
}

func (r *reformer) isCommentOnly(idx int) bool {
	for _, t := range r.lines[idx].toks {
		if t.Kind != lex.KindComment {
			return false
		}
	}
	return len(r.lines[idx].toks) > 0
}

// renderLine writes one line: depth indentation, the tokens joined by the
// adjacency table, trailing whitespace stripped (a comment's text is
// verbatim inside, so only the line's end is trimmed).
func (r *reformer) renderLine(idx int) string {
	ln := r.lines[idx]
	infos := r.classify(ln.toks)
	depth := r.eff[idx]
	if depth < 0 {
		depth = 0
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", 2*depth))
	for i, t := range ln.toks {
		if i > 0 && r.gap(ln.toks[i-1], infos[i-1], t, infos[i]) == 1 {
			b.WriteByte(' ')
		}
		b.WriteString(t.Text)
	}
	return strings.TrimRight(b.String(), " \t")
}

// --- the adjacency table (design D3) ----------------------------------------
//
// gap decides the spaces between two adjacent tokens of one line. Priority
// as the design fixes it: parenthesis/bracket zero > pipe one > the rest.

func (r *reformer) classify(toks []lex.Token) []tokinfo {
	infos := make([]tokinfo, len(toks))
	for i, t := range toks {
		switch t.Kind {
		case "|":
			k := posKey(t.Line, t.Col)
			infos[i].closureOpen = r.closureOpen[k]
			infos[i].closureClose = r.closureClose[k]
		case "<", ">", ">>":
			infos[i].typeAngle = !r.binaryOps[posKey(t.Line, t.Col)]
		case "!", "~":
			infos[i].unary = true
		case "-":
			if i == 0 {
				infos[i].unary = true
				break
			}
			p := toks[i-1]
			infos[i].unary = !(isWordish(p) || p.Kind == ")" || p.Kind == "]")
		}
	}
	return infos
}

func (r *reformer) gap(a lex.Token, ia tokinfo, b lex.Token, ib tokinfo) int {
	// A comment runs to end of line; one space before it, verbatim within.
	if b.Kind == lex.KindComment {
		return 1
	}
	// Parenthesis and bracket adjacency is tight (highest priority).
	if a.Kind == "(" || a.Kind == "[" {
		return 0
	}
	if b.Kind == ")" || b.Kind == "]" {
		return 0
	}
	// An empty block is glued.
	if a.Kind == "{" && b.Kind == "}" {
		return 0
	}
	// Braces take one space beside any neighbor sharing their line.
	if b.Kind == "{" || a.Kind == "{" || b.Kind == "}" || a.Kind == "}" {
		return 1
	}
	// Type-application brackets cluster tight; comparisons (the AST's
	// Binary positions) fell through to the binary row below.
	if ia.typeAngle || ib.typeAngle {
		return 0
	}
	// An opening parenthesis or bracket adds nothing before itself: the
	// call form after any word-like token or closer is tight; separators
	// and operators keep their own trailing space.
	if b.Kind == "(" || b.Kind == "[" {
		if isWordish(a) || a.Kind == ")" || a.Kind == "]" || a.Kind == "." || a.Kind == "?" || ia.unary {
			return 0
		}
		return 1
	}
	// Closure delimiter pipes: zero on the parameter side, one on the body
	// side (the paren row above already won at call openings).
	if a.Kind == "|" {
		if ia.closureOpen {
			return 0
		}
		return 1
	}
	if b.Kind == "|" {
		if ib.closureClose {
			return 0
		}
		return 1
	}
	// Prefix operators: one before, none after — ahead of the binary row,
	// because a minus the classification placed in prefix position is by
	// definition not the binary operator this round (the ordering was the
	// M12-found defect: every `-x` spaced to `- x`, against the grammar
	// chapter's own examples).
	if ib.unary {
		return 1
	}
	if ia.unary {
		return 0
	}
	// Binary operators, one on each side.
	if isBinaryKind(a.Kind) || isBinaryKind(b.Kind) {
		return 1
	}
	// Comma, semicolon, colon: none before, one after.
	if a.Kind == "," || a.Kind == ";" || a.Kind == ":" {
		return 1
	}
	if b.Kind == "," || b.Kind == ";" || b.Kind == ":" {
		return 0
	}
	// Member access and ? propagation chain tight.
	if a.Kind == "." || b.Kind == "." || a.Kind == "?" || b.Kind == "?" {
		return 0
	}
	// Words separate, and so does everything unlisted.
	return 1
}

// isBinaryKind is the closed operator set that takes one space on each
// side: arithmetic, comparison, logic, shifts, bitwise, assignment, the
// range, and the two arrows. The angle members reach this row only when
// the AST proved them comparisons.
func isBinaryKind(kind string) bool {
	switch kind {
	case "+", "-", "*", "/", "%",
		"<", ">", "<=", ">=", "==", "!=",
		"&&", "||", "<<", ">>", "&", "|", "^",
		"=", "..", "->", "=>":
		return true
	}
	return false
}

// isWordish reports whether the token is word-like: an identifier,
// keyword, literal, attribute unit, or the underscore. Any of these in
// call position keeps the parenthesis tight.
func isWordish(t lex.Token) bool {
	switch t.Kind {
	case lex.KindIdent, lex.KindKeyword, lex.KindInt, lex.KindFloat,
		lex.KindString, lex.KindRune, lex.KindAttr, "_":
		return true
	}
	return false
}
