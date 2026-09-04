## MODIFIED Requirements

### Requirement: Interface declarations

An interface declaration is a top-level item `interface Name { items }`, optionally prefixed by `pub` and optionally carrying a generic parameter clause after the name per this chapter's Generic parameter declarations. `Name` is PascalCase (`E0011`) and joins the module's one name space under chapter 6, colliding with no other name (`E0404`). The brace group is block-like for chapter 2's line-joining: its items stand one per line, separated by inferred boundaries, with no separator token. Items are method signatures and associated-type declarations (Associated types); an interface declares capability, not data — a `name: type` field item fits no production and is rejected under chapter 2's unexpected-token diagnostic (`E0105`). A method signature is `fn name(receiver, p1: T1, ..., pn: Tn) [effect-segment] -> R` — the receiver first and bare per Method receivers, the remaining parameters and the optional return annotation under chapter 6's forms, and an optional effect segment between the parameter list and the arrow per chapter 16, the declaration spelling with the `effect` keyword; a signature without a receiver is rejected with `E0801`. A method MAY carry a block body — a default method per Default methods, whose segment governs its body and every override's under chapter 16's exact-match rule. Method names are camelCase (`E0012`). Interface members carry no `pub`: their reach is the interface's own.

#### Scenario: An interface parses as a top-level item

- **WHEN** `interface Describable { fn describe(self) -> String }` appears at the top level
- **THEN** it declares the interface `Describable` with the one method `describe`, reachable as the interface is

#### Scenario: A method without a receiver is rejected

- **WHEN** an interface method is written `fn describe(d: Describable) -> String` — an annotated first parameter and no bare receiver
- **THEN** the compiler rejects it with `E0801:` method declares no receiver; the first parameter is the receiver and is written bare

#### Scenario: An interface name collides in the module name space

- **WHEN** a module declares `interface Describable { ... }` and also `record Describable { ... }`
- **THEN** the compiler rejects the second declaration with `E0404:` duplicate name in one module

#### Scenario: A generic interface with an associated type parses

- **WHEN** an interface is written `interface Container<T> {` on one line, the items `type Element` and `fn first(mut self) -> Element` each on their own line, and `}` on the last
- **THEN** it declares a one-parameter interface with the associated type `Element` and the method `first`, per this chapter's Generic parameter declarations and Associated types

#### Scenario: A method with an effect segment parses

- **WHEN** `interface Store { fn get(mut self, k: String) effect io -> String }` appears
- **THEN** the signature parses with its effect segment between the parameter list and the arrow, per chapter 16; omitted, the method states a pure signature

### Requirement: Impl declarations

An impl declaration is a top-level item `impl Name for Head { items }` — optionally with a generic clause after `impl` and a where clause after the head — implementing the interface `Name` for the head type. The interface MUST be a declared interface of this or an imported module; the head MUST be a nominal type — a record, newtype, or sum name, or a generic application of one — and anything else fits no production: a tuple head, a base type head, or a bare generic parameter head is rejected with `E0811`. Items are associated-type bindings then method definitions, block-like line-joining as an interface's. Locality — the orphan rule: the impl is legal in a module only when the interface or the head type is declared in that module (`E0810` otherwise); an impl block itself carries no `pub` — its methods' reach is the method-level `pub` per Inherent impls and the interface's own reach. Completeness: the impl MUST define every interface method that has no default (`E0807` otherwise) and MAY define defaulted ones; a defined method MUST match its interface method's signature — same name, same receiver mutability, same parameter count and types in order, same return type, parameter names free to differ (`E0808` otherwise) — and same effect set: the impl method's set MUST equal the interface declaration's exactly, missing and extra tags alike rejected with `E1404` under chapter 16. Uniqueness: an interface is implemented at most once for one head type under substitution — a second impl, or a generic impl overlapping a concrete one, is rejected with `E0809`. An impl of an interface with associated types binds them per Associated types first.

#### Scenario: An impl parses, binds, and defines

- **WHEN** `impl Describable for User { fn describe(self) -> String { self.name } }` appears, `Describable` declaring `fn describe(self) -> String`
- **THEN** it implements `Describable` for `User`; the method's receiver is `self: User` by the impl head, and the signature matches the interface's

#### Scenario: A missing non-defaulted method is rejected

- **WHEN** an interface declares `describe` and `render`, neither defaulted, and an impl defines only `describe`
- **THEN** the compiler rejects it with `E0807:` impl misses a non-defaulted interface method, naming `render`

#### Scenario: A signature mismatch is rejected

- **WHEN** an interface declares `fn set(mut self, key: String)` and the impl writes `fn set(self, key: String)`
- **THEN** the compiler rejects it with `E0808:` impl method signature mismatches the interface method — the receiver mutability differs

#### Scenario: A duplicate impl is rejected

- **WHEN** a module holds `impl Describable for User { ... }` twice
- **THEN** the compiler rejects the second with `E0809:` duplicate impl of one interface for one type

#### Scenario: An orphan impl is rejected

- **WHEN** a module imports both `Describable` and `User` and writes `impl Describable for User { ... }` — neither declared locally
- **THEN** the compiler rejects it with `E0810:` orphan impl; the interface or the head type must be declared in this module

#### Scenario: A tuple impl head is rejected

- **WHEN** `impl Describable for (Int64, String) { ... }` appears
- **THEN** the compiler rejects it with `E0811:` impl head is not a nominal type; tuples implement no interfaces — chapter 8's obligation is discharged here

#### Scenario: A generic impl with a where clause parses

- **WHEN** `impl<T> Describable for Box<T> where T: Describable { fn describe(self) -> String { self.value.describe() } }` appears with `record Box<T> { value: T }` declared
- **THEN** it implements `Describable` for every `Box<T>` whose `T` does, per Where clauses
