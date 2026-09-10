package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T7-3 the six acute combinators (design D6, chapter 11). The standard
// library writes their bodies in We over Iterator<T>; the emission face
// takes the loop the body describes and emits it directly, so the emitted
// spellings are:
//
//   - the call form is `<list>.iterator().<name>(args)` — the receiver's
//     face comes from the same environment a for statement's source does,
//     and a receiver that is no List is not this face at all;
//   - the walk is the for walk's: snapshot, root, length, counted head,
//     element read per pass — so chapter 17's fixed-sequence rule holds
//     here for the same reason and by the same code path;
//   - the callback is a fn value built by the T6 machinery at the call
//     site, outside the loop, and called `<fnptr>(env, elem…)`;
//   - fold and reduce carry an accumulator in a slot (it crosses the back
//     edge), count carries the count, and any/all/find branch out of the
//     loop at the element that decides them;
//   - reduce and find answer Option<E> — the sum pair, None 0 and Some 1 —
//     and stop where the payload word cannot be the value a match arm
//     would bind.
//
// The emitted shape is pinned below; the answers are the conformance
// goldens' and the loop's own behaviour is T7-2's, already pinned.

// iterOf is `<src>.iterator()`.
func iterOf(src ast.Expr) ast.Expr { return callOn(src, "iterator") }

// acute is `<src>.iterator().<name>(args…)`.
func acute(src ast.Expr, name string, args ...ast.Expr) *ast.Call {
	return callOn(iterOf(src), name, args...)
}

// lam is a short closure over bare parameters: the combinator position
// fixes their faces, so nothing here names a type.
func lam(body ast.Expr, params ...string) *ast.Closure {
	ps := make([]ast.Param, len(params))
	for i, n := range params {
		ps[i] = ast.Param{Name: n}
	}
	return &ast.Closure{Params: ps, Short: true,
		Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: body}}}}
}

// acuteModule binds a three-integer list and one combinator result, and
// closes the body. The two Option-answering combinators stop here: a sum
// has no interpolation face in this build, so printing one is a boundary
// of its own rather than a fact about the combinator.
func acuteModule(name string, args ...ast.Expr) *ast.File {
	return listModule(nil,
		listBind("xs", intLit("1"), intLit("2"), intLit("3")),
		letBind("n", acute(ident("xs"), name, args...)),
		okReturn(),
	)
}

// resultModule is acuteModule that also prints the result — the shape the
// four scalar-answering combinators take.
func resultModule(name string, args ...ast.Expr) *ast.File {
	return listModule(nil,
		listBind("xs", intLit("1"), intLit("2"), intLit("3")),
		letBind("n", acute(ident("xs"), name, args...)),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	)
}

// midLoads matches the instructions between one element's conversion and
// the callback's operands: loading the fn value's own pair back out, which
// is T6's spelling and no part of a combinator's shape.
const midLoads = `(?:\s+%v\d+ = (?:getelementptr|load) [^\n]*\n)*`

// accSlot reads back the slot an accumulator store wrote, and requires
// that store to precede the walk's snapshot — the callback's carrier is
// built in between, so the two are never adjacent.
func accSlot(t *testing.T, ir, storeRe string) string {
	t.Helper()
	m := regexp.MustCompile(storeRe + `, ptr (%v\d+)`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("no store matching %q:\n%s", storeRe, ir)
	}
	if strings.Index(ir, m[0]) > strings.Index(ir, "= call ptr @__we_list_snap") {
		t.Fatalf("the accumulator is initialised before the snapshot:\n%s", ir)
	}
	return m[1]
}

// TestAcuteWalkSnapshotsBeforeTheHead: a combinator's walk is chapter
// 17's iterator — the sequence is fixed where the iterator is taken — and
// the combinator is where that matters most, since the callback may
// append to the very carrier being walked. The copy is rooted (the walk
// outlives every allocation the callback makes) and the element reads are
// the SNAPSHOT's, not the live carrier's.
func TestAcuteWalkSnapshotsBeforeTheHead(t *testing.T) {
	ir := assertClean(t, resultModule("any",
		lam(binOp(">", ident("x"), intLit("0")), "x")))
	m := regexp.MustCompile(`(%v\d+) = call ptr @__we_list_snap\(ptr (%v\d+)\)\n\s+call void @__we_root_push\(ptr (%v\d+)\)`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the walk snapshots and roots before it reads:\n%s", ir)
	}
	snap, src := m[1], m[2]
	if m[3] != snap {
		t.Fatalf("the root pushed is the snapshot %s, not %s:\n%s", snap, m[3], ir)
	}
	if strings.Index(ir, "= call ptr @__we_list_snap") > strings.Index(ir, "chead") {
		t.Fatalf("the snapshot is taken before the head opens:\n%s", ir)
	}
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = call i64 @__we_list_get\(ptr ` + regexp.QuoteMeta(snap) + `, i64 %v\d+\)`).MatchString(ir) {
		t.Fatalf("the element reads run over the snapshot %s, not the source %s:\n%s", snap, src, ir)
	}
}

// TestAcuteFoldThreadsTheAccumulator: fold's accumulator crosses the
// loop's back edge, so it lives in a slot: initialised once before the
// head, read into the call, written back from it, and read out after the
// exit. The call carries the accumulator first and the element second —
// chapter 11's `f(acc, x)`. The initial value is deliberately NOT the
// domain's identity: the slot's opening store is the init expression's own
// value, and an init of zero would leave that store indistinguishable from
// one that ignores the init and opens at zero.
func TestAcuteFoldThreadsTheAccumulator(t *testing.T) {
	ir := assertClean(t, resultModule("fold", intLit("5"),
		lam(binOp("+", ident("acc"), ident("x")), "acc", "x")))
	acc := regexp.QuoteMeta(accSlot(t, ir, `store i64 5`))
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = call i64 @__we_list_get\(ptr %v\d+, i64 %v\d+\)\n` + midLoads + `\s+%v\d+ = load i64, ptr ` + acc + `\n\s+%v\d+ = call i64 %v\d+\(ptr %v\d+, i64 %v\d+, i64 %v\d+\)\n\s+store i64 %v\d+, ptr ` + acc).MatchString(ir) {
		t.Fatalf("the body reads the element, calls with (acc, element), and writes the accumulator back:\n%s", ir)
	}
	if !regexp.MustCompile(`cexit\d+:\n\s+%v\d+ = load i64, ptr ` + acc).MatchString(ir) {
		t.Fatalf("the result is the accumulator read after the exit:\n%s", ir)
	}
}

// TestAcuteCountCountsThePasses: count takes no callback — the walk's own
// passes are the answer — so nothing here builds a fn value, and the
// counter is a slot the loop increments and the exit reads.
func TestAcuteCountCountsThePasses(t *testing.T) {
	ir := assertClean(t, resultModule("count"))
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = call i64 @__we_list_get\(ptr %v\d+, i64 %v\d+\)\n\s+%v\d+ = load i64, ptr (%v\d+)\n\s+%v\d+ = add i64 %v\d+, 1\n\s+store i64 %v\d+, ptr %v\d+`).MatchString(ir) {
		t.Fatalf("each pass increments the count:\n%s", ir)
	}
	if strings.Contains(ir, "@__we_alloc") {
		t.Fatalf("count builds no callback:\n%s", ir)
	}
}

// TestAcuteAnyShortCircuits: any's answer flips to true at the first
// element whose predicate holds, and the walk ends there rather than
// running the rest of the carrier — the hit block stores and branches
// straight to the exit.
func TestAcuteAnyShortCircuits(t *testing.T) {
	ir := assertClean(t, resultModule("any",
		lam(binOp(">", ident("x"), intLit("1")), "x")))
	res := regexp.QuoteMeta(accSlot(t, ir, `store i64 0`))
	if !regexp.MustCompile(`%v\d+ = icmp eq i64 %v\d+, 1\n\s+br i1 %v\d+, label %qhit\d+, label %qcont\d+`).MatchString(ir) {
		t.Fatalf("the predicate's hit branches out of the loop:\n%s", ir)
	}
	if !regexp.MustCompile(`qhit\d+:\n\s+store i64 1, ptr ` + res + `\n\s+br label %cexit\d+`).MatchString(ir) {
		t.Fatalf("the hit stores true and ends the walk:\n%s", ir)
	}
}

// TestAcuteAllShortCircuits: all is any's dual — the answer starts true
// and the first element whose predicate fails makes it false and ends the
// walk. A carrier that runs out leaves the true standing.
func TestAcuteAllShortCircuits(t *testing.T) {
	ir := assertClean(t, resultModule("all",
		lam(binOp(">", ident("x"), intLit("1")), "x")))
	res := regexp.QuoteMeta(accSlot(t, ir, `store i64 1`))
	if !regexp.MustCompile(`%v\d+ = icmp eq i64 %v\d+, 0\n\s+br i1 %v\d+, label %qhit\d+, label %qcont\d+`).MatchString(ir) {
		t.Fatalf("all ends the walk on the predicate's miss:\n%s", ir)
	}
	if !regexp.MustCompile(`qhit\d+:\n\s+store i64 0, ptr ` + res + `\n\s+br label %cexit\d+`).MatchString(ir) {
		t.Fatalf("the hit stores false and ends the walk:\n%s", ir)
	}
}

// TestAcuteReduceStartsFromTheFirstElement: reduce takes no initial
// value — index zero IS the accumulator. The first pass stores it into the
// payload and raises the tag; every later pass folds through the callback.
// An empty carrier leaves the tag at zero, which is None.
func TestAcuteReduceStartsFromTheFirstElement(t *testing.T) {
	ir := assertClean(t, acuteModule("reduce",
		lam(binOp("+", ident("acc"), ident("x")), "acc", "x")))
	m := regexp.MustCompile(`store i64 0, ptr (%v\d+)\n\s+store i64 0, ptr (%v\d+)`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("reduce opens with the pair at None:\n%s", ir)
	}
	tag, pay := regexp.QuoteMeta(m[1]), regexp.QuoteMeta(m[2])
	if !regexp.MustCompile(`%v\d+ = icmp eq i64 %v\d+, 0\n\s+br i1 %v\d+, label %rfirst\d+, label %rlater\d+`).MatchString(ir) {
		t.Fatalf("the first pass is the accumulator, the rest are steps:\n%s", ir)
	}
	if !regexp.MustCompile(`rfirst\d+:\n\s+store i64 1, ptr ` + tag + `\n\s+store i64 %v\d+, ptr ` + pay + `\n\s+br label %cstep\d+`).MatchString(ir) {
		t.Fatalf("the first element becomes Some(element):\n%s", ir)
	}
	if !regexp.MustCompile(`rlater\d+:\n` + midLoads + `\s+%v\d+ = load i64, ptr ` + pay + `\n\s+%v\d+ = call i64 %v\d+\(ptr %v\d+, i64 %v\d+, i64 %v\d+\)\n\s+store i64 %v\d+, ptr ` + pay).MatchString(ir) {
		t.Fatalf("a later pass folds (acc, element) back into the payload:\n%s", ir)
	}
}

// TestAcuteFindAnswersSomeAtTheFirstHit: find's predicate decides alone —
// the first element it holds becomes the Some payload and the walk ends;
// no element leaves None. The payload is the element WORD, the very word
// the walk read.
func TestAcuteFindAnswersSomeAtTheFirstHit(t *testing.T) {
	ir := assertClean(t, acuteModule("find",
		lam(binOp(">", ident("x"), intLit("1")), "x")))
	m := regexp.MustCompile(`store i64 0, ptr (%v\d+)\n\s+store i64 0, ptr (%v\d+)`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("find opens with the pair at None:\n%s", ir)
	}
	tag, pay := regexp.QuoteMeta(m[1]), regexp.QuoteMeta(m[2])
	if !regexp.MustCompile(`fhit\d+:\n\s+store i64 1, ptr ` + tag + `\n\s+store i64 %v\d+, ptr ` + pay + `\n\s+br label %cexit\d+`).MatchString(ir) {
		t.Fatalf("the hit answers Some and ends the walk:\n%s", ir)
	}
	// The payload is the element read's own register — a payload the loop
	// recomputed would be the same shape while answering something else.
	elem := regexp.MustCompile(`cbody\d+:\n\s+(%v\d+) = call i64 @__we_list_get`).FindStringSubmatch(ir)
	if elem == nil {
		t.Fatalf("the body reads the element:\n%s", ir)
	}
	if !strings.Contains(ir, "store i64 "+elem[1]+", ptr "+m[2]) {
		t.Fatalf("the Some payload is the element word %s itself:\n%s", elem[1], ir)
	}
}

// TestAcuteFoldFloatAccumulator: a Float64 accumulator takes a double
// slot and a `double` call operand, so the domain the chapter's own
// declaration puts the accumulator in is the domain the loop carries it
// in — no conversion, and no double's bits taken for an integer's.
func TestAcuteFoldFloatAccumulator(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		listBind("fs", floatLit("1.5"), floatLit("2.5")),
		letBind("n", acute(ident("fs"), "fold", floatLit("0.0"),
			lam(binOp("+", ident("acc"), ident("x")), "acc", "x"))),
		okReturn(),
	))
	acc := regexp.QuoteMeta(accSlot(t, ir, `store double \S+`))
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = call i64 @__we_list_get\(ptr %v\d+, i64 %v\d+\)\n\s+%v\d+ = bitcast i64 %v\d+ to double\n` + midLoads + `\s+%v\d+ = load double, ptr ` + acc + `\n\s+%v\d+ = call double %v\d+\(ptr %v\d+, double %v\d+, double %v\d+\)`).MatchString(ir) {
		t.Fatalf("the accumulator and the element both cross as doubles:\n%s", ir)
	}
}

// TestAcuteCallbackTakesTheElementsFace: a gc record element is one word —
// its handle — so the loop converts the word back to a pointer and the
// callback's parameter is that pointer, with the record key carried in the
// signature so a field read inside the body lands on the record's own
// offsets.
func TestAcuteCallbackTakesTheElementsFace(t *testing.T) {
	ir := assertClean(t, listModule(
		[]ast.Item{recDecl("Cell", "gc", fld("n", "Int64"))},
		letBind("cs", &ast.ListLit{Elems: []ast.Expr{construct("Cell", init1("n", intLit("1"))),
			construct("Cell", init1("n", intLit("2")))}}),
		varLit("total", "0"),
		assignTo("total", acute(ident("cs"), "fold", intLit("0"),
			lam(binOp("+", ident("acc"), memberOf(ident("c"), "n")), "acc", "c"))),
	), "define internal i64 @.cb0(ptr %env, i64 %acc, ptr %c) {")
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = call i64 @__we_list_get\(ptr %v\d+, i64 %v\d+\)\n\s+%v\d+ = inttoptr i64 %v\d+ to ptr\n` + midLoads + `\s+%v\d+ = load i64, ptr %v\d+\n\s+%v\d+ = call i64 %v\d+\(ptr %v\d+, i64 %v\d+, ptr %v\d+\)`).MatchString(ir) {
		t.Fatalf("the element crosses as the pointer its handle names:\n%s", ir)
	}
}

// TestAcuteCallbackIsBuiltOutsideTheLoop: the callback's carrier is a gc
// allocation at the creation site, and the creation site is before the
// head — an allocation inside the loop would rebuild the closure on every
// pass and re-freeze captures that chapter 12 freezes once.
func TestAcuteCallbackIsBuiltOutsideTheLoop(t *testing.T) {
	ir := assertClean(t, resultModule("any",
		lam(binOp(">", ident("x"), intLit("1")), "x")))
	alloc := strings.Index(ir, "@__we_alloc(i64 32)")
	head := strings.Index(ir, "chead")
	if alloc < 0 || head < 0 || alloc > head {
		t.Fatalf("the callback is built before the head opens (alloc %d, head %d):\n%s", alloc, head, ir)
	}
	// And the loop body holds no allocation of its own: everything the
	// thunk needs was frozen before the walk began.
	if regexp.MustCompile(`cbody\d+:[\s\S]*?@__we_alloc`).MatchString(ir) {
		t.Fatalf("the body builds nothing:\n%s", ir)
	}
}

// TestAcutePredicateCopiesValueRecordElements: a byval record is copied
// on every binding (chapter 8), and the callback's parameter is a binding
// — so each pass hands the body its own record rather than a share of the
// carrier's object.
func TestAcutePredicateCopiesValueRecordElements(t *testing.T) {
	ir := assertClean(t, listModule(
		[]ast.Item{recDecl("Val", "value", fld("n", "Int64"))},
		letBind("vs", &ast.ListLit{Elems: []ast.Expr{construct("Val", init1("n", intLit("1"))),
			construct("Val", init1("n", intLit("2")))}}),
		letBind("n", acute(ident("vs"), "any",
			lam(binOp(">", memberOf(ident("v"), "n"), intLit("1")), "v"))),
		okReturn(),
	))
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = call i64 @__we_list_get\(ptr %v\d+, i64 %v\d+\)\n\s+%v\d+ = inttoptr i64 %v\d+ to ptr\n\s+%v\d+ = call ptr @__we_alloc`).MatchString(ir) {
		t.Fatalf("the callback receives a copy of the value record:\n%s", ir)
	}
}

// TestAcuteBoolAndRunePayloadsAnswer: Bool and Rune ride the i64 domain
// — a Bool's word is 0 or 1, a Rune's its code point — so the pair a
// reduce or find answers carries a value of the element's own type, and
// the loop emits rather than stopping at a payload it cannot answer.
func TestAcuteBoolAndRunePayloadsAnswer(t *testing.T) {
	assertClean(t, listModule(nil,
		listBind("bs", boolLit("true"), boolLit("false")),
		letBind("r", acute(ident("bs"), "reduce",
			lam(binOp("||", ident("p"), ident("q")), "p", "q"))),
		okReturn(),
	), "chead")
	assertClean(t, listModule(nil,
		listBind("rs", runeLit("'a'"), runeLit("'b'")),
		letBind("g", acute(ident("rs"), "find",
			lam(binOp("==", ident("c"), runeLit("'b'")), "c"))),
		okReturn(),
	), "chead")
}

// TestAcuteReduceFloatElementsStopAtTheBoundary: reduce answers
// Option<E>, and a match arm binds the payload into the scalar domain —
// so a Float64 element's payload word would be a bit pattern the arm
// would read as an integer. The element domain is the loop's own, and the
// loop stops where it cannot answer the declared type.
func TestAcuteReduceFloatElementsStopAtTheBoundary(t *testing.T) {
	_, ni := Emit(listModule(nil,
		listBind("fs", floatLit("1.5"), floatLit("2.5")),
		letBind("n", acute(ident("fs"), "reduce",
			lam(binOp("+", ident("a"), ident("b")), "a", "b"))),
		okReturn(),
	), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}

// TestAcuteFindGcElementsStopAtTheBoundary: the same rule for a gc
// handle — the payload word would be an address the arm would read as a
// number.
func TestAcuteFindGcElementsStopAtTheBoundary(t *testing.T) {
	_, ni := Emit(listModule(
		[]ast.Item{recDecl("Cell", "gc", fld("n", "Int64"))},
		letBind("cs", &ast.ListLit{Elems: []ast.Expr{construct("Cell", init1("n", intLit("1")))}}),
		letBind("n", acute(ident("cs"), "find",
			lam(binOp(">", memberOf(ident("c"), "n"), intLit("0")), "c"))),
		okReturn(),
	), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}

// TestAcuteOverANonListSourceStops: the recognized form needs a List
// receiver — a String's iterator is another face's, and this face leaves
// it to that one rather than reading a string as a carrier.
func TestAcuteOverANonListSourceStops(t *testing.T) {
	_, ni := Emit(listModule(nil,
		letBind("s", strLit("ab")),
		letBind("n", acute(ident("s"), "count")),
		okReturn(),
	), "demo")
	if ni == nil {
		t.Fatal("a non-List receiver is not this face")
	}
}

// TestAcuteOverANonIteratorReceiverStops: the recognized form's receiver is
// the ITERATOR call itself. A member call that is not `.iterator()` is not
// this face even over a List — reading it as one would drop the call and
// walk the receiver expression instead, which is a silently wrong answer
// rather than a missing feature. So the walk needs the name, not just a
// List under some call.
func TestAcuteOverANonIteratorReceiverStops(t *testing.T) {
	_, ni := Emit(listModule(nil,
		listBind("xs", intLit("1"), intLit("2")),
		letBind("n", callOn(callOn(ident("xs"), "twice"), "count")),
		okReturn(),
	), "demo")
	if ni == nil {
		t.Fatal("a receiver call that is not the iterator's is not this face")
	}
}

// TestAcuteOverAUserIterableStops: a user impl of Iterable is not this
// face either — its element face is a fact the environment does not carry
// — so the walk stops at the body boundary as it does for a for statement
// over the same source.
func TestAcuteOverAUserIterableStops(t *testing.T) {
	_, ni := Emit(listModule(
		[]ast.Item{recDecl("Range2", "gc", fld("n", "Int64"))},
		letBind("r", construct("Range2", init1("n", intLit("0")))),
		letBind("n", acute(ident("r"), "count")),
		okReturn(),
	), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}

// TestAcuteOutsideTheSixStops: the five lazy combinators are B1b's — their
// Dyn<Iterator<U>> results need the vtable face — so a lazy name over a
// List receiver is not this face and lands on the boundary below.
func TestAcuteOutsideTheSixStops(t *testing.T) {
	_, ni := Emit(listModule(nil,
		listBind("xs", intLit("1")),
		letBind("n", acute(ident("xs"), "filter",
			lam(binOp(">", ident("x"), intLit("0")), "x"))),
		okReturn(),
	), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}
