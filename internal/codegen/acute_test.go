package codegen

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T7-3/T8-1 the seven acute combinators (design D6, D8 decision 1,
// chapter 11). The standard library writes their bodies in We over
// Iterator<T>; the emission face takes the loop the body describes and
// emits it directly, so the emitted spellings are:
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
//   - collect answers the carrier a list literal builds — an empty list
//     opened before the walk, one push per pass — so the result is a
//     List<T> the element vocabulary reads directly, and no box is built
//     on its way out (T8-1);
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

// TestAcuteCollectBuildsTheCarrierWhileWalking: collect's answer is the
// carrier a list literal builds, so the call opens an empty list before
// the walk and pushes every element the walk answers. Each push reads
// the carrier back out of its slot — a growing push answers a new
// identity (list.c) and the slot is what carries it across the back
// edge — and the answer replaces the root in the same breath, the move
// being tracked by replacing the pushed pointer rather than writing
// through it (the shadow stack holds values, not addresses).
func TestAcuteCollectBuildsTheCarrierWhileWalking(t *testing.T) {
	ir := assertClean(t, acuteModule("collect"))
	m := regexp.MustCompile(`(%v\d+) = call ptr @__we_list_new\(i64 0, i64 0\)\n\s+call void @__we_root_push\(ptr %v\d+\)\n\s+store ptr %v\d+, ptr (%v\d+)\n\s+%v\d+ = call ptr @__we_list_snap`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the carrier is opened empty and rooted before the walk:\n%s", ir)
	}
	lst := regexp.QuoteMeta(m[2])
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = call i64 @__we_list_get\(ptr %v\d+, i64 %v\d+\)\n\s+%v\d+ = load ptr, ptr ` + lst + `\n\s+%v\d+ = call ptr @__we_list_push\(ptr %v\d+, i64 %v\d+\)\n\s+call void @__we_root_pop\(\)\n\s+call void @__we_root_push\(ptr %v\d+\)\n\s+store ptr %v\d+, ptr ` + lst).MatchString(ir) {
		t.Fatalf("each pass pushes the element and replaces the carrier's root:\n%s", ir)
	}
	if !regexp.MustCompile(`cexit\d+:\n\s+%v\d+ = load ptr, ptr ` + lst).MatchString(ir) {
		t.Fatalf("the result is the carrier read after the exit:\n%s", ir)
	}
	if !strings.Contains(ir, "declare ptr @__we_list_new(i64, i64)") {
		t.Fatalf("the carrier comes from the list family's own carving:\n%s", ir)
	}
}

// TestAcuteCollectTracesTheGcElement: a traced face's growth push is the
// first emitted push that may move — the copy's allocation can fire a
// collection with this pass's element still in a register — so the
// element handle is rooted for exactly that call and popped right after
// it, the discipline gc.c's header states for every gc reference. The
// carrier opens traced, which is what makes its own descriptor cover the
// pushed words.
func TestAcuteCollectTracesTheGcElement(t *testing.T) {
	ir := assertClean(t, listModule(
		[]ast.Item{recDecl("Cell", "gc", fld("n", "Int64"))},
		letBind("cs", &ast.ListLit{Elems: []ast.Expr{construct("Cell", init1("n", intLit("1"))),
			construct("Cell", init1("n", intLit("2")))}}),
		letBind("ys", acute(ident("cs"), "collect")),
		okReturn(),
	))
	if !strings.Contains(ir, "call ptr @__we_list_new(i64 0, i64 1)") {
		t.Fatalf("a traced face opens a traced carrier:\n%s", ir)
	}
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = call i64 @__we_list_get\(ptr %v\d+, i64 %v\d+\)\n\s+%v\d+ = inttoptr i64 %v\d+ to ptr\n\s+call void @__we_root_push\(ptr %v\d+\)\n\s+%v\d+ = load ptr, ptr %v\d+\n\s+%v\d+ = call ptr @__we_list_push\(ptr %v\d+, i64 %v\d+\)\n\s+call void @__we_root_pop\(\)\n\s+call void @__we_root_pop\(\)`).MatchString(ir) {
		t.Fatalf("the element is rooted around the push and popped right after it:\n%s", ir)
	}
}

// TestAcuteCollectResultWalksAgain: the answer registers in the same
// environment a list literal's binding does, so the next combinator (or
// for statement) walks it through that vocabulary — the exit's read of
// the carrier slot is the very register the second walk snapshots.
func TestAcuteCollectResultWalksAgain(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		listBind("xs", intLit("1"), intLit("2")),
		letBind("ys", acute(ident("xs"), "collect")),
		letBind("n", acute(ident("ys"), "count")),
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	))
	m := regexp.MustCompile(`cexit\d+:\n\s+(%v\d+) = load ptr, ptr %v\d+\n`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the collect's exit reads its carrier out of the slot:\n%s", ir)
	}
	snap := strings.LastIndex(ir, fmt.Sprintf("@__we_list_snap(ptr %s)", m[1]))
	if snap < 0 || snap < strings.Index(ir, m[0]) {
		t.Fatalf("the second walk snapshots the collected carrier:\n%s", ir)
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

// TestAcuteOverARecordThatIsNoIterableStops: a record is no source this
// face walks unless an impl bound the association under it — this fixture
// carries no impl at all — so the face declines and the call stops below
// it at the body boundary. The record source that DOES walk is the
// protocol pins below; what this one keeps is the gate: membership is the
// association's, not the receiver's shape.
func TestAcuteOverARecordThatIsNoIterableStops(t *testing.T) {
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

// --- the protocol face over a user Iterable (T7-B) -------------------------

// acuteIterSrc is one program carrying the protocol under the combinators:
// a handle with a cursor of its own, an Iterable whose association names
// it, and the body below. A `for` over the same shape is T7-2's; what
// this pins is that a combinator reads the SAME handle the statement does
// — one `iterator` call for the whole call, one `next` per pass, the tag
// ending the walk — so a combinator over a user sequence is no second
// iteration convention standing beside the statement's.
const acuteIterSrc = `import std.io

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
    pub fn iterator(self) -> CountIter {
        return CountIter { i: 0, hi: self.n }
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let r = Range2 { n: 3 }
%s    return Ok(())
}
`

// emitAcuteIter emits one program whose main body is body, which arrives
// already indented. src is one of these; a protocol face this build has
// no word for is a failure of the fixture, not of the test.
func emitAcuteIter(t *testing.T, body string) string {
	t.Helper()
	src := fmt.Sprintf(acuteIterSrc, body)
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q over the user-Iterable acute face:\n%s", ni.What, src)
	}
	return ir
}

// walkLabel reads back one walk's block name. The numbering is the
// emitter's own — a combinator that builds a callback opens blocks of its
// own first — so a pin that named a number would pin the wrong thing.
func walkLabel(t *testing.T, ir, prefix string) string {
	t.Helper()
	m := regexp.MustCompile(prefix + `\d+`).FindString(ir)
	if m == "" {
		t.Fatalf("no %s block in the walk:\n%s", prefix, ir)
	}
	return m
}

// TestAcuteProtocolBuildsTheHandleOnce: the combinator takes the same
// snapshot the for statement takes, and takes it the same way —
// `iterator` runs once for the whole call, before the head opens, and
// every pass reaches the sequence through that one handle. A walk that
// called `iterator` per pass would restart the sequence under the
// callback's own appends, which is chapter 17's fixed-sequence rule.
func TestAcuteProtocolBuildsTheHandleOnce(t *testing.T) {
	ir := emitAcuteIter(t, "    let n = r.iterator().count()\n")
	if got := countCall(ir, "ptr", "main.Range2.iterator"); got != 1 {
		t.Fatalf("want exactly one iterator call site, got %d:\n%s", got, ir)
	}
	head := walkLabel(t, ir, "chead")
	order(t, ir,
		"call ptr @main.Range2.iterator(ptr",
		"call void @__we_root_push(ptr", // the handle outlives every pass
		"br label %"+head,
		head+":",
		"call { i64, i64, i64 } @main.CountIter.next(ptr",
	)
	if got := strings.Count(ir, "@main.CountIter.next"); got != 2 {
		t.Fatalf("want the define plus one static call site, got %d:\n%s", got, ir)
	}
	if strings.Contains(ir, "__we_list_") {
		t.Fatalf("a user Iterable rides no carrier verb:\n%s", ir)
	}
}

// TestAcuteProtocolHeadTestsTheTag: the walk ends on the handle's own
// answer rather than on a length — there is no length to compare against
// — and the index it tests is `Some`'s in the variant table the callee's
// signature carries, which is the index the construction inside `next`
// writes. Nothing here invents a second tag convention.
func TestAcuteProtocolHeadTestsTheTag(t *testing.T) {
	ir := emitAcuteIter(t, "    let n = r.iterator().count()\n")
	head := blockAt(ir, walkLabel(t, ir, "chead"))
	if head == "" {
		t.Fatalf("missing the walk's head:\n%s", ir)
	}
	if strings.Contains(head, "icmp slt i64") {
		t.Fatalf("the protocol walk counts nothing against a bound:\n%s", head)
	}
	order(t, head, "call { i64, i64, i64 } @main.CountIter.next(ptr",
		"extractvalue", "store i64", "load i64, ptr", "icmp eq i64", "br i1")
	if !regexp.MustCompile(`icmp eq i64 %v\d+, 1`).MatchString(head) {
		t.Fatalf("the test names Some's index in the callee's table:\n%s", head)
	}
}

// TestAcuteProtocolFeedsTheCallbackThePayload: the element a callback
// takes is the Option's payload word — the same word a match arm would
// bind, read out of the same slot — so an element means one thing down
// both faces of the protocol. The face is the handle's declaration, which
// is why an Int64 payload crosses as an i64 with no conversion in between.
func TestAcuteProtocolFeedsTheCallbackThePayload(t *testing.T) {
	ir := emitAcuteIter(t, "    let n = r.iterator().fold(0, |acc, x| acc + x)\n")
	body := blockAt(ir, walkLabel(t, ir, "cbody"))
	if body == "" {
		t.Fatalf("missing the walk's body:\n%s", ir)
	}
	m := regexp.MustCompile(`(%v\d+) = load i64, ptr`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the body reads its element:\n%s", body)
	}
	if !strings.Contains(ir, "i64 "+m[1]+")") {
		t.Fatalf("the callback takes the payload word %s itself:\n%s", m[1], ir)
	}
}

// TestAcuteProtocolReduceCountsThePasses: reduce reads an ordinal to take
// the first element as its accumulator rather than as a folded step, and
// a handle offers none — so the walk keeps a count of its own, opened at
// zero before the head, read there, and advanced in the step block both
// forms share.
func TestAcuteProtocolReduceCountsThePasses(t *testing.T) {
	ir := emitAcuteIter(t, "    let n = r.iterator().reduce(|acc, x| acc + x)\n")
	head := walkLabel(t, ir, "chead")
	m := regexp.MustCompile(`store i64 0, ptr (%v\d+)\n\s+br label %` + head).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the pass counter opens at zero before the head:\n%s", ir)
	}
	counter := m[1]
	if !regexp.MustCompile(regexp.QuoteMeta(head) + `:\n\s+%v\d+ = load i64, ptr ` + regexp.QuoteMeta(counter)).MatchString(ir) {
		t.Fatalf("the head reads the pass count %s:\n%s", counter, ir)
	}
	if !regexp.MustCompile(`cbody\d+:\n\s+%v\d+ = load i64, ptr %v\d+\n\s+%v\d+ = icmp eq i64 %v\d+, 0\n\s+br i1 %v\d+, label %rfirst\d+, label %rlater\d+`).MatchString(ir) {
		t.Fatalf("the first pass is the accumulator, the rest are steps:\n%s", ir)
	}
	step := blockAt(ir, walkLabel(t, ir, "cstep"))
	if step == "" {
		t.Fatalf("missing the walk's step:\n%s", ir)
	}
	order(t, step, "= add i64", "store i64", ", ptr "+counter, "br label %"+head)
}

// TestAcuteProtocolShortCircuitsToTheWalkExit: the predicate decides, and
// the element that decides it ends the walk — the hit block branches to
// the very exit the head's own tag test branches to, so the two sources
// share one way out and a short circuit here leaves no second exit a
// later reader could mistake for the answer's.
func TestAcuteProtocolShortCircuitsToTheWalkExit(t *testing.T) {
	ir := emitAcuteIter(t, "    let b = r.iterator().any(|x| x > 1)\n")
	m := regexp.MustCompile(`br i1 %v\d+, label %(cbody\d+), label %(cexit\d+)`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the head tests the tag against its own body and exit:\n%s", ir)
	}
	hit := blockAt(ir, walkLabel(t, ir, "qhit"))
	if hit == "" {
		t.Fatalf("the predicate's hit is a block of its own:\n%s", ir)
	}
	if !strings.Contains(hit, "store i64 1") || !strings.Contains(hit, "br label %"+m[2]) {
		t.Fatalf("the hit answers true and leaves by %s:\n%s", m[2], hit)
	}
	if m[1][len("cbody"):] != m[2][len("cexit"):] {
		t.Fatalf("the body and exit belong to one walk: %s, %s", m[1], m[2])
	}
}

// TestAcuteProtocolStopsOnAStringPayload: a String element is two words —
// its data and its length — and a pass has one, so no callback here could
// take it. count reads no element at all and could still answer, and it
// stops too: the element face is the source's, fixed before the
// combinator is chosen. That is the carrier face's own reading of a
// String element — `List<String>` stops under the same source — so the
// two sources of this walk agree on what they accept, which is the point.
func TestAcuteProtocolStopsOnAStringPayload(t *testing.T) {
	const src = `import std.io

pub type AppError = Failed(String)

record WordIter { i: Int64 }

impl Iterator<String> for WordIter {
    pub fn next(mut self) -> Option<String> { return None }
}

record Source { n: Int64 }

impl Iterable<String> for Source {
    type Iter = WordIter
    pub fn iterator(self) -> WordIter { return WordIter { i: 0 } }
}

pub fn main() effect io -> Result<(), AppError> {
    let s = Source { n: 0 }
%s    return Ok(())
}
`
	for _, c := range []struct{ what, body string }{
		{"count", "    let n = s.iterator().count()\n"},
		// The accumulator is the Int64 the callback declares, so the body
		// type-checks with a String element — the checker admits this
		// source either way, and what stops it is the emission.
		{"fold", "    let n = s.iterator().fold(0, |acc, x| acc)\n"},
	} {
		f, sh := checkShapes(t, "main.we", fmt.Sprintf(src, c.body))
		_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
		if ni == nil || ni.What != bndMainBody {
			t.Fatalf("%s over a String handle: want %q, got %+v", c.what, bndMainBody, ni)
		}
	}
}

// TestAcuteOverACallSourceStops names a gap rather than a decision: a
// source's record is read from the forms the layouts name — a binding, a
// field chain, a construction — so a CALL's result is no head this build
// can key on and the combinator stops where it stopped before, which is
// the boundary a for statement over that same source takes (T7-2). The
// checker admits the source either way (E0901 is satisfied by the
// callee's return), so the boundary is the emission's alone.
func TestAcuteOverACallSourceStops(t *testing.T) {
	const src = `import std.io

pub type AppError = Failed(String)

record CountIter { i: Int64 }

impl Iterator<Int64> for CountIter {
    pub fn next(mut self) -> Option<Int64> { return None }
}

record Range2 { n: Int64 }

impl Iterable<Int64> for Range2 {
    type Iter = CountIter
    pub fn iterator(self) -> CountIter { return CountIter { i: 0 } }
}

fn make() -> Range2 { return Range2 { n: 0 } }

pub fn main() effect io -> Result<(), AppError> {
    let n = make().iterator().count()
    return Ok(())
}
`
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni == nil {
		t.Fatal("a call's record result is no head the emission names")
	}
	if !strings.Contains(ni.What, "statement set") {
		t.Fatalf("want the body boundary word, got %q", ni.What)
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

// TestAcuteCollectTakesNoArgument: the call form is `collect()` — a
// with-argument spelling is no form this family recognises, so it stops
// on the body word rather than being quietly reinterpreted.
func TestAcuteCollectTakesNoArgument(t *testing.T) {
	_, ni := Emit(listModule(nil,
		listBind("xs", intLit("1")),
		letBind("n", acute(ident("xs"), "collect", intLit("1"))),
		okReturn(),
	), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}
