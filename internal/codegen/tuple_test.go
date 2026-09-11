package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T5 newtype and tuple value faces (design D4). A newtype erases to its
// underlying type at every position — construction, `.value`, parameter,
// return — so nothing in the IR says "newtype"; a tuple is a value
// aggregate, materialized in a stack slot, destructured element-wise, and
// expanded field-per-word wherever it crosses a call boundary.

// newtypeDecl builds `newtype Name(Underlying)`.
func newtypeDecl(name, underlying string) *ast.NewtypeDecl {
	return &ast.NewtypeDecl{Name: name, Underlying: named(underlying)}
}

// tupType builds a tuple type `(T1, ..., Tn)`.
func tupType(names ...string) *ast.TupleType {
	t := &ast.TupleType{}
	for _, n := range names {
		t.Elems = append(t.Elems, named(n))
	}
	return t
}

// tup builds a tuple expression `(e1, ..., en)`.
func tup(elems ...ast.Expr) *ast.Tuple { return &ast.Tuple{Elems: elems} }

// patTuple builds a tuple pattern `(p1, ..., pn)` of plain name bindings.
func patTuple(names ...string) *ast.PatTuple {
	p := &ast.PatTuple{}
	for _, n := range names {
		p.Elems = append(p.Elems, &ast.PatBinding{Name: n})
	}
	return p
}

// TestNewtypeConstructionIsIdentity: `UserId(42)` is the value 42 — no
// allocation, no call, no wrapper (chapter 8's zero-cost promise: the
// layout erases).
func TestNewtypeConstructionIsIdentity(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{newtypeDecl("UserId", "Int64")},
		&ast.Binding{Kw: "let", Name: "id", Init: &ast.Call{Fn: ident("UserId"), Args: []ast.Expr{intLit("42")}}},
		&ast.Binding{Kw: "let", Name: "n", Init: memberOf(ident("id"), "value")},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "@__we_str_of_i64(i64 42)")
	if strings.Contains(ir, "__we_alloc") {
		t.Fatalf("the newtype erases — no object is allocated:\n%s", ir)
	}
}

// TestNewtypeParameterIsItsUnderlying: a newtype parameter crosses the fn
// boundary as the underlying type, and `.value` inside the body is the
// identity — the callee's define says nothing about UserId.
func TestNewtypeParameterIsItsUnderlying(t *testing.T) {
	raw := &ast.FnDecl{
		Name:   "raw",
		Params: []ast.Param{{Name: "id", Type: named("UserId")}},
		Ret:    named("Int64"),
		Body: ast.Block{Items: []ast.Stmt{
			&ast.Return{HasValue: true, Value: memberOf(ident("id"), "value")},
		}},
	}
	ir := assertClean(t, recModule(
		[]ast.Item{newtypeDecl("UserId", "Int64"), raw},
		&ast.Binding{Kw: "let", Name: "n",
			Init: &ast.Call{Fn: ident("raw"), Args: []ast.Expr{
				&ast.Call{Fn: ident("UserId"), Args: []ast.Expr{intLit("7")}}}}},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define i64 @main.raw(i64 %id)")
	if !strings.Contains(ir, "call i64 ") {
		t.Fatalf("the call passes the underlying value:\n%s", ir)
	}
}

// TestNewtypeOverStringIsIdentity: the erasure holds whatever the
// underlying family is — a String newtype carries the same two words.
func TestNewtypeOverStringIsIdentity(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{newtypeDecl("Name", "String")},
		&ast.Binding{Kw: "let", Name: "n",
			Init: &ast.Call{Fn: ident("Name"), Args: []ast.Expr{strLit(`"ab"`)}}},
		&ast.Binding{Kw: "let", Name: "s", Init: memberOf(ident("n"), "value")},
		ioCall("io", "println", ident("s")),
		okReturn(),
	), "")
	if strings.Contains(ir, "__we_alloc") {
		t.Fatalf("the newtype erases — no object is allocated:\n%s", ir)
	}
	if !strings.Contains(ir, `c"ab"`) {
		t.Fatalf("the underlying string is the value:\n%s", ir)
	}
}

// TestTupleConstructionAggregates: a tuple value lives in a stack
// aggregate of its elements' IR words, each stored at its offset. The
// elements are read back through the pattern — chapter 8's tuple
// pattern and destructuring is the language's element reader (a member
// name is an identifier; `p.0` is E0105).
func TestTupleConstructionAggregates(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{},
		&ast.Binding{Kw: "let", Name: "p", Init: tup(intLit("1"), intLit("2"))},
		&ast.Binding{Kw: "let", Pat: patTuple("a", "b"), Init: ident("p")},
		ioCall("io", "println", interpLit([]string{"", ""}, binOp("+", ident("a"), ident("b")))),
		okReturn(),
	), "alloca { i64, i64 }")
	if !strings.Contains(ir, "store i64 1, ptr %") || !strings.Contains(ir, "store i64 2, ptr %") {
		t.Fatalf("both elements are stored into the aggregate:\n%s", ir)
	}
	// Each element takes its own slot: the second store at the first's
	// address would leave the tuple holding one value twice (found by
	// mutating the offset step — the stores' presence alone missed it).
	if gepsOf(ir, 0) == 0 || gepsOf(ir, 8) == 0 {
		t.Fatalf("the elements take offsets 0 and 8:\n%s", ir)
	}
}

// TestTupleDestructuringLoadsElements: `let (a, b) = pair()` binds each
// name from the corresponding element — the pattern is a load per slot,
// never a call.
func TestTupleDestructuringLoadsElements(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{&ast.FnDecl{
			Name: "pair", Ret: tupType("Int64", "Int64"),
			Body: ast.Block{Items: []ast.Stmt{
				&ast.Return{HasValue: true, Value: tup(intLit("1"), intLit("2"))},
			}},
		}},
		&ast.Binding{Kw: "let", Pat: patTuple("a", "b"),
			Init: &ast.Call{Fn: ident("pair")}},
		ioCall("io", "println", interpLit([]string{"", ""}, binOp("+", ident("a"), ident("b")))),
		okReturn(),
	), "define { i64, i64 } @main.pair()")
	if got := strings.Count(ir, "= load i64, ptr %"); got < 2 {
		t.Fatalf("each element loads from its slot: want 2 loads, got %d:\n%s", got, ir)
	}
}

// TestTupleReturnIsAValueAggregate: a tuple return is the multi-value
// return — the same literal-struct face the String pair already rides —
// and the caller's binding holds the aggregate it lands back in.
func TestTupleReturnIsAValueAggregate(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{&ast.FnDecl{
			Name: "pair", Ret: tupType("Int64", "Int64"),
			Body: ast.Block{Items: []ast.Stmt{
				&ast.Return{HasValue: true, Value: tup(intLit("1"), intLit("2"))},
			}},
		}},
		&ast.Binding{Kw: "let", Name: "p", Init: &ast.Call{Fn: ident("pair")}},
		&ast.Binding{Kw: "let", Pat: patTuple("a", "b"), Init: ident("p")},
		ioCall("io", "println", interpLit([]string{"", ""}, binOp("+", ident("a"), ident("b")))),
		okReturn(),
	), "ret { i64, i64 }")
	// The call's callee is the program fn's loaded slot, not the symbol
	// (design D3's indirection — the one face every program fn call
	// takes); what the return's family settles is the call's own type.
	if !regexp.MustCompile(`= call \{ i64, i64 \} %v\d+\(\)`).MatchString(ir) {
		t.Fatalf("the call takes the aggregate:\n%s", ir)
	}
}

// TestTupleParameterExpands: a tuple parameter crosses as one IR
// parameter per element (design D4's per-field spread) — the callee's
// define takes the fields, never a tuple pointer.
func TestTupleParameterExpands(t *testing.T) {
	sum := &ast.FnDecl{
		Name:   "sum",
		Params: []ast.Param{{Name: "p", Type: tupType("Int64", "Int64")}},
		Ret:    named("Int64"),
		Body: ast.Block{Items: []ast.Stmt{
			&ast.Binding{Kw: "let", Pat: patTuple("lo", "hi"), Init: ident("p")},
			&ast.Return{HasValue: true, Value: binOp("+", ident("lo"), ident("hi"))},
		}},
	}
	ir := assertClean(t, recModule(
		[]ast.Item{sum},
		&ast.Binding{Kw: "let", Name: "n",
			Init: &ast.Call{Fn: ident("sum"), Args: []ast.Expr{tup(intLit("3"), intLit("4"))}}},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "define i64 @main.sum(i64 %p.0, i64 %p.1)")
	// One i64 operand per element at the call site — the callee operand is
	// the loaded slot again (design D3), so the expansion shows in the
	// argument list rather than in a named callee.
	if !regexp.MustCompile(`= call i64 %v\d+\(i64 \S+, i64 \S+\)`).MatchString(ir) {
		t.Fatalf("the argument expands field-per-word:\n%s", ir)
	}
}
