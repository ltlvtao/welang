// Internal shared face of the We runtime's cooperative scheduler (M9b
// design D1/D2). sched.c owns these structures; conc.c (the wait machine
// and primitive families) links against the ABI below. The emitted code
// and the C harnesses only ever see the __we_-prefixed entry points.
#ifndef WE_SCHED_H
#define WE_SCHED_H

#include <ucontext.h>

typedef enum {
    WE_READY = 1,
    WE_RUNNING = 2,
    WE_PARKED = 3,
    WE_DONE = 4,
    WE_DRAIN = 5,     // parked at advanceTime's barrier (test.c's clock face)
    WE_ABANDONED = 6, // the test boundary's sweep: never scheduled again
} we_task_state;

struct we_scope;
struct we_gc_window; // gc.c's per-task root window (swap target, own chain)

// One cooperative task. Task 0 (is_main) wraps the emitted __we_main and
// lives on its own stack like every other task, so the main body can park
// under the scheduler's own context (joins are blocking calls).
typedef struct we_task {
    ucontext_t ctx;
    char *stack; // malloc'd; NULL never happens past boot
    long long (*thunk)(void *env);
    void *env;
    int slot; // index into the scheduler's task table (the makecontext
              // hand-off: int params are the only portable currency)
    we_task_state state;

    long long exit_code;    // thunk outcome: the Ok payload
    int has_result;         // 0 = the task failed (panic_msg carries why)
    char *panic_msg;        // malloc'd copy: outlives the task's stack
    int cancel_flag;        // cooperative cancel requested (scope expiry or handle)
    int is_main;

    // Per-task GC root window (design D7): the scheduler swaps this in at
    // every task entry, so a parked task's roots stay reachable to a
    // collection run by any other task.
    struct we_gc_window *gcwin;

    struct we_wait_link *links; // ledger of every queue this task is parked in
    struct we_wait_link *awaiters; // joiners parked on this task's outcome
    void *sig;                  // lazily built CancelSignal (conc.c's face)
    struct we_scope *scope;     // the scope frame counting this task, if any
    struct we_task *qnext;      // ready-queue link
    struct we_task *next;       // all-tasks ledger link
} we_task;

// A registration on one wait source's queue (design D2, F1's fix): next
// threads the source's queue, task_next threads the owner's ledger, so a
// cancel can invalidate every registration without knowing the sources.
typedef struct we_wait_link {
    we_task *owner;
    struct we_wait_link *next;
    struct we_wait_link *task_next;
    int dead; // set by cancel: source operations skip and reap dead links
    // A registration served by a select arm completes through this hook
    // instead of a plain wake (the arm must fire before the owner runs);
    // NULL on ordinary registrations.
    void (*complete)(struct we_wait_link *);
} we_wait_link;

// One scope frame (chapter 18's scope forms). deadline_ms is an absolute
// millisecond reading in its own clock's domain, -1 without a timeout
// clause: scopes entered under the test's virtual clock (the virtual flag)
// carry their deadline on that clock — it moves only when a runnable task
// advances it — while every other scope rides CLOCK_MONOTONIC.
typedef struct we_scope {
    struct we_scope *parent;
    long long deadline_ms;
    long long pending;  // unfinished tasks created inside
    int virtual;        // the deadline rides the test's virtual clock
    int timed_out;
    int collect_all;    // join across panics instead of fail-fast
    int panicked;       // a plain-scope task failed: propagate to the owner
    char *panic_msg;
    we_task *owner;
    struct we_scope *next; // active-scope ledger (deadline scan)
} we_scope;

// Entry points the emitted code and the harnesses call.
void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_yield(void);
void __we_task_fail(const char *msg); // noreturn for the calling task
long long __we_handle_await(void *h, long long *payload);
long long __we_handle_cancel(void *h);
void *__we_scope_enter(long long deadline_ms, long long collect_all);
long long __we_scope_leave(void *scope);
// The piercing-exit face (chapter 18: an early return, break, or continue
// through an open scope discharges the scope's remaining task handles):
// mark the scope's deadline-expired state and cancel its unfinished tasks
// (the cooperative-return path); the joiner's own leave waits them out.
void __we_scope_cancel(void *scope);

// The face conc.c builds on.
we_task *__we_cur_task(void);
void __we_task_park(void);      // suspend the current (registered) task
void __we_task_wake(we_task *t); // PARKED -> READY (idempotent no-op else)
int __we_cur_cancelled(void);
we_wait_link *__we_link_new(void);
void __we_link_attach(we_wait_link *l); // init a family's extended link onto the ledger
void __we_link_free(we_wait_link *l);

// The face test.c builds on (M10b design D4): the task ledger and table
// count for the boundary's leftover sweep, the deadline expiry advance
// pushes for the virtual scopes, the clock-crossing barrier advance parks
// at (every ready task runs to its next block before the clock moves, so
// every wait wanting the crossing is registered first), and the sweep's
// abandonment — a leftover task is never cancelled back into the run, it
// is taken out of scheduling entirely.
we_task *__we_all_tasks(void);
int __we_task_table_count(void);
void __we_expire_deadlines(void);
void __we_sched_drain(void);
void __we_task_abandon(we_task *t);

// The clock face test.c owns and the scheduler reads: which clock the run
// is under, its reading, the clock-parked count for the abort line, and
// the real-clock sleep waits folded into the idle loop.
int __we_test_virtual(void);
long long __we_test_vnow(void);
long long __we_virtual_parked(void);
long long __we_sleep_earliest_deadline(void);
void __we_sleep_expire(void);

// The exploration face (M10c design D2/D3/D4). test.c owns the globals
// (startup.c's argv sets them; the harnesses set them directly); sched.c
// owns the armed pick policy and the splitmix64 stream; test.c owns the
// trace. __we_explore_arm hands one attempt its schedule: baseline keeps
// the FIFO face (iteration zero, and every normal run — nothing else ever
// arms), any other arming picks a seeded random index off the ready
// queue. The barrier queue and the clock's same-instant release stay
// FIFO either way — chapter 20 pinned those, exploration only widens the
// scheduler's own choice.
extern int __we_explore_on;
extern long long __we_explore_iters;
extern int __we_explore_reduce;
void __we_explore_arm(unsigned long long seed, int baseline);
long long __we_ready_count(void);

// splitmix64: the pick stream and the seed derivation. __we_rng_next is
// the canonical generator (state advances by the golden-ratio constant,
// the output is the mixed state); __we_explore_seed mixes the attempt's
// identity (test ordinal n, iteration i, attempt a) into one seed — the
// same triple always arms the same schedule.
void __we_rng_seed(unsigned long long s);
unsigned long long __we_rng_next(void);
unsigned long long __we_explore_seed(long long n, long long i, long long a);

// The observable trace (design D4): three events, recorded only while an
// explored run is in flight. The actor is the task the event belongs to
// (the sender of a completed send even when the receiver's loop completes
// it); the target is the channel's per-run creation ordinal for the
// channel pair, the completing task's own id for a completion. Ids are
// normalized per run (task ids relative to the extent's spawn base, chan
// ids from a counter zeroed with each run) so the double-run pair
// compares equal on a deterministic test.
enum { WE_EV_SEND = 0, WE_EV_RECV = 1, WE_EV_DONE = 2 };
void __we_trace_event(int kind, long long target, we_task *actor);
long long __we_trace_alloc_id(void);
long long __we_trace_task_id(we_task *t); // the id a task carries in the current run

// The verdict faces (M10c design D5/D6/D7/D8). E1901: the pair comparator
// — pure, over stride-four quadruples (task, kind, target, seq), first
// divergence's index or -1 — and the render (e1/e2 are the diverging
// quadruples, NULL for the side whose trace ended; json picks the
// protocol face; iteration and attempt arrive 1-based). The fxgate's
// single point of judgment and its recorded reason; the POR class key
// (the truncation flag rides); and the explored deadlock's settle
// hand-off — sched.c's idle loop sets both, handle_await consumes the
// flag and answers the message as the await's failure payload.
long long __we_e1901_first_div(const long long *q0, long long n0,
                               const long long *q1, long long n1);
void __we_e1901_render(long long iteration, long long attempt, long long k,
                       const long long *e1, const long long *e2, int json);
void __we_explore_fx_check(const char *name, long long len);
const char *__we_explore_guard_reason(void);
unsigned long long __we_por_key(const long long *q, long long n, int truncated);
extern int __we_explore_settle_pending;
extern char __we_explore_settle_msg[128];

// The root-window face gc.c provides (design D7).
struct we_gc_window *__we_gc_window_new(void);
void __we_gc_window_retire(struct we_gc_window *w);
void __we_gc_window_swap(struct we_gc_window *w);

#endif
