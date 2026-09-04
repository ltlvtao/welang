# Examples: composites-ownership

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–8. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Methods, interfaces, derives, sum types, and the reference model are annotated as pending their chapters.

### Records, construction, update

```we
record User {
    id: UserId,
    name: String,
}

byval record Point {
    x: Float64,
    y: Float64,
}

newtype UserId(Int64)

let u1 = User { id: UserId(1), name: "Ada" }
let u2 = User { name: "bob" with &u1 }   // new value; u1 unchanged
let id = u1.id.value                     // field access, then unwrap

User { id: UserId(1), name: 42 }         // E0501: name is String
User { id: UserId(1), email: "a" }       // E0604: no such field
User { name: "bob" with &p }             // E0603: p is not a User
u1.name = "bob"                          // E0105: no field assignment
```

### Ownership categories

```we
byval record Bad { u: User }             // E0601: User is gc, not
                                        // value or a base type

byres record FileHandle {
    fd: Int64,
}
// release operations and Releasable enforcement are the
// resource chapter's; this chapter fixes the category:

FileHandle { fd: 3 with &h }             // E0606: resources are
                                        // never updated this way

fn scale(p: Point, k: Float64) -> Point {
    Point { x: p.x * k, y: p.y * k }    // byval: the caller's p is
}                                       // an independent copy
```

### Tuples and the unit type

```we
let pair: (Int64, String) = (1, "a")
let (n, s) = pair                        // destructuring let

match pair {
    (0, tag) => tag,
    (count, _) => "many",
}

let nothing = ()                         // the unique unit value
let alsoNothing = { io.println("hi") }   // block typed ()

(1, "a", 3, 4, 5, 6, 7, 8, 9)            // E0602: arity above eight
let one: (Int64) = (1)                   // grouping, not a tuple
```

### Value discard

```we
fn step() -> Int64 { ... }

step()                                   // E0605: non-unit value dropped
let _ = step()                           // explicit discard: legal
io.println("hi")                         // unit: no ceremony needed

fn quietly() {
    step()                               // E0605: valueless function,
    let _ = step()                       // same rule at the body tail
}
```

### If arm agreement and shadowing

```we
let flag = true

if flag { 1 } else { io.println("no") }  // E0501: Int64 vs ()

let x = 1
let x = normalize(x)                     // later binding wins; the
                                        // first x is out of reach

fn trim(a: String) -> String {
    let a = a.trim()                     // shadowing a parameter is
    a                                     // legal: nearest binding
}
```

### Pending later chapters

```we
// Methods (mut self, inherent), interfaces, impl, and derives
// (Eq, Hash on newtype) arrive with the interfaces chapter;
// variant patterns and Never with sum types; Ref/Shared with the
// concurrency chapters; List<T> and indexing with collections:
//
// impl Eq for UserId { ... }
// match shape { Circle(r) => r, _ => 0 }
// let xs: List<Int64> = build()
```
