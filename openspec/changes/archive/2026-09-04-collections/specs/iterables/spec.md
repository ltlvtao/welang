## ADDED Requirements

### Requirement: Iterator combinators

The `Iterator` interface declares eleven default methods — the combinators — fixed by this requirement and provided by the standard library's default bodies: an impl inherits them as chapter 10's default methods, and an override MUST repeat the interface signature exactly, generic clause included (`E0808`). The lazy family performs no element work at the call and returns a derived iterator: `fn map<U>(mut self, f: fn(T) -> U) -> Dyn<Iterator<U>>`, `fn filter(mut self, f: fn(T) -> Bool) -> Dyn<Iterator<T>>`, `fn take(mut self, n: Int64) -> Dyn<Iterator<T>>`, `fn skip(mut self, n: Int64) -> Dyn<Iterator<T>>`. The eager family advances the receiver to exhaustion at the call and returns its result: `fn collect(mut self) -> List<T>`, `fn fold<U>(mut self, init: U, f: fn(U, T) -> U) -> U`, `fn reduce(mut self, f: fn(T, T) -> T) -> Option<T>`, `fn count(mut self) -> Int64`, `fn any(mut self, f: fn(T) -> Bool) -> Bool`, `fn all(mut self, f: fn(T) -> Bool) -> Bool`, `fn find(mut self, f: fn(T) -> Bool) -> Option<T>`.

A lazy combinator's returned iterator draws the receiver's remaining elements on demand — `map` yields each transformed by `f`, `filter` yields those satisfying `f`, `take` at most the first `n`, `skip` all after the first `n` — and a chain of lazy combinators stacks layers over the one original sequence, drawing each element through every layer once, with no intermediate collection. The receiver binding remains a live handle to the same one-shot object — chapter 8's gc aliasing: whichever handle calls `next` draws the next element, and this chapter's exactly-once-in-order and permanent-exhaustion contracts hold of the object, not per binding. An eager combinator leaves the receiver exhausted when it returns: `collect` builds a `List<T>` of the remaining elements in order — the collections chapter's type, prelude-visible under this change's prelude amendment — `fold` applies `f` left to right from `init`, `reduce` seeds with the first element and yields `None` on an empty receiver, `count` counts the remainder, `any` and `all` test the predicate and stop at the first deciding element, and `find` yields the first satisfying element as `Option`, `None` when none does.

The function parameters are pure function types — `fn(T) -> U` with no effect segment — so a closure performing effects does not fit: the value's inferred set is not a subset of the empty expected set, and the rejection is chapter 16's `E1402` at the argument agreement position. There is no combinator-specific purity code, and there is no `forEach`: v0.8 typed its function pure while routing side effects to it, and an effectful one would need the effect polymorphism no chapter ratifies — side effects over a sequence are the for statement's, whose body carries its enclosing declaration's effects. A `.forEach(...)` call is chapter 10's `E0816`. A method generic parameter — `U` of `map` and `fold` — is determined from the call's own text, the function argument's return type, under chapter 10's single-direction rule; an argument that determines nothing is `E0827`.

#### Scenario: A lazy chain transforms in order

- **WHEN** `[1, 2, 3].iterator().map(|x| x * 10).collect()` runs
- **THEN** the result is the `List<Int64>` holding `10`, `20`, `30` — each element flows once through the map layer, drawn on demand, with no intermediate collection

#### Scenario: An effectful closure does not fit

- **WHEN** `names.iterator().map(|s| save(s))` appears and `save` declares `effect io`
- **THEN** the compiler rejects it with `E1402:` function value effect set does not match the expected type's — `f`'s parameter type is a pure function type; the side effect belongs to a for statement

#### Scenario: reduce on an empty receiver yields None

- **WHEN** `reduce(f)` is called on an empty or exhausted iterator
- **THEN** the result is `None` — the seed would be the first element, and there is none

#### Scenario: any stops at the first deciding element

- **WHEN** `any(f)` is called and an element satisfies `f`
- **THEN** the call returns `true` without drawing further elements; `all` mirrors it on the first failing element

#### Scenario: The receiver stays one live handle

- **WHEN** `let derived = it.map(f)` is followed by a `next` call on the binding `it` itself
- **THEN** that call draws the object's next element — one sequence, whichever handle advances it; `derived` never sees the drawn element

#### Scenario: forEach does not exist

- **WHEN** `xs.iterator().forEach(|x| put(x))` appears
- **THEN** the compiler rejects it with `E0816:` no such member on the receiver's type; side effects over a sequence are the for statement's

#### Scenario: U comes from the call's own text

- **WHEN** `names.iterator().map(|n| n.size()).count()` appears
- **THEN** `U` is `Int64`, determined by the function argument's return type under chapter 10's single-direction rule; no expected-type inference participates

## MODIFIED Requirements

### Requirement: The Iterator interface

The standard library declares the generic interface `pub interface Iterator<T> { fn next(mut self) -> Option<T> }` — one method under chapter 10's declaration forms. `next` advances the iterator by one element and returns `Option<T>`: `Some(element)` while elements remain — the iterator MUST return each element exactly once, in order — and `None` once exhausted. Exhaustion is permanent: every call after the first `None` returns `None`, and an iterator MUST NOT be rewound, reset, or replayed. An iterator is stateful by construction — the `mut self` receiver is the advance — so a value-category head cannot honestly implement the interface: a `mut self` method on a `byval` head is chapter 10's `E0812`. The interface is open: a module may implement `Iterator<T>` for its own nominal heads under chapter 10's impl rules, the same as any interface — custom collections produce custom iterators, statically dispatched. Combinators — `map`, `filter`, and their siblings — are this interface's own default methods, ratified with the collections chapter's amendment: their signatures, laziness, purity, and determination rules are fixed by this chapter's Iterator combinators requirement.

#### Scenario: next yields elements in order then exhausts

- **WHEN** `next` is called across exhaustion on an iterator over `0`, `1`, `2`
- **THEN** it returns `Some(0)`, `Some(1)`, `Some(2)`, then `None`, and `None` again on every further call

#### Scenario: A user iterator parses and implements

- **WHEN** a module declares a gc record `Cursor` and writes `impl Iterator<Int64> for Cursor` whose `next(mut self) -> Option<Int64>` advances a position field
- **THEN** the impl is legal under chapter 10 — orphan rule, signature match, uniqueness — and its calls are statically dispatched

#### Scenario: A value-category iterator head is rejected

- **WHEN** `impl Iterator<Int64>` is written for a `byval` record with `fn next(mut self) -> Option<Int64>`
- **THEN** the compiler rejects it with `E0812:` mut self receiver on a value-category type, per chapter 10

#### Scenario: A combinator call resolves to the default method

- **WHEN** `.map(...)` or `.filter(...)` is called on a value whose type implements `Iterator<T>`
- **THEN** the call resolves to the interface's default method under chapter 10's member resolution — no `E0816` — and an impl that overrides it repeats the interface signature exactly

### Requirement: The for-loop protocol

The iterable expression of chapter 5's for statement MUST have a type implementing `Iterable<T>` for some element type `T`; every other type is rejected with `E0901`. The rule is single-form: an `Iterator<T>` alone is not a for iterable — a bare iterator is consumed by explicit `next` calls, or wrapped once the standard library provides adapters — and no interface other than `Iterable` makes a type iterable. Execution: the iterable expression is evaluated exactly once; `iterator` is obtained exactly once; `next` is called repeatedly; each `Some(element)` executes the body once with the element bound under chapter 5's name rules; the first `None` ends the loop. The element type `T` is the loop binding's type. The types ratified so far carry the builtin implementations — `String` over `Rune` and `Range<T>` over `T` (String iteration, The Range type) — and the collections chapter's `List`, `Map`, and `Set` are iterables over their elements and entries on this protocol.

#### Scenario: A non-iterable expression is rejected

- **WHEN** `for x in 5 { ... }` appears — `Int64` implements no `Iterable`
- **THEN** the compiler rejects it with `E0901:` for-in expression does not implement Iterable

#### Scenario: A bare iterator is not a for iterable

- **WHEN** `for x in iter { ... }` appears with `iter` of a type implementing only `Iterator<Int64>`
- **THEN** the compiler rejects it with `E0901:` for-in expression does not implement Iterable; a bare iterator is consumed by explicit `next` calls

#### Scenario: The iterator is obtained exactly once

- **WHEN** the same collection binding backs two for statements
- **THEN** each statement obtains its own iterator and both see all elements; the iterable is never consumed by iteration

### Requirement: Iterable implementer obligations

Implementations of `Iterable` answer the ownership categories of chapter 8. A resource-category type MUST NOT implement `Iterable` — the rejection is `E0903`: an iterator holds a live view of its collection across the loop's executions, and a handle whose reach outlives the single deterministic release point a resource's discipline depends on is not honest; traversing a resource's contents materializes them first — one explicit read into a collection, then iterate the collection. A value-category iterable iterates honestly by copy: chapter 8's value semantics apply — `iterator` receives its own copy of the receiver, and iterating never consumes or mutates the original binding. The gc collections' iteration semantics are fixed by the collections chapter — a snapshot at the `iterator` call — and this chapter fixed the protocol they rest on.

#### Scenario: A resource impl is rejected

- **WHEN** `impl Iterable<Int64>` is written for a `byres` record
- **THEN** the compiler rejects it with `E0903:` impl of Iterable for a resource-category type; traverse by materializing first

#### Scenario: A value iterable is never consumed

- **WHEN** a `byval` record implementing `Iterable<T>` backs two for statements
- **THEN** both see all elements; the binding is unaffected by either loop
