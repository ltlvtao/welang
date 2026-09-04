# We 语言规范 —— 第 1 章：词法结构

### Requirement: 源文件

We 源文件是 UTF-8 编码的文本文件，扩展名 `.we`。允许并忽略单个前导 U+FEFF（字节序标记）。换行为 LF；CRLF 被接受并计为一次换行。U+FEFF 的其他任何出现、任何非合法 UTF-8 的字节序列、以及未跟 LF 的裸 CR 均为词法错误。非 ASCII 字符仅允许出现在注释与字面量内部。

#### Scenario: 拒绝非法编码

- **WHEN** 源文件包含非合法 UTF-8 的字节序列，或非 ASCII 字符出现在只允许 token 的位置（注释与字面量之外）
- **THEN** 词法分析器报告 `E0001: invalid character` 并给出出错字节或字符的偏移，且不产出 token 流

#### Scenario: BOM 与换行被机械归一

- **WHEN** 源文件以 U+FEFF 开头或使用 CRLF 换行
- **THEN** BOM 被忽略，每个 CRLF 恰计为一次换行；两者均不产生诊断

### Requirement: Token 模型

词法分析是无上下文的：每个字符序列的 token 种类仅由对本章闭集 token 清单的最大吞噬匹配决定，绝不依赖语法上下文。空白与注释分隔 token，此外无意义。We 的上下文相关词法规则集合为**空集且封闭**：本规范定义零个软关键字、零种依上下文解释的 token，且任何后续章节不得引入。

#### Scenario: 最大吞噬决定 token 边界

- **WHEN** 某字符序列在同一位置存在更长与更短的 token 匹配（例如 `1_000`、`0x1F`、`!=`）
- **THEN** 词法分析器总是产生本章清单允许的最长匹配，且结果与周围 token 无关

#### Scenario: 出现上下文相关词法提案

- **WHEN** 未来某提案建议依语法位置不同地解释某个单词或符号（软关键字或上下文 token）
- **THEN** 它与本条 Requirement 的封闭空集冲突，必须拒绝，除非先通过 spec 层变更修订本章

### Requirement: 标识符

标识符为 `[A-Za-z_][A-Za-z0-9_]*`。标识符区分大小写。该集合之外的一切字符——包括所有非 ASCII 字母、非 ASCII 数字的数字字符、以及易混淆同形字符——禁止用于标识符：We 不存在 Unicode 标识符。非 ASCII 文本可自由用于注释与字面量。

#### Scenario: 拒绝非 ASCII 标识符

- **WHEN** 某名字以非 ASCII 字符书写（例如视觉上近似 ASCII 字母的西里尔或 CJK 字符）
- **THEN** 词法分析器在首个出错字符处报告 `E0001: invalid character`，并以同形字符易混淆性作为标识符仅限 ASCII 的理由

#### Scenario: 大小写敏感是全面的

- **WHEN** 两个标识符仅大小写不同（例如 `userId` 与 `UserID`）
- **THEN** 它们是两个不同的标识符，彼此无任何已定义的关系

### Requirement: 关键字

关键字处处完全保留：关键字 token 在任何语法位置都不是标识符。We 定义**零软关键字**——不存在任何可将保留字用作名字的上下文，也不存在任何使标识符变成保留字的上下文。初始关键字集合为封闭枚举：

```
fn let var pub import as mut
if else return match for in while loop break continue defer
true false foreign
record byval byres newtype with
type
interface impl where derives
scope resource
effect
task select case timeout collectAll
```

新增关键字是修订本清单的 spec 层变更。清单只收录其语法已被或必然被某章批准的词；特性专属词（效应、类型、并发、测试）随批准它的章节一同进入清单。复合类型章节在其自身变更中加入 `record byval byres newtype with`——按破坏性变更如实记录：这五个词此前是标识符，自本清单修订起成为关键字。sum 类型章节在其自身变更中加入 `type`——同样如实记录：该词此前是标识符，自本清单修订起成为关键字。接口与泛型章节以同样方式加入 `interface impl where derives`：这四个词此前是标识符，自本清单修订起成为关键字。资源章节以同样方式加入 `scope resource`：这两个词此前是标识符，自本清单修订起成为关键字——`scope` 将引导更多复合形式，其预留先于那些使用，如 `mut` 之预留。效果章节以同样方式加入 `effect`：该词此前是标识符，自本清单修订起成为关键字——它按第 16 章引入效果声明与声明效果段。并发章节以同样方式加入 `task select case timeout collectAll`：这五个词此前是标识符，自本清单修订起成为关键字——它们引导 task 块、select 表达式与在那儿批准的 scope 块形式，`scope` 已由资源章节先于这些复合使用预留，如其修订所记。`mut` 自初始清单即在，其语法随第 10 章 `mut self` 接收者兑付——其预留先于其使用。

#### Scenario: 关键字被用作名字

- **WHEN** 枚举清单中的关键字出现在需要标识符的位置（例如名为 `match` 的变量）
- **THEN** 解析器报告语法诊断，指出保留字及其位置；词法分析器永不将其作为标识符 token 提供

#### Scenario: 后续章节需要新关键字

- **WHEN** 某内容章节批准的语法引入了不在本清单中的保留字
- **THEN** 其 spec 增量必须在同一变更中修订本清单，且该修订按破坏性变更如实记录

#### Scenario: 复合关键字被保留

- **WHEN** `record`、`byval`、`byres`、`newtype` 或 `with` 出现在要求标识符的位置（例如名为 `record` 的变量）
- **THEN** 解析器报告具名该保留字的语法诊断；这五个词依复合类型章节的修订成为关键字

#### Scenario: type 关键字被保留

- **WHEN** `type` 出现在要求标识符的位置（例如名为 `type` 的变量）
- **THEN** 解析器报告具名该保留字的语法诊断；该词依 sum 类型章节的修订成为关键字

#### Scenario: 接口关键字被保留

- **WHEN** `interface`、`impl`、`where` 或 `derives` 出现在要求标识符的位置（例如名为 `impl` 的变量）
- **THEN** 解析器报告具名该保留字的语法诊断；这四个词依接口与泛型章节的修订成为关键字

#### Scenario: 资源关键字被保留

- **WHEN** `scope` 或 `resource` 出现在要求标识符的位置（例如名为 `scope` 的变量）
- **THEN** 解析器报告具名该保留字的语法诊断；这两个词依资源章节的修订成为关键字

#### Scenario: effect 关键字被保留

- **WHEN** `effect` 出现在要求标识符的位置（例如名为 `effect` 的变量）
- **THEN** 解析器报告具名该保留字的语法诊断；该词依效果章节的修订成为关键字

#### Scenario: 并发关键字被保留

- **WHEN** `task`、`select`、`case`、`timeout` 或 `collectAll` 出现在要求标识符的位置（例如名为 `task` 的变量）
- **THEN** 解析器报告具名该保留字的语法诊断；这五个词依并发章节的修订成为关键字

### Requirement: 数值字面量

整数字面量写为十进制（`123`）、十六进制（`0x` 前缀，至少一位十六进制数字）、八进制（`0o` 前缀，至少一位 `0`–`7`）或二进制（`0b` 前缀，至少一位 `0`/`1`）。十进制字面量禁止前导 `0`（`0` 本身合法）。下划线 `_` 是数字分组分隔符，**仅允许在两个数字之间**——不可前导、不可结尾、不可连续、不可紧邻进制前缀、不可紧邻后缀。整数后缀：`i8` `i16` `i32` `i64` `u8` `u16` `u32` `u64`。浮点字面量含 `.` 且两侧各至少一位数字，可带指数部分（`e`/`E`、可选正负号、至少一位数字），仅限十进制；浮点后缀：`f32` `f64`。后缀紧贴最后一位数字。无后缀字面量所指名的类型由类型章节定义；本章只定义形态。

#### Scenario: 分隔符位置是机械的

- **WHEN** 数值字面量中的下划线不在两个数字之间（例如 `1__0`、`_100`、`100_`、`0x_FF`、`100_i64`）
- **THEN** 词法分析器在该字面量处报告 `E0006: invalid numeric literal`，并陈述分隔符规则

#### Scenario: 进制前缀要求数字

- **WHEN** 字面量以 `0x`、`0o` 或 `0b` 开头却未跟至少一位该进制数字（例如 `0x`、`0b2`）
- **THEN** 词法分析器报告 `E0006: invalid numeric literal`，指明期望的数字集合

#### Scenario: 后缀绑定整个字面量

- **WHEN** 字面量带后缀（例如 `42i8`、`3.14f32`）
- **THEN** 后缀是同一字面量 token 的一部分；未知后缀（例如 `42int`）报告 `E0006: invalid numeric literal`——后缀是闭集

### Requirement: 字符串与 rune 字面量

字符串字面量是单行的 `"..."`；rune 字面量是 `'x'`，恰含一个字符或一个转义。插值：字符串字面量内部，`$` 紧跟 `{` 开启一个含平衡花括号表达式区域的插值孔；`$` 不跟 `{` 时是普通字符；rune 字面量内不存在插值。转义集合为闭集且两者相同：`\n` `\t` `\r` `\0` `\\` `\"` `\'` `\u{h..}`（1–6 位十六进制，值 ≤ U+10FFFF，禁止代理项）。`\` 之后的其他任何字符均为错误。本章阶段的 We 不存在多行字符串与原始（免转义）字符串；新增任一形态是 spec 层变更。

#### Scenario: 字面量未终结

- **WHEN** 字符串或 rune 字面量未在行结束前闭合（rune 情形：未在同一行的 `'` 之前闭合）
- **THEN** 词法分析器在起始引号处报告 `E0002: unterminated string literal` 或 `E0003: unterminated rune literal`

#### Scenario: 未知转义

- **WHEN** 字面量中 `\` 后跟的字符不在闭集转义集合内（例如 `\e`、`\x41`）
- **THEN** 词法分析器报告 `E0005: invalid escape sequence`，指出该字符与允许的集合

#### Scenario: 插值孔在词法上平衡

- **WHEN** 字符串中的 `${` 在字面量内花括号不平衡（例如 `"${f(}"`）
- **THEN** 词法分析器在起始 `${` 处报告 `E0007: unbalanced interpolation braces`；不跟 `{` 的 `$` 永不视为插值

### Requirement: 运算符与标点

运算符与标点清单为封闭枚举。运算符：`+ - * / %` `== != < <= > >=` `&& || !` `& | ^ << >> ~` `=` `..`。标点：`( ) { } [ ] , ; : . -> => _ ?`，以及 token `#[` 与其闭合 `]`（属性单元，一个词法结构）。运算符重载不存在（目标场景），自定义运算符不可定义：本清单之外的字符（例如 `$`、`@`、`` ` ``）永不构成 token。`<<` 与 `>>` 在最大吞噬下是单 token；与嵌套泛型右尖括号的交互（需要分隔）由泛型章节规定。`..` 在最大吞噬下是单 token，数值字面量的最大吞噬绝不吸收开启 `..` token 的点：`0..9` 词法化为 `0`、`..`、`9`。

#### Scenario: 未知运算符序列

- **WHEN** 字符序列不匹配本章清单中的任何 token（例如 `$`、`@`、不跟 `[` 的 `#`）
- **THEN** 词法分析器在首个未匹配字符处报告 `E0001: invalid character`；编译器绝不从上下文猜测运算符

#### Scenario: 多字符 token 在最大吞噬下无歧义

- **WHEN** 输入包含 `<<`、`<=`、`->` 或 `=>`
- **THEN** 各为单 token；拆分或重组（`< <`、`- >`）是词法错误或不同的 token 流，由最大吞噬独自裁决

#### Scenario: 区间 token 与相邻数字分离

- **WHEN** 输入包含 `0..9` 或 `1.5..2.5`
- **THEN** `..` 是单 token；序列词法化为 `0` `..` `9` 与 `1.5` `..` `2.5`——不构成浮点 `0.` 或 `.9`，因为浮点要求点两侧各有一位数字

### Requirement: 注释

三种注释形态：行注释 `//` 至行尾；块注释 `/* */` 不嵌套（内层 `/*` 是普通文本；首个 `*/` 闭合注释）；文档注释 `///` 至行尾，其附着规则由声明章节定义。注释可含任意 Unicode 文本。注释与空白一样分隔 token。

#### Scenario: 块注释未终结

- **WHEN** `/*` 在文件结束前未被 `*/` 闭合
- **THEN** 词法分析器在起始 `/*` 处报告 `E0004: unterminated block comment`

#### Scenario: 不嵌套是机械的

- **WHEN** 块注释内部包含 `/* ... */`
- **THEN** 首个 `*/` 闭合整个注释；内层 `/*` 永不视为开启嵌套层级

### Requirement: 属性词法单元

属性是 token `#[` 后跟名字、可选的带括号参数、以及闭合 `]`——附着到下一个声明的一个词法单元。属性参数只能是编译期字面量值（字符串、数值、布尔、字面量列表）。属性命名空间是封闭白名单：白名单之外的属性名是编译错误，绝不静默忽略。存在哪些属性由拥有它们的章节定义；本章定义形态、仅字面量参数规则、以及闭集白名单义务（第 0 章原则 8）。

#### Scenario: 未知属性

- **WHEN** 属性指名的词不在由已批准章节汇编的白名单中（例如尚无章节批准 `#[inline]` 时使用它）
- **THEN** 编译器报告 `E0008: unknown attribute` 并给出名字与位置；不得跳过

#### Scenario: 非字面量属性参数

- **WHEN** 属性参数是表达式、函数调用或引用（例如 `#[timeout(5000 + n)]`）
- **THEN** 编译器在该参数处报告 `E0009: non-literal attribute argument`；属性合法性保持局部可判定（第 0 章原则 1）

### Requirement: 命名约定

标识符拼写由绑定类别固定并作为错误强制执行，使每个 We 代码库只有一种约定：类型名（`record`/`interface`/`newtype`/和类型/别名形态，由类型章节批准）为 PascalCase；变量、函数、方法为 camelCase；模块名小写并以点分隔；常量随变量规则（无 SCREAMING_CASE）。该检查仅由标识符本身加其绑定类别即可判定。

#### Scenario: 约定违规是错误而非风格提示

- **WHEN** 类型名非 PascalCase（例如 `userRecord`）、变量/函数/方法名非 camelCase（例如 `User_Id`）、或模块名非小写加点
- **THEN** 编译器分别在声明处报告 `E0011: type name must be PascalCase`、`E0012: variable, function, and method names must be camelCase` 或 `E0013: module names must be lowercase`

### Requirement: 诊断码方案

诊断码为 `E`（错误）或 `W`（警告）后恰四位数字；`E` 与 `W` 共享同一号码空间，一个四位数同一时刻至多存在一个严重级。分段与已分配码的唯一注册表是 `docs/spec/diagnostics.toml`：每个已分配码在那里恰有一条目（严重级、标题、说明、修复建议、归属章节与 Requirement），码何时发出由各章节的 Requirements 与 Scenarios 定义。词法段 `E0001`–`E0099` 归本章所有；段内 `E0001`–`E0009` 为 token 级错误，`E0011`–`E0019` 为命名块（`E0011`–`E0013` 已分配），`E0010` 与 `E0014`–`E0099` 预留给本章的未来修订。警告码遵循同一方案；本章未分配任何警告码。

#### Scenario: 后续章节认领分段

- **WHEN** 某内容章节需要新诊断码
- **THEN** 其变更必须在同一变更中扩展 `docs/spec/diagnostics.toml`——在段位表认领区段，并为每个已分配码添加一条目——且对既有码重编号被禁止（第 0 章：诊断协议是稳定性承诺）

#### Scenario: 诊断码跨章唯一

- **WHEN** 两个章节会以不同含义定义同一码
- **THEN** 注册表拒绝第二个定义：码键全局唯一，恰有一个严重级、一个标题、一个归属；validate 机械执法

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| token | 词法单元 |
| token stream | token 流 |
| maximal munch | 最大吞噬 |
| keyword | 关键字 |
| soft keyword | 软关键字 |
| identifier | 标识符 |
| literal | 字面量 |
| rune literal | rune 字面量 |
| escape sequence | 转义序列 |
| interpolation | 插值 |
| attribute | 属性 |
| naming convention | 命名约定 |
| diagnostic code | 诊断码 |
| segment | 分段 |
| closed set | 闭集 |
