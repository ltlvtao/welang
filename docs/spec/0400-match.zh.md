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

已批准的模式集是封闭的。字面量模式：第 1 章字面量——数值、字符串、rune、布尔——命中与字面量相等的匹配对象。绑定模式：非 `_` 的标识符，命中任意匹配对象并把值绑定到该名字于该臂内。通配符 `_`：命中任意匹配对象且不绑定任何东西；`_` 永远不是绑定名。或模式 `p1 | p2`：任一分支模式命中即命中。守卫模式 `pattern if cond`：模式命中且 `cond` 求值为真时命中。`|` 与 `if` 是第 1 章 token，且模式位与表达式位是不相交的文法位，故与按位或逻辑运算符无歧义。变体模式 `Name(args)` 与元组模式 `(a, b)` 未批准：它们随 sum types 与元组被类型章节批准时经成对修订进入；在此之前按第 2 章意外 token 诊断拒绝。`_exh_ignore` 不存在：通配符仅 `_`。

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

#### Scenario: 变体模式未批准

- **WHEN** `Point(0, 0)` 作为臂模式出现
- **THEN** 编译器按第 2 章意外 token 诊断拒绝，直至成对修订批准变体模式

### Requirement: 或模式绑定一致性

同一或模式的所有分支绑定的名字集合完全一致；不一致以 `E0302` 拒绝。该检查是句法的——只看名字；所绑名字的类型一致性由类型章节批准。

#### Scenario: 一致的或模式

- **WHEN** 或模式各分支绑定相同名字集（含都不绑定），例如 `200 | 404`
- **THEN** 该或模式被接受

#### Scenario: 不一致的或模式

- **WHEN** 或模式各分支绑定不同名字，例如 `a | b`
- **THEN** 编译器以 `E0302:` or-pattern branches must bind the same names 拒绝

### Requirement: 守卫

守卫模式是 `pattern if cond`：`cond` 是仅在模式命中后求值的表达式，且可引用该模式的绑定。带守卫的臂不计入穷尽性；穷尽性规则——含"包含守卫臂的 match 必须保留无守卫的穷尽兜底臂"——由类型章节批准。本章只定形、惰性与作用域。条件的类型化——须为布尔类型——亦由类型章节批准。

#### Scenario: 守卫可见模式绑定

- **WHEN** 某臂的模式是 `n if n > limit`
- **THEN** 守卫条件可引用 `n`

#### Scenario: 守卫是惰性的

- **WHEN** 某臂的模式未命中匹配对象且该臂带守卫
- **THEN** 该守卫条件不求值

### Requirement: Match 诊断段位

match 章节拥有注册表 `docs/spec/diagnostics.toml` `[segments]` 声明的段位 `E0300`–`E0399`。首批分配：`E0301` 零臂 match、`E0302` 或模式分支须绑定相同名字。`E0300` 与 `E0303`–`E0399` 预留：类型层 match 诊断——穷尽性、守卫覆盖、臂一致——经后续修订在本段内分配进入。触发语义在本章 Requirements；条目在注册表。

#### Scenario: 某 match 码被发出

- **WHEN** 工具链发出任何 `E03xx` 诊断
- **THEN** 其完整条目可从 `docs/spec/diagnostics.toml` 取得，owner 为 `0400-match`

#### Scenario: 类型层修订需要 match 诊断

- **WHEN** 类型章节批准需要新诊断的 match 类型化规则
- **THEN** 其变更在同一变更内扩展注册表，在 `E0300`–`E0399` 内分配号码

## 示例（非权威）

下面的示例只用第 1–4 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。变体与元组模式标注为待其成对修订。

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
    200 => handleOk()          // 字面量模式
    404 | 410 => handleGone()  // 或模式，零绑定：一致
    other => handleOther(other) // 绑定模式：任意值，臂内绑定
    _ => handleAny()           // 通配符：任意值，不绑定
}
// 臂自上而下尝试；首中即胜，故上面的通配符臂在取值意义上
// 不可达——可达性规则随类型章节到来（本章程式批准为非目标）

match n {
    n2 if n2 > limit => "big"  // 守卫模式；守卫可见绑定，
    _ => "small"               // 且仅在模式命中后求值
}

match p {
    Point(0, 0) => "origin"    // E0105: 变体模式随 sum types
    _ => "elsewhere"           // 成对修订进入
}
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
