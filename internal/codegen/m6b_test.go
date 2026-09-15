package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T12 (design D9 clusters 6 and 9): the `?` over a user Result, and the
// synchronous type's return position. The golden run faces observe these
// from the outside; the pins here hold the arms from the inside — the
// fn-side propagate, the main-side report (the runtime String payload and
// the bare-variant constant), the Ok binding's face — plus the two gates
// that decide which arm a table reaches, and the caller-side half of the
// prim return: the re-root that follows the call.

// resultOf is `Result<T, E>` as a declared return.
func resultOf(t, e ast.TypeRef) *ast.NamedType {
	return &ast.NamedType{Name: "Result", Args: []ast.TypeRef{t, e}}
}

// q is `expr?` — the Prop with no selector is the question form.
func q(x ast.Expr) *ast.Prop { return &ast.Prop{X: x} }

// The two gates are pure reads of the table: the Ok face must bind in
// one word (or none), and the report line must name exactly one error
// variant whose one payload is a String — or none at all. Everything
// else the gates refuse, so the `?` stops honestly rather than handing
// the caller a raw word read as a number, or printing a line the table
// cannot name.
func TestQuestionGatesAnswerByFace(t *testing.T) {
	i64 := fnParamAbi{kind: abiI64, typ: "Int64"}
	str := fnParamAbi{kind: abiStr}
	gc := fnParamAbi{kind: abiGc}
	f64 := fnParamAbi{kind: abiDouble}
	okFace := func(pay ...fnParamAbi) []sumVariantShape {
		return []sumVariantShape{{name: "Ok", pay: pay}}
	}
	okCases := []struct {
		name   string
		shapes []sumVariantShape
		want   bool
	}{
		{"unit Ok", okFace(), true},
		{"one i64 word", okFace(i64), true},
		{"two i64 words", okFace(i64, i64), false},
		{"a String pair", okFace(str), false},
		{"a gc handle", okFace(gc), false},
		{"a Float64's bits", okFace(f64), false},
		{"not a Result head", []sumVariantShape{{name: "Some", pay: []fnParamAbi{i64}}}, false},
	}
	for _, c := range okCases {
		t.Run(c.name, func(t *testing.T) {
			if got := questionOkFace(c.shapes); got != c.want {
				t.Fatalf("questionOkFace = %v, want %v", got, c.want)
			}
		})
	}
	table := func(err sumVariantShape) []sumVariantShape {
		return []sumVariantShape{{name: "Ok", pay: []fnParamAbi{i64}}, err}
	}
	lines := []struct {
		name       string
		shapes     []sumVariantShape
		wantName   string
		wantStrPay bool
		wantOK     bool
	}{
		{"a runtime String payload", table(sumVariantShape{name: "Failed", pay: []fnParamAbi{str}}), "Failed", true, true},
		{"a bare variant's constant line", table(sumVariantShape{name: "Boom"}), "Boom", false, true},
		{"a numeric payload", table(sumVariantShape{name: "Bad", pay: []fnParamAbi{i64}}), "", false, false},
		{"a two-word payload", table(sumVariantShape{name: "Wide", pay: []fnParamAbi{str, str}}), "", false, false},
		{"a multi-variant error face", []sumVariantShape{{name: "Ok", pay: []fnParamAbi{i64}}, {name: "A"}, {name: "B"}}, "", false, false},
	}
	for _, c := range lines {
		t.Run(c.name, func(t *testing.T) {
			name, strPay, ok := questionErrLine(c.shapes)
			if name != c.wantName || strPay != c.wantStrPay || ok != c.wantOK {
				t.Fatalf("questionErrLine = (%q, %v, %v), want (%q, %v, %v)",
					name, strPay, ok, c.wantName, c.wantStrPay, c.wantOK)
			}
		})
	}
}

// moduleDefineOf slices one fn's define text out of the module. A module
// fn lives at `@main.<name>`; the synthesized entry `__we_main` carries no
// module prefix.
func moduleDefineOf(ir, sym string) string {
	i := strings.Index(ir, "@main."+sym+"(")
	if i < 0 {
		i = strings.Index(ir, "@"+sym+"(")
	}
	if i < 0 {
		return ""
	}
	j := strings.Index(ir[i+1:], "\ndefine ")
	k := strings.Index(ir[i+1:], "\n@slot.")
	end := len(ir[i:])
	if j < 0 && k >= 0 {
		end = k + 1
	} else if j >= 0 && (k < 0 || j < k) {
		end = j + 1
	}
	return ir[i : i+end]
}

// A fn propagates its own Result whole: the Err arm rebuilds the three
// words as the return's aggregate and takes the same exit protocol a
// return takes — the deferred drain and the root pops — without ever
// naming a report line. That is the fn/main asymmetry: the fn side never
// asks questionErrLine, so a multi-variant or non-String error face
// propagates as freely as a bare one.
func TestQuestionPropagatesWholeFromAFnBody(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		pubFn("inner", nil, resultOf(named("Int64"), named("AppError")),
			retValue(call(ident("Err"), call(ident("Failed"), strLit(`"boom"`))))),
		pubFn("outer", nil, resultOf(named("Int64"), named("AppError")),
			&ast.Binding{Kw: "let", Name: "v", Init: q(call(ident("inner")))},
			retValue(call(ident("Ok"), binOp("+", ident("v"), intLit("1i64")))),
		),
		mainDecl(letBind("w", q(call(ident("outer")))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	def := moduleDefineOf(ir, "outer")
	if def == "" {
		t.Fatalf("no define for main.outer:\n%s", ir)
	}
	// The Err arm is the slice between the question's two labels: it
	// rebuilds exactly the three words and returns them whole. The Ok
	// path builds its own three words later — those are `Ok(v + 1)`'s,
	// not the propagate's, so the count is pinned per arm, not per def.
	e := strings.Index(def, "\nqerr")
	k := strings.Index(def, "\nqok")
	if e < 0 || k < 0 || e > k {
		t.Fatalf("no question join labels:\n%s", def)
	}
	errArm := def[e:k]
	if got := strings.Count(errArm, "= insertvalue { i64, i64, i64 }"); got != 3 {
		t.Fatalf("Err-arm insertvalue count %d, want 3 (the three words rebuilt):\n%s", got, def)
	}
	wantIR(t, errArm, "ret { i64, i64, i64 } %v", "the whole-Result exit")
	if strings.Contains(def, "__we_fail") {
		t.Fatalf("the fn side must not name a report line:\n%s", def)
	}
	// The Ok path keeps emitting into the same body: the checked add sits
	// after the question's join, not inside the Err arm. (An Int64 add
	// emits as the overflow-checked intrinsic, never a bare `add i64`.)
	if strings.Index(def, "@llvm.sadd.with.overflow.i64(") < k {
		t.Fatalf("the Ok path must follow the question:\n%s", def)
	}
}

// The main-side report renders the payload at run time: the head is an
// interned constant, the payload pair rides through the same concat the
// error tail uses, and the newline joins as a constant segment. No
// compile-time constant spells the whole line — the message is the
// program's own data.
func TestQuestionRendersTheErrPayloadAtRunTime(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		appError(),
		pubFn("fetch", nil, resultOf(named("Int64"), named("AppError")),
			retValue(call(ident("Err"), call(ident("Failed"), strLit(`"boom"`))))),
		mainDecl(letBind("v", q(call(ident("fetch")))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	def := moduleDefineOf(ir, "__we_main")
	wantIR(t, ir, `c"error: Failed: "`, "the interned head")
	if got := countCall(def, "%struct.we_str", "__we_str_concat"); got != 2 {
		t.Fatalf("concat count %d, want 2 (head, then newline):\n%s", got, def)
	}
	wantIR(t, def, "call void @__we_fail(ptr %", "the report handed to the runtime")
	if strings.Contains(ir, `c"error: Failed: boom`) {
		t.Fatalf("the whole line must not be a compile-time constant:\n%s", ir)
	}
}

// A bare variant's line needs nothing the program holds: it is the same
// global constant form the static arm writes, one byte longer than the
// text for the newline.
func TestQuestionReportsABareVariantAsAConstant(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		&ast.SumDecl{Pub: true, Name: "Bare", Variants: []ast.Variant{{Name: "Boom"}}},
		pubFn("fail", nil, resultOf(&ast.UnitType{}, named("Bare")),
			retValue(call(ident("Err"), ident("Boom")))),
		mainDecl(letBind("v", q(call(ident("fail")))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	def := moduleDefineOf(ir, "__we_main")
	wantIR(t, ir, `c"error: Boom\0A"`, "the whole line as one constant")
	wantIR(t, def, "call void @__we_fail(ptr @.q0, i64 12)", "the constant report")
}

// The Ok binding reads the payload's own face: the shape entry's declared
// base name is what the interpolation domain and the arithmetic width
// answer from, exactly as a returned scalar's do — so the i64 converter
// renders the bound name, and no bool or f64 converter appears.
func TestQuestionOkBindingReadsThePayloadFace(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		stdIoImport("io"),
		appError(),
		pubFn("give", nil, resultOf(named("Int64"), named("AppError")),
			retValue(call(ident("Ok"), intLit("7i64")))),
		mainDecl(
			letBind("v", q(call(ident("give")))),
			ioCall("io", "println", interp(ident("v"))),
			okReturn(),
		),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "call %struct.we_str @__we_str_of_i64(", "the payload's own converter")
	if strings.Contains(ir, "__we_str_of_bool") || strings.Contains(ir, "__we_str_of_f64") {
		t.Fatalf("no other face's converter belongs here:\n%s", ir)
	}
}

// The caller's half of the prim return: the answer crosses as its own
// pointer, and the call site re-roots it right away — the callee's root
// died at its exit, and the ckPrim contract roots a primitive where it
// is made. The main body reaches the fn through its slot, so the call
// itself is the indirect `call ptr %v…()` — and nothing may sit between
// that call and the push.
func TestPrimReturnCallReRootsTheAnswer(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", File: &ast.File{Items: []ast.Item{
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		appError(),
		pubFn("make", nil, syncMutex(), retValue(mutexCtor())),
		mainDecl(letBind("m", call(ident("make"))), okReturn()),
	}}}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@slot.main.make = global ptr @main.make", "the slot the main body calls through")
	def := moduleDefineOf(ir, "__we_main")
	i := strings.Index(def, "= call ptr %")
	if i < 0 {
		t.Fatalf("no call through the slot:\n%s", def)
	}
	j := strings.Index(def[i:], "call void @__we_root_push(")
	if j < 0 {
		t.Fatalf("no re-root after the call:\n%s", def)
	}
	between := def[i : i+j]
	lines := strings.Split(between, "\n")
	if len(lines) != 2 || strings.TrimSpace(lines[1]) != "" || !strings.HasSuffix(lines[0], "()") {
		t.Fatalf("something other than the call sits before the push:\n%q", between)
	}
}
