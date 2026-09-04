# Proposal: error-mechanism —— 错误机制（第 14 章）

## Why

语言至今以指针承诺错误机制而未定义它：

- `docs/spec/0000-principles.md:128`（P9）：错误 MUST 只经单一信令与传播机制；并行通道 MUST NOT 存在——"The concrete mechanism is defined by the error-handling chapter of this specification"。本变更即该章。
- `docs/spec/0900-sum-types.md:82`：`Never` 的产生形式"are the error-mechanism chapter's"——自 ch9 落地起悬置。
- `docs/spec/0700-types.md:67` 与 `:77`：运行时整数溢出陷阱"trapping construct's name and capture boundary are ratified by the error-mechanism chapter"——ch7 的指针。
- `docs/spec/0300-control-flow.md:81`：defer 体内错误传播规则"follow the error and effects chapters"——错误一侧待本变更落定（效果一侧仍留待效果章）。
- `docs/spec/1300-resources.md:184`：示例导语标注"Panic unwinding of a scope in flight is annotated as pending the error-mechanism chapter"；归档变更 resources-releasable 的裁决记录 4 与非目标承诺 panic 展开经 ch13 R2 修订绑定到同一释放保证。注：ch13 落地文的 R2 正文并无占位句——该承诺记录于归档裁决；本变更为 R2 增补展开句，正是兑现。
- `docs/spec/1000-interfaces.md:607`：`empty<T>` 示例注释"producers are the error-mechanism chapter's"——实现侧注记，随章落地刷新。
- `docs/spec/0800-composites.md:214`：丢弃规则（`E0605`，拼写 `let _ = expr`）已为 Result 的丢弃执法备好轨道。
- `docs/spec/0100-lexical.md:131`：`?` 已在第 1 章标点清单内——预留先于使用，本变更不触动 ch1。
- `refr/spec-0.8.md` §24–§26.1：v0.8 的 defer 禁 `?`、panic/todo/assert 形式、Result+`?` 规则、自动提升规则及 E0401–E0408——本变更逐条裁决其去留。

## What Changes

1. 新增第 14 章「错误机制」（`docs/spec/1400-errors.md`，双文）：`Result<T, E>` 为 stdlib 泛型 sum（`Ok(T) | Err(E)`），E 位 MUST 为命名 sum 类型（`E1204`），构造/匹配/穷尽性全部经 ch9/ch4 既有机制涌现；丢弃执法落 E0605（拼写 `let _ = expr`），绑定未用不强制。
2. `?` 传播运算符：后缀 `expr?`，操作数须为 `Result`（`E1201`）；仅活在最内层返回 `Result` 的函数/闭包上下文，defer 体与模块顶层禁用（`E1202`）；`E_src` MUST 等同 `E_dst`，不采纳自动提升（`E1203`）；`Ok` 解包为载荷，`Err` 即早返回——ch13 返回通道，资源义务随行。
3. panic 家族：`panic(msg: String) -> Never`、`todo(msg: String) -> Never`、`assert(cond: Bool, msg: String)` 皆为 stdlib 普通函数——无新关键字、无新产生式；不可捕获（catch/recover/try 无产生式，E0105）；运行时整数溢出陷阱命名为 panic（兑现 ch7 指针）；缺文本走普通调用检查，不设专码。
4. 展开与终止：panic 展开调用栈，每块按块出口处理——ch13 保证（逆声明序释放、内块先出、先于本函数 defer）扩展至展开出口；栈尽进程 abort，abort 即捕获边界；并发章（若批准）经 R4 修订绑定任务级边界，无第二通道。
5. 单一错误机制立场：失败声明在签名、`?` 唯一传播拼写、失败跨函数边界即签名中的值——P9 落地，局部可判定（P1）。
6. 六处宿主修订：ch2 表达式骨架（后缀枚举 += `expr?`）与运算符优先表（level 1 行）；ch3 Defer（错误一侧落定，`?` 禁入 defer 体）；ch7 整数溢出（陷阱构造命名 + 场景 THEN 更新）；ch9 底类型（Never 产生形式指针落地）；ch13 scope resource 语句（R2 增补展开句 + panic 穿透场景；另示例导语 D14 刷新）。
7. 诊断注册表：新段位 `E1200`–`E1299`（owner `1400-errors`），分配 E1201–E1204，未认领区间改为 `E1300`–`E9999`；docs_sync 20 → 21 对。

v0.8 代码映射（含偏离）：E0401（未处理 Result）→ 既有 E0605，缩窄为丢弃点强制——偏离 v0.8 的绑定未用即拒，披露于裁决 4；E0402（E 非具体 sum）→ E1204；E0403（panic 缺文本）→ 不继承专码，走普通调用检查——偏离披露；E0723（defer 内 `?`）→ 折叠进 E1202；E0404–E0408（自动提升族）→ 不采纳。

## 裁决记录

1. **panic 语义：stdlib 函数族 + 不可捕获终止**。`panic`/`todo`/`assert` 为标准库普通函数声明（第 6/7/10 章机制涌现），不引入关键字或产生式；任何形式都不可捕获——无 catch/recover/try，abort 即捕获边界。备选「关键字化 + 可捕获 unwind」被否：捕获即第二错误通道，直接违反 P9；Result 是唯一传播通道，panic 是终止。
2. **不采纳自动提升：E 严格一致**。`?` 要求 `E_src` 与 `E_dst` 为同一命名类型（`E1203`），无隐式包装或提升。v0.8 §26.1 的自动提升族（E0404–E0408）整体不采纳：隐式转换点是本语言仅有的豁免留给 `Never`（ch9 已记录），错误位再开豁免与 ch7 E0501 无隐式转换及 P5 显式性相抵；转换一律显式 `match`（组合子留待 stdlib 表面变更）。
3. **E 位 = 命名 sum 类型**。`Result` 的 E 参数 MUST 为命名 sum（`E1204`）——基类型（含 String）、记录、接口、Dyn 盒、泛型参数皆拒。约束是穷尽性的：命名 sum 变体集封闭，`Result` 的 match 可穷尽；泛型参数位无此封闭，且本规范不设 is-a-sum 约束。对 E 的泛型抽象助手（如 `fn run[E](...)`）因此不可表达——继承 v0.8 E0402 立场，design.md 披露权衡。
4. **`let _ =` 拼写 + 组合子延后**。Result 丢弃 = 既有 E0605 与 `let _ = expr` 拼写；绑定未用不强制（v0.8 E0401 缩窄为丢弃点执法，偏离披露）。`.ignore()` 不采纳——与 `let _ =` 冗余拼写。组合子（map/mapErr/andThen/orElse/unwrapOr/isOk/isErr）延后至 stdlib 表面变更——ch11 Iterator 先例：先语言机制，后库表面。

## 目标与非目标

目标：

- 落定 P9 的具体机制：Result + `?` + match，单一通道
- 兑现全部在册指针：ch9 Never 产生形式、ch7 溢出陷阱命名、ch3 defer 错误侧、ch13 展开绑定、ch10 实现侧注记
- panic/todo/assert 以最小表面（三个 stdlib 声明）入语言，零语法新增
- 诊断段位 E1200–E1299 开段 + 四码落位，一码一讯

非目标：

- 效果系统（v0.8 §27）——后续效果章；ch3 指针的效果一侧不受本变更触动。
- Result 组合子（map/andThen 等）——stdlib 表面变更另行立项。
- `.ignore()` 拼写——裁决 4 拒绝，非延后。
- catch/recover——按 P9 永久拒绝，非延后。
- 并发任务级捕获边界——并发章（若批准）经本章 R4 修订绑定；本章只定进程 abort 边界。
- panic 消息的格式约定（码表、前缀规范等）——stdlib 文档之职，非规范正文。

## 影响层

- spec

## 影响范围

- 新增 `docs/spec/1400-errors.md` 与 `docs/spec/1400-errors.zh.md`（6 Requirements / 27 Scenarios + 示例 + 术语对照）
- 修改 `docs/spec/0200-grammar.md` 与 `.zh.md`（Expression skeleton、Operator precedence and associativity 两处）
- 修改 `docs/spec/0300-control-flow.md` 与 `.zh.md`（Defer）
- 修改 `docs/spec/0700-types.md` 与 `.zh.md`（Integer overflow semantics）
- 修改 `docs/spec/0900-sum-types.md` 与 `.zh.md`（The bottom type）
- 修改 `docs/spec/1300-resources.md` 与 `.zh.md`（The scope resource statement + 示例导语刷新）
- 修改 `docs/spec/diagnostics.toml`（段位 `E1200`–`E1299`、E1201–E1204 条目、未认领区间改 `E1300`–`E9999`）
- docs_sync 校验对 20 → 21

## 审计记录

**2026-09-04 · welang-spec-impact-audit · 通过**

1. 问题真实性：通过。Why 全部锚点实核（ch0:128 P9 承诺句、ch9:82、ch7:67/77、ch3:81、ch13:184、ch10:607、ch8:214、ch1:131 均为落地文原句；refr/spec-0.8.md §24–§26.1 固定版本）——皆为在册悬置指针，非实现偏好。
2. 影响层声明：通过。change.yaml layers=[spec] 与 proposal 影响层一致；纯 spec 层变更，无 compiler/stdlib/tooling 项。
3. 规范增量范围：通过。ADDED 6（errors 章）+ MODIFIED 6（ch2×2、ch3、ch7、ch9、ch13）；新码 E1201–E1204 经 grep 全库唯一（docs/spec 零命中，唯一 active 变更即本变更）；段位 E1200–E1299 取自未认领区间 E1200–E9999 之首，无冲突。
4. 原则一致性：通过。P9 落地（单一通道、abort 唯一边界、并发经修订预留）；P1 保持（E_src≡E_dst 同型比较局部可判定，D3）；P5 保持（拒绝自动提升即拒绝新隐式转换点；唯一豁免 Never 系 ch9 已记录，非本变更引入）；E 位命名 sum 为收窄非突破；无原则偏离项，无 ADR 欠账。
5. 参考基线固定：通过。refr/spec-0.8.md §24（defer 禁 ?）、§25（panic 家族）、§26 与 §26.1（Result+? 与自动提升）逐节裁决并记录去留（含四项偏离披露：E0401 缩窄、E0403 不继承、E0404–E0408 不采纳、E0723 折叠）。
6. 验收边界：通过。目标可机械判定（ch14 双文 6R/25S 落地、六宿主修订×2、段位+4 码、docs_sync 21 对）；非目标六项排除蔓延（效果、组合子、.ignore()、catch 永拒、并发边界、消息格式）。
7. 粒度：通过。Result、?、panic 家族、展开互为前提不可拆（panic 需 Never、? 需 Result、展开需 ch13），六宿主修订皆指向本章的指针回填；组合子已按 ch11 先例拆出。

## 审查记录

**2026-09-04 · welang-change-review · 通过（发现 2 项，已处置）**

1–4 职责边界：通过。proposal 纯黑盒（指针+目标）；spec 增量纯行为（MUST 用法合规，无 SHOULD/MAY 欠账；R6 owner 命名系 ch13 段位条目先例）；design D1–D7 与增量无矛盾，被拒替代方案与理由齐备（关键字化、自动提升、任意 E 位、.ignore()）；tasks 三项均有来源/验证，无 deferred/未决。

5 场景覆盖：**F1**——短闭包 `?` 的推断规则（值类型定为 `Result<U, E_dst>`）在 R2 正文有、场景无；探针核对 ch12:88 后补场景 `A short closure's ? fixes its value type`（用注解参数 `|s: String| parse(s)?`——裸参闭包依赖期望函数类型属 E1001 地界，场景须避开；该场景同时演练最内层上下文在顶层绑定位的合法性）。**F2**——E1202 的三个子情形（非 Result 函数、defer 体、模块顶层）仅前二有场景，补 `Propagation at the module top level is rejected`。处置：errors 增量 25→27 场景（R2 8→10），tasks/proposal 计数同步；validate --strict 复跑通过。

6–8：通过。无空章节；task 1 机器比对即 spec 层测试先行；负向验证真实执行——E1299 负例注入宿主章实测 FAIL（消息 `diagnostic usage 'E1299:' has no registry entry` 已记录），另发现用量扫描只覆盖 docs/spec 不扫 delta（负例注入须落宿主章，tasks 验证措辞已按此执行）。

9 完成度：通过。类型检查（R1 E 位、R2 T/E_src/E_dst、Never 位置协议）与运行时（R4 展开序、abort 边界）齐备；codegen 于纯 spec 层不适用（fn-types/iterables/resources 先例）。

10 未决问题：通过。无阻塞项；组合子/catch/并发边界均为显式非目标或修订预留，非 TODO。

旁证核对（无缺陷）：E1202 标题对 defer 情形的覆盖系裁决 2 的 E0723 折叠；ch7 场景 THEN 与 R4 展开序一致（深层无资源代码中展开为空操作、直接 abort）；ch2 闭表立场与 `?` 经本章修订入表自洽；Some/None 先例在 ch9:53；ch6:234 注释不刷新（其 pending 属模块系统，非 Result）。

## 实现审查记录

**2026-09-04 · welang-code-review · 通过（无缺陷项）**

1. 规范符合性：ch14 EN 落地文与 delta 机器断言逐字一致（landed==delta True）；六宿主修订 hunk 逐条过目，变更全部落在目标 Requirement 块内，零外溢；zh 六块与 EN delta 语义镜像。
2. 验证诚实性：tasks 三项的每条验证命令均在本会话真实运行并留有输出——validate --strict/--all --strict、场景清点（27+30=55）、宿主场景集机器比对、一码一讯对齐、E1299 负例注入（FAIL 消息原文记录）、条目计数 91→95、docs_sync 20→21、zh 镜像计数/代码块字节一致/全角冒号扫描零命中、宿主 EN=ZH 场景计数（6/5/4/3/4/8）。
3. 测试先行证据：spec 层变更，无编译器可跑；机器比对与负例注入即本层测试（fn-types/iterables/resources 先例）。
4. 诊断协议稳定：未触及任何 --json 字段（无编译器产物）；E1201–E1204 全局唯一、四字段齐备、requirement 解析到 1400-errors.md 实标题（注册表 clean 证明）；负例证明注册门真实拦截。
5. 单一权威：长期事实即刻落 docs/spec（ch14 双文 + 六宿主 + 注册表）；变更目录只余过程记录；全部落地代码注释为英文。
6. 红线复核：git status 恰为影响范围清单（12 宿主文件 + diagnostics.toml + 2 新章 + 变更目录），refr/ 零触及；提交信息待「提交」时呈现，无署名 trailer。
7. 最小可信验证：validate --all --strict + registry clean + docs_sync 21 对齐，全部终绿。

途中发现并即修：examples 子节标题 H2→H3（docs_sync 标题结构检查拦截，ch13 先例）；注册表条目初漏 owner/requirement/allocated 三字段（validate FAIL 拦截后补齐）。

## 归档记录

**2026-09-04 · welang-archive-sync · 完成**

- 规范提升：ch14 双文（docs/spec/1400-errors.md + .zh.md，6R/27S）+ 六宿主修订×2（ch2×2/ch3/ch7/ch9/ch13）+ 注册表（段位 E1200–E1299、E1201–E1204、未认领改 E1300–E9999、91→95 条）已在 complete 前实时落地，复验逐字一致。
- 决策提升：无新 ADR——四裁决均无原则例外（审计第 4 点），取舍与被拒方案存于本 proposal 裁决记录与 design.md（fn-types/iterables/resources 先例）。
- 状态提升：无 roadmap 目录，不创建。
- 移动归档：openspec/changes/error-mechanism → openspec/changes/archive/2026-09-04-error-mechanism，status: archived。
- 复验：validate --all --strict + registry clean 终绿（下步执行）。
