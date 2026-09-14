package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T7-2/T7-3: the iterator protocol and a user Iterable (design D7). The
// three inline for forms are one concrete type's specialization of the
// protocol each; what is pinned here is the form every other source
// takes — a record whose impl binds `type Iter = …`. Red before it:
// `for c in r` stopped at the body's boundary word, because emitFor
// dispatched over the three inline faces alone.
//
// The protocol is the check stage's own three clauses, and each has a pin
// below: the source evaluates once, `iterator` runs exactly once, and the
// walk ends on the first tag that is not `Some` rather than on a length
// the loop could compare against.

// iterSrc is one program carrying the whole protocol: a handle with a
// cursor of its own, an Iterable whose association names it, and a loop.
// The handle counts down so that a `next` whose mutation was lost — a
// receiver crossing by value — would never terminate.
const iterSrc = `import std.io

pub type AppError = Failed(String)

record CountIter { i: Int64, hi: Int64 }

impl Iterator<Int64> for CountIter {
    pub fn next(mut self) -> Option<Int64> {
        if self.i >= self.hi { return None }
        let v = self.i
        self.i = self.i + 1
        return Some(v)
    }
}

record Range2 { n: Int64 }

impl Iterable<Int64> for Range2 {
    type Iter = CountIter
    pub fn iterator(self) -> %s {
        return CountIter { i: 0, hi: self.n }
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let r = Range2 { n: 3 }
    for c in r {
        io.println("${c}")
    }
    return Ok(())
}
`

// emitIters emits one program whose `iterator` return is spelled the two
// ways the checker admits: the concrete head, and the association's own
// name.
func emitIters(t *testing.T, ret string) string {
	t.Helper()
	src := strings.Replace(iterSrc, "%s", ret, 1)
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q for the `-> %s` spelling:\n%s", ni.What, ret, src)
	}
	return ir
}

// TestForProtocolBuildsTheHandleOnce: the protocol's first clause. The
// handle is built before the head opens and the walk calls `next` through
// the register it landed in, so the sequence is the one the source
// denotes no matter what the body does to the source expression — the
// same snapshot discipline the List walk takes over its carrier.
func TestForProtocolBuildsTheHandleOnce(t *testing.T) {
	ir := emitIters(t, "CountIter")
	if got := countCall(ir, "ptr", "main.Range2.iterator"); got != 1 {
		t.Fatalf("want exactly one iterator call site, got %d:\n%s", got, ir)
	}
	order(t, ir,
		"call ptr @main.Range2.iterator(ptr",
		"call void @__we_root_push(ptr", // the handle is rooted for the walk
		"br label %forhead0",
		"forhead0:",
		"call { i64, i64, i64 } @main.CountIter.next(ptr",
	)
	if got := strings.Count(ir, "@main.CountIter.next"); got != 2 {
		t.Fatalf("want the define plus one static call site, got %d:\n%s", got, ir)
	}
}

// TestForProtocolEndsOnTheNoneTag: the protocol's third clause. The head
// is the handle's own answer — a tag test against `Some`'s index in the
// variant table the callee's signature carries, which is the index the
// construction inside `next` writes. Nothing compares a length or an
// index, because the walk has neither.
func TestForProtocolEndsOnTheNoneTag(t *testing.T) {
	ir := emitIters(t, "CountIter")
	head := blockAt(ir, "forhead0")
	if head == "" {
		t.Fatalf("missing the loop head:\n%s", ir)
	}
	if !strings.Contains(head, "icmp eq i64") {
		t.Fatalf("the head tests the tag for equality:\n%s", head)
	}
	if strings.Contains(head, "icmp slt i64") {
		t.Fatalf("the protocol walk counts nothing down:\n%s", head)
	}
	// The tag is read out of the slot the call's own extractvalue filled,
	// so the discriminant is the callee's table and not a second
	// convention invented here.
	order(t, head, "extractvalue { i64, i64, i64 }", "store i64", "load i64, ptr", "icmp eq i64", "br i1")
	// Some is the second declared variant of Option in the emission's own
	// table (None first), which is where the index comes from.
	if !strings.Contains(head, "icmp eq i64") || !strings.Contains(head, ", 1") {
		t.Fatalf("the test names Some's index in the slot's table:\n%s", head)
	}
}

// TestForProtocolBindsThePayload: the element reaches the body through
// the same per-position path a match arm's payload takes, so the binding
// carries the payload's declared face — here an Int64, which the
// interpolated hole renders as an integer.
func TestForProtocolBindsThePayload(t *testing.T) {
	ir := emitIters(t, "CountIter")
	body := blockAt(ir, "forbody0")
	if body == "" {
		t.Fatalf("missing the loop body:\n%s", ir)
	}
	order(t, body, "load i64, ptr", "__we_str_of_i64")
	// The iterator's own state advances in the handle, so the body holds
	// no second counter and no store back into the walk.
	if strings.Contains(body, "store i64") {
		t.Fatalf("the pass's element is read, not written:\n%s", body)
	}
}

// TestForProtocolStepsThroughTheHandle: continue lands on the step block,
// which is a plain back edge — the handle carries the walk's state, so
// there is no counter to advance and a continue reaches the next `next`
// rather than the same element twice.
func TestForProtocolStepsThroughTheHandle(t *testing.T) {
	ir := emitIters(t, "CountIter")
	step := blockAt(ir, "forcont0")
	if step == "" {
		t.Fatalf("missing the step block:\n%s", ir)
	}
	if !strings.Contains(step, "br label %forhead0") {
		t.Fatalf("the step is the back edge:\n%s", step)
	}
	if strings.Contains(step, "store") {
		t.Fatalf("the protocol step advances nothing:\n%s", step)
	}
	if got := strings.Count(ir, "alloca i64"); got != 3 {
		t.Fatalf("want the Option's three payload slots alone, got %d:\n%s", got, ir)
	}
}

// TestForProtocolAssociationSpellingResolves: the checker admits the
// association's own name where the emission needs a record, and the impl
// tree carries the binding either way. Both spellings name one head, so
// both produce one define and one call — the association is the
// authority, not the signature's spelling.
func TestForProtocolAssociationSpellingResolves(t *testing.T) {
	concrete := emitIters(t, "CountIter")
	assoc := emitIters(t, "Iter")
	for _, ir := range []string{concrete, assoc} {
		wantIR(t, ir, "define ptr @main.Range2.iterator(ptr %self)", "the impl's own iterator define")
		wantIR(t, ir, "define { i64, i64, i64 } @main.CountIter.next(ptr %self)", "the handle's next define")
		wantIR(t, ir, "call ptr @main.Range2.iterator(ptr", "the walk's iterator call")
	}
	wantNoIR(t, assoc, "%Iter", "the association's name reaches no IR symbol")
}

// TestForProtocolStringPayload: the payload's face is the handle's
// declaration, so an Iterator<String> binds its element as a String —
// the two-word pair a match arm's String payload binds — rather than as
// the handle word it rode in.
func TestForProtocolStringPayload(t *testing.T) {
	const src = `import std.io

pub type AppError = Failed(String)

record Words { i: Int64 }

impl Iterator<String> for Words {
    pub fn next(mut self) -> Option<String> {
        return None
    }
}

record Source { n: Int64 }

impl Iterable<String> for Source {
    type Iter = Words
    pub fn iterator(self) -> Words {
        return Words { i: 0 }
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let s = Source { n: 0 }
    for w in s {
        io.println(w)
    }
    return Ok(())
}
`
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// The element is the pair the payload words hold: the data word is
	// loaded as a pointer and the length word beside it, exactly as a
	// match arm's String payload binds — not the word passed through.
	body := blockAt(ir, "forbody0")
	if body == "" {
		t.Fatalf("missing the loop body:\n%s", ir)
	}
	if !strings.Contains(body, "load ptr, ptr") || !strings.Contains(body, "load i64, ptr") {
		t.Fatalf("a String element binds as its two words:\n%s", body)
	}
}

// TestForProtocolStopsOnACallSource is the boundary pin beside the
// positive ones, and it names a gap rather than a decision: the source's
// record is read from the forms the layouts name — a binding, a field
// chain, a construction — so a *call's* result is no head this build can
// key on and the loop stops where it stopped before. The checker admits
// the source either way (E0901 is satisfied by the callee's return), so
// the boundary is the emission's alone, and it is recorded as such.
func TestForProtocolStopsOnACallSource(t *testing.T) {
	const src = `import std.io

pub type AppError = Failed(String)

record CountIter { i: Int64 }

impl Iterator<Int64> for CountIter {
    pub fn next(mut self) -> Option<Int64> {
        return None
    }
}

record Range2 { n: Int64 }

impl Iterable<Int64> for Range2 {
    type Iter = CountIter
    pub fn iterator(self) -> CountIter {
        return CountIter { i: 0 }
    }
}

fn make() -> Range2 {
    return Range2 { n: 0 }
}

pub fn main() effect io -> Result<(), AppError> {
    for c in make() {
        io.println("${c}")
    }
    return Ok(())
}
`
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni == nil {
		t.Fatalf("a call's record result is no head the emission names")
	}
	if !strings.Contains(ni.What, "statement set") {
		t.Fatalf("want the body boundary word, got %q", ni.What)
	}
}

// TestAssocRetResolvesTheImplsOwnBinding: the resolution is the impl's
// association binding, not the name `Iter` — a head is free to call its
// association anything an interface declares, and a method returning a
// type the impl did not bind is left exactly as written.
func TestAssocRetResolvesTheImplsOwnBinding(t *testing.T) {
	nt := func(name string) *ast.NamedType { return &ast.NamedType{Name: name} }
	md := &ast.FnDecl{Name: "iterator", Ret: nt("Iter")}
	got := assocRet(md, map[string]*ast.NamedType{"Iter": nt("CountIter")})
	if got == md {
		t.Fatal("a return spelling an association resolves to the binding")
	}
	if got.Ret.(*ast.NamedType).Name != "CountIter" {
		t.Fatalf("the return = %v, want CountIter", got.Ret)
	}
	if md.Ret.(*ast.NamedType).Name != "Iter" {
		t.Fatalf("the declaration tree was rewritten in place: %v", md.Ret)
	}
	for _, c := range []struct {
		what string
		md   *ast.FnDecl
	}{
		{"a return the impl did not bind", &ast.FnDecl{Name: "m", Ret: nt("Other")}},
		{"a qualified return", &ast.FnDecl{Name: "m", Ret: &ast.NamedType{Qual: "other", Name: "Iter"}}},
		{"an applied return", &ast.FnDecl{Name: "m", Ret: &ast.NamedType{Name: "Iter", Args: []ast.TypeRef{nt("Int64")}}}},
		{"a non-named return", &ast.FnDecl{Name: "m", Ret: &ast.TupleType{}}},
	} {
		if got := assocRet(c.md, map[string]*ast.NamedType{"Iter": nt("CountIter")}); got != c.md {
			t.Fatalf("%s: rewritten to %v", c.what, got.Ret)
		}
	}
}
