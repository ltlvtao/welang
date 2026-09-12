package codegen

import (
	"strconv"
	"strings"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// This file is B1b's naming layer (design D2): the mangler that renders
// one instantiation's type arguments into a symbol suffix, and the table
// that gives every {declaration, arguments} pair one symbol. Naming lives
// in codegen because codegen already owns it — a symbol's key is the
// module-qualified key the fn table and the method table spell — and the
// checker's Shape view carries no rendering on purpose, since a
// declaration's own name is not its identity (two modules may declare the
// same one).
//
// A declaration with no type arguments takes no suffix: every symbol B1a
// spelled rides through sym() byte for byte, which is what keeps the
// existing IR snapshots and the conformance corpus still.

// declIndex maps every declaration node in the program to the
// module-qualified key its symbols are spelled with ("<module>.<Name>").
// It is built over the module list rather than during pass one's walk so
// that a generic declaration is keyed too: the walk stops at B1a's
// generic boundary, but a generic declaration is exactly what an
// instantiation names.
func declIndex(mods []ProgModule) map[ast.Item]string {
	keys := make(map[ast.Item]string)
	for _, m := range mods {
		for _, it := range m.File.Items {
			switch d := it.(type) {
			case *ast.SumDecl:
				keys[d] = m.Key + "." + d.Name
			case *ast.RecordDecl:
				keys[d] = m.Key + "." + d.Name
			case *ast.NewtypeDecl:
				keys[d] = m.Key + "." + d.Name
			case *ast.InterfaceDecl:
				keys[d] = m.Key + "." + d.Name
			case *ast.ForeignBlock:
				// An opaque record declares no symbol of its own (chapter
				// 19: no layout, no constructor), but it is a declaration a
				// type argument can name, so it keys like any other.
				for _, x := range d.Items {
					if r, ok := x.(*ast.RecordDecl); ok {
						keys[r] = m.Key + "." + r.Name
					}
				}
			}
		}
	}
	return keys
}

// declKey renders one declaration's identity inside a mangled type: its
// module-qualified key, which is codegen's own naming (design D1 hands
// the checker's shapes over bare, because a declaration's name alone does
// not identify it). A builtin declaration carries its name instead: the
// builtin faces and collections are package singletons with no source
// file, so no two of them can collide.
func (e *emitter) declKey(d typecheck.ShapeDecl) string {
	if d.Name != "" {
		return d.Name
	}
	key, ok := e.declKeys[d.Node]
	if !ok {
		// The index keys every declaration of every module the emitter
		// walks, so a miss is a shape from another program — never a
		// user's.
		panic("codegen: mangling a declaration outside the program")
	}
	return key
}

// mangleShape renders one type as design D2's encoding. The alphabet is
// the one LLVM's bare identifiers admit ([-a-zA-Z$._][-a-zA-Z$._0-9]*):
// a symbol is spelled into the IR unquoted. "$" separates every segment —
// a character a We identifier can never contain (the lexical chapter's
// closed inventory names it among those that never form a token), so no
// segment can be read as a name.
func (e *emitter) mangleShape(s typecheck.Shape) string {
	switch s.Kind {
	case typecheck.ShapeBase:
		return s.Name
	case typecheck.ShapeUnit:
		return "U"
	case typecheck.ShapeNever:
		return "N"
	case typecheck.ShapeTuple:
		// T$<n>$<elems>: the arity is written out, so the reading of the
		// elements that follow stays left to right.
		return "T$" + strconv.Itoa(len(s.Elems)) + "$" + e.mangleList(s.Elems)
	case typecheck.ShapeFn:
		// F$<params>_<ret>: "_" divides the parameter faces from the one
		// return, and a fn declaring none reads V — a valueless fn and a
		// unit-returning one are different types (design D1's second
		// correction), so they must not mangle alike.
		ret := "V"
		if s.Ret != nil {
			ret = e.mangleShape(*s.Ret)
		}
		return "F$" + e.mangleList(s.Params) + "_" + ret
	case typecheck.ShapeNominal:
		return e.mangleApply(e.declKey(s.Decl), s.Args)
	case typecheck.ShapeDyn:
		// The erased box is a type of its own (chapter 10): Dyn<I> is not
		// I, so the face it erases is marked as boxed.
		return "Dyn$" + e.mangleApply(e.declKey(s.Decl), s.Args)
	case typecheck.ShapeParam, typecheck.ShapeAssoc:
		// A position the site left open names no instantiation: only a
		// resolved application reaches a symbol. Registering one that
		// still carries the enclosing declaration's own position is the
		// driving layer's mistake (design D3), never a form to encode.
		panic("codegen: mangling an unresolved type position")
	}
	panic("codegen: mangling an unknown shape")
}

// mangleApply renders one declaration and the arguments it is applied to:
// the key alone where it takes none, else the arguments appended.
func (e *emitter) mangleApply(key string, args []typecheck.Shape) string {
	if len(args) == 0 {
		return key
	}
	return key + "$" + e.mangleList(args)
}

// mangleSuffix renders one instantiation's symbol suffix, or nothing at
// all where it has no type arguments — the empty suffix every B1a symbol
// keeps.
func (e *emitter) mangleSuffix(args []typecheck.Shape) string {
	if len(args) == 0 {
		return ""
	}
	return "$" + e.mangleList(args)
}

// mangleList renders a position list, "$"-separated.
func (e *emitter) mangleList(args []typecheck.Shape) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = e.mangleShape(a)
	}
	return strings.Join(parts, "$")
}

// instTable is the program's instantiation registry (design D2): every
// {declaration, type arguments} the emitter has reached, in registration
// order — the order their defines emit in (design D3), the determinism
// the method table's order established. The symbol is the entry's
// identity, so interning here is what decides that two sites are one
// instantiation.
type instTable struct {
	bySym map[string]int // symbol -> index into order
	order []inst
}

// inst is one registered instantiation: the declaration's key, the suffix
// its arguments render (so the symbol is the two joined), and the
// arguments themselves — kept so a symbol met a second time can be
// checked against what it claims to name.
type inst struct {
	decl   string
	suffix string
	args   []typecheck.Shape
}

// register interns one instantiation and returns its symbol. The
// arguments are the caller's truth and the suffix their rendering: a
// symbol met again under a different argument list is one name for two
// types, which no later stage could tell apart — it fails here rather
// than emitting one define for both.
func (t *instTable) register(decl, suffix string, args []typecheck.Shape) string {
	if t.bySym == nil {
		t.bySym = make(map[string]int)
	}
	sym := decl + suffix
	if i, seen := t.bySym[sym]; seen {
		if !sameShapes(t.order[i].args, args) {
			panic("codegen: " + sym + " names two instantiations")
		}
		return sym
	}
	t.bySym[sym] = len(t.order)
	t.order = append(t.order, inst{decl: decl, suffix: suffix, args: args})
	return sym
}

// sameShapes reports whether two argument lists are the same types by the
// checker's own identity rules (Shape.Equal).
func sameShapes(a, b []typecheck.Shape) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}
