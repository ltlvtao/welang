# Traps — dangling-02

Scoring face (not shown to the model). Each entry names the detection path.

- `let n: Int64 = t.await()`: the await answers with a Result (task
  panic included), not the bare value — E0501 at check when the annotation
  or use disagrees.
- Dropping the await's Result (`let _ = t.await()` then returning a
  constant): wrong value — reference test catches (test exit 1, latent).
- Matching only the Ok arm of the await result: non-exhaustive — E0305
  at check.
- Blind `(t.await())?` propagation in a non-Result function: E1202 at
  check (the propagation site).
- Handle un-awaited / escaped: E1607; task outside scope: E1618.
