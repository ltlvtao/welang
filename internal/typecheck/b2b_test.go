package typecheck

import "testing"

// TestCollectionMembersThirteenSurface: the stdlib thirteen (B2b design
// D4-1) check clean in the shapes the table writes, and every signature
// slot that can disagree produces the honest E0501 — the argument-vs-
// parameter form for parameter slots, the binding-vs-annotation form for
// return slots. The Map faces pin K/V substitution through the receiver's
// own arguments (Option<String>, List<String>), and the record face pins
// T threading into removeAt's payload.
func TestCollectionMembersThirteenSurface(t *testing.T) {
	ok := []struct{ name, src string }{
		{"list add binds unit", "fn f(xs: List<Int64>) {\n    let ys = xs.add(1)\n}\n"},
		{"list surface round trip", "fn f(xs: List<Int64>) {\n    xs.add(1)\n    let y = xs.removeAt(0)\n    let z = xs.get(0)\n    let n = xs.size()\n}\n"},
		{"map surface round trip", "fn f(m: Map<Int64, Int64>) {\n    m.put(1, 2)\n    let y = m.remove(1)\n    let z = m.get(1)\n    let ks = m.keys()\n    let n = m.size()\n}\n"},
		{"set surface round trip", "fn f(s: Set<Int64>) {\n    s.add(4)\n    let b = s.remove(4)\n    let c = s.has(4)\n    let n = s.size()\n}\n"},
		{"record element threads into removeAt payload", "record Cell { hi: Int64 }\nfn f(xs: List<Cell>, c: Cell) -> Int64 {\n    xs.add(c)\n    let o = xs.removeAt(0)\n    let r = match o {\n        Some(v) => v.hi\n        None => 0\n    }\n    r\n}\n"},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			wantOK(t, tc.src)
		})
	}

	bad := []struct {
		name      string
		src       string
		msg       string
		line, col int
	}{
		{"list add argument", "fn f(xs: List<Int64>) {\n    xs.add(\"x\")\n}\n", "the argument is String, the parameter is Int64", 2, 12},
		{"list removeAt argument", "fn f(xs: List<Int64>) {\n    let y = xs.removeAt(\"x\")\n}\n", "the argument is String, the parameter is Int64", 2, 25},
		{"list removeAt returns Option", "fn f(xs: List<Int64>) {\n    let y: Int64 = xs.removeAt(0)\n}\n", "the expression is Option<Int64>, the annotation is Int64", 2, 9},
		{"void members bind unit only", "fn f(xs: List<Int64>) {\n    let y: Int64 = xs.add(1)\n}\n", "the expression is (), the annotation is Int64", 2, 9},
		{"size returns Int64", "fn f(xs: List<Int64>) {\n    let b: Bool = xs.size()\n}\n", "the expression is Int64, the annotation is Bool", 2, 9},
		{"map put takes K first", "fn f(m: Map<Int64, Int64>) {\n    m.put(\"x\", 2)\n}\n", "the argument is String, the parameter is Int64", 2, 11},
		{"map put takes V second", "fn f(m: Map<Int64, Int64>) {\n    m.put(1, \"x\")\n}\n", "the argument is String, the parameter is Int64", 2, 14},
		{"map remove returns Option of V", "fn f(m: Map<Int64, String>) {\n    let y: Int64 = m.remove(1)\n}\n", "the expression is Option<String>, the annotation is Int64", 2, 9},
		{"map keys returns List of K", "fn f(m: Map<String, Int64>) {\n    let y: Int64 = m.keys()\n}\n", "the expression is List<String>, the annotation is Int64", 2, 9},
		{"map get stays key-typed", "fn f(m: Map<String, Int64>) {\n    let z = m.get(1)\n}\n", "the argument is Int64, the parameter is String", 2, 19},
		{"set add argument", "fn f(s: Set<Int64>) {\n    s.add(\"x\")\n}\n", "the argument is String, the parameter is Int64", 2, 11},
		{"set remove returns Bool", "fn f(s: Set<Int64>) {\n    let y: Int64 = s.remove(1)\n}\n", "the expression is Bool, the annotation is Int64", 2, 9},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			wantDiag(t, tc.src, "E0501", tc.msg, tc.line, tc.col)
		})
	}
}

// TestCollectionMembersOffTableFallThrough: everything past the thirteen
// still answers E0816 through the unchanged fall-through arm (B2b design
// D4-2) — the surface widens, the boundary does not move past it.
func TestCollectionMembersOffTableFallThrough(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		msg       string
		line, col int
	}{
		{"list push", "fn f(xs: List<Int64>) {\n    let ys = xs.push(1)\n}\n", `"push" is not a member of List<Int64>`, 2, 17},
		{"map values", "fn f(m: Map<Int64, Int64>) {\n    m.values()\n}\n", `"values" is not a member of Map<Int64, Int64>`, 2, 7},
		{"set clear", "fn f(s: Set<Int64>) {\n    s.clear()\n}\n", `"clear" is not a member of Set<Int64>`, 2, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantDiag(t, tc.src, "E0816", tc.msg, tc.line, tc.col)
		})
	}
}
