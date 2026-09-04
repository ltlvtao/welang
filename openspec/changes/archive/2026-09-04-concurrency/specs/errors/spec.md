## MODIFIED Requirements

### Requirement: Unwinding

A panic in flight unwinds the call stack: every block in progress exits as a block exit. Scope-resource blocks release per chapter 13's guarantee — exactly one `release` call per binding, in reverse declaration order — and a function's defers run in reverse statement order after the inner blocks' releases, the inner block exiting first; the ordering is the one chapter 13 fixed for normal exits, extended to unwound exits by this requirement. Resources transfer nothing during unwinding: the releases are the scope-exit machinery's alone (chapter 13's single release trigger holds on every exit kind). When no frames remain, the process aborts with the panic's message; the abort is the capture boundary this specification defines — no expression of the language observes a panic. The concurrency chapter binds task-internal panics to this same mechanism (chapter 0, Principle 9): inside the task, unwinding runs this requirement's ordering — scope releases, then defers — and the task boundary is the second capture boundary this specification defines, where the unwinding panic becomes the `Err` a handle's `await` yields; no second channel enters.

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
