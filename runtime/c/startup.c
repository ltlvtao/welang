// The We runtime's C entry (chapter 15 R6's entry semantics; ADR-0002's
// pinned-clang runtime sources). The compiler emits __we_main; this side
// owns the process: Ok returns map to exit 0 (the IR returns 0), and the
// Err path never returns — the generated code calls __we_fail with the
// report line, which is written verbatim to stderr before exiting 1.
#include <stdio.h>
#include <stdlib.h>

int __we_main(void);
void __we_gc_boot(void);

int main(void) {
    __we_gc_boot();
    return __we_main();
}

void __we_fail(const char *line, long long len) {
    if (len > 0) {
        fwrite(line, 1, (size_t)len, stderr);
    }
    exit(1);
}
