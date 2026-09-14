package codegen

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// T8-2 the four lazy combinators (design D8 decision 2, chapter 11). The
// call performs no element work — that is what lazy means here — so what a
// call emits is two objects and a define, nothing else:
//
//   - the ADAPTER the combinator names, holding the source box at 16 and
//     the call's one parameter at 24 — a fn value's carrier for map and
//     filter, a plain count for take and skip;
//   - the BOX the chain's result face Dyn<Iterator<U>> always is, its
//     table at 16 pointing at the adapter's vtable and its payload at 24
//     holding the adapter;
//   - the adapter's `next`, the body each combinator differs in, reading
//     the pair above and answering the three-word sum every next answers.
//
// The source box is the builtin iterator object (T7-4) for a list's own
// iterator, or the box an inner lazy call answered — the chain's own
// inductive case — so a chain is boxes holding boxes holding the object,
// and every dispatch in it goes through a table. The element work happens
// where an acute combinator walks the outermost box: one `next` call per
// pass, the tag ending the walk — the same walk a for statement takes.
//
// The pins below run through the real check pipeline (checkShapes), not
// the bare Emit helper: the iterator face's slot table lives in the
// shapes the checker registers, and a pipeline without them stops this
// face for a reason no program can reach (that gap is the Emit helper's
// own, pinned by the re-anchored stop test at the bottom).

// lazyBody is one program whose source is a builtin Int64 list and whose
// body is body — the one source the lazy face admits, so the pins are
// about the adapters and not about the source.
func lazyBody(body string) string {
	return fmt.Sprintf(`import std.io

pub type AppError = Failed(String)

pub fn main() effect io -> Result<(), AppError> {
    let xs = [1, 2, 3]
%s    return Ok(())
}
`, body)
}

// emitLazySrc checks and emits one lazy-program, refusing boundaries: the
// face's own claim is that these programs run, so a boundary here is a
// failure of the fixture or of the face, never a fact to pin.
func emitLazySrc(t *testing.T, src string) string {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q over the lazy face:\n%s", ni.What, src)
	}
	return ir
}

// lazyStop checks and emits one lazy-program, answering its boundary: the
// negative pins' helper, the mirror of emitLazySrc.
func lazyStop(t *testing.T, src string) *NotImplemented {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	return ni
}

// objectAfter names the object whose +16 slot carries head's table — the
// box an outer adapter holds at its own 16. The store names the gep's
// result, so the pin reads one line up, to the gep's base.
func objectAfter(t *testing.T, ir, head string) string {
	t.Helper()
	m := regexp.MustCompile(`getelementptr i8, ptr (%v\d+), i64 16\n\s+store ptr ` + regexp.QuoteMeta(head) + `, ptr %v\d+`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("no object carries the table %s:\n%s", head, ir)
	}
	return m[1]
}

// defineOf slices one adapter's own next-define out of the IR — its body
// between the header and the brace at column zero. The blocks inside every
// adapter share names (entry, step, none), so a pin that wants one
// adapter's block must ask within its define, not across the module.
func defineOf(t *testing.T, ir, name string) string {
	t.Helper()
	m := regexp.MustCompile(`define internal [^\n]*@` + regexp.QuoteMeta(name) + `\(ptr %self\) \{\n([\s\S]*?)\n\}`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("no define for %s:\n%s", name, ir)
	}
	return m[1]
}

// TestLazyMapBuildsTheAdapterAndItsBox: map's call site builds exactly the
// pair design D8 names. The adapter holds the source box at 16 (traced: a
// box is a gc value) and the fn carrier at 24; the box holds the table at
// 16 and the adapter at 24; the table's one slot is the adapter head's
// next-thunk, and the head's name carries both element domains — the
// source's T and the check stage's recorded U, which the parameter alone
// cannot say.
func TestLazyMapBuildsTheAdapterAndItsBox(t *testing.T) {
	ir := emitLazySrc(t, lazyBody("    let ys = xs.iterator().map(|x| x * 2)\n"))
	box := objectAfter(t, ir, "@.vt.Iterator$Int64.MapIter$Int64$Int64")
	// The box's payload at 24 names the adapter it carries.
	m := regexp.MustCompile(`getelementptr i8, ptr ` + regexp.QuoteMeta(box) + `, i64 24\n\s+store ptr (%v\d+), ptr %v\d+\n`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the box carries its adapter at 24:\n%s", ir)
	}
	obj := m[1]
	// The adapter's +16 store writes the very register the list iterator's
	// own box is — the source this adapter walks is that box.
	src := regexp.MustCompile(`getelementptr i8, ptr ` + regexp.QuoteMeta(obj) + `, i64 16\n\s+store ptr (%v\d+), ptr %v\d+\n`).FindStringSubmatch(ir)
	if src == nil {
		t.Fatalf("the adapter holds a source at 16:\n%s", ir)
	}
	srcBox := objectAfter(t, ir, "@.vt.Iterator$Int64.ListIter$Int64")
	if src[1] != srcBox {
		t.Fatalf("the source at 16 is the list iterator's box %s, not %s:\n%s", srcBox, src[1], ir)
	}
	// And the parameter at 24 is the fn value's carrier — the allocation
	// whose own +16 store wrote the callback's code pointer.
	car := regexp.MustCompile(`(%v\d+) = call ptr @__we_alloc\(i64 32\)\n\s+store ptr @\.fnmap\d+, ptr %v\d+\n\s+call void @__we_root_push\(ptr %v\d+\)\n\s+%v\d+ = getelementptr i8, ptr %v\d+, i64 16\n\s+store ptr @\.cb\d+, ptr %v\d+\n\s+%v\d+ = getelementptr i8, ptr %v\d+, i64 24`).FindStringSubmatch(ir)
	if car == nil {
		t.Fatalf("the callback's carrier is built before its holder:\n%s", ir)
	}
	if !strings.Contains(ir, "getelementptr i8, ptr "+regexp.QuoteMeta(obj)+", i64 24\n  store ptr "+car[1]+", ptr ") {
		t.Fatalf("the adapter holds the fn carrier %s at 24:\n%s", car[1], ir)
	}
	// The head's name names both domains and the table's one slot is its
	// next — the face's only slot (T6's pin).
	if !strings.Contains(ir, "@.vt.Iterator$Int64.MapIter$Int64$Int64 = private unnamed_addr constant [1 x ptr] [ptr @.vt.Iterator$Int64.MapIter$Int64$Int64.next]") {
		t.Fatalf("the table's one slot is the adapter head's next:\n%s", ir)
	}
}

// TestLazyFilterMissLoopsTheDispatch: a failed predicate re-enters the
// dispatch rather than answering — filter's next answers the first element
// that holds — and the loop head the miss branches back to cannot be the
// define's entry block, so the body opens with a fall-through into a block
// of its own.
func TestLazyFilterMissLoopsTheDispatch(t *testing.T) {
	ir := emitLazySrc(t, lazyBody("    let ys = xs.iterator().filter(|x| x > 1)\n"))
	fn := defineOf(t, ir, "FilterIter$Int64.next")
	m := regexp.MustCompile(`^entry:\n\s+br label %loop\nloop:`).FindString(fn)
	if m == "" {
		t.Fatalf("the miss loop sits in a block of its own past the entry:\n%s", fn)
	}
	loop := blockAt(fn, "loop")
	order(t, loop, "= call { i64, i64, i64 } %thk(ptr %src)", "icmp eq i64 %tag, 1", "br i1 %is, label %test, label %none")
	test := blockAt(fn, "test")
	order(t, test, "%v = extractvalue { i64, i64, i64 } %o, 1", "= call i64 %fn(ptr %env, i64 %v)", "%keep = icmp ne i64 %b, 0", "br i1 %keep, label %step, label %loop")
	none := blockAt(fn, "none")
	if !strings.Contains(none, "%tg = phi i64 [ 1, %step ], [ 0, %loop ]") {
		t.Fatalf("the none phi's edges are the loop's own:\n%s", none)
	}
}

// TestLazyTakeCountsDownInItsSlot: take's parameter is a count that crosses
// the back edge — a slot the body reads before each dispatch, tests at
// zero, and decrements when a pass answers. An exhausted take answers None
// without dispatching at all: the count test precedes the source's next.
func TestLazyTakeCountsDownInItsSlot(t *testing.T) {
	ir := emitLazySrc(t, lazyBody("    let ys = xs.iterator().take(2)\n"))
	fn := defineOf(t, ir, "TakeIter$Int64.next")
	m := regexp.MustCompile(`^entry:\n\s+%rp = getelementptr i8, ptr %self, i64 24\n\s+%r = load i64, ptr %rp\n\s+%done = icmp sle i64 %r, 0\n\s+br i1 %done, label %none, label %head`).FindString(fn)
	if m == "" {
		t.Fatalf("the count test opens the body, ahead of any dispatch:\n%s", fn)
	}
	step := blockAt(fn, "step")
	order(t, step, "%r1 = sub i64 %r, 1", "store i64 %r1, ptr %rp", "br label %none")
	none := blockAt(fn, "none")
	if !strings.Contains(none, "%tg = phi i64 [ 1, %step ], [ 0, %entry ], [ 0, %head ]") {
		t.Fatalf("the none phi carries the exhausted edge too:\n%s", none)
	}
}

// TestLazySkipDrainsThenPasses: skip's count spends itself in a drain loop
// of its own — each drained Some decrements the slot and re-tests — and
// once spent the body is a pass-through: the pass dispatch answers what the
// source answers, payload untouched. The drain and the pass keep separate
// register prefixes, so both dispatches can name their own next result.
func TestLazySkipDrainsThenPasses(t *testing.T) {
	ir := emitLazySrc(t, lazyBody("    let ys = xs.iterator().skip(1)\n"))
	fn := defineOf(t, ir, "SkipIter$Int64.next")
	m := regexp.MustCompile(`^entry:\n\s+br label %check\ncheck:\n\s+%sp0 = getelementptr i8, ptr %self, i64 24\n\s+%s0 = load i64, ptr %sp0\n\s+%draining = icmp sgt i64 %s0, 0\n\s+br i1 %draining, label %drain, label %pass`).FindString(fn)
	if m == "" {
		t.Fatalf("the drain test opens the body past the entry block:\n%s", fn)
	}
	dec := blockAt(fn, "dec")
	order(t, dec, "%s1 = sub i64 %s0, 1", "store i64 %s1, ptr %sp0", "br label %check")
	pass := blockAt(fn, "pass")
	order(t, pass, "%pthk = load ptr, ptr %pthp", "%po = call { i64, i64, i64 } %pthk(ptr %psrc)", "br i1 %pis, label %step, label %none")
	step := blockAt(fn, "step")
	if !strings.Contains(step, "%v = extractvalue { i64, i64, i64 } %po, 1") {
		t.Fatalf("the pass answers the source's own payload word:\n%s", step)
	}
	if strings.Contains(step, "call i64 %fn") {
		t.Fatalf("skip's payload crosses untouched — no fn of its own reads it:\n%s", step)
	}
}

// TestLazyMapFloatURoundTripsTheBits: a Float64 U rides the double domain
// inside the adapter — the fn is called with a double and answered one, and
// the word the sum carries is the double's bits — the same two casts an
// acute callback's domain takes, so the chain's U is not a second numeric
// convention but the vocabulary's own.
func TestLazyMapFloatURoundTripsTheBits(t *testing.T) {
	ir := emitLazySrc(t, `import std.io

pub type AppError = Failed(String)

pub fn main() effect io -> Result<(), AppError> {
    let fs = [1.5, 2.5]
    let ys = fs.iterator().map(|x| x + 1.0)
    return Ok(())
}
`)
	m := regexp.MustCompile(`define internal \{ i64, i64, i64 \} @MapIter\$Float64\$Float64\.next\(ptr %self\) \{[\s\S]*?%vd = bitcast i64 %v to double[\s\S]*?%m = call double %fn\(ptr %env, double %vd\)[\s\S]*?%w = bitcast double %m to i64`).FindString(ir)
	if m == "" {
		t.Fatalf("the Float64 element and U both cross as doubles, cast at the edges:\n%s", ir)
	}
}

// TestLazyChainThreadsBoxToBox: the chain's inductive case — take over
// map — is boxes holding boxes: take's adapter holds map's BOX at 16, not
// the list's object, and the two heads carry their own tables. The walk
// that consumes the chain dispatches once per pass through the outermost
// table, the adapters calling inward through theirs.
func TestLazyChainThreadsBoxToBox(t *testing.T) {
	ir := emitLazySrc(t, lazyBody("    let ys = xs.iterator().map(|x| x).take(2)\n"))
	if !strings.Contains(ir, "@.vt.Iterator$Int64.MapIter$Int64$Int64") {
		t.Fatalf("the map head keeps its own table:\n%s", ir)
	}
	// take's box carries its adapter at 24; that adapter's +16 store names
	// the link it walks, and the register it names is map's BOX — not the
	// list iterator's box, which map's own adapter holds one link down.
	takeBox := objectAfter(t, ir, "@.vt.Iterator$Int64.TakeIter$Int64")
	m := regexp.MustCompile(`getelementptr i8, ptr ` + regexp.QuoteMeta(takeBox) + `, i64 24\n\s+store ptr (%v\d+), ptr %v\d+\n`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("take's box carries its adapter at 24:\n%s", ir)
	}
	src := regexp.MustCompile(`getelementptr i8, ptr ` + regexp.QuoteMeta(m[1]) + `, i64 16\n\s+store ptr (%v\d+), ptr %v\d+\n`).FindStringSubmatch(ir)
	if src == nil {
		t.Fatalf("take's adapter holds a source at 16:\n%s", ir)
	}
	mapBox := objectAfter(t, ir, "@.vt.Iterator$Int64.MapIter$Int64$Int64")
	if src[1] != mapBox {
		t.Fatalf("take's adapter holds map's box %s, not %s:\n%s", mapBox, src[1], ir)
	}
	listBox := objectAfter(t, ir, "@.vt.Iterator$Int64.ListIter$Int64")
	if src[1] == listBox {
		t.Fatalf("the chain skips no link — take holds map's box, not the list's %s:\n%s", listBox, ir)
	}
}

// TestLazyAcuteWalksTheChainsBox: an acute combinator over a lazy
// receiver walks the box the chain answered — the head dispatches through
// the outermost table, one next per pass — and the walk it reuses is the
// acute loop's own (the counter, the exit, the payload slot), so the two
// faces share one way of walking rather than the lazy one inventing a
// second.
func TestLazyAcuteWalksTheChainsBox(t *testing.T) {
	ir := emitLazySrc(t, lazyBody("    let n = xs.iterator().map(|x| x).count()\n"))
	head := walkLabel(t, ir, "chead")
	h := blockAt(ir, head)
	// The head's dispatch: load the table out of the box, slot 0, call.
	m := regexp.MustCompile(`(%v\d+) = getelementptr i8, ptr (%v\d+), i64 16\n\s+(%v\d+) = load ptr, ptr %v\d+\n\s+(%v\d+) = getelementptr i8, ptr %v\d+, i64 0\n\s+(%v\d+) = load ptr, ptr %v\d+\n\s+(%v\d+) = call \{ i64, i64, i64 \} %v\d+\(ptr %v\d+\)`).FindStringSubmatch(h)
	if m == nil {
		t.Fatalf("the head dispatches next through the box's own table:\n%s", h)
	}
	if m[2] != objectAfter(t, ir, "@.vt.Iterator$Int64.MapIter$Int64$Int64") {
		t.Fatalf("the dispatched box is the chain's result box:\n%s", ir)
	}
	if got := strings.Count(ir, "@__we_list_snap"); got != 0 {
		t.Fatalf("a lazy chain walks no carrier snapshot, got %d:\n%s", got, ir)
	}
}

// TestLazyOverAStringSourceStops: a String's iterator is no source the
// lazy face admits — the builtin object face has no String element domain
// (the same closed face the carrier vocabulary reads) — so the call is not
// this face's and lands on the boundary below, exactly where it landed
// before the face existed.
func TestLazyOverAStringSourceStops(t *testing.T) {
	ni := lazyStop(t, `import std.io

pub type AppError = Failed(String)

pub fn main() effect io -> Result<(), AppError> {
    let s = "ab"
    let ys = s.iterator().map(|x| x)
    return Ok(())
}
`)
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}

// TestLazyOverAGcElementSourceStops: a gc record element has no word the
// adapter's sum could carry for a further acute walk to read back — the
// same element-domain boundary the carrier vocabulary takes — so the call
// stops rather than the box guessing a face.
func TestLazyOverAGcElementSourceStops(t *testing.T) {
	ni := lazyStop(t, `import std.io

pub type AppError = Failed(String)

record Cell { n: Int64 }

pub fn main() effect io -> Result<(), AppError> {
    let cs = [Cell { n: 1 }]
    let ys = cs.iterator().map(|c| c)
    return Ok(())
}
`)
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}

// No arity pin lives here: a `take()` or `take(1, 2)` spelling never
// reaches this emitter — the checker's own arity word ("calls with an
// argument count the callee does not declare") stops it first, exactly as
// a Float64 count stops at E0501 — so the emission's one-argument guard
// is defense in depth, unreachable through the checked pipeline the other
// pins run through.

// T8-3 the bound form (design D8 decision 2's closing half): the receiver
// of an acute call may NAME a box — a `let` binding holding the
// Dyn<Iterator<U>> a chain answered or a builtin source's explicitly boxed
// iterator — rather than be that chain inline. The binding carries the box
// with its face (bindResult's gc arm), so the walk the acute loop takes is
// the same one the inline form takes: the box's own table, one next per
// pass. Only the Iterator face is claimed by name: a binding of any other
// erased face keeps its call to the generic dispatch below, which answers
// the names that face's table actually holds.

// TestLazyAcuteWalksABoundChainsBox: the chain's result crosses a `let`
// and the acute call still walks THE box the chain answered — the walk
// head's table load reads the very register the box's own +16 store wrote
// through, and the chain was built once, at its own call, not again at the
// read. The count answers over the walked box, never a carrier snapshot.
func TestLazyAcuteWalksABoundChainsBox(t *testing.T) {
	ir := emitLazySrc(t, lazyBody("    let it = xs.iterator().map(|x| x * 2)\n    let n = it.count()\n"))
	h := blockAt(ir, walkLabel(t, ir, "chead"))
	m := regexp.MustCompile(`getelementptr i8, ptr (%v\d+), i64 16`).FindStringSubmatch(h)
	if m == nil {
		t.Fatalf("the walk head loads the box's table word:\n%s", h)
	}
	if box := objectAfter(t, ir, "@.vt.Iterator$Int64.MapIter$Int64$Int64"); m[1] != box {
		t.Fatalf("the walked box is the chain's result box %s, not %s:\n%s", box, m[1], ir)
	}
	if got := strings.Count(ir, "@.vt.Iterator$Int64.MapIter$Int64$Int64 = "); got != 1 {
		t.Fatalf("the chain was built once, got %d tables:\n%s", got, ir)
	}
	if got := strings.Count(ir, "@__we_list_snap"); got != 0 {
		t.Fatalf("a bound box walks no carrier snapshot, got %d:\n%s", got, ir)
	}
}

// TestLazyAcuteWalksABoundBuiltinBox: an explicitly boxed builtin iterator
// (T7-4's object inside a Dyn of the Iterator face) crosses a `let` the
// same way — the walk dispatches through the bound box's own table, and
// the object's next is the thunk the table's one slot holds.
func TestLazyAcuteWalksABoundBuiltinBox(t *testing.T) {
	ir := emitLazySrc(t, lazyBody("    let b = Dyn<Iterator<Int64> >(xs.iterator())\n    let n = b.count()\n"))
	h := blockAt(ir, walkLabel(t, ir, "chead"))
	m := regexp.MustCompile(`getelementptr i8, ptr (%v\d+), i64 16`).FindStringSubmatch(h)
	if m == nil {
		t.Fatalf("the walk head loads the box's table word:\n%s", h)
	}
	if box := objectAfter(t, ir, "@.vt.Iterator$Int64.ListIter$Int64"); m[1] != box {
		t.Fatalf("the walked box is the bound builtin box %s, not %s:\n%s", box, m[1], ir)
	}
	if !strings.Contains(ir, "define internal { i64, i64, i64 } @ListIter$Int64.next(ptr %self)") {
		t.Fatalf("the object's own next is defined:\n%s", ir)
	}
}

// TestAcuteOverAUserFacesOwnCountDispatches: the name check is the whole
// boundary — a user interface whose own slot is NAMED `count` (one of the
// seven) is not the Iterator face, so a binding of its box must dispatch
// through ITS table to the user's method, not be walked as an iterator.
// The walk would be the wrong program: it would spin the box's payload as
// a cursor the user never declared.
func TestAcuteOverAUserFacesOwnCountDispatches(t *testing.T) {
	ir := emitLazySrc(t, `import std.io

pub type AppError = Failed(String)

interface Box {
    fn count(mut self) -> Int64
}

record Held {
    n: Int64
}

impl Box for Held {
    fn count(mut self) -> Int64 {
        return self.n
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Box>(Held { n: 7 })
    let n = d.count()
    io.println("${n}")
    return Ok(())
}
`)
	if !strings.Contains(ir, "@.vt.main.Box.main.Held") {
		t.Fatalf("the user face's own table is built:\n%s", ir)
	}
	if !strings.Contains(ir, "@main.Held.count") {
		t.Fatalf("the user's count is the slot's target:\n%s", ir)
	}
	if strings.Contains(ir, "chead") || strings.Contains(ir, "@.vt.Iterator") {
		t.Fatalf("a user count slot dispatches; no iterator walk was claimed:\n%s", ir)
	}
}
