package weruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// M10b design D4/D7: the virtual clock and the assertion helpers answer in
// C, where the scheduler lives. The state machine (begin resets, advance
// moves, end restores the real clock), the same-instant FIFO release, the
// dual-source deadline (a virtual sleep parks on the virtual clock, a real
// sleep actually waits), the leftover sweep at end, each helper's report
// line, and the virtual-domain deadlock abort — all pinned through the
// same embed-and-clang face the M9b harnesses ride. Written test-first:
// test.c does not exist yet, so these fail to compile here.

// clockStateHarness pins the state machine: begin resets vnow to zero,
// advance moves it, a second begin resets it again, and end restores the
// real clock (a wall reading, orders of magnitude past the virtual face).
const clockStateHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void __we_test_begin(void);
void __we_test_end(void);
void __we_advance(long long ms);
long long __we_time_now(void);

int we_main(void) {
    __we_test_begin();
    long long a = __we_time_now();
    __we_advance(50);
    long long b = __we_time_now();
    __we_test_end();
    __we_test_begin();
    long long c = __we_time_now();
    __we_advance(5);
    long long d = __we_time_now();
    __we_test_end();
    long long e = __we_time_now(); /* real: wall milliseconds, huge */
    printf("a=%lld b=%lld c=%lld d=%lld real=%lld\n", a, b, c, d, e > 1000000);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// clockFifoHarness pins the same-instant release: two tasks sleeping the
// same virtual duration wake in registration order (design D4's FIFO).
const clockFifoHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
long long __we_yield(void);
void __we_test_begin(void);
void __we_test_end(void);
void __we_advance(long long ms);
long long __we_time_now(void);
void __we_time_sleep(long long ms);

static char woke[8];
static int woke_n;

static long long sleeper(void *env) {
    __we_time_sleep(50);
    woke[woke_n++] = '0' + (int)(long long)env;
    return 0;
}

int we_main(void) {
    __we_test_begin();
    void *h1 = __we_task_new(sleeper, (void *)1);
    void *h2 = __we_task_new(sleeper, (void *)2);
    __we_yield(); /* both park at virtual deadline 50 */
    __we_advance(100);
    long long v = 0;
    __we_handle_await(h1, &v);
    __we_handle_await(h2, &v);
    long long now = __we_time_now();
    __we_test_end();
    woke[woke_n] = 0;
    printf("woke=%s now=%lld\n", woke, now);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// clockDualHarness pins the dual source: a huge virtual sleep releases the
// instant the clock advances past it (no wall time at all), and after end a
// real sleep of 30 ms actually waits on the wall clock.
const clockDualHarness = `#include <stdio.h>
#include <time.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
long long __we_yield(void);
void __we_test_begin(void);
void __we_test_end(void);
void __we_advance(long long ms);
void __we_time_sleep(long long ms);

static long long real_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long long)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}

static long long vsleeper(void *env) {
    __we_time_sleep(10000000);
    return 7;
}

int we_main(void) {
    __we_test_begin();
    void *h = __we_task_new(vsleeper, 0);
    __we_yield();
    long long t0 = real_ms();
    __we_advance(10000000);
    long long vdt = real_ms() - t0; /* ~0: the virtual release is free */
    long long v = 0;
    long long tag = __we_handle_await(h, &v);
    __we_test_end();
    long long r0 = real_ms();
    __we_time_sleep(30); /* real mode: the wall clock owns the deadline */
    long long rdt = real_ms() - r0;
    printf("tag=%lld val=%lld vdt<5=%lld rdt>20=%lld\n", tag, v, vdt < 5, rdt > 20);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// clockScopeHarness pins the scope deadline's virtual face: a scope entered
// inside the virtual domain carries its deadline on the virtual clock, so
// the advance that passes it expires the scope (leave reports TimedOut)
// without any wall time passing.
const clockScopeHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
void *__we_scope_enter(long long deadline_ms, long long collect_all);
long long __we_scope_leave(void *scope);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_recv(void *ch, long long *out);
void __we_test_begin(void);
void __we_test_end(void);
void __we_advance(long long ms);

static void *g_ch;

static long long parked(void *env) {
    long long v = 0;
    __we_chan_recv(g_ch, &v); /* parks: only the scope expiry cancels it */
    return 0;
}

int we_main(void) {
    g_ch = __we_prim_new_chan(1, 8, 0);
    __we_test_begin();
    void *sc = __we_scope_enter(50, 0); /* virtual deadline 50 */
    __we_task_new(parked, 0);
    __we_advance(100); /* passes the deadline: expiry cancels the parked task */
    long long r = __we_scope_leave(sc);
    __we_test_end();
    printf("timedout=%lld\n", r);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// clockLeftoverHarness pins the sweep as abandonment: a task spawned inside
// the test that never finishes is never run again — a parked one stays
// parked, a queued one leaves the ready queue — so its body's remaining
// statements never execute, the driver proceeds, and a second test begins
// from a quiet task face and a zero clock.
const clockLeftoverHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_yield(void);
void *__we_prim_new_chan(long long cap, long long slot_width, void *desc);
long long __we_chan_recv(void *ch, long long *out);
void __we_test_begin(void);
void __we_test_end(void);
void __we_advance(long long ms);
long long __we_time_now(void);

static void *g_ch;
static int ran_parked, ran_queued;

static long long parked(void *env) {
    long long v = 0;
    __we_chan_recv(g_ch, &v); /* parks forever inside the test */
    ran_parked = 1;
    return 0;
}

static long long queued(void *env) {
    ran_queued = 1; /* spawned late: still queued when the test ends */
    return 0;
}

int we_main(void) {
    g_ch = __we_prim_new_chan(1, 8, 0);
    __we_test_begin();
    __we_task_new(parked, 0);
    __we_yield();               /* it runs into its park */
    __we_advance(10);           /* unrelated tick: it stays parked */
    __we_task_new(queued, 0);   /* never scheduled before the end */
    __we_test_end();            /* the sweep abandons both */
    __we_test_begin();          /* the next test begins from zero */
    long long now = __we_time_now();
    __we_test_end();
    printf("ran_parked=%d ran_queued=%d now=%lld\n", ran_parked, ran_queued, now);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// clockDrainHarness pins the crossing barrier: advanceTime first runs
// every ready task to its next block, so a spawned-but-unscheduled task
// registers its wait before the clock moves and the advance releases it
// at the crossed instant — without the barrier the sleep would register
// against the already-moved clock and never come due (a virtual-domain
// deadlock, not a pass).
const clockDrainHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void __we_test_begin(void);
void __we_test_end(void);
void __we_advance(long long ms);
void __we_time_sleep(long long ms);
long long __we_time_now(void);

static int ran;

static long long sleeper(void *env) {
    __we_time_sleep(100); /* registers once the barrier runs it */
    ran = 1;
    return 0;
}

int we_main(void) {
    __we_test_begin();
    void *h = __we_task_new(sleeper, 0); /* queued, never yielded to */
    long long t0 = __we_time_now();
    __we_advance(100); /* the barrier runs it, then crosses */
    long long v = 0;
    __we_handle_await(h, &v);
    long long now = __we_time_now();
    __we_test_end();
    printf("ran=%d now=%lld t0=%lld\n", ran, now, t0);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// assertHelpersHarness pins each helper's report line (design D7): the
// failing faces ride task_fail's stored message out through await, the
// passing faces return normally.
const assertHelpersHarness = `#include <stdio.h>
#include <string.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void __we_assert_true(long long cond);
void __we_assert_false(long long cond);
void __we_assert_eq_i64(long long got, long long want);
void __we_assert_eq_str(const char *gp, long long gl, const char *wp, long long wl);
void __we_assert_eq_i64_at(const char *path, long long got, long long want);
void __we_assert_eq_u64_at(const char *path, unsigned long long got, unsigned long long want);
void __we_assert_eq_bool_at(const char *path, long long got, long long want);
void __we_assert_eq_str_at(const char *path, const char *gp, long long gl, const char *wp, long long wl);
void __we_assert_eq_variant_at(const char *path, const char *got, const char *want);

static long long f_true(void *env) { __we_assert_true(0); return 0; }
static long long f_false(void *env) { __we_assert_false(1); return 0; }
static long long f_eq_i64(void *env) { __we_assert_eq_i64(3, 4); return 0; }
static long long f_eq_str(void *env) { __we_assert_eq_str("abc", 3, "abd", 3); return 0; }
static long long f_pass(void *env) {
    __we_assert_true(1);
    __we_assert_false(0);
    __we_assert_eq_i64(4, 4);
    __we_assert_eq_str("abc", 3, "abc", 3);
    return 0;
}

// T10 (design D9): the positioned rows a composite comparison reports
// through. A null path is a top-level leaf, whose line is the pre-T10 one;
// a path prefixes the position, and nothing follows it but the leaf's own
// wording.
static long long f_eq64_at(void *env) { __we_assert_eq_i64_at("Point.x", 3, 4); return 0; }
static long long f_eq64_top(void *env) { __we_assert_eq_i64_at(0, 3, 4); return 0; }
static long long f_eq_u64(void *env) { __we_assert_eq_u64_at(0, 18446744073709551615ULL, 6); return 0; }
static long long f_eq_bool(void *env) { __we_assert_eq_bool_at("P.flag", 1, 0); return 0; }
static long long f_eq_str_at(void *env) {
    __we_assert_eq_str_at("P.name", "ab", 2, "cd", 2);
    return 0;
}
// The sum row reports the variants unquoted: they are names the compiler
// wrote into the position, not values the program holds.
static long long f_eq_variant(void *env) {
    __we_assert_eq_variant_at("Shape", "Circle", "Rect");
    return 0;
}
static long long f_pass2(void *env) {
    __we_assert_eq_i64_at("P.x", 4, 4);
    __we_assert_eq_u64_at("P.n", 7, 7);
    __we_assert_eq_bool_at("P.b", 0, 0);
    __we_assert_eq_str_at("P.s", "ab", 2, "ab", 2);
    return 0;
}

static void report(const char *what, long long (*thunk)(void *)) {
    long long v = 0;
    long long tag = __we_handle_await(__we_task_new(thunk, 0), &v);
    printf("%s tag=%lld msg=%s\n", what, tag, tag ? (const char *)v : "-");
}

int we_main(void) {
    report("true:", f_true);
    report("false:", f_false);
    report("eq64:", f_eq_i64);
    report("eqstr:", f_eq_str);
    report("pass:", f_pass);
    report("eq64at:", f_eq64_at);
    report("eq64top:", f_eq64_top);
    report("equ64:", f_eq_u64);
    report("eqbool:", f_eq_bool);
    report("eqstrat:", f_eq_str_at);
    report("eqvar:", f_eq_variant);
    report("pass2:", f_pass2);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// clockDeadlockHarness pins the virtual-domain abort: inside a test, a
// task parked on the virtual clock with nobody left to advance it is a
// dead run — one stderr line naming the virtual clock, exit 1 (the count
// is the clock-parked tasks; the awaiting driver is not a clock wait).
const clockDeadlockHarness = `#include <stdio.h>

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);
void __we_test_begin(void);
void __we_time_sleep(long long ms);

static long long stuck(void *env) {
    __we_time_sleep(1000); /* never advanced */
    return 0;
}

int we_main(void) {
    __we_test_begin();
    void *h = __we_task_new(stuck, 0);
    long long v = 0;
    __we_handle_await(h, &v); /* the driver parks on the join, not the clock */
    printf("unreachable\n");
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

// reportHarness pins the report face (design D5/D6): one passing test
// with a 50ms virtual crossing, one failing test with a stored message —
// the human lines and the JSON Lines both byte-pinned, the failure's
// reason on stderr in JSON mode (R7's field set is closed), the summary
// summing the virtual durations and owning the exit code (0 when the
// all-pass scene runs, 1 otherwise).
const reportHarness = `#include <stdio.h>
#include <string.h>

void __we_sched_boot(int (*main_fn)(void));
void __we_test_begin(void);
void __we_test_end(void);
void __we_test_report(const char *, long long, const char *, long long, long long, const char *);
void __we_test_summary(void);
void __we_test_set_json(int);
void __we_advance(long long ms);

static int scene_pass;

static void one(const char *name, long long len, int cross, long long failed, const char *reason) {
    __we_test_begin();
    __we_advance(cross);
    __we_test_end();
    __we_test_report("tests/m_test.we", 15, name, len, failed, reason);
}

int we_main(void) {
    if (scene_pass) {
        one("ticks", 5, 50, 0, 0);
        one("fine", 4, 25, 0, 0);
        __we_test_summary();
        return 0;
    }
    one("ticks", 5, 50, 0, 0);
    one("boom", 4, 0, 1, "kaboom");
    __we_test_summary();
    return 0;
}

int main(int argc, char **argv) {
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--json") == 0) {
            __we_test_set_json(1);
        } else if (strcmp(argv[i], "--pass") == 0) {
            scene_pass = 1;
        }
    }
    __we_sched_boot(we_main);
    return 0;
}
`

// m10bSources is the compile set of the M10b harnesses: the M9b runtime
// plus test.c (the scheduler now reads the clock face test.c owns, so no
// link resolves without it).
func m10bSources(mainSrc string) (map[string]string, []string) {
	return map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader, "str.h": StrHeader,
			"conc.c": ConcSource, "test.c": TestSource, "str.c": StrSource, "main.c": mainSrc,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"}
}

// runReport builds the report harness once and runs one scene: the
// binary's stdout, stderr, and exit code all pinned.
func runReport(t *testing.T, args []string, wantOut, wantErr string, wantExit int) {
	t.Helper()
	m, in := m10bSources(reportHarness)
	dir := t.TempDir()
	for name, src := range m {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	clargs := []string{"-o", filepath.Join(dir, "harness")}
	for _, in := range in {
		clargs = append(clargs, filepath.Join(dir, in))
	}
	if out, err := exec.Command(pinnedClang(t), clargs...).CombinedOutput(); err != nil {
		t.Fatalf("clang compile: %v\n%s", err, out)
	}
	cmd := exec.Command(filepath.Join(dir, "harness"), args...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if wantExit == 0 && err != nil {
		t.Fatalf("run %v: %v\nstdout: %s", args, err, stdout.String())
	}
	if wantExit != 0 {
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != wantExit {
			t.Fatalf("run %v: want exit %d, got %v", args, wantExit, err)
		}
	}
	if stdout.String() != wantOut {
		t.Fatalf("stdout mismatch (%v):\nwant: %q\ngot:  %q", args, wantOut, stdout.String())
	}
	if stderr.String() != wantErr {
		t.Fatalf("stderr mismatch (%v):\nwant: %q\ngot:  %q", args, wantErr, stderr.String())
	}
}

func TestM10bReportHuman(t *testing.T) {
	runReport(t, nil,
		"pass  tests/m_test.we: ticks (50ms)\n"+
			"fail  tests/m_test.we: boom (0ms)\n"+
			"  kaboom\n"+
			"total 2, passed 1, failed 1 (50ms)\n", "", 1)
}

func TestM10bReportHumanAllPass(t *testing.T) {
	runReport(t, []string{"--pass"},
		"pass  tests/m_test.we: ticks (50ms)\n"+
			"pass  tests/m_test.we: fine (25ms)\n"+
			"total 2, passed 2, failed 0 (75ms)\n", "", 0)
}

func TestM10bReportJSON(t *testing.T) {
	runReport(t, []string{"--json"},
		"{\"type\":\"test-result\",\"file\":\"tests/m_test.we\",\"name\":\"ticks\",\"status\":\"pass\",\"duration_ms\":50}\n"+
			"{\"type\":\"test-result\",\"file\":\"tests/m_test.we\",\"name\":\"boom\",\"status\":\"fail\",\"duration_ms\":0}\n"+
			"{\"type\":\"test-summary\",\"total\":2,\"passed\":1,\"failed\":1,\"duration_ms\":50}\n",
		"tests/m_test.we: boom: kaboom\n", 1)
}

func TestM10bClockStateMachine(t *testing.T) {
	m, in := m10bSources(clockStateHarness)
	compileAndRun(t, m, in, "a=0 b=50 c=0 d=5 real=1\n")
}

func TestM10bClockFifo(t *testing.T) {
	m, in := m10bSources(clockFifoHarness)
	compileAndRun(t, m, in, "woke=12 now=100\n")
}

func TestM10bClockDualSource(t *testing.T) {
	m, in := m10bSources(clockDualHarness)
	compileAndRun(t, m, in, "tag=0 val=7 vdt<5=1 rdt>20=1\n")
}

func TestM10bClockScopeVirtualDeadline(t *testing.T) {
	m, in := m10bSources(clockScopeHarness)
	compileAndRun(t, m, in, "timedout=1\n")
}

func TestM10bClockLeftoverSweep(t *testing.T) {
	m, in := m10bSources(clockLeftoverHarness)
	compileAndRun(t, m, in, "ran_parked=0 ran_queued=0 now=0\n")
}

func TestM10bClockCrossingBarrier(t *testing.T) {
	m, in := m10bSources(clockDrainHarness)
	compileAndRun(t, m, in, "ran=1 now=100 t0=0\n")
}

func TestM10bAssertHelpers(t *testing.T) {
	m, in := m10bSources(assertHelpersHarness)
	compileAndRun(t, m, in,
		"true: tag=1 msg=assertion failed\n"+
			"false: tag=1 msg=assertion failed: expected false\n"+
			"eq64: tag=1 msg=assertion failed: got 3, want 4\n"+
			"eqstr: tag=1 msg=assertion failed: got \"abc\", want \"abd\"\n"+
			"pass: tag=0 msg=-\n"+
			// T10: the same two shapes carrying a position, the two
			// families it renders afresh, and the null path that leaves
			// the line above byte for byte what it was.
			"eq64at: tag=1 msg=assertion failed: at Point.x: got 3, want 4\n"+
			"eq64top: tag=1 msg=assertion failed: got 3, want 4\n"+
			"equ64: tag=1 msg=assertion failed: got 18446744073709551615, want 6\n"+
			"eqbool: tag=1 msg=assertion failed: at P.flag: got true, want false\n"+
			"eqstrat: tag=1 msg=assertion failed: at P.name: got \"ab\", want \"cd\"\n"+
			"eqvar: tag=1 msg=assertion failed: at Shape: got Circle, want Rect\n"+
			"pass2: tag=0 msg=-\n")
}

func TestM10bClockDeadlock(t *testing.T) {
	m, _ := m10bSources(clockDeadlockHarness)
	dir := t.TempDir()
	for name, src := range m {
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
	var stderr strings.Builder
	cmd := exec.Command(filepath.Join(dir, "harness"))
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("want exit 1, got %v", err)
	}
	want := "we: deadlock: 1 tasks parked with no wake source (the virtual clock only advances when a runnable task calls advanceTime)\n"
	if stderr.String() != want {
		t.Fatalf("stderr mismatch:\nwant: %q\ngot:  %q", want, stderr.String())
	}
}
