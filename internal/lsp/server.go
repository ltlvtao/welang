package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/ltlvtao/welang/internal/diag"
)

// CheckResult is what the injected pipeline reports for one document text:
// the diagnostics of the run, and a non-empty Boundary when the pipeline
// stopped at an implementation boundary (no events — nothing to push, the
// document keeps its last diagnostics).
type CheckResult struct {
	Diagnostics []diag.Diagnostic
	Boundary    string
}

// Server is the session machine of docs/lsp.md's Lifecycle: the state flags,
// the open documents, and the injected check pipeline. The pipeline is the
// same function `we check <file.we>` runs, so the editor face cannot diverge
// from the command line's verdicts.
type Server struct {
	check func(path, src string) CheckResult
}

// NewServer builds a server around one injected check pipeline.
func NewServer(check func(path, src string) CheckResult) *Server {
	return &Server{check: check}
}

// document is one open text document: its current text and the version the
// text was last checked at (the push envelope carries it so the editor never
// paints old-revision diagnostics on new text).
type document struct {
	text    string
	version int
}

// Serve runs one session: read frames, dispatch, answer, push — until exit
// or transport end. It returns the process exit code: 0 when shutdown came
// before exit (or the stream simply ended), nonzero when exit arrived
// without shutdown.
func (s *Server) Serve(r io.Reader, w io.Writer) int {
	reader := bufio.NewReader(r)
	initialized := false
	shutdown := false
	docs := map[string]document{}
	for {
		m, err := Read(reader)
		if err == ErrParse {
			// A malformed frame is answered, not fatal (docs/lsp.md,
			// Lifecycle): id null, -32700, and the session keeps serving.
			writeMessage(w, &Message{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`null`),
				Error:   &Error{Code: -32700, Message: "parse error"},
			})
			continue
		}
		if err != nil {
			// Transport end (EOF or worse): same exit rule as exit proper.
			if shutdown {
				return 0
			}
			return 1
		}
		if m.Method == "exit" {
			// exit always lands, handshake or no handshake.
			if shutdown {
				return 0
			}
			return 1
		}
		if !initialized {
			if m.Method == "initialize" {
				initialized = true
				writeMessage(w, &Message{
					JSONRPC: "2.0",
					ID:      m.ID,
					Result: json.RawMessage(`{"capabilities":{"textDocumentSync":1},` +
						`"serverInfo":{"name":"we"}}`),
				})
				continue
			}
			if isRequest(m) {
				writeMessage(w, &Message{
					JSONRPC: "2.0",
					ID:      m.ID,
					Error:   &Error{Code: -32002, Message: "server not initialized"},
				})
			}
			continue
		}
		switch {
		case m.Method == "initialize" || m.Method == "initialized":
			// The handshake is done; repeats are nothing.
		case m.Method == "shutdown":
			shutdown = true
			writeMessage(w, &Message{
				JSONRPC: "2.0",
				ID:      m.ID,
				Result:  json.RawMessage(`null`),
			})
		case m.Method == "textDocument/didOpen":
			var p struct {
				TextDocument struct {
					URI     string `json:"uri"`
					Version int    `json:"version"`
					Text    string `json:"text"`
				} `json:"textDocument"`
			}
			if decodeParams(m, &p) {
				docs[p.TextDocument.URI] = document{text: p.TextDocument.Text, version: p.TextDocument.Version}
				s.publish(w, p.TextDocument.URI, docs[p.TextDocument.URI])
			}
		case m.Method == "textDocument/didChange":
			var p struct {
				TextDocument struct {
					URI     string `json:"uri"`
					Version int    `json:"version"`
				} `json:"textDocument"`
				ContentChanges []struct {
					Text string `json:"text"`
				} `json:"contentChanges"`
			}
			if decodeParams(m, &p) && len(p.ContentChanges) > 0 {
				d := docs[p.TextDocument.URI]
				// Full sync: the single change's text is the whole document.
				d.text = p.ContentChanges[0].Text
				d.version = p.TextDocument.Version
				docs[p.TextDocument.URI] = d
				s.publish(w, p.TextDocument.URI, d)
			}
		case m.Method == "textDocument/didClose":
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			if decodeParams(m, &p) {
				delete(docs, p.TextDocument.URI)
				// The close push: an empty array, then the document is gone.
				writeDiagnostics(w, p.TextDocument.URI, 0, nil)
			}
		case strings.HasPrefix(m.Method, "$/"):
			// $/ notifications are silently ignored (server-internal names
			// the server has not claimed).
		case isRequest(m):
			writeMessage(w, &Message{
				JSONRPC: "2.0",
				ID:      m.ID,
				Error:   &Error{Code: -32601, Message: "method not found"},
			})
		}
	}
}

// publish checks one document's current text and pushes the mapped
// diagnostics. A pipeline stop at an implementation boundary produces no
// event, so nothing is pushed — the document keeps its last diagnostics
// (docs/lsp.md, Document synchronization).
func (s *Server) publish(w io.Writer, uri string, d document) {
	res := s.check(pathOf(uri), d.text)
	if res.Boundary != "" {
		return
	}
	writeDiagnostics(w, uri, d.version, res.Diagnostics)
}

// writeDiagnostics pushes one textDocument/publishDiagnostics notification:
// the uri, the checked text's version, and every diagnostic mapped per
// docs/lsp.md's mapping table.
func writeDiagnostics(w io.Writer, uri string, version int, ds []diag.Diagnostic) {
	var b strings.Builder
	b.WriteString(`{"uri":`)
	b.WriteString(jsonEncode(uri))
	b.WriteString(`,"version":`)
	b.WriteString(itoa(version))
	b.WriteString(`,"diagnostics":[`)
	for i, d := range ds {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(lspDiagnostic(d))
	}
	b.WriteString(`]}`)
	writeMessage(w, &Message{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  json.RawMessage(b.String()),
	})
}

// lspDiagnostic maps one pipeline diagnostic to the protocol's Diagnostic
// object per the mapping table: severity error/warning/note/help to 1/2/3/4,
// the registry code as-is, the same message string, and the 1-based point
// as a single-character 0-based span. help has no protocol slot and rides
// only on the command-line face.
func lspDiagnostic(d diag.Diagnostic) string {
	var b strings.Builder
	b.WriteString(`{"range":{"start":{"line":`)
	b.WriteString(itoa(d.Line() - 1))
	b.WriteString(`,"character":`)
	b.WriteString(itoa(d.Column() - 1))
	b.WriteString(`},"end":{"line":`)
	b.WriteString(itoa(d.Line() - 1))
	b.WriteString(`,"character":`)
	b.WriteString(itoa(d.Column()))
	b.WriteString(`}},"severity":`)
	b.WriteString(itoa(lspSeverity(d.Severity())))
	b.WriteString(`,"code":`)
	b.WriteString(jsonEncode(d.Code()))
	b.WriteString(`,"source":"we","message":`)
	b.WriteString(jsonEncode(d.Message()))
	b.WriteString(`}`)
	return b.String()
}

// lspSeverity maps the four rendering levels to the protocol's four numeric
// severities: Error, Warning, Information, Hint.
func lspSeverity(s diag.Severity) int {
	switch s {
	case diag.SeverityError:
		return 1
	case diag.SeverityWarning:
		return 2
	case diag.SeverityNote:
		return 3
	}
	return 4 // help — Hint
}

// pathOf turns a file:// URI into the path the check pipeline names files
// by. URIs that are not file:// (or do not parse) pass through untouched —
// the pipeline's own diagnostics then carry what it was given.
func pathOf(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	if p, err := url.PathUnescape(u.EscapedPath()); err == nil {
		return p
	}
	return u.Path
}

// isRequest tells requests (they carry an id and deserve an answer, even an
// error) from notifications (silence is legal) — the base protocol's one
// distinction the dispatcher needs.
func isRequest(m *Message) bool { return len(m.ID) > 0 && string(m.ID) != "null" }

// decodeParams unmarshals a message's params into v; on failure the message
// is dropped — a notification whose params do not decode produces nothing.
func decodeParams(m *Message, v any) bool {
	return json.Unmarshal(m.Params, v) == nil
}

// writeMessage frames one message and ignores write errors: the session's
// read loop is the authority on lifetime, a dead output stream surfaces
// there as a read end.
func writeMessage(w io.Writer, m *Message) {
	_ = Write(w, m)
}

// jsonEncode encodes one string with encoding/json so escaping is exactly
// the standard library's — the same discipline as the pipeline's own JSON
// face.
func jsonEncode(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// itoa keeps strconv behind a one-line name symmetrical with jsonEncode.
func itoa(n int) string {
	return strconv.Itoa(n)
}
