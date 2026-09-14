package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T4 String emission (design D3). Before the task the String face carried
// a literal's constant pair, an fn parameter's registers and a field
// chain's loaded word — and nothing else: the concatenation, the chapter
// 17 members, the equality, and every interpolated literal stopped at
// bndMainBody. These tests pin the widened face by its ABI symbols: the
// two-word struct return of the value-to-string family, the runtime's
// comparison and access calls, and the two places the emitter spends no
// call at all (byteLength is the pair's length word; a String hole is the
// already-live pair).
//
// Two of them pin defects rather than boundaries: a literal carrying holes
// must never bind its raw source text (decodeStringLiteral reads `$`, `{`
// and `}` as ordinary characters, so the un-widened path would bind
// "a${n}b" verbatim), and a hole's value must be computed once, at the
// binding that names it, not at each use.

// interpLit builds one interpolated literal: the literal token's raw text
// (what the description tower reads — the emission never touches it), the
// decoded runs between the holes (len(segs) == len(holes)+1) and the hole
// expressions.
func interpLit(segs []string, holes ...ast.Expr) *ast.Literal {
	return &ast.Literal{Kind: "string", Text: `"…"`, Segs: segs, Holes: holes}
}

// countCall counts the call sites of one runtime symbol — the declare line
// carries the name too, so the match is the call's own spelling.
func countCall(ir, ret, sym string) int {
	return strings.Count(ir, "= call "+ret+" @"+sym+"(")
}

// TestStringConcatEmitsTheRuntimeJoin: `a + b` over two String operands is
// the runtime's concatenation, whose result is the two-word struct — the
// named type renders with the record structs, and the words come back out
// through extractvalue.
func TestStringConcatEmitsTheRuntimeJoin(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "a", Init: strLit(`"ab"`)},
		&ast.Binding{Kw: "let", Name: "b", Init: strLit(`"cd"`)},
		&ast.Binding{Kw: "let", Name: "s", Init: binOp("+", ident("a"), ident("b"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "%struct.we_str = type { ptr, i64 }",
		"declare %struct.we_str @__we_str_concat(ptr, i64, ptr, i64)",
		"= call %struct.we_str @__we_str_concat(ptr @.s0, i64 2, ptr @.s1, i64 2)",
		"= extractvalue %struct.we_str",
	)
	if countCall(ir, "%struct.we_str", "__we_str_concat") != 1 {
		t.Fatalf("one concat per `+`:\n%s", ir)
	}
	// The print takes the two extracted words — a register pair, where a
	// plain literal's would have been a constant global and an immediate.
	if m := regexp.MustCompile(`call void %v\d+\(ptr %v\d+, i64 %v\d+\)`).FindString(ir); m == "" {
		t.Fatalf("the joined pair reaches the print:\n%s", ir)
	}
}

// TestStringConcatChainsLeftToRight: `a + b + c` is one concat per
// operator, so a three-operand join is two calls.
func TestStringConcatChainsLeftToRight(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "a", Init: strLit(`"a"`)},
		&ast.Binding{Kw: "let", Name: "b", Init: strLit(`"b"`)},
		&ast.Binding{Kw: "let", Name: "c", Init: strLit(`"c"`)},
		&ast.Binding{Kw: "let", Name: "s", Init: binOp("+",
			binOp("+", ident("a"), ident("b")), ident("c"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "= call %struct.we_str @__we_str_concat(")
	if got := countCall(ir, "%struct.we_str", "__we_str_concat"); got != 2 {
		t.Fatalf("two concats for `a + b + c`, got %d:\n%s", got, ir)
	}
}

// TestNumericPlusStaysNumeric: the concatenation arm is gated on both
// operands being Strings, so the integer family keeps its checked add.
func TestNumericPlusStaysNumeric(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "n", Init: binOp("+", intLit("1"), intLit("2"))},
		ioCall("io", "println", ident("n")),
		okReturn(),
	), "llvm.sadd.with.overflow.i64")
	if strings.Contains(ir, "__we_str_") {
		t.Fatalf("an integer add reaches no String symbol:\n%s", ir)
	}
}

// TestInterpolationRendersBaseTypes pins the value domain and the
// converter each kind takes: the integer family through the i64 view
// (UInt64 alone unsigned), Float64 through the double, Bool and Rune
// through their own words.
func TestInterpolationRendersBaseTypes(t *testing.T) {
	cases := []struct {
		name  string
		prep  ast.Stmt
		hole  ast.Expr
		wants string
	}{
		{
			"int64",
			&ast.Binding{Kw: "let", Name: "n", Init: intLit("7")},
			ident("n"),
			"= call %struct.we_str @__we_str_of_i64(i64 7)",
		},
		{
			"uint64",
			&ast.Binding{Kw: "let", Name: "n", Typ: named("UInt64"), Init: intLit("7")},
			ident("n"),
			"= call %struct.we_str @__we_str_of_u64(i64 7)",
		},
		{
			"uint8",
			&ast.Binding{Kw: "let", Name: "n", Typ: named("UInt8"), Init: intLit("7")},
			ident("n"),
			"= call %struct.we_str @__we_str_of_i64(i64 7)",
		},
		{
			"bool",
			&ast.Binding{Kw: "let", Name: "f", Init: boolLit("true")},
			ident("f"),
			"= call %struct.we_str @__we_str_of_bool(i64 1)",
		},
		{
			"rune",
			&ast.Binding{Kw: "let", Name: "r", Init: runeLit(`'a'`)},
			ident("r"),
			"= call %struct.we_str @__we_str_of_rune(i64 97)",
		},
		{
			"float64",
			&ast.Binding{Kw: "let", Name: "x", Init: floatLit("1.5")},
			ident("x"),
			"= call %struct.we_str @__we_str_of_f64(double 0x3FF8000000000000)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ir := assertClean(t, m9bModule(
				tc.prep,
				&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"", ""}, tc.hole)},
				ioCall("io", "println", ident("m")),
				okReturn(),
			), tc.wants)
			// A single hole with empty runs on both sides joins nothing:
			// the rendered value is the whole string.
			if strings.Contains(ir, "__we_str_concat") {
				t.Fatalf("a lone hole needs no concat:\n%s", ir)
			}
		})
	}
}

// TestInterpolationStringHoleIsIdentity: a String hole is the value's own
// pair — design D3's String identity, no converter and no copy.
func TestInterpolationStringHoleIsIdentity(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"ab"`)},
		&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"<", ">"}, ident("s"))},
		ioCall("io", "println", ident("m")),
		okReturn(),
	), "= call %struct.we_str @__we_str_concat(ptr @.s0, i64 1, ptr @.s1, i64 2)",
	)
	if strings.Contains(ir, "__we_str_of_") {
		t.Fatalf("a String hole is the identity:\n%s", ir)
	}
}

// TestInterpolationJoinsAdjacentHoles: the runs between holes contribute
// nothing when they are empty, so `"${a}${b}"` is one join, not three.
func TestInterpolationJoinsAdjacentHoles(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "n", Init: intLit("1")},
		&ast.Binding{Kw: "let", Name: "m", Init: intLit("2")},
		&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"", "", ""},
			ident("n"), ident("m"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "= call %struct.we_str @__we_str_concat(")
	if got := countCall(ir, "%struct.we_str", "__we_str_concat"); got != 1 {
		t.Fatalf("adjacent holes join once, got %d:\n%s", got, ir)
	}
	if strings.Contains(ir, `c""`) {
		t.Fatalf("an empty run is interned as a constant:\n%s", ir)
	}
}

// TestInterpolationNeverBindsRawText: the literal's raw source is what a
// description reads; the value is the rendered join. Binding the raw text
// would make `"a${n}b"` the four-character string `a${n}b`.
func TestInterpolationNeverBindsRawText(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "n", Init: intLit("7")},
		&ast.Binding{Kw: "let", Name: "m", Init: &ast.Literal{
			Kind: "string", Text: `"a${n}b"`,
			Segs:  []string{"a", "b"},
			Holes: []ast.Expr{ident("n")},
		}},
		ioCall("io", "println", ident("m")),
		okReturn(),
	), `c"a"`, `c"b"`, "= call %struct.we_str @__we_str_of_i64(i64 7)")
	if strings.Contains(ir, "${") {
		t.Fatalf("the raw source text reached the constants:\n%s", ir)
	}
}

// TestInterpolationHoleEmitsAtTheBinding: a hole's expression is computed
// where the literal is bound — its runtime buffers are allocated there —
// so naming the binding twice reads the same pair instead of rerunning the
// holes.
func TestInterpolationHoleEmitsAtTheBinding(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "n", Init: intLit("7")},
		&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"n=", ""}, ident("n"))},
		ioCall("io", "println", ident("m")),
		ioCall("io", "println", ident("m")),
		okReturn(),
	), "= call %struct.we_str @__we_str_of_i64(i64 7)")
	if got := countCall(ir, "%struct.we_str", "__we_str_of_i64"); got != 1 {
		t.Fatalf("the hole runs once at its binding, got %d:\n%s", got, ir)
	}
}

// TestInterpolationDiscardStillRuns: `let _ = "…"` discards the pair but
// not the holes — a hole may call, and a call has effects.
func TestInterpolationDiscardStillRuns(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "n", Init: intLit("7")},
		&ast.Binding{Kw: "let", Name: "_", Init: interpLit([]string{"", ""}, ident("n"))},
		okReturn(),
	), "= call %struct.we_str @__we_str_of_i64(i64 7)")
	if strings.Contains(ir, "__we_str_concat") {
		t.Fatalf("a lone hole needs no concat:\n%s", ir)
	}
	if !strings.Contains(ir, "ret i32 0") {
		t.Fatalf("the discarded literal still emits:\n%s", ir)
	}
}

// TestStringEqualityUsesTheRuntime: `==` over two String operands is the
// byte comparison, widened into the i64 domain like every other boolean
// (chapter 10 compares base types; String is one).
func TestStringEqualityUsesTheRuntime(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "a", Init: strLit(`"ab"`)},
		&ast.Binding{Kw: "let", Name: "b", Init: strLit(`"cd"`)},
		&ast.Binding{Kw: "let", Name: "same", Init: binOp("==", ident("a"), ident("b"))},
		ioCall("io", "println", ident("same")),
		okReturn(),
	), "declare i64 @__we_str_eq(ptr, i64, ptr, i64)",
		"= call i64 @__we_str_eq(ptr @.s0, i64 2, ptr @.s1, i64 2)",
		"icmp ne i64",
		"zext i1",
	)
	// The predicate tests the runtime's answer against zero — never a
	// pointer word or a length against another length — and `==` is that
	// answer being non-zero (the runtime answers 1 when the bytes agree).
	if m := regexp.MustCompile(`icmp ne i64 %v\d+, 0`).FindString(ir); m == "" {
		t.Fatalf("the equality compares the runtime's answer:\n%s", ir)
	}
}

// TestStringInequalityIsNe: `!=` is the same call under the other
// predicate — the runtime's answer being zero.
func TestStringInequalityIsNe(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "a", Init: strLit(`"ab"`)},
		&ast.Binding{Kw: "let", Name: "b", Init: strLit(`"ab"`)},
		&ast.Binding{Kw: "let", Name: "diff", Init: binOp("!=", ident("a"), ident("b"))},
		ioCall("io", "println", ident("diff")),
		okReturn(),
	), "= call i64 @__we_str_eq(", "icmp eq i64")
}

// TestStringEqualityBranchesInIf: the comparison is an i64-domain
// condition, so an if over it takes the ordinary conditional branch.
func TestStringEqualityBranchesInIf(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "a", Init: strLit(`"ab"`)},
		&ast.Binding{Kw: "let", Name: "b", Init: strLit(`"cd"`)},
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", ident("a"), ident("b")),
			Then: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"same"`))}},
			Else: blockOf(ioCall("io", "println", strLit(`"diff"`))),
		}},
		okReturn(),
	), "= call i64 @__we_str_eq(", "br i1")
}

// TestByteLengthIsTheLengthWord: the pair carries the byte count, so
// byteLength is an identity on the second word — no call, and a program
// that only asks for lengths declares no member of the family at all.
func TestByteLengthIsTheLengthWord(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "n", Init: callOn(ident("s"), "byteLength")},
		ioCall("io", "println", ident("n")),
		okReturn(),
	), "call void @__we_println_i64(i64 3)")
	if strings.Contains(ir, "__we_str_") {
		t.Fatalf("byteLength spends no call:\n%s", ir)
	}
}

// TestStringAccessMembersCallTheRuntime: runeCount and charAt walk code
// points, byteSlice spans bytes; each takes the receiver's pair first.
func TestStringAccessMembersCallTheRuntime(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "n", Init: callOn(ident("s"), "runeCount")},
		ioCall("io", "println", ident("n")),
		okReturn(),
	), "declare i64 @__we_str_runecount(ptr, i64)",
		"= call i64 @__we_str_runecount(ptr @.s0, i64 3)")

	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "n", Init: callOn(ident("s"), "runeCount")},
		ioCall("io", "println", ident("n")),
		okReturn(),
	), "declare i64 @__we_str_runecount(ptr, i64)",
		"= call i64 @__we_str_runecount(ptr @.s0, i64 3)")

	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "c", Init: callOn(ident("s"), "charAt", intLit("1"))},
		ioCall("io", "println", ident("c")),
		okReturn(),
	), "declare i64 @__we_str_charat(ptr, i64, i64)",
		"= call i64 @__we_str_charat(ptr @.s0, i64 3, i64 1)")

	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "t", Init: callOn(ident("s"), "byteSlice", intLit("0"), intLit("2"))},
		ioCall("io", "println", ident("t")),
		okReturn(),
	), "declare %struct.we_str @__we_str_byteslice(ptr, i64, i64, i64)",
		"= call %struct.we_str @__we_str_byteslice(ptr @.s0, i64 3, i64 0, i64 2)",
		"= extractvalue %struct.we_str",
	)
}

// TestCharAtRendersAsRune: the member's result type is the declaration's
// (charAt yields a Rune), so a hole of it renders the code point's
// character rather than its number.
func TestCharAtRendersAsRune(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "c", Init: callOn(ident("s"), "charAt", intLit("1"))},
		&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"<", ">"}, ident("c"))},
		ioCall("io", "println", ident("m")),
		okReturn(),
	), "= call i64 @__we_str_charat(",
		"= call %struct.we_str @__we_str_of_rune(i64 ")
	if strings.Contains(ir, "__we_str_of_i64") {
		t.Fatalf("a Rune renders as a character:\n%s", ir)
	}
}

// TestRuneCountRendersAsInt: the other access member's result is an Int64,
// so the same hole renders the count.
func TestRuneCountRendersAsInt(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "n", Init: callOn(ident("s"), "runeCount")},
		&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"n=", ""}, ident("n"))},
		ioCall("io", "println", ident("m")),
		okReturn(),
	), "= call %struct.we_str @__we_str_of_i64(i64 ")
}

// TestInterpolationOverMemberChain: a String field's pair rides the chain
// the same way the print face reads it.
func TestInterpolationOverMemberChain(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		stdIoImport("io"),
		appError(),
		&ast.RecordDecl{
			Pub: true, Cat: "gc", Name: "Point",
			Fields: []ast.FieldDecl{{Name: "label", Typ: &ast.NamedType{Name: "String"}}},
		},
		mainDecl(
			&ast.Binding{Kw: "let", Name: "p", Init: &ast.Construct{
				Name:   "Point",
				Fields: []ast.FieldInit{{Name: "label", Value: strLit(`"ab"`)}},
			}},
			&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"<", ">"},
				&ast.Member{Recv: ident("p"), Name: "label"})},
			ioCall("io", "println", ident("m")),
			okReturn(),
		),
	}}
	ir := assertClean(t, f, "= call %struct.we_str @__we_str_concat(")
	if strings.Contains(ir, "__we_str_of_") {
		t.Fatalf("a String field is the identity:\n%s", ir)
	}
}

// TestHoleRendersAStringValuedForm: the ruled domain is the base types and
// String, and a hole renders its value through the value-to-string family.
// A value-position control form whose arms all answer String is inside that
// set — the form's sink carries the String pair (a ptr word and a length
// word), the arms join on it, and the hole renders the form as itself: a
// String needs no converter, so the IR carries no str_of call at all.
func TestHoleRendersAStringValuedForm(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"", ""},
			&ast.If{
				Cond: binOp(">", intLit("1"), intLit("0")),
				Then: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: strLit(`"a"`)}}},
				Else: blockOf(&ast.ExprStmt{Expr: strLit(`"b"`)}),
			})},
		ioCall("io", "println", ident("m")),
		okReturn(),
	)
	ir := assertClean(t, f)
	if strings.Contains(ir, "__we_str_of_") {
		t.Fatalf("a String-valued form is the identity:\n%s", ir)
	}
}

// TestValueFormClassifiesInTheBlocksOwnFrame: the block arm answers in the
// block's own frame — blockKind installs it — rather than the outer one the
// classifier happens to run in. The program below is well typed — the
// block's `b` is its own, the checker scopes it per block exactly as the
// emitter does — and reading the outer frame instead would classify the
// tail as skBool, rendering the inner 5 through the bool converter and
// printing "true". Every integer family is one i64 word, so nothing
// downstream could tell; the converter the rendering actually names is the
// pin.
func TestValueFormClassifiesInTheBlocksOwnFrame(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "let", Name: "b", Init: boolLit("true")},
		&ast.Binding{Kw: "let", Name: "k", Init: blockOf(
			&ast.Binding{Kw: "let", Name: "b", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("5")},
			&ast.ExprStmt{Expr: ident("b")})},
		&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"", ""}, ident("k"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	)
	ir := assertClean(t, f, "= call %struct.we_str @__we_str_of_i64(")
	if strings.Contains(ir, "__we_str_of_bool") {
		t.Fatalf("the inner b is an Int64:\n%s", ir)
	}
}

// TestBlockKindLeavesTheOuterFrameAlone: the other half of the frame rule.
// Installing the block's faces must not touch the bindings the block is
// nested in — the frame blockKind opens is the block's own, and the outer
// `b` keeps its bool face after the block is classified. A classifier that
// wrote into the outer frame would flip the second rendering to the i64
// converter, and the program would print the inner 5 twice.
func TestBlockKindLeavesTheOuterFrameAlone(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "let", Name: "b", Init: boolLit("true")},
		&ast.Binding{Kw: "let", Name: "k", Init: blockOf(
			&ast.Binding{Kw: "let", Name: "b", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("5")},
			&ast.ExprStmt{Expr: ident("b")})},
		&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"", ""}, ident("k"))},
		&ast.Binding{Kw: "let", Name: "t", Init: interpLit([]string{"", ""}, ident("b"))},
		ioCall("io", "println", ident("s")),
		ioCall("io", "println", ident("t")),
		okReturn(),
	)
	ir := assertClean(t, f,
		"= call %struct.we_str @__we_str_of_i64(",
		"= call %struct.we_str @__we_str_of_bool(")
	if n := countCall(ir, "%struct.we_str", "__we_str_of_bool"); n != 1 {
		t.Fatalf("the outer b renders once as a bool, got %d:\n%s", n, ir)
	}
}

// TestValueFormStringFaceRenders: the other side of the same rule. With the
// value-position forms classified, an unannotated `let` bound from one is a
// slot the string chain can read — the hole renders it through the i64
// converter, once, at the binding that names it. Before the arm, valueKind
// answered skNone for the form and this program stopped at bndMainBody.
func TestValueFormStringFaceRenders(t *testing.T) {
	form := &ast.If{
		Cond: binOp(">", ident("a"), intLit("0")),
		Then: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: intLit("10")}}},
		Else: blockOf(&ast.ExprStmt{Expr: intLit("20")}),
	}
	ir := assertClean(t, m9bModule(
		typedLit("a", "1"),
		&ast.Binding{Kw: "let", Name: "n", Init: form},
		&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"n=", ""}, ident("n"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "= call %struct.we_str @__we_str_of_i64(")
	if n := countCall(ir, "%struct.we_str", "__we_str_of_i64"); n != 1 {
		t.Fatalf("expected one converter at the binding, got %d:\n%s", n, ir)
	}
}

// TestJoinKindCarriesAStringPair: the form's sink carries two faces now —
// a numeric word or a String pair — and the join admits a pair of String
// arms alongside the numeric families it always admitted. Mixed faces
// still answer nothing: no single slot can hold both, and the checker has
// already declined the program the mix would spell.
func TestJoinKindCarriesAStringPair(t *testing.T) {
	if k := joinKind(skStr, skStr); k != skStr {
		t.Fatalf("two String arms join, got %v", k)
	}
	if k := joinKind(skI64, skStr); k != skNone {
		t.Fatalf("a mixed pair answers nothing, got %v", k)
	}
	if k := joinKind(skStr, skI64); k != skNone {
		t.Fatalf("a mixed pair answers nothing, got %v", k)
	}
	if k := joinKind(skF64, skF64); k != skF64 {
		t.Fatalf("the numeric families join as before, got %v", k)
	}
}

// TestValueFormMixedArmsStillStop: a control form whose arms answer
// different faces still stops at the body boundary. The checker rejects
// the program first (E0501 at the arms); this pins the emission's own
// join — the classifier built straight from the AST never sees the
// checker, and the join is where it answers the mix.
func TestValueFormMixedArmsStillStop(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"", ""},
			&ast.If{
				Cond: binOp(">", intLit("1"), intLit("0")),
				Then: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: intLit("1")}}},
				Else: blockOf(&ast.ExprStmt{Expr: strLit(`"a"`)}),
			})},
		ioCall("io", "println", ident("m")),
		okReturn(),
	)
	_, ni := Emit(f, "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("expected the body boundary, got %v", ni)
	}
}

// TestBlockKindPoisonsNamesItCannotFace: a block binding whose face the
// classifier cannot install — a pattern binding, or a plain binding whose
// initializer answers no single kind — poisons the name for the block's
// own classification rather than letting the outer binding leak through.
// The tail that reads such a name declines, so the form stops instead of
// rendering the outer face: a wrong value is the failure poisoning
// prevents.
func TestBlockKindPoisonsNamesItCannotFace(t *testing.T) {
	shapes := []struct {
		name string
		bind ast.Stmt
		tail ast.Expr
	}{
		{"pattern", &ast.Binding{Kw: "let", Pat: &ast.PatTuple{Elems: []ast.Pattern{
			&ast.PatBinding{Name: "x"}, &ast.PatBinding{Name: "y"},
		}}, Init: &ast.Tuple{Elems: []ast.Expr{intLit("1"), intLit("2")}}}, ident("x")},
		{"sum init", &ast.Binding{Kw: "let", Name: "v", Init: &ast.Call{Fn: ident("Some"), Args: []ast.Expr{intLit("1")}}}, ident("v")},
	}
	for _, s := range shapes {
		f := m9bModule(
			&ast.Binding{Kw: "let", Name: "x", Init: boolLit("true")},
			&ast.Binding{Kw: "let", Name: "k", Init: &ast.BlockExpr{Block: ast.Block{Items: []ast.Stmt{
				s.bind, &ast.ExprStmt{Expr: s.tail},
			}}}},
			&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"", ""}, ident("k"))},
			ioCall("io", "println", ident("s")),
			okReturn(),
		)
		_, ni := Emit(f, "demo")
		if ni == nil || ni.What != bndMainBody {
			t.Fatalf("%s: expected the body boundary, got %v", s.name, ni)
		}
	}
}

// TestCallStrKindAnswersAFnValueByName: a fn value bound under the callee
// name answers from the signature its binding saw — the classifier reads
// the same fnEnv the emission's call path dispatches through, so the
// value-string and operand positions classify together. A value whose
// signature the emitter never saw answers nothing, exactly as its emission
// does.
func TestCallStrKindAnswersAFnValueByName(t *testing.T) {
	e := &emitter{}
	e.fnEnv = map[string]fnValue{
		"seen":   {typed: true, abi: fnAbi{retName: "Int64"}},
		"unseen": {typed: false},
	}
	if k := e.callStrKind(&ast.Call{Fn: ident("seen")}); k != skI64 {
		t.Fatalf("a seen signature answers its return's family, got %v", k)
	}
	if k := e.callStrKind(&ast.Call{Fn: ident("unseen")}); k != skNone {
		t.Fatalf("an unseen signature answers nothing, got %v", k)
	}
}

// TestVarWithAnUnannotatedInitializerBinds: a var the annotation does not
// name takes its face from the initializer's own classification — the
// storage is laid for the face the init answered, and the assignment
// stores through it. The program prints the assigned value, not the
// initializer's.
func TestVarWithAnUnannotatedInitializerBinds(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "var", Name: "s", Init: strLit(`"a"`)},
		&ast.Assign{Name: "s", Value: strLit(`"b"`)},
		ioCall("io", "println", ident("s")),
		okReturn(),
	)
	assertClean(t, f)
}

// TestTupleValueRendersIntoAStringPosition: a tuple value read into a
// String position renders as its whole shape — each element loads out of
// the aggregate at its own offset and renders in its own face, the parts
// joined with the interned separator. The rendering is the element
// converters the IR actually names: one i64 call, no bool call, and the
// separator constant the join interned.
func TestTupleValueRendersIntoAStringPosition(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "let", Name: "t", Init: &ast.Tuple{
			Elems: []ast.Expr{intLit("1"), intLit("2")}}},
		&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"", ""}, ident("t"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	)
	ir := assertClean(t, f)
	if n := countCall(ir, "%struct.we_str", "__we_str_of_i64"); n != 2 {
		t.Fatalf("each i64 element renders through its own converter, got %d:\n%s", n, ir)
	}
	if strings.Contains(ir, "__we_str_of_bool") {
		t.Fatalf("no element is a bool:\n%s", ir)
	}
	if !strings.Contains(ir, `c", "`) {
		t.Fatalf("the join interns the two-character separator:\n%s", ir)
	}
}

// TestStringIteratorMemberStops: `.iterator()` is an iteration protocol of
// its own (the for-in source rides it directly); as a member call it stays
// outside the set.
func TestStringIteratorMemberStops(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "let", Name: "s", Init: strLit(`"abc"`)},
		&ast.Binding{Kw: "let", Name: "it", Init: callOn(ident("s"), "iterator")},
		okReturn(),
	)
	_, ni := Emit(f, "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("expected the body boundary, got %v", ni)
	}
}

// TestStringValueReturningCallInHoleStops: a call the emitter cannot
// resolve stops like any other unknown — the classification is the callee
// declaration's, and an unresolved name has none.
func TestUnknownCallInHoleStops(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "let", Name: "m", Init: interpLit([]string{"", ""},
			&ast.Call{Fn: ident("mystery")})},
		ioCall("io", "println", ident("m")),
		okReturn(),
	)
	_, ni := Emit(f, "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("expected the body boundary, got %v", ni)
	}
}

// TestConcatAsIoArgumentIsNotScalar pins a defect the value programs
// caught: the println shortcut classified every Binary argument as a
// numeric form, so `io.println(a + b)` handed a String join to the i64
// renderer and stopped at the boundary. The classification decides, and a
// String join takes the byte-pair print.
func TestConcatAsIoArgumentIsNotScalar(t *testing.T) {
	ir := assertClean(t, m9bModule(
		ioCall("io", "println", binOp("+", strLit(`"ab"`), strLit(`"cd"`))),
		okReturn(),
	), "= call %struct.we_str @__we_str_concat(ptr @.s0, i64 2, ptr @.s1, i64 2)")
	if strings.Contains(ir, "__we_println_i64") {
		t.Fatalf("a String join is not an i64 render:\n%s", ir)
	}
	if m := regexp.MustCompile(`call void %v\d+\(ptr %v\d+, i64 %v\d+\)`).FindString(ir); m == "" {
		t.Fatalf("the joined pair reaches the print:\n%s", ir)
	}
}

// TestUInt64LiteralCarriesItsSuffix: a `u64` literal classifies unsigned
// on its own — no annotation needed — so its hole takes the unsigned
// renderer.
func TestUInt64LiteralCarriesItsSuffix(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "u", Init: intLit("7u64")},
		&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"u=", ""}, ident("u"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "= call %struct.we_str @__we_str_of_u64(i64 7)")
}

// TestUInt64LiteralPastI64KeepsItsBits: the largest UInt64 has no positive
// i64 reading, so its operand is the two's-complement one — the same
// register bits, rendered by the unsigned view in full.
func TestUInt64LiteralPastI64KeepsItsBits(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "u", Init: intLit("18446744073709551615u64")},
		&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"u=", ""}, ident("u"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "= call %struct.we_str @__we_str_of_u64(i64 -1)")
	if strings.Contains(ir, "__we_str_of_i64") {
		t.Fatalf("an unsigned literal never takes the signed renderer:\n%s", ir)
	}
}
