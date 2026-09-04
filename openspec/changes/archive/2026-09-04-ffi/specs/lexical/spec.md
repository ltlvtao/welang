## MODIFIED Requirements

### Requirement: Keywords

Keywords are fully reserved everywhere: a keyword token is never an identifier, in any syntactic position. We defines **zero soft keywords** — there is no context in which a reserved word may be used as a name, and no context in which an identifier becomes a reserved word. The initial keyword set is closed and enumerated:

```
fn let var pub import as mut
if else return match for in while loop break continue defer
true false foreign
record byval byres newtype with
type
interface impl where derives
scope resource
effect
task select case timeout collectAll
```

Adding a keyword is a spec-layer change that amends this list. The list contains only words whose syntax is or will be ratified by a chapter; feature-specific words (effects, types, concurrency, testing) join the list together with the chapter that ratifies them. The composite-types chapter added `record byval byres newtype with` in its own change — a breaking change recorded as such there: those five words were identifiers before and are keywords from this list's amendment on. The sum-types chapter added `type` in its own change, recorded the same way: the word was an identifier before and is a keyword from this list's amendment on. The interfaces-and-generics chapter added `interface impl where derives` the same way: four words were identifiers before and are keywords from this list's amendment on. The resources chapter added `scope resource` the same way: two words were identifiers before and are keywords from this list's amendment on — `scope` will lead further composite forms, its reservation predating those uses as `mut`'s did. The effects chapter added `effect` the same way: the word was an identifier before and is a keyword from this list's amendment on — it introduces the effect declaration and the declaration effect segment under chapter 16. The concurrency chapter added `task select case timeout collectAll` the same way: five words were identifiers before and are keywords from this list's amendment on — they lead the task block, the select expression, and the scope block forms ratified there, `scope` having been reserved by the resources chapter ahead of these composite uses, as its amendment recorded. `mut`, present in the initial list, gains its syntax with `mut self` receivers under chapter 10 — its reservation predates its use. `foreign`, present in the initial list, gains its syntax with the foreign block under chapter 19 — its reservation predates its use, as `mut`'s did.

#### Scenario: A keyword is used as a name

- **WHEN** a keyword from the enumerated list appears where an identifier is required (for example a variable named `match`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word and its position; the lexer never offers it as an identifier token

#### Scenario: A later chapter needs a new keyword

- **WHEN** a content chapter ratifies syntax that introduces a reserved word not in this list
- **THEN** its spec delta MUST amend this list in the same change, and the amendment is a breaking change recorded as such

#### Scenario: The composite keywords are reserved

- **WHEN** `record`, `byval`, `byres`, `newtype`, or `with` appears where an identifier is required (for example a variable named `record`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word; the five are keywords under the composite-types chapter's amendment

#### Scenario: The type keyword is reserved

- **WHEN** `type` appears where an identifier is required (for example a variable named `type`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word; the word is a keyword under the sum-types chapter's amendment

#### Scenario: The interface keywords are reserved

- **WHEN** `interface`, `impl`, `where`, or `derives` appears where an identifier is required (for example a variable named `impl`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word; the four are keywords under the interfaces-and-generics chapter's amendment

#### Scenario: The resources keywords are reserved

- **WHEN** `scope` or `resource` appears where an identifier is required (for example a variable named `scope`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word; the two are keywords under the resources chapter's amendment

#### Scenario: The effect keyword is reserved

- **WHEN** `effect` appears where an identifier is required (for example a variable named `effect`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word; the word is a keyword under the effects chapter's amendment

#### Scenario: The concurrency keywords are reserved

- **WHEN** `task`, `select`, `case`, `timeout`, or `collectAll` appears where an identifier is required (for example a variable named `task`)
- **THEN** the parser reports a syntax diagnostic naming the reserved word; the five are keywords under the concurrency chapter's amendment
