// Package typecheck implements the type stage over the parsed module:
// chapter 7's type references resolve into types, chapter 9's sum types
// and their constructor mechanism check, chapter 15's main convention
// holds the root module, and the base-type literals and operators of the
// types chapter carry the no-coercion agreement rule (E0501) with the
// constant-folding overflow check (E0502). Chapter 10's declarations
// validate here — interfaces, impls, and derives clauses with their
// declaration-side rules, the member sets they register, and member
// resolution over them: nominal fields and methods, the Dyn box's
// carried face, and the base types' builtin members (an unconstrained
// parameter is opaque, and the iterator combinators are the collections
// pass's) — with the explicit-application substitution (a bare generic
// form's determination and the where rules stop at honest boundaries).
// Name resolution walks the four layers — block locals, the generic
// clause, the module namespace, the prelude — and the not-yet-
// implemented names stop at honest boundaries, never at E1304. The stage
// stops at the first diagnostic or boundary (chapter 21: an E-severity
// diagnostic stops the pipeline).
package typecheck

import (
	"fmt"
	"math/big"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/diag"
)

// Mode selects the module context: one file checked alone, or the root
// module of a project whose directory carries we.toml (the main
// convention binds the root module alone).
type Mode int

const (
	SingleFile Mode = iota
	Project
)

// NotImplemented reports a ratified-but-unimplemented form the type stage
// reached; What names the form group and the CLI prints the boundary line
// with exit 70.
type NotImplemented struct{ What string }

// The boundary Whats of this stage's closed table (design D14). Each later
// milestone deletes its rows; the two spec-gap rows carry their follow-up
// registration.
const (
	bndStdModules = "standard-library modules (chapter 15)"
	// assertEqual's domain face (M10a design D8): the scalar comparanda
	// are checked here; the Eq-generic widening over composites is the
	// standard library's own change, not this checker's.
	bndAssertEqDomain = "assertEqual beyond the scalar, Bool, and String domains (the Eq-generic face is the standard library's own widening)"
	bndDomainGap      = "arithmetic and comparisons beyond the ratified numeric and Bool domains (spec gap; roadmap follow-up)"
	bndArityGap       = "calls with an argument count the callee does not declare (spec gap; roadmap follow-up)"
	bndCalleeGap      = "calls on values that are not functions (spec gap; roadmap follow-up)"
	// The two residual M5 boundary rows: a module-level destructure stops
	// at the composite boundary (module initialization order is the module
	// system's, M6), and walkItems keeps the control-flow row as its
	// default guard so a future statement form stops honestly instead of
	// silently skipping. Never a silent skip, never a panic.
	bndCtlForms  = "chapter 3 (control flow) forms"
	bndCompForms = "chapter 8 (composites) forms"
)

// tHelps carries each code's remediation from the registry
// (docs/spec/diagnostics.toml). One home until the registry embed lands
// (roadmap follow-up 2).
var tHelps = map[string]string{
	"E0501": "Make the types agree at the source: use a literal suffix (42i32) or an explicit conversion method (toInt64(), toBytes()); the method inventory is the standard library's.",
	"E0502": "Widen the type (suffix or annotation), restructure the computation, or opt into wrapping explicitly (wrappingAdd() and siblings).",
	"E0503": "Make the condition a Bool expression: compare explicitly (n != 0), call a predicate, or apply a conversion method.",
	"E0605": "Bind the value to use it, chain it onward, declare a return type, or discard it explicitly with `let _ = expr`.",
	"E0303": "Spell a declared variant of the scrutinee's type, or match a value of the type that declares the variant; the declaration, not the pattern, fixes the variant set.",
	"E0304": "Write one sub-pattern per declared payload type; to group payloads, declare the variant with a record payload.",
	"E0305": "Cover the missing variant, or add an unguarded wildcard arm `_ => ...`; literal arms never replace a wildcard on a base type.",
	"E0306": "Keep or add an unguarded arm set that covers every value by itself, typically a wildcard `_ => ...` fallback after the guarded refinements.",
	"E0307": "Delete the unreachable arm, or reorder so the general arm (typically the wildcard) comes last.",
	"E0702": "Drop byval so the sum is gc, or store value-category/base data in the payload; if reference semantics are needed, the sum is not a value sum.",
	"E0703": "Drop the Never annotation, or move it to a function's return type where a non-producing path is genuinely declared.",
	"E0704": "Construct with the payload (`Circle(1.0)`); first-class constructor values arrive, if at all, with the function-types chapter.",
	"E0827": "Write the explicit form: `empty<Int64>()`, `Pair<Int64, String> { ... }`.",
	"E0828": "Match the declaration's arity, or fix the declaration's clause.",
	"E0601": "Make the record gc (drop byval), or store value-category/base data in the field; if reference semantics are needed, the record is not a value record.",
	"E0603": "Name the record type at the head and give an update base of that same type; other forms are not constructible or updatable records.",
	"E0604": "Match the record's declared field set exactly in a construction, name only declared fields in an update, or access a field the record declares.",
	"E0606": "Change the resource's fields through the mechanisms of the resource and interfaces chapters; resource records are not updatable with `with &`.",
	"E0816": "Fix the name, or declare the member; unknown-field access on records lands here too.",
	"E1001": "Annotate the parameters - |x: Int64| x + 1 - or give the binding a function-type annotation so the parameters take their types from it.",
	"E1002": "Materialize the resource's contents first - one explicit read into a value - and capture the materialized value.",
	"E1003": "Carry the intermediate in a local binding of the closure body, or hold the state in a gc-category cell and capture that.",
	"E1204": "Declare a named sum type carrying the failure variants and use it as the error parameter.",
	"E1302": "Create the file at the expected path, fix the path spelling, or add the dependency to the cache.",
	"E1304": "Fix the spelling, declare the name, import the module, or qualify through an existing import name.",
	"E1305": "Declare exactly one pub fn main() -> Result<(), E> in src/main.we with E a named sum type.",
	"E1401": "Add the missing tag to the enclosing declaration's effect segment, or call a pure function instead.",
	"E1402": "Widen the expected type's effect segment to cover the value's set, or supply a value that performs less.",
	"E1404": "Copy the interface method's effect segment into the impl method's signature exactly, in both directions.",
	"E1405": "Move the work into main, or initialize from a pure computation over constants.",
	"E1614": "Wrap the composite in Mutex or RwLock, or hold the base-type part in the Atomic.",
	"E1615": "Use Atomic for base types and Mutex or RwLock for other composites.",
	"E1616": "Construct with a positive count; guard non-constant counts before the call - a non-positive count at runtime is a checked panic of chapter 14's family.",
	"E1617": "Annotate the binding, or place the construction where a parameter or declared return fixes the type.",
	"E1606": "Delete the impl: the type is Shareable exactly when its own shape says so, and the bound T: Shareable checks that shape.",
	"E1608": "Take a CancelSignal parameter when a caller wants to cancel the work, or move the call inside the task block that should answer cancellation.",
	"E1803": "Copy the target's signature — parameters, return, and segment — exactly; adapt the body, not the contract.",
	"E1804": "Mock a module-level monomorphic fn — own-module bare name or imported pub qualified name; wrap richer targets in such a fn and mock the wrapper.",
	"E1805": "Keep one mock per target per block; a second test block may mock the same target for itself.",
	"E1806": "Advance the clock inside the test that observes it; plain code has no clock to advance.",
	"E1618": "Open a scope block around the task creation, or move the work into an ordinary function call.",
	"E1602": "Carry the state in a synchronized type (store the list in a Mutex, send it through a Channel), or capture only Shareable data and let the task receive the rest.",
	"E1603": "Carry the mutable state in a shared-state type and reach it through its methods, or copy the value into an immutable binding before the task block.",
	"E1604": "Restrict the type to self methods, keep the task out, or materialize the contents and capture those instead.",
	"E1605": "Declare the parameter with a Shareable bound, or keep the capture to concrete Shareable types.",
	"E1705": "Marshall on the We side - explicit field-by-field calls, buffers filled through foreign functions, opaque handles plus length queries; one value per call.",
	"E1706": "Return an opaque handle plus a length query plus a copy into a caller-provided buffer - the explicit marshalling idiom.",
	"E1707": "Obtain the value from a foreign function's declared return; the native side mints the handle.",
	"E1607": "Await or cancel the handle on every path from its creation; on early exits the scope's own exit discharge covers it - plain and timeout forms canceling the rest, collectAll joining them.",
}

// stop and bstop unwind the check at the first diagnostic or boundary,
// mirroring the parser's control flow.
type stop struct{ d diag.Diagnostic }
type bstop struct{ what string }

// --- types -------------------------------------------------------------------

// Type is one semantic type. Equality is structural (design D1): base
// names, the unit and bottom singletons, named sums with their type
// arguments, tuples element-wise, and fn types parameter- and return-wise.
type Type interface{ String() string }

type baseType string

func (b baseType) String() string { return string(b) }

type unitType struct{}

func (unitType) String() string { return "()" }

type neverType struct{}

func (neverType) String() string { return "Never" }

// paramRef names one type parameter position of a generic declaration or
// builtin sum (chapter 10): the position by index, the clause name for
// rendering. It materializes through substitution — an explicit
// application's arguments or an expected type's — so a declaration-side
// view keeps its clause names.
type paramRef struct {
	idx  int
	name string
}

func (p paramRef) String() string { return p.name }

type tupleType struct{ elems []Type }

func (t tupleType) String() string {
	return "(" + typeJoin(t.elems) + ")"
}

type fnType struct {
	params []Type
	tags   []string
	ret    Type
}

func (f fnType) String() string {
	s := "fn(" + typeJoin(f.params) + ")"
	if len(f.tags) > 0 {
		ds := make([]string, len(f.tags))
		for i, t := range f.tags {
			ds[i] = displayTag(t)
		}
		s += " " + strings.Join(ds, " ")
	}
	return s + " -> " + f.ret.String()
}

// namedType is one declared sum applied to its type arguments (the builtin
// sums and chapter 10's generic sums carry them; a monomorphic declaration
// carries none).
type namedType struct {
	decl *sumInfo
	args []Type
}

func (n namedType) String() string {
	if len(n.args) == 0 {
		return n.decl.name
	}
	return n.decl.name + "<" + typeJoin(n.args) + ">"
}

// recordType is one declared record applied to its type arguments; the
// declaration owns the field list and the ownership category, so the type
// is a reference plus the application (design D6: one authority per fact).
type recordType struct {
	decl *recordInfo
	args []Type
}

func (r recordType) String() string {
	if len(r.args) == 0 {
		return r.decl.name
	}
	return r.decl.name + "<" + typeJoin(r.args) + ">"
}

// newtypeType is one declared newtype applied to its type arguments: a
// zero-cost wrapper whose category and member follow its underlying type.
type newtypeType struct {
	decl *newtypeInfo
	args []Type
}

func (n newtypeType) String() string {
	if len(n.args) == 0 {
		return n.decl.name
	}
	return n.decl.name + "<" + typeJoin(n.args) + ">"
}

// ifaceType is one declared interface applied to its type arguments; the
// declaration owns the associated types and the method signatures. An
// interface occupies type slots only as Dyn<Interface> — a value slot
// rejects the bare name (E0821).
type ifaceType struct {
	decl *ifaceInfo
	args []Type
}

func (i ifaceType) String() string {
	if len(i.args) == 0 {
		return i.decl.name
	}
	return i.decl.name + "<" + typeJoin(i.args) + ">"
}

// assocRef names one associated type position of an interface; it
// materializes through the impl's bindings (inside the interface's own
// default bodies it stays a position — the body types against the
// interface, not an impl).
type assocRef struct {
	iface *ifaceInfo
	name  string
}

func (a assocRef) String() string { return a.name }

// dynType is the erased box (chapter 10): the one interface it carries,
// applied to its arguments. Its member set is the carried interface's
// method set alone — a value of an implementing type boxes in, and the
// box hands back only the face. An interface declaring associated types
// never boxes (E0819): the erased value could not honor the binding.
type dynType struct {
	inf  *ifaceInfo
	args []Type
}

func (d dynType) String() string {
	box := d.inf.name
	if len(d.args) > 0 {
		box += "<" + typeJoin(d.args) + ">"
	}
	return "Dyn<" + box + ">"
}

func typeJoin(ts []Type) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = t.String()
	}
	return strings.Join(parts, ", ")
}

// sameType is structural equality (design D1).
func sameType(a, b Type) bool {
	switch x := a.(type) {
	case baseType:
		y, ok := b.(baseType)
		return ok && x == y
	case unitType:
		_, ok := b.(unitType)
		return ok
	case neverType:
		_, ok := b.(neverType)
		return ok
	case namedType:
		y, ok := b.(namedType)
		if !ok || x.decl != y.decl || len(x.args) != len(y.args) {
			return false
		}
		for i := range x.args {
			if !sameType(x.args[i], y.args[i]) {
				return false
			}
		}
		return true
	case tupleType:
		y, ok := b.(tupleType)
		if !ok || len(x.elems) != len(y.elems) {
			return false
		}
		for i := range x.elems {
			if !sameType(x.elems[i], y.elems[i]) {
				return false
			}
		}
		return true
	case fnType:
		y, ok := b.(fnType)
		if !ok || len(x.params) != len(y.params) || len(x.tags) != len(y.tags) {
			return false
		}
		for i := range x.tags {
			if x.tags[i] != y.tags[i] {
				return false
			}
		}
		if !sameType(x.ret, y.ret) {
			return false
		}
		for i := range x.params {
			if !sameType(x.params[i], y.params[i]) {
				return false
			}
		}
		return true
	case paramRef:
		y, ok := b.(paramRef)
		return ok && x.idx == y.idx && x.name == y.name
	case recordType:
		y, ok := b.(recordType)
		return ok && x.decl == y.decl && argsEqual(x.args, y.args)
	case newtypeType:
		y, ok := b.(newtypeType)
		return ok && x.decl == y.decl && argsEqual(x.args, y.args)
	case ifaceType:
		y, ok := b.(ifaceType)
		return ok && x.decl == y.decl && argsEqual(x.args, y.args)
	case dynType:
		y, ok := b.(dynType)
		return ok && x.inf == y.inf && argsEqual(x.args, y.args)
	case assocRef:
		y, ok := b.(assocRef)
		return ok && x.iface == y.iface && x.name == y.name
	}
	return false
}

// argsEqual is element-wise type equality for type argument slices.
func argsEqual(a, b []Type) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameType(a[i], b[i]) {
			return false
		}
	}
	return true
}

// agree is the agreement judgment: structural equality, with the bottom
// type agreeing everywhere (a Never expression inhabits every type).
func agree(a, b Type) bool {
	if _, never := a.(neverType); never {
		return true
	}
	if _, never := b.(neverType); never {
		return true
	}
	return sameType(a, b)
}

// --- declarations ------------------------------------------------------------

// sumInfo is one declared sum: its name, its clause names (chapter 10), its
// byval flag (the value-category declaration), variants with resolved
// payload types (nil payloads = a unit variant), its derive targets, and
// its registered methods (impls and derives; design D3).
type sumInfo struct {
	name     string
	params   []string
	byval    bool
	variants []variantInfo
	derives  []string
	methods  []memberMethod
}

type variantInfo struct {
	name       string
	payloads   []Type
	payloadPos [][2]int // each payload's type token (E0823 anchors)
}

// recordInfo is one declared record (chapter 8): its ownership category,
// its clause names and derive targets, its fields with resolved types, and
// its registered methods. cat is the declared prefix: "value" (byval
// record), "resource" (byres record), "gc" (default). opaque marks the
// chapter 19 foreign entry face: a fieldless record standing for a
// native-side type — a handle typecheck holds and E1707 keeps We code from
// minting.
type recordInfo struct {
	name    string
	cat     string
	params  []string
	derives []string
	fields  []fieldInfo
	methods []memberMethod
	opaque  bool
}

type fieldInfo struct {
	name      string
	typ       Type
	line, col int // the field's type token (E0823 anchors)
}

// newtypeInfo is one declared newtype: the name, the clause names and
// derive targets, the resolved underlying type (the wrapper's layout is
// erased; the category and the one member "value" derive from it), and its
// registered methods.
type newtypeInfo struct {
	name       string
	params     []string
	derives    []string
	underlying Type
	underLine  int
	underCol   int
	methods    []memberMethod
}

// memberMethod is one method of a nominal type's member set (design D3's
// sources after the fields): the callable view (receiver excluded,
// substituted at the application), the origin — "inherent", the interface
// name, or the derive target — the receiver mutability, and whether the
// view still holds method-clause parameters (its calls wait for the
// determination pass).
type memberMethod struct {
	name      string
	fn        fnType
	from      string
	mutRecv   bool
	generic   bool
	line, col int // the anchored method-name or clause token (E0814)
}

// ifaceInfo is one declared interface (chapter 10): its clause names, its
// associated types, and its method signatures with optional default
// bodies. The builtin derive targets (Eq, Hash, Show) carry the marker —
// their methods come from derives clauses, never hand-written impls
// (E0822).
type ifaceInfo struct {
	name         string
	params       []string
	assocs       []string
	methods      []ifaceMethod
	deriveTarget bool
}

// ifaceMethod is one interface method: the receiver mutability, the
// parameter and return types resolved against the interface's clause and
// associated types, the resolved effect segment (chapter 16 — canonical
// keys, nil = pure; an impl's method must equal it exactly, E1404), an
// optional default body, its own method clause, and its name token (the
// E0808/E0814 anchors).
type ifaceMethod struct {
	name       string
	recvMut    bool
	params     []Type
	ret        Type // nil = valueless
	tags       []string
	typeParams []string
	body       *ast.Block
	// paramNames names the parameters for a default body's walk (the
	// user path reads the declaration node's own names; the builtin
	// faces have no node, so the registration states them).
	paramNames []string
	line, col  int
}

// implInfo records one validated impl: the interface (nil = inherent), its
// arguments, the head type and its names, the associated bindings, and
// whether parameters remain unresolved (E0809's overlap test). It carries
// everything the bodies pass re-enters with.
type implInfo struct {
	decl      *ast.ImplDecl
	iface     *ifaceInfo
	ifaceArgs []Type
	head      Type
	headName  string
	headStr   string
	generic   bool
	assocs    map[string]Type
}

// catOf reports a type's ownership category (design D6): base, unit, and
// bottom types are value; a record carries its declared category; a
// newtype and a sum derive from their declarations (a byval sum is
// value); a tuple derives element-wise (any resource element makes the
// tuple one, else any gc element); a fn value is gc.
func catOf(t Type) string {
	switch x := t.(type) {
	case baseType, unitType, neverType, paramRef:
		return "value"
	case recordType:
		return x.decl.cat
	case newtypeType:
		return catOf(x.decl.underlying)
	case namedType:
		if x.decl.byval {
			return "value"
		}
		return "gc"
	case tupleType:
		cat := "value"
		for _, e := range x.elems {
			switch catOf(e) {
			case "resource":
				return "resource"
			case "gc":
				cat = "gc"
			}
		}
		return cat
	case fnType:
		return "gc"
	case ifaceType:
		return "gc" // a boxed value is a reference; the bare name never values
	case dynType:
		return "gc" // the box is a reference by construction
	}
	return "value"
}

// catNoun names a type the category-honesty diagnostics (E0601/E0702)
// reject: the message says why the type is not copyable by value.
func catNoun(t Type) string {
	switch x := t.(type) {
	case recordType:
		if x.decl.cat == "resource" {
			return "byres record"
		}
		return "gc record" // a value record never fails the check
	case newtypeType:
		return catNoun(x.decl.underlying)
	case namedType:
		return "gc sum"
	case tupleType:
		return "tuple holding a non-value element"
	case fnType:
		return "function value"
	}
	return "non-value type"
}

// --- chapter 10 helpers: clauses, substitution, capability -------------------

// typeParamNames reads a generic clause's names.
func typeParamNames(ps []*ast.TypeParam) []string {
	if len(ps) == 0 {
		return nil
	}
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return names
}

// paramScope builds a declaration's clause scope: each name at its
// parameter position (nil without a clause).
func paramScope(params []string) map[string]Type {
	if len(params) == 0 {
		return nil
	}
	m := make(map[string]Type, len(params))
	for i, n := range params {
		m[n] = paramRef{idx: i, name: n}
	}
	return m
}

// clauseArgs is a declaration's own parameter positions as its self-view —
// the derives-generated methods and the default bodies type against the
// declaration, not an application.
func clauseArgs(params []string) []Type {
	if len(params) == 0 {
		return nil
	}
	args := make([]Type, len(params))
	for i, n := range params {
		args[i] = paramRef{idx: i, name: n}
	}
	return args
}

// paramScopeOffset builds a method clause's scope: the names sit after the
// enclosing clause's positions, so an application of the enclosing
// declaration substitutes the outer positions only (E0826 keeps the two
// namespaces disjoint).
func paramScopeOffset(ps []*ast.TypeParam, offset int) map[string]Type {
	if len(ps) == 0 {
		return nil
	}
	m := make(map[string]Type, len(ps))
	for i, p := range ps {
		m[p.Name] = paramRef{idx: offset + i, name: p.Name}
	}
	return m
}

// paramIdx finds a name in a clause's names (-1 absent).
func paramIdx(params []string, name string) int {
	for i, n := range params {
		if n == name {
			return i
		}
	}
	return -1
}

// ifaceScope is an interface's signature scope: the clause positions and
// the associated-type holes (the holes stay positions — an impl's bindings
// materialize them).
func ifaceScope(inf *ifaceInfo) map[string]Type {
	m := paramScope(inf.params)
	if len(inf.assocs) == 0 {
		return m
	}
	if m == nil {
		m = map[string]Type{}
	}
	for _, a := range inf.assocs {
		m[a] = assocRef{iface: inf, name: a}
	}
	return m
}

// subst replaces a type's parameter positions with an application's
// arguments and its associated positions with an impl's bindings (design
// D6's substitution — structural, bottom-up; unknown positions stay).
func subst(t Type, args []Type, assocs map[string]Type) Type {
	switch x := t.(type) {
	case paramRef:
		if x.idx < len(args) {
			return args[x.idx]
		}
		return x
	case assocRef:
		if r, ok := assocs[x.name]; ok {
			return r
		}
		return x
	case tupleType:
		elems := make([]Type, len(x.elems))
		for i, e := range x.elems {
			elems[i] = subst(e, args, assocs)
		}
		return tupleType{elems: elems}
	case fnType:
		return fnType{params: substArgs(x.params, args, assocs), tags: x.tags, ret: subst(x.ret, args, assocs)}
	case namedType:
		return namedType{decl: x.decl, args: substArgs(x.args, args, assocs)}
	case recordType:
		return recordType{decl: x.decl, args: substArgs(x.args, args, assocs)}
	case newtypeType:
		return newtypeType{decl: x.decl, args: substArgs(x.args, args, assocs)}
	case ifaceType:
		return ifaceType{decl: x.decl, args: substArgs(x.args, args, assocs)}
	case dynType:
		return dynType{inf: x.inf, args: substArgs(x.args, args, assocs)}
	}
	return t
}

func substArgs(as, args []Type, assocs map[string]Type) []Type {
	if len(as) == 0 {
		return nil
	}
	out := make([]Type, len(as))
	for i, a := range as {
		out[i] = subst(a, args, assocs)
	}
	return out
}

// rebaseClause shifts a type's parameter references from one method-clause
// metric space to another: every paramRef at or above from — a method-level
// clause position, per the outer-then-method offset the clause scopes build
// — moves by to-from. The method clause agreement compares an interface
// method's substituted signature with the impl method's own, and the two
// sides resolve their identical method clauses at different offsets (the
// interface's after its own clause, the impl's after the impl's); the
// rebase aligns them (design D10(b)).
func rebaseClause(t Type, from, to int) Type {
	if from == to {
		return t
	}
	shift := to - from
	switch x := t.(type) {
	case paramRef:
		if x.idx >= from {
			return paramRef{idx: x.idx + shift, name: x.name}
		}
	case tupleType:
		elems := make([]Type, len(x.elems))
		for i, e := range x.elems {
			elems[i] = rebaseClause(e, from, to)
		}
		return tupleType{elems: elems}
	case fnType:
		return fnType{params: rebaseClauseList(x.params, from, to), tags: x.tags, ret: rebaseClause(x.ret, from, to)}
	case namedType:
		return namedType{decl: x.decl, args: rebaseClauseList(x.args, from, to)}
	case recordType:
		return recordType{decl: x.decl, args: rebaseClauseList(x.args, from, to)}
	case newtypeType:
		return newtypeType{decl: x.decl, args: rebaseClauseList(x.args, from, to)}
	case ifaceType:
		return ifaceType{decl: x.decl, args: rebaseClauseList(x.args, from, to)}
	case dynType:
		return dynType{inf: x.inf, args: rebaseClauseList(x.args, from, to)}
	}
	return t
}

func rebaseClauseList(as []Type, from, to int) []Type {
	if len(as) == 0 {
		return nil
	}
	out := make([]Type, len(as))
	for i, a := range as {
		out[i] = rebaseClause(a, from, to)
	}
	return out
}

// substFn substitutes a callable view.
func substFn(f fnType, args []Type, assocs map[string]Type) fnType {
	return fnType{params: substArgs(f.params, args, assocs), tags: f.tags, ret: subst(f.ret, args, assocs)}
}

// containsParam reports whether a type still holds a parameter or
// associated position: its facts defer to the application (a generic
// declaration checks at each instantiation, design D8).
func containsParam(t Type) bool {
	switch x := t.(type) {
	case paramRef, assocRef:
		return true
	case tupleType:
		return argsContainParam(x.elems)
	case fnType:
		return fnContainsParam(x)
	case namedType:
		return argsContainParam(x.args)
	case recordType:
		return argsContainParam(x.args)
	case newtypeType:
		return argsContainParam(x.args)
	case ifaceType:
		return argsContainParam(x.args)
	case dynType:
		return argsContainParam(x.args)
	}
	return false
}

func argsContainParam(args []Type) bool {
	for _, a := range args {
		if containsParam(a) {
			return true
		}
	}
	return false
}

func fnContainsParam(f fnType) bool {
	return argsContainParam(f.params) || containsParam(f.ret)
}

// targetIn reports whether a derives clause carries one target.
func targetIn(targets []string, target string) bool {
	for _, t := range targets {
		if t == target {
			return true
		}
	}
	return false
}

// carries reports whether a type carries one derive capability (design
// D8): base types carry every target; a nominal type carries through its
// own clause — its parts substituted and checked the same way; a parameter
// or associated position does not carry (its bounds are the where
// clause's). visiting holds the declarations on the current path — a
// recursive type carries only if its cycle's entries do.
func carries(t Type, target string, visiting map[*recordInfo]bool, sumVisiting map[*sumInfo]bool, ntVisiting map[*newtypeInfo]bool) bool {
	switch x := t.(type) {
	case baseType:
		return true
	case recordType:
		if !targetIn(x.decl.derives, target) || visiting[x.decl] {
			return false
		}
		visiting[x.decl] = true
		defer delete(visiting, x.decl)
		for _, f := range x.decl.fields {
			if !carries(subst(f.typ, x.args, nil), target, visiting, sumVisiting, ntVisiting) {
				return false
			}
		}
		return true
	case newtypeType:
		if !targetIn(x.decl.derives, target) || ntVisiting[x.decl] {
			return false
		}
		ntVisiting[x.decl] = true
		defer delete(ntVisiting, x.decl)
		return carries(subst(x.decl.underlying, x.args, nil), target, visiting, sumVisiting, ntVisiting)
	case namedType:
		if !targetIn(x.decl.derives, target) || sumVisiting[x.decl] {
			return false
		}
		sumVisiting[x.decl] = true
		defer delete(sumVisiting, x.decl)
		for _, v := range x.decl.variants {
			for _, p := range v.payloads {
				if !carries(subst(p, x.args, nil), target, visiting, sumVisiting, ntVisiting) {
					return false
				}
			}
		}
		return true
	}
	return false
}

// carriesType is the top of the capability check: fresh visiting sets.
func carriesType(t Type, target string) bool {
	return carries(t, target, map[*recordInfo]bool{}, map[*sumInfo]bool{}, map[*newtypeInfo]bool{})
}

// deriveMethods builds one derives clause's generated methods (design D8):
// Eq → equals(other: Self) -> Bool, Hash → hash() -> Int64, Show →
// toDebugString() -> String. self is the declaration's own view — on a
// generic declaration the parameter positions, so the methods instantiate
// with the type.
func deriveMethods(self Type, targets []string, clauseLine, clauseCol int) []memberMethod {
	var ms []memberMethod
	for _, t := range targets {
		var m memberMethod
		switch t {
		case "Eq":
			m = memberMethod{name: "equals", from: "derive:Eq",
				fn: fnType{params: []Type{self}, ret: baseType("Bool")}}
		case "Hash":
			m = memberMethod{name: "hash", from: "derive:Hash",
				fn: fnType{ret: baseType("Int64")}}
		case "Show":
			m = memberMethod{name: "toDebugString", from: "derive:Show",
				fn: fnType{ret: baseType("String")}}
		default:
			continue
		}
		m.line, m.col = clauseLine, clauseCol
		ms = append(ms, m)
	}
	return ms
}

// addMethod registers one method into a declaration's member set under the
// collision rule (E0814): a field precedes every method source, an
// inherent method precedes interface and derive methods, an inherited
// default against an inherent method names the inherent one, two
// interfaces collide at the impl head. anchor names the incoming method's
// token; implHead the enclosing impl's head (an inherited default's
// responsible declaration).
func (c *checker) addMethod(owner string, methods *[]memberMethod, hasField func(string) bool, m memberMethod, anchorLine, anchorCol, implHeadLine, implHeadCol int) {
	if hasField != nil && hasField(m.name) {
		c.fail(anchorLine, anchorCol, "E0814", fmt.Sprintf(
			"member name collision on one type — %q already has the field %q; one type gives one name one meaning",
			owner, m.name))
	}
	for i := range *methods {
		e := &(*methods)[i]
		if e.name != m.name {
			continue
		}
		if e.from == "inherent" {
			c.fail(anchorLine, anchorCol, "E0814", fmt.Sprintf(
				"member name collision on one type — %q already has the inherent method %q; one type gives one name one meaning",
				owner, m.name))
		}
		if strings.HasPrefix(e.from, "derive:") {
			c.fail(anchorLine, anchorCol, "E0814", fmt.Sprintf(
				"member name collision on one type — %q already has the derived method %q; one type gives one name one meaning",
				owner, m.name))
		}
		if m.from == "inherent" || strings.HasPrefix(m.from, "derive:") {
			c.fail(anchorLine, anchorCol, "E0814", fmt.Sprintf(
				"member name collision on one type — %q already receives %q from %q; one type gives one name one meaning",
				owner, m.name, e.from))
		}
		c.fail(implHeadLine, implHeadCol, "E0814", fmt.Sprintf(
			"member name collision on one type — %q receives %q from both %q and %q; one type gives one name one meaning",
			owner, m.name, e.from, m.from))
	}
	*methods = append(*methods, m)
}

// valueNoun names a nominal head's kind for E0812's message.
func valueNoun(t Type) string {
	switch t.(type) {
	case recordType:
		return "record"
	case newtypeType:
		return "newtype"
	case namedType:
		return "sum"
	}
	return "type"
}

// pushTypes/popTypes stack the generic-clause layers (innermost wins).
func (c *checker) pushTypes(m map[string]Type) { c.typeScope = append(c.typeScope, m) }
func (c *checker) popTypes()                   { c.typeScope = c.typeScope[:len(c.typeScope)-1] }

// lookupTypeScope walks the clause layers innermost-first.
func (c *checker) lookupTypeScope(name string) (Type, bool) {
	for i := len(c.typeScope) - 1; i >= 0; i-- {
		if t, ok := c.typeScope[i][name]; ok {
			return t, true
		}
	}
	return nil, false
}

// resolveArgs resolves a generic application's argument references.
func (c *checker) resolveArgs(refs []ast.TypeRef, slot slotKind) []Type {
	args := make([]Type, len(refs))
	for i, r := range refs {
		args[i] = c.resolveTypeRef(r, slot)
		// Every explicit instantiation passes here — an fn's clause, a
		// construction head, an interface clause — and chapter 13 holds
		// them all: a resource in a type-argument position is the box
		// route under another spelling (E1106, at the argument's own
		// reference; an inferred application carries no reference to
		// anchor and stays a ratified revision).
		if catOf(args[i]) == "resource" {
			line, col := refPos(r)
			c.fail(line, col, "E1106", fmt.Sprintf(
				"resource type in a composite position — %q appears as a generic argument at the instantiation; a resource type appears only in binding positions - a parameter, a let or scope-head binding, a return type",
				args[i].String()))
		}
	}
	return args
}

// arityFail is the E0828 judgment shared by every explicit-clause form:
// anchored at the clause's own token, falling back to the head's.
func (c *checker) arityFail(argLine, argCol, line, col int, name string, params, got int) {
	l, cc := argLine, argCol
	if l == 0 {
		l, cc = line, col
	}
	noun := "type arguments"
	if params == 1 {
		noun = "type argument"
	}
	c.fail(l, cc, "E0828", fmt.Sprintf(
		"type argument arity mismatch — %q wants %d %s, got %d; match the declaration's arity",
		name, params, noun, got))
}

// valueSlotIface holds the value-slot rule for bare interface names
// (E0821): an interface occupies type slots only as the erased box.
func (c *checker) valueSlotIface(x *ast.NamedType, args []Type) {
	box := x.Name
	if len(args) > 0 {
		box += "<" + typeJoin(args) + ">"
	}
	c.fail(x.Line, x.Col, "E0821", fmt.Sprintf(
		"interface name used as a value type — %q names an interface; the value type slot takes the box form %q",
		x.Name, "Dyn<"+box+">"))
}

// builtinIface reads the prelude's interface names (nil for none).
func builtinIface(name string) *ifaceInfo {
	switch name {
	case "Iterator":
		return iteratorIface
	case "Iterable":
		return iterableIface
	case "Eq":
		return eqIface
	case "Releasable":
		return releasableIface
	case "Hash":
		return hashIface
	case "Show":
		return showIface
	}
	return nil
}

// The builtin sums (chapter 15's prelude): Result with Ok and Err over
// the parameter positions, Option with Some and None over its one.
var (
	resultSum = &sumInfo{
		name:   "Result",
		params: []string{"T", "E"},
		variants: []variantInfo{
			{name: "Ok", payloads: []Type{paramRef{idx: 0, name: "T"}}},
			{name: "Err", payloads: []Type{paramRef{idx: 1, name: "E"}}},
		},
	}
	optionSum = &sumInfo{
		name:   "Option",
		params: []string{"T"},
		variants: []variantInfo{
			{name: "Some", payloads: []Type{paramRef{idx: 0, name: "T"}}},
			{name: "None"},
		},
	}

	// The std collection sums (chapter 17): their type-level shape — the
	// names, the clauses, the arity judgment (E0828) — resolves with this
	// milestone's generic application. The spec-anchored access family
	// (get/has, and the Iterable entry point) resolves; the rest of the
	// member surface is the standard library's, so a miss behind them
	// stops at the std-modules boundary (design D10 — never privately
	// rejected).
	listSum = &sumInfo{name: "List", params: []string{"T"}}
	mapSum  = &sumInfo{name: "Map", params: []string{"K", "V"}}
	setSum  = &sumInfo{name: "Set", params: []string{"T"}}

	// Range<T> is chapter 11's builtin generic type: one parameter bound
	// to chapter 7's eight integer types, value-category — a range value
	// is its two bounds and nothing more, so binding copies the pair —
	// constructed only by the range operator. It carries a builtin
	// Iterable<T> implementation (the iteration pass reads it); the
	// element type is not re-judged at a bare annotation application (no
	// ratified trigger names that position — the operand positions are
	// E0902's).
	rangeSum = &sumInfo{name: "Range", params: []string{"T"}, byval: true}
)

// collectionSum reports whether a nominal declaration is one of the std
// collection sums (whose member surface beyond the anchored families is
// the standard library's).
func collectionSum(decl *sumInfo) bool {
	return decl == listSum || decl == mapSum || decl == setSum
}

// terminationFnType reads the panic family's prelude signature (chapter
// 14): panic and todo take one String and produce Never (chapter 9's
// bottom type, satisfying any declared return), assert takes a Bool and a
// String and produces the unit value. They are ordinary functions — a
// user's same-named declaration wins in the module's name space (the
// symbols resolve first), a call takes the ordinary fn-value path, and no
// dedicated diagnostic exists for a missing message: the declaration
// requires it, unauditable termination is the alternative.
func terminationFnType(name string) fnType {
	if name == "assert" {
		return fnType{params: []Type{baseType("Bool"), baseType("String")}, ret: unitType{}}
	}
	return fnType{params: []Type{baseType("String")}, ret: neverType{}}
}

// The builtin interfaces (chapter 10's derive targets and chapter 11's
// protocol): the derive targets are marker faces whose methods derives
// clauses generate; Iterator and Iterable register their declared shape.
// The protocol's eleven combinator defaults arrive with the collections
// pass — their signatures carry the collection types (disclosed in the
// task record).
var (
	eqIface   = &ifaceInfo{name: "Eq", deriveTarget: true}
	hashIface = &ifaceInfo{name: "Hash", deriveTarget: true}
	showIface = &ifaceInfo{name: "Show", deriveTarget: true}

	iteratorIface = &ifaceInfo{name: "Iterator", params: []string{"T"}}
	iterableIface = &ifaceInfo{name: "Iterable", params: []string{"T"}}

	// Releasable is chapter 13's release protocol: every byres record
	// owns its release and implements it (E1101/E1102 hold the category
	// discipline; E0808 anchors the signature through the ordinary
	// interface machinery).
	releasableIface = &ifaceInfo{name: "Releasable"}
)

func init() {
	// Releasable (chapter 13): fn release(mut self) — valueless, the one
	// method every byres record's impl defines.
	releasableIface.methods = []ifaceMethod{{name: "release", recvMut: true}}
	// Iterator<T>: fn next(mut self) -> Option<T> — the protocol's one
	// non-defaulted method — plus the eleven combinator defaults
	// (chapter 11's Iterator combinators requirement): four lazy — each
	// returning a derived Dyn<Iterator> handle — and seven eager. map and
	// fold carry their own method clause <U>, fresh against the
	// interface's <T> (so U sits at clause position 1); an override must
	// repeat the clause exactly (E0808). A non-nil body is the defaulted
	// marker E0807 and the inherited registration (E0814) read; since M8
	// the eager six carry real bodies the walk checks at every check's
	// start (checkBuiltinCombinators), the lazy four and collect the
	// compiler-intrinsic marker.
	t0 := Type(paramRef{idx: 0, name: "T"})
	u1 := Type(paramRef{idx: 1, name: "U"})
	dynIter := func(elem Type) Type {
		return dynType{inf: iteratorIface, args: []Type{elem}}
	}
	fnOf := func(params []Type, ret Type) Type {
		return fnType{params: params, ret: ret}
	}
	// The real default bodies (design D3) build as the same AST nodes a
	// source body parses to — walked by the same machine at every check's
	// start (checkBuiltinCombinators), never a privileged path. The six
	// eager combinators state their algorithms in the approved surface
	// (match on self.next(), the closure parameters, recursion); the lazy
	// four and collect return values whose construction faces (a Dyn box,
	// a List) the M8 subset does not carry (design D10), so their defaults
	// stay the compiler-intrinsic marker — chapter 21 R9's
	// implementation-by-compiler reads the empty block as the honest
	// form, and the walk skips it as it does at a user interface.
	lit := func(kind, text string) *ast.Literal {
		return &ast.Literal{Kind: kind, Text: text, Line: stdBodyLine, Col: stdBodyCol}
	}
	id := func(name string) *ast.Ident {
		return &ast.Ident{Name: name, Line: stdBodyLine, Col: stdBodyCol}
	}
	call := func(fn ast.Expr, args ...ast.Expr) *ast.Call {
		return &ast.Call{Fn: fn, Args: args, Line: stdBodyLine, Col: stdBodyCol}
	}
	selfCall := func(name string, args ...ast.Expr) *ast.Call {
		return call(&ast.Member{Recv: id("self"), Name: name, Line: stdBodyLine, Col: stdBodyCol, NameLine: stdBodyLine, NameCol: stdBodyCol}, args...)
	}
	stmt := func(e ast.Expr) ast.Stmt {
		return &ast.ExprStmt{Expr: e, Line: stdBodyLine, Col: stdBodyCol}
	}
	block := func(items ...ast.Stmt) *ast.BlockExpr {
		return &ast.BlockExpr{Block: ast.Block{Items: items, Line: stdBodyLine, Col: stdBodyCol}, Line: stdBodyLine, Col: stdBodyCol}
	}
	body := func(items ...ast.Stmt) *ast.Block {
		return &ast.Block{Items: items, Line: stdBodyLine, Col: stdBodyCol}
	}
	closure := func(names []string, e ast.Expr) *ast.Closure {
		ps := make([]ast.Param, len(names))
		for i, n := range names {
			ps[i] = ast.Param{Name: n, NameLine: stdBodyLine, NameCol: stdBodyCol}
		}
		return &ast.Closure{Short: true, Params: ps, Body: ast.Block{Items: []ast.Stmt{stmt(e)}, Line: stdBodyLine, Col: stdBodyCol}, Line: stdBodyLine, Col: stdBodyCol}
	}
	// The Option<T> the eager bodies produce: a let annotation is the one
	// green position for Some/None without a surrounding expected (match
	// arms thread none — chapter 4's rule, not a gap), so the bodies name
	// their results through typed bindings.
	optionT := func() *ast.NamedType {
		return &ast.NamedType{Name: "Option", Args: []ast.TypeRef{&ast.NamedType{Name: "T", Line: stdBodyLine, Col: stdBodyCol}}, Line: stdBodyLine, Col: stdBodyCol}
	}
	letOption := func(name string, init ast.Expr) *ast.Binding {
		return &ast.Binding{Kw: "let", Name: name, Typ: optionT(), Init: init, Line: stdBodyLine, Col: stdBodyCol, NameLine: stdBodyLine, NameCol: stdBodyCol}
	}
	armSome := func(bind string, e ast.Expr) ast.MatchArm {
		return ast.MatchArm{
			Pat:  &ast.PatVariant{Name: "Some", Args: []ast.Pattern{&ast.PatBinding{Name: bind, Line: stdBodyLine, Col: stdBodyCol}}, Line: stdBodyLine, Col: stdBodyCol},
			Body: e, Line: stdBodyLine, Col: stdBodyCol,
		}
	}
	armNone := func(e ast.Expr) ast.MatchArm {
		return ast.MatchArm{Pat: &ast.PatVariant{Name: "None", Line: stdBodyLine, Col: stdBodyCol}, Body: e, Line: stdBodyLine, Col: stdBodyCol}
	}
	nextMatch := func(bind string, some, none ast.Expr) *ast.Match {
		return &ast.Match{Scrutinee: selfCall("next"), Arms: []ast.MatchArm{armSome(bind, some), armNone(none)}, Line: stdBodyLine, Col: stdBodyCol}
	}
	ifExpr := func(cond, then, els ast.Expr) *ast.If {
		return &ast.If{Cond: cond, Then: ast.Block{Items: []ast.Stmt{stmt(then)}, Line: stdBodyLine, Col: stdBodyCol}, Else: els, Line: stdBodyLine, Col: stdBodyCol}
	}
	// fold: match self.next() { Some(x) => self.fold(f(init, x), f), None => init }
	foldBody := body(stmt(nextMatch("x",
		selfCall("fold", call(id("f"), id("init"), id("x")), id("f")),
		id("init"),
	)))
	// reduce: match self.next() {
	//   Some(first) => { let folded: Option<T> = Some(self.fold(first, |acc, x| f(acc, x))); folded }
	//   None => { let none: Option<T> = None; none }
	// }
	reduceBody := body(stmt(nextMatch("first",
		block(letOption("folded", call(id("Some"), selfCall("fold", id("first"), closure([]string{"acc", "x"}, call(id("f"), id("acc"), id("x")))))), stmt(id("folded"))),
		block(letOption("none", id("None")), stmt(id("none"))),
	)))
	// count: self.fold(0, |acc, _| acc + 1)
	countBody := body(stmt(selfCall("fold",
		lit("int", "0"),
		closure([]string{"acc", "_"}, &ast.Binary{Op: "+", L: id("acc"), R: lit("int", "1"), Line: stdBodyLine, Col: stdBodyCol}),
	)))
	// any: match self.next() { Some(x) => if f(x) { true } else { self.any(f) }, None => false }
	anyBody := body(stmt(nextMatch("x",
		ifExpr(call(id("f"), id("x")), lit("bool", "true"), block(stmt(selfCall("any", id("f"))))),
		lit("bool", "false"),
	)))
	// all: match self.next() { Some(x) => if f(x) { self.all(f) } else { false }, None => true }
	allBody := body(stmt(nextMatch("x",
		ifExpr(call(id("f"), id("x")), block(stmt(selfCall("all", id("f")))), lit("bool", "false")),
		lit("bool", "true"),
	)))
	// find: match self.next() {
	//   Some(x) => { let hit: Option<T> = Some(x); let out = if f(x) { hit } else { self.find(f) }; out }
	//   None => { let none: Option<T> = None; none }
	// }
	// (the if binds through `out`: a plain block's final item carries its
	// value, and an if there is a statement position — the block-value
	// rule reads the binding, chapter 7's subset as it stands)
	findBody := body(stmt(nextMatch("x",
		block(letOption("hit", call(id("Some"), id("x"))),
			&ast.Binding{Kw: "let", Name: "out", Init: ifExpr(call(id("f"), id("x")), id("hit"), block(stmt(selfCall("find", id("f"))))), Line: stdBodyLine, Col: stdBodyCol, NameLine: stdBodyLine, NameCol: stdBodyCol},
			stmt(id("out"))),
		block(letOption("none", id("None")), stmt(id("none"))),
	)))
	iteratorIface.methods = []ifaceMethod{
		{
			name:    "next",
			recvMut: true,
			ret:     namedType{decl: optionSum, args: []Type{t0}},
		},
		// fn map<U>(mut self, f: fn(T) -> U) -> Dyn<Iterator<U>>
		{
			name: "map", recvMut: true, typeParams: []string{"U"}, body: &ast.Block{},
			paramNames: []string{"f"},
			params:     []Type{fnOf([]Type{t0}, u1)},
			ret:        dynIter(u1),
		},
		// fn filter(mut self, f: fn(T) -> Bool) -> Dyn<Iterator<T>>
		{
			name: "filter", recvMut: true, body: &ast.Block{},
			paramNames: []string{"f"},
			params:     []Type{fnOf([]Type{t0}, baseType("Bool"))},
			ret:        dynIter(t0),
		},
		// fn take(mut self, n: Int64) -> Dyn<Iterator<T>>
		{
			name: "take", recvMut: true, body: &ast.Block{},
			paramNames: []string{"n"},
			params:     []Type{baseType("Int64")},
			ret:        dynIter(t0),
		},
		// fn skip(mut self, n: Int64) -> Dyn<Iterator<T>>
		{
			name: "skip", recvMut: true, body: &ast.Block{},
			paramNames: []string{"n"},
			params:     []Type{baseType("Int64")},
			ret:        dynIter(t0),
		},
		// fn collect(mut self) -> List<T>
		{
			name: "collect", recvMut: true, body: &ast.Block{},
			ret: namedType{decl: listSum, args: []Type{t0}},
		},
		// fn fold<U>(mut self, init: U, f: fn(U, T) -> U) -> U
		{
			name: "fold", recvMut: true, typeParams: []string{"U"}, body: foldBody,
			paramNames: []string{"init", "f"},
			params:     []Type{u1, fnOf([]Type{u1, t0}, u1)},
			ret:        u1,
		},
		// fn reduce(mut self, f: fn(T, T) -> T) -> Option<T>
		{
			name: "reduce", recvMut: true, body: reduceBody,
			paramNames: []string{"f"},
			params:     []Type{fnOf([]Type{t0, t0}, t0)},
			ret:        namedType{decl: optionSum, args: []Type{t0}},
		},
		// fn count(mut self) -> Int64
		{
			name: "count", recvMut: true, body: countBody,
			ret: baseType("Int64"),
		},
		// fn any(mut self, f: fn(T) -> Bool) -> Bool
		{
			name: "any", recvMut: true, body: anyBody,
			paramNames: []string{"f"},
			params:     []Type{fnOf([]Type{t0}, baseType("Bool"))},
			ret:        baseType("Bool"),
		},
		// fn all(mut self, f: fn(T) -> Bool) -> Bool
		{
			name: "all", recvMut: true, body: allBody,
			paramNames: []string{"f"},
			params:     []Type{fnOf([]Type{t0}, baseType("Bool"))},
			ret:        baseType("Bool"),
		},
		// fn find(mut self, f: fn(T) -> Bool) -> Option<T>
		{
			name: "find", recvMut: true, body: findBody,
			paramNames: []string{"f"},
			params:     []Type{fnOf([]Type{t0}, baseType("Bool"))},
			ret:        namedType{decl: optionSum, args: []Type{t0}},
		},
	}
	// Iterable<T>: type Iter; fn iterator(self) -> Iter — Iter is the
	// associated position; the impl's binding must implement Iterator<T>
	// for the same elements (E0904, the iteration pass).
	iterableIface.assocs = []string{"Iter"}
	iterableIface.methods = []ifaceMethod{{
		name: "iterator",
		ret:  assocRef{iface: iterableIface, name: "Iter"},
	}}
}

// The base types' builtin members. String carries the spec-anchored
// families: the iteration entry point (chapter 11) — a String iterates
// runes, so iterator hands the Iterator<Rune> face, the handle type
// itself the standard library's — and the two-layer access family
// (chapter 17): byteLength and byteSlice on the byte layer, runeCount
// and charAt on the code-point layer. The rest of the stdlib surface —
// the conversion methods among them — stays behind the std-modules
// boundary: a member no anchored family names is never privately
// rejected.
var stringMembers = map[string]fnType{
	"iterator":   {ret: ifaceType{decl: iteratorIface, args: []Type{baseType("Rune")}}},
	"byteLength": {ret: baseType("Int64")},
	"byteSlice":  {params: []Type{baseType("Int64"), baseType("Int64")}, ret: baseType("String")},
	"runeCount":  {ret: baseType("Int64")},
	"charAt":     {params: []Type{baseType("Int64")}, ret: baseType("Rune")},
}

// collectionMembers holds each std collection's spec-anchored members:
// the access family (chapter 17 — List.get and Map.get return Option,
// Set.has returns Bool) and the iteration entry point (chapter 11
// ratifies the three as iterables; Map iterates (K, V) entries). The
// mutation surface and everything beyond is the standard library's
// (design D10 — a miss stops at the std-modules boundary).
func collectionMembers(t namedType) map[string]fnType {
	m := map[string]fnType{}
	switch t.decl {
	case listSum:
		m["iterator"] = fnType{ret: ifaceType{decl: iteratorIface, args: []Type{t.args[0]}}}
		m["get"] = fnType{
			params: []Type{baseType("Int64")},
			ret:    namedType{decl: optionSum, args: []Type{t.args[0]}},
		}
	case mapSum:
		m["iterator"] = fnType{ret: ifaceType{decl: iteratorIface, args: []Type{tupleType{elems: t.args}}}}
		m["get"] = fnType{
			params: []Type{t.args[0]},
			ret:    namedType{decl: optionSum, args: []Type{t.args[1]}},
		}
	case setSum:
		m["iterator"] = fnType{ret: ifaceType{decl: iteratorIface, args: []Type{t.args[0]}}}
		m["has"] = fnType{
			params: []Type{t.args[0]},
			ret:    baseType("Bool"),
		}
	}
	return m
}

// The chapter 18 shared-state types (concurrency): checker-side nominal
// declarations — the List/Map precedent, no synthetic source, for faces a
// We file cannot write (the builtin types carry builtin surfaces). The
// four NAMED sums beside them (TimeoutError, TaskPanic, SendResult,
// ReceiveResult) are real declarations in the std.concurrent module's
// synthetic file instead — We-writable shapes the same ingest machinery
// rides. Every type here is gc-category by construction (sumInfo without
// the byval flag); the closed member sets live in concurrentMembers.
var (
	mutexSum        = &sumInfo{name: "Mutex", params: []string{"T"}}
	rwlockSum       = &sumInfo{name: "RwLock", params: []string{"T"}}
	atomicSum       = &sumInfo{name: "Atomic", params: []string{"T"}}
	atomicRefSum    = &sumInfo{name: "AtomicRef", params: []string{"T"}}
	condSum         = &sumInfo{name: "Cond", params: []string{"T"}}
	semaphoreSum    = &sumInfo{name: "Semaphore"}
	channelSum      = &sumInfo{name: "Channel", params: []string{"T"}}
	sendOnlySum     = &sumInfo{name: "SendOnly", params: []string{"T"}}
	receiveOnlySum  = &sumInfo{name: "ReceiveOnly", params: []string{"T"}}
	taskHandleSum   = &sumInfo{name: "TaskHandle", params: []string{"T"}}
	cancelSignalSum = &sumInfo{name: "CancelSignal"}
)

// concurrentTypes indexes the shared-state declarations for the qualified
// resolution path (conc.Mutex through the import) — the bare names stay
// unresolved (the qualified spellings are the reachable ones).
var concurrentTypes = map[string]*sumInfo{
	"Mutex": mutexSum, "RwLock": rwlockSum, "Atomic": atomicSum,
	"AtomicRef": atomicRefSum, "Cond": condSum, "Semaphore": semaphoreSum,
	"Channel": channelSum, "SendOnly": sendOnlySum,
	"ReceiveOnly": receiveOnlySum, "TaskHandle": taskHandleSum,
	"CancelSignal": cancelSignalSum,
}

// concurrentType reports whether a nominal declaration is one of the
// chapter 18 shared-state types (whose member sets are closed — a miss is
// E0816, never the stdlib boundary).
func concurrentType(decl *sumInfo) bool {
	_, ok := concurrentTypes[decl.name]
	return ok
}

// shareableMarker is chapter 18's compiler-attached marker: an interface
// declaration with no clause, no associated types, and no method set — a
// bound on it grants nothing and its application dispatches to the shape
// judgment (shareableOf), never to an implements walk. The marker occupies
// no value surface (a bound, never a parameter or a box) and is
// prelude-visible — no import gates the name (the panic family's
// precedent).
var shareableMarker = &ifaceInfo{name: "Shareable"}

// shareableOf is the Shareable membership judgment (chapter 18): a type is
// Shareable exactly when it belongs to the closed set — the base types,
// unit, value records whose every field is Shareable, value sums whose
// every payload is Shareable, tuples of Shareable types, and the
// synchronized gc classes (the declarer alone decides — the type arguments
// never descend: an Atomic argument is already an atomic base type).
// Everything else is outside: closures and fn values, non-synchronized gc
// (the collections, gc records), resources (their captures walk E1604's
// own gate), boxes, and the bottom type.
func shareableOf(t Type) bool {
	switch x := t.(type) {
	case baseType:
		return true
	case unitType:
		return true
	case recordType:
		if x.decl.cat != "value" {
			return false
		}
		for _, f := range x.decl.fields {
			ft := f.typ
			if len(x.args) > 0 {
				ft = subst(ft, x.args, nil)
			}
			if !shareableOf(ft) {
				return false
			}
		}
		return true
	case namedType:
		if concurrentType(x.decl) {
			return true
		}
		if !x.decl.byval {
			return false
		}
		for _, v := range x.decl.variants {
			for _, p := range v.payloads {
				pt := p
				if len(x.args) > 0 {
					pt = subst(pt, x.args, nil)
				}
				if !shareableOf(pt) {
					return false
				}
			}
		}
		return true
	case tupleType:
		for _, e := range x.elems {
			if !shareableOf(e) {
				return false
			}
		}
		return true
	}
	return false
}

// shareableBoundArg is the Shareable judgment at a bound's application: a
// clause position carrying the bound hands its parameter through (M6a's
// bound-carried machine — the same walk implementsFace reads), anything
// else answers the closed set.
func (c *checker) shareableBoundArg(t Type) bool {
	if pr, ok := t.(paramRef); ok {
		if c.fnBounds == nil {
			return false
		}
		for _, b := range c.fnBounds.bounds {
			if b.idx == pr.idx && b.face.decl == shareableMarker {
				return true
			}
		}
		return false
	}
	return shareableOf(t)
}

// concurrentMembers holds each shared-state type's closed member set
// (chapter 18): the synchronized state cells' update/get/set family,
// RwLock's read, the cell-specific faces, and the channel and handle
// methods. The views are pure fn types — waiting performs no effect
// (chapter 16's carve-out lands as signature purity). RwLock.read's own
// clause position (U) stays symbolic past the receiver's positions: the
// call's text determines it, the method-call machine's ordinary course.
// The three named sums a return mentions resolve through the module's
// bucket — the pointer-identical declarations the ingest built.
func (c *checker) concurrentMembers(t namedType) map[string]fnType {
	cell := func() map[string]fnType {
		e := paramRef{idx: 0, name: "T"}
		return map[string]fnType{
			"update": {params: []Type{fnType{params: []Type{e}, ret: e}}, ret: e},
			"get":    {ret: e},
			"set":    {params: []Type{e}, ret: unitType{}},
		}
	}
	opt := func(inner Type) Type {
		return namedType{decl: optionSum, args: []Type{inner}}
	}
	switch t.decl {
	case mutexSum, atomicSum, atomicRefSum:
		return cell()
	case rwlockSum:
		m := cell()
		e := paramRef{idx: 0, name: "T"}
		u := paramRef{idx: 1, name: "U"}
		m["read"] = fnType{params: []Type{fnType{params: []Type{e}, ret: u}}, ret: u}
		return m
	case condSum:
		e := paramRef{idx: 0, name: "T"}
		return map[string]fnType{
			"wait":      {params: []Type{fnType{params: []Type{e}, ret: baseType("Bool")}}, ret: unitType{}},
			"signal":    {ret: unitType{}},
			"broadcast": {ret: unitType{}},
		}
	case semaphoreSum:
		return map[string]fnType{
			"acquire":      {ret: unitType{}},
			"tryAcquire":   {ret: baseType("Bool")},
			"release":      {ret: unitType{}},
			"currentCount": {ret: baseType("Int64")},
		}
	case channelSum, sendOnlySum:
		e := paramRef{idx: 0, name: "T"}
		m := map[string]fnType{
			"send": {params: []Type{e}, ret: unitType{}},
			"trySend": {params: []Type{e},
				ret: namedType{decl: c.stdConcSum("SendResult")}},
			"close": {ret: unitType{}},
		}
		if t.decl == channelSum {
			m["receive"] = fnType{ret: opt(e)}
			m["tryReceive"] = fnType{
				ret: namedType{decl: c.stdConcSum("ReceiveResult"), args: []Type{e}},
			}
			m["toSendOnly"] = fnType{ret: namedType{decl: sendOnlySum, args: t.args}}
			m["toReceiveOnly"] = fnType{ret: namedType{decl: receiveOnlySum, args: t.args}}
		}
		return m
	case receiveOnlySum:
		e := paramRef{idx: 0, name: "T"}
		return map[string]fnType{
			"receive":    {ret: opt(e)},
			"tryReceive": {ret: namedType{decl: c.stdConcSum("ReceiveResult"), args: []Type{e}}},
		}
	case taskHandleSum:
		e := paramRef{idx: 0, name: "T"}
		return map[string]fnType{
			"await":  {ret: namedType{decl: resultSum, args: []Type{e, namedType{decl: c.stdConcSum("TaskPanic")}}}},
			"cancel": {ret: unitType{}},
		}
	case cancelSignalSum:
		return map[string]fnType{
			"isCancelled":    {ret: baseType("Bool")},
			"awaitCancelled": {ret: unitType{}},
		}
	}
	return nil
}

// stdConcSum reads one named sum of the std.concurrent module from the
// ingest-built bucket — the pointer-identical declaration every return
// view must mention (the sameType machine keys on the pointer).
func (c *checker) stdConcSum(name string) *sumInfo {
	bucket := c.modules["std.concurrent"]
	if sym, ok := bucket[name]; ok && sym.kind == symType {
		return sym.sum
	}
	return &sumInfo{name: name}
}

// The prelude's base type names (chapter 7's base types; Bytes has no
// literal form — its values come from methods, the standard library's).
var baseNames = map[string]bool{
	"Int8": true, "Int16": true, "Int32": true, "Int64": true,
	"UInt8": true, "UInt16": true, "UInt32": true, "UInt64": true,
	"Float32": true, "Float64": true,
	"Bool": true, "String": true, "Rune": true, "Bytes": true,
}

var intNames = map[string]bool{
	"Int8": true, "Int16": true, "Int32": true, "Int64": true,
	"UInt8": true, "UInt16": true, "UInt32": true, "UInt64": true,
}

// intBounds holds each integer type's closed range, built once.
var intBounds = map[string]*[2]big.Int{}

func init() {
	fill := func(name string, lo, hi int64) {
		intBounds[name] = &[2]big.Int{*big.NewInt(lo), *big.NewInt(hi)}
	}
	fill("Int8", -128, 127)
	fill("Int16", -32768, 32767)
	fill("Int32", -2147483648, 2147483647)
	fill("Int64", -9223372036854775808, 9223372036854775807)
	u := func(name string, bits int) {
		hi := new(big.Int).Lsh(big.NewInt(1), uint(bits))
		hi.Sub(hi, big.NewInt(1))
		intBounds[name] = &[2]big.Int{*big.NewInt(0), *hi}
	}
	u("UInt8", 8)
	u("UInt16", 16)
	u("UInt32", 32)
	u("UInt64", 64)
}

func inRange(v *big.Int, t Type) bool {
	name, ok := t.(baseType)
	if !ok {
		return true
	}
	bounds, known := intBounds[string(name)]
	if !known {
		return true
	}
	return v.Cmp(&bounds[0]) >= 0 && v.Cmp(&bounds[1]) <= 0
}

func isInt(t Type) bool {
	b, ok := t.(baseType)
	return ok && intNames[string(b)]
}

func isNumeric(t Type) bool {
	b, ok := t.(baseType)
	if !ok {
		return false
	}
	return intNames[string(b)] || b == "Float32" || b == "Float64"
}

// isString reports whether t is the base String type — the concatenation
// and equality domain of design D3.
func isString(t Type) bool {
	b, ok := t.(baseType)
	return ok && b == "String"
}

func isUnit(t Type) bool {
	_, ok := t.(unitType)
	return ok
}

// --- module symbols ----------------------------------------------------------

type symKind int

const (
	symFn symKind = iota
	symLet
	symType
	symVariant
	symImport
	symRecord
	symNewtype
	symIface
	symEffect
)

type symbol struct {
	kind    symKind
	fn      *ast.FnDecl
	letType Type // nil until the binding's initializer is typed (source order)
	sum     *sumInfo
	rec     *recordInfo
	nt      *newtypeInfo
	iface   *ifaceInfo
	vi      int // variant index into sum
	// mod is the import's canonical module key (symImport alone): the
	// dotted path the loader resolved the import to.
	mod string
	// pub is the declaration's pub bit (chapter 15): cross-module reach
	// is exactly the qualified form, and only for pub items (E1303).
	pub bool
}

// --- the checker -------------------------------------------------------------

type checker struct {
	file     string
	mode     Mode
	syms     map[string]*symbol
	locals   []map[string]Type
	fnRet    Type // the enclosing fn's declared return; nil = valueless
	fnParams map[*ast.FnDecl][]Type
	fnRets   map[*ast.FnDecl]Type
	// fnTags holds each fn or method declaration's resolved effect segment
	// (chapter 16): canonical keys, first-occurrence order (design D3).
	// nil/absent = pure. AST-keyed, so one checker's every module shares it.
	fnTags map[*ast.FnDecl][]string
	// modKey is the live module's canonical key (the ingest argument):
	// a bare custom tag canonicalizes against it, and the qualified
	// resolution reads the same spelling back from the import's bucket.
	modKey string
	// The body context of the declaration whose body is being walked
	// (chapter 16, design D4): bodyTags/bodyName carry its resolved effect
	// segment and its name — every call in the body judges its callee's
	// set against bodyTags (E1401), and the message names bodyName.
	// inBody is false outside any body walk (a top-level initializer is
	// E1405's own face); inClosure suspends the judgment inside a closure
	// body — constructing a closure is not performing it, and the body's
	// calls join the closure's own inferred set (design D6). A defer body
	// keeps the enclosing context: chapter 3 shifts its timing, never its
	// ownership.
	bodyTags  []string
	bodyName  string
	inBody    bool
	inClosure bool
	// bodyTest is the chapter 20 driver sentinel (design D3): a test
	// block's own body walk raises it, and checkCallEffect stands down —
	// a test is driver code (chapter 16:36), its calls judged by no
	// declared segment. A task body inside resets to its own segment
	// (taskType's swap), a mock body to the target's (design D4), and a
	// closure body keeps its inference face (the inClosure branch reads
	// first); the plain control forms inherit, like every body context.
	bodyTest bool
	// testExtent is the chapter 20 clock depth (design D7): 1 inside a
	// test block's extent — the body itself and, without a swap, the task
	// and scope bodies inside it (the test's own extent, chapter 20:102)
	// — and 0 in a closure or mock body and everywhere outside a test.
	// advanceTime's name-value and call faces gate on it (E1806).
	testExtent int
	// mockSeen is the live test block's mock identities (design D6): the
	// resolved (module key, fn name) pairs it has mocked — one block
	// mocks one target at most once (E1805). checkTestDecl swaps a fresh
	// map in for each block; blocks are independent by chapter 20:71.
	mockSeen map[string]bool
	// The M11 advisory collector (design D4): advisories holds the
	// W-severity findings of the walk; advRoot arms collection for the
	// graph's root module alone (the std bodies, the builtin combinators,
	// and every dependency module walk the same hooks and stay silent —
	// the advisory surface is the compiled root's own code); advInTest
	// spans a whole test body subtree (a task or mock body inside keeps
	// it — chapter 20's calls are real there, unlike bodyTest, which
	// resets per body kind); advPend carries a test block's unmocked
	// custom-effect calls to their settle at the block's exit, where the
	// complete mockSeen set decides (a mock anywhere in the block covers
	// every call in it — the runtime installs at block start).
	advisories []diag.Diagnostic
	advRoot    bool
	advInTest  bool
	advPend    []pendingW1910
	// topLetName carries the binding name of the top-level initializer
	// being walked (chapter 16, design D8): that walk runs outside any
	// declaration body (inBody false), so the first effectful call it
	// types is E1405's — the one ratified position that evaluates outside
	// a signature, and it must stay pure.
	topLetName string
	// letName carries the name of the binding whose initializer is being
	// typed (chapter 18, design D2): the expectation-free channel
	// construction names it in E1617's context — the one diagnostic whose
	// text reads the binding, not the expression. A destructure carries no
	// single name; its initializer walks under the empty string.
	letName string
	// The chapter 18 lexical context (design D6): scopeDepth counts the
	// enclosing compound-scope bodies — a task block must sit inside one
	// (E1618; the compound forms alone join handles) — and taskDepth the
	// enclosing task-block bodies (currentCancelSignal's lexical rule,
	// E1608). Both cross closure bodies: the containment is lexical,
	// chapter 18's own word. taskVal/inTaskBody hold the task body's
	// inferred value judgment — every early exit and the tail expression
	// must agree, no coercion ever inserted — reset by a closure body,
	// whose returns are its own; bodyTask marks that the live bodyTags
	// name a task extent, so E1401's message addresses the task.
	scopeDepth int
	taskDepth  int
	taskVal    Type
	inTaskBody bool
	bodyTask   bool
	// varScopes tracks var bindings by block layer (chapter 18, design
	// D4): the task-capture pre-pass reads it back — a var capture is
	// E1603 whatever its type. Only walkItems' binding case ever writes
	// it (parameters, patterns, and scope heads are let-shaped), so the
	// layers stay one-per-block like the locals stack it mirrors.
	varScopes []map[string]bool
	// typeScope is the generic-clause layer stack: a declaration's clause
	// parameters (and an impl's associated bindings) shadow the module
	// namespace while its annotations and bodies resolve (chapter 10).
	typeScope []map[string]Type
	// impls holds the validated impls in source order — E0809's uniqueness
	// scan and the bodies pass read them back.
	impls []*implInfo
	// recvMut marks that the enclosing method body runs under mut self
	// (E0813's one legal field-write site).
	recvMut bool
	// fnWheres and implWheres hold each declaration's validated where
	// clause (nil = none); fnBounds is the clause active in the body being
	// typed — a bounded parameter's member set and satisfaction read it
	// (design D6).
	fnWheres   map[*ast.FnDecl]*whereInfo
	implWheres map[*ast.ImplDecl]*whereInfo
	fnBounds   *whereInfo
	// The capture ledger (design D9): one layer per active closure, with
	// the locals-stack depth the closure entered at. A name use settling
	// below an entry's boundary is a capture of that closure — nested
	// closures record into every crossed layer (a pass-through capture is
	// the outer closure's too).
	closures      []map[string]captureInfo
	closureBounds []int
	// closureTags is chapter 16's inference ledger (design D6): one layer
	// per active closure — the union of the callee sets of the calls its
	// body performs, recorded into the innermost layer only (a nested
	// closure's body is its own inference, never the outer's).
	// closureType reads the layer back as the closure value's own fn-type
	// segment.
	closureTags [][]string
	// resLets records each let's resource category as the walk types it
	// (chapter 13): the liveness pass (resource.go) reads the facts back
	// when it re-walks a typed-clean body.
	resLets map[*ast.Binding]bool
	// prop is the live propagation context (chapter 14, design D7): the
	// innermost enclosing function body's identity and declared return.
	// Every body entry (fn, method, closure) swaps it; a defer body walk
	// swaps in the no-return context. Never nil after Check's
	// initialization — the zero value is the module top level.
	prop *propCtx
	// modules holds every ingested module's namespace keyed by its
	// canonical dotted path (chapter 15, design D9): syms is the live
	// module's own bucket — one namespace per module is E1303's judgment
	// base. isRoot gates the main convention to the root module alone;
	// noMain lifts it for a test-module root (M10b design D6 — the entry
	// point there is the synthesized driver, so E1305 does not bind).
	modules map[string]map[string]*symbol
	isRoot  bool
	noMain  bool
}

// propCtx is the propagation context of one body (chapter 14, design D7):
// which function the innermost enclosing body belongs to, its declared
// return, and the two positions that carry no return — a defer body and
// the module top level (E1202's shapes). A short closure is the one
// inferring context: its `?`s fix the closure's value type from the
// operands, agreeing on one error type.
type propCtx struct {
	fnName    string // "" at the module top level; "(closure)" in a closure
	ret       Type   // the body's declared return; nil = valueless
	deferBody bool   // a defer body: it runs at exit, no return to reach
	short     bool   // a short closure's body: `?` fixes the value type
	shortErr  Type   // the first `?`'s error type — every later `?` agrees
}

// captureInfo is one ledger entry: the captured binding's ownership
// category (gc = live reference, value = copy snapshot, resource =
// rejected at the use) and the first use's position.
type captureInfo struct {
	cat  string
	line int
	col  int
}

// Check runs the type stage over one parsed module. It returns the first
// diagnostic, or a NotImplemented boundary, or both nil on a clean check.
func Check(f *ast.File, file string, mode Mode) (d *diag.Diagnostic, ni *NotImplemented) {
	c := newChecker(mode)
	defer stopTo(&d, &ni)
	// The stdlib's own faces check first, every time (design D3): the
	// builtin combinators' real default bodies walk the same method-body
	// machine a user interface's defaults do, over the empty module
	// scope. ingest overwrites the file name after the walk.
	c.file = stdBodyFile
	c.checkBuiltinCombinators()
	// Check takes one self-contained file: the compiler-provided std
	// modules it imports ingest first — the same provided-before-importing
	// order the project loader's graph produces for project checks.
	for _, key := range stdImports(f) {
		if sf, ok := StdModule(key); ok {
			c.ingest(sf, key, key, false)
		}
	}
	c.ingest(f, file, "main", true)
	return nil, nil
}

// Module is one imported module of a project's graph (chapter 15): the
// loader resolved it, read it, parsed it, and ordered it — Key is the
// canonical dotted path, Path the file the diagnostics anchor against.
type Module struct {
	Key  string
	Path string
	File *ast.File
}

// CheckTestRoot types one test module as its own graph root (M10b design
// D6): the same stdlib-first walk and the same provided-before-importing
// dependency ingest as a project check, but the main convention does not
// bind — a test module's entry point is the synthesized driver, not
// src/main.we's fn main. Each test module of a run checks as its own
// root over its own dependency slice.
func CheckTestRoot(root *ast.File, rootPath, rootKey string, deps []Module) (d *diag.Diagnostic, ni *NotImplemented) {
	c := newChecker(Project)
	defer stopTo(&d, &ni)
	c.file = stdBodyFile
	c.checkBuiltinCombinators()
	for _, m := range deps {
		c.ingest(m.File, m.Path, m.Key, false)
	}
	c.noMain = true
	c.ingest(root, rootPath, rootKey, true)
	return nil, nil
}

// CheckProject runs the type stage over a multi-module project (design
// D9): the dependency modules first, in the loader's post-order (the
// imported check before the importing — the deterministic initialization
// order), then the root module, whose main convention binds it alone.
// A diagnostic in any module stops the whole check (the first error
// wins, whatever module holds it).
func CheckProject(root *ast.File, rootPath string, deps []Module) (d *diag.Diagnostic, ni *NotImplemented) {
	c := newChecker(Project)
	defer stopTo(&d, &ni)
	// The same stdlib-first walk a single-file check runs (design D3).
	c.file = stdBodyFile
	c.checkBuiltinCombinators()
	for _, m := range deps {
		c.ingest(m.File, m.Path, m.Key, false)
	}
	c.ingest(root, rootPath, "main", true)
	return nil, nil
}

// newChecker builds one checker over a single-module check. The maps the
// declaration passes fill are AST-keyed (safe across every module one
// checker ingests); the namespace is per-module — ingest swaps it.
func newChecker(mode Mode) *checker {
	return &checker{
		mode:       mode,
		syms:       map[string]*symbol{},
		modules:    map[string]map[string]*symbol{},
		isRoot:     true,
		fnParams:   map[*ast.FnDecl][]Type{},
		fnRets:     map[*ast.FnDecl]Type{},
		fnTags:     map[*ast.FnDecl][]string{},
		fnWheres:   map[*ast.FnDecl]*whereInfo{},
		implWheres: map[*ast.ImplDecl]*whereInfo{},
		resLets:    map[*ast.Binding]bool{},
		prop:       &propCtx{},
	}
}

// stopTo catches the checker's panic-based stop (fail and bnd panic out
// of whatever depth the walk sits at), landing the diagnostic or
// boundary in the entry's named returns; anything else re-panics.
func stopTo(d **diag.Diagnostic, ni **NotImplemented) {
	if r := recover(); r != nil {
		switch s := r.(type) {
		case stop:
			*d, *ni = &s.d, nil
		case bstop:
			*d, *ni = nil, &NotImplemented{What: s.what}
		default:
			panic(r)
		}
	}
}

// ingest checks one module of the graph under its own namespace: the
// bucket registers under the canonical key (the qualified-form resolution
// reads it back), the file swaps for the module's own anchors, and the
// declaration passes run over its tree. Per-body state resets between
// modules — the walks are symmetric, but the reset keeps one module's
// walk tail from ever bleeding into the next.
func (c *checker) ingest(f *ast.File, file, key string, root bool) {
	c.file = file
	c.isRoot = root
	c.modKey = key
	c.syms = map[string]*symbol{}
	c.modules[key] = c.syms
	c.locals = nil
	c.typeScope = nil
	c.fnRet = nil
	c.fnBounds = nil
	c.recvMut = false
	c.closures = nil
	c.closureBounds = nil
	c.prop = &propCtx{}
	c.checkModule(f)
}

func (c *checker) fail(line, col int, code, message string) {
	d := diag.Error(code, message).At(c.file, line, col)
	if h, ok := tHelps[code]; ok {
		d = d.WithHelp(h)
	}
	panic(stop{d})
}

func (c *checker) bnd(what string) { panic(bstop{what}) }

// deriveTargetNames reads a derives clause's targets (nil without one).
func deriveTargetNames(d *ast.DerivesClause) []string {
	if d == nil {
		return nil
	}
	return d.Targets
}

// checkModule walks the module in the pipeline's order: symbol
// registration, import resolution (module resolution precedes type
// checking, chapter 21's R2), the main convention (project mode), the
// annotation pass (every declaration's types resolve under its own generic
// clause; each derives clause checks and registers its generated methods,
// design D8), the impl pass (chapter 10's declaration rules in source
// order, design D3), then the binding initializers and the bodies in
// source order.
func (c *checker) checkModule(f *ast.File) {
	// Pass 1: register the module's names (the parser has verified the
	// one-module-one-name-space rule). The chapter 10 declarations carry
	// their clause names and derive targets here; their types resolve in
	// the annotation pass.
	var implDecls []*ast.ImplDecl
	for _, it := range f.Items {
		switch x := it.(type) {
		case *ast.Import:
			name := x.Alias
			if name == "" {
				name = x.Path[len(x.Path)-1]
			}
			c.syms[name] = &symbol{kind: symImport, mod: strings.Join(x.Path, ".")}
		case *ast.FnDecl:
			c.syms[x.Name] = &symbol{kind: symFn, fn: x, pub: x.Pub}
		case *ast.TopLet:
			if x.Binding.Name != "_" {
				c.syms[x.Binding.Name] = &symbol{kind: symLet, pub: x.Pub}
			}
		case *ast.SumDecl:
			sum := &sumInfo{
				name:    x.Name,
				params:  typeParamNames(x.TypeParams),
				byval:   x.Byval,
				derives: deriveTargetNames(x.Derives),
			}
			for _, v := range x.Variants {
				sum.variants = append(sum.variants, variantInfo{name: v.Name})
			}
			c.syms[x.Name] = &symbol{kind: symType, sum: sum, pub: x.Pub}
			for i, v := range x.Variants {
				c.syms[v.Name] = &symbol{kind: symVariant, sum: sum, vi: i, pub: x.Pub}
			}
		case *ast.RecordDecl:
			c.syms[x.Name] = &symbol{kind: symRecord, rec: &recordInfo{
				name:    x.Name,
				cat:     x.Cat,
				params:  typeParamNames(x.TypeParams),
				derives: deriveTargetNames(x.Derives),
			}, pub: x.Pub}
		case *ast.NewtypeDecl:
			c.syms[x.Name] = &symbol{kind: symNewtype, nt: &newtypeInfo{
				name:    x.Name,
				params:  typeParamNames(x.TypeParams),
				derives: deriveTargetNames(x.Derives),
			}, pub: x.Pub}
		case *ast.InterfaceDecl:
			inf := &ifaceInfo{name: x.Name, params: typeParamNames(x.TypeParams)}
			for _, a := range x.Assocs {
				inf.assocs = append(inf.assocs, a.Name)
			}
			c.syms[x.Name] = &symbol{kind: symIface, iface: inf, pub: x.Pub}
		case *ast.EffectDecl:
			// The parser has settled the name discipline (E0012, the
			// built-in conflict E1403, and the one-name-space rule E0404);
			// the checker's face is the pub bit alone (chapter 15) and the
			// declaration a bare or qualified tag resolves against.
			c.syms[x.Name] = &symbol{kind: symEffect, pub: x.Pub}
		case *ast.ImplDecl:
			implDecls = append(implDecls, x)
		case *ast.ForeignBlock:
			// Chapter 19's entries join the module's one name space under
			// their own kinds — a foreign fn is an ordinary symFn the call
			// machinery resolves by name, an opaque record an ordinary
			// symRecord carrying the opaque bit. No symbol kind of their
			// own: the boundary is a declaration site, not a namespace.
			for _, e := range x.Items {
				switch it := e.(type) {
				case *ast.FnDecl:
					c.syms[it.Name] = &symbol{kind: symFn, fn: it, pub: it.Pub}
				case *ast.RecordDecl:
					c.syms[it.Name] = &symbol{kind: symRecord, rec: &recordInfo{
						name:    it.Name,
						cat:     it.Cat,
						params:  typeParamNames(it.TypeParams),
						derives: deriveTargetNames(it.Derives),
						opaque:  it.Opaque,
					}, pub: it.Pub}
				}
			}
		}
	}
	// Import dispositions.
	for _, it := range f.Items {
		if imp, ok := it.(*ast.Import); ok {
			c.checkImport(imp)
		}
	}
	// The main convention binds the root module alone.
	if c.mode == Project && c.isRoot && !c.noMain {
		c.checkMain(f)
	}
	// Pass 2a: resolve every declaration's types under its own clause —
	// variant payloads, record fields, newtype underlyings, fn signatures,
	// interface signatures (E0826) — then check each derives clause and
	// register its generated methods (E0823's declaration side, design D8).
	for _, it := range f.Items {
		switch x := it.(type) {
		case *ast.SumDecl:
			sum := c.syms[x.Name].sum
			c.pushTypes(paramScope(sum.params))
			for i, v := range x.Variants {
				var payloads []Type
				var pos [][2]int
				for _, tr := range v.Payload {
					pt := c.resolveTypeRef(tr, slotAnn)
					// A value sum's payloads face E0702's category honesty
					// (an M3 gap closed with the composite chapter); a
					// parameter position passes here (its category is the
					// application's) and re-checks at each instantiation.
					if sum.byval && catOf(pt) != "value" {
						line, col := refPos(tr)
						c.fail(line, col, "E0702", fmt.Sprintf(
							"value sum payload is not of the value category or a base type — the payload of %q is %q, a %s; a copy is only honest when everything in it is copyable by value",
							v.Name, pt.String(), catNoun(pt)))
					}
					// A gc sum's payloads face chapter 13's composite ban —
					// a payload slot would share the one handle (E1106, at
					// the payload's own reference).
					if !sum.byval && catOf(pt) == "resource" {
						line, col := refPos(tr)
						c.fail(line, col, "E1106", fmt.Sprintf(
							"resource type in a composite position — %q appears as a sum payload; a resource type appears only in binding positions - a parameter, a let or scope-head binding, a return type",
							pt.String()))
					}
					payloads = append(payloads, pt)
					l, co := refPos(tr)
					pos = append(pos, [2]int{l, co})
				}
				sum.variants[i].payloads = payloads
				sum.variants[i].payloadPos = pos
			}
			c.checkSumDerives(x, sum)
			c.popTypes()
		case *ast.RecordDecl:
			rec := c.syms[x.Name].rec
			c.pushTypes(paramScope(rec.params))
			for _, f := range x.Fields {
				ft := c.resolveTypeRef(f.Typ, slotAnn)
				fline, fcol := refPos(f.Typ)
				// E0601's category honesty anchors at the field's type
				// reference — the fact the copy would betray; a parameter
				// position passes here and re-checks at each instantiation.
				if rec.cat == "value" && catOf(ft) != "value" {
					c.fail(fline, fcol, "E0601", fmt.Sprintf(
						"value record field is not of the value category or a base type — the field %q is %q, a %s; a copy is only honest when everything in it is copyable by value",
						f.Name, ft.String(), catNoun(ft)))
				}
				// A gc record's fields face chapter 13's composite ban — a
				// field slot would share the one handle (E1106, at the
				// field's own reference; the byres category owns nested
				// resources and stays outside the ban, the byval one is
				// E0601's own rejection above).
				if rec.cat == "gc" && catOf(ft) == "resource" {
					c.fail(fline, fcol, "E1106", fmt.Sprintf(
						"resource type in a composite position — %q appears as a gc record field; a resource type appears only in binding positions - a parameter, a let or scope-head binding, a return type",
						ft.String()))
				}
				rec.fields = append(rec.fields, fieldInfo{name: f.Name, typ: ft, line: fline, col: fcol})
			}
			c.checkRecordDerives(x, rec)
			c.popTypes()
		case *ast.NewtypeDecl:
			nt := c.syms[x.Name].nt
			c.pushTypes(paramScope(nt.params))
			nt.underlying = c.resolveTypeRef(x.Underlying, slotAnn)
			nt.underLine, nt.underCol = refPos(x.Underlying)
			// A newtype wraps a value; the underlying slot faces chapter
			// 13's composite ban — a wrapped resource shares the one handle
			// (E1106, at the underlying reference).
			if catOf(nt.underlying) == "resource" {
				c.fail(nt.underLine, nt.underCol, "E1106", fmt.Sprintf(
					"resource type in a composite position — %q appears as a newtype's underlying type; a resource type appears only in binding positions - a parameter, a let or scope-head binding, a return type",
					nt.underlying.String()))
			}
			c.checkNewtypeDerives(x, nt)
			c.popTypes()
		case *ast.FnDecl:
			c.pushTypes(paramScope(typeParamNames(x.TypeParams)))
			var params []Type
			for _, p := range x.Params {
				params = append(params, c.resolveTypeRef(p.Type, slotAnn))
			}
			c.fnParams[x] = params
			c.fnTags[x] = c.resolveEffectTags(x.EffectTags, x.EffectLine, x.EffectCol)
			if x.Ret != nil {
				c.fnRets[x] = c.resolveTypeRef(x.Ret, slotRet)
			}
			c.fnWheres[x] = c.checkWhereDecl(typeParamNames(x.TypeParams), x.Where)
			c.popTypes()
		case *ast.InterfaceDecl:
			c.checkInterface(x)
		case *ast.TopLet:
			if x.Binding.Typ != nil && x.Binding.Name != "_" {
				c.syms[x.Binding.Name].letType = c.resolveTypeRef(x.Binding.Typ, slotAnn)
			}
		case *ast.ForeignBlock:
			for _, e := range x.Items {
				switch it := e.(type) {
				case *ast.FnDecl:
					// The signature resolves exactly as a declaration's own
					// (the entry feeds the same fnParams/fnTags/fnRets maps
					// the call machinery reads — the boundary is a
					// declaration site, not a new call form). The slots pass
					// slotRet, not slotAnn: a foreign signature's Never
					// judgment is the crossing set's own (E1705's tail for
					// a parameter, legal as the declared return), and the
					// annotation-position E0703 must not pre-empt it.
					var params []Type
					for _, p := range it.Params {
						params = append(params, c.resolveTypeRef(p.Type, slotRet))
					}
					c.fnParams[it] = params
					// The segment is mandatory at parse (E1703); the bare
					// keyword form resolves as the empty set — the explicit
					// pure claim — through the same resolver as any other.
					c.fnTags[it] = c.resolveEffectTags(it.EffectTags, it.EffectLine, it.EffectCol)
					if it.Ret != nil {
						c.fnRets[it] = c.resolveTypeRef(it.Ret, slotRet)
					}
					c.checkCrossingSet(it, params)
				case *ast.RecordDecl:
					// An opaque entry rides the record machinery whole: the
					// field loop runs zero times (the parser holds the
					// zero-field rule, E0105), and a derives clause, if
					// written, checks against no fields through the
					// ordinary walk (design D2 — chapter 19 adds no rule of
					// its own here).
					rec := c.syms[it.Name].rec
					c.pushTypes(paramScope(rec.params))
					c.checkRecordDerives(it, rec)
					c.popTypes()
				}
			}
		}
	}
	// Pass 2a2: the impls in source order, after every declaration's facts
	// resolved — heads, interfaces, and member sets read complete (design
	// D3's judgment order).
	for _, x := range implDecls {
		c.checkImplDecl(x)
	}
	// Pass 2b: top-level binding initializers, then the bodies in source
	// order — a module binding references only earlier ones, and every body
	// (a fn, an interface default, an impl method) reads the registered
	// member sets.
	for _, it := range f.Items {
		if x, ok := it.(*ast.TopLet); ok {
			// The initializer walks under its binding's name (chapter 16):
			// outside any declaration body, its first effectful call is
			// E1405's own face.
			savedTopLet := c.topLetName
			c.topLetName = x.Binding.Name
			t := c.checkBinding(&x.Binding)
			c.topLetName = savedTopLet
			if catOf(t) == "resource" {
				// The module top level binds no resource (chapter 13): no
				// channel exists outside function bodies, so no path could
				// transfer it (E1104, at the let itself; the var spelling,
				// if the grammar ever grows it here, is E1105's shape).
				if x.Binding.Kw == "var" {
					c.fail(x.Line, x.Col, "E1105", fmt.Sprintf(
						"resource binding rebound — %q is declared var and rebindable; a resource binding is let-shaped, and the one sanctioned move is a transfer",
						x.Binding.Name))
				}
				c.fail(x.Line, x.Col, "E1104", fmt.Sprintf(
					"resource binding outside its release discipline — %q is bound at the module top level, where no channel exists outside function bodies; move the binding into a function body and transfer it there",
					x.Binding.Name))
			}
			if x.Binding.Name != "_" {
				c.syms[x.Binding.Name].letType = t
			}
		}
	}
	for _, it := range f.Items {
		switch x := it.(type) {
		case *ast.FnDecl:
			c.checkFnDecl(x)
		case *ast.InterfaceDecl:
			c.checkIfaceBodies(x)
		case *ast.ImplDecl:
			c.checkImplBodies(x)
		case *ast.TestDecl:
			c.checkTestDecl(x)
		}
	}
	// Chapter 13's module-level completeness, at the true module tail:
	// every byres record owns its release — an impl later in the file
	// than its record satisfies the obligation (every impl has registered
	// by now), and only an impl in the record's own module counts. The
	// judgment runs after the bodies so a use-site diagnostic (E0606's
	// update, E1002's capture) reports ahead of the declaration's missing
	// impl (E1101, at the record's name token). A byres opaque entry of a
	// foreign block joins the same walk: chapter 19 hands the resource
	// discipline to chapter 13's machine whole, no rule of its own
	// (design D2).
	var resRecords []*ast.RecordDecl
	for _, it := range f.Items {
		switch x := it.(type) {
		case *ast.RecordDecl:
			if x.Cat == "resource" {
				resRecords = append(resRecords, x)
			}
		case *ast.ForeignBlock:
			for _, e := range x.Items {
				if rd, ok := e.(*ast.RecordDecl); ok && rd.Cat == "resource" {
					resRecords = append(resRecords, rd)
				}
			}
		}
	}
	for _, rd := range resRecords {
		rec := c.syms[rd.Name].rec
		released := false
		for _, im := range c.impls {
			if im.iface != releasableIface {
				continue
			}
			if rt, ok := im.head.(recordType); ok && rt.decl == rec {
				released = true
				break
			}
		}
		if !released {
			c.fail(rd.NameLine, rd.NameCol, "E1101", fmt.Sprintf(
				"resource record must implement Releasable — %q declares no impl of Releasable in its own module; add impl Releasable for %s { fn release(mut self) { ... } } in the record's module, or declare a gc or value record if it owns no resource",
				rd.Name, rd.Name))
		}
	}
}

// checkCrossingSet judges one foreign fn entry's signature against chapter
// 19's closed crossing set (design D2): the base scalars — String and Bytes
// among them, the ABI's pointer-plus-length pair — cross as parameters
// alone (E1706's own face at the return), an opaque record bare of
// arguments crosses as the one-pointer handle, and Never crosses as the
// declared return alone (the one trust point). Everything else — records
// with fields, sums, tuples, the generic containers, closures and fn
// types, a parameterized opaque — does not cross: values cross one at a
// time, or they do not cross at all. params are the entry's resolved
// parameter types (pass 2a fills them just before this runs).
func (c *checker) checkCrossingSet(x *ast.FnDecl, params []Type) {
	for i, p := range x.Params {
		t := params[i]
		if crosses(t) {
			continue
		}
		line, col := refPos(p.Type)
		if _, never := t.(neverType); never {
			c.fail(line, col, "E1705", fmt.Sprintf(
				"foreign function signature holds a type outside the crossing set — parameter %q holds %s, which does not cross; Never is a declared return position only",
				p.Name, t.String()))
		}
		c.fail(line, col, "E1705", fmt.Sprintf(
			"foreign function signature holds a type outside the crossing set — parameter %q holds %s, which does not cross; values cross one at a time, or they do not cross at all",
			p.Name, t.String()))
	}
	if x.Ret == nil {
		return
	}
	ret := c.fnRets[x]
	if _, never := ret.(neverType); never {
		return // the declared return is Never's one legal crossing
	}
	if b, base := ret.(baseType); base && (string(b) == "String" || string(b) == "Bytes") {
		line, col := refPos(x.Ret)
		c.fail(line, col, "E1706", fmt.Sprintf(
			"String or Bytes is not a foreign return type — %q declares a %s return; return an opaque handle plus a length query plus a copy into a caller-provided buffer",
			x.Name, ret.String()))
	}
	if !crosses(ret) {
		line, col := refPos(x.Ret)
		c.fail(line, col, "E1705", fmt.Sprintf(
			"foreign function signature holds a type outside the crossing set — the return type holds %s, which does not cross; values cross one at a time, or they do not cross at all",
			ret.String()))
	}
}

// crosses reports whether t is in chapter 19's crossing set for a
// parameter position: a base scalar (the baseTypes' own closed set — the
// eight integers, both floats, Bool, Rune, String, and Bytes), or an
// opaque record bare of generic arguments (the handle; an instantiated
// opaque is a different type each way and does not cross). Never's
// position rule (declared returns only) belongs to the callers, not this
// predicate.
func crosses(t Type) bool {
	if b, ok := t.(baseType); ok {
		return baseNames[string(b)]
	}
	if rt, ok := t.(recordType); ok {
		return rt.decl.opaque && len(rt.args) == 0
	}
	return false
}

// checkImport applies the import dispositions: std paths answer from the
// compiler-provided registry (project mode's loader already placed every
// provided module in the graph; single-file mode pre-ingested them at
// Check); in project mode the loader walked the graph — E1301 and E1302
// are its faces, and every import's target sits in the module buckets
// already; single-file mode has no source root for file-system paths.
func (c *checker) checkImport(imp *ast.Import) {
	if imp.Path[0] == "std" {
		key := strings.Join(imp.Path, ".")
		if _, ok := StdModule(key); !ok {
			c.fail(imp.PathLine, imp.PathCol, "E1302", StdModuleNotFound(key))
		}
		return
	}
	if c.mode == SingleFile {
		rel := filepath.Join(imp.Path...) + ".we"
		c.fail(imp.PathLine, imp.PathCol, "E1302", fmt.Sprintf(
			"module not found — %q cannot resolve in single-file mode: no source root exists (a project would expect it at src/%s); run we check against the project directory",
			strings.Join(imp.Path, "."), rel))
	}
}

// --- chapter 15 R1: the compiler-provided std segment -----------------------

// StdModule is the registry of compiler-provided modules. The std segment
// never resolves against the file system, so the project loader and the
// single-file pre-ingest both ask here — one authority for which std
// modules a build provides. Each module is synthetic We source: real
// declarations the same checker machinery a user module rides walks (the
// provision is compiler-built; the checking is not privileged). M8
// provides std.io (design D2); the self-hosting route — real sources over
// a foreign layer — is design D9's disclosed follow-up.
func StdModule(key string) (*ast.File, bool) {
	switch key {
	case "std.io":
		return &ast.File{Items: []ast.Item{
			&ast.FnDecl{
				Pub:        true,
				Name:       "println",
				Params:     []ast.Param{{Name: "s", Type: &ast.NamedType{Name: "String"}}},
				EffectTags: []string{"io"},
			},
			&ast.FnDecl{
				Pub:        true,
				Name:       "print",
				Params:     []ast.Param{{Name: "s", Type: &ast.NamedType{Name: "String"}}},
				EffectTags: []string{"io"},
			},
		}}, true
	case "std.concurrent":
		return stdConcurrentFile, true
	case "std.test":
		// M10a design D8: the two Bool faces are plain declarations — the
		// ordinary import surface types their calls (the std.io println
		// precedent); assertEqual is not an item but a call face, riding
		// importCall's dispatch ahead of the gate, so it has no marker
		// declaration here — a mock on it resolves nowhere (E1304), the
		// honest reading of a face that is not a module-level fn.
		return &ast.File{Items: []ast.Item{
			&ast.FnDecl{Pub: true, Name: "assertTrue", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
				Params: []ast.Param{{Name: "cond", Type: &ast.NamedType{Name: "Bool", Line: 1, Col: 1}, NameLine: 1, NameCol: 1}}},
			&ast.FnDecl{Pub: true, Name: "assertFalse", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
				Params: []ast.Param{{Name: "cond", Type: &ast.NamedType{Name: "Bool", Line: 1, Col: 1}, NameLine: 1, NameCol: 1}}},
		}}, true
	case "std.time":
		// M10b design D4: the fourth synthetic module — now and sleep as
		// plain declarations with the time effect segment (the std.io
		// precedent again; the loading gate keys on the module key, so a
		// user module named time never disturbs the entry). The bodies are
		// the We-writable fiction the M8 discipline wants — a tail-produced
		// Int64 for now, an empty body for sleep — the same checker
		// machinery walks them, and the run face rides the virtual clock
		// test.c owns inside the test extent and the wall clock outside it.
		return &ast.File{Items: []ast.Item{
			&ast.FnDecl{Pub: true, Name: "now", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
				EffectTags: []string{"time"},
				Ret:        &ast.NamedType{Name: "Int64", Line: 1, Col: 1},
				Body: ast.Block{Items: []ast.Stmt{&ast.ExprStmt{
					Expr: &ast.Literal{Kind: "int", Text: "0"}, Line: 1, Col: 1,
				}}, Line: 1, Col: 1}},
			&ast.FnDecl{Pub: true, Name: "sleep", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
				Params:     []ast.Param{{Name: "ms", Type: &ast.NamedType{Name: "Int64", Line: 1, Col: 1}, NameLine: 1, NameCol: 1}},
				EffectTags: []string{"time"}},
		}}, true
	}
	return nil, false
}

// stdConcurrentFile is the std.concurrent module's synthetic source
// (design D2): the four named sums are real declarations — We-writable
// shapes the same ingest machinery a user module rides (the std.io
// precedent) — and channel's marker FnDecl exists for the qualified
// name's reach; its typing is the expectation-driven construction path,
// never the fn-call one. The declarations sit at the file's 1:1 (the
// synthetic prelude's convention — no diagnostic anchors inside). A
// package-level singleton: one file object per run, so the bucket's
// sumInfo pointers stay identical however many faces read them back.
// The variant name Closed appears in two sums — chapter 18's own
// enumeration spells it in both SendResult and ReceiveResult; the
// synthetic module never parses (the parser owns E0404's one-name-space
// rule), and pattern resolution is scrutinee-side, so the shared name
// gates the pub reach alone.
var stdConcurrentFile = &ast.File{Items: []ast.Item{
	&ast.SumDecl{Pub: true, Name: "TimeoutError", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
		Variants: []ast.Variant{{Name: "TimedOut", Line: 1, Col: 1}}},
	&ast.SumDecl{Pub: true, Name: "TaskPanic", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
		Variants: []ast.Variant{{Name: "Panicked", Line: 1, Col: 1,
			Payload: []ast.TypeRef{&ast.NamedType{Name: "String", Line: 1, Col: 1}}}}},
	&ast.SumDecl{Pub: true, Name: "SendResult", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
		Variants: []ast.Variant{
			{Name: "Sent", Line: 1, Col: 1}, {Name: "Full", Line: 1, Col: 1}, {Name: "Closed", Line: 1, Col: 1}}},
	&ast.SumDecl{Pub: true, Name: "ReceiveResult", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
		TypeParams: []*ast.TypeParam{{Name: "T", Line: 1, Col: 1}},
		Variants: []ast.Variant{
			{Name: "Received", Line: 1, Col: 1,
				Payload: []ast.TypeRef{&ast.NamedType{Name: "T", Line: 1, Col: 1}}},
			{Name: "Empty", Line: 1, Col: 1}, {Name: "Closed", Line: 1, Col: 1}}},
	&ast.FnDecl{Pub: true, Name: "channel", Line: 1, Col: 1, NameLine: 1, NameCol: 1,
		Params: []ast.Param{{Name: "n", Type: &ast.NamedType{Name: "Int64", Line: 1, Col: 1}, NameLine: 1, NameCol: 1}}},
}}

// StdModuleNotFound renders E1302's std form — the one text the loader and
// the import pass report for a std path no build provides.
func StdModuleNotFound(key string) string {
	return fmt.Sprintf(
		"module not found — no standard-library module %q exists in this build; the std segment is compiler-provided, so the name is misspelled or the module is not implemented yet",
		key)
}

// stdImports lists a file's std import keys in source order, deduplicated —
// the pre-ingest order of single-file mode.
func stdImports(f *ast.File) []string {
	var keys []string
	seen := map[string]bool{}
	for _, it := range f.Items {
		imp, ok := it.(*ast.Import)
		if !ok || imp.Path[0] != "std" {
			continue
		}
		key := strings.Join(imp.Path, ".")
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

// --- chapter 15: cross-module resolution -----------------------------------

// importSym resolves one qualified item through an import's target module
// (chapter 15): the item must be declared in the target and pub — a
// declared non-pub item is E1303's shape, an undeclared one E1304's. The
// anchor is the qualified name's first token, whatever position holds it.
func (c *checker) importSym(mod, item string, line, col int) *symbol {
	bucket := c.modules[mod]
	sym, ok := bucket[item]
	if !ok {
		c.fail(line, col, "E1304", fmt.Sprintf(
			"unresolved name — the module %q declares no %q; qualify through a declared pub item of the import",
			mod, item))
	}
	if !sym.pub {
		c.fail(line, col, "E1303", fmt.Sprintf(
			"cross-module use of a module-local item — %q is declared in %q without pub; cross-module reach is exactly the qualified form name.item through an import, and only for pub items",
			item, mod))
	}
	return sym
}

// importTypeRef resolves a qualified type reference (b.Point) through the
// import's bucket: the pub gate first, then the ordinary named-resolution
// kinds — the target's declaration nodes drive the same machinery a local
// reference uses (the AST-keyed maps every module's pass filled).
func (c *checker) importTypeRef(mod string, x *ast.NamedType, slot slotKind) Type {
	// The chapter 18 shared-state types ride the qualifier without a
	// module declaration (checker-side singletons, design D2) — the gate
	// is the module key itself, so a user module's same-named Mutex never
	// reaches this branch.
	if mod == "std.concurrent" {
		if sum, ok := concurrentTypes[x.Name]; ok {
			c.checkArity(x, sum.name, sum.params)
			return namedType{decl: sum, args: c.resolveArgs(x.Args, slotGeneric)}
		}
	}
	sym := c.importSym(mod, x.Name, x.Line, x.Col)
	switch sym.kind {
	case symType:
		c.checkArity(x, sym.sum.name, sym.sum.params)
		return namedType{decl: sym.sum, args: c.resolveArgs(x.Args, slotGeneric)}
	case symRecord:
		c.checkArity(x, sym.rec.name, sym.rec.params)
		return recordType{decl: sym.rec, args: c.resolveArgs(x.Args, slotGeneric)}
	case symNewtype:
		c.checkArity(x, sym.nt.name, sym.nt.params)
		return newtypeType{decl: sym.nt, args: c.resolveArgs(x.Args, slotGeneric)}
	case symIface:
		c.checkArity(x, sym.iface.name, sym.iface.params)
		args := c.resolveArgs(x.Args, slotGeneric)
		c.valueSlotIface(x, args)
		return ifaceType{decl: sym.iface, args: args}
	default:
		// a value name held by the target module fills no type slot
		c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
	}
	panic("unreachable import type")
}

// importIface resolves a qualified interface reference (a where bound, a
// Dyn argument, an impl clause) through the import's bucket: the pub gate
// first, then the interface kind — a non-interface lands in the caller's
// own voice (each position names one in its own code).
func (c *checker) importIface(mod string, nt *ast.NamedType, code, notIface string) ifaceType {
	sym := c.importSym(mod, nt.Name, nt.Line, nt.Col)
	if sym.kind != symIface {
		c.fail(nt.Line, nt.Col, code, notIface)
	}
	c.checkArity(nt, sym.iface.name, sym.iface.params)
	return ifaceType{decl: sym.iface, args: c.resolveArgs(nt.Args, slotGeneric)}
}

// importMember types one qualified value reference (b.helper) outside a
// call head: the pub gate first (anchored at the receiver's first token,
// the qualified name's start), then the value kinds. A generic fn's
// qualified name is a family like its bare name is (chapter 12) — only
// the call head determines it.
func (c *checker) importMember(mod string, id *ast.Ident, x *ast.Member) Type {
	sym := c.importSym(mod, x.Name, id.Line, id.Col)
	switch sym.kind {
	case symFn:
		if len(sym.fn.TypeParams) > 0 {
			c.fail(x.NameLine, x.NameCol, "E1004", fmt.Sprintf(
				"generic function name in value position — %q is a family of functions, not one function; call it or wrap it: |x| %s(x)",
				x.Name, x.Name))
		}
		ret := Type(unitType{})
		if r, has := c.fnRets[sym.fn]; has {
			ret = r
		}
		return fnType{params: c.fnParams[sym.fn], tags: c.fnTags[sym.fn], ret: ret}
	case symLet:
		return sym.letType
	case symVariant:
		return c.variantType(sym.sum, sym.vi, x.NameLine, x.NameCol, x.Name)
	}
	// type names carry no value
	c.fail(x.NameLine, x.NameCol, "E1304", bareUnresolved(x.Name))
	panic("unreachable import member")
}

// importCall types one qualified call head (b.helper(…), b.NotFound(…)):
// the pub gate first, then the symbol-kind dispatch a bare call head
// takes — a generic fn's determination and a variant's payload check ride
// the same machinery. The std.concurrent constructions dispatch ahead of
// the gate (checker-side singletons, design D2): the shared-state cells
// take their parameter from the argument, the channel from the expected
// type at its position.
func (c *checker) importCall(mod string, x *ast.Call, expected Type) Type {
	m := x.Fn.(*ast.Member)
	if mod == "std.concurrent" {
		if sum, ok := concurrentTypes[m.Name]; ok {
			return c.concurrentCtorCall(sum, x, expected)
		}
		if m.Name == "channel" {
			return c.channelCall(x, expected)
		}
	}
	// The std.test equality assertion rides its own face ahead of the
	// gate (M10a design D8): a same-type-pair judgment, not a declared
	// fn. The gate is the module key itself — a user module's
	// same-named items never reach this branch.
	if mod == "std.test" && m.Name == "assertEqual" {
		return c.assertEqualCall(x)
	}
	sym := c.importSym(mod, m.Name, m.Recv.(*ast.Ident).Line, m.Recv.(*ast.Ident).Col)
	switch sym.kind {
	case symFn:
		return c.fnCall(sym.fn, x, mod)
	case symVariant:
		return c.ctorCall(sym.sum, sym.vi, x)
	case symNewtype:
		return c.newtypeCall(sym.nt, x)
	}
	c.bnd(bndCalleeGap)
	panic("unreachable import call")
}

// --- chapter 18: the shared-state constructions (design D2) --------------------

// concurrentCtorCall types one shared-state cell construction
// (conc.Mutex(v), conc.Semaphore(n), …): the one argument's type is the
// cell's parameter — no determination machinery, no explicit type-argument
// form. The judgments the compiler can make at the site it makes
// (E1614/E1615 for the atomic cells' argument shapes, E1616 for the
// semaphore's constant count); everything else is the argument's own
// position-wise check.
func (c *checker) concurrentCtorCall(sum *sumInfo, x *ast.Call, expected Type) Type {
	id := x.Fn.(*ast.Member).Recv.(*ast.Ident)
	if len(x.Args) != 1 {
		c.bnd(bndArityGap)
	}
	switch sum {
	case mutexSum, rwlockSum, atomicSum, atomicRefSum:
		t := c.typeOf(x.Args[0], nil)
		switch sum {
		case atomicSum:
			if !atomicBaseType(t) {
				c.fail(id.Line, id.Col, "E1614", fmt.Sprintf(
					"Atomic type argument is not an atomic base type — the argument has type %q, not one of the eight integer types, Bool, Float32, Float64; composite values have no single-word read-modify-write to be atomic over, so wrap the composite in Mutex or RwLock, or hold the base-type part in the Atomic",
					t.String()))
			}
		case atomicRefSum:
			rt, isRec := t.(recordType)
			if !isRec || rt.decl.cat != "gc" {
				c.fail(id.Line, id.Col, "E1615", fmt.Sprintf(
					"AtomicRef type argument is not a gc record type — the argument has type %q; base types belong in Atomic, and non-record shapes have no whole-record pointer swap, so use Atomic for base types and Mutex or RwLock for other composites",
					t.String()))
			}
		}
		return namedType{decl: sum, args: []Type{t}}
	case condSum:
		// A cond binds the mutex it waits on: the one argument is a
		// Mutex<X>, and the cond's own parameter takes X.
		at := c.typeOf(x.Args[0], nil)
		mt, ok := at.(namedType)
		if !ok || mt.decl != mutexSum {
			line, col := exprPos(x.Args[0])
			c.fail(line, col, "E0501", fmt.Sprintf(
				"mixed types — the argument is %s, the parameter is Mutex<T>; no coercion is ever inserted",
				at.String()))
		}
		return namedType{decl: condSum, args: mt.args}
	case semaphoreSum:
		c.semaphoreCount(x.Args[0])
		c.checkArgs(x.Args, []Type{baseType("Int64")})
		return namedType{decl: semaphoreSum}
	}
	c.bnd(bndCalleeGap)
	panic("unreachable concurrent construction")
}

// assertEqualCall types one std.test equality assertion (st.assertEqual(got,
// want), M10a design D8): the first argument fixes T, the second must be
// the same type — sameType, chapter 20's word for the pair — and T's
// domain is the scalar one: the eight integer types, Bool, String. Beyond
// that domain the face is an honest boundary, not a judgment: chapter
// 10's Eq rides derives alone (base-type impl heads are E0811, manual
// impls E0822, composite equality is the generated .equals()), so an
// Eq-bound generic here would need a builtin-instances face the spec
// does not fix — and would widen every `fn f<T where T: Eq>` in the same
// stroke. The widening is the standard library's own change to make.
// Unit return, no effect segment: an assertion observes, it never
// performs.
func (c *checker) assertEqualCall(x *ast.Call) Type {
	if len(x.Args) != 2 {
		c.bnd(bndArityGap)
	}
	got := c.typeOf(x.Args[0], nil)
	if b, ok := got.(baseType); !ok || !assertEqScalar[b] {
		c.bnd(bndAssertEqDomain)
	}
	want := c.typeOf(x.Args[1], got)
	if !sameType(want, got) {
		line, col := exprPos(x.Args[1])
		c.fail(line, col, "E0501", fmt.Sprintf(
			"mixed types — the argument is %s, the parameter is %s; no coercion is ever inserted",
			want.String(), got.String()))
	}
	return unitType{}
}

// assertEqScalar is assertEqual's domain (M10a design D8): the eight
// integer types, Bool, and String — the comparanda whose equality the
// language fixes without an Eq instance.
var assertEqScalar = map[baseType]bool{
	"Int64": true, "Int32": true, "Int16": true, "Int8": true,
	"UInt64": true, "UInt32": true, "UInt16": true, "UInt8": true,
	"Bool": true, "String": true,
}

// channelCall types one channel construction (conc.channel(n), design
// D2): the element type has exactly one source — the expected type at
// the construction's position, a Channel<T> fixing T. Nothing runs from
// the capacity argument, and no explicit type-argument form exists; a
// position that fixes nothing is E1617 (the construction names the
// binding it initializes when one names it).
func (c *checker) channelCall(x *ast.Call, expected Type) Type {
	id := x.Fn.(*ast.Member).Recv.(*ast.Ident)
	if len(x.Args) != 1 {
		c.bnd(bndArityGap)
	}
	c.checkArgs(x.Args, []Type{baseType("Int64")})
	if nt, ok := expected.(namedType); ok && nt.decl == channelSum {
		return namedType{decl: channelSum, args: nt.args}
	}
	context := "no declaration fixes the element type"
	if c.letName != "" {
		context = fmt.Sprintf("the binding %q carries no annotation and no declaration fixes the element type", c.letName)
	}
	c.fail(id.Line, id.Col, "E1617", fmt.Sprintf(
		"channel construction without an expected type — %s, and the language infers nothing; annotate the binding, or place the construction where a parameter or declared return fixes the type",
		context))
	panic("unreachable channel construction")
}

// atomicBaseType reports one of the types an Atomic may hold (chapter
// 18): the eight integer types, Bool, Float32, Float64 — the shapes a
// single-word read-modify-write covers.
func atomicBaseType(t Type) bool {
	b, ok := t.(baseType)
	return ok && (intNames[string(b)] || b == "Bool" || b == "Float32" || b == "Float64")
}

// semaphoreCount judges one Semaphore construction's count (chapter 18):
// a count the compiler can decide — one integer literal, or a unary
// minus directly before one — it decides at the site; a non-positive
// constant is E1616, anchored at the count's own token. Any other count
// expression leaves its judgment to the runtime's checked panic.
func (c *checker) semaphoreCount(e ast.Expr) {
	v, ok := constCount(e)
	if !ok || v.Sign() > 0 {
		return
	}
	line, col := exprPos(e)
	c.fail(line, col, "E1616", fmt.Sprintf(
		"Semaphore count is not a positive integer — the count constant-evaluates to %s, and what the compiler can decide, it decides, at the construction site; construct with a positive count - a non-positive count at runtime is a checked panic of chapter 14's family",
		v.String()))
}

// constCount reads a count expression's constant value: one integer
// literal, or a unary minus directly before one (the minus renders in
// the value).
func constCount(e ast.Expr) (*big.Int, bool) {
	if lit, ok := e.(*ast.Literal); ok && lit.Kind == "int" {
		_, v := intLiteral(lit.Text)
		return v, true
	}
	if u, ok := e.(*ast.Unary); ok && u.Op == "-" {
		if lit, ok := u.X.(*ast.Literal); ok && lit.Kind == "int" {
			_, v := intLiteral(lit.Text)
			return new(big.Int).Neg(v), true
		}
	}
	return nil, false
}

// importQualifier reports the canonical module key when name is an import
// of the live module and no local binding shadows it — the one shape a
// qualified reference resolves cross-module through.
func (c *checker) importQualifier(name string) (string, bool) {
	if _, shadowed := c.lookupLocal(name); shadowed {
		return "", false
	}
	sym, ok := c.syms[name]
	if !ok || sym.kind != symImport {
		return "", false
	}
	return sym.mod, true
}

// --- chapter 16: effect tag resolution ---------------------------------------

// builtInTag names one of the language-level tags (io, net, time): they
// name their effect with no declaration anywhere — a bare use in a segment
// resolves on sight, and no module item may take their names (E1403 is the
// parser's face at the declaration).
func builtInTag(name string) bool {
	return name == "io" || name == "net" || name == "time"
}

// displayTag renders a canonical effect key for a message or a type
// rendering: a built-in key is bare already, and a custom key shows its
// effect's own name (the part after the module). Every message that names
// a tag and the fn type rendering both go through here, so one effect has
// one display everywhere.
func displayTag(key string) string {
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[i+1:]
	}
	return key
}

// resolveEffectTag resolves one segment tag — the bare spelling or the
// qualified module.name — to its canonical key (design D3): a built-in is
// its own bare key; a custom effect's key is <module key>.<name>, so the
// bare spelling inside the declaring module and the qualified spelling
// through an import meet at one key. A built-in needs no declaration; a
// bare custom tag reads the live module's effect declarations (E1304); a
// qualified tag reads the import's target through chapter 15's pub gate
// (the qualifier that is no import is E1304's own shape, the non-pub
// target E1303's). The anchor is the segment's first tag.
func (c *checker) resolveEffectTag(tag string, line, col int) string {
	if builtInTag(tag) {
		return tag
	}
	if qual, name, ok := strings.Cut(tag, "."); ok {
		mod, is := c.importQualifier(qual)
		if !is {
			c.fail(line, col, "E1304", fmt.Sprintf(
				"unresolved name — no import introduces %q, so the effect tag %q resolves nowhere; import the module or declare the effect locally",
				qual, tag))
		}
		sym := c.importSym(mod, name, line, col)
		if sym.kind != symEffect {
			c.fail(line, col, "E1304", fmt.Sprintf(
				"unresolved name — the module %q declares no %q; qualify through a declared pub item of the import",
				mod, name))
		}
		return mod + "." + name
	}
	if sym, ok := c.syms[tag]; ok && sym.kind == symEffect {
		return c.modKey + "." + tag
	}
	c.fail(line, col, "E1304", fmt.Sprintf(
		"unresolved name — no effect named %q is declared in this module; declare it, fix the spelling, or import the module that declares it",
		tag))
	panic("unreachable effect tag")
}

// resolveEffectTags resolves one segment's tags in source order and keeps
// each distinct key's first occurrence (design D3) — the set a declaration
// or a fn type carries from here on.
func (c *checker) resolveEffectTags(tags []string, line, col int) []string {
	if len(tags) == 0 {
		return nil
	}
	var keys []string
	seen := map[string]bool{}
	for _, t := range tags {
		k := c.resolveEffectTag(t, line, col)
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

// checkCallEffect is chapter 16's call judgment (design D4): the callee's
// set must be a subset of the enclosing declaration's. Inside a closure
// body the judgment is suspended — the callee's set joins the closure's
// own inferred set instead (design D6); a defer body keeps the enclosing
// context (chapter 3 shifts timing, not ownership). Outside any body walk
// the caller is a top-level initializer — chapter 16's one ratified
// evaluation position outside a signature, and it must stay pure (E1405,
// design D8; a closure literal in the initializer exempted itself above:
// constructing it is not performing it). The E1401 message names the
// difference set's display tags, the callee, and the enclosing
// declaration.
func (c *checker) checkCallEffect(calleeTags []string, callee string, line, col int) {
	if len(calleeTags) == 0 {
		return // the empty set (the panic family) fits every context
	}
	if c.inClosure {
		// The judgment is suspended — the callee's set joins the
		// innermost closure's own inferred set instead (design D6): a
		// nested closure's body is its own inference, never the outer's,
		// and a closure's construction contributes nothing to the
		// enclosing declaration.
		if len(c.closureTags) > 0 {
			top := &c.closureTags[len(c.closureTags)-1]
			for _, t := range calleeTags {
				if !tagIn(t, *top) {
					*top = append(*top, t)
				}
			}
		}
		return
	}
	if c.bodyTest {
		// The test-body sentinel (chapter 16:36): a test is driver code,
		// E1401 does not fire at calls inside one. E1402 still judges a
		// closure built there (its own agreement, never this judgment).
		return
	}
	if !c.inBody {
		ds := make([]string, len(calleeTags))
		for i, t := range calleeTags {
			ds[i] = displayTag(t)
		}
		c.fail(line, col, "E1405", fmt.Sprintf(
			"effectful call in a top-level initializer — the initializer of %q calls %q, which performs effect %s; move the work into main, or initialize from a pure computation over constants",
			c.topLetName, callee, quoteTags(ds)))
	}
	var missing []string
	for _, t := range calleeTags {
		if !tagIn(t, c.bodyTags) {
			missing = append(missing, t)
		}
	}
	if len(missing) == 0 {
		return
	}
	ds := make([]string, len(missing))
	for i, m := range missing {
		ds[i] = displayTag(m)
	}
	// A task extent addresses the task, not a named declaration: the body
	// answers the task's own declared segment (chapter 18, design D6).
	if c.bodyTask {
		c.fail(line, col, "E1401", fmt.Sprintf(
			"undeclared effect at a call — %q performs effect %s which the task block does not declare; add the missing tag to the task's effect segment, or call a pure function instead",
			callee, quoteTags(ds)))
		return
	}
	c.fail(line, col, "E1401", fmt.Sprintf(
		"undeclared effect at a call — %q performs effect %s which %q does not declare; add the missing tag to the enclosing declaration's effect segment, or call a pure function instead",
		callee, quoteTags(ds), c.bodyName))
}

// tagIn reports whether the canonical key is in the declared set.
func tagIn(key string, set []string) bool {
	for _, s := range set {
		if s == key {
			return true
		}
	}
	return false
}

// tagSetEq is the canonical set equality both ways (chapter 16 R5's exact
// agreement — membership is what matters; the segments resolve
// duplicate-free, so equal length plus one-way containment is equality).
func tagSetEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		if !tagIn(x, b) {
			return false
		}
	}
	return true
}

// quoteTags renders a difference set for an effect message: one tag
// quoted, several quoted and comma-joined.
func quoteTags(ds []string) string {
	if len(ds) == 1 {
		return fmt.Sprintf("%q", ds[0])
	}
	q := make([]string, len(ds))
	for i, d := range ds {
		q[i] = fmt.Sprintf("%q", d)
	}
	return strings.Join(q, ", ")
}

// calleeSite reports a fn-typed call head's display name and anchor for
// E1401's message: the head's own name (an identifier's or a member's),
// anchored at the head's first token — the callee name itself at a bare
// head, the receiver's first token at a member head (design D4; the same
// rule methodCall's own hook applies).
func calleeSite(e ast.Expr) (string, int, int) {
	name := ""
	switch h := e.(type) {
	case *ast.Ident:
		name = h.Name
	case *ast.Member:
		name = h.Name
	}
	line, col := exprPos(e)
	return name, line, col
}

// checkFnSlot is chapter 16's woven judgment at one type-agreement
// position holding a fn value (design D5), called where the site's plain
// agreement has already failed: with the structure agreeing (parameters,
// return, nested fn types — the nested comparison exact, no variance),
// the top-level segment compares as a subset — a value performing a
// subset of the slot's effects fits (a purer function serves an
// effect-expecting slot, the Q3 flip) and reports green; a value
// performing more is E1402's own face at the site's anchor, naming the
// extra tags. fit false with no report marks a structural mismatch — the
// caller's E0501 stands, message unchanged.
func (c *checker) checkFnSlot(value, slot Type, line, col int) bool {
	v, vok := value.(fnType)
	s, sok := slot.(fnType)
	if !vok || !sok {
		return false
	}
	vv, ss := v, s
	vv.tags, ss.tags = nil, nil
	if !agree(vv, ss) {
		return false
	}
	var extra []string
	for _, t := range v.tags {
		if !tagIn(t, s.tags) {
			extra = append(extra, t)
		}
	}
	if len(extra) == 0 {
		return true
	}
	ds := make([]string, len(extra))
	for i, t := range extra {
		ds[i] = displayTag(t)
	}
	c.fail(line, col, "E1402", fmt.Sprintf(
		"function value effect set does not match the expected type's — the value performs effect %s and the expected type's effect segment does not include it; widen the expected type's effect segment to cover the value's set, or supply a value that performs less",
		quoteTags(ds)))
	panic("unreachable fn slot")
}

// checkMain enforces the main convention (design D12's chain): the root
// module declares exactly one main, pub, parameterless, returning the
// builtin Result<(), E>; the E position's named-sum constraint is
// E1204's, not this code's.
func (c *checker) checkMain(f *ast.File) {
	var main *ast.FnDecl
	for _, it := range f.Items {
		if fd, ok := it.(*ast.FnDecl); ok && fd.Name == "main" {
			main = fd
			break
		}
	}
	tail := "; declare exactly one pub fn main() -> Result<(), E> in src/main.we with E a named sum type"
	if main == nil {
		c.fail(1, 1, "E1305", "main function signature violation — the root module declares no fn main"+tail)
	}
	if !main.Pub {
		c.fail(main.Line, main.Col, "E1305", "main function signature violation — main must be pub"+tail)
	}
	if len(main.Params) > 0 {
		c.fail(main.Line, main.Col, "E1305", "main function signature violation — main's signature is not pub fn main() -> Result<(), E>: it declares parameters"+tail)
	}
	nt, ok := main.Ret.(*ast.NamedType)
	_, shadowed := c.syms["Result"]
	shaped := ok && nt.Qual == "" && nt.Name == "Result" && !shadowed &&
		len(nt.Args) == 2 && isUnitRef(nt.Args[0])
	if !shaped {
		c.fail(main.Line, main.Col, "E1305", "main function signature violation — main's signature is not pub fn main() -> Result<(), E>: its return is not Result<(), E>"+tail)
	}
	// The E position: E1204's registry entry draws this line.
	e := c.resolveTypeRef(nt.Args[1], slotGeneric)
	if _, named := e.(namedType); !named {
		line, col := refPos(nt.Args[1])
		c.fail(line, col, "E1204", fmt.Sprintf(
			"Result error type is not a named sum type — the error parameter resolves to %s; declare a named sum carrying the failure variants and use it as the error parameter",
			e.String()))
	}
}

func isUnitRef(tr ast.TypeRef) bool {
	_, ok := tr.(*ast.UnitType)
	return ok
}

// --- chapter 10: interfaces and impls ----------------------------------------

// checkInterface validates one interface declaration (design D3): each
// method clause is fresh against the interface's own clause (E0826), and
// each signature resolves against the clause positions and the associated
// holes. Default bodies check in the bodies pass.
func (c *checker) checkInterface(x *ast.InterfaceDecl) {
	inf := c.syms[x.Name].iface
	c.pushTypes(ifaceScope(inf))
	defer c.popTypes()
	for i := range x.Methods {
		ms := &x.Methods[i]
		for _, tp := range ms.TypeParams {
			if paramIdx(inf.params, tp.Name) >= 0 {
				c.fail(tp.Line, tp.Col, "E0826", fmt.Sprintf(
					"generic parameter shadows an enclosing parameter — the method clause of %q redeclares %q; a clause name is fresh against the enclosing clause",
					ms.Name, tp.Name))
			}
		}
		c.pushTypes(paramScopeOffset(ms.TypeParams, len(inf.params)))
		im := ifaceMethod{
			name:       ms.Name,
			recvMut:    ms.Recv == ast.RecvMutSelf,
			typeParams: typeParamNames(ms.TypeParams),
			body:       ms.Body,
			line:       ms.NameLine,
			col:        ms.NameCol,
		}
		for _, p := range ms.Params {
			im.params = append(im.params, c.resolveTypeRef(p.Type, slotAnn))
		}
		// The segment sits between the parameters and the arrow in the
		// source, so it resolves between them here too.
		im.tags = c.resolveEffectTags(ms.EffectTags, ms.EffectLine, ms.EffectCol)
		if ms.HasRet {
			im.ret = c.resolveTypeRef(ms.Ret, slotRet)
		}
		c.popTypes()
		inf.methods = append(inf.methods, im)
	}
}

// checkIfaceBodies types an interface's default bodies: self is the
// interface's own view (the clause positions — the body types against the
// interface, not an impl), the return is the signature's, and the scope
// carries the clause, the associated holes, and the method's own clause.
func (c *checker) checkIfaceBodies(x *ast.InterfaceDecl) {
	inf := c.syms[x.Name].iface
	for i := range inf.methods {
		im := &inf.methods[i]
		if im.body == nil {
			continue
		}
		self := ifaceType{decl: inf, args: clauseArgs(inf.params)}
		c.pushTypes(ifaceScope(inf))
		c.pushTypes(paramScopeOffset(x.Methods[i].TypeParams, len(inf.params)))
		c.checkMethodBody(im.name, x.Methods[i].Params, im.params, im.body, im.ret, self, im.recvMut, im.tags)
		c.popTypes()
		c.popTypes()
	}
}

// The builtin combinator bodies' synthetic position and file: every node
// the registration builds carries line/column 1/1, and the walk runs
// under this file name — a failure there names the stdlib's own body,
// never a program's path.
const (
	stdBodyLine = 1
	stdBodyCol  = 1
	stdBodyFile = "(std combinators)"
)

// checkBuiltinCombinators walks the builtin interfaces' real default
// bodies through the same frame and method-body machinery
// checkIfaceBodies gives a user interface's defaults (design D3): the
// interface's clause scope, the method's own clause, self as the
// interface's own view — no privileged path. It runs at every check's
// entry over the checker's empty module scope, so the prelude faces
// alone answer (a program's shadowing never reaches the stdlib's
// bodies); a marker body (the compiler-intrinsic combinators) walks
// nothing, exactly as an empty user default does.
func (c *checker) checkBuiltinCombinators() {
	self := ifaceType{decl: iteratorIface, args: clauseArgs(iteratorIface.params)}
	for i := range iteratorIface.methods {
		im := &iteratorIface.methods[i]
		if im.body == nil || len(im.body.Items) == 0 {
			continue
		}
		params := make([]ast.Param, len(im.paramNames))
		for j, name := range im.paramNames {
			params[j] = ast.Param{Name: name, NameLine: stdBodyLine, NameCol: stdBodyCol}
		}
		tps := make([]*ast.TypeParam, len(im.typeParams))
		for j, name := range im.typeParams {
			tps[j] = &ast.TypeParam{Name: name, Line: stdBodyLine, Col: stdBodyCol}
		}
		c.pushTypes(ifaceScope(iteratorIface))
		c.pushTypes(paramScopeOffset(tps, len(iteratorIface.params)))
		c.checkMethodBody(im.name, params, im.params, im.body, im.ret, self, im.recvMut, im.tags)
		c.popTypes()
		c.popTypes()
	}
}

// checkMethodBody types one method body (a plain fn's is a method body
// with no self): the parameters scope their names, self joins them, and
// recvMut marks the receiver form — the one legal field-write context
// (E0813).
func (c *checker) checkMethodBody(name string, params []ast.Param, types []Type, body *ast.Block, ret Type, self Type, mutRecv bool, tags []string) {
	savedRet, savedLocals, savedRecv, savedProp := c.fnRet, c.locals, c.recvMut, c.prop
	savedTags, savedName, savedInBody, savedInClosure := c.bodyTags, c.bodyName, c.inBody, c.inClosure
	// The body context as a plain fn's (chapter 16), over the method's own
	// declared segment — an impl method's declaration, or the interface
	// method's own set at a default body (design D7).
	c.bodyTags, c.bodyName, c.inBody, c.inClosure = tags, name, true, false
	c.fnRet, c.recvMut = ret, mutRecv
	// The method's own propagation context (chapter 14), as a fn's.
	c.prop = &propCtx{fnName: name, ret: ret}
	c.locals = []map[string]Type{{}}
	if self != nil {
		c.locals[0]["self"] = self
	}
	for i, p := range params {
		c.locals[0][p.Name] = types[i]
	}
	valued := ret != nil
	c.walkItems(body.Items, walkFn)
	if valued && !tailProduces(body.Items) {
		c.fail(body.Line, body.Col, "E0501", fmt.Sprintf(
			"mixed types — the body produces (), the declared return is %s; no coercion is ever inserted",
			ret.String()))
	}
	// Chapter 13's release discipline over the typed-clean method body
	// (self stays outside it: the receiver is the mechanism's position,
	// not an owned handle).
	c.resCheck(body.Items, params, types)
	c.fnRet, c.locals, c.recvMut, c.prop = savedRet, savedLocals, savedRecv, savedProp
	c.bodyTags, c.bodyName, c.inBody, c.inClosure = savedTags, savedName, savedInBody, savedInClosure
}

// resolvedMethod carries one impl method's resolved signature (the body
// pass re-enters with it).
type resolvedMethod struct {
	fd     *ast.FnDecl
	params []Type
	ret    Type // nil = valueless
	mut    bool
}

// checkImplDecl validates one impl (design D3's judgment order): the where
// clause's validation, the interface resolution, the head's nominality (E0811), the
// derive-target guard (E0822), uniqueness (E0809), the associated bindings
// (E0805), completeness (E0807), the per-method agreement (E0826, E0808,
// E0812), and the member-set registration (E0814).
func (c *checker) checkImplDecl(x *ast.ImplDecl) {
	params := typeParamNames(x.TypeParams)
	c.pushTypes(paramScope(params))
	defer c.popTypes()
	c.implWheres[x] = c.checkWhereDecl(params, x.Where)

	var inf *ifaceInfo
	var ifaceArgs []Type
	ifaceView := Type(nil)
	if x.Iface != nil {
		// Chapter 18's marker is compiler-attached: a manual impl has no
		// method set to write (E1606, at the impl head) — checked before
		// the interface resolution, which would read the marker as an
		// unresolved name instead.
		if nt, ok := x.Iface.(*ast.NamedType); ok && nt.Qual == "" && nt.Name == "Shareable" {
			c.fail(x.Line, x.Col, "E1606", "Shareable cannot be manually implemented — the impl head names Shareable, a marker the compiler computes from a declaration's own shape: field types, payload types, category; delete the impl - the type is Shareable exactly when its own shape says so, and the bound T: Shareable checks that shape")
		}
		iv := c.resolveIfaceRef(x.Iface)
		inf, ifaceArgs, ifaceView = iv.decl, iv.args, iv
	}

	// The head: a nominal declaration, applied to its arguments.
	head := c.resolveTypeRef(x.Head, slotGeneric)
	var headName string
	var headMethods *[]memberMethod
	var hasField func(string) bool
	switch h := head.(type) {
	case recordType:
		headName, headMethods = h.decl.name, &h.decl.methods
		hasField = func(n string) bool {
			for _, f := range h.decl.fields {
				if f.name == n {
					return true
				}
			}
			return false
		}
	case newtypeType:
		headName, headMethods = h.decl.name, &h.decl.methods
	case namedType:
		headName, headMethods = h.decl.name, &h.decl.methods
	default:
		detail := "impl head is not a nominal type — "
		switch h := head.(type) {
		case baseType:
			detail += fmt.Sprintf("the head %q is a base type", h.String())
		case tupleType:
			detail += "the head is a tuple"
		case paramRef:
			detail += fmt.Sprintf("the head %q is a generic parameter", h.String())
		case ifaceType:
			detail += fmt.Sprintf("the head %q names an interface", h.String())
		default:
			detail += fmt.Sprintf("the head %q is not a nominal type", head.String())
		}
		c.fail(x.Line, x.Col, "E0811", detail+
			"; an impl head names a record, a newtype, or a sum, or a generic application of one")
	}

	// The builtin derive targets take derives clauses, never impls (E0822).
	if inf != nil && inf.deriveTarget {
		c.fail(x.Line, x.Col, "E0822", fmt.Sprintf(
			"manual impl of a builtin derive target — %q is a builtin derive target; its methods are generated by the derives clause, never hand-written",
			inf.name))
	}

	// Releasable belongs to resource heads only (chapter 13): the impl
	// hands the record its release, so a gc or value head has nothing to
	// release (E1102, at the impl head).
	if inf == releasableIface && catOf(head) != "resource" {
		c.fail(x.Line, x.Col, "E1102", fmt.Sprintf(
			"Releasable implemented by a non-resource type — %q is not a byres record; remove the impl, or declare the head type as a byres record so it owns its release",
			headName))
	}

	// Uniqueness (E0809): one interface at most once for one head. A
	// generic side (a clause, or a head holding a parameter position)
	// makes the pair an overlap judgment, not a duplicate.
	if inf != nil {
		generic := len(params) > 0 || argsContainParam(headArgs(head))
		for _, prev := range c.impls {
			if prev.iface != inf || prev.headName != headName {
				continue
			}
			if generic || prev.generic {
				c.fail(x.Line, x.Col, "E0809", fmt.Sprintf(
					"duplicate impl of one interface for one type — the impl for %q overlaps the generic impl for %q; an interface is implemented at most once for one head",
					head.String(), prev.headStr))
			}
			c.fail(x.Line, x.Col, "E0809", fmt.Sprintf(
				"duplicate impl of one interface for one type — %q is implemented for %q a second time; an interface is implemented at most once for one head",
				inf.name, headName))
		}
	}

	// The associated bindings (E0805): every hole bound, no stray name.
	assocs := map[string]Type{}
	if inf != nil {
		for _, b := range x.Assocs {
			assocs[b.Name] = c.resolveTypeRef(b.Type, slotGeneric)
		}
		for _, a := range inf.assocs {
			if _, ok := assocs[a]; !ok {
				c.fail(x.Line, x.Col, "E0805", fmt.Sprintf(
					"impl misses an associated type binding — the impl of %q for %q binds no %q; an impl binds every associated type of the interface",
					ifaceView.String(), headName, a))
			}
		}
		for _, b := range x.Assocs {
			if paramIdx(inf.assocs, b.Name) < 0 {
				c.fail(b.Line, b.Col, "E1304", fmt.Sprintf(
					"unresolved name — %q is no associated type of %q; an impl binds the interface's declared associated types",
					b.Name, inf.name))
			}
		}
	}

	// Completeness (E0807): every non-defaulted method defined.
	defines := func(name string) bool {
		for _, fd := range x.Methods {
			if fd.Name == name {
				return true
			}
		}
		return false
	}
	if inf != nil {
		for i := range inf.methods {
			im := &inf.methods[i]
			if im.body != nil || defines(im.name) {
				continue
			}
			c.fail(x.Line, x.Col, "E0807", fmt.Sprintf(
				"impl misses a non-defaulted interface method — the impl of %q for %q defines no %q; an impl defines every interface method that has no default",
				ifaceView.String(), headName, im.name))
		}
	}

	// The Iterable protocol obligations (chapter 11): a resource-category
	// head implements no Iterable (E0903 — an iterator holds a live view
	// of its collection across the loop's executions, outliving the one
	// deterministic release point a resource's discipline depends on),
	// and the Iter binding implements Iterator for the same elements
	// (E0904 — the handle contract, part of the interface every impl
	// honors).
	if inf == iterableIface {
		if catOf(head) == "resource" {
			c.fail(x.Line, x.Col, "E0903", fmt.Sprintf(
				"impl of Iterable for a resource-category type — %q is resource-category; a resource implements no %q: materialize a collection first",
				headName, "Iterable"))
		}
		if iter, ok := assocs["Iter"]; ok {
			face := ifaceType{decl: iteratorIface, args: ifaceArgs}
			if !c.implementsFace(iter, face) {
				c.fail(x.Line, x.Col, "E0904", fmt.Sprintf(
					"impl binds Iter to a non-iterator type — the impl binds %q to %q, which implements no %q; the handle contract is part of the interface",
					"Iter", iter.String(), face.String()))
			}
		}
	}

	// The methods: clause freshness (E0826), then the clause agreement and
	// the extra-method judgment (E0808 — both compare names alone, so they
	// precede the signature resolution and a broken signature never
	// misreports), then the resolved signatures under the clause, the
	// bindings, and the method's own clause.
	if len(assocs) > 0 {
		c.pushTypes(assocs)
	}
	var methods []resolvedMethod
	for _, fd := range x.Methods {
		for _, tp := range fd.TypeParams {
			if paramIdx(params, tp.Name) >= 0 {
				c.fail(tp.Line, tp.Col, "E0826", fmt.Sprintf(
					"generic parameter shadows an enclosing parameter — the method clause of %q redeclares %q; a clause name is fresh against the enclosing clause",
					fd.Name, tp.Name))
			}
		}
		if inf != nil {
			if im := findIfaceMethod(inf, fd.Name); im == nil {
				c.fail(fd.NameLine, fd.NameCol, "E0808", fmt.Sprintf(
					"impl method signature mismatches the interface method — the method %q matches no method of %q; an impl defines the interface's methods",
					fd.Name, inf.name))
			} else {
				c.checkMethodClause(fd, im, inf)
			}
		}
		c.pushTypes(paramScopeOffset(fd.TypeParams, len(params)))
		m := resolvedMethod{fd: fd, mut: fd.Recv == ast.RecvMutSelf}
		for _, p := range fd.Params {
			m.params = append(m.params, c.resolveTypeRef(p.Type, slotAnn))
		}
		if fd.Ret != nil {
			m.ret = c.resolveTypeRef(fd.Ret, slotRet)
		}
		c.popTypes()
		c.fnParams[fd] = m.params
		c.fnTags[fd] = c.resolveEffectTags(fd.EffectTags, fd.EffectLine, fd.EffectCol)
		if m.ret != nil {
			c.fnRets[fd] = m.ret
		}
		methods = append(methods, m)
	}

	// The per-method agreement (E0808) against the interface's method —
	// with the impl's interface arguments and bindings substituted into
	// the interface's signature — then the receiver discipline (E0812).
	for i := range methods {
		m := &methods[i]
		if inf != nil {
			if im := findIfaceMethod(inf, m.fd.Name); im != nil {
				c.checkImplMethodSig(m, im, inf, ifaceArgs, assocs, len(params))
			}
		}
		if m.mut && catOf(head) == "value" {
			c.fail(m.fd.NameLine, m.fd.NameCol, "E0812", fmt.Sprintf(
				"mut self receiver on a value-category type — %q takes mut self on %q, a value-category %s; a value copy has nothing to mutate in place",
				m.fd.Name, headName, valueNoun(head)))
		}
	}

	// Registration (E0814): the provided methods first (the impl's own
	// signatures), then the inherited defaults with the interface's
	// substituted signature, anchored at the impl head — the responsible
	// declaration (design D3).
	for i := range methods {
		m := &methods[i]
		from := "inherent"
		if inf != nil {
			from = inf.name
		}
		view := fnType{params: m.params, tags: c.fnTags[m.fd], ret: retOrUnit(m.ret)}
		c.addMethod(headName, headMethods, hasField,
			memberMethod{name: m.fd.Name, fn: view, from: from, mutRecv: m.mut, line: m.fd.NameLine, col: m.fd.NameCol},
			m.fd.NameLine, m.fd.NameCol, x.Line, x.Col)
	}
	if inf != nil {
		for i := range inf.methods {
			im := &inf.methods[i]
			if im.body == nil || defines(im.name) {
				continue
			}
			view := fnType{
				params: substArgs(im.params, ifaceArgs, assocs),
				tags:   im.tags,
				ret:    retOrUnit(subst(im.ret, ifaceArgs, assocs)),
			}
			c.addMethod(headName, headMethods, hasField,
				memberMethod{name: im.name, fn: view, from: inf.name, mutRecv: im.recvMut, line: x.Line, col: x.Col},
				x.Line, x.Col, x.Line, x.Col)
		}
	}

	c.impls = append(c.impls, &implInfo{
		decl:      x,
		iface:     inf,
		ifaceArgs: ifaceArgs,
		head:      head,
		headName:  headName,
		headStr:   head.String(),
		generic:   len(params) > 0 || argsContainParam(headArgs(head)),
		assocs:    assocs,
	})
	if len(assocs) > 0 {
		c.popTypes()
	}
}

// headArgs reads a head type's application arguments (nil without).
func headArgs(t Type) []Type {
	switch h := t.(type) {
	case recordType:
		return h.args
	case newtypeType:
		return h.args
	case namedType:
		return h.args
	case ifaceType:
		return h.args
	}
	return nil
}

// retOrUnit renders a valueless view's return as the unit type.
func retOrUnit(t Type) Type {
	if t == nil {
		return unitType{}
	}
	return t
}

// findIfaceMethod finds an interface method by name (nil absent).
func findIfaceMethod(inf *ifaceInfo, name string) *ifaceMethod {
	for i := range inf.methods {
		if inf.methods[i].name == name {
			return &inf.methods[i]
		}
	}
	return nil
}

// checkMethodClause holds one method's clause agreement with its interface
// method (E0808): the comparison reads names alone, so it runs before the
// signature resolves — a broken signature never misreports. The anchor is
// the method name.
func (c *checker) checkMethodClause(fd *ast.FnDecl, im *ifaceMethod, inf *ifaceInfo) {
	implNames := typeParamNames(fd.TypeParams)
	if nameSeqEq(im.typeParams, implNames) {
		return
	}
	ifaceRend := "<" + strings.Join(im.typeParams, ", ") + ">"
	implRend := "<" + strings.Join(implNames, ", ") + ">"
	var detail string
	switch {
	case len(im.typeParams) > 0 && len(implNames) == 0:
		detail = fmt.Sprintf("the method clause of %q is %s in %q and absent in the impl", fd.Name, ifaceRend, inf.name)
	case len(im.typeParams) == 0:
		detail = fmt.Sprintf("the method clause of %q is absent in %q and %s in the impl", fd.Name, inf.name, implRend)
	default:
		detail = fmt.Sprintf("the method clause of %q is %s in %q and %s in the impl", fd.Name, ifaceRend, inf.name, implRend)
	}
	c.fail(fd.NameLine, fd.NameCol, "E0808", "impl method signature mismatches the interface method — "+detail+"; the method clause repeats exactly")
}

// checkImplMethodSig holds one method's agreement with its interface
// method (E0808, design D3's order): the receiver, the parameter count and
// types, and the return. The interface's signature arrives substituted
// with the impl's interface arguments and associated bindings, its
// method-clause positions rebased onto the impl method's own offset (the
// identical method clauses resolve at different offsets — the interface's
// after the interface's clause, the impl's after the impl's, design
// D10(b)); every anchor is the method name.
func (c *checker) checkImplMethodSig(m *resolvedMethod, im *ifaceMethod, inf *ifaceInfo, ifaceArgs []Type, assocs map[string]Type, clause int) {
	nl, nc := m.fd.NameLine, m.fd.NameCol
	recvStr := func(mut bool) string {
		if mut {
			return "mut self"
		}
		return "self"
	}
	if im.recvMut != m.mut {
		c.fail(nl, nc, "E0808", fmt.Sprintf(
			"impl method signature mismatches the interface method — the receiver of %q is %s in the impl and %s in %q; signatures match exactly",
			m.fd.Name, recvStr(m.mut), recvStr(im.recvMut), inf.name))
	}
	if len(m.params) != len(im.params) {
		c.fail(nl, nc, "E0808", fmt.Sprintf(
			"impl method signature mismatches the interface method — the parameter count of %q is %d in the impl and %d in %q; signatures match exactly",
			m.fd.Name, len(m.params), len(im.params), inf.name))
	}
	for i := range im.params {
		want := rebaseClause(subst(im.params[i], ifaceArgs, assocs), len(inf.params), clause)
		if !sameType(m.params[i], want) {
			c.fail(nl, nc, "E0808", fmt.Sprintf(
				"impl method signature mismatches the interface method — the parameter %q of %q is %s in the impl and %s in %q; signatures match exactly",
				m.fd.Params[i].Name, m.fd.Name, m.params[i].String(), want.String(), inf.name))
		}
	}
	iRet := rebaseClause(subst(im.ret, ifaceArgs, assocs), len(inf.params), clause)
	switch {
	case m.ret != nil && iRet == nil:
		c.fail(nl, nc, "E0808", fmt.Sprintf(
			"impl method signature mismatches the interface method — the return of %q is %s in the impl and absent in %q; signatures match exactly",
			m.fd.Name, m.ret.String(), inf.name))
	case m.ret == nil && iRet != nil:
		c.fail(nl, nc, "E0808", fmt.Sprintf(
			"impl method signature mismatches the interface method — the return of %q is absent in the impl and %s in %q; signatures match exactly",
			m.fd.Name, iRet.String(), inf.name))
	case m.ret != nil && !sameType(m.ret, iRet):
		c.fail(nl, nc, "E0808", fmt.Sprintf(
			"impl method signature mismatches the interface method — the return of %q is %s in the impl and %s in %q; signatures match exactly",
			m.fd.Name, m.ret.String(), iRet.String(), inf.name))
	}
	// Chapter 16's exact segment equality (design D7, R5), after the
	// signature's own judgments: an impl method's declared set must equal
	// the interface method's in both directions — a caller sees the
	// interface's set, whichever implementation runs. An omitted segment
	// is the empty set (pure) and compares like any other; an override of
	// a default body walks this same judgment.
	if !tagSetEq(c.fnTags[m.fd], im.tags) {
		side := func(ts []string) string {
			if len(ts) == 0 {
				return "declares no effect segment"
			}
			ds := make([]string, len(ts))
			for i, t := range ts {
				ds[i] = displayTag(t)
			}
			return fmt.Sprintf("declares %s", quoteTags(ds))
		}
		c.fail(nl, nc, "E1404", fmt.Sprintf(
			"impl method effect set disagrees with the interface — the impl method %s and the interface method %s; copy the interface method's effect segment into the impl method's signature exactly, in both directions",
			side(c.fnTags[m.fd]), side(im.tags)))
	}
}

// nameSeqEq compares two name slices element-wise.
func nameSeqEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// resolveIfaceRef resolves an impl's interface clause. The position
// resolves interfaces only: a declared interface of the module or one of
// the prelude's faces (the clause's parameters may apply it). No ratified
// code names a non-interface there, so the miss composes under E1304's
// resolution rule — a registry follow-up candidate, disclosed in the task
// record.
func (c *checker) resolveIfaceRef(tr ast.TypeRef) ifaceType {
	nt, ok := tr.(*ast.NamedType)
	if !ok {
		line, col := refPos(tr)
		c.fail(line, col, "E1304",
			"unresolved name — the impl clause names no declared interface; the impl clause resolves a declared interface of this or an imported module")
	}
	if nt.Qual != "" {
		if mod, is := c.importQualifier(nt.Qual); is {
			return c.importIface(mod, nt, "E1304",
				fmt.Sprintf("unresolved name — %q names no declared interface; the impl clause resolves a declared interface of this or an imported module", nt.Name))
		}
		c.fail(nt.Line, nt.Col, "E1304", fmt.Sprintf(
			"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
			nt.Qual, nt.Qual+"."+nt.Name))
	}
	if t, inClause := c.lookupTypeScope(nt.Name); inClause {
		_ = t // a clause parameter names no interface
		c.fail(nt.Line, nt.Col, "E1304", fmt.Sprintf(
			"unresolved name — %q is a generic parameter, not a declared interface; the impl clause resolves a declared interface of this or an imported module",
			nt.Name))
	}
	if sym, ok := c.syms[nt.Name]; ok {
		if sym.kind == symIface {
			c.checkArity(nt, sym.iface.name, sym.iface.params)
			return ifaceType{decl: sym.iface, args: c.resolveArgs(nt.Args, slotGeneric)}
		}
		c.fail(nt.Line, nt.Col, "E1304", fmt.Sprintf(
			"unresolved name — %q names no declared interface; the impl clause resolves a declared interface of this or an imported module",
			nt.Name))
	}
	inf := builtinIface(nt.Name)
	if inf == nil {
		c.fail(nt.Line, nt.Col, "E1304", fmt.Sprintf(
			"unresolved name — %q names no declared interface; the impl clause resolves a declared interface of this or an imported module",
			nt.Name))
	}
	c.checkArity(nt, inf.name, inf.params)
	return ifaceType{decl: inf, args: c.resolveArgs(nt.Args, slotGeneric)}
}

// checkImplBodies types an impl's method bodies: the scope re-enters with
// the clause, the associated bindings, and each method's own clause; self
// is the head (the registered view's declaration); recvMut carries the
// receiver form.
func (c *checker) checkImplBodies(x *ast.ImplDecl) {
	var info *implInfo
	for _, im := range c.impls {
		if im.decl == x {
			info = im
			break
		}
	}
	if info == nil {
		return // unreachable: pass 2a2 validated every impl
	}
	params := typeParamNames(x.TypeParams)
	c.pushTypes(paramScope(params))
	defer c.popTypes()
	if len(info.assocs) > 0 {
		c.pushTypes(info.assocs)
		defer c.popTypes()
	}
	savedBounds := c.fnBounds
	c.fnBounds = c.implWheres[x]
	for _, fd := range x.Methods {
		c.pushTypes(paramScopeOffset(fd.TypeParams, len(params)))
		c.checkMethodBody(fd.Name, fd.Params, c.fnParams[fd], &fd.Body, c.fnRets[fd], info.head, fd.Recv == ast.RecvMutSelf, c.fnTags[fd])
		c.popTypes()
	}
	c.fnBounds = savedBounds
}

// --- chapter 10: where clauses (design D6) ------------------------------------

// fnBound is one validated where bound: the clause position it constrains
// and the interface face it grants (the position's member set through the
// bound's, and its satisfaction at each application).
type fnBound struct {
	idx  int
	face ifaceType
}

// fnEq is one validated associated-type equality: the subject position,
// the associated name, and the concrete right side.
type fnEq struct {
	idx   int
	assoc string
	rhs   Type
}

// whereInfo is a declaration's validated where clause (nil = none).
type whereInfo struct {
	bounds []fnBound
	eqs    []fnEq
}

// checkWhereDecl validates one declaration's where clause (design D6):
// every subject names a clause parameter of the declaration, every bound
// names a declared interface (E0829, anchored at the bound's first
// token), every equality's left side names an associated type one of the
// subject's collected bounds declares, and the right side is concrete
// (E0831, anchored at the right side's first token). The clause is the
// declaration's fact: its body reads the granted method sets, each
// application the satisfaction (E0830).
func (c *checker) checkWhereDecl(params []string, wheres []*ast.WhereBound) *whereInfo {
	if len(wheres) == 0 {
		return nil
	}
	var info whereInfo
	for _, wb := range wheres {
		idx := paramIdx(params, wb.Subject)
		if idx < 0 {
			// a where clause constrains the declaration's own clause
			// parameters (chapter 10 composes the parser family's E0105)
			c.fail(wb.Line, wb.Col, "E0105", fmt.Sprintf(
				"unexpected token — the where subject %q is no generic parameter of the declaration; a where clause constrains the declaration's own clause parameters",
				wb.Subject))
		}
		for _, ref := range wb.Ifaces {
			info.bounds = append(info.bounds, fnBound{idx: idx, face: c.boundIface(ref)})
		}
		for _, eq := range wb.Eq {
			line, col := refPos(eq.RHS)
			rhs := c.resolveTypeRef(eq.RHS, slotGeneric)
			if containsParam(rhs) {
				c.fail(line, col, "E0831", fmt.Sprintf(
					"associated-type equality right side is not concrete — the right side %q is a generic parameter; the equality binds the associated type to a base type, a nominal type, or a generic application of them",
					rhs.String()))
			}
			declared := false
			for _, b := range info.bounds {
				if b.idx != idx {
					continue
				}
				for _, a := range b.face.decl.assocs {
					if a == eq.Assoc {
						declared = true
					}
				}
			}
			if !declared {
				c.fail(eq.Line, eq.Col, "E0105", fmt.Sprintf(
					"unexpected token — %q is no associated type of the subject's bounds; the equality's left side names an associated type one of the subject's bound interfaces declares",
					eq.Assoc))
			}
			info.eqs = append(info.eqs, fnEq{idx: idx, assoc: eq.Assoc, rhs: rhs})
		}
	}
	return &info
}

// boundIface resolves one where bound's interface reference: a bound
// grants an interface's method set, so the position reads interfaces
// only — a non-interface name is E0829 (named by the name itself) and an
// unknown one is E1304, both anchored at the bound's first token (the
// position mirrors the Dyn box's interface argument, never the value
// slot's E0821).
func (c *checker) boundIface(nt *ast.NamedType) ifaceType {
	if nt.Qual != "" {
		if mod, is := c.importQualifier(nt.Qual); is {
			return c.importIface(mod, nt, "E0829", notAnIface(nt.Name))
		}
		full := nt.Qual + "." + nt.Name
		c.fail(nt.Line, nt.Col, "E1304", fmt.Sprintf(
			"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
			nt.Qual, full))
	}
	if _, inClause := c.lookupTypeScope(nt.Name); inClause {
		// a clause parameter names no interface
		c.fail(nt.Line, nt.Col, "E0829", notAnIface(nt.Name))
	}
	if sym, ok := c.syms[nt.Name]; ok {
		if sym.kind == symIface {
			c.checkArity(nt, sym.iface.name, sym.iface.params)
			return ifaceType{decl: sym.iface, args: c.resolveArgs(nt.Args, slotGeneric)}
		}
		c.fail(nt.Line, nt.Col, "E0829", notAnIface(nt.Name))
	}
	if inf := builtinIface(nt.Name); inf != nil {
		c.checkArity(nt, inf.name, inf.params)
		return ifaceType{decl: inf, args: c.resolveArgs(nt.Args, slotGeneric)}
	}
	if nt.Name == "Shareable" {
		// chapter 18's compiler-attached marker: prelude-visible (the
		// panic family's precedent — no import gates the name), no method
		// set — the application's judgment is the closed set's own shape
		// rule, never an implements walk
		c.checkArity(nt, "Shareable", nil)
		return ifaceType{decl: shareableMarker}
	}
	switch nt.Name {
	case "Never", "Result", "Option", "Dyn", "List", "Map", "Set", "Range":
		// known prelude names that are no interfaces — E0829 below
	default:
		if !baseNames[nt.Name] {
			c.fail(nt.Line, nt.Col, "E1304", bareUnresolved(nt.Name))
		}
	}
	c.fail(nt.Line, nt.Col, "E0829", notAnIface(nt.Name))
	panic("unreachable bound iface")
}

// notAnIface composes E0829's message.
func notAnIface(name string) string {
	return fmt.Sprintf(
		"where bound is not a declared interface — the bound %q names no interface; bounds grant interface method sets, and only interfaces have them",
		name)
}

// checkWhereSatisfies holds one where clause at an application (E0830):
// each bound's subject position, having received its type argument, walks
// the same implements judgment the Dyn box reads — one mechanism, two
// codes (the anchor here is the call head, design D6).
func (c *checker) checkWhereSatisfies(info *whereInfo, args []Type, x *ast.Call, name string) {
	if info == nil {
		return
	}
	for _, b := range info.bounds {
		if b.idx >= len(args) {
			continue
		}
		if b.face.decl == shareableMarker {
			// the marker grants no method set — the judgment is the
			// closed set's own shape rule (design D3)
			if c.shareableBoundArg(args[b.idx]) {
				continue
			}
		} else if c.implementsFace(args[b.idx], b.face) {
			continue
		}
		line, col := exprPos(x.Fn)
		c.fail(line, col, "E0830", fmt.Sprintf(
			"type argument does not satisfy a where bound — %q implements no %q; the where bound of %q holds at every call",
			args[b.idx].String(), b.face.String(), name))
	}
}

// --- chapter 10: derives clauses (design D8) ----------------------------------

// checkRecordDerives checks a record's derives clause (E0823's declaration
// side — a parameter position defers to each instantiation) and registers
// the generated methods into the member set.
func (c *checker) checkRecordDerives(x *ast.RecordDecl, rec *recordInfo) {
	if len(rec.derives) == 0 {
		return
	}
	for _, target := range rec.derives {
		for _, f := range rec.fields {
			if containsParam(f.typ) {
				continue
			}
			if carriesType(f.typ, target) {
				continue
			}
			c.fail(f.line, f.col, "E0823", fmt.Sprintf(
				"derive field requirement unmet — the field %q is %q, which carries no %q; every field carries each target of the clause",
				f.name, f.typ.String(), target))
		}
	}
	self := Type(recordType{decl: rec})
	if len(rec.params) > 0 {
		self = recordType{decl: rec, args: clauseArgs(rec.params)}
	}
	c.registerDerives(self, rec.derives, rec.name, &rec.methods, func(n string) bool {
		for _, f := range rec.fields {
			if f.name == n {
				return true
			}
		}
		return false
	}, x.Derives.Line, x.Derives.Col)
}

// checkNewtypeDerives checks a newtype's derives clause: the underlying
// type carries each target (a parameter position defers to each
// instantiation).
func (c *checker) checkNewtypeDerives(x *ast.NewtypeDecl, nt *newtypeInfo) {
	if len(nt.derives) == 0 {
		return
	}
	for _, target := range nt.derives {
		if containsParam(nt.underlying) {
			continue
		}
		if carriesType(nt.underlying, target) {
			continue
		}
		c.fail(nt.underLine, nt.underCol, "E0823", fmt.Sprintf(
			"derive field requirement unmet — the underlying type is %q, which carries no %q; the underlying type carries each target of the clause",
			nt.underlying.String(), target))
	}
	self := Type(newtypeType{decl: nt})
	if len(nt.params) > 0 {
		self = newtypeType{decl: nt, args: clauseArgs(nt.params)}
	}
	c.registerDerives(self, nt.derives, nt.name, &nt.methods, nil, x.Derives.Line, x.Derives.Col)
}

// checkSumDerives checks a sum's derives clause: every payload of every
// variant carries each target (parameter positions defer to each
// instantiation).
func (c *checker) checkSumDerives(x *ast.SumDecl, sum *sumInfo) {
	if len(sum.derives) == 0 {
		return
	}
	for _, target := range sum.derives {
		for i := range sum.variants {
			v := &sum.variants[i]
			for j := range v.payloads {
				if containsParam(v.payloads[j]) {
					continue
				}
				if carriesType(v.payloads[j], target) {
					continue
				}
				c.fail(v.payloadPos[j][0], v.payloadPos[j][1], "E0823", fmt.Sprintf(
					"derive field requirement unmet — the payload of %q is %q, which carries no %q; every payload carries each target of the clause",
					v.name, v.payloads[j].String(), target))
			}
		}
	}
	self := Type(namedType{decl: sum})
	if len(sum.params) > 0 {
		self = namedType{decl: sum, args: clauseArgs(sum.params)}
	}
	c.registerDerives(self, sum.derives, sum.name, &sum.methods, nil, x.Derives.Line, x.Derives.Col)
}

// registerDerives adds one derives clause's generated methods (design D8)
// under the collision rule (E0814).
func (c *checker) registerDerives(self Type, targets []string, owner string, methods *[]memberMethod, hasField func(string) bool, line, col int) {
	ms := deriveMethods(self, targets, line, col)
	for i := range ms {
		c.addMethod(owner, methods, hasField, ms[i], line, col, line, col)
	}
}

// refPos returns a type reference node's anchor position.
func refPos(tr ast.TypeRef) (int, int) {
	switch x := tr.(type) {
	case *ast.NamedType:
		return x.Line, x.Col
	case *ast.TupleType:
		return x.Line, x.Col
	case *ast.UnitType:
		return x.Line, x.Col
	case *ast.FnType:
		return x.Line, x.Col
	}
	return 1, 1
}

// --- annotations (chapter 7's slots) ------------------------------------------

// slotKind marks which never-legal rule applies to a slot: an ordinary
// annotation position rejects a bare Never (E0703), a declared fn return
// accepts it, and a generic argument position defers to the parameter's
// own rules (Result's E position is E1204's).
type slotKind int

const (
	slotAnn slotKind = iota
	slotRet
	slotGeneric
)

func (c *checker) resolveTypeRef(tr ast.TypeRef, slot slotKind) Type {
	switch x := tr.(type) {
	case *ast.UnitType:
		return unitType{}
	case *ast.TupleType:
		elems := make([]Type, len(x.Elems))
		for i, e := range x.Elems {
			elems[i] = c.resolveTypeRef(e, slotAnn)
			// Chapter 13's composite-position ban: a tuple element is a
			// composite slot — a value there would share the one handle
			// (E1106, at the element's own reference).
			if catOf(elems[i]) == "resource" {
				line, col := refPos(e)
				c.fail(line, col, "E1106", fmt.Sprintf(
					"resource type in a composite position — %q appears as a tuple element; a resource type appears only in binding positions - a parameter, a let or scope-head binding, a return type",
					elems[i].String()))
			}
		}
		return tupleType{elems: elems}
	case *ast.FnType:
		params := make([]Type, len(x.Params))
		for i, p := range x.Params {
			params[i] = c.resolveTypeRef(p, slotAnn)
		}
		// The type slot's segment resolves like a declaration's (design
		// D3): canonical keys in, so the agreement and subset faces later
		// compare one spelling of one effect.
		return fnType{params: params, tags: c.resolveEffectTags(x.EffectTags, x.TagLine, x.TagCol), ret: c.resolveTypeRef(x.Ret, slotRet)}
	case *ast.NamedType:
		return c.resolveNamed(x, slot)
	}
	panic("unreachable typeref")
}

// resolveNamed resolves one named reference through the three layers.
// E1304's two message shapes are position-independent: "type positions
// resolve by the same rules" is the registry's rule, so a type position
// reads the same bare and qualifier messages a value position does.
func (c *checker) resolveNamed(x *ast.NamedType, slot slotKind) Type {
	if x.Qual != "" {
		if mod, is := c.importQualifier(x.Qual); is {
			return c.importTypeRef(mod, x, slot)
		}
		full := x.Qual + "." + x.Name
		c.fail(x.Line, x.Col, "E1304", fmt.Sprintf(
			"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
			x.Qual, full))
	}
	// A generic clause's parameters shadow the module namespace while the
	// declaration's annotations and bodies resolve (chapter 10).
	if t, ok := c.lookupTypeScope(x.Name); ok {
		if len(x.Args) > 0 {
			c.arityFail(x.ArgLine, x.ArgCol, x.Line, x.Col, x.Name, 0, len(x.Args))
		}
		return t
	}
	// The module namespace shadows the prelude.
	if sym, ok := c.syms[x.Name]; ok {
		switch sym.kind {
		case symType:
			c.checkArity(x, sym.sum.name, sym.sum.params)
			return namedType{decl: sym.sum, args: c.resolveArgs(x.Args, slotGeneric)}
		case symRecord:
			c.checkArity(x, sym.rec.name, sym.rec.params)
			return recordType{decl: sym.rec, args: c.resolveArgs(x.Args, slotGeneric)}
		case symNewtype:
			c.checkArity(x, sym.nt.name, sym.nt.params)
			return newtypeType{decl: sym.nt, args: c.resolveArgs(x.Args, slotGeneric)}
		case symIface:
			// An interface occupies type slots only as Dyn<Interface> —
			// the bare name in a value slot is E0821 (the impl clause's
			// position resolves interfaces and never walks here).
			c.checkArity(x, sym.iface.name, sym.iface.params)
			args := c.resolveArgs(x.Args, slotGeneric)
			c.valueSlotIface(x, args)
			return ifaceType{decl: sym.iface, args: args}
		default:
			// a value name held by the module fills no type slot
			c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
		}
	}
	switch {
	case x.Name == "Never":
		c.checkArity(x, "Never", nil)
		if slot == slotAnn {
			c.fail(x.Line, x.Col, "E0703",
				"Never in a non-return annotation position — the bottom type has no values; it annotates only a declared fn return type")
		}
		return neverType{}
	case x.Name == "Result" || x.Name == "Option":
		sum := resultSum
		if x.Name == "Option" {
			sum = optionSum
		}
		c.checkArity(x, x.Name, sum.params)
		args := make([]Type, len(x.Args))
		for i, a := range x.Args {
			args[i] = c.resolveTypeRef(a, slotGeneric)
			// The builtin sums resolve their arguments inline (the E1204
			// check below reads them); the instantiation ban is the same
			// resolveArgs one — a resource in the payload slot is the box
			// route (E1106, at the argument's own reference).
			if catOf(args[i]) == "resource" {
				line, col := refPos(a)
				c.fail(line, col, "E1106", fmt.Sprintf(
					"resource type in a composite position — %q appears as a generic argument at the instantiation; a resource type appears only in binding positions - a parameter, a let or scope-head binding, a return type",
					args[i].String()))
			}
		}
		if sum == resultSum {
			if _, named := args[1].(namedType); !named {
				line, col := refPos(x.Args[1])
				c.fail(line, col, "E1204", fmt.Sprintf(
					"Result error type is not a named sum type — the error parameter resolves to %s; declare a named sum carrying the failure variants and use it as the error parameter",
					args[1].String()))
			}
		}
		return namedType{decl: sum, args: args}
	case builtinIface(x.Name) != nil:
		// the prelude's interfaces (the derive targets and the iteration
		// protocol) face the same value-slot rule (E0821)
		inf := builtinIface(x.Name)
		c.checkArity(x, inf.name, inf.params)
		args := c.resolveArgs(x.Args, slotGeneric)
		c.valueSlotIface(x, args)
		return ifaceType{decl: inf, args: args}
	case x.Name == "List" || x.Name == "Map" || x.Name == "Set":
		sum := listSum
		if x.Name == "Map" {
			sum = mapSum
		}
		if x.Name == "Set" {
			sum = setSum
		}
		c.checkArity(x, x.Name, sum.params)
		return namedType{decl: sum, args: c.resolveArgs(x.Args, slotGeneric)}
	case x.Name == "Range":
		// chapter 11's builtin generic type; its parameter's integer
		// obligation is the operator's (E0902 at the operands — the
		// annotation application carries no ratified trigger position)
		c.checkArity(x, "Range", rangeSum.params)
		return namedType{decl: rangeSum, args: c.resolveArgs(x.Args, slotGeneric)}
	case x.Name == "Dyn":
		// the erased box: one interface argument, declaring no associated
		// types (E0819's binding could not survive erasure)
		c.checkArity(x, "Dyn", []string{"I"})
		dt := c.dynFace(x.Args)
		// The box route, type face (chapter 13): under the completeness
		// obligation every box of Releasable would box a resource, and a
		// box shares its handle (E1106, at the Dyn reference).
		if dt.inf == releasableIface {
			c.fail(x.Line, x.Col, "E1106", fmt.Sprintf(
				"resource type in a composite position — \"Dyn<%s>\" is the box route in a type position; under the completeness obligation every box of Releasable would box a resource, and a resource is never boxed",
				dt.inf.name))
		}
		return Type(dt)
	case x.Name == "Shareable":
		// chapter 18's marker occupies no value surface (the spec's own
		// sentence): a bound, never a parameter annotation, a let slot,
		// or a type argument
		c.fail(x.Line, x.Col, "E0821", fmt.Sprintf(
			"interface name used as a value type — %q is the compiler-attached marker of chapter 18; it stands in bounds (T: Shareable), never in a value type slot",
			x.Name))
	case baseNames[x.Name]:
		c.checkArity(x, x.Name, nil)
		return baseType(x.Name)
	}
	c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
	panic("unreachable name")
}

// checkArity compares a generic application's argument count with the
// declaration's, anchored at the application's own `<` token.
func (c *checker) checkArity(x *ast.NamedType, name string, params []string) {
	if len(x.Args) == len(params) {
		return
	}
	line, col := x.ArgLine, x.ArgCol
	if line == 0 {
		line, col = x.Line, x.Col
	}
	noun := "type arguments"
	if len(params) == 1 {
		noun = "type argument"
	}
	c.fail(line, col, "E0828", fmt.Sprintf(
		"type argument arity mismatch — %q wants %d %s, got %d; match the declaration's arity",
		name, len(params), noun, len(x.Args)))
}

func bareUnresolved(name string) string {
	return fmt.Sprintf(
		"unresolved name — %q is held by no scope: no block-local binding, module declaration, or prelude name carries it; fix the spelling, declare the name, or import its module",
		name)
}

// --- statements and bodies ---------------------------------------------------

// checkBinding types one binding: the annotation resolves (an ordinary
// slot), the initializer types against it as the expected type, and the
// disagreement anchors at the binding's name token. It returns the
// binding's type; registration is the caller's (locals or module).
func (c *checker) checkBinding(b *ast.Binding) Type {
	if b.Pat != nil {
		// A module-level destructure stays at the composite boundary (the
		// block-level form types via checkLetPattern).
		c.bnd(bndCompForms)
	}
	var ann Type
	if b.Typ != nil {
		ann = c.resolveTypeRef(b.Typ, slotAnn)
	}
	// The initializer walks under its binding's name (chapter 18, design
	// D2): an expectation-free channel construction inside it names the
	// binding in E1617's context.
	savedLet := c.letName
	c.letName = b.Name
	it := c.typeOf(b.Init, ann)
	c.letName = savedLet
	if ann != nil && !agree(it, ann) && !c.checkFnSlot(it, ann, b.NameLine, b.NameCol) {
		c.fail(b.NameLine, b.NameCol, "E0501", fmt.Sprintf(
			"mixed types — the expression is %s, the annotation is %s; no coercion is ever inserted",
			it.String(), ann.String()))
	}
	if ann != nil {
		return ann
	}
	return it
}

// checkLetPattern types a block-level let-tuple destructure (chapter 8):
// the pattern is irrefutable (the parser guarantees bindings, wildcards,
// and nested tuples), so the initializer must be a tuple of the same
// arity and each element binds its name at its position's type. The
// bindings enter the current scope — a later same-name binding shadows by
// overwriting, the M3 mechanism the spec semantics keep.
func (c *checker) checkLetPattern(b *ast.Binding) {
	var ann Type
	if b.Typ != nil {
		ann = c.resolveTypeRef(b.Typ, slotAnn)
	}
	// A destructure binds many names — no one name names the initializer
	// (E1617's context walks under the empty string here).
	savedLet := c.letName
	c.letName = ""
	it := c.typeOf(b.Init, ann)
	c.letName = savedLet
	if ann != nil {
		if !agree(it, ann) {
			line, col := patAnchor(b.Pat)
			if !c.checkFnSlot(it, ann, line, col) {
				c.fail(line, col, "E0501", fmt.Sprintf(
					"mixed types — the expression is %s, the annotation is %s; no coercion is ever inserted",
					it.String(), ann.String()))
			}
		}
		it = ann
	}
	c.destructure(b.Pat, it)
}

// destructure binds one irrefutable pattern's names at their positions'
// types; a non-tuple (or wrong-arity) scrutinee against a tuple pattern
// is E0501 at the pattern.
func (c *checker) destructure(p ast.Pattern, t Type) {
	switch x := p.(type) {
	case *ast.PatWildcard:
	case *ast.PatBinding:
		c.locals[len(c.locals)-1][x.Name] = t
	case *ast.PatTuple:
		tt, ok := t.(tupleType)
		if !ok || len(tt.elems) != len(x.Elems) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — the tuple pattern is matched against %s, which is not a tuple of the same arity; no coercion is ever inserted",
				t.String()))
		}
		for i, e := range x.Elems {
			c.destructure(e, tt.elems[i])
		}
	}
}

// patAnchor reports a pattern's first-token position (the annotation
// disagreement of a destructuring let anchors at its pattern).
func patAnchor(p ast.Pattern) (int, int) {
	switch x := p.(type) {
	case *ast.PatLiteral:
		return x.Line, x.Col
	case *ast.PatWildcard:
		return x.Line, x.Col
	case *ast.PatBinding:
		return x.Line, x.Col
	case *ast.PatOr:
		return x.Line, x.Col
	case *ast.PatTuple:
		return x.Line, x.Col
	case *ast.PatVariant:
		return x.Line, x.Col
	}
	return 1, 1
}

// checkAssign types one assignment against the target binding's type,
// anchoring at the target's name token; a target crossing a closure
// boundary faces the capture ledger first (E1003/E1002). The self.field
// form is the one field write (chapter 10): only a mut self method body
// holds it (E0813), and the field checks against the receiver's record.
func (c *checker) checkAssign(a *ast.Assign) {
	if a.Field != "" {
		st, layer, ok := c.lookupLocalDepth("self")
		if !c.recvMut {
			// the placement rule precedes any field knowledge (E0813's
			// fact is the receiver form, not the field)
			if ok {
				c.fail(a.Line, a.Col, "E0813", fmt.Sprintf(
					"field write outside a mut self method body — the write to %q sits in a plain %q method; the receiver field assignment lives only inside a mut self body",
					"self."+a.Field, "self"))
			}
			c.fail(a.Line, a.Col, "E0813", fmt.Sprintf(
				"field write outside a mut self method body — the write to %q sits outside any method body; the receiver field assignment lives only inside a mut self body",
				"self."+a.Field))
		}
		if !ok {
			c.fail(a.Line, a.Col, "E1304", bareUnresolved("self"))
		}
		rt, isRec := st.(recordType)
		if !isRec {
			c.fail(a.Line, a.Col, "E0816", fmt.Sprintf(
				"no such member on the receiver's type — %s declares no field %q; the receiver field assignment names a record field",
				st.String(), a.Field))
		}
		var ft Type
		for _, f := range rt.decl.fields {
			if f.name == a.Field {
				ft = subst(f.typ, rt.args, nil)
				break
			}
		}
		if ft == nil {
			c.fail(a.Line, a.Col, "E0816", fmt.Sprintf(
				"no such member on the receiver's type — %q has no field %q; the receiver field assignment names a declared field",
				rt.decl.name, a.Field))
		}
		c.noteCapture("self", st, layer, a.Line, a.Col)
		c.resAssignCheck(a)
		vt := c.typeOf(a.Value, ft)
		if !agree(vt, ft) {
			c.fail(a.Line, a.Col, "E0501", fmt.Sprintf(
				"mixed types — the value is %s, the field %q is %s; no coercion is ever inserted",
				vt.String(), a.Field, ft.String()))
		}
		return
	}
	t, layer, ok := c.lookupLocalDepth(a.Name)
	if !ok {
		sym, is := c.syms[a.Name]
		if !is || sym.kind != symLet || sym.letType == nil {
			c.fail(a.Line, a.Col, "E1304", bareUnresolved(a.Name))
		}
		t = sym.letType
		layer = -1
	}
	// Chapter 13's alias ban fires before the type judgment: an assignment
	// moving a resource binding on either side is the rebound shape (E1105,
	// at the `=`) — the mismatch below would otherwise mask it.
	if catOf(t) == "resource" {
		c.fail(a.OpLine, a.OpCol, "E1105",
			"resource binding rebound — the assignment moves a resource binding on one side; the one sanctioned move is a transfer, which hands over and kills the source")
	}
	c.resAssignCheck(a)
	c.noteAssignCapture(a.Name, t, layer, a.Line, a.Col)
	vt := c.typeOf(a.Value, t)
	if !agree(vt, t) {
		c.fail(a.Line, a.Col, "E0501", fmt.Sprintf(
			"mixed types — the value is %s, %q is %s; no coercion is ever inserted",
			vt.String(), a.Name, t.String()))
	}
}

// checkReturn types one return against the enclosing fn's declared
// return: a valued return anchors at the value's first token, a bare
// return in a valued fn at the keyword itself.
func (c *checker) checkReturn(r *ast.Return) {
	// A task body's returns carry the task's own inferred value (chapter
	// 18), never a declared return — the exits' join takes over here.
	if c.inTaskBody {
		c.taskReturn(r)
		return
	}
	if c.fnRet == nil {
		return // a valueless fn — the parser guarantees bare returns only
	}
	if !r.HasValue {
		c.fail(r.Line, r.Col, "E0501", fmt.Sprintf(
			"mixed types — the return expression is (), the declared return is %s; no coercion is ever inserted",
			c.fnRet.String()))
	}
	vt := c.typeOf(r.Value, c.fnRet)
	if !agree(vt, c.fnRet) {
		line, col := exprPos(r.Value)
		if !c.checkFnSlot(vt, c.fnRet, line, col) {
			c.fail(line, col, "E0501", fmt.Sprintf(
				"mixed types — the return expression is %s, the declared return is %s; no coercion is ever inserted",
				vt.String(), c.fnRet.String()))
		}
	}
}

// checkFnDecl types one fn body: parameters scope their names, the tail
// rule holds the body's final item, and a valued body that produces no
// value reports at the declaration itself.
func (c *checker) checkFnDecl(fd *ast.FnDecl) {
	savedRet, savedLocals, savedRecv, savedBounds, savedProp := c.fnRet, c.locals, c.recvMut, c.fnBounds, c.prop
	savedTags, savedName, savedInBody, savedInClosure := c.bodyTags, c.bodyName, c.inBody, c.inClosure
	// The body context (chapter 16): every call in this body judges its
	// callee's set against the fn's own declared segment, named by the fn
	// in E1401's message.
	c.bodyTags, c.bodyName, c.inBody, c.inClosure = c.fnTags[fd], fd.Name, true, false
	c.fnRet, c.recvMut, c.fnBounds = c.fnRets[fd], false, c.fnWheres[fd]
	// The fn's own propagation context (chapter 14): its `?`s face its
	// declared return, named by the fn itself (E1202's message).
	c.prop = &propCtx{fnName: fd.Name, ret: c.fnRet}
	c.locals = []map[string]Type{{}}
	for i, p := range fd.Params {
		c.locals[0][p.Name] = c.fnParams[fd][i]
	}
	valued := c.fnRet != nil
	c.walkItems(fd.Body.Items, walkFn)
	if valued && !tailProduces(fd.Body.Items) {
		c.fail(fd.Line, fd.Col, "E0501", fmt.Sprintf(
			"mixed types — the body produces (), the declared return is %s; no coercion is ever inserted",
			c.fnRet.String()))
	}
	// Chapter 13's release discipline, over the typed-clean body (the
	// last judgment this body faces).
	c.resCheck(fd.Body.Items, fd.Params, c.fnParams[fd])
	c.fnRet, c.locals, c.recvMut, c.fnBounds, c.prop = savedRet, savedLocals, savedRecv, savedBounds, savedProp
	c.bodyTags, c.bodyName, c.inBody, c.inClosure = savedTags, savedName, savedInBody, savedInClosure
}

// checkTestDecl types one test block's body (chapter 20, design D3/D7):
// a valueless driver context — the parse stage already holds the return
// discipline under the name "(test)" (E0402), and here the body walk
// raises the two chapter 20 flags: the driver sentinel (E1401 stands
// down inside, a task body inside resets to its own segment, a closure
// keeps its inference face) and the clock extent (advanceTime legal
// inside, the depth ends at closure and — with the mock face — mock
// body walls). Mock items of the body walk in T5 (design D4); until
// then the walk's statement faces alone run, and the mock goldens stay
// red exactly there.
func (c *checker) checkTestDecl(td *ast.TestDecl) {
	savedTags, savedName, savedInBody, savedInClosure, savedDriver := c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTest
	savedRet, savedProp, savedLocals, savedExtent := c.fnRet, c.prop, c.locals, c.testExtent
	c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTest = nil, "(test)", true, false, true
	// The valueless face of a full closure without a declared return: no
	// fnRet to face, the propagation context names the test, the block's
	// own locals.
	c.fnRet, c.prop = nil, &propCtx{fnName: "(test)"}
	c.testExtent = 1
	c.locals = []map[string]Type{{}}
	savedSeen := c.mockSeen
	c.mockSeen = map[string]bool{}
	// The advisory sentinel spans the whole subtree (M11 design D4):
	// unlike bodyTest, task and mock bodies inside keep it — chapter
	// 20's calls are real there. The pending list settles at the exit,
	// against the block's complete mock set.
	savedInTest, savedPend := c.advInTest, c.advPend
	c.advInTest, c.advPend = true, nil
	c.walkItems(td.Body.Items, walkFn)
	c.settleW1910()
	c.advInTest, c.advPend = savedInTest, savedPend
	c.mockSeen = savedSeen
	// Chapter 13's release discipline over the typed-clean body: a test
	// declares bindings like any statement code, and a resource binding's
	// flow obligations hold inside a test exactly as outside one.
	c.resCheck(td.Body.Items, nil, nil)
	c.fnRet, c.prop, c.locals = savedRet, savedProp, savedLocals
	c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTest = savedTags, savedName, savedInBody, savedInClosure, savedDriver
	c.testExtent = savedExtent
}

// checkMockDecl runs chapter 20's mock chain (design D4-D6), first hit
// stops: the target resolves (a bare name through the module's own
// symbols, a qualified name through the import face — E1304/E1303, the
// existing texts), the resolved symbol must be a module-level
// monomorphic fn (E1804 renders the actual category), the restated
// signature must copy the target's verbatim (E1803, three categories:
// parameter list, declared return, effect segment), one block mocks one
// target at most once (E1805, resolved identity — an alias is the same
// module, so the same target), and the body walks as a fn body of the
// target's own shape: its declared segment (E1401 judges inside), its
// return discipline, its clock-less extent (E1806's conservative read).
func (c *checker) checkMockDecl(md *ast.MockDecl) {
	display := md.Target
	if md.TargetQual != "" {
		display = md.TargetQual + "." + md.Target
	}
	var fn *ast.FnDecl
	identity := c.modKey + "." + md.Target
	if md.TargetQual == "" {
		sym, ok := c.syms[md.Target]
		if !ok {
			c.fail(md.TargetLine, md.TargetCol, "E1304", bareUnresolved(md.Target))
		}
		fn = c.mockableTarget(sym, md)
	} else {
		mod, ok := c.importQualifier(md.TargetQual)
		if !ok {
			c.fail(md.TargetLine, md.TargetCol, "E1304", bareUnresolved(md.Target))
		}
		sym := c.importSym(mod, md.Target, md.TargetLine, md.TargetCol)
		fn = c.mockableTarget(sym, md)
		identity = mod + "." + md.Target
	}
	// The restatement resolves under this module's own scope, then the
	// three categories compare (design D5): names equal as strings, types
	// sameType, the tag sequence equal in order (canonical keys both
	// sides — a qualified custom tag and its canonical target agree).
	mockParams := make([]Type, len(md.Params))
	for i, pp := range md.Params {
		mockParams[i] = c.resolveTypeRef(pp.Type, slotAnn)
	}
	mockRet := Type(nil)
	if md.HasRet {
		mockRet = c.resolveTypeRef(md.Ret, slotRet)
	}
	mockTags := c.resolveEffectTags(md.EffectTags, md.EffectLine, md.EffectCol)
	targetParams, targetTags := c.fnParams[fn], c.fnTags[fn]
	targetRet, targetHasRet := c.fnRets[fn], fn.Ret != nil
	renderParams := func(ps []ast.Param, ts []Type) string {
		parts := make([]string, len(ps))
		for i, pp := range ps {
			parts[i] = pp.Name + ": " + ts[i].String()
		}
		return strings.Join(parts, ", ")
	}
	renderTags := func(keys []string) string {
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = displayTag(k)
		}
		return strings.Join(parts, " ")
	}
	sig := func(params string, ret Type, hasRet bool, tags string) string {
		r := display + "(" + params + ")"
		if hasRet {
			r += " -> " + ret.String()
		}
		if tags != "" {
			r += " effect " + tags
		}
		return r
	}
	mockSig := sig(renderParams(md.Params, mockParams), mockRet, md.HasRet, renderTags(mockTags))
	targetSig := sig(renderParams(fn.Params, targetParams), targetRet, targetHasRet, renderTags(targetTags))
	switch {
	case len(md.Params) != len(fn.Params):
		c.fail(md.Line, md.Col, "E1803", fmt.Sprintf(
			"mock signature does not match its target — the parameter list differs: mock %s, target %s", mockSig, targetSig))
	default:
		for i, pp := range md.Params {
			// The restated names and types copy verbatim (design D5):
			// string equality per parameter name, sameType per slot,
			// position by position.
			if pp.Name != fn.Params[i].Name || !sameType(mockParams[i], targetParams[i]) {
				c.fail(md.Line, md.Col, "E1803", fmt.Sprintf(
					"mock signature does not match its target — the parameter list differs: mock %s, target %s", mockSig, targetSig))
			}
		}
	}
	if md.HasRet != targetHasRet || (md.HasRet && !sameType(mockRet, targetRet)) {
		c.fail(md.Line, md.Col, "E1803", fmt.Sprintf(
			"mock signature does not match its target — the declared return differs: mock %s, target %s", mockSig, targetSig))
	}
	if len(mockTags) != len(targetTags) {
		c.fail(md.Line, md.Col, "E1803", fmt.Sprintf(
			"mock signature does not match its target — the effect segment differs: mock %s, target %s", mockSig, targetSig))
	} else {
		for i, k := range mockTags {
			// Verbatim in order (chapter 20:34): effect io net differs
			// from effect net io — the segment is a sequence, not a set,
			// in a restatement (design D5's disclosed reading).
			if k != targetTags[i] {
				c.fail(md.Line, md.Col, "E1803", fmt.Sprintf(
					"mock signature does not match its target — the effect segment differs: mock %s, target %s", mockSig, targetSig))
			}
		}
	}
	if c.mockSeen[identity] {
		c.fail(md.Line, md.Col, "E1805", fmt.Sprintf(
			"duplicate mock of one target in a test block — %s is mocked twice in this block; one block mocks one target at most once", md.Target))
	}
	c.mockSeen[identity] = true
	// The body: a fn body of the target's own shape — the walk's context
	// swaps mirror checkFnDecl's, with the clock extent closed (E1806's
	// conservative read: the mock body answers the target's contract,
	// never the test's extent) and the target's parameters scoping the
	// names (the verbatim restatement makes the two spellings one).
	savedTags, savedName, savedInBody, savedInClosure, savedDriver := c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTest
	savedRet, savedProp, savedLocals, savedExtent, savedRecv := c.fnRet, c.prop, c.locals, c.testExtent, c.recvMut
	c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTest = targetTags, fn.Name, true, false, false
	c.fnRet, c.prop, c.recvMut = targetRet, &propCtx{fnName: fn.Name, ret: targetRet}, false
	c.testExtent = 0
	c.locals = []map[string]Type{{}}
	for i, pp := range fn.Params {
		c.locals[0][pp.Name] = targetParams[i]
	}
	c.walkItems(md.Body.Items, walkFn)
	if targetHasRet && !tailProduces(md.Body.Items) {
		c.fail(md.Line, md.Col, "E0501", fmt.Sprintf(
			"mixed types — the body produces (), the declared return is %s; no coercion is ever inserted",
			targetRet.String()))
	}
	c.resCheck(md.Body.Items, fn.Params, targetParams)
	c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTest = savedTags, savedName, savedInBody, savedInClosure, savedDriver
	c.fnRet, c.prop, c.locals, c.testExtent, c.recvMut = savedRet, savedProp, savedLocals, savedExtent, savedRecv
}

// mockableTarget is the chain's second judgment (design D4): the resolved
// symbol must be a module-level monomorphic fn — E1804 renders the actual
// category otherwise (an impl or interface method name resolves nowhere as
// a module symbol, so the reachable categories are the four below).
func (c *checker) mockableTarget(sym *symbol, md *ast.MockDecl) *ast.FnDecl {
	notMockable := func(category string) {
		c.fail(md.TargetLine, md.TargetCol, "E1804", fmt.Sprintf(
			"mock target is not a mockable function — %s is %s; only a module-level monomorphic fn is mockable", md.Target, category))
	}
	switch sym.kind {
	case symFn:
		// A foreign fn is chapter 19's other monomorphic module-level
		// category, but the boundary is not a We-side definition: no body
		// exists to substitute, the call goes to the native symbol, and a
		// mock would intercept nothing the program calls (E1804's category
		// face, design D2).
		if sym.fn.Foreign {
			notMockable("a foreign function")
		}
		if len(sym.fn.TypeParams) > 0 {
			notMockable("a generic fn")
		}
		return sym.fn
	case symVariant, symType, symRecord, symNewtype:
		notMockable("a constructor")
	case symLet:
		notMockable("a top-level binding")
	case symIface:
		notMockable("an interface")
	case symEffect:
		notMockable("an effect declaration")
	case symImport:
		notMockable("an import")
	}
	panic("unreachable mock target")
}

// tailProduces reports whether the body's final item can carry the fn's
// value: a final expression (checked against the declared return by the
// walk itself) or a valued return. A bare-return tail has already
// reported inside the walk.
func tailProduces(items []ast.Stmt) bool {
	if len(items) == 0 {
		return false
	}
	switch last := items[len(items)-1].(type) {
	case *ast.ExprStmt:
		return true
	case *ast.Return:
		return last.HasValue
	}
	return false
}

// walkMode marks a block's tail rule (design D7's ternary): a plain
// block's final expression is its value; a control form's body (while,
// loop, defer, an else-less statement if's then block) drops its tail —
// the value is never carried, so a non-unit tail is E0605's control-form
// shape; a fn body's tail faces the declared return (the valueless
// variant faces E0605's fn shape with the declare-a-return-type escape).
type walkMode int

const (
	walkPlain walkMode = iota
	walkControl
	walkFn
)

// walkItems types one block's items and returns the block's value: the
// final expression item's type, unit otherwise. The mode fixes the tail
// rule (walkMode above); if and match in statement position carry their
// own E0605 wordings, so they report before the generic expression
// statements do.
func (c *checker) walkItems(items []ast.Stmt, mode walkMode) Type {
	c.locals = append(c.locals, map[string]Type{})
	c.varScopes = append(c.varScopes, map[string]bool{})
	defer func() {
		c.locals = c.locals[:len(c.locals)-1]
		c.varScopes = c.varScopes[:len(c.varScopes)-1]
	}()
	val := Type(unitType{})
	for i, s := range items {
		last := i == len(items)-1
		switch st := s.(type) {
		case *ast.Binding:
			if st.Pat != nil {
				c.checkLetPattern(st)
				if st.Kw == "var" {
					addNames(c.varScopes[len(c.varScopes)-1], patBindNames(st.Pat))
				}
				break
			}
			t := c.checkBinding(st)
			if catOf(t) == "resource" {
				c.resLets[st] = true // the liveness pass's fact (resource.go)
			}
			if st.Name != "_" {
				c.locals[len(c.locals)-1][st.Name] = t
				if st.Kw == "var" {
					c.varScopes[len(c.varScopes)-1][st.Name] = true
				}
			}
		case *ast.Assign:
			c.checkAssign(st)
		case *ast.MockDecl:
			// Chapter 20's mock item: reachable only at a test body's own
			// depth (the parser's E1802 holds every other position), so
			// the chain needs no position gate of its own.
			c.checkMockDecl(st)
		case *ast.Return:
			c.checkReturn(st)
		case *ast.While:
			c.checkCond(st.Cond, "while condition")
			c.walkItems(st.Body.Items, walkControl)
		case *ast.Loop:
			c.walkItems(st.Body.Items, walkControl)
		case *ast.Defer:
			// A defer body runs at fn exit and carries no return of its
			// own (chapter 14): `?` there has nothing to propagate to
			// (E1202's defer shape), whatever fn encloses the defer.
			savedProp := c.prop
			c.prop = &propCtx{deferBody: true}
			c.walkItems(st.Block.Items, walkControl)
			c.prop = savedProp
		case *ast.Break, *ast.Continue:
			// Loop placement is E0201's, held at parse; nothing types here.
		case *ast.ForStmt:
			c.checkFor(st)
		case *ast.ScopeRes:
			c.checkScopeRes(st)
		case *ast.ExprStmt:
			// A fn body's tail expression checks against the declared
			// return — the expected type threads into the expression, so
			// chapter 10's determination reads it at the constructors.
			var exp Type
			if mode == walkFn && last && c.fnRet != nil {
				exp = c.fnRet
			}
			t := c.typeOf(st.Expr, exp)
			// if and match in statement position name what was dropped in
			// their own words, anchored at the keyword (the arms, the
			// arm bodies) — the generic wordings below never fire for
			// them (a stop already left this loop). A tail that carries
			// the block's value is not a statement position: a plain
			// block's final item is its value, and a fn body's faces the
			// declared return — a value-category tail there is the return
			// value, never a drop (chapter 12 makes a closure's body a
			// function body by the same rule, which is why `|a, b| if a >
			// b { a } else { b }` is a closure returning its arm).
			dropped := !last || mode == walkControl || (mode == walkFn && c.fnRet == nil)
			if dropped {
				if _, never := t.(neverType); !never && !isUnit(t) {
					switch ix := st.Expr.(type) {
					case *ast.If:
						c.fail(ix.Line, ix.Col, "E0605", fmt.Sprintf(
							"non-unit value dropped — the statement's if arms are %s, not (); bind it, chain it onward, or discard it explicitly with let _ = expr",
							t.String()))
					case *ast.Match:
						c.fail(ix.Line, ix.Col, "E0605", fmt.Sprintf(
							"non-unit value dropped — the unbound match-arm bodies are %s, not (); bind it, chain it onward, or discard it explicitly with let _ = expr",
							t.String()))
					}
				}
			}
			if _, never := t.(neverType); !never && !isUnit(t) {
				line, col := exprPos(st.Expr)
				switch {
				case mode == walkFn && last && c.fnRet == nil:
					c.fail(line, col, "E0605", fmt.Sprintf(
						"non-unit value dropped — the final expression is %s, not (); bind it, chain it onward, declare a return type, or discard it explicitly with let _ = expr",
						t.String()))
				case mode == walkControl && last:
					c.fail(line, col, "E0605", fmt.Sprintf(
						"non-unit value dropped — the final item of a control form's body is %s, not (); bind it, chain it onward, or discard it explicitly with let _ = expr",
						t.String()))
				case !(mode == walkPlain && last) && !(mode == walkFn && last):
					c.fail(line, col, "E0605", fmt.Sprintf(
						"non-unit value dropped — the statement's expression is %s, not (); bind it, chain it onward, or discard it explicitly with let _ = expr",
						t.String()))
				}
			}
			if last {
				val = t
				if mode == walkFn && c.fnRet != nil && !agree(t, c.fnRet) {
					// A bare `?` tail hands its payload Ok-wrapped to the
					// caller (chapter 14's scenario: the closure `fn(x) ->
					// Result<...> { parse(x)? }` is legal) — the unwrapped
					// type faces the declared return's Ok slot; any other
					// tail faces the full declared return.
					want := c.fnRet
					if _, isProp := st.Expr.(*ast.Prop); isProp {
						if rt, ok := c.fnRet.(namedType); ok && rt.decl == resultSum {
							want = rt.args[0]
						}
					}
					if !agree(t, want) {
						line, col := exprPos(st.Expr)
						c.fail(line, col, "E0501", fmt.Sprintf(
							"mixed types — the final expression is %s, the declared return is %s; no coercion is ever inserted",
							t.String(), c.fnRet.String()))
					}
				}
			}
		default:
			// A statement form this stage does not type yet (the boundary
			// table's last rows) stops here, never silently.
			c.bnd(bndCtlForms)
		}
	}
	return val
}

// checkCond types one condition position (chapter 3/4: if, while, match
// guard — one rule, one code, the site in the message), anchored at the
// condition expression's first token.
func (c *checker) checkCond(e ast.Expr, site string) {
	t := c.typeOf(e, nil)
	if agree(t, baseType("Bool")) {
		return
	}
	line, col := exprPos(e)
	c.fail(line, col, "E0503", fmt.Sprintf(
		"condition is not Bool — the %s is %s; condition positions are if, while, and match guard",
		site, t.String()))
}

// checkIf types one if (design D7): the condition is a Bool position
// (E0503); with an else, the arm blocks' values agree (E0501 at the if)
// and the agreed type is the if's type — the inhabited side of a Never
// arm; an else-less if has no value (the parser holds value positions to
// E0202), and its then block is a control body whose dropped tail the
// walk reports.
func (c *checker) checkIf(x *ast.If) Type {
	c.checkCond(x.Cond, "if condition")
	if x.Else == nil {
		c.walkItems(x.Then.Items, walkControl)
		return unitType{}
	}
	tt := c.walkItems(x.Then.Items, walkPlain)
	et := c.typeOf(x.Else, nil)
	if !agree(tt, et) {
		c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
			"mixed types — the if arms are %s and %s; no coercion is ever inserted",
			tt.String(), et.String()))
	}
	if _, never := tt.(neverType); never {
		return et
	}
	return tt
}

// checkMatch types one match (design D8): the patterns type against the
// scrutinee (binds, E0303/E0304, or-pattern agreement E0501), each guard
// is a condition position, each arm body types in a scope holding the
// binds, the bodies agree (E0501 at the match), and the unguarded arms
// alone must cover every value of the scrutinee's type — usefulness, not
// arm counting, decides (E0305/E0306/E0307).
func (c *checker) checkMatch(x *ast.Match) Type {
	scrut := c.typeOf(x.Scrutinee, nil)
	var res Type
	var prevRows [][]ast.Pattern // earlier UNGUARDED arms' expanded rows
	anyGuard := false
	for i, arm := range x.Arms {
		binds := map[string]Type{}
		nodes := map[string]ast.Pattern{}
		c.checkPattern(arm.Pat, scrut, binds, nodes)
		rows := expandOr(patRows(arm.Pat))
		if arm.Guard == nil {
			// A statically dead arm is rejected before its guard-less
			// body types: every branch of its pattern is already covered.
			dead := true
			for _, q := range rows {
				if c.useful(prevRows, q, []Type{scrut}) {
					dead = false
					break
				}
			}
			if dead && len(rows) > 0 {
				c.fail(arm.Line, arm.Col, "E0307", fmt.Sprintf(
					"unreachable match arm — every value matching this arm's pattern is already covered by earlier arms; statically decidable dead arms are rejected"))
			}
			prevRows = append(prevRows, rows...)
		} else {
			anyGuard = true
		}
		c.locals = append(c.locals, binds)
		if arm.Guard != nil {
			c.checkCond(arm.Guard, "match guard")
		}
		bt := c.typeOf(arm.Body, nil)
		c.locals = c.locals[:len(c.locals)-1]
		if i == 0 {
			res = bt
			continue
		}
		if !agree(bt, res) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — one match arm is %s, the others are %s; no coercion is ever inserted",
				bt.String(), res.String()))
		}
		if _, never := res.(neverType); never {
			res = bt
		}
	}
	if w := c.missing(prevRows, []Type{scrut}); w != nil {
		if anyGuard {
			c.fail(x.Line, x.Col, "E0306", fmt.Sprintf(
				"guarded match without an unguarded exhaustive fallback — the unguarded arms alone do not cover %s; guards refine arms but never prove coverage",
				w.top()))
		}
		switch w.vec[0].kind {
		case witVariant:
			c.fail(x.Line, x.Col, "E0305", fmt.Sprintf(
				"match is not exhaustive — the variant %q of %q is not covered; the unguarded arms must match every value of the scrutinee's type",
				w.vec[0].name, w.vec[0].sum))
		case witAny:
			c.fail(x.Line, x.Col, "E0305", fmt.Sprintf(
				"match is not exhaustive — %q does not enumerate its values and no wildcard arm stands; base types are never exhausted by literals",
				scrut.String()))
		default:
			c.fail(x.Line, x.Col, "E0305", fmt.Sprintf(
				"match is not exhaustive — the value shape %q is not covered; the unguarded arms must match every value of the scrutinee's type",
				w.vec[0].render()))
		}
	}
	if _, never := res.(neverType); never {
		return res
	}
	return res
}

// checkPattern types one pattern against the scrutinee's type, binding
// each name at its position's type. nodes records where each name was
// bound — the or-pattern disagreement anchors at the later branch's
// binding token. Guards never reach here (they are expressions).
func (c *checker) checkPattern(p ast.Pattern, scrut Type, binds map[string]Type, nodes map[string]ast.Pattern) {
	switch x := p.(type) {
	case *ast.PatLiteral:
		lt := c.patternLiteralType(x)
		if !agree(lt, scrut) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — the literal pattern is %s, the matched type is %s; no coercion is ever inserted",
				lt.String(), scrut.String()))
		}
	case *ast.PatWildcard:
	case *ast.PatBinding:
		binds[x.Name] = scrut
		nodes[x.Name] = x
	case *ast.PatTuple:
		tt, ok := scrut.(tupleType)
		if !ok || len(tt.elems) != len(x.Elems) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — the tuple pattern is matched against %s, which is not a tuple of the same arity; no coercion is ever inserted",
				scrut.String()))
		}
		for i, e := range x.Elems {
			c.checkPattern(e, tt.elems[i], binds, nodes)
		}
	case *ast.PatVariant:
		if x.Qualified {
			// the qualified form reaches the variant through the import's
			// bucket — the pub gate (chapter 15), then the same scrutinee
			// matching the bare form takes
			if mod, is := c.importQualifier(x.Qual); is {
				c.importSym(mod, x.Name, x.Line, x.Col)
			} else {
				c.fail(x.Line, x.Col, "E1304", fmt.Sprintf(
					"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
					x.Qual, x.Qual+"."+x.Name))
			}
		}
		nt, ok := scrut.(namedType)
		if !ok {
			c.fail(x.Line, x.Col, "E0303", fmt.Sprintf(
				"pattern names no variant of the scrutinee's type — the scrutinee is %q, which declares no variants; variant patterns need a sum-typed scrutinee",
				scrut.String()))
		}
		var v *variantInfo
		for i := range nt.decl.variants {
			if nt.decl.variants[i].name == x.Name {
				v = &nt.decl.variants[i]
				break
			}
		}
		if v == nil {
			c.fail(x.Line, x.Col, "E0303", fmt.Sprintf(
				"pattern names no variant of the scrutinee's type — %q is not a variant of %q; variant patterns resolve lexically against the declaration, with no nearest-match search",
				x.Name, nt.decl.name))
		}
		if len(x.Args) != len(v.payloads) {
			c.fail(x.Line, x.Col, "E0304", fmt.Sprintf(
				"variant pattern payload arity mismatch — the pattern holds %d sub-patterns, the variant %q declares %d; payload elements bind position-wise",
				len(x.Args), x.Name, len(v.payloads)))
		}
		for i, a := range x.Args {
			// A payload slot the declaration writes as a clause position
			// binds at the scrutinee's application of it — the pattern's
			// binding types read the scrutinee, not the declaration (a
			// generic sum's payload positions substituted with its
			// arguments; the fix M9a's std.concurrent matches surfaced).
			pt := v.payloads[i]
			if len(nt.args) > 0 {
				pt = subst(pt, nt.args, nil)
			}
			c.checkPattern(a, pt, binds, nodes)
		}
	case *ast.PatOr:
		// Branch 1 fixes the name set; a later branch binding one name at
		// a disagreeing type is E0501 at that branch's binding token.
		var base map[string]Type
		for i, br := range x.Branches {
			b := map[string]Type{}
			n := map[string]ast.Pattern{}
			c.checkPattern(br, scrut, b, n)
			if i == 0 {
				base = b
				continue
			}
			for _, name := range sortedNames(b) {
				ht, has := base[name]
				if has && !agree(ht, b[name]) {
					bp := n[name]
					line, col := patAnchor(bp)
					c.fail(line, col, "E0501", fmt.Sprintf(
						"mixed types — the branches of one or-pattern bind %q as %s and %s; no coercion is ever inserted",
						name, ht.String(), b[name].String()))
				}
			}
		}
		for name, t := range base {
			binds[name] = t
		}
	}
}

// patternLiteralType reads one pattern literal's type the way the
// expression side's literalType does (suffixes, defaults), without the
// range check — a literal in pattern position checks agreement, not the
// fold pipeline.
func (c *checker) patternLiteralType(x *ast.PatLiteral) Type {
	switch x.Kind {
	case "int":
		t, _ := intLiteral(x.Text)
		return t
	case "float":
		for _, suf := range []string{"f32", "f64"} {
			if strings.HasSuffix(x.Text, suf) && !hexPrefixed(x.Text) {
				return baseType(map[string]string{"f32": "Float32", "f64": "Float64"}[suf])
			}
		}
		return baseType("Float64")
	case "string":
		return baseType("String")
	case "rune":
		return baseType("Rune")
	case "bool":
		return baseType("Bool")
	}
	panic("unreachable pattern literal")
}

// sortedNames gives a map's names in declaration-independent stable
// order (map iteration order must not pick the reported branch).
func sortedNames(m map[string]Type) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// --- usefulness (design D8: Maranget's I-protocol, guard-free) ----------------

// patRows expands one pattern into its matrix rows: an or-pattern is one
// row per branch (its branches' name sets agree — E0302 held at parse).
func patRows(p ast.Pattern) [][]ast.Pattern {
	if or, is := p.(*ast.PatOr); is {
		var out [][]ast.Pattern
		for _, br := range or.Branches {
			out = append(out, patRows(br)...)
		}
		return out
	}
	return [][]ast.Pattern{{p}}
}

// expandOr splits every row still holding an or-pattern cell into one
// row per branch, repeatedly, until no cell is an or-pattern.
func expandOr(rows [][]ast.Pattern) [][]ast.Pattern {
	for {
		var out [][]ast.Pattern
		split := false
		for _, r := range rows {
			at, or := -1, (*ast.PatOr)(nil)
			for i, cell := range r {
				if o, is := cell.(*ast.PatOr); is {
					at, or = i, o
					break
				}
			}
			if at < 0 {
				out = append(out, r)
				continue
			}
			split = true
			for _, br := range or.Branches {
				nr := make([]ast.Pattern, 0, len(r))
				nr = append(nr, r[:at]...)
				nr = append(nr, br)
				nr = append(nr, r[at+1:]...)
				out = append(out, nr)
			}
		}
		rows = out
		if !split {
			return rows
		}
	}
}

// ctorKey identifies a row head's constructor: a variant name, the tuple
// constructor, or a literal's own value (literals are their own 0-ary
// constructors over infinite domains).
type ctorKey struct {
	tuple   bool
	variant string
	litKind string
	litText string
}

func headKey(p ast.Pattern) (ctorKey, bool) {
	switch x := p.(type) {
	case *ast.PatVariant:
		return ctorKey{variant: x.Name}, true
	case *ast.PatTuple:
		return ctorKey{tuple: true}, true
	case *ast.PatLiteral:
		return ctorKey{litKind: x.Kind, litText: x.Text}, true
	}
	return ctorKey{}, false // a wildcard or binding: no constructor
}

// specialize keeps the rows a constructor argument vector could still
// match: rows headed by that constructor carry the head's sub-patterns,
// wildcard-headed rows carry fresh wildcards, others drop out.
func specialize(rows [][]ast.Pattern, k ctorKey, arity int) [][]ast.Pattern {
	var out [][]ast.Pattern
	for _, r := range rows {
		hk, has := headKey(r[0])
		if !has {
			out = append(out, append(wildRow(arity), r[1:]...))
			continue
		}
		if hk == k {
			var cells []ast.Pattern
			switch h := r[0].(type) {
			case *ast.PatVariant:
				cells = h.Args
			case *ast.PatTuple:
				cells = h.Elems
			}
			out = append(out, append(append([]ast.Pattern{}, cells...), r[1:]...))
		}
	}
	return expandOr(out)
}

// wildRow is arity wildcards.
func wildRow(n int) []ast.Pattern {
	row := make([]ast.Pattern, n)
	for i := range row {
		row[i] = &ast.PatWildcard{}
	}
	return row
}

// defaultRows keeps wildcard-headed rows only — the constructor set of
// the dropped rows covers nothing on an infinite domain.
func defaultRows(rows [][]ast.Pattern) [][]ast.Pattern {
	var out [][]ast.Pattern
	for _, r := range rows {
		if _, has := headKey(r[0]); !has {
			out = append(out, r[1:])
		}
	}
	return out
}

// finiteCtors reports a type's constructor set when it is finite: a sum
// declares its variants, a tuple type is one k-ary constructor. Base,
// unit, fn, record, and newtype types have infinite domains — no finite
// enumeration, and no Bool special case (design D8).
func finiteCtors(t Type) ([]ctorKey, [][]Type, bool) {
	switch s := t.(type) {
	case namedType:
		// A sum declaring no variants is not a finite empty domain — no
		// legal sum declares zero (the parser holds that), so this shape
		// is the checker's opaque one: the collections' method-only sums
		// and the std.concurrent named sums before their module ingests.
		// Reading it as empty would judge every pattern under it dead
		// (E0307's false face); it is an unknown domain like any other.
		if len(s.decl.variants) == 0 {
			return nil, nil, false
		}
		keys := make([]ctorKey, len(s.decl.variants))
		payloads := make([][]Type, len(s.decl.variants))
		for i, v := range s.decl.variants {
			keys[i] = ctorKey{variant: v.name}
			// The payload slots an application carries are its arguments'
			// — the usefulness matrix specializes through the applied
			// types (a clause position here would read as an opaque
			// domain and misjudge nested constructor arms).
			if len(s.args) > 0 {
				payloads[i] = substArgs(v.payloads, s.args, nil)
				continue
			}
			payloads[i] = v.payloads
		}
		return keys, payloads, true
	case tupleType:
		elems := make([]Type, len(s.elems))
		copy(elems, s.elems)
		return []ctorKey{{tuple: true}}, [][]Type{elems}, true
	}
	return nil, nil, false
}

// useful reports whether the query row q can still match a value no
// earlier row matches (Maranget's usefulness): a wildcard query probes
// every finite constructor (and the default matrix on infinite domains),
// a constructor query specializes both sides.
func (c *checker) useful(rows [][]ast.Pattern, q []ast.Pattern, types []Type) bool {
	if len(q) == 0 {
		return len(rows) == 0
	}
	if hk, has := headKey(q[0]); has {
		var args []ast.Pattern
		rest := q[1:]
		switch h := q[0].(type) {
		case *ast.PatVariant:
			args = h.Args
		case *ast.PatTuple:
			args = h.Elems
		}
		pts := c.payloadTypes(hk, types[0])
		if len(pts) == 0 && len(args) > 0 {
			return false // unreachable: E0304 held the arity first
		}
		return c.useful(specialize(rows, hk, len(args)), append(append([]ast.Pattern{}, args...), rest...),
			append(append([]Type{}, pts...), types[1:]...))
	}
	if keys, payloads, finite := finiteCtors(types[0]); finite {
		for i, k := range keys {
			if c.useful(specialize(rows, k, len(payloads[i])), wildRow(len(payloads[i])), payloads[i]) {
				return true
			}
		}
		return false
	}
	return c.useful(defaultRows(rows), q[1:], types[1:])
}

// payloadTypes resolves a constructor's argument types at the scrutinee
// position's type (nil for a literal — the checker held its agreement).
func (c *checker) payloadTypes(k ctorKey, t Type) []Type {
	if keys, payloads, finite := finiteCtors(t); finite {
		for i, key := range keys {
			if key == k {
				return payloads[i]
			}
		}
	}
	return nil
}

// witness is the uncovered value finding: a vector of shapes for the
// probed columns, rendered for E0305/E0306's messages.
type witness struct {
	vec []*witShape
}

type witShape struct {
	kind int // witVariant | witTuple | witAny
	name string
	sum  string
	subs []*witShape
}

const (
	witVariant = iota
	witTuple
	witAny
)

func (w *witShape) render() string {
	switch w.kind {
	case witVariant:
		if len(w.subs) > 0 {
			return w.name + "(" + witJoin(w.subs) + ")"
		}
		return w.name
	case witTuple:
		return "(" + witJoin(w.subs) + ")"
	default:
		return "_"
	}
}

func witJoin(ws []*witShape) string {
	parts := make([]string, len(ws))
	for i, w := range ws {
		parts[i] = w.render()
	}
	return strings.Join(parts, ", ")
}

// top renders the finding for E0306's message: a bare variant names its
// sum; anything else renders its shape.
func (w *witness) top() string {
	first := w.vec[0]
	if first.kind == witVariant {
		return fmt.Sprintf("%q of %q", first.name, first.sum)
	}
	return fmt.Sprintf("%q", first.render())
}

// missing searches the uncovered value shapes no row covers (nil = the
// rows are exhaustive): the exhaustiveness side of usefulness, walking
// the same specialization lattice with the witness carried back up. The
// returned vector covers the probed types in order — a constructor head
// summarizes its argument shapes, sibling columns pass through.
func (c *checker) missing(rows [][]ast.Pattern, types []Type) *witness {
	if len(types) == 0 {
		if len(rows) == 0 {
			return &witness{vec: []*witShape{}} // the empty vector stands
		}
		return nil
	}
	if keys, payloads, finite := finiteCtors(types[0]); finite {
		rest := types[1:]
		for i, k := range keys {
			m := len(payloads[i])
			sub := c.missing(specialize(rows, k, m),
				append(append([]Type{}, payloads[i]...), rest...))
			if sub != nil {
				var head *witShape
				if k.tuple {
					head = &witShape{kind: witTuple, subs: sub.vec[:m]}
				} else {
					head = &witShape{kind: witVariant, name: k.variant, sum: ctorSum(types[0])}
				}
				return &witness{vec: append([]*witShape{head}, sub.vec[m:]...)}
			}
		}
		return nil
	}
	if sub := c.missing(defaultRows(rows), types[1:]); sub != nil {
		return &witness{vec: append([]*witShape{{kind: witAny}}, sub.vec...)}
	}
	return nil
}

// ctorSum names the sum a variant constructor belongs to (the tuple
// constructor names none).
func ctorSum(t Type) string {
	if n, ok := t.(namedType); ok {
		return n.decl.name
	}
	return ""
}

// --- expressions -------------------------------------------------------------

// exprPos returns an expression's first-token position: binary nodes
// anchor at their operator, so the leftmost operand is walked down;
// calls and members walk to their heads.
func exprPos(e ast.Expr) (int, int) {
	switch x := e.(type) {
	case *ast.Binary:
		return exprPos(x.L)
	case *ast.Call:
		return exprPos(x.Fn)
	case *ast.Member:
		return exprPos(x.Recv)
	case *ast.Prop:
		return exprPos(x.X)
	case *ast.Ident:
		return x.Line, x.Col
	case *ast.Literal:
		return x.Line, x.Col
	case *ast.Unary:
		return x.Line, x.Col
	case *ast.BlockExpr:
		return x.Line, x.Col
	case *ast.Unit:
		return x.Line, x.Col
	case *ast.Construct:
		return x.Line, x.Col
	case *ast.Tuple:
		return x.Line, x.Col
	case *ast.ListLit:
		return x.Line, x.Col
	case *ast.Closure:
		return x.Line, x.Col
	case *ast.If:
		return x.Line, x.Col
	case *ast.Match:
		return x.Line, x.Col
	}
	return 1, 1
}

// nodePos returns a node's own recorded position (a binary node's is its
// operator — the fold anchor; exprPos walks to the first token instead).
func nodePos(e ast.Expr) (int, int) {
	switch x := e.(type) {
	case *ast.Binary:
		return x.Line, x.Col
	case *ast.Call:
		return x.Line, x.Col
	case *ast.Member:
		return x.Line, x.Col
	case *ast.Ident:
		return x.Line, x.Col
	case *ast.Literal:
		return x.Line, x.Col
	case *ast.Unary:
		return x.Line, x.Col
	case *ast.BlockExpr:
		return x.Line, x.Col
	case *ast.Unit:
		return x.Line, x.Col
	}
	return 1, 1
}

// typeOf types one expression; expected is the threading type where the
// position carries one (binding annotation, assignment target, argument
// parameter, return declaration, constructor payload), nil otherwise.
func (c *checker) typeOf(e ast.Expr, expected Type) Type {
	switch x := e.(type) {
	case *ast.Literal:
		return c.literalType(x)
	case *ast.Unit:
		return unitType{}
	case *ast.Ident:
		return c.identType(x, expected)
	case *ast.Unary:
		if x.Op == "-" && constInt(x) {
			return c.constType(x)
		}
		return c.unaryType(x)
	case *ast.Binary:
		if x.Op == ".." {
			return c.rangeType(x)
		}
		if foldOp(x.Op) && constInt(x) {
			return c.constType(x)
		}
		return c.binaryType(x)
	case *ast.Call:
		return c.callType(x, expected)
	case *ast.Member:
		return c.memberType(x, false)
	case *ast.BlockExpr:
		// A block's value is its final expression item; expected types do
		// not thread into blocks.
		return c.walkItems(x.Block.Items, walkPlain)
	case *ast.If:
		return c.checkIf(x)
	case *ast.Match:
		return c.checkMatch(x)
	case *ast.Construct:
		return c.constructType(x)
	case *ast.Tuple:
		elems := make([]Type, len(x.Elems))
		for i, e := range x.Elems {
			elems[i] = c.typeOf(e, nil)
		}
		return tupleType{elems: elems}
	case *ast.Closure:
		return c.closureType(x, expected)
	case *ast.ListLit:
		return c.listLitType(x, expected)
	case *ast.Prop:
		return c.propType(x)
	case *ast.TaskExpr:
		return c.taskType(x)
	case *ast.ScopeExpr:
		return c.scopeType(x)
	case *ast.SelectExpr:
		return c.selectType(x)
	}
	panic("unreachable expr")
}

// taskType types one task block (chapter 18, design D6): the creation must
// sit lexically inside a compound scope's body (E1618 — a handle with no
// scope to join it), the body walks as a function-body context under the
// task's OWN declared effect segment — the body's calls answer the task,
// never the enclosing declaration, and the creation itself performs no
// calls, only the capture copies — and the value is the handle
// TaskHandle<T>, T the body's block value (unit for a valueless body;
// every early exit agrees with the tail, and a resource T never travels
// into a handle — E1106, a composite position).
func (c *checker) taskType(x *ast.TaskExpr) Type {
	if c.scopeDepth == 0 {
		c.fail(x.Line, x.Col, "E1618", "task block outside any scope block — no enclosing scope block joins the handle, and no task outlives its scope; open a scope block around the task creation, or move the work into an ordinary function call")
	}
	savedTags, savedName, savedInBody, savedInClosure, savedTask, savedDriver := c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTask, c.bodyTest
	savedRet, savedProp, savedVal, savedIn := c.fnRet, c.prop, c.taskVal, c.inTaskBody
	// A task extent answers the task (chapter 18), never the test that
	// hosts it: the body judges against its own declared segment, so the
	// test's driver sentinel stands down for the walk (design D3's reset).
	c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTask, c.bodyTest = c.resolveEffectTags(x.EffectTags, x.EffectLine, x.EffectCol), "(task)", true, false, true, false
	// A task body carries its value early through return like a closure's
	// (chapter 18's function-body context): the fn's own return machinery
	// stands down, the exits' join takes over. The `?` suffix has no
	// declaration to face either — the task's value is inferred, never a
	// Result contract (E1202's no-context shape).
	c.fnRet, c.prop = nil, &propCtx{fnName: "(task)"}
	c.taskVal, c.inTaskBody = nil, true
	// The capture pre-pass runs before the body walk (design D4): a
	// capture diagnostic precedes any the body's own typing would raise.
	c.checkTaskCaptures(x)
	c.taskDepth++
	t := c.walkItems(x.Body.Items, walkPlain)
	c.taskDepth--
	c.inTaskBody = savedIn
	// Join the tail with the early exits: the first exit fixes the
	// candidate, the tail and every later exit must agree with it (the fn
	// machinery's own discipline — a bare return exits with unit, and the
	// if-return pair's block value is () against a valued exit, exactly
	// as a declared fn's body judges it today).
	if last, ok := lastExpr(x.Body.Items); ok && c.taskVal != nil && !agree(c.taskVal, t) {
		c.fail(last.Line, last.Col, "E0501", fmt.Sprintf(
			"mixed types — the final expression is %s, the task block's value is %s; no coercion is ever inserted",
			t.String(), c.taskVal.String()))
	}
	if c.taskVal != nil {
		t = c.taskVal
	}
	c.taskVal = savedVal
	if catOf(t) == "resource" {
		line, col := x.Line, x.Col
		if last, ok := lastExpr(x.Body.Items); ok {
			line, col = exprPos(last.Expr)
		}
		c.fail(line, col, "E1106", fmt.Sprintf(
			"resource type in a composite position — %q appears as a task block's value; a resource type appears only in binding positions - a parameter, a let or scope-head binding, a return type",
			t.String()))
	}
	c.fnRet, c.prop = savedRet, savedProp
	c.bodyTags, c.bodyName, c.inBody, c.inClosure, c.bodyTask, c.bodyTest = savedTags, savedName, savedInBody, savedInClosure, savedTask, savedDriver
	return namedType{decl: taskHandleSum, args: []Type{t}}
}

// lastExpr reports the block's final item when it is an expression
// statement (the block-value tail), with the statement's own position.
func lastExpr(items []ast.Stmt) (*ast.ExprStmt, bool) {
	if len(items) == 0 {
		return nil, false
	}
	es, ok := items[len(items)-1].(*ast.ExprStmt)
	return es, ok
}

// advanceTimeOutside is E1806's message (design D7): the virtual clock is
// bound to test blocks (chapter 20), so the value face and the call face
// report the same text outside the test extent.
const advanceTimeOutside = "advanceTime called outside a test block — the virtual clock is bound to test blocks; advance the clock inside the test that observes it"

// outsideTaskMsg renders E1608's message: where the call sits — the
// enclosing body's own declaration, or the module top level — with the
// static rule and the registry's remediation.
func outsideTaskMsg(bodyName string) string {
	sits := "the call sits at the module top level"
	if bodyName != "" {
		sits = fmt.Sprintf("the call sits in %q", bodyName)
	}
	return fmt.Sprintf(
		"currentCancelSignal called outside a task block — %s, where no current task exists and no default signal does, and the check is static; take a CancelSignal parameter when a caller wants to cancel the work, or move the call inside the task block that should answer cancellation",
		sits)
}

// taskReturn types one return inside a task body: the body's value type is
// inferred from its exits — a valued return contributes its type, a bare
// one exits with unit — and the exits must agree among themselves before
// the tail joins (the fn machinery's mirror, E0501 at the disagreement).
func (c *checker) taskReturn(r *ast.Return) {
	vt := Type(unitType{})
	if r.HasValue {
		vt = c.typeOf(r.Value, nil)
	}
	if c.taskVal == nil {
		c.taskVal = vt
		return
	}
	if !agree(c.taskVal, vt) {
		if r.HasValue {
			line, col := exprPos(r.Value)
			c.fail(line, col, "E0501", fmt.Sprintf(
				"mixed types — the return expression is %s, the task block's value is %s; no coercion is ever inserted",
				vt.String(), c.taskVal.String()))
		}
		c.fail(r.Line, r.Col, "E0501", fmt.Sprintf(
			"mixed types — the return expression is (), the task block's value is %s; no coercion is ever inserted",
			c.taskVal.String()))
	}
}

// scopeType types one compound scope block (chapter 18, design D7): the
// timeout clause types Int64 (E0501 at the clause expression), the body
// walks in a new scope frame — the depth the E1618 gate reads, so a task
// may be created inside — and the value is the body's block value, the
// timeout forms wrapping it in Result<T, TimeoutError> through the
// synthetic module's own declaration.
func (c *checker) scopeType(x *ast.ScopeExpr) Type {
	if x.Timeout != nil {
		tt := c.typeOf(x.Timeout, baseType("Int64"))
		if !agree(tt, baseType("Int64")) {
			line, col := exprPos(x.Timeout)
			c.fail(line, col, "E0501", fmt.Sprintf(
				"mixed types — the timeout expression is %s, the clause wants Int64; no coercion is ever inserted",
				tt.String()))
		}
	}
	c.scopeDepth++
	val := c.walkItems(x.Body.Items, walkPlain)
	c.scopeDepth--
	// The handle discipline runs after the body typed clean (the chapter 13
	// release walk's position): every handle the body created must take its
	// one fire on every path that is not an early exit (design D5).
	c.handleCheck(x.Body.Items)
	if x.Timeout != nil {
		return namedType{decl: resultSum, args: []Type{val, namedType{decl: c.stdConcSum("TimeoutError")}}}
	}
	return val
}

// selectType types one select expression (chapter 18, design D7): each
// case's source types first on the existing member-call road, then must
// sit in the four-call wait-source closed set (E1609); the case's binding
// takes the wait's own value shape (Channel/ReceiveOnly receive → the
// element Option, TaskHandle await → the body Result, CancelSignal
// awaitCancelled → unit); the arms must agree in type (E1610), and the
// select's value is the taken case's body value.
func (c *checker) selectType(x *ast.SelectExpr) Type {
	var val Type
	for i, cs := range x.Cases {
		c.typeOf(cs.Source, nil)
		bind := c.waitSourceBind(cs)
		c.locals = append(c.locals, map[string]Type{})
		if !cs.Wildcard && cs.Name != "_" {
			c.locals[len(c.locals)-1][cs.Name] = bind
		}
		bt := c.typeOf(cs.Body, nil)
		c.locals = c.locals[:len(c.locals)-1]
		if i == 0 {
			val = bt
		} else if !agree(bt, val) {
			c.fail(cs.Line, cs.Col, "E1610", fmt.Sprintf(
				"select arms disagree in type — the arms produce %q and %q, and the select expression's value is the taken case's body value; give the arms one type, or bind what differs and unify in a following expression",
				val.String(), bt.String()))
		}
	}
	return val
}

// waitSourceBind judges one select case's source against the closed
// wait-source set (E1609) and returns the case binding's own value shape.
func (c *checker) waitSourceBind(cs ast.SelectCase) Type {
	if call, ok := cs.Source.(*ast.Call); ok {
		if m, ok := call.Fn.(*ast.Member); ok {
			if nt, isN := c.typeOf(m.Recv, nil).(namedType); isN {
				switch m.Name {
				case "receive":
					if nt.decl == channelSum || nt.decl == receiveOnlySum {
						return namedType{decl: optionSum, args: []Type{nt.args[0]}}
					}
				case "await":
					if nt.decl == taskHandleSum {
						return namedType{decl: resultSum, args: []Type{
							nt.args[0], namedType{decl: c.stdConcSum("TaskPanic")}}}
					}
				case "awaitCancelled":
					if nt.decl == cancelSignalSum {
						return unitType{}
					}
				}
			}
		}
	}
	line, col := exprPos(cs.Source)
	c.fail(line, col, "E1609", "select source is not a wait source — the source expression is not one of Channel.receive(), ReceiveOnly.receive(), TaskHandle.await(), CancelSignal.awaitCancelled() and nothing else; use one of the four wait-source calls as the case's source, or call the non-waiting expression outside the select")
	return nil
}

// propType types the propagation postfix `expr?` (chapter 14, design D7).
// The operand must be the canonical Result (E1201 — Option gets its own
// note, it is the shape a reader most likely wanted); the position must
// hold a live context — the innermost enclosing body's declared Result
// return, with a defer body and the module top level carrying none (E1202,
// naming the enclosing function); and the operand's error type must be the
// declared one by identity (E1203 — no implicit wrapping or lifting). The
// value is the Ok payload: on Err the context function returns early
// through its own return, the resource obligations travelling as with any
// return (chapter 13). A short closure's `?` is the one inferring context:
// the operands fix the error type and closureType wraps the value type.
func (c *checker) propType(x *ast.Prop) Type {
	t := c.typeOf(x.X, nil)
	nt, ok := t.(namedType)
	if !ok || nt.decl != resultSum {
		note := "; only Result values propagate, an Option unwraps by match"
		if ok && nt.decl == optionSum {
			note = " and Option does not propagate; match the value to take its payload"
		}
		c.fail(x.Line, x.Col, "E1201", fmt.Sprintf(
			"operand of ? is not a Result type — the operand is %q%s", t.String(), note))
	}
	p := c.prop
	if p.deferBody {
		c.fail(x.Line, x.Col, "E1202", "? outside a function returning Result — the propagation stands in a defer body, which runs at exit and has no return to propagate to; handle the value by match at this position")
	}
	if p.short {
		if p.shortErr == nil {
			p.shortErr = nt.args[1]
		} else if !sameType(nt.args[1], p.shortErr) {
			c.fail(x.Line, x.Col, "E1203", fmt.Sprintf(
				"? error type disagrees with the declared error type — %q is not the declared error type %q; there is no implicit wrapping or lifting, error conversion is explicit",
				nt.args[1].String(), p.shortErr.String()))
		}
		return nt.args[0]
	}
	if p.fnName == "" {
		c.fail(x.Line, x.Col, "E1202", "? outside a function returning Result — the propagation stands at the module top level, where no return exists to propagate to; move it into a function or closure whose return type is Result")
	}
	ret := p.ret
	if ret == nil {
		ret = unitType{}
	}
	dst, ok := ret.(namedType)
	if !ok || dst.decl != resultSum {
		c.fail(x.Line, x.Col, "E1202", fmt.Sprintf(
			"? outside a function returning Result — the innermost enclosing function %q returns %q, not a Result; move the propagation into a function or closure whose return type is Result, or handle the value by match at this position",
			p.fnName, ret.String()))
	}
	if !sameType(nt.args[1], dst.args[1]) {
		c.fail(x.Line, x.Col, "E1203", fmt.Sprintf(
			"? error type disagrees with the declared error type — %q is not the declared error type %q; there is no implicit wrapping or lifting, error conversion is explicit",
			nt.args[1].String(), dst.args[1].String()))
	}
	return nt.args[0]
}

// closureType types one closure (design D9): fully annotated parameters
// are self-sufficient; a bare parameter takes its type from the expected
// function type's position (E1001 without one — the expected return
// never flows back, the body carries the value). The body types in fn
// context — the closure's own return stack (E0402's shapes at parse),
// deferred items legal at its top level — with the declared return
// holding the full form's tail and the short form's single-expression
// body carrying its own value. The capture ledger records every name use
// crossing the boundary (gc live, value snapshot, resource rejected).
func (c *checker) closureType(x *ast.Closure, expected Type) Type {
	bare := false
	for _, p := range x.Params {
		if p.Type == nil {
			bare = true
			break
		}
	}
	var params []Type
	if !bare {
		for _, p := range x.Params {
			params = append(params, c.resolveTypeRef(p.Type, slotAnn))
		}
	} else {
		et, ok := expected.(fnType)
		if !ok {
			c.fail(x.Line, x.Col, "E1001", fmt.Sprintf(
				"bare-parameter closure without an expected function type — the parameters are bare and no explicit function type is expected at this binding; annotate the parameters or bind under a function-type annotation"))
		}
		if len(x.Params) != len(et.params) {
			c.bnd(bndArityGap)
		}
		for i, p := range x.Params {
			if p.Type == nil {
				params = append(params, et.params[i])
				continue
			}
			pt := c.resolveTypeRef(p.Type, slotAnn)
			// A symbolic expected position (a method-generic clause
			// position the call will determine) cannot disagree — the
			// annotation itself is one candidate for the determination.
			if !containsParam(et.params[i]) && !agree(pt, et.params[i]) {
				c.fail(p.NameLine, p.NameCol, "E0501", fmt.Sprintf(
					"mixed types — the parameter is %s, the expected position carries %s; no coercion is ever inserted",
					pt.String(), et.params[i].String()))
			}
			params = append(params, pt)
		}
	}
	c.closures = append(c.closures, map[string]captureInfo{})
	c.closureBounds = append(c.closureBounds, len(c.locals))
	c.closureTags = append(c.closureTags, nil)
	defer func() {
		c.closures = c.closures[:len(c.closures)-1]
		c.closureBounds = c.closureBounds[:len(c.closureBounds)-1]
		c.closureTags = c.closureTags[:len(c.closureTags)-1]
	}()
	savedRet, savedLocals, savedProp := c.fnRet, c.locals, c.prop
	c.locals = append(c.locals, map[string]Type{})
	for i, p := range x.Params {
		if p.Name != "_" {
			c.locals[len(c.locals)-1][p.Name] = params[i]
		}
	}
	// The closure body suspends chapter 16's call judgment (design D6):
	// constructing the closure is not performing it, and the body's calls
	// join the closure's own inferred set — the judgment returns when the
	// closure value is called through its fn type.
	savedInClosure := c.inClosure
	c.inClosure = true
	// A closure's returns answer the closure's own fnRet (swapped in
	// below), never the task join of a task body the closure sits in —
	// the exit-value machine stands down inside the closure body. The
	// task's lexical depth (E1608's gate) is deliberately untouched:
	// containment reads through closures.
	savedInTask, savedTaskVal := c.inTaskBody, c.taskVal
	c.inTaskBody = false
	// The clock's extent ends at the closure wall (design D7's literal
	// reading of chapter 20:102): a closure body is a function body of
	// its own (chapter 12), not the test's extent — advanceTime inside
	// one is E1806.
	savedExtent := c.testExtent
	c.testExtent = 0
	ret := Type(unitType{})
	switch {
	case x.Ret != nil:
		c.fnRet = c.resolveTypeRef(x.Ret, slotRet)
		// The closure is its own propagation context (chapter 14): a full
		// closure's `?`s face its declared return, never the function the
		// closure sits in.
		c.prop = &propCtx{fnName: "(closure)", ret: c.fnRet}
		c.walkItems(x.Body.Items, walkFn)
		if !tailProduces(x.Body.Items) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — the body produces (), the declared return is %s; no coercion is ever inserted",
				c.fnRet.String()))
		}
		ret = c.fnRet
	case x.Short:
		// the normalized single-expression body is the value; no declared
		// return exists to face (chapter 12: the return type comes from
		// the body)
		c.fnRet = nil
		// A short closure's `?` is the one inferring context (chapter 14):
		// the operands fix one error type, and the closure's value type
		// wraps to Result over the body's type — the binding the closure
		// reaches is legal because the closure is the `?`'s innermost
		// function.
		c.prop = &propCtx{fnName: "(closure)", short: true}
		ret = c.walkItems(x.Body.Items, walkPlain)
		if c.prop.shortErr != nil {
			ret = namedType{decl: resultSum, args: []Type{ret, c.prop.shortErr}}
		}
	default:
		c.fnRet = nil
		c.prop = &propCtx{fnName: "(closure)"}
		c.walkItems(x.Body.Items, walkFn)
	}
	// Chapter 13's release discipline over the typed-clean body: a closure
	// is a fn body for the bindings it declares (its parameters among
	// them — a resource-typed closure parameter is the transfer channel's
	// callee side).
	c.resCheck(x.Body.Items, x.Params, params)
	c.fnRet, c.locals, c.prop = savedRet, savedLocals, savedProp
	c.inClosure = savedInClosure
	c.inTaskBody, c.taskVal = savedInTask, savedTaskVal
	c.testExtent = savedExtent
	// The inference's settled set is the closure value's own segment
	// (chapter 16 R4): every face of the value from here — the woven
	// agreement at a slot, the call judgment when it is called — weighs
	// it like a declared segment.
	return fnType{params: params, tags: c.closureTags[len(c.closureTags)-1], ret: ret}
}

// constructType types a record construction or update (design D6): the
// head names a record (E0603 otherwise), a resource head rejects updates
// outright (E0606 — the category is a declaration fact, so it precedes the
// field checks), the update base carries exactly the head's record type
// (E0603 at the base's first token), and the field set matches the
// declaration — undeclared names, duplicates, and (for a full
// construction) missing fields are E0604; each field value checks against
// its declared type (E0501, anchored at the field name).
func (c *checker) constructType(x *ast.Construct) Type {
	var sym *symbol
	if x.Qual != "" {
		// a qualified head constructs another module's record through the
		// pub gate (chapter 15); a non-import qualifier is E1304's shape
		if mod, is := c.importQualifier(x.Qual); is {
			sym = c.importSym(mod, x.Name, x.Line, x.Col)
		} else {
			c.fail(x.Line, x.Col, "E1304", fmt.Sprintf(
				"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
				x.Qual, x.Qual+"."+x.Name))
		}
	} else {
		var ok bool
		sym, ok = c.syms[x.Name]
		if !ok {
			if baseNames[x.Name] {
				c.fail(x.Line, x.Col, "E0603", fmt.Sprintf(
					"construction or update head or base is not the record type — the head %q names a base type, not a record; construction and update heads name record types",
					x.Name))
			}
			c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
		}
	}
	if sym.kind != symRecord {
		what := "a value name"
		switch sym.kind {
		case symType:
			what = "a sum type"
		case symNewtype:
			what = "a newtype"
		}
		c.fail(x.Line, x.Col, "E0603", fmt.Sprintf(
			"construction or update head or base is not the record type — the head %q names %s, not a record; construction and update heads name record types",
			x.Name, what))
	}
	rec := sym.rec
	// Chapter 19's opaque discipline precedes every field judgment
	// (E1707): a zero-field record satisfies chapter 8's
	// all-fields-exactly-once vacuously, so an unrestricted Socket { }
	// would mint a handle from nothing — the native side mints, We code
	// only receives. Construction and update share the head, so the one
	// check holds both (ahead of E0606's update face and E0604's fields).
	if rec.opaque {
		c.fail(x.Line, x.Col, "E1707", fmt.Sprintf(
			"construction or update of a foreign opaque type — %s is minted on the native side only; obtain the value from a foreign function's declared return",
			rec.name))
	}
	// The generic clause instantiates the declaration (chapter 10): the
	// arity judgment first (E0828), the honesty re-checks at the
	// application (E0601/E0823), then the field checks against the
	// substituted declared types. A bare generic head determines from the
	// construction itself (design D6): an update's base names the
	// application directly, a full construction's field values unify
	// against the declared shapes.
	if len(x.TypeArgs) > 0 && len(x.TypeArgs) != len(rec.params) {
		c.arityFail(x.ArgLine, x.ArgCol, x.Line, x.Col, rec.name, len(rec.params), len(x.TypeArgs))
	}
	var args []Type
	if len(x.TypeArgs) > 0 {
		args = c.resolveArgs(x.TypeArgs, slotGeneric)
	}
	if len(rec.params) > 0 && len(args) == 0 {
		args = c.inferConstructArgs(rec, x)
	}
	rt := recordType{decl: rec, args: args}
	if args != nil {
		c.checkImplWhereAtConstruct(rec, args, x)
		c.checkFieldInstantiation(rec, typeArgAnchors(x.TypeArgs, args, x.Line, x.Col), args, rt)
	}
	if x.Base != nil {
		// The category check precedes the field checks (E0606's fact is
		// the declaration's, not the update's).
		if rec.cat == "resource" {
			c.fail(x.Line, x.Col, "E0606", fmt.Sprintf(
				"update expression on a resource record — the head %q names a byres record; a resource has identity, and copying its fields would duplicate the handle",
				x.Name))
		}
		bt := c.typeOf(x.Base, rt)
		if !agree(bt, rt) {
			line, col := exprPos(x.Base)
			c.fail(line, col, "E0603", fmt.Sprintf(
				"construction or update head or base is not the record type — the update base is %q, the head names %q; an update base has exactly the head's record type",
				bt.String(), rec.name))
		}
	}
	declared := map[string]Type{}
	for _, f := range rec.fields {
		declared[f.name] = subst(f.typ, args, nil)
	}
	seen := map[string]bool{}
	for _, f := range x.Fields {
		dt, has := declared[f.Name]
		if !has {
			c.fail(f.Line, f.Col, "E0604", fmt.Sprintf(
				"field set does not match the record — %q declares no field %q; a construction or update names only declared fields",
				rec.name, f.Name))
		}
		if seen[f.Name] {
			c.fail(f.Line, f.Col, "E0604", fmt.Sprintf(
				"field set does not match the record — %q names the field %q twice; a full construction names every declared field exactly once",
				rec.name, f.Name))
		}
		seen[f.Name] = true
		vt := c.typeOf(f.Value, dt)
		if !agree(vt, dt) && !c.checkFnSlot(vt, dt, f.Line, f.Col) {
			c.fail(f.Line, f.Col, "E0501", fmt.Sprintf(
				"mixed types — the field %q is %s, the declared field is %s; no coercion is ever inserted",
				f.Name, vt.String(), dt.String()))
		}
	}
	if x.Base == nil {
		for _, f := range rec.fields {
			if !seen[f.name] {
				c.fail(x.Line, x.Col, "E0604", fmt.Sprintf(
					"field set does not match the record — the construction of %q leaves out %q; a full construction names every declared field exactly once",
					rec.name, f.name))
			}
		}
	}
	return rt
}

// checkImplWhereAtConstruct holds a generic impl's where bounds at a
// construction of its head (E0830's construction position, design D10(c)):
// the head's application determines the impl's clause positions, and each
// bound holds at every instantiation — whatever the site does with the
// impl's interface. The clause is the impl's declaration fact; the anchor
// is the construction head.
func (c *checker) checkImplWhereAtConstruct(rec *recordInfo, args []Type, x *ast.Construct) {
	for _, im := range c.impls {
		if !im.generic {
			continue
		}
		h, ok := im.head.(recordType)
		if !ok || h.decl != rec || len(h.args) != len(args) {
			continue
		}
		w := c.implWheres[im.decl]
		if w == nil {
			continue
		}
		// The impl head's argument positions bind the impl's clause —
		// a parameter position in the head receives the construction's
		// argument at the same position.
		binds := map[int]Type{}
		for i, ha := range h.args {
			if p, is := ha.(paramRef); is {
				binds[p.idx] = args[i]
			}
		}
		for _, b := range w.bounds {
			subject, bound := binds[b.idx]
			if !bound {
				continue
			}
			// the Shareable bound's instantiation judgment is the same
			// shape rule the fn-application site reads (design D3)
			if b.face.decl == shareableMarker {
				if c.shareableBoundArg(subject) {
					continue
				}
			} else if c.implementsFace(subject, b.face) {
				continue
			}
			c.fail(x.Line, x.Col, "E0830", fmt.Sprintf(
				"type argument does not satisfy a where bound — %q implements no %q; the where bound of the impl for %q holds at every instantiation",
				subject.String(), b.face.String(), im.headStr))
		}
	}
}

// checkFieldInstantiation re-checks a record's honesty at an application:
// a value record's field categories (E0601) and a derives clause's
// capability (E0823) hold at every instantiation, anchored at the
// argument that materialized the failing position (an inferred
// application anchors at the construction head — the determination is the
// head's).
func (c *checker) checkFieldInstantiation(rec *recordInfo, anchors [][2]int, args []Type, rt recordType) {
	for _, f := range rec.fields {
		pr, isRef := f.typ.(paramRef)
		if !isRef || pr.idx >= len(args) {
			continue
		}
		sub := args[pr.idx]
		if containsParam(sub) {
			continue
		}
		if rec.cat == "value" && catOf(sub) != "value" {
			line, col := anchors[pr.idx][0], anchors[pr.idx][1]
			c.fail(line, col, "E0601", fmt.Sprintf(
				"value record field is not of the value category or a base type — %q instantiates the field %q with %q, a %s; a copy is only honest when everything in it is copyable by value",
				rt.String(), f.name, sub.String(), catNoun(sub)))
		}
		for _, target := range rec.derives {
			if carriesType(sub, target) {
				continue
			}
			line, col := anchors[pr.idx][0], anchors[pr.idx][1]
			c.fail(line, col, "E0823", fmt.Sprintf(
				"derive field requirement unmet — %q instantiates %q with %q, which carries no %q; the clause is checked at each instantiation",
				rt.String(), pr.name, sub.String(), target))
		}
	}
}

// typeArgAnchors builds an instantiation's anchors: an explicit clause
// anchors each argument at its own token, an inferred application at the
// application's head (the determination is the head's — no argument token
// exists).
func typeArgAnchors(refs []ast.TypeRef, args []Type, line, col int) [][2]int {
	anchors := make([][2]int, len(args))
	for i := range anchors {
		if i < len(refs) {
			l, cc := refPos(refs[i])
			anchors[i] = [2]int{l, cc}
			continue
		}
		anchors[i] = [2]int{line, col}
	}
	return anchors
}

// inferConstructArgs determines a bare generic construction head's
// application (design D6): an update's base names the application
// directly — the head and the base name one declaration, so the base's
// arguments are the head's; a full construction's field values unify
// against the declared shapes, with the full-set judgment (E0604) first —
// a missing field is the field set's fact, not an undetermined position.
func (c *checker) inferConstructArgs(rec *recordInfo, x *ast.Construct) []Type {
	if x.Base != nil {
		bt := c.typeOf(x.Base, nil)
		br, ok := bt.(recordType)
		if !ok || br.decl != rec || len(br.args) != len(rec.params) {
			line, col := exprPos(x.Base)
			c.fail(line, col, "E0603", fmt.Sprintf(
				"construction or update head or base is not the record type — the update base is %q, the head names %q; an update base has exactly the head's record type",
				bt.String(), rec.name))
		}
		return br.args
	}
	byName := map[string]ast.Expr{}
	seen := map[string]bool{}
	for _, f := range x.Fields {
		byName[f.Name] = f.Value
		seen[f.Name] = true
	}
	for _, f := range rec.fields {
		if !seen[f.name] {
			c.fail(x.Line, x.Col, "E0604", fmt.Sprintf(
				"field set does not match the record — the construction of %q leaves out %q; a full construction names every declared field exactly once",
				rec.name, f.name))
		}
	}
	shapes := make([]Type, len(rec.fields))
	values := make([]ast.Expr, len(rec.fields))
	for i, f := range rec.fields {
		shapes[i] = f.typ
		values[i] = byName[f.name]
	}
	return c.inferValueArgs(shapes, values, len(rec.params), func() {
		c.fail(x.Line, x.Col, "E0827", fmt.Sprintf(
			"generic call does not determine its type arguments — the construction of %q determines no type argument; a construction with no determining field value takes the explicit form %s<Int64> { ... }",
			rec.name, rec.name))
	})
}

// literalType types one literal: integer literals carry their suffix or
// default to Int64 with the range check, floats their suffix or Float64.
func (c *checker) literalType(x *ast.Literal) Type {
	switch x.Kind {
	case "int":
		t, v := intLiteral(x.Text)
		if !inRange(v, t) {
			c.fail(x.Line, x.Col, "E0502", fmt.Sprintf(
				"integer overflow — the constant expression overflows %s: %s; widen the type (suffix or annotation) or restructure the computation",
				t.String(), v.String()))
		}
		return t
	case "float":
		for _, suf := range []string{"f32", "f64"} {
			if strings.HasSuffix(x.Text, suf) && !hexPrefixed(x.Text) {
				return baseType(map[string]string{"f32": "Float32", "f64": "Float64"}[suf])
			}
		}
		return baseType("Float64")
	case "string":
		return baseType("String")
	case "rune":
		return baseType("Rune")
	case "bool":
		return baseType("Bool")
	}
	panic("unreachable literal")
}

// intLiteral decodes one integer literal into its type and value: the
// i/u suffix names the type (Int64 without one); underscores separate
// digit groups; 0x/0b/0o carry the base.
func intLiteral(text string) (Type, *big.Int) {
	t := "Int64"
	valText := text
	for suf, name := range map[string]string{
		"i8": "Int8", "i16": "Int16", "i32": "Int32", "i64": "Int64",
		"u8": "UInt8", "u16": "UInt16", "u32": "UInt32", "u64": "UInt64",
	} {
		if strings.HasSuffix(text, suf) && (len(text) > len(suf) && isDigitOrHexSafe(text[:len(text)-len(suf)])) {
			t = name
			valText = text[:len(text)-len(suf)]
			break
		}
	}
	v, ok := new(big.Int).SetString(stripUnderscores(valText), 0)
	if !ok {
		// The lexer has validated the literal's shape; hex letters can
		// only pair with a base prefix.
		v = new(big.Int)
	}
	return baseType(t), v
}

// isDigitOrHexSafe reports whether a suffix-stripped prefix still holds
// a literal (at least one character the lexer could have accepted).
func isDigitOrHexSafe(s string) bool {
	return s != "" && s != "0x" && s != "0b" && s != "0o" && s != "0X" && s != "0B" && s != "0O"
}

func stripUnderscores(s string) string {
	return strings.ReplaceAll(s, "_", "")
}

func hexPrefixed(s string) bool {
	return strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") ||
		strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") ||
		strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O")
}

// unaryType types one unary operator with the domain rules: ! wants Bool,
// - wants a numeric, ~ wants an integer; violations anchor at the
// operator token.
func (c *checker) unaryType(x *ast.Unary) Type {
	t := c.typeOf(x.X, nil)
	switch x.Op {
	case "!":
		if t != Type(baseType("Bool")) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — operand of %q is %s; no coercion is ever inserted", x.Op, t.String()))
		}
	case "-":
		if !isNumeric(t) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — operand of %q is %s; no coercion is ever inserted", x.Op, t.String()))
		}
	case "~":
		if !isInt(t) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — operand of %q is %s; no coercion is ever inserted", x.Op, t.String()))
		}
	}
	return t
}

// binaryType types one binary operator with the domain table (design
// D4): disagreeing operands are E0501 at the operator; a same-type pair
// beyond the ratified domains stops at the spec-gap boundary.
func (c *checker) binaryType(x *ast.Binary) Type {
	lt := c.typeOf(x.L, nil)
	rt := c.typeOf(x.R, nil)
	if !sameType(lt, rt) {
		c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
			"mixed types — operands of %q are %s and %s; no coercion is ever inserted",
			x.Op, lt.String(), rt.String()))
	}
	switch x.Op {
	case "+", "-", "*", "/", "%":
		if isString(lt) {
			// Chapter 10 approves String equality ("`==` compares base
			// types only", and String is one); concatenation is the `+`
			// reading of the same pair, and chapter 7's silence on a
			// concatenation operator is a gap the reference build closes
			// (design D3). Every other operator on Strings — the
			// arithmetic siblings, ordering — stays at the boundary.
			if x.Op != "+" {
				c.bnd(bndDomainGap)
			}
			return lt
		}
		if !isNumeric(lt) {
			c.bnd(bndDomainGap)
		}
		return lt
	case "<", "<=", ">", ">=":
		if !isNumeric(lt) {
			c.bnd(bndDomainGap)
		}
		return baseType("Bool")
	case "==", "!=":
		return baseType("Bool")
	case "&&", "||":
		if lt != Type(baseType("Bool")) {
			c.bnd(bndDomainGap)
		}
		return baseType("Bool")
	case "&", "^", "|", "<<", ">>":
		if !isInt(lt) {
			c.bnd(bndDomainGap)
		}
		return lt
	}
	panic("unreachable op")
}

// foldOp reports whether op joins the constant fold set.
func foldOp(op string) bool {
	switch op {
	case "+", "-", "*", "/", "%", "<<", ">>", "&", "^", "|":
		return true
	}
	return false
}

// constInt reports whether e is a constant integer tree (design D5's
// foldable set: integer literals, unary minus, the fold-set operators).
func constInt(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Literal:
		return x.Kind == "int"
	case *ast.Unary:
		return x.Op == "-" && constInt(x.X)
	case *ast.Binary:
		return foldOp(x.Op) && constInt(x.L) && constInt(x.R)
	}
	return false
}

// constType types a constant integer tree: evaluation runs in
// arbitrary-precision signed arithmetic and ONE range check follows,
// against the expression's own type, anchored at the operator that
// produced the out-of-range value (a bare literal anchors at itself).
func (c *checker) constType(e ast.Expr) Type {
	res := c.foldTree(e)
	if res.val != nil && !inRange(res.val, res.t) {
		line, col := nodePos(e)
		if res.anchor != nil {
			line, col = nodePos(res.anchor)
		}
		detail := res.val.String()
		if _, isBinary := e.(*ast.Binary); isBinary {
			detail = res.render + " = " + res.val.String()
		}
		c.fail(line, col, "E0502", fmt.Sprintf(
			"integer overflow — the constant expression overflows %s: %s; widen the type (suffix or annotation) or restructure the computation",
			res.t.String(), detail))
	}
	return res.t
}

// foldResult carries one folded subtree: its type, value (nil when a
// division by zero or a negative shift stopped evaluation), the rendered
// operand form, and the innermost operator whose result left the range.
type foldResult struct {
	t      Type
	val    *big.Int
	render string
	anchor ast.Expr
}

func (c *checker) foldTree(e ast.Expr) foldResult {
	switch x := e.(type) {
	case *ast.Literal:
		t, v := intLiteral(x.Text)
		return foldResult{t: t, val: v, render: v.String()}
	case *ast.Unary: // "-" (constInt guarantees the operand)
		r := c.foldTree(x.X)
		v := new(big.Int).Neg(r.val)
		out := foldResult{t: r.t, val: v, render: v.String(), anchor: r.anchor}
		if out.anchor == nil && !inRange(v, out.t) {
			out.anchor = x
		}
		return out
	case *ast.Binary:
		l := c.foldTree(x.L)
		r := c.foldTree(x.R)
		if !sameType(l.t, r.t) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — operands of %q are %s and %s; no coercion is ever inserted",
				x.Op, l.t.String(), r.t.String()))
		}
		v, ok := foldApply(x.Op, l.val, r.val)
		if !ok {
			// a constant division or remainder by zero is not checked
			// (design D5): the type stands, the value does not
			return foldResult{t: l.t}
		}
		out := foldResult{
			t:      l.t,
			val:    v,
			render: l.render + " " + x.Op + " " + r.render,
			anchor: l.anchor,
		}
		if out.anchor == nil {
			out.anchor = r.anchor
		}
		if out.anchor == nil && !inRange(v, out.t) {
			out.anchor = x
		}
		return out
	}
	panic("unreachable fold")
}

// foldApply applies one fold-set operator in big arithmetic; ok is false
// when evaluation is undefined (zero divisor, negative shift).
func foldApply(op string, a, b *big.Int) (*big.Int, bool) {
	switch op {
	case "+":
		return new(big.Int).Add(a, b), true
	case "-":
		return new(big.Int).Sub(a, b), true
	case "*":
		return new(big.Int).Mul(a, b), true
	case "/":
		if b.Sign() == 0 {
			return nil, false
		}
		return new(big.Int).Quo(a, b), true
	case "%":
		if b.Sign() == 0 {
			return nil, false
		}
		return new(big.Int).Rem(a, b), true
	case "<<":
		if b.Sign() < 0 || !b.IsInt64() || b.Int64() > 1<<20 {
			return nil, false
		}
		return new(big.Int).Lsh(a, uint(b.Int64())), true
	case ">>":
		if b.Sign() < 0 || !b.IsInt64() {
			return nil, false
		}
		if b.Int64() > 1<<20 {
			return new(big.Int), true
		}
		return new(big.Int).Rsh(a, uint(b.Int64())), true
	case "&":
		return new(big.Int).And(a, b), true
	case "|":
		return new(big.Int).Or(a, b), true
	case "^":
		return new(big.Int).Xor(a, b), true
	}
	return nil, false
}

// --- names in value positions --------------------------------------------------

// lookupLocal walks the local scope chain innermost-first.
func (c *checker) lookupLocal(name string) (Type, bool) {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if t, ok := c.locals[i][name]; ok {
			return t, true
		}
	}
	return nil, false
}

// lookupLocalDepth is lookupLocal reporting the layer the name settled
// in — the capture ledger's crossing test needs it.
func (c *checker) lookupLocalDepth(name string) (Type, int, bool) {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if t, ok := c.locals[i][name]; ok {
			return t, i, true
		}
	}
	return nil, -1, false
}

// noteCapture records one name use crossing closure boundaries into every
// crossed ledger layer (a pass-through capture belongs to each outer
// closure holding the path), and rejects a resource capture at the use
// (E1002 — a handle may not escape its single deterministic release
// point). layer -1 marks the module namespace, below every boundary.
func (c *checker) noteCapture(name string, t Type, layer int, line, col int) {
	if len(c.closures) == 0 {
		return
	}
	cat := catOf(t)
	for i := range c.closures {
		if layer < c.closureBounds[i] {
			if _, has := c.closures[i][name]; !has {
				c.closures[i][name] = captureInfo{cat: cat, line: line, col: col}
			}
		}
	}
	if cat == "resource" {
		c.fail(line, col, "E1002", fmt.Sprintf(
			"resource binding captured by a closure — the closure reads %q, a byres binding; a handle escaping the single deterministic release point is not honest",
			name))
	}
}

// noteAssignCapture records one assignment whose target crosses a closure
// boundary: a gc capture is live (the write reaches the shared cell), a
// value capture is a copy frozen at creation (E1003), a resource rejects
// as at its read (E1002). Assignments crossing no boundary keep M3's
// behavior — chapters 2/8 draw no mutability rule yet (follow-up 7).
func (c *checker) noteAssignCapture(name string, t Type, layer int, line, col int) {
	if len(c.closures) == 0 {
		return
	}
	crossed := false
	cat := catOf(t)
	for i := range c.closures {
		if layer >= c.closureBounds[i] {
			continue
		}
		crossed = true
		if _, has := c.closures[i][name]; !has {
			c.closures[i][name] = captureInfo{cat: cat, line: line, col: col}
		}
	}
	if !crossed {
		return
	}
	switch cat {
	case "value":
		c.fail(line, col, "E1003", fmt.Sprintf(
			"assignment to a captured value-category binding — %q is captured as a copy fixed at closure creation and the snapshot is frozen; mutations belong to gc bindings",
			name))
	case "resource":
		c.fail(line, col, "E1002", fmt.Sprintf(
			"resource binding captured by a closure — the closure reads %q, a byres binding; a handle escaping the single deterministic release point is not honest",
			name))
	}
}

// lookupValue resolves a name to a value type across the layers.
func (c *checker) lookupValue(name string) (Type, bool) {
	if t, ok := c.lookupLocal(name); ok {
		return t, true
	}
	if sym, ok := c.syms[name]; ok && sym.kind == symLet {
		if sym.letType == nil {
			// a forward reference to a module binding whose initializer
			// is not yet typed (module bindings reference earlier ones)
			return nil, false
		}
		return sym.letType, true
	}
	return nil, false
}

// identType types one bare identifier in value position.
func (c *checker) identType(x *ast.Ident, expected Type) Type {
	if t, layer, ok := c.lookupLocalDepth(x.Name); ok {
		c.noteCapture(x.Name, t, layer, x.Line, x.Col)
		return t
	}
	if sym, ok := c.syms[x.Name]; ok {
		switch sym.kind {
		case symLet:
			if sym.letType == nil {
				c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
			}
			c.noteCapture(x.Name, sym.letType, -1, x.Line, x.Col)
			return sym.letType
		case symVariant:
			return c.variantType(sym.sum, sym.vi, x.Line, x.Col, x.Name)
		case symFn:
			// A fn's bare name in value position is its fn type (chapter
			// 12) — a generic fn's name is a family, not one function, and
			// a bare call of it waits for the determination pass instead.
			if len(sym.fn.TypeParams) > 0 {
				c.fail(x.Line, x.Col, "E1004", fmt.Sprintf(
					"generic function name in value position — %q is a family of functions, not one function; call it or wrap it: |x| %s(x)",
					x.Name, x.Name))
			}
			ret := Type(unitType{})
			if r, has := c.fnRets[sym.fn]; has {
				ret = r
			}
			return fnType{params: c.fnParams[sym.fn], tags: c.fnTags[sym.fn], ret: ret}
		default:
			// type and import names carry no value
			c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
		}
	}
	switch x.Name {
	case "panic", "todo", "assert":
		// The panic family's prelude signatures (chapter 14): ordinary
		// functions, the bare name its fn type like any fn's.
		return terminationFnType(x.Name)
	case "currentCancelSignal":
		// Chapter 18's accessor as a value: the 0-ary fn type, legal
		// only lexically inside a task body (E1608 elsewhere — the
		// containment reads through closures, never the join machine).
		if c.taskDepth == 0 {
			c.fail(x.Line, x.Col, "E1608", outsideTaskMsg(c.bodyName))
		}
		return fnType{ret: namedType{decl: cancelSignalSum}}
	case "advanceTime":
		// Chapter 20's clock control (design D7): an ordinary
		// (Int64) -> () value position-gated to the test extent — the
		// clock is not a value that leaves the test (E1806 otherwise),
		// and no effect segment exists to declare (the clock is a
		// runtime control token, chapter 20:102).
		if c.testExtent == 0 {
			c.fail(x.Line, x.Col, "E1806", advanceTimeOutside)
		}
		return fnType{params: []Type{baseType("Int64")}, ret: unitType{}}
	case "None":
		if nt, ok := expected.(namedType); ok && nt.decl == optionSum {
			return nt
		}
		c.fail(x.Line, x.Col, "E0827", undetermined("None", "Option"))
	case "Ok", "Err", "Some":
		c.fail(x.Line, x.Col, "E0704", fmt.Sprintf(
			"payloaded variant constructor used without arguments — %q carries a payload; construct with the call form %s(...)",
			x.Name, x.Name))
	}
	c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
	panic("unreachable ident")
}

func undetermined(name, sum string) string {
	return fmt.Sprintf(
		"generic call does not determine its type arguments — %q does not determine the type arguments of %s; annotate the position with the full application",
		name, sum)
}

// variantType types a bare user variant reference: a unit variant is its
// sum's value; a payloaded constructor used without arguments is E0704.
func (c *checker) variantType(sum *sumInfo, vi int, line, col int, name string) Type {
	v := sum.variants[vi]
	if len(v.payloads) > 0 {
		c.fail(line, col, "E0704", fmt.Sprintf(
			"payloaded variant constructor used without arguments — %q carries a payload; construct with the call form %s(...)",
			name, name))
	}
	return namedType{decl: sum}
}

// memberType types one member access (design D3's member sets, design
// D4's four-way resolution, design D6's substitution): a record resolves
// by its fields then methods, a newtype by its one member "value" then
// methods, a sum by its methods — every view substituted at the
// application; an interface's own view and the Dyn box resolve the
// carried face's method set alone (E0815 on a miss); the base types
// carry their builtin members, a member no anchored family names staying
// behind the std-modules boundary; and a generic parameter without a
// where bound is opaque (E0817 — a bounded one's granted set is the
// where pass's). asCall marks a call head — the one position a method
// view fits (chapter 12): a bare method name in any other value
// production is E0105. forEach exists nowhere — sequence effects are the
// for statement's (E0816). An unresolvable bare receiver identifier
// reads as a failed qualifier (E1304 at the receiver).
func (c *checker) memberType(x *ast.Member, asCall bool) Type {
	t, _ := c.memberTypeRecv(x, asCall)
	return t
}

// memberTypeRecv is memberType carrying the receiver's own type back:
// the call path's determination reads the positions the receiver holds,
// and the receiver types once — here, not twice (its side effects —
// captures, effect sets — walk a single time either way).
func (c *checker) memberTypeRecv(x *ast.Member, asCall bool) (Type, Type) {
	if id, ok := x.Recv.(*ast.Ident); ok {
		// An import name in the receiver is the qualified form's head
		// (chapter 15): the item resolves through the target's bucket,
		// never through the import name's own (valueless) type.
		if mod, is := c.importQualifier(id.Name); is {
			return c.importMember(mod, id, x), nil
		}
		if !c.nameResolvable(id.Name) {
			full := id.Name + "." + x.Name
			c.fail(id.Line, id.Col, "E1304", fmt.Sprintf(
				"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
				id.Name, full))
		}
	}
	if x.Name == "forEach" {
		c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
			"no such member on the receiver's type — the receiver's type has no member %q; forEach does not exist: sequence effects belong to the for statement",
			x.Name))
	}
	rt := c.typeOf(x.Recv, nil)
	return c.memberOfType(rt, x, asCall), rt
}

// memberOfType resolves the member against one receiver type: the four
// source sets of design D4 (the nominal declarations, the interface
// position, the Dyn face, the builtins), with a bounded parameter reading
// its bounds' granted method sets and an associated position its equality
// binding (design D6). A method view still holding clause positions flows
// to its call, which determines them from the arguments.
func (c *checker) memberOfType(t Type, x *ast.Member, asCall bool) Type {
	switch t := t.(type) {
	case recordType:
		for _, f := range t.decl.fields {
			if f.name == x.Name {
				return subst(f.typ, t.args, nil)
			}
		}
		for i := range t.decl.methods {
			m := &t.decl.methods[i]
			if m.name != x.Name {
				continue
			}
			// Chapter 13's single release trigger: user code never names
			// release on a resource receiver — the impl's own body
			// included — the early spelling is an early scope exit (E1104,
			// at the receiver's first token).
			if asCall && x.Name == "release" && t.decl.cat == "resource" {
				line, col := exprPos(x.Recv)
				c.fail(line, col, "E1104", fmt.Sprintf(
					"resource binding outside its release discipline — %q calls release directly; release has one trigger, the scope-exit machinery, and the spelling for early release is an early scope exit",
					recvText(x.Recv)+".release()"))
			}
			c.methodUse(x, asCall)
			return substFn(m.fn, t.args, nil)
		}
		c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
			"no such member on the receiver's type — %q has no member %q; member access names a field or a method of the receiver's type",
			t.decl.name, x.Name))
	case newtypeType:
		if x.Name == "value" {
			return subst(t.decl.underlying, t.args, nil)
		}
		for i := range t.decl.methods {
			m := &t.decl.methods[i]
			if m.name != x.Name {
				continue
			}
			c.methodUse(x, asCall)
			return substFn(m.fn, t.args, nil)
		}
		c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
			"no such member on the receiver's type — %q has no member %q; a newtype's single member is \"value\"",
			t.decl.name, x.Name))
	case namedType:
		for i := range t.decl.methods {
			m := &t.decl.methods[i]
			if m.name != x.Name {
				continue
			}
			c.methodUse(x, asCall)
			return substFn(m.fn, t.args, nil)
		}
		if collectionSum(t.decl) {
			// the spec-anchored families resolve; the rest of the
			// collection surface is the standard library's (design D10 —
			// never privately rejected)
			if view, ok := collectionMembers(t)[x.Name]; ok {
				c.methodUse(x, asCall)
				return view
			}
			c.bnd(bndStdModules)
		}
		if concurrentType(t.decl) {
			// the chapter 18 faces are the language's own closed sets
			// (design D2): a member no family names is a miss (E0816),
			// never the stdlib boundary. The view substitutes the
			// receiver's arguments into the clause positions — RwLock's
			// read keeps its own clause position (U) for the call's text
			// to determine.
			if view, ok := c.concurrentMembers(t)[x.Name]; ok {
				c.methodUse(x, asCall)
				return substFn(view, t.args, nil)
			}
			c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
				"no such member on the receiver's type — %q has no member %q; member access names a field or a method of the receiver's type",
				t.decl.name, x.Name))
		}
		c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
			"no such member on the receiver's type — %q has no member %q; member access names a field or a method of the receiver's type",
			t.decl.name, x.Name))
	case ifaceType:
		for i := range t.decl.methods {
			im := &t.decl.methods[i]
			if im.name != x.Name {
				continue
			}
			c.methodUse(x, asCall)
			return substFn(fnType{params: im.params, tags: im.tags, ret: retOrUnit(im.ret)}, t.args, nil)
		}
		c.fail(x.NameLine, x.NameCol, "E0815", fmt.Sprintf(
			"method call outside the receiver's method set — %q is outside the method set of %q; an interface position reaches its own interface's methods only",
			x.Name, t.String()))
	case dynType:
		for i := range t.inf.methods {
			im := &t.inf.methods[i]
			if im.name != x.Name {
				continue
			}
			c.methodUse(x, asCall)
			return substFn(fnType{params: im.params, tags: im.tags, ret: retOrUnit(im.ret)}, t.args, nil)
		}
		c.fail(x.NameLine, x.NameCol, "E0815", fmt.Sprintf(
			"method call outside the receiver's method set — %q is outside the method set of %q; a Dyn value reaches its own interface's methods only",
			x.Name, t.String()))
	case baseType:
		if t == "String" {
			if view, ok := stringMembers[x.Name]; ok {
				c.methodUse(x, asCall)
				return view
			}
		}
		// the stdlib surface (the conversion methods, the byte views) —
		// a member no anchored family names is never privately rejected
		// (design D10)
		c.bnd(bndStdModules)
	case paramRef:
		// A bounded parameter reaches its bounds' granted method sets
		// (design D6's union — the face's application substitutes the
		// bound interface's clause positions); an unbounded parameter is
		// opaque (E0817).
		if c.fnBounds != nil {
			bounded := false
			for _, b := range c.fnBounds.bounds {
				if b.idx != t.idx {
					continue
				}
				bounded = true
				for i := range b.face.decl.methods {
					im := &b.face.decl.methods[i]
					if im.name != x.Name {
						continue
					}
					c.methodUse(x, asCall)
					return substFn(fnType{params: im.params, tags: im.tags, ret: retOrUnit(im.ret)}, b.face.args, nil)
				}
			}
			if bounded {
				c.fail(x.NameLine, x.NameCol, "E0815", fmt.Sprintf(
					"method call outside the receiver's method set — %q is outside the method set of %q; a where-bounded parameter reaches its bounds' method sets only",
					x.Name, t.String()))
			}
		}
		c.fail(x.NameLine, x.NameCol, "E0817", fmt.Sprintf(
			"method call on an unconstrained generic parameter — the parameter %q carries no where bound, and an unconstrained parameter is opaque; name a where bound granting the method set",
			t.name))
	case tupleType, fnType, unitType, neverType:
		c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
			"no such member on the receiver's type — %s has no member %q; member access names a field or a method of the receiver's type",
			t.String(), x.Name))
	case assocRef:
		// An associated position's member set is its equality binding's
		// (design D6): the where clause binds the position to a concrete
		// type and the member reads through it; an unbound position
		// reaches nothing.
		if c.fnBounds != nil {
			for _, eq := range c.fnBounds.eqs {
				if eq.assoc == t.name {
					return c.memberOfType(eq.rhs, x, asCall)
				}
			}
		}
		c.fail(x.NameLine, x.NameCol, "E0815", fmt.Sprintf(
			"method call outside the receiver's method set — %q is outside the method set of %q; an associated position reaches members through its equality binding",
			x.Name, t.String()))
	}
	panic("unreachable member")
}

// methodUse holds the method-value rule (chapter 12): a method view fits
// a call head alone. Every other value production rejects the bare
// member name — E0105 composes the parser family's prefix, and the wrap
// example names the receiver when it is a plain name.
func (c *checker) methodUse(x *ast.Member, asCall bool) {
	if asCall {
		return
	}
	wrap := fmt.Sprintf("|x| x.%s()", x.Name)
	if id, ok := x.Recv.(*ast.Ident); ok {
		wrap = fmt.Sprintf("|%s| %s.%s()", id.Name, id.Name, x.Name)
	}
	c.fail(x.NameLine, x.NameCol, "E0105", fmt.Sprintf(
		"unexpected token — %q resolves to a method; a method name fits no ratified value production: call it or wrap it: %s",
		x.Name, wrap))
}

// headImplementsIterator reports whether a nominal head carries a
// validated impl of the builtin Iterator interface.
func (c *checker) headImplementsIterator(headName string) bool {
	for _, im := range c.impls {
		if im.iface == iteratorIface && im.headName == headName {
			return true
		}
	}
	return false
}

// nameResolvable reports whether a bare name resolves anywhere (a local,
// a module declaration, a prelude name): a qualifier candidate that
// resolves is a receiver, not a failed qualifier.
func (c *checker) nameResolvable(name string) bool {
	if _, ok := c.lookupLocal(name); ok {
		return true
	}
	if _, ok := c.syms[name]; ok {
		return true
	}
	switch name {
	case "panic", "todo", "assert", "currentCancelSignal", "advanceTime",
		"Ok", "Err", "Some", "None", "Never", "Result", "Option",
		"List", "Map", "Set", "Range", "Dyn", "Shareable",
		"Iterator", "Iterable", "Eq", "Hash", "Show":
		return true
	}
	return baseNames[name]
}

// --- chapter 10: the Dyn box --------------------------------------------------

// dynFace types the box's one argument (E0819/E0820): an interface
// declaring no associated types — the erased value could not honor a
// binding — resolved through the layers a declaration's clause reads.
func (c *checker) dynFace(args []ast.TypeRef) dynType {
	iv := c.dynIfaceRef(args[0])
	if len(iv.decl.assocs) > 0 {
		line, col := refPos(args[0])
		c.fail(line, col, "E0819", fmt.Sprintf(
			"Dyn of an interface with associated types — %q declares the associated type %q; the erased value could not honor the binding",
			iv.decl.name, iv.decl.assocs[0]))
	}
	return dynType{inf: iv.decl, args: iv.args}
}

// dynIfaceRef resolves the box's interface argument: the position reads
// interfaces only, so a non-interface name is E0820 (named by the name
// itself) and an unknown one is E1304 — both anchored at the name.
func (c *checker) dynIfaceRef(tr ast.TypeRef) ifaceType {
	nt, ok := tr.(*ast.NamedType)
	if !ok {
		t := c.resolveTypeRef(tr, slotGeneric)
		line, col := refPos(tr)
		c.fail(line, col, "E0820", fmt.Sprintf(
			"Dyn argument is not an interface type — %q is not an interface; Dyn boxes an interface type",
			t.String()))
	}
	if nt.Qual != "" {
		if mod, is := c.importQualifier(nt.Qual); is {
			return c.importIface(mod, nt, "E0820", fmt.Sprintf(
				"Dyn argument is not an interface type — %q is not an interface; Dyn boxes an interface type",
				nt.Name))
		}
		full := nt.Qual + "." + nt.Name
		c.fail(nt.Line, nt.Col, "E1304", fmt.Sprintf(
			"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
			nt.Qual, full))
	}
	if _, inClause := c.lookupTypeScope(nt.Name); inClause {
		// a clause parameter names no interface
		c.fail(nt.Line, nt.Col, "E0820", fmt.Sprintf(
			"Dyn argument is not an interface type — %q is not an interface; Dyn boxes an interface type",
			nt.Name))
	}
	if sym, ok := c.syms[nt.Name]; ok {
		if sym.kind == symIface {
			c.checkArity(nt, sym.iface.name, sym.iface.params)
			return ifaceType{decl: sym.iface, args: c.resolveArgs(nt.Args, slotGeneric)}
		}
		c.fail(nt.Line, nt.Col, "E0820", fmt.Sprintf(
			"Dyn argument is not an interface type — %q is not an interface; Dyn boxes an interface type",
			nt.Name))
	}
	if inf := builtinIface(nt.Name); inf != nil {
		c.checkArity(nt, inf.name, inf.params)
		return ifaceType{decl: inf, args: c.resolveArgs(nt.Args, slotGeneric)}
	}
	switch nt.Name {
	case "Shareable":
		// chapter 18's marker is never boxed either — no value surface
		c.fail(nt.Line, nt.Col, "E0820", fmt.Sprintf(
			"Dyn argument is not an interface type — %q is the compiler-attached marker of chapter 18; it stands in bounds (T: Shareable), never as a box",
			nt.Name))
	case "Never", "Result", "Option", "Dyn", "List", "Map", "Set", "Range":
		// prelude names that are no interfaces — E0820 below
	default:
		if !baseNames[nt.Name] {
			c.fail(nt.Line, nt.Col, "E1304", bareUnresolved(nt.Name))
		}
	}
	c.fail(nt.Line, nt.Col, "E0820", fmt.Sprintf(
		"Dyn argument is not an interface type — %q is not an interface; Dyn boxes an interface type",
		nt.Name))
	panic("unreachable dyn iface")
}

// dynCall types the box construction Dyn<I>(value) (chapter 10): the
// value's type implements the carried face — the face's own value boxes
// directly (an iterator position hands its own handle), a where bound the
// enclosing clause grants the position, a head's validated impl carries
// (exactly, or a generic impl through the determination), and anything
// else implements nothing (E0818, anchored at the value's first token).
func (c *checker) dynCall(x *ast.Call) Type {
	if len(x.TypeArgs) != 1 {
		c.arityFail(x.ArgLine, x.ArgCol, x.Line, x.Col, "Dyn", 1, len(x.TypeArgs))
	}
	box := c.dynFace(x.TypeArgs)
	if len(x.Args) != 1 {
		c.bnd(bndArityGap)
	}
	at := c.typeOf(x.Args[0], nil)
	// The box route, construction face (chapter 13): a resource-typed
	// argument boxes the one handle, whatever face the box carries (E1106,
	// at the construction head's first token).
	if catOf(at) == "resource" {
		line, col := exprPos(x)
		c.fail(line, col, "E1106", fmt.Sprintf(
			"resource type in a composite position — the construction Dyn<%s>(r) with a resource-typed argument is the box route; a resource is never boxed",
			box.inf.name))
	}
	face := ifaceType{decl: box.inf, args: box.args}
	if c.implementsFace(at, face) {
		return box
	}
	line, col := exprPos(x.Args[0])
	c.fail(line, col, "E0818", fmt.Sprintf(
		"Dyn construction from a non-implementing type — %q implements no %q; a Dyn box carries a value of an implementing type",
		at.String(), face.String()))
	panic("unreachable dyn call")
}

// nominalDeclName names a nominal type's declaration ("" otherwise).
func nominalDeclName(t Type) string {
	switch x := t.(type) {
	case recordType:
		return x.decl.name
	case newtypeType:
		return x.decl.name
	case namedType:
		return x.decl.name
	}
	return ""
}

// callType types one call. An explicit generic clause faces its arity
// judgment before any callee disposition (E0828) — the clause reads the
// declaration the name carries, even one the value layer would reject.
// Callee-name resolution follows (E1304); the prelude's boundary names;
// then the argument count (the spec-gap boundary) and the arguments
// against the parameters.
func (c *checker) callType(x *ast.Call, expected Type) Type {
	// A qualified call head (b.helper(…)) takes the symbol-kind dispatch
	// a bare head takes (chapter 15): a generic fn's determination and a
	// variant's payload check ride the same machinery through the target
	// module's declaration nodes.
	if m, ok := x.Fn.(*ast.Member); ok {
		if id, is := m.Recv.(*ast.Ident); is {
			if mod, imp := c.importQualifier(id.Name); imp {
				return c.importCall(mod, x, expected)
			}
		}
	}
	if id, ok := x.Fn.(*ast.Ident); ok {
		if t, isLocal := c.lookupLocal(id.Name); isLocal {
			ft, isFn := t.(fnType)
			if !isFn {
				c.bnd(bndCalleeGap)
			}
			return c.fnValueCall(ft, x)
		}
		if len(x.TypeArgs) > 0 {
			if sym, ok := c.syms[id.Name]; ok {
				want, name := 0, id.Name
				switch sym.kind {
				case symFn:
					want, name = len(sym.fn.TypeParams), sym.fn.Name
				case symVariant:
					want = len(sym.sum.params)
				case symNewtype:
					want, name = len(sym.nt.params), sym.nt.name
				case symType:
					want = len(sym.sum.params)
				}
				if len(x.TypeArgs) != want {
					c.arityFail(x.ArgLine, x.ArgCol, x.Line, x.Col, name, want, len(x.TypeArgs))
				}
			}
		}
		if sym, ok := c.syms[id.Name]; ok {
			switch sym.kind {
			case symFn:
				return c.fnCall(sym.fn, x, "")
			case symVariant:
				return c.ctorCall(sym.sum, sym.vi, x)
			case symNewtype:
				return c.newtypeCall(sym.nt, x)
			default:
				c.bnd(bndCalleeGap)
			}
		}
		switch id.Name {
		case "panic", "todo", "assert":
			// The panic family's prelude signatures (chapter 14): the call
			// takes the ordinary fn-value path — the arguments check
			// position-wise, the arity gap stays the honest boundary.
			return c.fnValueCall(terminationFnType(id.Name), x)
		case "currentCancelSignal":
			// Chapter 18's accessor: a 0-ary call returning the ambient
			// signal, legal only lexically inside a task body (E1608 —
			// the rule is containment, so a closure inside the task body
			// counts as inside).
			if c.taskDepth == 0 {
				c.fail(id.Line, id.Col, "E1608", outsideTaskMsg(c.bodyName))
			}
			return c.fnValueCall(fnType{ret: namedType{decl: cancelSignalSum}}, x)
		case "advanceTime":
			// The call face of the same clock typing (design D7): the
			// arguments check position-wise through the ordinary fn-value
			// path (advanceTime(true) is E0501 there), the position gate
			// reads the same extent, and the produced value is unit.
			if c.testExtent == 0 {
				c.fail(id.Line, id.Col, "E1806", advanceTimeOutside)
			}
			return c.fnValueCall(fnType{params: []Type{baseType("Int64")}, ret: unitType{}}, x)
		case "Ok", "Err", "Some", "None":
			return c.builtinCtorCall(id, x, expected)
		case "Dyn":
			return c.dynCall(x)
		case "List", "Map", "Set":
			// the collection names are types, not constructors — the
			// literals build lists and the stdlib builds the rest, so
			// the head is a non-function callee (the spec-gap row)
			c.bnd(bndCalleeGap)
		}
		c.fail(id.Line, id.Col, "E1304", bareUnresolved(id.Name))
	}
	// a non-name callee: typed, then called through its fn type — a call
	// head is the one position a method view fits (chapter 12's
	// method-value rule exempts the head here, never the arguments)
	var t Type
	var head *ast.Member
	var recvT Type
	if m, is := x.Fn.(*ast.Member); is {
		head = m
		// the member head rides the full preamble (the qualified form,
		// the forEach guard) and hands the receiver's type back with the
		// view — the determination reads the positions it holds.
		t, recvT = c.memberTypeRecv(m, true)
	} else {
		t = c.typeOf(x.Fn, nil)
	}
	ft, isFn := t.(fnType)
	if !isFn {
		c.bnd(bndCalleeGap)
	}
	var rt Type
	if head != nil && fnContainsParam(ft) {
		// a method view still holding clause positions: the arguments
		// determine them (design D6's method generics). A fn-typed
		// value's parameters are the enclosing scope's positions, never
		// a callee's to determine — that path stays positional.
		rt = c.methodCall(ft, head, x, recvT)
	} else {
		rt = c.fnValueCall(ft, x)
	}
	// Chapter 18's nested-access discipline (E1613), after the arguments
	// typed clean (the release walk's position): a shared cell's own
	// callback re-acquiring the cell — direct, per binding, no chasing
	// into further functions (the spec's own stated boundary). The
	// receiver's named type is in hand here and nowhere later, so the
	// W1912 blocking-operation advisory rides the same tail (design D4).
	if head != nil {
		c.nestedAccessCheck(head, x, recvT)
		c.adviseBlocking(head, recvT)
	}
	return rt
}

// nestedAccessCheck fires E1613's walk when one call is a shared cell's
// update or read carrying a closure argument.
func (c *checker) nestedAccessCheck(m *ast.Member, x *ast.Call, recv Type) {
	if m.Name != "update" && m.Name != "read" {
		return
	}
	id, ok := m.Recv.(*ast.Ident)
	if !ok {
		return
	}
	nt, isN := recv.(namedType)
	if !isN || (nt.decl != mutexSum && nt.decl != rwlockSum && nt.decl != atomicSum && nt.decl != atomicRefSum) {
		return
	}
	for _, a := range x.Args {
		if cl, isCl := a.(*ast.Closure); isCl {
			c.checkNestedAccess(id.Name, m.Name, cl)
			// The W1911 companion (M11 design D4) on the same site: the
			// callback is the one subtree where a pass of the binding
			// onward is the indirect face E1613 cannot see.
			c.adviseIndirect(id.Name, cl)
		}
	}
}

// --- chapter 10: generic determination (design D6) ----------------------------

// unifyInto binds parameter positions from an argument's type (design
// D6's single-direction unification: the argument's structure flows into
// the parameter's positions, never back — expected-type inference does
// not exist). A bound position must agree; a shape carrying positions
// against a different shape does not determine. The map may hold partial
// bindings after a false.
func unifyInto(decl, arg Type, m map[int]Type) bool {
	if pr, ok := decl.(paramRef); ok {
		if b, has := m[pr.idx]; has {
			if sameType(b, arg) {
				return true
			}
			// The argument may still hold the very position being
			// determined — a bare closure's parameters took it from the
			// expected view (the method-generic threading). A position is
			// consistent with its own binding: its materialization is the
			// determination's, and the post-call agreement check compares
			// both sides substituted.
			if a, is := arg.(paramRef); is && a.idx == pr.idx && a.name == pr.name {
				return true
			}
			return false
		}
		m[pr.idx] = arg
		return true
	}
	if !containsParam(decl) {
		return true
	}
	switch d := decl.(type) {
	case tupleType:
		a, ok := arg.(tupleType)
		if !ok || len(a.elems) != len(d.elems) {
			return false
		}
		for i := range d.elems {
			if !unifyInto(d.elems[i], a.elems[i], m) {
				return false
			}
		}
		return true
	case fnType:
		a, ok := arg.(fnType)
		if !ok || len(a.params) != len(d.params) {
			return false
		}
		for i := range d.params {
			if !unifyInto(d.params[i], a.params[i], m) {
				return false
			}
		}
		return unifyInto(d.ret, a.ret, m)
	case namedType:
		a, ok := arg.(namedType)
		return ok && a.decl == d.decl && unifyIntoArgs(d.args, a.args, m)
	case recordType:
		a, ok := arg.(recordType)
		return ok && a.decl == d.decl && unifyIntoArgs(d.args, a.args, m)
	case newtypeType:
		a, ok := arg.(newtypeType)
		return ok && a.decl == d.decl && unifyIntoArgs(d.args, a.args, m)
	case ifaceType:
		a, ok := arg.(ifaceType)
		return ok && a.decl == d.decl && unifyIntoArgs(d.args, a.args, m)
	case dynType:
		a, ok := arg.(dynType)
		return ok && a.inf == d.inf && unifyIntoArgs(d.args, a.args, m)
	}
	return false
}

func unifyIntoArgs(decl, arg []Type, m map[int]Type) bool {
	if len(decl) != len(arg) {
		return false
	}
	for i := range decl {
		if !unifyInto(decl[i], arg[i], m) {
			return false
		}
	}
	return true
}

// inferUnify is the determination's failing wrapper: a shape that does
// not determine, or a bound position's disagreement, is E0501 at the
// argument (never a silent skip).
func (c *checker) inferUnify(decl Type, arg Type, m map[int]Type, a ast.Expr) {
	if unifyInto(decl, arg, m) {
		return
	}
	line, col := exprPos(a)
	c.fail(line, col, "E0501", fmt.Sprintf(
		"mixed types — the argument is %s, the parameter is %s; no coercion is ever inserted",
		arg.String(), decl.String()))
}

// inferValueArgs determines clause positions from value expressions
// against declared shapes — the shared body of the call, constructor,
// and construction determinations. Values type without an expected
// (expected-type inference does not exist); an open position after every
// value reports through failOpen.
func (c *checker) inferValueArgs(shapes []Type, values []ast.Expr, clauseLen int, failOpen func()) []Type {
	if len(shapes) != len(values) {
		c.bnd(bndArityGap)
	}
	bindings := map[int]Type{}
	for i, v := range values {
		vt := c.typeOf(v, nil)
		c.inferUnify(shapes[i], vt, bindings, v)
	}
	args := make([]Type, clauseLen)
	open := false
	for i := range args {
		if b, ok := bindings[i]; ok {
			args[i] = b
		} else {
			open = true
		}
	}
	if open {
		failOpen()
	}
	return args
}

// substMap substitutes through the determination's sparse bindings (only
// the bound positions replace — a method view's determination binds the
// positions its arguments reached).
func substMap(t Type, m map[int]Type) Type {
	if len(m) == 0 {
		return t
	}
	switch x := t.(type) {
	case paramRef:
		if r, ok := m[x.idx]; ok {
			return r
		}
	case tupleType:
		elems := make([]Type, len(x.elems))
		for i, e := range x.elems {
			elems[i] = substMap(e, m)
		}
		return tupleType{elems: elems}
	case fnType:
		return fnType{params: substMapList(x.params, m), tags: x.tags, ret: substMap(x.ret, m)}
	case namedType:
		return namedType{decl: x.decl, args: substMapList(x.args, m)}
	case recordType:
		return recordType{decl: x.decl, args: substMapList(x.args, m)}
	case newtypeType:
		return newtypeType{decl: x.decl, args: substMapList(x.args, m)}
	case ifaceType:
		return ifaceType{decl: x.decl, args: substMapList(x.args, m)}
	case dynType:
		return dynType{inf: x.inf, args: substMapList(x.args, m)}
	}
	return t
}

func substMapList(as []Type, m map[int]Type) []Type {
	if len(as) == 0 {
		return nil
	}
	out := make([]Type, len(as))
	for i, a := range as {
		out[i] = substMap(a, m)
	}
	return out
}

// openRefs collects the parameter positions a view still holds unbound —
// a method's determination must reach every position its signature names
// (a position only the return reaches is as open as one an argument
// misses).
func openRefs(f fnType, m map[int]Type) []paramRef {
	var out []paramRef
	var walk func(t Type)
	walk = func(t Type) {
		switch x := t.(type) {
		case paramRef:
			if _, bound := m[x.idx]; !bound {
				out = append(out, x)
			}
		case tupleType:
			for _, e := range x.elems {
				walk(e)
			}
		case fnType:
			for _, p := range x.params {
				walk(p)
			}
			walk(x.ret)
		case namedType, recordType, newtypeType, ifaceType, dynType:
			for _, a := range typeArgsOf(x) {
				walk(a)
			}
		}
	}
	for _, p := range f.params {
		walk(p)
	}
	walk(f.ret)
	return out
}

// typeArgsOf reads a nominal type's application arguments.
func typeArgsOf(t Type) []Type {
	switch x := t.(type) {
	case namedType:
		return x.args
	case recordType:
		return x.args
	case newtypeType:
		return x.args
	case ifaceType:
		return x.args
	case dynType:
		return x.args
	}
	return nil
}

// recvHeld collects the clause positions a receiver's own application
// carries: a method view's interface positions arrive substituted from
// the receiver, so the call's arguments never owe them (chapter 10's
// two namespaces — the method's own clause alone is the arguments' to
// determine). A default body's walk is the extreme case: self is the
// interface's own view, every position symbolic and every one the
// receiver's.
func recvHeld(t Type) map[paramRef]bool {
	var held map[paramRef]bool
	var add func(t Type)
	add = func(t Type) {
		switch x := t.(type) {
		case paramRef:
			if held == nil {
				held = map[paramRef]bool{}
			}
			held[x] = true
		case tupleType:
			for _, e := range x.elems {
				add(e)
			}
		case fnType:
			for _, p := range x.params {
				add(p)
			}
			add(x.ret)
		case namedType, recordType, newtypeType, ifaceType, dynType:
			for _, a := range typeArgsOf(x) {
				add(a)
			}
		}
	}
	add(t)
	return held
}

// methodCall types one call through a method view still holding clause
// positions (design D6's method generics): a method call carries no
// explicit type-argument form, so the arguments determine the positions
// — an open position after every argument is E0827 at the member name,
// and each argument agrees with its determined parameter (E0501). The
// positions the receiver's own application carries are excluded from
// the demand (recvHeld): they arrived substituted from the receiver, so
// the arguments never owe them — the method's own clause alone is the
// arguments' to determine.
func (c *checker) methodCall(ft fnType, m *ast.Member, x *ast.Call, recv Type) Type {
	// Chapter 16 as at a plain call (design D4), through the method view
	// the registry hands the receiver — an impl method's segment, or the
	// interface method's own at a bound, boxed, or generic receiver. The
	// anchor is the receiver's first token, the call's head.
	line, col := exprPos(m.Recv)
	c.checkCallEffect(ft.tags, m.Name, line, col)
	if len(x.Args) != len(ft.params) {
		c.bnd(bndArityGap)
	}
	bindings := map[int]Type{}
	argTypes := make([]Type, len(x.Args))
	for i, a := range x.Args {
		// The declared position threads as the argument's expected type —
		// the method-generic clause positions stay symbolic inside it, so
		// a bare-parameter closure takes its parameter types from the
		// declaration while its return still determines the clause by
		// unification (chapter 11: U comes from the call's own text). An
		// earlier argument's determination substitutes in first: fold's
		// init pins U before the closure's parameters read it (the
		// threading the builtin bodies themselves rely on).
		argTypes[i] = c.typeOf(a, substMap(ft.params[i], bindings))
		c.inferUnify(ft.params[i], argTypes[i], bindings, a)
	}
	if missing := openRefs(ft, bindings); len(missing) > 0 {
		// A position the receiver itself holds stays open in the view
		// (the default-body walk's self is the extreme case — every
		// interface position symbolic, every one the receiver's).
		held := recvHeld(recv)
		if len(held) > 0 {
			kept := missing[:0]
			for _, pr := range missing {
				if !held[pr] {
					kept = append(kept, pr)
				}
			}
			missing = kept
		}
		if len(missing) > 0 {
			c.fail(m.NameLine, m.NameCol, "E0827", fmt.Sprintf(
				"generic call does not determine its type arguments — the call of %q determines no %q; a method call carries no explicit type-argument form: restructure the call so its arguments determine it",
				m.Name, missing[0].name))
		}
	}
	for i, a := range x.Args {
		// Both sides substitute before comparing: an argument typed under
		// the threading may still hold the clause's own positions (a bare
		// closure's view), and those positions are the determination's —
		// the substituted sides compare like for like. The woven judgment
		// gets its chance before E0501's plain face (design D5, the same
		// weave the plain-call slot rides): a closure performing more
		// than the slot's segment expects reports E1402 at the argument.
		if want := substMap(ft.params[i], bindings); !agree(substMap(argTypes[i], bindings), want) {
			line, col := exprPos(a)
			if !c.checkFnSlot(substMap(argTypes[i], bindings), want, line, col) {
				c.fail(line, col, "E0501", fmt.Sprintf(
					"mixed types — the argument is %s, the parameter is %s; no coercion is ever inserted",
					argTypes[i].String(), want.String()))
			}
		}
	}
	return substMap(ft.ret, bindings)
}

// implementsFace is the implements judgment (design D6) — one mechanism
// behind two codes (the Dyn box's E0818 and the where bound's E0830
// differ in anchor and message alone): the face's own value, a where
// bound the enclosing clause grants the position, a validated exact
// impl, or a generic impl whose head the argument's shape determines
// with the interface's positions agreeing.
func (c *checker) implementsFace(at Type, face ifaceType) bool {
	if sameType(at, face) {
		return true
	}
	if pr, ok := at.(paramRef); ok {
		if c.fnBounds == nil {
			return false
		}
		for _, b := range c.fnBounds.bounds {
			if b.idx == pr.idx && b.face.decl == face.decl && argsEqual(b.face.args, face.args) {
				return true
			}
		}
		return false
	}
	for _, im := range c.impls {
		if im.iface != face.decl {
			continue
		}
		if sameType(im.head, at) && argsEqual(im.ifaceArgs, face.args) {
			return true
		}
	}
	if name := nominalDeclName(at); name != "" {
		for _, im := range c.impls {
			if im.iface != face.decl || im.headName != name {
				continue
			}
			bindings := map[int]Type{}
			if !unifyInto(im.head, at, bindings) {
				continue
			}
			applied := make([]Type, len(im.ifaceArgs))
			for i, a := range im.ifaceArgs {
				applied[i] = substMap(a, bindings)
			}
			if argsEqual(applied, face.args) {
				return true
			}
		}
	}
	return false
}

// fnValueCall types one call through a fn-typed value (chapter 12): the
// arguments check position-wise against the parameters (E0501), a count
// mismatch is the arity spec-gap boundary, and the fn type's return is
// the call's.
func (c *checker) fnValueCall(ft fnType, x *ast.Call) Type {
	// Chapter 16 through the fn type's own segment (design D4): a
	// fn-typed parameter, let, or field — the callee's name is the head's
	// own. The panic family rides here with the empty set: no subset of
	// it can fail, so the judgment is a no-op there by construction.
	name, line, col := calleeSite(x.Fn)
	c.checkCallEffect(ft.tags, name, line, col)
	if len(x.Args) != len(ft.params) {
		c.bnd(bndArityGap)
	}
	c.checkArgs(x.Args, ft.params)
	return ft.ret
}

// fnCall types one call to a module fn: a generic clause instantiates
// the declaration (chapter 10) — the written arguments (E0828's arity
// first, at the call head's pre-block), or the determination from the
// call's arguments (design D6's single-direction unification); the where
// clause holds at every application (E0830, at the call head). mod is
// the callee's canonical module key — "" for a name of the live module,
// the import's resolved key for a qualified call — the one fact the
// W1910 mock-identity judgment needs that the body machinery does not.
func (c *checker) fnCall(fd *ast.FnDecl, x *ast.Call, mod string) Type {
	// Chapter 16's call judgment leads (design D4): the callee's declared
	// set faces the enclosing body's before any application machinery —
	// an effect segment is never generic, so no determination can change
	// it.
	line, col := exprPos(x.Fn)
	c.checkCallEffect(c.fnTags[fd], fd.Name, line, col)
	c.adviseUnmocked(fd, mod, x)
	params := c.fnParams[fd]
	var args []Type
	if len(fd.TypeParams) > 0 {
		if len(x.TypeArgs) > 0 {
			args = c.resolveArgs(x.TypeArgs, slotGeneric)
		} else {
			args = c.inferValueArgs(params, x.Args, len(fd.TypeParams), func() {
				line, col := exprPos(x.Fn)
				c.fail(line, col, "E0827", fmt.Sprintf(
					"generic call does not determine its type arguments — %q determines no type argument; a call with no determining argument takes the explicit form %s<Int64>()",
					fd.Name, fd.Name))
			})
		}
		params = substArgs(params, args, nil)
		c.checkWhereSatisfies(c.fnWheres[fd], args, x, fd.Name)
	}
	if len(x.Args) != len(params) {
		c.bnd(bndArityGap)
	}
	c.checkArgs(x.Args, params)
	if fd.Ret == nil {
		return unitType{}
	}
	return subst(c.fnRets[fd], args, nil)
}

// ctorCall types one call to a user variant constructor: a generic
// clause instantiates the sum — the payloads substitute, and the
// category (E0702) and capability (E0823) honesty re-checks at the
// application, anchored at the argument tokens or the call head (a
// determined application's anchor is the head's — no argument token
// exists); a bare call of a generic sum determines from its payload
// arguments (design D6).
func (c *checker) ctorCall(sum *sumInfo, vi int, x *ast.Call) Type {
	v := &sum.variants[vi]
	var args []Type
	if len(x.TypeArgs) > 0 {
		args = c.resolveArgs(x.TypeArgs, slotGeneric)
	}
	if len(sum.params) > 0 && len(args) == 0 {
		args = c.inferValueArgs(v.payloads, x.Args, len(sum.params), func() {
			line, col := exprPos(x.Fn)
			c.fail(line, col, "E0827", fmt.Sprintf(
				"generic call does not determine its type arguments — %q determines no type argument; a call with no determining argument takes the explicit form %s<Int64>()",
				v.name, v.name))
		})
	}
	payloads := v.payloads
	if args != nil {
		payloads = substArgs(v.payloads, args, nil)
		c.checkPayloadInstantiation(sum, v, typeArgAnchors(x.TypeArgs, args, x.Line, x.Col), args)
	}
	if len(x.Args) != len(payloads) {
		c.bnd(bndArityGap)
	}
	c.checkArgs(x.Args, payloads)
	if args != nil {
		return namedType{decl: sum, args: args}
	}
	return namedType{decl: sum}
}

// checkPayloadInstantiation re-checks a sum's honesty at an application:
// a value sum's payload categories (E0702) and a derives clause's
// capability (E0823) hold at every instantiation, anchored at the
// argument that materialized the failing position.
func (c *checker) checkPayloadInstantiation(sum *sumInfo, v *variantInfo, anchors [][2]int, args []Type) {
	for _, p := range v.payloads {
		pr, isRef := p.(paramRef)
		if !isRef || pr.idx >= len(args) {
			continue
		}
		sub := args[pr.idx]
		if containsParam(sub) {
			continue
		}
		applied := fmt.Sprintf("%s<%s>", v.name, typeJoin(args))
		if sum.byval && catOf(sub) != "value" {
			line, col := anchors[pr.idx][0], anchors[pr.idx][1]
			c.fail(line, col, "E0702", fmt.Sprintf(
				"value sum payload is not of the value category or a base type — %q instantiates the payload of %q with %q, a %s; a copy is only honest when everything in it is copyable by value",
				applied, sum.name, sub.String(), catNoun(sub)))
		}
		for _, target := range sum.derives {
			if carriesType(sub, target) {
				continue
			}
			line, col := anchors[pr.idx][0], anchors[pr.idx][1]
			c.fail(line, col, "E0823", fmt.Sprintf(
				"derive field requirement unmet — %q instantiates %q with %q, which carries no %q; the clause is checked at each instantiation",
				applied, pr.name, sub.String(), target))
		}
	}
}

// newtypeCall types the call form of a newtype construction (chapter 8):
// exactly one argument checked against the underlying type — substituted
// at an application (the written arguments or the determination from the
// one argument, design D6), whose capability (E0823) re-checks there (a
// newtype's category derives from its underlying, so no category check
// holds at the application); the wrapper is the result.
func (c *checker) newtypeCall(nt *newtypeInfo, x *ast.Call) Type {
	var args []Type
	if len(x.TypeArgs) > 0 {
		args = c.resolveArgs(x.TypeArgs, slotGeneric)
	}
	if len(nt.params) > 0 && len(args) == 0 {
		args = c.inferValueArgs([]Type{nt.underlying}, x.Args, len(nt.params), func() {
			line, col := exprPos(x.Fn)
			c.fail(line, col, "E0827", fmt.Sprintf(
				"generic call does not determine its type arguments — %q determines no type argument; a call with no determining argument takes the explicit form %s<Int64>()",
				nt.name, nt.name))
		})
	}
	under := nt.underlying
	if args != nil {
		under = subst(nt.underlying, args, nil)
		anchors := typeArgAnchors(x.TypeArgs, args, x.Line, x.Col)
		if pr, isRef := nt.underlying.(paramRef); isRef && pr.idx < len(args) {
			sub := args[pr.idx]
			if !containsParam(sub) {
				for _, target := range nt.derives {
					if carriesType(sub, target) {
						continue
					}
					line, col := anchors[pr.idx][0], anchors[pr.idx][1]
					c.fail(line, col, "E0823", fmt.Sprintf(
						"derive field requirement unmet — %q instantiates %q with %q, which carries no %q; the clause is checked at each instantiation",
						newtypeType{decl: nt, args: args}.String(), pr.name, sub.String(), target))
				}
			}
		}
	}
	if len(x.Args) != 1 {
		c.bnd(bndArityGap)
	}
	c.checkArgs(x.Args, []Type{under})
	return newtypeType{decl: nt, args: args}
}

// builtinCtorCall types Ok/Err/Some/None: the expected type determines
// the application; without one (or against a different shape) the call
// does not determine its type arguments (E0827).
func (c *checker) builtinCtorCall(id *ast.Ident, x *ast.Call, expected Type) Type {
	if id.Name == "None" {
		if nt, ok := expected.(namedType); ok && nt.decl == optionSum && len(x.Args) == 0 {
			return nt
		}
		if len(x.Args) != 0 {
			c.bnd(bndArityGap)
		}
		c.fail(id.Line, id.Col, "E0827", undetermined("None", "Option"))
	}
	var sum *sumInfo
	pos := 0
	switch id.Name {
	case "Ok":
		sum, pos = resultSum, 0
	case "Err":
		sum, pos = resultSum, 1
	case "Some":
		sum, pos = optionSum, 0
	}
	if nt, ok := expected.(namedType); ok && nt.decl == sum {
		payload := nt.args[pos]
		if len(x.Args) != 1 {
			c.bnd(bndArityGap)
		}
		c.checkArgs(x.Args, []Type{payload})
		return nt
	}
	c.fail(id.Line, id.Col, "E0827", undetermined(id.Name, sum.name))
	panic("unreachable ctor")
}

// checkArgs types call arguments against their parameter types with the
// expected type threaded; disagreements anchor at the argument's first
// token.
func (c *checker) checkArgs(args []ast.Expr, params []Type) {
	for i, a := range args {
		at := c.typeOf(a, params[i])
		if !agree(at, params[i]) {
			line, col := exprPos(a)
			if !c.checkFnSlot(at, params[i], line, col) {
				c.fail(line, col, "E0501", fmt.Sprintf(
					"mixed types — the argument is %s, the parameter is %s; no coercion is ever inserted",
					at.String(), params[i].String()))
			}
		}
	}
}

// --- chapter 11/17: iteration, ranges, and collections (design D9) ------------

// rangeType types one range operator `a..b` (chapter 11): each operand is
// one of the eight integer types (E0902, anchored at the offending
// operand's first token — a literal names its own text), the two sides
// agree (E0501 at the operator, the binary convention), and the operator
// builds the builtin Range<T> over the operands' type.
func (c *checker) rangeType(x *ast.Binary) Type {
	lt := c.typeOf(x.L, nil)
	rt := c.typeOf(x.R, nil)
	for _, side := range []struct {
		e ast.Expr
		t Type
	}{{x.L, lt}, {x.R, rt}} {
		if isInt(side.t) {
			continue
		}
		label := side.t.String()
		if lit, ok := side.e.(*ast.Literal); ok {
			label = lit.Text
		}
		line, col := exprPos(side.e)
		c.fail(line, col, "E0902", fmt.Sprintf(
			"range operand is not an integer type — the operand %q is %q; a range's operands are the eight integer types",
			label, side.t.String()))
	}
	if !sameType(lt, rt) {
		c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
			"mixed types — operands of %q are %s and %s; no coercion is ever inserted",
			x.Op, lt.String(), rt.String()))
	}
	return namedType{decl: rangeSum, args: []Type{lt}}
}

// listLitType types one list literal (chapter 17): an empty literal takes
// the expected type's application whole (E1501 without one — the empty
// literal determines no element type by itself); a non-empty literal's
// first element fixes the element type and every later element agrees
// (E0501 at the disagreeing element's first token). The result is the
// builtin List<T> over the element type.
func (c *checker) listLitType(x *ast.ListLit, expected Type) Type {
	if len(x.Elems) == 0 {
		if nt, ok := expected.(namedType); ok && nt.decl == listSum {
			return nt
		}
		c.fail(x.Line, x.Col, "E1501",
			"empty list literal has no expected type — the empty list literal determines no element type; annotate the position, for example let xs: List<Int64> = []")
	}
	elem := c.typeOf(x.Elems[0], nil)
	for _, e := range x.Elems[1:] {
		t := c.typeOf(e, nil)
		if !sameType(t, elem) {
			line, col := exprPos(e)
			c.fail(line, col, "E0501", fmt.Sprintf(
				"mixed types — the elements are %s and %s; no coercion is ever inserted",
				elem.String(), t.String()))
		}
	}
	return namedType{decl: listSum, args: []Type{elem}}
}

// checkFor types one for statement (chapters 5 and 11): the iterable
// expression implements Iterable (E0901 — a builtin source, a validated
// impl, or a where bound granting the position), the head pattern binds
// against the element type (E0501 at a tuple head against a non-tuple
// element), and the body is a control body (its dropped tail is E0605's)
// with the pattern's names in scope.
func (c *checker) checkFor(st *ast.ForStmt) {
	elem := c.forElem(st)
	c.locals = append(c.locals, map[string]Type{})
	c.bindForPattern(st.Pat, elem)
	c.walkItems(st.Body.Items, walkControl)
	c.locals = c.locals[:len(c.locals)-1]
}

// checkScopeRes types chapter 13's scope resource statement: each head
// expression takes over a handle — its type must implement Releasable
// (E1103, at the head expression's first token) — the bindings join one
// fresh locals layer, and the body is a plain nested block. The binding's
// transfer discipline is the liveness pass's (design D4); a defer inside
// the block is E0204's placement rule, held at parse.
func (c *checker) checkScopeRes(st *ast.ScopeRes) {
	binds := map[string]Type{}
	for _, b := range st.Binds {
		t := c.typeOf(b.Val, nil)
		if !c.implementsFace(t, ifaceType{decl: releasableIface}) {
			line, col := exprPos(b.Val)
			c.fail(line, col, "E1103", fmt.Sprintf(
				"scope resource head does not implement Releasable — the head expression's type %q implements no \"Releasable\"; bind the expression with let, or make the head's type a byres record implementing Releasable",
				t.String()))
		}
		binds[b.Name] = t
	}
	c.locals = append(c.locals, binds)
	c.walkItems(st.Body.Items, walkControl)
	c.locals = c.locals[:len(c.locals)-1]
}

// forElem holds the for's element judgment (E0901, anchored at the
// iterable expression's first token): an Iterable yields its element type;
// a bare Iterator value names its own shape — the protocol's two message
// forms.
func (c *checker) forElem(st *ast.ForStmt) Type {
	t := c.typeOf(st.Iter, nil)
	if elem, ok := c.iterElemOf(t); ok {
		return elem
	}
	line, col := exprPos(st.Iter)
	if c.isIteratorValue(t) {
		c.fail(line, col, "E0901", "for-in expression does not implement Iterable — a bare iterator is consumed by explicit next calls; a for iterates an \"Iterable\" and this expression yields an \"Iterator\" alone")
	}
	c.fail(line, col, "E0901", fmt.Sprintf(
		"for-in expression does not implement Iterable — %q implements no %q; the iterable expression of a for implements Iterable",
		t.String(), "Iterable"))
	panic("unreachable for elem")
}

// iterElemOf reads one type's iterated element (chapter 11): the builtin
// sources — a String iterates runes, a Range, a List, and a Set iterate
// their parameter, a Map iterates (K, V) entry tuples — then a validated
// impl of Iterable (exactly, or a generic impl the argument's shape
// determines), then a where bound granting a clause position the
// interface.
func (c *checker) iterElemOf(t Type) (Type, bool) {
	if b, ok := t.(baseType); ok && b == "String" {
		return baseType("Rune"), true
	}
	if n, ok := t.(namedType); ok {
		switch n.decl {
		case rangeSum, listSum, setSum:
			return n.args[0], true
		case mapSum:
			return tupleType{elems: n.args}, true
		}
	}
	for _, im := range c.impls {
		if im.iface != iterableIface {
			continue
		}
		if sameType(im.head, t) {
			return im.ifaceArgs[0], true
		}
		bindings := map[int]Type{}
		if unifyInto(im.head, t, bindings) {
			if elem := substMap(im.ifaceArgs[0], bindings); !containsParam(elem) {
				return elem, true
			}
		}
	}
	if pr, ok := t.(paramRef); ok && c.fnBounds != nil {
		for _, b := range c.fnBounds.bounds {
			if b.idx == pr.idx && b.face.decl == iterableIface {
				return b.face.args[0], true
			}
		}
	}
	return nil, false
}

// isIteratorValue reports whether a type is an Iterator value — E0901's
// bare-iterator shape: the interface's own face, the erased box carrying
// it, a where-bounded position, or a nominal head with a validated
// Iterator impl.
func (c *checker) isIteratorValue(t Type) bool {
	switch f := t.(type) {
	case ifaceType:
		return f.decl == iteratorIface
	case dynType:
		return f.inf == iteratorIface
	case paramRef:
		if c.fnBounds == nil {
			return false
		}
		for _, b := range c.fnBounds.bounds {
			if b.idx == f.idx && b.face.decl == iteratorIface {
				return true
			}
		}
	}
	if name := nominalDeclName(t); name != "" && c.headImplementsIterator(name) {
		return true
	}
	return false
}

// bindForPattern binds a for head's irrefutable pattern against the
// element type (chapters 5 and 8): the parser holds the head to a binding,
// the wildcard, or a tuple of them, so a tuple head against a non-tuple
// (or differently-arity) element is E0501 at the pattern's first token,
// and each element pattern binds recursively into the loop's scope.
func (c *checker) bindForPattern(p ast.Pattern, elem Type) {
	switch x := p.(type) {
	case *ast.PatWildcard:
	case *ast.PatBinding:
		if x.Name != "_" {
			c.locals[len(c.locals)-1][x.Name] = elem
		}
	case *ast.PatTuple:
		tt, ok := elem.(tupleType)
		if !ok || len(tt.elems) != len(x.Elems) {
			c.fail(x.Line, x.Col, "E0501", fmt.Sprintf(
				"mixed types — the loop head pattern is a %d-tuple, the element type is %q; no coercion is ever inserted",
				len(x.Elems), elem.String()))
		}
		for i, e := range x.Elems {
			c.bindForPattern(e, tt.elems[i])
		}
	}
}
