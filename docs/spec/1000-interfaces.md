# We Language Specification — Chapter 10: Interfaces and generics

### Requirement: Interface declarations

An interface declaration is a top-level item `interface Name { items }`, optionally prefixed by `pub` and optionally carrying a generic parameter clause after the name per this chapter's Generic parameter declarations. `Name` is PascalCase (`E0011`) and joins the module's one name space under chapter 6, colliding with no other name (`E0404`). The brace group is block-like for chapter 2's line-joining: its items stand one per line, separated by inferred boundaries, with no separator token. Items are method signatures and associated-type declarations (Associated types); an interface declares capability, not data — a `name: type` field item fits no production and is rejected under chapter 2's unexpected-token diagnostic (`E0105`). A method signature is `fn name(receiver, p1: T1, ..., pn: Tn) -> R` — the receiver first and bare per Method receivers, the remaining parameters and the optional return annotation under chapter 6's forms; a signature without a receiver is rejected with `E0801`. A method MAY carry a block body — a default method per Default methods. Method names are camelCase (`E0012`). Interface members carry no `pub`: their reach is the interface's own.

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

### Requirement: Associated types

An associated type is declared inside an interface body as `type Name` on its own item: `Name` is PascalCase (`E0011`), an interface declares at most four associated types (`E0803` otherwise), and a declaration carries no upper bound — `type Name: Bound` is rejected with `E0804`. An associated type names a type hole the implementations fix: inside the interface's method signatures the name is usable as a type reference; an associated-type name of the enclosing interface is a valid type reference there and inside its impls' signatures and bodies. Every impl of the interface MUST bind each declared associated type with an item `type Name = TypeRef` — the right side a type reference under chapter 7 as amended, a bare generic parameter of the impl's own clause permitted — and the bindings MUST precede all method definitions of the impl (`E0806` otherwise); a missing binding is rejected with `E0805`. An interface declaring associated types MUST NOT be boxed with `Dyn` (`E0819`, Dyn values): the erased value could not honor the binding.

#### Scenario: An associated type is declared and used

- **WHEN** an interface declares `type Element` and `fn next(mut self) -> Element` on their own lines, and an impl binds `type Element = Int64` before its methods
- **THEN** the interface's `next` returns the binding's type at that impl, `Int64`

#### Scenario: Five associated types are rejected

- **WHEN** an interface declares five associated types
- **THEN** the compiler rejects it with `E0803:` associated type count above four

#### Scenario: A bound on the declaration is rejected

- **WHEN** `type Element: Eq` appears inside an interface body
- **THEN** the compiler rejects it with `E0804:` associated type declares an upper bound; bounds belong in where clauses at use sites

#### Scenario: A missing binding is rejected

- **WHEN** an impl of `Stream` defines its methods but binds no `Element`
- **THEN** the compiler rejects it with `E0805:` impl misses an associated type binding

#### Scenario: A binding after a method definition is rejected

- **WHEN** an impl's `type Element = Int64` item appears below its first method definition
- **THEN** the compiler rejects it with `E0806:` associated type binding after a method definition; bindings come first

### Requirement: Impl declarations

An impl declaration is a top-level item `impl Name for Head { items }` — optionally with a generic clause after `impl` and a where clause after the head — implementing the interface `Name` for the head type. The interface MUST be a declared interface of this or an imported module; the head MUST be a nominal type — a record, newtype, or sum name, or a generic application of one — and anything else fits no production: a tuple head, a base type head, or a bare generic parameter head is rejected with `E0811`. Items are associated-type bindings then method definitions, block-like line-joining as an interface's. Locality — the orphan rule: the impl is legal in a module only when the interface or the head type is declared in that module (`E0810` otherwise); an impl block itself carries no `pub` — its methods' reach is the method-level `pub` per Inherent impls and the interface's own reach. Completeness: the impl MUST define every interface method that has no default (`E0807` otherwise) and MAY define defaulted ones; a defined method MUST match its interface method's signature — same name, same receiver mutability, same parameter count and types in order, same return type, parameter names free to differ (`E0808` otherwise). Uniqueness: an interface is implemented at most once for one head type under substitution — a second impl, or a generic impl overlapping a concrete one, is rejected with `E0809`. An impl of an interface with associated types binds them per Associated types first.

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

### Requirement: Method receivers

Every method — an interface signature or a definition in an impl block — begins with a receiver: the first parameter, written bare without a type annotation, in one of two forms `self` or `mut self`; its type is the impl head's type, fixed by the impl and never written. A method without a receiver is rejected with `E0801` — inherent impls included, for a method nothing would attach to. The receiver MUST be named `self`; the spelling is enforced as chapter 1 enforces its case conventions, and any other name is rejected with `E0802`. `self` is not a keyword — it is an ordinary camelCase binding usable in the body like any other, and `mut` is the chapter-1 keyword; the receiver form is a distinct production: a bare first parameter in an impl-block method. `self` binds the receiver immutably — reads of fields and method calls are free; `mut self` binds it mutably and additionally admits the receiver field assignment of Receiver field assignment. A `mut self` receiver on a value-category type — a `byval` record or sum — is rejected with `E0812`: value semantics has no honest in-place mutation; gc and resource (`byres`) types admit it. A mut-self method is callable through any receiver expression, an immutable binding included: the binding's immutability governs rebinding, not the object's state — the write lands in place on the shared value, which is the point of the adjudicated in-place channel.

#### Scenario: A self receiver reads

- **WHEN** an impl method `fn upper(self) -> String { self.name }` runs with `self: User` declaring `name: String`
- **THEN** the body reads the receiver's field through the immutable binding

#### Scenario: A mut self receiver declares mutation

- **WHEN** an impl method on a gc record is declared `fn rename(mut self, to: String)` and its body writes `self.name = to`
- **THEN** the receiver binds mutably and the write is the receiver field assignment of this chapter

#### Scenario: A receiver named otherwise is rejected

- **WHEN** an impl method is declared `fn upper(me) -> String`
- **THEN** the compiler rejects it with `E0802:` method receiver must be named self; the spelling keeps every method body uniform

#### Scenario: A mut self receiver on a value-category type is rejected

- **WHEN** an impl on a `byval` record declares `fn bump(mut self)`
- **THEN** the compiler rejects it with `E0812:` mut self receiver on a value-category type; a value copy has nothing to mutate in place

#### Scenario: A mut-self method runs through an immutable binding

- **WHEN** `c.bump()` is called on `let c = Counter { count: 0 }` with `bump` a mut-self method writing `self.count`
- **THEN** the call stands and the write lands in place on the shared value — `c.count` reads the new value; `c` itself is never rebound

### Requirement: Receiver field assignment

Inside a `mut self` method body, `self.field = expr` is the language's one field-assignment form, opening the channel chapter 8 deferred to the interfaces chapter: a gc receiver mutates in place — later reads through the same binding see the write — and a resource (`byres`) receiver mutates likewise. The target is exactly `self.field`: one level, the receiver spelled `self` (`E0802`), the name a field of the receiver's type (`E0816` otherwise); `self.f.g = e`, `u.name = e`, and every other field target fit no production and are rejected under chapter 2's unexpected-token diagnostic (`E0105`). A `self.field = expr` appearing outside a mut-self body — inside a `self` method, a plain fn, or at the top level — is rejected with `E0813`. The expression MUST agree with the field's declared type (`E0501`). Assignment is a statement under chapter 2: it produces no value.

#### Scenario: A gc receiver mutates in place

- **WHEN** `u.rename("bob")` runs `fn rename(mut self, to: String) { self.name = to }` on a gc `User`
- **THEN** the receiver's own field is written; a later `u.name` through the same binding reads `"bob"` — no copy was made

#### Scenario: A resource receiver mutates in place

- **WHEN** a `byres` record's method `fn setFd(mut self, fd: Int64) { self.fd = fd }` runs
- **THEN** the resource's field is written in place; the resource's identity is unchanged

#### Scenario: A write inside a plain self method is rejected

- **WHEN** a method declared `fn reset(self)` writes `self.count = 0`
- **THEN** the compiler rejects it with `E0813:` field write outside a mut self method body; the fix is `mut self` at the receiver

#### Scenario: A write to an unknown field is rejected

- **WHEN** a mut-self body writes `self.email = x` and the receiver's type declares no `email`
- **THEN** the compiler rejects it with `E0816:` no such member on the receiver's type

### Requirement: Inherent impls

An inherent impl is a top-level item `impl Head { items }` with no `for`: it attaches methods directly to a nominal type — the head forms and the nominal rule are Impl declarations'. Locality is the orphan rule's: an inherent impl is legal only in a module that declares the head type (`E0810` otherwise) — extension of a foreign type by inherent methods does not exist; the capability route for a foreign type is a local interface. Items are method definitions with receivers per Method receivers; a definition MAY be prefixed `pub`, making it reachable from importing modules, and without `pub` it is module-local. In a for-form impl the same `pub` governs direct name reach only — through the interface, the interface's reachability governs whatever the method's own. An inherent method shares a name with no other member of the type (`E0814`, Member name resolution).

#### Scenario: An inherent impl parses and its method is called

- **WHEN** `impl User { fn displayName(self) -> String { self.name } }` appears and `u.displayName()` is called
- **THEN** the inherent method resolves per Member name resolution and returns the field's value

#### Scenario: An inherent impl of a foreign type is rejected

- **WHEN** a module imports `User` and writes `impl User { ... }` without declaring it
- **THEN** the compiler rejects it with `E0810:` orphan impl; inherent methods attach only to locally declared types

#### Scenario: A pub method is reachable across modules

- **WHEN** an inherent impl declares `pub fn emit(self)` and `fn log(self)`, and an importing module holds a value of the type
- **THEN** `emit` is callable there and `log` is not — it is module-local; the fix at the use site is a pub member of its own

### Requirement: Member name resolution

Postfix member access on a receiver resolves against the receiver's type: the candidate names are the type's fields (chapter 8), its inherent methods (Inherent impls), the methods of every interface it implements (Impl declarations), and generated derive methods (Derives). This grounds chapter 2's deferred field-or-method resolution and chapter 8's record side. A name that is no field and no method of the type is rejected with `E0816`. One name, one meaning: a type MUST NOT carry two members of one name — a field against a method, an inherent method against an interface method, or methods of two interfaces — rejected with `E0814` at the responsible declarations. Where the receiver's static position is an interface — a `Dyn<I>` value (Dyn values) or `self` inside a default method body (Default methods) — the candidate set is that interface's method set alone: a name outside it is rejected with `E0815`, whether or not some erased type might carry it; fields are never in the set. An unconstrained generic parameter is opaque — no member calls on it, rejected with `E0817` — and where bounds restore the bounded interfaces' method sets (Where clauses), calls are checked against them. Method call arguments and results follow chapter 7 as everywhere: argument agreement `E0501`, the result type the method's declared return.

#### Scenario: A method resolves through an implemented interface

- **WHEN** `u.describe()` is called, `User` implementing `Describable` with `fn describe(self) -> String`
- **THEN** the member resolves to the interface's method; the call's type is `String`

#### Scenario: An unknown member is rejected

- **WHEN** `u.email` appears and the receiver's type declares no field or method `email`
- **THEN** the compiler rejects it with `E0816:` no such member on the receiver's type

#### Scenario: A field and a method of one name are rejected

- **WHEN** a record declares the field `name` and an inherent impl declares `fn name(self) -> String`
- **THEN** the compiler rejects the collision with `E0814:` member name collision on one type

#### Scenario: Methods of two interfaces of one name are rejected

- **WHEN** a type implements two interfaces that both declare `render`
- **THEN** the compiler rejects the second impl with `E0814:` member name collision on one type

#### Scenario: A call outside a Dyn method set is rejected

- **WHEN** `d.inherentHelper()` appears with `d: Dyn<Describable>` and `inherentHelper` not a method of `Describable`
- **THEN** the compiler rejects it with `E0815:` method call outside the receiver's method set; the erased type's own members are not reachable

#### Scenario: A call on an unconstrained parameter is rejected

- **WHEN** `fn show<T>(x: T)` calls `x.describe()` with no bound on `T`
- **THEN** the compiler rejects it with `E0817:` method call on an unconstrained generic parameter; the fix is `where T: Describable`

### Requirement: Default methods

An interface method MAY declare a body — a default method. An impl MAY omit a defaulted method: the default body runs for that type; it MAY define the method itself, the signature matching as every impl method's (`E0808`). Inside a default body, `self`'s static position is the interface: its members are the interface's method set alone (`E0815` outside it), fields never in the set — no concrete type is known there. A default body may call the interface's other methods through `self`; such calls are late-bound — they run the impl's body where one exists, the default otherwise. A type MUST NOT inherit a default for a name it also declares inherently — two bodies would answer one selector — rejected with `E0814`; the fix is defining the method in the for-form impl, which is the override. This tightens v0.8's inherent-priority rule from silent selection to rejection, recorded in this change's design.

#### Scenario: An omitted method runs the default

- **WHEN** an interface declares `fn greet(self) -> String { "hello" }` and `User`'s impl omits `greet`
- **THEN** `u.greet()` runs the default body and yields `"hello"`

#### Scenario: A defined method overrides the default

- **WHEN** the same interface's impl for `Admin` defines `fn greet(self) -> String { "welcome" }`
- **THEN** `a.greet()` runs the impl's body — the definition overrides the default

#### Scenario: A default calls a sibling through self

- **WHEN** a default `greet` body calls `self.name()` and the impl defines `name` itself
- **THEN** the call runs the impl's `name` — late-bound through the receiver

#### Scenario: An inherent method against an inherited default is rejected

- **WHEN** a type with an inherent `fn greet(self)` implements the interface while omitting `greet`, inheriting its default
- **THEN** the compiler rejects it with `E0814:` member name collision on one type; the fix is defining the method in the impl

### Requirement: Dyn values

`Dyn<Interface>` is a type reference under chapter 7 as amended naming a type-erased box: a gc-category value carrying some implementing type's value, reached only through the interface's method set. `Dyn` is an ordinary PascalCase type name of the standard scope — not a keyword — like the base type names; its collisions are the module-system chapter's business as theirs are. Construction is exactly one form, the explicit one: `Dyn<Interface>(expr)`, where the expression's type MUST implement the interface (`E0818` otherwise); no contextual `Dyn(expr)` form exists and none is pending — the adjudicated single-form rule, zero inference at the box. The interface MUST be an interface — `Dyn<Int64>`, `Dyn<User>`, `Dyn<T>` are rejected with `E0820` — and MUST NOT declare associated types (`E0819`). A bare interface name is not a value type: `let d: Describable` or a parameter `x: Describable` is rejected with `E0821` — an interface occupies type slots only as `Dyn<Interface>`. Member calls on a Dyn value see the interface's method set alone (`E0815`). Dyn boxes are gc values: assignment, binding, argument passing, and return share the box — no copy of the boxed value occurs.

#### Scenario: A Dyn box is constructed and called

- **WHEN** `let d = Dyn<Describable>(u)` appears with `User` implementing `Describable`, then `d.describe()` is called
- **THEN** `d` is of type `Dyn<Describable>` and the call dispatches to `User`'s implementation

#### Scenario: A non-implementing construction is rejected

- **WHEN** `Dyn<Describable>(42)` appears and `Int64` implements no `Describable`
- **THEN** the compiler rejects it with `E0818:` Dyn construction from a non-implementing type

#### Scenario: An associated-type interface is rejected

- **WHEN** `Dyn<Stream>(s)` appears and `Stream` declares the associated type `Element`
- **THEN** the compiler rejects it with `E0819:` Dyn of an interface with associated types; the binding could not be honored

#### Scenario: A non-interface argument is rejected

- **WHEN** `Dyn<Int64>(5)` or `Dyn<User>(u)` appears
- **THEN** the compiler rejects it with `E0820:` Dyn argument is not an interface type

#### Scenario: A bare interface type slot is rejected

- **WHEN** `fn f(x: Describable)` or `let d: Describable = ...` appears
- **THEN** the compiler rejects it with `E0821:` interface name used as a value type; the box form is `Dyn<Describable>`

### Requirement: Derives

A record, newtype, or sum declaration MAY carry a derives clause: the keyword `derives` (chapter 1 as amended) trailing a comma-separated list from the closed set `Eq`, `Hash`, `Show`, written on the declaration's last line — `record Point { x: Float64, y: Float64 } derives Eq, Hash`, `newtype UserId(Int64) derives Eq`, `type Shape = Circle(Float64) | Rectangle(Float64, Float64) derives Eq`; a `derives` starting a line after a complete declaration fits no production (chapter 2's `E0105`). An unknown or duplicated target is rejected with `E0824`. The clause generates methods that join the type's members exactly as implemented ones, callable and colliding under Member name resolution: `Eq` generates `.equals(other: Self) -> Bool`, `Hash` generates `.hash() -> Int64`, `Show` generates `.toDebugString() -> String`; the generated bodies' runtime behavior is the runtime's — this chapter fixes the surface and the requirements. `Eq` and `Hash` require every field type, underlying type, or payload type to carry the capability — base types carry it, composites carry it through their own derives — else `E0823`; `Show` asks nothing. On generic declarations the requirement is checked at each instantiation (`E0823` there). The targets are builtin: a manual `impl Eq for ...` is rejected with `E0822` — composite equality is the generated `.equals()`, never the operator: `==` compares base types only under chapter 7, and this is the whole story of composite equality, as adjudicated. `Encodable` and `Decodable` are deferred to the JSON change and `Shareable` to the concurrency chapter; they will extend this set through this requirement's amendment.

#### Scenario: Three derives parse on their declarations

- **WHEN** `record Point { x: Float64, y: Float64 } derives Eq, Hash`, `newtype UserId(Int64) derives Eq`, and `type Shape = Circle(Float64) | Rectangle(Float64, Float64) derives Eq` appear
- **THEN** each parses with its trailing clause; `Point` gains `.equals` and `.hash`, `UserId` gains `.equals`, `Shape` gains `.equals` — and `Show` would generate `.toDebugString` likewise

#### Scenario: Composite equality is .equals()

- **WHEN** `p1.equals(p2)` appears on two `Point` values that derive `Eq`
- **THEN** the call resolves to the generated method and yields a `Bool` comparing field-wise

#### Scenario: The operator stays base-types-only

- **WHEN** `p1 == p2` appears on `Point` values
- **THEN** the compiler rejects it under chapter 7's `E0501`; the fix is `p1.equals(p2)` — `==` compares base types only

#### Scenario: A manual impl of a derive target is rejected

- **WHEN** `impl Eq for User { ... }` appears
- **THEN** the compiler rejects it with `E0822:` manual impl of a builtin derive target; the clause is `record User { ... } derives Eq`

#### Scenario: An unmet field requirement is rejected

- **WHEN** `record Wrap { inner: Shape } derives Eq` appears and `Shape` derives no `Eq`
- **THEN** the compiler rejects it with `E0823:` derive field requirement unmet, naming `Shape`; the fix is `derives Eq` on `Shape` too

#### Scenario: An unknown or duplicate target is rejected

- **WHEN** `derives Ord` or `derives Eq, Eq` appears
- **THEN** the compiler rejects it with `E0824:` unknown or duplicate derive target; the closed set is `Eq`, `Hash`, `Show`

### Requirement: Generic parameter declarations

A generic parameter clause is `<T1, ..., Tk>` with k from 1 to 8 — k above eight is rejected with `E0825`, the arity mirror of the composite chapters' eights — each parameter a PascalCase name (`E0011`) scoped to its declaration. The clause may be carried by: top-level fn declarations (between the name and the parameter list), record declarations (after the name, before the brace), sum type declarations (after the name, before `=`), interface declarations (after the name, before the brace), and impl declarations (after `impl`, before the interface name). Generic methods — a clause on an impl-block method — are not ratified; they arrive, if wanted, through this requirement's amendment. A clause MUST NOT redeclare a name of an enclosing clause (`E0826`); no nesting context is ratified at this chapter, and the rule binds from the moment one arrives — generic methods or nested declarations with the function-types chapter. Within the declaration a parameter name is usable at every type-reference position; an unconstrained parameter is opaque — no member calls (`E0817`), operators under chapter 7's rules — until a where bound names it. Category honesty on generic fields is checked at each instantiation: `byval record Box<T> { value: T }` instantiated `Box<User>` with gc `User` is rejected under chapter 8's `E0601` at the instantiation, and the value-sum mirror under `E0702` likewise.

#### Scenario: A generic fn clause parses

- **WHEN** `fn identity<T>(x: T) -> T { x }` appears as a top-level item
- **THEN** it parses as one fn declaration with a one-parameter clause per this chapter; chapter 6's forms govern the rest

#### Scenario: Record, sum, and interface clauses parse

- **WHEN** `record Box<T> { value: T }`, `type Result<T> = Ok(T) | Err(String)`, and `interface Sink<T> { fn send(mut self, item: T) }` appear
- **THEN** each parses with its clause; `T` is usable in the fields, payloads, and signatures respectively

#### Scenario: Nine parameters are rejected

- **WHEN** a declaration carries a nine-parameter clause
- **THEN** the compiler rejects it with `E0825:` generic parameter count above eight

#### Scenario: A shadowing clause is rejected when nesting arrives

- **WHEN** a later chapter ratifies a nesting context and an inner clause redeclares an enclosing clause's name
- **THEN** the compiler rejects it with `E0826:` generic parameter shadows an enclosing parameter; the rule is ratified now and binds then

#### Scenario: A value instantiation with a gc argument is rejected

- **WHEN** `byval record Box<T> { value: T }` is instantiated `Box<User>` with `User` a gc record
- **THEN** the compiler rejects the instantiation under chapter 8's `E0601` — category honesty is checked per instantiation

### Requirement: Generic type references and inference

A type reference may be a generic application `Name<T1, ..., Tk>` — chapter 7's named-type rule as amended — where `Name` names a declaration carrying a k-parameter clause and each argument is a type reference; an arity mismatch is rejected with `E0828`. Nested closers must be separated: maximal munch takes `>>` as the shift token (chapter 1), so the nested application is written `Box<Box<Int64> >`, and the unseparated `Box<Box<Int64>>` is rejected under chapter 2's `E0105` with that remediation — this discharges the pointer chapter 1's operator inventory left to the generics chapter. A call of a generic fn or a construction of a generic type determines its type arguments in exactly one of two ways: unification — each argument expression's type unified with its annotated parameter type, or each field or payload expression's type with its declared type at constructions — or the explicit form: `name<T1, ..., Tk>(args)` for fns, `Name<T1, ..., Tk>(args)` for newtype and variant construction, `Name<T1, ..., Tk> { ... }` for record construction and update heads. Expected-type inference does not exist: a type argument the arguments do not determine is rejected with `E0827` — the fix is the explicit form — the adjudicated single-direction rule keeping every call decidable from its own text. An explicit form's arity mismatch is `E0828`. Bounds are checked at every call and instantiation (`E0830`, Where clauses).

#### Scenario: An inferred call unifies from arguments

- **WHEN** `identity(3)` is called on `fn identity<T>(x: T) -> T`
- **THEN** `T` unifies with the argument's type; the call's type is that type — no explicit form is needed

#### Scenario: An explicit call writes its arguments

- **WHEN** `identity<Float64>(1.0)` or `Pair<Int64, String> { first: 1, second: "a" }` appears, `Pair` declaring two parameters
- **THEN** the type arguments are as written, checked for arity (`E0828`) and bounds

#### Scenario: An undetermined call is rejected

- **WHEN** `empty()` is called on `fn empty<T>() -> Box<T>`
- **THEN** the compiler rejects it with `E0827:` generic call does not determine its type arguments; the fix is `empty<Int64>()`

#### Scenario: An arity mismatch is rejected

- **WHEN** `identity<Int64, String>(1)` appears on the one-parameter `identity`
- **THEN** the compiler rejects it with `E0828:` type argument arity mismatch

#### Scenario: Unseparated nested closers are rejected

- **WHEN** `Box<Box<Int64>>` appears in a type slot
- **THEN** the lexer's maximal munch has taken `>>` as the shift token and the compiler rejects the slot under chapter 2's `E0105`, with the separated form `Box<Box<Int64> >` as the remediation

#### Scenario: A construction infers from its fields

- **WHEN** `Pair { first: 1, second: "a" }` appears with `record Pair<A, B> { first: A, second: B }`
- **THEN** `A` and `B` unify with the field expressions' types; the construction's type is `Pair<Int64, String>`

### Requirement: Where clauses

A where clause trails a generic fn declaration's signature (before the body) or an impl head (before the brace): `where c1, c2, ...`, constraints comma-separated, each a bound `T: Bound`, a multi-bound `T: A + B` — the `+` here is the bound combiner of this grammar position, disjoint from expression position per chapter 2's principle — or an associated-type equality `T.Assoc == Concrete`. A bound MUST name a declared interface (`E0829` otherwise): a record, sum, base type, or generic parameter name fits no bound. Bounds grant method sets: within the declaration, a bounded parameter carries its bounds' method sets (Member name resolution), and nothing else about it is known. At every call or instantiation the type arguments MUST satisfy the constraints — an argument not implementing a bound is rejected with `E0830`. An equality's right side MUST be a concrete type — a base type, a nominal type, or a generic application of them; `T.Element == U` and `T.Element == T` are rejected with `E0831` — and inside the declaration the named associated type is then usable as that concrete type. Where clauses on interface declarations are not ratified at this chapter; bounds there, if wanted, arrive through this requirement's amendment.

#### Scenario: A bound grants its method set

- **WHEN** `fn show<T>(x: T) -> String where T: Describable { x.describe() }` appears
- **THEN** the bounded call resolves per Member name resolution; the unbounded form would be `E0817`

#### Scenario: A multi-bound with the plus combiner parses

- **WHEN** `where T: Eq + Hash` appears in a where clause
- **THEN** it constrains `T` to both bounds; both method sets are available inside

#### Scenario: A non-interface bound is rejected

- **WHEN** `where T: User` appears with `User` a record
- **THEN** the compiler rejects it with `E0829:` where bound is not a declared interface

#### Scenario: An unsatisfied bound at a call is rejected

- **WHEN** `show(42)` is called on the bounded `show` above and `Int64` implements no `Describable`
- **THEN** the compiler rejects it with `E0830:` type argument does not satisfy a where bound

#### Scenario: A non-concrete equality right side is rejected

- **WHEN** `where T: Stream, T.Element == U` appears with `U` a generic parameter
- **THEN** the compiler rejects it with `E0831:` associated-type equality right side is not concrete

#### Scenario: An equality fixes an associated type inside

- **WHEN** a where clause carries `T.Element == Int64` and the body sums `self.next()` into an `Int64`
- **THEN** inside the declaration `T.Element` is `Int64` — the sum needs no conversion

### Requirement: No variance, no subtyping

No variance annotations exist: no declaration position marks a generic parameter co- or contravariant, and no keyword for variance is ratified. No subtyping exists: implementing an interface is capability, not inheritance — `impl I for T` does not make `T` acceptable where `I`, where `Dyn<I>`, or where a differently instantiated generic is expected; each such position is rejected under chapter 7's `E0501`. The erasure route is exactly `Dyn<I>` (Dyn values) and the parametric route exactly a bounded parameter (Where clauses): a `User` implementing `Describable` reaches `fn f(d: Dyn<Describable>)` only as `Dyn<Describable>(u)` and `fn g<T>(t: T) where T: Describable` directly as `g(u)`. Records and sums are nominal and closed: no declared subtype relation exists anywhere in the language.

#### Scenario: An implementing type is not its interface

- **WHEN** `f(u)` appears with `u: User` implementing `Describable` and `f` expecting `Dyn<Describable>`
- **THEN** the compiler rejects it under `E0501`; the fix is `f(Dyn<Describable>(u))` — capability is not subtyping

#### Scenario: The parametric route admits it directly

- **WHEN** `g(u)` appears with `g` declared `fn g<T>(t: T) where T: Describable`
- **THEN** the call stands: `T` unifies with `User`, whose capability satisfies the bound

### Requirement: No operator overloading

The operator inventory (chapter 1) and the operators' semantics (chapter 7) are closed: no production binds an operator to a method — an impl or method named `+`, `==`, or `[]` fits no production and is rejected under chapter 2's `E0105`, for an operator token is never an identifier. Composite equality is the generated `.equals()` (Derives); arithmetic and indexing on composites, where they exist, are named methods of their owning chapters — `.add()`, `.at()` — never operators; `==` compares base types only (chapter 7). This is standing, adjudicated alongside the derives set: overloading would make every operator's type a lie.

#### Scenario: An operator-named method is rejected

- **WHEN** an impl declares `fn +(self, other: Point) -> Point`
- **THEN** the compiler rejects it under chapter 2's `E0105`: `+` is an operator token, never a method name; the named method is `.add()`

#### Scenario: The operator on composites is rejected

- **WHEN** `p1 == p2` or `p1 + p2` appears on `Point` values
- **THEN** the compiler rejects both under `E0501`; the fixes are `p1.equals(p2)` and `p1.add(p2)`

### Requirement: Interfaces and generics diagnostics segment

The interfaces-and-generics chapter owns registry segment `E0800`–`E0899` declared in `docs/spec/diagnostics.toml` `[segments]`. Allocations: `E0801` method declares no receiver, `E0802` method receiver must be named self, `E0803` associated type count above four, `E0804` associated type declares an upper bound, `E0805` impl misses an associated type binding, `E0806` associated type binding after a method definition, `E0807` impl misses a non-defaulted interface method, `E0808` impl method signature mismatches the interface method, `E0809` duplicate impl of one interface for one type, `E0810` orphan impl, `E0811` impl head is not a nominal type, `E0812` mut self receiver on a value-category type, `E0813` field write outside a mut self method body, `E0814` member name collision on one type, `E0815` method call outside the receiver's method set, `E0816` no such member on the receiver's type, `E0817` method call on an unconstrained generic parameter, `E0818` Dyn construction from a non-implementing type, `E0819` Dyn of an interface with associated types, `E0820` Dyn argument is not an interface type, `E0821` interface name used as a value type, `E0822` manual impl of a builtin derive target, `E0823` derive field requirement unmet, `E0824` unknown or duplicate derive target, `E0825` generic parameter count above eight, `E0826` generic parameter shadows an enclosing parameter, `E0827` generic call does not determine its type arguments, `E0828` type argument arity mismatch, `E0829` where bound is not a declared interface, `E0830` type argument does not satisfy a where bound, `E0831` associated-type equality right side is not concrete. `E0826`'s rule is ratified with no nesting surface yet; it fires when one is ratified. `E0800` and `E0832`–`E0899` remain reserved for amendments of this chapter. Trigger semantics live in this chapter's Requirements; entries live in the registry.

#### Scenario: An interfaces code is emitted

- **WHEN** any `E08xx` diagnostic is emitted by the toolchain
- **THEN** its complete entry is retrievable from `docs/spec/diagnostics.toml` with owner `1000-interfaces`

#### Scenario: A later change needs this segment's codes

- **WHEN** a later chapter ratifies rules requiring new interface or generics diagnostics — the concurrency chapter's `Shareable`, the JSON change's codecs
- **THEN** its change extends the registry within `E0800`–`E0899` in the same change, or claims its own segment

## Examples (non-authoritative)

The examples below illustrate the Requirements above using only surface forms ratified by chapters 1–10. They are illustrative and non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines marked with a diagnostic code are rejected forms, shown with the code the compiler emits. Generic methods, Encodable/Decodable, Shareable, and effect sets are annotated as pending or deferred.

### Interfaces, impls, and inherent methods

```we
interface Describable {
    fn describe(self) -> String
}

record User {
    id: UserId,
    name: String
} derives Eq, Show

impl Describable for User {
    fn describe(self) -> String {
        "user ${self.name}"
    }
}

impl User {
    pub fn displayName(self) -> String {
        self.name
    }
}

let u = User { id: UserId(1), name: "Ada" }
let text = u.describe()          // through the interface
let name = u.displayName()       // inherent, pub-reachable
```

### Associated types and default methods

```we
interface Stream {
    type Element
    fn next(mut self) -> Element
}

interface Greeter {
    fn greet(self) -> String {
        "hello"                  // a default method: bodies in interfaces
    }
    fn name(self) -> String
}

record IntStream {
    data: Int64
}

impl Stream for IntStream {
    type Element = Int64         // bindings come before the methods
    fn next(mut self) -> Int64 {
        self.data = self.data + 1
        self.data
    }
}

impl Greeter for User {
    fn name(self) -> String {
        self.name
    }
}

let g = u.greet()                // default body runs: greet is omitted
```

### Mutation through mut self

```we
record Counter {
    count: Int64
}

impl Counter {
    fn bump(mut self) {
        self.count = self.count + 1   // the one field-assignment form
    }
    fn read(self) -> Int64 {
        self.count
    }
}

let c = Counter { count: 0 }
c.bump()
let n = c.read()                 // 1: the gc receiver mutated in place

impl Counter {
    fn reset(self) {
        self.count = 0           // E0813: field write outside a mut self
    }                            // method body
}

byval record Pair {
    left: Int64,
    right: Int64
}

impl Pair {
    fn swap(mut self) {          // E0812: mut self receiver on a
    }                            // value-category type
}
```

### Dyn values

```we
let d = Dyn<Describable>(u)      // the one construction form
let text = d.describe()          // the interface's method set alone

let bad = Dyn<Describable>(42)   // E0818: construction from a
                                 // non-implementing type
let s = Dyn<Stream>(stream)      // E0819: Stream has associated types
let n = Dyn<Int64>(5)            // E0820: argument is not an interface

fn label(x: Describable) -> String {
    x.describe()
}                                // E0821: interface name used as a
                                 // value type; write Dyn<Describable>
```

### Derives

```we
record Point {
    x: Float64,
    y: Float64
} derives Eq, Hash, Show

newtype UserId(Int64) derives Eq, Show

type Shape = Circle(Float64) | Rectangle(Float64, Float64) derives Eq

let same = p1.equals(p2)         // composite equality is the generated
let text = p1.toDebugString()    // method; Show asks nothing
let h = p1.hash()

let eq = p1 == p2                // E0501: == compares base types only
impl Eq for Point {             // E0822: manual impl of a builtin
}                                // derive target
record Wrap { inner: Shape } derives Ord
                                 // E0824: unknown or duplicate derive
                                 // target; the set is Eq, Hash, Show
```

### Generics and where clauses

```we
fn identity<T>(x: T) -> T {
    x
}

record Box<T> {
    value: T
}

impl<T> Describable for Box<T> where T: Describable {
    fn describe(self) -> String {
        "box of ${self.value.describe()}"
    }
}

fn show<T>(x: T) -> String where T: Describable {
    x.describe()
}

fn sum<T>(s: T) -> Int64 where T: Stream, T.Element == Int64 {
    s.next() + 1                 // T.Element is Int64 inside
}

let a = identity(3)              // T unifies with the argument: Int64
let b = identity<Float64>(1.0)   // the explicit form
let nested: Box<Box<Int64> > = Box { value: Box { value: 1 } }
                                 // nested closers separated: >> is the
                                 // shift token under maximal munch

fn empty<T>() -> Box<T> {
    ...                          // a Never expression satisfies any return;
}                                // producers are chapter 14's: panic
                                 // and todo

let bad1 = empty()               // E0827: undetermined; write empty<Int64>()
let bad2 = identity<Int64, String>(1)
                                 // E0828: type argument arity mismatch
let bad3: Box<Box<Int64>>        // E0105: unseparated nested closers
```

### Rejected forms

```we
impl Describable for Foreign {  // E0810: orphan impl — neither side
}                                // is declared in this module
impl Describable for (Int64, String) {
}                                // E0811: impl head is not a nominal type
impl Describable for User {
    fn describe(me) -> String {
        ""
    }
}                                // E0802: receiver must be named self
interface Broken {
    fn helper(x: Int64)
}                                // E0801: method declares no receiver
impl User {
    fn name(self) -> String {
        ""
    }
}                                // E0814: member name collision — the
                                 // field name already answers
fn render<T>(x: T) -> String {
    x.describe()
}                                // E0817: unconstrained parameter is
                                 // opaque; add where T: Describable
fn bound<T>(x: T) where T: Point {
}                                // E0829: where bound is not a declared
                                 // interface
```

### Pending later chapters

```we
// Generic methods, Encodable/Decodable (the JSON change), Shareable
// (the concurrency chapter), and interface-level where clauses are
// not ratified at this chapter; effect-set checks belong to the
// effects chapter and are deliberately absent here:
//
// impl Box<T> { fn pair<U>(self, other: U) -> ... }
// record Config derives Encodable
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| interface | 接口 |
| impl block | impl 块 |
| method | 方法 |
| receiver | 接收者 |
| associated type | 关联类型 |
| default method | 默认方法 |
| inherent impl | 固有 impl |
| orphan rule | 孤儿规则 |
| derives clause | derives 子句 |
| derive target | 派生目标 |
| type-erased box | 类型擦除箱 |
| Dyn value | Dyn 值 |
| generic parameter | 泛型参数 |
| type argument | 类型实参 |
| where clause | where 子句 |
| bound | 约束 |
| method set | 方法集 |
| member name resolution | 成员名字解析 |
| capability | 能力 |
| subtyping | 子类型化 |
| variance | 型变 |
| late-bound | 晚绑定 |
