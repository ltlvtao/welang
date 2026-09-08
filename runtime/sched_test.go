package weruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// M9b design D1/D2/D4: the scheduler answers for its contracts in C, where
// the tasks live — cooperative FIFO scheduling with park/wake handoff,
// task-0 boot through the trampoline, the deadline sleep's timeout path,
// honest deadlock abort, and per-task GC roots surviving parks. The embed
// variables carry the sources, so missing ones fail here at compile time.

// schedFifoHarness pins the scheduling order: two tasks created by task 0
// run FIFO to their yields, the joiner wakes behind the second task's
// yield, and await consumes each thunk's i64 outcome.
const schedFifoHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
long long __we_yield(void);

static long long thunk_a(void *env) { printf("a-start\n"); __we_yield(); printf("a-end\n"); return 41; }
static long long thunk_b(void *env) { printf("b-start\n"); __we_yield(); printf("b-end\n"); return 42; }

int we_main(void) {
    void *ha = __we_task_new(thunk_a, 0);
    void *hb = __we_task_new(thunk_b, 0);
    long long v = 0;
    long long ta = __we_handle_await(ha, &v);
    long long tb = __we_handle_await(hb, &v);
    printf("joined-a=%lld,%lld,%lld\n", ta, tb, v);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// schedPanicHarness pins the task-panic boundary (design D6): a thunk that
// fails stores its message, defer-inverting aside, and await reports tag 1
// with the message pointer; the process does not abort.
const schedPanicHarness = `#include <stdio.h>
#include <string.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void __we_task_fail(const char *msg);

static long long boom(void *env) { __we_task_fail("kapow"); return 0; }

int we_main(void) {
    void *h = __we_task_new(boom, 0);
    long long v = 0;
    long long tag = __we_handle_await(h, &v);
    printf("tag=%lld msg=%s\n", tag, (const char *)v);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// schedTimeoutHarness pins the real-clock timeout path (design D4): a task
// parked on an empty channel under a 30 ms scope is woken by expiry, the
// leave reports TimedOut, and the run's wall time actually passed.
const schedTimeoutHarness = `#include <stdio.h>
#include <time.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void *__we_scope_enter(long long deadline_ms, long long collect_all);
long long __we_scope_leave(void *scope);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_recv(void *ch, long long *out);

static void *g_ch;

static long long sleeper(void *env) {
    long long v = 0;
    long long got = __we_chan_recv(g_ch, &v); /* parks until cancelled */
    printf("sleeper-woke=%lld\n", got);
    return 0;
}

static long long now_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long long)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}

int we_main(void) {
    g_ch = __we_prim_new_chan(1, 8, 0);
    long long t0 = now_ms();
    void *sc = __we_scope_enter(30, 0);
    __we_task_new(sleeper, 0);
    long long r = __we_scope_leave(sc);
    long long dt = now_ms() - t0;
    printf("leave=%lld dt>20=%lld\n", r, dt > 20);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// schedDeadlockHarness pins honest deadlock (design D2): two tasks each
// parked on a rendezvous the other would complete never wake, and the
// joining main parks with them — the scheduler reports one stderr line
// (all three parked tasks) and exits 70.
const schedDeadlockHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_send(void *ch, long long v);
long long __we_chan_recv(void *ch, long long *out);

static void *g_a;
static void *g_b;

static long long t1(void *env) {
    long long v = 0;
    __we_chan_send(g_a, 1);
    __we_chan_recv(g_b, &v);
    return 0;
}

static long long t2(void *env) {
    long long v = 0;
    __we_chan_send(g_b, 2);
    __we_chan_recv(g_a, &v);
    return 0;
}

int we_main(void) {
    g_a = __we_prim_new_chan(0, 8, 0);
    g_b = __we_prim_new_chan(0, 8, 0);
    void *h1 = __we_task_new(t1, 0);
    void *h2 = __we_task_new(t2, 0);
    long long v = 0;
    __we_handle_await(h1, &v);
    __we_handle_await(h2, &v);
    printf("unreachable\n");
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// schedGcParkHarness pins per-task roots surviving parks (design D7): a
// task allocates, roots, parks; another task forces a collection; the
// woken task reads its payload intact.
const schedGcParkHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_send(void *ch, long long v);
long long __we_chan_recv(void *ch, long long *out);
void *__we_alloc(long long n);
long long __we_gc_collect(void);
void __we_root_push(void *p);
void __we_root_pop(void);

static void *g_go;

static long long holder(void *env) {
    char *blk = __we_alloc(32);
    *(long long *)(blk + 8) = 32;
    *(long long *)(blk + 16) = 7777;
    __we_root_push(blk);
    long long v = 0;
    __we_chan_recv(g_go, &v); /* parks rooted */
    printf("survived=%lld\n", *(long long *)(blk + 16));
    __we_root_pop();
    return 0;
}

static long long collector(void *env) {
    printf("swept=%lld\n", __we_gc_collect());
    __we_chan_send(g_go, 1);
    return 0;
}

int we_main(void) {
    g_go = __we_prim_new_chan(0, 8, 0);
    __we_root_push(g_go); /* the emitter's root for the live local: the
                             channel must survive the cross-task collect */
    void *h = __we_task_new(holder, 0);
    __we_task_new(collector, 0);
    long long v = 0;
    __we_handle_await(h, &v);
    __we_root_pop();
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM9bSchedFIFO(t *testing.T) {
	compileAndRun(t,
		map[string]string{"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "test.c": TestSource, "main.c": schedFifoHarness},
		[]string{"gc.c", "sched.c", "test.c", "main.c"},
		"a-start\nb-start\na-end\nb-end\njoined-a=0,0,42\n")
}

func TestM9bSchedTaskPanic(t *testing.T) {
	compileAndRun(t,
		map[string]string{"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "test.c": TestSource, "main.c": schedPanicHarness},
		[]string{"gc.c", "sched.c", "test.c", "main.c"},
		"tag=1 msg=kapow\n")
}

func TestM9bSchedTimeout(t *testing.T) {
	compileAndRun(t,
		map[string]string{"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "conc.c": ConcSource, "test.c": TestSource, "main.c": schedTimeoutHarness},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "main.c"},
		"sleeper-woke=0\nleave=1 dt>20=1\n")
}

func TestM9bSchedGcParkSurvival(t *testing.T) {
	compileAndRun(t,
		map[string]string{"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "conc.c": ConcSource, "test.c": TestSource, "main.c": schedGcParkHarness},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "main.c"},
		"swept=0\nsurvived=7777\n")
}

func TestM9bSchedDeadlock(t *testing.T) {
	dir := t.TempDir()
	for name, src := range map[string]string{
		"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "conc.c": ConcSource, "test.c": TestSource, "main.c": schedDeadlockHarness,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	args := []string{"-o", filepath.Join(dir, "harness")}
	for _, in := range []string{"gc.c", "sched.c", "conc.c", "test.c", "main.c"} {
		args = append(args, filepath.Join(dir, in))
	}
	if out, err := exec.Command(pinnedClang(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("clang compile: %v\n%s", err, out)
	}
	var stderr strings.Builder
	cmd := exec.Command(filepath.Join(dir, "harness"))
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 70 {
		t.Fatalf("want exit 70, got %v", err)
	}
	if !strings.HasPrefix(stderr.String(), "we: deadlock: ") {
		t.Fatalf("stderr mismatch: %q", stderr.String())
	}
}
