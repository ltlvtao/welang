# We Language Specification — Chapter 3: Control Flow

### Requirement: If and else expressions

An if expression is `if cond block` or `if cond block else else-arm`, where `cond` is an expression and `else-arm` is a block or another if expression (chains terminate in a block or an if without else). An if without else produces no value and MUST NOT appear where a value is required (`E0202` — statically decidable from the missing else). An if with else is an expression form: on each evaluation its value is the taken arm's block value under chapter 2's Blocks and block value. The arms of an if-with-else MUST agree in type per chapter 8: an arm whose block has no value types as the unit type, and a disagreement is rejected under chapter 7's `E0501`; the never-type's exclusion from agreement is the sum-types chapter's. This chapter fixes the form and the arm-scoping. Each arm introduces its own scope: bindings inside an arm MUST NOT leak past the if. The condition's typing — that it must be the boolean type — is ratified by the types chapter.

#### Scenario: if with else used as a value

- **WHEN** `let x = if c { 1 } else { 2 }` is parsed and `c` holds true
- **THEN** `x` receives the then-arm's block value, here `1`; arm block values follow chapter 2 unchanged

#### Scenario: if without else in a value position

- **WHEN** `let x = if c { 1 }` appears — an if with no else used where a value is required
- **THEN** the compiler rejects it with `E0202:` valueless form in value position, naming the form that produces no value

#### Scenario: Arm bindings do not leak

- **WHEN** an arm binds a name, for example `if c { let a = 1 a } else { 0 }`
- **THEN** `a` is not visible after the if expression; each arm is its own scope

#### Scenario: else-if chain

- **WHEN** `if c1 { a } else if c2 { b } else { c }` is parsed
- **THEN** the else arm holds one nested if expression; the chain is two if expressions, each following this Requirement

#### Scenario: One arm values, one does not

- **WHEN** `if c { 1 } else { io.println("no") }` appears at an expression position
- **THEN** the compiler rejects it under `E0501` per chapter 8: the first arm is `Int64`, the second is the unit type; the arms must agree

### Requirement: While and loop

A while statement is `while cond block`; a loop statement is `loop block`. Both are statements: they produce no value and MUST NOT appear where a value is required (`E0202`). Their bodies follow chapter 2 blocks; body bindings do not leak past the statement. The condition's typing is ratified by the types chapter.

#### Scenario: while as a statement item

- **WHEN** `while more() { step() }` appears as a block item
- **THEN** it parses as a while statement; the body is a chapter 2 block

#### Scenario: loop in a value position

- **WHEN** `let x = loop { step() }` appears — a loop used where a value is required
- **THEN** the compiler rejects it with `E0202:` valueless form in value position

### Requirement: Break and continue

`break` and `continue` are bare statement forms: they carry no value and accept no label. They are legal only inside the body of a loop ratified by an owning chapter (while and loop here; the iteration chapter ratifies for). Outside such a body, the compiler rejects them with `E0201`. Neither form produces a value.

#### Scenario: break inside a loop

- **WHEN** `loop { if done() { break } step() }` is parsed
- **THEN** the break is legal: it exits the innermost enclosing loop

#### Scenario: continue outside any loop

- **WHEN** `continue` appears in a block that is not inside a loop body
- **THEN** the compiler rejects it with `E0201:` break or continue outside a loop

#### Scenario: Labelled break is not ratified

- **WHEN** `break 'outer` or any labelled break/continue form appears
- **THEN** the compiler rejects it as an unratified production (chapter 2's unexpected-token diagnostic); labels would enter only through a spec change

### Requirement: Return

`return` is a statement in two forms: bare `return` and `return expr`. This chapter ratifies the forms only: where return is legal (function bodies) and how its expression's type relates to the enclosing function is ratified by the declarations chapter, and the never-type interaction by the types chapter.

#### Scenario: return with an expression

- **WHEN** `return a + b` appears as an item
- **THEN** it parses as a return statement carrying one expression operand

#### Scenario: bare return

- **WHEN** `return` appears alone as an item
- **THEN** it parses as a bare return; its legality and typing follow the declarations chapter

### Requirement: Defer

A defer statement is `defer block` — the body MUST be a block; any other operand is rejected with `E0203`. Defer is legal only as a direct item of a function body's block; any other placement is rejected with `E0204` (function bodies are ratified by the declarations chapter; until then the rejectable side is what fires). Multiple defer statements in one function body execute in reverse order at function exit, including exits through early return or break. Defer shifts execution timing only; it is not the resource-safety guarantee — that is chapter 13's scope resource, whose scope-exit releases run before the enclosing function's own defers, the inner block exiting first. Error propagation inside a defer body is chapter 14's rule: the `?` operator is rejected there (`E1202`) — a defer body runs at exit and has no return to propagate to. Effect rules inside a defer body are chapter 16's: a defer body's calls count toward the enclosing function's declared effects (`E1401`), because defer shifts execution timing only, never effect attribution — defer introduces no exception to either.

#### Scenario: Defer body must be a block

- **WHEN** `defer cleanup()` appears — a defer whose operand is an expression, not a block
- **THEN** the compiler rejects it with `E0203:` defer body must be a block

#### Scenario: Defer inside a nested block

- **WHEN** `let x = { defer { log() } compute() }` appears — a defer inside a nested block expression rather than as a direct function-body item
- **THEN** the compiler rejects it with `E0204:` defer placement, naming the enclosing block that is not a function body

#### Scenario: Propagation inside a defer body

- **WHEN** `defer { readFile(path)? }` appears — the propagation postfix inside a defer body
- **THEN** the compiler rejects it with `E1202:` ? outside a function returning Result; a defer body runs at exit and has no return to propagate to

#### Scenario: Reverse order at exit

- **WHEN** a function body contains `defer { a() }` then later `defer { b() }` and the function exits
- **THEN** `b()` runs before `a()`; defers execute in reverse statement order

### Requirement: Control-flow diagnostics segment

The control-flow chapter owns registry segment `E0200`–`E0299` declared in `docs/spec/diagnostics.toml` `[segments]`. First allocations: `E0201` break or continue outside a loop, `E0202` valueless form in value position, `E0203` defer body must be a block, `E0204` defer placement. `E0200` and `E0205`–`E0299` are reserved. Trigger semantics live in this chapter's Requirements; entries live in the registry. Allocating further control-flow codes extends the registry in the same change.

#### Scenario: A control-flow code is emitted

- **WHEN** any `E02xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `0300-control-flow`

#### Scenario: A later change needs control-flow diagnostics

- **WHEN** a later chapter ratifies control-flow productions requiring new diagnostics
- **THEN** its change allocates numbers within `E0200`–`E0299` by extending the registry in the same change

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–3. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Function-body surface arrives with the declarations chapter; defer's positive form is annotated accordingly.

### if and else

```we
let x = if c { 1 } else { 2 }     // value: the taken arm's block value
let y = if c { 1 }                 // E0202: valueless form in value position

let z = if c {
    let a = 1
    a                              // then-arm value
} else {
    0
}
// a is NOT visible here: each arm is its own scope

let grade = if n >= 90 { "high" } else if n >= 50 { "mid" } else { "low" }
// else-if is an else arm holding one nested if
```

### while, loop, break, continue

```we
while more() { step() }            // statement item
let x = loop { step() }            // E0202: loops produce no value

var found: Option<Item> = None     // accumulator is the value channel
loop {
    let it = next()
    if it.isEnd() { break }        // bare break: exits innermost loop
    if !it.matches() { continue }
    found = Some(it)
    break
}

continue                           // E0201: break or continue outside a loop
```

### return

```we
return
return a + b
```

### defer

```we
defer cleanup()                    // E0203: defer body must be a block
let x = {
    defer { log() }                // E0204: defer placement, this block
    compute()                      // is not a function body
}

// positive form (function-body surface arrives with the declarations
// chapter):
//
// fn work() {
//     defer { close(f) }          // direct function-body item, OK
//     if bad() { return }         // early exit still runs defers
//     use(f)
// }                               // at exit: defers run in reverse order
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| if expression | if 表达式 |
| arm | 分支臂 |
| value position | 取值位置 |
| bare form | 裸形式 |
| reverse order | 逆序 |
| function body | 函数体 |
| statement family | 语句族 |
