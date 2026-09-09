# Task implconv-01: checksum stays in the wide domain

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn packetScore(base: Int64) -> Int64` in `src/main.we`.

A sizing routine: start from `base`, add the fixed header weight `3`, and
when the running total exceeds `10` add the jumbo surcharge `100`. Return
the total. All arithmetic is `Int64` — bare integer literals in We are
`Int64`, and no width conversion is ever inserted silently.

Examples: `packetScore(8) = 8 + 3 + 100 = 111` (11 exceeds 10),
`packetScore(2) = 5` (5 does not).
