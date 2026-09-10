// The String runtime's shared face (design D3): the (pointer, length) pair
// every entry consumes or returns, and the family's declarations. The pair
// is a two-eightbyte struct — both eightbytes INTEGER class, so clang
// returns it in two registers, the ABI the emitter's IR spells
// { ptr, i64 }. A String value is an immutable UTF-8 byte sequence whose
// two words ride every operand slot; the buffer is allocated with malloc
// and never reclaimed (design D3 as corrected at T4: the gc domain holds
// gc objects, and a raw byte buffer is not one — a non-gc pointer on the
// shadow root stack would be read as a block header by the collector).
#ifndef WE_STR_H
#define WE_STR_H

struct we_str {
    const char *p;
    long long n;
};

// concat returns a fresh buffer holding a's bytes then b's.
struct we_str __we_str_concat(const char *a, long long an, const char *b, long long bn);

// eq compares by bytes and length: 1 when equal, 0 otherwise.
long long __we_str_eq(const char *a, long long an, const char *b, long long bn);

// The two access layers of chapter 17: runecount and charat walk code
// points, byteslice spans bytes. An index or range outside the value
// terminates the calling task through chapter 14's family.
long long __we_str_runecount(const char *p, long long n);
long long __we_str_charat(const char *p, long long n, long long i);
struct we_str __we_str_byteslice(const char *p, long long n, long long lo, long long hi);

// The value-to-string family interpolation rides (design D3): each returns
// a fresh buffer holding the value's rendered form.
struct we_str __we_str_of_i64(long long v);
struct we_str __we_str_of_u64(unsigned long long v);
struct we_str __we_str_of_f64(double v);
struct we_str __we_str_of_bool(long long v);
struct we_str __we_str_of_rune(long long v);

#endif
