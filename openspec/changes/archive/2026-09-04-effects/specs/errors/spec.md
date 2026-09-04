## MODIFIED Requirements

### Requirement: The single error mechanism

Errors flow through exactly one mechanism: `Result` values declared in signatures, propagated by `?`, consumed by `match` — chapter 0, Principle 9 operationalized. This chapter fixes the consequences. A function that can fail says so in its return type: there is no throws clause, no effect-segment escape — chapter 16's tags track environmental effects, not failure — and no unchecked channel. `?` is the only propagation operator: no other postfix or prefix form propagates — `expr!` and `try expr` fit no production of chapter 2's grammar. A failure crossing a function boundary is a value in a signature, so every path of failure is visible at the call site and the discipline is locally decidable (chapter 0, Principle 1). A panic is termination, not an error channel: it carries no payload type, matches nothing, and is caught by nothing.

#### Scenario: No other propagation spelling exists

- **WHEN** `expr!` or `try f()` appears at an expression position
- **THEN** the compiler rejects it with chapter 2's unexpected-token diagnostic (`E0105`); the propagation postfix is `?` alone

#### Scenario: Failure is visible in the signature

- **WHEN** a caller sees `fn parse(s: String) -> Result<Config, ParseError>`
- **THEN** every failure path is declared in the signature; nothing propagates implicitly, and the caller's handling is checkable at the call site alone
