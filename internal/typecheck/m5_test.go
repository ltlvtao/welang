package typecheck

import (
	"testing"
)

// M5 (control-and-composites) checker tests: chapter 8 declarations and
// composites, chapter 3 control-flow typing, chapter 4 pattern typing and
// Maranget usefulness, chapter 12 fn values and closures. Message details
// mirror the T1 golden table — those JSON files are the CLI-face contract,
// these tests are the package-level mirror with positions.

// --- D6: declarations, construction, update, newtype, tuples ------------------

func TestRecordDeclarations(t *testing.T) {
	wantOK(t, "record User { name: String, age: Int64 }\n")
	wantOK(t, "byval record Point { x: Int64, y: Int64 }\n")
	// Since M6b a byres record owns its release: the fixture carries its
	// impl (chapter 13's module-level completeness).
	wantOK(t, "byres record Conn { host: String }\nimpl Releasable for Conn {\n    fn release(mut self) {\n        return\n    }\n}\n")
	wantOK(t, "record Empty { }\n")
	wantOK(t, "pub record P { x: Int64 }\n")
	// Category honesty: a copy is only honest when everything in it is
	// copyable by value (chapter 8).
	wantDiag(t, "record Inner { name: String }\n\nbyval record Outer { inner: Inner }\n",
		"E0601", `the field "inner" is "Inner", a gc record`, 3, 29)
	wantDiag(t, "record Inner { name: String }\n\nbyval type Shape = Circle(Inner)\n",
		"E0702", `the payload of "Circle" is "Inner", a gc record`, 3, 27)
}

func TestConstructionAndUpdate(t *testing.T) {
	wantOK(t, "record User { name: String, age: Int64 }\n\nfn older(u: User) -> User {\n    return User { age: u.age + 1 with &u }\n}\n")
	wantDiag(t, "pub type AppError = Failed(String)\n\nfn f() {\n    let x = AppError { code: 1 }\n}\n",
		"E0603", `the head "AppError" names a sum type`, 4, 13)
	wantDiag(t, "record User { name: String }\n\nrecord Admin { name: String }\n\nfn f(u: User) {\n    let v = Admin { name: \"b\" with &u }\n}\n",
		"E0603", `the update base is "User", the head names "Admin"`, 6, 37)
	wantDiag(t, "record User { name: String }\n\nfn f() {\n    let u = User { name: \"a\", email: \"b\" }\n}\n",
		"E0604", `"User" declares no field "email"`, 4, 31)
	wantDiag(t, "record User { name: String, age: Int64 }\n\nfn f() {\n    let u = User { name: \"a\" }\n}\n",
		"E0604", `the construction of "User" leaves out "age"`, 4, 13)
	wantDiag(t, "record User { name: String }\n\nfn f() {\n    let u = User { name: \"a\", name: \"b\" }\n}\n",
		"E0604", `"User" names the field "name" twice`, 4, 31)
	wantDiag(t, "byres record Conn { host: String }\n\nfn f(c: Conn) {\n    let d = Conn { host: \"x\" with &c }\n}\n",
		"E0606", `the head "Conn" names a byres record`, 4, 13)
	wantDiag(t, "record User { name: String, age: Int64 }\n\nfn f() {\n    let u = User { name: \"a\", age: true }\n}\n",
		"E0501", `the field "age" is Bool, the declared field is Int64`, 4, 31)
}

func TestNewtype(t *testing.T) {
	wantOK(t, "newtype UserId(Int64)\n\nfn raw(id: UserId) -> Int64 {\n    return id.value\n}\n")
	wantOK(t, "newtype UserId(Int64)\n\nfn f() {\n    let id = UserId(42)\n}\n")
	wantDiag(t, "newtype UserId(Int64)\n\nfn f() {\n    let id = UserId(42)\n    let n: Int64 = id\n}\n",
		"E0501", "the expression is UserId, the annotation is Int64", 5, 9)
	wantDiag(t, "newtype UserId(Int64)\n\nfn f() {\n    let id = UserId(42)\n    let raw = id.inner\n}\n",
		"E0816", `"UserId" has no member "inner"`, 5, 18)
}

func TestTuples(t *testing.T) {
	wantOK(t, "fn f() -> Int64 {\n    let pair = (1, \"a\")\n    let (n, s) = pair\n    return n\n}\n")
	wantOK(t, "fn f() {\n    let u = ()\n}\n")
	wantOK(t, "fn f(t: (Int64, (Int64, Int64))) -> Int64 {\n    let (a, (_, b)) = t\n    return b\n}\n")
	wantDiag(t, "fn f() {\n    let n: Int64 = (1, 2)\n}\n",
		"E0501", "mixed types", 2, 9)
}

// --- D10: member resolution ---------------------------------------------------

func TestMemberResolution(t *testing.T) {
	wantOK(t, "record User { name: String }\n\nfn label(u: User) -> String {\n    return u.name\n}\n")
	wantDiag(t, "record User { name: String }\n\nfn f(u: User) {\n    let n = u.email\n}\n",
		"E0816", `"User" has no member "email"`, 4, 15)
	// Base-type receivers keep the chapter-15 boundary (D10: unchanged).
	wantBnd(t, "fn f(s: String) -> Int64 {\n    let n = s.length\n    return 1\n}\n", bndStdModules)
}

// --- D7: control-flow typing ---------------------------------------------------

func TestControlFlowTyping(t *testing.T) {
	wantOK(t, "fn classify(n: Int64) -> String {\n    let label = if n > 0 { \"pos\" } else if n < 0 { \"neg\" } else { \"zero\" }\n    return label\n}\n")
	wantOK(t, "fn f(c: Bool) {\n    if c {\n        let u = ()\n    }\n}\n")
	wantOK(t, "fn f() -> Int64 {\n    var n = 0\n    while true {\n        n = n + 1\n        if n > 3 {\n            break\n        }\n        continue\n    }\n    return n\n}\n")
	wantOK(t, "fn f() {\n    loop {\n        break\n    }\n}\n")
	wantOK(t, "fn step() {\n}\n\nfn f() -> Int64 {\n    defer { step() }\n    return 1\n}\n")
	wantOK(t, "fn f() {\n    let x: () = if true { () } else { () }\n}\n")

	// Arm agreement: one if arm disagrees with the others and the context.
	wantDiag(t, "fn f(c: Bool) -> Int64 {\n    let x = if c { 1 } else { \"a\" }\n    return x\n}\n",
		"E0501", "mixed types", 2, 13)
	// Condition positions: if, while, and match guard (chapter 3/4).
	wantDiag(t, "fn f() -> Int64 {\n    let x = if 1 { 2 } else { 3 }\n    return x\n}\n",
		"E0503", "the if condition is Int64", 2, 16)
	wantDiag(t, "fn f() {\n    while 5 {\n        break\n    }\n}\n",
		"E0503", "the while condition is Int64", 2, 11)
	// Value drop: statement position and the final item of a control
	// form's body (the ternary walk modes of D7).
	wantDiag(t, "fn f(c: Bool) {\n    if c { 1 } else { 2 }\n}\n",
		"E0605", "the statement's if arms are Int64", 2, 5)
	wantDiag(t, "fn f(c: Bool) {\n    match c {\n        _ => 1\n    }\n}\n",
		"E0605", "the unbound match-arm bodies are Int64", 2, 5)
	wantDiag(t, "fn f(c: Bool) {\n    while c {\n        1 + 2\n    }\n}\n",
		"E0605", "the final item of a control form's body is Int64", 3, 9)
}

// --- D8: pattern typing and usefulness ----------------------------------------

func TestPatternTyping(t *testing.T) {
	wantOK(t, "pub type Light = Red | Green | Yellow\n\nfn code(l: Light) -> Int64 {\n    return match l {\n        Red => 1\n        Green => 2\n        Yellow => 3\n    }\n}\n")
	wantOK(t, "pub type Shape = Circle(Float64) | Square(Float64) | Point\n\nfn f(s: Shape) -> Float64 {\n    return match s {\n        Circle(w) | Square(w) => w\n        Point => 0.0\n    }\n}\n")
	wantOK(t, "fn f(n: Int64) -> Int64 {\n    return match n {\n        0 => n\n        _ => n + 1\n    }\n}\n")
	wantDiag(t, "pub type Shape = Circle(Float64) | Square(Float64)\n\nfn f(s: Shape) -> Float64 {\n    return match s {\n        Triangle(r) => r\n        _ => 0.0\n    }\n}\n",
		"E0303", `"Triangle" is not a variant of "Shape"`, 5, 9)
	wantDiag(t, "pub type Shape = Circle(Float64) | Square(Float64)\n\nfn f(n: Int64) -> Int64 {\n    return match n {\n        Circle(r) => 1\n        _ => 2\n    }\n}\n",
		"E0303", `the scrutinee is "Int64", which declares no variants`, 5, 9)
	wantDiag(t, "pub type Shape = Circle(Float64) | Square(Float64)\n\nfn f(s: Shape) -> Float64 {\n    return match s {\n        Circle(a, b) => a\n        _ => 0.0\n    }\n}\n",
		"E0304", `the pattern holds 2 sub-patterns, the variant "Circle" declares 1`, 5, 9)
	// Or-pattern branches binding one name at two types (chapter 4).
	wantDiag(t, "pub type Shape = Circle(Float64) | Square(Int64)\n\nfn f(s: Shape) -> Float64 {\n    return match s {\n        Circle(w) | Square(w) => w\n    }\n}\n",
		"E0501", `the branches of one or-pattern bind "w" as Float64 and Int64`, 5, 28)
	// Guard typing: a guard is a condition position.
	wantDiag(t, "pub type Shape = Circle(Float64) | Square(Float64)\n\nfn f(s: Shape) -> Float64 {\n    return match s {\n        Circle(r) if r => r\n        _ => 0.0\n    }\n}\n",
		"E0503", "the match guard is Float64", 5, 22)
}

func TestExhaustivenessMatrix(t *testing.T) {
	// Sum coverage: every variant, or a wildcard.
	wantDiag(t, "pub type Light = Red | Green | Yellow\n\nfn f(l: Light) -> Int64 {\n    return match l {\n        Red => 1\n        Green => 2\n    }\n}\n",
		"E0305", `the variant "Yellow" of "Light" is not covered`, 4, 12)
	wantDiag(t, "fn f(n: Int64) -> Int64 {\n    return match n {\n        1 => n\n        2 => n\n    }\n}\n",
		"E0305", `"Int64" does not enumerate its values and no wildcard arm stands`, 2, 12)
	// Product coverage over a tuple of sums: the adversarial matrix.
	wantOK(t, "pub type Opt = Hit(Int64) | Miss\n\nfn f(p: (Opt, Opt)) -> Int64 {\n    return match p {\n        (Hit(a), Hit(b)) => a + b\n        (Hit(a), Miss) => a\n        (Miss, Hit(b)) => b\n        (Miss, Miss) => 0\n    }\n}\n")
	// The false-exhaustiveness counterexample: two shapes covered, the
	// or-pattern reads as total but (Hit, Hit) — and (Miss, Miss) — stay
	// reachable. Usefulness, not arm counting, decides (D8). Both branches
	// bind the same name set (E0302's rule); the shapes stay uncovered.
	wantDiag(t, "pub type Opt = Hit(Int64) | Miss\n\nfn f(p: (Opt, Opt)) -> Int64 {\n    return match p {\n        (Hit(a), Miss) | (Miss, Hit(a)) => a\n    }\n}\n",
		"E0305", `the value shape "(Hit, Hit)" is not covered`, 4, 12)
	// Guards refine arms but never prove coverage: the unguarded set
	// alone must be exhaustive.
	wantDiag(t, "pub type Shape = Circle(Float64) | Square(Float64)\n\nfn f(s: Shape) -> Float64 {\n    return match s {\n        Circle(r) if r > 1.0 => r\n        Square(w) => w\n    }\n}\n",
		"E0306", `the unguarded arms alone do not cover "Circle" of "Shape"`, 4, 12)
	wantOK(t, "pub type Shape = Circle(Float64) | Square(Float64)\n\nfn f(s: Shape) -> Float64 {\n    return match s {\n        Circle(r) if r > 1.0 => r\n        Square(w) => w\n        Circle(r) => r\n    }\n}\n")
	// Static dead arms are rejected.
	wantDiag(t, "pub type Light = Red | Green | Yellow\n\nfn f(l: Light) -> Int64 {\n    return match l {\n        _ => 1\n        Red => 2\n    }\n}\n",
		"E0307", "already covered by earlier arms", 6, 9)
	wantDiag(t, "pub type Light = Red | Green | Yellow\n\nfn f(l: Light) -> Int64 {\n    return match l {\n        Red => 1\n        Red => 2\n        _ => 3\n    }\n}\n",
		"E0307", "already covered by earlier arms", 6, 9)
	wantDiag(t, "fn f(n: Int64) -> Int64 {\n    return match n {\n        1 => n\n        1 => n\n        _ => n\n    }\n}\n",
		"E0307", "already covered by earlier arms", 4, 9)
}

// --- D9: fn values, closures, captures -----------------------------------------

func TestFnValues(t *testing.T) {
	wantOK(t, "fn square(n: Int64) -> Int64 {\n    return n * n\n}\n\nfn apply(f: fn(Int64) -> Int64, v: Int64) -> Int64 {\n    return f(v)\n}\n\nfn f() -> Int64 {\n    let g = square\n    return apply(g, 2)\n}\n")
	wantDiag(t, "fn greet(n: String) -> Int64 {\n    return 1\n}\n\nfn apply(f: fn(Int64) -> Int64, v: Int64) -> Int64 {\n    return f(v)\n}\n\nfn f() -> Int64 {\n    return apply(greet, 1)\n}\n",
		"E0501", "the argument is fn(String) -> Int64, the parameter is fn(Int64) -> Int64", 10, 18)
}

func TestClosures(t *testing.T) {
	wantOK(t, "fn f() -> Int64 {\n    let add = fn(a: Int64, b: Int64) -> Int64 { return a + b }\n    let run = fn() { }\n    run()\n    return add(1, 2)\n}\n")
	wantOK(t, "fn f() -> Int64 {\n    let inc = |x: Int64| x + 1\n    return inc(1)\n}\n")
	wantOK(t, "fn f() -> Int64 {\n    let inc: fn(Int64) -> Int64 = |x| x + 1\n    return inc(1)\n}\n")
	wantOK(t, "fn apply(f: fn(Int64) -> Int64, v: Int64) -> Int64 {\n    return f(v)\n}\n\nfn f() -> Int64 {\n    return apply(|x| x * 2, 5)\n}\n")
	wantOK(t, "fn f() -> Int64 {\n    let pick = fn(c: Bool) -> Int64 { return if c { 1 } else { 2 } }\n    return pick(true)\n}\n")
	// A closure's body is a function body (chapter 12): its final expression
	// is the closure's return value, so an if or a match there is a value,
	// never a dropped statement — the discard rule binds the *statement*
	// positions, which a body tail is not.
	wantOK(t, "fn f() -> Int64 {\n    let pick = |a: Int64, b: Int64| if a > b { a } else { b }\n    return pick(1, 2)\n}\n")
	wantOK(t, "fn f() -> Int64 {\n    let pick: fn(Int64, Int64) -> Int64 = |a, b| { if a > b { a } else { b } }\n    return pick(1, 2)\n}\n")
	wantOK(t, "fn f() -> Int64 {\n    let pick = |n: Int64| match n {\n        0 => 1\n        _ => 2\n    }\n    return pick(0)\n}\n")
	wantOK(t, "fn apply(f: fn(Int64, Int64) -> Int64, a: Int64, b: Int64) -> Int64 {\n    return f(a, b)\n}\n\nfn f() -> Int64 {\n    return apply(|a, b| if a > b { a } else { b }, 1, 2)\n}\n")
	// Bare parameters need an expected function type.
	wantDiag(t, "fn f() {\n    let g = |x| x\n}\n",
		"E1001", "the parameters are bare and no explicit function type is expected", 2, 13)
}

func TestCaptureLedger(t *testing.T) {
	// Value captures are snapshots fixed at closure creation (green read,
	// frozen write), gc captures stay live (chapter 12).
	wantOK(t, "fn f() -> Int64 {\n    var n = 1\n    let read = |x: Int64| n + x\n    n = 2\n    return read(0)\n}\n")
	wantOK(t, "record Counter { n: Int64 }\n\nfn f() -> Int64 {\n    var c = Counter { n: 0 }\n    let bump = fn() -> Int64 {\n        c = Counter { n: c.n + 1 }\n        return c.n\n    }\n    let _ = bump()\n    return c.n\n}\n")
	wantOK(t, "fn f() -> Int64 {\n    var n = 1\n    let outer = |x: Int64| {\n        let inner = |y: Int64| n + y\n        inner(x)\n    }\n    return outer(2)\n}\n")
	wantDiag(t, "byres record Conn { host: String }\n\nfn f() {\n    let c = Conn { host: \"h\" }\n    let g = |x: Int64| c\n}\n",
		"E1002", `the closure reads "c", a byres binding`, 5, 24)
	wantDiag(t, "fn f() {\n    var n = 1\n    let g = |x: Int64| { n = x }\n    g(2)\n}\n",
		"E1003", `"n" is captured as a copy fixed at closure creation`, 3, 26)
}

// --- ch8 name shadowing --------------------------------------------------------

func TestShadowing(t *testing.T) {
	wantOK(t, "fn g(x: Int64) -> String {\n    return \"s\"\n}\n\nfn laterBinding() -> String {\n    let x = 1\n    let x = g(x)\n    return x\n}\n")
	wantOK(t, "fn paramShadow(n: Int64) -> String {\n    let n = \"s\"\n    return n\n}\n")
	wantOK(t, "fn nearestBinding() -> Int64 {\n    let x = 1\n    if true {\n        let x = \"s\"\n    }\n    return x\n}\n")
}
