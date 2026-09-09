# Task dangling-02: consume the job handle inside

You are writing We (the `we` language) source for an existing project.
The project already carries `we.toml` and the scoring tests under `tests/`;
your submission replaces `src/main.we` wholesale. The tests import your
module as `main` (for example `import main` then `main.fn(...)`), so the
module path and every public signature below are fixed contracts.

Language reminders that matter here: names are camelCase (E0012), no
implicit conversion between integer widths is ever inserted (E0501), and
`std.concurrent` carries `Channel`, `Mutex`, `task`, and `scope`.

Implement `pub fn jobResult() effect io -> Int64` in `src/main.we`.

Run one task whose body produces the value `5` (a task body's final
expression is its result). Obtain the task's result inside this function
and return it. The handle must be fully consumed before the function
returns — no handle may escape.

Note the result shape: awaiting a task that produces a value does not hand
back the bare value.
