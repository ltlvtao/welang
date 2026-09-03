# ADR-0001: English-authoritative bilingual documentation under docs/

- Status: Accepted
- Date: 2026-09-03
- Decided in: openspec change `dual-language-docs`

## Context

Project interaction happens in Chinese, while `docs/` was English-only. A single language cannot serve both the Chinese-reading primary audience and the ecosystem/tooling value of English (spec deltas are literal English fragments, commit messages are English, identifiers and diagnostic terms are English).

## Decision

Documentation under `docs/` ships with dual-language synchronized support:

1. **English is the authoritative text.** Spec deltas are written in English and merged verbatim into the English canon; the Chinese rendering is a synchronized translation. On any disagreement, English governs, and the translation is corrected in the change that introduced the divergence.
2. **Sibling-file layout.** Every `*.md` has a paired `<basename>.zh.md` in the same directory, inheriting every naming element (numeric prefixes, ADR numbers).
3. **Structural checking only.** `openspec/tools/docs_sync.py` checks pairing, heading counts per level, and fenced code block counts. Semantic equivalence of translations is carried by change review, not by script.
4. **Separate from `validate.py`.** The two checkers have different responsibilities, triggers, and failure semantics; each occupies its own row in the verification ladder.

## Consequences

- Documentation writing and review effort roughly doubles; accepted.
- A change that touches `docs/` does not reach `complete` until its translation is updated and `docs_sync.py` passes.
- Files outside `docs/` stay single-language by design (`AGENTS.md`, `openspec/`, `.agents/` Chinese; code comments and commit messages English).

## Rejected alternatives

- **Chinese-authoritative docs:** would reverse the "spec deltas are literal English fragments" rule and separate the canon from English identifiers and diagnostic terms. Rejected (user decision, 2026-09-03).
- **Parallel tree (`docs/` + `docs-zh/`):** doubles path maintenance, requires cross-tree link rewrites, and makes orphaning easier. Rejected.
- **Single-file bilingual (alternating languages):** cannot render one language at a time; noisy diffs; ambiguous anchors. Rejected.
- **Merging `docs_sync.py` into `validate.py`:** couples document pairing to change-artifact structure with different trigger conditions. Rejected.
