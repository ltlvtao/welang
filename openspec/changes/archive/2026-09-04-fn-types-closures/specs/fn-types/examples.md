## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–12. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Iterator combinators, generic function instantiation, and method values are annotated as pending their owning changes.

### Function types and function values

```we
let f: fn(Int64) -> Int64 = square       // a monomorphic fn name is a
let nine = f(3)                          // value; the call is postfix

fn apply(f: fn(Int64) -> Int64, x: Int64) -> Int64 {
    f(x)                                 // the parameter's call
}

let make = fn() -> Bool { ready() }      // zero-parameter, valueless
let side = fn(s: String) { put(s) }      // no return type: fn(String) -> ()
```

### Closures

```we
let inc = |x: Int64| x + 1               // annotated: self-contained
let f: fn(Int64) -> Int64 = |x| x * 2    // bare: types from the annotation
let out = apply(|x| x + 1, 3)            // bare: types from the parameter

let get = fn(m: Option<Int64>) -> Int64 {
    return match m {                     // return exits the closure
        Some(n) => n
        None => 0
    }
}

var c = Counter { n: 0 }                 // gc var: captured live
let bump = fn() { c = Counter { n: c.n + 1 } }  // zero-parameter: the
bump()                                   // full form; there is no || form
```

### Captures answer the ownership categories

```we
let text = "fixed"                       // base type: copy at creation
let show = fn() { put(text) }            // zero-parameter: full form only

var p = Point { x: 1, y: 2 }             // byval: snapshot at creation
let read = fn() -> Int64 { p.x }
p = Point { x: 9, y: 9 }
// read() still sees 1: the capture is the creation-time copy
```

### Rejected forms

```we
let g = |x| x + 1                        // E1001: bare-parameter closure
                                        // without an expected function type
let h = pairUp                           // E1004: generic function name
                                        // in value position
let z = || ready()                       // E0105: unexpected token; ||
                                        // is logical-or, a zero-parameter
                                        // closure is fn() -> ...
let m = user.describe                    // E0105: unexpected token; a
                                        // method value is not ratified
let bad = fn(Int64)                      // E0105: unexpected token; the
                                        // arrow and return type are
                                        // required
let eff: fn(Int64) io -> Int64 = f       // E0105: unexpected token; no
                                        // effect segment is ratified
let one = 1 + |x| x                      // E0105: unexpected token; write
                                        // 1 + (|x| x)
|x| x + 1                                // E0102: statement begins with a
                                        // continuation token

fn over(log: LogFile) -> fn() -> Int64 { // E1002: resource binding
    fn() -> Int64 { log.lines() }        // captured by a closure
}

var n = 0
let step = fn() { n = n + 1 }            // E1003: assignment to a captured
                                        // value-category binding
let g2 = Circle                          // E0704: payloaded variant
                                        // constructor used without
                                        // arguments
```

### Pending later changes

```we
// Iterator combinators arrive with their owning change as default
// methods of Iterator; generic instantiation `map<Int64>` and generic
// methods with chapter 10's own amendment; method values, if ever,
// with this chapter's amendment:
//
// let out = names.iterator()
//     .filter(|n| n.size() > 2)
//     .map(|n| n.toUpper())
//     .collect()
```
