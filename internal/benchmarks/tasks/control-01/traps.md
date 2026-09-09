# Traps — control-01

Scoring face (not shown to the model). Each entry names the detection path.

- Off-by-one bounds (`i < n` vs `i <= n`): wrong total — reference test
  catches (test exit 1, latent).
- Forgetting to advance the loop counter: a busy loop with no wait
  source never parks, so no verdict face exists in-process — the run
  simply never terminates. Disclosed runner limit: the in-process runner
  cannot digest it; the real-batch face needs a wall-clock guard. Not
  machine-calibratable here.
- Mixing a narrow counter with the Int64 total: E0501 at check.
