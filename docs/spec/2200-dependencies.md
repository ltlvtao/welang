# We Language Specification — Chapter 22: Dependencies


### Requirement: Dependency declarations

The manifest's `[dependencies]` table declares the project's direct dependencies: each key is a package name, each value that package's version constraint under this chapter's grammar. The table is optional — a manifest without it declares an empty dependency set, and `we new` writes none. A key MUST be a package name under chapter 1's naming convention — lowercase letters, digits, and hyphens — and the reserved name `std` is not declarable: the standard library is built in per chapter 15, never a dependency; a key outside the convention, or `std`, MUST be rejected with `E2006:` invalid dependency name. The manifest's own `version` field is a semantic version under this chapter's shape — chapter 21 left the shape to the ecosystem layer, and this chapter is that layer arriving; an illegal value MUST be rejected with `E2004:` invalid version value. A declared dependency no source answers is resolution's to reject (this chapter's requirement on it); the declaration itself is checked here, and a value that is not a well-formed constraint string is rejected under the grammar requirement.

#### Scenario: An absent table is an empty dependency set

- **WHEN** a manifest carries no `[dependencies]` table and a project command runs
- **THEN** the project has no direct dependencies, resolution finds an empty graph, and the run proceeds — absence is a statement, not an error

#### Scenario: An illegal dependency name is rejected

- **WHEN** a manifest writes `My_Lib = "^1.0.0"` under `[dependencies]`
- **THEN** the toolchain reports `E2006:` invalid dependency name — package names follow chapter 1's convention, lowercase letters, digits, and hyphens

#### Scenario: The reserved std is not declarable

- **WHEN** a manifest writes `std = "^1.0.0"` under `[dependencies]`
- **THEN** the toolchain reports `E2006:` invalid dependency name — `std` names the built-in standard library per chapter 15, and it is never a dependency

#### Scenario: The project's own version is a semantic version

- **WHEN** a manifest writes `version = "1.0"` or `version = "preview"`
- **THEN** the toolchain reports `E2004:` invalid version value — the field's shape is the semantic version this chapter fixes, and chapter 21's deferral to the ecosystem layer is discharged

### Requirement: Semantic versions and constraint meanings

A version is three dot-separated non-negative integers without leading zeros — `major.minor.patch`, compared component-wise numerically. Pre-release suffixes and build metadata are deliberately not carried: the comparison stays total and numeric, and their absence is this chapter's own boundary, not an oversight to be fixed silently later. The constraint grammar is exactly four forms, each a floor over versions: `^x.y.z` — floor `x.y.z`, ceiling below `(x+1).0.0`, the same major version; `~x.y.z` — floor `x.y.z`, ceiling below `x.(y+1).0`, the same minor version; `>=x.y.z` — floor `x.y.z`, no ceiling; `=x.y.z` — floor and ceiling exactly `x.y.z`. Four forms, four meanings, one spelling each — the uniqueness principle's exception for borrowing the semver ecosystem's operators, as v0.8 already argued: these are four different semantics each with exactly one spelling, not one semantic with many paths. There is no zero-major special case: `^` means the same major uniformly, `~` the same minor uniformly, whatever the major's value. A constraint string outside the four forms, or one whose operand is not a well-formed version, MUST be rejected with `E2003:` invalid version constraint.

#### Scenario: The four forms carry their four meanings

- **WHEN** the constraints `^1.2.3`, `~1.2.3`, `>=1.2.3`, and `=1.2.3` are each read
- **THEN** their floors are all `1.2.3`; their ceilings are below `2.0.0`, below `1.3.0`, none, and exactly `1.2.3` respectively

#### Scenario: A fifth form is rejected

- **WHEN** a manifest writes `lib = "1.x"` or `lib = ">1.0.0"` or `lib = "^1, ^2"`
- **THEN** the toolchain reports `E2003:` invalid version constraint — the grammar is the four forms, wildcards, comparators beyond the set, and compositions are all outside it

#### Scenario: A malformed operand is rejected

- **WHEN** a manifest writes `lib = "^1.2"` or `lib = "~01.2.3"`
- **THEN** the toolchain reports `E2003:` invalid version constraint — the operand must be a well-formed version, three integers, no leading zeros

### Requirement: Resolution by the maximum of floors

Resolution selects one version per package, and the rule is the maximum of floors: every constraint the graph imposes on a package contributes its floor, and the resolved version is the maximum of those floors. The graph is every manifest reachable from the root — the root's `[dependencies]`, and each resolved package's own manifest read at its resolved version. The rule runs to a fixed point: a newly included package's manifest may add floors on further packages, floors only raise picks, picks only widen the graph, and a finite source universe makes the iteration well-founded. The outcome is order-independent — the maximum of a set does not depend on the order it was collected in — and it is registry-state-independent: a newer release appearing in the sources changes no resolution until a constraint's floor asks for it, for the picks are a function of the manifests alone. Every pick must satisfy every constraint in the graph and must exist in the sources: a resolved version violating any ceiling, or a demanded version no source holds, MUST be rejected with `E2002:` unsatisfiable dependency constraint, the message naming the package, the pick, and the violated constraint with its dependent chain. A package name that no source answers at all MUST be reported with `E2001:` unknown dependency. One version per package holds across the whole graph — version splitting does not exist, two parts of a graph cannot build against two versions of one package. `std` never enters resolution: it is built in, chapter 15's reservation, and no constraint is read for it.

#### Scenario: Transitive floors converge to their maximum

- **WHEN** the root declares `a = "^1.0.0"` and `b = "~1.2.0"`, and `a` at `1.0.0` declares `b = ">=1.1.0"`
- **THEN** `a` resolves to `1.0.0` — the only floor on it is its declared floor — and `b` resolves to `1.2.0`, the maximum of the floors `1.1.0` and `1.2.0`, which exists and satisfies both constraints; one version each, whichever order the graph was walked

#### Scenario: A ceiling violation is unsatisfiable

- **WHEN** the root declares `a = "^1.0.0"` and `b = "=1.0.0"`, and `a` at its resolved version declares `b = "^2.0.0"`
- **THEN** the floors on `b` are `1.0.0` and `2.0.0`, the maximum pick `2.0.0` violates the `=1.0.0` ceiling, and the toolchain reports `E2002:` unsatisfiable dependency constraint naming the chain

#### Scenario: An unknown name resolves to nothing

- **WHEN** the root declares `left-pad = "^1.0.0"` and no source holds a package of that name
- **THEN** the toolchain reports `E2001:` unknown dependency — the declaration is well-formed, the name simply answers to nothing

#### Scenario: Resolution is order-independent

- **WHEN** two toolchains resolve the same manifest against the same available versions, or one manifest's table rows are reordered
- **THEN** the picks are identical — the maximum of floors is a function of the constraint set, not of any traversal, search, or ordering

### Requirement: The lockfile

The lockfile is a TOML file named `we.lock` at the project root, machine-written by resolution, committed by the project. It records the transitive closure: for every package the build needs, its exact resolved version and a digest over its acquired content. A lock satisfies the graph when every package the graph reaches has a locked version, every locked version exists in the sources, and every constraint in the graph — the root's and each locked package's own manifest read at its locked version — is satisfied by the locked versions. The lock is the build's truth while it satisfies the graph — the exact versions it names are the versions the build uses, and this is the reproducibility mechanism: two checkouts of one project with one lockfile build against one set of versions whatever their manifests' constraints would freshly admit. A lock that no longer satisfies — a constraint changed, or the graph grew past the lock's closure — sends resolution running again, and the lock is rewritten with the new picks. A malformed lockfile is regenerated, not diagnosed: it is a machine-written surface with no hand-editing contract, and a corrupted one is simply redone. The integrity check is the digest: content acquired into the cache that does not hash to its lock entry's digest MUST be rejected with `E2005:` lockfile integrity mismatch — the lock remembers what was resolved, and the build refuses content that is not what was locked.

#### Scenario: A satisfying lock wins, offline

- **WHEN** a project's `we.lock` names exact versions satisfying the manifest's constraints and every one is present in the cache
- **THEN** the build uses the locked versions, resolution does not run, and the command touches no network — the lock is the truth, the cache suffices, the build reproduces

#### Scenario: A stale lock is re-resolved and rewritten

- **WHEN** a constraint is raised from `^1.0.0` to `^2.0.0` and a project command runs
- **THEN** the lock no longer satisfies the graph, resolution runs under the maximum-of-floors rule, the lock is rewritten with the new picks, and the build proceeds against them

#### Scenario: A digest mismatch fails the build

- **WHEN** a package's cached content hashes to something other than its lock entry's digest
- **THEN** the toolchain reports `E2005:` lockfile integrity mismatch naming the package — the content is not what was locked, and the build refuses it

#### Scenario: A malformed lock is regenerated

- **WHEN** a `we.lock` that is not parseable TOML, or misses a resolved package's entry, sits in the project root
- **THEN** resolution runs as if no lock existed and rewrites the file — no diagnostic is emitted for a machine-written surface

### Requirement: Acquisition and the cache

Every subcommand that runs the check pipeline — `we build`, `we check`, `we run`, `we test`, `we vet`, `we doc` — resolves and acquires before the pipeline's module-resolution stage: the dependency cache must answer every non-local import the compilation will ask for, or the pipeline stops before it starts. `we fmt` and `we clean` do not resolve: neither runs the pipeline. Acquisition fills the dependency cache; the cache's location, layout, eviction, and network protocol are the toolchain's mechanism, fixed nowhere here — observable only through what resolves and what fails, and chapter 15's priority holds unchanged: a dotted path resolving under the project's `src/` names that local module, the cache answers the rest, and `std.` is built in and resolves against nothing. An acquisition needing no new version performs no network access — a satisfied cache means the command runs offline, and this is the property continuous integration depends on. No new subcommand exists for any of this: acquisition is implicit in the pipeline commands, the command surface of chapter 21 stands unchanged, and upgrading a dependency is editing its constraint and running the command again — the deterministic resolution does the rest.

#### Scenario: Pipeline commands acquire before module resolution

- **WHEN** `we check` runs on a project whose imports include `some.lib.util` declared under `[dependencies]`
- **THEN** resolution and acquisition complete first, the cache answers the import under chapter 15's mapping, and the pipeline then runs exactly as for a cache that was always full

#### Scenario: A satisfied cache touches no network

- **WHEN** every version the lock names is already in the cache and the machine has no network
- **THEN** `we build` completes — acquisition had nothing to fetch, so it had nothing to need the network for

#### Scenario: Local sources win over the cache

- **WHEN** `import util.helper` resolves both to `src/util/helper.we` and to a dependency package `util`
- **THEN** the local module answers, deterministically, per chapter 15's priority — this chapter changes nothing of it

#### Scenario: The command surface is unchanged

- **WHEN** a proposal suggests a `we install` or `we add` subcommand for acquisition
- **THEN** it must first amend chapter 21's closed subcommand set — this chapter's design is deliberate: acquisition is implicit, the model writes the manifest row itself, and the surface stays as chapter 21 fixed it

### Requirement: What acquisition does not fix

This chapter names what it leaves open. The package registry and the publish story — the server protocol, package search, accounts and authentication, name-squatting policy, and any `we publish` command — is not designed here: it is ecosystem infrastructure needing operational reality, honestly named as this chapter's one registered gap, and a change designing it extends this chapter or claims its own. Mirror and proxy configuration is the toolchain's mechanism, as is the cache's layout and eviction. Vendoring — copying dependencies into the project tree — is unfixed: nothing here promises it, forbids it, or defines its layout. `we clean` removes build artifacts and touches neither the dependency cache — it lives outside the project, and its management is the mechanism's — nor `we.lock`, which is not a build artifact but the project's record. No acquisition latency, bandwidth, or storage budget is promised anywhere in this chapter.

#### Scenario: The registry and publish story is the named gap

- **WHEN** a change proposal claims this chapter designed the registry or `we publish`
- **THEN** it contradicts this requirement — the gap is registered here for a dedicated change, and the acquisition this chapter fixes ends at the filled cache

#### Scenario: clean leaves the cache and the lockfile

- **WHEN** `we clean` runs on a project with a full cache and a committed lockfile
- **THEN** build artifacts are removed; the cache outside the project and `we.lock` in the root both stand untouched — the cache is the mechanism's, and the lockfile is the project's record, not a build artifact

#### Scenario: No acquisition performance promise

- **WHEN** a toolchain is measured against this chapter for fetch latency or cache footprint
- **THEN** nothing answers — this chapter fixes what resolves and what fails, and performance is evaluation's to measure, not specification's to promise

### Requirement: The dependencies diagnostics segment

The dependencies chapter owns the registry segment `E2000`–`E2099`, declared in `docs/spec/diagnostics.toml` under `[segments]`, and the unclaimed range narrows to `E2100`–`E9999`. The segment holds the six errors `E2001` unknown dependency, `E2002` unsatisfiable dependency constraint, `E2003` invalid version constraint, `E2004` invalid version value, `E2005` lockfile integrity mismatch, `E2006` invalid dependency name — v0.8's `E0111` renumbered to `E2001`, v0.8's `E0151` renumbered to `E2004` and widened from the project's own version to any version value. The segment's unallocated numbers — `2000` and `2007`–`2099` — are reserved for this chapter's amendments, letterless per chapter 99's shared-space rule. No `W` code is allocated here: all six findings stop the run, and no advisory finding of this layer is promised. Trigger semantics live in this chapter's requirements; the entries live in the registry.

#### Scenario: A dependency code is emitted

- **WHEN** the toolchain emits any `E20xx` diagnostic
- **THEN** its full entry is retrievable from `docs/spec/diagnostics.toml` under owner `2200-dependencies`

#### Scenario: A later change needs this segment's codes

- **WHEN** a future amendment of this chapter needs a new diagnostic
- **THEN** it extends the registry within `E2000`–`E2099` in the same change, or claims its own segment

## Examples (non-authoritative)

The examples below use only surface forms ratified by chapters 1–22. They are illustrative, non-authoritative: in any conflict, the Requirements and Scenarios prevail. Lines annotated with a diagnostic code are rejected forms, shown with the code the toolchain emits.

### Declaring dependencies

```toml
# file: we.toml — the [dependencies] table chapter 22 fixes; absent it is
# an empty dependency set, and we new writes none
name = "web"
version = "0.2.1"
type = "executable"

[dependencies]
router = "^1.4.0"        # floor 1.4.0, ceiling below 2.0.0 — same major
codec = "~2.1.3"         # floor 2.1.3, ceiling below 2.2.0 — same minor
logging = ">=0.3.0"      # floor 0.3.0, no ceiling
testkit = "=1.0.0"       # floor and ceiling exactly 1.0.0
```

```toml
# My_Lib = "^1.0.0"                      // E2006: invalid dependency name
# std = "^1.0.0"                         // E2006: invalid dependency name
# lib = "1.x"                            // E2003: invalid version constraint
# lib = "^1.2"                           // E2003: invalid version constraint
# version = "1.0"                        // E2004: invalid version value
```

### Resolution by the maximum of floors

```toml
# the root asks:
#   a = "^1.0.0"
#   b = "~1.2.0"
# and a at 1.0.0 asks:
#   b = ">=1.1.0"
#
# floors on a: 1.0.0            -> a resolves to 1.0.0
# floors on b: 1.1.0, 1.2.0     -> b resolves to 1.2.0 (exists, satisfies both)
```

```toml
# the root asks:
#   a = "^1.0.0"
#   b = "=1.0.0"
# and a at 1.0.0 asks:
#   b = "^2.0.0"
#
# floors on b: 1.0.0, 2.0.0     -> pick 2.0.0 violates the =1.0.0 ceiling
#                                      // E2002: unsatisfiable dependency constraint
# left-pad = "^1.0.0"           // E2001: unknown dependency (no source answers)
```

### The lockfile

```toml
# file: we.lock — machine-written, committed by the project; the exact
# versions and digests of the transitive closure
[[package]]
name = "a"
version = "1.0.0"
digest = "sha256-9f2c…"

[[package]]
name = "b"
version = "1.2.0"
digest = "sha256-4b8e…"
```

```sh
we check        # lock satisfies the graph + cache holds all -> locked versions, no network
# constraint raised ^1.0.0 -> ^2.0.0, we check again:
#                resolution runs, we.lock rewritten with the new picks, build proceeds
# cached content hashes against its lock entry:
#                                        // E2005: lockfile integrity mismatch
```

### Acquisition and the cache

```we
// file: src/main.we — the import that reaches the cache
import router.route

fn main() effect io -> Result<(), E> {
    return Ok(route(handle))             // router resolves from the cache under
}                                        // chapter 15's mapping; local src/ wins
                                         // over any same-named cache package
```

```sh
# we fmt          — no resolution, no pipeline
# we clean        — removes artifacts; the cache and we.lock stand untouched
# we install      — the shell's error: no such subcommand, chapter 21's set is
#                   closed and this chapter adds none — write the manifest row
```

### Pending later changes

```toml
# The registry and publish story is the named gap: server protocol, search,
# accounts and auth, name policy, and any we publish command — a dedicated
# change extends this chapter or claims its own. Mirrors, proxies, cache
# layout and eviction are the toolchain's mechanism, unfixed here.
```

## Terminology

Key terms of this chapter, English to Chinese, for translation consistency:

| English | 中文 |
| --- | --- |
| dependency | 依赖 |
| package name | 包名 |
| version constraint | 版本约束 |
| semantic version | 语义版本 |
| floor | 下界 |
| ceiling | 上界 |
| resolution | 解析 |
| maximum of floors | 最大下界 |
| fixed point | 不动点 |
| lockfile | 锁文件 |
| digest | 摘要 |
| integrity mismatch | 完整性失配 |
| dependency cache | 依赖缓存 |
| acquisition | 获取 |
| transitive closure | 传递闭包 |
| offline build | 离线构建 |
| registry | 包仓库 |
| named gap | 指名留白 |
| vendoring | 源内嵌仓 |
