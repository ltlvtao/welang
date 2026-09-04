## MODIFIED Requirements

### Requirement: File structure and module identity

A source file is a module. The file's top level is a sequence of top-level items: import declarations, fn declarations, record and newtype declarations per chapter 8, sum type declarations per chapter 9, interface declarations and impl blocks per chapter 10, foreign blocks per chapter 19, and top-level let bindings. Items may appear in any order; ordering conventions are the formatter's business, not the grammar's. Statements and expressions MUST NOT appear as top-level items — they exist only inside blocks — and a top-level token sequence fitting no item production MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). The module's name is the dotted lowercase path naming it per chapter 1's naming conventions; a path resolves to a file under chapter 15's mapping — directory nesting under the source root, the reserved `std` segment, and the dependency cache.

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

### Requirement: Function declarations

A fn declaration is `fn name(params) block` or `fn name(params) -> type block`, optionally prefixed by `pub`. The name is an identifier under chapter 1's naming conventions (`E0012`). The parameter list is zero or more `name: type` pairs separated by commas; every parameter MUST carry a type annotation — a bare parameter name fits no production and MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`), which names the `name: type` production. A declared return type after `->` states that the function produces a value; its absence states that the function produces none. The grammar of types filling the annotation slots is ratified by the types chapter. Generic parameters are ratified by chapter 10: `fn name<T1, ..., Tk>(params)` carries the clause between the name and the parameter list, and a where clause may trail the signature before the body; the one bare-parameter exception is the chapter-10 method receiver — inside impl blocks the first parameter is `self` or `mut self`, written bare, its type fixed by the impl head. Effect segments are ratified by chapter 16: a fn declaration may carry `effect tag1 tag2 ...` between the parameter list and the arrow or body, and the checks the segment participates in are that chapter's. Foreign declarations are ratified by chapter 19: inside a foreign block a fn declaration is this signature form with no body, and the effect segment is required there. `mut` parameters remain deferred to their owning chapter.

#### Scenario: A function with a full signature

- **WHEN** `pub fn add(a: Int64, b: Int64) -> Int64 { a + b }` appears as a top-level item
- **THEN** it parses as one fn declaration: two annotated parameters, a declared return type, and a chapter-2 block body

#### Scenario: A parameter without an annotation is rejected

- **WHEN** `fn f(x) { }` appears
- **THEN** the compiler rejects it with `E0105:` unexpected token at the parameter, naming the `name: type` production; parameter types are never inferred

#### Scenario: A function without a return type produces no value

- **WHEN** `fn log(m: String) { emit(m) }` appears
- **THEN** it parses as a fn declaration that produces no value; the omitted return type states this and no `->` clause is required

#### Scenario: A generic function parses

- **WHEN** `fn identity<T>(x: T) -> T { x }` appears as a top-level item
- **THEN** it parses as one fn declaration with a one-parameter generic clause per chapter 10; the clause sits between the name and the parameter list and the rest follows this chapter's forms

#### Scenario: A function with an effect segment parses

- **WHEN** `fn write(msg: String) effect io { save(msg) }` appears as a top-level item
- **THEN** it parses as one fn declaration with an effect segment between the parameter list and the body, per chapter 16; omitted, the declaration states a pure function
