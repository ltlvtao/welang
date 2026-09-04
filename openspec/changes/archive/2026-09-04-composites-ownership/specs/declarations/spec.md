# Delta: declarations (host amendments for composites)

## MODIFIED Requirements

### Requirement: File structure and module identity

A source file is a module. The file's top level is a sequence of top-level items: import declarations, fn declarations, record and newtype declarations per chapter 8, and top-level let bindings. Items may appear in any order; ordering conventions are the formatter's business, not the grammar's. Statements and expressions MUST NOT appear as top-level items — they exist only inside blocks — and a top-level token sequence fitting no item production MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). The module's name is the dotted lowercase path naming it per chapter 1's naming conventions; how a path resolves to a file (directory mapping, standard-library priority, dependency cache) is ratified by the module-system chapter.

#### Scenario: A file parses as one module

- **WHEN** a source file holds imports, fn declarations, record and newtype declarations, and top-level let bindings
- **THEN** each parses as one top-level item and the file is one module; nothing else is accepted at the top level

#### Scenario: A statement at the top level is rejected

- **WHEN** an expression or an assignment appears as a top-level item, for example `count = count + 1` or `compute()` at the file's top level
- **THEN** the compiler rejects it with `E0105:` unexpected token, naming the item productions considered

#### Scenario: Import is a top-level-only declaration

- **WHEN** `import std.io` appears inside a block
- **THEN** the compiler rejects it with `E0105:` unexpected token; imports exist only as top-level items

### Requirement: Function bodies, return and defer

A fn's body block is the function context. Inside it — including blocks nested within it — chapter 3's return forms are legal: with a declared return type, `return expr` exits early with that value, bare `return`'s legality on a value-producing path is ratified by the types chapter; without one, bare `return` exits early and `return expr` MUST be rejected with `E0402`. A return outside any function body — at the top level, or inside a defer body, which runs at function exit where no live control flow remains — MUST be rejected with `E0401`. With a declared return type, the body's block value is the function's implicit return value: a final expression item returns it, and early return carries it; whether every path produces a value is the types chapter's. Without a declared return type, the body's final item is governed by chapter 8's value-discard rule: a final expression whose type is not the unit type MUST be explicitly discarded with `let _ =`, and the unit type needs no ceremony. A fn body's direct items are the placements where chapter 3's defer is legal, grounding that Requirement's rejectable side.

#### Scenario: The final expression is the implicit return value

- **WHEN** a function declares `-> Int64` and its body's final item is `a + b`
- **THEN** the function returns that value; writing `return a + b` in the same position is equivalent

#### Scenario: An early return from a nested block

- **WHEN** `if done { return acc }` appears inside a value-producing function's body
- **THEN** the return is legal, exits the function, and any defers run in reverse order per chapter 3's Defer

#### Scenario: return with a value in a valueless function

- **WHEN** a function without a declared return type contains `return 1`
- **THEN** the compiler rejects it with `E0402:` return with a value in a function that declares none

#### Scenario: return outside a function body

- **WHEN** `return` appears at the top level, or inside a defer body
- **THEN** the compiler rejects it with `E0401:` return outside a function body

#### Scenario: Defer's ratified placement

- **WHEN** a fn body's direct item is `defer { release() }`
- **THEN** it is legal under chapter 3's Defer and runs at function exit in reverse order; the same defer elsewhere continues to fire `E0204`

#### Scenario: A non-unit final expression in a valueless function is rejected

- **WHEN** a function without a declared return type ends in `step()`, which returns `Int64`
- **THEN** the compiler rejects it with `E0605:` non-unit value dropped per chapter 8; the fix is `let _ = step()` or a declared return type
