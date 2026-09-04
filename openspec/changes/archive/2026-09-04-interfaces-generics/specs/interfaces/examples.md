## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–10. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Generic methods, Encodable/Decodable, Shareable, and effect sets are annotated as pending or deferred.

### Interfaces, impls, and inherent methods

```we
interface Describable {
    fn describe(self) -> String
}

record User {
    id: UserId,
    name: String
} derives Eq, Show

impl Describable for User {
    fn describe(self) -> String {
        "user ${self.name}"
    }
}

impl User {
    pub fn displayName(self) -> String {
        self.name
    }
}

let u = User { id: UserId(1), name: "Ada" }
let text = u.describe()          // through the interface
let name = u.displayName()       // inherent, pub-reachable
```

### Associated types and default methods

```we
interface Stream {
    type Element
    fn next(mut self) -> Element
}

interface Greeter {
    fn greet(self) -> String {
        "hello"                  // a default method: bodies in interfaces
    }
    fn name(self) -> String
}

record IntStream {
    data: Int64
}

impl Stream for IntStream {
    type Element = Int64         // bindings come before the methods
    fn next(mut self) -> Int64 {
        self.data = self.data + 1
        self.data
    }
}

impl Greeter for User {
    fn name(self) -> String {
        self.name
    }
}

let g = u.greet()                // default body runs: greet is omitted
```

### Mutation through mut self

```we
record Counter {
    count: Int64
}

impl Counter {
    fn bump(mut self) {
        self.count = self.count + 1   // the one field-assignment form
    }
    fn read(self) -> Int64 {
        self.count
    }
}

let c = Counter { count: 0 }
c.bump()
let n = c.read()                 // 1: the gc receiver mutated in place

impl Counter {
    fn reset(self) {
        self.count = 0           // E0813: field write outside a mut self
    }                            // method body
}

byval record Pair {
    left: Int64,
    right: Int64
}

impl Pair {
    fn swap(mut self) {          // E0812: mut self receiver on a
    }                            // value-category type
}
```

### Dyn values

```we
let d = Dyn<Describable>(u)      // the one construction form
let text = d.describe()          // the interface's method set alone

let bad = Dyn<Describable>(42)   // E0818: construction from a
                                 // non-implementing type
let s = Dyn<Stream>(stream)      // E0819: Stream has associated types
let n = Dyn<Int64>(5)            // E0820: argument is not an interface

fn label(x: Describable) -> String {
    x.describe()
}                                // E0821: interface name used as a
                                 // value type; write Dyn<Describable>
```

### Derives

```we
record Point {
    x: Float64,
    y: Float64
} derives Eq, Hash, Show

newtype UserId(Int64) derives Eq, Show

type Shape = Circle(Float64) | Rectangle(Float64, Float64) derives Eq

let same = p1.equals(p2)         // composite equality is the generated
let text = p1.toDebugString()    // method; Show asks nothing
let h = p1.hash()

let eq = p1 == p2                // E0501: == compares base types only
impl Eq for Point {             // E0822: manual impl of a builtin
}                                // derive target
record Wrap { inner: Shape } derives Ord
                                 // E0824: unknown or duplicate derive
                                 // target; the set is Eq, Hash, Show
```

### Generics and where clauses

```we
fn identity<T>(x: T) -> T {
    x
}

record Box<T> {
    value: T
}

impl<T> Describable for Box<T> where T: Describable {
    fn describe(self) -> String {
        "box of ${self.value.describe()}"
    }
}

fn show<T>(x: T) -> String where T: Describable {
    x.describe()
}

fn sum<T>(s: T) -> Int64 where T: Stream, T.Element == Int64 {
    s.next() + 1                 // T.Element is Int64 inside
}

let a = identity(3)              // T unifies with the argument: Int64
let b = identity<Float64>(1.0)   // the explicit form
let nested: Box<Box<Int64> > = Box { value: Box { value: 1 } }
                                 // nested closers separated: >> is the
                                 // shift token under maximal munch

fn empty<T>() -> Box<T> {
    ...                          // a Never expression satisfies any return;
}                                // producers are the error-mechanism
                                 // chapter's

let bad1 = empty()               // E0827: undetermined; write empty<Int64>()
let bad2 = identity<Int64, String>(1)
                                 // E0828: type argument arity mismatch
let bad3: Box<Box<Int64>>        // E0105: unseparated nested closers
```

### Rejected forms

```we
impl Describable for Foreign {  // E0810: orphan impl — neither side
}                                // is declared in this module
impl Describable for (Int64, String) {
}                                // E0811: impl head is not a nominal type
impl Describable for User {
    fn describe(me) -> String {
        ""
    }
}                                // E0802: receiver must be named self
interface Broken {
    fn helper(x: Int64)
}                                // E0801: method declares no receiver
impl User {
    fn name(self) -> String {
        ""
    }
}                                // E0814: member name collision — the
                                 // field name already answers
fn render<T>(x: T) -> String {
    x.describe()
}                                // E0817: unconstrained parameter is
                                 // opaque; add where T: Describable
fn bound<T>(x: T) where T: Point {
}                                // E0829: where bound is not a declared
                                 // interface
```

### Pending later chapters

```we
// Generic methods, Encodable/Decodable (the JSON change), Shareable
// (the concurrency chapter), and interface-level where clauses are
// not ratified at this chapter; effect-set checks belong to the
// effects chapter and are deliberately absent here:
//
// impl Box<T> { fn pair<U>(self, other: U) -> ... }
// record Config derives Encodable
```
