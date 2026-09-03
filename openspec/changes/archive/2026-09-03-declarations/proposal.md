# Proposal: declarations

## Why

1. 语言至今只有块内世界：ch2 批准了块/语句/表达式，ch3/ch4/ch5 批准了控制流、match、for——但**文件顶层结构从未批准**。ch1 只说"源文件是 UTF-8 文本文件"；顶层放什么、一个文件是什么，规范沉默。任何实际程序都需要至少一个 fn。
2. 三个章节的前向引用悬空等待本章：ch1 文档注释"attachment rules are defined by the declarations chapter"；ch1 属性"attached to the next declaration"（"declaration"一词至今无已批准含义）；ch3 return "where return is legal (function bodies) ... is ratified by the declarations chapter"；ch3 defer "function bodies are ratified by the declarations chapter; until then the rejectable side is what fires"。本章落地即全部兑现——**零宿主修订**（首次纯 ADDITIVE 内容章节）。
3. v0.8 素材固定：§45 `import path [as alias]` + `pub` 可见性；§46 顶层仅 `let` 无 `var`；§63 模块=文件、点分路径映射目录；§13.4 参数 `name: Type` 全注解。§13.4-13.5 函数类型/effect 标注/Lambda、§47 foreign 块、main 约定（§45 例）依赖类型/效应/FFI/错误机制——延后并命名路径。

## 目标与非目标

目标：

1. 建第 6 章 `docs/spec/0600-declarations.md`（+zh 孪生）：文件结构与模块同一性、import 声明、fn 声明、函数体语境（return/defer 正面向）、顶层绑定、pub 可见性、模块名空间、文档注释附着。
2. 注册表：认领段位 E0400–E0499（owner 0600-declarations），分配 E0401–E0405 五码；未认领段收窄为 E0500–E9999。
3. 兑现全部四处前向引用（ch1 文档注释/属性、ch3 return 语境/defer 函数体），零宿主章节修订。

非目标：

1. 模块解析算法（std 优先、src 根映射、依赖缓存、循环依赖拒绝）→ 模块系统章节（未来新章，非本变更创建）。
2. 跨模块可见性执法诊断（引用他模块非 pub 名字的拒绝码）与 main 函数约定（`pub fn main() -> Result<(), E>`）→ 模块系统/错误机制章节。
3. 类型层：参数/返回类型的文法、fn 类型化、调用检查、函数值/闭包/Lambda（`|x|` 短语法）、函数类型作类型 → 类型章节成对修订。
4. 泛型参数（`fn f<T>`）、effect 标注（`fn(Int64) io -> Int64`）→ 泛型/效应章节。
5. `foreign` 声明块、`mut` 参数/方法接收者（`impl`）→ FFI/类型章节。
6. 属性白名单仍为空：声明上任何 `#[...]` 按 ch1 E0008 拒绝，本章不新增属性、不改动 ch1。
7. 顶层名字与块内局部名的遮蔽规则 → 名字解析/类型章节（本章只定模块内顶层唯一性）。
8. 跨模块初始化顺序与模块初始化模型 → 模块系统章节（本章只定模块内源序）。

## What Changes

1. `specs/declarations/spec.md`：8 条 ADDED Requirements——File structure and module identity / Import declarations / Function declarations / Function bodies, return and defer / Top-level bindings / Visibility with pub / Module name space / Documentation comments。
2. `specs/declarations/examples.md`：示例节（非权威，提升时逐字并入第 6 章）。
3. `docs/spec/diagnostics.toml`：段位表新增 `E0400-E0499`（owner 0600-declarations），未认领段 `E0400-E9999` 收窄为 `E0500-E9999`；新增 5 条目 E0401–E0405。
4. 无宿主章节修订：ch1/ch2/ch3 既有文本的前向引用在本章落地后自然消解。

## 影响层

- spec：仅规范层（一个规范增量 + 注册表段位认领与 5 条目）。

## 影响范围

- 新章 `docs/spec/0600-declarations.md`（+zh）；`docs/spec/diagnostics.toml`。零既有章节文件改动。

## 裁决记录

2026-09-03 用户裁决四项（均取推荐）：

1. **无返回值函数的体末项表达式：隐式丢弃，值级检查延后至类型层**——形式层不设卡（`fn log(m: String) { emit(m) }` 直接合法）；"丢弃了非 unit 实值"需要 unit 类型才可判定，随类型章节成对修订。与 match 穷尽性延后同一哲学。
2. **顶层 var 拒绝**——继承 v0.8 §46：全局可变状态不存在（模块级可变状态是并发与推理的双重陷阱）；`var` 仅块内合法（E0403）。
3. **import 形式集：仅 `import path` 与 `import path as 别名`**——两形（P2）；模块名以最后一段（或别名）进入本模块名空间，成员经 `.` 访问；选择性导入（`import a.{b, c}`）与通配不批准。
4. **注解严格度：参数类型强制；返回类型可省（省 = 无返回值）**——参数无推断（P1 局部可判定）；返回类型省略即声明不产出值；无返回值函数里 `return expr` 形式层立即拒绝（E0402）。

## 审计记录

2026-09-03 审计（candidate → ready 关卡，7 点）：

1. **问题真实性：通过。** Why 三条均可核验：`fn let var pub import as mut` 在第 1 章 21 词清单（当日读取 49–53 行）；文件顶层结构全仓未批准——`top level|top-level` 在全部 EN 章节零命中（当日 grep）；四处前向引用原文在档（ch1:126 文档注释、ch1:140 属性 "attached to the next declaration"、ch3:62 return 语境、ch3:76 defer 函数体，当日读取）；v0.8 素材固定 §45/§46/§63/§13.2-13.4（当日读取 1735–1775、2355–2375、588–633 行）。
2. **影响层声明：通过。** layers `[spec]`；一个规范增量 + 注册表段位认领与 5 条目，均 spec 层。
3. **规范增量范围：通过。** 8 条 ADDED 单能力（declarations）+ examples.md + 注册表扩展；零 MODIFIED——所需关键字（fn/pub/import/as）与 `->` 均在 ch1 既有清单（当日核对），函数体用 ch2 块（引用非修改），ch3 前向引用自然消解（D9）；块内 `fn`/`import`/`pub` 走 ch2 E0105（关键字可起始语句、产生式不适配——与 ch2 语句起始类不冲突，当日核对 ch2 行 19）；E0401–E0405 在 candidate 阶段未注册属既定允许（使用扫描只覆盖 docs/spec/）。
4. **原则一致性：通过。** P1：参数注解强制（无推断）、import 恰引入一个名字、文档附着位置可判定；P2：import 两形、单一返回机制（块值 + 显式 return，D1）；P4：defer 体内 return 在歧义诞生前封死（D6）、模块内一个名字空间；无原则例外，无需 ADR。
5. **参考基线固定：通过。** 素材固定 §45（import/pub）、§46（顶层仅 let）、§63（模块=文件、路径映射——映射细节判归模块系统章节并披露）、§13.2-13.4（参数全注解）。偏离逐项披露：main 约定延后、解析算法延后、v0.8 码 E0141/E0110/E0150 系不继承（E04xx 全新分配）、§13.5 Lambda 不继承（随类型章节）。
6. **验收边界：通过。** 目标机械可判：第 6 章存在含 8 条 Requirement/23 Scenario、注册表段位 + 5 条目后 registry clean、宿主零改动（git status 可核）、zh 孪生 + docs_sync。非目标八项明示。
7. **粒度：通过。** 单能力（declarations）+ 注册表扩展；与类型/模块系统/FFI/效应显式解耦（非目标 1–5、7、8）。纯 ADDITIVE 首例（D9）不改任何宿主章节，粒度自然最小。

**结论：通过，进入 ready。**

## 审查记录

2026-09-03 语义审查（ready → active 关卡，10 点）：

**职责边界**：(1)–(4) 全部通过——proposal 纯黑盒；增量全为可观察行为（项结构、名字引入、拒绝路径、求值顺序），MUST/MUST NOT 用于 BCP 14；design D1–D11 与增量一致（D5 裸参数→E0105 不设新码、D6 E0401 双位置、D9 零宿主修订均与增量条文对应）；tasks 三项有来源/验证，无未决选择。

**内容质量**：(5) 覆盖 normal（项结构、import 两形、fn 签名、隐式返回、文档附着）、boundary（别名 vs 末段、嵌套块内提前 return、defer 正面向、空行不断链、源序求值）、failure（E0105 三类位置、E0013、E0401–E0405）✓；(6) 8 条 Requirement 均有行为增量 ✓；(7) spec 层测试先行——断言先于章文件，E0501 哨兵注入在注册表维护之前（task 2）✓；(8) 负向断言真实（注入 + 增量内七类拒绝场景）✓；(9) 完成度闭环按 spec 层先例：类型化/解析/初始化均以命名路径前向引用（类型/模块系统章节，非目标 1–3、8）✓；(10) 四裁决已闭环，无未决问题 ✓。

**发现与处置（F1）**：增量初稿全文零 BCP 14 关键字——此前各章在承重义务处均用 MUST/MUST NOT，纯陈述句风格断档。修复：8 条 Requirement 承重处插入 14 个 MUST/MUST NOT（拒绝义务、注解强制、名字唯一性、顶层 var 禁令等），并顺手改掉一处别扭措辞（"importing into any name other than one"→"importing under more than one name"）。修复后 validate strict 通过。

**边界说明（记录，无工件改动）**：(a) **E0012/E0013 复用面**——ch1 命名约定按"绑定类别"表述，fn 名与 import 别名的绑定类别在本章获得首个声明位置，E0012/E0013 条目文本已通用覆盖，无需改注册表。(b) **块内 `fn`/`pub`/`import` 走 E0105**——关键字属 ch2 语句起始固定类，失败在产生式适配层，与 ch2 无冲突。(c) **增量共 23 个 Scenario**（3+3+3+5+3+2+2+2），实现期以此数为准。

**结论：通过，进入 active。**

## 实现审查记录

2026-09-03 实现审查（active → complete 关卡，7 点）：

1. **规范符合性：通过。** 提升后机器逐字核对 8/8：全部 Requirement（场景集 3+3+3+5+3+2+2+2，共 23）与增量逐字一致（含 F1 修复后的 MUST 措辞）；`## Examples (non-authoritative)` 2643 字符与 examples.md 逐字一致；zh 孪生结构全镜像（H3 req 8=8、H4 scen 23=23、示例小节 12=12、fence 8=8）。
2. **验证诚实性：通过。** tasks.md 三项验证行与实测一致：validate strict 两级（change/全仓）均 OK；E0501 负例注入实跑（exit=1，消息原文 "diagnostic usage 'E0501:' has no registry entry in docs/spec/diagnostics.toml"，还原后 clean）；docs_sync 12→13 对全对齐；逐字/镜像核对为本审查实跑。条目先于章文件的 5 条瞬态 FAIL 如实记录于 task 2 验证行（match 先例）。
3. **测试先行证据：通过（spec 层对应物）。** layers [spec]，无编译器代码；注入先行（E0501 哨兵先于注册表扩展，消息原文已录）；conformance §52/§62 不适用。
4. **诊断协议稳定：通过。** 注册表 23→28 条：段位表新增 E0400–E0499（owner 0600-declarations），未认领段收窄为 E0500–E9999，5 个新码全局唯一；既有 23 条零改动（git diff 仅 +55/−1 全为新增段与条目）；每条目 requirement 字段机械解析到本章 Requirement 标题（5/5 OK）。
5. **单一权威：通过。** 长期事实落位：新章 EN+zh（含术语表 14 对）、注册表段位与条目；变更目录仅持增量与过程记录；EN 章/注册表英文，zh 为配对翻译（诊断消息串保留英文原文，ASCII 冒号扫描 0 命中）。
6. **红线复核：通过。** 工作树范围恰为 docs/spec/ 下注册表 + 两个新章文件 + 变更目录；宿主章节零改动（git status 机械复核，兑现 D9）；无 refr/ 内容进入；commit 信息待用户明示后生成（无署名 trailer）；diff 与非目标八项无冲突（模块解析/可见性执法/main/类型化/泛型/effect/foreign/遮蔽均未引入）。
7. **最小可信验证已跑：通过。** validate.py --all --strict（OK）+ docs_sync.py（13 对 OK）+ 机器逐字 8/8 + 示例节 diff 为零 + zh 镜像计数全等 + 全角冒号扫描 0 命中 + E04xx 条目解析 5/5。

**发现与处理**：无新发现（F1 已在审查关卡修复并记录；实现期零返工）。

**结论：通过，进入 complete，转归档。**

## 归档记录

2026-09-03 归档（complete → archived）：

1. **规范提升已完成**：`docs/spec/0600-declarations.md`（+zh 孪生）创建；零宿主章节修订（D9：ch1 文档注释、ch1 属性 "next declaration"、ch3 return 语境、ch3 defer 函数体四处前向引用落地即消解）；`docs/spec/diagnostics.toml` 段位 E0400–E0499 认领 + E0401–E0405 五条目 + 未认领段收窄为 E0500–E9999。
2. **决策提升：无 ADR。** design D1–D11 均为常规取舍（体值对称、隐式丢弃、顶层 var 禁令、import 两形、注解严格度、E0401 双位置、名字空间、文档附着、零宿主、源序初始化、段位注脚），无原则例外与信任边界决策；design.md 随归档保留。
3. **状态提升：无 roadmap**（沿前例，openspec 层留档即止）。
4. **移动归档**：`git mv` 至 `openspec/changes/archive/2026-09-03-declarations`，status → archived。
5. **复验通过**：validate.py --all --strict OK（无活跃变更，registry clean）；docs_sync 13 对 OK。
