# We Language Specification — Chapter 4: Match

### Requirement: Match expression form

A match expression is `match scrutinee { arm... }`: the `match` keyword, one expression (the scrutinee), and a brace group holding one or more arms. The brace group is not a chapter-2 block: it holds arms, not statements, and has no block value. The scrutinee is evaluated exactly once, before any arm is considered. Arms are considered top to bottom; the first arm whose pattern matches is taken, and later arms are not evaluated. The match expression's value is the taken arm's body value. A match with zero arms is rejected with `E0301`. Match is an expression form: used as a block item it is an expression statement under chapter 2, and no separate valueless match form exists.

#### Scenario: match used as a value

- **WHEN** a match whose arms are expression-bodied initializes a binding, for example `let x = match c {` with arms `200 => 1` and `_ => 0` on their own lines, and `c` equals `200`
- **THEN** `x` receives the taken arm's body value, here `1`

#### Scenario: the first matching arm wins

- **WHEN** two arms both match the scrutinee, for example a literal arm followed by a wildcard arm
- **THEN** the earlier arm is taken; the later arm is not evaluated

#### Scenario: the scrutinee is evaluated once

- **WHEN** the scrutinee is a call, for example `match next() {` with arm `n => n`
- **THEN** the call is evaluated exactly once, before arm selection

#### Scenario: a match with zero arms

- **WHEN** `match c { }` appears — a match whose brace group holds no arms
- **THEN** the compiler rejects it with `E0301:` match requires at least one arm

### Requirement: Match arms

An arm is `pattern => body`, where `body` is an expression; blocks are expressions under chapter 2, so `pattern => expr` and `pattern => { ... }` are one rule, not two forms. Arms are separated by inferred boundaries under chapter 2's line-joining: the continuation set and boundary inference that govern statements govern arms identically, each arm starting on its own line; no separator token exists between arms, and a `,` between arms fits no ratified production (chapter 2's unexpected-token diagnostic). The arm's pattern bindings — and its guard's condition, when present — are visible inside that arm and MUST NOT leak past it.

#### Scenario: block-bodied arm

- **WHEN** an arm's body is a block, for example `_ => {` with items `log()` and `0` and a closing `}`
- **THEN** the arm's body value is the block value under chapter 2's Blocks and block value

#### Scenario: arm bindings do not leak

- **WHEN** an arm binds a name through its pattern, for example the arm `n => n`
- **THEN** the binding is not visible after the match expression; each arm is its own scope

#### Scenario: comma between arms

- **WHEN** `200 => 1, _ => 0` appears — arms separated by a comma on one line
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic; no separator token is ratified between arms

### Requirement: Pattern set

The ratified pattern set is closed. Literal patterns: a chapter-1 literal — numeric, string, rune, or boolean — matching a scrutinee equal to the literal. Binding patterns: an identifier other than `_`, matching any scrutinee and binding the value to the name for that arm. The wildcard `_`: matches any scrutinee and binds nothing; `_` is never a binding name. Or-patterns `p1 | p2`: match when any branch pattern matches. Guarded patterns `pattern if cond`: match when the pattern matches and `cond` evaluates to true. The `|` and `if` are chapter-1 tokens, and pattern position is a grammar position disjoint from expression position, so no ambiguity with the bitwise or logical operators arises. Variant patterns `Name(args)` and tuple patterns `(a, b)` are not ratified: they enter through a paired amendment when sum types and tuples are ratified by the types chapter; until then they are rejected under chapter 2's unexpected-token diagnostic. `_exh_ignore` does not exist: the wildcard is `_` alone.

#### Scenario: literal pattern

- **WHEN** an arm's pattern is a chapter-1 literal, for example `200`
- **THEN** the arm matches a scrutinee equal to the literal

#### Scenario: wildcard pattern

- **WHEN** an arm's pattern is `_`
- **THEN** it matches any scrutinee and binds nothing

#### Scenario: binding pattern

- **WHEN** an arm's pattern is an identifier other than `_`, for example `n`
- **THEN** it matches any scrutinee, binding the value to `n` inside that arm

#### Scenario: or-pattern

- **WHEN** an arm's pattern is `200 | 404`
- **THEN** the arm matches when either branch's pattern matches

#### Scenario: variant pattern is not ratified

- **WHEN** `Point(0, 0)` appears as an arm pattern
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic until the paired amendment ratifies variant patterns

### Requirement: Or-pattern binding consistency

All branches of one or-pattern bind exactly the same set of names; a mismatch is rejected with `E0302`. The check is syntactic — names only; agreement of the bound names' types is ratified by the types chapter.

#### Scenario: consistent or-pattern

- **WHEN** an or-pattern's branches bind the same set of names, including binding none, for example `200 | 404`
- **THEN** the or-pattern is accepted

#### Scenario: inconsistent or-pattern

- **WHEN** an or-pattern's branches bind different names, for example `a | b`
- **THEN** the compiler rejects it with `E0302:` or-pattern branches must bind the same names

### Requirement: Guards

A guarded pattern is `pattern if cond`: `cond` is an expression evaluated only when the pattern has matched, and it may reference the pattern's bindings. Guarded arms do not count toward exhaustiveness; the exhaustiveness rules — including that a match containing guarded arms must retain an unguarded exhaustive fallback — are ratified by the types chapter. This chapter fixes the form, the laziness, and the scoping only. The condition's typing — that it must be the boolean type — is likewise ratified by the types chapter.

#### Scenario: the guard sees the pattern's bindings

- **WHEN** an arm's pattern is `n if n > limit`
- **THEN** the guard's condition may reference `n`

#### Scenario: the guard is lazy

- **WHEN** an arm's pattern does not match the scrutinee and that arm carries a guard
- **THEN** the guard's condition is not evaluated

### Requirement: Match diagnostics segment

The match chapter owns registry segment `E0300`–`E0399` declared in `docs/spec/diagnostics.toml` `[segments]`. First allocations: `E0301` match requires at least one arm, `E0302` or-pattern branches must bind the same names. `E0300` and `E0303`–`E0399` are reserved: typing-level match diagnostics — exhaustiveness, guard coverage, arm agreement — enter through later amendments allocating within this segment. Trigger semantics live in this chapter's Requirements; entries live in the registry.

#### Scenario: a match code is emitted

- **WHEN** any `E03xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `0400-match`

#### Scenario: a typing amendment needs match diagnostics

- **WHEN** the types chapter ratifies match typing rules requiring new diagnostics
- **THEN** its change allocates numbers within `E0300`–`E0399` by extending the registry in the same change

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–4. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Variant and tuple patterns are annotated as pending their paired amendment.

### match as a value

```we
let x = match code {
    200 => "ok"
    _ => "other"
}
// match is an expression: x receives the taken arm's body value

let y = match code {
    200 => 1
    _ => {
        log("unexpected status")
        0
    }
}
// a block body is one rule with an expression body: blocks are expressions
```

### patterns

```we
match status {
    200 => handleOk()          // literal pattern
    404 | 410 => handleGone()  // or-pattern, no bindings: consistent
    other => handleOther(other) // binding pattern: any value, binds for the arm
    _ => handleAny()           // wildcard: any value, binds nothing
}
// arms are tried top to bottom; the first match wins, so the wildcard
// arm above is unreachable in VALUE terms — reachability rules arrive
// with the types chapter (ratified as non-goal here)

match n {
    n2 if n2 > limit => "big"  // guarded pattern; the guard sees the
    _ => "small"               // binding, and is evaluated only if the
}                              // pattern matched

match p {
    Point(0, 0) => "origin"    // E0105: variant patterns arrive with the
    _ => "elsewhere"           // sum-types paired amendment
}
```

### or-pattern binding consistency

```we
match code {
    200 | 404 => "known"       // branches bind the same names: none, OK
    a | b => "either"          // E0302: or-pattern branches must bind
    _ => "?"                   // the same names
}
```

### rejected forms

```we
let x = match code { }         // E0301: match requires at least one arm

let y = match code { 200 => 1, _ => 0 }
//                               ^ rejected under chapter 2's unexpected-token
// diagnostic: arms are newline-separated, no separator token exists
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| match expression | match 表达式 |
| scrutinee | 匹配对象 |
| arm | 分支臂 |
| pattern | 模式 |
| literal pattern | 字面量模式 |
| binding pattern | 绑定模式 |
| wildcard | 通配符 |
| or-pattern | 或模式 |
| guard | 守卫 |
| exhaustiveness | 穷尽性 |
