package typecheck

import (
	"reflect"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// The export face's own tests (design D1): the projection of the
// checker's types onto the closed Shape view, the declaration face's slot
// order, the identity that keeps two modules' same-named types apart, and
// the sites a real check registers.

// TestShapeProjection pins the projection form by form — the shapes
// codegen instantiates from, each carrying exactly the payload its arm
// needs and nothing that renders.
func TestShapeProjection(t *testing.T) {
	point := &recordInfo{name: "Point", cat: "gc"}
	wrap := &recordInfo{name: "Wrap", cat: "gc", params: []string{"T"}}

	base := shapeOf(baseType("Int64"))
	if base.Kind != ShapeBase || base.Name != "Int64" {
		t.Fatalf("Int64 projected as %+v", base)
	}
	if got := shapeOf(unitType{}); got.Kind != ShapeUnit {
		t.Fatalf("unit projected as %+v", got)
	}
	if got := shapeOf(neverType{}); got.Kind != ShapeNever {
		t.Fatalf("Never projected as %+v", got)
	}
	if got := shapeOf(paramRef{idx: 1, name: "U"}); got.Kind != ShapeParam || got.Pos != 1 {
		t.Fatalf("a clause position projected as %+v", got)
	}
	// A valueless position is unit at the type level; the positions that
	// must keep the distinction carry a nil Ret instead (ShapeFn below).
	if got := shapeOf(nil); got.Kind != ShapeUnit {
		t.Fatalf("the absent type projected as %+v", got)
	}

	// Wrap<Point> and List<Int64>: a nominal application carries its
	// declaration and its arguments, recursively.
	wrapped := shapeOf(recordType{decl: wrap, args: []Type{recordType{decl: point}}})
	if wrapped.Kind != ShapeNominal || wrapped.Decl.Kind != DeclRecord || wrapped.Decl.Name != "Wrap" {
		t.Fatalf("Wrap<Point> projected as %+v", wrapped)
	}
	if len(wrapped.Args) != 1 || wrapped.Args[0].Decl.Name != "Point" {
		t.Fatalf("Wrap<Point> arguments projected as %+v", wrapped.Args)
	}
	list := shapeOf(namedType{decl: listSum, args: []Type{baseType("Int64")}})
	if list.Kind != ShapeNominal || list.Decl.Kind != DeclSum || list.Decl.Name != "List" {
		t.Fatalf("List<Int64> projected as %+v", list)
	}
	if len(list.Args) != 1 || !list.Args[0].Equal(base) {
		t.Fatalf("List<Int64> argument projected as %+v", list.Args)
	}

	// A tuple keeps its elements in order; a fn keeps its parameters and
	// its one return.
	tup := shapeOf(tupleType{elems: []Type{baseType("Int64"), baseType("String")}})
	if tup.Kind != ShapeTuple || len(tup.Elems) != 2 || tup.Elems[1].Name != "String" {
		t.Fatalf("a tuple projected as %+v", tup)
	}
	fn := shapeOf(fnType{params: []Type{baseType("Int64")}, ret: baseType("Bool")})
	if fn.Kind != ShapeFn || len(fn.Params) != 1 || fn.Ret == nil || fn.Ret.Kind != ShapeBase || fn.Ret.Name != "Bool" {
		t.Fatalf("fn(Int64) -> Bool projected as %+v", fn)
	}

	// Dyn<Iterator<Int64>>: the box's face, the interface and its
	// argument.
	dyn := shapeOf(dynType{inf: iteratorIface, args: []Type{baseType("Int64")}})
	if dyn.Kind != ShapeDyn || dyn.Decl.Kind != DeclIface || dyn.Decl.Name != "Iterator" {
		t.Fatalf("Dyn<Iterator<Int64>> projected as %+v", dyn)
	}
	if len(dyn.Args) != 1 || dyn.Args[0].Name != "Int64" {
		t.Fatalf("Dyn argument projected as %+v", dyn.Args)
	}

	// An associated position is itself: unresolved here, and named.
	assoc := shapeOf(assocRef{iface: iterableIface, name: "Iter"})
	if assoc.Kind != ShapeAssoc || assoc.Name != "Iter" || assoc.Decl.Name != "Iterable" {
		t.Fatalf("an associated position projected as %+v", assoc)
	}
}

// TestShapeEqualIsIdentity checks the equality codegen keys on: base types
// by name, applications by declaration identity plus arguments, and the
// absent return of a fn that declares none.
func TestShapeEqualIsIdentity(t *testing.T) {
	one := shapeOf(namedType{decl: listSum, args: []Type{baseType("Int64")}})
	two := shapeOf(namedType{decl: listSum, args: []Type{baseType("Int64")}})
	if !one.Equal(two) {
		t.Fatal("two projections of List<Int64> compare unequal")
	}
	if one.Equal(shapeOf(namedType{decl: listSum, args: []Type{baseType("String")}})) {
		t.Fatal("List<Int64> equals List<String>")
	}
	valueless := shapeOf(fnType{params: []Type{baseType("Int64")}})
	if valueless.Ret != nil {
		t.Fatal("a fn declaring no return projected a return")
	}
	if valueless.Equal(shapeOf(fnType{params: []Type{baseType("Int64")}, ret: unitType{}})) {
		t.Fatal("a valueless fn equals one returning unit")
	}
	if !valueless.Equal(shapeOf(fnType{params: []Type{baseType("Int64")}})) {
		t.Fatal("two projections of the same valueless fn compare unequal")
	}
}

// TestIteratorFaceIsOneSlot pins the declaration face D6 dispatches from:
// Iterator<T>'s method set is not in any source file, and exactly one of
// its methods carries no default body — next, whose return is Option<T> at
// the interface's own position.
func TestIteratorFaceIsOneSlot(t *testing.T) {
	face, ok := faceNamed(t, "Iterator")
	if !ok {
		t.Fatal("no Iterator face in the registry")
	}
	if face.Decl.Kind != DeclIface || face.Decl.Node != nil {
		t.Fatalf("the builtin face's identity is %+v", face.Decl)
	}
	if len(face.Slots) != 1 {
		t.Fatalf("Iterator's slots are %+v, want exactly [next]", face.Slots)
	}
	next := face.Slots[0]
	if next.Name != "next" || len(next.Params) != 0 {
		t.Fatalf("the slot is %+v", next)
	}
	if next.Ret == nil || next.Ret.Kind != ShapeNominal || next.Ret.Decl.Name != "Option" {
		t.Fatalf("next's return projects as %+v", next.Ret)
	}
	if len(next.Ret.Args) != 1 || next.Ret.Args[0].Kind != ShapeParam || next.Ret.Args[0].Pos != 0 {
		t.Fatalf("Option's argument projects as %+v", next.Ret.Args)
	}
}

// TestIteratorCombinatorsAreNotSlots pins the other half of the same
// criterion: the combinators carry a default body (the eager ones a real
// body, the lazy ones and collect the compiler-intrinsic marker), so none
// of them is an obligation a box must dispatch.
func TestIteratorCombinatorsAreNotSlots(t *testing.T) {
	face, ok := faceNamed(t, "Iterator")
	if !ok {
		t.Fatal("no Iterator face in the registry")
	}
	for _, name := range []string{"map", "filter", "take", "skip", "collect", "fold", "reduce", "count", "any", "all", "find"} {
		for _, s := range face.Slots {
			if s.Name == name {
				t.Fatalf("%q registered as a vtable slot; its default body is an obligation the box does not carry", name)
			}
		}
	}
}

// TestReleasableFaceIsOneValuelessSlot pins the second builtin face a box
// carries: release has no default body, takes no parameters, and declares
// no return — the nil Ret that keeps valueless distinct from unit.
func TestReleasableFaceIsOneValuelessSlot(t *testing.T) {
	face, ok := faceNamed(t, "Releasable")
	if !ok {
		t.Fatal("no Releasable face in the registry")
	}
	if len(face.Slots) != 1 || face.Slots[0].Name != "release" {
		t.Fatalf("Releasable's slots are %+v", face.Slots)
	}
	if face.Slots[0].Ret != nil {
		t.Fatalf("release's return projects as %+v, want nil", face.Slots[0].Ret)
	}
}

// TestShapeDeclIdentitySeparatesModules is the negative assertion behind
// the no-rendering rule: two modules declaring the same name are two
// declarations, told apart by their nodes — never by the shared name,
// which this face does not carry at all.
func TestShapeDeclIdentitySeparatesModules(t *testing.T) {
	src := "pub record Wrap<T> { v: T }\n\npub fn id<U>(x: U) -> U {\n    return x\n}\n\nfn probe() -> Int64 {\n    let w = Wrap<Int64> { v: 1 }\n    return id<Int64>(w.v)\n}\n"
	fa, sha := checkForShapes(t, "a.we", src)
	fb, shb := checkForShapes(t, "b.we", src)
	a := wrapShapeOf(t, fa, sha)
	b := wrapShapeOf(t, fb, shb)

	// A user declaration carries its node and no name: the node identifies
	// it, and the name is the rendering string this face refuses to hand
	// out (two modules' Wrap would share it).
	for _, s := range []Shape{a, b} {
		if s.Decl.Kind != DeclRecord || s.Decl.Node == nil {
			t.Fatalf("a user declaration projected as %+v", s.Decl)
		}
		if s.Decl.Name != "" {
			t.Fatalf("a user declaration exported the name %q", s.Decl.Name)
		}
	}
	// The node is the declaration's own, not a copy or a stand-in.
	if a.Decl.Node != declNamed(t, fa, "Wrap") {
		t.Fatal("the projected node is not the module's own Wrap declaration")
	}
	if a.Decl == b.Decl {
		t.Fatal("two modules' Wrap declarations share an identity")
	}
	if a.Equal(b) {
		t.Fatal("two modules' Wrap<Int64> compare equal")
	}
	// The arguments are the same base type on both sides: what separates
	// the shapes is the declaration alone.
	if !a.Args[0].Equal(b.Args[0]) {
		t.Fatalf("the fixtures' arguments differ: %+v vs %+v", a.Args[0], b.Args[0])
	}

	// A builtin declaration is told apart the other way: it has no node,
	// its name is collision-free among the singletons, and the same
	// singleton projects the same identity everywhere.
	la := shapeOf(namedType{decl: listSum, args: []Type{baseType("Int64")}})
	lb := shapeOf(namedType{decl: listSum, args: []Type{baseType("Int64")}})
	if la.Decl.Node != nil || la.Decl.Name != "List" {
		t.Fatalf("a builtin declaration projected as %+v", la.Decl)
	}
	if !la.Equal(lb) {
		t.Fatal("the builtin declaration does not project the same identity twice")
	}
}

// TestShapesRegistryRecordsSites pins the two site kinds a check fills
// with clause bindings: a generic call's and a construction's.
func TestShapesRegistryRecordsSites(t *testing.T) {
	src := "record Point { x: Int64 }\n\nrecord Wrap<T> { v: T }\n\nfn id<U>(x: U) -> U {\n    return x\n}\n\nfn probe() -> Int64 {\n    let w = Wrap<Point> { v: Point { x: 1 } }\n    return id<Int64>(w.v.x)\n}\n"
	f, sh := checkForShapes(t, "sites.we", src)

	calls := sitesWhere(sh, func(s Site) bool { return s.Ret.Kind == ShapeBase && len(s.Params) > 0 })
	if len(calls) != 1 {
		t.Fatalf("the check registered %d call sites, want 1: %+v", len(calls), calls)
	}
	call := calls[0]
	if len(call.Args) != 1 || call.Args[0].Pos != 0 || call.Args[0].Shape.Name != "Int64" {
		t.Fatalf("the call's binding is %+v", call.Args)
	}
	if len(call.Params) != 1 || !call.Params[0].Equal(call.Args[0].Shape) {
		t.Fatalf("the call's parameter face is %+v", call.Params)
	}
	if !call.Ret.Equal(call.Args[0].Shape) {
		t.Fatalf("id<Int64>'s return is %+v", call.Ret)
	}

	// The construction's face: the clause binding, the application's own
	// shape, and no parameter list (a construction calls nothing).
	ctor := wrapSiteOf(t, f, sh)
	if ctor.Ret.Decl.Node != declNamed(t, f, "Wrap") {
		t.Fatal("the construction's shape does not carry the declaration's own node")
	}
	if len(ctor.Args) != 1 || ctor.Args[0].Pos != 0 {
		t.Fatalf("the construction's binding is %+v", ctor.Args)
	}
	if ctor.Args[0].Shape.Kind != ShapeNominal || ctor.Args[0].Shape.Decl.Node != declNamed(t, f, "Point") {
		t.Fatalf("the construction bound %+v, want Point", ctor.Args[0].Shape)
	}
	if len(ctor.Params) != 0 {
		t.Fatalf("a construction recorded a parameter face: %+v", ctor.Params)
	}
	if ctor.Boxed != nil {
		t.Fatalf("a construction recorded a boxed type: %+v", ctor.Boxed)
	}
	if !ctor.Ret.Args[0].Equal(ctor.Args[0].Shape) {
		t.Fatalf("the construction's shape does not carry its argument: %+v", ctor.Ret)
	}

	// A monomorphic construction is not an instantiation and is no key:
	// Point's own construction registered nothing.
	for _, s := range allSites(sh) {
		if s.Ret.Kind == ShapeNominal && s.Ret.Decl.Node == declNamed(t, f, "Point") {
			t.Fatalf("Point's construction registered as a generic site: %+v", s)
		}
	}
}

// TestShapesRegistryRecordsBoxes pins the Dyn construction's own fact: the
// concrete type the argument boxes, which no syntax at the site states.
func TestShapesRegistryRecordsBoxes(t *testing.T) {
	src := "record Point { x: Int64 }\n\nimpl Iterator<Int64> for Point {\n    fn next(mut self) -> Option<Int64> {\n        None\n    }\n}\n\nfn probe() -> Int64 {\n    let d = Dyn<Iterator<Int64> >(Point { x: 1 })\n    let _ = d\n    return 0\n}\n"
	f, sh := checkForShapes(t, "box.we", src)
	boxes := sitesWhere(sh, func(s Site) bool { return s.Boxed != nil })
	if len(boxes) != 1 {
		t.Fatalf("the check registered %d box sites, want 1", len(boxes))
	}
	box := boxes[0]
	if box.Boxed.Kind != ShapeNominal || box.Boxed.Decl.Node != declNamed(t, f, "Point") {
		t.Fatalf("the boxed type projects as %+v", box.Boxed)
	}
	if box.Ret.Kind != ShapeDyn || box.Ret.Decl.Name != "Iterator" || box.Ret.Decl.Node != nil {
		t.Fatalf("the box's face projects as %+v", box.Ret)
	}
	if len(box.Ret.Args) != 1 || box.Ret.Args[0].Name != "Int64" {
		t.Fatalf("the box's argument projects as %+v", box.Ret.Args)
	}
	if len(box.Args) != 0 {
		t.Fatalf("a Dyn construction recorded clause bindings: %+v", box.Args)
	}
}

// TestShapesRegistryLeavesOrdinaryCallsAlone is the other side of the same
// rule: a monomorphic call binds nothing, so the registry does not grow
// with the program's call graph — and neither do the stdlib's own bodies,
// which the check walks first and whose nodes no consumer can key on.
func TestShapesRegistryLeavesOrdinaryCallsAlone(t *testing.T) {
	src := "fn twice(x: Int64) -> Int64 {\n    return x + x\n}\n\nfn id<U>(x: U) -> U {\n    return x\n}\n\nfn probe() -> Int64 {\n    return twice(twice(1))\n}\n"
	if got := allSites(checkForShapesOnly(t, "mono.we", src)); len(got) != 0 {
		t.Fatalf("a monomorphic program registered sites: %+v", got)
	}
}

// TestShapesFaceCarriesNoRendering is the mechanical half of the negative
// assertion: the export's types carry no rendering method, so no consumer
// can name a declaration by a string this stage produced. The manual half
// is the symbol-by-symbol review of the file.
func TestShapesFaceCarriesNoRendering(t *testing.T) {
	for _, ty := range []reflect.Type{
		reflect.TypeFor[Shape](),
		reflect.TypeFor[ShapeDecl](),
		reflect.TypeFor[Arg](),
		reflect.TypeFor[Site](),
		reflect.TypeFor[Slot](),
		reflect.TypeFor[Face](),
		reflect.TypeFor[*Shapes](),
	} {
		for _, name := range []string{"String", "GoString", "Format", "Error"} {
			if _, ok := ty.MethodByName(name); ok {
				t.Errorf("%s carries %s() — the export must not render", ty, name)
			}
		}
	}
}

// faceNamed reads one face out of the registry an empty check hands a
// consumer, so the assertion runs against what codegen actually receives.
func faceNamed(t *testing.T, name string) (Face, bool) {
	t.Helper()
	for _, f := range checkForShapesOnly(t, "faces.we", "fn probe() -> Int64 {\n    return 1\n}\n").Faces() {
		if f.Decl.Name == name {
			return f, true
		}
	}
	return Face{}, false
}

// checkForShapes runs one check over the source and returns the tree it
// walked with its registry, failing the test on any diagnostic or
// boundary.
func checkForShapes(t *testing.T, name, src string) (*ast.File, *Shapes) {
	t.Helper()
	f := parseModule(t, name, src)
	d, ni, sh := Check(f, name, SingleFile)
	if d != nil || ni != nil {
		t.Fatalf("%s: want clean, got d=%v ni=%+v", name, d, ni)
	}
	if sh == nil {
		t.Fatalf("%s: a clean check handed out no registry", name)
	}
	return f, sh
}

// checkForShapesOnly is checkForShapes where the tree is not needed.
func checkForShapesOnly(t *testing.T, name, src string) *Shapes {
	t.Helper()
	_, sh := checkForShapes(t, name, src)
	return sh
}

// declNamed finds one type declaration of the tree by name — the node a
// consumer would match a projected ShapeDecl against.
func declNamed(t *testing.T, f *ast.File, name string) ast.Item {
	t.Helper()
	for _, it := range f.Items {
		switch d := it.(type) {
		case *ast.RecordDecl:
			if d.Name == name {
				return d
			}
		case *ast.SumDecl:
			if d.Name == name {
				return d
			}
		case *ast.NewtypeDecl:
			if d.Name == name {
				return d
			}
		case *ast.InterfaceDecl:
			if d.Name == name {
				return d
			}
		}
	}
	t.Fatalf("the fixture declares no type named %q", name)
	return nil
}

// allSites flattens the registry's site map, in no order.
func allSites(sh *Shapes) []Site {
	out := make([]Site, 0, len(sh.sites))
	for _, s := range sh.sites {
		out = append(out, s)
	}
	return out
}

// sitesWhere returns every site matching the predicate.
func sitesWhere(sh *Shapes, pred func(Site) bool) []Site {
	var out []Site
	for _, s := range allSites(sh) {
		if pred(s) {
			out = append(out, s)
		}
	}
	return out
}

// wrapSiteOf reads one check's Wrap construction site — the application
// the check determined, arguments and all — back out of its registry.
func wrapSiteOf(t *testing.T, f *ast.File, sh *Shapes) Site {
	t.Helper()
	want := declNamed(t, f, "Wrap")
	for _, s := range allSites(sh) {
		if s.Ret.Kind == ShapeNominal && s.Ret.Decl.Node == want && len(s.Ret.Args) == 1 {
			return s
		}
	}
	t.Fatal("no Wrap construction in the registry")
	return Site{}
}

// wrapShapeOf is wrapSiteOf's shape alone.
func wrapShapeOf(t *testing.T, f *ast.File, sh *Shapes) Shape {
	t.Helper()
	return wrapSiteOf(t, f, sh).Ret
}

// TestShapesRegistryMethodCallIsSparse pins the one wrinkle the site shape
// carries: at a method call the receiver's positions are already
// substituted into the view, so the site lists only the positions the call
// itself determined — what the receiver resolved stands in Params and Ret
// instead, and is not a clause binding of this call. The bindings are
// numbered in the method's own clause, which is what the call's arguments
// determine.
func TestShapesRegistryMethodCallIsSparse(t *testing.T) {
	src := "record Age { n: Int64 }\n\ninterface Wrapper<T> {\n    fn pick<U, V>(self, a: U, b: V) -> T\n}\n\nimpl Wrapper<Age> for Age {\n    fn pick<U, V>(self, a: U, b: V) -> Age {\n        return self\n    }\n}\n\nfn probe() -> Int64 {\n    let a = Age { n: 1 }\n    let r = a.pick(2, \"x\")\n    return r.n\n}\n"
	f, sh := checkForShapes(t, "method.we", src)
	call := sitesWhere(sh, func(s Site) bool { return s.Ret.Kind == ShapeNominal && s.Ret.Decl.Node == declNamed(t, f, "Age") })
	if len(call) != 1 {
		t.Fatalf("the method call registered %d sites, want 1: %+v", len(call), call)
	}
	s := call[0]
	if len(s.Args) != 2 {
		t.Fatalf("the call's bindings are %+v, want the method's own two positions", s.Args)
	}
	for i, want := range []string{"Int64", "String"} {
		if s.Args[i].Pos != i || s.Args[i].Shape.Name != want {
			t.Fatalf("binding %d is %+v, want %s at position %d", i, s.Args[i], want, i)
		}
	}
	if len(s.Params) != 2 || s.Params[0].Name != "Int64" || s.Params[1].Name != "String" {
		t.Fatalf("the call's parameter face is %+v, want the determined clause", s.Params)
	}
	// T is not among the bindings: the receiver resolved it, and the
	// resolution stands inline in Ret.
	if s.Ret.Decl.Node != declNamed(t, f, "Age") {
		t.Fatalf("the call's return is %+v; the receiver's position did not substitute inline", s.Ret)
	}
	for _, a := range s.Args {
		if a.Shape.Kind == ShapeNominal && a.Shape.Decl.Node == declNamed(t, f, "Age") {
			t.Fatalf("the receiver's own resolution was recorded as a clause binding: %+v", a)
		}
	}
}
