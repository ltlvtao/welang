# We Language Specification — Chapter 5: Iteration

### Requirement: For statement

A for statement is `for name in expr block`: `for` and `in` are chapter-1 keywords, `name` is an identifier, `_` (which binds nothing), or a tuple pattern under chapter 8, `expr` is an expression, and the body is a chapter-2 block. A tuple pattern in the head binds element-wise under chapter 8's rules: its bindings match the element type's tuple, and a pattern against a non-tuple element type is rejected under chapter 7's `E0501` at the binding position. For is a statement: it produces no value and MUST NOT appear where a value is required (`E0202`). The iterable expression is evaluated exactly once, before iteration begins. The statement iterates the elements of the iterable's value in order, each exactly once; for each element the body executes once with `name` bound to that element, and each iteration's binding is independent and scoped to that execution — it MUST NOT leak past the loop. `break` and `continue` are legal inside a for body under chapter 3's Break and continue: for is a loop for that requirement. The iterable expression's type MUST implement `Iterable<T>` for its element type `T` under chapter 11's iterable protocols — the rejection is `E0901` — and the statement's execution model, one iterator advanced by `next` until `None`, is chapter 11's for-loop protocol.

#### Scenario: for as a statement item

- **WHEN** `for i in 0..3 { sum = sum + i }` appears as a block item
- **THEN** it parses as a for statement; the body executes once per element, with `i` bound to `0`, `1`, `2` in order

#### Scenario: for in a value position

- **WHEN** `let x = for i in 0..3 { i }` appears — a for used where a value is required
- **THEN** the compiler rejects it with `E0202:` valueless form in value position

#### Scenario: the loop binding does not leak

- **WHEN** a for statement binds `i`, for example `for i in items { use(i) }`
- **THEN** `i` is not visible after the statement; each iteration's binding is its own scope

#### Scenario: break and continue inside for

- **WHEN** `for i in items { if bad(i) { break } step(i) }` is parsed
- **THEN** the break is legal under chapter 3's Break and continue and exits the for; continue analogously proceeds to the next element

#### Scenario: the wildcard name binds nothing

- **WHEN** `for _ in 0..3 { step() }` appears
- **THEN** the body executes once per element with no binding; `_` is never a binding name

#### Scenario: a tuple pattern in the loop head

- **WHEN** `for (a, b) in pairs { ... }` appears and the iterable's element type is a tuple
- **THEN** the pattern binds element-wise per chapter 8, one binding per element, scoped like any loop binding

### Requirement: Range expression

The range operator `..` is a binary operator, the loosest level of chapter 2's precedence table, non-associative on its own level: `a..b..c` is rejected with `E0104`. The range is right-exclusive: `start..end` excludes `end`. Over integer operands — the eight integer types of chapter 7; an operand of any other type, the floats included, is rejected with `E0902` under chapter 11 — iteration yields each value from `start` inclusive to `end` exclusive in order by unit steps — `0..3` iterates `0`, `1`, `2` — and a range whose start is not below its end yields no elements. A range is an expression: usable wherever an expression is legal, not only in a for header; arithmetic and postfix forms on either operand group tighter (`0..n-1` is `0..(n-1)`). The Range type, its typing, and its builtin `Iterable` implementation are chapter 11's.

#### Scenario: range as an expression

- **WHEN** `let r = 0..n` appears
- **THEN** it parses as a range expression at chapter 2's loosest binary level; `0..n-1` groups as `0..(n-1)`

#### Scenario: right-exclusivity

- **WHEN** `for i in 0..3 { collect(i) }` runs
- **THEN** `i` takes `0`, `1`, `2` — never `3`

#### Scenario: chained range is rejected

- **WHEN** `a..b..c` is parsed
- **THEN** the compiler rejects it with `E0104:` chained non-associative operator; each range must be a single explicit group

#### Scenario: open-ended ranges are not ratified

- **WHEN** `..5` or `5..` appears
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic; both range bounds are required

#### Scenario: an empty range iterates zero times

- **WHEN** `for i in 3..0 { step() }` appears
- **THEN** the body executes zero times; the statement completes without iterating

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–5 and the iterable protocols they rest on. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. The combinator example below reaches into chapter 11's Iterator default methods, carried by chapter 12's closures.

### for over a range

```we
var sum = 0
for i in 0..10 {            // right-exclusive: i takes 0..9, never 10
    sum = sum + i
}
// i is NOT visible here: each iteration's binding is its own scope

for _ in 0..3 {             // wildcard name: runs the body 3 times,
    retry()                 // binds nothing
}

for i in 3..0 {             // start not below end: zero iterations
    never()
}
```

### for over an iterable expression

```we
for item in loadItems() {   // the expression is evaluated exactly once,
    handle(item)            // then its elements iterate in order
    if skip(item) { continue }
    if done(item) { break } // break/continue legal: for is a loop under
}                           // chapter 3's Break and continue

let x = for i in 0..3 { i } // E0202: valueless form in value position
for (a, b) in pairs {       // a tuple pattern in the head: binds
    use(a, b)               // element-wise per chapter 8
}
```

### range as an expression

```we
let r = 0..n                // a range is an expression, not only a for
let page = offset..offset + size
// groups as offset..(offset + size): arithmetic binds tighter than `..`

let bad = a..b..c           // E0104: chained non-associative operator
let open = ..5              // E0105: both range bounds are required
```

### Combinators over a range

```we
let out = (0..10)
    .iterator()                 // chapter 11: Range implements Iterable
    .filter(|x| x > 2)          // lazy layers — no intermediate collection
    .map(|x| x * 10)
    .collect()                  // List<Int64>: 30, 40, 50, 60, 70, 80, 90
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| for statement | for 语句 |
| iterable expression | 可迭代表达式 |
| loop variable | 循环变量 |
| iteration | 迭代 |
| element | 元素 |
| range | 区间 |
| right-exclusive | 右排他 |
| unit step | 单位步长 |
