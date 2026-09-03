# We 语言规范 —— 第 2 章：语法

### Requirement: 解析模型与歧义拒绝

解析的单位是源文件的 token 流（第 1 章批准），每个 token 附带其源行标注。每个 token 序列 MUST 解析为恰一棵树，或被诊断拒绝。存在两种可辩解析的序列 MUST 以 `E0101` 拒绝并具名竞争解释（第 0 章原则 4）。解析 MUST NOT 依赖 token 流及其行标注之外的信息。

#### Scenario: 某构造有两种解析

- **WHEN** 某已批准产生式对一个 token 序列容许两棵不同的树
- **THEN** 编译器以 `E0101:` parse ambiguity 拒绝该序列，消息具名竞争解释

#### Scenario: 片段仅凭局部结构即可解析

- **WHEN** 解析器决定某片段的结构
- **THEN** 其决定只使用该片段的 token、其行标注与已批准产生式——绝不使用调用点或项目全局信息

### Requirement: 行接续与分号推断

语句边界是推断的，不是书写的；源码不携带分号。圆括号 `( ... )` 与方括号（含属性单元 `#[ ... ]`）内部，换行不承载意义。花括号界定块，不是本条意义上的括号，块内适用深度 0 规则。在括号深度 0，每个源行最后一个 token 之后存在语句边界，UNLESS 该 token 属于续行集：某个二元运算符（`+ - * / % == != < <= > >= && || & | ^ << >> ..`）、`=` 或 `.`。续行集是闭集；扩展它是 spec 变更。能起始语句的 token 是固定类：标识符、字面量、关键字、`(`、`{` 与前缀运算符 `!`、`-`、`~`。

#### Scenario: 行尾运算符使语句续行

- **WHEN** 括号深度 0 处某行以二元运算符结尾，例如 `a +` 之后下一行是 `b`
- **THEN** 不插入语句边界；解析延续到下一行

#### Scenario: 行尾操作数使语句结束

- **WHEN** 括号深度 0 处某行以标识符、字面量或闭括号结尾，例如 `let a = b` 之后下一行是 `- c`
- **THEN** 插入语句边界；`- c` 作为一元负号表达式开始一个新语句；意图 `a - c` MUST 写在一行或让运算符尾随

#### Scenario: 括号内换行无意义

- **WHEN** 括号构造跨行，例如调用写作一行的 `f(a,` 与下一行的 `b)`
- **THEN** 换行不起作用；只在括号深度 0 推断边界

#### Scenario: 语句以仅可续行的 token 起始

- **WHEN** 括号深度 0 处某语句以不能起始语句的 token 开始，例如完整上一行之后的 `.method()`
- **THEN** 编译器以 `E0102:` statement begins with a continuation token 拒绝，同时指出该 token 与上一行行尾

### Requirement: 块与块值

块是 `{`，接着由推断的语句边界分隔的项序列，然后 `}`。块是表达式。其值是末项（当末项是表达式时）；末项是语句或块为空时，块无值。要显式丢弃末项表达式的值，作者以 `_` 绑定作为末项（`let _ = expr`）。块值的类型化由类型章节批准。

#### Scenario: 末项表达式即块值

- **WHEN** 块的末项是表达式，例如块以 `a * 2` 结尾且其后无项
- **THEN** 块的值是该表达式

#### Scenario: 末项语句使块无值

- **WHEN** 块的末项是绑定或赋值，或块为空
- **THEN** 块无值

#### Scenario: 丢弃值是显式的

- **WHEN** 末项表达式的值不是意图中的块值
- **THEN** 作者以 `let _ = expr` 作为末项；块随之无值

### Requirement: 语句

语句形式按语句族逐族批准，每族由其归属章节批准；各族及其成员的集合按章节封闭，只能经归属章节的 spec 层变更增长。本章批准：绑定语句（`let name = expr` 与 `var name = expr`，各自可在 `=` 前带类型注解 `name: type`；类型的文法由类型章节批准）、赋值语句（`name = expr`，更丰富的赋值目标随其归属章节批准）、表达式语句（任意表达式作为一项）。控制流章节批准控制流语句（if、while、loop、break、continue、return、defer）；迭代章节批准 for 语句。赋值是语句不是表达式：它不产出值，MUST NOT 出现在要求表达式的位置。

#### Scenario: 赋值嵌于要求表达式的位置

- **WHEN** 赋值出现在表达式位置内部，例如 `let x = y = 1` 或调用实参 `f(a = 1)`
- **THEN** 编译器以 `E0103:` assignment is not an expression 拒绝

#### Scenario: 绑定携带可选注解

- **WHEN** 绑定写作 `let a = expr` 或 `let a: type = expr`
- **THEN** 两种形式都被接受；注解按类型章节把名字绑定到该类型

#### Scenario: 表达式语句

- **WHEN** 非绑定非赋值的表达式作为块项出现
- **THEN** 它是表达式语句；当它同时是末项时，依"块与块值"成为块值

#### Scenario: 语句族的增长

- **WHEN** 后续章节批准新的语句形式
- **THEN** 它们经该章节自身范围内的 spec 层变更进入；本章已批准的语句族不因此改变

### Requirement: 表达式骨架

初等表达式是：标识符、第 1 章批准的字面量、括号表达式 `( e )`。后缀形式是成员访问 `receiver.name` 与调用 `expr(args)`，链式左结合；被访问名字是字段还是方法由类型章节解析。一元前缀运算符是 `!`、`-`、`~`，结合紧于一切二元运算符。关键字引领的表达式形式按归属章节批准：控制流章节批准带 else 的 `if`，match 章节批准 `match`；它们处于 primary/postfix/unary 骨架之外、不修改骨架，且关键字引领的表达式形式只能经其归属章节的 spec 层变更进入。索引语法 `expr[expr]` 延后至集合章节；`[` 与 `]` 仍是词法 token。

#### Scenario: 后缀链从左到右分组

- **WHEN** 解析 `a.f(x).g(y)`
- **THEN** 后缀从初等向外依次应用——访问 `f`、调用、访问 `g`、调用——即分组 `(((a.f)(x)).g)(y)`；访问与调用从左到右成链

#### Scenario: 括号精确分组

- **WHEN** `( e )` 出现在任意表达式位置
- **THEN** 括号固定 `e` 的分组；除本章要求显式分组处外，括号形式与 `e` 可互换

#### Scenario: 关键字引领表达式形式的使用

- **WHEN** 带 else 的 `if` 或 `match` 出现在表达式位置
- **THEN** 该形式遵循其归属章节的 Requirements；本骨架的 primary、postfix、unary 层不变

### Requirement: 运算符优先级与结合性

二元运算符优先级是下述闭表，最紧在前；结合性按档列出，比较与等同档及区间档为非结合：

| 档 | 运算符 | 结合性 |
| --- | --- | --- |
| 1 | 后缀调用与成员访问 | 左 |
| 2 | 一元前缀 `!` `-` `~` | 前缀 |
| 3 | `*` `/` `%` | 左 |
| 4 | `+` `-` | 左 |
| 5 | `<<` `>>` | 左 |
| 6 | `&` | 左 |
| 7 | `^` | 左 |
| 8 | `\|` | 左 |
| 9 | `<` `<=` `>` `>=` `==` `!=` | 无 |
| 10 | `&&` | 左 |
| 11 | `\|\|` | 左 |
| 12 | `..` | 无 |

表是闭集：加运算符、改档或改结合性都是 spec 变更。`=`、`.`、`->`、`=>` 不是二元运算符，在本章范围内绝不出现在表达式内部。

#### Scenario: 混合算术按表分组

- **WHEN** 解析 `a + b * c`
- **THEN** 依档 3 与档 4 分组为 `a + (b * c)`

#### Scenario: 位运算符结合紧于比较

- **WHEN** 解析 `a & mask == flag`
- **THEN** 依档 6 与档 9 分组为 `(a & mask) == flag`

#### Scenario: 链式比较被拒绝

- **WHEN** 不带括号解析 `a < b < c` 或 `x == y == z`
- **THEN** 编译器以 `E0104:` chained non-associative operator 拒绝，修复建议给出拆分形式，例如 `(a < b) && (b < c)`

#### Scenario: 括号化比较不成链

- **WHEN** 解析 `(a < b) && (b < c)`
- **THEN** 每个比较是单一显式组，不触发 `E0104:`

#### Scenario: 链式区间被拒绝

- **WHEN** 解析 `a..b..c`
- **THEN** 编译器以 `E0104:` chained non-associative operator 拒绝；每个区间必须是单一显式组

### Requirement: 语法诊断段位

语法章节拥有注册表 `docs/spec/diagnostics.toml` `[segments]` 声明的段位 `E0100`–`E0199`。首批分配：`E0101` 解析歧义、`E0102` 语句以续行 token 起始、`E0103` 赋值不是表达式、`E0104` 链式非结合运算符、`E0105` 意外 token。`E0100` 与 `E0106`–`E0199` 预留。触发语义在本章 Requirements；条目在注册表。进一步分配语法码在同一变更内扩展注册表。

#### Scenario: 某 token 不适配任何已批准产生式

- **WHEN** 当前解析位置的 token 不适配任何已批准产生式——例如索引形式 `list[0]` 而索引语法尚未批准，或孤立的闭括号
- **THEN** 编译器以 `E0105:` unexpected token 拒绝，具名该 token 与该位置考虑过的产生式

#### Scenario: 某语法码被发出

- **WHEN** 工具链发出任何 `E01xx` 诊断
- **THEN** 其完整条目——严重级、标题、说明、修复建议、owner `0200-grammar`——可从 `docs/spec/diagnostics.toml` 取得

#### Scenario: 后续变更需要语法诊断

- **WHEN** 后续章节批准需要新诊断的语法产生式，例如控制流
- **THEN** 其变更在同一变更内扩展注册表，在 `E0100`–`E0199` 内分配号码；段外号码校验失败

## 示例（非权威）

下面的示例只用骨架已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。

### 行接续

```we
// 分号从不书写；语句边界是推断的。
let a = compute()
let b = a * 2

// 行尾运算符使语句续行。
let total = a +
    b

// 行尾操作数使语句结束：下一行是"新语句"（一元负号），不是续行。
// 要做减法，把 `a - b` 写在一行，或让运算符尾随。
let x = a
    - b

// 圆括号与方括号内部换行无意义。
let config = build(
    host,
    port,
)

// 链式续行用尾随点；前导点被拒绝。
let y = svc.
    query()
let z = svc
    .query()   // E0102: statement begins with a continuation token
```

### 块与块值

```we
// 块是表达式；末项表达式即其值。
let x = {
    let a = compute()
    let b = a * 2
    b + 1
}

// 末项是语句则无值；用 `let _ =` 显式丢弃。
let u = {
    save(record)
    let _ = load()
}

// 末项表达式"恒"是值，即使是普通调用——
// 不想要该值时须刻意丢弃。
let v = {
    let _ = save(record)
    load()
}
```

### 语句

```we
let a = 1
var count: Int64 = 0   // 类型名随类型章节到来；
                       // 注解形式在本章批准
count = count + a      // 赋值是语句

let x = y = 1          // E0103: assignment is not an expression
f(a = 1)               // E0103: 实参位置是表达式
```

### 表达式与优先级

```we
let y = a.f(x).g(z)             // 后缀成链：(((a.f)(x)).g)(z)
let n = -x
let t = !flag
let m = ~bits

let q = a + b * c               // a + (b * c)
let r = a & mask == flag        // (a & mask) == flag：位运算更紧
let ok = a < b < c              // E0104: chained non-associative operator
let good = (a < b) && (b < c)   // 显式分组即拆分形式

let first = list[0]             // E0105: 索引语法尚未批准；
                                // 集合章节将加入
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| statement boundary | 语句边界 |
| line-joining | 行接续 |
| continuation set | 续行集 |
| block value | 块值 |
| binding statement | 绑定语句 |
| assignment statement | 赋值语句 |
| expression statement | 表达式语句 |
| primary expression | 初等表达式 |
| member access | 成员访问 |
| non-associative | 非结合 |
