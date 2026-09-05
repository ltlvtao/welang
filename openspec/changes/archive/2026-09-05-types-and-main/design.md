# design.md — types-and-main

机制自由度的固定记录。每条决策给出规范依据；无依据处显式标注为机制读法并给出论证。位点锚点（行列）按 M2 先例在单测与黄金中钉死，此处定锚点规则不定具体行列。

## D1 包与类型表示

新包 `internal/typecheck`。类型值是不可变结构，一个封闭集合：

- `Base(name)`——13 基类型（`Int8 Int16 Int32 Int64 UInt8 UInt16 UInt32 UInt64 Float32 Float64 Bool String Bytes Rune`；ch7 R1 原文点名的十三个，`Never` 与 `()` 明文不是基类型）；
- `Unit`——ch8 单元类型，唯一值 `()`；
- `Never`——ch9 底类型，无值；
- `Named(decl, args)`——具名声明的应用：`decl` 指向模块内和式声明（M3 唯一具名种类）或内建和式（D9），`args` 为空当声明无子句；
- `Tuple(elems)`、`Fn(params, tags, ret)`——注解槽可写的另两形（ch7 R5）。

类型相等是结构相等：`Base` 比名、`Named` 比声明同一性 + 实参逐位、`Fn` 比参数表 + 效果标签表 + 返回（ch12 的函数型等位「exactly matching signature」由结构相等承载，其场景化检查属 M5）。无子类型、无方差——ch10 的 no-subtyping 立场经 ch12 文本明文延伸到函数型，M3 同构执行。

## D2 作用域链

三层由内向外（ch15 R4）：**块局部**（块语句栈；参数域是函数体的最内层；`let`/`var` 从其语句起生效——ch8 遮蔽的操作内容：同域重绑定合法、后绑定胜、最近绑定解析）→ **模块单一名字空间**（M2 的 `names` map 升格为符号表：fn、顶层 let、和式类型名、变体名、import 引入名；E0404 仍只在模块层）→ **预导入外层域**（D3）。具名解析失败在查尽三层后报 E1304。局部名遮蔽模块名与预导入名一律合法无诊断（ch15 R2 场景钉死）；模块名遮蔽预导入名同合法。

函数体内 `return e` 的 `e` 在参数域 + 块域内定型；顶层 let 的初始化式只有模块域 + 预导入域（无局部）。

## D3 预导入封闭表（30 名，ch15 R2 原文清点）

13 基类型名 + `Never` + `Dyn` + `Result` `Ok` `Err` + `Option` `Some` `None` + `List` `Map` `Set` + `panic` `todo` `assert` + `Shareable` + `currentCancelSignal` + `advanceTime`。逐名 M3 处置：

| 名 | 处置 |
| --- | --- |
| 13 基类型名 | `Base` 类型，全量可用 |
| `Never` | 类型可用（仅函数返回注解位，E0703 余位拒绝） |
| `Result` `Ok` `Err` `Option` `Some` `None` | D9 内建和式，全量可用 |
| `List` `Map` `Set` | 名已登记；类型位或值位使用 → 边界行 `std collection types (chapter 17)` |
| `panic` `todo` `assert` | 使用 → 边界行 `termination functions (chapter 14)` |
| `Dyn` | 使用（含 `Dyn<I>` 形）→ 边界行 `Dyn boxes (chapter 10)` |
| `Shareable` | 使用 → 边界行 `Shareable markers (chapter 18)` |
| `currentCancelSignal` | 使用 → 边界行 `task-scope and time-control functions (chapters 18 and 20)` |
| `advanceTime` | 同上一行（与 `currentCancelSignal` 共用一组措辞） |

封闭性：表中名被本地声明遮蔽时本地胜（无 E0404）；表中之外的名永不由预导入解析。`Ok`/`Err`/`Some`/`None` 是**变体名**（构造机制 D8），不是独立类型名——`Ok` 出现在类型位 → E1304。

## D4 字面量定型、E0501 位点与运算符域

**字面量定型 context-free**（ch7 R2 + R3 场景钉死）：无后缀 `42` → `Int64`、`3.14` → `Float64`；后缀按 ch1 集合名型；`true/false` → `Bool`、字符串（含插值孔，孔内不定型——D17）→ `String`、rune → `Rune`。注解不回染字面量：`let n: Int32 = 42` 是 E0501（Int64 表达式对 Int32 注解），这正是 R3 的场景原文。

**E0501 协议位与锚点**（R3「值必须与槽一致的每一位置」；四个点名位 + 机制读法覆盖位）：

| 位 | 锚点 |
| --- | --- |
| 二元运算符两侧型不一致 | 运算符 token |
| 一元 `!` 非 Bool、`-` 非数值、`~` 非整型 | 运算符 token |
| 绑定注解 vs 初始化式 | 绑定名 token |
| 实参 vs 形参（fn 调用与变体构造同权） | 实参表达式首 token |
| `return e` 的 e vs 声明返回 | `return` 关键字 |
| 赋值 `x = e` 的 e vs x 的型（机制读法：R3 的「every position」覆盖赋值这一槽位；ch8/ch12 后续点名不改此读法） | 名 token |
| 有值函数体尾表达式 vs 声明返回（D6） | 体尾表达式首 token |

**运算符域表**（机制读法，论证：ch2 只批优先级封闭表；ch7 只批「同型才可组合」及其场景——数值算术、同型比较等号、Bool 逻辑；无任何章节固定各运算符接受的类型集，`String + String` 是否语言特性无规范答案——roadmap follow-up #4 登记）：

- `+ - * / %`：两同型数值（整型或浮点）；
- `& ^ | ~ << >>`：两同型整型（`~` 一元同域）；
- `< <= > >=`：两同型**数值** → `Bool`（String 序走方法，未批准则不批）；
- `== !=`：两同型（ch7 场景含 String 比较）→ `Bool`；
- `&& ||`、一元 `!`：`Bool`；
- `..`：任何出现 → 边界行 `range expressions (chapter 11)`（ch11 owns）。

域外组合（如 `"a" + "b"`、`"a" < "b"`）→ 边界行 `arithmetic and comparisons beyond the ratified numeric and Bool domains (spec gap; roadmap follow-up)`，exit 70。同型但域外不是 E0501（E0501 是「两型须一致而不一致」）；域外是「无已批准定型」——诚实报告为不可查，不私定接受或拒绝。

## D5 E0502 常量折叠

**折叠集**：整型字面量（含后缀与 based 形）、一元 `-`、二元 `+ - * / % << >> & ^ |`，括号自然归组；整型两侧必须已同型（E0501 先判）。求值用任意大精度有符号整数（机制选择），完成后按**表达式自身的型**（非注解——D4 的 context-free 原则）做一次范围检：越界 → E0502，锚点在产生越界值的运算符 token（裸字面量越界锚在字面量）。示例：`let x = 127i8 + 1` → E0502；`let x = -128i8` → 干净（整表达式折叠后未越界）；`let n: Int32 = 42` → E0501 而非 E0502（先于折叠判一致）。浮点表达式永不 E0502（IEEE 不查，ch7 R4 原文）；常量除零/模零不查（E0502 只批溢出；陷阱语义属错误机制章）；负移位数视为不可折叠（跳过范围检，不定型错误）。

## D6 块值与函数体尾四分规则（ch7 R7 + ch8 两小片 + E0605）

**块值**（R7）：块作为表达式，其值 = 最后一个条目的表达式型；最后条目非表达式（绑定/赋值/return）或块空 → `()`。

**函数体尾四分**（有声明返回的 fn；机制读法：R3 的返回位 + R7 块值的合取）：

1. 尾条目是有值 `return` → 合法，e 已在 D4 表判过；
2. 尾条目是裸 `return` → E0501（`()` 对声明返回型），锚点 `return` 关键字（ch6 让裸 return 过解析、「exhaustiveness is the types stage's」——此即类型阶段的落点）；
3. 尾条目是表达式 → 其型 vs 声明返回，E0501 于表达式首 token；
4. 尾条目是绑定/赋值或体空 → E0501 锚点 **fn 关键字**（函数声明起点 token，含 pub 前缀时为 pub；体不产值这一事实锚在声明上）。

**无声明返回的 fn**：体尾为非 `()` 型表达式 → E0605 锚该表达式（ch8 Value discard 场景原文位）。**表达式语句**：型非 `()` → E0605 锚该表达式。`let _ = e` 任意型合法（显式丢弃）；`let x = e` 与赋值照常定型。控制形体尾与语句位 match 臂两个 E0605 位 M5 随形式抵达。

## D7 ch9 和式声明解析

`parseItem` 的 `type` 分支从边界行改为 `parseSumDecl`；`bndTypes` 常量与派发表该行删除。文法按 R1 原文：

- 前缀：可选 `pub`，随后可选 `byval`，序固定（机制读法：R1 只说「optionally prefixed by pub and byval」；骨架与场景例均为 pub 在外。`byval pub type` → E0105 指明前缀序）；
- `type Name =`：名 PascalCase 否则 E0011（E0011 注册表明文「Sum type names and variant names」）；名入模块名字空间（E0404）；
- 名后 `<` → 泛型子句，`bndGener` 边界（Q1 裁量，M6）；
- 变体表：`V` 或 `V(T1, …, Tk)`；变体名 PascalCase → E0011、入名字空间 → E0404；载荷为 `parseTypeRef` 全量复用（M2 已落地）；k > 8 → E0701 锚变体名；载荷内 `Never` → E0703（D8 位点表）；
- 最后变体后 `derives` → `bndGener` 边界（M6）；
- **行接续**：`=` 与 `|` 在 ch2 续行集内（二元运算符），故 `type Shape =` 换行续、变体行尾随 `|` 续——解析循环在项中部跨行无深度零检查，天然成立；**行首 `|`** 的机制：变体循环遇 `|` 且 `p.brokeLine()`（`|` 起新行）即**终止声明返回**，`parseFile` 的 `checkStart(true)` 对 `|`（不在语句起始类）报 E0102——ch9 场景原文的码与措辞由此免费获得，不新增机制；
- 单元变体名可与预导入名同名（如 `type T = Int64` 是「变体名为 Int64 的单变体和式」）：语法完全合法，ch15 R4 的模块层遮蔽语义处理使用处；design 记录此边界例，黄金钉一枚。

语句位 `type` 维持 E0105（和式是顶层项）。

## D8 构造机制与 E0703 位点

**裸 PascalCase 标识符**解析（值位）：按 D2 链解析——命中变体名：单元变体 → 该和式类型的值；带载变体 → E0704 锚该名（ch9 R2 原文）；命中类型名（如 `Shape` 裸用于值位）→ E1304（具名解析的值位读法：类型名非值；机制读法，黄金钉）；命中预导入变体（`None` 等）→ D9。裸 camelCase → 局部/fn/顶层绑定照常。

**调用形 `Name(args)`**：被调名解析——变体名：构造（见下）；模块 fn：按签名逐实参判 E0501（D4 表）；具名单参 fn 的裸名作**值**（`let g = add`）→ 边界行 `function values (chapter 12)`（ch12 M5）。实参**个数**与被调非函数值两形无诊断码（E0304 只管 match 模式）——roadmap follow-up #5 登记，M3 以边界行处理：`calls with an argument count the callee does not declare (spec gap; roadmap follow-up)` 与 `calls on values that are not functions (spec gap; roadmap follow-up)`，exit 70。

**期望类型线程**（构造与字面量定型的合流点；机制读法：R3 四协议位是「槽定型」位，槽型向下传入表达式是检查器的自然实现）：绑定注解、`return` 位、实参位（形参型）、构造载荷位四处向下传期望型。内建和式（D9）由此定参；用户和式非泛型、无需推断。期望型**不**改变字面量定型（context-free 不破）。

**E0703 位点表**（ch9 bottom type 原文枚举 + fn 型参数位经 ch12「参数位持类型引用」同构）：绑定注解 `let x: Never`、参数位 `fn f(x: Never)`、变体载荷 `type X = Stop(Never)`、元组元素 `(Int64, Never)`、fn 型参数位 `fn(Never) -> ()`——全部 E0703 锚该 `Never` 名；合法位唯二：fn 声明返回注解、fn 型返回位 `fn() -> Never`（后者 ch12 场景）。`Result<Int64, Never>` 的归属：**E1204**（论证：ch14 R1 的约束是「E 位必须具名和式」，Never 非具名和式即违反该约束；E0703 的 fires-under 枚举（绑定/字段/参数/载荷/元组元素）是封闭枚举，泛型实参位不在其中；ch14 对 E 位的专门约束更近）——design 钉死此读法，黄金两枚（E1204 命中 + E0703 枚举位各一）。

## D9 内建 Result/Option

预导入层注册两个**内建和式声明**（与用户声明同结构、不可遮蔽其变体语义——除非本地整名遮蔽）：`Result<T, E> = Ok(T) | Err(E)`（ch14 R1 原文）、`Option<T> = Some(T) | None`。构造与用户和式完全同权（Q1 裁量）；差别仅在带泛型参数：

- **E1204**：`Result` 应用的第二实参解析后非具名和式（基类型/`()`/元组/fn 型/`Never`）→ E1204 锚该实参的型引用首 token。具名和式含用户和式与内建和式自身（`Result<(), Result<(), E2>>` 的 E 位是内建和式——具名，合法）。record/interface/Dyn 位M6/M5 后自然并入此判据；
- **E0828**：应用实参数 ≠ 声明子句元数 → E0828 锚 `<` token（`Result<Int64>`、`Option`、`Option<A, B>`；用户零子句和式的应用 `MySum<Int64>`（0 对 1）同码——ch10 该 Requirement 的原文「differ in count from the declaration's clause」覆盖零子句声明）；
- **E0827**：构造无期望型且参数无法定全（`let x = Ok(())` 的 E 未定、裸 `None`/`let x = None` 的 T 未定）→ E0827 锚构造名（ch10「generic call does not determine its type arguments」；内建和式构造在 M3 就是该 Requirement 的触达面）。期望型线程（D8）到达时自上而下定参，不从 E0827。

## D10 名字解析与 import 的 M3 处置（E1302/E1304/E1303）

**E1304 三形**（ch15 R4 + 注册表描述）：裸标识符查尽三层无果；限定形 `name.item` 的 `name` 非 import 引入名；类型位同规则（`let x: Foo` 无 Foo → E1304 锚该型引用）。锚点均为该名 token。

**import 的处置表**（单模块项目 + 单文件两模式，Q3 裁量）：

| 输入 | 项目模式（`we check .`） | 单文件模式（`we check f.we`） |
| --- | --- | --- |
| `import std.…`（任何 std 首段，含裸 `import std`） | 边界行 `standard-library modules (chapter 15)`（本构建未提供任何 std 模块；E1302 的 std 子句描述 std 存在后的稳态，此前诚实报不可查） | 同左 |
| 本地 import，期望路径文件**不存在** | E1302，消息含映射期望路径（`src/util.we` 等，ch15 R1 映射：非末段 = 目录、末段 = `.we`、相对 src/） | E1302 单文件措辞：无源根存在，消息含「would expect … under a source root; run project mode」要义 |
| 本地 import，文件存在 | 边界行 `multi-module programs (chapter 15)`（多模块编译 M6） | 不可能（无源根，上一行已 E1302） |

import 引入名照常入模块名字空间（E0404 机制 M2 已有），但 M3 内任何 import 都不能成功绑定模块内容——故限定形的限定符必为「非 import 名」→ E1304。**E1303 无 M3 触发面**（跨模块 pub 检查需被导入模块的内容，全部路径先被上表截断）——声明式披露，码与机制随 M6 落地。

## D11 项目模式与 manifest（ch21 R1/R8）

`we check <dir>`：读 `<dir>/we.toml` → 源根 `<dir>/src/` → 根模块 `<dir>/src/main.we`。tests/ 目录不编入（M2 骨架行为既有注释的兑现）；src/ 下其他 .we 文件仅在 import 抵达时按 D10 处置，不被主动编译。

**极简 TOML 读取子集**（机制读法：R8 只批三个必需键 + 若干值域；读取器只认骨架所需文法——顶层与表头下的 `key = "value"` / `key = word` / `key = 整数` 行、`[table]` 头、注释与空白；无法解析的行**忽略**（前向容忍，未知键同忽略）；必需键的值不可读 → E1905 点名该键）。触发表：

| 检查 | 码 | 锚点 |
| --- | --- | --- |
| we.toml 不存在 / name、version、type 任一缺失 | E1905（消息点名缺失键） | manifest 路径 1:1 / 所在键 |
| name 空 或 字符集非 `[a-z0-9-]` | E1904（roadmap follow-up 1 的现行规则） | name 键 |
| version 非三段点分非负整数（无前导零） | E2004 | version 键 |
| type 非 `executable` / `library` | E1903（消息列合法值与实得值） | type 键 |
| `explore-iterations` 非正整数 / `[vet]` 条目非 warning/error/ignore | E1903 | 所在键 |

**根模块缺文件** → E1305（读法论证：E1305 的补救「Declare exactly one pub fn main() in src/main.we」以根模块文件为预设；文件缺失使约定无从谈起，是同一 Requirement 的最浅违反；E1302 的 fires-under 是 import 路径，不覆盖根模块）。锚点：期望路径 1:1，消息点名 `src/main.we`。

单文件模式（`we check f.we`）无 manifest、无 main 要求：文件即模块，跑词法 → 解析 → 类型检查，干净即成功（D13）。`we check` 对非 `.we` 文件维持 M0/M2 usage 错误。

## D12 E1305 main 形状判据链（ch15 R6）

对根模块符号表按序判：存在名为 `main` 的 fn → 无 → E1305 锚 1:1「declares no fn main」；非 `pub` → E1305 锚声明首 token；参数表非空 → E1305 同锚；无返回注解或返回注解非 `Result<(), E>` 形（`Result` 须解析到**预导入内建**——本地 `type Result = …` 遮蔽后即非该形 → E1305；第一实参须为 `()`）→ E1305 同锚；形合而 E 位非具名和式 → **E1204**（注册表明文划界「A well-shaped main whose error position violates the named-sum constraint is E1204's, not this code's」）。多个 `main` 由解析期 E0404 先触（一名字空间）。效果段场景（R6「effect segment conforms」）：带效果段的 main 在 M3 抵达不了——解析在 `effect` 关键字先触 `bndEffect` 边界（ch16，M7）——该场景 M7 前不可达，披露不测。

## D13 成功输出与管线级（ch21 R2 + Q4 裁量）

干净检查三面：默认**静默** exit 0；`--json` 零事件 exit 0；`--verbose` 一行 `we: check passed (1 module)`。管道可观察次序保持 R2：词法 → 解析 → 模块解析（M3 = manifest + import 处置 + main 形状）→ 类型检查 →（效果/所有权/文档检查级）。**边界按「形式到达」触发**（M2 既立原则的推广，机制读法）：效果/所有权/文档三级在 M3 可解析子集内没有任何待查形式存在（无效果段、无资源、doc 附着已在解析期完成）——故三级不产生边界行，骨架端到端可绿；这是「not-implemented 边界随里程碑收缩到零」路线在级维度的表述。类型检查自身成功后不再有尾边界（M2 的「type checking 未实现」行删除）。

## D14 边界 What 封闭表（checker 级新增 + M2 行增删）

新增（模板 `we: %s are not implemented in this reference build yet`，均复数名词短语）：

| What | 触达 |
| --- | --- |
| `std collection types (chapter 17)` | List/Map/Set 使用 |
| `termination functions (chapter 14)` | panic/todo/assert 使用 |
| `Dyn boxes (chapter 10)` | Dyn 使用 |
| `Shareable markers (chapter 18)` | Shareable 使用 |
| `task-scope and time-control functions (chapters 18 and 20)` | currentCancelSignal/advanceTime 使用 |
| `function values (chapter 12)` | 具名单参 fn 裸名作值 |
| `range expressions (chapter 11)` | `..` 出现于已定型表达式 |
| `standard-library modules (chapter 15)` | import std.*；方法访问——接收者名可解析（任一层）时的 `.` 成员位（实现披露：方法清单是标准库的，M4 起抵达；接收者名不可解析的裸标识读 E1304 限定符形，见 D10） |
| `multi-module programs (chapter 15)` | 项目模式本地 import 且文件存在 |
| `arithmetic and comparisons beyond the ratified numeric and Bool domains (spec gap; roadmap follow-up)` | D4 域外组合 |
| `calls with an argument count the callee does not declare (spec gap; roadmap follow-up)` | 实参个数不符 |
| `calls on values that are not functions (spec gap; roadmap follow-up)` | 非函数被调 |

删除的 M2 行：`chapter 7 (types) forms`（`type` 声明落地）、`()` 单元值的 `chapter 8 (composites) forms` 行（单元值落地；元组表达式/构造花括号的 ch8 行保留至 M5）、CLI 尾边界 `type checking` 行。其余 M2 解析边界（ch3/ch4/ch5/ch10 子句/ch12/ch16/ch17/ch18/ch19/ch20/mut 参数）原样保留——解析边界在解析期先触，checker 不会见到那些形式的树。

## D15 F3：closeAngle 与 ch10 分隔收口规则的符合性修正

**证据**（`docs/spec/1000-interfaces.md`，Generic type references and inference）：「Nested closers must be separated: maximal munch takes `>>` as the shift token (chapter 1), so the nested application is written `Box<Box<Int64> >`, and the unseparated `Box<Box<Int64>>` is rejected under chapter 2's `E0105` with that remediation — this discharges the pointer chapter 1's operator inventory left to the generics chapter.」M2 design D5（及 F2 修复）在收口位拆分 `>>`/`>=`，接受了规范明文拒绝的 `Vec<Vec<Int64>>`——实现与已批准规范相反，且 M2 的测试与黄金钉死了错误行为。M3 的类型检查恰好抵达泛型应用位，修正随本变更测试先行落地：

- `closeAngle` 删除 `>>`/`>=` 分支，收口位只认裸 `>`；
- 收口位遇 `>>`/`>=` → E0105，消息含分隔书写补救（要义：`nested closers are separated — write Name<Name<Int64> >; ">>" is the shift operator (chapter 1)`），锚该 token；
- 合法面新增 `Vec<Vec<Int64> >`（分隔形）；`let m: Map<String, Vec<Int64> > = never` 需 `> =` 间空格或换行——`>=` 熔合同拒（同一 maximal-munch 原则，ch10 文字只例示 `>>`，机制同构推广，披露）；
- 测试翻转清单：parser_test TestTypeRefs 的 `Map<String, Vec<Int64>>`、`Vec<Vec<Int64>>` 两枚 clean 用例翻为 E0105；`>>=`/`>=` 两枚 junction 用例翻为 E0105；新增分隔形 clean 用例；D14 黄金同步。

此修正在 proposal Why、tasks 完成记录、归档与提交信息中四处置 disclosed。

## D16 黄金用例表（约 28 枚，章节场景原例优先）

骨架绿（项目模式端到端，fixture 目录）：1 枚。E0011（和式名/变体名各一）：2。E0404（变体对 fn 撞名，ch9 场景原例）：1。E0701（九载荷）：1。E0703（枚举位 + fn 返回合法对照）：2。E0704（带载裸用）：1。E0501：注解位（`let n: Int32 = 42`，ch7 原例）、运算符位、实参位、返回位、赋值位：5。E0502：`127i8 + 1`、裸字面量越界：2。E0605：表达式语句、无声明返回体尾：2。E1204：`Result<(), String>`（main 形外）、E0703 论证位：2。E0828：`Result<Int64>`、零子句应用：2。E0827：`let x = Ok(())`：1。E1302：单文件本地 import、项目缺文件：2。E1304：裸名、类型位、限定符：3。E1305：缺 main、非 pub、错形：3。E1905/E1904/E2004/E1903：4。E0105 分隔收口（F3）：2（未分隔拒 + 分隔受）。边界行代表（std import、List、panic、多模块）：4。改写：check-clean / check-clean-json（现期望 `()` ch8 边界 70——`()` 转合法后失效，且其文件缺 `AppError` 声明将成 E1304；改写为带和式声明的真干净文件 → 静默 exit 0 / `--json` 零事件）与 M2 触及 F3 面的其余用例。总计约 40 枚（新增 + 改写），实现时按此表逐枚登记。

## D17 被拒方案与披露

- **上下文相关字面量定型**（注解回染 `let n: Int32 = 42`）：ch7 R3 场景原文反例直接拒绝——那是 E0501，不是重定型。
- **保留 closeAngle 拆分**（以 M2 事实表面为由）：ch10:334 明文拒绝，规范优先；M2 的钉死测试一并翻转（F3）。
- **私定运算符全域接受 / 全域 E0501**：无条款支撑接受（`String + String` 未批为特性）也无码支撑拒绝——边界行 + follow-up #4 是唯一不发明行为的路径。
- **私定实参个数 E0501 化**：E0501 注册表描述是型不一致，个数不符非其义——边界行 + follow-up #5。
- **多模块最小符号导出**（让 E1303 有触发面）：超出单模块切片，M6 一次性落地更小可验证。
- **注册表 go:embed**：维持 follow-up 2 的调用点硬编码。
- **插值孔内定型**：无条款（ch7 只批字面量整体 String）——不查不报也非边界（字面量本身完全合法），披露即可。
- **效果/所有权/文档级空转出边界**：会令骨架永不绿，与 M3 里程碑目标（`we check .` green on skeleton）直接冲突；按「形式到达」原则（D13）处理。

## D18 测试策略

parser 修订：派发表 `type` 行翻转、TestTypeRefs F3 翻转、和式声明与行接续新测（含行首 `|` E0102、`type T = Int64` 遮蔽边例）。typecheck 单测：按 D3/D4/D6/D8/D9/D12 的表逐行设例。conformance：D16 表全量 + 真二进制证据（退出码、JSON 协议面、静默面）。红先行：T2/T3 对当前构建必须红（F3 翻转用例即天然红——现行构建接受未分隔形）。
