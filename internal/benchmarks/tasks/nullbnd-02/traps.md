# Traps — nullbnd-02

Scoring face (not shown to the model). Each entry names the detection path.

- Folding the three poll states into one match arm (or into Option):
  the arms disagree with the sum type — check rejects the non-exhaustive or
  mistyped match; conflating empty with closed yields the wrong score —
  reference test catches (test exit 1, latent).
- Queuing only one value (forgetting the second send): the third poll
  still reports closed, but the sum is short — reference test catches
  (test exit 1).
- Forgetting `close`: the third poll answers empty-not-closed — no bonus —
  reference test catches (test exit 1).
- Matching only `Received`: non-exhaustive — E0305 at check.
