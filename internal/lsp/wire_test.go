package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// TestFrameRoundTrip pins the wire's first contract: a message written and
// read back loses no field.
func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	msg := &Message{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"x":1}`),
	}
	if err := Write(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got, err := Read(bufio.NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	if got.JSONRPC != "2.0" || got.Method != "initialize" {
		t.Fatalf("field loss: %+v", got)
	}
	if string(got.ID) != "1" || string(got.Params) != `{"x":1}` {
		t.Fatalf("payload loss: id=%s params=%s", got.ID, got.Params)
	}
}

// TestFrameHeaderExact pins the base protocol envelope: the header names the
// body's exact byte count, and the body is what follows the blank line.
func TestFrameHeaderExact(t *testing.T) {
	var buf bytes.Buffer
	body := `{"jsonrpc":"2.0","id":7,"method":"shutdown"}`
	if err := Write(&buf, &Message{JSONRPC: "2.0", ID: json.RawMessage(`7`), Method: "shutdown"}); err != nil {
		t.Fatal(err)
	}
	want := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	if buf.String() != want {
		t.Fatalf("frame mismatch:\n got %q\nwant %q", buf.String(), want)
	}
}

// TestNotificationOmitsID pins the notification shape: no id field at all
// (omitempty, not null).
func TestNotificationOmitsID(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, &Message{JSONRPC: "2.0", Method: "initialized"}); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	if i := strings.Index(body, "\r\n\r\n"); i < 0 {
		t.Fatalf("no header terminator: %q", body)
	} else {
		body = body[i+4:]
	}
	if strings.Contains(body, `"id"`) {
		t.Fatalf("notification carries id: %s", body)
	}
}

// TestReadMalformedJSON pins the parse-error class: a well-framed message
// whose body is not JSON reads back as ErrParse, distinct from transport
// errors.
func TestReadMalformedJSON(t *testing.T) {
	raw := "Content-Length: 4\r\n\r\nnot{"
	_, err := Read(bufio.NewReader(strings.NewReader(raw)))
	if err != ErrParse {
		t.Fatalf("want ErrParse, got %v", err)
	}
}

// TestReadBadHeader pins the header contract: no Content-Length header is a
// transport error, not a parse error.
func TestReadBadHeader(t *testing.T) {
	_, err := Read(bufio.NewReader(strings.NewReader("garbage\r\n\r\n")))
	if err == nil || err == ErrParse {
		t.Fatalf("want non-parse error, got %v", err)
	}
}

// TestErrorResponseShape pins the error result's encoding: code and message
// under an error key, no result beside it.
func TestErrorResponseShape(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, &Message{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Error:   &Error{Code: -32601, Message: "method not found"},
	}); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, `"error":{"code":-32601,"message":"method not found"}`) {
		t.Fatalf("error shape mismatch: %s", s)
	}
	if strings.Contains(s, `"result"`) {
		t.Fatalf("error response carries result: %s", s)
	}
}

// TestStringIDs pins id transparency: string ids (LSP clients send them)
// round-trip byte-for-byte, the server never interprets one.
func TestStringIDs(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, &Message{JSONRPC: "2.0", ID: json.RawMessage(`"abc-1"`), Method: "foo"}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(bufio.NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	if string(got.ID) != `"abc-1"` {
		t.Fatalf("string id lost: %s", got.ID)
	}
}
