# Task errpath-02: one value then the stream runs dry

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn drainOrTimeout() effect io -> Int64` in `src/main.we`.

The function owns two private channels of `Int64` (capacity 1 each):
`loaded`, pre-queued with the value `7`, and `dry`, permanently empty.

1. Read the queued value from `loaded` (this receive is immediate).
2. Wait for a value on `dry` under a `scope timeout(50)` deadline, with the
   wait running inside a task body joined by the scope.
3. Consume the scope result explicitly.

Return `got + 1000` when the deadline fired (the only outcome for `dry`;
`7 + 1000 = 1007`) and `got + 1` had a value arrived.
