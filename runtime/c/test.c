// The We runtime's testing face (chapter 20's run tower; M10b design
// D4/D7): the virtual clock, the test boundary, and the assertion helpers.
// Between __we_test_begin and __we_test_end the clock is virtual — an
// Int64 millisecond counter that moves only when a runnable task calls
// __we_advance (advanceTime's runtime face), so a test's timing is
// deterministic however long its sleeps. Outside that extent the wall
// clock rules: __we_time_now reads CLOCK_MONOTONIC, __we_time_sleep parks
// on a real deadline the scheduler's idle loop sleeps toward. The sleep
// wait is the one new wait source — a FIFO queue of registrations, each
// carrying its own clock's absolute deadline; the same-instant release
// wakes them in registration order (design D4).
//
// The boundary sweeps: end abandons every task spawned inside the extent
// that never finished — their registrations die and they are taken out of
// scheduling entirely (a cancelled task would wake and run its remaining
// statements; an abandoned one never runs again), so the next test begins
// from a quiet task face and a zero clock. Assertions ride the ordinary
// task-fail ABI (the driver's await reads the stored message out; the
// report lines are design D7's wording). The report face renders each
// test's line and the summary — human bytes or chapter 21 R7's JSON
// Lines, the argv --json flag choosing — and the summary owns the exit
// code.
#define _XOPEN_SOURCE 700

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "sched.h"
#include "str.h"

// The clock state. mode is real (0) until a test begins; vnow is the
// virtual clock, milliseconds since the extent began; the task baseline is
// the table count at begin — the sweep's dividing line for leftovers.
// vstart/vdur carry one test's duration (the begin/end virtual diff —
// deterministic, chapter 20's clock being the test's own).
static int test_virtual;
static long long test_vnow;
static int test_task_base;
static long long test_vstart, test_vdur;

int __we_test_virtual(void) { return test_virtual; }

long long __we_test_vnow(void) { return test_vnow; }

static long long wall_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long long)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}

// --- the sleep wait source ---------------------------------------------------

// One sleep registration: the scheduler's wait link (so cancellation
// invalidates it like every other registration) plus the deadline and the
// clock it belongs to. qnext threads the queue in registration order; the
// queue is singly linked and short (a test's parked sleeps), so the unlink
// walks from the head and stays idempotent — the waker may already have
// detached the node when the sleeper comes to settle it.
typedef struct we_sleep_wait {
    we_wait_link base;
    long long deadline; // absolute ms in its own clock's domain
    int virtual;        // which clock the deadline rides
    struct we_sleep_wait *qnext;
} we_sleep_wait;

static we_sleep_wait *sleep_head;

static void sleep_unlink(we_sleep_wait *w) {
    we_sleep_wait **p = &sleep_head;
    while (*p && *p != w) {
        p = &(*p)->qnext;
    }
    if (*p) {
        *p = w->qnext;
    }
}

// The earliest outstanding real-clock sleep deadline, -1 without one: the
// scheduler's idle loop folds this into its sleep target. Virtual sleeps
// never enter — the virtual clock moves only from a runnable task.
long long __we_sleep_earliest_deadline(void) {
    long long best = -1;
    for (we_sleep_wait *w = sleep_head; w; w = w->qnext) {
        if (!w->virtual && !w->base.dead && (best < 0 || w->deadline < best)) {
            best = w->deadline;
        }
    }
    return best;
}

// Release due real-clock sleeps. The idle loop calls this after its own
// clock_nanosleep; the waker unlinks the node and wakes the owner, the
// owner (back from park) settles the memory.
void __we_sleep_expire(void) {
    long long now = wall_ms();
    for (we_sleep_wait *w = sleep_head; w;) {
        we_sleep_wait *next = w->qnext;
        if (!w->virtual && !w->base.dead && w->deadline <= now) {
            sleep_unlink(w);
            __we_task_wake(w->base.owner);
        }
        w = next;
    }
}

// Tasks parked on the virtual clock: the abort line's count (a join parked
// on a task is not a clock wait — the driver's await is not counted).
long long __we_virtual_parked(void) {
    long long n = 0;
    for (we_sleep_wait *w = sleep_head; w; w = w->qnext) {
        if (w->virtual && !w->base.dead) {
            n++;
        }
    }
    return n;
}

void __we_time_sleep(long long ms) {
    // The extended-node protocol the primitive families ride: allocate the
    // family's own node (we_wait_link embedded first) and attach it to the
    // ledger — __we_link_new would size a bare link and the fields past it
    // would write out of bounds.
    we_sleep_wait *w = calloc(1, sizeof *w);
    if (!w) {
        abort();
    }
    __we_link_attach(&w->base);
    w->virtual = test_virtual;
    w->deadline = (test_virtual ? test_vnow : wall_ms()) + ms;
    // Append at the tail: the queue's order is the registration order the
    // same-instant release walks.
    w->qnext = NULL;
    if (!sleep_head) {
        sleep_head = w;
    } else {
        we_sleep_wait *t = sleep_head;
        while (t->qnext) {
            t = t->qnext;
        }
        t->qnext = w;
    }
    __we_task_park();
    // Woken by release (the node is unlinked) or by cancel (dead, still
    // queued) — either way the sleeper owns the memory.
    sleep_unlink(w);
    __we_link_free(&w->base);
}

// --- the test boundary -------------------------------------------------------

void __we_test_begin(void) {
    test_virtual = 1;
    test_vnow = 0;
    test_vstart = 0;
    test_task_base = __we_task_table_count();
}

void __we_test_end(void) {
    // Abandon the leftovers: every task spawned inside the extent that
    // never finished is taken out of scheduling — its registrations die
    // (no source ever serves it) and it never runs again. The sweep runs
    // here, before the next begin resets the clock.
    for (we_task *t = __we_all_tasks(); t; t = t->next) {
        if (t->slot >= test_task_base && t->state != WE_DONE) {
            __we_task_abandon(t);
        }
    }
    test_vdur = test_vnow - test_vstart;
    test_virtual = 0;
}

// advanceTime's runtime face: cross the barrier first (every ready task
// runs to its next block, so every wait wanting this crossing is
// registered), then move the virtual clock, release every due virtual
// wait in registration order (the queue is append-ordered, so the
// head-to-tail walk is the FIFO), and expire due virtual scopes — all
// ahead of any real deadline the run still holds.
void __we_advance(long long ms) {
    __we_sched_drain();
    test_vnow += ms;
    for (we_sleep_wait *w = sleep_head; w;) {
        we_sleep_wait *next = w->qnext;
        if (w->virtual && !w->base.dead && w->deadline <= test_vnow) {
            sleep_unlink(w);
            __we_task_wake(w->base.owner);
        }
        w = next;
    }
    __we_expire_deadlines();
}

long long __we_time_now(void) {
    return test_virtual ? test_vnow : wall_ms();
}

// --- the exploration face (M10c design D2/D3/D4) -----------------------------

// The argv face: startup.c parses --explore/--iterations/--no-reduce into
// these globals (the C harnesses set them directly). A normal run leaves
// __we_explore_on at zero and none of this section does anything.
int __we_explore_on;
long long __we_explore_iters = 100;
int __we_explore_reduce = 1;

#define EXPLORE_ATTEMPTS 4 // the per-iteration resampling budget (design D3)

// One test's exploration state: the honest counters the aggregate line
// carries and the stop conditions. A failure is a finding — the remaining
// iterations say nothing about an already-failed test — and the OR
// aggregate rides every attempt that ran, deduplication never swallows
// one (design D3's orthogonality rule).
static long long ex_attempts;       // attempts actually run
static long long ex_max_vdur;       // the longest single virtual run (D10)
static int ex_failed;               // any run failed
static const char *ex_fail_reason;  // the first failure's reason
static char ex_fail_buf[384];       // its composed storage (written once)
static int ex_guard_aborted;        // E1902's stop flag
static int ex_run_failed;           // the in-flight run's outcome
static const char *ex_run_reason;

// The meta pair the driver emits ahead of each drive: the report anchor
// and the guards' diagnostic position. A normal run stashes harmlessly.
static const char *ex_file, *ex_desc;
static long long ex_flen, ex_dlen, ex_line, ex_col;

void __we_test_meta(const char *file, long long flen, const char *desc,
                    long long dlen, long long line, long long col) {
    ex_file = file;
    ex_flen = flen;
    ex_desc = desc;
    ex_dlen = dlen;
    ex_line = line;
    ex_col = col;
}

// --- the observable trace (design D4) ----------------------------------------

// Three events make a run's interleaving observable: a channel send that
// completed (delivered or buffered), a channel receive that completed, a
// task that finished. One buffer per half of the double-run pair — the
// runner records an attempt's two runs side by side, so a deterministic
// test shows two equal halves. Ids are normalized per run: chan ids come
// from a creation counter zeroed with each run, task ids are relative to
// the extent's spawn base, so the second half's fresh tasks land on the
// same ids as their first-half counterparts. The cap is honest about its
// limit: a full buffer stops recording and flags the truncation (a
// truncated run never pretends to be comparable).
#define TRACE_CAP 4096

typedef struct {
    long long task;   // the acting task's run id
    int kind;         // WE_EV_*
    long long target; // chan id (send/receive) or the completing task's run id
} we_ev;

static we_ev trace_runs[2][TRACE_CAP];
static long long trace_run_n[2];
static int trace_run_trunc[2];
static int trace_slot;            // which half records now
static int trace_on;              // recording armed: explored runs only
static long long trace_prim_next; // the chan creation counter

long long __we_trace_alloc_id(void) { return trace_prim_next++; }

long long __we_trace_task_id(we_task *t) {
    return (long long)t->slot - test_task_base;
}

void __we_trace_event(int kind, long long target, we_task *actor) {
    if (!trace_on) {
        return; // one branch: a normal run never records
    }
    if (trace_run_n[trace_slot] >= TRACE_CAP) {
        trace_run_trunc[trace_slot] = 1; // full: drop, and say so
        return;
    }
    we_ev *e = &trace_runs[trace_slot][trace_run_n[trace_slot]++];
    e->task = __we_trace_task_id(actor);
    e->kind = kind;
    e->target = target;
}

// Begin one run's recording into the selected half: the counters (events,
// truncation, chan creations) start from zero, and recording arms only
// under exploration.
void __we_trace_reset(void) {
    trace_on = __we_explore_on != 0;
    trace_run_n[trace_slot] = 0;
    trace_run_trunc[trace_slot] = 0;
    trace_prim_next = 0;
}

void __we_trace_select(int half) { trace_slot = half ? 1 : 0; }

long long __we_trace_count(void) { return trace_run_n[trace_slot]; }

int __we_trace_truncated(void) { return trace_run_trunc[trace_slot]; }

// Event k as the observable quadruple (task, kind, target, seq): seq is
// the event's ordinal among its own (task, kind) prefix, derived by the
// same walk that reads the buffer front to back.
void __we_trace_get(long long k, long long out[4]) {
    we_ev *e = &trace_runs[trace_slot][k];
    long long seq = 0;
    for (long long j = 0; j < k; j++) {
        we_ev *p = &trace_runs[trace_slot][j];
        if (p->task == e->task && p->kind == e->kind) {
            seq++;
        }
    }
    out[0] = e->task;
    out[1] = e->kind;
    out[2] = e->target;
    out[3] = seq;
}

// --- the verdict faces (M10c design D5/D6/D7/D8) ------------------------------

// report_json is the mode bit both the diagnostics here and the report
// face below read; the single definition lives here, ahead of both.
static int report_json;
static void json_str(const char *s, long long len); // defined with the report face

// The exploration diagnostics render through the two fixed protocol faces
// the toolchain's diag package owns (design D5): the human one-liner on
// stderr and the JSON Lines event on stdout, the field order copied from
// the Go side's closed commitment (type, severity, code, message, file,
// line, column). The anchor is the test declaration's position — the
// meta pair's stash.
static void diag_human(const char *code, const char *msg) {
    fprintf(stderr, "%.*s:%lld:%lld: error[%s]: %s\n",
            (int)ex_flen, ex_file, ex_line, ex_col, code, msg);
}

static void diag_json(const char *code, const char *msg) {
    fputs("{\"type\":\"diagnostic\",\"severity\":\"error\",\"code\":\"", stdout);
    fputs(code, stdout);
    fputs("\",\"message\":", stdout);
    json_str(msg, (long long)strlen(msg));
    fputs(",\"file\":", stdout);
    json_str(ex_file, ex_flen);
    printf(",\"line\":%lld,\"column\":%lld}\n", ex_line, ex_col);
}

// E1901 (design D5): the same armed schedule ran twice and the observable
// traces disagreed — something outside the virtualized determinism is
// acting, the strongest honesty claim the exploration makes. The
// comparator is pure (the harness feeds synthetic traces; the runner
// feeds the pair's flattened halves). Returns the first divergence's
// index, or -1 when the arrays agree — a strict prefix diverges where the
// shorter side runs out.
long long __we_e1901_first_div(const long long *q0, long long n0,
                               const long long *q1, long long n1) {
    long long n = n0 < n1 ? n0 : n1;
    for (long long k = 0; k < n; k++) {
        for (int j = 0; j < 4; j++) {
            if (q0[k * 4 + j] != q1[k * 4 + j]) {
                return k;
            }
        }
    }
    return n0 == n1 ? -1 : n;
}

// The retained message is single-sourced: the render prints it, the
// aggregate's reason line repeats it verbatim.
static char ex_div_msg[512];

// One quadruple's compact face — the rendering the harnesses share; a
// NULL side is the trace that ended before this event.
static void ev_render(char *buf, size_t cap, const long long *e) {
    if (!e) {
        snprintf(buf, cap, "<end>");
        return;
    }
    snprintf(buf, cap, "%lld/%c/%lld/%lld", e[0], "SRC"[e[1]], e[2], e[3]);
}

void __we_e1901_render(long long iteration, long long attempt, long long k,
                       const long long *e1, const long long *e2, int json) {
    char b1[48], b2[48];
    ev_render(b1, sizeof b1, e1);
    ev_render(b2, sizeof b2, e2);
    snprintf(ex_div_msg, sizeof ex_div_msg,
             "exploration detected nondeterminism — the same explored schedule"
             " produced divergent observable traces (iteration %lld, attempt %lld)"
             " — first divergence at event %lld: %s vs %s",
             iteration, attempt, k, b1, b2);
    if (json) {
        diag_json("E1901", ex_div_msg);
    } else {
        diag_human("E1901", ex_div_msg);
    }
}

// The fxgate's single point of judgment (design D6): control reached the
// gate, so the call took the slot's default, so no mock was on the path —
// the condition W1910 advises about in an ordinary run, an error under
// exploration. A normal run passes straight through (the gate's
// fallthrough body is the real function); under exploration the
// diagnostic renders once per test (the idempotence flag), the body still
// runs (an already-failed run's settled fact), and the verdict stops the
// test's exploration.
static int ex_guard_rendered;
static char ex_guard_msg[256];

const char *__we_explore_guard_reason(void) {
    return ex_guard_rendered ? ex_guard_msg : 0;
}

void __we_explore_fx_check(const char *name, long long len) {
    if (!__we_explore_on || ex_guard_rendered) {
        return;
    }
    ex_guard_rendered = 1;
    snprintf(ex_guard_msg, sizeof ex_guard_msg,
             "unmocked effect executed during exploration — the explored path"
             " called %.*s, whose custom effect had no mock",
             (int)len, name);
    if (report_json) {
        diag_json("E1902", ex_guard_msg);
    } else {
        diag_human("E1902", ex_guard_msg);
    }
    ex_guard_aborted = 1;
}

// The Mazurkiewicz class key (design D7): each task's own event order
// hashed — FNV-1a 64 rolling over its quadruples in global order — the
// per-task hashes taken as an anonymous multiset (sorted, concatenated,
// hashed again), the truncation flag folded in last. Task identity never
// pairs across runs, so anonymity is the honest approximation: one grade
// coarser than a strict per-process pairing, never coarser than the
// probe's own observable face.
#define FNV_OFF 0xcbf29ce484222325ULL
#define FNV_PR 0x100000001b3ULL

static unsigned long long fnv_byte(unsigned long long h, unsigned long long b) {
    return (h ^ b) * FNV_PR;
}

static unsigned long long fnv_i64(unsigned long long h, long long v) {
    unsigned long long u = (unsigned long long)v;
    for (int i = 0; i < 8; i++) {
        h = fnv_byte(h, (u >> (8 * i)) & 0xff);
    }
    return h;
}

unsigned long long __we_por_key(const long long *q, long long n, int truncated) {
    static long long ids[TRACE_CAP];
    static unsigned long long hs[TRACE_CAP];
    long long nt = 0;
    for (long long k = 0; k < n; k++) {
        const long long *e = q + k * 4;
        long long j = 0;
        while (j < nt && ids[j] != e[0]) {
            j++;
        }
        if (j == nt) { // this task's first event: a fresh roll
            ids[nt] = e[0];
            hs[nt] = FNV_OFF;
            nt++;
        }
        hs[j] = fnv_i64(fnv_i64(fnv_i64(fnv_i64(hs[j], e[0]), e[1]), e[2]), e[3]);
    }
    for (long long j = 1; j < nt; j++) { // the multiset face: ascending
        unsigned long long h = hs[j];
        long long i = j - 1;
        while (i >= 0 && hs[i] > h) {
            hs[i + 1] = hs[i];
            i--;
        }
        hs[i + 1] = h;
    }
    unsigned long long key = FNV_OFF;
    for (long long j = 0; j < nt; j++) {
        key = fnv_i64(key, (long long)hs[j]);
    }
    return fnv_byte(key, truncated ? 1 : 0);
}

// The seen-class set (design D7): open addressing over malloc'd storage,
// capacity twice the iteration count (each iteration accepts at most one
// attempt, so it inserts at most one key), opened when a test's drive
// begins and freed when it ends. The cardinality is the report's classes.
static unsigned long long *keyset;
static unsigned char *keyset_used;
static long long keyset_cap, keyset_n;

static void keyset_open(void) {
    keyset_cap = __we_explore_iters * 2;
    if (keyset_cap < 8) {
        keyset_cap = 8;
    }
    keyset = calloc((size_t)keyset_cap, sizeof *keyset);
    keyset_used = calloc((size_t)keyset_cap, sizeof *keyset_used);
    if (!keyset || !keyset_used) {
        abort();
    }
    keyset_n = 0;
}

static void keyset_close(void) {
    free(keyset);
    free(keyset_used);
    keyset = 0;
    keyset_used = 0;
}

// Insert-if-absent: 1 when the class was already seen, 0 after inserting
// and counting it. --no-reduce skips the verdict's query but keeps the
// insert, so classes still counts distinct classes.
static int keyset_insert(unsigned long long key) {
    long long i = (long long)(key % (unsigned long long)keyset_cap);
    for (;;) {
        if (!keyset_used[i]) {
            keyset_used[i] = 1;
            keyset[i] = key;
            keyset_n++;
            return 0;
        }
        if (keyset[i] == key) {
            return 1;
        }
        i = (i + 1) % keyset_cap;
    }
}

// Flatten the selected half into quadruples for the comparator and the
// key, deriving each event's seq in one linear pass: a counter per
// (task, kind), rows keyed by task id + 1 (the driver's task 0 rides at
// id -1 inside an extent; ids are otherwise dense from the spawn base).
// The scratch pair is static BSS — the cap's 4096 events bound it, and a
// normal run never touches any of this.
static long long *seq_rows;
static long long seq_row_cap;
static long long ex_pair[2][TRACE_CAP * 4];

static long long flatten_pair(long long *out) {
    long long n = trace_run_n[trace_slot];
    long long top = 1;
    for (long long k = 0; k < n; k++) {
        we_ev *e = &trace_runs[trace_slot][k];
        if (e->task + 2 > top) {
            top = e->task + 2;
        }
    }
    if (top > seq_row_cap) { // grow-by-need: ids accumulate across attempts
        seq_rows = realloc(seq_rows, (size_t)top * 3 * sizeof *seq_rows);
        if (!seq_rows) {
            abort();
        }
        seq_row_cap = top;
    }
    memset(seq_rows, 0, (size_t)top * 3 * sizeof *seq_rows);
    for (long long k = 0; k < n; k++) {
        we_ev *e = &trace_runs[trace_slot][k];
        long long *ct = &seq_rows[(e->task + 1) * 3 + e->kind];
        out[k * 4 + 0] = e->task;
        out[k * 4 + 1] = e->kind;
        out[k * 4 + 2] = e->target;
        out[k * 4 + 3] = (*ct)++;
    }
    return n;
}

// The explored deadlock's conversion state (design D8): the idle loop
// found every task parked with no wake source under this schedule — a
// finding, not a crash. sched.c writes the message and the flag, abandons
// the run's tasks, and wakes the driver; handle_await's loop consumes the
// flag and answers the message as the await's failure payload. The reset
// between runs clears a flag the shape never read.
int __we_explore_settle_pending;
char __we_explore_settle_msg[128];

// One attempt's verdict (design D3/D5/D6/D7), called after the pair's
// second half: the guard first (its diagnostic already rendered mid-run;
// half two never ran), then a failed run (a finding stops the exploration
// — nothing after it is new information, and the failed attempt never
// enters the class set), then the pair comparison (a truncated half never
// compares — an honest skip, not an equality), then the class key's
// accept-or-resample. 0 stops the whole exploration, 1 accepts the
// attempt, 2 resamples within the budget.
static int verdict_attempt(long long iteration, long long attempt) {
    if (ex_guard_aborted) {
        ex_failed = 1;
        ex_fail_reason = ex_guard_msg; // the classified finding names itself
        return 0;
    }
    if (ex_failed) {
        return 0; // the explored schedule failed: the finding is the answer
    }
    __we_trace_select(0);
    long long n0 = flatten_pair(ex_pair[0]);
    __we_trace_select(1);
    long long n1 = flatten_pair(ex_pair[1]);
    if (!trace_run_trunc[0] && !trace_run_trunc[1]) {
        long long div = __we_e1901_first_div(ex_pair[0], n0, ex_pair[1], n1);
        if (div >= 0) {
            __we_e1901_render(iteration + 1, attempt + 1, div,
                              div < n0 ? ex_pair[0] + div * 4 : 0,
                              div < n1 ? ex_pair[1] + div * 4 : 0,
                              report_json);
            ex_failed = 1;
            ex_fail_reason = ex_div_msg;
            return 0;
        }
    }
    int seen = keyset_insert(__we_por_key(ex_pair[0], n0, trace_run_trunc[0]));
    if (!__we_explore_reduce || !seen) {
        return 1; // this attempt's class stands for the iteration
    }
    return 2; // a seen class: resample
}

static void aggregate_report(void); // defined with the report face below

// --- the drive runner (design D3) --------------------------------------------

// Between runs — both halves of a pair and consecutive attempts — the
// world is the quiet face the thunk's own end left behind: the sweep
// already abandoned the leftovers and the clock already reset. What
// remains is to verify the queue really drained (an unempty queue here is
// an internal error, not a test outcome), clear a settle flag the
// driver's shape never consumed, and disarm the pick policy so nothing
// outside an armed run ever randomizes.
static void explore_reset_world(void) {
    if (__we_ready_count() != 0) {
        fprintf(stderr, "we: internal error: ready queue not empty after an explored run\n");
        abort();
    }
    __we_explore_settle_pending = 0;
    __we_explore_arm(0, 1);
}

// The drive face, one shape for both modes: a normal run calls the thunk
// once (the M10b inline sequence behind one indirect call); exploration
// wraps it in the iteration loop — each attempt arms one schedule, runs
// the thunk twice from a fully reset world (the E1901 pair; the guard
// settles an attempt after its first half), hands the attempt to the
// verdict, and stops at the first finding. Every attempt counts honestly;
// the first failure's reason rides the aggregate.
void __we_test_drive(long long n, void (*thunk)(void)) {
    if (!__we_explore_on) {
        thunk();
        return;
    }
    ex_attempts = 0;
    ex_failed = 0;
    ex_fail_reason = 0;
    ex_max_vdur = 0;
    ex_guard_aborted = 0;
    ex_guard_rendered = 0;
    keyset_open();
    for (long long i = 0; i < __we_explore_iters; i++) {
        for (long long a = 0; a < EXPLORE_ATTEMPTS; a++) {
            ex_attempts++;
            unsigned long long seed = __we_explore_seed(n, i, a);
            int baseline = (i == 0 && a == 0); // iteration zero: FIFO baseline
            for (int half = 0; half < 2; half++) {
                __we_explore_arm(seed, baseline);
                __we_trace_select(half);
                __we_trace_reset();
                ex_run_failed = 0;
                ex_run_reason = 0;
                thunk();
                explore_reset_world();
                if (ex_run_failed && !ex_fail_reason && ex_run_reason) {
                    // the prefix says which schedule failed; the buffer is
                    // written once, so the first reason stays the reason
                    snprintf(ex_fail_buf, sizeof ex_fail_buf,
                             "explored schedule failed: %s", ex_run_reason);
                    ex_fail_reason = ex_fail_buf;
                }
                if (ex_run_failed) { // a finding survives the pair either way
                    ex_failed = 1;
                }
                if (ex_max_vdur < test_vdur) {
                    ex_max_vdur = test_vdur;
                }
                if (ex_guard_aborted) {
                    break; // the guard settled the attempt: half two is mute
                }
            }
            if (verdict_attempt(i, a) != 2) {
                break; // accepted, or the exploration stops on a finding
            }
        }
        if (ex_failed || ex_guard_aborted) {
            break; // a finding: nothing after it is new information
        }
    }
    __we_explore_arm(0, 1); // disarm: the process face returns to FIFO
    keyset_close();
    aggregate_report();
}

// --- the report face (design D5/D6/D10) --------------------------------------

// The driver's boundary quartet ends here: one report per test, one
// summary per run. The lines are the T1 goldens' bytes — the human pair
// (status, two spaces, file: name, the duration in parens; a failing
// reason indented two spaces on its own line) and the JSON Lines pair
// (chapter 21 R7's closed field sets, type first). The child renders
// both; the CLI relays stdout verbatim (design D6). The failure reason
// rides the await's Err payload — a panic-message C string — and in JSON
// mode goes to stderr (R7's field set is closed; a reason field would
// invent a spec face). An explored test reports once, after the drive:
// the aggregate line below stands for every iteration.

static long long rep_total, rep_passed, rep_failed, rep_ms;

void __we_test_set_json(int on) { report_json = on; }

// json_str writes one JSON string: the two mandatory escapes, the three
// control short forms, \u for the remaining control range; bytes >= 0x80
// pass through (JSON strings carry raw UTF-8).
static void json_str(const char *s, long long len) {
    putchar('"');
    for (long long i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        switch (c) {
        case '"':
            fputs("\\\"", stdout);
            break;
        case '\\':
            fputs("\\\\", stdout);
            break;
        case '\n':
            fputs("\\n", stdout);
            break;
        case '\r':
            fputs("\\r", stdout);
            break;
        case '\t':
            fputs("\\t", stdout);
            break;
        default:
            if (c < 0x20) {
                printf("\\u%04x", c);
            } else {
                putchar(c);
            }
        }
    }
    putchar('"');
}

void __we_test_report(const char *file, long long flen, const char *desc,
                      long long dlen, long long failed, const char *reason) {
    if (__we_explore_on) {
        // An explored run never prints its own line — the aggregate line
        // replaces N iterations' worth of per-run faces (design D10); the
        // outcome rides the runner's accounting instead.
        ex_run_failed = (int)failed;
        ex_run_reason = reason;
        return;
    }
    rep_total++;
    if (failed) {
        rep_failed++;
    } else {
        rep_passed++;
    }
    rep_ms += test_vdur;
    if (report_json) {
        fputs("{\"type\":\"test-result\",\"file\":", stdout);
        json_str(file, flen);
        fputs(",\"name\":", stdout);
        json_str(desc, dlen);
        printf(",\"status\":\"%s\",\"duration_ms\":%lld}\n", failed ? "fail" : "pass", test_vdur);
        if (failed && reason) {
            fprintf(stderr, "%.*s: %.*s: %s\n", (int)flen, file, (int)dlen, desc, reason);
        }
        return;
    }
    printf("%s  %.*s: %.*s (%lldms)\n", failed ? "fail" : "pass",
           (int)flen, file, (int)dlen, desc, test_vdur);
    if (failed && reason) {
        printf("  %s\n", reason);
    }
}

// The aggregate line (design D10): one line stands for the whole
// exploration — the configured iteration count, the attempts actually
// run, the distinct classes found — with the duration the longest single
// run's virtual time (the probe's repetition is not the test's cost) and
// the summary accounting fed exactly once. The reason line names the
// finding: the guard's or E1901's own message, or an explored schedule's
// failure under the prefix that says which schedule failed.
static void aggregate_report(void) {
    int failed = ex_failed || ex_guard_aborted;
    rep_total++;
    if (failed) {
        rep_failed++;
    } else {
        rep_passed++;
    }
    rep_ms += ex_max_vdur;
    if (report_json) {
        fputs("{\"type\":\"test-result\",\"file\":", stdout);
        json_str(ex_file, ex_flen);
        fputs(",\"name\":", stdout);
        json_str(ex_desc, ex_dlen);
        printf(",\"status\":\"%s\",\"duration_ms\":%lld,\"explore\":"
               "{\"iterations\":%lld,\"attempts\":%lld,\"classes\":%lld}}\n",
               failed ? "fail" : "pass", ex_max_vdur, __we_explore_iters,
               ex_attempts, keyset_n);
        if (failed && ex_fail_reason) {
            fprintf(stderr, "%.*s: %.*s: %s\n",
                    (int)ex_flen, ex_file, (int)ex_dlen, ex_desc, ex_fail_reason);
        }
        return;
    }
    printf("%s  %.*s: %.*s (explore %lld iterations, %lld attempts, %lld classes)\n",
           failed ? "fail" : "pass", (int)ex_flen, ex_file, (int)ex_dlen, ex_desc,
           __we_explore_iters, ex_attempts, keyset_n);
    if (failed && ex_fail_reason) {
        printf("  %s\n", ex_fail_reason);
    }
}

// The summary closes the run and owns the process's exit code: 1 with
// any failure, 0 otherwise (chapter 21 R5's 0/1; compile failure's 2 is
// the CLI's face, before a child ever runs).
void __we_test_summary(void) {
    if (report_json) {
        printf("{\"type\":\"test-summary\",\"total\":%lld,\"passed\":%lld,"
               "\"failed\":%lld,\"duration_ms\":%lld}\n",
               rep_total, rep_passed, rep_failed, rep_ms);
    } else {
        printf("total %lld, passed %lld, failed %lld (%lldms)\n",
               rep_total, rep_passed, rep_failed, rep_ms);
    }
    exit(rep_failed > 0 ? 1 : 0);
}

// --- the assertion helpers (design D7) ---------------------------------------

// The report lines are the design's wording, byte-pinned by the T1 goldens
// through the runner's fail report: the plain true face, the fixed
// expected-false line, and the two equality shapes.

void __we_assert_true(long long cond) {
    if (!cond) {
        __we_task_fail("assertion failed");
    }
}

void __we_assert_false(long long cond) {
    if (cond) {
        __we_task_fail("assertion failed: expected false");
    }
}

void __we_assert_eq_i64(long long got, long long want) {
    if (got != want) {
        char buf[96];
        snprintf(buf, sizeof buf, "assertion failed: got %lld, want %lld", got, want);
        __we_task_fail(buf);
    }
}

void __we_assert_eq_str(const char *gp, long long gl, const char *wp, long long wl) {
    if (gl != wl || memcmp(gp, wp, (size_t)gl) != 0) {
        char buf[256];
        snprintf(buf, sizeof buf, "assertion failed: got \"%.*s\", want \"%.*s\"",
                 (int)gl, gp, (int)wl, wp);
        __we_task_fail(buf);
    }
}

// T10 (design D9): a composite comparison reports the position it reached
// ahead of the leaf's own wording. The two rows above stay as they are —
// they are the pre-T10 spellings two byte-pinned goldens hold, and a
// top-level leaf still routes to them — so what follows is the same two
// shapes carrying a position, plus the two families whose rendering the
// comparison fixes.
//
// The position is the one input a program sizes: a declaration nested
// thirty-two deep is thirty-two identifiers, and a String leaf's value is
// as long as the program made it. So the message is built at its exact
// length rather than in a fixed buffer — a silently truncated report would
// name a position the program never had. The allocation is retained rather
// than freed: the tail below never returns, which is at once why the free
// is unreachable and why the comparison needs no short-circuit of its own
// — the first leaf that differs is the last call that runs.
static void eq_fail(const char *path, const char *fmt, ...) {
    static const char head[] = "assertion failed: ";
    va_list ap;
    int n;

    va_start(ap, fmt);
    n = vsnprintf(NULL, 0, fmt, ap);
    va_end(ap);
    if (n < 0) {
        __we_task_fail("assertion failed");
    }
    // "at " and ": " bracket the position when there is one; a null path
    // is a top-level comparison, whose report is the line it always was.
    size_t pn = path ? strlen(path) + 5 : 0;
    char *msg = malloc(sizeof head - 1 + pn + (size_t)n + 1);
    if (!msg) {
        abort();
    }
    memcpy(msg, head, sizeof head - 1);
    size_t at = sizeof head - 1;
    if (path) {
        size_t plen = strlen(path);
        memcpy(msg + at, "at ", 3);
        memcpy(msg + at + 3, path, plen);
        at += 3 + plen;
        msg[at++] = ':';
        msg[at++] = ' ';
    }
    va_start(ap, fmt);
    vsnprintf(msg + at, (size_t)n + 1, fmt, ap);
    va_end(ap);
    __we_task_fail(msg);
}

void __we_assert_eq_i64_at(const char *path, long long got, long long want) {
    if (got != want) {
        eq_fail(path, "got %lld, want %lld", got, want);
    }
}

// The UInt64 row renders the magnitude, never the bit pattern's signed
// reading — the same rule __we_str_of_u64 holds.
void __we_assert_eq_u64_at(const char *path, unsigned long long got, unsigned long long want) {
    if (got != want) {
        eq_fail(path, "got %llu, want %llu", got, want);
    }
}

// The Bool row renders true/false where the pre-T10 numeric route printed
// the word's own 1/0.
void __we_assert_eq_bool_at(const char *path, long long got, long long want) {
    if (got != want) {
        eq_fail(path, "got %s, want %s", __we_str_of_bool(got).p, __we_str_of_bool(want).p);
    }
}

void __we_assert_eq_str_at(const char *path, const char *gp, long long gl,
                           const char *wp, long long wl) {
    if (gl != wl || memcmp(gp, wp, (size_t)gl) != 0) {
        eq_fail(path, "got \"%.*s\", want \"%.*s\"", (int)gl, gp, (int)wl, wp);
    }
}
