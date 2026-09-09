package cli

import (
	"bytes"
	"strings"
	"testing"
)

// M10c (design D9): the exploration runner's pure faces — the three-flag
// parse (--explore/--iterations N/--no-reduce, test-scoped like --filter),
// the manifest [test] table read with its E1903 gate (shared loadManifest
// face: one validation, four subcommands), the iteration default chain
// (explicit flag > manifest > 100), and the subprocess argv passthrough.
// Written test-first: red today on the faces alone.

func newCliEnv() (*env, *bytes.Buffer, *bytes.Buffer) {
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	return &env{stdout: out, stderr: errb, color: "auto"}, out, errb
}

// The three flags parse under test (both --iterations forms), stay
// unknown under every other subcommand, and reject non-positive or
// non-integer iteration values as usage errors with the pinned message.
func TestM10cParseExploreFlags(t *testing.T) {
	e, _, _ := newCliEnv()
	pos, code := e.parseOptions("test", []string{".", "--explore", "--iterations", "3", "--no-reduce"})
	if code != exitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	if len(pos) != 1 || pos[0] != "." {
		t.Fatalf("positional: want [.], got %v", pos)
	}
	if !e.explore || !e.noReduce || !e.itersSet || e.itersFlag != 3 {
		t.Fatalf("flags not read: explore=%v noReduce=%v iters=%d(set=%v)",
			e.explore, e.noReduce, e.itersFlag, e.itersSet)
	}

	e, _, _ = newCliEnv()
	pos, code = e.parseOptions("test", []string{"--iterations=7", "."})
	if code != exitOK || !e.itersSet || e.itersFlag != 7 || len(pos) != 1 {
		t.Fatalf("= form: code %d iters %d(set %v) pos %v", code, e.itersFlag, e.itersSet, pos)
	}
	// A second flag overwrites the first (last wins, like --color).
	pos, code = e.parseOptions("test", []string{"--iterations", "9"})
	if code != exitOK || e.itersFlag != 9 {
		t.Fatalf("overwrite: code %d iters %d", code, e.itersFlag)
	}

	// Invalid values: non-integer, zero, negative, bare flag without a
	// value — all usage errors with the one message shape.
	for _, args := range [][]string{
		{"--iterations", "abc"},
		{"--iterations=abc"},
		{"--iterations", "0"},
		{"--iterations", "-2"},
		{"--iterations"},
	} {
		e, _, errb := newCliEnv()
		_, code := e.parseOptions("test", args)
		if code != exitUsage {
			t.Fatalf("%v: want usage exit, got %d", args, code)
		}
		msg := errb.String()
		if !strings.HasPrefix(msg, "we: invalid --iterations value") || !strings.Contains(msg, "(want a positive integer)") {
			t.Fatalf("%v: message %q", args, msg)
		}
	}

	// The flags are test-scoped: under check they stay unknown options.
	e, _, errb := newCliEnv()
	if _, code := e.parseOptions("check", []string{"--explore"}); code != exitUsage {
		t.Fatalf("check --explore: want usage exit, got %d", code)
	}
	if !strings.Contains(errb.String(), `unknown option "--explore"`) {
		t.Fatalf("check message: %q", errb.String())
	}
}

// The [test] table read: a legal explore-iterations surfaces from the
// shared loadManifest face; a non-positive or non-integer value is E1903
// (the registry text names this key); an absent table or key is simply
// absent — the default chain's business, not an error.
func TestM10cManifestTestTable(t *testing.T) {
	base := "name = \"demo\"\nversion = \"0.1.0\"\ntype = \"executable\"\n"
	cases := []struct {
		name    string
		toml    string
		want    string
		wantErr string
	}{
		{"legal", "\n[test]\nexplore-iterations = 5\n", "5", ""},
		{"zero", "\n[test]\nexplore-iterations = 0\n", "",
			"we.toml:1:1: error[E1903]: invalid toolchain configuration value — explore-iterations must be a positive integer, got 0\n"},
		{"string", "\n[test]\nexplore-iterations = \"many\"\n", "",
			"we.toml:1:1: error[E1903]: invalid toolchain configuration value — explore-iterations must be a positive integer, got many\n"},
		{"float", "\n[test]\nexplore-iterations = 1.5\n", "",
			"we.toml:1:1: error[E1903]: invalid toolchain configuration value — explore-iterations must be a positive integer, got 1.5\n"},
		{"no table", "", "", ""},
		{"other keys unread", "\n[test]\nexplore-seed = 1\n", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, map[string]string{"we.toml": base + tc.toml})
			chdir(t, dir)
			e, _, errb := newCliEnv()
			m, code := e.loadManifest(".", false)
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
			if got := m["test.explore-iterations"]; got != tc.want {
				t.Fatalf("table value: want %q, got %q", tc.want, got)
			}
		})
	}
}

// The default chain: an explicit --iterations wins over the manifest,
// the manifest over the built-in 100, and an absent everything is 100.
func TestM10cResolveExploreIters(t *testing.T) {
	cases := []struct {
		flagSet  bool
		flag     int
		manifest string
		want     int
	}{
		{true, 3, "5", 3},
		{true, 3, "", 3},
		{false, 0, "5", 5},
		{false, 0, "", 100},
	}
	for _, tc := range cases {
		if got := resolveExploreIters(tc.flagSet, tc.flag, tc.manifest); got != tc.want {
			t.Fatalf("resolve(%v,%d,%q): want %d, got %d", tc.flagSet, tc.flag, tc.manifest, tc.want, got)
		}
	}
}

// The subprocess argv passthrough: --json first (the M10b order), then
// the explore face — --explore and the resolved --iterations ride
// together, --no-reduce when given; a normal run adds nothing.
func TestM10cTestBinaryArgs(t *testing.T) {
	e, _, _ := newCliEnv()
	if got := e.testBinaryArgs(); len(got) != 0 {
		t.Fatalf("plain run: want no args, got %v", got)
	}
	e.json = true
	if got := e.testBinaryArgs(); strings.Join(got, " ") != "--json" {
		t.Fatalf("json only: got %v", got)
	}
	e.explore, e.exploreIters, e.noReduce = true, 5, true
	want := "--json --explore --iterations 5 --no-reduce"
	if got := e.testBinaryArgs(); strings.Join(got, " ") != want {
		t.Fatalf("want %q, got %q", want, strings.Join(got, " "))
	}
}
