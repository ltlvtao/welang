## ADDED Requirements

### Requirement: The Option type

The standard library declares the generic sum type `pub type Option<T> = Some(T) | None` — a gc sum under chapter 9's declaration forms carrying one generic parameter under chapter 10's, named and reached by the module rules of chapter 6. `Some(T)` carries a value of the element type `T`; `None` is the absence of one. Construction and matching follow chapter 9 exactly: `Some(expr)` builds the carrying variant by the call form, `None` stands bare as a unit variant, and variant patterns destructure both. `Option` is the language's one canonical option type: when an interface needs an element-or-absence result — this chapter's `next` is the first — it MUST return `Option`, and later chapters MUST bind their option-shaped results to it rather than declaring siblings. The option's further method inventory (`unwrap` and its siblings) is the standard library's own surface, not ratified here.

#### Scenario: Some and None construct

- **WHEN** `Some(1)` and `None` appear where `Option<Int64>` is expected
- **THEN** both are values of the declared standard-library sum type per chapter 9's constructors

#### Scenario: match recovers the carried value

- **WHEN** a match on an `Option<Int64>` holds the arms `Some(n) => n + 1` and `None => 0`
- **THEN** the variant patterns bind per chapter 9 and the arms agree on `Int64`

#### Scenario: A generic instantiation is checked per use

- **WHEN** `Option<String>` appears in a type slot
- **THEN** it is a generic application under chapter 10, its constructors typed at that instantiation

### Requirement: The Iterator interface

The standard library declares the generic interface `pub interface Iterator<T> { fn next(mut self) -> Option<T> }` — one method under chapter 10's declaration forms. `next` advances the iterator by one element and returns `Option<T>`: `Some(element)` while elements remain — the iterator MUST return each element exactly once, in order — and `None` once exhausted. Exhaustion is permanent: every call after the first `None` returns `None`, and an iterator MUST NOT be rewound, reset, or replayed. An iterator is stateful by construction — the `mut self` receiver is the advance — so a value-category head cannot honestly implement the interface: a `mut self` method on a `byval` head is chapter 10's `E0812`. The interface is open: a module may implement `Iterator<T>` for its own nominal heads under chapter 10's impl rules, the same as any interface — custom collections produce custom iterators, statically dispatched. Combinators — `map`, `filter`, and their siblings — are not ratified by this chapter: they need function values, arrive with their owning change, and land as default methods of this interface with their behavior fixed by the spec.

#### Scenario: next yields elements in order then exhausts

- **WHEN** `next` is called across exhaustion on an iterator over `0`, `1`, `2`
- **THEN** it returns `Some(0)`, `Some(1)`, `Some(2)`, then `None`, and `None` again on every further call

#### Scenario: A user iterator parses and implements

- **WHEN** a module declares a gc record `Cursor` and writes `impl Iterator<Int64> for Cursor` whose `next(mut self) -> Option<Int64>` advances a position field
- **THEN** the impl is legal under chapter 10 — orphan rule, signature match, uniqueness — and its calls are statically dispatched

#### Scenario: A value-category iterator head is rejected

- **WHEN** `impl Iterator<Int64>` is written for a `byval` record with `fn next(mut self) -> Option<Int64>`
- **THEN** the compiler rejects it with `E0812:` mut self receiver on a value-category type, per chapter 10

#### Scenario: Combinators are not ratified here

- **WHEN** `.map(...)` or `.filter(...)` is called on an iterator value
- **THEN** the compiler rejects it with `E0816:` no such member on the receiver's type, per chapter 10, until the combinator change ratifies them

### Requirement: The Iterable interface

The standard library declares the generic interface `pub interface Iterable<T> { type Iter; fn iterator(self) -> Iter }` — one associated type and one method under chapter 10's declaration forms. The element type `T` travels as a generic parameter — a `List<Int64>` iterates `Int64` — while `Iter`, the iterator-handle type, is the implementer's associated binding: each impl fixes one concrete iterator type for its head. Every impl of `Iterable<T>` MUST bind `Iter` to a type that implements `Iterator<T>` for the same element type — the handle contract — and an impl that binds anything else is rejected with `E0904`. The contract reflects into bounds: wherever a type is known to implement `Iterable<T>` — concretely, or only through a bound — its `Iter` carries `Iterator<T>`'s method set, because the `E0904` obligation is part of the interface every impl honors. An iterable is repeatable: each call to `iterator` returns a fresh, independent iterator over the same elements, and the iterable itself is never consumed. `Iterable` declares an associated type and so cannot be boxed — `Dyn<Iterable<T>>` is chapter 10's `E0819` — while `Iterator<T>` declares none and may be boxed, `Dyn<Iterator<T>>`, when an erased iterator handle is wanted.

#### Scenario: Each iterator call yields a fresh iterator

- **WHEN** `iterator` is called twice on one collection binding and both returned iterators are fully consumed
- **THEN** both yield the same elements in order; the collection re-iterates from its head

#### Scenario: A manual impl binds its iterator type

- **WHEN** a module writes `impl Iterable<Int64> for IntSet` with the items `type Iter = IntSetIter` and `fn iterator(self) -> IntSetIter`
- **THEN** the impl is legal per chapter 10, with `IntSetIter` implementing `Iterator<Int64>` as the handle contract requires

#### Scenario: An Iter bound to a non-iterator is rejected

- **WHEN** an impl of `Iterable<T>` binds `type Iter = Int64`, a type implementing no `Iterator`
- **THEN** the compiler rejects it with `E0904:` impl binds Iter to a non-iterator type

#### Scenario: A bound's Iter carries the method set

- **WHEN** a generic fn declares `where C: Iterable<Int64>` and its body calls `c.iterator().next()`
- **THEN** the `next` call resolves to `Iterator<Int64>`'s method set; without the bound the call would be chapter 10's `E0817`

#### Scenario: An equality narrows the iterator type

- **WHEN** a where clause writes `where C: Iterable<Int64>, C.Iter == CountingIter`
- **THEN** inside the declaration `C.Iter` is usable as `CountingIter` per chapter 10's equality rule

### Requirement: The for-loop protocol

The iterable expression of chapter 5's for statement MUST have a type implementing `Iterable<T>` for some element type `T`; every other type is rejected with `E0901`. The rule is single-form: an `Iterator<T>` alone is not a for iterable — a bare iterator is consumed by explicit `next` calls, or wrapped once the standard library provides adapters — and no interface other than `Iterable` makes a type iterable. Execution: the iterable expression is evaluated exactly once; `iterator` is obtained exactly once; `next` is called repeatedly; each `Some(element)` executes the body once with the element bound under chapter 5's name rules; the first `None` ends the loop. The element type `T` is the loop binding's type. The types ratified so far carry two builtin implementations only — `String` over `Rune` and `Range<T>` over `T` (String iteration, The Range type); the standard library's collection types arrive with the collections chapter.

#### Scenario: A non-iterable expression is rejected

- **WHEN** `for x in 5 { ... }` appears — `Int64` implements no `Iterable`
- **THEN** the compiler rejects it with `E0901:` for-in expression does not implement Iterable

#### Scenario: A bare iterator is not a for iterable

- **WHEN** `for x in iter { ... }` appears with `iter` of a type implementing only `Iterator<Int64>`
- **THEN** the compiler rejects it with `E0901:` for-in expression does not implement Iterable; a bare iterator is consumed by explicit `next` calls

#### Scenario: The iterator is obtained exactly once

- **WHEN** the same collection binding backs two for statements
- **THEN** each statement obtains its own iterator and both see all elements; the iterable is never consumed by iteration

### Requirement: String iteration

`String` carries a builtin implementation of `Iterable<Rune>`: the element type is `Rune`, the `Iter` binding is a standard-library iterator type over `Rune` — its name is the standard library's, not this chapter's — and iteration yields the string's code points in order, one per element. This lands chapter 7's recorded fact that a string's logical iteration yields `Rune`. Byte-level traversal is not iteration: it is the explicit `toBytes()` conversion's, per chapter 7's no-implicit-conversion obligation, and `Bytes` carries no `Iterable` implementation at all. The implementation is builtin and unique — a manual attempt fits no impl production, a base-type head being chapter 10's `E0811`, and no second implementation can exist for one head.

#### Scenario: for over a string yields runes in order

- **WHEN** `for c in "hì" { put(c) }` runs
- **THEN** `c` binds each code point as `Rune`, in order

#### Scenario: A manual impl for String is rejected

- **WHEN** `impl Iterable<Rune> for String` is written
- **THEN** the compiler rejects it with `E0811:` impl head is not a nominal type, per chapter 10; the builtin implementation is the one

#### Scenario: Bytes is not iterable

- **WHEN** `for b in bytes { ... }` appears with `bytes: Bytes`
- **THEN** the compiler rejects it with `E0901:` for-in expression does not implement Iterable; the byte view is the explicit conversion's

### Requirement: The Range type

`Range<T>` is a builtin generic type with one parameter `T`, and `T` MUST be one of chapter 7's eight integer types: a range operand of any other type — the floats included — is rejected with `E0902`. A range value is constructed only by chapter 5's range operator, its two operands of the same integer type — mixed operands are chapter 7's `E0501` — typing themselves per chapter 7's literal rules: `0..n` is `Range<Int64>`. A range value is its two bounds and nothing more: it carries no state, and binding, passing, or returning it copies the pair. `Range<T>` carries a builtin implementation of `Iterable<T>` — the `Iter` binding a standard-library iterator type over `T` — and iterating it yields each value from `start` inclusive to `end` exclusive in order by unit steps: chapter 5's iteration rule, now type-grounded, with a start not below its end yielding no elements. The implementation is builtin and unique as `String`'s is (`E0811` on a manual attempt).

#### Scenario: A range binds with its type

- **WHEN** `let r = 0..n` appears with `n: Int64`
- **THEN** `r` is `Range<Int64>`, a builtin generic application in the type-reference grammar of chapters 7 and 10

#### Scenario: A float operand is rejected

- **WHEN** `1.5..2.5` appears
- **THEN** the compiler rejects it with `E0902:` range operand is not an integer type; float spans are explicit loops

#### Scenario: A non-numeric operand is rejected likewise

- **WHEN** `'a'..'z'` appears
- **THEN** the compiler rejects it with `E0902:` range operand is not an integer type

#### Scenario: Mixed integer operands are rejected

- **WHEN** `0i32..9i64` appears
- **THEN** the compiler rejects it under `E0501` per chapter 7; the bounds must be one integer type

#### Scenario: Iteration yields unit steps

- **WHEN** `for i in 2..5 { step(i) }` runs
- **THEN** `i` takes `2`, `3`, `4` — start inclusive, end exclusive, one step at a time

### Requirement: Iterable implementer obligations

Implementations of `Iterable` answer the ownership categories of chapter 8. A resource-category type MUST NOT implement `Iterable` — the rejection is `E0903`: an iterator holds a live view of its collection across the loop's executions, and a handle whose reach outlives the single deterministic release point a resource's discipline depends on is not honest; traversing a resource's contents materializes them first — one explicit read into a collection, then iterate the collection. A value-category iterable iterates honestly by copy: chapter 8's value semantics apply — `iterator` receives its own copy of the receiver, and iterating never consumes or mutates the original binding. The gc collections' snapshot-or-cursor obligations belong to the collections chapter that ratifies those types; this chapter fixes the protocol they rest on.

#### Scenario: A resource impl is rejected

- **WHEN** `impl Iterable<Int64>` is written for a `byres` record
- **THEN** the compiler rejects it with `E0903:` impl of Iterable for a resource-category type; traverse by materializing first

#### Scenario: A value iterable is never consumed

- **WHEN** a `byval` record implementing `Iterable<T>` backs two for statements
- **THEN** both see all elements; the binding is unaffected by either loop

### Requirement: Iterable protocols diagnostics segment

The iterable-protocols chapter owns registry segment `E0900`–`E0999` declared in `docs/spec/diagnostics.toml` `[segments]`. Allocations: `E0901` for-in expression does not implement Iterable, `E0902` range operand is not an integer type, `E0903` impl of Iterable for a resource-category type, `E0904` impl binds Iter to a non-iterator type. `E0900` and `E0905`–`E0999` remain reserved for amendments of this chapter. Trigger semantics live in this chapter's Requirements; entries live in the registry.

#### Scenario: An iterables code is emitted

- **WHEN** any `E09xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `1100-iterables`

#### Scenario: A later change needs this segment's codes

- **WHEN** a later chapter ratifies rules requiring new iterable diagnostics — the collections chapter among them
- **THEN** its change extends the registry within `E0900`–`E0999` in the same change, or claims its own segment
