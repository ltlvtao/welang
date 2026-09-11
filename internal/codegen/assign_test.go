package codegen

import (
	"regexp"
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

// strVar builds `var name: String = init` (the T9-4 storage annotation: a
// var String names its type, exactly as a var scalar does — an unannotated
// var is the older boundary for both families).
func strVar(name string, init ast.Expr) *ast.Binding {
	return &ast.Binding{Kw: "var", Name: name, Typ: named("String"), Init: init}
}

// strParam builds one `s: String` parameter.
func strParam(name string) ast.Param {
	return ast.Param{Name: name, Type: named("String")}
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

// --- T9-4: the String two-word storage face -------------------------------
//
// A String is a (ptr, len) pair, so the addressable face a scalar takes one
// slot for takes two. The split above carries over unchanged: a name the
// body writes owns its words from the binding on, a name it only reads
// keeps whatever face its initializer produced. What the extra words buy is
// the one thing a pair of SSA operands cannot do — a String binding that is
// reassigned (the face the M15 control-03 trap was calibrated against).

// firstAlloca returns the register one alloca of typ takes ("%v0" for
// "  %v0 = alloca ptr") — the storage the assertions below then follow
// through its stores and loads without pinning the counter's numbering.
func firstAlloca(t *testing.T, ir, typ string) string {
	t.Helper()
	m := regexp.MustCompile(`(%\w+) = alloca ` + regexp.QuoteMeta(typ) + "\n").FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("no alloca %s in:\n%s", typ, ir)
	}
	return m[1]
}

// TestVarStringOwnsItsTwoWords: `var s: String = "a"` reserves both words at
// the entry, stores the literal's pair into them, and reads them back at the
// println. Two words, not one: the data word and the length word are two
// allocas, and nothing else in the body takes a third.
func TestVarStringOwnsItsTwoWords(t *testing.T) {
	ir := assertClean(t, m9bModule(
		strVar("s", strLit(`"a"`)),
		ioCall("io", "println", ident("s")),
		okReturn(),
	))
	if got := strings.Count(ir, "alloca ptr"); got != 1 {
		t.Fatalf("want one data word, got %d:\n%s", got, ir)
	}
	if got := strings.Count(ir, "alloca i64"); got != 1 {
		t.Fatalf("want one length word, got %d:\n%s", got, ir)
	}
	order(t, ir, "entry:", "alloca ptr", "alloca i64",
		"store ptr @.s0, ptr", "store i64 1, ptr",
		"load ptr, ptr", "load i64, ptr")
}

// TestStringReassignStoresThroughTheSameWords: the second literal lands in
// the words the first one opened, and the read after it is what makes the
// write observable — a name that kept reading its binding-time face would
// print "a" here.
func TestStringReassignStoresThroughTheSameWords(t *testing.T) {
	ir := assertClean(t, m9bModule(
		letBind("s", strLit(`"a"`)),
		assignTo("s", strLit(`"b"`)),
		ioCall("io", "println", ident("s")),
		okReturn(),
	))
	data := firstAlloca(t, ir, "ptr")
	lenw := firstAlloca(t, ir, "i64")
	order(t, ir,
		"store ptr @.s0, ptr "+data, "store i64 1, ptr "+lenw,
		"store ptr @.s1, ptr "+data, "store i64 1, ptr "+lenw,
		"= load ptr, ptr "+data, "= load i64, ptr "+lenw)
}

// TestStringSelfAssignReadsBeforeItWrites: `s = s + "b"` hands the join the
// pair the name held — both words load ahead of both stores. The reverse
// order would feed the concatenation the value being written, so the
// assertion is on which registers the call actually consumes.
func TestStringSelfAssignReadsBeforeItWrites(t *testing.T) {
	ir := assertClean(t, m9bModule(
		letBind("s", strLit(`"a"`)),
		assignTo("s", binOp("+", ident("s"), strLit(`"b"`))),
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "declare %struct.we_str @__we_str_concat")
	data := firstAlloca(t, ir, "ptr")
	lenw := firstAlloca(t, ir, "i64")
	m := regexp.MustCompile(`(%\w+) = load ptr, ptr ` + regexp.QuoteMeta(data) +
		`\n\s+(%\w+) = load i64, ptr ` + regexp.QuoteMeta(lenw) +
		`\n\s+%\w+ = call %struct\.we_str @__we_str_concat\(ptr (%\w+), i64 (%\w+),`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the self-assignment must load both words before joining them:\n%s", ir)
	}
	if m[1] != m[3] || m[2] != m[4] {
		t.Fatalf("the join consumes %s/%s, not the name's own %s/%s:\n%s", m[3], m[4], m[1], m[2], ir)
	}
	order(t, ir, "call %struct.we_str @__we_str_concat", "store ptr", "store i64")
}

// TestStringAliasOwnWords: a read-only alias shares the source's face and
// reserves nothing; the moment the alias is written it takes its own words,
// because a store through it must not reach back into the source.
func TestStringAliasOwnWords(t *testing.T) {
	ir := assertClean(t, m9bModule(
		letBind("a", strLit(`"a"`)),
		&ast.Binding{Kw: "let", Name: "b", Init: ident("a")},
		assignTo("b", strLit(`"b"`)),
		ioCall("io", "println", ident("a")),
		ioCall("io", "println", ident("b")),
		okReturn(),
	))
	if got := strings.Count(ir, "alloca ptr"); got != 1 {
		t.Fatalf("only the written alias needs words, got %d data word(s):\n%s", got, ir)
	}
	if !regexp.MustCompile(`call void %\w+\(ptr @\.s0, i64 1\)`).MatchString(ir) {
		t.Fatalf("the source must still read its own literal:\n%s", ir)
	}
	data := firstAlloca(t, ir, "ptr")
	order(t, ir, "store ptr @.s1, ptr "+data, "load ptr, ptr "+data)
}

// TestStringParamTakesSlotWhenAssigned: a String parameter arrives as two IR
// words and those words are read-only, so a body that writes the name copies
// them into a pair of words of its own before anything lands in them.
func TestStringParamTakesSlotWhenAssigned(t *testing.T) {
	ir := assertClean(t, drModule(pubFn("f", []ast.Param{strParam("s")}, named("String"),
		assignTo("s", strLit(`"x"`)),
		retValue(ident("s")),
	)), "define { ptr, i64 } @main.f(ptr %s0, i64 %s1)")
	data := firstAlloca(t, ir, "ptr")
	lenw := firstAlloca(t, ir, "i64")
	order(t, ir, "entry:",
		"store ptr %s0, ptr "+data, "store i64 %s1, ptr "+lenw,
		"store ptr @.s0, ptr "+data,
		"= load ptr, ptr "+data, "= load i64, ptr "+lenw)
}

// TestStringParamUnwrittenKeepsItsWords: the copy is what a write costs, and
// a body that only reads the parameter pays nothing — the name keeps the
// incoming pair and no word is reserved for it.
func TestStringParamUnwrittenKeepsItsWords(t *testing.T) {
	ir := assertClean(t, drModule(pubFn("f", []ast.Param{strParam("s")}, named("String"),
		retValue(ident("s")),
	)), "define { ptr, i64 } @main.f(ptr %s0, i64 %s1)")
	if strings.Contains(ir, "alloca") {
		t.Fatalf("an unwritten parameter reserves nothing:\n%s", ir)
	}
	if !strings.Contains(ir, "insertvalue { ptr, i64 } undef, ptr %s0, 0") {
		t.Fatalf("the tail must return the incoming pair:\n%s", ir)
	}
}

// TestStringFnReturnReadsTheWritableName: the return face reads a written
// name's words and rebuilds the aggregate through insertvalue — a ret takes
// a constant, and an SSA pair loaded at the tail is not one.
func TestStringFnReturnReadsTheWritableName(t *testing.T) {
	ir := assertClean(t, drModule(pubFn("f", nil, named("String"),
		strVar("r", strLit(`"a"`)),
		retValue(ident("r")),
	)), "define { ptr, i64 } @main.f()")
	data := firstAlloca(t, ir, "ptr")
	lenw := firstAlloca(t, ir, "i64")
	order(t, ir,
		"load ptr, ptr "+data, "load i64, ptr "+lenw,
		"insertvalue { ptr, i64 } undef, ptr",
		"insertvalue { ptr, i64 } %",
		"ret { ptr, i64 } %")
}

// The four pins below are the storage face's other entrances — the ones the
// T9-4 mutation battery found unwatched (a written String bound from a value
// expression or from a call, and read back through a newtype's `.value`).
// Each writes the name, so each binding site owes the two words; the
// mutations that dropped the words left the later store with no address at
// all.

// TestStringConcatBindingOwnsItsWords: `let s = a + b` computes the join at
// the binding, and the pair it leaves is an SSA pair of extractvalues — no
// address for a later store. A body that writes `s` therefore lands the
// join's two words in words of the name's own at the binding.
func TestStringConcatBindingOwnsItsWords(t *testing.T) {
	ir := assertClean(t, m9bModule(
		letBind("a", strLit(`"ab"`)),
		letBind("b", strLit(`"cd"`)),
		letBind("s", binOp("+", ident("a"), ident("b"))),
		assignTo("s", strLit(`"x"`)),
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "declare %struct.we_str @__we_str_concat(ptr, i64, ptr, i64)")
	data := firstAlloca(t, ir, "ptr")
	lenw := firstAlloca(t, ir, "i64")
	m := regexp.MustCompile(`(%\w+) = extractvalue %struct\.we_str %\w+, 0\n\s+(%\w+) = extractvalue %struct\.we_str %\w+, 1\n\s+store ptr (%\w+), ptr (%\w+)\n\s+store i64 (%\w+), ptr (%\w+)`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the binding must land the join's pair in the name's words:\n%s", ir)
	}
	if m[1] != m[3] || m[2] != m[5] {
		t.Fatalf("the words stored are not the join's extractvalues:\n%s", ir)
	}
	if m[4] != data || m[6] != lenw {
		t.Fatalf("the join lands in %s/%s, not the name's %s/%s:\n%s", m[4], m[6], data, lenw, ir)
	}
	order(t, ir, "store ptr @", "ptr "+data, "= load ptr, ptr "+data, "= load i64, ptr "+lenw)
}

// TestStringInterpolatedBindingOwnsItsWords: an interpolated literal is the
// same shape one join deeper, and the promise is the same one — the last
// join of the binding's chain is what lands in the name's words.
func TestStringInterpolatedBindingOwnsItsWords(t *testing.T) {
	ir := assertClean(t, m9bModule(
		letBind("s", interpLit([]string{"a", "b"}, intLit("1"))),
		assignTo("s", strLit(`"x"`)),
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "declare %struct.we_str @__we_str_concat(ptr, i64, ptr, i64)")
	data := firstAlloca(t, ir, "ptr")
	lenw := firstAlloca(t, ir, "i64")
	if !regexp.MustCompile(`= call %struct\.we_str @__we_str_concat\([^\n]*\)\n\s+%\w+ = extractvalue[^\n]*\n\s+%\w+ = extractvalue[^\n]*\n\s+store ptr %\w+, ptr ` + regexp.QuoteMeta(data) + `\n\s+store i64 %\w+, ptr ` + regexp.QuoteMeta(lenw)).MatchString(ir) {
		t.Fatalf("the interpolation's pair must land in the name's words:\n%s", ir)
	}
	order(t, ir, "store ptr @.s2, ptr "+data, "= load ptr, ptr "+data)
}

// TestStringCallResultOwnsItsWords: a call's String result arrives as a
// two-word aggregate, so the binding extracts it — and a written name's
// words are what the extraction lands in.
func TestStringCallResultOwnsItsWords(t *testing.T) {
	f := pubFn("f", nil, named("String"), retValue(strLit(`"hi"`)))
	ir := assertClean(t, recModule([]ast.Item{f},
		letBind("s", call(ident("f"))),
		assignTo("s", strLit(`"x"`)),
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "= call { ptr, i64 } %")
	data := firstAlloca(t, ir, "ptr")
	lenw := firstAlloca(t, ir, "i64")
	m := regexp.MustCompile(`(%\w+) = extractvalue \{ ptr, i64 \} %\w+, 0\n\s+(%\w+) = extractvalue \{ ptr, i64 \} %\w+, 1\n\s+store ptr (%\w+), ptr (%\w+)\n\s+store i64 (%\w+), ptr (%\w+)`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the call's pair must land in the name's words:\n%s", ir)
	}
	if m[1] != m[3] || m[2] != m[5] {
		t.Fatalf("the words stored are not the call's extractvalues:\n%s", ir)
	}
	if m[4] != data || m[6] != lenw {
		t.Fatalf("the call result lands in %s/%s, not the name's %s/%s:\n%s", m[4], m[6], data, lenw, ir)
	}
	order(t, ir, "store ptr @", "ptr "+data, "= load ptr, ptr "+data)
}

// TestNewtypeValueReadsTheWritableName: a newtype over String is erased, so
// a written `n: Name` holds its words exactly as a written `s: String`
// does — and its one member read is the binding's own face, which for a
// written name is the loaded pair. The load is the whole assertion: a face
// that handed back the words themselves would give the caller a stack
// address where the aggregate was declared.
func TestNewtypeValueReadsTheWritableName(t *testing.T) {
	label := pubFn("label", []ast.Param{{Name: "n", Type: named("Name")}}, named("String"),
		assignTo("n", ident("n")),
		retValue(memberOf(ident("n"), "value")),
	)
	ir := assertClean(t, recModule([]ast.Item{newtypeDecl("Name", "String"), label},
		okReturn(),
	), "define { ptr, i64 } @main.label(ptr %n0, i64 %n1)")
	data := firstAlloca(t, ir, "ptr")
	lenw := firstAlloca(t, ir, "i64")
	m := regexp.MustCompile(`(%\w+) = load ptr, ptr ` + regexp.QuoteMeta(data) +
		`\n\s+(%\w+) = load i64, ptr ` + regexp.QuoteMeta(lenw) +
		`\n\s+%\w+ = insertvalue \{ ptr, i64 \} undef, ptr (%\w+), 0\n\s+%\w+ = insertvalue \{ ptr, i64 \} %\w+, i64 (%\w+), 1`).FindStringSubmatch(ir)
	if m == nil {
		t.Fatalf("the `.value` read must rebuild the aggregate from the loaded words:\n%s", ir)
	}
	if m[1] != m[3] || m[2] != m[4] {
		t.Fatalf("the returned aggregate carries %s/%s, not the name's loaded %s/%s:\n%s", m[3], m[4], m[1], m[2], ir)
	}
}

// TestStringWordsHoistedToEntry: the D13 discipline is about words, not
// domains — a two-word String face reserved inside a loop body would take
// four stack slots per iteration exactly as a scalar's single one did.
func TestStringWordsHoistedToEntry(t *testing.T) {
	ir := assertClean(t, m9bModule(
		varLit("i", "0"),
		strVar("s", strLit(`"a"`)),
		whileShape(binOp("<", ident("i"), intLit("3")),
			assignTo("i", binOp("+", ident("i"), intLit("1"))),
			strVar("r", strLit(`"b"`)),
			assignTo("r", strLit(`"c"`)),
			ioCall("io", "println", ident("r")),
		),
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "whbody0")
	if got, want := strings.Count(ir, "alloca ptr"), 2; got != want {
		t.Fatalf("want %d data words, got %d:\n%s", want, got, ir)
	}
	if got, want := strings.Count(ir, "alloca i64"), 3; got != want {
		t.Fatalf("want %d length words (i, s, r), got %d:\n%s", want, got, ir)
	}
	order(t, ir, "entry:", "alloca i64", "alloca ptr", "alloca i64",
		"alloca ptr", "alloca i64", "br label %whhead0")
	body := afterLabel(ir, "whbody0")
	if body == "" {
		t.Fatalf("missing the loop body label:\n%s", ir)
	}
	if strings.Contains(body, "alloca") {
		t.Fatalf("a word reserved inside the loop body is taken once per iteration:\n%s", ir)
	}
}
