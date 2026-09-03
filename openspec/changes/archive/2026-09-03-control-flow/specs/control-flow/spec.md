# Delta: control-flow

All Requirements in this capability are ADDED.

## ADDED Requirements

### Requirement: If and else expressions

An if expression is `if cond block` or `if cond block else else-arm`, where `cond` is an expression and `else-arm` is a block or another if expression (chains terminate in a block or an if without else). An if without else produces no value and MUST NOT appear where a value is required (`E0202` — statically decidable from the missing else). An if with else is an expression form: on each evaluation its value is the taken arm's block value under chapter 2's Blocks and block value. Rules for if-with-else whose arms do not agree in producing a value are ratified by the types chapter; this chapter fixes only the form and the arm-scoping. Each arm introduces its own scope: bindings inside an arm MUST NOT leak past the if. The condition's typing — that it must be the boolean type — is ratified by the types chapter.

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

A defer statement is `defer block` — the body MUST be a block; any other operand is rejected with `E0203`. Defer is legal only as a direct item of a function body's block; any other placement is rejected with `E0204` (function bodies are ratified by the declarations chapter; until then the rejectable side is what fires). Multiple defer statements in one function body execute in reverse order at function exit, including exits through early return or break. Defer shifts execution timing only; it is not the resource-safety guarantee, which the resource chapter ratifies separately. Error propagation and effect rules inside a defer body follow the error and effects chapters; defer introduces no exception to either.

#### Scenario: Defer body must be a block

- **WHEN** `defer cleanup()` appears — a defer whose operand is an expression, not a block
- **THEN** the compiler rejects it with `E0203:` defer body must be a block

#### Scenario: Defer inside a nested block

- **WHEN** `let x = { defer { log() } compute() }` appears — a defer inside a nested block expression rather than as a direct function-body item
- **THEN** the compiler rejects it with `E0204:` defer placement, naming the enclosing block that is not a function body

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
