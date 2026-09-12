# Traps — control-03

Scoring face (not shown to the model). Each entry names the detection path.

- Attempting concatenation (`s1 + s2`): ratified by T4 of codegen-mono —
  the join emits and runs, so the wrong output reaches the reference test
  instead of stopping at a boundary (latent, test exit 1). Before T4 this
  was the honest-boundary face (check exit 70), reported as boundary per
  the methodology.
- Branching on string equality inside the implementation (`if s == "x"`):
  ratified by T4 as well — the comparison emits and runs, and the wrong
  branch reaches the reference test (latent). Before T4 the run declined
  the comparison at the test stage (test exit 70 — the test-malformed
  bucket).
- Holding the value in a mutable String binding (`var r: String = s`) to
  branch on it: ratified by T9-4 of codegen-mono — the two-word storage
  face emits, so the wrong branch reaches the reference test instead of
  stopping at a boundary (latent, test exit 1). Between T4 and T9-4 this
  was the task's test-malformed carrier (test exit 70), which is the
  calibration this entry predicted it would stop being. The generic-fn
  boundary that carried the bucket afterwards retired with B1b T4
  (monomorphization drives the applications the check stage determined),
  exactly as the previous entry predicted it would.
- Renaming the contract (`pub fn echo` written as `echo2`): the prompt
  fixes every public signature, so this submission checks clean — its own
  source is self-consistent — and the test stage reports the unresolved
  name (E1304, exit 2) compiling the reference test against it. The
  test-malformed bucket's carrier now, in the black-box battery and in
  the trap table alike. Unlike the three boundaries above it retires on a
  change to name resolution, not on a widening.
- Wrong literal: reference test catches (test exit 1, latent).
