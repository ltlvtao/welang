## MODIFIED Requirements

### Requirement: The project manifest

The project manifest is a TOML file named `we.toml` at the project root — the directory holding it is the project root, chapter 15's fact restated. The skeleton this chapter fixes: `name`, a string under chapter 1's naming convention — an illegal value MUST be rejected with `E1904:` invalid project name; `version`, a semantic version under chapter 22's shape — an illegal value MUST be rejected with `E2004:` invalid version value; `type`, one of `executable` or `library` — any other value is `E1903:` invalid toolchain configuration value; the `[vet]` table of the advisory layer's requirement; and the `[test]` table, whose key `explore-iterations` is a positive integer — a non-positive or non-integer value is `E1903`. A project invocation — a directory-path command — with no manifest, or a manifest missing `name`, `version`, or `type`, MUST be rejected with `E1905:` project manifest missing or incomplete. The `[dependencies]` table and the lockfile are chapter 22's: the table is optional — absent it is an empty dependency set — its keys and values are checked under that chapter, and `we.lock` is machine-written by its resolution.

#### Scenario: The skeleton is checked

- **WHEN** `we build` runs in a directory whose `we.toml` lacks `type`
- **THEN** the toolchain reports `E1905:` project manifest missing or incomplete, naming the missing key

#### Scenario: Illegal values are named

- **WHEN** a manifest writes `type = "bin"` or `explore-iterations = 0`
- **THEN** both are rejected with `E1903:` invalid toolchain configuration value, the message naming the key, the legal values, and what was found

#### Scenario: we new writes the skeleton

- **WHEN** `we new demo` runs
- **THEN** the created `we.toml` carries `name = "demo"`, a `version`, `type = "executable"`, and no tables the toolchain does not define; `src/main.we` holds a `pub fn main` and `tests/` holds one empty test module

#### Scenario: Dependencies are chapter 22's

- **WHEN** a manifest carries a `[dependencies]` table
- **THEN** its keys and values are checked under chapter 22 — an illegal name or the reserved `std` is `E2006`, a malformed constraint is `E2003` — and resolution and acquisition run under that chapter's requirements before any pipeline the command runs

### Requirement: What the toolchain does not fix

This chapter names what it leaves open. The Language Server Protocol lives in a separate document of its own, as v0.8 already intended — the one promise made here is consistency: an editor service's diagnostics are `we check`'s, the same pipeline reporting the same codes, and no editor surface may diverge from the command line's verdicts. Localization of diagnostic output is the toolchain's — the registry's entries are English and the translation layer is outside the specification. Incremental compilation, caching, and build parallelism are implementation details behind the one promise that the same inputs produce the same outputs. Cross-file test parallelism and reporting layout are likewise the implementation's. No performance budget — build time, latency, footprint — is promised anywhere in this chapter.

#### Scenario: Editor diagnostics are check's diagnostics

- **WHEN** an editor service reports a diagnostic for a file
- **THEN** the codes, messages, and verdicts are those `we check` reports for the same source — one pipeline, one truth, whichever face it shows

#### Scenario: No performance promise exists

- **WHEN** a toolchain is measured against this chapter for build speed or latency budgets
- **THEN** nothing answers — the chapter promises output determinism and observable contracts, and performance is evaluation's to measure, not specification's to promise
