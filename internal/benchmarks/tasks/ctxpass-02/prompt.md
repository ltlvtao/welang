# Task ctxpass-02: hand the context through a channel

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn scaled(base: Int64) effect io -> Int64` in `src/main.we`.

The worker task must not capture the caller's `base` at all — not even by
copy. Instead, hand the configuration to the worker through an unbuffered
channel (capacity 0): send `base + 1` as the config. The worker receives
its config, doubles it, and replies `config * 2` on a reply channel
(capacity 1). The worker is joined inside its scope, and the function
returns the reply. For `base = 10`: config 11, reply `22`.
