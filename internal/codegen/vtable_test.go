package codegen

import (
	"regexp"
	"strings"
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

// t6DispatchSrc is the task's dispatch shape: the box's method is called,
// so the call site has to read the table the construction emitted. The
// impl's body both reads and writes its receiver, so what the thunk hands
// over is a real object and not a value the call could have carried.
const t6DispatchSrc = `import std.io

pub type AppError = Failed(String)

record CountIter { n: Int64, hi: Int64 }

impl Iterator<Int64> for CountIter {
    fn next(mut self) -> Option<Int64> {
        if self.n >= self.hi {
            return None
        }
        let v = self.n
        self.n = self.n + 1
        return Some(v)
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Iterator<Int64> >(CountIter { n: 10, hi: 12 })
    let a = d.next()
    match a {
        Some(v) => { io.println("${v}") }
        None => { io.println("none") }
    }
    return Ok(())
}
`

// dispatchRe matches one method call through a box's table, capturing each
// register so the chain can be checked link by link: Go's regexp has no
// backreferences, so what lines up is asserted below rather than in the
// pattern. The slot's own offset is a capture — a table holding more than
// one slot only dispatches correctly if the index is the member's — and so
// is the return face, since the call is emitted at the slot's face and not
// at a common one.
var dispatchRe = regexp.MustCompile(
	`(%v\d+) = getelementptr i8, ptr (%v\d+), i64 16\n` +
		`\s+(%v\d+) = load ptr, ptr (%v\d+)\n` +
		`\s+(%v\d+) = getelementptr i8, ptr (%v\d+), i64 (\d+)\n` +
		`\s+(%v\d+) = load ptr, ptr (%v\d+)\n` +
		`\s+(%v\d+) = call (.+?) (%v\d+)\(ptr (%v\d+)\)`)

// wantDispatch asserts one dispatch's whole load chain and answers the
// slot's offset and the return face the call was emitted at. The groups
// are, in order: the address the table is loaded from, the box, the table,
// the two registers each load repeats, the slot's address, the slot's
// offset, the call's register, the return face, the thunk and the receiver
// passed.
func wantDispatch(t *testing.T, ir string) (slot, face string) {
	t.Helper()
	m := dispatchRe.FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("no dispatch through the box's table:\n%s", ir)
	}
	for _, c := range []struct {
		what      string
		got, want string
	}{
		{"the table is loaded from the address just computed", m[4], m[1]},
		{"the slot is taken out of the loaded table", m[6], m[3]},
		{"the thunk is loaded from the slot's address", m[9], m[5]},
		{"the call goes through the loaded thunk, not a symbol", m[12], m[8]},
		{"the receiver passed is the box itself, not its payload", m[13], m[2]},
	} {
		if c.got != c.want {
			t.Fatalf("%s: %s vs %s\n--\n%s", c.what, c.got, c.want, ir)
		}
	}
	return m[7], m[11]
}

// TestDispatchReadsTheSlotOutOfTheBox pins the dispatch point's own form
// (design D6 decision 5): the table word at the box's own offset, the slot
// at its index within that table, the loaded thunk as the callee — the
// same load a fn-value call takes out of its carrier, at the same offsets.
//
// What is passed is the box, not its payload: the thunk is the one that
// knows the payload's face, so unwrapping belongs to it and the call site
// stays free of the face. And the slot's own ABI is what the call is
// emitted at, so a sum return crosses whole instead of being narrowed to
// the words a common return face would allow.
func TestDispatchReadsTheSlotOutOfTheBox(t *testing.T) {
	f, sh := checkShapes(t, "main.we", t6DispatchSrc)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	if slot, face := wantDispatch(t, ir); face != "{ i64, i64, i64 }" || slot != "0" {
		t.Fatalf("the call is emitted at %q out of slot %s, want the slot's own sum face at the face's only slot", face, slot)
	}
}

// TestDispatchIsEmittedOnlyWhereATableWas pins the boundary the dispatch
// shares with the table (design D6 decision 4's refutation, which is where
// it becomes load-bearing): a box whose payload face is a value has no
// table — the word at 16 is the null T5 wrote — so a call through it is
// not emitted at all. The face a value carries says which slots it would
// be dispatched at, never that a table is there to read, and a jump
// through the null would be the wrong answer rather than a missing one.
//
// `Celsius` is the carrier of exactly that shape: a newtype boxed behind
// a face whose one slot `describe` is a method it implements. What the
// test above dispatches, this one declines, and the payload face is the
// whole of the difference.
func TestDispatchIsEmittedOnlyWhereATableWas(t *testing.T) {
	f, sh := checkShapes(t, "main.we", `pub type AppError = Failed(String)

interface Describe {
    fn describe(self) -> String
}

newtype Celsius(Int64)

impl Describe for Celsius {
    fn describe(self) -> String {
        "c"
    }
}

pub fn main() -> Result<(), AppError> {
    let d = Dyn<Describe>(Celsius(1))
    let s = d.describe()
    return Ok(())
}
`)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni == nil {
		t.Fatal("a call through a box whose payload is a value emitted")
	}
}

// TestDispatchOnAGcPayloadBoxIsEmitted is the same program as the pin
// above with the one difference that decides it — the payload is a record,
// so the table was emitted and the call resolves through it. The two are
// one predicate apart, which is what keeps the boundary from being a
// coincidence of the source shape.
func TestDispatchOnAGcPayloadBoxIsEmitted(t *testing.T) {
	f, sh := checkShapes(t, "main.we", `pub type AppError = Failed(String)

interface Describe {
    fn describe(self) -> String
}

record Point {
    x: Int64,
}

impl Describe for Point {
    fn describe(self) -> String {
        "p"
    }
}

pub fn main() -> Result<(), AppError> {
    let d = Dyn<Describe>(Point { x: 1 })
    let s = d.describe()
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@.vt.main.Describe.main.Point", "the table of a gc payload box")
	if slot, face := wantDispatch(t, ir); face != "{ ptr, i64 }" || slot != "0" {
		t.Fatalf("the call is emitted at %q out of slot %s, want the slot's own String face", face, slot)
	}
}

// TestDispatchTakesTheSlotAtItsOwnIndex pins that the slot read is indexed
// by the member's place in the table and not by the face's declaration
// order: `name` has a body of its own and so holds no slot, which makes
// `sides` the interface's third declaration and the table's second entry.
// A call that read the first word, or read at the declaration's index,
// would reach `area`'s thunk — or past the array — and the offset is the
// only thing in the IR that tells them apart.
func TestDispatchTakesTheSlotAtItsOwnIndex(t *testing.T) {
	f, sh := checkShapes(t, "main.we", `import std.io

pub type AppError = Failed(String)

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
        5
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Shape>(Square { s: 1 })
    let n = d.sides()
    io.println("${n}")
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// The table is where the index means something: `sides` is its second
	// entry, so a call reading the second word reaches the head's own
	// `sides` and nothing else.
	wantIR(t, ir,
		"@.vt.main.Shape.main.Square = private unnamed_addr constant [2 x ptr] "+
			"[ptr @.vt.main.Shape.main.Square.area, ptr @.vt.main.Shape.main.Square.sides]",
		"the table the index is read against")
	if slot, face := wantDispatch(t, ir); slot != "8" || face != "i64" {
		t.Fatalf("the call reads slot offset %s at face %q, want the second slot at i64", slot, face)
	}
}

// TestVtableIsStaticAndNeverAllocated pins design D6's negative boundary:
// a table is a compile-time constant and a thunk is a define, so neither is
// ever heap-allocated. D6 decision 2 leaves a box without a tag because
// there is no downcast to read one with, and that is the same fact from the
// other side — the table is reached by the code that built the box, not by
// asking the box what it is.
func TestVtableIsStaticAndNeverAllocated(t *testing.T) {
	f, sh := checkShapes(t, "main.we", t6DispatchSrc)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir,
		"@.vt.Iterator$Int64.main.CountIter = private unnamed_addr constant [1 x ptr] "+
			"[ptr @.vt.Iterator$Int64.main.CountIter.next]",
		"the table as a constant initializer, so nothing allocates it")
	for _, line := range strings.Split(ir, "\n") {
		if !strings.Contains(line, "@.vt.") {
			continue
		}
		if strings.Contains(line, "__we_alloc") {
			t.Fatalf("a table or thunk is heap-allocated: %s", line)
		}
	}
	for _, m := range dispatchAllocRe.FindAllString(ir, -1) {
		t.Fatalf("a call produces a table: %s", m)
	}
}

// dispatchAllocRe matches any call whose result names a table or thunk: a
// table is data the module owns, so the only reads of one are the address
// it is stored by and the loads made through that address.
var dispatchAllocRe = regexp.MustCompile(`= call [^\n]*@\.vt\.`)

// TestTwoFacesOverOneHeadKeepSeparateTables pins the other negative half of
// D6's boundary: one head behind two interfaces gets one table per face and
// no merged one. A merged table would have to hold a slot for each face's
// methods, and a call through either would then read an index that only one
// of the two faces agrees with — the widths and the orderings are the
// interfaces', and nothing here reconciles them.
func TestTwoFacesOverOneHeadKeepSeparateTables(t *testing.T) {
	f, sh := checkShapes(t, "main.we", `import std.io

pub type AppError = Failed(String)

interface A { fn a(self) -> Int64 }
interface B { fn b(self) -> Int64 }

record R { x: Int64 }

impl A for R {
    fn a(self) -> Int64 { 1 }
}
impl B for R {
    fn b(self) -> Int64 { 2 }
}

pub fn main() effect io -> Result<(), AppError> {
    let p = Dyn<A>(R { x: 1 })
    let q = Dyn<B>(R { x: 2 })
    let s = p.a()
    let t = q.b()
    io.println("${s}")
    io.println("${t}")
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@.vt.main.A.main.R = private unnamed_addr constant [1 x ptr] [ptr @.vt.main.A.main.R.a]",
		"the first face's own table, holding only its own slot")
	wantIR(t, ir, "@.vt.main.B.main.R = private unnamed_addr constant [1 x ptr] [ptr @.vt.main.B.main.R.b]",
		"the second face's own table, holding only its own slot")
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
