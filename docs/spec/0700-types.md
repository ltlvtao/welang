# We Language Specification — Chapter 7: Types

### Requirement: Base type inventory

The base types are a closed set of thirteen: the checked integers `Int8` `Int16` `Int32` `Int64` `UInt8` `UInt16` `UInt32` `UInt64`, the IEEE 754 floats `Float32` `Float64`, `Bool`, `String` (an immutable UTF-8 byte sequence; logical iteration yields `Rune`, whose protocol is ratified by the iterable-protocols chapter), `Bytes` (a byte sequence), and `Rune` (a single Unicode code point, U+0000..U+10FFFF). `Never` and `()` are not members of this inventory: the bottom type is ratified by the sum-types chapter (chapter 9) and the unit type by the composite-types chapter (chapter 8); each is a type of its own kind, not a base type. Adding a base type is a spec-layer change amending this inventory.

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

Integer arithmetic is checked; it MUST NOT silently wrap. A compile-time-detectable overflow — one visible to constant evaluation of literal expressions — is a compile error (`E0502`). A runtime overflow is a checked trap: the operation never produces a wrapped value; the trap is chapter 14's `panic` naming the operation, and its capture boundary is the process abort — no handler observes it. Wrapping semantics are available only through explicit methods (`wrappingAdd()` and siblings; the inventory is the standard library's). Float arithmetic follows IEEE 754 and has no overflow check.

#### Scenario: Constant overflow is a compile error

- **WHEN** a literal expression overflows its type at compile time, for example `9223372036854775807 + 1` as `Int64`
- **THEN** the compiler rejects it with `E0502:` integer overflow at the expression

#### Scenario: Runtime overflow traps, never wraps

- **WHEN** an integer operation overflows at run time, for example adding two `Int64` values whose sum exceeds the maximum
- **THEN** the operation does not produce a wrapped value; it traps as a `panic` naming the operation (chapter 14's construct), and the process aborts — no handler observes it

#### Scenario: Wrapping is explicit

- **WHEN** wrapping semantics are wanted, for example in a hash computation
- **THEN** the author calls a wrapping method explicitly; the ordinary operators keep checked semantics

### Requirement: Type references

A type reference — the form filling every type-annotation slot ratified by chapters 2 and 6 — is a named type or a structural composite: a named type is a PascalCase identifier, optionally module-qualified as `module.Name` reaching an imported module's public type per chapter 6, and optionally a generic application `Name<T1, ..., Tk>` per chapter 10, the arguments type references and the arity the declaration's own clause's; a tuple type is `(T1, T2, ..., Tn)` with n from 2 to 8, each element itself a type reference; the unit type is `()`; `Dyn<Interface>` per chapter 10 names a type-erased interface box; and a function type `fn(T1, ..., Tn) -> T` per the fn-types chapter is a type-reference form — zero or more comma-separated parameter types, each a type reference, an optional effect segment of bare space-separated tags between the parameter types and the arrow per chapter 16, an arrow, and a required return type, itself a type reference with the unit type `()` in the valueless position; its grammar and agreement are that chapter's. Every other type syntax does not exist yet; it arrives with its owning chapter through this chapter's amendment. A type slot holding anything but a named reference, a generic application, a `Dyn<Interface>` form, a tuple type, the unit type, or a function type MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`). Whether a well-formed name resolves to a ratified type is decided by chapter 15's name resolution — bare names through scopes innermost-first to the prelude, qualified names through the import.

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

- **WHEN** an annotation holds `fn(Int64) -> Int64`, or the effect-carrying `fn(Int64) io -> Int64` with its bare-tag segment per chapter 16
- **THEN** it is the function-type form per the fn-types chapter; the slot holds that signature's functions, the effect segment stating which effects they may perform

#### Scenario: Unratified type syntax is rejected

- **WHEN** a type slot holds a bracketed type such as `[Int64]`, or any other unratified type syntax
- **THEN** the compiler rejects it with `E0105:` unexpected token at the slot; the form arrives, if ever, with its owning chapter

### Requirement: Condition positions

The operands of condition positions are `Bool`. An `if` condition, a `while` condition, and a match guard MUST be `Bool`; anything else is rejected with `E0503`. This grounds chapter 3's condition typing and chapter 4's guard typing.

#### Scenario: A non-Bool condition is rejected

- **WHEN** an `if` or `while` condition is not `Bool`, for example `if count` with `count: Int64`
- **THEN** the compiler rejects it with `E0503:` condition is not Bool

#### Scenario: A non-Bool match guard is rejected

- **WHEN** a match guard is not `Bool`, for example `n if n` with `n: Int64`
- **THEN** the compiler rejects it with `E0503:` condition is not Bool

### Requirement: Block value typing

A block with a value has the type of its final expression item; a block without a value — final item a statement, or empty — has the unit type `()`, which chapter 8 ratifies and frees from the discard obligation. This grounds chapter 2's deferred block-value typing.

#### Scenario: A block's type is its final expression's type

- **WHEN** a block ends in `a + b` with `a`, `b`: `Int64`
- **THEN** the block's type is `Int64` per chapter 2's Blocks and block value

#### Scenario: A valueless block has the unit type

- **WHEN** a block's final item is a statement or the block is empty
- **THEN** the block has no value and its type is `()`; it may stand where the unit type is required, per chapter 8

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–7 and the iterable protocols they rest on. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. The conversion and wrapping method inventory is annotated as pending its owning change; string iteration is chapter 11's, the collection types chapter 17's.

### Literals and defaults

```we
let n = 42            // Int64: unsuffixed integer default
let x = 3.14          // Float64: unsuffixed float default
let small = 100i32    // Int32: suffix names the type
let mask = 0xFFu8     // UInt8: base prefix and suffix combine
let flag = true       // Bool
let name = "Ada"      // String
let first = 'A'       // Rune

let typed: Float32 = 2.5f32   // annotation and literal agree
```

### No implicit conversion

```we
let a: Int64 = 1
let b: Int32 = 2
let sum = a + b               // E0501: operands of different types

let count: Int64 = 5
let size: UInt64 = 5
let bad = count + size        // E0501: signed and unsigned never mix

let s = "abc"
let c = 'a'
let cmp = s == c              // E0501: String and Rune stay separate

let n: Int32 = 42             // E0501: the literal is Int64, the
                              // annotation is Int32 — no coercion;
                              // write 42i32 or convert explicitly

// every transition is an explicit method (stdlib names, shown
// for shape only):
let wide = small.toInt64()
let bytes = s.toBytes()
```

### Overflow never wraps silently

```we
let max = 9223372036854775807
let boom = max + 1            // E0502: integer overflow (constant-folded)

let hi: Int64 = 4611686018427387904
let lo: Int64 = 4611686018427387904
let pair = hi + lo            // runtime checked trap, never a wrapped
                              // value; the trap is chapter 14's panic
                              // naming the operation

let hashed = hash()
let mixed = hashed.wrappingAdd(1)   // wrapping is always explicit
```

### Type references fill the annotation slots

```we
fn parse(text: String) -> Int64 {   // named base types
    transform(text)
}

let u: user.User = user.find(1)     // module-qualified reference per
                                    // chapter 6's import names
```

### Conditions are Bool

```we
let count = 3
if count { step() }          // E0503: condition is not Bool
while ready() { poll() }     // legal: the call is Bool

match next() {
    n if n { }               // E0503: guard must be Bool
    _ { }
}
```

### String iterates by code point

```we
for c in name { step(c) }      // chapter 11: builtin Iterable<Rune>;
                                // c binds each code point in order
```

### Landed later chapters

```we
// Fn types landed with the function-types chapter; the collection
// types (List among them) landed with the collections chapter — the
// generic application form they use is chapter 10's, the names are
// prelude-visible:
//
// let f: fn(Int64) -> Int64 = square
// let ids: List<UserId> = build()
// fn forEach(items: List<Int64>) { }
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| base type | 基础类型 |
| literal typing | 字面量类型化 |
| suffix | 后缀 |
| implicit conversion | 隐式转换 |
| explicit conversion | 显式转换 |
| type-agreement position | 类型一致位置 |
| checked arithmetic | 检查算术 |
| overflow | 溢出 |
| wrapping | 回绕 |
| type reference | 类型引用 |
| module-qualified name | 模块限定名 |
| annotation slot | 注解槽 |
| condition position | 条件位置 |
| block value | 块值 |
| valueless block | 无值块 |
