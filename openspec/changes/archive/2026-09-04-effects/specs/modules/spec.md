## MODIFIED Requirements

### Requirement: The main convention

A program's entry module is the root module: the module `main` at the source root, the file `src/main.we`. The root module MUST declare exactly one `pub fn main() -> Result<(), E>`, with `E` a named sum type under chapter 14's constraint — the v0.8 draft's `Error` is one legal spelling of `E`, not a required one. A root module declaring no `main`, a `main` without `pub`, or a `main` whose signature is not of that shape MUST be rejected with `E1305:` main function signature violation. `main` MAY carry an effect segment per chapter 16 — `pub fn main() effect io -> Result<(), AppError>` conforms: process-start work is exactly what `main` is for, the segment travels with the declaration, and the shape `E1305` checks is the parameterless parameter list, the `Result<(), E>` return, and the `pub` — the effect segment is no part of the violation. In a module other than the root module, `main` is an ordinary fn name carrying no convention. `main` runs after every module's initialization. Returning `Ok(())` exits the process with code 0; returning `Err(e)` exits non-zero after reporting the failure's message on stderr — the message's format is the standard library's business, not this chapter's. `main` takes no parameters: the executable's command-line reach is the standard library's, not the language's.

#### Scenario: A conforming root module

- **WHEN** `src/main.we` declares `pub fn main() -> Result<(), AppError>` and returns `Ok(())`
- **THEN** the program compiles, every module initializes, `main` runs, and the process exits with code 0

#### Scenario: main without pub is rejected

- **WHEN** `src/main.we` declares `fn main() -> Result<(), AppError>` without `pub`
- **THEN** the compiler rejects it with `E1305:` main function signature violation

#### Scenario: A main with an effect segment conforms

- **WHEN** `src/main.we` declares `pub fn main() effect io -> Result<(), AppError>` whose body performs io under chapter 16
- **THEN** the program compiles and runs as the convention's; the effect segment travels with the declaration and is no part of the shape `E1305` checks

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
