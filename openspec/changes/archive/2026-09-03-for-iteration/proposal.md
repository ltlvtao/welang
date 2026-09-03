# Proposal: for-iteration

## Why

1. `for` 与 `in` 在第 1 章 21 词关键字清单中保留至今且无产生式——任何使用落第 2 章 E0105。第 3 章 Break and continue 及 E0201 条目已显式预写"the iteration chapter ratifies for / adds for"，本章是既定归属的兑现。
2. 区间 `..` 不在第 1 章封闭算符清单内——这是首个**成对 lexical+grammar 修订**：词法层入 token、语法层入优先级表/续行集/语句族枚举，四处在同一变更内一致扩展（注册表与命名规则预期的机制首次启用）。v0.8 §21.2 固定右排他语义与 `range(start, end)` 等价性（等价展开判为实现细节，排除）。
3. 迭代器协议（§21.2.1 Iterable/Iterator）与组合子（§21.2.2）依赖接口/泛型/闭包/Option/List——均未批准；延后并命名进入路径。

## 目标与非目标

目标：

1. 建第 5 章 `docs/spec/0500-iteration.md`（+zh 孪生）：for 语句（求值语义、逐元素绑定、break/continue 归属）与区间表达式（右排他、数值步进、空区间）。
2. 成对修订：第 1 章算符清单 + `..`（含 maximal-munch 钉死）；第 2 章行接续续行集 +1 token、语句族枚举 + for、优先级表新增第 12 档 `..`（none）。
3. 注册表：E0104 描述/修复扩展覆盖区间链（枚举扩展，非重定义语义）。

非目标：

1. Iterable/Iterator 协议、迭代器自动满足 Iterable、可迭代性检查（v0.8 E0252 对应物）→ 类型章节成对修订。
2. 组合子 map/filter/take/skip/collect/...（需闭包/Option/List）→ 后续修订；纯度规则（v0.8 E0504 对应物）同期。
3. 含端 `..=` 与开放端区间（`..end`/`start..`）不批准（裁决 1）；未来经 spec change 进入。
4. 循环变量类型化、每次迭代的绑定别名/可变性语义 → 类型章节。
5. 所有权类别交互（gc 快照/value 副本/resource 不实现 Iterable）→ 所有权/resource 章节。
6. match 变体/元组模式（for 解构依赖它）→ 维持 match 章节既定的成对修订路径。

## What Changes

1. `specs/iteration/spec.md`：2 条 ADDED Requirements——For statement、Range expression。
2. `specs/lexical/spec.md`：1 条 MODIFIED（Operators and punctuation：算符清单 + `..`，munch 钉死句，+1 Scenario）。
3. `specs/grammar/spec.md`：3 条 MODIFIED（Line-joining and semicolon inference：续行集枚举 + `..`；Statements：语句族枚举 + iteration 章/for；Operator precedence and associativity：第 12 档 `..`/none + 非结合句扩展 +1 Scenario）。
4. `docs/spec/diagnostics.toml`：E0104 description/remediation 枚举扩展（比较集 → 比较集 + 区间链例）。

## 影响层

- spec：仅规范层（四个规范增量 + 注册表条目描述扩展）。不改编译器/工具行为。

## 影响范围

- 新章 `docs/spec/0500-iteration.md`（+zh）；`docs/spec/0100-lexical.md`（+zh）一条 MODIFIED；`docs/spec/0200-grammar.md`（+zh）三条 MODIFIED；`docs/spec/diagnostics.toml`。

## 裁决记录

2026-09-03 用户裁决一项（取推荐）：

1. **区间形式集：仅 `..` 右排他**——单形式（P2）；`..=` 不入词法清单，含端需求写 `start..end+1` 或未来经 spec change 加形式；开放端区间（`..end`/`start..`）同样不批准，待集合/切片章节需要时再议。v0.8 同构。

## 审计记录

2026-09-03 审计（candidate → ready 关卡，7 点）：

1. **问题真实性：通过。** Why 三条均可核验：`for`/`in` 在第 1 章 21 词清单且全仓无产生式（当日读取清单 47–54 行）；第 3 章 Break and continue 原文 "the iteration chapter ratifies for" 与 E0201 条目 "(while or loop; the iteration chapter adds for)" 均已预写归属（当日读取）；`..` 不在第 1 章算符清单（当日读取 105–120 行）；v0.8 素材固定 `refr/spec-0.8.md` §21.2/§21.2.1/§21.2.2（818–911 行，当日读取）。
2. **影响层声明：通过。** layers `[spec]`；四个规范增量 + 注册表描述扩展均 spec 层。
3. **规范增量范围：通过（含一起草错误被核对机制捕获并修正）。** 2 ADDED + 4 MODIFIED（lexical 1、grammar 3）逐条列于 What Changes；无新码分配，E0104 描述扩展无冲突；四处 MODIFIED 旧文与现行章节 diff 机械核对——差异恰为预定插入（清单 +`..`、续行集 +`..`、语句族 +for 句、优先级 +第 12 档行 +非结合句扩展），零漂移。起草期错误：优先级 Requirement 的 Scenario 集凭记忆误写（混入表达式骨架的 "Postfix chains"），对照现行章节核对时捕获，已修正为原四 Scenario 逐字 + 新增 "Chained range is rejected"——verbatim 核对是必要关卡而非形式。
4. **原则一致性：通过。** P2：区间单形式（D1）；P1：四个闭集（算符清单、续行集、优先级表、语句族）同一变更内一致扩展，各自明文"扩展即 spec change"路径被遵守；P4：`..` 独立非结合档消除混档歧义（D2），munch 切分钉死（D4）——无原则例外，无需 ADR。
5. **参考基线固定：通过。** 素材固定 §21.2 全节；偏离逐项披露：`range(start, end)` 等价展开判为实现细节排除（D7 改以右排他/步进/空区间陈述语义）、协议与组合子延后并命名路径（D5）、v0.8 码 E0252/E0504/E0705 不继承。
6. **验收边界：通过。** 目标机械可判：第 5 章存在含 2 条 Requirement、四处成对合并逐字、E0104 扩展后 registry clean、zh 孪生 + docs_sync。非目标六项明示。
7. **粒度：通过。** 单能力（iteration）+ 成对宿主章修订——配对本身是本变更的主张（该机制首次启用：词法+语法+新章同步）；不与类型/协议耦合（显式非目标 1、2）。

**结论：通过，进入 ready。**

## 审查记录

2026-09-03 语义审查（ready → active 关卡，10 点）：

**职责边界**：(1)–(4) 全部通过——proposal 纯黑盒；增量全为可观察行为（求值语义、闭集扩展、拒绝路径），BCP 14 仅用 MUST/MUST NOT；design D1–D8 与增量一致（D6 零新码/不认领段位与 What Changes 无段位项一致）；tasks 三项有来源/验证，无未决选择。

**内容质量**：(5) 覆盖 normal（for 取值语义、区间表达式、通配名）、boundary（右排他、空区间零迭代、break/continue、续行）、failure（E0202 取值位置、E0104 链、E0105 解构/开放端）✓；(6) 各 Requirement 均有行为增量 ✓；(7) spec 层断言先行——E0401 注入先于注册表维护（task 2）✓；(8) 负向断言真实（注入 + 增量内三码拒绝场景）✓；(9) 完成度闭环按 spec 层先例：for/区间求值语义已定，类型化以非目标 + 前向引用明示 ✓；(10) 裁决已闭环 ✓。

**机械复核（本关新增）**：四处 MODIFIED 的 Scenario 集与现行章节机器比对——全部前缀逐字一致，新增恰为两项（lexical "The range token splits from adjacent numerals"、precedence "Chained range is rejected"），另两处零场景变动。与审计点 3 捕获的起草错误互证：场景集核对列入实现期验证（task 1 已含）。

**边界说明（记录，无工件改动）**：(a) **E0102/E0202/E0205 复用面**——`..` 入续行集后行首 `..` 仍走 E0102（既例 `.method()` 同类）；for 取值位置走 E0202（条目通类措辞已覆盖）；E0205–E0299 预留注释不变。(b) **iteration delta 共 11 个 Scenario**（6+5），实现期以此数为准。

**结论：通过，进入 active。**

## 实现审查记录

2026-09-03 实现审查（active → complete 关卡，7 点）：

1. **规范符合性：通过。** 提升后机器逐字核对 6/6：第 5 章 2 条 Requirement（For statement / Range expression，场景集 6+5）与增量逐字一致；四处宿主合并（ch1 Operators and punctuation 场景集 3、ch2 Line-joining 4、Statements 4、Operator precedence 5）与 MODIFIED 逐字一致，场景集机器比对全匹配；`## Examples (non-authoritative)` 2028 字符与 examples.md 逐字一致。
2. **验证诚实性：通过。** tasks.md 三项验证行与实测一致：validate.py for-iteration --strict 与 --all --strict 均 OK（registry clean）；E0401 负例注入实跑（exit=1，消息原文 "diagnostic usage 'E0401:' has no registry entry"，还原 clean）；docs_sync 12 对全对齐；逐字/场景集核对为本审查实跑（见点 1）。任务 1 验证行中 "E0104/E0102/E0205 等码与 Scenarios 消息串逐一对齐" 复核：E0104/E0105/E0202 出现于增量场景，E0205 未出现于增量（仅提案叙述）——验证行措辞以 "等" 泛指，非虚报，记录不改。
3. **测试先行证据：通过（spec 层对应物）。** layers [spec]，无编译器代码；测试先行的对应物为注入先行：E0401 负例先于注册表维护执行并记录消息原文（task 2 验证行）；conformance §52/§62 不适用（无 --json 产物）。
4. **诊断协议稳定：通过。** 零新码（注册表仍 23 条）；E0104 仅 description/remediation 枚举扩展（比较集 → 比较集 + 区间链），owner/requirement 字段复核不变（0200-grammar / Operator precedence and associativity）；无删除、重命名、重定义语义。
5. **单一权威：通过。** 长期事实全部落位：新章 EN+zh、ch1/ch2 四处合并、注册表扩展；变更目录仅持增量与过程记录；EN 章/注册表英文，zh 孪生为配对翻译（结构镜像、诊断消息串保留英文原文）。
6. **红线复核：通过。** 工作树范围恰为 7 项（4 改宿主文件 + 注册表 + 2 新章 + 变更目录），无 refr/ 内容进入；commit 信息待用户明示后生成（无署名 trailer）；diff 与非目标六项无冲突（协议/组合子/含端与开放端/类型化/所有权/解构均未引入）。
7. **最小可信验证已跑：通过。** validate.py --all --strict（OK）+ docs_sync.py（12 对 OK）+ 机器逐字 6/6 + 示例节 diff 为零 + 全角冒号扫描 0 命中（修复 1 处：zh 示例注释 E0105：→E0105:）。

**发现与处理（F1）**：zh 孪生示例注释一处全角冒号（`E0105：`）违反"诊断消息串 ASCII 冒号"惯例，冒号扫描捕获后即改；EN 无同类问题。

**结论：通过，进入 complete，转归档。**

## 归档记录

2026-09-03 归档（complete → archived）：

1. **规范提升已完成**：`docs/spec/0500-iteration.md`（+zh 孪生）创建；ch1 Operators and punctuation、ch2 Line-joining / Statements / Operator precedence 四处 MODIFIED 逐字合并（EN+zh）；`docs/spec/diagnostics.toml` E0104 枚举扩展。
2. **决策提升：无 ADR。** design D1–D8 均为常规取舍（形式集、档位、续行集、munch 钉死、延后路径、零码、可观察语义、绑定形式），无原则例外与信任边界决策；design.md 随归档保留。
3. **状态提升：无 roadmap**（沿前例，openspec 层留档即止）。
4. **移动归档**：`git mv` 至 `openspec/changes/archive/2026-09-03-for-iteration`，status → archived。
5. **复验通过**：validate.py --all --strict OK（无活跃变更，registry clean）；docs_sync 12 对 OK。
