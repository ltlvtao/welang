package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T8-1 module-level `let` bindings (design D7, chapter 15 R5). Chapter 15
// makes every module's top-level initializers run exactly once, at process
// start, before main — the import graph's post-order, source order within
// one module. The emission face is:
//
//   - a scalar binding owns one LLVM global, `@<key>.<name>`, and its
//     initializer stores straight into it (D7's direct store): the read
//     face is one load, so a fn body, a later initializer of the same
//     module and a
//     reading module all resolve a top-level name the same way, and no copy
//     or slot stands between the binding and its storage;
//   - a module with at least one binding gets `@<key>.init`, whose body is
//     its initializers in source order; a module with none gets no init and
//     no call, so the programs that carry no top-level binding emit exactly
//     the bytes they emitted before (M10b's zero-observation disclosure,
//     kept wherever it still holds);
//   - `__we_main` opens with the inits in load order — the dependency
//     modules first, the root last — which is chapter 15's post-order;
//   - a panic inside an initializer takes the task-fail tail, so the
//     process aborts: there is no capture boundary before main (R5).
//
// The face covers three storage shapes: a scalar word, a String's pair of
// words, and one global holding a collectable handle (T8-2B, whose pins are
// toplet_gc_test.go's). A binding the emitter cannot classify statically
// still stops at the boundary word — the composites' faces are the ones
// left. The emitted shape is pinned below; the answers are the conformance
// goldens'.

// topLetDecl is one module-level binding: `[pub] let name [: typ] = init`.
func topLetDecl(name string, typ ast.TypeRef, init ast.Expr) *ast.TopLet {
	return &ast.TopLet{Pub: true, Binding: ast.Binding{Kw: "let", Name: name, Typ: typ, Init: init}}
}

// topProg is the root build module: the given module-level items, then the
// AppError sum and a main whose body is the given statements closed by Ok.
func topProg(decls []ast.Item, stmts ...ast.Stmt) ProgModule {
	items := append([]ast.Item{}, decls...)
	items = append(items, appError())
	return ProgModule{Key: "main", File: &ast.File{
		Items: append(items, mainDecl(append(stmts, okReturn())...)),
	}}
}

// topFn is a reading module's fn: `pub fn name(args) -> Int64 { return body }`.
func topFn(name string, body ast.Expr) ast.Item {
	return pubFn(name, nil, named("Int64"), retValue(body))
}

// entryTail is everything from the entry define on: the init calls and the
// entry's own statements are both there, in that order.
func entryTail(ir string) string {
	i := strings.Index(ir, "define i32 @__we_main()")
	if i < 0 {
		return ""
	}
	return ir[i:]
}

// initBody is the named module init's define, from its first line to the
// entry define (the init defines land ahead of it in the fn group).
func initBody(t *testing.T, ir, key string) string {
	t.Helper()
	i := strings.Index(ir, "define void @"+key+".init()")
	if i < 0 {
		t.Fatalf("no init define for %q:\n%s", key, ir)
	}
	return ir[i:]
}

// TestTopLetScalarBindsTheGlobalDirectly: the initializer's value is stored
// straight into the module global — no slot, no copy — and that global is
// the binding's whole storage.
func TestTopLetScalarBindsTheGlobalDirectly(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		topLetDecl("base", nil, intLit("7")),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@main.base = internal global i64 0", "the binding's global")
	wantIR(t, ir, "define void @main.init()", "the module init")
	wantIR(t, ir, "store i64 7, ptr @main.base", "the direct store")
}

// TestTopLetInitRunsBeforeTheEntryBody: the call is the entry's first
// instruction, ahead of main's own statements — chapter 15 puts every
// initializer before main runs, and main's body is what runs after it.
func TestTopLetInitRunsBeforeTheEntryBody(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		topLetDecl("base", nil, intLit("7")),
	}, letBind("n", ident("base")))})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	tail := entryTail(ir)
	if !regexp.MustCompile(`entry:\n  call void @main\.init\(\)\n`).MatchString(tail) {
		t.Fatalf("the entry opens with the module init:\n%s", tail)
	}
	call := strings.Index(tail, "call void @main.init()")
	read := strings.Index(tail, "load i64, ptr @main.base")
	if read < 0 {
		t.Fatalf("main's body does not read the binding's global:\n%s", tail)
	}
	if call > read {
		t.Fatalf("the init call lands after the entry's own statements:\n%s", tail)
	}
}

// TestTopLetInitsRunInLoadOrder: the import graph's post-order — a
// dependency module initializes before the module that imports it, and the
// root last, adjacent and in that order.
func TestTopLetInitsRunInLoadOrder(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		topLetDecl("base", nil, intLit("1")),
	}}}
	root := topProg([]ast.Item{topLetDecl("top", nil, intLit("2"))})
	ir, ni := EmitProgram(ModeBuild, []ProgModule{util, root})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	if !regexp.MustCompile(`call void @util\.init\(\)\n  call void @main\.init\(\)\n`).MatchString(entryTail(ir)) {
		t.Fatalf("the dependency's init does not run first:\n%s", entryTail(ir))
	}
}

// TestTopLetDoublyImportedModuleInitializesOnce: chapter 15's doubly-imported
// module initializes once. Two modules import the same third, and both
// importers reach it through the one global its init wrote.
//
// The count is what is pinned, and the count needs a module with more than
// one binding to have anything to say: the call is emitted per binding, so a
// module's second binding is the first thing the once-only dedup has to
// catch. Both halves of the chapter's claim therefore ride this program —
// u3's two bindings make the dedup do work within a module, and the two
// importers make its key meet the walk twice.
//
// Nothing a We program can write observes the count: the top level has no
// side effect to run twice and no mutable state to increment, so a second
// init would rewrite the same globals with the same values and every answer
// would stand. The count is pinned here, at the emission layer, because that
// is the only layer that can see it.
func TestTopLetDoublyImportedModuleInitializesOnce(t *testing.T) {
	deep := ProgModule{Key: "u3", File: &ast.File{Items: []ast.Item{
		topLetDecl("base", nil, intLit("40")),
		topLetDecl("more", nil, binOp("+", ident("base"), intLit("1"))),
	}}}
	left := ProgModule{Key: "u1", File: &ast.File{Items: []ast.Item{
		topLetDecl("a", nil, memberOf(ident("u3"), "base")),
	}}}
	right := ProgModule{Key: "u2", File: &ast.File{Items: []ast.Item{
		topLetDecl("b", nil, memberOf(ident("u3"), "more")),
	}}}
	root := topProg(nil, letBind("n", memberOf(ident("u1"), "a")))
	ir, ni := EmitProgram(ModeBuild, []ProgModule{deep, left, right, root})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	if got := strings.Count(entryTail(ir), "call void @u3.init()"); got != 1 {
		t.Fatalf("the doubly-imported module initializes %d times, want 1:\n%s", got, entryTail(ir))
	}
	if got := strings.Count(ir, "define void @u3.init()"); got != 1 {
		t.Fatalf("%d init defines for u3, want 1:\n%s", got, ir)
	}
	// The same claim for a module the root alone owns: two bindings, one
	// call. This is the dedup's within-module half, with no import graph in
	// the way.
	root2 := topProg([]ast.Item{
		topLetDecl("p", nil, intLit("1")),
		topLetDecl("q", nil, binOp("+", ident("p"), intLit("1"))),
	}, letBind("n", ident("q")))
	ir2, ni2 := EmitProgram(ModeBuild, []ProgModule{root2})
	if ni2 != nil {
		t.Fatalf("boundary: %s", ni2.What)
	}
	if got := strings.Count(entryTail(ir2), "call void @main.init()"); got != 1 {
		t.Fatalf("a two-binding module initializes %d times, want 1:\n%s", got, entryTail(ir2))
	}
}

// TestTopLetShadowedQualifierIsNoModule: a local that carries a module's name
// shadows it (chapter 6), so a member read through that name is the local's,
// not the module's binding. The guard is what keeps the two apart: without it
// the qualifier would resolve to the module and the read would take the
// global's value where the program asked for the local's field.
//
// The accepted-program shape rides the run-toplet-shadowed-qualifier golden;
// this pin carries the same claim at the emission layer, where the answer is
// visible as the absence of the global's load.
func TestTopLetShadowedQualifierIsNoModule(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		topLetDecl("base", nil, intLit("3")),
	}}}
	root := topProg([]ast.Item{recDecl("Box", "value", fld("base", "Int64"))},
		letBind("util", construct("Box", init1("base", intLit("7")))),
		letBind("n", memberOf(ident("util"), "base")),
	)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{util, root})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantNoIR(t, ir, "load i64, ptr @util.base", "the shadowed name is no module")
}

// TestTopLetSourceOrderWithinAModule: within one module the initializers
// run in source order, and an initializer reads an earlier binding through
// that binding's global — the same load face a fn body uses.
func TestTopLetSourceOrderWithinAModule(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		topLetDecl("a", nil, intLit("1")),
		topLetDecl("b", nil, binOp("+", ident("a"), intLit("3"))),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	body := initBody(t, ir, "main")
	// The initializers run in source order, and the second reads the first
	// through its global: the loaded register is the arithmetic's own left
	// operand, so the value it computes from is the store above it.
	m := regexp.MustCompile(`store i64 1, ptr @main\.a\n  (%v\d+) = load i64, ptr @main\.a\n  %v\d+ = call \{ i64, i1 \} @llvm\.sadd\.with\.overflow\.i64\(i64 (%v\d+), i64 3\)\n`).FindStringSubmatch(body)
	if m == nil || m[1] != m[2] {
		t.Fatalf("the second initializer does not read the first through its global:\n%s", body)
	}
	if strings.Index(body, "ptr @main.b") < strings.Index(body, "ptr @main.a") {
		t.Fatalf("the initializers do not run in source order:\n%s", body)
	}
}

// TestTopLetCrossModuleReadLoadsTheDependencyGlobal: a fn body reads
// another module's binding through the qualified name, and one load of that
// module's global is the whole read face.
func TestTopLetCrossModuleReadLoadsTheDependencyGlobal(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		topLetDecl("base", nil, intLit("3")),
	}}}
	root := topProg(nil, letBind("n", memberOf(ident("util"), "base")))
	ir, ni := EmitProgram(ModeBuild, []ProgModule{util, root})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "load i64, ptr @util.base", "the qualified read")
}

// TestTopLetSameModuleFnReadsTheGlobal: a fn body of the owning module
// reads the bare name through the global too — the module's own bodies and
// its initializers share one read face, so nothing can drift between them.
func TestTopLetSameModuleFnReadsTheGlobal(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		topLetDecl("base", nil, intLit("4")),
		topFn("four", ident("base")),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	i := strings.Index(ir, "define i64 @main.four()")
	if i < 0 {
		t.Fatalf("no define for the reading fn:\n%s", ir)
	}
	wantIR(t, ir[i:], "load i64, ptr @main.base", "the bare name reads the global")
}

// TestTopLetReadKeepsItsDomain: a name bound from a module-level binding
// carries the binding's domain onward. An unannotated top-level let has no
// annotation to lend a reader — the global's type is the only thing that
// says what a load of it means — so a reader that reread a declaration the
// binding need not have would classify an integer as unclassified and stop
// the interpolation that names it.
func TestTopLetReadKeepsItsDomain(t *testing.T) {
	util := ProgModule{Key: "util", File: &ast.File{Items: []ast.Item{
		topLetDecl("base", nil, intLit("3")),
	}}}
	root := ProgModule{Key: "main", File: listModule(nil,
		letBind("n", memberOf(ident("util"), "base")),
		letBind("m", interpLit([]string{"n=", ""}, ident("n"))),
		ioCall("io", "println", ident("m")),
	)}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{util, root})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "= call %struct.we_str @__we_str_of_i64", "the read's domain reaches the hole")
}

// TestTopLetFloatOwnsADoubleGlobal: the Float64 domain takes a double
// global, so the binding's storage carries the value's own domain rather
// than a bit pattern read back as an integer.
func TestTopLetFloatOwnsADoubleGlobal(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		topLetDecl("f", named("Float64"), floatLit("1.5")),
	}, letBind("g", ident("f")))})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@main.f = internal global double 0.0", "the float global")
	if !regexp.MustCompile(`store double \S+, ptr @main\.f`).MatchString(ir) {
		t.Fatalf("the float initializer does not store as a double:\n%s", ir)
	}
	// The read is typed by the binding's domain too: the storage face is one
	// global of one type, and a reader that loaded it as an integer would
	// read the bit pattern rather than the value.
	if !regexp.MustCompile(`%v\d+ = load double, ptr @main\.f`).MatchString(ir) {
		t.Fatalf("the float read is not a double load:\n%s", ir)
	}
	wantNoIR(t, ir, "load i64, ptr @main.f", "the float global is not read as an integer")
}

// TestTopLetDiscardBindsNoGlobal: `let _ = 7` runs its initializer — the
// discard is chapter 6's, and the module still owes the evaluation — so the
// init exists while no symbol names the value.
func TestTopLetDiscardBindsNoGlobal(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		topLetDecl("_", nil, intLit("7")),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define void @main.init()", "the module still owes the evaluation")
	wantNoIR(t, ir, "@main._", "a discard binds nothing")
}

// TestTopLetListStops is retired: it pinned the list binding's STOP, and
// T8-2B implemented that face — a List binding now owns a handle global and
// registers it (TestTopLetListOwnsAHandleGlobalAndRegisters). The name is
// gone rather than re-anchored because it names a fact that no longer
// holds, exactly as the String face's stop was retired in T8-2A.

// TestTopLetInitTrapAborts: chapter 15 R5 — an initializer that fails
// aborts the process. The trap rides the task-fail tail inside the init
// define itself, so no capture boundary stands between the failure and the
// abort.
func TestTopLetInitTrapAborts(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{topProg([]ast.Item{
		topLetDecl("q", nil, binOp("/", intLit("5"), intLit("0"))),
	})})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "call void @__we_task_fail(ptr @.dvz0)", "the initializer's trap")
	wantIR(t, initBody(t, ir, "main"), "define void @main.init()", "inside the init define")
}

// TestTopLetAbsentEmitsNoInit: the M10b disclosure's zero-observation
// property, kept for the programs that still have it — a module with no
// top-level binding adds no define and no call.
func TestTopLetAbsentEmitsNoInit(t *testing.T) {
	ir, ni := EmitProgram(ModeBuild, []ProgModule{progBuild(okReturn())})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantNoIR(t, ir, "define void @main.init", "no binding, no init define")
	wantNoIR(t, ir, "call void @main.init", "no binding, no init call")
}
