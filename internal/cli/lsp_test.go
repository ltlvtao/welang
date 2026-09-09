package cli

// The LSP face's tests drive the real we binary as a subprocess (decision
// Q4): the protocol owns stdin/stdout, so in-process Run cannot host a
// session and an injected-stream double would test a different thing than
// the editors run. TestMain builds the binary once and every test reuses
// it; the conformance golden corpus stays untouched — `we lsp` has no
// golden cases by design (its face is pinned here, against check itself).

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// lspBin is the once-built binary TestMain leaves for the session tests.
var lspBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "we-lsp-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	bin := filepath.Join(dir, "we")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/ltlvtao/welang/cmd/we").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s", err, out)
		os.Exit(1)
	}
	lspBin = bin
	os.Exit(m.Run())
}

// lspClient is one real `we lsp` session over the subprocess's pipes.
type lspClient struct {
	t   *testing.T
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

// startLsp launches one server process in dir.
func startLsp(t *testing.T, dir string) *lspClient {
	t.Helper()
	cmd := exec.Command(lspBin, "lsp")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr // boundary lines stay visible when a test fails
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := &lspClient{t: t, cmd: cmd, in: in, out: bufio.NewReader(out)}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	return c
}

// send writes one framed message from raw JSON.
func (c *lspClient) send(raw string) {
	c.t.Helper()
	body := []byte(raw)
	if _, err := fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		c.t.Fatal(err)
	}
	if _, err := c.in.Write(body); err != nil {
		c.t.Fatal(err)
	}
}

// recv reads one framed message as a decoded object, with a deadline — a
// server that never answers must fail the test, not hang it.
func (c *lspClient) recv() map[string]any {
	c.t.Helper()
	ch := make(chan map[string]any, 1)
	go func() {
		length := -1
		for {
			line, err := c.out.ReadString('\n')
			if err != nil {
				ch <- nil
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					length = n
				}
			}
		}
		if length < 0 {
			ch <- nil
			return
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(c.out, body); err != nil {
			ch <- nil
			return
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			ch <- nil
			return
		}
		ch <- m
	}()
	select {
	case m := <-ch:
		if m == nil {
			c.t.Fatal("no frame came back")
		}
		return m
	case <-time.After(30 * time.Second):
		c.t.Fatal("timed out waiting for a frame")
		return nil
	}
}

// req sends a request and reads its response.
func (c *lspClient) req(id, method, params string) map[string]any {
	c.t.Helper()
	c.send(`{"jsonrpc":"2.0","id":` + id + `,"method":"` + method + `","params":` + params + `}`)
	return c.recv()
}

// initialize runs the handshake and returns the capabilities result.
func (c *lspClient) initialize() map[string]any {
	c.t.Helper()
	res := c.req("1", "initialize", `{"rootUri":null,"capabilities":{}}`)
	if res["error"] != nil {
		c.t.Fatalf("initialize failed: %v", res["error"])
	}
	c.send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
	return res["result"].(map[string]any)
}

// sendOpen delivers a document's first text without reading back — the
// boundary face produces no push, so the read belongs to the caller.
func (c *lspClient) sendOpen(uri, text string) {
	c.t.Helper()
	c.send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":` +
		jsonString(uri) + `,"languageId":"we","version":1,"text":` + jsonString(text) + `}}}`)
}

// open publishes a document's first text and returns the push.
func (c *lspClient) open(uri, text string) map[string]any {
	c.t.Helper()
	c.sendOpen(uri, text)
	return c.recv()["params"].(map[string]any)
}

// jsonString is the tests' one string encoder (escaping is the standard
// library's).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// TestLspLifecycle pins the whole documented sequence against the real
// binary: handshake, dirty open, clean change, close, shutdown, exit —
// and the process exit code 0.
func TestLspLifecycle(t *testing.T) {
	dir := t.TempDir()
	c := startLsp(t, dir)
	res := c.initialize()
	caps := res["capabilities"].(map[string]any)
	if caps["textDocumentSync"] != float64(1) {
		t.Fatalf("textDocumentSync = %v, want 1 (full)", caps["textDocumentSync"])
	}
	if res["serverInfo"].(map[string]any)["name"] != "we" {
		t.Fatalf("serverInfo.name = %v", res["serverInfo"])
	}
	p := c.open("file://"+filepath.Join(dir, "a.we"), "let x: Int64 = \"s\"\n")
	if p["uri"] != "file://"+filepath.Join(dir, "a.we") || p["version"] != float64(1) {
		t.Fatalf("push envelope: %v", p)
	}
	if ds := p["diagnostics"].([]any); len(ds) != 1 {
		t.Fatalf("want 1 diagnostic, got %d", len(ds))
	}
	c.send(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":` +
		jsonString("file://"+filepath.Join(dir, "a.we")) + `,"version":2},"contentChanges":[{"text":"let x: Int64 = 1\n"}]}}`)
	if p = c.recv()["params"].(map[string]any); len(p["diagnostics"].([]any)) != 0 {
		t.Fatalf("want clean push, got %v", p)
	}
	c.send(`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":` +
		jsonString("file://"+filepath.Join(dir, "a.we")) + `}}}`)
	if p = c.recv()["params"].(map[string]any); len(p["diagnostics"].([]any)) != 0 {
		t.Fatalf("want close push, got %v", p)
	}
	m := c.req("9", "shutdown", `{}`)
	if m["result"] != nil {
		t.Fatalf("shutdown result: %v", m["result"])
	}
	c.send(`{"jsonrpc":"2.0","method":"exit"}`)
	if err := c.cmd.Wait(); err != nil {
		t.Fatalf("shutdown-then-exit: %v", err)
	}
}

// TestLspExitWithoutShutdown pins the exit-code face on the real process:
// exit before shutdown is a nonzero exit.
func TestLspExitWithoutShutdown(t *testing.T) {
	dir := t.TempDir()
	c := startLsp(t, dir)
	c.initialize()
	c.send(`{"jsonrpc":"2.0","method":"exit"}`)
	if err := c.cmd.Wait(); err == nil {
		t.Fatal("exit without shutdown must exit nonzero")
	}
}

// The parity corpus: one error face, one advisory face, one boundary face.
// The same text runs through `we check --json` and through the server's
// push; the two must agree per docs/lsp.md's mapping table, field by field.
var parityCorpus = []struct {
	name string
	text string
}{
	{
		name: "error-e0501",
		text: "let x: Int64 = \"s\"\n",
	},
	{
		name: "advisory-w1912",
		text: "import std.concurrent as conc\n\nfn work() {\n    let sem = conc.Semaphore(1)\n    sem.acquire()\n}\n",
	},
	{
		name: "boundary-composites",
		text: "let (a, b) = (1, 2)\n",
	},
}

// TestLspMatchesCheck is the binding's one promise as a test: for the same
// source text, the CLI's JSON event and the server's push carry the same
// code, message, and severity (mapped), at the same position (mapped) —
// and a boundary form, where check prints its line and exits 70 with no
// event, produces no push at all.
func TestLspMatchesCheck(t *testing.T) {
	for _, row := range parityCorpus {
		t.Run(row.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "probe.we")
			if err := os.WriteFile(file, []byte(row.text), 0o644); err != nil {
				t.Fatal(err)
			}
			// The command-line face: `we check probe.we --json`.
			cmd := exec.Command(lspBin, "check", file, "--json")
			cmd.Dir = dir
			var stdout strings.Builder
			cmd.Stdout = &stdout
			checkExit := cmd.Run()
			var events []map[string]any
			for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
				if line == "" {
					continue
				}
				var ev map[string]any
				if err := json.Unmarshal([]byte(line), &ev); err != nil {
					t.Fatalf("check event not JSON: %q", line)
				}
				events = append(events, ev)
			}
			// The editor face: the same text through didOpen.
			c := startLsp(t, dir)
			c.initialize()
			c.sendOpen("file://"+file, row.text)

			if row.name == "boundary-composites" {
				// Check: exit 70, no events; the server: no push — the
				// shutdown response is the next frame after didOpen.
				if checkExit == nil {
					t.Fatal("boundary form must exit nonzero (70)")
				}
				if len(events) != 0 {
					t.Fatalf("boundary form emitted events: %v", events)
				}
				m := c.req("7", "shutdown", `{}`)
				if m["result"] != nil {
					t.Fatalf("boundary must not push; got %v", m)
				}
				return
			}

			// Both faces report exactly the finding(s) check's pipeline
			// stopped at; map each event per the table and compare.
			p := c.recv()["params"].(map[string]any)
			ds := p["diagnostics"].([]any)
			if len(ds) != len(events) {
				t.Fatalf("check reported %d event(s), server pushed %d: %v vs %v", len(events), len(ds), events, ds)
			}
			for i, ev := range events {
				d := ds[i].(map[string]any)
				if d["code"] != ev["code"] {
					t.Errorf("code: check %v, lsp %v", ev["code"], d["code"])
				}
				if d["message"] != ev["message"] {
					t.Errorf("message: check %v, lsp %v", ev["message"], d["message"])
				}
				wantSeverity := map[string]float64{"error": 1, "warning": 2, "note": 3, "help": 4}[ev["severity"].(string)]
				if d["severity"] != wantSeverity {
					t.Errorf("severity: check %v -> lsp %v, want %v", ev["severity"], d["severity"], wantSeverity)
				}
				// The 1-based point becomes a single-character 0-based span.
				wantStart := map[string]any{
					"line":      ev["line"].(float64) - 1,
					"character": ev["column"].(float64) - 1,
				}
				wantEnd := map[string]any{
					"line":      ev["line"].(float64) - 1,
					"character": ev["column"].(float64),
				}
				r := d["range"].(map[string]any)
				if !mapEqual(r["start"].(map[string]any), wantStart) || !mapEqual(r["end"].(map[string]any), wantEnd) {
					t.Errorf("range: check line %v col %v, lsp range %v", ev["line"], ev["column"], r)
				}
				if d["source"] != "we" {
					t.Errorf("source: %v", d["source"])
				}
			}
		})
	}
}

// mapEqual compares two decoded JSON objects key by key.
func mapEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range b {
		if a[k] != v {
			return false
		}
	}
	return true
}
