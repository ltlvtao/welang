# Task race-01: shared counter under two tasks

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn sharedCount() effect io -> Int64` in `src/main.we`.

Two tasks each increment one shared counter exactly 50 times, then both are
joined. Return the counter's final value. The scoring test asserts the
result is exactly `100` — every intermediate read-modify-write on the
shared state must go through a synchronization discipline the language
accepts for cross-task state.
