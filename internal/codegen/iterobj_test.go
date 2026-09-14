package codegen

import (
	"strings"
	"testing"
)

// T7-4: the builtin iterator object (design D7 decision 3). A builtin
// source's iterator is the one value whose boxed type is an interface face
// rather than a concrete head — `<list>.iterator()` types as
// `Iterator<Int64>` itself — so the head key, the table, the thunk and the
// `next` body are all synthesized here, and the object they front is what
// makes the box dispatchable at all: a thunk reads its receiver out of the
// payload's first word, so a payload that were the list handle itself
// would hand `next` the list with the position nowhere to live.
//
// Red before it: `Dyn<Iterator<Int64> >(xs.iterator())` stopped at the
// body's boundary word, because boxFace classified the interface face
// naming no box encoding. The three inline for forms and the six acute
// combinators are the zero-cost path this must leave alone (decision 1):
// neither reaches emitBox, so neither gains an object.

// iterBoxSrc is one program carrying the whole face: a List binding whose
// iterator is boxed, and the box dispatched through its table.
const iterBoxSrc = `pub type AppError = Failed(String)

pub fn main() -> Result<(), AppError> {
    let xs = [10, 12]
    let d = Dyn<Iterator<Int64> >(xs.iterator())
    let a = d.next()
    return Ok(())
}
`

// emitIterBoxIR emits the fixture and answers its module.
func emitIterBoxIR(t *testing.T, src string) string {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q:\n%s", ni.What, src)
	}
	return ir
}

// TestListIteratorObjectCarriesTheTracedHandle is the task's IR pin: the
// object's payload is the pair design D7 decision 3 names, and the
// descriptor it gets is derived from that face alone — the handle at the
// header's own shadow is a gc reference, the index beside it is not. The
// bitmap is the whole assertion: bit 0 is offset 16, and there is no bit
// for offset 24.
//
// The box's own descriptor is the second pin, and it is a different
// global: a box's word at 16 is its table pointer, which no descriptor
// marks, so its one traced word is the payload word at 24. Two faces that
// differ in exactly one bit cannot share a descriptor, and they do not.
func TestListIteratorObjectCarriesTheTracedHandle(t *testing.T) {
	ir := emitIterBoxIR(t, iterBoxSrc)
	// The object's descriptor: slot 0 — offset 16, the handle — traced.
	wantIR(t, ir, "@.dynmap0 = private unnamed_addr constant [1 x i64] [i64 1]",
		"the iterator object's descriptor marks the handle at 16")
	// The box's: its one traced word is the payload at 24.
	wantIR(t, ir, "@.dynmap1 = private unnamed_addr constant [1 x i64] [i64 2]",
		"the box's descriptor marks the object pointer at 24")
	if n := strings.Count(ir, "= private unnamed_addr constant [1 x i64]"); n != 2 {
		t.Fatalf("want exactly two descriptors, got %d:\n%s", n, ir)
	}
	// The two blocks in build order, each one rooted with its map word and
	// only then filled: the object's 16 takes the handle and its 24 the
	// zero index; the box's 16 takes the table and its 24 the object. The
	// ordering is the safety argument — __we_alloc zeroes the block and the
	// collector skips a null child, so the traced store is reached only
	// after the block is both rooted and described.
	order(t, ir,
		"call ptr @__we_alloc(i64 32)",
		"store ptr @.dynmap0, ptr %",
		"call void @__we_root_push(ptr %",
		", i64 16\n  store ptr %", // the handle into the object
		", i64 24\n  store i64 0, ptr %",
		"call ptr @__we_alloc(i64 32)",
		"store ptr @.dynmap1, ptr %",
		"call void @__we_root_push(ptr %",
		", i64 16\n  store ptr @.vt.Iterator$Int64.ListIter$Int64, ptr %",
		", i64 24\n  store ptr %", // the object into the box
	)
}

// TestListIteratorHeadIsSynthetic pins the head key and everything filed
// under it. The check stage records the construction's boxed type as the
// interface face, which names no concrete head, so `ListIter$Int64` is
// minted here — and it is not the interface's own `Iterator$Int64` key,
// which is what a second implementing head of the same element domain
// would collide with.
//
// The `next` define is a define and not a thunk: it is the walk, and the
// thunk fronts it the way it fronts any head's method.
func TestListIteratorHeadIsSynthetic(t *testing.T) {
	ir := emitIterBoxIR(t, iterBoxSrc)
	wantIR(t, ir,
		"@.vt.Iterator$Int64.ListIter$Int64 = private unnamed_addr constant [1 x ptr] "+
			"[ptr @.vt.Iterator$Int64.ListIter$Int64.next]",
		"the table: the face's key, the synthetic head's, and the one slot")
	wantIR(t, ir, "define internal { i64, i64, i64 } @ListIter$Int64.next(ptr %self)",
		"the walk itself, under the synthetic head's own symbol")
	wantIR(t, ir, "define internal { i64, i64, i64 } @.vt.Iterator$Int64.ListIter$Int64.next(ptr %box)",
		"the thunk, at the slot's own signature")
	// The thunk's whole convention: the receiver is the box's payload word,
	// which is the object — never the list, and never the box itself.
	wantIR(t, ir,
		"  %addr = getelementptr i8, ptr %box, i64 24\n"+
			"  %recv = load ptr, ptr %addr\n"+
			"  %r = call { i64, i64, i64 } @ListIter$Int64.next(ptr %recv)",
		"the thunk unwraps the box and hands `next` the object")
	// One define of the walk per head, however many boxes name it.
	if n := strings.Count(ir, "@ListIter$Int64.next(ptr"); n != 2 {
		t.Fatalf("want the define plus the thunk's one call, got %d:\n%s", n, ir)
	}
}

// TestListIteratorNextReadsThroughTheObject pins the walk's body: the
// handle comes out of the object and the index goes back into it, so the
// position survives between calls — the whole reason the pair travels
// together. The carrier is read live rather than snapshotted: the object
// holds the list itself (design D7 decision 3), and reading the length and
// the element through the handle each pass is what makes it that list's
// own iterator rather than a copy of one.
//
// The tag written is the one Option gives Some in the emitter's own
// variant table — None 0, Some 1 — and it is written as a phi rather than
// through the constant a zero-payload sum takes, because the payload word
// here is a register.
func TestListIteratorNextReadsThroughTheObject(t *testing.T) {
	ir := emitIterBoxIR(t, iterBoxSrc)
	order(t, ir,
		"  %hp = getelementptr i8, ptr %self, i64 16",
		"  %list = load ptr, ptr %hp",
		"  %ip = getelementptr i8, ptr %self, i64 24",
		"  %i = load i64, ptr %ip",
		"  %n = call i64 @__we_list_len(ptr %list)",
		"  %more = icmp slt i64 %i, %n",
		"  %w = call i64 @__we_list_get(ptr %list, i64 %i)",
		"  store i64 %i1, ptr %ip",
		"  %tag = phi i64 [ 1, %step ], [ 0, %entry ]",
		"  %pay = phi i64 [ %w, %step ], [ 0, %entry ]",
	)
}

// TestListIteratorObjectIsNotBuiltOnTheZeroCostPaths pins design D7
// decision 1's other half: the object exists for the form that has to hold
// one. A `for` over the same binding and an acute combinator over it are
// both emitted in place — no `@ListIter$`, no `@.dynmap`, no `next`
// define — so the specializations that were already there stay what they
// were. A count over the same List is the sharpest of the three: it walks
// the very carrier the box holds, and it never mints an object for it.
func TestListIteratorObjectIsNotBuiltOnTheZeroCostPaths(t *testing.T) {
	const src = `pub type AppError = Failed(String)

pub fn main() -> Result<(), AppError> {
    let xs = [10, 12]
    var acc: Int64 = 0
    for x in xs { acc = acc + x }
    let n = xs.iterator().count()
    return Ok(())
}
`
	ir := emitIterBoxIR(t, src)
	wantNoIR(t, ir, "ListIter", "an iterator object on a for walk or an acute combinator")
	wantNoIR(t, ir, "@.dynmap", "a descriptor for a block this program never builds")
	// The for walk's own two shapes are what replaced it, unchanged.
	wantIR(t, ir, "call ptr @__we_list_snap(ptr", "the for walk's snapshot")
	wantIR(t, ir, "call i64 @__we_list_len(ptr", "the acute walk's length read")
}

// TestListIteratorElementDomainPicksTheHead pins the head's second half.
// The object's layout does not depend on the element — its two words are a
// handle and an index whatever the carrier holds — but `next`'s return
// type does, so two element domains are two heads. A shared head would
// answer one signature for both, which is the one thing a vtable cannot
// do.
func TestListIteratorElementDomainPicksTheHead(t *testing.T) {
	for _, tc := range []struct{ elem, head string }{
		{"Int64", "ListIter$Int64"},
		{"Bool", "ListIter$Bool"},
		{"Float64", "ListIter$Float64"},
	} {
		src := strings.ReplaceAll(t7IterBoxFor, "%s", tc.elem)
		ir := emitIterBoxIR(t, src)
		wantIR(t, ir, "define internal { i64, i64, i64 } @"+tc.head+".next(ptr %self)",
			"the "+tc.elem+" element's own head")
		wantIR(t, ir, "@.vt.Iterator$"+tc.elem+"."+tc.head+" = private unnamed_addr constant",
			"the table under the "+tc.elem+" face")
	}
}

// t7IterBoxFor is the head-key fixture with the element domain left open.
// The carrier is a binding rather than a call: the element face is read off
// the receiver the box is built from, and a binding carries an annotation
// where a call's result would have to be inferred back out of a signature.
const t7IterBoxFor = `pub type AppError = Failed(String)

pub fn main() -> Result<(), AppError> {
    let xs: List<%s> = []
    let d = Dyn<Iterator<%s> >(xs.iterator())
    return Ok(())
}
`
