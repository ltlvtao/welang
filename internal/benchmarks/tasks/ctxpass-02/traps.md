# Traps — ctxpass-02

Scoring face (not shown to the model). Each entry names the detection path.

- The worker capturing an enclosing `var`: E1603.
- Never sending the config: the worker parks on its receive forever —
  deadlock abort — reference test catches (test exit 1, latent).
- Sending the reply before the reply channel exists or reading it before
  joining: wrong order — wrong value or deadlock (test exit 1). (A
  receive-with-match placed before the scope block was once mis-scored as a
  compiler fault by the codegen dominance defect; that defect is fixed as of
  codegen-mono T1 — see errpath-02's note.)
- Forgetting the join: E1607.
