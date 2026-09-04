# We Language Specification — Chapter 9: Sum types

### Requirement: Sum declarations

A sum type declaration is a top-level item `type Name = V1 | V2 | ... | Vn` with n at least one, optionally prefixed by `pub` and `byval`: `Name` is the sum type's name and each `Vi` a variant — `Unit` alone for a unit variant, or `Unit(T1, T2, ..., Tk)` with k from 1 to 8, each `Ti` a type reference under chapter 7. There is no `|` before the first variant. A payload arity above eight is rejected with `E0701` — the fix is a record payload, which names its fields. The type's name and every variant's name are PascalCase (`E0011`) and join the module's one name space under chapter 6, colliding with no other name (`E0404`). Layout follows chapter 2: the `|` between variants is an operator token, so a declaration spanning lines carries `=` or `|` at each line's end — a line starting with `|` after a complete variant is rejected under chapter 2's continuation diagnostic (`E0102`). A sum type is a composite type under chapter 8: `type` is the `gc` category and `byval type` the `value` category; the resource and newtype categories do not apply to sums, and a sum type MUST NOT change category after declaration.

#### Scenario: A sum declaration parses as a top-level item

- **WHEN** `type Shape = Circle(Float64) | Rectangle(Float64, Float64)` appears at the top level
- **THEN** it declares the gc sum type `Shape` with variants `Circle` and `Rectangle` and their payload types

#### Scenario: A multi-line declaration trails its separators

- **WHEN** a declaration is written `type Shape =` on one line, `Circle(Float64) |` on the next, and `Rectangle(Float64, Float64)` on the last
- **THEN** the lines continue per chapter 2's continuation set and the declaration parses as one item

#### Scenario: A leading separator on its own line is rejected

- **WHEN** a declaration is written `type Shape = Circle(Float64)` on one line and `| Rectangle(Float64, Float64)` starting the next
- **THEN** the first line already ends at a boundary and the next begins with `|`, a token that cannot begin a statement; the compiler rejects it with `E0102:` statement begins with a continuation token

#### Scenario: A payload arity above eight is rejected

- **WHEN** a variant with nine payload types appears
- **THEN** the compiler rejects it with `E0701:` variant payload arity above eight; the fix is a record payload

#### Scenario: A variant name collides in the module name space

- **WHEN** a module declares `type Shape = Circle(Float64)` and also `fn Circle(r: Float64)`
- **THEN** the compiler rejects the second declaration with `E0404:` duplicate name in one module

#### Scenario: byval declares the value category

- **WHEN** `byval type Axis = X(Float64) | Y(Float64)` appears
- **THEN** it declares a value-category sum type under chapter 8's ownership categories

### Requirement: Variant constructors

A variant with payload is constructed by the call form `Name(e1, ..., ek)`, k matching the declared payload arity, each `ei` an expression of its payload type at an agreement position (`E0501` on mismatch); the constructed expression's type is the enclosing sum type. A unit variant is constructed by its bare name — a PascalCase identifier expression whose type is the sum type. Whether a payloaded variant's bare name is a first-class value is the function-types chapter's; until then a payloaded constructor used without its payload is rejected with `E0704`. A public variant of an imported module is reached as `module.Name(args)` per chapter 6, the same qualified form as every other module-level name. The one name space guarantees a name is never both a function and a variant (`E0404`), so the call form is never ambiguous.

#### Scenario: Construct with payload

- **WHEN** `Circle(1.0)` appears where a `Shape` is required, `type Shape = Circle(Float64) | Rectangle(Float64, Float64)` declared
- **THEN** it is an expression of type `Shape` carrying the payload

#### Scenario: A unit variant stands bare

- **WHEN** `None` appears where an `Option` is required, `None` a declared unit variant
- **THEN** the bare PascalCase identifier is the constructed value; no call form is written

#### Scenario: A payload type mismatch is rejected

- **WHEN** `Circle("big")` appears with `Circle(Float64)` declared
- **THEN** the compiler rejects the argument under `E0501`; the fix is an explicit conversion

#### Scenario: A payloaded constructor without its payload is rejected

- **WHEN** `Circle` appears as a value, `Circle(Float64)` declared
- **THEN** the compiler rejects it with `E0704:` payloaded variant constructor used without arguments; first-class constructors are the function-types chapter's

### Requirement: Value sums

A `byval type` has value semantics under chapter 8's value category: construction, assignment, binding, argument passing, and return copy the whole value. Every payload type of a value sum MUST be a base type or of the value category (`E0702` otherwise) — the mirror of chapter 8's value records: a copy is only honest when everything in it is copyable by value.

#### Scenario: Passing copies

- **WHEN** a value-sum value is passed to a function and the callee inspects it
- **THEN** the callee holds an independent copy; the caller's value is unaffected

#### Scenario: A non-value payload is rejected

- **WHEN** `byval type Bad = Wrap(User)` appears with `User` a gc record
- **THEN** the compiler rejects it with `E0702:` value sum payload is not of the value category or a base type

### Requirement: The bottom type

`Never` is the bottom type: it has no values; an expression of type `Never` is a computation that never produces one. The forms that produce `Never` are the error-mechanism chapter's; this chapter fixes the type's positions and agreements. `Never` may appear only as a declared function return type: a variable binding, field, parameter, variant payload, or tuple element annotated `Never` is rejected with `E0703`. An expression of type `Never` may appear at any type-agreement position of any expected type — it never produces a value to disagree — the one deliberate exemption from chapter 7's no-implicit-conversion obligation, recorded as such. `Never` drops out of arm agreement: for if under chapter 3 and chapter 8, and for match under chapter 4, an arm whose type is `Never` is excluded from the agreement check, and arms that are all `Never` agree as `Never`.

#### Scenario: A forbidden annotation is rejected

- **WHEN** `let x: Never = ...` or a parameter `f(x: Never)` appears
- **THEN** the compiler rejects it with `E0703:` Never in a non-return annotation position

#### Scenario: A Never arm drops out of agreement

- **WHEN** an if or match arm's type is `Never` and the other arms agree on `Int64`
- **THEN** the construct's type is `Int64`; the `Never` arm is not compared

#### Scenario: All-Never arms agree as Never

- **WHEN** every arm of an if-with-else or match has type `Never`
- **THEN** the construct's type is `Never`

#### Scenario: A Never expression satisfies any return type

- **WHEN** a function declares `-> Int64` and a path's final expression has type `Never`
- **THEN** the path satisfies the declaration; the expression never produces a value

## Examples (non-authoritative)

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

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| sum type | sum 类型 |
| variant | 变体 |
| unit variant | 无载荷变体 |
| payloaded variant | 载荷变体 |
| payload | 载荷 |
| variant constructor | 变体构造器 |
| value sum | 值 sum |
| bottom type | 底类型 |
