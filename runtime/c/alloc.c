// The allocation ABI seed (roadmap M4 names the allocator): the symbols
// and signatures the compiler's heap-allocating forms target from M5 on.
// The M4 generated code calls neither (its accepted forms allocate
// nothing); the linker drops unreferenced definitions. Over malloc for
// now — the precise-GC design replaces the body, not the ABI
// (ADR-0002's runtime scope).
#include <stdlib.h>

void *__we_alloc(long long n) {
    return malloc((size_t)n);
}

void __we_free(void *p) {
    free(p);
}
