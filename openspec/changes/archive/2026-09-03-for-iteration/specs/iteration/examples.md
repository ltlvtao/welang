# Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–5. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Iterator protocol and combinators are annotated as pending the types-chapter pairing.

### for over a range

```we
var sum = 0
for i in 0..10 {            // right-exclusive: i takes 0..9, never 10
    sum = sum + i
}
// i is NOT visible here: each iteration's binding is its own scope

for _ in 0..3 {             // wildcard name: runs the body 3 times,
    retry()                 // binds nothing
}

for i in 3..0 {             // start not below end: zero iterations
    never()
}
```

### for over an iterable expression

```we
for item in loadItems() {   // the expression is evaluated exactly once,
    handle(item)            // then its elements iterate in order
    if skip(item) { continue }
    if done(item) { break } // break/continue legal: for is a loop under
}                           // chapter 3's Break and continue

let x = for i in 0..3 { i } // E0202: valueless form in value position
for (a, b) in pairs {       // E0105: head destructuring arrives with
    use(a, b)               // tuple patterns (paired amendment)
}
```

### range as an expression

```we
let r = 0..n                // a range is an expression, not only a for
let page = offset..offset + size
// groups as offset..(offset + size): arithmetic binds tighter than `..`

let bad = a..b..c           // E0104: chained non-associative operator
let open = ..5              // E0105: both range bounds are required
```

### pending the types chapter

```we
// The iterable/iterator protocols, and methods like iterator(), are
// ratified by the types chapter's pairing; combinators additionally
// need closures, Option, and List:
//
// let out = items.iterator()
//     .filter(|x| x > 2)
//     .map(|x| x * 10)
//     .collect()
```
