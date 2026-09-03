# We Language Specification — Chapter 5: Iteration

### Requirement: For statement

A for statement is `for name in expr block`: `for` and `in` are chapter-1 keywords, `name` is an identifier or `_` (which binds nothing), `expr` is an expression, and the body is a chapter-2 block. For is a statement: it produces no value and MUST NOT appear where a value is required (`E0202`). The iterable expression is evaluated exactly once, before iteration begins. The statement iterates the elements of the iterable's value in order, each exactly once; for each element the body executes once with `name` bound to that element, and each iteration's binding is independent and scoped to that execution — it MUST NOT leak past the loop. `break` and `continue` are legal inside a for body under chapter 3's Break and continue: for is a loop for that requirement. Whether the iterable expression's type may be iterated — the iterable and iterator protocols — is ratified by the types chapter.

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

#### Scenario: destructuring in the loop head is not ratified

- **WHEN** `for (a, b) in pairs` appears
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic until tuple patterns are ratified by their paired amendment

### Requirement: Range expression

The range operator `..` is a binary operator, the loosest level of chapter 2's precedence table, non-associative on its own level: `a..b..c` is rejected with `E0104`. The range is right-exclusive: `start..end` excludes `end`. Over numeric operands, iteration yields each value from `start` inclusive to `end` exclusive in order by unit steps — `0..3` iterates `0`, `1`, `2` — and a range whose start is not below its end yields no elements. A range is an expression: usable wherever an expression is legal, not only in a for header; arithmetic and postfix forms on either operand group tighter (`0..n-1` is `0..(n-1)`). The Range type, its typing, iteration over non-numeric operands, and its iterable implementation are ratified by the types chapter.

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

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–5. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Iterator protocol and combinators are annotated as pending the types-chapter pairing.

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
for (a, b) in pairs {       // E0105: head destructuring arrives with
    use(a, b)               // tuple patterns (paired amendment)
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

### pending the types chapter

```we
// The iterable/iterator protocols, and methods like iterator(), are
// ratified by the types chapter's pairing; combinators additionally
// need closures, Option, and List:
//
// let out = items.iterator()
//     .filter(|x| x > 2)
//     .map(|x| x * 10)
//     .collect()
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
