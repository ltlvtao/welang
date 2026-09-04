## MODIFIED Requirements

### Requirement: Expression skeleton

Primary expressions are: identifiers, literals ratified by chapter 1, parenthesized expressions `( e )` — where, per chapter 8, `( e1, e2, ..., en )` is a tuple expression, `( )` is the unit value, and a parenthesized single expression is grouping only — and record construction expressions `TypeRef { field: expr, ... }` per chapter 8, whose head is a named type reference: a PascalCase identifier or `module.Name`. Postfix forms are member access `receiver.name` and call `expr(args)`, chaining left-associatively; member access on a record denotes its field per chapter 8, and whether an accessed name is a field or a method is resolved by chapter 10's member name resolution. Unary prefix operators are `!`, `-`, `~`, binding tighter than every binary operator. Keyword-led expression forms are ratified by their owning chapters: the control-flow chapter ratifies `if` with else, the match chapter ratifies `match`; they sit outside the primary/postfix/unary skeleton, do not modify it, and a keyword-led expression form enters only through a spec-layer change in its owning chapter. The fn-types chapter ratifies the closure forms — the keyword-led `fn(params) -> type block` and the `|params| body` short form — which sit outside this skeleton likewise; their grammar, typing, and positional rules are that chapter's. Index syntax `expr[expr]` is deferred to the collections chapter; `[` and `]` remain lexical tokens.

#### Scenario: Postfix chains group left-to-right

- **WHEN** `a.f(x).g(y)` is parsed
- **THEN** the postfixes apply in sequence from the primary outward — access `f`, call, access `g`, call — which is the grouping `(((a.f)(x)).g)(y)`; access and call chain left-to-right

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
