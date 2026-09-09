# Task ctxpass-01: carry the context in by copy

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn scaledMerge(base: Int64) effect io -> Int64` in `src/main.we`.

Two tasks both need the caller's `base` value and a shared scale `3`: one
task computes `base + scale`, the other `base * scale`. The enclosing
values must reach the task bodies legally — mutable enclosing state is not
capturable — so carry the context in the way the capture rules sanction.
Both tasks are joined, both results collected via a channel, and the total
returned. For `base = 10`: `13 + 30 = 43`.
