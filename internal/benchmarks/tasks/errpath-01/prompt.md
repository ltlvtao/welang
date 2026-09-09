# Task errpath-01: empty wait must time out

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn emptyWait() effect io -> Int64` in `src/main.we`.

The function owns a private `Channel<Int64>` (capacity 1) that no one ever
sends on. It must wait for a value on that channel under a `scope timeout(50)`
deadline, driven by the virtual clock (the scoring tests advance time).

- If a value arrived before the deadline, return `1`.
- If the deadline fired first (the only outcome here, since the channel stays
  empty), return `100`.

The wait itself must run inside a task body joined by the scope; the scope's
result must be consumed explicitly. The function is deterministic: it returns
`100` on every run of the scoring tests.
