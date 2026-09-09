// The M11 advisory collector (design D4): three W-severity findings that
// hang on the checker's own walk — binding types, call-target effect sets,
// and mock identity are the checker's facts, never re-derived here. The
// hooks fire on every walk; the advRoot gate keeps the finding set to the
// compiled root's own code (the std bodies, the builtin combinators, and
// every dependency module walk silently). The pipeline renders and gates
// the returned findings under the manifest's [vet] postures (design D5);
// collection itself never stops a check.

package typecheck

import (
	"fmt"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/diag"
)

// The three helps — the registry's remediations, verbatim (diagnostics.toml
// :1500/:1509/:1518; the M1 mode: no rewording, one authority).
const (
	helpW1910 = "Mock the custom-effect function on the test's path, or accept the real call deliberately."
	helpW1911 = "Review the called function; if it touches the same shared value, restructure so the callback completes before the nested access."
	helpW1912 = "No change required; the note informs lock-wait chain analysis. Wrap with scope timeout if the wait must be bounded."
)

// pendingW1910 is one unmocked custom-effect call of a test block, waiting
// for the block's settle: the callee's mock identity (E1805's machine —
// canonical module key + name), its display name, the first custom tag's
// display, and the call's anchor.
type pendingW1910 struct {
	identity string
	fn       string
	tag      string
	line     int
	col      int
}

// callAnchor is the callee-name token of a call head (design D4's anchor
// rule): the identifier at a bare head, the member's own name token at a
// qualified head — never the receiver's first token, where exprPos lands.
func callAnchor(e ast.Expr) (int, int) {
	switch h := e.(type) {
	case *ast.Ident:
		return h.Line, h.Col
	case *ast.Member:
		return h.NameLine, h.NameCol
	}
	return exprPos(e)
}

// adviseUnmocked holds W1910's pending face: a call inside a test body
// subtree to a fn whose declared segment names a custom tag — the M10c
// isCustomEffect criterion (any tag beyond the three built-in bare keys),
// here over the checker's canonical keys: one criterion, two consumers.
// The finding waits for the block's settle, where the complete mockSeen
// set decides — a mock anywhere in the block covers every call in it
// (the runtime installs mocks at block start, chapter 20, so "no mock on
// that path" reads at block grain — design's disclosed reading).
func (c *checker) adviseUnmocked(fd *ast.FnDecl, mod string, x *ast.Call) {
	if !c.advRoot || !c.isRoot || !c.advInTest {
		return
	}
	for _, t := range c.fnTags[fd] {
		if builtInTag(t) {
			continue
		}
		identity := c.modKey + "." + fd.Name
		if mod != "" {
			identity = mod + "." + fd.Name
		}
		line, col := callAnchor(x.Fn)
		c.advPend = append(c.advPend, pendingW1910{
			identity: identity, fn: fd.Name, tag: displayTag(t), line: line, col: col,
		})
		return // the first custom tag names the finding
	}
}

// settleW1910 decides a test block's pending unmocked calls against the
// block's complete mock set, at the block's exit (checkTestDecl's swap
// point): the identities are E1805's own calculus, one machine for the
// duplicate-mock error and the unmocked advisory alike.
func (c *checker) settleW1910() {
	for _, p := range c.advPend {
		if c.mockSeen[p.identity] {
			continue
		}
		c.advisories = append(c.advisories, diag.Warning("W1910", fmt.Sprintf(
			"unmocked custom effect in a test — %s carries custom effect %s, no mock in this test", p.fn, p.tag)).
			At(c.file, p.line, p.col).WithHelp(helpW1910))
	}
}

// adviseIndirect holds W1911: inside a synchronized cell's own callback
// (the E1613 site — Mutex/RwLock/Atomic/AtomicRef update or read), a call
// that passes the binding onward, as an argument or in receiver position.
// The walk pierces every nested function body of the callback's subtree —
// closures above all, where E1613's own walk stops — with each layer's
// parameters shadowing within it (inherited down the layers). The direct
// re-acquisition face (the binding as receiver of update/get/set/read at
// the callback's own level) needs no carve-out here: E1613 fires first on
// that walk and stops the check before this one runs.
func (c *checker) adviseIndirect(name string, cl *ast.Closure) {
	if !c.advRoot || !c.isRoot {
		return
	}
	shadow := map[string]bool{}
	for _, p := range cl.Params {
		if p.Name != "_" {
			shadow[p.Name] = true
		}
	}
	var block func(items []ast.Stmt, shadow map[string]bool)
	// passed reports whether the identifier is the binding itself, live at
	// this layer (a same-named parameter shadows the binding within its
	// own subtree and everything under it).
	passed := func(id *ast.Ident, shadow map[string]bool) bool {
		return id.Name == name && !shadow[name]
	}
	var expr func(e ast.Expr, shadow map[string]bool)
	expr = func(e ast.Expr, shadow map[string]bool) {
		switch x := e.(type) {
		case *ast.Ident, *ast.Literal, *ast.Unit:
		case *ast.Unary:
			expr(x.X, shadow)
		case *ast.Binary:
			expr(x.L, shadow)
			expr(x.R, shadow)
		case *ast.Tuple:
			for _, el := range x.Elems {
				expr(el, shadow)
			}
		case *ast.ListLit:
			for _, el := range x.Elems {
				expr(el, shadow)
			}
		case *ast.Member:
			expr(x.Recv, shadow)
		case *ast.Prop:
			expr(x.X, shadow)
		case *ast.Call:
			// The two pass-sites: the receiver of a member call, and a
			// pure identifier argument. The anchor is the call's own name
			// token either way (design D4).
			if m, ok := x.Fn.(*ast.Member); ok {
				if id, ok := m.Recv.(*ast.Ident); ok && passed(id, shadow) {
					c.fireW1911(name, m.Name, m.NameLine, m.NameCol)
				}
				expr(m.Recv, shadow)
			} else {
				expr(x.Fn, shadow)
			}
			for _, a := range x.Args {
				if id, ok := a.(*ast.Ident); ok && passed(id, shadow) {
					if m, ok := x.Fn.(*ast.Member); ok {
						c.fireW1911(name, m.Name, m.NameLine, m.NameCol)
					} else if h, ok := x.Fn.(*ast.Ident); ok {
						c.fireW1911(name, h.Name, h.Line, h.Col)
					}
					break
				}
			}
			for _, a := range x.Args {
				expr(a, shadow)
			}
		case *ast.Construct:
			if x.Base != nil {
				expr(x.Base, shadow)
			}
			for _, f := range x.Fields {
				expr(f.Value, shadow)
			}
		case *ast.If:
			expr(x.Cond, shadow)
			block(x.Then.Items, shadow)
			if x.Else != nil {
				expr(x.Else, shadow)
			}
		case *ast.Match:
			expr(x.Scrutinee, shadow)
			for _, arm := range x.Arms {
				if arm.Guard != nil {
					expr(arm.Guard, shadow)
				}
				expr(arm.Body, shadow)
			}
		case *ast.BlockExpr:
			block(x.Block.Items, shadow)
		case *ast.Closure:
			inner := make(map[string]bool, len(shadow)+len(x.Params))
			for k := range shadow {
				inner[k] = true
			}
			for _, p := range x.Params {
				if p.Name != "_" {
					inner[p.Name] = true
				}
			}
			block(x.Body.Items, inner)
		case *ast.TaskExpr:
			block(x.Body.Items, shadow)
		case *ast.ScopeExpr:
			expr(x.Timeout, shadow)
			block(x.Body.Items, shadow)
		case *ast.SelectExpr:
			for _, cs := range x.Cases {
				expr(cs.Source, shadow)
				expr(cs.Body, shadow)
			}
		}
	}
	block = func(items []ast.Stmt, shadow map[string]bool) {
		for _, s := range items {
			switch st := s.(type) {
			case *ast.Binding:
				expr(st.Init, shadow)
			case *ast.Assign:
				expr(st.Value, shadow)
			case *ast.Return:
				if st.HasValue {
					expr(st.Value, shadow)
				}
			case *ast.ExprStmt:
				expr(st.Expr, shadow)
			case *ast.While:
				expr(st.Cond, shadow)
				block(st.Body.Items, shadow)
			case *ast.Loop:
				block(st.Body.Items, shadow)
			case *ast.ForStmt:
				expr(st.Iter, shadow)
				block(st.Body.Items, shadow)
			case *ast.Defer:
				block(st.Block.Items, shadow)
			case *ast.Break, *ast.Continue:
			case *ast.ScopeRes:
				for _, sb := range st.Binds {
					expr(sb.Val, shadow)
				}
				block(st.Body.Items, shadow)
			}
		}
	}
	block(cl.Body.Items, shadow)
}

// fireW1911 appends one W1911 finding at the passing call's name token.
func (c *checker) fireW1911(binding, callee string, line, col int) {
	c.advisories = append(c.advisories, diag.Warning("W1911", fmt.Sprintf(
		"possible indirect nested access to one shared value — %s is passed to %s inside its own callback", binding, callee)).
		At(c.file, line, col).WithHelp(helpW1911))
}

// adviseBlocking holds W1912: a body directly calls one of the five
// operations whose waits block — the receiver's named type exactly the
// corresponding primitive. trySend and tryReceive are non-blocking and
// never enter the face; the SendOnly/ReceiveOnly views are different
// declarations and drop out by the pointer comparison itself.
func (c *checker) adviseBlocking(m *ast.Member, recv Type) {
	if !c.advRoot || !c.isRoot {
		return
	}
	nt, isN := recv.(namedType)
	if !isN {
		return
	}
	var prim string
	switch nt.decl {
	case semaphoreSum:
		if m.Name == "acquire" {
			prim = "Semaphore"
		}
	case condSum:
		if m.Name == "wait" {
			prim = "Cond"
		}
	case channelSum:
		if m.Name == "send" || m.Name == "receive" {
			prim = "Channel"
		}
	case taskHandleSum:
		if m.Name == "await" {
			prim = "TaskHandle"
		}
	}
	if prim == "" {
		return
	}
	c.advisories = append(c.advisories, diag.Warning("W1912", fmt.Sprintf(
		"function may block on a wait — %s.%s may block", prim, m.Name)).
		At(c.file, m.NameLine, m.NameCol).WithHelp(helpW1912))
}

// Advisories runs the advisory collection over one parsed module — the
// same walk Check runs, with the collector armed for the root ingest
// (design D4/D5). The pipeline runs this face after the check itself has
// passed; a stop mid-walk — impossible on a file that checked clean, and
// reachable only for a direct caller — lands recovered, with whatever
// was collected before the stop.
func Advisories(f *ast.File, file string, mode Mode) []diag.Diagnostic {
	c := newChecker(mode)
	var d *diag.Diagnostic
	var ni *NotImplemented
	defer stopTo(&d, &ni)
	// The same stdlib-first walk Check runs (design D3).
	c.file = stdBodyFile
	c.checkBuiltinCombinators()
	for _, key := range stdImports(f) {
		if sf, ok := StdModule(key); ok {
			c.ingest(sf, key, key, false)
		}
	}
	c.advRoot = true
	c.ingest(f, file, "main", true)
	return c.advisories
}

// AdvisoriesProject collects over a project's graph — the same walk
// CheckProject runs, armed for the root module alone.
func AdvisoriesProject(root *ast.File, rootPath string, deps []Module) []diag.Diagnostic {
	c := newChecker(Project)
	var d *diag.Diagnostic
	var ni *NotImplemented
	defer stopTo(&d, &ni)
	c.file = stdBodyFile
	c.checkBuiltinCombinators()
	for _, m := range deps {
		c.ingest(m.File, m.Path, m.Key, false)
	}
	c.advRoot = true
	c.ingest(root, rootPath, "main", true)
	return c.advisories
}

// AdvisoriesTestRoot collects over one test module as its own graph root
// (M10b design D6's shape) — the same walk CheckTestRoot runs, armed for
// the test module alone.
func AdvisoriesTestRoot(root *ast.File, rootPath, rootKey string, deps []Module) []diag.Diagnostic {
	c := newChecker(Project)
	var d *diag.Diagnostic
	var ni *NotImplemented
	defer stopTo(&d, &ni)
	c.file = stdBodyFile
	c.checkBuiltinCombinators()
	for _, m := range deps {
		c.ingest(m.File, m.Path, m.Key, false)
	}
	c.noMain = true
	c.advRoot = true
	c.ingest(root, rootPath, rootKey, true)
	return c.advisories
}
