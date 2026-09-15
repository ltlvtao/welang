// The scope value forms (design D9 cluster 12): the plain and collectAll
// forms' value is the body's block value under chapter 2 — the tail's own
// face, not a Result's three slots — while the timeout forms keep the
// Ok/Err sum the spec gives them. These pins hold the emitter side of
// that split: what the binding carries, what the timeout still stores,
// and where the honest stops remain.

package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// scopeVal builds a value-form scope whose body runs statements then a
// tail expression.
func scopeVal(timeout string, collectAll bool, body ...ast.Stmt) *ast.ScopeExpr {
	s := &ast.ScopeExpr{Body: ast.Block{Items: body}}
	if timeout != "" {
		s.Timeout = intLit(timeout)
	}
	s.CollectAll = collectAll
	return s
}

// tailStmt wraps one expression as a block's tail.
func tailStmt(x ast.Expr) ast.Stmt { return &ast.ExprStmt{Expr: x} }

// TestScopeValueFormCarriesTheBodiesOwnFace: a plain scope's numeric tail
// is the scope's value in its own face. The consumer arithmetic reads the
// tail's SSA word directly — no Result slots anywhere — and the tail runs
// after the body statements, so a name the body rebinds answers from the
// rebinding (the body's exit value, never its entry's).
func TestScopeValueFormCarriesTheBodiesOwnFace(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "n", Init: intLit("10")},
		&ast.Binding{Kw: "let", Name: "b", Init: scopeVal("", false,
			&ast.Binding{Kw: "let", Name: "n", Init: intLit("3")},
			tailStmt(binOp("+", ident("n"), intLit("4"))))},
		&ast.Binding{Kw: "let", Name: "w", Init: binOp("+", ident("b"), intLit("5"))},
		ioCall("io", "println", interpLit([]string{"v ", ""}, ident("b"))),
		ioCall("io", "println", interpLit([]string{"w ", ""}, ident("w"))),
		okReturn(),
	), "call ptr @__we_scope_enter", "call i64 @__we_scope_leave")
	if n := strings.Count(ir, "= call { i64, i1 } @llvm.sadd.with.overflow.i64("); n != 2 {
		t.Fatalf("the tail's add and the consumer's add, nothing more: got %d\n%s", n, ir)
	}
	// The tail adds the body's n (3), not the outer 10 — the tail runs
	// inside the body's frame, after its bindings.
	if !strings.Contains(ir, "@llvm.sadd.with.overflow.i64(i64 3, i64 4)") {
		t.Fatalf("the tail must read the body's rebinding:\n%s", ir)
	}
	if !strings.Contains(ir, "@llvm.sadd.with.overflow.i64(i64 %v") {
		t.Fatalf("the consumer adds the tail's SSA word:\n%s", ir)
	}
	// The old answer stored tag/payload/pay1 into three slots after the
	// leave; the body's own face stores nothing.
	if strings.Contains(ir, "store i64") {
		t.Fatalf("the plain form's value rides SSA, no Result slots:\n%s", ir)
	}
}

// TestScopeValueFormAnswersAStringFace: a plain scope whose tail is a
// String binds the pair, so the concatenation inside the body and the
// interpolation outside it read the same two words a `let` over the tail
// itself would hold.
func TestScopeValueFormAnswersAStringFace(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: scopeVal("", false,
			&ast.Binding{Kw: "let", Name: "p", Init: strLit(`"a"`)},
			tailStmt(binOp("+", ident("p"), strLit(`"b"`))))},
		ioCall("io", "println", interpLit([]string{"s ", ""}, ident("s"))),
		okReturn(),
	), "call ptr @__we_scope_enter", "call i64 @__we_scope_leave")
	if n := countCall(ir, "%struct.we_str", "__we_str_concat"); n != 2 {
		t.Fatalf("one concat per `+` and one for the interpolation's own prefix: got %d\n%s", n, ir)
	}
}

// TestScopeTimeoutKeepsTheResultFace: the timeout forms' value is the
// spec's Result — Ok(b)/Err(TimedOut) — and that answer is the three
// slots the plain form dropped: tag, payload word, and the zero pay1.
func TestScopeTimeoutKeepsTheResultFace(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "r", Init: scopeVal("5", false,
			tailStmt(intLit("7")))},
		okReturn(),
	), "call ptr @__we_scope_enter(i64 5, i64 0)", "call i64 @__we_scope_leave")
	if n := strings.Count(ir, "store i64"); n != 3 {
		t.Fatalf("the timeout answer is the three-slot Result: got %d i64 stores\n%s", n, ir)
	}
}

// TestScopeCollectAllAnswersTheBodyFace: collectAll without a timeout is
// a plain form for the value — the body's own face, the enter call
// carrying the join flag.
func TestScopeCollectAllAnswersTheBodyFace(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "d", Init: scopeVal("", true,
			tailStmt(intLit("9")))},
		&ast.Binding{Kw: "let", Name: "w", Init: binOp("+", ident("d"), intLit("1"))},
		okReturn(),
	), "call ptr @__we_scope_enter(i64 -1, i64 1)", "call i64 @__we_scope_leave")
	if strings.Contains(ir, "store i64") {
		t.Fatalf("collectAll's plain value is the body's face, no Result slots:\n%s", ir)
	}
	if n := strings.Count(ir, "= call { i64, i1 } @llvm.sadd.with.overflow.i64("); n != 1 {
		t.Fatalf("the consumer's add reads the tail's word: got %d\n%s", n, ir)
	}
}

// TestScopeValueFormStopsOnAFaceItCannotCarry: a tail whose face the
// value tower does not carry — a record's handle — keeps stopping at the
// binding, the honest boundary the gc-tail golden anchors.
func TestScopeValueFormStopsOnAFaceItCannotCarry(t *testing.T) {
	const src = `import std.io

pub type AppError = Failed(String)

record Cell {
    hi: Int64
}

pub fn main() effect io -> Result<(), AppError> {
    let h = scope {
        Cell { hi: 7 }
    }
    io.println("h ${h.hi}")
    return Ok(())
}
`
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni == nil {
		t.Fatalf("a record tail is no face the value form carries")
	}
	if !strings.Contains(ni.What, "statement set") {
		t.Fatalf("want the body boundary word, got %q", ni.What)
	}
}

// TestScopeValueFormBindsInsideAFnBody: the same split holds inside a
// function body — the binding carries the tail's word, and the return
// hands the caller exactly that operand.
func TestScopeValueFormBindsInsideAFnBody(t *testing.T) {
	ir := assertClean(t, &ast.File{Items: []ast.Item{
		stdIoImport("io"),
		appError(),
		pubFn("f", nil, named("Int64"),
			&ast.Binding{Kw: "let", Name: "r", Init: scopeVal("", false,
				tailStmt(intLit("5")))},
			retValue(ident("r"))),
		mainDecl(
			letBind("v", call(ident("f"))),
			ioCall("io", "println", ident("v")),
			okReturn()),
	}}, "call ptr @__we_scope_enter", "call i64 @__we_scope_leave")
	if !strings.Contains(ir, "ret i64 5") {
		t.Fatalf("the fn returns the tail's own operand:\n%s", ir)
	}
}
