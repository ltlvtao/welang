# Traps — resleak-01

Scoring face (not shown to the model). Each entry names the detection path.

- Never closing the channel and receiving a fixed count anyway: the
  fourth receive blocks forever — deadlock abort — caught by the reference
  test (test exit 1, latent).
- Matching only `Some`: non-exhaustive match — E0305 at check.
- Sending after close: runtime panic of the chapter 14 family — the
  reference test fails (test exit 1).
- Treating the end-of-stream signal as a value (adding it as a number):
  the payload is not a numeric value — wrong total or check rejection.
