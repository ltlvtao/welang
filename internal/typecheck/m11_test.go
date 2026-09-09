package typecheck

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/diag"
	"github.com/ltlvtao/welang/internal/parser"
)

// M11 design D4: the advisory collector hangs on the checker's own walk —
// binding types, call-target effect sets, and mock identity are the
// checker's facts, never re-derived here. Advisories returns W-severity
// diagnostics only; the pipeline renders and gates them (D5).

func advisoriesOf(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	// The file name ends _test.we: the parser's module-identity gate
	// (E1801) holds test blocks to that suffix, so the fixtures that
	// carry one must parse under a test-module name.
	f, d, ni := parser.Parse("m11_test.we", []byte(src))
	if d != nil {
		t.Fatalf("parse diagnostic: %s", d.Human())
	}
	if ni != nil {
		t.Fatalf("parse boundary: %s", ni.What)
	}
	return Advisories(f, "m11_test.we", SingleFile)
}

// wantAdvisory pins one finding: the code and the message part identify
// the advisory among any same-code siblings (W1912's five arrive in one
// list), then the position and severity assert on that one.
func wantAdvisory(t *testing.T, src, code, part string, line, col int) {
	t.Helper()
	for _, a := range advisoriesOf(t, src) {
		if a.Code() == code && strings.Contains(a.Message(), part) {
			pos := `"line":` + itoa(line) + `,"column":` + itoa(col)
			if !strings.Contains(a.JSON(), pos) {
				t.Fatalf("%s position: want %d:%d, got %s", code, line, col, a.JSON())
			}
			if !strings.Contains(a.JSON(), `"severity":"warning"`) {
				t.Fatalf("%s severity: %s", code, a.JSON())
			}
			return
		}
	}
	t.Fatalf("advisory %s (%s) not fired for %q", code, part, src)
}

func wantNoAdvisory(t *testing.T, src, code string) {
	t.Helper()
	for _, a := range advisoriesOf(t, src) {
		if a.Code() == code {
			t.Fatalf("advisory %s fired unexpectedly: %s", code, a.Human())
		}
	}
}

const m11Head = "import std.concurrent as conc\n\npub type AppError = Failed(String)\n\n"

// W1910: a test body calls a custom-effect fn with no matching mock in the
// same test declaration; a mock anywhere in the test covers the whole block
// (mocks install at block start — chapter 20's runtime fact).
func TestM11W1910(t *testing.T) {
	fires := "effect db\n\nfn save() effect db {\n    return\n}\n\ntest \"gate\" {\n    save()\n    assert(true, \"done\")\n}\n"
	wantAdvisory(t, fires, "W1910", "save carries custom effect db", 8, 5)

	mocked := "effect db\n\nfn save() effect db {\n    return\n}\n\ntest \"gate\" {\n    mock save() effect db {\n        return\n    }\n    save()\n    assert(true, \"done\")\n}\n"
	wantNoAdvisory(t, mocked, "W1910")

	// A call outside any test is not W1910's face.
	outside := "effect db\n\nfn save() effect db {\n    return\n}\n\npub fn main() -> Result<(), AppError> {\n    save()\n    return Ok(())\n}\n\npub type AppError = Failed(String)\n"
	wantNoAdvisory(t, outside, "W1910")
}

// W1911: inside a shared cell's own callback the binding is passed as an
// argument to some call — one call through, conservative by design. The
// direct same-binding access is E1613's face, not this advisory's.
func TestM11W1911(t *testing.T) {
	src := m11Head + "fn touch(h: conc.Mutex<Int64>, v: Int64) -> Int64 {\n    let out = h.get()\n    return v + out\n}\n\npub fn main() -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    let r = m.update(|v| touch(m, v))\n    let _ = r\n    return Ok(())\n}\n"
	wantAdvisory(t, src, "W1911", "m is passed to touch", 12, 26)

	// A different binding passed through is free.
	other := m11Head + "fn touch(h: conc.Mutex<Int64>, v: Int64) -> Int64 {\n    let out = h.get()\n    return v + out\n}\n\npub fn main() -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    let n = conc.Mutex(9)\n    let r = m.update(|v| touch(n, v))\n    let _ = r\n    return Ok(())\n}\n"
	wantNoAdvisory(t, other, "W1911")

	// The direct nested access is E1613's own face — no W1911 on top.
	direct := m11Head + "pub fn main() -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    let x = m.update(|v| m.get() + 1)\n    let _ = x\n    return Ok(())\n}\n"
	wantNoAdvisory(t, direct, "W1911")
}

// W1912: a body directly calls one of the five blocking operations with the
// receiver's named type exactly the corresponding primitive; trySend and
// tryReceive are non-blocking and never fire it.
func TestM11W1912(t *testing.T) {
	src := m11Head + "pub fn main() effect io -> Result<(), AppError> {\n    let m = conc.Mutex(0)\n    let cv = conc.Cond(m)\n    let sem = conc.Semaphore(1)\n    let ch: conc.Channel<Int64> = conc.channel(2)\n    scope {\n        let t = task effect io {\n            let _ = m.set(1)\n            let _ = sem.release()\n            let _ = ch.close()\n        }\n        let _ = t.await()\n    }\n    sem.acquire()\n    cv.wait(|v| v > 0)\n    ch.send(1)\n    let a = ch.receive()\n    let _ = a\n    let b = ch.trySend(2)\n    let c = ch.tryReceive()\n    let _ = b\n    let _ = c\n    return Ok(())\n}\n"
	for _, tc := range []struct {
		part string
		line int
		col  int
	}{
		{"TaskHandle.await may block", 16, 19},
		{"Semaphore.acquire may block", 18, 9},
		{"Cond.wait may block", 19, 8},
		{"Channel.send may block", 20, 8},
		{"Channel.receive may block", 21, 16},
	} {
		wantAdvisory(t, src, "W1912", tc.part, tc.line, tc.col)
	}
	// trySend/tryReceive in the same body fire nothing (checked by the
	// wantAdvisory calls above finding exactly the five; assert the total).
	all := advisoriesOf(t, src)
	n := 0
	for _, a := range all {
		if a.Code() == "W1912" {
			n++
		}
	}
	if n != 5 {
		t.Fatalf("want exactly 5 W1912 findings, got %d", n)
	}
}
