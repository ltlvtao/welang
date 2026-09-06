package typecheck

import (
	"testing"
)

// M6a (generics-iterables-collections) checker tests: chapter 10 interface
// and impl validation, receiver discipline, the four-source member set, the
// Dyn face, single-direction generic determination, where bounds, the
// chapter 11 iteration protocol, and the chapter 17 collection face
// (design D3–D9). Negative sources mirror the T1 golden table verbatim so
// codes, message fragments, and positions stay identical to the CLI-face
// contract; those JSON files are the protocol, these tests are the
// package-level mirror.

// --- D3: interface and impl validation -----------------------------------------

func TestIfaceValidation(t *testing.T) {
	wantOK(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\n")
	// A default method body sits in the interface position: self reaches
	// the interface's own method set (D4 rule 1).
	wantOK(t, "interface Greeter {\n    fn greet(self) -> String\n    fn greetLoud(self) -> String {\n        self.greet()\n    }\n}\nrecord User { name: String }\nimpl Greeter for User {\n    fn greet(self) -> String {\n        \"hi\"\n    }\n}\n")
	// Judgment order (D3): head nominality, uniqueness, assoc bindings,
	// completeness, signature agreement.
	wantDiag(t, "interface HasIter {\n    type Iter\n}\nrecord Cfg { n: Int64 }\nimpl HasIter for Cfg {\n    fn it(self) -> Int64 {\n        0\n    }\n}\n",
		"E0805", `binds no "Iter"`, 5, 1)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n}\n",
		"E0807", `defines no "describe"`, 5, 1)
	wantDiag(t, "interface Describable {\n    fn describe(self, prefix: String) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self, prefix: Int64) -> String {\n        \"u\"\n    }\n}\n",
		"E0808", `the parameter "prefix" of "describe" is Int64 in the impl and String in "Describable"`, 6, 8)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self) -> Int64 {\n        0\n    }\n}\n",
		"E0808", `the return of "describe" is Int64 in the impl and String in "Describable"`, 6, 8)
	wantDiag(t, "interface Maker {\n    fn pick<T>(self) -> T\n}\nrecord M { n: Int64 }\nimpl Maker for M {\n    fn pick(self) -> T {\n        panic(\"unimplemented\")\n    }\n}\n",
		"E0808", `the method clause of "pick" is <T> in "Maker" and absent in the impl`, 6, 8)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"a\"\n    }\n}\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"b\"\n    }\n}\n",
		"E0809", `"Describable" is implemented for "User" a second time`, 10, 1)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord Wrap<T> { value: T }\nimpl<T> Describable for Wrap<T> {\n    fn describe(self) -> String {\n        \"w\"\n    }\n}\nimpl Describable for Wrap<Int64> {\n    fn describe(self) -> String {\n        \"c\"\n    }\n}\n",
		"E0809", `the impl for "Wrap<Int64>" overlaps the generic impl`, 10, 1)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nimpl Describable for Int64 {\n    fn describe(self) -> String {\n        \"i\"\n    }\n}\n",
		"E0811", `the head "Int64" is a base type`, 4, 1)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nimpl Describable for (Int64, String) {\n    fn describe(self) -> String {\n        \"t\"\n    }\n}\n",
		"E0811", "the head is a tuple", 4, 1)
}

// --- D3: receiver discipline -----------------------------------------------------

func TestReceiverDiscipline(t *testing.T) {
	// mut self on a gc record parses and checks; the in-place field write
	// is the language's only field-write form.
	wantOK(t, "record Counter { n: Int64 }\nimpl Counter {\n    fn bump(mut self) -> Int64 {\n        self.n = self.n + 1\n        self.n\n    }\n}\nfn f(c: Counter) -> Int64 {\n    return c.bump()\n}\n")
	wantDiag(t, "byval record Cell { n: Int64 }\nimpl Cell {\n    fn bump(mut self) -> Int64 {\n        0\n    }\n}\n",
		"E0812", `"bump" takes mut self on "Cell", a value-category record`, 3, 8)
	wantDiag(t, "byval record CountIter { n: Int64 }\nimpl Iterator<Int64> for CountIter {\n    fn next(mut self) -> Option<Int64> {\n        None\n    }\n}\n",
		"E0812", `"next" takes mut self on "CountIter"`, 3, 8)
	wantDiag(t, "record Cell { n: Int64 }\nimpl Cell {\n    fn read(self) -> Int64 {\n        self.n = 1\n        0\n    }\n}\n",
		"E0813", `the write to "self.n" sits in a plain "self" method`, 4, 9)
}

// --- D3/D4: member sets, collisions, method values -------------------------------

func TestMemberSets(t *testing.T) {
	wantOK(t, "record User { name: String }\nimpl User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\nfn f(u: User) -> String {\n    return u.describe()\n}\n")
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { describe: String }\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\n",
		"E0814", `"User" already has the field "describe"`, 6, 8)
	wantDiag(t, "interface Named1 {\n    fn name(self) -> String\n}\ninterface Named2 {\n    fn name(self) -> String\n}\nrecord User { n: Int64 }\nimpl Named1 for User {\n    fn name(self) -> String {\n        \"1\"\n    }\n}\nimpl Named2 for User {\n    fn name(self) -> String {\n        \"2\"\n    }\n}\n",
		"E0814", `"User" receives "name" from both "Named1" and "Named2"`, 13, 1)
	wantDiag(t, "interface Named {\n    fn name(self) -> String\n}\nrecord U2 { n: Int64 }\nimpl U2 {\n    fn name(self) -> String {\n        \"i\"\n    }\n}\nimpl Named for U2 {\n    fn name(self) -> String {\n        \"n\"\n    }\n}\n",
		"E0814", `"U2" already has the inherent method "name"`, 11, 8)
	wantDiag(t, "interface Greeter {\n    fn greet(self) -> String\n}\ninterface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Greeter for User {\n    fn greet(self) -> String {\n        \"hi\"\n    }\n}\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\nfn f(d: Dyn<Greeter>) -> String {\n    d.describe()\n}\n",
		"E0815", `"describe" is outside the method set of "Dyn<Greeter>"`, 19, 7)
	wantDiag(t, "interface Greeter {\n    fn greet(self) -> String {\n        self.missing()\n    }\n}\n",
		"E0815", "an interface position reaches its own interface's methods only", 3, 14)
	wantDiag(t, "fn f(s: String) {\n    let it = s.iterator()\n    let _ = it.forEach(|r| r)\n}\n",
		"E0816", "forEach does not exist: sequence effects belong to the for statement", 3, 16)
	wantDiag(t, "fn len<T>(x: T) -> Int64 {\n    let n = x.size()\n    0\n}\n",
		"E0817", `the parameter "T" carries no where bound`, 2, 15)
	// A method name resolves but fits no value production (chapter 12).
	wantDiag(t, "record User { name: String }\nimpl User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\nfn f(u: User) {\n    let d = u.describe\n}\n",
		"E0105", "resolves to a method; a method name fits no ratified value production", 8, 15)
}

// --- D4: the Dyn face --------------------------------------------------------------

func TestDynFace(t *testing.T) {
	wantOK(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\nfn f(u: User) -> String {\n    let d = Dyn<Describable>(u)\n    return d.describe()\n}\n")
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nfn f() {\n    let d = Dyn<Describable>(1)\n}\n",
		"E0818", `"Int64" implements no "Describable"`, 5, 30)
	wantDiag(t, "fn f(s: String) {\n    let d = Dyn<Iterable<Int64> >(s)\n}\n",
		"E0819", `"Iterable" declares the associated type "Iter"`, 2, 17)
	wantDiag(t, "fn f(d: Dyn<Iterable<Int64> >) -> Int64 {\n    0\n}\n",
		"E0819", `"Iterable" declares the associated type "Iter"`, 1, 13)
	wantDiag(t, "fn f(n: Int64) {\n    let d = Dyn<Int64>(n)\n}\n",
		"E0820", `"Int64" is not an interface`, 2, 17)
	wantDiag(t, "fn f(d: Dyn<Int64>) -> Int64 {\n    0\n}\n",
		"E0820", `"Int64" is not an interface`, 1, 13)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nfn f(d: Describable) -> Int64 {\n    0\n}\n",
		"E0821", `the value type slot takes the box form "Dyn<Describable>"`, 4, 9)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nfn f() -> Describable {\n    todo(\"unreachable\")\n}\n",
		"E0821", `the value type slot takes the box form "Dyn<Describable>"`, 4, 11)
	// A local Dyn shadows the prelude name (chapter 15's shadowing law).
	wantDiag(t, "type Dyn = Win | Lose\nfn f() {\n    let x = Dyn<Int64>(1)\n}\n",
		"E0828", `"Dyn" wants 0 type arguments, got 1`, 3, 22)
}

// --- D8: derives --------------------------------------------------------------------

func TestDerives(t *testing.T) {
	wantOK(t, "record Point { x: Int64, y: Int64 } derives Eq, Hash, Show\nfn f(p: Point) {\n    let same = p.equals(Point { x: 1, y: 2 })\n    let h = p.hash()\n    let text = p.toDebugString()\n}\n")
	wantDiag(t, "record Point { x: Int64, y: Int64 }\nimpl Eq for Point {\n}\n",
		"E0822", `"Eq" is a builtin derive target`, 2, 1)
	wantDiag(t, "record Inner { n: Int64 }\nrecord Outer { inner: Inner } derives Eq\n",
		"E0823", `the field "inner" is "Inner", which carries no "Eq"`, 2, 23)
	wantDiag(t, "record Inner { n: Int64 }\nrecord Wrap<T> { value: T } derives Eq\nfn f(i: Inner) {\n    let w = Wrap<Inner> { value: i }\n}\n",
		"E0823", `"Wrap<Inner>" instantiates "T" with "Inner", which carries no "Eq"`, 4, 18)
}

// --- D6: generic determination --------------------------------------------------------

func TestGenericDetermination(t *testing.T) {
	wantOK(t, "fn identity<T>(x: T) -> T {\n    x\n}\nfn f() {\n    let a = identity(3)\n    let b = identity<Float64>(1.0)\n}\n")
	wantOK(t, "record Pair<A, B> { first: A, second: B }\nfn f() {\n    let p1 = Pair { first: 1, second: \"a\" }\n    let p2 = Pair<Int64, String> { first: 2, second: \"b\" }\n}\n")
	wantOK(t, "record Box2<T> { value: T }\nfn f() {\n    let b: Box2<Box2<Int64> > = Box2 { value: Box2 { value: 1 } }\n}\n")
	wantOK(t, "interface Holder {\n    fn held(self) -> Int64\n}\nrecord Wrap<T> { value: T }\nimpl<T> Holder for Wrap<T> {\n    fn held(self) -> Int64 {\n        0\n    }\n}\nfn f(w: Wrap<Int64>) -> Int64 {\n    return w.held()\n}\n")
	wantOK(t, "newtype Tagged<T>(T)\nfn f() {\n    let t = Tagged<Int64>(1)\n    let v = t.value\n}\n")
	wantOK(t, "type Opt2<T> = Some2(T) | None2\nfn f() {\n    let a = Some2<Int64>(2)\n}\n")
	// Method clauses are fresh against the enclosing clause (E0826).
	wantDiag(t, "interface Boxed<T> {\n    fn unwrap<T>(self) -> T\n}\n",
		"E0826", `the method clause of "unwrap" redeclares "T"`, 2, 15)
	wantDiag(t, "fn empty<T>() -> Option<T> {\n    None\n}\nfn f() {\n    let x = empty()\n}\n",
		"E0827", `"empty" determines no type argument`, 5, 13)
	wantDiag(t, "interface Maker {\n    fn make<T>(self) -> Option<T>\n}\nrecord M { n: Int64 }\nimpl Maker for M {\n    fn make<T>(self) -> Option<T> {\n        None\n    }\n}\nfn f(m: M) {\n    let xs = m.make()\n}\n",
		"E0827", `the call of "make" determines no "T"`, 11, 16)
	wantDiag(t, "fn identity<T>(x: T) -> T {\n    x\n}\nfn f(n: Int64) -> Int64 {\n    identity<Int64, String>(n)\n}\n",
		"E0828", `"identity" wants 1 type argument, got 2`, 5, 27)
	wantDiag(t, "fn f(xs: List<Int64, String>) -> Int64 {\n    0\n}\n",
		"E0828", `"List" wants 1 type argument, got 2`, 1, 14)
	// Instantiation honesty: category is rechecked per instantiation.
	wantDiag(t, "byval record Box<T> { value: T }\nrecord User { name: String }\nfn f(u: User) {\n    let b = Box<User> { value: u }\n}\n",
		"E0601", `"Box<User>" instantiates the field "value" with "User", a gc record`, 4, 17)
	wantDiag(t, "byval type Slot<T> = Full(T) | Empty\nrecord User { name: String }\nfn f(u: User) {\n    let s = Full<User>(u)\n}\n",
		"E0702", `"Full<User>" instantiates the payload of "Slot" with "User", a gc record`, 4, 18)
	// A generic function name is a family, not one function (chapter 12).
	wantDiag(t, "fn identity<T>(x: T) -> T {\n    x\n}\nfn f() {\n    let g = identity\n}\n",
		"E1004", `"identity" is a family of functions, not one function`, 5, 13)
}

// --- D6: where bounds -------------------------------------------------------------------

func TestWhereBounds(t *testing.T) {
	wantOK(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\nfn show<T>(x: T) -> String where T: Describable {\n    x.describe()\n}\nfn f(u: User) -> String {\n    return show(u)\n}\n")
	wantOK(t, "interface A1 {\n    fn a(self) -> Int64\n}\ninterface B1 {\n    fn b(self) -> String\n}\nfn use2<T>(x: T) -> Int64 where T: A1 + B1 {\n    x.a()\n}\n")
	wantOK(t, "interface Holder {\n    type Item\n}\nfn take<T>(x: T) -> Int64 where T: Holder, T.Item == Int64 {\n    0\n}\n")
	wantDiag(t, "fn f<T>(x: T) -> Int64 where T: Int64 {\n    0\n}\n",
		"E0829", `the bound "Int64" names no interface`, 1, 33)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nfn show<T>(x: T) -> String where T: Describable {\n    x.describe()\n}\nrecord User { name: String }\nfn f(u: User) -> String {\n    show(u)\n}\n",
		"E0830", `"User" implements no "Describable"; the where bound of "show" holds at every call`, 9, 5)
	wantDiag(t, "interface Describable {\n    fn describe(self) -> String\n}\nfn show<T>(x: T) -> String where T: Describable {\n    x.describe()\n}\nrecord User { name: String }\nfn f(u: User) -> String {\n    show<User>(u)\n}\n",
		"E0830", `"User" implements no "Describable"; the where bound of "show" holds at every call`, 9, 5)
	wantDiag(t, "interface It {\n    type Iter\n}\nfn f<T, U>(x: T, y: U) -> Int64 where T: It, T.Iter == U {\n    0\n}\n",
		"E0831", `the right side "U" is a generic parameter`, 4, 56)
}

// --- D9: the iteration protocol ------------------------------------------------------------

func TestIterProtocol(t *testing.T) {
	wantOK(t, "fn f() {\n    for r in \"abc\" {\n        let _ = r\n    }\n    for i in 0..3 {\n        let _ = i\n    }\n    let xs = [1, 2, 3]\n    for x in xs {\n        let _ = x\n    }\n}\n")
	wantOK(t, "record CountIter { n: Int64 }\nimpl Iterator<Int64> for CountIter {\n    fn next(mut self) -> Option<Int64> {\n        None\n    }\n}\nrecord Range2 { n: Int64 }\nimpl Iterable<Int64> for Range2 {\n    type Iter = CountIter\n    fn iterator(self) -> CountIter {\n        CountIter { n: 0 }\n    }\n}\nfn f(r: Range2) {\n    for x in r {\n        let _ = x\n    }\n}\n")
	wantOK(t, "fn f() {\n    let it = \"ab\".iterator()\n    let first = it.next()\n}\n")
	wantOK(t, "fn f() {\n    let d = Dyn<Iterator<Rune> >(\"abc\".iterator())\n    let first = d.next()\n}\n")
	wantOK(t, "fn f() {\n    let mapped = \"abc\".iterator().map(|r| r).collect()\n    let total = \"ab\".iterator().fold(0, |acc, r| acc)\n    let n = \"abc\".iterator().count()\n}\n")
	wantDiag(t, "fn f() {\n    for x in 5 {\n        let _ = x\n    }\n}\n",
		"E0901", `"Int64" implements no "Iterable"`, 2, 14)
	wantDiag(t, "fn f(s: String) {\n    for r in s.iterator() {\n        let _ = r\n    }\n}\n",
		"E0901", "a bare iterator is consumed by explicit next calls", 2, 14)
	wantDiag(t, "fn f() {\n    let r = 0..1.5\n}\n",
		"E0902", `the operand "1.5" is "Float64"`, 2, 16)
	wantDiag(t, "record CountIter { n: Int64 }\nimpl Iterator<Int64> for CountIter {\n    fn next(mut self) -> Option<Int64> {\n        None\n    }\n}\nbyres record FileHandle { fd: Int64 }\nimpl Iterable<Int64> for FileHandle {\n    type Iter = CountIter\n    fn iterator(self) -> CountIter {\n        CountIter { n: 0 }\n    }\n}\n",
		"E0903", `"FileHandle" is resource-category`, 8, 1)
	wantDiag(t, "record Count { n: Int64 }\nimpl Iterable<Int64> for Count {\n    type Iter = Int64\n    fn iterator(self) -> Int64 {\n        0\n    }\n}\n",
		"E0904", `the impl binds "Iter" to "Int64"`, 2, 1)
	wantDiag(t, "fn f(s: String) {\n    for (a, b) in s {\n        let _ = a\n    }\n}\n",
		"E0501", `the loop head pattern is a 2-tuple, the element type is "Rune"`, 2, 9)
	wantDiag(t, "fn f() {\n    let r = 1i32..5i64\n}\n",
		"E0501", `operands of ".." are Int32 and Int64`, 2, 17)
}

// --- D9: the collection face -----------------------------------------------------------------

func TestCollectionsFace(t *testing.T) {
	wantOK(t, "fn f() {\n    let xs: List<Int64> = [1, 2, 3]\n    let ys: List<Int64> = []\n}\n")
	wantOK(t, "fn firstOf(xs: List<Int64>) -> Option<Int64> {\n    xs.get(0)\n}\nfn stats(mm: Map<String, Int64>, ss: Set<Int64>) -> Int64 {\n    let v = mm.get(\"a\")\n    let has = ss.has(1)\n    0\n}\n")
	wantOK(t, "fn f(text: String) {\n    let n = text.byteLength()\n    let s = text.byteSlice(0, 1)\n    let c = text.runeCount()\n    let r = text.charAt(0)\n}\n")
	wantOK(t, "fn countEntries(mm: Map<String, Int64>) -> Int64 {\n    let total = 0\n    for (k, v) in mm {\n        let _ = k\n    }\n    total\n}\n")
	wantDiag(t, "fn f() {\n    let xs = []\n}\n",
		"E1501", "the empty list literal determines no element type", 2, 14)
	wantDiag(t, "interface Describable {\n    fn describe(self, prefix: Int64) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self, prefix: Int64) -> String {\n        \"u\"\n    }\n}\nfn f(u: User) {\n    let text = u.describe(\"p\")\n}\n",
		"E0501", "the argument is String, the parameter is Int64", 11, 27)
	// Element agreement: the first element sets the type, a later one
	// that disagrees anchors its own first token (unit-only case; the
	// golden table pins the other E0501 reuse sites).
	wantDiag(t, "fn f() {\n    let xs = [1, \"a\"]\n}\n",
		"E0501", "the elements are Int64 and String", 2, 18)
	// The stdlib surface stays behind the honest boundary: a member the
	// anchored families do not name is not privately rejected (D4 rule 4).
	wantBnd(t, "fn f(xs: List<Int64>) {\n    let ys = xs.add(1)\n}\n", bndStdModules)
	// Method-call arity keeps the spec-gap boundary row (follow-up #5).
	wantBnd(t, "interface Describable {\n    fn describe(self) -> String\n}\nrecord User { name: String }\nimpl Describable for User {\n    fn describe(self) -> String {\n        \"u\"\n    }\n}\nfn f(u: User) {\n    let text = u.describe(1)\n}\n", bndArityGap)
}
