# Traps — resleak-02

Scoring face (not shown to the model). Each entry names the detection path.

- Forgetting the deferred release (only `set(3)`): the guard stays `3` —
  reference test catches (test exit 1, latent).
- `defer` placed anywhere but a direct item of the function body's block
  (or the task body's block): E0204 at check.
- Handle un-awaited: E1607.
- Acquiring without ever scheduling the release and masking it with a
  matching body order (set 3, then set 7 as the body's last statement):
  observably identical here — not a catchable face at L1, disclosed; the
  catchable face is the forgotten release above.
