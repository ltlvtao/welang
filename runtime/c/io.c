// The We runtime's io pair (chapter 21's std.io core surface; design D7).
// Each call writes immediately — flush timing is observable, so the
// stream is flushed on every call (no batching exists before M9's
// concurrency work). println writes the bytes then one newline; an empty
// payload still carries the newline. The i64 pair renders decimal (M9b:
// scalar arguments reach println now that bodies compute — Int64/Bool
// ride the i64 domain, so one integer renderer covers both).
//
// An explored process discards the bytes (M10c design D10): a hundred
// re-runs' interleaved output is neither a readable report nor a
// deterministic observable face — the aggregate line replaces it. The
// calls themselves still execute (the io effect runs; only the written
// bytes drop), and a normal run writes as always.
#include <stdio.h>

extern int __we_explore_on; // test.c owns the argv face

void __we_println(const char *s, long long n) {
    if (__we_explore_on) {
        return;
    }
    if (n > 0) {
        fwrite(s, 1, (size_t)n, stdout);
    }
    fputc('\n', stdout);
    fflush(stdout);
}

void __we_print(const char *s, long long n) {
    if (__we_explore_on) {
        return;
    }
    if (n > 0) {
        fwrite(s, 1, (size_t)n, stdout);
    }
    fflush(stdout);
}

void __we_println_i64(long long v) {
    if (__we_explore_on) {
        return;
    }
    printf("%lld\n", v);
    fflush(stdout);
}

void __we_print_i64(long long v) {
    if (__we_explore_on) {
        return;
    }
    printf("%lld", v);
    fflush(stdout);
}
