# Task control-03: string tag and identity

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement two functions in `src/main.we`:

- `pub fn tag() -> String` — returns the fixed tag `"we-bench"`.
- `pub fn echo(s: String) -> String` — returns exactly the string passed
  in.

The scoring tests compare results for equality. The string domain here is
literal production and pass-through; no transformation is required.
