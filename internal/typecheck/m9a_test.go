package typecheck

import (
	"testing"
)

// M9a concurrency-check checker-face unit tests (design D2–D8): loading,
// the builtin type surfaces, the Shareable marker, task-block checks and
// captures, the handle discipline, scope and select typing, and the
// constructor codes. Written test-first; the conformance goldens carry
// the straight-line faces, these carry the half-forms around them.

const m9aHead = "import std.concurrent as conc\n\npub type AppError = Failed(String)\n\n"

const m9aFile = "byres record FileHandle { fd: Int64 }\nimpl Releasable for FileHandle {\n    fn release(mut self) {\n        return\n    }\n}\n\nfn openFile() -> FileHandle {\n    return FileHandle { fd: 1 }\n}\n"

func TestStdConcurrentLoading(t *testing.T) {
	wantOK(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n")
	wantOK(t, "import std.concurrent\n\npub type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n")
	wantOK(t, "pub type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n")
	// Unknown std modules keep the E1302 std form.
	wantDiag(t, "import std.channel\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n", "E1302", `no standard-library module "std.channel" exists`, 1, 8)
	// Bare names stay unresolved without the import (spec: the qualified
	// spellings are the reachable ones).
	wantDiag(t, "pub type AppError = Failed(String)\n\npub fn main() -> Result<(), AppError> {\n    let m = Mutex(0)\n    return Ok(())\n}\n", "E1304", `"Mutex" is held by no scope`, 4, 13)
}

func TestConcMemberFaces(t *testing.T) {
	// The four state cells' shared surface, and RwLock's read.
	wantOK(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    let a = m.update(|v| v + 1)\n    let b = m.get()\n    m.set(5)\n    let r = conc.RwLock(\"s\")\n    let u = r.read(|s| s)\n    let at = conc.Atomic(true)\n    at.set(false)\n    let n = conc.Atomic(3).get()\n    return Ok(())\n}\n")
	// Wrong argument typing rides the existing E0501.
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    m.set(\"x\")\n    return Ok(())\n}\n", "E0501", "", 7, 11)
	// Members outside the closed sets do not exist.
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    m.unlock()\n    return Ok(())\n}\n", "E0816", "", 7, 7)
	// Send-only views lose receive, receive-only lose send.
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let ch: conc.Channel<Int64> = conc.channel(4)\n    let so = ch.toSendOnly()\n    let got = so.receive()\n    return Ok(())\n}\n", "E0816", "", 8, 18)
}

func TestShareableMarker(t *testing.T) {
	// The bound resolves prelude-visibly and admits the closed set.
	wantOK(t, "pub type AppError = Failed(String)\n\npub fn pass<T>(x: T) -> T where T: Shareable {\n    return x\n}\n\npub fn main() -> Result<(), AppError> {\n    let a = pass(5)\n    let b = pass(true)\n    return Ok(())\n}\n")
	wantOK(t, m9aHead+"pub fn pass<T>(x: T) -> T where T: Shareable {\n    return x\n}\n\npub fn main() -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    let c = pass(m)\n    return Ok(())\n}\n")
	// Value records of Shareable fields are Shareable; a List field is not.
	wantOK(t, "pub type AppError = Failed(String)\n\nbyval record Pt { x: Int64, y: Int64 }\n\npub fn pass<T>(v: T) -> T where T: Shareable {\n    return v\n}\n\npub fn main() -> Result<(), AppError> {\n    let p = pass(Pt { x: 1, y: 2 })\n    return Ok(())\n}\n")
	wantDiag(t, "pub type AppError = Failed(String)\n\npub fn pass<T>(v: T) -> T where T: Shareable {\n    return v\n}\n\npub fn main() -> Result<(), AppError> {\n    let xs = [1, 2]\n    let ys = pass(xs)\n    return Ok(())\n}\n", "E0830", `"List<Int64>" implements no "Shareable"`, 9, 14)
	// A closure is not Shareable (spec scenario).
	wantDiag(t, "pub type AppError = Failed(String)\n\npub fn pass<T>(v: T) -> T where T: Shareable {\n    return v\n}\n\npub fn main() -> Result<(), AppError> {\n    let f = |n: Int64| n\n    let g = pass(f)\n    return Ok(())\n}\n", "E0830", `implements no "Shareable"`, 9, 13)
	// Manual impls are E1606.
	wantDiag(t, "pub type AppError = Failed(String)\n\nbyval record Rec { n: Int64 }\n\nimpl Shareable for Rec { }\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n", "E1606", "Shareable cannot be manually implemented", 5, 1)
	// Non-interface bounds that are not Shareable keep E0829.
	wantDiag(t, "pub type AppError = Failed(String)\n\nbyval record Rec { n: Int64 }\n\npub fn pass<T>(v: T) -> T where T: Rec {\n    return v\n}\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n", "E0829", "where bound is not a declared interface", 5, 36)
}

func TestTaskBlockFaces(t *testing.T) {
	// E1618: no enclosing scope block.
	wantDiag(t, m9aHead+"fn top() effect net {\n    let t = task effect net { 1 }\n    let _ = t.await()\n    return\n}\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n", "E1618", "task block outside any scope block", 6, 13)
	// E1401: the body's calls answer the task's own declared set.
	wantDiag(t, m9aHead+"fn fetch() effect net {\n    return\n}\n\npub fn main() -> Result<(), AppError> {\n    scope {\n        let t = task effect io { fetch() }\n        let _ = t.await()\n    }\n    return Ok(())\n}\n", "E1401", `"fetch" performs effect "net"`, 11, 34)
	// E1106: a resource-typed body value.
	wantDiag(t, m9aFile+m9aHead+"fn go() effect net {\n    scope {\n        let t = task effect net { openFile() }\n        let _ = t.await()\n    }\n    return\n}\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n", "E1106", `"FileHandle" appears as a task block's value`, 17, 35)
	// currentCancelSignal: inside a task body it types; outside is E1608.
	wantOK(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    scope {\n        let t = task effect net {\n            let stop = currentCancelSignal()\n            let flag = stop.isCancelled()\n            let _ = flag\n        }\n        let _ = t.await()\n    }\n    return Ok(())\n}\n")
	wantDiag(t, m9aHead+"fn plain() {\n    let s = currentCancelSignal()\n}\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n", "E1608", "currentCancelSignal called outside a task block", 6, 13)
	// A closure lexically inside the task body still counts as inside
	// (the full closure form — a zero-parameter short closure does not
	// exist, chapter 12's rule).
	wantOK(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    scope {\n        let t = task effect net {\n            let probe = fn() -> conc.CancelSignal {\n                return currentCancelSignal()\n            }\n            let _ = probe\n        }\n        let _ = t.await()\n    }\n    return Ok(())\n}\n")
	// Creating a task contributes nothing to the enclosing extent: main
	// stays pure while the task declares net.
	wantOK(t, m9aHead+"fn fetch() effect net {\n    return\n}\n\npub fn main() -> Result<(), AppError> {\n    scope {\n        let t = task effect net { fetch() }\n        let _ = t.await()\n    }\n    return Ok(())\n}\n")
}

func TestTaskCaptureDiscipline(t *testing.T) {
	// Judgment order: var fires before anything else.
	wantDiag(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    var total = 0\n    scope {\n        let t = task effect net { total }\n        let _ = t.await()\n    }\n    return Ok(())\n}\n", "E1603", "task block captures a var binding", 8, 35)
	// Synchronized gc captures are legal.
	wantOK(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    scope {\n        let t = task effect net { m.get() }\n        let _ = t.await()\n    }\n    return Ok(())\n}\n")
	// A gc record is not synchronized.
	wantDiag(t, m9aHead+"record Node { n: Int64 }\n\npub fn main() effect net -> Result<(), AppError> {\n    let node = Node { n: 1 }\n    scope {\n        let t = task effect net { node }\n        let _ = t.await()\n    }\n    return Ok(())\n}\n", "E1602", "task block captures unsynchronized gc state", 10, 35)
	// Resources with only release (all reads) capture legally — the body
	// reads a field, its value stays a base type (a resource T would be
	// E1106's, the task-value rule, not the capture rule's face).
	wantOK(t, m9aFile+m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope resource(f = openFile()) {\n        scope {\n            let t = task effect net { f.fd }\n            let _ = t.await()\n        }\n    }\n    return Ok(())\n}\n")
	// Generic parameters need the Shareable bound.
	wantDiag(t, m9aHead+"pub fn go<T>(v: T) effect net {\n    scope {\n        let t = task effect net { v }\n        let _ = t.await()\n    }\n    return\n}\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n", "E1605", "task block captures a generic-parameter binding without a Shareable bound", 7, 35)
	wantOK(t, m9aHead+"pub fn go<T>(v: T) effect net where T: Shareable {\n    scope {\n        let t = task effect net { v }\n        let _ = t.await()\n    }\n    return\n}\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n")
	// Bindings local to the task body are not captures.
	wantOK(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net {\n            let xs = [1, 2]\n            let n = xs.get(0)\n            let _ = n\n        }\n        let _ = t.await()\n    }\n    return Ok(())\n}\n")
}

func TestHandleDiscipline(t *testing.T) {
	// Un-awaited on the fall-through path.
	wantDiag(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net { 1 }\n    }\n    return Ok(())\n}\n", "E1607", "task handle reaches scope exit un-awaited", 7, 17)
	// A bare task statement is the discarded-handle extreme form.
	wantDiag(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope {\n        task effect net { 1 }\n    }\n    return Ok(())\n}\n", "E1607", "task handle reaches scope exit un-awaited", 7, 9)
	// A second fire after an await.
	wantDiag(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net { 1 }\n        let _ = t.await()\n        t.cancel()\n    }\n    return Ok(())\n}\n", "E1607", "task handle reaches scope exit un-awaited", 7, 17)
	// Early exits discharge: break takes the penetrating path out.
	wantOK(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    while true {\n        scope {\n            let t = task effect net { 1 }\n            let _ = t.await()\n            break\n        }\n    }\n    return Ok(())\n}\n")
	wantOK(t, m9aHead+"pub fn main(c: Bool) effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net { 1 }\n        if c {\n            let _ = t.await()\n        } else {\n            t.cancel()\n        }\n    }\n    return Ok(())\n}\n")
	// One arm awaiting, the other falling through: the join path misses.
	wantDiag(t, m9aHead+"pub fn main(c: Bool) effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net { 1 }\n        if c {\n            let _ = t.await()\n        }\n    }\n    return Ok(())\n}\n", "E1607", "task handle reaches scope exit un-awaited", 7, 17)
	// Created and consumed inside one loop iteration.
	wantOK(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    for i in 0..3 {\n        scope {\n            let t = task effect net { i }\n            let _ = t.await()\n        }\n    }\n    return Ok(())\n}\n")
	// Nested scopes analyze their own handles.
	wantOK(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net { 1 }\n        let _ = t.await()\n        scope {\n            let u = task effect net { 2 }\n            let _ = u.await()\n        }\n    }\n    return Ok(())\n}\n")
	wantDiag(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net { 1 }\n        let _ = t.await()\n        scope {\n            let u = task effect net { 2 }\n            let _ = u.await()\n        }\n        let w = task effect net { 3 }\n    }\n    return Ok(())\n}\n", "E1607", "task handle reaches scope exit un-awaited", 13, 17)
}

func TestScopeSelectTyping(t *testing.T) {
	// The timeout clause types Int64 under the existing E0501.
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let r = scope timeout(true) { 5 }\n    let _ = r\n    return Ok(())\n}\n", "E0501", "", 6, 27)
	// Wait sources: the four-call closed set and the arm agreement.
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let xs = [1, 2]\n    let a: conc.Channel<Int64> = conc.channel(4)\n    let v = select {\n        case n = xs.get(0) => n\n        case y = a.receive() => y\n    }\n    let _ = v\n    return Ok(())\n}\n", "E1609", "select source is not a wait source", 9, 18)
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let a: conc.Channel<Int64> = conc.channel(4)\n    let b: conc.Channel<String> = conc.channel(4)\n    let v = select {\n        case x = a.receive() => x\n        case y = b.receive() => y\n    }\n    let _ = v\n    return Ok(())\n}\n", "E1610", "select arms disagree in type", 10, 9)
	// await as a select source is a fire on the handle.
	wantOK(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net { 5 }\n        let ch: conc.Channel<Int64> = conc.channel(4)\n        let v = select {\n            case r = t.await() => 0\n            case x = ch.receive() => 1\n        }\n        let _ = v\n    }\n    return Ok(())\n}\n")
	wantDiag(t, m9aHead+"pub fn main() effect net -> Result<(), AppError> {\n    scope {\n        let t = task effect net { 5 }\n        let ch: conc.Channel<Int64> = conc.channel(4)\n        let v = select {\n            case r = t.await() => 0\n            case x = ch.receive() => 1\n        }\n        let _ = v\n        let w = t.await()\n        let _ = w\n    }\n    return Ok(())\n}\n", "E1607", "task handle reaches scope exit un-awaited", 7, 17)
}

func TestConcConstructors(t *testing.T) {
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let ready = conc.Atomic(\"name\")\n    let _ = ready\n    return Ok(())\n}\n", "E1614", "Atomic type argument is not an atomic base type", 6, 17)
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let r = conc.AtomicRef(3)\n    let _ = r\n    return Ok(())\n}\n", "E1615", "AtomicRef type argument is not a gc record type", 6, 13)
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let none = conc.Semaphore(0)\n    let _ = none\n    return Ok(())\n}\n", "E1616", "Semaphore count is not a positive integer", 6, 31)
	// The negative constant folds too; a positive one passes.
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let none = conc.Semaphore(-1)\n    let _ = none\n    return Ok(())\n}\n", "E1616", "Semaphore count is not a positive integer", 6, 31)
	wantOK(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let gate = conc.Semaphore(10)\n    gate.acquire()\n    gate.release()\n    return Ok(())\n}\n")
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let bare = conc.channel(4)\n    let _ = bare\n    return Ok(())\n}\n", "E1617", "channel construction without an expected type", 6, 16)
	// The expectation may come from a parameter position too.
	wantOK(t, m9aHead+"fn make() -> conc.Channel<Int64> {\n    return conc.channel(4)\n}\n\npub fn main() -> Result<(), AppError> {\n    return Ok(())\n}\n")
	// E1613: same-binding nested access; distinct bindings are free.
	wantDiag(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let a = conc.Mutex(0)\n    let x = a.update(|v| a.get() + 1)\n    let _ = x\n    return Ok(())\n}\n", "E1613", "nested access to one shared value within its own callback", 7, 26)
	wantOK(t, m9aHead+"pub fn main() -> Result<(), AppError> {\n    let a = conc.Mutex(0)\n    let b = conc.Mutex(0)\n    let x = a.update(|v| v + b.get())\n    let y = b.update(|v| v + a.get())\n    let _ = x\n    let _ = y\n    return Ok(())\n}\n")
}
