# We Language Specification — Chapter 2: Grammar

### Requirement: Parse model and ambiguity rejection

The unit of parsing is a source file's token stream as ratified by chapter 1, annotated with the source line of each token. Every token sequence MUST parse to exactly one tree or be rejected with a diagnostic. A sequence admitting two defensible parses MUST be rejected with `E0101` naming the competing interpretations (chapter 0, Principle 4). Parsing MUST NOT depend on information beyond the token stream and its line annotations.

#### Scenario: A construct has two parses

- **WHEN** a ratified production admits two different trees for one token sequence
- **THEN** the compiler rejects the sequence with `E0101:` parse ambiguity, and the message names the competing interpretations

#### Scenario: A fragment parses from local structure alone

- **WHEN** a parser decides the structure of a fragment
- **THEN** its decision uses only the fragment's tokens, their line annotations, and ratified productions — never call-site or project-wide information

### Requirement: Line-joining and semicolon inference

Statement boundaries are inferred, not written; the source carries no semicolons. Inside round brackets `( ... )` and square brackets — including attribute units `#[ ... ]` — line breaks carry no significance. Inside braces, which delimit blocks and are not brackets for this Requirement, the depth-zero rule applies. At bracket depth zero, a statement boundary is present after the last token of every source line UNLESS that token is in the continuation set: a binary operator (`+ - * / % == != < <= > >= && || & | ^ << >>`), `=`, or `.`. The continuation set is closed; extending it is a spec change. Tokens that begin statements are a fixed class: identifiers, literals, keywords, `(`, `{`, and the prefix operators `!`, `-`, `~`.

#### Scenario: Operator at end of line continues the statement

- **WHEN** at bracket depth zero a line ends with a binary operator, for example `a +` followed by `b` on the next line
- **THEN** no statement boundary is inserted; the parse continues onto the next line

#### Scenario: Operand at end of line ends the statement

- **WHEN** at bracket depth zero a line ends with an identifier, literal, or closing bracket, for example `let a = b` followed by `- c` on the next line
- **THEN** a statement boundary is inserted; `- c` begins a new statement as a unary-minus expression, and the intent `a - c` MUST be written on one line or with the operator trailing

#### Scenario: Line breaks inside brackets are insignificant

- **WHEN** a bracketed construct spans lines, for example a call written `f(a,` on one line and `b)` on the next
- **THEN** the line breaks have no effect; boundaries are inferred only at bracket depth zero

#### Scenario: Statement begins with a continuation-only token

- **WHEN** at bracket depth zero a statement begins with a token that cannot begin a statement, for example `.method()` after a complete previous line
- **THEN** the compiler rejects it with `E0102:` statement begins with a continuation token, pointing at both the token and the end of the previous line

### Requirement: Blocks and block value

A block is `{`, then a sequence of items separated by inferred statement boundaries, then `}`. A block is an expression. Its value is its final item when that item is an expression; when the final item is a statement, or the block is empty, the block has no value. To discard a final expression's value explicitly, the author binds it to `_` as the final item (`let _ = expr`). Typing of block values is ratified by the types chapter.

#### Scenario: Trailing expression is the block value

- **WHEN** a block's final item is an expression, for example a block ending in `a * 2` with no item after it
- **THEN** the block's value is that expression

#### Scenario: Final statement leaves no value

- **WHEN** a block's final item is a binding or assignment, or the block is empty
- **THEN** the block has no value

#### Scenario: Discarding a value is explicit

- **WHEN** a final expression's value is not the intended block value
- **THEN** the author writes `let _ = expr` as the final item; the block then has no value

### Requirement: Statements

Statement forms are ratified family by family, each family by its owning chapter; the set of families and their members is closed per chapter and grows only through spec-layer changes in the owning chapter. This chapter ratifies: binding statements (`let name = expr` and `var name = expr`, each optionally with a type annotation `name: type` before `=`; the grammar of types is ratified by the types chapter), assignment statements (`name = expr`, where richer assignment targets are ratified with their owning chapters), and expression statements (any expression as an item). The control-flow chapter ratifies control-flow statements (if, while, loop, break, continue, return, defer). Assignment is a statement, not an expression: it produces no value and MUST NOT appear where an expression is required.

#### Scenario: Assignment nested where an expression is required

- **WHEN** assignment appears inside an expression position, for example `let x = y = 1` or a call argument `f(a = 1)`
- **THEN** the compiler rejects it with `E0103:` assignment is not an expression

#### Scenario: Binding carries an optional annotation

- **WHEN** a binding is written as `let a = expr` or `let a: type = expr`
- **THEN** both forms are accepted; the annotation binds the name to that type per the types chapter

#### Scenario: Expression statement

- **WHEN** an expression that is not a binding or an assignment appears as a block item
- **THEN** it is an expression statement; when it is also the final item it is the block value per Blocks and block value

#### Scenario: A statement family grows

- **WHEN** a later chapter ratifies new statement forms
- **THEN** they enter through a spec-layer change in that chapter's own scope; this chapter's ratified families are unchanged by it

### Requirement: Expression skeleton

Primary expressions are: identifiers, literals ratified by chapter 1, and parenthesized expressions `( e )`. Postfix forms are member access `receiver.name` and call `expr(args)`, chaining left-associatively; whether an accessed name is a field or a method is resolved by the types chapter. Unary prefix operators are `!`, `-`, `~`, binding tighter than every binary operator. Keyword-led expression forms are ratified by their owning chapters: the control-flow chapter ratifies `if` with else, the match chapter ratifies `match`; they sit outside the primary/postfix/unary skeleton, do not modify it, and a keyword-led expression form enters only through a spec-layer change in its owning chapter. Index syntax `expr[expr]` is deferred to the collections chapter; `[` and `]` remain lexical tokens.

#### Scenario: Postfix chains group left-to-right

- **WHEN** `a.f(x).g(y)` is parsed
- **THEN** the postfixes apply in sequence from the primary outward — access `f`, call, access `g`, call — which is the grouping `(((a.f)(x)).g)(y)`; access and call chain left-to-right

#### Scenario: Parentheses group exactly

- **WHEN** `( e )` appears at any expression position
- **THEN** the parentheses fix the grouping of `e`; the parenthesized form is interchangeable with `e` except where this chapter requires explicit grouping

#### Scenario: A keyword-led expression form is used

- **WHEN** `if` with else or `match` appears at an expression position
- **THEN** the form follows its owning chapter's requirements; the primary, postfix, and unary layers of this skeleton are unchanged

### Requirement: Operator precedence and associativity

Binary operator precedence is the following closed table, tightest first; associativity is as listed per level, and the comparison and equality level is non-associative:

| Level | Operators | Associativity |
| --- | --- | --- |
| 1 | postfix call and member access | left |
| 2 | unary prefix `!` `-` `~` | prefix |
| 3 | `*` `/` `%` | left |
| 4 | `+` `-` | left |
| 5 | `<<` `>>` | left |
| 6 | `&` | left |
| 7 | `^` | left |
| 8 | `\|` | left |
| 9 | `<` `<=` `>` `>=` `==` `!=` | none |
| 10 | `&&` | left |
| 11 | `\|\|` | left |

The table is closed: adding an operator, changing a level, or changing associativity is a spec change. `=`, `.`, `->`, and `=>` are not binary operators and never appear inside expressions at this chapter's scope.

#### Scenario: Mixed arithmetic groups by the table

- **WHEN** `a + b * c` is parsed
- **THEN** it groups as `a + (b * c)` per levels 3 and 4

#### Scenario: Bitwise operators bind tighter than comparison

- **WHEN** `a & mask == flag` is parsed
- **THEN** it groups as `(a & mask) == flag` per levels 6 and 9

#### Scenario: Chained comparison is rejected

- **WHEN** `a < b < c` or `x == y == z` is parsed without parentheses
- **THEN** the compiler rejects it with `E0104:` chained non-associative operator, and the remediation shows the split form, for example `(a < b) && (b < c)`

#### Scenario: Parenthesized comparison does not chain

- **WHEN** `(a < b) && (b < c)` is parsed
- **THEN** each comparison is a single explicit group and no `E0104:` fires

### Requirement: Grammar diagnostics segment

The grammar chapter owns registry segment `E0100`–`E0199` declared in `docs/spec/diagnostics.toml` `[segments]`. First allocations: `E0101` parse ambiguity, `E0102` statement begins with a continuation token, `E0103` assignment is not an expression, `E0104` chained non-associative operator, `E0105` unexpected token. `E0100` and `E0106`–`E0199` are reserved. Trigger semantics live in this chapter's Requirements; entries live in the registry. Allocating further grammar codes extends the registry in the same change.

#### Scenario: A token fits no ratified production

- **WHEN** at the current parse position a token fits no ratified production — for example an index form `list[0]` while index syntax is not yet ratified, or a stray closing bracket
- **THEN** the compiler rejects it with `E0105:` unexpected token, naming the token and the productions considered at that position

#### Scenario: A grammar code is emitted

- **WHEN** any `E01xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry — severity, title, description, remediation, owner `0200-grammar` — is retrievable from `docs/spec/diagnostics.toml`

#### Scenario: A later change needs grammar diagnostics

- **WHEN** a later chapter ratifies grammar productions requiring new diagnostics, for example control flow
- **THEN** its change allocates numbers within `E0100`–`E0199` by extending the registry in the same change; numbers outside the segment fail validation

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only skeleton-ratified surface forms. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits.

### Line-joining

```we
// Semicolons are never written; statement boundaries are inferred.
let a = compute()
let b = a * 2

// A trailing operator continues onto the next line.
let total = a +
    b

// An operand at end of line ends the statement: the next line is a NEW
// statement (unary minus), not a continuation. To subtract, write `a - b`
// on one line or trail the operator.
let x = a
    - b

// Inside round and square brackets, line breaks are insignificant.
let config = build(
    host,
    port,
)

// Chaining continues with a trailing dot; a leading dot is rejected.
let y = svc.
    query()
let z = svc
    .query()   // E0102: statement begins with a continuation token
```

### Blocks and block value

```we
// A block is an expression; its final expression is its value.
let x = {
    let a = compute()
    let b = a * 2
    b + 1
}

// A final statement leaves no value; discard explicitly with `let _ =`.
let u = {
    save(record)
    let _ = load()
}

// The final expression is ALWAYS the value, even a plain call —
// discard deliberately when the value is not intended.
let v = {
    let _ = save(record)
    load()
}
```

### Statements

```we
let a = 1
var count: Int64 = 0   // type names arrive with the types chapter;
                       // the annotation form is ratified here
count = count + a      // assignment is a statement

let x = y = 1          // E0103: assignment is not an expression
f(a = 1)               // E0103: argument position is an expression
```

### Expressions and precedence

```we
let y = a.f(x).g(z)             // postfixes chain: (((a.f)(x)).g)(z)
let n = -x
let t = !flag
let m = ~bits

let q = a + b * c               // a + (b * c)
let r = a & mask == flag        // (a & mask) == flag: bitwise binds tighter
let ok = a < b < c              // E0104: chained non-associative operator
let good = (a < b) && (b < c)   // explicit grouping is the split form

let first = list[0]             // E0105: index syntax is not ratified yet;
                                // the collections chapter adds it
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| statement boundary | 语句边界 |
| line-joining | 行接续 |
| continuation set | 续行集 |
| block value | 块值 |
| binding statement | 绑定语句 |
| assignment statement | 赋值语句 |
| expression statement | 表达式语句 |
| primary expression | 初等表达式 |
| member access | 成员访问 |
| non-associative | 非结合 |
