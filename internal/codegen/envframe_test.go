package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// The env-frame suite pins the T1 scoping fix (follow-up #16). A block
// (match arm, if branch, scope body, while body) may register names only
// for the duration of its own emission; today a block-local binding
// writes the flat env maps and never rolls back, so a later read of a
// same-named outer binding picks the inner operand:
//
//   - the inner operand is an SSA register defined inside the block and
//     the block boundary is an LLVM block (match arms, if branches) —
//     clang rejects the module ("Instruction does not dominate all
//     uses", the errpath-02 shape, recorded against the real toolchain);
//   - the block emits inline (scope bodies) — the module builds, and the
//     program silently computes the wrong value.
//
// Either way the join must read the outer binding. The outer bindings
// below hold literal constants, so a correct join reads the literal
// operand: `icmp eq i64 5, 5`. Today the join reads the block register:
// `icmp eq i64 %vN, 5` — red on every assertion here.

// shadowAST is the common dominance shape: an outer scalar `k` (5) and
// `m` (2), a shadowing block statement that binds `k` to the register
// `m + 1`, then a post-block read of `k` comparing against 5.
func shadowAST(block ast.Stmt) *ast.File {
	return m9bModule(
		&ast.Binding{Kw: "var", Name: "m", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("2")},
		&ast.Binding{Kw: "let", Name: "k", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("5")},
		block,
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", ident("k"), intLit("5")),
			Then: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"five"`))}},
			Else: blockOf(ioCall("io", "println", strLit(`"notfive"`))),
		}},
		okReturn(),
	)
}

// shadowLet is the block-local binding `let k = m + 1` (a register).
func shadowLet() *ast.Binding {
	return &ast.Binding{Kw: "let", Name: "k", Typ: &ast.NamedType{Name: "Int64"},
		Init: binOp("+", ident("m"), intLit("1"))}
}

// assertJoinReadsOuter pins that the module's k-comparison compares the
// outer constant on both sides — the block-local register must never
// reach the join.
func assertJoinReadsOuter(t *testing.T, ir string) {
	t.Helper()
	if !strings.Contains(ir, "icmp eq i64 5, 5") {
		t.Fatalf("the post-block read of the shadowed outer binding must compare the outer\n"+
			"constant (icmp eq i64 5, 5); the block-local register leaked into the join:\n%s", ir)
	}
}

// TestEnvFrameMatchArmShadow: a match arm payload binding shadowing the
// outer `k`, with the arm-local register read after the join.
func TestEnvFrameMatchArmShadow(t *testing.T) {
	ir := assertClean(t, m9bModule(
		chanBind("ch", "1"),
		&ast.ExprStmt{Expr: callOn(ident("ch"), "send", intLit("7"))},
		&ast.Binding{Kw: "let", Name: "v", Init: callOn(ident("ch"), "receive")},
		&ast.Binding{Kw: "var", Name: "m", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("2")},
		&ast.Binding{Kw: "let", Name: "k", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("5")},
		&ast.ExprStmt{Expr: &ast.Match{
			Scrutinee: ident("v"),
			Arms: []ast.MatchArm{
				{Pat: &ast.PatVariant{Name: "Some", Args: []ast.Pattern{&ast.PatBinding{Name: "k"}}},
					Body: blockOf(shadowLet())},
				{Pat: &ast.PatVariant{Name: "None"},
					Body: blockOf(&ast.Binding{Kw: "let", Name: "_", Init: intLit("0")})},
			},
		}},
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", ident("k"), intLit("5")),
			Then: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"five"`))}},
			Else: blockOf(ioCall("io", "println", strLit(`"notfive"`))),
		}},
		okReturn(),
	), "mjoin")
	assertJoinReadsOuter(t, ir)
}

// TestEnvFrameTaskCapture: the errpath-02 shape — a match arm binding
// runs before a scope whose task captures the shadowed outer name; the
// env-block store must carry the outer operand.
func TestEnvFrameTaskCapture(t *testing.T) {
	ir := assertClean(t, m9bModule(
		chanBind("ch", "1"),
		&ast.ExprStmt{Expr: callOn(ident("ch"), "send", intLit("7"))},
		&ast.Binding{Kw: "let", Name: "v", Init: callOn(ident("ch"), "receive")},
		&ast.Binding{Kw: "let", Name: "n", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("5")},
		&ast.ExprStmt{Expr: &ast.Match{
			Scrutinee: ident("v"),
			Arms: []ast.MatchArm{
				{Pat: &ast.PatVariant{Name: "Some", Args: []ast.Pattern{&ast.PatBinding{Name: "n"}}},
					Body: blockOf(&ast.Binding{Kw: "let", Name: "_", Init: intLit("0")})},
				{Pat: &ast.PatVariant{Name: "None"},
					Body: blockOf(&ast.Binding{Kw: "let", Name: "_", Init: intLit("0")})},
			},
		}},
		&ast.Binding{Kw: "let", Name: "r", Init: &ast.ScopeExpr{
			Timeout: intLit("1000"),
			Body: ast.Block{Items: []ast.Stmt{
				&ast.Binding{Kw: "let", Name: "t", Init: taskIo(
					&ast.Binding{Kw: "let", Name: "_", Init: ident("n")},
				)},
				&ast.Binding{Kw: "let", Name: "_", Init: callOn(ident("t"), "await")},
			}},
		}},
		okReturn(),
	), "mjoin", "__we_task_new")
	join := afterLabel(ir, "mjoin0")
	if !strings.Contains(join, "store i64 5, ptr %") {
		t.Fatalf("task env store after the shadowing match must carry the outer binding's\n"+
			"constant operand, got no `store i64 5` in the join region:\n%s", join)
	}
}

// TestEnvFrameIfBranchShadow: an if branch binding shadowing the outer
// `k`; the register must not reach the post-join read.
func TestEnvFrameIfBranchShadow(t *testing.T) {
	ir := assertClean(t, shadowAST(&ast.ExprStmt{Expr: &ast.If{
		Cond: binOp("==", intLit("1"), intLit("1")),
		Then: ast.Block{Items: []ast.Stmt{shadowLet()}},
		Else: blockOf(),
	}}), "ifjoin")
	assertJoinReadsOuter(t, ir)
}

// TestEnvFrameScopeBodyShadow: a scope body is a block — a binding made
// inside it must not reach a post-leave read of the same-named outer
// binding (this shape builds today but computes the wrong value).
func TestEnvFrameScopeBodyShadow(t *testing.T) {
	ir := assertClean(t, shadowAST(&ast.ExprStmt{Expr: &ast.ScopeExpr{
		Body: ast.Block{Items: []ast.Stmt{shadowLet()}},
	}}), "__we_scope_leave")
	assertJoinReadsOuter(t, ir)
}

// TestEnvFrameWhileBodyShadow: a while body binding shadowing the outer
// `k`; the loop-local register must not reach a post-loop read.
func TestEnvFrameWhileBodyShadow(t *testing.T) {
	ir := assertClean(t, shadowAST(&ast.While{
		Cond: binOp("<", ident("m"), intLit("3")),
		Body: ast.Block{Items: []ast.Stmt{
			shadowLet(),
			&ast.Assign{Name: "m", Value: binOp("+", ident("m"), intLit("1"))},
		}},
	}), "whexit")
	assertJoinReadsOuter(t, ir)
}

// afterLabel slices the IR from the label onward (labels end with ":").
func afterLabel(ir, label string) string {
	i := strings.Index(ir, label+":")
	if i < 0 {
		return ""
	}
	return ir[i:]
}
