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
	bndFnValues    = "function values (chapter 12)"
	bndRange       = "range expressions (chapter 11)"
	bndStdModules  = "standard-library modules (chapter 15)"
	bndMultiModule = "multi-module programs (chapter 15)"
	bndDomainGap   = "arithmetic and comparisons beyond the ratified numeric and Bool domains (spec gap; roadmap follow-up)"
	bndArityGap    = "calls with an argument count the callee does not declare (spec gap; roadmap follow-up)"
	bndCalleeGap   = "calls on values that are not functions (spec gap; roadmap follow-up)"
)

// tHelps carries each code's remediation from the registry
// (docs/spec/diagnostics.toml). One home until the registry embed lands
// (roadmap follow-up 2).
var tHelps = map[string]string{
	"E0501": "Make the types agree at the source: use a literal suffix (42i32) or an explicit conversion method (toInt64(), toBytes()); the method inventory is the standard library's.",
	"E0502": "Widen the type (suffix or annotation), restructure the computation, or opt into wrapping explicitly (wrappingAdd() and siblings).",
	"E0605": "Bind the value to use it, chain it onward, declare a return type, or discard it explicitly with `let _ = expr`.",
	"E0703": "Drop the Never annotation, or move it to a function's return type where a non-producing path is genuinely declared.",
	"E0704": "Construct with the payload (`Circle(1.0)`); first-class constructor values arrive, if at all, with the function-types chapter.",
	"E0827": "Write the explicit form: `empty<Int64>()`, `Pair<Int64, String> { ... }`.",
	"E0828": "Match the declaration's arity, or fix the declaration's clause.",
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
// this slice), and variants with resolved payload types (nil payloads =
// a unit variant).
type sumInfo struct {
	name     string
	params   int
	variants []variantInfo
}

type variantInfo struct {
	name     string
	payloads []Type
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
)

type symbol struct {
	kind    symKind
	fn      *ast.FnDecl
	letType Type // nil until the binding's initializer is typed (source order)
	sum     *sumInfo
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
			sum := &sumInfo{name: x.Name, params: 0}
			for _, v := range x.Variants {
				sum.variants = append(sum.variants, variantInfo{name: v.Name})
			}
			c.syms[x.Name] = &symbol{kind: symType, sum: sum}
			for i, v := range x.Variants {
				c.syms[v.Name] = &symbol{kind: symVariant, sum: sum, vi: i}
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
					payloads = append(payloads, c.resolveTypeRef(tr, slotAnn))
				}
				sum.variants[i].payloads = payloads
			}
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

// checkAssign types one assignment against the target binding's type,
// anchoring at the target's name token.
func (c *checker) checkAssign(a *ast.Assign) {
	t, ok := c.lookupValue(a.Name)
	if !ok {
		c.fail(a.Line, a.Col, "E1304", bareUnresolved(a.Name))
	}
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
	c.walkItems(fd.Body.Items, true, valued)
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

// walkItems types one block's items and returns the block's value: the
// final expression item's type, unit otherwise. fnBody marks the fn's
// own block (its tail faces the declared return; a valueless fn's tail
// faces the E0605 discard rule with the fn's shape).
func (c *checker) walkItems(items []ast.Stmt, fnBody, fnValued bool) Type {
	c.locals = append(c.locals, map[string]Type{})
	defer func() { c.locals = c.locals[:len(c.locals)-1] }()
	val := Type(unitType{})
	for i, s := range items {
		last := i == len(items)-1
		tail := fnBody && last
		switch st := s.(type) {
		case *ast.Binding:
			t := c.checkBinding(st)
			if st.Name != "_" {
				c.locals[len(c.locals)-1][st.Name] = t
			}
		case *ast.Assign:
			c.checkAssign(st)
		case *ast.Return:
			c.checkReturn(st)
		case *ast.ExprStmt:
			t := c.typeOf(st.Expr, nil)
			// A dropped value outside the unit type is E0605: the fn's own
			// valueless tail names the fn's escape (declare a return
			// type), an inner statement does not. A plain block's final
			// expression is its value — it is never dropped here.
			if _, never := t.(neverType); !never && !isUnit(t) {
				line, col := exprPos(st.Expr)
				switch {
				case tail && !fnValued:
					c.fail(line, col, "E0605", fmt.Sprintf(
						"non-unit value dropped — the final expression is %s, not (); bind it, chain it onward, declare a return type, or discard it explicitly with let _ = expr",
						t.String()))
				case !tail:
					c.fail(line, col, "E0605", fmt.Sprintf(
						"non-unit value dropped — the statement's expression is %s, not (); bind it, chain it onward, or discard it explicitly with let _ = expr",
						t.String()))
				}
			}
			if last {
				val = t
				if fnValued && !agree(t, c.fnRet) {
					line, col := exprPos(st.Expr)
					c.fail(line, col, "E0501", fmt.Sprintf(
						"mixed types — the final expression is %s, the declared return is %s; no coercion is ever inserted",
						t.String(), c.fnRet.String()))
				}
			}
		}
	}
	return val
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
		return c.walkItems(x.Block.Items, false, false)
	}
	panic("unreachable expr")
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
	if t, ok := c.lookupLocal(x.Name); ok {
		return t
	}
	if sym, ok := c.syms[x.Name]; ok {
		switch sym.kind {
		case symLet:
			if sym.letType == nil {
				c.fail(x.Line, x.Col, "E1304", bareUnresolved(x.Name))
			}
			return sym.letType
		case symVariant:
			return c.variantType(sym.sum, sym.vi, x)
		default:
			// fn names as values are chapter 12's; type and import names
			// carry no value
			if sym.kind == symFn {
				c.bnd(bndFnValues)
			}
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

// memberType types one member access. An unresolvable bare receiver
// identifier reads as a failed qualifier (E1304 at the receiver); a
// resolved receiver's method inventory is the standard library's.
func (c *checker) memberType(x *ast.Member) Type {
	if id, ok := x.Recv.(*ast.Ident); ok && !c.nameResolvable(id.Name) {
		full := id.Name + "." + x.Name
		c.fail(id.Line, id.Col, "E1304", fmt.Sprintf(
			"unresolved name — the qualifier %q of %q is not an import name; qualify through an existing import name",
			id.Name, full))
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
			if _, isFn := t.(fnType); !isFn {
				c.bnd(bndCalleeGap)
			}
			c.bnd(bndFnValues) // a fn-typed value is chapter 12's
		}
		if sym, ok := c.syms[id.Name]; ok {
			switch sym.kind {
			case symFn:
				return c.fnCall(sym.fn, x)
			case symVariant:
				return c.ctorCall(sym.sum, sym.vi, x)
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
	// a non-name callee: typed, then judged a function or not
	t := c.typeOf(x.Fn, nil)
	if _, isFn := t.(fnType); !isFn {
		c.bnd(bndCalleeGap)
	}
	c.bnd(bndFnValues) // calling a fn-typed value is chapter 12's
	panic("unreachable call")
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
