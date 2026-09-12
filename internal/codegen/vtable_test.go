package codegen

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// T6 vtable and dynamic dispatch (design D6). Three faces are pinned here:
// the impl pairing table, widened from "an interface with no arguments" to
// the face an impl actually satisfies — {interface, arguments}; the vtable
// and its thunks; and the dispatch point that reads a box's table word.

// t6Src is one program carrying every fact this file's first two tests
// read: an interface with a type parameter and a default body, a head that
// implements it at Int64, and a member call that can only resolve through
// the default's instantiation. The parameterized default is what the
// pairing row makes real — with no row the body never instantiates, and
// the call stops at the body word.
const t6Src = `import std.io

pub type AppError = Failed(String)

interface Seq<T> {
    fn get(self) -> T
    fn first(self) -> T {
        self.get()
    }
}

record Cursor { n: Int64 }

impl Seq<Int64> for Cursor {
    fn get(self) -> Int64 {
        7
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let c = Cursor { n: 0 }
    io.println("${c.first()}")
    return Ok(())
}
`

// pairEmitter builds the emitter state the pairing table is collected
// against: the module's declarations indexed the way the collect pass
// indexes them, the module's interfaces registered as its pre-pass
// registers them, and the check's own face registry. Nothing else — the
// key is a resolution over those three tables. It is built from one
// check's own tree and registry, so a test that also emits from them
// keeps the site table the emitter reads.
func pairEmitter(t *testing.T, f *ast.File, sh *typecheck.Shapes) *emitter {
	t.Helper()
	e := &emitter{
		curKey:    "main",
		shapes:    []*typecheck.Shapes{sh},
		declKeys:  declIndex([]ProgModule{{Key: "main", File: f}}),
		ifaceDefs: map[string]*ast.InterfaceDecl{},
	}
	for _, it := range f.Items {
		if d, ok := it.(*ast.InterfaceDecl); ok {
			e.ifaceDefs["main."+d.Name] = d
		}
	}
	return e
}

// TestPairingKeyNamesTheInterfaceFace: a pairing row is filed under the
// interface's own declaration key with the arguments the impl applied it
// at, and which declaration a written name denotes is the module's answer
// — its own `Seq` is `main.Seq`, while a name the module does not declare
// and the checker owns is the builtin face's bare name. An interface the
// walked module does not hold is no row at all.
func TestPairingKeyNamesTheInterfaceFace(t *testing.T) {
	f, sh := checkShapes(t, "main.we", t6Src)
	e := pairEmitter(t, f, sh)
	cases := []struct {
		what    string
		ref     ast.TypeRef
		key     string
		declKey string
		ok      bool
	}{
		{"a builtin face applied", applied("Iterator", named("Int64")), "Iterator$Int64", "Iterator", true},
		{"a builtin face bare", named("Show"), "Show", "Show", true},
		{"a module interface applied", applied("Seq", named("Int64")), "main.Seq$Int64", "main.Seq", true},
		{"an interface of another module", &ast.NamedType{Qual: "other", Name: "Seq", Args: []ast.TypeRef{named("Int64")}}, "", "", false},
		{"no interface at all", named("Nothing"), "", "", false},
	}
	for _, c := range cases {
		key, declKey, args, ok := e.ifacePairKey("main", c.ref)
		if ok != c.ok {
			t.Fatalf("%s: ok = %v, want %v", c.what, ok, c.ok)
		}
		if key != c.key || declKey != c.declKey {
			t.Fatalf("%s: key = %q declKey = %q, want %q and %q", c.what, key, declKey, c.key, c.declKey)
		}
		if c.ok && len(args) != len(appliedArgs(c.ref)) {
			t.Fatalf("%s: the row carries %d arguments, want %d", c.what, len(args), len(appliedArgs(c.ref)))
		}
	}
	// The arguments ride the row itself: a default body instantiates at
	// them, and a thunk substitutes its own positions with them.
	_, _, args, ok := e.ifacePairKey("main", applied("Iterator", named("Int64")))
	if !ok || len(args) != 1 || args[0].Kind != typecheck.ShapeBase || args[0].Name != "Int64" {
		t.Fatalf("the builtin face's arguments = %+v, want one ShapeBase Int64", args)
	}
}

// appliedArgs is the argument list a written reference carries, for the
// count above: a bare name carries none.
func appliedArgs(t ast.TypeRef) []ast.TypeRef {
	if n, ok := t.(*ast.NamedType); ok {
		return n.Args
	}
	return nil
}

// t6IterSrc is the task's own shape: an impl of the builtin parameterized
// face for a concrete head. It declares no interface — Iterator<T>'s
// method set is the check stage's — so what the pairing table files is the
// pairing itself, and the row is what a vtable is indexed by.
const t6IterSrc = `pub type AppError = Failed(String)

record Cursor { n: Int64 }

impl Iterator<Int64> for Cursor {
    fn next(mut self) -> Option<Int64> {
        None
    }
}

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`

// TestParameterizedImplEntersThePairingTable: an impl of a parameterized
// interface registers a row under {interface, arguments}, carrying the head
// that satisfied it. An impl whose interface carries no arguments is the
// same registration with no arguments to mangle (the rows the corpus
// already exercises); what this pins is that the argument list is no
// longer what keeps an impl out of the table whole.
func TestParameterizedImplEntersThePairingTable(t *testing.T) {
	f, sh := checkShapes(t, "main.we", t6IterSrc)
	e := pairEmitter(t, f, sh)
	e.methods = map[string]*fnDef{}
	e.ifacePairs = map[string]*ifacePair{}
	e.enterModule("main")
	collectImpls(t, e, f)
	p := e.ifacePairs["Iterator$Int64"]
	if p == nil {
		t.Fatalf("`impl Iterator<Int64> for Cursor` registered no pairing row; rows: %v", e.ifaceOrd)
	}
	if len(p.heads) != 1 || p.heads[0] != "main.Cursor" {
		t.Fatalf("the row's heads = %v, want [main.Cursor]", p.heads)
	}
	if p.declKey != "Iterator" || p.modKey != "main" {
		t.Fatalf("the row files %q of %q, want Iterator of main", p.declKey, p.modKey)
	}
	if len(p.args) != 1 || p.args[0].Kind != typecheck.ShapeBase || p.args[0].Name != "Int64" {
		t.Fatalf("the row's arguments = %+v, want one ShapeBase Int64", p.args)
	}
	if _, ok := e.methods["main.Cursor.next"]; !ok {
		t.Fatalf("the impl's own method did not join the method table: %v", e.methodsOrd)
	}
}

// TestParameterizedDefaultInstantiatesPerHead: an interface's default body
// becomes one define per implementing head at the arguments the impl
// applied it at, and the `self.get()` inside it resolves to that head's
// own method. The no-argument case is the same walk (see
// TestDefaultMethodInstantiatesPerHead); what this adds is the row the
// table was missing — an impl of a parameterized interface was excluded
// whole, so its default never instantiated and the call that needs it
// stopped at the body word.
func TestParameterizedDefaultInstantiatesPerHead(t *testing.T) {
	f, sh := checkShapes(t, "main.we", t6Src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define i64 @main.Cursor.first(ptr %self)", "the parameterized default's instantiation")
	wantIR(t, ir, "call i64 @main.Cursor.get(ptr %self)", "the default body's dispatch to the head's own method")
	wantNoIR(t, ir, "@main.Seq.", "the interface owns no symbol — its default instantiates per head")
}

// collectImpls registers one module's impls the way the collect pass walks
// them, so a test can read the method table the pairing rows were filed
// into — the table a thunk's forwarding target comes from.
func collectImpls(t *testing.T, e *emitter, f *ast.File) {
	t.Helper()
	for _, it := range f.Items {
		if d, ok := it.(*ast.ImplDecl); ok {
			if ni := e.collectImpl("main", d); ni != nil {
				t.Fatalf("the impl stopped at a boundary: %s", ni.What)
			}
		}
	}
}

// t6DynIterSrc is t6IterSrc plus the one site that makes a vtable real: a
// box whose payload is a gc handle and whose face has a slot. The table is
// emitted where a box is built, not where an impl is read, so the
// construction is what the pins below need.
const t6DynIterSrc = `pub type AppError = Failed(String)

record CountIter { n: Int64 }

impl Iterator<Int64> for CountIter {
    fn next(mut self) -> Option<Int64> {
        None
    }
}

pub fn main() -> Result<(), AppError> {
    let d = Dyn<Iterator<Int64> >(CountIter { n: 0 })
    return Ok(())
}
`

// TestIteratorFaceCarriesOneSlot: the builtin parameterized face has no
// source file, so its method set is the check stage's registration alone —
// and of `Iterator<T>`'s twelve methods exactly one carries no default
// body. That one is the vtable's whole content: a face's slots are its
// non-defaulted methods in declaration order (design D6 decision 1/2), so
// the table of an `Iterator<Int64>` box is one word wide and the word is
// `next`. Its signature is the shape the slot classifier must be able to
// read at Int64 — a prelude sum reached through no substitution the source
// ever wrote.
func TestIteratorFaceCarriesOneSlot(t *testing.T) {
	f, sh := checkShapes(t, "main.we", t6DynIterSrc)
	for _, face := range sh.Faces() {
		if face.Decl.Name != "Iterator" {
			continue
		}
		if len(face.Slots) != 1 {
			var names []string
			for _, s := range face.Slots {
				names = append(names, s.Name)
			}
			t.Fatalf("Iterator<T> carries %d slots (%v), want exactly next", len(face.Slots), names)
		}
		s := face.Slots[0]
		if s.Name != "next" {
			t.Fatalf("Iterator<T>'s one slot is %q, want next", s.Name)
		}
		if s.Ret == nil || s.Ret.Kind != typecheck.ShapeNominal || s.Ret.Decl.Name != "Option" || len(s.Ret.Args) != 1 {
			t.Fatalf("next's return is %+v, want the prelude Option at one position", s.Ret)
		}
		if s.Ret.Args[0].Kind != typecheck.ShapeParam || s.Ret.Args[0].Pos != 0 {
			t.Fatalf("next's return is applied at %+v, want the face's own position", s.Ret.Args[0])
		}
		// The face is the checker's, not the module's: an interface with
		// no source has no declaration node to collect.
		if _, declared := pairEmitter(t, f, sh).ifaceDefs["main.Iterator"]; declared {
			t.Fatal("Iterator is a builtin face — the module declares no Iterator")
		}
		return
	}
	t.Fatal("the check registered no Iterator face")
}

// TestVtableGlobalHoldsTheSlotsInDeclarationOrder pins the table's own
// form (design D6 decision 3): one global per {interface face, head},
// named for the face's key and the head's, a private constant array of
// slot pointers — and the contents are the interface's non-defaulted
// methods in the order they were declared, so `name` (which has a body of
// its own) is not in it and the two that are keep their written order.
func TestVtableGlobalHoldsTheSlotsInDeclarationOrder(t *testing.T) {
	f, sh := checkShapes(t, "main.we", `pub type AppError = Failed(String)

interface Shape {
    fn area(self) -> Int64
    fn name(self) -> String { "shape" }
    fn sides(self) -> Int64
}

record Square { s: Int64 }

impl Shape for Square {
    fn area(self) -> Int64 {
        4
    }
    fn sides(self) -> Int64 {
        4
    }
}

pub fn main() -> Result<(), AppError> {
    let d = Dyn<Shape>(Square { s: 1 })
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir,
		"@.vt.main.Shape.main.Square = private unnamed_addr constant [2 x ptr] "+
			"[ptr @.vt.main.Shape.main.Square.area, ptr @.vt.main.Shape.main.Square.sides]",
		"the vtable global: the face's key, the head's key, and the slots in declaration order")
	wantIR(t, ir, "define internal i64 @.vt.main.Shape.main.Square.area(ptr %box)",
		"the first slot's thunk, at the slot's own signature")
	wantIR(t, ir, "define internal i64 @.vt.main.Shape.main.Square.sides(ptr %box)",
		"the second slot's thunk")
	// `name` has a body of its own, so it is no obligation to an
	// implementor and no slot in the table: the call goes to the head's
	// own default instantiation instead.
	wantNoIR(t, ir, "@.vt.main.Shape.main.Square.name", "a slot for a defaulted method")
	wantIR(t, ir, "define { ptr, i64 } @main.Square.name(ptr %self)",
		"the defaulted method's own instantiation, where the default is dispatched")
}

// TestVtableThunkForwardsToTheImplSymbol pins the thunk's own form
// (design D6 decision 4): the thin adapter reads the receiver out of the
// box's payload and calls the implementing head's method — the same
// `define internal ... (ptr %box, ...)` shape emitFnRef's adapter takes —
// and the call names exactly the symbol the method table filed the head's
// define under, never a spelling built here.
func TestVtableThunkForwardsToTheImplSymbol(t *testing.T) {
	f, sh := checkShapes(t, "main.we", t6DynIterSrc)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// The table of the builtin face at Int64, for the head the impl
	// registered: one slot, `next`, and nothing else.
	wantIR(t, ir,
		"@.vt.Iterator$Int64.main.CountIter = private unnamed_addr constant [1 x ptr] "+
			"[ptr @.vt.Iterator$Int64.main.CountIter.next]",
		"an Iterator<Int64> box's table is one slot wide")
	// The thunk, whole: the receiver comes out of the payload at the box's
	// own offset, the rest of the slot's words forward as they are, and
	// the call is direct — the table is what made it indirect, and the
	// thunk is what makes it static again.
	wantIR(t, ir, `define internal { i64, i64, i64 } @.vt.Iterator$Int64.main.CountIter.next(ptr %box) {
entry:
  %addr = getelementptr i8, ptr %box, i64 24
  %recv = load ptr, ptr %addr
  %r = call { i64, i64, i64 } @main.CountIter.next(ptr %recv)
  ret { i64, i64, i64 } %r
}`,
		"the slot's thunk")
	// The forwarding target is the method table's own symbol. The emitter
	// is built from this same check, so the key it is read under is the
	// key the impl registered.
	e := pairEmitter(t, f, sh)
	e.methods = map[string]*fnDef{}
	e.ifacePairs = map[string]*ifacePair{}
	e.enterModule("main")
	collectImpls(t, e, f)
	fd, ok := e.methods["main.CountIter.next"]
	if !ok {
		t.Fatalf("the impl's method did not join the method table: %v", e.methodsOrd)
	}
	if fd.sym() != "main.CountIter.next" {
		t.Fatalf("the method's symbol is %q, want main.CountIter.next", fd.sym())
	}
	wantIR(t, ir, "@"+fd.sym()+"(ptr %recv)", "the thunk's call, by the method's own symbol")
	wantIR(t, ir, "define { i64, i64, i64 } @"+fd.sym()+"(ptr %self)",
		"the define that symbol names")
}

// TestVtableIsEmittedWhereABoxIsBuilt: the table is a site's, not an
// impl's. A module whose impl is never boxed carries no thunk and no
// global — the demand is the construction, so a program that only names
// the pairing pays nothing for it.
func TestVtableIsEmittedWhereABoxIsBuilt(t *testing.T) {
	f, sh := checkShapes(t, "main.we", t6IterSrc)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantNoIR(t, ir, "@.vt.", "an impl with no box site carries no table")
	wantNoIR(t, ir, "define internal", "and no thunk")
}
