## MODIFIED Requirements

### Requirement: The prelude

The prelude is a fixed set of standard-scope names visible in every module without import: the base type names of chapter 7, `Never` of chapter 9, `Dyn` of chapter 10, the result type names `Result`, `Ok`, and `Err`, the option type names `Option`, `Some`, and `None`, the collection type names `List`, `Map`, and `Set` of the collections chapter, the termination function names `panic`, `todo`, and `assert` of chapter 14, the compiler-attached marker `Shareable` of the concurrency chapter, and the cancellation accessor `currentCancelSignal` of the concurrency chapter. The set is closed: adding a name to it is a spec change to this chapter, not a standard-library release. A module's own declarations shadow prelude names — the prelude is an outer scope and explicit declarations win, chapter 0's Principle 5; shadowing a prelude name is legal and raises no diagnostic, for `E0404` governs duplicates among one module's own names only. Everything else the standard library holds is an ordinary module under `std.`, reached by import alone.

#### Scenario: Prelude names need no import

- **WHEN** a module with no imports uses `String`, `List`, `Result`, `Ok`, `Err`, and `panic`, and a task block body in it calls `currentCancelSignal()`
- **THEN** each name resolves to its standard-scope meaning; no import is required

#### Scenario: A local declaration shadows a prelude name

- **WHEN** a module declares `fn panic(msg: String) -> Never` of its own
- **THEN** the module's `panic` shadows the prelude's within it, with no collision diagnostic; `E0404` still governs duplicates among the module's own names

#### Scenario: The cancellation accessor's legality is the concurrency chapter's

- **WHEN** `currentCancelSignal()` is called inside a task block body, and again in a plain function body outside any task block
- **THEN** the first resolves to the prelude's accessor; the second is the concurrency chapter's `E1608` — the prelude carries the name, that chapter fixes where it is legal

#### Scenario: The rest of the standard library is imported

- **WHEN** a module uses a name of `std.io` without `import std.io`
- **THEN** the use is rejected as an unresolved name under `E1304`; standard-library modules beyond the prelude are reached by import alone
