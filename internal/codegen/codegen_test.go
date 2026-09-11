package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// okModule is the `we new` skeleton's root module: one erased sum and the
// main-shaped Ok return.
func okModule() *ast.File {
	return &ast.File{Items: []ast.Item{
		&ast.SumDecl{
			Pub:  true,
			Name: "AppError",
			Variants: []ast.Variant{
				{Name: "Failed", Payload: []ast.TypeRef{&ast.NamedType{Name: "String"}}},
			},
		},
		&ast.FnDecl{
			Pub:  true,
			Name: "main",
			Ret:  &ast.NamedType{Name: "Result", Args: []ast.TypeRef{&ast.UnitType{}, &ast.NamedType{Name: "AppError"}}},
			Body: ast.Block{Items: []ast.Stmt{&ast.Return{HasValue: true, Value: &ast.Call{Fn: &ast.Ident{Name: "Ok"}, Args: []ast.Expr{&ast.Unit{}}}}}},
		},
	}}
}

// errModule returns `return Err(Failed(payload))` with payload as the raw
// literal text (quotes included, as the parser stores it).
func errModule(payload string) *ast.File {
	f := okModule()
	f.Items[1].(*ast.FnDecl).Body.Items = []ast.Stmt{
		&ast.Return{HasValue: true, Value: &ast.Call{
			Fn:   &ast.Ident{Name: "Err"},
			Args: []ast.Expr{&ast.Call{Fn: &ast.Ident{Name: "Failed"}, Args: []ast.Expr{&ast.Literal{Kind: "string", Text: payload}}}},
		}},
	}
	return f
}

const okIR = `; ModuleID = 'demo'

define i32 @__we_main() {
entry:
  ret i32 0
}
`

// "error: Failed: boom\n" is 20 bytes: 15 of prefix, 4 of payload, newline.
const errIR = `; ModuleID = 'demo'

@.err = private unnamed_addr constant [20 x i8] c"error: Failed: boom\0A"

declare void @__we_fail(ptr, i64) noreturn

define i32 @__we_main() {
entry:
  call void @__we_fail(ptr @.err, i64 20)
  unreachable
}
`

func TestOkIR(t *testing.T) {
	ir, ni := Emit(okModule(), "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if ir != okIR {
		t.Fatalf("Ok-path IR mismatch:\nwant:\n%s\ngot:\n%s", okIR, ir)
	}
}

func TestErrIR(t *testing.T) {
	ir, ni := Emit(errModule("\"boom\""), "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if ir != errIR {
		t.Fatalf("Err-path IR mismatch:\nwant:\n%s\ngot:\n%s", errIR, ir)
	}
}

// The report line carries the DECODED payload bytes: We escapes resolve
// first, then the constant re-escapes for the IR text form — printable
// ASCII raw (quote and backslash excluded), everything else \XX uppercase
// hex. The doubled forms are out: the pinned toolchain's assembly lexer
// breaks on \" inside a c-string (verified against the real clang), so
// quote is \22 and backslash \5C.
func TestErrEscapeDecoding(t *testing.T) {
	cases := []struct {
		name      string
		payload   string
		constLine string
		length    int64
	}{
		{"tab quote newline", `"tab\there\"\n"`, `c"error: Failed: tab\09here\22\0A\0A"`, 26},
		{"unicode escape", `"\u{4e2d}\n"`, `c"error: Failed: \E4\B8\AD\0A\0A"`, 20},
		{"escaped backslash", `"a\\b\n"`, `c"error: Failed: a\5Cb\0A\0A"`, 20},
		{"nul and quote", `"\0\"\n"`, `c"error: Failed: \00\22\0A\0A"`, 19},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ir, ni := Emit(errModule(c.payload), "demo")
			if ni != nil {
				t.Fatalf("unexpected boundary: %s", ni.What)
			}
			want := "@.err = private unnamed_addr constant [" + itoa(c.length) + " x i8] " + c.constLine
			if !strings.Contains(ir, want) {
				t.Fatalf("constant line mismatch:\nwant substring: %s\ngot IR:\n%s", want, ir)
			}
			if !strings.Contains(ir, "i64 "+itoa(c.length)+")") {
				t.Fatalf("length operand mismatch:\ngot IR:\n%s", ir)
			}
		})
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// The body word, spelled once: most rows of the table below answer with
// it, and a row that means to pin a boundary of its own must not be able
// to match it by accident.
const BODYW = "main bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"

// Each What row fires on its own trigger (design D3's closed table).
func TestBoundaryWhats(t *testing.T) {
	cases := []struct {
		name string
		file *ast.File
		what string
	}{
		{
			// An Int64-literal initializer was outside M8's expression
			// subset; M9b's T6 emits numeric lets (the operand rides the
			// i64 domain — no instruction until a use), so the form pins
			// clean emission now (the flip rides the T6 disclosure).
			"binding in body",
			replaceBody(okModule(), []ast.Stmt{
				&ast.Binding{Kw: "let", Name: "x", Init: &ast.Literal{Kind: "int", Text: "1"}},
				&ast.Return{HasValue: true, Value: &ast.Call{Fn: &ast.Ident{Name: "Ok"}, Args: []ast.Expr{&ast.Unit{}}}},
			}),
			"",
		},
		{
			"tail-expression form",
			replaceBody(okModule(), []ast.Stmt{
				&ast.ExprStmt{Expr: &ast.Call{Fn: &ast.Ident{Name: "Ok"}, Args: []ast.Expr{&ast.Unit{}}}},
			}),
			"main bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)",
		},
		{
			"bare return",
			replaceBody(okModule(), []ast.Stmt{&ast.Return{}}),
			"main bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)",
		},
		// The four rows below were the M4 Err-payload table, whose one
		// accepted shape was a single string-literal variant argument.
		// T9-2 retires that word — the report line renders every payload
		// the variant declares — and what is left of these programs is
		// what stops them now: a payload-bearing variant named bare, a
		// name no binding holds, and a constructor the error sum does not
		// declare. Each is the body word's own trigger, not a report
		// boundary's: an arity or a name the check stage rejects.
		{
			"payload variant named bare as Err argument",
			replaceBody(errModule("\"boom\""), []ast.Stmt{
				&ast.Return{HasValue: true, Value: &ast.Call{
					Fn:   &ast.Ident{Name: "Err"},
					Args: []ast.Expr{&ast.Ident{Name: "Failed"}},
				}},
			}),
			BODYW,
		},
		{
			"payload name no binding holds",
			replaceBody(errModule("\"boom\""), []ast.Stmt{
				&ast.Return{HasValue: true, Value: &ast.Call{
					Fn:   &ast.Ident{Name: "Err"},
					Args: []ast.Expr{&ast.Call{Fn: &ast.Ident{Name: "Failed"}, Args: []ast.Expr{&ast.Ident{Name: "x"}}}},
				}},
			}),
			BODYW,
		},
		{
			"interpolated payload with an unbound hole",
			errModule("\"a${b}c\""),
			BODYW,
		},
		{
			"unknown payload constructor",
			replaceBody(errModule("\"boom\""), []ast.Stmt{
				&ast.Return{HasValue: true, Value: &ast.Call{
					Fn:   &ast.Ident{Name: "Err"},
					Args: []ast.Expr{&ast.Call{Fn: &ast.Ident{Name: "Unknown"}, Args: []ast.Expr{&ast.Literal{Kind: "string", Text: "\"x\""}}}},
				}},
			}),
			BODYW,
		},
		{
			// The M10b multi-function widening (design D8): a valueless
			// helper defines under its module-qualified symbol with its
			// slot and rides clean — the old other-fns row retired. The
			// define and slot land in the fn groups; main's bytes are
			// untouched.
			"extra module function",
			appendItem(okModule(), &ast.FnDecl{Name: "helper"}),
			"",
		},
		{
			// The T8-2 row. A String binding owns its pair of globals and
			// emits clean, like a scalar's — the two shapes differ in word
			// count, not in whether the face is covered.
			"String top-level binding",
			appendItem(okModule(), &ast.TopLet{Binding: ast.Binding{
				Kw: "let", Name: "greeting", Typ: named("String"), Init: strLit(`"hi"`),
			}}),
			"",
		},
		{
			// The T8-2B row. A handle into a collectable block owns one
			// global, and the root table can see it (the slot's address is
			// registered at the entry head) — so this face emits clean too,
			// and the three storage shapes are all covered.
			"gc handle top-level binding",
			appendItem(okModule(), &ast.TopLet{Binding: ast.Binding{
				Kw: "let", Name: "xs", Typ: &ast.NamedType{Name: "List", Args: []ast.TypeRef{named("Int64")}},
				Init: &ast.ListLit{Elems: []ast.Expr{intLit("1")}},
			}}),
			"",
		},
		{
			"scalar top-level binding",
			appendItem(okModule(), &ast.TopLet{Binding: ast.Binding{
				Kw: "let", Name: "answer", Typ: named("Int64"), Init: intLit("42"),
			}}),
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ni := Emit(c.file, "demo")
			if c.what == "" {
				// The flipped-green form: clean emission.
				if ni != nil {
					t.Fatalf("expected clean emission, got boundary %q", ni.What)
				}
				return
			}
			if ni == nil {
				t.Fatalf("expected boundary %q, got none", c.what)
			}
			if ni.What != c.what {
				t.Fatalf("boundary What mismatch:\nwant: %q\ngot:  %q", c.what, ni.What)
			}
		})
	}
}

// Two payloads of Int64: the multi-payload variant form (golden
// build-bnd-err-payload's module shape).
// T9-2 (design D8): the report line renders every payload word the
// variant declares, in declaration order. This program is the
// two-word shape the retired one-string-literal rule used to stop, and
// it is the module the build-err-payload-eager golden compiles: the
// constant folds through the same constant path the one-word payload
// took, so what the rule's removal bought is a wider line, not a wider
// emitter.
func TestErrReportMultiPayload(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		&ast.SumDecl{
			Pub:  true,
			Name: "AppError",
			Variants: []ast.Variant{
				{Name: "Failed", Payload: []ast.TypeRef{&ast.NamedType{Name: "Int64"}, &ast.NamedType{Name: "Int64"}}},
			},
		},
		&ast.FnDecl{
			Pub:  true,
			Name: "main",
			Ret:  &ast.NamedType{Name: "Result", Args: []ast.TypeRef{&ast.UnitType{}, &ast.NamedType{Name: "AppError"}}},
			Body: ast.Block{Items: []ast.Stmt{&ast.Return{HasValue: true, Value: &ast.Call{
				Fn: &ast.Ident{Name: "Err"},
				Args: []ast.Expr{&ast.Call{Fn: &ast.Ident{Name: "Failed"}, Args: []ast.Expr{
					&ast.Literal{Kind: "int", Text: "1i64"}, &ast.Literal{Kind: "int", Text: "2i64"},
				}}},
			}}}},
		},
	}}
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	// "error: Failed: 1, 2\n" is 20 bytes: 15 of prefix, 4 of payload
	// ("1, 2" — the separator belongs to the renderer, not the payload),
	// newline.
	wantIR(t, ir, `@.err = private unnamed_addr constant [20 x i8] c"error: Failed: 1, 2\0A"`, "the report line")
	wantIR(t, ir, "call void @__we_fail(ptr @.err, i64 20)", "the fail call")
}

// Emission is a pure function: two calls over the same module agree
// byte-for-byte (chapter 21's same-input-same-output).
func TestEmitDeterministic(t *testing.T) {
	a, _ := Emit(errModule("\"boom\""), "demo")
	b, _ := Emit(errModule("\"boom\""), "demo")
	if a != b {
		t.Fatalf("two emissions of the same module differ")
	}
}

func replaceBody(f *ast.File, items []ast.Stmt) *ast.File {
	f.Items[1].(*ast.FnDecl).Body.Items = items
	return f
}

func appendItem(f *ast.File, it ast.Item) *ast.File {
	f.Items = append(f.Items, it)
	return f
}

// T9-2 (design D8): a payload that is not a constant cannot fold into the
// report line, so the line renders at run time through the same
// value-to-String face interpolation uses — __we_str_of_* per word, then
// the runtime's own concatenation, with the separator and the newline as
// interned literals of their own. __we_fail still takes the pair the
// chain ends on, so the face it presents to the runtime is unchanged.
func TestErrReportRendersNonConstantPayload(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		bigErr(),
		mainDecl(
			letBind("n", intLit("41")),
			&ast.Return{HasValue: true, Value: call(ident("Err"), call(ident("Failed"), intLit("11i64"), ident("n")))},
		),
	}}
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	wantIR(t, ir, `@.s0 = private unnamed_addr constant [15 x i8] c"error: Failed: "`, "the literal head")
	wantIR(t, ir, `@.s1 = private unnamed_addr constant [2 x i8] c", "`, "the separator")
	wantIR(t, ir, "declare %struct.we_str @__we_str_of_i64(i64)", "the word renderer")
	for _, frag := range []string{
		"call %struct.we_str @__we_str_of_i64(i64 11)",
		"call %struct.we_str @__we_str_of_i64(i64 41)",
		"call %struct.we_str @__we_str_concat(ptr @.s0, i64 15,",
		"i64 %v8, ptr @.s1, i64 2)",
	} {
		wantIR(t, ir, frag, "the report chain")
	}
	wantIR(t, ir, "call void @__we_fail(ptr %v16, i64 %v17)", "the fail call")
	wantNoIR(t, ir, "@.err = private", "a constant folded line")
}

// Two report sites in one entry do not share a line. `@.err` keeps the
// M4 spelling for the first and the rest take a numbered global, so a
// deep return and the tail return each print the bytes they spelled —
// the shape a main with an early error beside its trailing one reaches.
func TestErrReportSitesDoNotShareAGlobal(t *testing.T) {
	f := &ast.File{Items: []ast.Item{appError(), mainDecl(
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", intLit("1"), intLit("1")),
			Then: ast.Block{Items: []ast.Stmt{&ast.Return{HasValue: true, Value: &ast.Call{
				Fn:   ident("Err"),
				Args: []ast.Expr{&ast.Call{Fn: ident("Failed"), Args: []ast.Expr{strLit(`"early"`)}}},
			}}}},
		}},
		&ast.Return{HasValue: true, Value: &ast.Call{
			Fn:   ident("Err"),
			Args: []ast.Expr{&ast.Call{Fn: ident("Failed"), Args: []ast.Expr{strLit(`"late"`)}}},
		}},
	)}}
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	// "error: Failed: early\n" is 21 bytes, "...late\n" is 20.
	wantIR(t, ir, `@.err = private unnamed_addr constant [21 x i8] c"error: Failed: early\0A"`, "the early return's line")
	wantIR(t, ir, `@.err1 = private unnamed_addr constant [20 x i8] c"error: Failed: late\0A"`, "the tail return's line")
	wantIR(t, ir, "call void @__we_fail(ptr @.err, i64 21)", "the early site's fail call")
	wantIR(t, ir, "call void @__we_fail(ptr @.err1, i64 20)", "the tail site's fail call")
}
