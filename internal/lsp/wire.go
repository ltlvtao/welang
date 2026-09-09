// Package lsp implements the We language server (docs/lsp.md, the binding
// document chapter 21 R12 names): the JSON-RPC 2.0 wire over stdio and the
// session machine on top of it. The server's one job is the diagnostics
// push — the checking pipeline is injected, so every diagnostic it sends is
// the pipeline's own and the editor face cannot diverge from the command
// line's verdicts.
package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ErrParse marks a framed message whose body is not JSON (JSON-RPC -32700
// upstream): the session answers it and keeps reading. Every other read
// error is a transport error and ends the session.
var ErrParse = errors.New("lsp: message body is not JSON")

// Message is one JSON-RPC 2.0 message: a request (ID + Method + Params), a
// notification (Method + Params, no ID), or a response (ID + Result or
// Error). ID, Params, and Result ride as raw JSON — the server echoes IDs
// byte-for-byte and never reshapes what it did not read.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is the JSON-RPC error object of a failed response.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Write frames one message onto w: the base protocol header — the body's
// UTF-8 byte count — then the blank line, then the body (docs/lsp.md,
// Transport).
func Write(w io.Writer, m *Message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// Read reads one framed message from r: header lines up to the blank line
// (Content-Length is the one header read, case-sensitively — the binding
// fixes the client's spelling), then exactly that many body bytes. A body
// that does not parse as JSON reads back as ErrParse; a frame without a
// Content-Length header, a short body, or io.EOF is that error itself —
// the session's caller tells the classes apart and acts (Serve: answer,
// end).
func Read(r *bufio.Reader) (*Message, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // the blank line ends the header block
		}
		if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				length = n
			}
		}
	}
	if length < 0 {
		return nil, errors.New("lsp: frame carries no Content-Length header")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	var m Message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, ErrParse
	}
	return &m, nil
}
