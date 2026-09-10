package parser

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// T4 interpolation holes (design D3): chapter 1's `${ … }` region is
// lexical — the scanner balances the braces and the literal rides as one
// token — so the expressions inside the holes are the parser's reading.
// The literal keeps its raw Text and grows Segs (the decoded runs between
// holes) plus Holes (one parsed expression each), which is what lets the
// codegen stage render a hole without re-entering the lexer.
//
// The hole's expression is lexed and parsed on its own, rebased onto the
// literal's own line and column, so every diagnostic inside a hole lands
// at its source position rather than at the literal's.

// topLetLit returns the literal initializer of a single-file top-level
// `let` binding.
func topLetLit(t *testing.T, src string) *ast.Literal {
	t.Helper()
	f := wantClean(t, src)
	items := f.Items
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	tl, ok := items[0].(*ast.TopLet)
	if !ok {
		t.Fatalf("expected a top-level let, got %T", items[0])
	}
	lit, ok := tl.Binding.Init.(*ast.Literal)
	if !ok {
		t.Fatalf("expected a literal initializer, got %T", tl.Binding.Init)
	}
	return lit
}

// TestInterpolationHoleSplit pins the segment/hole split: one decoded run
// before each hole and one after, in source order.
func TestInterpolationHoleSplit(t *testing.T) {
	lit := topLetLit(t, `let msg = "a${x}b"`)
	if lit.Kind != "string" || lit.Text != `"a${x}b"` {
		t.Fatalf("literal shape: %q %q", lit.Kind, lit.Text)
	}
	if len(lit.Segs) != 2 || lit.Segs[0] != "a" || lit.Segs[1] != "b" {
		t.Fatalf("segments: %q", lit.Segs)
	}
	if len(lit.Holes) != 1 || exprStr(lit.Holes[0]) != "x" {
		t.Fatalf("holes: %v", lit.Holes)
	}

	// Adjacent holes leave the middle segments empty; the split holds
	// len(Segs) == len(Holes)+1 in every shape.
	lit = topLetLit(t, `let msg = "${a}${b}"`)
	if len(lit.Segs) != 3 || lit.Segs[0] != "" || lit.Segs[1] != "" || lit.Segs[2] != "" {
		t.Fatalf("adjacent holes: %q", lit.Segs)
	}
	if len(lit.Holes) != 2 || exprStr(lit.Holes[0]) != "a" || exprStr(lit.Holes[1]) != "b" {
		t.Fatalf("adjacent holes: %v", lit.Holes)
	}
}

// TestInterpolationSegmentsDecodeEscapes: the segments are decoded bytes
// under chapter 1's closed escape set, while the holes keep their own
// source form.
func TestInterpolationSegmentsDecodeEscapes(t *testing.T) {
	lit := topLetLit(t, `let msg = "a\nb${x}\tz"`)
	if len(lit.Segs) != 2 || lit.Segs[0] != "a\nb" || lit.Segs[1] != "\tz" {
		t.Fatalf("segments: %q", lit.Segs)
	}
}

// TestInterpolationHoleExpressions: a hole holds one expression of the
// ordinary grammar — calls with nested literals included.
func TestInterpolationHoleExpressions(t *testing.T) {
	lit := topLetLit(t, `let msg = "${f(1, "q")} tail"`)
	if len(lit.Holes) != 1 || exprStr(lit.Holes[0]) != `f(1,"q")` {
		t.Fatalf("holes: %v", lit.Holes)
	}
	if len(lit.Segs) != 2 || lit.Segs[1] != " tail" {
		t.Fatalf("segments: %q", lit.Segs)
	}
	lit = topLetLit(t, `let msg = "${a + 1}"`)
	if exprStr(lit.Holes[0]) != "(a+1)" {
		t.Fatalf("operator hole: %v", lit.Holes)
	}
}

// TestInterpolationHolePositions: the hole's expression carries the file's
// own coordinates, so a later stage can report inside it. Here `n` sits at
// column 15 of the line.
func TestInterpolationHolePositions(t *testing.T) {
	lit := topLetLit(t, `let m = "abc${n}def"`)
	id, ok := lit.Holes[0].(*ast.Ident)
	if !ok {
		t.Fatalf("expected an identifier, got %T", lit.Holes[0])
	}
	if id.Line != 1 || id.Col != 15 {
		t.Fatalf("hole position: %d:%d, want 1:15", id.Line, id.Col)
	}
}

// TestInterpolationDollarNotBrace: `$` not followed by `{` is an ordinary
// character (chapter 1), so the literal stays plain.
func TestInterpolationDollarNotBrace(t *testing.T) {
	lit := topLetLit(t, `let m = "cost $ 5"`)
	if len(lit.Holes) != 0 || len(lit.Segs) != 0 {
		t.Fatalf("a plain literal carries no holes: %q %v", lit.Segs, lit.Holes)
	}
}

// TestInterpolationHoleErrors: a hole that is not one expression is a
// syntax error at the hole's own position (chapter 1 balances the braces;
// what they hold is the parser's to read).
func TestInterpolationHoleErrors(t *testing.T) {
	// The empty hole: `${}` holds nothing. The position is the region's
	// first character — the closing brace, where the expression was due.
	wantDiag(t, `let m = "${}"`, "E0105", "the ${} region is empty", 1, 12)
	// A hole holding no expression: the same anchor, the `}` after `1 +`.
	wantDiag(t, `let m = "${1 +}"`, "E0105", "cannot begin an expression", 1, 15)
	// Two expressions in one hole: the leftover token's own position.
	wantDiag(t, `let m = "${x y}"`, "E0105", "holds exactly one expression", 1, 14)
}

// TestInterpolationNestedLiteralHoles: a nested literal inside a hole may
// carry its own holes — the child parse recurses through the ordinary
// literal production.
func TestInterpolationNestedLiteralHoles(t *testing.T) {
	lit := topLetLit(t, `let m = "${g("x${y}")}"`)
	call, ok := lit.Holes[0].(*ast.Call)
	if !ok {
		t.Fatalf("expected a call, got %T", lit.Holes[0])
	}
	inner, ok := call.Args[0].(*ast.Literal)
	if !ok || len(inner.Holes) != 1 || exprStr(inner.Holes[0]) != "y" {
		t.Fatalf("nested hole: %+v", call.Args[0])
	}
	if len(inner.Segs) != 2 || inner.Segs[0] != "x" {
		t.Fatalf("nested segments: %q", inner.Segs)
	}
}

// TestInterpolationHoleKeepsRawText: Text stays the source form (what the
// formatter and the description tower read), never the decoded bytes.
func TestInterpolationHoleKeepsRawText(t *testing.T) {
	lit := topLetLit(t, `let m = "a\n${x}"`)
	if lit.Text != `"a\n${x}"` {
		t.Fatalf("raw text: %q", lit.Text)
	}
	if !strings.Contains(lit.Text, "${x}") {
		t.Fatalf("raw text lost the hole: %q", lit.Text)
	}
}
