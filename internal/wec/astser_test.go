package wec

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/parser"
)

// This file is the B3c AST serializer (the wec-parser change, design D4):
// the Go-side authority for the parse section's P-line family. The tower's
// reference output is the lexical serialization followed by whichever
// section internal/parser's three-return dispatch picks — the E line, the
// N line, or this file's ast section — and the wec side must match those
// bytes exactly, so the spellings below ARE the specification the port
// transcribes.
//
// The P-line family's general rule (design D4): one node per line, pre-order
// (a parent precedes its children; siblings follow source order), the line
// opening with the node name, the node's position, then every scalar field
// in struct-declaration order — booleans as 0/1, strings esc'd, int fields
// decimal. Slice-of-node fields ride as child lines in declaration order
// (an empty slice contributes no lines); a nil pointer-typed child writes a
// lone "-" line so optional slots stay positional. Variable-length []string
// fields (effect tags, derive targets, import paths, interpolation segments)
// join the P line as further esc'd fields: esc never emits a raw space, and
// identifiers never render as pure digits, so the joins stay unambiguous.

// parseSection runs the reference parser over src and renders the section
// its three-return dispatch picks: the diagnostic E line, the
// NotImplemented N line, or the ast section. Callers only reach here after
// a clean lexical scan (a lexical error is the whole output on its own).
func parseSection(src []byte) string {
	file, d, ni := parser.Parse("wec-input.we", src)
	if d != nil {
		return fmt.Sprintf("E %d %d %s %s\n", d.Line(), d.Column(), d.Code(), esc(d.Message()))
	}
	if ni != nil {
		return "N " + ni.What + "\n"
	}
	return serializeAST(file)
}

// serializeAST renders one parsed file as the success state: the ast header
// and the pre-order P lines.
func serializeAST(f *ast.File) string {
	var b strings.Builder
	b.WriteString("ast\n")
	serFile(&b, f)
	return b.String()
}

// fS/fI append one more field to a P line in progress.
func fS(b *strings.Builder, s string) { b.WriteByte(' '); b.WriteString(s) }
func fI(b *strings.Builder, i int)    { b.WriteByte(' '); b.WriteString(strconv.Itoa(i)) }

// b01 renders a bool scalar as the 0/1 field.
func b01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// serFile renders File — the section's root. File carries no position of
// its own, so its position fields are 0 (design D4's absent-position rule);
// its one scalar is the test-module fact. DocAttach children precede the
// top-level items (the struct's own field order).
func serFile(b *strings.Builder, f *ast.File) {
	b.WriteString("P file")
	fI(b, 0)
	fI(b, 0)
	fS(b, b01(f.IsTestModule))
	b.WriteByte('\n')
	for i := range f.Docs {
		serDocAttach(b, &f.Docs[i])
	}
	for _, it := range f.Items {
		serItem(b, it)
	}
}

// serDocAttach renders one documentation unit: its span and target item as
// scalars, its content lines as the same L lines the docs section uses.
// DocAttach carries no position of its own (design D4's absent-position
// rule renders 0 0).
func serDocAttach(b *strings.Builder, d *ast.DocAttach) {
	b.WriteString("P docattach")
	fI(b, 0)
	fI(b, 0)
	fI(b, d.StartLine)
	fI(b, d.EndLine)
	fI(b, d.Item)
	b.WriteByte('\n')
	for _, ln := range d.Lines {
		b.WriteString("L ")
		b.WriteString(esc(ln))
		b.WriteByte('\n')
	}
}

// serTypeParams renders a generic clause (shared by every declaration that
// carries one).
func serTypeParams(b *strings.Builder, tps []*ast.TypeParam) {
	for _, tp := range tps {
		b.WriteString("P tparam")
		fI(b, tp.Line)
		fI(b, tp.Col)
		fS(b, esc(tp.Name))
		b.WriteByte('\n')
	}
}

// serOptTypeRef renders an optional type slot: the "-" marker when absent.
func serOptTypeRef(b *strings.Builder, tr ast.TypeRef) {
	if tr == nil {
		b.WriteString("-\n")
		return
	}
	serTypeRef(b, tr)
}

// serOptExpr renders an optional expression slot.
func serOptExpr(b *strings.Builder, e ast.Expr) {
	if e == nil {
		b.WriteString("-\n")
		return
	}
	serExpr(b, e)
}

// serOptPattern renders an optional pattern slot.
func serOptPattern(b *strings.Builder, p ast.Pattern) {
	if p == nil {
		b.WriteString("-\n")
		return
	}
	serPat(b, p)
}

// serParams renders a parameter list (fn declarations, method signatures,
// closures). Param carries no position of its own — only the name's — so
// the position fields render 0 0 (design D4's absent-position rule).
func serParams(b *strings.Builder, ps []ast.Param) {
	for i := range ps {
		p := &ps[i]
		b.WriteString("P param")
		fI(b, 0)
		fI(b, 0)
		fS(b, esc(p.Name))
		fI(b, p.NameLine)
		fI(b, p.NameCol)
		b.WriteByte('\n')
		serOptTypeRef(b, p.Type) // nil on a short closure's bare parameter
	}
}

// serBlock renders a block: its position and its statement children.
func serBlock(b *strings.Builder, bl ast.Block) {
	b.WriteString("P block")
	fI(b, bl.Line)
	fI(b, bl.Col)
	b.WriteByte('\n')
	for _, st := range bl.Items {
		serStmt(b, st)
	}
}

func serItem(b *strings.Builder, it ast.Item) {
	switch v := it.(type) {
	case *ast.Import:
		b.WriteString("P import")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Alias))
		fI(b, v.PathLine)
		fI(b, v.PathCol)
		for _, seg := range v.Path {
			fS(b, esc(seg))
		}
		b.WriteByte('\n')
	case *ast.FnDecl:
		b.WriteString("P fn")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Pub))
		fS(b, esc(v.Name))
		fS(b, esc(string(v.Recv)))
		for _, t := range v.EffectTags {
			fS(b, esc(t))
		}
		fI(b, v.EffectLine)
		fI(b, v.EffectCol)
		fS(b, b01(v.Foreign))
		fI(b, v.NameLine)
		fI(b, v.NameCol)
		b.WriteByte('\n')
		serTypeParams(b, v.TypeParams)
		serParams(b, v.Params)
		serOptTypeRef(b, v.Ret)
		serWheres(b, v.Where)
		serBlock(b, v.Body)
	case *ast.TopLet:
		b.WriteString("P toplet")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Pub))
		b.WriteByte('\n')
		serBinding(b, &v.Binding)
	case *ast.SumDecl:
		b.WriteString("P sum")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Pub))
		fS(b, b01(v.Byval))
		fS(b, esc(v.Name))
		fI(b, v.NameLine)
		fI(b, v.NameCol)
		b.WriteByte('\n')
		serTypeParams(b, v.TypeParams)
		for i := range v.Variants {
			vr := &v.Variants[i]
			b.WriteString("P variant")
			fI(b, vr.Line)
			fI(b, vr.Col)
			fS(b, esc(vr.Name))
			b.WriteByte('\n')
			for _, tr := range vr.Payload {
				serTypeRef(b, tr)
			}
		}
		serOptDerives(b, v.Derives)
	case *ast.RecordDecl:
		b.WriteString("P record")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Pub))
		fS(b, esc(v.Cat))
		fS(b, esc(v.Name))
		fS(b, b01(v.Opaque))
		fI(b, v.NameLine)
		fI(b, v.NameCol)
		b.WriteByte('\n')
		serTypeParams(b, v.TypeParams)
		for i := range v.Fields {
			fd := &v.Fields[i]
			b.WriteString("P field")
			fI(b, fd.Line)
			fI(b, fd.Col)
			fS(b, esc(fd.Name))
			b.WriteByte('\n')
			serOptTypeRef(b, fd.Typ)
		}
		serOptDerives(b, v.Derives)
	case *ast.NewtypeDecl:
		b.WriteString("P newtype")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Pub))
		fS(b, esc(v.Name))
		fI(b, v.NameLine)
		fI(b, v.NameCol)
		b.WriteByte('\n')
		serTypeParams(b, v.TypeParams)
		serOptTypeRef(b, v.Underlying)
		serOptDerives(b, v.Derives)
	case *ast.InterfaceDecl:
		b.WriteString("P interface")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Pub))
		fS(b, esc(v.Name))
		fI(b, v.NameLine)
		fI(b, v.NameCol)
		b.WriteByte('\n')
		serTypeParams(b, v.TypeParams)
		for _, ad := range v.Assocs {
			b.WriteString("P assoc")
			fI(b, ad.Line)
			fI(b, ad.Col)
			fS(b, esc(ad.Name))
			b.WriteByte('\n')
		}
		for i := range v.Methods {
			serMethodSig(b, &v.Methods[i])
		}
	case *ast.ImplDecl:
		b.WriteString("P impl")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serTypeParams(b, v.TypeParams)
		serOptTypeRef(b, v.Iface)
		serTypeRef(b, v.Head)
		serWheres(b, v.Where)
		for _, ab := range v.Assocs {
			b.WriteString("P assocbind")
			fI(b, ab.Line)
			fI(b, ab.Col)
			fS(b, esc(ab.Name))
			b.WriteByte('\n')
			serTypeRef(b, ab.Type)
		}
		for _, m := range v.Methods {
			serItem(b, m)
		}
	case *ast.EffectDecl:
		b.WriteString("P effect")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Pub))
		fS(b, esc(v.Name))
		fI(b, v.NameLine)
		fI(b, v.NameCol)
		b.WriteByte('\n')
	case *ast.TestDecl:
		b.WriteString("P test")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Desc))
		fI(b, v.DescLine)
		fI(b, v.DescCol)
		b.WriteByte('\n')
		serBlock(b, v.Body)
	case *ast.ForeignBlock:
		b.WriteString("P foreign")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.ABI))
		b.WriteByte('\n')
		for _, entry := range v.Items {
			serItem(b, entry)
		}
	default:
		panic(fmt.Sprintf("astser: unknown item %T", it))
	}
}

// serMethodSig renders one interface method — a bare signature or one with
// a default body (the *Block option). MethodSig carries no position of its
// own — only the name's — so the position fields render 0 0 (design D4's
// absent-position rule).
func serMethodSig(b *strings.Builder, m *ast.MethodSig) {
	b.WriteString("P msig")
	fI(b, 0)
	fI(b, 0)
	fS(b, esc(m.Name))
	fS(b, esc(string(m.Recv)))
	for _, t := range m.EffectTags {
		fS(b, esc(t))
	}
	fI(b, m.EffectLine)
	fI(b, m.EffectCol)
	fS(b, b01(m.HasRet))
	fI(b, m.NameLine)
	fI(b, m.NameCol)
	b.WriteByte('\n')
	serTypeParams(b, m.TypeParams)
	serParams(b, m.Params)
	serOptTypeRef(b, m.Ret)
	if m.Body == nil {
		b.WriteString("-\n")
		return
	}
	serBlock(b, *m.Body)
}

// serWheres renders a where clause's bounds.
func serWheres(b *strings.Builder, ws []*ast.WhereBound) {
	for _, w := range ws {
		b.WriteString("P where")
		fI(b, w.Line)
		fI(b, w.Col)
		fS(b, esc(w.Subject))
		b.WriteByte('\n')
		for _, nt := range w.Ifaces {
			serTypeRef(b, nt)
		}
		for _, eq := range w.Eq {
			b.WriteString("P typeeq")
			fI(b, eq.Line)
			fI(b, eq.Col)
			fS(b, esc(eq.Assoc))
			b.WriteByte('\n')
			serTypeRef(b, eq.RHS)
		}
	}
}

// serOptDerives renders a derives clause slot.
func serOptDerives(b *strings.Builder, d *ast.DerivesClause) {
	if d == nil {
		b.WriteString("-\n")
		return
	}
	b.WriteString("P derives")
	fI(b, d.Line)
	fI(b, d.Col)
	for _, t := range d.Targets {
		fS(b, esc(t))
	}
	b.WriteByte('\n')
}

func serStmt(b *strings.Builder, st ast.Stmt) {
	switch v := st.(type) {
	case *ast.Binding:
		serBinding(b, v)
	case *ast.MockDecl:
		serMock(b, v)
	case *ast.Assign:
		b.WriteString("P assign")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Name))
		fS(b, esc(v.Field))
		fI(b, v.OpLine)
		fI(b, v.OpCol)
		b.WriteByte('\n')
		serExpr(b, v.Value)
	case *ast.Return:
		b.WriteString("P return")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.HasValue))
		b.WriteByte('\n')
		serOptExpr(b, v.Value)
	case *ast.ExprStmt:
		b.WriteString("P exprstmt")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serExpr(b, v.Expr)
	case *ast.While:
		b.WriteString("P while")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serExpr(b, v.Cond)
		serBlock(b, v.Body)
	case *ast.Loop:
		b.WriteString("P loop")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serBlock(b, v.Body)
	case *ast.Break:
		fmt.Fprintf(b, "P break %d %d\n", v.Line, v.Col)
	case *ast.Continue:
		fmt.Fprintf(b, "P continue %d %d\n", v.Line, v.Col)
	case *ast.Defer:
		b.WriteString("P defer")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serBlock(b, v.Block)
	case *ast.ForStmt:
		b.WriteString("P for")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serPat(b, v.Pat)
		serExpr(b, v.Iter)
		serBlock(b, v.Body)
	case *ast.ScopeRes:
		b.WriteString("P scoperes")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		for i := range v.Binds {
			sb := &v.Binds[i]
			b.WriteString("P scopebind")
			fI(b, sb.Line)
			fI(b, sb.Col)
			fS(b, esc(sb.Name))
			b.WriteByte('\n')
			serExpr(b, sb.Val)
		}
		serBlock(b, v.Body)
	default:
		panic(fmt.Sprintf("astser: unknown stmt %T", st))
	}
}

// serBinding renders the let/var binding — both the statement form and the
// TopLet's carried one.
func serBinding(b *strings.Builder, v *ast.Binding) {
	b.WriteString("P binding")
	fI(b, v.Line)
	fI(b, v.Col)
	fS(b, esc(v.Kw))
	fS(b, esc(v.Name))
	fI(b, v.NameLine)
	fI(b, v.NameCol)
	b.WriteByte('\n')
	serOptPattern(b, v.Pat)
	serOptTypeRef(b, v.Typ)
	serExpr(b, v.Init)
}

// serMock renders a mock declaration (a direct item of a test body).
func serMock(b *strings.Builder, v *ast.MockDecl) {
	b.WriteString("P mock")
	fI(b, v.Line)
	fI(b, v.Col)
	fS(b, esc(v.Target))
	fS(b, esc(v.TargetQual))
	for _, t := range v.EffectTags {
		fS(b, esc(t))
	}
	fI(b, v.EffectLine)
	fI(b, v.EffectCol)
	fS(b, b01(v.HasRet))
	fI(b, v.TargetLine)
	fI(b, v.TargetCol)
	b.WriteByte('\n')
	serParams(b, v.Params)
	serOptTypeRef(b, v.Ret)
	serBlock(b, v.Body)
}

func serExpr(b *strings.Builder, e ast.Expr) {
	switch v := e.(type) {
	case *ast.Prop:
		b.WriteString("P prop")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serExpr(b, v.X)
	case *ast.Ident:
		fmt.Fprintf(b, "P ident %d %d %s\n", v.Line, v.Col, esc(v.Name))
	case *ast.Literal:
		b.WriteString("P literal")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Kind))
		fS(b, esc(v.Text))
		for _, seg := range v.Segs {
			fS(b, esc(seg))
		}
		b.WriteByte('\n')
		for _, h := range v.Holes {
			serExpr(b, h)
		}
	case *ast.Unary:
		b.WriteString("P unary")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Op))
		b.WriteByte('\n')
		serExpr(b, v.X)
	case *ast.Binary:
		b.WriteString("P binary")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Op))
		b.WriteByte('\n')
		serExpr(b, v.L)
		serExpr(b, v.R)
	case *ast.Call:
		b.WriteString("P call")
		fI(b, v.Line)
		fI(b, v.Col)
		fI(b, v.ArgLine)
		fI(b, v.ArgCol)
		b.WriteByte('\n')
		serExpr(b, v.Fn)
		for _, a := range v.Args {
			serExpr(b, a)
		}
		for _, tr := range v.TypeArgs {
			serTypeRef(b, tr)
		}
	case *ast.Member:
		b.WriteString("P member")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Name))
		fI(b, v.NameLine)
		fI(b, v.NameCol)
		b.WriteByte('\n')
		serExpr(b, v.Recv)
	case *ast.BlockExpr:
		b.WriteString("P blockexpr")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serBlock(b, v.Block)
	case *ast.Unit:
		fmt.Fprintf(b, "P unit %d %d\n", v.Line, v.Col)
	case *ast.If:
		b.WriteString("P if")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serExpr(b, v.Cond)
		serBlock(b, v.Then)
		serOptExpr(b, v.Else)
	case *ast.Match:
		b.WriteString("P match")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		serExpr(b, v.Scrutinee)
		for i := range v.Arms {
			a := &v.Arms[i]
			b.WriteString("P arm")
			fI(b, a.Line)
			fI(b, a.Col)
			b.WriteByte('\n')
			serPat(b, a.Pat)
			serOptExpr(b, a.Guard)
			serExpr(b, a.Body)
		}
	case *ast.TaskExpr:
		b.WriteString("P task")
		fI(b, v.Line)
		fI(b, v.Col)
		for _, t := range v.EffectTags {
			fS(b, esc(t))
		}
		fI(b, v.EffectLine)
		fI(b, v.EffectCol)
		b.WriteByte('\n')
		serBlock(b, v.Body)
	case *ast.ScopeExpr:
		b.WriteString("P scopeexpr")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.CollectAll))
		b.WriteByte('\n')
		serOptExpr(b, v.Timeout)
		serBlock(b, v.Body)
	case *ast.SelectExpr:
		b.WriteString("P select")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		for i := range v.Cases {
			c := &v.Cases[i]
			b.WriteString("P selectcase")
			fI(b, c.Line)
			fI(b, c.Col)
			fS(b, b01(c.Wildcard))
			fS(b, esc(c.Name))
			b.WriteByte('\n')
			serExpr(b, c.Source)
			serExpr(b, c.Body)
		}
	case *ast.Construct:
		b.WriteString("P construct")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Qual))
		fS(b, esc(v.Name))
		fI(b, v.ArgLine)
		fI(b, v.ArgCol)
		b.WriteByte('\n')
		for _, tr := range v.TypeArgs {
			serTypeRef(b, tr)
		}
		for i := range v.Fields {
			fi := &v.Fields[i]
			b.WriteString("P fieldinit")
			fI(b, fi.Line)
			fI(b, fi.Col)
			fS(b, esc(fi.Name))
			b.WriteByte('\n')
			serExpr(b, fi.Value)
		}
		serOptExpr(b, v.Base)
	case *ast.Tuple:
		b.WriteString("P tuple")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		for _, el := range v.Elems {
			serExpr(b, el)
		}
	case *ast.Closure:
		b.WriteString("P closure")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Short))
		b.WriteByte('\n')
		serParams(b, v.Params)
		serOptTypeRef(b, v.Ret)
		serBlock(b, v.Body)
	case *ast.ListLit:
		b.WriteString("P listlit")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		for _, el := range v.Elems {
			serExpr(b, el)
		}
	default:
		panic(fmt.Sprintf("astser: unknown expr %T", e))
	}
}

func serPat(b *strings.Builder, p ast.Pattern) {
	switch v := p.(type) {
	case *ast.PatLiteral:
		b.WriteString("P patlit")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Kind))
		fS(b, esc(v.Text))
		b.WriteByte('\n')
	case *ast.PatWildcard:
		fmt.Fprintf(b, "P patwild %d %d\n", v.Line, v.Col)
	case *ast.PatBinding:
		fmt.Fprintf(b, "P patbind %d %d %s\n", v.Line, v.Col, esc(v.Name))
	case *ast.PatOr:
		b.WriteString("P pator")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		for _, br := range v.Branches {
			serPat(b, br)
		}
	case *ast.PatTuple:
		b.WriteString("P pattuple")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		for _, el := range v.Elems {
			serPat(b, el)
		}
	case *ast.PatVariant:
		b.WriteString("P patvariant")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, b01(v.Qualified))
		fS(b, esc(v.Qual))
		fS(b, esc(v.Name))
		b.WriteByte('\n')
		for _, a := range v.Args {
			serPat(b, a)
		}
	default:
		panic(fmt.Sprintf("astser: unknown pattern %T", p))
	}
}

func serTypeRef(b *strings.Builder, tr ast.TypeRef) {
	switch v := tr.(type) {
	case *ast.NamedType:
		b.WriteString("P named")
		fI(b, v.Line)
		fI(b, v.Col)
		fS(b, esc(v.Qual))
		fS(b, esc(v.Name))
		fI(b, v.ArgLine)
		fI(b, v.ArgCol)
		b.WriteByte('\n')
		for _, a := range v.Args {
			serTypeRef(b, a)
		}
	case *ast.TupleType:
		b.WriteString("P tupletype")
		fI(b, v.Line)
		fI(b, v.Col)
		b.WriteByte('\n')
		for _, el := range v.Elems {
			serTypeRef(b, el)
		}
	case *ast.UnitType:
		fmt.Fprintf(b, "P unittype %d %d\n", v.Line, v.Col)
	case *ast.FnType:
		b.WriteString("P fntype")
		fI(b, v.Line)
		fI(b, v.Col)
		for _, t := range v.EffectTags {
			fS(b, esc(t))
		}
		fI(b, v.TagLine)
		fI(b, v.TagCol)
		b.WriteByte('\n')
		for _, p := range v.Params {
			serTypeRef(b, p)
		}
		serOptTypeRef(b, v.Ret)
	default:
		panic(fmt.Sprintf("astser: unknown typeref %T", tr))
	}
}

// TestParseSectionPinsTheFormat anchors the parse section's own shape with
// hand-written bytes — the format-pin posture of the lexical pin in
// wec_test.go: these pin the SPEC, not parser behavior, so a serializer
// rewrite on either side of the tower fails here first. The empty source is
// the minimal shape (File's absent position renders as 0 0 and its
// test-module fact as the third field); the one-binding source pins the
// pre-order parent/child lines and the "-" markers of Binding's optional
// pattern and type slots.
func TestParseSectionPinsTheFormat(t *testing.T) {
	if got := parseSection([]byte("")); got != "ast\nP file 0 0 0\n" {
		t.Fatalf("empty source parse section: %q", got)
	}
	want := "ast\n" +
		"P file 0 0 0\n" +
		"P toplet 1 1 0\n" +
		"P binding 1 1 let x 1 5\n" +
		"-\n" +
		"-\n" +
		"P literal 1 9 int 1\n"
	if got := parseSection([]byte("let x = 1\n")); got != want {
		t.Fatalf("one-binding source parse section:\nwant %q\ngot  %q", want, got)
	}
}
