package codegen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/parser"
)

// T10 assertEqual's structural comparison (B1a design D9). Until T10 the
// comparand domain was the leaf set and nothing else, and a record
// comparand stopped at the boundary; now the domain closes over the
// composites that declare Eq and the comparison walks them leaf by leaf,
// reporting the first leaf that differs at the position it reached.
//
// These programs are written as source and parsed, unlike the rest of the
// package's, which are hand-built AST. What this face turns on is a
// property of the declarations — a derives clause, a field's type, a
// tuple's elements — and source a reader can check against the ruling is
// worth more here than the isolation a hand-built tree would buy:
// parsing sits below emission, and the check stage still never runs.

// t10Emit parses one test module's source and emits it.
func t10Emit(t *testing.T, src string) (string, *NotImplemented) {
	t.Helper()
	f, d, ni := parser.Parse("m_test.we", []byte(src))
	if d != nil || ni != nil {
		t.Fatalf("parse: d=%v ni=%v", d, ni)
	}
	f.IsTestModule = true
	return EmitProgram(ModeTest, []ProgModule{{Key: "tests.m_test", File: f}})
}

// wantOrder asserts the fragments appear in the IR in the order given.
// The comparison's order is its semantics: the first leaf that differs is
// the one whose helper reports, so a walk that emitted its leaves out of
// declaration order would report a position the program never reached.
func wantOrder(t *testing.T, ir string, frags ...string) {
	t.Helper()
	at := 0
	for _, f := range frags {
		i := strings.Index(ir[at:], f)
		if i < 0 {
			t.Fatalf("IR is missing %q after byte %d:\n%s", f, at, ir)
		}
		at += i + len(f)
	}
}

// A record comparand walks its fields in declaration order, each leaf
// reporting under its own position.
func TestT10AssertEqualRecordWalk(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

record Point { name: String, x: Int64 } derives Eq

test "rec" {
    let a = Point { name: "ab", x: 1 }
    let b = Point { name: "cd", x: 2 }
    st.assertEqual(a, b)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, `c"Point.name\00"`, "the String field's position was not interned")
	wantIR(t, ir, `c"Point.x\00"`, "the Int64 field's position was not interned")
	// The pool's indices are assigned in emission order, so the call sites
	// witness the walk's order as surely as the constants would: p0 is the
	// first field, p1 the second.
	wantOrder(t, ir,
		"call void @__we_assert_eq_str_at(ptr @.p0, ptr",
		"call void @__we_assert_eq_i64_at(ptr @.p1,",
	)
}

// A nested record contributes its field names and not its own type name:
// the segment that reached it already says which field it is.
func TestT10AssertEqualRecordNested(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

record Inner { x: Int64 } derives Eq
record Outer { i: Inner, s: String } derives Eq

test "nested" {
    let a = Outer { i: Inner { x: 1 }, s: "a" }
    let b = Outer { i: Inner { x: 2 }, s: "b" }
    st.assertEqual(a, b)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantOrder(t, ir, `c"Outer.i.x\00"`, `c"Outer.s\00"`)
	if strings.Contains(ir, `Inner\00`) {
		t.Fatalf("the nested record repeated its type name in a position:\n%s", ir)
	}
}

// A record that declares no clause has no generated equality to assert,
// so it stops at the residual row the check stage's own stop uses.
func TestT10AssertEqualRecordNeedsTheClause(t *testing.T) {
	_, ni := t10Emit(t, `
import std.test as st

record Q { x: Int64 }

test "no clause" {
    let a = Q { x: 1 }
    let b = Q { x: 2 }
    st.assertEqual(a, b)
}
`)
	if ni == nil || ni.What != bndGenericFns {
		t.Fatalf("want the residual row %q, got %v", bndGenericFns, ni)
	}
}

// The leaf set is the check stage's, so a component outside it stops even
// though the clause is legal and the generated .equals() would work.
func TestT10AssertEqualRecordLeafBoundaries(t *testing.T) {
	cases := []struct{ name, decl, a, b string }{
		{"float field", "record F { v: Float64 } derives Eq", "F { v: 1.5 }", "F { v: 2.5 }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ni := t10Emit(t, "import std.test as st\n\n"+c.decl+"\ntest \"leaf\" {\n    let a = "+c.a+
				"\n    let b = "+c.b+"\n    st.assertEqual(a, b)\n}\n")
			if ni == nil || ni.What != bndGenericFns {
				t.Fatalf("want the residual row %q, got %v", bndGenericFns, ni)
			}
		})
	}
}

// The two families the ruling aligns: the top-level leaf renders Bool as
// true/false and UInt64 by its unsigned magnitude, where the pre-T10
// numeric route printed 1/0 and the bit pattern's signed reading.
func TestT10AssertEqualTopLevelLeafFamilies(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

test "leaves" {
    st.assertEqual(true, false)
    st.assertEqual(18446744073709551615u64, 2u64)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "call void @__we_assert_eq_bool_at(ptr null", "Bool takes its own row")
	wantIR(t, ir, "call void @__we_assert_eq_u64_at(ptr null", "UInt64 takes its own row")
}

// The top-level Int64 and String leaves keep the pre-T10 symbols. Those
// two calls are the byte-pinned goldens' only emitters, so keeping the
// rows distinct makes the modules they appear in unchanged by
// construction rather than by inspection.
func TestT10AssertEqualTopLevelKeepsTheOldRows(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

test "top" {
    st.assertEqual(3, 4)
    st.assertEqual("abc", "abd")
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, "call void @__we_assert_eq_i64(i64 3, i64 4)", "the i64 row is the old one")
	wantIR(t, ir, "call void @__we_assert_eq_str(ptr", "the String row is the old one")
	for _, sym := range []string{"__we_assert_eq_i64_at", "__we_assert_eq_str_at"} {
		if strings.Contains(ir, sym) {
			t.Fatalf("the top-level leaves took %s:\n%s", sym, ir)
		}
	}
}

// The path is the report, so the walk gives up before a declaration nested
// past eqPathMaxSegments can spend the report on the path rather than on
// the leaf. A record field's record is looked up in the same module only,
// which is what lets a chain grow a path without bound — the bound is the
// walk's own, and it is asked at every descent rather than once at the
// root, because the descent is what lengthens the position.
func TestT10AssertEqualRecordDepthBound(t *testing.T) {
	// A chain of n records: R0 holds R1 holds ... R{n-1} holds the leaf.
	// Ri's position carries i segments after the root's, so the deepest
	// record of a chain of n reads a position of n-1 segments.
	source := func(n int) string {
		var b strings.Builder
		b.WriteString("import std.test as st\n\n")
		for i := 0; i < n-1; i++ {
			fmt.Fprintf(&b, "record R%d { f: R%d } derives Eq\n", i, i+1)
		}
		fmt.Fprintf(&b, "record R%d { x: Int64 } derives Eq\n", n-1)
		b.WriteString("\ntest \"deep\" {\n    let a = ")
		for i := 0; i < n-1; i++ {
			fmt.Fprintf(&b, "R%d { f: ", i)
		}
		fmt.Fprintf(&b, "R%d { x: 1 }", n-1)
		b.WriteString(strings.Repeat(" }", n-1))
		b.WriteString("\n    let b = ")
		for i := 0; i < n-1; i++ {
			fmt.Fprintf(&b, "R%d { f: ", i)
		}
		fmt.Fprintf(&b, "R%d { x: 2 }", n-1)
		b.WriteString(strings.Repeat(" }", n-1))
		b.WriteString("\n    st.assertEqual(a, b)\n}\n")
		return b.String()
	}
	// The ends are written as literals rather than read off the constant:
	// the bound is a decided value, and a test spelled in terms of it would
	// follow it anywhere. A chain of 32 records reads its deepest position
	// in 31 segments and walks the whole way; a chain of 33 reads one in 32
	// and stops.
	if _, ni := t10Emit(t, source(33)); ni == nil {
		t.Fatalf("a chain past the %d-segment bound; emission accepted it", eqPathMaxSegments)
	}
	if _, ni := t10Emit(t, source(32)); ni != nil {
		t.Fatalf("a chain within the %d-segment bound; emission stopped at %q", eqPathMaxSegments, ni.What)
	}
}

// The mirror: the rows the check stage's own table enumerates, one for
// one. The two stages decide the same domain — this is the test that says
// so, and it is the one that would catch a widening on either side
// without the other. Its row list grows with the track: a tuple and a sum
// comparand join it in the commits that widen them.
func TestT10AssertEqualDomainMirror(t *testing.T) {
	cases := []struct {
		name  string
		decls string
		a, b  string
		want  bool
	}{
		{"i64 literal", "", "1", "2", true},
		{"bool literal", "", "true", "false", true},
		{"string literal", "", `"a"`, `"b"`, true},
		{"eq record", "record P { x: Int64 } derives Eq\n", "P { x: 1 }", "P { x: 2 }", true},
		{
			"nested eq record",
			"record I { x: Int64 } derives Eq\nrecord O { i: I, s: String } derives Eq\n",
			"O { i: I { x: 1 }, s: \"a\" }", "O { i: I { x: 2 }, s: \"b\" }", true,
		},
		{"plain record", "record Q { x: Int64 }\n", "Q { x: 1 }", "Q { x: 2 }", false},
		{"nested without a clause", "record I { x: Int64 }\nrecord O { i: I } derives Eq\n", "O { i: I { x: 1 } }", "O { i: I { x: 2 } }", false},
		{"float field", "record F { v: Float64 } derives Eq\n", "F { v: 1.5 }", "F { v: 2.5 }", false},
		// A Rune field stops a step earlier than the domain: the layout has
		// no row for it, so the record's construction is the body's stop.
		// The mirror asks only whether the row is out, which it is.
		{"rune field", "record R { v: Rune } derives Eq\n", "R { v: 'a' }", "R { v: 'b' }", false},
		{"list", "", "[1, 2]", "[1, 3]", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ni := t10Emit(t, "import std.test as st\n\n"+c.decls+"\ntest \"row\" {\n    let a = "+c.a+
				"\n    let b = "+c.b+"\n    st.assertEqual(a, b)\n}\n")
			if c.want && ni != nil {
				t.Fatalf("the domain admits this row; emission stopped at %q", ni.What)
			}
			if !c.want && ni == nil {
				t.Fatalf("the domain refuses this row; emission accepted it")
			}
		})
	}
}
