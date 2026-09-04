## MODIFIED Requirements

### Requirement: Variant constructors

A variant with payload is constructed by the call form `Name(e1, ..., ek)`, k matching the declared payload arity, each `ei` an expression of its payload type at an agreement position (`E0501` on mismatch); the constructed expression's type is the enclosing sum type. A unit variant is constructed by its bare name — a PascalCase identifier expression whose type is the sum type. A payloaded variant's bare name is not a first-class value — the fn-types chapter answered the question negatively: construction is the call form only, and a payloaded constructor used without its payload is rejected with `E0704`; when a constructor function is wanted, a closure wraps it, `|r| Circle(r)`. A public variant of an imported module is reached as `module.Name(args)` per chapter 6, the same qualified form as every other module-level name. The one name space guarantees a name is never both a function and a variant (`E0404`), so the call form is never ambiguous.

#### Scenario: Construct with payload

- **WHEN** `Circle(1.0)` appears where a `Shape` is required, `type Shape = Circle(Float64) | Rectangle(Float64, Float64)` declared
- **THEN** it is an expression of type `Shape` carrying the payload

#### Scenario: A unit variant stands bare

- **WHEN** `None` appears where an `Option` is required, `None` a declared unit variant
- **THEN** the bare PascalCase identifier is the constructed value; no call form is written

#### Scenario: A payload type mismatch is rejected

- **WHEN** `Circle("big")` appears with `Circle(Float64)` declared
- **THEN** the compiler rejects the argument under `E0501`; the fix is an explicit conversion

#### Scenario: A payloaded constructor without its payload is rejected

- **WHEN** `Circle` appears as a value, `Circle(Float64)` declared
- **THEN** the compiler rejects it with `E0704:` payloaded variant constructor used without arguments; construct by the call form, or wrap a closure when a constructor function is wanted
