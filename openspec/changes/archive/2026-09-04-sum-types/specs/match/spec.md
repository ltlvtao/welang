# Delta: match (exhaustiveness, arm agreement, variant patterns)

## ADDED Requirements

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

## MODIFIED Requirements

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

### Requirement: Match diagnostics segment

The match chapter owns registry segment `E0300`–`E0399` declared in `docs/spec/diagnostics.toml` `[segments]`. Allocations: `E0301` match requires at least one arm, `E0302` or-pattern branches must bind the same names, `E0303` variant pattern names no variant of the scrutinee's type, `E0304` variant pattern payload arity mismatch, `E0305` match is not exhaustive, `E0306` guarded match without an unguarded exhaustive fallback, `E0307` unreachable match arm — the typing-level match diagnostics this segment was reserved for, entered through the sum-types chapter's paired amendment. `E0300` and `E0308`–`E0399` remain reserved for later amendments. Trigger semantics live in this chapter's Requirements; entries live in the registry.

#### Scenario: a match code is emitted

- **WHEN** any `E03xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `0400-match`

#### Scenario: a typing amendment needs match diagnostics

- **WHEN** the types chapter ratifies match typing rules requiring new diagnostics
- **THEN** its change allocates numbers within `E0300`–`E0399` by extending the registry in the same change
