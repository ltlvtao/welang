# Task control-02: product by nested repetition

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn slowProduct(a: Int64, b: Int64) -> Int64` in `src/main.we`.

Multiply two non-negative integers by repeated addition using nested loops
(the `*` operator exists but this task requires the loop form): the outer
loop runs `a` times, the inner adds `b` once per inner step. The inner
counter must restart for every outer step. `slowProduct(3, 4) = 12`,
`slowProduct(0, 9) = 0`, `slowProduct(5, 1) = 5`.
