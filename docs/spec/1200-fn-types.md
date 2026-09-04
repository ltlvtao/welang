# We Language Specification — Chapter 12: Function types and closures


### Requirement: The function type

A function type is `fn(T1, ..., Tn) [effect-segment] -> T`: the `fn` keyword, a parenthesized comma-separated list of zero or more parameter types, each itself a type reference under chapter 7, an optional effect segment of one or more bare space-separated tags between the parameter types and the arrow — chapter 16's ratification, no `effect` keyword in type position — an arrow, and a return type, itself a type reference and REQUIRED — a function that produces no value has the type `fn(...) -> ()`, the unit type under chapter 8. Omitting the segment states the pure function type; the tags' declarations and resolution, and the agreement checks the segment participates in, are chapter 16's. The parameter list carries no arity cap, mirroring chapter 6's declaration surface. A function type is monomorphic: its parameter and return positions hold type references only, never generic parameters — a generic function names a family, not one function, and has no single function type (Function values). Type slots accept the function type as a type-reference form under chapter 7's amendment.

#### Scenario: A function type in an annotation slot

- **WHEN** `let f: fn(Int64) -> Int64` appears with a matching function value on the right
- **THEN** the annotation is the function type of exactly that signature; `f` binds function values of no other

#### Scenario: An effect segment in a function type

- **WHEN** `let f: fn(Int64) io -> Int64` appears — bare space-separated tags between the parameter types and the arrow
- **THEN** the annotation is the function type of Int64-to-Int64 functions that may perform io; the segment's spelling is bare tags, the `effect` keyword fitting no production in type position

#### Scenario: A zero-parameter function type

- **WHEN** `fn() -> Bool` appears in a type slot
- **THEN** it is the type of zero-parameter functions returning `Bool`

#### Scenario: A valueless signature types through the unit type

- **WHEN** `fn(String) -> ()` appears in a type slot
- **THEN** it is the type of functions that produce no value; the return position is the unit type, never omitted

#### Scenario: A malformed function type is rejected

- **WHEN** a type slot holds `fn(Int64)` with no arrow, or any other malformed function-type fragment
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic (`E0105`) at the slot; the arrow and the return type are part of the production

#### Scenario: A function type nests in composite positions

- **WHEN** a tuple type `(fn(Int64) -> Int64, Int64)` or a generic application `Box<fn(Int64) -> Int64>` appears in a type slot
- **THEN** the function type is a type-reference form like any other; chapter 8's tuple and chapter 10's application rules apply unchanged

### Requirement: Function values

A monomorphic declared fn — a top-level fn of chapter 6 carrying no generic clause — is a value: used by its bare name in expression position, its type is its signature `fn(ParamTypes) -> ReturnType`, a valueless declaration typing `fn(...) -> ()`. A call through a function-typed value is chapter 2's postfix call like any other. A function value MUST agree with a function type only at the exactly matching signature: every agreement position of chapter 7 applies, and a mismatched signature is `E0501` — no variance, no arity or return widening, chapter 10's no-subtyping stance carrying to function types. A generic fn's name is not a value: the generic clause makes the name a family, and using it in value position is `E1004`; the call form remains its only use. Method values are not ratified: a method name resolved on its receiver without a call resolves per chapter 10 but fits no ratified value production — the compiler rejects it under chapter 2's unexpected-token diagnostic (`E0105`) — and the receiver-carrying closure is the form that travels.

#### Scenario: A fn name binds and calls

- **WHEN** `square` is declared as `fn square(x: Int64) -> Int64` and `let f = square` then `f(3)` appears
- **THEN** `f` has the type `fn(Int64) -> Int64`; the call is chapter 2's postfix call and yields `9`

#### Scenario: A signature mismatch is rejected

- **WHEN** `let f: fn(Int64, Int64) -> Int64 = square` appears with `square` of one parameter
- **THEN** the compiler rejects it with `E0501` at the binding-versus-annotation position; the signature must match exactly

#### Scenario: A generic fn name in value position is rejected

- **WHEN** `let g = pairUp` appears with `fn pairUp<T>(a: T, b: T) -> Tuple2<T>` declared
- **THEN** the compiler rejects it with `E1004:` generic function name in value position; the call form `pairUp(1, 2)` is unaffected

#### Scenario: A function value as an argument

- **WHEN** `apply(square, 3)` appears with `fn apply(f: fn(Int64) -> Int64, x: Int64) -> Int64` declared
- **THEN** the argument agrees with the parameter's function type exactly and the call is legal

#### Scenario: A method value is not ratified

- **WHEN** `let f = user.describe` appears, `describe` a method of `user`'s type
- **THEN** the access resolves per chapter 10's member name resolution but fits no ratified value production; the compiler rejects it under chapter 2's unexpected-token diagnostic (`E0105`); `|u| u.describe()` carries the receiver instead

### Requirement: The full closure form

The full closure form is `fn(params) block` or `fn(params) -> type block` — anonymous, in expression position. The parameter list is chapter 6's exactly: zero or more `name: type` pairs, every parameter annotated, no receiver exception. The return type is omittable, its absence stating that the closure produces no value, typing `fn(...) -> ()`. The body is a chapter-2 block. The form is self-contained: parameter and return types MUST be written, never inferred, and it is legal at any expression position. It is one token of lookahead from chapter 6's declaration: `fn` followed by `(` is a closure in expression position; `fn` followed by a name at the top level is a declaration — different productions in different positions.

#### Scenario: A full closure binds with its type

- **WHEN** `let add = fn(x: Int64, y: Int64) -> Int64 { x + y }` appears
- **THEN** `add` has the type `fn(Int64, Int64) -> Int64`; the block value is the return value

#### Scenario: A valueless full closure

- **WHEN** `fn(s: String) { put(s) }` appears
- **THEN** it is a closure of type `fn(String) -> ()`, its absence of a return type stating no value

#### Scenario: A full closure passed inline

- **WHEN** `apply(fn(x: Int64) -> Int64 { x * 2 }, 5)` appears with `apply` as above
- **THEN** the closure agrees with the parameter's function type and the call is legal

#### Scenario: A closure is not a declaration

- **WHEN** `fn(x: Int64) -> Int64 { x }` appears as an expression item and `fn triple(x: Int64) -> Int64 { x * 3 }` appears at the top level
- **THEN** the first is a closure — `fn` met `(`; the second is chapter 6's declaration — `fn` met a name; one token separates the productions

### Requirement: The short closure form

The short closure form is `|p1, ..., pk| body` with k from 1 up: a pipe-delimited comma-separated parameter list, then a body that is one expression or a `{ ... }` block; the body's value is the closure's return value, and `return` and `defer` inside it follow Closure bodies are function bodies. A zero-parameter closure is the full form's only — `||` is chapter 1's logical-or token, one token by maximal munch, so no zero-parameter short form exists to write. A parameter is `name` bare or `name: type` annotated. The fully annotated form is self-contained and legal at any expression position. A bare parameter — any one of them — makes the closure depend on an expected function type at its position: a binding's annotation, a parameter's declared function type, or a return position's declared function type. There, each bare parameter takes the corresponding type from the expectation, the return type comes from the body, and each annotated parameter MUST agree with the expectation (`E0501` otherwise). Where no expected function type exists, a bare-parameter closure is rejected with `E1001`. The short form sits outside the primary/postfix/unary skeleton with the full form, and its body is maximal: postfixes and binary operators after the first expression of the body belong to the body, so the closure itself takes no postfix — calling it is parenthesized, `(|x| x + 1)(2)` — and as a direct operand of a binary or unary operator it MUST be parenthesized: an unparenthesized `|` in operand position fits no production and is rejected under chapter 2's unexpected-token diagnostic (`E0105`). A statement-initial `|` is chapter 2's `E0102`, the statement-start class not admitting it — and a dropped closure value would be `E0605` in any case.

#### Scenario: An annotated short closure stands alone

- **WHEN** `let inc = |x: Int64| x + 1` appears
- **THEN** the closure is self-contained, of type `fn(Int64) -> Int64`; no expectation was needed

#### Scenario: A bare short closure takes its types from an annotation

- **WHEN** `let f: fn(Int64) -> Int64 = |x| x + 1` appears
- **THEN** `x` takes `Int64` from the annotation, the return comes from the body, and the binding agrees

#### Scenario: A bare short closure as an argument

- **WHEN** `apply(|x| x + 1, 3)` appears with `apply`'s parameter of function type
- **THEN** the closure's parameters take the parameter's declared function type; the call is legal

#### Scenario: A bare short closure without an expectation is rejected

- **WHEN** `let f = |x| x + 1` appears with no annotation
- **THEN** the compiler rejects it with `E1001:` bare-parameter closure without an expected function type; annotate the parameters or the binding

#### Scenario: A short closure in operand position is parenthesized

- **WHEN** `1 + |x| x` appears
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic (`E0105`) at the operand; `1 + (|x| x)` is the legal grouping

#### Scenario: A statement-initial short closure is rejected

- **WHEN** a statement begins with `|x| x + 1`
- **THEN** the compiler rejects it with `E0102:` statement begins with a continuation token; `|` opens no statement

#### Scenario: No zero-parameter short form exists

- **WHEN** `let z = || ready()` appears — or the spaced `| |` variant — where a zero-parameter closure is wanted
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic (`E0105`): `||` is the logical-or token and a parameter list opens with a name, so the zero-parameter closure is the full form `fn() -> ...` only

#### Scenario: Calling a short closure takes parentheses

- **WHEN** `(|x| x + 1)(2)` appears
- **THEN** the parentheses fix the receiver; the value is `3`, and without them the call postfix would belong to the body

### Requirement: Closure bodies are function bodies

A closure's body block is a function body: its final expression is its return value under chapter 2's block value; `return` returns from the closure, never from an enclosing named function — a control-flow target is always locally decidable, the enclosing call chain never consulted; early `return` from nested blocks inside the body is legal under chapter 6's model; and a `defer` inside a closure body runs at the closure body's exit under chapter 3. The function-body contexts of `E0401` are chapter 6's named fns and this chapter's closures; the module top level and defer bodies remain returnless. A closure is itself a value like any other — it may be bound, passed, returned from its enclosing function, and called later, its captures keeping the reach they were created with (Capture discipline).

#### Scenario: return exits the closure only

- **WHEN** a fn holds `let get = fn(m: Option<Int64>) -> Int64 { return match m { Some(n) => n None => 0 } }` and calls `get(Some(3))`
- **THEN** the `return` carries the closure's value; the enclosing fn's control flow is untouched, its own `return` still required to carry its own value

#### Scenario: defer runs at the closure body's exit

- **WHEN** a closure body holds `defer { done() }` before its final expression
- **THEN** `done()` runs when the closure body exits — per call — under chapter 3's LIFO rule

#### Scenario: A closure escapes its enclosing fn

- **WHEN** a fn returns a closure capturing a gc-category binding and the caller invokes the returned closure after the fn has returned
- **THEN** the closure is callable and sees its captures per Capture discipline; gc means the traced cell outlives the frame

### Requirement: Capture discipline

A closure is of the gc category — one category, reference semantics: storable, passable, returnable, and escaping; a value-category closure would need escape analysis this specification does not ratify. Captures answer chapter 8's ownership categories. A binding of a gc-category type is captured by reference: reads and assignments inside the body see and mutate the live binding, the captured cell being traced and sound to escape. A binding of a value-category type or of a base type is captured by copy at closure creation: the body reads the value as it was then, later mutations of the outer binding are invisible, and an assignment inside the body to such a captured binding is rejected with `E1003` — the snapshot is frozen; a local binding carries the intermediate value instead. A resource-category binding MUST NOT be captured — `E1002:` resource binding captured by a closure: a closure outlives its creation site by construction, and a handle escaping the single deterministic release point a resource's discipline depends on is not honest; the resource's contents materialize first and the closure captures the materialized value.

#### Scenario: A gc binding is captured live

- **WHEN** a `var` binding of a gc record type is captured and the closure's body assigns it, then the closure is called
- **THEN** the assignment mutates the live binding; the change is visible outside and persists after the call

#### Scenario: A value binding is captured by copy

- **WHEN** a `byval` record binding is captured, the outer binding is rebound after the closure's creation, and the closure is called
- **THEN** the closure reads the value it saw at creation; the rebind is invisible to it

#### Scenario: A resource binding is not captured

- **WHEN** a closure body reads a `byres` record binding of the enclosing scope
- **THEN** the compiler rejects it with `E1002:` resource binding captured by a closure; materialize the contents first and capture those

#### Scenario: An assignment to a captured value binding is rejected

- **WHEN** a `var n: Int64` binding is captured and the closure's body holds `n = n + 1`
- **THEN** the compiler rejects it with `E1003:` assignment to a captured value-category binding; carry the intermediate in a local binding or a gc cell

#### Scenario: A gc var counter works through capture

- **WHEN** `var c = Counter { n: 0 }` with gc `Counter` and a closure body reassigns `c` across calls
- **THEN** the rebinds act on the live binding; each call sees the previous call's result

### Requirement: Function types diagnostics segment

The fn-types chapter owns registry segment `E1000`–`E1099` declared in `docs/spec/diagnostics.toml` `[segments]`. Allocations: `E1001` bare-parameter closure without an expected function type, `E1002` resource binding captured by a closure, `E1003` assignment to a captured value-category binding, `E1004` generic function name in value position. `E1000` and `E1005`–`E1099` remain reserved for amendments of this chapter. Trigger semantics live in this chapter's Requirements; entries live in the registry.

#### Scenario: A fn-types code is emitted

- **WHEN** any `E10xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `1200-fn-types`

#### Scenario: A later change needs this segment's codes

- **WHEN** a later chapter ratifies rules requiring new function-value diagnostics — the combinator change among them
- **THEN** its change extends the registry within `E1000`–`E1099` in the same change, or claims its own segment

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–12 and 16. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Iterator combinators, generic function instantiation, and method values are annotated as pending their owning changes.

### Function types and function values

```we
let f: fn(Int64) -> Int64 = square       // a monomorphic fn name is a
let nine = f(3)                          // value; the call is postfix
let eff: fn(Int64) io -> Int64 = fetch   // effect segment: bare tags,
                                         // chapter 16's ratification

fn apply(f: fn(Int64) -> Int64, x: Int64) -> Int64 {
    f(x)                                 // the parameter's call
}

let make = fn() -> Bool { ready() }      // zero-parameter, valueless
let side = fn(s: String) { put(s) }      // no return type: fn(String) -> ()
```

### Closures

```we
let inc = |x: Int64| x + 1               // annotated: self-contained
let f: fn(Int64) -> Int64 = |x| x * 2    // bare: types from the annotation
let out = apply(|x| x + 1, 3)            // bare: types from the parameter

let get = fn(m: Option<Int64>) -> Int64 {
    return match m {                     // return exits the closure
        Some(n) => n
        None => 0
    }
}

var c = Counter { n: 0 }                 // gc var: captured live
let bump = fn() { c = Counter { n: c.n + 1 } }  // zero-parameter: the
bump()                                   // full form; there is no || form
```

### Captures answer the ownership categories

```we
let text = "fixed"                       // base type: copy at creation
let show = fn() { put(text) }            // zero-parameter: full form only

var p = Point { x: 1, y: 2 }             // byval: snapshot at creation
let read = fn() -> Int64 { p.x }
p = Point { x: 9, y: 9 }
// read() still sees 1: the capture is the creation-time copy
```

### Rejected forms

```we
let g = |x| x + 1                        // E1001: bare-parameter closure
                                        // without an expected function type
let h = pairUp                           // E1004: generic function name
                                        // in value position
let z = || ready()                       // E0105: unexpected token; ||
                                        // is logical-or, a zero-parameter
                                        // closure is fn() -> ...
let m = user.describe                    // E0105: unexpected token; a
                                        // method value is not ratified
let bad = fn(Int64)                      // E0105: unexpected token; the
                                        // arrow and return type are
                                        // required
let one = 1 + |x| x                      // E0105: unexpected token; write
                                        // 1 + (|x| x)
|x| x + 1                                // E0102: statement begins with a
                                        // continuation token

fn over(log: LogFile) -> fn() -> Int64 { // E1002: resource binding
    fn() -> Int64 { log.lines() }        // captured by a closure
}

var n = 0
let step = fn() { n = n + 1 }            // E1003: assignment to a captured
                                        // value-category binding
let g2 = Circle                          // E0704: payloaded variant
                                        // constructor used without
                                        // arguments
```

### Pending later changes

```we
// Iterator combinators arrive with their owning change as default
// methods of Iterator; generic instantiation `map<Int64>` and generic
// methods with chapter 10's own amendment; method values, if ever,
// with this chapter's amendment:
//
// let out = names.iterator()
//     .filter(|n| n.size() > 2)
//     .map(|n| n.toUpper())
//     .collect()
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| function type | 函数类型 |
| function value | 函数值 |
| closure | 闭包 |
| full closure form | 完整闭包形式 |
| short closure form | 短闭包形式 |
| bare parameter | 裸参数 |
| annotated parameter | 标注参数 |
| expected function type | 期望函数类型 |
| capture | 捕获 |
| capture discipline | 捕获纪律 |
| copy snapshot | 拷贝快照 |
| live binding | 活绑定 |
| traced cell | 追踪单元格 |
| escape | 逃逸 |
| monomorphic | 单态 |
| generic clause | 泛型子句 |
| method value | 方法值 |
| effect segment | 效应段 |
| block value | 块值 |
