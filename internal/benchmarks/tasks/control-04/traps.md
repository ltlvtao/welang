# Traps — control-04

Scoring face (not shown to the model). Each entry names the detection path.

- Using `%` or `/`: no longer a decline — the reference build emits
  checked division and remainder (codegen-mono design D2), so this path
  is a clean program and the trap is retired.
- Loop condition off-by-one (`m > 2` instead of `m >= 2`): `2` itself
  classifies wrong — reference test catches (test exit 1, latent).
