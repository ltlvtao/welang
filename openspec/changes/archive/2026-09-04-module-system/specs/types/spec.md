## MODIFIED Requirements

### Requirement: Type references

A type reference — the form filling every type-annotation slot ratified by chapters 2 and 6 — is a named type or a structural composite: a named type is a PascalCase identifier, optionally module-qualified as `module.Name` reaching an imported module's public type per chapter 6, and optionally a generic application `Name<T1, ..., Tk>` per chapter 10, the arguments type references and the arity the declaration's own clause's; a tuple type is `(T1, T2, ..., Tn)` with n from 2 to 8, each element itself a type reference; the unit type is `()`; `Dyn<Interface>` per chapter 10 names a type-erased interface box; and a function type `fn(T1, ..., Tn) -> T` per the fn-types chapter is a type-reference form — zero or more comma-separated parameter types, each a type reference, an arrow, and a required return type, itself a type reference with the unit type `()` in the valueless position; its grammar and agreement are that chapter's. Every other type syntax does not exist yet; it arrives with its owning chapter through this chapter's amendment. A type slot holding anything but a named reference, a generic application, a `Dyn<Interface>` form, a tuple type, the unit type, or a function type MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). Whether a well-formed name resolves to a ratified type is decided by chapter 15's name resolution — bare names through scopes innermost-first to the prelude, qualified names through the import.

#### Scenario: A qualified type reference

- **WHEN** an annotation holds `user.User` where `user` names an imported module
- **THEN** it is a type reference to that module's public `User` type

#### Scenario: A tuple type in an annotation slot

- **WHEN** an annotation holds `(Int64, String)`
- **THEN** it is the type reference of a two-element tuple, per chapter 8's arity bounds

#### Scenario: A generic application in an annotation slot

- **WHEN** an annotation holds `Box<Int64>` with `record Box<T>` declared
- **THEN** it is the type reference of the generic application, `T` taken as `Int64` per chapter 10

#### Scenario: A Dyn form in an annotation slot

- **WHEN** an annotation holds `Dyn<Describable>` with `Describable` a declared interface
- **THEN** it is the type reference of the type-erased box per chapter 10

#### Scenario: A function type in an annotation slot

- **WHEN** an annotation holds `fn(Int64) -> Int64`
- **THEN** it is the function-type form per the fn-types chapter; the slot holds that signature's functions

#### Scenario: Unratified type syntax is rejected

- **WHEN** a type slot holds an effect-carrying function type such as `fn(Int64) io -> Int64`, or any other unratified type syntax
- **THEN** the compiler rejects it with `E0105:` unexpected token at the slot; the form arrives, if ever, with its owning chapter
