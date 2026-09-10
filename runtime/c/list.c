// The We runtime's List family (design D6): the growable element vector
// chapter 17's snapshot iteration and chapters 11/17's eager combinators
// run over. A list is one gc block — the frozen header, then three payload
// slots this family owns, then the elements — so a list reference is one
// pointer and the collector owns the storage.
//
// One element is one word: a scalar (Int64, UInt64, Float64, Bool, Rune) or
// a gc handle, the two domains the emitter's value tower already speaks. An
// element holding a gc reference is traced through the block's map
// descriptor, which this family builds at allocation — see install below.
//
// Growth (push) carves a fresh, larger block and copies: the storage moves,
// which is why push answers with the list's new identity. This collector
// never compacts, so the abandoned block stays exactly where it was and
// dies at the next sweep — no forwarding, no write barrier, nothing for the
// caller to update beyond the returned pointer.
#include "list.h"

#include <stdlib.h>

void *__we_alloc(long long n);        // gc.c: the frozen M4 allocation ABI (ADR-0002)
void __we_task_fail(const char *msg); // sched.c: the calling task's failure report

// The payload layout. Slot k of the payload sits at block word k+2 — the
// frozen header takes words 0 and 1.
#define L_LEN 0    // the element count
#define L_CAP 1    // the carved element slots
#define L_TRACED 2 // whether an element word holds a gc reference
#define L_ELEM 3   // element 0

static long long *slot(void *l, long long k) { return (long long *)l + 2 + k; }
static long long *elem(void *l, long long i) { return slot(l, L_ELEM) + i; }

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
static void install(void *l, long long traced) {
    if (!traced) {
        *(void **)l = NULL; // a scalar domain: nothing to trace through
        return;
    }
    unsigned long long sz = ((unsigned long long *)l)[1] & ~(unsigned long long)7;
    unsigned long long nslots = (sz - 16) / 8; // never below 3: a list is carved with three payload slots
    unsigned long long *desc = calloc((nslots + 63) / 64, sizeof *desc);
    if (!desc) {
        abort(); // the allocator's failure is fatal, the posture gc.c itself takes
    }
    for (unsigned long long i = L_ELEM; i < nslots; i++) {
        desc[i >> 6] |= (unsigned long long)1 << (i & 63);
    }
    *(void **)l = desc;
}

void *__we_list_new(long long cap, long long traced) {
    if (cap < 0) {
        cap = 0; // the emitter never asks for a negative capacity; the clamp keeps a later push inside the block
    }
    void *l = __we_alloc(16 + 8 * (L_ELEM + cap));
    if (!l) {
        abort(); // nothing left to fall back to, as in gc.c's carving
    }
    *slot(l, L_LEN) = 0;
    *slot(l, L_CAP) = cap;
    *slot(l, L_TRACED) = traced;
    install(l, traced);
    return l;
}

void *__we_list_push(void *l, long long v) {
    long long len = *slot(l, L_LEN);
    if (len == *slot(l, L_CAP)) {
        // Doubling keeps push amortized constant. The receiver and anything
        // its elements reference must be rooted by the caller across this
        // allocation — the family cannot know which element words are gc
        // references, so it cannot root them itself.
        long long ncap = len ? len * 2 : 4;
        void *nl = __we_list_new(ncap, *slot(l, L_TRACED));
        for (long long i = 0; i < len; i++) {
            *elem(nl, i) = *elem(l, i);
        }
        *slot(nl, L_LEN) = len;
        l = nl;
    }
    *elem(l, len) = v;
    *slot(l, L_LEN) = len + 1;
    return l;
}

long long __we_list_get(void *l, long long i) {
    if (i < 0 || i >= *slot(l, L_LEN)) {
        __we_task_fail("List element read out of range");
    }
    return *elem(l, i);
}

long long __we_list_len(void *l) { return *slot(l, L_LEN); }

void *__we_list_snap(void *l) {
    long long len = *slot(l, L_LEN);
    void *s = __we_list_new(len, *slot(l, L_TRACED));
    for (long long i = 0; i < len; i++) {
        *elem(s, i) = *elem(l, i);
    }
    *slot(s, L_LEN) = len;
    return s;
}
