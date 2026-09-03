# Examples: types-foundations

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–7. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Composite types, protocols, and the conversion/wrapping method inventory are annotated as pending their chapters.

### Literals and defaults

```we
let n = 42            // Int64: unsuffixed integer default
let x = 3.14          // Float64: unsuffixed float default
let small = 100i32    // Int32: suffix names the type
let mask = 0xFFu8     // UInt8: base prefix and suffix combine
let flag = true       // Bool
let name = "Ada"      // String
let first = 'A'       // Rune

let typed: Float32 = 2.5f32   // annotation and literal agree
```

### No implicit conversion

```we
let a: Int64 = 1
let b: Int32 = 2
let sum = a + b               // E0501: operands of different types

let count: Int64 = 5
let size: UInt64 = 5
let bad = count + size        // E0501: signed and unsigned never mix

let s = "abc"
let c = 'a'
let cmp = s == c              // E0501: String and Rune stay separate

let n: Int32 = 42             // E0501: the literal is Int64, the
                              // annotation is Int32 — no coercion;
                              // write 42i32 or convert explicitly

// every transition is an explicit method (stdlib names, shown
// for shape only):
let wide = small.toInt64()
let bytes = s.toBytes()
```

### Overflow never wraps silently

```we
let max = 9223372036854775807
let boom = max + 1            // E0502: integer overflow (constant-folded)

let hi: Int64 = 4611686018427387904
let lo: Int64 = 4611686018427387904
let pair = hi + lo            // runtime checked trap, never a wrapped
                              // value; the trap's construct arrives with
                              // the error-mechanism chapter

let hashed = hash()
let mixed = hashed.wrappingAdd(1)   // wrapping is always explicit
```

### Type references fill the annotation slots

```we
fn parse(text: String) -> Int64 {   // named base types
    transform(text)
}

let u: user.User = user.find(1)     // module-qualified reference per
                                    // chapter 6's import names

let pair: (Int64, Int64) = ...      // E0105: tuple types arrive with the
                                    // composite-types chapter
```

### Conditions are Bool

```we
let count = 3
if count { step() }          // E0503: condition is not Bool
while ready() { poll() }     // legal: the call is Bool

match next() {
    n if n { }               // E0503: guard must be Bool
    _ { }
}
```

### Pending later chapters

```we
// Never arrives with sum types; () with composites; record/newtype
// with the ownership slice; List<T> and fn types with generics and
// the function-types slice; String's logical iteration protocol
// (for c in str yielding Rune) with the iterable-protocols chapter:
//
// record Point { x: Int64, y: Int64 }
// let ids: List<UserId> = build()
// fn forEach(items: List<Int64>) { }
```
