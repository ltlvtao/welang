package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// B2b T4's face: the List member calls ride the carrier where the
// carrier already answers (push for add, len for size) and the coll
// entries' out trio where the surface semantics differ from the walks'
// (get answers absence, never the carrier's trap; removeAt shifts the
// live tail). The pins run through the real check pipeline (checkShapes)
// for the same reason fs's do: the face is about what the checker
// already accepts, and the classification tower's hole arm is part of
// it — a size call renders in a String position the way byteLength
// always has.

// listSrc is one program whose main body is body over the io import —
// the skeleton every pin varies.
func listSrc(body string) string {
	return `import std.io

pub type AppError = Failed(String)

pub fn main() effect io -> Result<(), AppError> {
` + body + `    return Ok(())
}
`
}

// emitListFace checks and emits one program, refusing boundaries: these
// programs build, so a boundary here is a failure of the fixture or of
// the face, never a fact to pin.
func emitListFace(t *testing.T, src string) string {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q over the List member face:\n%s", ni.What, src)
	}
	return ir
}

// listFaceStop checks and emits one program, answering its boundary: the
// negative pins' helper.
func listFaceStop(t *testing.T, src string) *NotImplemented {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	return ni
}

// countLines is the count of IR lines containing frag — the walk-versus-
// member entry split counts occurrences rather than trusting one.
func countLines(ir, frag string) int {
	n := 0
	for _, line := range strings.Split(ir, "\n") {
		if strings.Contains(line, frag) {
			n++
		}
	}
	return n
}

// TestListOptionMembersEmitsTheOutTrio: get and removeAt ride the coll
// entries' trailing [3 x i64] out block — the fs family's shape — and
// the three gep loads at 0/8/16 land in three fresh i64 slots, which is
// the fused Option's whole slot shape, built from words the C side wrote
// with None 0 / Some 1.
func TestListOptionMembersEmitsTheOutTrio(t *testing.T) {
	for _, member := range []string{"get", "removeAt"} {
		ir := emitListFace(t, listSrc(`    let xs: List<Int64> = [10, 20]
    match xs.`+member+`(1) {
        Some(v) => { io.println("hit") }
        None => { io.println("none") }
    }
`))
		sym := "__we_coll_list_get"
		if member == "removeAt" {
			sym = "__we_coll_list_remove_at"
		}
		wantIR(t, ir, "declare void @"+sym+"(ptr, i64, ptr)", member+"'s declare row")
		out := definedReg(t, ir, "= alloca [3 x i64]")
		line := ""
		for _, l := range strings.Split(ir, "\n") {
			if strings.Contains(l, "call void @"+sym+"(") {
				line = l
				break
			}
		}
		if line == "" {
			t.Fatalf("%s: IR missing the entry call\n--\n%s", member, ir)
		}
		if !strings.HasSuffix(strings.TrimSpace(line), "ptr "+out+")") {
			t.Fatalf("%s: the out block is not the call's trailing operand:\n%s", member, line)
		}
		for _, off := range []string{"0", "8", "16"} {
			wantIR(t, ir, "getelementptr i8, ptr "+out+", i64 "+off, "the trio's word at +"+off)
		}
	}
}

// TestListGetRidesTheSurfaceEntryNotTheCarrier: one program walks the
// carrier and reads a member — the walk keeps the carrier's own
// __we_list_get (its lengths are inside by construction), the member
// takes the coll entry (an out-of-range read is absence), and neither
// takes the other's symbol.
func TestListGetRidesTheSurfaceEntryNotTheCarrier(t *testing.T) {
	ir := emitListFace(t, listSrc(`    let xs: List<Int64> = [10, 20]
    for x in xs {
        io.println("w")
    }
    match xs.get(1) {
        Some(v) => { io.println("hit") }
        None => { io.println("none") }
    }
`))
	if n := countLines(ir, "call void @__we_coll_list_get("); n != 1 {
		t.Fatalf("the member's surface entry appears %d times, want 1:\n%s", n, ir)
	}
	if n := countLines(ir, "call i64 @__we_list_get("); n != 1 {
		t.Fatalf("the walk's carrier entry appears %d times, want 1:\n%s", n, ir)
	}
	wantIR(t, ir, "declare i64 @__we_list_get(ptr, i64)", "the carrier's declare row stays")
}

// TestListAddReusesTheCarrierPush: add rides __we_list_push with its
// result dropped — the handle is stable, so the pointer the call
// answers is the one the binding already holds — and the element word
// crosses as the carrier's own element pipeline spells it.
func TestListAddReusesTheCarrierPush(t *testing.T) {
	ir := emitListFace(t, listSrc(`    let xs: List<Int64> = [10]
    xs.add(20)
    io.println("n:${xs.size()}")
`))
	if n := countLines(ir, "call ptr @__we_list_push("); n != 2 {
		t.Fatalf("push appears %d times (the literal's own and the member's), want 2:\n%s", n, ir)
	}
	wantIR(t, ir, "call i64 @__we_list_len(", "size rides the carrier's len")
	wantIR(t, ir, "__we_str_of_i64", "the hole renders the size through the i64 converter")
}

// TestListMemberStringPayloadStops: a String element's Some payload is
// the pair the one payload word cannot hold, so get on a List<String>
// keeps the honest stop — the face's own disclosed boundary, the same
// row the reduce and find payload faces take.
func TestListMemberStringPayloadStops(t *testing.T) {
	ni := listFaceStop(t, listSrc(`    let xs: List<String> = ["a"]
    match xs.get(0) {
        Some(v) => { io.println("hit") }
        None => { io.println("none") }
    }
`))
	if ni == nil {
		t.Fatal("a String element's Option payload crossed a one-word slot")
	}
}

// B2b T5's face: the keyed constructors replace their call sites with
// the runtime entries, the domain tags read off the faces the operands
// carry, and the seven Map and Set members ride the same element
// pipeline the List pair does. The pins keep the T4 discipline — the
// real check pipeline, because the constructor's generic clause is the
// checker's own inference (the Site registration below pins that
// separately), and IR fragments rather than whole-file goldens.

// collSrc is listSrc's skeleton over both std imports: the constructor
// face needs collections beside io.
func collSrc(body string) string {
	return `import std.io
import std.collections

pub type AppError = Failed(String)

pub fn main() effect io -> Result<(), AppError> {
` + body + `    return Ok(())
}
`
}

// emitCollFace is emitListFace's twin for the keyed face.
func emitCollFace(t *testing.T, src string) string {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q over the keyed face:\n%s", ni.What, src)
	}
	return ir
}

// collFaceStop is listFaceStop's twin for the keyed face.
func collFaceStop(t *testing.T, src string) *NotImplemented {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	return ni
}

// findCtorCall walks main's body for the constructor call node — the
// Site registry keys the call node itself, so the pin has to hand At the
// very node the parse built.
func findCtorCall(t *testing.T, src, name string) (ast.Expr, *typecheck.Shapes) {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	var found ast.Expr
	for _, it := range f.Items {
		fn, ok := it.(*ast.FnDecl)
		if !ok || fn.Name != "main" {
			continue
		}
		for _, st := range fn.Body.Items {
			b, ok := st.(*ast.Binding)
			if !ok {
				continue
			}
			if c, ok := b.Init.(*ast.Call); ok {
				if m, ok := c.Fn.(*ast.Member); ok && m.Name == name {
					found = c
				}
			}
		}
	}
	if found == nil {
		t.Fatalf("no %s call in:\n%s", name, src)
	}
	return found, sh
}

// TestCollCtorRegistersItsSite: the checker's own registration, pinned
// where the emitter never reads it — the faces come from the operands,
// the one-authority ruling — so this row is what says the inference seam
// (design D3-3) resolves a qualified generic std call the way a user
// module's is: K and V unified from the two lists' element types, the
// return the application Map<K, V>.
func TestCollCtorRegistersItsSite(t *testing.T) {
	node, sh := findCtorCall(t, collSrc(`    let m = collections.mapOf([1, 2], ["a", "b"])
`), "mapOf")
	site, ok := sh.At(node)
	if !ok {
		t.Fatal("mapOf's call registered no site")
	}
	if len(site.Args) != 2 {
		t.Fatalf("mapOf site bound %d clause positions, want 2 (K, V)", len(site.Args))
	}
	if site.Args[0].Pos != 0 || site.Args[0].Shape.Name != "Int64" {
		t.Fatalf("K bound to %+v, want Int64 at 0", site.Args[0])
	}
	if site.Args[1].Pos != 1 || site.Args[1].Shape.Name != "String" {
		t.Fatalf("V bound to %+v, want String at 1", site.Args[1])
	}
	if site.Ret.Decl.Name != "Map" || len(site.Ret.Args) != 2 {
		t.Fatalf("mapOf returns %+v, want the Map application", site.Ret)
	}

	node, sh = findCtorCall(t, collSrc(`    let s = collections.setOf([1, 2, 3])
`), "setOf")
	site, ok = sh.At(node)
	if !ok {
		t.Fatal("setOf's call registered no site")
	}
	if len(site.Args) != 1 || site.Args[0].Shape.Name != "Int64" {
		t.Fatalf("setOf bound %+v, want one Int64 position", site.Args)
	}
	if site.Ret.Decl.Name != "Set" || len(site.Ret.Args) != 1 {
		t.Fatalf("setOf returns %+v, want the Set application", site.Ret)
	}
}

// TestCollCtorEmitsDomainTagsAndReRoots: the domain words are i64
// immediates the call line itself spells — kdom names the key domain (1
// for the String box handle, 0 for the identity eight and Bool), vtrace
// whether the value word holds a reference the collector traces — and
// the answer is re-rooted before anything else can allocate, the line
// the T12 return discipline made canonical.
func TestCollCtorEmitsDomainTagsAndReRoots(t *testing.T) {
	ir := emitCollFace(t, collSrc(`    let m = collections.mapOf(["a", "b"], [1, 2])
    let s = collections.setOf([1, 2, 3])
`))
	lines := strings.Split(ir, "\n")
	for _, row := range []struct {
		call string
		tags string
	}{
		{"call ptr @__we_coll_map_of(", ", i64 1, i64 0)"},
		{"call ptr @__we_coll_set_of(", ", i64 0)"},
	} {
		at := -1
		for i, l := range lines {
			if strings.Contains(l, row.call) {
				at = i
				break
			}
		}
		if at < 0 {
			t.Fatalf("IR missing %q:\n%s", row.call, ir)
		}
		if !strings.HasSuffix(strings.TrimSpace(lines[at]), row.tags) {
			t.Fatalf("the domain tags read %q, want the line to end %q:\n%s", lines[at], row.tags, ir)
		}
		// The very next instruction re-roots the handle — nothing sits
		// between the carve and the push.
		if next := strings.TrimSpace(lines[at+1]); next != "call void @__we_root_push(ptr %"+regOf(lines[at])+")" {
			t.Fatalf("the ctor answer is not re-rooted on the next line (%q):\n%s", next, ir)
		}
	}
	wantIR(t, ir, "declare ptr @__we_coll_map_of(ptr, ptr, i64, i64)", "mapOf's declare row")
	wantIR(t, ir, "declare ptr @__we_coll_set_of(ptr, i64)", "setOf's declare row")
}

// regOf reads a call line's result register (%vN = call ...).
func regOf(line string) string {
	l := strings.TrimSpace(line)
	l = strings.TrimPrefix(l, "%")
	return l[:strings.Index(l, " ")]
}

// TestCollMemberFaces: the seven members ride their rows — put and the
// Set's add write and answer nothing, get and a Map's remove come back
// through the out trio, keys answers a re-rooted carrier, and the one
// word answers (size, the Set's has) read straight into the i64 domain.
func TestCollMemberFaces(t *testing.T) {
	ir := emitCollFace(t, collSrc(`    let m = collections.mapOf([1, 2], [10, 20])
    m.put(3, 30)
    match m.get(1) {
        Some(v) => { io.println("hit") }
        None => { io.println("none") }
    }
    match m.remove(1) {
        Some(v) => { io.println("rm") }
        None => { io.println("gone") }
    }
    let ks = m.keys()
    for k in ks {
        io.println("k")
    }
    io.println("n:${m.size()}")
    let s = collections.setOf([1])
    s.add(2)
    io.println("h:${s.has(2)}")
    io.println("sn:${s.size()}")
`))
	wantIR(t, ir, "call void @__we_coll_map_put(ptr", "put writes through the entry")
	wantIR(t, ir, "call void @__we_coll_map_get(ptr", "get rides the out trio")
	wantIR(t, ir, "call void @__we_coll_map_remove(ptr", "remove rides the out trio")
	wantIR(t, ir, "call ptr @__we_coll_map_keys(ptr", "keys answers a carrier")
	wantIR(t, ir, "call i64 @__we_coll_map_size(ptr", "size answers i64")
	wantIR(t, ir, "call void @__we_coll_set_add(ptr", "the Set's add writes through the entry")
	wantIR(t, ir, "call i64 @__we_coll_set_has(ptr", "the Set's has answers i64")
	wantIR(t, ir, "call i64 @__we_coll_set_size(ptr", "the Set's size answers i64")
	if n := countLines(ir, "= alloca [3 x i64]"); n < 2 {
		t.Fatalf("the Option pair read %d out trios, want at least 2 (get, remove):\n%s", n, ir)
	}
	// keys' answer walks as any List does: the for over the binding
	// spends the carrier's own len and get.
	wantIR(t, ir, "call i64 @__we_list_len(", "the keys walk rides the carrier len")
	// The hole renders the Bool and i64 members through their converters.
	wantIR(t, ir, "__we_str_of_i64", "size and the Set's answers render in holes")
}

// TestCollKeysIsReRooted: the List keys() hands back is fresh C-side
// carving — the caller re-roots it the way every gc return is, before
// the walk's first element can allocate.
func TestCollKeysIsReRooted(t *testing.T) {
	ir := emitCollFace(t, collSrc(`    let m = collections.mapOf([1, 2], [10, 20])
    let ks = m.keys()
`))
	lines := strings.Split(ir, "\n")
	at := -1
	for i, l := range lines {
		if strings.Contains(l, "call ptr @__we_coll_map_keys(") {
			at = i
			break
		}
	}
	if at < 0 {
		t.Fatalf("IR missing the keys call:\n%s", ir)
	}
	if next := strings.TrimSpace(lines[at+1]); next != "call void @__we_root_push(ptr %"+regOf(lines[at])+")" {
		t.Fatalf("keys' answer is not re-rooted on the next line (%q):\n%s", next, ir)
	}
}

// TestCollStringKeyBoxes: a String key crosses as the handle of the
// 32-byte box the element pipeline carves — the same box a String
// element of a List rides — so put's key word is a fresh alloc whose
// pointer and length words land at +16 and +24.
func TestCollStringKeyBoxes(t *testing.T) {
	ir := emitCollFace(t, collSrc(`    let m = collections.mapOf(["a"], [1])
    m.put("b", 2)
`))
	if n := countLines(ir, "call ptr @__we_alloc(i64 32)"); n != 2 {
		t.Fatalf("the box pipeline ran %d times (the literal's key, the put's key), want 2:\n%s", n, ir)
	}
	wantIR(t, ir, "call void @__we_coll_map_put(ptr", "the put rides the entry")
}

// TestCollParamAndReturnCarryTheFaces: a Map crosses a call boundary as
// its one rooted pointer, and the faces travel the signature both ways
// (design D5-1's param slots): the callee's members on the parameter
// classify through the pair the signature carried, and the caller's
// members on a returned handle classify through the pair the return
// carried.
func TestCollParamAndReturnCarryTheFaces(t *testing.T) {
	src := `import std.io
import std.collections

pub type AppError = Failed(String)

fn make() -> Map<Int64, Int64> {
    return collections.mapOf([1, 2], [10, 20])
}

fn fifth(m: Map<Int64, Int64>) -> Int64 {
    m.put(5, 50)
    return m.size()
}

pub fn main() effect io -> Result<(), AppError> {
    let m = make()
    io.println("n:${fifth(m)}")
    return Ok(())
}
`
	ir := emitCollFace(t, src)
	wantIR(t, ir, "define ptr @main.make()", "a Map return crosses as one pointer")
	wantIR(t, ir, "define i64 @main.fifth(ptr %m)", "a Map parameter crosses as one pointer")
	wantIR(t, ir, "= load ptr, ptr @slot.main.make", "the call loads the slot's fn")
	wantIR(t, ir, "= call ptr %", "the call answers the handle")
	wantIR(t, ir, "= load ptr, ptr @slot.main.fifth", "the fifth call loads its slot")
	wantIR(t, ir, "= call i64 %", "the argument is the handle, not a copy")
	wantIR(t, ir, "call void @__we_coll_map_put(ptr %m", "the callee's put classifies through the signature's pair")
	wantIR(t, ir, "call i64 @__we_coll_map_size(ptr %m", "the callee's size classifies the same way")
}

// TestCollChainedReceiverStops: a constructor's result read directly as
// a receiver classifies nowhere — the faces ride bindings, and no
// binding exists mid-chain — so the chained form keeps the honest stop
// and the bind-first form is the canonical one (the T8-3 precedent,
// listFaceOf's own boundary).
func TestCollChainedReceiverStops(t *testing.T) {
	ni := collFaceStop(t, collSrc(`    collections.mapOf([1, 2], [10, 20]).put(3, 30)
`))
	if ni == nil {
		t.Fatal("a chained receiver crossed without a binding to carry its faces")
	}
}

// TestCollTupleForStaysStopped: the pair iteration stays at the body
// boundary — the sanctioned path over a Map is keys() plus get(), and
// this pin holds that no widening of the walk quietly opened the tuple
// form.
func TestCollTupleForStaysStopped(t *testing.T) {
	ni := collFaceStop(t, collSrc(`    let m = collections.mapOf([1, 2], [10, 20])
    for (k, v) in m {
        io.println("x")
    }
`))
	if ni == nil {
		t.Fatal("the tuple for over a Map stopped stopping")
	}
}

// TestCollDomainRefusals: the Eq domain is the gate the keyed families
// add — a Float64 key (its word is not a bijection on its values) stops
// at the constructor, and the one-word value domain the List element
// face holds a Map to stops at both the constructor's operand and the
// signature that names it.
func TestCollDomainRefusals(t *testing.T) {
	if ni := collFaceStop(t, collSrc(`    let m = collections.mapOf([1.5, 2.5], [1, 2])
`)); ni == nil {
		t.Fatal("a Float64 key crossed the Eq domain gate")
	}
	if ni := collFaceStop(t, collSrc(`    let s = collections.setOf([1.5])
`)); ni == nil {
		t.Fatal("a Float64 element crossed the Set's slot domain gate")
	}
	src := `import std.io
import std.collections

pub type AppError = Failed(String)

fn f(m: Map<Int64, Option<Int64> >) -> Int64 {
    return 1
}

pub fn main() effect io -> Result<(), AppError> {
    return Ok(())
}
`
	if ni := collFaceStop(t, src); ni == nil {
		t.Fatal("an Option value domain crossed into a signature")
	}
}
