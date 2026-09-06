package typecheck

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/parser"
)

// M8 (stdlib-and-gc) checker tests: the std.io loading face, the call and
// shadowing behavior around it, and the combinator real-body walk (design
// D1–D3). Negative sources mirror the T1 golden table verbatim so codes,
// message fragments, and positions stay identical to the CLI-face contract.

// --- D1/D2: std.io loads; calls carry effect io -----------------------------

func TestStdIoLoading(t *testing.T) {
	// A declared-io function calls through the module; print and println.
	wantOK(t, "import std.io\n\nfn work() effect io {\n    io.println(\"x\")\n    io.print(\"y\")\n    return\n}\n")
	// The alias form introduces its own name.
	wantOK(t, "import std.io as out\n\nfn work() effect io {\n    out.println(\"x\")\n    return\n}\n")
	// println returns unit: binding it and discarding stays green.
	wantOK(t, "import std.io\n\nfn work() effect io {\n    let u = io.println(\"x\")\n    let _ = u\n    return\n}\n")
	// println fits a fn-type slot with an io segment, and calls through it.
	wantOK(t, "import std.io\n\nfn work(f: fn(String) io -> ()) effect io {\n    f(\"x\")\n    return\n}\n\nfn caller() effect io {\n    work(io.println)\n    return\n}\n")
}

// --- D1: unknown std paths report E1302's std form ----------------------------

func TestStdUnknownModule(t *testing.T) {
	wantDiagProject(t, "import std.json\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
		"E1302", `no standard-library module "std.json" exists in this build`, 1, 8)
	wantDiagProject(t, "import std\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
		"E1302", `no standard-library module "std" exists in this build`, 1, 8)
	// Single-file mode has no source root, but std is not a file-system
	// module: the same std-form E1302 answers there too.
	wantDiag(t, "import std.json\n\nfn work() {\n    return\n}\n",
		"E1302", `no standard-library module "std.json" exists in this build`, 1, 8)
	// And std.io itself works in single-file mode — no source root needed.
	wantOK(t, "import std.io\n\nfn work() effect io {\n    io.println(\"x\")\n    return\n}\n")
}

// --- D2: the call-site judgment meets the first real library function --------

func TestStdIoCallEffect(t *testing.T) {
	wantDiag(t, "import std.io\n\nfn work() {\n    io.println(\"x\")\n    return\n}\n",
		"E1401", `"println" performs effect "io" which "work" does not declare`, 4, 5)
	// The difference set: a net-declaring function still owes io.
	wantDiag(t, "import std.io\n\nfn work() effect net {\n    io.println(\"x\")\n    return\n}\n",
		"E1401", `"println" performs effect "io" which "work" does not declare`, 4, 5)
	// A top-level initializer meets E1405 with the library name.
	wantDiagProject(t, "import std.io\n\nlet banner = io.println(\"boot\")\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n",
		"E1405", `the initializer of "banner" calls "println", which performs effect "io"`, 3, 14)
	// Argument typing is the ordinary E0501 site.
	wantDiag(t, "import std.io\n\nfn work() effect io {\n    io.println(42)\n    return\n}\n",
		"E0501", `the argument is Int64, the parameter is String`, 4, 16)
}

// --- D1: shadowing around the import name -------------------------------------

func TestStdIoShadow(t *testing.T) {
	// A module-level fn cannot shadow the import name: one module has one
	// name space, and the parser itself rejects the duplicate before any
	// std loading runs (golden check-stdio-shadow-fn rides this face).
	src := "import std.io\n\nfn io() {\n    return\n}\n\nfn work() {\n    io.println(\"x\")\n    return\n}\n"
	_, d, ni := parser.Parse("test.we", []byte(src))
	if ni != nil || d == nil || d.Code() != "E0404" || !strings.Contains(d.Message(), `"io" is already declared at line 1`) {
		t.Fatalf("want parse-stage E0404 name collision, got d=%v ni=%+v", d, ni)
	}
	// A block-local binding shadows the import name (P5): io resolves to
	// the Int64 local, so the form is member access on a base type —
	// println is no anchored Int64 member, and un-anchored stdlib members
	// stop at the honest boundary (design D10: never privately rejected).
	// E1304's qualifier form belongs to the undeclared-receiver face only.
	wantBnd(t, "import std.io\n\nfn work() {\n    let io = 1\n    io.println(\"x\")\n    return\n}\n",
		"standard-library modules (chapter 15)")
	// Without the import at all, the same qualifier face answers (lock).
	wantDiag(t, "fn work() {\n    io.println(\"x\")\n    return\n}\n",
		"E1304", `the qualifier "io" of "io.println" is not an import name`, 2, 5)
}

// --- D3: combinator real bodies walk the checker like user code ---------------

func TestCombinatorRealBodies(t *testing.T) {
	// The natural fold closure: U is determined by the initial value and
	// threads into the closure's parameter typing (the M6a gap this fixes).
	wantOK(t, "fn work(xs: List<Int64>) -> Int64 {\n    let total = xs.iterator().fold(0, |acc, x| acc + x)\n    return total\n}\n")
	// An effectful closure at a combinator's pure slot is E1402 at the
	// argument position (the M7 weaving gap this fixes).
	wantDiag(t, "effect db\n\nfn save(s: Int64) effect db {\n    return\n}\n\nfn work(xs: List<Int64>) {\n    let ys = xs.iterator().map(|x| save(x))\n    let _ = ys\n    return\n}\n",
		"E1402", `the value performs effect "db" and the expected type's effect segment does not include it`, 8, 32)
}

// --- D3: the green combinator faces stay green with real bodies (locks) -------

func TestCombinatorGreenFaces(t *testing.T) {
	wantOK(t, "fn work(xs: List<Int64>) {\n    let ys = xs.iterator().map(|x| x).collect()\n    let _ = ys\n    return\n}\n")
	wantOK(t, "fn work(xs: List<Int64>) {\n    let ys = xs.iterator().map(|x| x + 1).map(|x| x * 2).collect()\n    let _ = ys\n    return\n}\n")
	wantOK(t, "fn work(xs: List<Int64>) {\n    let ys = xs.iterator().skip(1).take(2).collect()\n    let _ = ys\n    return\n}\n")
	wantOK(t, "fn work(xs: List<Int64>) -> Bool {\n    let hit = xs.iterator().any(|x| x > 1)\n    let all = xs.iterator().all(|x| x > 1)\n    let found = xs.iterator().find(|x| x > 1)\n    let _ = all\n    let _ = found\n    return hit\n}\n")
	wantOK(t, "fn work(xs: List<Int64>) {\n    let ys = xs.iterator().filter(|x| x > 1).collect()\n    let n = xs.iterator().count()\n    let red = xs.iterator().reduce(|a, b| a + b)\n    let _ = ys\n    let _ = n\n    let _ = red\n    return\n}\n")
}
