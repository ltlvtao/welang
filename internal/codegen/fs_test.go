package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/parser"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// B2a T2's face: the fs family's seven entries answer a fused Result
// through the out-parameter trio, and the module's own FsError reaches a
// caller's signatures qualified — `Result<_, fs.FsError>` — a spelling
// the sum machinery never resolved before (a std module is no program
// module, so its declarations reached no pass-one walk; the import site
// registers them). The pins run through the real check pipeline
// (checkShapes) for the same reason listabi's do: the face is about what
// the checker already accepts.

// fsSrc is one program whose main body is body over the fs import — the
// skeleton every pin varies.
func fsSrc(body string) string {
	return `import std.io
import std.fs

pub fn main() effect io -> Result<(), fs.FsError> {
` + body + `    return Ok(())
}
`
}

// emitFs checks and emits one program, refusing boundaries: these
// programs build, so a boundary here is a failure of the fixture or of
// the face, never a fact to pin.
func emitFs(t *testing.T, src string) string {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q over the fs face:\n%s", ni.What, src)
	}
	return ir
}

// fsStop checks and emits one program, answering its boundary: the
// negative pins' helper.
func fsStop(t *testing.T, src string) *NotImplemented {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	return ni
}

// callOperandCount counts one call line's operands — the out trio's
// whole shape check: a unit entry hands three words (path pair, out), a
// data entry five (two pairs, out).
func callOperandCount(t *testing.T, line string) int {
	t.Helper()
	i := strings.Index(line, "(")
	if i < 0 {
		t.Fatalf("takes no operands:\n%s", line)
	}
	inner := strings.TrimSuffix(line[i+1:], ")")
	return len(strings.Split(inner, ", "))
}

// TestFsUnitEntryEmitsTheOutTrio: the entry rides its slot like every
// mockable face, the out block is one [3 x i64] alloca, and the three
// gep loads at 0/8/16 land in three fresh i64 slots — the fused
// Result's whole slot shape, built from words the C side wrote.
func TestFsUnitEntryEmitsTheOutTrio(t *testing.T) {
	ir := emitFs(t, fsSrc(`    match fs.removeFile("x") {
        Ok(_) => { io.println("ok") }
        Err(_) => { io.println("bad") }
    }
`))
	wantIR(t, ir, "@slot.fs.removeFile = global ptr @__we_fs_remove_file", "the entry's slot")
	wantIR(t, ir, "declare void @__we_fs_remove_file(ptr, i64, ptr)", "the declare row")
	wantIR(t, ir, "= alloca [3 x i64]", "the out block")
	out := definedReg(t, ir, "= alloca [3 x i64]")
	for _, off := range []string{"0", "8", "16"} {
		wantIR(t, ir, "getelementptr i8, ptr "+out+", i64 "+off, "the out trio's word at +"+off)
	}
	if n := callOperandCount(t, slotCallLine(t, ir, "fs.removeFile")); n != 3 {
		t.Fatalf("removeFile took %d operands, want 3 (path pair, out):\n%s", n, ir)
	}
}

// TestFsDataEntryTakesBothPairs: writeFile's data String crosses as its
// own (ptr, i64) pair beside the path's — five operands at the call.
func TestFsDataEntryTakesBothPairs(t *testing.T) {
	ir := emitFs(t, fsSrc(`    match fs.writeFile("x", "d") {
        Ok(_) => { io.println("ok") }
        Err(_) => { io.println("bad") }
    }
`))
	if n := callOperandCount(t, slotCallLine(t, ir, "fs.writeFile")); n != 5 {
		t.Fatalf("writeFile took %d operands, want 5 (path pair, data pair, out):\n%s", n, ir)
	}
}

// TestQualifiedFsErrorFusesTheErrRange: `Result<(), fs.FsError>` in a
// signature classifies — the qualified sum resolves through the import
// site's registration, so the helper's define carries the sum's own
// three-word return — and the fused table shows in the Err arm's range
// test (Ok at zero, the error variants above it), which only a table
// whose head is Ok renders. The entry itself unwraps its Result through
// the exit protocol (an i32 face), so the signature pin reads a helper.
func TestQualifiedFsErrorFusesTheErrRange(t *testing.T) {
	ir := emitFs(t, `import std.io
import std.fs

fn probe() effect io -> Result<(), fs.FsError> {
    match fs.makeDir("d") {
        Ok(_) => { io.println("ok") }
        Err(_) => { io.println("bad") }
    }
    return Ok(())
}

pub fn main() effect io -> Result<(), fs.FsError> {
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

// TestListDirOkBindsTheWalkFace: listDir's Ok handle is re-rooted at the
// call the way every gc return is (the C side rooted it only across its
// own construction loop), and the arm-bound name carries the element
// face payloadShape ferried — the body's for walks it without any
// annotation of its own.
func TestListDirOkBindsTheWalkFace(t *testing.T) {
	ir := emitFs(t, fsSrc(`    match fs.listDir("d") {
        Ok(xs) => {
            for n in xs { io.println("entry:${n}") }
        }
        Err(_) => { io.println("bad") }
    }
`))
	h := definedReg(t, ir, " = inttoptr i64")
	wantIR(t, ir, "call void @__we_root_push(ptr "+h+")", "the handle re-roots at the call")
	wantIR(t, ir, "call ptr @__we_list_snap", "the arm-bound name walks by its carried face")
	wantIR(t, ir, "call i64 @__we_list_get", "each entry is the walk's own read")
}

// TestQuestionOnFsReadBindsTheStringPair: `?` over the whole-read's
// Result answers a String expression — the Ok path reads the slot's two
// payload words as the pair (the inttoptr is the pointer word's read),
// and the println hands that pair across.
func TestQuestionOnFsReadBindsTheStringPair(t *testing.T) {
	ir := emitFs(t, fsSrc(`    let t = fs.readFile("f")?
    io.println(t)
`))
	// The body owns two inttoptr reads: the Err fold's payload word and
	// the Ok bind's — the bind is the later one, so the pin reads the
	// last, and the println must hand exactly that pointer across.
	h := lastDefinedReg(t, ir, " = inttoptr i64")
	if !strings.Contains(slotCallLine(t, ir, "io.println"), "ptr "+h) {
		t.Fatalf("println did not receive the pair's pointer word %s:\n%s", h, ir)
	}
}

// TestQuestionOnListOkStillStops: the `?` Ok face binds no gc handle —
// a wider face would hand the caller a raw word read as a number, so
// listDir under `?` keeps stopping at the boundary (an honest stop this
// change does not widen; a golden that needs it belongs with the task
// that widens it).
func TestQuestionOnListOkStillStops(t *testing.T) {
	ni := fsStop(t, fsSrc(`    let xs = fs.listDir("d")?
    io.println("unreachable")
`))
	if ni == nil {
		t.Fatal("a `?` over listDir's Ok handle bound a value")
	}
	if ni.What != bndMainBody {
		t.Fatalf("boundary word: %q", ni.What)
	}
}

// TestFsMockRidesTheOutTrio: the fs family's mockable face is the C entry
// behind its own slot (not a stdFnEntries sym), so the mock installs over
// that slot and restores back to the entry. And because the entry answers
// a fused Result through a trailing out block — a calling shape no std
// entry and no program fn shares — the mock rides two defines: the body
// under the sum ABI the declaration spells, and the void wrapper the slot
// installs, forwarding the sum's three registers to the caller's out
// block at the offsets the C side's own readers gep (8*N — extractvalue
// index N mirrors gepLoadI64 at 8*N, so tag, pay, pay1 keep their order).
func TestFsMockRidesTheOutTrio(t *testing.T) {
	// The real test-module pipeline: std.fs rides the check graph as a
	// dep (its FsError registration is what `Result<_, fs.FsError>` in
	// the mock's own signature resolves through) and is filtered from
	// the program face, whose call faces are the emitter's own table.
	f, d, pni := parser.Parse("m_test.we", []byte(`import std.fs

test "mocked read" {
    mock fs.readFile(path: String) -> Result<String, fs.FsError> effect io {
        return Ok("stub")
    }
    match fs.readFile("/etc/hostname") {
        Ok(_) => { }
        Err(_) => { }
    }
}
`))
	if d != nil || pni != nil {
		t.Fatalf("parse: d=%v ni=%v", d, pni)
	}
	f.IsTestModule = true
	sf, ok := typecheck.StdModule("std.fs")
	if !ok || sf == nil {
		t.Fatal("std.fs not registered")
	}
	td, tni, sh := typecheck.CheckTestRoot(f, "tests/m_test.we", "tests.m_test", []typecheck.Module{
		{Key: "std.fs", Path: "std/fs.we", File: sf},
	})
	if td != nil || tni != nil {
		t.Fatalf("check: %v %+v", td, tni)
	}
	ir, ni := EmitProgram(ModeTest, []ProgModule{
		{Key: "tests.m_test", ID: "demo", File: f, Shapes: sh},
	})
	if ni != nil {
		t.Fatalf("boundary %q over the fs mock face:\n%s", ni.What, ir)
	}
	wantIR(t, ir, "@slot.fs.readFile = global ptr @__we_fs_read_file", "the family's own slot defaults to the C entry")
	wantIR(t, ir, "store ptr @tests.m_test.mock.0, ptr @slot.fs.readFile", "install")
	wantIR(t, ir, "store ptr @__we_fs_read_file, ptr @slot.fs.readFile", "restore returns to the C entry")
	wantIR(t, ir, "define { i64, i64, i64 } @tests.m_test.mock.0.body(ptr %path0, i64 %path1)", "the body define keeps the sum ABI")
	wantIR(t, ir, "define void @tests.m_test.mock.0(ptr %path0, i64 %path1, ptr %__out)", "the wrapper takes the entry's own calling shape")
	wantIR(t, ir, "call { i64, i64, i64 } @tests.m_test.mock.0.body(ptr %path0, i64 %path1)", "the wrapper forwards to the body")
	wantIR(t, ir, "extractvalue { i64, i64, i64 } %m, 2", "the third sum register comes out")
	wantIR(t, ir, "getelementptr i8, ptr %__out, i64 16", "and lands at the out block's +16 word")
}
