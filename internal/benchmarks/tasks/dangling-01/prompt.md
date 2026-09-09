# Task dangling-01: join both producers before the scope ends

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn joinedSum() effect io -> Int64` in `src/main.we`.

Spawn two producer tasks under one `scope`: the first sends the value `5`
and the second sends `6` into a channel of capacity 2. Both handles must be
joined before the scope body ends. After the scope, collect both values
from the channel and return their sum (`11`).

A task handle is a scoped thing in We: every handle is awaited (or
cancelled) on every path before its scope exits, and no handle outlives
its scope.
