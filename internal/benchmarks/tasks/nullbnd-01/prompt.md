# Task nullbnd-01: count above threshold until end of stream

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn countAbove() -> Int64` in `src/main.we`.

The function owns a buffered `Channel<Int64>` (capacity 4). Feed the
readings `5`, `12`, `9`, `20` into the channel, close it, then drain the
whole stream to its permanent end. Return how many readings exceed `10`
(the answer is `2`).

Every receive on the drained stream reports the boundary — the counting
loop must treat it as end-of-stream, not as a reading, and must not block
past it.
