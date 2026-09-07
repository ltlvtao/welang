// The We runtime's io pair (chapter 21's std.io core surface; design D7).
// Each call writes immediately — flush timing is observable, so the
// stream is flushed on every call (no batching exists before M9's
// concurrency work). println writes the bytes then one newline; an empty
// payload still carries the newline. The i64 pair renders decimal (M9b:
// scalar arguments reach println now that bodies compute — Int64/Bool
// ride the i64 domain, so one integer renderer covers both).
#include <stdio.h>

void __we_println(const char *s, long long n) {
    if (n > 0) {
        fwrite(s, 1, (size_t)n, stdout);
    }
    fputc('\n', stdout);
    fflush(stdout);
}

void __we_print(const char *s, long long n) {
    if (n > 0) {
        fwrite(s, 1, (size_t)n, stdout);
    }
    fflush(stdout);
}

void __we_println_i64(long long v) {
    printf("%lld\n", v);
    fflush(stdout);
}

void __we_print_i64(long long v) {
    printf("%lld", v);
    fflush(stdout);
}
