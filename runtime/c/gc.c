// The We runtime's collector (chapter 16's precise stop-the-world design;
// ADR-0003's carrier). The root protocol is a shadow stack: the compiler
// emits __we_root_push/__we_root_pop around every gc reference's lifetime,
// so the root set is exact by construction — no conservative stack scan
// exists. Objects carry the frozen header {map@0, size@8} with payload
// from 16: the map word points at the compiler's layout descriptor — one
// bitmap word per 64 payload slots, bit i set when the word at 16+8*i
// holds a gc reference — and bit 0 of the stored word is the mark bit
// (descriptors are at least 8-aligned). Between collections no mark bits
// exist: mark sets them, sweep clears every survivor's.
//
// Allocation bumps within malloc'd chunks once the free list (first fit,
// split when the remainder holds a header) misses. Every byte of a
// chunk's used prefix is covered by exactly one block header, so sweep
// walks those prefixes and rebuilds the free list from scratch — a
// block's freeness lives only in the rebuilt list, never in a stale bit.
#include <stdlib.h>
#include <string.h>

typedef unsigned long long u64;

#define MARK ((u64)1)

// Block sizes are multiples of 8, so size bit 0 is free to carry the
// free-list tag: a block already reclaimed (on the list) keeps it set,
// which is how a later sweep counts only newly-dead blocks.
static u64 blk_size(void *p) { return ((u64 *)p)[1] & ~(u64)7; }
static void blk_set_size(void *p, u64 n) { ((u64 *)p)[1] = n; }
static void *blk_map(void *p) { return (void *)(*(u64 *)p & ~MARK); }
static int blk_marked(void *p) { return (*(u64 *)p & MARK) != 0; }
static int blk_free(void *p) { return (((u64 *)p)[1] & 1) != 0; }

// Chunks hand out 8-aligned blocks from their data tails.
struct chunk { struct chunk *next; u64 cap, used; };
#define CHUNK_DATA(c) ((char *)(c) + sizeof(struct chunk))
#define CHUNK_GRAN ((u64)1 << 20)

static struct chunk *chunks;
static void *free_list; // threaded through each free block's map slot
static u64 allocated_since; // carve bytes since the last collection

static void **roots;
static u64 nroots, roots_cap;

static void **wstack; // the mark phase's worklist
static u64 wlen, wstack_cap;

static int booted;

void __we_gc_boot(void) {
    if (booted) {
        return;
    }
    booted = 1; // the zero-initialized globals are the boot state
}

static void boot_check(void) {
    if (!booted) {
        __we_gc_boot();
    }
}

static void push_free(void *p) {
    *(void **)p = free_list;
    free_list = p;
    ((u64 *)p)[1] |= 1; // the free tag rides size bit 0
}

static void push_work(void *p) {
    if (wlen == wstack_cap) {
        wstack_cap = wstack_cap ? wstack_cap * 2 : 64;
        wstack = realloc(wstack, wstack_cap * sizeof *wstack);
        if (!wstack) {
            abort(); // a stop-the-world collector has no fallback
        }
    }
    wstack[wlen++] = p;
}

// Mark every block reachable from the root stack, then sweep the chunk
// prefixes: survivors lose their mark bit, everything else enters the
// rebuilt free list. Returns the number of blocks swept.
long long __we_gc_collect(void) {
    boot_check();
    for (u64 i = 0; i < nroots; i++) {
        void *p = roots[i];
        if (p && !blk_marked(p)) {
            *(u64 *)p |= MARK;
            push_work(p);
        }
    }
    while (wlen > 0) {
        char *p = wstack[--wlen];
        u64 *desc = blk_map(p);
        if (!desc) {
            continue; // no descriptor: nothing to trace through
        }
        u64 nslots = (blk_size(p) - 16) / 8;
        for (u64 i = 0; i < nslots; i++) {
            if (desc[i >> 6] & ((u64)1 << (i & 63))) {
                void *child = *(void **)(p + 16 + 8 * i);
                if (child && !blk_marked(child)) {
                    *(u64 *)child |= MARK;
                    push_work(child);
                }
            }
        }
    }
    free_list = NULL;
    u64 swept = 0;
    for (struct chunk *c = chunks; c; c = c->next) {
        char *p = CHUNK_DATA(c), *end = p + c->used;
        while (p < end) {
            char *next = p + blk_size(p); // read before push_free rewrites the map slot
            if (blk_marked(p)) {
                *(u64 *)p &= ~MARK;
            } else {
                if (!blk_free(p)) {
                    swept++; // already-free blocks re-enter the list uncounted
                }
                push_free(p);
            }
            p = next;
        }
    }
    allocated_since = 0;
    return (long long)swept;
}

void __we_root_push(void *p) {
    boot_check();
    if (nroots == roots_cap) {
        roots_cap = roots_cap ? roots_cap * 2 : 64;
        roots = realloc(roots, roots_cap * sizeof *roots);
        if (!roots) {
            abort();
        }
    }
    roots[nroots++] = p;
}

void __we_root_pop(void) {
    if (nroots > 0) {
        nroots--;
    }
}

// The frozen M4 ABI (ADR-0002): the size argument already includes the
// header space, the returned block is zeroed with the size word written
// (the constructor's code stores the map pointer; a zeroed map slot keeps
// a half-filled object trace-safe), and a collection fires at the byte
// threshold before the new block is carved — the construction protocol
// roots everything live before evaluating fields, so entry is the safe
// point.
#define GC_THRESHOLD ((u64)1 << 20)

void *__we_alloc(long long n) {
    boot_check();
    u64 sz = n < 16 ? 16 : (u64)n;
    sz = (sz + 7) & ~(u64)7;
    if (allocated_since >= GC_THRESHOLD) {
        __we_gc_collect();
    }
    void *p = NULL;
    void **prev = &free_list;
    for (void *f = free_list; f; ) {
        void *next = *(void **)f;
        if (blk_size(f) >= sz) {
            u64 rem = blk_size(f) - sz;
            if (rem >= 16) {
                char *rest = (char *)f + sz;
                blk_set_size(rest, rem);
                *prev = rest;
                *(void **)rest = next;
                ((u64 *)rest)[1] |= 1; // the split remainder enters the list
            } else {
                *prev = next;
            }
            p = f;
            break;
        }
        prev = (void **)f;
        f = next;
    }
    if (!p) {
        struct chunk *c = chunks;
        if (!c || c->cap - c->used < sz) {
            u64 cap = CHUNK_GRAN < sz ? sz : CHUNK_GRAN;
            c = malloc(sizeof *c + cap);
            if (!c) {
                return NULL;
            }
            c->next = chunks;
            chunks = c;
            c->cap = cap;
            c->used = 0;
        }
        p = CHUNK_DATA(c) + c->used;
        c->used += sz;
    }
    memset(p, 0, sz);
    blk_set_size(p, sz);
    allocated_since += sz;
    return p;
}

void __we_free(void *p) {
    boot_check();
    if (p) {
        push_free(p); // immediate reclamation: the caller has dropped it
    }
}
