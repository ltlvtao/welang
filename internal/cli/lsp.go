package cli

import (
	"os"

	"github.com/ltlvtao/welang/internal/lsp"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// runLsp runs the language server: the LSP binding document's face
// (docs/lsp.md, chapter 21 R12's separate document) over stdio. The
// injected check pipeline is the single-file one `we check <file.we>` runs
// — checkSrc's stages over the document's own text — so the editor's
// diagnostics are check's diagnostics, the binding's one promise. The
// resolved [path] argument stays uniform with the pipeline subcommands
// (E1907 fires on a missing path) but the server's documents arrive as
// URIs and the path is otherwise unused; the global options parse and are
// ignored — stdout is the protocol stream, not a report face.
func (e *env) runLsp() int {
	srv := lsp.NewServer(func(path, src string) lsp.CheckResult {
		file, ds, boundary := checkSrc(path, []byte(src))
		if boundary != "" {
			return lsp.CheckResult{Boundary: boundary}
		}
		if len(ds) > 0 {
			return lsp.CheckResult{Diagnostics: ds}
		}
		// The advisory layer rides the single-file face at the warning
		// default — no manifest, so nothing can promote (check's own
		// posture for a bare file); the findings ride the same push.
		return lsp.CheckResult{Diagnostics: typecheck.Advisories(file, path, typecheck.SingleFile)}
	})
	return srv.Serve(os.Stdin, os.Stdout)
}
