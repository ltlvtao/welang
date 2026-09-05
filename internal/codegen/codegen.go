// Package codegen emits textual LLVM IR for the M4 acceptance set (design
// D3/D4 of the native-vertical change): the skeleton module — erased sum
// declarations plus one main — with main's body being exactly `return
// Ok(())` or `return Err(V("literal"))`. Everything else that survives type
// checking stops at a boundary What. Emission is a pure function — no
// paths, no counters, no time — which is chapter 21's same-input-same-output
// made byte-level.
package codegen

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ltlvtao/welang/internal/ast"
)

// NotImplemented reports one type-clean form outside the acceptance set.
// What slots into `we: %s are not implemented in this reference build yet`;
// the table it draws from is design D3's closed four-row list.
type NotImplemented struct {
	What string
}

// The four boundary Whats, verbatim from design D3's closed table.
const (
	bndMainBody   = "main bodies beyond a single Ok or Err return statement"
	bndErrPayload = "Err payloads beyond one plain string-literal variant argument"
	bndOtherFns   = "functions other than main in code generation"
	bndTopLets    = "top-level value bindings in code generation"
)

// Emit renders f as textual LLVM IR under the module name (the manifest
// name at the call site). A non-nil NotImplemented means f was type-clean
// but outside the acceptance set; the IR string is then empty.
func Emit(f *ast.File, module string) (string, *NotImplemented) {
	// Pass one: collect the module's sum table for the Err variant
	// attribution and find main. Sums emit zero IR — their values never
	// appear as runtime values in the two accepted shapes.
	sums := make(map[string]map[string][]ast.TypeRef)
	var main *ast.FnDecl
	for _, it := range f.Items {
		switch d := it.(type) {
		case *ast.SumDecl:
			variants := make(map[string][]ast.TypeRef, len(d.Variants))
			for _, v := range d.Variants {
				variants[v.Name] = v.Payload
			}
			sums[d.Name] = variants
		case *ast.FnDecl:
			if d.Name == "main" && main == nil {
				main = d
				continue
			}
			return "", &NotImplemented{What: bndOtherFns}
		case *ast.TopLet:
			return "", &NotImplemented{What: bndTopLets}
		case *ast.RecordDecl, *ast.NewtypeDecl:
			// Records and newtypes erase (design D11): their declarations
			// emit zero IR. Their value expressions (construction, update,
			// the call form) never reach this stage within the accepted
			// main-body shapes — the M4 body boundary holds them.
			continue
		case *ast.Import:
			// Unreachable: typecheck stops std/multi-module forms at its own
			// boundary before code generation runs. Row 3 is the nearest
			// honest line if that guarantee ever changes.
			return "", &NotImplemented{What: bndOtherFns}
		}
	}
	if main == nil {
		// Defensive: Project-mode typecheck rejects a missing main (E1305).
		return "", &NotImplemented{What: bndMainBody}
	}

	items := main.Body.Items
	if len(items) != 1 {
		return "", &NotImplemented{What: bndMainBody}
	}
	ret, ok := items[0].(*ast.Return)
	if !ok || !ret.HasValue {
		return "", &NotImplemented{What: bndMainBody}
	}
	call, ok := ret.Value.(*ast.Call)
	if !ok {
		return "", &NotImplemented{What: bndMainBody}
	}
	fn, ok := call.Fn.(*ast.Ident)
	if !ok {
		return "", &NotImplemented{What: bndMainBody}
	}

	switch fn.Name {
	case "Ok":
		if len(call.Args) != 1 {
			return "", &NotImplemented{What: bndMainBody}
		}
		if _, ok := call.Args[0].(*ast.Unit); !ok {
			return "", &NotImplemented{What: bndMainBody}
		}
		return fmt.Sprintf("; ModuleID = '%s'\n\n"+
			"define i32 @__we_main() {\n"+
			"entry:\n"+
			"  ret i32 0\n"+
			"}\n", module), nil

	case "Err":
		if len(call.Args) != 1 {
			return "", &NotImplemented{What: bndErrPayload}
		}
		ctor, ok := call.Args[0].(*ast.Call)
		if !ok || len(ctor.Args) != 1 {
			return "", &NotImplemented{What: bndErrPayload}
		}
		vfn, ok := ctor.Fn.(*ast.Ident)
		if !ok {
			return "", &NotImplemented{What: bndErrPayload}
		}
		lit, ok := ctor.Args[0].(*ast.Literal)
		if !ok || lit.Kind != "string" {
			return "", &NotImplemented{What: bndErrPayload}
		}
		payload, ok := decodeStringLiteral(lit.Text)
		if !ok {
			return "", &NotImplemented{What: bndErrPayload}
		}
		if !isStringPayloadVariant(sums, main.Ret, vfn.Name) {
			return "", &NotImplemented{What: bndErrPayload}
		}
		// The report line is the runtime-written byte sequence: the payload
		// decodes to the bytes the source means, and __we_fail writes exactly
		// this many of them (design D5).
		line := "error: " + vfn.Name + ": " + payload + "\n"
		var sb strings.Builder
		fmt.Fprintf(&sb, "; ModuleID = '%s'\n\n", module)
		fmt.Fprintf(&sb, "@.err = private unnamed_addr constant [%d x i8] c\"%s\"\n\n", len(line), irEscape(line))
		sb.WriteString("declare void @__we_fail(ptr, i64) noreturn\n\n")
		sb.WriteString("define i32 @__we_main() {\nentry:\n")
		fmt.Fprintf(&sb, "  call void @__we_fail(ptr @.err, i64 %d)\n", len(line))
		sb.WriteString("  unreachable\n}\n")
		return sb.String(), nil

	default:
		return "", &NotImplemented{What: bndMainBody}
	}
}

// isStringPayloadVariant is the variant-attribution back-check of design
// D3: name must be a variant of the E in main's `Result<(), E>` return
// annotation, carrying exactly one String payload. Typecheck already
// established the semantics; this only confirms the attribution from the
// module's own declarations, without leaning on typecheck internals.
func isStringPayloadVariant(sums map[string]map[string][]ast.TypeRef, ret ast.TypeRef, name string) bool {
	res, ok := ret.(*ast.NamedType)
	if !ok || res.Qual != "" || res.Name != "Result" || len(res.Args) != 2 {
		return false
	}
	errTy, ok := res.Args[1].(*ast.NamedType)
	if !ok || errTy.Qual != "" {
		return false
	}
	payload, ok := sums[errTy.Name][name]
	if !ok || len(payload) != 1 {
		return false
	}
	str, ok := payload[0].(*ast.NamedType)
	return ok && str.Qual == "" && str.Name == "String" && len(str.Args) == 0
}

// decodeStringLiteral resolves the chapter 1 escape set of a raw string
// literal (Text holds the source slice, quotes included) to the bytes the
// program means. A false return flags an interpolation hole — the one
// payload form M4 does not accept. The lexer has already validated the
// escape syntax, so every other path resolves.
func decodeStringLiteral(text string) (string, bool) {
	if len(text) < 2 || text[0] != '"' || text[len(text)-1] != '"' {
		return "", false
	}
	inner := text[1 : len(text)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c != '\\' {
			if c == '$' && i+1 < len(inner) && inner[i+1] == '{' {
				return "", false
			}
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(inner) {
			return "", false
		}
		switch inner[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '0':
			b.WriteByte(0)
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case '\'':
			b.WriteByte('\'')
		case 'u':
			r, end, ok := parseUnicodeEscape(inner, i)
			if !ok || !utf8.ValidRune(r) {
				return "", false
			}
			var buf [4]byte
			b.Write(buf[:utf8.EncodeRune(buf[:], r)])
			i = end
		default:
			return "", false
		}
	}
	return b.String(), true
}

// parseUnicodeEscape reads the `\u{1..6 hex digits}` form starting at the
// 'u' (index i) of an escape's host string, returning the rune value and
// the index of the closing brace.
func parseUnicodeEscape(s string, i int) (rune, int, bool) {
	if i+1 >= len(s) || s[i+1] != '{' {
		return 0, 0, false
	}
	end := strings.IndexByte(s[i+2:], '}')
	if end < 0 {
		return 0, 0, false
	}
	hex := s[i+2 : i+2+end]
	if len(hex) < 1 || len(hex) > 6 {
		return 0, 0, false
	}
	var v rune
	for j := 0; j < len(hex); j++ {
		d := hexDigit(hex[j])
		if d < 0 {
			return 0, 0, false
		}
		v = v*16 + rune(d)
	}
	return v, i + 2 + end, true
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// irEscape re-escapes decoded bytes for the c"..." constant form: printable
// ASCII except quote and backslash stays raw; every other byte — controls,
// high-bit, quote, backslash — becomes \XX uppercase hex. The doubled forms
// \" and \\ are deliberately unused: on the pinned toolchain the assembly
// lexer breaks on \" inside a c-string (verified empirically — the string
// terminates early), while the hex forms compile, link, and round-trip
// through __we_fail byte-exact.
func irEscape(bytes string) string {
	var b strings.Builder
	for i := 0; i < len(bytes); i++ {
		c := bytes[i]
		switch {
		case c == '"' || c == '\\' || c < 0x20 || c > 0x7E:
			fmt.Fprintf(&b, "\\%02X", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
