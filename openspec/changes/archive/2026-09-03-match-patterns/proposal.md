# Proposal: match-patterns

## Why

1. `match` 在第 1 章 21 词关键字清单中保留至今且无任何产生式——现在任何使用都落第 2 章意外 token 诊断（E0105）。控制流章节已显式把 match 排除为非目标，留给本章；队列顺序用户已认可（match → for/迭代 → 声明 → 类型）。
2. v0.8 §21.3–21.5 提供经演示验证的素材：模式种类表（字面量/绑定/通配/变体/嵌套/元组/或/守卫）、"守卫不计入穷尽性"纪律、字面量类型须 `_` 兜底。需按本仓分层重新批准：形式层现在定，类型层（穷尽性、类型化）前向引用。
3. 注册表机械约束：条目必须落在其 owner 拥有的段内（validate 检查 (d)，`validate.py:247`），而 match 的触发语义写在自己章节文件里 → 必须自有段位 E0300–E0399；未认领范围随之收窄至 E0400–E9999。E02xx 段内注释中"match arms"预留字样同步移除（归属已定）。

## 目标与非目标

目标：

1. 建第 4 章 `docs/spec/0400-match.md`（+zh 孪生）：match 表达式形式、臂、封闭模式集（字面量/绑定/通配/或/守卫）、求值语义（scrutinee 恰一次求值、臂自上而下、首中即取不再看后续臂）。
2. 注册表：段位 E0300–E0399（owner 0400-match）+ 首批 E0301/E0302 两码。
3. 第 2 章表达式骨架 MODIFIED：补"关键字引领的表达式形式按归属章节批准"增长规则——与语句族规则对称，并追溯性地为第 3 章 if-as-expression 定基（当时未修订骨架，属已披露的结构缺口）。

非目标：

1. 穷尽性检查——含"守卫分支不计入穷尽性证明、必须有守卫外兜底"（§21.4）与"字面量模式类型须 `_` 兜底"（§21.5）——全部前向引用类型章节；码在 E0303–E0399 预留。
2. 变体模式 `Name(args)` 与元组模式 `(a, b)` 及嵌套解构：随 sum types/元组经成对修订进入；此前按第 2 章 E0105 拒绝。
3. 臂可达性/支配分析（`_` 之后的死臂）→ 类型层（裁决 3：单一分析路径）。
4. 臂体类型一致、守卫条件布尔类型化、绑定类型推断 → 类型章节。
5. `_exh_ignore` 不存在（裁决 2：通配符仅 `_`）。
6. for/迭代与 `..` → 独立变更（队列下一项）。

## What Changes

1. `specs/match/spec.md`：6 条 ADDED Requirements——Match expression form、Match arms、Pattern set、Or-pattern binding consistency、Guards、Match diagnostics segment。
2. `specs/grammar/spec.md`：1 条 MODIFIED（Expression skeleton：关键字引领表达式形式的增长规则 + 新 Scenario）。
3. `docs/spec/diagnostics.toml`：新增段位 E0300–E0399；未认领段收窄 E0300–E9999 → E0400–E9999；E0200–E0299 段内注释去 "match arms" 字样；新增条目 E0301、E0302。

## 影响层

- spec：仅规范层（两个规范增量 + 注册表扩展）。不改编译器/工具行为。

## 影响范围

- 新章 `docs/spec/0400-match.md`（+zh）；`docs/spec/0200-grammar.md`（+zh）表达式骨架一条 MODIFIED；`docs/spec/diagnostics.toml`。

## 裁决记录

2026-09-03 用户裁决三项（均取推荐）：

1. **臂分隔：换行分隔，无分隔符**——与语句边界推断同构（第 2 章行接续模型管臂），不引入逗号。代价（接受）：单行多臂不可能；臂间逗号按未批准产生式拒绝（E0105）。
2. **`_exh_ignore`：不继承**——通配符仅 `_`（P2 单形式）。穷尽性豁免若确有需求，在类型章节批准穷尽性检查时连同语义设计，而非现在批准一个类型层落地前不可触发的空壳。
3. **臂可达性：延后至类型层**——可达性与穷尽性是同一分析的两面，在类型层一次批准；不建第二条更弱的句法分析路径。

## 审计记录

2026-09-03 审计（candidate → ready 关卡，7 点）：

1. **问题真实性：通过。** Why 三条均可核验：`match` 在第 1 章 21 词清单且全仓无产生式（grep 零命中冲突文本）；v0.8 素材固定 `refr/spec-0.8.md` §21.3–21.5（912–979 行，当日读取）；段位机械约束在 `validate.py:247`（条目须落 owner 拥有的段内——当日读取验证）。
2. **影响层声明：通过。** layers `[spec]`；两个规范增量 + 注册表扩展均 spec 层；不改编译器/工具行为。
3. **规范增量范围：通过。** 6 条 ADDED（match）+ 1 条 MODIFIED（grammar Expression skeleton）逐条列于 What Changes；E0301/E0302 对照注册表与 active 变更（当前无）无冲突；E/W 无同号冲突（E03xx 全空）；MODIFIED 旧文与 `docs/spec/0200-grammar.md` 现行文本 diff 核对——差异恰为插入句，零漂移；用法扫描只覆盖 docs/spec/，候选期 clean 属预期（条目提升前落位即可）。
4. **原则一致性：通过。** P2：臂体单规则（块即表达式，v0.8 双形式坍缩，D4）、通配符唯一（D2）、无分隔符 token（D1）；P1：模式集封闭且进入路径命名（D6）、段位封闭；P4：模式位与表达式位不相交，`=>` 依第 2 章"不出现在表达式内"在守卫条件处无歧义收束——无需 ADR（无原则例外）。
5. **参考基线固定：通过。** 素材固定 §21.3–21.5；偏离逐项披露：`_exh_ignore` 弃（D2）、穷尽性延后（D5）、变体/元组延后（D6）、空块 `Unit` 类型化不入（D4）；v0.8 码 E0304/E0305/E0306 不继承（本仓编号全新分配）。
6. **验收边界：通过。** 目标机械可判：第 4 章存在含 6 条 Requirement、注册表段位 + 2 条目、validate strict + registry clean、第 2 章骨架逐字合并、zh 孪生 + docs_sync。非目标六项明示排除面。
7. **粒度：通过。** 单能力 + 宿主章一条 MODIFIED + 注册表；不与 for/迭代/类型耦合（显式非目标 1、6）；validate + 注册表检查垂直验证。

**结论：通过，进入 ready。**

## 审查记录

2026-09-03 语义审查（ready → active 关卡，10 点）：

**职责边界**：(1)–(4) 全部通过——proposal 纯黑盒（Why #3 的 validate 机制引用属过程论证，与 control-flow 先例同式）；增量全为可观察行为（求值次序、选择语义、拒绝路径），BCP 14 仅用 MUST/MUST NOT、无 SHOULD/MAY；design D1–D8 与增量一致（D4 的"空块 Unit 属类型层"与增量零冲突）；tasks 三项均有来源/验证，无未决选择。

**内容质量**：(5) 覆盖 normal（取值/模式/守卫）、boundary（零臂 E0301、逗号与变体模式拒绝、守卫惰性）、failure（E0301/E0302/E0105 路径）✓；(6) 六条 Requirement 均有行为增量，无空章节 ✓；(7) spec 层断言先行——负例注入先于条目（task 2），先例同 control-flow ✓；(8) E0303 未注册码注入为真实负向验证 ✓；(9) 完成度闭环按 spec 层先例：求值语义已定（scrutinee 恰一次、首中即取、守卫惰性），类型化以非目标 + 前向引用明示 ✓；(10) 三项裁决已闭环，无阻塞性未决 ✓。

**边界说明（记录，无工件改动）**：(a) **E0105/E03xx 分界**——未批准产生式（臂间逗号、变体/元组模式）走第 2 章意外 token 兜底；E03xx 只拥有已批准形式的违规（E0301 零臂、E0302 或模式绑集）。与 control-flow 的 E0105/E0202 分界记录同式。(b) **Scenario 计数**——6 条共 18 个（4+3+5+2+2+2），实现期以此数为准。

**结论：通过，进入 active。**

## 实现审查记录

2026-09-03 实现审查（active → complete 关卡，7 点）：

1. **规范符合性：通过。** 第 4 章 6 条 Requirement 与增量逐字一致（diff 核对；第 6 条尾部差异为章节示例/术语装置已知误报，本体 common-lines 全等）；第 2 章表达式骨架与 MODIFIED 逐字一致（3 个 Scenario）；注册表 E0301/E0302 的 requirement 字段解析到章节实际标题（Match expression form / Or-pattern binding consistency），validate strict + registry clean。
2. **验证诚实性：通过，无发现。** 三项任务的验证命令本轮真实运行：validate（定向 + 全仓 strict）、docs_sync（11 对，含新 0400-match 对）、逐字 diff ×3、E0303 负例注入（消息原文 "diagnostic usage 'E0303:' has no registry entry"，exit=1，git checkout 还原）、全角冒号扫描（零）、场景计数双侧 grep（18）。E02xx 注释修订与 E0300-E9999 收窄均为注释/空段操作，未触碰任何既有条目。
3. **测试先行证据：N/A。** spec 层变更（layers `[spec]`），无编译器实现与测试；断言先行体现为注入先于条目（task 2 两步法记录在案）。
4. **诊断协议稳定：通过。** 新码 E0301/E0302 全局唯一、同变更登记；E0300 与 E0303–E0399 预留未用（触发扫描为零）；未动任何既有条目与 `--json` 字段。
5. **单一权威：通过。** 长期事实落位：段位与条目在 diagnostics.toml、触发语义在第 4 章 Requirements、示例节按通例并入章节正文、术语 10 条进第 4 章术语表（arm 复用第 3 章同一名，无第二种称呼）；变更目录仅存增量与过程记录。
6. **红线复核：通过。** `refr/` 未入暂存；commit 信息将无署名 trailer；diff 无越界（未动 validate.py 与任何工具）。
7. **最小可信验证已跑：通过。** spec 层验证阶梯全绿：validate.py --all --strict、docs_sync.py（11 对）、双语结构镜像 + 诊断消息串英文保留（E0301/E0302 各 4 处，EN+zh 对称）。

**结论：通过（无发现），进入 complete。**

## 归档记录

2026-09-03 归档（complete → archived）：

- **规范提升**：第 4 章 `docs/spec/0400-match.md` + `.zh.md`（6 Requirement / 18 Scenario，非权威示例节 4 小节，术语 10 条）；第 2 章表达式骨架按 MODIFIED 逐字更新（EN+zh，插入关键字引领表达式形式增长规则 + 新 Scenario "A keyword-led expression form is used"，追溯性为第 3 章 if-as-expression 定基）；注册表 E0300–E0399 段位 + E0301/E0302 条目 + 未认领收窄至 E0400–E9999 + E02xx 注释去 "match arms"。
- **决策提升**：无 ADR——三项裁决（换行分隔、弃 `_exh_ignore`、可达性延后）均遵循既有原则（P1/P2 单形式、单一分析路径），无原则例外；取舍理由保留于裁决记录与 design D1–D3。术语 10 条入第 4 章术语表，arm 与第 3 章同名同义。
- **状态提升**：无 docs/roadmap/，按技能约定不创建空目录，留档于此。
- **移动归档**：`git mv` 至 `openspec/changes/archive/2026-09-03-match-patterns`，status → archived。
- **复验**：validate --all --strict 通过（零 active 变更 + registry clean）。
