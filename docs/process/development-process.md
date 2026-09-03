# welang Development Process

This document is the single authority for how work happens in the welang repository. It is adapted from the AI-assisted development process used by the openTalon project (spec-driven changes, skill-gated reviews, mechanical validation), reshaped for a spec-first language project.

## 1. Purpose and Principles

welang develops an AI-native programming language. The product chain is: **canonical language spec → reference compiler → stdlib → tooling → evaluation benchmarks**. Quality rests on the following principles, in priority order:

1. **Spec first.** The canonical spec under `docs/spec/` is the only authority on language behavior. No compiler or tooling code that changes language behavior may be written without a corresponding spec delta in an active change.
2. **One authoritative location per class of fact.** Language behavior lives only in the spec; tradeoffs live only in ADRs; execution state lives only in the roadmap; process rules live only in this document. Changes are process increments, never long-term authority.
3. **Verify before checking off.** Every task in a change must carry a verification (exact command + expected result). A task may not be marked done unless its verification has actually been run and passed.
4. **Test first for compiler behavior.** Behavior-changing compiler work starts with a failing target test (including golden diagnostic cases), then the implementation.
5. **Small, vertically verifiable changes.** Never create a change named "implement the whole compiler" or "merge the whole review". Each change must be independently acceptable.
6. **Enforcement is mechanical where possible.** Rules that can be checked by script or hook are checked by script or hook; model discipline supplements, never replaces, mechanical gates.

## 2. Change Lifecycle

```
candidate ──(spec-impact audit)──▶ ready ──(four artifacts + change review)──▶ active
    ──(implementation + verification + code review)──▶ complete ──(archive sync)──▶ archived
```

| State | Meaning | Entry condition | Exit condition |
| --- | --- | --- | --- |
| `candidate` | An idea with a problem statement, not yet committed to | `proposal.md` draft + `change.yaml` exists | spec-impact audit passed (skill: `welang-spec-impact-audit`) |
| `ready` | Scope, impact layers, and acceptance boundary are fixed | audit passed | four artifacts complete and semantically reviewed (skill: `welang-change-review`) |
| `active` | Being implemented | change review passed | all tasks verified, code review passed (skill: `welang-code-review`) |
| `complete` | Implemented and verified, awaiting baseline promotion | code review passed | baseline promotion done (skill: `welang-archive-sync`) |
| `archived` | Done; long-term facts promoted; directory moved to `openspec/changes/archive/` | promotion done | — |

State transitions are recorded in the change's `change.yaml` (`status` field). The validator enforces artifact completeness per state.

## 3. Change Artifacts

Every change lives in `openspec/changes/<kebab-name>/` and consists of:

| Artifact | Responsibility | Does NOT contain |
| --- | --- | --- |
| `change.yaml` | Name, status, impact layers | Anything else |
| `proposal.md` | Why the change is needed; goals and non-goals; what changes; impact layers and scope | Requirements, design decisions, task lists |
| `specs/<capability>/spec.md` | Black-box spec deltas: Requirements with Scenarios (WHEN/THEN) | Implementation, owners, code paths, migration history |
| `design.md` | White-box decisions: chosen approach, rejected alternatives, invariants affected, verification strategy | Requirements, task lists |
| `tasks.md` | Verifiable units of work; every checkbox carries Source + Verification | Undefined work, deferred items, open choices |

### 3.1 `change.yaml`

```yaml
name: <kebab-name>        # must match the directory name
status: candidate         # candidate | ready | active | complete | archived
layers: [spec]            # subset of: spec | compiler | stdlib | tooling | benchmark | process | docs
```

### 3.2 `proposal.md` (required headers)

- `## Why` — the problem, in black-box terms
- `## 目标与非目标` — goals and explicit non-goals
- `## What Changes` — the delta summary
- `## 影响层` — impact layers with per-layer black-box change summary
- `## 影响范围` — what else is affected or explicitly not affected

### 3.3 Spec deltas

Deltas use section headers `## ADDED Requirements` / `## MODIFIED Requirements` / `## REMOVED Requirements` / `## RENAMED Requirements`. Each `### Requirement:` must have at least one `#### Scenario:` containing `**WHEN**` and `**THEN**`. Mandatory obligations use MUST / MUST NOT; deviations use SHOULD with stated conditions; options use MAY with stated defaults.

A change that alters language behavior MUST include a spec delta. A change that only affects internals (refactor, test infrastructure) MAY omit `specs/` but MUST say so in `## 影响层` with layer `compiler` (or similar) and a justification.

### 3.4 `tasks.md`

Every checkbox line (`- [ ]` / `- [x]`) must be followed by two lines:

```
来源：<proposal / requirement+scenario / design section>
验证：<exact command + expected result>
```

Rules:

- A checked (`- [x]`) task with an empty or missing verification line is a validation failure.
- Verification that has not actually been run may not be checked off — this is agent discipline, reviewed by `welang-code-review`.
- Prohibited paths (things the change must NOT do) require real negative assertions, not prose.
- Unresolved questions that affect behavior, contracts, or acceptance MUST block the related tasks; they may not be hidden with SHOULD/MAY/TODO.

## 4. Verification Ladder

Before commit or push, select the minimal credible set for the actual diff (skill: `welang-pre-push-checks`):

| Diff scope | Required verification |
| --- | --- |
| Docs / process / openspec artifacts only | `python3 openspec/tools/validate.py --all --strict`; `python3 openspec/tools/docs_sync.py`; `git diff --check`; staged files contain no `refr/` |
| Canonical spec (`docs/spec/`) | Spec consistency review: diagnostic codes globally unique, `§` cross-references valid, terminology consistent; plus the row above |
| Compiler / tooling code | `go build ./...`; `go test ./...`; affected conformance golden cases |
| Diagnostics protocol | Protocol snapshot tests (JSON Lines field stability) |

Verification commands are fixed from day one and may only change through a process-layer change.

### 4.1 Documentation language and bilingual sync

Documentation under `docs/` ships with dual-language synchronized support: English is authoritative, and every document carries a paired Chinese translation `<basename>.zh.md` in the same directory. A change that creates or modifies a document under `docs/` updates its translation within the same change; `openspec/tools/docs_sync.py` (pairing, heading counts per level, fenced code block counts) must pass before the change reaches `complete`. The convention's authoritative statement lives in `docs/README.md`, section "Documentation Language".

## 5. Commit Protocol

- Commit messages are in English and explain **why**, not list files.
- Attribution trailers (`Co-Authored-By`, `Generated with ...`, or similar) are forbidden — enforced by the `commit-msg` git hook.
- Decision-context trailers are allowed and encouraged: `Constraint:`, `Rejected:`, `Tested:`, `Not-tested:`.
- One change maps to one commit or a small set of vertically verifiable commits.

## 6. Enforcement Model

Four layers, strongest first:

1. **Git hooks (mechanical, tool-agnostic).** Versioned hooks under `.githooks/`, activated once per clone via `git config core.hooksPath .githooks`. `pre-commit` blocks any commit with `refr/` staged; `commit-msg` rejects messages containing `Co-Authored-By` or `Generated with`. These fire regardless of which tool (or human) runs git. Bypassing with `--no-verify` is forbidden.
2. **Validator (mechanical).** `openspec/tools/validate.py --all --strict` enforces artifact completeness per state, required headers, scenario structure, and task verification format.
3. **Skills (semantic gates).** `welang-spec-impact-audit`, `welang-change-review`, `welang-code-review`, `welang-pre-push-checks`, `welang-archive-sync` — invoked at the corresponding lifecycle transitions. A strict validation pass does NOT replace semantic review.
4. **AGENTS.md (entry discipline).** Hard rules and invariants required reading for every session, including the top-priority `refr/` rule.

## 7. Baseline Promotion (Archive)

When a change reaches `complete`, the `welang-archive-sync` skill promotes:

- Spec deltas → promoted into their authoritative home: language-behavior capabilities into the canonical spec under `docs/spec/` (creating the directory/version on first need); other project capabilities into the authoritative location declared in the change's `proposal.md`.
- Long-term tradeoffs → ADRs under `docs/decisions/`.
- Execution state → roadmap (or `openspec/changes/` listing until a roadmap exists).
- The change directory → `openspec/changes/archive/<name>/`.
- Re-run the validator after promotion; it must pass.

## 8. Known Scope (Current Limitations)

- The validator checks structure, not semantics; semantic review is carried by skills.
- Diagnostic-code global uniqueness is checked as part of spec review until a spec-registry script exists (a future `spec` layer change may add it).
- `docs/spec/`, `docs/decisions/`, `docs/roadmap/` do not exist yet; they are created by the first archive that needs them.
