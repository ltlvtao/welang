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
	"os"
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
	bndTermination = "termination functions (chapter 14)"
	bndShareable   = "Shareable markers (chapter 18)"
	bndTaskTime    = "task-scope and time-control functions (chapters 18 and 20)"
	bndStdModules  = "standard-library modules (chapter 15)"
	bndMultiModule = "multi-module programs (chapter 15)"
	bndDomainGap   = "arithmetic and comparisons beyond the ratified numeric and Bool domains (spec gap; roadmap follow-up)"
	bndArityGap    = "calls with an argument count the callee does not declare (spec gap; roadmap follow-up)"
	bndCalleeGap   = "calls on values that are not functions (spec gap; roadmap follow-up)"
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
		s += " " + strings.Join(f.tags, " ")
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
// record), "resource" (byres record), "gc" (default).
type recordInfo struct {
	name    string
	cat     string
	params  []string
	derives []string
	fields  []fieldInfo
	methods []memberMethod
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
// associated types, an optional default body, its own method clause, and
// its name token (the E0808/E0814 anchors).
type ifaceMethod struct {
	name       string
	recvMut    bool
	params     []Type
	ret        Type // nil = valueless
	typeParams []string
	body       *ast.Block
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
)

func init() {
	// Iterator<T>: fn next(mut self) -> Option<T> — the protocol's one
	// non-defaulted method — plus the eleven combinator defaults
	// (chapter 11's Iterator combinators requirement): four lazy — each
	// returning a derived Dyn<Iterator> handle — and seven eager. map and
	// fold carry their own method clause <U>, fresh against the
	// interface's <T> (so U sits at clause position 1); an override must
	// repeat the clause exactly (E0808). The default bodies are the
	// standard library's (M8): a non-nil body is the defaulted marker
	// E0807 and the inherited registration (E0814) read, and the builtin
	// interfaces have no declarations whose bodies pass would walk them.
	t0 := Type(paramRef{idx: 0, name: "T"})
	u1 := Type(paramRef{idx: 1, name: "U"})
	dynIter := func(elem Type) Type {
		return dynType{inf: iteratorIface, args: []Type{elem}}
	}
	fnOf := func(params []Type, ret Type) Type {
		return fnType{params: params, ret: ret}
	}
	iteratorIface.methods = []ifaceMethod{
		{
			name:    "next",
			recvMut: true,
			ret:     namedType{decl: optionSum, args: []Type{t0}},
		},
		// fn map<U>(mut self, f: fn(T) -> U) -> Dyn<Iterator<U>>
		{
			name: "map", recvMut: true, typeParams: []string{"U"}, body: &ast.Block{},
			params: []Type{fnOf([]Type{t0}, u1)},
			ret:    dynIter(u1),
		},
		// fn filter(mut self, f: fn(T) -> Bool) -> Dyn<Iterator<T>>
		{
			name: "filter", recvMut: true, body: &ast.Block{},
			params: []Type{fnOf([]Type{t0}, baseType("Bool"))},
			ret:    dynIter(t0),
		},
		// fn take(mut self, n: Int64) -> Dyn<Iterator<T>>
		{
			name: "take", recvMut: true, body: &ast.Block{},
			params: []Type{baseType("Int64")},
			ret:    dynIter(t0),
		},
		// fn skip(mut self, n: Int64) -> Dyn<Iterator<T>>
		{
			name: "skip", recvMut: true, body: &ast.Block{},
			params: []Type{baseType("Int64")},
			ret:    dynIter(t0),
		},
		// fn collect(mut self) -> List<T>
		{
			name: "collect", recvMut: true, body: &ast.Block{},
			ret: namedType{decl: listSum, args: []Type{t0}},
		},
		// fn fold<U>(mut self, init: U, f: fn(U, T) -> U) -> U
		{
			name: "fold", recvMut: true, typeParams: []string{"U"}, body: &ast.Block{},
			params: []Type{u1, fnOf([]Type{u1, t0}, u1)},
			ret:    u1,
		},
		// fn reduce(mut self, f: fn(T, T) -> T) -> Option<T>
		{
			name: "reduce", recvMut: true, body: &ast.Block{},
			params: []Type{fnOf([]Type{t0, t0}, t0)},
			ret:    namedType{decl: optionSum, args: []Type{t0}},
		},
		// fn count(mut self) -> Int64
		{
			name: "count", recvMut: true, body: &ast.Block{},
			ret: baseType("Int64"),
		},
		// fn any(mut self, f: fn(T) -> Bool) -> Bool
		{
			name: "any", recvMut: true, body: &ast.Block{},
			params: []Type{fnOf([]Type{t0}, baseType("Bool"))},
			ret:    baseType("Bool"),
		},
		// fn all(mut self, f: fn(T) -> Bool) -> Bool
		{
			name: "all", recvMut: true, body: &ast.Block{},
			params: []Type{fnOf([]Type{t0}, baseType("Bool"))},
			ret:    baseType("Bool"),
		},
		// fn find(mut self, f: fn(T) -> Bool) -> Option<T>
		{
			name: "find", recvMut: true, body: &ast.Block{},
			params: []Type{fnOf([]Type{t0}, baseType("Bool"))},
			ret:    namedType{decl: optionSum, args: []Type{t0}},
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
	c := &checker{
		file:       file,
		mode:       mode,
		syms:       map[string]*symbol{},
		fnParams:   map[*ast.FnDecl][]Type{},
		fnRets:     map[*ast.FnDecl]Type{},
		fnWheres:   map[*ast.FnDecl]*whereInfo{},
		implWheres: map[*ast.ImplDecl]*whereInfo{},
	}
	defer func() {
		if r := recover(); r != nil {
			switch s := r.(type) {
			case stop:
				d, ni = &s.d, nil
			case bstop:
				d, ni = nil, &NotImplemented{What: s.what}
			default:
				panic(r)
			}
		}
	}()
	c.checkModule(f)
	return nil, nil
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
			c.syms[name] = &symbol{kind: symImport}
		case *ast.FnDecl:
			c.syms[x.Name] = &symbol{kind: symFn, fn: x}
		case *ast.TopLet:
			if x.Binding.Name != "_" {
				c.syms[x.Binding.Name] = &symbol{kind: symLet}
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
			c.syms[x.Name] = &symbol{kind: symType, sum: sum}
			for i, v := range x.Variants {
				c.syms[v.Name] = &symbol{kind: symVariant, sum: sum, vi: i}
			}
		case *ast.RecordDecl:
			c.syms[x.Name] = &symbol{kind: symRecord, rec: &recordInfo{
				name:    x.Name,
				cat:     x.Cat,
				params:  typeParamNames(x.TypeParams),
				derives: deriveTargetNames(x.Derives),
			}}
		case *ast.NewtypeDecl:
			c.syms[x.Name] = &symbol{kind: symNewtype, nt: &newtypeInfo{
				name:    x.Name,
				params:  typeParamNames(x.TypeParams),
				derives: deriveTargetNames(x.Derives),
			}}
		case *ast.InterfaceDecl:
			inf := &ifaceInfo{name: x.Name, params: typeParamNames(x.TypeParams)}
			for _, a := range x.Assocs {
				inf.assocs = append(inf.assocs, a.Name)
			}
			c.syms[x.Name] = &symbol{kind: symIface, iface: inf}
		case *ast.ImplDecl:
			implDecls = append(implDecls, x)
		}
	}
	// Import dispositions.
	for _, it := range f.Items {
		if imp, ok := it.(*ast.Import); ok {
			c.checkImport(imp)
		}
	}
	// The main convention binds the root module alone.
	if c.mode == Project {
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
				rec.fields = append(rec.fields, fieldInfo{name: f.Name, typ: ft, line: fline, col: fcol})
			}
			c.checkRecordDerives(x, rec)
			c.popTypes()
		case *ast.NewtypeDecl:
			nt := c.syms[x.Name].nt
			c.pushTypes(paramScope(nt.params))
			nt.underlying = c.resolveTypeRef(x.Underlying, slotAnn)
			nt.underLine, nt.underCol = refPos(x.Underlying)
			c.checkNewtypeDerives(x, nt)
			c.popTypes()
		case *ast.FnDecl:
			c.pushTypes(paramScope(typeParamNames(x.TypeParams)))
			var params []Type
			for _, p := range x.Params {
				params = append(params, c.resolveTypeRef(p.Type, slotAnn))
			}
			c.fnParams[x] = params
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
			t := c.checkBinding(&x.Binding)
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
		}
	}
}

// checkImport applies the import dispositions (design D10): std is not
// provided; a project expects a local module at src/<path>.we and, when
// present, multi-module programs stop at their boundary; single-file mode
// has no source root at all.
func (c *checker) checkImport(imp *ast.Import) {
	joined := strings.Join(imp.Path, ".")
	if imp.Path[0] == "std" {
		c.bnd(bndStdModules)
	}
	rel := filepath.Join(imp.Path...) + ".we"
	if c.mode == SingleFile {
		c.fail(imp.PathLine, imp.PathCol, "E1302", fmt.Sprintf(
			"module not found — %q cannot resolve in single-file mode: no source root exists (a project would expect it at src/%s); run we check against the project directory",
			joined, rel))
	}
	if _, err := os.Stat(filepath.Join("src", rel)); err != nil {
		c.fail(imp.PathLine, imp.PathCol, "E1302", fmt.Sprintf(
			"module not found — the import %q expects the module at src/%s and no file is there; create the file at the expected path, fix the path spelling, or add the dependency to the cache",
			joined, rel))
	}
	c.bnd(bndMultiModule)
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
		c.checkMethodBody(x.Methods[i].Params, im.params, im.body, im.ret, self, im.recvMut)
		c.popTypes()
		c.popTypes()
	}
}

// checkMethodBody types one method body (a plain fn's is a method body
// with no self): the parameters scope their names, self joins them, and
// recvMut marks the receiver form — the one legal field-write context
// (E0813).
func (c *checker) checkMethodBody(params []ast.Param, types []Type, body *ast.Block, ret Type, self Type, mutRecv bool) {
	savedRet, savedLocals, savedRecv := c.fnRet, c.locals, c.recvMut
	c.fnRet, c.recvMut = ret, mutRecv
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
	c.fnRet, c.locals, c.recvMut = savedRet, savedLocals, savedRecv
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
				c.checkImplMethodSig(m, im, inf, ifaceArgs, assocs)
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
		view := fnType{params: m.params, ret: retOrUnit(m.ret)}
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
// with the impl's interface arguments and associated bindings; every
// anchor is the method name.
func (c *checker) checkImplMethodSig(m *resolvedMethod, im *ifaceMethod, inf *ifaceInfo, ifaceArgs []Type, assocs map[string]Type) {
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
		want := subst(im.params[i], ifaceArgs, assocs)
		if !sameType(m.params[i], want) {
			c.fail(nl, nc, "E0808", fmt.Sprintf(
				"impl method signature mismatches the interface method — the parameter %q of %q is %s in the impl and %s in %q; signatures match exactly",
				m.fd.Params[i].Name, m.fd.Name, m.params[i].String(), want.String(), inf.name))
		}
	}
	iRet := subst(im.ret, ifaceArgs, assocs)
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
		c.checkMethodBody(fd.Params, c.fnParams[fd], &fd.Body, c.fnRets[fd], info.head, fd.Recv == ast.RecvMutSelf)
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
		if sym, is := c.syms[nt.Qual]; is && sym.kind == symImport {
			c.bnd(bndMultiModule)
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
	switch nt.Name {
	case "Never", "Result", "Option", "Dyn", "List", "Map", "Set", "Range", "Shareable":
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
		if c.implementsFace(args[b.idx], b.face) {
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
		}
		return tupleType{elems: elems}
	case *ast.FnType:
		params := make([]Type, len(x.Params))
		for i, p := range x.Params {
			params[i] = c.resolveTypeRef(p, slotAnn)
		}
		return fnType{params: params, tags: x.EffectTags, ret: c.resolveTypeRef(x.Ret, slotRet)}
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
		return Type(c.dynFace(x.Args))
	case x.Name == "Shareable":
		c.bnd(bndShareable)
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
	it := c.typeOf(b.Init, ann)
	if ann != nil && !agree(it, ann) {
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
	it := c.typeOf(b.Init, ann)
	if ann != nil {
		if !agree(it, ann) {
			line, col := patAnchor(b.Pat)
			c.fail(line, col, "E0501", fmt.Sprintf(
				"mixed types — the expression is %s, the annotation is %s; no coercion is ever inserted",
				it.String(), ann.String()))
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
		c.fail(line, col, "E0501", fmt.Sprintf(
			"mixed types — the return expression is %s, the declared return is %s; no coercion is ever inserted",
			vt.String(), c.fnRet.String()))
	}
}

// checkFnDecl types one fn body: parameters scope their names, the tail
// rule holds the body's final item, and a valued body that produces no
// value reports at the declaration itself.
func (c *checker) checkFnDecl(fd *ast.FnDecl) {
	savedRet, savedLocals, savedRecv, savedBounds := c.fnRet, c.locals, c.recvMut, c.fnBounds
	c.fnRet, c.recvMut, c.fnBounds = c.fnRets[fd], false, c.fnWheres[fd]
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
	c.fnRet, c.locals, c.recvMut, c.fnBounds = savedRet, savedLocals, savedRecv, savedBounds
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
	defer func() { c.locals = c.locals[:len(c.locals)-1] }()
	val := Type(unitType{})
	for i, s := range items {
		last := i == len(items)-1
		switch st := s.(type) {
		case *ast.Binding:
			if st.Pat != nil {
				c.checkLetPattern(st)
				break
			}
			t := c.checkBinding(st)
			if st.Name != "_" {
				c.locals[len(c.locals)-1][st.Name] = t
			}
		case *ast.Assign:
			c.checkAssign(st)
		case *ast.Return:
			c.checkReturn(st)
		case *ast.While:
			c.checkCond(st.Cond, "while condition")
			c.walkItems(st.Body.Items, walkControl)
		case *ast.Loop:
			c.walkItems(st.Body.Items, walkControl)
		case *ast.Defer:
			c.walkItems(st.Block.Items, walkControl)
		case *ast.Break, *ast.Continue:
			// Loop placement is E0201's, held at parse; nothing types here.
		case *ast.ForStmt:
			c.checkFor(st)
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
			// them (a stop already left this loop).
			if dropped := mode != walkFn || !last || c.fnRet == nil; dropped {
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
					line, col := exprPos(st.Expr)
					c.fail(line, col, "E0501", fmt.Sprintf(
						"mixed types — the final expression is %s, the declared return is %s; no coercion is ever inserted",
						t.String(), c.fnRet.String()))
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
			// a qualified variant head is multi-module by construction
			c.bnd(bndMultiModule)
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
			c.checkPattern(a, v.payloads[i], binds, nodes)
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
		keys := make([]ctorKey, len(s.decl.variants))
		payloads := make([][]Type, len(s.decl.variants))
		for i, v := range s.decl.variants {
			keys[i] = ctorKey{variant: v.name}
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
	}
	panic("unreachable expr")
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
	defer func() {
		c.closures = c.closures[:len(c.closures)-1]
		c.closureBounds = c.closureBounds[:len(c.closureBounds)-1]
	}()
	savedRet, savedLocals := c.fnRet, c.locals
	c.locals = append(c.locals, map[string]Type{})
	for i, p := range x.Params {
		if p.Name != "_" {
			c.locals[len(c.locals)-1][p.Name] = params[i]
		}
	}
	ret := Type(unitType{})
	switch {
	case x.Ret != nil:
		c.fnRet = c.resolveTypeRef(x.Ret, slotRet)
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
		ret = c.walkItems(x.Body.Items, walkPlain)
	default:
		c.fnRet = nil
		c.walkItems(x.Body.Items, walkFn)
	}
	c.fnRet, c.locals = savedRet, savedLocals
	return fnType{params: params, ret: ret}
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
	if x.Qual != "" {
		// a qualified head reaches another module's records or fails as a
		// qualifier — either way this build stops before it types
		if sym, ok := c.syms[x.Qual]; ok && sym.kind == symImport {
			c.bnd(bndMultiModule)
		}
		c.fail(x.Line, x.Col, "E1304", fmt.Sprintf(
			"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
			x.Qual, x.Qual+"."+x.Name))
	}
	sym, ok := c.syms[x.Name]
	if !ok {
		if baseNames[x.Name] {
			c.fail(x.Line, x.Col, "E0603", fmt.Sprintf(
				"construction or update head or base is not the record type — the head %q names a base type, not a record; construction and update heads name record types",
				x.Name))
		}
		c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
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
		if !agree(vt, dt) {
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
			return c.variantType(sym.sum, sym.vi, x)
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
			return fnType{params: c.fnParams[sym.fn], ret: ret}
		default:
			// type and import names carry no value
			c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
		}
	}
	switch x.Name {
	case "panic", "todo", "assert":
		c.bnd(bndTermination)
	case "currentCancelSignal", "advanceTime":
		c.bnd(bndTaskTime)
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
func (c *checker) variantType(sum *sumInfo, vi int, x *ast.Ident) Type {
	v := sum.variants[vi]
	if len(v.payloads) > 0 {
		c.fail(x.Line, x.Col, "E0704", fmt.Sprintf(
			"payloaded variant constructor used without arguments — %q carries a payload; construct with the call form %s(...)",
			x.Name, x.Name))
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
	if id, ok := x.Recv.(*ast.Ident); ok && !c.nameResolvable(id.Name) {
		full := id.Name + "." + x.Name
		c.fail(id.Line, id.Col, "E1304", fmt.Sprintf(
			"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
			id.Name, full))
	}
	if x.Name == "forEach" {
		c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
			"no such member on the receiver's type — the receiver's type has no member %q; forEach does not exist: sequence effects belong to the for statement",
			x.Name))
	}
	return c.memberOfType(c.typeOf(x.Recv, nil), x, asCall)
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
			return substFn(fnType{params: im.params, ret: retOrUnit(im.ret)}, t.args, nil)
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
			return substFn(fnType{params: im.params, ret: retOrUnit(im.ret)}, t.args, nil)
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
					return substFn(fnType{params: im.params, ret: retOrUnit(im.ret)}, b.face.args, nil)
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
		if sym, is := c.syms[nt.Qual]; is && sym.kind == symImport {
			c.bnd(bndMultiModule)
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
		c.bnd(bndShareable)
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
				return c.fnCall(sym.fn, x)
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
			c.bnd(bndTermination)
		case "currentCancelSignal", "advanceTime":
			c.bnd(bndTaskTime)
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
	if m, is := x.Fn.(*ast.Member); is {
		head = m
		t = c.memberType(m, true)
	} else {
		t = c.typeOf(x.Fn, nil)
	}
	ft, isFn := t.(fnType)
	if !isFn {
		c.bnd(bndCalleeGap)
	}
	if head != nil && fnContainsParam(ft) {
		// a method view still holding clause positions: the arguments
		// determine them (design D6's method generics). A fn-typed
		// value's parameters are the enclosing scope's positions, never
		// a callee's to determine — that path stays positional.
		return c.methodCall(ft, head, x)
	}
	return c.fnValueCall(ft, x)
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

// methodCall types one call through a method view still holding clause
// positions (design D6's method generics): a method call carries no
// explicit type-argument form, so the arguments determine the positions
// — an open position after every argument is E0827 at the member name,
// and each argument agrees with its determined parameter (E0501).
func (c *checker) methodCall(ft fnType, m *ast.Member, x *ast.Call) Type {
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
		// unification (chapter 11: U comes from the call's own text).
		argTypes[i] = c.typeOf(a, ft.params[i])
		c.inferUnify(ft.params[i], argTypes[i], bindings, a)
	}
	if missing := openRefs(ft, bindings); len(missing) > 0 {
		c.fail(m.NameLine, m.NameCol, "E0827", fmt.Sprintf(
			"generic call does not determine its type arguments — the call of %q determines no %q; a method call carries no explicit type-argument form: restructure the call so its arguments determine it",
			m.Name, missing[0].name))
	}
	for i, a := range x.Args {
		// Both sides substitute before comparing: an argument typed under
		// the threading may still hold the clause's own positions (a bare
		// closure's view), and those positions are the determination's —
		// the substituted sides compare like for like.
		if want := substMap(ft.params[i], bindings); !agree(substMap(argTypes[i], bindings), want) {
			line, col := exprPos(a)
			c.fail(line, col, "E0501", fmt.Sprintf(
				"mixed types — the argument is %s, the parameter is %s; no coercion is ever inserted",
				argTypes[i].String(), want.String()))
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
// clause holds at every application (E0830, at the call head).
func (c *checker) fnCall(fd *ast.FnDecl, x *ast.Call) Type {
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
			c.fail(line, col, "E0501", fmt.Sprintf(
				"mixed types — the argument is %s, the parameter is %s; no coercion is ever inserted",
				at.String(), params[i].String()))
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
