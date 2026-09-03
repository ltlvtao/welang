# lexical Specification Delta

## ADDED Requirements

### Requirement: Source files

A We source file is a UTF-8 encoded text file with the `.we` extension. A single leading U+FEFF (byte-order mark) is permitted and ignored. Line breaks are LF; CRLF is accepted and treated as one line break. Any other appearance of U+FEFF, any byte sequence that is not valid UTF-8, and a bare CR not followed by LF are lexical errors. Non-ASCII characters are permitted only inside comments and literals.

#### Scenario: Invalid encoding is rejected

- **WHEN** a source file contains a byte sequence that is not valid UTF-8, or a non-ASCII character where only a token may appear (outside comments and literals)
- **THEN** the lexer reports `E0001: invalid character` with the offset of the offending byte or character and does not produce a token stream

#### Scenario: BOM and line endings are normalized mechanically

- **WHEN** a source file begins with U+FEFF or uses CRLF line breaks
- **THEN** the BOM is ignored and each CRLF counts as exactly one line break; neither produces a diagnostic

### Requirement: Token model

Lexing is context-free: the token kind of every character sequence is determined solely by maximal-munch matching against the closed token inventory of this chapter, never by parsing context. Whitespace and comments separate tokens and are otherwise insignificant. The set of context-sensitive lexical rules in We is **empty and closed**: this specification defines zero soft keywords and zero context-dependent token interpretations, and no future chapter may introduce one.

#### Scenario: Maximal munch resolves token boundaries

- **WHEN** a character sequence admits a longer and a shorter token match at the same position (for example `1_000`, `0x1F`, `!=`)
- **THEN** the lexer always produces the longest match permitted by this chapter's inventory, and the result is independent of surrounding tokens

#### Scenario: A context-sensitive lexing proposal appears

- **WHEN** a future proposal suggests interpreting a word or symbol differently depending on syntactic position (a soft keyword or contextual token)
- **THEN** it conflicts with this Requirement's closed empty set and MUST be rejected unless it first amends this chapter through a spec-layer change

### Requirement: Identifiers

An identifier is `[A-Za-z_][A-Za-z0-9_]*`. Identifiers are case-sensitive. All characters outside this set — including all non-ASCII letters, digits that are not ASCII digits, and confusable homoglyphs — are forbidden in identifiers: Unicode identifiers do not exist in We. Non-ASCII text is freely usable in comments and literals.

#### Scenario: A non-ASCII identifier is rejected

- **WHEN** a name is written with non-ASCII characters (for example a Cyrillic or CJK letter that visually resembles an ASCII letter)
- **THEN** the lexer reports `E0001: invalid character` at the first offending character, naming homoglyph confusability as the reason identifiers are ASCII-only

#### Scenario: Case sensitivity is total

- **WHEN** two identifiers differ only in letter case (for example `userId` and `UserID`)
- **THEN** they are two distinct identifiers with no defined relationship

### Requirement: Keywords

Keywords are fully reserved everywhere: a keyword token is never an identifier, in any syntactic position. We defines **zero soft keywords** — there is no context in which a reserved word may be used as a name, and no context in which an identifier becomes a reserved word. The initial keyword set is closed and enumerated:

```
fn let var pub import as mut
if else return match for in while loop break continue defer
true false foreign
```

Adding a keyword is a spec-layer change that amends this list. The list contains only words whose syntax is or will be ratified by a chapter; feature-specific words (effects, types, concurrency, testing) join the list together with the chapter that ratifies them.

#### Scenario: A keyword is used as a name

- **WHEN** a keyword from the enumerated list appears where an identifier is required (for example a variable named `match`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word and its position; the lexer never offers it as an identifier token

#### Scenario: A later chapter needs a new keyword

- **WHEN** a content chapter ratifies syntax that introduces a reserved word not in this list
- **THEN** its spec delta MUST amend this list in the same change, and the amendment is a breaking change recorded as such

### Requirement: Numeric literals

Integer literals are written in decimal (`123`), hexadecimal (`0x` prefix, at least one hex digit), octal (`0o` prefix, at least one digit `0`–`7`), or binary (`0b` prefix, at least one digit `0`/`1`). A leading `0` on a decimal literal is forbidden (`0` itself is allowed). The underscore `_` is a digit-group separator permitted **only between two digits** — never leading, trailing, doubled, adjacent to a base prefix, or adjacent to a suffix. Integer suffixes: `i8` `i16` `i32` `i64` `u8` `u16` `u32` `u64`. Float literals contain `.` with at least one digit on each side, optionally with an exponent (`e`/`E`, optional sign, at least one digit), in decimal only; float suffixes: `f32` `f64`. A suffix immediately follows the last digit. The type denoted by an unsuffixed literal is defined by the types chapter; this chapter defines only the forms.

#### Scenario: Separator placement is mechanical

- **WHEN** a numeric literal contains an underscore that is not between two digits (for example `1__0`, `_100`, `100_`, `0x_FF`, `100_i64`)
- **THEN** the lexer reports `E0006: invalid numeric literal` at the literal, stating the separator rule

#### Scenario: Base prefixes require digits

- **WHEN** a literal begins `0x`, `0o`, or `0b` and is not followed by at least one digit of that base (for example `0x`, `0b2`)
- **THEN** the lexer reports `E0006: invalid numeric literal` naming the expected digit set

#### Scenario: Suffixes bind to the whole literal

- **WHEN** a literal carries a suffix (for example `42i8`, `3.14f32`)
- **THEN** the suffix is part of the same literal token, and an unknown suffix (for example `42int`) reports `E0006: invalid numeric literal` — suffixes are a closed set

### Requirement: String and rune literals

A string literal is `"..."` on a single line; a rune literal is `'x'` holding exactly one character or one escape. Interpolation: inside a string literal, `$` immediately followed by `{` opens an interpolation hole containing a balanced-brace expression region; `$` not followed by `{` is an ordinary character; interpolation does not exist inside rune literals. The escape set is closed and identical for both: `\n` `\t` `\r` `\0` `\\` `\"` `\'` `\u{h..}` (1–6 hex digits, value ≤ U+10FFFF, surrogate values forbidden). Any other character after `\` is an error. Multiline strings and raw (unescaped) strings do not exist in We at this chapter; adding one is a spec-layer change.

#### Scenario: Unterminated literal

- **WHEN** a string or rune literal is not closed before the end of the line (for the rune case: before the closing `'` on the same line)
- **THEN** the lexer reports `E0002: unterminated string literal` or `E0003: unterminated rune literal` at the opening quote

#### Scenario: Unknown escape

- **WHEN** a literal contains `\` followed by a character outside the closed escape set (for example `\e`, `\x41`)
- **THEN** the lexer reports `E0005: invalid escape sequence` naming the character and the permitted set

#### Scenario: Interpolation hole is lexically balanced

- **WHEN** a string contains `${` whose braces are not balanced within the literal (for example `"${f(}"`)
- **THEN** the lexer reports `E0007: unbalanced interpolation braces` at the opening `${`; a `$` not followed by `{` is never treated as interpolation

### Requirement: Operators and punctuation

The operator and punctuation inventory is closed and enumerated. Operators: `+ - * / %` `== != < <= > >=` `&& || !` `& | ^ << >> ~` `=`. Punctuation: `( ) { } [ ] , ; : . -> => _ ?` and the token `#[` with its closing `]` (attribute unit, one lexical structure). Operator overloading does not exist (target profile), and custom operators cannot be defined: characters outside this inventory (for example `$`, `@`, `` ` ``) never form tokens. `<<` and `>>` are single tokens under maximal munch; the interaction with nested generic closers (which require separation) is specified by the generics chapter.

#### Scenario: Unknown operator sequence

- **WHEN** a character sequence matches no token in this chapter's inventory (for example `$`, `@`, `#` not followed by `[`)
- **THEN** the lexer reports `E0001: invalid character` at the first unmatched character; the compiler never guesses an operator from context

#### Scenario: Multi-character tokens are unambiguous under maximal munch

- **WHEN** input contains `<<`, `<=`, `->`, or `=>`
- **THEN** each is a single token; splitting or recombining them (`< <`, `- >`) is a lexical error or a different token stream, as maximal munch alone decides

### Requirement: Comments

Three comment forms: line comments `//` to end of line; block comments `/* */` which do not nest (an inner `/*` is ordinary text; the first `*/` closes the comment); documentation comments `///` to end of line, whose attachment rules are defined by the declarations chapter. Comments may contain any Unicode text. Comments separate tokens like whitespace.

#### Scenario: Unterminated block comment

- **WHEN** a `/*` is not closed by `*/` before end of file
- **THEN** the lexer reports `E0004: unterminated block comment` at the opening `/*`

#### Scenario: Non-nesting is mechanical

- **WHEN** a block comment contains `/* ... */` inside it
- **THEN** the first `*/` closes the whole comment; the inner `/*` is never treated as opening a nested level

### Requirement: Attribute lexical unit

An attribute is the token `#[` followed by a name, optional parenthesized arguments, and the closing `]` — one lexical unit attached to the next declaration. Attribute arguments are compile-time literal values only (string, number, boolean, literal lists). The attribute namespace is a closed whitelist: an attribute name outside the whitelist is a compile error, never silently ignored. Which attributes exist is defined by the chapters that own them; this chapter defines the form, the literal-only argument rule, and the closed-whitelist obligation (chapter 0, Principle 8).

#### Scenario: Unknown attribute

- **WHEN** an attribute names a word not in the whitelist assembled from ratified chapters (for example `#[inline]` when no chapter has ratified it)
- **THEN** the compiler reports `E0008: unknown attribute` with the name and position; it is not skipped

#### Scenario: Non-literal attribute argument

- **WHEN** an attribute argument is an expression, a function call, or a reference (for example `#[timeout(5000 + n)]`)
- **THEN** the compiler reports `E0009: non-literal attribute argument` at the argument; attribute legality stays locally decidable (chapter 0, Principle 1)

### Requirement: Naming conventions

Identifier spelling is fixed by binding kind and enforced as an error, so every We codebase reads with one convention: type names (`record`/`interface`/`newtype`/sum-type/alias forms, as ratified by the types chapter) are PascalCase; variables, functions, and methods are camelCase; module names are lowercase with dot separation; constants follow the variable rule (no SCREAMING_CASE). The check is decidable from the identifier alone plus its binding kind.

#### Scenario: Convention violation is an error, not a style hint

- **WHEN** a type name is not PascalCase (for example `userRecord`), a variable/function/method name is not camelCase (for example `User_Id`), or a module name is not lowercase-dotted
- **THEN** the compiler reports `E0011: type name must be PascalCase`, `E0012: variable, function, and method names must be camelCase`, or `E0013: module names must be lowercase` respectively, at the declaration

### Requirement: Diagnostic code scheme

Diagnostic codes are `E` (error) or `W` (warning) followed by exactly four digits. Codes are globally unique across the specification; each code belongs to exactly one segment and one chapter, and segments are allocated by this table, extended only by spec-layer changes:

| Segment | Domain | Owner |
| --- | --- | --- |
| `E0001`–`E0099` | Lexical and naming | This chapter |
| `E0100`–`E0199` | Grammar and parsing | Grammar chapter |
| `E0200`+ | Later domains | Claimed by their chapters via amendment |

This chapter allocates: `E0001` invalid character, `E0002` unterminated string literal, `E0003` unterminated rune literal, `E0004` unterminated block comment, `E0005` invalid escape sequence, `E0006` invalid numeric literal, `E0007` unbalanced interpolation braces, `E0008` unknown attribute, `E0009` non-literal attribute argument, `E0011` type name convention, `E0012` value and function name convention, `E0013` module name convention. Within the lexical segment, `E0001`–`E0009` are token-level errors and `E0011`–`E0019` the naming block; `E0010` and `E0014`–`E0099` are reserved for amendments of this chapter. Warning codes follow the same scheme; none are allocated by this chapter.

#### Scenario: A later chapter claims a segment

- **WHEN** a content chapter needs new diagnostic codes
- **THEN** its spec delta MUST extend this segment table in the same change, allocating an unclaimed range and enumerating each code's meaning; renumbering existing codes is forbidden (chapter 0: diagnostics protocol is a stability commitment)

#### Scenario: Codes are unique across chapters

- **WHEN** two chapters would define the same code with different meanings
- **THEN** the spec consistency review rejects the second definition; the segment table is the single registry
