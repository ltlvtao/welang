# FFI — illustrative examples

The examples below use only surface forms ratified by chapters 1–19. They are illustrative, non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines annotated with a diagnostic code are rejected forms, shown with the code the compiler emits.

### A foreign module

```we
foreign "c" {
    record Socket { }                         // gc handle, minted only by crossing
    byval record Errno { }                    // value carried by copy
    byres record File { }                     // resource — pairs impl Releasable below

    fn socket() -> Socket effect io
    fn connect(sock: Socket, addr: String) -> Int64 effect io
    fn errno() -> Errno effect                // bare segment: the pure claim
    fn abs(x: Int64) -> Int64 effect
}

// foreign "go" { }                           // E1701: foreign block's ABI string is not "c"
// fn nested() { foreign "c" { } }            // E1702: foreign block outside the top level
```

### Resources cross by type

```we
foreign "c" {
    byres record File { }

    fn open(path: String) -> File effect io
    fn read(fd: File, buf: Bytes, cap: Int64) -> Int64 effect io
    fn close(fd: File) effect io
}

impl Releasable for File {
    fn release(mut self) {                    // chapter 13's obligation, met by a call
        let _ = close(self)                   // the impl body is ordinary checked code
    }
}

fn load(path: String) -> Bytes effect io {
    scope resource f = open(path)             // obligation attaches at the binding
    let buf = Bytes(4096)
    let n = read(f, buf, 4096)
    buf
}                                             // scope exit releases f exactly once
```

### Calls are ordinary calls

```we
foreign "c" {
    fn abs(x: Int64) -> Int64 effect
    fn write(fd: Int64, buf: Bytes) -> Int64 effect io
    fn exit(code: Int64) -> Never effect io
}

fn pure(x: Int64) -> Int64 { abs(x) }         // bare segment: the caller owes nothing
// fn oops(x: Int64) -> Int64 { write(1, b) } // E1401: undeclared effect at a call
fn quit(code: Int64) effect io { exit(code) } // Never declared: the call diverges
```

### Marshalling is We-side code

```we
foreign "c" {
    record CString { }                        // opaque handle — no String returns exist

    fn strlen(s: CString) -> Int64 effect
    fn memcpy(dst: Bytes, src: CString, n: Int64) effect io
}

fn textOf(s: CString) -> String {             // explicit copy, every step checked
    let n = strlen(s)
    let buf = Bytes(n)
    let _ = memcpy(buf, s, n)
    String.fromBytes(buf)                     // stdlib surface, not a crossing
}
```

### Rejected forms

```we
// foreign "c" {
//     fn dial(addr: String) -> Socket        // E1703: foreign function declaration without an effect segment
//     fn id<T>(x: T) -> T effect             // E1704: foreign function declaration with a generic clause
//     fn sum(xs: List<Int64>) -> Int64 effect io
//                                            // E1705: foreign function signature holds a type outside the crossing set
//     fn registerCallback(cb: fn(Int64) -> Int64) effect io
//                                            // E1705: foreign function signature holds a type outside the crossing set
//     fn errmsg() -> String effect io        // E1706: String or Bytes is not a foreign return type
// }
// let s = Socket { }                         // E1707: construction or update of a foreign opaque type
// let t = Socket { ... with &sock }          // E1707: construction or update of a foreign opaque type
// let f: fn(Int64) -> Int64 = abs            // E0105: unexpected token — a foreign name is not a value
```

### Pending later changes

```we
// Callbacks (fn values at the boundary) are a registered gap: effect
// attribution, runtime context, and calling convention have no ratified
// answer yet. Linkage — symbol binding, name mangling, library search —
// is the toolchain chapter's business:
//
// fn registerCallback(cb: ???) effect io     // not ratified
```
