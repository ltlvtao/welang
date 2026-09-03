# docs-language Specification Delta

## ADDED Requirements

### Requirement: Paired translation files

Every Markdown document under `docs/` MUST have a paired Chinese translation file in the same directory, named `<basename>.zh.md`, where `<basename>` carries every naming element of the source file, including any numeric prefix.

#### Scenario: New document is added

- **WHEN** a change adds `docs/spec/0100-types.md`
- **THEN** the same change MUST also add `docs/spec/0100-types.zh.md`

#### Scenario: Missing pairing is detected

- **WHEN** `docs_sync.py` runs against a repository state where `docs/process/development-process.md` has no sibling `.zh.md`
- **THEN** it MUST report a failure naming the unpaired file and exit non-zero

#### Scenario: Orphan translation is detected

- **WHEN** a `.zh.md` file exists whose source file does not
- **THEN** `docs_sync.py` MUST report it as a failure

### Requirement: English authority

The English document is the authoritative text. Where the English and Chinese renderings of the same document disagree, the English text governs, and the Chinese rendering MUST be corrected in the change that introduced the divergence.

#### Scenario: Divergence between renderings

- **WHEN** a reviewer finds that `docs/README.md` and `docs/README.zh.md` state different rules
- **THEN** `docs/README.md` is applied as the rule, and the divergence is treated as a translation defect to be fixed

### Requirement: Structural synchronization check

The repository MUST provide `openspec/tools/docs_sync.py` that verifies, for every `*.md` under `docs/` other than translation files themselves: (a) a paired `<basename>.zh.md` exists; (b) the count of headings at each level matches between the pair; (c) the count of fenced code blocks matches between the pair. It MUST also report orphan translation files. It MUST exit 0 when all checks pass and 1 otherwise.

#### Scenario: Heading drift between a pair

- **WHEN** the English document gains a new `## Section` heading that is not reflected in its translation
- **THEN** `docs_sync.py` reports a heading-count mismatch for that pair and exits 1

#### Scenario: Clean state

- **WHEN** every document pair is present and structurally aligned
- **THEN** `docs_sync.py` prints an OK summary and exits 0

### Requirement: Bilingual sync is part of change completion

A change that creates or modifies a document under `docs/` MUST NOT reach `complete` until the paired translation reflects the same change and `docs_sync.py` passes.

#### Scenario: docs change seeks completion

- **WHEN** a change edits `docs/README.md` without editing `docs/README.zh.md`
- **THEN** the change fails review and remains below `complete`
