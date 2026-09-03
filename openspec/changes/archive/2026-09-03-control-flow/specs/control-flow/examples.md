# Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–3. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Function-body surface arrives with the declarations chapter; defer's positive form is annotated accordingly.

### if and else

```we
let x = if c { 1 } else { 2 }     // value: the taken arm's block value
let y = if c { 1 }                 // E0202: valueless form in value position

let z = if c {
    let a = 1
    a                              // then-arm value
} else {
    0
}
// a is NOT visible here: each arm is its own scope

let grade = if n >= 90 { "high" } else if n >= 50 { "mid" } else { "low" }
// else-if is an else arm holding one nested if
```

### while, loop, break, continue

```we
while more() { step() }            // statement item
let x = loop { step() }            // E0202: loops produce no value

var found: Option<Item> = None     // accumulator is the value channel
loop {
    let it = next()
    if it.isEnd() { break }        // bare break: exits innermost loop
    if !it.matches() { continue }
    found = Some(it)
    break
}

continue                           // E0201: break or continue outside a loop
```

### return

```we
return
return a + b
```

### defer

```we
defer cleanup()                    // E0203: defer body must be a block
let x = {
    defer { log() }                // E0204: defer placement, this block
    compute()                      // is not a function body
}

// positive form (function-body surface arrives with the declarations
// chapter):
//
// fn work() {
//     defer { close(f) }          // direct function-body item, OK
//     if bad() { return }         // early exit still runs defers
//     use(f)
// }                               // at exit: defers run in reverse order
```
