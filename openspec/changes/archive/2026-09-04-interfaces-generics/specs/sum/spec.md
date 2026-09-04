## MODIFIED Requirements

### Requirement: Sum declarations

A sum type declaration is a top-level item `type Name = V1 | V2 | ... | Vn` with n at least one, optionally prefixed by `pub` and `byval`, and optionally carrying a generic parameter clause after the name and a derives clause after the last variant, both per chapter 10: `Name` is the sum type's name and each `Vi` a variant — `Unit` alone for a unit variant, or `Unit(T1, T2, ..., Tk)` with k from 1 to 8, each `Ti` a type reference under chapter 7 as amended. There is no `|` before the first variant. A payload arity above eight is rejected with `E0701` — the fix is a record payload, which names its fields. The type's name and every variant's name are PascalCase (`E0011`) and join the module's one name space under chapter 6, colliding with no other name (`E0404`). Layout follows chapter 2: the `|` between variants is an operator token, so a declaration spanning lines carries `=` or `|` at each line's end — a line starting with `|` after a complete variant is rejected under chapter 2's continuation diagnostic (`E0102`). A sum type is a composite type under chapter 8: `type` is the `gc` category and `byval type` the `value` category; the resource and newtype categories do not apply to sums, and a sum type MUST NOT change category after declaration. A generic payload's category honesty and derive requirements are checked at each instantiation under chapter 10.

#### Scenario: A sum declaration parses as a top-level item

- **WHEN** `type Shape = Circle(Float64) | Rectangle(Float64, Float64)` appears at the top level
- **THEN** it declares the gc sum type `Shape` with variants `Circle` and `Rectangle` and their payload types

#### Scenario: A multi-line declaration trails its separators

- **WHEN** a declaration is written `type Shape =` on one line, `Circle(Float64) |` on the next, and `Rectangle(Float64, Float64)` on the last
- **THEN** the lines continue per chapter 2's continuation set and the declaration parses as one item

#### Scenario: A leading separator on its own line is rejected

- **WHEN** a declaration is written `type Shape = Circle(Float64)` on one line and `| Rectangle(Float64, Float64)` starting the next
- **THEN** the first line already ends at a boundary and the next begins with `|`, a token that cannot begin a statement; the compiler rejects it with `E0102:` statement begins with a continuation token

#### Scenario: A payload arity above eight is rejected

- **WHEN** a variant with nine payload types appears
- **THEN** the compiler rejects it with `E0701:` variant payload arity above eight; the fix is a record payload

#### Scenario: A variant name collides in the module name space

- **WHEN** a module declares `type Shape = Circle(Float64)` and also `fn Circle(r: Float64)`
- **THEN** the compiler rejects the second declaration with `E0404:` duplicate name in one module

#### Scenario: byval declares the value category

- **WHEN** `byval type Axis = X(Float64) | Y(Float64)` appears
- **THEN** it declares a value-category sum type under chapter 8's ownership categories

#### Scenario: A generic sum with a derives clause parses

- **WHEN** `type Result<T> = Ok(T) | Err(String) derives Eq` appears at the top level
- **THEN** it declares a one-parameter gc sum per chapter 10 whose `.equals` requirement on `T` is checked at each instantiation
