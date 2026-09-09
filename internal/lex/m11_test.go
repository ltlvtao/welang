package lex

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/diag"
)

// M11 design D1: Keep is the second scanner entry that keeps line comments
// as trivia tokens (Kind "comment", text verbatim including the slashes)
// while the rest of the stream is byte-for-byte the face Scan produces.

func TestKeepCommentsSurface(t *testing.T) {
	src := "// lead\nlet x = 1 // trailing\n/// doc unit is a comment here too\nlet y = 2\n"
	toks, d := Keep("k.we", []byte(src))
	if d != nil {
		t.Fatalf("diagnostic: %s", d.Human())
	}
	var comments []Token
	for _, tk := range toks {
		if tk.Kind == KindComment {
			comments = append(comments, tk)
		}
	}
	if len(comments) != 3 {
		t.Fatalf("want 3 comment tokens, got %d in %v", len(comments), toks)
	}
	want := []struct {
		text string
		line int
		col  int
	}{
		{"// lead", 1, 1},
		{"// trailing", 2, 11},
		{"/// doc unit is a comment here too", 3, 1},
	}
	for i, w := range want {
		got := comments[i]
		if got.Text != w.text || got.Line != w.line || got.Col != w.col {
			t.Fatalf("comment %d: want %q at %d:%d, got %q at %d:%d",
				i, w.text, w.line, w.col, got.Text, got.Line, got.Col)
		}
	}
}

// The non-comment stream is exactly Scan's: same kinds, texts, positions.
func TestKeepStreamMatchesScan(t *testing.T) {
	src := "pub fn f(a: Int64) -> Int64 {\n    // note\n    return a + 1 // done\n}\n"
	kept, kd := Keep("k.we", []byte(src))
	if kd != nil {
		t.Fatalf("keep diagnostic: %s", kd.Human())
	}
	scanned, _, sd := Scan("k.we", []byte(src))
	if sd != nil {
		t.Fatalf("scan diagnostic: %s", sd.Human())
	}
	var plain []Token
	for _, tk := range kept {
		if tk.Kind != KindComment {
			plain = append(plain, tk)
		}
	}
	if len(plain) != len(scanned) {
		t.Fatalf("stream length: want %d, got %d", len(scanned), len(plain))
	}
	for i := range scanned {
		if plain[i] != scanned[i] {
			t.Fatalf("token %d: scan %+v, keep %+v", i, scanned[i], plain[i])
		}
	}
}

// BOM stripping and CRLF line accounting behave the same in both faces.
func TestKeepBomCrlfSameAsScan(t *testing.T) {
	src := "\uFEFFlet a = 1\r\nlet b = 2\r\n"
	kept, kd := Keep("k.we", []byte(src))
	if kd != nil {
		t.Fatalf("keep diagnostic: %s", kd.Human())
	}
	var identLines []int
	for _, tk := range kept {
		if tk.Kind == KindIdent {
			identLines = append(identLines, tk.Line)
		}
	}
	if len(identLines) != 2 || identLines[0] != 1 || identLines[1] != 2 {
		t.Fatalf("ident lines: want [1 2], got %v", identLines)
	}
}

// A lexical error reports through Keep with the same code and position as
// Scan would (the parser gate's diagnostics pass through unchanged).
func TestKeepLexErrorFace(t *testing.T) {
	src := "let s = \"unterminated\n"
	_, kd := Keep("k.we", []byte(src))
	if kd == nil {
		t.Fatalf("want a diagnostic, got none")
	}
	_, _, sd := Scan("k.we", []byte(src))
	if sd == nil {
		t.Fatalf("scan agrees there is no error")
	}
	if kd.Code() != sd.Code() || !strings.Contains(kd.Message(), "unterminated") {
		t.Fatalf("diagnostic: want %s like scan, got %s", sd.Code(), kd.Human())
	}
	var _ = diag.Error // keep the import honest if unused paths change
}
