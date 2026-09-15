// The We runtime's fs family (B2a design D3/D7): the seven whole-file
// entries std.fs rides. Each takes the M12 cross-boundary shape — a String
// argument arrives as its (pointer, length) pair, never one pointer,
// because a We String may hold a NUL byte — and answers through the D7-2
// out-parameter form: the caller carves three words and the entry writes
// the fused Result's tag (0 = Ok, 1 = FsFailed) with its payload pair —
// a String's (pointer, length) in the read case, nothing in the unit
// cases, the list handle in the list case, the message's (pointer,
// length) in every Err case.
//
// The message form is the family's contract with the entry report line:
// "{op} {path}: {strerror}" — the op names the entry, the path is the
// caller's own bytes, strerror renders errno. A NUL inside the path is
// rejected before any syscall: C would silently truncate at it, and a
// truncated path is a different file, not an error. The face is ruling
// 3's black box — no handles, no streams, no permission bits: makeDir is
// one single-level mkdir at 0755 (umask still applies), removeDir one
// rmdir of an empty directory, listDir one directory's entry names in
// strcmp ascending order (not paths — the caller joins).
//
// Buffers this family carves follow str.c's posture: malloc'd, never
// reclaimed. The Ok String, the message, the path copies, and the entry
// names boxed into the list all outlive the call by design; reclamation
// belongs with the allocator work ADR-0003 gates.
#include "list.h"

#include <dirent.h>
#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

void *__we_alloc(long long n);   // gc.c: the frozen M4 allocation ABI (ADR-0002)
void __we_root_push(void *p);    // gc.c: the shadow root stack
void __we_root_pop(void);        // gc.c

// path_ok copies the caller's String bytes into a NUL-terminated buffer
// and answers NULL when the bytes hold a NUL — the reject-not-truncate
// rule. The copy is the syscall's operand; the original pair never
// promises a terminator.
static char *path_ok(const char *p, long long n) {
    if (p == NULL || n < 0) {
        return NULL;
    }
    if (memchr(p, 0, (size_t)n) != NULL) {
        return NULL;
    }
    char *buf = malloc((size_t)n + 1);
    if (!buf) {
        abort(); // the allocator's failure is fatal, the posture gc.c takes
    }
    memcpy(buf, p, (size_t)n);
    buf[n] = 0;
    return buf;
}

// fail writes the Err half: a fresh message buffer holding
// "{op} {path}: {strerror}". The message is a We String payload — the
// report line renders it verbatim — so it takes the never-reclaimed
// posture like every other String buffer.
static void fail(long long out[3], const char *op, const char *path) {
    const char *why = strerror(errno);
    size_t need = (size_t)snprintf(NULL, 0, "%s %s: %s", op, path, why);
    char *msg = malloc(need + 1);
    if (!msg) {
        abort();
    }
    snprintf(msg, need + 1, "%s %s: %s", op, path, why);
    out[0] = 1;
    out[1] = (long long)(uintptr_t)msg;
    out[2] = (long long)need;
}

// nul_fail is the NUL reject's report: no errno exists, so the message
// is the fixed sentence and op/path stay out of it.
static void nul_fail(long long out[3]) {
    const char *msg = "path contains a NUL byte";
    out[0] = 1;
    out[1] = (long long)(uintptr_t)msg;
    out[2] = (long long)strlen(msg);
}

// ok_unit and ok_str write the Ok halves. The unit's payload words are
// zeroed so the three-word form never carries stale stack bytes.
static void ok_unit(long long out[3]) {
    out[0] = 0;
    out[1] = 0;
    out[2] = 0;
}

static void ok_str(long long out[3], const char *p, long long n) {
    out[0] = 0;
    out[1] = (long long)(uintptr_t)p;
    out[2] = n;
}

// read_all is the whole-read: a growing buffer filled to EOF. A size
// known up front (fseek/ftell) breaks on the files that are not regular
// yet still openable, so the loop is the honest shape — and its error
// exit is where a directory opened for reading finally surfaces
// (EISDIR arrives at the first fread, not at fopen).
static char *read_all(FILE *f, long long *n_out) {
    size_t cap = 4096, len = 0;
    char *buf = malloc(cap);
    if (!buf) {
        abort();
    }
    for (;;) {
        if (len == cap) {
            cap *= 2;
            char *nb = realloc(buf, cap);
            if (!nb) {
                abort();
            }
            buf = nb;
        }
        size_t got = fread(buf + len, 1, cap - len, f);
        len += got;
        if (got == 0) {
            if (ferror(f)) {
                free(buf);
                return NULL;
            }
            break; // EOF
        }
    }
    *n_out = (long long)len;
    return buf;
}

// write_all is writeFile/appendFile's shared body: open, one fwrite,
// close — the error checks at each stop set the entry's errno.
static int write_all(const char *path, const char *data, long long n, const char *mode) {
    FILE *f = fopen(path, mode);
    if (!f) {
        return -1;
    }
    if (n > 0 && fwrite(data, 1, (size_t)n, f) != (size_t)n) {
        fclose(f);
        return -1;
    }
    if (fclose(f) != 0) {
        return -1;
    }
    return 0;
}

void __we_fs_read_file(const char *p, long long n, long long out[3]) {
    char *path = path_ok(p, n);
    if (!path) {
        nul_fail(out);
        return;
    }
    FILE *f = fopen(path, "rb");
    if (!f) {
        fail(out, "read", path);
        return;
    }
    long long len = 0;
    char *buf = read_all(f, &len);
    if (!buf) {
        fail(out, "read", path);
        return;
    }
    fclose(f);
    ok_str(out, buf, len);
}

void __we_fs_write_file(const char *p, long long n, const char *d, long long dn, long long out[3]) {
    char *path = path_ok(p, n);
    if (!path) {
        nul_fail(out);
        return;
    }
    if (write_all(path, d, dn, "wb") != 0) {
        fail(out, "write", path);
        return;
    }
    ok_unit(out);
}

void __we_fs_append_file(const char *p, long long n, const char *d, long long dn, long long out[3]) {
    char *path = path_ok(p, n);
    if (!path) {
        nul_fail(out);
        return;
    }
    if (write_all(path, d, dn, "ab") != 0) {
        fail(out, "append", path);
        return;
    }
    ok_unit(out);
}

void __we_fs_remove_file(const char *p, long long n, long long out[3]) {
    char *path = path_ok(p, n);
    if (!path) {
        nul_fail(out);
        return;
    }
    if (unlink(path) != 0) {
        fail(out, "remove", path);
        return;
    }
    ok_unit(out);
}

void __we_fs_make_dir(const char *p, long long n, long long out[3]) {
    char *path = path_ok(p, n);
    if (!path) {
        nul_fail(out);
        return;
    }
    if (mkdir(path, 0755) != 0) {
        fail(out, "mkdir", path);
        return;
    }
    ok_unit(out);
}

void __we_fs_remove_dir(const char *p, long long n, long long out[3]) {
    char *path = path_ok(p, n);
    if (!path) {
        nul_fail(out);
        return;
    }
    if (rmdir(path) != 0) {
        fail(out, "rmdir", path);
        return;
    }
    ok_unit(out);
}

// box_str carves the T11-2 String element box: a descriptor-less 32-byte
// gc object — map word null (no word inside is a gc reference), pointer
// at 16, length at 24. The same shape the emitter's element emission
// carves; the map word is already null because the allocator zeroes.
static void *box_str(const char *p, long long n) {
    void *box = __we_alloc(32);
    *(const char **)((char *)box + 16) = p;
    *(long long *)((char *)box + 24) = n;
    return box;
}

void __we_fs_list_dir(const char *p, long long n, long long out[3]) {
    char *path = path_ok(p, n);
    if (!path) {
        nul_fail(out);
        return;
    }
    DIR *d = opendir(path);
    if (!d) {
        fail(out, "list", path);
        return;
    }
    // First pass: the plain-C collection (malloc'd names, not boxes —
    // nothing gc-related happens before the count is known, so the list
    // can be carved at exactly that size and no push ever grows).
    char **names = NULL;
    size_t count = 0, cap = 0;
    struct dirent *de;
    while ((de = readdir(d)) != NULL) {
        if (strcmp(de->d_name, ".") == 0 || strcmp(de->d_name, "..") == 0) {
            continue;
        }
        if (count == cap) {
            cap = cap ? cap * 2 : 16;
            char **ng = realloc(names, cap * sizeof *ng);
            if (!ng) {
                abort();
            }
            names = ng;
        }
        char *cp = malloc(strlen(de->d_name) + 1);
        if (!cp) {
            abort();
        }
        strcpy(cp, de->d_name);
        names[count++] = cp;
    }
    closedir(d);
    // strcmp ascending: the family's determinism promise (P1) — one
    // directory, one order, whatever the filesystem handed back.
    for (size_t i = 1; i < count; i++) {
        for (size_t j = i; j > 0 && strcmp(names[j - 1], names[j]) > 0; j--) {
            char *t = names[j - 1];
            names[j - 1] = names[j];
            names[j] = t;
        }
    }
    // Second pass: the list. Carved at the exact count, rooted once for
    // the whole loop — every box becomes reachable through it before the
    // next box's allocation can fire a collection, and no push reaches
    // the growth branch, so the identity never moves. The pop is safe:
    // the caller re-roots the handle at the call site before any further
    // allocation runs (D2's discipline, mirrored from the emitter's).
    void *l = __we_list_new((long long)count, 1);
    __we_root_push(l);
    for (size_t i = 0; i < count; i++) {
        long long nlen = (long long)strlen(names[i]);
        void *box = box_str(names[i], nlen);
        __we_list_push(l, (long long)(uintptr_t)box);
    }
    __we_root_pop();
    free(names); // the pointer array only; the name bytes are the boxes' own
    out[0] = 0;
    out[1] = (long long)(uintptr_t)l;
    out[2] = 0;
}
