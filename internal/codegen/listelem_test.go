package codegen

import (
	"strings"
	"testing"
)

// T11-2's face: a String element is two words and the carrier's slot is
// one, so the element rides as a handle to a box holding the pair — a
// descriptor-less 32-byte gc object {map@0 null, size@8, strptr@16,
// strlen@24}. The map word stays null because a String's buffer is
// malloc'd, outside the collector's domain: there is no word in the box
// the collector could trace through, the all-scalar box's precedent. The
// carrier traces the handle exactly as it traces a record's — the traced
// word __we_list_new takes is one predicate (listElem.traces) that the
// literal opening and the collecting walk both ask — and the box roots
// until the body's exit by the allocObj protocol's own accounting.
//
// The read back is the walk's own: one __we_list_get answers the handle,
// the binding converts it to the box and loads the pair out of it, and
// the name binds as any String does — slot-backed where the body assigns
// it, the operand pair otherwise. What still stops is every face that
// hands the element to a callback: the callback's parameter would be the
// two-word String itself and the box handle is not the String, so the
// combinators that read elements stop at their own boundary while count
// and collect — the two that do not — open with the carrier.

// linesFrom answers the n consecutive IR lines starting at the first one
// containing frag — order pins read a construction's own instruction
// sequence rather than its lines' scattered presence.
func linesFrom(t *testing.T, ir, frag string, n int) []string {
	t.Helper()
	lines := strings.Split(ir, "\n")
	for i, line := range lines {
		if strings.Contains(line, frag) {
			if i+n > len(lines) {
				t.Fatalf("%q sits too near the end:\n%s", frag, ir)
			}
			return lines[i : i+n]
		}
	}
	t.Fatalf("no line with %q:\n%s", frag, ir)
	return nil
}

// TestStringElementBoxesIntoTheCarrier: the literal carves its carrier
// traced — a String's word is a gc handle now — and every element takes
// the allocObj protocol: alloc 32, a null map word, the root push, then
// the pair at 16 and 24, and the handle (not the String) is what push
// receives.
func TestStringElementBoxesIntoTheCarrier(t *testing.T) {
	ir := emitListAbi(t, listAbiSrc("", `    let xs = ["a", "bb"]
    let n = xs.iterator().count()
    io.println("${n}")
`))
	wantIR(t, ir, "call ptr @__we_list_new(i64 2, i64 1)", "the carrier traces its String element words")
	if got := strings.Count(ir, "call ptr @__we_alloc(i64 32)"); got != 2 {
		t.Fatalf("%d element boxes for 2 elements:\n%s", got, ir)
	}
	box := definedReg(t, ir, "call ptr @__we_alloc(i64 32)")
	open := linesFrom(t, ir, "call ptr @__we_alloc(i64 32)", 3)
	wantIR(t, open[1], "store ptr null, ptr "+box, "no descriptor: no word in the box is a gc reference")
	wantIR(t, open[2], "call void @__we_root_push(ptr "+box+")", "the box roots by the allocObj protocol")
	wantIR(t, ir, "store ptr @.s", "the pair's pointer word rides at 16")
	wantIR(t, ir, "store i64 2, ptr", "the pair's length word rides at 24")
	handle := definedReg(t, ir, "ptrtoint ptr "+box)
	carrier := definedReg(t, ir, "call ptr @__we_list_new")
	wantIR(t, ir, "call ptr @__we_list_push(ptr "+carrier+", i64 "+handle+")", "push receives the handle, not the String")
}

// TestStringElementWalkReadsTheBoxBack: the walk's one read answers the
// handle, the binding turns it back into the box, and the pair loads out
// of 16 and 24 — the two loads an unassigned String name keeps as its
// operand pair.
func TestStringElementWalkReadsTheBoxBack(t *testing.T) {
	ir := emitListAbi(t, listAbiSrc("", `    let xs = ["a", "bb"]
    for s in xs { io.println(s) }
`))
	word := resultReg(firstLineWith(t, ir, "call i64 @__we_list_get"))
	box := definedReg(t, ir, "inttoptr i64 "+word)
	back := linesFrom(t, ir, "inttoptr i64 "+word, 5)
	wantIR(t, back[1], "getelementptr i8, ptr "+box+", i64 16", "the pointer word's address")
	wantIR(t, back[2], "load ptr, ptr", "the pointer word's load")
	wantIR(t, back[3], "getelementptr i8, ptr "+box+", i64 24", "the length word's address")
	wantIR(t, back[4], "load i64, ptr", "the length word's load")
}

// operandBetween answers the substring of line between pre and post — the
// operand a conversion names, read back out of its own instruction.
func operandBetween(t *testing.T, line, pre, post string) string {
	t.Helper()
	i := strings.Index(line, pre)
	if i < 0 {
		t.Fatalf("no %q in %q", pre, line)
	}
	rest := line[i+len(pre):]
	j := strings.Index(rest, post)
	if j < 0 {
		t.Fatalf("no %q after %q in %q", post, pre, line)
	}
	return rest[:j]
}

// firstLineWith answers the one line containing frag, refusing ambiguity:
// these pins read a walk with exactly one element read.
func firstLineWith(t *testing.T, ir, frag string) string {
	t.Helper()
	var found []string
	for line := range strings.SplitSeq(ir, "\n") {
		if strings.Contains(line, frag) {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d lines with %q:\n%s", len(found), frag, ir)
	}
	return found[0]
}

// TestStringElementBindsAssignedNamesThroughSlots: a name the body assigns
// takes the two slots an assignment stores through — the loads out of the
// box land in storage, and the body's read loads the current contents
// back, exactly as an assigned String binding does anywhere else.
func TestStringElementBindsAssignedNamesThroughSlots(t *testing.T) {
	ir := emitListAbi(t, listAbiSrc("", `    let xs = ["a", "bb"]
    for s in xs {
        s = "z"
        io.println(s)
    }
`))
	word := resultReg(firstLineWith(t, ir, "call i64 @__we_list_get"))
	stored := linesFrom(t, ir, "inttoptr i64 "+word, 7)
	wantIR(t, stored[5], "store ptr %", "the pointer word stores through its slot")
	wantIR(t, stored[6], "store i64 %", "the length word stores through its slot")
}

// TestStringElementStopsWhereACallbackWouldTakeIt: the combinators that
// hand the element to a callback stop at their own boundary — the
// callback's parameter would be the two-word String itself, and the box
// handle is not the String. count and collect, the two that never touch
// the element, are open faces beside it.
func TestStringElementStopsWhereACallbackWouldTakeIt(t *testing.T) {
	ni := listAbiStop(t, listAbiSrc("", `    let xs = ["a", "bb"]
    let hit = xs.iterator().any(|s| true)
    io.println("${hit}")
`))
	if ni == nil {
		t.Fatal("a callback took a String element as one word")
	}
	if ni.What != bndMainBody {
		t.Fatalf("boundary word: %q", ni.What)
	}
}

// TestStringElementCollectKeepsTheBoxRooted: a collecting walk discovers
// its count as it goes, so its pushes grow the carrier — and the growth
// allocation can sweep the very box this pass is storing. The pass's
// element is rooted for exactly that call and popped after it, the same
// pair a gc record element takes, and the result carrier is carved traced
// because its words are handles too.
func TestStringElementCollectKeepsTheBoxRooted(t *testing.T) {
	ir := emitListAbi(t, listAbiSrc("", `    let xs = ["a", "bb"]
    let ys = xs.iterator().collect()
    let n = ys.iterator().count()
    io.println("${n}")
`))
	wantIR(t, ir, "call ptr @__we_list_new(i64 0, i64 1)", "the collected carrier traces its words")
	root := linesFrom(t, ir, "inttoptr i64", 8)
	wantIR(t, root[1], "call void @__we_root_push(ptr %", "the pass's box is rooted across the push")
	word := operandBetween(t, root[0], "inttoptr i64 ", " to ptr")
	wantIR(t, root[2], "load ptr, ptr", "the pass reads the carrier from its slot")
	wantIR(t, root[3], "call ptr @__we_list_push(", "the pass stores its element")
	wantIR(t, root[3], "i64 "+word+")", "the raw word — not the box — is what the push stores")
	wantIR(t, root[4], "call void @__we_root_pop()", "the element root pops after the push")
	wantIR(t, root[5], "call void @__we_root_pop()", "the carrier's old root pops for its new identity")
	wantIR(t, root[6], "call void @__we_root_push(ptr %", "the new identity is the standing root")
	wantIR(t, root[7], "store ptr %", "the slot takes the new identity")
}

// TestNestedStringListElementStillRefuses: a nested collection still has
// no element face — the String widening changed what one word can hold,
// not the rule that a carrier's own face would have to be written down
// too. The refusal is elemFaceOfType's own entry check, unchanged.
func TestNestedStringListElementStillRefuses(t *testing.T) {
	ni := listAbiStop(t, listAbiSrc(
		"fn f(xs: List<List<String> >) -> Int64 {\n    return 0\n}\n",
		"",
	))
	if ni == nil {
		t.Fatal("a nested List laid out a carrier face")
	}
	if ni.What != bndFnBody {
		t.Fatalf("boundary word: %q", ni.What)
	}
}
