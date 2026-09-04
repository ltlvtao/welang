// Package version holds the reference toolchain's version manifest: the
// compiler version, the canonical spec baseline label, and the pinned LLVM
// toolchain version. Version values never enter the spec's text (chapter 0,
// mechanism-neutrality discipline); this package is their one home. The Go
// toolchain pin's one authority is go.mod's toolchain directive (ADR-0002).
package version

// CompilerVersion is the reference compiler's version, a semantic version
// under chapter 22's shape (three dot-separated non-negative integers
// without leading zeros).
const CompilerVersion = "0.1.0"

// SpecVersion labels the canonical spec baseline this compiler implements:
// the first canonical baseline after the private v0.8 draft. The label is
// mechanism — the spec's own text carries no version values.
const SpecVersion = "0.9.0"

// LLVMPin is the pinned LLVM toolchain version (ADR-0002's dependency
// table). This slice displays it only; driving opt/llc/lld against it lands
// with the native-codegen milestone of the implementation roadmap.
const LLVMPin = "21.1.8"
