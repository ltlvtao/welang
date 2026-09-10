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

// TestForNonRangeBnd is the boundary pin: a non-range source is not
// emitted yet — the String source needs the runtime's rune walk (T4) and
// List its carrier (T7). Re-anchored as those land.
func TestForNonRangeBnd(t *testing.T) {
	_, ni := Emit(m9bModule(
		&ast.ForStmt{Pat: &ast.PatBinding{Name: "c"}, Iter: strLit(`"abc"`), Body: ast.Block{Items: []ast.Stmt{ioCall("io", "println", ident("c"))}}},
		okReturn(),
	), "demo")
	if ni == nil {
		t.Fatalf("a String source is outside this build's set")
	}
	if !strings.Contains(ni.What, "statement set") {
		t.Fatalf("want the body boundary word, got %q", ni.What)
	}
}
