// The we doc subcommand (chapter 21's documentation face, M11 design D6):
// the check pipeline first — doc is a pipeline command (Q4), so the
// advisory segment runs under the manifest's postures and an E or a
// promoted finding stops with no page — then one Markdown page per module
// of the document set: the root graph filtered as the build face filters
// it (std.* out) on the project face, the named file alone on the
// single-file face. Each page carries the pub declarations in source
// order — a heading, the canonical signature in a fence, the /// unit's
// lines as prose outside the fence. --check reports the undocumented
// ones on stderr as plain tool lines — outside the diagnostic registry
// and unaffected by --json — and writes nothing; a gap set is exit 1.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// runDoc dispatches like check: a directory is the project face, a .we
// file the single-file face (no manifest — every posture is the warning
// default), anything else the usage error.
func (e *env) runDoc(path string, info os.FileInfo) int {
	if info.IsDir() {
		return e.runDocProject(path)
	}
	if !strings.HasSuffix(path, ".we") {
		return e.usageErr("doc wants a .we file, got %q", path)
	}
	return e.runDocSingle(path)
}

// docModule is one module of the document set: its page name (the module
// key), its path in the pages' own reference form (slash-separated,
// project-relative — the --check gap lines print it), and its parsed
// file.
type docModule struct {
	Key  string
	Path string
	File *ast.File
}

// runDocProject documents one project directory through the shared
// loader and the check pipeline, then renders the document set: the root
// module plus the root graph's non-std modules (the build face's own
// filter — std.* documents nothing of the project).
func (e *env) runDocProject(dir string) int {
	manifest, file, _, mods, code := e.loadProject(dir, false)
	if code != exitOK {
		return code
	}
	rootPath := filepath.Join(dir, "src", "main.we")
	found, code := e.projectAdvisories(dir, rootPath, file, mods)
	if code != exitOK {
		return code
	}
	if promoted, _ := e.advise(manifest, found); promoted {
		return exitDiagnostic
	}
	set := []docModule{{Key: "main", Path: docRelPath(dir, rootPath), File: file}}
	for _, m := range mods {
		if m.Key == "std" || strings.HasPrefix(m.Key, "std.") {
			continue
		}
		set = append(set, docModule{Key: m.Key, Path: docRelPath(dir, m.Path), File: m.File})
	}
	return e.docSet(set, dir)
}

// runDocSingle documents one .we file as its own set: the single-file
// check pipeline with its advisory layer (no manifest, so no posture can
// promote), then the page under the file's directory's docs/ — the
// design's disclosed default root.
func (e *env) runDocSingle(path string) int {
	file, code := e.loadFile(path)
	if file == nil {
		return code
	}
	e.advise(nil, typecheck.Advisories(file, path, typecheck.SingleFile))
	key := strings.TrimSuffix(filepath.Base(path), ".we")
	return e.docSet([]docModule{{Key: key, Path: filepath.ToSlash(path), File: file}}, filepath.Dir(path))
}

// docSet renders or checks the document set against one default root.
// --check reports each undocumented pub declaration and writes nothing;
// the default face writes one page per module — docs/{key}.md, flat and
// dotted, mirroring the module key — silent on success, `we: wrote
// {path}` per page under --verbose.
func (e *env) docSet(set []docModule, dir string) int {
	if e.docCheck {
		gaps := false
		for _, m := range set {
			for _, it := range pubDocItems(m.File) {
				if it.doc != "" {
					continue
				}
				fmt.Fprintf(e.stderr, "we: undocumented pub declaration %s at %s:%d\n", it.name, m.Path, it.line)
				gaps = true
			}
		}
		if gaps {
			return exitDiagnostic
		}
		return exitOK
	}
	root := e.docOutput
	if root == "" {
		root = filepath.Join(dir, "docs")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return e.fsError(err)
	}
	for _, m := range set {
		p := filepath.Join(root, m.Key+".md")
		if err := os.WriteFile(p, []byte(renderDocPage(m.Key, m.File)), 0o644); err != nil {
			return e.fsError(err)
		}
		if e.verbose {
			fmt.Fprintf(e.stdout, "we: wrote %s\n", filepath.ToSlash(p))
		}
	}
	return exitOK
}

// docRelPath renders one module's path as the pages' own reference form:
// slash-separated, relative to the project root.
func docRelPath(dir, path string) string {
	if rel, err := filepath.Rel(dir, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

// docItem is one pub declaration of a doc page: the heading name, the
// declaration line (--check's anchor), the canonical signature for the
// fence, and the /// unit's lines as prose (empty when undocumented).
type docItem struct {
	name string
	line int
	sig  string
	doc  string
}

// pubDocItems walks one module's top-level items in source order and
// keeps the pub declarations of the six documentable kinds (design D6):
// fn, let, record, newtype, sum type, and interface. A pub effect
// declaration names a tag, not an API surface — the design's list keeps
// it out; impl blocks likewise (their methods document through the
// interface page).
func pubDocItems(f *ast.File) []docItem {
	docs := map[int]string{}
	for _, d := range f.Docs {
		docs[d.Item] = strings.Join(d.Lines, "\n")
	}
	var items []docItem
	for i, it := range f.Items {
		var d docItem
		switch n := it.(type) {
		case *ast.FnDecl:
			if !n.Pub {
				continue
			}
			d = docItem{n.Name, n.Line, "pub " + renderFnSig(n.Name, n.TypeParams, n.Params, n.EffectTags, n.Ret, n.Where), docs[i]}
		case *ast.TopLet:
			if !n.Pub {
				continue
			}
			d = docItem{n.Binding.Name, n.Line, renderTopLet(n), docs[i]}
		case *ast.RecordDecl:
			if !n.Pub {
				continue
			}
			d = docItem{n.Name, n.Line, renderRecordDecl(n), docs[i]}
		case *ast.NewtypeDecl:
			if !n.Pub {
				continue
			}
			d = docItem{n.Name, n.Line, renderNewtypeDecl(n), docs[i]}
		case *ast.SumDecl:
			if !n.Pub {
				continue
			}
			d = docItem{n.Name, n.Line, renderSumDecl(n), docs[i]}
		case *ast.InterfaceDecl:
			if !n.Pub {
				continue
			}
			d = docItem{n.Name, n.Line, renderInterfaceDecl(n), docs[i]}
		default:
			continue
		}
		items = append(items, d)
	}
	return items
}

// renderDocPage renders one module's page: the module heading, then each
// pub declaration as a heading, a fenced canonical signature, and the
// /// unit's lines as prose after the fence.
func renderDocPage(key string, f *ast.File) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Module %s\n", key)
	for _, it := range pubDocItems(f) {
		b.WriteString("\n### " + it.name + "\n\n```\n" + it.sig + "\n```\n")
		if it.doc != "" {
			b.WriteString("\n" + it.doc + "\n")
		}
	}
	return b.String()
}

// --- canonical signatures ----------------------------------------------------

// renderFnSig renders a fn declaration's canonical signature — chapter
// 6's order, the effect segment before the return type, no body.
func renderFnSig(name string, tp []*ast.TypeParam, params []ast.Param, tags []string, ret ast.TypeRef, where []*ast.WhereBound) string {
	var b strings.Builder
	b.WriteString("fn " + name)
	writeTypeParams(&b, tp)
	b.WriteString("(" + strings.Join(paramStrings(params), ", ") + ")")
	writeEffect(&b, tags)
	if ret != nil {
		b.WriteString(" -> " + renderType(ret))
	}
	writeWhere(&b, where)
	return b.String()
}

// renderMethodSig renders one interface member's signature — the
// receiver first in the parameter list, a default body dropped as every
// body is (the page documents the contract, not the implementation).
func renderMethodSig(m ast.MethodSig) string {
	ps := paramStrings(m.Params)
	switch m.Recv {
	case ast.RecvSelf:
		ps = append([]string{"self"}, ps...)
	case ast.RecvMutSelf:
		ps = append([]string{"mut self"}, ps...)
	}
	var b strings.Builder
	b.WriteString("fn " + m.Name)
	writeTypeParams(&b, m.TypeParams)
	b.WriteString("(" + strings.Join(ps, ", ") + ")")
	writeEffect(&b, m.EffectTags)
	if m.HasRet && m.Ret != nil {
		b.WriteString(" -> " + renderType(m.Ret))
	}
	return b.String()
}

// paramStrings renders one parameter list — `name: type` pairs, or the
// bare name where the form carries no annotation slot (a short
// closure's parameters).
func paramStrings(params []ast.Param) []string {
	ps := make([]string, len(params))
	for i, p := range params {
		if p.Type == nil {
			ps[i] = p.Name
			continue
		}
		ps[i] = p.Name + ": " + renderType(p.Type)
	}
	return ps
}

// writeTypeParams renders a generic clause `<T1, …, Tk>`.
func writeTypeParams(b *strings.Builder, tp []*ast.TypeParam) {
	if len(tp) == 0 {
		return
	}
	names := make([]string, len(tp))
	for i, t := range tp {
		names[i] = t.Name
	}
	b.WriteString("<" + strings.Join(names, ", ") + ">")
}

// writeEffect renders a declaration's effect segment — the `effect`
// keyword and space-separated tags — through the display rule every tag
// rendering shares (typecheck's displayTag): a qualified key shows the
// effect's own name.
func writeEffect(b *strings.Builder, tags []string) {
	if len(tags) == 0 {
		return
	}
	ds := make([]string, len(tags))
	for i, t := range tags {
		if j := strings.LastIndex(t, "."); j >= 0 {
			t = t[j+1:]
		}
		ds[i] = t
	}
	b.WriteString(" effect " + strings.Join(ds, " "))
}

// writeWhere renders a where clause after the return type as in source:
// the bound form `Subject: Iface + Iface2`, the equality form
// `Subject.Assoc == TypeRef`, bounds comma-joined.
func writeWhere(b *strings.Builder, ws []*ast.WhereBound) {
	if len(ws) == 0 {
		return
	}
	parts := make([]string, len(ws))
	for i, w := range ws {
		switch {
		case len(w.Ifaces) > 0:
			ns := make([]string, len(w.Ifaces))
			for j, t := range w.Ifaces {
				ns[j] = renderType(t)
			}
			parts[i] = w.Subject + ": " + strings.Join(ns, " + ")
		default:
			eqs := make([]string, len(w.Eq))
			for j, eq := range w.Eq {
				eqs[j] = w.Subject + "." + eq.Assoc + " == " + renderType(eq.RHS)
			}
			parts[i] = strings.Join(eqs, ", ")
		}
	}
	b.WriteString(" where " + strings.Join(parts, ", "))
}

// renderRecordDecl renders the record declaration's canonical form —
// category keyword, fields comma-joined, derives tail; zero fields brace
// tight.
func renderRecordDecl(n *ast.RecordDecl) string {
	var b strings.Builder
	b.WriteString("pub ")
	switch n.Cat {
	case "value":
		b.WriteString("byval ")
	case "resource":
		b.WriteString("byres ")
	}
	b.WriteString("record " + n.Name)
	writeTypeParams(&b, n.TypeParams)
	if len(n.Fields) == 0 {
		b.WriteString(" {}")
	} else {
		fs := make([]string, len(n.Fields))
		for i, f := range n.Fields {
			fs[i] = f.Name + ": " + renderType(f.Typ)
		}
		b.WriteString(" { " + strings.Join(fs, ", ") + " }")
	}
	writeDerives(&b, n.Derives)
	return b.String()
}

// renderSumDecl renders the sum declaration's canonical form — variants
// pipe-joined, a payload in parentheses, derives tail.
func renderSumDecl(n *ast.SumDecl) string {
	var b strings.Builder
	b.WriteString("pub ")
	if n.Byval {
		b.WriteString("byval ")
	}
	b.WriteString("type " + n.Name)
	writeTypeParams(&b, n.TypeParams)
	vs := make([]string, len(n.Variants))
	for i, v := range n.Variants {
		vs[i] = v.Name
		if len(v.Payload) > 0 {
			ps := make([]string, len(v.Payload))
			for j, t := range v.Payload {
				ps[j] = renderType(t)
			}
			vs[i] += "(" + strings.Join(ps, ", ") + ")"
		}
	}
	b.WriteString(" = " + strings.Join(vs, " | "))
	writeDerives(&b, n.Derives)
	return b.String()
}

// renderNewtypeDecl renders the newtype declaration's canonical form —
// the underlying type in parentheses, derives tail.
func renderNewtypeDecl(n *ast.NewtypeDecl) string {
	var b strings.Builder
	b.WriteString("pub newtype " + n.Name)
	writeTypeParams(&b, n.TypeParams)
	b.WriteString("(" + renderType(n.Underlying) + ")")
	writeDerives(&b, n.Derives)
	return b.String()
}

// renderTopLet renders the top-level binding's canonical form with its
// initializer rendered back.
func renderTopLet(n *ast.TopLet) string {
	s := "pub " + n.Binding.Kw + " " + n.Binding.Name
	if n.Binding.Typ != nil {
		s += ": " + renderType(n.Binding.Typ)
	}
	return s + " = " + renderExpr(n.Binding.Init)
}

// renderInterfaceDecl renders the interface declaration with its members
// two-space indented, one per line — associated-type holes first, then
// methods (the parse keeps the two lists apart; source interleaving is
// not preserved, a disclosed face).
func renderInterfaceDecl(n *ast.InterfaceDecl) string {
	var b strings.Builder
	b.WriteString("pub interface " + n.Name)
	writeTypeParams(&b, n.TypeParams)
	b.WriteString(" {\n")
	for _, a := range n.Assocs {
		b.WriteString("  type " + a.Name + "\n")
	}
	for _, m := range n.Methods {
		b.WriteString("  " + renderMethodSig(m) + "\n")
	}
	b.WriteString("}")
	return b.String()
}

func writeDerives(b *strings.Builder, d *ast.DerivesClause) {
	if d == nil {
		return
	}
	b.WriteString(" derives " + strings.Join(d.Targets, ", "))
}

// --- type and expression renderings ------------------------------------------

// renderType renders a type reference's canonical spelling: qualified
// names dotted, generic applications bracketed, tuples and the unit and
// fn types in their source forms (the fn type's tags are bare — chapter
// 7's `fn(String) db -> ()`, no effect keyword).
func renderType(t ast.TypeRef) string {
	switch n := t.(type) {
	case *ast.NamedType:
		s := n.Name
		if n.Qual != "" {
			s = n.Qual + "." + s
		}
		if len(n.Args) > 0 {
			args := make([]string, len(n.Args))
			for i, a := range n.Args {
				args[i] = renderType(a)
			}
			s += "<" + strings.Join(args, ", ") + ">"
		}
		return s
	case *ast.TupleType:
		elems := make([]string, len(n.Elems))
		for i, el := range n.Elems {
			elems[i] = renderType(el)
		}
		return "(" + strings.Join(elems, ", ") + ")"
	case *ast.UnitType:
		return "()"
	case *ast.FnType:
		params := make([]string, len(n.Params))
		for i, p := range n.Params {
			params[i] = renderType(p)
		}
		s := "fn(" + strings.Join(params, ", ") + ")"
		if len(n.EffectTags) > 0 {
			ds := make([]string, len(n.EffectTags))
			for i, t := range n.EffectTags {
				if j := strings.LastIndex(t, "."); j >= 0 {
					t = t[j+1:]
				}
				ds[i] = t
			}
			s += " " + strings.Join(ds, " ")
		}
		return s + " -> " + renderType(n.Ret)
	}
	return ""
}

// renderExpr renders an expression back in its canonical single-line
// form. Only a pub let's initializer reaches a page, but the renderer is
// total over the expression grammar — the page face never meets a form
// it cannot spell. Multi-line bodies fold to `{ item; item }`.
func renderExpr(x ast.Expr) string {
	switch n := x.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.Literal:
		return n.Text
	case *ast.Unary:
		return n.Op + renderExpr(n.X)
	case *ast.Binary:
		return renderExpr(n.L) + " " + n.Op + " " + renderExpr(n.R)
	case *ast.Call:
		s := renderExpr(n.Fn)
		if len(n.TypeArgs) > 0 {
			args := make([]string, len(n.TypeArgs))
			for i, t := range n.TypeArgs {
				args[i] = renderType(t)
			}
			s += "<" + strings.Join(args, ", ") + ">"
		}
		return s + "(" + joinExprs(n.Args) + ")"
	case *ast.Member:
		return renderExpr(n.Recv) + "." + n.Name
	case *ast.Prop:
		return renderExpr(n.X) + "?"
	case *ast.Unit:
		return "()"
	case *ast.Tuple:
		return "(" + joinExprs(n.Elems) + ")"
	case *ast.ListLit:
		return "[" + joinExprs(n.Elems) + "]"
	case *ast.Construct:
		var b strings.Builder
		if n.Qual != "" {
			b.WriteString(n.Qual + ".")
		}
		b.WriteString(n.Name)
		if len(n.TypeArgs) > 0 {
			args := make([]string, len(n.TypeArgs))
			for i, t := range n.TypeArgs {
				args[i] = renderType(t)
			}
			b.WriteString("<" + strings.Join(args, ", ") + ">")
		}
		b.WriteString(" { ")
		fs := make([]string, len(n.Fields))
		for i, f := range n.Fields {
			fs[i] = f.Name + ": " + renderExpr(f.Value)
		}
		b.WriteString(strings.Join(fs, ", "))
		if n.Base != nil {
			b.WriteString(" with &" + renderExpr(n.Base))
		}
		b.WriteString(" }")
		return b.String()
	case *ast.BlockExpr:
		return renderBlock(n.Block)
	case *ast.If:
		s := "if " + renderExpr(n.Cond) + " " + renderBlock(n.Then)
		if n.Else != nil {
			s += " else " + renderExpr(n.Else)
		}
		return s
	case *ast.Match:
		arms := make([]string, len(n.Arms))
		for i, a := range n.Arms {
			arms[i] = renderPattern(a.Pat) + " => " + renderExpr(a.Body)
			if a.Guard != nil {
				arms[i] = renderPattern(a.Pat) + " if " + renderExpr(a.Guard) + " => " + renderExpr(a.Body)
			}
		}
		return "match " + renderExpr(n.Scrutinee) + " { " + strings.Join(arms, "; ") + " }"
	case *ast.Closure:
		if n.Short {
			return "|" + strings.Join(paramStrings(n.Params), ", ") + "| " + renderBlock(n.Body)
		}
		s := "fn(" + strings.Join(paramStrings(n.Params), ", ") + ")"
		if n.Ret != nil {
			s += " -> " + renderType(n.Ret)
		}
		return s + " " + renderBlock(n.Body)
	case *ast.TaskExpr:
		var b strings.Builder
		b.WriteString("task")
		writeEffect(&b, n.EffectTags)
		b.WriteString(" " + renderBlock(n.Body))
		return b.String()
	case *ast.ScopeExpr:
		s := "scope"
		if n.Timeout != nil {
			s += " timeout(" + renderExpr(n.Timeout) + ")"
		}
		if n.CollectAll {
			s += " collectAll"
		}
		return s + " " + renderBlock(n.Body)
	case *ast.SelectExpr:
		cases := make([]string, len(n.Cases))
		for i, c := range n.Cases {
			name := c.Name
			if c.Wildcard {
				name = "_"
			}
			cases[i] = "case " + name + " = " + renderExpr(c.Source) + " => " + renderExpr(c.Body)
		}
		return "select { " + strings.Join(cases, "; ") + " }"
	}
	return ""
}

func joinExprs(xs []ast.Expr) string {
	ss := make([]string, len(xs))
	for i, x := range xs {
		ss[i] = renderExpr(x)
	}
	return strings.Join(ss, ", ")
}

// renderBlock renders a block's items on one line, `; `-joined; an
// empty block braces tight.
func renderBlock(b ast.Block) string {
	if len(b.Items) == 0 {
		return "{}"
	}
	ss := make([]string, len(b.Items))
	for i, it := range b.Items {
		ss[i] = renderStmt(it)
	}
	return "{ " + strings.Join(ss, "; ") + " }"
}

func renderStmt(s ast.Stmt) string {
	switch n := s.(type) {
	case *ast.Binding:
		head := n.Kw + " "
		if n.Pat != nil {
			head += renderPattern(n.Pat)
		} else {
			head += n.Name
		}
		if n.Typ != nil {
			head += ": " + renderType(n.Typ)
		}
		return head + " = " + renderExpr(n.Init)
	case *ast.Assign:
		target := n.Name
		if n.Field != "" {
			target = "self." + n.Field
		}
		return target + " = " + renderExpr(n.Value)
	case *ast.Return:
		if n.HasValue {
			return "return " + renderExpr(n.Value)
		}
		return "return"
	case *ast.ExprStmt:
		return renderExpr(n.Expr)
	case *ast.While:
		return "while " + renderExpr(n.Cond) + " " + renderBlock(n.Body)
	case *ast.Loop:
		return "loop " + renderBlock(n.Body)
	case *ast.Break:
		return "break"
	case *ast.Continue:
		return "continue"
	case *ast.Defer:
		return "defer " + renderBlock(n.Block)
	case *ast.ForStmt:
		return "for " + renderPattern(n.Pat) + " in " + renderExpr(n.Iter) + " " + renderBlock(n.Body)
	case *ast.ScopeRes:
		binds := make([]string, len(n.Binds))
		for i, b := range n.Binds {
			binds[i] = b.Name + " = " + renderExpr(b.Val)
		}
		return "scope resource(" + strings.Join(binds, ", ") + ") " + renderBlock(n.Body)
	case *ast.MockDecl:
		var b strings.Builder
		b.WriteString("mock ")
		if n.TargetQual != "" {
			b.WriteString(n.TargetQual + ".")
		}
		b.WriteString(n.Target + "(" + strings.Join(paramStrings(n.Params), ", ") + ")")
		if n.HasRet && n.Ret != nil {
			b.WriteString(" -> " + renderType(n.Ret))
		}
		writeEffect(&b, n.EffectTags)
		b.WriteString(" " + renderBlock(n.Body))
		return b.String()
	}
	return ""
}

// renderPattern renders one pattern's canonical single-line form.
func renderPattern(p ast.Pattern) string {
	switch n := p.(type) {
	case *ast.PatLiteral:
		return n.Text
	case *ast.PatWildcard:
		return "_"
	case *ast.PatBinding:
		return n.Name
	case *ast.PatOr:
		bs := make([]string, len(n.Branches))
		for i, b := range n.Branches {
			bs[i] = renderPattern(b)
		}
		return strings.Join(bs, " | ")
	case *ast.PatTuple:
		es := make([]string, len(n.Elems))
		for i, e := range n.Elems {
			es[i] = renderPattern(e)
		}
		return "(" + strings.Join(es, ", ") + ")"
	case *ast.PatVariant:
		s := n.Name
		if n.Qualified {
			s = n.Qual + "." + s
		}
		if len(n.Args) > 0 {
			as := make([]string, len(n.Args))
			for i, a := range n.Args {
				as[i] = renderPattern(a)
			}
			s += "(" + strings.Join(as, ", ") + ")"
		}
		return s
	}
	return ""
}
