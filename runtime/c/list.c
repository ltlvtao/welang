// The We runtime's List family (design D6): the growable element vector
// chapter 17's snapshot iteration and chapters 11/17's eager combinators
// run over. Version 2 (B2b design D1): a list value is a stable 32-byte
// handle — the frozen header every block carries, the data pointer at
// payload word 0, and one word the allocator leaves zero — and the
// elements live in the data block it points at. Chapter 17's gc semantics
// make two bindings over one list two handles of a single object, so the
// value's identity must not change when it grows: push carves a larger
// data block, copies, and swaps the data pointer in place, and every
// handle — the caller's, and every copy an alias holds — sees the new
// storage. The five entry signatures are unchanged, and no emitter or C
// consumer reaches past them into either block.
//
// One element is one word: a scalar (Int64, UInt64, Float64, Bool, Rune) or
// a gc handle, the two domains the emitter's value tower already speaks. An
// element holding a gc reference is traced through the data block's map
// descriptor, which this family builds at allocation — see install below.
// The handle's own descriptor is the constant bitmap [1]: gc.c marks
// payload word 16+8i with bit i, and the handle's only traced word is the
// data pointer at payload word 0.
//
// Rooting: __we_alloc may collect before it carves, and the shadow stack
// sees only pushed roots, never C locals. __we_list_new bridges its two
// carves by rooting the handle until its data pointer is wired; across
// push's growth carve the receiver is the caller's to root, the standing
// contract — a rooted handle keeps the old data block, and through its
// descriptor every element, alive across the carve and the copy. This
// collector never compacts, so the abandoned block stays exactly where it
// was and dies at the next sweep — no forwarding, no write barrier, and
// nothing for any handle to update.
#include "list.h"

#include <stdlib.h>

void *__we_alloc(long long n);        // gc.c: the frozen M4 allocation ABI (ADR-0002)
void __we_task_fail(const char *msg); // sched.c: the calling task's failure report
void __we_root_push(void *p);         // gc.c: the shadow stack, bridging the carve below
void __we_root_pop(void);

// The data block's payload layout — the layout this family has always had.
// Slot k of the payload sits at block word k+2: the frozen header takes
// words 0 and 1.
#define L_LEN 0    // the element count
#define L_CAP 1    // the carved element slots
#define L_TRACED 2 // whether an element word holds a gc reference
#define L_ELEM 3   // element 0

// The handle's descriptor: the constant bitmap [1], one word, one bit. The
// data pointer is the handle's only traced payload word (gc.c's bit i
// marks word 16+8i, and the pointer sits at word 16). A static array
// serves — the collector only ever reads a descriptor, and descriptors
// are never reclaimed, the same lifetime install's malloc'd bitmaps take.
static unsigned long long handle_desc[1] = {1};

// slot and elem address the DATA block's payload: slot k at block word
// k+2, element i at slot L_ELEM + i. data_of is the handle's one hop.
static long long *slot(void *d, long long k) { return (long long *)d + 2 + k; }
static long long *elem(void *d, long long i) { return slot(d, L_ELEM) + i; }
static void *data_of(void *l) { return *(void **)((char *)l + 16); }

// install sets the block's map word to the layout descriptor the collector
// traces through. A capacity is a runtime value, so no compiler-emitted
// static descriptor can cover a list; this family builds one instead. Three
// details are load bearing:
//
//   - The bitmap's word count is derived from the block's size word — the
//     very word the collector reads its payload slot count from — rather
//     than from the requested capacity. The two agree today (the allocator
//     writes the rounded request), and taking the count from the collector's
//     own source is what keeps them from drifting apart: a mask shorter than
//     the scanned payload leaves the collector reading past its end, while a
//     longer one is merely wasted.
//   - Every element slot's bit is set at allocation — the whole capacity,
//     not the live prefix. A fresh block's payload is zeroed and the
//     collector skips a null child, so bits over unused slots are harmless,
//     and push then never has to touch the descriptor.
//   - A list of scalars gets no descriptor at all: the map word stays null
//     and the mark phase skips the block's contents outright.
//
// Descriptors are malloc'd and never reclaimed. A block's death is the
// sweep's to discover and it calls no hook this family could free a
// descriptor from, so one descriptor per traced list allocation is the
// price. It is bounded by the program's own list-building volume and its
// reclamation belongs with the allocator work ADR-0003 gates (B2) — the
// posture String's buffers already take (str.c). Growth rebuilds the
// descriptor for the new capacity and abandons the old one; B1a's emitter
// carves each literal at its exact element count and push never grows, so
// that path is the harness's and later tasks' to exercise.
static void install(void *d, long long traced) {
    if (!traced) {
        *(void **)d = NULL; // a scalar domain: nothing to trace through
        return;
    }
    unsigned long long sz = ((unsigned long long *)d)[1] & ~(unsigned long long)7;
    unsigned long long nslots = (sz - 16) / 8; // never below 3: a data block is carved with three payload slots
    unsigned long long *desc = calloc((nslots + 63) / 64, sizeof *desc);
    if (!desc) {
        abort(); // the allocator's failure is fatal, the posture gc.c itself takes
    }
    for (unsigned long long i = L_ELEM; i < nslots; i++) {
        desc[i >> 6] |= (unsigned long long)1 << (i & 63);
    }
    *(void **)d = desc;
}

// carve_data allocates and initializes a data block with room for cap
// elements, descriptor included — the shape __we_list_new opens and push's
// growth branch rebuilds.
static void *carve_data(long long cap, long long traced) {
    void *d = __we_alloc(16 + 8 * (L_ELEM + cap));
    if (!d) {
        abort(); // nothing left to fall back to, as in gc.c's carving
    }
    *slot(d, L_LEN) = 0;
    *slot(d, L_CAP) = cap;
    *slot(d, L_TRACED) = traced;
    install(d, traced);
    return d;
}

void *__we_list_new(long long cap, long long traced) {
    if (cap < 0) {
        cap = 0; // the emitter never asks for a negative capacity; the clamp keeps a later push inside the block
    }
    void *h = __we_alloc(32);
    if (!h) {
        abort(); // nothing left to fall back to, as in gc.c's carving
    }
    *(void **)h = handle_desc;
    // The data carve below can fire a collection, and until the data
    // pointer is wired the handle is reachable only from this C local —
    // invisible to the shadow stack. The bridge roots it across the gap.
    // An unwired data word is a marked zero the collector already
    // tolerates: a data block's unused capacity slots are that shape.
    __we_root_push(h);
    void *d = carve_data(cap, traced);
    *(void **)((char *)h + 16) = d;
    __we_root_pop();
    return h;
}

void *__we_list_push(void *l, long long v) {
    void *d = data_of(l);
    long long len = *slot(d, L_LEN);
    if (len == *slot(d, L_CAP)) {
        // Doubling keeps push amortized constant. Growth swaps the data
        // pointer in place inside the handle, so the answer is the
        // receiver's own identity and every alias sees the new storage.
        // The receiver and anything its elements reference must be rooted
        // by the caller across this allocation — the family cannot know
        // which element words are gc references, so it cannot root them
        // itself. The new block needs no bridge of its own: it holds no
        // aliases before the swap, and the old one is reached through the
        // handle until the sweep takes it.
        long long ncap = len ? len * 2 : 4;
        void *nd = carve_data(ncap, *slot(d, L_TRACED));
        for (long long i = 0; i < len; i++) {
            *elem(nd, i) = *elem(d, i);
        }
        *slot(nd, L_LEN) = len;
        *(void **)((char *)l + 16) = nd;
        d = nd;
    }
    *elem(d, len) = v;
    *slot(d, L_LEN) = len + 1;
    return l;
}

long long __we_list_get(void *l, long long i) {
    void *d = data_of(l);
    if (i < 0 || i >= *slot(d, L_LEN)) {
        __we_task_fail("List element read out of range");
    }
    return *elem(d, i);
}

long long __we_list_len(void *l) { return *slot(data_of(l), L_LEN); }

void *__we_list_snap(void *l) {
    void *d = data_of(l);
    long long len = *slot(d, L_LEN);
    void *s = __we_list_new(len, *slot(d, L_TRACED));
    void *sd = data_of(s);
    for (long long i = 0; i < len; i++) {
        *elem(sd, i) = *elem(d, i);
    }
    *slot(sd, L_LEN) = len;
    return s;
}
