# Task resleak-02: release guard runs after the work

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn guarded() effect io -> Int64` in `src/main.we`.

A task acquires a resource modeled by a `conc.Mutex<Int64>` guard: it sets
the guard to `3` on acquisition, then continues its body, and must arrange
for the release (setting the guard to `7`) to run when the task's body
finishes — no matter how the body is extended later. The function joins the
task and returns the guard's final value (`7`).

The release must be scheduled at acquisition time, not performed as the
body's last statement: the scoring contract is the guard's value after the
task has been joined.
