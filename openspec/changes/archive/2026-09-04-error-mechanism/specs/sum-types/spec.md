## MODIFIED Requirements

### Requirement: The bottom type

`Never` is the bottom type: it has no values; an expression of type `Never` is a computation that never produces one. The forms that produce `Never` are chapter 14's — the standard library's `panic` and `todo` functions; this chapter fixes the type's positions and agreements. `Never` may appear only as a declared function return type: a variable binding, field, parameter, variant payload, or tuple element annotated `Never` is rejected with `E0703`. An expression of type `Never` may appear at any type-agreement position of any expected type — it never produces a value to disagree — the one deliberate exemption from chapter 7's no-implicit-conversion obligation, recorded as such. `Never` drops out of arm agreement: for if under chapter 3 and chapter 8, and for match under chapter 4, an arm whose type is `Never` is excluded from the agreement check, and arms that are all `Never` agree as `Never`.

#### Scenario: A forbidden annotation is rejected

- **WHEN** `let x: Never = ...` or a parameter `f(x: Never)` appears
- **THEN** the compiler rejects it with `E0703:` Never in a non-return annotation position

#### Scenario: A Never arm drops out of agreement

- **WHEN** an if or match arm's type is `Never` and the other arms agree on `Int64`
- **THEN** the construct's type is `Int64`; the `Never` arm is not compared

#### Scenario: All-Never arms agree as Never

- **WHEN** every arm of an if-with-else or match has type `Never`
- **THEN** the construct's type is `Never`

#### Scenario: A Never expression satisfies any return type

- **WHEN** a function declares `-> Int64` and a path's final expression has type `Never`
- **THEN** the path satisfies the declaration; the expression never produces a value
