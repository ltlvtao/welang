// The We runtime's String family (design D3): concatenation, equality, the
// two access layers chapter 17 ratifies (bytes through byteLength and
// byteSlice, code points through runeCount and charAt), the value-to-string
// converters interpolation rides, and the out-of-range panic faces.
//
// Storage (design D3 as corrected at T4): a String's bytes live in a
// malloc'd buffer, outside the gc domain — a raw byte buffer is not a gc
// object, carries no header and no descriptor, and a non-gc pointer pushed
// on the shadow root stack would be read as a block header by the
// collector. Buffers are never reclaimed: the family's allocation is
// bounded by the program's own string-building volume, and its reclamation
// belongs with the allocator work ADR-0003 gates (B2). The two provenances
// are therefore immortal — the private constants the emitter interns and
// this family's malloc'd buffers — which is why byteslice may return an
// interior pointer (a slice shares its source's bytes; a String is
// immutable, so nothing can observe the sharing).
//
// The panic faces are chapter 17's: "a byte operation with an out-of-range
// range is a panic, the chapter 14 family" and "an out-of-range index a
// panic likewise". Each message names the operation, the family's naming
// discipline (chapter 14).
#include "str.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

void __we_task_fail(const char *msg); // sched.c: the calling task's failure report

// utf8_step decodes one code point at byte offset i, reporting its width.
// An ill-formed sequence — a stray continuation byte, a truncated tail, an
// overlong form, a surrogate, or a value past U+10FFFF — reads as U+FFFD
// and advances one byte, so runeCount and charAt agree with each other and
// stay total on any byte string (chapter 17 fixes no reading for one;
// this is the reference build's, disclosed).
static long long utf8_step(const unsigned char *s, long long n, long long i, long long *width) {
    static const long long min_cp[4] = {0, 0x80, 0x800, 0x10000};
    unsigned char b = s[i];
    int want;
    long long cp;
    if (b < 0x80) {
        *width = 1;
        return b;
    }
    if ((b & 0xe0) == 0xc0) {
        want = 1, cp = b & 0x1f;
    } else if ((b & 0xf0) == 0xe0) {
        want = 2, cp = b & 0x0f;
    } else if ((b & 0xf8) == 0xf0) {
        want = 3, cp = b & 0x07;
    } else {
        *width = 1;
        return 0xfffd;
    }
    if (i + want >= n) {
        *width = 1;
        return 0xfffd;
    }
    for (int k = 1; k <= want; k++) {
        unsigned char c = s[i + k];
        if ((c & 0xc0) != 0x80) {
            *width = 1;
            return 0xfffd;
        }
        cp = (cp << 6) | (c & 0x3f);
    }
    if (cp < min_cp[want] || cp > 0x10ffff || (cp >= 0xd800 && cp <= 0xdfff)) {
        *width = 1;
        return 0xfffd;
    }
    *width = want + 1;
    return cp;
}

// str_alloc carves one buffer; the allocator's failure is fatal, the
// posture gc.c's chunk carving takes (a runtime with no fallback stops).
static char *str_alloc(long long n) {
    char *p = malloc(n ? (size_t)n : 1);
    if (!p) {
        abort();
    }
    return p;
}

struct we_str __we_str_concat(const char *a, long long an, const char *b, long long bn) {
    struct we_str r;
    r.n = an + bn;
    r.p = str_alloc(r.n);
    memcpy((char *)r.p, a, (size_t)an);
    memcpy((char *)r.p + an, b, (size_t)bn);
    return r;
}

long long __we_str_eq(const char *a, long long an, const char *b, long long bn) {
    if (an != bn) {
        return 0;
    }
    return memcmp(a, b, (size_t)an) == 0 ? 1 : 0;
}

long long __we_str_runecount(const char *p, long long n) {
    const unsigned char *s = (const unsigned char *)p;
    long long count = 0;
    for (long long off = 0; off < n;) {
        long long w;
        (void)utf8_step(s, n, off, &w);
        off += w;
        count++;
    }
    return count;
}

long long __we_str_charat(const char *p, long long n, long long i) {
    const unsigned char *s = (const unsigned char *)p;
    long long off = 0;
    for (long long idx = 0; idx < i && off < n; idx++) {
        long long w;
        (void)utf8_step(s, n, off, &w);
        off += w;
    }
    if (i < 0 || off >= n) {
        __we_task_fail("String charAt out of range");
    }
    long long w;
    return utf8_step(s, n, off, &w);
}

struct we_str __we_str_byteslice(const char *p, long long n, long long lo, long long hi) {
    struct we_str r;
    if (lo < 0 || hi < lo || hi > n) {
        __we_task_fail("String byteSlice out of range");
    }
    r.p = p + lo;
    r.n = hi - lo;
    return r;
}

// of_u64 renders magnitude*neg as decimal; the sign rides a prefix so the
// minimum value (whose magnitude is not an i64) renders in full.
static struct we_str of_u64(unsigned long long mag, int neg) {
    char buf[24];
    int i = 0;
    do {
        buf[i++] = (char)('0' + (mag % 10));
        mag /= 10;
    } while (mag);
    if (neg) {
        buf[i++] = '-';
    }
    struct we_str r;
    r.n = i;
    r.p = str_alloc(i);
    for (int k = 0; k < i; k++) {
        ((char *)r.p)[k] = buf[i - 1 - k];
    }
    return r;
}

struct we_str __we_str_of_i64(long long v) {
    unsigned long long mag = v < 0 ? 0ULL - (unsigned long long)v : (unsigned long long)v;
    return of_u64(mag, v < 0);
}

// The UInt64 view of the same magnitude: the full unsigned inventory is a
// value of the type, so its rendering is the unsigned decimal and never a
// sign.
struct we_str __we_str_of_u64(unsigned long long v) {
    return of_u64(v, 0);
}

// of_f64 renders the shortest decimal that reads back as the same double:
// the first precision whose %g rendering survives strtod, at most 17
// significant digits (the round-trip bound). Non-finite values ride %g's
// own words (inf, -inf, nan); negative zero keeps its sign.
struct we_str __we_str_of_f64(double v) {
    char buf[40];
    int prec = 1;
    for (; prec < 17; prec++) {
        snprintf(buf, sizeof buf, "%.*g", prec, v);
        if (strtod(buf, NULL) == v) {
            break;
        }
    }
    if (prec == 17) {
        snprintf(buf, sizeof buf, "%.17g", v);
    }
    size_t len = strlen(buf);
    struct we_str r;
    r.n = (long long)len;
    r.p = str_alloc(r.n);
    memcpy((char *)r.p, buf, len);
    return r;
}

struct we_str __we_str_of_bool(long long v) {
    static const char yes[] = "true";
    static const char no[] = "false";
    struct we_str r;
    r.p = v ? yes : no;
    r.n = v ? 4 : 5;
    return r;
}

// of_rune encodes one code point as UTF-8; a value outside the Rune
// inventory (U+0000..U+10FFFF, surrogates excluded) encodes as U+FFFD, so
// the conversion stays total.
struct we_str __we_str_of_rune(long long v) {
    unsigned long long cp = (unsigned long long)v;
    if (cp > 0x10ffff || (cp >= 0xd800 && cp <= 0xdfff)) {
        cp = 0xfffd;
    }
    char buf[4];
    int len;
    if (cp < 0x80) {
        buf[0] = (char)cp;
        len = 1;
    } else if (cp < 0x800) {
        buf[0] = (char)(0xc0 | (cp >> 6));
        buf[1] = (char)(0x80 | (cp & 0x3f));
        len = 2;
    } else if (cp < 0x10000) {
        buf[0] = (char)(0xe0 | (cp >> 12));
        buf[1] = (char)(0x80 | ((cp >> 6) & 0x3f));
        buf[2] = (char)(0x80 | (cp & 0x3f));
        len = 3;
    } else {
        buf[0] = (char)(0xf0 | (cp >> 18));
        buf[1] = (char)(0x80 | ((cp >> 12) & 0x3f));
        buf[2] = (char)(0x80 | ((cp >> 6) & 0x3f));
        buf[3] = (char)(0x80 | (cp & 0x3f));
        len = 4;
    }
    struct we_str r;
    r.n = len;
    r.p = str_alloc(len);
    memcpy((char *)r.p, buf, (size_t)len);
    return r;
}
