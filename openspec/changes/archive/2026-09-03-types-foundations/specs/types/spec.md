# Delta: types (base types, conversions, overflow, references)

## ADDED Requirements

### Requirement: Base type inventory

The base types are a closed set of thirteen: the checked integers `Int8` `Int16` `Int32` `Int64` `UInt8` `UInt16` `UInt32` `UInt64`, the IEEE 754 floats `Float32` `Float64`, `Bool`, `String` (an immutable UTF-8 byte sequence; logical iteration yields `Rune`, whose protocol is ratified by the iterable-protocols chapter), `Bytes` (a byte sequence), and `Rune` (a single Unicode code point, U+0000..U+10FFFF). `Never` and `()` do not exist yet: the bottom type arrives with the sum-types chapter and the unit type with the composite-types chapter. Adding a base type is a spec-layer change amending this inventory.

#### Scenario: A base type name denotes its type

- **WHEN** a type reference names one of the thirteen base types, for example `Int64` or `String`
- **THEN** it denotes that base type; the names are PascalCase per chapter 1's naming conventions

#### Scenario: The inventory is closed

- **WHEN** a later chapter needs a new base type
- **THEN** it enters only through a spec-layer change amending this Requirement's inventory; no other route exists

### Requirement: Literal typing

An unsuffixed integer literal is `Int64`; an unsuffixed float literal is `Float64`. A suffixed numeric literal denotes the type its suffix names: `i8`..`i64` map to `Int8`..`Int64`, `u8`..`u64` to `UInt8`..`UInt64`, `f32`/`f64` to `Float32`/`Float64`, per chapter 1's closed suffix set. The keyword literals `true` and `false` are `Bool`; a string literal is `String`; a rune literal is `Rune`.

#### Scenario: Unsuffixed literals take the defaults

- **WHEN** the literals `42` and `3.14` appear
- **THEN** `42` is `Int64` and `3.14` is `Float64`

#### Scenario: Suffixed literals name their type

- **WHEN** the literals `42u8`, `1000i32`, and `3.14f32` appear
- **THEN** they are `UInt8`, `Int32`, and `Float32` respectively

#### Scenario: Keyword and delimited literals type themselves

- **WHEN** `true`, `"text"`, and `'x'` appear
- **THEN** they are `Bool`, `String`, and `Rune` respectively

### Requirement: No implicit conversion

We has no implicit conversions. Operands of different types MUST NOT combine: mixed integer widths, signed with unsigned, the two float widths, and any of `String`, `Bytes`, and `Rune` with another are rejected with `E0501`. The obligation covers every position where a value's type must agree with a slot: a binding's expression MUST match its annotation, a call argument MUST match its parameter, and a return expression MUST match the declared return type — each mismatch is rejected under `E0501`, and no coercion is ever inserted. Every cross-type transition is an explicit method (`toInt64()`, `toBytes()`, and their siblings; the method inventory is the standard library's). This obligation binds later type chapters equally: a future type with an implicit conversion to another type is a spec-layer exception requiring its own change.

#### Scenario: Mixed integer widths are rejected

- **WHEN** `Int64` and `Int32` operands combine, for example `a + b` with `a: Int64` and `b: Int32`
- **THEN** the compiler rejects it with `E0501:` operands of different types; the fix is an explicit conversion method

#### Scenario: Signed and unsigned never mix implicitly

- **WHEN** `Int64` and `UInt64` operands combine
- **THEN** the compiler rejects it with `E0501:` operands of different types; sign conversion is explicit

#### Scenario: String, Bytes, and Rune stay separate

- **WHEN** operands of two of `String`, `Bytes`, and `Rune` combine, for example comparing a `String` with a `Rune`
- **THEN** the compiler rejects it with `E0501:` operands of different types; the transitions `toBytes()`, `toString()`, and their siblings are explicit

#### Scenario: An annotation of a different type is rejected

- **WHEN** a binding annotates `Int32` while its expression is the unsuffixed literal `42`, hence `Int64` — `let n: Int32 = 42`
- **THEN** the compiler rejects it under `E0501` with no coercion; the fix is the suffix (`42i32`) or an explicit conversion method

#### Scenario: Arguments and returns match their declared types

- **WHEN** a call argument's type differs from its parameter's annotation, or a return expression's type differs from the declared return type
- **THEN** the compiler rejects the mismatch under `E0501` at that position; no coercion is inserted

### Requirement: Integer overflow semantics

Integer arithmetic is checked; it MUST NOT silently wrap. A compile-time-detectable overflow — one visible to constant evaluation of literal expressions — is a compile error (`E0502`). A runtime overflow is a checked trap: the operation never produces a wrapped value, and the trapping construct's name and capture boundary are ratified by the error-mechanism chapter. Wrapping semantics are available only through explicit methods (`wrappingAdd()` and siblings; the inventory is the standard library's). Float arithmetic follows IEEE 754 and has no overflow check.

#### Scenario: Constant overflow is a compile error

- **WHEN** a literal expression overflows its type at compile time, for example `9223372036854775807 + 1` as `Int64`
- **THEN** the compiler rejects it with `E0502:` integer overflow at the expression

#### Scenario: Runtime overflow traps, never wraps

- **WHEN** an integer operation overflows at run time, for example adding two `Int64` values whose sum exceeds the maximum
- **THEN** the operation does not produce a wrapped value; it traps as a checked failure whose construct the error-mechanism chapter names

#### Scenario: Wrapping is explicit

- **WHEN** wrapping semantics are wanted, for example in a hash computation
- **THEN** the author calls a wrapping method explicitly; the ordinary operators keep checked semantics

### Requirement: Type references

A type reference — the form filling every type-annotation slot ratified by chapters 2 and 6 — is a named type: a PascalCase identifier, optionally module-qualified as `module.Name` reaching an imported module's public type per chapter 6. Tuple types, function types, generic applications, and every other type syntax do not exist yet; each arrives with its owning chapter through this chapter's amendment. A type slot holding anything but a named reference MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). Whether a well-formed name resolves to a ratified type is decided with module resolution by the module-system chapter.

#### Scenario: A qualified type reference

- **WHEN** an annotation holds `user.User` where `user` names an imported module
- **THEN** it is a type reference to that module's public `User` type

#### Scenario: Unratified type syntax is rejected

- **WHEN** a type slot holds tuple or function-type syntax, for example `let pair: (Int64, Int64) = ...`
- **THEN** the compiler rejects it with `E0105:` unexpected token at the slot; those forms arrive with their owning chapters

### Requirement: Condition positions

The operands of condition positions are `Bool`. An `if` condition, a `while` condition, and a match guard MUST be `Bool`; anything else is rejected with `E0503`. This grounds chapter 3's condition typing and chapter 4's guard typing.

#### Scenario: A non-Bool condition is rejected

- **WHEN** an `if` or `while` condition is not `Bool`, for example `if count` with `count: Int64`
- **THEN** the compiler rejects it with `E0503:` condition is not Bool

#### Scenario: A non-Bool match guard is rejected

- **WHEN** a match guard is not `Bool`, for example `n if n` with `n: Int64`
- **THEN** the compiler rejects it with `E0503:` condition is not Bool

### Requirement: Block value typing

A block with a value has the type of its final expression item; a block without a value — final item a statement, or empty — has no type. This grounds chapter 2's deferred block-value typing; how a valueless block behaves where a type is required is ratified with the unit type by the composite-types chapter.

#### Scenario: A block's type is its final expression's type

- **WHEN** a block ends in `a + b` with `a`, `b`: `Int64`
- **THEN** the block's type is `Int64` per chapter 2's Blocks and block value

#### Scenario: A valueless block has no type

- **WHEN** a block's final item is a statement or the block is empty
- **THEN** the block has no value and no type; its use where a type is required follows the composite-types chapter
