# We Language Specification — Chapter 6: Declarations

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

### Requirement: Function declarations

A fn declaration is `fn name(params) block` or `fn name(params) -> type block`, optionally prefixed by `pub`. The name is an identifier under chapter 1's naming conventions (`E0012`). The parameter list is zero or more `name: type` pairs separated by commas; every parameter MUST carry a type annotation — a bare parameter name fits no production and MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`), which names the `name: type` production. A declared return type after `->` states that the function produces a value; its absence states that the function produces none. The grammar of types filling the annotation slots is ratified by the types chapter. Generic parameters are ratified by chapter 10: `fn name<T1, ..., Tk>(params)` carries the clause between the name and the parameter list, and a where clause may trail the signature before the body; the one bare-parameter exception is the chapter-10 method receiver — inside impl blocks the first parameter is `self` or `mut self`, written bare, its type fixed by the impl head. Effect segments are ratified by chapter 16: a fn declaration may carry `effect tag1 tag2 ...` between the parameter list and the arrow or body, and the checks the segment participates in are that chapter's. `mut` parameters and foreign declarations are ratified by their owning chapters.

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

### Requirement: Module name space

One module has one name space. fn names, top-level let names, and names introduced by import declarations share it; a second declaration of the same name MUST be rejected with `E0404`. Block-local names are not part of this space; their shadowing relationship with top-level names is ratified by the types chapter.

#### Scenario: Two declarations with one name

- **WHEN** one module declares `fn f` twice, or `fn f` and `let f`
- **THEN** the compiler rejects the second with `E0404:` duplicate name in one module

#### Scenario: An import name collides with a declaration

- **WHEN** one module contains `import std.io as io` and `fn io()`
- **THEN** the compiler rejects the later one with `E0404:` duplicate name in one module

### Requirement: Documentation comments

Consecutive `///` lines form one documentation unit, per chapter 1's lexical form. A documentation unit attaches to the next top-level item; blank lines and ordinary comments between the unit and that item do not break the attachment. A documentation unit with no following top-level item in the file MUST be rejected with `E0405`. What documentation content means and how the toolchain renders it are the tooling chapters'; this chapter ratifies attachment only.

#### Scenario: A documentation unit attaches to the next item

- **WHEN** two `///` lines precede a fn declaration
- **THEN** they form one documentation unit attached to that fn; the next item after it carries no attachment from them

#### Scenario: An orphan documentation comment

- **WHEN** a `///` run has no following top-level item before the end of the file
- **THEN** the compiler rejects it with `E0405:` orphan documentation comment

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–6. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Typing-level rules are annotated as pending the types chapter.

### A module of declarations

```we
import std.io as io
import models.user

/// Retry budget shared across this module's handlers.
pub let maxRetries = 3

pub fn handle(id: Int64) -> Bool {
    let u = user.find(id)
    retry(u, maxRetries)
}

fn retry(u: user.User, budget: Int64) -> Bool {   // module-local: no pub
    io.println("retrying")
    true
}
```

### Return, defer, and the body's value

```we
pub fn loadAll(ids: List<Int64>) -> Int64 {   // final expression is the
    var total = 0                             // implicit return value
    for id in ids {
        total = total + process(id)
    }
    total
}

pub fn process(id: Int64) -> Int64 {
    defer { io.println("done") }   // legal: a direct item of the fn body
    if id < 0 {
        return 0                   // early return; defer runs at exit
    }
    transform(id)
}

fn emit(line: String) {            // no return type: produces no value
    io.println(line)               // final expression's value is
}                                  // discarded implicitly (types chapter)
```

### Rejected forms

```we
var count = 0                      // E0403: var at the top level
return 1                           // E0401: return outside a function body
fn bad(x) { }                      // E0105: parameter annotation required
fn noValue() { return 1 }          // E0402: value return in a valueless fn
pub import std.io                  // E0105: pub prefixes fn and let only
import Models.User                 // E0013: module names must be lowercase
fn io() { }                        // E0404: duplicate name in one module
                                  // (io already introduced by import)

fn f() {
    import std.io                  // E0105: import is top-level only
    return
}
```

### Pending later chapters

```we
// Module resolution, the cross-module visibility diagnostic, and
// the main convention landed with chapter 15; generic parameters
// landed with the interfaces chapter; effect segments landed with
// chapter 16. mut parameters and foreign blocks are their owning
// chapters':
//
// fn read(path: String) effect io -> String
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| declaration | 声明 |
| top-level item | 顶层项 |
| module | 模块 |
| import declaration | import 声明 |
| alias | 别名 |
| fn declaration | fn 声明 |
| parameter | 参数 |
| return type | 返回类型 |
| function context | 函数语境 |
| top-level binding | 顶层绑定 |
| module-local | 模块内局部 |
| name space | 名字空间 |
| documentation unit | 文档单元 |
| orphan | 孤儿 |
