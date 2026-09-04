# We Language Specification — Chapter 11: Iterable protocols

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

Implementations of `Iterable` answer the ownership categories of chapter 8. A resource-category type MUST NOT implement `Iterable` — the rejection is `E0903`: an iterator holds a live view of its collection across the loop's executions, and a handle whose reach outlives the single deterministic release point a resource's discipline depends on is not honest; traversing a resource's contents materializes them first — one explicit read into a collection, then iterate the collection. A value-category iterable iterates honestly by copy: chapter 8's value semantics apply — `iterator` receives its own copy of the receiver, and iterating never consumes or mutates the original binding. The gc collections' iteration semantics are fixed by the collections chapter — a snapshot at the `iterator` call — and this chapter fixed the protocol they rest on.

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

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–11. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. The combinator examples below reach into chapter 12's closures and chapter 17's `List`.

### Option and iteration

```we
let maybe = Some(3)                    // Some carries its element
let none: Option<Int64> = None         // None stands bare

let score = match maybe {              // arms agree on Int64
    Some(n) => n
    None => 0
}
```

### Iterators and iterables

```we
let it = names.iterator()              // fresh per call; the collection
let first = match it.next() {          // itself is never consumed
    Some(name) => name
    None => "anonymous"
}

impl Iterable<Int64> for IntSet {      // a manual impl binds its own
    type Iter = IntSetIter             // iterator type; the contract:
    fn iterator(self) -> IntSetIter {  // Iter implements Iterator<Int64>
        IntSetIter { set: self, pos: 0 }
    }
}
```

### for over strings and ranges

```we
for c in "hì" {                        // String: builtin Iterable<Rune>
    put(c)                             // c is Rune, code points in order
}

for i in 2..5 {                        // Range<Int64>: 2, 3, 4
    step(i)
}
```

### Rejected forms

```we
for x in 5 { }                         // E0901: for-in expression does
                                       // not implement Iterable
for b in bytes { }                     // E0901: Bytes carries no
                                       // implementation; convert first
let r = 1.5..2.5                       // E0902: range operand is not an
                                       // integer type
let m = 0i32..9i64                     // E0501: the bounds must be one
                                       // integer type
impl Iterable<Int64> for LogFile { }   // E0903: LogFile is of the
                                       // resource category
impl Iterable<Rune> for String { }     // E0811: a base-type head fits no
                                       // impl production
impl Iterable<Int64> for IntSet {
    type Iter = Int64                  // E0904: impl binds Iter to a
    fn iterator(self) -> Int64 { 0 }   // non-iterator type
}
for (a, b) in names { }                // E0501: the element type String
                                       // is not a tuple
```

### Combinators

```we
let out = names.iterator()
    .filter(|n| n.size() > 2)  // lazy: a layer over the receiver
    .map(|n| n.toUpper())      // lazy: U comes from this call's text
    .collect()                 // List<String>: eager — the chain ends

let total = [1, 2, 3].iterator().fold(0, |acc, x| acc + x)   // 6
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| option type | 选项类型 |
| element type | 元素类型 |
| iterator | 迭代器 |
| iterable | 可迭代值 |
| iterator handle | 迭代器句柄 |
| handle contract | 句柄契约 |
| exhaustion | 耗尽 |
| one-shot | 一次性消耗 |
| builtin implementation | 内建实现 |
| code point | 码点 |
| bounds | 边界 |
| materialize | 物化 |
| combinator | 组合子 |
