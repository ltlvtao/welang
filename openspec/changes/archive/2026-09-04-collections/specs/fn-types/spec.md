## MODIFIED Requirements

### Requirement: Function types diagnostics segment

The fn-types chapter owns registry segment `E1000`–`E1099` declared in `docs/spec/diagnostics.toml` `[segments]`. Allocations: `E1001` bare-parameter closure without an expected function type, `E1002` resource binding captured by a closure, `E1003` assignment to a captured value-category binding, `E1004` generic function name in value position. `E1000` and `E1005`–`E1099` remain reserved for amendments of this chapter. Trigger semantics live in this chapter's Requirements; entries live in the registry.

#### Scenario: A fn-types code is emitted

- **WHEN** any `E10xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `1200-fn-types`

#### Scenario: A later change needs this segment's codes

- **WHEN** a later chapter ratifies rules requiring new function-value diagnostics
- **THEN** its change extends the registry within `E1000`–`E1099` in the same change, or claims its own segment — the combinator change took the second route, its purity rule riding chapter 16's `E1402`
