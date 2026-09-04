## MODIFIED Requirements

### Requirement: Closure bodies are function bodies

A closure's body block is a function body: its final expression is its return value under chapter 2's block value; `return` returns from the closure, never from an enclosing named function — a control-flow target is always locally decidable, the enclosing call chain never consulted; early `return` from nested blocks inside the body is legal under chapter 6's model; and a `defer` inside a closure body runs at the closure body's exit under chapter 3. The function-body contexts of `E0401` are chapter 6's named fns, this chapter's closures, — by the concurrency chapter's amendment — that chapter's task bodies, and — by the testing chapter's amendment — that chapter's test and mock bodies; the module top level and defer bodies remain returnless. A closure is itself a value like any other — it may be bound, passed, returned from its enclosing function, and called later, its captures keeping the reach they were created with (Capture discipline).

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

#### Scenario: Test and mock bodies are function-body contexts

- **WHEN** a test block's body holds an early bare `return` ending the test, and a mock declaration's body holds `return` with the target's declared value
- **THEN** both are legal under this requirement's amended enumeration — the test produces no value so bare `return` is its form, and the mock answers its target's signature; `return expr` inside a test body is `E0402` as in any valueless function
