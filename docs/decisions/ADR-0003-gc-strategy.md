# ADR-0003: Shadow-stack rooting with stop-the-world precise mark-sweep

- Status: Accepted
- Date: 2026-09-07
- Decided in: openspec change `stdlib-and-gc`

## Context

ADR-0002 made the self-built runtime product scope, precise reclamation included. M8 lands the first real collector, and the honest starting shape matters: M8 programs are single-threaded straight-line main bodies; the compiler owns every gc reference's lifetime (the construction protocol roots objects at allocation); nothing concurrent, generational, or moving has a surface to validate against yet. The decision needs a design that is precise from day one, freezes as little as possible into the ABI, and names the gates at which each deferred technique re-enters.

## Decision

1. **Rooting is a shadow stack.** The compiler emits `__we_root_push(ptr)` / `__we_root_pop()` at gc-reference lifetime boundaries; the runtime keeps a root stack (thread-local — global while M8 is single-threaded). Exactness holds by construction: every root is a compiler-inserted pointer to a gc object, so no conservative stack scan exists anywhere in the design.
2. **Collection is stop-the-world precise mark-sweep.** Objects carry the frozen header `{ map@0, size@8 }` with payload from offset 16. `map` points at the type's layout descriptor — a compiler-generated IR constant holding the reference bitmap, one word per 64 payload slots, bit *i* set when the word at `16 + 8*i` is a gc reference; String's double word is not a reference. The mark bit rides bit 0 of the stored map word (descriptors are at least 8-aligned), and the free-list tag rides size bit 0 (sizes are 8-aligned) — no extra header words, no stale-state bits: sweep rebuilds the free list from the chunk prefixes, so a block's freeness lives only in the rebuilt list.
3. **Allocation bumps within 1 MiB chunks, first-fit over the free list** (splitting when the remainder holds a header). Collection fires at the `__we_alloc` entry when carved bytes cross the threshold — the construction protocol makes entry the safe point: everything live is on the root stack and the in-flight block is not yet carved.
4. **The frozen ABI is `__we_alloc(i64) ptr` / `__we_free(ptr)`** — unchanged from M4's promise. The runtime writes the header's size word and returns a zeroed block; the compiler's construction code stores the map pointer. New runtime symbols this ADR records: the root pair, `__we_gc_collect()` (test and probe surface, not a We-language surface), and the descriptor convention of item 2.
5. **The construction protocol is alloc → map store → immediate root push → field stores in source order.** The outer object is rooted before any field value evaluates, so a nested allocation never races a collection with its parent unrooted.
6. **Evolution routing** (recorded here, deliberately not in M8): M9's concurrency work opens the generational + write-barrier versus concurrent-marking decision gate; LLVM stack maps (`gc "..."` attributes) are the route to drop shadow-stack push/pop overhead; a moving (compacting) collector enters through a read-barrier cost evaluation. Each route inherits the precise heap walk because descriptors and exact roots already exist.

## Consequences

- Stop-the-world is honest for M8's single-threaded programs; pause budgets stay implementation-quality goals (ADR-0002's mechanism-neutrality discipline) and become spec commitments only through a spec-layer change.
- Every construction pays root push/pop calls until stack maps land — a bounded, measurable cost, accepted for the M8 set.
- The descriptor convention and exact root set are the assets every later collector builds on; neither the frozen ABI nor the header layout needs to break on the evolution routes.
- Sweep returns the count of newly-dead blocks (blocks already on the free list re-enter uncounted) — the probe-observable semantics the survival probes pin.

## Rejected alternatives

- **Conservative stack scanning:** reads imprecise, pins false positives, and forecloses the moving route — incompatible with ADR-0002's precise-reclamation commitment.
- **Reference counting:** cycles need a tracing backup anyway, and atomic counters tax the single-threaded case for no M8 benefit.
- **LLVM stack maps now:** the right long-term rooting carrier, but it spends codegen investment M8's accepted set does not include; recorded as the evolution route instead.
- **Moving/compacting collection now:** no fragmentation pressure at M8 scale, and the read-barrier cost is unmeasured; enters only through its evaluation gate.
