## MODIFIED Requirements

### Requirement: Record declarations

A record declaration is a top-level item `record Name { fields }`, optionally prefixed by `byval` or `byres` and by `pub`, and optionally carrying a generic parameter clause after the name and a derives clause after the brace group, both per chapter 10; the fields are zero or more `name: type` pairs separated by commas, each type a type reference under chapter 7 (as amended by this chapter and by chapter 10); field names follow chapter 1's variable rule — camelCase (`E0012`); the record's name is PascalCase (`E0011`). A record with zero fields is legal. A generic field's category honesty and derive requirements are checked at each instantiation under chapter 10. Record and newtype names join the module's one name space under chapter 6 and MUST NOT collide with any other name (`E0404`).

#### Scenario: A record parses as a top-level item

- **WHEN** `record User { id: UserId, name: String }` appears at the top level
- **THEN** it declares the gc record `User` with fields `id: UserId` and `name: String`

#### Scenario: An empty record is legal

- **WHEN** `byval record Empty { }` appears
- **THEN** it declares a value-category record with no fields; an empty record of value category carries no data

#### Scenario: A record name collides in the module name space

- **WHEN** a module declares `record User { ... }` and `fn User() { ... }`
- **THEN** the compiler rejects the second declaration with `E0404:` duplicate name in one module

#### Scenario: A generic record with a derives clause parses

- **WHEN** `record Box<T> { value: T } derives Eq` appears at the top level
- **THEN** it declares a one-parameter gc record per chapter 10 whose `.equals` requirement on `T` is checked at each instantiation

### Requirement: Update expressions

An update expression is `TypeRef { field: expr, ..., with &old }` with the same head forms as a construction: it produces a NEW value of the head's record type where the named fields take the given expressions and every unnamed field is copied from `old`. The base `old` MUST be an expression of exactly the head's record type (`E0603` otherwise); the named field set MUST name only declared fields (`E0604`), and each given expression MUST match its field's type (`E0501`). The base value is unaffected: the update never mutates it. Update expressions exist only for gc and value records: a resource record has identity and MUST NOT be updated this way (`E0606`); its fields change only through the resource chapter's release mechanics and chapter 10's receiver field assignment — the mechanisms this chapter deferred to, now landed.

#### Scenario: An update copies the unnamed fields

- **WHEN** `User { name: "bob" with &u1 }` appears, `User` declaring `id` and `name`, `u1: User`
- **THEN** the result is a new `User` whose `name` is `"bob"` and whose `id` is `u1`'s; `u1` is unchanged

#### Scenario: A non-record base is rejected

- **WHEN** an update expression's base has a type other than the constructed record, for example `Point { x: 1.0 with &origin}` with `origin: User`
- **THEN** the compiler rejects it with `E0603:` update base does not have the constructed record type

#### Scenario: A resource record cannot be updated

- **WHEN** `FileHandle { fd: 3 with &h }` appears, `FileHandle` a byres record
- **THEN** the compiler rejects it with `E0606:` update expression on a resource record

### Requirement: Field access

Postfix member access `receiver.name` on a value of a record type denotes that record's field, grounding chapter 2's deferred field-or-method resolution for the record side: the receiver's type names the record, the accessed name one of its fields, and the expression's type is the field's declared type. The full candidate set — fields together with inherent methods, interface methods, and generated derive methods — is chapter 10's member name resolution: a name that is no field and no method of the receiver's type is rejected there (`E0816`), and method calls are chapter 10's. Field access reads: outside a mut-self method body there is no assignment to a field anywhere in the language — `obj.field = value` fits no production and MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`), regardless of visibility, category, or module; inside one, chapter 10 ratifies exactly `self.field = expr` as the receiver field assignment, the channel this chapter deferred to the interfaces chapter.

#### Scenario: Field access reads a field

- **WHEN** `u.name` appears with `u: User` declaring `name: String`
- **THEN** it is an expression of type `String` naming that field's value

#### Scenario: Field assignment does not exist outside methods

- **WHEN** `u.name = "bob"` appears outside a mut-self method body
- **THEN** the compiler rejects it under `E0105:` unexpected token; the one field-assignment form is chapter 10's `self.field` inside a mut-self body — `u` is no receiver — and modification outside methods is the update expression, which produces a new value

#### Scenario: An unknown member access is rejected

- **WHEN** `u.email` appears with `User` declaring no field or method `email`
- **THEN** the compiler rejects it under chapter 10's member name resolution with `E0816:` no such member on the receiver's type

### Requirement: Newtype declarations

A newtype declaration is a top-level item `newtype Name(Underlying)`, optionally prefixed by `pub`: `Name` is a new type, of the newtype ownership category, whose runtime layout is its underlying type's and is erased — zero overhead, no implicit conversion in either direction. Construction is the call form `Name(expr)` with `expr` of the underlying type; a call whose callee names a newtype is a construction (chapter 6's one name space guarantees a name is never both a function and a newtype, so the call form is never ambiguous). Unwrapping is the field access `.value`, whose type is the underlying type. Mixing a newtype with its underlying type — or with any other type — is rejected under chapter 7's `E0501`. A newtype MAY carry a derives clause per chapter 10 — `newtype UserId(Int64) derives Eq` generates `.equals`; generic newtypes are not ratified. This lands the derives deferral this chapter recorded.

#### Scenario: Construct and unwrap

- **WHEN** `let id = UserId(42)` then `let n = id.value` appear, `newtype UserId(Int64)` declared
- **THEN** `id` is of type `UserId`, and `n` is of type `Int64` with value `42`

#### Scenario: No implicit conversion either way

- **WHEN** `UserId(1) + 1` or `let uid: UserId = 42` appears
- **THEN** the compiler rejects both under `E0501`; the transitions are `UserId(42)` and `.value`, both explicit

#### Scenario: The call form is a construction

- **WHEN** `UserId(42)` appears and the module also declares `fn UserId(x: Int64)` — it does not, because `E0404` forbids the collision
- **THEN** the one name space makes the construction reading the only reading; no ambiguity rule is needed

#### Scenario: A newtype with a derives clause parses

- **WHEN** `newtype UserId(Int64) derives Eq, Show` appears at the top level
- **THEN** it declares the newtype with generated `.equals` and `.toDebugString` methods per chapter 10
