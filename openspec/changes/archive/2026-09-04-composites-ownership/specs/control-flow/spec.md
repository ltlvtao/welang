# Delta: control-flow (host amendment for composites)

## MODIFIED Requirements

### Requirement: If and else expressions

An if expression is `if cond block` or `if cond block else else-arm`, where `cond` is an expression and `else-arm` is a block or another if expression (chains terminate in a block or an if without else). An if without else produces no value and MUST NOT appear where a value is required (`E0202` — statically decidable from the missing else). An if with else is an expression form: on each evaluation its value is the taken arm's block value under chapter 2's Blocks and block value. The arms of an if-with-else MUST agree in type per chapter 8: an arm whose block has no value types as the unit type, and a disagreement is rejected under chapter 7's `E0501`; the never-type's exclusion from agreement is the sum-types chapter's. This chapter fixes the form and the arm-scoping. Each arm introduces its own scope: bindings inside an arm MUST NOT leak past the if. The condition's typing — that it must be the boolean type — is ratified by the types chapter.

#### Scenario: if with else used as a value

- **WHEN** `let x = if c { 1 } else { 2 }` is parsed and `c` holds true
- **THEN** `x` receives the then-arm's block value, here `1`; arm block values follow chapter 2 unchanged

#### Scenario: if without else in a value position

- **WHEN** `let x = if c { 1 }` appears — an if with no else used where a value is required
- **THEN** the compiler rejects it with `E0202:` valueless form in value position, naming the form that produces no value

#### Scenario: Arm bindings do not leak

- **WHEN** an arm binds a name, for example `if c { let a = 1 a } else { 0 }`
- **THEN** `a` is not visible after the if expression; each arm is its own scope

#### Scenario: else-if chain

- **WHEN** `if c1 { a } else if c2 { b } else { c }` is parsed
- **THEN** the else arm holds one nested if expression; the chain is two if expressions, each following this Requirement

#### Scenario: One arm values, one does not

- **WHEN** `if c { 1 } else { io.println("no") }` appears at an expression position
- **THEN** the compiler rejects it under `E0501` per chapter 8: the first arm is `Int64`, the second is the unit type; the arms must agree
