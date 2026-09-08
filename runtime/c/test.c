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

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "sched.h"

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

// --- the report face (design D5/D6) -----------------------------------------

// The driver's boundary quartet ends here: one report per test, one
// summary per run. The lines are the T1 goldens' bytes — the human pair
// (status, two spaces, file: name, the duration in parens; a failing
// reason indented two spaces on its own line) and the JSON Lines pair
// (chapter 21 R7's closed field sets, type first). The child renders
// both; the CLI relays stdout verbatim (design D6). The failure reason
// rides the await's Err payload — a panic-message C string — and in JSON
// mode goes to stderr (R7's field set is closed; a reason field would
// invent a spec face).

static long long rep_total, rep_passed, rep_failed, rep_ms;
static int report_json;

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
