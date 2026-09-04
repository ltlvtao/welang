## MODIFIED Requirements

### Requirement: The scope resource statement

The scope resource statement is `scope resource(name = expr, ..., name = expr) block`, with at least one binding. The two words `scope` and `resource` enter chapter 1's keyword list by this chapter's amendment; the statement opens with them, so chapter 2's statement-start class — which admits keywords — admits it with no further amendment, and chapter 2's statement-family enumeration credits this chapter. Each binding is `name = expr` with no annotation slot: the binding's type is the head expression's, and the head expression's type MUST implement Releasable — a head whose type does not is rejected with `E1103:` scope resource head does not implement Releasable; under the completeness obligation above the implementers are exactly the `byres record`s, so the check rejects every non-resource head. Head expressions evaluate left to right; each name binds for the block. At block exit the compiler guarantees exactly one `release` call per binding, in reverse declaration order; exits pierced by `return`, `break`, or `continue` are block exits and release the same way. A `defer` inside the scope block is chapter 3's `E0204` — defer is a direct function-body item only — and the scope-exit releases run before the enclosing function's own defers, the inner block exiting first. The statement produces no value: it is not an expression. A panic unwinding this block is a block exit of this guarantee: the releases run in reverse declaration order, before the enclosing function's defers — chapter 14's unwinding requirement extends the guarantee to unwound exits; every exit kind releases.

#### Scenario: A fresh construction released at exit

- **WHEN** `scope resource(f = openFile("data.txt")) { let n = f.read() }` appears in a function body and the block completes
- **THEN** at block exit `release` runs exactly once for `f`, in the implementation the impl of Releasable for FileHandle provides

#### Scenario: Reverse declaration order at exit

- **WHEN** a scope resource statement binds `a` then `b` and the block exits
- **THEN** `b`'s release runs before `a`'s

#### Scenario: Return pierces the scope

- **WHEN** the scope block contains an early `return` of a value
- **THEN** the releases run at that exit, before the function's value completes; the enclosing function's own defers run after, the inner block exiting first

#### Scenario: Break pierces the scope

- **WHEN** a scope resource statement appears in a loop body and the block contains `break`
- **THEN** the releases run at that exit, before the loop is left

#### Scenario: Panic unwinds the scope

- **WHEN** a panic fires while the scope block is in progress
- **THEN** the releases run at that exit per this requirement's guarantee, before the enclosing function's defers — chapter 14's unwinding requirement; every exit kind releases

#### Scenario: A non-resource head is rejected

- **WHEN** `scope resource(u = makeUser())` appears, `makeUser` returning a gc record
- **THEN** the compiler rejects it with `E1103:` scope resource head does not implement Releasable

#### Scenario: Defer inside the scope block

- **WHEN** `scope resource(f = openFile("a")) { defer { log() } }` appears — a defer as an item of the scope block
- **THEN** the compiler rejects it with chapter 3's `E0204`: defer placement; defer stays a direct function-body item

#### Scenario: At least one binding

- **WHEN** `scope resource() { }` appears — an empty binding list
- **THEN** the statement fits no production and chapter 2's unexpected-token diagnostic (`E0105`) reports the `)` that offers no binding
