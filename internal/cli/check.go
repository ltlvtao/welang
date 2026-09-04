package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/ltlvtao/welang/internal/parser"
)

// runCheck runs the check subcommand at this slice's boundary: the lexical
// and parsing stages over one source file. A directory reaches the project
// boundary (the project mode is a later milestone), a non-.we path is a
// usage error, and the first diagnostic — lexical or syntactic — exits 1 on
// the protocol faces. A clean parse hands off at the type-checking boundary;
// a form a ratified chapter owns that this build has not implemented yet
// stops there, one boundary line per form group. Both kinds of boundary are
// plain stderr lines without a diagnostic event, under --json too: they are
// implementation-transient, not a spec surface (chapter 21's exit 70).
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
	file, d, ni := parser.Parse(path, src)
	if d != nil {
		e.report(*d)
		return exitDiagnostic
	}
	if ni != nil {
		fmt.Fprintf(e.stderr, "we: %s are not implemented in this reference build yet\n", ni.What)
		return exitNotImplemented
	}
	_ = file // the tree feeds the type-checking milestone
	fmt.Fprintln(e.stderr, "we: type checking is not implemented in this reference build yet")
	return exitNotImplemented
}
