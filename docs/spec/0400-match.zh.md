# We 语言规范 —— 第 4 章：Match

### Requirement: Match 表达式形式

match 表达式是 `match scrutinee { arm... }`：`match` 关键字、一个表达式（匹配对象 scrutinee）、一个装有一个或多个臂的花括号组。该花括号组不是第 2 章块：它装臂而非语句，且无块值。匹配对象在任一臂被考虑之前恰求值一次。臂自上而下考虑；首个模式命中的臂被取用，其后的臂不求值。match 表达式的值为所取臂的臂体值。零臂的 match 以 `E0301` 拒绝。match 是表达式形式：作为块项出现时即第 2 章表达式语句，不存在单独的"无值 match"形式。

#### Scenario: match 用作值

- **WHEN** 臂为表达式体的 match 初始化绑定，例如 `let x = match c {` 且臂 `200 => 1` 与 `_ => 0` 各自立行，`c` 等于 `200`
- **THEN** `x` 取所取臂的臂体值，此处为 `1`

#### Scenario: 首个命中的臂胜出

- **WHEN** 两臂都能命中匹配对象，例如字面量臂之后跟通配符臂
- **THEN** 取较早的臂；较晚的臂不求值

#### Scenario: 匹配对象恰求值一次

- **WHEN** 匹配对象是调用，例如 `match next() {` 且臂 `n => n`
- **THEN** 该调用在臂选择之前恰求值一次

#### Scenario: 零臂的 match

- **WHEN** 出现 `match c { }`——花括号组内无臂的 match
- **THEN** 编译器以 `E0301:` match requires at least one arm 拒绝

### Requirement: Match 臂

臂是 `pattern => body`，其中 `body` 是表达式；块在第 2 章下是表达式，故 `pattern => expr` 与 `pattern => { ... }` 是一条规则而非两种形式。臂按第 2 章行接续以推断边界分隔：管语句的续行集与边界推断不加修改地管臂，每臂自立一行；臂间无分隔符 token，臂间 `,` 不合任何已批准产生式（第 2 章意外 token 诊断）。臂的模式绑定——及其守卫条件（若有）——在该臂内可见，MUST NOT 泄漏过臂。

#### Scenario: 块体臂

- **WHEN** 某臂的体是块，例如 `_ => {` 含项 `log()` 与 `0` 及闭 `}`
- **THEN** 该臂的臂体值是第 2 章"块与块值"下的块值

#### Scenario: 臂绑定不泄漏

- **WHEN** 某臂经其模式绑定名字，例如臂 `n => n`
- **THEN** 该绑定在 match 表达式之后不可见；每臂是自己的作用域

#### Scenario: 臂间逗号

- **WHEN** 出现 `200 => 1, _ => 0`——单行内以逗号分隔的臂
- **THEN** 编译器按第 2 章意外 token 诊断拒绝；臂间未批准任何分隔符 token

### Requirement: 模式集

已批准的模式集是封闭的。字面量模式：第 1 章字面量——数值、字符串、rune、布尔——命中与字面量相等的匹配对象。绑定模式：非 `_` 的标识符，命中任意匹配对象并把值绑定到该名字于该臂内。通配符 `_`：命中任意匹配对象且不绑定任何东西；`_` 永远不是绑定名。或模式 `p1 | p2`：任一分支模式命中即命中。守卫模式 `pattern if cond`：模式命中且 `cond` 求值为真时命中。元组模式 `(p1, p2, ..., pn)`：依第 8 章命中同元数的元组值，每个子模式是任何已批准模式，递归嵌套。变体模式，依第 9 章：载荷变体为 `Name(p1, ..., pk)`、单元变体为 `Name` 单独出现，子模式自载荷逐位绑定、递归嵌套；模式的 `Name` MUST 是匹配对象 sum 类型已声明的变体（否则 `E0303`），且载荷数恰为声明数（否则 `E0304`）。大小写切分是词法的：变体名 PascalCase（`E0011`）、绑定名 camelCase（`E0012`），故裸 PascalCase 模式恒为变体引用、恒非绑定。变体模式可反驳：只出现在 match 臂——绑定语句的名字位只收不可反驳模式，故 `let Circle(r) = s` 按第 2 章意外 token 诊断（`E0105`）拒绝。`|` 与 `if` 是第 1 章 token，且模式位与表达式位是不相交的文法位，故与按位或逻辑运算符无歧义。`_exh_ignore` 不存在：通配符仅 `_`。

#### Scenario: 字面量模式

- **WHEN** 某臂的模式是第 1 章字面量，例如 `200`
- **THEN** 该臂命中与该字面量相等的匹配对象

#### Scenario: 通配符模式

- **WHEN** 某臂的模式是 `_`
- **THEN** 它命中任意匹配对象且不绑定

#### Scenario: 绑定模式

- **WHEN** 某臂的模式是非 `_` 标识符，例如 `n`
- **THEN** 它命中任意匹配对象，把值绑定到 `n` 于该臂内

#### Scenario: 或模式

- **WHEN** 某臂的模式是 `200 | 404`
- **THEN** 任一分支的模式命中时该臂命中

#### Scenario: 元组模式

- **WHEN** 某臂的模式是 `(a, b)` 且匹配对象是二元元组
- **THEN** 该臂命中，依第 8 章把 `a` 与 `b` 绑定到各元素

#### Scenario: 变体模式命中并绑定

- **WHEN** 某臂的模式是 `Circle(r)` 且匹配对象是携带 `Circle(2.0)` 的 `Shape`
- **THEN** 该臂命中，依第 9 章自载荷把 `r` 绑定为 `2.0`

#### Scenario: 嵌套模式

- **WHEN** 某臂的模式是 `(Circle(r), _)` 且匹配对象是首元素携带 `Circle(1.0)` 的 `(Shape, Shape)`
- **THEN** 变体模式在元组模式内嵌套命中，把 `r` 绑定为 `1.0`

#### Scenario: 未知变体被拒绝

- **WHEN** 某臂的模式是 `Triangle(r)` 而匹配对象的 sum 类型未声明 `Triangle`
- **THEN** 编译器以 `E0303:` pattern names no variant of the scrutinee's type 拒绝

#### Scenario: 载荷数不匹配被拒绝

- **WHEN** 某臂的模式是 `Circle(a, b)` 而声明为 `Circle(Float64)`
- **THEN** 编译器以 `E0304:` variant pattern payload arity mismatch 拒绝

#### Scenario: let 中的变体模式被拒绝

- **WHEN** `let Circle(r) = s` 作为绑定语句出现
- **THEN** 编译器以 `E0105:` unexpected token 拒绝；可反驳模式仅限 match

### Requirement: 或模式绑定一致性

同一或模式的所有分支绑定的名字集合完全一致；不一致以 `E0302` 拒绝。名字集检查是句法的；类型并非仅仅声称：分支绑定相同名字时，其绑定类型 MUST 一致，不一致按第 7 章 `E0501` 拒绝——一个名字在一种模式内绑定两个类型，即是共用一名的两个绑定。本条落定或模式绑定被悬置的类型一致。

#### Scenario: 一致的或模式

- **WHEN** 或模式各分支绑定相同名字集（含都不绑定），例如 `200 | 404`
- **THEN** 该或模式被接受

#### Scenario: 不一致的或模式

- **WHEN** 或模式各分支绑定不同名字，例如 `a | b`
- **THEN** 编译器以 `E0302:` or-pattern branches must bind the same names 拒绝

#### Scenario: 同名异型被拒绝

- **WHEN** 或模式各分支以不同类型绑定同一名字，例如对 `(Int64, String)` 的 `(a, _) | (_, a)`
- **THEN** 编译器以 `E0501` 拒绝：`a` 在一支绑 `Int64`、在另一支绑 `String`

### Requirement: 守卫

守卫模式是 `pattern if cond`：`cond` 是仅在模式命中后求值的表达式，且可引用该模式的绑定。带守卫的臂不计入穷尽性：穷尽性规则——含"包含守卫臂的 match 必须保留无守卫的穷尽兜底臂"——是本章"穷尽性"条款的，类型侧事实归第 9 章。本章只定形、惰性与作用域。条件的类型化——须为布尔类型——由第 7 章批准（`E0503`）。

#### Scenario: 守卫可见模式绑定

- **WHEN** 某臂的模式是 `n if n > limit`
- **THEN** 守卫条件可引用 `n`

#### Scenario: 守卫是惰性的

- **WHEN** 某臂的模式未命中匹配对象且该臂带守卫
- **THEN** 该守卫条件不求值

### Requirement: 穷尽性

match MUST 穷尽：其无守卫臂的模式合起来 MUST 命中匹配对象类型的每一个值。sum 类型的值即其变体，故列全变体——以变体名模式或其上的或模式——即穷尽。基础类型不可穷尽：字面量模式永不能覆盖 `Int64`、`String` 或 `Bool`，故对基础类型的 match MUST 保留通配符臂，对任何值集不可枚举的类型亦然。元组匹配对象依第 8 章：元组类型恰以其元素类型的可穷尽性为可穷尽性。非穷尽的 match 以 `E0305` 拒绝。守卫臂不计入穷尽证明——含守卫臂的 match MUST 保留无守卫的穷尽兜底，否则 `E0306`——故加守卫纯是臂的细化，绝非覆盖的削弱。永不可能命中的臂——其每个值已被更早的臂覆盖，如通配符之后的臂、被列出两次的变体——以 `E0307` 拒绝。哪些类型枚举其值是类型章节的事实；本条款定 match 层检查。

#### Scenario: 列全变体即穷尽

- **WHEN** 对恰有这些变体的 `s: Shape` 的 match 持有臂 `Circle(r) => 1` 与 `Rectangle(w, h) => 2`
- **THEN** 该 match 无需通配符即穷尽；检查是静态的

#### Scenario: 缺失变体被拒绝

- **WHEN** 对 `Shape` 的 match 覆盖 `Circle` 而未覆盖 `Rectangle`，且无通配符
- **THEN** 编译器以 `E0305:` match is not exhaustive 拒绝，指名未覆盖的变体

#### Scenario: 基础类型 match 需要通配符

- **WHEN** 对 `Int64` 的 match 持有字面量臂 `200` 与 `404` 而无通配符
- **THEN** 编译器以 `E0305` 拒绝；字面量模式永不穷尽基础类型，`Bool` 亦然

#### Scenario: 守卫臂不计数

- **WHEN** 对 `Shape` 的 match 持有 `Circle(r) if r > 1.0 => ...` 与 `Rectangle(w, h) => ...` 而无 `Circle` 的无守卫兜底
- **THEN** 编译器以 `E0306:` guarded match without an unguarded exhaustive fallback 拒绝

#### Scenario: 通配符之后的臂不可达

- **WHEN** 通配符臂之后还跟着另一臂，例如 `_ => 0` 之后是 `Circle(r) => 1`
- **THEN** 编译器以 `E0307:` unreachable match arm 拒绝较晚的臂

#### Scenario: 变体列出两次不可达

- **WHEN** 对 `Shape` 的 match 持有两个 `Circle` 臂且其间无通配符
- **THEN** 编译器以 `E0307:` unreachable match arm 拒绝第二个

#### Scenario: sum 元组的逐元素穷尽

- **WHEN** 对 `(Shape, Shape)` 的 match 无通配符地枚举每种变体组合
- **THEN** 该 match 依第 8 章逐元素规则穷尽

### Requirement: 臂类型一致

值位使用的 match 各臂类型 MUST 一致：每臂的体类型为同一类型，`Never` 臂依第 9 章排除，全 `Never` 的诸臂一致为 `Never`。不一致在 match 表达式处按第 7 章 `E0501` 拒绝。语句位使用的 match，其一致类型非 unit 类型者落第 8 章弃值规则——即彼处指名的"语句位未绑定 match 臂体"。本条落定本章诊断段位预留的臂一致议题。

#### Scenario: 臂类型不一致

- **WHEN** 表达式位的 match 持有臂 `Circle(r) => 1` 与 `Rectangle(w, h) => "wide"`
- **THEN** 编译器以 `E0501` 拒绝：两臂为 `Int64` 与 `String`

#### Scenario: unit 臂作语句成立

- **WHEN** 语句位 match 的每臂均为 unit 类型
- **THEN** 该 match 自由成立；无需弃值形式

#### Scenario: 非 unit 语句位 match 被拒绝

- **WHEN** 语句位 match 的各臂一致为 `Int64`
- **THEN** 编译器以 `E0605:` non-unit value dropped 拒绝；修复为 `let _ = match ...` 或消费该值

### Requirement: Match 诊断段位

match 章节拥有注册表 `docs/spec/diagnostics.toml` `[segments]` 声明的段位 `E0300`–`E0399`。分配：`E0301` 零臂 match、`E0302` 或模式分支须绑定相同名字、`E0303` 变体模式未指名匹配对象类型的任何变体、`E0304` 变体模式载荷数不匹配、`E0305` match 非穷尽、`E0306` 带守卫 match 缺无守卫穷尽兜底、`E0307` 不可达 match 臂——本段位为之预留的类型层 match 诊断，经 sum 类型章的成对修订进入。`E0300` 与 `E0308`–`E0399` 继续为后续修订预留。触发语义在本章 Requirements；条目在注册表。

#### Scenario: 某 match 码被发出

- **WHEN** 工具链发出任何 `E03xx` 诊断
- **THEN** 其完整条目可从 `docs/spec/diagnostics.toml` 取得，owner 为 `0400-match`

#### Scenario: 类型层修订需要 match 诊断

- **WHEN** 类型章节批准需要新诊断的 match 类型化规则
- **THEN** 其变更在同一变更内扩展注册表，在 `E0300`–`E0399` 内分配号码

## 示例（非权威）

下面的示例只用第 1–4 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。元组模式归第 8 章、变体模式归第 9 章，均在各自章节示例。

### match 作为值

```we
let x = match code {
    200 => "ok"
    _ => "other"
}
// match 是表达式：x 取所取臂的臂体值

let y = match code {
    200 => 1
    _ => {
        log("unexpected status")
        0
    }
}
// 块体与表达式体同属一条规则：块即表达式
```

### 模式

```we
match status {
    200 => handleOk()          // literal pattern
    404 | 410 => handleGone()  // or-pattern, no bindings: consistent
    other => handleOther(other) // binding pattern: any value, binds for
}                               // the arm — and exhausts: nothing may
                                // follow it (E0307)

match n {
    n2 if n2 > limit => "big"  // guarded pattern; the guard sees the
    _ => "small"               // binding, and is evaluated only if the
}                              // pattern matched; the unguarded wildcard
                               // is the exhaustive fallback (E0306)
```

### 穷尽性与臂类型一致

```we
match level {                 // level is Int64: a base type
    0 => off()
    _ => on()                 // required: literals never exhaust a
}                             // base type (E0305)

match level {
    0 => off()                // E0305: match is not exhaustive —
}                             // literals never exhaust a base type

match level {
    n if n > 3 => loud()
    n => quiet(n)             // the unguarded fallback exhausts; a
}                             // guarded arm alone would be E0306

match level {
    _ => off()
    0 => on()                 // E0307: unreachable match arm
}

let label = match code {      // value position: the arms agree (all
    200 => "fine"             // String), so the match has a type
    _ => "other"
}

match code {                  // statement position, arms agreeing on
    200 => "fine"             // String: non-unit value dropped (E0605,
    _ => "other"              // chapter 8) — `let _ = match ...` or a
}                             // unit-bodied arm set
```

### 或模式绑定一致性

```we
match code {
    200 | 404 => "known"       // 各分支绑定相同名字：零绑定，OK
    a | b => "either"          // E0302: or-pattern branches must
    _ => "?"                   // bind the same names
}
```

### 被拒绝的形式

```we
let x = match code { }         // E0301: match requires at least one arm

let y = match code { 200 => 1, _ => 0 }
//                               ^ 按第 2 章意外 token 诊断拒绝：
// 臂以换行分隔，无分隔符 token
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| match expression | match 表达式 |
| scrutinee | 匹配对象 |
| arm | 分支臂 |
| pattern | 模式 |
| literal pattern | 字面量模式 |
| binding pattern | 绑定模式 |
| wildcard | 通配符 |
| or-pattern | 或模式 |
| guard | 守卫 |
| exhaustiveness | 穷尽性 |
| exhaustive fallback | 穷尽兜底 |
| variant pattern | 变体模式 |
| unreachable arm | 不可达臂 |
| arm agreement | 臂类型一致 |
