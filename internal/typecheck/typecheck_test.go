package typecheck

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/parser"
)

// --- helpers ---------------------------------------------------------------

// runCheck parses clean, then runs the checker in the given mode.
func runCheck(t *testing.T, src string, mode Mode) (*diag.Diagnostic, *NotImplemented) {
	t.Helper()
	f, d, ni := parser.Parse("test.we", []byte(src))
	if d != nil {
		t.Fatalf("parse diagnostic: %s", d.Human())
	}
	if ni != nil {
		t.Fatalf("parse boundary: %s", ni.What)
	}
	return Check(f, "test.we", mode)
}

func wantOK(t *testing.T, src string) {
	t.Helper()
	if d, ni := runCheck(t, src, SingleFile); d != nil || ni != nil {
		t.Fatalf("want clean, got d=%v ni=%+v", d, ni)
	}
}

func wantOKProject(t *testing.T, src string) {
	t.Helper()
	if d, ni := runCheck(t, src, Project); d != nil || ni != nil {
		t.Fatalf("want clean, got d=%v ni=%+v", d, ni)
	}
}

// wantDiag asserts the one diagnostic with code, message fragment, and
// position (through the JSON rendering, whose field order the protocol
// fixes).
func wantDiag(t *testing.T, src, code, part string, line, col int) {
	t.Helper()
	d, ni := runCheck(t, src, SingleFile)
	if ni != nil {
		t.Fatalf("expected %s, got boundary %q", code, ni.What)
	}
	if d == nil {
		t.Fatalf("expected %s, got a clean check: %q", code, src)
	}
	if d.Code() != code {
		t.Fatalf("expected code %s, got %s (%s)", code, d.Code(), d.Message())
	}
	if !strings.Contains(d.Message(), part) {
		t.Fatalf("message %q missing %q", d.Message(), part)
	}
	pos := `"line":` + itoa(line) + `,"column":` + itoa(col)
	if !strings.Contains(d.JSON(), pos) {
		t.Fatalf("expected position %d:%d, got %s", line, col, d.JSON())
	}
}

func wantDiagProject(t *testing.T, src, code, part string, line, col int) {
	t.Helper()
	f, dni, pni := parser.Parse("src/main.we", []byte(src))
	if dni != nil || pni != nil {
		t.Fatalf("parse failed: %v %+v", dni, pni)
	}
	d, ni := Check(f, "src/main.we", Project)
	if ni != nil {
		t.Fatalf("expected %s, got boundary %q", code, ni.What)
	}
	if d == nil {
		t.Fatalf("expected %s, got a clean check", code)
	}
	if d.Code() != code || !strings.Contains(d.Message(), part) {
		t.Fatalf("expected %s (%q), got %s (%s)", code, part, d.Code(), d.Message())
	}
	pos := `"line":` + itoa(line) + `,"column":` + itoa(col)
	if !strings.Contains(d.JSON(), pos) {
		t.Fatalf("expected position %d:%d, got %s", line, col, d.JSON())
	}
}

// wantBnd asserts the checker-level not-implemented boundary with its
// exact What string (design D14's closed table).
func wantBnd(t *testing.T, src, what string) {
	t.Helper()
	d, ni := runCheck(t, src, SingleFile)
	if d != nil {
		t.Fatalf("expected boundary %q, got diagnostic: %s", what, d.Human())
	}
	if ni == nil {
		t.Fatalf("expected boundary %q, got a clean check: %q", what, src)
	}
	if ni.What != what {
		t.Fatalf("boundary What %q, want %q", ni.What, what)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// The boundary Whats come from the implementation's closed table (design
// D14); the conformance goldens pin the same strings through the CLI face.

// --- D4: literal typing ------------------------------------------------------

func TestLiterals(t *testing.T) {
	// Context-free defaults: unsuffixed 42 is Int64, never retyped.
	wantDiag(t, "let n: Int32 = 42\n", "E0501", "the expression is Int64, the annotation is Int32", 1, 5)
	wantOK(t, "let n: Int32 = 42i32\n")
	wantOK(t, "let f: Float32 = 1.0f32\n")
	wantDiag(t, "let f: Float64 = 1\n", "E0501", "the expression is Int64, the annotation is Float64", 1, 5)
	wantOK(t, "let b: Bool = true\n")
	wantDiag(t, "let b: Bool = 1\n", "E0501", "the expression is Int64, the annotation is Bool", 1, 5)
	wantOK(t, "let s: String = \"hi\"\n")
	wantOK(t, "let c: Rune = 'x'\n")
	// Every base type is usable as an annotation; the suffix set names them
	// (Bytes has no literal form in this subset — its values come from
	// methods, chapter 17's).
	wantOK(t, "let a: Int8 = 1i8\nlet b: UInt64 = 1u64\nlet c: Float32 = 1.5f32\n")
}

// --- D4: operator domains and E0501 anchors -----------------------------------

func TestOperators(t *testing.T) {
	wantOK(t, "let x = 1 + 2\n")
	wantOK(t, "let x = 1i32 + 2i32\n")
	wantOK(t, "let x = 1.5 + 2.5\n")
	wantOK(t, "let x = 1 & 2\n")
	wantOK(t, "let x = 1 << 2\n")
	wantOK(t, "let x = 1 < 2\n")
	wantOK(t, "let x = \"a\" == \"b\"\n")
	wantOK(t, "let x = true && false\n")
	wantOK(t, "let x = !true\n")
	wantOK(t, "let x = -1\n")
	wantOK(t, "let x = ~1\n")
	// Disagreement between operands is E0501 at the operator.
	wantDiag(t, "let x = 1.5 + 2\n", "E0501", `operands of "+" are Float64 and Int64`, 1, 13)
	wantDiag(t, "let x = true && 1\n", "E0501", `operands of "&&" are Bool and Int64`, 1, 14)
	// Unary domain violations are E0501 at the operator.
	wantDiag(t, "let x = -\"a\"\n", "E0501", `operand of "-" is String`, 1, 9)
	wantDiag(t, "let x = !1\n", "E0501", `operand of "!" is Int64`, 1, 9)
	wantDiag(t, "let x = ~1.5\n", "E0501", `operand of "~" is Float64`, 1, 9)
	// Same-type but beyond the ratified domains: the spec-gap boundary
	// (unary rows are E0501 per design D4's anchor table).
	wantBnd(t, "let x = \"a\" + \"b\"\n", bndDomainGap)
	wantBnd(t, "let x = \"a\" < \"b\"\n", bndDomainGap)
	wantBnd(t, "let x = 1.0 & 2.0\n", bndDomainGap)
	wantBnd(t, "let x = 1 && 2\n", bndDomainGap)
	wantDiag(t, "let x = ~true\n", "E0501", `operand of "~" is Bool`, 1, 9)
	// Ranges are chapter 11's.
	wantBnd(t, "let x = 1..2\n", bndRange)
}

// --- D5: E0502 constant folding ----------------------------------------------

func TestE0502Folding(t *testing.T) {
	wantDiag(t, "let x = 127i8 + 1i8\n", "E0502", "overflows Int8: 127 + 1 = 128", 1, 15)
	wantOK(t, "let x = -128i8\n")
	wantDiag(t, "let x = -129i8\n", "E0502", "overflows Int8", 1, 9)
	wantDiag(t, "let x = 9223372036854775808\n", "E0502", "overflows Int64", 1, 9)
	wantOK(t, "let x = 9223372036854775807\n")
	// The innermost producing operator carries the anchor.
	wantDiag(t, "let x = (127i8 + 1i8) * 2i8\n", "E0502", "overflows Int8", 1, 16)
	// Annotation agreement is judged before folding; floats are never E0502.
	wantDiag(t, "let n: Int32 = 42\n", "E0501", "the expression is Int64", 1, 5)
	wantOK(t, "let f = 1.0 / 0.0\n")
	wantOK(t, "let f: Float32 = 3.4e38f32\n")
}

// --- D6: block values, fn tails, E0605 ---------------------------------------

func TestBlockValueAndTails(t *testing.T) {
	wantOK(t, "fn f() -> Int64 {\n    let two = 2\n    two + two\n}\n")
	// Bare return against a declared return.
	wantDiag(t, "fn f() -> Int64 {\n    return\n}\n",
		"E0501", "the return expression is (), the declared return is Int64", 2, 5)
	// Tail expression against the declared return.
	wantDiag(t, "fn f() -> Int64 {\n    \"no\"\n}\n",
		"E0501", "the final expression is String, the declared return is Int64", 2, 5)
	// Binding tail in a valued fn: the fact anchors at the declaration.
	wantDiag(t, "fn f() -> Int64 {\n    let x = 1\n}\n",
		"E0501", "the body produces (), the declared return is Int64", 1, 1)
	wantDiag(t, "fn f() -> Int64 {\n}\n",
		"E0501", "the body produces ()", 1, 1)
	// Discards outside the unit type: an inner statement vs the fn's own
	// valueless tail (the tail's remediation adds the return-type escape).
	wantDiag(t, "fn f() {\n    1 + 2\n    let u = ()\n}\n",
		"E0605", "the statement's expression is Int64, not ()", 2, 5)
	wantDiag(t, "fn f() {\n    \"tail\"\n}\n",
		"E0605", "the final expression is String, not ()", 2, 5)
	wantOK(t, "fn f() {\n    let _ = 1 + 2\n}\n")
	wantOK(t, "fn f() {\n    let u = ()\n}\n")
	// A block's value is its final expression item; non-expression tails
	// make it the unit value.
	wantOK(t, "fn f() -> () {\n    return {\n        let t = 1\n    }\n}\n")
	wantDiag(t, "let u: Int64 = {\n    let t = 1\n}\n",
		"E0501", "the expression is (), the annotation is Int64", 1, 5)
	// Assignment against the binding's type.
	wantDiag(t, "fn f() {\n    var x = 1\n    x = 2.5\n}\n",
		"E0501", "the value is Float64, \"x\" is Int64", 3, 5)
}

// --- D7/D8: sums, constructors, E0703 ----------------------------------------

func TestSumsAndCtors(t *testing.T) {
	wantOK(t, "type Light = On | Off\nlet x = On\n")
	wantOK(t, "type Shape = Circle(Float64)\nlet c = Circle(1.0)\n")
	wantDiag(t, "type Shape = Circle(Float64)\nfn f() {\n    let g = Circle\n}\n",
		"E0704", `"Circle" carries a payload`, 3, 13)
	wantDiag(t, "type Shape = Circle(Float64)\nlet c = Circle(\"r\")\n",
		"E0501", "the argument is String, the parameter is Float64", 2, 16)
	// Argument-count and non-function callee diagnostics are not yet
	// ratified: the spec-gap boundaries.
	wantBnd(t, "type Light = On | Off\nlet x = On(1)\n", bndArityGap)
	wantBnd(t, "type Shape = Circle(Float64)\nlet c = Circle(1.0, 2.0)\n", bndArityGap)
	wantBnd(t, "fn id(x: Int64) -> Int64 {\n    return x\n}\nlet y = id(1, 2)\n", bndArityGap)
	wantBnd(t, "let y = 3(1)\n", bndCalleeGap)
	// A type name used as a value resolves to nothing.
	wantDiag(t, "type Shape = Circle(Float64)\nlet s = Shape\n",
		"E1304", `"Shape" is held by no scope`, 2, 9)
	// Monomorphic named fns call by signature; their bare name as a value
	// carries the fn type (chapter 12 landed with M5).
	wantOK(t, "fn id(x: Int64) -> Int64 {\n    return x\n}\nlet y = id(3)\n")
	wantOK(t, "fn id(x: Int64) -> Int64 {\n    return x\n}\nlet g: fn(Int64) -> Int64 = id\n")
	// E0703: the enumerated non-return positions of Never.
	wantDiag(t, "fn f() {\n    let x: Never = 1\n}\n",
		"E0703", "Never in a non-return annotation position", 2, 12)
	wantDiag(t, "fn f(x: Never) {\n    return\n}\n",
		"E0703", "Never in a non-return annotation position", 1, 9)
	wantDiag(t, "type X = Stop(Never)\n",
		"E0703", "Never in a non-return annotation position", 1, 15)
	wantDiag(t, "let t: (Int64, Never) = 1\n",
		"E0703", "Never in a non-return annotation position", 1, 16)
	wantDiag(t, "fn g(f: fn(Never) -> ()) {\n    return\n}\n",
		"E0703", "Never in a non-return annotation position", 1, 12)
	wantOK(t, "fn dead() -> Never {\n    return dead()\n}\n")
}

// --- D9: builtin Result and Option --------------------------------------------

func TestBuiltinSums(t *testing.T) {
	wantOK(t, "type AppError = Failed(String)\nlet r: Result<Int64, AppError> = Ok(1)\n")
	wantOK(t, "let o: Option<Int64> = Some(3)\n")
	wantOK(t, "let o: Option<Int64> = None\n")
	wantDiag(t, "fn f() -> Result<Int64, String> {\n    return Ok(1)\n}\n",
		"E1204", "the error parameter resolves to String", 1, 25)
	wantDiag(t, "fn f() -> Result<Int64, Never> {\n    return Ok(1)\n}\n",
		"E1204", "the error parameter resolves to Never", 1, 25)
	wantDiag(t, "fn f() -> Result<Int64> {\n    return Ok(1)\n}\n",
		"E0828", `"Result" wants 2 type arguments, got 1`, 1, 17)
	wantDiag(t, "type S = A | B\nfn f(x: S<Int64>) {\n    return\n}\n",
		"E0828", `"S" wants 0 type arguments, got 1`, 2, 10)
	wantDiag(t, "let o: Option<Int64, String> = None\n",
		"E0828", `"Option" wants 1 type argument, got 2`, 1, 14)
	wantDiag(t, "fn f() {\n    let x = Ok(())\n}\n",
		"E0827", `"Ok" does not determine the type arguments of Result`, 2, 13)
	wantDiag(t, "fn f() {\n    let x = None\n}\n",
		"E0827", `"None" does not determine the type arguments of Option`, 2, 13)
	// Expected-type threading reaches nested constructors (the nested
	// closers separated per chapter 10's rule).
	wantOK(t, "type E2 = Failed(String)\nlet r: Result<Int64, Result<(), E2> > = Err(Err(Failed(\"x\")))\n")
	// Variant names are not type names.
	wantDiag(t, "let x: Ok = 1\n", "E1304", `"Ok" is held by no scope`, 1, 8)
}

// --- D2/D3/D10: name resolution and the prelude -------------------------------

func TestNameResolution(t *testing.T) {
	wantDiag(t, "fn f() {\n    let x = mystery\n}\n",
		"E1304", `"mystery" is held by no scope`, 2, 13)
	wantDiag(t, "fn f(x: Foo) {\n    return\n}\n",
		"E1304", `"Foo" is held by no scope`, 1, 9)
	wantDiag(t, "fn f() {\n    let x = net.Read()\n}\n",
		"E1304", `the qualifier "net" of "net.Read" is not an import name`, 2, 13)
	// Prelude names without an M3 type are honest boundaries, never E1304.
	wantBnd(t, "fn f() {\n    let xs: List<Int64> = 1\n}\n", bndCollections)
	wantBnd(t, "fn f() {\n    let m: Map<String, Int64> = 1\n}\n", bndCollections)
	wantBnd(t, "fn f() {\n    panic(\"boom\")\n}\n", bndTermination)
	wantBnd(t, "fn f() {\n    todo()\n}\n", bndTermination)
	wantBnd(t, "fn f() {\n    assert(true)\n}\n", bndTermination)
	wantBnd(t, "fn f(x: Dyn) {\n    return\n}\n", bndDyn)
	wantBnd(t, "fn f(x: Dyn<Int64>) {\n    return\n}\n", bndDyn)
	wantBnd(t, "fn f(x: Shareable) {\n    return\n}\n", bndShareable)
	wantBnd(t, "fn f() {\n    let s = currentCancelSignal\n}\n", bndTaskTime)
	wantBnd(t, "fn f() {\n    advanceTime(1)\n}\n", bndTaskTime)
	// Local declarations shadow prelude names legally.
	wantOK(t, "fn panic(msg: String) -> Never {\n    return panic(msg)\n}\n\nfn f() { return }\n")
	wantOK(t, "type Option = A | B\nlet x: Option = A\n")
	// Base type names may be shadowed by variants, and uses resolve to
	// the module declaration.
	wantOK(t, "type T = Int64\nlet x = Int64\n")
	// Imports: std is not provided; single-file locals have no source root.
	wantBnd(t, "import std\n\nfn f() { return }\n", bndStdModules)
	wantDiag(t, "import util\n\nfn f() { return }\n",
		"E1302", `"util" cannot resolve in single-file mode`, 1, 8)
}

// --- D11/D12: project mode and the main shape ---------------------------------

func TestMainShape(t *testing.T) {
	skeleton := "type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n"
	wantOKProject(t, skeleton)
	wantDiagProject(t, "pub fn other() { return }\n",
		"E1305", "the root module declares no fn main", 1, 1)
	wantDiagProject(t, "fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\ntype AppError = Failed(String)\n",
		"E1305", "main must be pub", 1, 1)
	wantDiagProject(t, "pub fn main(x: Int64) -> Result<(), AppError> {\n    return Ok(x)\n}\n\ntype AppError = Failed(String)\n",
		"E1305", "it declares parameters", 1, 1)
	// A locally shadowed Result is no longer the convention's shape.
	wantDiagProject(t, "type Result = A | B\n\ntype AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n",
		"E1305", "its return is not Result<(), E>", 5, 1)
	// A well-shaped main whose error position is not a named sum is
	// E1204's (the registry draws that line).
	wantDiagProject(t, "pub fn main() -> Result<(), String> {\n    return Ok(())\n}\n",
		"E1204", "the error parameter resolves to String", 1, 29)
}

// --- D1: structural equality, through agreement outcomes ----------------------

func TestStructuralEquality(t *testing.T) {
	wantOK(t, "fn keep(o: Option<Int64>) -> Option<Int64> {\n    return o\n}\n")
	wantDiag(t, "fn keep(o: Option<Int64>) -> Option<Int32> {\n    return o\n}\n",
		"E0501", "the return expression is Option<Int64>, the declared return is Option<Int32>", 2, 12)
	wantDiag(t, "let t: (Int64, String) = 1\n",
		"E0501", "the annotation is (Int64, String)", 1, 5)
	wantOK(t, "fn apply(f: fn(Int64) -> Int64, x: Int64) -> Int64 {\n    return x\n}\n")
}
