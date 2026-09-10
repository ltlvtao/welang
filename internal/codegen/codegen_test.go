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
		{
			"unit variant as Err argument",
			replaceBody(errModule("\"boom\""), []ast.Stmt{
				&ast.Return{HasValue: true, Value: &ast.Call{
					Fn:   &ast.Ident{Name: "Err"},
					Args: []ast.Expr{&ast.Ident{Name: "Failed"}},
				}},
			}),
			"Err payloads beyond one plain string-literal variant argument",
		},
		{
			"non-literal payload",
			replaceBody(errModule("\"boom\""), []ast.Stmt{
				&ast.Return{HasValue: true, Value: &ast.Call{
					Fn:   &ast.Ident{Name: "Err"},
					Args: []ast.Expr{&ast.Call{Fn: &ast.Ident{Name: "Failed"}, Args: []ast.Expr{&ast.Ident{Name: "x"}}}},
				}},
			}),
			"Err payloads beyond one plain string-literal variant argument",
		},
		{
			"interpolated payload",
			errModule("\"a${b}c\""),
			"Err payloads beyond one plain string-literal variant argument",
		},
		{
			"unknown payload constructor",
			replaceBody(errModule("\"boom\""), []ast.Stmt{
				&ast.Return{HasValue: true, Value: &ast.Call{
					Fn:   &ast.Ident{Name: "Err"},
					Args: []ast.Expr{&ast.Call{Fn: &ast.Ident{Name: "Unknown"}, Args: []ast.Expr{&ast.Literal{Kind: "string", Text: "\"x\""}}}},
				}},
			}),
			"Err payloads beyond one plain string-literal variant argument",
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
			// The T8-1 row. A scalar binding owns its global and emits
			// clean; a carrier is the face that still stops, so the row
			// carries a String — the shape the boundary word now covers.
			"carrier top-level binding",
			appendItem(okModule(), &ast.TopLet{Binding: ast.Binding{
				Kw: "let", Name: "greeting", Typ: named("String"), Init: strLit(`"hi"`),
			}}),
			"top-level value bindings in code generation",
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
func TestBoundaryWhatMultiPayload(t *testing.T) {
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
	_, ni := Emit(f, "demo")
	if ni == nil || ni.What != "Err payloads beyond one plain string-literal variant argument" {
		t.Fatalf("want Err-payload boundary, got: %v", ni)
	}
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
