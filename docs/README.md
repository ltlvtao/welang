# welang Documentation Index

## Documentation Principles

welang documentation follows "one authoritative location per class of fact". The language spec defines language behavior, ADRs record long-term tradeoffs, the roadmap manages execution state, and the process document governs how work happens. Nothing is duplicated across these locations.

Documents under `docs/` ship with dual-language synchronized support: English text plus a paired Chinese translation (`<basename>.zh.md`) — see [Documentation Language](#documentation-language). Agent-facing operational files outside `docs/` (`AGENTS.md`, `openspec/`, `.agents/skills/`) are written in Chinese; conversation language is Chinese.

## Documentation Language

Documentation under `docs/` ships with dual-language synchronized support:

- Every `*.md` document has a paired Chinese translation in the same directory, named `<basename>.zh.md`. The translation carries every naming element of the source file, including numeric prefixes and ADR numbers.
- English is the authoritative text. Where the English and Chinese renderings of a document disagree, English governs; the translation is corrected in the change that introduced the divergence.
- Bilingual sync is part of a change's definition of done: a change that creates or modifies a document under `docs/` does not reach `complete` until its translation reflects the same change and `openspec/tools/docs_sync.py` passes.
- `docs_sync.py` checks structure only (pair existence, heading counts per level, fenced code block counts). Semantic equivalence of translations is carried by change review, not by script.

Files outside `docs/` are single-language by design: `AGENTS.md`, `openspec/`, and `.agents/` are Chinese; code comments and commit messages are English.

## Document Naming Conventions

Numeric prefixes encode reading order where order matters; semantic names carry identity where cross-referencing matters:

- **Positional numbering for design corpora.** Canonical spec chapters under `docs/spec/` are `<nnnn>-<slug>.md` — four-digit zero-padded, stride 100 (`0000`, `0100`, `0200`, ...), so a new chapter inserts between neighbors without renumbering. `0000` is reserved for chapter 0 (foundations); cross-cutting registry chapters count from the tail (`9900` is the diagnostics registry chapter), so content chapters growing upward and registry chapters anchored at the end never collide. ADRs use the same stride: `ADR-<nnnn>-<slug>.md`. Translations inherit every naming element (see Documentation Language).
- **Date prefixes for append-only ledgers.** A change directory is renamed to `YYYY-MM-DD-<name>` when moved into `openspec/changes/archive/` — the date is the total order, needs no concurrency coordination, and `validate.py` does not check `archive/`.
- **Semantic kebab names for identity.** Active change directories and `specs/<capability>/` capability names keep kebab-case semantic names — a change name is a join key referenced across artifacts, and the capability-to-chapter mapping is decided at promotion, not fixed in advance.
- **Fixed structural names are exempt.** `README.md`, `AGENTS.md`, `SKILL.md`, `change.yaml`, and the four change-artifact filenames never carry prefixes — tooling and cross-tool conventions depend on these names. The same exemption covers machine artifacts: `docs/spec/diagnostics.toml` (the diagnostic code registry) is a fixed structural name — English only, no paired translation, parsed directly by compilers, tools, and `validate.py`.

## Directory Responsibilities

| Directory or file | Authoritative responsibility | Does NOT carry |
| --- | --- | --- |
| [`AGENTS.md`](../AGENTS.md) | Agent entry point (cross-tool standard): hard rules, language invariants, workflow summary, verification ladder, commit protocol, environment setup | Language requirements, roadmap state, process details |
| [`docs/process/`](process/development-process.md) | The R&D process (single authority for how work happens) | Language behavior, change-specific decisions |
| `docs/spec/` | Canonical language specification, versioned, plus the machine-readable diagnostic code registry `diagnostics.toml` (single entry authority for every allocated code) | Implementation decisions, migration history, execution state |
| [`docs/lsp.md`](lsp.md) | The LSP binding document (chapter 21 R12's separate document): protocol baseline, transport, lifecycle, diagnostic mapping | Editor capabilities beyond diagnostics, any new diagnostic codes |
| [`docs/benchmarks.md`](benchmarks.md) | The evaluation methodology document (M15): three metrics, task taxonomy, trap lists, batch schema, feedback protocol, runner verdict order, statistical discipline | Model calls and real-batch production, statistical conclusions from the seed set |
| `docs/decisions/` | Long-term ADRs: status, dependencies, rejected options (created when the first ADR is needed) | Requirements, task lists |
| `docs/roadmap/` | Strategy, milestones, change catalog, execution state (created when the roadmap outgrows `openspec/changes/`) | Requirements, field-level design |
| [`openspec/`](../openspec/README.md) | Change management: active/archived changes with proposal/spec-delta/design/tasks artifacts; repo artifact checkers (`validate.py` for change structure, `docs_sync.py` for bilingual pairing) | Long-term authority of any fact (changes are process increments) |
| [`refr/`](../.gitignore) | Private reference material (spec v0.8 draft + review). Never committed — top-priority rule, hook-enforced | Any authority; nothing in `refr/` defines language behavior |
| [`.agents/`](../.agents/) | Project R&D skills (lifecycle review gates), tool-agnostic | Process authority (that lives in `docs/process/`), product-runtime assets |
| [`.githooks/`](../.githooks/) | Versioned git hooks (mechanical enforcement: `refr/` ban, attribution ban), activated via `core.hooksPath` | Any policy that cannot be enforced at git level |

## Reading Paths

### Every change

1. Read [`AGENTS.md`](../AGENTS.md).
2. Read [the development process](process/development-process.md).
3. Load the minimal document set relevant to the target of the change, per the table above.

### Language behavior change

1. [Development process](process/development-process.md)
2. Current canonical spec under `docs/spec/` (latest version)
3. The relevant `openspec/changes/<name>/` artifacts

### Tooling / verification

1. [Development process](process/development-process.md), verification ladder section
2. Affected package or module documentation

## Update Rules

- Process changes go through an openspec change touching the `process` layer, then are promoted into `docs/process/`.
- Language behavior changes go through an openspec change with a spec delta; on archive, the delta is promoted into `docs/spec/`.
- Long-term tradeoffs are promoted into `docs/decisions/` as ADRs on archive.
- Do not create empty directories or placeholder documents; create a location when real content first requires it.
