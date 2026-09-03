# Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–4. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Variant and tuple patterns are annotated as pending their paired amendment.

### match as a value

```we
let x = match code {
    200 => "ok"
    _ => "other"
}
// match is an expression: x receives the taken arm's body value

let y = match code {
    200 => 1
    _ => {
        log("unexpected status")
        0
    }
}
// a block body is one rule with an expression body: blocks are expressions
```

### patterns

```we
match status {
    200 => handleOk()          // literal pattern
    404 | 410 => handleGone()  // or-pattern, no bindings: consistent
    other => handleOther(other) // binding pattern: any value, binds for the arm
    _ => handleAny()           // wildcard: any value, binds nothing
}
// arms are tried top to bottom; the first match wins, so the wildcard
// arm above is unreachable in VALUE terms — reachability rules arrive
// with the types chapter (ratified as non-goal here)

match n {
    n2 if n2 > limit => "big"  // guarded pattern; the guard sees the
    _ => "small"               // binding, and is evaluated only if the
}                              // pattern matched

match p {
    Point(0, 0) => "origin"    // E0105: variant patterns arrive with the
    _ => "elsewhere"           // sum-types paired amendment
}
```

### or-pattern binding consistency

```we
match code {
    200 | 404 => "known"       // branches bind the same names: none, OK
    a | b => "either"          // E0302: or-pattern branches must bind
    _ => "?"                   // the same names
}
```

### rejected forms

```we
let x = match code { }         // E0301: match requires at least one arm

let y = match code { 200 => 1, _ => 0 }
//                               ^ rejected under chapter 2's unexpected-token
// diagnostic: arms are newline-separated, no separator token exists
```
