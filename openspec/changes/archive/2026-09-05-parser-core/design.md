# parser-core — 设计

规范依据：`docs/spec/0200-grammar.md`（下称 ch2）全部 Requirement；`docs/spec/0600-declarations.md`（下称 ch6）全部 Requirement；`docs/spec/0700-types.md`（下称 ch7）R「Type references」（仅文法）；注册表 E0101–E0105、E0012/E0013、E0401–E0405 条目逐字引用。裁决（2026-09-05）：Q1 逐章 70 边界行；Q2 四组诊断码全落地；Q3 完整 ch7 类型引用文法。以下每条决策标注规范条款与机制自由度的边界。

## D1 AST 模型与包边界

`internal/ast` 持节点，`internal/parser` 持解析器（M3 类型检查器 import ast 而不依赖 parser，避免环）。节点最小但带位置（M3 消费）：

- `File{Items []Item}`；Item = `Import{Pub? no—import 无 pub, Path []string, Alias string, Line, Col}` / `FnDecl{Pub, Name, Params []Param, Ret TypeRef(nil=未声明), Body Block, …}` / `TopLet{Pub, Binding}`。
- `Param{Name string, Type TypeRef}`；`Binding{Kw("let"|"var"), Name string("_" 表丢弃), Typ TypeRef?, Init Expr}`（元组模式头属 ch8，M2 到达即边界，不入 AST）。
- `Block{Items []Stmt}`；Stmt = Binding / `Assign{Name, Value}` / `Return{HasValue, Value}` / `ExprStmt{Expr}`。
- Expr = `Ident` / `Literal{Kind, Text}` / `Unary{Op, X}` / `Binary{Op, L, R}` / `Call{Fn, Args}` / `Member{Recv, Name}` / `BlockExpr{Block}`。括号分组不入 AST（分组即结合，ch2「parentheses fix the grouping」）。
- TypeRef = `Named{Qual string, Name string, Args []TypeRef}`（裸名/限定名/泛型应用/Dyn 同形） / `Tuple{Elems}` / `Unit` / `FnType{Params []TypeRef, EffectTags []string, Ret TypeRef}`。
- 位置：复用 token 的 1 起 rune 行列（M1 lex 同单位）；节点记起始 token 位置，`Binary` 另记运算符位置（E0104 定位用）。

## D2 API 与首错误停止

`parser.Parse(name string, src []byte) (*ast.File, *diag.Diagnostic, *NotImplemented)`。内部先 `lex.Scan`（token 流 + 文档单元，D10），词法诊断原样上浮。解析期任何 E 诊断 panic-stop（M1 lex 同构）：每文件至多一个诊断、不产出 AST（ch2 parse model「exactly one tree or rejected」、ch21 R2「E 诊断停止管线」）。`NotImplemented{What string}` 是 **非诊断** 的瞬态边界信号（Q1）：解析到达已批准未实现形式时停止并报告形式组名，cli 印 `we: %s are not implemented in this reference build yet` + exit 70。`lex.File` 既有签名语义不动；新增 `lex.Scan(name, src) (toks []Token, docs []DocUnit, d *diag.Diagnostic)` 为主入口，`File` 变薄壳。

## D3 行接续：解析器内嵌判定（无预处理趟）

行接续 **不能** 做纯 token 预处理：构造表达式的 `{` 是括号、块的 `{` 不是（ch2 line-joining 条款），两者在解析上下文之外不可区分——判定必须与解析同置。机制：

- 解析器维护 `bracketDepth`：仅 `(` 分组与调用实参区进入时 +1（M2 的括号上下文只此两种；`[` 列表字面量与构造 `{` 在 **入口处** 即转 ch17/ch8 边界，永不入内）。**类型槽**（`:` 与 `->` 之后）整段按类型文法解析，其中的 `(`/`<` 由类型文法自管，不与表达式深度互扰。
- 边界判定只发生在两处可选继续点：
  1. **二元运算循环**（完整操作数之后、深度零）：下一 token 若在 **新行** 且 **刚消费的 token 不属续行集**（二元运算符 `+ - * / % == != < <= > >= && || & | ^ << >> ..`、`=`、`.`，ch2 封闭表）→ 表达式终止、语句完成；同新行但刚消费 token 属续行集 → 跨行继续（`a +`⏎`b`）；同行 → 按运算符表正常判定。
  2. **块/顶层项循环**（一项完成之后）：遇 `}`/eof 结束；否则若新行 → 新项；同行残余 token → 该项文法未尽，E0105 于残余 token（`let a = 1 2`）。
- **语句起始类**（ch2 封闭类）：ident、字面量（int/float/string/rune 及 true/false）、关键字、`(`、`{`、`!`、`-`、`~`。新项首 token 不属类 → **E0102 于该 token**，消息同时点名上一语句结束行（「pointing at both the token and the end of the previous line」；文件首无前句时措辞取「no preceding statement」）。`[` 不在类内——**行首 `[` 是 E0102**（ch17 列表字面量虽为初等表达式，但语句起始类是封闭类、`[` 不在其中；`let x = [` 中 `[` 处于表达式位，走 ch17 边界，两者不冲突）。
- 括号内换行无意义（ch2 场景）：深度 > 0 时上述两处判定一律不看行号。

## D4 语句族与 `=` 的位置规则

- **绑定**：`let|var` → 名位（ident 或 `_`；`(` → ch8 元组模式边界；关键字 → E0105）→ 可选 `: typeref` → `=`（缺失 → E0105 于实际 token）→ 初值表达式。`_` 名 + 类型注解？`let _: Int64 = e` 形状合法（名位接受 `_`），无诊断。
- **赋值**：语句以 ident 起、**同行** 下一 token 为 `=` → `name = expr`。`x`⏎`= 1`：边界插入后 `=` 起项 → E0102（`=` 属续行集）。
- **`=` 尾随的三分规则**（ch2/ch6 场景钉死）：
  1. **表达式语境**（let/var 初值、调用实参、括号内、return 后）完整表达式后遇 `=` → **E0103 于 `=`**（`let x = y = 1`、`f(a = 1)` 两原例）。
  2. **语句层** 完整表达式语句后同行遇 `=` → 目标非裸名，**E0105 于 `=`**（ch2 场景 `u.name = "bob"` 原例：唯一字段赋值形式是 ch10 的 `self.field`）。
  3. 例外：完整表达式是 `self.field` 形（裸 ident `self` + 单层 `.name`，`self` 非关键字、词法即 ident）→ **ch10 边界**（该形式 ch10 批准，未实现不冒称非法）。
- **return**：`return` / `return expr` 两形（Q2）。函数体语境栈：仅 fn 体内（含嵌套块）合法；体外交遇 `return` → **E0401 于 return**（顶层原例）；所在 fn 无声明返回类型而 `return expr` → **E0402 于 return**（`fn noValue() { return 1 }` 原例）。裸 `return` 于有返回类型函数：解析接受，路径完备性属类型章（ch6 条款明文）。
- 顶层 var → **E0403 于 var**（ch6 原例）；`pub var` → pub 产生式只要 fn/let → **E0105 于 var**（ch6 R5「before any other token sequence」）。

## D5 表达式骨架与 12 级优先表

- **初等**：ident / ch1 字面量 / true/false / `(`grp / `{`块表达式（块即表达式，ch2 核心）/ 一元 `! - ~`（结合紧于一切二元，级 2）。
- **后缀链左结合**（级 1）：`.name` 成员、`(args)` 调用；`?` → **ch14 边界**；`[` → **E0105**（注册表原例 `list[0]`：索引「no production holds it」，永久拒绝非未实现）；`{` → 见构造检测。
- **二元 12 级封闭表**（ch2 原表，级高者松）：3 `* / %`、4 `+ -`、5 `<< >>`、6 `&`、7 `^`、8 `|`、9 `< <= > >= == !=`（非结合）、10 `&&`、11 `||`、12 `..`（非结合，ch5 修订并入）。precedence climbing，逐级左结合；**9/12 级链式 → E0104 于第二个运算符**（`a < b < c`、`x == y == z`、`a..b..c` 三原例；同级混链 `a < b == c` 同罪——「one non-associative precedence level」）。`..` 前缀形不存在（ch5「a binary operator」明文，grep 无反例）→ 表达式位起手 `..` → E0105。
- **构造检测**（ch8 边界）：初等解析出 ident（或 `.name` 链）**未带调用/其他后缀**、同行紧随 `{`、链末名 PascalCase 起头（ch8「a PascalCase identifier … or module.Name」+ E0011 令类型名恒 PascalCase）→ **ch8 边界**；末名小写 → 永非构造头 → **E0105 于 `{`**。泛型构造头 `Box<Int64> {` 的前瞻检测 **不做**（`<` 与比较式同形，检测代价与收益不成比例）——M2 中它按比较式解析，`<` 与 `>` 同属 9 级非结合级，故实际落 **E0104 于第二个运算符 `>`**（实现期以真实二进制复跑确认，修正审查期预判的 E0105），ch8 里程碑（M5）落地构造时自然吸收；作为已知缺口披露于 design 与实现审查，不掩盖。类型槽内泛型闭包与 `>>`/`>=` 的 maximal-munch 合并（`Vec<Vec<Int64>>`、`Map<K,V>= …`）由解析器在 `>` 收口处拆分——类型槽是无歧义语境，`<` 永非比较。
- **括号三分**：`(` 表达式 `)` 分组；`(`起手即 `)` → 单元 → **ch8 边界**；`(` 表达式 `,` → 元组 → **ch8 边界**（于逗号处）。
- **闭包**：表达式位 `fn` → ch12 边界；表达式位 `|`（位级或不能起表达式，唯一 ratified 起手形是 `|params|` 短闭包）→ ch12 边界。

## D6 关键字派发表（41 词穷举，按位置三分）

语句位与顶层位对同一关键字可不同判（顶层位语句非法是 ch6 R1 钉死的 E0105，不能冒边界）。表为权威映射，单测穷举对照：

| 关键字 | 顶层位 | 语句位 | 表达式位 |
| --- | --- | --- | --- |
| let / var / pub / import / as | let 顶层绑定 / **E0403** / pub 前缀 / import 项 / E0105 | 绑定语句 ×2 / E0105 ×3（pub、import 原例） | E0105 ×5 |
| fn | fn 声明 | **ch12 边界**（闭包） | **ch12 边界** |
| mut | E0105 | E0105 | E0105；参数名位 → **mut 参数边界**（「mut parameters」，owning chapter 规范未钉） |
| if / while / loop / break / continue / defer | E0105（语句不进顶层，ch6 R1 原例） | **ch3 边界** | if → **ch3 边界**，余 E0105 |
| else / in | E0105 | E0105（永不起项） | E0105 |
| match | E0105 | **ch4 边界** | **ch4 边界** |
| for | E0105 | **ch5 边界** | E0105 |
| return | **E0401**（原例） | return 语句（D4） | E0105（return 非表达式） |
| true / false | E0105（表达式语句不进顶层） | 字面量 | 字面量 |
| record / byval / byres / newtype | **ch8 边界**（顶层项） | E0105 | E0105 |
| type | **ch7 边界**（别名声明） | E0105 | E0105 |
| interface / impl | **ch10 边界** | E0105 | E0105 |
| where / derives / with | E0105（子句关键字永不起项） | E0105 | E0105 |
| scope | E0105 | **scope 边界**（「scope statements (chapters 13 and 18)」，ch13 scope resource + ch18 scope 块两族） | **scope 边界** |
| resource | E0105（`scope resource` 的第二词，永不首起） | E0105 | E0105 |
| effect | **ch16 边界**（`effect name` 顶层项，ch16 R1） | E0105 | E0105；签名内效果段位 → **ch16 边界** |
| task / select | E0105 | **ch18 边界** | **ch18 边界** |
| case / timeout / collectAll | E0105（永不首起） | E0105 | E0105 |
| foreign | **ch19 边界** | E0105 | E0105 |
| test / mock | **ch20 边界** / E0105 | E0105 / **ch20 边界**（mock 声明是测试块体直项，ch20 R） | E0105 / E0105 |

边界 What 串封闭表：`chapter 3 (control flow) forms` / `chapter 4 (match) forms` / `chapter 5 (iteration) forms` / `chapter 7 (types) forms` / `chapter 8 (composites) forms` / `chapter 10 (interfaces and generics) forms` / `chapter 12 (fn types and closures) forms` / `chapter 13 and 18 (scope) forms` / `chapter 14 (errors) forms` / `chapter 16 (effects) forms` / `chapter 17 (collections) forms` / `chapter 18 (concurrency) forms` / `chapter 19 (ffi) forms` / `chapter 20 (testing) forms` / `mut parameters`。后续里程碑实现对应章时删行，表随 roadmap 收缩到零。

## D7 fn 声明、参数与逗号规则

`[pub] fn name ( params ) [-> typeref] block`。参数：`name: typeref` 对，逗号分隔；**裸参数**（无 `:`）→ **E0105 于参数名 token**，消息点名 `name: type` 产生式与「parameter types are never inferred」（ch6 原例 `fn bad(x)`）。名后泛型子句 `<T…>`（于名字与参数表之间）→ **ch10 边界**；`->` 后/体前效果段（裸 tag 串）→ **ch16 边界**；`mut` 于参数名位 → mut 参数边界（D6）。实参表/参数表/元组类型元素/fn 类型参数表 **允许尾随逗号**（ch2 Examples 多行调用 `port,`⏎`)` 形 + M1 属性参数 D7 同裁；规范无「trailing comma」明文，全 grep 证实——统一规则记为机制决策）。返回类型存在性仅入 AST（E0402 判定用），类型检查 M3。

## D8 import、命名约定（E0012/E0013）

- import：`import path [as alias]`，path = 点分隔 ident 段（词法形状），每段 **`^[a-z][a-z0-9]*$`**（E0013；「lowercase with dot separation」+ 全规范路径例证 grep 无下划线/大写/连字符——连字符词法上就不成 ident）。alias 同 charset。违规 → **E0013 于该段 token**（ch6 原例 `import Models.User`）。引入名 = alias 或末段；非法命名的 import 不引入名（先 E0013 停止）。`as` 后非 ident → E0105。
- E0012（camelCase = `^[a-z][a-zA-Z0-9]*$`）：fn 名、参数名、let/var 绑定名（顶层与块内；「variables, functions, and methods」+ ch16 R1「The name is camelCase … (E0012)」佐证函数族恒 camelCase）。`_` 豁免（标点非 ident）。违规 → **E0012 于名字 token**。**E0011 无 M2 触发点**：类型名绑定（type/record/… 声明）全是后续章节形式；类型 **引用** 位的非 PascalCase 名走 E0105（ch7「a named type is a PascalCase identifier」+ 该章 E0105 场景），非 E0011（那是声明处码）。

## D9 模块名字空间（E0404）

一张 `map[name]firstPos`：fn 名、顶层 let 名、import 引入名，按出现序登记；后来者撞名 → **E0404 于后来名字 token**，消息点名两处行号（ch6「rejects the later one」）。块内名不入此表（ch6 R7 明文）。

## D10 文档单元与 E0405

`lex.Scan` 旁路产出 `DocUnit{StartLine, EndLine int, Lines []string}`：连续 `///` 行合一个单元（空行或普通注释断开即新单元）；注释仍不产 token（ch1 不动）。附着取 **章文直读**：单元附着于其后首个顶层项（项起始行 > 单元末行）；「blank lines and ordinary comments … do not break the attachment」是章文明示的 **不** 破坏项，非破坏条件枚举是发明——块内 `///` 因此也附着于其后首个顶层项（直读结果，记为机制解释；doc 渲染属 M11）。文件内其后再无顶层项 → **E0405 于单元首行 1 列**。两个单元可附同一项（无禁止条款，渲染属工具章）。

## D11 `we check` 接线与边界行

runCheck 新序：E1907（不动）→ 目录 → 项目边界 70（M3 不变）→ 非 .we → usage 2（不变）→ 读失败 → fsError 1（不变）→ `parser.Parse`：诊断（词法或语法，含 E0012/E0013/E0102–E0105/E0401–E0405）→ 双面报告 + 1；`NotImplemented` → `we: %s are not implemented in this reference build yet` + 70（stderr 一行，--json 下 stdout 仍空——瞬态实现状态非规范表面，M1 同裁）；干净解析 → `we: type checking is not implemented in this reference build yet` + 70（阶段级边界从「解析」收缩为「类型检查」，M3 接管）。M1 的「parsing is not implemented」行删除，check-clean(-json) 两黄金随改写。

## D12 E0105 消息表（注册表义务：点名 token 与「该位置考虑过的产生式」）

| 位 | 消息点名考虑过的产生式 |
| --- | --- |
| 顶层项首 | `top-level items: import, fn, let (with pub)`（ch6 原例措辞「naming the item productions considered」） |
| 顶层 pub 后 | `pub prefixes fn and let` |
| 表达式位 | `expressions: identifier, literal, (e), { block }, unary ! - ~`（按到达点裁剪） |
| 后缀位遇 `[` | `postfix: .name, (args); indexing is by named methods (chapter 17)` |
| 语句层 `=`（非 self 目标） | `assignment targets are bare names` |
| 参数位裸名 | `parameter wants name: type; parameter types are never inferred` |
| 类型槽 | `type references: Name, module.Name, Name<T…>, (T1, …, Tn), (), Dyn<I>, fn(…) -> T` |
| 绑定头缺 `=` / 名位非法 | `binding wants let|var name [: type] = expr` |

E0102/E0103/E0104 消息以注册表标题起头，点名字符串统一双引号（M1 一致性先例）。位置表：E0102 于违规 token（消息含上一语句行号）、E0103 于 `=`、E0104 于第二个链式运算符、E0105 于违规 token、E0401/E0402 于 `return`、E0403 于 `var`、E0404 于后来名、E0405 于单元首行、E0012/E0013 于名字/段 token。help 取注册表 remediation 压缩（M1 helps 同构，embed 落地前过渡，roadmap 待办 2）。

## D13 E0101 无 M2 触发输入（论证）

解析器为确定性递归下降：每个判定点键于 token 种类 + 行事实 + 深度，接受序列恰产一棵树（ch2 parse model 的「exactly one tree」由构造保证）。12 级表 + 封闭续行集 + 语句起始类下无任何双解序列（C/JS 式 `f(x)\n(y)` 陷阱被深度零边界规则钉死为两语句——ch2 场景明文）。E0101 是第 0 章原则 4 的 **阀门**：为未来可能引入双解表面的章节预留，本变更零触发、零黄金用例（注册表条目照引、help 照嵌）。若后续章节引入真双解，走该章 spec 层变更并在此段补触发例。

## D14 conformance 黄金用例（期望全部手钉自规范场景）

| 用例 | 输入要点 | 期望锚点 |
| --- | --- | --- |
| check-parse-clean | ch2+ch6 全形模块（import×2、pub let、pub fn 带注解与返回、块值、优先级混合、后缀链、尾随逗号多行调用） | exit 70，type checking 边界一行 |
| check-parse-clean-json | 同上 + `--json` | stdout 空、边界行 stderr |
| check-e0102-leading-dot | `svc`⏎`.query()`（ch2 原例） | E0102，双位置（token + 上语句行） |
| check-e0103 | `let x = y = 1`（ch2 原例，fn 体内） | E0103 于 `=` |
| check-e0103-call | `work(a = 1)`（ch2 原例） | E0103 |
| check-e0104 | `let ok = a < b < c`（ch2 原例） | E0104 于第二个 `<` |
| check-e0104-range | `let r = a..b..c` | E0104 |
| check-e0105-index | `let first = list[0]`（注册表原例） | E0105 于 `[`，点名命名方法索引 |
| check-e0105-top-stmt | 顶层 `compute()`（ch6 原例） | E0105，点名顶层项产生式 |
| check-e0105-field-assign | `u.name = "bob"`（ch2 原例） | E0105 于 `=` |
| check-e0105-bare-param | `fn bad(x) { }`（ch6 原例） | E0105 于参数名 |
| check-e0105-json | E0105 + `--json` | JSON 事件全字段（file/line/column/help） |
| check-e0401 | 顶层 `return 1`（ch6 原例） | E0401 |
| check-e0402 | `fn noValue() { return 1 }`（ch6 原例） | E0402 |
| check-e0403 | 顶层 `var count = 0`（ch6 原例） | E0403 |
| check-e0404 | `fn f` ×2（ch6 原例） | E0404 于后来者 |
| check-e0405 | 尾部孤儿 `///`（ch6 场景） | E0405 于单元首行 |
| check-e0012 | `fn Bad_Name()` | E0012 于名字 |
| check-e0013 | `import Models.User`（ch6 原例） | E0013 |
| check-boundary-ch8 | fn 体内 `User { id: 1 }` 构造 | `chapter 8 (composites) forms` 边界行 + 70 |
| check-boundary-ch8-json | 同上 + `--json` | stdout 空、边界行 stderr |

改写：check-clean / check-clean-json（骨架内容 `Ok(())` 中 `()` 单元表达式命中 ch8 边界行——同输入新期望，随本变更记录披露）；实现期追加第三例同源改写 **check-bom-crlf**（同骨架输入 + BOM/CRLF 归一化冒烟，旧期望钉住「parsing 未实现」边界行，管线推进后随改 ch8 边界行——发现于 T7 接线后的黄金复跑）。M1 其余 32 例不动（check-boundary / check-boundary-json 目录边界两例保持原样，新边界用例命名避开）。

## D15 被拒替代方案

- **行接续预处理趟**：构造 `{` 与块 `{` 在解析外不可区分（D3），预处理必错；判定内嵌是唯一忠实实现。
- **未实现形式报 E0105**（Q1 弃）：对已批准形式报「不合任何产生式」与注册表 E0105 语义相悖。
- **AST 由 parser 包私有**：M3 类型检查器立即需要；独立 ast 包避免 M3 反向依赖 parser。
- **括号节点入 AST**：分组即结合，冗余节点只增不变量。
- **泛型构造头前瞻检测**（`Box<Int64> {`）：`<` 与比较式同形，为一条边界行做此检测不值；披露为已知缺口由 ch8 里程碑吸收（D5）。
- **尾随逗号一律拒绝**：与 ch2 Examples 多行调用形及 M1 属性参数先例相悖，且规范无明文禁令（D7）。
- **E0405 附着加「破坏项」条件**（块内 `///` 视孤儿）：章文只枚举 **不** 破坏项，发明破坏条件越权；直读结果记为机制解释（D10）。
- **E0101 造用例**：无真双解输入，造例即 mock/捷径掩蔽；论证替代（D13）。
