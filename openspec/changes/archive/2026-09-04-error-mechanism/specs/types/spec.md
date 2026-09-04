## MODIFIED Requirements

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
