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
