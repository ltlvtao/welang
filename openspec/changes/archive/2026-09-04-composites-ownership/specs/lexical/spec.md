# Delta: lexical (host amendment for composites)

## MODIFIED Requirements

### Requirement: Keywords

Keywords are fully reserved everywhere: a keyword token is never an identifier, in any syntactic position. We defines **zero soft keywords** — there is no context in which a reserved word may be used as a name, and no context in which an identifier becomes a reserved word. The initial keyword set is closed and enumerated:

```
fn let var pub import as mut
if else return match for in while loop break continue defer
true false foreign
record byval byres newtype with
```

Adding a keyword is a spec-layer change that amends this list. The list contains only words whose syntax is or will be ratified by a chapter; feature-specific words (effects, types, concurrency, testing) join the list together with the chapter that ratifies them. The composite-types chapter added `record byval byres newtype with` in its own change — a breaking change recorded as such there: those five words were identifiers before and are keywords from this list's amendment on.

#### Scenario: A keyword is used as a name

- **WHEN** a keyword from the enumerated list appears where an identifier is required (for example a variable named `match`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word and its position; the lexer never offers it as an identifier token

#### Scenario: A later chapter needs a new keyword

- **WHEN** a content chapter ratifies syntax that introduces a reserved word not in this list
- **THEN** its spec delta MUST amend this list in the same change, and the amendment is a breaking change recorded as such

#### Scenario: The composite keywords are reserved

- **WHEN** `record`, `byval`, `byres`, `newtype`, or `with` appears where an identifier is required (for example a variable named `record`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word; the five are keywords under the composite-types chapter's amendment
