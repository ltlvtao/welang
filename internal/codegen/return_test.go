package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T2 statement set (design D1): the return at any depth. Every body kind
// has one exit protocol — a return answers it wherever it sits, and the
// statements lexically after it are dead: the block ends at the exit
// sequence's own terminator, so nothing emits until a fresh label opens
// (the enclosing structure's guarded joins do the opening).
//
// The exit sequence, in order: the value computes (it may read bindings
// the exit is about to leave behind), every open scope discharges
// innermost-first (chapter 18:231), the body's defers drain in reverse
// registration order, the body's root pushes pop, and the ret closes the
// block. Red before the statement set (real toolchain): each shape
// stopped at the boundary word — "control flow" in the M8 boundary table
// and the build-bnd-new-forms conformance case, which expected exit 70 on
// exactly this source.

// drModule wraps a declaration plus a main that tail-returns Ok(()) —
// the body under test rides its own exit protocol.
func drModule(fn *ast.FnDecl) *ast.File {
	return &ast.File{Items: []ast.Item{stdIoImport("io"), appError(), fn, mainDecl(okReturn())}}
}

// TestReturnInIfArm: a return inside a branch emits its exit sequence in
// that arm; the join branch is suppressed (the arm ended in its own
// terminator) while the join label still opens for what follows.
func TestReturnInIfArm(t *testing.T) {
	fn := pubFn("pick", []ast.Param{i64Param("n")}, named("Int64"),
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", ident("n"), intLit("1")),
			Then: ast.Block{Items: []ast.Stmt{retValue(intLit("11"))}},
		}},
		retValue(intLit("22")),
	)
	ir := assertClean(t, drModule(fn), "define i64 @main.pick(", "ret i64 11", "ret i64 22")
	if strings.Contains(ir, "unreachable\n  br") {
		t.Fatalf("an instruction lands after a terminator:\n%s", ir)
	}
	order(t, ir, "ret i64 11", "ifjoin", "ret i64 22")
	if strings.Contains(ir, "br label %ifjoin0\n") == false {
		// The tail path reaches the join through the cond's false edge —
		// that br is the cond's own, emitted before the arms.
		order(t, ir, "label %ifjoin0", "ret i64 22")
	}
}

// TestReturnDrainsDefers: the defers drain at the exit path that is
// emitted, and every path owes them — the deep return's block carries
// the cleanup, and the tail's block carries it again.
func TestReturnDrainsDefers(t *testing.T) {
	fn := pubFn("pick", []ast.Param{i64Param("n")}, named("Int64"),
		&ast.Defer{Block: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"cleanup"`))}}},
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", ident("n"), intLit("1")),
			Then: ast.Block{Items: []ast.Stmt{retValue(intLit("11"))}},
		}},
		retValue(intLit("22")),
	)
	ir := assertClean(t, drModule(fn))
	if got := strings.Count(ir, "load ptr, ptr @slot.io.println"); got != 2 {
		t.Fatalf("want one cleanup per exit path (2), got %d:\n%s", got, ir)
	}
	// Each path runs its cleanup before its own ret.
	order(t, ir,
		"load ptr, ptr @slot.io.println", "ret i64 11",
		"ifjoin0",
		"load ptr, ptr @slot.io.println", "ret i64 22",
	)
}

// TestReturnPiercesScope: a return through an open scope discharges it at
// the return site — cancel (fail-fast) then join — before the ret. The
// scope's own tail never joins again: the body diverted.
func TestReturnPiercesScope(t *testing.T) {
	fn := pubFn("f", []ast.Param{i64Param("n")}, named("Int64"),
		&ast.ExprStmt{Expr: &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{
			letDiscard(taskIo(&ast.ExprStmt{Expr: intLit("1")})),
			retValue(intLit("60")),
		}}}},
		retValue(intLit("0")),
	)
	ir := assertClean(t, drModule(fn), "define i64 @main.f(")
	order(t, ir,
		"call ptr @__we_scope_enter(",
		"call void @__we_scope_cancel(",
		"call i64 @__we_scope_leave(",
		"ret i64 60",
	)
	if got := strings.Count(ir, "call i64 @__we_scope_leave("); got != 1 {
		t.Fatalf("want exactly one leave (the pierce path), got %d:\n%s", got, ir)
	}
}

// TestReturnPiercesInnerToOuter: nested scopes discharge innermost-first.
func TestReturnPiercesInnerToOuter(t *testing.T) {
	inner := &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{retValue(intLit("5"))}}}
	outer := &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: inner}}}}
	fn := pubFn("f", nil, named("Int64"), &ast.ExprStmt{Expr: outer}, retValue(intLit("0")))
	ir := assertClean(t, drModule(fn))
	order(t, ir,
		"call void @__we_scope_cancel(",
		"call i64 @__we_scope_leave(",
		"call void @__we_scope_cancel(",
		"call i64 @__we_scope_leave(",
		"ret i64 5",
	)
	if got := strings.Count(ir, "call void @__we_scope_cancel("); got != 2 {
		t.Fatalf("want two cancels (both scopes cross), got %d:\n%s", got, ir)
	}
}

// TestReturnPopsRoots: the body's root pushes pop before the ret — the
// defer blocks may still read the rooted objects, so the drain runs
// first.
func TestReturnPopsRoots(t *testing.T) {
	fn := pubFn("f", []ast.Param{i64Param("n")}, named("Int64"),
		&ast.Defer{Block: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"c"`))}}},
		&ast.ExprStmt{Expr: &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{
			letDiscard(taskIo(&ast.ExprStmt{Expr: intLit("1")})),
			retValue(intLit("9")),
		}}}},
		retValue(intLit("0")),
	)
	ir := assertClean(t, drModule(fn))
	order(t, ir,
		"call void @__we_root_push(",
		"call void @__we_scope_cancel(",
		"call i64 @__we_scope_leave(",
		"load ptr, ptr @slot.io.println",
		"call void @__we_root_pop()",
		"ret i64 9",
	)
}

// TestReturnNoPopsInTaskThunk: the task thunk is its own body — a deep
// return answers i64 and pops nothing (its pushes ride the task's own gc
// window, retired at task end). A body that returned early also must not
// disturb the enclosing body's scopes.
func TestReturnNoPopsInTaskThunk(t *testing.T) {
	fn := pubFn("f", []ast.Param{i64Param("n")}, named("Int64"),
		&ast.ExprStmt{Expr: &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{
			letDiscard(taskIo(
				&ast.ExprStmt{Expr: &ast.If{
					Cond: binOp(">", ident("n"), intLit("0")),
					Then: ast.Block{Items: []ast.Stmt{retValue(intLit("21"))}},
				}},
				&ast.ExprStmt{Expr: intLit("7")},
			)),
			retValue(intLit("0")),
		}}}},
		retValue(intLit("0")),
	)
	ir := assertClean(t, drModule(fn))
	at := strings.Index(ir, "define internal i64 @.task0(")
	if at < 0 {
		t.Fatalf("missing task thunk:\n%s", ir)
	}
	thunk := ir[at:]
	if end := strings.Index(thunk, "\n}\n"); end >= 0 {
		thunk = thunk[:end]
	}
	if !strings.Contains(thunk, "ret i64 21") {
		t.Fatalf("the thunk's deep return answers i64:\n%s", thunk)
	}
	if strings.Contains(thunk, "__we_root_pop") {
		t.Fatalf("a task thunk pops nothing:\n%s", thunk)
	}
	if strings.Contains(thunk, "__we_scope_cancel") || strings.Contains(thunk, "__we_scope_leave") {
		t.Fatalf("a thunk return does not discharge the enclosing body's scopes:\n%s", thunk)
	}
}

// TestReturnInsideLoop: a return inside a loop body ends that arm; the
// loop's back branch is suppressed and the exit label still opens for
// what follows the loop.
func TestReturnInsideLoop(t *testing.T) {
	fn := pubFn("f", []ast.Param{i64Param("n")}, named("Int64"),
		whileShape(binOp(">", ident("n"), intLit("0")),
			&ast.ExprStmt{Expr: &ast.If{
				Cond: binOp("==", ident("n"), intLit("2")),
				Then: ast.Block{Items: []ast.Stmt{retValue(intLit("99"))}},
			}},
		),
		retValue(intLit("7")),
	)
	ir := assertClean(t, drModule(fn), "whhead", "whexit")
	if strings.Contains(ir, "unreachable\n  br") {
		t.Fatalf("an instruction lands after a terminator:\n%s", ir)
	}
	order(t, ir, "ret i64 99", "br label %whhead", "whexit", "ret i64 7")
}

// TestMainDeepReturn: the entry's protocol is the Result face — an early
// Ok completes with ret i32 0, an early Err reports through __we_fail,
// and the tail after the branch is still emitted for the other path.
func TestMainDeepReturn(t *testing.T) {
	f := &ast.File{Items: []ast.Item{stdIoImport("io"), appError(), mainDecl(
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", intLit("1"), intLit("1")),
			Then: ast.Block{Items: []ast.Stmt{okReturn()}},
		}},
		&ast.ExprStmt{Expr: &ast.Call{Fn: &ast.Member{Recv: ident("io"), Name: "println"}, Args: []ast.Expr{strLit(`"tail"`)}}},
		&ast.Return{HasValue: true, Value: &ast.Call{Fn: ident("Err"), Args: []ast.Expr{
			&ast.Call{Fn: ident("Failed"), Args: []ast.Expr{strLit(`"boom"`)}},
		}}},
	)}}
	ir := assertClean(t, f, "define i32 @__we_main()")
	order(t, ir,
		"ret i32 0",
		"ifjoin0",
		"@slot.io.println",
		"call void @__we_fail(",
		"unreachable",
	)
}

// TestTestDeepReturn: the test context's exit is valueless — a return at
// any depth emits ret void with the exit sequence ahead of it.
func TestTestDeepReturn(t *testing.T) {
	tm := ProgModule{Key: "tests.t_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		stdIoImport("io"),
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		&ast.TestDecl{Desc: "early", Body: ast.Block{Items: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.If{
				Cond: binOp("==", intLit("1"), intLit("1")),
				Then: ast.Block{Items: []ast.Stmt{&ast.Return{}}},
			}},
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
	}}}
	ir, ni := EmitProgram(ModeTest, []ProgModule{progBuild(okReturn()), tm})
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	at := strings.Index(ir, "define void @tests.t_test.test.0()")
	if at < 0 {
		t.Fatalf("missing test define:\n%s", ir)
	}
	def := ir[at:]
	if end := strings.Index(def, "\n}\n"); end >= 0 {
		def = def[:end]
	}
	order(t, def, "ret void", "ifjoin0")
	if strings.Contains(def, "unreachable\n  br") {
		t.Fatalf("an instruction lands after a terminator:\n%s", def)
	}
}

// TestVoidBodyTrailingExpressionIsAStatement: a fn that declares no return
// type has no implicit return — chapter 6 sends its trailing item to
// chapter 8's value-discard rule instead (a non-unit trailing
// expression MUST be discarded explicitly with `let _ =`; unit needs no
// ceremony), and chapter 6's implicit return is granted to the
// declared-return-type case alone. So a void body's last expression
// statement is a statement like
// any other: it emits and its value is dropped, and the define closes with
// a plain ret void. Reading it as a value tail would hand the void family
// a value it has nowhere to put — the boundary — which is exactly the
// shape `impl Releasable`'s release body takes: the resource bodies this
// build's scope resource statement dispatches to end in a method call.
func TestVoidBodyTrailingExpressionIsAStatement(t *testing.T) {
	fn := pubFn("note", nil, nil,
		ioCall("io", "println", strLit(`"noted"`)),
	)
	ir := assertClean(t, drModule(fn), "define void @main.note()", "ret void")
	at := strings.Index(ir, "define void @main.note()")
	body := ir[at:]
	if end := strings.Index(body, "\n}\n"); end >= 0 {
		body = body[:end]
	}
	order(t, body, "load ptr, ptr @slot.io.println", "call void %", "ret void")
	// The statement is not dropped either: the call emits once, in the
	// body's own straight line — a promoted tail would lift it out of that
	// line into a value block of its own.
	if got := strings.Count(ir, "load ptr, ptr @slot.io.println"); got != 1 {
		t.Fatalf("the trailing call emits once, got %d:\n%s", got, ir)
	}
}
