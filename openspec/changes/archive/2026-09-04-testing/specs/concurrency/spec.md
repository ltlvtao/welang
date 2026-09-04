## MODIFIED Requirements

### Requirement: Scheduling promises

The task system promises eventual execution and nothing past it: every task created in a scope body is eventually scheduled and runs to its own end, no task starving forever by scheduler defect; the order of task execution, the interleaving of concurrent tasks, the fairness of time-slice allocation, and the promptness of a wake after a signal are all unspecified — the runtime's own, with no chapter promising a bound. The testing chapter's amendment is the one refinement: inside a test block the scheduler is deterministic — one test source with one `advanceTime` sequence yields one observable interleaving, the same on every run, that chapter's requirement fixing what determinism is promised — and every execution outside test mode stays under this paragraph's unspecified. The unspecified is safe: whatever interleaving occurs, each task's own executions follow this specification's single-task rules — chapter 8's aliasing, chapter 13's releases, chapter 12's closures — and the synchronization types' disciplines are the only crossings between tasks, so no interleaving breaks a rule this specification states.

#### Scenario: Every task eventually runs

- **WHEN** a scope body creates tasks
- **THEN** each is eventually scheduled and runs; none waits forever without a turn

#### Scenario: Interleaving is unspecified but never rule-breaking

- **WHEN** two tasks run concurrently over one `Mutex`-held value with unrelated work between their `update` calls
- **THEN** the interleaving is any of the possible ones; whichever occurs, each `update`'s callback ran indivisibly — the discipline holds under every schedule

#### Scenario: Test mode is the one refinement

- **WHEN** the same test, creating tasks and advancing the virtual clock by one fixed sequence, runs twice
- **THEN** the observable interleaving is identical between the runs; outside test mode, the unspecified above governs exactly as before

### Requirement: Panics at the task boundary

A panic unwinding inside a task follows chapter 14's rule inside the task: blocks exit, scope-resource bindings release exactly once each in reverse order, defers run in reverse statement order, and no expression of the language observes the flight. The task boundary is a capture boundary — the one this specification ratifies besides the process abort: a task that ends in a panic leaves no process abort and no second channel; its handle's `await` yields `Err(TaskPanic)` — the standard library's named sum `pub type TaskPanic = Panicked(String)` carrying the panic's message — so a joining scope meets the failure through chapter 14's ordinary `Result` means, Principle 9's one mechanism holding. A task cancelled without an awaited result — its handle cancelled and never awaited — captures its own panic at the same boundary and discards it: the scope chose to abandon the result, and no path re-opens it. Panics in `main` or in a module initializer abort the process exactly as chapter 14 fixed them; concurrency changes nothing there. The testing chapter's amendment adds the third capture boundary in its own place: a panic unwinding inside a test block ends at the test boundary as that test's failure, no process abort — chapter 20's requirement carries the rule.

#### Scenario: A panicking task surfaces at await

- **WHEN** a task body calls `panic("boom")` and its handle is awaited inside the scope
- **THEN** `await` yields `Err(Panicked("boom"))`; the task's own unwinding released its resources and ran its defers first, and the process did not abort

#### Scenario: A cancelled task's panic is discarded

- **WHEN** a task panics during a run whose handle was cancelled and never awaited
- **THEN** the panic is captured at the task boundary and discarded — the scope abandoned the result; no channel carries it

#### Scenario: A panic in main still aborts

- **WHEN** `panic("boom")` fires in `main` outside any task
- **THEN** the process aborts with the message, chapter 14's rule unchanged
