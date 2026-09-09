# Task race-02: fan-in merge of two partial results

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn mergeScale(base: Int64) effect io -> Int64` in `src/main.we`.

Two tasks run concurrently: one computes `base + 1`, the other `base + 2`.
Each sends its partial result into a channel; the function collects both
after joining the tasks and returns the total. For `base = 10` the result
is `23`.
