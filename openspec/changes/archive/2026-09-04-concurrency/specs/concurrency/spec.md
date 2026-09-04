## ADDED Requirements

### Requirement: The shared-state types

The standard library, in its `std.concurrent` module, declares four shared-state types — `Mutex<T>`, `RwLock<T>`, `Atomic<T>`, and `AtomicRef<T>` — each generic with one parameter under chapter 10's forms, each a gc-category value under chapter 8's aliasing: shared by reference across tasks, never copied. Each is constructed exactly one way, by its own name applied to an initial value — `Mutex(0)`, `RwLock(text)`, `Atomic(true)`, `AtomicRef(defaultConfig)` — and the name is the whole contract: which synchronization discipline a value carries is read from its type, never from a construction mode behind one name (the adjudicated split; v0.8's single `Shared<T>` with method sets varying by construction is not inherited, its checking not locally decidable across a function boundary). All four share one method trio — `fn update(mut self, f: fn(T) -> T) -> T`, a read-modify-write holding the value's discipline, returning the new value; `fn get(self) -> T`, a read; `fn set(mut self, v: T)`, a blind write — and `RwLock<T>` adds `fn read<U>(self, f: fn(T) -> U) -> U`, a read under a lock shared with other concurrent readers, its `U` determined from the call's own text under chapter 10's single-direction rule (`E0827` when nothing determines it). The callback parameters are pure function types — `fn(T) -> T` and `fn(T) -> U` with no effect segment — so a closure performing effects does not fit, rejected with chapter 16's `E1402` at the argument agreement position; there is no shared-state-specific purity code, the combinator precedent carrying here. `Atomic<T>`'s parameter MUST be an atomic base type — one of chapter 7's eight integer types, `Bool`, `Float32`, or `Float64` — any other argument rejected with `E1614:` Atomic type argument is not an atomic base type; `AtomicRef<T>`'s parameter MUST be a gc-category record type, any other rejected with `E1615:` AtomicRef type argument is not a gc record type. The types' waiting is not an effect: waiting on these primitives carries no effect tag, chapter 16's carve-out with this change's amendment. The module's names are reached by import under chapter 15 — `import std.concurrent` introduces the name `concurrent`, through which `concurrent.Mutex` and its kin qualify; this chapter's Requirements and Scenarios write the names bare for reading, the qualified spellings being the reachable ones.

#### Scenario: The four names carry their discipline

- **WHEN** with `import std.concurrent` in scope, annotations hold `concurrent.Mutex<Int64>`, `concurrent.RwLock<Map<String, User>>`, `concurrent.Atomic<Bool>`, and `concurrent.AtomicRef<Config>` with `Config` a gc record
- **THEN** each resolves to the declared type through the import's name, and each method call on a value is checked against that type's own method set alone

#### Scenario: update holds the discipline for the whole read-modify-write

- **WHEN** two tasks call `counter.update(|v| v + 1)` on one `Mutex<Int64>` and both calls complete
- **THEN** the counter is exactly two higher: each `f` ran between its own acquire and release, no interleaving inside the callback

#### Scenario: RwLock readers share, writers exclude

- **WHEN** several tasks hold `cache.read(|m| m.get("key"))` concurrently on one `RwLock<Map<String, User>>`
- **THEN** the reads may proceed together — the read lock is shared — while a `set` or `update` excludes every reader for its duration

#### Scenario: An impure callback does not fit

- **WHEN** `counter.update(|v| save(v))` appears and `save` declares `effect io`
- **THEN** the compiler rejects it with `E1402:` function value effect set does not match the expected type's — `f`'s parameter type is a pure function type; perform the effect on the returned value, outside the callback

#### Scenario: Atomic rejects a non-atomic parameter

- **WHEN** `Atomic("name")` or `Atomic(users)` appears with `users: List<User>`
- **THEN** the compiler rejects it with `E1614:` Atomic type argument is not an atomic base type; wrap the composite in `Mutex` or `RwLock` instead

#### Scenario: AtomicRef rejects a non-record parameter

- **WHEN** `AtomicRef(3)` appears
- **THEN** the compiler rejects it with `E1615:` AtomicRef type argument is not a gc record type; `Atomic` is the type for atomic base values

### Requirement: Nested access to one shared value is rejected

A callback running under one shared value's discipline — the `f` of `update` or `read`, on that same value — MUST NOT directly access that value again: a call to `update`, `get`, `set`, or `read` on the same binding inside its own callback is rejected with `E1613:` nested access to one shared value within its own callback. The check is direct and per binding: it sees calls written on the same binding in the callback body, and it does not follow calls into other functions — an indirect nested access through a helper function is not caught here, a boundary this specification states rather than hides; the vet-tier heuristic survey is the tooling chapter's, not this layer's. A callback MAY access other shared values — nesting across distinct bindings is legal, lock ordering being the programmer's discipline.

#### Scenario: A direct nested access is rejected

- **WHEN** `m.update(|v| m.get() + v)` appears on a `Mutex<Int64>` binding `m`
- **THEN** the compiler rejects it with `E1613:` nested access to one shared value within its own callback; the callback already holds the value — `v` is it

#### Scenario: A distinct binding nests legally

- **WHEN** `a.update(|x| x + b.get())` appears with `a` and `b` distinct shared values
- **THEN** the access is legal; cross-value ordering is the programmer's discipline, not this rule's

### Requirement: The Cond type

The `std.concurrent` module declares `Cond<T>`, a condition variable bound to one `Mutex<T>` at construction — `Cond(m)` with `m: Mutex<T>` is the one constructor, and a non-`Mutex` argument is rejected under chapter 7's `E0501` at the argument position: the pairing with `RwLock` that v0.8 allowed is not inherited, condition variables on read-write locks being notoriously error-prone and the single pairing being the adjudicated surface. `Cond<T>` is a gc-category value sharing the mutex's lifetime, safe to pass across tasks. Its methods: `fn wait(mut self, f: fn(T) -> Bool)` — release the bound mutex's discipline, block the calling task, and on each wake re-acquire and re-check `f`, returning once `f` holds on the re-acquired value; the release-wait-reacquire-recheck loop is the operation's fixed semantics, spurious wakes invisible to the caller. `fn signal(mut self)` wakes one waiting task if any; `fn broadcast(mut self)` wakes all. Waking is a hint, not a guarantee: a woken task re-checks the predicate before returning from its own `wait`. The predicate is a pure function type `fn(T) -> Bool`; an effectful closure is `E1402` at the argument position, the shared-state purity rule carrying here unchanged. Waiting carries no effect tag (chapter 16's carve-out).

#### Scenario: wait blocks until the predicate holds

- **WHEN** a consumer task holds `notEmpty.wait(|q| q.size() > 0)` on a `Cond<List<Job>>` bound to a mutex-held queue that is empty, and a producer then adds a job and calls `notEmpty.signal()`
- **THEN** the consumer's `wait` returns after its re-acquired check sees the queue non-empty; it cannot return while the predicate fails

#### Scenario: Construction binds a mutex by type

- **WHEN** `Cond(cache)` appears with `cache: RwLock<Map<String, User>>`
- **THEN** the compiler rejects it under `E0501` at the argument position — `Cond`'s parameter type is `Mutex<T>`, and the rwlock pairing is not a surface of this language

#### Scenario: An impure predicate does not fit

- **WHEN** `notEmpty.wait(|q| log(q) && q.size() > 0)` appears and `log` declares `effect io`
- **THEN** the compiler rejects it with `E1402:` function value effect set does not match the expected type's; check the effect after `wait` returns

#### Scenario: A woken task re-checks

- **WHEN** a task wakes from `wait` and another task has meanwhile taken the last element
- **THEN** the re-acquired check sees the predicate false and the task waits again; the wake was a hint and the loop is `wait`'s own

### Requirement: The Semaphore type

The `std.concurrent` module declares `Semaphore`, a counting semaphore limiting concurrent access to a resource: `Semaphore(n)` with `n: Int64` constructs one holding `n` permits. `n` MUST be a positive integer: a count that is constant-evaluable and not positive is a compile error under `E1616:` Semaphore count is not a positive integer (chapter 7's overflow precedent — what the compiler can decide, it decides); a non-constant count that is zero or negative at runtime is a checked trap of chapter 14's family. The methods: `fn acquire(mut self)` — take one permit, blocking while none is free; `fn tryAcquire(mut self) -> Bool` — take one permit without blocking, `false` when none is free; `fn release(mut self)` — return one permit; releasing past the constructed count is a runtime panic of chapter 14's family, a logic error surfaced early rather than silently absorbed; `fn currentCount(self) -> Int64` — the momentary count, for inspection only, stale the moment it returns. `Semaphore` is a gc-category value, safe across tasks. Waiting carries no effect tag (chapter 16's carve-out). The timeout route is the scope's, not the semaphore's: an `acquire` with a deadline is `scope timeout(...)` around it, one waiting path per primitive (v0.8's stance inherited).

#### Scenario: acquire limits concurrency

- **WHEN** eleven tasks each hold `dbLimiter.acquire()` then `defer { dbLimiter.release() }` around a query, on a `Semaphore(10)`
- **THEN** at most ten queries run at once; the eleventh `acquire` blocks until a `release` returns a permit

#### Scenario: tryAcquire does not block

- **WHEN** `tryAcquire()` is called with all permits taken
- **THEN** it returns `false` immediately; the caller keeps its own course of action

#### Scenario: A constant non-positive count is a compile error

- **WHEN** `Semaphore(0)` or `Semaphore(-3)` appears
- **THEN** the compiler rejects it with `E1616:` Semaphore count is not a positive integer

#### Scenario: Releasing past the count panics

- **WHEN** `release()` runs on a `Semaphore(2)` whose two permits are already free
- **THEN** the call panics at runtime under chapter 14's family — more releases than acquires is a logic error, surfaced not absorbed

### Requirement: Task blocks

A task block is `task effect tag1 tag2 ... block` — the `task` keyword (chapter 1 as amended), an effect segment of the chapter 16 spelling (`effect` followed by one or more tags), and a chapter 2 block. It is a keyword-led expression form under chapter 2's skeleton amendment. The segment is REQUIRED: a task block runs in its own task, not in the enclosing function's extent, so its effects have no enclosing declaration to ride — the block declares its own, and omission is rejected with `E1601:` task block without an effect segment. Within the block's body every call's effect set MUST be a subset of the declared set, chapter 16's `E1401` ruling inside the task's own extent; creating the task — evaluating the block expression itself — performs no calls, only the capture copies of Task capture discipline, so the enclosing function owes no effect for the creation. The task block's value is a `TaskHandle<T>` — a gc-category handle from `std.concurrent` — where `T` is the body's block value type under chapter 2, the unit type for a valueless body; `T` MUST NOT be a resource type: the body's value travels into a handle, a composite position under chapter 13's `E1106`. A task block MUST be lexically inside some scope block's body — a handle with no scope to join it fits no production's intent, rejected with `E1618:` task block outside any scope block; a task body may itself hold scope blocks, nesting concurrency inside the task. The task body is a function-body context like a closure's: `return` carries the task's value early, a `defer` inside it runs at its exit under chapter 3, and the function-body contexts of `E0401` extend to it by chapter 12's amended enumeration. The handle's methods: `fn await(mut self) -> Result<T, TaskPanic>` — wait for the task's completion and yield its value, a panic inside the task surfacing as `Err(TaskPanic)` per Panics at the task boundary; `fn cancel(mut self)` — request cancellation of the task per CancelSignal and currentCancelSignal, without waiting. Each is one-shot: awaiting or cancelling consumes the handle, and the handle-discipline rule of Scope blocks fixes the rest. Both perform waiting or coordination only — no effect tag (chapter 16's carve-out).

#### Scenario: A task declares its own effects

- **WHEN** `let t = task effect net { fetchA() }` appears inside a scope block with `fetchA` declaring `effect net`
- **THEN** the task block parses, its handle binds, and the enclosing function's own segment is untouched — the body's effects answer the task's declaration, not any enclosing one

#### Scenario: A task block without a segment is rejected

- **WHEN** `task { fetchA() }` appears
- **THEN** the compiler rejects it with `E1601:` task block without an effect segment; a task runs outside every enclosing extent and declares its own effects

#### Scenario: An effect beyond the segment is rejected inside the task

- **WHEN** `task effect net { save("x") }` appears with `save` declaring `effect io`
- **THEN** the compiler rejects it with `E1401:` undeclared effect at a call — io is not in the task's declared set

#### Scenario: A task outside any scope is rejected

- **WHEN** a task block appears in a function body with no enclosing scope block
- **THEN** the compiler rejects it with `E1618:` task block outside any scope block; the handle must join a scope, and none is in scope

#### Scenario: A resource-typed task body is rejected

- **WHEN** a task block's body's final expression is a `byres` record value
- **THEN** the compiler rejects it under chapter 13's `E1106` — the value would travel into `TaskHandle<T>`, a composite position; materialize the contents first

#### Scenario: await yields the body's value

- **WHEN** `let t = task effect net { fetchA() }` runs to completion and `t.await()` is called
- **THEN** the call yields `Ok(a)` with `a` the body's value; `t` is consumed and no second `await` or `cancel` is legal on it

### Requirement: Task capture discipline

A task block's captures answer chapter 8's ownership categories under a discipline of their own: a task runs concurrently with its creator, so a capture is a channel across which races are possible, and the rules are the adjudicated closure of that fact. Closures proper — chapter 12's forms — stay under chapter 12's synchronous discipline; this requirement governs task blocks alone, and chapter 12's amendment in this change says so in one sentence. A binding of a base type, the unit type, or a value-category type — value records, value sums, tuples — is captured by copy at task creation: an immutable snapshot of the shape, safe by construction, a gc element inside it being a reference whose own synchronization discipline travels with it. A `var` binding MUST NOT be captured, whatever its type — rejected with `E1603:` task block captures a var binding — the pair of mutability and concurrency being a race by definition; the value goes in a shared-state type or is copied to an immutable binding first. A binding of a gc-category type MUST be of a synchronized type — one of `Mutex`, `RwLock`, `Atomic`, `AtomicRef`, `Cond`, `Semaphore`, `Channel`, `SendOnly`, `ReceiveOnly`, `TaskHandle`, `CancelSignal` — when captured; any other gc type, the collections included, is rejected with `E1602:` task block captures unsynchronized gc state, and the same code rejects a value-category binding whose parts hold gc state the Shareable set excludes — a record with a `List` field, a tuple with a `List` element: unsynchronized gc state crossing a task boundary is a data race, whatever shape carries it across, and the synchronized types are the ones whose methods make sharing safe. A resource-category binding MAY be captured exactly when its type declares no `mut self` method besides `release` — every method a task can race on is a read — the scan being of the type's declared method signatures, locally decidable; a type declaring any other `mut self` method is rejected with `E1604:` task block captures a resource whose type declares mut self methods. The capture transfers nothing of the resource's release obligation: `release` stays the scope-exit machinery's single trigger under chapter 13, the tasks' handles being references, not ownership; the scope block that joins the tasks sits inside the resource's own scope, or the binding would not be in scope at all. A binding whose type mentions a generic parameter of the enclosing declaration MAY be captured only when that parameter carries a `Shareable` bound — rejected otherwise with `E1605:` task block captures a generic-parameter binding without a Shareable bound; resources never reach generic positions under chapter 13's `E1106`, so the bound never faces a resource.

#### Scenario: A base-type capture is a copy

- **WHEN** a task block captures `let limit: Int64 = 10` and the enclosing binding is rebound after the task's creation
- **THEN** the task reads `10` for its whole life; the capture is the creation-time copy

#### Scenario: A var capture is rejected

- **WHEN** `var total = 0` is in scope and a task block reads `total`
- **THEN** the compiler rejects it with `E1603:` task block captures a var binding; carry the state in a `Mutex` and read it through `get`

#### Scenario: A bare gc capture is rejected

- **WHEN** a task block captures `let xs = [1, 2, 3]` — a `List<Int64>` binding
- **THEN** the compiler rejects it with `E1602:` task block captures unsynchronized gc state; store the list in a `Mutex` or send it through a `Channel`

#### Scenario: Unsynchronized gc state inside a value shape is rejected alike

- **WHEN** a task block captures `let pair = (1, [1, 2])` — a tuple whose second element is a bare `List<Int64>`
- **THEN** the compiler rejects it with `E1602:` task block captures unsynchronized gc state; the shape being value-category changes nothing, the list it carries is live across tasks

#### Scenario: A synchronized gc capture is legal

- **WHEN** a task block captures `let m = Mutex(0)` and its body calls `m.update(|v| v + 1)`
- **THEN** the capture is legal — `Mutex` is a synchronized type and its methods carry their discipline with them

#### Scenario: A read-only resource capture is legal

- **WHEN** a `byres` record type `Config` declares only `self` methods besides `release`, and a task block captures a `scope resource` binding of it
- **THEN** the capture is legal: no method two tasks could race on exists, and `release` stays the scope-exit machinery's alone

#### Scenario: A mutable resource capture is rejected

- **WHEN** a resource type declares `fn bump(mut self)` and a task block captures a binding of it
- **THEN** the compiler rejects it with `E1604:` task block captures a resource whose type declares mut self methods; restrict the type to `self` methods or keep the task out

#### Scenario: A generic capture needs the bound

- **WHEN** a generic fn's task block captures a binding of type `T` and `T` carries no bound
- **THEN** the compiler rejects it with `E1605:` task block captures a generic-parameter binding without a Shareable bound; declare the bound or keep the capture concrete

### Requirement: The Shareable marker

`Shareable` is a marker interface the compiler attaches: a type is Shareable exactly when it belongs to the closed set — chapter 7's base types; the unit type; value-category records whose every field type is Shareable; value-category sums whose every payload type is Shareable; tuples of Shareable types; and the synchronized gc types (`Mutex`, `RwLock`, `Atomic`, `AtomicRef`, `Cond`, `Semaphore`, `Channel`, `SendOnly`, `ReceiveOnly`, `TaskHandle`, `CancelSignal`). The set is closed and mechanically computed from declarations; the compiler attaches the marker, nothing else does, and a manual `impl Shareable for ...` is rejected with `E1606:` Shareable cannot be manually implemented. Gc types outside the synchronized set are not Shareable — a closure or function value is not Shareable either, a closure able to hold live gc state no static rule splits from a pure one; passing logic across tasks goes through a `Channel` or a shared-state cell, not a captured closure. `Shareable` is not a derives target — chapter 10's derives set is untouched, its forward sentence amended in this change — and it occupies no value surface: it is a bound, written `T: Shareable` in a where clause or the declaration's own clause, never a parameter or a box. The name is prelude-visible under chapter 15's amendment — a compiler-attached marker is a language-level name, not a module item, and a bound a generic declaration cannot even write is no bound at all.

#### Scenario: The bound admits the closed set

- **WHEN** a generic fn declares a `Shareable`-bounded parameter and is applied at `Int64`, a value record of Shareable fields, and `Mutex<Int64>`
- **THEN** each application is legal — each type's Shareable membership follows from the closed set alone

#### Scenario: A manual impl is rejected

- **WHEN** `impl Shareable for Conn {}` is written
- **THEN** the compiler rejects it with `E1606:` Shareable cannot be manually implemented; the marker is computed from the declaration, and `Conn` is Shareable exactly when its own shape says so

#### Scenario: A closure is not Shareable

- **WHEN** a generic fn's `Shareable`-bounded parameter is applied at a closure's type
- **THEN** the application is rejected — function values are outside the closed set; send the work item across a channel instead

### Requirement: CancelSignal and currentCancelSignal

`CancelSignal` is a gc-category type of `std.concurrent`, the cooperative-cancellation token of the task system. A task's signal is created with the task and shared by every handle to it: `fn cancel(mut self)` on a `TaskHandle` marks that task's signal cancelled; an expired `scope timeout` marks the signals of its unfinished tasks; nothing else does. The signal's methods: `fn isCancelled(self) -> Bool` — the non-blocking query; `fn awaitCancelled(mut self)` — block until the signal is cancelled, the fourth of select's wait sources. Cancellation is cooperative and nothing interrupts a running task by force: a cancelled task keeps executing until it checks its own signal and returns, or finishes; a task that never checks runs to completion, its cancellation signaled but unanswered — the boundary is stated, not hidden, and forced interruption exists nowhere in this language, it being incompatible with chapter 13's single release point and the shared-state disciplines alike. Reading a signal performs no effect (chapter 16's carve-out). A task body reaches its own signal through `currentCancelSignal()` — a language-level builtin name of the prelude under chapter 15's amendment in this change, callable lexically inside a task block's body only; anywhere else it is rejected with `E1608:` currentCancelSignal called outside a task block, a static error, never a default value or a null signal at runtime.

#### Scenario: A task polls its signal

- **WHEN** a loop inside a task body checks `currentCancelSignal().isCancelled()` each pass and the handle's `cancel()` fires mid-run
- **THEN** the next check sees `true` and the task returns on its own; until it checks, it runs

#### Scenario: A task that never checks runs to completion

- **WHEN** a task's body never touches its signal and its handle is cancelled
- **THEN** the task still runs to completion; the joining scope waits for it — cancellation is a request, not an interruption

#### Scenario: currentCancelSignal outside a task is rejected

- **WHEN** `currentCancelSignal()` appears in a plain fn body outside any task block
- **THEN** the compiler rejects it with `E1608:` currentCancelSignal called outside a task block; take a `CancelSignal` parameter instead when a caller wants to cancel

### Requirement: The implicit-acquisition criterion

`currentCancelSignal()` is this language's one builtin that acquires a value from its calling context rather than a parameter, and the boundary of that exception is fixed here, mechanically: a later chapter MAY ratify another implicit acquisition only by a spec-layer change that shows the candidate against all four conditions of this criterion. One — non-business data: the value is the runtime's or the concurrency system's own control token, carrying no user-defined semantics. Two — lifetime forced: the value's lifetime is bound to the syntactic structure it is acquired in; there is no way to hold it past that structure, and no way for it to be missing while inside. Three — missing statically decidable: use outside the structure is a compile error, never a runtime default or an absent value in disguise. Four — no competing path: no explicit route to the same value exists, so no caller ever chooses between two spellings. `currentCancelSignal()` passes all four — a pure control token, bound to the task block, `E1608` outside it, and no constructor or parameter form of a task's own signal exists. A request context fails the first by definition — it is business data — and explicit parameters are its route, the chapter 0 principles needing no second mechanism.

#### Scenario: The criterion holds a later candidate

- **WHEN** a later change proposes a new context-acquired builtin
- **THEN** its spec amendment answers all four conditions against this requirement's text, and a failure of any one is grounds for rejection on this criterion alone

#### Scenario: A request context is not one

- **WHEN** a proposal argues for an implicit current-request accessor
- **THEN** it fails condition one — trace identifiers and tenants are business data — and the explicit parameter route is the answer this specification gives

### Requirement: Scope blocks

A scope block is `scope block`, `scope timeout(expr) block`, `scope collectAll block`, or `scope timeout(expr) collectAll block` — the `scope` keyword (reserved by chapter 1 since the resources change, its composite uses ratified here), an optional `timeout` clause with an `Int64` expression, an optional `collectAll` marker, and a chapter 2 block. It is a keyword-led expression form under chapter 2's skeleton amendment: the plain and `collectAll` forms' value is the body's block value under chapter 2; the `timeout` forms' value is `Ok(b)` with `b` the body's block value when the body completes inside the budget, and `Err(TimedOut)` — the one constructor of the standard library's named sum `pub type TimeoutError = TimedOut` — when the budget expires, the error handled by chapter 14's ordinary means, `?`, `match`, or `let _ =`; dropping a `Result`-valued scope expression is chapter 8's `E0605`. The scope's own discipline: every `TaskHandle` binding created in the body MUST be awaited or cancelled exactly once on every control-flow path from its creation to the body's end — a handle that reaches the body's normal completion un-awaited, or a second `await` or `cancel` on one handle, is rejected with `E1607:` task handle reaches scope exit un-awaited, or await and cancel both fire, the analysis being chapter 13's single-trigger path analysis applied to handles. An early exit by `?` inside the body — or by a `return`, `break`, or `continue` piercing it — discharges the remaining handles without violation, and the two forms say how: the plain and `timeout` forms cancel them — fail-fast, the first error retreating the whole scope — while a `collectAll` form lets them run to completion and joins them before the exit completes; the choice between the forms is mechanical, fail-fast when the first error makes the rest pointless, collectAll when the results are all wanted, one criterion for one spelling (chapter 0's Principle 2). Scope exit joins its tasks: a scope block returns only after every task created in its body has finished, cancelled or not — no task outlives its scope, and nesting is the same rule inward — an inner scope joins its own tasks before the outer one proceeds, and an expired outer timeout's cancellation reaches the inner scope's tasks through their signals. On timeout expiry the unfinished tasks' signals are marked, the scope waits for their cooperative return, and the value is the `Err`; the budget's granularity is the runtime's, no chapter promising a bound. Waiting at a scope boundary carries no effect tag (chapter 16's carve-out); creating tasks performed no effect either, so a scope block's whole extent answers no enclosing declaration — the tasks' effects answer their own blocks' segments.

#### Scenario: A plain scope joins two tasks

- **WHEN** a scope body creates two fetch tasks, awaits both with `?`, and both succeed
- **THEN** the scope's value is the two results; the block returns only after both tasks finished

#### Scenario: fail-fast cancels the rest

- **WHEN** inside a plain scope the first `await`'s `?` yields `Err` and the second handle has not been awaited
- **THEN** the `?` exits the scope after cancelling the second task through its signal; the handle obligation is discharged by the exit, and the task runs until it checks its signal or finishes — a `return` or `break` piercing the body discharges the same way

#### Scenario: collectAll waits for every result

- **WHEN** the same shape runs under `scope collectAll` and the first `await`'s `?` yields `Err`
- **THEN** the scope waits for the second task's completion before the error leaves the block; the full result set was the point of the form

#### Scenario: A handle reaching completion un-awaited is rejected

- **WHEN** a scope body creates a task handle and completes without any `await` or `cancel` on it, on any path
- **THEN** the compiler rejects it with `E1607:` task handle reaches scope exit un-awaited, or await and cancel both fire; join it or cancel it on every path

#### Scenario: A double use is rejected

- **WHEN** a scope body holds `t.await()` and then `t.cancel()` on the same handle binding
- **THEN** the compiler rejects it with `E1607:` task handle reaches scope exit un-awaited, or await and cancel both fire — the handle is one-shot, one of the two on each path

#### Scenario: timeout wraps the body's value

- **WHEN** `scope timeout(500) { fetchRemote() }` completes in budget with `fetchRemote()` yielding a `Response`
- **THEN** the scope expression's value is `Ok(response)` of type `Result<Response, TimeoutError>`, handled by chapter 14's ordinary means

#### Scenario: timeout expires into Err

- **WHEN** the same scope's body outlives 500 milliseconds
- **THEN** its unfinished tasks' signals are marked, the scope waits for their cooperative return, and the expression's value is `Err(TimedOut)`

#### Scenario: The timeout clause's expression is Int64

- **WHEN** `scope timeout("500") { work() }` appears
- **THEN** the compiler rejects it under `E0501` — the clause's expression must be `Int64`

### Requirement: Panics at the task boundary

A panic unwinding inside a task follows chapter 14's rule inside the task: blocks exit, scope-resource bindings release exactly once each in reverse order, defers run in reverse statement order, and no expression of the language observes the flight. The task boundary is a capture boundary — the one this specification ratifies besides the process abort: a task that ends in a panic leaves no process abort and no second channel; its handle's `await` yields `Err(TaskPanic)` — the standard library's named sum `pub type TaskPanic = Panicked(String)` carrying the panic's message — so a joining scope meets the failure through chapter 14's ordinary `Result` means, Principle 9's one mechanism holding. A task cancelled without an awaited result — its handle cancelled and never awaited — captures its own panic at the same boundary and discards it: the scope chose to abandon the result, and no path re-opens it. Panics in `main` or in a module initializer abort the process exactly as chapter 14 fixed them; concurrency changes nothing there.

#### Scenario: A panicking task surfaces at await

- **WHEN** a task body calls `panic("boom")` and its handle is awaited inside the scope
- **THEN** `await` yields `Err(Panicked("boom"))`; the task's own unwinding released its resources and ran its defers first, and the process did not abort

#### Scenario: A cancelled task's panic is discarded

- **WHEN** a task panics during a run whose handle was cancelled and never awaited
- **THEN** the panic is captured at the task boundary and discarded — the scope abandoned the result; no channel carries it

#### Scenario: A panic in main still aborts

- **WHEN** `panic("boom")` fires in `main` outside any task
- **THEN** the process aborts with the message, chapter 14's rule unchanged

### Requirement: The Channel type

The `std.concurrent` module declares `Channel<T>`, a first-class channel of structured concurrency: a fifo queue of `T` values with a bounded buffer. A channel is constructed by the standard library's `channel(n)` — `n: Int64` the buffer capacity, `channel(0)` the unbuffered synchronous channel — and the element type comes from the expected type at the position, an annotation, a parameter, or a declared return, the empty-list precedent; a construction at a position with no expected type is rejected with `E1617:` channel construction without an expected type, for the language infers nothing and no explicit type-argument call form exists (chapter 10's single-direction rule). `Channel<T>` is a gc-category value, safe across tasks. The methods: `fn send(mut self, v: T)` — enqueue one value, blocking while the buffer is full; sending on a closed channel is a runtime panic of chapter 14's family, the write-after-close being a logic error surfaced early. `fn receive(mut self) -> Option<T>` — dequeue one value, `Some(v)`, blocking while the buffer is empty and open; `None` once the channel is closed and drained, permanently, the receiver's end-of-stream. `fn close(mut self)` — close the channel; receives drain the buffer then yield `None` forever. `fn trySend(mut self, v: T) -> SendResult` and `fn tryReceive(mut self) -> ReceiveResult<T>` — the non-blocking forms, their three-state answers carried by the standard library's named sums `pub type SendResult = Sent | Full | Closed` and `pub type ReceiveResult<T> = Received(T) | Empty | Closed`: success, not-now, and closed do not fold into `Option` or `Result` without losing which of the two failures it was, and the closed sums match exhaustively under chapter 9's rules like any other. Sending and receiving carry no effect tag (chapter 16's carve-out).

#### Scenario: A channel takes its element type from the annotation

- **WHEN** `let ch: Channel<Int64> = channel(4)` appears
- **THEN** `ch` is a `Channel<Int64>` with a four-slot buffer; the expected type fixed the element type and no other form exists

#### Scenario: A construction without an expectation is rejected

- **WHEN** `let ch = channel(4)` appears with no annotation
- **THEN** the compiler rejects it with `E1617:` channel construction without an expected type; annotate the binding or pass the channel where a type fixes it

#### Scenario: receive yields Some until close drains

- **WHEN** a producer sends 1 and 2 then closes, and a consumer receives three times
- **THEN** the receives yield `Some(1)`, `Some(2)`, then `None` — the buffer drained, the channel closed, `None` permanent

#### Scenario: trySend answers without blocking

- **WHEN** `trySend(v)` runs on a full buffer and then on a closed channel
- **THEN** the answers are `Full` and `Closed` — the value was not sent either way, and the caller knows which case it is by the variant, not by a probe

#### Scenario: send after close panics

- **WHEN** `send(v)` runs on a closed channel
- **THEN** the call panics at runtime under chapter 14's family; close means the sender is done, and writing past it is surfaced, not absorbed

### Requirement: Directional channel views

`SendOnly<T>` and `ReceiveOnly<T>` are view types of `Channel<T>` from `std.concurrent`: `SendOnly<T>` carries `send`, `trySend`, and `close`; `ReceiveOnly<T>` carries `receive` and `tryReceive`. A view is made by explicit conversion — `fn toSendOnly(self) -> SendOnly<T>` and `fn toReceiveOnly(self) -> ReceiveOnly<T>` on the channel — and by nothing else: the argument-position implicit narrowing of v0.8 is not inherited, this specification's no-implicit-conversion rule holding unbroken (chapter 7's `E0501`; the `Never` exemption is the one there is), and an explicit method costs one token where the implicit form hid a conversion in the parameter's annotation. The conversion is one-way: no member turns a view back into a `Channel<T>`, a call to a member that does not exist resolving under chapter 10's `E0816`, and the reintroduction of the dropped capability is the thing the view exists to prevent. Views are gc-category values, safe across tasks, waiting carries no effect tag, and a manual `impl` of any interface over a view or any other type of this chapter is rejected under chapter 10's `E0811` — the builtin types carry builtin surfaces, no manual impl production fits them.

#### Scenario: A channel narrows explicitly for a producer

- **WHEN** a task block passes `ch.toSendOnly()` to a fn declared `fn produce(out: SendOnly<Int64>)`
- **THEN** the conversion is the one token that makes the function's write-only intent readable in its signature; inside the fn, no receive exists to call

#### Scenario: The wrong direction has no member

- **WHEN** `out.receive()` appears with `out: SendOnly<Int64>`
- **THEN** the compiler rejects it with `E0816:` no such member on the receiver's type — the view carries its direction's members alone

#### Scenario: No way back to the full channel

- **WHEN** a `SendOnly<T>` binding is used where a `Channel<T>` is expected
- **THEN** the compiler rejects it under `E0501` — no member and no conversion returns the full channel; make the view from the channel again, at the source

#### Scenario: A manual impl over a chapter-18 type is rejected

- **WHEN** `impl Iterable<Int64> for Channel<Int64>` is written
- **THEN** the compiler rejects it with `E0811:` impl head is not a nominal type, chapter 10's rule over builtin heads; the builtin types carry builtin surfaces

### Requirement: The select expression

A select expression is `select { case name = source => body ... }` — two or more cases, newline-separated, each `case` (chapter 1 as amended), a binding name or the wildcard `_`, an equals sign, a wait source, an arrow, and a body expression; the arms of a chapter 4 pattern set beyond the binding and the wildcard fit no production here — a variant pattern would need a no-match policy no rule gives, and `case` binds what the source yields, all of it. A select expression is a keyword-led expression form under chapter 2's skeleton amendment: its value is the taken case's body value, the arms MUST agree in type under chapter 8's agreement rule — `E1610:` select arms disagree in type — a statement-position select follows chapter 8's discard rule (`E0605`), and the unit-typed body is the common shape, `process(v)` discarding nothing. The wait source is one of exactly four method calls — `Channel<T>.receive()`, `ReceiveOnly<T>.receive()`, `TaskHandle<T>.await()`, `CancelSignal.awaitCancelled()` — the closed set, nothing else; any other expression in the source position is rejected with `E1609:` select source is not a wait source. A pattern beyond a name or `_` in the case position is rejected with `E1611:` select case pattern is not a binding or the wildcard; fewer than two cases is rejected with `E1612:` select holds fewer than two cases — a one-case select is a plain call wearing syntax. The semantics: the expression waits at its sources, takes one ready case — which one, when several are ready, is unspecified-but-safe: any choice is a valid execution and none races or corrupts — binds that source's yield to the name, and evaluates that case's body as the value. Waiting carries no effect tag (chapter 16's carve-out); the source calls are the primitives' own.

#### Scenario: select races a receive against completion

- **WHEN** `select { case v = ch.receive() => process(v) case _ = stop.awaitCancelled() => abandon() }` runs with `stop` already cancelled and `ch` holding a value
- **THEN** one ready case is taken — either, both being valid — its body evaluated with the source's yield bound; the other source's state is untouched, and the arms agree on the unit type

#### Scenario: A non-source is rejected

- **WHEN** a select case holds `xs.get(0)` as its source
- **THEN** the compiler rejects it with `E1609:` select source is not a wait source; the wait sources are the four calls, and `get` is not among them

#### Scenario: Arms must agree

- **WHEN** one case's body types `Int64` and the other's types `String`
- **THEN** the compiler rejects it with `E1610:` select arms disagree in type; give the arms one type, the unit type included

#### Scenario: A variant pattern in a case is rejected

- **WHEN** `case Some(v) = ch.receive() => v` appears
- **THEN** the compiler rejects it with `E1611:` select case pattern is not a binding or the wildcard; bind the whole `Option` and match it in the body

#### Scenario: A single case is rejected

- **WHEN** a select expression holds one case
- **THEN** the compiler rejects it with `E1612:` select holds fewer than two cases; call the source directly

### Requirement: Scheduling promises

The task system promises eventual execution and nothing past it: every task created in a scope body is eventually scheduled and runs to its own end, no task starving forever by scheduler defect; the order of task execution, the interleaving of concurrent tasks, the fairness of time-slice allocation, and the promptness of a wake after a signal are all unspecified — the runtime's own, with no chapter promising a bound. The unspecified is safe: whatever interleaving occurs, each task's own executions follow this specification's single-task rules — chapter 8's aliasing, chapter 13's releases, chapter 12's closures — and the synchronization types' disciplines are the only crossings between tasks, so no interleaving breaks a rule this specification states.

#### Scenario: Every task eventually runs

- **WHEN** a scope body creates tasks
- **THEN** each is eventually scheduled and runs; none waits forever without a turn

#### Scenario: Interleaving is unspecified but never rule-breaking

- **WHEN** two tasks run concurrently over one `Mutex`-held value with unrelated work between their `update` calls
- **THEN** the interleaving is any of the possible ones; whichever occurs, each `update`'s callback ran indivisibly — the discipline holds under every schedule

### Requirement: Concurrency diagnostics segment

The concurrency chapter owns registry segment `E1600`–`E1699` declared in `docs/spec/diagnostics.toml` `[segments]`. Allocations: `E1601` task block without an effect segment, `E1602` task block captures unsynchronized gc state, `E1603` task block captures a var binding, `E1604` task block captures a resource whose type declares mut self methods, `E1605` task block captures a generic-parameter binding without a Shareable bound, `E1606` Shareable cannot be manually implemented, `E1607` task handle reaches scope exit un-awaited, or await and cancel both fire, `E1608` currentCancelSignal called outside a task block, `E1609` select source is not a wait source, `E1610` select arms disagree in type, `E1611` select case pattern is not a binding or the wildcard, `E1612` select holds fewer than two cases, `E1613` nested access to one shared value within its own callback, `E1614` Atomic type argument is not an atomic base type, `E1615` AtomicRef type argument is not a gc record type, `E1616` Semaphore count is not a positive integer, `E1617` channel construction without an expected type, `E1618` task block outside any scope block. `E1600` and `E1619`–`E1699` remain reserved for amendments of this chapter. Trigger semantics live in this chapter's Requirements; entries live in the registry.

#### Scenario: A concurrency code is emitted

- **WHEN** any `E16xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `1800-concurrency`

#### Scenario: A later change needs this segment's codes

- **WHEN** a later chapter ratifies rules requiring new concurrency diagnostics — the transaction family among them
- **THEN** its change extends the registry within `E1600`–`E1699` in the same change, or claims its own segment
