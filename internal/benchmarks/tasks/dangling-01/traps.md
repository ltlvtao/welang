# Traps — dangling-01

Scoring face (not shown to the model). Each entry names the detection path.

- A handle reaching scope exit un-awaited (forgetting either join):
  E1607 at check.
- Spawning a task outside any scope: E1618.
- Returning or otherwise moving a handle out of its scope to await it
  elsewhere: E1607 (the handle escape face).
- Classic alias bait — byres-record handles with `let g = f` rebinding:
  E1105 / E1104 at check when written; disclosed mapping: the linear
  resource face is check-anchored only in this build (its runtime use is
  not implemented), and this task's prompt does not invite it.
