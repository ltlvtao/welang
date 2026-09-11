package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T8-2B's face: a module-level binding whose value is a collectable handle
// (a gc record, a List) keeps that handle in one global of its own, and the
// compiler hands the collector the global's ADDRESS at the entry head.
//
// The root table exists because no shadow stack sees a global: the root
// protocol's __we_root_push covers the roots a body holds in registers, and
// a module-level binding's handle lives outside every body. Registering the
// address rather than the value is what makes a store barrier unnecessary —
// the slots are zeroed statics, a collection before the first store reads
// NULL and skips them, and a collection after it reads whatever the store
// left. Design D7's "root barrier around the initializers" is answered that
// way rather than by a barrier: a barrier orders a VALUE into a scan, and
// there is no value in a slot the scan dereferences itself.
//
// A String binding is NOT registered, and that is design D3 rather than an
// omission: a String's bytes are a private constant or a malloc'd buffer,
// and the mark phase would read the first word of one as a block header.
// See the T8-2A record's first ruling.
//
// The pins below are the emitted shape; the runtime consequence is
// run-toplet-gc-*'s goldens.

// topGcIR emits the given modules and fails on a boundary: every pin here
// is about the emission's shape rather than about a stop.
func topGcIR(t *testing.T, mods ...ProgModule) string {
	t.Helper()
	ir, ni := EmitProgram(ModeBuild, mods)
	if ni != nil {
		t.Fatalf("expected clean emission, got boundary %q", ni.What)
	}
	return ir
}

// gcNode is the record every pin below binds at module level: one Int64
// field, so the handle is a block with a layout descriptor and one traced
// slot's worth of payload.
func gcNode() *ast.RecordDecl { return recDecl("Node", "gc", fld("value", "Int64")) }

func nodeInit(v string) *ast.Construct { return construct("Node", init1("value", intLit(v))) }

// TestTopLetGcRecordOwnsAHandleGlobal: one global holding the handle
// itself. Not the String shape's pair — a handle is one word, and the read
// is one load.
func TestTopLetGcRecordOwnsAHandleGlobal(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{gcNode(), topLetDecl("g", named("Node"), nodeInit("42"))}))
	wantIR(t, ir, "@main.g = internal global ptr null", "the handle's own global")
	wantNoIR(t, ir, "@main.g.p", "a handle is not the String pair")
	wantNoIR(t, ir, "@main.g.len", "a handle is not the String pair")
}

// TestTopLetGcRegistersTheSlotAheadOfTheInit: the collector has to know a
// slot before an initializer can put a handle in it, so the registration is
// the entry's own head and the init call comes after it.
func TestTopLetGcRegistersTheSlotAheadOfTheInit(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{gcNode(), topLetDecl("g", named("Node"), nodeInit("42"))}))
	wantIR(t, ir, "declare void @__we_gc_root_global(ptr)", "the runtime entry point")
	wantIR(t, ir, "call void @__we_gc_root_global(ptr @main.g)", "the slot's address")
	entry := entryTail(ir)
	reg := strings.Index(entry, "@__we_gc_root_global(ptr @main.g)")
	call := strings.Index(entry, "call void @main.init()")
	if reg < 0 || call < 0 || reg > call {
		t.Fatalf("the registration must precede the init call:\n%s", entry)
	}
}

// TestTopLetGcInitStoresTheHandleAndDropsTheName: the initializer binds
// through the ordinary gc path and the store takes the handle from there;
// the name is then dropped, so every later read takes the global's load
// rather than a register no collection can see.
func TestTopLetGcInitStoresTheHandleAndDropsTheName(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{gcNode(), topLetDecl("g", named("Node"), nodeInit("42"))},
		letBind("k", memberOf(ident("g"), "value")),
	))
	init := initBody(t, ir, "main")
	wantIR(t, init, "store ptr %v", "the handle is stored from a register")
	wantIR(t, init, ", ptr @main.g", "into the binding's own global")
	wantIR(t, ir, "%v0 = load ptr, ptr @main.g", "the later read is the global's load")
}

// TestTopLetGcReadIsOneLoad: a module-level handle's field read is the
// load of the global, then the ordinary hop walk — the same two steps a
// local gc binding's read takes, with the register load replaced.
func TestTopLetGcReadIsOneLoad(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{gcNode(), topLetDecl("g", named("Node"), nodeInit("42"))},
		letBind("k", memberOf(ident("g"), "value")),
	))
	wantIR(t, ir, "  %v0 = load ptr, ptr @main.g\n  %v1 = getelementptr i8, ptr %v0, i64 16\n  %v2 = load i64, ptr %v1\n",
		"the global's load feeds the hop walk")
}

// TestTopLetListOwnsAHandleGlobalAndRegisters: a List binding takes the
// same storage face. Its read is not the record face — a list is walked,
// not field-read — so the binding site takes the carrier and the element
// face together, which is what listEnv holds for a local one.
func TestTopLetListOwnsAHandleGlobalAndRegisters(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{
		topLetDecl("ns", &ast.NamedType{Name: "List", Args: []ast.TypeRef{named("Int64")}},
			&ast.ListLit{Elems: []ast.Expr{intLit("1"), intLit("2")}}),
	},
		walk("x", ident("ns"), letBind("z", ident("x"))),
	))
	wantIR(t, ir, "@main.ns = internal global ptr null", "the carrier's own global")
	wantIR(t, ir, "call void @__we_gc_root_global(ptr @main.ns)", "the slot's address")
	wantIR(t, ir, "load ptr, ptr @main.ns", "the walk's source is the global's load")
}

// The two pins below were added later, from T8-2B's mutation battery:
// three
// mutations survived it, and all three turn out to be the same untested
// shape — a module-level gc binding COPIED into a local by a plain `let`.
// `bindTopRead` has exactly one caller (the `let` statement's top-level
// name arm), and the pins above reach a top-level binding by reading it
// in place — through `for x in ns` and through a member of the binding
// itself — so the copy arm was the one branch no program ran. The two
// faces it carries are the record key and the list element face, and
// dropping either turns the copy into a boundary stop rather than a
// silent wrong answer, which is why a clean-emission pin is enough to
// bite.

// TestTopLetGcCopiedToALocalKeepsTheRecordFace: `let k = g` over a
// module-level record binding hands the local the record's key along with
// the handle, so `k.value` walks the same layout `g.value` would. A copy
// that kept the pointer and lost the key would have a handle it could not
// read a field out of.
func TestTopLetGcCopiedToALocalKeepsTheRecordFace(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{gcNode(), topLetDecl("g", named("Node"), nodeInit("42"))},
		letBind("k", ident("g")),
		letBind("n", memberOf(ident("k"), "value")),
	))
	wantIR(t, ir, "  %v0 = load ptr, ptr @main.g\n  %v1 = getelementptr i8, ptr %v0, i64 16\n  %v2 = load i64, ptr %v1\n",
		"the copy is one load and the field read is the same hop walk")
	wantIR(t, ir, "@.map.main.Node", "the walk resolves the binding's own layout")
}

// TestTopLetListCopiedToALocalKeepsTheElementFace: the same copy over a
// module-level List. An element face is what makes the walked `y` a value
// the body can use, so a copy that kept the carrier and lost the face
// would leave the loop variable unclassified.
//
// The element is a gc record rather than an Int64, and that is what makes
// the pin bite: a walk over a scalar-element list re-derives its kind from
// the element's own ABI at the point of use, so dropping the face on that
// path is invisible — the mutation battery's third survivor, and the
// reason this pin was re-aimed. A gc element has no such fallback: only
// the binding's face says which record the walked handle is, so losing it
// stops the loop.
func TestTopLetListCopiedToALocalKeepsTheElementFace(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{
		gcNode(),
		topLetDecl("ns", &ast.NamedType{Name: "List", Args: []ast.TypeRef{named("Node")}},
			&ast.ListLit{Elems: []ast.Expr{nodeInit("1")}}),
	},
		letBind("m", ident("ns")),
		walk("y", ident("m"), letBind("z", memberOf(ident("y"), "value"))),
	))
	wantIR(t, ir, "  %v0 = load ptr, ptr @main.ns\n", "the copy is one load of the carrier")
	if n := strings.Count(ir, "load ptr, ptr @main.ns"); n != 1 {
		t.Fatalf("the copy reads the global once, the emission reads it %d times:\n%s", n, ir)
	}
	wantIR(t, ir, "@.map.main.Node", "the walked element resolves the record's layout")
}

// TestTopLetGcCrossModuleReadLoadsTheQualifiedGlobal: a dependency's
// handle is a global in the same LLVM module, so the read is the same one
// load — from the dependency's symbol. Both a fn body and the entry take
// it, and both are the same instruction.
func TestTopLetGcCrossModuleReadLoadsTheQualifiedGlobal(t *testing.T) {
	dep := ProgModule{Key: "u1", File: &ast.File{Items: []ast.Item{
		recDecl("Leaf", "gc", fld("n", "Int64")),
		topLetDecl("shared", named("Leaf"), construct("Leaf", init1("n", intLit("77")))),
	}}}
	root := topProg([]ast.Item{
		topFn("read", memberOf(&ast.Member{Recv: ident("u1"), Name: "shared"}, "n")),
	},
		letBind("k", memberOf(&ast.Member{Recv: ident("u1"), Name: "shared"}, "n")),
	)
	ir := topGcIR(t, dep, root)
	wantIR(t, ir, "@u1.shared = internal global ptr null", "the dependency's global")
	wantIR(t, ir, "call void @__we_gc_root_global(ptr @u1.shared)", "the dependency's slot")
	if n := strings.Count(ir, "load ptr, ptr @u1.shared"); n != 2 {
		t.Fatalf("both readers take the global's load, the emission has %d:\n%s", n, ir)
	}
	wantIR(t, ir, "define void @u1.init()", "the dependency owns the init")
}

// TestTopLetStringIsNotRegistered: design D3's storage ruling. A String's
// bytes are outside the gc domain, so the collector must not be told about
// a global holding one — the mark phase would read the first word of a
// malloc'd buffer as a block header.
func TestTopLetStringIsNotRegistered(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{
		topLetDecl("s", named("String"), strLit(`"hi"`)),
	}))
	wantIR(t, ir, "@main.s.p = internal global ptr null", "the pair is still the String's face")
	wantNoIR(t, ir, "__we_gc_root_global", "a String is not a collectable handle")
	wantNoIR(t, ir, "@main.s = internal global ptr null", "and takes no handle global")
}

// TestTopLetGcValueRecordStops: chapter 8 gives every binding of a value
// record its own object, so a module-level one would hold a copy the
// initializer made rather than a handle into a collectable block — a
// different storage story, and not this face.
func TestTopLetGcValueRecordStops(t *testing.T) {
	_, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		recDecl("Point", "value", fld("x", "Int64")),
		topLetDecl("p", named("Point"), construct("Point", init1("x", intLit("1")))),
	})})
	if ni == nil {
		t.Fatal("a value record binding is not the handle face")
	}
	if ni.What != "top-level value bindings in code generation" {
		t.Fatalf("boundary word: %q", ni.What)
	}
}

// TestInitBodyDischargesItsPushes: the init body is a body, so its exit
// owes its own pushes. This is the face T8-2B-0 deferred — at that point
// no top-level binding rooted anything, so a pin would have been vacuous —
// and it is also what makes this commit's negative control observable: a
// leaked init root would keep the handle marked with or without the root
// table, and the table's absence would look like a table that works.
func TestInitBodyDischargesItsPushes(t *testing.T) {
	ir := topGcIR(t, topProg([]ast.Item{gcNode(), topLetDecl("g", named("Node"), nodeInit("42"))}))
	init := initBody(t, ir, "main")
	if n := strings.Count(init, "__we_root_push"); n == 0 {
		t.Fatalf("the init body pushes nothing — the pin would be vacuous:\n%s", init)
	} else {
		popsBefore(t, init, "ret void", n)
	}
}
