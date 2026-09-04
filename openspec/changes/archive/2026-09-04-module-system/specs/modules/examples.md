# Chapter 15 examples (non-authoritative)

The examples below illustrate the Requirements of the modules chapter using only surface forms ratified by chapters 1–15. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits.

### Paths and resolution

```we
import models.user                    // resolves to src/models/user.we
import std.io                         // compiler-provided; src/std/ is never
                                      // consulted, the std segment reserved
import external.package.name          // dependency cache, same mapping

import models.missing
// E1302: module not found — expected src/models/missing.we

// a.we: import b    b.we: import a
// E1301: circular module dependency — extract shared code into a third module
```

### The prelude and shadowing

```we
fn run(s: String) -> Result<Int64, AppError> {
    let n = parse(s)?                // String, Result, Ok: prelude, no import
    if n < 0 { panic("negative") }   // panic: prelude, chapter 14's
    return Ok(n)
}

fn assert(cond: Bool) { }            // legal: the module's own assert
                                     // shadows the prelude's within it

fn write(s: String) {
    io.print(s)
    // E1304: unresolved name — io is not an import name; import std.io
}
```

### Visibility across modules

```we
// b.we
pub fn create() -> User { ... }
fn helper() { ... }
record Config { ... }

// a.we
import b

let u = b.create()                   // pub: reached through the import name
let h = b.helper()
// E1303: cross-module use of a module-local item — helper is not pub

let cfg: b.Config = make()
// E1303: cross-module use of a module-local item — type positions too
```

### Name resolution

```we
let x = compute()
// E1304: unresolved name — no declaration, import, or prelude name holds it

import models.user
let y = models.user.create()
// E1304: unresolved name — the qualifier is the import name, user

fn size(c: Config) -> Int64 { ... }
// E1304: unresolved name — no type Config in any scope
```

### Initialization and main

```we
// src/models/config.we
pub let retries = { log("config init"); 3 }   // runs once, before main

// src/main.we
import models.config

pub fn main() -> Result<(), AppError> {
    return Ok(())                   // config initialized first; exit 0
}

pub fn main() -> Int64 { 0 }
// E1305: main function signature violation — the shape is Result<(), E>
```
