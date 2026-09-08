package codegen

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M8 (stdlib-and-gc) emission tests: the main-body statement sequence, the
// gc-record construction protocol (alloc -> map store -> root push -> field
// stores, outer rooted before any nested allocation), the String double-word
// operands, and the boundary vocabulary of the widened accepted set (design
// D4/D5/D6). Snapshots are byte-exact like the M4 pair above.

// stdIoImport is the root module's std.io import, aliased when alias != "".
func stdIoImport(alias string) *ast.Import {
	imp := &ast.Import{Path: []string{"std", "io"}}
	if alias != "" {
		imp.Alias = alias
	}
	return imp
}

// appError is the `we new` skeleton's erased error sum.
func appError() *ast.SumDecl {
	return &ast.SumDecl{
		Pub:      true,
		Name:     "AppError",
		Variants: []ast.Variant{{Name: "Failed", Payload: []ast.TypeRef{&ast.NamedType{Name: "String"}}}},
	}
}

// mainDecl wraps statements into the main-shaped declaration.
func mainDecl(stmts ...ast.Stmt) *ast.FnDecl {
	return &ast.FnDecl{
		Pub:  true,
		Name: "main",
		Ret:  &ast.NamedType{Name: "Result", Args: []ast.TypeRef{&ast.UnitType{}, &ast.NamedType{Name: "AppError"}}},
		Body: ast.Block{Items: stmts},
	}
}

func okReturn() *ast.Return {
	return &ast.Return{HasValue: true, Value: &ast.Call{Fn: &ast.Ident{Name: "Ok"}, Args: []ast.Expr{&ast.Unit{}}}}
}

func ioCall(qual, name string, arg ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{Expr: &ast.Call{
		Fn:   &ast.Member{Recv: &ast.Ident{Name: qual}, Name: name},
		Args: []ast.Expr{arg},
	}}
}

// helloModule is the golden hello program: a let of a String literal, the
// println through the import name, the Ok tail.
func helloModule(alias string) *ast.File {
	return &ast.File{Items: []ast.Item{
		stdIoImport(alias),
		appError(),
		mainDecl(
			&ast.Binding{Kw: "let", Name: "name", Init: &ast.Literal{Kind: "string", Text: `"We"`}},
			ioCall("io", "println", &ast.Ident{Name: "name"}),
			okReturn(),
		),
	}}
}

// M10b slots the io pair (design D3): the call loads the slot global, so
// a mock can fill it — the bytes the M8 goldens pinned change by exactly
// that (one slot line, the load, the indirect call), everything else
// rides byte-identical.
const m8HelloIR = `; ModuleID = 'demo'

@.s0 = private unnamed_addr constant [2 x i8] c"We"

@slot.io.println = global ptr @__we_println

declare void @__we_println(ptr, i64)

define i32 @__we_main() {
entry:
  %v0 = load ptr, ptr @slot.io.println
  call void %v0(ptr @.s0, i64 2)
  ret i32 0
}
`

func TestM8HelloIR(t *testing.T) {
	ir, ni := Emit(helloModule(""), "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if ir != m8HelloIR {
		t.Fatalf("hello IR mismatch:\nwant:\n%s\ngot:\n%s", m8HelloIR, ir)
	}
}

// The import item itself leaves no IR trace (std imports erase; only the
// calls they enable appear), and the alias form reaches the same emission —
// the qualifier resolves through the file's std.io import, not by spelling.
func TestM8ImportErasure(t *testing.T) {
	base := helloModule("")
	base.Items = base.Items[1:] // drop the import, keep everything else
	plain, ni := Emit(base, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary without import: %s", ni.What)
	}
	if plain != m8HelloIR {
		t.Fatalf("import-less IR should equal the imported form:\nwant:\n%s\ngot:\n%s", m8HelloIR, plain)
	}
	aliased := helloModule("out")
	aliased.Items[2].(*ast.FnDecl).Body.Items[1] = ioCall("out", "println", &ast.Ident{Name: "name"})
	ir, ni := Emit(aliased, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary on alias form: %s", ni.What)
	}
	if ir != m8HelloIR {
		t.Fatalf("alias IR mismatch:\nwant:\n%s\ngot:\n%s", m8HelloIR, ir)
	}
}

// A local import erases at the single-module face: the imported name is
// not part of this program (no module keyed b rides a single-file Emit),
// the hello body never calls through it, and the import itself emits
// nothing — the M10b widening made cross-module links call-site facts
// (the fn table), not module-level stops. The IR equals the plain hello.
func TestM8LocalImportBoundary(t *testing.T) {
	f := helloModule("")
	f.Items[0] = &ast.Import{Path: []string{"b"}}
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary on local import: %s", ni.What)
	}
	if ir != m8HelloIR {
		t.Fatalf("local-import IR should equal the plain hello:\nwant:\n%s\ngot:\n%s", m8HelloIR, ir)
	}
}

// print is println's un-newlined sibling; both io symbols ride the same
// operand form. A unit discard binding (`let _ =`) emits the call and drops
// the value (chapter 8's discard exemption).
func TestM8PrintAndDiscardIR(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		stdIoImport(""),
		appError(),
		mainDecl(
			&ast.Binding{Kw: "let", Name: "_", Init: &ast.Call{
				Fn:   &ast.Member{Recv: &ast.Ident{Name: "io"}, Name: "print"},
				Args: []ast.Expr{&ast.Literal{Kind: "string", Text: `"hi"`}},
			}},
			okReturn(),
		),
	}}
	want := `; ModuleID = 'demo'

@.s0 = private unnamed_addr constant [2 x i8] c"hi"

@slot.io.print = global ptr @__we_print

declare void @__we_print(ptr, i64)

define i32 @__we_main() {
entry:
  %v0 = load ptr, ptr @slot.io.print
  call void %v0(ptr @.s0, i64 2)
  ret i32 0
}
`
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if ir != want {
		t.Fatalf("print/discard IR mismatch:\nwant:\n%s\ngot:\n%s", want, ir)
	}
}

// greeterModule constructs one gc record and prints a field through it.
func greeterModule() *ast.File {
	return &ast.File{Items: []ast.Item{
		stdIoImport(""),
		appError(),
		&ast.RecordDecl{
			Pub:    true,
			Cat:    "gc",
			Name:   "Greeter",
			Fields: []ast.FieldDecl{{Name: "greeting", Typ: &ast.NamedType{Name: "String"}}},
		},
		mainDecl(
			&ast.Binding{Kw: "let", Name: "g", Init: &ast.Construct{
				Name:   "Greeter",
				Fields: []ast.FieldInit{{Name: "greeting", Value: &ast.Literal{Kind: "string", Text: `"hello"`}}},
			}},
			ioCall("io", "println", &ast.Member{Recv: &ast.Ident{Name: "g"}, Name: "greeting"}),
			okReturn(),
		),
	}}
}

// Object header {map@0, size@8}, fields from 16; Greeter is header + one
// String double-word = 32 bytes. The map descriptor is the reference-slot
// bitmap; a String field is not a GC reference in M8 (all buffers are
// constants — D5), so Greeter's bitmap is zero. Root push follows the map
// store of every construction (before field stores — a nested allocation
// must never run while its parent is unrooted); pops ride the reverse order
// at the single ret. A String let needs no root: not a gc record.
const m8RecordIR = `; ModuleID = 'demo'

%struct.main.Greeter = type { ptr, i64 }

@.s0 = private unnamed_addr constant [5 x i8] c"hello"

@.map.main.Greeter = private unnamed_addr constant [1 x i64] [i64 0]

@slot.io.println = global ptr @__we_println

declare ptr @__we_alloc(i64)
declare void @__we_root_push(ptr)
declare void @__we_root_pop()
declare void @__we_println(ptr, i64)

define i32 @__we_main() {
entry:
  %v0 = call ptr @__we_alloc(i64 32)
  store ptr @.map.main.Greeter, ptr %v0
  call void @__we_root_push(ptr %v0)
  %v1 = getelementptr i8, ptr %v0, i64 16
  store ptr @.s0, ptr %v1
  %v2 = getelementptr i8, ptr %v0, i64 24
  store i64 5, ptr %v2
  %v3 = getelementptr i8, ptr %v0, i64 16
  %v4 = load ptr, ptr %v3
  %v5 = getelementptr i8, ptr %v0, i64 24
  %v6 = load i64, ptr %v5
  %v7 = load ptr, ptr @slot.io.println
  call void %v7(ptr %v4, i64 %v6)
  call void @__we_root_pop()
  ret i32 0
}
`

func TestM8RecordIR(t *testing.T) {
	ir, ni := Emit(greeterModule(), "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if ir != m8RecordIR {
		t.Fatalf("record IR mismatch:\nwant:\n%s\ngot:\n%s", m8RecordIR, ir)
	}
}

// The nested-construction protocol: Wrapper roots before Point allocates,
// Point's temp root outlives its store into the parent (pops only at ret,
// reverse order — M8 main bodies are straight-line, so a rooted-dead value
// simply survives). Wrapper's bitmap marks slot 0 (the Point reference at
// offset 16); the String field words stay clear. Field-chain loads walk
// pointer hops: load w.point, then the String pair inside it.
func TestM8NestedRecordIR(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		stdIoImport(""),
		appError(),
		&ast.RecordDecl{
			Pub:    true,
			Cat:    "gc",
			Name:   "Point",
			Fields: []ast.FieldDecl{{Name: "name", Typ: &ast.NamedType{Name: "String"}}},
		},
		&ast.RecordDecl{
			Pub:  true,
			Cat:  "gc",
			Name: "Wrapper",
			Fields: []ast.FieldDecl{
				{Name: "point", Typ: &ast.NamedType{Name: "Point"}},
				{Name: "label", Typ: &ast.NamedType{Name: "String"}},
			},
		},
		mainDecl(
			&ast.Binding{Kw: "let", Name: "w", Init: &ast.Construct{
				Name: "Wrapper",
				Fields: []ast.FieldInit{
					{Name: "point", Value: &ast.Construct{
						Name:   "Point",
						Fields: []ast.FieldInit{{Name: "name", Value: &ast.Literal{Kind: "string", Text: `"p"`}}},
					}},
					{Name: "label", Value: &ast.Literal{Kind: "string", Text: `"l"`}},
				},
			}},
			ioCall("io", "println", &ast.Member{
				Recv: &ast.Member{Recv: &ast.Ident{Name: "w"}, Name: "point"},
				Name: "name",
			}),
			okReturn(),
		),
	}}
	want := `; ModuleID = 'demo'

%struct.main.Point = type { ptr, i64 }
%struct.main.Wrapper = type { ptr, ptr, i64 }

@.s0 = private unnamed_addr constant [1 x i8] c"p"
@.s1 = private unnamed_addr constant [1 x i8] c"l"

@.map.main.Point = private unnamed_addr constant [1 x i64] [i64 0]
@.map.main.Wrapper = private unnamed_addr constant [1 x i64] [i64 1]

@slot.io.println = global ptr @__we_println

declare ptr @__we_alloc(i64)
declare void @__we_root_push(ptr)
declare void @__we_root_pop()
declare void @__we_println(ptr, i64)

define i32 @__we_main() {
entry:
  %v0 = call ptr @__we_alloc(i64 40)
  store ptr @.map.main.Wrapper, ptr %v0
  call void @__we_root_push(ptr %v0)
  %v1 = call ptr @__we_alloc(i64 32)
  store ptr @.map.main.Point, ptr %v1
  call void @__we_root_push(ptr %v1)
  %v2 = getelementptr i8, ptr %v1, i64 16
  store ptr @.s0, ptr %v2
  %v3 = getelementptr i8, ptr %v1, i64 24
  store i64 1, ptr %v3
  %v4 = getelementptr i8, ptr %v0, i64 16
  store ptr %v1, ptr %v4
  %v5 = getelementptr i8, ptr %v0, i64 24
  store ptr @.s1, ptr %v5
  %v6 = getelementptr i8, ptr %v0, i64 32
  store i64 1, ptr %v6
  %v7 = getelementptr i8, ptr %v0, i64 16
  %v8 = load ptr, ptr %v7
  %v9 = getelementptr i8, ptr %v8, i64 16
  %v10 = load ptr, ptr %v9
  %v11 = getelementptr i8, ptr %v8, i64 24
  %v12 = load i64, ptr %v11
  %v13 = load ptr, ptr @slot.io.println
  call void %v13(ptr %v10, i64 %v12)
  call void @__we_root_pop()
  call void @__we_root_pop()
  ret i32 0
}
`
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if ir != want {
		t.Fatalf("nested-record IR mismatch:\nwant:\n%s\ngot:\n%s", want, ir)
	}
}

// The Err tail after statements: the record construction emits in full, the
// fail call ends the function — no pops on a noreturn path (nothing runs
// after it), the process dies inside __we_fail.
func TestM8RecordErrIR(t *testing.T) {
	f := &ast.File{Items: []ast.Item{
		stdIoImport(""),
		appError(),
		&ast.RecordDecl{
			Pub:    true,
			Cat:    "gc",
			Name:   "Greeter",
			Fields: []ast.FieldDecl{{Name: "greeting", Typ: &ast.NamedType{Name: "String"}}},
		},
		mainDecl(
			&ast.Binding{Kw: "let", Name: "g", Init: &ast.Construct{
				Name:   "Greeter",
				Fields: []ast.FieldInit{{Name: "greeting", Value: &ast.Literal{Kind: "string", Text: `"hi"`}}},
			}},
			&ast.Return{HasValue: true, Value: &ast.Call{
				Fn:   &ast.Ident{Name: "Err"},
				Args: []ast.Expr{&ast.Call{Fn: &ast.Ident{Name: "Failed"}, Args: []ast.Expr{&ast.Literal{Kind: "string", Text: `"boom"`}}}},
			}},
		),
	}}
	want := `; ModuleID = 'demo'

%struct.main.Greeter = type { ptr, i64 }

@.s0 = private unnamed_addr constant [2 x i8] c"hi"

@.map.main.Greeter = private unnamed_addr constant [1 x i64] [i64 0]

@.err = private unnamed_addr constant [20 x i8] c"error: Failed: boom\0A"

declare ptr @__we_alloc(i64)
declare void @__we_root_push(ptr)
declare void @__we_fail(ptr, i64) noreturn

define i32 @__we_main() {
entry:
  %v0 = call ptr @__we_alloc(i64 32)
  store ptr @.map.main.Greeter, ptr %v0
  call void @__we_root_push(ptr %v0)
  %v1 = getelementptr i8, ptr %v0, i64 16
  store ptr @.s0, ptr %v1
  %v2 = getelementptr i8, ptr %v0, i64 24
  store i64 2, ptr %v2
  call void @__we_fail(ptr @.err, i64 20)
  unreachable
}
`
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if ir != want {
		t.Fatalf("record-Err IR mismatch:\nwant:\n%s\ngot:\n%s", want, ir)
	}
}

// Supersets of the accepted set stop at the widened vocabulary (design D4's
// new What row). Each case fires on its own trigger.
func TestM8BodyBoundaryWhats(t *testing.T) {
	newWhat := "main bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"
	cases := []struct {
		name string
		file *ast.File
	}{
		{
			"arithmetic in io argument",
			helloModulePatch(func(f *ast.File) {
				f.Items[2].(*ast.FnDecl).Body.Items[1] = ioCall("io", "println", &ast.Binary{
					Op: "+", L: &ast.Literal{Kind: "string", Text: `"a"`}, R: &ast.Ident{Name: "name"},
				})
			}),
		},
		{
			"interpolated literal",
			helloModulePatch(func(f *ast.File) {
				f.Items[2].(*ast.FnDecl).Body.Items[1] = ioCall("io", "println", &ast.Literal{Kind: "string", Text: `"hello ${name}"`})
			}),
		},
		{
			"control flow",
			helloModulePatch(func(f *ast.File) {
				f.Items[2].(*ast.FnDecl).Body.Items = append([]ast.Stmt{&ast.ExprStmt{Expr: &ast.If{
					Cond: &ast.Literal{Kind: "bool", Text: "true"},
					Then: ast.Block{Items: []ast.Stmt{okReturn()}},
				}}}, f.Items[2].(*ast.FnDecl).Body.Items...)
			}),
		},
		{
			"method call on a record",
			func() *ast.File {
				f := greeterModule()
				mainBody := &f.Items[3].(*ast.FnDecl).Body
				mainBody.Items = []ast.Stmt{
					mainBody.Items[0],
					&ast.Binding{Kw: "let", Name: "text", Init: &ast.Call{
						Fn: &ast.Member{Recv: &ast.Ident{Name: "g"}, Name: "greet"},
					}},
					okReturn(),
				}
				return f
			}(),
		},
		{
			"bare-ident let initializer",
			helloModulePatch(func(f *ast.File) {
				f.Items[2].(*ast.FnDecl).Body.Items = append(
					[]ast.Stmt{&ast.Binding{Kw: "let", Name: "copy", Init: &ast.Ident{Name: "name"}}},
					f.Items[2].(*ast.FnDecl).Body.Items...)
			}),
		},
		{
			"non-io call statement",
			helloModulePatch(func(f *ast.File) {
				f.Items[2].(*ast.FnDecl).Body.Items = append([]ast.Stmt{&ast.ExprStmt{Expr: &ast.Call{Fn: &ast.Ident{Name: "helper"}}}}, f.Items[2].(*ast.FnDecl).Body.Items...)
			}),
		},
		{
			"tuple-pattern binding",
			helloModulePatch(func(f *ast.File) {
				f.Items[2].(*ast.FnDecl).Body.Items[0] = &ast.Binding{
					Kw:  "let",
					Pat: &ast.PatTuple{Elems: []ast.Pattern{&ast.PatBinding{Name: "a"}, &ast.PatBinding{Name: "b"}}},
					Init: &ast.Construct{
						Name:   "Greeter",
						Fields: []ast.FieldInit{{Name: "greeting", Value: &ast.Literal{Kind: "string", Text: `"x"`}}},
					},
				}
				f.Items = append([]ast.Item{&ast.RecordDecl{
					Pub: true, Cat: "gc", Name: "Greeter",
					Fields: []ast.FieldDecl{{Name: "greeting", Typ: &ast.NamedType{Name: "String"}}},
				}}, f.Items...)
			}),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ni := Emit(c.file, "demo")
			if ni == nil {
				t.Fatalf("expected boundary %q, got none", newWhat)
			}
			if ni.What != newWhat {
				t.Fatalf("boundary What mismatch:\nwant: %q\ngot:  %q", newWhat, ni.What)
			}
		})
	}
}

// helloModulePatch clones the hello program and applies fn to the copy.
func helloModulePatch(fn func(*ast.File)) *ast.File {
	f := helloModule("")
	fn(f)
	return f
}
