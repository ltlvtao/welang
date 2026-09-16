package codegen

import (
	"strings"
	"testing"
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
