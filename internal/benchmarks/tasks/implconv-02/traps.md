# Traps — implconv-02

Scoring face (not shown to the model). Each entry names the detection path.

- Adding a bare literal to the Int32 accumulator (`acc + 3`): E0501 at
  check (the literal is Int64, the accumulator Int32).
- Returning the Int32 accumulator from the Int64 function: E0501 at check.
- Declaring the accumulator Int64 while the parameter is Int32
  (`var acc: Int64 = seed`): E0501.
- Comparing across widths (`acc == 5`): E0501.
