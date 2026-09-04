## ADDED Requirements

### Requirement: Effect declarations and the built-in tags

An effect declaration is a top-level item `effect name`, optionally prefixed by `pub`. The name is camelCase under chapter 1's naming conventions (`E0012`) and joins the module's one name space under chapter 6, colliding with no other name (`E0404`). The built-in tags are `io`, `net`, and `time` — language-level names, not module items; a custom effect MUST NOT take a built-in's name and MUST be rejected with `E1403:` effect name conflicts with a built-in effect. Effect names resolve under chapter 15's name resolution like every other name: bare within the declaring module, qualified as `mod.tag` through an import when pub. The check an effect tag participates in never depends on which module declared it or whether it is built-in — one rule for all tags.

#### Scenario: A custom effect declares and joins the name space

- **WHEN** `effect db` appears at the top level
- **THEN** it declares the effect `db`, camelCase, in the module's one name space — a second `effect db` or a `fn db` in the same module is `E0404:` duplicate name in one module

#### Scenario: A built-in name is refused for a custom effect

- **WHEN** `effect io`, `effect net`, or `effect time` appears
- **THEN** the compiler rejects it with `E1403:` effect name conflicts with a built-in effect

#### Scenario: Built-in tags are not module names

- **WHEN** a module declares `fn time(n: Int64) -> Int64` or binds `let net = 3`
- **THEN** both are legal: built-in tags are language-level, not module items, so they occupy no name in the module's name space — `E1403` governs declaring an effect under a built-in's name, never using that word as an ordinary name

#### Scenario: A pub effect reaches cross-module qualified

- **WHEN** module B declares `pub effect db` and module A imports B
- **THEN** A's signatures qualify the tag as `b.db`; the check is the same one that governs a bare tag in its own module

#### Scenario: An unresolved tag is rejected

- **WHEN** an effect segment holds a tag that no built-in, declaration, or qualified import provides
- **THEN** the compiler rejects it with `E1304:` unresolved name under chapter 15's resolution

### Requirement: The effect segment

A fn declaration and an interface method signature MAY carry an effect segment between the parameter list and the arrow (or the body, when no return type is written): `effect tag1 tag2 ...` — the `effect` keyword introducing one or more space-separated tags. Omission states a pure function; a segment states the function may perform exactly those effects. Within a function body — defer bodies included, which run at exit but belong to the enclosing function's own extent — every call's effect set MUST be a subset of the declared set; a call requiring an effect outside it MUST be rejected with `E1401:` undeclared effect at a call, naming the effect and the callee. Defer shifts execution timing only, never effect attribution: a defer body's calls count toward the enclosing function's declared effects (chapter 3's pointer, landed). The panic family of chapter 14 carries no effect: termination is not a side effect, and `panic`, `todo`, and `assert` are callable from any function.

#### Scenario: A segment declares and checks

- **WHEN** `fn write(msg: String) effect io { save(msg) }` appears and `save` declares `effect io`
- **THEN** the segment parses, `save`'s set is within the declared set, and the declaration is legal

#### Scenario: A segment carries several tags

- **WHEN** `fn sync(rows: List<Int64>) effect io net { send(persist(rows)) }` appears with `send` declaring `effect net` and `persist` declaring `effect io`
- **THEN** the two-tag segment parses and both calls are within the declared set; declaring more than the body performs is legal, performing beyond the segment is not

#### Scenario: An undeclared effect at a call is rejected

- **WHEN** `fn greet() { save("hi") }` appears and `save` declares `effect io`
- **THEN** the compiler rejects it with `E1401:` undeclared effect at a call — io is not in the declared set; the fix is declaring the segment or calling a pure function

#### Scenario: Omission states purity

- **WHEN** a fn declaration carries no effect segment
- **THEN** its body may call only pure functions; any effectful call inside is `E1401:` undeclared effect at a call

#### Scenario: A segment without a return type

- **WHEN** `fn log(msg: String) effect io { save(msg) }` appears — segment present, no `->` type
- **THEN** the segment sits between the parameter list and the body and parses; the absence of a return type is independent of the segment

#### Scenario: A defer body's effects attribute to the enclosing function

- **WHEN** a function declaring no effects holds `defer { save("x") }` and `save` declares `effect io`
- **THEN** the compiler rejects it with `E1401:` undeclared effect at a call; defer shifts timing, never effect attribution — the enclosing function must declare io or the defer body must call pure code

#### Scenario: The panic family is effect-free

- **WHEN** a pure function's body calls `panic("unreachable")` or `assert(n > 0, "positive")`
- **THEN** no effect diagnostic fires; termination is not a side effect

### Requirement: The effect segment in function types

A function type's effect segment sits between the parameter types and the arrow as bare space-separated tags — no `effect` keyword in type position: `fn(T1, ..., Tn) tag1 tag2 -> T`. Omission states the pure function type. The split spelling is the ratified one: declarations introduce their segment with `effect`, types carry bare tags — the two surfaces keep their own introducers, and neither spelling is accepted in the other position (`E0105`). At every type-agreement position holding a function value — binding against an annotation, argument against parameter, return against the declared type — the value's effect set MUST be a subset of the expected type's set; a value requiring an effect the expected type does not declare MUST be rejected with `E1402:` function value effect set does not match the expected type's. The subset direction is deliberate and one-way: a purer value satisfies an effect-expecting slot — it can only do less — while an effectful value in a pure slot is `E1402`'s.

#### Scenario: A type segment parses bare

- **WHEN** an annotation holds `fn(Int64) io -> Int64`
- **THEN** it is the type of Int64-to-Int64 functions that may perform io, per chapter 12 as amended

#### Scenario: An effectful value in a pure slot is rejected

- **WHEN** a closure whose body calls io-declaring code, or the io-declaring `save` itself as a fn name, is bound as `let f: fn(Int64) -> Int64 = ...`
- **THEN** the compiler rejects it with `E1402:` function value effect set does not match the expected type's — io is not in the expected set; the rule binds function values however they are named

#### Scenario: Calling an effectful-typed parameter counts as declared

- **WHEN** `fn apply(f: fn(Int64) io -> Int64) { f(3) }` appears — the parameter's type declares io, and `apply` declares nothing
- **THEN** the compiler rejects it with `E1401:` undeclared effect at a call — the call through `f` performs what `f`'s type declares; written `fn apply(f: fn(Int64) io -> Int64) effect io -> Int64 { f(3) }` it is legal, the segment and the call balancing

#### Scenario: A pure value satisfies an effect-expecting slot

- **WHEN** `let f: fn(Int64) io -> Int64 = |n| n + 1` appears — the closure's inferred set is empty
- **THEN** the binding is legal; the empty set is a subset of io, and doing less is always safe

#### Scenario: Neither spelling crosses positions

- **WHEN** a fn declaration writes bare tags (`fn f() io { }`) or a function type writes the keyword (`fn(Int64) effect io -> Int64`)
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic (`E0105`); each surface keeps its own introducer

### Requirement: Closure effect sets are inferred

Both closure forms — the full `fn(params) -> type block` and the short `|p| expr` — carry no effect segment to write; their effect sets are inferred from the body's calls and checked at every agreement position under the subset rule (`E1402`). A closure's inferred set is the union of its body's calls' sets, exactly as its value type is fixed by its body under chapter 12 — one inference rule for both forms, no annotation site exists or is added.

#### Scenario: A full closure's set is inferred from its body

- **WHEN** `let f = fn(n: Int64) -> Int64 { write(log, n); n + 1 }` appears and `write` declares `effect io`
- **THEN** `f`'s type carries the inferred set {io}: `fn(Int64) io -> Int64` under chapter 12 as amended

#### Scenario: A short closure's set is inferred the same way

- **WHEN** `items.map(|s| parse(s))` appears with `parse` pure — the short form checks exactly as the full one
- **THEN** the closure's set is empty; had `parse` declared an effect, the same union rule would carry it

#### Scenario: Inference is checked at the agreement position

- **WHEN** the io-carrying `f` above is passed where `fn(Int64) -> Int64` is expected
- **THEN** the compiler rejects it with `E1402:` function value effect set does not match the expected type's

#### Scenario: Constructing an effectful closure is not performing it

- **WHEN** a pure function's body holds `let f = |s: String| { save(s) }` — constructing the io-inferring closure without calling it — and returns `f`
- **THEN** the pure declaration is legal: the inferred set rides the value, it does not taint the constructing function; only a call — `save` directly, `f("x")` inside, or `f` escaping to an effectful-typed slot — participates in any check

### Requirement: Effects in interfaces and impls

An interface method signature MAY carry an effect segment — declaration spelling, `effect` keyword — and an impl method's effect set MUST equal the interface declaration's exactly, missing and extra tags alike rejected with `E1404:` impl method effect set disagrees with the interface. A default method's segment (chapter 10) governs its body and every override's, the same exact-match rule binding an override to the signature it replaces. Calls through an interface value — a `Dyn<Interface>` box or a generic bound — are checked against the interface-declared set: static dispatch has no effect blind spot, whichever implementation runs.

#### Scenario: An interface method declares a segment

- **WHEN** `interface Store { fn get(mut self, k: String) effect io -> String }` appears
- **THEN** the signature parses with its segment; implementations are bound to exactly {io}

#### Scenario: An impl set must match exactly

- **WHEN** the impl of `Store` for `FileStore` declares `fn get(mut self, k: String) effect net -> String`
- **THEN** the compiler rejects it with `E1404:` impl method effect set disagrees with the interface — net is not io, extra and missing tags alike disagree

#### Scenario: A default-method override matches the segment too

- **WHEN** an interface declares `fn reload(mut self) effect io` with a default body and an impl overrides it declaring no segment, its body calling io-declaring code
- **THEN** the override is rejected: `E1404` for dropping the interface's io from the signature, and the impl signature being pure, `E1401` for its effectful body — the override replaces the default under the same exact-match rule

#### Scenario: A segmentless impl of a segmentless interface is pure

- **WHEN** the interface method declares no segment and the impl's body calls io-declaring code
- **THEN** the call is `E1401:` undeclared effect at a call; the impl signature is pure and its body must be

#### Scenario: A call through the interface checks the interface's set

- **WHEN** a `Dyn<Store>` value's `get` is called inside a function declaring `effect io`
- **THEN** the call is legal — the interface-declared set is what the caller sees, whichever implementation runs

### Requirement: Effect checking at module initialization

Top-level let initializers are the one ratified position that evaluates outside any function signature, so their rule is fixed here: initializers MUST be pure. An initializer calling an effectful function MUST be rejected with `E1405:` effectful call in a top-level initializer — process-start work belongs in `main`, keeping chapter 15's eager initialization deterministic in content as well as order (chapter 0, Principle 1).

#### Scenario: A pure initializer is legal

- **WHEN** `pub let retries = 3` or `pub let name = "svc"` appears at the top level
- **THEN** the initializer is legal; literals and pure calls always are

#### Scenario: An effectful initializer is rejected

- **WHEN** `pub let host = loadHost()` appears and `loadHost` declares `effect io`
- **THEN** the compiler rejects it with `E1405:` effectful call in a top-level initializer — move the work into main

#### Scenario: Initialization order stays content-deterministic

- **WHEN** every module's initializers are pure
- **THEN** process start performs no environmental reads or writes; module initialization is a deterministic computation over constants, whichever platform runs it

### Requirement: The effects diagnostics segment

The effects chapter owns registry segment `E1400`–`E1499`, declared in `docs/spec/diagnostics.toml` `[segments]` with owner `1600-effects`. Allocations: `E1401` undeclared effect at a call, `E1402` function value effect set does not match the expected type's, `E1403` effect name conflicts with a built-in effect, `E1404` impl method effect set disagrees with the interface, `E1405` effectful call in a top-level initializer. `E1400` and `E1406`–`E1499` are reserved for this chapter's amendments; later chapters needing segment codes claim their own segments.

#### Scenario: The segment is retrievable

- **WHEN** a diagnostics consumer looks up any `E14xx` code in the registry
- **THEN** the segment entry names owner `1600-effects`, and each allocated code's entry carries severity, title, description, remediation, owner, requirement, and allocated date

#### Scenario: Later changes extend within the segment

- **WHEN** a later spec-layer change to this chapter needs a new code
- **THEN** it allocates from `E1400`–`E1499` within its own change; chapters outside effects claim other segments
