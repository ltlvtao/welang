### List literals and access

```we
let xs = [1, 2, 3]                     // List<Int64>: the elements agree
let ys: List<String> = []              // the annotation is the only source
                                       // of the empty literal's element type

let bare = []
// E1501: empty list literal has no expected type — write the annotation

let mixed = [1, "two"]
// E0501: operands of different types — one element type; convert
// explicitly so the elements agree

match xs.get(3) {                      // out of range is a value, not a trap
    Some(n) => put(n)
    None => put("absent")
}

let bad = xs[0]
// E0105: unexpected token — a bracketed index fits no production;
// indexing is by the named access methods
```

### Maps, sets, and the String layers

```we
fn total(m: Map<String, Int64>) -> Int64 {
    match m.get("count") {             // absence is a value
        Some(n) => n
        None => 0
    }
}

fn isAdmin(s: Set<Int64>, id: Int64) -> Bool {
    s.has(id)                          // membership is a Bool
}

let runes = "hì".runeCount()           // 2: code points
let bytes = "hì".byteLength()          // 3: UTF-8 bytes — a different number

let slice = "hello".byteSlice(0, 3)    // "hel": bytes, end exclusive
let beyond = "hello".charAt(7)
// panics: the index names storage that does not exist; range checks
// are the caller's, the panic is the boundary
```

### Snapshot iteration

```we
let it = jobs.iterator()               // the sequence is fixed here
jobs.add(newJob)                       // a mut method through the binding —
                                       // not seen by it; gc aliasing, and
let fresh = jobs.iterator()            // a fresh call sees the fresh state

for (name, count) in m {               // Map iterates (K, V) entries
    put(name)
}
```

### Combinators

```we
let out = [1, 2, 3, 4, 5]
    .iterator()
    .filter(|x| x > 2)
    .map(|x| x * 10)
    .collect()                         // List<Int64>: 30, 40, 50 — layers,
                                       // no intermediate collection

let total = [1, 2, 3]
    .iterator()
    .fold(0, |acc, x| acc + x)         // 6: eager, left to right

let biggest = [1, 2, 3]
    .iterator()
    .reduce(|a, b| if a > b { a } else { b })   // Some(3): empty gives None

let eff = names.iterator().map(|s| load(s))
// E1402: function value effect set does not match the expected type's
// — combinators take pure functions; effects are the for statement's

let each = names.iterator().forEach(|s| put(s))
// E0816: no such member on the receiver's type — forEach does not exist
```
