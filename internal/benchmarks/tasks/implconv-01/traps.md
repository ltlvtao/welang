# Traps — implconv-01

Scoring face (not shown to the model). Each entry names the detection path.

- Declaring a narrow accumulator (`var t: Int32 = ...` or Int16/Int8) and
  assigning a bare literal to it: E0501 at check (the literal is Int64).
- Mixing widths in one expression (`t + 3i32` with `t: Int64`): E0501.
- Passing a narrow value where the Int64 contract expects Int64 (e.g. the
  test call sites): E0501.
- Widening by assignment or return (Int32 result into an Int64 return):
  E0501 — the rule has no exception.
