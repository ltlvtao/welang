# Task implconv-02: narrow ids keep their own domain

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn gate(seed: Int32) -> Int64` in `src/main.we`.

Ids arrive as `Int32`. Keep the arithmetic in the narrow domain: add the
fixed offset `3` (written `3i32`), then compare against `5i32`. Return
`1` when the gated value equals five, `0` otherwise. The public return is
`Int64` — do not return the narrow value itself.

Examples: `gate(2i32) = 1` (2 + 3 = 5), `gate(9i32) = 0`.
