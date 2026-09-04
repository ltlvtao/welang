## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–11. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Iterator combinators and the standard library's collection types are annotated as pending their chapters.

### Option and iteration

```we
let maybe = Some(3)                    // Some carries its element
let none: Option<Int64> = None         // None stands bare

let score = match maybe {              // arms agree on Int64
    Some(n) => n
    None => 0
}
```

### Iterators and iterables

```we
let it = names.iterator()              // fresh per call; the collection
let first = match it.next() {          // itself is never consumed
    Some(name) => name
    None => "anonymous"
}

impl Iterable<Int64> for IntSet {      // a manual impl binds its own
    type Iter = IntSetIter             // iterator type; the contract:
    fn iterator(self) -> IntSetIter {  // Iter implements Iterator<Int64>
        IntSetIter { set: self, pos: 0 }
    }
}
```

### for over strings and ranges

```we
for c in "hì" {                        // String: builtin Iterable<Rune>
    put(c)                             // c is Rune, code points in order
}

for i in 2..5 {                        // Range<Int64>: 2, 3, 4
    step(i)
}
```

### Rejected forms

```we
for x in 5 { }                         // E0901: for-in expression does
                                       // not implement Iterable
for b in bytes { }                     // E0901: Bytes carries no
                                       // implementation; convert first
let r = 1.5..2.5                       // E0902: range operand is not an
                                       // integer type
let m = 0i32..9i64                     // E0501: the bounds must be one
                                       // integer type
impl Iterable<Int64> for LogFile { }   // E0903: LogFile is of the
                                       // resource category
impl Iterable<Rune> for String { }     // E0811: a base-type head fits no
                                       // impl production
impl Iterable<Int64> for IntSet {
    type Iter = Int64                  // E0904: impl binds Iter to a
    fn iterator(self) -> Int64 { 0 }   // non-iterator type
}
for (a, b) in names { }                // E0501: the element type String
                                       // is not a tuple
```

### Pending later chapters

```we
// Iterator combinators (map, filter, and the lazy and eager families)
// need function values and arrive with their owning change as default
// methods of Iterator; the collection types (List, Map, Set) arrive
// with the collections chapter on this chapter's protocol:
//
// let out = names.iterator()
//     .filter(|n| n.size() > 2)
//     .map(|n| n.toUpper())
//     .collect()
```
