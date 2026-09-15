// The We runtime's process entry (B2a design D4/D7): std.process rides
// this one spawn. The command crosses as its (pointer, length) pair and
// the argument list as its carrier handle — the entry reads the carrier
// through its own accessors, each element box's String pair at 16/24 —
// and the answer comes back through the D7-2 out-parameter form: the
// fused Result's tag (0 = Ok, 1 = ProcessFailed) with the Ok half's
// ProcessResult record handle or the Err half's message pair.
//
// The capture is ruling 3's whole-run shape: fork, execvp (PATH search
// when the command carries no slash), both output pipes drained by one
// poll(2) loop to EOF — a sequential read of one pipe deadlocks a child
// that fills it while writing the other — then waitpid. A normal exit
// reports its code; a signalled death reports 128 + sig, the shell's
// convention. The child's stdin is the parent's own, untouched: feeding
// a subprocess is not this face's business. An exec failure reaches the
// parent through a close-on-exec pipe: the child writes the errno there
// and _exits, while a successful exec closes the pipe by construction,
// so the parent's read distinguishes spawn failure from a program that
// merely exited badly.
//
// A NUL inside the command or an argument is rejected before any spawn:
// execvp and the argv entries are C strings, and a truncated command or
// argument is a different program, not an error — the same
// reject-not-truncate rule the fs family takes for paths.
//
// The Ok record follows the T11-2 judgment: its field set is Int64 and
// String x2, and a String's bytes live in the malloc domain, so no word
// inside is a gc reference and the map word stays null — the allocator
// zeroes it, and nothing overwrites it. No allocation follows the carve
// (the capture buffers are malloc'd before it), so the entry needs no
// root of its own; the caller re-roots the handle at the call site
// before any further allocation can run (D2's discipline). Buffers this
// entry carves follow str.c's posture: malloc'd, never reclaimed.
#include "list.h"

#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

void *__we_alloc(long long n); // gc.c: the frozen M4 allocation ABI (ADR-0002)
long long __we_list_len(void *l);   // list.c: the carrier's element count
long long __we_list_get(void *l, long long i); // list.c: one element's word

// pfail writes the Err half with a fixed sentence — the NUL rejects,
// where no errno exists to render.
static void pfail(long long out[3], const char *msg) {
    out[0] = 1;
    out[1] = (long long)(uintptr_t)msg;
    out[2] = (long long)strlen(msg);
}

// fail writes the Err half from errno: "exec {cmd}: {strerror}" is the
// message form the entry's report line renders (the command, not an op
// word — spawn is this family's one operation).
static void fail(long long out[3], const char *op, const char *cmd) {
    const char *why = strerror(errno);
    size_t need = (size_t)snprintf(NULL, 0, "%s %s: %s", op, cmd, why);
    char *msg = malloc(need + 1);
    if (!msg) {
        abort(); // the allocator's failure is fatal, the posture gc.c takes
    }
    snprintf(msg, need + 1, "%s %s: %s", op, cmd, why);
    out[0] = 1;
    out[1] = (long long)(uintptr_t)msg;
    out[2] = (long long)need;
}

// nul_copy copies one String's bytes into a NUL-terminated buffer and
// answers NULL when the bytes hold a NUL — the reject-not-truncate rule.
static char *nul_copy(const char *p, long long n) {
    if (p == NULL || n < 0) {
        return NULL;
    }
    if (memchr(p, 0, (size_t)n) != NULL) {
        return NULL;
    }
    char *buf = malloc((size_t)n + 1);
    if (!buf) {
        abort();
    }
    memcpy(buf, p, (size_t)n);
    buf[n] = 0;
    return buf;
}

// sbuf is one pipe's growing capture: append-only, malloc'd, never
// reclaimed — the bytes become the record's stdout/stderr pair, which
// outlives the call by design.
struct sbuf {
    char *p;
    size_t len;
};

static void sbuf_add(struct sbuf *b, const char *chunk, size_t n) {
    size_t want = b->len + n;
    size_t cap = b->len ? b->len : 4096;
    while (cap < want) {
        cap *= 2;
    }
    char *nb = realloc(b->p, cap);
    if (!nb) {
        abort();
    }
    b->p = nb;
    memcpy(b->p + b->len, chunk, n);
    b->len = want;
}

void __we_proc_run(const char *cmd, long long cmdn, void *args, long long out[3]) {
    char *argv0 = nul_copy(cmd, cmdn);
    if (!argv0) {
        pfail(out, "command contains a NUL byte");
        return;
    }
    long long argc = 1 + __we_list_len(args);
    char **argv = malloc(sizeof *argv * (size_t)(argc + 1));
    if (!argv) {
        abort();
    }
    argv[0] = argv0;
    for (long long i = 0; i < argc - 1; i++) {
        void *box = (void *)(uintptr_t)__we_list_get(args, i);
        const char *p = *(const char **)((char *)box + 16);
        long long n = *(long long *)((char *)box + 24);
        char *cp = nul_copy(p, n);
        if (!cp) {
            pfail(out, "argument contains a NUL byte");
            return;
        }
        argv[i + 1] = cp;
    }
    argv[argc] = NULL;

    int outp[2], errp[2], fxp[2];
    if (pipe(outp) != 0) {
        fail(out, "pipe", argv0);
        return;
    }
    if (pipe(errp) != 0) {
        fail(out, "pipe", argv0);
        return;
    }
    if (pipe(fxp) != 0) {
        fail(out, "pipe", argv0);
        return;
    }
    // The exec-fail pipe's write end closes itself on a successful exec,
    // which is the whole trick: reading zero means the program started.
    fcntl(fxp[1], F_SETFD, fcntl(fxp[1], F_GETFD) | FD_CLOEXEC);

    pid_t pid = fork();
    if (pid < 0) {
        fail(out, "fork", argv0);
        return;
    }
    if (pid == 0) {
        // The child: capture ends over the standard descriptors, every
        // other pipe fd closed, stdin untouched (the documented face),
        // then execvp. A return is a spawn failure — the errno crosses
        // the close-on-exec pipe and the exit code is arbitrary.
        close(fxp[0]);
        dup2(outp[1], 1);
        dup2(errp[1], 2);
        close(outp[0]);
        close(outp[1]);
        close(errp[0]);
        close(errp[1]);
        execvp(argv[0], argv);
        int e = errno;
        ssize_t w = write(fxp[1], &e, sizeof e);
        (void)w;
        _exit(127);
    }
    close(outp[1]);
    close(errp[1]);
    close(fxp[1]);

    // Both pipes drained by one poll loop to EOF — the deadlock face of
    // a sequential read is the reason the family polls: a child that
    // fills one pipe while the parent blocks on the other never finishes.
    struct sbuf ob = {0}, eb = {0};
    int openfds = 2;
    int done[2] = {0, 0};
    while (openfds > 0) {
        struct pollfd pf[2] = {
            {outp[0], done[0] ? 0 : POLLIN, 0},
            {errp[0], done[1] ? 0 : POLLIN, 0},
        };
        int n = poll(pf, 2, -1);
        if (n < 0) {
            if (errno == EINTR) {
                continue;
            }
            fail(out, "poll", argv0);
            return;
        }
        for (int i = 0; i < 2; i++) {
            if (done[i] || !(pf[i].revents & (POLLIN | POLLHUP | POLLERR))) {
                continue;
            }
            char chunk[4096];
            ssize_t got = read(pf[i].fd, chunk, sizeof chunk);
            if (got > 0) {
                if (i == 0) {
                    sbuf_add(&ob, chunk, (size_t)got);
                } else {
                    sbuf_add(&eb, chunk, (size_t)got);
                }
                continue;
            }
            if (got == 0) {
                close(pf[i].fd);
                done[i] = 1;
                openfds--;
                continue;
            }
            if (errno != EINTR) {
                fail(out, "read", argv0);
                return;
            }
        }
    }

    int e = 0;
    ssize_t r = read(fxp[0], &e, sizeof e);
    close(fxp[0]);
    if (r == (ssize_t)sizeof e) {
        // execvp failed in the child; the errno crossed the pipe.
        errno = e;
        fail(out, "exec", argv0);
        return;
    }

    int status = 0;
    if (waitpid(pid, &status, 0) < 0) {
        fail(out, "wait", argv0);
        return;
    }
    long long code;
    if (WIFEXITED(status)) {
        code = WEXITSTATUS(status);
    } else if (WIFSIGNALED(status)) {
        code = 128 + WTERMSIG(status); // the shell's signalled-exit encoding
    } else {
        code = -1;
    }

    // The answer record: frozen header (map null — no traced word), the
    // exit code at 16, the stdout pair at 24/32, the stderr pair at
    // 40/48 — the layout a We-side construction of this record takes.
    // No allocation follows the carve, so no root is owed here.
    static const char empty[] = "";
    void *rec = __we_alloc(56);
    *(long long *)((char *)rec + 16) = code;
    *(const char **)((char *)rec + 24) = ob.p ? ob.p : empty;
    *(long long *)((char *)rec + 32) = (long long)ob.len;
    *(const char **)((char *)rec + 40) = eb.p ? eb.p : empty;
    *(long long *)((char *)rec + 48) = (long long)eb.len;
    out[0] = 0;
    out[1] = (long long)(uintptr_t)rec;
    out[2] = 0;
}
