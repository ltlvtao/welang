package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T11-1's face: the builtin List crosses a signature boundary as one gc
// handle whose element face the signature's argument fixed. Three positions
// carry that pair — a parameter slot (the carrier enters, bindTupleParams
// records the face and bindDefineParams routes it into listEnv), a return
// slot (fnRetOperand emits the carrier, the caller's callResult carries the
// face home), and an argument expression (emitListOperand names the carrier,
// the call passes it un-copied). The element face a signature may carry is
// read by the same gate everywhere: elemFaceOfType, whose entry refuses a
// nested collection — so a signature slot and an element read cannot
// disagree about nesting.
//
// The pins run through the real check pipeline (checkShapes): the face is
// about what the checker already accepts, so a bare Emit helper would only
// test the emitter against trees the checker never blessed.

// listAbiSrc is one program whose source declares tally over a List and
// whose main body is body — the skeleton every crossing pin varies.
func listAbiSrc(fn, body string) string {
	return `import std.io

pub type AppError = Failed(String)

` + fn + `

pub fn main() effect io -> Result<(), AppError> {
` + body + `    return Ok(())
}
`
}

// emitListAbi checks and emits one program, refusing boundaries: the face's
// own claim is that these programs build, so a boundary here is a failure of
// the fixture or of the face, never a fact to pin.
func emitListAbi(t *testing.T, src string) string {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	ir, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	if ni != nil {
		t.Fatalf("boundary %q over the list ABI face:\n%s", ni.What, src)
	}
	return ir
}

// listAbiStop checks and emits one program, answering its boundary: the
// negative pins' helper, the mirror of emitListAbi.
func listAbiStop(t *testing.T, src string) *NotImplemented {
	t.Helper()
	f, sh := checkShapes(t, "main.we", src)
	_, ni := EmitProgram(ModeBuild, []ProgModule{{Key: "main", ID: "demo", File: f, Shapes: sh}})
	return ni
}

// tallyDecl is the callee every crossing pin uses: one List parameter, one
// Int64 back, with walk as the body's own statements before the return. It
// declares io so the walk may println.
func tallyDecl(walk string) string {
	return "fn tally(xs: List<Int64>) effect io -> Int64 {\n" + walk + "    return 0\n}\n"
}

// fnDefine is the named define's own text: its line through the line before
// the next define.
func fnDefine(t *testing.T, ir, sym string) string {
	t.Helper()
	var lines []string
	start := -1
	for i, line := range strings.Split(ir, "\n") {
		if start < 0 {
			if strings.HasPrefix(line, "define ") && strings.Contains(line, "@"+sym+"(") {
				start = i
			}
			continue
		}
		if strings.HasPrefix(line, "define ") {
			return strings.Join(lines, "\n")
		}
		lines = append(lines, line)
	}
	if start < 0 {
		t.Fatalf("no define for %q:\n%s", sym, ir)
	}
	return strings.Join(lines, "\n")
}

// definedReg is the register the one line containing frag defines — the
// token before " =" on that line.
func definedReg(t *testing.T, ir, frag string) string {
	t.Helper()
	for _, line := range strings.Split(ir, "\n") {
		if strings.Contains(line, frag) {
			if reg := resultReg(line); reg != "" {
				return reg
			}
			t.Fatalf("%q defines no register:\n%s", frag, line)
		}
	}
	t.Fatalf("no line with %q:\n%s", frag, ir)
	return ""
}

// lastDefinedReg is definedReg over the last line containing frag — the
// register a functional growth ends on: a list literal's handle is its
// final push's result, not the empty list's.
func lastDefinedReg(t *testing.T, ir, frag string) string {
	t.Helper()
	reg := ""
	for _, line := range strings.Split(ir, "\n") {
		if strings.Contains(line, frag) {
			if r := resultReg(line); r != "" {
				reg = r
			}
		}
	}
	if reg == "" {
		t.Fatalf("no defining line with %q:\n%s", frag, ir)
	}
	return reg
}

// slotCallLine answers the one call line that goes through key's mock slot:
// a main-body call addresses its callee by the register the slot loaded —
// `call i64 %r(...)` — never by symbol.
func slotCallLine(t *testing.T, ir, key string) string {
	t.Helper()
	reg := definedReg(t, ir, "load ptr, ptr @slot."+key)
	for _, line := range strings.Split(ir, "\n") {
		if strings.Contains(line, "call ") && strings.Contains(line, reg+"(") {
			return line
		}
	}
	t.Fatalf("no call through %q's slot:\n%s", key, ir)
	return ""
}

// firstPtrOperand is a call line's first operand — the register handed
// across, whatever its name.
func firstPtrOperand(t *testing.T, line string) string {
	t.Helper()
	i := strings.Index(line, "(ptr ")
	if i < 0 {
		t.Fatalf("takes no ptr operand:\n%s", line)
	}
	rest := line[i+len("(ptr "):]
	if j := strings.IndexAny(rest, ",)"); j >= 0 {
		return rest[:j]
	}
	t.Fatalf("unterminated operand list:\n%s", line)
	return ""
}

// TestListParamCrossesTheSignature: the parameter takes the carrier in its
// own ABI slot — a ptr define named by the declaration — and the argument is
// the literal's own handle register, handed across un-copied: the register
// the construction ended on is the register the call receives.
func TestListParamCrossesTheSignature(t *testing.T) {
	ir := emitListAbi(t, listAbiSrc(tallyDecl(""), `    let xs = [1, 2, 3]
    io.println("${tally(xs)}")
`))
	wantIR(t, ir, "define i64 @main.tally(ptr %xs)", "the carrier is the parameter's whole slot")
	carrier := lastDefinedReg(t, ir, "@__we_list_push")
	if op := firstPtrOperand(t, slotCallLine(t, ir, "main.tally")); op != carrier {
		t.Fatalf("tally received %s, the literal's handle is %s:\n%s", op, carrier, ir)
	}
}

// TestListParamBindsTheWalkInsideTheCallee: the callee does not re-read the
// signature to walk its parameter — bindDefineParams routed the face into
// listEnv, so the body's for is the same snapshot walk a local binding gets.
func TestListParamBindsTheWalkInsideTheCallee(t *testing.T) {
	ir := emitListAbi(t, listAbiSrc(tallyDecl(`    for x in xs {
        io.println("${x}")
    }
`), `    let xs = [1, 2, 3]
    io.println("${tally(xs)}")
`))
	def := fnDefine(t, ir, "main.tally")
	wantIR(t, def, "call ptr @__we_list_snap", "the walk snapshots the carrier")
	wantIR(t, def, "call i64 @__we_list_get", "each element is the walk's own read")
}

// TestListReturnCarriesItsElementFace: a List return is the carrier in the
// return slot — a ptr define, a ptr call result in the caller — and the face
// rides that result: the caller's for walks without any annotation of its
// own, which only the carried face makes possible.
func TestListReturnCarriesItsElementFace(t *testing.T) {
	ir := emitListAbi(t, listAbiSrc(`fn make() -> List<Int64> {
    return [1, 2, 3]
}
`, `    let ys = make()
    for v in ys {
        io.println("${v}")
    }
`))
	wantIR(t, ir, "define ptr @main.make()", "the carrier is the return's whole slot")
	if line := slotCallLine(t, ir, "main.make"); !strings.Contains(line, "call ptr ") {
		t.Fatalf("make's result is not a carrier:\n%s", line)
	}
	wantIR(t, ir, "call ptr @__we_list_snap", "the returned face feeds the caller's walk")
	wantIR(t, ir, "call i64 @__we_list_get", "each element is the walk's own read")
}

// TestListCallArgumentRidesItsResult: a call whose argument is itself a call
// passes the inner result straight through — the register the make call
// bound is the register the tally call receives, with nothing materialized
// between them.
func TestListCallArgumentRidesItsResult(t *testing.T) {
	ir := emitListAbi(t, listAbiSrc(`fn make() -> List<Int64> {
    return [1, 2, 3]
}
`+tallyDecl(""), `    io.println("${tally(make())}")
`))
	made := resultReg(slotCallLine(t, ir, "main.make"))
	if op := firstPtrOperand(t, slotCallLine(t, ir, "main.tally")); op != made {
		t.Fatalf("tally received %s, make returned %s:\n%s", op, made, ir)
	}
}

// TestNestedListElementHasNoCarrierFace: a nested List names no element face
// — elemFaceOfType's entry refuses it, the classifier's List arm asks that
// same read, so the signature itself refuses and the define never emits.
// The separator spelling `> >` is the checker's own rule for nested generic
// closers, not this face's.
func TestNestedListElementHasNoCarrierFace(t *testing.T) {
	ni := listAbiStop(t, listAbiSrc(
		"fn f(xs: List<List<Int64> >) -> Int64 {\n    return 0\n}\n",
		"",
	))
	if ni == nil {
		t.Fatal("a nested List laid out a carrier face")
	}
	if ni.What != bndFnBody {
		t.Fatalf("boundary word: %q", ni.What)
	}
}

// TestStringElementListRefusesAtTheClassifier: a String element is two
// words, and this build's element slot is one, so the classifier refuses
// the signature — the same read that refuses the literal. T11-2 widens the
// element representation and this pin flips with it: re-anchor then.
func TestStringElementListRefusesAtTheClassifier(t *testing.T) {
	ni := listAbiStop(t, listAbiSrc(
		"fn f(xs: List<String>) -> Int64 {\n    return 0\n}\n",
		"",
	))
	if ni == nil {
		t.Fatal("a String element named an element face")
	}
	if ni.What != bndFnBody {
		t.Fatalf("boundary word: %q", ni.What)
	}
}

// TestModuleLevelListArgumentCrosses: the carrier's sources are the same
// three for a module-level binding — emitListOperand's Member arm reads the
// dependency's global, and that load's register is what the call receives.
func TestModuleLevelListArgumentCrosses(t *testing.T) {
	dep := ProgModule{Key: "u1", File: &ast.File{Items: []ast.Item{
		topLetDecl("xs", &ast.NamedType{Name: "List", Args: []ast.TypeRef{named("Int64")}},
			&ast.ListLit{Elems: []ast.Expr{intLit("1"), intLit("2"), intLit("3")}}),
	}}}
	// tally stays module-local and is still called through its mock slot —
	// every main-body call is — so the pin reads the slot's call line.
	root := topProg([]ast.Item{
		&ast.FnDecl{Name: "tally",
			Params: []ast.Param{param("xs", &ast.NamedType{Name: "List", Args: []ast.TypeRef{named("Int64")}})},
			Ret:    named("Int64"), Body: ast.Block{Items: []ast.Stmt{retValue(intLit("0"))}}},
	},
		letBind("n", &ast.Call{Fn: ident("tally"),
			Args: []ast.Expr{&ast.Member{Recv: ident("u1"), Name: "xs"}}}),
	)
	ir := topGcIR(t, dep, root)
	wantIR(t, ir, "define i64 @main.tally(ptr %xs)", "the carrier is the parameter's whole slot")
	held := definedReg(t, ir, "load ptr, ptr @u1.xs")
	if op := firstPtrOperand(t, slotCallLine(t, ir, "main.tally")); op != held {
		t.Fatalf("tally received %s, the global holds %s:\n%s", op, held, ir)
	}
}
