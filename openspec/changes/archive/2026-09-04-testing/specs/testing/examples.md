# Testing — illustrative examples

The examples below use only surface forms ratified by chapters 1–20. They are illustrative, non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines annotated with a diagnostic code are rejected forms, shown with the code the compiler emits.

### A test module

```we
// file: math_test.we — the _test.we name makes the module a test module
import std.test

fn abs(n: Int64) -> Int64 {                 // helper under test, same module
    if n < 0 { -n } else { n }
}

test "absolute value" {
    std.test.assertEqual(abs(-3), 3)        // the assert family is std.test surface
}

test "zero" {
    assert(abs(0) == 0, "abs of zero")      // assert itself is prelude, chapter 14
}
```

### Mocks intercept by name

```we
// file: cache_test.we
import std.test
import store

fn load(key: String) -> String effect io { store.read(key) }   // under test

test "served from the mock, not the store" {
    mock store.read(key: String) -> String effect io { "cached" }
    std.test.assertEqual(load("k"), "cached")   // the call site resolves to the
}                                               // mock's body; the store never runs

test "a second mock of one target is a duplicate" {
    mock store.read(key: String) -> String effect io { "cached" }
    // mock store.read(key: String) -> String effect io { "other" }
    //                                     // E1805: duplicate mock of one target in a test block
}
```

### The virtual clock

```we
// file: timer_test.we
import std.test

test "a scope timeout fires under advanceTime" {
    fn pollPeriod() -> Int64 effect time { 10 }    // time runs on the virtual clock

    scope timeout(100) {
        task effect time {
            let _ = sleep(50)                   // registered on the virtual clock
        }
    }
    advanceTime(100)                            // the budget expires here,
    std.test.assertTrue(true)                   // deterministically, every run
}
```

### Deterministic scheduling

```we
// file: relay_test.we
import std.test

test "relay order reproduces on every run" {
    let ch = channel(0)                          // unbuffered, std.concurrent
    scope {
        task effect time { let _ = sleep(10); ch.send(1) }
        task effect time { let _ = sleep(10); ch.send(2) }
    }
    advanceTime(10)                              // both waits tied at one instant:
    // the wake order is fixed for this source — same test, same advanceTime
    // sequence, same order, on every run
}
```

### Rejected forms

```we
// test "x" { }                               // E1801: test block outside a test module
//                                            //        (in a file not ending _test.we)
// fn helper() { mock f() { } }               // E1802: mock declaration outside a test block
// mock find(name: String) -> User { }        // E1803: mock signature does not match its target
//                                            //        (target: fn find(id: Int64) -> User effect io)
// mock id(x: Int64) -> Int64 { x }           // E1804: mock target is not a mockable function
//                                            //        (target: fn id<T>(x: T) -> T)
// pub test "x" { }                           // E0105: unexpected token — pub on a test block
// fn drive() { advanceTime(5) }              // E1806: advanceTime called outside a test block
```

### Pending later changes

```we
// The test runner — we test, the tests/ directory layout, parallelism,
// reporting, exit codes — is the toolchain chapter's business; exploration
// (--explore, iteration controls, partial-order reduction) and its guard
// diagnostics (W0601, E1001, E1003) are that layer's too. The std.test
// surface — the assertEqual family, property combinators — is a standard
// library release's:
//
// we test --explore --iterations 200        // toolchain, not this chapter
```
