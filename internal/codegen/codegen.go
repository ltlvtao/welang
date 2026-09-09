// Package codegen emits textual LLVM IR (the M10b program-level face,
// design D1 of the testing-run change): a whole program — one root
// module plus its dependency modules — as one module of IR. Every We fn
// is defined under its module-qualified symbol with a slot global
// pointing at the real body, and call sites load the slot (design D3:
// mock interception fills these slots); the root main alone is the
// build entry (__we_main, no slot), while the synthesized test driver
// owns the entry in test mode. Bodies are the M9b statement set (scalars,
// strings, records, primitives, io, task/scope/select, ?, match,
// while/if, defer, one tail return). Records construct through the gc
// protocol (alloc, map store, root push, field stores — the outer object
// rooted before any nested allocation); Strings ride as double-word
// operands from a constant pool. Everything else that survives type
// checking stops at a boundary What. Emission is a pure function — no
// paths, no counters beyond first-appearance ordering, no time — which
// is chapter 21's same-input-same-output made byte-level.
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
// rows ride unchanged. M10b (design D8) retires bndOtherFns and
// bndTestModule with the multi-function widening and adds the two rows
// below.
const (
	bndMainBody     = "main bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"
	bndErrPayload   = "Err payloads beyond one plain string-literal variant argument"
	bndTopLets      = "top-level value bindings in code generation"
	bndTaskBody     = "task bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"
	bndCallbackBody = "callback bodies beyond straight-line scalar expressions (no control flow, blocking calls, io, or captures of outer bindings)"
	// M10b design D8, verbatim.
	bndGenericFns = "generic functions in code generation (monomorphization is the B-track codegen-full widening)"
	bndFnBody     = "function bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"
	// M10b design D7: assertEqual's comparand domain — the Eq-generic
	// face is the standard library's own widening, not this build's.
	bndAssertEqDomain = "assertEqual beyond the scalar, Bool, and String domains (the Eq-generic face is the standard library's own widening)"
)

func bndMain() *NotImplemented     { return &NotImplemented{What: bndMainBody} }
func bndErrPay() *NotImplemented   { return &NotImplemented{What: bndErrPayload} }
func bndTask() *NotImplemented     { return &NotImplemented{What: bndTaskBody} }
func bndCallback() *NotImplemented { return &NotImplemented{What: bndCallbackBody} }
func bndFn() *NotImplemented       { return &NotImplemented{What: bndFnBody} }
func bndEqDomain() *NotImplemented { return &NotImplemented{What: bndAssertEqDomain} }

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
	// M10b (design D4/D7): the virtual clock pair and the assertion
	// pair, appended after every M9b row — a program that references
	// none of them declares none, so every prior module's bytes ride
	// unchanged.
	{"__we_time_now", "declare i64 @__we_time_now()"},
	{"__we_time_sleep", "declare void @__we_time_sleep(i64)"},
	{"__we_assert_true", "declare void @__we_assert_true(i64)"},
	{"__we_assert_false", "declare void @__we_assert_false(i64)"},
	// M10b (design D5/D7, T5): the equality pair, the crossing face, and
	// the driver's boundary quartet, after the D4 rows under the same
	// discipline. The report takes the file and description as explicit
	// (ptr, i64) pairs — the constant pool carries no NUL — while the
	// failure reason arrives as a C string the await's Err payload holds.
	{"__we_assert_eq_i64", "declare void @__we_assert_eq_i64(i64, i64)"},
	{"__we_assert_eq_str", "declare void @__we_assert_eq_str(ptr, i64, ptr, i64)"},
	{"__we_advance", "declare void @__we_advance(i64)"},
	{"__we_test_begin", "declare void @__we_test_begin()"},
	{"__we_test_end", "declare void @__we_test_end()"},
	{"__we_test_report", "declare void @__we_test_report(ptr, i64, ptr, i64, i64, ptr)"},
	{"__we_test_summary", "declare void @__we_test_summary()"},
	// M10c (design D1/D6): the extracted driver's pair — the meta carrying
	// file/description as explicit (ptr, i64) pairs plus the declaration's
	// line/col (the exploration guards' anchor), the drive taking the
	// test's global ordinal and its thunk — and the fxgate's single point
	// of judgment (the name as a (ptr, i64) pair, the pool carrying no
	// NUL).
	{"__we_test_meta", "declare void @__we_test_meta(ptr, i64, ptr, i64, i64, i64)"},
	{"__we_test_drive", "declare void @__we_test_drive(i64, ptr)"},
	{"__we_explore_fx_check", "declare void @__we_explore_fx_check(ptr, i64)"},
}

// ProgModule is one module of a program emission (design D1): the module
// key its qualified symbols carry, the ModuleID the header line names
// (the single-file face's module string; the root's manifest name in
// project builds), the parsed file itself, and — for a test module — the
// source path the report lines name (the driver's file face; the key's
// dots cannot invert to slashes without it, T5).
type ProgModule struct {
	Key  string
	ID   string
	Path string
	File *ast.File
}

// ProgramMode selects the entry's owner: ModeBuild makes the root
// module's main the entry; ModeTest downgrades every main to an ordinary
// slotted fn and leaves __we_main to the synthesized driver (design D5).
type ProgramMode int

const (
	ModeBuild ProgramMode = iota
	ModeTest
)

// stdEntry is one std-library fn entry the emitter can call: the runtime
// symbol the slot holds and whether it yields an i64. The argument
// shapes are positional per entry (io takes the String pair, test takes
// one Bool, sleep takes one Int64).
type stdEntry struct {
	sym string
	ret bool // yields i64
}

// stdFnEntries is the closed std fn-entry table (design D3): entries are
// slottable call faces — the check face accepts mocks on these literals,
// so the run face must reach them through slots. assertEqual is not an
// entry (it routes to the eq helpers directly, T5); std.concurrent has
// no fn entries (its constructors and methods are primCtor faces, never
// slottable).
var stdFnEntries = map[string]map[string]stdEntry{
	"io": {
		"println": {sym: "__we_println"},
		"print":   {sym: "__we_print"},
	},
	"test": {
		"assertTrue":  {sym: "__we_assert_true"},
		"assertFalse": {sym: "__we_assert_false"},
	},
	"time": {
		"now":   {sym: "__we_time_now", ret: true},
		"sleep": {sym: "__we_time_sleep"},
	},
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

// strBinding is one String value face: either a literal still holding
// its decoded bytes (a use interns them lazily — an unused let emits
// nothing, the M8 discipline), or an operand pair a fn parameter or an
// aggregate return produced ("%s0"/"%s1" — already live registers).
type strBinding struct {
	data   string
	length int
	dataOp string // non-empty: the operand form wins over data/length
	lenOp  string
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
	sums    map[string]map[string][]ast.TypeRef // keyed "<module>.<sum>"
	sumsOrd map[string][]string                 // sum key -> variant names, decl order
	records map[string]*ast.RecordDecl          // keyed "<module>.<record>"
	order   []recRef
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

	// M10b program state (design D1/D3). Record and sum tables are
	// module-keyed above; the fields below carry the module graph, the
	// fn table, and the slot globals every We fn and every used std
	// entry gets.
	mode       ProgramMode
	rootKey    string                       // the entry-owning module's key ("main")
	rootID     string                       // the ModuleID header's name
	modKeys    map[string]bool              // the program's module keys
	modImports map[string]map[string]string // module key -> qualifier -> module key
	modStd     map[string]map[string]string // module key -> qualifier -> std key
	modConc    map[string]map[string]bool   // module key -> concurrent alias set
	stdQuals   map[string]string            // the walked module's qualifier -> std key
	fns        []fnDef                      // program fns, module order then source order
	fnTable    map[string]*fnDef            // "<key>.<name>" -> def
	fnsDone    []string                     // finished fn define texts
	slots      []string                     // slot globals, materialization order
	slotSeen   map[string]bool
	curKey     string // the module whose body is being walked/emitted
	curImports map[string]string

	// M10b test state (design D5). tests collects the TestDecls pass one
	// sees in test mode (module order, source order within); the tower
	// emitter turns each into a test fn, its mocks, and a wrapper, and
	// records the driver's step. testBody is set while a test fn's body
	// is being emitted — the one body a valueless return may leave early
	// from any depth (the checker's "(test)" context is valueless; a fn
	// body keeps its one-tail-return discipline).
	tests    []testRef
	testBody bool
	drives   []driveStep
}

// enterModule loads m's import faces into the working fields — call
// faces resolve through the importing module's own names (an alias one
// module binds never reaches another module's body).
func (e *emitter) enterModule(key string) {
	e.curKey = key
	e.curImports = e.modImports[key]
	e.stdQuals = e.modStd[key]
	e.concAlias = e.modConc[key]
}

// resolveQual maps one call qualifier to a target module key: the
// walked module's imports first (alias or last segment), then the
// qualifier itself when it names a program module — the no-import form
// of a cross-module call resolves by module key (the M8 erasure parity:
// the io qualifier resolves by name without the import too).
func (e *emitter) resolveQual(name string) string {
	if k, ok := e.curImports[name]; ok && e.modKeys[k] {
		return k
	}
	if e.modKeys[name] {
		return name
	}
	return ""
}

// resolveStd maps one call qualifier to a std module key: the walked
// module's std imports first, then the qualifier itself when it spells
// a std module name (the M8 fallback that keeps the import-less io form
// equal to the imported form).
func (e *emitter) resolveStd(name string) string {
	if k, ok := e.stdQuals[name]; ok {
		return k
	}
	if _, ok := stdFnEntries[name]; ok {
		return name
	}
	return ""
}

// recRef pairs a record declaration with its module-qualified name — the
// symbol tables key by "<key>.<Name>" so same-named records across
// modules cannot collide (one layout each, their own map descriptors).
type recRef struct {
	name string
	decl *ast.RecordDecl
}

// fnDef is one program fn: its module key, its source name, and the
// declaration. The define's symbol is "<key>.<name>". The ABI classifies
// lazily (at the define or the first call site, whichever comes first —
// call sites in the entry precede the defines) and caches here, in the
// callee's own module's tables.
type fnDef struct {
	key   string
	name  string
	decl  *ast.FnDecl
	abi   fnAbi
	abiOK bool
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
// M10b adds the fn body as a fourth context (design D8) — the same M9b
// statement set under its own anchor word.
type bodyCtx int

const (
	ctxMain bodyCtx = iota
	ctxTask
	ctxCallback
	ctxFn
)

func (e *emitter) bnd() *NotImplemented {
	switch e.ctx {
	case ctxTask:
		return bndTask()
	case ctxCallback:
		return bndCallback()
	case ctxFn:
		return bndFn()
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
// name at the call site) — the single-file face of program emission
// (design D1): one root module keyed "main". A non-nil NotImplemented
// means f was type-clean but outside the acceptance set; the IR string
// is then empty.
func Emit(f *ast.File, module string) (string, *NotImplemented) {
	return EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: module, File: f}})
}

// EmitProgram renders the whole program — dependency modules in graph
// order, the root module last — as one module of IR (design D1). Pass one
// walks every module's declarations: sums and records under module keys
// (same-named declarations across modules cannot collide), the import
// faces per module, and the fn table — every We fn; the root build main
// alone is the entry instead. Declarations that never reach a runtime
// value — newtypes, interfaces, impls, effects — emit zero IR; a
// record's type and map lines appear only when a construction uses it.
// Pass two emits bodies: the entry first (its value numbering starts at
// v0, so every single-main module rides byte-identical through the
// widening), then every fn define under its module-qualified symbol,
// each with its slot global.
func EmitProgram(mode ProgramMode, mods []ProgModule) (string, *NotImplemented) {
	if len(mods) == 0 {
		return "", bndMain() // defensive: an empty program
	}
	e := &emitter{
		sums:       make(map[string]map[string][]ast.TypeRef),
		sumsOrd:    make(map[string][]string),
		records:    make(map[string]*ast.RecordDecl),
		strEnv:     make(map[string]strBinding),
		gcEnv:      make(map[string]gcBinding),
		strPool:    make(map[string]string),
		usedRecs:   make(map[string]bool),
		declUsed:   make(map[string]bool),
		ctx:        ctxMain,
		scalars:    make(map[string]scalarSlot),
		sums2:      make(map[string]sumSlot),
		prims:      make(map[string]string),
		modKeys:    make(map[string]bool),
		modImports: make(map[string]map[string]string),
		modStd:     make(map[string]map[string]string),
		modConc:    make(map[string]map[string]bool),
		fnTable:    make(map[string]*fnDef),
		slotSeen:   make(map[string]bool),
		mode:       mode,
	}
	root := mods[len(mods)-1]
	e.rootKey, e.rootID = root.Key, root.ID

	// Pass one: tables. Each module's import faces record under its own
	// key — a body resolves call qualifiers through its module's names
	// only (an alias one module binds never reaches another's body).
	var entry *ast.FnDecl
	for _, m := range mods {
		e.modImports[m.Key] = make(map[string]string)
		e.modStd[m.Key] = make(map[string]string)
		e.modConc[m.Key] = make(map[string]bool)
		e.modKeys[m.Key] = true
		e.enterModule(m.Key)
		for _, it := range m.File.Items {
			switch d := it.(type) {
			case *ast.SumDecl:
				variants := make(map[string][]ast.TypeRef, len(d.Variants))
				names := make([]string, len(d.Variants))
				for i, v := range d.Variants {
					variants[v.Name] = v.Payload
					names[i] = v.Name
				}
				key := m.Key + "." + d.Name
				e.sums[key] = variants
				e.sumsOrd[key] = names
			case *ast.FnDecl:
				if len(d.TypeParams) != 0 {
					return "", &NotImplemented{What: bndGenericFns}
				}
				if d.Name == "main" && m.Key == root.Key && mode == ModeBuild && entry == nil {
					entry = d
					continue
				}
				fd := &fnDef{key: m.Key, name: d.Name, decl: d}
				e.fns = append(e.fns, *fd)
				e.fnTable[m.Key+"."+d.Name] = fd
			case *ast.TopLet:
				return "", &NotImplemented{What: bndTopLets}
			case *ast.RecordDecl:
				key := m.Key + "." + d.Name
				if _, seen := e.records[key]; !seen {
					e.order = append(e.order, recRef{name: key, decl: d})
				}
				e.records[key] = d
			case *ast.NewtypeDecl:
				// Newtypes are zero-cost wrappers (chapter 8): the layout
				// erases and their value expressions stop at the body
				// boundary within the accepted shapes.
				continue
			case *ast.InterfaceDecl, *ast.ImplDecl:
				// Interfaces and impls erase (design D11 of the generics
				// change): a declaration-level fact only — the member sets
				// live in the type stage — so neither emits IR. An
				// explicit case, not a silent fall-through: a future form
				// arriving here must decide, never vanish.
				continue
			case *ast.EffectDecl:
				// Effect declarations erase (M7 design D10): chapter 16 is
				// a compile-time discipline — the check stage consumes
				// every segment, and the tag names reach no IR and no
				// runtime face.
				continue
			case *ast.TestDecl:
				// Test declarations are the run tower's own material
				// (design D5): a test build collects them here (module
				// order, source order within) and the tower emitter gives
				// each a test fn, its mocks, and a wrapper; a build mode
				// contributes its imports and fns and nothing else. An
				// explicit case, never a silent skip. A mock parses only
				// at a test body's own depth (the parser's E1802 holds
				// every other position — it is a statement, not an item),
				// and advanceTime's legal positions lie inside the test
				// extent (E1806 outside it) — both live strictly inside
				// this declaration, so neither needs a walk case of its
				// own here.
				if mode == ModeTest {
					path := m.Path
					if path == "" {
						// The direct-EmitProgram face (unit tests): a test
						// module's key is its path with dots, so the inverse
						// renders the report's file string.
						path = strings.ReplaceAll(m.Key, ".", "/") + ".we"
					}
					e.tests = append(e.tests, testRef{key: m.Key, path: path, decl: d})
				}
				continue
			case *ast.Import:
				// Only the std segment erases (design D4): a std import
				// item leaves no IR trace and brings its call faces. A
				// local import names a program module — its fns ride the
				// fn table under the joined path key; the import itself
				// emits nothing (the link is the call's qualifier, not a
				// global).
				if len(d.Path) > 0 && d.Path[0] == "std" {
					if len(d.Path) == 2 {
						a := d.Alias
						if a == "" {
							a = d.Path[1]
						}
						switch d.Path[1] {
						case "concurrent":
							e.concAlias[a] = true
						case "io", "test", "time":
							e.stdQuals[a] = d.Path[1]
						default:
							return "", e.bnd() // an unknown std module is the check stage's to reject; never silent
						}
					}
					continue
				}
				a := d.Alias
				if a == "" {
					a = d.Path[len(d.Path)-1]
				}
				e.curImports[a] = strings.Join(d.Path, ".")
			}
		}
	}

	// Pass two: bodies. The entry emits first — value numbering starts at
	// v0 there — then every fn define, each under the snapshot protocol
	// the task thunks use (its own local environments and gc window).
	if mode == ModeBuild {
		if entry == nil {
			// Defensive: Project-mode typecheck rejects a missing main (E1305).
			return "", bndMain()
		}
		e.enterModule(root.Key)
		e.mainRet = entry.Ret
		items := entry.Body.Items
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
	}
	for i := range e.fns {
		if ni := e.emitFnDefine(&e.fns[i]); ni != nil {
			return "", ni
		}
	}
	if mode == ModeTest {
		// The test tower (design D5): the test and mock defines ride
		// after the program fns, then the driver's steps land in the
		// body render emits as __we_main.
		if ni := e.emitTestTower(); ni != nil {
			return "", ni
		}
	}
	return e.render(e.rootID), nil
}

// testRef is one collected TestDecl: its module key (the qualified
// symbols' prefix), the source path the report lines name, and the
// declaration itself.
type testRef struct {
	key  string
	path string
	decl *ast.TestDecl
}

// mockInstall is one mock the driver installs for a test: the slot it
// rides, the mock fn's symbol, and the real body's symbol the restore
// stores back.
type mockInstall struct {
	slot    string
	mock    string
	restore string
}

// driveStep is one test's slice of the synthesized driver: the report's
// file and description strings (raw bytes — the driver interns them at
// emission), the declaration's line/col (the meta pair's anchor face),
// the owning module's key (the drive thunk's name), the wrapper the
// spawn names, and the mock installs in source order.
type driveStep struct {
	file  string
	desc  string
	line  int
	col   int
	key   string
	wrap  string
	mocks []mockInstall
}

// emitTestTower emits every collected test (design D5): per test module,
// source order — a test fn under "<key>.test.<n>", its mocks under
// "<key>.mock.<n>" (a per-module counter across the module's tests), and
// an internal wrapper "<key>.wrap.<n>" carrying the spawn protocol; the
// driver's steps record here, and emitDriver renders them into the body
// the entry carries.
func (e *emitter) emitTestTower() *NotImplemented {
	testN := map[string]int{}
	mockN := map[string]int{}
	for i := range e.tests {
		tr := &e.tests[i]
		n := testN[tr.key]
		testN[tr.key] = n + 1
		e.enterModule(tr.key)
		step := driveStep{file: tr.path, desc: tr.decl.Desc, line: tr.decl.Line, col: tr.decl.Col,
			key: tr.key, wrap: fmt.Sprintf("@%s.wrap.%d", tr.key, n)}
		var mocks []*ast.MockDecl
		for _, st := range tr.decl.Body.Items {
			if md, ok := st.(*ast.MockDecl); ok {
				mocks = append(mocks, md)
			}
		}
		for _, md := range mocks {
			m := mockN[tr.key]
			mockN[tr.key] = m + 1
			inst, ni := e.emitMockDefine(md, tr.key, m)
			if ni != nil {
				return ni
			}
			step.mocks = append(step.mocks, inst)
		}
		if ni := e.emitTestDefine(tr.decl, tr.key, n); ni != nil {
			return ni
		}
		e.thunks = append(e.thunks, fmt.Sprintf(
			"define internal i64 @%s.wrap.%d(ptr %%env) {\nentry:\n  call void @%s.test.%d()\n  ret i64 0\n}\n",
			tr.key, n, tr.key, n))
		e.drives = append(e.drives, step)
	}
	e.enterModule(e.rootKey)
	e.emitDriver()
	return nil
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
		reg, rkey, ni := e.emitConstruct(init)
		if ni != nil {
			return ni
		}
		if s.Name != "_" {
			e.gcEnv[s.Name] = gcBinding{rec: rkey, reg: reg}
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
	case ckStr:
		if name != "_" {
			e.strEnv[name] = res.strBind
		}
		return nil
	case ckGc:
		if name != "_" {
			e.gcEnv[name] = gcBinding{rec: res.recKey, reg: res.gcReg}
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

// emitIoCall emits `qual.println(arg)` / `qual.print(arg)` with a String
// argument — through the io slot (design D3: the check face accepts mocks
// on these literals, so the run face reaches them indirectly). The
// qualifier is any name — resolution happened at the type stage; a
// qualifier that the body binds locally is a member call on that value
// instead (a record field call is outside the set).
func (e *emitter) emitIoCall(call *ast.Call) *NotImplemented {
	fn, ok := call.Fn.(*ast.Member)
	if !ok {
		return e.bnd()
	}
	qual, ok := fn.Recv.(*ast.Ident)
	if !ok {
		return e.bnd()
	}
	if e.isLocalName(qual.Name) {
		return e.bnd()
	}
	var sym string
	switch fn.Name {
	case "println":
		sym = "__we_println"
	case "print":
		sym = "__we_print"
	default:
		return e.bnd()
	}
	if len(call.Args) != 1 {
		return e.bnd()
	}
	p, l, ni := e.emitStringExpr(call.Args[0])
	if ni != nil {
		return ni
	}
	slot := e.slotFor("io."+fn.Name, "@"+sym)
	e.use(sym)
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = load ptr, ptr %s", v, slot))
	e.inst(fmt.Sprintf("call void %%%s(ptr %s, i64 %s)", v, p, l))
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
	ckStr                  // a String double word (M10b fn returns)
	ckGc                   // a record pointer (M10b fn returns and ctors)
)

type callResult struct {
	kind    callKind
	i64     string // the ckI64/ckPrim operand
	isFloat bool
	sum     sumSlot    // the ckSum slots
	strBind strBinding // the ckStr operand pair
	gcReg   string     // the ckGc pointer operand
	recKey  string     // the ckGc record's module-qualified key
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
// as i64 immediates, floats as the exact double hex bits — the bare hex
// form, the consumer spells the type (every use site knows the domain
// from the isFloat flag; a prefixed operand here would double the type
// spelling there. Found while pinning the M10b float call face — no
// prior golden reached a float path).
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
		return fmt.Sprintf("0x%016X", math.Float64bits(f)), true, true
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
		if ref := e.chanElemRecord(typ); ref != "" {
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
// gc record T names in the walked module (its module-qualified key), or
// "" when the element domain is not a traced record reference.
func (e *emitter) chanElemRecord(typ ast.TypeRef) string {
	n, ok := typ.(*ast.NamedType)
	if !ok || n.Name != "Channel" || len(n.Args) != 1 {
		return ""
	}
	elem, ok := n.Args[0].(*ast.NamedType)
	if !ok || elem.Qual != "" || len(elem.Args) != 0 {
		return ""
	}
	r, ok := e.records[e.curKey+"."+elem.Name]
	if !ok || r.Cat != "gc" || len(r.TypeParams) != 0 {
		return ""
	}
	return e.curKey + "." + elem.Name
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
// the bare record constructors and program fns of the walked module
// (M10b: every fn call site loads the callee's slot — design D3), the
// concurrent constructors (alias-qualified; a channel's element domain
// rides the binding's annotation), the primitive methods, and the std
// fn entries — the io pair (scalar argument through the direct i64
// renderer, the design's exception; String argument through the slot),
// the test assertions and the time pair through their slots. typ is the
// enclosing let's annotation, nil at statement position.
func (e *emitter) emitCall(call *ast.Call, typ ast.TypeRef) (callResult, *NotImplemented) {
	if id, ok := call.Fn.(*ast.Ident); ok {
		switch id.Name {
		case "panic":
			if len(call.Args) != 1 {
				return callResult{}, e.bnd()
			}
			return e.emitPanic(call.Args[0])
		case "todo":
			// The panic family's other message face (chapter 14): the
			// same literal discipline, the same task-fail tail.
			if len(call.Args) != 1 {
				return callResult{}, e.bnd()
			}
			return e.emitPanic(call.Args[0])
		case "assert":
			// The prelude's two-argument face (chapter 14's signature,
			// design D7): the condition runs, the failure tail rides the
			// task-fail ABI with the message.
			if len(call.Args) != 2 {
				return callResult{}, e.bnd()
			}
			return e.emitAssert(call.Args[0], call.Args[1])
		case "advanceTime":
			// The clock control's runtime face (chapter 20, design D4) —
			// a language-level name, no slot (design D3).
			if len(call.Args) != 1 {
				return callResult{}, e.bnd()
			}
			op, isF, ni := e.emitNumExpr(call.Args[0])
			if ni != nil {
				return callResult{}, ni
			}
			if isF {
				return callResult{}, e.bnd()
			}
			e.use("__we_advance")
			e.inst(fmt.Sprintf("call void @__we_advance(i64 %s)", op))
			return callResult{kind: ckVoid}, nil
		case "currentCancelSignal":
			if len(call.Args) != 0 {
				return callResult{}, e.bnd()
			}
			e.use("__we_sig_current")
			v := e.value()
			e.inst(fmt.Sprintf("%%%s = call ptr @__we_sig_current()", v))
			return callResult{kind: ckPrim, i64: "%" + v}, nil
		}
		// The bare record constructor: the positional face (the check
		// stage accepts both); a parsed program constructs through the
		// named-field form. Both land in the same protocol.
		if _, ok := e.records[e.curKey+"."+id.Name]; ok {
			return e.emitCtorCall(id.Name, call.Args)
		}
		// A program fn of the walked module, through its slot.
		if fd, ok := e.fnTable[e.curKey+"."+id.Name]; ok {
			return e.emitFnCall(fd, call.Args)
		}
		return callResult{}, e.bnd()
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
	if recv, ok := fn.Recv.(*ast.Ident); ok && !e.isLocalName(recv.Name) {
		// A program module's fn: the qualifier resolves through the walked
		// module's imports first, then by the module's own key (the
		// no-import form of a cross-module call resolves by key).
		if k := e.resolveQual(recv.Name); k != "" {
			fd, ok := e.fnTable[k+"."+fn.Name]
			if !ok {
				return callResult{}, e.bnd()
			}
			return e.emitFnCall(fd, call.Args)
		}
		// A std fn entry — slottable call faces, one row per entry.
		if sk := e.resolveStd(recv.Name); sk != "" {
			// assertEqual is a call face, not an entry (M10a's ruling, D3):
			// the two comparanda route straight to the equality pair.
			if sk == "test" && fn.Name == "assertEqual" {
				return e.emitAssertEqual(call.Args)
			}
			if ent, ok := stdFnEntries[sk][fn.Name]; ok {
				if sk == "io" {
					// The scalar shortcut stays a direct call: the i64
					// pair has no String bytes and no mock face (design
					// D3's exception).
					if len(call.Args) == 1 && e.argIsScalar(call.Args[0]) {
						op, isF, ni := e.emitNumExpr(call.Args[0])
						if ni != nil {
							return callResult{}, ni
						}
						if !isF {
							sym := "__we_println_i64"
							if fn.Name == "print" {
								sym = "__we_print_i64"
							}
							e.use(sym)
							e.inst(fmt.Sprintf("call void @%s(i64 %s)", sym, op))
							return callResult{kind: ckVoid}, nil
						}
					}
					ni := e.emitIoCall(call)
					if ni != nil {
						return callResult{}, ni
					}
					return callResult{kind: ckIo}, nil
				}
				return e.emitStdEntryCall(sk+"."+fn.Name, ent, call.Args)
			}
		}
	}
	return callResult{}, e.bnd()
}

// emitStdEntryCall emits one test/time entry call through its slot: the
// argument shapes are positional per entry (Bool rides the i64 domain,
// the sleep duration one Int64), and now yields its i64.
func (e *emitter) emitStdEntryCall(name string, ent stdEntry, args []ast.Expr) (callResult, *NotImplemented) {
	slot := e.slotFor(name, "@"+ent.sym)
	e.use(ent.sym)
	fp := e.value()
	e.inst(fmt.Sprintf("%%%s = load ptr, ptr %s", fp, slot))
	if ent.ret {
		if len(args) != 0 {
			return callResult{}, e.bnd()
		}
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 %%%s()", v, fp))
		return callResult{kind: ckI64, i64: "%" + v}, nil
	}
	if len(args) != 1 {
		return callResult{}, e.bnd()
	}
	op, isF, ni := e.emitNumExpr(args[0])
	if ni != nil {
		return callResult{}, ni
	}
	if isF {
		return callResult{}, e.bnd()
	}
	e.inst(fmt.Sprintf("call void %%%s(i64 %s)", fp, op))
	return callResult{kind: ckVoid}, nil
}

// emitCtorCall is the bare record-constructor face: positional arguments
// map onto the declaration's field order, then the named-field protocol
// takes over (one construction path, two source shapes).
func (e *emitter) emitCtorCall(name string, args []ast.Expr) (callResult, *NotImplemented) {
	rec, ok := e.records[e.curKey+"."+name]
	if !ok || len(rec.TypeParams) != 0 || len(args) != len(rec.Fields) {
		return callResult{}, e.bnd()
	}
	fs := make([]ast.FieldInit, len(args))
	for i, fd := range rec.Fields {
		fs[i] = ast.FieldInit{Name: fd.Name, Value: args[i]}
	}
	reg, rkey, ni := e.emitConstruct(&ast.Construct{Name: name, Fields: fs})
	if ni != nil {
		return callResult{}, ni
	}
	return callResult{kind: ckGc, gcReg: reg, recKey: rkey}, nil
}

// slotFor materializes one slot global at its first call site (mirroring
// the constant pool's first-appearance naming): every used fn face gets
// exactly one slot, in first-call order. The root build main never
// passes through here — the entry is not mockable (design D3's
// exception).
func (e *emitter) slotFor(name, target string) string {
	if !e.slotSeen[name] {
		e.slotSeen[name] = true
		e.slots = append(e.slots, "@slot."+name+" = global ptr "+target)
	}
	return "@slot." + name
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
		reg, _, ni := e.emitConstruct(c)
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
		if r, ok := st.(*ast.Return); ok {
			// Returns sit at the body tail only — the one widening is the
			// test context's valueless early exit (a failed test's source
			// discharges its scope handles through a return the runtime
			// never reaches past the task-fail tail).
			if !e.testBody {
				return e.bnd()
			}
			if ni := e.emitTestReturn(r); ni != nil {
				return ni
			}
			continue
		}
		if ni := e.emitStmt(st); ni != nil {
			return ni
		}
	}
	return nil
}

// emitTestReturn emits the test context's valueless early exit: ret void
// plus the never-reached continuation label that keeps the block
// structure well-formed for whatever lexically follows.
func (e *emitter) emitTestReturn(r *ast.Return) *NotImplemented {
	if r.HasValue {
		return e.bnd()
	}
	e.inst("ret void")
	n := e.blocks
	e.blocks++
	e.label(fmt.Sprintf("tr%d", n))
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

// emitAssert emits the prelude's two-argument assert (design D7): the
// condition under the i1 branch, the failure tail riding the task-fail
// ABI with the message literal (the same NUL-terminated constant face
// emitPanic interns), the passing face simply continuing. The message
// keeps the literal discipline (a non-literal stops at the body word).
func (e *emitter) emitAssert(cond, msg ast.Expr) (callResult, *NotImplemented) {
	lit, ok := msg.(*ast.Literal)
	if !ok || lit.Kind != "string" {
		return callResult{}, e.bnd()
	}
	data, ok := decodeStringLiteral(lit.Text)
	if !ok {
		return callResult{}, e.bnd()
	}
	c, ni := e.condI64(cond)
	if ni != nil {
		return callResult{}, ni
	}
	n := e.blocks
	e.blocks++
	pass := fmt.Sprintf("asp%d", n)
	fail := fmt.Sprintf("asf%d", n)
	e.inst(fmt.Sprintf("br i1 %s, label %%%s, label %%%s", c, pass, fail))
	e.label(fail)
	e.use("__we_task_fail")
	cn := fmt.Sprintf("@.p%d", len(e.panics))
	e.panics = append(e.panics, fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\\00\"", cn, len(data)+1, irEscape(data)))
	e.inst(fmt.Sprintf("call void @__we_task_fail(ptr %s)", cn))
	e.inst("unreachable")
	e.label(pass)
	return callResult{kind: ckVoid}, nil
}

// emitAssertEqual routes the two comparanda to the equality pair (design
// D7): the integer widths and Bool ride the i64 domain, String pairs the
// four-word face. The family is decided by shape before any emission —
// a trial emission could leave a call twice in the stream. The domain
// itself is the check stage's face (bndAssertEqDomain); a shape outside
// both families here is defensive only.
func (e *emitter) emitAssertEqual(args []ast.Expr) (callResult, *NotImplemented) {
	if len(args) != 2 {
		return callResult{}, e.bnd()
	}
	strFace := func(x ast.Expr) bool {
		switch v := x.(type) {
		case *ast.Literal:
			return v.Kind == "string"
		case *ast.Ident:
			_, ok := e.strEnv[v.Name]
			return ok
		case *ast.Member:
			return true // the String field-chain face
		}
		return false
	}
	// Records and sums sit beyond the comparand domain: their equality
	// is the Eq-generic face, the standard library's own widening.
	domainFace := func(x ast.Expr) bool {
		switch v := x.(type) {
		case *ast.Ident:
			_, g := e.gcEnv[v.Name]
			_, s := e.sums2[v.Name]
			return g || s
		case *ast.Construct:
			return true
		}
		return false
	}
	if domainFace(args[0]) || domainFace(args[1]) {
		return callResult{}, bndEqDomain()
	}
	// A call comparand contributes its own result's face: an
	// Int64-returning call rides the numeric route, a String-returning
	// one the string route. The M9b expression emitters take no direct
	// calls (a call enters through a let binding); the comparand
	// position is the one widened face — the goldens pass calls straight
	// in — so the resolution lives here, not in those sets.
	var ops [2]string       // a numeric call's operand
	var strs [2]*strBinding // a string call's operand pair
	isCall, isStr := [2]bool{}, [2]bool{}
	for i, x := range args {
		c, ok := x.(*ast.Call)
		if !ok {
			isStr[i] = strFace(x)
			continue
		}
		res, ni := e.emitCall(c, nil)
		if ni != nil {
			return callResult{}, ni
		}
		isCall[i] = true
		switch res.kind {
		case ckI64:
			if res.isFloat {
				return callResult{}, bndEqDomain()
			}
			ops[i] = res.i64
		case ckStr:
			b := res.strBind
			strs[i] = &b
			isStr[i] = true
		default:
			return callResult{}, bndEqDomain()
		}
	}
	if isStr[0] || isStr[1] {
		var sp, sl [2]string
		for i, x := range args {
			if isCall[i] {
				sp[i], sl[i] = strs[i].dataOp, strs[i].lenOp
				continue
			}
			p, l, ni := e.emitStringExpr(x)
			if ni != nil {
				return callResult{}, ni
			}
			sp[i], sl[i] = p, l
		}
		e.use("__we_assert_eq_str")
		e.inst(fmt.Sprintf("call void @__we_assert_eq_str(ptr %s, i64 %s, ptr %s, i64 %s)", sp[0], sl[0], sp[1], sl[1]))
		return callResult{kind: ckVoid}, nil
	}
	var np [2]string
	for i, x := range args {
		if isCall[i] {
			np[i] = ops[i]
			continue
		}
		op, isF, ni := e.emitNumExpr(x)
		if ni != nil {
			return callResult{}, ni
		}
		if isF {
			return callResult{}, bndEqDomain()
		}
		np[i] = op
	}
	e.use("__we_assert_eq_i64")
	e.inst(fmt.Sprintf("call void @__we_assert_eq_i64(i64 %s, i64 %s)", np[0], np[1]))
	return callResult{kind: ckVoid}, nil
}

// emitQuestion emits `expr?`: the sum's Err branch runs the error tail —
// a panic-message payload (an await's TaskPanic) hands the C string to
// the task-fail ABI, a static report line (a timeout scope) writes the
// M8 fail constant — and the Ok branch continues with the payload as
// the expression's value. The two tails are the entry's faces (the
// scheduler settles the report and the exit); inside a fn a `?` stops —
// a fn returns its Result, the B-track widens the propagation.
func (e *emitter) emitQuestion(p *ast.Prop) (callResult, *NotImplemented) {
	if e.ctx == ctxFn {
		return callResult{}, bndFn()
	}
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
		if r, ok := st.(*ast.Return); ok {
			// A nested block's Return discipline: fn bodies take returns
			// at the tail only; the test context's valueless early exit
			// rides through the scope the same way.
			if !e.testBody {
				return callResult{}, e.bnd()
			}
			if ni := e.emitTestReturn(r); ni != nil {
				return callResult{}, ni
			}
			continue
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
// field. The pair may be a constant global plus immediate, two loaded
// registers, or — for a name an fn parameter or an aggregate return
// produced — the operand pair already live in registers.
func (e *emitter) emitStringExpr(x ast.Expr) (string, string, *NotImplemented) {
	switch v := x.(type) {
	case *ast.Literal:
		if v.Kind != "string" {
			return "", "", e.bnd()
		}
		data, ok := decodeStringLiteral(v.Text)
		if !ok {
			return "", "", e.bnd()
		}
		return e.intern(data), strconv.Itoa(len(data)), nil
	case *ast.Ident:
		b, ok := e.strEnv[v.Name]
		if !ok {
			return "", "", e.bnd()
		}
		if b.dataOp != "" {
			return b.dataOp, b.lenOp, nil
		}
		return e.intern(b.data), strconv.Itoa(b.length), nil
	case *ast.Member:
		return e.emitFieldChainString(v)
	default:
		return "", "", e.bnd()
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
				return "", "", e.bnd()
			}
			return e.walkChain(g.reg, g.rec, hops)
		case *ast.Member:
			x = r
		default:
			return "", "", e.bnd()
		}
	}
}

// walkChain emits the loads for hops over base (a record pointer of the
// type recKey names); the last hop must land on a String field.
func (e *emitter) walkChain(base, recKey string, hops []string) (string, string, *NotImplemented) {
	for _, h := range hops[:len(hops)-1] {
		slot, ok := e.fieldSlotOf(recKey, h)
		if !ok || slot.kind != fkRef {
			return "", "", e.bnd()
		}
		base = e.gepLoadPtr(base, slot.off)
		recKey = slot.typ
	}
	slot, ok := e.fieldSlotOf(recKey, hops[len(hops)-1])
	if !ok || slot.kind != fkStr {
		return "", "", e.bnd()
	}
	return e.gepLoadPtr(base, slot.off), e.gepLoadI64(base, slot.off+8), nil
}

// layout computes a record's field slots: offsets from 16 (the frozen
// header {map@0, size@8} of design D6), sizes, and the reference bitmap
// derived kinds. key is the module the record declares in — a reference
// field resolves in the record's own module, so fkRef targets carry
// key+"."+FieldName. A field outside the M8 shape fails the whole
// emission.
func (e *emitter) layout(key string, rec *ast.RecordDecl) ([]fieldSlot, int, bool) {
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
			r, ok := e.records[key+"."+n.Name]
			if !ok || r.Cat != "gc" || len(r.TypeParams) != 0 {
				return nil, 0, false
			}
			slots[i].kind = fkRef
			slots[i].typ = key + "." + n.Name
			slots[i].isRef = true
			off += 8
		}
	}
	return slots, off, true
}

// recModKey splits a record's module-qualified key into the module key it
// declares in. Record names are identifiers (no '.'), so the last
// separator divides — a module key of a nested path may carry earlier
// separators of its own.
func recModKey(recKey string) string {
	if i := strings.LastIndex(recKey, "."); i >= 0 {
		return recKey[:i]
	}
	return recKey
}

// fieldSlotOf finds one field's slot by name; recKey is the record's
// module-qualified name.
func (e *emitter) fieldSlotOf(recKey, field string) (fieldSlot, bool) {
	rec, ok := e.records[recKey]
	if !ok {
		return fieldSlot{}, false
	}
	slots, _, ok := e.layout(recModKey(recKey), rec)
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
// parent unrooted — then the field stores in source order. The record
// resolves in the walked module (a qualified head resolves its module
// first — cross-module records construct the same way, each under its
// own module's symbols); the return pairs the pointer with the record's
// module-qualified key, the name every later field walk keys by.
func (e *emitter) emitConstruct(c *ast.Construct) (string, string, *NotImplemented) {
	key := e.curKey
	if c.Qual != "" {
		key = e.resolveQual(c.Qual)
		if key == "" {
			return "", "", e.bnd()
		}
	}
	rec, ok := e.records[key+"."+c.Name]
	if !ok || rec.Cat != "gc" || len(rec.TypeParams) != 0 || len(c.TypeArgs) != 0 {
		return "", "", e.bnd()
	}
	slots, total, ok := e.layout(key, rec)
	if !ok {
		return "", "", e.bnd()
	}
	rkey := key + "." + rec.Name
	e.usedRecs[rkey] = true
	e.use("__we_alloc")
	e.use("__we_root_push")
	e.pushes++
	reg := "%" + e.value()
	e.inst(fmt.Sprintf("%s = call ptr @__we_alloc(i64 %d)", reg, total))
	e.inst(fmt.Sprintf("store ptr @.map.%s, ptr %s", rkey, reg))
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", reg))
	byName := make(map[string]int, len(rec.Fields))
	for i, fd := range rec.Fields {
		byName[fd.Name] = i
	}
	for _, fi := range c.Fields {
		idx, ok := byName[fi.Name]
		if !ok {
			return "", "", e.bnd()
		}
		slot := slots[idx]
		switch slot.kind {
		case fkStr:
			lit, ok := fi.Value.(*ast.Literal)
			if !ok || lit.Kind != "string" {
				return "", "", e.bnd()
			}
			data, ok := decodeStringLiteral(lit.Text)
			if !ok {
				return "", "", e.bnd()
			}
			e.gepStore(reg, slot.off, "ptr "+e.intern(data))
			e.gepStore(reg, slot.off+8, "i64 "+strconv.Itoa(len(data)))
		case fkRef:
			nc, ok := fi.Value.(*ast.Construct)
			if !ok {
				return "", "", e.bnd()
			}
			child, _, ni := e.emitConstruct(nc)
			if ni != nil {
				return "", "", ni
			}
			e.gepStore(reg, slot.off, "ptr "+child)
		case fkScalar:
			imm, ok := scalarImmediate(fi.Value)
			if !ok {
				return "", "", e.bnd()
			}
			e.gepStore(reg, slot.off, "i64 "+imm)
		}
	}
	return reg, rkey, nil
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
		if !isStringPayloadVariant(e.sums, e.rootKey, e.mainRet, vfn.Name) {
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

// --- the M10b fn defines and calls (design D1/D2/D3) --------------------------

// fnAbiKind is one family of the D2 table — the six calling shapes a
// value or parameter can ride. Value records share the gc record's ptr
// family (this reference build has one heap representation, so the
// design's sret row stays unused).
type fnAbiKind int

const (
	abiVoid fnAbiKind = iota
	abiI64
	abiDouble
	abiStr // { ptr, i64 }
	abiGc  // ptr
	abiSum // { i64, i64 }
)

type fnParamAbi struct {
	kind fnAbiKind
	key  string // the record/sum's module-qualified key (abiGc/abiSum)
}

// fnAbi is one fn's calling shape: the return family plus one entry per
// source parameter (a String or sum parameter takes two IR words).
type fnAbi struct {
	ret      fnAbiKind
	retTyp   string   // the define's result type spelling
	retKey   string   // the ret record/sum's key ("Result" for the prelude sum)
	variants []string // the ret sum's variant names, decl order (abiSum)
	params   []fnParamAbi
}

// fnAbiOf classifies one fn's signature against the walked module's
// tables (enter the fn's module first). A type the families cannot name —
// a qualified reference, a generic application, an unknown name —
// reports false and the caller stops at the fn body word.
func (e *emitter) fnAbiOf(d *ast.FnDecl) (fnAbi, bool) {
	class := func(t ast.TypeRef) (fnAbiKind, string, bool) {
		n, ok := t.(*ast.NamedType)
		if !ok || n.Qual != "" {
			return abiVoid, "", false
		}
		if n.Name == "Result" && len(n.Args) == 2 {
			return abiSum, "Result", true // the prelude sum — Ok/Err
		}
		if len(n.Args) != 0 {
			return abiVoid, "", false
		}
		switch n.Name {
		case "String":
			return abiStr, "", true
		case "Float64":
			return abiDouble, "", true
		case "Int64", "Int32", "Int16", "Int8", "UInt64", "UInt32", "UInt16", "UInt8", "Bool":
			return abiI64, "", true
		}
		key := e.curKey + "." + n.Name
		if _, ok := e.records[key]; ok {
			return abiGc, key, true
		}
		if _, ok := e.sums[key]; ok {
			return abiSum, key, true
		}
		return abiVoid, "", false
	}
	var abi fnAbi
	switch t := d.Ret.(type) {
	case nil:
		abi.ret = abiVoid
		abi.retTyp = "void"
	case *ast.NamedType:
		k, key, ok := class(t)
		if !ok {
			return abi, false
		}
		abi.ret, abi.retKey = k, key
		switch k {
		case abiI64:
			abi.retTyp = "i64"
		case abiDouble:
			abi.retTyp = "double"
		case abiStr:
			abi.retTyp = "{ ptr, i64 }"
		case abiGc:
			abi.retTyp = "ptr"
		case abiSum:
			abi.retTyp = "{ i64, i64 }"
			if key == "Result" {
				abi.variants = []string{"Ok", "Err"}
			} else {
				abi.variants = e.sumsOrd[key]
			}
		}
	default:
		return abi, false
	}
	for _, p := range d.Params {
		k, key, ok := class(p.Type)
		if !ok {
			return abi, false
		}
		abi.params = append(abi.params, fnParamAbi{kind: k, key: key})
	}
	return abi, true
}

// classify resolves fd's ABI lazily — in fd's own module's tables — and
// caches it on the def (the fnTable and the fns slice share the object).
func (e *emitter) classify(fd *fnDef) (fnAbi, bool) {
	if !fd.abiOK {
		caller := e.curKey
		e.enterModule(fd.key)
		fd.abi, fd.abiOK = e.fnAbiOf(fd.decl)
		e.enterModule(caller)
	}
	return fd.abi, fd.abiOK
}

// isCustomEffect reports whether one fn's effect segment names a custom
// tag (M10c design D6): any tag beyond the three built-in bare keys — a
// bare custom tag and a qualified mod.name form alike are custom (chapter
// 16's built-ins are bare language-level tags, so a qualified tag never
// names one). The predicate reads the declaration's own segment, no
// typecheck help — the check tower already owns the segment's legality.
func isCustomEffect(d *ast.FnDecl) bool {
	for _, t := range d.EffectTags {
		switch t {
		case "io", "net", "time":
		default:
			return true
		}
	}
	return false
}

// fnSlotTarget is the symbol a fn's slot holds by default (M10c design
// D6): the passthrough gate for a custom-effect fn — reaching the gate
// means the call took the slot's default, which means no mock was on the
// path — and the real body for every built-in-or-pure fn (the gate is an
// exploration face, not a calling-face change).
func fnSlotTarget(fd *fnDef) string {
	if isCustomEffect(fd.decl) {
		return fmt.Sprintf("@%s.%s.fxgate", fd.key, fd.name)
	}
	return fmt.Sprintf("@%s.%s", fd.key, fd.name)
}

// emitFxGate emits one custom-effect fn's passthrough gate (M10c design
// D6): the same ABI as the real define, a guard check at the entry, then
// a direct call of the real body under its own name — never through the
// slot, so the gate cannot re-enter itself. The check's name operand is
// the declaration's bare name; the result register %r is fixed (the gate
// reads no parameter environments, so a parameter literally named r —
// the bindDefineParams spelling — is the one collision this synthesized
// face accepts, a B-track cleanup's to own).
func (e *emitter) emitFxGate(fd *fnDef, abi fnAbi) {
	var ps []string
	for i, p := range fd.decl.Params {
		n := "%" + p.Name
		switch abi.params[i].kind {
		case abiDouble:
			ps = append(ps, "double "+n)
		case abiStr:
			ps = append(ps, "ptr "+n+"0", "i64 "+n+"1")
		case abiSum:
			ps = append(ps, "i64 "+n+"0", "i64 "+n+"1")
		case abiGc:
			ps = append(ps, "ptr "+n)
		default: // abiI64
			ps = append(ps, "i64 "+n)
		}
	}
	name := e.intern(fd.name)
	e.use("__we_explore_fx_check")
	// The forwarded arguments keep their typed spellings — the call reads
	// exactly as the define's own parameter list.
	call := fmt.Sprintf("call %s @%s.%s(%s)", abi.retTyp, fd.key, fd.name,
		strings.Join(ps, ", "))
	var body strings.Builder
	fmt.Fprintf(&body, "  call void @__we_explore_fx_check(ptr %s, i64 %d)\n", name, len(fd.name))
	if abi.ret == abiVoid {
		fmt.Fprintf(&body, "  %s\n  ret void\n", call)
	} else {
		fmt.Fprintf(&body, "  %%r = %s\n  ret %s %%r\n", call, abi.retTyp)
	}
	e.thunks = append(e.thunks, fmt.Sprintf(
		"define internal %s @%s.%s.fxgate(%s) {\nentry:\n%s}\n",
		abi.retTyp, fd.key, fd.name, strings.Join(ps, ", "), body.String()))
}

// emitFnDefine emits one program fn: the define under its
// module-qualified symbol, the parameter environments (scalars, the
// String double word, record pointers, the sum pair re-housed in two
// allocas), the body under the fn context — its own local environments
// and gc window, the snapshot protocol the task thunks use — and the
// family's return. The fn's gc window is its own: its pushes balance
// inside (return value, defers inverted, pops, ret), and a record return
// is rooted by its caller after the call — nothing may allocate between
// the callee's last pop and the caller's push. The slot global follows
// the define: every We fn is defined and slotted, called or not (design
// D1 — dead slots are dead weight, never a correctness face).
func (e *emitter) emitFnDefine(fd *fnDef) *NotImplemented {
	abi, ok := e.classify(fd)
	if !ok {
		return bndFn()
	}
	e.enterModule(fd.key)

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
	e.ctx = ctxFn
	e.body = strings.Builder{}
	e.scalars = make(map[string]scalarSlot)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.defers = nil
	e.caps = nil
	e.pushes = 0

	// The parameter list and environments (the shared define face).
	ps := e.bindDefineParams(fd.decl.Params, abi)

	// The statements; a return anywhere but the tail is a control-flow
	// shape outside the set.
	items := fd.decl.Body.Items
	var tail *ast.Return
	if len(items) > 0 {
		if r, ok := items[len(items)-1].(*ast.Return); ok {
			tail = r
			items = items[:len(items)-1]
		}
	}
	for _, st := range items {
		if _, ok := st.(*ast.Return); ok {
			restore()
			return bndFn()
		}
		if ni := e.emitStmt(st); ni != nil {
			restore()
			return ni
		}
	}
	ret, ni := e.fnRetVal(abi, tail)
	if ni != nil {
		restore()
		return ni
	}
	for i := len(e.defers) - 1; i >= 0; i-- {
		if ni := e.emitBlockStmts(e.defers[i].Items); ni != nil {
			restore()
			return ni
		}
	}
	for i := 0; i < e.pushes; i++ {
		e.use("__we_root_pop")
		e.inst("call void @__we_root_pop()")
	}
	body := e.body.String()
	restore()
	e.fnsDone = append(e.fnsDone, fmt.Sprintf(
		"define %s @%s.%s(%s) {\nentry:\n%s  ret %s\n}\n",
		abi.retTyp, fd.key, fd.name, strings.Join(ps, ", "), body, ret))
	// A custom-effect fn's default face is its gate, emitted behind the
	// real define under the real define's unchanged name (M10c D6).
	if isCustomEffect(fd.decl) {
		e.emitFxGate(fd, abi)
	}
	e.slotFor(fd.key+"."+fd.name, fnSlotTarget(fd))
	return nil
}

// bindDefineParams renders one define's IR parameter list and binds the
// parameter environments — the shared face of the fn, mock, and test
// defines (a String or sum parameter takes two IR words, name0/name1;
// the source name keeps its spelling — a parameter literally named v0 or
// s0 collides with the fresh counter's registers, a B-track cleanup owns
// that).
func (e *emitter) bindDefineParams(params []ast.Param, abi fnAbi) []string {
	var ps []string
	for i, p := range params {
		pa := abi.params[i]
		switch pa.kind {
		case abiI64:
			ps = append(ps, "i64 %"+p.Name)
			if p.Name != "_" {
				e.scalars[p.Name] = scalarSlot{operand: "%" + p.Name}
			}
		case abiDouble:
			ps = append(ps, "double %"+p.Name)
			if p.Name != "_" {
				e.scalars[p.Name] = scalarSlot{operand: "%" + p.Name, isFloat: true}
			}
		case abiStr:
			ps = append(ps, "ptr %"+p.Name+"0", "i64 %"+p.Name+"1")
			if p.Name != "_" {
				e.strEnv[p.Name] = strBinding{dataOp: "%" + p.Name + "0", lenOp: "%" + p.Name + "1"}
			}
		case abiGc:
			ps = append(ps, "ptr %"+p.Name)
			if p.Name != "_" {
				e.gcEnv[p.Name] = gcBinding{rec: pa.key, reg: "%" + p.Name}
			}
		case abiSum:
			ps = append(ps, "i64 %"+p.Name+"0", "i64 %"+p.Name+"1")
			if p.Name != "_" {
				ts := e.value()
				e.inst(fmt.Sprintf("%%%s = alloca i64", ts))
				e.inst(fmt.Sprintf("store i64 %%%s0, ptr %%%s", p.Name, ts))
				pp := e.value()
				e.inst(fmt.Sprintf("%%%s = alloca i64", pp))
				e.inst(fmt.Sprintf("store i64 %%%s1, ptr %%%s", p.Name, pp))
				e.sums2[p.Name] = sumSlot{tag: "%" + ts, pay: "%" + pp, variants: e.sumsOrd[pa.key]}
			}
		}
	}
	return ps
}

// mockTarget resolves one mock's target to the faces the install and the
// restore need: the ABI the mock fn carries (a program fn's own classi-
// fication, in its own module's tables — a restated record or sum type
// would not resolve under the test module's key), the slot the calls
// ride, and the real body's symbol the restore stores back. A std entry
// classifies by its restated signature — the entries' types are the base
// scalars and String, which classify under any module's tables.
func (e *emitter) mockTarget(md *ast.MockDecl) (fnAbi, string, string, *NotImplemented) {
	if md.TargetQual != "" {
		if k := e.resolveQual(md.TargetQual); k != "" {
			fd, ok := e.fnTable[k+"."+md.Target]
			if !ok {
				return fnAbi{}, "", "", e.bnd()
			}
			abi, ok := e.classify(fd)
			if !ok {
				return fnAbi{}, "", "", bndFn()
			}
			slot := fd.key + "." + fd.name
			// The restore returns to the slot's default: the gate for a
			// custom-effect fn, the real body otherwise (M10c D6 — the
			// post-mock default is the gate again, never the real body).
			return abi, slot, fnSlotTarget(fd), nil
		}
		if sk := e.resolveStd(md.TargetQual); sk != "" {
			ent, ok := stdFnEntries[sk][md.Target]
			if !ok {
				return fnAbi{}, "", "", e.bnd()
			}
			abi, ok := e.fnAbiOf(&ast.FnDecl{Params: md.Params, Ret: md.Ret})
			if !ok {
				return fnAbi{}, "", "", bndFn()
			}
			return abi, sk + "." + md.Target, "@" + ent.sym, nil
		}
		return fnAbi{}, "", "", e.bnd()
	}
	fd, ok := e.fnTable[e.curKey+"."+md.Target]
	if !ok {
		return fnAbi{}, "", "", e.bnd()
	}
	abi, ok := e.classify(fd)
	if !ok {
		return fnAbi{}, "", "", bndFn()
	}
	slot := fd.key + "." + fd.name
	// The unqualified face mirrors the qualified one: the restore returns
	// to the slot's default (the gate for a custom-effect fn, M10c D6).
	return abi, slot, fnSlotTarget(fd), nil
}

// emitMockDefine emits one mock fn under "<key>.mock.<n>" with the
// target's ABI (design D3: the interception is transparent to callers,
// so the mock carries the target's own calling shape) and the mock's
// parameter names, the body walking as a fn body — the one-tail-return
// discipline and the family's return through fnRetVal. No slot: the mock
// is never itself a call face, only a value a slot holds.
func (e *emitter) emitMockDefine(md *ast.MockDecl, key string, n int) (mockInstall, *NotImplemented) {
	abi, slot, restore, ni := e.mockTarget(md)
	if ni != nil {
		return mockInstall{}, ni
	}

	savedCtx, savedBody := e.ctx, e.body
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	restoreState := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
	}
	e.ctx = ctxFn
	e.body = strings.Builder{}
	e.scalars = make(map[string]scalarSlot)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.defers = nil
	e.caps = nil
	e.pushes = 0

	ps := e.bindDefineParams(md.Params, abi)
	items := md.Body.Items
	var tail *ast.Return
	if len(items) > 0 {
		if r, ok := items[len(items)-1].(*ast.Return); ok {
			tail = r
			items = items[:len(items)-1]
		}
	}
	for _, st := range items {
		if _, ok := st.(*ast.Return); ok {
			restoreState()
			return mockInstall{}, bndFn()
		}
		if ni := e.emitStmt(st); ni != nil {
			restoreState()
			return mockInstall{}, ni
		}
	}
	ret, ni := e.fnRetVal(abi, tail)
	if ni != nil {
		restoreState()
		return mockInstall{}, ni
	}
	for i := len(e.defers) - 1; i >= 0; i-- {
		if ni := e.emitBlockStmts(e.defers[i].Items); ni != nil {
			restoreState()
			return mockInstall{}, ni
		}
	}
	for i := 0; i < e.pushes; i++ {
		e.use("__we_root_pop")
		e.inst("call void @__we_root_pop()")
	}
	body := e.body.String()
	restoreState()
	sym := fmt.Sprintf("%s.mock.%d", key, n)
	e.fnsDone = append(e.fnsDone, fmt.Sprintf(
		"define %s @%s(%s) {\nentry:\n%s  ret %s\n}\n",
		abi.retTyp, sym, strings.Join(ps, ", "), body, ret))
	return mockInstall{slot: slot, mock: "@" + sym, restore: restore}, nil
}

// emitTestDefine emits one test fn under "<key>.test.<n>" — void, no
// parameters, the body walking as a fn body under the one widening the
// test context owns: a valueless return may leave the body from any
// depth (the checker's "(test)" context is valueless; each emits ret
// void and opens the never-reached continuation that keeps the block
// structure well-formed for the statements that lexically follow). The
// mock declarations ride ahead of this as their own defines.
func (e *emitter) emitTestDefine(td *ast.TestDecl, key string, n int) *NotImplemented {
	savedCtx, savedBody := e.ctx, e.body
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	savedTestBody := e.testBody
	restore := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
		e.testBody = savedTestBody
	}
	e.ctx = ctxFn
	e.testBody = true
	e.body = strings.Builder{}
	e.scalars = make(map[string]scalarSlot)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.defers = nil
	e.caps = nil
	e.pushes = 0

	for _, st := range td.Body.Items {
		if _, ok := st.(*ast.MockDecl); ok {
			continue // its define rode ahead of this walk
		}
		if r, ok := st.(*ast.Return); ok {
			if ni := e.emitTestReturn(r); ni != nil {
				restore()
				return ni
			}
			continue
		}
		if ni := e.emitStmt(st); ni != nil {
			restore()
			return ni
		}
	}
	for i := len(e.defers) - 1; i >= 0; i-- {
		if ni := e.emitBlockStmts(e.defers[i].Items); ni != nil {
			restore()
			return ni
		}
	}
	for i := 0; i < e.pushes; i++ {
		e.use("__we_root_pop")
		e.inst("call void @__we_root_pop()")
	}
	body := e.body.String()
	restore()
	e.fnsDone = append(e.fnsDone, fmt.Sprintf(
		"define void @%s.test.%d() {\nentry:\n%s  ret void\n}\n",
		key, n, body))
	return nil
}

// emitDriver renders the recorded steps (design D1/D5): each test's step
// sequence rides its own re-enterable internal thunk "<key>.drive.<n>" —
// n the driver's global ordinal, the seed and report identity — with the
// instruction stream the M10b inline driver carried (mock installs in
// source order, begin, one spawned wrapper task awaited at the task
// boundary under the M9b handle ABI, end, the restores, the report under
// a branch whose failing arm hands the panic-message C string); the
// entry carries one meta/drive pair per test — the meta naming the
// declaration's line/col, the exploration guards' anchor — then the
// summary, which owns the process's exit code. One emission form for
// both modes: the runtime's drive face decides whether the thunk
// re-enters.
func (e *emitter) emitDriver() {
	for i := range e.drives {
		st := &e.drives[i]
		savedBody := e.body
		e.body = strings.Builder{}
		for _, m := range st.mocks {
			// The install materializes the slot even when no call site
			// referenced it this build (a mocked-but-never-called target
			// still needs its global to exist).
			slot := e.slotFor(m.slot, m.restore)
			e.inst(fmt.Sprintf("store ptr %s, ptr %s", m.mock, slot))
		}
		e.use("__we_test_begin")
		e.inst("call void @__we_test_begin()")
		e.use("__we_task_new")
		h := e.value()
		e.inst(fmt.Sprintf("%%%s = call ptr @__we_task_new(ptr %s, ptr null)", h, st.wrap))
		e.use("__we_handle_await")
		pay := e.value()
		e.inst(fmt.Sprintf("%%%s = alloca i64", pay))
		tag := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 @__we_handle_await(ptr %%%s, ptr %%%s)", tag, h, pay))
		e.use("__we_test_end")
		e.inst("call void @__we_test_end()")
		for _, m := range st.mocks {
			e.inst(fmt.Sprintf("store ptr %s, ptr @slot.%s", m.restore, m.slot))
		}
		f := e.intern(st.file)
		d := e.intern(st.desc)
		n := e.blocks
		e.blocks++
		c := e.value()
		e.inst(fmt.Sprintf("%%%s = icmp eq i64 %%%s, 0", c, tag))
		e.inst(fmt.Sprintf("br i1 %%%s, label %%tk%dp, label %%tk%df", c, n, n))
		e.label(fmt.Sprintf("tk%dp", n))
		e.use("__we_test_report")
		e.inst(fmt.Sprintf("call void @__we_test_report(ptr %s, i64 %d, ptr %s, i64 %d, i64 0, ptr null)",
			f, len(st.file), d, len(st.desc)))
		e.inst(fmt.Sprintf("br label %%tk%dq", n))
		e.label(fmt.Sprintf("tk%df", n))
		r := e.value()
		e.inst(fmt.Sprintf("%%%s = load i64, ptr %%%s", r, pay))
		p := e.value()
		e.inst(fmt.Sprintf("%%%s = inttoptr i64 %%%s to ptr", p, r))
		e.use("__we_test_report")
		e.inst(fmt.Sprintf("call void @__we_test_report(ptr %s, i64 %d, ptr %s, i64 %d, i64 1, ptr %%%s)",
			f, len(st.file), d, len(st.desc), p))
		e.inst(fmt.Sprintf("br label %%tk%dq", n))
		e.label(fmt.Sprintf("tk%dq", n))
		body := e.body.String()
		e.body = savedBody
		e.thunks = append(e.thunks, fmt.Sprintf(
			"define internal void @%s.drive.%d() {\nentry:\n%s  ret void\n}\n",
			st.key, i, body))
		e.use("__we_test_meta")
		e.inst(fmt.Sprintf("call void @__we_test_meta(ptr %s, i64 %d, ptr %s, i64 %d, i64 %d, i64 %d)",
			f, len(st.file), d, len(st.desc), st.line, st.col))
		e.use("__we_test_drive")
		e.inst(fmt.Sprintf("call void @__we_test_drive(i64 %d, ptr @%s.drive.%d)", i, st.key, i))
	}
	e.use("__we_test_summary")
	e.inst("call void @__we_test_summary()")
	e.inst("ret i32 0")
}

// fnRetVal emits the family's return and yields the instruction's tail
// (everything after "ret "). A value-shaped fn must end in a return of
// the family's shape; a void fn's tail return is optional and carries no
// value. The value computes before the defers and the pops — nothing
// between the last pop and the ret may allocate.
func (e *emitter) fnRetVal(abi fnAbi, tail *ast.Return) (string, *NotImplemented) {
	switch abi.ret {
	case abiVoid:
		if tail != nil && tail.HasValue {
			return "", bndFn()
		}
		return "void", nil
	case abiI64, abiDouble:
		if tail == nil || !tail.HasValue {
			return "", bndFn()
		}
		// The tail return's value face includes a direct call of the
		// family's own ABI (the M10b goldens return fn results straight
		// through: return double(double(n))). The M9b expression
		// emitters stay call-free; the call resolves here, computing at
		// the same pre-defer position any other value face does.
		if c, ok := tail.Value.(*ast.Call); ok {
			res, ni := e.emitCall(c, nil)
			if ni != nil {
				return "", ni
			}
			if res.kind != ckI64 || res.isFloat != (abi.ret == abiDouble) {
				return "", bndFn()
			}
			if abi.ret == abiDouble {
				return "double " + res.i64, nil
			}
			return "i64 " + res.i64, nil
		}
		op, isF, ni := e.emitNumExpr(tail.Value)
		if ni != nil {
			return "", ni
		}
		if (abi.ret == abiDouble) != isF {
			return "", bndFn()
		}
		if isF {
			return "double " + op, nil
		}
		return "i64 " + op, nil
	case abiStr:
		if tail == nil || !tail.HasValue {
			return "", bndFn()
		}
		// An SSA double word cannot ride inside a ret's aggregate
		// constant — LLVM takes constant structs there, so the pair
		// builds through insertvalue (register-level, no allocation:
		// the pre-pop position stays legal).
		strPair := func(dataOp, lenOp string) string {
			v0 := e.value()
			e.inst(fmt.Sprintf("%%%s = insertvalue { ptr, i64 } undef, ptr %s, 0", v0, dataOp))
			v1 := e.value()
			e.inst(fmt.Sprintf("%%%s = insertvalue { ptr, i64 } %%%s, i64 %s, 1", v1, v0, lenOp))
			return "{ ptr, i64 } %" + v1
		}
		switch v := tail.Value.(type) {
		case *ast.Ident:
			b, ok := e.strEnv[v.Name]
			if !ok || b.dataOp == "" {
				return "", bndFn()
			}
			return strPair(b.dataOp, b.lenOp), nil
		case *ast.Literal:
			if v.Kind != "string" {
				return "", bndFn()
			}
			data, ok := decodeStringLiteral(v.Text)
			if !ok {
				return "", bndFn()
			}
			return fmt.Sprintf("{ ptr, i64 } { ptr %s, i64 %d }", e.intern(data), len(data)), nil
		case *ast.Call:
			res, ni := e.emitCall(v, nil)
			if ni != nil {
				return "", ni
			}
			if res.kind != ckStr {
				return "", bndFn()
			}
			return strPair(res.strBind.dataOp, res.strBind.lenOp), nil
		}
		return "", bndFn()
	case abiGc:
		if tail == nil || !tail.HasValue {
			return "", bndFn()
		}
		switch v := tail.Value.(type) {
		case *ast.Ident:
			g, ok := e.gcEnv[v.Name]
			if !ok {
				return "", bndFn()
			}
			return "ptr " + g.reg, nil
		case *ast.Construct:
			reg, _, ni := e.emitConstruct(v)
			if ni != nil {
				return "", ni
			}
			return "ptr " + reg, nil
		}
		return "", bndFn()
	case abiSum:
		if tail == nil || !tail.HasValue {
			return "", bndFn()
		}
		c, ok := tail.Value.(*ast.Call)
		if !ok {
			return "", bndFn()
		}
		id, ok := c.Fn.(*ast.Ident)
		if !ok {
			return "", bndFn()
		}
		// Ok(unit) on a Result return is the zero pair; every Err shape
		// and every payload-bearing return stops (the B-track widens).
		if abi.retKey == "Result" {
			if id.Name != "Ok" || len(c.Args) != 1 {
				return "", bndFn()
			}
			if _, ok := c.Args[0].(*ast.Unit); !ok {
				return "", bndFn()
			}
			return "{ i64, i64 } { i64 0, i64 0 }", nil
		}
		if len(c.Args) != 0 {
			return "", bndFn()
		}
		for i, n := range abi.variants {
			if n == id.Name {
				return fmt.Sprintf("{ i64, i64 } { i64 %d, i64 0 }", i), nil
			}
		}
		return "", bndFn()
	}
	return "", bndFn()
}

// emitFnCall emits one program-fn call through its slot (design D3): the
// slot load, the argument list per the callee's families, and the result
// per the return family — a String return extracts its two words into
// the binding's operand pair, a record return is rooted by the caller,
// a sum return lands in the two-slot environment.
func (e *emitter) emitFnCall(fd *fnDef, args []ast.Expr) (callResult, *NotImplemented) {
	abi, ok := e.classify(fd)
	if !ok {
		return callResult{}, bndFn()
	}
	if len(args) != len(fd.decl.Params) {
		return callResult{}, e.bnd()
	}
	slot := e.slotFor(fd.key+"."+fd.name, fnSlotTarget(fd))
	fp := e.value()
	e.inst(fmt.Sprintf("%%%s = load ptr, ptr %s", fp, slot))
	var ops []string
	for i, a := range args {
		// A call argument of the family's own ABI rides its result
		// directly (the M10b goldens nest fn results: double(double(n))).
		// The M9b expression emitters stay call-free — the resolution is
		// this argument position's, not those sets'.
		if c, ok := a.(*ast.Call); ok {
			res, ni := e.emitCall(c, nil)
			if ni != nil {
				return callResult{}, ni
			}
			switch abi.params[i].kind {
			case abiI64:
				if res.kind != ckI64 || res.isFloat {
					return callResult{}, e.bnd()
				}
				ops = append(ops, "i64 "+res.i64)
			case abiDouble:
				if res.kind != ckI64 || !res.isFloat {
					return callResult{}, e.bnd()
				}
				ops = append(ops, "double "+res.i64)
			case abiStr:
				if res.kind != ckStr {
					return callResult{}, e.bnd()
				}
				ops = append(ops, "ptr "+res.strBind.dataOp, "i64 "+res.strBind.lenOp)
			default:
				return callResult{}, e.bnd()
			}
			continue
		}
		switch abi.params[i].kind {
		case abiI64:
			op, isF, ni := e.emitNumExpr(a)
			if ni != nil {
				return callResult{}, ni
			}
			if isF {
				return callResult{}, e.bnd()
			}
			ops = append(ops, "i64 "+op)
		case abiDouble:
			op, isF, ni := e.emitNumExpr(a)
			if ni != nil {
				return callResult{}, ni
			}
			if !isF {
				return callResult{}, e.bnd()
			}
			ops = append(ops, "double "+op)
		case abiStr:
			p, l, ni := e.emitStringExpr(a)
			if ni != nil {
				return callResult{}, ni
			}
			ops = append(ops, "ptr "+p, "i64 "+l)
		case abiGc:
			switch v := a.(type) {
			case *ast.Ident:
				g, ok := e.gcEnv[v.Name]
				if !ok {
					return callResult{}, e.bnd()
				}
				ops = append(ops, "ptr "+g.reg)
			case *ast.Construct:
				reg, _, ni := e.emitConstruct(v)
				if ni != nil {
					return callResult{}, ni
				}
				ops = append(ops, "ptr "+reg)
			default:
				return callResult{}, e.bnd()
			}
		case abiSum:
			id, ok := a.(*ast.Ident)
			if !ok {
				return callResult{}, e.bnd()
			}
			s, ok := e.sums2[id.Name]
			if !ok {
				return callResult{}, e.bnd()
			}
			ops = append(ops, "i64 "+e.loadNum(s.tag, false), "i64 "+e.loadNum(s.pay, false))
		}
	}
	join := strings.Join(ops, ", ")
	switch abi.ret {
	case abiVoid:
		e.inst(fmt.Sprintf("call void %%%s(%s)", fp, join))
		return callResult{kind: ckVoid}, nil
	case abiI64:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 %%%s(%s)", v, fp, join))
		return callResult{kind: ckI64, i64: "%" + v}, nil
	case abiDouble:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call double %%%s(%s)", v, fp, join))
		return callResult{kind: ckI64, i64: "%" + v, isFloat: true}, nil
	case abiStr:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call { ptr, i64 } %%%s(%s)", v, fp, join))
		p := e.value()
		e.inst(fmt.Sprintf("%%%s = extractvalue { ptr, i64 } %%%s, 0", p, v))
		l := e.value()
		e.inst(fmt.Sprintf("%%%s = extractvalue { ptr, i64 } %%%s, 1", l, v))
		return callResult{kind: ckStr, strBind: strBinding{dataOp: "%" + p, lenOp: "%" + l}}, nil
	case abiGc:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call ptr %%%s(%s)", v, fp, join))
		reg := "%" + v
		e.use("__we_root_push")
		e.pushes++
		e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", reg))
		return callResult{kind: ckGc, gcReg: reg, recKey: abi.retKey}, nil
	case abiSum:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call { i64, i64 } %%%s(%s)", v, fp, join))
		t := e.value()
		e.inst(fmt.Sprintf("%%%s = extractvalue { i64, i64 } %%%s, 0", t, v))
		p := e.value()
		e.inst(fmt.Sprintf("%%%s = extractvalue { i64, i64 } %%%s, 1", p, v))
		ts := e.value()
		e.inst(fmt.Sprintf("%%%s = alloca i64", ts))
		e.inst(fmt.Sprintf("store i64 %%%s, ptr %%%s", t, ts))
		pp := e.value()
		e.inst(fmt.Sprintf("%%%s = alloca i64", pp))
		e.inst(fmt.Sprintf("store i64 %%%s, ptr %%%s", p, pp))
		return callResult{kind: ckSum, sum: sumSlot{tag: "%" + ts, pay: "%" + pp, variants: abi.variants}}, nil
	}
	return callResult{}, e.bnd()
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
// lines and map descriptors in declaration order over the used records —
// module-qualified names; the constant pool, the report line, the slot
// globals in first-use order), the declares in fixed order, the fn
// defines and the thunk defines, and the entry. In test mode the entry is
// the synthesized driver's (design D5); in build mode the root main is
// the entry.
func (e *emitter) render(module string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "; ModuleID = '%s'\n\n", module)

	var groups [][]string
	var structs, maps []string
	for _, r := range e.order {
		if !e.usedRecs[r.name] {
			continue
		}
		slots, _, ok := e.layout(recModKey(r.name), r.decl)
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
			structs = append(structs, fmt.Sprintf("%%struct.%s = type {}", r.name))
		} else {
			structs = append(structs, fmt.Sprintf("%%struct.%s = type { %s }", r.name, strings.Join(parts, ", ")))
		}
		maps = append(maps, fmt.Sprintf("@.map.%s = private unnamed_addr constant %s", r.name, mapLiteral(bitmap, len(slots))))
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
	if len(e.slots) > 0 {
		groups = append(groups, e.slots)
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
	for _, f := range e.fnsDone {
		sb.WriteString(f + "\n")
	}
	for _, th := range e.thunks {
		sb.WriteString(th + "\n")
	}
	// The synthesized test driver owns the entry in test mode (design
	// D5) — its steps rendered into the body ahead of this; in build mode
	// the root main is the entry.
	sb.WriteString("define i32 @__we_main() {\nentry:\n")
	sb.WriteString(e.body.String())
	sb.WriteString("}\n")
	return sb.String()
}

// isStringPayloadVariant is the variant-attribution back-check of design
// D3: name must be a variant of the E in the entry main's `Result<(), E>`
// return annotation, carrying exactly one String payload. key is the
// entry module's — the error type resolves in its own module's table.
// Typecheck already established the semantics; this only confirms the
// attribution from the module's own declarations, without leaning on
// typecheck internals.
func isStringPayloadVariant(sums map[string]map[string][]ast.TypeRef, key string, ret ast.TypeRef, name string) bool {
	res, ok := ret.(*ast.NamedType)
	if !ok || res.Qual != "" || res.Name != "Result" || len(res.Args) != 2 {
		return false
	}
	errTy, ok := res.Args[1].(*ast.NamedType)
	if !ok || errTy.Qual != "" {
		return false
	}
	payload, ok := sums[key+"."+errTy.Name][name]
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
