package codegen

import (
	"testing"

	"github.com/ltlvtao/welang/internal/typecheck"
)

// B2a T4's face: std.string is the one std module whose bodies are real
// (design D5), so the pipeline defines them the way it defines a program
// module's fns and the calls resolve through the same slot machine — the
// D5-3 open question, answered by the probe this change records: the
// pipeline needed one release (the module rides the program face) and
// one import row (the qualifier enters curImports, which the modKeys
// guard leaves inert for every keyed std module); emitCall itself
// changed nothing.

// strSrc is one program over the string import — the skeleton every pin
// varies.
func strSrc(body string) string {
	return `import std.io
import std.string

pub type AppError = Failed(String)

pub fn main() effect io -> Result<(), AppError> {
` + body + `    return Ok(())
}
`
}

// emitStr checks and emits one program with std.string riding the program
// face (the release build.go gives it), refusing boundaries: these
// programs build, so a boundary here is a failure of the fixture or of
// the face, never a fact to pin.
func emitStr(t *testing.T, src string) string {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	sf, ok := typecheck.StdModule("std.string")
	if !ok || sf == nil {
		t.Fatal("std.string not registered")
	}
	ir, ni := EmitProgram(ModeBuild, []ProgModule{
		{Key: "std.string", ID: "std.string", File: sf, Shapes: sh},
		{Key: "main", ID: "demo", File: f, Shapes: sh},
	})
	if ni != nil {
		t.Fatalf("boundary %q over the string face:\n%s", ni.What, src)
	}
	return ir
}

// TestStringJoinEmitsDefineAndSlot: join's define carries the carrier's
// handle and the separator's pair and answers the built String's pair —
// the ABI a List<String> parameter and a String return take — and the
// call site rides the same slot shape every fn call does, the slot
// pointing at the define itself (D5-3's first sub-question: the slot of
// a real define is the keyed slot's own shape). The body walks the
// carrier through the snapshot protocol and reads each element's pair
// out of its box.
func TestStringJoinEmitsDefineAndSlot(t *testing.T) {
	ir := emitStr(t, strSrc(`    let many: List<String> = ["a", "b"]
    io.println(string.join(many, "-"))
`))
	wantIR(t, ir, "define { ptr, i64 } @std.string.join(ptr %parts, ptr %sep0, i64 %sep1)", "join's define")
	wantIR(t, ir, "@slot.std.string.join = global ptr @std.string.join", "the slot points at the define")
	wantIR(t, ir, "load ptr, ptr @slot.std.string.join", "the call site rides the slot")
	wantIR(t, ir, "call ptr @__we_list_snap", "the body walks the carrier")
	wantIR(t, ir, "call i64 @__we_list_get", "each element is the walk's own read")
	wantIR(t, ir, "call %struct.we_str @__we_str_concat", "the parts join by concatenation")
}

// TestStringRepeatEmitsTheWhileBody: repeat's define carries the String
// pair and the count as words, and its body is the while loop the source
// writes — the counter slot, the comparison, the concatenation.
func TestStringRepeatEmitsTheWhileBody(t *testing.T) {
	ir := emitStr(t, strSrc(`    io.println(string.repeat("ab", 3))
`))
	wantIR(t, ir, "define { ptr, i64 } @std.string.repeat(ptr %s0, i64 %s1, i64 %n)", "repeat's define")
	wantIR(t, ir, "@slot.std.string.repeat = global ptr @std.string.repeat", "repeat's slot")
	wantIR(t, ir, "icmp slt i64 ", "the while's count test")
}

// TestStringJoinCallsFromAPureContext: join and repeat declare no effect
// section (chapter 16's explicit purity, design D5), so a pure helper —
// no effect segment of its own — calls them the same way main does; the
// define's body and the call site are effect-blind, which is the whole
// of the purity claim the module makes.
func TestStringJoinCallsFromAPureContext(t *testing.T) {
	ir := emitStr(t, `import std.io
import std.string

fn banner(parts: List<String>) -> String {
    return "[" + string.join(parts, ",") + "]"
}

pub type AppError = Failed(String)

pub fn main() effect io -> Result<(), AppError> {
    let many: List<String> = ["a", "b"]
    io.println(banner(many))
    return Ok(())
}
`)
	wantIR(t, ir, "load ptr, ptr @slot.std.string.join", "a pure context calls join the same way")
	wantIR(t, ir, "define { ptr, i64 } @main.banner(", "the pure helper itself defines")
}
