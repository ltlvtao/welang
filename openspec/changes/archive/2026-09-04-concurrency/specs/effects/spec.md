## MODIFIED Requirements

### Requirement: The effect segment

A fn declaration and an interface method signature MAY carry an effect segment between the parameter list and the arrow (or the body, when no return type is written): `effect tag1 tag2 ...` — the `effect` keyword introducing one or more space-separated tags. Omission states a pure function; a segment states the function may perform exactly those effects. Within a function body — defer bodies included, which run at exit but belong to the enclosing function's own extent — every call's effect set MUST be a subset of the declared set; a call requiring an effect outside it MUST be rejected with `E1401:` undeclared effect at a call, naming the effect and the callee. Defer shifts execution timing only, never effect attribution: a defer body's calls count toward the enclosing function's declared effects (chapter 3's pointer, landed). The panic family of chapter 14 carries no effect: termination is not a side effect, and `panic`, `todo`, and `assert` are callable from any function. The concurrency chapter's task block carries this requirement's segment under the spelling `task effect tag1 tag2 ... block` — REQUIRED there and never optional: a task runs outside every enclosing extent, its body's calls answering its own declared set, never an enclosing declaration's. The concurrency chapter's waits carry no effect either: blocking on a lock, a condition, a semaphore permit, a channel, a task's completion, or a cancellation signal reads no environment and writes none — waiting is not a side effect, the primitives' waiting methods are callable from any function, and a scope block's whole extent answers no enclosing declaration.

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

#### Scenario: A task block's segment is this requirement's spelling

- **WHEN** `task effect net { fetchA() }` appears with `fetchA` declaring `effect net`
- **THEN** the segment is this requirement's — `effect` plus bare tags — and the body's calls are checked against the task's own declared set by the concurrency chapter's rules

#### Scenario: Waiting carries no effect

- **WHEN** a function declaring no effect segment holds `slot.acquire()` or `ch.send(v)`
- **THEN** no effect diagnostic fires; waiting on the concurrency primitives is not a side effect
