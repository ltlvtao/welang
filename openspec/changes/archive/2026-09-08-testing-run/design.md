# design — testing-run（M10b）

规范基线：`docs/spec/2000-testing.md`（已批 9 R / 34 S——虚拟钟 R3 :76、advanceTime R4 :100、确定性调度 R5 :119、test 边界 R6 :138、断言面 R7 :157、不固定面 R8 :176）、`docs/spec/2100-toolchain.md` R5 `we test`（:95）、R7 JSON Lines 协议（:148——**test 事件 schema 已批**：`test-result`{file, name, status, duration_ms} + `test-summary`{total, passed, failed, duration_ms}）。引用行号以 2026-09-08 工作树为准。四项 candidate 裁决（Q1 std.time 最小面 / Q2 单二进制顺序跑 / Q3 一律槽形 / Q4 事件 schema——**起草期修正见 D6**）与 M10a 预裁两项继承（mock 载体 = 运行时按名函数表；test 边界 = 每测试一任务）为本 design 的裁决前提。

## D1 程序级发射：EmitProgram、模块限定符号、两停点更替

**现状**：`Emit(f *ast.File, module string)`（codegen.go:268）单文件单模块——pass 一只走查根文件，`bndOtherFns`（:292/:344）拦一切非 main fn 与非 std 导入、`bndTestModule`（:327）拦 test 模块、`bndTopLets`（:294）拦顶层绑定。

**程序级形**：

- 新入口 `EmitProgram(mode, modules)`：mode ∈ {build, test}；modules = 装载器后序模块列（`loadGraph` 的产物——check.go:282 根键 `"main"`），build 模式根模块的 main 是入口、test 模式见 D5。`Emit` 单文件形保留（单测/单文件路径）。
- **模块限定符号**：一切 We 级 fn 的 IR 符号 = `@<模块键>.<fn名>`（根模块键 `main`；test 模块键 = 装载器为其指派的键，tests/ 下按路径派生）。LLVM 未引号标识符字符集 `[-a-zA-Z$._0-9]+` 收点与下划线——We 模块段 `[a-z0-9]`、fn 名 camelCase 无点，`键.名` 无碰撞（fn 名不含点使拼合单射）。C 运行时符号维持 `__we_` 前缀双下划线——与 We 限定符空间不相交。
- **符号表**：pass 一从「单文件走查」扩为「全模块走查」——sum 表、record 表、fn 表（模块键+名 → 符号/槽/签名 ABI 形）按模块收集后并为一体；同序（装载后序）逐模块发射。跨模块调用查 fn 表经槽间接（D3）。
- **停点更替**：`bndOtherFns` 与 `bndTestModule` 全删（常量 + 两 case 位 + M10a 单文件 test 路由）；新增 `bndGenericFns`（FnDecl TypeParams 非空——`generic functions in code generation (monomorphization is the B-track codegen-full widening)`）与 `bndFnBody`（fn 体越 M9b 语句集——词面 D8）。`bndTopLets` 维持（顶层绑定不发射——模块 init 见下）。
- **模块初始化 = 静态槽初始化（披露）**：ch15 R5 急切后序恰一次——在无顶层绑定可发射（bndTopLets 停点在位）的 M10b 集内，模块 init 无任何可观察行为，落为**数据段静态初始化**（槽全局量带初值，D3）而非运行时 init 代码：可观察契约（恰一次、无观察者）平凡成立。顶层绑定发射归 B 轨（届时 init 需真代码，本决议随之退役）。

**被拒替代**：每模块一 `@<key>.init` 空函数 + harness 逐个调用（更贴 ch15 字面，但 M10b 集内是纯空转——为零观察行为引入调用序列，反而不诚实）；模块键哈希化符号（可读性损失，无碰撞收益）。

## D2 We 级 fn 调用 ABI（T2 定形）

跨 fn 边界的值传递族——M9b 既有 ABI 面（main 无参、task thunk env 块、回调标量直传）零触碰；本族只管 **We fn ↔ We fn** 调用位。**精确形由 T2 单测定形为契约**（M9b T2 先例——design 钉族、测试钉形）：

| 值族 | 参数位 | 返回位 |
| --- | --- | --- |
| 基标量（整族/Bool/Rune；窄整型符号扩展入 i64 域） | i64 直传 | i64 |
| Float | f64 直传 | f64 |
| String | 双字 (ptr, i64) 顺序双参数 | `{ptr, i64}` 聚合值返回 |
| gc record | ptr（构造协议产物） | ptr |
| value record / tuple | ptr（指向值快照） | sret 出参指针（首参位附加） |
| sum | 双槽 {tag, payload} 顺序双参数 | `{i64, i64}` 聚合值返回（payload i64 域） |
| unit / 无值 | 不传 | void |

- **sret 约定**：返回 value record / tuple 的 fn 在参数表首位附加 `ptr sret` 出参，返回 void——避开聚合值返回的调用约定分叉；String/sum 的双字聚合返回用 LLVM 结构体值返回（两字足够小且无变长）。
- **mock 体同 ABI**（D3）：mock 体按目标签名同族发射——拦截对调用方全透明。
- **回调内调用 We fn**：M9b callback 直线标量集不拓宽（非目标）——callback 体内调用 We fn 走同族（标量参数足够覆盖直线集）。

## D3 槽形：函数表与拦截

**Q3 一律槽形**——每个构建中模块级单态 fn 一枚全局槽：

- `@slot.<模块键>.<fn名> = global ptr @<模块键>.<fn名>`——真体发射于限定符号，槽初值指向真体（**静态初始化 = D1 模块 init 决议**）。
- **一切调用位经槽**：`%f = load ptr, ptr @slot.…` + indirect call。无直接 `call @<key>.<fn>` 形（自调用/互调用同规）。
- **入口例外（披露）**：build 模式根模块 main 是入口（`__we_main`，boot 直呼）无调用位——不设槽；test 模式项目 main 降格为普通 fn（D5）——设槽。`mock main()`（字面合法目标，M10a synthetic 同款读）在 test 编译下经槽可达。
- **std fn 条目一律入槽（审查 F1 修正）**：凡装载为 Pub FnDecl 且调用走 fn 调用面的合成条目皆入槽——`std.io` println/print（emitIoCall :615 现直呼 `__we_println`，调用位改经槽）、`std.test` assertTrue/assertFalse（typecheck.go:2285 合成声明，M10a D8「the std.io println precedent」——mock 于其字面合法，故运行面必须真拦）、`std.time` now/sleep（D4 新增）：`@slot.<std键>.<名> = global ptr @<运行时符号>`（真体 = C 运行时符号）。初稿只列 std.io 且 D7 误记 assertTrue 直呼——检查面接受而运行不拦即结构化谎言，修正为一律槽形。
- **构造面条目不入槽（F1 披露 + follow-up）**：`std.concurrent` 的 channel 是 marker FnDecl（typecheck.go:2323），定型走 expectation-driven 构造路（importCall :2469）非 fn 调用路，运行面是 primCtor（codegen.go:919 `__we_prim_new_chan`，实参含期望类型派生的元素位图/描述符——非声明签名可复述）。M10a 字面裁决钉了 mock 于 channel 检查面接受（m10a_test.go:178），但唯一可复述签名是无返回 unit 形（marker 无返回段）——对 `Channel<T>` 期待位是类型谎言，**运行面无法诚实拦截**。决议：本变更维持检查面零改（纯实现承诺），channel 不入槽（其构造调用位维持 primCtor 直呼）；张力随归档登记 follow-up（E1804 构造面重分类 vs M10a「无特判」理据——独立裁决点，非本变更 smuggle）。
- **特型面无槽**：assertEqual（非模块条目而是调用面——M10a E1304 已裁）、预导入 advanceTime/panic 族/todo/currentCancelSignal（语言级名非模块 fn）——直呼，与 M10a「mock 于 assertEqual → E1304」一致。
- **拦截 = 槽装填**：harness 于 test 开始按 mock 源序 `store ptr @<testkey>.mock.<n>, ptr @slot.<目标键>.<名>`、结束恢复真体——M10a 预裁「运行时按名函数表」的落地形：**槽集即函数表，(模块键,名) 即按名索引**；ch20 cache 形（helper 体内调用点被拦）天然成立——helper 体的调用位本就经槽。
- **mock 体发射**：每 mock 一枚 fn `@<test模块键>.mock.<序号>`，签名按目标 ABI 族（参数/返回同形，D2），体走 emitBody（fn 语境参数化——目标段/返回形）。mock 块嵌套语句集 = fn 体集（M9b 集内合法；越集停 bndFnBody）。

**被拒替代**：仅测试二进制槽形（普通构建零开销，但两种发射形 = 两套 IR 对拍面——Q3 已裁拒绝）；调用点改写式拦截（编译期替换调用目标——与「运行时函数表」预裁相悖，且同模块多 test 各异 mock 无法静态共存）。

## D4 虚拟钟运行时

新文件 `runtime/c/test.c` + `runtime/c/sched.c` 触点 + `runtime/runtime.go` embed 行。

**钟状态机**（test.c）：

- 全局：`mode`（real/virtual）、`vnow`（Int64 毫秒）、`test_task_base`（spawn 记账基线）。`__we_test_begin()`：mode→virtual、vnow=0、记当前任务计数；`__we_test_end()`：作废残留（下）、mode→real。
- **每测试复位归零**：ch20:76「the clock starts at zero」——begin 复位，跨测试不累积。
- `__we_advance(ms)`（test.c，advanceTime 的运行面）：**越刻屏障（T4 增补决议）**先行——拨钟前排空就绪队列，每个 READY 任务运行至其下一阻塞点（`__we_sched_drain`，新任务态 WE_DRAIN 的 FIFO 屏障队列；嵌套 advance 按到达序应答）——使一切想要本次跨越的等待先登记（ch20:121 场景读 = 等待已登记才谈跨越释放；否则 spawn 后未及运行的任务把 sleep 登记到已拨后的钟上、永不到期）；随后 vnow += ms；释放一切到期虚拟等待（deadline ≤ vnow），**同刻按登记序 FIFO**（ch20:119「the runtime's own」——本实现读 = 等待登记先后，披露为决议）；释放后调度循环自然接管。
- `__we_time_now()`：virtual 模式返 vnow、real 模式返 CLOCK_MONOTONIC 毫秒（std.time now 的双源）。
- `__we_time_sleep(ms)`：virtual 模式 park 到虚拟绝对时刻 vnow+ms（经既有等待机器登记、deadline 双源标记）；real 模式走 M9b 真实 deadline 路径。

**deadline 双源分派**（sched.c）：deadline 计算入口按 mode 分派——virtual 模式下 scope timeout 与 sleep 的 deadline = vnow + ms（虚拟绝对毫秒）；等待链节点增一位 `virtual` 标记。**空闲睡**：M9b 空闲循环睡到最早活跃真实 deadline——virtual 模式下若就绪空且全部 parked 皆虚拟等待 → 无真实唤醒源：若 parked 非空即**虚拟域死锁**，同款诚实 abort（消息点名 virtual clock 与 advanceTime——可运行的 task 才能拨钟，无人可跑即死锁）。真实 deadline 在场则照常睡（测试域内 time 全虚拟，混合形 = real 模式残留，仅 test 边界外存在）。

**残留作废**（`__we_test_end`，披露为决议）：ch20:138 只钉「run continues past the failed test」——结束后仍 parked 的任务（虚拟钟 sleep、channel、semaphore 一切源）ch20 未定。决议：begin 后 spawn 的一切任务，其等待链标 dead（M9b 取消机器）、任务**永久弃置**——弃置 ≠ 取消（T4 修正：取消唤醒任务、其剩余语句仍执行；弃置 = 永不再调度，新任务态 WE_ABANDONED）：READY 位者摘出就绪队列、屏障队列位者摘出屏障、PARKED 者停在原处（栈不回收——follow-up #11 有界泄漏面，同款）。**到达面（T4 定档）**：合法 We 源中 passing test 留不下未 join 任务（E1618 任务必在 scope 内 + E1607 路径分析 + scope 退出即 join）——弃置的到达形 = **失败测试**（panic 中途穿域，scope 无人 leave，其任务成弃置对象）。**作废先于下一测试 begin**——下一测试从零任务面起步。

**GC 面**：弃置任务窗口留在任务链上（M9b collect 遍历全部窗口含 PARKED）——根保持、可收集性不受扰；不释放（与 #11 一致）。

**std.time 第四枚**（typecheck）：`StdModule("time")` 注册——`now() -> Int64 effect time` / `sleep(ms: Int64) effect time` 两 Pub FnDecl（合成声明，M8 std.io 先例）；`import std.time as time` 合格到达。**装载门于模块键**（用户同名不受扰，M9a 同款）。效果段 `time` 使域外调用照常受 E1401 纪律；test 体内 E1401 抑制（M10a D3 哨兵）天然覆盖——**测试域内 time 虚拟 = 钟的分派职责，效果面零特判**（ch20:76「total for the time effect」由运行时分派兑现，不由检查器）。

## D5 合成 harness 与 test 边界

**test 编译的入口** = 合成驱动 fn（IR 名 `__we_main`，boot 位）：

1. （静态槽初始化已承模块 init——零代码，D1。）
2. 逐测试（D6 序：文件路径排序、文件内源序）：
   - 该 test 块的 mock 按源序装填槽（store mock 符号入槽）；
   - `__we_test_begin()`；
   - spawn 一任务（thunk fn = 合成的 per-test 包装：调 test fn 本体）+ await——**M10a 预裁「每测试一任务」**：Ok = pass、`Err(TaskPanic)` = fail（panic 报文 C 串为失败原因行），M9b 任务边界全套复用（报文常量、GC 窗口、完成链）；
   - `__we_test_end()`；
   - 恢复槽（store 真体回槽）；
   - `__we_test_report(file, desc, status, duration)`（test.c——计数 + 渲染，duration = begin/end 虚拟钟差）。
3. `__we_test_summary()`（test.c——渲染汇总、置进程退出码 0/1）。

- **test fn 本体**：TestDecl 发射为 `@<test模块键>.test.<序号>` fn（无参无返回）；体走 emitBody（fn 语境参数化——`(test)` valueless、test 体语句集 = fn 体集）。**per-test 包装 thunk** 承担 spawn 协议（env 块零捕获——test fn 无参）。
- **panic 直达边界**：test fn 体内 `__we_task_fail`（assert 族/panic/溢出陷阱同入口，M9b 统一路）→ 任务以 TaskPanic 终结 → 驱动判 fail → 进程续跑下一测试——ch20:138「the process continues」的落地形。**panic 报文不洗练**（失败原因行 = panic 消息原样）。
- **--filter 经 argv（修订：见 D6）**——filter 在 CLI 侧匹配后**烘进 harness 综合**：只有匹配的测试进驱动序（空匹配 = 零测试驱动、汇总报零、exit 0）。子进程不实现 regexp（C 运行时无 regex 面；ch21:110 只说 matches the pattern——pattern 语言未固定，实现读 = Go regexp，披露）。
- **--json 亦经 argv**：C main 读 argv 见 `--json` 则 `__we_test_report/summary` 走 JSON Lines 渲染（schema 已批——ch21 R7，D6）。

**被拒替代**：驱动为 C 代码（test 列表/符号/槽皆编译期知识——IR 综合最薄；C 侧需要 We 类型面知识，分层反了）；每测试一进程（隔离强——但虚拟钟/死锁/退出码面全变形，且 N 次 fork 无收益）。

## D6 we test CLI（含 Q4 起草期修正）

新文件 `internal/cli/test.go`；cli.go:43 `implemented: true` + dispatch 增 `test` 路由（cli.go:106 case 列）。

**发现**：`filepath.WalkDir("tests/")` 收 `*_test.we` → 按路径**字符串排序**（跨文件序 ch21:100 留实现——本实现读 = 排序路径序，Q2 裁决、披露）；`src/*_test.we` 不在默认集（ch21:101 场景——普通模块照项目编译，不跑）。

**并图编译**：每 test 模块为根走 `loadGraph`（导入映射 src/，既有机器复用）→ 各图并集去重（模块键一体的 fn/sum/record 表）；每根 typecheck（test 模块根**无 main 要求——E1305 不适用**；test 块检查链 M10a 全套照跑）。任一诊断 → 诊断照常（stderr / --json JSON Lines）+ **exit 2，零测试跑**（ch21:113「the compile failure never runs a test」）。

**构建执行**：EmitProgram(test 模式) → 钉版 clang 链接（build.go 三步序复用）→ 工件 `build/<name>.test`（项目名取 manifest）→ 子进程执行（stdio 接注入流，M9b run 形）。**单文件 `we test <file>.we`**：单文件编译 std.*-only（既有单文件面）+ 该文件为 test 集 → 工件走 `os.MkdirTemp` 临时目录用后删（follow-up #6 维持——命名未批，披露）；非 `_test.we` 单文件 = 零测试集空跑 exit 0（字面读：默认集 = 该文件 test 块 = 无，披露）。

**退出码**：子进程 0 → 0；子进程 1 → 1；编译失败 → 2。**子进程 abort（虚拟域死锁 D4 / 运行时诚实 abort）**：stderr 原样转达 + exit 1（ch21 定死 0/1/2；abort 是运行失败非编译失败，归 1——披露为决议）。

**报告渲染归属**：人类报告与 JSON 事件**皆由子进程渲染**（test.c `__we_test_report/summary` 双形，argv `--json` 切换），CLI 逐字转达 stdout——conformance 黄金端到端捕获子进程行为；CLI 不解析行协议。

**Q4 起草期修正（candidate 自纠，规范权威优先）**：Q4 曾框「实现自有 schema 披露非规范面」——design 期核对 ch21 R7（:148）发现 **test 事件 schema 已批**：`test-result`{file, name, status, duration_ms} 与 `test-summary`{total, passed, failed, duration_ms}，字段稳定性是章文承诺非实现承诺。Q4 修正为：**落 ch21 R7 已批 schema 原样**；实现自有面收窄为 ch21 未固定处——人类报告行格式、`duration_ms` 语义（本实现：测试域 = 虚拟钟差——确定性、域外真实毫秒）、status 取值拼写（`pass`/`fail`，ch21 两结局词汇）。被拒面维持不变（--json 仅诊断）。

## D7 断言运行面

- **assertTrue/assertFalse**：`__we_assert_true(i64 cond)` / `__we_assert_true` 取反形——cond=0 → `__we_task_fail` 定报文 `assertion failed`（assertTrue）；assertFalse 定报文 `assertion failed: expected false`。失败 = panic 一条路（ch20:157「assertions arrive by the test boundary's one route」）。
- **assert（预lude 双参形）**：`assert(cond, "msg")` → cond 检 + `__we_task_fail(msg)`——用户消息直用（msg 非串字面量形维持 M9b 边界）。
- **assertEqual 域内助手**（test.c）：`__we_assert_eq_i64(i64 got, i64 want)`（Int 族/Bool 同用）——失配 `__we_task_fail("assertion failed: got %ld, want %ld")`；`__we_assert_eq_str(ptr gp, i64 gl, ptr wp, i64 wl)`——长度或 memcmp 失配 `__we_task_fail("assertion failed: got \"%.*s\", want \"%.*s\")`。**报文实现自有**（ch20 未固定失败报文——披露）；域外（record 等）维持 M10a bndAssertEqDomain 停点。
- **emitCall 分派扩**：裸名 `assert`/`todo`（panic 族既有面旁）——直呼无槽（语言级名，D3）；`st.assertTrue/assertFalse` 经 **std 条目槽**（D3——检查面 mock 字面合法，运行面经槽真拦，F1 修正）。

## D8 边界词面与既有面对账

**新词两枚**（codegen.go 常量块）：

- `bndGenericFns = "generic functions in code generation (monomorphization is the B-track codegen-full widening)"`
- `bndFnBody = "function bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"`——与 bndMainBody/bndTaskBody 同集异锚（语句集零拓宽，B1 归属）。

**删**：`bndOtherFns`、`bndTestModule`（常量 + case 位）+ M10a 单文件 test 路由（build.go:36 `file.IsTestModule` 直达 Emit 位删——test 模块经 `we build <file>` 回落 whatSingleFileBuild 命名边界）。

**既有 conformance 黄金预期对账（T1 实测后逐枚披露）**：

| 黄金 | 现预期 | 落点 |
| --- | --- | --- |
| build-bnd-fn | exit 70 bndOtherFns | **翻绿**（valueless 空 fn 体集内 → 发射，构建净 exit 0） |
| build-bnd-conc-fn | exit 70 bndOtherFns | **翻绿**（scope/task/await 体 = M9b 集内） |
| build-bnd-multi-module | exit 70 bndOtherFns | **翻绿**（util 模块 fn 发射——跨模块程序级真行为） |
| build-bnd-test-module | exit 70 bndTestModule | **回落 whatSingleFileBuild**（exit 70 换 What） |
| build-bnd-test-fns-first | exit 70 bndOtherFns | **回落 whatSingleFileBuild**（同上） |
| build-bnd-toplet / build-bnd-body / build-bnd-m6b-main-body / build-bnd-err-payload / build-bnd-new-forms | 各自停点 | **零触碰**（bndTopLets/bndMainBody/bndErrPayload 维持） |
| check-conc-advancetime-bnd 等检查面 | M10a 形 | **零触碰**（检查面零改） |

**既有单测对账**：M10a codegen 停点单测 `TestM10aTestModuleBoundary`（两形）随路由删而**翻面/退役**（行为翻面随批披露——两形分别回落命名边界）；m8/m9 IR 字节对拍单测（println 直呼 → 槽间接）**预期更新**——`m8HelloIR` 族逐枚对账披露；M9a/M9b 钉 bndOtherFns 形的单测逐枚核对（预期：fn 体集内形翻绿、越集形换 bndFnBody What）。

## D9 确定性论证

- **M9b 单线程协作调度本确定**——同一唤醒序下运行可重现（ch18 承诺面）；唯一非确定源 = 真实时钟 deadline 到期次序。
- **测试域消灭该源**：time 效果全虚拟（ch20:76 total）——sleep/scope timeout 的 deadline 皆虚拟绝对毫秒、只被 advanceTime 推动（M9b 空闲睡不介入——无真实 deadline 在场）。
- **同刻序 = 登记序 FIFO**（D4 披露决议）：advanceTime 释放同刻等待按登记先后——同一测试的两次运行得同一序。
- **越刻屏障定登记时点**（T4 增补决议，D4）：advanceTime 先排空就绪队列再拨钟——spawn 未跑的任务先运行至阻塞点登记其等待；登记序因此 = 屏障内的运行序，同一测试两次运行同一序。
- **io/net 不虚拟**（ch20:79 如实直说）——测试内 io 是真实副作用（黄金以 println 定序输出；不依赖真实时序断言）。
- **承诺边界**（ch18 carve-out + ch20:119）：承诺**重现**非次序——确定性调度不定执行序，定「同输入同输出」；`--explore` 越此承诺（M10c）。

## D10 黄金矩阵（T1 定数，预计 ~38）

**runner 面**（ch21 R5）：单测试过（exit 0 + 报告行）；文件内源序（三块完成序）；失败续跑（一败一过——后测试仍跑、exit 1）；全败 exit 1；空 tests/（零测试 exit 0）；`--filter` 命中子集；`--filter` 空匹配（空跑 exit 0）；编译失败 exit 2 零测试跑；跨文件排序路径序（两文件）；`src/*_test.we` 不入默认集（在跑集外、不扰）；`--json` 事件双行（test-result + test-summary 字段逐字）；单文件 `we test x_test.we`（std-only）跑绿；单文件非 test 模块空跑 exit 0。**~13**

**test 边界与断言面**（ch20 R6/R7）：panic 直达边界转失败（进程续跑实证——后续测试跑）；assert(cond,msg) 失败报文；assertTrue/assertFalse 过/败各形；assertEqual Int64/Bool/String 过 + 失配报文各一；assertEqual 域外维持边界 What。**~9**

**mock 拦截面**（ch20 R3 族 + cache 形）：own-module helper 拦截（**cache 形**——helper 体内调用点被拦：mock store.read 后经 helper 的调用走 mock 体）；mock 力量止于其块（后一测试见真体）；一块双 mock；跨模块 pub fn 拦截；`io.println` 拦截（std 条目入槽的实证）；两块各 mock 同目标（块间独立）；mock 不拦他名（mock f 后块内调用未被 mock 的 helper g 走 g 真体——「a mock of `f` does not mock what `f` calls」的正钉形）。真体不可达（ch20:48「the target's own body … may never run」——被拦调用的一切可观察值皆来自 mock 体）已由 cache 形覆盖，不另立黄金。**~7**

**虚拟钟与确定性**（ch20 R3/R4/R5）：now() 两次相同（advance 前）；advanceTime 释放 sleep 任务（spawn + sleep(100) + advance(100) → 完成）；同刻多任务登记序 FIFO（两 sleep 任务释放序 = spawn 序）；scope timeout 读虚拟钟（timeout 到期经 advance 触发——fail-fast 路真行为）；std.time 域外真实钟（sleep(50) 真睡 + now() 前后差 ≥50——宽裕断言）；虚拟域死锁（sleep 无 advance → abort 转达 exit 1）；残留作废（**T4 重构：passing 形在 E1618+E1607 下不可达——黄金源改失败测试形**[panic 中途 spawn 慢任务、test 结束弃置不执行剩余体——下一测试不受扰]）。**~7**

**多函数发射面**（roadmap M10b 行首件）：fn 带参带返回调用（i64）；fn 链调用（a 调 b）；跨模块 fn 调用（test 导入 src 模块调 pub fn）；String 参数/返回 fn；record 参数 fn；valueless fn 调用；泛型 fn → bndGenericFns；fn 体越集（for 循环体）→ bndFnBody；翻绿三枚（build-bnd-fn/conc-fn/multi-module——D8 表）。**~9**

（负例锚点、报文文本一律真二进制探针捕获零手拼；files 映射控制 tests/ 目录形。）

## D11 单测面

- `internal/codegen/m10b_test.go`：槽发射（全局量 + 调用位间接形；std fn 条目槽三来源（io/test/time）各形 + channel 构造面**不**槽化断言）、fn ABI 族逐形（**T2 定形契约**——参数/返回全族矩阵）、harness 综合形（驱动序/槽装填/包装 thunk）、bndGenericFns/bndFnBody 停点、test/build 双模式入口、mock 体发射同 ABI、main 降格（test 模式槽在/build 模式无）。
- `internal/typecheck/m10b_test.go`：std.time 装载（now/sleep 签名、效果段 time、E1401 域外照查、别名到达、用户同名不受扰）。
- `internal/cli/m10b_test.go`：发现排序序、filter 正则匹配面、退出码映射（0/1/2/abort→1）。
- runtime C harness（`runtime/c` 测试惯例）：钟状态机（begin 复位/advance 前进/FIFO）、deadline 双源、残留作废、assert 助手报文。
- 既有更新面：D8 单测对账清单逐枚披露。

## D12 验证阶梯与披露义务

三阶（M9a/M9b/M10a 惯例）：①黄金/单测先红（红因分类记档——预期类：we test exit 70 边界行 / 编译期既有诊断 / 锁定绿）；②逐任务翻绿对账（conformance 总数 543 → 543+N 双向 comm、既有触碰面恰为 D8 表 + 意外即停查因）；③真机黑盒电池（真二进制：项目模式 `we test .` 全链、虚拟钟时序探针（真实墙钟对照——虚拟 sleep(10⁹) 瞬回）、mock 拦截真跑、exit 码四形、--json 逐字段、abort 转达）。收尾全量：`go clean -testcache && go test ./...`、gofmt/vet、`validate --all --strict`、docs_sync 对数不变（roadmap 双语对已在册）。

**披露义务清单**：

- design 静默处决议（本 D 编已列）：模块 init 静态化（D1）、sret 形（D2）、入口例外（D3）、同刻 FIFO 与残留作废与虚拟域死锁归 1（D4/D6）、跨文件排序路径序与 duration 语义与 status 拼写（D6）、断言报文文本（D7）——tasks 完成记录复记。
- **Q4 起草期修正**（D6）——proposal 裁决记录已同步修正，ready 报告向用户复述。
- 既有黄金随批更新：D8 表逐枚 + 意外触碰零容忍（停查因）。
- 既有单测随批更新：IR 对拍族与停点钉族逐枚披露。
- 实现若揭既有面缺陷：修复披露，不得静默绕过（M9a finiteCtors、M10a 锚位先例）。
