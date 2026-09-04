# We Language Specification — Chapter 13: Resources and Releasable


### Requirement: The Releasable interface

The standard library declares the interface `Releasable` under chapter 10's declaration forms: one method, `fn release(mut self)`, a `mut self` receiver, no return type — producing the unit type — no generic parameters, no associated types. Releasable declares no associated types, so chapter 10's box rule would admit `Dyn<Releasable>`; this chapter's composite-position ban rejects it (Resources keep to binding positions) — a resource is never boxed, the box's shared-handle semantics at odds with the single-handle discipline. Releasable is the resource category's release contract, and the resource category is its only implementer: an impl of Releasable whose head type is not a `byres record` — a gc record, a value record, a sum, a newtype, a base type, a generic parameter — is rejected with `E1102:` Releasable implemented by a non-resource type. Every `byres record` MUST implement Releasable; the obligation is declaration completeness, checked where the record is declared, chapter 10's orphan rule holding the impl in the record's own module: a `byres record` whose declaring module holds no impl of Releasable for it is rejected with `E1101:` resource record must implement Releasable. The method's signature is the interface's; an impl that deviates — a `self` receiver, a declared return type — is chapter 10's `E0808`.

#### Scenario: A byres record implements Releasable

- **WHEN** `byres record FileHandle { fd: Int64 }` is declared and its module holds `impl Releasable for FileHandle { fn release(mut self) { close(self.fd) } }`
- **THEN** the declaration is complete; FileHandle carries the release contract and is fit for the scope resource statement

#### Scenario: A non-resource implementer is rejected

- **WHEN** `impl Releasable for User` appears, `User` a gc record
- **THEN** the compiler rejects it with `E1102:` Releasable implemented by a non-resource type

#### Scenario: A byres record without an impl is rejected

- **WHEN** `byres record LogFile { fd: Int64 }` is declared and its module holds no impl of Releasable for LogFile
- **THEN** the compiler rejects it with `E1101:` resource record must implement Releasable; the fix is the impl, not a different record category

#### Scenario: A deviating release signature is chapter 10's

- **WHEN** an impl of Releasable writes the method as `fn release(mut self) -> Bool`
- **THEN** the compiler rejects it with chapter 10's `E0808`: impl method signature mismatches the interface method

### Requirement: The scope resource statement

The scope resource statement is `scope resource(name = expr, ..., name = expr) block`, with at least one binding. The two words `scope` and `resource` enter chapter 1's keyword list by this chapter's amendment; the statement opens with them, so chapter 2's statement-start class — which admits keywords — admits it with no further amendment, and chapter 2's statement-family enumeration credits this chapter. Each binding is `name = expr` with no annotation slot: the binding's type is the head expression's, and the head expression's type MUST implement Releasable — a head whose type does not is rejected with `E1103:` scope resource head does not implement Releasable; under the completeness obligation above the implementers are exactly the `byres record`s, so the check rejects every non-resource head. Head expressions evaluate left to right; each name binds for the block. At block exit the compiler guarantees exactly one `release` call per binding, in reverse declaration order; exits pierced by `return`, `break`, or `continue` are block exits and release the same way. A `defer` inside the scope block is chapter 3's `E0204` — defer is a direct function-body item only — and the scope-exit releases run before the enclosing function's own defers, the inner block exiting first. The statement produces no value: it is not an expression. A panic unwinding this block is a block exit of this guarantee: the releases run in reverse declaration order, before the enclosing function's defers — chapter 14's unwinding requirement extends the guarantee to unwound exits; every exit kind releases.

#### Scenario: A fresh construction released at exit

- **WHEN** `scope resource(f = openFile("data.txt")) { let n = f.read() }` appears in a function body and the block completes
- **THEN** at block exit `release` runs exactly once for `f`, in the implementation the impl of Releasable for FileHandle provides

#### Scenario: Reverse declaration order at exit

- **WHEN** a scope resource statement binds `a` then `b` and the block exits
- **THEN** `b`'s release runs before `a`'s

#### Scenario: Return pierces the scope

- **WHEN** the scope block contains an early `return` of a value
- **THEN** the releases run at that exit, before the function's value completes; the enclosing function's own defers run after, the inner block exiting first

#### Scenario: Break pierces the scope

- **WHEN** a scope resource statement appears in a loop body and the block contains `break`
- **THEN** the releases run at that exit, before the loop is left

#### Scenario: Panic unwinds the scope

- **WHEN** a panic fires while the scope block is in progress
- **THEN** the releases run at that exit per this requirement's guarantee, before the enclosing function's defers — chapter 14's unwinding requirement; every exit kind releases

#### Scenario: A non-resource head is rejected

- **WHEN** `scope resource(u = makeUser())` appears, `makeUser` returning a gc record
- **THEN** the compiler rejects it with `E1103:` scope resource head does not implement Releasable

#### Scenario: Defer inside the scope block

- **WHEN** `scope resource(f = openFile("a")) { defer { log() } }` appears — a defer as an item of the scope block
- **THEN** the compiler rejects it with chapter 3's `E0204`: defer placement; defer stays a direct function-body item

#### Scenario: At least one binding

- **WHEN** `scope resource() { }` appears — an empty binding list
- **THEN** the statement fits no production and chapter 2's unexpected-token diagnostic (`E0105`) reports the `)` that offers no binding

### Requirement: The release discipline

A resource binding's lifetime is linear and function-local. Within every function body, a binding of a resource type — a parameter, a `let` binding, a scope-resource head binding — is live from its binding point, and on every control path it MUST reach exactly one of three transfers: a move into a scope-resource head, `scope resource(x = f)`, where the head binding takes over and the source binding dies at the transfer; a `return` of the binding, where the obligation moves to the caller; or an argument pass at a call whose parameter declares that resource type, where the obligation moves to the callee — the signature carries it. A control path on which a live resource binding reaches the end of its scope otherwise is rejected with `E1104:` resource binding outside its release discipline; a use of a binding after its transfer point is the same rejection. `break` and `continue` end their inner blocks' scopes the same way: a let-bound resource not transferred before them is the unreleased fall-through. The module top level binds no resources: a top-level `let` of a resource type fits no channel and is rejected with `E1104:` resource binding outside its release discipline likewise. The analysis covers the function body's flow graph alone — every obligation that crosses a function boundary is written in a signature — so the discipline is locally decidable (chapter 0, Principle 1).

#### Scenario: A parameter discharges through a head transfer

- **WHEN** a fn declares a parameter `f` of a resource type and its body is `scope resource(x = f) { work(x) }`
- **THEN** the parameter's obligation is discharged: the head binding takes over, and `f` is dead after the transfer

#### Scenario: Unreleased fall-through is rejected

- **WHEN** a function body contains `let f = openFile("a")` and a control path reaches the end of the function without a transfer of `f`
- **THEN** the compiler rejects it with `E1104:` resource binding outside its release discipline, naming the binding and the path

#### Scenario: Use after transfer is rejected

- **WHEN** `f` is passed as an argument to a fn whose parameter declares `f`'s resource type, and a later statement reads `f`
- **THEN** the compiler rejects the read with `E1104:` resource binding outside its release discipline; the binding died at the argument pass

#### Scenario: Return carries the obligation up

- **WHEN** a function body's last statement is `return f`, `f` a live resource binding
- **THEN** the path is legal; the obligation moves to the caller, whose own paths are checked the same way

#### Scenario: The module top level binds no resources

- **WHEN** a module top-level `let f = openFile("a")` appears — a top-level binding of a resource type
- **THEN** the compiler rejects it with `E1104:` resource binding outside its release discipline; no channel exists outside function bodies

#### Scenario: Conditional paths each transfer

- **WHEN** a function body binds `f` and an `if`/`else` returns `f` on one arm while the other passes it to a fn whose parameter declares the type
- **THEN** every path transfers `f` exactly once; the function is accepted

### Requirement: One release trigger

Release has one trigger: the scope-exit machinery. User code MUST NOT call `release` directly — a method call naming `release` with a resource-typed receiver, in any expression position of user code, is rejected with `E1104:` resource binding outside its release discipline. Inside an impl of Releasable the method is being defined, its body ordinary code; a call naming `release` on a resource-typed receiver there is the same rejection. Early release is expressed by exiting the scope: a `return`, `break`, or `continue` releases at the boundary, and that is the only spelling. This closes the double-release gap: a binding under a scope head cannot be released twice, because the machinery's call is the only call.

#### Scenario: A direct release call is rejected

- **WHEN** `f.release()` appears in a function body, `f` a live resource binding
- **THEN** the compiler rejects it with `E1104:` resource binding outside its release discipline; the spelling for early release is an early scope exit

#### Scenario: Early exit is the early release

- **WHEN** a scope block opens `f` and a guard `return`s before the block's end
- **THEN** the release runs once at that exit; no manual call is needed or allowed

### Requirement: No aliasing of resource bindings

A resource binding is the one handle to the resource. A second binding to the same resource — `let g = f` or `var g = f` with `f` resource-typed — is rejected with `E1105:` resource binding rebound; an assignment `g = f` with either side resource-typed is the same rejection. A resource binding is `let`-shaped: `var f = openFile("a")`, a rebindable resource binding, is rejected with `E1105:` resource binding rebound likewise. The one sanctioned move is a transfer (The release discipline): it does not alias — it hands over and kills the source binding.

#### Scenario: A let alias is rejected

- **WHEN** `let g = f` appears, `f` a live resource binding
- **THEN** the compiler rejects it with `E1105:` resource binding rebound

#### Scenario: A var declaration of a resource type is rejected

- **WHEN** `var f = openFile("a")` appears — a rebindable binding of a resource type
- **THEN** the compiler rejects it with `E1105:` resource binding rebound; resource bindings are `let`-shaped

#### Scenario: Assignment into or out of a resource binding is rejected

- **WHEN** `g = f` appears with `f` resource-typed, or a resource-typed `g` is the target of any assignment
- **THEN** the compiler rejects it with `E1105:` resource binding rebound

### Requirement: Resources keep to binding positions

A resource type appears only in binding positions: a parameter, a `let` or scope-head binding, a function's return type. A resource type in a composite position — a tuple element, a gc record's field (a byval field is E0601's own rejection), a sum payload, a newtype's underlying type — is rejected with `E1106:` resource type in a composite position. A resource type as a generic argument is the same rejection at the instantiation site, whatever the generic's uses of the parameter — `Box<FileHandle>` and `id<FileHandle>` alike; a generic body cannot carry a maybe-resource obligation locally decidable, so the binding-position refinement is reserved as an amendment of this chapter. The box route is closed with it: `Dyn<Releasable>` in a type position and the construction `Dyn<Releasable>(r)` with a resource-typed argument are the same rejection — under the completeness obligation every box of Releasable would box a resource, and the box is a shared-handle composite. The sibling rulings already in force carry the same reach argument: a closure MUST NOT capture a resource binding (`E1002`, chapter 12), and a resource type MUST NOT implement `Iterable` (`E0903`, chapter 11) — a handle with more than one path to it outlives the single deterministic release point the discipline depends on.

#### Scenario: A gc record field is rejected

- **WHEN** `record Conn { file: FileHandle }` appears — a gc record with a resource-typed field
- **THEN** the compiler rejects it with `E1106:` resource type in a composite position

#### Scenario: A sum payload is rejected

- **WHEN** a `type` declaration's variant carries a `FileHandle` payload
- **THEN** the compiler rejects it with `E1106:` resource type in a composite position

#### Scenario: A tuple element is rejected

- **WHEN** a function declares a parameter of type `(FileHandle, Int64)`
- **THEN** the compiler rejects it with `E1106:` resource type in a composite position

#### Scenario: A newtype underlying type is rejected

- **WHEN** `newtype WrappedFile of FileHandle with Releasable` appears
- **THEN** the compiler rejects it with `E1106:` resource type in a composite position; the newtype wraps values, not resources

#### Scenario: A generic instantiation is rejected

- **WHEN** `Box<FileHandle>` or `id<FileHandle>` appears as a type expression, the latter with `id`'s parameter in binding positions only
- **THEN** the compiler rejects it with `E1106:` resource type in a composite position, at the instantiation; the binding-position refinement is a reserved amendment

#### Scenario: A Dyn box of Releasable is rejected

- **WHEN** `Dyn<Releasable>(f)` appears with `f` resource-typed, or `Dyn<Releasable>` appears as an annotation's type
- **THEN** the compiler rejects it with `E1106:` resource type in a composite position; the box shares handles, the discipline does not

### Requirement: Resources diagnostics segment

The resources chapter owns registry segment `E1100`–`E1199`, declared in `docs/spec/diagnostics.toml` `[segments]` with owner `1300-resources`. Allocations: `E1101` resource record must implement Releasable, `E1102` Releasable implemented by a non-resource type, `E1103` scope resource head does not implement Releasable, `E1104` resource binding outside its release discipline, `E1105` resource binding rebound, `E1106` resource type in a composite position. `E1100` and `E1107`–`E1199` remain reserved for amendments of this chapter — a later chapter needing segment codes claims its own segment.

#### Scenario: The segment is retrievable

- **WHEN** a diagnostics consumer looks up any `E11xx` code in the registry
- **THEN** the segment entry names owner `1300-resources`, and each allocated code's entry carries severity, title, description, remediation, owner, requirement, and allocated date

#### Scenario: Later changes extend within the segment

- **WHEN** a later spec-layer change of this chapter needs a new code
- **THEN** it allocates from `E1100`–`E1199` in its own change; a chapter outside resources claims a different segment

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–13. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits.

### Releasable and its obligations

```we
byres record FileHandle { fd: Int64 }    // a byres record: the category
                                         // carries the impl obligation
impl Releasable for FileHandle {
    fn release(mut self) { close(self.fd) }  // mut self, no return type
}

byres record LogFile { fd: Int64 }       // E1101: resource record must
                                         // implement Releasable — no impl
impl Releasable for User { ... }         // E1102: Releasable implemented
                                         // by a non-resource type
impl Releasable for FileHandle {
    fn release(mut self) -> Bool { ... } // E0808: signature mismatch
}
```

### The scope resource statement

```we
fn countLines(name: String) -> Int64 {
    scope resource(f = openFile(name)) { // head implements Releasable
        return f.read()                  // return pierces: releases run
    }                                    // once, before the defers of the
}                                        // enclosing function body run

fn two() {
    scope resource(a = openFile("a"), b = openFile("b")) {
        work(a)
        work(b)
    }                                    // at exit: b released, then a
}
```

### The release discipline: three channels, function-local

```we
fn copy(src: FileHandle) -> FileHandle { // the parameter is a channel: the
    scope resource(s = src) {            // signature carries the obligation
        return makeOut()                 // return: caller takes the value
    }
}

fn useAndPass(f: FileHandle) {           // parameter: obligation starts here
    log(f.read())                        // reading is ordinary use
    sink(f)                              // argument pass: sink's parameter
}                                        // declares the type; f dies here

fn leaky() {
    let f = openFile("a")                // E1104: resource binding outside
    log(f.read())                        // its release discipline — the
}                                        // path ends with no transfer

let f = openFile("a")                    // E1104 at the top level: the
                                         // module top level binds no
                                         // resources
```

### One trigger, no aliases, binding positions only

```we
fn guarded(name: String) -> Int64 {
    scope resource(f = openFile(name)) {
        if empty(name) { return 0 }      // early exit is the early release
        return f.read()                  // no f.release() anywhere: a
    }                                    // direct call is E1104
}

fn aliases(f: FileHandle) {
    let g = f                            // E1105: resource binding rebound
    var h = openFile("a")                // E1105: resource bindings are
}                                        // let-shaped

record Conn { file: FileHandle }         // E1106: resource type in a
type Wrapped = Open(FileHandle)          // composite position — the field,
newtype WrappedFile of FileHandle with Releasable  // the payload, the under-
let boxed: Box<FileHandle> = pack(f)     // lying, the generic argument,
let d = Dyn<Releasable>(f)               // the Dyn box: never boxed; the
                                         // box shares handles
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| resource | 资源 |
| release | 释放 |
| release contract | 释放契约 |
| declaration completeness | 声明完备性 |
| scope resource statement | scope resource 语句 |
| head expression | 头表达式 |
| head binding | 头绑定 |
| block exit | 块出口 |
| pierce | 穿透 |
| scope-exit machinery | 块出口机制 |
| release discipline | 释放纪律 |
| linear | 线性 |
| function-local | 函数局 |
| transfer | 转移 |
| channel | 通道 |
| obligation | 义务 |
| use after transfer | 转移后使用 |
| release trigger | 释放触发点 |
| early release | 早释放 |
| double release | 双重释放 |
| aliasing | 别名 |
| binding position | 绑定位 |
| composite position | 组合位 |
| boxing | 装箱 |
