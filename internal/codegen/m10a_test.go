package codegen

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M10a design D9: a test module stops at the code-generation boundary.
// Two orders, both honest: a module of only test blocks stops at the
// test-module row; a helper fn before the first test stops at the
// other-functions row first (the item walk decides in source order,
// before any body is read — the helper's own emission is M10b's
// multi-function widening). The std.test import erases through the
// existing std path (the M9a concurrent precedent), and the
// unreachable-below-the-stop argument — MockDecl and advanceTime never
// have walk cases because nothing below the stop can contain them — is
// pinned by the walk's source itself (codegen.go's TestDecl case).
func TestM10aTestModuleBoundary(t *testing.T) {
	onlyTests := &ast.File{Items: []ast.Item{
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		&ast.TestDecl{Desc: "only", Line: 2, Col: 1},
	}}
	_, ni := Emit(onlyTests, "demo")
	if ni == nil || ni.What != bndTestModule {
		t.Fatalf("only-tests module: want %q, got %+v", bndTestModule, ni)
	}

	fnsFirst := &ast.File{Items: []ast.Item{
		&ast.FnDecl{Name: "abs", Line: 1, Col: 1,
			Params: []ast.Param{{Name: "n", Type: &ast.NamedType{Name: "Int64"}}},
			Ret:    &ast.NamedType{Name: "Int64"}},
		&ast.TestDecl{Desc: "absolute", Line: 5, Col: 1},
	}}
	_, ni = Emit(fnsFirst, "demo")
	if ni == nil || ni.What != bndOtherFns {
		t.Fatalf("fns-first module: want %q, got %+v", bndOtherFns, ni)
	}
}
