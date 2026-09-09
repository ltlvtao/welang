// The We runtime's wait machine and the six primitive families of
// chapter 18 (M9b design D3–D6). Every blocking call walks the same
// protocol: probe the fast path, register a wait link on the source's
// queue, park, and on wake re-check the loop — so a wake is only ever a
// hint and correctness never depends on who woke whom or why. A
// cancelled wake makes the loop's cancelled exit answer the empty form
// (receive reports None, send stays unsent, acquire untaken, the cond
// loop breaks out); that is chapter 18's cooperative return made real.
//
// Single-threaded scheduling carries the mutual exclusion of
// Mutex/RwLock/Atomic/AtomicRef (nothing preempts a callback — E1402's
// purity is the critical section), so the cells are direct storage.
// Channels and the cells themselves are gc-allocated with the frozen
// header, and reference-bearing layouts carry the descriptor the emitter
// passes: waiting values live on their (possibly parked) owner's root
// stack, buffered values inside the gc-traced buffer block.
//
// Every completion point a value moves through also feeds the exploration
// trace (M10c design D4): deliver records the receiver's receive, the
// take points record the parked sender's send, and each local completion
// records its own — one no-op branch in a normal run.
#include <stdlib.h>
#include <string.h>

#include "sched.h"

extern void *__we_alloc(long long n);

struct we_select;

// The one family-wide registration node. The base is the scheduler's
// (queue thread + owner ledger); the tail serves every family: channels
// park senders with a value and receivers with a destination, selects
// park with the arm they race for.
typedef struct we_link {
    we_wait_link base;
    long long value;            // a parked sender's unsent payload
    long long *out;             // a parked receiver's destination
    struct we_select *sel;      // the select arm this registration serves
    long long arm;
    struct we_link *sel_next;   // the select's pending list
    int consumed;               // a delivery finished this registration:
                                // the owner returns instead of re-parking
} we_link;

// Queue helpers: the source's own list, threaded by base.next. Dead
// links (cancelled or fired-off select losers) are reaped as the walk
// finds them — __we_link_free unthreads the owner's ledger and frees the
// whole node. Detaching takes a live link off the queue; the owner wakes
// and settles its own registration, so deliveries never free a link the
// owner still holds.

static we_link *q_pop_alive(we_link **head) {
    while (*head) {
        we_link *l = *head;
        if (l->base.dead) {
            *head = (we_link *)l->base.next;
            __we_link_free(&l->base);
            continue;
        }
        *head = (we_link *)l->base.next;
        return l;
    }
    return NULL;
}

static we_link *q_peek_alive(we_link **head) {
    while (*head) {
        if ((*head)->base.dead) {
            we_link *dead = *head;
            *head = (we_link *)dead->base.next;
            __we_link_free(&dead->base);
            continue;
        }
        return *head;
    }
    return NULL;
}

static void q_push(we_link **head, we_link *l) {
    while (*head) {
        head = (we_link **)&(*head)->base.next;
    }
    l->base.next = NULL;
    *head = l;
}

static we_link *q_detach(we_link **head, we_link *l) {
    while (*head) {
        if (*head == l) {
            *head = (we_link *)l->base.next;
            return l;
        }
        head = (we_link **)&(*head)->base.next;
    }
    return NULL;
}

// Drop a registration fully: queue already detached by the caller.
// __we_link_free unthreads the owner's ledger and frees the node.
static void we_link_drop(we_link *l) {
    __we_link_free(&l->base);
}

// --- the shared-state cells -------------------------------------------------

// {map@0 (zero: no traced slots), size@8, value@16} — 24 bytes gc-allocated.
void *__we_prim_new_mutex(long long v) {
    long long *cell = __we_alloc(24);
    cell[2] = v;
    return cell;
}

void *__we_prim_new_rwlock(long long v) {
    return __we_prim_new_mutex(v);
}

// The reference cell carries its payload's descriptor: bit 0 of the
// bitmap covers the one payload slot at offset 16.
void *__we_prim_new_atomic_ref(long long p, void *desc) {
    long long *cell = __we_alloc(24);
    *(void **)cell = desc;
    cell[2] = p;
    return cell;
}

void *__we_prim_new_atomic(long long v) {
    return __we_prim_new_mutex(v);
}

long long __we_prim_get(void *cell) { return ((long long *)cell)[2]; }

void __we_prim_set(void *cell, long long v) { ((long long *)cell)[2] = v; }

long long __we_prim_update(void *cell, long long (*f)(void *, long long), void *env) {
    long long v = f(env, ((long long *)cell)[2]);
    ((long long *)cell)[2] = v;
    return v;
}

long long __we_prim_read(void *cell, long long (*f)(void *, long long), void *env) {
    return f(env, ((long long *)cell)[2]);
}

// --- Cond -------------------------------------------------------------------

// Every gc-allocated family object starts with the frozen header's two
// words ({map@0, size@8} — gc.c owns them; the payload fields follow at
// 16). The header pads below keep the C layout on the gc layout: a family
// struct laid out from offset 0 would write its first fields straight
// into the map and size words.
typedef struct we_prim_hdr {
    void *map;
    long long size;
} we_prim_hdr;

typedef struct we_cond {
    we_prim_hdr hdr;
    we_link *waiters; // malloc'd queue head: untraced, reaped lazily
    void *mtx;        // the paired cell: predicates read its current value
} we_cond;

// {map@0 zero, size@8, waiters@16 (malloc, untraced), mtx@24} — 32 bytes.
void *__we_prim_new_cond(void *mtx) {
    we_cond *c = __we_alloc(32);
    c->waiters = NULL;
    c->mtx = mtx;
    return c;
}

// The predicate loop: wake-ups are hints, the predicate decides, and the
// parked link is re-registered fresh each round (the waker detaches it).
// A cancelled wake breaks out (chapter 18's cooperative return).
long long __we_cond_wait(void *cc, long long (*pred)(void *, long long), void *env) {
    we_cond *c = cc;
    while (!pred(env, __we_prim_get(c->mtx))) {
        if (__we_cur_cancelled()) {
            return 1;
        }
        we_link *l = calloc(1, sizeof *l);
        if (!l) {
            abort();
        }
        __we_link_attach(&l->base);
        q_push(&c->waiters, l);
        __we_task_park();
        we_link_drop(l); // detached by the waker (or dead-marked): settle
    }
    return 0;
}

void __we_cond_signal(void *cc) {
    we_cond *c = cc;
    we_link *l = q_pop_alive(&c->waiters);
    if (l) {
        __we_task_wake(l->base.owner);
    }
}

void __we_cond_broadcast(void *cc) {
    we_cond *c = cc;
    we_link *l = q_pop_alive(&c->waiters);
    while (l) {
        __we_task_wake(l->base.owner);
        l = q_pop_alive(&c->waiters);
    }
}

// --- Semaphore --------------------------------------------------------------

typedef struct we_sem {
    we_prim_hdr hdr;
    long long count, max;
    we_link *waiters;
} we_sem;

// {map@0 zero, size@8, count@16, max@24, waiters@32} — 40 bytes.
void *__we_prim_new_sem(long long n) {
    we_sem *s = __we_alloc(40);
    s->count = n;
    s->max = n;
    s->waiters = NULL;
    return s;
}

long long __we_sem_acquire(void *ss) {
    we_sem *s = ss;
    for (;;) {
        if (s->count > 0) {
            s->count--;
            return 0;
        }
        if (__we_cur_cancelled()) {
            return 1; // untaken
        }
        we_link *l = calloc(1, sizeof *l);
        if (!l) {
            abort();
        }
        __we_link_attach(&l->base);
        q_push(&s->waiters, l);
        __we_task_park();
        we_link_drop(l); // detached by the releasing waker: settle
    }
}

long long __we_sem_try_acquire(void *ss) {
    we_sem *s = ss;
    if (s->count > 0) {
        s->count--;
        return 1;
    }
    return 0;
}

void __we_sem_release(void *ss) {
    we_sem *s = ss;
    if (s->count >= s->max) {
        __we_task_fail("semaphore released past its constructed count");
        return;
    }
    s->count++;
    we_link *l = q_pop_alive(&s->waiters);
    if (l) {
        __we_task_wake(l->base.owner); // the loop re-checks and takes
    }
}

long long __we_sem_count(void *ss) { return ((we_sem *)ss)->count; }

// --- Channel ----------------------------------------------------------------

typedef struct we_chan {
    we_prim_hdr hdr;
    long long cap, head, count, slot_width, closed;
    we_link *senders;
    we_link *receivers;
    char *buf; // gc-allocated ring: cap * slot_width bytes
    long long trace_id; // the per-run creation ordinal (the trace's chan id)
} we_chan;

// The channel object: {map@0 (descriptor: buf slot traced), size@8,
// cap@16, head@24, count@32, slot_width@40, closed@48, senders@56
// (malloc, untraced), receivers@64 (same), buf@72, trace_id@80} — 88
// bytes. Slot 7 (offset 72) is the one traced reference.
static unsigned long long chan_desc[1] = {0x80};

void *__we_prim_new_chan(long long cap, long long slot_width, void *desc) {
    we_chan *ch = __we_alloc(88);
    *(void **)ch = chan_desc;
    ch->cap = cap;
    ch->slot_width = slot_width ? slot_width : 8;
    ch->trace_id = __we_trace_alloc_id();
    if (cap > 0) {
        ch->buf = __we_alloc(16 + cap * ch->slot_width);
        *(void **)ch->buf = desc; // element bitmap: traced when reference-bearing
    }
    return ch;
}

static void buf_put(we_chan *ch, long long v) {
    long long idx = (ch->head + ch->count) % ch->cap;
    memcpy(ch->buf + 16 + idx * ch->slot_width, &v, (size_t)ch->slot_width);
}

static long long buf_get(we_chan *ch) {
    long long v = 0;
    memcpy(&v, ch->buf + 16 + ch->head * ch->slot_width, (size_t)ch->slot_width);
    ch->head = (ch->head + 1) % ch->cap;
    return v;
}

static void select_fire(struct we_select *sel, long long arm, long long v);

// Deliver to a live parked receiver (already detached from the queue):
// straight through its out slot, or into its select's taken arm. The
// owner settles the link after waking (consumed is its signal).
static void deliver(we_chan *ch, we_link *r, long long v) {
    if (r->out) {
        *r->out = v;
    } else if (r->sel) {
        select_fire(r->sel, r->arm, v);
    }
    r->consumed = 1;
    __we_trace_event(WE_EV_RECV, ch->trace_id, r->base.owner);
    __we_task_wake(r->base.owner);
}

// A parked sender completes (already detached): its value fills the
// freed buffer space and its send returns.
static void sender_fill(we_chan *ch, we_link *s) {
    buf_put(ch, s->value);
    ch->count++;
    s->consumed = 1;
    __we_trace_event(WE_EV_SEND, ch->trace_id, s->base.owner);
    __we_task_wake(s->base.owner);
}

long long __we_chan_send(void *cc, long long v) {
    we_chan *ch = cc;
    for (;;) {
        if (ch->closed) {
            __we_task_fail("send on a closed channel");
            return 1;
        }
        we_link *r = q_pop_alive(&ch->receivers);
        if (r) { // a waiting receiver takes it now (rendezvous or head start)
            deliver(ch, r, v);
            __we_trace_event(WE_EV_SEND, ch->trace_id, __we_cur_task());
            return 0;
        }
        if (ch->count < ch->cap) {
            buf_put(ch, v);
            ch->count++;
            __we_trace_event(WE_EV_SEND, ch->trace_id, __we_cur_task());
            return 0;
        }
        if (__we_cur_cancelled()) {
            return 1; // unsent
        }
        we_link *l = calloc(1, sizeof *l);
        if (!l) {
            abort();
        }
        __we_link_attach(&l->base);
        l->value = v;
        q_push(&ch->senders, l);
        __we_task_park();
        if (l->consumed) {
            we_link_drop(l);
            return 0;
        }
        we_link_drop(l); // plain wake (close): the loop re-checks closed
    }
}

long long __we_chan_recv(void *cc, long long *out) {
    we_chan *ch = cc;
    for (;;) {
        if (ch->count > 0) {
            long long v = buf_get(ch);
            ch->count--;
            we_link *s = q_pop_alive(&ch->senders);
            if (s) {
                sender_fill(ch, s);
            }
            *out = v;
            __we_trace_event(WE_EV_RECV, ch->trace_id, __we_cur_task());
            return 1;
        }
        we_link *s = q_pop_alive(&ch->senders);
        if (s) { // rendezvous: take the parked sender's payload now
            *out = s->value;
            s->consumed = 1;
            __we_trace_event(WE_EV_SEND, ch->trace_id, s->base.owner);
            __we_task_wake(s->base.owner);
            __we_trace_event(WE_EV_RECV, ch->trace_id, __we_cur_task());
            return 1;
        }
        if (ch->closed) {
            return 0; // None, permanently
        }
        if (__we_cur_cancelled()) {
            *out = 0;
            return 0; // the cancelled wake's empty form
        }
        we_link *l = calloc(1, sizeof *l);
        if (!l) {
            abort();
        }
        __we_link_attach(&l->base);
        l->out = out;
        q_push(&ch->receivers, l);
        __we_task_park();
        if (l->consumed) { // the delivered value already sits in *out
            we_link_drop(l);
            return 1;
        }
        we_link_drop(l); // plain wake: the loop re-checks
    }
}

long long __we_chan_close(void *cc) {
    we_chan *ch = cc;
    if (ch->closed) {
        return 0;
    }
    ch->closed = 1;
    // Every waiter's loop re-checks: senders fail, receivers drain then
    // report None — the close semantics live in the loops, not here.
    we_link *w = q_pop_alive(&ch->receivers);
    while (w) {
        __we_task_wake(w->base.owner);
        w = q_pop_alive(&ch->receivers);
    }
    w = q_pop_alive(&ch->senders);
    while (w) {
        __we_task_wake(w->base.owner);
        w = q_pop_alive(&ch->senders);
    }
    return 0;
}

// The try pair never parks (a parked counterparty is fair game though).
long long __we_chan_try_send(void *cc, long long v) {
    we_chan *ch = cc;
    if (ch->closed) {
        return 2;
    }
    we_link *r = q_pop_alive(&ch->receivers);
    if (r) {
        deliver(ch, r, v);
        __we_trace_event(WE_EV_SEND, ch->trace_id, __we_cur_task());
        return 0;
    }
    if (ch->count >= ch->cap) {
        return 1; // full
    }
    buf_put(ch, v);
    ch->count++;
    __we_trace_event(WE_EV_SEND, ch->trace_id, __we_cur_task());
    return 0;
}

long long __we_chan_try_recv(void *cc, long long *out) {
    we_chan *ch = cc;
    if (ch->count > 0) {
        *out = buf_get(ch);
        ch->count--;
        we_link *s = q_pop_alive(&ch->senders);
        if (s) {
            sender_fill(ch, s);
        }
        __we_trace_event(WE_EV_RECV, ch->trace_id, __we_cur_task());
        return 1; // Received
    }
    we_link *s = q_pop_alive(&ch->senders);
    if (s) {
        *out = s->value;
        s->consumed = 1;
        __we_trace_event(WE_EV_SEND, ch->trace_id, s->base.owner);
        __we_task_wake(s->base.owner);
        __we_trace_event(WE_EV_RECV, ch->trace_id, __we_cur_task());
        return 1;
    }
    return ch->closed ? 2 : 0; // Closed : Empty
}

// --- CancelSignal -----------------------------------------------------------

typedef struct we_sig {
    we_link *waiters;
} we_sig;

// {map@0 zero, size@8, waiters@16} — 24 bytes.
void *__we_sig_current(void) {
    we_task *t = __we_cur_task();
    if (!t->sig) {
        we_sig *g = __we_alloc(24);
        g->waiters = NULL;
        t->sig = g;
    }
    return t->sig;
}

long long __we_sig_check(void *gg) {
    (void)gg;
    return __we_cur_cancelled();
}

// Park until cancellation. The scheduler's cancel path is what performs
// the wake (scope expiry or handle cancel), so this only registers; the
// dead-marked link settles here.
long long __we_sig_wait(void *gg) {
    we_sig *g = gg;
    if (__we_cur_cancelled()) {
        return 0;
    }
    we_link *l = calloc(1, sizeof *l);
    if (!l) {
        abort();
    }
    __we_link_attach(&l->base);
    q_push(&g->waiters, l);
    __we_task_park();
    we_link_drop(l);
    return 0;
}

// --- select -----------------------------------------------------------------

typedef struct we_select {
    long long n;     // arms registered (indexing stays stable per add)
    long long taken; // -1 until an arm fires
    long long value; // the fired arm's delivered value
    we_link *pending; // registrations still live on their source queues
} we_select;

// Fire the select (fast path or delivery): record the arm, dead-mark
// every other registration so their sources skip and reap them.
static void select_fire(struct we_select *sel, long long arm, long long v) {
    if (sel->taken >= 0) {
        return; // already decided
    }
    sel->taken = arm;
    sel->value = v;
    for (we_link *l = sel->pending; l; l = l->sel_next) {
        if (l->arm != arm) {
            l->base.dead = 1;
        }
    }
}

void *__we_select_new(void) {
    we_select *sel = calloc(1, sizeof *sel);
    if (!sel) {
        abort();
    }
    sel->taken = -1;
    return sel;
}

static we_link *sel_reg(we_select *sel, long long arm) {
    we_link *l = calloc(1, sizeof *l);
    if (!l) {
        abort();
    }
    __we_link_attach(&l->base);
    l->sel = sel;
    l->arm = arm;
    l->sel_next = sel->pending;
    sel->pending = l;
    sel->n++;
    return l;
}

void __we_select_add_recv(void *s, void *cc) {
    we_select *sel = s;
    we_chan *ch = cc;
    long long arm = sel->n;
    if (ch->count > 0) { // ready now: take without registering
        long long v = buf_get(ch);
        ch->count--;
        we_link *snd = q_pop_alive(&ch->senders);
        if (snd) {
            sender_fill(ch, snd);
        }
        __we_trace_event(WE_EV_RECV, ch->trace_id, __we_cur_task());
        select_fire(sel, arm, v);
        return;
    }
    we_link *s2 = q_pop_alive(&ch->senders);
    if (s2) { // a parked sender hands over now
        __we_trace_event(WE_EV_SEND, ch->trace_id, s2->base.owner);
        select_fire(sel, arm, s2->value);
        s2->consumed = 1;
        __we_task_wake(s2->base.owner);
        __we_trace_event(WE_EV_RECV, ch->trace_id, __we_cur_task());
        return;
    }
    if (ch->closed) {
        select_fire(sel, arm, 0); // receive on a closed channel: None
        return;
    }
    we_link *l = sel_reg(sel, arm);
    l->out = NULL; // deliveries route through sel/arm
    q_push(&ch->receivers, l);
}

void __we_select_add_send(void *s, void *cc, long long v) {
    we_select *sel = s;
    we_chan *ch = cc;
    long long arm = sel->n;
    if (ch->closed) {
        select_fire(sel, arm, 0); // sending on closed is the body's panic face
        return;
    }
    we_link *r = q_pop_alive(&ch->receivers);
    if (r) {
        deliver(ch, r, v); // deliver routes the select fire itself
        __we_trace_event(WE_EV_SEND, ch->trace_id, __we_cur_task());
        return;
    }
    if (ch->count < ch->cap) {
        buf_put(ch, v);
        ch->count++;
        __we_trace_event(WE_EV_SEND, ch->trace_id, __we_cur_task());
        select_fire(sel, arm, 0);
        return;
    }
    we_link *l = sel_reg(sel, arm);
    l->value = v;
    q_push(&ch->senders, l);
}

// The await/sig arms complete through the scheduler's hook: fire the
// arm, hand the link's death back to the scheduler (it unthreads the
// registration itself), then wake the owner.
static void select_complete(we_wait_link *base) {
    we_link *l = (we_link *)base;
    select_fire(l->sel, l->arm, 0);
    we_link **p = &l->sel->pending; // off the select's list: the
    while (*p && *p != l) {         // scheduler's path frees this node
        p = &(*p)->sel_next;
    }
    if (*p) {
        *p = l->sel_next;
    }
    l->sel = NULL; // the park-side settle skips scheduler-owned links
    __we_task_wake(base->owner);
}

void __we_select_add_await(void *s, void *h) {
    we_select *sel = s;
    we_task *t = h;
    long long arm = sel->n;
    if (t->state == WE_DONE) {
        select_fire(sel, arm, 0); // the outcome reads off the handle after
        return;
    }
    we_link *l = sel_reg(sel, arm);
    l->base.complete = select_complete;
    l->base.next = t->awaiters;
    t->awaiters = &l->base;
}

void __we_select_add_sig(void *s) {
    we_select *sel = s;
    long long arm = sel->n;
    if (__we_cur_cancelled()) {
        select_fire(sel, arm, 0);
        return;
    }
    we_task *t = __we_cur_task();
    if (!t->sig) {
        __we_sig_current();
    }
    we_sig *g = t->sig;
    we_link *l = sel_reg(sel, arm);
    l->base.complete = select_complete; // a cancel wake fires the arm too
    q_push(&g->waiters, l);
}

long long __we_select_park(void *s) {
    we_select *sel = s;
    if (sel->taken < 0) {
        __we_task_park();
    }
    // Settle the winner: its delivery detached it from the source queue
    // (consumed marks it), so the owner drops it here. Losers stay
    // dead-marked on their sources' queues for the lazy reap; links the
    // scheduler already freed (await/sig arms) left this list in their
    // own complete hook.
    for (we_link *l = sel->pending; l;) {
        we_link *next = l->sel_next;
        if (l->consumed) {
            we_link_drop(l);
        }
        l = next;
    }
    sel->pending = NULL;
    return sel->taken;
}

long long __we_select_value(void *s) { return ((we_select *)s)->value; }
