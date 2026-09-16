// The We runtime's Map and Set carriers (B2b design D2): the open-addressed
// hash tables the collections surface runs over. A map or set value is a
// stable 32-byte handle — the frozen header, the data pointer at payload
// word 0, one reserved word — in the same posture the List carrier takes
// (list.h), and the table lives in the data block it points at:
//
//   Map:   LEN, CAP, TOMB, KDOM, VTRACE — five words — then the occupancy
//          bytes (one per slot, padded to a word), then the key words,
//          then the value words.
//   Set:   LEN, CAP, TOMB, KDOM — four words — then the occupancy bytes,
//          then the slot words.
//
// CAP is a power of two, never below 8. The occupancy byte is 0 (empty),
// 1 (live), or 2 (tombstone). KDOM names the key domain: 0 the identity
// domain — the eight integer widths and Bool, one word, hash the word —
// and 1 the String domain, where a key is the T11-2 box (bytes at +16,
// length at +24) hashed by FNV-1a over its content and compared by
// content, two equal strings being distinct boxes. VDOM values and Set
// elements are one word each, the List element domain; VTRACE states
// whether that word holds a gc reference.
//
// Probing is linear from `hash & (cap-1)`; a lookup skips tombstones and
// misses at the first empty slot. Deletion marks a tombstone and leaves
// the key word in place — the descriptor still points at it, and a stale
// handle marked is one kept alive a round too long, the conservative
// side of safe. A table whose live and dead together pass three quarters
// of its capacity doubles: the actives re-place into a fresh block, the
// tombstones vanish, and the data pointer swaps in place inside the
// handle — the List growth posture, so aliases see it.
//
// The entries answering Option write the fused trio {tag, pay0, pay1}:
// None is tag 0 and Some is tag 1 — the order the codegen side's Option
// variant table carries — with the payload word in pay0 and pay1 zeroed,
// the same shape the fs family's out trio established.
#ifndef WE_COLL_H

#define WE_COLL_H

// map_of zips two lists into a map: the keys list and the values list
// must agree in length — a mismatch is a task failure through chapter
// 14's family, the trap naming the operation. Duplicate keys take the
// last value (put semantics). kdom names the key domain, vtrace whether
// a value word holds a gc reference; the caller roots both lists across
// the call, and the returned handle is the caller's to root before any
// further allocation.
void *__we_coll_map_of(void *keys, void *vals, long long kdom, long long vtrace);

// set_of builds a set from the items list; kdom as map_of's.
void *__we_coll_set_of(void *items, long long kdom);

// put places (k, v): an equal live key overwrites the value, a new key
// grows the table when the load bound asks. The receiver must stay
// rooted across the call; the traced argument words are rooted for the
// growth branch here as depth.
void __we_coll_map_put(void *m, long long k, long long v);

// get answers Some with the value or None, through the out trio.
void __we_coll_map_get(void *m, long long k, long long out[3]);

// remove takes the entry out — a tombstone in its slot — and answers
// Some with the value it held, or None on a miss.
void __we_coll_map_remove(void *m, long long k, long long out[3]);

// keys answers a fresh List of the map's live keys in table order: one
// fixed order per call, no promise across mutations.
void *__we_coll_map_keys(void *m);

// size is the live entry count — not the capacity.
long long __we_coll_map_size(void *m);

// add places one element: a duplicate is a no-op. Growth as put's.
void __we_coll_set_add(void *s, long long x);

// remove takes the element out and answers whether it was there.
long long __we_coll_set_remove(void *s, long long x);

// has answers whether the element is live in the set.
long long __we_coll_set_has(void *s, long long x);

// size is the live element count.
long long __we_coll_set_size(void *s);

// list_get is the surface `List.get` chapter 17 ratifies: an index
// outside the list is absence — None through the out trio — never the
// carrier's own trap (list.h's __we_list_get), which the emitted walks
// keep inside their lengths by construction.
void __we_coll_list_get(void *l, long long i, long long out[3]);

// list_remove_at takes the element at i out — the live tail shifts one
// slot left inside the carrier's data block — and answers Some with the
// element it held, or None on an out-of-range index.
void __we_coll_list_remove_at(void *l, long long i, long long out[3]);

#endif
