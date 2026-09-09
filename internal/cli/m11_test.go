package cli

import (
	"strings"
	"testing"
)

// M11: we doc's two flags (design D6) — --output takes a directory in both
// forms, --check is a bare boolean; both are doc-scoped like we test's
// exploration trio, staying unknown options elsewhere.

func TestM11ParseDocFlags(t *testing.T) {
	e, _, _ := newCliEnv()
	pos, code := e.parseOptions("doc", []string{".", "--output", "site", "--check"})
	if code != exitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	if len(pos) != 1 || pos[0] != "." {
		t.Fatalf("positional: want [.], got %v", pos)
	}
	if e.docOutput != "site" || !e.docCheck {
		t.Fatalf("flags not read: output=%q check=%v", e.docOutput, e.docCheck)
	}

	e, _, _ = newCliEnv()
	pos, code = e.parseOptions("doc", []string{"--output=pages", "."})
	if code != exitOK || e.docOutput != "pages" || len(pos) != 1 {
		t.Fatalf("= form: code %d output %q pos %v", code, e.docOutput, pos)
	}

	// Last wins, like --color and --iterations.
	e, _, _ = newCliEnv()
	if _, code := e.parseOptions("doc", []string{"--output", "a", "--output", "b"}); code != exitOK || e.docOutput != "b" {
		t.Fatalf("overwrite: code %d output %q", code, e.docOutput)
	}

	// A missing value is a usage error with the one message shape.
	e, _, errb := newCliEnv()
	if _, code := e.parseOptions("doc", []string{"--output"}); code != exitUsage {
		t.Fatalf("missing value: want usage exit, got %d", code)
	}
	if !strings.HasPrefix(errb.String(), "we: --output wants a directory") {
		t.Fatalf("missing value message: %q", errb.String())
	}

	// The flags are doc-scoped: under check they stay unknown options.
	e, _, errb = newCliEnv()
	if _, code := e.parseOptions("check", []string{"--check"}); code != exitUsage {
		t.Fatalf("check --check: want usage exit, got %d", code)
	}
	if !strings.Contains(errb.String(), `unknown option "--check"`) {
		t.Fatalf("check message: %q", errb.String())
	}
}

// The [vet] table (design D5): three keys, three legal values, E1903 with
// the registry's message shape for anything else, keys surfacing with the
// section prefix like the [test] table before them.
func TestM11ManifestVetTable(t *testing.T) {
	base := "name = \"demo\"\nversion = \"0.1.0\"\ntype = \"executable\"\n"
	cases := []struct {
		name    string
		toml    string
		want    string
		wantErr string
	}{
		{"warning", "\n[vet]\nW1910 = \"warning\"\n", "warning", ""},
		{"error", "\n[vet]\nW1911 = \"error\"\n", "error", ""},
		{"ignore", "\n[vet]\nW1912 = \"ignore\"\n", "ignore", ""},
		{"all three", "\n[vet]\nW1910 = \"warning\"\nW1911 = \"error\"\nW1912 = \"ignore\"\n", "error", ""},
		{"strict rejected", "\n[vet]\nW1912 = \"strict\"\n", "",
			"we.toml:1:1: error[E1903]: invalid toolchain configuration value — vet.W1912 must be \"warning\", \"error\", or \"ignore\", got strict\n"},
		{"unquoted rejected", "\n[vet]\nW1910 = error\n", "",
			"we.toml:1:1: error[E1903]: invalid toolchain configuration value — vet.W1910 must be \"warning\", \"error\", or \"ignore\", got error\n"},
		{"no table", "", "", ""},
		{"other keys unread", "\n[vet]\nW1999 = \"warning\"\n", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, map[string]string{"we.toml": base + tc.toml})
			chdir(t, dir)
			e, _, errb := newCliEnv()
			m, _, code := e.loadManifest(".", false)
			if tc.wantErr != "" {
				if code != exitDiagnostic {
					t.Fatalf("want diagnostic exit, got %d", code)
				}
				if errb.String() != tc.wantErr {
					t.Fatalf("message:\nwant %q\ngot  %q", tc.wantErr, errb.String())
				}
				return
			}
			if code != exitOK {
				t.Fatalf("want exit 0, got %d (%s)", code, errb.String())
			}
			if tc.want == "" {
				return
			}
			key := "vet.W1911"
			if tc.name == "warning" {
				key = "vet.W1910"
			} else if tc.name == "ignore" || tc.name == "strict rejected" {
				key = "vet.W1912"
			}
			if got := m[key]; got != tc.want {
				t.Fatalf("table value %s: want %q, got %q", key, tc.want, got)
			}
		})
	}
}

// vetPosture maps a manifest's [vet] entry to its posture; an absent key or
// table is the default warning (the registry's default severity).
func TestM11VetPosture(t *testing.T) {
	if got := vetPosture(map[string]string{"vet.W1910": "error"}, "W1910"); got != "error" {
		t.Fatalf("error: got %q", got)
	}
	if got := vetPosture(map[string]string{"vet.W1910": "ignore"}, "W1910"); got != "ignore" {
		t.Fatalf("ignore: got %q", got)
	}
	if got := vetPosture(map[string]string{"vet.W1911": "warning"}, "W1910"); got != "warning" {
		t.Fatalf("other key: got %q", got)
	}
	if got := vetPosture(nil, "W1912"); got != "warning" {
		t.Fatalf("nil manifest: got %q", got)
	}
}

// collectWeFiles is we fmt's project face (design D2, adjudication Q2):
// every .we under the directory except the build/ subtree, lexically.
func TestM11CollectWeFiles(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"we.toml":         "name = \"demo\"\n",
		"src/main.we":     "pub fn main() {\n    return\n}\n",
		"src/sub/a.we":    "pub fn a() {\n    return\n}\n",
		"tests/x_test.we": "test \"x\" {\n    assert(true, \"ok\")\n}\n",
		"loose.we":        "pub fn loose() {\n    return\n}\n",
		"docs/readme.md":  "not a we file\n",
		"build/main.we":   "pub fn built() {\n    return\n}\n",
		"build/gen/b.we":  "pub fn b() {\n    return\n}\n",
	})
	got, err := collectWeFiles(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	want := []string{"loose.we", "src/main.we", "src/sub/a.we", "tests/x_test.we"}
	if len(got) != len(want) {
		t.Fatalf("files: want %v, got %v", want, got)
	}
	for i := range want {
		if !strings.HasSuffix(got[i], want[i]) {
			t.Fatalf("file %d: want suffix %q, got %q", i, want[i], got[i])
		}
	}
}
