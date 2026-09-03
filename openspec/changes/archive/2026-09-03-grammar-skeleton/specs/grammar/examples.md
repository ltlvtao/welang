# Examples (non-authoritative)

The examples below illustrate the Requirements above using only skeleton-ratified surface forms. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits.

### Line-joining

```we
// Semicolons are never written; statement boundaries are inferred.
let a = compute()
let b = a * 2

// A trailing operator continues onto the next line.
let total = a +
    b

// An operand at end of line ends the statement: the next line is a NEW
// statement (unary minus), not a continuation. To subtract, write `a - b`
// on one line or trail the operator.
let x = a
    - b

// Inside round and square brackets, line breaks are insignificant.
let config = build(
    host,
    port,
)

// Chaining continues with a trailing dot; a leading dot is rejected.
let y = svc.
    query()
let z = svc
    .query()   // E0102: statement begins with a continuation token
```

### Blocks and block value

```we
// A block is an expression; its final expression is its value.
let x = {
    let a = compute()
    let b = a * 2
    b + 1
}

// A final statement leaves no value; discard explicitly with `let _ =`.
let u = {
    save(record)
    let _ = load()
}

// The final expression is ALWAYS the value, even a plain call —
// discard deliberately when the value is not intended.
let v = {
    let _ = save(record)
    load()
}
```

### Statements

```we
let a = 1
var count: Int64 = 0   // type names arrive with the types chapter;
                       // the annotation form is ratified here
count = count + a      // assignment is a statement

let x = y = 1          // E0103: assignment is not an expression
f(a = 1)               // E0103: argument position is an expression
```

### Expressions and precedence

```we
let y = a.f(x).g(z)             // postfixes chain: (((a.f)(x)).g)(z)
let n = -x
let t = !flag
let m = ~bits

let q = a + b * c               // a + (b * c)
let r = a & mask == flag        // (a & mask) == flag: bitwise binds tighter
let ok = a < b < c              // E0104: chained non-associative operator
let good = (a < b) && (b < c)   // explicit grouping is the split form

let first = list[0]             // E0105: index syntax is not ratified yet;
                                // the collections chapter adds it
```
