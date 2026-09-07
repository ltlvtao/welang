package codegen

import (
	"strings"
	"testing"

	"github.com/ltlvtao/welang/internal/ast"
)

// M9b widens the M8 straight-line emitter into the two-layer concurrent
// set (design D8/D9): main bodies and task bodies share one emitter, and
// every concurrent form lands on the runtime ABI family pinned here by
// name. The boundary Whats get their v2 vocabulary — bndMainBody reworded,
// bndTaskBody and bndCallbackBody new — so a future emitter case cannot
// silently swallow a form ("explicit case, never vanish", the M4
// discipline). Red today: the v2 words do not exist and the emitter stops
// at the M8 vocabulary.

// m9bModule wraps main-shaped statements with both std imports (io and
// concurrent) and the skeleton error sum.
func m9bModule(stmts ...ast.Stmt) *ast.File {
	return &ast.File{Items: []ast.Item{
		stdIoImport("io"),
		&ast.Import{Path: []string{"std", "concurrent"}, Alias: "conc"},
		appError(),
		mainDecl(stmts...),
	}}
}

func intLit(text string) *ast.Literal { return &ast.Literal{Kind: "int", Text: text} }

func strLit(text string) *ast.Literal { return &ast.Literal{Kind: "string", Text: text} }

func ident(name string) *ast.Ident { return &ast.Ident{Name: name} }

func binOp(op string, l, r ast.Expr) *ast.Binary {
	return &ast.Binary{Op: op, L: l, R: r}
}

// callOn builds `recv.name(args…)`.
func callOn(recv ast.Expr, name string, args ...ast.Expr) *ast.Call {
	return &ast.Call{Fn: &ast.Member{Recv: recv, Name: name}, Args: args}
}

// chanBind builds `let name: conc.Channel<Int64> = conc.channel(cap)`.
func chanBind(name, capText string) *ast.Binding {
	return &ast.Binding{
		Kw: "let", Name: name,
		Typ:  &ast.NamedType{Qual: "conc", Name: "Channel", Args: []ast.TypeRef{&ast.NamedType{Name: "Int64"}}},
		Init: &ast.Call{Fn: &ast.Member{Recv: ident("conc"), Name: "channel"}, Args: []ast.Expr{intLit(capText)}},
	}
}

// taskIo builds `task effect io { stmts }`.
func taskIo(stmts ...ast.Stmt) *ast.TaskExpr {
	return &ast.TaskExpr{
		EffectTags: []string{"io"}, EffectLine: 1, EffectCol: 6,
		Body: ast.Block{Items: stmts},
	}
}

// blockOf wraps statements in the BlockExpr an arm or clause body needs.
func blockOf(stmts ...ast.Stmt) *ast.BlockExpr {
	return &ast.BlockExpr{Block: ast.Block{Items: stmts}}
}

// assertClean pins that emission finished and the IR carries every
// required substring (the ABI face — stable across emitter internals).
func assertClean(t *testing.T, f *ast.File, want ...string) string {
	t.Helper()
	ir, ni := Emit(f, "demo")
	if ni != nil {
		t.Fatalf("expected clean emission, got boundary %q", ni.What)
	}
	for _, w := range want {
		if !strings.Contains(ir, w) {
			t.Fatalf("IR missing %q:\n%s", w, ir)
		}
	}
	return ir
}

func TestM9bScalarAndControl(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "var", Name: "i", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("0")},
		&ast.Binding{Kw: "var", Name: "total", Typ: &ast.NamedType{Name: "Int64"}, Init: intLit("0")},
		&ast.While{
			Cond: binOp("<", ident("i"), intLit("5")),
			Body: ast.Block{Items: []ast.Stmt{
				&ast.Assign{Name: "total", Value: binOp("+", ident("total"), ident("i"))},
				&ast.Assign{Name: "i", Value: binOp("+", ident("i"), intLit("1"))},
			}},
		},
		&ast.ExprStmt{Expr: &ast.If{
			Cond: binOp("==", ident("total"), intLit("10")),
			Then: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"ten"`))}},
			Else: blockOf(ioCall("io", "println", strLit(`"other"`))),
		}},
		okReturn(),
	), "add", "icmp", "br i1")
}

func TestM9bPrimCtorCall(t *testing.T) {
	cases := []struct {
		name string
		file *ast.File
		abi  string
	}{
		{"mutex ctor", m9bModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()}, okReturn()), "__we_prim_new_mutex"},
		{"rwlock ctor", m9bModule(
			&ast.Binding{Kw: "let", Name: "rw", Init: &ast.Call{
				Fn: &ast.Member{Recv: ident("conc"), Name: "RwLock"}, Args: []ast.Expr{intLit("1")},
			}}, okReturn()), "__we_prim_new_rwlock"},
		{"atomic ctor", m9bModule(
			&ast.Binding{Kw: "let", Name: "a", Init: &ast.Call{
				Fn: &ast.Member{Recv: ident("conc"), Name: "Atomic"}, Args: []ast.Expr{&ast.Literal{Kind: "bool", Text: "true"}},
			}}, okReturn()), "__we_prim_new_atomic"},
		{"semaphore ctor", m9bModule(
			&ast.Binding{Kw: "let", Name: "s", Init: &ast.Call{
				Fn: &ast.Member{Recv: ident("conc"), Name: "Semaphore"}, Args: []ast.Expr{intLit("1")},
			}}, okReturn()), "__we_prim_new_sem"},
		{"channel ctor", m9bModule(chanBind("ch", "2"), okReturn()), "__we_prim_new_chan"},
		{"cond ctor", m9bModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
			&ast.Binding{Kw: "let", Name: "c", Init: &ast.Call{
				Fn: &ast.Member{Recv: ident("conc"), Name: "Cond"}, Args: []ast.Expr{ident("m")},
			}}, okReturn()), "__we_prim_new_cond"},
		{"update call", m9bModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
			&ast.Binding{Kw: "let", Name: "n", Init: callOn(ident("m"), "update", &ast.Closure{
				Params: []ast.Param{{Name: "v"}}, Short: true,
				Body: ast.Block{Items: []ast.Stmt{
					&ast.ExprStmt{Expr: binOp("+", ident("v"), intLit("1"))},
				}},
			})}, okReturn()), "__we_prim_update"},
		{"set get calls", m9bModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
			&ast.ExprStmt{Expr: callOn(ident("m"), "set", intLit("7"))},
			&ast.Binding{Kw: "let", Name: "n", Init: callOn(ident("m"), "get")}, okReturn()), "__we_prim_set"},
		{"read call", m9bModule(
			&ast.Binding{Kw: "let", Name: "rw", Init: &ast.Call{
				Fn: &ast.Member{Recv: ident("conc"), Name: "RwLock"}, Args: []ast.Expr{intLit("1")},
			}},
			&ast.Binding{Kw: "let", Name: "u", Init: callOn(ident("rw"), "read", &ast.Closure{
				Params: []ast.Param{{Name: "v"}}, Short: true,
				Body: ast.Block{Items: []ast.Stmt{
					&ast.ExprStmt{Expr: binOp("*", ident("v"), intLit("2"))},
				}},
			})}, okReturn()), "__we_prim_read"},
		{"semaphore calls", m9bModule(
			&ast.Binding{Kw: "let", Name: "s", Init: &ast.Call{
				Fn: &ast.Member{Recv: ident("conc"), Name: "Semaphore"}, Args: []ast.Expr{intLit("1")},
			}},
			&ast.ExprStmt{Expr: callOn(ident("s"), "acquire")},
			&ast.Binding{Kw: "let", Name: "g", Init: callOn(ident("s"), "tryAcquire")},
			&ast.ExprStmt{Expr: callOn(ident("s"), "release")},
			&ast.Binding{Kw: "let", Name: "c", Init: callOn(ident("s"), "currentCount")}, okReturn()), "__we_sem_acquire"},
		{"channel calls", m9bModule(
			chanBind("ch", "2"),
			&ast.ExprStmt{Expr: callOn(ident("ch"), "send", intLit("1"))},
			&ast.Binding{Kw: "let", Name: "v", Init: callOn(ident("ch"), "receive")},
			&ast.Binding{Kw: "let", Name: "t", Init: callOn(ident("ch"), "trySend", intLit("2"))},
			&ast.Binding{Kw: "let", Name: "r", Init: callOn(ident("ch"), "tryReceive")},
			&ast.ExprStmt{Expr: callOn(ident("ch"), "close")}, okReturn()), "__we_chan_send"},
		{"handle calls", m9bModule(
			&ast.Binding{Kw: "let", Name: "t", Init: taskIo()},
			&ast.Binding{Kw: "let", Name: "r", Init: callOn(ident("t"), "await")},
			&ast.ExprStmt{Expr: callOn(ident("t"), "cancel")}, okReturn()), "__we_handle_await"},
		{"cond calls", m9bModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
			&ast.Binding{Kw: "let", Name: "c", Init: &ast.Call{
				Fn: &ast.Member{Recv: ident("conc"), Name: "Cond"}, Args: []ast.Expr{ident("m")},
			}},
			&ast.ExprStmt{Expr: callOn(ident("c"), "signal")},
			&ast.ExprStmt{Expr: callOn(ident("c"), "broadcast")}, okReturn()), "__we_cond_signal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertClean(t, c.file, c.abi)
		})
	}
}

func TestM9bSumSlots(t *testing.T) {
	assertClean(t, m9bModule(
		chanBind("ch", "1"),
		&ast.Binding{Kw: "let", Name: "v", Init: callOn(ident("ch"), "receive")},
		&ast.ExprStmt{Expr: &ast.Match{
			Scrutinee: ident("v"),
			Arms: []ast.MatchArm{
				{Pat: &ast.PatVariant{Name: "Some", Args: []ast.Pattern{&ast.PatBinding{Name: "x"}}},
					Body: blockOf(ioCall("io", "println", ident("x")))},
				{Pat: &ast.PatVariant{Name: "None"},
					Body: blockOf(ioCall("io", "println", strLit(`"none"`)))},
			},
		}},
		okReturn(),
	), "__we_chan_recv", "icmp")
}

func TestM9bTaskThunk(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "t", Init: taskIo(ioCall("io", "println", strLit(`"hi"`)))},
		&ast.Binding{Kw: "let", Name: "r", Init: callOn(ident("t"), "await")},
		okReturn(),
	), "__we_task_new", "__we_root_push")
	if strings.Count(ir, "define") < 2 {
		t.Fatalf("expected a separate thunk definition alongside main:\n%s", ir)
	}
}

func TestM9bScopeForms(t *testing.T) {
	task := func() *ast.TaskExpr { return taskIo(ioCall("io", "println", strLit(`"t"`))) }
	cases := []struct {
		name string
		file *ast.File
	}{
		{"plain scope", m9bModule(&ast.ExprStmt{Expr: &ast.ScopeExpr{
			Body: ast.Block{Items: []ast.Stmt{
				&ast.Binding{Kw: "let", Name: "t", Init: task()},
				&ast.Binding{Kw: "let", Name: "_", Init: callOn(ident("t"), "await")},
			}},
		}}, okReturn())},
		{"timeout scope", m9bModule(
			&ast.Binding{Kw: "let", Name: "r", Init: &ast.ScopeExpr{
				Timeout: intLit("5"),
				Body: ast.Block{Items: []ast.Stmt{
					&ast.Binding{Kw: "let", Name: "t", Init: task()},
					&ast.Binding{Kw: "let", Name: "_", Init: callOn(ident("t"), "await")},
				}},
			}}, okReturn())},
		{"collectAll scope", m9bModule(&ast.ExprStmt{Expr: &ast.ScopeExpr{
			CollectAll: true,
			Body: ast.Block{Items: []ast.Stmt{
				&ast.Binding{Kw: "let", Name: "t", Init: task()},
				&ast.Binding{Kw: "let", Name: "_", Init: callOn(ident("t"), "await")},
			}},
		}}, okReturn())},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertClean(t, c.file, "__we_scope_enter", "__we_scope_leave")
		})
	}
}

func TestM9bSelectForms(t *testing.T) {
	sel := func() *ast.SelectExpr {
		src := func(ch string) ast.Expr { return callOn(ident(ch), "receive") }
		return &ast.SelectExpr{Cases: []ast.SelectCase{
			{Name: "x", Source: src("a"), Body: intLit("1"), Line: 1, Col: 5},
			{Name: "y", Source: src("b"), Body: intLit("2"), Line: 2, Col: 5},
		}}
	}
	cases := []struct {
		name string
		file *ast.File
	}{
		{"select statement", m9bModule(chanBind("a", "1"), chanBind("b", "1"),
			&ast.ExprStmt{Expr: sel()}, okReturn())},
		{"select value", m9bModule(chanBind("a", "1"), chanBind("b", "1"),
			&ast.Binding{Kw: "let", Name: "v", Init: sel()}, okReturn())},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertClean(t, c.file, "__we_select_new", "__we_select_add_recv", "__we_select_park")
		})
	}
}

func TestM9bQuestionEmission(t *testing.T) {
	assertClean(t, m9bModule(
		&ast.Binding{Kw: "let", Name: "t", Init: taskIo(ioCall("io", "println", strLit(`"hi"`)))},
		&ast.ExprStmt{Expr: &ast.Prop{X: callOn(ident("t"), "await")}},
		okReturn(),
	), "__we_handle_await", "br i1")
}

func TestM9bDeferOrder(t *testing.T) {
	ir := assertClean(t, m9bModule(
		&ast.Defer{Block: ast.Block{Items: []ast.Stmt{ioCall("io", "println", strLit(`"deferred"`))}}},
		ioCall("io", "println", strLit(`"body"`)),
		okReturn(),
	))
	// Deferred emission is inverted to the scope's end: the body's
	// constant enters the pool (emission order) before the defer's.
	if strings.Index(ir, "body") >= strings.Index(ir, "deferred") {
		t.Fatalf("defer not inverted to scope end:\n%s", ir)
	}
}

func TestM9bBoundaryWhats(t *testing.T) {
	fnCall := &ast.ExprStmt{Expr: &ast.Call{Fn: &ast.Ident{Name: "go"}}}
	cases := []struct {
		name string
		file *ast.File
		what string
	}{
		{"main user fn call", m9bModule(fnCall, okReturn()), bndMainBody},
		{"task body user fn call", m9bModule(
			&ast.Binding{Kw: "let", Name: "t", Init: &ast.TaskExpr{
				EffectTags: []string{"io"}, EffectLine: 1, EffectCol: 6,
				Body: ast.Block{Items: []ast.Stmt{fnCall}},
			}}, okReturn()), bndTaskBody},
		{"callback body io call", m9bModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
			&ast.Binding{Kw: "let", Name: "n", Init: callOn(ident("m"), "update", &ast.Closure{
				Params: []ast.Param{{Name: "v"}}, Short: true,
				Body: ast.Block{Items: []ast.Stmt{ioCall("io", "println", ident("v"))}},
			})}, okReturn()), bndCallbackBody},
		{"callback body user fn call", m9bModule(
			&ast.Binding{Kw: "let", Name: "m", Init: mutexCtor()},
			&ast.Binding{Kw: "let", Name: "n", Init: callOn(ident("m"), "update", &ast.Closure{
				Params: []ast.Param{{Name: "v"}}, Short: true,
				Body: ast.Block{Items: []ast.Stmt{fnCall}},
			})}, okReturn()), bndCallbackBody},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ni := Emit(c.file, "demo")
			if ni == nil {
				t.Fatalf("expected boundary %q, got none", c.what)
			}
			if ni.What != c.what {
				t.Fatalf("boundary What %q, want %q", ni.What, c.what)
			}
		})
	}
}
