## MODIFIED Requirements

### Requirement: Expression skeleton

Primary expressions are: identifiers, literals ratified by chapter 1, parenthesized expressions `( e )` — where, per chapter 8, `( e1, e2, ..., en )` is a tuple expression, `( )` is the unit value, and a parenthesized single expression is grouping only — record construction expressions `TypeRef { field: expr, ... }` per chapter 8, whose head is a named type reference: a PascalCase identifier or `module.Name`, and list literals `[e1, e2, ..., en]` per the collections chapter — the same square brackets that bracket line-joining, there holding a literal's elements. Postfix forms are member access `receiver.name`, call `expr(args)`, and the error-propagation postfix `expr?` of chapter 14, chaining left-associatively; member access on a record denotes its field per chapter 8, and whether an accessed name is a field or a method is resolved by chapter 10's member name resolution. Unary prefix operators are `!`, `-`, `~`, binding tighter than every binary operator. Keyword-led expression forms are ratified by their owning chapters: the control-flow chapter ratifies `if` with else, the match chapter ratifies `match`; they sit outside the primary/postfix/unary skeleton, do not modify it, and a keyword-led expression form enters only through a spec-layer change in its owning chapter. The fn-types chapter ratifies the closure forms — the keyword-led `fn(params) -> type block` and the `|params| body` short form — which sit outside this skeleton likewise; their grammar, typing, and positional rules are that chapter's. Index syntax `expr[expr]` is not a form of this language: the collections chapter ratifies indexing by named methods only, and a revision of that policy there is the sole route to a bracketed index production — this skeleton holds none. `[` and `]` remain lexical tokens.

#### Scenario: Postfix chains group left-to-right

- **WHEN** `a.f(x).g(y)` is parsed
- **THEN** the postfixes apply in sequence from the primary outward — access `f`, call, access `g`, call — which is the grouping `(((a.f)(x)).g)(y)`; access and call chain left-to-right

#### Scenario: The propagation postfix chains

- **WHEN** `f()?.name` is parsed
- **THEN** it groups as `(f()?).name` — the propagation postfix is a postfix of this skeleton, granted by chapter 14, and member access lands on the unwrapped payload

#### Scenario: Parentheses group exactly

- **WHEN** `( e )` appears at any expression position
- **THEN** the parentheses fix the grouping of `e`; the parenthesized form is interchangeable with `e` except where this chapter requires explicit grouping

#### Scenario: A keyword-led expression form is used

- **WHEN** `if` with else or `match` appears at an expression position
- **THEN** the form follows its owning chapter's requirements; the primary, postfix, and unary layers of this skeleton are unchanged

#### Scenario: A closure form is used

- **WHEN** `fn(x: Int64) -> Int64 { x }` or `|x: Int64| x` appears at an expression position
- **THEN** the form follows the fn-types chapter's requirements; the primary, postfix, and unary layers of this skeleton are unchanged

#### Scenario: A construction expression is a primary

- **WHEN** `User { id: UserId(1), name: "Ada" }` appears at an expression position
- **THEN** it is a primary expression under chapter 8's construction rules; postfixes chain onto it as onto any primary

#### Scenario: A list literal is a primary

- **WHEN** `[1, 2, 3]` appears at an expression position
- **THEN** it is a primary expression under the collections chapter's rules; postfixes chain onto it as onto any primary

### Requirement: Grammar diagnostics segment

The grammar chapter owns registry segment `E0100`–`E0199` declared in `docs/spec/diagnostics.toml` `[segments]`. First allocations: `E0101` parse ambiguity, `E0102` statement begins with a continuation token, `E0103` assignment is not an expression, `E0104` chained non-associative operator, `E0105` unexpected token. `E0100` and `E0106`–`E0199` are reserved. Trigger semantics live in this chapter's Requirements; entries live in the registry. Allocating further grammar codes extends the registry in the same change.

#### Scenario: A token fits no ratified production

- **WHEN** at the current parse position a token fits no ratified production — for example a stray closing bracket, or an index form `list[0]`, a construct no production of this language holds
- **THEN** the compiler rejects it with `E0105:` unexpected token, naming the token and the productions considered at that position

#### Scenario: A grammar code is emitted

- **WHEN** any `E01xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry — severity, title, description, remediation, owner `0200-grammar` — is retrievable from `docs/spec/diagnostics.toml`

#### Scenario: A later change needs grammar diagnostics

- **WHEN** a later chapter ratifies grammar productions requiring new diagnostics, for example control flow
- **THEN** its change allocates numbers within `E0100`–`E0199` by extending the registry in the same change; numbers outside the segment fail validation
