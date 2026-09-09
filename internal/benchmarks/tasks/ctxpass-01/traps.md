# Traps — ctxpass-01

Scoring face (not shown to the model). Each entry names the detection path.

- Tasks referencing an enclosing `var`: E1603 at check (copy into an
  immutable binding or use shared state instead).
- Capturing a plain record as context: E1602 unless the type is
  synchronized.
- Recomputing `base` from module-level state: the bait is closed by
  design (no module-level var exists, E0403) — a design note, not a
  calibrated trap; the copy-in or shared-state route is the only path.
- Forgetting a join: E1607; collecting one partial: wrong total —
  reference test catches.
