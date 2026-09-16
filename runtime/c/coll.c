// The We runtime's Map and Set carriers (B2b design D2): open-addressed
// hash tables with linear probing, tombstone deletion, and doubling
// rehash — the real data structures behind the collections surface. A
// map or set value is a stable 32-byte handle (list.h's posture: the
// frozen header, the data pointer at payload word 0, one reserved word,
// the constant descriptor [1]) and the table lives in the data block it
// points at. Growth doubles the table and swaps the data pointer in
// place, so every alias sees it and the handle's identity never changes.
//
// The key domain is the Eq domain: identity words (the eight integer
// widths and Bool) or String boxes hashed by FNV-1a over content and
// compared by content — two equal strings are distinct boxes, so the
// bytes decide. Domain labels ride the data block itself (KDOM, VTRACE),
// which keeps every entry self-describing: no caller passes a domain at
// read time. Values and Set elements are one word each, the List element
// domain.
//
// Tracing is two-zone, from the data block's runtime descriptor: the key
// zone is traced exactly when the keys are String boxes (an identity key
// is a scalar word and never a reference — but a String key is a box and
// always is), and the value zone exactly when VTRACE says so. Descriptors
// are malloc'd at construction and rehash and never reclaimed, the
// posture list.c's install takes.
//
// Rooting follows fs.c's discipline: an entry roots the receiver it may
// grow across, the constructors root the lists they read, and put's
// growth branch roots its own traced argument words — words not yet in
// any table, alive only through the caller's own discipline, given one
// round of depth here.
#include "coll.h"
#include "list.h"

#include <stdint.h>
#include <stdlib.h>
#include <string.h>

void *__we_alloc(long long n);        // gc.c: the frozen M4 allocation ABI (ADR-0002)
void __we_task_fail(const char *msg); // sched.c: the calling task's failure report
void __we_root_push(void *p);         // gc.c: the shadow stack
void __we_root_pop(void);

// The data block's payload words. A Map carries five header words, a Set
// four; the occupancy bytes follow, one per slot, padded to a word; the
// key words and (a Map) the value words follow that. Slot counts are
// powers of two, so the padding is the cap's own eighth.
#define T_LEN 0    // the live entry count
#define T_CAP 1    // the table capacity, a power of two >= 8
#define T_TOMB 2   // the tombstone count
#define T_KDOM 3   // 0 = identity keys, 1 = String-box keys
#define T_VTRACE 4 // whether a value word holds a gc reference (Map only)
#define MAP_WORDS 5
#define SET_WORDS 4

// The handle's descriptor: the constant bitmap [1] — the data pointer at
// payload word 0 is the handle's one traced word (list.c's own [1], the
// same fact in this family's file).
static unsigned long long handle_desc[1] = {1};

// The load bound: a table doubles when its live and dead together pass
// three quarters of the capacity — tombstones included, because a probe's
// cost walks both. The bound keeps one empty slot in the table always
// (3/4 * 8 + 1 = 7 = cap - 1 at the smallest), which is what a lookup's
// miss-stop needs to terminate.
static long long crowded(long long len, long long tomb, long long cap) {
    return (len + tomb) * 4 > cap * 3;
}

// A String key box: bytes at +16, length at +24, the map word null.
static const char *box_bytes(void *box) { return *(const char **)((char *)box + 16); }
static long long box_len(void *box) { return *(long long *)((char *)box + 24); }

// FNV-1a over the bytes — the content, never the box's address.
static unsigned long long fnv1a(const unsigned char *p, long long n) {
    unsigned long long h = 14695981039346656037ULL;
    for (long long i = 0; i < n; i++) {
        h ^= p[i];
        h *= 1099511628211ULL;
    }
    return h;
}

static unsigned long long key_hash(long long kdom, long long k) {
    if (kdom == 0) {
        return (unsigned long long)k; // identity: the word itself; narrow widths arrive extended, bijective
    }
    void *box = (void *)(uintptr_t)k;
    return fnv1a((const unsigned char *)box_bytes(box), box_len(box));
}

static long long key_eq(long long kdom, long long a, long long b) {
    if (kdom == 0) {
        return a == b;
    }
    void *ba = (void *)(uintptr_t)a;
    void *bb = (void *)(uintptr_t)b;
    long long na = box_len(ba);
    return na == box_len(bb) && memcmp(box_bytes(ba), box_bytes(bb), (size_t)na) == 0;
}

// The shared-core view: one small struct both families' entries operate
// on, so hash, probe, insert, and rehash live once.
typedef struct {
    long long *len;
    long long *cap;
    long long *tomb;
    long long *kdom;
    long long *vtrace; // NULL for a Set
    unsigned char *occ;
    long long *keys;
    long long *vals; // NULL for a Set
} tbl;

static void *data_of(void *l) { return *(void **)((char *)l + 16); }

// tbl_of binds the view to a data block; hwords says Map or Set. The
// occupancy region is cap bytes padded to a word — cap being a power of
// two >= 8, the padding is cap/8 words.
static void tbl_of(void *d, long long hwords, tbl *t) {
    long long *w = (long long *)d + 2; // payload word 0
    long long cap = w[T_CAP];
    unsigned char *occ = (unsigned char *)(w + hwords);
    t->len = &w[T_LEN];
    t->cap = &w[T_CAP];
    t->tomb = &w[T_TOMB];
    t->kdom = &w[T_KDOM];
    t->vtrace = hwords == MAP_WORDS ? &w[T_VTRACE] : 0;
    t->occ = occ;
    t->keys = (long long *)(occ + ((cap + 7) & ~7LL));
    t->vals = hwords == MAP_WORDS ? t->keys + cap : 0;
}

// install_desc builds the two-zone descriptor the collector traces
// through, list.c's install posture: the bitmap's word count derives
// from the block's size word — the collector's own source — every bit
// of both zones is set over the whole capacity (an unused slot holds a
// zero the collector skips), and a table with nothing to trace gets no
// descriptor at all.
static void install_desc(void *d, long long hwords, long long cap, long long vtrace) {
    long long trace_keys = 0;
    tbl probe;
    tbl_of(d, hwords, &probe);
    if (*probe.kdom == 1) {
        trace_keys = 1; // String keys are boxes; identity keys never trace
    }
    long long trace_vals = probe.vals && vtrace;
    if (!trace_keys && !trace_vals) {
        *(void **)d = NULL; // scalar through and through: the mark phase skips the contents
        return;
    }
    unsigned long long sz = ((unsigned long long *)d)[1] & ~(unsigned long long)7;
    unsigned long long nslots = (sz - 16) / 8;
    unsigned long long *desc = calloc((nslots + 63) / 64, sizeof *desc);
    if (!desc) {
        abort(); // the allocator's failure is fatal, the posture gc.c itself takes
    }
    unsigned long long base = (unsigned long long)(hwords + ((cap + 7) / 8));
    if (trace_keys) {
        for (long long j = 0; j < cap; j++) {
            desc[(base + (unsigned long long)j) >> 6] |= (unsigned long long)1 << ((base + (unsigned long long)j) & 63);
        }
    }
    if (trace_vals) {
        for (long long j = 0; j < cap; j++) {
            unsigned long long b = base + (unsigned long long)cap + (unsigned long long)j;
            desc[b >> 6] |= (unsigned long long)1 << (b & 63);
        }
    }
    *(void **)d = desc;
}

// carve_table allocates and initializes a data block — the shape the
// constructors open and rehash rebuilds.
static void *carve_table(long long hwords, long long cap, long long kdom, long long vtrace) {
    long long p = (cap + 7) & ~7LL;
    long long zones = hwords == MAP_WORDS ? 2 : 1;
    void *d = __we_alloc(16 + 8 * hwords + p + 8 * cap * zones);
    if (!d) {
        abort(); // nothing left to fall back to, as in gc.c's carving
    }
    long long *w = (long long *)d + 2;
    w[T_LEN] = 0;
    w[T_CAP] = cap;
    w[T_TOMB] = 0;
    w[T_KDOM] = kdom;
    if (hwords == MAP_WORDS) {
        w[T_VTRACE] = vtrace;
    }
    install_desc(d, hwords, cap, vtrace);
    return d;
}

// new_coll carves the handle and wires its table, the bridge closing the
// gap list.c's __we_list_new closes: the table carve can collect, and
// until the data pointer is wired the handle is only a C local.
static void *new_coll(long long hwords, long long cap, long long kdom, long long vtrace) {
    void *h = __we_alloc(32);
    if (!h) {
        abort(); // nothing left to fall back to, as in gc.c's carving
    }
    *(void **)h = handle_desc;
    __we_root_push(h);
    void *d = carve_table(hwords, cap, kdom, vtrace);
    *(void **)((char *)h + 16) = d;
    __we_root_pop();
    return h;
}

// find_slot returns the slot whose live key equals k, or -1 on a miss —
// the probe skips tombstones and stops at the first empty slot, which
// the load bound guarantees exists. On a miss *home is that stopping
// slot, the insertion point when no tombstone was passed; *first_tomb
// collects the first tombstone passed, for insertion's reuse, or -1.
static long long find_slot(tbl *t, long long k, long long *first_tomb, long long *home) {
    long long cap = *t->cap;
    long long i = (long long)(key_hash(*t->kdom, k) & (unsigned long long)(cap - 1));
    *first_tomb = -1;
    for (;;) {
        unsigned char o = t->occ[i];
        if (o == 0) {
            *home = i;
            return -1;
        }
        if (o == 2 && *first_tomb < 0) {
            *first_tomb = i;
        }
        if (o == 1 && key_eq(*t->kdom, t->keys[i], k)) {
            return i;
        }
        i = (i + 1) & (cap - 1);
    }
}

// raw_place sets an entry down in an empty slot, length counted — the
// rehash and the fill path for inputs already known duplicate-free.
static void raw_place(tbl *t, long long k, long long v) {
    long long cap = *t->cap;
    long long i = (long long)(key_hash(*t->kdom, k) & (unsigned long long)(cap - 1));
    while (t->occ[i] != 0) {
        i = (i + 1) & (cap - 1);
    }
    t->occ[i] = 1;
    t->keys[i] = k;
    if (t->vals) {
        t->vals[i] = v;
    }
    (*t->len)++;
}

// tbl_insert places (k, v) with put semantics: an equal live key
// overwrites the value (the length unchanged); a miss takes the first
// tombstone the probe passed — the deleted slot reborn, the tombstone
// count shrinking — or the empty slot the probe stopped at. Answers 1
// when the table grew an entry.
static long long tbl_insert(tbl *t, long long k, long long v) {
    long long ft;
    long long home;
    long long i = find_slot(t, k, &ft, &home);
    if (i >= 0) {
        if (t->vals) {
            t->vals[i] = v;
        }
        return 0;
    }
    if (ft >= 0) {
        i = ft;
        (*t->tomb)--;
    } else {
        i = home; // the probe's stopping slot: the first empty one
    }
    t->occ[i] = 1;
    t->keys[i] = k;
    if (t->vals) {
        t->vals[i] = v;
    }
    (*t->len)++;
    return 1;
}

// grow_table doubles the table: carve, re-place the actives (a fresh
// table has no tombstones and no duplicates — the live keys are unique
// by invariant), and swap the data pointer in place inside the handle.
// The receiver is the caller's to root across this — the rooted handle
// keeps the old block, and through its descriptor every key and value,
// alive through the carve and the re-placement.
static void *grow_table(void *h, tbl *t, long long hwords) {
    long long vtrace = t->vtrace ? *t->vtrace : 0;
    void *nd = carve_table(hwords, *t->cap * 2, *t->kdom, vtrace);
    tbl nt;
    tbl_of(nd, hwords, &nt);
    for (long long i = 0; i < *t->cap; i++) {
        if (t->occ[i] == 1) {
            raw_place(&nt, t->keys[i], t->vals ? t->vals[i] : 0);
        }
    }
    *(void **)((char *)h + 16) = nd; // in place; the old block dies at the next sweep
    return nd;
}

// tbl_put is the growth rule both writers share: past the load bound,
// double first — the carve can collect while this call's own argument
// words are still only the caller's discipline, so the traced ones are
// rooted for exactly the branch (fs.c's depth).
static void tbl_put(void *h, tbl *t, long long hwords, long long k, long long v) {
    if (crowded(*t->len, *t->tomb, *t->cap)) {
        long long pushed = 0;
        if (*t->kdom == 1) {
            __we_root_push((void *)(uintptr_t)k);
            pushed++;
        }
        if (t->vtrace && *t->vtrace) {
            __we_root_push((void *)(uintptr_t)v);
            pushed++;
        }
        void *nd = grow_table(h, t, hwords);
        while (pushed-- > 0) {
            __we_root_pop();
        }
        tbl_of(nd, hwords, t);
    }
    tbl_insert(t, k, v);
}

// The Option trio: None is tag 0, Some tag 1 — the order the codegen
// side's Option variant table carries — the payload in pay0, pay1
// zeroed so the three-word form never carries stale stack bytes (fs.c's
// ok_unit posture).
static void out_none(long long out[3]) {
    out[0] = 0;
    out[1] = 0;
    out[2] = 0;
}

static void out_some(long long out[3], long long v) {
    out[0] = 1;
    out[1] = v;
    out[2] = 0;
}

void *__we_coll_map_of(void *keys, void *vals, long long kdom, long long vtrace) {
    long long n = __we_list_len(keys);
    if (n != __we_list_len(vals)) {
        __we_task_fail("mapOf length mismatch");
    }
    long long cap = 8; // sized so the fill never asks the table to grow
    while (crowded(n, 0, cap)) {
        cap <<= 1;
    }
    __we_root_push(keys);
    __we_root_push(vals);
    void *h = new_coll(MAP_WORDS, cap, kdom, vtrace);
    tbl t;
    tbl_of(data_of(h), MAP_WORDS, &t);
    for (long long i = 0; i < n; i++) {
        tbl_insert(&t, __we_list_get(keys, i), __we_list_get(vals, i));
    }
    __we_root_pop();
    __we_root_pop();
    return h;
}

void *__we_coll_set_of(void *items, long long kdom) {
    long long n = __we_list_len(items);
    long long cap = 8;
    while (crowded(n, 0, cap)) {
        cap <<= 1;
    }
    __we_root_push(items);
    void *h = new_coll(SET_WORDS, cap, kdom, 0);
    tbl t;
    tbl_of(data_of(h), SET_WORDS, &t);
    for (long long i = 0; i < n; i++) {
        tbl_insert(&t, __we_list_get(items, i), 0);
    }
    __we_root_pop();
    return h;
}

void __we_coll_map_put(void *m, long long k, long long v) {
    __we_root_push(m);
    tbl t;
    tbl_of(data_of(m), MAP_WORDS, &t);
    tbl_put(m, &t, MAP_WORDS, k, v);
    __we_root_pop();
}

void __we_coll_map_get(void *m, long long k, long long out[3]) {
    tbl t;
    tbl_of(data_of(m), MAP_WORDS, &t);
    long long ft;
    long long home; /* unused here: a read never inserts */
    long long i = find_slot(&t, k, &ft, &home);
    if (i < 0) {
        out_none(out);
        return;
    }
    out_some(out, t.vals[i]);
}

void __we_coll_map_remove(void *m, long long k, long long out[3]) {
    __we_root_push(m); // the standing shape: an entry roots its receiver (fs.c); this path carves nothing
    tbl t;
    tbl_of(data_of(m), MAP_WORDS, &t);
    long long ft;
    long long home; /* unused here: a removal never inserts */
    long long i = find_slot(&t, k, &ft, &home);
    if (i < 0) {
        out_none(out);
    } else {
        out_some(out, t.vals[i]);
        t.occ[i] = 2;
        (*t.len)--;
        (*t.tomb)++;
        // The key word stays: the descriptor still points at it, and a
        // stale handle marked is one kept alive a round too long — the
        // conservative side of safe.
    }
    __we_root_pop();
}

void *__we_coll_map_keys(void *m) {
    __we_root_push(m); // the table is read across the list's construction
    tbl t;
    tbl_of(data_of(m), MAP_WORDS, &t);
    void *l = __we_list_new(*t.len, *t.kdom == 1); // exact capacity: no growth
    __we_root_push(l);
    for (long long i = 0; i < *t.cap; i++) {
        if (t.occ[i] == 1) {
            l = __we_list_push(l, t.keys[i]);
        }
    }
    __we_root_pop();
    __we_root_pop();
    return l;
}

long long __we_coll_map_size(void *m) {
    tbl t;
    tbl_of(data_of(m), MAP_WORDS, &t);
    return *t.len;
}

void __we_coll_set_add(void *s, long long x) {
    __we_root_push(s);
    tbl t;
    tbl_of(data_of(s), SET_WORDS, &t);
    tbl_put(s, &t, SET_WORDS, x, 0);
    __we_root_pop();
}

long long __we_coll_set_remove(void *s, long long x) {
    tbl t;
    tbl_of(data_of(s), SET_WORDS, &t);
    long long ft;
    long long home; /* unused here: a removal never inserts */
    long long i = find_slot(&t, x, &ft, &home);
    if (i < 0) {
        return 0;
    }
    t.occ[i] = 2;
    (*t.len)--;
    (*t.tomb)++;
    return 1;
}

long long __we_coll_set_has(void *s, long long x) {
    tbl t;
    tbl_of(data_of(s), SET_WORDS, &t);
    long long ft;
    long long home; /* unused here: a read never inserts */
    return find_slot(&t, x, &ft, &home) >= 0;
}

long long __we_coll_set_size(void *s) {
    tbl t;
    tbl_of(data_of(s), SET_WORDS, &t);
    return *t.len;
}

void __we_coll_list_get(void *l, long long i, long long out[3]) {
    long long n = __we_list_len(l);
    if (i < 0 || i >= n) {
        out_none(out); // the surface face: absence, not the carrier's trap
        return;
    }
    out_some(out, __we_list_get(l, i));
}

void __we_coll_list_remove_at(void *l, long long i, long long out[3]) {
    long long n = __we_list_len(l);
    if (i < 0 || i >= n) {
        out_none(out);
        return;
    }
    out_some(out, __we_list_get(l, i));
    // The shift walks the carrier's data block under the layout list.h
    // publishes — three payload words, then the elements — one family
    // reading the other's published shape, in place, no carve. The
    // vacated tail word is zeroed: a marked zero is the shape unused
    // capacity already has.
    void *d = data_of(l);
    long long *elems = (long long *)d + 2 + 3;
    for (long long j = i; j + 1 < n; j++) {
        elems[j] = elems[j + 1];
    }
    elems[n - 1] = 0;
    ((long long *)d)[2] = n - 1;
}
