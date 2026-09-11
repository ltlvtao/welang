package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T8-2a root discharge per body (design D7's root protocol; the design's
// own wording is "根推送按 body 记账（e.pushes 在每个 body 出口弹）" — the
// accounting is per body and every body's exit pops).
//
// A body is chapter 2's block, not just a function's: an if's arm, a
// match's arm, a loop's body and a bare block are all bodies, and each is
// emitted ONCE while running many times (or not at all). The counter
// `e.pushes` is a compile-time fact about one emission of a body, so the
// discharge has to be emitted where the body's runtime path ends, and
// every edge that leaves a body early — break, continue — carries its
// own.
//
// The defect this face repairs was found while probing T8-2B, and both
// halves of it are visible only against the runtime:
//
//   - a loop's body is emitted once and runs per pass, so its pushes
//     accumulate in the task's root window without bound — a program that
//     allocates inside a loop pins every allocation it ever made, and the
//     collector's sweep reclaims nothing at all (measured: 100k passes
//     left 283998 live root slots and `swept=0` at every collection);
//   - a module init's body never popped, so every top-level handle stayed
//     rooted for the process's life — which is also what made T8-2B's
//     negative control unobservable.
//
// One face of the defect is NOT pinned here: `@<key>.init`'s own body.
// At this point in the milestone no top-level binding is a collectable
// handle — a scalar is a word, a String is a malloc'd buffer outside the
// gc domain (D3) — so an init body roots nothing and a pin on its
// discharge would be vacuous. The pin travels with T8-2B, whose gc
// top-level bindings are what put roots in an init body at all.
//
// The pins below are the emitted shape; the runtime consequence is the
// change record's real-machine evidence and the run goldens'.

// bodyTail is the block the IR's last occurrence of the given label opens:
// the discharge sits at the end of a body, so the assertion reads from the
// body's label to whatever follows it.
func bodyTail(ir, label string) string {
	// The label carries the emitter's block ordinal (`forbody0`), and the
	// same stem appears in the branches that name it, so the definition is
	// the line whose whole text is the label.
	loc := regexp.MustCompile(`\n` + label + `[0-9]*:\n`).FindStringIndex(ir)
	if loc == nil {
		return ""
	}
	return ir[loc[0]:]
}

// popsBefore pins that the line carrying the needle arrives with exactly
// n root pops immediately before it: adjacent, in that order, with no
// instruction between them, and starting at that line's own first
// character so a pop belonging to an earlier statement is not counted.
//
// Exactly, because the window is a stack: a discharge one pop long would
// take a root the body never pushed, one that stops short would leave the
// body's own behind, and a loose match would call both of those the pinned
// shape — n pops span n-1 pops plus the line.
func popsBefore(t *testing.T, ir, needle string, n int) {
	t.Helper()
	if n < 1 {
		t.Fatalf("a discharge of %d roots is no discharge at all — the pin would be vacuous", n)
	}
	i := strings.Index(ir, needle)
	if i < 0 {
		t.Fatalf("IR missing %q:\n%s", needle, ir)
	}
	i = strings.LastIndex(ir[:i], "\n") + 1
	pop := "  call void @__we_root_pop()\n"
	got := 0
	for j := i - len(pop); j >= 0 && strings.HasPrefix(ir[j:], pop); j -= len(pop) {
		got++
	}
	if got != n {
		t.Fatalf("the line carrying %q owes %d root pops, the emission carries %d:\n%s", needle, n, got, ir)
	}
}

// noPopsBefore pins that the line arrives with no discharge immediately
// before it: the body owed nothing there. A counter left standing after a
// body already paid would pay a second time — at the function's exit, for
// roots it never pushed.
func noPopsBefore(t *testing.T, ir, line string) {
	t.Helper()
	i := strings.Index(ir, line)
	if i < 0 {
		t.Fatalf("IR missing %q:\n%s", line, ir)
	}
	if strings.HasSuffix(ir[:i], "  call void @__we_root_pop()\n") {
		t.Fatalf("%q is preceded by a root pop the body did not owe:\n%s", line, ir)
	}
}

// TestWhileBodyDischargesEachPassesRoots: the list literal in the body
// pushes its carrier's roots once per pass, so the pass has to pop them
// before branching back to the head — otherwise the window grows by two
// slots for every iteration the program ever runs.
func TestWhileBodyDischargesEachPassesRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("i", intLit("0")),
		letBind("c", boolLit("true")),
		whileShape(binOp("<", ident("i"), intLit("3")),
			letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1"), intLit("2"), intLit("3")}}),
		),
		okReturn(),
	), "br label %whhead")
	body := bodyTail(ir, "whbody")
	if strings.Count(body, "__we_root_push") == 0 {
		t.Fatalf("the body pushes nothing — the pin would be vacuous:\n%s", body)
	}
	popsBefore(t, body, "br label %whhead", strings.Count(body[:strings.Index(body, "br label %whhead")], "__we_root_push"))
}

// TestLoopBodyDischargesEachPassesRoots: `loop` branches back to its body
// label, and the discharge lands the same way.
func TestLoopBodyDischargesEachPassesRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		&ast.Loop{Body: ast.Block{Items: []ast.Stmt{
			letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1")}}),
			&ast.ExprStmt{Expr: &ast.If{Cond: binOp("==", intLit("1"), intLit("1")), Then: ast.Block{Items: []ast.Stmt{&ast.Break{}}}}},
		}}},
		okReturn(),
	), "lpexit")
	body := bodyTail(ir, "lpbody")
	popsBefore(t, body, "br label %lpbody", strings.Count(body[:strings.Index(body, "br label %lpbody")], "__we_root_push"))
}

// TestForListBodyDischargesEachPassesRoots: a for over a List takes the
// carrier's snapshot before the head and roots it; that push happens once,
// outside the body, and stays live across every back edge — so the body's
// own discharge is its own pushes, and the snapshot's root is still there
// when the loop exits (it is popped with the rest of the enclosing body's).
func TestForListBodyDischargesEachPassesRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("src", &ast.ListLit{Elems: []ast.Expr{intLit("1"), intLit("2")}}),
		walk("x", ident("src"),
			letBind("ys", &ast.ListLit{Elems: []ast.Expr{intLit("7")}}),
		),
		okReturn(),
	), "forexit")
	body := bodyTail(ir, "forbody")
	seg := body[:strings.Index(body, "  br label %forcont")]
	if n := strings.Count(seg, "__we_root_push"); n == 0 {
		t.Fatalf("the body pushes nothing — the pin would be vacuous:\n%s", body)
	} else {
		popsBefore(t, body, "br label %forcont", n)
	}
}

// TestIfArmDischargesItsRoots: an arm is a body. Whichever arm runs, the
// join has to see the window depth the if opened with — otherwise the two
// edges into the join disagree, and every later pop takes someone else's
// slot.
func TestIfArmDischargesItsRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("c", boolLit("true")),
		&ast.ExprStmt{Expr: &ast.If{
			Cond: ident("c"),
			Then: ast.Block{Items: []ast.Stmt{letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1")}})}},
			Else: blockOf(letBind("ys", &ast.ListLit{Elems: []ast.Expr{intLit("2")}})),
		}},
		okReturn(),
	), "ifjoin")
	then := bodyTail(ir, "ifthen")
	popsBefore(t, then, "br label %ifjoin", strings.Count(then[:strings.Index(then, "br label %ifjoin")], "__we_root_push"))
	elseB := bodyTail(ir, "ifelse")
	popsBefore(t, elseB, "br label %ifjoin", strings.Count(elseB[:strings.Index(elseB, "br label %ifjoin")], "__we_root_push"))
}

// TestNestedBodyDischargesOnlyItsOwnRoots: a body inside a body owns its
// own pushes and nothing else. The arm's discharge has already paid for
// the arm by the time the loop body's end runs, so the loop body's count
// is its own — counting the arm's again would pop below the depth the
// body opened at, and the enclosing body's roots would go with it.
func TestNestedBodyDischargesOnlyItsOwnRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("i", intLit("0")),
		letBind("c", boolLit("true")),
		whileShape(binOp("<", ident("i"), intLit("3")),
			letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1")}}),
			&ast.ExprStmt{Expr: &ast.If{Cond: ident("c"), Then: ast.Block{Items: []ast.Stmt{
				letBind("ys", &ast.ListLit{Elems: []ast.Expr{intLit("2")}}),
			}}}},
		),
		okReturn(),
	), "br label %whhead")
	body := bodyTail(ir, "whbody")
	// The body's own: the literal's two pushes. The arm's two were paid at
	// the arm's own edge.
	popsBefore(t, body, "br label %whhead", 2)
	noPopsBefore(t, ir, "  ret i32 0\n")
}

// TestBreakLeavesTheEnclosingBodysRoots: breaking out of a loop leaves the
// window at the depth the loop body opened at — not at the function's
// zero. A root the enclosing body pushed before the loop is still read
// after it, so a discharge that reached further would hand the collector
// a live object.
func TestBreakLeavesTheEnclosingBodysRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("keep", &ast.ListLit{Elems: []ast.Expr{intLit("9")}}),
		letBind("c", boolLit("true")),
		whileShape(ident("c"),
			letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1")}}),
			&ast.Break{},
		),
		okReturn(),
	), "whexit")
	body := bodyTail(ir, "whbody")
	popsBefore(t, body, "br label %whexit", 2)
}

// TestValueArmDischargesItsRoots: an arm in a value form is the same body
// with a sink instead of a fall-through. Its pushes are owed in the same
// place — after the tail has computed, before the edge to the join — and
// the arm that pushed them is the only one that knows the count.
func TestValueArmDischargesItsRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("c", boolLit("true")),
		letBind("x", &ast.If{
			Cond: ident("c"),
			Then: ast.Block{Items: []ast.Stmt{
				letBind("ys", &ast.ListLit{Elems: []ast.Expr{intLit("1")}}),
				&ast.ExprStmt{Expr: intLit("3")},
			}},
			Else: blockOf(&ast.ExprStmt{Expr: intLit("4")}),
		}),
		okReturn(),
	), "ifjoin")
	then := bodyTail(ir, "ifthen")
	if n := strings.Count(then[:strings.Index(then, "br label %ifjoin")], "__we_root_push"); n == 0 {
		t.Fatalf("the arm pushes nothing — the pin would be vacuous:\n%s", then)
	} else {
		popsBefore(t, then, "br label %ifjoin", n)
	}
	// The main body pushed nothing of its own, so its tail owes nothing —
	// the arm paid for the arm.
	noPopsBefore(t, ir, "  ret i32 0\n")
}

// TestBareBlockDischargesItsRoots: a block that is not a body of its own
// in the source is still one in the emission — its pushes are its own, and
// it pays them where it ends, before the statement that follows runs. The
// body's tail then owes only what the body itself pushed.
func TestBareBlockDischargesItsRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		&ast.ExprStmt{Expr: blockOf(
			letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1"), intLit("2")}}),
		)},
		letBind("ys", &ast.ListLit{Elems: []ast.Expr{intLit("9")}}),
		okReturn(),
	))
	if strings.Count(ir, "__we_root_push") == 0 {
		t.Fatalf("the bare block pushes nothing — the pin would be vacuous:\n%s", ir)
	}
	// The block pays where it ends: its two pushes (the carrier, then the
	// identity the pushes leave behind) are popped immediately before the
	// statement after the block opens its own carrier — that second
	// literal is a one-element list, so the needle is its allocation and
	// nothing earlier spells the same call.
	popsBefore(t, ir, "@__we_list_new(i64 1, i64 0)", 2)
	// The body's tail then owes only its own: the second literal's two.
	popsBefore(t, ir, "ret i32 0", 2)
}

// TestBreakDischargesThePassesRoots: a break leaves the pass before the
// pass's own end, so it owes the discharge itself — the code it jumps past
// is exactly the code that would have paid.
func TestBreakDischargesThePassesRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("i", intLit("0")),
		letBind("c", boolLit("true")),
		whileShape(binOp("<", ident("i"), intLit("3")),
			&ast.ExprStmt{Expr: &ast.If{Cond: ident("c"), Then: ast.Block{Items: []ast.Stmt{
				letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1")}}),
				&ast.Break{},
			}}}},
		),
		okReturn(),
	), "whexit")
	arm := bodyTail(ir, "ifthen")
	popsBefore(t, arm, "br label %whexit", strings.Count(arm[:strings.Index(arm, "br label %whexit")], "__we_root_push"))
}

// TestContinueDischargesThePassesRoots: continue restarts the loop, so its
// edge owes the same discharge the fall-through end of the body pays.
func TestContinueDischargesThePassesRoots(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("i", intLit("0")),
		letBind("c", boolLit("true")),
		whileShape(binOp("<", ident("i"), intLit("3")),
			&ast.ExprStmt{Expr: &ast.If{Cond: ident("c"), Then: ast.Block{Items: []ast.Stmt{
				letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1")}}),
				&ast.Continue{},
			}}}},
		),
		okReturn(),
	), "whhead")
	arm := bodyTail(ir, "ifthen")
	popsBefore(t, arm, "br label %whhead", strings.Count(arm[:strings.Index(arm, "br label %whhead")], "__we_root_push"))
}

// TestReturnDischargesTheWholeLiveSet: a return leaves the function, so it
// owes every root still live on its path — the enclosing body's along with
// the arm's. Break and continue stop at the loop's depth because their far
// side is still inside the function; a return has no far side, and
// discharging only to the arm's own depth would strand the enclosing
// body's roots in the window for the caller's collections to mark.
func TestReturnDischargesTheWholeLiveSet(t *testing.T) {
	ir := assertClean(t, listModule([]ast.Item{
		pubFn("f", nil, named("Int64"),
			letBind("keep", &ast.ListLit{Elems: []ast.Expr{intLit("9")}}),
			letBind("c", boolLit("true")),
			&ast.ExprStmt{Expr: &ast.If{Cond: ident("c"), Then: ast.Block{Items: []ast.Stmt{
				letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("1")}}),
				retValue(intLit("7")),
			}}}},
			retValue(intLit("0")),
		),
	},
		letBind("n", &ast.Call{Fn: ident("f")}),
	))
	// The enclosing literal's two plus the arm's two.
	popsBefore(t, ir, "ret i64 7", 4)
}

// TestReturnFromALoopBodyDischargesTheWholeLiveSet: the same fact reached
// through the body T8-2B-0 recorded as out of reach. That record's premise
// — "the M9b statement set admits one tail return per body, so a return
// inside a loop is still a boundary" — does not hold: `for` and `while`
// bodies take a return today, in main and in a fn both (probed on the real
// toolchain at f303bb4 and at 69dbf78), and emitReturn has been at any depth
// since T2's deep returns. The loop frame is what break and continue stop
// at; a return leaves the function, so its discharge is the whole live set
// and not the frame's base.
//
// The count is the reason the arm exists. Seven roots are live at the
// return: the enclosing body's literal (the carrier, then the identity its
// push leaves behind), the walk's source literal (the same pair — it is
// written at the loop's position, so it materializes inside the body), the
// walk's own snapshot, and the loop body's literal. A discharge that
// stopped at the loop frame would pop the body's two and hand the
// collector the other five.
func TestReturnFromALoopBodyDischargesTheWholeLiveSet(t *testing.T) {
	ir := assertClean(t, listModule([]ast.Item{
		pubFn("f", nil, named("Int64"),
			letBind("keep", &ast.ListLit{Elems: []ast.Expr{intLit("9")}}),
			walk("x", &ast.ListLit{Elems: []ast.Expr{intLit("1")}},
				letBind("xs", &ast.ListLit{Elems: []ast.Expr{intLit("2")}}),
				retValue(intLit("7")),
			),
			retValue(intLit("0")),
		),
	},
		letBind("n", &ast.Call{Fn: ident("f")}),
	))
	// The enclosing literal's two, the source literal's two, the
	// snapshot's one, the body's two.
	popsBefore(t, ir, "ret i64 7", 7)
}

// TestBodiesWithoutRootsEmitNoDischarge: the repair must not touch a body
// that pushes nothing — the whole M8/M9b corpus (and every golden pinned
// from it) rides on those bytes staying identical.
func TestBodiesWithoutRootsEmitNoDischarge(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		letBind("i", intLit("0")),
		letBind("c", boolLit("true")),
		whileShape(binOp("<", ident("i"), intLit("3")),
			letBind("n", binOp("+", intLit("1"), intLit("2"))),
		),
		okReturn(),
	), "whexit")
	if strings.Contains(ir, "__we_root_pop") || strings.Contains(ir, "__we_root_push") {
		t.Fatalf("a body that roots nothing emits root traffic:\n%s", ir)
	}
}
