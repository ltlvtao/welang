package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T5-4 scope resource (chapter 13). The statement binds one or more
// resources to a block; every exit of that block runs exactly one release
// per binding, in reverse declaration order, and an exit that pierces the
// block — a return, a break, a continue — is a block exit like any other
// (chapter 13: "被 return、break 或 continue 穿透的出口是块出口，同样释放").
// The release is a method call through the T5 table: the head type's own
// `release`, which is what `impl Releasable for Head` contributes. The
// head expression's record pointer is the receiver, so a resource binding
// is a record binding with a release obligation attached.

// resDecl builds `byres record Name { fields }`.
func resDecl(name string, fields ...ast.FieldDecl) *ast.RecordDecl {
	return recDecl(name, "resource", fields...)
}

// releaseImpl builds the `impl Releasable for Head` every resource record
// owes (chapter 13's declaration completeness, E1101).
func releaseImpl(head string, body ...ast.Stmt) *ast.ImplDecl {
	return implDecl("Releasable", head, method("release", ast.RecvMutSelf, "", body...))
}

// resBind builds one `name = expr` head binding.
func resBind(name string, val ast.Expr) ast.ScopeBind {
	return ast.ScopeBind{Name: name, Val: val}
}

// scopeRes builds `scope resource(binds) block`.
func scopeRes(binds []ast.ScopeBind, stmts ...ast.Stmt) *ast.ScopeRes {
	return &ast.ScopeRes{Binds: binds, Body: ast.Block{Items: stmts}}
}

// defineText slices one define out of the module text by its symbol — the
// body a call-site assertion has to read in place.
func defineText(ir, sym string) string {
	at := strings.Index(ir, "@"+sym+"(")
	if at < 0 {
		return ""
	}
	start := strings.LastIndex(ir[:at], "define ")
	end := strings.Index(ir[at:], "\n}")
	if start < 0 || end < 0 {
		return ""
	}
	return ir[start : at+end]
}

// allocRegs lists the registers record allocations land in, in emission
// order — a head expression's record is its allocation.
func allocRegs(ir string) []string {
	re := regexp.MustCompile(`(%\w+) = call ptr @__we_alloc\(`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(ir, -1) {
		out = append(out, m[1])
	}
	return out
}

// releaseOps lists the receiver registers of the release calls, in
// emission order — the order their instructions stand in the IR.
func releaseOps(ir string) []string {
	re := regexp.MustCompile(`call void @main\.Handle\.release\(ptr (%\w+)\)`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(ir, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestScopeResourceReleasesInReverseOrder: the block's normal exit runs
// one release per binding, in reverse declaration order (chapter 13: "块
// 出口处编译器保证每绑定恰一次 release 调用，按声明逆序"). Each head
// expression allocates its record, so the allocation order names the
// bindings and the release order has to be its mirror.
func TestScopeResourceReleasesInReverseOrder(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			resDecl("Handle", fld("fd", "Int64")),
			releaseImpl("Handle", &ast.Return{}),
		},
		scopeRes([]ast.ScopeBind{
			resBind("a", construct("Handle", init1("fd", intLit("1")))),
			resBind("b", construct("Handle", init1("fd", intLit("2")))),
		},
			// The binding is the block's (chapter 13: "每个名字为该块绑定"):
			// reading a field through it is the ordinary record read, and the
			// name is out of reach past the block.
			&ast.Binding{Kw: "let", Name: "n", Init: memberOf(ident("b"), "fd")},
			ioCall("io", "println", interpLit([]string{"", ""}, ident("n")))),
		okReturn(),
	), "call void @main.Handle.release(ptr %")
	regs := allocRegs(ir)
	ops := releaseOps(ir)
	if len(regs) != 2 || len(ops) != 2 {
		t.Fatalf("want two allocations and two releases, got %d and %d:\n%s", len(regs), len(ops), ir)
	}
	if ops[0] != regs[1] || ops[1] != regs[0] {
		t.Fatalf("b (allocated second) releases first: releases %v, allocs %v:\n%s", ops, regs, ir)
	}
}

// TestScopeResourceEarlyReturnReleases: a return inside the block is a
// block exit — the releases run at the return site, ahead of the ret, in
// reverse declaration order, and the returned operand computes first (it
// may read a binding the exit is about to release). The block's own
// falling-out exit is a different runtime path and owes its own pair:
// each exit releases once, and the statement's end releases only where
// the exit it falls out of is reachable. A fn whose block leaves the
// statement as its last item has no value to return (the statement
// produces none — E0501), so the trailing return is the body's tail.
func TestScopeResourceEarlyReturnReleases(t *testing.T) {
	use := &ast.FnDecl{
		Name: "use", Ret: named("Int64"),
		Body: ast.Block{Items: []ast.Stmt{
			scopeRes([]ast.ScopeBind{
				resBind("a", construct("Handle", init1("fd", intLit("1")))),
				resBind("b", construct("Handle", init1("fd", intLit("2")))),
			}, &ast.Return{HasValue: true, Value: memberOf(ident("a"), "fd")}),
			&ast.Return{HasValue: true, Value: intLit("0")},
		}},
	}
	ir := assertClean(t, recModule(
		[]ast.Item{resDecl("Handle", fld("fd", "Int64")), releaseImpl("Handle", &ast.Return{}), use},
		&ast.Binding{Kw: "let", Name: "n", Init: &ast.Call{Fn: ident("use")}},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define i64 @main.use()")
	body := defineText(ir, "main.use")
	if body == "" {
		t.Fatalf("the fn has its own define:\n%s", ir)
	}
	regs := allocRegs(body)
	ops := releaseOps(body)
	// The block's only statement is the return, so the exit that would
	// fall out of the block is unreachable: this path is the only one the
	// statement owes, and it releases each binding exactly once.
	if len(regs) != 2 || len(ops) != 2 {
		t.Fatalf("the return path releases both bindings, once each: allocs %v, releases %v:\n%s", regs, ops, body)
	}
	if ops[0] != regs[1] || ops[1] != regs[0] {
		t.Fatalf("the return path releases in reverse declaration order: releases %v, allocs %v:\n%s", ops, regs, body)
	}
	if ret := strings.Index(body, "\n  ret i64 "); ret < 0 || strings.Index(body, "call void @main.Handle.release") > ret {
		t.Fatalf("the releases run at the return site, ahead of the ret:\n%s", body)
	}
}

// TestScopeResourceReleasesOnEveryExitPath: a return inside the block
// discharges at its own site and the exit that falls out of the block
// discharges at the block's end — two runtime paths, each releasing
// every binding exactly once, in reverse declaration order. The shape is
// chapter 13's golden (a conditional return guarded inside the block
// with the fn's own return after it), and it is why the release
// sequence cannot be hoisted or shared: the two paths branch apart.
func TestScopeResourceReleasesOnEveryExitPath(t *testing.T) {
	use := &ast.FnDecl{
		Name: "use", Ret: named("Int64"),
		Body: ast.Block{Items: []ast.Stmt{
			scopeRes([]ast.ScopeBind{
				resBind("a", construct("Handle", init1("fd", intLit("1")))),
				resBind("b", construct("Handle", init1("fd", intLit("2")))),
			}, &ast.ExprStmt{Expr: &ast.If{
				Cond: binOp(">", memberOf(ident("a"), "fd"), intLit("0")),
				Then: ast.Block{Items: []ast.Stmt{
					&ast.Return{HasValue: true, Value: memberOf(ident("a"), "fd")},
				}},
			}}),
			&ast.Return{HasValue: true, Value: intLit("0")},
		}},
	}
	ir := assertClean(t, recModule(
		[]ast.Item{resDecl("Handle", fld("fd", "Int64")), releaseImpl("Handle", &ast.Return{}), use},
		&ast.Binding{Kw: "let", Name: "n", Init: &ast.Call{Fn: ident("use")}},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define i64 @main.use()")
	body := defineText(ir, "main.use")
	regs := allocRegs(body)
	ops := releaseOps(body)
	if len(regs) != 2 || len(ops) != 4 {
		t.Fatalf("two exits, one release per binding at each: allocs %v, releases %v:\n%s", regs, ops, body)
	}
	for path := 0; path < 2; path++ {
		if ops[2*path] != regs[1] || ops[2*path+1] != regs[0] {
			t.Fatalf("each exit releases in reverse declaration order: releases %v, allocs %v:\n%s", ops, regs, body)
		}
	}
	ret := strings.Index(body, "\n  ret i64 ")
	if rel := strings.Index(body, "call void @main.Handle.release"); ret < 0 || rel < 0 || rel > ret {
		t.Fatalf("the return path's releases run ahead of the ret:\n%s", body)
	}
}

// TestScopeResourceReleaseDispatchesToTheImplBody: the release named at
// the block exit is the head type's own method — the impl body the table
// holds under the head, with the resource pointer as its receiver. The
// interface contributes no symbol of its own (chapter 10's erasure, the
// same face `impl Releasable for Head` rides everywhere else).
func TestScopeResourceReleaseDispatchesToTheImplBody(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			resDecl("Handle", fld("fd", "Int64")),
			releaseImpl("Handle", &ast.Return{}),
		},
		scopeRes([]ast.ScopeBind{resBind("a", construct("Handle", init1("fd", intLit("1"))))}),
		okReturn(),
	), "define void @main.Handle.release(ptr %self)")
	if strings.Contains(ir, "@main.Releasable.") {
		t.Fatalf("the interface owns no symbol — the impl body is the head's:\n%s", ir)
	}
	if body := defineText(ir, "main.Handle.release"); !strings.Contains(body, "ret void") {
		t.Fatalf("the release body emits its own define:\n%s", ir)
	}
}

// TestScopeResourceReleasesBeforeFunctionDefers: chapter 13's ordering —
// "块出口释放先于外围函数自己的 defer 运行，内块先出". A return inside
// the block is both exits at once: the releases run first, the fn's
// deferred blocks after them.
func TestScopeResourceReleasesBeforeFunctionDefers(t *testing.T) {
	use := &ast.FnDecl{
		Name: "use", Ret: named("Int64"),
		Body: ast.Block{Items: []ast.Stmt{
			// The deferred block formats a number: its call is the marker
			// the ordering assertion reads (a plain literal would leave the
			// defer's io call indistinguishable from any other).
			&ast.Defer{Block: ast.Block{Items: []ast.Stmt{
				ioCall("io", "println", interpLit([]string{"", ""}, intLit("7"))),
			}}},
			scopeRes([]ast.ScopeBind{resBind("a", construct("Handle", init1("fd", intLit("1"))))},
				&ast.Return{HasValue: true, Value: intLit("1")}),
			&ast.Return{HasValue: true, Value: intLit("0")},
		}},
	}
	ir := assertClean(t, recModule(
		[]ast.Item{resDecl("Handle", fld("fd", "Int64")), releaseImpl("Handle", &ast.Return{}), use},
		&ast.Binding{Kw: "let", Name: "n", Init: &ast.Call{Fn: ident("use")}},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define i64 @main.use()")
	body := defineText(ir, "main.use")
	rel := strings.Index(body, "call void @main.Handle.release")
	dfr := strings.Index(body, "@__we_str_of")
	if rel < 0 || dfr < 0 {
		t.Fatalf("the return path runs both the release and the defer:\n%s", body)
	}
	if rel > dfr {
		t.Fatalf("the block exit releases before the fn's defer runs:\n%s", body)
	}
}

// TestScopeResourceBreakReleases: a break piercing the block is a block
// exit too — the release runs at the break's own site, ahead of the
// branch to the loop's exit label. The loop frame names the nesting the
// break crosses, so the resource block it leaves is exactly the one that
// releases.
func TestScopeResourceBreakReleases(t *testing.T) {
	use := &ast.FnDecl{
		Name: "use", Ret: named("Int64"),
		Body: ast.Block{Items: []ast.Stmt{
			&ast.Loop{Body: ast.Block{Items: []ast.Stmt{
				scopeRes([]ast.ScopeBind{resBind("a", construct("Handle", init1("fd", intLit("1"))))},
					&ast.Break{}),
			}}},
			&ast.Return{HasValue: true, Value: intLit("0")},
		}},
	}
	ir := assertClean(t, recModule(
		[]ast.Item{resDecl("Handle", fld("fd", "Int64")), releaseImpl("Handle", &ast.Return{}), use},
		&ast.Binding{Kw: "let", Name: "n", Init: &ast.Call{Fn: ident("use")}},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define i64 @main.use()")
	body := defineText(ir, "main.use")
	rel := strings.Index(body, "call void @main.Handle.release")
	br := strings.Index(body, "br label %lpexit")
	if rel < 0 || br < 0 {
		t.Fatalf("the break path releases through the block:\n%s", body)
	}
	if rel > br {
		t.Fatalf("the release runs at the break's own site, ahead of the loop's branch:\n%s", body)
	}
}

// TestScopeResourceOpaqueHeadReleasesThroughTheNativeClose: the resource
// face of chapter 19's opaque types. A foreign block may declare a `byres
// record` whose value is a native handle — it enters We code only by
// crossing, a foreign call's declared return — and chapter 13's
// obligations hold unchanged: the impl's release body is ordinary checked
// code, and calling a foreign `close` is how the release is implemented.
// So the scope head's expression is a foreign call, its value an opaque
// pointer rather than an allocated record, its head key the opaque's, and
// the release it dispatches to is the same method-table entry any other
// resource's release is.
func TestScopeResourceOpaqueHeadReleasesThroughTheNativeClose(t *testing.T) {
	fb := &ast.ForeignBlock{ABI: "c", Line: 1, Col: 1, Items: []ast.Item{
		recDecl("CFile", ""),
		foreignFn("fopen", []ast.Param{
			{Name: "path", Type: named("String")},
			{Name: "mode", Type: named("String")},
		}, named("CFile")),
		foreignFn("fclose", []ast.Param{{Name: "f", Type: named("CFile")}}, nil),
	}}
	release := implDecl("Releasable", "CFile", method("release", ast.RecvMutSelf, "",
		&ast.ExprStmt{Expr: &ast.Call{Fn: ident("fclose"), Args: []ast.Expr{ident("self")}}}))
	head := &ast.Call{Fn: ident("fopen"), Args: []ast.Expr{strLit(`"a"`), strLit(`"r"`)}}
	bind := []ast.ScopeBind{resBind("f", head)}
	mod := ProgModule{Key: "main", ID: "demo", File: &ast.File{Items: []ast.Item{
		fb, appError(), release,
		mainDecl(scopeRes(bind), okReturn()),
	}}}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{mod})
	if ni != nil {
		t.Fatalf("unexpected boundary: %s", ni.What)
	}
	// The head's call is the value — no object is allocated for an opaque.
	if strings.Contains(ir, "__we_alloc") {
		t.Fatalf("an opaque handle is the native side's object, not ours:\n%s", ir)
	}
	order(t, ir, "call ptr @fopen(", "call void @main.CFile.release(")
	// The release body passes the handle on to the native close, and the
	// release is called with the pointer the head's call produced.
	if !strings.Contains(ir, "call void @fclose(ptr %self)") {
		t.Fatalf("the impl body calls the native close with its own handle:\n%s", ir)
	}
	if !regexp.MustCompile(`call void @main\.CFile\.release\(ptr (%v\d+)\)`).MatchString(ir) {
		t.Fatalf("the block exit calls the release the table holds:\n%s", ir)
	}
}

// TestScopeResourceNestedInScopeExprDischargesInnermostFirst: the two
// stacks hold the same nesting, and one exit crosses both. A return
// inside a resource block inside a loop inside a `scope` expression
// leaves three frames at once, and chapter 13's "内块先出" orders them by
// how they actually nest: the resource releases first, the scope object
// cancels and leaves after. Depth is the one number that compares across
// the two stacks — a scope object's index and a resource block's index
// are not the same scale — which is what the body's shared nesting
// ordinal gives them; mutating the discharge to the other sort order
// leaves this shape leaving the scope before the resource inside it is
// released.
func TestScopeResourceNestedInScopeExprDischargesInnermostFirst(t *testing.T) {
	scope := &ast.ScopeExpr{Body: ast.Block{Items: []ast.Stmt{
		&ast.Loop{Body: ast.Block{Items: []ast.Stmt{
			scopeRes([]ast.ScopeBind{resBind("a", construct("Handle", init1("fd", intLit("1"))))},
				retValue(intLit("7"))),
		}}},
	}}}
	fn := pubFn("pick", nil, named("Int64"),
		&ast.ExprStmt{Expr: scope},
		retValue(intLit("0")),
	)
	ir := assertClean(t, recModule(
		[]ast.Item{resDecl("Handle", fld("fd", "Int64")), releaseImpl("Handle", &ast.Return{}), fn},
		&ast.Binding{Kw: "let", Name: "n", Init: &ast.Call{Fn: ident("pick")}},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define i64 @main.pick()")
	body := defineText(ir, "main.pick")
	if body == "" {
		t.Fatalf("the fn has its own define:\n%s", ir)
	}
	order(t, body,
		"call void @main.Handle.release(",
		"call void @__we_scope_cancel(",
		"call i64 @__we_scope_leave(",
		"ret i64 7",
	)
	if got := strings.Count(body, "call void @main.Handle.release("); got != 1 {
		t.Fatalf("the return path releases its one binding once, got %d:\n%s", got, body)
	}
	// The scope's own fall-out is a second runtime path and leaves once
	// there — the return path's leave is the one ahead of its ret.
	if rel, lv, ret := strings.Index(body, "call void @main.Handle.release("),
		strings.Index(body, "call i64 @__we_scope_leave("),
		strings.Index(body, "ret i64 7"); rel < 0 || lv < 0 || ret < 0 || lv > ret {
		t.Fatalf("the return path leaves the scope ahead of its ret:\n%s", body)
	}
}
