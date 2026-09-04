# Resource records — MODIFIED Requirements

## MODIFIED Requirements

### Requirement: Resource records

A `byres record` declares a resource: reference-passed, with explicit lifetime management. A resource record MUST be released through its Releasable implementation; the release operations, their placement, and the enforcement discipline are chapter 13's — the Releasable contract, the scope resource statement, and the linear release discipline. This chapter fixes the category and its exclusion from update expressions. Duplication discipline: a resource value's identity is the resource; no form in this chapter duplicates one (`E0606` on update; construction makes a new resource, not a copy).

#### Scenario: A resource record declares its category

- **WHEN** `byres record FileHandle { fd: Int64 }` appears
- **THEN** it declares a resource-category record; its release mechanism is chapter 13's Releasable contract

#### Scenario: Resources are not silently droppable

- **WHEN** an expression of a resource record type stands as a discarded statement
- **THEN** chapter 8's value-discard rule applies with no exemption: the non-Unit value MUST be explicitly discarded
