# design — testing-explore（M10c）

依 Q1–Q4 裁决与 ch21:126-141/R8（:174）/diagnostics.toml E1901(:1433)/E1902(:1442)/E1903(:1451) 取证落机制。所有跨文件 ABI 以 T2 单测定形为准（M9b 惯例，实现期修正回填本文件披露）。

## D1 探索塔形态：drive-thunk 提取 + 一形两模式 runner

M10b 的 emitDriver 把每测试序列内联在 `__we_main` 体里（codegen.go:3399-3450：mock 装 → begin → task_new(wrap) → handle_await → end → 恢复 → report 分支 → summary）。重跑一个测试需要重入这段序列——内联形无从重入，整 `__we_main` 重入则失每测试种子性（Q3 已拒）。

**提取形**：每测试序列原样提取为 `define internal void @<key>.drive.<n>()`（零参——所需皆全局：槽全局、wrap fn、interned 字符串；序列内部指令零改写，M10b 黄金的行为面由「序列逐指令不变」保）。`__we_main` 对每测试发射两调用，再 summary：

```llvm
call void @__we_test_meta(ptr @.str.file, i64 <flen>, ptr @.str.desc, i64 <dlen>, i64 <line>, i64 <col>)
call void @__we_test_drive(i64 <n>, ptr @<key>.drive.<n>)
```

`__we_test_drive`（runtime）两模式**一形**：常态 = 直调 thunk 一次（每测试一次额外间接调用，成本披露）；探索态 = 迭代环（D3）。`__we_test_meta` 常态 = 无害暂存（探索态用于报告锚定与守卫诊断位置）。发射形唯一——常态与探索态同一 IR，无编译期分叉（M10b Q3 一形纪律延伸）。

`__we_test_report` 语义不变（thunk 内调用，六参原样）；探索态下 report 不打印而记账（D10 聚合行由 runner 发）。m10b 单测 TestM10bHarnessSynthesis 钉的是 IR 形——随批更新（涉触码盘点已列），conformance 行为黄金零触碰为证。

**测试序号 n**：驱动序内的全局序号（跨模块唯一），种子与报告的身份输入。`@<key>.drive.<n>` 命名沿用 wrap 的 `<key>.<名>.<n>` 族（`wrap` → `drive`）。

## D2 调度策略钩子：取队点注入 + splitmix64 + 域界

**注入点** = sched.c 调度循环的取队处（:504 `we_task *t = ready_pop();`）——就绪队列的队头选取是唯一策略自由度。改 `we_pick_ready()`：常态（explore 未激活或迭代 0）= 现行 FIFO 队头；探索态 = 从就绪队列随机索引取（维护 ready 计数，`idx = rng_next() % count`，走链摘除——与 ready_unlink 同语义）。

**PRNG = splitmix64**（uint64 三步混洗，~10 行，无 UB，跨平台确定）。种子推导：

```
seed(n, i, a) = splitmix64( splitmix64(n * K1 ^ i * K2) ^ (a * K3) )
```

K1/K2/K3 = 黄金比例常数族（0x9E3779B97F4A7C15 / 0xBF58476D1CE4E5B9 / 0x94D049BB133111EB），常数与序列由 C 单测钉死（首 16 个输出对表）——黄金 byte-exact 可 pin 的根基。

**迭代 0 = 确定性基线**（Q1 裁决）：策略关，现行 FIFO——普通 `we test` 已覆盖的类是等价类集的第一个成员（D7 去重锚）。

**域界（裁决记录 Q1 的机制面）**：随机化只落取队点。越刻屏障队列（WE_DRAIN FIFO）与虚拟钟同刻释放序**不动**——ch20:121 钉死的是钟语义（等待先登记、同刻 FIFO），探索探的是**调度选择**。屏障排空后落入就绪队列的同刻任务集合内部可被随机取队重排（探 tied wakes——章文自己的示例场景），但释放入队的 FIFO 顺序本身不变。黄金 test-clock-same-tick-fifo 常态零触碰。

## D3 迭代环与全复位

`__we_test_drive` 探索态主环（test.c）：

```
for i in 0 .. N-1:                       // N = iterations
  for a in 0 .. A-1:                     // A = 4 尝试预算（POR 重采样）
    arm_policy(seed(n, i, a), baseline=(i==0 && a==0))
    run(thunk); trace1 = take_trace(); reset_world()
    if guard_aborted → E1902 已渲染（D6）；此测试探索终止   // 双跑之间查守卫：第一跑已定性，第二跑无意义
    run(thunk); trace2 = take_trace(); reset_world()
    if trace1 != trace2 → E1901（D5）；此测试探索终止
    key = mazurkiewicz(trace1)
    if !reduce || unseen(key): accept（记账 outcome、key 入集）；break
    // 全落已见类 = 尝试耗尽，该迭代停滞（诚实计数 attempts）
  if guard_aborted: break                // E1902（D6）
aggregate_report()
```

**语义定形**：N = 探索迭代数（每迭代目标一个新等价类）；A = 每迭代尝试上限（POR 重采样预算）；每尝试 = 同调度双跑（E1901 判定所需，Q4 裁决）；`--no-reduce` 时 a=0 直接收（不重采样），双跑仍在。最坏执行数 2·A·N（proposal 风险 1 披露）；报告行携 attempts 实数。

**失败即停（与守卫同形）**：任一跑失败（断言 panic / 探索态死锁转换）→ 该测试探索终止，聚合 fail，reason = 该跑 reason。失败是发现，余迭代对已败测试无信息量。

**去重与聚合正交**：POR 键接受与否只引导重采样与 classes 计数；**结果聚合 = 全部已跑尝试的 OR**——若实现期揭出去重吞失败的读法歧义，以本句为准（先于实现定形）。

**全复位清单**（reset_world，介于双跑两半之间与迭代之间同一函数）：
- 虚拟钟：归零（thunk 内 `__we_test_begin` 本就重置，end 弃置残留——M10b 机制复用）；
- 虚拟等待链：end 清扫（残留作废 WE_ABANDONED，M10b 机制）；
- 就绪队列：end 后必空（弃置摘链）——防御性断言，非空即内部错误 abort（诚实）；
- mock 槽：thunk 自带装/恢复（M10b 机制，零改）；
- 迹缓冲与规范化计数器（D4）：每跑清零；
- POR 键集：每测试一次分配，drive 结束释放；
- GC 堆：**不回收**——跨迭代累积（follow-up #11 族，N 有界故泄漏有界；proposal 非目标披露）；
- 任务结构：跨迭代递增不复用（同上）。

## D4 可观察迹：三事件 + 规范化 id

章文三事件族（E1901 描述「observable interleavings」、ch21 R6 场景面）：

| 事件 | 挂钩点 | 记录 |
|---|---|---|
| send 完成 | conc.c Channel send 交付/入缓冲处 | (task, 'S', chan_id, seq) |
| receive 完成 | conc.c Channel receive 交付处 | (task, 'R', chan_id, seq) |
| task 完成 | sched.c task_done | (task, 'C', task_id, seq) |

**挂钩覆盖（实现定形，T3 披露）**：凡「值发生移动」的完成点一律记录——`deliver`（停泊接收方被交付）、`sender_fill`（停泊发送方入缓冲）、两处 rendezvous 直取（发送方事件先于本地接收方事件——远端先于本地的统一次序）、缓冲完成、try 对、select 即取臂；关闭通路的 None 接收不记（无值移动）。比表中两处命名的面更宽，同族同判定，无盲区。

**规范化**：chan_id = 原语创建序号（test.c 维护创建计数器，每跑清零——堆地址跨迭代不稳，创建序稳定）；task_id = 任务创建序号**相对该跑 extent 基准的差**（begin 记录的表计数——slot 单调递增不复用，E1901 对比是同尝试内两跑对比而第二跑的任务 slot 严格更大，裸 slot 会使每对都分歧；实现期修正回填，T3 记录披露）。seq = 每（task, kind）单调序——同任务同类多次事件保序。

**双记录**：全局序（E1901 对比用——总序即「observable interleaving」的字面）+ 每任务子序（D7 等价键用）。实现形 = 双跑对双缓冲（`__we_trace_select(0/1)` 两半，每跑记一半，seq 由读取时按缓冲序派生——与「追加时同步写两处」同观察面，单一数据源，T3 记录披露）。

**上限**：每跑 4096 事件，满即停录并置 truncated 标志。截断的跑不参与 E1901 对比（诚实跳过——两跑都完整才可比）；等价键含 truncated 位。测试规模超 4096 事件属异常，披露不拦。

**只录探索态**：常态运行零记录开销（一处分支）。

## D5 E1901：同调度双跑对比 + 渲染

每尝试两跑同种子同策略 → 调度序列相同 → 全局迹**必须**相同（虚拟域内一切确定：单线程、虚拟钟、mock 槽、确定性分配）。分歧 = 虚拟化确定性之外的东西在起（真等待、未 mock 效果的真实行为——注册表 remediation 原文）→ E1901。

对比 = 长度 + 逐元素（kind/task/target/seq 四元组）。分歧即：
- 渲染诊断（title 逐字 `exploration detected nondeterminism`，detail = `the same explored schedule produced divergent observable traces (iteration {i}, attempt {a}) — first divergence at event {k}: {e1} vs {e2}`，e1/e2 = 四元组紧凑渲染）；
- 锚 = `__we_test_meta` 的 file:line:col（测试声明位置，风险 3 披露）；
- 双面：stderr 人类行 `file:line:col: error[E1901]: title — detail`（diag.Human 同形）；--json = stdout 诊断事件（diag.JSON 字段序 type/severity/code/message/file/line/column——M0 快照钉死形，C 侧复制披露）；
- 此测试探索终止（守卫后余迭代无意义），聚合 fail，exit 1。

**We 级正例不可构造**（proposal 非目标）：语言内今日无非确定性源——ch20 兑现的自证。正例 = runtime C harness 注入测试（harness 直接以不同迹喂对比器）；We 级黄金钉负例（确定性测试不触发）。

**渲染细节（实现定形，T4 披露）**：对比器为纯函数 `__we_e1901_first_div(q0,n0,q1,n1)`（stride-4 四元组，严格前缀在短侧尽处分歧）；e1/e2 紧凑渲染 = `{task}/{S|R|C}/{target}/{seq}`（harness 同形），尽侧渲染 `<end>`；iteration/attempt 由调用方传入**人类计数（1 基）**，runner 传 i+1/a+1；C 侧 diag 复制不带 help 字段（协议「有才出现」的合法省略）；E1901 的 message 同时留作聚合 reason（渲染与 reason 单一数据源）。聚合 reason 语义（黄金钉死）：探索调度失败 = `explored schedule failed: {run reason}` 前缀；守卫/E1901 = 诊断 message 原文。

## D6 E1902：custom-effect 槽门（fxgate）

**判定面**（章文 + 注册表）：探索运行执行了未 mock 的**custom effect**——`W1910` 同条件在探针下的错误化（ch21:127「the condition W1910 advises about in an ordinary run, an error under exploration」）。custom = 声明段含非内建标签（内建 = io/net/time 三裸键；bare 自定义与 `mod.name` 合格形皆 custom）。

**机制**：M10b 槽形的单点延伸——custom-effect fn 的**槽默认 = 直通门**：

```llvm
define internal {i64,i64} @main.query.fxgate(ptr %a) {
entry:
  call void @__we_explore_fx_check(ptr @.str.qname, i64 11)
  %r = call {i64,i64} @main.query(ptr %a)
  ret {i64,i64} %r
}
```

- 同 ABI 克隆（六族全支持——聚合返回 ret 聚合值、ptr 族直传；T2 定形矩阵）；
- `@slot.main.query` 初值 = fxgate（非真体）；
- 驱动恢复常量同步换 fxgate（M10b 恢复 = 真体名 → 换 fxgate 名）；
- 真体 define 名不变（`@main.query`——fxgate 内直调，不回槽防递归）；
- **达门即未 mock**：mock 装填把槽指向 mock fn，门永不进入；门被进入 ⟺ 调用走了默认槽 ⟺ 未 mock。单一发射形（M10b Q3 先例），无调用点分析。

`__we_explore_fx_check(name, len)`（test.c）：常态直退（探索未激活）；探索态 → 渲染 E1902（title 逐字 `unmocked effect executed during exploration`，detail = `the explored path called {name}, whose custom effect had no mock`，name = **fn 声明名裸名**），置 guard_abort，本尝试余体照常执行（今日 custom-effect fn 体是 We 代码本就确定；未来 FFI 真体不确定——门在入口拦住判定，体执行属已败跑的既成事实，披露），runner 见 guard_abort 终止此测试探索，聚合 fail，exit 1。每测试至多渲染一次（guard_abort 幂等门）。

**W1910 复用面**：M11 的 `we vet` 落 W1910 时同一门换告警渲染（设计注记，非本变更面）。

**检查塔零改动论证（原则 10 三要素的检查侧）**：探索塔零语言面——不新增 We 源形式、不新增声明/类型/效果规则；效果段与 test 体的检查由 ch16/ch20 检查塔（M7/M10a）早已承载。custom-tag 判定不经 typecheck：codegen 直读 FnDecl 声明段原文（内建 = 裸 `io`/`net`/`time` 三键；ch16 裁决内建标签是语言级非模块项，故合格形永不内建），谓词自足无需解析器帮忙。manifest 侧 E1903 走 check.go 既有装载门（M3 面）。

## D7 POR：等价类去重

等价关系 = Mazurkiewicz（Q2 裁决）：两全局迹等价 ⟺ 每任务自身事件序相同。操作化：

- 每任务子序 → FNV-1a 64 滚动 hash（事件四元组逐个喂）；
- 等价键 = 全部任务子序 hash 的**可重集**（排序后串接再 hash；任务匿名——跨迭代任务身份不稳，匿名多重集是诚实近似，披露）；
- 键集 = 开放寻址表（malloc，容 2N；测试结束释放）；
- 落已见类 → 该尝试作废重采样（D3 预算 A 内）；`--no-reduce` = 不查集直收（每迭代 1 尝试）；
- classes = 键集基数，入报告行。

**诚实边界**：匿名多重集把「任务身份置换」也视为等价——比严格 Mazurkiewicz（按进程配对）粗一档的归约。这个方向不丢诚实：被合并的类内，各任务自身事件序完全相同，即在本探针的可观察面（三事件 per-task）下确无可区分的行为差异；粗一档换来的是任务身份免跨迭代配对（身份本就不稳）。报告行携 attempts/classes 实数，采样永非穷尽（章文自己的话）。

## D8 探索态死锁转换

常态死锁 = abort + exit 1（M10b 黄金钉死，不动）。探索态下被探调度可能死锁——那是**发现**不是崩溃：转换为该调度的失败并续跑。

机制（无 longjmp——复用 panic 通路）：死锁检测处（调度循环空闲分支）探索态分岔：
- 对驱动 await 的 main（task 0）force-settle：handle_await 返回 tag 1、payload = 消息 C 串 `deadlock: {N} tasks parked with no wake source under the explored schedule`；
- 全部残留任务 `__we_task_abandon`（WE_ABANDONED 永不再调度，M10b 机制）；
- main 从 await 唤醒 → thunk 的 report 分支自然发 fail（reason = 上述消息）→ runner 记该尝试失败 → 续迭代。

**构造面披露**：本原语集（channel/semaphore/await + 虚拟钟 deadline）下 park 拓扑 schedule-独立——配对则全序完成、失配则全序死锁，故 We 级「只在某调度死锁」正例预计不可构造（T1 尝试，不成则 C harness 单测承载转换面 + 披露）。

**实现定形（T4 披露）**：We 级正例确不可构造（黄金矩阵无死锁枚，D11 对账成立），转换面由 runtime C harness 承载（stuck 任务 + 驱动 await → settle → 聚合 fail + 进程续跑全钉死）。机制落点：idle 分支探索分岔 `explore_settle(parked)`——消息入 test.c 缓冲（N = 检测时全部 park 计数，含驱动 await 本身）、旗标置位、非 done 全弃置、唤醒 task0；`handle_await` 循环顶消费旗标（tag 1 + payload = 消息）；settle 见前次未消费即内部错误 abort（形状防御——驱动形保证 main 只 park 于 await）；run 间复位清旗标。

## D9 CLI 面 + manifest [test] + E1903

**旗标解析**（cli.go parseOptions，test 子命令专属——`--filter` 先例）：

- `--explore`（裸 bool）；`--no-reduce`（裸 bool）；
- `--iterations N` / `--iterations=N`：十进制正整数，非法 = usage 2（`invalid --iterations value`）；
- 默认序：显式 `--iterations` > manifest `[test].explore-iterations` > **100**（实现自定，披露——章文不定数值缺省）。

**manifest 读取**（check.go loadManifest 扩展，共享面）：解析 `[test]` 表的 `explore-iterations` 键——正整数存值；**非正或非整数 → E1903**（注册表 description 点名「an explore-iterations that is not a positive integer」；报文 = title + key + legal + found，`invalid toolchain configuration value — explore-iterations must be a positive integer, got {v}`）。E1903 在 check/build/run/test 四面一致（loadManifest 共享——与 type/version/name 同待遇，ch21:174 场景「a manifest writes … explore-iterations = 0」THEN E1903）。表内其余键不校验（与 [vet] 今日同姿势——spec 只定该一键）。无 [test] 表或无该键 = 缺省 100。

**argv 透传**（test.go runTestBinary）：`--explore`（若激活）+ `--iterations N`（解析后终值——manifest 默认在 Go 侧解析完毕）+ `--no-reduce`（若给）+ `--json`（先例）。

**startup.c 解析**：argv 扫描三旗标（--json 先例的扩展）→ 置 runtime 全局（`__we_explore_on` / `__we_explore_iters` / `__we_explore_reduce`）→ `__we_test_drive` 按全局走环。

**退出码**：子进程 0（全确定全过）/ 1（任一失败或守卫）；编译败 2 在 CLI 侧（M10b 面零改）；usage 2 = 旗标非法。

## D10 报告面

**人类行**（M10b 行形延伸）：

```
pass  tests/m_test.we: probe tied wakes (explore 200 iterations, 200 attempts, 14 classes)
fail  tests/m_test.we: probe tied wakes (explore 200 iterations, 213 attempts, 3 classes)
  explored schedule failed: assertion failed: got 2, want 1
```

- 聚合行 = 探索全部迭代的结果：任一尝试失败 → fail + 缩进原因行（首个失败尝试的 reason——多失败取首，披露）；守卫终止 → fail + 守卫 detail 行。
- 常态行零改（`(0ms)` 形不动）。

**JSON**：test-result 事件增可选对象字段 `"explore":{"iterations":N,"attempts":A,"classes":K}`（R7 只增不删改名承诺内；常态缺省省略该字段）。summary 事件零改（探索失败计入 failed）。守卫诊断 = 标准诊断事件（child 发，stdout）。

**duration_ms** = 探索中最长单次虚拟时长（各次迭代虚拟钟差取 max——量测的是测试本身而非探针重复；语义披露）。

**探索期 io 输出抑制**：探索跑中 `io.println`/`io.print` 的写入弃置（io 效果照常执行——门与迹判定零影响，io 不入迹；弃置的只是字节）。理由：N 百次重跑的流式输出既非可读报告也非确定可观察面（重复 × 交错），聚合行取代之；常态 `we test` 单跑输出零改。运行时实现 = io.c 出口一处探索态分支。

**实现定形（T4 披露）**：JSON 面的失败原因走 stderr `file: name: reason`（M10b 单测试 JSON fail 无 reason 字段先例的延伸——R7 字段集闭合，reason 不发明新字段；此面黄金未钉，runtime C 单测钉死）；io 抑制落 io.c 四出口（println/print/println_i64/print_i64）各一探索态分支——`__we_explore_on` 定义于 test.c（单一权威），io.c 仅 extern；代价 = runtime io 单测编译集扩容至含 sched/test（行为断言零改）。

**汇总行**：`total {N}, passed {P}, failed {F} ({D}ms)` 零改——D 同取 max 语义。

## D11 conformance 黄金矩阵

命名 `test-explore-*`，全部 byte-exact（种子确定 → 可 pin）。预计 ~18 枚（表枚 19 槽 18 枚——explore-pass 兼旗标面，定数随 T1 披露）：

| 组 | 枚 | 钉死面 |
|---|---|---|
| 基础 | explore-pass（确定性测试 exit 0 + 行含 attempts/classes）/ explore-iterations-flag（`--iterations 3`）/ explore-manifest-iterations（[test] 表 5）/ explore-default-iterations（无键 → 100 行面）| 旗标面 + 报告行 + 默认值 |
| 发现 | **explore-order-found**（断言只在非基线调度失败——两 tied 任务 send 序翻转 → fail + exit 1 + 原因行，钉死首个失败迭代号）| Q1/Q4 展示黄金：探索真找到确定性调度不会取的交错 |
| 守卫 | explore-e1902（unmocked custom effect → 诊断 + exit 1）/ explore-e1902-mocked（mock 后门静默 pass）/ explore-e1901-negative（确定性测试不触发）| E1902 双面 + E1901 负例 |
| io 抑制 | explore-io-suppressed（打印测试探索跑——stdout 无打印字节，聚合行取代）| D10 抑制面 |
| POR | explore-no-reduce（tied 源 attempts == iterations == 3——重采样关的直证；classes = 1）/ explore-reduce-merges（tied 源 attempts 9 vs no-reduce 3——去重重采样合并的直证）| D7 直证 |
| manifest | explore-manifest-invalid（`explore-iterations = 0` → E1903 + exit 1，test 面）/ explore-manifest-invalid-check（check 面同码）| E1903 两面 |
| usage | explore-usage-iterations（`--iterations abc` → usage 2）| 旗标门 |
| 协议 | explore-json（test-result explore 对象 + summary + exit 码保持）| D10 JSON 面 |
| 既有面 | explore-compile-fail（exit 2 不变）/ explore-filter（--filter + --explore 收窄）/ explore-single-file（单文件面）/ explore-empty（空集 total 0 exit 0）| 组合面 |

负面对账：**既有 588 枚零触碰**（T1 全量跑对账，`git status` testdata 无既有文件改动为证——目标 8）。

**We 级不可构造面**（proposal 非目标披露，C harness 承载）：E1901 正例（注入对比器）、死锁转换正例（若 T1 不可构造）。

## D12 验证阶梯

T8 终验固定序：`go build ./...` + `go vet ./...` → `go test ./...`（全包）→ runtime C harness 全套件 → conformance 全量（588 + 新增）→ 真二进制黑盒电池（T7）→ `python3 openspec/tools/validate.py --all --strict` → `python3 openspec/tools/docs_sync.py`（对数不变取「变更创建态」基线——M10b T8 判明的操作口径）→ `git status` 对账（refr/ 零触碰、既有黄金零触碰）。
