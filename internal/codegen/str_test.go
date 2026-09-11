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

// TestHoleOutsideTheDomainStops: the ruled domain is the base types and
// String, and a hole renders its value through the value-to-string family.
// A value-position control form whose arms answer String is outside that
// set — the form's result slot is the numeric one (valueForm.put takes only
// ckI64), so the join has no domain to classify and the hole stops at the
// body boundary rather than guessing a converter. The control form itself
// is classified (TestValueFormStringFaceRenders); it is its String arms
// that stay out.
func TestHoleOutsideTheDomainStops(t *testing.T) {
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
	_, ni := Emit(f, "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("expected the body boundary, got %v", ni)
	}
}

// TestValueFormDeclinesABlockThatBinds: the block arm declines a block that
// binds a name before its tail, because the classifier reads the frame the
// block is classified in and the emission has already restored it. The
// program below is well typed — the block's `b` is its own, the checker
// scopes it per block exactly as the emitter does — and answering skBool
// from the outer `b` would render the inner 5 through the bool converter
// and print "true". Every integer family is one i64 word, so nothing
// downstream could tell; the boundary is the answer.
func TestValueFormDeclinesABlockThatBinds(t *testing.T) {
	f := m9bModule(
		&ast.Binding{Kw: "let", Name: "b", Init: boolLit("true")},
		&ast.Binding{Kw: "let", Name: "k", Init: blockOf(
			&ast.Binding{Kw: "let", Name: "b", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("5")},
			&ast.ExprStmt{Expr: ident("b")})},
		&ast.Binding{Kw: "let", Name: "s", Init: interpLit([]string{"", ""}, ident("k"))},
		ioCall("io", "println", ident("s")),
		okReturn(),
	)
	_, ni := Emit(f, "demo")
	if ni == nil || ni.What != bndMainBody {
		t.Fatalf("expected the body boundary, got %v", ni)
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
