package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T2-a statement set (design D1): the bare block statement and the
// widened assignment surface. A block is a statement (and a scope): its
// bindings die where it closes, and what follows reads the outer binding
// of the same name. An assignment reaches any numeric name the body
// binds — a let as well as a var — because the checker has always
// accepted it (follow-up #7): the emitter's refusal was a check/build
// split, not a rule (probe: `let n = 1; n = 2` passes `we check` and
// stops `we build` at the M9b body boundary).
//
// The name an assignment writes needs an address, so a let the body
// assigns leaves its SSA operand for a stack slot; a let the body only
// reads keeps the operand it is today. That split is per body and
// decided before anything emits, so a binding's storage never depends on
// the statement order around it.

// typedLit builds `let name: Int64 = lit`.
func typedLit(name, text string) *ast.Binding {
	return &ast.Binding{Kw: "let", Name: name, Typ: &ast.NamedType{Name: "Int64"}, Init: intLit(text)}
}

// varLit builds `var name: Int64 = lit`.
func varLit(name, text string) *ast.Binding {
	return &ast.Binding{Kw: "var", Name: name, Typ: &ast.NamedType{Name: "Int64"}, Init: intLit(text)}
}

// assignTo builds `name = v`.
func assignTo(name string, v ast.Expr) *ast.Assign {
	return &ast.Assign{Name: name, Value: v}
}

// TestBareBlockScope: the bare block is a scope — the block-local `s`
// dies with it, and the statement after the block reads the outer `s`.
// The two println calls carry the two literals' lengths, so the order of
// the operands is the whole assertion: the inner binding (6) first, the
// outer one (3) after the block closes.
func TestBareBlockScope(t *testing.T) {
	block := &ast.ExprStmt{Expr: &ast.BlockExpr{Block: ast.Block{Items: []ast.Stmt{
		letBind("s", strLit(`"inside"`)),
		ioCall("io", "println", ident("s")),
	}}}}
	ir := assertClean(t, m9bModule(
		letBind("s", strLit(`"out"`)),
		block,
		ioCall("io", "println", ident("s")),
		okReturn(),
	))
	order(t, ir, "i64 6", "i64 3")
}

// TestBareBlockAssignsOuter: a bare block is a statement of the body it
// sits in, so an assignment inside it reaches the outer name — the
// name needs a slot however deep the write sits.
func TestBareBlockAssignsOuter(t *testing.T) {
	ir := assertClean(t, drModule(pubFn("f", nil, named("Int64"),
		typedLit("n", "1"),
		&ast.ExprStmt{Expr: &ast.BlockExpr{Block: ast.Block{Items: []ast.Stmt{
			assignTo("n", intLit("7")),
		}}}},
		retValue(ident("n")),
	)))
	order(t, ir, "entry:", "alloca i64", "store i64 1, ptr", "store i64 7, ptr", "load i64, ptr")
}

// TestAssignLetTakesSlot: a let the body assigns owns one entry-block
// slot — the initializer stores into it, the assignment reads and
// rewrites it, and the return reads it once more.
func TestAssignLetTakesSlot(t *testing.T) {
	ir := assertClean(t, drModule(pubFn("f", nil, named("Int64"),
		typedLit("n", "1"),
		assignTo("n", binOp("+", ident("n"), intLit("1"))),
		retValue(ident("n")),
	)))
	if got := strings.Count(ir, "alloca i64"); got != 1 {
		t.Fatalf("want exactly one slot (the assigned let), got %d:\n%s", got, ir)
	}
	order(t, ir,
		"entry:",
		"alloca i64",
		"store i64 1, ptr",
		"load i64, ptr", // the assignment reads the name
		"store i64",     // and rewrites it
		"load i64, ptr", // the tail reads it again
		"ret i64",
	)
}

// TestAssignParamTakesSlot: a parameter the body assigns takes a slot
// too — the incoming register stores into it before anything reads the
// name as a name.
func TestAssignParamTakesSlot(t *testing.T) {
	ir := assertClean(t, drModule(pubFn("f", []ast.Param{i64Param("n")}, named("Int64"),
		assignTo("n", binOp("+", ident("n"), intLit("1"))),
		retValue(ident("n")),
	)), "define i64 @main.f(i64 %n)")
	order(t, ir,
		"entry:",
		"alloca i64",
		"store i64 %n, ptr",
		"load i64, ptr",
		"ret i64",
	)
}

// TestReadOnlyLetKeepsSSA: a name the body never assigns stays what it
// is — an SSA operand, with nothing reserved for it.
func TestReadOnlyLetKeepsSSA(t *testing.T) {
	ir := assertClean(t, drModule(pubFn("f", nil, named("Int64"),
		typedLit("n", "4"),
		retValue(ident("n")),
	)))
	if strings.Contains(ir, "alloca") {
		t.Fatalf("a read-only let reserves nothing:\n%s", ir)
	}
	if !strings.Contains(ir, "ret i64 4") {
		t.Fatalf("the tail must carry the bound constant:\n%s", ir)
	}
}

// TestAssignAliasOwnSlot: `let b = a` shares a's storage while both are
// read-only, but an assigned alias needs its own — the write must not
// reach back through the alias into the source.
func TestAssignAliasOwnSlot(t *testing.T) {
	ir := assertClean(t, drModule(pubFn("f", nil, named("Int64"),
		typedLit("a", "1"),
		&ast.Binding{Kw: "let", Name: "b", Init: ident("a")},
		assignTo("b", intLit("2")),
		retValue(ident("a")),
	)))
	if got := strings.Count(ir, "alloca i64"); got != 1 {
		t.Fatalf("only the assigned alias needs a slot, got %d:\n%s", got, ir)
	}
	order(t, ir, "store i64 1, ptr", "store i64 2, ptr", "ret i64 1")
}

// TestSlotHoistedToEntry is the regression pin for the defect the slot
// face exposed (design D13): a slot reserved at its binding site would
// be allocated afresh on every pass through an enclosing loop and the
// stack it takes returns only when the function does. Measured on the
// real toolchain before the fix: a loop of 30M iterations around a
// loop-body `var` died on a segmentation fault with the alloca text
// sitting in the loop body.
func TestSlotHoistedToEntry(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("i", "0"),
		whileShape(binOp("<", ident("i"), intLit("3")),
			varLit("j", "1"),
			assignTo("i", binOp("+", ident("i"), ident("j"))),
		),
		okReturn(),
	), "whbody")
	if got := strings.Count(ir, "alloca i64"); got != 2 {
		t.Fatalf("want two slots, got %d:\n%s", got, ir)
	}
	order(t, ir, "entry:", "alloca i64", "alloca i64", "br label %whhead")
	body := afterLabel(ir, "whbody0")
	if body == "" {
		t.Fatalf("missing the loop body label:\n%s", ir)
	}
	if strings.Contains(body, "alloca") {
		t.Fatalf("a slot reserved inside the loop body is taken once per iteration:\n%s", ir)
	}
}
