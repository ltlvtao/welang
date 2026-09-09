// Package diag constructs diagnostics and renders the two fixed protocol
// faces of chapter 21: the JSON Lines event (stdout, machine-operable) and
// the human-readable one-liner (stderr). The JSON field set and order are a
// stability commitment: an existing field is never removed or renamed, a new
// field may be added.
package diag

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Severity is a rendering level of the JSON Lines protocol. The registry's
// two severities map onto error and warning; note and help are advisory
// rendering levels later stages may use.
type Severity string

// The four rendering levels, in protocol order.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
	SeverityHelp    Severity = "help"
)

// Diagnostic is one diagnosable finding. Construct through Error (later:
// Warning/Note) so the event type stays fixed.
type Diagnostic struct {
	severity Severity
	code     string
	message  string
	file     string
	line     int
	column   int
	help     string
}

// Error constructs an error-severity diagnostic. code is the registry code
// (for example "E1904"); message MUST start with the code's registry title —
// one code, one message.
func Error(code, message string) Diagnostic {
	return Diagnostic{severity: SeverityError, code: code, message: message}
}

// Warning constructs a warning-severity diagnostic (the registry's W codes,
// chapter 21 R4's advisory findings). The message discipline is Error's:
// it starts with the code's registry title.
func Warning(code, message string) Diagnostic {
	return Diagnostic{severity: SeverityWarning, code: code, message: message}
}

// AsError returns the same finding at error severity — the promotion face
// of the manifest's [vet] table (M11): the registry's one-code-one-
// severity holds on the registry side, severity is the rendering layer
// here, so a promoted advisory keeps its W code and renders as an error.
func (d Diagnostic) AsError() Diagnostic {
	d.severity = SeverityError
	return d
}

// At attaches a source position. Command-level diagnostics carry the empty
// file and 0/0, per chapter 21's protocol example for E1907.
func (d Diagnostic) At(file string, line, column int) Diagnostic {
	d.file = file
	d.line = line
	d.column = column
	return d
}

// WithHelp attaches the registry entry's remediation as the optional help
// field. Without it the field is omitted from the JSON event entirely —
// the protocol fixes help only "when one exists".
func (d Diagnostic) WithHelp(help string) Diagnostic {
	d.help = help
	return d
}

// Message returns the message text (which starts with the registry title).
func (d Diagnostic) Message() string { return d.message }

// Code returns the registry code.
func (d Diagnostic) Code() string { return d.code }

// JSON renders the diagnostic as one JSON Lines event object without a
// trailing newline. Field order is fixed: type, severity, code, message,
// file, line, column, help.
func (d Diagnostic) JSON() string {
	var b strings.Builder
	b.WriteString(`{"type":"diagnostic","severity":`)
	b.WriteString(jsonString(string(d.severity)))
	b.WriteString(`,"code":`)
	b.WriteString(jsonString(d.code))
	b.WriteString(`,"message":`)
	b.WriteString(jsonString(d.message))
	b.WriteString(`,"file":`)
	b.WriteString(jsonString(d.file))
	b.WriteString(`,"line":`)
	b.WriteString(strconv.Itoa(d.line))
	b.WriteString(`,"column":`)
	b.WriteString(strconv.Itoa(d.column))
	if d.help != "" {
		b.WriteString(`,"help":`)
		b.WriteString(jsonString(d.help))
	}
	b.WriteString(`}`)
	return b.String()
}

// Human renders the human-readable one-line form, matching chapter 21's
// example shape `error[E0102]: ...`. A positioned diagnostic (line != 0)
// carries the conventional file:line:column prefix; command-level
// diagnostics keep the bare shape. The caller adds the newline.
func (d Diagnostic) Human() string {
	prefix := ""
	if d.line != 0 {
		prefix = d.file + ":" + strconv.Itoa(d.line) + ":" + strconv.Itoa(d.column) + ": "
	}
	return prefix + string(d.severity) + "[" + d.code + "]: " + d.message
}

// jsonString encodes one string value with encoding/json so escaping is
// exactly the standard library's (safe for control characters, quotes,
// backslashes, and non-ASCII text in messages).
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		// json.Marshal cannot fail for a string input; keep the encoder
		// total rather than panicking on an impossible path.
		return `""`
	}
	return string(b)
}
