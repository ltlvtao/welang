# Task control-04: parity by repeated subtraction

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn parityClass(n: Int64) -> Int64` in `src/main.we`.

For a non-negative `n`, return `0` when `n` is even and `1` when odd, by
repeated subtraction of `2` (a subtraction loop — the modulo operator is
not in the ratified arithmetic set for this exercise).
`parityClass(10) = 0`, `parityClass(7) = 1`, `parityClass(0) = 0`,
`parityClass(1) = 1`.
