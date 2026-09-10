package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T7-2 Lists (design D6, chapters 11 and 17). The carrier is one frozen
// object — {len, cap, traced} ahead of the elements — and the emitter's
// spellings for it are:
//
//   - a literal opens the carrier with its element count as the capacity
//     and pushes each element as the one word its face occupies;
//   - the element face is either a scalar domain (the word IS the value;
//     a Float64 rides as its bit pattern) or a gc record (the word is the
//     handle, and the carrier's trace bit says so);
//   - a for over a List takes the carrier's snapshot before the head
//     opens and walks it by index, the head pattern binding the word the
//     element read answers;
//   - everything the one-word element cannot hold — a String's two words,
//     a sum's two, a tuple's several, a nested collection — stops at the
//     body boundary instead of storing a truncation of itself.
//
// The unit pins below are the emitted shape (the trace bit and the walk's
// order); the behaviour is the conformance goldens' and the carrier's own
// trace is list_test.go's in runtime/.

// listModule wraps main-shaped statements into the root build module,
// closing the body with the tail return every main owes.
func listModule(decls []ast.Item, stmts ...ast.Stmt) *ast.File {
	items := []ast.Item{stdIoImport("io"), appError()}
	items = append(items, decls...)
	return &ast.File{Items: append(items, mainDecl(append(stmts, okReturn())...))}
}

// listOf binds `let name: List<E> = [elems…]` where the annotation names
// the element type.
func listOf(name, elem string, elems ...ast.Expr) *ast.Binding {
	return &ast.Binding{
		Kw: "let", Name: name,
		Typ:  &ast.NamedType{Name: "List", Args: []ast.TypeRef{&ast.NamedType{Name: elem}}},
		Init: &ast.ListLit{Elems: elems},
	}
}

// walk binds `for x in src { body… }`.
func walk(x string, src ast.Expr, body ...ast.Stmt) *ast.ForStmt {
	return &ast.ForStmt{Pat: &ast.PatBinding{Name: x}, Iter: src, Body: ast.Block{Items: body}}
}

// TestListLiteralOpensTheCarrierAndPushesEachElement: the literal's
// capacity IS its element count, so the pushes form one chain of
// identities and the value is its last register; a scalar element carries
// no trace bit — saying an integer's bits are a reference would send the
// collector after a word that addresses nothing.
func TestListLiteralOpensTheCarrierAndPushesEachElement(t *testing.T) {
	ir := assertClean(t, listModule(nil, letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1"), intLit("2"), intLit("3")}})),
		"@__we_list_new(i64 3, i64 0)",
		"call void @__we_root_push(ptr %v0)")
	if !regexp.MustCompile(`%v\d+ = call ptr @__we_list_push\(ptr %v0, i64 1\)\n\s+%v\d+ = call ptr @__we_list_push\(ptr %v\d+, i64 2\)`).MatchString(ir) {
		t.Fatalf("the pushes chain from the creation register:\n%s", ir)
	}
}

// TestListRecordElementsAreTraced: a record element's word is its handle,
// so the carrier opens traced and each element crosses as its address.
// The trace bit is the only thing that keeps the elements alive when the
// walk reads them back after a collection.
func TestListRecordElementsAreTraced(t *testing.T) {
	ir := assertClean(t, listModule(
		[]ast.Item{recDecl("Cell", "gc", fld("n", "Int64"))},
		letBind("cs", &ast.ListLit{Elems: []ast.Expr{
			construct("Cell", init1("n", intLit("1"))),
			construct("Cell", init1("n", intLit("2"))),
		}}),
	), "@__we_list_new(i64 2, i64 1)")
	if !regexp.MustCompile(`%v\d+ = ptrtoint ptr %v\d+ to i64\n\s+%v\d+ = call ptr @__we_list_push`).MatchString(ir) {
		t.Fatalf("a gc element crosses as its address:\n%s", ir)
	}
}

// TestListEmptyLiteralTakesItsAnnotation: `[]` determines no element type
// (chapter 17's E1501), so the annotation is the whole source of the
// face — and the emission opens a zero-capacity carrier, which the walk
// then leaves empty.
func TestListEmptyLiteralTakesItsAnnotation(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		listOf("xs", "Int64"),
		walk("x", ident("xs")),
	), "@__we_list_new(i64 0, i64 0)")
	if !strings.Contains(ir, "@__we_list_snap") {
		t.Fatalf("the walk snapshots even an empty carrier:\n%s", ir)
	}
}

// TestListWalkSnapshotsBeforeTheHead: chapter 17 fixes the iterated
// sequence at the moment the iterator is taken, so the copy is made once,
// before the head — and the copy is a gc object the walk owes a root, or
// the first element allocation the body makes could sweep it.
func TestListWalkSnapshotsBeforeTheHead(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		listBind("xs", intLit("1"), intLit("2")),
		walk("x", ident("xs")),
	))
	snap := strings.Index(ir, "= call ptr @__we_list_snap")
	head := strings.Index(ir, "forhead0:")
	if snap < 0 || head < 0 || snap > head {
		t.Fatalf("the snapshot is taken before the head opens (snap %d, head %d):\n%s", snap, head, ir)
	}
	m := regexp.MustCompile(`(%v\d+) = call ptr @__we_list_snap\(ptr (%v\d+)\)\n\s+call void @__we_root_push\(ptr (%v\d+)\)\n\s+%v\d+ = call i64 @__we_list_len`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the snapshot is made and rooted before its length is read:\n%s", ir)
	}
	snapReg, srcReg := m[1], m[2]
	if m[3] != snapReg {
		t.Fatalf("the root pushed is the snapshot %s, not %s:\n%s", snapReg, m[3], ir)
	}
	// The walk reads one element per pass, in the body, on the pass's
	// index — and it reads the SNAPSHOT, not the live carrier: reading the
	// source would be the same register shape while iterating a sequence
	// chapter 17 did not fix.
	if !regexp.MustCompile(`forbody0:\n\s+%v\d+ = call i64 @__we_list_get\(ptr ` + regexp.QuoteMeta(snapReg) + `, i64 %v\d+\)`).MatchString(ir) {
		t.Fatalf("the element read runs in the body on the counter, over the snapshot %s (source %s):\n%s", snapReg, srcReg, ir)
	}
}

// TestListWalkBindsGcHeadByReference: a record element's binding names the
// record the carrier holds, so a field read through the head lands on the
// record's own offset — no copy, because a gc record IS a reference.
func TestListWalkBindsGcHeadByReference(t *testing.T) {
	ir := assertClean(t, listModule(
		[]ast.Item{recDecl("Cell", "gc", fld("n", "Int64"))},
		letBind("cs", &ast.ListLit{Elems: []ast.Expr{construct("Cell", init1("n", intLit("5")))}}),
		varLit("total", "0"),
		walk("c", ident("cs"), assignTo("total", binOp("+", ident("total"), memberOf(ident("c"), "n")))),
	))
	if !regexp.MustCompile(`%v\d+ = inttoptr i64 %v\d+ to ptr\n\s+%v\d+ = load i64, ptr %v\d+\n\s+%v\d+ = getelementptr i8, ptr %v\d+, i64 16`).MatchString(ir) {
		t.Fatalf("the head converts the element word back to a pointer and the field read lands on the record's own offset:\n%s", ir)
	}
}

// TestListWalkCopiesValueRecordElements: a byval record is copied on
// every binding (chapter 8), and the head pattern is a binding — so the
// loop copies each element rather than handing the body a share of the
// carrier's object.
func TestListWalkCopiesValueRecordElements(t *testing.T) {
	ir := assertClean(t, listModule(
		[]ast.Item{recDecl("Val", "value", fld("n", "Int64"))},
		letBind("vs", &ast.ListLit{Elems: []ast.Expr{construct("Val", init1("n", intLit("5")))}}),
		varLit("total", "0"),
		walk("v", ident("vs"), assignTo("total", binOp("+", ident("total"), memberOf(ident("v"), "n")))),
	), "@__we_list_new(i64 1, i64 1)")
	if !regexp.MustCompile(`forbody0:\n\s+%v\d+ = call i64 @__we_list_get\(ptr %v\d+, i64 %v\d+\)\n\s+%v\d+ = inttoptr i64 %v\d+ to ptr\n\s+%v\d+ = call ptr @__we_alloc`).MatchString(ir) {
		t.Fatalf("the head copies the element it read:\n%s", ir)
	}
}

// TestListFloatElementsRideAsBitPatterns: the carrier holds one word per
// element whatever the face, so a Float64 crosses as its bit pattern —
// converted at the push and back at the head — and never as a trivially
// converted number.
func TestListFloatElementsRideAsBitPatterns(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		listBind("fs", floatLit("1.5"), floatLit("2.5")),
		walk("f", ident("fs")),
	))
	if !regexp.MustCompile(`%v\d+ = bitcast double \S+ to i64\n\s+%v\d+ = call ptr @__we_list_push`).MatchString(ir) {
		t.Fatalf("a float element is pushed as its bit pattern:\n%s", ir)
	}
	if !regexp.MustCompile(`%v\d+ = bitcast i64 %v\d+ to double`).MatchString(ir) {
		t.Fatalf("the head converts the bit pattern back to the double:\n%s", ir)
	}
}

// listBind binds a literal with no annotation: the first element's own
// form fixes the face there.
func listBind(name string, elems ...ast.Expr) *ast.Binding {
	return &ast.Binding{Kw: "let", Name: name, Init: &ast.ListLit{Elems: elems}}
}

// TestListStringElementsStopAtTheBoundary: a String is a (ptr, len) pair,
// and the carrier's element slot is one word — so a List<String> has no
// face this build can store, and the literal stops rather than writing
// half of every element.
func TestListStringElementsStopAtTheBoundary(t *testing.T) {
	_, ni := Emit(listModule(nil, listOf("xs", "String", strLit("a"), strLit("b"))), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}

// TestListSumElementsStopAtTheBoundary: a sum is two words (tag and
// payload) for the same reason.
func TestListSumElementsStopAtTheBoundary(t *testing.T) {
	_, ni := Emit(listModule(nil, listOf("xs", "Option<Int64>", intLit("1"))), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}

// TestListCrossingAFnBoundaryStops: the carrier is one pointer, but its
// element face is a fact no ABI carries yet — so a List parameter or
// argument stops at the boundary rather than letting the callee walk a
// carrier whose word width it would have to guess.
func TestListCrossingAFnBoundaryStops(t *testing.T) {
	mod := ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		pubFn("tally", []ast.Param{{Name: "xs", Type: &ast.NamedType{Name: "List", Args: []ast.TypeRef{named("Int64")}}}}, named("Int64"),
			retValue(intLit("0"))),
		mainDecl(letDiscard(callOn(ident("tally"), "call", intLit("1"))), okReturn()),
	}}}
	if _, ni := EmitProgram(ModeBuild, []ProgModule{mod}); ni == nil {
		t.Fatal("a List parameter is not this build's ABI")
	}
}

// TestListLiteralOutsideABindingStops: the widening is scoped to the two
// positions that need it — a binding and a for's source. A literal
// anywhere else (here: dropped as a statement) stops at the body
// boundary, so the acceptance set is what the tests say it is.
func TestListLiteralOutsideABindingStops(t *testing.T) {
	_, ni := Emit(listModule(nil, &ast.ExprStmt{Expr: &ast.ListLit{Elems: []ast.Expr{intLit("1")}}}), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}

// TestListWalkOverAUserIterableStops: the three builtin sources are this
// build's; a user impl of Iterable — whose protocol runs through the
// interface's own machinery — stops at the body word.
func TestListWalkOverAUserIterableStops(t *testing.T) {
	_, ni := Emit(listModule(
		[]ast.Item{recDecl("Range2", "gc", fld("n", "Int64"))},
		letBind("r", construct("Range2", init1("n", intLit("0")))),
		walk("x", ident("r")),
	), "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("want %q, got %+v", bndMainBody, ni)
	}
}
