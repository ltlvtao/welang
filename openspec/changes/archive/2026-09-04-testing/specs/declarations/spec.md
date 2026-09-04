## MODIFIED Requirements

### Requirement: File structure and module identity

A source file is a module. The file's top level is a sequence of top-level items: import declarations, fn declarations, record and newtype declarations per chapter 8, sum type declarations per chapter 9, interface declarations and impl blocks per chapter 10, foreign blocks per chapter 19, test blocks per chapter 20 — legal only in a test module, a file whose name ends `_test.we` — and top-level let bindings. Items may appear in any order; ordering conventions are the formatter's business, not the grammar's. Statements and expressions MUST NOT appear as top-level items — they exist only inside blocks — and a top-level token sequence fitting no item production MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). The module's name is the dotted lowercase path naming it per chapter 1's naming conventions; a path resolves to a file under chapter 15's mapping — directory nesting under the source root, the reserved `std` segment, and the dependency cache.

#### Scenario: A file parses as one module

- **WHEN** a source file holds imports, fn declarations, record, newtype, and sum type declarations, interface declarations and impl blocks, and top-level let bindings
- **THEN** each parses as one top-level item and the file is one module; nothing else is accepted at the top level

#### Scenario: A statement at the top level is rejected

- **WHEN** an expression or an assignment appears as a top-level item, for example `count = count + 1` or `compute()` at the file's top level
- **THEN** the compiler rejects it with `E0105:` unexpected token, naming the item productions considered

#### Scenario: Import is a top-level-only declaration

- **WHEN** `import std.io` appears inside a block
- **THEN** the compiler rejects it with `E0105:` unexpected token; imports exist only as top-level items

#### Scenario: A foreign block is a top-level item

- **WHEN** a source file holds `foreign "c" { ... }` among its imports, fn declarations, and top-level let bindings
- **THEN** the block parses as one top-level item per chapter 19; a foreign block inside any block is rejected there (`E1702`), and the items it declares join the module's one name space

#### Scenario: A test block is a top-level item of a test module

- **WHEN** the file `math_test.we` holds `test "adds" { let _ = add(1, 2) }` among its imports and fn declarations, while a file not ending `_test.we` holds the same block
- **THEN** the first parses as one top-level item per chapter 20; the second is rejected there (`E1801`) — the `_test.we` name is the module-identity fact that carries the legality
