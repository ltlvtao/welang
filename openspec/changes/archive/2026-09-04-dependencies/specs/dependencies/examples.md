# Dependencies — illustrative examples

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
