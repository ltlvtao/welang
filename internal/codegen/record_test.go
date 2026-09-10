package codegen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T5 record value face (design D4). Before the task a record's members
// were readable only when the chain ended on a String field: a scalar
// or Float64 field read, a construction whose field values are
// expressions rather than literals, an update expression, and the copy
// semantics of a value record all stopped at the body boundary.
//
// The copy face is the one place the task's plan met the emitter's root
// protocol: design D4 names a `__we_rec_copy` runtime face, but a callee's
// root pushes cannot be balanced by its caller — the pushes an allocation
// owes are counted per body (`e.pushes`, popped at the body's exits), so
// the copy expands inline at each site and hands its pushes to the body
// that asked for it. The face is real; its spelling is the emitter's.

// fld builds one record field declaration.
func fld(name, typ string) ast.FieldDecl {
	return ast.FieldDecl{Name: name, Typ: named(typ)}
}

// recDecl builds one record declaration of the given category.
func recDecl(name, cat string, fields ...ast.FieldDecl) *ast.RecordDecl {
	return &ast.RecordDecl{Name: name, Cat: cat, Fields: fields}
}

// construct builds `Name { field: value, ... }`.
func construct(name string, inits ...ast.FieldInit) *ast.Construct {
	return &ast.Construct{Name: name, Fields: inits}
}

// init1 builds one field initializer.
func init1(name string, value ast.Expr) ast.FieldInit {
	return ast.FieldInit{Name: name, Value: value}
}

// memberOf builds `recv.name`.
func memberOf(recv ast.Expr, name string) *ast.Member {
	return &ast.Member{Recv: recv, Name: name}
}

// recModule wraps statements with the skeleton sum, the declarations, and
// the io import (the print face the record programs observe through).
func recModule(decls []ast.Item, stmts ...ast.Stmt) *ast.File {
	items := []ast.Item{stdIoImport("io"), appError()}
	items = append(items, decls...)
	items = append(items, mainDecl(stmts...))
	return &ast.File{Items: items}
}

// gepsOf counts the getelementptr sites addressing one offset — the
// field access shape: one gep plus one load, never a call.
func gepsOf(ir string, off int) int {
	re := regexp.MustCompile(`getelementptr i8, ptr %[\w.]+, i64 ` + strconv.Itoa(off) + "\n")
	return len(re.FindAllString(ir, -1))
}

// readsAt counts the read sites addressing one offset — the gep/load
// pair a member read emits. The construction stores through a gep of
// their own, so counting geps alone cannot tell a read from a store.
func readsAt(ir string, off int) int {
	re := regexp.MustCompile(`getelementptr i8, ptr %[\w.]+, i64 ` + strconv.Itoa(off) + "\n\\s+%[\\w.]+ = load i64, ptr %")
	return len(re.FindAllString(ir, -1))
}

// allocCount counts the allocations a copy protocol spends.
func allocCount(ir string) int {
	return strings.Count(ir, "= call ptr @__we_alloc(")
}

// TestRecordScalarFieldRead: `p.x` on an Int64 field is one
// getelementptr at the field's offset plus one load — the unified member
// read that replaces the String-only chain (chapter 8's field access is
// the same production whatever the field's kind).
func TestRecordScalarFieldRead(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{recDecl("Point", "gc", fld("x", "Int64"), fld("y", "Int64"))},
		&ast.Binding{Kw: "let", Name: "p", Init: construct("Point",
			init1("x", intLit("1")), init1("y", intLit("2")))},
		&ast.Binding{Kw: "let", Name: "n", Init: binOp("+", memberOf(ident("p"), "x"), memberOf(ident("p"), "y"))},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("n"))),
		okReturn(),
	), "getelementptr i8, ptr %v", "= load i64, ptr %")
	if gepsOf(ir, 16) == 0 || gepsOf(ir, 24) == 0 {
		t.Fatalf("both fields read at 16 and 24:\n%s", ir)
	}
	// Each name is tied to its own word. Reading both fields only says
	// the pair 16/24 is in use — shifting every read by one word keeps
	// that true, so the pairing needs a program that reads one field
	// alone (found by mutating the offset to off+8: unit green, golden
	// red; see the T5 record's mutation table).
	only := assertClean(t, recModule(
		[]ast.Item{recDecl("Point", "gc", fld("x", "Int64"), fld("y", "Int64"))},
		&ast.Binding{Kw: "let", Name: "p", Init: construct("Point",
			init1("x", intLit("1")), init1("y", intLit("2")))},
		&ast.Binding{Kw: "let", Name: "m", Init: memberOf(ident("p"), "y")},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("m"))),
		okReturn(),
	), "")
	if readsAt(only, 24) != 1 || readsAt(only, 16) != 0 {
		t.Fatalf("`p.y` is field y's word alone:\n%s", only)
	}
}

// TestRecordFloatFieldRead: a Float64 field loads a double, and the
// float domain survives into its consumers.
func TestRecordFloatFieldRead(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{recDecl("Circle", "gc", fld("r", "Float64"))},
		&ast.Binding{Kw: "let", Name: "c", Init: construct("Circle", init1("r", &ast.Literal{Kind: "float", Text: "1.5"}))},
		&ast.Binding{Kw: "let", Name: "d", Init: binOp("*", memberOf(ident("c"), "r"), &ast.Literal{Kind: "float", Text: "2.0"})},
		ioCall("io", "println", interpLit([]string{"", ""}, ident("d"))),
		okReturn(),
	), "load double, ptr %")
	if !strings.Contains(ir, "fmul double") {
		t.Fatalf("the read field multiplies in the double domain:\n%s", ir)
	}
}

// TestRecordUpdateCopiesTheBase: `Point { x: 3 with &p }` allocates a
// second object, copies the unnamed field from the base, and overwrites
// the named one — the base is never stored through (chapter 8: "the base
// value is unaffected").
func TestRecordUpdateCopiesTheBase(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{recDecl("Point", "gc", fld("x", "Int64"), fld("y", "Int64"))},
		&ast.Binding{Kw: "let", Name: "p", Init: construct("Point",
			init1("x", intLit("1")), init1("y", intLit("2")))},
		&ast.Binding{Kw: "let", Name: "q", Init: &ast.Construct{
			Name:   "Point",
			Fields: []ast.FieldInit{init1("x", intLit("3"))},
			Base:   ident("p"),
		}},
		ioCall("io", "println", interpLit([]string{"", ""}, memberOf(ident("q"), "x"))),
		okReturn(),
	), "= call ptr @__we_alloc(i64 32)")
	if got := allocCount(ir); got != 2 {
		t.Fatalf("the update allocates its own object: want 2 allocations, got %d:\n%s", got, ir)
	}
	// The copied field is a load of the base's word stored into the new
	// object; the overwritten one is the literal 3. The load has to be
	// what the store carries — a load anywhere in the module also comes
	// from the read below (found by mutating the copy's source operand
	// to a constant: the presence check stayed green).
	if !strings.Contains(ir, "= load i64, ptr %") {
		t.Fatalf("the unnamed field copies from the base:\n%s", ir)
	}
	if !regexp.MustCompile(`store i64 %v\d+, ptr %`).MatchString(ir) {
		t.Fatalf("the copied field stores the base's loaded word:\n%s", ir)
	}
}

// TestValueRecordBindingCopies: binding a byval record copies the whole
// value — chapter 8's "two bindings of a copied value share nothing".
func TestValueRecordBindingCopies(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{recDecl("Point", "value", fld("x", "Int64"), fld("y", "Int64"))},
		&ast.Binding{Kw: "let", Name: "p", Init: construct("Point",
			init1("x", intLit("1")), init1("y", intLit("2")))},
		&ast.Binding{Kw: "let", Name: "q", Init: ident("p")},
		ioCall("io", "println", interpLit([]string{"", ""}, memberOf(ident("q"), "x"))),
		okReturn(),
	), "")
	if got := allocCount(ir); got != 2 {
		t.Fatalf("a value-record binding copies: want 2 allocations, got %d:\n%s", got, ir)
	}
}

// TestGcRecordBindingShares: the same re-binding over a gc record emits
// no second allocation — chapter 8's categories differ exactly here.
func TestGcRecordBindingShares(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{recDecl("Point", "gc", fld("x", "Int64"))},
		&ast.Binding{Kw: "let", Name: "p", Init: construct("Point", init1("x", intLit("1")))},
		&ast.Binding{Kw: "let", Name: "q", Init: ident("p")},
		ioCall("io", "println", interpLit([]string{"", ""}, memberOf(ident("q"), "x"))),
		okReturn(),
	), "")
	if got := allocCount(ir); got != 1 {
		t.Fatalf("a gc-record binding shares its object: want 1 allocation, got %d:\n%s", got, ir)
	}
}

// TestValueRecordArgumentCopies: passing a value record copies at the
// call site (chapter 8's "Passing copies" scenario), so the callee's
// parameter is an independent object.
func TestValueRecordArgumentCopies(t *testing.T) {
	shift := &ast.FnDecl{
		Name:   "shift",
		Params: []ast.Param{{Name: "p", Type: named("Point")}},
		Ret:    named("Point"),
		Body: ast.Block{Items: []ast.Stmt{
			&ast.Return{HasValue: true, Value: construct("Point",
				init1("x", binOp("+", memberOf(ident("p"), "x"), intLit("1"))),
				init1("y", memberOf(ident("p"), "y")))},
		}},
	}
	ir := assertClean(t, recModule(
		[]ast.Item{recDecl("Point", "value", fld("x", "Int64"), fld("y", "Int64")), shift},
		&ast.Binding{Kw: "let", Name: "p", Init: construct("Point",
			init1("x", intLit("1")), init1("y", intLit("2")))},
		&ast.Binding{Kw: "let", Name: "q",
			Init: &ast.Call{Fn: ident("shift"), Args: []ast.Expr{ident("p")}}},
		ioCall("io", "println", interpLit([]string{"", ""}, memberOf(ident("q"), "x"))),
		okReturn(),
	), "")
	// One allocation for p, one for the argument copy, one for the
	// callee's return value: three in total.
	if got := allocCount(ir); got != 3 {
		t.Fatalf("the argument copies and the return copies: want 3 allocations, got %d:\n%s", got, ir)
	}
}

// TestNestedValueRecordCopyIsDeep: a copy of a value record whose field
// is itself a value record copies the inner value too — "share nothing"
// reaches every level.
func TestNestedValueRecordCopyIsDeep(t *testing.T) {
	ir := assertClean(t, recModule(
		[]ast.Item{
			recDecl("Inner", "value", fld("n", "Int64")),
			recDecl("Outer", "value", fld("inner", "Inner")),
		},
		&ast.Binding{Kw: "let", Name: "a", Init: construct("Outer",
			init1("inner", construct("Inner", init1("n", intLit("1")))))},
		&ast.Binding{Kw: "let", Name: "b", Init: ident("a")},
		ioCall("io", "println", interpLit([]string{"", ""}, memberOf(memberOf(ident("b"), "inner"), "n"))),
		okReturn(),
	), "")
	// One Inner + one Outer for the construction, then the copy's own
	// Inner and Outer: four.
	if got := allocCount(ir); got != 4 {
		t.Fatalf("the nested copy recurses: want 4 allocations, got %d:\n%s", got, ir)
	}
}

// TestRecordConstructionTakesExpressions: a field initializer is an
// expression, not a literal slot — chapter 8's construction form admits
// any expression of the field's type.
func TestRecordConstructionTakesExpressions(t *testing.T) {
	assertClean(t, recModule(
		[]ast.Item{recDecl("Point", "gc", fld("x", "Int64"), fld("label", "String"))},
		&ast.Binding{Kw: "let", Name: "n", Init: intLit("1")},
		&ast.Binding{Kw: "let", Name: "p", Init: construct("Point",
			init1("x", binOp("+", ident("n"), intLit("1"))),
			init1("label", strLit(`"a"`)))},
		ioCall("io", "println", memberOf(ident("p"), "label")),
		okReturn(),
	), "= call ptr @__we_alloc(i64 40)")
}
