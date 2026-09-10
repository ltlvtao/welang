package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T2 statement set (design D1/D6): the for statement over a Range. Red
// before it: `for i in 0..3` stopped at the body's boundary word (chapter
// 5's iteration clause is check-green since M3; codegen never emitted the
// form).
//
// The emission is a counted loop rather than the materialized array design
// D6 first named: the two are observationally identical for a Range — the
// bounds are evaluated once in source order, iteration runs unit steps to
// the end, and nothing in the ratified surface observes the allocation —
// while the counted loop needs neither the collection carrier (T7's
// `__we_list_*`) nor a heap. The bounds are read into registers ahead of
// the head, so an assignment to a bound inside the body cannot change the
// iteration count, and the counter lives in its own slot, so the loop
// variable is bound afresh from it each pass — an assignment to the
// variable cannot corrupt the sequence (chapter 5: each Some(element)
// executes the body once with the element bound under the name rules).

// forRange builds `for name in lo..hi { body }`.
func forRange(name string, lo, hi ast.Expr, body ...ast.Stmt) *ast.ForStmt {
	return &ast.ForStmt{
		Pat:  &ast.PatBinding{Name: name},
		Iter: binOp("..", lo, hi),
		Body: ast.Block{Items: body},
	}
}

// blockAt slices one basic block out of the IR: from its label line to the
// next label line. Empty means the label is absent.
func blockAt(ir, label string) string {
	lines := strings.Split(ir, "\n")
	start := -1
	for i, ln := range lines {
		if ln == label+":" {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasSuffix(lines[i], ":") && !strings.HasPrefix(lines[i], " ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// TestForRangeCounts: the head tests the counter against the end bound,
// the body runs, the continuation block steps the counter and branches
// back — break's exit and continue's step are their own labels.
func TestForRangeCounts(t *testing.T) {
	ir := assertClean(t, m9bModule(
		forRange("i", intLit("0"), intLit("3"), ioCall("io", "println", ident("i"))),
		okReturn(),
	), "forhead0", "forbody0", "forcont0", "forexit0")
	order(t, ir,
		"store i64 0, ptr", // the counter takes the start bound
		"br label %forhead0",
		"load i64, ptr", // the head re-reads the counter
		"icmp slt i64",  // against the end bound
		"br i1",         // body or exit
		"br label %forcont0",
		"store i64", // the step writes the counter back
		"br label %forhead0",
	)
	if got := strings.Count(ir, "br label %forhead0"); got != 2 {
		t.Fatalf("want the entry branch and the back edge, got %d:\n%s", got, ir)
	}
}

// TestForRangeEvaluatesBoundsOnce: the end bound is read before the loop
// and the head compares the register it landed in — a fresh read per
// iteration would let an assignment to the bound inside the body cut the
// sequence short (the bound is a slot here, so a re-read would show as a
// second load in the head).
func TestForRangeEvaluatesBoundsOnce(t *testing.T) {
	ir := assertClean(t, m9bModule(
		typedLit("b", "3"),
		forRange("i", intLit("0"), ident("b"), assignTo("b", intLit("1"))),
		okReturn(),
	), "forhead0")
	head := blockAt(ir, "forhead0")
	if head == "" {
		t.Fatalf("missing the loop head:\n%s", ir)
	}
	if got := strings.Count(head, "load i64, ptr"); got != 1 {
		t.Fatalf("the head reads the counter once (the bound is a register), got %d loads:\n%s", got, head)
	}
	if !strings.Contains(head, "icmp slt i64") {
		t.Fatalf("the head tests the counter against the bound:\n%s", head)
	}
}

// TestForRangeFreshBindingPerIteration: the loop variable is a binding of
// each element, not the counter itself — the body's assignment writes its
// own slot and the step still reads the counter's.
func TestForRangeFreshBindingPerIteration(t *testing.T) {
	ir := assertClean(t, m9bModule(
		forRange("i", intLit("0"), intLit("3"),
			assignTo("i", binOp("+", ident("i"), intLit("5"))),
			ioCall("io", "println", ident("i")),
		),
		okReturn(),
	), "forhead0", "forcont0")
	if got := strings.Count(ir, "alloca i64"); got != 2 {
		t.Fatalf("want the counter's slot and the assigned variable's, got %d:\n%s", got, ir)
	}
	// The step loads the counter, adds one, and writes it back — it never
	// reads the variable the body assigned.
	cont := blockAt(ir, "forcont0")
	if cont == "" {
		t.Fatalf("missing the step block:\n%s", ir)
	}
	if !strings.Contains(cont, "load i64, ptr") || !strings.Contains(cont, "store i64") {
		t.Fatalf("the step reads and rewrites the counter:\n%s", cont)
	}
}

// TestForRangeContinueSteps: continue lands on the step block, not the
// head — a continue that skipped the step would spin forever.
func TestForRangeContinueSteps(t *testing.T) {
	ir := assertClean(t, m9bModule(
		forRange("i", intLit("0"), intLit("3"),
			&ast.ExprStmt{Expr: &ast.If{
				Cond: binOp("==", ident("i"), intLit("1")),
				Then: ast.Block{Items: []ast.Stmt{&ast.Continue{}}},
			}},
			ioCall("io", "println", ident("i")),
		),
		okReturn(),
	), "forcont0", "forexit0")
	if got := strings.Count(ir, "br label %forcont0"); got != 2 {
		t.Fatalf("want the body's fallthrough and the continue, got %d:\n%s", got, ir)
	}
	if strings.Contains(ir, "unreachable\n  br") {
		t.Fatalf("an instruction lands after a terminator:\n%s", ir)
	}
}

// TestForRangeBreakLeaves: break branches to the loop's exit label, and
// the exit opens for what follows.
func TestForRangeBreakLeaves(t *testing.T) {
	ir := assertClean(t, m9bModule(
		forRange("i", intLit("0"), intLit("3"), &ast.Break{}),
		okReturn(),
	), "forexit0")
	order(t, ir, "br label %forexit0", "forexit0:", "ret i32 0")
}

// TestForRangeWildcardPattern: `for _ in …` binds nothing — the counter is
// still there, and no loop-variable slot is taken.
func TestForRangeWildcardPattern(t *testing.T) {
	st := forRange("_", intLit("0"), intLit("3"), ioCall("io", "println", strLit(`"tick"`)))
	st.Pat = &ast.PatWildcard{}
	ir := assertClean(t, m9bModule(st, okReturn()), "forhead0", "forbody0")
	if got := strings.Count(ir, "alloca i64"); got != 1 {
		t.Fatalf("want the counter's slot alone, got %d:\n%s", got, ir)
	}
	if strings.Contains(blockAt(ir, "forbody0"), "store i64") {
		t.Fatalf("a wildcard head binds nothing to store:\n%s", ir)
	}
}

// TestForNonRangeBnd is the boundary pin: a source outside the emitted
// set stops at the body boundary. The String source was this pin until
// T4-3 walked its code points; the List source needs its carrier (T7), so
// it holds the pin now. Re-anchored as that lands.
func TestForNonRangeBnd(t *testing.T) {
	_, ni := Emit(m9bModule(
		&ast.ForStmt{
			Pat:  &ast.PatBinding{Name: "x"},
			Iter: &ast.Construct{Name: "List", TypeArgs: []ast.TypeRef{named("Int64")}, Fields: []ast.FieldInit{{Name: "items", Value: strLit(`"abc"`)}}},
			Body: ast.Block{Items: []ast.Stmt{ioCall("io", "println", ident("x"))}},
		},
		okReturn(),
	), "demo")
	if ni == nil {
		t.Fatalf("a List source is outside this build's set")
	}
	if !strings.Contains(ni.What, "statement set") {
		t.Fatalf("want the body boundary word, got %q", ni.What)
	}
}

// --- T4-3: the String source ------------------------------------------------

// forString builds `for name in iter { body }` over a non-Range source.
func forString(name string, iter ast.Expr, body ...ast.Stmt) *ast.ForStmt {
	return &ast.ForStmt{
		Pat:  &ast.PatBinding{Name: name},
		Iter: iter,
		Body: ast.Block{Items: body},
	}
}

// TestForStringSourceWalksRunes: a String source iterates its code points.
// The value is read once — its pair is a register pair for the whole loop —
// and the walk is the chapter 17 pair: runeCount fixes the count, and each
// pass takes one code point by index. The index space is code points, so a
// multi-byte value is not walked byte by byte.
func TestForStringSourceWalksRunes(t *testing.T) {
	ir := assertClean(t, m9bModule(
		// Six bytes, five code points: the walk counts code points.
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"héllo"`)},
		forString("c", ident("s"), ioCall("io", "println", ident("c"))),
		okReturn(),
	), "declare i64 @__we_str_runecount(ptr, i64)",
		"declare i64 @__we_str_charat(ptr, i64, i64)",
		"= call i64 @__we_str_runecount(ptr @.s0, i64 6)",
		"= call i64 @__we_str_charat(ptr @.s0, i64 6, i64",
	)
	// The count is read once, ahead of the head; each pass takes one code
	// point, so the loop body carries exactly one charAt per iteration.
	if got := countCall(ir, "i64", "__we_str_runecount"); got != 1 {
		t.Fatalf("the count is read once, got %d:\n%s", got, ir)
	}
	if got := countCall(ir, "i64", "__we_str_charat"); got != 1 {
		t.Fatalf("one charAt per pass, got %d:\n%s", got, ir)
	}
	// The head tests the index against the count, not against a length.
	head := blockAt(ir, "forhead0")
	if !strings.Contains(head, "icmp slt i64") {
		t.Fatalf("the head counts down the rune index:\n%s", ir)
	}
	// The source's words reach the runtime as registers, never re-evaluated
	// inside the loop.
	if body := blockAt(ir, "forbody0"); strings.Contains(body, "__we_str_runecount") {
		t.Fatalf("the source is read once, ahead of the loop:\n%s", ir)
	}
}

// TestForStringSourceBindsRunes: the loop variable is a Rune — the element
// domain the String source yields — so an interpolated hole over it takes
// the code-point renderer rather than the integer one.
func TestForStringSourceBindsRunes(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"ab"`)},
		forString("c", ident("s"),
			ioCall("io", "println", interpLit([]string{"[", "]"}, ident("c")))),
		okReturn(),
	), "= call %struct.we_str @__we_str_of_rune(i64 ")
}

// TestForStringSourceStepsOnce: the continuation block advances the index
// and branches back, and continue lands on it rather than on the head — a
// continue that skipped the step would spin on one code point forever.
func TestForStringSourceStepsOnce(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		forString("c", ident("s"),
			&ast.ExprStmt{Expr: &ast.If{Cond: ident("c"), Then: ast.Block{Items: []ast.Stmt{&ast.Continue{}}}}}),
		okReturn(),
	), "forcont0")
	cont := blockAt(ir, "forcont0")
	if !strings.Contains(cont, "add i64") || !strings.Contains(cont, "store i64") {
		t.Fatalf("the step advances the index:\n%s", ir)
	}
	if !strings.Contains(cont, "br label %forhead0") {
		t.Fatalf("the step branches back to the head:\n%s", ir)
	}
}

// TestForStringBreakLeavesTheWalk: break's exit is the loop's own, so a
// break inside the walk lands after it.
func TestForStringBreakLeavesTheWalk(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		forString("c", ident("s"),
			&ast.ExprStmt{Expr: &ast.If{Cond: ident("c"), Then: ast.Block{Items: []ast.Stmt{&ast.Break{}}}}}),
		ioCall("io", "println", strLit(`"after"`)),
		okReturn(),
	), "forexit0")
	// The arm's block carries the edge out (the if's join is the body's
	// own fallthrough), and the exit runs what follows the walk.
	if !strings.Contains(ir, "br label %forexit0") {
		t.Fatalf("break leaves the walk:\n%s", ir)
	}
	if exit := blockAt(ir, "forexit0"); !strings.Contains(exit, "call void") {
		t.Fatalf("the exit runs what follows the walk:\n%s", ir)
	}
}
