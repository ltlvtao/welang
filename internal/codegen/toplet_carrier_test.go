package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T8-2 module-level carrier bindings (design D7, chapter 15 R5). A binding
// whose value is more than one scalar word is a carrier, and the two carrier
// families the design names are not the same problem:
//
//   - a String is a (ptr, len) pair whose bytes live in a malloc'd buffer or
//     in one of the emitter's private constants. Design D3's storage ruling
//     puts both provenances OUTSIDE the gc domain — a raw byte buffer has no
//     header and no descriptor, and the runtime's own note says a non-gc
//     pointer on the root face would be read as a block header. So a String
//     global is never registered as a root: it needs its pair stored and
//     loaded, nothing more, and it survives collection because it is not
//     collectable in the first place;
//   - a gc record or a list is a handle into a collectable block, so its
//     global IS a gc root and must be visible to every collection. That is
//     `__we_gc_root_global`, and it is the half of this task that changes
//     the runtime.
//
// The String half is pinned here. Its face:
//
//   - one global per word, `@<key>.<name>.p` and `@<key>.<name>.len`, both
//     zero-initialized and both stored by the module init through the same
//     one-pair read face every other String value uses;
//   - the read is the storage face: two loads, no register reuse, so a fn
//     body, a later initializer of the same module and a reading module all
//     see the pair the init wrote;
//   - no root registration, for the storage reason above.
//
// The emitted shape is pinned below; the answers are the conformance
// goldens'.

// strTopDecl is one module-level String binding with a literal initializer:
// `[pub] let name: String = init`.
func strTopDecl(name string, init ast.Expr) *ast.TopLet {
	return topLetDecl(name, named("String"), init)
}

// TestTopLetStringOwnsAPairOfGlobals: the binding's storage is one global
// per word of the pair, named off the binding's qualified symbol.
func TestTopLetStringOwnsAPairOfGlobals(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		strTopDecl("greeting", strLit(`"hi"`)),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@main.greeting.p = internal global ptr null", "the pair's data word")
	wantIR(t, ir, "@main.greeting.len = internal global i64 0", "the pair's length word")
	wantIR(t, ir, "define void @main.init()", "the module still owns its initializer")
}

// TestTopLetStringInitStoresBothWords: the initializer's pair reaches both
// globals — the literal's interned bytes and its length.
func TestTopLetStringInitStoresBothWords(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		strTopDecl("greeting", strLit(`"hi"`)),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	body := initBody(t, ir, "main")
	wantIR(t, ir, "@.s0 = private unnamed_addr constant [2 x i8] c\"hi\"", "the interned bytes")
	wantIR(t, body, "store ptr @.s0, ptr @main.greeting.p", "the data word's store")
	wantIR(t, body, "store i64 2, ptr @main.greeting.len", "the length word's store")
}

// TestTopLetStringReadLoadsBothWords: a reader takes the pair from the
// globals — two loads, data word first — so the read face and the storage
// face are one, and nothing can drift between a fn body, a later
// initializer and a reader in another module. The registers are pinned
// with their roles: the data word is the pointer operand and the length is
// the i64 one, and a pair read the other way round is a wrong answer no
// substring of the loads alone would catch.
func TestTopLetStringReadLoadsBothWords(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		strTopDecl("greeting", strLit(`"hi"`)),
	}, ioCall("io", "println", ident("greeting")))})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	tail := entryTail(ir)
	wantIR(t, tail, "\n  %v0 = load ptr, ptr @main.greeting.p\n  %v1 = load i64, ptr @main.greeting.len\n",
		"the pair's two loads, data word first")
	wantIR(t, tail, "call void %v2(ptr %v0, i64 %v1)", "the data word in the pointer position, the length in the i64 one")
}

// TestTopLetStringCrossModuleRead: a qualified name reaches another
// module's pair through that module's globals, and the same two loads are
// the whole read face.
func TestTopLetStringCrossModuleRead(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		strTopDecl("base", strLit(`"ok"`)),
	}}}
	root := ProgModule{Key: "main", File: listModule(nil,
		ioCall("io", "println", memberOf(ident("util"), "base")),
	)}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{util, root})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@util.base.p = internal global ptr null", "the dependency's pair")
	wantIR(t, ir, "load ptr, ptr @util.base.p", "the qualified data load")
	wantIR(t, ir, "load i64, ptr @util.base.len", "the qualified length load")
}

// TestTopLetStringReadsStayOutOfTheScalarDomain: a String binding is not a
// scalar word, so it must not answer a scalar read — a name bound from it
// carries the pair onward, not an i64 reinterpretation of the pointer.
func TestTopLetStringReadsStayOutOfTheScalarDomain(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		strTopDecl("greeting", strLit(`"hi"`)),
	}, letBind("copy", ident("greeting")))})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantNoIR(t, ir, "@main.greeting = internal global i64", "no scalar global for a pair")
	wantNoIR(t, ir, "load i64, ptr @main.greeting\n", "no scalar load of the binding")
}

// TestTopLetStringCrossModuleConcat: the qualified read inside a String
// expression — the concatenation's operand, not a bare statement — is the
// same pair face, so the concat consumes the globals directly.
func TestTopLetStringCrossModuleConcat(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		strTopDecl("base", strLit(`"ok"`)),
	}}}
	root := ProgModule{Key: "main", File: listModule(nil,
		ioCall("io", "println", &ast.Binary{
			Op: "+",
			L:  memberOf(ident("util"), "base"),
			R:  strLit(`"!"`),
		}),
	)}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{util, root})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	tail := entryTail(ir)
	wantIR(t, tail, "load ptr, ptr @util.base.p", "the qualified data load")
	wantIR(t, tail, "load i64, ptr @util.base.len", "the qualified length load")
	wantIR(t, tail, "@__we_str_concat(ptr %v0, i64 %v1, ptr @.s0, i64 1)", "the globals feed the concat directly")
}

// TestTopLetStringIsNoGcRoot: design D3's storage ruling is why this half
// needs no runtime change — a String's bytes are a private constant or a
// malloc'd buffer, neither of them a gc block, and the collector would read
// the first word of one as a block header. A String binding therefore
// registers nothing, and the runtime's root table is T8-2's other half.
func TestTopLetStringIsNoGcRoot(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		strTopDecl("greeting", strLit(`"hi"`)),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantNoIR(t, ir, "__we_gc_root_global", "a String's bytes are outside the gc domain")
}

// TestTopLetStringFromACallIsAPairToo: a String a function answered is the
// same pair face as a literal's — the globals do not care which provenance
// the bytes have, and D3 makes both provenances immortal.
func TestTopLetStringFromACallIsAPairToo(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		pubFn("mk", nil, named("String"), retValue(strLit(`"made"`))),
		strTopDecl("made", &ast.Call{Fn: ident("mk")}),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	body := initBody(t, ir, "main")
	wantIR(t, body, "store ptr ", "the data word's store")
	if !strings.Contains(body, ", ptr @main.made.p") {
		t.Fatalf("the call's pair does not reach the globals:\n%s", body)
	}
	wantIR(t, body, ", ptr @main.made.len", "the length word's store")
}

// TestTopLetStringDiscardBindsNoPair: `let _ = "hi"` still evaluates — the
// discard is chapter 6's — but no symbol names the value, so no global pair
// appears for it.
func TestTopLetStringDiscardBindsNoPair(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		strTopDecl("_", strLit(`"hi"`)),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define void @main.init()", "the module still owes the evaluation")
	wantNoIR(t, ir, "@main._", "a discard binds nothing")
}
