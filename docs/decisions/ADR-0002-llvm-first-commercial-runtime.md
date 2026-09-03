# ADR-0002: LLVM-first compilation with a self-built commercial runtime

- Status: Accepted
- Date: 2026-09-03
- Decided in: openspec change `ratify-language-foundations`

## Context

spec v0.8 (private reference, `refr/spec-0.8.md`) was implemented once as a demo by compiling to Go. The project goal has since been raised to a **commercially deliverable language product** — compiler and runtime both — with an extreme pursuit of performance, efficiency, and resource footprint. The demo-era host strategy (Go backend, `foreign "go"`, `runtime.We*` type-mapping tables) caps that goal: precise GC, tail calls, and the optimization ceiling are all bounded by the host's runtime model.

## Decision

1. **LLVM IR from day one.** The compiler emits LLVM IR and produces native binaries through LLVM backends; the toolchain pins LLVM versions as a discipline. No intermediate C-source layer.
2. **The runtime is product scope, not a cost.** It is self-built and includes a complete garbage-collection mechanism (precise reclamation), a scheduler, I/O multiplexing, and the concurrency primitives the spec defines.
3. **Compiler implementation language stays Go for v1, decoupled from the target.** It emits textual IR and drives LLVM tooling; the implementation language never constrains the compilation target. Long-term milestone: **self-hosting** — rewriting the compiler in We once the language is stable enough to bootstrap (user adjudication, 2026-09-03). The canonical spec deliberately names no implementation language; it carries only the decoupling discipline.
4. **FFI is `foreign "c"`** — a controlled interoperation boundary at the C ABI, subject to full type, effect, and ownership checking. Go-ecosystem interop, if ever needed, goes through a C bridge in a future decision.
5. **Spec discipline: mechanism neutrality.** The spec defines observable semantics and guarantees on its own terms. Precise GC, pause budgets, and tail-call behavior are implementation-quality goals carried by the runtime and measured by benchmarks; they become spec commitments only through an explicit spec-layer change.

## Dependencies

The product depends on a small, enumerated set of third-party components, each version-pinned (user direction, 2026-09-03):

| Component | Role | Pin |
| --- | --- | --- |
| LLVM **21.1.8** | IR target and native backends (`opt` / `llc` / `lld`) | Exact version; upgrades are explicit toolchain-layer changes |
| clang / clang++ (from the pinned LLVM 21.1.8) | Compiling the runtime's C/C++ sources | Same version pin as LLVM; the runtime ships as prebuilt objects/bitcode with the toolchain, so We users need no C toolchain (user adjudication, 2026-09-03) |
| Go toolchain | Compiler implementation language (v1; self-hosting is the long-term milestone) | Pinned via the `go.mod` toolchain directive at compiler kickoff; the authoritative pin lives there, not in this ADR |
| Platform libc / C ABI | FFI boundary and runtime linkage | Per target platform; the support matrix is decided when the runtime lands |

Pinning rules:

- The live, authoritative version pin migrates to the toolchain's version manifest when the compiler repository structure lands; this ADR records the discipline and the initial pins, not the living values.
- The canonical spec (`docs/spec/`) never carries version numbers — observable semantics are defined on the language's own terms (chapter 0, mechanism-neutrality discipline). Version bumps touch the manifest and ADRs, never the spec text.
- No other third-party runtime dependencies: the runtime is self-built by Decision 2.

## Consequences

- The runtime (GC, scheduler, netpoll, primitives) is a permanent engineering commitment with its own roadmap; it is the accepted price of the commercial goal.
- The evaluation methodology (First-Pass Compile Rate etc.) keeps its center of gravity on the frontend and diagnostics loop (`we check` does not depend on code generation).
- v0.8's host-specific content (`foreign "go"`, `runtime.We*` mapping tables) is demoted to counter-example reference.

## Rejected alternatives

- **Keep the v0.8 Go backend:** the fastest path (the demo proves it), but it caps precise GC, tail calls, and optimization headroom — incompatible with the commercial goal (user decision, 2026-09-03).
- **C source as intermediate layer:** portable and debuggable, but the same ceiling problem one level down — precise stack scanning, zero-cost tail calls, and fine-grained optimization still blocked; C compilation also adds a toolchain stage without removing LLVM from the eventual path. Rejected after user adjudication (2026-09-03).
- **gcc for the runtime's C/C++ sources:** a second toolchain family with its own version line; no LLVM bitcode or cross-linking LTO; per-target cross-toolchains; its strengths (exotic and embedded platforms) are out of the ratified target profile; GPLv3 is a worse fit for a commercial toolchain than LLVM's permissive licensing. Rejected in favor of the pinned-family clang (2026-09-03).
