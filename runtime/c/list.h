// The We runtime's List carrier (design D6): the growable element vector
// chapter 17's snapshot iteration and the eager combinators run over. A
// list is one gc block — the frozen header every object carries, then
// three payload slots the family owns (length, capacity, the element
// domain's traced flag) and the element words from slot 3 on. One element
// is one word: a scalar or a gc handle. The layout is the family's
// business; the emitter sees an opaque `ptr`.
//
// An element that is a gc reference is traced through the block's map
// descriptor, which this family builds at allocation time — a list's
// capacity is a runtime value, so no compiler-emitted static descriptor
// can cover it (see list.c for the descriptor's lifetime).
#ifndef WE_LIST_H
#define WE_LIST_H

// new carves a list with room for cap elements. traced states whether an
// element word holds a gc reference (1) or a scalar (0); it is the
// block's descriptor that answers the collector from here on.
void *__we_list_new(long long cap, long long traced);

// push appends one element and returns the list: a full vector doubles,
// which moves the block, so the caller MUST take the result as the
// list's new identity.
void *__we_list_push(void *l, long long v);

// get reads one element. This is the carrier's own accessor, not the
// surface `List.get` chapter 17 ratifies: that one answers Option and
// treats an index outside the list as absence — a value, no trap. Here an
// out-of-range index is a compiler bug, so it terminates the calling task
// through chapter 14's family and the message names this operation. The
// loops emitted over a list stay inside its length by construction.
long long __we_list_get(void *l, long long i);

// len is the element count — not the capacity.
long long __we_list_len(void *l);

// snap returns a fresh list holding the receiver's elements as they are
// now: chapter 17's snapshot, taken at the call, so later mutation of the
// receiver is not seen by iteration over the result.
void *__we_list_snap(void *l);

#endif
