## MODIFIED Requirements

### Requirement: Dyn values

`Dyn<Interface>` is a type reference under chapter 7 as amended naming a type-erased box: a gc-category value carrying some implementing type's value, reached only through the interface's method set. `Dyn` is an ordinary PascalCase type name of the standard scope — not a keyword — like the base type names; it is a prelude name under chapter 15, and a local declaration shadows it as it shadows any prelude name. Construction is exactly one form, the explicit one: `Dyn<Interface>(expr)`, where the expression's type MUST implement the interface (`E0818` otherwise); no contextual `Dyn(expr)` form exists and none is pending — the adjudicated single-form rule, zero inference at the box. The interface MUST be an interface — `Dyn<Int64>`, `Dyn<User>`, `Dyn<T>` are rejected with `E0820` — and MUST NOT declare associated types (`E0819`). A bare interface name is not a value type: `let d: Describable` or a parameter `x: Describable` is rejected with `E0821` — an interface occupies type slots only as `Dyn<Interface>`. Member calls on a Dyn value see the interface's method set alone (`E0815`). Dyn boxes are gc values: assignment, binding, argument passing, and return share the box — no copy of the boxed value occurs.

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
