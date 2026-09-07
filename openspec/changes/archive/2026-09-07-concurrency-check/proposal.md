# 提案 — concurrency-check（M9a）

## Why

roadmap M9 承诺 ch18（tasks、channels、调度器、虚拟钟）。三项表面裁决（2026-09-07）把 M9 切成两塔：**M9a 检查塔（本变更）**落 ch18 全部静态面，**M9b 运行塔**落调度器与原语运行时与 codegen 发射（GC 任务局部根栈随之，ADR-0003 演进门）。ch18 已批全量（17 Requirement / 68 Scenario），E1601–E1618 已在 `docs/spec/diagnostics.toml` 在册——本变更与 M8 同为**纯实现**，规范增量零。

代码里四组预留边界自 M5/M6a/M6b 起等待归位，全部可黑盒验证：

1. **`bndConc`/`bndConcScope`**（parser，四处 bnd 位）：`task`/`select` 的语句形与表达式形停在「chapter 18 (concurrency) forms」；`scope` 的复合形（裸 `scope {`、`scope timeout(...)`、`scope collectAll`）停在「chapter 18 (scope) forms」——只有 `scope resource` 产生式在（M6b 落）。
2. **`bndTaskTime`**（typecheck，两处）：`currentCancelSignal` 的名值位与调用位今日停「task-scope and time-control functions (chapters 18 and 20)」。
3. **`bndShareable`**（typecheck，三处）：`Shareable` 在类型位停「Shareable markers (chapter 18)」、bound 位今日报 E0829（在「已知 prelude 名非接口」名单里空转）、`derives Shareable` 已被 E0824 钉住（「it is a where bound, not a clause target」）。
4. **std.concurrent 装载**：`import std.concurrent` 今日 E1302 std 形（未知模块）——M8 的 `StdModule` 注册表就绪待扩。

规范明文排除的面不在本变更：`transaction`/`WeakRef`/内存区域「await their own chapters」（ch18:551 Pending later changes）；vet 层跨函数嵌套访问启发式巡检属 tooling 章（ch18:39 边界自述）。

## 裁决记录（candidate 阶段三项表面裁决，2026-09-07，均采纳推荐）

- **Q1 M9 体量切分 = M9a 检查塔 + M9b 运行塔**（M6a/M6b 先例同构）：静态面（3 个关键字形 + 捕获纪律 + 句柄纪律 + select 定型 + 11 个内建类型 + 18 码）与动态面（调度器 + 六族原语 + Channel + 超时钟 + TaskPanic 边界 + codegen 发射）各自独立走全生命周期门。被拒面：单变更全量（tasks 预计 T1–T20+，工件过大、审查面过宽、中途上下文耗尽风险高）；三拆（M9c 虚拟钟单独成变更偏薄——其唯一可观测精化在 ch20 测试块内）。
- **Q2 调度器形态 = 单线程协作**（M9b 落地，本变更记路由）：规范调度承诺只要求「每任务最终运行」+「交错不指定」（ch18:376）——串行/协作谱系天然合法；真并行迫使 GC 立即线程安全化而规范无任何需要并行性的承诺。被拒面：OS 线程并行（成本前置、收益无规范面、ADR-0003 分代/并发门被迫现在打开）。
- **Q3 虚拟钟归属 = M9b 只做真实时钟，虚拟钟归 M10**：规范唯一的虚拟钟精化「测试模式确定性」是 ch20 的修正案（ch18:376 测试段），`advanceTime` 只在测试块内可观测——`advanceTime` 的 bndTaskTime 边界**本变更维持不动**（M10 归位），`currentCancelSignal` 归位。roadmap M9 行相应改写为 M9a/M9b 两行（M6 拆分先例）。

## 现状与差距

- **parser**：`task`/`select` 双位 bndConc（语句入口 parser.go:1651、表达式入口 :2927）；`scope` 非 resource 形 bndConcScope（:1985）。关键字 `task`/`select`/`case`/`timeout`/`collectAll`/`scope` 已在 ch1 保留闭集（lex.go:46-48）——零 lexer 改动。关键字先导表达式形有先例（`if`/`match`，parseIf/parseMatch）。
- **typecheck**：Shareable 三处 bnd/E0829 空转（:3164/:3489/:5748）；currentCancelSignal 两处 bndTaskTime（:5401/:5874）。
- **装载**：`StdModule(key)` 注册表（M8）现只挂 std.io——std.concurrent 待注册；`StdModuleNotFound` 报文路径已通用。
- **可复用检查机器**（全部既有已批面）：资源纪律的结构化活跃性路径 walk（M6b resource.go——ch18:231 明文「chapter 13's single-trigger path analysis applied to handles」）；闭包捕获/自由变量面（ch12）；E1401/E1402 效果机器与闭包推断（M7）；泛型 bound 机器（M6a）；内建泛型名义类型先例（List/Map/Set 的 sumInfo + collectionMembers）；prelude 内建名先例（panic/todo/assert 族）；组合子实参位效果织入（M8 methodCall）。
- **运行/codegen**：全静态面落成后动态语义仍不可观测——`we build` 对一切含 task/scope/select 的程序停既有边界行（bndMainBody/bndOtherFns），诚实边界由 M9b 收口。

## 目标与非目标

**目标**：

1. parser 三关键字形全落：`task effect tag... block`（表达式形）、`scope [timeout(expr)] [collectAll] block`（表达式形）、`select { case name = source => body ... }`（表达式形）——含 E0105 家族的错误报文位。
2. std.concurrent 装载与检查面：`StdModule` 注册（四命名 sum `TimeoutError`/`TaskPanic`/`SendResult`/`ReceiveResult<T>` 真声明 + `channel(n)` fn 声明）；11 个内建类型（`Mutex`/`RwLock`/`Atomic`/`AtomicRef`/`Cond`/`Semaphore`/`Channel`/`SendOnly`/`ReceiveOnly`/`TaskHandle`/`CancelSignal`）的检查面——泛型参数、gc 品类、构造形、封闭成员集（update/get/set/read/wait/signal/broadcast/acquire/tryAcquire/release/currentCount/await/cancel/send/receive/close/trySend/tryReceive/toSendOnly/toReceiveOnly/isCancelled/awaitCancelled）。
3. `Shareable` 标记：闭集机械计算（基础类型/unit/全 Shareable 字段 value 记录/全 Shareable 载荷 value sum/Shareable 元组/同步 gc 类型），bound 位真分派，E1606 手写 impl 拒绝，prelude 可见（无 import 可写 bound）。
4. 任务块检查：E1601 段必需、E1401 任务自身 extent 内调用子集检查、E1618 词法 scope 要求、体值资源型 E1106、函数体语境扩展（return 携值/E0401 语境）。
5. 任务捕获纪律：E1602（非同步 gc 状态，含 value 形状内部）、E1603（var）、E1604（mut self 资源）、E1605（无 Shareable bound 的泛型位）。
6. scope 块与句柄纪律：四形（plain/timeout/collectAll/timeout+collectAll）的值语义定型（timeout 形 `Result<T, TimeoutError>`）、E1607 句柄路径分析（每路径恰一次 await/cancel，早出口 discharge）、timeout 子句 Int64（E0501）。
7. select 定型：四等待源闭集（E1609）、臂型一致（E1610）、case 模式限绑定/通配（E1611）、≥2 case（E1612）。
8. 杂项码：E1613 同绑定嵌套访问、E1614/E1615 Atomic/AtomicRef 实参、E1616 Semaphore 常量计数、E1617 channel 构造无期望型。
9. conformance 黄金新增 + 单测先行全周期（测试先行、红→绿证据、真机电池）。

**非目标**：

- 调度器、六族原语与 Channel 的运行时行为、TaskPanic/timeout 的运行时语义、codegen 对并发形的发射（**M9b**）。
- 虚拟钟、`advanceTime`、测试模式确定性调度（**M10**——`advanceTime` 边界本变更维持）。
- `transaction` 多锁原子复合、`WeakRef`、内存区域（各自「await their own chapters」）。
- vet 层跨函数嵌套访问启发式巡检（tooling 章）。
- 规范增量与诊断注册表改动（零——18 码全部在册纯落实现）。

## What Changes

无规范增量：ch18 已批全量、E1601–E1618 已在注册表，本变更全部行为都是已批面的落实现。变更交付：parser 产生式与 AST 节点（Task/Scope/Select 表达式）、std.concurrent StdModule 注册与内建类型成员集、Shareable 计算标记与 bound 分派、任务捕获纪律层、句柄纪律路径分析、select 定型、杂项码接线、conformance 黄金与单测。被替换的边界：bndConc/bndConcScope 全删（parser 四处）、bndShareable 三处、bndTaskTime 的 currentCancelSignal 半边（advanceTime 半边维持）。

## 影响层

| 层 | 触及 |
| --- | --- |
| compiler | parser 产生式、typecheck 装载/类型/纪律层、conformance 黄金 |
| stdlib | std.concurrent 内建模块注册（M8 StdModule 机制扩容） |

（纯实现——「无规范增量」豁免路径，规范层零触及。）

## 影响范围

| 层 | 文件 | 动作 |
| --- | --- | --- |
| compiler | `internal/ast/ast.go` | Task/Scope/Select 表达式节点 + SelectCase |
| compiler | `internal/parser/parser.go` | 四处 bnd 位删 + 三产生式 + E0105 家族报文位 |
| compiler | `internal/typecheck/typecheck.go` | Shareable/currentCancelSignal 边界归位、内建并发类型注册、任务/scope/select 定型、杂项码 |
| compiler | `internal/typecheck/resource.go`（纪律层单文件扩容） | 捕获纪律 walk、句柄纪律路径分析 |
| stdlib | `internal/typecheck`（StdModule 注册表处） | std.concurrent 合成声明（四 sum + channel fn）+ 内建类型面 |
| tests | `internal/parser`/`internal/typecheck` 单测、conformance 黄金（预计 ~60 枚新增） | 测试先行 |

## 涉触码盘点

- parser.go 四处 bnd 位（:1651/:1985/:2927 语句与表达式入口 + scope 复合位）删除换真产生式；`case`/`timeout`/`collectAll` 从「fit no statement production」报文的意外 token 清单移入产生式分派。
- typecheck.go：bndShareable 三处、bndTaskTime 的 currentCancelSignal 位归位；:3164 bound 名单的 Shareable 行改真分派；内建并发类型检查面（构造/成员/bound）。
- 既有测试更新面（随批更新，逐处披露）：parser_test.go:201-203/:711 族（bndConc/bndConcScope 行为锁）、m6b_test.go:76-81（scope 复合形边界锁）、typecheck_test.go:311-313（Shareable/bndTaskTime 行为锁）——全部从「停边界」改钉新真实行为。conformance 既有 431 枚零触碰预期（无黄金锁这些边界——已核）。
- 新增：conformance 黄金（18 码 × 正负例 + 绿面族）、单测（parser 产生式/typecheck 纪律）、真机电池。

## 已知风险与开放问题

- **句柄纪律 ≠ 资源纪律的形状差**：句柄是「消费一次」（await/cancel 二选一）而资源是「释放一次」（块出口单触发）——复用 M6b 路径 walk 时 kill 位的语义改造（早出口 discharge = plain/timeout 形取消、collectAll 形 join 后放行）是设计工作，不是机械套用。
- **捕获纪律的自由变量分析**与 ch12 闭包捕获机器的分叉点（task 块不是闭包——ch18:135 明文分治）需要在 design 里定边界。
- **select 源闭集识别**：四等待源是方法调用头（`ch.receive()`/`t.await()`/`sig.awaitCancelled()`）——定型需要识别接收者类型 × 方法名的闭集判定，接收者未定型时的次序问题。
- **TaskPanic 的检查面可达性**：`await` 返回 `Result<T, TaskPanic>`——TaskPanic 是 std.concurrent 的命名 sum，用户写 `match` 臂或注解时经 `concurrent.TaskPanic` 合格名到达；`Result` 的 E 实参位经推断流入时 sameType 机器需认识它（既有跨模块 sum 先例可循）。
- **开放问题（design 收敛）**：内建并发类型的 IR 侧零发射（M9a）下 Construct `Mutex(0)` 在 build 停哪一行——预计 bndMainBody 族，design D 钉死逐形。

## 审计记录（2026-09-07，welang-spec-impact-audit 7 条，通过）

1. **问题真实性 ✓**：Why 引 docs/spec/1800-concurrency.md（已批 17 R/68 S）与 roadmap M9 行；四组边界全部 file:line 可验（parser.go:1651/:1985/:2927、typecheck.go:3164/:3489/:5401/:5748/:5874）——`we check` 对 `task effect io { 1 }` 程序今日 exit 70 停解析边界，黑盒可复现。
2. **影响层声明 ✓**：change.yaml `layers: [compiler, stdlib]` 与 proposal 影响层表一致；触及 compiler/stdlib 无 spec 层——「无规范增量」豁免标记在 What Changes 首句（validate.py 标记检查通过）。
3. **规范增量范围 ✓**：零增量零新码——E1601–E1618 逐枚核对在册（`[diagnostic.E16xx]` 18/18，owner 1800-concurrency）；无 active 变更的 ADDED 段可冲突（本变更外无 active）。
4. **原则一致性 ✓**：纯落实现已批面；隐式获取例外（currentCancelSignal）是 ch18:217 已批的四条件机械判据，非本变更引入；无原则突破项。
5. **参考基线固定 ✓**：全部引用 docs/spec/ 权威章节（ch18 行号引注）；零 refr/ 草案引用。
6. **验收边界 ✓**：目标 1–8 全部可机械判定（黄金/单测/边界测试翻绿）；非目标五条显式排除 M9b/M10/transaction 族/vet 层/注册表改动。
7. **粒度 ✓**：单章静态面垂直切片，检查级可观测（we check 诊断面），独立于 M9b 验收；与 M6a（四章检查面）同粒度先例。

**结论**：通过，status → ready。

## 审查记录（2026-09-07，welang-change-review 10 条，通过；F1 处置后 status → active）

1. **proposal 职责 ✓**：黑盒问题（四组 file:line 可验边界）与目标 1–8（行为面）；「可复用检查机器」段是差距论证支撑非实现路径；无任务清单混入。
2. **spec 增量 ✓**：无 specs/ 增量——纯实现豁免路径（What Changes 首句标记，validate 认可）；故 BCP 14 面不存在，规范义务由 ch18 已批原文承载。
3. **design 唯一路径 ✓**：D1–D13 每条单一路径；D2 记被拒替代（合成 record+impl 真声明——体不可书写的两难）；引注精确到 file:line 与章节条目；与「无 spec 增量」无矛盾面。
4. **tasks 职责 ✓**：T1–T12 全部来源/验证齐，无 deferred/non-goal 混入（「维持不动」项均为负向验证面）。
5. **场景覆盖 ✓**：目标面含绿面族（装载/成员/定型/纪律绿）、负例族（18 码逐枚）、边界停点（D9 逐形）——normal/boundary/failure 三路俱备（纯实现变更的映射形）。
6. **无空章节 ✓**：proposal 七段与 design D1–D13 全部承载行为内容，无凑格式包装层。
7. **测试先行 ✓**：T1 黄金先红 + T2 单测先红；T3–T9 实现任务以 T1/T2 红名单为翻绿验收。
8. **负向断言 ✓**：18 码每码黄金负例（D12 负例族 ~28）+ D10 不可达清单真机逐条实证（T10）+ advanceTime 维持半边的边界锁（T6 验证面）。
9. **完成度闭环 ✓（原则 10 映射）**：类型检查 = D2–D8 全量；代码生成 = D9 零发射 + 停点逐形钉死（新节点防静默漏过）；运行时 = D10 不可达清单显式枚举 + M9b 承接路由（两塔切分裁决的落形）——三要素各有着落，「不做」侧有实证义务非沉默。
10. **未决问题阻塞 ✓**：proposal 开放问题（并发构造在 build 停哪行）已在 design D9 钉死（bndMainBody 族逐形）；无 SHOULD/MAY/TODO 掩盖的行为未决。

**F1（发现与处置）**：proposal 影响范围表原写「纪律层，新文件或 resource.go 扩」——「或」是未定方案（第 3/4 条纪律）。处置：定死 resource.go 单文件扩容（D5 本就是其 walk 骨架改造，ch13/ch18 纪律同族归一）；design D4 已同步补记文件归属。

**结论**：通过，status → active。

## 审查记录（2026-09-07，welang-code-review 7 条，通过；status → complete）

1. **规范符合性 ✓**：抽查四路对齐已批 ch18 原文——E1607 早出口 discharge（spec :231「`?` … or by a `return`, `break`, or `continue` piercing it — discharges」= walk 的 discharge 集）、select 四等待源闭集与 E1610 臂一致（:347）、注册表分配 E1601–E1618 逐码吻合（:395，`diagnostics.toml` 无重码）、Shareable 无值面两形（:179「never a parameter or a box」→ E0821/E0820 分码，T5 已披露决议）。实现期全部「design 静默处决议」均为保守/字面读并逐条披露于 tasks.md；无规范外接受/拒绝行为。
2. **验证诚实性 ✓**：T1–T10 逐条复核——T10 当日重证 conformance 475 全绿、`go clean -testcache && go test ./...` 十包全绿、gofmt -l 空、go vet 清；各任务记录携带的运行证据（红名单、黑盒探针、对账数）与代码现状一致，无「勾了没跑」项。
3. **测试先行证据 ✓**：T1（44 黄金先红，红因分类在案）与 T2（parser 包 undefined 符号编译红 + typecheck 8 函数红）先于 T3–T9 实现；黄金 stderr 合 §52 人类可读形（`path:line:col: error[CODE]: message`）、JSON 面 §62 协议实测完好（E1607 --json 全字段 + help）。
4. **诊断协议稳定 ✓**：--json 字段零删改；新码 E1601–E1618 全局唯一（注册表 `[diagnostic.E16xx]` 18/18 在册、`uniq -d` 空）；tHelps 补齐 E1602–E1607 条目（registry remediation 逐字），Human 面零影响。
5. **单一权威 ✓**：长期事实归位——执行状态进 roadmap（M9 行拆 M9a/M9b，本变更归档提交携带）；执行策略（两塔切分、真实时钟裁决）已在 proposal 裁决记录 + roadmap；design 静默处决议随 tasks.md 归档可查；代码注释全英文（仅 lex 测试的 Unicode 注释字面量为测试数据）；无滞留变更目录的长期事实。
6. **红线复核 ✓**：`refr/` 在 .gitignore 且 status 零命中；提交（T12）信息英文无署名 trailer；diff 触界盘点——ast.go/parser.go×2/resource.go/typecheck.go×2/m6b_test.go + 新增 m9a_test.go×2/黄金 44/变更目录，与「涉触码盘点」逐项对齐，无越界改动（既有测试更新三处均在 T3/T5/T6 记录披露）。
7. **最小可信验证已跑 ✓**：按 pre-push-checks 编译器档——`go build ./...`、clean-cache `go test ./...` 全绿、受影响 conformance 黄金（475 全量）绿、诊断输出触及（E16xx 新面）已含协议面实测（--json 快照形手验）；`validate.py concurrency-check --strict` 过。

**F1（审查发现与处置）**：真机电池（T10）揭出**既有面缺陷**——finiteCtors 将零变体 sumInfo（stdConcSum 未装载回退、M5 的 list/map/set/range 单例）当「有限空构造集」，通配探针遍历零键返回假 → `match xs { _ => 0 }`（List scrutinee）与 `match r { Ok(n) => …, Err(_) => … }`（scope timeout 无 import 形）假报 E0307。处置：修复（零变体 → 非有限域，opaque 域走 default 矩阵）+ tasks.md T10 D13 披露 + E0305 穷尽性不受损实证 + 全量回归零损失；三枚既有 E0307 黄金皆用户 sum 面、零改写。

**结论**：通过，status → complete。
