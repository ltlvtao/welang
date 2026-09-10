package weruntime

import "testing"

// The List family's C harness (design D6): the carrier's five entries —
// construction at a capacity, append with the doubling growth, element
// reads, the length, and the snapshot chapter 17's iteration takes — plus
// the two contracts the collector depends on: a traced list's map
// descriptor covers every element slot of the block that was actually
// carved, and the elements it points at survive a collection reached
// through the list. The out-of-range read is a task failure, so it rides
// the scheduler harness shape the String family's panic face uses.

// listHarnessMain asserts the carrier's contracts in C, where the block
// layout lives. CHECK prints one line per failure; the exit code is the
// count's truth value (0 = all passed). The reachability section runs
// first, while the only blocks in existence are the three it names — its
// swept counts are exact then, and later sections allocate freely without
// a collection in sight.
const listHarnessMain = `#include <stdio.h>
#include "list.h"

void *__we_alloc(long long n);
void __we_free(void *p);
void __we_gc_boot(void);
long long __we_gc_collect(void);
void __we_root_push(void *p);
void __we_root_pop(void);

static int failures = 0;
#define CHECK(cond) do { \
    if (!(cond)) { printf("FAIL %d: %s\n", __LINE__, #cond); failures++; } \
} while (0)

/* The carrier's payload slots, read as the family writes them. */
#define LEN(l) (((long long *)(l))[2])
#define CAP(l) (((long long *)(l))[3])
#define TRACED(l) (((long long *)(l))[4])
#define MAP(l) (*(void **)(l))

/* The descriptor's word count as the collector derives it: from the size
   word the allocator wrote, not from what the family asked for. */
static unsigned long long nslots(void *l) {
    return ((((unsigned long long *)l)[1] & ~(unsigned long long)7) - 16) / 8;
}

static long long bit(void *l, unsigned long long i) {
    return (((unsigned long long *)MAP(l))[i >> 6] >> (i & 63)) & 1;
}

/* The element slot at index i sits 40 + 8*i into the block: the header's
   two words, the three payload slots, then the elements. */
static long long elem_at(void *l, long long i) { return *(long long *)((char *)l + 40 + 8 * i); }

int main(void) {
    __we_gc_boot();

    /* Reachability: a rooted list keeps the objects its element slots
       point at — the descriptor's element bits are what the mark phase
       walks — and a collect reached through the list sweeps nothing. */
    void *a = __we_alloc(32);
    void *b = __we_alloc(32);
    *(long long *)((char *)a + 16) = 111;
    *(long long *)((char *)b + 16) = 222;
    void *row = __we_list_new(2, 1);
    row = __we_list_push(row, (long long)a);
    row = __we_list_push(row, (long long)b);
    __we_root_push(row);
    a = 0;
    b = 0; /* the list is the only handle left */
    CHECK(__we_gc_collect() == 0);
    CHECK(*(long long *)((char *)*(void **)((char *)row + 40) + 16) == 111);
    CHECK(*(long long *)((char *)*(void **)((char *)row + 48) + 16) == 222);

    /* Unrooted, the list and both referents go: the root stack, not the
       list's own storage, is the root set. */
    __we_root_pop();
    CHECK(__we_gc_collect() == 3);

    /* An empty list: three payload slots present, no elements, no
       descriptor — a scalar domain traces nothing. */
    void *empty = __we_list_new(0, 0);
    CHECK(empty != 0);
    CHECK(__we_list_len(empty) == 0);
    CHECK(CAP(empty) == 0);
    CHECK(MAP(empty) == 0);

    /* Append and read: the elements in the order pushed, the length the
       count, the capacity the room carved. */
    void *xs = __we_list_new(4, 0);
    CHECK(__we_list_len(xs) == 0);
    xs = __we_list_push(xs, 10);
    xs = __we_list_push(xs, -20);
    xs = __we_list_push(xs, 9223372036854775807LL);
    CHECK(__we_list_len(xs) == 3);
    CHECK(CAP(xs) == 4);
    CHECK(__we_list_get(xs, 0) == 10);
    CHECK(__we_list_get(xs, 1) == -20);
    CHECK(__we_list_get(xs, 2) == 9223372036854775807LL);

    /* Growth: a full list doubles, moves, and carries its elements and
       its domain across; the capacity is the doubled room. */
    void *g = __we_list_new(2, 0);
    g = __we_list_push(g, 1);
    g = __we_list_push(g, 2);
    CHECK(CAP(g) == 2);
    g = __we_list_push(g, 3);
    CHECK(CAP(g) == 4);
    CHECK(__we_list_len(g) == 3);
    CHECK(__we_list_get(g, 0) == 1 && __we_list_get(g, 1) == 2 && __we_list_get(g, 2) == 3);
    for (long long i = 4; i < 100; i++) {
        g = __we_list_push(g, i); /* 64 slots once len reaches 64, 128 at the last one */
    }
    CHECK(__we_list_len(g) == 99);
    CHECK(__we_list_get(g, 98) == 99); /* the element at index k is the value k+1 */
    CHECK(__we_list_get(g, 3) == 4);
    CHECK(CAP(g) == 128);

    /* The descriptor: a traced list's bitmap covers every element slot of
       the carved block and claims no other slot. The collector reads this
       mask, so a missed or an extra bit is a crash or a corruption. */
    void *tl = __we_list_new(3, 1);
    CHECK(MAP(tl) != 0);
    CHECK(TRACED(tl) == 1);
    CHECK(CAP(tl) == 3);
    for (unsigned long long i = 0; i < nslots(tl); i++) {
        CHECK(bit(tl, i) == (i >= 3 ? 1 : 0));
    }
    /* Growth rebuilds the mask for the larger block, same rule. */
    tl = __we_list_push(tl, 0);
    tl = __we_list_push(tl, 0);
    tl = __we_list_push(tl, 0);
    tl = __we_list_push(tl, 0); /* 3 -> 6 slots */
    CHECK(CAP(tl) == 6);
    CHECK(MAP(tl) != 0);
    CHECK(TRACED(tl) == 1);
    for (unsigned long long i = 0; i < nslots(tl); i++) {
        CHECK(bit(tl, i) == (i >= 3 ? 1 : 0));
    }
    for (long long i = 0; i < 6; i++) {
        CHECK(elem_at(tl, i) == 0);
    }

    /* A reused block is masked like a fresh one: the allocator hands an
       exact-size free block back whole, and the descriptor follows the
       block that exists — its size word — not the block's history. */
    void *donor = __we_list_new(2, 1); /* 56 bytes: the header, three slots, two elements */
    CHECK(nslots(donor) == 5);
    __we_free(donor);
    void *reuse = __we_list_new(2, 1);
    CHECK(reuse == donor);
    CHECK(nslots(reuse) == 5);
    CHECK(CAP(reuse) == 2);
    CHECK(MAP(reuse) != 0);
    for (unsigned long long i = 0; i < nslots(reuse); i++) {
        CHECK(bit(reuse, i) == (i >= 3 ? 1 : 0));
    }

    /* Snapshot: an independent block with the same elements in the same
       order. The source moving on — appended to, or with an element slot
       rewritten — is not seen by the snapshot (chapter 17's iteration
       fixes the sequence at the call). */
    void *src = __we_list_new(4, 0);
    src = __we_list_push(src, 7);
    src = __we_list_push(src, 8);
    src = __we_list_push(src, 9);
    void *snap = __we_list_snap(src);
    CHECK(snap != src);
    CHECK(__we_list_len(snap) == 3);
    CHECK(__we_list_get(snap, 0) == 7 && __we_list_get(snap, 1) == 8 && __we_list_get(snap, 2) == 9);
    src = __we_list_push(src, 10);
    *(long long *)((char *)src + 40) = 70;
    CHECK(__we_list_len(snap) == 3);
    CHECK(__we_list_get(snap, 0) == 7);
    CHECK(__we_list_get(src, 0) == 70);
    /* The snapshot carries the element domain: a traced source's snapshot
       is traced too, so its elements keep the receiver's reachability. */
    void *tsnap = __we_list_snap(tl);
    CHECK(MAP(tsnap) != 0);
    CHECK(TRACED(tsnap) == 1);
    CHECK(__we_list_len(tsnap) == 4);
    /* And an empty source snapshots empty, not off the end. */
    void *esnap = __we_list_snap(empty);
    CHECK(__we_list_len(esnap) == 0);

    if (failures == 0) {
        printf("list harness: all checks passed\n");
    }
    return failures != 0;
}
`

// listPanicHarness pins the carrier's out-of-range read: this accessor is
// the emitter's own, so an index outside the list is a compiler bug and
// stops the calling task through chapter 14's family, the message naming
// the operation. The We-level `List.get` chapter 17 ratifies answers
// Option and never traps — it is a different surface.
const listPanicHarness = `#include <stdio.h>
#include "list.h"

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);

static long long boom_past(void *env) {
    (void)env;
    void *l = __we_list_new(2, 0);
    l = __we_list_push(l, 7);
    l = __we_list_push(l, 8);
    __we_list_get(l, 2); /* one past the end */
    return 0;
}

static long long boom_below(void *env) {
    (void)env;
    void *l = __we_list_new(2, 0);
    l = __we_list_push(l, 7);
    __we_list_get(l, -1);
    return 0;
}

static void report(const char *what, long long (*thunk)(void *)) {
    void *h = __we_task_new(thunk, 0);
    long long v = 0;
    long long tag = __we_handle_await(h, &v);
    printf("%s tag=%lld msg=%s\n", what, tag, (const char *)v);
}

int we_main(void) {
    report("past", boom_past);
    report("below", boom_below);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestListHarness(t *testing.T) {
	// The carrier's out-of-range face calls __we_task_fail, so the happy
	// harness links the scheduler and test faces alongside — the String
	// family's harness takes the same shape.
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"conc.c": ConcSource, "test.c": TestSource, "list.h": ListHeader,
			"list.c": ListSource, "main.c": listHarnessMain,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "list.c", "main.c"},
		"list harness: all checks passed\n")
}

func TestListPanicHarness(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"conc.c": ConcSource, "test.c": TestSource, "list.h": ListHeader,
			"list.c": ListSource, "main.c": listPanicHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "list.c", "main.c"},
		"past tag=1 msg=List element read out of range\nbelow tag=1 msg=List element read out of range\n")
}
