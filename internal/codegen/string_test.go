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

// The keyed conversion family's emission face (B3a T3, design D3/D4):
// the seven names dispatch through the keyed set ahead of the program
// face — the alias's mixed dispatch — and every call rides a slot the
// check face's mock interception can later swap (the fs family's shape;
// the collections constructors' no-slot contrast does not apply, these
// are monomorphic module fns and mockable by chapter 20's rule). The
// three parses pass the String pair and answer the Option trio through
// the out words; the four bridges are the register forms.

// TestStringParseIntRidesTheKeyedSlot: parseInt's call site loads the
// keyed slot (the declare the slot's default references), passes the
// String pair and the out trio, and the match reads the trio's three
// words — the None/Some tag arithmetic and the payload word the coll
// get family fixed.
func TestStringParseIntRidesTheKeyedSlot(t *testing.T) {
	ir := emitStr(t, strSrc(`    let n = string.parseInt("12")
    match n {
        Some(v) => { io.println("got ${v}") }
        None => { io.println("none") }
    }
`))
	wantIR(t, ir, "declare void @__we_string_parse_int(ptr, i64, ptr)", "parse_int's declare")
	wantIR(t, ir, "@slot.string.parseInt = global ptr @__we_string_parse_int", "the slot defaults at the C entry")
	wantIR(t, ir, "load ptr, ptr @slot.string.parseInt", "the call site rides the slot")
	wantIR(t, ir, "call void %", "the parse answers through the out trio, not a register")
}

// TestStringParseFloatCarriesTheBitWord: parseFloat's Some payload word
// is the double's bits — the carrier's own word for a Float64 — so the
// arm's binding reads it back through the bitcast the payload-face
// pipeline takes (payRoundTrip's skF64 arm), and the None arm is the
// same zero-word answer the tag arithmetic writes.
func TestStringParseFloatCarriesTheBitWord(t *testing.T) {
	ir := emitStr(t, strSrc(`    let f = string.parseFloat("1.5")
    match f {
        Some(w) => { io.println("w ${w}") }
        None => { io.println("none") }
    }
`))
	wantIR(t, ir, "declare void @__we_string_parse_float(ptr, i64, ptr)", "parse_float's declare")
	wantIR(t, ir, "@slot.string.parseFloat = global ptr @__we_string_parse_float", "the keyed slot")
	wantIR(t, ir, "bitcast i64 %", "the Some arm reads the payload word back as a double")
	wantIR(t, ir, "@__we_str_of_f64", "the binding renders in the Float64 domain")
}

// TestStringParseUIntsPayloadIsTheU64Word: parseUInt's payload rides the
// i64 word as the u64 pattern — the same one-word read Int64 takes, the
// typeName carrying the unsigned domain to the renderer.
func TestStringParseUIntsPayloadIsTheU64Word(t *testing.T) {
	ir := emitStr(t, strSrc(`    let u = string.parseUInt("7")
    match u {
        Some(v) => { io.println("u ${v}") }
        None => { io.println("none") }
    }
`))
	wantIR(t, ir, "declare void @__we_string_parse_uint(ptr, i64, ptr)", "parse_uint's declare")
	wantIR(t, ir, "@slot.string.parseUInt = global ptr @__we_string_parse_uint", "the keyed slot")
	wantIR(t, ir, "load ptr, ptr @slot.string.parseUInt", "the call rides the slot")
}

// TestStringRuneBridgesAnswerRegisters: the rune identity pair is i64 in
// and i64 out over the slot — runeCode taking the Rune operand as its
// code point (the i64 domain Rune has no storage form of its own) and
// runeFrom's answer binding in the Rune domain, which the interpolation
// renders through of_rune.
func TestStringRuneBridgesAnswerRegisters(t *testing.T) {
	ir := emitStr(t, strSrc(`    let c = string.runeCode('A')
    io.println("c ${c}")
    let r = string.runeFrom(65)
    io.println("r ${r}")
`))
	wantIR(t, ir, "declare i64 @__we_string_rune_code(i64)", "rune_code's declare")
	wantIR(t, ir, "@slot.string.runeCode = global ptr @__we_string_rune_code", "runeCode's slot")
	wantIR(t, ir, "declare i64 @__we_string_rune_from(i64)", "rune_from's declare")
	wantIR(t, ir, "@slot.string.runeFrom = global ptr @__we_string_rune_from", "runeFrom's slot")
	wantIR(t, ir, "call i64 %", "the bridges answer in the i64 register form")
	wantIR(t, ir, "@__we_str_of_rune", "runeFrom's answer renders as a Rune")
}

// TestStringFloatBridgesCrossDomains: the bit reinterpretation pair
// crosses the two register readings — floatBits takes the double word
// and answers i64, floatFromBits takes i64 and answers the double word
// the Float64 domain's renderer picks up.
func TestStringFloatBridgesCrossDomains(t *testing.T) {
	ir := emitStr(t, strSrc(`    let b = string.floatBits(2.5)
    io.println("b ${b}")
    let g = string.floatFromBits(b)
    io.println("g ${g}")
`))
	wantIR(t, ir, "declare i64 @__we_string_float_bits(double)", "float_bits's declare")
	wantIR(t, ir, "@slot.string.floatBits = global ptr @__we_string_float_bits", "floatBits's slot")
	wantIR(t, ir, "(double 0x4004000000000000)", "floatBits takes the double word — the 2.5 constant folded to its exact bit pattern")
	wantIR(t, ir, "declare double @__we_string_float_from_bits(i64)", "float_from_bits's declare")
	wantIR(t, ir, "call double %", "floatFromBits answers the double word")
	wantIR(t, ir, "@__we_str_of_f64", "the Float64 answer renders in its own domain")
}

// TestStringMixedDispatchKeepsBothRoutes: one alias, two routes — the
// keyed set answers parseInt through its slot, join falls to the
// program face's define-and-slot unchanged. This is the whole of design
// D4's mixed dispatch: a name's membership in the keyed set is the only
// thing that decides its route.
func TestStringMixedDispatchKeepsBothRoutes(t *testing.T) {
	ir := emitStr(t, strSrc(`    let many: List<String> = ["a", "b"]
    let s = string.join(many, "-")
    let n = string.parseInt("12")
    match n {
        Some(v) => { io.println("${s} ${v}") }
        None => { io.println("${s} none") }
    }
`))
	wantIR(t, ir, "define { ptr, i64 } @std.string.join(ptr %parts, ptr %sep0, i64 %sep1)", "join keeps its program-face define")
	wantIR(t, ir, "@slot.std.string.join = global ptr @std.string.join", "join keeps its own slot")
	wantIR(t, ir, "@slot.string.parseInt = global ptr @__we_string_parse_int", "parseInt rides the keyed slot")
	wantIR(t, ir, "load ptr, ptr @slot.std.string.join", "join's call site is unchanged")
	wantIR(t, ir, "load ptr, ptr @slot.string.parseInt", "parseInt's call site rides the keyed slot")
}
