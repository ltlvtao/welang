package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
	"github.com/ltlvtao/welang/internal/parser"
	"github.com/ltlvtao/welang/internal/typecheck"
)

// The mangler's alphabet is the one LLVM's bare identifiers admit: a
// symbol is spelled into the IR unquoted (codegen.go's define and call
// lines), so every byte of it has to be one of these.
var bareIdent = regexp.MustCompile(`^[-a-zA-Z$._][-a-zA-Z$._0-9]*$`)

// mangleFixture builds the emitter side the mangler reads: the
// declaration index of a program whose modules hold the named records.
// Nothing else of the emitter is touched, so a test states its
// declarations once and mangles against them.
func mangleFixture(mods ...ProgModule) *emitter {
	return &emitter{declKeys: declIndex(mods)}
}

// recordMod is one module holding the named records, in order.
func recordMod(key string, names ...string) ProgModule {
	f := &ast.File{}
	for _, n := range names {
		f.Items = append(f.Items, &ast.RecordDecl{Name: n})
	}
	return ProgModule{Key: key, ID: key, File: f}
}

// declOf finds one record declaration of a module fixture — the node the
// test then names in a ShapeDecl.
func declOf(t *testing.T, m ProgModule, name string) *ast.RecordDecl {
	t.Helper()
	for _, it := range m.File.Items {
		if r, ok := it.(*ast.RecordDecl); ok && r.Name == name {
			return r
		}
	}
	t.Fatalf("module %s declares no record %s", m.Key, name)
	return nil
}

func shpBase(name string) typecheck.Shape {
	return typecheck.Shape{Kind: typecheck.ShapeBase, Name: name}
}

func shpNominal(d typecheck.ShapeDecl, args ...typecheck.Shape) typecheck.Shape {
	return typecheck.Shape{Kind: typecheck.ShapeNominal, Decl: d, Args: args}
}

// TestMangleShapeForms pins design D2's encoding form by form. The user
// declaration rows are the one place this build departs from D2's
// written table: an argument that names a user declaration encodes as
// its module-qualified key, not its bare name. A bare name is exactly
// the collision D1 rejected the rendering string for — two modules each
// declaring Point would render one argument for two types — so the
// encoding is built from what names a declaration, never from what it is
// called.
func TestMangleShapeForms(t *testing.T) {
	a := recordMod("a", "Point", "Wrap")
	e := mangleFixture(a)
	point := declOf(t, a, "Point")
	wrap := declOf(t, a, "Wrap")

	pointD := typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: point}
	wrapD := typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: wrap}
	listD := typecheck.ShapeDecl{Kind: typecheck.DeclSum, Name: "List"}
	iterD := typecheck.ShapeDecl{Kind: typecheck.DeclIface, Name: "Iterator"}

	pt := shpNominal(pointD)
	int64t := shpBase("Int64")

	cases := []struct {
		name string
		sh   typecheck.Shape
		want string
	}{
		{"Int64", shpBase("Int64"), "Int64"},
		{"String", shpBase("String"), "String"},
		{"Bool", shpBase("Bool"), "Bool"},
		{"Float64", shpBase("Float64"), "Float64"},
		{"Int8", shpBase("Int8"), "Int8"},
		{"Unit", typecheck.Shape{Kind: typecheck.ShapeUnit}, "U"},
		{"Never", typecheck.Shape{Kind: typecheck.ShapeNever}, "N"},
		{"List<Int64>", shpNominal(listD, int64t), "List$Int64"},
		{"Wrap<Point>", shpNominal(wrapD, pt), "a.Wrap$a.Point"},
		{"Wrap<List<Int64>>", shpNominal(wrapD, shpNominal(listD, int64t)), "a.Wrap$List$Int64"},
		{"List<Wrap<Point>>", shpNominal(listD, shpNominal(wrapD, pt)), "List$a.Wrap$a.Point"},
		{"tuple", typecheck.Shape{Kind: typecheck.ShapeTuple, Elems: []typecheck.Shape{int64t, pt}}, "T$2$Int64$a.Point"},
		{"three-tuple", typecheck.Shape{Kind: typecheck.ShapeTuple, Elems: []typecheck.Shape{int64t, pt, {Kind: typecheck.ShapeNever}}}, "T$3$Int64$a.Point$N"},
		{"fn", typecheck.Shape{Kind: typecheck.ShapeFn, Params: []typecheck.Shape{int64t}, Ret: &pt}, "F$Int64_a.Point"},
		{"valueless fn", typecheck.Shape{Kind: typecheck.ShapeFn}, "F$_V"},
		{"unit-returning fn", typecheck.Shape{Kind: typecheck.ShapeFn, Ret: &typecheck.Shape{Kind: typecheck.ShapeUnit}}, "F$_U"},
		{"dyn", typecheck.Shape{Kind: typecheck.ShapeDyn, Decl: iterD, Args: []typecheck.Shape{int64t}}, "Dyn$Iterator$Int64"},
		{"bare dyn", typecheck.Shape{Kind: typecheck.ShapeDyn, Decl: iterD}, "Dyn$Iterator"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := e.mangleShape(c.sh)
			if got != c.want {
				t.Fatalf("encoding is %q, want %q", got, c.want)
			}
			if !bareIdent.MatchString(got) {
				t.Fatalf("encoding %q is not an LLVM bare identifier", got)
			}
		})
	}
}

// TestMangleSuffixAndSymbols pins the other half of D2's boundary: a
// declaration with no type arguments takes no suffix, so every symbol
// B1a spelled is unchanged — and an instantiated one appends the suffix
// its arguments render.
func TestMangleSuffixAndSymbols(t *testing.T) {
	a := recordMod("a", "Point", "Wrap")
	e := mangleFixture(a)
	pointD := typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: declOf(t, a, "Point")}

	if got := e.mangleSuffix(nil); got != "" {
		t.Fatalf("no arguments rendered the suffix %q", got)
	}
	if got := e.mangleSuffix([]typecheck.Shape{shpBase("Int64")}); got != "$Int64" {
		t.Fatalf("one argument rendered %q, want $Int64", got)
	}
	if got := e.mangleSuffix([]typecheck.Shape{shpBase("Int64"), shpNominal(pointD)}); got != "$Int64$a.Point" {
		t.Fatalf("two arguments rendered %q, want $Int64$a.Point", got)
	}

	plain := &fnDef{key: "main", name: "id"}
	if got := plain.sym(); got != "main.id" {
		t.Fatalf("a plain fn's symbol is %q, want main.id", got)
	}
	method := &fnDef{key: "main", name: "pick", recvKey: "main.Age"}
	if got := method.sym(); got != "main.Age.pick" {
		t.Fatalf("a method's symbol is %q, want main.Age.pick", got)
	}
	inst := &fnDef{key: "main", name: "id", suffix: e.mangleSuffix([]typecheck.Shape{shpBase("Int64")})}
	if got := inst.sym(); got != "main.id$Int64" {
		t.Fatalf("an instantiated fn's symbol is %q, want main.id$Int64", got)
	}
	if !bareIdent.MatchString(inst.sym()) {
		t.Fatalf("symbol %q is not an LLVM bare identifier", inst.sym())
	}
}

// TestMangleSeparatesSameNamedDeclarations is the negative assertion
// behind D2's refusal of a rendering string: two modules each declaring
// Point render the same text, and one declaration applied to them is two
// instantiations. Readable segments are the other face of the same
// choice — the encoding spells the declaration names, so an IR reader
// sees which instantiation a symbol is.
func TestMangleSeparatesSameNamedDeclarations(t *testing.T) {
	mods := []ProgModule{recordMod("a", "Point", "Wrap"), recordMod("b", "Point", "Wrap")}
	e := mangleFixture(mods...)
	wrapD := typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: declOf(t, mods[0], "Wrap")}
	same := typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: declOf(t, mods[0], "Wrap")}

	// The one declaration, applied to two modules' Point.
	na := e.mangleApply("a.Wrap", []typecheck.Shape{shpNominal(typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: declOf(t, mods[0], "Point")})})
	nb := e.mangleApply("a.Wrap", []typecheck.Shape{shpNominal(typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: declOf(t, mods[1], "Point")})})
	if na == nb {
		t.Fatalf("two modules' Point rendered one encoding: %q", na)
	}
	if na != "a.Wrap$a.Point" || nb != "a.Wrap$b.Point" {
		t.Fatalf("encodings are %q and %q, want a.Wrap$a.Point and a.Wrap$b.Point", na, nb)
	}
	// Readable, not hashed: both declaration names are in the symbol.
	for _, s := range []string{na, nb} {
		if !strings.Contains(s, "Wrap") || !strings.Contains(s, "Point") {
			t.Fatalf("symbol %q does not spell the declarations it names", s)
		}
	}

	// The same two modules' Wrap declarations are two entries in the
	// index — the node is the identity, in the index as in the checker.
	da := shpNominal(wrapD)
	db := shpNominal(typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: declOf(t, mods[1], "Wrap")})
	if e.mangleShape(da) == e.mangleShape(db) {
		t.Fatalf("two modules' Wrap rendered one encoding: %q", e.mangleShape(da))
	}
	if e.declKey(wrapD) != e.declKey(same) {
		t.Fatal("the same declaration node rendered two keys")
	}
}

// TestInstTableInternsOneSymbol pins the table's two rules: one
// {declaration, arguments} is one symbol however many sites reach it, and
// registration order is the order the defines take (design D3).
func TestInstTableInternsOneSymbol(t *testing.T) {
	var tab instTable
	int64t := []typecheck.Shape{shpBase("Int64")}
	stringt := []typecheck.Shape{shpBase("String")}

	first := tab.register("main.id", "$Int64", int64t)
	if first != "main.id$Int64" {
		t.Fatalf("the interned symbol is %q, want main.id$Int64", first)
	}
	if again := tab.register("main.id", "$Int64", int64t); again != first {
		t.Fatalf("one instantiation interned two symbols: %q and %q", first, again)
	}
	if len(tab.order) != 1 {
		t.Fatalf("one instantiation registered %d entries", len(tab.order))
	}
	tab.register("main.id", "$String", stringt)
	if len(tab.order) != 2 {
		t.Fatalf("two instantiations registered %d entries", len(tab.order))
	}
	if tab.order[0].suffix != "$Int64" || tab.order[1].suffix != "$String" {
		t.Fatalf("registration order is %+v", tab.order)
	}
	// A different declaration under the same suffix is a different symbol,
	// not a repeat: the key is the declaration's, never the suffix alone.
	if got := tab.register("main.pick", "$Int64", int64t); got != "main.pick$Int64" {
		t.Fatalf("a second declaration interned %q", got)
	}
	if len(tab.order) != 3 {
		t.Fatalf("three instantiations registered %d entries", len(tab.order))
	}
}

// TestInstTableRefusesOneSymbolForTwoTypes is the table's guard: a symbol
// met again under a different argument list is one name for two types,
// which no later stage could tell apart. It fails loudly rather than
// emitting one define for both.
func TestInstTableRefusesOneSymbolForTwoTypes(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("two argument lists under one symbol were accepted")
		}
		if !strings.Contains(fmt.Sprint(r), "names two instantiations") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	var tab instTable
	tab.register("main.id", "$Int64", []typecheck.Shape{shpBase("Int64")})
	tab.register("main.id", "$Int64", []typecheck.Shape{shpBase("String")})
}

// TestDeclIndexKeysEveryDeclaration pins the index pass one builds: every
// declaration node of every module, under the key its symbols spell.
// Foreign blocks are walked too — an opaque record is a declaration a
// type argument can name even though it emits no symbol of its own.
func TestDeclIndexKeysEveryDeclaration(t *testing.T) {
	src := "record Point { x: Int64 }\n\npub type Shape = Dot | Line\n\npub type Meters = Int64\n\npub interface Sized {\n    fn size(self) -> Int64\n}\n\nforeign \"c\" {\n    byres record File { }\n}\n"
	f, d, ni := parser.Parse("main.we", []byte(src))
	if d != nil || ni != nil {
		t.Fatalf("parse failed: %v %+v", d, ni)
	}
	keys := declIndex([]ProgModule{{Key: "main", ID: "demo", File: f}})

	want := map[string]string{
		"Point":  "main.Point",
		"Shape":  "main.Shape",
		"Meters": "main.Meters",
		"Sized":  "main.Sized",
		"File":   "main.File",
	}
	for _, it := range f.Items {
		var name string
		switch x := it.(type) {
		case *ast.RecordDecl:
			name = x.Name
		case *ast.SumDecl:
			name = x.Name
		case *ast.NewtypeDecl:
			name = x.Name
		case *ast.InterfaceDecl:
			name = x.Name
		case *ast.ForeignBlock:
			// The block's opaque records are declared inside it, never at
			// the module's top level, and they key all the same.
			for _, y := range x.Items {
				r, ok := y.(*ast.RecordDecl)
				if !ok {
					continue
				}
				if got, ok := keys[r]; !ok || got != "main."+r.Name {
					t.Fatalf("opaque record %s keyed as %q (present=%v)", r.Name, got, ok)
				}
			}
		}
		if name == "" {
			continue
		}
		got, ok := keys[it]
		if !ok {
			t.Fatalf("declaration %s is not in the index", name)
		}
		if got != want[name] {
			t.Fatalf("declaration %s keyed as %q, want %q", name, got, want[name])
		}
	}
	if len(keys) != len(want) {
		t.Fatalf("the index holds %d declarations, want %d", len(keys), len(want))
	}
}

// TestMangleCheckerShapes is the bridge the hand-built cases cannot be:
// a real check's registry, mangled. Every argument the checker resolved
// for a generic application is a form the mangler encodes — no site
// carries a position left open (the mangler's contract), and the
// ctor's argument is the module's own Point, not its bare name.
func TestMangleCheckerShapes(t *testing.T) {
	src := "record Point { x: Int64 }\n\nrecord Wrap<T> { v: T }\n\nfn id<U>(x: U) -> U {\n    return x\n}\n\nfn probe() -> Int64 {\n    let w = Wrap<Point> { v: Point { x: 1 } }\n    return id<Int64>(w.v.x)\n}\n"
	f, d, ni := parser.Parse("main.we", []byte(src))
	if d != nil || ni != nil {
		t.Fatalf("parse failed: %v %+v", d, ni)
	}
	td, tni, sh := typecheck.Check(f, "main.we", typecheck.SingleFile)
	if td != nil || tni != nil {
		t.Fatalf("check failed: %v %+v", td, tni)
	}
	e := mangleFixture(ProgModule{Key: "main", ID: "demo", File: f})

	sites, seen := 0, map[string]bool{}
	walkExprs(f, func(x ast.Expr) {
		site, ok := sh.At(x)
		if !ok {
			return
		}
		sites++
		args := make([]typecheck.Shape, 0, len(site.Args))
		for _, a := range site.Args {
			args = append(args, a.Shape)
		}
		seen[e.mangleSuffix(args)] = true
	})
	if sites == 0 {
		t.Fatal("the check registered no site for a program that instantiates twice")
	}
	// The construction of Wrap<Point>: its argument is this module's Point.
	if !seen["$main.Point"] {
		t.Fatalf("the ctor's binding did not mangle to $main.Point; encodings seen: %v", keysOf(seen))
	}
	// The call of id<Int64>: the suffix is what its symbol carries.
	if !seen["$Int64"] {
		t.Fatalf("the call's binding did not mangle to $Int64; encodings seen: %v", keysOf(seen))
	}
}

// TestMangledSymbolsLinkThroughClang is the alphabet's own check: the
// symbol forms the mangler produces are spelled into a module's IR
// unquoted, so the pinned toolchain has to accept them, link them, and
// run them. It is what can be said at the naming layer — the end-to-end
// `we build` of a program's own instantiations arrives with the emission
// that registers them.
func TestMangledSymbolsLinkThroughClang(t *testing.T) {
	mods := []ProgModule{recordMod("main", "Point", "Wrap")}
	e := mangleFixture(mods...)
	wrapD := typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: declOf(t, mods[0], "Wrap")}
	pt := shpNominal(typecheck.ShapeDecl{Kind: typecheck.DeclRecord, Node: declOf(t, mods[0], "Point")})

	syms := []string{
		// The form D2 writes out: a declaration applied to a declaration.
		(&fnDef{key: "main", name: "id", suffix: e.mangleSuffix([]typecheck.Shape{shpNominal(wrapD, pt)})}).sym(),
		(&fnDef{key: "main", name: "id", suffix: e.mangleSuffix([]typecheck.Shape{pt})}).sym(),
		(&fnDef{key: "main", name: "id", suffix: e.mangleSuffix([]typecheck.Shape{
			{Kind: typecheck.ShapeTuple, Elems: []typecheck.Shape{shpBase("Int64"), pt}},
		})}).sym(),
		(&fnDef{key: "main", name: "id", suffix: e.mangleSuffix([]typecheck.Shape{
			{Kind: typecheck.ShapeFn, Params: []typecheck.Shape{shpBase("Int64")}, Ret: &pt},
		})}).sym(),
		(&fnDef{key: "main", name: "id", suffix: e.mangleSuffix([]typecheck.Shape{
			{Kind: typecheck.ShapeDyn, Decl: typecheck.ShapeDecl{Kind: typecheck.DeclIface, Name: "Iterator"}, Args: []typecheck.Shape{shpBase("Int64")}},
		})}).sym(),
	}

	var b strings.Builder
	b.WriteString("; ModuleID = 'mangle'\n\n")
	for i, s := range syms {
		if strings.ContainsAny(s, "@%\";") {
			t.Fatalf("symbol %q carries a character the IR text would read as syntax", s)
		}
		fmt.Fprintf(&b, "define i64 @%s(i64 %%x) {\nentry:\n  %%r = add i64 %%x, %d\n  ret i64 %%r\n}\n\n", s, i+1)
	}
	// main calls each define and sums them: a linked binary returns zero
	// only where every call reached the define under its own name.
	b.WriteString("define i32 @main() {\nentry:\n")
	want := 0
	for i, s := range syms {
		arg := 10 * (i + 1)
		fmt.Fprintf(&b, "  %%v%d = call i64 @%s(i64 %d)\n", i, s, arg)
		want += arg + i + 1
	}
	acc := "%v0"
	for i := 1; i < len(syms); i++ {
		fmt.Fprintf(&b, "  %%s%d = add i64 %s, %%v%d\n", i, acc, i)
		acc = fmt.Sprintf("%%s%d", i)
	}
	fmt.Fprintf(&b, "  %%ok = icmp eq i64 %s, %d\n  %%e = select i1 %%ok, i32 0, i32 1\n  ret i32 %%e\n}\n", acc, want)

	dir := t.TempDir()
	ll := filepath.Join(dir, "mangle.ll")
	if err := os.WriteFile(ll, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "mangle")
	if out, err := exec.Command(pinnedClang(t), "-o", bin, ll).CombinedOutput(); err != nil {
		t.Fatalf("clang rejected the mangled symbols: %v\n%s\nIR:\n%s", err, out, b.String())
	}
	if out, err := exec.Command(bin).CombinedOutput(); err != nil {
		t.Fatalf("the linked binary failed: %v\n%s", err, out)
	}
	// The control: the same toolchain rejects a character outside the
	// alphabet, so what the run above shows is the rule being met, not the
	// rule being absent.
	bad := filepath.Join(dir, "bad.ll")
	if err := os.WriteFile(bad, []byte("define i32 @a#b() {\nentry:\n  ret i32 0\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(pinnedClang(t), "-c", "-o", filepath.Join(dir, "bad.o"), bad).CombinedOutput(); err == nil {
		t.Fatalf("the toolchain accepted a symbol outside the alphabet:\n%s", out)
	}
}

// pinnedClang skips the test where the pinned toolchain is absent — the
// gate internal/cli's build face applies, restated here because the CLI
// imports this package and cannot be imported back.
func pinnedClang(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("clang", "--version").Output()
	if err != nil || !strings.Contains(string(out), "clang version 21.1.8") {
		t.Skipf("pinned clang unavailable: %v", err)
	}
	return "clang"
}

// walkExprs visits every expression of a tree's fn bodies — the registry
// keys call, construction, and box nodes, and a fixture long enough to
// instantiate holds them in bodies.
func walkExprs(f *ast.File, visit func(ast.Expr)) {
	for _, it := range f.Items {
		fn, ok := it.(*ast.FnDecl)
		if !ok {
			continue
		}
		walkStmts(fn.Body.Items, visit)
	}
}

func walkStmts(items []ast.Stmt, visit func(ast.Expr)) {
	for _, st := range items {
		switch x := st.(type) {
		case *ast.Binding:
			walkExpr(x.Init, visit)
		case *ast.Return:
			if x.HasValue {
				walkExpr(x.Value, visit)
			}
		case *ast.ExprStmt:
			walkExpr(x.Expr, visit)
		}
	}
}

// walkExpr visits one expression and the subexpressions a fixture
// reaches: enough to meet every site the registry keys.
func walkExpr(x ast.Expr, visit func(ast.Expr)) {
	if x == nil {
		return
	}
	visit(x)
	switch e := x.(type) {
	case *ast.Call:
		walkExpr(e.Fn, visit)
		for _, a := range e.Args {
			walkExpr(a, visit)
		}
	case *ast.Member:
		walkExpr(e.Recv, visit)
	case *ast.Construct:
		walkExpr(e.Base, visit)
		for _, fi := range e.Fields {
			walkExpr(fi.Value, visit)
		}
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
