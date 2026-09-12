package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// T3 monomorphization driver (design D3). The emitter instantiates lazily
// from a worklist: a body site that writes an application registers the
// declaration's instantiation and the define loop drains what the
// registration appended, so a template reached through another
// instantiation's body emits too. What is pinned here is that driver —
// the instantiation a written argument names, the registration order the
// defines follow, and the internment that keeps two sites one
// instantiation.

// tparam builds one type parameter of a generic clause.
func tparam(name string) *ast.TypeParam { return &ast.TypeParam{Name: name} }

// targs builds one explicit type-argument clause.
func targs(ts ...ast.TypeRef) []ast.TypeRef { return ts }

// genericRec declares `record Name<T…> { fields }`.
func genericRec(name string, params []*ast.TypeParam, fields ...ast.FieldDecl) *ast.RecordDecl {
	return &ast.RecordDecl{Name: name, TypeParams: params, Fields: fields}
}

// genericFn declares `fn name<T…>(params) -> ret { body }`.
func genericFn(name string, params []*ast.TypeParam, ps []ast.Param, ret ast.TypeRef, stmts ...ast.Stmt) *ast.FnDecl {
	return &ast.FnDecl{Name: name, TypeParams: params, Params: ps, Ret: ret, Body: ast.Block{Items: stmts}}
}

// param builds one `name: type` parameter.
func param(name string, typ ast.TypeRef) ast.Param { return ast.Param{Name: name, Type: typ} }

// applied builds `Name<args…>`, the reference form an instantiation is
// written as.
func applied(name string, args ...ast.TypeRef) *ast.NamedType {
	return &ast.NamedType{Name: name, Args: args}
}

// ctorTypeArgs builds a construction with an explicit generic clause.
func ctorTypeArgs(name string, args []ast.TypeRef, inits ...ast.FieldInit) *ast.Construct {
	return &ast.Construct{Name: name, TypeArgs: args, Fields: inits}
}

// calledWith builds `fn<args…>(callArgs…)`.
func calledWith(fn string, typeArgs []ast.TypeRef, callArgs ...ast.Expr) *ast.Call {
	return &ast.Call{Fn: &ast.Ident{Name: fn}, Args: callArgs, TypeArgs: typeArgs}
}

// retExpr builds `return expr`.
func retExpr(x ast.Expr) *ast.Return { return &ast.Return{HasValue: true, Value: x} }

// interp builds one interpolated string literal around a single hole —
// the form the parser hands the emitter (the runs before and after, and
// the expression the region read). A literal carrying `${` in its raw
// text with no holes is not a parse and does not stand in for one.
func interp(hole ast.Expr) *ast.Literal {
	return &ast.Literal{Kind: "string", Text: `"${…}"`, Segs: []string{"", ""}, Holes: []ast.Expr{hole}}
}

// instModule wraps the declarations and the main statements into one
// module, the io import and the skeleton error sum included.
func instModule(decls []ast.Item, stmts ...ast.Stmt) ProgModule {
	items := []ast.Item{stdIoImport("io"), appError()}
	items = append(items, decls...)
	items = append(items, mainDecl(stmts...))
	return ProgModule{Key: "main", File: &ast.File{Items: items}}
}

// A written application names one instantiation of the record, and what
// the tables hold under it is a synthesized declaration of its own: the
// template's fields with the parameters substituted. The template itself
// lays out nothing — a generic record is not a type until it is applied,
// and the struct line under its bare key would be a phantom type beside
// the real ones.
func TestGenericRecordInstantiationLaysOut(t *testing.T) {
	box := genericRec("Box", []*ast.TypeParam{tparam("T")}, fld("v", "T"))
	ir, ni := EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{box},
		letBind("b", ctorTypeArgs("Box", targs(named("Int64")), init1("v", intLit("7")))),
		ioCall("io", "println", interp(memberOf(ident("b"), "v"))),
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "%struct.main.Box$Int64 = type { i64 }", "the instantiated layout")
	wantNoIR(t, ir, "%struct.main.Box =", "the unapplied template's layout")
	wantIR(t, ir, "@.map.main.Box$Int64", "the instantiated map descriptor")
}

// Two instantiations of one template are two types: each carries its own
// layout, and the argument each was applied at is what the field's slot
// holds. The String instantiation's field is a pair while Int64's is one
// word, which is the substitution reaching the layout rather than a name
// being rewritten.
func TestGenericRecordInstantiationsDifferByArgument(t *testing.T) {
	box := genericRec("Box", []*ast.TypeParam{tparam("T")}, fld("v", "T"))
	ir, ni := EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{box},
		letBind("n", ctorTypeArgs("Box", targs(named("Int64")), init1("v", intLit("7")))),
		letBind("s", ctorTypeArgs("Box", targs(named("String")), init1("v", strLit(`"w"`)))),
		ioCall("io", "println", interp(memberOf(ident("n"), "v"))),
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "%struct.main.Box$Int64 = type { i64 }", "the scalar instantiation")
	wantIR(t, ir, "%struct.main.Box$String = type { ptr, i64 }", "the String instantiation")
}

// The driver's recursion (design D3): a body site registers the
// instantiation it reaches, and the define loop drains what the
// registration appended. `f<Int64>` reaches `g<Int64>` from inside its own
// body — after the loop had already passed that point in the list — so g's
// define exists only because the loop is a fixpoint.
//
// `fn f<T>(x: Box<T>)` reaches g with its own type argument written,
// which is what makes the second registration transitive rather than
// independent: the record application, the parameter's binding, and the
// callee's arguments all substitute one substitution.
func TestRecursiveInstantiationRegistersInOrder(t *testing.T) {
	box := genericRec("Box", []*ast.TypeParam{tparam("T")}, fld("v", "T"))
	g := genericFn("g", []*ast.TypeParam{tparam("T")},
		[]ast.Param{param("b", applied("Box", named("T")))}, named("T"),
		retExpr(memberOf(ident("b"), "v")))
	f := genericFn("f", []*ast.TypeParam{tparam("T")},
		[]ast.Param{param("b", applied("Box", named("T")))}, named("T"),
		retExpr(calledWith("g", targs(named("T")), ident("b"))))

	ir, ni := EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{box, g, f},
		letBind("n", calledWith("f", targs(named("Int64")),
			ctorTypeArgs("Box", targs(named("Int64")), init1("v", intLit("7"))))),
		ioCall("io", "println", interp(ident("n"))),
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	gAt := strings.Index(ir, "define i64 @main.g$Int64(")
	fAt := strings.Index(ir, "define i64 @main.f$Int64(")
	if gAt < 0 || fAt < 0 {
		t.Fatalf("an instantiated define is missing (g at %d, f at %d)\n--\n%s", gAt, fAt, ir)
	}
	// Registration order is the order the defines emit in (design D3): f
	// was reached from main, g only from f's own body.
	if fAt > gAt {
		t.Fatalf("g's define precedes f's: the loop is not draining its own registrations\n--\n%s", ir)
	}
	// The call reaches the instantiation through its slot (design D3's
	// calling face), so the slot's own line is what names it.
	wantIR(t, ir, "@slot.main.g$Int64 = global ptr @main.g$Int64", "the transitive call")
	// The record is one instantiation across the site and the signatures:
	// the layout the construct stored into is the one f's and g's
	// parameter bound at.
	wantIR(t, ir, "%struct.main.Box$Int64 = type { i64 }", "the shared layout")
}

// Interning (design D2): two sites writing the same application are one
// instantiation, so the module carries one define and one layout for it.
// The count is asserted rather than the presence, because a second
// instantiation would be a second name for one type — the collision the
// table exists to catch.
func TestInstantiationIsInternedAcrossSites(t *testing.T) {
	box := genericRec("Box", []*ast.TypeParam{tparam("T")}, fld("v", "T"))
	id := genericFn("id", []*ast.TypeParam{tparam("T")},
		[]ast.Param{param("x", named("T"))}, named("T"), retExpr(ident("x")))

	ir, ni := EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{box, id},
		letBind("a", calledWith("id", targs(named("Int64")), intLit("1"))),
		letBind("b", calledWith("id", targs(named("Int64")), intLit("2"))),
		letBind("c", ctorTypeArgs("Box", targs(named("Int64")), init1("v", intLit("3")))),
		letBind("d", ctorTypeArgs("Box", targs(named("Int64")), init1("v", intLit("4")))),
		ioCall("io", "println", interp(ident("a"))),
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	if n := strings.Count(ir, "define i64 @main.id$Int64("); n != 1 {
		t.Fatalf("the fn instantiated %d times, want 1\n--\n%s", n, ir)
	}
	if n := strings.Count(ir, "%struct.main.Box$Int64 = type "); n != 1 {
		t.Fatalf("the record instantiated %d times, want 1\n--\n%s", n, ir)
	}
}

// A generic callee whose arguments the call leaves unwritten is the check
// stage's inference to make (design D1), and this layer does not guess it:
// the site stops at the boundary the whole B1a corpus pins, with the same
// message and exit code an inferred application always had.
func TestUnwrittenTypeArgumentsStopAtTheGenericBoundary(t *testing.T) {
	id := genericFn("id", []*ast.TypeParam{tparam("T")},
		[]ast.Param{param("x", named("T"))}, named("T"), retExpr(ident("x")))
	_, ni := EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{id},
		letBind("a", calledWith("id", nil, intLit("1"))),
		ioCall("io", "println", interp(ident("a"))),
		okReturn(),
	)})
	if ni == nil {
		t.Fatal("an inferred application emitted")
	}
	if ni.What != bndGenericFns {
		t.Fatalf("boundary word = %q, want %q", ni.What, bndGenericFns)
	}
}

// An unapplied generic head is a template, not a type: constructing one is
// no record the tables hold, so the site stops at the boundary rather than
// laying out a declaration with a parameter in it.
func TestUnappliedGenericConstructionStops(t *testing.T) {
	box := genericRec("Box", []*ast.TypeParam{tparam("T")}, fld("v", "T"))
	_, ni := EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{box},
		letBind("b", ctorTypeArgs("Box", nil, init1("v", intLit("7")))),
		ioCall("io", "println", interp(memberOf(ident("b"), "v"))),
		okReturn(),
	)})
	if ni == nil {
		t.Fatal("an unapplied generic head laid out")
	}
}

// Design D3's upper-bound guard, the task's hard requirement: an argument
// deeper than the declaration's own parameter count plus the limit is an
// internal error, never an unbounded walk. `f<Box<T>>` reached from inside
// f is the shape that deepens itself — every round binds T one Box deeper,
// so each instantiation is a new name and nothing downstream ever meets a
// registration twice to stop on.
//
// The failure is a panic naming the instantiation, not a hang: the
// boundary vocabulary is for programs this build does not carry yet, and a
// driver that cannot terminate is not a program's fault.
func TestSelfDeepeningInstantiationIsAnInternalError(t *testing.T) {
	box := genericRec("Box", []*ast.TypeParam{tparam("T")}, fld("v", "T"))
	f := genericFn("f", []*ast.TypeParam{tparam("T")}, nil, nil,
		&ast.ExprStmt{Expr: calledWith("f", targs(applied("Box", named("T"))))})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("the self-deepening instantiation returned")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "exceeds the type argument depth limit") {
			t.Fatalf("panic = %v, want the depth guard's message", r)
		}
	}()
	EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{box, f},
		&ast.ExprStmt{Expr: calledWith("f", targs(named("Int64")))},
		okReturn(),
	)})
	t.Fatal("the driver returned where it should have stopped")
}

// The same guard, reached without a declaration: a tuple argument grows by
// nesting alone, so no table ever holds the same name twice and the
// interning that stops a plain self-reference cannot apply. The fn side's
// own check is the only thing between this shape and an unbounded symbol
// list.
func TestDeepeningTupleInstantiationIsAnInternalError(t *testing.T) {
	f := genericFn("f", []*ast.TypeParam{tparam("T")}, nil, nil,
		&ast.ExprStmt{Expr: calledWith("f", targs(&ast.TupleType{Elems: []ast.TypeRef{named("T"), named("Int64")}}))})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("the deepening tuple instantiation returned")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "exceeds the type argument depth limit") {
			t.Fatalf("panic = %v, want the depth guard's message", r)
		}
	}()
	EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{f},
		&ast.ExprStmt{Expr: calledWith("f", targs(named("Int64")))},
		okReturn(),
	)})
	t.Fatal("the driver returned where it should have stopped")
}

// A recursive instantiation that does not deepen is not a loop: the
// interned symbol makes the second visit the same instantiation, so a
// template that reaches its own instantiation at the same arguments
// terminates on the registration. This is the shape the guard must not
// reject, and the reason the guard reads depth rather than counting
// visits.
func TestSelfReferenceAtTheSameArgumentsTerminates(t *testing.T) {
	f := genericFn("f", []*ast.TypeParam{tparam("T")},
		[]ast.Param{param("x", named("T"))}, named("T"),
		retExpr(calledWith("f", targs(named("T")), ident("x"))))
	ir, ni := EmitProgram(ModeBuild, []ProgModule{instModule([]ast.Item{f},
		letBind("n", calledWith("f", targs(named("Int64")), intLit("1"))),
		ioCall("io", "println", interp(ident("n"))),
		okReturn(),
	)})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	if n := strings.Count(ir, "define i64 @main.f$Int64("); n != 1 {
		t.Fatalf("the self-referring template emitted %d defines, want 1\n--\n%s", n, ir)
	}
}

// The driver is deterministic: registration order is a list, never a map
// walk, so the same source emits the same bytes. Two emissions of an
// independently rebuilt AST are compared in one process, where Go
// randomizes every map iteration — a registration keyed by a map would
// show here as a difference in define order or symbol spelling.
func TestInstantiationEmissionIsDeterministic(t *testing.T) {
	src := func() ProgModule {
		box := genericRec("Box", []*ast.TypeParam{tparam("T")}, fld("v", "T"))
		pair := genericRec("Pair", []*ast.TypeParam{tparam("T"), tparam("U")},
			ast.FieldDecl{Name: "a", Typ: named("T")}, ast.FieldDecl{Name: "b", Typ: named("U")})
		id := genericFn("id", []*ast.TypeParam{tparam("T")},
			[]ast.Param{param("x", named("T"))}, named("T"), retExpr(ident("x")))
		g := genericFn("g", []*ast.TypeParam{tparam("T")},
			[]ast.Param{param("b", applied("Box", named("T")))}, named("T"),
			retExpr(memberOf(ident("b"), "v")))
		f := genericFn("f", []*ast.TypeParam{tparam("T")},
			[]ast.Param{param("b", applied("Box", named("T")))}, named("T"),
			retExpr(calledWith("g", targs(named("T")), ident("b"))))
		return instModule([]ast.Item{box, pair, id, g, f},
			letBind("n", calledWith("f", targs(named("Int64")),
				ctorTypeArgs("Box", targs(named("Int64")), init1("v", intLit("7"))))),
			letBind("s", calledWith("id", targs(named("String")), strLit(`"w"`))),
			letBind("p", ctorTypeArgs("Pair", targs(named("Int64"), named("String")),
				init1("a", intLit("1")), init1("b", strLit(`"x"`)))),
			ioCall("io", "println", interp(ident("n"))),
			okReturn(),
		)
	}
	first, ni := EmitProgram(ModeBuild, []ProgModule{src()})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	for i := 0; i < 8; i++ {
		again, ni := EmitProgram(ModeBuild, []ProgModule{src()})
		if ni != nil {
			t.Fatalf("boundary on run %d: %s", i, ni.What)
		}
		if again != first {
			t.Fatalf("run %d differs from the first:\n--first--\n%s\n--run %d--\n%s", i, first, i, again)
		}
	}
	// The instantiated faces are in the module at all: byte-identity over
	// an empty set of them would prove nothing.
	wantIR(t, first, "@main.f$Int64", "the transitive instantiation")
	wantIR(t, first, "@main.g$Int64", "the fn reached from its body")
	wantIR(t, first, "@main.id$String", "the String instantiation")
	wantIR(t, first, "%struct.main.Pair$Int64$String = type ", "the two-parameter layout")
}

// The shape projection is the emitter's inverse of the checker's own
// identity (design D2): a reference the substitution produced carries the
// shape it stands for, and a shape built back from it mangles to the key
// the tables hold. What that buys is the one table — a site's written
// application and an instantiated body's substituted one name the same
// instantiation.
func TestShapeKeyRoundTripsThroughTheProjection(t *testing.T) {
	point := &ast.RecordDecl{Name: "Point"}
	e := &emitter{declKeys: declIndex([]ProgModule{{Key: "main", ID: "main",
		File: &ast.File{Items: []ast.Item{point}}}})}
	e.records = map[string]*ast.RecordDecl{}
	e.order = nil

	ref := e.refOfShape(typecheck.Shape{
		Kind: typecheck.ShapeNominal,
		Decl: typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: point},
	})
	n, ok := ref.(*ast.NamedType)
	if !ok {
		t.Fatalf("a nominal shape rendered %T", ref)
	}
	back, ok := e.shapeOfRef(n)
	if !ok {
		t.Fatal("the rendered reference did not project back")
	}
	got, ok := e.shapeKey(back)
	if !ok {
		t.Fatal("the projected shape did not render a key")
	}
	if got != "main.Point" {
		t.Fatalf("shapeKey(shapeOfRef(refOfShape(s))) = %q, want %q", got, "main.Point")
	}
	if got, want := e.namedKey("main", n), "main.Point"; got != want {
		t.Fatalf("namedKey = %q, want %q", got, want)
	}
}
