## MODIFIED Requirements

### Requirement: The prelude

The prelude is a fixed set of standard-scope names visible in every module without import: the base type names of chapter 7, `Never` of chapter 9, `Dyn` of chapter 10, the result type names `Result`, `Ok`, and `Err`, the option type names `Option`, `Some`, and `None`, the collection type names `List`, `Map`, and `Set` of the collections chapter, and the termination function names `panic`, `todo`, and `assert` of chapter 14. The set is closed: adding a name to it is a spec change to this chapter, not a standard-library release. A module's own declarations shadow prelude names — the prelude is an outer scope and explicit declarations win, chapter 0's Principle 5; shadowing a prelude name is legal and raises no diagnostic, for `E0404` governs duplicates among one module's own names only. Everything else the standard library holds is an ordinary module under `std.`, reached by import alone.

#### Scenario: Prelude names need no import

- **WHEN** a module with no imports uses `String`, `List`, `Result`, `Ok`, `Err`, and `panic`
- **THEN** each name resolves to its standard-scope meaning; no import is required

#### Scenario: A local declaration shadows a prelude name

- **WHEN** a module declares `fn panic(msg: String) -> Never` of its own
- **THEN** the module's `panic` shadows the prelude's within it, with no collision diagnostic; `E0404` still governs duplicates among the module's own names

#### Scenario: The rest of the standard library is imported

- **WHEN** a module uses a name of `std.io` without `import std.io`
- **THEN** the use is rejected as an unresolved name under `E1304`; standard-library modules beyond the prelude are reached by import alone
