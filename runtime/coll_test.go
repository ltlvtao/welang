package weruntime

import "testing"

// The Map and Set family's C harness (B2b design D2): the fifteen
// entries over the two-layer carrier — the stable handle, the table
// behind it, the linear probe with tombstones, the doubling rehash that
// swaps the data pointer in place — plus the two contracts the collector
// depends on: the descriptor's key zone keeps String key boxes alive and
// its value zone keeps traced value words alive, both proven across a
// churn that crosses the collection threshold several times. The
// identity hash makes slot arithmetic observable, so the tombstone-reuse
// and rehash-boundary sections pin exact header words. The mapOf length
// mismatch is a task failure, so it rides the scheduler harness shape
// the List carrier's panic face uses.

// collHarnessMain asserts the carrier's contracts in C, where the block
// layouts live. The reachability section runs first, while the only
// blocks in existence are the ones it names — its swept counts are exact
// then. The storm sections run last: they churn past the threshold on
// purpose and assert nothing about counts afterward.
const collHarnessMain = `#include <stdio.h>
#include <string.h>
#include "coll.h"
#include "list.h"

void *__we_alloc(long long n);
void __we_gc_boot(void);
long long __we_gc_collect(void);
void __we_root_push(void *p);
void __we_root_pop(void);

static int failures = 0;
#define CHECK(cond) do { \
    if (!(cond)) { printf("FAIL %d: %s\n", __LINE__, #cond); failures++; } \
} while (0)

/* The handle's one traced word — the data pointer at payload word 0 —
   and the data block's own descriptor word. */
#define DATA(h) (*(void **)((char *)(h) + 16))
#define MAPB(b) (*(void **)(b))

static unsigned long long nslots(void *b) {
    return ((((unsigned long long *)b)[1] & ~(unsigned long long)7) - 16) / 8;
}

static long long bit(void *b, unsigned long long i) {
    return (((unsigned long long *)MAPB(b))[i >> 6] >> (i & 63)) & 1;
}

/* The table's header words: payload word k sits at block word k+2. */
#define T_LEN(d) (((long long *)(d))[2])
#define T_CAP(d) (((long long *)(d))[3])
#define T_TOMB(d) (((long long *)(d))[4])
#define T_KDOM(d) (((long long *)(d))[5])
#define T_VTRACE(d) (((long long *)(d))[6])
#define L_TRACED(d) (((long long *)(d))[4]) /* a list data block's domain word */

/* A String key box, the T11-2 shape: bytes at +16, length at +24, and a
   null map word — nothing traces into the bytes, the box itself is the
   reference. */
static void *make_box(const char *s, long long n) {
    void *b = __we_alloc(32);
    *(void **)b = 0;
    *(const char **)((char *)b + 16) = s;
    *(long long *)((char *)b + 24) = n;
    return b;
}

static long long box_word_eq(long long w, const char *s, long long n) {
    void *b = (void *)(unsigned long long)w;
    if (*(long long *)((char *)b + 24) != n) {
        return 0;
    }
    const char *bp = *(const char **)((char *)b + 16);
    return memcmp(bp, s, (size_t)n) == 0;
}

/* A scalar list at exact capacity: two blocks, and no push ever grows. */
static void *ints(long long *v, long long n) {
    void *l = __we_list_new(n, 0);
    for (long long i = 0; i < n; i++) {
        l = __we_list_push(l, v[i]);
    }
    return l;
}

int main(void) {
    __we_gc_boot();

    /* Reachability, counted exactly while these are the only blocks in
       existence: the two constructor lists die at the first sweep — the
       map owns its contents now — while the key box and the traced value
       block live, through the table's two descriptor zones. Five blocks
       remain after that sweep; unrooted, all five go. */
    {
        void *rk = make_box("reach", 5);
        void *rvb = __we_alloc(32);
        *(long long *)((char *)rvb + 16) = 77;
        void *rks = __we_list_new(1, 1);
        rks = __we_list_push(rks, (long long)rk);
        void *rvs = __we_list_new(1, 0);
        rvs = __we_list_push(rvs, (long long)rvb);
        void *rm = __we_coll_map_of(rks, rvs, 1, 1);
        rk = 0;
        rvb = 0;
        rks = 0;
        rvs = 0;
        __we_root_push(rm);
        CHECK(__we_gc_collect() == 4); /* the four list blocks; the box and the value survived */
        /* Both zones of a capacity-8 table: key bits 6..13, value bits
           14..21, the five header words and the occ word unclaimed. */
        CHECK(MAPB(DATA(rm)) != 0);
        CHECK(nslots(DATA(rm)) == 22);
        for (unsigned long long i = 0; i < 22; i++) {
            CHECK(bit(DATA(rm), i) == (i >= 6 ? 1 : 0));
        }
        /* The handle: the constant [1] — the data pointer's one bit, the
           reserved word claimed by nothing. */
        CHECK(MAPB(rm) != 0);
        CHECK(nslots(rm) == 2);
        CHECK(bit(rm, 0) == 1);
        CHECK(bit(rm, 1) == 0);
        /* A read through a fresh equal-content box finds the entry, and
           the value word still points at the referent the zone kept. */
        void *probe = make_box("reach", 5);
        long long out[3];
        __we_coll_map_get(rm, (long long)probe, out);
        CHECK(out[0] == 1);
        CHECK(*(long long *)((char *)out[1] + 16) == 77);
        __we_root_pop();
        CHECK(__we_gc_collect() == 5); /* handle, table, box, value, probe */
    }

    /* Int-key roundtrip: put, get, overwrite, remove, and miss against
       the identity hash, where a key's home slot is the key's own low
       bits. */
    {
        long long kv[3] = {1, 2, 3};
        long long vv[3] = {10, 20, 30};
        void *m = __we_coll_map_of(ints(kv, 3), ints(vv, 3), 0, 0);
        CHECK(__we_coll_map_size(m) == 3);
        CHECK(T_CAP(DATA(m)) == 8);
        CHECK(T_KDOM(DATA(m)) == 0);
        CHECK(MAPB(DATA(m)) == 0); /* scalar through and through: no descriptor */
        long long out[3];
        __we_coll_map_get(m, 1, out);
        CHECK(out[0] == 1 && out[1] == 10 && out[2] == 0);
        __we_coll_map_get(m, 3, out);
        CHECK(out[0] == 1 && out[1] == 30);
        /* Overwrite: same live key, new value, no length change. */
        __we_coll_map_put(m, 1, 11);
        CHECK(__we_coll_map_size(m) == 3);
        __we_coll_map_get(m, 1, out);
        CHECK(out[0] == 1 && out[1] == 11);
        /* Removal answers the value it held, then absence — a None that
           zeroes a garbage-prefilled trio. */
        __we_coll_map_remove(m, 1, out);
        CHECK(out[0] == 1 && out[1] == 11);
        CHECK(T_TOMB(DATA(m)) == 1);
        out[0] = 7;
        out[1] = 7;
        out[2] = 7;
        __we_coll_map_get(m, 1, out);
        CHECK(out[0] == 0 && out[1] == 0 && out[2] == 0);
        __we_coll_map_remove(m, 1, out);
        CHECK(out[0] == 0); /* a miss removes nothing */
        CHECK(__we_coll_map_size(m) == 2);
        /* Tombstone reuse: key 9 homes at slot 1 — the slot key 1 left —
           so the insert takes the tombstone back and the count shrinks. */
        __we_coll_map_put(m, 9, 90);
        CHECK(T_TOMB(DATA(m)) == 0);
        CHECK(__we_coll_map_size(m) == 3);
        __we_coll_map_get(m, 9, out);
        CHECK(out[0] == 1 && out[1] == 90);
        __we_coll_map_get(m, 1, out);
        CHECK(out[0] == 0); /* the removed key did not come back */
    }

    /* The rehash boundary, deterministic under the identity hash: six
       entries fill a capacity-8 table to the bound's edge, the seventh
       still fits, and the eighth doubles first — the handle keeps its
       identity, the data block behind it turns over, and an alias held
       in a second local sees the new table. */
    {
        long long kv[6] = {1, 2, 3, 4, 5, 6};
        long long vv[6] = {10, 20, 30, 40, 50, 60};
        void *m = __we_coll_map_of(ints(kv, 6), ints(vv, 6), 0, 0);
        CHECK(T_CAP(DATA(m)) == 8);
        __we_coll_map_put(m, 7, 70);
        CHECK(T_CAP(DATA(m)) == 8);
        CHECK(__we_coll_map_size(m) == 7);
        void *alias = m;
        void *before = DATA(m);
        __we_coll_map_put(m, 8, 80); /* the eighth: double first */
        CHECK(m == alias);
        CHECK(DATA(m) != before);
        CHECK(DATA(alias) == DATA(m));
        CHECK(T_CAP(DATA(m)) == 16);
        CHECK(__we_coll_map_size(m) == 8);
        for (long long k = 1; k <= 8; k++) {
            long long out[3];
            __we_coll_map_get(m, k, out);
            CHECK(out[0] == 1 && out[1] == k * 10);
        }
        CHECK(MAPB(DATA(m)) == 0); /* still scalar: no descriptor after growth */
    }

    /* Delete-driven growth: tombstones count toward the bound, so a
       table emptied by seven removals doubles on its next insert — and
       the fresh table carries no tombstones at all. */
    {
        long long kv[6] = {1, 2, 3, 4, 5, 6};
        long long vv[6] = {0, 0, 0, 0, 0, 0};
        void *m = __we_coll_map_of(ints(kv, 6), ints(vv, 6), 0, 0);
        __we_coll_map_put(m, 7, 0);
        CHECK(T_CAP(DATA(m)) == 8);
        long long out[3];
        for (long long k = 1; k <= 7; k++) {
            __we_coll_map_remove(m, k, out);
            CHECK(out[0] == 1);
        }
        CHECK(__we_coll_map_size(m) == 0);
        CHECK(T_TOMB(DATA(m)) == 7);
        __we_coll_map_put(m, 42, 420);
        CHECK(T_CAP(DATA(m)) == 16);
        CHECK(T_TOMB(DATA(m)) == 0);
        CHECK(__we_coll_map_size(m) == 1);
        __we_coll_map_get(m, 42, out);
        CHECK(out[0] == 1 && out[1] == 420);
    }

    /* String keys: two equal strings are distinct boxes, and the table
       treats them as one key — hash and equality both over the content.
       Reads, overwrites, and removals through boxes minted later all
       resolve by bytes. */
    {
        void *ks = __we_list_new(2, 1);
        ks = __we_list_push(ks, (long long)make_box("aa", 2));
        ks = __we_list_push(ks, (long long)make_box("bb", 2));
        long long vv[2] = {1, 2};
        void *m = __we_coll_map_of(ks, ints(vv, 2), 1, 0);
        CHECK(T_KDOM(DATA(m)) == 1);
        CHECK(T_VTRACE(DATA(m)) == 0);
        CHECK(MAPB(DATA(m)) != 0); /* the key zone is traced: the boxes */
        /* Key zone only: bits 6..13 claimed, the value half untouched. */
        CHECK(nslots(DATA(m)) == 22);
        for (unsigned long long i = 0; i < 22; i++) {
            CHECK(bit(DATA(m), i) == ((i >= 6 && i < 14) ? 1 : 0));
        }
        long long out[3];
        __we_coll_map_get(m, (long long)make_box("aa", 2), out);
        CHECK(out[0] == 1 && out[1] == 1);
        __we_coll_map_put(m, (long long)make_box("bb", 2), 20); /* overwrite through a fresh box */
        CHECK(__we_coll_map_size(m) == 2);
        __we_coll_map_get(m, (long long)make_box("bb", 2), out);
        CHECK(out[0] == 1 && out[1] == 20);
        __we_coll_map_put(m, (long long)make_box("cc", 2), 30); /* different content: a new key */
        CHECK(__we_coll_map_size(m) == 3);
        __we_coll_map_remove(m, (long long)make_box("aa", 2), out);
        CHECK(out[0] == 1 && out[1] == 1);
        __we_coll_map_get(m, (long long)make_box("aa", 2), out);
        CHECK(out[0] == 0);
    }

    /* A value-zone-only table: identity keys untraced, values traced —
       the descriptor claims the value half and nothing else. */
    {
        long long kv[1] = {5};
        long long vv[1] = {0};
        void *m = __we_coll_map_of(ints(kv, 1), ints(vv, 1), 0, 1);
        void *vb = __we_alloc(32);
        *(long long *)((char *)vb + 16) = 55;
        __we_coll_map_put(m, 5, (long long)vb);
        CHECK(MAPB(DATA(m)) != 0);
        CHECK(nslots(DATA(m)) == 22);
        for (unsigned long long i = 0; i < 22; i++) {
            CHECK(bit(DATA(m), i) == (i >= 14 ? 1 : 0));
        }
        long long out[3];
        __we_coll_map_get(m, 5, out);
        CHECK(out[0] == 1 && *(long long *)((char *)out[1] + 16) == 55);
    }

    /* keys(): a fresh list of the live keys in table order, the domain
       carried — String-keyed maps yield traced lists, scalar maps plain
       ones — and every live key present exactly once. */
    {
        long long kv[3] = {1, 2, 3};
        long long vv[3] = {10, 20, 30};
        void *sm = __we_coll_map_of(ints(kv, 3), ints(vv, 3), 0, 0);
        void *ks = __we_coll_map_keys(sm);
        CHECK(__we_list_len(ks) == 3);
        CHECK(L_TRACED(DATA(ks)) == 0);
        for (long long k = 1; k <= 3; k++) {
            long long found = 0;
            for (long long i = 0; i < 3; i++) {
                if (__we_list_get(ks, i) == k) {
                    found++;
                }
            }
            CHECK(found == 1);
        }
        /* A String-keyed map's keys list is traced, and membership is by
           content. */
        void *sks = __we_list_new(2, 1);
        sks = __we_list_push(sks, (long long)make_box("xx", 2));
        sks = __we_list_push(sks, (long long)make_box("yy", 2));
        long long sv[2] = {7, 8};
        void *tm = __we_coll_map_of(sks, ints(sv, 2), 1, 0);
        void *tk = __we_coll_map_keys(tm);
        CHECK(__we_list_len(tk) == 2);
        CHECK(L_TRACED(DATA(tk)) == 1);
        long long seen_x = 0;
        long long seen_y = 0;
        for (long long i = 0; i < 2; i++) {
            long long w = __we_list_get(tk, i);
            seen_x += box_word_eq(w, "xx", 2);
            seen_y += box_word_eq(w, "yy", 2);
        }
        CHECK(seen_x == 1 && seen_y == 1);
        /* A map emptied before the call keys to the empty list. */
        long long ek[1] = {9};
        long long ev[1] = {90};
        void *em = __we_coll_map_of(ints(ek, 1), ints(ev, 1), 0, 0);
        long long out[3];
        __we_coll_map_remove(em, 9, out);
        void *eks = __we_coll_map_keys(em);
        CHECK(__we_list_len(eks) == 0);
    }

    /* The Set family: construction deduplicates, has answers membership,
       add of a known element is a no-op, remove answers whether it was
       there — and the same tombstone and growth rules hold, keyed by the
       element's own low bits. */
    {
        long long items[5] = {5, 3, 5, 9, 3};
        void *s = __we_coll_set_of(ints(items, 5), 0);
        CHECK(__we_coll_set_size(s) == 3);
        CHECK(T_CAP(DATA(s)) == 8);
        CHECK(T_KDOM(DATA(s)) == 0);
        CHECK(MAPB(DATA(s)) == 0);
        CHECK(__we_coll_set_has(s, 5) == 1);
        CHECK(__we_coll_set_has(s, 4) == 0);
        __we_coll_set_add(s, 4);
        CHECK(__we_coll_set_size(s) == 4);
        __we_coll_set_add(s, 5); /* a duplicate: nothing changes */
        CHECK(__we_coll_set_size(s) == 4);
        CHECK(__we_coll_set_remove(s, 3) == 1);
        CHECK(__we_coll_set_has(s, 3) == 0);
        CHECK(__we_coll_set_remove(s, 3) == 0); /* already gone */
        CHECK(__we_coll_set_size(s) == 3);
        CHECK(T_TOMB(DATA(s)) == 1);
        /* Tombstone reuse: 11 homes at slot 3, the slot 3 left. */
        __we_coll_set_add(s, 11);
        CHECK(T_TOMB(DATA(s)) == 0);
        CHECK(__we_coll_set_size(s) == 4);
        CHECK(__we_coll_set_has(s, 11) == 1);
        /* Growth: to the bound's edge, then one more doubles. */
        __we_coll_set_add(s, 1);
        __we_coll_set_add(s, 2);
        __we_coll_set_add(s, 6); /* seven live: still capacity 8 */
        CHECK(T_CAP(DATA(s)) == 8);
        __we_coll_set_add(s, 7); /* the eighth doubles first */
        CHECK(T_CAP(DATA(s)) == 16);
        CHECK(__we_coll_set_size(s) == 8);
        long long want[8] = {1, 2, 4, 5, 6, 7, 9, 11};
        for (long long i = 0; i < 8; i++) {
            CHECK(__we_coll_set_has(s, want[i]) == 1);
        }
        CHECK(__we_coll_set_has(s, 3) == 0); /* the removed one stayed out */
        /* A String set traces its slot zone: bits 5..12 of a 13-slot
           block, membership by content. */
        void *sitems = __we_list_new(1, 1);
        sitems = __we_list_push(sitems, (long long)make_box("ss", 2));
        void *ss = __we_coll_set_of(sitems, 1);
        CHECK(T_KDOM(DATA(ss)) == 1);
        CHECK(MAPB(DATA(ss)) != 0);
        CHECK(nslots(DATA(ss)) == 13);
        for (unsigned long long i = 0; i < 13; i++) {
            CHECK(bit(DATA(ss), i) == (i >= 5 ? 1 : 0));
        }
        CHECK(__we_coll_set_has(ss, (long long)make_box("ss", 2)) == 1);
        CHECK(__we_coll_set_has(ss, (long long)make_box("tt", 2)) == 0);
    }

    /* The surface List face: get answers absence for an out-of-range
       index — the carrier's own trap is the emitter's contract, not the
       surface's — and remove_at answers the element it took, the live
       tail shifting one slot left inside the carrier. */
    {
        long long lv[3] = {10, 20, 30};
        void *l = ints(lv, 3);
        long long out[3];
        out[0] = 7;
        out[1] = 7;
        out[2] = 7;
        __we_coll_list_get(l, 1, out);
        CHECK(out[0] == 1 && out[1] == 20 && out[2] == 0);
        __we_coll_list_get(l, 3, out); /* one past: absence, trio zeroed */
        CHECK(out[0] == 0 && out[1] == 0 && out[2] == 0);
        __we_coll_list_get(l, -1, out);
        CHECK(out[0] == 0);
        __we_coll_list_remove_at(l, 0, out);
        CHECK(out[0] == 1 && out[1] == 10 && out[2] == 0);
        CHECK(__we_list_len(l) == 2);
        CHECK(__we_list_get(l, 0) == 20 && __we_list_get(l, 1) == 30);
        __we_coll_list_remove_at(l, 1, out); /* [20, 30] loses its tail */
        CHECK(out[0] == 1 && out[1] == 30);
        CHECK(__we_list_len(l) == 1);
        CHECK(__we_list_get(l, 0) == 20);
        __we_coll_list_remove_at(l, 5, out); /* out of range: absence */
        CHECK(out[0] == 0);
        CHECK(__we_list_len(l) == 1);
        __we_coll_list_remove_at(l, 0, out);
        CHECK(out[0] == 1 && out[1] == 20);
        CHECK(__we_list_len(l) == 0);
        __we_coll_list_remove_at(l, 0, out);
        CHECK(out[0] == 0); /* empty: absence */
    }

    /* The storms: past the collector's threshold, a rooted map is the
       only handle left — its key boxes live through the key zone, its
       value blocks through the value zone, and both read back after the
       churn. Junk writes its own counter into payload word 0, a range
       the markers never use, so a swept-and-reused block reads wrong. */
    {
        /* Value zone: identity keys, traced values. */
        long long kv[8] = {1, 2, 3, 4, 5, 6, 7, 8};
        long long zero[8] = {0, 0, 0, 0, 0, 0, 0, 0};
        void *vm = __we_coll_map_of(ints(kv, 8), ints(zero, 8), 0, 1);
        __we_root_push(vm);
        for (long long k = 1; k <= 8; k++) {
            void *mb = __we_alloc(32);
            *(long long *)((char *)mb + 16) = 0x5AA00000LL + k;
            __we_coll_map_put(vm, k, (long long)mb);
        }
        for (long long i = 0; i < 40000; i++) { /* 2.5 MB: several collections */
            void *junk = __we_alloc(64);
            *(long long *)((char *)junk + 16) = i;
        }
        for (long long k = 1; k <= 8; k++) {
            long long out[3];
            __we_coll_map_get(vm, k, out);
            CHECK(out[0] == 1);
            CHECK(*(long long *)((char *)out[1] + 16) == 0x5AA00000LL + k);
        }
        __we_root_pop();

        /* Key zone: String keys, scalar values — the boxes themselves are
           the reachability question, read back through boxes minted after
           the churn. */
        const char *kn[4] = {"storm-alpha", "storm-beta", "storm-gamma", "storm-delta"};
        long long kl[4] = {11, 10, 11, 11};
        void *sks = __we_list_new(4, 1);
        for (long long i = 0; i < 4; i++) {
            sks = __we_list_push(sks, (long long)make_box(kn[i], kl[i]));
        }
        long long sv[4] = {401, 402, 403, 404};
        void *km = __we_coll_map_of(sks, ints(sv, 4), 1, 0);
        sks = 0;
        __we_root_push(km);
        for (long long i = 0; i < 40000; i++) {
            void *junk = __we_alloc(64);
            *(long long *)((char *)junk + 16) = i;
        }
        for (long long i = 0; i < 4; i++) {
            long long out[3];
            __we_coll_map_get(km, (long long)make_box(kn[i], kl[i]), out);
            CHECK(out[0] == 1 && out[1] == 401 + i);
        }
        /* Removal resolves through a post-churn box too. */
        long long out[3];
        __we_coll_map_remove(km, (long long)make_box(kn[1], kl[1]), out);
        CHECK(out[0] == 1 && out[1] == 402);
        __we_coll_map_get(km, (long long)make_box(kn[1], kl[1]), out);
        CHECK(out[0] == 0);
        __we_root_pop();
    }

    if (failures == 0) {
        printf("coll harness: all checks passed\n");
    }
    return failures != 0;
}
`

// collPanicHarness pins the constructor's length contract: mapOf zips
// two lists, and a mismatch is the calling task's failure through
// chapter 14's family, the trap naming the operation.
const collPanicHarness = `#include <stdio.h>
#include "coll.h"
#include "list.h"

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);

static long long boom(void *env) {
    (void)env;
    void *ks = __we_list_new(2, 0);
    ks = __we_list_push(ks, 1);
    ks = __we_list_push(ks, 2);
    void *vs = __we_list_new(1, 0);
    vs = __we_list_push(vs, 3); /* two keys, one value */
    __we_coll_map_of(ks, vs, 0, 0);
    return 0;
}

static void report(const char *what, long long (*thunk)(void *)) {
    void *h = __we_task_new(thunk, 0);
    long long v = 0;
    long long tag = __we_handle_await(h, &v);
    printf("%s tag=%lld msg=%s\n", what, tag, (const char *)v);
}

int we_main(void) {
    report("mismatch", boom);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestCollHarness(t *testing.T) {
	// The family's out trio and constructors call __we_task_fail, so the
	// harness links the scheduler and test faces alongside — the List and
	// String families' harnesses take the same shape.
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "str.h": StrHeader,
			"conc.c": ConcSource, "test.c": TestSource, "str.c": StrSource, "list.h": ListHeader,
			"list.c": ListSource, "coll.h": CollHeader, "coll.c": CollSource, "main.c": collHarnessMain,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "list.c", "coll.c", "main.c"},
		"coll harness: all checks passed\n")
}

func TestCollPanicHarness(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "str.h": StrHeader,
			"conc.c": ConcSource, "test.c": TestSource, "str.c": StrSource, "list.h": ListHeader,
			"list.c": ListSource, "coll.h": CollHeader, "coll.c": CollSource, "main.c": collPanicHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "list.c", "coll.c", "main.c"},
		"mismatch tag=1 msg=mapOf length mismatch\n")
}
