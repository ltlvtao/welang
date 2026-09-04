# Delta: types (host amendments for composites)

## MODIFIED Requirements

### Requirement: Type references

A type reference — the form filling every type-annotation slot ratified by chapters 2 and 6 — is a named type or a structural composite: a named type is a PascalCase identifier, optionally module-qualified as `module.Name` reaching an imported module's public type per chapter 6; a tuple type is `(T1, T2, ..., Tn)` with n from 2 to 8, each element itself a type reference; the unit type is `()`. Function types, generic applications, and every other type syntax do not exist yet; each arrives with its owning chapter through this chapter's amendment. A type slot holding anything but a named reference, a tuple type, or the unit type MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). Whether a well-formed name resolves to a ratified type is decided with module resolution by the module-system chapter.

#### Scenario: A qualified type reference

- **WHEN** an annotation holds `user.User` where `user` names an imported module
- **THEN** it is a type reference to that module's public `User` type

#### Scenario: A tuple type in an annotation slot

- **WHEN** an annotation holds `(Int64, String)`
- **THEN** it is the type reference of a two-element tuple, per chapter 8's arity bounds

#### Scenario: Unratified type syntax is rejected

- **WHEN** a type slot holds function-type or generic syntax, for example `fn(Int64) -> Int64` or `List<Int64>`
- **THEN** the compiler rejects it with `E0105:` unexpected token at the slot; those forms arrive with their owning chapters

### Requirement: Block value typing

A block with a value has the type of its final expression item; a block without a value — final item a statement, or empty — has the unit type `()`, which chapter 8 ratifies and frees from the discard obligation. This grounds chapter 2's deferred block-value typing.

#### Scenario: A block's type is its final expression's type

- **WHEN** a block ends in `a + b` with `a`, `b`: `Int64`
- **THEN** the block's type is `Int64` per chapter 2's Blocks and block value

#### Scenario: A valueless block has the unit type

- **WHEN** a block's final item is a statement or the block is empty
- **THEN** the block has no value and its type is `()`; it may stand where the unit type is required, per chapter 8
