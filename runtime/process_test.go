package weruntime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// procHarnessMain walks the process entry through both halves: every Ok
// face (the echo capture, the PATH search a slash-less command takes, the
// non-zero exits, the stderr capture, both pipes at once, the signalled
// death's 128+9, and the deadlock face — 70000 bytes down stderr, past one
// pipe's capacity, before stdout's first byte, which a sequential reader
// answers by hanging) and every Err face (the missing command's exec
// report and both NUL rejects). The Ok reader prints the record at the
// layout a We-side construction takes — exit code at 16, the pairs at
// 24/32 and 40/48 — pinning the C-side carve against the emitter's. The
// harness reads each answer before the next face allocates, mirroring the
// call-site re-root contract the entry leans on.
const procHarnessMain = `#include <stdint.h>
#include <stdio.h>
#include <string.h>

void *__we_alloc(long long n);
void *__we_list_new(long long cap, long long traced);
void *__we_list_push(void *l, long long v);
void __we_root_push(void *p);
void __we_proc_run(const char *cmd, long long cmdn, void *args, long long out[3]);

// prec prints one Ok half: the exit code, then both captured pairs between
// brackets; an Err half prints its message instead.
static void prec(const char *what, long long out[3]) {
    printf("%s=%lld", what, out[0]);
    if (out[0] == 0) {
        char *rec = (char *)(uintptr_t)out[1];
        printf("[%lld][", *(long long *)(rec + 16));
        fwrite(*(const char **)(rec + 24), 1, (size_t)*(long long *)(rec + 32), stdout);
        printf("][");
        fwrite(*(const char **)(rec + 40), 1, (size_t)*(long long *)(rec + 48), stdout);
        printf("]\n");
    } else {
        printf("[");
        fwrite((const char *)(uintptr_t)out[1], 1, (size_t)out[2], stdout);
        printf("]\n");
    }
}

// mkargs builds one List<String> carrier the way the emitter's literal
// does: traced (the element words are boxes), each element a 32-byte box
// holding the String pair at 16/24, the carrier rooted — the capacity is
// the count, so no push ever grows it.
static void *mkargs(const char *const *ss, long long n) {
    void *l = __we_list_new(n, 1);
    __we_root_push(l);
    for (long long i = 0; i < n; i++) {
        void *box = __we_alloc(32);
        *(const char **)((char *)box + 16) = ss[i];
        *(long long *)((char *)box + 24) = (long long)strlen(ss[i]);
        l = __we_list_push(l, (long long)(uintptr_t)box);
    }
    return l;
}

int main(void) {
    long long out[3];

    const char *echo_args[] = {"hi"};
    __we_proc_run("/bin/echo", 9, mkargs(echo_args, 1), out);
    prec("echo", out);

    const char *path_args[] = {"via-path"};
    __we_proc_run("echo", 4, mkargs(path_args, 1), out);
    prec("path", out);

    __we_proc_run("/bin/false", 10, mkargs(NULL, 0), out);
    prec("false", out);

    const char *exit_args[] = {"-c", "exit 3"};
    __we_proc_run("/bin/sh", 7, mkargs(exit_args, 2), out);
    prec("exit3", out);

    const char *err_args[] = {"-c", "echo oops 1>&2"};
    __we_proc_run("/bin/sh", 7, mkargs(err_args, 2), out);
    prec("stderr", out);

    const char *both_args[] = {"-c", "echo out; echo err 1>&2"};
    __we_proc_run("/bin/sh", 7, mkargs(both_args, 2), out);
    prec("both", out);

    const char *sig_args[] = {"-c", "kill -9 $$"};
    __we_proc_run("/bin/sh", 7, mkargs(sig_args, 2), out);
    prec("sig", out);

    // The deadlock face: lengths only, so the want line stays readable.
    const char *big_args[] = {"-c", "head -c 70000 /dev/zero 1>&2; echo tail"};
    __we_proc_run("/bin/sh", 7, mkargs(big_args, 2), out);
    printf("big=%lld code=%lld out=%lld err=%lld\n", out[0],
        out[0] == 0 ? *(long long *)((char *)(uintptr_t)out[1] + 16) : 0LL,
        out[0] == 0 ? *(long long *)((char *)(uintptr_t)out[1] + 32) : 0LL,
        out[0] == 0 ? *(long long *)((char *)(uintptr_t)out[1] + 48) : 0LL);

    __we_proc_run("/no/such/cmd", 12, mkargs(NULL, 0), out);
    prec("missing", out);

    __we_proc_run("a\0b", 3, mkargs(NULL, 0), out);
    prec("nul-cmd", out);

    // The NUL reject for an argument needs a hand-carved box: a C string
    // cannot carry the byte past its terminator.
    void *nulargs = __we_list_new(1, 1);
    __we_root_push(nulargs);
    void *nb = __we_alloc(32);
    *(const char **)((char *)nb + 16) = "a\0b";
    *(long long *)((char *)nb + 24) = 3;
    nulargs = __we_list_push(nulargs, (long long)(uintptr_t)nb);
    __we_proc_run("/bin/echo", 9, nulargs, out);
    prec("nul-arg", out);
    return 0;
}
`

// TestProcHarness compiles the process entry with its link set (the
// collector, the scheduler test.c owns the clock face of, and the list
// carrier the argument list crosses as) and runs it with the harness's
// working directory inside the temp dir.
func TestProcHarness(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"gc.c": GCSource, "sched.h": SchedHeader, "sched.c": SchedSource,
		"test.c": TestSource, "str.h": StrHeader, "str.c": StrSource,
		"list.h": ListHeader, "list.c": ListSource,
		"process.c": ProcessSource, "main.c": procHarnessMain,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	args := []string{"-o", filepath.Join(dir, "harness")}
	for _, in := range []string{"gc.c", "sched.c", "test.c", "str.c", "list.c", "process.c", "main.c"} {
		args = append(args, filepath.Join(dir, in))
	}
	if out, err := exec.Command(pinnedClang(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("clang compile: %v\n%s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), harnessRun)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(dir, "harness"))
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("harness run: %v\nstdout: %s", err, out)
	}
	want := "echo=0[0][hi\n][]\n" +
		"path=0[0][via-path\n][]\n" +
		"false=0[1][][]\n" +
		"exit3=0[3][][]\n" +
		"stderr=0[0][][oops\n]\n" +
		"both=0[0][out\n][err\n]\n" +
		"sig=0[137][][]\n" +
		"big=0 code=0 out=5 err=70000\n" +
		"missing=1[exec /no/such/cmd: No such file or directory]\n" +
		"nul-cmd=1[command contains a NUL byte]\n" +
		"nul-arg=1[argument contains a NUL byte]\n"
	if string(out) != want {
		t.Fatalf("stdout mismatch:\nwant: %q\ngot:  %q", want, string(out))
	}
}
