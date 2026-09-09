package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/diag"
)

// harness wires one server to an in-test client over two pipes and runs
// Serve in the background — the session machine's whole face is reachable
// through send/recv alone.
type harness struct {
	t      *testing.T
	cw     io.WriteCloser // the client writes requests here
	cr     *bufio.Reader  // ... and reads responses here
	done   chan int       // Serve's exit code, reaped once
	code   int
	reaped bool
}

func startSession(t *testing.T, check func(path, src string) CheckResult) *harness {
	t.Helper()
	// client -> server: the client writes cw, the server reads stdin.
	stdin, cw := io.Pipe()
	// server -> client: the server writes stdout, the client reads here.
	cr, stdout := io.Pipe()
	srv := NewServer(check)
	done := make(chan int, 1)
	go func() { done <- srv.Serve(stdin, stdout) }()
	h := &harness{t: t, cw: cw, cr: bufio.NewReader(cr), done: done}
	t.Cleanup(func() {
		cw.Close()     // ends the server's read loop if it still runs
		stdout.Close() // ... and unblocks any pending write
		h.exitCode()   // wait for Serve; a no-op when the test already reaped
	})
	return h
}

// exitCode returns Serve's exit code, reading the channel the first time
// only — the cleanup path and the test body share the one send.
func (h *harness) exitCode() int {
	h.t.Helper()
	if !h.reaped {
		h.code = <-h.done
		h.reaped = true
	}
	return h.code
}

// send writes one framed message from raw JSON.
func (h *harness) send(raw string) {
	h.t.Helper()
	if _, err := h.cw.Write([]byte("Content-Length: " + strconv.Itoa(len(raw)) + "\r\n\r\n" + raw)); err != nil {
		h.t.Fatal(err)
	}
}

// recv reads one framed message.
func (h *harness) recv() *Message {
	h.t.Helper()
	m, err := Read(h.cr)
	if err != nil {
		h.t.Fatal(err)
	}
	return m
}

// req sends a request and returns the next message back (responses come in
// order on this synchronous face).
func (h *harness) req(id, method, params string) *Message {
	h.t.Helper()
	h.send(`{"jsonrpc":"2.0","id":` + id + `,"method":"` + method + `","params":` + params + `}`)
	return h.recv()
}

// initialize runs the handshake and returns the capabilities result.
func (h *harness) initialize() map[string]any {
	h.t.Helper()
	m := h.req("1", "initialize", `{"rootUri":null,"capabilities":{}}`)
	if m.Error != nil {
		h.t.Fatalf("initialize failed: %+v", m.Error)
	}
	var res map[string]any
	if err := json.Unmarshal(m.Result, &res); err != nil {
		h.t.Fatalf("initialize result: %v", err)
	}
	h.send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
	return res
}

// publish reads the next notification as publishDiagnostics params.
func (h *harness) publish() map[string]any {
	h.t.Helper()
	m := h.recv()
	if m.Method != "textDocument/publishDiagnostics" {
		h.t.Fatalf("want publishDiagnostics, got method %q", m.Method)
	}
	var p map[string]any
	if err := json.Unmarshal(m.Params, &p); err != nil {
		h.t.Fatal(err)
	}
	return p
}

// --- the session face -------------------------------------------------------

// TestSessionFullLifecycle walks the whole documented sequence: handshake,
// open (dirty), change (clean), close, shutdown, exit — and the exit code
// is 0 when shutdown came first.
func TestSessionFullLifecycle(t *testing.T) {
	checks := 0
	h := startSession(t, func(path, src string) CheckResult {
		checks++
		if strings.Contains(src, "!!!") {
			return CheckResult{Diagnostics: []diag.Diagnostic{
				diag.Error("E0501", "type mismatch — operands of different types").At(path, 1, 9),
			}}
		}
		return CheckResult{}
	})
	res := h.initialize()
	caps := res["capabilities"].(map[string]any)
	if caps["textDocumentSync"] != float64(1) {
		t.Fatalf("textDocumentSync = %v, want 1 (full)", caps["textDocumentSync"])
	}
	info := res["serverInfo"].(map[string]any)
	if info["name"] != "we" {
		t.Fatalf("serverInfo.name = %v", info["name"])
	}
	h.send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///p/a.we","languageId":"we","version":1,"text":"let x = !!!"}}}`)
	p := h.publish()
	if p["uri"] != "file:///p/a.we" || p["version"] != float64(1) {
		t.Fatalf("publish envelope: %v", p)
	}
	if ds := p["diagnostics"].([]any); len(ds) != 1 {
		t.Fatalf("want 1 diagnostic, got %d", len(ds))
	}
	h.send(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"file:///p/a.we","version":2},"contentChanges":[{"text":"let x = 1"}]}}`)
	if p = h.publish(); len(p["diagnostics"].([]any)) != 0 {
		t.Fatalf("want clean push, got %v", p)
	}
	h.send(`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":"file:///p/a.we"}}}`)
	if p = h.publish(); len(p["diagnostics"].([]any)) != 0 {
		t.Fatalf("want close push, got %v", p)
	}
	m := h.req("9", "shutdown", `{}`)
	if m.Error != nil || string(m.Result) != "null" {
		t.Fatalf("shutdown response: %+v", m)
	}
	h.send(`{"jsonrpc":"2.0","method":"exit"}`)
	if code := h.exitCode(); code != 0 {
		t.Fatalf("shutdown-then-exit code = %d, want 0", code)
	}
	if checks != 2 {
		t.Fatalf("check ran %d times, want 2 (open + change)", checks)
	}
}

// TestSessionDiagnosticMapping pins the mapping table field by field: the
// 1-based point becomes a single-character 0-based span, severity maps one
// to one, the code rides as-is, the message is the same string, and source
// is constant "we".
func TestSessionDiagnosticMapping(t *testing.T) {
	h := startSession(t, func(path, src string) CheckResult {
		return CheckResult{Diagnostics: []diag.Diagnostic{
			diag.Error("E0501", "type mismatch — operands of different types").At(path, 3, 8),
			diag.Warning("W1912", "may-block note — this operation can block the task").At(path, 4, 2),
		}}
	})
	h.initialize()
	h.send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///p/b.we","languageId":"we","version":1,"text":"x"}}}`)
	ds := h.publish()["diagnostics"].([]any)
	if len(ds) != 2 {
		t.Fatalf("want 2 diagnostics, got %d", len(ds))
	}
	e := ds[0].(map[string]any)
	r := e["range"].(map[string]any)
	start := r["start"].(map[string]any)
	end := r["end"].(map[string]any)
	if start["line"] != float64(2) || start["character"] != float64(7) || end["line"] != float64(2) || end["character"] != float64(8) {
		t.Fatalf("range mapping: %v", r)
	}
	if e["severity"] != float64(1) || e["code"] != "E0501" || e["source"] != "we" {
		t.Fatalf("error fields: %v", e)
	}
	if e["message"] != "type mismatch — operands of different types" {
		t.Fatalf("message: %v", e["message"])
	}
	w := ds[1].(map[string]any)
	if w["severity"] != float64(2) || w["code"] != "W1912" {
		t.Fatalf("warning fields: %v", w)
	}
}

// TestSessionRequestBeforeInitialize pins -32002: nothing is served before
// initialize, save exit.
func TestSessionRequestBeforeInitialize(t *testing.T) {
	h := startSession(t, func(path, src string) CheckResult { return CheckResult{} })
	m := h.req("1", "textDocument/hover", `{"textDocument":{"uri":"file:///p/a.we"},"position":{"line":0,"character":0}}`)
	if m.Error == nil || m.Error.Code != -32002 {
		t.Fatalf("want -32002, got %+v", m)
	}
}

// TestSessionUnknownMethod pins -32601 for unknown non-$/ requests.
func TestSessionUnknownMethod(t *testing.T) {
	h := startSession(t, func(path, src string) CheckResult { return CheckResult{} })
	h.initialize()
	m := h.req("2", "workspace/symbol", `{}`)
	if m.Error == nil || m.Error.Code != -32601 {
		t.Fatalf("want -32601, got %+v", m)
	}
}

// TestSessionDollarNotificationIgnored pins $/ silence: the notification
// produces no output — the next response proves the stream stayed clean.
func TestSessionDollarNotificationIgnored(t *testing.T) {
	h := startSession(t, func(path, src string) CheckResult { return CheckResult{} })
	h.initialize()
	h.send(`{"jsonrpc":"2.0","method":"$/progress","params":{}}`)
	m := h.req("3", "shutdown", `{}`)
	if m.Error != nil {
		t.Fatalf("$/ noise broke the stream: %+v", m)
	}
}

// TestSessionParseErrorAnswered pins -32700: a malformed frame is answered
// (id null) and the session keeps serving.
func TestSessionParseErrorAnswered(t *testing.T) {
	h := startSession(t, func(path, src string) CheckResult { return CheckResult{} })
	h.send(`not json at all`)
	m := h.recv()
	if m.Error == nil || m.Error.Code != -32700 || string(m.ID) != "null" {
		t.Fatalf("want -32700 with id null, got %+v", m)
	}
	if res := h.initialize(); res["capabilities"] == nil {
		t.Fatal("session did not survive the parse error")
	}
}

// TestSessionBoundarySilence pins the boundary face: when the pipeline
// stops at an implementation boundary there is no event, so no push — the
// shutdown response is the first outbound message after didOpen.
func TestSessionBoundarySilence(t *testing.T) {
	h := startSession(t, func(path, src string) CheckResult { return CheckResult{Boundary: "main bodies beyond let bindings"} })
	h.initialize()
	h.send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///p/c.we","languageId":"we","version":1,"text":"x"}}}`)
	m := h.req("5", "shutdown", `{}`)
	if m.Error != nil || m.Method != "" {
		t.Fatalf("boundary must not push: first outbound was %+v", m)
	}
}

// TestSessionExitWithoutShutdown pins the exit code: exit before shutdown
// is nonzero (the base protocol's face).
func TestSessionExitWithoutShutdown(t *testing.T) {
	stdin, cw := io.Pipe()
	cr, stdout := io.Pipe()
	srv := NewServer(func(path, src string) CheckResult { return CheckResult{} })
	done := make(chan int, 1)
	go func() { done <- srv.Serve(stdin, stdout) }()
	res := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	cw.Write([]byte("Content-Length: " + strconv.Itoa(len(res)) + "\r\n\r\n" + res))
	Read(bufio.NewReader(cr)) // drain the initialize response
	ex := `{"jsonrpc":"2.0","method":"exit"}`
	cw.Write([]byte("Content-Length: " + strconv.Itoa(len(ex)) + "\r\n\r\n" + ex))
	if code := <-done; code == 0 {
		t.Fatal("exit without shutdown must be nonzero")
	}
	cw.Close()
	stdout.Close()
}
