package weruntime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// fsHarnessMain walks the fs family's seven entries through both faces:
// every Ok half (the whole-read, the unit writes, the ordered list) and
// every Err half (EEXIST, ENOENT, ENOTEMPTY, and the NUL reject that
// fires before any syscall). The list face prints the boxed entries back
// through the carrier's own accessors, so the T11-2 box shape — pointer
// at 16, length at 24 — is pinned here from the C side too. Data bytes
// may hold a NUL (the pair, not the pointer, is the String), and the
// read-nul-data line roundtrips one while nul-read rejects the same byte
// in a path.
const fsHarnessMain = `#include <stdint.h>
#include <stdio.h>

void __we_fs_read_file(const char *p, long long n, long long out[3]);
void __we_fs_write_file(const char *p, long long n, const char *d, long long dn, long long out[3]);
void __we_fs_append_file(const char *p, long long n, const char *d, long long dn, long long out[3]);
void __we_fs_remove_file(const char *p, long long n, long long out[3]);
void __we_fs_make_dir(const char *p, long long n, long long out[3]);
void __we_fs_remove_dir(const char *p, long long n, long long out[3]);
void __we_fs_list_dir(const char *p, long long n, long long out[3]);
long long __we_list_len(void *l);
long long __we_list_get(void *l, long long i);

// pout prints one non-list answer: the tag, then the payload between
// brackets — an Ok String's bytes or an Err's message. A unit Ok carries
// two zero words and prints the tag alone.
static void pout(const char *what, long long out[3]) {
    printf("%s=%lld", what, out[0]);
    if (out[1] != 0 || out[2] != 0) {
        printf("[");
        fwrite((const char *)(uintptr_t)out[1], 1, (size_t)out[2], stdout);
        printf("]");
    }
    printf("\n");
}

// plist prints the list answer by walking the returned handle: the
// element words are boxes, and the box's two payload words are the
// entry name's pair.
static void plist(const char *what, long long out[3]) {
    printf("%s=%lld[", what, out[0]);
    if (out[0] == 0) {
        void *l = (void *)(uintptr_t)out[1];
        long long n = __we_list_len(l);
        for (long long i = 0; i < n; i++) {
            void *box = (void *)(uintptr_t)__we_list_get(l, i);
            const char *p = *(const char **)((char *)box + 16);
            long long len = *(long long *)((char *)box + 24);
            if (i > 0) {
                printf(",");
            }
            fwrite(p, 1, (size_t)len, stdout);
        }
    }
    printf("]\n");
}

int main(void) {
    long long out[3];
    __we_fs_make_dir("rt", 2, out);
    pout("mkdir", out);
    __we_fs_make_dir("rt", 2, out);
    pout("mkdir-again", out);
    __we_fs_write_file("rt/a.txt", 8, "alpha-", 6, out);
    pout("write", out);
    __we_fs_append_file("rt/a.txt", 8, "beta", 4, out);
    pout("append", out);
    __we_fs_read_file("rt/a.txt", 8, out);
    pout("read", out);
    __we_fs_write_file("rt/n.txt", 8, "a\0b", 3, out);
    pout("write-nul-data", out);
    __we_fs_read_file("rt/n.txt", 8, out);
    pout("read-nul-data", out);
    __we_fs_write_file("rt/e.txt", 8, "x", 0, out);
    pout("write-empty", out);
    __we_fs_read_file("rt/e.txt", 8, out);
    pout("read-empty", out);
    __we_fs_write_file("rt/z.txt", 8, "z", 1, out);
    pout("write-z", out);
    __we_fs_write_file("rt/M.txt", 8, "m", 1, out);
    pout("write-m", out);
    __we_fs_make_dir("rt/sub", 6, out);
    pout("mkdir-sub", out);
    __we_fs_list_dir("rt", 2, out);
    plist("list", out);
    __we_fs_read_file("no-such-file", 12, out);
    pout("read-missing", out);
    __we_fs_read_file("a\0b", 3, out);
    pout("nul-read", out);
    __we_fs_make_dir("a\0b", 3, out);
    pout("nul-mkdir", out);
    __we_fs_remove_file("rt/a.txt", 8, out);
    pout("rm", out);
    __we_fs_remove_dir("rt", 2, out);
    pout("rmdir-nonempty", out);
    __we_fs_remove_file("rt/z.txt", 8, out);
    pout("rm-z", out);
    __we_fs_remove_file("rt/M.txt", 8, out);
    pout("rm-m", out);
    __we_fs_remove_file("rt/n.txt", 8, out);
    pout("rm-n", out);
    __we_fs_remove_file("rt/e.txt", 8, out);
    pout("rm-e", out);
    __we_fs_remove_dir("rt/sub", 6, out);
    pout("rmdir-sub", out);
    __we_fs_remove_dir("rt", 2, out);
    pout("rmdir", out);
    __we_fs_remove_dir("rt", 2, out);
    pout("rmdir-again", out);
    __we_fs_list_dir("rt", 2, out);
    pout("list-missing", out);
    return 0;
}
`

// TestFsHarness compiles the fs family with its link set (the collector,
// the scheduler test.c owns the clock face of, and the list carrier) and
// runs it with the harness's working directory inside the temp dir — the
// entries' relative paths must land there, not in the source tree.
func TestFsHarness(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"gc.c": GCSource, "sched.h": SchedHeader, "sched.c": SchedSource,
		"test.c": TestSource, "str.h": StrHeader, "str.c": StrSource,
		"list.h": ListHeader, "list.c": ListSource,
		"fs.c": FsSource, "main.c": fsHarnessMain,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	args := []string{"-o", filepath.Join(dir, "harness")}
	for _, in := range []string{"gc.c", "sched.c", "test.c", "str.c", "list.c", "fs.c", "main.c"} {
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
	want := "mkdir=0\n" +
		"mkdir-again=1[mkdir rt: File exists]\n" +
		"write=0\n" +
		"append=0\n" +
		"read=0[alpha-beta]\n" +
		"write-nul-data=0\n" +
		"read-nul-data=0[a\x00b]\n" +
		"write-empty=0\n" +
		"read-empty=0[]\n" +
		"write-z=0\n" +
		"write-m=0\n" +
		"mkdir-sub=0\n" +
		"list=0[M.txt,a.txt,e.txt,n.txt,sub,z.txt]\n" +
		"read-missing=1[read no-such-file: No such file or directory]\n" +
		"nul-read=1[path contains a NUL byte]\n" +
		"nul-mkdir=1[path contains a NUL byte]\n" +
		"rm=0\n" +
		"rmdir-nonempty=1[rmdir rt: Directory not empty]\n" +
		"rm-z=0\n" +
		"rm-m=0\n" +
		"rm-n=0\n" +
		"rm-e=0\n" +
		"rmdir-sub=0\n" +
		"rmdir=0\n" +
		"rmdir-again=1[rmdir rt: No such file or directory]\n" +
		"list-missing=1[list rt: No such file or directory]\n"
	if string(out) != want {
		t.Fatalf("stdout mismatch:\nwant: %q\ngot:  %q", want, string(out))
	}
}
