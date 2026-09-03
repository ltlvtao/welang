# lexical Specification Delta

## MODIFIED Requirements

### Requirement: Diagnostic code scheme

Diagnostic codes are `E` (error) or `W` (warning) followed by exactly four digits; `E` and `W` share one number space, so a four-digit number exists with at most one severity at a time. The single registry of segments and allocated codes is `docs/spec/diagnostics.toml`: every allocated code has exactly one entry there (severity, title, description, remediation, owner, owning Requirement), and chapters define when codes fire through their Requirements and Scenarios. The lexical segment `E0001`–`E0099` is owned by this chapter; within it, `E0001`–`E0009` are token-level errors, `E0011`–`E0019` the naming block (`E0011`–`E0013` allocated), and `E0010` together with `E0014`–`E0099` are reserved for amendments of this chapter. Warning codes follow the same scheme; none are allocated by this chapter.

#### Scenario: A later chapter claims a segment

- **WHEN** a content chapter needs new diagnostic codes
- **THEN** its change extends `docs/spec/diagnostics.toml` in the same change — a claimed range in the segment table plus one entry per allocated code — and renumbering existing codes is forbidden (chapter 0: the diagnostics protocol is a stability commitment)

#### Scenario: Codes are unique across chapters

- **WHEN** two chapters would define the same code with different meanings
- **THEN** the registry rejects the second definition: a code key is globally unique, carrying exactly one severity, one title, and one owner; validation enforces it mechanically
