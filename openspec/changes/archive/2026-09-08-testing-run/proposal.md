# 提案 — testing-run（M10b）

## Why

roadmap M10b 承诺 ch20/ch21 的运行塔六件：**codegen 多函数发射拓宽、`we test` runner、虚拟钟、确定性调度、test 边界、mock 拦截**。ch20 已批全量（虚拟钟 R3、advanceTime R4、确定性调度 R5、test 边界 R6），ch21 `we test`（R5：tests/ 递归发现、文件内源序、pass/fail 两结局、`--filter`、退出码 0/1/2）同在册——本变更与 M9b 同为**纯实现，规范增量零**。

M10a 检查塔（2026-09-08 归档）把一切运行面停在两处诚实边界：codegen 的 `bndTestModule`（test 模块）与 `bndOtherFns`（除 main 外的函数——「multi-function codegen widening」正是 M10b 行文）；`we test` 子命令自 M0 起是 exit-70 边界行。四组等待归位的运行语义全部可黑盒验证：

1. **mock 拦截**：M10a Q2 已裁「运行时按名函数表」——ch20 cache 示例定死拦截须覆盖 src 模块 helper 体内的调用点（mock `store.read` 后 `load` 内部调用也被拦），检查塔只落了声明检查。
2. **test 边界**：M10a Q3 已裁「每测试一任务」——panic 在边界转为该测试失败、进程续跑（ch20 R6 第三捕获边界）。
3. **虚拟钟**：advanceTime 检查塔只落了定型与 E1806 位置；运行语义（钟只动于 advanceTime 与登记等待、scope timeout 读虚拟钟、同刻唤醒确定序）全部待落。ch20 场景明文需要 stdlib 钟读数（"calls the standard library's clock twice"），而 stdlib 至今无 time 面。
4. **确定性调度**：M9b 单线程协作调度器的唯一非确定源是真实时钟 deadline；测试域内 time 等待全部走虚拟钟后即全确定（ch18 carve-out 的兑现面）。

## 裁决记录（candidate 阶段四项裁决，2026-09-08，均采纳推荐；M10a 预裁两项继承）

- **Q1 std.time 最小面 = 随本变更落地**：`std.time` 第四枚合成模块（M8 StdModule 机制）——`now() -> Int64 effect time` / `sleep(ms: Int64) effect time`；测试域内走虚拟钟、域外走真实钟（调度器 deadline 机器复用）。被拒面：推迟（ch20「钟读数两次相同」场景无 stdlib 面可测，黄金覆盖缺口）。
- **Q2 运行形态 = 单二进制、排序路径序、顺序跑**：全部 test 模块 + 各自导入图并集成一个 harness 二进制（一次 clang 链接），模块初始化按 ch15 后序恰一次（顶层绑定不发射——design D1 披露模块 init 空转），文件间按路径排序序、文件内源序。被拒面：每 test 模块一二进制（隔离强但 N 次链接、聚合面自建；跨模块 mock 表进程内全局无差）。
- **Q3 mock 槽编译形 = 一律槽形**：所有构建中模块级单态 fn 的调用位皆经槽间接调用（槽默认 = 真体，测试入口装/卸）。一种发射形、一套测试面；普通 build 每次调用多一次指针加载（参考构建接受，优化归 B1 后）。被拒面：仅测试二进制槽形（普通构建零开销，但两种发射形 = 两套 IR 对拍面）。
- **Q4 `we test --json` 面 = 落 ch21 R7 已批 test 事件 schema**（design 期修正）：每测试一 `test-result`{file, name, status, duration_ms} 事件 + 一 `test-summary`{total, passed, failed, duration_ms} 汇总——design 期核对 ch21 R7（2100-toolchain.md:148）发现该 schema **已批**（字段稳定性是章文承诺），Q4 起草时「实现自有 schema 披露非规范面」的框法作废，规范权威优先；实现自有面收窄为 ch21 未固定处（人类报告行格式、duration_ms 语义 = 测试域虚拟钟差、status 拼写 pass/fail）。编译诊断照常 JSON Lines。被拒面：--json 仅诊断（消费者拿不到机器可读测试结果）。
- **继承（M10a candidate 预裁）**：mock 拦截载体 = 运行时按名函数表（Q3 本变更落编译形）；test 边界载体 = 每测试一任务（design D5 落地）。

## 现状与差距

- **codegen**：`Emit(f *ast.File, module)` 单文件单模块（codegen.go:268）——多模块程序只发射根文件、非 std 导入无发射面；item 走查 `bndOtherFns`（:292/:344）拦一切非 main fn、`bndTestModule`（:327）拦 test 模块、`bndTopLets`（:294）拦顶层绑定。语句集 = M9b 双层直线集（标量 i64 域、sum 双槽、match/while/if、defer、`?`、task/scope/select、primitives）——**本变更是「函数处处可发射」，语句集不拓宽**（表达式全量是 B1 `codegen-full`）。
- **调用 ABI**：main 无参、task thunk 走 env 块、回调走标量直传——We 级 fn 间调用（参数/返回跨 fn 边界）尚无 ABI，M10b 定形（design D2）。
- **mock**：检查塔全链在位（E1802–E1805、mockSeen、目标段体走查）；MockDecl 在 codegen 无 case（TestDecl 停点先于体内任何发射，M10a D9 不可达论证）。
- **虚拟钟**：M9b 调度器 deadline = CLOCK_MONOTONIC 绝对毫秒 + clock_nanosleep TIMER_ABSTIME；无钟状态机、无 advance、无测试域分派。
- **std.test**：合成声明 + 检查面（assertTrue/assertFalse/assertEqual 特型面）在位；**运行时无比较量、无失败报文**——assert 族运行面（M9b panic 机器）可达，assertEqual 域外比较与失配报文待落。
- **CLI**：`"test": {takesPath: true}`（cli.go:43）注册在案，dispatch 未列（cli.go:106）——exit-70 边界行。conformance Case schema（args/setup/files/exit/stdout/stderr）可直接承载 runner 黄金（`cli.Run` 进程内跑，clang 子进程与 build 黄金同路）。
- **装载**：test 模块的项目面通路——`we test` 发现的 test 模块各自作为独立编译单元挂在项目图上（其导入映射 src/）；`src/*_test.we` 不在默认集（ch21:101 场景），且经导入图不可达（follow-up #12 张力维持，非本变更面）。

## 目标与非目标

**目标**：

1. codegen 多函数发射：程序级发射（多模块、模块限定符符号、跨模块调用）、We 级 fn 调用 ABI（标量/String 双字/record 指针/sum 双槽族——精确形 T2 定形）、`bndOtherFns` 与 `bndTestModule` 删、泛型 fn 声明新停点（单态化归 B1）、非 main fn 体越集新停点 `bndFnBody`（既有 main/task/callback 词零触碰）。
2. mock 槽拦截：每个模块级单态 fn 真体 + 全局槽，一切调用位经槽间接（helper 体内天然覆盖——cache 示例形）；std fn 条目同槽（io println/print、test assertTrue/assertFalse、time now/sleep——检查面 mock 字面合法，运行面必须真拦）；channel 构造面例外与登记的 follow-up 见 design D3 披露；harness 于 test 开始按源序装、结束恢复；mock 体按目标签名发射。
3. 虚拟钟运行时：钟状态机（虚拟/真实模式、每测试复位归零）、`__we_advance`（释放登记序 FIFO 的到期等待）、deadline 双源分派（scope timeout 与 sleep 测试域内走虚拟、域外走 M9b 真实路径）、test 结束残留虚拟等待作废（M9b 取消机器复用，披露）。
4. std.time 最小面：`StdModule("time")` 第四枚——`now()`/`sleep(ms)`，测试域内虚拟、域外真实（Q1）。
5. test 边界与 harness：合成 `__we_main` 逐测试 spawn 任务 + await——Ok = pass、TaskPanic = fail（复用 M9b 任务边界全套：panic 报文、GC 窗口、完成链）；panic 报文为失败原因行；进程续跑下一测试。
6. std.test 运行面：assertTrue/assertFalse/assertEqual 运行时比较（标量/Bool/String memcmp）与失配报文，失败经 assert 族一条 panic 路。
7. `we test` runner：tests/ 递归发现（路径排序序）、单二进制构建执行（Q2）、文件内源序、退出码 0/1/2（编译失败零测试跑）、`--filter`（regexp 实现读）、`--json` 事件面（Q4——ch21 R7 已批 schema：test-result/test-summary 两事件）、单文件测试编译形（std-only 导入）。
8. conformance 黄金新增（T1 定数 45，初稿 ~38 见 F3）+ 单测先行全周期 + 黑盒电池；既有黄金随批更新逐枚对账披露（M10a 两枚 test-module build 黄金预期回落 `whatSingleFileBuild`——follow-up #6 维持；`build-bnd-conc-fn` 等 `bndOtherFns` 钉逐枚对账）。

**非目标**：

- 表达式/语句集拓宽（for/迭代器运行时、闭包任意捕获、组合子真体、String 非字面量构建——**B1 `codegen-full`**；本变更语句集 = M9b 集）。
- 泛型单态化（泛型 fn 声明停点——B1）；顶层绑定发射（`bndTopLets` 维持——模块 init 空转，B 轨归位）。
- `--explore`、POR、E1901/E1902、`[test].explore-iterations`（**M10c**）。
- test 模块可导入性/`src/*_test.we` 编译通路（follow-up #12 维持）；单文件工件命名（follow-up #6 维持——单文件测试编译的工件走临时目录，披露）。
- 依赖非空项目的测试运行（既有依赖边界门照拦——M4 起面）；跨文件并行、测试级超时（ch21 留实现，顺序跑已定）。
- 规范增量与诊断注册表改动（零——全部行为由已批 ch20/ch21 固定）。

## What Changes

无规范增量：ch20 R3–R6 与 ch21 R5 已批、E1801–E1806 与 runner 退出码已在册，本变更全部行为都是已批面的落实现。变更交付：程序级 codegen（多函数/跨模块/槽形调用/harness/test fn/mock fn）、虚拟钟运行时与 deadline 双源、std.time 第四枚、std.test 运行比较面、`we test` CLI 全链（发现/构建/执行/报告/filter/--json/退出码）。被替换的边界：`bndTestModule` 全删（codegen case + 常量 + M10a 单文件路由改回落）、`bndOtherFns` 全删、`we test` 子命令 exit-70 边界行删；新增诚实停点 `bndGenericFns`（泛型声明）与 `bndFnBody`（非 main fn 体越集）。

## 影响层

| 层 | 触及 |
| --- | --- |
| compiler | codegen 程序级发射与槽形、cli test runner、conformance 黄金 |
| stdlib | std.time 合成模块第四枚、std.test 运行比较面、虚拟钟与 harness 运行载体（runtime/c/ 的 C 实现） |

（纯实现——「无规范增量」豁免路径，规范层零触及。）

## 影响范围

| 层 | 文件 | 动作 |
| --- | --- | --- |
| compiler | `internal/codegen/codegen.go` | 程序级 Emit、多函数/槽形/harness/test fn/mock fn、bndOtherFns/bndTestModule 删、bndGenericFns/bndFnBody 新 |
| compiler | `internal/cli/test.go`（新） | `we test` runner：发现/编译/构建/执行/报告/filter/--json |
| compiler | `internal/cli/cli.go` | dispatch 增 `test` 路由 |
| compiler | `internal/cli/build.go` | EmitProgram 接线；M10a 单文件 test 路由改回落（D8） |
| stdlib | `internal/typecheck/typecheck.go` | `StdModule("time")` 第四枚（now/sleep 合成声明） |
| stdlib | `runtime/c/test.c`（新）、`runtime/c/sched.c`、`runtime/c/startup.c` | 钟状态机/advance/deadline 双源/残留作废；harness 启动接 argv（--json） |
| stdlib | `runtime/runtime.go` | test.c embed |
| tests | codegen/cli 单测（m10b）、runtime C harness、conformance 黄金（T1 定数 45 新增） | 测试先行 |
| docs | `docs/roadmap/0000-reference-implementation.md`（+ .zh.md） | M10b 行翻 done（归档时） |

## 涉触码盘点

- codegen.go：Emit 单文件形保留（单测/build 单文件路径）、新增程序级装配；pass 一扩全局 fn 表（modkey+name → 符号/槽）；item 走查 FnDecl 全发射（main 在测试编译降格为普通 fn——入口归 harness，披露）、TypeParams 非空停 bndGenericFns、TopLet 维持 bndTopLets、TestDecl 发射 test fn、MockDecl 发射 mock fn；调用位一律槽间接（内建调用面——panic 族/组合子/advanceTime——维持直呼，std 条目经槽，D3 分界）；emitBody 复用（fn 语境参数化）。
- sched.c：deadline 入口按钟模式分派；等待登记链挂钟；`__we_advance` 释放序；test 结束作废。
- test.c：`__we_test_begin/end`（钟复位/模式切换/报告收集）、assertEqual 比较与报文、now/sleep 运行面。
- cli：test.go 发现（filepath.WalkDir + 排序）、并图编译（装载机器复用）、EmitProgram → clang → 执行子进程（argv 注入 filter）、退出码映射、报告渲染、--json 事件；cli.go（test 注册分派、--filter 解析、exitCompileFailure 常量）、build.go（compileProgram 提取——build/test 共用钉版 clang 序）、check.go（loadProject 第四返回值 count → []Module 的连带更名）。（审查 F1 补行）
- typecheck.go：std.time 第四枚合成模块装载（Q1）+ CheckTestRoot——test 根无 main 约定的检查入口。（审查 F1 补行）
- runtime 载体：sched.h（test.c 钟面的跨文件声明）、startup.c（argv 读 `--json` 切 test.c 双形渲染）、runtime.go（embed TestSource）、runtime 单测编译集补链 test.c。（审查 F1 补行）
- conformance：黄金 45 新增（T1 矩阵 design D10 定数，543 → 588）；既有随批预期——M10a 两枚（build-bnd-test-module/build-bnd-test-fns-first）回落 whatSingleFileBuild、build-bnd-conc-fn 逐枚对账（fn 体今可发射，停点位 T1 实测定）、其余零触碰预期。
- 既有单测更新面：codegen 边界单测（M10a TestM10aTestModuleBoundary 两形——行为翻面随批披露）、M9a/M9b 停点单测若钉 bndOtherFns 形逐枚对账。

## 已知风险与开放问题

- **调用 ABI 定形**：跨 fn 边界的值形（String 双字、sum 双槽、record 指针）在返回位与参数位的精确传递（sret 出参 vs 聚合值返回）——design D2 钉族、T2 单测定形（M9b T2 先例）。
- **同刻唤醒确定性策略**：登记序 FIFO 是运行时自有读（ch20「the runtime's own」），design D4 披露为决议。
- **test 边界后的残留任务**：测试结束时仍 park 在虚拟钟的内部任务作废——规范未定此形（ch20 只钉「run continues past the failed test」），design D4 披露为实现决议。
- **槽形对既有 IR 黄金的触碰面**：一律槽形改变一切 fn 调用的发射形——既有 IR 对拍黄金（m8HelloIR 等 codegen 单测）预期随批更新，逐处披露。
- **channel mock 张力（审查 F1 披露，随归档登记 follow-up）**：M10a 字面裁决钉了 `mock conc.channel` 检查面接受（m10a_test.go:178），但其运行面是 primCtor 构造（期望类型派生实参，非声明签名可复述）——唯一可复述的 mock 签名是无返回 unit 形，运行面无法诚实拦截。本变更维持检查面零改、channel 不入槽；E1804 构造面重分类 vs M10a「无特判」理据留作独立裁决点（design D3）。
- **开放问题（design 收敛）**：非 main fn 体越集停点的词面（D8——bndFnBody 新词 vs 复用既有词）；`build-bnd-conc-fn` 等历史钉的行为落点（D8——T1 实测对账）；单文件测试编译的工件落点（D6——临时目录）。

## 审计记录

**2026-09-08：通过（7/7，发现 2 项已当场处置），status → ready。**

1. 问题真实性通过——Why 的四组运行语义缺口（mock 拦截、test 边界、虚拟钟、确定性调度）皆为可黑盒验证的工程缺口：roadmap M10b 行在册、ch20 R3–R6（2000-testing.md :76/:100/:119/:138）与 ch21 R5（:95）已批、M10a 归档停点事实（bndTestModule/bndOtherFns、cli.go:43 exit-70）全部可对照工作树复核。
2. 影响层通过——change.yaml `layers: [compiler, stdlib]` 与影响层表逐行一致；纯实现路径（M9a/M9b/M10a 先例）：行为全部由已批章节固定，What Changes 显式「无规范增量」，非目标末条显式声明规范增量与注册表零触及。**发现 F1（已处置）**：影响范围表初稿以 `runtime` 作层标签——非 layers 集合成员；按 M9b 先例（runtime/c/ 文件行归 stdlib 层）改标，影响层行同步点名运行载体。
3. 规范增量范围通过——增量零；本变更触及的 E1801–E1806/E1301–E1305 族全部在册落实现，无新增码、无冲突、无与 active 变更重复（无 active 变更）。
4. 原则一致性通过——无隐式转换、无原则例外；Q3 槽形以一次指针加载换单一发射形是实现形选择（非规范面），「参考构建接受、优化归 B 轨后」显式披露；同刻 FIFO/残留作废/虚拟域死锁归 1 等 ch20 静默处全部以 design 决议披露非静默发明。
5. 参考基线通过——引用全部为已批 docs/spec 文件并锚行号（design 首行固定 2026-09-08 工作树基线）；follow-up #6/#12/#13 以「维持」引用未当既成规范消费。
6. 验收边界通过——目标 1–8 机械可判（黄金矩阵 ~38 对账、D8 六枚既有黄金落点表、停点更替清单、退出码四形）；非目标 7 条栅出 B1（表达式/语句集/泛型/顶层绑定）、M10c（探索面）、follow-up #6/#12 维持面。
7. 粒度通过——运行塔单垂直单元（codegen 拓宽 → 槽拦截 → 虚拟钟 → harness 边界 → runner → 黄金），M9b 运行塔同构先例；探索塔（M10c）与全量发射（B1）各自独立走门。

**发现 F2（已处置，Q4 起草期修正）**：candidate 裁决时 Q4 框「实现自有 schema 披露非规范面」——审计对照 ch21 R7（2100-toolchain.md:148）发现 test 事件 schema（test-result/test-summary 字段集与稳定性承诺）**已批**，规范权威优先：裁决记录、目标 7、design D6 三处同步修正为「落 ch21 R7 已批 schema 原样」，实现自有面收窄为章文未固定处（人类报告行、duration_ms 语义、status 拼写）。

`openspec/tools/validate.py testing-run --strict`：OK。

## 审查记录

**2026-09-08：通过（10/10，发现 3 项已当场处置），status → active。**

1. **proposal 职责**通过——Why 四组缺口皆黑盒可验证（roadmap/ch20/ch21/M10a 停点对照），目标 1–8 为行为目标；实现决策全部下沉 design（proposal 以 D 编号引用）；无任务清单混入。裁决记录与涉触码盘点为 M9a 起既定格式（决策日志 + 现状码行锚）。
2. **spec 增量**通过——纯实现豁免路径（M9b/M10a 先例）：行为全部由已批 ch20 R3–R6、ch21 R5/R7 固定，What Changes 显式声明零增量，非目标末条栅死；无伪 spec 段、无 specs/ 目录。
3. **design**通过——D1–D12 每处唯一最小路径，被拒替代具理据（D1 init 空转、D3 双发射形、D5 C 驱动/进程隔离）；引用精确到 file:line（codegen.go:268/:292/:327/:615/:919、typecheck.go:2285/:2323/:2469、m10a_test.go:178）与章文行号（ch20:48/:76/:119/:138/:157、ch21:95/:113/:148）；与零增量无矛盾——规范静默处（同刻 FIFO、残留作废、abort→1、init 静态化）皆以披露决议呈现非静默发明。**发现 F1（已处置）**：D3 初稿只列 std.io 入槽而 D7 误记 `st.assertTrue/assertFalse` 直呼无槽——二者皆合成 Pub FnDecl 条目（typecheck.go:2285，M10a D8「std.io println precedent」），检查面 mock 字面合法而运行不拦即结构化谎言；修正为 std fn 条目一律入槽（io/test/time），channel 构造面例外带理据与 follow-up（见 F1 全文于风险节与 design D3）。
4. **tasks**通过——T1–T11 每项来源（proposal 目标 / design D）与验证（命令 + 预期）齐备；无 deferred：T11「等用户明示提交」是提交流程红线非悬置方案，T1「T1 定数」是任务内精化非未决设计。
5. **场景覆盖**通过——D10 矩阵 normal（过/调用/钟推进/JSON 双行）、boundary（空集空跑、filter 空匹配、exit 2、abort 转达、单文件两形、域外 assertEqual、src/*_test.we 不入集）、failure-degradation（失败续跑、panic 边界、虚拟域死锁诚实 abort、bnd 两新停点）三路齐。
6. **无空章节**通过——四工件章节皆实。
7. **测试先行**通过——T1 黄金先红（红因三分类记档）+ T2 单测先红（codegen/typecheck/cli，runtime C 随 T4 按 M9b 惯例）先于一切实现任务。
8. **负向断言**通过——bndGenericFns/bndFnBody 黄金（编译期越界形）、assertEqual 域外停点黄金、`src/*_test.we` 集外黄金、filter 空匹配 exit 0；既有 E1801–E1806 负例族零触碰由 D8 表对账钉住。
9. **完成度闭环**通过——类型检查（std.time 装载，既有链零改）、代码生成（D1 程序级/D2 ABI/D3 槽/D5 harness/D7 断言面/D8 停点）、运行时（D4 钟状态机 + D7 助手 + harness 运行载体）三要素设计齐。
10. **未决问题阻塞**通过——proposal 三开放问题（bndFnBody 词面 / conc-fn 落点 / 单文件工件位）已全部在 design 收敛（D8/D8/D6）；审查新增的 channel mock 张力（F1）是**已决的边界披露 + follow-up 登记**（本变更内决议：不入槽、检查面零改），重分类裁决显式划出本变更范围，不构成阻塞。

**发现 F2（已处置）**：D10 第 7 枚 mock 黄金初稿语义混乱且引了凭记忆的短语（「按名拦截不递归」非章文）——回读 ch20 原文重推导：「the target's own body … may never run」（:48）确认真体不可达已被 cache 形覆盖（槽形下 mock 体内调目标名即拦自身，无需编译期豁免——D3 一律槽形与章文相容）；该枚改为「mock 不拦他名」正钉形黄金（「a mock of `f` does not mock what `f` calls」场景的正钉）。

**发现 F3（已处置）**：黄金计数提案正文三处 ~40 与 design D10/审计记录 ~38 不齐——统一为 ~38（T1 定数）。

`openspec/tools/validate.py testing-run --strict`：OK（active 后复验）。

## 审查记录（2026-09-08，welang-code-review 7 条，通过；status → complete）

1. **规范符合性 ✓**：抽查对齐已批 ch20/ch21 原文——ch21 R5 默认集（`tests/` 递归 `*_test.we`；`src/*_test.we` 编译不入集 = 黄金 test-run-src-test-not-in-set）、文件内源序（test-run-source-order）与跨文件序「the implementation's, unspecified」（路径序为 design D6 披露读）、双结局词表（pass/fail，assert 与 panic 同一失败路——ch20「one failure route」）、退码三形与「the compile failure never runs a test」（本审查期真二进制复现：E1304 → exit 2 零测试跑）、filter 空匹配空跑 exit 0（test-run-filter-empty）；ch21 R7 JSON 事件字段集逐字（本审查期 python set 断言：test-result 五字段 / test-summary 五字段、summary 无 error 计数、`--json` 不改退码——双跑皆 exit 1）；ch20 虚拟钟四面（两读同刻 test-clock-now-stable / 钟只在 advanceTime 与登记等待处走 test-clock-now-tick / scope timeout 由 advance 跨过 test-clock-scope-timeout-virtual / io 不虚拟化真调——mock println 黄金钉拦截是隔离之路）、mock 三面（pub-only 跨模块 test-mock-cross-module / force 止于己块 test-mock-scope-ends / 不拦他名 test-mock-not-others）、panic 边界败而非死续跑（test-run-fail-continues）、E1806 面零触碰（M10a 既有）；确定性调度以单线程 FIFO 运行时构造兑现（黄金字节级跨跑可复跑）。无规范外接受/拒绝行为。
2. **验证诚实性 ✓**：T1–T8 逐条复核，本审查期实跑——`go test ./internal/conformance/` ok 37.3s（磁盘黄金 `ls testdata/cases | wc -l` = 588 对账）、`validate.py testing-run --strict` OK、真二进制四面抽复现（混跑 exit 1 报告行+缩进原因行 / `--json` 三事件字段集与计数 python 断言 / 编译败 E1304 exit 2 人形 stderr 与 JSON 事件双形 / 上述三面 exit 与 T7 记档逐字同）；docs_sync 两态 239/31 相等（T8 判明记档）；T8 clean-cache 九包全绿为本会话实跑。无「勾了没跑」项。
3. **测试先行证据 ✓**：T1 45 枚黄金先红（红名单 44 + 锁定绿 1，红因三分类记档于 T1 完成记录）；T2 单测编译期红（`undefined: ProgModule`/`undefined: discoverTests`/std.time E1302 逐符号记档）先于实现；T4 runtime C 与实现同任务（M9b 惯例）。**披露**：本仓库单提交、变更全量未提交（提交候 T11 用户明示）——先行证据以 tasks.md 红名单记录 + 黄金文件本身为载，无提交历史可佐，审查认定其充分（红因分类含实现不可能产出的边界行原文）。
4. **诊断协议稳定 ✓**：零新诊断码（E1304/E1803/E1401/E0501 等全为在册复用）；`diagnostics.toml` 零触碰、`internal/diag` 零修改（`git status` 证）；`--json` 诊断事件字段零触碰（本审查期编译败探针 JSON 形字段集含 help 与 registry 逐字）；新边界词 bndGenericFns/bndFnBody 为 exit-70 词非协议面（M9a 先例）；test-result/test-summary 是 ch21 R7 已批 schema 的原样落地面非新增。
5. **单一权威 ✓**：纯实现路径零规范增量（无 specs/ 目录、docs/spec 零触碰——`git status` 证）；实现自有面（人类报告行、duration_ms = 虚拟钟差、同刻 FIFO、跨文件路径序、abort→1）全部以 design 决议 + tasks 披露呈现，非遗留事实；channel mock 张力与 B1 优化面随 T10 归档登记 follow-up；新增代码注释全英文（test.go/cli.go/build.go/test.c/sched.c 逐文件过目）。
6. **红线复核 ✓**：refr/ 零命中（.gitignore + 钩子在册）；零提交发生（T11 候用户明示，提交信息红线随 T11 执行）；diff 触界 = 22 跟踪文件（改 20 + 删 1[m10a_test.go]）+ 未跟踪新增（m10b 单测 ×3/test.c/黄金 45/变更目录）。**发现 F1（当场处置）**：涉触码盘点漏行——typecheck.go（std.time 装载 + CheckTestRoot）与 runtime 载体（sched.h/startup.c/runtime.go + 单测编译集）、cli 面拆行（cli.go/build.go/check.go 各自的落点）未列；按 M10a F1 先例补行三则于盘点（行尾标注），本记录披露。**发现 F2（笔误披露）**：tasks T9 行文误写 welang-change-review——七条实现审查技能是 welang-code-review（change-review 十条是 ready→active 关卡，变更创建时已过）；按 M10a 同款执行，T9 行已改并披露。
7. **最小可信验证已跑 ✓**：T8 全阶梯（clean-cache 九包 / gofmt / vet / validate --all --strict / docs_sync 两态 / `git diff --check` 干净）+ 本审查期复验（conformance 全量、validate strict、真二进制四面、黄金磁盘计数对账）。

**结论**：通过（发现 F1 补行、F2 技能名勘误，均已当场处置），status → complete，进入归档流程。

`openspec/tools/validate.py testing-run --strict`：OK（complete 后复验）。
