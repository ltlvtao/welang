# Delta: types (host amendment for sum types)

## MODIFIED Requirements

### Requirement: Base type inventory

The base types are a closed set of thirteen: the checked integers `Int8` `Int16` `Int32` `Int64` `UInt8` `UInt16` `UInt32` `UInt64`, the IEEE 754 floats `Float32` `Float64`, `Bool`, `String` (an immutable UTF-8 byte sequence; logical iteration yields `Rune`, whose protocol is ratified by the iterable-protocols chapter), `Bytes` (a byte sequence), and `Rune` (a single Unicode code point, U+0000..U+10FFFF). `Never` and `()` are not members of this inventory: the bottom type is ratified by the sum-types chapter (chapter 9) and the unit type by the composite-types chapter (chapter 8); each is a type of its own kind, not a base type. Adding a base type is a spec-layer change amending this inventory.

#### Scenario: A base type name denotes its type

- **WHEN** a type reference names one of the thirteen base types, for example `Int64` or `String`
- **THEN** it denotes that base type; the names are PascalCase per chapter 1's naming conventions

#### Scenario: The inventory is closed

- **WHEN** a later chapter needs a new base type
- **THEN** it enters only through a spec-layer change amending this Requirement's inventory; no other route exists
