# types-and-main — 第 7 章类型子集、第 15 章根模块与 main 形状、第 14 章 Result：`we check .` 对骨架绿

## Why

M2（parser-core，归档 2026-09-05）交付了解析阶段：`we check <file>.we` 对首个词法/语法诊断报告并 exit 1，干净解析停在 `we: type checking is not implemented in this reference build yet` 边界（exit 70）。按 roadmap M3 行（薄垂直优先），下一阶段是类型检查的最小可观察切片，其规范依据已全部批准：

- 第 7 章（`docs/spec/0700-types.md`）：Base type inventory（13 基类型封闭集）、Literal typing（无后缀默认 Int64/Float64，后缀名型，关键字与定界字面量自定型）、No implicit conversion（E0501 覆盖「值必须与槽一致的每一位置」）、Integer overflow semantics（E0502 常量可评估溢出为编译错误）、Block value typing；
- 第 8 章两小片（`docs/spec/0800-composites.md`，范围按裁决 Q2）：单元类型 `()`（一个值；无值块即 unit）与 Value discard（E0605——表达式语句、无声明返回函数体尾、控制形体尾、语句位 match 臂）；Local shadowing 的操作内容随作用域链落地（场景钉死属 M5）；
- 第 9 章（`docs/spec/0900-sum-types.md`）：Sum declarations（R1，范围按裁决 Q1）与 Variant constructors（R2）、The bottom type 的 E0703 位；
- 第 14 章 R「The Result type」（`docs/spec/1400-errors.md`）：`Result<T, E> = Ok(T) | Err(E)` 为普通泛型和式，E 位必须具名和式（E1204）；
- 第 15 章（`docs/spec/1500-modules.md`）：Module paths and file resolution 的 src/ 源根与根模块、The prelude 封闭集、Name resolution（R4 由内向外，E1304）、The main convention（R6，E1305）；Cross-module visibility（R3）在本切片诚实无触发面（见非目标）；
- 第 21 章（`docs/spec/2100-toolchain.md`）：R1 项目模式（目录 = 项目根，源根 = src/；单文件模式 std 内建、其余 import → E1302 措辞）、R2 管道次序可观察、R8 manifest 校验（E1905/E1904/E2004/E1903）；
- 注册表 `docs/spec/diagnostics.toml`：E0501/E0502（ch7 段）、E0605（ch8 段）、E0701/E0703/E0704（ch9 段）、E1204（ch14 段）、E1302/E1304/E1305（ch15 段）、E1903/E1904/E1905、E2004（ch21/ch22 段）、E0011（ch1 命名块，类型声明落地时生效——M2 显式缓期的兑现）、E0827/E0828（ch10 段，被构造机制与本切片的泛型应用位强制触达，同 M2 之 E0012/E0013 先例）。

可验证的黑盒缺口：今天没有任何输入能产生上列任何诊断——词法与语法干净的文件一律停在类型检查边界；`we check .`（目录）一律停在项目编译边界；`we new` 生成的骨架（`pub type AppError = Failed(String)` + `pub fn main() -> Result<(), AppError> { return Ok(()) }`）无法变绿，尽管其全部表面都已批准。

**一项 M2 符合性修正随本变更携带（测试先行，详见 design D15）**：第 10 章已批准文本钉死嵌套泛型收口必须分隔书写——「maximal munch takes `>>` as the shift token (chapter 1), so the nested application is written `Box<Box<Int64> >`, and the unseparated `Box<Box<Int64>>` is rejected under chapter 2's `E0105`」（`docs/spec/1000-interfaces.md` Generic type references 一节）。M2 的 closeAngle 在收口位拆分 `>>`/`>=`（design D5，经 F2 修复），接受了 `Vec<Vec<Int64>>`——与已批准规范相反。M3 的类型检查恰好抵达泛型应用位，修正随本变更落地：收口位只认裸 `>`，`>>`/`>=` 于收口位报 E0105（消息含分隔书写补救），相关 M2 测试与黄金翻红翻转。

## 裁决记录（candidate 阶段四项表面裁决，2026-09-05）

- **Q1 第 9 章切割 → 声明全量 + 构造机制**：和式声明全量落地（pub/byval 前缀、载荷 0–8 之上 E0701、E0011/E0404、`|` 续行与行首 `|` 的 E0102；泛型子句与 derives 子句仍停第 10 章解析边界）+ 变体构造调用形 `Name(args)` 与裸单元变体（用户和式与预导入 Result/Option 同权）+ E0703（Never 仅函数返回注解位）+ E0704（带载变体裸用）。match/模式留在 M5。
- **Q2 第 7 章深度 → 全四协议位 + E0502 + 块值 + ch8 两小片**：字面量定型全量（默认 Int64/Float64、后缀映射、Bool/String/Rune 自定型）；E0501 四个点名协议位（运算符两侧 / 绑定注解 / 实参-形参 / 返回表达式对声明返回）并按「每一位置」机制读法覆盖赋值；E0502 常量可评估整型溢出（浮点 IEEE 不查）；块值定型；第 8 章两小片：`()` 单元类型与值、E0605 非单元丢弃。E0503 无 M3 触发面（条件位属 if/while/guard = M5+）→ 显式非目标。
- **Q3 模块模式 → 单模块项目 + 完整预导入登记**：项目模式落地（E1905 manifest 检查 + E1904/E2004/E1903 值校验、src/ 源根、根模块 src/main.we、E1305 main 形状）；单文件模式新语义（本地 import → E1302 无源根措辞、std import → std 未提供边界）；预导入 30 名完整登记（List/Map/Set/panic 等无 M3 类型的名按使用处出边界行，绝不误报 E1304）；多模块 import 解析与 E1301 → 诚实边界行，M6 拥有；E1303/E1304 的裸名与限定形解析落地（E1303 无 M3 触发面，声明式披露）。
- **Q4 成功输出 → 静默 exit 0**：干净检查无输出、exit 0；`--json` 零事件；`--verbose` 一行（`we: check passed (1 module)`）；边界行（exit 70）仅在抵达未实现表面时出现。

## 目标与非目标

### 目标

1. `internal/typecheck`：类型表示（13 基类型、`()`、Never、具名声明、内建和式 Result/Option、元组、fn 型）与结构相等；作用域链三层由内向外（块局部 → 模块单一名字空间 → 预导入外层域），实现 ch15 R4 与 ch8 遮蔽的操作内容（同域重绑定合法、后绑定胜、参数可遮蔽、模块层 E0404 独立）。
2. 字面量定型 context-free（无上下文再定型——ch7 场景钉死：无后缀字面量在不同注解下是 E0501，不是重定型）；E0501 四协议位 + 赋值位 + 构造实参位，位点锚点表钉死；运算符合法域表（数值域算术/位/移、同型比较等号出 Bool、Bool 逻辑——超出者按 roadmap 登记的 spec-gap 边界行处理，见目标 8）；E0502 常量折叠（折叠集、大整数求值后范围检、位点）；块值定型与函数体尾四分规则；E0605 丢弃位清单。
3. ch9 声明与构造：`type Name = V1 | … | Vn` 全量解析（前缀序 pub→byval、E0701/E0011/E0404、`=`/`|` 尾随续行、行首 `|` → E0102）；构造调用形与裸单元变体、E0704、载荷 E0501 逐实参；E0703 位点表（绑定注解/参数/变体载荷/元组元素/fn 型参数位）；期望类型线程（绑定注解、返回声明、实参-形参、构造载荷四处向下传）。
4. 内建 Result/Option：等价声明 `Result<T, E> = Ok(T) | Err(E)`、`Option<T> = Some(T) | None`；E1204（E 位非具名和式；`Result<_, Never>` 的归属论证见 design D8）；E0828（应用元数不符——含零子句具名声明的应用与 Result/Option 元数错配）；E0827（无期望类型时构造未定型，如裸 `let x = Ok(())` / `let x = None`）；`?` 传播维持解析边界（M6）。
5. ch15 名字解析：预导入 30 名封闭表登记与逐名处置；E1304 三形（裸名、限定形限定符非 import 名、类型位同规则）；import 的 M3 处置表（std → 边界行；单文件本地 → E1302 无源根措辞；项目本地缺失 → E1302 报期望路径；存在 → 多模块边界行）；E1303 无触发面的声明式披露。
6. ch21 项目模式：目录 → we.toml（极简 TOML 读取子集）→ E1905/E1904/E2004/E1903 触发表；源根 src/；根模块 src/main.we（缺文件读法 → E1305，design D11 论证）；tests/ 不编入；E1305 main 形状判据链（存在 → pub → 零参 → 返回 `Result<(), E>` 形；E 位非具名和式 → E1204；遮蔽 Result → E1305）；单文件模式语义；成功路径静默 exit 0（`--json` 零事件、`--verbose` 一行）。
7. M2 符合性修正 F3（测试先行）：closeAngle 收口位拆分删除，`>>`/`>=` 于收口位 → E0105 含分隔书写补救；M2 相关单测与黄金翻转；`Vec<Vec<Int64> >` 分隔形为合法面。
8. conformance：黄金用例新增与改写合计约 40 枚（每码至少一例、章节场景原例优先，含骨架端到端绿与 M2 面翻转改写——check-clean / check-clean-json 两枚因 `()` 转合法与 `AppError` 声明缺席需再次改写）；roadmap「Registered follow-ups」登记两条实现期发现的规范缺口（#4 运算符类型域表；#5 调用实参个数与非函数被调诊断码）——发现即登记，离开该表只能经各自的规范层变更。

### 非目标

- **match 与模式**（含 E0304、变体模式、语句位 match 臂的 E0605 位）→ M5；if/while/loop（含 E0503 条件位）→ M5。
- **`?` 传播（E1201/E1202/E1203）与 panic 家族签名** → M6：`?` 维持 M2 解析边界；panic/todo/assert 为预导入名，使用处出边界行，签名不落地。
- **多模块编译**（import 图遍历、E1301、跨模块 pub 检查 E1303、限定形实引用）→ M6：本切片 import 只做「解析失败可查 + 成功即边界」的诚实切分。
- **List/Map/Set 类型**（ch17）、**Dyn**（ch10）、**records/元组表达式/构造花括号**（ch8）→ 各自里程碑：名字已登记、使用处边界行或既有解析边界。
- **E0503**：无 M3 触发面（条件位全在 M5+ 形式内）。
- **泛型子句/derives**（ch10）、**闭包与 fn 值**（ch12：具名单参 fn 的裸名作值、fn 型等位检查的泛化）→ 后续里程碑；fn **类型注解**的解析与结构相等（含效果标签）在本切片内。
- **字符串插值孔内表达式的定型**：ch7 只批「定界字面量自定型为 String」，孔内表达式定型无条款——M3 不查不报，design D17 披露（无码可发，非边界）。
- **效果/所有权/文档检查级**：按「形式到达才出边界」原则（design D13），M3 可解析子集内无任何第 16/13/6 章待查形式抵达——不出边界行，骨架因此可绿；三级管线本身 M7/M5/M11 落地。
- **注册表 go:embed**：维持调用点硬编码（roadmap 待办 2）；**多错误批量报告**：维持首错误停止。

## What Changes

- 新增 `internal/typecheck/`（类型表示、作用域链、检查器 + 按章单测）。
- `internal/ast`：增和式声明节点（类型名/变体表/载荷为既有 TypeRef）与单元值表达式节点；`internal/parser`：和式声明解析、`()` 单元值（原 ch8 边界行删除）、`type` 关键字派发行翻写（原 ch7 边界行删除）、closeAngle 修正（F3）。
- `internal/cli`：runCheck 项目模式（manifest 校验 + 根模块 + main 形状）、单文件类型检查接线、成功路径三面（静默/`--json`/`--verbose`）、「type checking 未实现」尾边界删除。
- `internal/conformance`：新增与改写黄金用例（含项目模式端到端骨架绿）。
- `docs/roadmap/0000-reference-implementation.md` 与 `.zh.md`：登记 follow-ups #4/#5；M3 状态行由归档本变更的动作翻写为 done。

## 影响层

`compiler`（类型检查最小切片——已批准章节的实现，无规范增量）、`tooling`（`we check` 可观察表面：项目模式、成功输出、边界行——第 21 章既有行为的部分实现，无规范增量）。**本变更不改变语言行为、无规范增量**：一切接受/拒绝判定以已批准的 `docs/spec/0700-types.md`、`0800-composites.md`（两小片）、`0900-sum-types.md`、`1400-errors.md`（R1）、`1500-modules.md`、`2100-toolchain.md` 及注册表为依据；机制自由度（类型表示、作用域实现、位点锚点、运算符域表、TOML 读取子集、边界 What 措辞）在 design.md 固定并作为事实上的稳定表面记录；两条规范缺口以 roadmap follow-up 登记，不以本变更私定行为。

## 影响范围

- 新文件：`internal/typecheck/typecheck.go`、`internal/typecheck/typecheck_test.go`、约 40 个（新增与改写合计）`internal/conformance/testdata/cases/check-*.json`、若干项目模式 fixture 目录。
- 修改：`internal/ast/ast.go`、`internal/parser/parser.go`、`internal/parser/parser_test.go`（派发表行与 closeAngle 用例翻转）、`internal/cli/check.go`（项目模式与成功路径）、`internal/conformance/testdata/cases/`（既有用例中触及翻转面的少量改写）、`docs/roadmap/0000-reference-implementation.md` 与 `.zh.md`。
- 不动：`docs/spec/`、`diagnostics.toml`、`go.mod`、`internal/lex`、`internal/diag`、`internal/version`、M0/M1 黄金用例。

## 审计记录

**2026-09-05，candidate → ready，welang-spec-impact-audit 七条，通过。**

1. **问题真实性 ✓**：Why 的黑盒缺口可机械验证——现构建零输入能产生 E0501/E0502/E0605/E0701/E0703/E0704/E1204/E1302/E1304/E1305/E1903/E1904/E1905/E2004/E0011 任一码；`we check .` 恒为项目边界 70；`we new` 骨架不能绿。每章子弹头均引用 `docs/spec/` 具体文件与 Requirement 名。F3 修正引用 ch10 Generic type references and inference 原文整句。
2. **影响层声明 ✓**：change.yaml `layers: [compiler, tooling]` 与 proposal 影响层一致；纯内部实现 + 既有行为的部分实现，携带「本变更不改变语言行为、无规范增量」边界句，无 spec 层义务。
3. **规范增量范围 ✓**：零新增/修改/删除 Requirement 与诊断码；触达的 17 个码全部已存在于 `docs/spec/diagnostics.toml` 且标题核对一致（E0827/E0828 为 ch10 段既有码，经构造机制触达，同 M2 之 E0012/E0013 经 ch1 触达的先例）。两项实现期发现的规范缺口（运算符类型域、调用实参个数/非函数被调）以 roadmap follow-up 登记而非私定行为——符合「规范先行」的处理路径。
4. **原则一致性 ✓**：无隐式转换引入（字面量 context-free 定型强化之）；D13「形式到达才出边界」是 M2 既立原则的级维度推广，非突破；D4/D8 的机制读法（赋值位 E0501、E1204 对 `Result<_, Never>` 的归属）均在 design 论证并钉黄金，不构成对任何一条原则的偏离。
5. **参考基线固定 ✓**：本变更零 refr/ 引用；ch10 引文固定到文件与 Requirement 名；无外部项目引用。
6. **验收边界 ✓**：目标按码与端到端面（骨架绿、静默 0/--json 零事件/--verbose 一行、F3 翻转清单）机械判定；非目标显式排除 M5+ 全部形式、E0503、插值孔、三级管线、多模块编译，蔓延风险受控。
7. **粒度 ✓**：一个里程碑一个变更（roadmap 执行状态规则）；多章汇聚由骨架的跨章强制力解释（Why 第一段），单一切片可垂直验证（`we check .` 绿于骨架），无需拆分。

## 审查记录

**2026-09-05，ready → active，welang-change-review 十条，通过（两处发现已当场修正）。**

1. **proposal 职责 ✓**：Why 为黑盒缺口 + 章节引用；机制细节全部下放 design（proposal 仅按 M2 先例点名包名与文件面）。
2. **spec 增量 ✓（无增量变更的空判定）**：本变更零 spec delta；「本变更不改变语言行为、无规范增量」边界句在 影响层 一节，无 BCP 14 义务可违。
3. **design ✓（发现 1 已修正）**：路径唯一最小、被拒方案十七款有理由、引用精确到 Requirement 名与注册表标题。发现：黄金用例数 proposal「约 28」与 D16 逐项枚举（40）不一致——两处统一为「新增与改写合计约 40」。
4. **tasks ✓**：T1–T10 每项有 来源/验证；无 deferred、无未决方案。
5. **场景覆盖 ✓（发现 2 已修正）**：normal（骨架端到端绿、单文件真干净）/ boundary（遮蔽边例、E0827 对期望型线程、F3 分隔形）/ failure（17 码逐枚 + 边界行代表）齐备。发现：M2 的 check-clean / check-clean-json 两枚黄金在 M3 必然失效（其期望是 `()` 的 ch8 边界 70；`()` 转合法后行为改变，且其文件缺 `AppError` 声明将成 E1304）——D16 改写条目已补钉（改写为带和式声明的真干净文件，静默 0 / 零事件）。
6. **无空章节 ✓**：审查记录/实现审查记录为门占位（M2 先例），其余无凑格式内容。
7. **测试先行 ✓**：T2（conformance 红）与 T3（parser 翻转红 + typecheck 骨架红）先于一切实现任务；F3 翻转用例对现构建天然红。
8. **负向断言 ✓**：每个拒绝声明（17 码 + 12 边界 What + F3 两形）在 D16 有实样例黄金，非文字断言。
9. **完成度闭环 ✓（读法记录）**：原则 10 的三要素针对「语言特性类变更」；本变更零规范增量，是已批准行为在检查阶段的实现切片——类型检查即其本体，代码生成与运行时由 roadmap M4（native-vertical）按序落地，D13 记录管线级可观察面。与 M2（解析切片）过同一门的先例一致。
10. **未决问题阻塞 ✓**：四项表面裁决已答并烘焙；两项规范缺口的临时行为（边界行 70）是已定决策非未决问题，登记 follow-up 待规范层变更。

## 实现审查记录

**2026-09-05，active → complete，welang-code-review 七条，通过（真二进制证据，三项发现均已当场处置，见 3/4/6）。**

1. **规范符合性 ✓**：真二进制电池逐域复验——ch7 四协议位 E0501（注解/运算符/实参/返回）+ 赋值位、E0502 两形（`127i8 + 1i8` 于运算符 1:15、裸越界 1:9）；ch8 两小片（E0605 语句位/体尾位、`()` 合法）；ch9（E0703 参数位、E0704 带载裸用、E0011/E0404/E0701 经黄金）；ch14（E1204 于 main 形外与形内两锚）；ch15（E1302 单文件/项目两措辞、E1304 裸/限定/类型三形、E1305 五判据链、预导入边界行不误报 E1304）；ch21（E1905 缺文件/缺键、E1904/E2004/E1903 值形、成功三面、管线次序）。F3 现场复验：未分隔 `>>` → E0105 于 1:21，分隔形解析通过（抵 E1304 于 `Vec`——解析成功本身即证明）。骨架三面真机过：静默 0 / `--json` 零事件 0 / `--verbose` 一行。
2. **验证诚实性 ✓**：tasks.md T1–T8 逐项复核——本轮实际重跑：parser 10 套件、typecheck 9 套件、conformance 101 枚全绿、gofmt/build/vet/validate --strict/docs_sync/git diff --check 全过（T8 记录的输出与重跑一致）。T4 勾选与 tasks 勘误（T3 条目缺 T4 头）一并修正。
3. **测试先行证据 ✓（发现 1：测试侧勘误六款，已按协议「先红」后修正并披露于 tasks 完成记录）**：单测 5 处列号、`~true` 行归属（D4 锚表）、`1f32` 合法形、`Result<(), E2>>` 分隔、check-e0605-stmt 黄金源不相容、check-e0105-unseparated 黄金消息——全部为测试对表勘误非行为放宽，逐条理由在案。T2 红 52 枚、T3 红四套件+编译失败，计数在案。黄金面与 §52/§62 逐字一致（人形 `file:line:col: error[CODE]: message`，JSON 固定字段序）。
4. **诊断协议稳定 ✓（发现 2：E1304 消息形统一，已处置）**：实现期暴露 T2 两枚类型位黄金互不相容（check-e1304-type 要「type positions…」形，check-f3-separated 要裸形）——注册表三触发枚举 + 「type positions resolve by the same rules」为澄清句非第四触发，同规则即同消息：统一为裸/限定两形与位置无关，check-e1304-type 黄金更正、实现删 typeUnresolved。`--json` 字段序未动（M0–M2 既有 JSON 黄金全绿佐证）；触达 17 码全部既有注册表条目，零新增码。
5. **单一权威 ✓**：诊断权威在 `docs/spec/diagnostics.toml`（本变更触达既有条目，help 文本逐字取自注册表 remediation）；两项规范缺口以 follow-up 登记（#4/#5）非私定行为；代码注释全英文（M3 触达的 Go 文件零 CJK；lex/diag 测试文件中的 CJK 为 M0/M1 既有 Unicode 测试数据，非本变更 diff）；待提升内容无滞留（归档按 M2 先例纯过程工件）。
6. **红线复核 ✓（发现 3：`pub type` M2 符合性缺口，已按测试先行修复并披露）**：ch6 批准 `[pub] [byval] type` 而 M2 parseItem 的 `pub` 分支漏裸 `type` 行——6 枚黄金先红为证、补行后全绿（tasks 完成记录披露）。refr/ 在工作区但 .gitignore 命中（check-ignore 验证），不入提交；commit 信息将无署名 trailer；diff 范围 = 本变更目标文件（ast/cli/conformance/parser/typecheck + openspec 工件），无越界改动。
7. **最小可信验证已跑 ✓**：T8 阶梯全项本轮重跑通过；真二进制电池（约 40 次真实调用）覆盖三面 + 17 码 + 边界行。
