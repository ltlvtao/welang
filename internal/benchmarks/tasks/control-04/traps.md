# Traps — control-04

Scoring face (not shown to the model). Each entry names the detection path.

- Using `%` or `/`: check accepts the operator, the run declines it —
  the attempt lands check-clean with the test stage exiting 70 (the
  test-malformed bucket).
- Loop condition off-by-one (`m > 2` instead of `m >= 2`): `2` itself
  classifies wrong — reference test catches (test exit 1, latent).
- Mutating the parameter binding instead of a local copy: a `let`
  parameter is not an assignable slot; check tolerates the text but the
  run declines it (test stage exit 70 — test-malformed). The
  copy-into-var form is the sanctioned one.
