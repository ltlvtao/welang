# Defer — MODIFIED Requirements

## MODIFIED Requirements

### Requirement: Defer

A defer statement is `defer block` — the body MUST be a block; any other operand is rejected with `E0203`. Defer is legal only as a direct item of a function body's block; any other placement is rejected with `E0204` (function bodies are ratified by the declarations chapter; until then the rejectable side is what fires). Multiple defer statements in one function body execute in reverse order at function exit, including exits through early return or break. Defer shifts execution timing only; it is not the resource-safety guarantee — that is chapter 13's scope resource, whose scope-exit releases run before the enclosing function's own defers, the inner block exiting first. Error propagation and effect rules inside a defer body follow the error and effects chapters; defer introduces no exception to either.

#### Scenario: Defer body must be a block

- **WHEN** `defer cleanup()` appears — a defer whose operand is an expression, not a block
- **THEN** the compiler rejects it with `E0203:` defer body must be a block

#### Scenario: Defer inside a nested block

- **WHEN** `let x = { defer { log() } compute() }` appears — a defer inside a nested block expression rather than as a direct function-body item
- **THEN** the compiler rejects it with `E0204:` defer placement, naming the enclosing block that is not a function body

#### Scenario: Reverse order at exit

- **WHEN** a function body contains `defer { a() }` then later `defer { b() }` and the function exits
- **THEN** `b()` runs before `a()`; defers execute in reverse statement order
