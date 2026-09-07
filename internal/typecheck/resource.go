package typecheck

import (
	"fmt"
	"sort"

	"github.com/ltlvtao/welang/internal/ast"
)

// Chapter 13's release discipline — the structured liveness walk (design
// D4) and the alias ban's binding-side shapes (design D5). The assignment
// shape of E1105 and the release-direct shape of E1104 are woven into the
// typing pass (checkAssign and memberOfType); everything here runs after a
// body typed clean, over the same tree, with each let's resource category
// recorded by the walk (resLets).
//
// The model is the spec's: a resource binding — a parameter, a let, a scope
// resource head name — is live from its binding point, and every control
// path must hand it to one of the three transfers before its scope ends: a
// scope resource head (the head binding takes the handle over), a return of
// the binding (the obligation moves to the caller), or a call argument in a
// parameter's position (the obligation moves to the callee's signature). On
// a body that typed clean, each of those three is exactly a bare name in
// that syntactic position — a non-resource parameter would have been E0501,
// a return of the wrong type likewise — so the walk reads them off the tree
// without re-consulting types.

// resState is one resource binding's state on the walked path.
type resState int

const (
	resLive resState = iota
	resKilled
)

// resBind is one resource binding on a control path. Owned marks a scope
// resource head name: the scope-exit machinery owns its release, so it
// carries no drop obligation (its reads still check, and typing's
// release-direct rule still guards it). Shadowed holds the binding the name
// covered, restored at the block's exit.
type resBind struct {
	name      string
	line, col int // the binding's name token — the drop report's anchor
	state     resState
	seq       int // declaration order within the fn; the earliest bad reports
	owned     bool
	shadowed  *resBind
}

// resLoopFrame is the innermost loop's attribution set: the names already
// bound at the loop's entry (break/continue end the scopes of everything
// declared since), and the break snapshots that join the after-loop path.
type resLoopFrame struct {
	entry  map[string]bool
	breaks []map[string]resState
}

// resWalk is one body's liveness walk. env holds the resource bindings of
// every enclosing scope on the current path; branch points clone it and
// join back by "live on any path" — the judgment reads "a path with no
// transfer".
type resWalk struct {
	c      *checker
	env    map[string]*resBind
	seq    int
	all    map[int]*resBind // seq -> the declaration's own binding
	badSeq map[int]bool
	bad    []*resBind
	frames []*resLoopFrame
}

// resCheck runs the discipline over one typed-clean body: params first
// (each resource-typed parameter is a live binding from the fn's entry),
// then the items, then the single report — the earliest-declared binding
// that reached a scope end live on some path.
func (c *checker) resCheck(items []ast.Stmt, params []ast.Param, types []Type) {
	w := &resWalk{
		c:      c,
		env:    map[string]*resBind{},
		all:    map[int]*resBind{},
		badSeq: map[int]bool{},
	}
	for i := range params {
		if params[i].Name == "_" || catOf(types[i]) != "resource" {
			continue
		}
		w.declare(params[i].Name, params[i].NameLine, params[i].NameCol, false)
	}
	w.block(items, nil)
	if len(w.bad) == 0 {
		return
	}
	sort.Slice(w.bad, func(i, j int) bool { return w.bad[i].seq < w.bad[j].seq })
	b := w.bad[0]
	c.fail(b.line, b.col, "E1104", fmt.Sprintf(
		"resource binding outside its release discipline — the binding %q reaches the end of its scope on a path with no transfer; transfer it on every path - scope resource(x = %s), return %s, or pass it to a fn whose parameter declares the type",
		b.name, b.name, b.name))
}

// declare enters one resource binding into the current scope.
func (w *resWalk) declare(name string, line, col int, owned bool) *resBind {
	b := &resBind{
		name:     name,
		line:     line,
		col:      col,
		state:    resLive,
		seq:      w.seq,
		owned:    owned,
		shadowed: w.env[name],
	}
	w.seq++
	w.all[b.seq] = b
	w.env[name] = b
	return b
}

// markBad records a binding as having reached a scope end live — keyed by
// seq, so a branch clone's judgment lands on the declaration itself.
func (w *resWalk) markBad(b *resBind) {
	if w.badSeq[b.seq] {
		return
	}
	w.badSeq[b.seq] = true
	w.bad = append(w.bad, w.all[b.seq])
}

// block walks one block's items. pre carries bindings the block inherits as
// already declared (a scope resource head's owned names). Block exit is the
// judgment point: a binding declared here that is still live on this path
// has no transfer on it (E1104, reported once at the walk's end).
func (w *resWalk) block(items []ast.Stmt, pre []*resBind) {
	decls := append([]*resBind{}, pre...)
	terminated := false
	for _, s := range items {
		if terminated {
			break // the path ended at a return/break/continue; the rest is off it
		}
		terminated = w.stmt(s, &decls)
	}
	for _, b := range decls {
		if !b.owned && b.state == resLive {
			w.markBad(b)
		}
	}
	for _, b := range decls {
		if b.shadowed != nil {
			w.env[b.name] = b.shadowed
		} else {
			delete(w.env, b.name)
		}
	}
}

// stmt walks one statement; the return reports whether the control path
// terminated inside it (return, break, and continue all end their block's
// path — and, per chapter 13, all three are scope exits).
func (w *resWalk) stmt(s ast.Stmt, decls *[]*resBind) bool {
	switch st := s.(type) {
	case *ast.Binding:
		if st.Pat != nil {
			// A tuple pattern binds no resource — E1106 holds the element
			// slots — so only the initializer's reads can matter here.
			w.expr(st.Init)
			return false
		}
		if w.c.resLets[st] {
			// The alias ban: a second handle is E1105's shape (the spec's
			// let and var alias forms alike); a read of a killed source is
			// the use-after shape of E1104 — the walk knows which.
			if id, ok := st.Init.(*ast.Ident); ok {
				if src := w.env[id.Name]; src != nil {
					if src.state == resKilled {
						w.useAfter(id)
					}
					w.c.fail(st.NameLine, st.NameCol, "E1105", fmt.Sprintf(
						"resource binding rebound — %q is a second handle to the resource of %q; the one sanctioned move is a transfer, which hands over and kills the source",
						st.Name, id.Name))
				}
			}
			if st.Kw == "var" {
				w.c.fail(st.Line, st.Col, "E1105", fmt.Sprintf(
					"resource binding rebound — %q is declared var and rebindable; a resource binding is let-shaped, and the one sanctioned move is a transfer",
					st.Name))
			}
			w.expr(st.Init)
			*decls = append(*decls, w.declare(st.Name, st.NameLine, st.NameCol, false))
			return false
		}
		w.expr(st.Init)
		return false
	case *ast.Assign:
		// E1105's assignment shape fired during typing if either side moved
		// a resource; what remains is the ordinary read discipline.
		w.expr(st.Value)
		return false
	case *ast.Return:
		if st.HasValue {
			if id, ok := st.Value.(*ast.Ident); ok {
				if b := w.env[id.Name]; b != nil && b.state == resLive {
					b.state = resKilled // the return carries the obligation to the caller
				}
			} else {
				w.expr(st.Value)
			}
		}
		// The fn's exit is every still-live binding's scope end.
		for _, b := range w.env {
			if !b.owned && b.state == resLive {
				w.markBad(b)
			}
		}
		return true
	case *ast.While:
		w.expr(st.Cond)
		w.loop(st.Body.Items, true)
		return false
	case *ast.Loop:
		// A loop form enters unconditionally: no zero-iteration path joins.
		w.loop(st.Body.Items, false)
		return false
	case *ast.ForStmt:
		w.expr(st.Iter)
		// for is a loop form (chapter 5): an empty iterable is a path, and
		// break/continue exit it like any other.
		w.loop(st.Body.Items, true)
		return false
	case *ast.Defer:
		// The body runs at fn exit: its reads check against this point's
		// states, its own bindings face their own block exit, and nothing
		// it does flows back into this path.
		saved := w.env
		w.env = cloneResEnv(w.env)
		w.block(st.Block.Items, nil)
		w.env = saved
		return false
	case *ast.Break:
		w.exitLoop(true)
		return true
	case *ast.Continue:
		w.exitLoop(false)
		return true
	case *ast.ScopeRes:
		var owned []*resBind
		for _, sb := range st.Binds {
			transferred := false
			if id, ok := sb.Val.(*ast.Ident); ok {
				if b := w.env[id.Name]; b != nil && b.state == resLive {
					b.state = resKilled // the head binding takes the handle over
					transferred = true
				}
			}
			if !transferred {
				w.expr(sb.Val)
			}
			owned = append(owned, w.declare(sb.Name, sb.Line, sb.Col, true))
		}
		w.block(st.Body.Items, owned)
		return false
	case *ast.ExprStmt:
		w.expr(st.Expr)
		return false
	}
	return false
}

// loop walks a loop form's body. entryJoins marks whether a path that never
// enters the body exists (while and for: yes; loop: no). The after-loop
// path joins the fall-through with every break snapshot — and with the
// entry states when the body may not run — under "live on any path".
func (w *resWalk) loop(items []ast.Stmt, entryJoins bool) {
	frame := &resLoopFrame{entry: map[string]bool{}}
	entryStates := map[string]resState{}
	for name, b := range w.env {
		frame.entry[name] = true
		entryStates[name] = b.state
	}
	w.frames = append(w.frames, frame)
	w.block(items, nil)
	w.frames = w.frames[:len(w.frames)-1]
	joins := frame.breaks
	if entryJoins {
		joins = append(joins, entryStates)
	}
	for name, b := range w.env {
		live := b.state == resLive
		for _, snap := range joins {
			if s, ok := snap[name]; ok && s == resLive {
				live = true
			}
		}
		if live {
			b.state = resLive
		} else {
			b.state = resKilled
		}
	}
}

// exitLoop holds break/continue's scope end: everything declared since the
// innermost loop's entry ends here (chapter 13 reads the penetrating exits
// as block exits — an untransferred binding drops). A break additionally
// snapshots the loop-carried bindings for the after-loop join.
func (w *resWalk) exitLoop(isBreak bool) {
	frame := w.frames[len(w.frames)-1]
	snap := map[string]resState{}
	for name, b := range w.env {
		if frame.entry[name] {
			snap[name] = b.state
			continue
		}
		if !b.owned && b.state == resLive {
			w.markBad(b)
		}
	}
	if isBreak {
		frame.breaks = append(frame.breaks, snap)
	}
}

// cloneResEnv deep-copies a branch point's bindings: a kill inside one arm
// writes the clone, never the path's own state — the join decides.
func cloneResEnv(env map[string]*resBind) map[string]*resBind {
	out := make(map[string]*resBind, len(env))
	for k, v := range env {
		bc := *v
		out[k] = &bc
	}
	return out
}

// branch runs f on a clone of the path's bindings and returns the clone.
func (w *resWalk) branch(f func()) map[string]*resBind {
	saved := w.env
	w.env = cloneResEnv(w.env)
	f()
	out := w.env
	w.env = saved
	return out
}

// join merges branch results into the path's bindings: a binding is live
// after the fork when any path carries it live (the drop judgment reads "a
// path with no transfer"); an arm's own declarations never reach the join —
// its block exit judged them.
func (w *resWalk) join(envs ...map[string]*resBind) {
	for _, b := range w.env {
		live := false
		for _, e := range envs {
			if eb, ok := e[b.name]; ok && eb.state == resLive {
				live = true
				break
			}
		}
		if live {
			b.state = resLive
		} else {
			b.state = resKilled
		}
	}
}

// useAfter reports the one-handle breach at the read's own token.
func (w *resWalk) useAfter(id *ast.Ident) {
	w.c.fail(id.Line, id.Col, "E1104", fmt.Sprintf(
		"resource binding outside its release discipline — %q is used after its transfer; a transfer hands over and kills the source binding, and the one handle is gone",
		id.Name))
}

// recvText renders a receiver's name chain for the release-direct message
// (the golden's "f.release()" shape): an identifier, or a member chain
// rooted in one; any other receiver renders as a placeholder — the anchor
// already sits at its first token.
func recvText(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.Member:
		return recvText(x.Recv) + "." + x.Name
	}
	return "<expr>"
}

// resAssignCheck holds the alias ban's value side (chapter 13): a bare name
// of resource type as the assigned value moves the handle — the same
// rebound shape whatever the target (E1105, at the `=`; the target side's
// check sits in checkAssign beside this one).
func (c *checker) resAssignCheck(a *ast.Assign) {
	id, ok := a.Value.(*ast.Ident)
	if !ok {
		return
	}
	var t Type
	if lt, is := c.lookupLocal(id.Name); is {
		t = lt
	} else if sym, is := c.syms[id.Name]; is && sym.kind == symLet {
		t = sym.letType
	} else {
		return
	}
	if catOf(t) == "resource" {
		c.fail(a.OpLine, a.OpCol, "E1105",
			"resource binding rebound — the assignment moves a resource binding on one side; the one sanctioned move is a transfer, which hands over and kills the source")
	}
}

// expr walks one expression: bare names in transfer positions (a call
// argument, handled at the Call arm) kill their binding; every other read
// of a killed binding is the use-after shape (E1104, at the read).
func (w *resWalk) expr(e ast.Expr) {
	switch x := e.(type) {
	case *ast.Ident:
		if b := w.env[x.Name]; b != nil && b.state == resKilled {
			w.useAfter(x)
		}
	case *ast.Literal, *ast.Unit:
	case *ast.Unary:
		w.expr(x.X)
	case *ast.Binary:
		w.expr(x.L)
		w.expr(x.R)
	case *ast.Tuple:
		for _, el := range x.Elems {
			w.expr(el)
		}
	case *ast.ListLit:
		for _, el := range x.Elems {
			w.expr(el)
		}
	case *ast.Member:
		w.expr(x.Recv)
	case *ast.Call:
		w.expr(x.Fn)
		for _, a := range x.Args {
			if id, ok := a.(*ast.Ident); ok {
				if b := w.env[id.Name]; b != nil && b.state == resLive {
					b.state = resKilled // the argument hands the handle to the callee's signature
					continue
				}
			}
			w.expr(a)
		}
	case *ast.Construct:
		if x.Base != nil {
			w.expr(x.Base)
		}
		for _, f := range x.Fields {
			w.expr(f.Value)
		}
	case *ast.If:
		w.expr(x.Cond)
		thenEnv := w.branch(func() { w.block(x.Then.Items, nil) })
		elseEnv := w.branch(func() {
			switch el := x.Else.(type) {
			case nil:
				// No else arm: the clone untouched is the empty path.
			case *ast.BlockExpr:
				w.block(el.Block.Items, nil)
			case *ast.If:
				w.expr(el)
			}
		})
		w.join(thenEnv, elseEnv)
	case *ast.Match:
		w.expr(x.Scrutinee)
		envs := make([]map[string]*resBind, 0, len(x.Arms))
		for _, arm := range x.Arms {
			envs = append(envs, w.branch(func() {
				if arm.Guard != nil {
					w.expr(arm.Guard)
				}
				w.expr(arm.Body)
			}))
		}
		// Exhaustiveness held at typing: the arms alone cover every path.
		w.join(envs...)
	case *ast.BlockExpr:
		// A block in expression position runs on this path: its kills flow
		// onward, its own declarations face its own block exit.
		w.block(x.Block.Items, nil)
	case *ast.Closure:
		// The body's discipline ran inside closureType when it typed —
		// and no outer resource binding is readable here (E1002 held it).
	case *ast.Prop:
		w.expr(x.X)
	}
}

// --- chapter 18: task-block capture discipline (design D4) ------------------

// patBindNames collects one pattern's binding names (the wildcard binds
// nothing). Shared by the var-scope recording and the capture walk's
// pattern-position bindings.
func patBindNames(p ast.Pattern) []string {
	var names []string
	var walk func(p ast.Pattern)
	walk = func(p ast.Pattern) {
		switch x := p.(type) {
		case *ast.PatBinding:
			names = append(names, x.Name)
		case *ast.PatTuple:
			for _, e := range x.Elems {
				walk(e)
			}
		case *ast.PatVariant:
			for _, a := range x.Args {
				walk(a)
			}
		case *ast.PatOr:
			for _, b := range x.Branches {
				walk(b)
			}
		}
	}
	walk(p)
	return names
}

// addNames binds pattern names into a bound set (the wildcard and the
// discard name bind nothing).
func addNames(bound map[string]bool, names []string) {
	for _, n := range names {
		if n != "_" {
			bound[n] = true
		}
	}
}

// capUse is one free-identifier use inside a task body, with the use's
// own position — the capture judgments anchor at the first use (design
// D4).
type capUse struct {
	name      string
	line, col int
}

// taskCaptures collects the task body's free identifiers in source
// order: names used before the body itself binds them, position-aware
// (a binding scopes from its statement onward, branch bindings stay in
// their branch). Resolving them against the enclosing bindings is the
// caller's judgment — module-level names are not captures (a call or a
// type reference carries no state). This is deliberately NOT chapter
// 12's closure-capture ledger: ch18:135 keeps the two disciplines apart
// (closures take a reference-and-synchronization discipline, task blocks
// a category discipline).
func (c *checker) taskCaptures(x *ast.TaskExpr) []capUse {
	var uses []capUse
	var expr func(e ast.Expr, bound map[string]bool)
	var block func(items []ast.Stmt, bound map[string]bool)

	copyBound := func(b map[string]bool) map[string]bool {
		nb := make(map[string]bool, len(b)+1)
		for k := range b {
			nb[k] = true
		}
		return nb
	}

	expr = func(e ast.Expr, bound map[string]bool) {
		switch x := e.(type) {
		case *ast.Ident:
			if !bound[x.Name] {
				uses = append(uses, capUse{name: x.Name, line: x.Line, col: x.Col})
			}
		case *ast.Unary:
			expr(x.X, bound)
		case *ast.Binary:
			expr(x.L, bound)
			expr(x.R, bound)
		case *ast.Call:
			expr(x.Fn, bound)
			for _, a := range x.Args {
				expr(a, bound)
			}
		case *ast.Member:
			expr(x.Recv, bound)
		case *ast.BlockExpr:
			block(x.Block.Items, copyBound(bound))
		case *ast.If:
			expr(x.Cond, bound)
			block(x.Then.Items, copyBound(bound))
			if x.Else != nil {
				expr(x.Else, copyBound(bound))
			}
		case *ast.Match:
			expr(x.Scrutinee, bound)
			for _, arm := range x.Arms {
				ab := copyBound(bound)
				if arm.Guard != nil {
					expr(arm.Guard, ab) // the guard sees no arm bindings
				}
				addNames(ab, patBindNames(arm.Pat))
				expr(arm.Body, ab)
			}
		case *ast.TaskExpr:
			// A nested task's body is free in this one too — the capture
			// is transitive, and the inner block faces its own judgment
			// when its own typing runs.
			block(x.Body.Items, copyBound(bound))
		case *ast.ScopeExpr:
			if x.Timeout != nil {
				expr(x.Timeout, bound)
			}
			block(x.Body.Items, copyBound(bound))
		case *ast.SelectExpr:
			for _, cs := range x.Cases {
				expr(cs.Source, bound)
				cb := copyBound(bound)
				if !cs.Wildcard {
					addNames(cb, []string{cs.Name})
				}
				expr(cs.Body, cb)
			}
		case *ast.Construct:
			for _, f := range x.Fields {
				expr(f.Value, bound)
			}
			if x.Base != nil {
				expr(x.Base, bound)
			}
		case *ast.Tuple:
			for _, el := range x.Elems {
				expr(el, bound)
			}
		case *ast.Closure:
			cb := copyBound(bound)
			for _, p := range x.Params {
				if p.Name != "_" {
					cb[p.Name] = true
				}
			}
			block(x.Body.Items, cb)
		case *ast.ListLit:
			for _, el := range x.Elems {
				expr(el, bound)
			}
		case *ast.Prop:
			expr(x.X, bound)
		}
	}

	block = func(items []ast.Stmt, bound map[string]bool) {
		for _, s := range items {
			switch st := s.(type) {
			case *ast.Binding:
				expr(st.Init, bound)
				if st.Pat != nil {
					addNames(bound, patBindNames(st.Pat))
				} else if st.Name != "_" {
					bound[st.Name] = true
				}
			case *ast.Assign:
				if st.Field == "" {
					if !bound[st.Name] {
						uses = append(uses, capUse{name: st.Name, line: st.Line, col: st.Col})
					}
				} else {
					// self.field = v: the receiver name is the use.
					if !bound["self"] {
						uses = append(uses, capUse{name: "self", line: st.Line, col: st.Col})
					}
				}
				expr(st.Value, bound)
			case *ast.Return:
				if st.HasValue {
					expr(st.Value, bound)
				}
			case *ast.ExprStmt:
				expr(st.Expr, bound)
			case *ast.While:
				expr(st.Cond, bound)
				block(st.Body.Items, copyBound(bound))
			case *ast.Loop:
				block(st.Body.Items, copyBound(bound))
			case *ast.Defer:
				block(st.Block.Items, copyBound(bound))
			case *ast.Break, *ast.Continue:
			case *ast.ForStmt:
				expr(st.Iter, bound)
				fb := copyBound(bound)
				addNames(fb, patBindNames(st.Pat))
				block(st.Body.Items, fb)
			case *ast.ScopeRes:
				sb := copyBound(bound)
				for _, b := range st.Binds {
					expr(b.Val, sb)
					if b.Name != "_" {
						sb[b.Name] = true
					}
				}
				block(st.Body.Items, sb)
			}
		}
	}

	block(x.Body.Items, map[string]bool{})
	return uses
}

// isVarName reports whether the name settles on a var binding in the
// active block layers (innermost first, like the locals stack it
// mirrors). Parameters, patterns, and scope heads are let-shaped; only
// walkItems' binding case writes the layers.
func (c *checker) isVarName(name string) bool {
	for i := len(c.varScopes) - 1; i >= 0; i-- {
		if hit, ok := c.varScopes[i][name]; ok {
			return hit
		}
	}
	return false
}

// mentionsUnboundedParam reports the first generic parameter of the
// enclosing declaration that t mentions without a Shareable bound (the
// capture discipline's third judgment: the mention admits unsynchronized
// gc state at some instantiation).
func (c *checker) mentionsUnboundedParam(t Type) (string, bool) {
	switch x := t.(type) {
	case paramRef:
		if !c.shareableBoundArg(x) {
			return x.name, true
		}
	case recordType:
		for _, a := range x.args {
			if n, ok := c.mentionsUnboundedParam(a); ok {
				return n, true
			}
		}
	case namedType:
		for _, a := range x.args {
			if n, ok := c.mentionsUnboundedParam(a); ok {
				return n, true
			}
		}
	case newtypeType:
		for _, a := range x.args {
			if n, ok := c.mentionsUnboundedParam(a); ok {
				return n, true
			}
		}
		if n, ok := c.mentionsUnboundedParam(x.decl.underlying); ok {
			return n, true
		}
	case tupleType:
		for _, a := range x.elems {
			if n, ok := c.mentionsUnboundedParam(a); ok {
				return n, true
			}
		}
	case fnType:
		for _, a := range x.params {
			if n, ok := c.mentionsUnboundedParam(a); ok {
				return n, true
			}
		}
		if n, ok := c.mentionsUnboundedParam(x.ret); ok {
			return n, true
		}
	}
	return "", false
}

// resourceMutSelf reports the first mut self method a resource record's
// own impls declare beyond release — the read-modify-write surface
// another task can race on (E1604's fact).
func (c *checker) resourceMutSelf(rec *recordInfo) (string, bool) {
	for _, im := range c.impls {
		rt, ok := im.head.(recordType)
		if !ok || rt.decl != rec {
			continue
		}
		for _, m := range im.decl.Methods {
			if m.Recv == ast.RecvMutSelf && m.Name != "release" {
				return m.Name, true
			}
		}
	}
	return "", false
}

// --- chapter 18: scope-block handle discipline (design D5) -------------------

// hdlState is one task-handle binding's state on the walked path. Foreign
// marks a name this walk does not own — a shadowing parameter, pattern, or
// scope-resource head — so a fire on it neither counts nor reports.
type hdlState int

const (
	hdlPending hdlState = iota
	hdlFired
	hdlForeign
)

// hdlBind is one task-handle binding on a control path of a scope body.
// The anchor is the creating task keyword: both of E1607's trigger forms
// report at the creation, not at the miss.
type hdlBind struct {
	name      string
	line, col int // the creating task keyword — E1607's anchor
	state     hdlState
	shadowed  *hdlBind
}

// hdlWalk is one scope body's handle walk — the chapter 13 liveness
// skeleton with one-shot semantics: a fire (await() or cancel() on the
// binding) is the consumption, a block's fall-through end is the judgment
// for what still owes there, and the early exits (return, break, continue)
// discharge — the scope's own exit cancellation covers them.
type hdlWalk struct {
	c   *checker
	env map[string]*hdlBind
}

// handleCheck runs the discipline over one typed-clean scope body: every
// TaskHandle binding the body creates must take exactly one fire on every
// path that is not an early exit, and never two (E1607, both forms).
func (c *checker) handleCheck(items []ast.Stmt) {
	w := &hdlWalk{c: c, env: map[string]*hdlBind{}}
	w.block(items)
}

// declare enters one handle binding; the task keyword anchors its reports.
func (w *hdlWalk) declare(name string, tk *ast.TaskExpr) *hdlBind {
	b := &hdlBind{
		name:     name,
		line:     tk.Line,
		col:      tk.Col,
		state:    hdlPending,
		shadowed: w.env[name],
	}
	w.env[name] = b
	return b
}

// block walks one block of the scope body. The block's fall-through end is
// the judgment point for its own declarations: a binding still pending
// there has no fire on that path and no later name by which to take one.
// An early exit discharges instead (design D5).
func (w *hdlWalk) block(items []ast.Stmt) {
	var decls []*hdlBind
	terminated := false
	for _, s := range items {
		if terminated {
			break // the path ended at a return/break/continue; the rest is off it
		}
		terminated = w.stmt(s, &decls)
	}
	if !terminated {
		for _, b := range decls {
			if b.state == hdlPending {
				w.unawaited(b)
			}
		}
	}
	for _, b := range decls {
		if b.shadowed != nil {
			w.env[b.name] = b.shadowed
		} else {
			delete(w.env, b.name)
		}
	}
}

// stmt walks one statement; the return reports whether the path took an
// early exit (return, break, continue — the discharge set).
func (w *hdlWalk) stmt(s ast.Stmt, decls *[]*hdlBind) bool {
	switch st := s.(type) {
	case *ast.Binding:
		if st.Pat != nil {
			w.expr(st.Init)
			for _, n := range patBindNames(st.Pat) {
				if n == "_" {
					continue
				}
				*decls = append(*decls, &hdlBind{name: n, state: hdlForeign, shadowed: w.env[n]})
				w.env[n] = (*decls)[len(*decls)-1]
			}
			return false
		}
		if tk, ok := st.Init.(*ast.TaskExpr); ok {
			*decls = append(*decls, w.declare(st.Name, tk))
			// The task body is walked as a block of its own: a handle created
			// inside it is judged at that block's end, not silently skipped.
			w.block(tk.Body.Items)
			return false
		}
		w.expr(st.Init)
		return false
	case *ast.Assign:
		// A rebinding of a handle name takes a fresh handle of unknown
		// provenance — the binding owes its fire again (the conservative
		// read; aliasing stays outside the analysis, design D5).
		if st.Field == "" {
			if b := w.env[st.Name]; b != nil && b.state != hdlForeign {
				b.state = hdlPending
			}
		}
		w.expr(st.Value)
		return false
	case *ast.Return:
		return true // the fn exit discharges: the scope's exit covers it
	case *ast.While:
		w.expr(st.Cond)
		w.loop(st.Body.Items, true)
		return false
	case *ast.Loop:
		// A loop form enters unconditionally: no zero-iteration path joins.
		w.loop(st.Body.Items, false)
		return false
	case *ast.ForStmt:
		w.expr(st.Iter)
		saved := w.shadowPattern(st.Pat)
		w.loop(st.Body.Items, true)
		w.unshadow(saved)
		return false
	case *ast.Defer:
		// The body runs at fn exit: its own bindings face their own block
		// exit, and nothing it does flows back into this path.
		saved := w.env
		w.env = cloneHdlEnv(w.env)
		w.block(st.Block.Items)
		w.env = saved
		return false
	case *ast.Break, *ast.Continue:
		return true // the penetrating exits discharge (design D5)
	case *ast.ScopeRes:
		for _, sb := range st.Binds {
			w.expr(sb.Val)
		}
		saved := map[string]*hdlBind{}
		for _, sb := range st.Binds {
			if sb.Name == "_" {
				continue
			}
			saved[sb.Name] = w.env[sb.Name]
			w.env[sb.Name] = &hdlBind{name: sb.Name, state: hdlForeign}
		}
		w.block(st.Body.Items)
		for n, old := range saved {
			if old != nil {
				w.env[n] = old
			} else {
				delete(w.env, n)
			}
		}
		return false
	case *ast.ExprStmt:
		w.expr(st.Expr)
		return false
	}
	return false
}

// shadowPattern covers a pattern's binding names with foreign bindings and
// returns what they shadowed; unshadow restores. A handle awaiting under a
// shadowing pattern name is not this walk's fire.
func (w *hdlWalk) shadowPattern(p ast.Pattern) map[string]*hdlBind {
	saved := map[string]*hdlBind{}
	for _, n := range patBindNames(p) {
		if n == "_" {
			continue
		}
		saved[n] = w.env[n]
		w.env[n] = &hdlBind{name: n, state: hdlForeign}
	}
	return saved
}

func (w *hdlWalk) unshadow(saved map[string]*hdlBind) {
	for n, old := range saved {
		if old != nil {
			w.env[n] = old
		} else {
			delete(w.env, n)
		}
	}
}

// loop walks a loop form's body: entryJoins marks the zero-iteration path
// (while and for have it, loop does not). The body runs on this path's own
// bindings — a fire inside writes through — and the after-loop state joins
// the fall-through with the skip path under "pending on any path".
func (w *hdlWalk) loop(items []ast.Stmt, entryJoins bool) {
	entry := map[string]hdlState{}
	for _, b := range w.env {
		entry[b.name] = b.state
	}
	w.block(items)
	joins := []map[string]hdlState{}
	if entryJoins {
		joins = append(joins, entry)
	}
	for _, b := range w.env {
		pending := b.state == hdlPending
		for _, snap := range joins {
			if snap[b.name] == hdlPending {
				pending = true
			}
		}
		if pending {
			b.state = hdlPending
		} else {
			b.state = hdlFired
		}
	}
}

// expr walks one expression of the scope body: a call whose head is
// name.await()/name.cancel() is a fire on that handle binding, a TaskExpr
// in any position other than a direct let initializer is a discarded
// handle, and branch points clone and join under "pending on any path".
func (w *hdlWalk) expr(e ast.Expr) {
	switch x := e.(type) {
	case *ast.Ident, *ast.Literal, *ast.Unit:
	case *ast.Unary:
		w.expr(x.X)
	case *ast.Binary:
		w.expr(x.L)
		w.expr(x.R)
	case *ast.Tuple:
		for _, el := range x.Elems {
			w.expr(el)
		}
	case *ast.ListLit:
		for _, el := range x.Elems {
			w.expr(el)
		}
	case *ast.Member:
		w.expr(x.Recv)
	case *ast.Prop:
		w.expr(x.X)
	case *ast.Call:
		if m, ok := x.Fn.(*ast.Member); ok {
			if id, ok := m.Recv.(*ast.Ident); ok && (m.Name == "await" || m.Name == "cancel") {
				if b := w.env[id.Name]; b != nil && b.state != hdlForeign {
					if b.state == hdlFired {
						w.doubleFire(b)
					}
					b.state = hdlFired
					return // the head is a bare name; nothing else to walk
				}
			}
		}
		w.expr(x.Fn)
		for _, a := range x.Args {
			w.expr(a)
		}
	case *ast.Construct:
		if x.Base != nil {
			w.expr(x.Base)
		}
		for _, f := range x.Fields {
			w.expr(f.Value)
		}
	case *ast.If:
		w.expr(x.Cond)
		thenEnv := w.branch(func() { w.block(x.Then.Items) })
		elseEnv := w.branch(func() {
			switch el := x.Else.(type) {
			case nil:
				// No else arm: the clone untouched is the empty path.
			case *ast.BlockExpr:
				w.block(el.Block.Items)
			case *ast.If:
				w.expr(el)
			}
		})
		w.join(thenEnv, elseEnv)
	case *ast.Match:
		w.expr(x.Scrutinee)
		envs := make([]map[string]*hdlBind, 0, len(x.Arms))
		for _, arm := range x.Arms {
			envs = append(envs, w.branch(func() {
				if arm.Guard != nil {
					w.expr(arm.Guard) // the guard sees no arm bindings
				}
				for _, n := range patBindNames(arm.Pat) {
					if n != "_" {
						w.env[n] = &hdlBind{name: n, state: hdlForeign}
					}
				}
				w.expr(arm.Body)
			}))
		}
		// Exhaustiveness held at typing: the arms alone cover every path.
		w.join(envs...)
	case *ast.BlockExpr:
		// A block in expression position runs on this path: its fires flow
		// onward, its own declarations face its own block exit.
		w.block(x.Block.Items)
	case *ast.Closure:
		// The body is lexically inside the scope block, so its creations
		// face judgment here too; its parameters shadow for their extent.
		saved := map[string]*hdlBind{}
		for _, p := range x.Params {
			if p.Name == "_" {
				continue
			}
			saved[p.Name] = w.env[p.Name]
			w.env[p.Name] = &hdlBind{name: p.Name, state: hdlForeign}
		}
		w.block(x.Body.Items)
		w.unshadow(saved)
	case *ast.TaskExpr:
		// A task expression no binding captures is the discarded extreme
		// of the un-awaited form: no path can fire it (design D5).
		w.unawaitedAt(x)
	case *ast.ScopeExpr:
		if x.Timeout != nil {
			w.expr(x.Timeout)
		}
		w.block(x.Body.Items)
	case *ast.SelectExpr:
		for _, cs := range x.Cases {
			w.expr(cs.Source) // a source's await is a fire on the handle
		}
		envs := make([]map[string]*hdlBind, 0, len(x.Cases))
		for _, cs := range x.Cases {
			envs = append(envs, w.branch(func() {
				if !cs.Wildcard && cs.Name != "_" {
					w.env[cs.Name] = &hdlBind{name: cs.Name, state: hdlForeign}
				}
				w.expr(cs.Body)
			}))
		}
		w.join(envs...)
	}
}

// cloneHdlEnv deep-copies a branch point's bindings: a fire inside one arm
// writes the clone, never the path's own state — the join decides.
func cloneHdlEnv(env map[string]*hdlBind) map[string]*hdlBind {
	out := make(map[string]*hdlBind, len(env))
	for k, v := range env {
		bc := *v
		out[k] = &bc
	}
	return out
}

// branch runs f on a clone of the path's bindings and returns the clone.
func (w *hdlWalk) branch(f func()) map[string]*hdlBind {
	saved := w.env
	w.env = cloneHdlEnv(w.env)
	f()
	out := w.env
	w.env = saved
	return out
}

// join merges branch results into the path's bindings: a binding still
// owes after the fork when any path carries it pending; an arm's own
// declarations never reach the join — its block exit judged them.
func (w *hdlWalk) join(envs ...map[string]*hdlBind) {
	for _, b := range w.env {
		pending := false
		for _, e := range envs {
			if eb, ok := e[b.name]; ok && eb.state == hdlPending {
				pending = true
				break
			}
		}
		if pending {
			b.state = hdlPending
		} else {
			b.state = hdlFired
		}
	}
}

// unawaited reports the scope-exit form at the binding's creation.
func (w *hdlWalk) unawaited(b *hdlBind) {
	w.c.fail(b.line, b.col, "E1607", fmt.Sprintf(
		"task handle reaches scope exit un-awaited, or await and cancel both fire — the binding %q reaches the scope body's end on a path with neither await nor cancel; await or cancel the handle on every path from its creation - on early exits the scope's own exit discharge covers it, plain and timeout forms canceling the rest, collectAll joining them",
		b.name))
}

// unawaitedAt reports the discarded-handle extreme at the task keyword.
func (w *hdlWalk) unawaitedAt(x *ast.TaskExpr) {
	w.c.fail(x.Line, x.Col, "E1607", fmt.Sprintf(
		"task handle reaches scope exit un-awaited, or await and cancel both fire — the binding <expr> reaches the scope body's end on a path with neither await nor cancel; await or cancel the handle on every path from its creation - on early exits the scope's own exit discharge covers it, plain and timeout forms canceling the rest, collectAll joining them"))
}

// doubleFire reports the second fire at the binding's creation.
func (w *hdlWalk) doubleFire(b *hdlBind) {
	w.c.fail(b.line, b.col, "E1607", fmt.Sprintf(
		"task handle reaches scope exit un-awaited, or await and cancel both fire — the binding %q takes a cancel after an await on one path, and each handle is one-shot; await or cancel the handle on every path from its creation - on early exits the scope's own exit discharge covers it, plain and timeout forms canceling the rest, collectAll joining them",
		b.name))
}

// checkNestedAccess holds E1613: inside one shared cell's own callback
// (update or read), a member call whose receiver names the same binding
// re-acquires what the callback already holds. The walk is direct and
// per-binding — the callback's own blocks and branches, never into a
// further closure or a concurrent body (chapter 18's stated boundary);
// the callback's own parameters shadow, so a same-named parameter is not
// the binding.
func (c *checker) checkNestedAccess(name, outer string, cl *ast.Closure) {
	params := map[string]bool{}
	for _, p := range cl.Params {
		if p.Name != "_" {
			params[p.Name] = true
		}
	}
	hit := func(id *ast.Ident, method string) {
		if id.Name == name && !params[id.Name] {
			line, col := exprPos(id)
			c.fail(line, col, "E1613", fmt.Sprintf(
				"nested access to one shared value within its own callback — the callback running under %q's %s calls %s on %q again, re-acquiring what it holds; use the callback's own parameter - it is the value - access other shared values freely, and move cross-value work outside the callback",
				name, outer, method, name))
		}
	}
	var block func(items []ast.Stmt)
	var expr func(e ast.Expr)
	expr = func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.Ident, *ast.Literal, *ast.Unit:
		case *ast.Unary:
			expr(x.X)
		case *ast.Binary:
			expr(x.L)
			expr(x.R)
		case *ast.Tuple:
			for _, el := range x.Elems {
				expr(el)
			}
		case *ast.ListLit:
			for _, el := range x.Elems {
				expr(el)
			}
		case *ast.Member:
			expr(x.Recv)
		case *ast.Prop:
			expr(x.X)
		case *ast.Call:
			if m, ok := x.Fn.(*ast.Member); ok {
				if id, ok := m.Recv.(*ast.Ident); ok {
					switch m.Name {
					case "update", "get", "set", "read":
						hit(id, m.Name)
					}
				}
				expr(m.Recv)
			} else {
				expr(x.Fn)
			}
			for _, a := range x.Args {
				expr(a)
			}
		case *ast.Construct:
			if x.Base != nil {
				expr(x.Base)
			}
			for _, f := range x.Fields {
				expr(f.Value)
			}
		case *ast.If:
			expr(x.Cond)
			block(x.Then.Items)
			if x.Else != nil {
				expr(x.Else)
			}
		case *ast.Match:
			expr(x.Scrutinee)
			for _, arm := range x.Arms {
				if arm.Guard != nil {
					expr(arm.Guard)
				}
				expr(arm.Body)
			}
		case *ast.BlockExpr:
			block(x.Block.Items)
		case *ast.Closure, *ast.TaskExpr, *ast.ScopeExpr, *ast.SelectExpr:
			// A further function or concurrent body — the walk's boundary.
		}
	}
	block = func(items []ast.Stmt) {
		for _, s := range items {
			switch st := s.(type) {
			case *ast.Binding:
				expr(st.Init)
			case *ast.Assign:
				expr(st.Value)
			case *ast.Return:
				if st.HasValue {
					expr(st.Value)
				}
			case *ast.ExprStmt:
				expr(st.Expr)
			case *ast.While:
				expr(st.Cond)
				block(st.Body.Items)
			case *ast.Loop:
				block(st.Body.Items)
			case *ast.ForStmt:
				expr(st.Iter)
				block(st.Body.Items)
			case *ast.Defer:
				block(st.Block.Items)
			case *ast.Break, *ast.Continue:
			case *ast.ScopeRes:
				for _, sb := range st.Binds {
					expr(sb.Val)
				}
				block(st.Body.Items)
			}
		}
	}
	block(cl.Body.Items)
}

// checkTaskCaptures runs the capture discipline over one task block,
// before its body walk (design D4: a capture diagnostic precedes any the
// body's own typing would raise). Each capture — a free identifier that
// resolves to a binding outside the block — faces the four judgments in
// the spec's order: var (E1603, whatever its type), a resource whose
// type declares mut self methods beyond release (E1604), a mention of an
// unbounded generic parameter (E1605), unsynchronized gc state (E1602).
// A release-only resource capture is legal and owes nothing further
// here — its single deterministic release stays with the scope-resource
// machine (chapter 13's discipline, untouched by this pass).
func (c *checker) checkTaskCaptures(x *ast.TaskExpr) {
	seen := map[string]bool{}
	for _, u := range c.taskCaptures(x) {
		if seen[u.name] {
			continue
		}
		seen[u.name] = true
		t, ok := c.lookupLocal(u.name)
		if !ok {
			continue // a module-level name — a call or reference, no state
		}
		if c.isVarName(u.name) {
			c.fail(u.line, u.col, "E1603", fmt.Sprintf(
				"task block captures a var binding — %q is var, of whatever type: mutability and concurrency together are a race by definition; carry the state in a shared-state type and reach it through its methods, or copy the value into an immutable binding before the task block",
				u.name))
		}
		if catOf(t) == "resource" {
			if rt, isRec := t.(recordType); isRec {
				if m, has := c.resourceMutSelf(rt.decl); has {
					c.fail(u.line, u.col, "E1604", fmt.Sprintf(
						"task block captures a resource whose type declares mut self methods — %q has type %q, whose method %q takes mut self, a read-modify-write another task can race on; restrict the type to self methods, keep the task out, or materialize the contents and capture those instead",
						u.name, t.String(), m))
				}
			}
			continue // release-only resources capture legally; the rest of the order never applies
		}
		if n, bad := c.mentionsUnboundedParam(t); bad {
			c.fail(u.line, u.col, "E1605", fmt.Sprintf(
				"task block captures a generic-parameter binding without a Shareable bound — %q mentions the generic parameter %q, which carries no Shareable bound, so it admits unsynchronized gc state; declare the parameter with a Shareable bound, or keep the capture to concrete Shareable types",
				u.name, n))
		}
		// The Shareable judgment at a capture is the bound-aware one (T5's
		// machine): a bare generic parameter carrying the bound is
		// Shareable — E1605's own remediation says so — and every other
		// shape answers the closed set.
		if !c.shareableBoundArg(t) {
			// The middle clause names the failure's own shape (the
			// registry's two faces): a gc binding no discipline covers,
			// or a value shape whose parts hold the gc state.
			why := "which no synchronization discipline covers"
			switch x := t.(type) {
			case tupleType:
				why = "a value shape whose parts hold gc state the Shareable set excludes"
			case recordType:
				if x.decl.cat == "value" {
					why = "a value shape whose parts hold gc state the Shareable set excludes"
				}
			case namedType:
				if x.decl.byval {
					why = "a value shape whose parts hold gc state the Shareable set excludes"
				}
			}
			c.fail(u.line, u.col, "E1602", fmt.Sprintf(
				"task block captures unsynchronized gc state — the captured binding %q has type %q, %s; store the list in a Mutex, send it through a Channel, or capture only Shareable data and let the task receive the rest",
				u.name, t.String(), why))
		}
	}
}
