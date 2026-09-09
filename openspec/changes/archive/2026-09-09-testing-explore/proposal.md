# 提案 — testing-explore（M10c）

第 21 章 R6「Exploration and its guards」的运行塔实现：`we test --explore` 在调度器控制下重跑测试、选出确定性调度器不会取的交错，携两守卫诊断 E1901/E1902 与默认开启的偏序归约（POR）。**纯实现变更，不改变语言行为，无规范增量**——章文与两码注册表在册（E1901/E1902 owner 2100-toolchain / requirement Exploration / allocated 2026-09-04），本变更只实现。

## Why

ch21:126 已批面与 M10b 落地后的差距，逐条可验证：

1. **调度器只有一策**：M10b 的调度是确定性 FIFO（ready 队单链先进先出，sched.c:59-84；同刻唤醒 FIFO 由黄金 test-clock-same-tick-fifo 钉死）。「choosing interleavings the deterministic scheduler would not take」（ch21:126）需要第二策略——今日无处注入。
2. **两守卫诊断在册未实现**：E1901「exploration detected nondeterminism」（diagnostics.toml:1433）、E1902「unmocked effect executed during exploration」（:1442）已分配归 ch21 R6，运行时无任何发射点。「E1901 fires when explored runs diverge; E1902 when an explored run executes an unmocked custom effect」（ch21:128-129）是死文字。
3. **无迭代控制面**：`--iterations N` 与 manifest `[test].explore-iterations`（ch21:126、ch21:174——`a positive integer — a non-positive or non-integer value is E1903`）——[test] 表今日无读取面（check.go 手读 name/version/type + [dependencies] 计数）。
4. **无 POR 载体**：`partial-order reduction on by default, --no-reduce disabling it`（ch21:126）无对应机制。
5. **无探索报告面**：探索聚合结果（迭代数/等价类数/失败于哪个调度）在 M10b 的 pass/fail 行之外无处安放。

工程缺口（非实现偏好）：ch20:121 的确定性承诺（虚拟钟域内调度可重现）由 M10b 兑现，但该承诺的可信度今日只有「我们写了确定性代码」一证——探索塔是承诺的压力测试面，`--explore` 退出 0 是「该测试在 N 个调度下确定且全过」的可复现证据。

## 裁决记录（candidate 阶段，2026-09-08，四项均采纳推荐）

**Q1 交错机制 = 种子化随机调度**。每个调度决策点（park/唤醒后取队）以 PRNG 从就绪队列选取；种子 = f(测试身份, 迭代号)，同种子必产同交错；迭代 0 = 确定性基线序（现行 FIFO）。可复现性使黄金可 byte-exact 钉死，且「确定性调度器不会取的交错」有可判定语义：任何与基线序不同的选取序列。*被拒*：全交错枚举（组合爆炸，章文明言 exhaustion impossible）；旗标空转调度（构造半序让任务空转等真时序——违诚实原则，P9）。

**Q2 POR 面 = 等价类去重**。以「每任务自身可观察事件序」（Mazurkiewicz 等价）为键：迭代跑出的序落入已见等价类则在该迭代预算内重采样换新序，N 次迭代目标覆盖 N 个真行为类。诚实归约——全局序不同但各任务自序相同 = 等价，不重复探。*被拒*：睡眠集/源集全算法（研究级超配——等价类去重已拿到 POR 的主要收益「不重复探等价序」而无需证明完备性）；旗标空转。

**Q3 迭代形态 = 进程内迭代全复位**。harness 对每测试包迭代环（钟归零、槽恢复、任务清理后重跑）；一次编译一次执行，M9b 单进程协作调度的自然延伸。*被拒*：子进程每迭代重启（编译/装载成本 ×N，且 mock 槽/钟状态本就进程内——无跨进程收益）；整 __we_main 重入（调度作用于全套件混流，失每测试种子性与迹归属）。

**Q4 E1901 面 = 同调度双跑对比**。每个被探调度跑两遍，对比全局可观察迹（章文三事件：send/receive/completion 之序）；同调度下唯一可变项 = 虚拟确定性之外的东西在起作用 → E1901。*被拒*：跨迭代等价类分歧（把并发敏感误报为非确定性——两个不同调度的迹本就该不同）。

## 现状与差距

| 面 | 现状（M10b 后） | 差距 |
|---|---|---|
| 调度策略 | ready 队单链 FIFO，取队 ready_pop(sched.c:69) | 取队点策略注入 + PRNG + 种子推导 |
| 驱动形态 | emitDriver 内联每测试序列（mock 装→begin→spawn→await→end→恢复→report 分支，codegen.go:3399-3450） | 序列提取为可重入 thunk + 运行时 runner（一形两模式） |
| 迹记录 | 无 | 三事件（send/receive/completion）全局序 + 每任务序双记录，跨迭代规范化 id |
| 判定面 | 无 | E1901 双跑对比、E1902 custom-effect 槽门、POR 等价类键 |
| 死锁 | 域内死锁 abort + exit 1（M10b 黄金） | 探索态转换为该调度的失败（弃置余任务、续跑） |
| CLI | --filter 单旗标 | --explore / --iterations N / --no-reduce |
| manifest | [test] 无读取 | explore-iterations 读取 + E1903（注册表文点名该键） |
| 报告 | pass/fail 行 + test-result/test-summary 事件 | 探索聚合行 + test-result 增 explore 对象字段（R7 只增承诺内） |

## 目标与非目标

**目标（做完可机械判定）**：

1. `we test --explore` 项目面与单文件面全量：默认迭代数读 manifest `[test].explore-iterations`（缺省默认 100，实现自定披露），`--iterations N` 覆盖，`--no-reduce` 关 POR；非法 `--iterations` 值 = usage 2；退出码 0/1/2 复用（全确定全过 0 / 任一失败或守卫 1 / 编译败 2）。
2. 探索调度器：迭代 0 = 确定性基线（FIFO）；迭代 ≥1 = 种子化随机取队；种子 = splitmix64 混合（测试序号, 迭代号, 尝试号），C 单测钉死常数与序列。
3. 迭代环：每测试至多 N 迭代 × 每迭代至多 A=4 尝试 × 每尝试 2 跑（E1901 判定），全复位（钟归零/虚拟等待清理/残留任务弃置/mock 槽经 thunk 装/恢复/迹与计数器归零）。
4. E1901：同调度双跑全局迹对比，分歧即发（title 逐字自注册表），锚测试声明位置，双面渲染（stderr 人类行 / --json stdout 诊断事件）。
5. E1902：custom-effect fn（声明段含非内建标签）槽默认 = 同 ABI 直通门；探索态达门且未被 mock 覆盖即发；常态纯直通零语义差。
6. POR：等价键 = 每任务事件序 hash 的多重集；已见类重采样（预算内）；报告行携 attempts/classes 实数。
7. 报告面：探索聚合行 `pass/fail  {file}: {name} (explore {N} iterations, {A} attempts, {K} classes)`；--json 下 test-result 增 `"explore"` 对象（R7 只增字段）；duration_ms = 探索中最长单次虚拟时长（语义披露）。
8. 既有 588 枚 conformance 黄金**零回归**：不带 --explore 的行为逐字节不变（策略关/门直通/驱动重构行为中性）。
9. conformance 新黄金矩阵（预计 ~18，D11 表 19 槽 18 枚（explore-pass 兼旗标面），定数随 T1 披露）+ codegen/cli/runtime C 单测先行。

**非目标（显式栅出）**：

- W1910/W1911/W1912 三告警与 `[vet]` 表读取、E1903 的 vet 侧——`we vet` 层，M11 里程碑；本变更只落 E1901/E1902 与 manifest 侧 E1903。
- 研究级 POR 全算法（睡眠集/源集/完备性证明）——Q2 已裁等价类去重为面。
- 屏障队列与钟释放序的随机化——探索只动**调度选择**，ch20 钉死的同刻 FIFO 语义不动（域界，design D2）。
- E1901 的 We 级正例黄金——语言内今日无虚拟域非确定性源（ch20 兑现的自证），正例以 runtime C harness 注入测试承载（披露）。
- 探索态死锁转换的 We 级正例——本原语集 park 拓扑 schedule-独立，若 T1 不可构造即 C harness 承载（披露）。
- 覆盖率统计断言（classes 数的分布性质）——黄金只锁种子化确定输出。
- 跨迭代等价类分歧检测——Q4 已拒。
- GC 跨迭代回收——follow-up #11 族；迭代有界故泄漏有界。

## What Changes

- **runtime/c/sched.c**：取队点策略钩子（探索态随机索引取队，常态 FIFO 不动）；探索态死锁转换（force-fail 驱动 await + 全弃置，替代 abort）；splitmix64 PRNG。
- **runtime/c/test.c**：探索 runner 环（`__we_test_meta`/`__we_test_drive` 入口、迭代/尝试/双跑、全复位协议）；三事件迹记录（规范化 id）；POR 等价键集；E1901 对比器与双面渲染；E1902 门检查与渲染；探索聚合报告。
- **runtime/c/conc.c**：send/receive 迹事件挂钩（completion 事件在 sched.c task_done）。
- **runtime/c/startup.c**：argv 解析增 --explore/--iterations/--no-reduce（--json 先例）。
- **internal/codegen**：emitDriver 重构——每测试序列提取为 `@<key>.drive.<n>` thunk，`__we_main` 发 `__we_test_meta` + `__we_test_drive` 调用（一形两模式：常态 runner 直调 thunk 一次）；custom-effect fn 槽默认 = fxgate 直通门（真体 define 不改名，槽初值与恢复常量换 fxgate）。
- **internal/cli**：test 子命令三旗标解析；manifest `[test]` 读取与 E1903；runTestBinary argv 透传；迭代默认值解析。
- **internal/conformance**：test-explore-* 黄金矩阵。
- 单测：codegen m10c / cli m10c / runtime C harness m10c。

## 影响层

`change.yaml` layers = `[compiler, stdlib]`。compiler：codegen 发射形与 CLI 面（Go）；stdlib：runtime/c 运行时机制（C，M9b/M10b 载体惯例）。**不改变语言行为，无规范增量**——本变更不触 docs/spec/ 任何文件；E1901/E1902 注册表在册，manifest 侧 E1903 条文已点名 explore-iterations。

## 影响范围

- `docs/spec/`：零触碰（22 章与注册表早权威）。
- `docs/roadmap/0000-reference-implementation.md`（+.zh.md）：归档时 M10c 行 pending → done（T10）。
- `internal/codegen/codegen.go` + `internal/codegen/m10b_test.go`（HarnessSynthesis 形随批更新，披露）+ 新 `m10c_test.go`。
- `internal/cli/{cli.go,test.go,check.go}` + 新 `m10c_test.go`。
- `runtime/c/{sched.c,sched.h,test.c,conc.c,startup.c}` + C harness 新 m10c 套件；`runtime/runtime.go` embed 集不变（无新 .c 文件）。
- `internal/conformance/testdata/cases/`：新增 test-explore-*（既有 588 枚零触碰——目标 8 的负面对账）。
- `openspec/changes/testing-explore/`（本变更）。

## 涉触码盘点

- `internal/codegen/codegen.go`：emitDriver 重构（thunk 提取 + meta/drive 调用形）；slotFor 默认值面（custom-effect fn → fxgate）；驱动恢复常量；TestDecl 携位置入 meta。
- `internal/codegen/m10b_test.go`：TestM10bHarnessSynthesis 随发射形更新（IR 钉死单测随批改写，披露——行为面 conformance 黄金不动为证）。
- `internal/cli/cli.go`：parseOptions 增 --explore/--iterations/--no-reduce（test 子命令专属，= 形，非法值 usage 2）。
- `internal/cli/test.go`：runTestBinary argv 透传三旗标（--json 先例）。
- `internal/cli/check.go`：loadManifest 增 [test] 表读取（explore-iterations 正整数门 → E1903）。
- `runtime/c/sched.c`：取队点钩子、死锁转换、PRNG（或独立小节）。
- `runtime/c/test.c`：runner 环/迹/键集/对比器/渲染器（主要增量，~新增 300 行级）。
- `runtime/c/conc.c`：两事件挂钩。
- `runtime/c/startup.c`：argv 三旗标。
- `runtime/c/sched.h`：跨文件声明。

## 已知风险与开放问题

1. **attempts 语义 vs 章文「bounding the runs」**：`--iterations N` 读作迭代数（N 个探索迭代，每迭代预算内重采样），最坏 2·A·N = 8N 次执行。报告行携 attempts 实数保持诚实。若未来裁决要求字面 N 次执行，改预算策略不动机制。
2. **E1901/E1902 渲染双权威**：C 侧渲染复制 M0 diag.Human/JSON 格式（字段序/分隔符）。格式权威 = M0 快照与注册表 title；Go 侧永不渲染这两码（只生 under exploration）。注册表 go:embed（follow-up #2）落地时 C 侧字面量同批对齐。
3. **位置粒度**：E1901/E1902 锚测试声明位置（file:line:col 自 TestDecl.Pos）——B2 调试基建（源映射）前无调用点位置。披露为已知粒度。
4. **常态成本**：custom-effect fn 调用多一跳直通门（槽形先例成本，M10b 裁决 3 同族）；`__we_test_drive` 常态 = 每测试一次额外间接调用。
5. **迹上限**：每跑 4096 事件，溢出置截断标志——该跑不参与 E1901 对比（诚实跳过），等价键含截断位。披露。
6. **死锁转换构造面**：见非目标；T1 尝试构造，不成则 C harness 承载 + 披露。
7. **classes 确定性**：种子固定 → 同机器同输出（黄金可 pin）；跨机器整数运算均为 uint64 无 UB，可复现。

## 审计记录（2026-09-08，welang-spec-impact-audit 7 条，通过）

1. 问题真实性 ✓——Why 五条全黑盒可验证，ch21:126-141 与 diagnostics.toml:1433/:1442 行锚引用。
2. 影响层 ✓——change.yaml `[compiler, stdlib]` 与影响层节一致；纯实现豁免标记（「不改变语言行为，无规范增量」）文首与影响层两处。
3. 规范增量范围 ✓——零新码零新 Requirement；E1901/E1902 注册表在册（owner 2100-toolchain，allocated 2026-09-04），E1903 复用（注册表 description 已点名 `an explore-iterations that is not a positive integer`，manifest 侧非新分配）；无 active 变更冲突。
4. 原则一致性 ✓——P9（采样永非穷尽 + attempts 实数 + 截断诚实跳过）、P1（种子确定、判定局部）；无原则突破，无待 ADR 项。
5. 参考基线 ✓——全部锚 docs/spec/ 现行文行号，无 refr 依赖。
6. 验收边界 ✓——目标 1–9 机械可判（旗标/退出码/黄金计数/588 零回归负面对账）；非目标七栅足防蔓延。
7. 粒度 ✓——单一垂直能力（探索塔），三处改动同服务一能力。

结论：通过，status → ready。`validate.py testing-explore --strict` OK。

## 审查记录（2026-09-08，welang-change-review 10 条，通过；status → active）

先跑 `validate.py testing-explore --strict`：OK（结构过，入语义审查）。

**职责边界**：①proposal 只讲黑盒问题与目标 ✓（What Changes/涉触码盘点按 M10a/M10b 内部变更先例载机制概览与触面盘点）；②spec 增量：零（纯实现豁免路径，无 specs/ 目录）✓；③design 唯一最小路径 + Q1–Q4 被拒方案记录 + 行号级引用，与增量无矛盾 ✓；④tasks 全项有来源/验证 ✓（T4「We 级不可构造则 C 承载」是已定义的回退路径非未决选择）。

**内容质量**：⑤场景覆盖 normal（pass/fail/守卫/POR/manifest/旗标）+ boundary（空集/filter/单文件/编译败/usage/默认值）+ failure-degradation（迹截断/尝试停滞/死锁转换/守卫中止）✓；⑥无空章节 ✓；⑦T1/T2 先红 ✓；⑧负向断言真实（E1901 负例、mock 后静默、E1903 构造、usage 构造）✓；⑨完成度闭环——**发现 F1**；⑩无行为阻塞的开放问题 ✓（风险 1 的 attempts 语义已定形为披露而非未决）。

**发现与处置**：
- **F1（完成度闭环——检查侧论证缺位）**：design 未显式论证检查塔零改动。处置：D6 尾补「检查塔零改动论证」——零语言面，custom-tag 谓词 codegen 直读声明段原文（内建 = 裸 io/net/time 三键），manifest 侧 E1903 走既有装载门。
- **F2（唯一路径——guard 检查位序未定）**：E1902 于双跑第一跑触发时第二次跑无意义且可能在坏状态跑。处置：D3 环补「双跑之间查 guard_aborted」注释行。
- **F3（计数对齐，M10a「22→20」同族）**：proposal 目标 9 与 design D11/tasks T1 写「预计 ~17」，D11 表实枚 18 枚。处置：三处对齐 ~18（表枚 18）。
- **F4（成文笔误）**：D7 诚实边界句自问嵌陈述成文混乱。处置：重写为直陈——粗一档换来身份免配对，被合并类内 per-task 序全同、可观察面下无可区分差异。

结论：通过，status → active。

## 审查记录（2026-09-09，welang-code-review 7 条，实现完成后 active → complete 关）

1. **规范符合性** ✓——零规范增量纯实现变更；抽查实证：E1901/E1902 title 与 docs/spec/diagnostics.toml:1433/:1442 逐字吻合（test.c:386/:418）；E1903 报文点名 key/legal/found 三要素（注册条 description 自定形）；ch21:126-141 的迭代 0 基线/POR 默认开/--no-reduce 关/两守卫/退出码 0-1-2 全由黄金与探针钉死；无规范外行为（custom-tag 判定直读声明段原文，ch16 内建 = 裸三键）。
2. **验证诚实性** ✓——T1–T8 逐项核对：各完成记录的验证命令均真实运行且结果记录在案（本窗口 T6/T7/T8 的运行输出全数在转录：cli 绿/vet 净/conformance 606 fresh/六探针/D12 八步）；无「勾了但没跑」项。T4 判定序缺陷（失败尝试入键集致 classes/attempts 错）为红测试揭出、当场修复并披露——验证面起了作用的自证。
3. **测试先行证据** ✓——T1 黄金 18 枚先落先红（红因分类两类记录）；T2 单测先红（codegen 4 红 + cli 编译红）；runtime C 惯例同任务先红（T3 6/6、T4 8/8）；黄金期望输出符合 §52 人类行（M10b 行形延伸）与 §62 JSON Lines（字段序 type/…/explore 逐字段 python 断言）。
4. **诊断协议稳定** ✓——--json 既有字段零删零改，唯一 JSON 面变化 = test-result 增可选 explore 对象（R7 只增承诺内，常态缺省省略）；E1901/E1902 注册表在册实现（allocated 2026-09-04），E1903 复用（注册条 description 已点名 explore-iterations）；零新码分配。
5. **单一权威** ✓——零规范提升零新 ADR（机制载体 = 本变更记录 + roadmap 行，M10c 裁定）；`__we_explore_on` 单权威于 test.c（io.c/sched.c 仅 extern）；E1901 message 渲染与聚合 reason 单一数据源；代码注释/docs 英文（diff 全量 CJK 扫描零命中）。
6. **红线复核** ✓——refr/ 零触碰（git status grep 零匹配）；未提交（候用户明示，无 commit 故无 trailer 面）；diff = 11 跟踪文件 +1060/−46 + 新增测试/黄金/变更目录，全部 M10c 材料，无越界改动。
7. **最小可信验证已跑** ✓——D12 阶梯八步固定序全绿（build/vet/test 全包、runtime C harness、conformance 606 fresh、validate --all --strict、docs_sync 31 对齐、git status 对账），与 welang-pre-push-checks 对本 diff 的选取一致；黑盒六探针另附（T7）。

**实现期发现与处置汇总**（全部当时披露于 tasks.md 对应完成记录，此处汇总复核）：T1 黄金两处实现前修正（exit 码 1→2、E1902 锚 7:5→7:1）+ order-found 源笔误（Option 直比 Int64 命 Eq 域边界，match 解载荷修正——语义零改，钉值首跑吻合）；T2 codegen 期望值笔误两处（flen 15/缩进）；T3 design D4 回填两处（规范化 id 基准、双记录形）；T4 判定序缺陷修复 + 测试侧笔误两处 + TestIOHarness 编译集扩容；T6 argv 单元素缺陷（join 盲区，两元素修复）。m10b HarnessSynthesis「随批更新」消解为零改动（指令片段在 thunk 内存活——比计划更优，T2 披露）。

结论：通过，status → complete。
