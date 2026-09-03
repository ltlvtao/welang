# Examples: declarations

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–6. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Typing-level rules are annotated as pending the types chapter.

### A module of declarations

```we
import std.io as io
import models.user

/// Retry budget shared across this module's handlers.
pub let maxRetries = 3

pub fn handle(id: Int64) -> Bool {
    let u = user.find(id)
    retry(u, maxRetries)
}

fn retry(u: user.User, budget: Int64) -> Bool {   // module-local: no pub
    io.println("retrying")
    true
}
```

### Return, defer, and the body's value

```we
pub fn loadAll(ids: List<Int64>) -> Int64 {   // final expression is the
    var total = 0                             // implicit return value
    for id in ids {
        total = total + process(id)
    }
    total
}

pub fn process(id: Int64) -> Int64 {
    defer { io.println("done") }   // legal: a direct item of the fn body
    if id < 0 {
        return 0                   // early return; defer runs at exit
    }
    transform(id)
}

fn emit(line: String) {            // no return type: produces no value
    io.println(line)               // final expression's value is
}                                  // discarded implicitly (types chapter)
```

### Rejected forms

```we
var count = 0                      // E0403: var at the top level
return 1                           // E0401: return outside a function body
fn bad(x) { }                      // E0105: parameter annotation required
fn noValue() { return 1 }          // E0402: value return in a valueless fn
pub import std.io                  // E0105: pub prefixes fn and let only
import Models.User                 // E0013: module names must be lowercase
fn io() { }                        // E0404: duplicate name in one module
                                  // (io already introduced by import)

fn f() {
    import std.io                  // E0105: import is top-level only
    return
}
```

### Pending later chapters

```we
// Module resolution (std priority, src/ root, circular-dependency
// rejection), the cross-module visibility diagnostic, and the main
// convention are the module-system chapter's:
//
// import external.package.name
// pub fn main() -> Result<(), Error> { Ok(()) }
//
// Generic parameters, effect annotations, mut parameters, and
// foreign blocks are their owning chapters':
//
// pub fn pairOf<T>(x: T) -> List<T>
// fn read(path: String) io -> String
```
