# Traps — nullbnd-01

Scoring face (not shown to the model). Each entry names the detection path.

- Matching only `Some`: non-exhaustive match — E0305 at check (the
  boundary arm is mandatory).
- Converting the boundary into a numeric reading (adding it to the count):
  wrong count — reference test catches (test exit 1, latent).
- Forgetting `close`: the drain receive blocks forever — deadlock abort —
  reference test catches (test exit 1).
- Draining a fixed count instead of draining until the boundary hides
  only while the count matches; the missing `close` is what the reference
  test surfaces (the drain receive deadlocks — test exit 1).
