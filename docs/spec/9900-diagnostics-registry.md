# We Language Specification — Chapter 99: Diagnostics Registry

### Requirement: Registry authority and role split

`docs/spec/diagnostics.toml` is the single registry of the diagnostic code system. Chapters define **when** codes fire — trigger authority lives in Requirements and Scenarios; the registry defines the **entry** of every allocated code — entry authority: identity, severity, title, description, remediation, and owning chapter and Requirement. One code has exactly one meaning, one entry, and one owner. Chapter prose may reference codes and segment ranges for readability but MUST NOT redefine an entry or its fields.

#### Scenario: Chapter prose duplicates an entry

- **WHEN** a chapter delta describes a code's remediation or entry fields in prose instead of extending the registry
- **THEN** change review rejects it; the registry is the only place an entry is defined

#### Scenario: A consumer looks up a code

- **WHEN** any consumer — compiler, tool, model, or human — holds a diagnostic code emitted by the toolchain
- **THEN** the complete entry is retrievable from `docs/spec/diagnostics.toml` alone: severity, title, full description, remediation, and the owning chapter Requirement for trigger semantics

### Requirement: Entry schema

Every allocated code has exactly one `[diagnostic.<CODE>]` entry providing: `severity` (`error` or `warning`), `title` (stable short message, English), `description` (complete statement of what is rejected and why, pointing to the owning Requirement for the trigger), `remediation` (machine-operable fix guidance, chapter 0 Principle 7), `owner` (owning chapter file slug), `requirement` (Requirement title in the owner chapter), and `allocated` (ISO date of the ratifying change). Entries are written in English; localization of compiler output is a toolchain concern outside the registry. The registry format is TOML and MUST remain parseable by standard TOML readers.

#### Scenario: An entry lacks remediation

- **WHEN** a proposed registry entry omits `remediation` or carries remediation that is not actionable guidance
- **THEN** validation rejects the entry (Principle 7: machine-operable remediation is mandatory)

#### Scenario: Entry references resolve

- **WHEN** the registry is validated
- **THEN** every entry's `owner` names an existing chapter file under `docs/spec/` and its `requirement` names a Requirement heading in that file

### Requirement: Extension and stability

A new diagnostic code or segment enters only through a spec-layer change that extends `docs/spec/diagnostics.toml` in the same change. `E` and `W` share one number space: a given four-digit number exists with at most one severity at a time. Renumbering, deleting, or changing the meaning of an allocated code is forbidden (chapter 0: the diagnostics protocol is a stability commitment). Adding fields to the entry schema is backward-compatible; removing or renaming fields is not.

#### Scenario: A chapter claims codes

- **WHEN** a content chapter ratifies new diagnostics
- **THEN** its change allocates an unclaimed range in the registry's segment table and adds one entry per allocated code; renumbering existing codes fails review

#### Scenario: An E/W number collision appears

- **WHEN** a change would allocate `W0500` while `E0500` already exists
- **THEN** validation rejects the allocation; the number space is shared between severities

### Requirement: Segments

Segments are declared in the registry's `[segments]` table, each with a domain and an owner. The initial segments are: `E0001`–`E0099` lexical and naming (chapter 1), `E0100`–`E0199` grammar and parsing (grammar chapter), and `E0200` upward claimed by later chapters through amendment. Every registry entry MUST lie within a declared segment owned by its `owner` chapter; reserved ranges are declared as segment data, not enumerated as entries.

#### Scenario: An entry lies outside its owner's segment

- **WHEN** the registry contains an entry whose number lies in no segment owned by its `owner` chapter
- **THEN** validation rejects the entry

#### Scenario: A reserved code is allocated without amendment

- **WHEN** a change uses a number inside a declared reserved range without claiming it through the registry
- **THEN** validation and review reject the change; reserved numbers are not implicitly available

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| registry | 注册表 |
| diagnostic entry | 诊断条目 |
| remediation | 修复建议 |
| number space | 号码空间 |
| trigger authority | 触发权威 |
| entry authority | 条目权威 |
