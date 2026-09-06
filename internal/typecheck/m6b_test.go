package typecheck

import (
	"testing"
)

// M6b (modules-errors) checker tests: chapter 13 resources (the
// Releasable declaration layer, the scope resource statement, the
// structured-liveness discipline, the alias ban, the composite-position
// ban), chapter 14 in full (the `?` propagation operator and the panic
// family's real prelude signatures) — design D2–D8. Chapter 15's
// multi-module loading is CLI-face (design D9): its contract is pinned
// by the T1 project-mode goldens and gets loader tests in T7.
// Negative sources mirror the T1 golden table verbatim so codes,
// message fragments, and positions stay identical to the CLI-face
// contract.

const m6bPrelude = "byres record FileHandle { fd: Int64 }\nimpl Releasable for FileHandle {\n    fn release(mut self) {\n        return\n    }\n}\n\nfn openFile() -> FileHandle {\n    return FileHandle { fd: 1 }\n}\n"

const m6bTake = "fn take(f: FileHandle) {\n    scope resource(g = f) {\n        let n = g.fd\n    }\n}\n"

// --- D2: the Releasable declaration layer ---------------------------------------

func TestReleasableDecl(t *testing.T) {
	// The obligation is module-level: an impl later in the file satisfies
	// it (golden ch13-impl-late-green).
	wantOK(t, "fn use() {\n    scope resource(f = openFile()) {\n        let n = f.fd\n    }\n}\n\nfn openFile() -> FileHandle {\n    return FileHandle { fd: 1 }\n}\n\nbyres record FileHandle { fd: Int64 }\n\nimpl Releasable for FileHandle {\n    fn release(mut self) {\n        return\n    }\n}\n")
	wantOK(t, m6bPrelude)
	// E1101 anchors the record head name; the message names the record.
	wantDiag(t, "byres record FileHandle { fd: Int64 }\n\nfn f() {\n    return\n}\n",
		"E1101", `"FileHandle" declares no impl of Releasable`, 1, 14)
	// E1102 anchors the impl head; gc and sum heads alike.
	wantDiag(t, "record User { name: String }\n\nimpl Releasable for User {\n    fn release(mut self) {\n        return\n    }\n}\n",
		"E1102", `"User" is not a byres record`, 3, 1)
	wantDiag(t, "pub type Bag = Has(Int64)\n\nimpl Releasable for Bag {\n    fn release(mut self) {\n        return\n    }\n}\n",
		"E1102", `"Bag" is not a byres record`, 3, 1)
	// A wrong release signature is chapter 10's E0808, not a new code
	// (golden e0808-release-sig): the receiver must be mut self.
	wantDiag(t, "byres record FileHandle { fd: Int64 }\nimpl Releasable for FileHandle {\n    fn release(self) {\n        return\n    }\n}\n",
		"E0808", `the receiver of "release" is self in the impl and mut self in "Releasable"`, 3, 8)
}

// --- D3: the scope resource statement --------------------------------------------

func TestScopeResourceStmt(t *testing.T) {
	wantOK(t, m6bPrelude+"\nfn use() {\n    scope resource(f = openFile()) {\n        let n = f.fd\n    }\n}\n")
	// E1103 anchors the head expression's first token (golden pair).
	wantDiag(t, "fn f() {\n    scope resource(x = 1) {\n        return\n    }\n}\n",
		"E1103", `the head expression's type "Int64" implements no "Releasable"`, 2, 24)
	wantDiag(t, "record User { name: String }\n\nfn f() {\n    scope resource(u = User { name: \"n\" }) {\n        return\n    }\n}\n",
		"E1103", `the head expression's type "User" implements no "Releasable"`, 4, 24)
}

// --- D4: structured-control-flow liveness -----------------------------------------

func TestResourceLiveness(t *testing.T) {
	// The three sanctioned transfers (golden ch13-transfers-green).
	wantOK(t, m6bPrelude+"\n"+m6bTake+"\nfn give() -> FileHandle {\n    let f = openFile()\n    return f\n}\n\nfn passOn() {\n    let f = openFile()\n    take(f)\n}\n")
	// Both arms transfer: the if-join is legal (ch13:101).
	wantOK(t, m6bPrelude+"\n"+m6bTake+"\nfn use(c: Bool) {\n    let f = openFile()\n    if c {\n        take(f)\n    } else {\n        take(f)\n    }\n}\n")
	// A binding created and transferred inside the loop body is per-iteration.
	wantOK(t, m6bPrelude+"\n"+m6bTake+"\nfn use(c: Bool) {\n    while c {\n        let f = openFile()\n        take(f)\n    }\n}\n")
	// break after a transfer on the penetrating path (golden ch13-break-green).
	wantOK(t, m6bPrelude+"\n"+m6bTake+"\nfn use(c: Bool) {\n    while c {\n        let f = openFile()\n        if c {\n            take(f)\n            break\n        }\n        take(f)\n    }\n}\n")

	// E1104 drop: reaching scope end with no transfer on some path.
	wantDiag(t, m6bPrelude+"\nfn use() {\n    let f = openFile()\n}\n",
		"E1104", `the binding "f" reaches the end of its scope on a path with no transfer`, 13, 9)
	// E1104 use-after-transfer: one handle, killed by the transfer.
	wantDiag(t, m6bPrelude+"\nfn use() {\n    let f = openFile()\n    take(f)\n    let n = f.fd\n}\n\n"+m6bTake,
		"E1104", `"f" is used after its transfer`, 15, 13)
	// E1104 top-level: no channel exists outside function bodies.
	wantDiag(t, m6bPrelude+"\nlet f = openFile()\n",
		"E1104", `"f" is bound at the module top level`, 12, 1)
	// E1104 release direct call: one trigger, the scope-exit machinery.
	wantDiag(t, m6bPrelude+"\nfn use() {\n    scope resource(f = openFile()) {\n        f.release()\n    }\n}\n",
		"E1104", `"f.release()" calls release directly`, 14, 9)
	// E1104 break path: the penetrating path ends the block with no transfer.
	wantDiag(t, m6bPrelude+"\n"+m6bTake+"\nfn use(c: Bool) {\n    while c {\n        let f = openFile()\n        if c {\n            break\n        }\n        take(f)\n    }\n}\n",
		"E1104", `the binding "f" reaches the end of its scope on a path with no transfer`, 20, 13)
	// E1104 one-arm if: the empty arm reaches the join with the binding live.
	wantDiag(t, m6bPrelude+"\n"+m6bTake+"\nfn use(c: Bool) {\n    let f = openFile()\n    if c {\n        take(f)\n    }\n}\n",
		"E1104", `the binding "f" reaches the end of its scope on a path with no transfer`, 19, 9)
}

// --- D5: the alias ban ------------------------------------------------------------

func TestResourceAlias(t *testing.T) {
	// let alias anchors the second binding's name.
	wantDiag(t, m6bPrelude+"\nfn use() {\n    let f = openFile()\n    let g = f\n}\n",
		"E1105", `"g" is a second handle to the resource of "f"`, 14, 9)
	// var declaration: a resource binding is let-shaped.
	wantDiag(t, m6bPrelude+"\nfn use() {\n    var f = openFile()\n}\n",
		"E1105", `"f" is declared var and rebindable`, 13, 5)
	// Assignment anchors the `=`.
	wantDiag(t, m6bPrelude+"\nfn use() {\n    scope resource(f = openFile()) {\n        var g = 1\n        g = f\n    }\n}\n",
		"E1105", "the assignment moves a resource binding on one side", 15, 11)
}

// --- D6: the composite-position ban -----------------------------------------------

func TestResourceComposite(t *testing.T) {
	wantDiag(t, m6bPrelude+"\nfn use(t: (FileHandle, Int64)) {\n    return\n}\n",
		"E1106", `"FileHandle" appears as a tuple element`, 12, 12)
	wantDiag(t, m6bPrelude+"\nrecord Holder { f: FileHandle }\n",
		"E1106", `"FileHandle" appears as a gc record field`, 12, 20)
	wantDiag(t, m6bPrelude+"\npub type Bag = Has(FileHandle)\n",
		"E1106", `"FileHandle" appears as a sum payload`, 12, 20)
	wantDiag(t, m6bPrelude+"\nnewtype Wrapped(FileHandle)\n",
		"E1106", `"FileHandle" appears as a newtype's underlying type`, 12, 17)
	wantDiag(t, m6bPrelude+"\nfn id<T>(x: T) -> T {\n    return x\n}\n\nfn use() {\n    let f = openFile()\n    let g = id<FileHandle>(f)\n}\n",
		"E1106", `"FileHandle" appears as a generic argument at the instantiation`, 18, 16)
	// The box route, both faces: a type slot and a construction.
	wantDiag(t, m6bPrelude+"\nfn use(d: Dyn<Releasable>) {\n    return\n}\n",
		"E1106", `"Dyn<Releasable>" is the box route in a type position`, 12, 11)
	wantDiag(t, m6bPrelude+"\nfn use() {\n    let f = openFile()\n    let d = Dyn<Releasable>(f)\n}\n",
		"E1106", "the construction Dyn<Releasable>(r) with a resource-typed argument", 14, 13)
	// The byval field position is E0601's own rejection (ch13:141),
	// with the registry text verbatim (golden lock).
	wantDiag(t, m6bPrelude+"\nbyval record Holder {\n    f: FileHandle\n}\n",
		"E0601", `the field "f" is "FileHandle", a byres record; a copy is only honest`, 13, 8)
}

// --- D7: the `?` propagation operator ----------------------------------------------

func TestPropOperator(t *testing.T) {
	const g = "pub type AppError = Failed(String)\n\nfn g() -> Result<Int64, AppError> {\n    return Ok(1)\n}\n\n"
	// Ok unwraps; chains resolve the member on the payload; a short
	// closure body fixes the closure's value type (golden ch14 greens).
	wantOK(t, g+"pub fn main() -> Result<(), AppError> {\n    let a = g()?\n    let b = g()?\n    return Ok(())\n}\n")
	wantOK(t, "pub type AppError = Failed(String)\n\nrecord User {\n    name: String\n}\n\nfn g() -> Result<User, AppError> {\n    return Ok(User { name: \"n\" })\n}\n\npub fn main() -> Result<(), AppError> {\n    let n = g()?.name\n    return Ok(())\n}\n")
	wantOK(t, "pub type AppError = Failed(String)\n\nfn make(s: String) -> Result<Int64, AppError> {\n    return Ok(1)\n}\n\nfn apply(p: fn(String) -> Result<Int64, AppError>) -> Result<Int64, AppError> {\n    return p(\"x\")\n}\n\npub fn main() -> Result<(), AppError> {\n    let v = apply(|s: String| make(s)?)?\n    return Ok(())\n}\n")

	// E1201: Option does not propagate; neither do base types.
	wantDiag(t, "pub type AppError = Failed(String)\n\nfn g() -> Option<Int64> {\n    return None\n}\n\npub fn main() -> Result<(), AppError> {\n    let v = g()?\n    return Ok(())\n}\n",
		"E1201", `the operand is "Option<Int64>" and Option does not propagate`, 8, 16)
	wantDiag(t, "pub type AppError = Failed(String)\n\nfn g() -> Int64 {\n    return 1\n}\n\npub fn main() -> Result<(), AppError> {\n    let v = g()?\n    return Ok(())\n}\n",
		"E1201", `the operand is "Int64"`, 8, 16)
	// E1202: the context enumeration — non-Result fn, top level, defer body.
	wantDiag(t, g+"fn plain() {\n    let v = g()?\n}\n",
		"E1202", `the innermost enclosing function "plain" returns "()"`, 8, 16)
	wantDiag(t, g+"let v = g()?\n",
		"E1202", "the propagation stands at the module top level", 7, 12)
	wantDiag(t, g+"pub fn main() -> Result<(), AppError> {\n    defer {\n        let v = g()?\n    }\n    return Ok(())\n}\n",
		"E1202", "the propagation stands in a defer body", 9, 20)
	// E1203: no implicit wrapping or lifting between named error types.
	wantDiag(t, "pub type ErrA = Failed(String)\npub type ErrB = Other(String)\n\nfn g() -> Result<Int64, ErrA> {\n    return Err(Failed(\"x\"))\n}\n\npub fn main() -> Result<Int64, ErrB> {\n    let v = g()?\n    return Ok(v)\n}\n",
		"E1203", `"ErrA" is not the declared error type "ErrB"`, 9, 16)
	// A short closure's `?`s must agree on one error type among themselves
	// (the second `?` anchors).
	wantDiag(t, "pub type ErrA = Fa(String)\npub type ErrB = Fb(String)\n\nfn ga() -> Result<Int64, ErrA> {\n    return Ok(1)\n}\n\nfn gb() -> Result<Int64, ErrB> {\n    return Ok(2)\n}\n\nfn add(a: Int64, b: Int64) -> Int64 {\n    return a + b\n}\n\nfn take(p: fn(Int64) -> Result<Int64, ErrA>) -> Result<Int64, ErrA> {\n    return p(1)\n}\n\npub fn main() -> Result<(), ErrA> {\n    let v = take(|x: Int64| add(ga()?, gb()?))\n    return Ok(())\n}\n",
		"E1203", `"ErrB" is not the declared error type "ErrA"`, 21, 44)
	// The spec's closure scenario (ch14): a full closure declaring Result
	// is its own propagation context — a bare `?` tail is legal and hands
	// its payload Ok-wrapped; a non-Result declared return names
	// "(closure)".
	wantOK(t, "pub type AppError = Failed(String)\n\nfn parse(s: String) -> Result<Int64, AppError> {\n    return Ok(1)\n}\n\nfn take(p: fn(String) -> Result<Int64, AppError>) -> Result<Int64, AppError> {\n    return p(\"x\")\n}\n\nfn work() -> Int64 {\n    let v = take(fn(s: String) -> Result<Int64, AppError> { parse(s)? })\n    return 0\n}\n")
	wantDiag(t, "pub type AppError = Failed(String)\n\nfn g() -> Result<Int64, AppError> {\n    return Ok(1)\n}\n\nfn work() -> Int64 {\n    let f = fn() -> Int64 {\n        return g()?\n    }\n    return 2\n}\n",
		"E1202", `the innermost enclosing function "(closure)" returns "Int64"`, 9, 19)
	// A short closure at the module top level is its own innermost fn: the
	// binding is legal and the closure's value wraps to Result (the spec's
	// short-closure scenario).
	wantOK(t, "pub type ParseError = Failed(String)\n\nfn parse(s: String) -> Result<Int64, ParseError> {\n    return Ok(1)\n}\n\nlet f = |s: String| parse(s)?\n")
	// The bare `?` tail in a fn: legal against the declared Ok slot, E0501
	// against any other payload (anchored at the operand's first token).
	wantOK(t, "pub type AppError = Failed(String)\n\nfn g() -> Result<Int64, AppError> {\n    return Ok(1)\n}\n\nfn h() -> Result<Int64, AppError> {\n    g()?\n}\n")
	wantDiag(t, "pub type AppError = Failed(String)\n\nfn g() -> Result<Int64, AppError> {\n    return Ok(1)\n}\n\nfn h() -> Result<String, AppError> {\n    g()?\n}\n",
		"E0501", `the final expression is Int64, the declared return is Result<String, AppError>`, 8, 5)
	// A return-position `?` carries its unwrapped value against the full
	// declared return — the Ok-wrap is the tail-value mechanic only.
	wantDiag(t, "pub type AppError = Failed(String)\n\nfn g() -> Result<Int64, AppError> {\n    return Ok(1)\n}\n\nfn h() -> Result<Int64, AppError> {\n    return g()?\n}\n",
		"E0501", `the return expression is Int64, the declared return is Result<Int64, AppError>`, 8, 12)
}

// --- D8: the panic family's real signatures ------------------------------------------

func TestPanicFamily(t *testing.T) {
	// All three names are prelude functions with real signatures; calls
	// take the ordinary checking path (golden ch14-panic-family-green).
	wantOK(t, "fn find(id: Int64) -> Int64 {\n    if id == 0 {\n        panic(\"no such id\")\n    } else {\n        id\n    }\n}\n\nfn parse(s: String) -> Int64 {\n    todo(\"parsing not written yet\")\n}\n\nfn check(n: Int64) {\n    assert(n > 0, \"n must be positive\")\n    return\n}\n")
	// Never satisfies any return slot; a panic tail closes a function whose
	// declared type is not Never (golden ch14-panic-toplet-green), and a
	// top-level initializer may call panic (chapter 15's init order).
	wantOK(t, "fn boot(p: Bool) -> Int64 {\n    if p {\n        return 1\n    }\n    panic(\"no config\")\n}\n\nlet x = boot(true)\n")
	wantOK(t, "fn die() -> Never {\n    panic(\"boom\")\n}\n")
	// The argument types are ordinary: a Bool message is E0501.
	wantDiag(t, "fn f() {\n    panic(true)\n    return\n}\n",
		"E0501", "the argument is Bool, the parameter is String", 2, 11)
}
