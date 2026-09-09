# Traps — race-02

Scoring face (not shown to the model). Each entry names the detection path.

- Tasks capturing an enclosing `var`: E1603 at check.
- Handles reaching scope exit un-awaited: E1607.
- Collecting only one partial (or the same one twice): wrong total —
  reference test catches (test exit 1, latent).
- Un-buffered channel with sends before the receivers exist is a
  rendezvous, not a bug — but receiving before joining while the producer
  was never spawned deadlocks (test exit 1).
