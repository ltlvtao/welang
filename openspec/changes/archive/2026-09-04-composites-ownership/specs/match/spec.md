# Delta: match (host amendment for composites)

## MODIFIED Requirements

### Requirement: Pattern set

The ratified pattern set is closed. Literal patterns: a chapter-1 literal — numeric, string, rune, or boolean — matching a scrutinee equal to the literal. Binding patterns: an identifier other than `_`, matching any scrutinee and binding the value to the name for that arm. The wildcard `_`: matches any scrutinee and binds nothing; `_` is never a binding name. Or-patterns `p1 | p2`: match when any branch pattern matches. Guarded patterns `pattern if cond`: match when the pattern matches and `cond` evaluates to true. Tuple patterns `(p1, p2, ..., pn)`: match a tuple value of the same arity, each sub-pattern any ratified pattern, nesting recursively, per chapter 8. The `|` and `if` are chapter-1 tokens, and pattern position is a grammar position disjoint from expression position, so no ambiguity with the bitwise or logical operators arises. Variant patterns `Name(args)` are not ratified: they enter through a paired amendment when sum types are ratified by the sum-types chapter; until then they are rejected under chapter 2's unexpected-token diagnostic. `_exh_ignore` does not exist: the wildcard is `_` alone.

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

#### Scenario: variant pattern is not ratified

- **WHEN** `Point(0, 0)` appears as an arm pattern
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic until the paired amendment of the sum-types chapter ratifies variant patterns
