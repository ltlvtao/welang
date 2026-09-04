package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/ltlvtao/welang/internal/lex"
)

// runCheck runs the check subcommand at this slice's boundary: the lexical
// stage over one source file. A directory reaches the project boundary (the
// project mode is a later milestone), a non-.we path is a usage error, the
// first lexical diagnostic exits 1 on the protocol faces, and a clean lex
// hands off at the parsing boundary. Both boundaries are plain stderr lines
// without a diagnostic event, under --json too: they are implementation-
// transient, not a spec surface (chapter 21's exit 70).
func (e *env) runCheck(path string, info os.FileInfo) int {
	if info.IsDir() {
		fmt.Fprintln(e.stderr, "we: project compilation is not implemented in this reference build yet")
		return exitNotImplemented
	}
	if !strings.HasSuffix(path, ".we") {
		return e.usageErr("check wants a .we file, got %q", path)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return e.fsError(err)
	}
	if _, d := lex.File(path, src); d != nil {
		e.report(*d)
		return exitDiagnostic
	}
	fmt.Fprintln(e.stderr, "we: parsing is not implemented in this reference build yet")
	return exitNotImplemented
}
