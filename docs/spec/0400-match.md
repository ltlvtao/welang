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

The ratified pattern set is closed. Literal patterns: a chapter-1 literal — numeric, string, rune, or boolean — matching a scrutinee equal to the literal. Binding patterns: an identifier other than `_`, matching any scrutinee and binding the value to the name for that arm. The wildcard `_`: matches any scrutinee and binds nothing; `_` is never a binding name. Or-patterns `p1 | p2`: match when any branch pattern matches. Guarded patterns `pattern if cond`: match when the pattern matches and `cond` evaluates to true. Tuple patterns `(p1, p2, ..., pn)`: match a tuple value of the same arity, each sub-pattern any ratified pattern, nesting recursively, per chapter 8. Variant patterns, per chapter 9: `Name(p1, ..., pk)` for a payloaded variant and `Name` alone for a unit variant, the sub-patterns binding from the payload element-wise and nesting recursively; the pattern's `Name` MUST be a declared variant of the scrutinee's sum type (`E0303` otherwise) with exactly the declared payload arity (`E0304` otherwise). The case split is lexical: variant names are PascalCase (`E0011`) and binding names camelCase (`E0012`), so a bare PascalCase pattern is always a variant reference and never a binding. Variant patterns are refutable: they appear in match arms — a binding statement's name position accepts only irrefutable patterns, so `let Circle(r) = s` is rejected under chapter 2's unexpected-token diagnostic (`E0105`). The `|` and `if` are chapter-1 tokens, and pattern position is a grammar position disjoint from expression position, so no ambiguity with the bitwise or logical operators arises. `_exh_ignore` does not exist: the wildcard is `_` alone.

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

#### Scenario: tuple pattern

- **WHEN** an arm's pattern is `(a, b)` and the scrutinee is a tuple of two elements
- **THEN** the arm matches, binding `a` and `b` to the elements per chapter 8

#### Scenario: variant pattern matches and binds

- **WHEN** an arm's pattern is `Circle(r)` and the scrutinee is a `Shape` carrying `Circle(2.0)`
- **THEN** the arm matches, binding `r` to `2.0` from the payload per chapter 9

#### Scenario: nested patterns

- **WHEN** an arm's pattern is `(Circle(r), _)` and the scrutinee is a `(Shape, Shape)` whose first element carries `Circle(1.0)`
- **THEN** the nested variant pattern matches inside the tuple pattern, binding `r` to `1.0`

#### Scenario: an unknown variant is rejected

- **WHEN** an arm's pattern is `Triangle(r)` and the scrutinee's sum type declares no `Triangle`
- **THEN** the compiler rejects it with `E0303:` pattern names no variant of the scrutinee's type

#### Scenario: a payload arity mismatch is rejected

- **WHEN** an arm's pattern is `Circle(a, b)` with `Circle(Float64)` declared
- **THEN** the compiler rejects it with `E0304:` variant pattern payload arity mismatch

#### Scenario: a variant pattern in let is rejected

- **WHEN** `let Circle(r) = s` appears as a binding statement
- **THEN** the compiler rejects it under `E0105:` unexpected token; refutable patterns are match-only

### Requirement: Or-pattern binding consistency

All branches of one or-pattern bind exactly the same set of names; a mismatch is rejected with `E0302`. The name-set check is syntactic; the types are not merely asserted: when branches bind the same name, their bound types MUST agree, and a disagreement is rejected under chapter 7's `E0501` — a name bound at two types in one pattern would be two bindings wearing one name. This grounds the deferred type agreement of or-pattern bindings.

#### Scenario: consistent or-pattern

- **WHEN** an or-pattern's branches bind the same set of names, including binding none, for example `200 | 404`
- **THEN** the or-pattern is accepted

#### Scenario: inconsistent or-pattern

- **WHEN** an or-pattern's branches bind different names, for example `a | b`
- **THEN** the compiler rejects it with `E0302:` or-pattern branches must bind the same names

#### Scenario: the same name at two types is rejected

- **WHEN** an or-pattern's branches bind the same name at different types, for example `(a, _) | (_, a)` against `(Int64, String)`
- **THEN** the compiler rejects it under `E0501`: `a` binds `Int64` in one branch and `String` in the other

### Requirement: Guards

A guarded pattern is `pattern if cond`: `cond` is an expression evaluated only when the pattern has matched, and it may reference the pattern's bindings. Guarded arms do not count toward exhaustiveness: the exhaustiveness rules — including that a match containing guarded arms must retain an unguarded exhaustive fallback — are this chapter's Exhaustiveness requirement's, the type-side facts being chapter 9's. This chapter fixes the form, the laziness, and the scoping only. The condition's typing — that it must be the boolean type — is ratified by chapter 7 (`E0503`).

#### Scenario: the guard sees the pattern's bindings

- **WHEN** an arm's pattern is `n if n > limit`
- **THEN** the guard's condition may reference `n`

#### Scenario: the guard is lazy

- **WHEN** an arm's pattern does not match the scrutinee and that arm carries a guard
- **THEN** the guard's condition is not evaluated

### Requirement: Exhaustiveness

A match MUST be exhaustive: its unguarded arms' patterns together must match every value of the scrutinee's type. A sum type's values are its variants, so covering every variant — by named variant patterns or or-patterns over them — exhausts it. The base types cannot be exhausted: literal patterns never cover `Int64`, `String`, or `Bool`, so a match on a base type MUST retain a wildcard arm, as must a match on any type whose value set is not enumerated. Tuple scrutinees follow chapter 8: a tuple type is exhaustible exactly insofar as its element types are. A non-exhaustive match is rejected with `E0305`. Guarded arms do not count toward exhaustiveness — a match containing a guarded arm MUST retain an unguarded exhaustive fallback, else `E0306` — so adding a guard is purely a refinement of an arm, never a weakening of coverage. An arm that can never match — its every value already covered by earlier arms, as an arm after a wildcard, or a variant listed twice — is rejected with `E0307`. Which types enumerate their values is the type chapters' fact; this requirement fixes the match-level check.

#### Scenario: Covering every variant exhausts

- **WHEN** a match on `s: Shape` of exactly those variants holds the arms `Circle(r) => 1` and `Rectangle(w, h) => 2`
- **THEN** the match is exhaustive without a wildcard; the check is static

#### Scenario: A missing variant is rejected

- **WHEN** a match on `Shape` covers `Circle` but not `Rectangle`, with no wildcard
- **THEN** the compiler rejects it with `E0305:` match is not exhaustive, naming the uncovered variant

#### Scenario: A base-type match needs a wildcard

- **WHEN** a match on `Int64` holds literal arms `200` and `404` with no wildcard
- **THEN** the compiler rejects it with `E0305`; literal patterns never exhaust a base type, `Bool` included

#### Scenario: Guarded arms do not count

- **WHEN** a match on `Shape` holds `Circle(r) if r > 1.0 => ...` and `Rectangle(w, h) => ...` with no unguarded fallback for `Circle`
- **THEN** the compiler rejects it with `E0306:` guarded match without an unguarded exhaustive fallback

#### Scenario: An arm after a wildcard is unreachable

- **WHEN** a wildcard arm is followed by another arm, for example `_ => 0` then `Circle(r) => 1`
- **THEN** the compiler rejects the later arm with `E0307:` unreachable match arm

#### Scenario: A variant listed twice is unreachable

- **WHEN** a match on `Shape` holds two `Circle` arms with no wildcard between them
- **THEN** the compiler rejects the second with `E0307:` unreachable match arm

#### Scenario: A tuple of sums is exhaustible element-wise

- **WHEN** a match on `(Shape, Shape)` enumerates every combination of variants with no wildcard
- **THEN** the match is exhaustive per chapter 8's element-wise rule

### Requirement: Match arm agreement

Used as a value, a match's arms MUST agree in type: every arm's body type is the same type, where an arm whose type is `Never` is excluded per chapter 9 and arms that are all `Never` agree as `Never`. Disagreement is rejected under chapter 7's `E0501` at the match expression. Used as a statement, a match whose agreed type is not the unit type falls under chapter 8's value-discard rule — the unbound match-arm body in statement position named there. This grounds the arm-agreement topic this chapter's diagnostics segment reserved.

#### Scenario: Arms disagree in type

- **WHEN** a match at an expression position holds the arms `Circle(r) => 1` and `Rectangle(w, h) => "wide"`
- **THEN** the compiler rejects it under `E0501`: the arms are `Int64` and `String`

#### Scenario: Unit arms stand as a statement

- **WHEN** every arm of a statement-position match has the unit type
- **THEN** the match stands freely; no discard form is required

#### Scenario: A non-unit statement match is rejected

- **WHEN** a statement-position match's arms agree on `Int64`
- **THEN** the compiler rejects it with `E0605:` non-unit value dropped; the fix is `let _ = match ...` or consuming the value

### Requirement: Match diagnostics segment

The match chapter owns registry segment `E0300`–`E0399` declared in `docs/spec/diagnostics.toml` `[segments]`. Allocations: `E0301` match requires at least one arm, `E0302` or-pattern branches must bind the same names, `E0303` variant pattern names no variant of the scrutinee's type, `E0304` variant pattern payload arity mismatch, `E0305` match is not exhaustive, `E0306` guarded match without an unguarded exhaustive fallback, `E0307` unreachable match arm — the typing-level match diagnostics this segment was reserved for, entered through the sum-types chapter's paired amendment. `E0300` and `E0308`–`E0399` remain reserved for later amendments. Trigger semantics live in this chapter's Requirements; entries live in the registry.

#### Scenario: a match code is emitted

- **WHEN** any `E03xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `0400-match`

#### Scenario: a typing amendment needs match diagnostics

- **WHEN** the types chapter ratifies match typing rules requiring new diagnostics
- **THEN** its change allocates numbers within `E0300`–`E0399` by extending the registry in the same change

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–4. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Tuple patterns are chapter 8's and variant patterns chapter 9's; both are illustrated in their own chapters.

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
    other => handleOther(other) // binding pattern: any value, binds for
}                               // the arm — and exhausts: nothing may
                                // follow it (E0307)

match n {
    n2 if n2 > limit => "big"  // guarded pattern; the guard sees the
    _ => "small"               // binding, and is evaluated only if the
}                              // pattern matched; the unguarded wildcard
                               // is the exhaustive fallback (E0306)
```

### exhaustiveness and arm agreement

```we
match level {                 // level is Int64: a base type
    0 => off()
    _ => on()                 // required: literals never exhaust a
}                             // base type (E0305)

match level {
    0 => off()                // E0305: match is not exhaustive —
}                             // literals never exhaust a base type

match level {
    n if n > 3 => loud()
    n => quiet(n)             // the unguarded fallback exhausts; a
}                             // guarded arm alone would be E0306

match level {
    _ => off()
    0 => on()                 // E0307: unreachable match arm
}

let label = match code {      // value position: the arms agree (all
    200 => "fine"             // String), so the match has a type
    _ => "other"
}

match code {                  // statement position, arms agreeing on
    200 => "fine"             // String: non-unit value dropped (E0605,
    _ => "other"              // chapter 8) — `let _ = match ...` or a
}                             // unit-bodied arm set
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
| exhaustive fallback | 穷尽兜底 |
| variant pattern | 变体模式 |
| unreachable arm | 不可达臂 |
| arm agreement | 臂类型一致 |
