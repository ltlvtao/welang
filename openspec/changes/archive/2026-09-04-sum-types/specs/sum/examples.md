# Examples: sum-types

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–9. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Interfaces, derives, the error forms that produce `Never`, and first-class constructors are annotated as pending their chapters.

### Declarations and construction

```we
type Shape =
    Circle(Float64) |
    Rectangle(Float64, Float64)

byval type Axis = X(Float64) | Y(Float64)

pub type Event = Ping | Message(String)

let c = Circle(1.0)                       // payloaded: call form
let quiet = Ping                          // unit: bare name
let e = std_event.Message("hi")           // module-qualified variant

type Wide = Nine(Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64)
                                         // E0701: payload arity above eight
type Shape = Circle(Float64) | fn(Float64)
                                         // E0404: variant name collides
```

### Matching and exhaustiveness

```we
fn area(s: Shape) -> Float64 {
    match s {
        Circle(r) => 3.14159 * r * r
        Rectangle(w, h) => w * h         // exhaustive: no wildcard needed
    }
}

match code {
    200 | 404 => handleFine()
    _ => handleOther()                   // base type: wildcard required
}

match s {
    Circle(r) => r
}                                        // E0305: Rectangle not covered
match s {
    Circle(r) if r > 1.0 => r
    Rectangle(w, h) => w
}                                        // E0306: guarded arm needs an
                                         // unguarded exhaustive fallback
match s {
    _ => 0.0
    Circle(r) => r                       // E0307: arm after a wildcard
}
match s {
    Circle(r) => r
    Circle(r) => 0.0                     // E0307: variant listed twice
    _ => 0.0
}
```

### Variant patterns

```we
match s {
    Triangle(r) => r                     // E0303: no such variant
}
match s {
    Circle(a, b) => a                    // E0304: payload arity mismatch
}
let Circle(r) = c                        // E0105: refutable patterns
                                         // are match-only

match pair {
    (Circle(r), _) => r                  // nesting: variant in tuple
    _ => 0.0
}

match either {
    (a, _) | (_, a) => a                 // E0501: a binds Int64 here
    _ => 0                               // and String there
}
```

### The bottom type

```we
fn bail() -> Never { ... }               // producers: error-mechanism
                                         // chapter's

let x: Never = bail()                    // E0703: only return-type
fn f(x: Never) { }                       // annotations allowed

fn find(id: Int64) -> Int64 {
    if id == 0 {
        bail()                           // Never satisfies Int64 here
    } else {
        id
    }
}
```

### Pending later chapters

```we
// The forms that produce Never (panic, todo, trap names) arrive with
// the error-mechanism chapter; Eq/Show derives on sums and Dyn<...>
// open sets with the interfaces chapter; first-class constructors
// with the function-types chapter:
//
// derives Eq for Shape { ... }
// let g = Circle                        // pending first-class constructors
```
