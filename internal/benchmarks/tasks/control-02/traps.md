# Traps — control-02

Scoring face (not shown to the model). Each entry names the detection path.

- Resetting the inner counter outside the outer loop: the second outer
  step adds nothing — wrong product — reference test catches (test exit 1).
- Swapping the roles (running `b` outer, `a` inner) is correct arithmetic
  but the counter-reset bug still yields a wrong product; caught the same
  way.
- Narrow counters mixed with the Int64 total: E0501 at check.
