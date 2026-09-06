package typecheck

import (
	"testing"
)

// M7 (effects) checker tests: chapter 16's four check families and the
// closure inference (design D3–D8). Negative sources mirror the T1
// golden table verbatim so codes, message fragments, and positions stay
// identical to the CLI-face contract. Cross-module qualified tags are
// loader-face (design D9): their contract is pinned by the T1
// project-mode goldens and the cli loader tests; the single-file faces
// here cover the bare-tag resolution and both E1304 forms.

// --- D3: segment tags resolve against the module's effect declarations -----------

func TestEffectTagResolution(t *testing.T) {
	// A declared effect's segment on a green fn.
	wantOK(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nfn work() effect db {\n    save(\"x\")\n    return\n}\n")
	// Multiple tags resolve alike (net is built-in; db is declared).
	wantOK(t, "effect db\n\nfn work() effect db net {\n    return\n}\n")
	// A built-in tag needs no declaration.
	wantOK(t, "fn work() effect io {\n    return\n}\n")
	// E1304: a bare tag nothing declares, anchored at the tag.
	wantDiag(t, "fn work() effect db {\n    return\n}\n",
		"E1304", `no effect named "db" is declared in this module`, 1, 18)
	// E1304: a qualified tag with no import behind the first segment.
	wantDiag(t, "fn work() effect b.db {\n    return\n}\n",
		"E1304", `no import introduces "b", so the effect tag "b.db" resolves nowhere`, 1, 18)
	// An unresolved tag inside a fn type annotation is the same check.
	wantDiag(t, "let f: fn(String) db -> () = |s: String| { return }\n",
		"E1304", `no effect named "db" is declared in this module`, 1, 19)
}

// --- D4: the call-site subset (E1401) ---------------------------------------------

func TestCallSiteEffect(t *testing.T) {
	// Direct call: the callee's set must be a subset of the caller's.
	wantDiag(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nfn work() {\n    save(\"x\")\n    return\n}\n",
		"E1401", `"save" performs effect "db" which "work" does not declare`, 8, 5)
	// Method call through a concrete record receiver: the impl method's set.
	wantDiag(t, "record FileClient { fd: Int64 }\n\ninterface Client {\n    fn send(self, s: String) effect net\n}\n\nimpl Client for FileClient {\n    fn send(self, s: String) effect net {\n        return\n    }\n}\n\nfn work(c: FileClient) {\n    c.send(\"x\")\n    return\n}\n",
		"E1401", `"send" performs effect "net" which "work" does not declare`, 14, 5)
	// Method call through a generic bound: the interface method's set.
	wantDiag(t, "interface Client {\n    fn send(self, s: String) effect net\n}\n\nfn work<T>(c: T) where T: Client {\n    c.send(\"x\")\n    return\n}\n",
		"E1401", `"send" performs effect "net" which "work" does not declare`, 6, 5)
	// Call through a fn-typed parameter: the annotation's set.
	wantDiag(t, "effect db\n\nfn run(f: fn(String) db -> ()) {\n    f(\"x\")\n    return\n}\n",
		"E1401", `"f" performs effect "db" which "run" does not declare`, 4, 5)
	// A defer body's calls count toward the enclosing function (ch3).
	wantDiag(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nfn work() {\n    defer { save(\"x\") }\n    return\n}\n",
		"E1401", `"save" performs effect "db" which "work" does not declare`, 8, 13)
	// The message names only the missing tags (the difference set).
	wantDiag(t, "effect db\n\nfn save(s: String) effect db net {\n    return\n}\n\nfn work() effect db {\n    save(\"x\")\n    return\n}\n",
		"E1401", `"save" performs effect "net" which "work" does not declare`, 8, 5)
}

// --- D5: the agreement subset woven into the E0501 sites (E1402) -------------------

func TestEffectAgreement(t *testing.T) {
	// A pure value in an effect-expecting slot is legal (the Q3 flip).
	wantOK(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nlet f: fn(String) db -> () = |s: String| { return }\n")
	// An inferred effectful value in a pure slot is E1402 at the binding.
	wantDiag(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nlet f: fn(String) -> () = |s: String| { save(s) }\n",
		"E1402", `the value performs effect "db" and the expected type's effect segment does not include it`, 7, 5)
	// A superset of the slot is refused; the message names the extra tag.
	wantDiag(t, "effect db\n\nfn save(s: String) effect db { return }\nfn send(s: String) effect net { return }\n\nlet f: fn(String) db -> () = |s: String| {\n    save(s)\n    send(s)\n}\n",
		"E1402", `the value performs effect "net" and the expected type's effect segment does not include it`, 6, 5)
	// A structural mismatch keeps E0501's code (the woven split).
	wantDiag(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nlet f: fn(Int64) db -> () = |s: String| { save(s) }\n",
		"E0501", `the expression is fn(String) db -> (), the annotation is fn(Int64) db -> ()`, 7, 5)
	// Nested fn types compare tags exactly (no variance): the inner
	// mismatch is E0501, not a subset question.
	wantDiag(t, "effect db\n\nfn take(g: fn(String) db -> ()) effect db {\n    g(\"x\")\n    return\n}\n\nlet f: fn(fn(String) net -> ()) db -> () = take\n",
		"E0501", `the expression is fn(fn(String) db -> ()) db -> (), the annotation is fn(fn(String) net -> ()) db -> ()`, 8, 5)
	// The argument position carries the same woven check.
	wantDiag(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nfn pure(f: fn(String) -> ()) {\n    return\n}\n\nfn work() {\n    pure(|s: String| { save(s) })\n    return\n}\n",
		"E1402", `the value performs effect "db" and the expected type's effect segment does not include it`, 12, 10)
}

// --- D6: closure inference ---------------------------------------------------------

func TestClosureInference(t *testing.T) {
	// Constructing an effectful closure inside a pure fn is legal —
	// construction is not performance.
	wantOK(t, "effect db\n\nfn save(s: String) effect db { return }\n\nfn work() {\n    let g = |s: String| { save(s) }\n    return\n}\n")
	// A nested closure's body does not join the outer closure's set.
	wantOK(t, "effect db\n\nfn save(s: String) effect db { return }\n\nlet outer: fn(String) -> () = |s: String| {\n    let inner = |t: String| { save(t) }\n    return\n}\n")
	// Calling the inferred closure checks its set like a declared one.
	wantOK(t, "effect db\n\nfn save(s: String) effect db { return }\n\nfn work() effect db {\n    let g = |s: String| { save(s) }\n    g(\"x\")\n    return\n}\n")
	// A short closure infers through the slot's parameter type.
	wantOK(t, "effect db\n\nfn save(s: String) effect db { return }\n\nfn work() effect db {\n    let f: fn(String) db -> () = |s| save(s)\n    return\n}\n")
	// The inferred set rides the argument position.
	wantOK(t, "effect db\n\nfn save(s: String) effect db { return }\n\nfn apply(f: fn(String) db -> ()) effect db {\n    f(\"x\")\n    return\n}\n\nfn work() effect db {\n    apply(|s: String| { save(s) })\n    return\n}\n")
}

// --- D7: interface/impl exact equality (E1404) --------------------------------------

func TestImplEffectMatch(t *testing.T) {
	// An extra tag on the impl side.
	wantDiag(t, "effect db\n\ninterface Store {\n    fn put(self, s: String)\n}\n\nrecord Rec { fd: Int64 }\n\nimpl Store for Rec {\n    fn put(self, s: String) effect db {\n        return\n    }\n}\n",
		"E1404", `the impl method declares "db" and the interface method declares no effect segment`, 10, 8)
	// A missing tag on the impl side.
	wantDiag(t, "effect db\n\ninterface Store {\n    fn put(self, s: String) effect db\n}\n\nrecord Rec { fd: Int64 }\n\nimpl Store for Rec {\n    fn put(self, s: String) {\n        return\n    }\n}\n",
		"E1404", `the impl method declares no effect segment and the interface method declares "db"`, 10, 8)
	// Exact match on both sides is green, call included.
	wantOK(t, "effect db\n\ninterface Store {\n    fn put(self, s: String) effect db\n}\n\nrecord Rec { fd: Int64 }\n\nimpl Store for Rec {\n    fn put(self, s: String) effect db {\n        return\n    }\n}\n\nfn work(r: Rec) effect db {\n    r.put(\"x\")\n    return\n}\n")
	// A default body's calls count toward the interface method's own set;
	// the override copies the segment exactly.
	wantOK(t, "effect db\n\ninterface Store {\n    fn put(self, s: String) effect db\n    fn putTwice(self, s: String) effect db {\n        self.put(s)\n        self.put(s)\n    }\n}\n\nrecord Rec { fd: Int64 }\n\nimpl Store for Rec {\n    fn put(self, s: String) effect db {\n        return\n    }\n    fn putTwice(self, s: String) effect db {\n        return\n    }\n}\n")
}

// --- D8: top-level initializer purity (E1405) ----------------------------------------

func TestTopLetEffect(t *testing.T) {
	wantDiag(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nlet base = save(\"x\")\n",
		"E1405", `the initializer of "base" calls "save", which performs effect "db"`, 7, 12)
	// The message names every tag the callee performs.
	wantDiag(t, "effect db\n\nfn save(s: String) effect db net {\n    return\n}\n\nlet base = save(\"x\")\n",
		"E1405", `calls "save", which performs effect "db", "net"`, 7, 12)
	// Constructing a closure is exempt: construction is not performance.
	wantOK(t, "effect db\n\nfn save(s: String) effect db {\n    return\n}\n\nlet f = |s: String| { save(s) }\n")
}

// --- ch14: the panic family's zero set (regression locks) ----------------------------

func TestPanicZeroEffect(t *testing.T) {
	// A pure fn calling the panic family stays green: the family's set is
	// empty, so no subset can fail.
	wantOK(t, "fn die() -> Never { panic(\"boom\") }\nfn work() {\n    die()\n    return\n}\n")
	// The same at a top-level initializer: E1405 fires on effect sets, and
	// this call performs none.
	wantOK(t, "fn die() -> Never { panic(\"boom\") }\nlet boom = die()\n")
}
