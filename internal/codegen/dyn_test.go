package codegen

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// T5: the chapter 10 box (design D5). What is pinned here is the box's
// shape — one word of table pointer behind the frozen header, then the
// payload by carrying face — and the descriptor that keeps the payload
// alive. The dispatch that reads the table pointer is T6's.

var (
	// regAssign matches the definition line of one value register.
	regAssign = regexp.MustCompile(`^\s*(%v\d+) = `)
	// regName is every value register spelling, for normalization.
	regName = regexp.MustCompile(`%v\d+`)
)

// resultReg returns the register one line defines, or "".
func resultReg(line string) string {
	if m := regAssign.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// dynGlobal matches the definition line of one box descriptor.
var dynGlobal = regexp.MustCompile(`^(@\.dynmap\d+) = `)

// dynGlobals returns the descriptors a module defines — the definition
// lines only, since every store that names one mentions it too.
func dynGlobals(ir string) []string {
	var names []string
	for _, ln := range strings.Split(ir, "\n") {
		if m := dynGlobal.FindStringSubmatch(ln); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}

// normalizeRegs blanks the numbering: a sequence pin says what is
// emitted, in order, and a fresh-register counter is not part of that.
// The box's own registers stay distinguishable from the rest — %box is
// the box or an address off it, %r is anything else — because which
// register a word is read from is part of the face: a constant payload, a
// handle read through the ownership path, and a sum slot's words are
// three different words at the same offset.
func normalizeRegs(s string, owned map[string]bool) string {
	return regName.ReplaceAllStringFunc(s, func(m string) string {
		if owned[m] {
			return "%box"
		}
		return "%r"
	})
}

// boxRun returns the instruction run that builds one box of the given
// size, in order and stripped of register numbering: every line from the
// allocation onward that names the box or a register the box's own
// address arithmetic produced. Nothing else in a module is a box, so the
// run is the construction and nothing besides it — which is what lets a
// face be pinned whole, sequence and all, rather than probed line by
// line.
func boxRun(t *testing.T, ir string, size int) []string {
	t.Helper()
	lines := strings.Split(ir, "\n")
	alloc := fmt.Sprintf("@__we_alloc(i64 %d)", size)
	for i, ln := range lines {
		if !strings.Contains(ln, alloc) {
			continue
		}
		box := resultReg(ln)
		if box == "" {
			continue
		}
		owned := map[string]bool{box: true}
		var run []string
		for _, l := range lines[i:] {
			mention := false
			for reg := range owned {
				if regexp.MustCompile(regexp.QuoteMeta(reg) + `\b`).MatchString(l) {
					mention = true
					break
				}
			}
			if !mention {
				continue
			}
			if r := resultReg(l); r != "" {
				owned[r] = true
			}
			run = append(run, normalizeRegs(strings.TrimSpace(l), owned))
		}
		return run
	}
	t.Fatalf("no %d-byte box:\n%s", size, ir)
	return nil
}

// wantRun asserts one construction's whole instruction sequence, line for
// line. A face's shape is its sequence: the offsets, which word holds
// what, and what is not emitted between them.
func wantRun(t *testing.T, ir string, size int, want []string) {
	t.Helper()
	got := boxRun(t, ir, size)
	if len(got) != len(want) {
		t.Fatalf("box of %d bytes emitted %d instructions, want %d\n got:\n%s\nwant:\n%s\n--\n%s",
			size, len(got), len(want), strings.Join(got, "\n"), strings.Join(want, "\n"), ir)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("box instruction %d = %q, want %q\n--\n%s", i, got[i], want[i], ir)
		}
	}
}

// dynModule builds one module whose declarations are the given source,
// checked for real: the box's payload face comes from the registry the
// check hands over (design D1), so a source-driven test is the only kind
// that reaches the construction at all.
func dynModule(t *testing.T, src string) (*ast.File, *typecheck.Shapes) {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	return f, sh
}

const dynPrelude = `import std.io

pub type AppError = Failed(String)

interface Describe {
    fn describe(self) -> String
}
`

// A scalar-carrying box: the payload is one word, no descriptor is needed
// at all — the word is null and the collector skips the block's contents
// whole (design D5 decision 3/4, gc.c:174-176) — and the table pointer is
// the only other word behind the header. There is no room for a third:
// the box carries no type tag, because a box has no downcast face to read
// one.
func TestBoxScalarPayloadHasNoDescriptor(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
newtype Celsius(Int64)

impl Describe for Celsius {
    fn describe(self) -> String {
        "c"
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Describe>(Celsius(1))
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantRun(t, ir, 32, []string{
		"%box = call ptr @__we_alloc(i64 32)",
		"store ptr null, ptr %box",
		"call void @__we_root_push(ptr %box)",
		"%box = getelementptr i8, ptr %box, i64 16",
		"store ptr null, ptr %box",
		"%box = getelementptr i8, ptr %box, i64 24",
		"store i64 1, ptr %box",
	})
	wantNoIR(t, ir, "@.dynmap", "a descriptor for an all-scalar payload")
}

// A gc-carrying box: one traced payload word at 24, and the descriptor
// that says so. The bitmap is bit 1 — the box's first payload word — so
// the constant reads [i64 2], the same shape @.fnmap carries: both are
// one code-or-table pointer in the header's shadow followed by one gc
// word. The payload arrives through the ownership read every abiGc
// position takes, so a value-category record enters as a copy the box
// owns.
func TestBoxGcPayloadDescriptorMatchesTheFnCarrier(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
byval record Point {
    x: Int64,
    y: Int64,
}

impl Describe for Point {
    fn describe(self) -> String {
        "point"
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Describe>(Point { x: 1, y: 2 })
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@.dynmap0 = private unnamed_addr constant [1 x i64] [i64 2]",
		"the one-word gc payload's descriptor")
	wantRun(t, ir, 32, []string{
		"%box = call ptr @__we_alloc(i64 32)",
		"store ptr @.dynmap0, ptr %box",
		"call void @__we_root_push(ptr %box)",
		"%box = getelementptr i8, ptr %box, i64 16",
		"store ptr null, ptr %box",
		"%box = getelementptr i8, ptr %box, i64 24",
		"store ptr %r, ptr %box",
	})
}

// A String-carrying box: two words and neither traced. A String's bytes
// are malloc'd, outside the gc domain, so the data word is no gc
// reference and the descriptor stays null — the posture a String field
// takes in a record's layout.
func TestBoxStringPayloadIsTwoUntracedWords(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
newtype Tag(String)

impl Describe for Tag {
    fn describe(self) -> String {
        "t"
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Describe>(Tag("x"))
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantRun(t, ir, 40, []string{
		"%box = call ptr @__we_alloc(i64 40)",
		"store ptr null, ptr %box",
		"call void @__we_root_push(ptr %box)",
		"%box = getelementptr i8, ptr %box, i64 16",
		"store ptr null, ptr %box",
		"%box = getelementptr i8, ptr %box, i64 24",
		"store ptr @.s0, ptr %box",
		"%box = getelementptr i8, ptr %box, i64 32",
		"store i64 1, ptr %box",
	})
	wantNoIR(t, ir, "@.dynmap", "a descriptor for a String payload")
}

// A sum-carrying box: the whole three-word ABI, tag first, whether or not
// the widest variant fills them — the words are what a reader rebuilds.
// An all-scalar sum's payload holds no gc reference, so the box carries
// no descriptor.
func TestBoxSumPayloadCarriesTheThreeWords(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
type Shape = Circle(Int64) | Dot

impl Describe for Shape {
    fn describe(self) -> String {
        "s"
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Describe>(Circle(1))
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// The words are the sum's own ABI read back out of its slot — tag,
	// first payload, second — not constants written into the box: a box
	// carries what the value already is.
	wantRun(t, ir, 48, []string{
		"%box = call ptr @__we_alloc(i64 48)",
		"store ptr null, ptr %box",
		"call void @__we_root_push(ptr %box)",
		"%box = getelementptr i8, ptr %box, i64 16",
		"store ptr null, ptr %box",
		"%box = getelementptr i8, ptr %box, i64 24",
		"store i64 %r, ptr %box",
		"%box = getelementptr i8, ptr %box, i64 32",
		"store i64 %r, ptr %box",
		"%box = getelementptr i8, ptr %box, i64 40",
		"store i64 %r, ptr %box",
	})
	wantNoIR(t, ir, "@.dynmap", "a descriptor for an all-scalar sum")
}

// A sum whose variants agree that a payload word is a gc handle: the
// descriptor traces that word (bit 2, the word at 32) and nothing else.
// The agreement is what makes one compile-time constant exact — the tag
// that says which variant is live is a run-time value, so a position two
// variants read differently has no descriptor to write.
func TestBoxSumDescriptorTracesTheAgreedWord(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
record Inner {
    n: Int64,
}

type Shape = Circle(Inner) | Dot

impl Describe for Shape {
    fn describe(self) -> String {
        "s"
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Describe>(Circle(Inner { n: 1 }))
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "@.dynmap0 = private unnamed_addr constant [1 x i64] [i64 4]",
		"the first payload word's bit")
}

// Two variants that read one payload word differently stop: one
// descriptor covers every variant, and no bitmap describes a word that is
// a handle under one tag and a scalar under another.
func TestBoxSumWithDisagreeingVariantsStops(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
record Inner {
    n: Int64,
}

type Shape = Circle(Int64) | Boxed(Inner)

impl Describe for Shape {
    fn describe(self) -> String {
        "s"
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let d = Dyn<Describe>(Circle(1))
    return Ok(())
}
`)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni == nil {
		t.Fatal("a sum with no single payload face boxed")
	}
}

// One descriptor serves every box whose payload face agrees, because the
// trace bits are all a descriptor carries. Four constructions — three gc
// records and one scalar newtype — leave exactly one @.dynmap global, and
// the three gc boxes are the three that name it.
func TestBoxDescriptorsDedupeByPayloadFace(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
record Point {
    x: Int64,
}

record Spot {
    y: Int64,
}

newtype Celsius(Int64)

impl Describe for Point {
    fn describe(self) -> String {
        "point"
    }
}

impl Describe for Spot {
    fn describe(self) -> String {
        "spot"
    }
}

impl Describe for Celsius {
    fn describe(self) -> String {
        "c"
    }
}

pub fn main() effect io -> Result<(), AppError> {
    let a = Dyn<Describe>(Point { x: 1 })
    let b = Dyn<Describe>(Spot { y: 2 })
    let c = Dyn<Describe>(Celsius(3))
    let e = Dyn<Describe>(Point { x: 4 })
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	if got := dynGlobals(ir); len(got) != 1 || got[0] != "@.dynmap0" {
		t.Fatalf("four boxes of two payload faces left descriptors %v, want [@.dynmap0]\n--\n%s", got, ir)
	}
	if n := strings.Count(ir, "store ptr @.dynmap0, ptr %"); n != 3 {
		t.Fatalf("the three gc boxes took the descriptor %d times, want 3\n--\n%s", n, ir)
	}
}

// The box's ABI is one word (design D5 decision 6): a box crosses as a gc
// handle in a parameter and in a return, through the pointer face every
// record already uses, so the signature carries nothing the ABI did not
// already have.
func TestBoxCrossesAsOnePointerWord(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
record Point {
    x: Int64,
}

impl Describe for Point {
    fn describe(self) -> String {
        "point"
    }
}

fn make(v: Point) -> Dyn<Describe> {
    return Dyn<Describe>(v)
}

fn take(d: Dyn<Describe>) {
    let e = d
}

pub fn main() effect io -> Result<(), AppError> {
    let d = make(Point { x: 1 })
    take(d)
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "define ptr @main.make(ptr %v)", "the box's return face")
	wantIR(t, ir, "define void @main.take(ptr %d)", "the box's parameter face")
}

// A `where`-bound position boxes like any other concrete type: the check
// stage walked the generic body once, with the declaration's own position
// still open, and what it recorded there is the instantiation's to
// resolve. The define carries the argument the call determined, and the
// payload is the argument's face.
func TestBoxOfABoundedPositionResolvesAtTheInstantiation(t *testing.T) {
	f, sh := dynModule(t, dynPrelude+`
record Point {
    x: Int64,
}

impl Describe for Point {
    fn describe(self) -> String {
        "point"
    }
}

fn boxed<T>(v: T) -> Dyn<Describe> where T: Describe {
    return Dyn<Describe>(v)
}

pub fn main() effect io -> Result<(), AppError> {
    let d = boxed(Point { x: 1 })
    return Ok(())
}
`)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	if n := strings.Count(ir, "define ptr @main.boxed$main.Point("); n != 1 {
		t.Fatalf("the bounded position emitted %d defines, want 1\n--\n%s", n, ir)
	}
	wantIR(t, ir, "@.dynmap0 = private unnamed_addr constant [1 x i64] [i64 2]",
		"the resolved argument's descriptor")
}

// A construction the registry recorded nothing for leaves the box
// unemitted rather than having a payload face assumed: the check stage is
// the authority on what a box carries (design D1), and a tree that
// reached the emitter without it stops at the boundary.
func TestBoxWithoutARecordedSiteStops(t *testing.T) {
	_, ni := EmitProgram(ModeBuild, []ProgModule{instModule(nil,
		letBind("d", calledWith("Dyn", targs(named("Describe")), intLit("1"))),
		okReturn(),
	)})
	if ni == nil {
		t.Fatal("a box with no recorded site emitted")
	}
}
