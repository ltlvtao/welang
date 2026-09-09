# Traps — control-03

Scoring face (not shown to the model). Each entry names the detection path.

- Attempting concatenation (`s1 + s2`): outside the ratified arithmetic
  domains — the honest-boundary face (check exit 70), reported as boundary
  per the methodology.
- Branching on string equality inside the implementation (`if s == "x"`):
  check-clean, the run declines the comparison (test stage exit 70 — the
  test-malformed bucket).
- Wrong literal: reference test catches (test exit 1, latent).
