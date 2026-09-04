package diag

import (
	"encoding/json"
	"strings"
	"testing"
)

// e1904 mirrors the E1904 registry entry (docs/spec/diagnostics.toml): the
// message starts with the registry title; the remediation is the help field.
func e1904() Diagnostic {
	return Error("E1904", `invalid project name — "My_Project"`).
		WithHelp("Use lowercase letters, digits, and hyphens per chapter 1's naming convention.")
}

// TestJSONSnapshot pins the JSON Lines event byte-for-byte, with and without
// the optional help field. The field order and set are a stability
// commitment (chapter 21, the JSON Lines protocol): existing fields are
// never removed or renamed; new fields may be added. Changing these bytes
// without a spec-layer change is a protocol break.
func TestJSONSnapshot(t *testing.T) {
	got := e1904().JSON()
	want := `{"type":"diagnostic","severity":"error","code":"E1904","message":"invalid project name — \"My_Project\"","file":"","line":0,"column":0,"help":"Use lowercase letters, digits, and hyphens per chapter 1's naming convention."}`
	if got != want {
		t.Fatalf("JSON event mismatch:\n got: %s\nwant: %s", got, want)
	}

	noHelp := Error("E1907", `command path not found — "nosuchdir"`).JSON()
	wantNoHelp := `{"type":"diagnostic","severity":"error","code":"E1907","message":"command path not found — \"nosuchdir\"","file":"","line":0,"column":0}`
	if noHelp != wantNoHelp {
		t.Fatalf("JSON event without help mismatch:\n got: %s\nwant: %s", noHelp, wantNoHelp)
	}
}

// TestJSONPositionSnapshot pins the position fields' placement and rendering
// for a diagnostic that carries a source location.
func TestJSONPositionSnapshot(t *testing.T) {
	got := Error("E0102", "statement begins with a continuation token — the token is `.`").
		At("src/main.we", 3, 5).JSON()
	want := `{"type":"diagnostic","severity":"error","code":"E0102","message":"statement begins with a continuation token — the token is ` + "`.`" + `","file":"src/main.we","line":3,"column":5}`
	if got != want {
		t.Fatalf("JSON event with position mismatch:\n got: %s\nwant: %s", got, want)
	}
}

// TestHumanSnapshot pins the human-readable one-line form, matching chapter
// 21's example shape `error[E0102]: ...`.
func TestHumanSnapshot(t *testing.T) {
	got := e1904().Human()
	want := `error[E1904]: invalid project name — "My_Project"`
	if got != want {
		t.Fatalf("human rendering mismatch:\n got: %s\nwant: %s", got, want)
	}
}

// TestJSONEscapingRoundTrip checks that messages carrying characters JSON
// must escape still encode to a valid single-line object that parses back
// to the same fields — the event is machine-parseable line by line.
func TestJSONEscapingRoundTrip(t *testing.T) {
	d := Error("E0001", "invalid character — \"引\" \\ \n\t homoglyph confusability")
	line := d.JSON()
	if strings.ContainsAny(line, "\n") {
		t.Fatalf("JSON event must be one line, got: %q", line)
	}
	var parsed struct {
		Type     string `json:"type"`
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Message  string `json:"message"`
		File     string `json:"file"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
		Help     string `json:"help"`
	}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("event does not parse as JSON: %v\nevent: %s", err, line)
	}
	if parsed.Type != "diagnostic" || parsed.Severity != "error" || parsed.Code != "E0001" {
		t.Fatalf("round-trip lost fixed fields: %+v", parsed)
	}
	if parsed.Message != d.Message() {
		t.Fatalf("round-trip message mismatch: %q vs %q", parsed.Message, d.Message())
	}
}
