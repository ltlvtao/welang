## ADDED Requirements

### Requirement: The collection types

The standard library declares three collection types: `List<T>` — an ordered sequence of zero or more elements of one type; `Map<K, V>` — an unordered finite map from keys of one type to values of another; `Set<T>` — an unordered finite set of elements of one type. All three are generic — arities one, two, one — with the arguments type references under chapter 10, and all three are gc-category types under chapter 8: shared by reference, mutation visible through aliases, never copied by value semantics. The three names join the prelude under chapter 15's amendment in this change: a list literal needs no import to type, and the Option precedent — the type is language-visible, its full method inventory is the standard library's surface — governs the rest. All three implement `Iterable` under chapter 11 — `List<T>` over its elements, `Set<T>` over its elements, `Map<K, V>` over its entries as two-tuples `(K, V)` — with the iteration semantics fixed by Snapshot iteration below. The standard library's further convenience methods (`add`, `removeAt`, `put`, `keys`, and their siblings) are its own surface, not ratified here; what this chapter fixes is the semantics-bearing anchors: the types' categories and reach, literals, access, iteration, and the mutation surface.

#### Scenario: The three names are prelude-visible

- **WHEN** a module with no imports annotates `List<Int64>`, `Map<String, User>`, or `Set<Rune>`
- **THEN** each name resolves through the prelude to the standard library's declared type; no `import std.collections` is required

#### Scenario: The collections are gc-category

- **WHEN** two bindings alias one `List<Int64>` value and a mut method called through one alias adds an element
- **THEN** the element is visible through the other alias — chapter 8's gc semantics; collections are never silently copied

#### Scenario: Generic arity is fixed by the declaration

- **WHEN** an annotation holds `List<Int64, String>` or `Map<String>`
- **THEN** the compiler rejects it with `E0828:` type argument arity mismatch; `List` and `Set` take one argument, `Map` takes two

### Requirement: List literals

A list literal is `[e1, e2, ..., en]` — zero or more comma-separated expressions between square brackets — and is a primary expression of chapter 2's skeleton. Every element expression MUST have one common type, established element by element under chapter 7's no-implicit-conversion rule (`E0501`): the literal's element type is that type, and the literal's type is `List` of it. An empty literal `[]` takes its element type from the expected type at its position — an annotation, a parameter, a declared return — and an empty literal at a position with no expected type MUST be rejected with `E1501:` empty list literal has no expected type: the language infers nothing (chapter 0, Principle 1). Square brackets are brackets for chapter 2's line-joining rule; a literal MAY span lines. The same bracket tokens serve no other expression form: `expr[expr]` index syntax fits no production of this specification, now or by later amendment of chapter 2 alone — indexing is by named methods only (Indexing by named methods).

#### Scenario: A literal with agreeing elements

- **WHEN** `let xs = [1, 2, 3]` appears
- **THEN** the common type is `Int64` by chapter 1's literal defaults and the literal's type is `List<Int64>`

#### Scenario: Disagreeing elements are rejected

- **WHEN** `let xs = [1, "two"]` appears
- **THEN** the compiler rejects it with `E0501:` operands of different types — one element type, no implicit conversion; convert explicitly so the elements agree

#### Scenario: An empty literal takes the expected type

- **WHEN** `let xs: List<String> = []` appears, or `[]` is passed where `List<Int64>` is expected
- **THEN** the empty literal types as the expected type's instance; the annotation is the only source of the element type

#### Scenario: A bare empty literal is rejected

- **WHEN** `let xs = []` appears with no annotation and no other expectation at the binding
- **THEN** the compiler rejects it with `E1501:` empty list literal has no expected type — write the annotation, or start with elements

#### Scenario: A literal against a disagreeing annotation

- **WHEN** `let xs: List<String> = [1, 2]` appears — the elements fix one element type, the annotation another
- **THEN** the compiler rejects it with `E0501:` operands of different types at the binding agreement position; no conversion reconciles the two

#### Scenario: Index syntax fits no production

- **WHEN** `xs[0]` appears — bracketed operands after an expression
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic (`E0105`); indexing is by named methods, and no chapter may ratify `expr[expr]` without amending this policy here

#### Scenario: A literal spans lines inside the brackets

- **WHEN** a literal is written `[` on one line with elements one per line and `]` on the last
- **THEN** the line breaks carry no significance — square brackets are brackets under chapter 2

### Requirement: Indexing by named methods

The index form `expr[expr]` MUST NOT parse anywhere in the language: it fits no production, and what a bracketed access would mean is instead a named method whose contract is visible at the call site. The spec-anchored access family: `List<T>.get(i: Int64) -> Option<T>` — the element at position i, `None` when i is out of range — below zero or at or above the length — positions zero-based, no panic, no partial reads; `Map<K, V>.get(k: K) -> Option<V>` — the value at key k, `None` when absent; `Set<T>.has(t: T) -> Bool`. Out-of-range access is a `None`, not a trap and not an error channel: range reasoning stays a local `match` on the returned Option, locally decidable (chapter 0, Principle 1). `String` follows the same policy with its two layers under chapter 7's Rune iteration fact: byte-level `.byteLength() -> Int64` and `.byteSlice(start: Int64, end: Int64) -> String` — end exclusive, the span `start..end` as chapter 5's ranges; a byte operation with an out-of-range range is a panic, the chapter 14 family, because a byte range names storage that does not exist — and code-point-level `.runeCount() -> Int64` and `.charAt(i: Int64) -> Rune`, an out-of-range index a panic likewise. The two layers never share a name: a byte offset and a code-point position are different numbers, and a method name tells the reader which one they are holding (chapter 0, Principle 4).

#### Scenario: List access returns Option

- **WHEN** `xs.get(2)` appears on a three-element list, and `xs.get(3)` likewise
- **THEN** the first is `Some(element)` and the second is `None`; out-of-range is a value, and the caller matches on it

#### Scenario: Map and Set access

- **WHEN** `m.get("id")` appears on a map without that key, and `s.has(x)` on a set
- **THEN** the map access is `None` — absence is a value — and the set membership test is `Bool`

#### Scenario: A byte operation with an impossible range panics

- **WHEN** `"hello".byteSlice(0, 99)` or `"hello".charAt(7)` appears at runtime
- **THEN** the process terminates through chapter 14's panic family — the range names storage that does not exist; range checks are the caller's, and the panic is the boundary

#### Scenario: The byte and rune layers never share a name

- **WHEN** a reader sees `.byteSlice` or `.charAt` on a String
- **THEN** the name alone says which layer — bytes or code points; there is no unindexed `.slice` to guess at

### Requirement: Snapshot iteration

Calling `iterator` on a gc collection fixes the iteration's element sequence at that moment: the returned iterator MUST yield exactly the elements the collection held when `iterator` was called, in the collection's order for `List`, and in an order the snapshot itself fixes at the call for `Map` and `Set` — the unordered types guarantee no order across iterators, only that each iterator's own sequence is its call's snapshot — and an element added or removed after the call — through any alias — MUST NOT be seen by that iterator. A fresh `iterator` call returns a fresh snapshot. The rule makes reading a loop equal to knowing its iteration set (chapter 0, Principle 1), discharges chapter 11's deferred gc obligations as snapshot, and composes with chapter 13's resource materialize-first discipline: nothing about a collection iteration depends on executions the loop's text does not show. Snapshot is a semantic obligation, not an implementation command: an implementation may copy, share immutably, or copy-on-write, so long as the observable sequence is the call-time one.

#### Scenario: Mutation during iteration is not seen

- **WHEN** an iterator is obtained from a list, the list is mutated through an alias, and the iterator is then fully consumed
- **THEN** the yielded elements are exactly the call-time ones; the mutation is invisible to that iterator and visible to the next `iterator` call

#### Scenario: Fresh iterator, fresh snapshot

- **WHEN** `iterator` is called twice around a mutation
- **THEN** the first returned iterator yields the pre-mutation sequence, the second the post-mutation one — both consistent with call-time snapshots

#### Scenario: Set and Map iteration are snapshots likewise

- **WHEN** an iterator is obtained from a `Set<T>`, or from a `Map<K, V>` whose elements are `(K, V)` entries
- **THEN** the call-time rule holds identically; each collection type's iterator is a snapshot of what it held at the call

### Requirement: Collection mutation is explicit and aliased

The collection types carry mut methods — `mut self` methods under chapter 10's receiver rules — and MUST NOT carry any other mutation surface: there is no field assignment (collections declare no fields), no update expression (chapter 8's `with &` is for records), and no operator that mutates. A mut method called through one alias is visible through every alias — the gc semantics of chapter 8 — and mutation during another alias's iteration is safe because iteration is a snapshot: the iterator's sequence was fixed before. The method inventory beyond the anchored access family is the standard library's surface; what this chapter fixes is that every mutation is a named method call on a binding the reader can see.

#### Scenario: A mut method is visible through aliases

- **WHEN** `shared.add(x)` is called on a list aliased by a second binding, and the second binding's `size`-family method is then called
- **THEN** the added element is reflected — gc aliasing, chapter 8; no copy happened at the call

#### Scenario: No mutation form outside named methods

- **WHEN** a collection value is the target of an assignment `xs = ...` — rebinding, not mutation — or of any operator
- **THEN** rebinding follows chapter 2's assignment rules and touches no other alias; the only collection mutation is a `mut self` method call

### Requirement: The collections diagnostics segment

The collections chapter owns registry segment `E1500`–`E1599`, declared in `docs/spec/diagnostics.toml` `[segments]` with owner `1700-collections`. Allocation: `E1501` empty list literal has no expected type. `E1500` and `E1502`–`E1599` are reserved for this chapter's amendments; chapters needing further segment codes claim their own segments. Subscript attempts, element disagreement, combinator purity, and iterator-handle violations carry the codes of their owning chapters — `E0105`, `E0501`, `E1402`, `E0904` — and no code of this segment duplicates them.

#### Scenario: The segment is retrievable

- **WHEN** a diagnostics consumer looks up any `E15xx` code in the registry
- **THEN** the segment entry names owner `1700-collections`, and each allocated code's entry carries severity, title, description, remediation, owner, requirement, and allocated date

#### Scenario: Later changes extend within the segment

- **WHEN** a later spec-layer change to this chapter needs a new code
- **THEN** it allocates from `E1500`–`E1599` within its own change; collections diagnostics that renumber onto other chapters' facts do not exist
