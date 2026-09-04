# Concurrency — illustrative examples

The examples below use only surface forms ratified by chapters 1–18. They are illustrative, non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines annotated with a diagnostic code are rejected forms, shown with the code the compiler emits. The concurrency types live in `std.concurrent` and are reached by import — `import std.concurrent as conc` below, the alias keeping the lines short; `Shareable` and `currentCancelSignal` alone are prelude-visible.

### Shared state

```we
import std.concurrent as conc

let counter = conc.Mutex(0)                  // the name carries the discipline
let cache = conc.RwLock(text)                // readers share, writers exclude
let ready = conc.Atomic(true)                // atomic base type only
let cfg = conc.AtomicRef(Config { keep: 1 }) // gc record only

let next = counter.update(|v| v + 1)         // indivisible read-modify-write
let seen = cache.read(|s| s.size())          // shared read lock, U from text
ready.set(false)

let a = conc.Mutex(0)
let b = conc.Mutex(0)
a.update(|x| x + b.get())                    // distinct bindings: legal
// a.update(|x| a.get() + x)                 // E1613: nested access to one shared value within its own callback
// let bad = conc.Atomic("name")             // E1614: Atomic type argument is not an atomic base type
// let r = conc.AtomicRef(3)                 // E1615: AtomicRef type argument is not a gc record type
```

### Conditions and semaphores

```we
import std.concurrent as conc

let queue = conc.Mutex(List<Int64>())
let notEmpty = conc.Cond(queue)              // Cond binds a Mutex, by type

let dbLimiter = conc.Semaphore(10)           // positive count, by construction
// let none = conc.Semaphore(0)              // E1616: Semaphore count is not a positive integer

scope {
    let t = task effect net {
        dbLimiter.acquire()                  // at most ten run at once
        pull()
        dbLimiter.release()
    }
    let _ = t.await()
}
```

### Task blocks and scopes

```we
import std.concurrent as conc

fn fanOut(a: Url, b: Url) effect net -> Result<(Page, Page), conc.TaskPanic> {
    scope {
        let ta = task effect net { fetch(a) }
        let tb = task effect net { fetch(b) }
        let pa = ta.await()?                 // first Err cancels tb's task
        let pb = tb.await()?
        Ok((pa, pb))
    }
}

let both = scope collectAll {                // every result wanted
    let t1 = task effect net { fetch(x) }
    let t2 = task effect net { fetch(y) }
    collectPair(t1.await()?, t2.await())     // waits for t2 even on Err
}

let page = scope timeout(500) {              // Result<Page, conc.TimeoutError>
    fetchRemote()
}
```

### Cancellation

```we
scope timeout(3000) {
    let t = task effect net {
        defer { markStopped() }              // a task body is a function body
        while !currentCancelSignal().isCancelled() {
            pullChunk()                      // cooperative: it checks, or runs
        }
        done()                               // the task's value
    }
    let _ = t.await()                        // join on every path
}
```

### Channels and select

```we
import std.concurrent as conc

let ch: conc.Channel<Int64> = conc.channel(4)  // the annotation fixes T
ch.send(1)
let first = ch.receive()                       // Some(1) while values remain
ch.close()
// ch.send(2)                                  // panics at runtime: closed

let out = ch.toSendOnly()                      // explicit, one-way
fn produce(out: conc.SendOnly<Int64>) {
    out.send(7)                                // no receive exists to call
}

select {
    case v = ch.receive() => process(v)        // v binds the Option<Int64>
    case _ = stop.awaitCancelled() => abandon()
}
```

### Rejected forms

```we
// task { fetchA() }                           // E1601: task block without an effect segment
// let xs = [1, 2, 3]
// task effect net { put(xs) }                 // E1602: task block captures unsynchronized gc state
// var total = 0
// task effect net { put(total) }              // E1603: task block captures a var binding
// impl Shareable for Conn {}                  // E1606: Shareable cannot be manually implemented
// scope {
//     task effect net { fetchA() }            // E1607: task handle reaches scope exit un-awaited, or await and cancel both fire
// }
// fn plain() { currentCancelSignal() }        // E1608: currentCancelSignal called outside a task block
// select {
//     case n = xs.get(0) => n                 // E1609: select source is not a wait source
//     case _ = ch.receive() => 0
// }
// select {
//     case a = ch.receive() => a              // arms Option<Int64> and Option<String>
//     case s = names.receive() => s           // E1610: select arms disagree in type
// }
// select {
//     case Some(v) = ch.receive() => v        // E1611: select case pattern is not a binding or the wildcard
//     case _ = t.await() => 0
// }
// select {
//     case v = ch.receive() => v              // E1612: select holds fewer than two cases
// }
// let bare = conc.channel(4)                  // E1617: channel construction without an expected type
// fn top() {
//     task effect net { fetchA() }            // E1618: task block outside any scope block
// }
```

### Pending later changes

```we
// The multi-lock atomic composite (v0.8's Shared.transaction) is a
// registered gap: transfer-shaped work goes through per-value update
// calls under a conventional lock ordering for now. WeakRef and memory
// regions await their own chapters, and the cross-function nested-access
// survey and the may-block note await the tooling chapter's vet layer:
//
// transaction([a, b], |(x, y)| x - 1; y + 1)   // not ratified
```
