package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T3 expression value forms (design D2). Red before the task: every shape
// below stopped at the body boundary word on the real toolchain — the
// numeric expression set held literals, names, the checked + - *, the six
// comparisons and the i64 and/or, so `/`, `%`, the unary family, the
// value-position if/match/block, the operand-position call and the rune
// literal all fell through to bndMainBody (check green, build 70). Two of
// these tests pin defects rather than boundaries: scope's value form read
// its tail before the body (design D10-2: legal IR, wrong value), and
// `&&`/`||` evaluated both operands (D10-3: safe only while no operand
// could have an effect, which the call arm ends).
//
// The value forms follow the result-alloca mode: one slot per expression,
// each arm stores its value, the join loads it. The slot is reserved by
// the first arm that produces a value — an alloca's text still splices in
// behind the entry label, so where in the body it is reserved is
// unobservable — and every later arm writes that same slot. An expression
// whose arms produce no value is unit and reserves no slot at all.

// runeLit builds the rune literal `'x'` (the source form, quotes in).
func runeLit(text string) *ast.Literal { return &ast.Literal{Kind: "rune", Text: text} }

// floatLit builds the float literal `1.5` (an operand).
func floatLit(text string) *ast.Literal { return &ast.Literal{Kind: "float", Text: text} }

// floatBind builds `let name = <float literal>`.
func floatBind(name, text string) *ast.Binding {
	return &ast.Binding{Kw: "let", Name: name, Init: floatLit(text)}
}

// unary builds one prefix application `op x` (chapter 2's unary level).
func unary(op string, x ast.Expr) *ast.Unary { return &ast.Unary{Op: op, X: x} }

// ifVal builds `let name = if cond { then } else { els }` where both arms
// carry one expression item — the block value the parser leaves as the
// block's last item.
func ifVal(name string, cond ast.Expr, thenV, elseV ast.Expr) *ast.Binding {
	return &ast.Binding{Kw: "let", Name: name, Init: &ast.If{
		Cond: cond,
		Then: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: thenV}}},
		Else: blockOf(&ast.ExprStmt{Expr: elseV}),
	}}
}

// valBlock builds `{ stmts… tail }` — a block whose last item is the
// expression that carries its value.
func valBlock(tail ast.Expr, stmts ...ast.Stmt) *ast.BlockExpr {
	items := append(append([]ast.Stmt{}, stmts...), &ast.ExprStmt{Expr: tail})
	return &ast.BlockExpr{Block: ast.Block{Items: items}}
}

// firstSlot names the register of the body's first stack slot — the one
// the first binding that needs storage reserves (a `var`'s home).
func firstSlot(t *testing.T, ir string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^  (%v\d+) = alloca i64$`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("no i64 slot in:\n%s", ir)
	}
	return m[1]
}

// slotReadsBeforeWrite pins the D10-2 order for one slot: the last read
// of it must come after the last write to it. The defect reads the value
// before the body that produces it (legal IR, wrong value), so the
// assertion is exactly "the tail's read follows the body's write".
func slotReadsBeforeWrite(t *testing.T, ir, slot string) {
	t.Helper()
	read, write := -1, -1
	for i, ln := range strings.Split(ir, "\n") {
		if strings.Contains(ln, "= load i64, ptr "+slot) {
			read = i
		}
		if strings.Contains(ln, "store i64") && strings.HasSuffix(ln, "ptr "+slot) {
			write = i
		}
	}
	if read < 0 || write < 0 {
		t.Fatalf("slot %s is not read and written:\n%s", slot, ir)
	}
	if read < write {
		t.Fatalf("slot %s is read at line %d, before its write at line %d — the tail runs before the body:\n%s",
			slot, read, write, ir)
	}
}

// TestIntDivisionGuardsZeroDivisor: a division by a value the emitter
// cannot see is guarded — the zero test branches to the panic tail, and
// only the checked path reaches the sdiv.
func TestIntDivisionGuardsZeroDivisor(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("b", "2"),
		&ast.Binding{Kw: "let", Name: "q", Init: binOp("/", intLit("10"), ident("b"))},
		okReturn(),
	), "sdiv i64")
	order(t, ir,
		"icmp eq i64",               // the divisor tested against zero
		"br i1",                     // to the panic tail or the division
		"call void @__we_task_fail", // the zero-divisor trap
		"unreachable",
		"sdiv i64", // the checked path's division
	)
	if !strings.Contains(ir, `c"division by zero\00"`) {
		t.Fatalf("the trap names the operation:\n%s", ir)
	}
}

// TestIntDivisionGuardsSignedOverflow: LLVM's sdiv is undefined for the
// one overflowing pair (the minimum value by -1); chapter 7's checked
// semantics let no operation wrap or fall to undefined behaviour, so the
// pair traps like the + - * family does. The report names the operation
// (chapter 14's `Int64 add overflow` and siblings), so the division
// carries its own text rather than the add family's.
func TestIntDivisionGuardsSignedOverflow(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("b", "2"),
		&ast.Binding{Kw: "let", Name: "q", Init: binOp("/", intLit("10"), ident("b"))},
		okReturn(),
	), "sdiv i64")
	if !strings.Contains(ir, "-9223372036854775808") {
		t.Fatalf("the overflow guard tests the minimum value:\n%s", ir)
	}
	if !strings.Contains(ir, `c"Int64 div overflow\00"`) {
		t.Fatalf("the overflow trap names the operation:\n%s", ir)
	}
}

// TestIntDivisionConstantDivisorIsBare: a divisor the source spells and
// excludes from {0, -1} needs no guard, so the emission is the division
// alone — the shape the benchmark tasks' `n % 2` and `n / 2` bodies take.
// A spelled zero takes the guard instead: chapter 7's E0502 covers
// overflow alone, and the folder declines an undefined evaluation rather
// than reporting one, so the runtime trap is a zero divisor's only
// capture face.
func TestIntDivisionConstantDivisorIsBare(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("n", "7"),
		&ast.Binding{Kw: "let", Name: "q", Init: binOp("/", ident("n"), intLit("2"))},
		&ast.Binding{Kw: "let", Name: "m", Init: binOp("%", ident("n"), intLit("2"))},
		okReturn(),
	), "sdiv i64", "srem i64")
	if strings.Contains(ir, "__we_task_fail") {
		t.Fatalf("a spelled divisor needs no guard:\n%s", ir)
	}
}

// TestIntModuloGuardsZeroDivisor: `%` carries the same zero-divisor trap
// as `/`, and the one overflowing pair — whose remainder is exactly zero,
// so nothing overflows — rides a divisor of one instead of a branch: `a
// srem -1` is 0, and so is `a srem 1`.
func TestIntModuloGuardsZeroDivisor(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("b", "2"),
		&ast.Binding{Kw: "let", Name: "m", Init: binOp("%", intLit("10"), ident("b"))},
		okReturn(),
	), "srem i64")
	order(t, ir,
		"icmp eq i64",
		"br i1",
		"call void @__we_task_fail",
		"unreachable",
		"select i1",
		"srem i64",
	)
	if !strings.Contains(ir, `c"division by zero\00"`) {
		t.Fatalf("the trap names the operation:\n%s", ir)
	}
}

// TestFloatDivisionIsFdiv: floats ride IEEE arithmetic (chapter 7: no
// overflow check), so `/` is fdiv — no trap face — and `%` its fmod.
func TestFloatDivisionIsFdiv(t *testing.T) {
	ir := assertClean(t, m9bModule(
		floatBind("b", "2.0"),
		&ast.Binding{Kw: "let", Name: "q", Init: binOp("/", floatLit("3.0"), ident("b"))},
		okReturn(),
	), "fdiv double")
	if strings.Contains(ir, "__we_task_fail") {
		t.Fatalf("float division has no trap face:\n%s", ir)
	}
}

// TestFloatModuloIsFrem: the remainder keeps its domain's instruction.
func TestFloatModuloIsFrem(t *testing.T) {
	assertClean(t, m9bModule(
		floatBind("b", "2.0"),
		&ast.Binding{Kw: "let", Name: "m", Init: binOp("%", floatLit("3.0"), ident("b"))},
		okReturn(),
	), "frem double")
}

// TestUnaryNot: `!` is the zero test the boolean domain makes it — and
// the result widens back into the i64-domain boolean the binding carries.
func TestUnaryNot(t *testing.T) {
	ir := assertClean(t, m9bModule(
		typedLit("a", "1"),
		&ast.Binding{Kw: "let", Name: "n", Init: unary("!", binOp("==", ident("a"), intLit("1")))},
		okReturn(),
	), "icmp eq i64", "zext i1")
	order(t, ir, "icmp eq i64", "zext i1")
}

// TestUnaryNegate: `-x` is the checked subtraction from zero — the
// intrinsic and the panic tail of the + - * family, because negating the
// minimum value overflows (chapter 7).
func TestUnaryNegate(t *testing.T) {
	ir := assertClean(t, m9bModule(
		typedLit("a", "1"),
		&ast.Binding{Kw: "let", Name: "n", Init: unary("-", ident("a"))},
		okReturn(),
	), "llvm.ssub.with.overflow.i64")
	order(t, ir,
		"call { i64, i1 } @llvm.ssub.with.overflow.i64(i64 0,",
		"call void @__we_task_fail",
	)
}

// TestUnaryBitNot: `~x` is the all-ones xor — total, so no trap face.
func TestUnaryBitNot(t *testing.T) {
	ir := assertClean(t, m9bModule(
		typedLit("a", "1"),
		&ast.Binding{Kw: "let", Name: "n", Init: unary("~", ident("a"))},
		okReturn(),
	), "xor i64", "-1")
	if strings.Contains(ir, "__we_task_fail") {
		t.Fatalf("a bitwise complement cannot overflow:\n%s", ir)
	}
}

// TestRuneLiteralValue: a rune literal is an i64-domain code point
// (design D2: Rune has no separate storage form), escapes decoded under
// chapter 1's closed escape set.
func TestRuneLiteralValue(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "var", Name: "c", Typ: named("Int64"), Init: runeLit("'a'")},
		&ast.Binding{Kw: "var", Name: "nl", Typ: named("Int64"), Init: runeLit(`'\n'`)},
		&ast.Binding{Kw: "var", Name: "u", Typ: named("Int64"), Init: runeLit(`'\u{48}'`)},
		okReturn(),
	), "i64 97", "i64 10", "i64 72")
}

// TestFloatLetBindingKeepsDomain pins the D10-1 repair: a float the body
// binds keeps its domain, so the next use consumes a double rather than
// mistyping it as an i64. The pre-fix SSA face dropped isFloat, and the
// following float operation then stopped at the body boundary (the design
// predicted invalid IR; what the tree did was refuse the program at the
// mixed-domain check — see the completion record's disclosure).
func TestFloatLetBindingKeepsDomain(t *testing.T) {
	ir := assertClean(t, m9bModule(
		floatBind("h", "1.5"),
		&ast.Binding{Kw: "let", Name: "k", Init: binOp("+", ident("h"), floatLit("0.5"))},
		&ast.Binding{Kw: "let", Name: "r", Init: binOp("==", ident("k"), floatLit("2.0"))},
		okReturn(),
	), "fadd double", "fcmp oeq double")
	if strings.Contains(ir, "add i64") || strings.Contains(ir, "icmp eq i64") {
		t.Fatalf("the bound float stayed in the i64 domain:\n%s", ir)
	}
}

// TestValueIfResultAlloca: the value-position if stores each arm's value
// into one result slot and the join loads it — the value the rest of the
// body then consumes.
func TestValueIfResultAlloca(t *testing.T) {
	ir := assertClean(t, m9bModule(
		typedLit("a", "3"),
		ifVal("k", binOp(">", ident("a"), intLit("1")), intLit("10"), intLit("20")),
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", ident("k"), intLit("10")),
			Then: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"k-ten"`))}},
		}},
		okReturn(),
	), "ifthen0", "ifelse0", "ifjoin0")
	order(t, ir,
		"store i64 10, ptr", // the then arm's value
		"br label %ifjoin0",
		"store i64 20, ptr", // the else arm's value
		"ifjoin0:",
		"load i64, ptr", // the join reads the result
	)
}

// TestValueIfFloatAlloca: the result slot carries the domain the arms
// compute in — a double slot for a float-valued if.
func TestValueIfFloatAlloca(t *testing.T) {
	ir := assertClean(t, m9bModule(
		typedLit("a", "3"),
		ifVal("k", binOp(">", ident("a"), intLit("1")), floatLit("1.5"), floatLit("2.5")),
		okReturn(),
	), "alloca double")
	order(t, ir,
		"store double 0x3FF8000000000000, ptr", // 1.5
		"br label %ifjoin0",
		"store double 0x4004000000000000, ptr", // 2.5
		"ifjoin0:",
		"load double, ptr",
	)
}

// TestValueIfElseIfChain: an else-if chain is one value form — every arm
// of the chain stores into the same result slot.
func TestValueIfElseIfChain(t *testing.T) {
	inner := &ast.If{
		Cond: binOp("==", ident("a"), intLit("2")),
		Then: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: intLit("20")}}},
		Else: blockOf(&ast.ExprStmt{Expr: intLit("30")}),
	}
	ir := assertClean(t, m9bModule(
		typedLit("a", "3"),
		&ast.Binding{Kw: "let", Name: "k", Init: &ast.If{
			Cond: binOp(">", ident("a"), intLit("1")),
			Then: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{Expr: intLit("10")}}},
			Else: inner,
		}},
		okReturn(),
	), "ifthen0", "ifelse0")
	for _, v := range []string{
		"store i64 10, ptr", "store i64 20, ptr", "store i64 30, ptr",
	} {
		if strings.Count(ir, v) != 1 {
			t.Fatalf("each arm of the chain writes the result slot once, %q:\n%s", v, ir)
		}
	}
}

// TestValueUnitIfNoAlloca: an if whose arms produce no value is unit and
// reserves nothing — the arms run for their effect, the join carries on.
func TestValueUnitIfNoAlloca(t *testing.T) {
	ir := assertClean(t, m9bModule(
		typedLit("a", "3"),
		letDiscard(&ast.If{
			Cond: binOp(">", ident("a"), intLit("1")),
			Then: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"hi"`))}},
			Else: blockOf(ioCall("io", "println", strLit(`"ho"`))),
		}),
		okReturn(),
	), "ifthen0", "ifelse0")
	if strings.Contains(ir, "alloca") {
		t.Fatalf("a unit result reserves no slot:\n%s", ir)
	}
	if got := strings.Count(ir, "load ptr, ptr @slot.io.println"); got != 2 {
		t.Fatalf("both arms still run for their effect, got %d calls:\n%s", got, ir)
	}
}

// TestBlockValueBodyFirst: a block's value is its last expression,
// evaluated after the statements before it — `{ n = n + 1; n }` is the
// incremented value, not the one the block opened with.
func TestBlockValueBodyFirst(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("n", "0"),
		&ast.Binding{Kw: "let", Name: "l", Init: valBlock(ident("n"),
			assignTo("n", binOp("+", ident("n"), intLit("1"))))},
		okReturn(),
	))
	slotReadsBeforeWrite(t, ir, firstSlot(t, ir))
}

// TestValueMatchResultAlloca: the value-position match stores each arm's
// value into the result slot and the join loads it.
func TestValueMatchResultAlloca(t *testing.T) {
	ir := assertClean(t, m9bModule(
		chanBind("ch", "1"),
		&ast.Binding{Kw: "let", Name: "v", Init: callOn(ident("ch"), "receive")},
		&ast.Binding{Kw: "let", Name: "k", Init: &ast.Match{
			Scrutinee: ident("v"),
			Arms: []ast.MatchArm{
				{Pat: &ast.PatVariant{Name: "Some", Args: []ast.Pattern{&ast.PatBinding{Name: "x"}}},
					Body: valBlock(binOp("+", ident("x"), intLit("1")))},
				{Pat: &ast.PatVariant{Name: "None"},
					Body: valBlock(intLit("0"))},
			},
		}},
		okReturn(),
	), "marm0_0", "mjoin0")
	order(t, ir,
		"store i64", // the first arm's value
		"br label %mjoin0",
		"store i64", // the second arm's value
		"mjoin0:",
		"load i64, ptr",
	)
}

// TestCallInOperand: a user fn called from an operand position emits the
// call and the arithmetic consumes its result — the M8 emitter's call
// face reached from the numeric expression set (design D2).
func TestCallInOperand(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		stdIoImport("io"),
		appError(),
		pubFn("bump", []ast.Param{i64Param("n")}, named("Int64"),
			retValue(binOp("+", ident("n"), intLit("1")))),
		mainDecl(
			&ast.Binding{Kw: "let", Name: "v", Init: binOp("+", &ast.Call{Fn: ident("bump"), Args: []ast.Expr{intLit("5")}}, intLit("1"))},
			okReturn(),
		),
	}}
	ir := assertClean(t, f, "@slot.main.bump")
	order(t, ir,
		"load ptr, ptr @slot.main.bump",
		"call i64 %",
		"llvm.sadd.with.overflow.i64",
	)
}

// TestLogicShortCircuitSkipsRHS: `&&` and `||` branch — the deciding
// operand selects between the other operand's evaluation and the answer,
// so an operand that traps, calls, or panics runs only when the language
// says it does (design D2/D10-3).
func TestLogicShortCircuitSkipsRHS(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("d", "0"),
		&ast.Binding{Kw: "let", Name: "r", Init: binOp("&&",
			binOp("!=", ident("d"), intLit("0")),
			binOp("==", binOp("/", intLit("10"), ident("d")), intLit("1")))},
		okReturn(),
	), "lgc0rhs", "lgc0short", "lgc0join")
	rhs := blockAt(ir, "lgc0rhs")
	if rhs == "" {
		t.Fatalf("missing the right operand's block:\n%s", ir)
	}
	// The right operand's work begins in its own block — the divisor's
	// zero test is the first thing it does, and the division follows
	// under it. The short path carries neither.
	if !strings.Contains(rhs, "icmp eq i64") {
		t.Fatalf("the right operand is evaluated in its own block:\n%s", rhs)
	}
	short := blockAt(ir, "lgc0short")
	if strings.Contains(short, "sdiv i64") || strings.Contains(short, "icmp eq i64") {
		t.Fatalf("the short path must not evaluate the right operand:\n%s", short)
	}
	if evald := ir[strings.Index(ir, "lgc0rhs:"):strings.Index(ir, "lgc0join:")]; !strings.Contains(evald, "sdiv i64") {
		t.Fatalf("the division rides the evaluated path:\n%s", ir)
	}
	order(t, ir, "br i1", "lgc0rhs:", "sdiv i64", "lgc0join:", "phi i64")
}

// TestLogicOrShortsOnTrue: `||` takes its value from the true side.
func TestLogicOrShortsOnTrue(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("d", "0"),
		&ast.Binding{Kw: "let", Name: "r", Init: binOp("||",
			binOp("==", ident("d"), intLit("0")),
			binOp("==", binOp("/", intLit("10"), ident("d")), intLit("1")))},
		okReturn(),
	), "lgc0short")
	short := blockAt(ir, "lgc0short")
	if !strings.Contains(short, "br label %lgc0join") {
		t.Fatalf("the deciding operand short-circuits to the join:\n%s", short)
	}
	order(t, ir, "lgc0join:", "phi i64", "[ 1, %lgc0short ]")
}

// phiPreds returns each phi-bearing block's predecessor labels, checking
// the coupling as it goes: every entry must name a real block, and that
// block must have an edge to the phi's own. A substring assertion cannot
// see this coupling — a label can be present in the phi and still be the
// wrong block — while the real toolchain rejects the module outright
// ("PHI node entries do not match predecessors!"), which is how the
// defect this guards against reached a probe: an operand whose own
// emission opens blocks (a division guard) leaves the join reached from
// a block several labels past the one the operand opened.
func phiPreds(t *testing.T, ir string) map[string][]string {
	t.Helper()
	re := regexp.MustCompile(`\[\s*[^,\[\]]+, (%[\w.]+)\s*\]`)
	byBlock := map[string][]string{}
	block := ""
	for _, ln := range strings.Split(ir, "\n") {
		if strings.HasSuffix(ln, ":") && !strings.HasPrefix(ln, " ") {
			block = strings.TrimSuffix(ln, ":")
			continue
		}
		if !strings.Contains(ln, " = phi ") {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(ln, -1) {
			pred := strings.TrimPrefix(m[1], "%")
			b := blockAt(ir, pred)
			if b == "" {
				t.Fatalf("phi in %s names the unknown predecessor %s:\n%s", block, pred, ir)
			}
			if !strings.Contains(b, "label %"+block) {
				t.Fatalf("phi in %s names %s, which has no edge to it:\n%s", block, pred, ir)
			}
			byBlock[block] = append(byBlock[block], pred)
		}
	}
	return byBlock
}

// TestLogicShortCircuitPhiNamesRealPreds pins the join's entries to the
// blocks that actually reach it. The guarded division leaves its value in
// the guard's continuation, so the entry must name that block rather than
// the label the operand opened — the difference clang rejected.
func TestLogicShortCircuitPhiNamesRealPreds(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("d", "0"),
		&ast.Binding{Kw: "let", Name: "r", Init: binOp("&&",
			binOp("!=", ident("d"), intLit("0")),
			binOp("==", binOp("/", intLit("10"), ident("d")), intLit("1")))},
		okReturn(),
	), "lgc0join")
	ps := phiPreds(t, ir)["lgc0join"]
	if len(ps) != 2 {
		t.Fatalf("the join takes one entry per path, got %v:\n%s", ps, ir)
	}
	computes := false
	for _, p := range ps {
		if strings.Contains(blockAt(ir, p), "sdiv i64") {
			computes = true
		}
	}
	if !computes {
		t.Fatalf("no entry names the block computing the operand (entries %v):\n%s", ps, ir)
	}
}

// TestScopeValueBodyFirst pins the D10-2 repair: the scope's value form
// evaluates the tail after the body, so the value is what the body left.
// Before the fix the tail read the pre-body value — legal IR, wrong value
// (the `Ok(0)` the design names).
func TestScopeValueBodyFirst(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("n", "0"),
		&ast.Binding{Kw: "let", Name: "r", Init: &ast.ScopeExpr{
			Timeout: intLit("10"),
			Body: ast.Block{Items: []ast.Stmt{
				assignTo("n", binOp("+", ident("n"), intLit("1"))),
				&ast.ExprStmt{Expr: ident("n")},
			}},
		}},
		okReturn(),
	), "__we_scope_leave")
	slotReadsBeforeWrite(t, ir, firstSlot(t, ir))
}

// TestBitwiseLogicalOps: `& | ^` are total over the integer domain —
// one instruction each, no trap face — because every bit pattern the
// operands can carry is a representable result.
func TestBitwiseLogicalOps(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("b", "6"),
		&ast.Binding{Kw: "let", Name: "a", Init: binOp("&", intLit("12"), ident("b"))},
		&ast.Binding{Kw: "let", Name: "o", Init: binOp("|", intLit("12"), ident("b"))},
		&ast.Binding{Kw: "let", Name: "x", Init: binOp("^", intLit("12"), ident("b"))},
		okReturn(),
	), "and i64", "or i64", "xor i64")
	if strings.Contains(ir, "__we_task_fail") {
		t.Fatalf("a total operation carries no trap:\n%s", ir)
	}
}

// TestShiftGuardsAmount: LLVM's shift is poison for an amount at or past
// the operand width, and chapter 7's checked arithmetic lets no
// operation reach undefined behaviour, so the emitter guards the range
// itself — an unsigned compare covers the negative amounts too, since a
// signed amount there is a huge unsigned one.
func TestShiftGuardsAmount(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("k", "3"),
		&ast.Binding{Kw: "let", Name: "l", Init: binOp("<<", intLit("1"), ident("k"))},
		&ast.Binding{Kw: "let", Name: "r", Init: binOp(">>", intLit("64"), ident("k"))},
		okReturn(),
	), "shl i64", "ashr i64")
	if !strings.Contains(ir, "icmp ult i64") {
		t.Fatalf("the shift amount is range-checked:\n%s", ir)
	}
	order(t, ir,
		"icmp ult i64",
		"br i1",
		"call void @__we_task_fail",
		"unreachable",
		"shl i64",
	)
}

// TestShiftLeftChecksLostBits: a left shift that drops a bit off the top
// is the silent wrap chapter 7 forbids, and LLVM's shl discards those
// bits without a word. Shifting back recovers the operand exactly when
// nothing was lost, so the round trip is the check; a right shift loses
// only the low bits, which is what it means, and carries no second
// guard.
func TestShiftLeftChecksLostBits(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("k", "3"),
		&ast.Binding{Kw: "let", Name: "l", Init: binOp("<<", intLit("1"), ident("k"))},
		&ast.Binding{Kw: "let", Name: "r", Init: binOp(">>", intLit("64"), ident("k"))},
		okReturn(),
	), "shl i64", "ashr i64")
	if !strings.Contains(ir, `c"Int64 shift overflow\00"`) {
		t.Fatalf("the shift trap names the operation:\n%s", ir)
	}
	// Two guards for `<<` (amount, then the round trip), one for `>>`.
	if n := strings.Count(ir, "icmp eq i64 %"); n != 1 {
		t.Fatalf("the round-trip check is the left shift's alone, got %d equality tests:\n%s", n, ir)
	}
	lsh := blockAt(ir, "shk0")
	if !strings.Contains(lsh, "shl i64") {
		t.Fatalf("shk0 is not the left shift's checked block:\n%s", ir)
	}
}

// TestOverflowTrapNamesOperation pins chapter 14's report form: the trap
// names the operation — `Int64 add overflow` and siblings — so the three
// checked binaries and the negation each carry their own text rather
// than one shared `integer overflow`.
func TestOverflowTrapNamesOperation(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("b", "2"),
		&ast.Binding{Kw: "let", Name: "s", Init: binOp("+", intLit("1"), ident("b"))},
		&ast.Binding{Kw: "let", Name: "d", Init: binOp("-", intLit("1"), ident("b"))},
		&ast.Binding{Kw: "let", Name: "p", Init: binOp("*", intLit("3"), ident("b"))},
		&ast.Binding{Kw: "let", Name: "n", Init: unary("-", ident("b"))},
		okReturn(),
	), "llvm.sadd.with.overflow.i64", "llvm.ssub.with.overflow.i64", "llvm.smul.with.overflow.i64")
	for _, msg := range []string{"Int64 add overflow", "Int64 sub overflow", "Int64 mul overflow", "Int64 neg overflow"} {
		if !strings.Contains(ir, `c"`+msg+`\00"`) {
			t.Fatalf("no trap reports %q:\n%s", msg, ir)
		}
	}
	if strings.Contains(ir, `c"integer overflow\00"`) {
		t.Fatalf("the shared text is gone:\n%s", ir)
	}
}
