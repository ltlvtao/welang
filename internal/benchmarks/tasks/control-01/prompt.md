# Task control-01: series sum by loop

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn seriesSum(n: Int64) -> Int64` in `src/main.we`.

Return the sum of the integers `1` through `n` inclusive, computed by a
loop (not by the closed-form formula). `seriesSum(0) = 0`,
`seriesSum(1) = 1`, `seriesSum(10) = 55`.
