## MODIFIED Requirements

### Requirement: The function type

A function type is `fn(T1, ..., Tn) [effect-segment] -> T`: the `fn` keyword, a parenthesized comma-separated list of zero or more parameter types, each itself a type reference under chapter 7, an optional effect segment of one or more bare space-separated tags between the parameter types and the arrow — chapter 16's ratification, no `effect` keyword in type position — an arrow, and a return type, itself a type reference and REQUIRED — a function that produces no value has the type `fn(...) -> ()`, the unit type under chapter 8. Omitting the segment states the pure function type; the tags' declarations and resolution, and the agreement checks the segment participates in, are chapter 16's. The parameter list carries no arity cap, mirroring chapter 6's declaration surface. A function type is monomorphic: its parameter and return positions hold type references only, never generic parameters — a generic function names a family, not one function, and has no single function type (Function values). Type slots accept the function type as a type-reference form under chapter 7's amendment.

#### Scenario: A function type in an annotation slot

- **WHEN** `let f: fn(Int64) -> Int64` appears with a matching function value on the right
- **THEN** the annotation is the function type of exactly that signature; `f` binds function values of no other

#### Scenario: An effect segment in a function type

- **WHEN** `let f: fn(Int64) io -> Int64` appears — bare space-separated tags between the parameter types and the arrow
- **THEN** the annotation is the function type of Int64-to-Int64 functions that may perform io; the segment's spelling is bare tags, the `effect` keyword fitting no production in type position

#### Scenario: A zero-parameter function type

- **WHEN** `fn() -> Bool` appears in a type slot
- **THEN** it is the type of zero-parameter functions returning `Bool`

#### Scenario: A valueless signature types through the unit type

- **WHEN** `fn(String) -> ()` appears in a type slot
- **THEN** it is the type of functions that produce no value; the return position is the unit type, never omitted

#### Scenario: A malformed function type is rejected

- **WHEN** a type slot holds `fn(Int64)` with no arrow, or any other malformed function-type fragment
- **THEN** the compiler rejects it under chapter 2's unexpected-token diagnostic (`E0105`) at the slot; the arrow and the return type are part of the production

#### Scenario: A function type nests in composite positions

- **WHEN** a tuple type `(fn(Int64) -> Int64, Int64)` or a generic application `Box<fn(Int64) -> Int64>` appears in a type slot
- **THEN** the function type is a type-reference form like any other; chapter 8's tuple and chapter 10's application rules apply unchanged
