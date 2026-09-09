package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M10c (design D1/D6): the exploration tower's emission face. Written
// test-first — red today on the shapes alone (the drive thunk extraction,
// the meta/drive call pair, the fxgate passthrough gate). The structural
// claim of D1 is pinned here the way m10b's suite pinned the inline
// driver: __we_main carries exactly the meta/drive pairs and the summary,
// while every test's step sequence rides its own re-enterable thunk with
// the instruction stream unchanged (m10b's HarnessSynthesis fragments all
// survive inside the thunks — that suite stays green through the
// refactor, the conformance goldens riding beside it).

// entryBody extracts the __we_main define's body text (the driver's own
// instructions, free of every other define's).
func entryBody(t *testing.T, ir string) string {
	t.Helper()
	marker := "define i32 @__we_main() {\nentry:\n"
	i := strings.Index(ir, marker)
	if i < 0 {
		t.Fatalf("no __we_main entry in IR:\n%s", ir)
	}
	rest := ir[i+len(marker):]
	if j := strings.Index(rest, "\n}\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// --- D1: the drive-thunk extraction and the meta/drive pair -----------------

// The driver's shape: each test's step sequence (mock installs, begin,
// spawn, await, end, restores, report branch) rides a zero-parameter
// internal thunk `@<key>.drive.<n>`; __we_main emits one meta call (file,
// description, and the declaration's line/col — the exploration guards'
// anchor) and one drive call per test, then the summary. One emission
// form for both modes — the runtime decides whether drive re-enters.
func TestM10cDriveThunkExtraction(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		pubFn("three", nil, named("Int64"), retValue(intLit("3"))),
	}}}
	tm := ProgModule{Key: "tests.m_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		&ast.TestDecl{Desc: "plain", Line: 3, Col: 1, Body: ast.Block{Items: []ast.Stmt{
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
		&ast.TestDecl{Desc: "mocked", Line: 6, Col: 1, Body: ast.Block{Items: []ast.Stmt{
			&ast.MockDecl{Target: "three", TargetQual: "util", HasRet: true, Ret: named("Int64"),
				Body: ast.Block{Items: []ast.Stmt{retValue(intLit("7"))}}},
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
	}}}
	ir, ni := EmitProgram(ModeTest, []ProgModule{util, tm})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}

	// The thunks: one per test, under the wrap family's numbering.
	wantIR(t, ir, "define internal void @tests.m_test.drive.0()", "drive thunk 0")
	wantIR(t, ir, "define internal void @tests.m_test.drive.1()", "drive thunk 1")

	// The step sequence rides inside the thunks (m10b's fragments survive).
	wantIR(t, ir, "call void @__we_test_begin()", "begin")
	wantIR(t, ir, "__we_task_new(ptr @tests.m_test.wrap.0, ptr null)", "wrapper spawn")
	wantIR(t, ir, "call i64 @__we_handle_await(", "await")
	wantIR(t, ir, "call void @__we_test_report(", "report")
	wantIR(t, ir, "store ptr @tests.m_test.mock.0, ptr @slot.util.three", "mock install")

	// The entry carries exactly the pairs and the summary — nothing of
	// any test's step sequence leaks back into it.
	main := entryBody(t, ir)
	wantSeq := []string{
		"  call void @__we_test_meta(ptr @.s0, i64 15, ptr @.s1, i64 5, i64 3, i64 1)",
		"  call void @__we_test_drive(i64 0, ptr @tests.m_test.drive.0)",
		"  call void @__we_test_meta(ptr @.s0, i64 15, ptr @.s2, i64 6, i64 6, i64 1)",
		"  call void @__we_test_drive(i64 1, ptr @tests.m_test.drive.1)",
		"  call void @__we_test_summary()",
		"  ret i32 0",
	}
	if main != strings.Join(wantSeq, "\n") {
		t.Fatalf("entry body mismatch:\nwant:\n%s\ngot:\n%s", strings.Join(wantSeq, "\n"), main)
	}

	// The runtime faces are declared (used-only, like every declare row).
	wantIR(t, ir, "declare void @__we_test_meta(ptr, i64, ptr, i64, i64, i64)", "meta declare")
	wantIR(t, ir, "declare void @__we_test_drive(i64, ptr)", "drive declare")
}

// Build mode emits no meta/drive pair (no tests to drive) — the M8/M9b
// module bytes ride unchanged through the widening.
func TestM10cBuildFaceUnchanged(t *testing.T) {
	m := progBuild(okReturn())
	ir, ni := EmitProgram(ModeBuild, []ProgModule{m})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantNoIR(t, ir, "__we_test_meta", "build mode has no meta")
	wantNoIR(t, ir, "__we_test_drive", "build mode has no drive")
	wantNoIR(t, ir, "fxgate", "build mode without custom effects has no gate")
}

// --- D6: the fxgate passthrough gate ----------------------------------------

// A custom-effect fn (a segment beyond the three built-in bare tags) gets
// a same-ABI passthrough gate as its slot default: the gate checks the
// exploration guard, then calls the real body directly (not through the
// slot — no recursion). The real define's name is untouched; a built-in
// tag keeps the slot pointing at the real body (no gate).
func TestM10cFxGateCustomEffect(t *testing.T) {
	fxDB := &ast.EffectDecl{Name: "db"}
	save := &ast.FnDecl{Name: "save", EffectTags: []string{"db"},
		Body: ast.Block{Items: []ast.Stmt{&ast.Return{}}}}
	query := &ast.FnDecl{Pub: true, Name: "query", Params: []ast.Param{i64Param("a")},
		Ret: named("Int64"), EffectTags: []string{"db"},
		Body: ast.Block{Items: []ast.Stmt{retValue(ident("a"))}}}
	plainio := &ast.FnDecl{Name: "plainio", EffectTags: []string{"io"},
		Body: ast.Block{Items: []ast.Stmt{&ast.Return{}}}}
	m := ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
		fxDB, save, query, plainio,
		appError(),
		mainDecl(
			letDiscard(&ast.Call{Fn: ident("save")}),
			letDiscard(&ast.Call{Fn: ident("query"), Args: []ast.Expr{intLit("1")}}),
			letDiscard(&ast.Call{Fn: ident("plainio")}),
			okReturn(),
		),
	}}}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{m})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}

	// Custom: slot default is the gate; the gate checks then calls the
	// real body under its own name; the real defines are untouched.
	wantIR(t, ir, "@slot.main.save = global ptr @main.save.fxgate", "void-family gate slot")
	wantIR(t, ir, "define internal void @main.save.fxgate()", "void-family gate define")
	wantIR(t, ir, "call void @__we_explore_fx_check(ptr @.s0, i64 4)", "gate check names save")
	wantIR(t, ir, "call void @main.save()", "gate calls the real body directly")

	wantIR(t, ir, "@slot.main.query = global ptr @main.query.fxgate", "i64-family gate slot")
	wantIR(t, ir, "define internal i64 @main.query.fxgate(i64 %a)", "i64-family gate ABI clone")
	wantIR(t, ir, "call i64 @main.query(i64 %a)", "value family calls and yields")

	wantIR(t, ir, "define void @main.save()", "real save define name unchanged")
	wantIR(t, ir, "define i64 @main.query(i64 %a)", "real query define name unchanged")

	// Built-in io: no gate, the slot points at the real body.
	wantNoIR(t, ir, "@main.plainio.fxgate", "built-in tag gets no gate")
	wantIR(t, ir, "@slot.main.plainio = global ptr @main.plainio", "built-in slot default")

	wantIR(t, ir, "declare void @__we_explore_fx_check(ptr, i64)", "check declare")
}

// A qualified custom tag (mod.name form) is custom too — the qualified
// shape never names a built-in (chapter 16's built-ins are bare language
// tags), so the gate applies.
func TestM10cFxGateQualifiedTag(t *testing.T) {
	store := &ast.FnDecl{Name: "put", EffectTags: []string{"store.db"},
		Body: ast.Block{Items: []ast.Stmt{&ast.Return{}}}}
	m := ProgModule{Key: "main", File: &ast.File{Items: []ast.Item{
		store,
		appError(),
		mainDecl(letDiscard(&ast.Call{Fn: ident("put")}), okReturn()),
	}}}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{m})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@slot.main.put = global ptr @main.put.fxgate", "qualified tag is custom")
}

// The mock cycle over a custom-effect fn: install unchanged (the mock fn
// fills the slot, the gate is never entered), restore back to the gate —
// never the real body (the post-mock default is the gate again).
func TestM10cFxGateMockRestore(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		&ast.FnDecl{Pub: true, Name: "three", Ret: named("Int64"), EffectTags: []string{"db"},
			Body: ast.Block{Items: []ast.Stmt{retValue(intLit("3"))}}},
	}}}
	tm := ProgModule{Key: "tests.m_test", File: &ast.File{IsTestModule: true, Items: []ast.Item{
		&ast.Import{Path: []string{"std", "test"}, Alias: "st"},
		&ast.TestDecl{Desc: "mocked", Line: 3, Col: 1, Body: ast.Block{Items: []ast.Stmt{
			&ast.MockDecl{Target: "three", TargetQual: "util", HasRet: true, Ret: named("Int64"),
				Body: ast.Block{Items: []ast.Stmt{retValue(intLit("7"))}}},
			&ast.ExprStmt{Expr: qualCall("st", "assertTrue", boolLit("true"))},
		}}},
	}}}
	ir, ni := EmitProgram(ModeTest, []ProgModule{util, tm})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "store ptr @tests.m_test.mock.0, ptr @slot.util.three", "install unchanged")
	wantIR(t, ir, "store ptr @util.three.fxgate, ptr @slot.util.three", "restore returns to the gate")
	wantNoIR(t, ir, "store ptr @util.three, ptr @slot.util.three", "the real body never rides a restore")
}
