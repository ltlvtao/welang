// The We runtime's C entry (chapter 15 R6's entry semantics; ADR-0002's
// pinned-clang runtime sources). The compiler emits __we_main; the
// scheduler owns the process from here: task 0 wraps __we_main, Ok
// returns map to exit 0 (the IR returns 0), and the Err path never
// returns — the generated code calls __we_fail with the report line,
// which is written verbatim to stderr before exiting 1.
#include <stdio.h>
#include <stdlib.h>

int __we_main(void);
void __we_sched_boot(int (*main_fn)(void));

int main(void) {
    __we_sched_boot(__we_main);
    return 0; // __we_sched_boot enters the scheduler and does not return
}

void __we_fail(const char *line, long long len) {
    if (len > 0) {
        fwrite(line, 1, (size_t)len, stderr);
    }
    exit(1);
}
