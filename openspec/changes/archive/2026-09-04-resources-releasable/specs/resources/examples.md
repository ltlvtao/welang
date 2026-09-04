## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–13. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Panic unwinding of a scope in flight is annotated as pending the error-mechanism chapter.

### Releasable and its obligations

```we
byres record FileHandle { fd: Int64 }    // a byres record: the category
                                         // carries the impl obligation
impl Releasable for FileHandle {
    fn release(mut self) { close(self.fd) }  // mut self, no return type
}

byres record LogFile { fd: Int64 }       // E1101: resource record must
                                         // implement Releasable — no impl
impl Releasable for User { ... }         // E1102: Releasable implemented
                                         // by a non-resource type
impl Releasable for FileHandle {
    fn release(mut self) -> Bool { ... } // E0808: signature mismatch
}
```

### The scope resource statement

```we
fn countLines(name: String) -> Int64 {
    scope resource(f = openFile(name)) { // head implements Releasable
        return f.read()                  // return pierces: releases run
    }                                    // once, before the defers of the
}                                        // enclosing function body run

fn two() {
    scope resource(a = openFile("a"), b = openFile("b")) {
        work(a)
        work(b)
    }                                    // at exit: b released, then a
}
```

### The release discipline: three channels, function-local

```we
fn copy(src: FileHandle) -> FileHandle { // the parameter is a channel: the
    scope resource(s = src) {            // signature carries the obligation
        return makeOut()                 // return: caller takes the value
    }
}

fn useAndPass(f: FileHandle) {           // parameter: obligation starts here
    log(f.read())                        // reading is ordinary use
    sink(f)                              // argument pass: sink's parameter
}                                        // declares the type; f dies here

fn leaky() {
    let f = openFile("a")                // E1104: resource binding outside
    log(f.read())                        // its release discipline — the
}                                        // path ends with no transfer

let f = openFile("a")                    // E1104 at the top level: the
                                         // module top level binds no
                                         // resources
```

### One trigger, no aliases, binding positions only

```we
fn guarded(name: String) -> Int64 {
    scope resource(f = openFile(name)) {
        if empty(name) { return 0 }      // early exit is the early release
        return f.read()                  // no f.release() anywhere: a
    }                                    // direct call is E1104
}

fn aliases(f: FileHandle) {
    let g = f                            // E1105: resource binding rebound
    var h = openFile("a")                // E1105: resource bindings are
}                                        // let-shaped

record Conn { file: FileHandle }         // E1106: resource type in a
type Wrapped = Open(FileHandle)          // composite position — the field,
newtype WrappedFile of FileHandle with Releasable  // the payload, the under-
let boxed: Box<FileHandle> = pack(f)     // lying, the generic argument,
let d = Dyn<Releasable>(f)               // the Dyn box: never boxed; the
                                         // box shares handles
```
