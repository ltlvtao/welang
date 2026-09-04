# Examples (non-authoritative)

The examples below illustrate the change using only surface forms ratified by chapters 1–13. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits.

### Result and its error position

```we
type AppError = NotFound(String) | Denied(String)   // a named sum: a legal
                                                    // error parameter
type Lookup = Result<Int64, AppError>               // Ok(Int64) | Err(AppError)

let bad: Result<Int64, String> = make()
// E1204: Result error type is not a named sum type — String names no sum

fn run[E](r: Result<Int64, E>) { }
// E1204: Result error type is not a named sum type — the generic parameter
// admits no closure either
```

### Propagation with ?

```we
fn readConfig(path: String) -> Result<String, AppError> {
    let text = readFile(path)?             // Err(AppError) returns here on
    return Ok(text)                        // error; Ok unwraps to String
}

fn lenOf(path: String) -> Result<Int64, ParseError> {
    let text = readFile(path)?
    // E1203: ? error type disagrees with the declared error type — convert
    // by match, written explicitly
    return Ok(text.len())
}
```

### Context rules

```we
fn firstLine(s: String) -> Int64 {
    let text = parse(s)?
    // E1202: ? outside a function returning Result — the context function
    // returns Int64
    return text.len()
}

fn guard() {
    defer { readFile("a")? }
    // E1202: ? outside a function returning Result — a defer body runs at
    // exit; no return to propagate to
}

fn deep(id: Int64) -> Int64 {
    let n = find(id)?
    // E1201: operand of ? is not a Result type — Option does not propagate
    return n
}
```

### The panic family and unwinding

```we
fn find(id: Int64) -> Int64 {
    if id == 0 { panic("no such id") }     // Never: satisfies Int64
    return lookup(id)
}

fn half(m: Int64) -> Int64 {
    todo("half not written yet")           // panic with intent in the
}                                          // message; checks as Never

fn port(n: Int64) -> Int64 {
    assert(n > 0, "n must be positive")    // true: unit, continue;
    return n / 2                           // false: panic with the message
}

fn counted() {
    scope resource(a = openFile("a"), b = openFile("b")) {
        panic("boom")                      // unwinds: b released, then a,
    }                                      // then this function's defers,
    defer { log() }                        // then the abort — no handler
}
```

### Single mechanism

```we
let v = try parse("1")
// E0105: unexpected token — no try form; ? is the only propagation operator

let w = parse("1")!
// E0105: unexpected token — no ! postfix; the propagation postfix is ? alone
```
