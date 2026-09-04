## MODIFIED Requirements

### Requirement: File structure and module identity

A source file is a module. The file's top level is a sequence of top-level items: import declarations, fn declarations, record and newtype declarations per chapter 8, sum type declarations per chapter 9, interface declarations and impl blocks per chapter 10, and top-level let bindings. Items may appear in any order; ordering conventions are the formatter's business, not the grammar's. Statements and expressions MUST NOT appear as top-level items — they exist only inside blocks — and a top-level token sequence fitting no item production MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). The module's name is the dotted lowercase path naming it per chapter 1's naming conventions; a path resolves to a file under chapter 15's mapping — directory nesting under the source root, the reserved `std` segment, and the dependency cache.

#### Scenario: A file parses as one module

- **WHEN** a source file holds imports, fn declarations, record, newtype, and sum type declarations, interface declarations and impl blocks, and top-level let bindings
- **THEN** each parses as one top-level item and the file is one module; nothing else is accepted at the top level

#### Scenario: A statement at the top level is rejected

- **WHEN** an expression or an assignment appears as a top-level item, for example `count = count + 1` or `compute()` at the file's top level
- **THEN** the compiler rejects it with `E0105:` unexpected token, naming the item productions considered

#### Scenario: Import is a top-level-only declaration

- **WHEN** `import std.io` appears inside a block
- **THEN** the compiler rejects it with `E0105:` unexpected token; imports exist only as top-level items

### Requirement: Import declarations

An import declaration is `import path` or `import path as name`. The path MUST be a dotted lowercase module path and the alias, when present, MUST be a lowercase identifier — both under chapter 1's module naming convention (`E0013` at the declaration). An import introduces exactly one name into the module's name space: the path's last segment, or the alias when present. Through that name, the importing module reaches the target module's public items per Visibility with pub; the path resolves under chapter 15's mapping and cross-module visibility is enforced by chapter 15 (`E1303`). Selective imports (`import a.{b, c}`), wildcard imports, and importing under more than one name do not exist.

#### Scenario: Import without an alias introduces the last segment

- **WHEN** `import models.user` appears as a top-level item
- **THEN** the name `user` denotes the imported module; its public items are reached as `user.item`

#### Scenario: Import with an alias

- **WHEN** `import std.io as io` appears as a top-level item
- **THEN** the name `io` denotes the imported module; the path's last segment introduces no name of its own

#### Scenario: An import path or alias violates module naming

- **WHEN** an import path or alias is not lowercase-dotted, for example `import Models.User` or `import std.io as IO`
- **THEN** the compiler rejects it with `E0013:` module names must be lowercase at the declaration

### Requirement: Top-level bindings

A top-level binding is `let name = expr` or `pub let name = expr`, each optionally with a type annotation `name: type` before `=`, per chapter 2's binding form. `var` MUST NOT appear at the top level: a top-level `var` binding MUST be rejected with `E0403` — module-level mutable state does not exist, and `var` remains legal only inside blocks. The initializers of one module's top-level bindings evaluate at module initialization in source order; the cross-module initialization model — once per module, in import-graph post-order, before `main` — is chapter 15's.

#### Scenario: A top-level let binding

- **WHEN** `pub let maxRetries = 3` appears as a top-level item
- **THEN** it parses as a top-level binding whose visibility follows Visibility with pub

#### Scenario: A top-level var is rejected

- **WHEN** `var count = 0` appears as a top-level item
- **THEN** the compiler rejects it with `E0403:` var at the top level; the same form inside a block is unaffected

#### Scenario: Initializers evaluate in source order

- **WHEN** a module declares two top-level bindings whose initializers are `first()` then `second()`
- **THEN** within that module `first()` evaluates before `second()` at module initialization

### Requirement: Visibility with pub

`pub` is a prefix on ratified declaration kinds — fn declarations and top-level let bindings per this chapter, record and newtype declarations per chapter 8, sum type declarations per chapter 9, interface declarations and impl method definitions per chapter 10. A pub item is visible outside its module; an item without `pub` is module-local. Items of another module are reachable through an import only when pub; the diagnostic for using a non-pub item of another module is chapter 15's (`E1303`), and the main-function convention is chapter 15's. `pub` anywhere else — on an import, inside a block, before any other token sequence — fits no production and MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`).

#### Scenario: pub marks an item visible across modules

- **WHEN** module A imports module B and B declares `pub fn f()`
- **THEN** `f` is reachable from A through the imported module name; the same fn without `pub` is module-local and its cross-module use is rejected with chapter 15's `E1303:` cross-module use of a module-local item

#### Scenario: pub outside its ratified items

- **WHEN** `pub` prefixes an import or appears inside a block, for example `pub import std.io` or `let x = { pub fn f() { } }`
- **THEN** the compiler rejects it with `E0105:` unexpected token
