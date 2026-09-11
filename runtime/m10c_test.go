package weruntime

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// M10c (design D2/D3/D4): the exploration core answers in C, where the
// scheduler lives. The splitmix64 stream and seed derivation (pinned
// against independently computed tables), the armed pick policy (iteration
// zero's baseline stays FIFO; a seeded random pick reproduces byte-exact),
// the drive runner's world reset (each attempt starts from a quiet task
// face and a zero clock), and the three-event trace with its normalized
// ids and truncation flag. Written test-first: the exploration faces do
// not exist in sched.c/test.c/conc.c yet, so these harnesses fail to link
// here.

// sm64Harness pins the PRNG and the seed derivation: the first sixteen
// outputs from a named seed, and the derived attempt seeds for a spread
// of (test, iteration, attempt) triples — the constants' exact behavior,
// the root of every golden's reproducibility.
const sm64Harness = `#include <stdio.h>

void __we_rng_seed(unsigned long long s);
unsigned long long __we_rng_next(void);
unsigned long long __we_explore_seed(long long n, long long i, long long a);

int main(void) {
    __we_rng_seed(0x123456789abcdefULL);
    for (int i = 0; i < 16; i++) {
        printf("%016llx\n", __we_rng_next());
    }
    printf("seeds=%016llx %016llx %016llx %016llx\n",
           __we_explore_seed(0, 0, 0), __we_explore_seed(3, 1, 0),
           __we_explore_seed(3, 1, 1), __we_explore_seed(7, 2, 3));
    return 0;
}
`

// pickSceneHarness is the shared pick-policy scene: three recorder tasks
// and a main that yield-polls until all three finished. The ready queue
// holds [a, b, c, main] at the first pick; every pick with two or more
// queued tasks draws one rng output, so the completion order is a pure
// function of the armed seed.
const pickSceneHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
long long __we_yield(void);
void __we_test_begin(void);
void __we_test_end(void);
extern int __we_explore_on;
void __we_explore_arm(unsigned long long seed, int baseline);
void __we_trace_reset(void);

static char order[8];
static int order_n, da, db, dc;

static long long ta(void *env) { (void)env; order[order_n++] = 'a'; da = 1; return 0; }
static long long tb(void *env) { (void)env; order[order_n++] = 'b'; db = 1; return 0; }
static long long tc(void *env) { (void)env; order[order_n++] = 'c'; dc = 1; return 0; }

static void scene(unsigned long long seed, int baseline) {
    order_n = 0;
    da = db = dc = 0;
    __we_test_begin();
    __we_explore_arm(seed, baseline);
    __we_trace_reset();
    void *ha = __we_task_new(ta, 0);
    void *hb = __we_task_new(tb, 0);
    void *hc = __we_task_new(tc, 0);
    while (!da || !db || !dc) {
        __we_yield();
    }
    long long v = 0;
    __we_handle_await(ha, &v);
    __we_handle_await(hb, &v);
    __we_handle_await(hc, &v);
    __we_explore_arm(0, 1); // disarm: later scheduling is FIFO again
    __we_test_end();
    order[order_n] = 0;
}
`

// pickBaselineMain: armed with baseline, the seed is ignored and the
// order is the plain FIFO face — iteration zero's deterministic run.
const pickBaselineMain = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));

static char order[8];
static int order_n, da, db, dc;

static void scene(unsigned long long seed, int baseline);

int we_main(void) {
    extern int __we_explore_on;
    __we_explore_on = 1;
    scene(0xabcdef, 1); // baseline: the seed value must not matter
    printf("order=%s\n", order);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// pickRandomMain: armed random with seed 1 the order is bca (pinned from
// the stream: draws 3, 0 pick b, then c, then the head a). The same scene
// twice must reproduce both the order and the trace — the two recorded
// halves equal is E1901's precondition holding on a deterministic test.
const pickRandomMain = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
extern int __we_explore_on;
void __we_trace_select(int half);
void __we_trace_reset(void);
long long __we_trace_count(void);
void __we_trace_get(long long k, long long out[4]);

static char order[8];
static int order_n, da, db, dc;

static void scene(unsigned long long seed, int baseline);

static void dump_trace(void) {
    long long n = __we_trace_count();
    for (long long k = 0; k < n; k++) {
        long long e[4];
        __we_trace_get(k, e);
        printf("%s%lld/%c/%lld/%lld", k ? " " : "", e[0], "SRC"[e[1]], e[2], e[3]);
    }
    printf("\n");
}

int we_main(void) {
    __we_explore_on = 1;
    __we_trace_select(0);
    scene(1, 0);
    printf("order1=%s\n", order);
    __we_trace_select(1);
    scene(1, 0);
    printf("order2=%s\n", order);

    static long long keep[16][4];
    __we_trace_select(0);
    long long n0 = __we_trace_count();
    for (long long k = 0; k < n0; k++) {
        __we_trace_get(k, keep[k]);
    }
    __we_trace_select(1);
    long long n1 = __we_trace_count();
    int same = n0 == n1;
    for (long long k = 0; same && k < n1; k++) {
        long long e[4];
        __we_trace_get(k, e);
        for (int j = 0; j < 4; j++) {
            if (e[j] != keep[k][j]) {
                same = 0;
            }
        }
    }
    printf("same=%d n=%lld\n", same, n1);
    __we_trace_select(0);
    dump_trace();
    __we_trace_select(1);
    dump_trace();
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// traceHarness pins the three-event family with normalized ids: a
// rendezvous pair, a buffered pair, three task completions — the global
// order byte-pinned (the sender's completion lands behind the buffered
// pair's task because the woken sender queues behind it), chan ids from
// the per-run creation counter, task ids relative to the extent's base so
// the second scene's fresh tasks compare equal.
const traceHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void __we_test_begin(void);
void __we_test_end(void);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_send(void *ch, long long v);
long long __we_chan_recv(void *ch, long long *out);
extern int __we_explore_on;
void __we_explore_arm(unsigned long long seed, int baseline);
void __we_trace_select(int half);
void __we_trace_reset(void);
long long __we_trace_count(void);
void __we_trace_get(long long k, long long out[4]);

static void *g_c1, *g_c2;

static long long f_sender(void *env) {
    (void)env;
    __we_chan_send(g_c1, 7); // rendezvous: parks until the receiver takes it
    return 0;
}

static long long f_recv(void *env) {
    (void)env;
    long long v = 0;
    __we_chan_recv(g_c1, &v);
    return 0;
}

static long long f_both(void *env) {
    (void)env;
    __we_chan_send(g_c2, 9); // buffered: completes immediately
    long long v = 0;
    __we_chan_recv(g_c2, &v);
    return 0;
}

static void scene(void) {
    __we_test_begin();
    __we_explore_arm(7, 1); // baseline FIFO: the deterministic schedule
    __we_trace_reset();
    g_c1 = __we_prim_new_chan(0, 8, 0); // chan 0: rendezvous
    g_c2 = __we_prim_new_chan(1, 8, 0); // chan 1: buffered
    void *h1 = __we_task_new(f_sender, 0); // task id 0 in this extent
    void *h2 = __we_task_new(f_recv, 0);   // task id 1
    void *h3 = __we_task_new(f_both, 0);   // task id 2
    long long v = 0;
    __we_handle_await(h1, &v); // parks: the sender finishes last
    __we_handle_await(h2, &v);
    __we_handle_await(h3, &v);
    __we_explore_arm(0, 1);
    __we_test_end();
}

static void dump_trace(void) {
    long long n = __we_trace_count();
    for (long long k = 0; k < n; k++) {
        long long e[4];
        __we_trace_get(k, e);
        printf("%s%lld/%c/%lld/%lld", k ? " " : "", e[0], "SRC"[e[1]], e[2], e[3]);
    }
    printf("\n");
}

int we_main(void) {
    __we_explore_on = 1;
    __we_trace_select(0);
    scene();
    __we_trace_select(1);
    scene();

    static long long keep[16][4];
    __we_trace_select(0);
    long long n0 = __we_trace_count();
    for (long long k = 0; k < n0; k++) {
        __we_trace_get(k, keep[k]);
    }
    __we_trace_select(1);
    long long n1 = __we_trace_count();
    int same = n0 == n1;
    for (long long k = 0; same && k < n1; k++) {
        long long e[4];
        __we_trace_get(k, e);
        for (int j = 0; j < 4; j++) {
            if (e[j] != keep[k][j]) {
                same = 0;
            }
        }
    }
    printf("same=%d n=%lld\n", same, n1);
    __we_trace_select(0);
    dump_trace();
    __we_trace_select(1);
    dump_trace();
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// traceTruncHarness pins the cap: a task that moves 5000 channel values
// fills the 4096-event buffer exactly, the overflow sets the truncated
// flag, and the count stops at the cap.
const traceTruncHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void __we_test_begin(void);
void __we_test_end(void);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_send(void *ch, long long v);
long long __we_chan_recv(void *ch, long long *out);
extern int __we_explore_on;
void __we_explore_arm(unsigned long long seed, int baseline);
void __we_trace_reset(void);
long long __we_trace_count(void);
int __we_trace_truncated(void);

static void *g_ch;

static long long looper(void *env) {
    (void)env;
    for (int i = 0; i < 2500; i++) { // 2 events each: 5000 > 4096
        __we_chan_send(g_ch, i);
        long long v = 0;
        __we_chan_recv(g_ch, &v);
    }
    return 0;
}

int we_main(void) {
    __we_explore_on = 1;
    __we_test_begin();
    __we_explore_arm(5, 1);
    __we_trace_reset();
    g_ch = __we_prim_new_chan(1, 8, 0);
    void *h = __we_task_new(looper, 0);
    long long v = 0;
    __we_handle_await(h, &v);
    __we_explore_arm(0, 1);
    __we_test_end();
    printf("count=%lld truncated=%d\n", __we_trace_count(), __we_trace_truncated());
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// driveResetHarness pins the runner's loop and reset through the real
// face: two iterations (baseline then seeded), each attempt running the
// thunk twice (the E1901 pair), so the thunk body executes four times
// from a world that must be quiet every time — the leftover stays parked
// forever, the ready queue drains, the trace resets, and a fresh extent
// reads a zero clock.
const driveResetHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_yield(void);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_recv(void *ch, long long *out);
void __we_test_begin(void);
void __we_test_end(void);
void __we_test_report(const char *, long long, const char *, long long, long long, const char *);
void __we_test_meta(const char *, long long, const char *, long long, long long, long long);
void __we_test_drive(long long n, void (*thunk)(void));
long long __we_time_now(void);
long long __we_ready_count(void);
void __we_trace_reset(void);
long long __we_trace_count(void);
extern int __we_explore_on;
extern long long __we_explore_iters;

static void *g_gate;
static int ran_leftover, spawn_count;

static long long leftover(void *env) {
    (void)env;
    long long v = 0;
    __we_chan_recv(g_gate, &v); // parks for good inside the run
    ran_leftover = 1;           // unreachable: the sweep abandons it
    return 0;
}

static void drive_thunk(void) {
    __we_test_begin();
    spawn_count++;
    __we_task_new(leftover, 0);
    __we_yield();     // it reaches its park (or stays queued)
    __we_test_end();  // the sweep abandons it
    __we_test_report("tests/m.we", 10, "reset", 5, 0, 0);
}

int we_main(void) {
    __we_explore_on = 1;
    __we_explore_iters = 2; // i=0 baseline, i=1 seeded
    g_gate = __we_prim_new_chan(0, 8, 0);
    __we_test_meta("tests/m.we", 10, "reset", 5, 3, 1);
    __we_test_drive(0, drive_thunk);
    printf("spawn=%d ran=%d ready=%lld trace=%lld\n",
           spawn_count, ran_leftover, __we_ready_count(), __we_trace_count());
    __we_test_begin(); // a fresh extent from the quiet face
    long long now = __we_time_now();
    __we_test_end();
    printf("now=%lld\n", now);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// m10cRun is the compile set: the full runtime (the scheduler now reads
// the pick policy and the trace hooks span conc.c's delivery points).
func m10cRun(t *testing.T, mainSrc, want string) {
	t.Helper()
	m10cRunBoth(t, mainSrc, want, "")
}

// m10cRunBoth additionally pins stderr — the diagnostic faces render
// there (the human one-liner is the diag protocol's stderr face).
func m10cRunBoth(t *testing.T, mainSrc, want, wantErr string) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "str.h": StrHeader,
		"conc.c": ConcSource, "test.c": TestSource, "str.c": StrSource, "main.c": mainSrc,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	args := []string{"-o", filepath.Join(dir, "harness")}
	for _, in := range []string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"} {
		args = append(args, filepath.Join(dir, in))
	}
	if out, err := exec.Command(pinnedClang(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("clang compile: %v\n%s", err, out)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(filepath.Join(dir, "harness"))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("harness run: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	if stdout.String() != want {
		t.Errorf("stdout mismatch:\nwant: %q\ngot:  %q", want, stdout.String())
	}
	if stderr.String() != wantErr {
		t.Errorf("stderr mismatch:\nwant: %q\ngot:  %q", wantErr, stderr.String())
	}
}

func TestM10cSplitmix64(t *testing.T) {
	m10cRun(t, sm64Harness,
		"157a3807a48faa9d\n"+
			"d573529b34a1d093\n"+
			"2f90b72e996dccbe\n"+
			"a2d419334c4667ec\n"+
			"01404ce914938008\n"+
			"14bc574c2a2b4c72\n"+
			"b8fc5b1060708c05\n"+
			"8931545f4f9ea651\n"+
			"f984db4ef14fde1b\n"+
			"2680d065cb73ece7\n"+
			"cdb8c9cd9a62da0f\n"+
			"6a6e60fd5089adec\n"+
			"8eba85b28df77747\n"+
			"97f6c69811cfb13b\n"+
			"380e8b5c685039cf\n"+
			"d7ebcca19d49c3f5\n"+
			"seeds=a706dd2f4d197e6f cb2621e231f8ec7d 9d4f36d0142da494 e7158d61f4ee28e2\n")
}

func TestM10cPickBaselineFifo(t *testing.T) {
	src := pickSceneHarness + "\n" + pickBaselineMain
	m10cRun(t, src, "order=abc\n")
}

func TestM10cPickRandomReproducible(t *testing.T) {
	src := pickSceneHarness + "\n" + pickRandomMain
	trace := "1/C/1/0 2/C/2/0 0/C/0/0\n"
	m10cRun(t, src,
		"order1=bca\n"+
			"order2=bca\n"+
			"same=1 n=3\n"+
			trace+trace)
}

func TestM10cTraceEvents(t *testing.T) {
	trace := "0/S/0/0 1/R/0/0 1/C/1/0 2/S/1/0 2/R/1/0 2/C/2/0 0/C/0/0\n"
	m10cRun(t, traceHarness,
		"same=1 n=7\n"+trace+trace)
}

func TestM10cTraceTruncation(t *testing.T) {
	m10cRun(t, traceTruncHarness, "count=4096 truncated=1\n")
}

func TestM10cDriveResetWorld(t *testing.T) {
	// T4's verdict face completes the runner: every attempt now carries the
	// POR verdict, so the empty-trace scenario resamples its full budget
	// (5 attempts, 10 thunk runs) and the drive ends with the aggregate line.
	m10cRun(t, driveResetHarness,
		"pass  tests/m.we: reset (explore 2 iterations, 5 attempts, 1 classes)\n"+
			"spawn=10 ran=0 ready=0 trace=0\n"+
			"now=0\n")
}

// T4 (design D5/D6/D7/D8/D10): the verdict faces. The E1901 comparator
// and both diagnostic render faces, the fxgate's single point of judgment,
// the POR class key (pinned against an independently computed FNV table),
// the attempt verdict through the real drive (resample exhaustion,
// no-reduce, a mid-exploration failure, both aggregate report faces), the
// explored deadlock's conversion to a failing run, and the explored runs'
// discarded io bytes. Written test-first: none of these faces exist in
// test.c/sched.c/io.c yet.

// e1901Harness pins the comparator (equal, divergent, and one-side-longer
// pairs) and both render faces: the human line on stderr, the JSON Lines
// diagnostic event on stdout — the shapes the Go diag package owns, the
// field order its closed commitment. One side past its trace's end
// renders as <end>. Iteration and attempt arrive 1-based.
const e1901Harness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
long long __we_e1901_first_div(const long long *q0, long long n0,
                               const long long *q1, long long n1);
void __we_e1901_render(long long iteration, long long attempt, long long k,
                       const long long *e1, const long long *e2, int json);
void __we_test_meta(const char *, long long, const char *, long long, long long, long long);
void __we_test_set_json(int on);

int we_main(void) {
    // quadruples (task, kind, target, seq); kinds: send 0, receive 1, done 2
    static const long long a[12] = {0,0,0,0, 1,1,0,0, 2,2,2,0};
    static const long long b[12] = {0,0,0,0, 1,1,0,0, 2,2,9,0}; // diverges at 2
    printf("eq=%lld div=%lld pre=%lld\n",
           __we_e1901_first_div(a, 3, a, 3),
           __we_e1901_first_div(a, 3, b, 3),
           __we_e1901_first_div(a, 2, a, 3));
    __we_test_meta("tests/e.we", 10, "cmp", 3, 5, 2);
    static const long long e1[4] = {2,0,1,0};
    static const long long e2[4] = {3,1,0,1};
    __we_e1901_render(2, 1, 7, e1, e2, 0); // human face, stderr (1-based in)
    __we_test_set_json(1);
    __we_e1901_render(1, 1, 3, 0, e2, 1); // json face, one side ended
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM10cE1901CompareAndRender(t *testing.T) {
	m10cRunBoth(t, e1901Harness,
		"eq=-1 div=2 pre=2\n"+
			"{\"type\":\"diagnostic\",\"severity\":\"error\",\"code\":\"E1901\","+
			"\"message\":\"exploration detected nondeterminism — the same explored"+
			" schedule produced divergent observable traces (iteration 1, attempt 1)"+
			" — first divergence at event 3: <end> vs 3/R/0/1\","+
			"\"file\":\"tests/e.we\",\"line\":5,\"column\":2}\n",
		"tests/e.we:5:2: error[E1901]: exploration detected nondeterminism —"+
			" the same explored schedule produced divergent observable traces"+
			" (iteration 2, attempt 1) — first divergence at event 7:"+
			" 2/S/1/0 vs 3/R/0/1\n")
}

// fxgateHarness pins the gate: a normal run passes straight through with
// nothing recorded, an explored run renders E1902 once at the test
// declaration's position, and further gates stay silent (the idempotence
// flag) while the reason stays queryable for the aggregate line.
const fxgateHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void __we_test_meta(const char *, long long, const char *, long long, long long, long long);
void __we_explore_fx_check(const char *name, long long len);
const char *__we_explore_guard_reason(void);
extern int __we_explore_on;

int we_main(void) {
    __we_test_meta("tests/g.we", 11, "gate", 4, 7, 1);
    __we_explore_fx_check("save", 4); // a normal run: the real call runs
    printf("silent=%d\n", __we_explore_guard_reason() == 0);
    __we_explore_on = 1;
    __we_explore_fx_check("save", 4);  // renders E1902 once
    __we_explore_fx_check("other", 5); // idempotent: no second diagnostic
    printf("reason=%s\n", __we_explore_guard_reason());
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM10cFxGateCheck(t *testing.T) {
	m10cRunBoth(t, fxgateHarness,
		"silent=1\n"+
			"reason=unmocked effect executed during exploration — the explored"+
			" path called save, whose custom effect had no mock\n",
		"tests/g.we:7:1: error[E1902]: unmocked effect executed during"+
			" exploration — the explored path called save, whose custom effect"+
			" had no mock\n")
}

// porKeyHarness pins the class key against an independently computed
// table: two globally swapped traces with identical per-task orders key
// equal (the anonymous multiset), a different target keys differently,
// the truncation flag folds in, and the exact key value is pinned.
const porKeyHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
unsigned long long __we_por_key(const long long *q, long long n, int truncated);

int we_main(void) {
    static const long long a[8] = {0,0,0,0, 1,2,1,0};
    static const long long b[8] = {1,2,1,0, 0,0,0,0}; // same per-task orders
    static const long long d[8] = {0,0,0,0, 1,2,2,0}; // a different target
    printf("k=%016llx same=%d diff=%d trunc=%d\n",
           __we_por_key(a, 2, 0),
           __we_por_key(a, 2, 0) == __we_por_key(b, 2, 0),
           __we_por_key(a, 2, 0) != __we_por_key(d, 2, 0),
           __we_por_key(a, 2, 0) != __we_por_key(a, 2, 1));
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM10cPorKey(t *testing.T) {
	m10cRun(t, porKeyHarness,
		"k=50e11b4212785a7f same=1 diff=1 trunc=1\n")
}

// driveVerdictHarness runs four drives through the real runner: resample
// exhaustion (a schedule-free test merges every attempt into one class,
// 1 + 4*(N-1) attempts), no-reduce (one attempt per iteration, the set
// still dedups so classes counts distinct classes), a failure found
// mid-exploration (attempt two is a duplicate class, attempt three fails
// — the exact accounting of the order-found golden), and both aggregate
// report faces with the explore object behind duration_ms.
const driveVerdictHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void __we_test_begin(void);
void __we_test_end(void);
void __we_test_report(const char *, long long, const char *, long long, long long, const char *);
void __we_test_meta(const char *, long long, const char *, long long, long long, long long);
void __we_test_drive(long long n, void (*thunk)(void));
void __we_test_set_json(int on);
extern int __we_explore_on;
extern long long __we_explore_iters;
extern int __we_explore_reduce;

static void plain_thunk(void) {
    __we_test_begin();
    __we_test_end();
    __we_test_report("tests/p.we", 10, "probe", 5, 0, 0);
}

static long long runs;
static void flaky_thunk(void) { // passes twice, fails from the third attempt
    __we_test_begin();
    __we_test_end();
    __we_test_report("tests/o.we", 10, "order", 5, runs >= 4,
                     "assertion failed: got 2, want 1");
    runs++;
}

int we_main(void) {
    __we_explore_on = 1;
    __we_explore_iters = 3;
    __we_test_meta("tests/p.we", 10, "probe", 5, 3, 1);
    __we_test_drive(0, plain_thunk);
    __we_explore_reduce = 0;
    __we_test_meta("tests/q.we", 10, "probe", 5, 3, 1);
    __we_test_drive(1, plain_thunk);
    __we_explore_reduce = 1;
    runs = 0;
    __we_test_meta("tests/o.we", 10, "order", 5, 3, 1);
    __we_test_drive(2, flaky_thunk);
    __we_test_set_json(1);
    __we_explore_iters = 2;
    __we_test_meta("tests/p.we", 10, "probe", 5, 3, 1);
    __we_test_drive(3, plain_thunk);
    runs = 100; // every attempt fails now
    __we_test_meta("tests/o.we", 10, "order", 5, 3, 1);
    __we_test_drive(4, flaky_thunk);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM10cDriveVerdict(t *testing.T) {
	m10cRunBoth(t, driveVerdictHarness,
		"pass  tests/p.we: probe (explore 3 iterations, 9 attempts, 1 classes)\n"+
			"pass  tests/q.we: probe (explore 3 iterations, 3 attempts, 1 classes)\n"+
			"fail  tests/o.we: order (explore 3 iterations, 3 attempts, 1 classes)\n"+
			"  explored schedule failed: assertion failed: got 2, want 1\n"+
			"{\"type\":\"test-result\",\"file\":\"tests/p.we\",\"name\":\"probe\","+
			"\"status\":\"pass\",\"duration_ms\":0,"+
			"\"explore\":{\"iterations\":2,\"attempts\":5,\"classes\":1}}\n"+
			"{\"type\":\"test-result\",\"file\":\"tests/o.we\",\"name\":\"order\","+
			"\"status\":\"fail\",\"duration_ms\":0,"+
			"\"explore\":{\"iterations\":2,\"attempts\":1,\"classes\":0}}\n",
		"tests/o.we: order: explored schedule failed: assertion failed: got 2, want 1\n")
}

// guardDriveHarness pins the guard through the runner: the gate fires
// mid-run, half two never runs (the attempt is settled), the aggregate
// carries the guard's own message as the reason, and the diagnostic lands
// on stderr at the declaration position.
const guardDriveHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void __we_test_begin(void);
void __we_test_end(void);
void __we_test_report(const char *, long long, const char *, long long, long long, const char *);
void __we_test_meta(const char *, long long, const char *, long long, long long, long long);
void __we_test_drive(long long n, void (*thunk)(void));
void __we_explore_fx_check(const char *name, long long len);
extern int __we_explore_on;
extern long long __we_explore_iters;

static int runs;

static void drive_thunk(void) {
    __we_test_begin();
    runs++;
    __we_explore_fx_check("save", 4);
    __we_test_end();
    __we_test_report("tests/g.we", 10, "gate", 4, 0, 0);
}

int we_main(void) {
    __we_explore_on = 1;
    __we_explore_iters = 3;
    __we_test_meta("tests/g.we", 10, "gate", 4, 7, 1);
    __we_test_drive(0, drive_thunk);
    printf("runs=%d\n", runs);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM10cGuardThroughDrive(t *testing.T) {
	m10cRunBoth(t, guardDriveHarness,
		"fail  tests/g.we: gate (explore 3 iterations, 1 attempts, 0 classes)\n"+
			"  unmocked effect executed during exploration — the explored path"+
			" called save, whose custom effect had no mock\n"+
			"runs=1\n",
		"tests/g.we:7:1: error[E1902]: unmocked effect executed during"+
			" exploration — the explored path called save, whose custom effect"+
			" had no mock\n")
}

// deadlockHarness pins the conversion: a task parked with no wake source
// under the explored schedule settles the driving await with the
// deadlock message — the run fails, the exploration stops at the first
// attempt, and the process keeps running (the finding is a test outcome,
// not a crash). Two tasks are parked at the idle point: the stuck task
// and the driver's own await.
const deadlockHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_recv(void *ch, long long *out);
void __we_test_begin(void);
void __we_test_end(void);
void __we_test_report(const char *, long long, const char *, long long, long long, const char *);
void __we_test_meta(const char *, long long, const char *, long long, long long, long long);
void __we_test_drive(long long n, void (*thunk)(void));
extern int __we_explore_on;
extern long long __we_explore_iters;

static void *g_gate;

static long long stuck(void *env) {
    (void)env;
    long long v = 0;
    __we_chan_recv(g_gate, &v); // parks with no wake source, every schedule
    return 0;
}

static void drive_thunk(void) {
    __we_test_begin();
    void *h = __we_task_new(stuck, 0);
    long long v = 0;
    long long tag = __we_handle_await(h, &v); // the settle lands here
    __we_test_end();
    __we_test_report("tests/d.we", 10, "dead", 4, tag, (const char *)v);
}

int we_main(void) {
    g_gate = __we_prim_new_chan(0, 8, 0);
    __we_explore_on = 1;
    __we_explore_iters = 2;
    __we_test_meta("tests/d.we", 10, "dead", 4, 3, 1);
    __we_test_drive(0, drive_thunk);
    printf("still-alive\n");
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM10cDeadlockConvert(t *testing.T) {
	m10cRunBoth(t, deadlockHarness,
		"fail  tests/d.we: dead (explore 2 iterations, 1 attempts, 0 classes)\n"+
			"  explored schedule failed: deadlock: 2 tasks parked with no wake"+
			" source under the explored schedule\n"+
			"still-alive\n",
		"")
}

// ioSuppressHarness pins the discarded bytes: an explored process drops
// the io faces' output (the aggregate line replaces a hundred re-runs'
// interleavings) while the calls themselves still execute; a normal run
// writes as always.
const ioSuppressHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void __we_println(const char *s, long long n);
void __we_print(const char *s, long long n);
void __we_println_i64(long long v);
void __we_print_i64(long long v);
extern int __we_explore_on;

int we_main(void) {
    __we_println("NOISE", 5);
    __we_explore_on = 1;
    __we_println("HIDDEN", 6);
    __we_print("H2", 2);
    __we_println_i64(42);
    __we_print_i64(7);
    __we_explore_on = 0;
    __we_println("TAIL", 4);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestM10cIoSuppressed(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "str.h": StrHeader,
		"conc.c": ConcSource, "test.c": TestSource, "str.c": StrSource, "io.c": IOSource,
		"main.c": ioSuppressHarness,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	args := []string{"-o", filepath.Join(dir, "harness")}
	for _, in := range []string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "io.c", "main.c"} {
		args = append(args, filepath.Join(dir, in))
	}
	if out, err := exec.Command(pinnedClang(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("clang compile: %v\n%s", err, out)
	}
	got, err := exec.Command(filepath.Join(dir, "harness")).Output()
	if err != nil {
		t.Fatalf("harness run: %v\nstdout: %s", err, got)
	}
	if want := "NOISE\nTAIL\n"; string(got) != want {
		t.Fatalf("stdout mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}
