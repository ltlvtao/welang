// Package typecheck implements the type stage over the parsed module:
// chapter 7's type references resolve into types, chapter 9's sum types
// and their constructor mechanism check, chapter 15's main convention
// holds the root module, and the base-type literals and operators of the
// types chapter carry the no-coercion agreement rule (E0501) with the
// constant-folding overflow check (E0502). Name resolution walks the
// three layers — block locals, the module namespace, the prelude — and
// the prelude's not-yet-implemented names stop at honest boundaries,
// never at E1304. The stage stops at the first diagnostic or boundary
// (chapter 21: an E-severity diagnostic stops the pipeline).
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
	bndCollections = "std collection types (chapter 17)"
	bndTermination = "termination functions (chapter 14)"
	bndDyn         = "Dyn boxes (chapter 10)"
	bndShareable   = "Shareable markers (chapter 18)"
	bndTaskTime    = "task-scope and time-control functions (chapters 18 and 20)"
	bndRange       = "range expressions (chapter 11)"
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

// paramRef names a type parameter position of a builtin sum (Result's
// and Option's payloads); it materializes only through the expected
// type's arguments, so it never renders.
type paramRef int

func (p paramRef) String() string { return fmt.Sprintf("T%d", int(p)) }

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

// namedType is one declared sum applied to its type arguments (zero in
// this slice — generic declarations stop at chapter 10's parse boundary,
// so user sums are monomorphic and only the builtin sums carry args).
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

// recordType is one declared record; the declaration owns the field list
// and the ownership category, so the type is a reference to it (design
// D6: one authority per fact).
type recordType struct{ decl *recordInfo }

func (r recordType) String() string { return r.decl.name }

// newtypeType is one declared newtype: a zero-cost wrapper whose category
// and member follow its underlying type.
type newtypeType struct{ decl *newtypeInfo }

func (n newtypeType) String() string { return n.decl.name }

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
		return ok && x == y
	case recordType:
		y, ok := b.(recordType)
		return ok && x.decl == y.decl
	case newtypeType:
		y, ok := b.(newtypeType)
		return ok && x.decl == y.decl
	}
	return false
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

// sumInfo is one declared sum: its name, type parameter count (zero in
// this slice), its byval flag (the value-category declaration), and
// variants with resolved payload types (nil payloads = a unit variant).
type sumInfo struct {
	name     string
	params   int
	byval    bool
	variants []variantInfo
}

type variantInfo struct {
	name     string
	payloads []Type
}

// recordInfo is one declared record (chapter 8): its ownership category
// and its fields with resolved types. cat is the declared prefix:
// "value" (byval record), "resource" (byres record), "gc" (default).
type recordInfo struct {
	name   string
	cat    string
	fields []fieldInfo
}

type fieldInfo struct {
	name string
	typ  Type
}

// newtypeInfo is one declared newtype: the name and the resolved
// underlying type (the wrapper's layout is erased; the category and the
// one member "value" derive from it).
type newtypeInfo struct {
	name       string
	underlying Type
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

// The builtin sums (chapter 15's prelude): Result with Ok and Err over
// the parameter positions, Option with Some and None over its one.
var (
	resultSum = &sumInfo{
		name:   "Result",
		params: 2,
		variants: []variantInfo{
			{name: "Ok", payloads: []Type{paramRef(0)}},
			{name: "Err", payloads: []Type{paramRef(1)}},
		},
	}
	optionSum = &sumInfo{
		name:   "Option",
		params: 1,
		variants: []variantInfo{
			{name: "Some", payloads: []Type{paramRef(0)}},
			{name: "None"},
		},
	}
)

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
)

type symbol struct {
	kind    symKind
	fn      *ast.FnDecl
	letType Type // nil until the binding's initializer is typed (source order)
	sum     *sumInfo
	rec     *recordInfo
	nt      *newtypeInfo
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
		file:     file,
		mode:     mode,
		syms:     map[string]*symbol{},
		fnParams: map[*ast.FnDecl][]Type{},
		fnRets:   map[*ast.FnDecl]Type{},
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

// checkModule walks the module in the pipeline's order: symbol
// registration, import resolution (module resolution precedes type
// checking, chapter 21's R2), the main convention (project mode), the
// annotation pass, then the binding initializers and fn bodies in source
// order.
func (c *checker) checkModule(f *ast.File) {
	// Pass 1: register the module's names (the parser has verified the
	// one-module-one-name-space rule).
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
			sum := &sumInfo{name: x.Name, params: 0, byval: x.Byval}
			for _, v := range x.Variants {
				sum.variants = append(sum.variants, variantInfo{name: v.Name})
			}
			c.syms[x.Name] = &symbol{kind: symType, sum: sum}
			for i, v := range x.Variants {
				c.syms[v.Name] = &symbol{kind: symVariant, sum: sum, vi: i}
			}
		case *ast.RecordDecl:
			c.syms[x.Name] = &symbol{kind: symRecord, rec: &recordInfo{name: x.Name, cat: x.Cat}}
		case *ast.NewtypeDecl:
			c.syms[x.Name] = &symbol{kind: symNewtype, nt: &newtypeInfo{name: x.Name}}
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
	// Pass 2a: resolve every annotation slot (variant payloads, fn
	// parameters and returns, top-level binding annotations).
	for _, it := range f.Items {
		switch x := it.(type) {
		case *ast.SumDecl:
			sum := c.syms[x.Name].sum
			for i, v := range x.Variants {
				var payloads []Type
				for _, tr := range v.Payload {
					pt := c.resolveTypeRef(tr, slotAnn)
					// A value sum's payloads face E0702's category honesty
					// (an M3 gap closed with the composite chapter).
					if sum.byval && catOf(pt) != "value" {
						line, col := refPos(tr)
						c.fail(line, col, "E0702", fmt.Sprintf(
							"value sum payload is not of the value category or a base type — the payload of %q is %q, a %s; a copy is only honest when everything in it is copyable by value",
							v.Name, pt.String(), catNoun(pt)))
					}
					payloads = append(payloads, pt)
				}
				sum.variants[i].payloads = payloads
			}
		case *ast.RecordDecl:
			rec := c.syms[x.Name].rec
			for _, f := range x.Fields {
				ft := c.resolveTypeRef(f.Typ, slotAnn)
				// E0601's category honesty anchors at the field's type
				// reference — the fact the copy would betray.
				if rec.cat == "value" && catOf(ft) != "value" {
					line, col := refPos(f.Typ)
					c.fail(line, col, "E0601", fmt.Sprintf(
						"value record field is not of the value category or a base type — the field %q is %q, a %s; a copy is only honest when everything in it is copyable by value",
						f.Name, ft.String(), catNoun(ft)))
				}
				rec.fields = append(rec.fields, fieldInfo{name: f.Name, typ: ft})
			}
		case *ast.NewtypeDecl:
			c.syms[x.Name].nt.underlying = c.resolveTypeRef(x.Underlying, slotAnn)
		case *ast.FnDecl:
			var params []Type
			for _, p := range x.Params {
				params = append(params, c.resolveTypeRef(p.Type, slotAnn))
			}
			c.fnParams[x] = params
			if x.Ret != nil {
				c.fnRets[x] = c.resolveTypeRef(x.Ret, slotRet)
			}
		case *ast.TopLet:
			if x.Binding.Typ != nil && x.Binding.Name != "_" {
				c.syms[x.Binding.Name].letType = c.resolveTypeRef(x.Binding.Typ, slotAnn)
			}
		}
	}
	// Pass 2b: top-level binding initializers, then fn bodies — source
	// order both, so a module binding references only earlier ones.
	for _, it := range f.Items {
		if x, ok := it.(*ast.TopLet); ok {
			t := c.checkBinding(&x.Binding)
			if x.Binding.Name != "_" {
				c.syms[x.Binding.Name].letType = t
			}
		}
	}
	for _, it := range f.Items {
		if x, ok := it.(*ast.FnDecl); ok {
			c.checkFnDecl(x)
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
	// The module namespace shadows the prelude.
	if sym, ok := c.syms[x.Name]; ok {
		switch sym.kind {
		case symType:
			c.checkArity(x, sym.sum.name, sym.sum.params)
			return namedType{decl: sym.sum}
		case symRecord:
			c.checkArity(x, sym.rec.name, 0)
			return recordType{decl: sym.rec}
		case symNewtype:
			c.checkArity(x, sym.nt.name, 0)
			return newtypeType{decl: sym.nt}
		default:
			// a value name held by the module fills no type slot
			c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
		}
	}
	switch {
	case x.Name == "Never":
		c.checkArity(x, "Never", 0)
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
	case x.Name == "List" || x.Name == "Map" || x.Name == "Set":
		c.bnd(bndCollections)
	case x.Name == "Dyn":
		c.bnd(bndDyn)
	case x.Name == "Shareable":
		c.bnd(bndShareable)
	case baseNames[x.Name]:
		c.checkArity(x, x.Name, 0)
		return baseType(x.Name)
	}
	c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
	panic("unreachable name")
}

// checkArity compares a generic application's argument count with the
// declaration's, anchored at the application's own `<` token.
func (c *checker) checkArity(x *ast.NamedType, name string, params int) {
	if len(x.Args) == params {
		return
	}
	line, col := x.ArgLine, x.ArgCol
	if line == 0 {
		line, col = x.Line, x.Col
	}
	noun := "type arguments"
	if params == 1 {
		noun = "type argument"
	}
	c.fail(line, col, "E0828", fmt.Sprintf(
		"type argument arity mismatch — %q wants %d %s, got %d; match the declaration's arity",
		name, params, noun, len(x.Args)))
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
// boundary faces the capture ledger first (E1003/E1002).
func (c *checker) checkAssign(a *ast.Assign) {
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
	savedRet, savedLocals := c.fnRet, c.locals
	c.fnRet = c.fnRets[fd]
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
	c.fnRet, c.locals = savedRet, savedLocals
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
		case *ast.ExprStmt:
			t := c.typeOf(st.Expr, nil)
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
			c.bnd(bndRange)
		}
		if foldOp(x.Op) && constInt(x) {
			return c.constType(x)
		}
		return c.binaryType(x)
	case *ast.Call:
		return c.callType(x, expected)
	case *ast.Member:
		return c.memberType(x)
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
			if !agree(pt, et.params[i]) {
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
	rt := recordType{decl: rec}
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
		declared[f.name] = f.typ
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
			// 12); a valueless fn produces the unit type.
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

// memberType types one member access (design D10): records resolve by
// their declared fields and newtypes by their single member "value";
// every other name on those receivers is E0816 at the member name. An
// unresolvable bare receiver identifier reads as a failed qualifier
// (E1304 at the receiver); any other receiver's method inventory is the
// standard library's (the boundary invariant this milestone keeps).
func (c *checker) memberType(x *ast.Member) Type {
	if id, ok := x.Recv.(*ast.Ident); ok && !c.nameResolvable(id.Name) {
		full := id.Name + "." + x.Name
		c.fail(id.Line, id.Col, "E1304", fmt.Sprintf(
			"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
			id.Name, full))
	}
	switch t := c.typeOf(x.Recv, nil).(type) {
	case recordType:
		for _, f := range t.decl.fields {
			if f.name == x.Name {
				return f.typ
			}
		}
		c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
			"no such member on the receiver's type — %q has no member %q; member access names a field or a method of the receiver's type",
			t.decl.name, x.Name))
	case newtypeType:
		if x.Name == "value" {
			return t.decl.underlying
		}
		c.fail(x.NameLine, x.NameCol, "E0816", fmt.Sprintf(
			"no such member on the receiver's type — %q has no member %q; a newtype's single member is \"value\"",
			t.decl.name, x.Name))
	}
	c.bnd(bndStdModules)
	panic("unreachable member")
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
		"List", "Map", "Set", "Dyn", "Shareable":
		return true
	}
	return baseNames[name]
}

// callType types one call. Callee-name resolution precedes everything
// (E1304); the prelude's boundary names follow; then the argument count
// (the spec-gap boundary) and the arguments against the parameters.
func (c *checker) callType(x *ast.Call, expected Type) Type {
	if id, ok := x.Fn.(*ast.Ident); ok {
		if t, isLocal := c.lookupLocal(id.Name); isLocal {
			ft, isFn := t.(fnType)
			if !isFn {
				c.bnd(bndCalleeGap)
			}
			return c.fnValueCall(ft, x)
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
		}
		c.fail(id.Line, id.Col, "E1304", bareUnresolved(id.Name))
	}
	// a non-name callee: typed, then called through its fn type
	t := c.typeOf(x.Fn, nil)
	ft, isFn := t.(fnType)
	if !isFn {
		c.bnd(bndCalleeGap)
	}
	return c.fnValueCall(ft, x)
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

// fnCall types one call to a module fn.
func (c *checker) fnCall(fd *ast.FnDecl, x *ast.Call) Type {
	params := c.fnParams[fd]
	if len(x.Args) != len(params) {
		c.bnd(bndArityGap)
	}
	c.checkArgs(x.Args, params)
	if fd.Ret == nil {
		return unitType{}
	}
	return c.fnRets[fd]
}

// ctorCall types one call to a user variant constructor.
func (c *checker) ctorCall(sum *sumInfo, vi int, x *ast.Call) Type {
	v := sum.variants[vi]
	if len(x.Args) != len(v.payloads) {
		c.bnd(bndArityGap)
	}
	c.checkArgs(x.Args, v.payloads)
	return namedType{decl: sum}
}

// newtypeCall types the call form of a newtype construction (chapter 8):
// exactly one argument checked against the underlying type; the wrapper
// is the result.
func (c *checker) newtypeCall(nt *newtypeInfo, x *ast.Call) Type {
	if len(x.Args) != 1 {
		c.bnd(bndArityGap)
	}
	c.checkArgs(x.Args, []Type{nt.underlying})
	return newtypeType{decl: nt}
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
