# Task nullbnd-02: poll the closed state without blocking

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn pollScore() -> Int64` in `src/main.we`.

The function owns a buffered `Channel<Int64>` (capacity 4). Queue the
values `4` and `7`, close the channel, then issue exactly three
non-blocking polls (`tryReceive`). Each poll answers with a three-state
result distinguishing value-present, empty, and closed — the states do not
fold into one another.

Return the sum of the values received plus `100` if the closed state was
observed among the three polls (the answer is `4 + 7 + 100 = 111`).
