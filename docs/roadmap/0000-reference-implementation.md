# welang Reference Implementation Roadmap

This document is the single authority for reference-toolchain execution state: the strategy, the milestone sequence, and the follow-ups discovered while implementing. It carries no language behavior (that lives in `docs/spec/`), no long-term tradeoffs (those live in `docs/decisions/` — see ADR-0002 for the host and target strategy), and no process rules (`docs/process/development-process.md`).

## Strategy

- **Thin vertical first** (user adjudication, 2026-09-04): reach the minimal end-to-end `we new` → `we check` → `we build` → `we run` early — a minimal front-end subset, LLVM IR emission, a minimal C runtime — then widen every pipeline stage chapter by chapter. This honors ADR-0002's "LLVM IR from day one" and chapter 0's completeness closure in execution order, and buys the earliest honest end-to-end signal.
- **Spec first**: every slice implements ratified chapters; no compiler behavior without a spec basis (development process, invariant list).
- **One change per milestone**: each milestone is one small, vertically verifiable openspec change; its state row below flips in the change that archives it.

## Milestones

| ID | Change | Implements | State |
| --- | --- | --- | --- |
| M0 | `compiler-bootstrap` | Chapter 21 CLI surface subset: closed subcommand table, global options, `we version` / `we new` (E1904), E1907, JSON Lines diagnostic events, conformance harness | done 2026-09-05 |
| M1 | `lexical` | Chapter 1 token model, literals, comments, keywords; `we check <file>` runs the lexical stage (E0001–E0009) | done 2026-09-05 |
| M2 | `parser-core` | Chapter 2 grammar skeleton and chapter 6 declarations; `we check` runs lexing and parsing (E0101, E0102, declaration diagnostics) | done 2026-09-05 |
| M3 | `types-and-main` | Chapter 7 types subset, chapter 15 root module and main shape, chapter 14 `Result`; `we check .` green on the `we new` skeleton | done 2026-09-05 |
| M4 | `native-vertical` | Minimal LLVM IR emission, minimal C runtime (startup, allocator), `we build` and `we run` end to end on the skeleton program | pending |
| M5 | `control-and-composites` | Chapter 3 control flow, chapter 4 match, chapter 8 composites and ownership, chapter 9 sums, chapter 12 fn types and closures | pending |
| M6 | `modules-generics-errors` | Chapter 10 interfaces and generics, chapter 11 iterables, chapter 13 resources, chapter 14 in full, chapter 15 module resolution in full, chapter 17 collections | pending |
| M7 | `effects` | Chapter 16 effect declarations and effect checking | pending |
| M8 | `stdlib-and-gc` | `std.io` core; the runtime's precise GC design lands | pending |
| M9 | `concurrency` | Chapter 18 tasks, channels, scheduler, virtual clock | pending |
| M10 | `testing` | Chapter 20 test runner, mocks, exploration | pending |
| M11 | `fmt-vet-doc` | Chapter 21 formatter, vet, documentation generation | pending |
| M12 | `ffi` | Chapter 19 foreign blocks and build-time linkage (E1906) | pending |
| M13 | `dependencies` | Chapter 22 MVS resolution, `we.lock`, the dependency cache, a local registry fixture | pending |
| M14 | `lsp` | The LSP document and `we lsp` | pending |
| M15 | `benchmarks` | The evaluation suite (First-Pass Compile Rate and its siblings) | pending |

## Execution state rules

- A milestone's State flips only in the change that archives it; execution state lives only here.
- The sequence after M4 may reorder as widening reveals dependencies; any reorder is a change to this file carried by the affected milestone's change.
- The not-implemented boundary: subcommands this reference build does not run yet report one stderr line and exit 70 (transient implementation state, not a spec surface); the set shrinks to zero as milestones land.

## Registered follow-ups

Items found during implementation that the spec does not fix; each leaves this list only through its own change.

1. **Project-name precise rule.** Chapter 21 R1/R8 and chapter 22 R1 reference "chapter 1's naming convention" for TOML-level names, but chapter 1's naming-conventions requirement fixes identifier case by binding kind only; the concrete charset lives in chapter 22's text and E1904's remediation ("lowercase letters, digits, and hyphens"). A future lexical or toolchain amendment should fix the exact rule (leading or trailing hyphens, length) in chapter 1. Until then the implementation validates: non-empty, charset `[a-z0-9-]` exactly, nothing invented.
2. **Registry embedding.** The compiler hardcodes CLI-level registry titles and remediations at call sites; embedding `diagnostics.toml` (`go:embed`) lands with the first real pipeline stage, when the installed-binary registry question must be answered.
3. **Spec-version label.** `SpecVersion` "0.9.0" (`internal/version`) labels the first canonical baseline after the private v0.8 draft; if a different labeling is adjudicated, the change is one constant plus this row.
4. **Operator type-domain table.** The ratified text names the numeric and Bool operands of the operators chapter 2 accepts, but no requirement fixes the full per-operator domain table (which base types each operator admits at each operand). Until an amendment ratifies the table, same-type operand pairs beyond the ratified numeric/Bool/comparison domains stop at an honest boundary (`arithmetic and comparisons beyond the ratified numeric and Bool domains (spec gap; roadmap follow-up)`); disagreeing operands stay E0501. Found by M3 (`types-and-main` design D4).
5. **Call argument-count and non-function-callee diagnostics.** No ratified requirement fixes the diagnostic when a call's argument count differs from the callee's declaration, or when the callee is not a function. Until an amendment claims the codes, both stop at honest boundaries (`calls with an argument count the callee does not declare (spec gap; roadmap follow-up)`, `calls on values that are not functions (spec gap; roadmap follow-up)`). Found by M3 (`types-and-main` design D10).
