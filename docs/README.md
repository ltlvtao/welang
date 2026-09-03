# welang Documentation Index

## Documentation Principles

welang documentation follows "one authoritative location per class of fact". The language spec defines language behavior, ADRs record long-term tradeoffs, the roadmap manages execution state, and the process document governs how work happens. Nothing is duplicated across these locations.

All files under `docs/` are written in English (project convention). Agent-facing operational files outside `docs/` (`AGENTS.md`, `openspec/`, `.agents/skills/`) are written in Chinese; conversation language is Chinese.

## Directory Responsibilities

| Directory or file | Authoritative responsibility | Does NOT carry |
| --- | --- | --- |
| [`AGENTS.md`](../AGENTS.md) | Agent entry point (cross-tool standard): hard rules, language invariants, workflow summary, verification ladder, commit protocol, environment setup | Language requirements, roadmap state, process details |
| [`docs/process/`](process/development-process.md) | The R&D process (single authority for how work happens) | Language behavior, change-specific decisions |
| `docs/spec/` | Canonical language specification, versioned (created when the first spec change is archived) | Implementation decisions, migration history, execution state |
| `docs/decisions/` | Long-term ADRs: status, dependencies, rejected options (created when the first ADR is needed) | Requirements, task lists |
| `docs/roadmap/` | Strategy, milestones, change catalog, execution state (created when the roadmap outgrows `openspec/changes/`) | Requirements, field-level design |
| [`openspec/`](../openspec/README.md) | Change management: active/archived changes with proposal/spec-delta/design/tasks artifacts | Long-term authority of any fact (changes are process increments) |
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
