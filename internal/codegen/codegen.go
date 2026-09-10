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
	"maps"
	"math"
	"sort"
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
	bndMainBody   = "main bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"
	bndErrPayload = "Err payloads beyond one plain string-literal variant argument"
	bndTopLets    = "top-level value bindings in code generation"
	bndTaskBody   = "task bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"
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
	// T2 (design D1): the piercing-exit face — a return, break, or
	// continue through an open scope discharges its remaining handles
	// (chapter 18:231). Referenced only by programs that pierce, so every
	// prior module's bytes ride unchanged.
	{"__we_scope_cancel", "declare void @__we_scope_cancel(ptr)"},
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
	// T4 (design D3): the String family, appended under the same
	// discipline — a program that builds no string declares none. The
	// value-to-string converters return the two-word struct the C side
	// returns in registers, so the pair comes out of extractvalue (the
	// type line rides render() with the record structs).
	{"__we_str_concat", "declare %struct.we_str @__we_str_concat(ptr, i64, ptr, i64)"},
	{"__we_str_eq", "declare i64 @__we_str_eq(ptr, i64, ptr, i64)"},
	{"__we_str_runecount", "declare i64 @__we_str_runecount(ptr, i64)"},
	{"__we_str_charat", "declare i64 @__we_str_charat(ptr, i64, i64)"},
	{"__we_str_byteslice", "declare %struct.we_str @__we_str_byteslice(ptr, i64, i64, i64)"},
	{"__we_str_of_i64", "declare %struct.we_str @__we_str_of_i64(i64)"},
	{"__we_str_of_u64", "declare %struct.we_str @__we_str_of_u64(i64)"},
	{"__we_str_of_f64", "declare %struct.we_str @__we_str_of_f64(double)"},
	{"__we_str_of_bool", "declare %struct.we_str @__we_str_of_bool(i64)"},
	{"__we_str_of_rune", "declare %struct.we_str @__we_str_of_rune(i64)"},
	// T7 (design D6): the List carrier, appended under the same discipline
	// — a program that builds no list declares none. The constructor takes
	// the capacity and the element trace, push answers the list's identity
	// (growth moves the block), and get's word is the element's own face:
	// a scalar's value or a gc handle.
	{"__we_list_new", "declare ptr @__we_list_new(i64, i64)"},
	{"__we_list_push", "declare ptr @__we_list_push(ptr, i64)"},
	{"__we_list_get", "declare i64 @__we_list_get(ptr, i64)"},
	{"__we_list_len", "declare i64 @__we_list_len(ptr)"},
	{"__we_list_snap", "declare ptr @__we_list_snap(ptr)"},
	// T8-2B (design D7): the module-level root table. The registration
	// takes a slot's address, not a handle; a program with no gc top-level
	// binding registers nothing and declares nothing.
	{"__we_gc_root_global", "declare void @__we_gc_root_global(ptr)"},
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
	ret bool   // yields i64
	typ string // the entry's declared base type name, where it yields one
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
		"now":   {sym: "__we_time_now", ret: true, typ: "Int64"},
		"sleep": {sym: "__we_time_sleep"},
	},
}

// A record field's emission shape: String is the double word (16 bytes,
// not a gc reference — every M8 buffer is a constant, design D5), a
// record reference is one pointer slot, and the 8-byte base types ride
// one word each. A reference splits by the target's category, since the
// two copy differently: a gc- or resource-category target is shared (the
// copy keeps the pointer), a value-category target is copied with it
// (chapter 8's value records share nothing).
type fieldKind int

const (
	fkStr    fieldKind = iota
	fkRef              // a gc- or resource-category record reference
	fkVal              // a value-category record reference
	fkScalar           // one i64 word
	fkF64              // one double word
)

type fieldSlot struct {
	off   int
	kind  fieldKind
	typ   string // the referenced record's key, or the scalar's base type name
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

// listBinding is one List value's handle (design D6): the carrier pointer
// plus the element face its type fixed. Nothing in the value itself says
// what the carrier holds — its descriptor covers "words that are
// references", not the element's own domain — so the face rides the
// environment, exactly as a tuple's element shapes ride tupBinding.
type listBinding struct {
	reg  string // the carrier pointer: a creation's register or a literal's
	elem listElem
}

// listElem is one List's element face: the single word an element occupies
// in the carrier. A scalar face is its interpolation domain (the word IS
// the value; a Float64 rides as its bit pattern); a gc face is the
// module-qualified record key the handle points at, whose element slots
// the carrier's descriptor traces. Everything else — a String's two words,
// a sum's two, a tuple's several, a fn value, a nested collection — has no
// one-word face and stops at the body boundary.
type listElem struct {
	kind strKind
	rec  string
	gc   bool
}

// fnValue is one bound function value (design D5): the single pointer a
// binding holds — a gc carrier whose first word is the thunk's code
// pointer and whose second is its capture environment (null when the
// closure captures nothing). A call loads the pair out of the carrier;
// the callback ABI takes the pair as its two pointer words.
//
// The signature rides beside the carrier rather than in it: the calling
// shape is a static fact of the site that made the value, so a binding
// carries it on in the environment maps and a capture hands it to the
// thunk that reads the value back — exactly as a record capture hands on
// the record's key. A fn value whose signature the emitter did not see
// answers typed=false and stops at the fn body word rather than guessing
// an ABI.
type fnValue struct {
	carrier string
	abi     fnAbi
	typed   bool
}

// tupBinding is one tuple value's handle: the stack aggregate that IS the
// tuple, plus the element shapes it was laid out with. A tuple never
// leaves the stack — it crosses a call boundary as its bare words (design
// D4) and is rebuilt into an aggregate on the far side.
type tupBinding struct {
	ptr   string
	elems []tupleElem
}

// tupleElem is one element's place in a tuple's aggregate: the family it
// crosses a boundary as, the IR words it occupies there in order, and its
// byte offset (design D4's `{f0, f1, ...}` layout, element per field).
type tupleElem struct {
	kind  fnAbiKind
	key   string // the record key a gc element points at, "" otherwise
	typ   string // the element's base-type name, for the interpolation domain
	off   int
	words []string // "i64", "double", or "ptr" — one per word, in order
}

// aggTyp renders a tuple aggregate's IR type from its element shapes: the
// words the elements occupy, element by element.
func aggTyp(elems []tupleElem) string {
	var parts []string
	for _, el := range elems {
		parts = append(parts, el.words...)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
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
	// listEnv holds a List binding's handle: the carrier pointer plus the
	// element face its type fixed (design D6). Like the tuple's shapes it
	// is a static fact the value does not carry, so the typecheck-time
	// knowledge has to ride the environment to the iteration site.
	listEnv map[string]listBinding
	// tupEnv holds a tuple binding's handle: the stack aggregate that IS
	// the tuple value, plus the shape its elements were classified with.
	tupEnv map[string]tupBinding
	// ntEnv records the names whose static type is a newtype. Nothing in
	// the value says so — the wrapper erases (design D4) — so the fact
	// rides the environment, and the `.value` read is what spends it.
	ntEnv map[string]string

	strs     []strConst
	strPool  map[string]string
	usedRecs map[string]bool
	declUsed map[string]bool
	errConst string

	// strStruct records that some body calls the String family, whose
	// value-to-string members return design D3's two-word pair as a
	// struct: render() then emits the named type ahead of the bodies.
	strStruct bool

	body   strings.Builder
	fresh  int
	pushes int

	// M9b state.
	ctx     bodyCtx
	scalars map[string]scalarSlot
	sums2   map[string]sumSlot
	prims   map[string]string
	fnEnv   map[string]fnValue // local fn values (their gc carriers)
	thunks  []string           // finished callback and task defines
	ovfs    []string           // overflow report constants
	divzs   []string           // zero-divisor report constants
	blocks  int                // fresh block-label suffix
	// curBlock is the label of the block the running body is emitting
	// into: the entry block until a label() opens another. A phi that
	// joins a subexpression's branches names its predecessors from here
	// (emitLogic): the block reaching the join is the one the operand's
	// emission ended in, which its own control flow (a division guard, a
	// nested value form) may have left several blocks past the one the
	// operand opened. Like every other body-scoped field it is saved and
	// restored around the nested bodies emission swaps in.
	curBlock string

	concAlias map[string]bool // the std.concurrent import's alias set
	defers    []ast.Block     // a body's defer blocks, emission inverted
	panics    []string        // NUL-terminated panic-message constants
	caps      *captureSet     // the task body being emitted reads these
	chanDescs []string        // channel element-bitmap descriptor globals
	envMaps   []string        // task environment-block bitmap globals
	cdsc      int             // fresh channel-element descriptor count
	fnMap     string          // the fn-value carrier's trace descriptor
	cbN       int             // fresh closure-thunk count

	// T1 env scoping: per-block snapshots of the five env maps (see
	// pushEnv/popEnv). Block-local registrations land on the block's
	// clone and die with it — same-named outer bindings survive the
	// join (follow-up #16).
	frames []envFrame

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

	// T8-1/T8-2 module-level bindings (design D7). A binding owns its own
	// globals — `@<key>.<name>` for a scalar word or a collectable handle,
	// `@<key>.<name>.{p,len}` for a String's pair (T8-2A) — spelled by its
	// qualified symbol, which is also the read face's key. topLets holds
	// every module's bindings in the order the inits run them: pass one
	// walks the modules in load order, so the slice is chapter 15's
	// post-order with source order inside each module. topRoots names the
	// bindings that hold a collectable handle (T8-2B): their globals are
	// registered with the collector at the entry head.
	topLets     []topLetRef
	topSlots    map[string]topSlot
	topGlobals  []string
	topRoots    []string
	initEmitted map[string]bool

	// T5 method table (design D4). A method is keyed by its head type's
	// module-qualified key plus the method name — the receiver's record
	// names the head, so a call site resolves by what it already knows.
	// The interface's own face contributes no entry: an `impl Iface for
	// Head` method lands under Head, and a default body the interface
	// declares instantiates once per implementing head (a bare signature
	// never does). methodsOrd keeps the define order deterministic.
	methods    map[string]*fnDef
	methodsOrd []string
	// newtypes maps a newtype's key to its underlying type (design D4):
	// a value of the newtype IS a value of the underlying, at every
	// position, so the table answers `classType` and the `.value` read
	// alike. Generic newtypes are B1b's.
	newtypes   map[string]ast.TypeRef
	ifaceDefs  map[string]*ast.InterfaceDecl // "<module>.<interface>" -> decl
	ifaceHeads map[string][]string           // interface key -> implementing head keys, impl order
	ifaceSeen  map[string]map[string]bool    // interface key -> head keys already bound

	// M12 foreign state (design D4/D5). Opaque records live in their own
	// table — no layout, no constructor, no records/order entry (E1707
	// shut the We-side construction paths at check); declare lines ride
	// their own group, emitted for every entry whether called or not (the
	// same every-fn-emitted discipline the We fns ride). diverged marks a
	// body whose branch already ended in unreachable — a Never-returning
	// foreign call — so no statement, defer, pop, or ret follows it.
	opaques      map[string]bool
	foreignDecls []string
	diverged     bool

	// T2 statement-set state (design D1). assigned is the set of names
	// the body being emitted writes (collectAssigned, a pre-pass): a
	// name in it binds to a slot, one outside it keeps its SSA operand.
	// allocas buffers the stack
	// slots the body being emitted reserves — one entry per slot
	// request, flushed behind the entry label when the body closes (see
	// slot and bodyText). loopFrames is the break/continue target stack
	// for the body being walked — one frame per open loop, recording
	// the live-scope depth at its start so a piercing break or continue
	// discharges exactly the scopes it crosses (chapter 18:231).
	// scopeLive is the matching stack of compound scopes entered and
	// not yet left: a piercing exit emits their cancel or collect leave
	// at its own site, inner to outer. exit names the enclosing body's
	// return protocol — what a return at any depth answers with (nil
	// outside a body: the callback's single-expression face). inExit
	// guards re-entry while one exit sequence is being emitted (a
	// return inside a defer block is outside the set).
	assigned   map[string]bool
	allocas    []string
	loopFrames []loopFrame
	scopeLive  []liveScope
	// resFrames is the stack of open scope resource statements (chapter
	// 13) — one frame per block entered and not yet left, each holding
	// its bindings in declaration order. nest is the body's nesting
	// ordinal: every compound scope and every resource block takes the
	// next one as it opens, so a loop frame's or an exit's mark compares
	// against both stacks by one number — which is what lets a piercing
	// exit discharge the two kinds in the order they actually nest.
	resFrames []resFrame
	nest      int
	exit      *exitSite
	inExit    bool

	// M10b test state (design D5). tests collects the TestDecls pass one
	// sees in test mode (module order, source order within); the tower
	// emitter turns each into a test fn, its mocks, and a wrapper, and
	// records the driver's step. A test body's valueless return rides
	// exitSite{kind: exitTest} like every other body's exit.
	tests  []testRef
	drives []driveStep
}

// collectImpl enters one impl block's methods into the method table
// (design D4). The head names the table's key, so `impl Greeter for User`
// and `impl User` land in the same place — which is what lets a default
// body's `self.greet()` resolve. A generic impl is B1b's (its methods
// instantiate per type argument); a non-nominal head has no key to hang a
// method on. An interface impl also records the head under its interface,
// the pairing collectDefaults instantiates from.
func (e *emitter) collectImpl(modKey string, d *ast.ImplDecl) *NotImplemented {
	if len(d.TypeParams) != 0 {
		return &NotImplemented{What: bndGenericFns}
	}
	head, ok := d.Head.(*ast.NamedType)
	if !ok || head.Qual != "" || len(head.Args) != 0 {
		return e.bnd()
	}
	headKey := modKey + "." + head.Name
	for _, md := range d.Methods {
		if len(md.TypeParams) != 0 {
			return &NotImplemented{What: bndGenericFns}
		}
		key := headKey + "." + md.Name
		if _, seen := e.methods[key]; seen {
			continue // the first definition wins; a duplicate is the check stage's (E0807)
		}
		e.methods[key] = &fnDef{key: modKey, name: md.Name, recvKey: headKey, decl: md}
		e.methodsOrd = append(e.methodsOrd, key)
	}
	if iface, ok := d.Iface.(*ast.NamedType); ok && iface.Qual == "" && len(iface.Args) == 0 {
		ik := modKey + "." + iface.Name
		if e.ifaceSeen[ik] == nil {
			e.ifaceSeen[ik] = make(map[string]bool)
		}
		if !e.ifaceSeen[ik][headKey] {
			e.ifaceSeen[ik][headKey] = true
			e.ifaceHeads[ik] = append(e.ifaceHeads[ik], headKey)
		}
	}
	return nil
}

// collectDefaults instantiates every interface default body once per
// implementing head (design D4) — the step that makes a default method
// real. A head that defines the method itself keeps its own definition: a
// default is what an impl leaves unsaid. Runs after pass one, when every
// module's interfaces and impls are known, whatever their source order.
func (e *emitter) collectDefaults() {
	for ik, iface := range e.ifaceDefs {
		for _, headKey := range e.ifaceHeads[ik] {
			for _, m := range iface.Methods {
				if m.Body == nil {
					continue // a bare signature is the impl's obligation, not a body
				}
				key := headKey + "." + m.Name
				if _, ok := e.methods[key]; ok {
					continue
				}
				e.methods[key] = &fnDef{
					key:     ik[:strings.LastIndex(ik, ".")],
					name:    m.Name,
					recvKey: headKey,
					decl:    defaultFnDecl(m),
				}
				e.methodsOrd = append(e.methodsOrd, key)
			}
		}
	}
}

// defaultFnDecl restates one interface default body as the fn declaration
// its per-head instantiation emits — the same signature, the body the
// interface wrote.
func defaultFnDecl(m ast.MethodSig) *ast.FnDecl {
	d := &ast.FnDecl{
		Name:       m.Name,
		Recv:       m.Recv,
		Params:     m.Params,
		EffectTags: m.EffectTags,
		Body:       *m.Body,
	}
	if m.HasRet {
		d.Ret = m.Ret
	}
	return d
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
// callee's own module's tables. A foreign entry (chapter 19) joins the
// table but never the define list — its body is native, its contract a
// declare line, and its calls take the foreign ABI, not this one.
type fnDef struct {
	key     string
	name    string
	decl    *ast.FnDecl
	abi     fnAbi
	abiOK   bool
	foreign bool
	// recvKey is the head type's module-qualified key on a method (design
	// D4's method table); empty on a plain fn. It names the symbol's
	// middle segment — and, inside the body, the record `self` holds.
	recvKey string
}

// sym renders one fn's IR symbol: `<module>.<name>` for a plain fn,
// `<module>.<Head>.<name>` for a method — the head type's segment keeps a
// method from colliding with a same-named plain fn of its module.
func (fd *fnDef) sym() string {
	if fd.recvKey != "" {
		return fd.recvKey + "." + fd.name
	}
	return fd.key + "." + fd.name
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

// capSlot is one binding a closure captures, and its place in the
// environment block (design D5): the words start at off, n of them, and
// trace says per word whether the block's bitmap marks it a gc reference.
// A scalar is one untraced word — the copy capture of a value binding,
// frozen at the creation site. A primitive handle or a record pointer is
// one traced word — the reference capture of a gc binding, which stays
// one object with its enclosing scope. A String is its header's pair: the
// data pointer (traced) plus the length (not a pointer).
type capSlot struct {
	name  string
	off   int
	n     int
	trace [2]bool
	prim  bool    // the traced word is a primitive handle, not a fn carrier
	key   string  // a gc record capture names the record it points at
	kind  strKind // a scalar capture keeps its interpolation domain
	abi   fnAbi   // a fn capture keeps the signature its carrier was built with
	typed bool    // the capture above carries a signature at all
}

// closureEnv is one closure's capture set in layout order — the slots its
// environment block holds, and the length the allocation takes.
type closureEnv struct {
	slots []capSlot
}

// words is the block's payload word count.
func (c *closureEnv) words() int {
	n := 0
	for _, s := range c.slots {
		n += s.n
	}
	return n
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
	ctxFn
)

func (e *emitter) bnd() *NotImplemented {
	switch e.ctx {
	case ctxTask:
		return bndTask()
	case ctxFn:
		return bndFn()
	default:
		return bndMain()
	}
}

// scalarSlot is one numeric binding. A name the body assigns owns a
// memory slot and is read and written through it; one it only reads
// holds its operand directly (an SSA value or an immediate). Which one a
// name is follows from the body it binds in (collectAssigned), not from
// its keyword: a var always owns a slot, a let owns one only when
// something writes it. Floats ride the same slots with their own
// arithmetic.
type scalarSlot struct {
	operand string // the read-only face's value ("%v3" or "5")
	alloca  string // the addressable face's memory slot ("%v7")
	isFloat bool
	// kind classifies the slot in design D3's interpolation domain where
	// the binding site knew it statically — the annotation, the
	// initializer's literal kind, a callee's declared return type.
	// skNone means the slot's provenance is not statically decidable:
	// rendering an interpolation hole of it stops at the boundary.
	kind strKind
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

func (e *emitter) inst(s string) { e.body.WriteString("  " + s + "\n") }

// label opens a fresh LLVM block, which by construction is not terminated:
// whatever divergence (unreachable, a branch-arm return) closed the
// previous block is over. The divergence protocol therefore clears the
// flag here — a block open is the only thing that may follow a terminator,
// and every join site guards its unconditional branch with the flag so no
// instruction ever lands after one.
func (e *emitter) label(s string) {
	e.diverged = false
	e.curBlock = s
	e.body.WriteString(s + ":\n")
}

// beginBody starts a fresh body buffer, whose first block is the entry
// label the define format writes (see bodyText) — the one block no
// label() call announces, and so the one curBlock must be told about.
func (e *emitter) beginBody() {
	e.body = strings.Builder{}
	e.curBlock = "entry"
}
func (e *emitter) value() string  { v := "v" + strconv.Itoa(e.fresh); e.fresh++; return v }
func (e *emitter) use(sym string) { e.declUsed[sym] = true }

// slot reserves one stack slot of typ for the body being emitted and
// returns its pointer operand. The request takes its value number here —
// the body's numbering is the emission order and does not move — while
// the alloca's text buffers until the body closes (bodyText), where it
// splices in behind the entry label.
//
// The hoist is not cosmetic: LLVM allocates a non-entry alloca afresh on
// every pass through its block, and the stack it takes returns only when
// the function does. A binding inside a loop would therefore grow the
// stack by one slot per iteration — 30M iterations of an eight-byte slot
// is a segmentation fault, not a slow program (measured: the T2-a probe
// died at that count with the alloca visible in the loop body). A slot
// reserved once at the entry is one frame slot reused, whatever the loop
// count. Every slot the emitter hands out is scalar or a two-word sum
// pair with no pointer identity outside its own loads and stores, so the
// sharing across iterations is unobservable (a captured name reads its
// value; nothing takes the address).
func (e *emitter) slot(typ string) string {
	v := e.value()
	e.allocas = append(e.allocas, "  %"+v+" = alloca "+typ)
	return "%" + v
}

// bodyText closes the body under emission: the slots reserved during it
// splice in directly behind the entry label every define format writes.
// The buffer belongs to the body, so taking the text clears it (each
// body's caller restores whatever buffer was live around it).
func (e *emitter) bodyText() string {
	if len(e.allocas) == 0 {
		return e.body.String()
	}
	al := strings.Join(e.allocas, "\n") + "\n"
	e.allocas = nil
	return al + e.body.String()
}

// ForeignName is one foreign fn entry the link stage must resolve: the
// module that declares it, the entry's source name (the Q3 ruling: the
// declared name is the C symbol itself, unmangled), and the position the
// E1906 report names it from.
type ForeignName struct {
	Module    string
	Name      string
	Line, Col int
}

// ForeignNames collects the program's foreign fn entries, module order
// then source order within — the link tower's face (design D5): every
// declared name must exist among the native objects llvm-nm reports, and
// the ones that do not are E1906's list. Opaque records declare no
// symbol of their own.
func ForeignNames(mods []ProgModule) []ForeignName {
	var names []ForeignName
	for _, m := range mods {
		for _, it := range m.File.Items {
			fb, ok := it.(*ast.ForeignBlock)
			if !ok {
				continue
			}
			for _, x := range fb.Items {
				if f, ok := x.(*ast.FnDecl); ok && f.Foreign {
					names = append(names, ForeignName{Module: m.Key, Name: f.Name, Line: f.NameLine, Col: f.NameCol})
				}
			}
		}
	}
	return names
}

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
		sums:        make(map[string]map[string][]ast.TypeRef),
		sumsOrd:     make(map[string][]string),
		records:     make(map[string]*ast.RecordDecl),
		strEnv:      make(map[string]strBinding),
		gcEnv:       make(map[string]gcBinding),
		listEnv:     make(map[string]listBinding),
		tupEnv:      make(map[string]tupBinding),
		ntEnv:       make(map[string]string),
		newtypes:    make(map[string]ast.TypeRef),
		strPool:     make(map[string]string),
		usedRecs:    make(map[string]bool),
		declUsed:    make(map[string]bool),
		ctx:         ctxMain,
		exit:        &exitSite{kind: exitMain},
		assigned:    make(map[string]bool),
		scalars:     make(map[string]scalarSlot),
		sums2:       make(map[string]sumSlot),
		prims:       make(map[string]string),
		fnEnv:       make(map[string]fnValue),
		modKeys:     make(map[string]bool),
		modImports:  make(map[string]map[string]string),
		modStd:      make(map[string]map[string]string),
		modConc:     make(map[string]map[string]bool),
		fnTable:     make(map[string]*fnDef),
		slotSeen:    make(map[string]bool),
		topSlots:    make(map[string]topSlot),
		initEmitted: make(map[string]bool),
		opaques:     make(map[string]bool),
		methods:     make(map[string]*fnDef),
		ifaceDefs:   make(map[string]*ast.InterfaceDecl),
		ifaceHeads:  make(map[string][]string),
		ifaceSeen:   make(map[string]map[string]bool),
		mode:        mode,
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
				if ni := e.collectTopLet(m.Key, d); ni != nil {
					return "", ni
				}
			case *ast.RecordDecl:
				key := m.Key + "." + d.Name
				if _, seen := e.records[key]; !seen {
					e.order = append(e.order, recRef{name: key, decl: d})
				}
				e.records[key] = d
			case *ast.NewtypeDecl:
				// Newtypes are zero-cost wrappers (chapter 8): the layout
				// erases (design D4), so the table carries what a value of
				// the wrapper erases to. The generic form is B1b's, like
				// every other generic declaration's.
				if len(d.TypeParams) != 0 {
					return "", &NotImplemented{What: bndGenericFns}
				}
				e.newtypes[m.Key+"."+d.Name] = d.Underlying
			case *ast.InterfaceDecl:
				// The interface itself emits no IR — its member set is a
				// declaration-level fact the check stage consumes. Its
				// default bodies do reach IR, but never under the
				// interface's name: each instantiates per implementing
				// head (see collectDefaults), because a default body's
				// `self.m()` dispatches to the head's own method.
				e.ifaceDefs[m.Key+"."+d.Name] = d
			case *ast.ImplDecl:
				// An impl's methods reach IR under the head type's key —
				// the method table of design D4. The impl's own face (the
				// interface it satisfies, its where clause) stays a
				// declaration-level fact.
				if ni := e.collectImpl(m.Key, d); ni != nil {
					return "", ni
				}
			case *ast.EffectDecl:
				// Effect declarations erase (M7 design D10): chapter 16 is
				// a compile-time discipline — the check stage consumes
				// every segment, and the tag names reach no IR and no
				// runtime face.
				continue
			case *ast.ForeignBlock:
				// Chapter 19 (M12 design D4): the block's records are
				// opaque handles and its fns are native contracts. The
				// records collect first — an entry may reference an opaque
				// the block declares later — then every fn joins the fn
				// table under its module key and renders its declare line,
				// in block order, called or not. Fn entries never join the
				// define list: the body is native. Opaque records join
				// neither the record table nor its order — no layout, no
				// constructor, nothing to emit.
				for _, x := range d.Items {
					if r, ok := x.(*ast.RecordDecl); ok {
						e.opaques[m.Key+"."+r.Name] = true
					}
				}
				for _, x := range d.Items {
					f, ok := x.(*ast.FnDecl)
					if !ok || !f.Foreign {
						continue
					}
					e.fnTable[m.Key+"."+f.Name] = &fnDef{key: m.Key, name: f.Name, decl: f, foreign: true}
					line, ok := e.foreignDeclare(f)
					if !ok {
						return "", bndFn() // check already owns the crossing set; this is unreachable defense
					}
					e.foreignDecls = append(e.foreignDecls, line)
				}
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
	e.collectDefaults()
	// The module inits open the entry body, ahead of its own statements
	// (chapter 15: every initializer runs before main does). The calls
	// allocate no value numbers, so the entry's numbering is untouched.
	// The gc slots register ahead of even those: the collector has to know
	// a slot before any initializer can put a handle in it (T8-2B).
	e.emitTopRootRegistrations()
	e.emitInitCalls()
	if mode == ModeBuild {
		if entry == nil {
			// Defensive: Project-mode typecheck rejects a missing main (E1305).
			return "", bndMain()
		}
		e.enterModule(root.Key)
		e.mainRet = entry.Ret
		collectAssigned(entry.Body.Items, e.assigned)
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
	// The module inits (T8-1) ride the fn group ahead of __we_main: they
	// emit after the entry so the entry's value numbering is the entry's
	// own, and their bodies call through the fn table like any other.
	if ni := e.emitTopLetInits(); ni != nil {
		return "", ni
	}
	// The method table's defines (design D4), in collection order: an
	// impl's own methods first, then the defaults an interface contributes
	// to the heads that left them unsaid.
	for _, key := range e.methodsOrd {
		if ni := e.emitFnDefine(e.methods[key]); ni != nil {
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
	if e.diverged {
		// A Never-returning foreign call already terminated this branch:
		// unreachable is the block's terminator, so nothing after it
		// emits — the statements lexically following are dead by the
		// declaration's own semantics.
		return nil
	}
	switch s := st.(type) {
	case *ast.Binding:
		if s.Pat != nil {
			return e.emitLetPattern(s)
		}
		if s.Kw != "let" && s.Kw != "var" {
			return e.bnd()
		}
		if s.Kw == "var" {
			return e.emitVarBinding(s)
		}
		return e.emitLetBinding(s)
	case *ast.Assign:
		if s.Field != "" {
			// `self.field = value` (design D1's field write, design D4's
			// mut self): a store through the receiver pointer at the
			// field's layout offset — the write half of the unified
			// member face. The name is `self` by the grammar (chapter 10
			// admits no other target), so the binding settles it.
			g, ok := e.gcEnv[s.Name]
			if !ok {
				return e.bnd()
			}
			slot, ok := e.fieldSlotOf(g.rec, s.Field)
			if !ok {
				return e.bnd()
			}
			return e.emitFieldStore(g.reg, slot, s.Value)
		}
		slot, ok := e.scalars[s.Name]
		if !ok || slot.alloca == "" {
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
	case *ast.Loop:
		return e.emitLoop(s)
	case *ast.ForStmt:
		return e.emitFor(s)
	case *ast.Break:
		return e.emitBreak()
	case *ast.Continue:
		return e.emitContinue()
	case *ast.Return:
		return e.emitReturn(s)
	case *ast.ScopeRes:
		return e.emitScopeRes(s)
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
			if len(init.Holes) > 0 {
				// An interpolated literal computes its value at the
				// binding (the holes' expressions run here); the pair it
				// leaves is what the name holds.
				return e.bindStringValue(s.Name, init)
			}
			data, ok := decodeStringLiteral(init.Text)
			if !ok {
				return e.bnd()
			}
			if s.Name != "_" {
				e.strEnv[s.Name] = strBinding{data: data, length: len(data)}
			}
			return nil
		case "int", "bool", "float", "rune":
			op, isF, ok := e.numImmediate(init)
			if !ok {
				return e.bnd()
			}
			// The annotation names the bound type where it carries one;
			// otherwise the literal's own kind does (design D3's domain:
			// `let n: UInt64 = 5` renders unsigned).
			kind := baseStrKind(baseTypeName(s.Typ))
			if kind == skNone {
				kind = literalStrKind(init)
			}
			if s.Name != "_" {
				if e.assigned[s.Name] {
					e.bindScalarSlot(s.Name, op, isF, kind)
					return nil
				}
				e.scalars[s.Name] = scalarSlot{operand: op, isFloat: isF, kind: kind}
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
		// A List is a gc-category type (chapter 17): binding it shares the
		// carrier, exactly as the aliased reference it is.
		if b, ok := e.listEnv[init.Name]; ok {
			e.listEnv[s.Name] = b
			return nil
		}
		if k, is := e.ntEnv[init.Name]; is {
			e.ntEnv[s.Name] = k
		}
		if b, ok := e.tupEnv[init.Name]; ok {
			e.tupEnv[s.Name] = b
			return nil
		}
		if b, ok := e.strEnv[init.Name]; ok {
			e.strEnv[s.Name] = b
			return nil
		}
		if v, ok := e.scalars[init.Name]; ok {
			if e.assigned[s.Name] {
				// The alias is written, so it needs its own storage: copy
				// the value in rather than share the source's slot.
				op := v.operand
				if v.alloca != "" {
					op = e.loadNum(v.alloca, v.isFloat)
				}
				e.bindScalarSlot(s.Name, op, v.isFloat, v.kind)
				return nil
			}
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
		if v, ok := e.fnEnv[init.Name]; ok {
			e.fnEnv[s.Name] = v
			return nil
		}
		// A program fn named in value position: the constant pair, whose
		// slot-loaded code pointer already carries the signature.
		if !e.isLocalName(init.Name) {
			if fd, ok := e.fnTable[e.curKey+"."+init.Name]; ok {
				res, ni := e.emitFnRef(fd)
				if ni != nil {
					return ni
				}
				return e.bindResult(s.Name, res)
			}
		}
		if v, ok := e.gcEnv[init.Name]; ok {
			// Binding a value record copies the whole value (chapter 8:
			// every binding of a value record takes its own object); a gc
			// or resource record binds the same reference.
			if r, ok := e.records[v.rec]; ok && r.Cat == "value" {
				e.gcEnv[s.Name] = gcBinding{rec: v.rec, reg: e.emitRecCopy(v.reg, v.rec)}
				return nil
			}
			e.gcEnv[s.Name] = v
			return nil
		}
		if ts, ok := e.topName(init.Name); ok {
			return e.bindTopRead(s.Name, ts)
		}
		return e.bnd()
	case *ast.Closure:
		// A closure literal in value position (design D5): the thunk is
		// emitted where the creation site's bindings live, its captures
		// frozen into the environment block, and the name holds the one
		// carrier both parts travel in.
		res, ni := e.emitClosureValue(init, s.Typ)
		if ni != nil {
			return ni
		}
		return e.bindResult(s.Name, res)
	case *ast.Tuple:
		// A tuple binding holds the aggregate's handle (design D4): the
		// value is its stack slots, and its members are the elements.
		ptr, elems, ni := e.emitTupleAgg(init)
		if ni != nil {
			return ni
		}
		if s.Name != "_" {
			e.tupEnv[s.Name] = tupBinding{ptr: ptr, elems: elems}
		}
		return nil
	case *ast.Construct:
		reg, rkey, ni := e.emitConstruct(init)
		if ni != nil {
			return ni
		}
		if s.Name != "_" {
			e.gcEnv[s.Name] = gcBinding{rec: rkey, reg: reg}
		}
		return nil
	case *ast.ListLit:
		// A list literal builds its carrier here (design D6); the name
		// holds the pointer and the element face its type fixed, which is
		// what a later iteration or element read needs to know.
		reg, face, ni := e.emitListLit(init, s.Typ)
		if ni != nil {
			return ni
		}
		if s.Name != "_" {
			e.listEnv[s.Name] = listBinding{reg: reg, elem: face}
		}
		return nil
	case *ast.Member:
		// A field read binds the field's own face: a String pair, a
		// scalar word, or a nested record's reference. A value record
		// read out of another record is copied — the binding takes its
		// own object (chapter 8), not a share of the owner's.
		res, ni := e.emitMemberValue(init)
		if ni != nil {
			return ni
		}
		if res.kind == ckGc {
			if r, ok := e.records[res.recKey]; ok && r.Cat == "value" {
				res.gcReg = e.emitRecCopy(res.gcReg, res.recKey)
			}
		}
		return e.bindResult(s.Name, res)
	case *ast.Binary, *ast.Unary, *ast.If, *ast.Match, *ast.BlockExpr:
		// The operator family and the value-position control forms bind
		// through one path: the operand carries its domain into the slot
		// (or the SSA face) exactly as a literal's does.
		return e.bindNumericValue(s.Name, s.Init)
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

// bindNumericValue binds one numeric value expression (the operator
// family, the unary family, the value-position control forms) under name:
// a name the body assigns takes a slot, and both faces carry the value's
// domain — a float binding stays a float through its SSA operand as much
// as through its slot (design D10-1: a dropped domain left every later
// consumer spelling i64 over a double register).
func (e *emitter) bindNumericValue(name string, x ast.Expr) *NotImplemented {
	if e.valueKind(x) == skStr {
		// A String value expression — the concatenation, a String-returning
		// call, an interpolated literal — binds its pair, not a number.
		return e.bindStringValue(name, x)
	}
	res, ni := e.emitNumericValue(x)
	if ni != nil {
		return ni
	}
	switch res.kind {
	case ckVoid:
		// A valueless form binds only the discard (an if whose arms run
		// for their effect); a name has nothing to hold.
		if name != "_" {
			return e.bnd()
		}
		return nil
	case ckI64:
	default:
		return e.bnd()
	}
	if name == "_" {
		return nil
	}
	// The classification is the value's own (the same one an
	// interpolation hole reads), so a name bound from an expression
	// carries its domain onward: `let n = x + 1` renders as the integer
	// `x` was.
	kind := e.valueKind(x)
	if e.assigned[name] {
		e.bindScalarSlot(name, res.i64, res.isFloat, kind)
		return nil
	}
	e.scalars[name] = scalarSlot{operand: res.i64, isFloat: res.isFloat, kind: kind}
	return nil
}

// bindStringValue binds one String-valued expression under name (the
// concatenation, an interpolated literal, a call whose declared return is
// a String). The emission happens here — the holes' expressions and the
// call run at the binding — and the name holds the resulting operand pair.
// The discard still emits: a hole may call, and calls have effects.
func (e *emitter) bindStringValue(name string, x ast.Expr) *NotImplemented {
	res, ni := e.emitHole(x)
	if ni != nil {
		return ni
	}
	if name != "_" {
		e.strEnv[name] = res.strBind
	}
	return nil
}

// emitNumericValue emits one numeric value expression: a value-position
// control form joins through its result slot (and may carry no value at
// all), everything else is the numeric set's own operand.
func (e *emitter) emitNumericValue(x ast.Expr) (callResult, *NotImplemented) {
	switch x.(type) {
	case *ast.If, *ast.Match, *ast.BlockExpr:
		return e.emitValueForm(x)
	}
	op, isF, ni := e.emitNumExpr(x)
	if ni != nil {
		return callResult{}, ni
	}
	return callResult{kind: ckI64, i64: op, isFloat: isF}, nil
}

// bindResult stores one call-shaped value under the binding's name (the
// discard `_` keeps no environment face).
func (e *emitter) bindResult(name string, res callResult) *NotImplemented {
	if name != "_" && res.ntype != "" {
		// The binding's static type is a newtype: nothing in the value
		// says so (design D4's erasure), so the fact rides the name and
		// the `.value` read is what spends it.
		e.ntEnv[name] = res.ntype
	}
	switch res.kind {
	case ckVoid:
		return nil
	case ckTuple:
		if name != "_" {
			e.tupEnv[name] = res.tup
		}
		return nil
	case ckFn:
		if name != "_" {
			e.fnEnv[name] = res.fn
		}
		return nil
	case ckIo:
		if name != "_" {
			return e.bnd()
		}
		return nil
	case ckI64:
		if name != "_" {
			// The callee's declaration fixed the type where it named one
			// (a program fn's return, a String member's result): the
			// binding carries it for the interpolation domain.
			kind := baseStrKind(res.typeName)
			if e.assigned[name] {
				e.bindScalarSlot(name, res.i64, res.isFloat, kind)
				return nil
			}
			e.scalars[name] = scalarSlot{operand: res.i64, isFloat: res.isFloat, kind: kind}
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
	case *ast.BlockExpr:
		// A bare block statement: one more block scope in this body, its
		// statements walking in order. The block's value, if the tail
		// carries one, is discarded here — the check face owns that
		// (E0605), so codegen never sees a non-unit tail.
		return e.emitBlockStmts(v.Block.Items)
	case *ast.If:
		return e.emitIf(v, nil)
	case *ast.Match:
		return e.emitMatch(v, nil)
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
	ckIo    callKind = iota // an io call, emitted by the M8 path
	ckVoid                  // emitted, no value
	ckI64                   // a scalar value in the i64 domain (or a double)
	ckPrim                  // a primitive pointer (gc-rooted at construction)
	ckSum                   // an Option/Result two-slot value
	ckStr                   // a String double word (M10b fn returns)
	ckGc                    // a record pointer (M10b fn returns and ctors)
	ckTuple                 // a tuple value (its stack aggregate)
	ckFn                    // a function value (its gc carrier, D5)
)

type callResult struct {
	kind    callKind
	i64     string // the ckI64/ckPrim operand
	isFloat bool
	sum     sumSlot    // the ckSum slots
	strBind strBinding // the ckStr operand pair
	gcReg   string     // the ckGc pointer operand
	recKey  string     // the ckGc record's module-qualified key
	// typeName is the result's base-type name where the callee's
	// declaration fixed it (design D3's interpolation domain); empty when
	// the call's face does not carry one.
	typeName string
	// ntype is the newtype this result is a value of, where the site that
	// produced it knows (a construction). The value is the underlying's —
	// that is the erasure — so the tag exists only so a binding can
	// remember that `.value` unwraps it.
	ntype string
	// tup is the aggregate handle a tuple-valued result lands in; kind is
	// ckTuple when it is set.
	tup tupBinding
	// fn is the carrier a function-valued result holds; kind is ckFn when
	// it is set.
	fn fnValue
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
	if _, ok := e.tupEnv[name]; ok {
		return true
	}
	_, ok := e.caps.slot(name)
	return ok
}

// numImmediate renders a numeric literal as an IR operand: ints, bools,
// and runes as i64 immediates, floats as the exact double hex bits — the
// bare hex form, the consumer spells the type (every use site knows the
// domain from the isFloat flag; a prefixed operand here would double the
// type spelling there. Found while pinning the M10b float call face — no
// prior golden reached a float path).
func (e *emitter) numImmediate(l *ast.Literal) (string, bool, bool) {
	if l.Kind == "rune" {
		// A rune is its code point in the i64 domain — Rune has no
		// storage form of its own (design D2).
		r, ok := decodeRuneLiteral(l.Text)
		if !ok {
			return "", false, false
		}
		return strconv.FormatInt(r, 10), false, true
	}
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
			if s.alloca != "" {
				return e.loadNum(s.alloca, s.isFloat), s.isFloat, nil
			}
			return s.operand, s.isFloat, nil
		}
		if _, ok := e.caps.slot(v.Name); ok {
			// A task body reads an enclosing scalar through its env slot.
			op, isF, ni := e.loadCapture(v.Name)
			return op, isF, ni
		}
		if ts, ok := e.topName(v.Name); ok {
			// A module-level binding (T8-1): one load of its global. A
			// String binding's global is a pair, not a word — it is no
			// numeric operand, and the boundary is the honest answer.
			if ts.str {
				return "", false, e.bnd()
			}
			res := e.topRead(ts)
			return res.i64, res.isFloat, nil
		}
		return "", false, e.bnd()
	case *ast.Unary:
		return e.emitUnary(v)
	case *ast.Binary:
		switch v.Op {
		case "+", "-", "*":
			return e.emitOverflowArith(v)
		case "/", "%":
			return e.emitDivMod(v)
		case "<", "<=", ">", ">=", "==", "!=":
			return e.emitCompare(v)
		case "&&", "||":
			return e.emitLogic(v)
		case "&", "|", "^", "<<", ">>":
			return e.emitBitwise(v)
		}
		return "", false, e.bnd()
	case *ast.Call:
		// The operand-position call (design D2): a user fn's i64-domain
		// result feeds whatever operator surrounds the call, at any
		// nesting depth.
		res, ni := e.emitCall(v, nil)
		if ni != nil {
			return "", false, ni
		}
		if res.kind != ckI64 {
			return "", false, e.bnd()
		}
		return res.i64, res.isFloat, nil
	case *ast.If, *ast.Match, *ast.BlockExpr:
		// The value-position control forms (design D2): their arms join
		// through a result slot, and the loaded value is an operand like
		// any other.
		res, ni := e.emitValueForm(v)
		if ni != nil {
			return "", false, ni
		}
		if res.kind != ckI64 {
			return "", false, e.bnd()
		}
		return res.i64, res.isFloat, nil
	case *ast.Member:
		// A scalar or double field read (design D4's unified member read):
		// one getelementptr at the field's offset plus the load.
		res, ni := e.emitMemberValue(v)
		if ni != nil {
			return "", false, ni
		}
		if res.kind != ckI64 {
			return "", false, e.bnd()
		}
		return res.i64, res.isFloat, nil
	default:
		return "", false, e.bnd()
	}
}

// spelledInt reports the value of an integer literal the source spells —
// a divisor the emitter can check before it emits anything.
func spelledInt(x ast.Expr) (int64, bool) {
	lit, ok := x.(*ast.Literal)
	if !ok || lit.Kind != "int" {
		return 0, false
	}
	imm, ok := scalarImmediate(lit)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(imm, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// emitUnary emits the prefix family (chapter 2's unary level). `!` is the
// boolean domain's zero test, widened back into the i64 domain the
// bindings carry; `~` is the all-ones xor, total by construction; `-` is
// the checked subtraction from zero, because negating the minimum value
// overflows — chapter 7's integer arithmetic is checked, and floats ride
// IEEE (fneg, no trap).
func (e *emitter) emitUnary(u *ast.Unary) (string, bool, *NotImplemented) {
	op, isF, ni := e.emitNumExpr(u.X)
	if ni != nil {
		return "", false, ni
	}
	switch u.Op {
	case "!":
		if isF {
			return "", false, e.bnd()
		}
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = icmp eq i64 %s, 0", v, op))
		z := e.value()
		e.inst(fmt.Sprintf("%%%s = zext i1 %%%s to i64", z, v))
		return "%" + z, false, nil
	case "-":
		if isF {
			v := e.value()
			e.inst(fmt.Sprintf("%%%s = fneg double %s", v, op))
			return "%" + v, true, nil
		}
		return e.emitCheckedIntr("llvm.ssub.with.overflow.i64", "Int64 neg overflow", "0", op), false, nil
	case "~":
		if isF {
			return "", false, e.bnd()
		}
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = xor i64 %s, -1", v, op))
		return "%" + v, false, nil
	}
	return "", false, e.bnd()
}

// emitDivMod emits `/` and `%` (design D2). LLVM leaves both the zero
// divisor and the minimum-value-by-minus-one pair undefined, while
// chapter 7's arithmetic is checked to the last value, so the emitter
// carries the guards itself: a zero divisor runs the task-panic tail
// rather than reaching the division, and `/` traps the one pair whose
// quotient overflows. `%` needs no overflow guard — that pair's remainder
// is exactly zero, and `a srem 1` is `a srem -1`'s value without the
// undefined pair, so a select on the divisor closes it in one
// instruction. A divisor the source spells was checked at compile time
// (chapter 7's constant fold, E0502) and takes the bare division: the
// benchmark tasks' `n % 2` body is one srem. Floats follow chapter 7's
// IEEE rule — fdiv and frem, no trap face.
func (e *emitter) emitDivMod(b *ast.Binary) (string, bool, *NotImplemented) {
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
	flop, iop := "fdiv", "sdiv"
	if b.Op == "%" {
		flop, iop = "frem", "srem"
	}
	if af {
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = %s double %s, %s", v, flop, a, c))
		return "%" + v, true, nil
	}
	if n, ok := spelledInt(b.R); ok && n != 0 && n != -1 {
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = %s i64 %s, %s", v, iop, a, c))
		return "%" + v, false, nil
	}
	bl := e.blocks
	e.blocks++
	divz := fmt.Sprintf("divz%d", bl)
	divk := fmt.Sprintf("divk%d", bl)
	z := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp eq i64 %s, 0", z, c))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", z, divz, divk))
	e.label(divz)
	zn := fmt.Sprintf("@.dvz%d", len(e.divzs))
	e.divzs = append(e.divzs, msgConst(zn, "division by zero"))
	e.use("__we_task_fail")
	e.inst(fmt.Sprintf("call void @__we_task_fail(ptr %s)", zn))
	e.inst("unreachable")
	e.label(divk)
	if b.Op == "%" {
		m1 := e.value()
		e.inst(fmt.Sprintf("%%%s = icmp eq i64 %s, -1", m1, c))
		safe := e.value()
		e.inst(fmt.Sprintf("%%%s = select i1 %%%s, i64 1, i64 %s", safe, m1, c))
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = %s i64 %s, %%%s", v, iop, a, safe))
		return "%" + v, false, nil
	}
	// The minimum value by -1: LLVM's other undefined division pair, and
	// an overflow chapter 7's checked arithmetic reports like any other.
	mn := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp eq i64 %s, -9223372036854775808", mn, a))
	m1 := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp eq i64 %s, -1", m1, c))
	both := e.value()
	e.inst(fmt.Sprintf("%%%s = and i1 %%%s, %%%s", both, mn, m1))
	ob := e.blocks
	e.blocks++
	dovf := fmt.Sprintf("dovf%d", ob)
	dofk := fmt.Sprintf("dofk%d", ob)
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", both, dovf, dofk))
	e.label(dovf)
	e.trap("Int64 div overflow")
	e.label(dofk)
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = %s i64 %s, %s", v, iop, a, c))
	return "%" + v, false, nil
}

// emitBitwise emits the binary bit family — chapter 2's shift level and
// the `& | ^` level. The three logicals are total over the integer
// domain, but the shifts carry chapter 7's checked posture on both
// edges: LLVM's shift is poison for an amount at or past the width, and
// a left shift that drops a bit off the top is the silent wrap the
// chapter forbids. `>>` is the arithmetic shift — the sign-preserving
// reading of a signed operand — and loses only the low bits, which is
// what a right shift means; the round-trip check is a left shift's
// alone. Floats have no domain here and stop at the boundary.
func (e *emitter) emitBitwise(b *ast.Binary) (string, bool, *NotImplemented) {
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
	if b.Op != "<<" && b.Op != ">>" {
		op := map[string]string{"&": "and", "|": "or", "^": "xor"}[b.Op]
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = %s i64 %s, %s", v, op, a, c))
		return "%" + v, false, nil
	}
	op := "shl"
	if b.Op == ">>" {
		op = "ashr"
	}
	bl := e.blocks
	e.blocks += 2
	shk := fmt.Sprintf("shk%d", bl)
	sho := fmt.Sprintf("sho%d", bl)
	in := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp ult i64 %s, 64", in, c))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", in, shk, sho))
	e.label(sho)
	e.trap("Int64 shift overflow")
	e.label(shk)
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = %s i64 %s, %s", v, op, a, c))
	if b.Op == ">>" {
		return "%" + v, false, nil
	}
	bk := e.value()
	e.inst(fmt.Sprintf("%%%s = ashr i64 %%%s, %s", bk, v, c))
	same := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp eq i64 %%%s, %s", same, bk, a))
	svk := fmt.Sprintf("svk%d", bl+1)
	svf := fmt.Sprintf("svf%d", bl+1)
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", same, svk, svf))
	e.label(svf)
	e.trap("Int64 shift overflow")
	e.label(svk)
	return "%" + v, false, nil
}

// trap emits chapter 14's termination tail: the runtime prints msg and
// the process aborts. The caller reaches this block only on the failing
// edge, so the block ends unreachable.
func (e *emitter) trap(msg string) {
	e.use("__we_task_fail")
	cn := fmt.Sprintf("@.ovf%d", len(e.ovfs))
	e.ovfs = append(e.ovfs, msgConst(cn, msg))
	e.inst(fmt.Sprintf("call void @__we_task_fail(ptr %s)", cn))
	e.inst("unreachable")
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
	return e.emitCheckedIntr(intr, "Int64 "+opName[b.Op]+" overflow", a, c), false, nil
}

// emitCheckedIntr emits `a <intr> c` in chapter 7's checked form: the
// intrinsic reports the overflow bit, a failing branch runs the
// task-panic tail, and only the checked value flows on into the
// continuation the branch opens. `-x` and the division guards ride the
// same shape. `msg` is the report text: chapter 14's trap names the
// operation (`Int64 add overflow` and siblings), so each checked
// operation carries its own message rather than one shared text.
func (e *emitter) emitCheckedIntr(intr, msg, a, c string) string {
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
	e.trap(msg)
	e.label(fmt.Sprintf("cont%d", bl))
	return "%" + res
}

// msgConst formats a trap-report string constant: the NUL-terminated
// byte array the runtime prints after `Panicked: `.
func msgConst(name, msg string) string {
	return fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\\00\"", name, len(msg)+1, irEscape(msg))
}

// opName spells each checked operator as chapter 14's trap names it:
// `Int64 add overflow` and siblings, one word per operator.
var opName = map[string]string{"+": "add", "-": "sub", "*": "mul", "/": "div"}

var icmpPred = map[string]string{"<": "slt", "<=": "sle", ">": "sgt", ">=": "sge", "==": "eq", "!=": "ne"}
var fcmpPred = map[string]string{"<": "olt", "<=": "ole", ">": "ogt", ">=": "oge", "==": "oeq", "!=": "one"}

// emitCompare widens the boolean result into the i64 domain (design D8's
// Bool-as-i64 register rule).
func (e *emitter) emitCompare(b *ast.Binary) (string, bool, *NotImplemented) {
	if b.Op == "==" || b.Op == "!=" {
		if k := e.strCompare(b); k != "" {
			return k, false, nil
		}
	}
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

// strCompare emits `==`/`!=` over two String operands (chapter 10: the
// equality operators compare base types, and String is one) — by bytes and
// length, the runtime's own comparison. "" reports that the operands are
// not a String pair, which leaves the numeric face to emitCompare.
func (e *emitter) strCompare(b *ast.Binary) string {
	if e.valueKind(b.L) != skStr || e.valueKind(b.R) != skStr {
		return ""
	}
	ap, al, ni := e.emitStringExpr(b.L)
	if ni != nil {
		return ""
	}
	bp, bl, ni := e.emitStringExpr(b.R)
	if ni != nil {
		return ""
	}
	e.use("__we_str_eq")
	eq := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_str_eq(ptr %s, i64 %s, ptr %s, i64 %s)", eq, ap, al, bp, bl))
	// The runtime answers 1/0 in the i64 domain; `==` is that answer being
	// non-zero and `!=` its being zero, and the result widens like every
	// other boolean (design D8's Bool-as-i64 register rule).
	cmp := e.value()
	pred := "ne"
	if b.Op == "!=" {
		pred = "eq"
	}
	e.inst(fmt.Sprintf("%%%s = icmp %s i64 %%%s, 0", cmp, pred, eq))
	z := e.value()
	e.inst(fmt.Sprintf("%%%s = zext i1 %%%s to i64", z, cmp))
	return "%" + z
}

// emitLogic emits && / || in the short-circuit form (design D2): the left
// operand decides, and only the path the language names evaluates the
// right one — an operand that divides, calls, or panics runs exactly when
// the source says it does (the two-operand and/or form was safe only
// while no operand could have an effect, which the operand-position call
// ends). Both paths meet at a phi carrying the i64-domain boolean.
func (e *emitter) emitLogic(b *ast.Binary) (string, bool, *NotImplemented) {
	a, af, ni := e.emitNumExpr(b.L)
	if ni != nil || af {
		return "", false, ni
	}
	bl := e.blocks
	e.blocks++
	prefix := fmt.Sprintf("lgc%d", bl)
	rhsL, shortL, joinL := prefix+"rhs", prefix+"short", prefix+"join"
	// The short path's answer: the operand that decided it.
	short := "0"
	if b.Op == "||" {
		short = "1"
	}
	thenL, elseL := rhsL, shortL
	if b.Op == "||" {
		thenL, elseL = shortL, rhsL
	}
	av := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp ne i64 %s, 0", av, a))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", av, thenL, elseL))
	e.label(rhsL)
	c, cf, ni := e.emitNumExpr(b.R)
	if ni != nil || cf {
		return "", false, ni
	}
	cv := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp ne i64 %s, 0", cv, c))
	rz := e.value()
	e.inst(fmt.Sprintf("%%%s = zext i1 %%%s to i64", rz, cv))
	// The phi names the block each path actually reaches the join from —
	// not the label each path opened. An operand with control flow of its
	// own (a division guard, a nested value form, a call with a trap)
	// leaves its emission in a block several labels past rhsL, and LLVM
	// requires every phi entry to name a real predecessor.
	rhsPred, shortPred := e.curBlock, ""
	rhsFalls := !e.diverged
	if rhsFalls {
		e.inst(fmt.Sprintf("br label %%%s", joinL))
	}
	e.label(shortL)
	shortPred = e.curBlock
	e.inst(fmt.Sprintf("br label %%%s", joinL))
	e.label(joinL)
	v := e.value()
	if rhsFalls {
		e.inst(fmt.Sprintf("%%%s = phi i64 [ %%%s, %%%s ], [ %s, %%%s ]", v, rz, rhsPred, short, shortPred))
	} else {
		// The right operand's block already ended in its own terminator:
		// the join is reached from the deciding path alone.
		e.inst(fmt.Sprintf("%%%s = phi i64 [ %s, %%%s ]", v, short, shortPred))
	}
	return "%" + v, false, nil
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
	slot := e.slot(typ)
	if isF {
		e.inst(fmt.Sprintf("store double %s, ptr %s", op, slot))
	} else {
		e.inst(fmt.Sprintf("store i64 %s, ptr %s", op, slot))
	}
	if s.Name != "_" {
		e.scalars[s.Name] = scalarSlot{alloca: slot, isFloat: isF, kind: baseStrKind(t.Name)}
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
		if _, ok := e.scalars[v.Name]; ok {
			return true
		}
		// A module-level scalar binding (T8-1) is an i64 operand like any
		// other, so println routes it to the numeric renderer — its String
		// sibling is a byte pair and routes the other way (T8-2), and a gc
		// handle is neither.
		ts, ok := e.topName(v.Name)
		return ok && !ts.str && !ts.gc
	case *ast.Binary:
		// A `+` over two String operands is the concatenation (chapter
		// 10's closed operator set), not a numeric form: the
		// classification decides, so the shortcut leaves it to the String
		// face rather than handing a byte pair to the i64 renderer.
		return e.valueKind(v) != skStr
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
		// A newtype construction (chapter 8's `Name(expr)`): the wrapper
		// costs nothing and shows nowhere — design D4's erasure means the
		// result simply IS the inner expression's value, at the underlying
		// family's own face.
		if key, ok := e.newtypeOf(&ast.NamedType{Name: id.Name}); ok {
			if len(call.Args) != 1 {
				return callResult{}, e.bnd()
			}
			return e.emitNewtypeCtor(key, call.Args[0])
		}
		// The bare record constructor: the positional face (the check
		// stage accepts both); a parsed program constructs through the
		// named-field form. Both land in the same protocol.
		if _, ok := e.records[e.curKey+"."+id.Name]; ok {
			return e.emitCtorCall(id.Name, call.Args)
		}
		// A local fn value is called through its carrier (design D5) —
		// before the program-fn row, since a local binding shadows a fn
		// name of the same spelling exactly as it shadows anything else.
		if fv, ok := e.fnEnv[id.Name]; ok {
			return e.emitFnValueCall(fv, call.Args)
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
	if res, is, ni := e.emitAcute(fn, call); is {
		return res, ni
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
	if recvKey, ok := e.recvKeyOf(fn.Recv); ok {
		// The method table (design D4): the receiver's record names the
		// head, the member name the method, and the pair resolves to one
		// define. The receiver's key is read from the layouts without
		// emitting, so a member that turns out to be no method leaves the
		// boundary to the faces below rather than a half-emitted operand.
		if fd, is := e.methods[recvKey+"."+fn.Name]; is {
			return e.emitMethodCall(fd, fn.Recv, call.Args)
		}
	}
	if e.valueKind(fn.Recv) == skStr {
		// A chapter 17 String member over a String receiver (T4). The
		// classification gates it, so a receiver outside the domain falls
		// through to the qualifier faces below.
		return e.emitStrMember(fn.Recv, fn.Name, call.Args)
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
		// The restated entry table carries the base type (both ret entries
		// are Int64: the clock, chapter 20).
		return callResult{kind: ckI64, i64: "%" + v, typeName: ent.typ}, nil
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
	tagSlot := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %s", tag, tagSlot))
	paySlot := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 0, ptr %s", paySlot))
	return callResult{kind: ckSum, sum: sumSlot{tag: tagSlot, pay: paySlot, variants: variants}}, nil
}

// emitSumCall is the receive/await/tryReceive shape: the out slot takes
// the payload word, the return value is the discriminant, and the two
// alloca slots become the binding's {tag, payload} pair. variants maps
// the return value to the variant names the match arms carry.
func (e *emitter) emitSumCall(sym, ptr string, variants []string) (callResult, *NotImplemented) {
	e.use(sym)
	out := e.slot("i64")
	tag := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @%s(ptr %s, ptr %s)", tag, sym, ptr, out))
	tagSlot := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %s", tag, tagSlot))
	paySlot := e.slot("i64")
	pay := e.value()
	e.inst(fmt.Sprintf("%%%s = load i64, ptr %s", pay, out))
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %s", pay, paySlot))
	return callResult{kind: ckSum, sum: sumSlot{tag: tagSlot, pay: paySlot, variants: variants}}, nil
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
		// The predicate is a closure literal or a name already bound to
		// one; either way what the ABI wants is the pair — the thunk and
		// its environment block — which is exactly what a fn value holds
		// (design D5: the callback face and the fn-value face are one).
		var parts closureParts
		switch a := args[0].(type) {
		case *ast.Closure:
			// The predicate's own signature (chapter 18): the cell's
			// element in and out for update, a Bool for wait, and read's
			// U the call's own text determines. The check stage owns
			// whether the closure may be there at all; the shape below is
			// the callback ABI's, so a predicate whose return is not the
			// value the runtime hands back stops at the fn body word.
			var p, r ast.TypeRef = &ast.NamedType{Name: "Int64"}, &ast.NamedType{Name: "Int64"}
			if method == "wait" {
				r = &ast.NamedType{Name: "Bool"}
			}
			ex, ok := e.closureForType(&ast.FnType{Params: []ast.TypeRef{p}, Ret: r})
			if !ok {
				return callResult{}, bndFn()
			}
			ps, abi, ni := e.emitClosure(a, &ex)
			if ni != nil {
				return callResult{}, ni
			}
			if abi.ret != abiI64 || len(abi.params) != 1 || abi.params[0].kind != abiI64 {
				return callResult{}, bndFn()
			}
			parts = ps
		case *ast.Ident:
			fv, ok := e.fnEnv[a.Name]
			if !ok || !fv.typed {
				return callResult{}, e.bnd()
			}
			parts = e.fnParts(fv)
		default:
			return callResult{}, e.bnd()
		}
		// The callback's environment is the closure's own block — null for
		// one that captures nothing, which is the shape this face has
		// always had, so a capture-free predicate's module bytes are
		// unchanged.
		sym := map[string]string{"update": "__we_prim_update", "read": "__we_prim_read", "wait": "__we_cond_wait"}[method]
		e.use(sym)
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 @%s(ptr %s, ptr %s, ptr %s)", v, sym, ptr, parts.fnptr, parts.env))
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

// --- T6 closures and fn values (design D5) ------------------------------------

// walkBody visits every expression a body holds in evaluation order —
// including the bodies of the closures it nests, whose free names the
// enclosing capture collection must see. It reports false when the body
// holds a node shape it does not know: a capture set that might be
// missing a name may not be used at all, so an unknown shape stops the
// closure at the body boundary rather than compiling a wrong one.
func walkBody(b ast.Block, visit func(ast.Expr)) bool {
	ok := true
	var we func(ast.Expr)
	var ws func(ast.Stmt)
	var wb func(ast.Block)
	var wp func(ast.Pattern)

	we = func(x ast.Expr) {
		switch v := x.(type) {
		case nil, *ast.Ident, *ast.Literal, *ast.Unit:
		case *ast.Unary:
			we(v.X)
		case *ast.Binary:
			we(v.L)
			we(v.R)
		case *ast.Call:
			we(v.Fn)
			for _, a := range v.Args {
				we(a)
			}
		case *ast.Member:
			we(v.Recv)
		case *ast.BlockExpr:
			wb(v.Block)
		case *ast.If:
			we(v.Cond)
			wb(v.Then)
			we(v.Else)
		case *ast.Match:
			we(v.Scrutinee)
			for _, a := range v.Arms {
				wp(a.Pat)
				we(a.Guard)
				we(a.Body)
			}
		case *ast.TaskExpr:
			wb(v.Body)
		case *ast.ScopeExpr:
			we(v.Timeout)
			wb(v.Body)
		case *ast.SelectExpr:
			for _, c := range v.Cases {
				we(c.Source)
				we(c.Body)
			}
		case *ast.Construct:
			for _, f := range v.Fields {
				we(f.Value)
			}
			we(v.Base)
		case *ast.Tuple:
			for _, el := range v.Elems {
				we(el)
			}
		case *ast.ListLit:
			for _, el := range v.Elems {
				we(el)
			}
		case *ast.Closure:
			wb(v.Body)
		case *ast.Prop:
			we(v.X)
		default:
			ok = false
		}
		visit(x)
	}
	wp = func(p ast.Pattern) {
		switch v := p.(type) {
		case nil, *ast.PatLiteral, *ast.PatWildcard, *ast.PatBinding:
		case *ast.PatOr:
			for _, br := range v.Branches {
				wp(br)
			}
		case *ast.PatTuple:
			for _, el := range v.Elems {
				wp(el)
			}
		case *ast.PatVariant:
			for _, a := range v.Args {
				wp(a)
			}
		default:
			ok = false
		}
	}
	ws = func(st ast.Stmt) {
		switch v := st.(type) {
		case *ast.Binding:
			we(v.Init)
		case *ast.Assign:
			we(v.Value)
		case *ast.ExprStmt:
			we(v.Expr)
		case *ast.Return:
			we(v.Value)
		case *ast.While:
			we(v.Cond)
			wb(v.Body)
		case *ast.Loop:
			wb(v.Body)
		case *ast.Defer:
			wb(v.Block)
		case *ast.ForStmt:
			wp(v.Pat)
			we(v.Iter)
			wb(v.Body)
		case *ast.ScopeRes:
			for _, b := range v.Binds {
				we(b.Val)
			}
			wb(v.Body)
		case *ast.Break, *ast.Continue, *ast.MockDecl:
		default:
			ok = false
		}
	}
	wb = func(blk ast.Block) {
		for _, st := range blk.Items {
			ws(st)
		}
	}
	wb(b)
	return ok
}

// collectBoundNames gathers every name a body binds — the `let`/`var`
// names (pattern bindings included), the for-in heads, the scope-resource
// heads, and a nested closure's parameters with its own locals — so a
// capture walk can tell a closure's own local from a free name.
func collectBoundNames(items []ast.Stmt, into map[string]bool) {
	var wp func(ast.Pattern)
	var we func(ast.Expr)
	wp = func(p ast.Pattern) {
		switch v := p.(type) {
		case *ast.PatBinding:
			into[v.Name] = true
		case *ast.PatOr:
			for _, br := range v.Branches {
				wp(br)
			}
		case *ast.PatTuple:
			for _, el := range v.Elems {
				wp(el)
			}
		case *ast.PatVariant:
			for _, a := range v.Args {
				wp(a)
			}
		}
	}
	we = func(x ast.Expr) {
		switch v := x.(type) {
		case *ast.Closure:
			for _, p := range v.Params {
				into[p.Name] = true
			}
			collectBoundNames(v.Body.Items, into)
		case *ast.BlockExpr:
			collectBoundNames(v.Block.Items, into)
		case *ast.If:
			collectBoundNames(v.Then.Items, into)
			we(v.Else)
		case *ast.Match:
			for _, a := range v.Arms {
				wp(a.Pat)
				we(a.Body)
			}
		case *ast.TaskExpr:
			collectBoundNames(v.Body.Items, into)
		case *ast.ScopeExpr:
			collectBoundNames(v.Body.Items, into)
		case *ast.SelectExpr:
			for _, c := range v.Cases {
				if c.Name != "" {
					into[c.Name] = true
				}
				we(c.Body)
			}
		}
	}
	for _, st := range items {
		switch v := st.(type) {
		case *ast.Binding:
			if v.Name != "" {
				into[v.Name] = true
			}
			wp(v.Pat)
			we(v.Init)
		case *ast.ExprStmt:
			we(v.Expr)
		case *ast.Assign:
			we(v.Value)
		case *ast.Return:
			we(v.Value)
		case *ast.While:
			collectBoundNames(v.Body.Items, into)
		case *ast.Loop:
			collectBoundNames(v.Body.Items, into)
		case *ast.Defer:
			collectBoundNames(v.Block.Items, into)
		case *ast.ForStmt:
			wp(v.Pat)
			collectBoundNames(v.Body.Items, into)
		case *ast.ScopeRes:
			for _, b := range v.Binds {
				into[b.Name] = true
			}
			collectBoundNames(v.Body.Items, into)
		}
	}
}

// captureOf classifies one free name at a closure's creation site: the
// value/gc split design D5 fixes. A scalar copies its word, frozen at the
// creation site; a primitive handle, a record pointer, and a fn value copy
// their pointer (the object stays one object, reachable through the env
// block's trace bitmap); a String copies its header's pair. A name bound
// nowhere the creation site can see reports false — the closure body's
// read of it stops at the body boundary on its own.
func (e *emitter) captureOf(name string) (capSlot, bool) {
	if sl, ok := e.scalars[name]; ok && sl.alloca == "" {
		return capSlot{name: name, n: 1, kind: sl.kind}, true
	}
	if _, ok := e.prims[name]; ok {
		return capSlot{name: name, n: 1, trace: [2]bool{true}, prim: true}, true
	}
	if _, ok := e.strEnv[name]; ok {
		// A String's bytes are not this collector's: the runtime carves
		// them with malloc (runtime/c/str.c's str_alloc) and never frees
		// them, and the literal form points straight into the module's
		// read-only pool. The trace bitmap says which words are gc
		// references, so neither word is traced — a mark bit written
		// through a static pointer is a write to rodata and one written
		// through a heap buffer is a write into the string's own bytes.
		return capSlot{name: name, n: 2}, true
	}
	if g, ok := e.gcEnv[name]; ok {
		return capSlot{name: name, n: 1, trace: [2]bool{true}, key: g.rec}, true
	}
	if v, ok := e.fnEnv[name]; ok {
		return capSlot{name: name, n: 1, trace: [2]bool{true}, abi: v.abi, typed: v.typed}, true
	}
	if i, ok := e.caps.slot(name); ok {
		// Created inside a task body: the name is the enclosing scope's
		// capture, and the task's own two classes are what it can be.
		return capSlot{name: name, n: 1, trace: [2]bool{true}, prim: e.caps.prim[i]}, true
	}
	return capSlot{}, false
}

// collectClosureCaptures walks one closure body for the bindings it reads
// from the creation site's scope, in layout order — each slot's offset
// assigned here so the block's words follow one another.
func (e *emitter) collectClosureCaptures(cl *ast.Closure) ([]capSlot, bool) {
	locals := map[string]bool{"_": true}
	for _, p := range cl.Params {
		locals[p.Name] = true
	}
	collectBoundNames(cl.Body.Items, locals)

	var slots []capSlot
	seen := map[string]bool{}
	ok := walkBody(cl.Body, func(x ast.Expr) {
		id, is := x.(*ast.Ident)
		if !is || id.Name == "" || locals[id.Name] || seen[id.Name] {
			return
		}
		s, found := e.captureOf(id.Name)
		if !found {
			return
		}
		seen[id.Name] = true
		slots = append(slots, s)
	})
	if !ok {
		return nil, false
	}
	off := 0
	for i := range slots {
		slots[i].off = 16 + 8*off
		off += slots[i].n
	}
	return slots, true
}

// emitEnvBlock opens one environment block and stores the captured words
// into it: the frozen header's trace bitmap (one bit per word, set where
// the word holds a gc reference), the root push that keeps the block —
// and through it everything it points at — alive for the creating body's
// lifetime, and one store per captured word.
func (e *emitter) emitEnvBlock(slots []capSlot, wordsOf func(capSlot) ([]string, *NotImplemented)) (string, *NotImplemented) {
	words := 0
	for _, s := range slots {
		words += s.n
	}
	bitmap := make([]uint64, (words+63)/64)
	for _, s := range slots {
		wi := (s.off - 16) / 8
		for k := 0; k < s.n; k++ {
			if s.trace[k] {
				bitmap[(wi+k)/64] |= 1 << uint((wi+k)%64)
			}
		}
	}
	envMap := fmt.Sprintf("@.emap%d", len(e.envMaps))
	parts := make([]string, len(bitmap))
	for i, w := range bitmap {
		parts[i] = fmt.Sprintf("i64 %d", w)
	}
	e.envMaps = append(e.envMaps, fmt.Sprintf("%s = private unnamed_addr constant [%d x i64] [%s]",
		envMap, len(bitmap), strings.Join(parts, ", ")))
	e.use("__we_alloc")
	e.use("__we_root_push")
	e.pushes++
	env := "%" + e.value()
	e.inst(fmt.Sprintf("%s = call ptr @__we_alloc(i64 %d)", env, 16+8*words))
	e.inst(fmt.Sprintf("store ptr %s, ptr %s", envMap, env))
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", env))
	for _, s := range slots {
		ops, ni := wordsOf(s)
		if ni != nil {
			return "", ni
		}
		for k, op := range ops {
			e.gepStore(env, s.off+8*k, "i64 "+op)
		}
	}
	return env, nil
}

// captureWords loads one capture's words as i64 operands — the store
// side's half of the block's uniform i64 storage. A pointer capture
// arrives as its address word, which the thunk's load side inttoptrs back.
func (e *emitter) captureWords(s capSlot) ([]string, *NotImplemented) {
	switch {
	case s.key != "":
		return []string{e.ptrWord(e.gcEnv[s.name].reg)}, nil
	case s.prim:
		if p, ok := e.prims[s.name]; ok {
			return []string{e.ptrWord(p)}, nil
		}
		// The enclosing capture's handle: the word is already an address.
		op, _, ni := e.loadCapture(s.name)
		if ni != nil {
			return nil, ni
		}
		return []string{op}, nil
	case s.n == 2:
		// The pair resolves exactly as any String read does — a static
		// literal interns into the module's constants, a computed one
		// carries its own operand pair.
		p, l, ni := e.emitStringExpr(&ast.Ident{Name: s.name})
		if ni != nil {
			return nil, ni
		}
		return []string{e.ptrWord(p), l}, nil
	case s.trace[0]:
		return []string{e.ptrWord(e.fnEnv[s.name].carrier)}, nil
	}
	op, isF, ni := e.emitNumExpr(&ast.Ident{Name: s.name})
	if ni != nil {
		return nil, ni
	}
	if isF {
		// A float capture would ride the block as its bits re-read as an
		// integer; the two faces must agree, so it stops here (T11).
		return nil, e.bnd()
	}
	return []string{op}, nil
}

// materializeCaptures loads one closure's environment block into the
// thunk's own binding environments at its entry: every captured name then
// resolves exactly as a local does, at every face the body can use it — a
// scalar's word, a record's pointer (with its record key), a String's
// pair, a primitive's handle, a fn value's carrier.
func (e *emitter) materializeCaptures(slots []capSlot, env string) {
	for _, s := range slots {
		word := func(k int) string {
			p := e.value()
			e.inst(fmt.Sprintf("%%%s = getelementptr i8, ptr %s, i64 %d", p, env, s.off+8*k))
			w := e.value()
			e.inst(fmt.Sprintf("%%%s = load i64, ptr %%%s", w, p))
			return "%" + w
		}
		switch {
		case s.n == 2:
			e.strEnv[s.name] = strBinding{dataOp: e.wordPtr(word(0)), lenOp: word(1)}
		case s.key != "":
			e.gcEnv[s.name] = gcBinding{rec: s.key, reg: e.wordPtr(word(0))}
		case s.prim:
			e.prims[s.name] = e.wordPtr(word(0))
		case s.trace[0]:
			e.fnEnv[s.name] = fnValue{carrier: e.wordPtr(word(0)), abi: s.abi, typed: s.typed}
		default:
			e.scalars[s.name] = scalarSlot{operand: word(0), kind: s.kind}
		}
	}
}

// ptrWord converts one pointer operand to its address word, and wordPtr
// converts back — the two directions the environment block's uniform i64
// storage needs.
func (e *emitter) ptrWord(ptr string) string {
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = ptrtoint ptr %s to i64", v, ptr))
	return "%" + v
}

func (e *emitter) wordPtr(word string) string {
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = inttoptr i64 %s to ptr", v, word))
	return "%" + v
}

// closureParts is a closure's two operands as emitted: the thunk's code
// pointer and its environment block — the pair a callback ABI takes as
// its two pointer words, and what makeFnValue puts behind one value.
type closureParts struct {
	fnptr string
	env   string
}

// closureAbi resolves one closure's calling shape. A position that fixes
// it — a callback parameter, a fn-typed argument, an annotated binding —
// hands the classified signature in, and the closure's own declarations
// must agree with it where it writes any (chapter 12's E0501 is the check
// stage's rule; this is the emitter's own agreement: same arity, same
// word families). Without one, every parameter must name its type and the
// return is the closure's own or, where it names none, the body tail's
// domain. Anything else reports false — the closure stops at the fn body
// word rather than guessing an ABI.
func (e *emitter) closureAbi(cl *ast.Closure, expected *fnAbi) (fnAbi, bool) {
	if expected != nil {
		if len(cl.Params) != len(expected.params) {
			return fnAbi{}, false
		}
		for i, p := range cl.Params {
			if p.Type == nil {
				continue // a bare parameter takes the position's own type
			}
			k, _, ok := e.classType(p.Type)
			if !ok || k != expected.params[i].kind {
				return fnAbi{}, false
			}
		}
		if cl.Ret != nil {
			k, _, ok := e.classType(cl.Ret)
			if !ok || k != expected.ret {
				return fnAbi{}, false
			}
		}
		return *expected, true
	}
	params := make([]ast.Param, len(cl.Params))
	for i, p := range cl.Params {
		params[i] = p
		if p.Type == nil {
			return fnAbi{}, false // nothing in reach names this parameter
		}
	}
	if cl.Ret != nil {
		return e.fitAbi(cl.Ret, params)
	}
	// The closure names no return type: a short closure's body IS its
	// return (chapter 12 — the last expression's value is the closure's),
	// so the value's own domain is what the signature carries. A body
	// whose tail is no expression binds nothing and returns nothing; one
	// whose tail is an expression of no nameable domain is a shape this
	// face does not read, and the closure stops rather than guessing a
	// void thunk over a value the check stage knows is there.
	if len(cl.Body.Items) == 0 {
		return e.fitAbi(nil, params)
	}
	last, ok := cl.Body.Items[len(cl.Body.Items)-1].(*ast.ExprStmt)
	if !ok {
		return e.fitAbi(nil, params)
	}
	name := strKindName(e.valueKindIn(params, last.Expr))
	if name == "" {
		return fnAbi{}, false
	}
	return e.fitAbi(&ast.NamedType{Name: name}, params)
}

// closureForType classifies one written function type into the signature
// its thunks and call sites share.
func (e *emitter) closureForType(ft *ast.FnType) (fnAbi, bool) {
	if ft == nil || len(ft.EffectTags) != 0 {
		return fnAbi{}, false
	}
	return e.fitAbi(ft.Ret, paramsOfType(ft))
}

// valueKindIn classifies one expression with the enclosing closure's
// parameters in scope: the domain inference the signature's unnamed
// return needs runs before the thunk exists, so the parameters it reads
// are seeded from their own declarations rather than from the define's
// environments. The seeding is a scratch view — the creation site's
// environments are untouched.
func (e *emitter) valueKindIn(params []ast.Param, x ast.Expr) strKind {
	savedScalars, savedStr, savedGc := e.scalars, e.strEnv, e.gcEnv
	e.scalars = make(map[string]scalarSlot, len(savedScalars))
	for k, v := range savedScalars {
		e.scalars[k] = v
	}
	e.strEnv = make(map[string]strBinding, len(savedStr))
	for k, v := range savedStr {
		e.strEnv[k] = v
	}
	e.gcEnv = make(map[string]gcBinding, len(savedGc))
	for k, v := range savedGc {
		e.gcEnv[k] = v
	}
	for _, p := range params {
		name := ""
		if t, ok := p.Type.(*ast.NamedType); ok && t.Qual == "" && len(t.Args) == 0 {
			name = t.Name
		}
		switch k := baseStrKind(name); k {
		case skStr:
			e.strEnv[p.Name] = strBinding{}
		case skNone:
			// A named record parameter: its key is the module-qualified
			// name the creation site's module resolves it in, so a field
			// read off it classifies as the field's own domain.
			if name != "" {
				if _, ok := e.records[e.curKey+"."+name]; ok {
					e.gcEnv[p.Name] = gcBinding{rec: e.curKey + "." + name}
				}
			}
		default:
			e.scalars[p.Name] = scalarSlot{kind: k}
		}
	}
	k := e.valueKind(x)
	e.scalars, e.strEnv, e.gcEnv = savedScalars, savedStr, savedGc
	return k
}

// emitClosure emits one closure as an internal thunk define (design D5,
// chapter 12): the body IS a function body — its tail expression the
// implicit value, its `return`s the thunk's own, its `defer`s at the
// thunk's exit — and every binding the body reads from its creation
// site's scope is copied into an environment block there, where the
// bindings still live. The thunk's first parameter is that block; the
// declared parameters follow at the signature's own ABI.
func (e *emitter) emitClosure(cl *ast.Closure, expected *fnAbi) (closureParts, fnAbi, *NotImplemented) {
	abi, ok := e.closureAbi(cl, expected)
	if !ok {
		return closureParts{}, fnAbi{}, bndFn()
	}
	name := fmt.Sprintf("@.cb%d", e.cbN)
	e.cbN++
	slots, ok := e.collectClosureCaptures(cl)
	if !ok {
		return closureParts{}, fnAbi{}, bndFn()
	}
	// The environment block, built at the creation site: the captured
	// values are read here, before the thunk's own environments replace
	// these.
	env := "null"
	if len(slots) > 0 {
		blk, ni := e.emitEnvBlock(slots, e.captureWords)
		if ni != nil {
			return closureParts{}, fnAbi{}, ni
		}
		env = blk
	}

	savedCtx, savedBody := e.ctx, e.body
	savedCur := e.curBlock
	savedAllocas, savedAssigned := e.allocas, e.assigned
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedFns := e.fnEnv
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedList := e.listEnv
	savedTup, savedNt := e.tupEnv, e.ntEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	savedFrames := e.frames
	savedDiverged := e.diverged
	savedExit, savedInExit := e.exit, e.inExit
	savedLoops, savedScopes := e.loopFrames, e.scopeLive
	savedRes, savedNest := e.resFrames, e.nest
	restore := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.curBlock = savedCur
		e.allocas, e.assigned = savedAllocas, savedAssigned
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.fnEnv = savedFns
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.listEnv = savedList
		e.tupEnv, e.ntEnv = savedTup, savedNt
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
		e.frames = savedFrames
		e.diverged = savedDiverged
		e.exit, e.inExit = savedExit, savedInExit
		e.loopFrames, e.scopeLive = savedLoops, savedScopes
		e.resFrames, e.nest = savedRes, savedNest
	}
	e.ctx = ctxFn
	e.beginBody()
	e.curBlock = ""
	e.allocas = nil
	e.assigned = make(map[string]bool)
	e.diverged = false
	e.scalars = make(map[string]scalarSlot)
	e.fnEnv = make(map[string]fnValue)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.listEnv = make(map[string]listBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.tupEnv = make(map[string]tupBinding)
	e.ntEnv = make(map[string]string)
	e.defers = nil
	e.caps = nil
	e.pushes = 0
	e.exit = &exitSite{kind: exitFn, abi: abi}
	e.loopFrames, e.scopeLive, e.inExit = nil, nil, false
	e.resFrames, e.nest = nil, 0
	collectAssigned(cl.Body.Items, e.assigned)
	ps := e.bindDefineParams(cl.Params, abi)
	e.materializeCaptures(slots, "%env")
	body, retOp, diverged, ni := e.emitBodyCore(cl.Body.Items, abi, abi.ret != abiVoid)
	if ni != nil {
		restore()
		return closureParts{}, fnAbi{}, ni
	}
	restore()
	head := strings.Join(ps, ", ")
	if head != "" {
		head = ", " + head
	}
	if diverged {
		e.thunks = append(e.thunks, fmt.Sprintf(
			"define internal %s %s(ptr %%env%s) {\nentry:\n%s}\n", abi.retTyp, name, head, body))
	} else {
		e.thunks = append(e.thunks, fmt.Sprintf(
			"define internal %s %s(ptr %%env%s) {\nentry:\n%s  ret %s\n}\n",
			abi.retTyp, name, head, body, retOp))
	}
	return closureParts{fnptr: name, env: env}, abi, nil
}

// emitClosureValue emits one closure literal in value position: the thunk
// and its environment block behind the single carrier a fn value is. The
// binding's annotation, where it writes the function type, is the
// signature the closure's bare parameters read.
func (e *emitter) emitClosureValue(cl *ast.Closure, typ ast.TypeRef) (callResult, *NotImplemented) {
	var sig *fnAbi
	if ft, ok := typ.(*ast.FnType); ok {
		abi, ok := e.closureForType(ft)
		if !ok {
			return callResult{}, bndFn()
		}
		sig = &abi
	}
	parts, abi, ni := e.emitClosure(cl, sig)
	if ni != nil {
		return callResult{}, ni
	}
	return callResult{kind: ckFn, fn: e.makeFnValue(parts, abi)}, nil
}

// emitFnRef emits a program fn named in value position: the constant pair
// design D5 gives it — a null environment (a declared fn captures
// nothing) and a code pointer. The pointer is an adapter thunk's, not the
// fn's own symbol: every fn value is called as `<fnptr>(env, args...)`,
// which is the closure thunk's shape, so a declared fn — whose own
// signature has no environment parameter — is reached through the thunk
// that supplies one. One convention at every call site is what lets a
// body call through a parameter without knowing which kind of fn value
// arrived.
func (e *emitter) emitFnRef(fd *fnDef) (callResult, *NotImplemented) {
	if fd.foreign {
		return callResult{}, bndFn()
	}
	abi, ok := e.classify(fd)
	if !ok {
		return callResult{}, bndFn()
	}
	parts, ni := e.fnAdapter(fd, abi)
	if ni != nil {
		return callResult{}, ni
	}
	return callResult{kind: ckFn, fn: e.makeFnValue(parts, abi)}, nil
}

// fnAdapter emits the env-taking thunk that fronts one declared fn in
// value position: the fn's own ABI without the environment parameter,
// forwarded word for word.
func (e *emitter) fnAdapter(fd *fnDef, abi fnAbi) (closureParts, *NotImplemented) {
	name := fmt.Sprintf("@.cb%d", e.cbN)
	e.cbN++
	var decl, args []string
	n := 0
	for _, p := range abi.params {
		for _, wt := range abiWordTypes(p) {
			reg := fmt.Sprintf("%%a%d", n)
			decl = append(decl, wt+" "+reg)
			args = append(args, wt+" "+reg)
			n++
		}
	}
	head := strings.Join(decl, ", ")
	if head != "" {
		head = ", " + head
	}
	call := fmt.Sprintf("call %s %s(%s)", abi.retTyp, "@"+fd.sym(), strings.Join(args, ", "))
	var body string
	if abi.ret == abiVoid {
		body = "  " + call + "\n  ret void\n"
	} else {
		body = fmt.Sprintf("  %%r = %s\n  ret %s %%r\n", call, abi.retTyp)
	}
	e.thunks = append(e.thunks, fmt.Sprintf(
		"define internal %s %s(ptr %%env%s) {\nentry:\n%s}\n", abi.retTyp, name, head, body))
	return closureParts{fnptr: name, env: "null"}, nil
}

// abiWordTypes lists the IR words one classified parameter crosses as.
func abiWordTypes(p fnParamAbi) []string {
	switch p.kind {
	case abiI64:
		return []string{"i64"}
	case abiDouble:
		return []string{"double"}
	case abiStr:
		return []string{"ptr", "i64"}
	case abiSum:
		return []string{"i64", "i64"}
	case abiGc, abiFn:
		return []string{"ptr"}
	case abiTuple:
		var ws []string
		for _, el := range p.elems {
			ws = append(ws, el.words...)
		}
		return ws
	}
	return nil
}

// fnValueCall emits a call through a bound fn value: the pair loads back
// out of the carrier — the code pointer the call goes through, the
// environment the thunk's first parameter — and the arguments follow at
// the signature the value was built with.
func (e *emitter) emitFnValueCall(fv fnValue, args []ast.Expr) (callResult, *NotImplemented) {
	if !fv.typed {
		return callResult{}, bndFn()
	}
	if len(args) != len(fv.abi.params) {
		return callResult{}, e.bnd()
	}
	parts := e.fnParts(fv)
	return e.emitCallCore(fv.abi, parts.fnptr, []string{"ptr " + parts.env}, args)
}

// fnMapName is the carrier's trace descriptor: one payload word pair,
// whose second word is the gc environment (the first is a code pointer,
// never a gc reference).
func (e *emitter) fnMapName() string {
	if e.fnMap == "" {
		e.fnMap = fmt.Sprintf("@.fnmap%d", len(e.envMaps))
		e.envMaps = append(e.envMaps, e.fnMap+" = private unnamed_addr constant [1 x i64] [i64 2]")
	}
	return e.fnMap
}

// makeFnValue turns a closure's two operands into one function value: the
// gc carrier design D5 stores the pair in ({fnptr, env}), rooted by the
// creating body. The carrier is one pointer everywhere a fn value goes,
// and the pair loads back out of it at a call — or is handed straight to
// a callback ABI, whose (fn, env) shape it already is.
func (e *emitter) makeFnValue(parts closureParts, abi fnAbi) fnValue {
	e.use("__we_alloc")
	e.use("__we_root_push")
	e.pushes++
	car := "%" + e.value()
	e.inst(fmt.Sprintf("%s = call ptr @__we_alloc(i64 32)", car))
	e.inst(fmt.Sprintf("store ptr %s, ptr %s", e.fnMapName(), car))
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", car))
	e.gepStore(car, 16, "ptr "+parts.fnptr)
	e.gepStore(car, 24, "ptr "+parts.env)
	return fnValue{carrier: car, abi: abi, typed: true}
}

// emitFnArg yields one function value in argument position: the carrier
// the callee reads, emitted at this call site so the closure's captures
// freeze where the argument is written. sig is the parameter's declared
// signature, which a closure literal's bare parameters take their types
// from and which a name's own signature must agree with.
func (e *emitter) emitFnArg(a ast.Expr, sig *fnAbi) (fnValue, *NotImplemented) {
	switch v := a.(type) {
	case *ast.Closure:
		parts, abi, ni := e.emitClosure(v, sig)
		if ni != nil {
			return fnValue{}, ni
		}
		if sig != nil && !sameAbi(abi, *sig) {
			return fnValue{}, e.bnd()
		}
		return e.makeFnValue(parts, abi), nil
	case *ast.Ident:
		if fv, ok := e.fnEnv[v.Name]; ok {
			if sig != nil && (!fv.typed || !sameAbi(fv.abi, *sig)) {
				return fnValue{}, e.bnd()
			}
			return fv, nil
		}
		if !e.isLocalName(v.Name) {
			if fd, ok := e.fnTable[e.curKey+"."+v.Name]; ok {
				res, ni := e.emitFnRef(fd)
				if ni != nil {
					return fnValue{}, ni
				}
				if sig != nil && !sameAbi(res.fn.abi, *sig) {
					return fnValue{}, e.bnd()
				}
				return res.fn, nil
			}
		}
	}
	return fnValue{}, e.bnd()
}

// sameAbi reports whether two signatures are the same shape: the word
// counts and families a call must agree on (chapter 12's E0501 — the
// check stage owns the rule, this is the emitter's own agreement).
func sameAbi(a, b fnAbi) bool {
	if a.ret != b.ret || len(a.params) != len(b.params) {
		return false
	}
	for i := range a.params {
		if a.params[i].kind != b.params[i].kind {
			return false
		}
	}
	return true
}

// fnParts loads one bound function value's pair back out of its carrier.
func (e *emitter) fnParts(fv fnValue) closureParts {
	return closureParts{
		fnptr: e.gepLoadPtr(fv.carrier, 16),
		env:   e.gepLoadPtr(fv.carrier, 24),
	}
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

// envFrame is one block's environment snapshot: push clones the five
// per-body env maps onto the block's own view, pop restores the saved
// maps. Registrations a block makes therefore live exactly as long as
// the block's own emission — never past its join — and a same-named
// outer binding keeps its own operand underneath (the scoping the
// follow-up #16 dominance fix rests on). Cloning is cheap: the maps
// hold a body's local names only.
type envFrame struct {
	scalars map[string]scalarSlot
	strEnv  map[string]strBinding
	gcEnv   map[string]gcBinding
	listEnv map[string]listBinding
	tupEnv  map[string]tupBinding
	ntEnv   map[string]string
	sums2   map[string]sumSlot
	prims   map[string]string
}

// pushEnv opens one block scope: the maps are cloned (maps.Clone keeps
// a nil source nil — a fresh fn/task context's empty maps included) and
// the previous views saved for popEnv.
func (e *emitter) pushEnv() {
	e.frames = append(e.frames, envFrame{
		scalars: e.scalars, strEnv: e.strEnv, gcEnv: e.gcEnv,
		listEnv: e.listEnv,
		tupEnv:  e.tupEnv, ntEnv: e.ntEnv, sums2: e.sums2, prims: e.prims,
	})
	e.scalars = maps.Clone(e.scalars)
	e.strEnv = maps.Clone(e.strEnv)
	e.gcEnv = maps.Clone(e.gcEnv)
	e.listEnv = maps.Clone(e.listEnv)
	e.tupEnv = maps.Clone(e.tupEnv)
	e.ntEnv = maps.Clone(e.ntEnv)
	e.sums2 = maps.Clone(e.sums2)
	e.prims = maps.Clone(e.prims)
}

// popEnv closes one block scope, restoring the enclosing views.
func (e *emitter) popEnv() {
	f := e.frames[len(e.frames)-1]
	e.frames = e.frames[:len(e.frames)-1]
	e.scalars, e.strEnv, e.gcEnv, e.listEnv, e.tupEnv, e.ntEnv = f.scalars, f.strEnv, f.gcEnv, f.listEnv, f.tupEnv, f.ntEnv
	e.sums2, e.prims = f.sums2, f.prims
}

// valueForm is one value-position expression's join (design D2): the arms
// compute their values, each stores into the form's result slot, and the
// join loads it. The slot is reserved by the first arm that produces a
// value — an alloca's text splices in behind the entry label whenever it
// is requested, so where the reservation happens is unobservable — and
// every later arm writes that same slot. An expression whose arms produce
// no value at all is unit and reserves nothing; one that mixes valued and
// valueless arms has no single value to load (a mismatch the check face
// rules out) and stops at the body word.
type valueForm struct {
	slot    string
	isFloat bool
	unit    bool
}

// emitValueForm emits one value-position expression and returns its value:
// the if/match/block forms whose arms produce i64- or double-domain
// values. The load lands after the form's join — the outermost join for a
// chain — which is exactly where the consumption site reads it.
func (e *emitter) emitValueForm(x ast.Expr) (callResult, *NotImplemented) {
	vf := &valueForm{}
	var ni *NotImplemented
	switch v := x.(type) {
	case *ast.If:
		ni = e.emitIf(v, vf)
	case *ast.Match:
		ni = e.emitMatch(v, vf)
	case *ast.BlockExpr:
		ni = e.emitArmBlock(v.Block.Items, vf)
	default:
		return callResult{}, e.bnd()
	}
	if ni != nil {
		return callResult{}, ni
	}
	if vf.slot == "" {
		return callResult{kind: ckVoid}, nil
	}
	if vf.unit {
		return callResult{}, e.bnd()
	}
	return callResult{kind: ckI64, i64: e.loadNum(vf.slot, vf.isFloat), isFloat: vf.isFloat}, nil
}

// put writes one arm's value into the form's result slot, reserving the
// slot the first time. An arm whose expression carries no value — a
// valueless call, an io call — leaves the sink untouched and marks the
// form unit.
func (vf *valueForm) put(e *emitter, res callResult) *NotImplemented {
	switch res.kind {
	case ckVoid, ckIo:
		vf.unit = true
		return nil
	case ckI64:
		typ := "i64"
		if res.isFloat {
			typ = "double"
		}
		if vf.slot == "" {
			vf.slot, vf.isFloat = e.slot(typ), res.isFloat
		} else if vf.isFloat != res.isFloat {
			return e.bnd()
		}
		e.inst(fmt.Sprintf("store %s %s, ptr %s", typ, res.i64, vf.slot))
		return nil
	}
	// Strings, records, and sums have value forms of their own (design
	// D3/D4/D8); the result-slot join is the numeric set's.
	return e.bnd()
}

// emitArmBlock emits one arm's block in a value form: every statement but
// the tail walks as usual, then the tail — the block's value — computes
// into the form's sink. A block whose last item is not an expression
// carries no value: it is unit, and the arm stores nothing.
//
// The arm is a body like any block, so it discharges its own roots after
// the tail has computed — both arms of the join must hand the code after
// the if the depth the if opened with, and only the arm that pushed knows
// what it owes. The discharge follows the store rather than preceding it:
// the sink is where the value leaves the arm, and what the arm rooted is
// owed from the moment after that.
func (e *emitter) emitArmBlock(items []ast.Stmt, vf *valueForm) *NotImplemented {
	base := e.pushes
	e.pushEnv()
	defer e.popEnv()
	tail := ast.Expr(nil)
	if n := len(items); n > 0 {
		if es, ok := items[n-1].(*ast.ExprStmt); ok {
			tail = es.Expr
			items = items[:n-1]
		}
	}
	for _, st := range items {
		if ni := e.emitStmt(st); ni != nil {
			return ni
		}
	}
	if tail != nil && !e.diverged {
		if ni := e.storeValue(tail, vf); ni != nil {
			return ni
		}
	}
	if !e.diverged {
		e.dischargeRoots(base)
	}
	e.pushes = base
	return nil
}

// storeValue computes one value-position expression into the form's sink:
// a nested control form joins through the same sink, and anything else is
// the numeric set's own operand (which reaches the operand-position call
// and the unary family in turn).
func (e *emitter) storeValue(x ast.Expr, vf *valueForm) *NotImplemented {
	switch v := x.(type) {
	case *ast.If:
		return e.emitIf(v, vf)
	case *ast.Match:
		return e.emitMatch(v, vf)
	case *ast.BlockExpr:
		return e.emitArmBlock(v.Block.Items, vf)
	case *ast.Call:
		res, ni := e.emitCall(v, nil)
		if ni != nil {
			return ni
		}
		return vf.put(e, res)
	}
	op, isF, ni := e.emitNumExpr(x)
	if ni != nil {
		return ni
	}
	return vf.put(e, callResult{kind: ckI64, i64: op, isFloat: isF})
}

// emitBlockStmts emits a nested block's statements: the statements walk in
// order, and a return at any depth is the enclosing body's own exit
// (emitReturn carries the exit sequence). The body is one block scope:
// bindings it makes roll back when it closes.
//
// The block is also a body on the root face, and it closes as one: what it
// pushed it pops where its emission ends, so the code that follows runs at
// the window depth the block opened with. The block is emitted ONCE while
// its runtime path may run many times — a loop body, an arm of an if that
// runs on some passes and not others — so the pushes counted here are one
// path's, and the discharge belongs on the path, not at the function's
// exit. That is the accounting design D7's root protocol states ("根推送按
// body 记账，每个 body 出口弹"), and a block's end is its exit.
//
// A block that ended in its own terminator (a return, a break, a continue)
// owes nothing here: that edge carried its own discharge (emitReturn pops
// the live set, emitBreak/emitContinue discharge to the loop body's
// depth). The counter returns to the depth the block opened at either
// way — text after a terminator is dead, and a join reached from an arm
// must find the depth the arm did not change.
func (e *emitter) emitBlockStmts(items []ast.Stmt) *NotImplemented {
	base := e.pushes
	e.pushEnv()
	defer e.popEnv()
	for _, st := range items {
		if ni := e.emitStmt(st); ni != nil {
			return ni
		}
	}
	if !e.diverged {
		e.dischargeRoots(base)
	}
	e.pushes = base
	return nil
}

// dischargeRoots pops the roots pushed since base — the body's own, last
// push first, which is the order the runtime's shadow stack requires. It
// emits nothing when the body pushed nothing, so the whole existing corpus
// emits the same bytes it did before the accounting became per body.
func (e *emitter) dischargeRoots(base int) {
	for i := e.pushes - base; i > 0; i-- {
		e.use("__we_root_pop")
		e.inst("call void @__we_root_pop()")
	}
}

// emitIf emits the conditional: then plus optional else (a block or a
// nested else-if), both arms joining one continuation label. With a value
// form each arm computes its value into the form's sink rather than
// running as statements (design D2) — one shape, two faces: the arms
// differ in what they leave behind, not in how control reaches the join.
func (e *emitter) emitIf(s *ast.If, vf *valueForm) *NotImplemented {
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
	if ni := e.emitIfArm(s.Then.Items, vf); ni != nil {
		return ni
	}
	if vf != nil && s.Else == nil && vf.slot != "" {
		// A valueless else alongside a valued then has no single value:
		// the else path reaches the join without storing. The check face
		// rules the shape out — a valueless if is unit — so this is the
		// boundary's defensive face.
		return e.bnd()
	}
	// The join branch is guarded: an arm that ended diverged (unreachable,
	// or a return/break once those land) already terminated its block —
	// nothing may follow the terminator. The join label still opens so the
	// code after the if has a block to live in.
	if !e.diverged {
		e.inst(fmt.Sprintf("br label %%%s", join))
	}
	if s.Else != nil {
		e.label(elseL)
		switch els := s.Else.(type) {
		case *ast.BlockExpr:
			if ni := e.emitIfArm(els.Block.Items, vf); ni != nil {
				return ni
			}
		case *ast.If:
			if ni := e.emitIf(els, vf); ni != nil {
				return ni
			}
			// The nested if already closed on its own join; branch to
			// this one.
		}
		if !e.diverged {
			e.inst(fmt.Sprintf("br label %%%s", join))
		}
	}
	e.label(join)
	return nil
}

// emitIfArm emits one if arm: as statements when no value form is live,
// else as a value arm whose tail computes into the form's sink.
func (e *emitter) emitIfArm(items []ast.Stmt, vf *valueForm) *NotImplemented {
	if vf == nil {
		return e.emitBlockStmts(items)
	}
	return e.emitArmBlock(items, vf)
}

// exitKind names the enclosing body's return protocol (T2 deep returns,
// chapter 3's return and chapter 18's piercing exits).
type exitKind int

const (
	exitMain exitKind = iota // the entry's Result face: ret i32 0, or __we_fail
	exitFn                   // the declared fn's own ABI
	exitTask                 // the thunk's i64 value; no root pops (its own window)
	exitTest                 // valueless: ret void
)

// exitSite is the return protocol of the body being walked — what a
// `return` emit at any depth answers with.
type exitSite struct {
	kind exitKind
	abi  fnAbi // exitFn only
}

// loopFrame is one open loop's break/continue targets (chapter 3: the
// innermost enclosing loop). depth is the nesting ordinal the loop opened
// at: every compound scope and every resource block opened at or above it
// is lexically inside the loop body, and a piercing exit from the loop
// must discharge exactly those.
type loopFrame struct {
	brk   string // break target: the loop's exit label
	cont  string // continue target: the re-evaluation point
	depth int
	// roots is the root depth the body opened at. A break or continue
	// leaves the pass before the pass's own end, so the code that would
	// have discharged its roots is exactly the code it jumps past; the
	// edge carries the discharge itself, down to this depth — not to the
	// function's, because everything below it (the enclosing body's own
	// roots, a for's snapshot) is still live on the far side.
	roots int
}

// liveScope is one compound scope entered and not yet left: the handle
// __we_scope_enter returned, whether its exit collects (joins across
// panics) instead of cancelling (the fail-fast faces), and the nesting
// ordinal it opened at.
type liveScope struct {
	handle  string
	collect bool
	depth   int
}

// resFrame is one open scope resource statement (chapter 13): its
// bindings in declaration order, each naming the block binding, the
// resource record's key — the release dispatches on the head — and the
// record pointer the head produced, which is the receiver. depth is the
// nesting ordinal the block opened at.
type resFrame struct {
	live  []resBinding
	depth int
}

// resBinding is one binding of a scope resource head.
type resBinding struct {
	name string
	key  string
	reg  string
}

// unwind discharges every open compound scope and resource block at or
// above depth, innermost first (chapter 18:231 — an early return, break,
// or continue through an open scope discharges the scope's remaining
// handles without violation; chapter 13 — an exit a return, break or
// continue pierces is that block's exit and releases like any other).
// One sequence serves both kinds because their nesting is what orders
// them, not which kind they are: the entries of the two stacks interleave
// by the ordinal they opened at. A collect scope joins — its leave parks
// until every task returns; a plain or timeout scope is fail-fast:
// cancel marks it timed out and cancels its unfinished tasks, then its
// leave waits out the cooperative returns. A resource block releases its
// bindings in reverse declaration order. Nothing is popped here: the
// stacks are lexical, and the block that opened an entry closes it — a
// site that discharged on a diverging path leaves the enclosing emission
// to skip its own copy, the discipline drainDefers rides too.
func (e *emitter) unwind(depth int) *NotImplemented {
	type opening struct {
		depth int
		scope int // index into scopeLive, or -1 for a resource block
		res   int // index into resFrames when scope is -1
	}
	var open []opening
	for i := range e.scopeLive {
		if e.scopeLive[i].depth >= depth {
			open = append(open, opening{depth: e.scopeLive[i].depth, scope: i, res: -1})
		}
	}
	for i := range e.resFrames {
		if e.resFrames[i].depth >= depth {
			open = append(open, opening{depth: e.resFrames[i].depth, scope: -1, res: i})
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].depth > open[j].depth })
	for _, op := range open {
		if op.scope < 0 {
			if ni := e.releaseRes(e.resFrames[op.res].live); ni != nil {
				return ni
			}
			continue
		}
		ls := e.scopeLive[op.scope]
		if !ls.collect {
			e.use("__we_scope_cancel")
			e.inst(fmt.Sprintf("call void @__we_scope_cancel(ptr %s)", ls.handle))
		}
		e.use("__we_scope_leave")
		e.inst(fmt.Sprintf("call i64 @__we_scope_leave(ptr %s)", ls.handle))
	}
	return nil
}

// releaseRes discharges one resource block's bindings: exactly one
// release call per binding, in reverse declaration order (chapter 13).
// The call is a method call through the T5 table — the head's own
// `release`, the body `impl Releasable for Head` contributes — with the
// binding's record pointer as the receiver. A head whose record the
// table cannot name stops at the fn body word: the release is the one
// thing this statement owes and it may not be silently dropped.
func (e *emitter) releaseRes(live []resBinding) *NotImplemented {
	for i := len(live) - 1; i >= 0; i-- {
		fd, ok := e.methods[live[i].key+".release"]
		if !ok {
			return e.bnd()
		}
		abi, ok := e.classify(fd)
		if !ok {
			return bndFn()
		}
		if _, ni := e.emitCallCore(abi, "@"+fd.sym(), []string{"ptr " + live[i].reg}, nil); ni != nil {
			return ni
		}
	}
	return nil
}

// emitBreak leaves the innermost loop: discharge the scopes the exit
// crosses, branch to the loop's exit label, and mark the block diverged.
func (e *emitter) emitBreak() *NotImplemented {
	if len(e.loopFrames) == 0 {
		return e.bnd() // E0201 owns this face; the bnd is the checked-world defense
	}
	fr := e.loopFrames[len(e.loopFrames)-1]
	e.unwind(fr.depth)
	e.dischargeRoots(fr.roots)
	e.inst(fmt.Sprintf("br label %%%s", fr.brk))
	e.diverged = true
	return nil
}

// emitContinue restarts the innermost loop: the same discharge, then the
// branch back to the re-evaluation point.
func (e *emitter) emitContinue() *NotImplemented {
	if len(e.loopFrames) == 0 {
		return e.bnd()
	}
	fr := e.loopFrames[len(e.loopFrames)-1]
	e.unwind(fr.depth)
	e.dischargeRoots(fr.roots)
	e.inst(fmt.Sprintf("br label %%%s", fr.cont))
	e.diverged = true
	return nil
}

// emitLoopBody emits one loop's body under its frame: the frame is what
// break and continue address, and its roots entry is the depth the body
// opened at — the same depth the body's own discharge returns the window
// to, taken here so the two cannot drift apart.
func (e *emitter) emitLoopBody(items []ast.Stmt, fr loopFrame) *NotImplemented {
	fr.roots = e.pushes
	e.loopFrames = append(e.loopFrames, fr)
	ni := e.emitBlockStmts(items)
	e.loopFrames = e.loopFrames[:len(e.loopFrames)-1]
	return ni
}

// emitWhile emits the loop: head re-evaluates the condition, the body
// branches back, the exit label continues. The loop frame opened around
// the body gives break and continue their targets.
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
	if ni := e.emitLoopBody(s.Body.Items, loopFrame{brk: exit, cont: head, depth: e.nest}); ni != nil {
		return ni
	}
	// The back branch is guarded the same way the if joins are: a body
	// that diverged ended in its own terminator, and only the head's false
	// edge reaches the exit label then.
	if !e.diverged {
		e.inst(fmt.Sprintf("br label %%%s", head))
	}
	e.label(exit)
	return nil
}

// drainDefers runs the deferred blocks collected for this body in reverse
// registration order (chapter 3) at the exit path being emitted. The list
// stays intact: every exit path drains its own copy — a deep return drains
// at the return site, the tail drains in the tail's block, and a loop's
// continuation drains in its block. Each is a distinct runtime path, so
// each owes the body's defers; consuming the list at the first drain would
// leave every later path without its cleanup. A defer registered by the
// drain's own statements cannot re-enter (E0204 keeps defers at the fn
// body's top level).
func (e *emitter) drainDefers() *NotImplemented {
	for i := len(e.defers) - 1; i >= 0; i-- {
		if ni := e.emitBlockStmts(e.defers[i].Items); ni != nil {
			return ni
		}
	}
	return nil
}

// popRoots discharges the body's root pushes (the entry and the fns pop
// at exit; the task thunk pops nothing — its pushes ride the task's own
// gc window, retired at task end).
func (e *emitter) popRoots() {
	for i := 0; i < e.pushes; i++ {
		e.use("__we_root_pop")
		e.inst("call void @__we_root_pop()")
	}
}

// emitReturn emits one return at any depth (T2 deep returns, chapter 3):
// the value operand computes first — it may read bindings the exit is
// about to leave behind — then every open scope pierces innermost-first
// (chapter 18:231: a return through a scope discharges its remaining
// handles), the deferred blocks drain in reverse registration order, the
// body's root pushes pop, and the ret closes the block. The block is
// marked diverged so the enclosing structure guards its joins and the
// statements after the return never emit.
func (e *emitter) emitReturn(r *ast.Return) *NotImplemented {
	if e.exit == nil || e.inExit {
		return e.bnd() // outside a body, or inside another exit's emission
	}
	e.inExit = true
	defer func() { e.inExit = false }()
	switch e.exit.kind {
	case exitTest:
		// The test context is valueless, like its tail.
		if r.HasValue {
			return e.bnd()
		}
		e.unwind(0)
		if ni := e.drainDefers(); ni != nil {
			return ni
		}
		e.popRoots()
		e.inst("ret void")
		e.diverged = true
		return nil
	case exitMain:
		// The entry's Result face is the tail's own dispatch: Ok unit
		// completes, every other shape stops.
		if !r.HasValue {
			return bndMain()
		}
		e.unwind(0)
		if ni := e.emitTail(r.Value); ni != nil {
			return ni
		}
		e.diverged = true
		return nil
	case exitTask:
		// The thunk's value: a valueless return answers the same default
		// the value-less tail does.
		val := "0"
		if r.HasValue {
			op, isF, ni := e.emitNumExpr(r.Value)
			if ni != nil || isF {
				return bndTask()
			}
			val = op
		}
		e.unwind(0)
		if ni := e.drainDefers(); ni != nil {
			return ni
		}
		e.inst("ret i64 " + val)
		e.diverged = true
		return nil
	default: // exitFn
		ret, ni := e.fnRetOperand(e.exit.abi, r.Value, r.HasValue)
		if ni != nil {
			return ni
		}
		e.unwind(0)
		if ni := e.drainDefers(); ni != nil {
			return ni
		}
		e.popRoots()
		e.inst("ret " + ret)
		e.diverged = true
		return nil
	}
}

// emitLoop emits `loop block` (chapter 3): no condition — the body
// branches back to itself, break is the only exit, continue restarts the
// body. The frame's two targets are the same label, which is exactly the
// form's semantics.
func (e *emitter) emitLoop(s *ast.Loop) *NotImplemented {
	n := e.blocks
	e.blocks++
	body := fmt.Sprintf("lpbody%d", n)
	exit := fmt.Sprintf("lpexit%d", n)
	e.inst(fmt.Sprintf("br label %%%s", body))
	e.label(body)
	if ni := e.emitLoopBody(s.Body.Items, loopFrame{brk: exit, cont: body, depth: e.nest}); ni != nil {
		return ni
	}
	if !e.diverged {
		e.inst(fmt.Sprintf("br label %%%s", body))
	}
	e.label(exit)
	return nil
}

// emitFor emits `for pat in iter` — chapter 5's iteration clause under
// chapter 11's protocol: the source expression evaluates once, the
// iterator is built once, and iteration runs until exhaustion. The Range
// source emits as a counted loop rather than the materialized array
// design D6 first named; the two are observationally identical for a
// Range (see for_test.go's header for the argument), and the counted loop
// needs neither the collection carrier nor a heap. The String source
// walks its code points (T4-3). The List source walks its carrier's
// snapshot (T7-2); every other source — a Set, a Map, a user impl, a bare
// iterator — stops at this build's body boundary.
func (e *emitter) emitFor(s *ast.ForStmt) *NotImplemented {
	if rng, ok := s.Iter.(*ast.Binary); ok && rng.Op == ".." {
		return e.emitForRange(s, rng)
	}
	if e.valueKind(s.Iter) == skStr {
		return e.emitForString(s)
	}
	if _, ok := e.listFaceOf(s.Iter); ok {
		return e.emitForList(s)
	}
	return e.bnd()
}

// emitForRange emits the counted Range loop: both bounds evaluate here, in
// source order, before the head block opens, so a body that assigns
// through a name the bound read cannot change the count; the counter
// itself lives in a slot reserved for the whole body and is not nameable
// from source, so nothing the body does can disturb the sequence.
func (e *emitter) emitForRange(s *ast.ForStmt, rng *ast.Binary) *NotImplemented {
	lo, lof, ni := e.emitNumExpr(rng.L)
	if ni != nil {
		return ni
	}
	hi, hif, ni := e.emitNumExpr(rng.R)
	if ni != nil {
		return ni
	}
	if lof || hif {
		return e.bnd() // E0902 keeps a Range over the integer types
	}
	n := e.blocks
	e.blocks++
	head := fmt.Sprintf("forhead%d", n)
	body := fmt.Sprintf("forbody%d", n)
	step := fmt.Sprintf("forcont%d", n)
	exit := fmt.Sprintf("forexit%d", n)
	counter := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 %s, ptr %s", lo, counter))
	e.inst(fmt.Sprintf("br label %%%s", head))
	e.label(head)
	// The head reads the counter and tests it against the end bound's
	// register: the register is why a bound the body reassigns cannot cut
	// the sequence short.
	cur := e.loadNum(counter, false)
	cmp := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp slt i64 %s, %s", cmp, cur, hi))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", cmp, body, exit))
	e.label(body)
	// The loop variable is bound afresh each pass and dies with the loop:
	// its own frame, so an assignment in the body writes the binding's
	// slot (when the body assigns it) while the step still reads the
	// counter's.
	e.pushEnv()
	defer e.popEnv()
	if ni := e.bindForPattern(s.Pat, cur, skI64); ni != nil {
		return ni
	}
	if ni := e.emitLoopBody(s.Body.Items, loopFrame{brk: exit, cont: step, depth: e.nest}); ni != nil {
		return ni
	}
	if !e.diverged {
		e.inst(fmt.Sprintf("br label %%%s", step))
	}
	// The step is its own block because continue lands on it: a continue
	// that skipped the step would spin on one element forever. It re-reads
	// the counter rather than reusing the head's register — the slot is
	// the loop's state, the head's load is one pass's view of it.
	e.label(step)
	next := e.emitStep(e.loadNum(counter, false))
	e.inst(fmt.Sprintf("store i64 %s, ptr %s", next, counter))
	e.inst(fmt.Sprintf("br label %%%s", head))
	e.label(exit)
	return nil
}

// emitForString emits `for c in s` over a String source: the value is read
// once — its two words stay registers for the whole walk — and the
// chapter 17 access pair drives the iteration. runeCount fixes the count of
// code points ahead of the head, and each pass takes one code point by
// index, so a multi-byte value walks once per code point rather than once
// per byte; an empty value's count is zero, so the body never runs. The
// index lives in a slot reserved for the whole walk (the same shape the
// Range loop's counter takes), so the element binding is fresh each pass
// and the source is never re-evaluated. A String's bytes are not gc
// objects (design D3 as corrected at T4), so the pair needs no root.
func (e *emitter) emitForString(s *ast.ForStmt) *NotImplemented {
	p, l, ni := e.emitStringExpr(s.Iter)
	if ni != nil {
		return ni
	}
	n := e.blocks
	e.blocks++
	head := fmt.Sprintf("forhead%d", n)
	body := fmt.Sprintf("forbody%d", n)
	step := fmt.Sprintf("forcont%d", n)
	exit := fmt.Sprintf("forexit%d", n)
	e.use("__we_str_runecount")
	e.use("__we_str_charat")
	count := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_str_runecount(ptr %s, i64 %s)", count, p, l))
	counter := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 0, ptr %s", counter))
	e.inst(fmt.Sprintf("br label %%%s", head))
	e.label(head)
	cur := e.loadNum(counter, false)
	cmp := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp slt i64 %s, %%%s", cmp, cur, count))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", cmp, body, exit))
	e.label(body)
	// The element call runs here, in the body, on this pass's index: the
	// walk reads one code point per pass and the binding names it.
	ch := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_str_charat(ptr %s, i64 %s, i64 %s)", ch, p, l, cur))
	e.pushEnv()
	defer e.popEnv()
	if ni := e.bindForPattern(s.Pat, "%"+ch, skRune); ni != nil {
		return ni
	}
	if ni := e.emitLoopBody(s.Body.Items, loopFrame{brk: exit, cont: step, depth: e.nest}); ni != nil {
		return ni
	}
	if !e.diverged {
		e.inst(fmt.Sprintf("br label %%%s", step))
	}
	// The step is its own block because continue lands on it: a continue
	// that skipped the step would spin on one code point forever.
	e.label(step)
	next := e.emitStep(e.loadNum(counter, false))
	e.inst(fmt.Sprintf("store i64 %s, ptr %s", next, counter))
	e.inst(fmt.Sprintf("br label %%%s", head))
	e.label(exit)
	return nil
}

// emitForList emits `for pat in <list>`: chapter 17's snapshot iteration
// (design D6). The source is read once — the carrier pointer, from a
// binding's environment face or from a literal built right here — and its
// snapshot is taken before the head opens, so the sequence the walk sees
// is the one fixed at the call: an element added after it, through any
// alias, is not seen. The snapshot is a gc object and is rooted for the
// whole walk. The index lives in a slot reserved for the loop (the same
// shape the Range and String walks take), so the element binding is fresh
// each pass and the body's own writes cannot disturb the sequence; the
// element call runs in the body on this pass's index, and the head
// pattern binds the word it answers.
func (e *emitter) emitForList(s *ast.ForStmt) *NotImplemented {
	src, face, ni := e.listSource(s.Iter)
	if ni != nil {
		return ni
	}
	e.use("__we_list_snap")
	e.use("__we_list_len")
	e.use("__we_list_get")
	e.use("__we_root_push")
	e.pushes++
	snap := e.value()
	e.inst(fmt.Sprintf("%%%s = call ptr @__we_list_snap(ptr %s)", snap, src))
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %%%s)", snap))
	n := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_list_len(ptr %%%s)", n, snap))
	blk := e.blocks
	e.blocks++
	head := fmt.Sprintf("forhead%d", blk)
	body := fmt.Sprintf("forbody%d", blk)
	step := fmt.Sprintf("forcont%d", blk)
	exit := fmt.Sprintf("forexit%d", blk)
	counter := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 0, ptr %s", counter))
	e.inst(fmt.Sprintf("br label %%%s", head))
	e.label(head)
	cur := e.loadNum(counter, false)
	cmp := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp slt i64 %s, %%%s", cmp, cur, n))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", cmp, body, exit))
	e.label(body)
	w := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_list_get(ptr %%%s, i64 %s)", w, snap, cur))
	e.pushEnv()
	defer e.popEnv()
	if ni := e.bindForListElem(s.Pat, "%"+w, face); ni != nil {
		return ni
	}
	if ni := e.emitLoopBody(s.Body.Items, loopFrame{brk: exit, cont: step, depth: e.nest}); ni != nil {
		return ni
	}
	if !e.diverged {
		e.inst(fmt.Sprintf("br label %%%s", step))
	}
	e.label(step)
	next := e.emitStep(e.loadNum(counter, false))
	e.inst(fmt.Sprintf("store i64 %s, ptr %s", next, counter))
	e.inst(fmt.Sprintf("br label %%%s", head))
	e.label(exit)
	return nil
}

// listSource yields a for statement's List source: the carrier pointer and
// the element face. A named binding reads its environment face — the
// binding site already rooted the carrier, so nothing is owed here — and a
// literal builds one at its own position, source order intact (the
// literal's elements run before the snapshot is taken, which is what makes
// the walk's sequence the one the source expression denotes).
func (e *emitter) listSource(x ast.Expr) (string, listElem, *NotImplemented) {
	switch v := x.(type) {
	case *ast.Ident:
		if b, ok := e.listEnv[v.Name]; ok {
			return b.reg, b.elem, nil
		}
		// A module-level List (T8-2B): the carrier lives in a global, so
		// the walk takes one load of it — the binding site's face, read
		// where the name is spent.
		if ts, ok := e.topName(v.Name); ok {
			if b, ok := e.topListRead(ts); ok {
				return b.reg, b.elem, nil
			}
		}
	case *ast.Member:
		// A qualified module-level List (T8-2B): `u1.xs` is the same read
		// one module over — the qualifier is no value, so the member is
		// the binding itself.
		if ts, ok := e.topMember(v); ok {
			if b, ok := e.topListRead(ts); ok {
				return b.reg, b.elem, nil
			}
		}
	case *ast.ListLit:
		return e.emitListLit(v, nil)
	}
	return "", listElem{}, e.bnd()
}

// listFaceOf classifies a for statement's source without emitting: the
// element face where the source is a List binding or a list literal, false
// for every other iterable — the dispatch a for statement needs before it
// picks a walk.
func (e *emitter) listFaceOf(x ast.Expr) (listElem, bool) {
	switch v := x.(type) {
	case *ast.Ident:
		if b, ok := e.listEnv[v.Name]; ok {
			return b.elem, true
		}
		// A module-level List (T8-2B) classifies without emitting: the
		// slot carries the element face pass one fixed, exactly as the
		// local binding's does.
		if ts, ok := e.topName(v.Name); ok && ts.gc && ts.rec == "" {
			return ts.elem, true
		}
		return listElem{}, false
	case *ast.Member:
		if ts, ok := e.topMember(v); ok && ts.gc && ts.rec == "" {
			return ts.elem, true
		}
		return listElem{}, false
	case *ast.ListLit:
		return e.listElemFace(nil, v)
	}
	return listElem{}, false
}

// bindForListElem binds one pass's element — the one word the carrier
// answered — under the head pattern. A scalar face binds as any scalar
// does (the body assigns it, so it takes a slot; otherwise it keeps the
// register the element arrived in), a Float64 face converts the word's bit
// pattern back to the double it holds, and a gc face binds the record
// reference its handle names — copied when the record is a value-category
// one, chapter 8's rule for every binding of a value. A gc binding the
// body assigns has no slot to write (chapter 8's gc records bind by
// reference; the emitter's slot face for them is the B-track's), so the
// loop stops at the body boundary rather than dropping the write.
func (e *emitter) bindForListElem(pat ast.Pattern, word string, face listElem) *NotImplemented {
	switch p := pat.(type) {
	case *ast.PatWildcard:
		return nil
	case *ast.PatBinding:
		if p.Name == "_" {
			return nil
		}
		if face.gc {
			if e.assigned[p.Name] {
				return e.bnd()
			}
			reg := e.wordPtr(word)
			if r, ok := e.records[face.rec]; ok && r.Cat == "value" {
				reg = e.emitRecCopy(reg, face.rec)
			}
			e.gcEnv[p.Name] = gcBinding{rec: face.rec, reg: reg}
			return nil
		}
		op, isF := word, false
		if face.kind == skF64 {
			v := e.value()
			e.inst(fmt.Sprintf("%%%s = bitcast i64 %s to double", v, word))
			op, isF = "%"+v, true
		}
		if e.assigned[p.Name] {
			e.bindScalarSlot(p.Name, op, isF, face.kind)
			return nil
		}
		e.scalars[p.Name] = scalarSlot{operand: op, isFloat: isF, kind: face.kind}
		return nil
	default:
		return e.bnd()
	}
}

// bindForPattern binds one pass's element under the head pattern. `_`
// binds nothing. A name binds like any other scalar binding: the body
// assigns it, so it takes a slot; otherwise it stays the register the
// element arrived in. kind is the source's element domain — a Range yields
// Int64 counts, a String yields Runes (T4-3) — which the binding carries
// for the interpolation domain. Other head shapes (the tuple head of T2-c,
// the scope resource) are not this build's.
func (e *emitter) bindForPattern(pat ast.Pattern, op string, kind strKind) *NotImplemented {
	switch p := pat.(type) {
	case *ast.PatWildcard:
		return nil
	case *ast.PatBinding:
		if p.Name == "_" {
			return nil
		}
		if e.assigned[p.Name] {
			e.bindScalarSlot(p.Name, op, false, kind)
			return nil
		}
		e.scalars[p.Name] = scalarSlot{operand: op, kind: kind}
		return nil
	default:
		return e.bnd()
	}
}

// emitListLit emits one list literal (design D6): the carrier opens with
// the element count as its capacity, the elements run in source order —
// each stored as the one word its face occupies — and the pushed
// identity's register is what the value is. The pre-size is what makes
// the walk of the pushes simple: with capacity equal to the count no push
// can reach the growth branch, so the block never moves. The roots are
// pushed anyway for both ends of that chain — the creation and the final
// identity — because the rooting rule is what makes the literal safe
// under a collection triggered by any element expression that allocates,
// and it must not rest on an arithmetic coincidence a later widening
// could break.
//
// The element face comes from the annotation where the position carries
// one — the only source an empty literal has, chapter 17's E1501 — and
// from the first element's own form otherwise; the check stage has
// already held every later element to the first one's type (E0501), and
// the emission re-checks each against the face it fixed.
func (e *emitter) emitListLit(x *ast.ListLit, typ ast.TypeRef) (string, listElem, *NotImplemented) {
	face, ok := e.listElemFace(typ, x)
	if !ok {
		return "", listElem{}, e.bnd()
	}
	e.use("__we_list_new")
	e.use("__we_list_push")
	e.use("__we_root_push")
	e.pushes++
	traced := 0
	if face.gc {
		traced = 1
	}
	reg := "%" + e.value()
	e.inst(fmt.Sprintf("%s = call ptr @__we_list_new(i64 %d, i64 %d)", reg, len(x.Elems), traced))
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", reg))
	cur := reg
	for _, el := range x.Elems {
		w, ni := e.emitListElemValue(el, face)
		if ni != nil {
			return "", listElem{}, ni
		}
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call ptr @__we_list_push(ptr %s, i64 %s)", v, cur, w))
		cur = "%" + v
	}
	if len(x.Elems) > 0 {
		e.pushes++
		e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", cur))
	}
	return cur, face, nil
}

// emitListElemValue emits one element as the one word its face occupies: a
// scalar's value, a Float64's bit pattern, or a gc handle. The word is the
// same width whatever the face, which is the carrier's whole layout
// contract; a conversion the operator family cannot answer — a float where
// the face holds an integer, or the reverse — stops at the boundary
// rather than storing a reinterpretation of the wrong domain.
func (e *emitter) emitListElemValue(x ast.Expr, face listElem) (string, *NotImplemented) {
	if face.gc {
		reg, key, ni := e.emitRecordValue(x)
		if ni != nil {
			return "", ni
		}
		if key != face.rec {
			return "", e.bnd()
		}
		return e.ptrWord(reg), nil
	}
	op, isF, ni := e.emitNumExpr(x)
	if ni != nil {
		return "", ni
	}
	if face.kind == skF64 {
		if !isF {
			return "", e.bnd()
		}
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = bitcast double %s to i64", v, op))
		return "%" + v, nil
	}
	if isF {
		return "", e.bnd()
	}
	return op, nil
}

// listElemFace fixes a list literal's element face. The annotation names
// it where the literal's position carries one — the type is the only
// source an empty literal has — and the first element's own expression
// names it otherwise. A face the carrier cannot hold in one word, and an
// empty literal with no annotation to read (which the check stage already
// rejects, E1501), report false.
func (e *emitter) listElemFace(typ ast.TypeRef, x *ast.ListLit) (listElem, bool) {
	if n, ok := typ.(*ast.NamedType); ok && n.Qual == "" && n.Name == "List" && len(n.Args) == 1 {
		return e.elemFaceOfType(n.Args[0])
	}
	if len(x.Elems) == 0 {
		return listElem{}, false
	}
	return e.elemFaceOfExpr(x.Elems[0])
}

// elemFaceOfType reads one element type's face through the ABI classifier
// — which already erases newtype wrappers and resolves a record's
// module-qualified key — and keeps the families the one-word carrier can
// hold: the integer family, Bool, Rune, Float64 (as its bit pattern), and
// a record's handle. A String is two words, a sum two, a tuple several, a
// fn value a carrier of its own, and a nested collection a carrier whose
// own element face would have to be written down too; all of them report
// false.
func (e *emitter) elemFaceOfType(t ast.TypeRef) (listElem, bool) {
	t = e.derefNewtype(t)
	kind, key, ok := e.classType(t)
	if !ok {
		return listElem{}, false
	}
	switch kind {
	case abiI64:
		k := baseStrKind(baseTypeName(t))
		if k == skNone {
			return listElem{}, false
		}
		return listElem{kind: k}, true
	case abiDouble:
		return listElem{kind: skF64}, true
	case abiGc:
		return listElem{rec: key, gc: true}, true
	}
	return listElem{}, false
}

// elemFaceOfExpr reads one element expression's face where the expression's
// own form fixes it: its interpolation domain for a scalar, and its record
// for a gc value. Nothing is emitted — the walk runs before the carrier
// exists, and each element is emitted exactly once, later, from the face
// this fixed.
func (e *emitter) elemFaceOfExpr(x ast.Expr) (listElem, bool) {
	if k := e.valueKind(x); k != skNone {
		return listElem{kind: k}, true
	}
	if key, ok := e.recordKeyOf(x); ok {
		return listElem{rec: key, gc: true}, true
	}
	return listElem{}, false
}

// recordKeyOf statically names the record an expression denotes where its
// own form says so — a gc binding, a construction in this module or a
// qualified one that resolves. A form whose record only the emission could
// name (a call's result, a field read) reports false; nothing here emits.
func (e *emitter) recordKeyOf(x ast.Expr) (string, bool) {
	switch v := x.(type) {
	case *ast.Ident:
		g, ok := e.gcEnv[v.Name]
		return g.rec, ok
	case *ast.Construct:
		key := e.curKey
		if v.Qual != "" {
			key = e.resolveQual(v.Qual)
			if key == "" {
				return "", false
			}
		}
		rec, ok := e.records[key+"."+v.Name]
		if !ok || len(rec.TypeParams) != 0 || len(v.TypeArgs) != 0 {
			return "", false
		}
		return key + "." + rec.Name, true
	}
	return "", false
}

// --- T7-3: the six acute combinators (design D6) ---------------------------
//
// fold, reduce, count, any, all and find are methods on Iterator<T> whose
// bodies the standard library writes in We, and the check stage walks
// those bodies on every check. The emission face takes the other path
// design D6 chose: the call form is recognized and the loop the body
// describes is emitted directly — no std module fn body, no generic
// instantiation, and none of the library machinery this build does not
// have. The chapter text is the semantics either way; the conformance
// goldens pin the behaviour.
//
// The recognized form is `<list>.iterator().<name>(args)`. The receiver's
// face comes from a List binding or a literal — the same environment the
// for walk reads (T7-2) — and the callback is a function value built by
// the T6 machinery, so a closure literal and a bound fn value both work
// and captures freeze at the call site. The accumulator and the result
// stay in the word domain the combinator's own declaration supports; a
// face the sum pair cannot carry stops at the body boundary.

// acuteCombinator names the six eager combinators — the ones whose
// results are values, as against the five lazy ones (map/filter/take/
// skip/collect) whose Dyn<Iterator<U>> results belong to B1b's vtable
// face (design D6).
func acuteCombinator(name string) bool {
	switch name {
	case "fold", "reduce", "count", "any", "all", "find":
		return true
	}
	return false
}

// emitAcute recognizes and emits one combinator call. The bool says the
// form is this face at all — the name is one of the six AND the receiver
// is a List's own iterator — so a failure past that point is a boundary
// of this face rather than a fall-through to another one. A receiver
// that is not a List (a String's iterator, say) is not this face and
// leaves the call to the faces below, which stop it as they did before.
func (e *emitter) emitAcute(fn *ast.Member, call *ast.Call) (callResult, bool, *NotImplemented) {
	if !acuteCombinator(fn.Name) {
		return callResult{}, false, nil
	}
	it, ok := fn.Recv.(*ast.Call)
	if !ok {
		return callResult{}, false, nil
	}
	im, ok := it.Fn.(*ast.Member)
	if !ok || im.Name != "iterator" || len(it.Args) != 0 {
		return callResult{}, false, nil
	}
	if _, ok := e.listFaceOf(im.Recv); !ok {
		return callResult{}, false, nil
	}
	src, face, ni := e.listSource(im.Recv)
	if ni != nil {
		return callResult{}, true, ni
	}
	res, ni := e.emitAcuteLoop(fn.Name, src, face, call.Args)
	return res, true, ni
}

// listWalk is one combinator loop's live state: the carrier's snapshot,
// the length it was read with, the pass's index, and the four block names
// a pass moves through. word is this pass's element — the one word the
// carrier answered, before any face conversion.
type listWalk struct {
	snap, n, cur, word string
	counter            string
	head, body         string
	step, exit         string
}

// openListWalk emits the snapshot prologue and one pass's head: the copy,
// its root (the walk outlives the allocation the callback may make), the
// length, and the counter slot. from is the counter's initial value —
// reduce starts at one, its first element being the accumulator rather
// than a folded step. The body block is left open with the element word
// in hand.
func (e *emitter) openListWalk(src string, from int64) (listWalk, *NotImplemented) {
	e.use("__we_list_snap")
	e.use("__we_list_len")
	e.use("__we_list_get")
	e.use("__we_root_push")
	e.pushes++
	n := e.blocks
	e.blocks++
	w := listWalk{
		head: fmt.Sprintf("chead%d", n), body: fmt.Sprintf("cbody%d", n),
		step: fmt.Sprintf("cstep%d", n), exit: fmt.Sprintf("cexit%d", n),
	}
	sv := e.value()
	e.inst(fmt.Sprintf("%%%s = call ptr @__we_list_snap(ptr %s)", sv, src))
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %%%s)", sv))
	w.snap = "%" + sv
	lv := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_list_len(ptr %s)", lv, w.snap))
	w.n = "%" + lv
	w.counter = e.slot("i64")
	e.inst(fmt.Sprintf("store i64 %d, ptr %s", from, w.counter))
	e.inst(fmt.Sprintf("br label %%%s", w.head))
	e.label(w.head)
	w.cur = e.loadNum(w.counter, false)
	c := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp slt i64 %s, %s", c, w.cur, w.n))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", c, w.body, w.exit))
	e.label(w.body)
	ev := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_list_get(ptr %s, i64 %s)", ev, w.snap, w.cur))
	w.word = "%" + ev
	return w, nil
}

// closeListWalk ends the pass: the fall-through branch to the step block
// (skipped where the pass left a terminator of its own), the step itself,
// and the exit block the result reads in.
func (e *emitter) closeListWalk(w listWalk) {
	if !e.diverged {
		e.inst(fmt.Sprintf("br label %%%s", w.step))
	}
	e.label(w.step)
	next := e.emitStep(e.loadNum(w.counter, false))
	e.inst(fmt.Sprintf("store i64 %s, ptr %s", next, w.counter))
	e.inst(fmt.Sprintf("br label %%%s", w.head))
	e.label(w.exit)
}

// listElemWord converts one pass's element word into the operand a
// callback takes: a gc record's handle back to its pointer — copied where
// the record's category is a value, chapter 8's rule at every binding —
// and a Float64's bit pattern back to the double it holds. A word whose
// face the emitted loop never fixed as a scalar reports false.
func (e *emitter) listElemWord(face listElem, word string) (string, *NotImplemented) {
	if face.gc {
		reg := e.wordPtr(word)
		if r, ok := e.records[face.rec]; ok && r.Cat == "value" {
			reg = e.emitRecCopy(reg, face.rec)
		}
		return "ptr " + reg, nil
	}
	if face.kind == skF64 {
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = bitcast i64 %s to double", v, word))
		return "double %" + v, nil
	}
	if face.kind == skNone {
		return "", e.bnd()
	}
	return "i64 " + word, nil
}

// scalarWordFace reports whether one element's word is a value of the
// element's own type in the i64 domain — the only payload reduce and find
// can answer, a match arm binding the payload into exactly that domain. A
// gc record's word is a handle and a Float64's is a bit pattern: both are
// words, and neither is a value of the element's type, which is why
// neither family is in this set — listElemFace gives a gc element no kind
// at all, and a Float64 the one kind whose word is not its value.
func scalarWordFace(face listElem) bool {
	switch face.kind {
	case skI64, skU64, skBool, skRune:
		return true
	}
	return false
}

// faceAbiKind is the calling face one element's word crosses as.
func faceAbiKind(face listElem) fnAbiKind {
	switch {
	case face.gc:
		return abiGc
	case face.kind == skF64:
		return abiDouble
	}
	return abiI64
}

// faceParam is the callback parameter one element's face declares: the
// kind is the word's, the key is the record a gc handle names, and the
// base name is what the bare parameter would have been given, so a
// callback body that renders its parameter renders it as its own type.
func faceParam(face listElem) fnParamAbi {
	k := faceAbiKind(face)
	p := fnParamAbi{kind: k}
	switch k {
	case abiGc:
		p.key = face.rec
	case abiI64:
		p.typ = strKindName(face.kind)
	case abiDouble:
		p.typ = "Float64"
	}
	return p
}

// acuteCallback is the callback's signature, read off the combinator's
// own declaration (chapter 11): fold threads the accumulator's face
// through its parameter and its result, reduce takes and answers the
// element's, and a predicate takes the element's and answers Bool — which
// rides the i64 domain, exactly as a Bool parameter does in a declared fn.
// acc is the accumulator's kind, which only fold reads.
func acuteCallback(name string, face listElem, acc fnAbiKind) fnAbi {
	elem := faceParam(face)
	switch name {
	case "fold":
		return fnAbi{ret: acc, retTyp: abiTypOf(acc),
			params: []fnParamAbi{{kind: acc, typ: strKindName(accKindOf(acc))}, elem}}
	case "reduce":
		k := faceAbiKind(face)
		return fnAbi{ret: k, retTyp: abiTypOf(k), params: []fnParamAbi{elem, elem}}
	default:
		return fnAbi{ret: abiI64, retTyp: "i64", retName: "Bool", params: []fnParamAbi{elem}}
	}
}

// abiTypOf spells one calling kind as the define's result type, the way
// fitAbi spells a declared signature's — a signature built here has no
// declaration to read the spelling off.
func abiTypOf(k fnAbiKind) string {
	switch k {
	case abiVoid:
		return "void"
	case abiDouble:
		return "double"
	case abiStr:
		return "{ ptr, i64 }"
	case abiGc:
		return "ptr"
	case abiSum:
		return "{ i64, i64 }"
	}
	return "i64"
}

// accKindOf reads back the interpolation domain a fold accumulator's
// calling kind spells.
func accKindOf(k fnAbiKind) strKind {
	if k == abiDouble {
		return skF64
	}
	return skI64
}

// emitAcuteLoop dispatches the six. Every one of them shares the walk;
// what differs is the accumulator (fold, reduce), the short circuit (any,
// all, find), and the result (the sum pair for reduce and find, a scalar
// for the rest).
func (e *emitter) emitAcuteLoop(name, src string, face listElem, args []ast.Expr) (callResult, *NotImplemented) {
	want := map[string]int{"fold": 2, "reduce": 1, "count": 0, "any": 1, "all": 1, "find": 1}[name]
	if len(args) != want {
		return callResult{}, e.bnd()
	}
	switch name {
	case "count":
		return e.emitCount(src)
	case "fold":
		return e.emitFold(src, face, args)
	case "reduce":
		return e.emitReduce(src, face, args[0])
	case "any", "all":
		return e.emitQuantify(name, src, face, args[0])
	case "find":
		return e.emitFind(src, face, args[0])
	}
	return callResult{}, e.bnd()
}

// emitCount is `count()`: the walk counts its own passes. The element
// face is irrelevant to the answer — the count is the carrier's length,
// which the walk reads anyway — so it takes no face at all.
func (e *emitter) emitCount(src string) (callResult, *NotImplemented) {
	n := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 0, ptr %s", n))
	w, ni := e.openListWalk(src, 0)
	if ni != nil {
		return callResult{}, ni
	}
	cur := e.loadNum(n, false)
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = add i64 %s, 1", v, cur))
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %s", v, n))
	e.closeListWalk(w)
	return callResult{kind: ckI64, i64: e.loadNum(n, false), typeName: "Int64"}, nil
}

// emitFold is `fold(init, f)`: the accumulator starts at init and each
// pass replaces it with f(acc, element). The accumulator lives in a slot
// rather than an SSA value — it crosses the loop's back edge, so it needs
// an address, exactly as a loop-carried source binding does. Its face
// comes from init's own form; a gc init stops, the sum and tuple faces
// having no word here yet.
func (e *emitter) emitFold(src string, face listElem, args []ast.Expr) (callResult, *NotImplemented) {
	op, isF, ni := e.emitNumExpr(args[0])
	if ni != nil {
		return callResult{}, ni
	}
	accKind := abiI64
	slotTyp := "i64"
	if isF {
		accKind, slotTyp = abiDouble, "double"
	}
	acc := e.slot(slotTyp)
	e.inst(fmt.Sprintf("store %s %s, ptr %s", slotTyp, op, acc))
	fv, ni := e.emitFnArg(args[1], new(acuteCallback("fold", face, accKind)))
	if ni != nil {
		return callResult{}, ni
	}
	w, ni := e.openListWalk(src, 0)
	if ni != nil {
		return callResult{}, ni
	}
	elem, ni := e.listElemWord(face, w.word)
	if ni != nil {
		return callResult{}, ni
	}
	parts := e.fnParts(fv)
	cur := e.loadNum(acc, isF)
	r := e.value()
	e.inst(fmt.Sprintf("%%%s = call %s %s(ptr %s, %s, %s)",
		r, slotTyp, parts.fnptr, parts.env, slotTyp+" "+cur, elem))
	e.inst(fmt.Sprintf("store %s %%%s, ptr %s", slotTyp, r, acc))
	e.closeListWalk(w)
	return callResult{kind: ckI64, i64: e.loadNum(acc, isF), isFloat: isF, typeName: strKindName(accKindOf(accKind))}, nil
}

// emitReduce is `reduce(f)` over a non-empty carrier and None over an
// empty one: the first element is the accumulator, every later one is a
// folded step. The pair is the runtime's Option ABI — None is 0, Some is
// 1, as receive's own table has it. A Float64 or gc element stops: the
// payload word would be a bit pattern or a handle, and a match arm binds
// the payload into the scalar domain, so the loop would hand the body a
// reinterpretation of the wrong thing rather than a value.
func (e *emitter) emitReduce(src string, face listElem, arg ast.Expr) (callResult, *NotImplemented) {
	if !scalarWordFace(face) {
		return callResult{}, e.bnd()
	}
	tag := e.slot("i64")
	pay := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 0, ptr %s", tag))
	e.inst(fmt.Sprintf("store i64 0, ptr %s", pay))
	fv, ni := e.emitFnArg(arg, new(acuteCallback("reduce", face, 0)))
	if ni != nil {
		return callResult{}, ni
	}
	w, ni := e.openListWalk(src, 0)
	if ni != nil {
		return callResult{}, ni
	}
	n := e.blocks
	e.blocks++
	first, later := fmt.Sprintf("rfirst%d", n), fmt.Sprintf("rlater%d", n)
	c := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp eq i64 %s, 0", c, w.cur))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", c, first, later))
	e.label(first)
	e.inst(fmt.Sprintf("store i64 1, ptr %s", tag))
	e.inst(fmt.Sprintf("store i64 %s, ptr %s", w.word, pay))
	e.inst(fmt.Sprintf("br label %%%s", w.step))
	e.label(later)
	elem, ni := e.listElemWord(face, w.word)
	if ni != nil {
		return callResult{}, ni
	}
	parts := e.fnParts(fv)
	cur := e.loadNum(pay, false)
	r := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 %s(ptr %s, i64 %s, %s)", r, parts.fnptr, parts.env, cur, elem))
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %s", r, pay))
	e.closeListWalk(w)
	return callResult{kind: ckSum, sum: sumSlot{tag: tag, pay: pay, variants: []string{"None", "Some"}}}, nil
}

// emitQuantify is `any(f)` and `all(f)`: the predicate decides, and the
// first element that decides it ends the walk — the chapter's own body
// recurses on the remainder, which is the same answer without the work.
func (e *emitter) emitQuantify(name, src string, face listElem, arg ast.Expr) (callResult, *NotImplemented) {
	res := e.slot("i64")
	start, hitWhen := int64(0), int64(1)
	if name == "all" {
		start, hitWhen = 1, int64(0)
	}
	e.inst(fmt.Sprintf("store i64 %d, ptr %s", start, res))
	fv, ni := e.emitFnArg(arg, new(acuteCallback(name, face, 0)))
	if ni != nil {
		return callResult{}, ni
	}
	w, ni := e.openListWalk(src, 0)
	if ni != nil {
		return callResult{}, ni
	}
	elem, ni := e.listElemWord(face, w.word)
	if ni != nil {
		return callResult{}, ni
	}
	parts := e.fnParts(fv)
	p := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 %s(ptr %s, %s)", p, parts.fnptr, parts.env, elem))
	n := e.blocks
	e.blocks++
	hit, cont := fmt.Sprintf("qhit%d", n), fmt.Sprintf("qcont%d", n)
	c := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp eq i64 %%%s, %d", c, p, hitWhen))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", c, hit, cont))
	e.label(hit)
	e.inst(fmt.Sprintf("store i64 %d, ptr %s", 1-start, res))
	e.inst(fmt.Sprintf("br label %%%s", w.exit))
	e.label(cont)
	e.closeListWalk(w)
	return callResult{kind: ckI64, i64: e.loadNum(res, false), typeName: "Bool"}, nil
}

// emitFind is `find(f)`: Some(element) at the first hit, None when the
// carrier runs out — the predicate's answer is the whole test, so the
// match arm the chapter's body would take is the branch here. The same
// payload restriction reduce takes applies, for the same reason.
func (e *emitter) emitFind(src string, face listElem, arg ast.Expr) (callResult, *NotImplemented) {
	if !scalarWordFace(face) {
		return callResult{}, e.bnd()
	}
	tag := e.slot("i64")
	pay := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 0, ptr %s", tag))
	e.inst(fmt.Sprintf("store i64 0, ptr %s", pay))
	fv, ni := e.emitFnArg(arg, new(acuteCallback("find", face, 0)))
	if ni != nil {
		return callResult{}, ni
	}
	w, ni := e.openListWalk(src, 0)
	if ni != nil {
		return callResult{}, ni
	}
	elem, ni := e.listElemWord(face, w.word)
	if ni != nil {
		return callResult{}, ni
	}
	parts := e.fnParts(fv)
	p := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 %s(ptr %s, %s)", p, parts.fnptr, parts.env, elem))
	n := e.blocks
	e.blocks++
	hit, cont := fmt.Sprintf("fhit%d", n), fmt.Sprintf("fcont%d", n)
	c := e.value()
	e.inst(fmt.Sprintf("%%%s = icmp ne i64 %%%s, 0", c, p))
	e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", c, hit, cont))
	e.label(hit)
	e.inst(fmt.Sprintf("store i64 1, ptr %s", tag))
	e.inst(fmt.Sprintf("store i64 %s, ptr %s", w.word, pay))
	e.inst(fmt.Sprintf("br label %%%s", w.exit))
	e.label(cont)
	e.closeListWalk(w)
	return callResult{kind: ckSum, sum: sumSlot{tag: tag, pay: pay, variants: []string{"None", "Some"}}}, nil
}

// emitStep advances the loop counter by one. The add is unchecked because
// the step is provably reachable only below the end bound: the body — and
// so the step block, whether by fallthrough or by continue — is entered
// only on the head's `counter < hi` test, and hi is an i64, so the counter
// at the step is at most MaxInt64-1 and the sum cannot wrap. The counter
// is the loop's own state, not a source-visible value, so there is no
// chapter 7 faces to report through either.
func (e *emitter) emitStep(cur string) string {
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = add i64 %s, 1", v, cur))
	return "%" + v
}

// --- T8-1/T8-2 module-level bindings (design D7, chapter 15 R5) -----------

// topLetRef is one collected module-level binding: the module it belongs
// to, and the declaration whose initializer the module's init runs.
type topLetRef struct {
	key  string
	decl *ast.TopLet
}

// topSlot is one module-level binding's storage face: the qualified symbol
// it is spelled and read under, the domain it holds, and which of the
// three storage shapes it owns — one scalar global (the default: an i64 or
// a double), the pair of globals a String's (ptr, len) takes, or one
// global holding a collectable handle. `str` and `gc` are exclusive.
// `rec` and `elem` carry what a gc handle needs on the read side: the
// record key it names (empty for a List) and a List's element face.
type topSlot struct {
	sym     string
	kind    strKind
	isFloat bool
	str     bool
	gc      bool
	rec     string
	elem    listElem
}

// collectTopLet takes one module-level binding into the init plan. The
// domain is fixed here, before anything emits, because the globals' LLVM
// types have to be known to every body that reads them — and the reading
// bodies are emitted in an order the binding's own module need not precede.
// The classification is the emitter's ordinary static one over the
// initializer, and a binding whose annotation names a base type takes the
// annotation's domain instead, exactly as a `let` statement's does.
//
// The face covers the scalar word, the String pair and the collectable
// handle. The three are three storage shapes, not three domains: a String
// needs no gc root at all — design D3's storage ruling puts its bytes in a
// private constant or a malloc'd buffer, neither of them a collectable
// block, so a global holding one is a global the collector must NOT be
// told about — while a gc record or a List is exactly the case the root
// table exists for (T8-2B): a handle in a global no root scan can see is a
// use-after-free waiting for the first collection.
func (e *emitter) collectTopLet(key string, d *ast.TopLet) *NotImplemented {
	e.topLets = append(e.topLets, topLetRef{key: key, decl: d})
	name := d.Binding.Name
	if name == "_" {
		// The discard binds nothing — but it still evaluates (chapter 6),
		// so the module keeps its init and the initializer runs there.
		return nil
	}
	sym := key + "." + name
	if rec, elem, ok := e.topGcShape(d); ok {
		e.topSlots[sym] = topSlot{sym: sym, gc: true, rec: rec, elem: elem}
		e.topGlobals = append(e.topGlobals, fmt.Sprintf("@%s = internal global ptr null", sym))
		e.topRoots = append(e.topRoots, sym)
		return nil
	}
	kind := baseStrKind(baseTypeName(d.Binding.Typ))
	if kind == skNone {
		kind = e.valueKind(d.Binding.Init)
	}
	if kind == skNone {
		return &NotImplemented{What: bndTopLets}
	}
	e.topSlots[sym] = topSlot{
		sym:     sym,
		kind:    kind,
		isFloat: kind == skF64,
		str:     kind == skStr,
	}
	if kind == skStr {
		// The pair takes one global per word: two loads are the whole read
		// face, the same shape a scalar binding's single load has, and no
		// aggregate unpacking stands between a binding and its value.
		e.topGlobals = append(e.topGlobals,
			fmt.Sprintf("@%s.p = internal global ptr null", sym),
			fmt.Sprintf("@%s.len = internal global i64 0", sym))
		return nil
	}
	typ := "i64"
	zero := "0"
	if kind == skF64 {
		typ, zero = "double", "0.0"
	}
	e.topGlobals = append(e.topGlobals, fmt.Sprintf("@%s = internal global %s %s", sym, typ, zero))
	return nil
}

// topGcShape decides whether a module-level binding's value is a
// collectable handle, and which one: the record key a gc record names
// (elem empty), or a List's element face (rec empty). Emit-free, like
// every other pass-one classification: the answer rides the binding's
// slot so a body emitted in another module — before its owner's init —
// reads the handle at the face pass one fixed.
//
// The annotation answers first when it names a record or a List: a
// binding's declared type is the front end's own answer, and it is the
// only source for an initializer whose value form says nothing, such as a
// call. Otherwise the initializer's form decides — a construction, a list
// literal, or a read of a binding already classified this way — which is
// the same two-source rule the scalar and String faces take.
//
// A value record is NOT this face. Chapter 8 gives every binding of one
// its own object, so its global would hold a copy the initializer made
// rather than a handle into a collectable block, and that copy needs a
// storage story of its own; the binding stops.
func (e *emitter) topGcShape(d *ast.TopLet) (rec string, elem listElem, ok bool) {
	if t := e.derefNewtype(d.Binding.Typ); t != nil {
		if n, isN := t.(*ast.NamedType); isN && n.Qual == "" && n.Name == "List" && len(n.Args) == 1 {
			if face, isList := e.elemFaceOfType(n.Args[0]); isList {
				return "", face, true
			}
		}
		if key, isRec := e.recordKeyOfType(t); isRec && !e.valueRecord(key) {
			return key, listElem{}, true
		}
	}
	switch v := d.Binding.Init.(type) {
	case *ast.Construct:
		if key, isRec := e.recordKeyOf(v); isRec && !e.valueRecord(key) {
			return key, listElem{}, true
		}
	case *ast.ListLit:
		if face, isList := e.listElemFace(d.Binding.Typ, v); isList {
			return "", face, true
		}
	case *ast.Ident:
		if ts, isTop := e.topName(v.Name); isTop && ts.gc {
			return ts.rec, ts.elem, true
		}
	case *ast.Member:
		if ts, isTop := e.topMember(v); isTop && ts.gc {
			return ts.rec, ts.elem, true
		}
	}
	return "", listElem{}, false
}

// valueRecord reports whether key names a chapter 8 value-category record
// — the category that copies on every binding.
func (e *emitter) valueRecord(key string) bool {
	r, ok := e.records[key]
	return ok && r.Cat == "value"
}

// recordKeyOfType resolves a type annotation that names a record declared
// in the walked module (or in one its imports name), the non-generic
// spelling only — the same rule recordKeyOf applies to a construction.
func (e *emitter) recordKeyOfType(t ast.TypeRef) (string, bool) {
	n, ok := t.(*ast.NamedType)
	if !ok || len(n.Args) != 0 {
		return "", false
	}
	key := e.curKey
	if n.Qual != "" {
		key = e.resolveQual(n.Qual)
		if key == "" {
			return "", false
		}
	}
	if _, ok := e.records[key+"."+n.Name]; !ok {
		return "", false
	}
	return key + "." + n.Name, true
}

// topName resolves a bare name to the module-level binding the walked
// module holds under it. A local of the same name wins everywhere this is
// consulted, the shadowing chapter 6 gives every inner scope.
func (e *emitter) topName(name string) (topSlot, bool) {
	ts, ok := e.topSlots[e.curKey+"."+name]
	return ts, ok
}

// topMember resolves a qualified member expression — `util.base` — to the
// module-level binding it names. The qualifier resolves through the walked
// module's imports first and by module key second, exactly as a qualified
// call's does, and a qualifier that is a local name is no module at all.
func (e *emitter) topMember(m *ast.Member) (topSlot, bool) {
	id, ok := m.Recv.(*ast.Ident)
	if !ok || e.isLocalName(id.Name) {
		return topSlot{}, false
	}
	k := e.resolveQual(id.Name)
	if k == "" {
		return topSlot{}, false
	}
	ts, ok := e.topSlots[k+"."+m.Name]
	return ts, ok
}

// topRead loads one module-level binding's value: the global its
// initializer stored, read back in the domain the binding fixed. The read
// face is the storage face — the binding's own globals, no copy — so a fn
// body, a later initializer of the same module and a reading module all see
// the value the init wrote, and nothing can drift between them.
func (e *emitter) topRead(ts topSlot) callResult {
	if ts.str {
		p := e.value()
		e.inst(fmt.Sprintf("%%%s = load ptr, ptr @%s.p", p, ts.sym))
		l := e.value()
		e.inst(fmt.Sprintf("%%%s = load i64, ptr @%s.len", l, ts.sym))
		return callResult{kind: ckStr, strBind: strBinding{dataOp: "%" + p, lenOp: "%" + l}}
	}
	if ts.gc {
		// A collectable handle: the global holds the pointer itself, so the
		// read is one load — the face every gc binding's read takes, and
		// the handle every later walk starts from.
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = load ptr, ptr @%s", v, ts.sym))
		return callResult{kind: ckGc, gcReg: "%" + v, recKey: ts.rec}
	}
	typ := "i64"
	if ts.isFloat {
		typ = "double"
	}
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = load %s, ptr @%s", v, typ, ts.sym))
	return callResult{kind: ckI64, i64: "%" + v, isFloat: ts.isFloat, typeName: strKindName(ts.kind)}
}

// bindTopRead binds one module-level binding's value under a new name: the
// global is read where the name is bound, and the new name holds what it
// read — a copy of the word or of the String pair, the record handle every
// gc binding shares, or a List's carrier. The distinction that needs the
// extra step is the List: a List binds by sharing its carrier (chapter
// 17's gc category), and no callResult face spells a carrier, so the
// binding site takes listEnv's face directly. Every other shape is the
// ordinary bindResult.
func (e *emitter) bindTopRead(name string, ts topSlot) *NotImplemented {
	if ts.gc && ts.rec == "" {
		lb, ok := e.topListRead(ts)
		if !ok {
			return e.bnd()
		}
		if name != "_" {
			e.listEnv[name] = lb
		}
		return nil
	}
	return e.bindResult(name, e.topRead(ts))
}

// topListRead reads one module-level List binding's carrier. A List is a
// gc handle, but not the ckGc face: nothing downstream wants a bare
// pointer from it — a list is walked, not field-read — so the binding site
// takes the carrier and the element face together, which is exactly what
// listEnv holds for a local one.
func (e *emitter) topListRead(ts topSlot) (listBinding, bool) {
	if !ts.gc || ts.rec != "" {
		return listBinding{}, false
	}
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = load ptr, ptr @%s", v, ts.sym))
	return listBinding{reg: "%" + v, elem: ts.elem}, true
}

// emitTopRootRegistrations hands the collector the address of every
// module-level global that holds a collectable handle (T8-2B, design D7).
// They are the entry's first statements, ahead of the inits: the table has
// to know a slot before an initializer can put a handle in it. What is
// registered is the address rather than the handle — the slots are zeroed
// statics, so a collection between here and the first store reads NULL and
// skips them, and a store after it needs no barrier because every
// collection reads the slot anew.
func (e *emitter) emitTopRootRegistrations() {
	if len(e.topRoots) == 0 {
		return
	}
	e.use("__we_gc_root_global")
	for _, sym := range e.topRoots {
		e.inst(fmt.Sprintf("call void @__we_gc_root_global(ptr @%s)", sym))
	}
}

// emitInitCalls opens the entry body with the module inits in load order.
// The calls ride the head of __we_main rather than startup.c's own body:
// the module set is a compile-time fact no fixed C file can name, and the
// observable shape chapter 15 fixes — every initializer runs once, in
// post-order, before the root main body — is exactly this. They allocate
// no value numbers and emit no terminator, so a program with no top-level
// binding passes through here with its entry body byte-identical.
func (e *emitter) emitInitCalls() {
	for _, ref := range e.topLets {
		if e.initEmitted[ref.key] {
			continue
		}
		e.initEmitted[ref.key] = true
		e.inst(fmt.Sprintf("call void @%s.init()", ref.key))
	}
}

// emitInitDefine emits one module's `@<key>.init`: the module's
// top-level initializers in source order, under the same body protocol a
// fn define uses — its own environments and its own value buffer — with
// the entry's exit site, so a panic or a trap inside an initializer takes
// the task-fail tail and aborts the process (chapter 15 R5: there is no
// capture boundary before main).
func (e *emitter) emitInitDefine(key string, lets []*ast.TopLet) *NotImplemented {
	e.enterModule(key)
	savedCtx, savedBody := e.ctx, e.body
	savedCur := e.curBlock
	savedAllocas, savedAssigned := e.allocas, e.assigned
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedFns := e.fnEnv
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedList := e.listEnv
	savedTup, savedNt := e.tupEnv, e.ntEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	savedFrames := e.frames
	savedDiverged := e.diverged
	savedExit, savedInExit := e.exit, e.inExit
	savedLoops, savedScopes := e.loopFrames, e.scopeLive
	savedRes, savedNest := e.resFrames, e.nest
	restore := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.fnEnv = savedFns
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.listEnv = savedList
		e.tupEnv, e.ntEnv = savedTup, savedNt
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
		e.frames = savedFrames
		e.diverged, e.curBlock = savedDiverged, savedCur
		e.exit, e.inExit = savedExit, savedInExit
		e.loopFrames, e.scopeLive = savedLoops, savedScopes
		e.resFrames, e.nest = savedRes, savedNest
		e.allocas, e.assigned = savedAllocas, savedAssigned
	}
	e.ctx = ctxMain
	e.beginBody()
	e.allocas = nil
	e.assigned = make(map[string]bool)
	e.diverged = false
	e.scalars = make(map[string]scalarSlot)
	e.fnEnv = make(map[string]fnValue)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.listEnv = make(map[string]listBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.tupEnv = make(map[string]tupBinding)
	e.ntEnv = make(map[string]string)
	e.defers = nil
	e.caps = nil
	e.pushes = 0
	e.exit = &exitSite{kind: exitMain}
	e.loopFrames, e.scopeLive, e.inExit = nil, nil, false
	e.resFrames, e.nest = nil, 0
	for _, d := range lets {
		if ni := e.emitTopLetInit(key, d); ni != nil {
			restore()
			return ni
		}
	}
	if !e.diverged {
		// The init body is a body (T8-2B-0 put every body's exit to work),
		// and at this point in the milestone it is the only one whose pushes
		// the collector would otherwise never stop seeing: an initializer
		// that binds a collectable handle roots it while it evaluates, and
		// the store into the registered global is what makes that root
		// redundant — the global root table marks the handle from then on.
		// Leaving them pushed would also hide the table's own effect, which
		// is how this discharge was found.
		e.popRoots()
		e.inst("ret void")
	}
	body := e.bodyText()
	restore()
	e.fnsDone = append(e.fnsDone, "define void @"+key+".init() {\nentry:\n"+body+"}\n")
	return nil
}

// emitTopLetInit emits one binding's initializer and stores its value into
// the binding's globals. The initializer binds under its own name through
// the ordinary `let` path — so every value face that path accepts is
// accepted here — and the face it left is then re-read from the environment
// and stored; the name is dropped from the environment afterwards, because
// the globals, not the SSA operands, are the binding's storage and every
// later read must take the same loads. Both storage shapes hold a face the
// environment names: a scalar word, or a String's pair.
func (e *emitter) emitTopLetInit(key string, d *ast.TopLet) *NotImplemented {
	b := &d.Binding
	ts, isTop := e.topSlots[key+"."+b.Name]
	if b.Name == "_" {
		// The discard evaluates and binds nothing.
		return e.emitLetBinding(b)
	}
	if !isTop {
		return e.bnd() // unreachable: an unclassified binding stopped in pass one
	}
	if ni := e.emitLetBinding(b); ni != nil {
		return ni
	}
	if ts.gc {
		// The initializer bound the handle through the ordinary paths — a
		// gc binding in gcEnv, a List in listEnv — and the store takes it
		// from there. The name is dropped afterwards for the same reason
		// the other two shapes drop it: the global is the binding's
		// storage, and every later read must take the load rather than a
		// register a collection cannot see.
		if ts.rec == "" {
			lb, ok := e.listEnv[b.Name]
			if !ok {
				return e.bnd()
			}
			delete(e.listEnv, b.Name)
			e.inst(fmt.Sprintf("store ptr %s, ptr @%s", lb.reg, ts.sym))
			return nil
		}
		g, ok := e.gcEnv[b.Name]
		if !ok {
			return e.bnd()
		}
		delete(e.gcEnv, b.Name)
		e.inst(fmt.Sprintf("store ptr %s, ptr @%s", g.reg, ts.sym))
		return nil
	}
	if ts.str {
		bind, ok := e.strEnv[b.Name]
		if !ok {
			// The initializer emitted a face that is not a String — the
			// classification that admitted this binding named one, so the
			// two disagree and the honest answer is the boundary, not a
			// store of whatever the other face happened to be.
			return e.bnd()
		}
		delete(e.strEnv, b.Name)
		p, l := bind.dataOp, bind.lenOp
		if p == "" {
			// A literal's bytes are still decoded; the store is a use, so
			// the constant pool entry is minted here (the M8 discipline).
			p, l = e.intern(bind.data), strconv.Itoa(bind.length)
		}
		e.inst(fmt.Sprintf("store ptr %s, ptr @%s.p", p, ts.sym))
		e.inst(fmt.Sprintf("store i64 %s, ptr @%s.len", l, ts.sym))
		return nil
	}
	slot, ok := e.scalars[b.Name]
	if !ok {
		// The initializer emitted a face that is not a scalar word — the
		// classification that admitted this binding named a scalar, so the
		// two disagree and the honest answer is the boundary, not a store
		// of whatever the other face happened to be.
		return e.bnd()
	}
	delete(e.scalars, b.Name)
	op := slot.operand
	if slot.alloca != "" {
		op = e.loadNum(slot.alloca, slot.isFloat)
	}
	if slot.isFloat != ts.isFloat {
		return e.bnd()
	}
	typ := "i64"
	if ts.isFloat {
		typ = "double"
	}
	e.inst(fmt.Sprintf("store %s %s, ptr @%s", typ, op, ts.sym))
	return nil
}

// emitTopLetInits emits every collected module's init define, in load
// order, and returns the module groups the entry's calls walk. The defines
// are emitted after the entry body so the entry's value numbering still
// starts at v0 (the widening's byte-identical guarantee), and they land in
// the fn group ahead of __we_main.
func (e *emitter) emitTopLetInits() *NotImplemented {
	byKey := map[string][]*ast.TopLet{}
	var order []string
	for _, ref := range e.topLets {
		if _, seen := byKey[ref.key]; !seen {
			order = append(order, ref.key)
		}
		byKey[ref.key] = append(byKey[ref.key], ref.decl)
	}
	for _, k := range order {
		if ni := e.emitInitDefine(k, byKey[k]); ni != nil {
			return ni
		}
	}
	return nil
}

// emitMatch emits the two-slot sum match: load the discriminant, test
// each variant arm in order (the variant table maps names to the
// runtime's return codes), the wildcard arm as the default, and every
// arm joins one continuation. Payload bindings (Some(x)) load the
// payload word into the scalar domain.
func (e *emitter) emitMatch(s *ast.Match, vf *valueForm) *NotImplemented {
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
			// close), so the body emits here without reopening it. A
			// false guard there leaves the match without a value, which
			// only the join can carry.
			if ni := e.emitMatchArm(arm, slot, join, vf); ni != nil {
				return ni
			}
			if !e.diverged {
				e.inst(fmt.Sprintf("br label %%%s", join))
			}
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
		if ni := e.emitMatchArm(arm, slot, next, vf); ni != nil {
			return ni
		}
		if !e.diverged {
			e.inst(fmt.Sprintf("br label %%%s", join))
		}
		e.label(next)
	}
	if !fired {
		// typecheck exhausted; defensive — the final variant arm's
		// fall-through block (just labeled `next`) is open, so this
		// branch is unconditionally safe.
		e.inst(fmt.Sprintf("br label %%%s", join))
	}
	e.label(join)
	return nil
}

// emitMatchArm emits one arm; a PatVariant payload binding loads the
// payload word first. A guarded arm then evaluates its condition — only
// now, after the pattern matched (chapter 4's laziness), and inside the
// frame that carries the pattern's bindings — and a false condition
// falls through to the next arm's test. With a value form the body stores
// its value into the form's sink. The whole arm is one block scope — the
// payload binding and everything the body registers roll back when the
// arm closes, so no arm name is readable past the join (the follow-up #16
// leak this frame seals).
func (e *emitter) emitMatchArm(arm ast.MatchArm, slot sumSlot, guardFalseL string, vf *valueForm) *NotImplemented {
	e.pushEnv()
	defer e.popEnv()
	if pv, ok := arm.Pat.(*ast.PatVariant); ok && len(pv.Args) == 1 {
		if b, ok := pv.Args[0].(*ast.PatBinding); ok && b.Name != "_" {
			op := e.loadNum(slot.pay, false)
			// The payload's type is the variant's declaration (D4's
			// traversal): the arm binding carries no domain yet.
			if e.assigned[b.Name] {
				e.bindScalarSlot(b.Name, op, false, skNone)
			} else {
				e.scalars[b.Name] = scalarSlot{operand: op}
			}
		}
	}
	if arm.Guard != nil {
		if e.diverged {
			return nil
		}
		c, ni := e.condI64(arm.Guard)
		if ni != nil {
			return ni
		}
		bodyL := fmt.Sprintf("mbody%d", e.blocks)
		e.blocks++
		e.inst(fmt.Sprintf("br i1 %s, label %%%s, label %%%s", c, bodyL, guardFalseL))
		e.label(bodyL)
	}
	return e.emitMatchBody(arm.Body, vf)
}

// emitMatchBody emits one arm's body: a block body runs as an arm block
// (statements then tail), any other body is the arm's one expression.
// Without a value form the arm is a statement and the block's tail, if it
// carries one, is emitted as a statement like any other item.
func (e *emitter) emitMatchBody(body ast.Expr, vf *valueForm) *NotImplemented {
	blk, isBlock := body.(*ast.BlockExpr)
	if vf == nil {
		if !isBlock {
			return e.bnd() // a statement arm's body is a block in the M9b set
		}
		return e.emitBlockStmts(blk.Block.Items)
	}
	if isBlock {
		return e.emitArmBlock(blk.Block.Items, vf)
	}
	return e.storeValue(body, vf)
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
		if e.valueKind(x) == skStr {
			return true
		}
		// A member is the field-chain face whether or not the chain
		// resolves here: a chain that does not is its own stop.
		_, ok := x.(*ast.Member)
		return ok
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

// collectAssigned walks a body for the names its statements assign. The
// walk runs before anything emits: a name the body writes — however deep
// the write sits — needs an address from its binding on, so the binding
// site must already know. A nested body (a task body, a callback) is
// skipped: it binds its own environments, an assignment inside it cannot
// reach this body's names, and its own walk covers what it emits.
func collectAssigned(items []ast.Stmt, set map[string]bool) {
	var walkExpr func(ast.Expr)
	var walkStmt func(ast.Stmt)
	walkExpr = func(x ast.Expr) {
		switch v := x.(type) {
		case *ast.BlockExpr:
			for _, st := range v.Block.Items {
				walkStmt(st)
			}
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
				if a.Guard != nil {
					walkExpr(a.Guard)
				}
				walkExpr(a.Body)
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
		case *ast.TaskExpr, *ast.Closure:
			// A nested body's assignments belong to its own walk.
		case *ast.Unary:
			walkExpr(v.X)
		case *ast.Binary:
			walkExpr(v.L)
			walkExpr(v.R)
		case *ast.Call:
			walkExpr(v.Fn)
			for _, a := range v.Args {
				walkExpr(a)
			}
		case *ast.Member:
			walkExpr(v.Recv)
		case *ast.Prop:
			walkExpr(v.X)
		case *ast.Tuple:
			for _, el := range v.Elems {
				walkExpr(el)
			}
		case *ast.ListLit:
			for _, el := range v.Elems {
				walkExpr(el)
			}
		case *ast.Construct:
			walkExpr(v.Base)
			for _, f := range v.Fields {
				walkExpr(f.Value)
			}
		}
	}
	walkStmt = func(st ast.Stmt) {
		switch v := st.(type) {
		case *ast.Assign:
			set[v.Name] = true
			walkExpr(v.Value)
		case *ast.Binding:
			walkExpr(v.Init)
		case *ast.Return:
			if v.Value != nil {
				walkExpr(v.Value)
			}
		case *ast.ExprStmt:
			walkExpr(v.Expr)
		case *ast.While:
			walkExpr(v.Cond)
			for _, s := range v.Body.Items {
				walkStmt(s)
			}
		case *ast.Loop:
			for _, s := range v.Body.Items {
				walkStmt(s)
			}
		case *ast.ForStmt:
			walkExpr(v.Iter)
			for _, s := range v.Body.Items {
				walkStmt(s)
			}
		case *ast.ScopeRes:
			for _, b := range v.Binds {
				walkExpr(b.Val)
			}
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
}

// bindScalarSlot binds one numeric name the body assigns to a stack slot
// of the value's own width: the assignment stores through the name, so
// the name must live at an address. The width is the value's, decided
// here because a slot carries its type — the SSA face can afford to drop
// the flag (design D10-1's open defect), a slot cannot.
func (e *emitter) bindScalarSlot(name, op string, isF bool, kind strKind) {
	typ := "i64"
	if isF {
		typ = "double"
	}
	slot := e.slot(typ)
	if isF {
		e.inst(fmt.Sprintf("store double %s, ptr %s", op, slot))
	} else {
		e.inst(fmt.Sprintf("store i64 %s, ptr %s", op, slot))
	}
	e.scalars[name] = scalarSlot{alloca: slot, isFloat: isF, kind: kind}
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
	savedCur := e.curBlock
	savedAllocas, savedAssigned := e.allocas, e.assigned
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedFns := e.fnEnv
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedList := e.listEnv
	savedTup, savedNt := e.tupEnv, e.ntEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	savedFrames := e.frames
	savedDiverged := e.diverged
	savedExit, savedInExit := e.exit, e.inExit
	savedLoops, savedScopes := e.loopFrames, e.scopeLive
	savedRes, savedNest := e.resFrames, e.nest
	restore := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.fnEnv = savedFns
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.listEnv = savedList
		e.tupEnv, e.ntEnv = savedTup, savedNt
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
		e.frames = savedFrames
		e.diverged, e.curBlock = savedDiverged, savedCur
		e.exit, e.inExit = savedExit, savedInExit
		e.loopFrames, e.scopeLive = savedLoops, savedScopes
		e.resFrames, e.nest = savedRes, savedNest
		e.allocas, e.assigned = savedAllocas, savedAssigned
	}
	e.ctx = ctxTask
	e.beginBody()
	e.allocas = nil
	e.assigned = make(map[string]bool)
	e.diverged = false
	e.scalars = make(map[string]scalarSlot)
	e.fnEnv = make(map[string]fnValue)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.listEnv = make(map[string]listBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.tupEnv = make(map[string]tupBinding)
	e.ntEnv = make(map[string]string)
	e.defers = nil
	// The thunk is its own body: a return inside it answers i64, whatever
	// its depth, and neither a loop nor a scope of the enclosing body
	// crosses the boundary.
	e.exit = &exitSite{kind: exitTask}
	e.loopFrames, e.scopeLive, e.inExit = nil, nil, false
	e.resFrames, e.nest = nil, 0
	collectAssigned(t.Body.Items, e.assigned)
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
		if e.diverged {
			break // a Never call already terminated the task body
		}
		if ni := e.emitStmt(st); ni != nil {
			restore()
			return callResult{}, ni
		}
	}
	if e.diverged {
		// A diverged task body ends at its unreachable — no defers, no
		// value tail (the process the Never call ends takes the task
		// with it).
		body := e.bodyText()
		restore()
		e.thunks = append(e.thunks, fmt.Sprintf(
			"define internal i64 %s(ptr %%env) {\nentry:\n%s}\n", name, body))
	} else {
		// Deferred blocks invert at the tail, reverse registration order.
		if ni := e.drainDefers(); ni != nil {
			restore()
			return callResult{}, ni
		}
		body := e.bodyText()
		restore()
		e.thunks = append(e.thunks, fmt.Sprintf(
			"define internal i64 %s(ptr %%env) {\nentry:\n%s  ret i64 %s\n}\n",
			name, body, val))
	}

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
	items := s.Body.Items
	tail := ast.Expr(nil)
	if valueForm && len(items) > 0 {
		if es, ok := items[len(items)-1].(*ast.ExprStmt); ok {
			tail = es.Expr
			items = items[:len(items)-1]
		}
	}
	// The scope body is one block scope: bindings made inside (tasks,
	// scalars) roll back at leave, so nothing the body registered is
	// readable after it.
	e.pushEnv()
	e.scopeLive = append(e.scopeLive, liveScope{handle: "%" + sc, collect: s.CollectAll, depth: e.nest})
	e.nest++
	for _, st := range items {
		if ni := e.emitStmt(st); ni != nil {
			return callResult{}, ni
		}
	}
	// The tail is the scope's value and it belongs to the body: it runs
	// after every body statement and inside the body's own environment.
	// Reading it before the body — which is what the emitter used to do —
	// yields the value the body opened with, not the one it left (defect
	// D10-2: legal IR, wrong value).
	bodyVal := "0"
	if tail != nil && !e.diverged {
		res, ni := e.emitNumericValue(tail)
		if ni != nil {
			return callResult{}, ni
		}
		switch {
		case res.kind == ckVoid:
			// A valueless tail: the body ran for its effect, the payload
			// word stays zero (the tag carries the outcome).
		case res.kind == ckI64 && !res.isFloat:
			bodyVal = res.i64
		default:
			return callResult{}, e.bnd() // a payload the sum's word cannot carry
		}
	}
	e.popEnv()
	e.scopeLive = e.scopeLive[:len(e.scopeLive)-1]
	if e.diverged {
		// The body ended in its own terminator — a piercing break,
		// continue, or return already discharged this scope at its site
		// (cancel or collect leave), so nothing joins here and the
		// never-reached continuation is the enclosing context's.
		return callResult{kind: ckVoid}, nil
	}
	e.use("__we_scope_leave")
	to := e.value()
	e.inst(fmt.Sprintf("%%%s = call i64 @__we_scope_leave(ptr %%%s)", to, sc))
	if !valueForm {
		return callResult{kind: ckVoid}, nil
	}
	tagSlot := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 %%%s, ptr %s", to, tagSlot))
	paySlot := e.slot("i64")
	e.inst(fmt.Sprintf("store i64 %s, ptr %s", bodyVal, paySlot))
	return callResult{kind: ckSum, sum: sumSlot{
		tag: tagSlot, pay: paySlot,
		variants: []string{"Ok", "Err"}, errMsg: "error: TimedOut",
	}}, nil
}

// emitScopeRes emits chapter 13's scope resource statement. The heads
// evaluate left to right, in source order; each name becomes the block's
// binding — the resource record the head produced, which is what every
// `f.fd` inside the block reads through. Every exit of the block runs
// exactly one release per binding in reverse declaration order: the exit
// that falls out of the block releases here, an exit that pierces the
// block releases at its own site (emitReturn, emitBreak, emitContinue
// through unwind), and this statement's own emission then finds the
// block diverged and stays silent — each runtime path releases once. The
// statement produces no value (it is not an expression).
func (e *emitter) emitScopeRes(s *ast.ScopeRes) *NotImplemented {
	if len(s.Binds) == 0 {
		return e.bnd() // at least one binding is the grammar's (E1105's face)
	}
	// The bindings are the block's: registered in the enclosing
	// environment, so the body's own block scope sees them and nothing
	// after the block does.
	e.pushEnv()
	var live []resBinding
	for _, b := range s.Binds {
		reg, key, ni := e.emitResHead(b.Val)
		if ni != nil {
			return ni
		}
		e.gcEnv[b.Name] = gcBinding{rec: key, reg: reg}
		live = append(live, resBinding{name: b.Name, key: key, reg: reg})
	}
	e.resFrames = append(e.resFrames, resFrame{live: live, depth: e.nest})
	e.nest++
	if ni := e.emitBlockStmts(s.Body.Items); ni != nil {
		return ni
	}
	e.resFrames = e.resFrames[:len(e.resFrames)-1]
	e.popEnv()
	if !e.diverged {
		return e.releaseRes(live)
	}
	// The block ended in its own terminator: the path that left it
	// discharged at its site. The continuation label still opens — the
	// statement after the block needs a block to live in, exactly as an
	// if's join does.
	n := e.blocks
	e.blocks++
	e.label(fmt.Sprintf("srex%d", n))
	return nil
}

// emitResHead yields the handle a scope resource head denotes, with the
// head key its release dispatches under. A resource is a `byres record`,
// and chapter 19 lets a module declare one in either of two places: its
// own record list — a We-side record with fields, allocated and laid out
// like any other — or a foreign block, an opaque handle the native side
// owns and the We side only holds. The two live in different tables and
// take different emission faces; both name a head the method table keys
// `release` under, which is what makes the release dispatch one face.
//
// An opaque head can arrive only by crossing (chapter 19: a foreign
// function's declared return is the value's one way in — construction is
// E1707), so a call is the only further form to try, and its own
// classifier — not the callee's spelling — says which face it took.
func (e *emitter) emitResHead(x ast.Expr) (string, string, *NotImplemented) {
	c, ok := x.(*ast.Call)
	if !ok {
		return e.emitRecordValue(x)
	}
	res, ni := e.emitCall(c, nil)
	if ni != nil {
		return "", "", ni
	}
	if res.kind == ckGc {
		return res.gcReg, res.recKey, nil
	}
	if res.kind != ckPrim {
		return "", "", e.bnd()
	}
	d := e.calleeDecl(c)
	if d == nil {
		return "", "", e.bnd()
	}
	key, is := e.opaqueKeyOf(d.Ret)
	if !is {
		return "", "", e.bnd()
	}
	return res.i64, key, nil
}

// calleeDecl resolves a call expression's callee to its declaration in the
// fn table — the module's own name, or a qualified one through its imports
// (the same two lookups emitCall dispatches through). A callee that is no
// fn at all answers nil: a method, a constructor, an unknown name.
func (e *emitter) calleeDecl(c *ast.Call) *ast.FnDecl {
	switch fn := c.Fn.(type) {
	case *ast.Ident:
		if fd, ok := e.fnTable[e.curKey+"."+fn.Name]; ok {
			return fd.decl
		}
	case *ast.Member:
		recv, ok := fn.Recv.(*ast.Ident)
		if !ok {
			return nil
		}
		if k := e.resolveQual(recv.Name); k != "" {
			if fd, ok := e.fnTable[k+"."+fn.Name]; ok {
				return fd.decl
			}
		}
	}
	return nil
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
	res := e.slot("i64")
	for i, c := range s.Cases {
		cmp := e.value()
		e.inst(fmt.Sprintf("%%%s = icmp eq i64 %%%s, %d", cmp, arm, i))
		armL := fmt.Sprintf("slarm%d_%d", n, i)
		next := fmt.Sprintf("sltest%d_%d", n, i)
		e.inst(fmt.Sprintf("br i1 %%%s, label %%%s, label %%%s", cmp, armL, next))
		e.label(armL)
		// One frame per case: the yield binding lives only inside its arm
		// (the select arm's register must not shadow an outer name past
		// the join — the same block-scoping the match arms ride).
		e.pushEnv()
		if c.Name != "" && !c.Wildcard {
			e.use("__we_select_value")
			vv := e.value()
			e.inst(fmt.Sprintf("%%%s = call i64 @__we_select_value(ptr %%%s)", vv, sel))
			if c.Name != "_" {
				// The select arm's value is the runtime's i64 register;
				// its source type is not the binding site's to name.
				if e.assigned[c.Name] {
					e.bindScalarSlot(c.Name, "%"+vv, false, skNone)
				} else {
					e.scalars[c.Name] = scalarSlot{operand: "%" + vv}
				}
			}
		}
		op, _, ni := e.emitNumExpr(c.Body)
		if ni != nil {
			return callResult{}, ni
		}
		e.popEnv()
		e.inst(fmt.Sprintf("store i64 %s, ptr %s", op, res))
		e.inst(fmt.Sprintf("br label %%%s", join))
		e.label(next)
	}
	e.inst(fmt.Sprintf("br label %%%s", join))
	e.label(join)
	lv := e.value()
	e.inst(fmt.Sprintf("%%%s = load i64, ptr %s", lv, res))
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
// plain literal, an interpolated one, the `+` concatenation, a let-bound
// name, a field chain ending at a String field, or a call whose declared
// return is a String. The pair may be a constant global plus immediate,
// two loaded registers, or — for a name an fn parameter or an aggregate
// return produced — the operand pair already live in registers.
func (e *emitter) emitStringExpr(x ast.Expr) (string, string, *NotImplemented) {
	switch v := x.(type) {
	case *ast.Literal:
		if v.Kind != "string" {
			return "", "", e.bnd()
		}
		if len(v.Holes) > 0 {
			// The holes' values are computed here, not at a use: the pair
			// is whatever the concatenation produced (design D3's
			// interpolation face).
			res, ni := e.emitInterp(v)
			if ni != nil {
				return "", "", ni
			}
			return res.strBind.dataOp, res.strBind.lenOp, nil
		}
		data, ok := decodeStringLiteral(v.Text)
		if !ok {
			return "", "", e.bnd()
		}
		return e.intern(data), strconv.Itoa(len(data)), nil
	case *ast.Binary:
		return e.emitConcat(v)
	case *ast.Call:
		res, ni := e.emitCall(v, nil)
		if ni != nil {
			return "", "", ni
		}
		if res.kind != ckStr {
			return "", "", e.bnd()
		}
		return res.strBind.dataOp, res.strBind.lenOp, nil
	case *ast.Ident:
		b, ok := e.strEnv[v.Name]
		if !ok {
			if ts, isTop := e.topName(v.Name); isTop && ts.str {
				// A module-level String binding (T8-2): its globals are the
				// pair's storage, so the read is the two loads.
				r := e.topRead(ts)
				return r.strBind.dataOp, r.strBind.lenOp, nil
			}
			return "", "", e.bnd()
		}
		if b.dataOp != "" {
			return b.dataOp, b.lenOp, nil
		}
		return e.intern(b.data), strconv.Itoa(b.length), nil
	case *ast.Member:
		// The chain read resolves a qualified module-level binding itself
		// (emitMemberValue's T8-1 hook), so the String face is here already.
		return e.emitFieldChainString(v)
	default:
		return "", "", e.bnd()
	}
}

// chainOf resolves a member chain to the record that owns its last hop:
// the hops from the base binding to the member, the base's record key, and
// the chain's root register. The root is a let-bound record binding (or a
// method's self, which binds the same way); every hop but the last must
// load a record reference.
// chainHead is a member chain's starting point: the record it names, and
// where the base pointer lives. A local gc binding's is a register the
// body already holds; a module-level one (T8-2B) is a global, and only the
// head that actually reads the chain may take the load for it — the
// classification callers ask the same question with the same walk and must
// emit nothing.
type chainHead struct {
	rec string
	reg string // a local binding's register; empty when sym names a global
	sym string // the module-level global holding the handle
}

// chainHops walks a chain to its head without emitting. The head is a
// local gc binding, a module-level record binding of the walked module, or
// — where the innermost receiver is an import qualifier — the module-level
// binding the first hop names in that module (`u1.node.value`): the
// qualifier is no value at all, so the chain starts one hop in.
func (e *emitter) chainHops(m *ast.Member) (chainHead, []string, bool) {
	var hops []string
	x := m
	for {
		hops = append([]string{x.Name}, hops...)
		switch r := x.Recv.(type) {
		case *ast.Ident:
			if g, ok := e.gcEnv[r.Name]; ok {
				return chainHead{rec: g.rec, reg: g.reg}, hops, true
			}
			if ts, ok := e.topName(r.Name); ok && ts.gc && ts.rec != "" {
				return chainHead{rec: ts.rec, sym: ts.sym}, hops, true
			}
			if e.isLocalName(r.Name) {
				return chainHead{}, nil, false
			}
			k := e.resolveQual(r.Name)
			if k == "" || len(hops) < 2 {
				return chainHead{}, nil, false
			}
			ts, ok := e.topSlots[k+"."+hops[0]]
			if !ok || !ts.gc || ts.rec == "" {
				return chainHead{}, nil, false
			}
			return chainHead{rec: ts.rec, sym: ts.sym}, hops[1:], true
		case *ast.Member:
			x = r
		default:
			return chainHead{}, nil, false
		}
	}
}

func (e *emitter) chainOf(m *ast.Member) (base, recKey string, hops []string, ok bool) {
	h, hops, ok := e.chainHops(m)
	if !ok {
		return "", "", nil, false
	}
	if h.reg != "" {
		return h.reg, h.rec, hops, true
	}
	// A module-level head: one load of the binding's global, taken here at
	// the chain's own position (the same read topRead takes).
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = load ptr, ptr @%s", v, h.sym))
	return "%" + v, h.rec, hops, true
}

// walkHops loads the pointer hops before the last one, landing on the
// record that owns the final field.
func (e *emitter) walkHops(base, recKey string, hops []string) (string, string, bool) {
	for _, h := range hops[:len(hops)-1] {
		slot, ok := e.fieldSlotOf(recKey, h)
		if !ok || (slot.kind != fkRef && slot.kind != fkVal) {
			return "", "", false
		}
		base = e.gepLoadPtr(base, slot.off)
		recKey = slot.typ
	}
	return base, recKey, true
}

// emitMemberValue reads one field: the unified member read of design D4.
// The chain's trailing hop decides its face — a scalar's word, a double,
// a String's pair, or a nested record's pointer — and every face is one
// getelementptr plus the load of what it holds (the String-only chain the
// M8 era carried was the shape's one special case).
// emitNewtypeCtor emits one newtype construction: the result is the
// argument's own value — no allocation, no call, no wrapper (design D4's
// erasure) — carrying the wrapper's name for the `.value` read that
// spends it.
func (e *emitter) emitNewtypeCtor(key string, arg ast.Expr) (callResult, *NotImplemented) {
	under, ok := e.newtypes[key]
	if !ok {
		return callResult{}, e.bnd()
	}
	k, recKey, ok := e.classType(under)
	if !ok {
		return callResult{}, e.bnd()
	}
	res := callResult{ntype: key, typeName: baseTypeName(e.derefNewtype(under))}
	switch k {
	case abiI64:
		op, isF, ni := e.emitNumExpr(arg)
		if ni != nil || isF {
			return callResult{}, e.bnd()
		}
		res.kind, res.i64 = ckI64, op
	case abiDouble:
		op, isF, ni := e.emitNumExpr(arg)
		if ni != nil || !isF {
			return callResult{}, e.bnd()
		}
		res.kind, res.i64, res.isFloat = ckI64, op, true
	case abiStr:
		p, l, ni := e.emitStringExpr(arg)
		if ni != nil {
			return callResult{}, ni
		}
		res.kind, res.strBind = ckStr, strBinding{dataOp: p, lenOp: l}
	case abiGc:
		// Constructing a wrapper around a value constructs a value: the
		// underlying record's own category decides whether it copies.
		reg, _, ni := e.emitOwnedRecord(arg)
		if ni != nil {
			return callResult{}, ni
		}
		res.kind, res.gcReg, res.recKey = ckGc, reg, recKey
	default:
		return callResult{}, e.bnd()
	}
	return res, nil
}

// tupleElemVal is one emitted tuple element on its way into the
// aggregate: the shape it takes there, and the operands that fill it.
type tupleElemVal struct {
	el  tupleElem
	op  string // the single-word operand (i64, double, or ptr)
	p   string // a String element's data word
	l   string // a String element's length word
	sum sumSlot
}

// emitTupleAgg materializes a tuple expression: the elements emit first —
// their own domains are what the aggregate's field types come from — and
// the aggregate that holds them is reserved behind them (design D4's
// `{f0, f1, ...}` stack value).
func (e *emitter) emitTupleAgg(t *ast.Tuple) (string, []tupleElem, *NotImplemented) {
	if len(t.Elems) < 2 {
		return "", nil, e.bnd() // `( e )` is a grouping and `()` is unit
	}
	vals := make([]tupleElemVal, 0, len(t.Elems))
	off := 0
	for _, x := range t.Elems {
		v, ni := e.emitTupleElemValue(x)
		if ni != nil {
			return "", nil, ni
		}
		v.el.off = off
		off += 8 * len(v.el.words)
		vals = append(vals, v)
	}
	elems := make([]tupleElem, len(vals))
	for i, v := range vals {
		elems[i] = v.el
	}
	agg := e.slot(aggTyp(elems))
	for _, v := range vals {
		e.storeTupleElem(agg, v)
	}
	return agg, elems, nil
}

// emitTupleElemValue emits one element's value at its own face: the
// family comes from the expression's domain — the same classification
// every other value face reads — and a call's from the call's result.
func (e *emitter) emitTupleElemValue(x ast.Expr) (tupleElemVal, *NotImplemented) {
	if c, ok := x.(*ast.Call); ok {
		res, ni := e.emitCall(c, nil)
		if ni != nil {
			return tupleElemVal{}, ni
		}
		switch res.kind {
		case ckI64:
			if res.isFloat {
				return tupleElemVal{el: tupleElem{kind: abiDouble, typ: res.typeName, words: []string{"double"}}, op: res.i64}, nil
			}
			return tupleElemVal{el: tupleElem{kind: abiI64, typ: res.typeName, words: []string{"i64"}}, op: res.i64}, nil
		case ckStr:
			return tupleElemVal{el: tupleElem{kind: abiStr, typ: "String", words: []string{"ptr", "i64"}},
				p: res.strBind.dataOp, l: res.strBind.lenOp}, nil
		case ckGc:
			return tupleElemVal{el: tupleElem{kind: abiGc, key: res.recKey, words: []string{"ptr"}}, op: res.gcReg}, nil
		case ckSum:
			return tupleElemVal{el: tupleElem{kind: abiSum, words: []string{"i64", "i64"}}, sum: res.sum}, nil
		}
		return tupleElemVal{}, e.bnd()
	}
	switch k := e.valueKind(x); k {
	case skStr:
		p, l, ni := e.emitStringExpr(x)
		if ni != nil {
			return tupleElemVal{}, ni
		}
		return tupleElemVal{el: tupleElem{kind: abiStr, typ: "String", words: []string{"ptr", "i64"}},
			p: p, l: l}, nil
	case skF64:
		op, isF, ni := e.emitNumExpr(x)
		if ni != nil || !isF {
			return tupleElemVal{}, e.bnd()
		}
		return tupleElemVal{el: tupleElem{kind: abiDouble, typ: "Float64", words: []string{"double"}}, op: op}, nil
	case skI64, skU64, skBool, skRune:
		op, isF, ni := e.emitNumExpr(x)
		if ni != nil || isF {
			return tupleElemVal{}, e.bnd()
		}
		return tupleElemVal{el: tupleElem{kind: abiI64, typ: strKindName(k), words: []string{"i64"}}, op: op}, nil
	}
	if key, ok := e.recvKeyOf(x); ok {
		// A record element constructs (or copies) at the aggregate's own
		// store: the tuple holds values, never shares (chapter 8).
		reg, _, ni := e.emitOwnedRecord(x)
		if ni != nil {
			return tupleElemVal{}, ni
		}
		return tupleElemVal{el: tupleElem{kind: abiGc, key: key, words: []string{"ptr"}}, op: reg}, nil
	}
	if id, ok := x.(*ast.Ident); ok {
		if sl, ok := e.sums2[id.Name]; ok {
			return tupleElemVal{el: tupleElem{kind: abiSum, words: []string{"i64", "i64"}}, sum: sl}, nil
		}
	}
	return tupleElemVal{}, e.bnd()
}

// storeTupleElem writes one emitted element's words into the aggregate at
// its offset.
func (e *emitter) storeTupleElem(agg string, v tupleElemVal) {
	switch v.el.kind {
	case abiI64:
		e.gepStore(agg, v.el.off, "i64 "+v.op)
	case abiDouble:
		e.gepStore(agg, v.el.off, "double "+v.op)
	case abiGc:
		e.gepStore(agg, v.el.off, "ptr "+v.op)
	case abiStr:
		e.gepStore(agg, v.el.off, "ptr "+v.p)
		e.gepStore(agg, v.el.off+8, "i64 "+v.l)
	case abiSum:
		e.gepStore(agg, v.el.off, "i64 "+e.loadNum(v.sum.tag, false))
		e.gepStore(agg, v.el.off+8, "i64 "+e.loadNum(v.sum.pay, false))
	}
}

// emitTupleOperand yields a tuple-valued expression's aggregate handle
// and shapes: the shared entry of the construction, the destructuring
// pattern, and the call argument's expansion.
func (e *emitter) emitTupleOperand(x ast.Expr) (string, []tupleElem, *NotImplemented) {
	switch v := x.(type) {
	case *ast.Tuple:
		return e.emitTupleAgg(v)
	case *ast.Ident:
		if b, ok := e.tupEnv[v.Name]; ok {
			return b.ptr, b.elems, nil
		}
	case *ast.Call:
		res, ni := e.emitCall(v, nil)
		if ni != nil {
			return "", nil, ni
		}
		if res.kind == ckTuple {
			return res.tup.ptr, res.tup.elems, nil
		}
	}
	return "", nil, e.bnd()
}

// emitLetPattern binds one destructuring `let`: chapter 8's tuple pattern
// binds each name from the element it stands for (design D4's 模式解构)
// — a load per element, never a call.
func (e *emitter) emitLetPattern(s *ast.Binding) *NotImplemented {
	if s.Kw != "let" {
		return e.bnd()
	}
	pt, ok := s.Pat.(*ast.PatTuple)
	if !ok {
		return e.bnd()
	}
	ptr, elems, ni := e.emitTupleOperand(s.Init)
	if ni != nil {
		return ni
	}
	if len(pt.Elems) != len(elems) {
		return e.bnd()
	}
	for i, sub := range pt.Elems {
		switch b := sub.(type) {
		case *ast.PatWildcard:
		case *ast.PatBinding:
			if b.Name == "_" {
				continue
			}
			res, ni := e.loadTupleElem(ptr, elems[i])
			if ni != nil {
				return ni
			}
			// An element read out of a value record's aggregate is the
			// same value the aggregate holds a copy of; binding it takes
			// its own object (chapter 8's "every binding of a value
			// record takes its own object").
			if res.kind == ckGc {
				if r, ok := e.records[res.recKey]; ok && r.Cat == "value" {
					res.gcReg = e.emitRecCopy(res.gcReg, res.recKey)
				}
			}
			if ni := e.bindResult(b.Name, res); ni != nil {
				return ni
			}
		default:
			// A nested pattern names a sub-tuple's own elements: the
			// aggregate nests with the pattern (a follow-up).
			return e.bnd()
		}
	}
	return nil
}

// bindingFace returns name's own value face — what a `.value` unwrap
// hands back. A name lives in exactly one environment, so the lookups are
// exclusive.
func (e *emitter) bindingFace(name string) (callResult, *NotImplemented) {
	if b, ok := e.strEnv[name]; ok {
		return callResult{kind: ckStr, strBind: b}, nil
	}
	if b, ok := e.gcEnv[name]; ok {
		return callResult{kind: ckGc, gcReg: b.reg, recKey: b.rec}, nil
	}
	if sl, ok := e.scalars[name]; ok {
		op := sl.operand
		if sl.alloca != "" {
			op = e.loadNum(sl.alloca, sl.isFloat)
		}
		return callResult{kind: ckI64, i64: op, isFloat: sl.isFloat}, nil
	}
	return callResult{}, e.bnd()
}

// loadTupleElem reads one element out of a tuple's aggregate at its own
// face — the element's value, never a call.
func (e *emitter) loadTupleElem(agg string, el tupleElem) (callResult, *NotImplemented) {
	switch el.kind {
	case abiI64:
		return callResult{kind: ckI64, i64: e.gepLoadI64(agg, el.off), typeName: el.typ}, nil
	case abiDouble:
		return callResult{kind: ckI64, i64: e.gepLoadDouble(agg, el.off), isFloat: true, typeName: el.typ}, nil
	case abiStr:
		return callResult{kind: ckStr, strBind: strBinding{
			dataOp: e.gepLoadPtr(agg, el.off), lenOp: e.gepLoadI64(agg, el.off+8)}}, nil
	case abiGc:
		return callResult{kind: ckGc, gcReg: e.gepLoadPtr(agg, el.off), recKey: el.key}, nil
	}
	return callResult{}, e.bnd()
}

func (e *emitter) emitMemberValue(m *ast.Member) (callResult, *NotImplemented) {
	if ts, ok := e.topMember(m); ok {
		// A qualified module-level binding read (T8-1): `util.base` is one
		// load of that module's global, the same face the owning module's
		// own bodies take.
		return e.topRead(ts), nil
	}
	if id, ok := m.Recv.(*ast.Ident); ok {
		if _, is := e.ntEnv[id.Name]; is {
			// A newtype value's one member is `.value` (chapter 8): the
			// wrapper erased, so the read is the identity — the binding's
			// own face. Any other name is no member of the wrapper's.
			if m.Name != "value" {
				return callResult{}, e.bnd()
			}
			res, ni := e.bindingFace(id.Name)
			if ni != nil {
				return callResult{}, ni
			}
			if res.kind == ckI64 && res.typeName == "" {
				// The unwrapped value's domain is the underlying's, which
				// the wrapper's declaration names exactly.
				res.typeName = baseTypeName(e.derefNewtype(e.newtypes[e.ntEnv[id.Name]]))
			}
			return res, nil
		}
	}
	base, recKey, hops, ok := e.chainOf(m)
	if !ok {
		return callResult{}, e.bnd()
	}
	base, recKey, ok = e.walkHops(base, recKey, hops)
	if !ok {
		return callResult{}, e.bnd()
	}
	slot, ok := e.fieldSlotOf(recKey, hops[len(hops)-1])
	if !ok {
		return callResult{}, e.bnd()
	}
	switch slot.kind {
	case fkStr:
		return callResult{kind: ckStr, strBind: strBinding{
			dataOp: e.gepLoadPtr(base, slot.off),
			lenOp:  e.gepLoadI64(base, slot.off+8),
		}}, nil
	case fkScalar:
		return callResult{kind: ckI64, i64: e.gepLoadI64(base, slot.off),
			typeName: slot.typ}, nil
	case fkF64:
		return callResult{kind: ckI64, i64: e.gepLoadDouble(base, slot.off), isFloat: true,
			typeName: slot.typ}, nil
	case fkRef, fkVal:
		return callResult{kind: ckGc, gcReg: e.gepLoadPtr(base, slot.off),
			recKey: slot.typ}, nil
	}
	return callResult{}, e.bnd()
}

// emitFieldChainString is the String member face: the same read, with the
// pair as its only admissible result.
func (e *emitter) emitFieldChainString(m *ast.Member) (string, string, *NotImplemented) {
	res, ni := e.emitMemberValue(m)
	if ni != nil {
		return "", "", ni
	}
	if res.kind != ckStr {
		return "", "", e.bnd()
	}
	return res.strBind.dataOp, res.strBind.lenOp, nil
}

// --- the T4 String emission set (design D3) ----------------------------------
//
// A String value is a runtime (ptr, len) operand pair. The literal faces
// carried one from the M8 era (a constant global plus a length) and from
// M10b (a fn parameter's or an aggregate return's two live registers); T4
// widens the pair's provenance to the whole String domain — concatenation,
// the chapter 17 member family, equality, and the interpolation holes —
// and adds the one classification the interpolation renderer needs: which
// base type a hole's expression holds, where that is decidable statically.

// strKind classifies a value in the interpolation domain: the base-type
// family (design D3's eight integers, Float64, Bool, Rune) and String
// itself, nothing else.
type strKind int

const (
	skNone strKind = iota // outside the domain — a hole of it stops at the boundary
	skStr
	// skI64 is the whole integer family but UInt64: Int8..Int64 sign-extend
	// and UInt8..UInt32 zero-extend into the i64 domain, so each value is
	// its own i64 and one converter renders them all.
	skI64
	skU64 // UInt64 alone holds a magnitude its i64 does not
	skF64
	skBool
	skRune
)

// baseStrKind classifies one base type name — the annotation, parameter,
// and declared-return sites' contribution. Every other name (a record, a
// sum, a generic) and the empty name are skNone. Float32 is absent until
// the narrow-width track gives it a storage face at all (T11).
func baseStrKind(name string) strKind {
	switch name {
	case "String":
		return skStr
	case "UInt64":
		return skU64
	case "Int64", "Int32", "Int16", "Int8", "UInt32", "UInt16", "UInt8":
		return skI64
	case "Float64":
		return skF64
	case "Bool":
		return skBool
	case "Rune":
		return skRune
	}
	return skNone
}

// strKindName is the domain name one classification spells back out —
// the element a tuple value's own type contributes where the site that
// emitted it had only the domain to go on.
func strKindName(k strKind) string {
	switch k {
	case skStr:
		return "String"
	case skU64:
		return "UInt64"
	case skI64:
		return "Int64"
	case skF64:
		return "Float64"
	case skBool:
		return "Bool"
	case skRune:
		return "Rune"
	}
	return ""
}

// baseTypeName returns t's bare base-type name where the reference spells
// one (no qualifier, no type arguments); "" for everything else, which
// classifies as skNone.
func baseTypeName(t ast.TypeRef) string {
	n, ok := t.(*ast.NamedType)
	if !ok || n.Qual != "" || len(n.Args) != 0 {
		return ""
	}
	if baseStrKind(n.Name) == skNone {
		return ""
	}
	return n.Name
}

// literalStrKind classifies a literal in the interpolation domain: its
// token kind, refined by an integer literal's own type suffix — a `u64`
// value renders unsigned (design D3's domain), while the narrower
// unsigned types zero-extend into the i64 domain and render the same
// either way.
func literalStrKind(l *ast.Literal) strKind {
	if l.Kind == "int" {
		if _, suf := splitIntSuffix(l.Text); suf == "u64" {
			return skU64
		}
	}
	return litStrKind(l.Kind)
}

// litStrKind classifies a literal by its token kind — the domain an
// unannotated binding takes (Int64, Float64, Bool, Rune, String).
func litStrKind(kind string) strKind {
	switch kind {
	case "string":
		return skStr
	case "int":
		return skI64
	case "float":
		return skF64
	case "bool":
		return skBool
	case "rune":
		return skRune
	}
	return skNone
}

// valueKind classifies x in the interpolation domain where the answer is
// static: a literal's own kind, a binding whose site fixed its type
// (scalarSlot.kind), a String binding or field chain, a callee's declared
// return type, and the operators over those. Everything else — a numeric
// record field, a match arm's payload, a foreign result, a value-form join
// — is skNone and stops the hole at the boundary: design D3 names the base
// family and String, and the composite traversal is D4's (T5).
//
// The classification never exceeds what the emission can do: it gates the
// dispatch, and the emission re-checks the domain the value actually lands
// in, so a wrong guess costs a boundary rather than a wrong rendering.
func (e *emitter) valueKind(x ast.Expr) strKind {
	switch v := x.(type) {
	case *ast.Literal:
		return literalStrKind(v)
	case *ast.Ident:
		if _, ok := e.strEnv[v.Name]; ok {
			return skStr
		}
		if s, ok := e.scalars[v.Name]; ok {
			return s.kind
		}
		// A module-level binding (T8-1) carries the domain its global was
		// typed with, so a reader classifies it without emitting.
		if ts, ok := e.topName(v.Name); ok {
			return ts.kind
		}
		return skNone
	case *ast.Member:
		return e.memberKind(v)
	case *ast.Unary:
		switch v.Op {
		case "!":
			return skBool
		case "-":
			k := e.valueKind(v.X)
			if k == skF64 {
				return skF64
			}
			if k == skI64 || k == skU64 {
				return k
			}
			return skNone
		}
		return skNone
	case *ast.Binary:
		return e.binaryKind(v)
	case *ast.Call:
		return e.callStrKind(v)
	}
	return skNone
}

// binaryKind classifies an operator's result: a comparison or a logical
// join is a Bool, `+` over two Strings is the concatenation, and an
// integer arithmetic result is the i64 domain — Rune and Bool stay out of
// the arithmetic kinds, since a result computed in the i64 domain renders
// as the integer it is (a Rune operand's own rendering is its character,
// but `r + 1` is not that character's neighbour).
func (e *emitter) binaryKind(b *ast.Binary) strKind {
	switch b.Op {
	case "&&", "||", "==", "!=", "<", "<=", ">", ">=":
		return skBool
	case "+":
		if e.valueKind(b.L) == skStr && e.valueKind(b.R) == skStr {
			return skStr
		}
	}
	k := e.arithKind(b.L, b.R)
	return k
}

// arithKind is the numeric join of two operands: a Float64 operand makes
// the result a float, and two integer operands stay in the i64 domain.
func (e *emitter) arithKind(l, r ast.Expr) strKind {
	kl, kr := e.valueKind(l), e.valueKind(r)
	if kl == skF64 || kr == skF64 {
		return skF64
	}
	if kl == skNone || kr == skNone {
		return skNone
	}
	switch kl {
	case skI64, skU64:
		switch kr {
		case skI64, skU64:
			return skI64
		}
	}
	return skNone
}

// callStrKind reads a call's declared result type: a program fn's own
// signature, a chapter 17 String member's fixed result, or nothing where
// the declaration does not fix one.
func (e *emitter) callStrKind(call *ast.Call) strKind {
	if id, ok := call.Fn.(*ast.Ident); ok {
		fd, ok := e.fnTable[e.curKey+"."+id.Name]
		if !ok {
			return skNone
		}
		return e.fnRetKind(fd)
	}
	fn, ok := call.Fn.(*ast.Member)
	if !ok {
		return skNone
	}
	if e.valueKind(fn.Recv) == skStr {
		return strMemberKind(fn.Name)
	}
	// A method call carries its method's return family (design D4): the
	// table answers what the fn table answers for a plain call, so a
	// method result joins the String domain like any other.
	if recvKey, ok := e.recvKeyOf(fn.Recv); ok {
		if fd, is := e.methods[recvKey+"."+fn.Name]; is {
			return e.fnRetKind(fd)
		}
	}
	recv, ok := fn.Recv.(*ast.Ident)
	if !ok || e.isLocalName(recv.Name) {
		return skNone
	}
	if k := e.resolveQual(recv.Name); k != "" {
		if fd, ok := e.fnTable[k+"."+fn.Name]; ok {
			return e.fnRetKind(fd)
		}
	}
	return skNone
}

// fnRetKind classifies one program fn's declared return type.
func (e *emitter) fnRetKind(fd *fnDef) strKind {
	abi, ok := e.classify(fd)
	if !ok {
		return skNone
	}
	return baseStrKind(abi.retName)
}

// strMemberKind is the result family of chapter 17's String members: the
// two access layers (byteLength, runeCount yield Int64; charAt yields the
// Rune it reads; byteSlice a String view). iterator is outside the set.
func strMemberKind(method string) strKind {
	switch method {
	case "byteLength", "runeCount":
		return skI64
	case "charAt":
		return skRune
	case "byteSlice":
		return skStr
	}
	return skNone
}

// memberKind classifies a member chain's value statically (design D3's
// domain): a String field is skStr, a scalar field is its declared base
// type's kind, and a nested record's pointer is outside the domain.
func (e *emitter) memberKind(m *ast.Member) strKind {
	if ts, ok := e.topMember(m); ok {
		// A qualified module-level binding read (T8-1) carries its global's
		// domain.
		return ts.kind
	}
	if id, ok := m.Recv.(*ast.Ident); ok {
		if _, is := e.ntEnv[id.Name]; is {
			// The unwrap's domain is the underlying's — for a scalar
			// binding the slot already carries it; a String binding
			// renders as a String whatever the wrapper is called.
			if m.Name != "value" {
				return skNone
			}
			return e.valueKind(id)
		}
	}
	slot, ok := e.chainField(m)
	if !ok {
		return skNone
	}
	switch slot.kind {
	case fkStr:
		return skStr
	case fkScalar, fkF64:
		return baseStrKind(slot.typ)
	}
	return skNone
}

// chainField resolves a member chain's trailing field from the layouts
// alone — the emit-free half of the member read.
func (e *emitter) chainField(m *ast.Member) (fieldSlot, bool) {
	h, hops, ok := e.chainHops(m)
	if !ok {
		return fieldSlot{}, false
	}
	recKey := h.rec
	for _, h := range hops[:len(hops)-1] {
		slot, ok := e.fieldSlotOf(recKey, h)
		if !ok || (slot.kind != fkRef && slot.kind != fkVal) {
			return fieldSlot{}, false
		}
		recKey = slot.typ
	}
	return e.fieldSlotOf(recKey, hops[len(hops)-1])
}

// strCall emits one String-family runtime call whose result is the
// two-word pair: the call, then the two extractvalues that bring the words
// into registers. The C declaration returns the struct in two registers
// (both eightbytes INTEGER class), which is the shape clang lowers the
// declaration to — so the IR struct return and the C ABI agree.
func (e *emitter) strCall(sym, args string) (string, string, *NotImplemented) {
	e.use(sym)
	e.strStruct = true
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = call %%struct.we_str @%s(%s)", v, sym, args))
	p := e.value()
	e.inst(fmt.Sprintf("%%%s = extractvalue %%struct.we_str %%%s, 0", p, v))
	l := e.value()
	e.inst(fmt.Sprintf("%%%s = extractvalue %%struct.we_str %%%s, 1", l, v))
	return "%" + p, "%" + l, nil
}

// concatStr joins two String operand pairs through the runtime (a fresh
// buffer: neither operand's bytes are aliased into the result).
func (e *emitter) concatStr(ap, al, bp, bl string) (string, string, *NotImplemented) {
	return e.strCall("__we_str_concat", fmt.Sprintf("ptr %s, i64 %s, ptr %s, i64 %s", ap, al, bp, bl))
}

// emitHole renders one interpolation hole into a String pair (design D3's
// domain). The classification picks the converter; the emission then
// re-checks the domain the value landed in — a slot classified as an
// integer whose operand turns out to be a double stops rather than
// rendering the register's bits as a number.
func (e *emitter) emitHole(x ast.Expr) (callResult, *NotImplemented) {
	k := e.valueKind(x)
	if k == skNone {
		return callResult{}, e.bnd()
	}
	if k == skStr {
		p, l, ni := e.emitStringExpr(x)
		if ni != nil {
			return callResult{}, ni
		}
		return callResult{kind: ckStr, strBind: strBinding{dataOp: p, lenOp: l}}, nil
	}
	op, isF, ni := e.emitNumExpr(x)
	if ni != nil {
		return callResult{}, ni
	}
	var sym, arg string
	if k == skF64 {
		if !isF {
			return callResult{}, e.bnd()
		}
		sym, arg = "__we_str_of_f64", "double "+op
	} else {
		if isF {
			return callResult{}, e.bnd()
		}
		sym = map[strKind]string{
			skI64:  "__we_str_of_i64",
			skU64:  "__we_str_of_u64",
			skBool: "__we_str_of_bool",
			skRune: "__we_str_of_rune",
		}[k]
		arg = "i64 " + op
	}
	p, l, ni := e.strCall(sym, arg)
	if ni != nil {
		return callResult{}, ni
	}
	return callResult{kind: ckStr, strBind: strBinding{dataOp: p, lenOp: l}}, nil
}

// emitInterp emits one interpolated literal (chapter 1's `${ … }` regions):
// the decoded runs between the holes and each hole's rendered value, joined
// left to right by the runtime's concatenation. An empty run contributes
// nothing — the buffers between adjacent holes — so a literal whose holes
// are adjacent calls concat once per join rather than once per position.
//
// The result is a fresh buffer, never a segment constant: the constant pool
// holds literals and the concatenation allocates, so nothing here is
// written into a pool entry.
func (e *emitter) emitInterp(lit *ast.Literal) (callResult, *NotImplemented) {
	if len(lit.Segs) != len(lit.Holes)+1 || len(lit.Holes) == 0 {
		// The parser guarantees one run before each hole and one after
		// (len(Segs) == len(Holes)+1); a literal shaped otherwise did not
		// come from a parse, and an emission that indexed past the runs
		// would panic rather than report.
		return callResult{}, e.bnd()
	}
	type pair struct{ p, l string }
	var parts []pair
	for i, h := range lit.Holes {
		if seg := lit.Segs[i]; seg != "" {
			parts = append(parts, pair{e.intern(seg), strconv.Itoa(len(seg))})
		}
		res, ni := e.emitHole(h)
		if ni != nil {
			return callResult{}, ni
		}
		parts = append(parts, pair{res.strBind.dataOp, res.strBind.lenOp})
	}
	if seg := lit.Segs[len(lit.Holes)]; seg != "" {
		parts = append(parts, pair{e.intern(seg), strconv.Itoa(len(seg))})
	}
	acc := parts[0]
	for _, p := range parts[1:] {
		d, l, ni := e.concatStr(acc.p, acc.l, p.p, p.l)
		if ni != nil {
			return callResult{}, ni
		}
		acc = pair{d, l}
	}
	return callResult{kind: ckStr, strBind: strBinding{dataOp: acc.p, lenOp: acc.l}}, nil
}

// emitStrMember emits one chapter 17 String member call (design D3): the
// two access layers over the value's bytes. byteLength is the length
// operand itself — the pair carries the byte count, so the member is an
// identity on the second word and needs no call at all. runecount and
// charat walk code points, byteslice spans bytes, and each index space
// reports out of range through chapter 14's family (the C side raises the
// task failure; the emitter emits the call).
func (e *emitter) emitStrMember(recv ast.Expr, method string, args []ast.Expr) (callResult, *NotImplemented) {
	p, l, ni := e.emitStringExpr(recv)
	if ni != nil {
		return callResult{}, ni
	}
	index := func(x ast.Expr) (string, *NotImplemented) {
		op, isF, ni := e.emitNumExpr(x)
		if ni != nil {
			return "", ni
		}
		if isF {
			return "", e.bnd()
		}
		return op, nil
	}
	switch method {
	case "byteLength":
		if len(args) != 0 {
			return callResult{}, e.bnd()
		}
		return callResult{kind: ckI64, i64: l, typeName: "Int64"}, nil
	case "runeCount":
		if len(args) != 0 {
			return callResult{}, e.bnd()
		}
		e.use("__we_str_runecount")
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 @__we_str_runecount(ptr %s, i64 %s)", v, p, l))
		return callResult{kind: ckI64, i64: "%" + v, typeName: "Int64"}, nil
	case "charAt":
		if len(args) != 1 {
			return callResult{}, e.bnd()
		}
		i, ni := index(args[0])
		if ni != nil {
			return callResult{}, ni
		}
		e.use("__we_str_charat")
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 @__we_str_charat(ptr %s, i64 %s, i64 %s)", v, p, l, i))
		return callResult{kind: ckI64, i64: "%" + v, typeName: "Rune"}, nil
	case "byteSlice":
		if len(args) != 2 {
			return callResult{}, e.bnd()
		}
		lo, ni := index(args[0])
		if ni != nil {
			return callResult{}, ni
		}
		hi, ni := index(args[1])
		if ni != nil {
			return callResult{}, ni
		}
		sp, sl, ni := e.strCall("__we_str_byteslice", fmt.Sprintf("ptr %s, i64 %s, i64 %s, i64 %s", p, l, lo, hi))
		if ni != nil {
			return callResult{}, ni
		}
		return callResult{kind: ckStr, strBind: strBinding{dataOp: sp, lenOp: sl}}, nil
	}
	// iterator is a value form of its own (the for-in source rides it
	// directly at T4-3); as a member call it is outside the set.
	return callResult{}, e.bnd()
}

// emitConcat emits `a + b` over two String operands (chapter 10's closed
// operator set: `+` over base types that are String is the concatenation).
func (e *emitter) emitConcat(b *ast.Binary) (string, string, *NotImplemented) {
	if b.Op != "+" || e.valueKind(b.L) != skStr || e.valueKind(b.R) != skStr {
		return "", "", e.bnd()
	}
	ap, al, ni := e.emitStringExpr(b.L)
	if ni != nil {
		return "", "", ni
	}
	bp, bl, ni := e.emitStringExpr(b.R)
	if ni != nil {
		return "", "", ni
	}
	return e.concatStr(ap, al, bp, bl)
}

// layout computes a record's field slots: offsets from 16 (the frozen
// header {map@0, size@8} of design D6), sizes, and the reference bitmap
// derived kinds. key is the module the record declares in — a reference
// field resolves in the record's own module, so a reference target
// carries key+"."+FieldName. A field outside the family fails the whole
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
		case "Float64":
			slots[i].kind = fkF64
			slots[i].typ = n.Name
			off += 8
		case "Int64", "Int32", "Int16", "Int8", "UInt64", "UInt32", "UInt16", "UInt8", "Bool":
			// The widths share one i64 word until the narrow-width track
			// gives each its own storage face (T11); the declared name is
			// kept so the read carries its domain (design D3's table).
			slots[i].kind = fkScalar
			slots[i].typ = n.Name
			off += 8
		default:
			r, ok := e.records[key+"."+n.Name]
			if !ok || len(r.TypeParams) != 0 {
				return nil, 0, false
			}
			switch r.Cat {
			case "value":
				slots[i].kind = fkVal
			case "gc", "resource":
				slots[i].kind = fkRef
			default:
				return nil, 0, false
			}
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

// emitConstruct emits the construction protocol of design D4: alloc, map
// store, root push — the object is rooted before any field value
// evaluates, so a nested allocation never races a collection with its
// parent unrooted — then the field stores in source order. An update
// expression (`with &old`) takes the same protocol with the base's fields
// standing in for the unnamed ones: a fresh object, the base's values
// copied in, then the named fields overwritten. The record resolves in
// the walked module (a qualified head resolves its module first —
// cross-module records construct the same way, each under its own
// module's symbols); the return pairs the pointer with the record's
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
	if !ok || len(rec.TypeParams) != 0 || len(c.TypeArgs) != 0 {
		return "", "", e.bnd()
	}
	slots, total, ok := e.layout(key, rec)
	if !ok {
		return "", "", e.bnd()
	}
	rkey := key + "." + rec.Name
	e.usedRecs[rkey] = true
	var reg string
	if c.Base != nil {
		// The update's base must be the head's own record (chapter 8's
		// E0603); the copy carries every unnamed field from it.
		base, baseKey, ni := e.emitRecordValue(c.Base)
		if ni != nil {
			return "", "", ni
		}
		if baseKey != rkey {
			return "", "", e.bnd()
		}
		reg = e.emitRecCopy(base, rkey)
	} else {
		reg = e.allocRecord(rkey, total)
	}
	byName := make(map[string]int, len(rec.Fields))
	for i, fd := range rec.Fields {
		byName[fd.Name] = i
	}
	for _, fi := range c.Fields {
		idx, ok := byName[fi.Name]
		if !ok {
			return "", "", e.bnd()
		}
		if ni := e.emitFieldStore(reg, slots[idx], fi.Value); ni != nil {
			return "", "", ni
		}
	}
	return reg, rkey, nil
}

// allocRecord opens one record object: the allocation, the frozen header's
// map word, and the root push every construction owes before a field value
// can allocate.
func (e *emitter) allocRecord(recKey string, total int) string {
	e.use("__we_alloc")
	e.use("__we_root_push")
	e.pushes++
	reg := "%" + e.value()
	e.inst(fmt.Sprintf("%s = call ptr @__we_alloc(i64 %d)", reg, total))
	e.inst(fmt.Sprintf("store ptr @.map.%s, ptr %s", recKey, reg))
	e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", reg))
	return reg
}

// emitFieldStore writes one field of a record under construction (or one
// named field of an update, or one `self.field =` write) at its slot.
func (e *emitter) emitFieldStore(reg string, slot fieldSlot, value ast.Expr) *NotImplemented {
	switch slot.kind {
	case fkStr:
		p, l, ni := e.emitStringExpr(value)
		if ni != nil {
			return ni
		}
		e.gepStore(reg, slot.off, "ptr "+p)
		e.gepStore(reg, slot.off+8, "i64 "+l)
	case fkScalar:
		op, isF, ni := e.emitNumExpr(value)
		if ni != nil {
			return ni
		}
		if isF {
			return e.bnd()
		}
		e.gepStore(reg, slot.off, "i64 "+op)
	case fkF64:
		op, isF, ni := e.emitNumExpr(value)
		if ni != nil {
			return ni
		}
		if !isF {
			return e.bnd()
		}
		e.gepStore(reg, slot.off, "double "+op)
	case fkRef:
		child, _, ni := e.emitRecordValue(value)
		if ni != nil {
			return ni
		}
		e.gepStore(reg, slot.off, "ptr "+child)
	case fkVal:
		child, _, ni := e.emitOwnedRecord(value)
		if ni != nil {
			return ni
		}
		e.gepStore(reg, slot.off, "ptr "+child)
	}
	return nil
}

// emitRecordValue yields the record pointer an expression denotes — a
// binding, a construction or update, a call's result, a field read. It
// shares whatever object the expression names; ownership is the caller's
// business (see emitOwnedRecord).
func (e *emitter) emitRecordValue(x ast.Expr) (string, string, *NotImplemented) {
	switch v := x.(type) {
	case *ast.Ident:
		g, ok := e.gcEnv[v.Name]
		if !ok {
			return "", "", e.bnd()
		}
		return g.reg, g.rec, nil
	case *ast.Construct:
		return e.emitConstruct(v)
	case *ast.Call:
		res, ni := e.emitCall(v, nil)
		if ni != nil {
			return "", "", ni
		}
		if res.kind != ckGc {
			return "", "", e.bnd()
		}
		return res.gcReg, res.recKey, nil
	case *ast.Member:
		res, ni := e.emitMemberValue(v)
		if ni != nil {
			return "", "", ni
		}
		if res.kind != ckGc {
			return "", "", e.bnd()
		}
		return res.gcReg, res.recKey, nil
	}
	return "", "", e.bnd()
}

// emitOwnedRecord yields a record value the caller owns: a value-category
// record is copied unless the expression already produced a fresh object
// (a construction or update — chapter 8's value semantics, "two bindings
// of a copied value share nothing"). A gc- or resource-category record is
// shared, which is what its category means.
func (e *emitter) emitOwnedRecord(x ast.Expr) (string, string, *NotImplemented) {
	reg, recKey, ni := e.emitRecordValue(x)
	if ni != nil {
		return "", "", ni
	}
	if r, ok := e.records[recKey]; ok && r.Cat == "value" {
		if _, fresh := x.(*ast.Construct); !fresh {
			return e.emitRecCopy(reg, recKey), recKey, nil
		}
	}
	return reg, recKey, nil
}

// emitRecCopy emits the copy face of design D4 — a second object holding
// the source's fields. The two categories copy differently: a gc- or
// resource-category reference is shared (the copy keeps the pointer), a
// value-category reference is copied with its owner (deep — "share
// nothing" reaches every level). The new object is rooted like any other
// construction, and its push is the body's to pop.
func (e *emitter) emitRecCopy(src, recKey string) string {
	rec, ok := e.records[recKey]
	if !ok {
		return src
	}
	slots, total, ok := e.layout(recModKey(recKey), rec)
	if !ok {
		return src
	}
	e.usedRecs[recKey] = true
	dst := e.allocRecord(recKey, total)
	for _, s := range slots {
		switch s.kind {
		case fkStr:
			e.gepStore(dst, s.off, "ptr "+e.gepLoadPtr(src, s.off))
			e.gepStore(dst, s.off+8, "i64 "+e.gepLoadI64(src, s.off+8))
		case fkScalar:
			e.gepStore(dst, s.off, "i64 "+e.gepLoadI64(src, s.off))
		case fkF64:
			e.gepStore(dst, s.off, "double "+e.gepLoadDouble(src, s.off))
		case fkRef:
			e.gepStore(dst, s.off, "ptr "+e.gepLoadPtr(src, s.off))
		case fkVal:
			e.gepStore(dst, s.off, "ptr "+e.emitRecCopy(e.gepLoadPtr(src, s.off), s.typ))
		}
	}
	return dst
}

// splitIntSuffix splits an integer literal's digits from its type suffix
// (chapter 3's inventory); the suffix is "" where the literal spells none.
func splitIntSuffix(text string) (string, string) {
	for _, suf := range []string{"i64", "i32", "i16", "i8", "u64", "u32", "u16", "u8"} {
		if strings.HasSuffix(text, suf) {
			return text[:len(text)-len(suf)], suf
		}
	}
	return text, ""
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
		text, suf := splitIntSuffix(lit.Text)
		n, err := strconv.ParseInt(text, 0, 64)
		if err != nil {
			if suf == "" || suf[0] != 'u' {
				return "", false
			}
			// A u64 magnitude past the i64 inventory still has a register
			// form: the same bits, written as the signed reading of them
			// (the i64 operand is a bit pattern, design D2).
			u, uerr := strconv.ParseUint(text, 0, 64)
			if uerr != nil {
				return "", false
			}
			return strconv.FormatInt(int64(u), 10), true
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
	if e.diverged {
		return nil // the entry branch already ended in unreachable — no tail ret
	}
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
		if ni := e.drainDefers(); ni != nil {
			return ni
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
	// abiTuple crosses a boundary as its elements' bare words — one IR
	// parameter (or one returned field) per word, never a pointer (design
	// D4's 传参逐字段展开). In the body it is a stack aggregate, which is
	// what the value IS: a tuple never leaves the stack.
	abiTuple
	// abiFn is a function value at a parameter position (chapter 12's
	// apply(f, v)): the one carrier pointer design D5 makes a fn value,
	// carrying its own signature statically (fnParamAbi.sig) so the body
	// may call through it.
	abiFn
)

type fnParamAbi struct {
	kind  fnAbiKind
	key   string      // the record/sum's module-qualified key (abiGc/abiSum)
	typ   string      // the declared base type name, where the parameter names one
	elems []tupleElem // the element shapes (abiTuple)
	sig   *fnAbi      // the parameter's own signature (abiFn)
}

// fnAbi is one fn's calling shape: the return family plus one entry per
// source parameter (a String or sum parameter takes two IR words).
type fnAbi struct {
	ret      fnAbiKind
	retTyp   string      // the define's result type spelling
	retKey   string      // the ret record/sum's key ("Result" for the prelude sum)
	retName  string      // the declared base type name, where the return names one
	variants []string    // the ret sum's variant names, decl order (abiSum)
	elems    []tupleElem // the returned tuple's element shapes (abiTuple)
	params   []fnParamAbi
}

// fnAbiOf classifies one fn's signature against the walked module's
// tables (enter the fn's module first). A type the families cannot name —
// a qualified reference, a generic application, an unknown name —
// reports false and the caller stops at the fn body word.
// newtypeOf reports the module-qualified key when t names a newtype the
// walked module declares — the wrapper design D4 erases.
func (e *emitter) newtypeOf(t ast.TypeRef) (string, bool) {
	n, ok := t.(*ast.NamedType)
	if !ok || n.Qual != "" {
		return "", false
	}
	key := e.curKey + "." + n.Name
	if _, ok := e.newtypes[key]; !ok {
		return "", false
	}
	return key, true
}

// derefNewtype strips a newtype wrapper from a type reference: a value of
// the wrapper IS a value of the underlying (chapter 8's zero-cost promise,
// design D4's erasure), so every classification reads through the chain.
// A self-referential declaration (which the check stage rejects) stops the
// walk rather than the compiler.
func (e *emitter) derefNewtype(t ast.TypeRef) ast.TypeRef {
	seen := make(map[string]bool)
	for {
		key, ok := e.newtypeOf(t)
		if !ok || seen[key] {
			return t
		}
		seen[key] = true
		t = e.newtypes[key]
	}
}

// classType classifies one type reference: the ABI family it crosses a
// boundary as, and the module-qualified key a record or sum carries. The
// tables are the walked module's, so the caller enters that module first
// (classify does for a fn, emitNewtypeCtor for a construction).
func (e *emitter) classType(t ast.TypeRef) (fnAbiKind, string, bool) {
	t = e.derefNewtype(t)
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

func (e *emitter) fnAbiOf(d *ast.FnDecl) (fnAbi, bool) {
	return e.fitAbi(d.Ret, d.Params)
}

// fitAbi classifies one signature — a declared fn's or a closure's — into
// the calling shape its define and its call sites share.
func (e *emitter) fitAbi(ret ast.TypeRef, params []ast.Param) (fnAbi, bool) {
	class := e.classType
	var abi fnAbi
	switch t := ret.(type) {
	case nil:
		abi.ret = abiVoid
		abi.retTyp = "void"
	case *ast.TupleType:
		elems, ok := e.tupleShapeOf(t)
		if !ok {
			return abi, false
		}
		abi.ret, abi.retTyp, abi.elems = abiTuple, aggTyp(elems), elems
		return e.bindTupleParams(&abi, params)
	case *ast.NamedType:
		k, key, ok := class(t)
		if !ok {
			return abi, false
		}
		abi.ret, abi.retKey = k, key
		abi.retName = baseTypeName(e.derefNewtype(t))
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
	return e.bindTupleParams(&abi, params)
}

// paramsOfType turns one function type's parameter list into the Param
// shape the signature classifier takes (the type is all a fn value's own
// signature carries — no name, nothing else).
func paramsOfType(ft *ast.FnType) []ast.Param {
	ps := make([]ast.Param, len(ft.Params))
	for i, t := range ft.Params {
		ps[i] = ast.Param{Type: t}
	}
	return ps
}

// bindTupleParams classifies one signature's parameters and fills abi's
// parameter table — the shared tail of fitAbi, reached from both return
// paths (a tuple return returns early).
func (e *emitter) bindTupleParams(abi *fnAbi, params []ast.Param) (fnAbi, bool) {
	for _, p := range params {
		if tt, ok := p.Type.(*ast.TupleType); ok {
			elems, ok := e.tupleShapeOf(tt)
			if !ok {
				return *abi, false
			}
			abi.params = append(abi.params, fnParamAbi{kind: abiTuple, elems: elems})
			continue
		}
		if ft, ok := p.Type.(*ast.FnType); ok {
			// A fn-typed parameter (chapter 12): its own signature is a
			// static fact of the declaration, so the body's `f(v)` knows
			// the shape it calls. A signature the families cannot name —
			// an effect segment, a generic application, a nested fn
			// parameter this build does not carry — reports false.
			if len(ft.EffectTags) != 0 {
				return *abi, false
			}
			sig, ok := e.fitAbi(ft.Ret, paramsOfType(ft))
			if !ok {
				return *abi, false
			}
			abi.params = append(abi.params, fnParamAbi{kind: abiFn, sig: &sig})
			continue
		}
		k, key, ok := e.classType(p.Type)
		if !ok {
			return *abi, false
		}
		abi.params = append(abi.params, fnParamAbi{kind: k, key: key, typ: baseTypeName(e.derefNewtype(p.Type))})
	}
	return *abi, true
}

// tupleShapeOf lays out a tuple type's elements: the same words each
// element crosses a boundary with, at their offsets in one aggregate.
func (e *emitter) tupleShapeOf(t *ast.TupleType) ([]tupleElem, bool) {
	var elems []tupleElem
	off := 0
	for _, et := range t.Elems {
		k, key, ok := e.classType(et)
		if !ok {
			return nil, false
		}
		el := tupleElem{kind: k, key: key, typ: baseTypeName(e.derefNewtype(et)), off: off}
		switch k {
		case abiI64:
			el.words = []string{"i64"}
		case abiDouble:
			el.words = []string{"double"}
		case abiGc:
			el.words = []string{"ptr"}
		case abiStr:
			el.words = []string{"ptr", "i64"}
		case abiSum:
			el.words = []string{"i64", "i64"}
		default:
			return nil, false
		}
		off += 8 * len(el.words)
		elems = append(elems, el)
	}
	if len(elems) < 2 {
		return nil, false // `( e )` is a grouping and `()` is unit
	}
	return elems, true
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
		return fmt.Sprintf("@%s.fxgate", fd.sym())
	}
	return "@" + fd.sym()
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
	call := fmt.Sprintf("call %s @%s(%s)", abi.retTyp, fd.sym(),
		strings.Join(ps, ", "))
	var body strings.Builder
	fmt.Fprintf(&body, "  call void @__we_explore_fx_check(ptr %s, i64 %d)\n", name, len(fd.name))
	if abi.ret == abiVoid {
		fmt.Fprintf(&body, "  %s\n  ret void\n", call)
	} else {
		fmt.Fprintf(&body, "  %%r = %s\n  ret %s %%r\n", call, abi.retTyp)
	}
	e.thunks = append(e.thunks, fmt.Sprintf(
		"define internal %s @%s.fxgate(%s) {\nentry:\n%s}\n",
		abi.retTyp, fd.sym(), strings.Join(ps, ", "), body.String()))
}

// --- chapter 19: the foreign ABI (M12 design D4) -----------------------------------
//
// The We domain carries every integer as i64 and every float as double;
// the C side takes its own widths. Each crossing kind names its C
// spelling plus the conversion each direction needs — trunc/sext or
// zext for the integers (signedness per kind), fptrunc/fpext for the
// single-precision float; the i64 and double rows are identity. String
// and Bytes are not kinds: they expand to the (ptr, i64) pair in
// parameter position and never return (E1706). Never is not a kind
// either — a void return position whose call diverges. Opaque records
// are table lookups, not spellings: a single ptr.

type foreignKind struct {
	abi    string // the C-side IR spelling
	narrow string // the conversion a We-domain argument takes ("" when none)
	widen  string // the conversion a C return takes back ("" when none)
	float  bool   // the We domain carries this kind as a double
}

var foreignKinds = map[string]foreignKind{
	"Int8":    {abi: "i8", narrow: "trunc", widen: "sext"},
	"UInt8":   {abi: "i8", narrow: "trunc", widen: "zext"},
	"Int16":   {abi: "i16", narrow: "trunc", widen: "sext"},
	"UInt16":  {abi: "i16", narrow: "trunc", widen: "zext"},
	"Int32":   {abi: "i32", narrow: "trunc", widen: "sext"},
	"UInt32":  {abi: "i32", narrow: "trunc", widen: "zext"},
	"Rune":    {abi: "i32", narrow: "trunc", widen: "zext"},
	"Int64":   {abi: "i64"},
	"UInt64":  {abi: "i64"},
	"Bool":    {abi: "i8", narrow: "trunc", widen: "zext"},
	"Float32": {abi: "float", narrow: "fptrunc", widen: "fpext", float: true},
	"Float64": {abi: "double", float: true},
}

// bareTypeName names t when it is an unqualified, unapplied reference —
// the spelling the crossing kinds key on.
func bareTypeName(t ast.TypeRef) string {
	if n, ok := t.(*ast.NamedType); ok && n.Qual == "" && len(n.Args) == 0 {
		return n.Name
	}
	return ""
}

// opaqueKeyOf names the foreign opaque record a type reference denotes:
// the walked module's own name, or a qualified one through its imports
// (the no-import module-key fallback resolveQual carries). It is the
// opaque table's one reader — `isOpaqueRef` for the yes-or-no question,
// the scope resource head for the key a release dispatches under.
func (e *emitter) opaqueKeyOf(t ast.TypeRef) (string, bool) {
	n, ok := t.(*ast.NamedType)
	if !ok || len(n.Args) != 0 {
		return "", false
	}
	if n.Qual == "" {
		key := e.curKey + "." + n.Name
		return key, e.opaques[key]
	}
	if k := e.resolveQual(n.Qual); k != "" {
		key := k + "." + n.Name
		return key, e.opaques[key]
	}
	return "", false
}

// isOpaqueRef reports whether t names a foreign opaque record.
func (e *emitter) isOpaqueRef(t ast.TypeRef) bool {
	_, ok := e.opaqueKeyOf(t)
	return ok
}

// foreignRet is a foreign entry's return face: the call's IR result
// spelling, the crossing kind a widening return widens back through
// (zero value when none), and the two special families — an opaque's
// single ptr and Never's void, the return position that only diverges.
type foreignRet struct {
	typ    string
	k      foreignKind
	opaque bool
	never  bool
}

// foreignRetOf classifies one entry's declared return: no return and
// Never are void (Never alone diverges), the crossing kinds take their
// C spelling, an opaque takes a single ptr.
func (e *emitter) foreignRetOf(t ast.TypeRef) (foreignRet, bool) {
	if t == nil {
		return foreignRet{typ: "void"}, true
	}
	name := bareTypeName(t)
	if name == "Never" {
		return foreignRet{typ: "void", never: true}, true
	}
	if k, ok := foreignKinds[name]; ok {
		return foreignRet{typ: k.abi, k: k}, true
	}
	if e.isOpaqueRef(t) {
		return foreignRet{typ: "ptr", opaque: true}, true
	}
	return foreignRet{}, false
}

// foreignDeclare renders one entry's declare line — the whole native
// contract per the positional ABI map: each scalar at its C width,
// String and Bytes as their (ptr, i64) pair, an opaque as a single ptr,
// no return as void, Never as a void only ever seen diverging.
func (e *emitter) foreignDeclare(f *ast.FnDecl) (string, bool) {
	ret, ok := e.foreignRetOf(f.Ret)
	if !ok {
		return "", false
	}
	var ps []string
	for _, p := range f.Params {
		if k, ok := foreignKinds[bareTypeName(p.Type)]; ok {
			ps = append(ps, k.abi)
			continue
		}
		switch bareTypeName(p.Type) {
		case "String", "Bytes":
			ps = append(ps, "ptr", "i64")
		default:
			if !e.isOpaqueRef(p.Type) {
				return "", false // check owns the crossing set; unreachable defense
			}
			ps = append(ps, "ptr")
		}
	}
	return fmt.Sprintf("declare %s @%s(%s)", ret.typ, f.Name, strings.Join(ps, ", ")), true
}

// emitForeignCall emits one foreign call: no slot, no module-key
// qualifier — the declared name is the C symbol itself (the Q3 ruling:
// names under chapter 1's conventions pass through unmangled). The
// arguments ride the positional ABI map — each scalar narrowed out of
// the We domain, a String as its (ptr, i64) pair, an opaque as its
// single ptr — and the result widens back. A Never return ends the
// branch: unreachable is the terminator and the body stops emitting
// (diverged).
func (e *emitter) emitForeignCall(fd *fnDef, args []ast.Expr) (callResult, *NotImplemented) {
	if len(args) != len(fd.decl.Params) {
		return callResult{}, e.bnd()
	}
	ret, ok := e.foreignRetOf(fd.decl.Ret)
	if !ok {
		return callResult{}, bndFn()
	}
	var ops []string
	for i, a := range args {
		p := fd.decl.Params[i]
		if k, is := foreignKinds[bareTypeName(p.Type)]; is {
			op, ni := e.foreignScalarArg(a, k)
			if ni != nil {
				return callResult{}, ni
			}
			ops = append(ops, k.abi+" "+op)
			continue
		}
		switch bareTypeName(p.Type) {
		case "String":
			// The buffer face: a String argument rides its (ptr, i64)
			// pair, a nested call's String result alike.
			if c, is := a.(*ast.Call); is {
				res, ni := e.emitCall(c, nil)
				if ni != nil {
					return callResult{}, ni
				}
				if res.kind != ckStr {
					return callResult{}, e.bnd()
				}
				ops = append(ops, "ptr "+res.strBind.dataOp, "i64 "+res.strBind.lenOp)
				continue
			}
			d, l, ni := e.emitStringExpr(a)
			if ni != nil {
				return callResult{}, ni
			}
			ops = append(ops, "ptr "+d, "i64 "+l)
		case "Bytes":
			// Bytes has no We-side producer yet (the B2 stdlib face
			// carries it); the declare rides the map, the value face
			// waits for a value to carry.
			return callResult{}, e.bnd()
		default:
			if !e.isOpaqueRef(p.Type) {
				return callResult{}, bndFn()
			}
			// The handle face: a bound opaque is a pointer in the
			// environment, a nested foreign call's return is the same
			// face fresh.
			switch v := a.(type) {
			case *ast.Ident:
				// An opaque the body holds under a record-ish binding:
				// a method's receiver (an opaque head's only handle on
				// itself — chapter 19's release idiom passes it straight
				// to the native close, the type having no field to pass
				// instead) and a scope resource head (the call's fresh
				// result) both bind in the gc environment. Chapter 19's
				// other route in — a plain let of a foreign call's
				// return — lands in the prim table below.
				if g, is := e.gcEnv[v.Name]; is {
					if !e.opaques[g.rec] {
						return callResult{}, e.bnd()
					}
					ops = append(ops, "ptr "+g.reg)
					break
				}
				ptr, is := e.prims[v.Name]
				if !is {
					return callResult{}, e.bnd()
				}
				ops = append(ops, "ptr "+ptr)
			case *ast.Call:
				res, ni := e.emitCall(v, nil)
				if ni != nil {
					return callResult{}, ni
				}
				if res.kind != ckPrim {
					return callResult{}, e.bnd()
				}
				ops = append(ops, "ptr "+res.i64)
			default:
				return callResult{}, e.bnd()
			}
		}
	}
	join := strings.Join(ops, ", ")
	sym := "@" + fd.decl.Name
	switch {
	case ret.never:
		e.inst(fmt.Sprintf("call void %s(%s)", sym, join))
		e.inst("unreachable")
		e.diverged = true
		return callResult{kind: ckVoid}, nil
	case ret.typ == "void":
		e.inst(fmt.Sprintf("call void %s(%s)", sym, join))
		return callResult{kind: ckVoid}, nil
	case ret.opaque:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call ptr %s(%s)", v, sym, join))
		return callResult{kind: ckPrim, i64: "%" + v}, nil
	case ret.k.abi == "double":
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call double %s(%s)", v, sym, join))
		return callResult{kind: ckI64, i64: "%" + v, isFloat: true}, nil
	default:
		// Integers, Bool, Rune arrive at their C width and widen back
		// into the We i64 domain — sext or zext per the kind's
		// signedness.
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call %s %s(%s)", v, ret.k.abi, sym, join))
		if ret.k.widen == "" {
			return callResult{kind: ckI64, i64: "%" + v}, nil
		}
		w := e.value()
		e.inst(fmt.Sprintf("%%%s = %s %s %%%s to i64", w, ret.k.widen, ret.k.abi, v))
		return callResult{kind: ckI64, i64: "%" + w}, nil
	}
}

// foreignScalarArg emits one scalar argument in its C domain: a nested
// call rides its result when the float/int families agree, otherwise
// the expression must already sit in the We domain the kind maps from.
func (e *emitter) foreignScalarArg(a ast.Expr, k foreignKind) (string, *NotImplemented) {
	if c, ok := a.(*ast.Call); ok {
		res, ni := e.emitCall(c, nil)
		if ni != nil {
			return "", ni
		}
		if res.kind != ckI64 || res.isFloat != k.float {
			return "", e.bnd()
		}
		return e.narrowForeign(res.i64, k)
	}
	op, isF, ni := e.emitNumExpr(a)
	if ni != nil {
		return "", ni
	}
	if isF != k.float {
		return "", e.bnd()
	}
	return e.narrowForeign(op, k)
}

// narrowForeign converts one We-domain operand into the kind's C width —
// trunc for the integers, fptrunc for the single-precision float; the
// i64 and double rows carry no conversion.
func (e *emitter) narrowForeign(op string, k foreignKind) (string, *NotImplemented) {
	if k.narrow == "" {
		return op, nil
	}
	from := "i64"
	if k.float {
		from = "double"
	}
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = %s %s %s to %s", v, k.narrow, from, op, k.abi))
	return "%" + v, nil
}

// emitBodyCore emits one function body's statements and its return
// protocol — the tail item (a `return`, or the implicit value a declared
// return type gives the trailing expression, chapter 6), the deferred
// blocks in inversion, and the body's own gc window's pops — and returns
// the body text with the return line's operand. Every body that answers a
// return protocol shares it: the program fn, the method, the mock and
// test defines, and (chapter 12's 闭包体即函数体) every closure thunk. A
// body that diverged reports it through diverged instead: its terminator
// stands as the define's own, with no return, defers, or pops.
func (e *emitter) emitBodyCore(items []ast.Stmt, abi fnAbi, implicitTail bool) (string, string, bool, *NotImplemented) {
	var tail *ast.Return
	if len(items) > 0 {
		switch last := items[len(items)-1].(type) {
		case *ast.Return:
			tail = last
			items = items[:len(items)-1]
		case *ast.ExprStmt:
			// Chapter 6: it is a declared return type that makes the
			// body value the function's implicit return ("声明了返回
			// 类型时，体块值是函数的隐式返回值：末项表达式返回它").
			// Without one the trailing item is governed by chapter 8's
			// value-discard rule instead — a plain statement whose
			// value no one takes, unit needing no ceremony — so it
			// stays in items and emits through emitStmt like any other.
			if !implicitTail {
				break
			}
			tail = &ast.Return{HasValue: true, Value: last.Expr}
			items = items[:len(items)-1]
		}
	}
	for _, st := range items {
		if e.diverged {
			break // a Never call already terminated the body
		}
		if ni := e.emitStmt(st); ni != nil {
			return "", "", false, ni
		}
	}
	if e.diverged {
		return e.bodyText(), "", true, nil
	}
	ret, ni := e.fnRetVal(abi, tail)
	if ni != nil {
		return "", "", false, ni
	}
	if ni := e.drainDefers(); ni != nil {
		return "", "", false, ni
	}
	for i := 0; i < e.pushes; i++ {
		e.use("__we_root_pop")
		e.inst("call void @__we_root_pop()")
	}
	return e.bodyText(), ret, false, nil
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
	savedCur := e.curBlock
	savedAllocas, savedAssigned := e.allocas, e.assigned
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedFns := e.fnEnv
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedList := e.listEnv
	savedTup, savedNt := e.tupEnv, e.ntEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	savedFrames := e.frames
	savedDiverged := e.diverged
	savedExit, savedInExit := e.exit, e.inExit
	savedLoops, savedScopes := e.loopFrames, e.scopeLive
	savedRes, savedNest := e.resFrames, e.nest
	restore := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.fnEnv = savedFns
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.listEnv = savedList
		e.tupEnv, e.ntEnv = savedTup, savedNt
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
		e.frames = savedFrames
		e.diverged, e.curBlock = savedDiverged, savedCur
		e.exit, e.inExit = savedExit, savedInExit
		e.loopFrames, e.scopeLive = savedLoops, savedScopes
		e.resFrames, e.nest = savedRes, savedNest
		e.allocas, e.assigned = savedAllocas, savedAssigned
	}
	e.ctx = ctxFn
	e.beginBody()
	e.allocas = nil
	e.assigned = make(map[string]bool)
	e.diverged = false
	e.scalars = make(map[string]scalarSlot)
	e.fnEnv = make(map[string]fnValue)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.listEnv = make(map[string]listBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.tupEnv = make(map[string]tupBinding)
	e.ntEnv = make(map[string]string)
	e.defers = nil
	e.caps = nil
	e.pushes = 0
	// The body's return protocol (T2 deep returns): every return in this
	// define answers the declared ABI, whatever its depth. The loop and
	// scope stacks start empty — neither a break nor a return crosses a
	// body boundary.
	e.exit = &exitSite{kind: exitFn, abi: abi}
	e.loopFrames, e.scopeLive, e.inExit = nil, nil, false
	e.resFrames, e.nest = nil, 0
	// Which names this body writes is known before anything emits: a
	// parameter among them takes a slot instead of its incoming register.
	collectAssigned(fd.decl.Body.Items, e.assigned)

	// The parameter list and environments (the shared define face). A
	// method's receiver rides ahead of the declared parameters: the head's
	// record pointer, and the `self` binding every `self.field` and
	// `self.m()` in the body resolves through.
	ps := e.bindDefineParams(fd.decl.Params, abi)
	if fd.recvKey != "" {
		ps = append([]string{"ptr %self"}, ps...)
		e.gcEnv["self"] = gcBinding{rec: fd.recvKey, reg: "%self"}
	}

	// The statements and the return protocol (the shared body core).
	body, ret, diverged, ni := e.emitBodyCore(fd.decl.Body.Items, abi, fd.decl.Ret != nil)
	if ni != nil {
		restore()
		return ni
	}
	if diverged {
		// The body ended in unreachable (a Never-returning foreign
		// call): no return value, no defers, no pops — the terminator
		// stands as the define's own.
		restore()
		e.fnsDone = append(e.fnsDone, fmt.Sprintf(
			"define %s @%s(%s) {\nentry:\n%s}\n",
			abi.retTyp, fd.sym(), strings.Join(ps, ", "), body))
		if isCustomEffect(fd.decl) {
			e.emitFxGate(fd, abi) // the gate's own call never diverges; its callee does, at runtime
		}
		return nil
	}
	restore()
	e.fnsDone = append(e.fnsDone, fmt.Sprintf(
		"define %s @%s(%s) {\nentry:\n%s  ret %s\n}\n",
		abi.retTyp, fd.sym(), strings.Join(ps, ", "), body, ret))
	// A custom-effect fn's default face is its gate, emitted behind the
	// real define under the real define's unchanged name (M10c D6).
	if isCustomEffect(fd.decl) {
		e.emitFxGate(fd, abi)
	}
	if fd.recvKey == "" {
		// Program fns ride a slot so a mock can swap the pointer
		// (design D3). A method's dispatch is static — its call sites
		// name the symbol directly (emitMethodCall) — so a slot here
		// would be a global nothing reads.
		e.slotFor(fd.sym(), fnSlotTarget(fd))
	}
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
		if key, ok := e.newtypeOf(p.Type); ok && p.Name != "_" {
			// The parameter IS its underlying at the boundary (design
			// D4's erasure); the name remembers the wrapper, which is
			// what `.value` unwraps.
			e.ntEnv[p.Name] = key
		}
		switch pa.kind {
		case abiI64:
			ps = append(ps, "i64 %"+p.Name)
			if p.Name != "_" {
				// The declaration named the parameter's base type; the
				// binding carries it (design D3's interpolation domain:
				// `"${n}"` renders the parameter as its own type).
				if e.assigned[p.Name] {
					e.bindScalarSlot(p.Name, "%"+p.Name, false, baseStrKind(pa.typ))
				} else {
					e.scalars[p.Name] = scalarSlot{operand: "%" + p.Name, kind: baseStrKind(pa.typ)}
				}
			}
		case abiDouble:
			ps = append(ps, "double %"+p.Name)
			if p.Name != "_" {
				if e.assigned[p.Name] {
					e.bindScalarSlot(p.Name, "%"+p.Name, true, baseStrKind(pa.typ))
				} else {
					e.scalars[p.Name] = scalarSlot{operand: "%" + p.Name, isFloat: true, kind: baseStrKind(pa.typ)}
				}
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
		case abiTuple:
			// The words arrive flat (design D4's 传参逐字段展开) and the
			// body sees the aggregate they rebuild: a tuple value is a
			// stack aggregate wherever it is used.
			agg := e.slot(aggTyp(pa.elems))
			w := 0
			for _, el := range pa.elems {
				for i, wt := range el.words {
					reg := fmt.Sprintf("%%%s.%d", p.Name, w)
					ps = append(ps, wt+" "+reg)
					e.gepStore(agg, el.off+8*i, wt+" "+reg)
					w++
				}
			}
			if p.Name != "_" {
				e.tupEnv[p.Name] = tupBinding{ptr: agg, elems: pa.elems}
			}
		case abiSum:
			ps = append(ps, "i64 %"+p.Name+"0", "i64 %"+p.Name+"1")
			if p.Name != "_" {
				ts := e.slot("i64")
				e.inst(fmt.Sprintf("store i64 %%%s0, ptr %s", p.Name, ts))
				pp := e.slot("i64")
				e.inst(fmt.Sprintf("store i64 %%%s1, ptr %s", p.Name, pp))
				e.sums2[p.Name] = sumSlot{tag: ts, pay: pp, variants: e.sumsOrd[pa.key]}
			}
		case abiFn:
			ps = append(ps, "ptr %"+p.Name)
			if p.Name != "_" {
				e.fnEnv[p.Name] = fnValue{carrier: "%" + p.Name, abi: *pa.sig, typed: true}
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
	savedCur := e.curBlock
	savedAllocas, savedAssigned := e.allocas, e.assigned
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedFns := e.fnEnv
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedList := e.listEnv
	savedTup, savedNt := e.tupEnv, e.ntEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	savedFrames := e.frames
	savedDiverged := e.diverged
	savedExit, savedInExit := e.exit, e.inExit
	savedLoops, savedScopes := e.loopFrames, e.scopeLive
	savedRes, savedNest := e.resFrames, e.nest
	restoreState := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.fnEnv = savedFns
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.listEnv = savedList
		e.tupEnv, e.ntEnv = savedTup, savedNt
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
		e.frames = savedFrames
		e.diverged, e.curBlock = savedDiverged, savedCur
		e.exit, e.inExit = savedExit, savedInExit
		e.loopFrames, e.scopeLive = savedLoops, savedScopes
		e.resFrames, e.nest = savedRes, savedNest
		e.allocas, e.assigned = savedAllocas, savedAssigned
	}
	e.ctx = ctxFn
	e.beginBody()
	e.allocas = nil
	e.assigned = make(map[string]bool)
	e.diverged = false
	e.scalars = make(map[string]scalarSlot)
	e.fnEnv = make(map[string]fnValue)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.listEnv = make(map[string]listBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.tupEnv = make(map[string]tupBinding)
	e.ntEnv = make(map[string]string)
	e.defers = nil
	e.caps = nil
	e.pushes = 0
	e.exit = &exitSite{kind: exitFn, abi: abi}
	e.loopFrames, e.scopeLive, e.inExit = nil, nil, false
	e.resFrames, e.nest = nil, 0

	collectAssigned(md.Body.Items, e.assigned)
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
		if e.diverged {
			break // a Never call already terminated the mock body
		}
		if ni := e.emitStmt(st); ni != nil {
			restoreState()
			return mockInstall{}, ni
		}
	}
	if e.diverged {
		// A diverged mock body ends at its unreachable, no value tail.
		body := e.bodyText()
		restoreState()
		sym := fmt.Sprintf("%s.mock.%d", key, n)
		e.fnsDone = append(e.fnsDone, fmt.Sprintf(
			"define %s @%s(%s) {\nentry:\n%s}\n",
			abi.retTyp, sym, strings.Join(ps, ", "), body))
		return mockInstall{slot: slot, mock: "@" + sym, restore: restore}, nil
	}
	ret, ni := e.fnRetVal(abi, tail)
	if ni != nil {
		restoreState()
		return mockInstall{}, ni
	}
	if ni := e.drainDefers(); ni != nil {
		restoreState()
		return mockInstall{}, ni
	}
	for i := 0; i < e.pushes; i++ {
		e.use("__we_root_pop")
		e.inst("call void @__we_root_pop()")
	}
	body := e.bodyText()
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
	savedCur := e.curBlock
	savedAllocas, savedAssigned := e.allocas, e.assigned
	savedScalars, savedSums, savedPrims := e.scalars, e.sums2, e.prims
	savedFns := e.fnEnv
	savedStr, savedGc := e.strEnv, e.gcEnv
	savedList := e.listEnv
	savedTup, savedNt := e.tupEnv, e.ntEnv
	savedPushes, savedDefers, savedCaps := e.pushes, e.defers, e.caps
	savedFrames := e.frames
	savedDiverged := e.diverged
	savedExit, savedInExit := e.exit, e.inExit
	savedLoops, savedScopes := e.loopFrames, e.scopeLive
	savedRes, savedNest := e.resFrames, e.nest
	restore := func() {
		e.ctx, e.body = savedCtx, savedBody
		e.scalars, e.sums2, e.prims = savedScalars, savedSums, savedPrims
		e.fnEnv = savedFns
		e.strEnv, e.gcEnv = savedStr, savedGc
		e.listEnv = savedList
		e.tupEnv, e.ntEnv = savedTup, savedNt
		e.pushes, e.defers, e.caps = savedPushes, savedDefers, savedCaps
		e.frames = savedFrames
		e.diverged, e.curBlock = savedDiverged, savedCur
		e.exit, e.inExit = savedExit, savedInExit
		e.loopFrames, e.scopeLive = savedLoops, savedScopes
		e.resFrames, e.nest = savedRes, savedNest
		e.allocas, e.assigned = savedAllocas, savedAssigned
	}
	e.ctx = ctxFn
	e.beginBody()
	e.allocas = nil
	e.assigned = make(map[string]bool)
	e.diverged = false
	e.scalars = make(map[string]scalarSlot)
	e.fnEnv = make(map[string]fnValue)
	e.sums2 = make(map[string]sumSlot)
	e.prims = make(map[string]string)
	e.strEnv = make(map[string]strBinding)
	e.listEnv = make(map[string]listBinding)
	e.gcEnv = make(map[string]gcBinding)
	e.tupEnv = make(map[string]tupBinding)
	e.ntEnv = make(map[string]string)
	e.defers = nil
	e.caps = nil
	e.pushes = 0
	// The test context's return is valueless: a return at any depth emits
	// ret void (with the exit sequence ahead of it).
	e.exit = &exitSite{kind: exitTest}
	e.loopFrames, e.scopeLive, e.inExit = nil, nil, false
	e.resFrames, e.nest = nil, 0
	collectAssigned(td.Body.Items, e.assigned)

	for _, st := range td.Body.Items {
		if e.diverged {
			break // a Never call already terminated the test body
		}
		if _, ok := st.(*ast.MockDecl); ok {
			continue // its define rode ahead of this walk
		}
		if ni := e.emitStmt(st); ni != nil {
			restore()
			return ni
		}
	}
	if e.diverged {
		// A diverged test body ends at its unreachable — no defers, no
		// pops, no ret void (the Never call took the process before any
		// assertion could fail).
		body := e.bodyText()
		restore()
		e.fnsDone = append(e.fnsDone, fmt.Sprintf(
			"define void @%s.test.%d() {\nentry:\n%s}\n", key, n, body))
		return nil
	}
	if ni := e.drainDefers(); ni != nil {
		restore()
		return ni
	}
	for i := 0; i < e.pushes; i++ {
		e.use("__we_root_pop")
		e.inst("call void @__we_root_pop()")
	}
	body := e.bodyText()
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
		savedBody, savedAllocas := e.body, e.allocas
		savedAssigned, savedCur := e.assigned, e.curBlock
		e.beginBody()
		e.allocas, e.assigned = nil, make(map[string]bool)
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
		pay := e.slot("i64")
		tag := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 @__we_handle_await(ptr %%%s, ptr %s)", tag, h, pay))
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
		e.inst(fmt.Sprintf("%%%s = load i64, ptr %s", r, pay))
		p := e.value()
		e.inst(fmt.Sprintf("%%%s = inttoptr i64 %%%s to ptr", p, r))
		e.use("__we_test_report")
		e.inst(fmt.Sprintf("call void @__we_test_report(ptr %s, i64 %d, ptr %s, i64 %d, i64 1, ptr %%%s)",
			f, len(st.file), d, len(st.desc), p))
		e.inst(fmt.Sprintf("br label %%tk%dq", n))
		e.label(fmt.Sprintf("tk%dq", n))
		body := e.bodyText()
		e.body, e.allocas, e.assigned = savedBody, savedAllocas, savedAssigned
		e.curBlock = savedCur
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
	if tail == nil {
		return e.fnRetOperand(abi, nil, false)
	}
	// A valueless tail return is the absent tail for every family (a void
	// fn's optional return); a value-shaped family rejects it below.
	if !tail.HasValue {
		return e.fnRetOperand(abi, nil, false)
	}
	return e.fnRetOperand(abi, tail.Value, true)
}

// fnRetOperand renders one returned value as the ret instruction's
// operand text (the ABI table's value faces): the tail and every deep
// return share this face. hasValue false is the absent tail — legal only
// where the ABI answers void.
func (e *emitter) fnRetOperand(abi fnAbi, value ast.Expr, hasValue bool) (string, *NotImplemented) {
	switch abi.ret {
	case abiVoid:
		if hasValue {
			return "", bndFn()
		}
		return "void", nil
	case abiI64, abiDouble:
		if !hasValue {
			return "", bndFn()
		}
		// The tail return's value face includes a direct call of the
		// family's own ABI (the M10b goldens return fn results straight
		// through: return double(double(n))). The M9b expression
		// emitters stay call-free; the call resolves here, computing at
		// the same pre-defer position any other value face does.
		if c, ok := value.(*ast.Call); ok {
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
		op, isF, ni := e.emitNumExpr(value)
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
	case abiTuple:
		if !hasValue {
			return "", bndFn()
		}
		// The returned tuple is the multi-value return: the same
		// register-level insertvalue face the String pair rides, one
		// field per word (design D4).
		ptr, elems, ni := e.emitTupleOperand(value)
		if ni != nil {
			return "", ni
		}
		if aggTyp(elems) != abi.retTyp {
			return "", bndFn()
		}
		cur, w := "undef", 0
		for _, el := range elems {
			for i, wt := range el.words {
				at := el.off + 8*i
				var op string
				switch wt {
				case "i64":
					op = "i64 " + e.gepLoadI64(ptr, at)
				case "double":
					op = "double " + e.gepLoadDouble(ptr, at)
				case "ptr":
					op = "ptr " + e.gepLoadPtr(ptr, at)
				}
				v := e.value()
				e.inst(fmt.Sprintf("%%%s = insertvalue %s %s, %s, %d", v, abi.retTyp, cur, op, w))
				cur, w = "%"+v, w+1
			}
		}
		return abi.retTyp + " " + cur, nil
	case abiStr:
		if !hasValue {
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
		switch v := value.(type) {
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
			if len(v.Holes) > 0 {
				// A hole's pair is a register: the aggregate builds
				// through insertvalue like every other operand pair.
				res, ni := e.emitInterp(v)
				if ni != nil {
					return "", ni
				}
				return strPair(res.strBind.dataOp, res.strBind.lenOp), nil
			}
			data, ok := decodeStringLiteral(v.Text)
			if !ok {
				return "", bndFn()
			}
			return fmt.Sprintf("{ ptr, i64 } { ptr %s, i64 %d }", e.intern(data), len(data)), nil
		case *ast.Binary, *ast.Member:
			// A returned concatenation, and any member read that lands in
			// the String domain — a String field, a newtype's `.value`
			// unwrap (T5): the pair faces cover both.
			p, l, ni := e.emitStringExpr(v)
			if ni != nil {
				return "", ni
			}
			return strPair(p, l), nil
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
		if !hasValue {
			return "", bndFn()
		}
		// The returned value copies out of the body when its record is a
		// value record (chapter 8's "return copies the whole value"); a
		// fresh construction is already the caller's own object.
		reg, _, ni := e.emitOwnedRecord(value)
		if ni != nil {
			return "", ni
		}
		return "ptr " + reg, nil
	case abiSum:
		if !hasValue {
			return "", bndFn()
		}
		c, ok := value.(*ast.Call)
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

// recvKeyOf resolves the record a receiver expression denotes without
// emitting: a binding's own record, a record-typed field chain's, or a
// construction's. It is the method table's lookup key half — anything it
// cannot name is a boundary, never a wrong dispatch.
func (e *emitter) recvKeyOf(x ast.Expr) (string, bool) {
	switch v := x.(type) {
	case *ast.Ident:
		if _, is := e.ntEnv[v.Name]; is {
			// A newtype's own methods would key on the wrapper, which the
			// erased value cannot name — the head a call site reads is the
			// underlying's, so no dispatch is resolved here.
			return "", false
		}
		if g, ok := e.gcEnv[v.Name]; ok {
			return g.rec, true
		}
		// A module-level record binding (T8-2B) names its record from the
		// slot, without reading the global: the key is a compile-time fact.
		if ts, ok := e.topName(v.Name); ok && ts.gc && ts.rec != "" {
			return ts.rec, true
		}
		return "", false
	case *ast.Member:
		slot, ok := e.chainField(v)
		if !ok || (slot.kind != fkRef && slot.kind != fkVal) {
			return "", false
		}
		return slot.typ, true
	case *ast.Construct:
		key := e.curKey
		if v.Qual != "" {
			key = e.resolveQual(v.Qual)
			if key == "" {
				return "", false
			}
		}
		if _, ok := e.records[key+"."+v.Name]; !ok {
			return "", false
		}
		return key + "." + v.Name, true
	}
	return "", false
}

// emitMethodCall emits one method call (design D4): the receiver pointer
// leads the arguments and the call goes straight to the method's own
// symbol. The table is static — the receiver's type is known at the site —
// so nothing is loaded through a slot here; mocking's indirection belongs
// to program fns, and a receiver whose method the table does not name is
// the check stage's to have rejected.
func (e *emitter) emitMethodCall(fd *fnDef, recv ast.Expr, args []ast.Expr) (callResult, *NotImplemented) {
	abi, ok := e.classify(fd)
	if !ok {
		return callResult{}, bndFn()
	}
	if len(args) != len(fd.decl.Params) {
		return callResult{}, e.bnd()
	}
	// The receiver is read in the caller's own module: a method's head
	// type key is already qualified, so no module switch is owed for the
	// lookup, but the receiver expression itself resolves here.
	reg, _, ni := e.emitRecordValue(recv)
	if ni != nil {
		return callResult{}, ni
	}
	return e.emitCallCore(abi, "@"+fd.sym(), []string{"ptr " + reg}, args)
}

// emitFnCall emits one program-fn call through its slot (design D3): the
// slot load, the argument list per the callee's families, and the result
// per the return family — a String return extracts its two words into
// the binding's operand pair, a record return is rooted by the caller,
// a sum return lands in the two-slot environment. A foreign entry
// (chapter 19) takes the foreign ABI instead: no slot, no qualifier —
// the declared name is the C symbol itself.
func (e *emitter) emitFnCall(fd *fnDef, args []ast.Expr) (callResult, *NotImplemented) {
	if fd.foreign {
		return e.emitForeignCall(fd, args)
	}
	abi, ok := e.classify(fd)
	if !ok {
		return callResult{}, bndFn()
	}
	if len(args) != len(fd.decl.Params) {
		return callResult{}, e.bnd()
	}
	slot := e.slotFor(fd.sym(), fnSlotTarget(fd))
	fp := e.value()
	e.inst(fmt.Sprintf("%%%s = load ptr, ptr %s", fp, slot))
	return e.emitCallCore(abi, "%"+fp, nil, args)
}

// emitCallCore emits one call's argument list and result: the operands per
// the callee's families, the call itself, and the value face of its return
// family. callee is the operand the call goes through — a program fn's
// loaded slot register, or a method's own symbol (the table is static, so
// a method call owes no indirection). recvOp, where present, is the
// already-rendered receiver operand that leads the argument list.
func (e *emitter) emitCallCore(abi fnAbi, callee string, preOps []string, args []ast.Expr) (callResult, *NotImplemented) {
	ops := append([]string(nil), preOps...)
	for i, a := range args {
		if abi.params[i].kind == abiTuple {
			// A tuple argument expands field-per-word at the boundary
			// (design D4): the aggregate's words are read back out and
			// passed flat.
			ptr, elems, ni := e.emitTupleOperand(a)
			if ni != nil {
				return callResult{}, ni
			}
			if aggTyp(elems) != aggTyp(abi.params[i].elems) {
				return callResult{}, e.bnd()
			}
			for _, el := range elems {
				for j, wt := range el.words {
					at := el.off + 8*j
					switch wt {
					case "i64":
						ops = append(ops, "i64 "+e.gepLoadI64(ptr, at))
					case "double":
						ops = append(ops, "double "+e.gepLoadDouble(ptr, at))
					case "ptr":
						ops = append(ops, "ptr "+e.gepLoadPtr(ptr, at))
					}
				}
			}
			continue
		}
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
			case abiGc:
				if res.kind != ckGc {
					return callResult{}, e.bnd()
				}
				reg := res.gcReg
				if r, ok := e.records[res.recKey]; ok && r.Cat == "value" {
					reg = e.emitRecCopy(reg, res.recKey)
				}
				ops = append(ops, "ptr "+reg)
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
			// An argument of a value-category record copies at the call
			// site (chapter 8's "Passing copies"); a gc or resource
			// record passes its own reference.
			reg, _, ni := e.emitOwnedRecord(a)
			if ni != nil {
				return callResult{}, ni
			}
			ops = append(ops, "ptr "+reg)
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
		case abiFn:
			// The argument crosses as its carrier. A closure literal is
			// emitted here (its captures frozen at this call site), a
			// bound fn value passes its own, and a program fn name takes
			// the constant pair design D5 gives it.
			fv, ni := e.emitFnArg(a, abi.params[i].sig)
			if ni != nil {
				return callResult{}, ni
			}
			ops = append(ops, "ptr "+fv.carrier)
		}
	}
	join := strings.Join(ops, ", ")
	switch abi.ret {
	case abiVoid:
		e.inst(fmt.Sprintf("call void %s(%s)", callee, join))
		return callResult{kind: ckVoid}, nil
	case abiI64:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call i64 %s(%s)", v, callee, join))
		return callResult{kind: ckI64, i64: "%" + v, typeName: abi.retName}, nil
	case abiDouble:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call double %s(%s)", v, callee, join))
		return callResult{kind: ckI64, i64: "%" + v, isFloat: true, typeName: abi.retName}, nil
	case abiStr:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call { ptr, i64 } %s(%s)", v, callee, join))
		p := e.value()
		e.inst(fmt.Sprintf("%%%s = extractvalue { ptr, i64 } %%%s, 0", p, v))
		l := e.value()
		e.inst(fmt.Sprintf("%%%s = extractvalue { ptr, i64 } %%%s, 1", l, v))
		return callResult{kind: ckStr, strBind: strBinding{dataOp: "%" + p, lenOp: "%" + l}}, nil
	case abiTuple:
		// The returned aggregate comes back in registers; it lands in the
		// stack handle a tuple value always is, one extractvalue per word.
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call %s %s(%s)", v, abi.retTyp, callee, join))
		agg := e.slot(abi.retTyp)
		w := 0
		for _, el := range abi.elems {
			for i, wt := range el.words {
				ev := e.value()
				e.inst(fmt.Sprintf("%%%s = extractvalue %s %%%s, %d", ev, abi.retTyp, v, w))
				e.gepStore(agg, el.off+8*i, wt+" %"+ev)
				w++
			}
		}
		return callResult{kind: ckTuple, tup: tupBinding{ptr: agg, elems: abi.elems}}, nil
	case abiGc:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call ptr %s(%s)", v, callee, join))
		reg := "%" + v
		e.use("__we_root_push")
		e.pushes++
		e.inst(fmt.Sprintf("call void @__we_root_push(ptr %s)", reg))
		return callResult{kind: ckGc, gcReg: reg, recKey: abi.retKey}, nil
	case abiSum:
		v := e.value()
		e.inst(fmt.Sprintf("%%%s = call { i64, i64 } %s(%s)", v, callee, join))
		t := e.value()
		e.inst(fmt.Sprintf("%%%s = extractvalue { i64, i64 } %%%s, 0", t, v))
		p := e.value()
		e.inst(fmt.Sprintf("%%%s = extractvalue { i64, i64 } %%%s, 1", p, v))
		ts := e.slot("i64")
		e.inst(fmt.Sprintf("store i64 %%%s, ptr %s", t, ts))
		pp := e.slot("i64")
		e.inst(fmt.Sprintf("store i64 %%%s, ptr %s", p, pp))
		return callResult{kind: ckSum, sum: sumSlot{tag: ts, pay: pp, variants: abi.variants}}, nil
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

// gepLoadDouble loads a double word at base+off.
func (e *emitter) gepLoadDouble(base string, off int) string {
	v := e.value()
	e.inst(fmt.Sprintf("%%%s = getelementptr i8, ptr %s, i64 %d", v, base, off))
	r := e.value()
	e.inst(fmt.Sprintf("%%%s = load double, ptr %%%s", r, v))
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
			case fkRef, fkVal:
				parts = append(parts, "ptr")
			case fkScalar:
				parts = append(parts, "i64")
			case fkF64:
				parts = append(parts, "double")
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
	if e.strStruct {
		// The String pair as a named type (design D3): the C declaration
		// returns it in two registers, the same shape clang lowers the
		// struct to, so the emitter spells it and extracts the two words.
		structs = append(structs, "%struct.we_str = type { ptr, i64 }")
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
	if len(e.divzs) > 0 {
		groups = append(groups, e.divzs)
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
	if len(e.topGlobals) > 0 {
		groups = append(groups, e.topGlobals)
	}
	if len(e.foreignDecls) > 0 {
		groups = append(groups, e.foreignDecls)
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
	sb.WriteString(e.bodyText())
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

// decodeRuneLiteral resolves a rune literal (Text holds the source slice,
// quotes included) to its code point. The escape machine is the string
// literal's — chapter 1's set is closed and shared — applied to the one
// character the literal spells: the decoded bytes must be exactly one
// valid UTF-8 sequence. The lexer has already validated the syntax, so
// every other path resolves.
func decodeRuneLiteral(text string) (int64, bool) {
	if len(text) < 3 || text[0] != '\'' || text[len(text)-1] != '\'' {
		return 0, false
	}
	data, ok := decodeStringLiteral(`"` + text[1:len(text)-1] + `"`)
	if !ok {
		return 0, false
	}
	r, size := utf8.DecodeRuneInString(data)
	if r == utf8.RuneError && size <= 1 {
		return 0, false
	}
	if size != len(data) {
		return 0, false
	}
	return int64(r), true
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
