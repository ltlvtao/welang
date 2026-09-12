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
// key is a resolution over those three tables.
func pairEmitter(t *testing.T, src string) (*emitter, *ast.File) {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
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
	return e, f
}

// TestPairingKeyNamesTheInterfaceFace: a pairing row is filed under the
// interface's own declaration key with the arguments the impl applied it
// at, and which declaration a written name denotes is the module's answer
// — its own `Seq` is `main.Seq`, while a name the module does not declare
// and the checker owns is the builtin face's bare name. An interface the
// walked module does not hold is no row at all.
func TestPairingKeyNamesTheInterfaceFace(t *testing.T) {
	e, _ := pairEmitter(t, t6Src)
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
	e, f := pairEmitter(t, t6IterSrc)
	e.methods = map[string]*fnDef{}
	e.ifacePairs = map[string]*ifacePair{}
	e.enterModule("main")
	for _, it := range f.Items {
		if d, ok := it.(*ast.ImplDecl); ok {
			if ni := e.collectImpl("main", d); ni != nil {
				t.Fatalf("the impl stopped at a boundary: %s", ni.What)
			}
		}
	}
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
