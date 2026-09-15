package codegen

import (
	"testing"
)

// B2a T3's face: process.run answers its fused Result through the
// out-parameter trio with the Ok half a record handle the C side carved —
// the pins run through the real check pipeline for the same reason fs's
// do (the face is about what the checker already accepts, the module's
// declarations registered at the import site).

// procSrc is one program whose main body is body over the process import
// — the skeleton every pin varies.
func procSrc(body string) string {
	return `import std.io
import std.process

pub fn main() effect io -> Result<(), process.ProcessError> {
` + body + `    return Ok(())
}
`
}

// emitProc checks and emits one program, refusing boundaries: these
// programs build, so a boundary here is a failure of the fixture or of
// the face, never a fact to pin.
func emitProc(t *testing.T, src string) string {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q over the process face:\n%s", ni.What, src)
	}
	return ir
}

// procStop checks and emits one program, answering its boundary: the
// negative pins' helper.
func procStop(t *testing.T, src string) *NotImplemented {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	return ni
}

// TestProcRunEmitsTheFourOperandCall: the entry rides its slot, the
// answer block is one [3 x i64] alloca, the three gep loads at 0/8/16
// land in three fresh i64 slots, and the call takes four operands — the
// command's pair, the argument carrier's handle, the out block.
func TestProcRunEmitsTheFourOperandCall(t *testing.T) {
	ir := emitProc(t, procSrc(`    match process.run("/bin/echo", ["hi"]) {
        Ok(pr) => { io.println("ok") }
        Err(_) => { io.println("bad") }
    }
`))
	wantIR(t, ir, "@slot.process.run = global ptr @__we_proc_run", "the entry's slot")
	wantIR(t, ir, "declare void @__we_proc_run(ptr, i64, ptr, ptr)", "the declare row")
	wantIR(t, ir, "= alloca [3 x i64]", "the out block")
	out := definedReg(t, ir, "= alloca [3 x i64]")
	for _, off := range []string{"0", "8", "16"} {
		wantIR(t, ir, "getelementptr i8, ptr "+out+", i64 "+off, "the out trio's word at +"+off)
	}
	if n := callOperandCount(t, slotCallLine(t, ir, "process.run")); n != 4 {
		t.Fatalf("run took %d operands, want 4 (command pair, carrier, out):\n%s", n, ir)
	}
}

// TestProcRunOkHandleReRoots: the Ok half's record handle is a gc
// reference the C side carved unrooted (no allocation follows the
// carve), so the call site re-roots the way every gc return does — the
// first inttoptr in the body is that re-root (the call site emits before
// the arm dispatch; the arm binding's own read comes later).
func TestProcRunOkHandleReRoots(t *testing.T) {
	ir := emitProc(t, procSrc(`    match process.run("/bin/echo", ["hi"]) {
        Ok(pr) => { io.print(pr.stdout) }
        Err(_) => { io.println("bad") }
    }
`))
	h := definedReg(t, ir, " = inttoptr i64")
	wantIR(t, ir, "call void @__we_root_push(ptr "+h+")", "the Ok handle re-roots at the call")
}

// TestProcResultFieldsReadAtTheRecordLayout: the arm-bound name is the
// record handle, and its fields read at the layout a We-side
// construction takes — exitCode's word at 16, stdout's pair at 24/32 —
// pinning the C-side carve (the same offsets process.c writes) against
// the emitter's.
func TestProcResultFieldsReadAtTheRecordLayout(t *testing.T) {
	ir := emitProc(t, procSrc(`    match process.run("/bin/echo", ["hi"]) {
        Ok(pr) => {
            io.print(pr.stdout)
            io.println("code ${pr.exitCode}")
        }
        Err(_) => { io.println("bad") }
    }
`))
	rec := lastDefinedReg(t, ir, " = inttoptr i64")
	wantIR(t, ir, "getelementptr i8, ptr "+rec+", i64 24", "stdout's pointer word at +24")
	wantIR(t, ir, "getelementptr i8, ptr "+rec+", i64 32", "stdout's length word at +32")
	wantIR(t, ir, "getelementptr i8, ptr "+rec+", i64 16", "exitCode's word at +16")
}

// TestQualifiedProcessErrorFusesTheErrRange: `Result<_, process.ProcessError>`
// in a signature classifies — the qualified sum resolves through the
// import site's registration, so the helper's define carries the sum's
// own three-word return — and the fused table shows in the Err arm's
// range test (Ok at zero, ProcessFailed above it). The entry itself
// unwraps its Result through the exit protocol (an i32 face), so the
// signature pin reads a helper.
func TestQualifiedProcessErrorFusesTheErrRange(t *testing.T) {
	ir := emitProc(t, `import std.io
import std.process

fn probe() effect io -> Result<(), process.ProcessError> {
    match process.run("/bin/false", []) {
        Ok(_) => { io.println("ok") }
        Err(_) => { io.println("bad") }
    }
    return Ok(())
}

pub fn main() effect io -> Result<(), process.ProcessError> {
    match probe() {
        Ok(_) => { io.println("ok") }
        Err(_) => { io.println("bad") }
    }
    return Ok(())
}
`)
	wantIR(t, ir, "define { i64, i64, i64 } @main.probe()", "the qualified E reaches the helper's ABI")
	wantIR(t, ir, "icmp sge i64 ", "the Err arm admits the error variants at once")
}

// TestEmptyArgsLiteralRidesTheTypedFace: the argument literal crosses the
// typed literal path — the signature names the element face, so the empty
// list needs no element to read it from. The carrier the face answers is
// traced (the element words are String boxes), zero elements carved.
func TestEmptyArgsLiteralRidesTheTypedFace(t *testing.T) {
	ir := emitProc(t, procSrc(`    match process.run("/bin/false", []) {
        Ok(pr) => { io.println("code ${pr.exitCode}") }
        Err(_) => { io.println("bad") }
    }
`))
	wantIR(t, ir, "call ptr @__we_list_new(i64 0, i64 1)", "the empty carrier rides the signature's String face")
}

// TestQuestionOnProcRunStillStops: the `?` Ok face binds no gc handle —
// a wider face would hand the caller the record's raw word read as a
// number, so run under `?` keeps stopping at the boundary (an honest
// stop this change does not widen; a golden that needs it belongs with
// the task that widens it).
func TestQuestionOnProcRunStillStops(t *testing.T) {
	ni := procStop(t, procSrc(`    let pr = process.run("/bin/false", [])?
    io.println("unreachable")
`))
	if ni == nil {
		t.Fatal("a `?` over process.run's Ok handle bound a value")
	}
	if ni.What != bndMainBody {
		t.Fatalf("boundary word: %q", ni.What)
	}
}
