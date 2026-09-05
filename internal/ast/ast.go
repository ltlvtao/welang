// Package ast holds the syntax tree of chapters 2 and 6: one file is one
// module of top-level items (chapter 6), each fn body a block of statements
// (chapter 2), each expression built from the skeleton's primary, postfix,
// unary, and binary layers with the closed 12-level precedence table. Nodes
// are pure data carrying 1-based source positions (line, column) for the
// stages that follow; grouping parentheses fold away — grouping is binding.
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

// FnDecl is a fn declaration: `[pub] fn name(params) [-> type] block`. Ret
// nil means no declared return type — the function produces no value.
type FnDecl struct {
	Pub       bool
	Name      string
	Params    []Param
	Ret       TypeRef
	Body      Block
	Line, Col int // at fn (or pub)
	NameLine  int
	NameCol   int
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

// SumDecl is a sum type declaration (chapter 9): `[pub] [byval] type Name =
// V1 | ... | Vn`, n at least one, each variant bare (a unit variant) or
// carrying 1..8 payload type references. The generic parameter clause and
// derives clause are chapter 10's and stop at their parse boundary, so this
// node holds no fields for them.
type SumDecl struct {
	Pub       bool
	Byval     bool
	Name      string
	Variants  []Variant
	Line, Col int // at type (or byval/pub)
	NameLine  int
	NameCol   int
}

// Variant is one variant of a sum declaration: the bare name alone, or the
// name with its payload type references.
type Variant struct {
	Name      string
	Payload   []TypeRef
	Line, Col int // at the variant name
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

// Binding is `let|var name [: type] = expr`; Name "_" discards the value.
type Binding struct {
	Kw        string // "let" | "var"
	Name      string
	Typ       TypeRef
	Init      Expr
	Line, Col int // at the keyword
	NameLine  int
	NameCol   int
}

// Assign is `name = expr` — the statement, never an expression.
type Assign struct {
	Name      string
	Value     Expr
	Line, Col int // at the name
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

// Call is `expr(args)`; position at the opening parenthesis.
type Call struct {
	Fn        Expr
	Args      []Expr
	Line, Col int
}

// Member is `receiver.name`; position at the dot.
type Member struct {
	Recv      Expr
	Name      string
	Line, Col int
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

// FnType is `fn(T1, ..., Tn) [tags] -> T` (chapter 7's function type
// reference; EffectTags holds the bare segment when present).
type FnType struct {
	Params     []TypeRef
	EffectTags []string
	Ret        TypeRef
	Line, Col  int
}

func (Import *Import) item()  {}
func (f *FnDecl) item()       {}
func (t *TopLet) item()       {}
func (s *SumDecl) item()      {}
func (b *Binding) stmt()      {}
func (a *Assign) stmt()       {}
func (r *Return) stmt()       {}
func (e *ExprStmt) stmt()     {}
func (i *Ident) expr()        {}
func (l *Literal) expr()      {}
func (u *Unary) expr()        {}
func (b *Binary) expr()       {}
func (c *Call) expr()         {}
func (m *Member) expr()       {}
func (b *BlockExpr) expr()    {}
func (u *Unit) expr()         {}
func (n *NamedType) typeref() {}
func (t *TupleType) typeref() {}
func (u *UnitType) typeref()  {}
func (f *FnType) typeref()    {}
