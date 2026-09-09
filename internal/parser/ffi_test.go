package parser

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M12 (ffi) chapter 19 parser tests (design D1): the foreign-block
// production — the ABI literal, the item closure, the bare effect
// segment legal only inside the block — plus the four E17xx codes and
// the E0404/E0105 joins. Negative sources mirror the T1 golden table
// verbatim so codes, message fragments, and positions stay identical
// to the CLI-face contract. Written test-first: red today on the
// undefined symbols alone (ast.ForeignBlock, the Foreign/Opaque
// markers, the E1701–E1704 codes).

// The production proper: ABI "c", fn entries with the required segment
// (tags and bare), the three record prefixes as opaque declarations,
// pub entries, positions.
func TestForeignBlockProduction(t *testing.T) {
	src := "foreign \"c\" {\n    fn abs(x: Int64) effect -> Int64\n    fn open(path: String) effect io -> Int64\n    record Socket { }\n    byval record Errno { }\n    byres record File { }\n}\n"
	f := wantClean(t, src)
	fb, ok := f.Items[0].(*ast.ForeignBlock)
	if !ok {
		t.Fatalf("items[0]: want *ast.ForeignBlock, got %T", f.Items[0])
	}
	if fb.ABI != "c" {
		t.Fatalf("ABI: %q", fb.ABI)
	}
	if fb.Line != 1 || fb.Col != 1 {
		t.Fatalf("foreign keyword position: %d:%d", fb.Line, fb.Col)
	}
	if len(fb.Items) != 5 {
		t.Fatalf("items: %d entries, want 5", len(fb.Items))
	}
	fd, ok := fb.Items[0].(*ast.FnDecl)
	if !ok || fd.Name != "abs" {
		t.Fatalf("items[0]: want fn abs, got %+v", fb.Items[0])
	}
	if !fd.Foreign {
		t.Fatalf("fn entry must carry the Foreign marker")
	}
	if fd.Body.Items != nil {
		t.Fatalf("a foreign fn declares no body, got %+v", fd.Body.Items)
	}
	if len(fd.EffectTags) != 0 || fd.EffectLine == 0 {
		t.Fatalf("a bare segment is present (zero tags) but parsed: tags=%v line=%d", fd.EffectTags, fd.EffectLine)
	}
	od, ok := fb.Items[2].(*ast.RecordDecl)
	if !ok || od.Name != "Socket" {
		t.Fatalf("items[2]: want record Socket, got %+v", fb.Items[2])
	}
	if !od.Opaque {
		t.Fatalf("a zero-field foreign record must carry the Opaque marker")
	}
	if od.Cat != "gc" {
		t.Fatalf("no prefix = gc category, got %q", od.Cat)
	}
	// Two foreign blocks in one module share the one name space.
	f = wantClean(t, "foreign \"c\" {\n    fn abs(x: Int64) effect -> Int64\n}\n\nforeign \"c\" {\n    fn neg(x: Int64) effect -> Int64\n}\n")
	if len(f.Items) != 2 {
		t.Fatalf("two blocks: %d items", len(f.Items))
	}
	// pub entries parse (their reach is chapter 15's, like any pub).
	f = wantClean(t, "foreign \"c\" {\n    pub fn abs(x: Int64) effect -> Int64\n    pub record Socket { }\n}\n")
	if f.Items[0].(*ast.ForeignBlock).Items[0].(*ast.FnDecl).Pub != true {
		t.Fatalf("pub fn entry must carry Pub")
	}
}

// E1701: any string other than "c" in the ABI position, anchored at the
// literal.
func TestForeignBlockE1701(t *testing.T) {
	wantDiag(t, "foreign \"go\" {\n    fn abs(x: Int64) effect -> Int64\n}\n",
		"E1701", `got "go"`, 1, 9)
	// A non-string token there is the same code's family face.
	wantDiag(t, "foreign c {\n}\n",
		"E1701", `not "c"`, 1, 9)
}

// E1702: a foreign block below the top level, anchored at the keyword —
// today's E0105 generic face turns into the dedicated code.
func TestForeignBlockE1702(t *testing.T) {
	wantDiag(t, "pub fn main() -> () {\n    foreign \"c\" {\n    }\n    return\n}\n",
		"E1702", `the boundary is declared per module, not per expression`, 2, 5)
}

// E1703: a fn entry without a segment, anchored at the name.
func TestForeignBlockE1703(t *testing.T) {
	wantDiag(t, "foreign \"c\" {\n    fn dial(addr: String) -> Int64\n}\n",
		"E1703", `"dial" has no body; the declaration is the only evidence`, 2, 8)
}

// E1704: a generic clause on a fn entry, anchored at the name.
func TestForeignBlockE1704(t *testing.T) {
	wantDiag(t, "foreign \"c\" {\n    fn id<T>(x: T) effect -> T\n}\n",
		"E1704", `"id"; the boundary is monomorphic`, 2, 8)
}

// The item closure is fn and opaque record declarations, nothing else;
// each reject is E0105 naming the production (design D1).
func TestForeignBlockItemClosure(t *testing.T) {
	// let does not belong.
	wantDiag(t, "foreign \"c\" {\n    let x = 1\n}\n",
		"E0105", `holds foreign function declarations and opaque type declarations, nothing else`, 2, 5)
	// A fn with a body does not belong.
	wantDiag(t, "foreign \"c\" {\n    fn abs(x: Int64) effect -> Int64 {\n        return x\n    }\n}\n",
		"E0105", `holds foreign function declarations and opaque type declarations, nothing else`, 2, 5)
	// A record with fields is not an opaque declaration.
	wantDiag(t, "foreign \"c\" {\n    record Socket { fd: Int64 }\n}\n",
		"E0105", `holds foreign function declarations and opaque type declarations, nothing else`, 2, 5)
	// An effect declaration does not belong (the segment spelling inside
	// the fn entries is not an effect declaration).
	wantDiag(t, "foreign \"c\" {\n    effect db\n}\n",
		"E0105", `holds foreign function declarations and opaque type declarations, nothing else`, 2, 5)
}

// The bare segment is legal only inside a foreign block: a top-level
// fn keeps M7's rejection verbatim.
func TestForeignBlockBareSegmentScoping(t *testing.T) {
	wantDiag(t, "fn topNaked() effect {\n    return\n}\n",
		"E0105", `"{" where an effect segment names at least one effect`, 1, 22)
}

// Foreign entry names join the module's one name space: a duplicate
// across blocks (and against a top-level declaration) is E0404.
func TestForeignBlockE0404(t *testing.T) {
	wantDiag(t, "foreign \"c\" {\n    fn open(path: String) effect io -> Int64\n}\n\nforeign \"c\" {\n    fn open(path: String) effect io -> Int64\n}\n",
		"E0404", `"open" is already declared at line 2`, 6, 8)
	wantDiag(t, "fn abs(x: Int64) -> Int64 {\n    return x\n}\n\nforeign \"c\" {\n    fn abs(x: Int64) effect -> Int64\n}\n",
		"E0404", `"abs" is already declared at line 1`, 6, 8)
}

// The dispatch table row: `pub foreign` reports the pub-tail face
// (a foreign block carries no pub of its own).
func TestForeignBlockPubFace(t *testing.T) {
	wantDiag(t, "pub foreign \"c\" {\n}\n",
		"E0105", `"foreign" after pub`, 1, 5)
}
