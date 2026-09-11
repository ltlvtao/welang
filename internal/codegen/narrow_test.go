package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T11-3: chapter 7's checked arithmetic is checked against the DECLARED
// width, and the i64 register domain is not that width for six of the
// eight integer types. The tests below pin the emitter's three answers:
// a narrow operation carries the width its operands declared, the range
// test that answers the width's question is emitted beside (not instead
// of) the register's own checks, and an operation the width cannot take
// out of range carries no test at all.

// narrowVar builds `var name: T = <lit><suffix>`.
func narrowVar(name, typ, suffix string) *ast.Binding {
	return &ast.Binding{Kw: "var", Name: name, Typ: named(typ), Init: intLit("1" + suffix)}
}

// narrowLet builds `let name: T = init`.
func narrowLet(name, typ string, init ast.Expr) *ast.Binding {
	return &ast.Binding{Kw: "let", Name: name, Typ: named(typ), Init: init}
}

// narrowWidths is the six widths whose domain is narrower than the
// register's, with the two bounds narrowBounds tests against.
var narrowWidths = []struct {
	typ, suffix, lo, hi string
}{
	{"Int8", "i8", "-128", "127"},
	{"Int16", "i16", "-32768", "32767"},
	{"Int32", "i32", "-2147483648", "2147483647"},
	{"UInt8", "u8", "0", "255"},
	{"UInt16", "u16", "0", "65535"},
	{"UInt32", "u32", "0", "4294967295"},
}

// narrowModule wraps one narrow sum `let c: T = a + b` over two vars of
// the same width, plus a discard read so the operator is the only
// arithmetic in the body.
func narrowSumBody(typ, suffix string, x, y string) []ast.Stmt {
	return []ast.Stmt{
		narrowVar(x, typ, suffix),
		narrowVar(y, typ, suffix),
		narrowLet("c", typ, binOp("+", ident(x), ident(y))),
	}
}

// TestNarrowArithmeticChecksItsOwnWidth: every one of the six narrow
// widths gets the range pair around its declared bounds and reports the
// width's own name — the width the source wrote, not the register's.
func TestNarrowArithmeticChecksItsOwnWidth(t *testing.T) {
	for _, w := range narrowWidths {
		t.Run(w.typ, func(t *testing.T) {
			ir := assertClean(t, m9bModule(append(narrowSumBody(w.typ, w.suffix, "a", "b"),
				okReturn())...), w.typ+" add overflow")
			lo := "icmp slt i64 %v"
			if !strings.Contains(ir, lo) || !strings.Contains(ir, ", "+w.lo+"\n") {
				t.Fatalf("no lower-bound test for %s (%s):\n%s", w.typ, w.lo, ir)
			}
			if !strings.Contains(ir, "icmp sgt i64 %v") || !strings.Contains(ir, ", "+w.hi+"\n") {
				t.Fatalf("no upper-bound test for %s (%s):\n%s", w.typ, w.hi, ir)
			}
		})
	}
}

// TestNarrowArithmeticLeavesTheIntrinsicBehind: the i64 intrinsic is the
// full width's check and cannot see a narrow sum at all — `127i8 + 1i8`
// is 128, a perfectly good i64 — so a narrow width rides plain arithmetic
// and the intrinsic's own report must not appear in the body.
func TestNarrowArithmeticLeavesTheIntrinsicBehind(t *testing.T) {
	ir := assertClean(t, m9bModule(append(narrowSumBody("Int8", "i8", "a", "b"), okReturn())...),
		"add i64")
	for _, intr := range []string{"llvm.sadd.with.overflow", "llvm.ssub.with.overflow", "llvm.smul.with.overflow"} {
		if strings.Contains(ir, intr) {
			t.Fatalf("the narrow sum kept the i64 intrinsic %s:\n%s", intr, ir)
		}
	}
	if strings.Contains(ir, "Int64 add overflow") {
		t.Fatalf("the narrow sum reported the register's width:\n%s", ir)
	}
}

// TestNarrowUnsignedProductNamesItsOwnWidth: UInt32's widest product is
// past 2^63, where the register's signed reading is negative — the case
// that makes the intrinsic's sign flag a criterion deciding the wrong
// way for two unsigned operands. The width's range test is what decides.
func TestNarrowUnsignedProductNamesItsOwnWidth(t *testing.T) {
	ir := assertClean(t, m9bModule(
		narrowVar("a", "UInt32", "u32"),
		narrowVar("b", "UInt32", "u32"),
		narrowLet("c", "UInt32", binOp("*", ident("a"), ident("b"))),
		okReturn(),
	), "UInt32 mul overflow", "mul i64")
	if strings.Contains(ir, "llvm.smul.with.overflow") {
		t.Fatalf("the unsigned product kept the signed intrinsic:\n%s", ir)
	}
}

// TestFullWidthArithmeticKeepsTheIntrinsic: Int64 and UInt64 ARE the
// register, so their overflow is the intrinsic's to report and the width
// test would be a criterion whose answer is never the deciding one.
func TestFullWidthArithmeticKeepsTheIntrinsic(t *testing.T) {
	for _, typ := range []string{"Int64", "UInt64"} {
		t.Run(typ, func(t *testing.T) {
			suffix := ""
			if typ == "UInt64" {
				suffix = "u64"
			}
			ir := assertClean(t, m9bModule(append(narrowSumBody(typ, suffix, "a", "b"), okReturn())...),
				"llvm.sadd.with.overflow.i64", "Int64 add overflow")
			if strings.Contains(ir, ", 18446744073709551615\n") {
				t.Fatalf("%s grew a width test:\n%s", typ, ir)
			}
		})
	}
}

// TestNarrowDivisionChecksTheQuotientAlone: a narrow minimum by -1 is a
// value the register holds with room to spare (Int8's -128/-1 is 128), so
// the quotient takes the range test; a remainder cannot leave the width
// at all — `|r| < |divisor|` — and takes none.
func TestNarrowDivisionChecksTheQuotientAlone(t *testing.T) {
	div := assertClean(t, m9bModule(
		narrowVar("a", "Int8", "i8"),
		narrowVar("b", "Int8", "i8"),
		narrowLet("c", "Int8", binOp("/", ident("a"), ident("b"))),
		okReturn(),
	), "sdiv i64", "Int8 div overflow")
	if !strings.Contains(div, ", 127\n") {
		t.Fatalf("the quotient took no width test:\n%s", div)
	}
	rem := assertClean(t, m9bModule(
		narrowVar("a", "Int8", "i8"),
		narrowVar("b", "Int8", "i8"),
		narrowLet("c", "Int8", binOp("%", ident("a"), ident("b"))),
		okReturn(),
	), "srem i64")
	if strings.Contains(rem, "overflow") {
		t.Fatalf("the remainder grew a width test:\n%s", rem)
	}
}

// TestNarrowLeftShiftKeepsEveryCheck: a shift is a product by a power of
// two, and a narrow operand's product can leave the REGISTER before any
// declared width is consulted (2^31 by 2^33), so the register's two
// checks stay and the width test joins them. Both report the width the
// source wrote.
func TestNarrowLeftShiftKeepsEveryCheck(t *testing.T) {
	ir := assertClean(t, m9bModule(
		narrowVar("a", "Int8", "i8"),
		narrowVar("b", "Int8", "i8"),
		narrowLet("c", "Int8", binOp("<<", ident("a"), ident("b"))),
		okReturn(),
	), "shl i64", "ashr i64", "icmp ult i64 %v", "Int8 shift overflow")
	if strings.Contains(ir, "Int64 shift overflow") {
		t.Fatalf("the narrow shift reported the register's width:\n%s", ir)
	}
}

// TestNarrowLogicIsTotal: two values inside a width have no bit above it
// to set, so `& | ^` cannot leave the width — the width test would be a
// criterion that never decides, and these three carry no trap at all.
func TestNarrowLogicIsTotal(t *testing.T) {
	for _, op := range []string{"&", "|", "^"} {
		t.Run(op, func(t *testing.T) {
			ir := assertClean(t, m9bModule(
				narrowVar("a", "Int8", "i8"),
				narrowVar("b", "Int8", "i8"),
				narrowLet("c", "Int8", binOp(op, ident("a"), ident("b"))),
				okReturn(),
			))
			if strings.Contains(ir, "overflow") {
				t.Fatalf("`%s` grew a width test:\n%s", op, ir)
			}
		})
	}
}

// TestNarrowRightShiftChecksTheAmountAlone: an arithmetic right shift
// moves the top bit down and drops low ones, so the value stays inside the
// width whatever the amount — but the amount is still a shift's own
// pre-condition (LLVM's shift is poison at or past the register's width),
// and that check reports the width the source wrote.
func TestNarrowRightShiftChecksTheAmountAlone(t *testing.T) {
	ir := assertClean(t, m9bModule(
		narrowVar("a", "Int8", "i8"),
		narrowVar("b", "Int8", "i8"),
		narrowLet("c", "Int8", binOp(">>", ident("a"), ident("b"))),
		okReturn(),
	), "ashr i64", "icmp ult i64 %v", "Int8 shift overflow")
	for _, bound := range []string{", 127\n", ", -128\n"} {
		if strings.Contains(ir, bound) {
			t.Fatalf("`>>` grew a width test (%s):\n%s", bound, ir)
		}
	}
}

// TestNarrowComplementMasksTheUnsignedWidth: the i64 complement is the
// width's complement only for a sign-extended operand. A zero-extended
// one (~200u8 reads 200) complements to -201, which no UInt8 holds — the
// width's mask is what the not means there.
func TestNarrowComplementMasksTheUnsignedWidth(t *testing.T) {
	for _, tc := range []struct{ typ, suffix, mask string }{
		{"Int8", "i8", "-1"},
		{"Int16", "i16", "-1"},
		{"Int32", "i32", "-1"},
		{"UInt8", "u8", "255"},
		{"UInt16", "u16", "65535"},
		{"UInt32", "u32", "4294967295"},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			ir := assertClean(t, m9bModule(
				narrowVar("a", tc.typ, tc.suffix),
				narrowLet("b", tc.typ, unary("~", ident("a"))),
				okReturn(),
			), "xor i64 ")
			if !strings.Contains(ir, ", "+tc.mask+"\n") {
				t.Fatalf("the complement used the wrong mask (%s):\n%s", tc.mask, ir)
			}
		})
	}
}

// TestNarrowNegationChecksTheWidthMinimum: the width's own minimum is the
// only value a negation takes out of range, and the i64 subtraction from
// zero holds it exactly — so the range test is the whole check.
func TestNarrowNegationChecksTheWidthMinimum(t *testing.T) {
	ir := assertClean(t, m9bModule(
		narrowVar("a", "Int8", "i8"),
		narrowLet("b", "Int8", unary("-", ident("a"))),
		okReturn(),
	), "sub i64 0, %v", "Int8 neg overflow", ", 127\n")
	if strings.Contains(ir, "llvm.ssub.with.overflow") {
		t.Fatalf("the narrow negation kept the i64 intrinsic:\n%s", ir)
	}
}

// TestNarrowParamsCarryTheWidthIntoTheBody: a parameter's declared type
// is the width its body's arithmetic is checked against — the width rides
// the binding the signature made, not the operand's own text.
func TestNarrowParamsCarryTheWidthIntoTheBody(t *testing.T) {
	ir := assertClean(t, &ast.File{Items: []ast.Item{
		stdIoImport("io"),
		appError(),
		&ast.FnDecl{
			Name:   "sum",
			Params: []ast.Param{{Name: "a", Type: named("Int8")}, {Name: "b", Type: named("Int8")}},
			Ret:    named("Int8"),
			Body:   ast.Block{Items: []ast.Stmt{retValue(binOp("+", ident("a"), ident("b")))}},
		},
		mainDecl(okReturn()),
	}}, "Int8 add overflow", ", 127\n")
	if strings.Contains(ir, "llvm.sadd.with.overflow") {
		t.Fatalf("the narrow parameter sum kept the i64 intrinsic:\n%s", ir)
	}
}

// TestNarrowWidthSurvivesASlotRebind: a name the body assigns stores its
// value, and the width has to outlive the site that fixed it — the read
// after the store is a fresh value of the same declared width, and both
// the assignment's sum and the reader's are checked.
func TestNarrowWidthSurvivesASlotRebind(t *testing.T) {
	ir := assertClean(t, m9bModule(
		narrowVar("a", "Int8", "i8"),
		assignTo("a", binOp("+", ident("a"), intLit("1i8"))),
		narrowLet("b", "Int8", binOp("+", ident("a"), intLit("1i8"))),
		okReturn(),
	))
	if n := strings.Count(ir, "Int8 add overflow"); n != 2 {
		t.Fatalf("expected both sums checked, got %d checks:\n%s", n, ir)
	}
}

// TestNarrowListElementCarriesTheWidth: a list's element face holds the
// width its element type declared, so a body's arithmetic over two
// elements is checked even though no literal in the body names a width.
func TestNarrowListElementCarriesTheWidth(t *testing.T) {
	ir := assertClean(t, listModule(nil,
		listOf("xs", "Int8", intLit("1i8"), intLit("2i8")),
		walk("x", ident("xs"), narrowLet("y", "Int8", binOp("+", ident("x"), ident("x")))),
	), "Int8 add overflow", ", 127\n")
	if strings.Contains(ir, "llvm.sadd.with.overflow") {
		t.Fatalf("the element sum kept the i64 intrinsic:\n%s", ir)
	}
}

// TestNarrowDomainPrefersTheSideThatHasAWidth: a value-position form's
// join reports no width, so an operation over one still has an answer when
// the other operand carries one — the suffix `1i8` on the right is what
// makes `let p: Int8 = <form> + 1i8` a checked Int8 sum. The mirrored
// order checks the same sum through the left operand, so the pair pins
// both sides of the rule rather than one spelling of it.
func TestNarrowDomainPrefersTheSideThatHasAWidth(t *testing.T) {
	form := &ast.If{
		Cond: binOp(">", ident("a"), intLit("0")),
		Then: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: intLit("127i8")}}},
		Else: blockOf(&ast.ExprStmt{Expr: intLit("1i8")}),
	}
	ir := assertClean(t, m9bModule(
		typedLit("a", "1"),
		narrowLet("p", "Int8", binOp("+", form, intLit("1i8"))),
		narrowLet("q", "Int8", binOp("+", intLit("1i8"), form)),
		okReturn(),
	))
	if n := strings.Count(ir, "Int8 add overflow"); n != 2 {
		t.Fatalf("expected both orders checked, got %d checks:\n%s", n, ir)
	}
	if strings.Contains(ir, "llvm.sadd.with.overflow") {
		t.Fatalf("a join operand's sum kept the i64 intrinsic:\n%s", ir)
	}
}

// TestNarrowValueFormTakesTheAnnotation: a value-position control form's
// join is a slot the site did not type, so its emission has no width to
// report — `let n: Int8 = if …` is where the declaration's own annotation
// answers instead. The test binds two such names and adds them: neither
// operand carries a literal, so the sum is checked only if both bindings
// took the annotation's width.
//
// A value form bound to a `let` is reachable from source and runs: the
// shape this test mirrors — `let n: Int8 = if a > 0 {2i8} else {1i8}`
// twice, then their sum — is a program the corpus can spell, and it
// compiles and answers. What is out of reach is reading such a binding, or
// anything derived from it, into a `String` position: the string chain
// asks valueKind, which carries no If/Match/BlockExpr arm and answers
// skNone, so the hole stops there. The reachable shape has a probe behind
// it and no fixture yet — a golden for it is registered as a T13
// reconciliation item (docs/benchmarks.md, "Run-face anchoring", states
// the boundary). An earlier revision of this comment claimed the pipeline
// stopped the binding itself, which T12's probes disproved.
func TestNarrowValueFormTakesTheAnnotation(t *testing.T) {
	form := func(lit string) ast.Expr {
		return &ast.If{
			Cond: binOp(">", ident("a"), intLit("0")),
			Then: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: intLit(lit)}}},
			Else: blockOf(&ast.ExprStmt{Expr: intLit("1i8")}),
		}
	}
	ir := assertClean(t, m9bModule(
		typedLit("a", "1"),
		narrowLet("n", "Int8", form("127i8")),
		narrowLet("m", "Int8", form("2i8")),
		narrowLet("y", "Int8", binOp("+", ident("n"), ident("m"))),
		okReturn(),
	), "Int8 add overflow", ", 127\n")
	if strings.Contains(ir, "llvm.sadd.with.overflow") {
		t.Fatalf("the joined sum kept the i64 intrinsic:\n%s", ir)
	}
}
