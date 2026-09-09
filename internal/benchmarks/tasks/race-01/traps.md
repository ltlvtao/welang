# Traps — race-01

Scoring face (not shown to the model). Each entry names the detection path.

- Both tasks capturing a `var` counter (or any var) from the enclosing
  function: check rejects with E1603 (task block captures a var binding).
- Capturing an unsynchronized gc-category value (a plain record) in a task:
  E1602.
- Relying on interleaved unsynchronized increments: not expressible without
  tripping E1603/E1602 first (the design closes the bait); a wrong final
  count is caught by the reference test (test exit 1, latent).
- Spawning tasks outside any `scope`: E1618; handles reaching scope exit
  un-awaited: E1607.
