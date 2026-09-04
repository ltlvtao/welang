# Delta: declarations (host amendment for sum types)

## MODIFIED Requirements

### Requirement: File structure and module identity

A source file is a module. The file's top level is a sequence of top-level items: import declarations, fn declarations, record and newtype declarations per chapter 8, sum type declarations per chapter 9, and top-level let bindings. Items may appear in any order; ordering conventions are the formatter's business, not the grammar's. Statements and expressions MUST NOT appear as top-level items — they exist only inside blocks — and a top-level token sequence fitting no item production MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). The module's name is the dotted lowercase path naming it per chapter 1's naming conventions; how a path resolves to a file (directory mapping, standard-library priority, dependency cache) is ratified by the module-system chapter.

#### Scenario: A file parses as one module

- **WHEN** a source file holds imports, fn declarations, record, newtype, and sum type declarations, and top-level let bindings
- **THEN** each parses as one top-level item and the file is one module; nothing else is accepted at the top level

#### Scenario: A statement at the top level is rejected

- **WHEN** an expression or an assignment appears as a top-level item, for example `count = count + 1` or `compute()` at the file's top level
- **THEN** the compiler rejects it with `E0105:` unexpected token, naming the item productions considered

#### Scenario: Import is a top-level-only declaration

- **WHEN** `import std.io` appears inside a block
- **THEN** the compiler rejects it with `E0105:` unexpected token; imports exist only as top-level items
