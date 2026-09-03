# Delta: lexical (range token)

## MODIFIED Requirements

### Requirement: Operators and punctuation

The operator and punctuation inventory is closed and enumerated. Operators: `+ - * / %` `== != < <= > >=` `&& || !` `& | ^ << >> ~` `=` `..`. Punctuation: `( ) { } [ ] , ; : . -> => _ ?` and the token `#[` with its closing `]` (attribute unit, one lexical structure). Operator overloading does not exist (target profile), and custom operators cannot be defined: characters outside this inventory (for example `$`, `@`, `` ` ``) never form tokens. `<<` and `>>` are single tokens under maximal munch; the interaction with nested generic closers (which require separation) is specified by the generics chapter. `..` is a single token under maximal munch, and a numeric literal's maximal munch never absorbs a dot that begins a `..` token: `0..9` lexes as `0`, `..`, `9`.

#### Scenario: Unknown operator sequence

- **WHEN** a character sequence matches no token in this chapter's inventory (for example `$`, `@`, `#` not followed by `[`)
- **THEN** the lexer reports `E0001: invalid character` at the first unmatched character; the compiler never guesses an operator from context

#### Scenario: Multi-character tokens are unambiguous under maximal munch

- **WHEN** input contains `<<`, `<=`, `->`, or `=>`
- **THEN** each is a single token; splitting or recombining them (`< <`, `- >`) is a lexical error or a different token stream, as maximal munch alone decides

#### Scenario: The range token splits from adjacent numerals

- **WHEN** input contains `0..9` or `1.5..2.5`
- **THEN** `..` is a single token; the sequences lex as `0` `..` `9` and `1.5` `..` `2.5` — no float `0.` or `.9` is formed, because a float requires a digit on each side of the dot
