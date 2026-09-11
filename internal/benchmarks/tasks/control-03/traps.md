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
  calibration this entry predicted it would stop being. control-03 now
  carries no test-malformed shape: the bucket's carrier in the black-box
  battery is the generic-fn boundary (a chapter 10 declaration the test
  stage declines until B1b monomorphization), not a control-03 mutation.
- Wrong literal: reference test catches (test exit 1, latent).
