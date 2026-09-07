# 提案 — concurrency-run（M9b）

## Why

roadmap M9b 行承诺 ch18 运行塔：单线程协作调度器、共享状态与 channel 原语运行时、真实时钟 timeout（虚拟钟随 M10 落地）。M9a（2026-09-07 归档）已落 ch18 全部静态面——装载、内建类型面、Shareable、task 块与捕获纪律、scope/句柄纪律、select 定型，conformance 475 全绿——但一切运行动态语义停在 M8 体边界：`we build` 对含并发形的程序停在 bndMainBody/bndOtherFns exit 70，M9a design D10 不可达清单第 1–5 条（调度与交错、六族原语运行时、timeout 真实时钟、TaskPanic 边界捕获、select 运行时取臂）全部不可观测。本变更把 ch18 的运行语义落成真机可验证的行为，**纯实现零规范增量**（ch18 已批全量，调度承诺、原语语义、timeout 值面、TaskPanic 边界、select 取臂全部有 Requirement 承载）。

## 裁决记录（candidate 阶段三项表面裁决，2026-09-07，均采纳推荐）

- **Q1 上下文切换载体 = ucontext**：POSIX makecontext/swapcontext，零汇编、Linux/WSL2 直接可用；参考实现阶段切换成本可忽略（真性能关切是 M15 benchmarks 的事，届时换汇编是显式工具层变更）。被拒面：平台汇编（x86-64 手写最快，但 M9b 即背上平台矩阵——参考实现期不值）。
- **Q2 codegen 发射面 = 双层直线集**：main 体与 task 体共享一台直线发射器——M8 直线集上扩：标量 let/var 与算术比较、并发原语构造与方法调用（回调=内联闭包）、task/scope/select、`?` 传播、Option/Result 双臂 match、while/if 最小控制流、defer 直线倒排；**普通 fn 声明维持 bndOtherFns 不发射**（M9b 只发 ch18 语义所需的最小充分面，M6 拆分先例的同等纪律）。被拒面：通用函数体发射器（fn/闭包/全控制流全量——M9b 吞并 M5 以来全部 codegen 债务，与调度器运行时叠加面失控）。
- **Q3 阻塞实现 = 登记-挂起-唤醒真 park**：每原语带等待队列、select 登记多源、调度器空闲时睡到最近 deadline（单线程协作下空闲时唯一未来事件是超时——M9 无非阻塞 IO，唤醒只来自跑着的任务或到期）。被拒面：轮询+yield（try 形轮询退避——实现最简但就绪队列永非空、CPU 空转，靠调度承诺「eventually」遮羞；且给 M10 虚拟钟确定性调度埋雷）。

## 现状与差距

- **codegen**（internal/codegen/codegen.go，M8 形）：单 main 直线集发射器——let String 字面量/gc 构造、io 调用、单 Ok|Err 尾；sum 值零表示（Err 是报告行常量非值）；无控制流、无 fn 体、无闭包、无标量算术。M9a 的 D9 已把全部并发形钉在既有 default 停点（codegen/m9a_test.go 十形）。
- **运行时**（runtime/c/）：startup.c（22 行，main→__we_main 直调）、gc.c（212 行，全局单 root 栈 + alloc 入口 STW 收集）、io.c（21 行）。无调度器、无原语、无时钟。
- **GC 交互缺口**：roots 是全局单数组（gc.c:42-43）——单任务假设；多任务后阻塞任务的根不可达。ADR-0003 演进门明文「M9's concurrency work opens the generational + write-barrier versus concurrent-marking decision gate」——但**单线程协作下该门不打开**（任意时刻恰一任务在跑，alloc 触发的收集天然 STW，无并发标记需求）；真正必须落的是 root 集合的任务化。
- **可复用机器**：构造协议（alloc→map store→root push→字段 store，嵌套安全）；描述符位图（编译器侧引用位布局）；IR 重逃逸规则（大写十六进制）；clang 三步序与钉版门（build.go）；运行时 go:embed 分发（runtime/runtime.go）；C harness 测试模式（runtime/gc_test.go 先例）；conformance run 断言面（M4 hello/err 先例）。
- **M9a 黄金基线**：475 枚中 6 枚 build-bnd-conc-*（check 净 + build 停 bndMainBody）将随本变更翻绿——build/run 断言改写（随批更新，逐枚披露）；其余 469 枚零触碰预期（无并发形）。

## 目标与非目标

**目标**：

1. 调度器（runtime/c/sched.c）：ucontext 任务切换、task 0 = main 体（C main 起 trampoline）、就绪队列 FIFO、scope 帧栈（join/cancel+join 双形）、协作取消信号传播、空闲睡到最近 deadline。
2. 等待机器与六族原语（runtime/c/conc.c）：通用等待队列节点；Mutex/RwLock/Atomic/AtomicRef（单线程非重入推论下的直访实现，互斥由调度承载）、Cond（wait 谓词循环/spurious 不可见/signal/broadcast）、Semaphore（acquire 阻塞/tryAcquire/release-past-count panic/非常量非正 panic）、Channel（环形缓冲 send/receive/trySend/tryReceive/close 语义/send-after-close panic/容量 0 rendezvous）、TaskHandle（await/cancel 一次性）、CancelSignal（isCancelled/awaitCancelled）。
3. timeout 真实时钟：CLOCK_MONOTONIC 毫秒预算、到期标记未完任务信号、协作等待返回、`Err(TimedOut)` 值构造；`scope timeout` 值面 Ok|Err 双臂可 match。
4. TaskPanic 边界：任务体 panic 转任务结果槽（defer 倒排先跑）、await 返回 `Err(Panicked(msg))`、取消未 await 的 panic 丢弃、main 内 panic 维持进程 abort。
5. select 运行时：登记-挂起-取臂-撤销，四等待源全通，「unspecified-but-safe」取臂。
6. GC root 任务化：per-task root 栈、collect 遍历全部任务、原语对象与捕获环境块走 gc 分配（零位图/编译器位图）。
7. codegen 双层直线集：值表示（标量 i64、sum 双槽 {tag, payload}）、发射面清单见 design D8（标量算术比较/原语构造与调用/内联回调闭包/task 创建/scope enter-leave/select/`?`/Option-Result 双臂 match/while/if/defer）、运行时 ABI 函数族、边界词汇更新（bndMainBody v2 + bndTaskBody 新行）。
8. conformance：M9a 六枚 build-bnd 黄金翻绿 + run 族新黄金（build+run 双断言：调度交错、原语全链、timeout 双向、TaskPanic、select、close 语义、rendezvous）+ 单测先行全周期（红→绿证据、C harness、真机电池）。

**非目标**：

- 虚拟钟、`advanceTime`、测试模式确定性调度（**M10**——bndTaskTime 维持）。
- 通用函数体发射（fn 声明/闭包传值/元组/for/String 运算/match 一般形——bndOtherFns 与新边界词汇承载，M9b 只发 ch18 最小充分面）。
- `transaction`/`WeakRef`/内存区域（各自「await their own chapters」）；vet 层巡检（tooling 章）。
- 多线程并行与并发 GC（分代/写屏障/并发标记——ADR-0003 演进门在真并行时才打开；单线程协作下不触发）。
- 规范增量与诊断注册表改动（零——运行语义全部由 ch18 已批 Requirement 承载）。

## What Changes

无规范增量：ch18 已批全量，本变更全部行为都是已批面的落实现。变更交付：runtime/c/sched.c（调度器）+ runtime/c/conc.c（等待机器与六族原语）+ gc.c root 任务化改造 + codegen 双层直线集发射器（M8 直线集的扩面重写）+ build 链接序扩（rt-sched.o/rt-conc.o）+ conformance 黄金（六枚翻绿 + run 族新增）。被替换的边界：bndMainBody 词汇 v2（并发面入集）、新增 bndTaskBody（task 体专属诚实边界）。

## 影响层

| 层 | 触及 |
| --- | --- |
| compiler | codegen 发射器扩面重写、build 链接序、conformance 黄金 |
| stdlib | 运行时原语族（runtime/c/ 的 C 实现——std.concurrent 内建面的运行载体） |

（纯实现——「无规范增量」豁免路径，规范层零触及。）

## 影响范围

| 层 | 文件 | 动作 |
| --- | --- | --- |
| compiler | `internal/codegen/codegen.go` | 双层直线集发射器（值表示/标量面/原语调用/闭包/task/scope/select/?/match/while/if/defer） |
| compiler | `internal/codegen/m8_test.go`/`m9a_test.go` | 既有 IR 钉回归 + 边界词汇更新（随批披露） |
| stdlib | `runtime/c/sched.c`、`runtime/c/conc.c` | 新文件：调度器 + 等待机器与六族原语 |
| stdlib | `runtime/c/gc.c` | root 栈任务化、collect 遍历、原语分配协议 |
| stdlib | `runtime/runtime.go` | embed 新源 |
| compiler | `internal/cli/build.go` | clang 序扩（rt-sched/rt-conc 编译与链接） |
| tests | `runtime/sched_test.go`/`conc_test.go`（C harness）、`internal/codegen` 单测、conformance 黄金（六枚翻绿 + ~30 新增） | 测试先行 |

## 涉触码盘点

- codegen.go：发射器主体重写（保持纯函数/常量池/构造协议既有机器）；declareLines 扩运行时 ABI 族；边界词汇三处（bndMainBody 文案、bndTaskBody 新增、bndOtherFns 维持）。
- gc.c：roots 全局数组 → task 结构内嵌（`__we_root_push/pop` 定位当前任务）；`__we_gc_collect` 遍历全部任务 root 栈；`__we_alloc` 协议零变化（safe point 仍是构造入口）。
- build.go:179-206：写出发与 clang 序加 rt-sched.c/rt-conc.c 两源（编译序在 rt-gc 前——conc 依赖 sched 的任务结构？同层，序无依赖，按字母或依赖序定）。
- startup.c：main 起 trampoline（task 0 化）——`__we_main` 不再直调而入任务上下文（main 体从此可阻塞 join）。
- 既有测试更新面（随批，逐处披露）：codegen/m9a_test.go 十形的 What 断言（bndMainBody 词汇 v2 后文案变）、六枚 build-bnd-conc 黄金（翻绿+run 断言）；conformance 其余 469 枚零触碰预期；runtime/gc_test.go 回归（root 栈任务化后的既有探针）。
- 新增：runtime C harness 两套、codegen 单测族、conformance run 族黄金、真机电池。

## 已知风险与开放问题

- **GC 与调度的交互是本变更第一风险**：per-task root 栈改造动的是 M8 已验证的收集器核心——既有 GC 存活探针（4 MiB 风暴）必须在改造后零损重证；阻塞任务的根可达性要有专项 harness（任务持 gc 值跨 park 存活）。
- **ucontext 的 makecontext 传参限制**（int 传递、glibc 弃用警告）：trampoline 经全局/任务槽取参，警告面用编译旗标或 casts 消（真机验证钉死）。
- **sum 值表示是新机器**：Option/Result 双臂 match 引入 {tag, payload} 双槽——与 M8「Err=报告行常量」的既有路径并存（main 尾 Err 维持常量形），边界要清晰。
- **回调闭包的捕获环境块**：update/read 的闭包捕获 gc 引用（跨绑定访问）需要环境块 gc 分配+编译器位图——与 task 捕获块同一机器，但生命周期不同（回调即用即弃 vs 任务全程）。root 协议（回调执行期间环境块 rooted）要钉死。
- **开放问题（design 收敛）**：(a) `?` 在 main 体内的发射形（早退=main 的 Err 尾还是 scope 出口语义）；(b) 容量 0 rendezvous 的唤醒配对细节；(c) timeout 到期时正在运行的任务（不中断，标记后等协作返回——spec 已定，运行时落形）；(d) 原语对象内部等待节点的 malloc 生命周期（无 finalizer 的泄漏面）。

## 审计记录

**2026-09-07 审计（candidate → ready，welang-spec-impact-audit 七点）：通过。**

1. **问题真实性**：✅ 可验证工程缺口——ch18 运行动态语义全部不可观测（`we build` 停 bndMainBody/bndOtherFns exit 70），引用 roadmap M9b 行（docs/roadmap/0000-reference-implementation.md:26）、M9a design D10 不可达清单第 1–5 条（openspec/changes/archive/2026-09-07-concurrency-check/design.md）、以及 ch18 已批承载面：The shared-state types（1800-concurrency.md:3）、The Cond type（:51）、The Semaphore type（:75）、Task blocks（:99）、Scope blocks（:229）、Panics at the task boundary（:273）、The Channel type（:292）、The select expression（:345）、Scheduling promises（:374）。
2. **影响层声明**：✅ change.yaml layers [compiler, stdlib] 与影响层表一致；纯实现「无规范增量」豁免路径在 What Changes 显式声明，规范层零触及（M8/M9a 先例）。
3. **规范增量范围**：✅ 零新增/零修改/零删除 Requirement；零诊断码触碰（E1616 运行时陷阱半边、Semaphore release 超额 panic、send-after-close panic 均为 ch18 既有 Requirement 的落实现，非新码；死锁 abort 为诚实边界非规范表面——roadmap 未实现边界先例）。
4. **原则一致性**：✅ 无原则突破。调度「unspecified-but-safe」（ch18:374）在实现侧取 case 源序属实现选择（D13 披露非规范承诺）；P4 不受影响（确定性精化归 ch20/M10 是既定 carve-out）。ucontext 为实现载体非机制承诺，无需新 ADR（GC 侧演进门已由 ADR-0003 承载）。
5. **参考基线**：✅ 全部仓内权威（docs/spec/1800-concurrency.md、M9a 归档工件、ADR-0003、roadmap）；refr/ 未引用。
6. **验收边界**：✅ 目标 1–8 逐条机械可判（含 conformance 实际计数记档）；非目标五条显式排除（虚拟钟 M10、通用 fn 发射、transaction/WeakRef/region、多线程与并发 GC、规范增量）。
7. **粒度**：✅ 单一垂直单元（运行塔），与 M9a 检查塔的拆分裁决（2026-09-07）一致，一里程碑一变更。

## 审查记录

**2026-09-07 审查（ready → active，welang-change-review 十点）：通过（1 发现当场处置）。**

1–4 职责边界：proposal 黑盒面成立（涉触码盘点允许实现细节，M9a 先例）；零 spec 增量故 BCP 14 不适用；design 三裁决各带被拒面、引用精确；tasks 无 deferred/未决混入。5–10 内容质量：场景覆盖含 degradation（死锁/TaskPanic/close 排空）；无空章节；T1/T2 先红在前；负向断言真实（六枚翻绿+死锁+超额 panic+send-after-close+三边界词 What 钉）；完成度闭环三要素齐（检查层 M9a 已落+codegen D8+运行时 D1–D7）；proposal 开放问题 (a)–(d) 已全部在 design 收敛（→D8 emitQuestion/D3 rendezvous 快路径/D4 到期标记不中断/D7 泄漏披露），无悬空。

**发现 F1（当场处置）**：design D2 等待节点仅含同队列 `next`，任务账本链 `task->links` 的挂接指针未定义——select 多源撤销路径有实现歧义。处置：we_wait_link 增 `task_next` 字段（账本链显式双链），D2 已改。

## 审查记录（2026-09-07，welang-code-review 7 条，通过；status → complete）

1. **规范符合性 ✓**：抽查对齐已批 ch18 原文——取消任务的 panic 丢弃（:275「captured at the task boundary and discarded」= sched.c task_done 的 cancel_flag 守卫，T8 修正）、E1607 fallthrough 路径须显式 discharge（sem-blocking 黄金源按规范校正而非迁就实现）、match 臂换行分隔无分隔符 token（8 枚黄金源的 `,` 为 T1 笔误，E0105 拒收正确）、Semaphore 超额与 send-after-close 的运行时陷阱报文逐字、死锁 abort 为诚实边界非规范表面（ch18「eventual」只对可前进等待成立）。实现期全部缺陷修复（T8 七处 codegen + cancel-discard + 各任务记录所列）逐条披露于 tasks.md，无规范外接受/拒绝行为。
2. **验证诚实性 ✓**：T1–T10 逐条复核——T8 conformance 508/508、T9 电池 8/8、T10 三命令（go test 9 包绿 / validate --all --strict OK / docs_sync 31 对齐零触碰）；T5 记录失实（「38 红」实为 52 红）由 T10 成文修正而非掩盖；本审查期 gofmt -l 揪出 codegen.go 未格式化 → 当场 gofmt -w + build 复验；各任务记录携带的证据（红名单/对账数/黑盒探针）与代码现状一致，无「勾了没跑」项。
3. **测试先行证据 ✓**：T1 33 枚新黄金 + 5 枚翻绿先红（红因分类在案：全部 bndMainBody 边界 exit 70，codegen 发射面缺失）；T2 三套件编译期红（undefined: SchedSource/ConcSource/bndTaskBody/bndCallbackBody）先于实现；T3–T8 以红名单翻绿为验收，T2 harness 预钉错误四处随实现校正披露。
4. **诊断协议稳定 ✓**：零新诊断码（E1616 运行时陷阱半边、两类 panic 均为既有面落实现）；--json 字段零触碰；diagnostics.toml 零触碰；边界 What 词汇 v2（bndMainBody 改写 + bndTaskBody/bndCallbackBody 新增）随 T7 披露，四枚边界黄金同步、五枚改名一枚维持（D13-5 口径）。
5. **单一权威 ✓**：长期事实归位——执行状态进 roadmap（M9b 行随归档翻 done）；发射面 ABI 定形事实回填 design（本审查 F1 处置）；实现期决议与缺陷披露随 tasks.md 归档可查；代码注释全英文；无滞留变更目录的长期事实。
6. **红线复核 ✓**：refr/ 零命中（.gitignore + status 干净）；提交（T12）信息英文、无署名 trailer，等待用户明示；diff 触界盘点——modified 15（build.go/codegen.go/codegen_test.go/m8_test.go/m9a_test.go/4 枚边界黄金/gc.c/io.c/startup.c/runtime.go）+ 删除 5 枚 build-bnd-conc-* + 新增（m9b_test.go/33 枚 run-conc 黄金/sched.h/sched.c/conc.c/sched_test.go/conc_test.go），与「涉触码盘点」逐项对齐，无越界改动（既有测试更新面均 T6/T7 记录披露）。
7. **最小可信验证已跑 ✓**：T10 全量（go test ./... 9 包 + validate --all --strict + docs_sync 31 对）+ 本审查期复验（gofmt 清、go vet 清、go build ok）；受影响 conformance 黄金 508 全量绿；真机电池 8 项（调度交错真机观测/GC 风暴/timeout 双向真时钟/死锁 exit 70）。

**F1（审查发现与处置）**：T2 完成记录预告的 ABI 定形回填（「以上入 D2/D4/D5/D6 回填清单（T10 披露）」）未做——design 仍缺五枚发射面 ABI 事实。处置：发现即修——design 补「ABI（T2 定形，T11 回填）」五处：D3 TaskHandle（`__we_handle_await` tag 0/1 + payload 单出参槽双职责）、D3 Channel（`__we_chan_send/recv` 0/1 与 1/0 形）、D4（`__we_scope_enter(deadline_ms, collect_all)` 两参形 + leave 返回 TimedOut 位）、D5（select 五 API——new/add_recv/add_await/park/value，运行时另载 add_send/add_sig 发射面外）、D6（`__we_task_fail(msg)` 统一入口）；T2 预告的落位「D2」为笔误——按机制归属落 D3/D4/D5/D6（D2 通用等待协议无发射面 ABI），回填时一并披露。

**结论**：通过，status → complete。
