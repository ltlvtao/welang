package parser

import (
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M6a (generics-iterables-collections) parser tests: chapter 10 declaration
// productions, receiver parameters, generic/where/derives clauses, the for
// statement and its head patterns, list literals, and the postfix angle-
// bracket lookahead for explicit type arguments (design D1/D2). Structural
// cases pin the tree shapes T3/T4 must produce; diagnostic cases mirror the
// T1 golden table's parse-level codes (E0801–E0804, E0806, E0824, E0825)
// plus the existing-behavior locks the lookahead must preserve.

// --- D2: interface declarations ----------------------------------------------

func TestInterfaceDecls(t *testing.T) {
	f := wantClean(t, "interface Container<T> {\n    type Item\n    fn item(self) -> Item\n    fn describe(self) -> String {\n        \"c\"\n    }\n}\n")
	d, ok := f.Items[0].(*ast.InterfaceDecl)
	if !ok {
		t.Fatalf("InterfaceDecl wanted, got %T", f.Items[0])
	}
	if d.Name != "Container" {
		t.Fatalf("name Container wanted, got %s", d.Name)
	}
	if len(d.TypeParams) != 1 || d.TypeParams[0].Name != "T" {
		t.Fatalf("type params wanted [T], got %+v", d.TypeParams)
	}
	if len(d.Assocs) != 1 || d.Assocs[0].Name != "Item" {
		t.Fatalf("assocs wanted [Item], got %+v", d.Assocs)
	}
	if len(d.Methods) != 2 {
		t.Fatalf("two methods wanted, got %d", len(d.Methods))
	}
	// The receiver is its own field (design D1), not a Params entry.
	sig := d.Methods[0]
	if sig.Name != "item" || sig.Recv != ast.RecvSelf || sig.Body != nil {
		t.Fatalf("signature method shape wrong: %+v", sig)
	}
	if len(sig.Params) != 0 {
		t.Fatalf("the receiver is not a parameter: wanted no params, got %+v", sig.Params)
	}
	if ret, ok := sig.Ret.(*ast.NamedType); !ok || ret.Name != "Item" {
		t.Fatalf("return Item wanted, got %#v", sig.Ret)
	}
	def := d.Methods[1]
	if def.Name != "describe" || def.Body == nil {
		t.Fatalf("default method wanted a body, got %+v", def)
	}
	if len(def.Body.Items) != 1 {
		t.Fatalf("one-item default body wanted, got %d", len(def.Body.Items))
	}

	// pub interface is a ratified form (design D2: the pub dispatch gains
	// the row; the current E0105 is an unshipped permutation, not a rule).
	pf := wantClean(t, "pub interface Describable {\n    fn describe(self) -> String\n}\n")
	if pd := pf.Items[0].(*ast.InterfaceDecl); !pd.Pub || pd.Name != "Describable" {
		t.Fatalf("pub interface wanted, got %+v", pd)
	}

	// A generic method clause sits between the name and the parameter list.
	mf := wantClean(t, "interface Maker {\n    fn pick<T>(self) -> T\n}\n")
	ms := mf.Items[0].(*ast.InterfaceDecl).Methods[0]
	if len(ms.TypeParams) != 1 || ms.TypeParams[0].Name != "T" {
		t.Fatalf("method clause <T> wanted, got %+v", ms.TypeParams)
	}
	if ms.Recv != ast.RecvSelf {
		t.Fatalf("self receiver wanted, got %q", ms.Recv)
	}

	// mut self receivers parse on both signature and default forms.
	bf := wantClean(t, "interface Both {\n    fn a(self) -> Int64\n    fn b(mut self) -> Int64\n}\n")
	bd := bf.Items[0].(*ast.InterfaceDecl)
	if bd.Methods[0].Recv != ast.RecvSelf || bd.Methods[1].Recv != ast.RecvMutSelf {
		t.Fatalf("self/mut self wanted, got %q/%q", bd.Methods[0].Recv, bd.Methods[1].Recv)
	}

	// Interface body item shapes the chapter does not ratify.
	wantDiag(t, "interface Bad {\n    name: String\n}\n",
		"E0105", "fits no interface item production", 2, 5)
	wantDiag(t, "interface Boxed<T> where T: Eq {\n    fn unwrap(self) -> T\n}\n",
		"E0105", `"where" fits no production here`, 1, 20)
	wantDiag(t, "interface Bad {\n    fn Describe(self) -> String\n}\n",
		"E0012", `"Describe" is not camelCase`, 2, 8)
	wantDiag(t, "interface Twice {\n    fn dup(self) -> Int64\n    fn dup(self) -> String\n}\n",
		"E0404", `already declared at line 2`, 3, 8)
}

// --- D2: associated types ------------------------------------------------------

func TestInterfaceAssocLimits(t *testing.T) {
	f := wantClean(t, "interface Four {\n    type A\n    type B\n    type C\n    type D\n}\n")
	if d := f.Items[0].(*ast.InterfaceDecl); len(d.Assocs) != 4 {
		t.Fatalf("four assocs wanted, got %d", len(d.Assocs))
	}
	// The fifth associated type anchors the fifth `type` keyword (D5).
	wantDiag(t, "interface Five {\n    type A\n    type B\n    type C\n    type D\n    type E\n}\n",
		"E0803", `a fifth associated type "E"`, 6, 5)
	// A bound on the hole's declaration anchors the bound's first token.
	wantDiag(t, "interface Bounded {\n    type Iter: Iterator<Int64>\n}\n",
		"E0804", `the associated type "Iter" carries an upper bound`, 2, 16)
}

// --- D2: impl declarations -----------------------------------------------------

func TestImplDecls(t *testing.T) {
	// for-form.
	f := wantClean(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\n")
	d, ok := f.Items[2].(*ast.ImplDecl)
	if !ok {
		t.Fatalf("ImplDecl wanted, got %T", f.Items[2])
	}
	ifaceT, ok1 := d.Iface.(*ast.NamedType)
	if !ok1 || ifaceT.Name != "Describable" || len(ifaceT.Args) != 0 {
		t.Fatalf("iface Describable wanted, got %#v", d.Iface)
	}
	headT, ok2 := d.Head.(*ast.NamedType)
	if !ok2 || headT.Name != "User" || len(headT.Args) != 0 {
		t.Fatalf("head User wanted, got %#v", d.Head)
	}
	if len(d.TypeParams) != 0 {
		t.Fatalf("no clause wanted, got %+v", d.TypeParams)
	}
	if len(d.Methods) != 1 {
		t.Fatalf("one method wanted, got %d", len(d.Methods))
	}
	m := d.Methods[0]
	if m.Name != "describe" || m.Recv != ast.RecvSelf {
		t.Fatalf("impl method shape wrong: %+v", m)
	}

	// inherent form: no `for`, Iface nil.
	hf := wantClean(t, "record User { name: String }\nimpl User {\n    fn upper(self) -> String {\n        \"u\"\n    }\n}\n")
	hd := hf.Items[1].(*ast.ImplDecl)
	hInh, hInhOK := hd.Head.(*ast.NamedType)
	if hd.Iface != nil || !hInhOK || hInh.Name != "User" {
		t.Fatalf("inherent impl wanted nil iface and head User, got %#v / %#v", hd.Iface, hd.Head)
	}

	// generic impl: clause on impl, application in head.
	gf := wantClean(t, "interface Holder {\n    fn held(self) -> Int64\n}\nrecord Wrap<T> { value: T }\nimpl<T> Holder for Wrap<T> {\n    fn held(self) -> Int64 {\n        0\n    }\n}\n")
	gd := gf.Items[2].(*ast.ImplDecl)
	if len(gd.TypeParams) != 1 || gd.TypeParams[0].Name != "T" {
		t.Fatalf("impl clause <T> wanted, got %+v", gd.TypeParams)
	}
	gHead, gHeadOK := gd.Head.(*ast.NamedType)
	if !gHeadOK || gHead.Name != "Wrap" || len(gHead.Args) != 1 {
		t.Fatalf("head Wrap<T> wanted, got %#v", gd.Head)
	}
	if arg, ok := gHead.Args[0].(*ast.NamedType); !ok || arg.Name != "T" {
		t.Fatalf("head arg T wanted, got %#v", gHead.Args[0])
	}

	// associated-type bindings precede methods (E0806 anchors the late one).
	bf := wantClean(t, "interface HasIter {\n    type Iter\n}\nrecord Cfg { n: Int64 }\nimpl HasIter for Cfg {\n    type Iter = Int64\n    fn it(self) -> Int64 {\n        0\n    }\n}\n")
	bd := bf.Items[2].(*ast.ImplDecl)
	if len(bd.Assocs) != 1 || bd.Assocs[0].Name != "Iter" {
		t.Fatalf("binding Iter wanted, got %+v", bd.Assocs)
	}
	if ret, ok := bd.Assocs[0].Type.(*ast.NamedType); !ok || ret.Name != "Int64" {
		t.Fatalf("binding type Int64 wanted, got %#v", bd.Assocs[0].Type)
	}
	if len(bd.Methods) != 1 || bd.Methods[0].Name != "it" {
		t.Fatalf("method it wanted, got %+v", bd.Methods)
	}
	wantDiag(t, "interface HasIter {\n    type Iter\n}\nrecord Cfg { n: Int64 }\nimpl HasIter for Cfg {\n    fn it(self) -> Int64 {\n        0\n    }\n    type Iter = Int64\n}\n",
		"E0806", `the binding of "Iter" follows the method "it"`, 9, 5)

	// where clause on the impl head, after the head, before the brace.
	wf := wantClean(t, "interface A {\n    fn a(self) -> Int64\n}\nrecord Wrap<T> { value: T }\nimpl<T> A for Wrap<T> where T: A {\n    fn a(self) -> Int64 {\n        0\n    }\n}\n")
	wd := wf.Items[2].(*ast.ImplDecl)
	if len(wd.Where) != 1 || wd.Where[0].Subject != "T" {
		t.Fatalf("where T wanted, got %+v", wd.Where)
	}
	if len(wd.Where[0].Ifaces) != 1 || wd.Where[0].Ifaces[0].Name != "A" {
		t.Fatalf("bound A wanted, got %+v", wd.Where[0].Ifaces)
	}

	// mut self on an impl method parses (receiver discipline is the
	// checker's E0812, not a syntax rule).
	mf := wantClean(t, "record Cell { n: Int64 }\nimpl Cell {\n    fn bump(mut self) -> Int64 {\n        0\n    }\n}\n")
	if mm := mf.Items[1].(*ast.ImplDecl).Methods[0]; mm.Recv != ast.RecvMutSelf {
		t.Fatalf("mut self wanted, got %q", mm.Recv)
	}
}

// --- D2: receiver parameters ---------------------------------------------------

func TestReceiverParams(t *testing.T) {
	// The receiver is a bare first parameter: `self` or `mut self`.
	// An annotated first parameter anchors the parameter name (E0801).
	wantDiag(t, "interface Greeter {\n    fn greet(name: String) -> String\n}\n",
		"E0801", "the first parameter of \"greet\" carries an annotation", 2, 14)
	// Zero parameters anchors the parameter list's `(` (E0801).
	wantDiag(t, "interface Describable {\n    fn describe() -> String\n}\n",
		"E0801", `"describe" declares no parameters`, 2, 16)
	// Any other bare name anchors that name (E0802).
	wantDiag(t, "record User { name: String }\nimpl User {\n    fn upper(me) -> String {\n        \"u\"\n    }\n}\n",
		"E0802", `the receiver is spelled "me"`, 3, 14)
	// A plain fn keeps chapter 6's every-parameter-annotated rule.
	wantDiag(t, "fn f(me) -> Int64 {\n    0\n}\n",
		"E0105", `parameter "me" carries no annotation`, 1, 6)
}

// --- D2: generic clauses -------------------------------------------------------

func TestGenericClauses(t *testing.T) {
	ff := wantClean(t, "fn id<T>(x: T) -> T {\n    x\n}\n")
	if fp := ff.Items[0].(*ast.FnDecl); len(fp.TypeParams) != 1 || fp.TypeParams[0].Name != "T" {
		t.Fatalf("fn clause wanted, got %+v", fp.TypeParams)
	}
	rf := wantClean(t, "record Box<T> { value: T }\n")
	if rp := rf.Items[0].(*ast.RecordDecl); len(rp.TypeParams) != 1 {
		t.Fatalf("record clause wanted, got %+v", rp.TypeParams)
	}
	nf := wantClean(t, "newtype Tagged<T>(T)\n")
	if np := nf.Items[0].(*ast.NewtypeDecl); len(np.TypeParams) != 1 {
		t.Fatalf("newtype clause wanted, got %+v", np.TypeParams)
	}
	sf := wantClean(t, "type Opt<T> = Some(T) | None\n")
	if sp := sf.Items[0].(*ast.SumDecl); len(sp.TypeParams) != 1 {
		t.Fatalf("sum clause wanted, got %+v", sp.TypeParams)
	}
	// Nine is above the ratified eight; the anchor is the clause's `<`.
	wantDiag(t, "fn f<T1, T2, T3, T4, T5, T6, T7, T8, T9>(x: T1) -> T1 {\n    x\n}\n",
		"E0825", "the clause declares 9 parameters", 1, 5)
}

// --- D2: where clauses ---------------------------------------------------------

func TestWhereClauses(t *testing.T) {
	// fn signature tail, between the return and the body.
	f := wantClean(t, "fn show<T>(x: T) -> String where T: Describable {\n    \"\"\n}\n")
	fp := f.Items[0].(*ast.FnDecl)
	if len(fp.Where) != 1 || fp.Where[0].Subject != "T" {
		t.Fatalf("where T wanted, got %+v", fp.Where)
	}
	if len(fp.Where[0].Ifaces) != 1 || fp.Where[0].Ifaces[0].Name != "Describable" {
		t.Fatalf("bound Describable wanted, got %+v", fp.Where[0].Ifaces)
	}
	// `T: A + B` grants the union of two bounds.
	mf := wantClean(t, "fn use2<T>(x: T) -> Int64 where T: A1 + B1 {\n    0\n}\n")
	if wb := mf.Items[0].(*ast.FnDecl).Where[0]; len(wb.Ifaces) != 2 {
		t.Fatalf("two bounds wanted, got %+v", wb.Ifaces)
	}
	// `T.Item == Concrete` pins an associated type inside the declaration —
	// one WhereBound per constraint, so the equality rides the second bound.
	ef := wantClean(t, "fn take<T>(x: T) -> Int64 where T: Holder, T.Item == Int64 {\n    0\n}\n")
	ew := ef.Items[0].(*ast.FnDecl).Where
	if len(ew) != 2 {
		t.Fatalf("two where bounds wanted, got %+v", ew)
	}
	eb := ew[1]
	if eb.Subject != "T" || len(eb.Eq) != 1 || eb.Eq[0].Assoc != "Item" {
		t.Fatalf("equality Item wanted, got %+v", eb)
	}
	if rhs, ok := eb.Eq[0].RHS.(*ast.NamedType); !ok || rhs.Name != "Int64" {
		t.Fatalf("right side Int64 wanted, got %#v", eb.Eq[0].RHS)
	}
}

// --- D2: derives clauses -------------------------------------------------------

func TestDerivesClauses(t *testing.T) {
	rf := wantClean(t, "record Point { x: Int64, y: Int64 } derives Eq, Hash, Show\n")
	rd := rf.Items[0].(*ast.RecordDecl)
	if rd.Derives == nil || len(rd.Derives.Targets) != 3 {
		t.Fatalf("three targets wanted, got %+v", rd.Derives)
	}
	want := []string{"Eq", "Hash", "Show"}
	for i, name := range want {
		if rd.Derives.Targets[i] != name {
			t.Fatalf("target %d wanted %s, got %s", i, name, rd.Derives.Targets[i])
		}
	}
	nf := wantClean(t, "newtype Tagged(Int64) derives Eq\n")
	if nd := nf.Items[0].(*ast.NewtypeDecl); nd.Derives == nil || len(nd.Derives.Targets) != 1 {
		t.Fatalf("newtype derives wanted, got %+v", nd.Derives)
	}
	sf := wantClean(t, "type Opt = Some(Int64) | None derives Eq\n")
	if sd := sf.Items[0].(*ast.SumDecl); sd.Derives == nil {
		t.Fatalf("sum derives wanted, got %+v", sd.Derives)
	}
	pf := wantClean(t, "record Plain { x: Int64 }\n")
	if pd := pf.Items[0].(*ast.RecordDecl); pd.Derives != nil {
		t.Fatalf("no clause wanted, got %+v", pd.Derives)
	}
	// The target set is closed; each violation anchors the target name.
	wantDiag(t, "record Point { x: Int64 } derives Ord\n",
		"E0824", `"Ord" is not a derive target`, 1, 35)
	wantDiag(t, "record Point { x: Int64 } derives Eq, Eq\n",
		"E0824", `"Eq" appears twice in the clause`, 1, 39)
	wantDiag(t, "record Point { x: Int64 } derives Shareable\n",
		"E0824", `"Shareable" is not a derive target`, 1, 35)
}

// --- D2/T4: the for statement and its head patterns ------------------------------

func TestForStmt(t *testing.T) {
	ss := stmtsOf(t, "for x in xs {\n        let _ = x\n    }\n")
	fs, ok := ss[0].(*ast.ForStmt)
	if !ok {
		t.Fatalf("ForStmt wanted, got %T", ss[0])
	}
	if b, ok := fs.Pat.(*ast.PatBinding); !ok || b.Name != "x" {
		t.Fatalf("binding head wanted, got %#v", fs.Pat)
	}
	if id, ok := fs.Iter.(*ast.Ident); !ok || id.Name != "xs" {
		t.Fatalf("iterable xs wanted, got %#v", fs.Iter)
	}
	if len(fs.Body.Items) != 1 {
		t.Fatalf("one-item body wanted, got %d", len(fs.Body.Items))
	}

	// wildcard and tuple heads.
	ws := stmtsOf(t, "for _ in xs {\n        let _ = 1\n    }\n")
	if _, ok := ws[0].(*ast.ForStmt).Pat.(*ast.PatWildcard); !ok {
		t.Fatalf("wildcard head wanted, got %#v", ws[0].(*ast.ForStmt).Pat)
	}
	ts := stmtsOf(t, "for (k, v) in mm {\n        let _ = k\n    }\n")
	if tp, ok := ts[0].(*ast.ForStmt).Pat.(*ast.PatTuple); !ok || len(tp.Elems) != 2 {
		t.Fatalf("2-tuple head wanted, got %#v", ts[0].(*ast.ForStmt).Pat)
	}

	// A refutable head pattern is not a for head; the anchor is the
	// pattern's first token.
	wantDiag(t, "fn probe() {\n    for Some(x) in xs {\n        let _ = x\n    }\n}\n",
		"E0105", `the "for" head takes an irrefutable pattern`, 2, 9)
	// Value position keeps the E0202 family (statement-position rule).
	wantDiag(t, "fn f() {\n    let x = for i in 0..1 {\n        let _ = i\n    }\n}\n",
		"E0202", `"for" produces no value and cannot stand in a let initializer`, 2, 13)
}

// --- D2/T4: list literals ------------------------------------------------------

func TestListLit(t *testing.T) {
	e := exprOf(t, "[1, 2, 3]")
	l, ok := e.(*ast.ListLit)
	if !ok {
		t.Fatalf("ListLit wanted, got %T", e)
	}
	if len(l.Elems) != 3 {
		t.Fatalf("three elements wanted, got %d", len(l.Elems))
	}
	if _, ok := l.Elems[0].(*ast.Literal); !ok {
		t.Fatalf("literal element wanted, got %T", l.Elems[0])
	}
	// empty and trailing-comma forms.
	if el := exprOf(t, "[]"); len(el.(*ast.ListLit).Elems) != 0 {
		t.Fatalf("empty list wanted")
	}
	if tl := exprOf(t, "[1, 2, 3,]"); len(tl.(*ast.ListLit).Elems) != 3 {
		t.Fatalf("trailing comma folds away, got %d", len(tl.(*ast.ListLit).Elems))
	}
	// Brackets are a paren region: the literal folds across lines.
	f := wantClean(t, "fn f() {\n    let xs = [\n        1,\n        2,\n    ]\n}\n")
	b := f.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.Binding)
	if ml, ok := b.Init.(*ast.ListLit); !ok || len(ml.Elems) != 2 {
		t.Fatalf("folded two-element list wanted, got %#v", b.Init)
	}
	// postfix `[` stays the permanent rejection (chapter 17 policy).
	wantDiag(t, "fn f(s: String) {\n    let n = s[0]\n}\n",
		"E0105", "indexing is by named methods", 2, 14)
}

// --- D2/T4: explicit type arguments — the postfix angle-bracket lookahead ---------

func TestExplicitTypeArgs(t *testing.T) {
	// call form: f<T>(args)
	e := exprOf(t, "id<Int64>(1)")
	c, ok := e.(*ast.Call)
	if !ok {
		t.Fatalf("Call wanted, got %T", e)
	}
	if len(c.TypeArgs) != 1 {
		t.Fatalf("one type argument wanted, got %+v", c.TypeArgs)
	}
	if ta, ok := c.TypeArgs[0].(*ast.NamedType); !ok || ta.Name != "Int64" {
		t.Fatalf("Int64 argument wanted, got %#v", c.TypeArgs[0])
	}
	if _, ok := c.Fn.(*ast.Ident); !ok {
		t.Fatalf("bare ident head wanted, got %T", c.Fn)
	}

	// construct form: Name<T1, T2> { ... }
	f := wantClean(t, "fn f() {\n    let p = Pair<Int64, String> { first: 1, second: \"a\" }\n}\n")
	b := f.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.Binding)
	ct, ok := b.Init.(*ast.Construct)
	if !ok {
		t.Fatalf("Construct wanted, got %T", b.Init)
	}
	if len(ct.TypeArgs) != 2 {
		t.Fatalf("two type arguments wanted, got %+v", ct.TypeArgs)
	}

	// newtype/variant call form: Name<T>(arg)
	nf := wantClean(t, "newtype Tagged<T>(T)\nfn f() {\n    let t = Tagged<Int64>(1)\n}\n")
	nc := nf.Items[1].(*ast.FnDecl).Body.Items[0].(*ast.Binding).Init.(*ast.Call)
	if len(nc.TypeArgs) != 1 || nc.TypeArgs[0].(*ast.NamedType).Name != "Int64" {
		t.Fatalf("Tagged<Int64> wanted, got %+v", nc.TypeArgs)
	}

	// The lookahead never reaches a method head: `obj.m<T>(x)` keeps the
	// comparison reading (chapter 10's no-explicit-form rule for methods).
	wantDiag(t, "fn f() {\n    let text = u.describe<Int64>()\n}\n",
		"E0104", `">" chains a non-associative level`, 2, 32)

	// A comparison with a block stays a comparison.
	cf := wantClean(t, "fn f(a: Int64, b: Int64) {\n    if a < b {\n        let _ = a\n    }\n}\n")
	is, ok := cf.Items[0].(*ast.FnDecl).Body.Items[0].(*ast.ExprStmt).Expr.(*ast.If)
	if !ok {
		t.Fatalf("if expression statement wanted, got %T", cf.Items[0].(*ast.FnDecl).Body.Items[0])
	}
	if _, ok := is.Cond.(*ast.Binary); !ok {
		t.Fatalf("comparison condition wanted, got %T", is.Cond)
	}

	// Nested closers in type slots stay separated (chapter 1 rule).
	wantDiag(t, "fn f(b: Box2<Box2<Int64>>) -> Int64 {\n    0\n}\n",
		"E0105", `nested closers are separated`, 1, 24)
}
