# Delta: composites (records, tuples, newtype, unit, ownership, discard, shadowing)

## ADDED Requirements

### Requirement: Ownership categories

Every composite type belongs to exactly one of four ownership categories, and the category decides the type's passing, sharing, and lifetime rules: `gc` (reference-passed, lifetime managed by the collector), `value` (copied on assignment and passing), `resource` (reference-passed, explicit lifetime management), and `newtype` (a zero-cost compile-time wrapper whose layout is erased at runtime). A record declares its category by its prefix: `record` is `gc` by default, `byval record` is `value`, `byres record` is `resource`; a `newtype` declaration is the fourth category. No other category exists, and a composite type MUST NOT change category after declaration.

#### Scenario: The prefixes denote the categories

- **WHEN** the declarations `record User { ... }`, `byval record Point { ... }`, `byres record FileHandle { ... }`, and `newtype UserId(Int64)` appear
- **THEN** their categories are gc, value, resource, and newtype respectively

#### Scenario: The category set is closed

- **WHEN** a later chapter needs a fifth passing discipline
- **THEN** it enters only through a spec-layer change amending this Requirement; no other route exists

### Requirement: Record declarations

A record declaration is a top-level item `record Name { fields }`, optionally prefixed by `byval` or `byres` and by `pub`; the fields are zero or more `name: type` pairs separated by commas, each type a type reference under chapter 7 (as amended by this chapter); field names follow chapter 1's variable rule — camelCase (`E0012`); the record's name is PascalCase (`E0011`). A record with zero fields is legal. Record and newtype names join the module's one name space under chapter 6 and MUST NOT collide with any other name (`E0404`).

#### Scenario: A record parses as a top-level item

- **WHEN** `record User { id: UserId, name: String }` appears at the top level
- **THEN** it declares the gc record `User` with fields `id: UserId` and `name: String`

#### Scenario: An empty record is legal

- **WHEN** `byval record Empty { }` appears
- **THEN** it declares a value-category record with no fields; an empty record of value category carries no data

#### Scenario: A record name collides in the module name space

- **WHEN** a module declares `record User { ... }` and `fn User() { ... }`
- **THEN** the compiler rejects the second declaration with `E0404:` duplicate name in one module

### Requirement: Record construction expressions

A record construction expression is `TypeRef { field: expr, ... }`: the head `TypeRef` is a named type reference — a PascalCase identifier of the enclosing module, or `module.Name` reaching an imported module's public record per chapter 6 — and the braces hold the fields given as `field: expr` pairs separated by commas. The construction names every field of the record exactly once — a field the record does not declare, a declared field left out, or a field named twice is rejected with `E0604`. Each field's expression MUST have the field's declared type; a mismatch is rejected under chapter 7's no-implicit-conversion diagnostic (`E0501`). A head not naming a record type is rejected with `E0603`. Inside a construction's braces, line breaks carry no significance: the braces are brackets for chapter 2's line-joining rule, and the fields are comma-separated.

#### Scenario: A full construction

- **WHEN** `User { id: UserId(1), name: "Ada" }` appears where a value is required, `User` declaring `id: UserId` and `name: String`
- **THEN** it is an expression producing a new `User` value with those fields

#### Scenario: A field mismatch is rejected

- **WHEN** `User { id: UserId(1), name: 42 }` appears with `name: String`
- **THEN** the compiler rejects the `42` under `E0501`; the fix is an explicit conversion method

#### Scenario: A construction with a wrong field set is rejected

- **WHEN** a construction names `email` that `User` does not declare, or omits `name`, or names `id` twice
- **THEN** the compiler rejects it with `E0604:` construction names a field set that does not match the record

### Requirement: Update expressions

An update expression is `TypeRef { field: expr, ..., with &old }` with the same head forms as a construction: it produces a NEW value of the head's record type where the named fields take the given expressions and every unnamed field is copied from `old`. The base `old` MUST be an expression of exactly the head's record type (`E0603` otherwise); the named field set MUST name only declared fields (`E0604`), and each given expression MUST match its field's type (`E0501`). The base value is unaffected: the update never mutates it. Update expressions exist only for gc and value records: a resource record has identity and MUST NOT be updated this way (`E0606`); its fields change only through the mechanisms of the resource and interfaces chapters.

#### Scenario: An update copies the unnamed fields

- **WHEN** `User { name: "bob" with &u1 }` appears, `User` declaring `id` and `name`, `u1: User`
- **THEN** the result is a new `User` whose `name` is `"bob"` and whose `id` is `u1`'s; `u1` is unchanged

#### Scenario: A non-record base is rejected

- **WHEN** an update expression's base has a type other than the constructed record, for example `Point { x: 1.0 with &origin}` with `origin: User`
- **THEN** the compiler rejects it with `E0603:` update base does not have the constructed record type

#### Scenario: A resource record cannot be updated

- **WHEN** `FileHandle { fd: 3 with &h }` appears, `FileHandle` a byres record
- **THEN** the compiler rejects it with `E0606:` update expression on a resource record

### Requirement: Field access

Postfix member access `receiver.name` on a value of a record type denotes that record's field, grounding chapter 2's deferred field-or-method resolution for the record side: the receiver's type names the record, the accessed name one of its fields, and the expression's type is the field's declared type. Accessing a name the record does not declare is rejected (`E0604`); the receiver not being a record is not covered here — methods and other accessed forms arrive with the interfaces chapter. Field access reads: there is no assignment to a field anywhere in the language — `obj.field = value` fits no production and MUST be rejected under chapter 2's unexpected-token diagnostic (`E0105`), regardless of visibility, category, or module.

#### Scenario: Field access reads a field

- **WHEN** `u.name` appears with `u: User` declaring `name: String`
- **THEN** it is an expression of type `String` naming that field's value

#### Scenario: Field assignment does not exist

- **WHEN** `u.name = "bob"` appears anywhere
- **THEN** the compiler rejects it under `E0105:` unexpected token; modification is the update expression, which produces a new value

#### Scenario: An unknown field access is rejected

- **WHEN** `u.email` appears with `User` declaring no `email`
- **THEN** the compiler rejects it with `E0604:` the record declares no such field

### Requirement: Value records

A `byval record` has value semantics: assignment, binding, argument passing, and return copy the whole value; two bindings of a copied value share nothing. Every field of a value record MUST itself be of the base types or of the value category (`E0601` otherwise) — a copy is only honest when everything in it is copyable by value.

#### Scenario: Passing copies

- **WHEN** a `byval record Point` value `p` is passed to a function and the callee constructs a different `Point` from its parameter
- **THEN** the caller's `p` is unaffected; the parameter is an independent copy

#### Scenario: A non-value field is rejected

- **WHEN** `byval record Bad { u: User }` appears with `User` a gc record
- **THEN** the compiler rejects it with `E0601:` value record field is not of the value category or a base type

### Requirement: Resource records

A `byres record` declares a resource: reference-passed, with explicit lifetime management. A resource record MUST be released through its Releasable implementation; the release operations, their placement, and the enforcement diagnostic are the resource chapter's — this chapter fixes only the category and its exclusion from update expressions. Duplication discipline: a resource value's identity is the resource; no form in this chapter duplicates one (`E0606` on update; construction makes a new resource, not a copy).

#### Scenario: A resource record declares its category

- **WHEN** `byres record FileHandle { fd: Int64 }` appears
- **THEN** it declares a resource-category record; its release mechanism is the resource chapter's

#### Scenario: Resources are not silently droppable

- **WHEN** an expression of a resource record type stands as a discarded statement
- **THEN** chapter 8's value-discard rule applies with no exemption: the non-Unit value MUST be explicitly discarded

### Requirement: Newtype declarations

A newtype declaration is a top-level item `newtype Name(Underlying)`, optionally prefixed by `pub`: `Name` is a new type, of the newtype ownership category, whose runtime layout is its underlying type's and is erased — zero overhead, no implicit conversion in either direction. Construction is the call form `Name(expr)` with `expr` of the underlying type; a call whose callee names a newtype is a construction (chapter 6's one name space guarantees a name is never both a function and a newtype, so the call form is never ambiguous). Unwrapping is the field access `.value`, whose type is the underlying type. Mixing a newtype with its underlying type — or with any other type — is rejected under chapter 7's `E0501`. Derives arrive with the interfaces chapter.

#### Scenario: Construct and unwrap

- **WHEN** `let id = UserId(42)` then `let n = id.value` appear, `newtype UserId(Int64)` declared
- **THEN** `id` is of type `UserId`, and `n` is of type `Int64` with value `42`

#### Scenario: No implicit conversion either way

- **WHEN** `UserId(1) + 1` or `let uid: UserId = 42` appears
- **THEN** the compiler rejects both under `E0501`; the transitions are `UserId(42)` and `.value`, both explicit

#### Scenario: The call form is a construction

- **WHEN** `UserId(42)` appears and the module also declares `fn UserId(x: Int64)` — it does not, because `E0404` forbids the collision
- **THEN** the one name space makes the construction reading the only reading; no ambiguity rule is needed

### Requirement: Tuple types and expressions

A tuple type is `(T1, T2, ..., Tn)` with n from 2 to 8, each element a type reference; a tuple expression is `(e1, e2, ..., en)` with the same arity; a tuple expression's type is the tuple of its elements' types, and at any agreement position the tuple types MUST agree element-wise (`E0501`). An arity above eight is rejected with `E0602` — the fix is a record, which names its fields. Tuples do not implement interfaces; that obligation's mechanism is the interfaces chapter's. The unit type is not a tuple: `()` is its own form, and a parenthesized single expression `( e )` is grouping, not a tuple.

#### Scenario: A tuple value

- **WHEN** `let pair: (Int64, String) = (1, "a")` appears
- **THEN** `pair` has the tuple type; the elements' types agree with the annotation element-wise

#### Scenario: Element mismatch is rejected

- **WHEN** `let pair: (Int64, String) = ("a", 1)` appears
- **THEN** the compiler rejects it under `E0501` at each mismatched element

#### Scenario: Arity above eight is rejected

- **WHEN** a tuple type or expression with nine elements appears
- **THEN** the compiler rejects it with `E0602:` tuple arity above eight; the fix is a record

#### Scenario: One element is grouping, not a tuple

- **WHEN** `( e )` appears at any expression position
- **THEN** it groups `e` per chapter 2; there is no one-element tuple type

### Requirement: The unit type

The unit type is `()`; it has exactly one value, written `()`, which carries no data. A block without a value — final item a statement, or empty — has the unit type, amending chapter 7's block value typing. An expression of unit type may be discarded freely; every other type falls under the value-discard rule. The unit type may appear in type-annotation slots and as an element of tuple types.

#### Scenario: The unit value

- **WHEN** `()` appears at an expression position
- **THEN** it is the unique value of the unit type

#### Scenario: An empty block has the unit type

- **WHEN** a block is empty or its final item is a statement
- **THEN** the block's type is `()`, and it may stand where the unit type is required

### Requirement: Tuple patterns and destructuring

A tuple pattern is `(p1, p2, ..., pn)`: each sub-pattern is any ratified pattern, nesting recursively; the pattern matches a tuple value of the same arity, binding each sub-pattern's bindings from the corresponding element. A tuple pattern against a scrutinee that is not a tuple of the same arity is rejected under chapter 7's `E0501` at the match or binding position. One pattern MUST NOT bind the same name twice (`E0404`). A binding statement's name position accepts a tuple pattern — `let (a, b) = pair` destructures element-wise, the same rule as match. Or-pattern consistency under chapter 4 applies to tuple patterns unchanged: alternatives MUST bind the same name set. Exhaustiveness: a tuple type is exhaustible exactly insofar as its element types are — a tuple of non-exhaustible elements needs a wildcard somewhere.

#### Scenario: Destructuring by match

- **WHEN** `match pair { (a, b) => a + b }` appears with `pair: (Int64, Int64)`
- **THEN** the arm matches, binding `a` and `b` to the elements

#### Scenario: Destructuring by let

- **WHEN** `let (a, b) = pair` appears with `pair: (Int64, String)`
- **THEN** `a` is `Int64` and `b` is `String`; the statement is the binding statement of chapter 2 with a tuple pattern in its name position

#### Scenario: Nested patterns

- **WHEN** `match trio { (0, (a, _)) => a, _ => 0 }` appears with `trio: (Int64, (Int64, Int64))`
- **THEN** the nested tuple pattern matches the nested tuple; the wildcard sub-pattern binds nothing

#### Scenario: A duplicate binding in one pattern is rejected

- **WHEN** `let (x, x) = pair` appears
- **THEN** the compiler rejects it with `E0404:` one name bound twice in the same pattern

### Requirement: Value discard

A non-Unit value MUST NOT be dropped silently. Wherever an expression's value is not consumed — an expression statement, the final item of a function body without a declared return type, the final item of a control form's body block, an unbound match-arm body in statement position — the expression MUST be of the unit type or be explicitly discarded by binding to `_` (`let _ = expr`), else the compiler rejects it with `E0605`. The unit type needs no ceremony: unit-valued expressions stand freely. This grounds chapter 6's deferred value-level discard checking.

#### Scenario: A dropped non-Unit value is rejected

- **WHEN** `step()` as a statement returns `Int64`
- **THEN** the compiler rejects it with `E0605:` non-unit value dropped; the fix is `let _ = step()` or consuming the value

#### Scenario: Explicit discard satisfies the rule

- **WHEN** `let _ = step()` appears in the same position
- **THEN** the discard is explicit and the item is legal

#### Scenario: Unit needs no ceremony

- **WHEN** `io.println("hi")` as a statement returns the unit type
- **THEN** the statement stands freely; no discard form is required

### Requirement: If arm agreement

The arms of an if-with-else MUST agree in type: each arm's block type must be the same type, where an arm whose block has no value types as the unit type `()`. Disagreement is rejected under chapter 7's `E0501` at the if expression. This grounds chapter 3's deferred arm-agreement rule; the never-type's exclusion from agreement is the sum-types chapter's. In statement position, an if-with-else whose agreed type is not the unit type falls under the value-discard rule.

#### Scenario: Arms agree through the unit type

- **WHEN** `if flag { io.println("a") } else { io.println("b") }` appears — both arms unit-typed
- **THEN** the if expression has the unit type and stands as a statement freely

#### Scenario: One arm values, one does not

- **WHEN** `if flag { 1 } else { io.println("no") }` appears at an expression position
- **THEN** the compiler rejects it under `E0501`: the first arm is `Int64`, the second is `()`; one arm must convert explicitly or both must agree

### Requirement: Local shadowing

Inside a function body, a later `let` or `var` MAY rebind a name already bound in the same scope: the new binding wins from its statement onward, and the old binding loses its name — it is not modified, merely unreachable by that name. Parameters are bindings and MAY be shadowed the same way. Nested scopes shadow outward bindings by the same rule. Name resolution is the nearest enclosing binding: a use of a name denotes the innermost binding of that name visible at the use site. The module level is chapter 6's: one name, one binding, no shadowing (`E0404`).

#### Scenario: The later binding wins

- **WHEN** `let x = 1` then `let x = normalize(x)` appear in one scope
- **THEN** the second `x` is a new binding; uses after it see the normalized value, and the first binding is out of reach by name

#### Scenario: A parameter may be shadowed

- **WHEN** `fn f(a: Int64) { let a = trimmed(a) ... }` appears
- **THEN** the body's `a` is the new binding; the parameter remains the argument of `trimmed(a)` on the right of its own binding

#### Scenario: Resolution is nearest-binding

- **WHEN** an outer scope binds `n: Int64` and an inner arm binds `n: String`, and the arm body uses `n`
- **THEN** the use denotes the arm's `String` binding — the nearest enclosing binding at the use site
