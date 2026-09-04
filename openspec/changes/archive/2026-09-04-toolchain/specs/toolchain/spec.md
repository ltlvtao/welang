## ADDED Requirements

### Requirement: The we command surface

`we` is the one command-line entry, a subcommand model: every toolchain operation is `we <subcommand> [path] [options]`. The subcommand set is `we new`, `we build`, `we check`, `we run`, `we test`, `we fmt`, `we vet`, `we doc`, `we clean`, `we version`, `we lsp` — this chapter fixes the set; a subcommand outside it is the shell's error, not a diagnostic. A `[path]` argument is optional and defaults to the working directory: a directory path names a project — the directory holding the project manifest per chapter 15 — and a file path names one `.we` file compiled as its own single-file compilation. A path that names nothing MUST be reported with `E1907:` command path not found. In single-file compilation the named file is the whole compilation: `std.` imports resolve from the compiler's built-in modules and any other import is rejected with `E1302` — no source root exists to map it against, and the message says so. `we new <name>` creates the skeleton — a manifest, `src/main.we` with a `pub fn main`, and one empty test module under `tests/` — and a `<name>` outside chapter 1's naming convention MUST be rejected with `E1904:` invalid project name. `we clean` removes build artifacts and leaves sources and the manifest untouched. `we version` prints the compiler and specification versions in a fixed shape with the numbers themselves outside this specification's text — chapter 0's mechanism-neutrality discipline: version values never enter the spec, only the shape does. The global options are accepted by every subcommand: `--json` selects the JSON Lines protocol of this chapter's requirement on it and changes no exit code; `--color` takes `auto`, `always`, or `never`, `auto` the default; `--verbose` widens the human-readable output.

#### Scenario: A directory path names a project

- **WHEN** `we check .` runs in a directory whose project manifest exists
- **THEN** the compilation treats that directory as the project root per chapter 15 and the source root is its `src/`

#### Scenario: A single file is its own compilation

- **WHEN** `we check tool.we` runs on a file importing `std.io` only
- **THEN** the compilation resolves the import from the built-in modules and needs no manifest; a local import in the same file is `E1302`, the message naming that no source root exists in single-file mode

#### Scenario: A path that names nothing is rejected

- **WHEN** `we build nosuchdir` runs and no file or directory answers the path
- **THEN** the command reports `E1907:` command path not found and exits nonzero

#### Scenario: we new validates the project name

- **WHEN** `we new My_Project` runs
- **THEN** the command reports `E1904:` invalid project name and creates nothing — the name follows chapter 1's naming convention

### Requirement: The compile pipeline's observable contract

`we build`, `we check`, and `we run` share one pipeline: lexical analysis, parsing, module resolution, type checking, effect checking, ownership checking, and documentation checking, in that order — the order is observable, for a file with both a parse error and a type error reports the parse error. An `E`-severity diagnostic at any stage stops the pipeline: no later stage runs and no artifact is produced; a `W`-severity diagnostic never stops the pipeline by itself — an advisory promoted to error by the manifest's `[vet]` table stops it exactly as an error does. `we check` stops after documentation checking and produces no artifact — the check leg of chapter 0's generate–check–fix loop needs no codegen; `we build` continues through code generation and produces the artifact named by the manifest; `we run` builds and then executes the built artifact. The one performance promise is determinism of output, never of time: the same inputs produce the same outputs, and incremental compilation, caching, and parallel strategies are implementation details carrying no specification commitment. Linking belongs to `we build`: `we check` does not link, and the linkage of foreign declarations happens at build time under this chapter's requirement on it. Target selection — building for a platform other than the host — is a `we build`-only concern whose target vocabulary is the toolchain's mechanism, not fixed here.

#### Scenario: The stage order is observable

- **WHEN** one file holds both a parse error and a type error
- **THEN** the reported diagnostic is the parse error — the pipeline stops at the first stage that fails, and the type error is not reported in the same run

#### Scenario: An error stops the pipeline, a warning does not

- **WHEN** a project compiles with one warning and no errors, and again with one error added
- **THEN** the first run completes and produces its artifact with the warning reported; the second run stops at the error's stage, reports it, and produces nothing

#### Scenario: we check produces no artifact

- **WHEN** `we check` runs on a clean project
- **THEN** every diagnostic the full pipeline would report through documentation checking is reported and no build artifact is written

#### Scenario: Only the output is promised

- **WHEN** a project builds twice with no source change under two toolchain versions that differ in caching strategy
- **THEN** the artifacts are the same; nothing here promises the second build is faster, slower, or incremental at all

### Requirement: The formatter

`we fmt` formats `.we` sources deterministically: the same input always produces the same output, and the formatter's own output is a fixed point — running it twice changes nothing the second time. The formatter has no configuration options — chapter 0's Principle 7 commitment discharged here — and its rule set is the single source of surface style. The rules: indentation is two spaces, and a tab character is replaced by them; line endings are LF and trailing whitespace is stripped; at most one blank line separates top-level items and the file ends with exactly one newline; a single space surrounds each binary operator, follows each comma, follows each colon, and surrounds each `->`; braces carry one space after `{` and before `}` except the empty block, which is `{}`; import declarations are ordered alphabetically with the `std.*` group first, then third-party modules, one blank line between the groups. Two decisions are deliberately not made: the formatter never wraps a line — where to break a line is an author's judgment with no unique right answer — and it never aligns fields or comments — alignment depends on name lengths and turns renames into noise. Formatting findings such as a tab character or a long line are advisory findings of the toolchain's own, not registry diagnostics; this chapter registers none of them.

#### Scenario: Formatting is deterministic

- **WHEN** the same file is formatted twice from the same input
- **THEN** the two outputs are byte-identical, and formatting the formatter's own output changes nothing

#### Scenario: The rule set is observable

- **WHEN** a file using tabs, CRLF endings, `fn(x,y)`, and unordered imports is formatted
- **THEN** the output uses two-space indentation, LF endings, `fn(x, y)`, and the import groups in their fixed order

#### Scenario: No configuration exists to add

- **WHEN** a proposal suggests a formatter option for line width or style presets
- **THEN** it is rejected under chapter 0's Principle 7 scenario — the formatter's contract here is zero options

### Requirement: Advisory diagnostics and we vet

`we vet` runs the check pipeline and then the advisory layer: heuristic findings reported as `W`-severity diagnostics that the check pipeline alone does not produce. The advisory layer's registry surface is exactly the three warnings this chapter registers — `W1910:` unmocked custom effect in a test, `W1911:` possible indirect nested access to one shared value, `W1912:` function may block on a wait — each with its trigger fixed here; a toolchain MAY carry further heuristic findings of its own, and they are the implementation's surface, not this specification's: they occupy no registry numbers and no specification promises them. The named triggers: `W1910` fires when a test's code path calls a function of a custom effect tag with no mock on that path — the call is real, chapter 20 says so, and the finding advises the mock; `W1911` fires when a function called within a shared value's callback body may access the same binding again — chapter 18's `E1613` sees direct calls only, and this heuristic looks one call through, conservative and false-positive-tolerant, a survey, never a verification; `W1912` fires when a function body directly calls one of the operations whose waits block — `Semaphore.acquire`, `Cond.wait`, `Channel.send`, `Channel.receive`, `TaskHandle.await`, the five of chapter 18's own semantics — information chapter 16 deliberately keeps out of the effect system, offered here as a note instead. Each advisory is configurable in the manifest's `[vet]` table with the value `"warning"` (the default), `"error"` (promoted: it stops the pipeline and the build exactly as an error does), or `"ignore"` (suppressed); any other value MUST be rejected with `E1903:` invalid toolchain configuration value. `we vet` produces no artifact; it exits 0 when no finding remains unignored and 1 otherwise.

#### Scenario: The named warnings fire on their triggers

- **WHEN** a test calls an unmocked custom-effect function, a shared value's callback body calls a helper that may touch the same binding, and a function body calls `Semaphore.acquire`
- **THEN** the three findings `W1910`, `W1911`, and `W1912` are reported — each with its registry entry's title, each advisory, none stopping the check pipeline

#### Scenario: Promotion makes an advisory blocking

- **WHEN** the manifest sets `W1910 = "error"` and a test's path calls an unmocked custom-effect function
- **THEN** `we build` stops as at an error and produces no artifact; `we vet` exits 1

#### Scenario: An invalid configuration value is rejected

- **WHEN** the manifest's `[vet]` table writes `W1912 = "strict"`
- **THEN** the toolchain reports `E1903:` invalid toolchain configuration value, naming the key and the legal values

#### Scenario: Implementation-own findings are not spec surface

- **WHEN** a toolchain release adds a heuristic finding beyond the three named warnings
- **THEN** it reports under that toolchain's own naming and no specification text or registry entry promises it — a second toolchain need not produce it

### Requirement: we test

`we test` runs a project's tests. The default set is every `*_test.we` file under the project's `tests/` directory, recursively; a test module elsewhere in the project is an ordinary module under chapters 15 and 20 — compiled with the project, importable, and not in the default set. Within one file, test blocks run sequentially in source order; across files, order and parallelism are the implementation's, unspecified here — chapter 20 fixes what one test observes, this chapter fixes the run's shape. A test's outcome is `pass` when it runs to its own end and `fail` when it ends at chapter 20's test boundary — one failure route, the assertion and the panic arriving by the same boundary; no third outcome exists, and v0.8's skip and its error/pass split die with the syntax and the second mechanism that carried them. `--filter <pattern>` restricts the run to tests whose description matches the pattern; a filter matching no test runs an empty run that exits 0. `we test` exits 0 when every test passes, 1 when any fails, and 2 when the compilation itself fails — the compile failure never runs a test.

#### Scenario: The default set is tests/ recursively

- **WHEN** a project holds `tests/a_test.we`, `tests/unit/b_test.we`, and `src/helpers_test.we`, and `we test` runs
- **THEN** the first two files' test blocks are the run; the third compiles as an ordinary module and runs nothing

#### Scenario: In-file order is source order

- **WHEN** one test module holds three test blocks and a run reports their completion
- **THEN** the completion order matches the source order — the in-file sequence is a promise, made for reproduction

#### Scenario: One failure route, two spellings of cause

- **WHEN** one test's assertion is false and another panics directly
- **THEN** both outcomes are `fail` — chapter 20's boundary is the one route, and the report may name the cause but the outcome vocabulary is pass and fail

#### Scenario: Exit codes follow the run

- **WHEN** a project's tests all pass, then one is made to fail, then a syntax error is introduced into a test module
- **THEN** the three runs exit 0, 1, and 2 respectively — and in the third, no test runs at all

#### Scenario: A filter that matches nothing is an empty run

- **WHEN** `we test --filter "nosuch.*"` matches no description
- **THEN** the run is empty, the summary reports zero, and the exit code is 0

### Requirement: Exploration

`we test --explore` probes above chapter 20's determinism promise: it re-runs a test under the toolchain's scheduler control, choosing interleavings the deterministic scheduler would not take, with `--iterations N` bounding the runs — the default read from the manifest's `[test].explore-iterations` — and partial-order reduction on by default, `--no-reduce` disabling it. The coverage is sampling and this chapter says so: no exploration is exhaustive, none claims to be, and the number of interleavings of a nontrivial test makes exhaustion impossible in principle. Two guard diagnostics carry the honesty. `E1901:` exploration detected nondeterminism fires when the same test under exploration yields divergent observable interleavings — something outside the virtualized determinism is acting, and the probe found it. `E1902:` unmocked effect executed during exploration fires when an exploration run executes an unmocked custom effect — the real call's behavior breaks the probe's premise of reproducible interleavings, exactly the condition `W1910` advises about in an ordinary run, an error under exploration. Exploration exits 0 when every explored run is deterministic and passes, 1 on any failure or guard diagnostic, 2 on compile failure.

#### Scenario: Exploration probes alternative interleavings

- **WHEN** a test creating tied tasks and a channel runs under `--explore --iterations 200`
- **THEN** the toolchain drives the scheduler across interleavings the deterministic scheduler would not choose, within the iteration budget and with partial-order reduction collapsing equivalent runs

#### Scenario: Divergence is caught

- **WHEN** the same test's explored runs yield different observable interleavings
- **THEN** `E1901:` exploration detected nondeterminism is reported — something real acted outside the virtual clock, and the run exits 1

#### Scenario: An unmocked effect under exploration is an error

- **WHEN** an explored test's path calls an unmocked custom-effect function
- **THEN** `E1902:` unmocked effect executed during exploration is reported — the same condition `W1910` advises on in an ordinary run is an error here, for the probe's premise is reproduction

#### Scenario: The bound is honest

- **WHEN** any documentation of exploration claims exhaustive interleaving coverage
- **THEN** it contradicts this requirement — exploration samples, the deterministic promise reproduces, and neither claims enumeration

### Requirement: The JSON Lines protocol

Every subcommand accepts `--json` and then writes one JSON object per line — JSON Lines, streamable, the machine-operable face of chapter 0's Principle 7. The diagnostic event's fields are `type`, `severity`, `code`, `message`, `file`, `line`, `column`, and, when one exists, `help`; severity is one of `error`, `warning`, `note`, `help` — the four rendering levels; the registry's two severities map onto the first two, and the advisory layer may render at `note` or `help`. The test events are `test-result` with `file`, `name`, `status`, `duration_ms` and `test-summary` with `total`, `passed`, `failed`, `duration_ms` — the summary carrying no error count, there being no outcome but pass and fail. The field sets are a stability commitment: an existing field is never removed and never renamed, a new field may be added, and consumers may depend on this forward — the same discipline the registry's entry schema carries. `--json` changes no exit code: the codes are the human format's codes, and a pipeline reading them behaves identically.

#### Scenario: Diagnostics stream one object per line

- **WHEN** `we check --json` runs on a project with one error and one warning
- **THEN** each diagnostic is one JSON object on its own line with the fixed field set, parseable line by line as the run streams

#### Scenario: Test events carry the run

- **WHEN** `we test --json` runs on a project with two passing tests and one failing
- **THEN** three `test-result` objects and one `test-summary` with `total: 3, passed: 2, failed: 1` are emitted

#### Scenario: Fields are stable

- **WHEN** a later toolchain version adds a field to the diagnostic event
- **THEN** the existing fields keep their names and meanings — a consumer built against the field set here keeps working, and removing or renaming a field is forbidden

#### Scenario: The exit code ignores the format

- **WHEN** the same failing check runs with and without `--json`
- **THEN** both runs exit with the same code — the protocol changes the rendering, never the verdict

### Requirement: The project manifest

The project manifest is a TOML file named `we.toml` at the project root — the directory holding it is the project root, chapter 15's fact restated. The skeleton this chapter fixes: `name`, a string under chapter 1's naming convention — an illegal value MUST be rejected with `E1904:` invalid project name; `version`, a string whose shape is the ecosystem's to constrain, not this specification's; `type`, one of `executable` or `library` — any other value is `E1903:` invalid toolchain configuration value; the `[vet]` table of the advisory layer's requirement; and the `[test]` table, whose key `explore-iterations` is a positive integer — a non-positive or non-integer value is `E1903`. A project invocation — a directory-path command — with no manifest, or a manifest missing `name`, `version`, or `type`, MUST be rejected with `E1905:` project manifest missing or incomplete. What the manifest does not yet carry: dependency declarations, version constraints, and lockfiles are not fixed here — the acquisition story is this chapter's named gap, and the manifest's `[dependencies]` table belongs to the change that designs it.

#### Scenario: The skeleton is checked

- **WHEN** `we build` runs in a directory whose `we.toml` lacks `type`
- **THEN** the toolchain reports `E1905:` project manifest missing or incomplete, naming the missing key

#### Scenario: Illegal values are named

- **WHEN** a manifest writes `type = "bin"` or `explore-iterations = 0`
- **THEN** both are rejected with `E1903:` invalid toolchain configuration value, the message naming the key, the legal values, and what was found

#### Scenario: we new writes the skeleton

- **WHEN** `we new demo` runs
- **THEN** the created `we.toml` carries `name = "demo"`, a `version`, `type = "executable"`, and no tables the toolchain does not define; `src/main.we` holds a `pub fn main` and `tests/` holds one empty test module

#### Scenario: Dependencies are a named gap

- **WHEN** a manifest carries a `[dependencies]` table
- **THEN** this specification fixes nothing about it — neither its rejection nor its meaning; the acquisition story is the named gap of this chapter's open-questions requirement

### Requirement: Documentation generation

`we doc` renders the `///` documentation units of chapter 6's attachment rule into API documentation. The documented surface is the `pub` declarations and nothing else — a non-pub item appears in no page, the visibility discipline of chapter 15 carried into the rendered output. The default output directory is the project's `docs/` directory, `--output` redirects it, and `--check` verifies without generating: it reports pub declarations that carry no documentation unit, as an advisory finding of the toolchain's own — the completeness policy is the project's, not the specification's, and this chapter registers no code for it. Cross-references in documentation content are the tool's rendering business; an unresolvable one is the tool's advisory finding, likewise unregistered.

#### Scenario: The surface is pub-only

- **WHEN** a module holds documented pub and non-pub declarations and `we doc` runs
- **THEN** the pub declarations' pages exist and the non-pub declarations appear nowhere in the output

#### Scenario: Check verifies without generating

- **WHEN** `we doc --check` runs on a project whose every pub declaration carries a `///` unit, then on one with an undocumented pub fn
- **THEN** the first reports nothing and writes nothing; the second reports the gap as the toolchain's own advisory finding and still writes nothing

### Requirement: Linkage of foreign declarations

A name declared in a foreign block binds to exactly one native symbol of the platform's C ABI — chapter 19 defines the declaration and the boundary's checks; this chapter fixes the binding's observable contract. The binding happens at build time: `we check` does not link, and linkage questions do not arise in it. At build, a declared foreign name that binds to no discoverable symbol MUST be reported with `E1906:` unresolved native symbol, the message naming the declaration and the symbol sought. Everything between declaration and symbol is the toolchain's mechanism and is fixed nowhere here: the name mapping or mangling scheme, the library search paths, the library formats, and calling-convention variants beyond the platform C ABI are the build tool's own — observable only through their failures, which carry the one code above.

#### Scenario: An unbound symbol fails the build

- **WHEN** a foreign block declares `fn abs(x: Int64) -> Int64 effect` and the build finds no native symbol answering the binding
- **THEN** the build reports `E1906:` unresolved native symbol naming the declaration, and no artifact is produced

#### Scenario: Check never links

- **WHEN** the same project runs under `we check`
- **THEN** the declaration's checks of chapter 19 run and pass or fail on their own terms; no linkage question arises and `E1906` cannot fire

#### Scenario: The mechanism is the toolchain's

- **WHEN** two toolchains bind the same declaration through different mangling schemes and search paths
- **THEN** both are conforming — the spec fixes the declaration's checks, the binding's failure code, and nothing between

### Requirement: What the toolchain does not fix

This chapter names what it leaves open. Dependency acquisition — the manifest's `[dependencies]` table, version-constraint syntax, lockfile format, and any `install` or `publish` command — is not designed here; chapter 15 fixed the resolution mapping and the cache's priority, and the acquisition story that fills the cache awaits its own change, honestly named as this chapter's one registered gap. The Language Server Protocol lives in a separate document of its own, as v0.8 already intended — the one promise made here is consistency: an editor service's diagnostics are `we check`'s, the same pipeline reporting the same codes, and no editor surface may diverge from the command line's verdicts. Localization of diagnostic output is the toolchain's — the registry's entries are English and the translation layer is outside the specification. Incremental compilation, caching, and build parallelism are implementation details behind the one promise that the same inputs produce the same outputs. Cross-file test parallelism and reporting layout are likewise the implementation's. No performance budget — build time, latency, footprint — is promised anywhere in this chapter.

#### Scenario: The acquisition gap is named, not hidden

- **WHEN** a change proposal claims this chapter designed dependency acquisition
- **THEN** it contradicts this requirement — the gap is registered here for a dedicated change, and `we.toml`'s dependency table belongs to it

#### Scenario: Editor diagnostics are check's diagnostics

- **WHEN** an editor service reports a diagnostic for a file
- **THEN** the codes, messages, and verdicts are those `we check` reports for the same source — one pipeline, one truth, whichever face it shows

#### Scenario: No performance promise exists

- **WHEN** a toolchain is measured against this chapter for build speed or latency budgets
- **THEN** nothing answers — the chapter promises output determinism and observable contracts, and performance is evaluation's to measure, not specification's to promise

### Requirement: Toolchain diagnostics segment

The toolchain chapter owns the registry segment `E1900`–`E1999`, declared in `docs/spec/diagnostics.toml` under `[segments]`, and the unclaimed range narrows to `E2000`–`E9999`. `E` and `W` share the number space — chapter 99's rule, one number one severity — so the segment holds the errors `E1901` exploration detected nondeterminism, `E1902` unmocked effect executed during exploration, `E1903` invalid toolchain configuration value, `E1904` invalid project name, `E1905` project manifest missing or incomplete, `E1906` unresolved native symbol, `E1907` command path not found, and the warnings `W1910` unmocked custom effect in a test, `W1911` possible indirect nested access to one shared value, `W1912` function may block on a wait — the three advisories this chapter promises, v0.8's `W0601`, `W0755`, and `W0756` renumbered into the shared space. The segment's unallocated numbers — `1900`, `1908`–`1909`, and `1913`–`1999` — are reserved for this chapter's amendments, letterless per chapter 99's shared-space rule, for the numbers `1910`–`1912` are the warnings' own. Trigger semantics live in this chapter's requirements; the entries live in the registry.

#### Scenario: A toolchain code is emitted

- **WHEN** the toolchain emits any `E19xx` or `W19xx` diagnostic
- **THEN** its full entry is retrievable from `docs/spec/diagnostics.toml` under owner `2100-toolchain`

#### Scenario: A later change needs this segment's codes

- **WHEN** a future amendment of this chapter needs a new diagnostic
- **THEN** it extends the registry within `E1900`–`E1999` in the same change, or claims its own segment
