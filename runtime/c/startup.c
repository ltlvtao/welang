// The We runtime's C entry (chapter 15 R6's entry semantics; ADR-0002's
// pinned-clang runtime sources). The compiler emits __we_main; the
// scheduler owns the process from here: task 0 wraps __we_main, Ok
// returns map to exit 0 (the IR returns 0), and the Err path never
// returns — the generated code calls __we_fail with the report line,
// which is written verbatim to stderr before exiting 1. A test binary
// reads one flag of its own: --json switches the report face to chapter
// 21 R7's JSON Lines (the CLI relays it through argv, design D6).
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int __we_main(void);
void __we_sched_boot(int (*main_fn)(void));
void __we_test_set_json(int on);

int main(int argc, char **argv) {
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--json") == 0) {
            __we_test_set_json(1);
        }
    }
    __we_sched_boot(__we_main);
    return 0; // __we_sched_boot enters the scheduler and does not return
}

void __we_fail(const char *line, long long len) {
    if (len > 0) {
        fwrite(line, 1, (size_t)len, stderr);
    }
    exit(1);
}
