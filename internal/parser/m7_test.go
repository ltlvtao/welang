package parser

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M7 (effects) parser tests: the chapter 16 surfaces design D1/D2 land —
// the top-level effect declaration with its name discipline (E0012,
// E0404, E1403 and their ordering), the effect segment on fn/interface/
// impl method declarations, the closure-position segment refused as
// E0105 (adjudication Q4), and the two already-correct E0105 spelling
// faces locked. Structural cases pin the tree shapes T3 must produce;
// diagnostic cases mirror the T1 golden table's parse-level contract.

func m7File(t *testing.T, src string) *ast.File {
	t.Helper()
	f, d, ni := Parse("test.we", []byte(src))
	if d != nil {
		t.Fatalf("parse diagnostic: %s", d.Human())
	}
	if ni != nil {
		t.Fatalf("parse boundary: %s", ni.What)
	}
	return f
}

// --- D1: the top-level effect declaration --------------------------------------

func TestEffectDeclItem(t *testing.T) {
	// The pub form: name, pub bit, name position for the discipline anchor.
	f := m7File(t, "pub effect db\n")
	ed, ok := f.Items[0].(*ast.EffectDecl)
	if !ok {
		t.Fatalf("EffectDecl wanted, got %T", f.Items[0])
	}
	if ed.Name != "db" || !ed.Pub {
		t.Fatalf("pub effect db wanted, got %+v", ed)
	}
	if ed.NameLine != 1 || ed.NameCol != 12 {
		t.Fatalf("name position 1:12 wanted, got %d:%d", ed.NameLine, ed.NameCol)
	}
	// The bare form carries no pub.
	f = m7File(t, "effect db\n")
	ed, ok = f.Items[0].(*ast.EffectDecl)
	if !ok || ed.Name != "db" || ed.Pub {
		t.Fatalf("effect db wanted, got %+v (%T)", f.Items[0], f.Items[0])
	}
	if ed.NameLine != 1 || ed.NameCol != 8 {
		t.Fatalf("name position 1:8 wanted, got %d:%d", ed.NameLine, ed.NameCol)
	}
}

// --- D2: effect name discipline and its ordering --------------------------------

func TestEffectDeclNames(t *testing.T) {
	// E0012: effect names are camelCase, anchored at the name.
	wantDiag(t, "effect Db\n", "E0012",
		`effect names must be camelCase — "Db" is not camelCase`, 1, 8)
	// E1403: the built-in tags are language-level names, no module item
	// may take them; anchored at the name.
	wantDiag(t, "effect io\n", "E1403",
		`"io" is one of the built-in tags (io, net, time)`, 1, 8)
	wantDiag(t, "effect time\n", "E1403",
		`"time" is one of the built-in tags`, 1, 8)
	// E0404: an effect joins the module's single name space; the earlier
	// declaration wins, the anchor is the effect's name.
	wantDiag(t, "fn db() {\n    return\n}\n\neffect db\n", "E0404",
		`"db" is already declared at line 1`, 5, 8)
	// Ordering pin: a collision outranks the built-in conflict (golden
	// check-e0404-effect-order).
	wantDiag(t, "fn io() {\n    return\n}\n\neffect io\n", "E0404",
		`"io" is already declared at line 1`, 5, 8)
}

// --- D1: the effect segment on declarations --------------------------------------

func TestDeclEffectSegment(t *testing.T) {
	// A fn declaration's segment sits between the parameters and the body.
	f := m7File(t, "fn save(s: String) effect db {\n    return\n}\n")
	fd, ok := f.Items[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("FnDecl wanted, got %T", f.Items[0])
	}
	if len(fd.EffectTags) != 1 || fd.EffectTags[0] != "db" {
		t.Fatalf("segment [db] wanted, got %v", fd.EffectTags)
	}
	// Multiple tags, a return type, and a where clause all coexist with
	// the segment (source order: params, segment, ->, where).
	f = m7File(t, "fn get<T>(x: T) effect db net -> Int64 where T: Eq {\n    return 1\n}\n")
	fd = f.Items[0].(*ast.FnDecl)
	if len(fd.EffectTags) != 2 || fd.EffectTags[0] != "db" || fd.EffectTags[1] != "net" {
		t.Fatalf("segment [db net] wanted, got %v", fd.EffectTags)
	}
	if fd.Ret == nil {
		t.Fatal("return type after the segment wanted")
	}
	if len(fd.Where) != 1 {
		t.Fatal("where clause after the segment wanted")
	}
	// No segment means no tags.
	f = m7File(t, "fn f() {\n    return\n}\n")
	fd = f.Items[0].(*ast.FnDecl)
	if len(fd.EffectTags) != 0 {
		t.Fatalf("no segment wanted, got %v", fd.EffectTags)
	}
	// A qualified tag carries its module-qualified spelling verbatim,
	// mixed with bare tags freely.
	f = m7File(t, "fn work() effect b.db net {\n    return\n}\n")
	fd = f.Items[0].(*ast.FnDecl)
	if len(fd.EffectTags) != 2 || fd.EffectTags[0] != "b.db" || fd.EffectTags[1] != "net" {
		t.Fatalf("segment [b.db net] wanted, got %v", fd.EffectTags)
	}
	// A dot that names no effect is E0105 at the token after it.
	wantDiag(t, "fn work() effect b. {\n    return\n}\n", "E0105",
		`"{" where a qualified effect tag names its effect`, 1, 21)
	// An interface method signature carries its own segment.
	f = m7File(t, "interface Client {\n    fn send(self, s: String) effect net\n}\n")
	inf := f.Items[0].(*ast.InterfaceDecl)
	if len(inf.Methods) != 1 || len(inf.Methods[0].EffectTags) != 1 || inf.Methods[0].EffectTags[0] != "net" {
		t.Fatalf("interface method segment [net] wanted, got %+v", inf.Methods)
	}
	// A default body may follow the segment.
	f = m7File(t, "interface Store {\n    fn put(self, s: String) effect db {\n        return\n    }\n}\n")
	inf = f.Items[0].(*ast.InterfaceDecl)
	if len(inf.Methods[0].EffectTags) != 1 || inf.Methods[0].Body == nil {
		t.Fatalf("default method with segment wanted, got %+v", inf.Methods[0])
	}
	// An impl method declaration carries its own segment.
	f = m7File(t, "record Rec { fd: Int64 }\n\nimpl Rec {\n    fn put(self, s: String) effect db {\n        return\n    }\n}\n")
	impl := f.Items[1].(*ast.ImplDecl)
	if len(impl.Methods) != 1 || len(impl.Methods[0].EffectTags) != 1 || impl.Methods[0].EffectTags[0] != "db" {
		t.Fatalf("impl method segment [db] wanted, got %+v", impl.Methods)
	}
}

// --- Q4/D1: the closure-position segment is E0105 ---------------------------------

func TestClosureSegmentE0105(t *testing.T) {
	// The keyword spelling in a full closure's body position.
	wantDiag(t, "fn work() {\n    let f = fn(s: String) effect io {\n        return\n    }\n    return\n}\n",
		"E0105", `"effect" where a closure's body block opens`, 2, 27)
	// The bare-tag spelling in the same position (already correct today
	// — a locked face).
	wantDiag(t, "fn work() {\n    let f = fn(s: String) io {\n        return\n    }\n    return\n}\n",
		"E0105", `"io" where a closure's body block opens`, 2, 27)
	// The locked declaration-position bare tag (already correct today).
	wantDiag(t, "fn f() io {\n    return\n}\n",
		"E0105", `"io" where a fn body block opens`, 1, 8)
	// The locked type-position keyword (already correct today).
	wantDiag(t, "fn id(x: Int64) -> Int64 {\n    return x\n}\nlet f: fn(Int64) effect io -> Int64 = id\n",
		"E0105", `"effect" in a function type reference`, 4, 18)
}

// --- D3: the fn type's bare segment keeps parsing (regression) --------------------

func TestFnTypeEffectTags(t *testing.T) {
	// The type-position segment is bare tags only; the parser already
	// carries them (kept tags, multiple allowed).
	f := m7File(t, "fn run(g: fn(String) db net -> ()) {\n    return\n}\n")
	fd := f.Items[0].(*ast.FnDecl)
	ft, ok := fd.Params[0].Type.(*ast.FnType)
	if !ok {
		t.Fatalf("FnType param wanted, got %T", fd.Params[0].Type)
	}
	if len(ft.EffectTags) != 2 || ft.EffectTags[0] != "db" || ft.EffectTags[1] != "net" {
		t.Fatalf("type segment [db net] wanted, got %v", ft.EffectTags)
	}
	// The type slot takes the qualified spelling too.
	f = m7File(t, "fn run(g: fn(String) b.db -> ()) {\n    return\n}\n")
	fd = f.Items[0].(*ast.FnDecl)
	ft, ok = fd.Params[0].Type.(*ast.FnType)
	if !ok {
		t.Fatalf("FnType param wanted, got %T", fd.Params[0].Type)
	}
	if len(ft.EffectTags) != 1 || ft.EffectTags[0] != "b.db" {
		t.Fatalf("type segment [b.db] wanted, got %v", ft.EffectTags)
	}
}
