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

// A tuple compares element by element and reports the bare index: a tuple
// has no name of its own, so the index is the whole of what reaches it.
func TestT10AssertEqualTupleWalk(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

test "tup" {
    let a = ("ab", 1)
    let b = ("cd", 2)
    st.assertEqual(a, b)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, `c"0\00"`, "the first element's position was not interned")
	wantIR(t, ir, `c"1\00"`, "the second element's position was not interned")
	wantOrder(t, ir,
		"call void @__we_assert_eq_str_at(ptr @.p0, ptr",
		"call void @__we_assert_eq_i64_at(ptr @.p1,",
	)
}

// A record read out of a tuple is reached by its index and then by its
// field names, exactly as a record field of a record is: the segment that
// reached it already says which element it is.
func TestT10AssertEqualTupleRecordElement(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

record Point { x: Int64 } derives Eq

test "tup rec" {
    let a = (Point { x: 1 }, "a")
    let b = (Point { x: 2 }, "b")
    st.assertEqual(a, b)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantOrder(t, ir, `c"0.x\00"`, `c"1\00"`)
}

// The tuple carries a second source: a name already bound to one, whose
// shape rides the environment the way a list's element face does.
func TestT10AssertEqualTupleBinding(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

test "tup let" {
    let a = (1, "ab")
    let b = (1, "cd")
    st.assertEqual(a, b)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantOrder(t, ir,
		"call void @__we_assert_eq_i64_at(ptr @.",
		"call void @__we_assert_eq_str_at(ptr @.",
	)
}

// An element outside the leaf set stops the whole comparison, exactly as a
// record's field outside it does: the domain is one set, and a tuple does
// not widen it.
func TestT10AssertEqualTupleElementBoundaries(t *testing.T) {
	cases := []struct{ name, a, b string }{
		{"float element", "(1.5, 1)", "(2.5, 1)"},
		{"rune element", "('a', 1)", "('b', 1)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ni := t10Emit(t, "import std.test as st\n\ntest \"el\" {\n    let a = "+c.a+
				"\n    let b = "+c.b+"\n    st.assertEqual(a, b)\n}\n")
			if ni == nil || ni.What != bndGenericFns {
				t.Fatalf("want the residual row %q, got %v", bndGenericFns, ni)
			}
		})
	}
}

// A newtype is transparent: the wrapper erases (design D4), so a newtype
// over a leaf compares as that leaf and reports no position at the top
// level, and one over a record compares as the record and reports the
// record's own name.
func TestT10AssertEqualNewtypeTransparent(t *testing.T) {
	t.Run("over a leaf", func(t *testing.T) {
		ir, ni := t10Emit(t, `
import std.test as st

newtype UserId(Int64) derives Eq

test "nt" {
    let a = UserId(3)
    let b = UserId(4)
    st.assertEqual(a, b)
}
`)
		if ni != nil {
			t.Fatalf("boundary: %s", ni.What)
		}
		wantIR(t, ir, "call void @__we_assert_eq_i64(i64 3, i64 4)", "the wrapper did not erase")
	})
	t.Run("over a record", func(t *testing.T) {
		ir, ni := t10Emit(t, `
import std.test as st

record Point { x: Int64 } derives Eq
newtype Spot(Point) derives Eq

test "nt rec" {
    let a = Spot(Point { x: 1 })
    let b = Spot(Point { x: 2 })
    st.assertEqual(a, b)
}
`)
		if ni != nil {
			t.Fatalf("boundary: %s", ni.What)
		}
		wantIR(t, ir, `c"Point.x\00"`, "the path names the erased type, not the wrapper")
	})
}

// A sum comparison reads the discriminants first. Two values that are not
// the same variant share no payload: their words describe different
// declarations, so the report names the two variants and the payload walk
// is never reached; where the tags agree the walk dispatches on the tag
// and each variant pays its own positions, named by the variant and then
// by the declared index.
func TestT10AssertEqualSumWalk(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

type Shape = Circle(Int64) | Rect(Int64, Int64) derives Eq

test "sum" {
    let a = Circle(1)
    let b = Circle(2)
    st.assertEqual(a, b)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	// One name constant per variant, interned once for the pair: both
	// sides read the same table, so the report's two names are one pool
	// entry per variant rather than one per side.
	wantOrder(t, ir, `c"Circle\00"`, `c"Rect\00"`)
	// A variant contributes its name before the index, so two variants
	// with a first position each read as two positions — the root's own
	// name is interned ahead of them, the dispatch following the report.
	wantOrder(t, ir,
		`c"Shape\00"`,
		`c"Shape.Circle.0\00"`, `c"Shape.Rect.0\00"`, `c"Shape.Rect.1\00"`,
	)
	// The report for differing tags comes before the payload calls and
	// ends its block: nothing a differing pair could have compared runs.
	wantOrder(t, ir,
		"call void @__we_assert_eq_variant_at(ptr ",
		"call void @__we_assert_eq_i64_at(ptr ",
	)
	wantIR(t, ir, "unreachable", "the tag-differs block ends its block")
}

// The payload a variant declares is read through the slot's own words —
// the same face a match arm binds, without the binding.
func TestT10AssertEqualSumPayloadFaces(t *testing.T) {
	cases := []struct {
		name, decls, a, b string
		want              []string
	}{
		{
			"string payload",
			"type Tag = Named(String) | Bare derives Eq\n",
			"Named(\"ab\")", "Named(\"cd\")",
			[]string{`c"Tag.Named.0\00"`, "call void @__we_assert_eq_str_at(ptr "},
		},
		{
			"record payload",
			"record P { x: Int64 } derives Eq\ntype Box = Hold(P) | Empty derives Eq\n",
			"Hold(P { x: 1 })", "Hold(P { x: 2 })",
			[]string{`c"Box.Hold.0.x\00"`, "call void @__we_assert_eq_i64_at(ptr "},
		},
		{
			"payload position",
			"type Pair = Both(Int64, Int64) | Neither derives Eq\n",
			"Both(1, 2)", "Both(3, 4)",
			[]string{`c"Pair.Both.0\00"`, `c"Pair.Both.1\00"`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ir, ni := t10Emit(t, "import std.test as st\n\n"+c.decls+"\ntest \"p\" {\n    let a = "+c.a+
				"\n    let b = "+c.b+"\n    st.assertEqual(a, b)\n}\n")
			if ni != nil {
				t.Fatalf("boundary: %s", ni.What)
			}
			for _, f := range c.want {
				wantIR(t, ir, f, "the payload's face was not emitted")
			}
		})
	}
}

// Positional payloads read their own words: the second Int64 of two is the
// slot's other word, not a second read of the first. A walk that read one
// word twice would still emit both positions and still name them — the
// paths would be right and only the values wrong — so the check is on the
// pointers the operands were loaded from, which is the reading bindArmWord
// does and the one a second reader of the layout has to keep.
func TestT10AssertEqualSumPositionReadsItsOwnWord(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

type Pair = Both(Int64, Int64) | Neither derives Eq

test "pair" {
    let a = Both(1, 2)
    let b = Both(3, 4)
    st.assertEqual(a, b)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	loaded := func(line string) (string, bool) {
		const load = "load i64, ptr "
		i := strings.Index(line, load)
		if i < 0 {
			return "", false
		}
		return strings.TrimRight(line[i+len(load):], " \t\r"), true
	}
	// A scalar leaf loads its two sides and then calls, in that order, so
	// the two loads standing above a call are the words that call compares.
	var from [][2]string
	var last []string
	for _, line := range strings.Split(ir, "\n") {
		if p, ok := loaded(line); ok {
			if last = append(last, p); len(last) > 2 {
				last = last[1:]
			}
			continue
		}
		if !strings.Contains(line, "call void @__we_assert_eq_i64_at(ptr ") {
			continue
		}
		if len(last) != 2 {
			t.Fatalf("want two loads above %q, got %v", line, last)
		}
		from = append(from, [2]string{last[0], last[1]})
		last = nil
	}
	if len(from) != 2 {
		t.Fatalf("want the variant's two positions, got %d: %v", len(from), from)
	}
	if from[0] == from[1] {
		t.Fatalf("both positions read the same words: %v", from[0])
	}
}

// A sum that declares no clause has no generated equality to assert, and
// a payload outside the leaf set stops the walk that would have read it.
func TestT10AssertEqualSumBoundaries(t *testing.T) {
	cases := []struct{ name, decls, a, b string }{
		{"no clause", "type Shape = Circle(Int64) | Rect(Int64, Int64)\n", "Circle(1)", "Circle(2)"},
		{"float payload", "type Box = Hold(Float64) | Drop(Float64) derives Eq\n", "Hold(1.5)", "Drop(2.5)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ni := t10Emit(t, "import std.test as st\n\n"+c.decls+"\ntest \"s\" {\n    let a = "+c.a+
				"\n    let b = "+c.b+"\n    st.assertEqual(a, b)\n}\n")
			if ni == nil || ni.What != bndGenericFns {
				t.Fatalf("want the residual row %q, got %v", bndGenericFns, ni)
			}
		})
	}
}

// A sum reached as a call's result rather than as a construction is the
// same face: the table rides the signature, and the slot carries it.
func TestT10AssertEqualSumThroughAReturn(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

type Shape = Circle(Int64) | Dot derives Eq

fn pick(n: Int64) -> Shape {
    if n == 0 {
        return Circle(1)
    }
    return Dot
}

test "sum ret" {
    let a = pick(0)
    let b = pick(1)
    st.assertEqual(a, b)
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, `c"Shape.Circle.0\00"`, "the return's table reached the walk")
	wantIR(t, ir, "call void @__we_assert_eq_variant_at(ptr ", "the variants are reported")
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
		{"tuple", "", "(1, 2)", "(1, 3)", true},
		{"tuple of records", "record P { x: Int64 } derives Eq\n", "(P { x: 1 }, 1)", "(P { x: 2 }, 1)", true},
		{"tuple with a float", "", "(1.5, 1)", "(2.5, 1)", false},
		// A tuple does not nest: the shape and the layout both refuse a
		// tuple inside anything, so the domain stops where they do.
		{"nested tuple", "", "((1, 2), 3)", "((1, 3), 3)", false},
		{"newtype over a leaf", "newtype Id(Int64) derives Eq\n", "Id(1)", "Id(2)", true},
		{"sum", "type S = C(Int64) | E(Int64) derives Eq\n", "C(1)", "C(2)", true},
		{"sum variants differ", "type S = C(Int64) | E(Int64) derives Eq\n", "C(1)", "E(1)", true},
		{"sum without a clause", "type S = C(Int64) | E(Int64)\n", "C(1)", "E(1)", false},
		{"sum with a float payload", "type S = C(Float64) | E(Float64) derives Eq\n", "C(1.5)", "E(2.5)", false},
		// A sum element stops at the aggregate's face rather than at the
		// domain's: the aggregate stores a sum's three words and keeps no
		// variant table to read them at. Like the nested tuple above, the
		// row asks only whether the row is out.
		{"sum in a tuple", "type S = C(Int64) | E(Int64) derives Eq\n", "(C(1), 1)", "(C(2), 1)", false},
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

// A sum inside a tuple is the aggregate's boundary rather than the
// domain's: the check stage admits it (the sum's leaves are in the set)
// and the aggregate stores its three words, but the element face the walk
// reads a tuple through carries the kind, the key, and the words — not
// the variant table, which is what says what the tag's payload holds. So
// the walk ends where the shape it needs ends, as it does at a nested
// tuple, and the row is the residual one.
func TestT10AssertEqualSumElementBoundary(t *testing.T) {
	_, ni := t10Emit(t, `
import std.test as st

type Shape = Circle(Int64) | Rect(Int64, Int64) derives Eq

test "sum el" {
    let a = (Circle(1), 1)
    let b = (Circle(2), 1)
    st.assertEqual(a, b)
}
`)
	if ni == nil || ni.What != bndGenericFns {
		t.Fatalf("want the residual row %q, got %v", bndGenericFns, ni)
	}
}

// A prelude sum declares nothing for a clause to sit on, so the walk
// stops on it however the value got there — a parameter carries the
// table its signature classified it against, and the table is all the
// slot has. The two Option faces are the same answer chapter 10 gave the
// hand-written impl (E0822).
func TestT10AssertEqualPreludeSumsStayOut(t *testing.T) {
	_, ni := t10Emit(t, `
import std.test as st

fn same(a: Option<Int64>, b: Option<Int64>) {
    st.assertEqual(a, b)
}

test "prelude" {
    same(Some(1), Some(2))
}
`)
	if ni == nil || ni.What != bndGenericFns {
		t.Fatalf("want the residual row %q, got %v", bndGenericFns, ni)
	}
}

// The same parameter face on a sum that does declare the clause: the
// table rides the signature and the slot, so a comparison inside the
// callee walks exactly as one on a construction does.
func TestT10AssertEqualSumParameter(t *testing.T) {
	ir, ni := t10Emit(t, `
import std.test as st

type Shape = Circle(Int64) | Rect(Int64, Int64) derives Eq

fn same(a: Shape, b: Shape) {
    st.assertEqual(a, b)
}

test "param" {
    same(Circle(1), Circle(2))
}
`)
	if ni != nil {
		t.Fatalf("boundary: %s", ni.What)
	}
	wantIR(t, ir, `c"Shape.Circle.0\00"`, "the parameter's table reached the walk")
	wantIR(t, ir, "call void @__we_assert_eq_variant_at(ptr ", "the variants are reported")
}
