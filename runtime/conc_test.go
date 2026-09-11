package weruntime

import "testing"

// M9b design D3–D6: the six primitive families answer for their contracts
// in C — semaphore counting with the over-release trap, channel buffering
// with rendezvous and close-drain semantics, cond predicate loops, the
// shared-state cells' direct access under single-threaded scheduling, and
// select's arm-taking with the no-residue teardown that F1's task_next
// ledger link guarantees. The embed variable carries the source, so a
// missing one fails here at compile time.

// concSemHarness pins semaphore counting: acquire succeeds, tryAcquire
// reports both directions across release, currentCount tracks, and the
// over-release inside a task turns into that task's panic outcome.
const concSemHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void *__we_prim_new_sem(long long n);
long long __we_sem_acquire(void *s);
long long __we_sem_try_acquire(void *s);
void __we_sem_release(void *s);
long long __we_sem_count(void *s);

static long long greedy(void *env) {
    void *s = __we_prim_new_sem(1);
    __we_sem_acquire(s);
    __we_sem_release(s);
    __we_sem_release(s); /* past the constructed count */
    printf("unreachable\n");
    return 0;
}

int we_main(void) {
    void *s = __we_prim_new_sem(2);
    printf("acquire=%lld\n", __we_sem_acquire(s));
    printf("try=%lld\n", __we_sem_try_acquire(s));
    __we_sem_release(s);
    printf("try2=%lld\n", __we_sem_try_acquire(s));
    printf("count=%lld\n", __we_sem_count(s));
    void *h = __we_task_new(greedy, 0);
    long long v = 0;
    long long tag = __we_handle_await(h, &v);
    printf("greedy=%lld msg=%s\n", tag, (const char *)v);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// concRendezvousHarness pins the capacity-zero pairing: a receiver parked
// first is handed the value directly by the sender, both sides complete,
// and the transfer survives as an ordinary rendezvous.
const concRendezvousHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_send(void *ch, long long v);
long long __we_chan_recv(void *ch, long long *out);

static void *g_ch;

static long long receiver(void *env) {
    long long v = 0;
    long long got = __we_chan_recv(g_ch, &v);
    printf("recv=%lld\n", got);
    printf("val=%lld\n", v);
    return 0;
}

static long long sender(void *env) {
    long long r = __we_chan_send(g_ch, 9);
    printf("sent=%lld\n", r);
    return 0;
}

int we_main(void) {
    g_ch = __we_prim_new_chan(0, 8, 0);
    void *hr = __we_task_new(receiver, 0);
    void *hs = __we_task_new(sender, 0);
    long long v = 0;
    __we_handle_await(hr, &v);
    __we_handle_await(hs, &v);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// concCloseDrainHarness pins close semantics: buffered values drain after
// close, receive reports None permanently once empty, and send after close
// inside a task turns into that task's panic outcome.
const concCloseDrainHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_send(void *ch, long long v);
long long __we_chan_recv(void *ch, long long *out);
long long __we_chan_close(void *ch);

static void *g_ch;

static long long late(void *env) {
    __we_chan_send(g_ch, 1);
    printf("unreachable\n");
    return 0;
}

int we_main(void) {
    g_ch = __we_prim_new_chan(4, 8, 0);
    __we_chan_send(g_ch, 7);
    printf("close=%lld\n", __we_chan_close(g_ch));
    long long v = 0;
    long long d = __we_chan_recv(g_ch, &v);
    printf("drain=%lld,%lld\n", d, v);
    printf("empty=%lld\n", __we_chan_recv(g_ch, &v));
    void *h = __we_task_new(late, 0);
    long long out = 0;
    long long tag = __we_handle_await(h, &out);
    printf("late=%lld msg=%s\n", tag, (const char *)out);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// concCondHarness pins the predicate loop: waiters parked on a false
// predicate stay parked through a signal, and a broadcast paired with the
// state change wakes them to re-check and proceed.
const concCondHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void *__we_prim_new_mutex(long long v);
void *__we_prim_new_cond(void *mtx);
long long __we_cond_wait(void *c, long long (*pred)(void *, long long), void *env);
void __we_cond_signal(void *c);
void __we_cond_broadcast(void *c);
void __we_prim_set(void *cell, long long v);

static void *g_mtx;
static void *g_cond;

static long long positive(void *env, long long v) { return v > 0; }

static long long w1(void *env) {
    __we_cond_wait(g_cond, positive, 0);
    printf("w1\n");
    return 0;
}

static long long w2(void *env) {
    __we_cond_wait(g_cond, positive, 0);
    printf("w2\n");
    return 0;
}

static long long setter(void *env) {
    __we_cond_signal(g_cond); /* predicate still false: nobody proceeds */
    __we_prim_set(g_mtx, 2);
    __we_cond_broadcast(g_cond);
    printf("set\n");
    return 0;
}

int we_main(void) {
    g_mtx = __we_prim_new_mutex(0);
    g_cond = __we_prim_new_cond(g_mtx);
    void *h1 = __we_task_new(w1, 0);
    void *h2 = __we_task_new(w2, 0);
    void *hs = __we_task_new(setter, 0);
    long long v = 0;
    __we_handle_await(h1, &v);
    __we_handle_await(h2, &v);
    __we_handle_await(hs, &v);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// concCellHarness pins the shared-state cells' direct access: update runs
// its callback against the stored value, set/get round-trip, and RwLock's
// read callback projects to a new value.
const concCellHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_prim_new_mutex(long long v);
void *__we_prim_new_rwlock(long long v);
void *__we_prim_new_atomic(long long v);
long long __we_prim_update(void *cell, long long (*f)(void *, long long), void *env);
long long __we_prim_get(void *cell);
void __we_prim_set(void *cell, long long v);
long long __we_prim_read(void *cell, long long (*f)(void *, long long), void *env);

static long long add_one(void *env, long long v) { return v + 1; }
static long long twice(void *env, long long v) { return v * 2; }

int we_main(void) {
    void *m = __we_prim_new_mutex(10);
    printf("u1=%lld\n", __we_prim_update(m, add_one, 0));
    printf("u2=%lld\n", __we_prim_update(m, add_one, 0));
    __we_prim_set(m, 7);
    printf("get=%lld\n", __we_prim_get(m));
    void *rw = __we_prim_new_rwlock(3);
    printf("read=%lld\n", __we_prim_read(rw, twice, 0));
    void *a = __we_prim_new_atomic(1);
    printf("atom=%lld\n", __we_prim_get(a));
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// concSelectHarness pins select's register/park/arm protocol: the fast path
// takes the first ready source in case order, the true park wakes on a
// later send, and a source left behind by arm-taking carries no residue —
// the value sent after the arm-taken delivery still pairs with the next
// blocking receive on that channel.
const concSelectHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
long long __we_yield(void);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_send(void *ch, long long v);
long long __we_chan_recv(void *ch, long long *out);
void *__we_select_new(void);
void __we_select_add_recv(void *sel, void *ch);
long long __we_select_park(void *sel);
long long __we_select_value(void *sel);

static void *g_a;

static long long feeder(void *env) {
    __we_yield(); /* let task 0 register and park first */
    __we_chan_send(g_a, 9);
    printf("fed\n");
    __we_chan_send(g_a, 8); /* past the select: the residue probe */
    return 0;
}

int we_main(void) {
    void *a = __we_prim_new_chan(1, 8, 0);
    void *b = __we_prim_new_chan(1, 8, 0);
    __we_chan_send(b, 2);
    void *sel = __we_select_new();
    __we_select_add_recv(sel, a);
    __we_select_add_recv(sel, b);
    long long arm = __we_select_park(sel);
    printf("fast-arm=%lld v=%lld\n", arm, __we_select_value(sel));

    g_a = __we_prim_new_chan(1, 8, 0);
    void *a2 = g_a;
    void *b2 = __we_prim_new_chan(1, 8, 0);
    void *sel2 = __we_select_new();
    __we_select_add_recv(sel2, a2);
    __we_select_add_recv(sel2, b2);
    void *h = __we_task_new(feeder, 0);
    long long arm2 = __we_select_park(sel2);
    printf("park-arm=%lld v=%lld\n", arm2, __we_select_value(sel2));

    long long v = 0;
    long long got = __we_chan_recv(a2, &v); /* no residue: 8 still arrives */
    printf("after=%lld,%lld\n", got, v);
    long long w = 0;
    __we_handle_await(h, &w);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM9bConcSemaphore(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"str.h": StrHeader, "conc.c": ConcSource, "test.c": TestSource,
			"str.c": StrSource, "main.c": concSemHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"},
		"acquire=0\ntry=1\ntry2=1\ncount=0\ngreedy=1 msg=semaphore released past its constructed count\n")
}

func TestM9bConcRendezvous(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"str.h": StrHeader, "conc.c": ConcSource, "test.c": TestSource,
			"str.c": StrSource, "main.c": concRendezvousHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"},
		"sent=0\nrecv=1\nval=9\n")
}

func TestM9bConcCloseDrain(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"str.h": StrHeader, "conc.c": ConcSource, "test.c": TestSource,
			"str.c": StrSource, "main.c": concCloseDrainHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"},
		"close=0\ndrain=1,7\nempty=0\nlate=1 msg=send on a closed channel\n")
}

func TestM9bConcCondWaitSignal(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"str.h": StrHeader, "conc.c": ConcSource, "test.c": TestSource,
			"str.c": StrSource, "main.c": concCondHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"},
		"set\nw1\nw2\n")
}

func TestM9bConcCells(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"str.h": StrHeader, "conc.c": ConcSource, "test.c": TestSource,
			"str.c": StrSource, "main.c": concCellHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"},
		"u1=11\nu2=12\nget=7\nread=6\natom=1\n")
}

func TestM9bConcSelect(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"str.h": StrHeader, "conc.c": ConcSource, "test.c": TestSource,
			"str.c": StrSource, "main.c": concSelectHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"},
		"fast-arm=1 v=2\nfed\npark-arm=0 v=9\nafter=1,8\n")
}
