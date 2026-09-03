# Delta: grammar (Statements enumeration)

## MODIFIED Requirements

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
