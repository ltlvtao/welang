# We Language Specification — Chapter 14: The error mechanism


### Requirement: The Result type

The standard library declares the canonical result type as an ordinary generic sum under chapter 9's declaration forms: `pub type Result<T, E> = Ok(T) | Err(E)`, with `Ok` and `Err` ordinary PascalCase variant names in standard-library scope, like `Some` and `None`. Construction, patterns, and exhaustiveness are chapters 9 and 4's existing machinery; this chapter adds no matching rule. The error parameter `E` MUST be a named sum type — a sum type declared by name in the standard library or in user code. A base type (`String` included), a record, an interface, a `Dyn` box, or a generic parameter in the error position is rejected with `E1204:` Result error type is not a named sum type. The constraint is exhaustiveness's: a named sum's variant set is closed, so a match on a `Result` covers `Ok` and `Err` with all of `E`'s variants visible; the generic-parameter position admits no such closure, and this specification defines no is-a-sum bound. Discarding a `Result` value follows chapter 8's discard rule: a non-unit expression's value is not dropped silently, the spelling is `let _ = expr` (`E0605`), and no `.ignore()` method exists. A binding that is never used is not forced: there is no unused-binding check on `Result` or any other type.

#### Scenario: A Result matches exhaustively per existing machinery

- **WHEN** `r: Result<Int64, AppError>` is matched with `match r { Ok(n) => n, Err(e) => match e { ... } }`, the inner match covering every `AppError` variant
- **THEN** the match is exhaustive under chapter 9's generic-sum rules; no new matching rule of this chapter is invoked

#### Scenario: A base type in the error position is rejected

- **WHEN** `Result<Int64, String>` appears as a type expression
- **THEN** the compiler rejects it with `E1204:` Result error type is not a named sum type; `String` names no sum

#### Scenario: A generic parameter in the error position is rejected

- **WHEN** a function declares `fn run[E](r: Result<Int64, E>)` — the error position is a generic parameter
- **THEN** the compiler rejects it with `E1204:` Result error type is not a named sum type; a helper abstract over the error type is not expressible

#### Scenario: Discard is explicit at the drop site

- **WHEN** `readFile(path)` appears as an expression statement, or its value is bound and never used
- **THEN** the expression statement is rejected with chapter 8's `E0605`; the bound-and-unused form is accepted — forcing happens at the drop site only, and the spelling is `let _ = readFile(path)`

### Requirement: The propagation operator

The propagation operator is the postfix `expr?` — chapter 1's `?` token, granted grammar by chapter 2's postfix amendment. Its operand MUST be of a `Result<T, E_src>` type; an operand of any other type, `Option` included, is rejected with `E1201:` operand of ? is not a Result type. It is legal only in a live propagation context: the innermost enclosing function or full closure must declare a return type of the form `Result<U, E_dst>`, and a short closure's `?` fixes its inferred value type to `Result<U, E_dst>` accordingly — `?` returns from that innermost body itself, never from a function the closure is nested in or passed to. A defer body runs at exit and has no return to propagate to, and the module top level has no enclosing function: `?` there, and `?` in any body whose innermost function returns a non-Result type, is rejected with `E1202:` ? outside a function returning Result. `E_src` MUST be `E_dst` — the same named type — otherwise the compiler rejects with `E1203:` ? error type disagrees with the declared error type; there is no implicit wrapping or lifting (the single error mechanism): converting an error is explicit — a `match`, or a standard-library combinator when a future change ratifies one. On `Ok(t)` the expression's value is `t`, of type `T`; on `Err(e)` the enclosing function or closure returns `Err(e)` at that point — an early return under chapter 13's return channel, whose resource obligations travel as with any return. The postfix chains with member access and call (`f()?.name`, `g(x)?.next()`) and binds as tightly as any postfix under chapter 2's level 1.

#### Scenario: Ok unwraps to the payload

- **WHEN** `let n = readFile(path)?` is evaluated and `readFile` produced `Ok(42)` with payload type `Int64`
- **THEN** `n` is `42` of type `Int64`; execution continues at the next statement

#### Scenario: Err returns early from the context function

- **WHEN** the same statement is evaluated and `readFile` produced `Err(e)`
- **THEN** the enclosing function returns `Err(e)` at that point; no statement after the propagation runs

#### Scenario: A non-Result operand is rejected

- **WHEN** `let n = find(id)?` appears, `find` returning `Option<Int64>`, or `let m = 5?`
- **THEN** the compiler rejects it with `E1201:` operand of ? is not a Result type; `Option` does not propagate

#### Scenario: A non-Result function context is rejected

- **WHEN** a function declaring `-> Int64` contains `readFile(path)?`
- **THEN** the compiler rejects it with `E1202:` ? outside a function returning Result, naming the enclosing function

#### Scenario: Propagation at the module top level is rejected

- **WHEN** `readFile(path)?` appears at the module top level, outside any function body
- **THEN** the compiler rejects it with `E1202:` ? outside a function returning Result; no function is enclosing

#### Scenario: Propagation inside a defer body is rejected

- **WHEN** `defer { readFile(path)? }` appears — the propagation postfix inside a defer body
- **THEN** the compiler rejects it with `E1202:` ? outside a function returning Result; a defer body has no return to propagate to

#### Scenario: A closure propagates from itself

- **WHEN** `items.map(fn(x: String) -> Result<Int64, AppError> { parse(x)? })` appears inside a function returning `Int64`
- **THEN** the `?` is legal: its context is the closure, which declares a `Result` return; the closure returns from itself, not from the enclosing function

#### Scenario: A short closure's ? fixes its value type

- **WHEN** `let f = |s: String| parse(s)?` appears, `parse` of type `fn(String) -> Result<Int64, ParseError>`
- **THEN** the closure's value type is `fn(String) -> Result<Int64, ParseError>`: the `?` returns from the closure itself, and the top-level binding is legal because the innermost enclosing function of the `?` is the closure

#### Scenario: An error type disagreement is rejected

- **WHEN** a function declaring `-> Result<Int64, AppError>` contains `parse(s)?` with `parse` returning `Result<Int64, ParseError>`, `AppError` and `ParseError` distinct named sums
- **THEN** the compiler rejects it with `E1203:` ? error type disagrees with the declared error type; the conversion is a `match`, written explicitly

#### Scenario: The postfix chains tightly

- **WHEN** `f()?.name` is parsed
- **THEN** it groups as `(f()?).name` under chapter 2's level 1 — the propagation postfix binds as tightly as call and member access, and member access lands on the payload

### Requirement: The panic family

The standard library declares the termination forms as ordinary functions: `pub fn panic(msg: String) -> Never`, `pub fn todo(msg: String) -> Never`, and `pub fn assert(cond: Bool, msg: String)`. No keyword and no grammar production is added: they are called like any function, and a call's argument list is checked as any call's (chapters 6, 7, and 10); the message argument is required by the declaration — a termination without an explanation is unauditable, and no dedicated diagnostic is introduced for the missing message. `panic` and `todo` produce `Never` (chapter 9's bottom type): a call satisfies any declared return type, and control never continues past it. `todo` is `panic` with intent carried in the message — not-yet-written code marks itself and stays auditable. `assert` evaluates its condition first: on `true` it produces the unit value and execution continues; on `false` it is a `panic` carrying the message. Nothing catches a panic: no `catch`, `recover`, or `try` form exists in the grammar, and none may enter without amending this requirement — errors propagate through `Result` alone (chapter 0, Principle 9); a panic is termination, not an error channel. A runtime integer overflow (chapter 7) traps as a `panic` naming the operation — `Int64 add overflow` and siblings — the checked-trap construct chapter 7 deferred to this chapter.

#### Scenario: panic produces Never

- **WHEN** `fn find(id: Int64) -> Int64 { if id == 0 { panic("no such id") } else { id } }` is declared
- **THEN** the `panic` call satisfies the `Int64` return type per chapter 9's bottom-type rule; control never continues past the call

#### Scenario: todo marks not-yet-written code

- **WHEN** `todo("parsing not written yet")` appears in a returning position of a function declaring `-> Int64`
- **THEN** the code checks — the call is `Never`-typed — and at run time it terminates carrying the message

#### Scenario: assert passes through on true

- **WHEN** `assert(n > 0, "n must be positive")` is evaluated with `n == 5`
- **THEN** the call produces the unit value and execution continues at the next statement

#### Scenario: assert terminates on false

- **WHEN** the same call is evaluated with `n == 0`
- **THEN** it is a `panic` carrying the message `"n must be positive"`; control never continues past the call

#### Scenario: Runtime overflow traps as a named panic

- **WHEN** an `Int64` addition overflows at run time
- **THEN** the operation traps as a `panic` naming the operation, never producing a wrapped value — chapter 7's checked-trap construct, named by this chapter

#### Scenario: No catching form exists

- **WHEN** `catch`, `recover`, or `try` appears as a form
- **THEN** the compiler rejects it with chapter 2's unexpected-token diagnostic (`E0105`); no such production exists, and none may enter without amending this requirement

### Requirement: Unwinding

A panic in flight unwinds the call stack: every block in progress exits as a block exit. Scope-resource blocks release per chapter 13's guarantee — exactly one `release` call per binding, in reverse declaration order — and a function's defers run in reverse statement order after the inner blocks' releases, the inner block exiting first; the ordering is the one chapter 13 fixed for normal exits, extended to unwound exits by this requirement. Resources transfer nothing during unwinding: the releases are the scope-exit machinery's alone (chapter 13's single release trigger holds on every exit kind). When no frames remain, the process aborts with the panic's message; the abort is the capture boundary this specification defines — no expression of the language observes a panic. The concurrency chapter binds task-internal panics to this same mechanism (chapter 0, Principle 9): inside the task, unwinding runs this requirement's ordering — scope releases, then defers — and the task boundary is the second capture boundary this specification defines, where the unwinding panic becomes the `Err` a handle's `await` yields; no second channel enters. The testing chapter's amendment adds the third: inside a test block the flight ends at the test boundary — that test fails, the process does not abort — chapter 20's requirement carries the full rule.

#### Scenario: Unwinding releases resources

- **WHEN** a panic fires while a scope resource statement binding `a` then `b` is in progress
- **THEN** `b`'s release runs, then `a`'s — reverse declaration order — before the enclosing function's own defers run

#### Scenario: Defers run on the way out

- **WHEN** a function body contains `defer { a() }` then `defer { b() }` and a panic fires inside
- **THEN** after the inner blocks' releases, `b()` runs, then `a()` — reverse statement order, the same ordering as a normal exit

#### Scenario: The abort is the capture boundary

- **WHEN** unwinding completes with no frames remaining
- **THEN** the process aborts with the panic's message; no handler exists and none may be written without amending the panic family requirement

#### Scenario: A task panic unwinds then converts

- **WHEN** a panic fires inside a task body
- **THEN** the task's own blocks release and its defers run under this requirement's ordering, and at the task boundary the flight converts into the `Err` the handle's `await` yields — chapter 18's requirement carries the full rule

#### Scenario: A test panic unwinds then fails the test

- **WHEN** a panic fires inside a test block
- **THEN** the test's own blocks release and its defers run under this requirement's ordering, and at the test boundary the flight converts into that test's failure — chapter 20's requirement carries the full rule

### Requirement: The single error mechanism

Errors flow through exactly one mechanism: `Result` values declared in signatures, propagated by `?`, consumed by `match` — chapter 0, Principle 9 operationalized. This chapter fixes the consequences. A function that can fail says so in its return type: there is no throws clause, no effect-segment escape — chapter 16's tags track environmental effects, not failure — and no unchecked channel. `?` is the only propagation operator: no other postfix or prefix form propagates — `expr!` and `try expr` fit no production of chapter 2's grammar. A failure crossing a function boundary is a value in a signature, so every path of failure is visible at the call site and the discipline is locally decidable (chapter 0, Principle 1). A panic is termination, not an error channel: it carries no payload type, matches nothing, and is caught by nothing.

#### Scenario: No other propagation spelling exists

- **WHEN** `expr!` or `try f()` appears at an expression position
- **THEN** the compiler rejects it with chapter 2's unexpected-token diagnostic (`E0105`); the propagation postfix is `?` alone

#### Scenario: Failure is visible in the signature

- **WHEN** a caller sees `fn parse(s: String) -> Result<Config, ParseError>`
- **THEN** every failure path is declared in the signature; nothing propagates implicitly, and the caller's handling is checkable at the call site alone

### Requirement: Errors diagnostics segment

The errors chapter owns registry segment `E1200`–`E1299`, declared under `[segments]` in `docs/spec/diagnostics.toml` with owner `1400-errors`. Allocations: `E1201` operand of ? is not a Result type, `E1202` ? outside a function returning Result, `E1203` ? error type disagrees with the declared error type, `E1204` Result error type is not a named sum type. `E1200` and `E1205`–`E1299` are reserved for this chapter's amendments — later chapters needing segment codes claim their own segments.

#### Scenario: The segment is retrievable

- **WHEN** a diagnostics consumer looks up any `E12xx` code in the registry
- **THEN** the segment entry names owner `1400-errors`, and each allocated code's entry carries severity, title, description, remediation, owner, requirement, and allocated date

#### Scenario: A later change extends within the segment

- **WHEN** a later spec-layer change of this chapter needs a new code
- **THEN** it allocates from `E1200`–`E1299` within its own change; chapters outside errors claim other segments

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–13. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits.

### Result and its error position

```we
type AppError = NotFound(String) | Denied(String)   // a named sum: a legal
                                                    // error parameter
type Lookup = Result<Int64, AppError>               // Ok(Int64) | Err(AppError)

let bad: Result<Int64, String> = make()
// E1204: Result error type is not a named sum type — String names no sum

fn run[E](r: Result<Int64, E>) { }
// E1204: Result error type is not a named sum type — the generic parameter
// admits no closure either
```

### Propagation with ?

```we
fn readConfig(path: String) -> Result<String, AppError> {
    let text = readFile(path)?             // Err(AppError) returns here on
    return Ok(text)                        // error; Ok unwraps to String
}

fn lenOf(path: String) -> Result<Int64, ParseError> {
    let text = readFile(path)?
    // E1203: ? error type disagrees with the declared error type — convert
    // by match, written explicitly
    return Ok(text.len())
}
```

### Context rules

```we
fn firstLine(s: String) -> Int64 {
    let text = parse(s)?
    // E1202: ? outside a function returning Result — the context function
    // returns Int64
    return text.len()
}

fn guard() {
    defer { readFile("a")? }
    // E1202: ? outside a function returning Result — a defer body runs at
    // exit; no return to propagate to
}

fn deep(id: Int64) -> Int64 {
    let n = find(id)?
    // E1201: operand of ? is not a Result type — Option does not propagate
    return n
}
```

### The panic family and unwinding

```we
fn find(id: Int64) -> Int64 {
    if id == 0 { panic("no such id") }     // Never: satisfies Int64
    return lookup(id)
}

fn half(m: Int64) -> Int64 {
    todo("half not written yet")           // panic with intent in the
}                                          // message; checks as Never

fn port(n: Int64) -> Int64 {
    assert(n > 0, "n must be positive")    // true: unit, continue;
    return n / 2                           // false: panic with the message
}

fn counted() {
    scope resource(a = openFile("a"), b = openFile("b")) {
        panic("boom")                      // unwinds: b released, then a,
    }                                      // then this function's defers,
    defer { log() }                        // then the abort — no handler
}
```

### Single mechanism

```we
let v = try parse("1")
// E0105: unexpected token — no try form; ? is the only propagation operator

let w = parse("1")!
// E0105: unexpected token — no ! postfix; the propagation postfix is ? alone
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| error | 错误 |
| error mechanism | 错误机制 |
| error type | 错误类型 |
| result type | 结果类型 |
| propagation | 传播 |
| propagation operator | 传播运算符 |
| propagation context | 传播上下文 |
| payload | 载荷 |
| early return | 早返回 |
| termination | 终止 |
| panic family | panic 家族 |
| unwinding | 展开 |
| block exit | 块出口 |
| capture boundary | 捕获边界 |
| abort | 中止 |
| assertion | 断言 |
| named sum type | 命名 sum 类型 |
| exhaustiveness | 穷尽性 |
| lifting | 提升 |
| discard | 丢弃 |
| single error mechanism | 单一错误机制 |
