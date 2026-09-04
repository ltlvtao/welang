## MODIFIED Requirements

### Requirement: Type references

A type reference — the form filling every type-annotation slot ratified by chapters 2 and 6 — is a named type or a structural composite: a named type is a PascalCase identifier, optionally module-qualified as `module.Name` reaching an imported module's public type per chapter 6, and optionally a generic application `Name<T1, ..., Tk>` per chapter 10, the arguments type references and the arity the declaration's own clause's; a tuple type is `(T1, T2, ..., Tn)` with n from 2 to 8, each element itself a type reference; the unit type is `()`; and `Dyn<Interface>` per chapter 10 names a type-erased interface box. Function types and every other type syntax do not exist yet; each arrives with its owning chapter through this chapter's amendment. A type slot holding anything but a named reference, a generic application, a `Dyn<Interface>` form, a tuple type, or the unit type MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). Whether a well-formed name resolves to a ratified type is decided with module resolution by the module-system chapter.

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

#### Scenario: Unratified type syntax is rejected

- **WHEN** a type slot holds function-type syntax, for example `fn(Int64) -> Int64`
- **THEN** the compiler rejects it with `E0105:` unexpected token at the slot; the form arrives with its owning chapter
