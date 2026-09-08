// The We runtime's cooperative scheduler (chapter 18's scheduling
// promises; M9b design D1/D2/D4). One thread, N tasks: every task owns a
// ucontext and a malloc'd stack, task 0 wraps the emitted __we_main as an
// ordinary task, and the scheduler loop runs on its own stack so any task
// — task 0 included — can park beneath it. The ready queue is a single
// FIFO; chapter 18 leaves interleavings unspecified and this carries no
// promise beyond eventual progress. Idle means one of two honest states:
// a future deadline exists (sleep to it, then expire scopes and wake
// their tasks for the cooperative return) or none does — every task
// parked with no wake source, reported on stderr with exit 70.
//
// Cancellation is cooperative throughout: setting cancel_flag and waking
// a parked task is the whole mechanism; the woken primitive call sees the
// flag and returns its empty form (design's cancel-wake returns). Links
// die lazily — a cancelled task's registrations stay on their source
// queues flagged dead, and source operations skip and reap them — so no
// cancel path needs to know which family it interrupts.
//
// M10b (design D4) splits the clock in two: a run inside the test extent
// holds a virtual clock that moves only when a runnable task advances it,
// so scope deadlines taken there and the sleep waits test.c parks ride
// that clock, and the idle loop never sleeps toward a virtual deadline —
// idle with no real wake source under the virtual clock is the test
// domain's own deadlock, reported with the advanceTime line and exit 1.
#define _XOPEN_SOURCE 700

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <ucontext.h>

#include "sched.h"

#define TASK_STACK ((64 + 192) * 1024) // 256 KiB per task
#define SCHED_STACK (64 * 1024)

static we_task *ready_head, *ready_tail;
static we_task *drain_head, *drain_tail; // advanceTime's barrier queue
static we_task *all_tasks;
static we_scope *active_scopes;
static we_task *task0;
static we_task *cur_task; // NULL only inside the scheduler loop
static ucontext_t sched_ctx;
static char sched_stack[SCHED_STACK];

// The makecontext hand-off: int params are the only portable currency, so
// the trampoline resolves its task through this growing table instead of
// a single global (consecutive task_new calls would clobber one slot).
static we_task **task_table;
static int ntask_slots;

static long long now_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long long)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}

static void ready_push(we_task *t) {
    t->qnext = NULL;
    if (ready_tail) {
        ready_tail->qnext = t;
    } else {
        ready_head = t;
    }
    ready_tail = t;
}

static we_task *ready_pop(void) {
    we_task *t = ready_head;
    if (t) {
        ready_head = t->qnext;
        if (!ready_head) {
            ready_tail = NULL;
        }
    }
    return t;
}

static void all_push(we_task *t) {
    t->next = all_tasks;
    all_tasks = t;
}

// Take a queued task out of the ready queue (the abandonment sweep's READY
// face): singly linked, so the walk keeps the predecessor for the tail.
static void ready_unlink(we_task *t) {
    we_task *prev = NULL;
    for (we_task *c = ready_head; c; prev = c, c = c->qnext) {
        if (c == t) {
            if (prev) {
                prev->qnext = c->qnext;
            } else {
                ready_head = c->qnext;
            }
            if (ready_tail == t) {
                ready_tail = prev;
            }
            t->qnext = NULL;
            return;
        }
    }
}

we_task *__we_cur_task(void) { return cur_task; }

int __we_cur_cancelled(void) { return cur_task && cur_task->cancel_flag; }

// The test boundary's sweep material (M10b design D4): the whole-task
// ledger head, and the task-table count a begin records as its baseline.
we_task *__we_all_tasks(void) { return all_tasks; }

int __we_task_table_count(void) { return ntask_slots; }

// The current task suspends. The caller has already registered its wait
// links (or is the scope-leave joiner, which registers none).
void __we_task_park(void) {
    cur_task->state = WE_PARKED;
    swapcontext(&cur_task->ctx, &sched_ctx);
}

// PARKED -> READY. Waking a task that is not parked is a no-op: the task
// already holds its delivery (or was never parked), and double-waking
// would corrupt the ready queue.
void __we_task_wake(we_task *t) {
    if (t->state != WE_PARKED) {
        return;
    }
    t->state = WE_READY;
    ready_push(t);
}

we_wait_link *__we_link_new(void) {
    we_wait_link *l = calloc(1, sizeof *l);
    if (!l) {
        abort();
    }
    __we_link_attach(l);
    return l;
}

// A primitive family embeds we_wait_link as the first field of its own
// registration node; this puts that node on the owner's ledger.
void __we_link_attach(we_wait_link *l) {
    l->owner = cur_task;
    l->task_next = cur_task->links;
    cur_task->links = l;
}

void __we_link_free(we_wait_link *l) {
    // Detach from the owner's ledger (threaded by task_next); the source
    // queue drops dead links lazily as it walks them.
    we_wait_link **p = &l->owner->links;
    while (*p && *p != l) {
        p = &(*p)->task_next;
    }
    if (*p) {
        *p = l->task_next;
    }
    free(l);
}

// Mark every registration of the task dead (lazy invalidation): source
// operations skip dead links when deciding whom to wake, and reap them
// from their queues as they walk.
static void fail_links(we_task *t) {
    for (we_wait_link *l = t->links; l; l = l->task_next) {
        l->dead = 1;
    }
}

// The clock-crossing barrier (M10b design D4/D9): advanceTime parks here
// before it moves the virtual clock, and the scheduler answers it only
// when the ready queue is empty — every runnable task has run to its next
// block, so every wait that wants the crossing is registered first. FIFO
// in park order; nested advances (a task advancing while another waits at
// its own barrier) drain in the order they arrived. WE_DRAIN keeps the
// queue disjoint from wake's (a cancelled drain waiter stays queued: the
// flag rides home with it and the body's next block answers the empty
// form).
void __we_sched_drain(void) {
    cur_task->state = WE_DRAIN;
    cur_task->qnext = NULL;
    if (drain_tail) {
        drain_tail->qnext = cur_task;
    } else {
        drain_head = cur_task;
    }
    drain_tail = cur_task;
    swapcontext(&cur_task->ctx, &sched_ctx);
}

// The test boundary's sweep (M10b design D4): permanently abandon a task
// the test leaves behind. This is not a cancellation — a cancelled task
// wakes and answers its empty form, so its body's remaining statements
// still run; an abandoned one is taken out of scheduling entirely (out of
// the ready queue if queued, off the barrier queue if parked at one, and
// a parked one simply never resumes). Its registrations die so no source
// ever serves it; the task and its stack stay on the ledger (the gc
// window keeps its roots — the bounded-leak face, follow-up #11).
void __we_task_abandon(we_task *t) {
    fail_links(t);
    if (t->state == WE_READY) {
        ready_unlink(t);
    } else if (t->state == WE_DRAIN) {
        we_task *prev = NULL;
        for (we_task *c = drain_head; c; prev = c, c = c->qnext) {
            if (c == t) {
                if (prev) {
                    prev->qnext = c->qnext;
                } else {
                    drain_head = c->qnext;
                }
                if (drain_tail == t) {
                    drain_tail = prev;
                }
                t->qnext = NULL;
                break;
            }
        }
    }
    t->state = WE_ABANDONED;
}

// Wake for cancellation: the cooperative-return path. The woken primitive
// sees cancel_flag and answers its empty form. A parked select fires its
// first arm instead (an unspecified-but-safe choice — chapter 18 leaves
// the arm unspecified and the body's blocking calls all answer empty
// forms from here on).
static void cancel_wake(we_task *t) {
    t->cancel_flag = 1;
    if (t->state != WE_PARKED) {
        return;
    }
    we_wait_link *arm = NULL;
    for (we_wait_link *l = t->links; l; l = l->task_next) {
        l->dead = 1;
        if (!arm && l->complete) {
            arm = l;
        }
    }
    if (arm) {
        arm->dead = 0; // this one completes: it fires the arm and wakes
        arm->complete(arm);
        return;
    }
    __we_task_wake(t);
}

static void cancel_scope_tasks(we_scope *sc) {
    for (we_task *t = all_tasks; t; t = t->next) {
        if (t->scope == sc && t->state != WE_DONE) {
            cancel_wake(t);
        }
    }
}

// A task finished (normal return or __we_task_fail). Store the outcome,
// wake joiners, settle the scope accounting, and never run again.
static void task_done(we_task *t) {
    t->state = WE_DONE;
    __we_gc_window_retire(t->gcwin); // stale roots stop marking forever
    if (t->is_main) {
        // The process answer: Ok maps to the exit code, a failed main is
        // chapter 14's abort face (one report line, exit 1 — the task's
        // exit_code register stays 0 on the fail path, so the code is
        // settled here, not in __we_task_fail).
        if (!t->has_result) {
            fprintf(stderr, "error: Panicked: %s\n", t->panic_msg);
            exit(1);
        }
        exit((int)t->exit_code);
    }
    for (we_wait_link *l = t->awaiters; l;) {
        we_wait_link *next = l->next;
        if (!l->dead) {
            if (l->complete) {
                l->complete(l); // a select arm fires itself, then wakes
            } else {
                __we_task_wake(l->owner);
            }
        }
        __we_link_free(l); // wakes were one-shot: reap the registration
        l = next;
    }
    t->awaiters = NULL;
    we_scope *sc = t->scope;
    if (sc) {
        sc->pending--;
        // A cancelled task's panic is captured at the task boundary and
        // discarded (chapter 18: the scope chose to abandon the result —
        // no path re-opens it; a later await still reads the stored
        // outcome, so only the scope's own propagation skips it).
        if (!t->has_result && !t->cancel_flag && !sc->collect_all && !sc->panicked) {
            // A plain scope fails fast: cancel the siblings and hand the
            // panic to the owner's leave (which propagates it).
            sc->panicked = 1;
            sc->panic_msg = t->panic_msg;
            cancel_scope_tasks(sc);
        }
        if (sc->pending == 0 && sc->owner->state == WE_PARKED) {
            __we_task_wake(sc->owner);
        }
    }
    swapcontext(&t->ctx, &sched_ctx);
    abort(); // a finished task never resumes
}

static void trampoline(int slot) {
    we_task *t = task_table[slot];
    long long r = t->thunk(t->env);
    t->exit_code = r;
    t->has_result = 1;
    task_done(t);
}

// The reported failure of the calling task: defer-inversion is the
// emitter's job (the body's own tail), so this only stores the outcome
// and settles.
void __we_task_fail(const char *msg) {
    char *copy = malloc(strlen(msg) + 1);
    if (!copy) {
        abort();
    }
    strcpy(copy, msg);
    cur_task->panic_msg = copy;
    cur_task->has_result = 0;
    task_done(cur_task);
}

static we_task *task_alloc(long long (*thunk)(void *), void *env) {
    we_task *t = calloc(1, sizeof *t);
    if (!t) {
        abort();
    }
    t->gcwin = __we_gc_window_new();
    t->thunk = thunk;
    t->env = env;
    t->stack = malloc(TASK_STACK);
    if (!t->stack) {
        abort();
    }
    t->slot = ntask_slots;
    if (ntask_slots % 16 == 0) {
        we_task **grown = realloc(task_table, (size_t)(ntask_slots + 16) * sizeof *grown);
        if (!grown) {
            abort();
        }
        task_table = grown;
    }
    task_table[ntask_slots++] = t;
    getcontext(&t->ctx);
    t->ctx.uc_stack.ss_sp = t->stack;
    t->ctx.uc_stack.ss_size = TASK_STACK;
    t->ctx.uc_link = NULL;
    // POSIX prototypes the entry as void(void) but documents int params —
    // the cast is the sanctioned shape (proposal's known-risk note).
    makecontext(&t->ctx, (void (*)(void))trampoline, 1, t->slot);
    all_push(t);
    return t;
}

void *__we_task_new(long long (*thunk)(void *), void *env) {
    we_task *t = task_alloc(thunk, env);
    t->scope = cur_task ? cur_task->scope : NULL;
    if (t->scope) {
        t->scope->pending++;
    }
    t->state = WE_READY;
    ready_push(t);
    return t;
}

long long __we_yield(void) {
    cur_task->state = WE_READY;
    ready_push(cur_task);
    swapcontext(&cur_task->ctx, &sched_ctx);
    return 0;
}

// Await: the caller parks until the target task is DONE. Cancellation
// does not interrupt an await — the target's own cooperative return (or
// completion) is what wakes the joiner, so a cancelled joiner still
// observes the task's real outcome (chapter 18's awaited-cancelled tasks
// return cooperatively; the honest-deadlock face covers the rest).
long long __we_handle_await(void *h, long long *payload) {
    we_task *t = h;
    while (t->state != WE_DONE) {
        we_wait_link *l = __we_link_new();
        l->next = t->awaiters;
        t->awaiters = l;
        __we_task_park();
    }
    if (t->has_result) {
        *payload = t->exit_code;
        return 0;
    }
    *payload = (long long)(unsigned long long)t->panic_msg;
    return 1;
}

long long __we_handle_cancel(void *h) {
    we_task *t = h;
    if (t->state != WE_DONE) {
        cancel_wake(t);
    }
    return 0;
}

void *__we_scope_enter(long long deadline_ms, long long collect_all) {
    we_scope *sc = calloc(1, sizeof *sc);
    if (!sc) {
        abort();
    }
    sc->parent = cur_task->scope;
    cur_task->scope = sc;
    // The deadline rides the clock the run is under (M10b design D4): a
    // scope entered inside the test extent parks on the virtual clock,
    // every other scope on the wall.
    sc->virtual = __we_test_virtual();
    sc->deadline_ms = deadline_ms >= 0
        ? (sc->virtual ? __we_test_vnow() : now_ms()) + deadline_ms
        : -1;
    sc->collect_all = (int)collect_all;
    sc->owner = cur_task;
    sc->next = active_scopes;
    active_scopes = sc;
    return sc;
}

// Leave: join every task created inside. On timeout expiry the scope has
// already cancelled them; the join waits out their cooperative returns
// either way (chapter 18: waits for their cooperative return). The value
// is 0 Ok / 1 TimedOut — first to finish wins: pending reaching zero
// before expiry reads Ok even if the deadline then passes.
long long __we_scope_leave(void *scope) {
    we_scope *sc = scope;
    while (sc->pending > 0) {
        __we_task_park();
    }
    long long timed_out = sc->timed_out && sc->deadline_ms >= 0 ? 1 : 0;
    char *panic = sc->panicked ? sc->panic_msg : NULL;
    cur_task->scope = sc->parent;
    we_scope **p = &active_scopes;
    while (*p && *p != sc) {
        p = &(*p)->next;
    }
    if (*p) {
        *p = sc->next;
    }
    free(sc);
    if (panic) {
        __we_task_fail(panic); // a plain scope propagates its task's panic
    }
    return timed_out;
}

// Expire due scopes: mark, cancel their unfinished tasks (the cooperative
// returns drain pending), and the joiner wakes when the last one settles.
// Each scope reads its own clock (M10b design D4): a virtual scope expires
// against the virtual clock — so an advance can expire it — while every
// other scope reads the wall. Non-static: the advance face pushes an
// expiry after it moves the virtual clock, ahead of any scheduling.
void __we_expire_deadlines(void) {
    long long wall = now_ms();
    for (we_scope *sc = active_scopes; sc; sc = sc->next) {
        long long now = sc->virtual ? __we_test_vnow() : wall;
        if (sc->deadline_ms >= 0 && !sc->timed_out && sc->pending > 0 &&
            sc->deadline_ms <= now) {
            sc->timed_out = 1;
            cancel_scope_tasks(sc);
        }
    }
}

// The idle loop's sleep target: real-clock scopes only — a virtual
// deadline is never slept toward (the virtual clock moves from runnable
// tasks, never from the wall).
static long long earliest_deadline(void) {
    long long best = -1;
    for (we_scope *sc = active_scopes; sc; sc = sc->next) {
        if (!sc->virtual && sc->deadline_ms >= 0 && !sc->timed_out && sc->pending > 0 &&
            (best < 0 || sc->deadline_ms < best)) {
            best = sc->deadline_ms;
        }
    }
    return best;
}

static long long count_parked(void) {
    long long n = 0;
    for (we_task *t = all_tasks; t; t = t->next) {
        if (t->state == WE_PARKED) {
            n++;
        }
    }
    return n;
}

static void sched_run(void) {
    for (;;) {
        __we_expire_deadlines();
        __we_sleep_expire(); // due real sleeps release alongside the scopes
        we_task *t = ready_pop();
        if (!t) {
            if (drain_head) {
                // The barrier answers only an empty ready queue: the
                // crossing may proceed (the waiter moves the clock, its
                // release re-fills this loop).
                t = drain_head;
                drain_head = t->qnext;
                if (!drain_head) {
                    drain_tail = NULL;
                }
                t->qnext = NULL;
                t->state = WE_READY;
                ready_push(t);
                continue;
            }
            long long ddl = earliest_deadline();
            long long sd = __we_sleep_earliest_deadline();
            if (sd >= 0 && (ddl < 0 || sd < ddl)) {
                ddl = sd;
            }
            if (ddl < 0) {
                // Idle with no future event. Under the test's virtual
                // clock (M10b design D4) no wall deadline exists to sleep
                // toward — progress needs a runnable task to advance the
                // clock — so this is the test domain's own deadlock: one
                // line naming the virtual clock, exit 1 (the count is the
                // clock-parked tasks; a driver parked on a join is not a
                // clock wait, and with no clock waiter at all every parked
                // task counts). Outside that extent this is the honest
                // M9b face: every remaining task parked with nothing that
                // will ever wake it, exit 70.
                long long parked = count_parked();
                if (__we_test_virtual()) {
                    long long clocked = __we_virtual_parked();
                    fprintf(stderr, "we: deadlock: %lld tasks parked with no wake source"
                                    " (the virtual clock only advances when a runnable task"
                                    " calls advanceTime)\n",
                            clocked > 0 ? clocked : parked);
                    exit(1);
                }
                fprintf(stderr, "we: deadlock: %lld tasks parked with no wake source\n",
                        parked);
                exit(70);
            }
            struct timespec ts = {ddl / 1000, (ddl % 1000) * 1000000};
            clock_nanosleep(CLOCK_MONOTONIC, TIMER_ABSTIME, &ts, NULL);
            continue;
        }
        t->state = WE_RUNNING;
        cur_task = t;
        __we_gc_window_swap(t->gcwin); // roots push into this task's window
        swapcontext(&sched_ctx, &t->ctx);
        cur_task = NULL;
        __we_gc_window_swap(NULL); // scratch: the loop itself roots nothing
    }
}

static long long main_thunk(void *env) {
    int (*main_fn)(void) = (int (*)(void))env;
    return main_fn();
}

void __we_sched_boot(int (*main_fn)(void)) {
    extern void __we_gc_boot(void);
    __we_gc_boot();
    __we_gc_window_swap(NULL); // pre-task pushes land in the scratch window
    task0 = task_alloc(main_thunk, main_fn);
    task0->is_main = 1;
    task0->state = WE_READY;
    ready_push(task0);
    getcontext(&sched_ctx);
    sched_ctx.uc_stack.ss_sp = sched_stack;
    sched_ctx.uc_stack.ss_size = SCHED_STACK;
    sched_ctx.uc_link = NULL;
    makecontext(&sched_ctx, sched_run, 0);
    setcontext(&sched_ctx);
    abort(); // setcontext does not return
}
