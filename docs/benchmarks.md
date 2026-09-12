# The We Evaluation Suite

This is the evaluation methodology document for the We language — the single authority for how the language's compile-loop claim is measured. It defines the three metrics, the task taxonomy, the trap-list format, the batch schema, the feedback protocol, the runner's verdict order, and the statistical discipline. Two machine faces consume it: the embedded task set (`internal/benchmarks/tasks/`) and the batch runner (`internal/benchmarks`, Go). Where a face and this text disagree, this text governs and the face carries the bug.

The premise is chapter 0's first-class authoring face. In the generate–check–fix loop the model writes, the checker rejects or passes, and the model writes again — and the compiler is the one component in that loop that does not learn. Its four promised properties (observable, repeatable, controllable, attributable) are experimental claims until a suite measures them against model-written code; this suite is that measurement. Every verdict reduces to an exit code and an event stream from `we check` and `we test` — the same machine faces chapter 21 stabilizes for the loop itself, which is what makes the whole suite reproducible.

Three scope gates frame everything below. The suite judges, it never produces: model calls and feedback construction are batch-production work, and the runner's only inputs are batch files — repo artifacts under this document's schema. No CLI surface is added: `we bench` does not exist and cannot, chapter 21's command set being closed; the runner is repository tooling exercised in-process through Go tests. And the seed task set proves the methodology, not the language — no comparative claim about We comes out of the seed set itself.

## The three metrics

Every metric derives from per-attempt bucket classification. An attempt faces two verdict stages — `we check --json` first, `we test --json` only when check passes — and lands in exactly one of five buckets:

| Bucket | Verdict | FPCR | LBR |
| --- | --- | --- | --- |
| `clean` | check exit 0, test exit 0 | numerator and denominator | denominator |
| `latent` | check exit 0, test exit 1 | numerator and denominator | numerator and denominator |
| `test-malformed` | check exit 0, test exit 2 or 70 | numerator and denominator | denominator, never numerator |
| `rejected` | check exit 1 (an E-severity diagnostic event) | denominator | out of scope |
| `boundary` | check exit 70 or 2 (an honest unimplemented form, or malformed tool usage) | excluded from both sides, disclosed separately | out of scope |

The exit codes are chapter 21's stable contract: 0 pass, 1 error diagnostics found, 2 malformed usage, 70 a ratified form this reference build does not implement. Advisory `W` events never move a verdict — under the warning default the tool reports them and still exits 0, and a pass is the machine fact of exit 0, with no second reading.

**First-Pass Compile Rate (FPCR)** is measured over every task's round-1 attempt: (clean + latent + test-malformed) / (clean + latent + test-malformed + rejected). A first pass counts when the code compiles — whatever the tests then say about it; the test outcomes are LBR's question. Boundary attempts are excluded from both sides: an honest boundary is neither the model's fault nor a verdict on the language, and counting it on either side would pollute the number. The boundary share is disclosed beside the rate.

**Fix-Loop Convergence (FLC)** measures the loop's shape after a rejected first pass. The feedback protocol (below) fixes round k's input: the original prompt plus round k−1's `we check --json` event stream, verbatim. A sequence converges at its first round whose check exits 0; the convergence round is that round's number. Sequences truncate at N=5: with no check-pass round inside five attempts the sequence counts as not-converged — explicit truncation, no round count imputed. The report carries the convergence-round distribution (median, p90, max at the truncation value) and the not-converged count.

**Latent Bug Rate (LBR)** is measured over all check-pass attempts at every round, not only the last: latent / (clean + latent + test-malformed). The attempt level is the mechanical, ambiguity-free face. A task-level derived view is reported beside it — the share of tasks whose final check-pass attempt fails its tests, the intuitive reading — and the two are named apart, never blended.

Interpretation discipline: FPCR alone is not a language-quality number — a rejected attempt is positive evidence, the language catching an error the model made. The verdict face is the joint reading: FPCR with FLC (does it compile first, and how fast does the loop converge when it does not) and LBR (what slips past the checker to the tests). A language that rejected everything would score FPCR 0; a language that accepted everything would score LBR 1; the design's claim lives in the middle, and only the joint reading measures it.

## Task taxonomy

Tasks are classified by error root cause, not by business domain — the taxonomy measures which error classes the language design itself removes. Seven axes, each contributing two L1 tasks: **error-path** (a fallible call's failure branch unhandled), **race** (shared state mutated from concurrent tasks outside the synchronization discipline), **resource-leak** (a handle escaping or released outside the scope-resource discipline), **null-boundary** (an `Option` branch set with a case missing), **implicit-conversion** (mixed-width arithmetic an explicit-conversion design refuses), **dangling-reference** (use after move and aliasing discipline — the expressibility note below), and **context-passing** (implicit context capture and passing discipline — the same note).

Complexity is capped at L1 — single-function scope, 10–30 lines. Module-level (L2) and system-level (L3) tasks are future expansion, registered as a roadmap follow-up; the seed set proves the methodology first.

Four **reverse control** tasks (`control-01`..`control-04`) are pure algorithm and string work touching no axis. They separate the two explanations any axis movement could carry — "the language design removes this error class" versus "the model simply finds this syntax handy" — because control tasks move with the second factor only.

Each task directory `internal/benchmarks/tasks/<axis>-<nn>/` (controls: `control-<nn>`) holds four artifacts:

1. `prompt.md` — the requirement, in English. The prompt is the machine face of a real batch; LLM prompting is the evaluation baseline, and cross-model comparison needs one prompt language.
2. `traps.md` — the trap list (next section); the scoring face, never shown to the model.
3. `reference/` — the reference solution as a complete project (`we.toml` + `src/`), check-clean and test-green by construction — the self-certification test enforces both, machine-checked.
4. `reference/tests/` — the reference test face: tests ship with the task, not the attempt; against the reference solution `we test` is green.

**Attempt assembly.** The runner copies the reference project to a temporary directory and overlays the attempt's `files` (a path → source map) onto `src/`. The test face is fixed — scoring is objective — and the model's freedom is the whole of `src/`: restructuring it, adding modules, all legal. If a restructuring breaks the reference tests' imports, the test stage exits 2 and the attempt lands in `test-malformed` — an honest classification, not a runner error.

**Expressibility disclosure.** Two axes deserve their own paragraph. The classic decoy forms of dangling references and implicit context are closed by design, under two independent rulings: reference types are never introduced into We (the chapter 18 decision — `Ref<T>` does not exist), and module-level mutable state does not exist (`E0403`: no top-level `var`). The unexpressible cannot be a task. The context-passing axis therefore anchors on the adjacent expressible form — capture and effect discipline (`E1602`/`E1603`); the dangling-reference axis anchors on the task-handle lifecycle — a handle reaching scope exit un-awaited (`E1607`) and a task spawned outside any scope (`E1618`) — because the byres run face of this reference build is not implemented (any use of a byres resource inside a function body is run-declined), leaving `E1104`/`E1105` as check-anchored decoys that traps.md notes where the prompt invites them. Each such task's `traps.md` discloses the mapping. This is itself part of the measurement: when a model writes the closed form, `we check` rejects it outright — a suppressed FPCR and a converging loop is the design working, exactly as measured.

**Run-face anchoring.** The reference build's run tower implements a deliberate subset of the ratified language, and L1 tasks anchor within it. That subset has widened with the codegen-mono line: sums construct, return, and match — a user function may take and return `Result`/`Option`/a sum of its own, so a sum is no longer confined to the primitive calls that produce one; sum-typed and synchronized-typed parameters enter runnable slots; a numeric return may be any checked integer width rather than `Int64` alone (`fn sum(a: Int16, b: Int16) -> Int16` runs, and its arithmetic is checked against `Int16`, with an overflow trap that names the width); `String` equality and concatenation run, as do records with methods, tuples, newtypes, `List` iteration, closures and function values, and top-level bindings; `%` and `/` emit checked arithmetic; a function body may return from any depth, not only its tail; and a `let` bound from a value-position `if`/`match`/block whose branches agree on a scalar reads into a `String` position. The codegen-full line has since widened that subset to generic functions: a generic declaration's applications are monomorphized from what the check stage determined, so `fn id<T>(x: T) -> T` applied to an `Int64` runs end to end. What remains closed: a plain `scope { … }` read as a value, `?` unwrapping in the main body, a tuple-value binding read into a `String` position, a function value called in a value-string or operand position (`"${f(1)}"`, `f(1) + 1`; its statement and tail positions run), and a builtin-produced `Option` payload (`reduce`/`find`/`next`) read into a `String` position — each an honest boundary (exit 70), each disclosed here. One runtime rule is unchanged: a wait belongs in a task body — the main fiber parking on a `receive` inside its own timeout scope deadlocks under the virtual clock. These constraints shape task authoring, not model freedom: an attempt that leaves the subset is classified (boundary or test-malformed), never silently passed, and the set itself shrinks as the codegen-full line lands.

## Trap lists

Each task's `traps.md` carries the traps the requirement invites. Every trap is one entry naming the wrong shape in one line and the detection route in the next — a registry code (`we check` catches it) or a reference-test assertion (`we test` catches it). The list is the scoring face; it never enters a prompt.

A trap list is a checked artifact, not prose. The calibration test injects each trap's typical wrong shape into the reference solution and asserts that check or test — at least one — catches it. A trap nothing catches is a task-design defect: the task or the list is fixed and the miss disclosed, never left silent.

## Batch schema and feedback protocol

A batch file is one task's attempt sequence — a repo artifact:

```json
{
  "model": "string (free-text identifier)",
  "task": "task id (must exist in the task set)",
  "attempts": [
    {"round": 1, "files": {"src/main.we": "…"}},
    {"round": 2, "files": {"src/main.we": "…"}}
  ]
}
```

Load-time validation rejects the batch itself on: an empty `model`; an unknown `task`; rounds not starting at 1 and continuous (no gaps, no repeats); an empty `files`; any path outside `src/` — touching `tests/` or `we.toml` is tampering with the scoring face and is rejected, not classified. New files under `src/` are legal under the assembly model above.

**The feedback protocol** defines what a round-k (k>1) attempt's generation input was: the original `prompt.md` verbatim, plus the round-(k−1) `we check --json` event stream verbatim — line by line, codes, messages, files, lines, columns, help. That literal shape is the generate–check–fix loop: the model reads the machine's own words. The runner never constructs feedback — verdicts and production are separated by design (one authority per class of fact: judging belongs to the runner, batch production to the production process). Synthetic batches hand-write each round's text under this protocol's shape; real batches produce theirs however their process does, and the runner cannot tell the difference — by design.

## The runner's verdict order

Per attempt, four stages in order:

1. **Assemble.** Copy the reference project to a fresh temporary directory; overlay the attempt's `files` onto `src/`. The directory is removed after the attempt.
2. **Check.** Run `we check <dir> --json` through the in-process CLI entry — the same injected-stream face the conformance runner uses: no subprocess, no new entry point. Exit 0 proceeds; exit 1 lands in `rejected`; exit 70 or 2 lands in `boundary`.
3. **Test** (check-passers only). Run `we test <dir> --json` the same way. Exit 0 → `clean`; exit 1 → `latent`; exit 2 or 70 → `test-malformed`.
4. **Classify and record.** The attempt's bucket, exit codes, and diagnostic codes enter the report.

Determinism is a property of the suite, not an aspiration: no clock, no network, no path leakage. The working-directory discipline matches the conformance runner (enter, run, restore), and nothing machine-varying enters a report — the same batch file yields a byte-identical report on every run.

One disclosed limit: evaluation is in-process, so an attempt whose compiled code never terminates — a busy loop with no wait source, the one shape the deadlock detector cannot see — halts its evaluation. Deadlock-shaped wrong attempts abort deterministically under the virtual clock and classify normally; the real-batch production face must add a wall-clock guard before feeding untrusted attempts.

The **report** is a JSON machine face: per-attempt buckets, a per-task view, the three metrics (round-1 FPCR; FLC distribution with the not-converged count; attempt-level LBR with the task-level derived view), boundary and malformed counts (never absent — next section), and the sample sizes behind every ratio.

## Statistical and reporting discipline

- Every ratio carries its denominator. No bare percentages exist in a report.
- Truncation is explicit. FLC truncates at N=5; truncated sequences count as not-converged, and no round count is imputed for them.
- The boundary and test-malformed buckets are never silent. Every report carries both counts — the honest-boundary disclosure discipline of exit 70, carried into evaluation.
- Replays are byte-identical (the determinism above); temporary paths never enter a report — only task ids, buckets, and metrics.
- Real batch files are repo artifacts under this schema; reports are reproducible products and are not committed unless explicitly asked for.
- The seed set carries no statistical conclusion. Eighteen tasks prove the methodology runs; they power no comparative claim about the language — that is what real batches are for.
