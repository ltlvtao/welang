# Delta: grammar (range operator, continuation set, statement family)

## MODIFIED Requirements

### Requirement: Line-joining and semicolon inference

Statement boundaries are inferred, not written; the source carries no semicolons. Inside round brackets `( ... )` and square brackets — including attribute units `#[ ... ]` — line breaks carry no significance. Inside braces, which delimit blocks and are not brackets for this Requirement, the depth-zero rule applies. At bracket depth zero, a statement boundary is present after the last token of every source line UNLESS that token is in the continuation set: a binary operator (`+ - * / % == != < <= > >= && || & | ^ << >> ..`), `=`, or `.`. The continuation set is closed; extending it is a spec change. Tokens that begin statements are a fixed class: identifiers, literals, keywords, `(`, `{`, and the prefix operators `!`, `-`, `~`.

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

### Requirement: Statements

Statement forms are ratified family by family, each family by its owning chapter; the set of families and their members is closed per chapter and grows only through spec-layer changes in the owning chapter. This chapter ratifies: binding statements (`let name = expr` and `var name = expr`, each optionally with a type annotation `name: type` before `=`; the grammar of types is ratified by the types chapter), assignment statements (`name = expr`, where richer assignment targets are ratified with their owning chapters), and expression statements (any expression as an item). The control-flow chapter ratifies control-flow statements (if, while, loop, break, continue, return, defer); the iteration chapter ratifies the for statement. Assignment is a statement, not an expression: it produces no value and MUST NOT appear where an expression is required.

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

### Requirement: Operator precedence and associativity

Binary operator precedence is the following closed table, tightest first; associativity is as listed per level, and the comparison and equality level and the range level are non-associative:

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
| 12 | `..` | none |

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

#### Scenario: Chained range is rejected

- **WHEN** `a..b..c` is parsed
- **THEN** the compiler rejects it with `E0104:` chained non-associative operator; each range must be a single explicit group
