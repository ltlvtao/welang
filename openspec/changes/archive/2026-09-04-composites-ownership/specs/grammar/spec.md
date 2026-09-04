# Delta: grammar (host amendments for composites)

## MODIFIED Requirements

### Requirement: Line-joining and semicolon inference

Statement boundaries are inferred, not written; the source carries no semicolons. Inside round brackets `( ... )`, square brackets — including attribute units `#[ ... ]` — and the braces of a record construction expression, line breaks carry no significance. Inside braces that delimit blocks — which are not brackets for this Requirement — the depth-zero rule applies. At bracket depth zero, a statement boundary is present after the last token of every source line UNLESS that token is in the continuation set: a binary operator (`+ - * / % == != < <= > >= && || & | ^ << >> ..`), `=`, or `.`. The continuation set is closed; extending it is a spec change. Tokens that begin statements are a fixed class: identifiers, literals, keywords, `(`, `{`, and the prefix operators `!`, `-`, `~`.

#### Scenario: Operator at end of line continues the statement

- **WHEN** at bracket depth zero a line ends with a binary operator, for example `a +` followed by `b` on the next line
- **THEN** no statement boundary is inserted; the parse continues onto the next line

#### Scenario: Operand at end of line ends the statement

- **WHEN** at bracket depth zero a line ends with an identifier, literal, or closing bracket, for example `let a = b` followed by `- c` on the next line
- **THEN** a statement boundary is inserted; `- c` begins a new statement as a unary-minus expression, and the intent `a - c` MUST be written on one line or with the operator trailing

#### Scenario: Line breaks inside brackets are insignificant

- **WHEN** a bracketed construct spans lines, for example a call written `f(a,` on one line and `b)` on the next
- **THEN** the line breaks have no effect; boundaries are inferred only at bracket depth zero

#### Scenario: Line breaks inside construction braces are insignificant

- **WHEN** a record construction spans lines, for example `User {` then `id: UserId(1),` then `name: "Ada",` then `}` on successive lines
- **THEN** the line breaks have no effect; the construction's braces are brackets, its fields are comma-separated, and no statement boundaries are inferred inside

#### Scenario: Statement begins with a continuation-only token

- **WHEN** at bracket depth zero a statement begins with a token that cannot begin a statement, for example `.method()` after a complete previous line
- **THEN** the compiler rejects it with `E0102:` statement begins with a continuation token, pointing at both the token and the end of the previous line

### Requirement: Statements

Statement forms are ratified family by family, each family by its owning chapter; the set of families and their members is closed per chapter and grows only through spec-layer changes in the owning chapter. This chapter ratifies: binding statements (`let name = expr` and `var name = expr`, each optionally with a type annotation `name: type` before `=`; the name position accepts an identifier or, as ratified by chapter 8, a tuple pattern that destructures element-wise; the grammar of types is ratified by the types chapter), assignment statements (`name = expr`, where richer assignment targets are ratified with their owning chapters — chapter 8 closes the record-field side: no field assignment exists), and expression statements (any expression as an item). The control-flow chapter ratifies control-flow statements (if, while, loop, break, continue, return, defer); the iteration chapter ratifies the for statement. Assignment is a statement, not an expression: it produces no value and MUST NOT appear where an expression is required.

#### Scenario: Assignment nested where an expression is required

- **WHEN** assignment appears inside an expression position, for example `let x = y = 1` or a call argument `f(a = 1)`
- **THEN** the compiler rejects it with `E0103:` assignment is not an expression

#### Scenario: Binding carries an optional annotation

- **WHEN** a binding is written as `let a = expr` or `let a: type = expr`
- **THEN** both forms are accepted; the annotation binds the name to that type per the types chapter

#### Scenario: A binding with a tuple pattern

- **WHEN** `let (a, b) = pair` appears as an item
- **THEN** it is a binding statement whose name position is a tuple pattern under chapter 8; the elements bind element-wise

#### Scenario: Expression statement

- **WHEN** an expression that is not a binding or an assignment appears as a block item
- **THEN** it is an expression statement; when it is also the final item it is the block value per Blocks and block value

#### Scenario: Assignment stays name-targeted

- **WHEN** `u.name = "bob"` appears, `u` a record value
- **THEN** it fits no ratified assignment target — chapter 8 closes the record-field side: no field assignment exists — and MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`); modification is chapter 8's update expression

#### Scenario: A statement family grows

- **WHEN** a later chapter ratifies new statement forms
- **THEN** they enter through a spec-layer change in that chapter's own scope; this chapter's ratified families are unchanged by it

### Requirement: Expression skeleton

Primary expressions are: identifiers, literals ratified by chapter 1, parenthesized expressions `( e )` — where, per chapter 8, `( e1, e2, ..., en )` is a tuple expression, `( )` is the unit value, and a parenthesized single expression is grouping only — and record construction expressions `TypeRef { field: expr, ... }` per chapter 8, whose head is a named type reference: a PascalCase identifier or `module.Name`. Postfix forms are member access `receiver.name` and call `expr(args)`, chaining left-associatively; member access on a record denotes its field per chapter 8, and whether an accessed name is a field or a method is otherwise resolved by the types chapters. Unary prefix operators are `!`, `-`, `~`, binding tighter than every binary operator. Keyword-led expression forms are ratified by their owning chapters: the control-flow chapter ratifies `if` with else, the match chapter ratifies `match`; they sit outside the primary/postfix/unary skeleton, do not modify it, and a keyword-led expression form enters only through a spec-layer change in its owning chapter. Index syntax `expr[expr]` is deferred to the collections chapter; `[` and `]` remain lexical tokens.

#### Scenario: Postfix chains group left-to-right

- **WHEN** `a.f(x).g(y)` is parsed
- **THEN** the postfixes apply in sequence from the primary outward — access `f`, call, access `g`, call — which is the grouping `(((a.f)(x)).g)(y)`; access and call chain left-to-right

#### Scenario: Parentheses group exactly

- **WHEN** `( e )` appears at any expression position
- **THEN** the parentheses fix the grouping of `e`; the parenthesized form is interchangeable with `e` except where this chapter requires explicit grouping

#### Scenario: A keyword-led expression form is used

- **WHEN** `if` with else or `match` appears at an expression position
- **THEN** the form follows its owning chapter's requirements; the primary, postfix, and unary layers of this skeleton are unchanged

#### Scenario: A construction expression is a primary

- **WHEN** `User { id: UserId(1), name: "Ada" }` appears at an expression position
- **THEN** it is a primary expression under chapter 8's construction rules; postfixes chain onto it as onto any primary
