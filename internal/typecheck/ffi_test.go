package typecheck

import (
	"testing"
)

// M12 (ffi) chapter 19 checker tests (design D2/D3): the crossing set
// per position (E1705/E1706), the opaque discipline (E1707), the byres
// discipline as chapter 13's existing machine with zero new rules, the
// effects as zero new mechanism (E1401 through a foreign callee, the
// bare segment as the empty set), and the mock exclusion (E1804).
// Negative sources mirror the T1 golden table verbatim so codes,
// message fragments, and positions stay identical to the CLI-face
// contract. Written test-first: red today on the parse boundary alone
// (the foreign block does not parse yet), the assertions themselves
// naming the D2 contract.

// --- D2: the crossing set, parameter and return positions ------------------------

func TestForeignCrossingSetParams(t *testing.T) {
	// The full legal parameter set parses and checks clean: the eight
	// integers, both floats, Bool, Rune, String, Bytes, and an opaque.
	wantOK(t, "foreign \"c\" {\n    record Box { }\n    fn all(k: Int8, n: Int64, u: UInt8, f: Float32, g: Float64, b: Bool, r: Rune, s: String, bs: Bytes, o: Box) effect io -> Int64\n}\n\nfn work() effect io {\n    return\n}\n")
	// A composite parameter does not cross.
	wantDiag(t, "foreign \"c\" {\n    fn sum(xs: List<Int64>) effect io -> Int64\n}\n",
		"E1705", `parameter "xs" holds List<Int64>, which does not cross`, 2, 16)
	// A fn type parameter does not cross (the callback face).
	wantDiag(t, "foreign \"c\" {\n    fn registerCallback(cb: fn(Int64) -> Int64) effect io -> Int64\n}\n",
		"E1705", `parameter "cb" holds fn(Int64) -> Int64, which does not cross`, 2, 29)
	// Never in parameter position does not cross.
	wantDiag(t, "foreign \"c\" {\n    fn poison(x: Never) effect io -> Int64\n}\n",
		"E1705", `parameter "x" holds Never, which does not cross; Never is a declared return position only`, 2, 18)
}

func TestForeignCrossingSetReturns(t *testing.T) {
	// String and Bytes returns are E1706's own face, ahead of E1705.
	wantDiag(t, "foreign \"c\" {\n    fn errmsg() effect io -> String\n}\n",
		"E1706", `"errmsg" declares a String return`, 2, 30)
	wantDiag(t, "foreign \"c\" {\n    fn take() effect io -> Bytes\n}\n",
		"E1706", `"take" declares a Bytes return`, 2, 28)
	// Never in return position is legal — the one trust point.
	wantOK(t, "foreign \"c\" {\n    fn abortNow() effect io -> Never\n}\n\nfn work() effect io {\n    return\n}\n")
	// A composite return outside String/Bytes is E1705.
	wantDiag(t, "foreign \"c\" {\n    fn pair() effect io -> (Int64, Int64)\n}\n",
		"E1705", `the return type holds (Int64, Int64), which does not cross`, 2, 28)
}

// --- D2: the opaque discipline ----------------------------------------------------

func TestForeignOpaqueDiscipline(t *testing.T) {
	// Construction of an opaque type is E1707, ahead of E0604.
	wantDiag(t, "foreign \"c\" {\n    record Socket { }\n}\n\nfn work() {\n    let s = Socket { }\n    let _ = s\n    return\n}\n",
		"E1707", `Socket is minted on the native side only`, 6, 13)
	// Update is E1707 alike.
	wantDiag(t, "foreign \"c\" {\n    record Socket { }\n    fn socket() effect io -> Socket\n}\n\nfn work() effect io {\n    let s = socket()\n    let t = Socket { with &s }\n    let _ = t\n    return\n}\n",
		"E1707", `Socket is minted on the native side only`, 8, 13)
	// A value obtained from a foreign fn's declared return is fine.
	wantOK(t, "foreign \"c\" {\n    record Socket { }\n    fn socket() effect io -> Socket\n}\n\nfn work() effect io {\n    let s = socket()\n    let _ = s\n    return\n}\n")
}

// --- D2: the byres discipline (chapter 13's machine, zero new rules) --------------

func TestForeignByresDiscipline(t *testing.T) {
	// A byres opaque record is a resource record: no impl of Releasable
	// in its own module is E1101 verbatim.
	wantDiag(t, "foreign \"c\" {\n    byres record File { }\n    fn open(path: String) effect io -> File\n}\n",
		"E1101", `"File" declares no impl of Releasable in its own module`, 2, 18)
	// With the impl the full lifecycle is clean: scope resource, transfer
	// by argument, release body calling a foreign close. Two spellings the
	// early drafts got wrong and the real parser/checker rejected once the
	// forms landed: the foreign entries carry no body (the declaration is
	// the whole entry), and release restates Releasable's segmentless
	// signature exactly (E1404) — so close crosses with the bare segment,
	// the explicit pure claim a foreign block alone allows, freeing the
	// release body to call it (release semantics sit outside the effect
	// system).
	wantOK(t, "foreign \"c\" {\n    byres record File { }\n    fn open(path: String) effect io -> File\n    fn close(fd: File) effect\n}\n\nimpl Releasable for File {\n    fn release(mut self) {\n        let _ = close(self)\n        return\n    }\n}\n\nfn take(f: File) effect io {\n    scope resource(g = f) {\n    }\n    return\n}\n\nfn work() effect io {\n    scope resource(f = open(\"a.txt\")) {\n        take(f)\n    }\n    return\n}\n")
	// A composite position rejects the resource opaque (E1106 verbatim).
	wantDiag(t, "foreign \"c\" {\n    byres record File { }\n    fn open(path: String) effect io -> File\n}\n\nimpl Releasable for File {\n    fn release(mut self) {\n        return\n    }\n}\n\nfn work() effect io {\n    let xs: List<File> = []\n    let _ = xs\n    return\n}\n",
		"E1106", `"File" appears as a generic argument at the instantiation`, 13, 18)
}

// --- D3: effects — zero new mechanism ----------------------------------------------

func TestForeignEffects(t *testing.T) {
	// A foreign callee's segment is the callee's set at an ordinary call.
	wantDiag(t, "foreign \"c\" {\n    fn ioWrite(fd: Int64, count: Int64) effect io -> Int64\n}\n\nfn pure(count: Int64) -> Int64 {\n    let n = ioWrite(1, count)\n    return n\n}\n",
		"E1401", `"ioWrite" performs effect "io" which "pure" does not declare`, 6, 13)
	// A custom tag behaves alike end to end.
	wantDiag(t, "effect gpio\n\nforeign \"c\" {\n    fn readPin(pin: Int64) effect gpio -> Int64\n}\n\nfn caller(pin: Int64) -> Int64 {\n    let v = readPin(pin)\n    return v\n}\n",
		"E1401", `"readPin" performs effect "gpio" which "caller" does not declare`, 8, 13)
	// The bare segment states the empty set: callable from a pure fn.
	wantOK(t, "foreign \"c\" {\n    fn abs(x: Int64) effect -> Int64\n}\n\nfn pure(x: Int64) -> Int64 {\n    let n = abs(x)\n    return n\n}\n")
}

// --- D2: the mock exclusion --------------------------------------------------------

func TestForeignMockExcluded(t *testing.T) {
	// A foreign fn is not a mockable target (E1804's category face). The
	// mock restates the signature in chapter 20's order (the declared
	// return before the segment) and omits the segment — the grammar has
	// no bare-segment form there, and the target's bare segment is the
	// empty set either way; the category judgment fires first regardless.
	// The test block needs its _test.we module, as every mock test does.
	wantDiagAs(t, "tests/t_test.we", "foreign \"c\" {\n    fn abs(x: Int64) effect -> Int64\n}\n\ntest \"t\" {\n    mock abs(x: Int64) -> Int64 {\n        return 0\n    }\n    assert(true, \"done\")\n}\n",
		"E1804", `abs is a foreign function; only a module-level monomorphic fn is mockable`, 6, 10)
}
