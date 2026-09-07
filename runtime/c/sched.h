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

// One scope frame (chapter 18's scope forms). deadline_ms is absolute
// CLOCK_MONOTONIC milliseconds, -1 without a timeout clause.
typedef struct we_scope {
    struct we_scope *parent;
    long long deadline_ms;
    long long pending;  // unfinished tasks created inside
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

// The face conc.c builds on.
we_task *__we_cur_task(void);
void __we_task_park(void);      // suspend the current (registered) task
void __we_task_wake(we_task *t); // PARKED -> READY (idempotent no-op else)
int __we_cur_cancelled(void);
we_wait_link *__we_link_new(void);
void __we_link_attach(we_wait_link *l); // init a family's extended link onto the ledger
void __we_link_free(we_wait_link *l);

// The root-window face gc.c provides (design D7).
struct we_gc_window *__we_gc_window_new(void);
void __we_gc_window_retire(struct we_gc_window *w);
void __we_gc_window_swap(struct we_gc_window *w);

#endif
