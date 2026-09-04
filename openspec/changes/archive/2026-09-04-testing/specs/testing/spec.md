## ADDED Requirements

### Requirement: Test modules and test blocks

A test module is a source file whose name ends `_test.we` — a module-identity fact the compiler decides from the file's name, as it decides the module's path from it. Inside a test module, a test block is `test "description" { body }`: the `test` keyword, exactly one string literal — the test's description, carried for reports and carrying no semantics of its own — and a chapter-2 block. A test block is a top-level item per chapter 6's amended enumeration, legal only in a test module: in any other file the same form MUST be rejected with `E1801:` test block outside a test module. A test block carries no `pub` — it is not one of chapter 6's ratified pub kinds, and the prefix is rejected there as anywhere (`E0105`, chapter 6). A test block declares no effect segment; chapter 16's amendment fixes what that means for the calls inside. The body is a function body context per chapter 12's amended enumeration — bare `return` ends the test early, `return` with a value is rejected as in any valueless function (`E0402`), and `defer` runs at the body's exit under chapter 3. A test module is otherwise an ordinary module under chapter 15: imports, one name space, cross-module initialization, pub discipline — unchanged; where test files live under a project root and how their imports map to it is the toolchain's, not this specification's. Each test block runs to its own end or to this chapter's panic boundary; the order of test blocks within a file, across files, and any parallelism between them is not fixed here — the toolchain chapter's.

#### Scenario: A test block parses as a top-level item

- **WHEN** the file `math_test.we` holds `test "absolute value" { let _ = abs(-3) }` among its imports and helper fns
- **THEN** the block parses as one top-level item of a test module per this chapter; the description string and the body follow this requirement's forms

#### Scenario: A test block outside a test module is rejected

- **WHEN** `test "x" { }` appears in a file whose name does not end `_test.we`
- **THEN** the compiler rejects it with `E1801:` test block outside a test module — the block is the item it is, and the module it sits in is not a test module; the file's name decides

#### Scenario: pub on a test block is rejected

- **WHEN** `pub test "x" { }` appears in a test module
- **THEN** the compiler rejects it with `E0105:` unexpected token per chapter 6 — a test block is not a ratified pub kind

#### Scenario: A test module is an ordinary module

- **WHEN** a test module imports `std.test` and the module under test, declares helper fns and top-level bindings, and its tests call both
- **THEN** every chapter 15 and chapter 6 rule holds as for any module; nothing about the `_test.we` name changes resolution, initialization, or visibility

#### Scenario: The shape of one test's run is fixed, the run's shape is not

- **WHEN** a test module holds two test blocks
- **THEN** each runs to its own end or its panic boundary; which runs first, and whether they share a process, is the toolchain's — this chapter fixes what one test observes, not the order of tests

### Requirement: Mock declarations

Inside a test block's body — as a direct item of it — a mock declaration is `mock name(params) effect tag1 tag2 ... { body }`, `mock name(params) -> type effect tag1 tag2 ... { body }`, or the same forms with no segment: the `mock` keyword, the target name, a parameter list, an optional declared return, an optional effect segment, and a body. The target name is the function being mocked: a bare name resolving to this module's own top-level fn, or a qualified `mod.fn` resolving to an imported module's pub fn — chapter 15's resolution throughout, `E1304` when the name resolves nowhere and `E1303` when it resolves to another module's non-pub item, as anywhere. A mock declaration anywhere but a direct item of a test block body MUST be rejected with `E1802:` mock declaration outside a test block. The declaration's signature — every parameter with its type, in order and by name, the declared return or its absence, and the effect segment or its absence — MUST match the target declaration verbatim; a mismatch MUST be rejected with `E1803:` mock signature does not match its target — a mock restates the target's contract, it does not redesign it. The target MUST be a module-level monomorphic fn: a generic fn, an impl or interface method, a constructor, or any name that is not a plain fn declaration MUST be rejected with `E1804:` mock target is not a mockable function — the mock's one-shape story is the ordinary call, and generics, receivers, and field initialization have no ratified mock semantics. One test block mocks one target at most once: a second mock of the same target in the same block MUST be rejected with `E1805:` duplicate mock of one target in a test block. A mock in force means calls resolve by name: every call site of the target name inside that test block — nested blocks, task and scope bodies included — resolves to the mock's body instead of the target's. Interception is by name, not by proxy: the target's own body is untouched and may never run — a mock of `f` does not mock what `f` calls. The mock's body is a function body context checked against the declared segment: chapter 16's `E1401` applies there as in any function, the segment being the target's own verbatim.

#### Scenario: A mock of an own-module function intercepts its calls

- **WHEN** a test module's test block holds `mock readCount() -> Int64 effect { 7 }` and the code under test calls `readCount()`
- **THEN** the call resolves to the mock's body; the test observes 7, and the target's body — a foreign call, a file read, anything — never runs

#### Scenario: A mock of an imported pub function

- **WHEN** the target is `pub fn find(id: Int64) -> Option<User> effect io` of an imported module and the test block holds `mock user.find(id: Int64) -> Option<User> effect io { Some(User { id: id }) }`
- **THEN** the qualified target resolves per chapter 15 and the interception covers `user.find(...)` call sites within that block; the pub rule is the only road in, as for any cross-module reach

#### Scenario: A mock outside a test block is rejected

- **WHEN** `mock f() { }` appears at a module's top level or inside a plain fn body
- **THEN** the compiler rejects it with `E1802:` mock declaration outside a test block — mocks exist only as direct items of a test body

#### Scenario: A signature mismatch is rejected

- **WHEN** the target declares `fn find(id: Int64) -> Option<User> effect io` and the mock writes `mock find(name: String) -> Option<User> effect io { ... }`
- **THEN** the compiler rejects it with `E1803:` mock signature does not match its target — parameters, return, and segment are restated, not redesigned

#### Scenario: An unmockable target is rejected

- **WHEN** a mock names a generic fn (`fn id<T>(x: T) -> T`), an impl method, or a constructor such as `User`
- **THEN** the compiler rejects it with `E1804:` mock target is not a mockable function — only module-level monomorphic fns are mockable

#### Scenario: A second mock of one target is rejected

- **WHEN** one test block holds two mocks of `readCount`
- **THEN** the compiler rejects the second with `E1805:` duplicate mock of one target in a test block — one block, one target, one mock

#### Scenario: Interception is by name, not by proxy

- **WHEN** `f` calls `g` internally and a test mocks `f` only
- **THEN** calls to `g` are unaffected, wherever they appear — a mock of `f` mocks the name `f`, not the call graph under it

#### Scenario: A mock's force ends with its block

- **WHEN** one test block mocks `readCount` and a second test block in the same module calls `readCount()` with no mock of its own
- **THEN** the second block's call reaches the real `readCount` — the mock's force is its own block's, and each block mocks for itself

### Requirement: The virtual clock

Inside a test block — the block's own body and every task and scope body within it — every call whose effect set carries the `time` tag runs on the virtual clock: no wall time passes while such a call waits, and the clock the standard library's time functions report advances only at `advanceTime` and at waits registered upon the clock. The virtualization is total for `time` and honest about the rest: `io` and `net` are not virtualized — an unmocked io or net call in a test is a real call, reading the real file, opening the real socket — and an unmocked custom effect runs truly as well; the road to isolation is the mock, and this specification says so plainly rather than defaulting silently. A scope block's `timeout` clause inside a test block reads the virtual clock: the budget expires by `advanceTime` crossing it, never by waiting, so a timeout path is exercised deterministically.

#### Scenario: time-tagged calls observe the virtual clock

- **WHEN** a test calls the standard library's clock twice with no `advanceTime` between
- **THEN** the two reads report the same instant — the clock moves only where this chapter says it moves

#### Scenario: An unmocked io call is a real call

- **WHEN** a test reads a file through an unmocked io-tagged function
- **THEN** the read touches the real filesystem; isolation comes from mocking the function, and nothing here pretends otherwise

#### Scenario: A scope timeout fires under advanceTime

- **WHEN** a test block's scope carries `timeout(100)` and the test advances the virtual clock by 100 with the budget otherwise unspent
- **THEN** the timeout path runs — the budget was read from the virtual clock, and the path reproduces on every run

#### Scenario: The clock advances at exactly two places

- **WHEN** a test holds time-tagged calls and `advanceTime` calls
- **THEN** the reported instants change only across `advanceTime` and across waits registered on the clock — no other construct moves it

### Requirement: advanceTime

`advanceTime(d: Int64)` advances the virtual clock by `d` — the unit the standard library's clock functions report — releasing, in deterministic order, every wait registered at or before the crossed instant. The name is prelude per chapter 15's amendment, and it is legal only inside a test block's body — task and scope bodies included, for they are the test's own extent; anywhere else it MUST be rejected with `E1806:` advanceTime called outside a test block. The name is this language's second implicit acquisition, and chapter 18's criterion binds it: this requirement answers all four of that criterion's conditions against its text. One — non-business data: the virtual clock is the test runtime's own control token, carrying no user-defined semantics. Two — lifetime forced: the clock is bound to the test block; there is no handle to hold past the block and no way for it to be missing inside. Three — missing statically decidable: use outside a test body is `E1806`, a compile error, never a runtime default or an absent value in disguise. Four — no competing path: no constructor or parameter form of the test clock exists — the clock is not a value, and no caller ever chooses between two spellings of it.

#### Scenario: advanceTime advances and releases waits

- **WHEN** a task in the test waits for a duration registered on the virtual clock and the test calls `advanceTime` past it
- **THEN** the wake happens at the crossing, in deterministic order against waits tied at the same instant

#### Scenario: advanceTime outside a test block is rejected

- **WHEN** a plain fn of a test module — outside any test block — calls `advanceTime(5)`
- **THEN** the compiler rejects it with `E1806:` advanceTime called outside a test block; the prelude carries the name, this chapter fixes where it is legal

#### Scenario: advanceTime answers the four conditions

- **WHEN** this requirement is measured against chapter 18's implicit-acquisition criterion
- **THEN** all four conditions hold — a control token, bound to the test block, statically rejected outside it, with no competing spelling — and the criterion's scenario for later candidates is discharged here

### Requirement: Deterministic scheduling in test mode

Inside test mode — a test block and every concurrent structure it holds — interleaving is deterministic: the same test source with the same sequence of `advanceTime` calls yields the same observable interleaving on every run. Which interleaving is the one is not fixed here: the deterministic order of tied wakes and the scheduling policy producing it are the runtime's own — what this specification promises is reproduction, not any particular order. This is the one refinement of chapter 18's scheduling promises, by that chapter's amendment: outside test mode, the order of task execution and the interleaving of concurrent tasks remain unspecified exactly as that requirement fixed them.

#### Scenario: The same test interleaves the same way twice

- **WHEN** a test creating tasks and channels runs twice under one toolchain
- **THEN** the observable interleaving — the order of sends, receives, and completions — is identical between the runs

#### Scenario: Tied wakes resolve deterministically

- **WHEN** two waits are registered for the same virtual instant and the clock crosses it
- **THEN** the order of their wakes is fixed for that test source, whatever order the runtime's policy defines

#### Scenario: The promise is reproduction, not an order

- **WHEN** two runtimes run the same test
- **THEN** each produces its own one fixed interleaving, identical across its own runs — this chapter does not promise the two agree, only that each is self-consistent

### Requirement: Panic at the test boundary

A panic or `todo` unwinding inside a test block follows chapter 14's rule inside the block: blocks exit, scope-resource bindings release exactly once each in reverse order, defers run in reverse statement order, and no expression of the language observes the flight. The test boundary is a capture boundary — the third this specification defines, beside the process abort and the task boundary: at the boundary the flight converts into that test's failure; the process does not abort, and the run continues past the failed test. A test's outcome is not a language value — no expression reads it; the observer is the test runtime, as the joining scope is the task boundary's observer. `assert`'s failure is a panic per chapter 14, and a failed assertion fails its test through this one route — there is no second failure mechanism (chapter 0, Principle 9).

#### Scenario: A panic fails the test, not the process

- **WHEN** a test block's body panics
- **THEN** the test ends as a failure at the boundary and the run continues to the next test; no abort happens

#### Scenario: Resources release before the boundary

- **WHEN** the panicking test holds scope-resource bindings and defers
- **THEN** the releases and defers run under chapter 14's unwinding order inside the block, before the flight ends at the test boundary

#### Scenario: A failed assert is a failed test

- **WHEN** a test's `assert(cond, msg)` finds `cond` false
- **THEN** the panic it panics per chapter 14 crosses no expression, ends at the test boundary, and the test fails — one route, no second mechanism

### Requirement: Assertions and the standard-library test surface

`assert(cond, msg)` is the prelude's own — chapter 14's termination family, callable in a test as anywhere. The wider assertion family — `assertEqual` and its kin — is standard-library surface, not grammar: the functions live in `std.test` and are reached by import alone, and this chapter adds no syntax for them. Parameterized testing is a for loop over a table with a helper fn — surface chapters 3 and 6 ratified long ago; property testing is standard-library combinator closures over generators — chapter 12's surface. v0.8's `test_each ... with [...] as (a, b, expected)` and `property "..." for_all (a: Int64, b: Int64)` forms are not ratified: both are sugar over ratified surface, and sugar does not open a chapter (chapter 0, Principle 2).

#### Scenario: assert is callable in a test

- **WHEN** a test body calls `assert(n == 7, "n was not 7")`
- **THEN** the call is the prelude's chapter-14 function; its failure is a panic and fails the test at the boundary

#### Scenario: The assertEqual family is std.test surface

- **WHEN** a test wants `assertEqual(got, want)` and writes `import std.test` first
- **THEN** the call resolves as any imported pub fn; no grammar of this chapter is involved, and the missing import is `E1304` as anywhere

#### Scenario: Parameterized and property tests use ratified forms

- **WHEN** a table of cases and a for loop drive one body, or a generator closure feeds a property check
- **THEN** every form involved is chapters 3, 6, and 12 surface; this chapter's syntax is `test` and `mock` and nothing more

### Requirement: What testing does not fix

This chapter names what it leaves open. The execution order of test blocks within a file and across files, parallelism between them, filtering, reporting formats, exit codes, the `tests/` directory layout, and the mapping of a test module's imports to the project's source root — all the toolchain chapter's; this chapter fixes what one test observes, not the shape of a run. Exploration — v0.8's `--explore` with its iteration controls and partial-order reduction, probing interleavings the deterministic scheduler would not choose — is toolchain-layer business, as are its guard diagnostics (v0.8's W0601, E1001, E1003); this specification fixes the observable determinism such tools probe, honestly bounded: deterministic reproduction is a promise, exhaustive coverage of interleavings is not one and is claimed nowhere. An unmocked custom effect in a test is a real call — stated plainly; whether a toolchain warns about it is the toolchain's, and no warning exists at the spec layer.

#### Scenario: The shape of a run is the toolchain's

- **WHEN** a project's tests run under a toolchain
- **THEN** order, parallelism, filtering, reporting, and exit codes follow that chapter; nothing here constrains them beyond each test's own determinism

#### Scenario: Exploration is the toolchain's

- **WHEN** a tool explores alternative interleavings of a test
- **THEN** it probes above this chapter's determinism promise; the spec layer fixes reproduction, never exhaustive coverage, and the guard diagnostics of v0.8's explore mode are the toolchain layer's

#### Scenario: Unmocked custom effects run truly

- **WHEN** a test calls a function of a custom effect tag and mocks nothing on its path
- **THEN** the call executes truly; isolation is the mock's to give, and this chapter says so rather than defaulting silently

### Requirement: Testing diagnostics segment

The testing chapter owns the registry segment `E1800`–`E1899`, declared in `docs/spec/diagnostics.toml` under `[segments]`. Allocation: `E1801` test block outside a test module, `E1802` mock declaration outside a test block, `E1803` mock signature does not match its target, `E1804` mock target is not a mockable function, `E1805` duplicate mock of one target in a test block, `E1806` advanceTime called outside a test block. `E1800` and `E1807`–`E1899` are reserved for this chapter's amendments. Trigger semantics live in this chapter's requirements; the entries live in the registry.

#### Scenario: A testing code is emitted

- **WHEN** the toolchain emits any `E18xx` diagnostic
- **THEN** its full entry is retrievable from `docs/spec/diagnostics.toml` under owner `2000-testing`

#### Scenario: A later change needs this segment's codes

- **WHEN** a future amendment of this chapter needs a new diagnostic
- **THEN** it extends the registry within `E1800`–`E1899` in the same change, or claims its own segment
