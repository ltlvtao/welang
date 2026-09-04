## MODIFIED Requirements

### Requirement: Capture discipline

A closure is of the gc category — one category, reference semantics: storable, passable, returnable, and escaping; a value-category closure would need escape analysis this specification does not ratify. Captures answer chapter 8's ownership categories. A binding of a gc-category type is captured by reference: reads and assignments inside the body see and mutate the live binding, the captured cell being traced and sound to escape. A binding of a value-category type or of a base type is captured by copy at closure creation: the body reads the value as it was then, later mutations of the outer binding are invisible, and an assignment inside the body to such a captured binding is rejected with `E1003` — the snapshot is frozen; a local binding carries the intermediate value instead. A resource-category binding MUST NOT be captured — `E1002:` resource binding captured by a closure: a closure outlives its creation site by construction, and a handle escaping the single deterministic release point a resource's discipline depends on is not honest; the resource's contents materialize first and the closure captures the materialized value. This requirement governs closures — synchronous function values that run inside their creator's own extent; the task block, whose body runs concurrently with its creator, carries its own capture discipline in chapter 18 — synchronized gc types where this requirement admits gc by reference, no var capture where this requirement permits the live kind.

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

#### Scenario: A task block is not a closure

- **WHEN** a task block's body reads an enclosing binding
- **THEN** chapter 18's task capture discipline governs the read, not this requirement's; this requirement binds the closure forms of this chapter alone

### Requirement: Closure bodies are function bodies

A closure's body block is a function body: its final expression is its return value under chapter 2's block value; `return` returns from the closure, never from an enclosing named function — a control-flow target is always locally decidable, the enclosing call chain never consulted; early `return` from nested blocks inside the body is legal under chapter 6's model; and a `defer` inside a closure body runs at the closure body's exit under chapter 3. The function-body contexts of `E0401` are chapter 6's named fns, this chapter's closures, and — by the concurrency chapter's amendment — that chapter's task bodies; the module top level and defer bodies remain returnless. A closure is itself a value like any other — it may be bound, passed, returned from its enclosing function, and called later, its captures keeping the reach they were created with (Capture discipline).

#### Scenario: return exits the closure only

- **WHEN** a fn holds `let get = fn(m: Option<Int64>) -> Int64 { return match m { Some(n) => n None => 0 } }` and calls `get(Some(3))`
- **THEN** the `return` carries the closure's value; the enclosing fn's control flow is untouched, its own `return` still required to carry its own value

#### Scenario: defer runs at the closure body's exit

- **WHEN** a closure body holds `defer { done() }` before its final expression
- **THEN** `done()` runs when the closure body exits — per call — under chapter 3's LIFO rule

#### Scenario: A closure escapes its enclosing fn

- **WHEN** a fn returns a closure capturing a gc-category binding and the caller invokes the returned closure after the fn has returned
- **THEN** the closure is callable and sees its captures per Capture discipline; gc means the traced cell outlives the frame

#### Scenario: A task body is the third function-body context

- **WHEN** a task block's body holds an early `return` of its value, or a `defer` at its top level
- **THEN** both are legal under this requirement's amended enumeration and the concurrency chapter's own rule — `return` carries the task's value, the defer runs at the body's exit
