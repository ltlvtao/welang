package weruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/version"
)

// The C harness (design D6/D7): the runtime sources compile with the pinned
// clang and answer for their contracts — the allocation ABI and header
// layout, the shadow-stack root protocol, precise reachability through the
// map descriptor's reference slots, sweep returning blocks to the free
// list, and the io pair's exact stdout bytes. The embed variables carry the
// sources, so a missing gc.c/io.c fails here at compile time first.

// gcHarnessMain asserts the collector's contracts in C, where the objects
// live. CHECK prints one line per failure; the exit code is the count's
// truth value (0 = all passed).
const gcHarnessMain = `#include <stdio.h>

void *__we_alloc(long long n);
void __we_free(void *p);
void __we_gc_boot(void);
long long __we_gc_collect(void);
void __we_root_push(void *p);
void __we_root_pop(void);

static int failures = 0;
#define CHECK(cond) do { \
    if (!(cond)) { printf("FAIL %d: %s\n", __LINE__, #cond); failures++; } \
} while (0)

/* Map descriptors as the compiler emits them: one bitmap word per 64
   reference slots, bit i set when the word at offset 16 + 8*i is a gc
   reference. The parent's two slots are references; the leaf's are not. */
static const long long map_parent[] = {3};
static const long long map_leaf[] = {0};

int main(void) {
    __we_gc_boot();

    /* Allocation basics: distinct non-null blocks; the allocator writes
       the header's size word and leaves the map slot zeroed (zeroed
       blocks keep partially-filled objects mark-safe). */
    char *a = __we_alloc(32);
    char *b = __we_alloc(32);
    CHECK(a != 0);
    CHECK(b != 0 && a != b);
    CHECK(*(long long *)(a + 8) == 32);
    CHECK(*(void **)a == 0);

    /* An explicit collect with nothing rooted sweeps both. */
    CHECK(__we_gc_collect() == 2);

    /* Rooted survival: the parent is rooted, the leaf is not — the leaf
       stays reachable through the parent's first reference slot, and the
       payload words come through the collection intact. */
    char *parent = __we_alloc(32);
    *(void **)parent = (void *)map_parent;
    char *leaf = __we_alloc(32);
    *(void **)leaf = (void *)map_leaf;
    *(const char **)(leaf + 16) = "leaf-payload";
    *(long long *)(leaf + 24) = 11;
    __we_root_push(parent);
    *(void **)(parent + 16) = leaf;
    CHECK(__we_gc_collect() == 0);
    CHECK(*(long long *)(leaf + 24) == 11);

    /* Unrooted, both go: the root stack, not liveness analysis, is the
       root set. */
    __we_root_pop();
    CHECK(__we_gc_collect() == 2);

    /* Swept blocks return to the allocator: same-size allocation
       succeeds again. */
    char *again = __we_alloc(32);
    CHECK(again != 0);

    if (failures == 0) {
        printf("gc harness: all checks passed\n");
    }
    return failures != 0;
}
`

// ioHarnessMain pins the io pair's observable bytes: println writes the
// bytes then one newline, print writes the bytes alone — unbuffered
// semantics are the flush-timing contract (D7).
const ioHarnessMain = `void __we_println(const char *s, long long n);
void __we_print(const char *s, long long n);

int main(void) {
    __we_println("hi", 2);
    __we_print("ho", 2);
    return 0;
}
`

// pinnedClang mirrors the CLI's version gate: the runtime sources compile
// with the same pinned toolchain the build pipeline uses.
func pinnedClang(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("clang", "--version").CombinedOutput()
	if err != nil {
		t.Skipf("clang unavailable: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for i := 0; i+2 < len(fields); i++ {
			if fields[i] == "clang" && fields[i+1] == "version" {
				if fields[i+2] != version.LLVMPin {
					t.Skipf("clang reports %q, pin is %q", fields[i+2], version.LLVMPin)
				}
				return "clang"
			}
		}
	}
	t.Skipf("clang --version output carries no version line")
	return ""
}

// compileAndRun writes the given sources (compile order = the inputs
// slice), compiles them with the pinned clang, runs the binary, and
// compares stdout byte-for-byte.
func compileAndRun(t *testing.T, sources map[string]string, inputs []string, wantStdout string) {
	t.Helper()
	dir := t.TempDir()
	for name, src := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	args := []string{"-o", filepath.Join(dir, "harness")}
	for _, in := range inputs {
		args = append(args, filepath.Join(dir, in))
	}
	if out, err := exec.Command(pinnedClang(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("clang compile: %v\n%s", err, out)
	}
	got, err := exec.Command(filepath.Join(dir, "harness")).Output()
	if err != nil {
		t.Fatalf("harness run: %v\nstdout: %s", err, got)
	}
	if string(got) != wantStdout {
		t.Fatalf("stdout mismatch:\nwant: %q\ngot:  %q", wantStdout, got)
	}
}

func TestGCHarness(t *testing.T) {
	compileAndRun(t,
		map[string]string{"gc.c": GCSource, "main.c": gcHarnessMain},
		[]string{"gc.c", "main.c"},
		"gc harness: all checks passed\n")
}

func TestIOHarness(t *testing.T) {
	// io.c's explore-suppression branch references test.c's __we_explore_on,
	// so the harness links the sched/test faces alongside (M10c); the io
	// behavior under test is unchanged.
	compileAndRun(t,
		map[string]string{
			"gc.c": GCSource, "sched.c": SchedSource, "sched.h": SchedHeader,
			"test.c": TestSource, "io.c": IOSource, "main.c": ioHarnessMain,
		},
		[]string{"gc.c", "sched.c", "test.c", "io.c", "main.c"},
		"hi\nho")
}
