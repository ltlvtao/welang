# Task resleak-01: close the stream and drain it

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn drainSum() -> Int64` in `src/main.we`.

The function owns a buffered `Channel<Int64>` (capacity 4). Push the values
`10`, `20`, `30`, then close the channel, then receive until the stream
reports its permanent end. Return the total of the values observed (`60`).
The channel's end-of-stream signal is part of the contract: a drained,
closed channel reports it on every further receive.
