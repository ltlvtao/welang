# Traps — errpath-02

Scoring face (not shown to the model). Each entry names the detection path.

- Blocking `receive()` on `dry` with no deadline: parks forever — deadlock
  abort — caught by the reference test (test exit 1, latent).
- Dropping the scope result: E0605 at check.
- Missing `Err` arm of the scope result: non-exhaustive match — E0305
  at check.
- Assuming the second receive yields `None` instead of blocking: an open
  empty channel blocks, it does not yield None (None is close-only) — if
  written as an un-timeout'd drain it deadlocks (test exit 1); if written
  by closing `dry` first the value contract breaks (test exit 1).
- Implementation note (disclosed; defect since fixed): a
  match-with-arm-assignment executed before a `scope timeout` block in the
  same function once emitted non-dominating IR — `Instruction does not
  dominate all uses` — so a submission that ordered the two that way landed
  check-clean with the test stage failing (exit 1 carrying the clang
  diagnostic): a latent-shaped verdict caused by the compiler, not the
  model. The emission environment is now scoped per block
  (roadmap follow-up #16, fixed by codegen-mono T1), and the shape compiles,
  links, and runs clean on the reference build. The reference still defers
  its first match to after the scope; that ordering is no longer required.
