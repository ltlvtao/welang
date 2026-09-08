// Package codegen emits textual LLVM IR for the M8 acceptance set (design
// D4/D5 of the stdlib-and-gc change, widening M4's native-vertical set):
// the skeleton module — erased declarations plus one main — with main's
// body being a straight-line statement sequence of let bindings of String
// literals and gc-record constructions, io call statements, and a single
// Ok or Err return tail. Records construct through the gc protocol
// (alloc, map store, root push, field stores — the outer object rooted
// before any nested allocation); Strings ride as double-word operands
// from a constant pool. Everything else that survives type checking
// stops at a boundary What. Emission is a pure function — no paths, no
// counters beyond first-appearance ordering, no time — which is chapter
// 21's same-input-same-output made byte-level.
package codegen

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ltlvtao/welang/internal/ast"
)

// NotImplemented reports one type-clean form outside the acceptance set.
// What slots into `we: %s are not implemented in this reference build
// yet`; the table it draws from is design D4's closed row list.
type NotImplemented struct {
	What string
}

// The boundary Whats. bndMainBody is the M9b v2 vocabulary (design D9):
// the same statement set bndTaskBody names, anchored at main — the M8
// wording retired with the concurrent forms. bndErrPayload and the M4
// rows ride unchanged.
const (
	bndMainBody   = "main bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"
	bndErrPayload = "Err payloads beyond one plain string-literal variant argument"
	bndOtherFns   = "functions other than main in code generation"
	// M10a design D9: the test-module stop — chapter 20's run tower is
	// M10b's (multi-function widening, the we-test runner, mock
	// interception, the virtual clock).
	bndTestModule   = "test modules in code generation (the M10b run tower: test harness, mock interception, virtual clock)"
	bndTopLets      = "top-level value bindings in code generation"
	bndTaskBody     = "task bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"
	bndCallbackBody = "callback bodies beyond straight-line scalar expressions (no control flow, blocking calls, io, or captures of outer bindings)"
)

func bndMain() *NotImplemented     { return &NotImplemented{What: bndMainBody} }
func bndErrPay() *NotImplemented   { return &NotImplemented{What: bndErrPayload} }
func bndTask() *NotImplemented     { return &NotImplemented{What: bndTaskBody} }
func bndCallback() *NotImplemented { return &NotImplemented{What: bndCallbackBody} }

// The runtime symbols emission can reference, in the declare section's
// fixed order (alloc, the root pair, the io pair, the fail tail). M9b
// appends the task-fail tail, the scalar-io pair, the primitive-family
// constructors and methods, the scheduler face, and the overflow
// intrinsics after the M8 rows — a program that uses none of them
// declares none, so the M8 module bytes ride unchanged.
var declareLines = []struct{ sym, line string }{
	{"__we_alloc", "declare ptr @__we_alloc(i64)"},
	{"__we_root_push", "declare void @__we_root_push(ptr)"},
	{"__we_root_pop", "declare void @__we_root_pop()"},
	{"__we_println", "declare void @__we_println(ptr, i64)"},
	{"__we_print", "declare void @__we_print(ptr, i64)"},
	{"__we_fail", "declare void @__we_fail(ptr, i64) noreturn"},
	{"__we_task_fail", "declare void @__we_task_fail(ptr) noreturn"},
	{"__we_println_i64", "declare void @__we_println_i64(i64)"},
	{"__we_print_i64", "declare void @__we_print_i64(i64)"},
	{"__we_prim_new_mutex", "declare ptr @__we_prim_new_mutex(i64)"},
	{"__we_prim_new_rwlock", "declare ptr @__we_prim_new_rwlock(i64)"},
	{"__we_prim_new_atomic", "declare ptr @__we_prim_new_atomic(i64)"},
	{"__we_prim_new_sem", "declare ptr @__we_prim_new_sem(i64)"},
	{"__we_prim_new_chan", "declare ptr @__we_prim_new_chan(i64, i64, ptr)"},
	{"__we_prim_new_cond", "declare ptr @__we_prim_new_cond(ptr)"},
	{"__we_prim_update", "declare i64 @__we_prim_update(ptr, ptr, ptr)"},
	{"__we_prim_set", "declare void @__we_prim_set(ptr, i64)"},
	{"__we_prim_get", "declare i64 @__we_prim_get(ptr)"},
	{"__we_prim_read", "declare i64 @__we_prim_read(ptr, ptr, ptr)"},
	{"__we_cond_wait", "declare i64 @__we_cond_wait(ptr, ptr, ptr)"},
	{"__we_sig_current", "declare ptr @__we_sig_current()"},
	{"__we_sig_check", "declare i64 @__we_sig_check(ptr)"},
	{"__we_sig_wait", "declare i64 @__we_sig_wait(ptr)"},
	{"__we_select_add_await", "declare void @__we_select_add_await(ptr, ptr)"},
	{"__we_sem_acquire", "declare i64 @__we_sem_acquire(ptr)"},
	{"__we_sem_try_acquire", "declare i64 @__we_sem_try_acquire(ptr)"},
	{"__we_sem_release", "declare void @__we_sem_release(ptr)"},
	{"__we_sem_count", "declare i64 @__we_sem_count(ptr)"},
	{"__we_chan_send", "declare i64 @__we_chan_send(ptr, i64)"},
	{"__we_chan_recv", "declare i64 @__we_chan_recv(ptr, ptr)"},
	{"__we_chan_try_send", "declare i64 @__we_chan_try_send(ptr, i64)"},
	{"__we_chan_try_recv", "declare i64 @__we_chan_try_recv(ptr, ptr)"},
	{"__we_chan_close", "declare i64 @__we_chan_close(ptr)"},
	{"__we_cond_signal", "declare void @__we_cond_signal(ptr)"},
	{"__we_cond_broadcast", "declare void @__we_cond_broadcast(ptr)"},
	{"__we_handle_await", "declare i64 @__we_handle_await(ptr, ptr)"},
	{"__we_handle_cancel", "declare i64 @__we_handle_cancel(ptr)"},
	{"__we_task_new", "declare ptr @__we_task_new(ptr, ptr)"},
	{"__we_yield", "declare i64 @__we_yield()"},
	{"__we_scope_enter", "declare ptr @__we_scope_enter(i64, i64)"},
	{"__we_scope_leave", "declare i64 @__we_scope_leave(ptr)"},
	{"__we_select_new", "declare ptr @__we_select_new()"},
	{"__we_select_add_recv", "declare void @__we_select_add_recv(ptr, ptr)"},
	{"__we_select_add_send", "declare void @__we_select_add_send(ptr, ptr, i64)"},
	{"__we_select_park", "declare i64 @__we_select_park(ptr)"},
	{"__we_select_value", "declare i64 @__we_select_value(ptr)"},
	{"llvm.sadd.with.overflow.i64", "declare { i64, i1 } @llvm.sadd.with.overflow.i64(i64, i64)"},
	{"llvm.ssub.with.overflow.i64", "declare { i64, i1 } @llvm.ssub.with.overflow.i64(i64, i64)"},
	{"llvm.smul.with.overflow.i64", "declare { i64, i1 } @llvm.smul.with.overflow.i64(i64, i64)"},
}

// A record field's emission shape: String is the double word (16 bytes,
// not a gc reference — every M8 buffer is a constant, design D5), a gc
// record reference is one pointer slot, and the 8-byte scalars ride the
// construction-argument literal positions only.
type fieldKind int

const (
	fkStr fieldKind = iota
	fkRef
	fkScalar
)

type fieldSlot struct {
	off   int
	kind  fieldKind
	typ   string // the referenced record's name on fkRef
	isRef bool
}

// strConst is one constant-pool entry; String lets carry their decoded
// bytes until a use interns them (an unused let emits nothing).
type strConst struct {
	name string
	data string
}

type strBinding struct {
	data   string
	length int
}

type gcBinding struct {
	rec string
	reg string
}

// emitter is one Emit run's state: the collected tables, the constant
// pool, the instruction stream, and the fresh-value counter. The M9b
// additions ride alongside: the per-body context (which boundary word a
// stop reports), the scalar and primitive environments, and the thunk and
// overflow-report side streams (callback bodies and panic reports live
// outside the main instruction stream, in their own defines and globals).
type emitter struct {
	sums    map[string]map[string][]ast.TypeRef
	records map[string]*ast.RecordDecl
	order   []*ast.RecordDecl
	mainRet ast.TypeRef

	strEnv map[string]strBinding
	gcEnv  map[string]gcBinding

	strs     []strConst
	strPool  map[string]string
	usedRecs map[string]bool
	declUsed map[string]bool
	errConst string

	body   strings.Builder
	fresh  int
	pushes int

	// M9b state.
	ctx     bodyCtx
	scalars map[string]scalarSlot
	sums2   map[string]sumSlot
	prims   map[string]string
	thunks  []string // finished callback and task defines
	ovfs    []string // overflow report constants
	blocks  int      // fresh block-label suffix

	concAlias map[string]bool // the std.concurrent import's alias set
	defers    []ast.Block     // a body's defer blocks, emission inverted
	panics    []string        // NUL-terminated panic-message constants
	caps      *captureSet     // the task body being emitted reads these
	chanDescs []string        // channel element-bitmap descriptor globals
	envMaps   []string        // task environment-block bitmap globals
	cdsc      int             // fresh channel-element descriptor count
}

// captureSet is one task block's environment face: the names the body
// reads from the enclosing scope, each with its env-block slot. Scalars
// ride plain i64 slots; primitive pointers are traced words (the env
// block's bitmap marks them).
type captureSet struct {
	names []string
	prim  []bool
	ptr   string // the env block operand ("%v12"); empty outside a task
}

func (c *captureSet) slot(name string) (int, bool) {
	if c == nil {
		return 0, false
	}
	for i, n := range c.names {
		if n == name {
			return i, true
		}
	}
	return 0, false
}

// bodyCtx is which straight-line set is being emitted — the boundary
// word a stop reports depends on it (design D9): the same out-of-set
// form names the task body inside a task and the main body elsewhere.
type bodyCtx int

const (
	ctxMain bodyCtx = iota
	ctxTask
	ctxCallback
)

func (e *emitter) bnd() *NotImplemented {
	switch e.ctx {
	case ctxTask:
		return bndTask()
	case ctxCallback:
		return bndCallback()
	default:
		return bndMain()
	}
}

// scalarSlot is one numeric binding: a var owns an alloca (assignment
// stores into it); a let holds its operand directly (SSA value or
// immediate). Floats ride the same slots with their own arithmetic.
type scalarSlot struct {
	operand string // the let's value ("%v3" or "5"); empty for a var
	alloca  string // the var's memory slot ("%v7"); empty for a let
	isVar   bool
	isFloat bool
}

// sumSlot is one Option/Result value: the two-word {tag, payload} of
// design D8, as two i64 stack slots the emitter addresses by name. The
// variant table maps the runtime's return value to the source-level name
// the match arms carry (receive yields None=0/Some=1, the try trio their
// three-state codes, await and scope Ok=0/Err=1); the error faces carry
// what `?` reports — a panic-message C string on await, the fixed
// TimedOut line on a timeout scope.
type sumSlot struct {
	tag      string   // i64 alloca holding the discriminant
	pay      string   // i64 alloca holding the payload word
	variants []string // return value → variant name, index-addressed
	errPanic bool     // the Err payload is a panic-message C-string pointer
	errMsg   string   // a static Err report line (the `?` main tail writes it)
}

func (e *emitter) inst(s string)  { e.body.WriteString("  " + s + "\n") }
func (e *emitter) label(s string) { e.body.WriteString(s + ":\n") }
func (e *emitter) value() string  { v := "v" + strconv.Itoa(e.fresh); e.fresh++; return v }
func (e *emitter) use(sym string) { e.declUsed[sym] = true }

// Emit renders f as textual LLVM IR under the module name (the manifest
// name at the call site). A non-nil NotImplemented means f was type-clean
// but outside the acceptance set; the IR string is then empty.
func Emit(f *ast.File, module string) (string, *NotImplemented) {
	// Pass one: the module's sum table (the Err attribution), the record
	// table (construction layouts), and main. Declarations that never
	// reach a runtime value — sums, newtypes, interfaces, impls, effects
	// — emit zero IR; a record's type and map lines appear only when a
	// construction in main uses it.
	sums := make(map[string]map[string][]ast.TypeRef)
	records := make(map[string]*ast.RecordDecl)
	var order []*ast.RecordDecl
	var main *ast.FnDecl
	concAlias := make(map[string]bool)
	for _, it := range f.Items {
		switch d := it.(type) {
		case *ast.SumDecl:
			variants := make(map[string][]ast.TypeRef, len(d.Variants))
			for _, v := range d.Variants {
				variants[v.Name] = v.Payload
			}
			sums[d.Name] = variants
		case *ast.FnDecl:
			if d.Name == "main" && main == nil {
				main = d
				continue
			}
			return "", &NotImplemented{What: bndOtherFns}
		case *ast.TopLet:
			return "", &NotImplemented{What: bndTopLets}
		case *ast.RecordDecl:
			if _, seen := records[d.Name]; !seen {
				order = append(order, d)
			}
			records[d.Name] = d
		case *ast.NewtypeDecl:
			// Newtypes are zero-cost wrappers (chapter 8): the layout
			// erases and their value expressions stop at the body
			// boundary within the accepted shapes.
			continue
		case *ast.InterfaceDecl, *ast.ImplDecl:
			// Interfaces and impls erase (design D11 of the generics
			// change): a declaration-level fact only — the member sets
			// live in the type stage — so neither emits IR. An explicit
			// case, not a silent fall-through: a future form arriving
			// here must decide, never vanish.
			continue
		case *ast.EffectDecl:
			// Effect declarations erase (M7 design D10): chapter 16 is a
			// compile-time discipline — the check stage consumes every
			// segment, and the tag names reach no IR and no runtime face.
			continue
		case *ast.TestDecl:
			// Chapter 20's test block stops here (M10a design D9): the
			// run tower — the harness, mock interception, the virtual
			// clock — is M10b's. An explicit case, never a silent skip.
			// MockDecl and advanceTime are unreachable below this stop by
			// construction, so neither needs a walk case: a mock parses
			// only at a test body's own depth (the parser's E1802 holds
			// every other position), and advanceTime's legal positions
			// lie inside the test extent (E1806 outside it) — both live
			// strictly inside the TestDecl this walk stops at.
			return "", &NotImplemented{What: bndTestModule}
		case *ast.Import:
			// Only the std segment erases (design D4): a std import item
			// leaves no IR trace and enables the io calls; any other
			// import means the emitted program would span modules — the
			// honest stop is the other-functions row. The concurrent
			// import's alias rides along for the constructor face.
			if len(d.Path) > 0 && d.Path[0] == "std" {
				if len(d.Path) == 2 && d.Path[1] == "concurrent" {
					a := d.Alias
					if a == "" {
						a = d.Path[len(d.Path)-1]
					}
					concAlias[a] = true
				}
				continue
			}
			return "", &NotImplemented{What: bndOtherFns}
		}
	}
	if main == nil {
		// Defensive: Project-mode typecheck rejects a missing main (E1305).
		return "", bndMain()
	}

	e := &emitter{
		sums: sums, records: records, order: order, mainRet: main.Ret,
		strEnv:    make(map[string]strBinding),
		gcEnv:     make(map[string]gcBinding),
		strPool:   make(map[string]string),
		usedRecs:  make(map[string]bool),
		declUsed:  make(map[string]bool),
		ctx:       ctxMain,
		scalars:   make(map[string]scalarSlot),
		sums2:     make(map[string]sumSlot),
		prims:     make(map[string]string),
		concAlias: concAlias,
	}

	// Pass two: the statement sequence. The single return is the tail;
	// a return anywhere before the end is a control-flow shape outside
	// the straight-line set.
	items := main.Body.Items
	if len(items) == 0 {
		return "", bndMain()
	}
	for i, st := range items {
		if i == len(items)-1 {
			ret, ok := st.(*ast.Return)
			if !ok || !ret.HasValue {
				return "", bndMain()
			}
			if ni := e.emitTail(ret.Value); ni != nil {
				return "", ni
			}
			continue
		}
		if ni := e.emitStmt(st); ni != nil {
			return "", ni
		}
	}
	return e.render(module), nil
}

// emitStmt emits one statement of the accepted sequence. The M8 rows ride
// unchanged (String/gc-record lets, io calls); M9b adds the numeric
// bindings, assignments, the primitive constructors and method calls,
// scalar io arguments, and the concurrent statement forms (while, if,
// match, defer, task/scope/select, `?`).
func (e *emitter) emitStmt(st ast.Stmt) *NotImplemented {
	switch s := st.(type) {
	case *ast.Binding:
		if s.Pat != nil || (s.Kw != "let" && s.Kw != "var") {
			return e.bnd()
		}
		if s.Kw == "var" {
			return e.emitVarBinding(s)
		}
		return e.emitLetBinding(s)
	case *ast.Assign:
		slot, ok := e.scalars[s.Name]
		if !ok || !slot.isVar {
			return e.bnd()
		}
		op, isF, ni := e.emitNumExpr(s.Value)
		if ni != nil {
			return ni
		}
		if isF != slot.isFloat {
			return e.bnd()
		}
		if slot.isFloat {
			e.inst(fmt.Sprintf("store double %s, ptr %s", op, slot.alloca))
		} else {
			e.inst(fmt.Sprintf("store i64 %s, ptr %s", op, slot.alloca))
		}
		return nil
	case *ast.While:
		return e.emitWhile(s)
	case *ast.Defer:
		// Inverted emission: the block runs at body exit, reverse
		// registration order (the main tail and every task thunk drain
		// the collected list before their ret).
		e.defers = append(e.defers, s.Block)
		return nil
	case *ast.ExprStmt:
		return e.emitExprStmt(s.Expr)
	default:
		return e.bnd()
	}
}

// emitLetBinding emits `let name [: type] = init`. Numeric literals and
// expressions, strings, records, re-binding by name (a pure alias of any
// environment face), the concurrent constructors, and the value forms of
// task/scope/select and `?` each bind; an io call binds only as the
// discard (`let _ =`).
func (e *emitter) emitLetBinding(s *ast.Binding) *NotImplemented {
	switch init := s.Init.(type) {
	case *ast.Literal:
		switch init.Kind {
		case "string":
			data, ok := decodeStringLiteral(init.Text)
			if !ok {
				return e.bnd() // interpolation has no M9b emission
			}
			if s.Name != "_" {
				e.strEnv[s.Name] = strBinding{data: data, length: len(data)}
			}
			return nil
		case "int", "bool", "float":
			op, isF, ok := e.numImmediate(init)
			if !ok {
				return e.bnd()
			}
			if s.Name != "_" {
				e.scalars[s.Name] = scalarSlot{operand: op, isFloat: isF}
			}
			return nil
		default:
			return e.bnd()
		}
	case *ast.Ident:
		// A pure re-binding (`let _ = m`, `let _ = r`): alias the name's
		// environment face; nothing emits.
		if s.Name == "_" {
			return nil
		}
		if b, ok := e.strEnv[init.Name]; ok {
			e.strEnv[s.Name] = b
			return nil
		}
		if v, ok := e.scalars[init.Name]; ok {
			e.scalars[s.Name] = v
			return nil
		}
		if v, ok := e.sums2[init.Name]; ok {
			e.sums2[s.Name] = v
			return nil
		}
		if v, ok := e.prims[init.Name]; ok {
			e.prims[s.Name] = v
			return nil
		}
		if v, ok := e.gcEnv[init.Name]; ok {
			e.gcEnv[s.Name] = v
			return nil
		}
		return e.bnd()
	case *ast.Construct:
		reg, ni := e.emitConstruct(init)
		if ni != nil {
			return ni
		}
		if s.Name != "_" {
			e.gcEnv[s.Name] = gcBinding{rec: init.Name, reg: reg}
		}
		return nil
	case *ast.Binary:
		op, _, ni := e.emitNumExpr(init)
		if ni != nil {
			return ni
		}
		if s.Name != "_" {
			e.scalars[s.Name] = scalarSlot{operand: op}
		}
		return nil
	case *ast.Prop:
		res, ni := e.emitQuestion(init)
		if ni != nil {
			return ni
		}
		return e.bindResult(s.Name, res)
	case *ast.TaskExpr:
		res, ni := e.emitTask(init, s.Typ)
		if ni != nil {
			return ni
		}
		return e.bindResult(s.Name, res)
	case *ast.ScopeExpr:
		res, ni := e.emitScope(init, true)
		if ni != nil {
			return ni
		}
		return e.bindResult(s.Name, res)
	case *ast.SelectExpr:
		res, ni := e.emitSelect(init)
		if ni != nil {
			return ni
		}
		return e.bindResult(s.Name, res)
	case *ast.Call:
		// The concurrent constructors bind by name; other method calls
		// bind their result; a channel's element domain rides the
		// annotation.
		res, ni := e.emitCall(init, s.Typ)
		if ni != nil {
			return ni
		}
		return e.bindResult(s.Name, res)
	default:
		return e.bnd()
	}
}

// bindResult stores one call-shaped value under the binding's name (the
// discard `_` keeps no environment face).
func (e *emitter) bindResult(name string, res callResult) *NotImplemented {
	switch res.kind {
	case ckVoid:
		return nil
	case ckIo:
		if name != "_" {
			return e.bnd()
		}
		return nil
	case ckI64:
		if name != "_" {
			e.scalars[name] = scalarSlot{operand: res.i64, isFloat: res.isFloat}
		}
		return nil
	case ckPrim:
		if name != "_" {
			e.prims[name] = res.i64
		}
		return nil
	case ckSum:
		if name != "_" {
			e.sums2[name] = res.sum
		}
		return nil
	}
	return e.bnd()
}

// emitExprStmt emits one statement-position expression: the call rows of
// the M8 face plus the concurrent statement forms — if, match, select,
// bare task/scope statements, and the discarded `?`.
func (e *emitter) emitExprStmt(x ast.Expr) *NotImplemented {
	switch v := x.(type) {
	case *ast.Call:
		_, ni := e.emitCall(v, nil)
		return ni
	case *ast.If:
		return e.emitIf(v)
	case *ast.Match:
		return e.emitMatch(v)
	case *ast.SelectExpr:
		_, ni := e.emitSelect(v)
		return ni
	case *ast.ScopeExpr:
		_, ni := e.emitScope(v, false)
		return ni
	case *ast.TaskExpr:
		_, ni := e.emitTask(v, nil)
		return ni
	case *ast.Prop:
		_, ni := e.emitQuestion(v)
		return ni
	default:
		return e.bnd()
	}
}

// emitIoCall emits `qual.println(arg)` / `qual.print(arg)`. The qualifier
// is any name — resolution happened at the type stage; a qualifier that
// the body binds locally is a member call on that value instead (a
// record field call is outside the set).
func (e *emitter) emitIoCall(call *ast.Call) *NotImplemented {
	fn, ok := call.Fn.(*ast.Member)
	if !ok {
		return bndMain()
	}
	qual, ok := fn.Recv.(*ast.Ident)
	if !ok {
		return bndMain()
	}
	if e.isLocalName(qual.Name) {
		return bndMain()
	}
	var sym string
	switch fn.Name {
	case "println":
		sym = "__we_println"
	case "print":
		sym = "__we_print"
	default:
		return bndMain()
	}
	if len(call.Args) != 1 {
		return bndMain()
	}
	p, l, ni := e.emitStringExpr(call.Args[0])
	if ni != nil {
		return ni
	}
	e.use(sym)
	e.inst(fmt.Sprintf("call void @%s(ptr %s, i64 %s)", sym, p, l))
	return nil
}

// --- the M9b emission set ---------------------------------------------------

// callKind classifies a call's result for the binding site.
type callKind int

const (
	ckIo   callKind = iota // an io call, emitted by the M8 path
	ckVoid                 // emitted, no value
	ckI64                  // a scalar value in the i64 domain (or a double)
	ckPrim                 // a primitive pointer (gc-rooted at construction)
	ckSum                  // an Option/Result two-slot value
)

type callResult struct {
	kind    callKind
	i64     string // the ckI64/ckPrim operand
	isFloat bool
	sum     sumSlot // the ckSum slots
}

// isLocalName reports whether name is bound in the current body — locals
// shadow the io qualifier segment.
func (e *emitter) isLocalName(name string) bool {
	if _, ok := e.strEnv[name]; ok {
		return true
	}
	if _, ok := e.gcEnv[name]; ok {
		return true
	}
	if _, ok := e.prims[name]; ok {
		return true
	}
	if _, ok := e.scalars[name]; ok {
		return true
	}
	if _, ok := e.sums2[name]; ok {
		return true
	}
	_, ok := e.caps.slot(name)
	return ok
}

// numImmediate renders a numeric literal as an IR operand: ints and bools
// as i64 immediates, floats as the exact double hex form.
func (e *emitter) numImmediate(l *ast.Literal) (string, bool, bool) {
	if l.Kind == "float" {
		text := l.Text
		for _, suf := range []string{"f64", "f32", "F64", "F32"} {
			if strings.HasSuffix(text, suf) {
				text = text[:len(text)-len(suf)]
				break
			}
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return "", false, false
		}
		return fmt.Sprintf("double 0x%016X", math.Float64bits(f)), true, true
	}
	imm, ok := scalarImmediate(l)
	return imm, false, ok
}

// emitNumExpr emits one numeric operand (i64 domain or double), returning
// the operand and its domain. Arithmetic carries the E0502 trap: the
// intrinsic reports overflow, a failing branch runs the task-panic tail,
// and only the checked value flows on (design D8).
func (e *emitter) emitNumExpr(x ast.Expr) (string, bool, *NotImplemented) {
	switch v := x.(type) {
	case *ast.Literal:
		op, isF, ok := e.numImmediate(v)
		if !ok {
			return "", false, e.bnd()
		}
		return op, isF, nil
	case *ast.Ident:
		if s, ok := e.scalars[v.Name]; ok {
			if s.isVar {
				return e.loadNum(s.alloca, s.isFloat), s.isFloat, nil
			}
			return s.operand, s.isFloat, nil
		}
		if _, ok := e.caps.slot(v.Name); ok {
			// A task body reads an enclosing scalar through its env slot.
			op, isF, ni := e.loadCapture(v.Name)
			return op, isF, ni
		}
		return "", false, e.bnd()
	case *ast.Binary:
		switch v.Op {
		case "+", "-", "*":
			return e.emitOverflowArith(v)
		case "<", "<=", ">", ">=", "==", "!=":
			return e.emitCompare(v)
		case "&&", "||":
			return e.emitLogic(v)
		}
		return "", false, e.bnd()
	default:
		return "", false, e.bnd()
	}
}

func (e *emitter) loadNum(slot string, isFloat bool) string {
	typ := "i64"
	if isFloat {
		typ = "double"
	}
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = load %s, ptr %s", v, typ, slot))
	return "%" + v
}

// emitOverflowArith emits a checked + - * (the intrinsic + domain
// re-check + panic block of design D8); floats ride plain IEEE
// arithmetic — overflow there is an Inf, chapter 5's domain.
func (e *emitter) emitOverflowArith(b *ast.Binary) (string, bool, *NotImplemented) {
	a, af, ni := e.emitNumExpr(b.L)
	if ni != nil {
		return "", false, ni
	}
	c, cf, ni := e.emitNumExpr(b.R)
	if ni != nil {
		return "", false, ni
	}
	if af != cf {
		return "", false, e.bnd()
	}
	if af {
		op := map[string]string{"+": "fadd", "-": "fsub", "*": "fmul"}[b.Op]
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = %s double %s, %s", v, op, a, c))
		return "%" + v, true, nil
	}
	intr := map[string]string{
		"+": "llvm.sadd.with.overflow.i64",
		"-": "llvm.ssub.with.overflow.i64",
		"*": "llvm.smul.with.overflow.i64",
	}[b.Op]
	pair := e.value()
	e.use(intr)
	e.inst(fmt.Sprintf("%%%s = call { i64, i1 } @%s(i64 %s, i64 %s)", pair, intr, a, c))
	res := e.value()
	e.inst(fmt.Sprintf("%%%s = extractvalue { i64, i1 } %%%s, 0", res, pair))
	ov := e.value()
	e.inst(fmt.Sprintf("%%%s = extractvalue { i64, i1 } %%%s, 1", ov, pair))
	bl := e.blocks
	e.blocks++
	e.inst(fmt.Sprintf("br i1 %%%s, label %%ovf%d, label %%cont%d", ov, bl, bl))
	e.label(fmt.Sprintf("ovf%d", bl))
	e.use("__we_task_fail")
	cn := fmt.Sprintf("@.ovf%d", len(e.ovfs))
	e.ovfs = append(e.ovfs, cn+` = private unnamed_addr constant [17 x i8] c"integer overflow\00"`)
	e.inst(fmt.Sprintf("call void @__we_task_fail(ptr %s)", cn))
	e.inst("unreachable")
	e.label(fmt.Sprintf("cont%d", bl))
	return "%" + res, false, nil
}

var icmpPred = map[string]string{"<": "slt", "<=": "sle", ">": "sgt", ">=": "sge", "==": "eq", "!=": "ne"}
var fcmpPred = map[string]string{"<": "olt", "<=": "ole", ">": "ogt", ">=": "oge", "==": "oeq", "!=": "one"}

// emitCompare widens the boolean result into the i64 domain (design D8's
// Bool-as-i64 register rule).
func (e *emitter) emitCompare(b *ast.Binary) (string, bool, *NotImplemented) {
	a, af, ni := e.emitNumExpr(b.L)
	if ni != nil {
		return "", false, ni
	}
	c, cf, ni := e.emitNumExpr(b.R)
	if ni != nil {
		return "", false, ni
	}
	if af != cf {
		return "", false, e.bnd()
	}
	v := e.value()
	if af {
		e.inst(fmt.Sprintf("%%%s = fcmp %s double %s, %s", v, fcmpPred[b.Op], a, c))
	} else {
		e.inst(fmt.Sprintf("%%%s = icmp %s i64 %s, %s", v, icmpPred[b.Op], a, c))
	}
	z := e.value()
	e.inst(fmt.Sprintf("%%%s = zext i1 %%%s to i64", z, v))
	return "%" + z, false, nil
}

// emitLogic emits && / || as and/or over the i64-domain booleans. Both
// operands are side-effect-free numeric forms by construction (calls
// stop at the numeric expression set), so the non-branching form equals
// the short-circuit one here.
func (e *emitter) emitLogic(b *ast.Binary) (string, bool, *NotImplemented) {
	a, af, ni := e.emitNumExpr(b.L)
	if ni != nil {
		return "", false, ni
	}
	c, cf, ni := e.emitNumExpr(b.R)
	if ni != nil {
		return "", false, ni
	}
	if af || cf {
		return "", false, e.bnd()
	}
	av := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp ne i64 %s, 0", av, a))
	cv := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp ne i64 %s, 0", cv, c))
	op := "and i1"
	if b.Op == "||" {
		op = "or i1"
	}
	r := e.value()
	e.inst(fmt.Sprintf("%%%s = %s %%%s, %%%s", r, op, av, cv))
	z := e.value()
	e.inst(fmt.Sprintf("%%%s = zext i1 %%%s to i64", z, r))
	return "%" + z, false, nil
}

// emitVarBinding emits a numeric var: the alloca is the name's storage,
// assignment stores into it. The narrow int widths ride the i64 domain
// (sign-extended values; their arithmetic is outside the M9b set).
func (e *emitter) emitVarBinding(s *ast.Binding) *NotImplemented {
	isF := false
	t, ok := s.Typ.(*ast.NamedType)
	if !ok || t.Qual != "" || len(t.Args) != 0 {
		return e.bnd()
	}
	switch t.Name {
	case "Int64", "Int32", "Int16", "Int8", "UInt64", "UInt32", "UInt16", "UInt8", "Bool":
	case "Float64":
		isF = true
	default:
		return e.bnd()
	}
	op, of, ni := e.emitNumExpr(s.Init)
	if ni != nil {
		return ni
	}
	if of != isF {
		return e.bnd()
	}
	typ := "i64"
	if isF {
		typ = "double"
	}
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca %s", v, typ))
	if isF {
		e.inst(fmt.Sprintf("store double %s, ptr %%%s", op, v))
	} else {
		e.inst(fmt.Sprintf("store i64 %s, ptr %%%s", op, v))
	}
	if s.Name != "_" {
		e.scalars[s.Name] = scalarSlot{alloca: "%" + v, isVar: true, isFloat: isF}
	}
	return nil
}

// primCtors maps the concurrent constructor face to its ABI symbol and
// argument shape (scalar value, primitive pointer, or channel triple).
type primCtorSpec struct {
	sym  string
	kind int // 0: (i64), 1: (ptr), 2: (i64 cap, i64 slot, ptr desc)
}

var primCtors = map[string]primCtorSpec{
	"Mutex":     {"__we_prim_new_mutex", 0},
	"RwLock":    {"__we_prim_new_rwlock", 0},
	"Atomic":    {"__we_prim_new_atomic", 0},
	"Semaphore": {"__we_prim_new_sem", 0},
	"Cond":      {"__we_prim_new_cond", 1},
	"channel":   {"__we_prim_new_chan", 2},
}

// emitPrimCtor emits one constructor call. Every primitive object is a
// gc block, so the binding roots it like a record (the M8 push/pop
// protocol rides unchanged). A channel's element domain rides the
// binding's annotation: scalar elements take the untraced 8-byte slot,
// a gc-record element takes a one-slot traced descriptor (the buffer's
// element bitmap — the channel chain roots in-flight records).
func (e *emitter) emitPrimCtor(spec primCtorSpec, args []ast.Expr, typ ast.TypeRef) (string, *NotImplemented) {
	if len(args) != 1 {
		return "", e.bnd()
	}
	v := e.value()
	switch spec.kind {
	case 0:
		op, isF, ni := e.emitNumExpr(args[0])
		if ni != nil {
			return "", ni
		}
		if isF {
			return "", e.bnd()
		}
		e.use(spec.sym)
		e.inst(fmt.Sprintf("%%%s = call ptr @%s(i64 %s)", v, spec.sym, op))
	case 1:
		arg, ok := args[0].(*ast.Ident)
		if !ok {
			return "", e.bnd()
		}
		ptr, ok := e.prims[arg.Name]
		if !ok {
			return "", e.bnd()
		}
		e.use(spec.sym)
		e.inst(fmt.Sprintf("%%%s = call ptr @%s(ptr %s)", v, spec.sym, ptr))
	case 2:
		op, isF, ni := e.emitNumExpr(args[0])
		if ni != nil || isF {
			return "", e.bnd()
		}
		desc := "null"
		if ref := chanElemRecord(typ, e.records); ref != "" {
			e.usedRecs[ref] = true
			// The element bitmap: one traced slot per element word — a
			// gc-record element rides the buffer as a pointer, so the
			// collector walks channel → buffer → record.
			cn := fmt.Sprintf("@.cdesc%d", e.cdsc)
			e.cdsc++
			desc = cn
			e.chanDescs = append(e.chanDescs, cn+" = private unnamed_addr constant [1 x i64] [i64 1]")
		}
		e.use(spec.sym)
		e.inst(fmt.Sprintf("%%%s = call ptr @%s(i64 %s, i64 8, ptr %s)", v, spec.sym, op, desc))
	}
	ptr := "%" + v
	e.use("__we_root_push")
	e.pushes++
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", ptr))
	return ptr, nil
}

// chanElemRecord resolves a channel annotation `alias.Channel<T>` to the
// gc record T names, or "" when the element domain is not a traced
// record reference.
func chanElemRecord(typ ast.TypeRef, records map[string]*ast.RecordDecl) string {
	n, ok := typ.(*ast.NamedType)
	if !ok || n.Name != "Channel" || len(n.Args) != 1 {
		return ""
	}
	elem, ok := n.Args[0].(*ast.NamedType)
	if !ok || elem.Qual != "" || len(elem.Args) != 0 {
		return ""
	}
	r, ok := records[elem.Name]
	if !ok || r.Cat != "gc" || len(r.TypeParams) != 0 {
		return ""
	}
	return elem.Name
}

// argIsScalar is the static probe deciding println's argument domain
// before anything emits: a numeric form routes to the i64 renderer, a
// String form to the M8 byte-pair path.
func (e *emitter) argIsScalar(x ast.Expr) bool {
	switch v := x.(type) {
	case *ast.Literal:
		return v.Kind == "int" || v.Kind == "bool"
	case *ast.Ident:
		_, ok := e.scalars[v.Name]
		return ok
	case *ast.Binary:
		return true
	default:
		return false
	}
}

// emitCall dispatches one call: the bare panic and cancel-signal forms,
// the concurrent constructors (alias-qualified; a channel's element
// domain rides the binding's annotation), the primitive methods, and the
// io pair (scalar argument through the i64 renderer, String argument
// through the M8 path). typ is the enclosing let's annotation, nil at
// statement position.
func (e *emitter) emitCall(call *ast.Call, typ ast.TypeRef) (callResult, *NotImplemented) {
	if id, ok := call.Fn.(*ast.Ident); ok {
		switch id.Name {
		case "panic":
			if len(call.Args) != 1 {
				return callResult{}, e.bnd()
			}
			return e.emitPanic(call.Args[0])
		case "currentCancelSignal":
			if len(call.Args) != 0 {
				return callResult{}, e.bnd()
			}
			e.use("__we_sig_current")
			v := e.value()
			e.inst(fmt.Sprintf("%%%s = call ptr @__we_sig_current()", v))
			return callResult{kind: ckPrim, i64: "%" + v}, nil
		}
	}
	fn, ok := call.Fn.(*ast.Member)
	if !ok {
		return callResult{}, e.bnd()
	}
	if recv, ok := fn.Recv.(*ast.Ident); ok && e.concAlias[recv.Name] && !e.isLocalName(recv.Name) {
		if spec, is := primCtors[fn.Name]; is {
			ptr, ni := e.emitPrimCtor(spec, call.Args, typ)
			if ni != nil {
				return callResult{}, ni
			}
			return callResult{kind: ckPrim, i64: ptr}, nil
		}
		return callResult{}, e.bnd()
	}
	if recv, ok := fn.Recv.(*ast.Ident); ok {
		if ptr, is := e.prims[recv.Name]; is {
			return e.emitPrimCall(ptr, fn.Name, call.Args)
		}
		if _, is := e.caps.slot(recv.Name); is {
			op, isF, ni := e.loadCapture(recv.Name)
			if ni != nil || isF {
				return callResult{}, e.bnd()
			}
			v := e.value()
			e.inst(fmt.Sprintf("%%%s = inttoptr i64 %s to ptr", v, op))
			return e.emitPrimCall("%"+v, fn.Name, call.Args)
		}
	}
	if recv, ok := fn.Recv.(*ast.Ident); ok && !e.isLocalName(recv.Name) && (fn.Name == "println" || fn.Name == "print") {
		if len(call.Args) == 1 && e.argIsScalar(call.Args[0]) {
			op, isF, ni := e.emitNumExpr(call.Args[0])
			if ni == nil && !isF {
				sym := "__we_println_i64"
				if fn.Name == "print" {
					sym = "__we_print_i64"
				}
				e.use(sym)
				e.inst(fmt.Sprintf("call void @%s(i64 %s)", sym, op))
				return callResult{kind: ckVoid}, nil
			}
			if ni != nil {
				return callResult{}, ni
			}
		}
		ni := e.emitIoCall(call)
		if ni != nil {
			return callResult{}, ni
		}
		return callResult{kind: ckIo}, nil
	}
	return callResult{}, e.bnd()
}

// emitSumCall3 is trySend's value-argument shape: the discriminant is
// the return, the payload word is the fixed zero (the three states carry
// no payload).
func (e *emitter) emitSumCall3(sym, ptr, val string, variants []string) (callResult, *NotImplemented) {
	e.use(sym)
	tag := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @%s(ptr %s, i64 %s)", tag, sym, ptr, val))
	tagSlot := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca i64", tagSlot))
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %%%s", tag, tagSlot))
	paySlot := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca i64", paySlot))
	e.inst(fmt.Sprintf("store i64 0, ptr %%%s", paySlot))
	return callResult{kind: ckSum, sum: sumSlot{tag: "%" + tagSlot, pay: "%" + paySlot, variants: variants}}, nil
}

// emitSumCall is the receive/await/tryReceive shape: the out slot takes
// the payload word, the return value is the discriminant, and the two
// alloca slots become the binding's {tag, payload} pair. variants maps
// the return value to the variant names the match arms carry.
func (e *emitter) emitSumCall(sym, ptr string, variants []string) (callResult, *NotImplemented) {
	e.use(sym)
	out := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca i64", out))
	tag := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @%s(ptr %s, ptr %%%s)", tag, sym, ptr, out))
	tagSlot := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca i64", tagSlot))
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %%%s", tag, tagSlot))
	paySlot := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca i64", paySlot))
	pay := e.value()
	e.inst(fmt.Sprintf("%%%s = load i64, ptr %%%s", pay, out))
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %%%s", pay, paySlot))
	return callResult{kind: ckSum, sum: sumSlot{tag: "%" + tagSlot, pay: "%" + paySlot, variants: variants}}, nil
}

// emitPrimCall is the method dispatch over the six families plus the
// cancel-signal face (the ABI T2 pinned; the receive/await/try trio
// yields the two-slot sums with their variant tables).
func (e *emitter) emitPrimCall(ptr, method string, args []ast.Expr) (callResult, *NotImplemented) {
	one := func() bool { return len(args) == 1 }
	switch method {
	case "update", "read", "wait":
		if !one() {
			return callResult{}, e.bnd()
		}
		cl, ok := args[0].(*ast.Closure)
		if !ok {
			return callResult{}, e.bnd()
		}
		thunk, ni := e.emitCallback(cl)
		if ni != nil {
			return callResult{}, ni
		}
		sym := map[string]string{"update": "__we_prim_update", "read": "__we_prim_read", "wait": "__we_cond_wait"}[method]
		e.use(sym)
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 @%s(ptr %s, ptr %s, ptr null)", v, sym, ptr, thunk))
		return callResult{kind: ckI64, i64: "%" + v}, nil
	case "set", "send":
		if !one() {
			return callResult{}, e.bnd()
		}
		op, ni := e.emitValOrRef(args[0])
		if ni != nil {
			return callResult{}, ni
		}
		if method == "set" {
			e.use("__we_prim_set")
			e.inst(fmt.Sprintf("call void @__we_prim_set(ptr %s, i64 %s)", ptr, op))
			return callResult{kind: ckVoid}, nil
		}
		e.use("__we_chan_send")
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 @__we_chan_send(ptr %s, i64 %s)", v, ptr, op))
		return callResult{kind: ckI64, i64: "%" + v}, nil
	case "trySend":
		if !one() {
			return callResult{}, e.bnd()
		}
		op, ni := e.emitValOrRef(args[0])
		if ni != nil {
			return callResult{}, ni
		}
		res, ni := e.emitSumCall3("__we_chan_try_send", ptr, op, []string{"Sent", "Full", "Closed"})
		return res, ni
	case "get", "acquire", "tryAcquire", "currentCount", "cancel", "isCancelled", "awaitCancelled":
		if len(args) != 0 {
			return callResult{}, e.bnd()
		}
		sym := map[string]string{
			"get": "__we_prim_get", "acquire": "__we_sem_acquire",
			"tryAcquire": "__we_sem_try_acquire", "currentCount": "__we_sem_count",
			"cancel": "__we_handle_cancel", "isCancelled": "__we_sig_check",
			"awaitCancelled": "__we_sig_wait",
		}[method]
		e.use(sym)
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 @%s(ptr %s)", v, sym, ptr))
		return callResult{kind: ckI64, i64: "%" + v}, nil
	case "receive", "tryReceive", "await":
		if len(args) != 0 {
			return callResult{}, e.bnd()
		}
		if method == "receive" {
			return e.emitSumCall("__we_chan_recv", ptr, []string{"None", "Some"})
		}
		if method == "tryReceive" {
			return e.emitSumCall("__we_chan_try_recv", ptr, []string{"Empty", "Received", "Closed"})
		}
		res, ni := e.emitSumCall("__we_handle_await", ptr, []string{"Ok", "Err"})
		if ni == nil {
			res.sum.errPanic = true // the Err payload is the panic message
		}
		return res, ni
	case "release", "close", "signal", "broadcast":
		if len(args) != 0 {
			return callResult{}, e.bnd()
		}
		sym := map[string]string{
			"release": "__we_sem_release", "close": "__we_chan_close",
			"signal": "__we_cond_signal", "broadcast": "__we_cond_broadcast",
		}[method]
		e.use(sym)
		if method == "close" {
			v := e.value()
			e.inst(fmt.Sprintf("%%%s = call i64 @%s(ptr %s)", v, sym, ptr))
			return callResult{kind: ckI64, i64: "%" + v}, nil
		}
		e.inst(fmt.Sprintf("call void @%s(ptr %s)", sym, ptr))
		return callResult{kind: ckVoid}, nil
	}
	return callResult{}, e.bnd()
}

// emitValOrRef renders a send argument: a scalar in the i64 domain, or a
// gc-record construction narrowed to its pointer word (the channel's
// element bitmap traces it inside the buffer).
func (e *emitter) emitValOrRef(x ast.Expr) (string, *NotImplemented) {
	if op, isF, ni := e.emitNumExpr(x); ni == nil {
		if isF {
			return "", e.bnd()
		}
		return op, nil
	}
	if c, ok := x.(*ast.Construct); ok {
		reg, ni := e.emitConstruct(c)
		if ni != nil {
			return "", ni
		}
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = ptrtoint ptr %s to i64", v, reg))
		return "%" + v, nil
	}
	return "", e.bnd()
}

// emitCallback emits one callback predicate as an internal thunk define:
// `i64 (ptr env, i64 v)`, zero-capture (env is null — a closure that
// reaches an outer binding stops at the callback word), the body a
// single straight-line numeric expression (design D8's callback face).
func (e *emitter) emitCallback(cl *ast.Closure) (string, *NotImplemented) {
	if !cl.Short || len(cl.Params) != 1 || cl.Ret != nil || len(cl.Body.Items) != 1 {
		return "", bndCallback()
	}
	es, ok := cl.Body.Items[0].(*ast.ExprStmt)
	if !ok {
		return "", bndCallback()
	}
	savedCtx, savedScalars, savedBody := e.ctx, e.scalars, e.body
	e.ctx = ctxCallback
	e.scalars = map[string]scalarSlot{cl.Params[0].Name: {operand: "%v0"}}
	e.body = strings.Builder{}
	op, _, ni := e.emitNumExpr(es.Expr)
	// The callback's text snapshots before the restore — reading e.body
	// after would return the caller's builder (the swap-back already
	// happened), and the callback's own instructions would vanish.
	cb := e.body.String()
	e.body, e.ctx, e.scalars = savedBody, savedCtx, savedScalars
	if ni != nil {
		return "", ni
	}
	name := fmt.Sprintf("@.cb%d", len(e.thunks))
	e.thunks = append(e.thunks, fmt.Sprintf(
		"define internal i64 %s(ptr %%env, i64 %%v0) {\nentry:\n%s  ret i64 %s\n}\n",
		name, cb, op))
	return name, nil
}

// --- the M9b control-flow and concurrent forms --------------------------------

// condI64 emits a condition and reduces it to i1: the numeric value is
// nonzero-true in the i64 domain (Bool rides the domain). A method call
// (a predicate like isCancelled) routes through emitCall — its i64 result
// is the condition.
func (e *emitter) condI64(x ast.Expr) (string, *NotImplemented) {
	op := ""
	if call, ok := x.(*ast.Call); ok {
		res, ni := e.emitCall(call, nil)
		if ni != nil {
			return "", ni
		}
		if res.kind != ckI64 {
			return "", e.bnd()
		}
		op = res.i64
	} else {
		var ni *NotImplemented
		op, _, ni = e.emitNumExpr(x)
		if ni != nil {
			return "", ni
		}
	}
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp ne i64 %s, 0", v, op))
	return "%" + v, nil
}

// emitBlockStmts emits a nested block's statements (a block introduces
// no boundary of its own — the M9b set is flat).
func (e *emitter) emitBlockStmts(items []ast.Stmt) *NotImplemented {
	for _, st := range items {
		if _, ok := st.(*ast.Return); ok {
			return e.bnd() // returns sit at the body tail only
		}
		if ni := e.emitStmt(st); ni != nil {
			return ni
		}
	}
	return nil
}

// emitIf emits the conditional: then plus optional else (a block or a
// nested else-if), both arms joining one continuation label.
func (e *emitter) emitIf(s *ast.If) *NotImplemented {
	c, ni := e.condI64(s.Cond)
	if ni != nil {
		return ni
	}
	n := e.blocks
	e.blocks++
	join := fmt.Sprintf("ifjoin%d", n)
	thenL := fmt.Sprintf("ifthen%d", n)
	var elseL string
	switch s.Else.(type) {
	case nil:
		elseL = join
	case *ast.BlockExpr, *ast.If:
		elseL = fmt.Sprintf("ifelse%d", n)
	default:
		return e.bnd()
	}
	e.inst(fmt.Sprintf("br i1 %s, label %%%s, label %%%s", c, thenL, elseL))
	e.label(thenL)
	if ni := e.emitBlockStmts(s.Then.Items); ni != nil {
		return ni
	}
	e.inst(fmt.Sprintf("br label %%%s", join))
	if s.Else != nil {
		e.label(elseL)
		if els, ok := s.Else.(*ast.BlockExpr); ok {
			if ni := e.emitBlockStmts(els.Block.Items); ni != nil {
				return ni
			}
		} else if inner, ok := s.Else.(*ast.If); ok {
			if ni := e.emitIf(inner); ni != nil {
				return ni
			}
			// The nested if already closed on its own join; branch to
			// this one.
		}
		e.inst(fmt.Sprintf("br label %%%s", join))
	}
	e.label(join)
	return nil
}

// emitWhile emits the loop: head re-evaluates the condition, the body
// branches back, the exit label continues. break/continue are outside
// the M9b statement set (the boundary word names them absent).
func (e *emitter) emitWhile(s *ast.While) *NotImplemented {
	n := e.blocks
	e.blocks++
	head := fmt.Sprintf("whhead%d", n)
	body := fmt.Sprintf("whbody%d", n)
	exit := fmt.Sprintf("whexit%d", n)
	e.inst(fmt.Sprintf("br label %%%s", head))
	e.label(head)
	c, ni := e.condI64(s.Cond)
	if ni != nil {
		return ni
	}
	e.inst(fmt.Sprintf("br i1 %s, label %%%s, label %%%s", c, body, exit))
	e.label(body)
	if ni := e.emitBlockStmts(s.Body.Items); ni != nil {
		return ni
	}
	e.inst(fmt.Sprintf("br label %%%s", head))
	e.label(exit)
	return nil
}

// emitMatch emits the two-slot sum match: load the discriminant, test
// each variant arm in order (the variant table maps names to the
// runtime's return codes), the wildcard arm as the default, and every
// arm joins one continuation. Payload bindings (Some(x)) load the
// payload word into the scalar domain.
func (e *emitter) emitMatch(s *ast.Match) *NotImplemented {
	id, ok := s.Scrutinee.(*ast.Ident)
	if !ok {
		return e.bnd()
	}
	slot, ok := e.sums2[id.Name]
	if !ok {
		return e.bnd()
	}
	n := e.blocks
	e.blocks++
	join := fmt.Sprintf("mjoin%d", n)
	tag := e.loadNum(slot.tag, false)
	// The first comparison tests in the current block — no dispatch label
	// precedes it (a branch to an undefined label is exactly the bug the
	// v2 build removed); each arm's fall-through closes into its own
	// labeled block, where the next comparison (or the wildcard arm) runs.
	fired := false
	for i, arm := range s.Arms {
		pv, ok := arm.Pat.(*ast.PatVariant)
		if !ok {
			if _, wild := arm.Pat.(*ast.PatWildcard); !wild || fired {
				return e.bnd() // only one wildcard, in final position
			}
			// The default arm: everything left falls in — the previous
			// arm's fall-through block is already open (labeled at its
			// close), so the body emits here without reopening it.
			if ni := e.emitMatchArm(arm, slot); ni != nil {
				return ni
			}
			e.inst(fmt.Sprintf("br label %%%s", join))
			fired = true
			continue
		}
		idx := -1
		for j, v := range slot.variants {
			if v == pv.Name {
				idx = j
				break
			}
		}
		if idx < 0 {
			return e.bnd()
		}
		c := e.value()
		e.inst(fmt.Sprintf("%%%s = icmp eq i64 %s, %d", c, tag, idx))
		armL := fmt.Sprintf("marm%d_%d", n, i)
		next := fmt.Sprintf("mtest%d_%d", n, i)
		e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", c, armL, next))
		e.label(armL)
		if ni := e.emitMatchArm(arm, slot); ni != nil {
			return ni
		}
		e.inst(fmt.Sprintf("br label %%%s", join))
		e.label(next)
	}
	if !fired {
		e.inst(fmt.Sprintf("br label %%%s", join)) // typecheck exhausted; defensive
	}
	e.label(join)
	return nil
}

// emitMatchArm emits one arm's body; a PatVariant payload binding loads
// the payload word first.
func (e *emitter) emitMatchArm(arm ast.MatchArm, slot sumSlot) *NotImplemented {
	if pv, ok := arm.Pat.(*ast.PatVariant); ok && len(pv.Args) == 1 {
		if b, ok := pv.Args[0].(*ast.PatBinding); ok && b.Name != "_" {
			op := e.loadNum(slot.pay, false)
			e.scalars[b.Name] = scalarSlot{operand: op}
		}
	}
	blk, ok := arm.Body.(*ast.BlockExpr)
	if !ok {
		return e.bnd() // arm bodies are blocks in the M9b set
	}
	return e.emitBlockStmts(blk.Block.Items)
}

// emitPanic emits `panic("msg")`: the NUL-terminated message constant
// and the task-fail tail. Main is task 0, so one ABI serves both
// contexts (the scheduler renders the report and settles the exit code).
func (e *emitter) emitPanic(x ast.Expr) (callResult, *NotImplemented) {
	lit, ok := x.(*ast.Literal)
	if !ok || lit.Kind != "string" {
		return callResult{}, e.bnd()
	}
	data, ok := decodeStringLiteral(lit.Text)
	if !ok {
		return callResult{}, e.bnd()
	}
	e.use("__we_task_fail")
	cn := fmt.Sprintf("@.p%d", len(e.panics))
	e.panics = append(e.panics, fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\\00\"", cn, len(data)+1, irEscape(data)))
	e.inst(fmt.Sprintf("call void @__we_task_fail(ptr %s)", cn))
	e.inst("unreachable")
	n := e.blocks
	e.blocks++
	e.label(fmt.Sprintf("pd%d", n)) // the never-reached continuation
	return callResult{kind: ckVoid}, nil
}

// emitQuestion emits `expr?`: the sum's Err branch runs the error tail —
// a panic-message payload (an await's TaskPanic) hands the C string to
// the task-fail ABI, a static report line (a timeout scope) writes the
// M8 fail constant — and the Ok branch continues with the payload as
// the expression's value.
func (e *emitter) emitQuestion(p *ast.Prop) (callResult, *NotImplemented) {
	res, ni := e.emitSumSource(p.X)
	if ni != nil {
		return callResult{}, ni
	}
	slot := res.sum
	tag := e.loadNum(slot.tag, false)
	c := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp eq i64 %s, 0", c, tag))
	n := e.blocks
	e.blocks++
	okL := fmt.Sprintf("qok%d", n)
	errL := fmt.Sprintf("qerr%d", n)
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", c, okL, errL))
	e.label(errL)
	if slot.errPanic {
		msg := e.loadNum(slot.pay, false)
		pv := e.value()
		e.inst(fmt.Sprintf("%%%s = inttoptr i64 %s to ptr", pv, msg))
		e.use("__we_task_fail")
		e.inst(fmt.Sprintf("call void @__we_task_fail(ptr %%%s)", pv))
	} else {
		line := slot.errMsg
		if line == "" {
			return callResult{}, e.bnd()
		}
		e.use("__we_fail")
		cn := fmt.Sprintf("@.q%d", len(e.panics))
		e.panics = append(e.panics, fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\\0A\"", cn, len(line)+1, irEscape(line)))
		e.inst(fmt.Sprintf("call void @__we_fail(ptr %s, i64 %d)", cn, len(line)+1))
	}
	e.inst("unreachable")
	e.label(okL)
	pay := e.loadNum(slot.pay, false)
	return callResult{kind: ckI64, i64: pay}, nil
}

// emitSumSource evaluates the expression under a `?` or a select source:
// the sum-yielding calls, or a name already bound to one.
func (e *emitter) emitSumSource(x ast.Expr) (callResult, *NotImplemented) {
	switch v := x.(type) {
	case *ast.Ident:
		if s, ok := e.sums2[v.Name]; ok {
			return callResult{kind: ckSum, sum: s}, nil
		}
		return callResult{}, e.bnd()
	case *ast.Call:
		return e.emitCall(v, nil)
	default:
		return callResult{}, e.bnd()
	}
}

// collectCaptures walks a task body for identifier reads that name
// bindings of the enclosing scope: primitive handles (traced pointer
// slots) and scalars (plain i64 slots). Strings, records, and sums stop
// at the task boundary (the multi-word faces are outside the M9b set).
func (e *emitter) collectCaptures(items []ast.Stmt) *captureSet {
	caps := &captureSet{}
	seen := map[string]bool{}
	var walkExpr func(ast.Expr)
	var walkStmt func(ast.Stmt)
	read := func(name string) {
		if seen[name] || name == "_" {
			return
		}
		if _, ok := e.prims[name]; ok {
			seen[name] = true
			caps.names = append(caps.names, name)
			caps.prim = append(caps.prim, true)
		} else if _, ok := e.scalars[name]; ok {
			seen[name] = true
			caps.names = append(caps.names, name)
			caps.prim = append(caps.prim, false)
		}
	}
	walkExpr = func(x ast.Expr) {
		switch v := x.(type) {
		case *ast.Ident:
			read(v.Name)
		case *ast.Member:
			walkExpr(v.Recv)
		case *ast.Call:
			walkExpr(v.Fn)
			for _, a := range v.Args {
				walkExpr(a)
			}
		case *ast.Binary:
			walkExpr(v.L)
			walkExpr(v.R)
		case *ast.Prop:
			walkExpr(v.X)
		case *ast.If:
			walkExpr(v.Cond)
			for _, st := range v.Then.Items {
				walkStmt(st)
			}
			if v.Else != nil {
				walkExpr(v.Else)
			}
		case *ast.Match:
			walkExpr(v.Scrutinee)
			for _, a := range v.Arms {
				walkExpr(a.Body)
			}
		case *ast.BlockExpr:
			for _, st := range v.Block.Items {
				walkStmt(st)
			}
		case *ast.TaskExpr:
			for _, st := range v.Body.Items {
				walkStmt(st)
			}
		case *ast.ScopeExpr:
			if v.Timeout != nil {
				walkExpr(v.Timeout)
			}
			for _, st := range v.Body.Items {
				walkStmt(st)
			}
		case *ast.SelectExpr:
			for _, c := range v.Cases {
				walkExpr(c.Source)
				walkExpr(c.Body)
			}
		case *ast.Closure:
			// Callback bodies are zero-capture by their own boundary.
		}
	}
	walkStmt = func(st ast.Stmt) {
		switch v := st.(type) {
		case *ast.Binding:
			walkExpr(v.Init)
		case *ast.Assign:
			walkExpr(v.Value)
		case *ast.ExprStmt:
			walkExpr(v.Expr)
		case *ast.While:
			walkExpr(v.Cond)
			for _, s := range v.Body.Items {
				walkStmt(s)
			}
		case *ast.Defer:
			for _, s := range v.Block.Items {
				walkStmt(s)
			}
		}
	}
	for _, st := range items {
		walkStmt(st)
	}
	return caps
}

// loadCapture reads one env-block slot (the pointer classes are uniform
// here: a prim slot loads as i64 the call face wants).
func (e *emitter) loadCapture(name string) (string, bool, *NotImplemented) {
	i, ok := e.caps.slot(name)
	if !ok {
		return "", false, e.bnd()
	}
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = getelementptr i8, ptr %s, i64 %d", v, e.caps.ptr, 16+8*i))
	r := e.value()
	e.inst(fmt.Sprintf("%%%s = load i64, ptr %%%s", r, v))
	return "%" + r, false, nil
}

// emitTask emits one task block: the thunk define (the body under the
// task context, its tail expression the thunk's value), the environment
// block carrying the captured names (a gc allocation rooted by the
// creating scope — the M8 push/pop protocol covers its lifetime), and
// the constructor call yielding the handle.
func (e *emitter) emitTask(t *ast.TaskExpr, _ ast.TypeRef) (callResult, *NotImplemented) {
	caps := e.collectCaptures(t.Body.Items)

	// The env block: frozen header plus one word per capture, the
	// bitmap marking the traced pointer slots. It roots before the task
	// can run, and the creating body's exit pops it.
	n := len(caps.names)
	envMap := fmt.Sprintf("@.emap%d", len(e.envMaps))
	words := (n + 63) / 64
	if words == 0 {
		e.envMaps = append(e.envMaps, envMap+" = private unnamed_addr constant [0 x i64] zeroinitializer")
	} else {
		bitmap := make([]uint64, words)
		for i, isPrim := range caps.prim {
			if isPrim {
				bitmap[i/64] |= 1 << (i % 64)
			}
		}
		parts := make([]string, words)
		for i := range parts {
			parts[i] = fmt.Sprintf("i64 %d", bitmap[i])
		}
		e.envMaps = append(e.envMaps, fmt.Sprintf("%s = private unnamed_addr constant [%d x i64] [%s]", envMap, words, strings.Join(parts, ", ")))
	}
	e.use("__we_alloc")
	e.use("__we_root_push")
	env := "%" + e.value()
	e.inst(fmt.Sprintf("%s = call ptr @__we_alloc(i64 %d)", env, 16+8*n))
	e.inst(fmt.Sprintf("store ptr %s, ptr %s", envMap, env))
	e.pushes++
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", env))
	for i, name := range caps.names {
		var op string
		if p, ok := e.prims[name]; ok {
			// A primitive handle rides the slot as its address word — the
			// env block is uniform i64 storage, the thunk's loadCapture
			// inttoptrs it back.
			v := e.value()
			e.inst(fmt.Sprintf("%%%s = ptrtoint ptr %s to i64", v, p))
			op = "%" + v
		} else {
			op, _, _ = e.emitNumExpr(&ast.Ident{Name: name})
		}
		e.gepStore(env, 16+8*i, "i64 "+op)
	}

	// The thunk: the body under the task context, local environments
	// cleared, captures resolved through the env parameter. The root
	// pushes inside ride the task's own gc window (retire settles them
	// at task end), so the thunk tail returns without popping.
	name := fmt.Sprintf("@.task%d", len(e.thunks))
	savedCtx, savedBody := e.ctx, e.body
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	restore := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
	}
	e.ctx = ctxTask
	e.body = strings.Builder{}
	e.scalars = make(map[string]scalarSlot)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.defers = nil
	caps.ptr = "%env"
	e.caps = caps
	val := "0"
	items := t.Body.Items
	if len(items) > 0 {
		if es, ok := items[len(items)-1].(*ast.ExprStmt); ok {
			if _, isCall := es.Expr.(*ast.Call); !isCall {
				// A value-shaped tail expression: the thunk's value.
				if op, _, ni := e.emitNumExpr(es.Expr); ni == nil {
					val = op
					items = items[:len(items)-1]
				}
			}
		}
	}
	for _, st := range items {
		if _, ok := st.(*ast.Return); ok {
			restore()
			return callResult{}, bndTask()
		}
		if ni := e.emitStmt(st); ni != nil {
			restore()
			return callResult{}, ni
		}
	}
	// Deferred blocks invert at the tail, reverse registration order.
	for i := len(e.defers) - 1; i >= 0; i-- {
		if ni := e.emitBlockStmts(e.defers[i].Items); ni != nil {
			restore()
			return callResult{}, ni
		}
	}
	body := e.body.String()
	restore()
	e.thunks = append(e.thunks, fmt.Sprintf(
		"define internal i64 %s(ptr %%env) {\nentry:\n%s  ret i64 %s\n}\n",
		name, body, val))

	e.use("__we_task_new")
	h := e.value()
	e.inst(fmt.Sprintf("%%%s = call ptr @__we_task_new(ptr %s, ptr %s)", h, name, env))
	return callResult{kind: ckPrim, i64: "%" + h}, nil
}

// emitScope emits the compound scope: enter with the timeout (evaluated
// at the clause, -1 without) and the collectAll flag, the body as
// ordinary statements, leave joining every task created inside. The
// value form wraps the outcome: tag is leave's TimedOut bit, payload the
// body's tail value.
func (e *emitter) emitScope(s *ast.ScopeExpr, valueForm bool) (callResult, *NotImplemented) {
	dl := "-1"
	if s.Timeout != nil {
		op, isF, ni := e.emitNumExpr(s.Timeout)
		if ni != nil || isF {
			return callResult{}, ni
		}
		dl = op
	}
	ca := int64(0)
	if s.CollectAll {
		ca = 1
	}
	e.use("__we_scope_enter")
	sc := e.value()
	e.inst(fmt.Sprintf("%%%s = call ptr @__we_scope_enter(i64 %s, i64 %d)", sc, dl, ca))
	bodyVal := "0"
	items := s.Body.Items
	if valueForm && len(items) > 0 {
		if es, ok := items[len(items)-1].(*ast.ExprStmt); ok {
			if _, isCall := es.Expr.(*ast.Call); !isCall {
				if op, _, ni := e.emitNumExpr(es.Expr); ni == nil {
					bodyVal = op
					items = items[:len(items)-1]
				}
			}
		}
	}
	for _, st := range items {
		if _, ok := st.(*ast.Return); ok {
			return callResult{}, e.bnd()
		}
		if ni := e.emitStmt(st); ni != nil {
			return callResult{}, ni
		}
	}
	e.use("__we_scope_leave")
	to := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_scope_leave(ptr %%%s)", to, sc))
	if !valueForm {
		return callResult{kind: ckVoid}, nil
	}
	tagSlot := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca i64", tagSlot))
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %%%s", to, tagSlot))
	paySlot := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca i64", paySlot))
	e.inst(fmt.Sprintf("store i64 %s, ptr %%%s", bodyVal, paySlot))
	return callResult{kind: ckSum, sum: sumSlot{
		tag: "%" + tagSlot, pay: "%" + paySlot,
		variants: []string{"Ok", "Err"}, errMsg: "error: TimedOut",
	}}, nil
}

// emitSelect emits the racing select: one select object, one
// registration per case in source order (receive and await sources —
// the M9b golden face), the park, then the arm switch dispatching on
// the fired index. The taken case's body value is the expression's
// value (a result alloca joins the arms); the case name binds the
// source's yield inside its arm.
func (e *emitter) emitSelect(s *ast.SelectExpr) (callResult, *NotImplemented) {
	if len(s.Cases) < 2 {
		return callResult{}, e.bnd()
	}
	e.use("__we_select_new")
	sel := e.value()
	e.inst(fmt.Sprintf("%%%s = call ptr @__we_select_new()", sel))
	for _, c := range s.Cases {
		call, ok := c.Source.(*ast.Call)
		if !ok {
			return callResult{}, e.bnd()
		}
		m, ok := call.Fn.(*ast.Member)
		if !ok || len(call.Args) != 0 {
			return callResult{}, e.bnd()
		}
		recv, ok := m.Recv.(*ast.Ident)
		if !ok {
			return callResult{}, e.bnd()
		}
		src := e.sourcePtr(recv.Name)
		if src == "" {
			return callResult{}, e.bnd()
		}
		switch m.Name {
		case "receive":
			e.use("__we_select_add_recv")
			e.inst(fmt.Sprintf("call void @__we_select_add_recv(ptr %%%s, ptr %s)", sel, src))
		case "await":
			e.use("__we_select_add_await")
			e.inst(fmt.Sprintf("call void @__we_select_add_await(ptr %%%s, ptr %s)", sel, src))
		default:
			return callResult{}, e.bnd() // send sources are outside the M9b set
		}
	}
	e.use("__we_select_park")
	arm := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_select_park(ptr %%%s)", arm, sel))
	e.use("__we_select_value")

	n := e.blocks
	e.blocks++
	join := fmt.Sprintf("sljoin%d", n)
	res := e.value()
	e.inst(fmt.Sprintf("%%%s = alloca i64", res))
	for i, c := range s.Cases {
		cmp := e.value()
		e.inst(fmt.Sprintf("%%%s = icmp eq i64 %%%s, %d", cmp, arm, i))
		armL := fmt.Sprintf("slarm%d_%d", n, i)
		next := fmt.Sprintf("sltest%d_%d", n, i)
		e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", cmp, armL, next))
		e.label(armL)
		if c.Name != "" && !c.Wildcard {
			e.use("__we_select_value")
			vv := e.value()
			e.inst(fmt.Sprintf("%%%s = call i64 @__we_select_value(ptr %%%s)", vv, sel))
			if c.Name != "_" {
				e.scalars[c.Name] = scalarSlot{operand: "%" + vv}
			}
		}
		op, _, ni := e.emitNumExpr(c.Body)
		if ni != nil {
			return callResult{}, ni
		}
		e.inst(fmt.Sprintf("store i64 %s, ptr %%%s", op, res))
		e.inst(fmt.Sprintf("br label %%%s", join))
		e.label(next)
	}
	e.inst(fmt.Sprintf("br label %%%s", join))
	e.label(join)
	lv := e.value()
	e.inst(fmt.Sprintf("%%%s = load i64, ptr %%%s", lv, res))
	return callResult{kind: ckI64, i64: "%" + lv}, nil
}

// sourcePtr resolves a select source's receiver to its pointer operand
// (a local binding or a task capture).
func (e *emitter) sourcePtr(name string) string {
	if p, ok := e.prims[name]; ok {
		return p
	}
	if i, ok := e.caps.slot(name); ok {
		p := e.value()
		e.inst(fmt.Sprintf("%%%s = getelementptr i8, ptr %s, i64 %d", p, e.caps.ptr, 16+8*i))
		w := e.value()
		e.inst(fmt.Sprintf("%%%s = load i64, ptr %%%s", w, p))
		r := e.value()
		e.inst(fmt.Sprintf("%%%s = inttoptr i64 %%%s to ptr", r, w))
		return "%" + r
	}
	return ""
}

// emitStringExpr emits one String operand — the (ptr, len) pair — from a
// plain literal, a let-bound name, or a field chain ending at a String
// field. The pair may be a constant global plus immediate, or two loaded
// registers.
func (e *emitter) emitStringExpr(x ast.Expr) (string, string, *NotImplemented) {
	switch v := x.(type) {
	case *ast.Literal:
		if v.Kind != "string" {
			return "", "", bndMain()
		}
		data, ok := decodeStringLiteral(v.Text)
		if !ok {
			return "", "", bndMain()
		}
		return e.intern(data), strconv.Itoa(len(data)), nil
	case *ast.Ident:
		b, ok := e.strEnv[v.Name]
		if !ok {
			return "", "", bndMain()
		}
		return e.intern(b.data), strconv.Itoa(b.length), nil
	case *ast.Member:
		return e.emitFieldChainString(v)
	default:
		return "", "", bndMain()
	}
}

// emitFieldChainString resolves `root.f.g…s` where root names a let-bound
// gc record, the intermediate hops load record-reference fields, and the
// final hop loads a String field's double word.
func (e *emitter) emitFieldChainString(m *ast.Member) (string, string, *NotImplemented) {
	var hops []string
	x := m
	for {
		hops = append([]string{x.Name}, hops...)
		switch r := x.Recv.(type) {
		case *ast.Ident:
			g, ok := e.gcEnv[r.Name]
			if !ok {
				return "", "", bndMain()
			}
			return e.walkChain(g.reg, g.rec, hops)
		case *ast.Member:
			x = r
		default:
			return "", "", bndMain()
		}
	}
}

// walkChain emits the loads for hops over base (a record pointer of type
// recName); the last hop must land on a String field.
func (e *emitter) walkChain(base, recName string, hops []string) (string, string, *NotImplemented) {
	for _, h := range hops[:len(hops)-1] {
		slot, ok := e.fieldSlotOf(recName, h)
		if !ok || slot.kind != fkRef {
			return "", "", bndMain()
		}
		base = e.gepLoadPtr(base, slot.off)
		recName = slot.typ
	}
	slot, ok := e.fieldSlotOf(recName, hops[len(hops)-1])
	if !ok || slot.kind != fkStr {
		return "", "", bndMain()
	}
	return e.gepLoadPtr(base, slot.off), e.gepLoadI64(base, slot.off+8), nil
}

// layout computes a record's field slots: offsets from 16 (the frozen
// header {map@0, size@8} of design D6), sizes, and the reference bitmap
// derived kinds. A field outside the M8 shape fails the whole emission.
func (e *emitter) layout(rec *ast.RecordDecl) ([]fieldSlot, int, bool) {
	slots := make([]fieldSlot, len(rec.Fields))
	off := 16
	for i, fd := range rec.Fields {
		n, ok := fd.Typ.(*ast.NamedType)
		if !ok || n.Qual != "" || len(n.Args) != 0 {
			return nil, 0, false
		}
		slots[i].off = off
		switch n.Name {
		case "String":
			slots[i].kind = fkStr
			off += 16
		case "Int64", "UInt64", "Bool":
			slots[i].kind = fkScalar
			off += 8
		default:
			r, ok := e.records[n.Name]
			if !ok || r.Cat != "gc" || len(r.TypeParams) != 0 {
				return nil, 0, false
			}
			slots[i].kind = fkRef
			slots[i].typ = n.Name
			slots[i].isRef = true
			off += 8
		}
	}
	return slots, off, true
}

// fieldSlotOf finds one field's slot by name.
func (e *emitter) fieldSlotOf(recName, field string) (fieldSlot, bool) {
	rec, ok := e.records[recName]
	if !ok {
		return fieldSlot{}, false
	}
	slots, _, ok := e.layout(rec)
	if !ok {
		return fieldSlot{}, false
	}
	for i, fd := range rec.Fields {
		if fd.Name == field {
			return slots[i], true
		}
	}
	return fieldSlot{}, false
}

// emitConstruct emits the gc construction protocol of design D4: alloc,
// map store, root push — the object is rooted before any field value
// evaluates, so a nested allocation never races a collection with its
// parent unrooted — then the field stores in source order.
func (e *emitter) emitConstruct(c *ast.Construct) (string, *NotImplemented) {
	rec, ok := e.records[c.Name]
	if !ok || rec.Cat != "gc" || len(rec.TypeParams) != 0 {
		return "", bndMain()
	}
	slots, total, ok := e.layout(rec)
	if !ok {
		return "", bndMain()
	}
	e.usedRecs[rec.Name] = true
	e.use("__we_alloc")
	e.use("__we_root_push")
	e.pushes++
	reg := "%" + e.value()
	e.inst(fmt.Sprintf("%s = call ptr @__we_alloc(i64 %d)", reg, total))
	e.inst(fmt.Sprintf("store ptr @.map.%s, ptr %s", rec.Name, reg))
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", reg))
	byName := make(map[string]int, len(rec.Fields))
	for i, fd := range rec.Fields {
		byName[fd.Name] = i
	}
	for _, fi := range c.Fields {
		idx, ok := byName[fi.Name]
		if !ok {
			return "", bndMain()
		}
		slot := slots[idx]
		switch slot.kind {
		case fkStr:
			lit, ok := fi.Value.(*ast.Literal)
			if !ok || lit.Kind != "string" {
				return "", bndMain()
			}
			data, ok := decodeStringLiteral(lit.Text)
			if !ok {
				return "", bndMain()
			}
			e.gepStore(reg, slot.off, "ptr "+e.intern(data))
			e.gepStore(reg, slot.off+8, "i64 "+strconv.Itoa(len(data)))
		case fkRef:
			nc, ok := fi.Value.(*ast.Construct)
			if !ok {
				return "", bndMain()
			}
			child, ni := e.emitConstruct(nc)
			if ni != nil {
				return "", ni
			}
			e.gepStore(reg, slot.off, "ptr "+child)
		case fkScalar:
			imm, ok := scalarImmediate(fi.Value)
			if !ok {
				return "", bndMain()
			}
			e.gepStore(reg, slot.off, "i64 "+imm)
		}
	}
	return reg, nil
}

// scalarImmediate renders an int or bool literal as the i64 stored into a
// scalar field (the construction-argument positions of design D4).
func scalarImmediate(x ast.Expr) (string, bool) {
	lit, ok := x.(*ast.Literal)
	if !ok {
		return "", false
	}
	switch lit.Kind {
	case "int":
		text := lit.Text
		for _, suf := range []string{"i64", "i32", "i16", "i8", "u64", "u32", "u16", "u8"} {
			if strings.HasSuffix(text, suf) {
				text = text[:len(text)-len(suf)]
				break
			}
		}
		n, err := strconv.ParseInt(text, 0, 64)
		if err != nil {
			return "", false
		}
		return strconv.FormatInt(n, 10), true
	case "bool":
		if lit.Text == "true" {
			return "1", true
		}
		if lit.Text == "false" {
			return "0", true
		}
		return "", false
	default:
		return "", false
	}
}

// emitTail emits the single return: Ok pops every pushed root (reverse
// push order — M8 bodies are straight-line, so the pops ride one ret) and
// returns 0; Err ends the process inside __we_fail, a noreturn call that
// no pop precedes.
func (e *emitter) emitTail(v ast.Expr) *NotImplemented {
	call, ok := v.(*ast.Call)
	if !ok {
		return bndMain()
	}
	fn, ok := call.Fn.(*ast.Ident)
	if !ok {
		return bndMain()
	}
	switch fn.Name {
	case "Ok":
		if len(call.Args) != 1 {
			return bndMain()
		}
		if _, ok := call.Args[0].(*ast.Unit); !ok {
			return bndMain()
		}
		// The deferred blocks run before the roots pop (their statements
		// may still read rooted objects), reverse registration order.
		for i := len(e.defers) - 1; i >= 0; i-- {
			if ni := e.emitBlockStmts(e.defers[i].Items); ni != nil {
				return ni
			}
		}
		if e.pushes > 0 {
			e.use("__we_root_pop")
			for i := 0; i < e.pushes; i++ {
				e.inst("call void @__we_root_pop()")
			}
		}
		e.inst("ret i32 0")
		return nil
	case "Err":
		if len(call.Args) != 1 {
			return bndErrPay()
		}
		ctor, ok := call.Args[0].(*ast.Call)
		if !ok || len(ctor.Args) != 1 {
			return bndErrPay()
		}
		vfn, ok := ctor.Fn.(*ast.Ident)
		if !ok {
			return bndErrPay()
		}
		lit, ok := ctor.Args[0].(*ast.Literal)
		if !ok || lit.Kind != "string" {
			return bndErrPay()
		}
		payload, ok := decodeStringLiteral(lit.Text)
		if !ok {
			return bndErrPay()
		}
		if !isStringPayloadVariant(e.sums, e.mainRet, vfn.Name) {
			return bndErrPay()
		}
		// The report line is the runtime-written byte sequence: the
		// payload decodes to the bytes the source means, and __we_fail
		// writes exactly this many of them (design D5).
		line := "error: " + vfn.Name + ": " + payload + "\n"
		e.errConst = fmt.Sprintf("@.err = private unnamed_addr constant [%d x i8] c\"%s\"", len(line), irEscape(line))
		e.use("__we_fail")
		e.inst(fmt.Sprintf("call void @__we_fail(ptr @.err, i64 %d)", len(line)))
		e.inst("unreachable")
		return nil
	default:
		return bndMain()
	}
}

// --- the IR operand helpers ---------------------------------------------------

// gepStore stores one operand at base+off, materializing the address
// through its own getelementptr value.
func (e *emitter) gepStore(base string, off int, operand string) {
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = getelementptr i8, ptr %s, i64 %d", v, base, off))
	e.inst(fmt.Sprintf("store %s, ptr %%%s", operand, v))
}

// gepLoadPtr loads a pointer word at base+off.
func (e *emitter) gepLoadPtr(base string, off int) string {
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = getelementptr i8, ptr %s, i64 %d", v, base, off))
	r := e.value()
	e.inst(fmt.Sprintf("%%%s = load ptr, ptr %%%s", r, v))
	return "%" + r
}

// gepLoadI64 loads an i64 word at base+off.
func (e *emitter) gepLoadI64(base string, off int) string {
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = getelementptr i8, ptr %s, i64 %d", v, base, off))
	r := e.value()
	e.inst(fmt.Sprintf("%%%s = load i64, ptr %%%s", r, v))
	return "%" + r
}

// intern enters data in the constant pool, first appearance naming it.
func (e *emitter) intern(data string) string {
	if g, ok := e.strPool[data]; ok {
		return g
	}
	g := fmt.Sprintf("@.s%d", len(e.strs))
	e.strs = append(e.strs, strConst{name: g, data: data})
	e.strPool[data] = g
	return g
}

// mapLiteral renders a descriptor's bitmap: one i64 per 64 payload slots,
// bit i set when the word at offset 16+8*i is a gc reference. A record
// with no payload slots carries the empty zeroinitializer form.
func mapLiteral(bitmap []uint64, nslots int) string {
	words := (nslots + 63) / 64
	if words == 0 {
		return "[0 x i64] zeroinitializer"
	}
	parts := make([]string, words)
	for i := range parts {
		if i < len(bitmap) {
			parts[i] = fmt.Sprintf("i64 %d", bitmap[i])
		} else {
			parts[i] = "i64 0"
		}
	}
	return "[" + strconv.Itoa(words) + " x i64] [" + strings.Join(parts, ", ") + "]"
}

// render assembles the module: header, the global groups (struct type
// lines and map descriptors in declaration order over the used records,
// the constant pool, the report line), the declares in fixed order, and
// the one function.
func (e *emitter) render(module string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "; ModuleID = '%s'\n\n", module)

	var groups [][]string
	var structs, maps []string
	for _, r := range e.order {
		if !e.usedRecs[r.Name] {
			continue
		}
		slots, _, ok := e.layout(r)
		if !ok {
			continue // unreachable: a construction walked this layout already
		}
		var parts []string
		var bitmap []uint64
		for _, s := range slots {
			switch s.kind {
			case fkStr:
				parts = append(parts, "ptr", "i64")
			case fkRef:
				parts = append(parts, "ptr")
			case fkScalar:
				parts = append(parts, "i64")
			}
			if s.isRef {
				i := (s.off - 16) / 8
				for len(bitmap) <= i/64 {
					bitmap = append(bitmap, 0)
				}
				bitmap[i/64] |= 1 << (i % 64)
			}
		}
		if len(parts) == 0 {
			structs = append(structs, fmt.Sprintf("%%struct.%s = type {}", r.Name))
		} else {
			structs = append(structs, fmt.Sprintf("%%struct.%s = type { %s }", r.Name, strings.Join(parts, ", ")))
		}
		maps = append(maps, fmt.Sprintf("@.map.%s = private unnamed_addr constant %s", r.Name, mapLiteral(bitmap, len(slots))))
	}
	if len(structs) > 0 {
		groups = append(groups, structs)
	}
	if len(e.strs) > 0 {
		lines := make([]string, len(e.strs))
		for i, s := range e.strs {
			lines[i] = fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\"", s.name, len(s.data), irEscape(s.data))
		}
		groups = append(groups, lines)
	}
	if len(maps) > 0 {
		groups = append(groups, maps)
	}
	if e.errConst != "" {
		groups = append(groups, []string{e.errConst})
	}
	if len(e.ovfs) > 0 {
		groups = append(groups, e.ovfs)
	}
	if len(e.panics) > 0 {
		groups = append(groups, e.panics)
	}
	if len(e.envMaps) > 0 {
		groups = append(groups, e.envMaps)
	}
	if len(e.chanDescs) > 0 {
		groups = append(groups, e.chanDescs)
	}
	var decls []string
	for _, d := range declareLines {
		if e.declUsed[d.sym] {
			decls = append(decls, d.line)
		}
	}
	if len(decls) > 0 {
		groups = append(groups, decls)
	}
	for _, g := range groups {
		for _, l := range g {
			sb.WriteString(l + "\n")
		}
		sb.WriteString("\n")
	}
	for _, th := range e.thunks {
		sb.WriteString(th + "\n")
	}
	sb.WriteString("define i32 @__we_main() {\nentry:\n")
	sb.WriteString(e.body.String())
	sb.WriteString("}\n")
	return sb.String()
}

// isStringPayloadVariant is the variant-attribution back-check of design
// D3: name must be a variant of the E in main's `Result<(), E>` return
// annotation, carrying exactly one String payload. Typecheck already
// established the semantics; this only confirms the attribution from the
// module's own declarations, without leaning on typecheck internals.
func isStringPayloadVariant(sums map[string]map[string][]ast.TypeRef, ret ast.TypeRef, name string) bool {
	res, ok := ret.(*ast.NamedType)
	if !ok || res.Qual != "" || res.Name != "Result" || len(res.Args) != 2 {
		return false
	}
	errTy, ok := res.Args[1].(*ast.NamedType)
	if !ok || errTy.Qual != "" {
		return false
	}
	payload, ok := sums[errTy.Name][name]
	if !ok || len(payload) != 1 {
		return false
	}
	str, ok := payload[0].(*ast.NamedType)
	return ok && str.Qual == "" && str.Name == "String" && len(str.Args) == 0
}

// decodeStringLiteral resolves the chapter 1 escape set of a raw string
// literal (Text holds the source slice, quotes included) to the bytes the
// program means. A false return flags an interpolation hole — the one
// payload form M4 does not accept. The lexer has already validated the
// escape syntax, so every other path resolves.
func decodeStringLiteral(text string) (string, bool) {
	if len(text) < 2 || text[0] != '"' || text[len(text)-1] != '"' {
		return "", false
	}
	inner := text[1 : len(text)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c != '\\' {
			if c == '$' && i+1 < len(inner) && inner[i+1] == '{' {
				return "", false
			}
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(inner) {
			return "", false
		}
		switch inner[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '0':
			b.WriteByte(0)
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case '\'':
			b.WriteByte('\'')
		case 'u':
			r, end, ok := parseUnicodeEscape(inner, i)
			if !ok || !utf8.ValidRune(r) {
				return "", false
			}
			var buf [4]byte
			b.Write(buf[:utf8.EncodeRune(buf[:], r)])
			i = end
		default:
			return "", false
		}
	}
	return b.String(), true
}

// parseUnicodeEscape reads the `\u{1..6 hex digits}` form starting at the
// 'u' (index i) of an escape's host string, returning the rune value and
// the index of the closing brace.
func parseUnicodeEscape(s string, i int) (rune, int, bool) {
	if i+1 >= len(s) || s[i+1] != '{' {
		return 0, 0, false
	}
	end := strings.IndexByte(s[i+2:], '}')
	if end < 0 {
		return 0, 0, false
	}
	hex := s[i+2 : i+2+end]
	if len(hex) < 1 || len(hex) > 6 {
		return 0, 0, false
	}
	var v rune
	for j := 0; j < len(hex); j++ {
		d := hexDigit(hex[j])
		if d < 0 {
			return 0, 0, false
		}
		v = v*16 + rune(d)
	}
	return v, i + 2 + end, true
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// irEscape re-escapes decoded bytes for the c"..." constant form: printable
// ASCII except quote and backslash stays raw; every other byte — controls,
// high-bit, quote, backslash — becomes \XX uppercase hex. The doubled forms
// \" and \\ are deliberately unused: on the pinned toolchain the assembly
// lexer breaks on \" inside a c-string (verified empirically — the string
// terminates early), while the hex forms compile, link, and round-trip
// through __we_fail byte-exact.
func irEscape(bytes string) string {
	var b strings.Builder
	for i := 0; i < len(bytes); i++ {
		c := bytes[i]
		switch {
		case c == '"' || c == '\\' || c < 0x20 || c > 0x7E:
			fmt.Fprintf(&b, "\\%02X", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
