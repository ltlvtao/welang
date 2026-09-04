package cli

// The we new skeleton files, byte-fixed. Each follows the chapters that
// govern it: the manifest carries exactly the keys chapter 21's skeleton
// scenario names; src/main.we declares chapter 15's exact root-module main
// shape over a one-variant named sum (chapters 9 and 14); the test module
// is the one empty module chapter 21's scenario fixes.

const manifestSkeleton = `# file: we.toml — the skeleton chapter 21 fixes; [dependencies] is
# chapter 22's table — optional, absent means an empty dependency set
name = "demo"
version = "0.1.0"
type = "executable"
`

const mainSkeleton = `// file: src/main.we — the root module; chapter 15 fixes its main shape
pub type AppError = Failed(String)

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
`

const testSkeleton = `// file: tests/main_test.we — one empty test module; the default test set is
// every *_test.we under tests/, recursively, and this one holds no test block
`
