package weruntime

import "testing"

// The List family's C harness (design D6): the carrier's five entries —
// construction at a capacity, append with the doubling growth, element
// reads, the length, and the snapshot chapter 17's iteration takes — plus
// the contracts the collector depends on: a traced list's data-block
// descriptor covers every element slot of the block that was actually
// carved, and the elements it points at survive a collection reached
// through the list. Version 2 (B2b design D1) adds the handle face: the
// value is a stable 32-byte block whose one traced word is the data
// pointer, push answers the receiver's own identity across a growth that
// swaps the data block in place, and a copy of the handle held in a
// second C local sees the growth without being told. The out-of-range
// read is a task failure, so it rides the scheduler harness shape the
// String family's panic face uses.

// listHarnessMain asserts the carrier's contracts in C, where the block
// layouts live. CHECK prints one line per failure; the exit code is the
// count's truth value (0 = all passed). The reachability section runs
// first, while the only blocks in existence are the four it names — its
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

/* The handle's data pointer: payload word 0 of the 32-byte block. */
#define DATA(l) (*(void **)((char *)(l) + 16))

/* The data block's payload slots, read as the family writes them. */
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

/* The element slot at index i sits 40 + 8*i into the DATA block: the
   header's two words, the three payload slots, then the elements. */
static long long elem_at(void *d, long long i) { return *(long long *)((char *)d + 40 + 8 * i); }

int main(void) {
    __we_gc_boot();

    /* Reachability: a rooted list keeps the objects its element slots
       point at — the data-block descriptor's element bits are what the
       mark phase walks, reached through the handle's own one bit — and a
       collect reached through the list sweeps nothing. */
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
    CHECK(*(long long *)((char *)*(void **)((char *)DATA(row) + 40) + 16) == 111);
    CHECK(*(long long *)((char *)*(void **)((char *)DATA(row) + 48) + 16) == 222);

    /* Unrooted, the handle, the data block, and both referents go: the
       root stack, not the list's own storage, is the root set. */
    __we_root_pop();
    CHECK(__we_gc_collect() == 4);

    /* An empty list: the handle's descriptor is the constant [1] — the
       data pointer at payload word 0 is its one claimed word — and the
       data block carries three payload slots, no elements, and no
       descriptor of its own: a scalar domain traces nothing. */
    void *empty = __we_list_new(0, 0);
    CHECK(empty != 0);
    CHECK(__we_list_len(empty) == 0);
    CHECK(MAP(empty) != 0);
    CHECK(nslots(empty) == 2);
    CHECK(bit(empty, 0) == 1);
    CHECK(bit(empty, 1) == 0);
    CHECK(CAP(DATA(empty)) == 0);
    CHECK(MAP(DATA(empty)) == 0);
    CHECK(nslots(DATA(empty)) == 3);

    /* Append and read: the elements in the order pushed, the length the
       count, the capacity the room carved. */
    void *xs = __we_list_new(4, 0);
    CHECK(__we_list_len(xs) == 0);
    xs = __we_list_push(xs, 10);
    xs = __we_list_push(xs, -20);
    xs = __we_list_push(xs, 9223372036854775807LL);
    CHECK(__we_list_len(xs) == 3);
    CHECK(CAP(DATA(xs)) == 4);
    CHECK(__we_list_get(xs, 0) == 10);
    CHECK(__we_list_get(xs, 1) == -20);
    CHECK(__we_list_get(xs, 2) == 9223372036854775807LL);

    /* Alias visibility: a handle copied into a second C local is the same
       value — chapter 17's two handles of one object. Growth swaps the
       data pointer inside the handle, so the copy sees the new storage
       without being told, both entries fetch the same data block, and
       push answers the receiver's own identity. */
    void *av = __we_list_new(2, 0);
    void *alias = av;
    void *before = DATA(av);
    av = __we_list_push(av, 1);
    av = __we_list_push(av, 2);
    av = __we_list_push(av, 3); /* growth: 2 -> 4 */
    CHECK(av == alias);
    CHECK(DATA(av) != before);
    CHECK(DATA(alias) == DATA(av));
    CHECK(__we_list_len(alias) == 3);
    CHECK(__we_list_get(alias, 2) == 3);
    CHECK(__we_list_get(alias, 2) == __we_list_get(av, 2));

    /* Growth: a full list doubles — the handle stays put, the data block
       behind it turns over, and the elements and the domain carry across;
       the capacity is the doubled room. */
    void *g = __we_list_new(2, 0);
    g = __we_list_push(g, 1);
    g = __we_list_push(g, 2);
    CHECK(CAP(DATA(g)) == 2);
    g = __we_list_push(g, 3);
    CHECK(CAP(DATA(g)) == 4);
    CHECK(__we_list_len(g) == 3);
    CHECK(__we_list_get(g, 0) == 1 && __we_list_get(g, 1) == 2 && __we_list_get(g, 2) == 3);
    void *gid = g;
    for (long long i = 4; i < 100; i++) {
        g = __we_list_push(g, i); /* 64 slots once len reaches 64, 128 at the last one */
    }
    CHECK(g == gid); /* every growth returned the same handle */
    CHECK(__we_list_len(g) == 99);
    CHECK(__we_list_get(g, 98) == 99); /* the element at index k is the value k+1 */
    CHECK(__we_list_get(g, 3) == 4);
    CHECK(CAP(DATA(g)) == 128);

    /* The descriptor: a traced list's bitmap covers every element slot of
       the carved data block and claims no other slot. The collector reads
       this mask, so a missed or an extra bit is a crash or a corruption. */
    void *tl = __we_list_new(3, 1);
    CHECK(MAP(DATA(tl)) != 0);
    CHECK(TRACED(DATA(tl)) == 1);
    CHECK(CAP(DATA(tl)) == 3);
    for (unsigned long long i = 0; i < nslots(DATA(tl)); i++) {
        CHECK(bit(DATA(tl), i) == (i >= 3 ? 1 : 0));
    }
    /* Growth rebuilds the mask for the larger block, same rule. */
    tl = __we_list_push(tl, 0);
    tl = __we_list_push(tl, 0);
    tl = __we_list_push(tl, 0);
    tl = __we_list_push(tl, 0); /* 3 -> 6 slots */
    CHECK(CAP(DATA(tl)) == 6);
    CHECK(MAP(DATA(tl)) != 0);
    CHECK(TRACED(DATA(tl)) == 1);
    for (unsigned long long i = 0; i < nslots(DATA(tl)); i++) {
        CHECK(bit(DATA(tl), i) == (i >= 3 ? 1 : 0));
    }
    for (long long i = 0; i < 6; i++) {
        CHECK(elem_at(DATA(tl), i) == 0);
    }

    /* A reused block is masked like a fresh one: the allocator hands an
       exact-size free block back whole, and the descriptor follows the
       block that exists — its size word — not the block's history. A
       list owns two blocks now, so the donor's data block is freed first
       and its handle second: the free list is newest-first, and the new
       handle then takes the freed handle block exactly while carve_data
       takes the freed data block exactly. */
    void *donor = __we_list_new(2, 1);
    void *ddata = DATA(donor); /* 56 bytes: the header, three slots, two elements */
    CHECK(nslots(ddata) == 5);
    __we_free(ddata);
    __we_free(donor);
    void *reuse = __we_list_new(2, 1);
    CHECK(reuse == donor);
    CHECK(DATA(reuse) == ddata);
    CHECK(nslots(DATA(reuse)) == 5);
    CHECK(CAP(DATA(reuse)) == 2);
    CHECK(MAP(DATA(reuse)) != 0);
    for (unsigned long long i = 0; i < nslots(DATA(reuse)); i++) {
        CHECK(bit(DATA(reuse), i) == (i >= 3 ? 1 : 0));
    }

    /* Snapshot: an independent handle and data block with the same
       elements in the same order. The source moving on — appended to, or
       with an element slot rewritten — is not seen by the snapshot
       (chapter 17's iteration fixes the sequence at the call). */
    void *src = __we_list_new(4, 0);
    src = __we_list_push(src, 7);
    src = __we_list_push(src, 8);
    src = __we_list_push(src, 9);
    void *snap = __we_list_snap(src);
    CHECK(snap != src);
    CHECK(DATA(snap) != DATA(src));
    CHECK(__we_list_len(snap) == 3);
    CHECK(__we_list_get(snap, 0) == 7 && __we_list_get(snap, 1) == 8 && __we_list_get(snap, 2) == 9);
    src = __we_list_push(src, 10);
    *(long long *)((char *)DATA(src) + 40) = 70;
    CHECK(__we_list_len(snap) == 3);
    CHECK(__we_list_get(snap, 0) == 7);
    CHECK(__we_list_get(src, 0) == 70);
    /* The snapshot carries the element domain: a traced source's snapshot
       is traced too, so its elements keep the receiver's reachability. */
    void *tsnap = __we_list_snap(tl);
    CHECK(MAP(DATA(tsnap)) != 0);
    CHECK(TRACED(DATA(tsnap)) == 1);
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
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "str.h": StrHeader,
			"conc.c": ConcSource, "test.c": TestSource, "str.c": StrSource, "list.h": ListHeader,
			"list.c": ListSource, "main.c": listHarnessMain,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "list.c", "main.c"},
		"list harness: all checks passed\n")
}

func TestListPanicHarness(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "str.h": StrHeader,
			"conc.c": ConcSource, "test.c": TestSource, "str.c": StrSource, "list.h": ListHeader,
			"list.c": ListSource, "main.c": listPanicHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "list.c", "main.c"},
		"past tag=1 msg=List element read out of range\nbelow tag=1 msg=List element read out of range\n")
}
