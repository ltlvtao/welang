package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T9-2 (design D8): sum construction and the return face that
// materializes it. The three-word slot T9-1 widened is the layout; this
// is the first task that puts a value in the second payload word, so
// these tests are the semantic pin T9-1 could not have.

// annBind is a binding whose annotation names the value's type — the
// agreement position that lets a prelude constructor resolve at all.
func annBind(name, typ string, x ast.Expr, args ...ast.TypeRef) *ast.Binding {
	return &ast.Binding{Kw: "let", Name: name, Typ: &ast.NamedType{Name: typ, Args: args}, Init: x}
}

func optionOf(t ast.TypeRef) *ast.NamedType {
	return &ast.NamedType{Name: "Option", Args: []ast.TypeRef{t}}
}

func call(fn ast.Expr, args ...ast.Expr) *ast.Call {
	return &ast.Call{Fn: fn, Args: args}
}

// A construction in an annotated binding resolves through the
// annotation: Option<Int64> is the sum, Some is its second variant, and
// the payload lands in the first payload slot with the second one
// zeroed. The whole store sequence is pinned because the ordering is
// what every consumer downstream reads.
func TestSumCtorBindsAPreludePayload(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{progBuild(
		annBind("x", "Option", call(ident("Some"), intLit("1")), named("Int64")),
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// Some is the second variant of the Option table (None 0, Some 1), so
	// the tag is 1 and the payload word is the argument's 1; the second
	// payload word a one-word payload does not reach stays the zero
	// literal, which is what makes the slot's shape variant-independent.
	at := strings.Index(ir, "store i64 1, ptr")
	if at < 0 {
		t.Fatalf("no tag store:\n%s", ir)
	}
	tail := ir[at+len("store i64 1, ptr"):]
	wantIR(t, tail, "store i64 1, ptr", "the payload word")
	wantIR(t, tail[strings.Index(tail, "store i64 1, ptr"):], "store i64 0, ptr", "the zeroed second payload word")
}

// The prelude carries no payload type of its own: `Some(1)` says
// nothing about whether the sum is an Option<Int64> or an Option<UInt8>,
// and the check stage's answer is not recoverable from the argument.
// Without one of chapter 7's agreement positions naming the sum, the
// construction stays at the boundary rather than guessing.
func TestSumCtorWithoutAnExpectationStops(t *testing.T) {
	_, ni := EmitProgram(ModeBuild, []ProgModule{progBuild(
		letBind("x", call(ident("Some"), intLit("1"))),
		okReturn(),
	)})
	if ni == nil {
		t.Fatal("an unannotated prelude construction emitted")
	}
}

// A user sum needs no expectation: chapter 9 puts variant names in the
// module's one name space, so a bare name picks out exactly one
// declaration and both the tag and the payload shape come from it.
//
// The String payload is the reason the second payload word exists at all
// (D8): a (ptr, len) pair is the one payload that fills both, and this is
// the first construction in the corpus to put a non-zero value in pay1.
// The store order — tag, then the words in declaration order — is what
// every consumer downstream reads, so all three are pinned.
func TestSumCtorUserSumNeedsNoExpectation(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		&ast.SumDecl{Name: "Two", Variants: []ast.Variant{
			{Name: "P", Payload: []ast.TypeRef{named("String")}},
			{Name: "Q"},
		}},
		mainDecl(
			letBind("v", call(ident("P"), strLit(`"hi"`))),
			okReturn(),
		),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// P is the module's first variant, so its tag is the zero index. The
	// payload is one String: the interned data pointer in pay0 and the
	// length in pay1.
	wantIR(t, ir, "store i64 0, ptr", "the P tag")
	wantIR(t, ir, "ptrtoint ptr @.s0 to i64", "the String payload's data word")
	at := strings.Index(ir, "ptrtoint ptr @.s0 to i64")
	tail := ir[at:]
	wantIR(t, tail, "store i64 2, ptr", "the String payload's length word in pay1")
	if len(tail) == len(ir) {
		t.Fatal("the data word is not in the body")
	}
}

// A variant payload that needs a third word has no three-word encoding.
// The rule lives at the classification point and nowhere else, so the
// program stops at the boundary instead of emitting a truncated store.
func TestSumOverBudgetStops(t *testing.T) {
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		&ast.SumDecl{Name: "Big", Variants: []ast.Variant{
			{Name: "B", Payload: []ast.TypeRef{named("Int64"), named("Int64"), named("Int64")}},
		}},
		mainDecl(
			letBind("v", call(ident("B"), intLit("1"), intLit("2"), intLit("3"))),
			okReturn(),
		),
	}}}})
	if ni == nil {
		t.Fatal("a three-word payload classified into the ABI")
	}
}

// A sum payload would need tag bits the layout has not got: the fused
// space is spent on the outer discriminant. Refused by inspection.
func TestSumPayloadOfSumStops(t *testing.T) {
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		&ast.SumDecl{Name: "Inner", Variants: []ast.Variant{{Name: "I", Payload: []ast.TypeRef{named("Int64")}}}},
		&ast.SumDecl{Name: "Outer", Variants: []ast.Variant{{Name: "O", Payload: []ast.TypeRef{named("Inner")}}}},
		mainDecl(
			letBind("v", call(ident("O"), call(ident("I"), intLit("1")))),
			okReturn(),
		),
	}}}})
	if ni == nil {
		t.Fatal("a sum payload classified into the ABI")
	}
}

// --- T9-2: the return face and the fused error slot -----------------------

// A sum-typed fn return is the other half of the construction face: the
// value does not reach an alloca at all but has to leave through the
// three-word aggregate the ABI widened for it (T9-1). Both payload words
// are in the constant — the zero in pay1 is what a one-word variant
// declares, not an omission — so the whole define is pinned.
func TestSumCtorReturnsThroughTheFnFace(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		pubFn("f", nil, optionOf(named("Int64")), retValue(call(ident("Some"), intLit("3")))),
		mainDecl(letBind("x", call(ident("f"))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define { i64, i64, i64 } @main.f() {\nentry:\n  ret { i64, i64, i64 } { i64 1, i64 3, i64 0 }\n}",
		"the Some(3) return")
}

// A payload that is not a constant cannot ride the aggregate literal: an
// aggregate constant holds constants only, so the words are inserted one
// at a time into an undef (the same shape fnRetOperand's strPair uses).
// The second insert lands the String's length in pay1 — the word a
// two-word payload is the reason for.
func TestSumCtorFnPayloadFillsBothWords(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		&ast.SumDecl{Name: "Two", Variants: []ast.Variant{
			{Name: "P", Payload: []ast.TypeRef{named("String")}},
			{Name: "Q"},
		}},
		pubFn("g", nil, named("Two"), retValue(call(ident("P"), strLit(`"hi"`)))),
		mainDecl(letBind("y", call(ident("g"))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define { i64, i64, i64 } @main.g() {\nentry:\n"+
		"  %v8 = ptrtoint ptr @.s0 to i64\n"+
		"  %v9 = insertvalue { i64, i64, i64 } undef, i64 0, 0\n"+
		"  %v10 = insertvalue { i64, i64, i64 } %v9, i64 %v8, 1\n"+
		"  %v11 = insertvalue { i64, i64, i64 } %v10, i64 2, 2\n"+
		"  ret { i64, i64, i64 } %v11\n}", "the P(\"hi\") return")
}

// An arm binds a payload position by what the variant DECLARED there, not
// by the one word the M9b slots carried: a String position is the
// (ptr, len) pair, so the binding reads both words. Both loads are pinned
// by their slot — a binder that read pay0 twice would hand the callee the
// data pointer's bits as a length, which no output would show.
func TestSumPayloadBindsByDeclaredKind(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{progBuild(
		annBind("x", "Option", call(ident("Some"), strLit(`"hi"`)), named("String")),
		&ast.ExprStmt{Expr: &ast.Match{
			Scrutinee: ident("x"),
			Arms: []ast.MatchArm{
				{Pat: &ast.PatVariant{Name: "Some", Args: []ast.Pattern{&ast.PatBinding{Name: "s"}}},
					Body: blockOf(ioCall("io", "println", ident("s")))},
				{Pat: &ast.PatVariant{Name: "None"},
					Body: blockOf(ioCall("io", "println", strLit(`"none"`)))},
			},
		}},
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "  %v6 = load i64, ptr %v2\n  %v7 = inttoptr i64 %v6 to ptr\n  %v8 = load i64, ptr %v3",
		"the String payload's two words")
	wantIR(t, ir, "call void %v9(ptr %v7, i64 %v8)", "the pair reaching the callee")
}

// T9-2's fusion ruling: `Result<T, E>` spends one tag space on both
// halves, Ok at zero and E's variants above it, so `Err(p)` is not a
// variant the table names. The two spellings the checker accepts are the
// nested pattern, which indexes the fused table directly, and `Err(e)`,
// which rebinds the payload as an E-typed sum — its own tag is the fused
// one shifted down past Ok, and its payload words are the outer slot's,
// because the fusion spends no second pair on E.
func TestErrPatternBindsTheFusedErrorSum(t *testing.T) {
	cases := []struct {
		name    string
		arms    []ast.MatchArm
		shape   string
		bodyUse []string
	}{{
		name: "nested pattern indexes the fused table",
		arms: []ast.MatchArm{
			{Pat: okPat(), Body: blockOf(ioCall("io", "println", intLit("0")))},
			{Pat: &ast.PatVariant{Name: "Err", Args: []ast.Pattern{
				&ast.PatVariant{Name: "Failed", Args: []ast.Pattern{&ast.PatBinding{Name: "a"}, &ast.PatBinding{Name: "b"}}}}},
				Body: blockOf(ioCall("io", "println", ident("b")))},
			{Pat: &ast.PatVariant{Name: "Err", Args: []ast.Pattern{&ast.PatVariant{Name: "Nope"}}},
				Body: blockOf(ioCall("io", "println", intLit("9")))},
		},
		shape:   "icmp eq i64 %v3, 1",
		bodyUse: []string{"  %v6 = load i64, ptr %v1\n  %v7 = load i64, ptr %v2\n  call void @__we_println_i64(i64 %v7)"},
	}, {
		name: "Err(e) rebinds the shifted error sum",
		arms: []ast.MatchArm{
			{Pat: okPat(), Body: blockOf(ioCall("io", "println", intLit("0")))},
			{Pat: &ast.PatVariant{Name: "Err", Args: []ast.Pattern{&ast.PatBinding{Name: "e"}}},
				Body: blockOf(&ast.ExprStmt{Expr: &ast.Match{
					Scrutinee: ident("e"),
					Arms: []ast.MatchArm{
						{Pat: &ast.PatVariant{Name: "Failed", Args: []ast.Pattern{&ast.PatBinding{Name: "a"}, &ast.PatBinding{Name: "b"}}},
							Body: blockOf(ioCall("io", "println", ident("b")))},
						{Pat: &ast.PatVariant{Name: "Nope"}, Body: blockOf(ioCall("io", "println", intLit("9")))},
					},
				}})},
		},
		shape: "  %v5 = icmp sge i64 %v3, 1\n  br i1 %v5",
		bodyUse: []string{
			// The inner tag is the fused one minus Ok ...
			"  %v7 = sub i64 %v3, 1\n  store i64 %v7, ptr %v6",
			// ... so the inner table's first variant is the zero index.
			"  %v8 = load i64, ptr %v6\n  %v9 = icmp eq i64 %v8, 0",
			// ... and the payload words are still the outer slot's.
			"  %v10 = load i64, ptr %v1\n  %v11 = load i64, ptr %v2\n  call void @__we_println_i64(i64 %v11)",
		},
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
				bigErr(),
				mainDecl(errResult(), &ast.ExprStmt{Expr: &ast.Match{Scrutinee: ident("r"), Arms: c.arms}}, okReturn()),
			}}}})
			if ni != nil {
				t.Fatalf("boundary: %s", ni.What)
			}
			wantIR(t, ir, c.shape, "the error arm's tag test")
			for _, f := range c.bodyUse {
				wantIR(t, ir, f, "the bound payload")
			}
		})
	}
}

// `Err(_)` discards the whole error sum: no name to bind, and neither the
// shifted tag nor the payload words are read. The checker accepts it
// (verified against the real front end), so it must not be a boundary.
func TestErrWildcardConsumesNothing(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		bigErr(),
		mainDecl(errResult(), &ast.ExprStmt{Expr: &ast.Match{
			Scrutinee: ident("r"),
			Arms: []ast.MatchArm{
				{Pat: okPat(), Body: blockOf(ioCall("io", "println", intLit("0")))},
				{Pat: &ast.PatVariant{Name: "Err", Args: []ast.Pattern{&ast.PatWildcard{}}},
					Body: blockOf(ioCall("io", "println", intLit("9")))},
			},
		}}, okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantNoIR(t, ir, " = sub i64 ", "a tag shift for a discarded error sum")
}

// bigErr is a two-word error sum with a second variant: the shape the
// fused table needs to be more than a renaming of Ok.
func bigErr() *ast.SumDecl {
	return &ast.SumDecl{Pub: true, Name: "AppError", Variants: []ast.Variant{
		{Name: "Failed", Payload: []ast.TypeRef{named("Int64"), named("Int64")}},
		{Name: "Nope"},
	}}
}

// errResult is `let r: Result<(), AppError> = Err(Failed(1, 2))` — the
// fused value the two Err pattern spellings both consume.
func errResult() *ast.Binding {
	return annBind("r", "Result",
		call(ident("Err"), call(ident("Failed"), intLit("1i64"), intLit("2i64"))),
		&ast.UnitType{}, named("AppError"))
}

func okPat() *ast.PatVariant {
	return &ast.PatVariant{Name: "Ok", Args: []ast.Pattern{&ast.PatWildcard{}}}
}

// The fused error return is reachable from a declared fn too, not only
// from main's tail: `return Err(Failed(...))` under a declared
// `Result<T, AppError>` resolves through the same recursion, so the tag
// that leaves the body is the error variant's index in the fused table.
// The declared return is the whole expectation — no binding annotation is
// in sight.
func TestSumCtorFnReturnsFusedError(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		bigErr(),
		pubFn("mk", []ast.Param{i64Param("n")},
			&ast.NamedType{Name: "Result", Args: []ast.TypeRef{named("Int64"), named("AppError")}},
			&ast.ExprStmt{Expr: &ast.If{Cond: binOp(">", ident("n"), intLit("0")), Then: ast.Block{Items: []ast.Stmt{
				retValue(call(ident("Ok"), ident("n"))),
			}}}},
			retValue(call(ident("Err"), call(ident("Failed"), intLit("1i64"), intLit("2i64")))),
		),
		mainDecl(letBind("_", call(ident("mk"), intLit("1i64"))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// Ok carries the argument's own word; Failed is index 1 of the fused
	// table, so its tag leaves as the constant 1 with both payload words.
	wantIR(t, ir, "ret { i64, i64, i64 } { i64 1, i64 1, i64 2 }", "the Err(Failed(1, 2)) return")
}

// T9-3 (design D8): the parameter position. A sum crosses the call as its
// three words and the callee's arms resolve their table out of the
// DECLARED type — the same authority a binding's annotation carries, and
// the only one the callee has, since the argument it was handed says
// nothing about which variants exist.

// errParam is `Result<(), AppError>` as a declared parameter type.
func errParam(name string) ast.Param {
	return ast.Param{Name: name, Type: &ast.NamedType{Name: "Result",
		Args: []ast.TypeRef{&ast.UnitType{}, named("AppError")}}}
}

// The three arms read the fused table an annotation-position match reads:
// Ok is tag 0, and the error variants sit above it in declaration order,
// so Failed tests 1 and Nope tests 2. The Failed payload binds both words
// by what the variant declared — the second word is the one the caller's
// construction put there, which an arm that read the first word twice
// could not reach.
func TestSumParameterCarriesItsVariantTable(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		bigErr(),
		pubFn("name", []ast.Param{errParam("r")}, named("Int64"),
			&ast.ExprStmt{Expr: &ast.Match{
				Scrutinee: ident("r"),
				Arms: []ast.MatchArm{
					{Pat: okPat(), Body: blockOf(ioCall("io", "println", intLit("0")))},
					{Pat: &ast.PatVariant{Name: "Err", Args: []ast.Pattern{
						&ast.PatVariant{Name: "Failed", Args: []ast.Pattern{&ast.PatBinding{Name: "a"}, &ast.PatBinding{Name: "b"}}}}},
						Body: blockOf(ioCall("io", "println", ident("b")))},
					{Pat: &ast.PatVariant{Name: "Err", Args: []ast.Pattern{&ast.PatVariant{Name: "Nope"}}},
						Body: blockOf(ioCall("io", "println", intLit("9")))},
				},
			}},
			retValue(intLit("0")),
		),
		mainDecl(letDiscard(call(ident("name"), call(ident("Ok"), &ast.Unit{}))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// The parameter face is the three-word slot spelled as three scalars,
	// in the order every construction writes them.
	wantIR(t, ir, "define i64 @main.name(i64 %r0, i64 %r1, i64 %r2) {", "the sum parameter's face")
	// The arms' tag tests are the declaration's indices in the fused
	// table, read off the tag word the declaration named.
	wantIR(t, ir, "  %v12 = icmp eq i64 %v11, 0", "the Ok arm")
	wantIR(t, ir, "  %v13 = icmp eq i64 %v11, 1", "the Failed arm")
	wantIR(t, ir, "  %v16 = icmp eq i64 %v11, 2", "the Nope arm")
	// Both payload words, each from its own slot, by declaration.
	wantIR(t, ir, "  %v14 = load i64, ptr %v9\n  %v15 = load i64, ptr %v10\n  call void @__we_println_i64(i64 %v15)",
		"the two-word payload binding")
}

// A construction written at an argument position is a call, so it arrives
// through the call arm rather than the binding-name arm — and there it is
// the PARAMETER's declared type that names the sum, because the argument
// itself says nothing. Both spellings ride that one expectation: `Some(1)`
// with its parentheses, and the bare `None`, which had no parentheses to
// arrive through and so reached the arm as a plain name.
func TestSumConstructionAtAnArgumentPosition(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		pubFn("show", []ast.Param{{Name: "o", Type: optionOf(named("Int64"))}}, named("Int64"),
			retValue(intLit("0"))),
		mainDecl(
			letDiscard(call(ident("show"), call(ident("Some"), intLit("1i64")))),
			letDiscard(call(ident("show"), ident("None"))),
			okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// Some is index 1 with the argument in the first payload word and the
	// second one zeroed — the construction happens at the call site and
	// the three words cross as the callee's parameters.
	wantIR(t, ir, "  store i64 1, ptr %v1\n  store i64 1, ptr %v2\n  store i64 0, ptr %v3\n"+
		"  %v4 = load i64, ptr %v1\n  %v5 = load i64, ptr %v2\n  %v6 = load i64, ptr %v3\n"+
		"  %v7 = call i64 %v0(i64 %v4, i64 %v5, i64 %v6)", "the Some(1) argument")
	// None is index 0 and carries nothing, so all three words are the zero
	// literal — a nullary variant is a tag and no payload.
	wantIR(t, ir, "  store i64 0, ptr %v9\n  store i64 0, ptr %v10\n  store i64 0, ptr %v11\n"+
		"  %v12 = load i64, ptr %v9\n  %v13 = load i64, ptr %v10\n  %v14 = load i64, ptr %v11\n"+
		"  %v15 = call i64 %v8(i64 %v12, i64 %v13, i64 %v14)", "the bare None argument")
}
