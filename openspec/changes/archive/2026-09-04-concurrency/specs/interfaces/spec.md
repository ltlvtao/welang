## MODIFIED Requirements

### Requirement: Derives

A record, newtype, or sum declaration MAY carry a derives clause: the keyword `derives` (chapter 1 as amended) trailing a comma-separated list from the closed set `Eq`, `Hash`, `Show`, written on the declaration's last line — `record Point { x: Float64, y: Float64 } derives Eq, Hash`, `newtype UserId(Int64) derives Eq`, `type Shape = Circle(Float64) | Rectangle(Float64, Float64) derives Eq`; a `derives` starting a line after a complete declaration fits no production (chapter 2's `E0105`). An unknown or duplicated target is rejected with `E0824`. The clause generates methods that join the type's members exactly as implemented ones, callable and colliding under Member name resolution: `Eq` generates `.equals(other: Self) -> Bool`, `Hash` generates `.hash() -> Int64`, `Show` generates `.toDebugString() -> String`; the generated bodies' runtime behavior is the runtime's — this chapter fixes the surface and the requirements. `Eq` and `Hash` require every field type, underlying type, or payload type to carry the capability — base types carry it, composites carry it through their own derives — else `E0823`; `Show` asks nothing. On generic declarations the requirement is checked at each instantiation (`E0823` there). The targets are builtin: a manual `impl Eq for ...` is rejected with `E0822` — composite equality is the generated `.equals()`, never the operator: `==` compares base types only under chapter 7, and this is the whole story of composite equality, as adjudicated. `Encodable` and `Decodable` are deferred to the JSON change; they will extend this set through this requirement's amendment. `Shareable` is not a derive target and never becomes one: the concurrency chapter attaches that marker mechanically from a declaration's own shape, no clause expresses it, and a clause naming it is `E0824` — the closed set this requirement fixes is the whole of it.

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

#### Scenario: Shareable is not a derive target

- **WHEN** `record Point { x: Float64, y: Float64 } derives Shareable` appears
- **THEN** the compiler rejects it with `E0824:` unknown or duplicate derive target; the marker is computed from the declaration's shape, never written into a clause
