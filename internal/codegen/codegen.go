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

// The boundary Whats. bndMainBody is M8's widened vocabulary (design D4);
// the others ride unchanged from M4.
const (
	bndMainBody   = "main bodies beyond let bindings, io calls, and a single Ok or Err return statement"
	bndErrPayload = "Err payloads beyond one plain string-literal variant argument"
	bndOtherFns   = "functions other than main in code generation"
	bndTopLets    = "top-level value bindings in code generation"
)

func bndMain() *NotImplemented   { return &NotImplemented{What: bndMainBody} }
func bndErrPay() *NotImplemented { return &NotImplemented{What: bndErrPayload} }

// The runtime symbols emission can reference, in the declare section's
// fixed order (alloc, the root pair, the io pair, the fail tail).
var declareLines = []struct{ sym, line string }{
	{"__we_alloc", "declare ptr @__we_alloc(i64)"},
	{"__we_root_push", "declare void @__we_root_push(ptr)"},
	{"__we_root_pop", "declare void @__we_root_pop()"},
	{"__we_println", "declare void @__we_println(ptr, i64)"},
	{"__we_print", "declare void @__we_print(ptr, i64)"},
	{"__we_fail", "declare void @__we_fail(ptr, i64) noreturn"},
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
// pool, the instruction stream, and the fresh-value counter.
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
}

func (e *emitter) inst(s string)  { e.body.WriteString("  " + s + "\n") }
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
		case *ast.Import:
			// Only the std segment erases (design D4): a std import item
			// leaves no IR trace and enables the io calls; any other
			// import means the emitted program would span modules — the
			// honest stop is the other-functions row.
			if len(d.Path) > 0 && d.Path[0] == "std" {
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
		strEnv:   make(map[string]strBinding),
		gcEnv:    make(map[string]gcBinding),
		strPool:  make(map[string]string),
		usedRecs: make(map[string]bool),
		declUsed: make(map[string]bool),
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

// emitStmt emits one non-tail statement of the accepted sequence: a let
// binding (String literal or gc construction; `_` discards), or an io
// call as an expression statement.
func (e *emitter) emitStmt(st ast.Stmt) *NotImplemented {
	switch s := st.(type) {
	case *ast.Binding:
		if s.Kw != "let" || s.Pat != nil {
			return bndMain()
		}
		switch init := s.Init.(type) {
		case *ast.Literal:
			// String literals only: int and bool literals ride the
			// construction-argument positions, not binding positions.
			if init.Kind != "string" {
				return bndMain()
			}
			data, ok := decodeStringLiteral(init.Text)
			if !ok {
				return bndMain() // interpolation has no M8 emission
			}
			if s.Name != "_" {
				e.strEnv[s.Name] = strBinding{data: data, length: len(data)}
			}
			return nil
		case *ast.Construct:
			reg, ni := e.emitConstruct(init)
			if ni != nil {
				return ni
			}
			if s.Name != "_" {
				e.gcEnv[s.Name] = gcBinding{rec: init.Name, reg: reg}
			}
			return nil
		case *ast.Call:
			// An io call binds only as the discard (`let _ =`).
			if s.Name != "_" {
				return bndMain()
			}
			return e.emitIoCall(init)
		default:
			return bndMain()
		}
	case *ast.ExprStmt:
		call, ok := s.Expr.(*ast.Call)
		if !ok {
			return bndMain()
		}
		return e.emitIoCall(call)
	default:
		return bndMain()
	}
}

// emitIoCall emits `qual.println(arg)` / `qual.print(arg)`. The qualifier
// is any name — resolution happened at the type stage; a qualifier that
// main's body binds locally is a member call on that value instead (a
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
	if _, local := e.strEnv[qual.Name]; local {
		return bndMain()
	}
	if _, local := e.gcEnv[qual.Name]; local {
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
