package codegen

import (
	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// This file is B1b's monomorphization core (design D3): the substitution
// that turns one generic declaration's type parameters into the shapes an
// instantiation bound them to, and the synthesis of the instantiated
// declaration a key names.
//
// The authoritative word on which type arguments an application
// determined is the check stage's (design D1) — the emitter never
// re-infers them. What the emitter owns is naming and layout: a
// declaration applied to arguments is a declaration of its own, with its
// own mangled key (mangle.go), its own layout, and its own define.
//
// Substitution is lazy: nothing rewrites the source tree. The ambient
// instantiation (e.instEnv) is read where a type reference resolves, and
// the references it produces are recorded in e.subst/e.substShape, so a
// key travels with the node it names. Outside an instantiation e.instEnv
// is nil, every substitution is the identity, and the monomorphic path
// reads exactly the tables it always did.

// instCtx is one instantiation in flight: the declaration's type
// parameters and the shapes they are bound to, by position.
type instCtx struct {
	params []string
	args   []typecheck.Shape
}

// bind returns the shape one type-parameter name is bound to.
func (c *instCtx) bind(name string) (typecheck.Shape, bool) {
	for i, p := range c.params {
		if p == name && i < len(c.args) {
			return c.args[i], true
		}
	}
	return typecheck.Shape{}, false
}

// typeParamNames lists a declaration's type parameters in position order.
func typeParamNames(ps []*ast.TypeParam) []string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return names
}

// declTypeParams lists one declaration's type parameters, where the
// declaration is a user one. A builtin declaration carries a name and no
// node, so it has none.
func declTypeParams(d typecheck.ShapeDecl) []*ast.TypeParam {
	switch n := d.Node.(type) {
	case *ast.RecordDecl:
		return n.TypeParams
	case *ast.NewtypeDecl:
		return n.TypeParams
	case *ast.SumDecl:
		return n.TypeParams
	case *ast.InterfaceDecl:
		return n.TypeParams
	}
	return nil
}

// declKindOf names the declaration kind one node is, for the ShapeDecl a
// resolved shape carries.
func declKindOf(node ast.Item) (typecheck.DeclKind, bool) {
	switch node.(type) {
	case *ast.RecordDecl:
		return typecheck.DeclRecord, true
	case *ast.SumDecl:
		return typecheck.DeclSum, true
	case *ast.NewtypeDecl:
		return typecheck.DeclNewtype, true
	case *ast.InterfaceDecl:
		return typecheck.DeclIface, true
	}
	return 0, false
}

// declOfName resolves one written reference's head name to the declaration
// it names inside modKey: the concrete tables first, then the templates —
// a generic declaration names the same instantiation whether its
// application is written at a top level or inside another generic body.
func (e *emitter) declOfName(modKey, name string) (ast.Item, bool) {
	key := modKey + "." + name
	if d := e.records[key]; d != nil {
		return d, true
	}
	if d := e.sumDecls[key]; d != nil {
		return d, true
	}
	if d := e.generics[key]; d != nil {
		return d, true
	}
	if _, ok := e.newtypes[key]; ok {
		if d := e.newtypeDeclOfKey(key); d != nil {
			return d, true
		}
	}
	return nil, false
}

// substRef resolves one type reference under the ambient instantiation: a
// bound type parameter becomes the reference its shape renders, an
// application's arguments substitute recursively, and everything else
// stands as written. The result is memoized per instantiation, so one node
// resolves once and every site in that body sees one node.
func (e *emitter) substRef(t ast.TypeRef) ast.TypeRef {
	if e.instEnv == nil || t == nil {
		return t
	}
	if r, ok := e.instMemo[t]; ok {
		return r
	}
	r := e.substRefNew(t)
	e.instMemo[t] = r
	return r
}

func (e *emitter) substRefNew(t ast.TypeRef) ast.TypeRef {
	switch x := t.(type) {
	case *ast.NamedType:
		if len(x.Args) == 0 {
			if x.Qual != "" {
				// A qualified name names another module's declaration by
				// its own spelling; no type parameter is written qualified.
				return x
			}
			if s, ok := e.instEnv.bind(x.Name); ok {
				return e.refOfShape(s)
			}
			return x
		}
		args := make([]ast.TypeRef, len(x.Args))
		for i, a := range x.Args {
			args[i] = e.substRef(a)
		}
		return e.applyRef(x, args)
	case *ast.TupleType:
		elems := make([]ast.TypeRef, len(x.Elems))
		changed := false
		for i, el := range x.Elems {
			elems[i] = e.substRef(el)
			changed = changed || elems[i] != x.Elems[i]
		}
		if !changed {
			return x
		}
		return &ast.TupleType{Elems: elems, Line: x.Line, Col: x.Col}
	case *ast.FnType:
		params := make([]ast.TypeRef, len(x.Params))
		changed := false
		for i, p := range x.Params {
			params[i] = e.substRef(p)
			changed = changed || params[i] != x.Params[i]
		}
		ret := e.substRef(x.Ret)
		changed = changed || ret != x.Ret
		if !changed {
			return x
		}
		return &ast.FnType{Params: params, Ret: ret, EffectTags: x.EffectTags, Line: x.Line, Col: x.Col}
	}
	return t // UnitType: no position to substitute
}

// applyRef resolves one application whose arguments have already been
// resolved: the head declaration is instantiated at those arguments and
// the reference names the instantiation's key. An application whose head
// the walked program does not declare is left as an application of the
// same name — the prelude and the builtin collections have no declaration
// to instantiate, and a name the program does not declare is the check
// stage's verdict, not this one's to restate.
func (e *emitter) applyRef(x *ast.NamedType, args []ast.TypeRef) ast.TypeRef {
	modKey := e.curKey
	if x.Qual != "" {
		if modKey = e.resolveQual(x.Qual); modKey == "" {
			return x
		}
	}
	node, ok := e.declOfName(modKey, x.Name)
	if !ok {
		return &ast.NamedType{Qual: x.Qual, Name: x.Name, Args: args,
			Line: x.Line, Col: x.Col, ArgLine: x.ArgLine, ArgCol: x.ArgCol}
	}
	shapes := make([]typecheck.Shape, len(args))
	for i, a := range args {
		s, ok := e.shapeOfRef(a)
		if !ok {
			return x
		}
		shapes[i] = s
	}
	return e.refOfShape(typecheck.Shape{
		Kind: typecheck.ShapeNominal,
		Decl: typecheck.ShapeDecl{Node: node},
		Args: shapes,
	})
}

// resolveRef resolves one type reference as far as the emitter's tables
// reach: the ambient instantiation's bindings first (a template's type
// parameters), then every application still written out — a site's own
// `Box<Int64>` names the same instantiation a substituted one does, and
// what comes back is the key the tables hold either way. This is the
// entry every resolution point reads a reference through, so a table
// lookup never sees an open position or an uninstantiated head.
func (e *emitter) resolveRef(t ast.TypeRef) ast.TypeRef {
	s := e.substRef(t)
	switch x := s.(type) {
	case *ast.NamedType:
		if len(x.Args) == 0 {
			return s
		}
		if _, done := e.substShape[x]; done {
			return s // already an instantiation the substitution produced
		}
		args := make([]ast.TypeRef, len(x.Args))
		for i, a := range x.Args {
			args[i] = e.resolveRef(a)
		}
		return e.applyRef(x, args)
	case *ast.TupleType:
		elems := make([]ast.TypeRef, len(x.Elems))
		changed := false
		for i, el := range x.Elems {
			elems[i] = e.resolveRef(el)
			changed = changed || elems[i] != x.Elems[i]
		}
		if !changed {
			return s
		}
		return &ast.TupleType{Elems: elems, Line: x.Line, Col: x.Col}
	case *ast.FnType:
		params := make([]ast.TypeRef, len(x.Params))
		changed := false
		for i, p := range x.Params {
			params[i] = e.resolveRef(p)
			changed = changed || params[i] != x.Params[i]
		}
		ret := e.resolveRef(x.Ret)
		changed = changed || ret != x.Ret
		if !changed {
			return s
		}
		return &ast.FnType{Params: params, Ret: ret, EffectTags: x.EffectTags, Line: x.Line, Col: x.Col}
	}
	return s
}

// newtypeDeclOfKey recovers the declaration node behind one newtype key.
// The table holds the underlying type, not the declaration, so the node
// comes from the index the mangler keys every declaration by.
func (e *emitter) newtypeDeclOfKey(key string) ast.Item {
	for node, k := range e.declKeys {
		if k != key {
			continue
		}
		if _, ok := node.(*ast.NewtypeDecl); ok {
			return node
		}
	}
	return nil
}

// refForName builds one reference a shape rendered and records the shape
// it stands for, so the inverse projection reads a recorded fact rather
// than re-deriving one.
func (e *emitter) refForName(name string, s typecheck.Shape) *ast.NamedType {
	n := &ast.NamedType{Name: name}
	if e.substShape == nil {
		e.substShape = make(map[*ast.NamedType]typecheck.Shape)
	}
	e.substShape[n] = s
	return n
}

// refOfShape renders one shape as a type reference and, for a nominal one,
// registers the key it stands for: the reference's own name cannot carry
// the module the declaration lives in, so the key travels with the node
// (namedKey).
//
// A nominal shape also makes the instantiation it names real — a reference
// to it is a use of it, and the synthesis is where its concrete
// declaration comes from.
func (e *emitter) refOfShape(s typecheck.Shape) ast.TypeRef {
	switch s.Kind {
	case typecheck.ShapeBase:
		return e.refForName(s.Name, s)
	case typecheck.ShapeUnit:
		return &ast.UnitType{}
	case typecheck.ShapeNever:
		return e.refForName("Never", s)
	case typecheck.ShapeTuple:
		elems := make([]ast.TypeRef, len(s.Elems))
		for i, el := range s.Elems {
			elems[i] = e.refOfShape(el)
		}
		return &ast.TupleType{Elems: elems}
	case typecheck.ShapeFn:
		params := make([]ast.TypeRef, len(s.Params))
		for i, p := range s.Params {
			params[i] = e.refOfShape(p)
		}
		var ret ast.TypeRef
		if s.Ret != nil {
			ret = e.refOfShape(*s.Ret)
		}
		return &ast.FnType{Params: params, Ret: ret}
	case typecheck.ShapeNominal, typecheck.ShapeDyn:
		key, ok := e.shapeKey(s)
		if !ok {
			e.instFail("a shape this build cannot name")
			return nil
		}
		n := e.refForName(key, s)
		if e.subst == nil {
			e.subst = make(map[*ast.NamedType]string)
		}
		e.subst[n] = key
		return n
	case typecheck.ShapeParam:
		// A position the enclosing instantiation still has open: the
		// substitution that reached here had no binding for it, which is
		// the driving layer's mistake (design D3), never a name.
		e.instFail("an unresolved type position")
		return nil
	}
	e.instFail("a shape this build cannot name")
	return nil
}

// shapeKey renders one nominal shape's instantiation key — the mangled
// name (design D2) — and makes the instantiation real. A builtin
// declaration has no table to fill (its faces ride the emitter's own
// paths), so only its key is rendered.
func (e *emitter) shapeKey(s typecheck.Shape) (string, bool) {
	if s.Decl.Node == nil {
		if s.Decl.Name == "" {
			return "", false
		}
		return e.mangleApply(e.declKey(s.Decl), s.Args), true
	}
	key, ok := e.declKeys[s.Decl.Node]
	if !ok {
		return "", false
	}
	if len(s.Args) == 0 {
		return key, true
	}
	// The declaration's own key plus the rendered arguments: one key per
	// {declaration, arguments} pair, which is what the tables below are
	// keyed by and what the tables above look references up under.
	return e.instDecl(s.Decl, e.mangleApply(key, s.Args), s.Args)
}

// shapeOfRef is the inverse projection: the shape one type reference
// stands for, by the declaration identity the check stage itself uses — a
// user declaration by its own AST node, a builtin by its name. It is what
// keeps the emitter's mangling on the one table (design D2): a shape built
// here mangles to the symbol a shape from the checker mangles to.
func (e *emitter) shapeOfRef(t ast.TypeRef) (typecheck.Shape, bool) {
	switch x := t.(type) {
	case *ast.NamedType:
		if s, ok := e.substShape[x]; ok {
			// A reference the substitution produced: the shape it stands
			// for was recorded with it, whole.
			return s, true
		}
		args := make([]typecheck.Shape, len(x.Args))
		for i, a := range x.Args {
			s, ok := e.shapeOfRef(a)
			if !ok {
				return typecheck.Shape{}, false
			}
			args[i] = s
		}
		if x.Qual == "" && len(args) == 0 && baseStrKind(x.Name) != skNone {
			return typecheck.Shape{Kind: typecheck.ShapeBase, Name: x.Name}, true
		}
		modKey := e.curKey
		if x.Qual != "" {
			if modKey = e.resolveQual(x.Qual); modKey == "" {
				return typecheck.Shape{}, false
			}
		}
		node, ok := e.declOfName(modKey, x.Name)
		if !ok {
			if x.Qual == "" && len(args) != 0 {
				// A builtin collection: a declaration with a name and no
				// source.
				return typecheck.Shape{Kind: typecheck.ShapeNominal,
					Decl: typecheck.ShapeDecl{Name: x.Name}, Args: args}, true
			}
			return typecheck.Shape{}, false
		}
		kind, ok := declKindOf(node)
		if !ok {
			return typecheck.Shape{}, false
		}
		return typecheck.Shape{Kind: typecheck.ShapeNominal,
			Decl: typecheck.ShapeDecl{Kind: kind, Node: node}, Args: args}, true
	case *ast.TupleType:
		elems := make([]typecheck.Shape, len(x.Elems))
		for i, el := range x.Elems {
			s, ok := e.shapeOfRef(el)
			if !ok {
				return typecheck.Shape{}, false
			}
			elems[i] = s
		}
		return typecheck.Shape{Kind: typecheck.ShapeTuple, Elems: elems}, true
	case *ast.UnitType:
		return typecheck.Shape{Kind: typecheck.ShapeUnit}, true
	case *ast.FnType:
		params := make([]typecheck.Shape, len(x.Params))
		for i, p := range x.Params {
			s, ok := e.shapeOfRef(p)
			if !ok {
				return typecheck.Shape{}, false
			}
			params[i] = s
		}
		var ret *typecheck.Shape
		if x.Ret != nil {
			s, ok := e.shapeOfRef(x.Ret)
			if !ok {
				return typecheck.Shape{}, false
			}
			ret = &s
		}
		return typecheck.Shape{Kind: typecheck.ShapeFn, Params: params, Ret: ret}, true
	}
	return typecheck.Shape{}, false
}

// namedKey renders one unqualified named reference's module-qualified key:
// the module's own key for a name written in the source, and the key the
// substitution recorded for a reference it produced. modKey is the module
// the lookup belongs to — the walked module at a body site, the declaring
// module inside a layout — so a produced reference overrides it and
// everything else keeps the reading it had.
func (e *emitter) namedKey(modKey string, n *ast.NamedType) string {
	if key, ok := e.subst[n]; ok {
		return key
	}
	return modKey + "." + n.Name
}

// newtypeKey names the wrapper one newtype construction denotes, resolving
// an application the same way constructKey does for a record (design D3):
// a generic newtype's construction is an application like any other, and
// the instantiation is what its erasure reads — `Tagged<Int64>(1)` is a
// value of `main.Tagged$Int64`, whose underlying type the table holds.
//
// The constructor's call carries the arguments where they were written and
// the registry's site otherwise, so both spellings resolve here. The
// erasure itself is emitNewtypeCtor's; what this returns is only the key
// the wrapper's underlying type is filed under.
func (e *emitter) newtypeKey(name string, call *ast.Call) (string, bool) {
	key := e.curKey + "." + name
	if d, ok := e.declOfKey(e.curKey, name); ok && len(declTypeParams(d)) != 0 {
		if args, ok := e.siteArgs(call, len(declTypeParams(d))); ok {
			return e.instDecl(d, e.mangleApply(key, args), args)
		}
		if len(call.TypeArgs) == 0 {
			return "", false
		}
		app := &ast.NamedType{Name: name, Args: e.resolveRefs(call.TypeArgs),
			Line: call.Line, Col: call.Col}
		n, is := e.applyRef(app, app.Args).(*ast.NamedType)
		if !is {
			return "", false
		}
		return e.namedKey(e.curKey, n), true
	}
	if _, ok := e.newtypes[key]; !ok {
		return "", false
	}
	return key, true
}

// constructKey names the record one construction denotes, resolving an
// explicit application to its instantiation (design D3) and asking nothing
// beyond the declaration tables. It emits nothing — the synthesis an
// application triggers is a declaration, not a definition — which is what
// lets the static classifiers (a receiver's key, a list element's face)
// read a construction through the same resolution the emitter uses.
//
// A construction naming no declared record reports false: an unapplied
// generic head is a template, not a type, and a name no module holds is
// the check stage's verdict to restate.
func (e *emitter) constructKey(c *ast.Construct) (string, bool) {
	key := e.curKey
	if c.Qual != "" {
		if key = e.resolveQual(c.Qual); key == "" {
			return "", false
		}
	}
	head := key + "." + c.Name
	// An application of a generic declaration — written out or left to
	// inference, both are one site to the check stage (design D1), so
	// both take the same path here. A declaration the head does not name
	// (an ordinary record, a builtin collection) has no site and keeps
	// the plain key it always had.
	if d, ok := e.declOfKey(key, c.Name); ok && len(declTypeParams(d)) != 0 {
		args, ok := e.siteArgs(c, len(declTypeParams(d)))
		if !ok {
			if len(c.TypeArgs) == 0 {
				return "", false
			}
			// No registry reached this emitter (a hand-built one): the
			// written references are the only word on the arguments there
			// is, and they are the same word the check would have given.
			app := &ast.NamedType{Qual: c.Qual, Name: c.Name, Args: e.resolveRefs(c.TypeArgs), Line: c.Line, Col: c.Col}
			n, is := e.applyRef(app, app.Args).(*ast.NamedType)
			if !is {
				return "", false
			}
			head = e.namedKey(key, n)
		} else {
			k, ok := e.instDecl(d, e.mangleApply(head, args), args)
			if !ok {
				return "", false
			}
			head = k
		}
	}
	rec, ok := e.records[head]
	if !ok || len(rec.TypeParams) != 0 {
		return "", false
	}
	return key + "." + rec.Name, true
}

// resolveRefs maps a written reference list through the substitution in
// one step — the shape a construction's own type-argument list takes
// before it is resolved as an application.
func (e *emitter) resolveRefs(ts []ast.TypeRef) []ast.TypeRef {
	out := make([]ast.TypeRef, len(ts))
	for i, t := range ts {
		out[i] = e.resolveRef(t)
	}
	return out
}

// declOfKey is the ShapeDecl one named declaration of a module stands for
// — the node the tables hold, under the kind the check's own projection
// gives it. A name no declaration carries (a builtin collection, an
// import alias that resolved to nothing) has none.
func (e *emitter) declOfKey(modKey, name string) (typecheck.ShapeDecl, bool) {
	node, ok := e.declOfName(modKey, name)
	if !ok {
		return typecheck.ShapeDecl{}, false
	}
	kind, ok := declKindOf(node)
	if !ok {
		return typecheck.ShapeDecl{}, false
	}
	return typecheck.ShapeDecl{Kind: kind, Node: node, Name: name}, true
}

// genericFn returns the template one key names, where the program declares
// a generic fn under it. A fn is not a type, so it carries no Shape and
// never reaches the mangler: what an instantiation names is the symbol the
// fn table is keyed by.
func (e *emitter) genericFn(key string) (*ast.FnDecl, bool) {
	d, ok := e.generics[key].(*ast.FnDecl)
	return d, ok
}

// siteOf reads the check stage's verdict at one application node (B1b
// T4). The registries are small (one per check) and the nodes unique
// across them, so the scan is a handful of probes and the answer does not
// depend on which one holds the key.
func (e *emitter) siteOf(n ast.Expr) (typecheck.Site, bool) {
	if n == nil {
		return typecheck.Site{}, false
	}
	for _, reg := range e.shapes {
		if site, ok := reg.At(n); ok {
			return site, true
		}
	}
	return typecheck.Site{}, false
}

// siteArgs turns one site's clause bindings into the argument list an
// instantiation takes, in the declaration's own parameter order. A site
// that names fewer positions than the declaration declares is not a
// determined application — the caller stops at the boundary rather than
// filling the gap with a guess, which is the one thing D1 forbids.
func (e *emitter) siteArgs(n ast.Expr, params int) ([]typecheck.Shape, bool) {
	return e.siteArgsFrom(n, 0, params)
}

// siteArgsFrom reads one site's bindings with the clause's leading
// positions skipped. A method's site is recorded against the *whole*
// clause the receiver's view holds — its head's parameters occupy the
// first positions, in declaration order — so a generic method's own
// arguments begin after them. A free fn has no head, and the offset is
// zero. Positions in neither range are not this application's, and a
// position the site left unfilled is not a determined application — the
// caller stops at the boundary rather than filling the gap with a guess,
// which is the one thing D1 forbids.
func (e *emitter) siteArgsFrom(n ast.Expr, off, params int) ([]typecheck.Shape, bool) {
	site, ok := e.siteOf(n)
	if !ok {
		return nil, false
	}
	args := make([]typecheck.Shape, params)
	filled := make([]bool, params)
	for _, a := range site.Args {
		p := a.Pos - off
		if p < 0 || p >= params {
			return nil, false
		}
		args[p] = a.Shape
		filled[p] = true
	}
	for _, ok := range filled {
		if !ok {
			return nil, false
		}
	}
	return args, true
}

// instCallee resolves one call's callee to the definition to emit: a
// program fn's own def, or — for a generic fn — the instantiation the
// call applies it at.
//
// The arguments come from the check stage's site when it recorded one
// (design D1): the call's own node is the key, and what it carries covers
// both spellings — a call that wrote its type arguments and a call that
// left them to inference resolve to the same site, so reading it is one
// path rather than two. The written references answer only where no
// registry reached the emitter at all (the hand-built emitters the
// package's own tests drive); a call with neither is not a determined
// application and stops at the generic boundary rather than guessing.
func (e *emitter) instCallee(modKey, name string, call *ast.Call) (*fnDef, bool, *NotImplemented) {
	key := modKey + "." + name
	if fd, ok := e.fnTable[key]; ok {
		return fd, true, nil
	}
	tmpl, ok := e.genericFn(key)
	if !ok {
		return nil, false, nil
	}
	if args, ok := e.siteArgs(call, len(tmpl.TypeParams)); ok {
		return e.instFn(modKey, name, tmpl, args), true, nil
	}
	if len(call.TypeArgs) == 0 {
		return nil, true, bndGeneric()
	}
	args := make([]typecheck.Shape, len(call.TypeArgs))
	for i, t := range call.TypeArgs {
		s, ok := e.shapeOfRef(e.resolveRef(t))
		if !ok {
			return nil, true, bndGeneric()
		}
		args[i] = s
	}
	return e.instFn(modKey, name, tmpl, args), true, nil
}

// instFn registers one generic fn's instantiation and returns its def: the
// template, under the symbol its arguments mangle to, with the arguments
// kept so that the define, the call sites, and the callee's own module
// resolve one instantiation. The def joins the fn list at its registration
// — the order the defines emit in is the registration order (design D3),
// so a fn a body registers emits after that body's own define.
func (e *emitter) instFn(modKey, name string, d *ast.FnDecl, args []typecheck.Shape) *fnDef {
	key := modKey + "." + name
	suffix := e.mangleSuffix(args)
	// The guard belongs here as much as in instDecl: an argument that
	// deepens its own application without passing through a declaration —
	// a tuple the substitution keeps wrapping — registers a fresh symbol
	// every round, and nothing downstream would ever meet the same name
	// twice to stop on.
	e.instDepthCheck(len(d.TypeParams), key+suffix, args)
	sym := e.insts.register(key, suffix, args)
	if fd, ok := e.fnTable[sym]; ok {
		return fd
	}
	fd := &fnDef{key: modKey, name: name, decl: d, suffix: suffix, instArgs: args}
	e.fns = append(e.fns, *fd)
	e.fnTable[sym] = fd
	return fd
}

// enterFnInst installs fd's own instantiation for as long as the caller
// works on fd: the template's type parameters, bound to the arguments the
// site supplied. It returns the restore. A monomorphic fn installs
// nothing, and the substitution stays the identity.
func (e *emitter) enterFnInst(fd *fnDef) func() {
	savedInst, savedMemo := e.instEnv, e.instMemo
	if len(fd.instArgs) != 0 {
		e.instEnv = &instCtx{params: fd.instParamNames(), args: fd.instArgs}
		e.instMemo = make(map[ast.TypeRef]ast.TypeRef)
	}
	return func() { e.instEnv, e.instMemo = savedInst, savedMemo }
}

// instParamNames lists the type-parameter names a def's instantiation
// binds: what the def carries where the head supplied it, and the
// declaration's own otherwise.
func (fd *fnDef) instParamNames() []string {
	if len(fd.instParams) != 0 {
		return fd.instParams
	}
	return typeParamNames(fd.decl.TypeParams)
}

// instDecl instantiates one generic declaration at the given arguments and
// returns its key. The declaration kinds differ only in what the
// instantiation fills; the key, the registration, and the guard are
// shared.
func (e *emitter) instDecl(d typecheck.ShapeDecl, key string, args []typecheck.Shape) (string, bool) {
	e.instDepthCheck(len(declTypeParams(d)), key, args)
	// An impl over this declaration's template emits per instantiation of
	// it: the methods land under the key just synthesized, carrying the
	// arguments the head was instantiated at, so a receiver of this
	// instantiation's type resolves them off its own key. The
	// registration runs on the way in — before the fields substitute —
	// for the same reason the key is: a method a field's type reaches
	// must already be there, not registered twice.
	e.registerImpls(e.declKeys[d.Node], key, args)
	switch node := d.Node.(type) {
	case *ast.RecordDecl:
		return e.instRecord(key, node, args)
	case *ast.NewtypeDecl:
		return e.instNewtype(key, node, args)
	case *ast.SumDecl:
		return e.instSum(key, node, args)
	}
	// An interface instantiation emits no layout of its own: a boxed
	// value's face is read from the declaration, never from a table.
	return key, true
}

// instMethod instantiates one generic method at the arguments the call
// site determined, and returns its def — the method table's half of what
// instCallee does for the fn table. The receiver names the head; the
// method's own template is what the key holds, and the site is what
// completes it. Three answers, the same three instCallee gives: a def,
// "no such method here" for the caller's other faces, and a boundary for
// a method this is that the site left undetermined — a template the call
// does not pin is not an application this stage may guess at (design D1).
func (e *emitter) instMethod(recvKey, name string, call *ast.Call) (*fnDef, bool, *NotImplemented) {
	tmpl, ok := e.genericMethods[recvKey+"."+name]
	if !ok {
		return nil, false, nil
	}
	args, ok := e.siteArgsFrom(call, len(tmpl.headParams), len(tmpl.decl.TypeParams))
	if !ok {
		return nil, true, bndGeneric()
	}
	return e.instMethodAt(tmpl, args), true, nil
}

// instMethodAt registers one generic method's instantiation at the given
// arguments and returns its def, memoized on the suffixed key so two
// sites that determined the same arguments share one define.
func (e *emitter) instMethodAt(tmpl *methodTmpl, args []typecheck.Shape) *fnDef {
	suffix := e.mangleSuffix(args)
	key := tmpl.headKey + "." + tmpl.decl.Name + suffix
	if fd, ok := e.methods[key]; ok {
		return fd
	}
	e.instDepthCheck(len(tmpl.decl.TypeParams), tmpl.headKey+"."+tmpl.decl.Name, args)
	fd := &fnDef{key: tmpl.modKey, name: tmpl.decl.Name, recvKey: tmpl.headKey,
		decl: tmpl.decl, suffix: suffix, instArgs: args}
	if len(tmpl.headArgs) != 0 {
		// The head's instantiation is the outer scope: it binds first, and
		// the method's own arguments follow in declaration order.
		fd.instArgs = append(append([]typecheck.Shape{}, tmpl.headArgs...), args...)
		fd.instParams = append(append([]string{}, tmpl.headParams...), typeParamNames(tmpl.decl.TypeParams)...)
	}
	e.methods[key] = fd
	e.methodsOrd = append(e.methodsOrd, key)
	return fd
}

// instRecord synthesizes one generic record's instantiation: a concrete
// declaration of its own, under the instantiation's key, whose field types
// are the declaration's with the parameters substituted. Nothing
// downstream needs to know the record was generic — the layout, the
// construction, and the field walks read the synthesized declaration
// exactly as they read a written one.
//
// The key is registered before the fields substitute, so a declaration
// that reaches its own instantiation through a field (a recursive
// container) terminates on the registration instead of recursing.
func (e *emitter) instRecord(key string, d *ast.RecordDecl, args []typecheck.Shape) (string, bool) {
	if _, ok := e.records[key]; ok {
		return key, true
	}
	rec := &ast.RecordDecl{Cat: d.Cat, Opaque: d.Opaque,
		Name: e.instLocalName(d.Name, args),
		Line: d.Line, Col: d.Col, NameLine: d.NameLine, NameCol: d.NameCol}
	e.records[key] = rec

	savedInst, savedMemo := e.instEnv, e.instMemo
	e.instEnv = &instCtx{params: typeParamNames(d.TypeParams), args: args}
	e.instMemo = make(map[ast.TypeRef]ast.TypeRef)
	for _, f := range d.Fields {
		typ := e.substRef(f.Typ)
		if typ == nil {
			e.instFail("a record field the substitution could not name")
			return "", false
		}
		rec.Fields = append(rec.Fields, ast.FieldDecl{Name: f.Name, Typ: typ, Line: f.Line, Col: f.Col})
	}
	e.instEnv, e.instMemo = savedInst, savedMemo

	// The mangler and the shape projection both identify a declaration by
	// its node, so the instantiation is indexed under the key its
	// arguments render.
	e.declKeys[rec] = key
	e.order = append(e.order, recRef{name: key, decl: rec})
	return key, true
}

// instNewtype synthesizes one generic newtype's instantiation: the wrapper
// erases (design D4), so what the table needs is the substituted
// underlying type and nothing else.
func (e *emitter) instNewtype(key string, d *ast.NewtypeDecl, args []typecheck.Shape) (string, bool) {
	if _, ok := e.newtypes[key]; ok {
		return key, true
	}
	savedInst, savedMemo := e.instEnv, e.instMemo
	e.instEnv = &instCtx{params: typeParamNames(d.TypeParams), args: args}
	e.instMemo = make(map[ast.TypeRef]ast.TypeRef)
	under := e.substRef(d.Underlying)
	e.instEnv, e.instMemo = savedInst, savedMemo
	if under == nil {
		e.instFail("a newtype underlying type the substitution could not name")
		return "", false
	}
	e.newtypes[key] = under
	e.declKeys[d] = key
	return key, true
}

// instSum synthesizes one generic sum's instantiation: the variant table,
// the variant order, and the declaration the Eq question reads — all three
// re-keyed under the instantiation and carrying substituted payloads.
func (e *emitter) instSum(key string, d *ast.SumDecl, args []typecheck.Shape) (string, bool) {
	if _, ok := e.sums[key]; ok {
		return key, true
	}
	variants := make(map[string][]ast.TypeRef, len(d.Variants))
	names := make([]string, len(d.Variants))
	clone := &ast.SumDecl{Name: e.instLocalName(d.Name, args), Line: d.Line, Col: d.Col}
	e.sums[key] = variants
	e.sumsOrd[key] = names
	e.sumDecls[key] = clone

	savedInst, savedMemo := e.instEnv, e.instMemo
	e.instEnv = &instCtx{params: typeParamNames(d.TypeParams), args: args}
	e.instMemo = make(map[ast.TypeRef]ast.TypeRef)
	for i, v := range d.Variants {
		names[i] = v.Name
		payload := make([]ast.TypeRef, len(v.Payload))
		for j, p := range v.Payload {
			payload[j] = e.substRef(p)
			if payload[j] == nil {
				e.instFail("a variant payload the substitution could not name")
				return "", false
			}
		}
		variants[v.Name] = payload
		clone.Variants = append(clone.Variants, ast.Variant{Name: v.Name, Payload: payload, Line: v.Line, Col: v.Col})
	}
	e.instEnv, e.instMemo = savedInst, savedMemo

	e.declKeys[clone] = key
	return key, true
}

// instLocalName renders an instantiation's own declaration name: the
// declaration's name with the suffix its arguments mangle to. The
// module-qualified key is this name under the declaring module's key —
// which is what keeps the lookups that split a key at its last separator
// (recModKey) working unchanged.
func (e *emitter) instLocalName(name string, args []typecheck.Shape) string {
	return name + e.mangleSuffix(args)
}

// instFail reports one instantiation this build cannot carry. It is an
// internal error, not a boundary: every path that reaches it is a
// substitution the driving layer should not have produced — a shape with
// no name, or one whose declaration stands outside the program.
func (e *emitter) instFail(what string) {
	panic("codegen: " + what)
}

// instDepthLimit is the constant design D3's upper-bound guard adds to a
// declaration's type-parameter count. A legitimate instantiation's
// argument depth is bounded by the nesting the source writes; only
// inference that deepens its own application — `f<Box<T>>` reaching
// `f<T>` — grows without bound, and it is stopped here rather than
// looping forever.
const instDepthLimit = 32

// instDepthCheck enforces design D3's hard requirement: an argument deeper
// than the declaration's own type-parameter count plus the limit is an
// internal error rather than an unbounded walk.
func (e *emitter) instDepthCheck(params int, key string, args []typecheck.Shape) {
	limit := params + instDepthLimit
	for _, a := range args {
		if e.shapeDepth(a) > limit {
			panic("codegen: instantiation " + key + " exceeds the type argument depth limit")
		}
	}
}

// shapeDepth is the nesting depth of one shape: a bare name is zero and
// every application adds one, so the guard compares the depth of what
// inference produced against the depth the source could have written.
func (e *emitter) shapeDepth(s typecheck.Shape) int {
	d := 0
	switch s.Kind {
	case typecheck.ShapeNominal, typecheck.ShapeDyn:
		for _, a := range s.Args {
			if ad := e.shapeDepth(a); ad > d {
				d = ad
			}
		}
		return d + 1
	case typecheck.ShapeTuple:
		for _, el := range s.Elems {
			if ed := e.shapeDepth(el); ed > d {
				d = ed
			}
		}
		return d + 1
	case typecheck.ShapeFn:
		for _, p := range s.Params {
			if pd := e.shapeDepth(p); pd > d {
				d = pd
			}
		}
		if s.Ret != nil {
			if rd := e.shapeDepth(*s.Ret); rd > d {
				d = rd
			}
		}
		return d + 1
	}
	return 0
}
