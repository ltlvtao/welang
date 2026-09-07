// Package ast holds the syntax tree of chapters 2–6, 8, 10–12, and 17: one file
// is one module of top-level items (chapter 6), each fn body a block of
// statements (chapter 2) with chapter 3's control-flow statements, each
// expression built from the skeleton's primary, postfix, unary, and binary
// layers with the closed 12-level precedence table — the primaries include
// chapter 3's if, chapter 4's match, chapter 8's construction and tuple
// expressions, and chapter 12's closures, with chapter 4's patterns as their
// own grammar position. Nodes are pure data carrying 1-based source
// positions (line, column) for the stages that follow; grouping parentheses
// fold away — grouping is binding.
package ast

// File is one source file: one module (chapter 6's file structure).
type File struct {
	Items []Item
	// Docs records the /// documentation units in source order and the index
	// of the top-level item each attaches to (chapter 6's attachment rule).
	Docs []DocAttach
}

// DocAttach is one /// unit and the item it documents.
type DocAttach struct {
	StartLine int
	EndLine   int
	Lines     []string
	Item      int // index into File.Items
}

// Item is one top-level item of the module.
type Item interface{ item() }

// Import is `import path` or `import path as name` (chapter 6). Path holds
// the dotted segments; Alias is empty without `as`.
type Import struct {
	Path      []string
	Alias     string
	Line, Col int // at the import keyword
	// PathLine/PathCol sit at the FIRST path segment — the module's own
	// token, where the module-resolution diagnostics anchor.
	PathLine int
	PathCol  int
}

// FnDecl is a fn declaration: `[pub] fn name<T…>(params) [-> type] [where …]
// block`. Ret nil means no declared return type — the function produces no
// value. The chapter 10 clauses attach here: TypeParams between the name and
// the parameter list, Where between the return annotation and the body. An
// impl-block method definition reuses this node — Recv then carries its
// receiver (RecvNone on a plain fn).
type FnDecl struct {
	Pub        bool
	Name       string
	TypeParams []*TypeParam // chapter 10 generic clause; empty without
	Recv       RecvKind     // chapter 10 method receiver; RecvNone on a plain fn
	Params     []Param      // excludes the receiver
	Ret        TypeRef
	Where      []*WhereBound // chapter 10 where clause trailing the signature
	EffectTags []string      // chapter 16 segment between the parameters and -> / body; nil = pure
	EffectLine int           // at the segment's first tag (the discipline anchor)
	EffectCol  int
	Body       Block
	Line, Col  int // at fn (or pub)
	NameLine   int
	NameCol    int
}

// Param is one `name: type` pair of a parameter list.
type Param struct {
	Name     string
	Type     TypeRef
	NameLine int
	NameCol  int
}

// TopLet is a top-level binding: `[pub] let name [: type] = expr`.
type TopLet struct {
	Pub       bool
	Binding   Binding
	Line, Col int // at let (or pub)
}

// SumDecl is a sum type declaration (chapter 9): `[pub] [byval] type
// Name<T…> = V1 | ... | Vn [derives …]`, n at least one, each variant bare
// (a unit variant) or carrying 1..8 payload type references. The chapter 10
// clauses attach after the name (TypeParams) and after the last variant on
// the declaration's last line (Derives).
type SumDecl struct {
	Pub        bool
	Byval      bool
	Name       string
	TypeParams []*TypeParam // chapter 10 generic clause; empty without
	Variants   []Variant
	Derives    *DerivesClause // chapter 10 derives clause; nil without
	Line, Col  int            // at type (or byval/pub)
	NameLine   int
	NameCol    int
}

// Variant is one variant of a sum declaration: the bare name alone, or the
// name with its payload type references.
type Variant struct {
	Name      string
	Payload   []TypeRef
	Line, Col int // at the variant name
}

// RecordDecl is a record declaration (chapter 8): `[pub] [gc|byval|byres]
// record Name<T…> { fields } [derives …]`. Cat is "gc" (the default
// category), "value" (byval), or "resource" (byres). Zero fields are legal.
// The chapter 10 clauses attach after the name (TypeParams) and after the
// closing brace on the declaration's last line (Derives).
type RecordDecl struct {
	Pub        bool
	Cat        string // "gc" | "value" | "resource"
	Name       string
	TypeParams []*TypeParam // chapter 10 generic clause; empty without
	Fields     []FieldDecl
	Derives    *DerivesClause // chapter 10 derives clause; nil without
	Line, Col  int            // at record (or the outermost prefix)
	NameLine   int
	NameCol    int
}

// FieldDecl is one `name: type` field of a record declaration.
type FieldDecl struct {
	Name      string
	Typ       TypeRef
	Line, Col int // at the field name
}

// NewtypeDecl is `newtype Name<T…>(Underlying) [derives …]` (chapter 8): a
// zero-cost wrapper whose layout is erased. The chapter 10 clauses attach
// after the name (TypeParams) and after the closing paren on the
// declaration's last line (Derives).
type NewtypeDecl struct {
	Pub        bool
	Name       string
	TypeParams []*TypeParam // chapter 10 generic clause; empty without
	Underlying TypeRef
	Derives    *DerivesClause // chapter 10 derives clause; nil without
	Line, Col  int            // at newtype (or pub)
	NameLine   int
	NameCol    int
}

// --- chapter 10: interfaces, impls, generics ---------------------------------

// RecvKind is the receiver form of a method's parameter list: absent (a
// plain fn), `self`, or `mut self`. The zero value is RecvNone, so plain fn
// declarations carry it unset.
type RecvKind string

const (
	RecvNone    RecvKind = ""         // no receiver
	RecvSelf    RecvKind = "self"     // self
	RecvMutSelf RecvKind = "mut self" // mut self
)

// TypeParam is one PascalCase name of a generic parameter clause
// `<T1, …, Tk>` (chapter 10), k at most eight — at fn, record, newtype, sum,
// interface, and impl declarations and at method heads.
type TypeParam struct {
	Name      string
	Line, Col int // at the name
}

// InterfaceDecl is `[pub] interface Name<T…> { items }` (chapter 10). Items
// stand one per line, separated by inferred boundaries with no separator
// token: associated-type holes and method signatures, the latter optionally
// with a default body. Interface members carry no pub of their own.
type InterfaceDecl struct {
	Pub        bool
	Name       string
	TypeParams []*TypeParam
	Assocs     []*AssocDecl
	Methods    []MethodSig
	Line, Col  int // at interface (or pub)
	NameLine   int
	NameCol    int
}

// AssocDecl is one associated-type hole of an interface: `type Item`
// (chapter 10). A hole carries no bound on its declaration — bounds live in
// where clauses (E0804).
type AssocDecl struct {
	Name      string
	Line, Col int // at the name
}

// MethodSig is one method of an interface: a signature `fn name<T…>(recv,
// params…) [-> type]`, or the same with a default body (chapter 10). Params
// excludes the receiver — Recv carries it alone. Signature names join the
// module's one name space.
type MethodSig struct {
	Name       string
	TypeParams []*TypeParam
	Recv       RecvKind
	Params     []Param
	Ret        TypeRef
	HasRet     bool
	EffectTags []string // chapter 16 segment; nil = pure
	EffectLine int      // at the segment's first tag
	EffectCol  int
	Body       *Block // nil on a bare signature
	NameLine   int
	NameCol    int
}

// ImplDecl is `impl<T…> [Iface for] Head [where …] { items }` (chapter 10)
// — the for-form, or the inherent form with Iface nil. Iface and Head are
// full type references (a tuple head parses; its rejection is the checker's
// E0811). Items are associated-type bindings (all before any method) and
// method definitions, one per line; an impl block carries no pub.
type ImplDecl struct {
	TypeParams []*TypeParam
	Iface      TypeRef // nil on the inherent form
	Head       TypeRef
	Where      []*WhereBound
	Assocs     []*AssocBinding
	Methods    []*FnDecl
	Line, Col  int // at impl
}

// AssocBinding is one `type Name = TypeRef` of an impl: its binding of an
// interface associated type (chapter 10). Bindings precede every method
// definition (E0806).
type AssocBinding struct {
	Name      string
	Type      TypeRef
	Line, Col int // at the name
}

// WhereBound is one constraint of a where clause (chapter 10): the bound
// form `Subject: Iface[ + Iface2 …]` or the equality form `Subject.Assoc ==
// TypeRef`. One WhereBound holds one form — the bound list or the equality
// list, never both.
type WhereBound struct {
	Subject   string
	Ifaces    []*NamedType // the bound form; empty on the equality form
	Eq        []*TypeEq    // the equality form; empty on the bound form
	Line, Col int          // at the subject name
}

// TypeEq is one `Subject.Assoc == TypeRef` equality of a where clause; the
// right side is a concrete type (a generic parameter there is E0831).
type TypeEq struct {
	Assoc     string
	RHS       TypeRef
	Line, Col int // at ==
}

// DerivesClause is `derives Eq, Hash, Show` — the tail of a record, newtype,
// or sum declaration's last line (chapter 10). The target set is closed and
// duplicate-free (E0824).
type DerivesClause struct {
	Targets   []string
	Line, Col int // at derives
}

// Block is `{ items }` — itself an expression whose value is its final
// expression item (chapter 2's Blocks and block value).
type Block struct {
	Items     []Stmt
	Line, Col int // at {
}

// Stmt is one block item: a binding, an assignment, a return, or an
// expression statement (chapter 2's Statements).
type Stmt interface{ stmt() }

// Binding is `let|var name [: type] = expr` (Name "_" discards the value)
// or, with Pat set, `let|var (p1, …, pn) = expr` — an irrefutable tuple
// pattern of bindings and wildcards (chapter 8); Name is empty then.
type Binding struct {
	Kw        string // "let" | "var"
	Name      string
	Pat       Pattern // nil for the plain name form
	Typ       TypeRef
	Init      Expr
	Line, Col int // at the keyword
	NameLine  int
	NameCol   int
}

// Assign is `name = expr` — the statement, never an expression. With Field
// set it is `self.field = expr`, the receiver field write (chapter 10's one
// field-write form, legal only inside a mut self method body).
type Assign struct {
	Name      string
	Field     string // empty on a plain name; the field of a self write
	Value     Expr
	Line, Col int // at the name (self on the field form)
	// OpLine/OpCol sit at the `=` token — chapter 13's rebound diagnostic
	// (E1105) anchors at the operator, not at the target name.
	OpLine, OpCol int
}

// Return is `return` or `return expr` (chapter 6's function bodies).
type Return struct {
	HasValue  bool
	Value     Expr
	Line, Col int // at return
}

// ExprStmt wraps any expression as a block item.
type ExprStmt struct {
	Expr      Expr
	Line, Col int
}

// While is `while cond block` (chapter 3).
type While struct {
	Cond      Expr
	Body      Block
	Line, Col int // at while
}

// Loop is `loop block` (chapter 3).
type Loop struct {
	Body      Block
	Line, Col int // at loop
}

// Break leaves, Continue restarts, the innermost enclosing while or loop
// (chapter 3).
type Break struct{ Line, Col int }

type Continue struct{ Line, Col int }

// Defer is `defer block` — a direct item of a function body's block, run at
// function exit in reverse order (chapter 3).
type Defer struct {
	Block     Block
	Line, Col int // at defer
}

// ForStmt is `for pat in expr block` (chapters 5 and 11): the head pattern
// is irrefutable (a binding, the wildcard, or a tuple of them) and the
// iterated expression implements Iterable.
type ForStmt struct {
	Pat       Pattern
	Iter      Expr
	Body      Block
	Line, Col int // at for
}

// ScopeRes is `scope resource(name = expr, …) block` (chapter 13): each
// head binding takes over a resource handle and releases it at block
// exit. The binding carries no annotation slot — the head expression's
// type is the binding's type — and the names bind for the block.
type ScopeRes struct {
	Binds     []ScopeBind
	Body      Block
	Line, Col int // at scope
}

// ScopeBind is one `name = expr` of a scope resource head.
type ScopeBind struct {
	Name      string
	Val       Expr
	Line, Col int // at the name
}

// Prop is the `?` propagation suffix (chapter 14): level-1 postfix, so it
// chains with call and member access; the node anchors the `?` token.
type Prop struct {
	X         Expr
	Line, Col int // at ?
}

// Expr is one expression of the chapter 2 skeleton.
type Expr interface{ expr() }

// Ident names a value, function, module, or (PascalCase) type.
type Ident struct {
	Name      string
	Line, Col int
}

// Literal is one chapter 1 literal; Kind is int, float, string, rune, or
// bool (true/false). Text holds the source form — values are the types
// stage's to decode.
type Literal struct {
	Kind      string
	Text      string
	Line, Col int
}

// Unary is a prefix `!`, `-`, or `~` (precedence level 2).
type Unary struct {
	Op        string
	X         Expr
	Line, Col int // at the operator
}

// Binary is one binary operator application; Line/Col sit at the operator
// (the non-associative-level diagnostics anchor there).
type Binary struct {
	Op        string
	L, R      Expr
	Line, Col int
}

// Call is `expr(args)`; position at the opening parenthesis. TypeArgs holds
// an explicit generic clause of the bare-name call form `name<T…>(args)`
// (chapter 10) — the postfix angle-bracket lookahead, which never reaches a
// method head.
type Call struct {
	Fn        Expr
	Args      []Expr
	TypeArgs  []TypeRef
	Line, Col int
	// ArgLine/ArgCol sit at the closing `>` when an explicit generic clause
	// is present — the application's arity diagnostics (E0828) anchor there,
	// not at the call's `(`.
	ArgLine, ArgCol int
}

// Member is `receiver.name`; position at the dot.
type Member struct {
	Recv      Expr
	Name      string
	Line, Col int
	// NameLine/NameCol anchor the member name token (one past the dot) —
	// the resolution diagnostics (E0816) report there, not at the dot.
	NameLine, NameCol int
}

// BlockExpr is a block in expression position (a block is an expression).
type BlockExpr struct {
	Block     Block
	Line, Col int // at {
}

// Unit is the unit value `()` — the one value of the unit type (chapter 8's
// sliver of the composite chapter this slice implements).
type Unit struct {
	Line, Col int
}

// If is chapter 3's conditional expression: `if cond block [else
// block|if]`. Else nil marks the valueless statement form; an else-if chain
// nests (Else points at the inner If — `else { if … }` folds to the same
// shape); a plain else arm is a *BlockExpr.
type If struct {
	Cond      Expr
	Then      Block
	Else      Expr // nil | *BlockExpr | *If
	Line, Col int  // at if
}

// Match is chapter 4's match expression: `match scrutinee { arms }`.
type Match struct {
	Scrutinee Expr
	Arms      []MatchArm
	Line, Col int // at match
}

// MatchArm is `pattern [if cond] => body` — one arm. The body is one
// expression; a block body arrives as a *BlockExpr.
type MatchArm struct {
	Pat       Pattern
	Guard     Expr // nil without a guard
	Body      Expr
	Line, Col int // at the pattern's first token
}

// TaskExpr is chapter 18's task block: `task effect tag… block`, the
// handle-creating form. The effect segment is mandatory at parse (E1601);
// the body is a function-body context — return carries the block's value
// and the task's own declared set rules the body's calls (the checker's
// face).
type TaskExpr struct {
	EffectTags []string
	EffectLine int // at the segment's first tag (the tag-resolution anchor)
	EffectCol  int
	Body       Block
	Line, Col  int // at task
}

// ScopeExpr is chapter 18's compound scope: bare, timeout(n), collectAll,
// or timeout(n) collectAll. The block's value is the scope's value (the
// timeout forms wrap it in Result<T, TimeoutError>, the checker's face).
// The resource form is ScopeRes (chapter 13's, a statement).
type ScopeExpr struct {
	Timeout    Expr // nil without the clause
	CollectAll bool
	Body       Block
	Line, Col  int // at scope
}

// SelectExpr is chapter 18's select: two or more cases racing their wait
// sources; the taken case's body value is the expression's value.
type SelectExpr struct {
	Cases     []SelectCase
	Line, Col int // at select
}

// SelectCase is `case name = source => body`, or the wildcard `case _ =
// source => body`. The source is one of the four wait-source calls; the
// binding (when named) holds the source's yield for the body.
type SelectCase struct {
	Wildcard  bool
	Name      string
	Source    Expr
	Body      Expr
	Line, Col int // at case
}

// Construct is a record construction or update expression (chapter 8):
// `Head<T…> { field: value, … [with &base] }`. Base non-nil marks an update;
// TypeArgs holds the head's explicit generic clause (chapter 10).
type Construct struct {
	Qual      string // module qualifier, empty for a bare head
	Name      string
	TypeArgs  []TypeRef
	Fields    []FieldInit
	Base      Expr // nil for a full construction
	Line, Col int  // at the head's name token
	// ArgLine/ArgCol sit at the closing `>` of the explicit generic clause —
	// the application's arity diagnostics (E0828) anchor there.
	ArgLine, ArgCol int
}

// FieldInit is one `name: value` of a construction or update.
type FieldInit struct {
	Name      string
	Value     Expr
	Line, Col int // at the field name
}

// Tuple is `(e1, …, en)`, n at least 2 (a single group folds away into its
// operand; `()` is the Unit value).
type Tuple struct {
	Elems     []Expr
	Line, Col int // at (
}

// Closure is chapter 12's function value: the full form `fn(params) [->
// type] block`, or the short form `|p1, …, pk| body` whose single-expression
// body is normalized to a one-item block at parse time.
type Closure struct {
	Params    []Param // Type nil for a short form's bare parameter
	Ret       TypeRef // nil = produces no value
	Body      Block
	Short     bool
	Line, Col int // at fn or |
}

// ListLit is `[e1, …, en]` (chapter 17): n zero or more, trailing comma
// uniform, the brackets a paren region — the literal folds across lines.
type ListLit struct {
	Elems     []Expr
	Line, Col int // at [
}

// Pattern is one match pattern (chapter 4). The pattern grammar is a
// disjoint grammar position — patterns are neither expressions nor
// statements, and no construct mixes the two.
type Pattern interface{ pattern() }

// PatLiteral is one chapter-1 literal (literals carry no sign).
type PatLiteral struct {
	Kind      string // int | float | string | rune | bool
	Text      string
	Line, Col int
}

// PatWildcard is `_` — matches anything, binds nothing.
type PatWildcard struct{ Line, Col int }

// PatBinding binds one name to the matched value (or payload element).
type PatBinding struct {
	Name      string
	Line, Col int
}

// PatOr is a flat or-pattern `p1 | … | pn` (one node, any branch count).
type PatOr struct {
	Branches  []Pattern
	Line, Col int // at the first |
}

// PatTuple is a tuple pattern of two or more sub-patterns.
type PatTuple struct {
	Elems     []Pattern
	Line, Col int // at (
}

// PatVariant names a variant of the scrutinee's sum type, bare or with
// payload sub-patterns; Qualified marks the module-qualified form
// module.Name, with Qual carrying the module name.
type PatVariant struct {
	Qualified bool
	Qual      string // the module qualifier of the qualified form
	Name      string
	Args      []Pattern
	Line, Col int // at the variant name
}

// TypeRef is the form filling a type-annotation slot (chapter 7's type
// references): a named reference, a tuple, the unit type, or a fn type.
type TypeRef interface{ typeref() }

// NamedType is `Name`, `module.Name`, or a generic application
// `Name<T1, ..., Tk>` (Dyn<I> included — same shape). Args empty without a
// generic clause.
type NamedType struct {
	Qual      string // module segment, empty for a bare name
	Name      string
	Args      []TypeRef
	Line, Col int
	// ArgLine/ArgCol sit at the `<` when a generic clause is present —
	// the application token, where its arity diagnostic anchors.
	ArgLine int
	ArgCol  int
}

// TupleType is `(T1, ..., Tn)` with n at least 2 (chapter 7 defers the
// arity bound to chapter 8's milestone).
type TupleType struct {
	Elems     []TypeRef
	Line, Col int // at (
}

// UnitType is `()`.
type UnitType struct {
	Line, Col int
}

// EffectDecl is `effect name` (chapter 16): a top-level declaration that
// names one effect tag. It carries no body and no semantics beyond the
// name — the declaration makes the tag resolvable in segments within the
// module (and, with pub, through a qualified tag in an importing module).
type EffectDecl struct {
	Pub       bool
	Name      string
	Line, Col int // at effect (or pub)
	NameLine  int // at the name — the discipline anchor (E0012/E0404/E1403)
	NameCol   int
}

// FnType is `fn(T1, ..., Tn) [tags] -> T` (chapter 7's function type
// reference; EffectTags holds the bare segment when present).
type FnType struct {
	Params     []TypeRef
	EffectTags []string // chapter 16 segment; nil = pure
	TagLine    int      // at the segment's first tag
	TagCol     int
	Ret        TypeRef
	Line, Col  int
}

func (Import *Import) item()    {}
func (f *FnDecl) item()         {}
func (t *TopLet) item()         {}
func (s *SumDecl) item()        {}
func (r *RecordDecl) item()     {}
func (n *NewtypeDecl) item()    {}
func (i *InterfaceDecl) item()  {}
func (m *ImplDecl) item()       {}
func (e *EffectDecl) item()     {}
func (b *Binding) stmt()        {}
func (a *Assign) stmt()         {}
func (r *Return) stmt()         {}
func (e *ExprStmt) stmt()       {}
func (w *While) stmt()          {}
func (l *Loop) stmt()           {}
func (b *Break) stmt()          {}
func (c *Continue) stmt()       {}
func (d *Defer) stmt()          {}
func (f *ForStmt) stmt()        {}
func (s *ScopeRes) stmt()       {}
func (i *Ident) expr()          {}
func (l *Literal) expr()        {}
func (u *Unary) expr()          {}
func (b *Binary) expr()         {}
func (c *Call) expr()           {}
func (m *Member) expr()         {}
func (b *BlockExpr) expr()      {}
func (u *Unit) expr()           {}
func (i *If) expr()             {}
func (m *Match) expr()          {}
func (t *TaskExpr) expr()       {}
func (s *ScopeExpr) expr()      {}
func (s *SelectExpr) expr()     {}
func (c *Construct) expr()      {}
func (t *Tuple) expr()          {}
func (c *Closure) expr()        {}
func (l *ListLit) expr()        {}
func (p *Prop) expr()           {}
func (l *PatLiteral) pattern()  {}
func (w *PatWildcard) pattern() {}
func (b *PatBinding) pattern()  {}
func (o *PatOr) pattern()       {}
func (t *PatTuple) pattern()    {}
func (v *PatVariant) pattern()  {}
func (n *NamedType) typeref()   {}
func (t *TupleType) typeref()   {}
func (u *UnitType) typeref()    {}
func (f *FnType) typeref()      {}
