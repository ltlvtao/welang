package typecheck

import "github.com/ltlvtao/welang/internal/ast"

// This file is the checker's export face for the code stage (design D1,
// Q2). The emitter monomorphizes from the checker's own inference — which
// type arguments a call determined, which concrete type a box carries —
// and the checker throws that away when it returns. Re-deriving it in the
// emitter would make a second authority for one fact, so a check hands
// its caller this registry instead.
//
// The view is closed and small, and it carries no rendering: a Shape is
// data. Naming a declaration is codegen's authority (D1's rejected
// alternative) — two modules may declare the same name and only codegen
// holds the module key that tells them apart, so a name string here would
// collide exactly where it matters.

// ShapeKind names one arm of the closed Shape view.
type ShapeKind uint8

const (
	// ShapeParam is a type-parameter position of some generic
	// declaration, unresolved at this point. In a site's binding it is
	// the enclosing declaration's own position (a call inside fn f<T>
	// binds T); in a face's slot it is the interface's.
	ShapeParam ShapeKind = iota
	// ShapeBase is one of chapter 7's base types, by name.
	ShapeBase
	// ShapeNominal is a declared nominal type applied to its arguments:
	// records, sums, newtypes, interfaces, and the builtin collections
	// (List, Map, Set, Option, Result, Range, the chapter 18 cells).
	ShapeNominal
	// ShapeDyn is the erased box's face (chapter 10): the one interface
	// it carries, applied to its arguments.
	ShapeDyn
	// ShapeFn is a fn type: Params and Ret. A We fn has one return, so a
	// tuple return is a ShapeTuple in Ret; a fn declaring none carries a
	// nil Ret, the same reading a face's valueless slot takes.
	ShapeFn
	// ShapeTuple is a tuple type: Elems, in order.
	ShapeTuple
	// ShapeUnit is the unit type, written ().
	ShapeUnit
	// ShapeNever is chapter 9's bottom type.
	ShapeNever
	// ShapeAssoc is an interface's associated-type position (chapter 10),
	// unresolved until an impl binds it: Iterable<T>'s iterator returns
	// Iter. Only the declaration face's slots carry it.
	ShapeAssoc
)

// DeclKind names what one ShapeDecl identifies.
type DeclKind uint8

const (
	DeclRecord DeclKind = iota
	DeclSum
	DeclNewtype
	DeclIface
)

// ShapeDecl identifies one declaration. Exactly one of Node and Name is
// set: a user declaration by its own AST node — the tree both stages walk,
// never an annotation on it — and a builtin one by its name. The builtin
// declarations are package-level singletons with no source file, so a bare
// name cannot collide across modules there; the same name in two modules
// is two user declarations, which the node tells apart.
type ShapeDecl struct {
	Kind DeclKind
	Node ast.Item
	Name string
}

// Shape is one type, closed over the forms codegen must act on. It has no
// String method on purpose: this face hands out no rendering, because a
// declaration's name alone does not identify it.
type Shape struct {
	Kind   ShapeKind
	Pos    int       // ShapeParam: the type-parameter position
	Name   string    // ShapeBase: the base type's name; ShapeAssoc: the associated type's
	Decl   ShapeDecl // ShapeNominal, ShapeDyn, ShapeAssoc: the declaration
	Args   []Shape   // ShapeNominal, ShapeDyn: the application's arguments
	Params []Shape   // ShapeFn: the parameter faces
	Ret    *Shape    // ShapeFn: the one return; nil where the fn declares none
	Elems  []Shape   // ShapeTuple: the elements, in order
}

// Equal reports whether two shapes are the same type by the checker's own
// identity rules: base types by name, nominal and boxed types by
// declaration identity — so two modules' same-named Wrap<T> are never
// equal — and arguments recursively. It is what an instantiation table
// keys on, in place of a rendering string.
func (s Shape) Equal(o Shape) bool {
	if s.Kind != o.Kind {
		return false
	}
	switch s.Kind {
	case ShapeParam:
		return s.Pos == o.Pos
	case ShapeBase:
		return s.Name == o.Name
	case ShapeNominal, ShapeDyn:
		return s.Decl == o.Decl && shapesEqual(s.Args, o.Args)
	case ShapeAssoc:
		return s.Decl == o.Decl && s.Name == o.Name
	case ShapeFn:
		if (s.Ret == nil) != (o.Ret == nil) {
			return false
		}
		return shapesEqual(s.Params, o.Params) && (s.Ret == nil || s.Ret.Equal(*o.Ret))
	case ShapeTuple:
		return shapesEqual(s.Elems, o.Elems)
	}
	return true // Unit, Never: no payload
}

func shapesEqual(a, b []Shape) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}

// Arg is one clause binding: the shape a site bound to a declaration's
// type parameter, at that parameter's own position.
type Arg struct {
	Pos   int
	Shape Shape
}

// Site is what one application site resolved to (design D1). A site is
// registered only where the check applied a generic declaration — an
// ordinary monomorphic call binds nothing and is never a key.
type Site struct {
	// Args are the clause bindings the site determined, by position, in
	// the callee's own clause. At a method call the view arrives with the
	// receiver's positions already substituted, so what the receiver
	// resolved is no binding of this call: it stands inline in Params and
	// Ret instead. Empty where the site determined no position of its
	// own.
	Args []Arg
	// Params are the callee's parameter faces after substitution; nil
	// where the site applies no call (a construction, a Dyn box).
	Params []Shape
	// Ret is the site's own shape: what the application produces. A
	// valueless callee reads Unit.
	Ret Shape
	// Boxed is the concrete type a Dyn construction boxes (design D5);
	// nil at every other site.
	Boxed *Shape
}

// Slot is one interface method with no default body: a vtable slot
// (design D6 decision 2), carrying the signature the box dispatches on.
type Slot struct {
	Name   string
	Params []Shape
	Ret    *Shape // nil = the method is valueless
}

// Face is one interface declaration's dispatch face. It is the reason
// this export exists on the declaration side too: the builtin faces have
// no source file — Iterator<T>'s method set lives here and nowhere else,
// and which of its methods carry a default body (the intrinsic marker) is
// the checker's knowledge alone — so the emitter reads the slots from
// here, never from a tree it does not have.
type Face struct {
	Decl  ShapeDecl
	Slots []Slot
}

// Shapes is the read-only instantiation registry one check hands its
// caller. Every key is a node of the tree the check walked, so a caller
// holding that same tree holds the same keys.
type Shapes struct {
	sites map[ast.Expr]Site
	faces []Face
}

// At returns the resolution of one application site — the call or
// construction node itself — or false where the node is not a site (an
// ordinary monomorphic call, a node of another tree).
func (s *Shapes) At(n ast.Expr) (Site, bool) {
	if s == nil || n == nil {
		return Site{}, false
	}
	site, ok := s.sites[n]
	return site, ok
}

// Faces returns every interface's dispatch face: the builtin faces first
// (a fixed order), then the module interfaces in the order the check
// walked them.
func (s *Shapes) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.faces
}

// newShapes builds one check's registry: the builtin faces are read from
// the package singletons here rather than in a package-level var, because
// the combinator registration fills them in an init and a var would race
// that ordering. The compiler-attached markers (Shareable, the derive
// targets' faces) are declarations too, but their faces are empty by
// construction; Shareable in particular is an implicit bound, never a
// declared interface, so it is no face.
func newShapes() *Shapes {
	builtins := []*ifaceInfo{eqIface, hashIface, showIface, iteratorIface, iterableIface, releasableIface}
	faces := make([]Face, 0, len(builtins))
	for _, inf := range builtins {
		faces = append(faces, faceOf(ifaceDeclOf(inf), inf))
	}
	return &Shapes{sites: map[ast.Expr]Site{}, faces: faces}
}

// noteApply records one application site: the clause bindings the check
// determined and the callee's faces after substitution. A site whose
// bindings are empty applied no generic declaration and is not recorded —
// that is what keeps the registry to the instantiations codegen must
// emit, not every call in the program.
func (c *checker) noteApply(n ast.Expr, args []Arg, params []Type, ret Type) {
	if c.shapes == nil || len(args) == 0 {
		return
	}
	c.shapes.sites[n] = Site{Args: args, Params: shapesOf(params), Ret: shapeOf(ret)}
}

// noteBox records one Dyn construction's site (design D5): the concrete
// type the argument boxes, and the erased face the construction produces.
func (c *checker) noteBox(n ast.Expr, boxed, face Type) {
	if c.shapes == nil {
		return
	}
	b := shapeOf(boxed)
	c.shapes.sites[n] = Site{Boxed: &b, Ret: shapeOf(face)}
}

// noteFace appends one module interface's dispatch face, in walk order.
func (c *checker) noteFace(inf *ifaceInfo) {
	if c.shapes == nil {
		return
	}
	c.shapes.faces = append(c.shapes.faces, faceOf(ifaceDeclOf(inf), inf))
}

// faceOf reads one interface's slots: its methods with no default body,
// in declaration order. The criterion is the one the emitter's own method
// walk states (a nil body is an obligation), and the builtin combinator
// registration marks its compiler-intrinsic methods with an empty block
// rather than a nil one, so only the non-defaulted methods are slots.
func faceOf(d ShapeDecl, inf *ifaceInfo) Face {
	f := Face{Decl: d}
	for i := range inf.methods {
		im := &inf.methods[i]
		if im.body != nil {
			continue
		}
		f.Slots = append(f.Slots, Slot{Name: im.name, Params: shapesOf(im.params), Ret: retShape(im.ret)})
	}
	return f
}

func retShape(t Type) *Shape {
	if t == nil {
		return nil
	}
	s := shapeOf(t)
	return &s
}

// bindArgs projects a resolved clause list onto its bindings, position by
// position: index i is the declaration's i-th type parameter.
func bindArgs(args []Type) []Arg {
	if len(args) == 0 {
		return nil
	}
	bs := make([]Arg, len(args))
	for i, a := range args {
		bs[i] = Arg{Pos: i, Shape: shapeOf(a)}
	}
	return bs
}

// shapeOf projects one resolved type onto the closed view. A position or
// an associated type projects as itself: it is unresolved here, and
// saying so is the honest projection — only the enclosing instantiation
// or an impl fills it in.
func shapeOf(t Type) Shape {
	switch x := t.(type) {
	case nil:
		// A face with no written return (a valueless method) is unit at
		// the type level, the reading retOrUnit states; the positions
		// that must keep the distinction carry a nil Ret instead.
		return Shape{Kind: ShapeUnit}
	case baseType:
		return Shape{Kind: ShapeBase, Name: string(x)}
	case unitType:
		return Shape{Kind: ShapeUnit}
	case neverType:
		return Shape{Kind: ShapeNever}
	case paramRef:
		return Shape{Kind: ShapeParam, Pos: x.idx}
	case tupleType:
		return Shape{Kind: ShapeTuple, Elems: shapesOf(x.elems)}
	case fnType:
		return Shape{Kind: ShapeFn, Params: shapesOf(x.params), Ret: retShape(x.ret)}
	case namedType:
		return Shape{Kind: ShapeNominal, Decl: sumDeclOf(x.decl), Args: shapesOf(x.args)}
	case recordType:
		return Shape{Kind: ShapeNominal, Decl: recordDeclOf(x.decl), Args: shapesOf(x.args)}
	case newtypeType:
		return Shape{Kind: ShapeNominal, Decl: newtypeDeclOf(x.decl), Args: shapesOf(x.args)}
	case ifaceType:
		return Shape{Kind: ShapeNominal, Decl: ifaceDeclOf(x.decl), Args: shapesOf(x.args)}
	case dynType:
		return Shape{Kind: ShapeDyn, Decl: ifaceDeclOf(x.inf), Args: shapesOf(x.args)}
	case assocRef:
		return Shape{Kind: ShapeAssoc, Decl: ifaceDeclOf(x.iface), Name: x.name}
	}
	panic("unreachable type shape")
}

func shapesOf(ts []Type) []Shape {
	if len(ts) == 0 {
		return nil
	}
	out := make([]Shape, len(ts))
	for i, t := range ts {
		out[i] = shapeOf(t)
	}
	return out
}

// The four declaration identities. A user declaration carries its node —
// the checker holds it where it builds the entry, and a side table
// records it without the AST ever hearing about it. A builtin declaration
// carries its name instead: those entries are package singletons, so the
// name is collision-free among them, and a name-shaped identity cannot be
// mistaken for a user declaration's (which always carries a node).

func sumDeclOf(d *sumInfo) ShapeDecl {
	if d == nil {
		return ShapeDecl{Kind: DeclSum}
	}
	if d.node != nil {
		return ShapeDecl{Kind: DeclSum, Node: d.node}
	}
	return ShapeDecl{Kind: DeclSum, Name: d.name}
}

func recordDeclOf(d *recordInfo) ShapeDecl {
	if d == nil {
		return ShapeDecl{Kind: DeclRecord}
	}
	if d.node != nil {
		return ShapeDecl{Kind: DeclRecord, Node: d.node}
	}
	return ShapeDecl{Kind: DeclRecord, Name: d.name}
}

func newtypeDeclOf(d *newtypeInfo) ShapeDecl {
	if d == nil {
		return ShapeDecl{Kind: DeclNewtype}
	}
	if d.node != nil {
		return ShapeDecl{Kind: DeclNewtype, Node: d.node}
	}
	return ShapeDecl{Kind: DeclNewtype, Name: d.name}
}

func ifaceDeclOf(d *ifaceInfo) ShapeDecl {
	if d == nil {
		return ShapeDecl{Kind: DeclIface}
	}
	if d.node != nil {
		return ShapeDecl{Kind: DeclIface, Node: d.node}
	}
	return ShapeDecl{Kind: DeclIface, Name: d.name}
}
