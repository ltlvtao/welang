// The We runtime's List carrier (design D6): the growable element vector
// chapter 17's snapshot iteration and the eager combinators run over. A
// list value is a stable 32-byte handle — the frozen header every block
// carries, the data pointer at payload word 0, one reserved word — and
// the elements live in the data block it points at (list.c): three
// payload slots the family owns (length, capacity, the element domain's
// traced flag) and the element words from slot 3 on. One element is one
// word: a scalar or a gc handle. The handle and the blocks behind it are
// the family's business; the emitter sees an opaque `ptr`.
//
// An element that is a gc reference is traced through the data block's
// map descriptor, which this family builds at allocation time — a list's
// capacity is a runtime value, so no compiler-emitted static descriptor
// can cover it (see list.c for that descriptor's lifetime and for the
// handle's own constant descriptor).
#ifndef WE_LIST_H
#define WE_LIST_H

// new carves a handle wrapping a data block with room for cap elements.
// traced states whether an element word holds a gc reference (1) or a
// scalar (0); it is the data block's descriptor that answers the
// collector from here on.
void *__we_list_new(long long cap, long long traced);

// push appends one element and returns the list — the SAME handle it was
// handed: a full vector doubles behind the pointer, the data block
// swapped in place, so every alias of the list sees the growth and the
// result may be taken as the receiver itself. The caller MUST still root
// the receiver across the call: the growth carve can fire a collection,
// and the family cannot root the receiver's own elements itself.
void *__we_list_push(void *l, long long v);

// get reads one element through the handle's one hop. This is the
// carrier's own accessor, not the surface `List.get` chapter 17 ratifies:
// that one answers Option and treats an index outside the list as
// absence — a value, no trap. Here an out-of-range index is a compiler
// bug, so it terminates the calling task through chapter 14's family and
// the message names this operation. The loops emitted over a list stay
// inside its length by construction.
long long __we_list_get(void *l, long long i);

// len is the element count — not the capacity.
long long __we_list_len(void *l);

// snap returns a fresh handle wrapping a fresh data block holding the
// receiver's elements as they are now: chapter 17's snapshot, taken at
// the call, so later mutation of the receiver is not seen by iteration
// over the result.
void *__we_list_snap(void *l);

#endif
