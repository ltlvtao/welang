package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T5 method table (design D4). Before the task an impl block was erased
// whole (the declaration walk's explicit `case *ast.InterfaceDecl,
// *ast.ImplDecl: continue`), so no method had a symbol and every call
// `recv.name(...)` stopped at the body word. The table is static: a pair
// (head type, method name) resolves to one define, the receiver rides the
// first parameter, and the default body an interface declares is
// instantiated per implementing head — which is what makes `self.greet()`
// inside a default body resolve to the head's own method rather than to
// the interface's.

// implDecl builds `impl [Iface for] Head { methods }`.
func implDecl(iface, head string, methods ...*ast.FnDecl) *ast.ImplDecl {
	d := &ast.ImplDecl{Head: named(head), Methods: methods}
	if iface != "" {
		d.Iface = named(iface)
	}
	return d
}

// method builds one method definition.
func method(name string, recv ast.RecvKind, ret string, body ...ast.Stmt) *ast.FnDecl {
	d := &ast.FnDecl{Name: name, Recv: recv, Body: ast.Block{Items: body}}
	if ret != "" {
		d.Ret = named(ret)
	}
	return d
}

// ifaceDecl builds `interface Name { sigs }`.
func ifaceDecl(name string, sigs ...ast.MethodSig) *ast.InterfaceDecl {
	return &ast.InterfaceDecl{Name: name, Methods: sigs}
}

// sig builds one interface signature, body-less or with a default body.
func sig(name string, recv ast.RecvKind, ret string, body ...ast.Stmt) ast.MethodSig {
	s := ast.MethodSig{Name: name, Recv: recv, Ret: named(ret), HasRet: true}
	if body != nil {
		s.Body = &ast.Block{Items: body}
	}
	return s
}

// selfField builds `self.name`.
func selfField(name string) *ast.Member {
	return memberOf(ident("self"), name)
}

// selfWrite builds `self.name = value`.
func selfWrite(name string, value ast.Expr) *ast.Assign {
	return &ast.Assign{Name: "self", Field: name, Value: value}
}

// TestInherentMethodDispatches: an inherent method gets a define whose
// symbol is (module, head type, method name) and whose first parameter is
// the receiver; the call site passes the receiver pointer.
func TestInherentMethodDispatches(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			recDecl("Counter", "gc", fld("n", "Int64")),
			implDecl("", "Counter", method("get", ast.RecvSelf, "Int64",
				&ast.Return{HasValue: true, Value: selfField("n")})),
		},
		&ast.Binding{Kw: "let", Name: "c", Init: construct("Counter", init1("n", intLit("7")))},
		&ast.Binding{Kw: "let", Name: "v", Init: callOn(ident("c"), "get")},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("v"))),
		okReturn(),
	), "define i64 @main.Counter.get(ptr %self)")
	if !strings.Contains(ir, "call i64 @main.Counter.get(ptr %") {
		t.Fatalf("the call passes the receiver pointer:\n%s", ir)
	}
}

// TestMutSelfFieldWrite: `self.n = self.n + 1` stores through the receiver
// pointer at the field's layout offset — the write half of the unified
// member face (design D1's `self.field=`, design D4's mut self).
func TestMutSelfFieldWrite(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			recDecl("Counter", "gc", fld("n", "Int64")),
			implDecl("", "Counter", method("bump", ast.RecvMutSelf, "Int64",
				selfWrite("n", binOp("+", selfField("n"), intLit("1"))),
				&ast.Return{HasValue: true, Value: selfField("n")})),
		},
		&ast.Binding{Kw: "let", Name: "c", Init: construct("Counter", init1("n", intLit("0")))},
		&ast.Binding{Kw: "let", Name: "v", Init: callOn(ident("c"), "bump")},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("v"))),
		okReturn(),
	), "define i64 @main.Counter.bump(ptr %self)")
	if !strings.Contains(ir, "getelementptr i8, ptr %self, i64 16") {
		t.Fatalf("the write addresses the field at its offset:\n%s", ir)
	}
	if !strings.Contains(ir, "store i64 %") {
		t.Fatalf("the write stores the new value:\n%s", ir)
	}
}

// TestMutSelfCounterIsInPlace: two calls on one gc record share the
// object — the receiver is the pointer, never a copy, so the counter形
// counts (chapter 8's gc category: the binding shares).
func TestMutSelfCounterIsInPlace(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			recDecl("Counter", "gc", fld("n", "Int64")),
			implDecl("", "Counter", method("bump", ast.RecvMutSelf, "Int64",
				selfWrite("n", binOp("+", selfField("n"), intLit("1"))),
				&ast.Return{HasValue: true, Value: selfField("n")})),
		},
		&ast.Binding{Kw: "let", Name: "c", Init: construct("Counter", init1("n", intLit("0")))},
		&ast.Binding{Kw: "let", Name: "a", Init: callOn(ident("c"), "bump")},
		&ast.Binding{Kw: "let", Name: "b", Init: callOn(ident("c"), "bump")},
		ioCall("io", "println", interpLit([]string{"", " ", ""}, ident("a"), ident("b"))),
		okReturn(),
	), "")
	if got := allocCount(ir); got != 1 {
		t.Fatalf("the receiver is shared, not copied: want 1 allocation, got %d:\n%s", got, ir)
	}
}

// TestFnImplicitTailReturns: a fn body's final expression item is its
// implicit return value (chapter 6: "the body's block value is the
// function's implicit return value: a final expression item returns it").
// The method face surfaced it — every chapter 10 golden writes its method
// bodies that way — but the rule is the fn body's, not the method's.
func TestFnImplicitTailReturns(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			ifaceDecl("Describable", sig("describe", ast.RecvSelf, "String")),
			recDecl("User", "gc", fld("name", "String")),
			implDecl("Describable", "User", method("describe", ast.RecvSelf, "String",
				&ast.ExprStmt{Expr: strLit(`"u"`)})),
		},
		&ast.Binding{Kw: "let", Name: "u", Init: construct("User", init1("name", strLit(`"a"`)))},
		&ast.Binding{Kw: "let", Name: "text", Init: callOn(ident("u"), "describe")},
		ioCall("io", "println", ident("text")),
		okReturn(),
	), "define { ptr, i64 } @main.User.describe(ptr %self)")
	if !strings.Contains(ir, "ret { ptr, i64 }") {
		t.Fatalf("the final expression is the returned value:\n%s", ir)
	}
}

// TestMethodCallClassifiesAsString: a method call's result carries the
// method's return family, so `self.greet() + "!"` picks the concatenation
// face — the classification the whole String domain keys on has to know
// the table too, not just the fn table and the String members.
func TestMethodCallClassifiesAsString(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			ifaceDecl("Greeter",
				sig("greet", ast.RecvSelf, "String"),
				sig("greetLoud", ast.RecvSelf, "String",
					&ast.Return{HasValue: true, Value: binOp("+", callOn(ident("self"), "greet"), strLit(`"!"`))})),
			recDecl("User", "gc", fld("name", "String")),
			implDecl("Greeter", "User", method("greet", ast.RecvSelf, "String",
				&ast.Return{HasValue: true, Value: strLit(`"hi"`)})),
		},
		&ast.Binding{Kw: "let", Name: "u", Init: construct("User", init1("name", strLit(`"a"`)))},
		&ast.Binding{Kw: "let", Name: "text", Init: callOn(ident("u"), "greetLoud")},
		ioCall("io", "println", ident("text")),
		okReturn(),
	), "call %struct.we_str @__we_str_concat(ptr %v")
	if !strings.Contains(ir, "call { ptr, i64 } @main.User.greet(ptr %self)") {
		t.Fatalf("the left operand is the dispatched method call:\n%s", ir)
	}
}

// TestInterfaceImplDispatches: an `impl Iface for Head` method joins the
// same table under the head type — the call resolves by the receiver's
// record, the interface never appears in the IR (design D11's erasure for
// the declaration face, a real symbol for the body).
func TestInterfaceImplDispatches(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			ifaceDecl("Describable", sig("describe", ast.RecvSelf, "String")),
			recDecl("User", "gc", fld("name", "String")),
			implDecl("Describable", "User", method("describe", ast.RecvSelf, "String",
				&ast.Return{HasValue: true, Value: strLit(`"u"`)})),
		},
		&ast.Binding{Kw: "let", Name: "u", Init: construct("User", init1("name", strLit(`"a"`)))},
		&ast.Binding{Kw: "let", Name: "text", Init: callOn(ident("u"), "describe")},
		ioCall("io", "println", ident("text")),
		okReturn(),
	), "define { ptr, i64 } @main.User.describe(ptr %self)")
	if !strings.Contains(ir, "call { ptr, i64 } @main.User.describe(ptr %") {
		t.Fatalf("the call passes the receiver pointer:\n%s", ir)
	}
}

// TestDefaultMethodInstantiatesPerHead: an interface's default body
// becomes one define per implementing head, and the `self.greet()` inside
// it resolves to that head's own method — the reason the default is
// instantiated rather than emitted once against the interface.
func TestDefaultMethodInstantiatesPerHead(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			ifaceDecl("Greeter",
				sig("greet", ast.RecvSelf, "String"),
				sig("greetLoud", ast.RecvSelf, "String",
					&ast.Return{HasValue: true, Value: callOn(ident("self"), "greet")})),
			recDecl("User", "gc", fld("name", "String")),
			implDecl("Greeter", "User", method("greet", ast.RecvSelf, "String",
				&ast.Return{HasValue: true, Value: strLit(`"hi"`)})),
		},
		&ast.Binding{Kw: "let", Name: "u", Init: construct("User", init1("name", strLit(`"a"`)))},
		&ast.Binding{Kw: "let", Name: "text", Init: callOn(ident("u"), "greetLoud")},
		ioCall("io", "println", ident("text")),
		okReturn(),
	), "define { ptr, i64 } @main.User.greetLoud(ptr %self)")
	if !strings.Contains(ir, "call { ptr, i64 } @main.User.greet(ptr %self)") {
		t.Fatalf("the default body dispatches to the head's own method:\n%s", ir)
	}
	if strings.Contains(ir, "@main.Greeter.") {
		t.Fatalf("the interface owns no symbol — its default instantiates per head:\n%s", ir)
	}
}
