## ADDED Requirements

### Requirement: Module paths and file resolution

A module path is a dotted lowercase module path under chapter 1's module naming convention. A local module path resolves against the project's source root: every segment but the last names a directory and the last segment names a `.we` file, both relative to the `src/` directory of the project root — `import models.user` resolves to `src/models/user.we`. The project root is the directory holding the project manifest; the manifest's format is the toolchain's business, not this chapter's. A path's first segment `std` is reserved: a module path beginning `std.` always names a compiler-provided standard-library module, never resolved against the file system, and no local file or dependency module can occupy the `std` segment. Local paths take priority over the cache: a dotted path resolving under the project's `src/` names that local module; every other path resolves from the dependency cache under the same directory-and-file mapping — acquisition, version constraints, and lockfiles are the tooling layer's business, not this chapter's. An import whose target does not resolve MUST be rejected with `E1302:` module not found, the message naming the expected file path under the mapping above. Imports form a directed graph: a circular dependency, direct or transitive, MUST be rejected with `E1301:` circular module dependency — there are no forward declarations, and the fix is extracting the shared code into a third module both import.

#### Scenario: A local path maps to a file

- **WHEN** `import models.user` appears and `src/models/user.we` exists
- **THEN** the path resolves to that file under the mapping; the module's name is `models.user`

#### Scenario: A std path never touches the file system

- **WHEN** `import std.io` appears, whatever files exist under `src/std/`
- **THEN** the path names the compiler-provided standard-library module `std.io`; no file is consulted and the `std` segment cannot be occupied locally

#### Scenario: A missing module is rejected with its expected path

- **WHEN** `import models.missing` appears and `src/models/missing.we` does not exist, and no dependency cache entry answers the path
- **THEN** the compiler rejects it with `E1302:` module not found, the message naming the expected path `src/models/missing.we`

#### Scenario: A dependency path resolves from the cache

- **WHEN** `import external.package.name` appears and the dependency cache holds the module
- **THEN** the path resolves from the cache under the same directory-and-file mapping; a path that also resolves under the project's `src/` names the local module, deterministically

#### Scenario: A circular dependency is rejected

- **WHEN** module `a` imports `b` and module `b` imports `a`, directly or through further modules
- **THEN** the compiler rejects it with `E1301:` circular module dependency; there are no forward declarations, and the fix is a third module both import

### Requirement: The prelude

The prelude is a fixed set of standard-scope names visible in every module without import: the base type names of chapter 7, `Never` of chapter 9, `Dyn` of chapter 10, the result type names `Result`, `Ok`, and `Err`, the option type names `Option`, `Some`, and `None`, and the termination function names `panic`, `todo`, and `assert` of chapter 14. The set is closed: adding a name to it is a spec change to this chapter, not a standard-library release. A module's own declarations shadow prelude names — the prelude is an outer scope and explicit declarations win, chapter 0's Principle 5; shadowing a prelude name is legal and raises no diagnostic, for `E0404` governs duplicates among one module's own names only. Everything else the standard library holds is an ordinary module under `std.`, reached by import alone.

#### Scenario: Prelude names need no import

- **WHEN** a module with no imports uses `String`, `Result`, `Ok`, `Err`, and `panic`
- **THEN** each name resolves to its standard-scope meaning; no import is required

#### Scenario: A local declaration shadows a prelude name

- **WHEN** a module declares `fn panic(msg: String) -> Never` of its own
- **THEN** the module's `panic` shadows the prelude's within it, with no collision diagnostic; `E0404` still governs duplicates among the module's own names

#### Scenario: The rest of the standard library is imported

- **WHEN** a module uses a name of `std.io` without `import std.io`
- **THEN** the use is rejected as an unresolved name under `E1304`; standard-library modules beyond the prelude are reached by import alone

### Requirement: Cross-module visibility

Items bearing `pub` are a module's public surface: fn declarations and top-level let bindings per chapter 6, record and newtype declarations per chapter 8, sum type declarations per chapter 9, interface declarations per chapter 10, and impl method definitions per chapter 10. Everything else a module holds is module-local. Cross-module reach is exactly the qualified form `name.item` through an import's name; a use of another module's module-local item — in value position, type position, or as a variant constructor — MUST be rejected with `E1303:` cross-module use of a module-local item. Interface members carry no `pub` per chapter 10: their reach is the interface's, so a method reached through an interface value — a `Dyn<Interface>` box or a generic bound — is legal even when the impl method definition is module-local; `E1303` governs direct name reach alone.

#### Scenario: A pub fn reaches cross-module

- **WHEN** module B declares `pub fn create()` and module A imports B
- **THEN** `b.create()` resolves; the qualified form through the import name is the reach

#### Scenario: A module-local fn is rejected cross-module

- **WHEN** module B declares `fn helper()` without `pub` and module A calls `b.helper()`
- **THEN** the compiler rejects it with `E1303:` cross-module use of a module-local item

#### Scenario: A module-local type is rejected in type position

- **WHEN** module B declares `record Config` without `pub` and module A writes an annotation `b.Config`
- **THEN** the compiler rejects it with `E1303:` cross-module use of a module-local item; type positions enforce the same boundary

#### Scenario: A module-local variant constructor is rejected

- **WHEN** module B declares `type AppError = NotFound(String)` without `pub` and module A writes `b.NotFound("x")`
- **THEN** the compiler rejects it with `E1303:` cross-module use of a module-local item; the constructor position enforces the same boundary as value and type positions

#### Scenario: A module-local impl method reaches through the interface

- **WHEN** module B declares a pub interface, an impl whose method definitions are module-local, and a pub constructor yielding the implementing value, and module A boxes that value as `Dyn<Describable>` and calls the method
- **THEN** the call is legal: the interface's reachability governs, and `E1303` governs direct name reach alone

### Requirement: Name resolution

An unqualified name resolves through scopes innermost-first: block-local bindings and parameters under chapter 8's scope rules, then the module's own name space under chapter 6 — its declarations and import names in one space — then the prelude. The innermost scope holding the name wins. A name held by no scope MUST be rejected with `E1304:` unresolved name. A qualified form `name.item` resolves when `name` is an import name and `item` a pub item of the imported module; a qualifier that is not an import name and an item the imported module does not declare are rejected with `E1304:` unresolved name — an item declared but not pub is `E1303`'s, not this code's. The same rules fill type positions: whether a well-formed type name resolves — bare, module-qualified, or a prelude name — is decided here, discharging chapter 7's pending sentence; a local declaration shadows a prelude type name, `Dyn` included, as it shadows any prelude name.

#### Scenario: An unknown bare name is rejected

- **WHEN** `compute()` appears and no declaration, import, or prelude name holds `compute` in any scope
- **THEN** the compiler rejects it with `E1304:` unresolved name

#### Scenario: A missing qualifier or item is rejected

- **WHEN** `notauser.item` appears where `notauser` is no import name, or `user.missing()` appears where `user` is an import and the module declares no `missing`
- **THEN** the compiler rejects each with `E1304:` unresolved name — the qualifier names no module, or the module declares no such item

#### Scenario: An unresolved type name is rejected

- **WHEN** an annotation holds `Config` and no declaration, imported pub item, or prelude name provides a type `Config`
- **THEN** the compiler rejects it with `E1304:` unresolved name; type-name resolution is decided here, discharging chapter 7's pending sentence

#### Scenario: A shadowed prelude type name follows the local meaning

- **WHEN** a module declares `type Result = Win | Lose` of its own and annotates `let r: Result`
- **THEN** the annotation names the module's own sum; the prelude's `Result` is unreachable by bare name within the module, and no diagnostic fires — shadowing a prelude type name is legal

### Requirement: Module initialization

Every module's top-level let initializers run exactly once, at process start, before `main` runs — eager initialization, no lazy module loading. The order is the import graph's post-order: a module initializes only after every module it imports has fully initialized, the post-order walk following each module's imports in source order from the root module's; within one module, initializers run in source order under chapter 6. Circular dependencies never reach initialization — `E1301` rejects them at compile time — so the order is total and deterministic, chapter 0's Principle 1. A module imported by two others initializes once, not twice. An initializer may call functions of modules it imports: they are initialized before it. A panic during initialization unwinds per chapter 14 and aborts the process — there is no capture boundary before `main`.

#### Scenario: Imported modules initialize before their importers

- **WHEN** the root module imports `a`, `a` imports `b`, and each module's top-level initializer records its own module name
- **THEN** the recorded order is `b`, then `a`, then the root module's own initializers — import-graph post-order

#### Scenario: A doubly imported module initializes once

- **WHEN** the root module imports both `a` and `b`, and both import `c` carrying a top-level initializer
- **THEN** `c`'s initializer runs exactly once, before both `a`'s and `b`'s

#### Scenario: A panic during initialization aborts

- **WHEN** a top-level initializer calls `panic`
- **THEN** the process aborts with the panic's message after unwinding per chapter 14, before `main` runs — no capture boundary exists before `main`

### Requirement: The main convention

A program's entry module is the root module: the module `main` at the source root, the file `src/main.we`. The root module MUST declare exactly one `pub fn main() -> Result<(), E>`, with `E` a named sum type under chapter 14's constraint — the v0.8 draft's `Error` is one legal spelling of `E`, not a required one. A root module declaring no `main`, a `main` without `pub`, or a `main` whose signature is not of that shape MUST be rejected with `E1305:` main function signature violation. In a module other than the root module, `main` is an ordinary fn name carrying no convention. `main` runs after every module's initialization. Returning `Ok(())` exits the process with code 0; returning `Err(e)` exits non-zero after reporting the failure's message on stderr — the message's format is the standard library's business, not this chapter's. `main` takes no parameters: the executable's command-line reach is the standard library's, not the language's.

#### Scenario: A conforming root module

- **WHEN** `src/main.we` declares `pub fn main() -> Result<(), AppError>` and returns `Ok(())`
- **THEN** the program compiles, every module initializes, `main` runs, and the process exits with code 0

#### Scenario: main without pub is rejected

- **WHEN** `src/main.we` declares `fn main() -> Result<(), AppError>` without `pub`
- **THEN** the compiler rejects it with `E1305:` main function signature violation

#### Scenario: A missing main is rejected

- **WHEN** `src/main.we` declares no fn `main` at all
- **THEN** the compiler rejects it with `E1305:` main function signature violation

#### Scenario: A non-Result main shape is rejected

- **WHEN** `src/main.we` declares `pub fn main() -> Int64`
- **THEN** the compiler rejects it with `E1305:` main function signature violation; the shape is `Result<(), E>` with `E` a named sum type

#### Scenario: A non-sum error position in main is chapter 14's code

- **WHEN** `src/main.we` declares `pub fn main() -> Result<(), String>`
- **THEN** the compiler rejects it with `E1204:` Result error type is not a named sum type — the main shape itself conforms, so the error position's named-sum constraint is chapter 14's, and `E1305` does not fire

#### Scenario: Err at exit is non-zero

- **WHEN** `main` returns `Err(e)`
- **THEN** the process reports the failure's message on stderr and exits non-zero; `Ok(())` alone exits 0

#### Scenario: main in a non-root module is an ordinary fn

- **WHEN** an imported module declares `fn main() -> Int64`
- **THEN** it is an ordinary fn name carrying no convention; only the root module's `main` is the entry

### Requirement: The modules diagnostics segment

The modules chapter owns registry segment `E1300`–`E1399`, declared in `docs/spec/diagnostics.toml` `[segments]` with owner `1500-modules`. Allocations: `E1301` circular module dependency, `E1302` module not found, `E1303` cross-module use of a module-local item, `E1304` unresolved name, `E1305` main function signature violation. `E1300` and `E1306`–`E1399` are reserved for this chapter's amendments; later chapters needing segment codes claim their own segments.

#### Scenario: The segment is retrievable

- **WHEN** a diagnostics consumer looks up any `E13xx` code in the registry
- **THEN** the segment entry names owner `1500-modules`, and each allocated code's entry carries severity, title, description, remediation, owner, requirement, and allocated date

#### Scenario: Later changes extend within the segment

- **WHEN** a later spec-layer change to this chapter needs a new code
- **THEN** it allocates from `E1300`–`E1399` within its own change; chapters outside modules claim other segments
