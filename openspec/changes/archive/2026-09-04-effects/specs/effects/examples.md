# Effects — illustrative examples

The examples below use only surface forms ratified by chapters 1–16. They are illustrative, non-authoritative: on any conflict the Requirements and Scenarios prevail. Lines annotated with a diagnostic code are rejected forms, showing the code the compiler emits.

### Effect declarations

```we
effect db                              // camelCase, joins the module's one
                                       // name space (E0404 on collision)
effect io
// E1403: effect name conflicts with a built-in effect — io, net, time are the
// language's own tags

pub effect audit                       // cross-module: qualified b.audit

fn write(msg: String) effect io {      // declared set {io}
    save(msg)                          // save declares effect io: within
}

fn greet() {
    save("hi")
    // E1401: undeclared effect at a call — io is not in the declared set
}
```

### Segments and defer

```we
fn log(msg: String) effect io {        // segment without a return type
    save(msg)
}

fn pure(n: Int64) -> Int64 {
    assert(n > 0, "positive")          // panic family carries no effect
    panic("unreachable")               // callable from any function
}

fn bad() {
    defer { save("bye") }
    // E1401: undeclared effect at a call — defer shifts timing, never effect
    // attribution; the enclosing function must declare io
}
```

### Function types and closures

```we
let f: fn(Int64) io -> Int64 = fetch   // bare tags in type position
let g: fn(Int64) -> Int64 = |n| n + 1  // pure value in effect-expecting slot
                                       // is legal: empty set ⊆ {io}

let h: fn(Int64) -> Int64 = fetch
// E1402: function value effect set does not match the expected type's — io
// is not in the expected set

fn k() io { }                          // E0105: bare tags belong to type
                                       // position; declarations use `effect`

fn calc(rows: List<Int64>) -> Int64 {
    rows.map(|r| score(r))             // short closure's set inferred from
}                                      // body: score is pure, set is empty
```

### Interfaces and impls

```we
interface Store {
    fn get(mut self, k: String) effect io -> String
}

impl Store for FileStore {
    fn get(mut self, k: String) effect net -> String { ... }
    // E1404: impl method effect set disagrees with the interface — net is
    // not io
}

fn use(store: Dyn<Store>) effect io -> String {
    return store.get("k")              // checked against the interface's
}                                      // set, whichever implementation runs
```

### Initializers stay pure

```we
pub let retries = 3                    // literals and pure calls are fine

pub let host = loadHost()
// E1405: effectful call in a top-level initializer — move the work into main
```
