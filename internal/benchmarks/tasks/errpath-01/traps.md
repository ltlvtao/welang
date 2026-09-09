# Traps — errpath-01

Scoring face (not shown to the model). Each entry names the detection path.

- Waiting with a plain blocking `receive()` and no timeout: the fiber parks
  forever, the run aborts with the deadlock diagnostic — caught by the
  reference test (test exit 1, latent).
- Dropping the scope result (`scope timeout(...) { ... }` as a bare
  statement or `let _ =` it away): check rejects with E0605 (non-unit value
  dropped; the scope yields Result).
- Matching only the `Ok` arm of the scope result: non-exhaustive match —
  E0305 at check (chapter 9 exhaustiveness).
- Blind `r?` propagation: the function's declared return is Int64, not
  Result — E1202 at check (the propagation site).
- Receiving directly on the function's own fiber inside the timeout scope:
  parks the fiber that owns the scope — deterministic deadlock abort under
  the virtual clock — caught by the reference test (test exit 1).
