package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M12 (ffi) chapter 19 codegen tests (design D4/D5): the declare
// emission for foreign fn entries, the positional ABI map (String and
// Bytes as the (ptr, i64) two-scalar expansion, Bool as i8, Rune as
// i32, opaque as ptr, Never as void), the direct call without a slot,
// and the foreign-name collection the link tower consumes. Written
// test-first: red today on the undefined symbols alone (ast.ForeignBlock,
// ForeignNames, the Foreign marker).

// foreignFn builds one foreign fn entry.
func foreignFn(name string, params []ast.Param, ret ast.TypeRef) *ast.FnDecl {
	return &ast.FnDecl{Name: name, Params: params, Ret: ret, Foreign: true,
		EffectTags: []string{}, EffectLine: 1, EffectCol: 1}
}

// foreignMod wraps a foreign block (plus optional main-shaped
// statements) into the root build module.
func foreignMod(fb *ast.ForeignBlock, mainBody ...ast.Stmt) ProgModule {
	items := []ast.Item{fb, appError()}
	if mainBody != nil {
		items = append(items, mainDecl(mainBody...))
	}
	return ProgModule{Key: "main", ID: "demo", File: &ast.File{Items: items}}
}

// --- D4: declare emission and the ABI map ------------------------------------------

func TestForeignDeclareEmission(t *testing.T) {
	fb := &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		foreignFn("abs", []ast.Param{i64Param("x")}, named("Int64")),
	}}
	// The entry rides along (the fixture typo T5 caught: a main-less
	// module stops at the defensive entry boundary — in the real pipeline
	// E1305 owns that face ahead of codegen, so a fixture that skips
	// typecheck must still carry a main).
	ir, ni := EmitProgram(ModeBuild, []ProgModule{foreignMod(fb, okReturn())})
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if !strings.Contains(ir, "declare i64 @abs(i64)") {
		t.Fatalf("missing declare line:\n%s", ir)
	}
}

func TestForeignABIMap(t *testing.T) {
	fb := &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		// The scalar families: eight integers at width, floats, Bool i8,
		// Rune i32.
		foreignFn("scalars",
			[]ast.Param{
				{Name: "a", Type: named("Int8")},
				{Name: "b", Type: named("Int32")},
				{Name: "c", Type: named("UInt8")},
				{Name: "d", Type: named("UInt64")},
				{Name: "e", Type: named("Float32")},
				{Name: "f", Type: named("Float64")},
				{Name: "g", Type: named("Bool")},
				{Name: "h", Type: named("Rune")},
			}, nil),
		// String and Bytes expand one We parameter into (ptr, i64).
		foreignFn("buffers",
			[]ast.Param{
				{Name: "s", Type: named("String")},
				{Name: "bs", Type: named("Bytes")},
			}, nil),
		// An opaque travels as a single ptr.
		&ast.RecordDecl{Name: "Box", Cat: "gc", Opaque: true},
		foreignFn("opaque", []ast.Param{{Name: "o", Type: named("Box")}}, named("Box")),
		// Never returns void.
		foreignFn("abortNow", nil, named("Never")),
	}}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{foreignMod(fb, okReturn())})
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	for _, want := range []string{
		"declare void @scalars(i8, i32, i8, i64, float, double, i8, i32)",
		"declare void @buffers(ptr, i64, ptr, i64)",
		"declare ptr @opaque(ptr)",
		"declare void @abortNow()",
	} {
		if !strings.Contains(ir, want) {
			t.Fatalf("missing %q in:\n%s", want, ir)
		}
	}
}

// A call to a foreign fn goes straight to the native symbol — no slot
// (mocking is not a goal), no module-key qualifier.
func TestForeignCallDirect(t *testing.T) {
	fb := &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		foreignFn("abs", []ast.Param{i64Param("x")}, named("Int64")),
	}}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{foreignMod(fb,
		letDiscard(&ast.Call{Fn: ident("abs"), Args: []ast.Expr{intLit("-3")}}),
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if strings.Contains(ir, "@main.abs") {
		t.Fatalf("a foreign call must not go through a module slot:\n%s", ir)
	}
	if !strings.Contains(ir, "call i64 @abs(") {
		t.Fatalf("missing direct call:\n%s", ir)
	}
}

// A Never-returning foreign call diverges: the call is followed by
// unreachable in its branch.
func TestForeignNeverCall(t *testing.T) {
	fb := &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		foreignFn("abortNow", nil, named("Never")),
	}}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{foreignMod(fb,
		&ast.ExprStmt{Expr: &ast.Call{Fn: ident("abortNow")}},
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	if !strings.Contains(ir, "unreachable") {
		t.Fatalf("a Never call must be followed by unreachable:\n%s", ir)
	}
}

// A Never-returning foreign call inside a branch arm diverges the arm.
// The divergence protocol's guarded joins keep the emission well-formed:
// after the arm's unreachable nothing may emit until a fresh label opens,
// and the code after the branch lives on in the join. Red before the
// guards (real toolchain): the arm's unconditional `br label %join`
// landed after the `unreachable` — clang rejected the IR.
func TestForeignNeverBranchJoin(t *testing.T) {
	fb := &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		foreignFn("abortNow", nil, named("Never")),
	}}
	never := &ast.ExprStmt{Expr: &ast.Call{Fn: ident("abortNow")}}
	eq := func(lit string) ast.Expr { return binOp("==", ident("flag"), intLit(lit)) }
	bind := &ast.Binding{Kw: "let", Name: "flag", Typ: named("Int64"), Init: intLit("0")}

	for _, tc := range []struct {
		name string
		stmt ast.Stmt
		join bool // an explicit br to the join is still emitted (an
		// undiverged arm; the no-else shape reaches the join through the
		// cond's false edge instead — no explicit branch exists by design)
	}{
		{"then-arm diverges", &ast.ExprStmt{Expr: &ast.If{
			Cond: eq("1"),
			Then: ast.Block{Items: []ast.Stmt{never}},
		}}, false},
		{"else-arm diverges", &ast.ExprStmt{Expr: &ast.If{
			Cond: eq("0"),
			Then: ast.Block{Items: []ast.Stmt{&ast.Binding{Kw: "let", Name: "_", Init: intLit("0")}}},
			Else: blockOf(never),
		}}, true},
		{"both arms diverge", &ast.ExprStmt{Expr: &ast.If{
			Cond: eq("1"),
			Then: ast.Block{Items: []ast.Stmt{never}},
			Else: blockOf(never),
		}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ir, ni := Emit(foreignMod(fb, bind, tc.stmt, okReturn()).File, "demo")
			if ni != nil {
				t.Fatalf("unexpected boundary: %s", ni.What)
			}
			if strings.Contains(ir, "unreachable\n  br") {
				t.Fatalf("an instruction lands after a terminator:\n%s", ir)
			}
			// The join label always opens — the post-if code lives there.
			if !strings.Contains(ir, "ifjoin") {
				t.Fatalf("missing join label:\n%s", ir)
			}
			if tc.join != strings.Contains(ir, "br label %ifjoin") {
				t.Fatalf("join branch presence mismatch (want %v):\n%s", tc.join, ir)
			}
		})
	}
}

// --- D5: the name collection the link tower consumes --------------------------------

func TestForeignNamesCollected(t *testing.T) {
	mk := func(key string, fb *ast.ForeignBlock) ProgModule {
		return ProgModule{Key: key, ID: key, File: &ast.File{Items: []ast.Item{fb}}}
	}
	root := mk("main", &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		foreignFn("abs", []ast.Param{i64Param("x")}, named("Int64")),
	}})
	net := mk("net", &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		foreignFn("dial", []ast.Param{{Name: "addr", Type: named("String")}}, nil),
	}})
	names := ForeignNames([]ProgModule{root, net})
	if len(names) != 2 {
		t.Fatalf("want 2 names, got %+v", names)
	}
	if names[0].Module != "main" || names[0].Name != "abs" {
		t.Fatalf("names[0]: %+v", names[0])
	}
	if names[1].Module != "net" || names[1].Name != "dial" {
		t.Fatalf("names[1]: %+v", names[1])
	}
	// Opaque records declare no symbol of their own.
	only := mk("one", &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		&ast.RecordDecl{Name: "Box", Cat: "gc", Opaque: true},
	}})
	if got := ForeignNames([]ProgModule{only}); len(got) != 0 {
		t.Fatalf("an opaque record declares no symbol, got %+v", got)
	}
}
