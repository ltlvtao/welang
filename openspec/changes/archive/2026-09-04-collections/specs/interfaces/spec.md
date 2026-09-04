## MODIFIED Requirements

### Requirement: Interface declarations

An interface declaration is a top-level item `interface Name { items }`, optionally prefixed by `pub` and optionally carrying a generic parameter clause after the name per this chapter's Generic parameter declarations. `Name` is PascalCase (`E0011`) and joins the module's one name space under chapter 6, colliding with no other name (`E0404`). The brace group is block-like for chapter 2's line-joining: its items stand one per line, separated by inferred boundaries, with no separator token. Items are method signatures and associated-type declarations (Associated types); an interface declares capability, not data — a `name: type` field item fits no production and is rejected under chapter 2's unexpected-token diagnostic (`E0105`). A method signature is `fn name` — optionally carrying a generic parameter clause of its own after the name per Generic parameter declarations — followed by `(receiver, p1: T1, ..., pn: Tn) [effect-segment] -> R`: the receiver first and bare per Method receivers, the remaining parameters and the optional return annotation under chapter 6's forms, and an optional effect segment between the parameter list and the arrow per chapter 16, the declaration spelling with the `effect` keyword; a signature without a receiver is rejected with `E0801`. A method MAY carry a block body — a default method per Default methods, whose segment governs its body and every override's under chapter 16's exact-match rule. Method names are camelCase (`E0012`). Interface members carry no `pub`: their reach is the interface's own.

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

#### Scenario: A method generic clause parses

- **WHEN** `interface Source<T> { fn pick<U>(mut self, f: fn(T) -> U) -> Option<U> }` appears
- **THEN** the method's clause parses after the name per Generic parameter declarations, scoped to the method alone and determined at each call; the collections chapter's iterator combinators are the first declared carriers

### Requirement: Generic parameter declarations

A generic parameter clause is `<T1, ..., Tk>` with k from 1 to 8 — k above eight is rejected with `E0825`, the arity mirror of the composite chapters' eights — each parameter a PascalCase name (`E0011`) scoped to its declaration. The clause may be carried by: top-level fn declarations (between the name and the parameter list), record declarations (after the name, before the brace), sum type declarations (after the name, before `=`), interface declarations (after the name, before the brace), and impl declarations (after `impl`, before the interface name). Method generic clauses are ratified: a method signature or method definition MAY carry a clause of its own — after the method name, before the receiver-carrying parameter list — wherever methods are written: interface method signatures, impl-block method definitions, and inherent-impl methods alike. A method clause's parameters scope to the method: usable in its parameter and return types, invisible outside it, and determined at each call from the call's own text per Generic type references and inference — a method call carries no explicit type-argument form, so an argument set that determines nothing is `E0827` and restructuring the call is the fix. An impl method's clause MUST repeat its interface method's clause exactly — same names, same arity — under `E0808`'s signature match. A clause MUST NOT redeclare a name of an enclosing clause (`E0826`); a method clause nests inside its declaration's clause and the rule binds live there, while nested declarations — declarations inside declarations — remain unratified. Within the declaration a parameter name is usable at every type-reference position; an unconstrained parameter is opaque — no member calls (`E0817`), operators under chapter 7's rules — until a where bound names it. Category honesty on generic fields is checked at each instantiation: `byval record Box<T> { value: T }` instantiated `Box<User>` with gc `User` is rejected under chapter 8's `E0601` at the instantiation, and the value-sum mirror under `E0702` likewise.

#### Scenario: A generic fn clause parses

- **WHEN** `fn identity<T>(x: T) -> T { x }` appears as a top-level item
- **THEN** it parses as one fn declaration with a one-parameter clause per this chapter; chapter 6's forms govern the rest

#### Scenario: Record, sum, and interface clauses parse

- **WHEN** `record Box<T> { value: T }`, `type Result<T> = Ok(T) | Err(String)`, and `interface Sink<T> { fn send(mut self, item: T) }` appear
- **THEN** each parses with its clause; `T` is usable in the fields, payloads, and signatures respectively

#### Scenario: Nine parameters are rejected

- **WHEN** a declaration carries a nine-parameter clause
- **THEN** the compiler rejects it with `E0825:` generic parameter count above eight

#### Scenario: A method clause shadowing the interface's parameter is rejected

- **WHEN** `interface Sink<T> { fn send<T>(mut self, item: T) }` is written — the method clause redeclares the interface's parameter name
- **THEN** the compiler rejects it with `E0826:` generic parameter shadows an enclosing parameter; the rule binds live in the method-clause nesting ratified here

#### Scenario: A value instantiation with a gc argument is rejected

- **WHEN** `byval record Box<T> { value: T }` is instantiated `Box<User>` with `User` a gc record
- **THEN** the compiler rejects the instantiation under chapter 8's `E0601` — category honesty is checked per instantiation
