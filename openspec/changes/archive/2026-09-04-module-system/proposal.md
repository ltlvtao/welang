# Proposal: module-system —— 模块系统（第 15 章）

## Why

语言至今以指针承诺模块系统而未定义它：

- `docs/spec/0600-declarations.md:5`（文件即模块）：路径如何解析到文件（目录映射、标准库优先、依赖缓存）"is ratified by the module-system chapter"——自 ch6 落地起悬置。
- `docs/spec/0600-declarations.md:24`（import 声明）：路径解析与跨模块可见性执法"are the module-system chapter's"。
- `docs/spec/0600-declarations.md:101`（顶层绑定）：跨模块初始化模型"is the module-system chapter's"。
- `docs/spec/0600-declarations.md:120-125`（pub 可见性）：非 pub 项跨模块使用的诊断与 main 函数约定"are the module-system chapter's"——连同场景 THEN 中的占位"rejected by the module-system chapter's diagnostic"。
- `docs/spec/0600-declarations.md:229-241`（Pending later chapters 示例块）：模块解析、跨模块可见性诊断、main 约定与 `import external.package.name` 均标注为本章之责——本变更兑现并刷新该块。
- `docs/spec/0700-types.md:86`（类型引用）：良构名字是否解析到已批准类型"is decided with module resolution by the module-system chapter"。
- `docs/spec/1000-interfaces.md:227`（Dyn 值）：`Dyn` 名字冲突"are the module-system chapter's business as theirs are"——预导入遮蔽规则落定。
- 既有机制就绪：ch6 单一名字空间（`E0404`）与 `name.item` 合格形式；ch8 记录/newtype、ch9 sum、ch10 接口与 impl 方法定义的 `pub` 前缀（ch10:150）；ch9:44 变体构造器经 `module.Name(args)` 合格到达；ch14 `Result`/`panic` 为预导入集合供给了名字。
- `refr/spec-0.8.md` §42（main 约定）、§45–46（可见性与集成）、§63（模块解析：std 编译器内建、点号目录映射、依赖缓存、循环依赖拒绝）——本变更逐条裁决其去留与重编号。

## What Changes

1. 新增第 15 章「模块」（`docs/spec/1500-modules.md`，双文）：模块路径按点号映射到文件——末段之外每段一个目录、末段一个 `.we` 文件，相对项目根的 `src/`；`std` 首段保留给编译器内建标准库（永不文件系统解析）；本地路径优先于依赖缓存，其余路径自缓存按同一映射解析；找不到目标以 `E1302` 拒绝（报文携带期望文件路径）；循环依赖以 `E1301` 拒绝——无前向声明，修复即抽第三模块。
2. 预导入（prelude）：每个模块免 import 可见的封闭名字集——ch7 基类型名、`Never`、`Dyn`、`Result`/`Ok`/`Err`、`Option`/`Some`/`None`、`panic`/`todo`/`assert`；本地同名声明遮蔽预导入名（预导入是外层作用域，显式胜隐式，P5）；`E0404` 只管模块自有名之间的重复；标准库其余部分仍是 `std.` 下的普通模块，须 import。
3. 跨模块可见性执法：pub 面 = fn 声明、顶层 let、记录、newtype、sum、接口、impl 方法定义（ch10:150）；跨模块到达只有经 import 名的 `name.item` 合格形式；非 pub 项的跨模块使用——值位、类型位、变体构造器——以 `E1303` 拒绝；接口成员不带 pub（ch10 既有）：经接口值（`Dyn` 盒或泛型约束）的方法调用合法，即使 impl 方法定义模块局部。
4. 名字解析：非限定名自内向外——块局部（ch8）→ 模块单一名字空间（ch6，声明与 import 名同域）→ 预导入；无域持有即 `E1304`；限定式 `name.item` 中非 import 名的限定符、被导入模块未声明的项同落 `E1304`——已声明而非 pub 者归 `E1303`（一类事实一个码）；类型位同规——落定 ch7:86 悬句与 ch10 Dyn 冲突注记。
5. 模块初始化：急切——每模块顶层 let 初始化器恰一次、进程启动时、`main` 之前；次序 = import 图后序（被导入者先完），后序遍历自根模块的 import 按源序展开，模块内源序（ch6 既有）；循环依赖到不了初始化（`E1301` 编译期拒绝），故总序确定（P1）；初始化期 panic 依 ch14 展开后中止——`main` 之前无捕获边界。
6. main 约定：入口模块 = 源根的 `main` 模块（`src/main.we`）；根模块 MUST 声明恰一个 `pub fn main() -> Result<(), E>`，`E` 为任意命名 sum（ch14 约束；v0.8 的 `Error` 只是 `E` 的一个合法拼写，非必拼）；缺失、非 pub、形状不符以 `E1305` 拒绝；非根模块中 `main` 为普通函数名；`Ok(())` 退出码 0，`Err(e)` stderr 报告后非零退出；`main` 无参数——命令行可达性归标准库。
7. 诊断注册表：新段位 `E1300`–`E1399`（owner `1500-modules`），分配 E1301–E1305，未认领区间改为 `E1400`–`E9999`；docs_sync 21 → 22 对。
8. 六处宿主修订（双文）：ch6 文件即模块 / import 声明 / 顶层绑定 / pub 可见性（指针句落地；pub 枚举句刷新为已批准各族；场景 THEN 占位换 `E1303:` 全码）；ch7 类型引用（末句落地）；ch10 Dyn 值（冲突句落地为预导入遮蔽）；另 ch6「Pending later chapters」示例块 D14 刷新。

v0.8 代码映射（含偏离）：E0110（循环依赖）→ E1301；E0112（找不到模块）→ E1302（报文含期望路径，继承 §63）；E0141（main 非 pub 或形状不符）→ E1305，且并拢「缺失 main / 非 pub / 形状不符」为一码，报文携带具体违规——偏离 v0.8 的逐情形分码，design.md 披露；跨模块可见性诊断与未解析名诊断在 v0.8 §63 层面无专码——E1303/E1304 为本规范新增。

## 裁决记录

1. **预导入 = 全预导入 + 本地遮蔽**。预导入集为封闭列举（基类型名、`Never`、`Dyn`、`Result`/`Ok`/`Err`、`Option`/`Some`/`None`、`panic`/`todo`/`assert`），每个模块免 import 可见；本地同名声明合法遮蔽（预导入 = 外层作用域，P5 显式胜隐式）。备选「无预导入，一切经 `import std.core`」被否：语言规范自身已依赖这些名字（ch7 基类型、ch14 `Result`/`panic`），让每章示例带 import 噪声与 AI 生成最短可核对片段的目标（P2）相抵；备选「Rust 式 `#[prelude]` 细粒度控制」被否：v0.8 无此概念，引入即增加一层间接。集封闭：增删名字 = 本章程的 spec 变更，不是标准库发版。
2. **main 约定 = `src/main.we` + `E` 任意命名 sum**。入口模块固定为源根的 `main`；签名 `pub fn main() -> Result<(), E>`，`E` 为任意命名 sum——v0.8 §42 的 `Error` 是占位名，ch14 已把错误位约束为命名 sum，规范不应再钦点一个拼写。备选「固定 `Error` 名 + 标准库预声明」被否：多一层隐式依赖，且与「预导入名可遮蔽」相缠；备选「多入口 / main 参数承载 CLI」被否：v0.8 无此概念，CLI 可达性归标准库（`std.io`），本规范只定无参 main。
3. **初始化 = 急切 + DAG 后序，恰一次**。进程启动、`main` 之前，每模块顶层初始化器按 import 图后序跑一遍——被导入者先完，模块内源序。备选「惰性首次使用初始化」被否：初始化时机依赖首个使用点，隐式且难以局部判定（P1）；备选「显式 `init()` 协议」被否：v0.8 无此概念，徒增仪式。初始化期 panic 依 ch14 展开后中止——`main` 之前无捕获边界，与「abort 即捕获边界」一致。
4. **依赖范围 = 仅批准路径映射**。外部依赖模块自依赖缓存按与本地一致的点号映射解析；版本约束、锁文件、安装与发布是工具链层的业务，延后为独立工具链变更——v0.8 §65 本身即延后。备选「本变更一并定义版本语义」被否：超出 spec 层切片边界，且 v0.8 未决；备选「禁止外部依赖（单项目封闭）」被否：ch6 落地的 `import external.package.name` 示例已承认外部路径，收窄即回退已批准事实。

## 目标与非目标

目标：

- 落定 ch6/ch7/ch10 全部模块系统指针：路径→文件映射、跨模块可见性诊断、跨模块初始化模型、类型名解析、`Dyn` 冲突规则、main 约定
- 段位 E1300–E1399 归属 modules 章，E1301–E1305 五码入注册表
- 初始化次序确定可推理（P1），显式遮蔽规则（P5），一类事实一个码（E1303/E1304 分立）

非目标：

- 依赖版本选择、锁文件、安装与发布（工具链层变更；v0.8 §65 延后继承）
- 项目清单（manifest）格式——项目根概念保留，其文件形式是工具链的
- CLI/LSP 行为（v0.8 §64 工具链范围）；文档生成警告 W0901
- 选择性导入、通配导入、一名多导（ch6 已否，不重开）
- `as` 之外的模块别名、re-export、模块作为一等值
- 标准库 `std.*` 各模块的表面（stdlib 表面变更另行）
- 编译产物与链接模型（后端业务）

## 影响层：spec

仅 spec 层。无代码、无工具变更。

## 影响范围

新增：

- `docs/spec/1500-modules.md` + `docs/spec/1500-modules.zh.md`（第 15 章，7 Requirements / 29 Scenarios + 示例 + 术语对照）
- `openspec/changes/module-system/specs/modules/spec.md`（7 ADDED / 29 Scenarios）
- `openspec/changes/module-system/specs/modules/examples.md`

修订（双文）：

- `docs/spec/0600-declarations.md` + `.zh.md`：文件即模块（末句）、import 声明（指针句）、顶层绑定（指针句）、pub 可见性（枚举句刷新 + 指针句 + 场景 THEN）——4 Requirements；另「Pending later chapters」示例块 D14 刷新
- `docs/spec/0700-types.md` + `.zh.md`：类型引用末句（1 Requirement）
- `docs/spec/1000-interfaces.md` + `.zh.md`：Dyn 值冲突句（1 Requirement）

注册表：

- `docs/spec/diagnostics.toml`：`[segments."E1300-E1399"]` domain=modules owner=1500-modules；未认领区间 `E1300-E9999` → `E1400-E9999`；新增 `[diagnostic.E1301]`–`[diagnostic.E1305]` 五条（severity/title/description/remediation/owner/requirement/allocated）；注册表 95 → 100 条。9900 注册表章只定机制不列举条目，无需修订。

docs_sync：21 → 22 对（1500-modules.md / .zh.md 入对）。

## 审计记录

2026-09-04 —— 通过（7/7）：

1. 问题真实性 ✓：Why 引用 7 处已批准指针（ch6:5/:24/:101/:120-125/:229-241、ch7:86、ch10:227）与 v0.8 §42/§45–46/§63——全部是落地变更写下的悬置承诺，可逐条核对。
2. 影响层声明 ✓：change.yaml `layers: [spec]` 与 proposal 影响层一致；不触及 compiler/stdlib/tooling 行为。
3. 规范增量范围 ✓：7 ADDED（26 Scenarios）+ 6 MODIFIED（22 场景，机器比对：五组逐字继承，Visibility with pub 差异恰为披露三处——枚举句刷新、THEN 占位换 E1303 全码、场景标题去「two」）；E1301–E1305 对照注册表 95 条与全部 active 变更零冲突；段位 E1300–E1399 现属未认领区间 E1300–E9999，认领合法。负例注入实测：E1399 注入 docs/spec/0600-declarations.md → `FAIL: diagnostic usage 'E1399:' has no registry entry in docs/spec/diagnostics.toml`，还原后 clean。
4. 原则一致性 ✓：P1（初始化后序总确定、解析三级优先）、P2（预导入免 import 噪声）、P5（显式遮蔽胜隐式预导入）正面援引；无原则突破，无 ADR 需求。
5. 参考基线固定 ✓：refr/spec-0.8.md §42/§45–46/§63/§65 固定；偏离逐条披露（E0141→E1305 并拢、v0.8 沉默处补全解析优先级）。
6. 验收边界 ✓：目标可机械判定（指针落地、段位认领、五码入册、docs_sync 21→22）；非目标排除依赖版本化/清单格式/CLI/导入形式重开/别名与 re-export。
7. 粒度 ✓：单一垂直切片——一章 + 六宿主修订 + 注册表，无跨层。

validate --strict 通过（注入前后各一次，还原后 OK）；单码单消息扫描 9/9 通过。

## 审查记录

2026-09-04 —— 通过（10/10，发现 F1–F3 三处场景覆盖缺口，补齐后放行）：

1. proposal ✓：只讲黑盒缺口与目标；裁决记录与实现权衡归位（裁决在 proposal、路径在 design，与归档切片同构）。
2. spec 增量 ✓：全为黑盒行为（触发/结果/失败路径）；MUST 6 处使用合规，无 SHOULD/MAY。
3. design ✓：D1–D8 唯一路径 + 被拒替代与理由；引用精确到条目（ch10:150/§63/§42）；与增量无矛盾。
4. tasks ✓：来源/验证齐备；无 deferred/未决（过渡期 FAIL 为已知瞬态披露，errors 片先例）。
5. 场景覆盖 —— 补三场景（26→29）：
   - **F1**（R3）：正文断言变体构造器位执法（"or as a variant constructor"）但无场景钉住 → 新增「A module-local variant constructor is rejected」（`b.NotFound("x")` → E1303）。
   - **F2**（R4）：正文断言预导入类型名可遮蔽（`Dyn` included）但无场景钉住 → 新增「A shadowed prelude type name follows the local meaning」（本地 `type Result = Win | Lose` 后裸 `Result` 指本地 sum，无诊断）。
   - **F3**（R6）：`E1305`（main 形状）与 `E1204`（错误位命名 sum）的边界未钉住 → 新增「A non-sum error position in main is chapter 14's code」（`Result<(), String>` → E1204，形状合规则 E1305 不触发）——一类事实一个码的边界场景。
   正常/边界/失败路径余项全覆盖（缺缓存、循环、双重导入恰一次、init 期 panic、非根 main、Err 退出码）。
6. 无空章节 ✓。
7. 测试先行 ✓：spec 层切片，任务 1 即规范增量构造（errors/resources 先例）。
8. 负向断言 ✓：任务 2 负例注入 E1399（审计期已实测 FAIL 并还原，任务 2 执行时复做）。
9. 完成度闭环 ✓：解析/可见性/名字解析 = 编译期（D4/D5），初始化/退出码/panic 中止 = 运行时（D1/D7）；编译产物与链接为显式非目标。
10. 未决问题阻塞 ✓：无未决项。

计数同步：proposal 影响范围与 tasks 任务 1/3 已随 26→29 更新；validate --strict 复跑绿。

## 实现审查记录

2026-09-04 —— 通过（7/7）：

1. 规范符合性 ✓：落地与 delta 逐字节一致（章 7R、宿主 6R 机器断言 True）；诊断消息串与注册表标题逐字对齐。
2. 验证诚实性 ✓：三任务验证命令全部真实运行、输出留档（注入 FAIL 消息原文两见）；无"勾了没跑"。
3. 测试先行证据 ✓：spec 层切片，负例注入即负向测试（先证 FAIL 再扩展注册表）。
4. 诊断协议稳定 ✓：无既有条目改动；E1301–E1305 全局唯一且已登记；requirement 字段全部解析到 1500-modules.md 实际标题。
5. 单一权威 ✓：章文与宿主修订全部进 docs/spec/；变更目录只余变更工件（归档即历史）；docs 英文无中文混入（zh 孪生为配对译文）。
6. 红线复核 ✓：refr/ 未动未提交；diff 范围 = docs/spec 十文件 + openspec/changes/module-system；无越界改动。
7. 最小可信验证 ✓：validate --all --strict、docs_sync --check（22 对）、计数/字节/全角/单码单消息机器断言、负例注入两轮。

## 归档记录

2026-09-04 —— 归档（baseline promotion）：

- 规范提升：第 15 章 `1500-modules.md` + `.zh.md` 已落 docs/spec/（7R/29S）；六宿主修订（ch6×4、ch7、ch10 ×2 langs）与 ch6 Pending 块 D14 刷新已落地；全部指针句兑现，EN 全文无残留「module-system chapter」待批字样（grep 复核）。
- 决策提升：D1–D8 权衡留归档本档（沿 errors 片先例，不产切片 ADR；ADR-0001 仍为唯一 ADR）。
- 状态提升：无 docs/roadmap/，按约定在变更层留档。
- 移动归档：openspec/changes/module-system → openspec/changes/archive/2026-09-04-module-system；status=archived。
- 复验：validate --all --strict + docs_sync --check（22 对）终绿。
