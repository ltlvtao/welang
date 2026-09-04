# Toolchain — illustrative examples

The examples below use only surface forms ratified by chapters 1–21. They are illustrative, non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines annotated with a diagnostic code are rejected forms, shown with the code the toolchain emits.

### A project and its manifest

```toml
# file: we.toml — the skeleton this chapter fixes; the [dependencies]
# table is not designed yet and its absence is legal
name = "demo"
version = "0.1.0"
type = "executable"

[vet]
W1910 = "warning"        # the default, spelled out; "error" promotes, "ignore" suppresses

[test]
explore-iterations = 200
```

```toml
# type = "bin"                           // E1903: invalid toolchain configuration value
# explore-iterations = 0                 // E1903: invalid toolchain configuration value
# (we build run in a directory with no we.toml)
#                                        // E1905: project manifest missing or incomplete
```

### The command surface

```sh
we new demo                # skeleton: we.toml, src/main.we, tests/main_test.we
we check .                 # full check pipeline, no artifact, no link
we check tool.we           # single-file compilation: std.* only, no manifest needed
we build                   # artifact named by the manifest; links foreign declarations
we test                    # every *_test.we under tests/, recursively
we test --filter "relay.*" # descriptions matching the pattern; empty match exits 0
we test --explore --iterations 200
we version                 # shape fixed, numbers never in the spec's text
```

```sh
# we build nosuchdir                      # E1907: command path not found
# we new My_Project                       # E1904: invalid project name
```

### The pipeline's observable order

```sh
# a file holding both a parse error and a type error reports the parse error —
# the stages run in order and the first failing stage is the run's verdict:
#
# we check src/one.we
# error[E0102]: ... parse error reported, type error not reported in this run
```

### The formatter's rules

```we
// before we fmt (tabs, CRLF stripped here, unordered imports, tight commas):
// import zeta
// import std.io
// import alpha
// fn add(x: Int64,y: Int64)->Int64 { return x + y }
```

```we
// after we fmt:
import std.io

import alpha
import zeta

fn add(x: Int64, y: Int64) -> Int64 { x + y }
```

### we test's run shape

```we
// file: tests/order_test.we — three blocks run in source order
test "first" { assert(true, "first") }
test "second" { std.test.assertEqual(1 + 1, 2) }
test "third" { panic("arrived at the boundary") }   // this test fails, the run continues
```

```sh
# we test
# ... three test-result events in source order; exit 1 — one failure, no error outcome
```

### Exploration and its guards

```we
// file: tests/probe_test.we
test "probe tied wakes" {
    let ch = channel(0)
    scope {
        task effect time { let _ = sleep(10); ch.send(1) }
        task effect time { let _ = sleep(10); ch.send(2) }
    }
    advanceTime(10)
}
```

```sh
we test --explore --iterations 200
# E1901 fires when explored runs diverge; E1902 when an explored run executes an
# unmocked custom effect — the condition W1910 advises on in an ordinary run
```

### The JSON Lines protocol

```json
{"type":"diagnostic","severity":"error","code":"E1907","message":"command path not found","file":"","line":0,"column":0}
{"type":"test-result","file":"tests/order_test.we","name":"third","status":"fail","duration_ms":3}
{"type":"test-summary","total":3,"passed":2,"failed":1,"duration_ms":11}
```

### Linkage

```we
// file: src/math.we — declaration per chapter 19; binding per this chapter
foreign "c" {
    fn abs(x: Int64) -> Int64 effect
}
```

```sh
# we check    — no link, E1906 cannot fire
# we build    — E1906: unresolved native symbol, if no symbol answers the binding
```

### Advisory findings

```we
// W1910: unmocked custom effect in a test — a test path calls an unmocked
//        custom-effect function; real call, advised mock
// W1911: possible indirect nested access to one shared value — a helper called
//        inside a shared value's callback may touch the same binding; the
//        survey E1613 cannot make, conservative by design
// W1912: function may block on a wait — a function body calls Semaphore.acquire;
//        a may-block note, informational
```

### Pending later changes

```toml
# [dependencies] is the named gap: version constraints, lockfiles, install and
# publish — the acquisition story awaits its own change. The LSP protocol lives
# in a separate document; its one spec promise is here: editor diagnostics are
# we check's.
```
