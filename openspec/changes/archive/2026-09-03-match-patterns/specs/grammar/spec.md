# Delta: grammar (Expression skeleton growth rule)

## MODIFIED Requirements

### Requirement: Expression skeleton

Primary expressions are: identifiers, literals ratified by chapter 1, and parenthesized expressions `( e )`. Postfix forms are member access `receiver.name` and call `expr(args)`, chaining left-associatively; whether an accessed name is a field or a method is resolved by the types chapter. Unary prefix operators are `!`, `-`, `~`, binding tighter than every binary operator. Keyword-led expression forms are ratified by their owning chapters: the control-flow chapter ratifies `if` with else, the match chapter ratifies `match`; they sit outside the primary/postfix/unary skeleton, do not modify it, and a keyword-led expression form enters only through a spec-layer change in its owning chapter. Index syntax `expr[expr]` is deferred to the collections chapter; `[` and `]` remain lexical tokens.

#### Scenario: Postfix chains group left-to-right

- **WHEN** `a.f(x).g(y)` is parsed
- **THEN** the postfixes apply in sequence from the primary outward — access `f`, call, access `g`, call — which is the grouping `(((a.f)(x)).g)(y)`; access and call chain left-to-right

#### Scenario: Parentheses group exactly

- **WHEN** `( e )` appears at any expression position
- **THEN** the parentheses fix the grouping of `e`; the parenthesized form is interchangeable with `e` except where this chapter requires explicit grouping

#### Scenario: A keyword-led expression form is used

- **WHEN** `if` with else or `match` appears at an expression position
- **THEN** the form follows its owning chapter's requirements; the primary, postfix, and unary layers of this skeleton are unchanged
