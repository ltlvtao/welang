package weruntime

import "testing"

// The String family's C harness (design D3): the runtime's __we_str_* —
// concatenation, equality, the two-layer access pair (chapter 17's byte
// and code-point layers), the value-to-string converters interpolation
// rides, and the two out-of-range panic faces (chapter 17's "a byte
// operation with an out-of-range range is a panic", chapter 14's family).
// The happy paths assert byte-for-byte through the CHECK macro; the panic
// faces need a task context (__we_task_fail reports through the calling
// task), so they ride the scheduler harness shape and assert the awaited
// message.

// strHarnessMain asserts the family's contracts in C, where the buffers
// live. A struct-returning entry is read through its two fields — the
// pair the emitter's IR takes as { ptr, i64 }.
const strHarnessMain = `#include <stdio.h>
#include "str.h"

void __we_gc_boot(void);

static int failures = 0;
#define CHECK(cond) do { \
    if (!(cond)) { printf("FAIL %d: %s\n", __LINE__, #cond); failures++; } \
} while (0)

int main(void) {
    __we_gc_boot();

    /* concatenation: the bytes in order, the length the sum; an empty
       operand contributes nothing; a multi-byte operand keeps its bytes. */
    struct we_str c = __we_str_concat("ab", 2, "cd", 2);
    CHECK(c.n == 4);
    CHECK(c.p[0] == 'a' && c.p[1] == 'b' && c.p[2] == 'c' && c.p[3] == 'd');
    struct we_str e = __we_str_concat("", 0, "", 0);
    CHECK(e.n == 0);
    struct we_str m = __we_str_concat("h", 1, "\xc3\xa9", 2);
    CHECK(m.n == 3 && (unsigned char)m.p[1] == 0xc3 && (unsigned char)m.p[2] == 0xa9);

    /* equality: same bytes equal; a differing byte or a differing length
       (the prefix case) not equal; empty equals empty. */
    CHECK(__we_str_eq("abc", 3, "abc", 3) == 1);
    CHECK(__we_str_eq("abc", 3, "abd", 3) == 0);
    CHECK(__we_str_eq("ab", 2, "abc", 3) == 0);
    CHECK(__we_str_eq("", 0, "", 0) == 1);
    CHECK(__we_str_eq("a\x00b", 3, "a\x00c", 3) == 0);

    /* runeCount: code points, not bytes (chapter 17's two layers). */
    CHECK(__we_str_runecount("h\xc3\xa9llo", 6) == 5);
    CHECK(__we_str_runecount("", 0) == 0);
    CHECK(__we_str_runecount("\xf0\x9f\x98\x80", 4) == 1);

    /* charAt: the code point at a code-point index. */
    CHECK(__we_str_charat("h\xc3\xa9llo", 6, 0) == 'h');
    CHECK(__we_str_charat("h\xc3\xa9llo", 6, 1) == 0xe9);
    CHECK(__we_str_charat("h\xc3\xa9llo", 6, 4) == 'o');
    CHECK(__we_str_charat("\xf0\x9f\x98\x80", 4, 0) == 0x1f600);
    CHECK(__we_str_charat("ab", 2, 1) == 'b');

    /* byteSlice: the span start..end, end exclusive; a zero-width span is
       the empty string; a full span is the whole value. */
    struct we_str s = __we_str_byteslice("hello", 5, 0, 3);
    CHECK(s.n == 3 && s.p[0] == 'h' && s.p[2] == 'l');
    struct we_str z = __we_str_byteslice("hello", 5, 2, 2);
    CHECK(z.n == 0);
    struct we_str f = __we_str_byteslice("hello", 5, 0, 5);
    CHECK(f.n == 5 && f.p[4] == 'o');

    /* of_i64: decimal, the sign carried, the minimum value in range. */
    struct we_str i0 = __we_str_of_i64(0);
    CHECK(i0.n == 1 && i0.p[0] == '0');
    struct we_str i42 = __we_str_of_i64(42);
    CHECK(i42.n == 2 && i42.p[0] == '4' && i42.p[1] == '2');
    struct we_str ineg = __we_str_of_i64(-7);
    CHECK(ineg.n == 2 && ineg.p[0] == '-' && ineg.p[1] == '7');
    struct we_str imin = __we_str_of_i64(-9223372036854775807LL - 1);
    CHECK(imin.n == 20 && imin.p[0] == '-');

    /* of_u64: the unsigned view — no sign, the full inventory in range. */
    struct we_str u0 = __we_str_of_u64(0);
    CHECK(u0.n == 1 && u0.p[0] == '0');
    struct we_str u42 = __we_str_of_u64(42);
    CHECK(u42.n == 2 && u42.p[0] == '4' && u42.p[1] == '2');
    struct we_str umax = __we_str_of_u64(18446744073709551615ULL);
    CHECK(umax.n == 20 && umax.p[0] == '1');
    /* 9223372036854775808 is i64's minimum read unsigned: the same twenty
       digits of its true magnitude, no sign. */
    struct we_str ubig = __we_str_of_u64(9223372036854775808ULL);
    CHECK(ubig.n == 19 && ubig.p[0] == '9');

    /* of_f64: the shortest decimal that reads back as the same double. */
    struct we_str f1 = __we_str_of_f64(0.1);
    CHECK(f1.n == 3 && f1.p[0] == '0' && f1.p[1] == '.' && f1.p[2] == '1');
    struct we_str f2 = __we_str_of_f64(1.0);
    CHECK(f2.n == 1 && f2.p[0] == '1');
    struct we_str f3 = __we_str_of_f64(-2.25);
    CHECK(f3.n == 5 && f3.p[0] == '-');
    struct we_str f4 = __we_str_of_f64(1.0 / 3.0);
    CHECK(f4.n == 18);
    struct we_str f5 = __we_str_of_f64(0.0);
    CHECK(f5.n == 1 && f5.p[0] == '0');

    /* of_bool and of_rune: the literal words, and the code point's UTF-8. */
    struct we_str bt = __we_str_of_bool(1);
    CHECK(bt.n == 4 && bt.p[0] == 't');
    struct we_str bf = __we_str_of_bool(0);
    CHECK(bf.n == 5 && bf.p[0] == 'f');
    struct we_str r1 = __we_str_of_rune('A');
    CHECK(r1.n == 1 && r1.p[0] == 'A');
    struct we_str r2 = __we_str_of_rune(0xe9);
    CHECK(r2.n == 2 && (unsigned char)r2.p[0] == 0xc3);
    struct we_str r3 = __we_str_of_rune(0x4e16);
    CHECK(r3.n == 3 && (unsigned char)r3.p[0] == 0xe4);
    struct we_str r4 = __we_str_of_rune(0x1f600);
    CHECK(r4.n == 4 && (unsigned char)r4.p[0] == 0xf0);

    if (failures == 0) {
        printf("str harness: all checks passed\n");
    }
    return failures != 0;
}
`

// strPanicHarness pins the two out-of-range faces (chapter 17): charAt past
// the code-point count and byteSlice past the byte length each terminate
// the calling task through chapter 14's family, carrying a message that
// names the operation. The await yields tag 1 and the message pointer.
const strPanicHarness = `#include <stdio.h>
#include "str.h"

void __we_sched_boot(int (*main_fn)(void));
void *__we_task_new(long long (*thunk)(void *), void *env);
long long __we_handle_await(void *h, long long *payload);

static long long boom_charat(void *env) {
    (void)env;
    __we_str_charat("hello", 5, 7);
    return 0;
}

static long long boom_byteslice(void *env) {
    (void)env;
    __we_str_byteslice("hello", 5, 0, 99);
    return 0;
}

static void report(const char *what, long long (*thunk)(void *)) {
    void *h = __we_task_new(thunk, 0);
    long long v = 0;
    long long tag = __we_handle_await(h, &v);
    printf("%s tag=%lld msg=%s\n", what, tag, (const char *)v);
}

int we_main(void) {
    report("charAt", boom_charat);
    report("byteSlice", boom_byteslice);
    return 0;
}

int main(void) { __we_sched_boot(we_main); return 0; }
`

func TestStrHarness(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"conc.c": ConcSource, "test.c": TestSource, "str.h": StrHeader,
			"str.c": StrSource, "main.c": strHarnessMain,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"},
		"str harness: all checks passed\n")
}

func TestStrPanicHarness(t *testing.T) {
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"conc.c": ConcSource, "test.c": TestSource, "str.h": StrHeader,
			"str.c": StrSource, "main.c": strPanicHarness,
		},
		[]string{"gc.c", "sched.c", "conc.c", "test.c", "str.c", "main.c"},
		"charAt tag=1 msg=String charAt out of range\nbyteSlice tag=1 msg=String byteSlice out of range\n")
}
