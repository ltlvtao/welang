# tasks — concurrency-run（M9b）

- [x] T1 黄金先行：conformance run 族黄金落盘（~30 枚：原语四态/Cond/Semaphore 含 panic 面/Channel 全链/select/scope 四形/TaskPanic/cancel/GC 存活/死锁），全部 build+run 双断言，对当前构建预期红；M9a 六枚 build-bnd-conc-* 改写为 run 断言（随批更新计划内，先红）
  来源：proposal 目标 8 / design D11 D12
  验证：`go test ./internal/conformance/` 新增枚全红（build 停 bndMainBody exit 70）、六枚翻绿目标枚红；红输出留存证据记于完成记录

  **完成记录（2026-09-07）**：33 枚新黄金落盘（生成器 `/tmp/m9b-goldens/gen.py`，/tmp 惯例；`we run .` 单命令即 build+run 全链断言——run 内部先 build，M4 先例）。分布：四态胞 4（mutex-update 含 if 观测/set-get/rwlock-read/atomic-get）+ Cond 2（signal 跨任务/broadcast 三等待者序）+ Semaphore 3（basic 含 tryAcquire 双向/blocking 跨任务 park/overrelease panic 经 `?` 传播 main Err exit 1）+ Channel 6（send-recv/close-drain 两段/close 后 send panic/full-park 跨任务/rendezvous 容量 0/tryrecv 四态含 trySend 三臂穷尽）+ select 2（双 channel 就绪取源序首臂/await 双源——两任务先完成后 probe 取第一臂）+ scope 5（plain 纯序/timeout-ok/timeout-err 30ms 真时钟/collectAll 任务 panic await 捕获不退出/timeout-question `?` 传播 `error: TimedOut`）+ TaskPanic/cancel 5（taskpanic-await `error: Panicked: boom`/coop 取消唤醒后协作返回/never-checks 跑完/discard-panic 存活/sig-await 含 isCancelled 双向）+ 调度与语言面 6（interleave 跨 park 交错序/gc-park-survival gc record 过 rendezvous/deadlock 容量 0 交叉互等 exit 70/defer-task 倒排/while-scalar var+赋值+算术/match-option）。
  **六枚翻绿改写（M9a 边界黄金→run 断言，随批更新披露）**：5 枚改名翻绿（build-bnd-conc-{ctor,scope-stmt,scope-value,two-tasks} → run-conc-* 同名尾、build-bnd-conc-io-arg → run-conc-select-int——原程序 select 双空源在 M9b 下会死锁，程序改为先 `a.send(1)` 使源就绪，stdout 钉 "start\n"）；**1 枚维持不改**：build-bnd-conc-fn 今日停点实为 bndOtherFns（fn 声明先于 main 体遇），M9b 裁决 Q2 维持该边界——此枚转为 M9b 边界回归钉，proposal/记忆中「六枚翻绿」修正为「五枚翻绿 + 一枚维持」（D13-5 披露口径同步）。改写枚断言形：args `["run","."]`、exit 0、stdout 空（select-int "start\n"）、stderr 空、files 面删除（M4 run 先例）。
  **红证据**：38 红 = 33 新增 + 5 翻绿目标，全部 `run-conc-*` 前缀、全部 stderr 为既有 bndMainBody 边界行 exit 70（红因正确分类：codegen 发射面缺失，非检查面）；其余 470 枚全绿零回归。总数 475 → 508（470 既有 + 33 新 + 5 改名）。D12 预估 ≈505 → 实落 508。
  **黄金措辞预钉（实现前手写，待实现校正披露）**：panic/deadlock 报文四条按 design D2/D6 措辞预钉（"semaphore released past its constructed count" / "send on a closed channel" / "Panicked: boom" 直接载荷 / "we: deadlock: 2 tasks parked with no wake source"）。

- [x] T2 单测先行：runtime/sched_test.go（调度交错/park 唤醒/yield/timeout 真时钟/死锁 exit 70/GC 跨 park 存活）与 runtime/conc_test.go（Semaphore 超额 panic 槽/Channel rendezvous/close 排空/select 取臂撤销）C harness 先写；internal/codegen/m9b_test.go（发射面 IR 钉 + 三边界词 What 串钉）——三者先红
  来源：proposal 影响范围 / design D11
  验证：三套件编译期红（缺实现）证据留存；既有 runtime/codegen 套件零回归

  **完成记录（2026-09-07）**：三套件落盘，编译期红全因缺实现：runtime 编译红 `undefined: SchedSource` / `undefined: ConcSource`（embed 变量未定义——sched_test.go 与 conc_test.go 同因，输出被 too-many-errors 截断）；codegen 编译红 `undefined: bndTaskBody` / `undefined: bndCallbackBody`（×3，v2 边界词常量未定义）。全仓 `go test ./...`：cli/diag/lex/parser/typecheck/version 六包全绿，conformance 维持 T1 的 38 红（run-conc-* 族），既有面零意外回归（runtime/codegen 两包因编译红不可跑，即 M9a T2 先例的「包级编译失败恰为 undefined」形）。
  **C harness 面（runtime 侧规格，T3/T4 实现照此 ABI）**：sched_test.go 五测——FIFO 序（两 thunk 双 yield + await 双 join，钉 a-start/b-start/a-end/b-end/joined 序）、task panic 边界（`__we_task_fail` → await tag 1 + msg 载荷）、timeout 真时钟（scope 30ms + 空 channel park → leave 返回 TimedOut=1 且墙钟实过 >20ms）、死锁（容量 0 交叉互等 → 子进程 exit 70 + stderr "we: deadlock: " 前缀——新 helper 直跑子进程断言退出码，compileAndRun 只比 stdout 不够）、GC 跨 park（任务 root 块 park 中被另任务 collect 后 payload 存活 + swept 计数 0）。conc_test.go 六测——Semaphore（acquire/tryAcquire 双向/count + 超额 release 任务内 panic `semaphore released past its constructed count`）、rendezvous（receiver 先 park、sender 直接交接）、close 排空（drain 值 → None 永久 → send-after-close 任务内 panic `send on a closed channel`）、Cond（假谓词 signal 不放行 + set+broadcast 唤全重查）、四态胞直访（update×2/set-get/read/atomic-get）、select（快路径取源序首臂 + 真 park 唤醒 + **取臂后其余源零残留**——F1 的 task_next 账本链验证面，after 断言空 channel 再 recv 正常 park 配对）。
  **codegen 钉面**：m9b_test.go 九测——ScalarAndControl（var/while/算术/if → add+icmp+br i1）、PrimCtorCall（13 子例表驱动：六构造族 + update/set/get/read/sem 四法/channel 五法/handle 两法/cond 两法 → ABI 名钉）、SumSlots（receive + Some/None 双臂 match → `__we_chan_recv`+icmp）、TaskThunk（`__we_task_new`+`__we_root_push`+thunk define≥2）、ScopeForms（plain/timeout/collectAll → enter+leave）、SelectForms（语句/值两位置 → new+add_recv+park）、QuestionEmission（`t.await()?` → `__we_handle_await`+br i1 早退）、DeferOrder（倒排钉：body 常量池序先于 deferred）、BoundaryWhats（main fn 调用→bndMainBody v2 词、task 体 fn 调用→bndTaskBody、回调体 io/fn 调用→bndCallbackBody ×2）。
  **ABI 定形（测试固化，design 回填点）**：`__we_scope_enter(deadline_ms, collect_all)` 两参形（四形之别在 ABI 承载——collectAll 非 leave 语义可表达）；`__we_handle_await(h, *payload)` 返回 tag 0/1、payload 兼任 Ok 载荷与 panic msg 指针；`__we_select_new/add_recv/park/value` 五 API；`__we_chan_send/recv` 返回 0/1 与 1/0 形；`__we_task_fail(msg)` 为任务体 panic 入口。以上入 D2/D4/D5/D6 回填清单（T10 披露）。

- [x] T3 调度器：we_task 结构与 task 0 boot（startup.c 改 trampoline 形，main 体可阻塞 join）、就绪队列 FIFO 与 swapcontext 循环、makecontext 传参交接槽、deadline 空闲睡与死锁诚实 abort
  来源：proposal 目标 1 / design D1 D2
  验证：sched_test.go 转绿；既有 runtime 测试零回归

  **完成记录（2026-09-07）**：runtime/c/sched.h（we_task/we_wait_link/we_scope 共享结构 + ABI 面）+ sched.c（~380 行）+ startup.c 改造（main → `__we_sched_boot(__we_main)`，task 0 普通任务化、main 体自此可阻塞 join）+ runtime.go embed（SchedHeader/SchedSource）。实现要点：调度循环独立栈（static 64 KiB）+ setcontext 进入；任务栈 256 KiB malloc；task 0 经 main_thunk 包装与普通任务同构；deadline 用 CLOCK_MONOTONIC 绝对毫秒 + clock_nanosleep TIMER_ABSTIME 睡到最早活跃 scope；expire 标记 timed_out 后取消未完任务（协作返回，pending 由 task_done 归零唤醒 owner——leave 循环只被归零唤醒）；取消全链惰性失效（fail_links 标 dead，源操作跳过+收割）；plain scope 任务 panic fail-fast（取消同 scope 兄弟 + panic 传播 owner 的 leave）；main panic → stderr "error: Panicked: msg" + exit 1（ch14 abort 面）；死锁 → stderr "we: deadlock: N tasks parked with no wake source" + exit 70，**N 数全部 PARKED 任务（含 park 在 join 的 main）**——黄金 deadlock 枚预钉 "2 tasks" 须校正为 "3"（main await 同 park），T10 披露。
  **两处实现期修正（先于测试绿，D13 披露）**：① makecontext 全局交接槽被连续 task_new 覆盖（首次运行读到后者）→ 改 int 槽索引传递（增长任务表 + `(void(*)(void))` cast，POSIX 原型 void(void) 而文档许 int 参——proposal 已知风险预告的 cast 面）；② __we_link_free 账本遍历误用 next 字段 → task_next（账本链的本来语义）。
  **验证**：FIFO 与 TaskPanic 两 harness 手动编译运行精确匹配（a-start/b-start/a-end/b-end/joined-a=0,0,42；tag=1 msg=kapow）。timeout/deadlock/gc-park 三 harness 与既有 gc/io 回归须待 conc.c 落盘后包级合验（ConcSource undefined 挡编译）——T4 记录补验。

- [x] T4 等待机器与六族原语：we_wait_link 通用协议（probe/登记/挂起/唤醒/撤销）与取消唤醒路径、四态胞直访（退化披露）、Cond 谓词循环、Semaphore（含运行时陷阱半边）、Channel 全链（环形缓冲/rendezvous/close 排空/try 三态）、TaskHandle 与 CancelSignal、scope 帧四形与 Err(TimedOut) 构造、select 登记-park-撤销-取臂、TaskPanic 边界（defer 倒排-存槽-DONE-唤醒；main 内维持 __we_fail）
  来源：proposal 目标 2–5 / design D3 D4 D5 D6
  验证：conc_test.go 转绿；sched_test 零回归

  **完成记录（2026-09-07）**：runtime/c/conc.c 落盘（~670 行）。统一 we_link 族注册节点（we_wait_link base + value/out/sel/arm/sel_next/consumed），六族原语全链：四态胞直访（单线程调度下 Mutex/RwLock/Atomic/AtomicRef 为纯存储——E1402 纯度即临界区，退化面已按 D3 披露）；Cond 谓词循环（唤醒是提示、谓词定去留、取消破环协作返回）；Semaphore（超额 release → `__we_task_fail("semaphore released past its constructed count")`）；Channel 全链（环形缓冲 head/count 取模、rendezvous 直交接付、close 只 wake 全员——语义活在各循环里：sender 醒来见 closed 即 panic、receiver 排空后永久 None、try 三态 0/1/2）；CancelSignal（lazy 构造、登记即等待——取消唤醒由 sched 的 cancel 路径执行）；select（登记-park-撤销-取臂：快路径按 case 序取首就绪源、真 park 经 deliver/sender_fill/complete 三路交付、loser dead 标记留给源懒 reap）。
  **consumed 协议（实现期重构，D13 披露）**：初版唤醒-重查有缺陷——被直交付唤醒的任务醒来重查循环时旧 link 未消费未摘，会二次登记重 park。定形协议：交付方（deliver/sender_fill/rendezvous 取值）设 `consumed=1` 并 wake 但**不 drop**；阻塞循环醒来查 consumed——已消费则 drop 旧 link 返回，否则 drop 后重查循环。owner 醒来自己 settle，交付方永不 free 别人持有的节点（use-after-free 面归零）。
  **实现期修正（先于测试绿，D13 披露）**：① we_link_drop 对已 `__we_link_free`（内部即 free 整块）的节点二次 free → 删多余 free（rendezvous/cond/sem 的 tcache double-free 根因）；② we_cond/we_sem/we_chan 的 C 结构体字段从 offset 0 起，与冻结头 {map@0, size@8} 重叠——chan 的 head 读到 size 槽值导致环形索引错位、缓冲值丢失 → 三结构加 we_prim_hdr 16 字节占位头（载荷字段自此起于 16，与 gc 块头共存）；③ select 臂经 sched 唤醒（task_done/cancel_wake）不 fire 臂 → we_wait_link 加 complete 回调钩子，两路径对 select 臂走 complete（fire 首臂后 wake）；cancel 对 park 中的 select fire 首臂为 unspecified-but-safe 选择（ch18 未指定臂序）。
  **测试面修正（T2 预钉错误，随实现校正披露）**：① Semaphore 期望值三处（tryAcquire 双向语义、count 终值）；② CloseDrain/Rendezvous 的 printf 多参求值序不指定（clang 右到左）→ 拆行打印；③ Select 第三幕原设计 recv 无任何 wake 源（feeder 已结束）必真死锁——改为 feeder 双发（9 给 select、8 作残留探针），断言 `after=1,8` 配对成功，无残留验证面更真；④ Deadlock harness 原设计 main 创建后直返（task 0 完成 exit 0，任务从未跑）→ main await 双句柄，三任务互等成真死锁，exit 70 + stderr "we: deadlock: 3 tasks parked with no wake source"——**死锁计数 3 实证**（main join 同 park），与 T1 预钉 "2" 的校正预告一致，T8/T10 按此改黄金。
  **有界泄漏裁决（D13 披露）**：① select loser 的 dead link 留在无人再操作的源队列上（懒 reap 仅发生于源操作时）+ select 块本体 park 返回后仍被 value 读取——均不释放，有界于登记节点数；② sig 臂的取消竞态窗口（登记与 cancel 并发——单线程下指登记前 check 与 park 之间被 yield 切走）同理有界。均为泄漏不正确性，记入 D13 清单待 M10+ 收窄。
  **验证**：runtime 全包绿（conc 六测 + sched 五测 + 既有 gc/io/alloc 套件零回归）——含 T3 顺延的 timeout/deadlock/gc-park 包级合验。

- [x] T5 GC root 任务化：roots[]/root_depth 迁入 we_task、__we_root_push/pop 经 __we_cur_task 定位、collect 遍历全部任务 root 栈（含 PARKED）、既有 4 MiB 存活风暴探针零损重证
  来源：proposal 目标 6 / design D7
  验证：runtime 全套件绿；M8 conformance 黄金零触碰全绿

  **完成记录（2026-09-07）**：根窗口机制落 gc.c——we_gc_window {roots, n, cap, next}：push/pop 走活动窗口（active_win），collect 的标记阶段遍历 all_windows 注册链（**所有任务的窗口含 PARKED**——park 永不等于 drop）。挂载形与原文偏差披露：原计划「__we_root_push/pop 经 __we_cur_task 定位」需 gc.c 反向依赖 sched 符号（分层倒置），实现改为**调度切换点 swap**（sched_run 换任务前 `__we_gc_window_swap(t->gcwin)`、回程换 scratch）——等价且 gc/sched 分层保持单向（sched.h 前向声明 + gc.c 导出三 ABI：window_new/retire/swap）。
  **boot_win 静态头节点**：不链 sched.c 的旧 harness（M4–M8 的 gc/io 测试）collect 走它——初版遗漏挂链致 M8 TestGCHarness segfault 回归（根集空、全扫、use-after-free），静态初始化挂链修复后零回归（实现期修正，D13 披露）。
  **task_done retire**：窗口摘链 + 数组释放 + n 清零——panic 路径不 unwind 根栈，残留根若无 retire 将永久标记死对象；retire 后 done 任务的根从此停标（有界：panic 时根集大小，D13 清单同记）。
  **harness 修正（披露）**：GcPark 的 main 补 `__we_root_push(g_go)` 模拟 emitted shadow-stack 根——chan 是 C 全局不在根集，真实代码里编译器会对活跃局部发根登记；无此登记时 chan 块被跨任务 collect 扫除（swept=1 与 survived=7777 并存恰因 free 块未覆写）。
  **验证**：runtime 全包绿（含 M8 gc 风暴探针——4 MiB 存活风暴在窗口机制下经全任务标记零损）；M8 conformance 黄金零触碰（conformance 的 38 红均为 T1 目标红，build 停在 codegen 缺失边界，与 runtime 改动无涉）。
  **T10 修正（本记录前段失实）**：上句「38 红均为 T1 目标红」不实——T3 改 startup.c 引 `__we_sched_boot` 起，conformance build 枚因链接序未扩而炸（14 枚意外回归，面板实为 52 红 = 38 目标红 + 14 链接红），T5 当时误读为全目标红。T8 扩 build.go 链接序后 14 枚归位、38 枚转 T7/T8 逐枚消化。记入 T8 完成记录的预告在此成文。

- [x] T6 codegen A：标量 i64/Float64 值表示与 let/var 算术比较发射（含 E0502 溢出陷阱）、sum 双槽 {tag, payload} 与变体构造、原语构造（__we_prim_new_* 族）与 emitPrimCall 分派表、回调闭包机器（cb thunk + 环境块构造与 root 协议 + bndCallbackBody 边界词）
  来源：proposal 目标 7 / design D8 值表示与语句 1–5、D9
  验证：m9b_test.go 对应钉转绿；M8 既有 IR 钉零漂移

  **完成记录（2026-09-07）**：emitter 三向扩展——bodyCtx（bnd() 按语境选词，ctxTask 挂 T7 的 task 发射）；scalarSlot 环境（let 持 operand、var 持 alloca、isFloat 域标）；sumSlot 双槽环境；thunk/ovf 副流（回调 define 与溢出报文全局在主指令流之外）。标量发射：emitNumExpr 覆盖 int/bool/float 字面量（float 0x hex 精确 double）、Ident、`+ - *`、六比较（icmp/fcmp + zext 入 i64 域——Bool 即 i64）、`&& ||`（and/or i1；操作数限于无副作用数值形故非短路=短路）；**E0502 陷阱**：`llvm.{sadd,ssub,smul}.with.overflow.i64` + extractvalue 拆 {值,溢出} + br 失败臂 `__we_task_fail("integer overflow")` + unreachable + cont 块续流（Float64 走 IEEE 域无陷阱）；var 为 alloca+store（窄整型符号扩展入 i64 域）、Assign 仅 var。prim 构造六形分派（Mutex/RwLock/Atomic/Semaphore 标量值参、Cond(ptr)、channel(cap, 8, null)——**标量元素域**：引用载荷 channel 停 bndMainBody），构造即 root push（与 M8 record 同协议、尾部统一 pop）。emitPrimCall 分派表 15 方法：update/read 经回调机器、set/send/trySend、get/acquire/tryAcquire/currentCount/cancel、receive/tryReceive/await 构造 sum 双槽（out alloca + tag 返回值 + payload 装载）、release/close/signal/broadcast。回调机器：`define internal i64 @.cbN(ptr %env, i64 %v0)`、**零捕获**（谓词引用外层绑定停 bndCallbackBody）、体单直线数值表达式、body Builder 换向（thunk 流与主流隔离、共享常数池与溢出报文）。io 扩展：runtime/c/io.c 增 `__we_println_i64`/`__we_print_i64` 十进制渲染器——println/print 的标量参数走 i64 面、String 参数走 M8 双字面（字节不变）。declare 表尾追加 39 行（未用不 declare——M8 模块字节零漂移）。
  **既有钉面三处翻绿改写（随批披露）**：m9a_test 两形（conc ctor binding/discarded——改断 `__we_prim_new_mutex` ABI）、codegen_test 一形（`let x = 1` 数值绑定——改断 clean）；M8 四 IR 快照与 M4 面零漂移全绿。
  **诚实范围披露**：① `/` 与 `%` 未发射（无黄金无测试钉——bndMainBody 停，窄域运算同停）；② sum 构造仅经 receive/tryReceive/await 三入口（Some(x)/None 字面构造无独立发射——黄金 match 源全是 channel/handle 面，T7 match 只消费已构造双槽）；③ Float64 无 io 渲染（println 浮点参数停）；④ 闭包仅零捕获形（捕获机器为 M9b 边界——design D8 闭包捕获不在最小充分面）。
  **验证**：m9b PrimCtorCall 13 子例全绿 + BoundaryWhats 回调两例绿；ScalarAndControl/SumSlots/TaskThunk/ScopeForms/SelectForms/QuestionEmission/DeferOrder/BoundaryWhats(task 形)/handle calls 红均为 T7 面（控制流/task/scope/select/?/defer/match）。M8/M9a/M4 全绿。

- [x] T7 codegen B：`?` 两语境发射（emitQuestion：main Err 尾 / task Err 结束）、Option/Result 双臂 match（tag icmp + 载荷解构）、while/if 最小控制流（emitBody 递归）、defer 倒排发射、task 块（thunk 发射 + env root 交接协议 + __we_task_new）、scope 形（enter/leave + timeout 求值点）、select 形（register/park/臂序 switch）、bndMainBody v2 词汇改写与 bndTaskBody 新词、m9a_test.go 十形 What 断言同步改写（披露）
  来源：proposal 目标 7 / design D8 语句 6–10、D9
  验证：m9b_test.go 全套转绿；m9a_test 改写面逐处披露

  **完成记录（2026-09-07）**：发射面按 38 枚黄金源全集反推实现（m9b_test 九测只钉最小面，黄金通读揭出 wait/sig/panic/`?`/引用载荷 channel/别名遮蔽/Ident 重绑定/trySend 三态等真实需求——全落）。控制流：emitIf（else nil|BlockExpr|嵌套 If 共一 join 标签）、emitWhile（head/body/exit 三标签，break/continue 边界外）、emitMatch（tag 链式 icmp + wildcard 兜底 + join；PatBinding 装载 payload 入标量域）、emitBlockStmts（嵌套块内 Return → bnd——return 只在体尾）。defer 收集列表倒排发射（main 尾 Ok 分支与 task thunk 尾均先于 root pop/ret）。`?`：emitQuestion（icmp tag → ok/err 双臂；errPanic 形 inttoptr 载荷 + `__we_task_fail`——await 来源；errMsg 形静态常量 + `__we_fail`——timeout scope 来源 "error: TimedOut"）；emitSumSource（Ident 透传 sums2 或 Call 求值）。task 块：collectCaptures 预扫描（prims → traced 槽、scalars → i64 槽；String/record/sum 捕获停边界）+ env 块（`__we_alloc(16+8n)` + `@.emapN` 位图 + root push——创建体尾部统一 pop）+ thunk define（ctx 切换 ctxTask、局部环境清空、caps.ptr=%env、restore 闭包；尾值 = 体尾 ExprStmt 值表达式经 emitNumExpr，非值形尾 0）+ defers 倒排 + `__we_task_new`；thunk 内 root push 由窗口 retire 兜底不弹。scope 形：emitScope(valueForm)——deadline（nil→-1）/enter(dl, collectAll)/体/leave；值形尾值提取 + sum{Ok,Err, errMsg:"error: TimedOut"}。select 形：emitSelect——select_new → 每臂 add_recv/add_await（send 源停——黄金无面）→ park → 臂索引链式 icmp → result alloca join（无 phi）→ case 名绑 `__we_select_value`。panic：`panic("...")` → `@.pN` NUL 常量 + `__we_task_fail` + unreachable + pd 续流标签（main 亦是 task 0，一 ABI 双语境）。别名集：pass one 收 Import{std,concurrent} 的 Alias（空则尾段 "concurrent"）；isLocalName 扩含 captures（局部名遮蔽 conc 别名）；io 判定维持宽松形。
  **T6 形修正（披露）**：trySend T6 曾发成 i64 直接值——黄金 `match r4 {Sent/Full/Closed}` 三臂穷尽需要 sum 双槽 → 改 emitSumCall3（tag=返回值、pay 固定 0、variants [Sent,Full,Closed]）。
  **v2 词全套改写（披露）**：m9a_test.go 十例表结构加 abi 字段——bare task statement 停 bndTaskBody（体内 Unit 停 task 词）、let init task/scope value/conc ctor 两形翻绿钉 `__we_task_new`/`__we_scope_enter`/`__we_prim_new_mutex`、其余六形补 ""（m8_test.go 七例与 codegen_test.go 两例 v1→v2 字面量同步）；conformance 四枚边界黄金改写（build-bnd-body `let x: Int64 = 1` 随 T6 数值 let 翻绿 exit 0；build-bnd-m6b-main-body/build-bnd-new-forms/build-ch10-method-call-boundary stderr 钉 v2 词——M9a 预钉的 M8 词退役）。
  **事件（披露）**：T7 中间态一次 python heredoc 路径笔误在 internal/ 落 untracked 旧快照 `internal/codegen.go`（毒化 internal 包名）——确认无 importer 后删除，未进任何提交。

- [x] T8 端到端与黄金：build.go 链接序扩（rt-sched.c/rt-conc.c 写出与编译链接）与 runtime.go embed、T1 黄金全绿（run 族 + 六枚翻绿枚）、其余 469 枚零触碰零回归、全仓 `go test ./...` 绿
  来源：proposal 影响范围 / design D11 D12
  验证：conformance 全绿实际计数记档（预计 ≈505）

  **完成记录（2026-09-07）**：conformance 508/508 全绿（38 枚 run-conc 全绿 + 470 枚既有零回归），全仓 `go test ./...` 9 包全绿。实现缺陷七处（D13 逐条披露）：① emitCallback 在 builder 换向 restore 后才取 e.body.String()——回调文本取到外层 thunk 流、自身 icmp/zext 丢弃（cond-signal `%v9` undefined 根因）→ 快照先于 restore；② emitCompare 两处 `return nil` 吞掉内层边界（L/R 操作数越界时静默续流）→ 改回传 ni；③ emitSumCall payload 落槽直接 store out alloca 指针（ptr 当 i64）→ 补 load；④ task env 块 prim 捕获 store 未 ptrtoint（ptr 当 i64）→ 补 ptrtoint（thunk 侧 loadCapture inttoptr 配对）；⑤ ovf 报文常量长度 [16 x i8] vs 17 字节（"integer overflow\0"）→ [17 x i8]；⑥ emitMatch/emitSelect 首测试分支 `br label %mtestN`/`%sltestN` 指向未定义标签（首 icmp 本就在当前块）→ 删初始分支；match wildcard 臂在上一臂 fall-through 块上二次打同名标签（重复标签）→ wildcard 体直接入当前块；⑦ emitPrimCtor 的 cdesc 全局只写名字不写定义（`@.cdesc0` 裸引用）→ 补 `= private unnamed_addr constant [1 x i64] [i64 1]` 定义。运行塔一处：sched.c task_done 对已取消任务（cancel_flag）不再向 scope 传播 panic——ch18「取消未 await 的任务其 panic 在任务边界捕获并丢弃」（await 仍读存储结果，仅 scope 传播路径跳过）；附带 T3 遗留修正：task_done is_main fail 分支 `exit((int)t->exit_code)`（fail 时恒 0）→ `exit(1)`。
  **黄金修正（D13 随批更新披露，均 T1 预钉预告或源缺陷）**：a) deadlock stderr "2 tasks"→"3 tasks"（T2 harness 已实证 main join 同 park，T1 预钉预告兑现）；b) 8 枚黄金源 match 臂间 `,` 分隔符移除（chan-close-drain/chan-send-recv/chan-tryrecv/gc-park-survival/match-option/scope-timeout-err/scope-timeout-ok/sig-await——规范钉「match 臂换行分隔无分隔符」，E0105 拒收正确，T1 源笔误）；c) sem-blocking 源补 `let _ = t.await()`（E1607 钉 fallthrough 路径须显式 discharge，T1 源违反规范——早退路径才由 scope 出口 discharge 兜底）；d) 四枚边界黄金（T7 记录披露）。其余 464 枚黄金字节零触碰。
  **T5 记录失实修正（T10 成文核对项，此处先记）**：T3 改 startup.c 引 `__we_sched_boot` 后 conformance build 枚即因链接未定义而炸（14 枚意外回归 52 红），T5 记录「38 红均为 T1 目标红」不实——实为 38 目标红 + 14 链接红，T8 扩链接序后 14 枚回归归位。

- [x] T9 真机电池：真二进制综合程序（check 静默 → build → run 双向断言，含 timeout 双向/TaskPanic 边界/死锁 exit 70）、调度交错真机观测、GC 风暴与并发混合长跑
  来源：design D11
  验证：电池脚本输出留存；异常零

  **完成记录（2026-09-07）**：电池脚本 `/tmp/m9b-battery/run.sh`（/tmp 惯例）八项 8/8 全过、零异常，输出留存 /tmp/m9b-battery/：b1 综合程序（while/算术/Mutex update/Channel 收发/match/scope+task+defer/select/多 io——check 静默 → build → run 全链，stdout 七行钉）、b2 GC 风暴（3×gc record 经缓冲 channel 收发 + match）、b3 timeout-err（30ms 真时钟 → "timedout"）、b4 TaskPanic 边界（collectAll 内 panic("boundary") + good 任务 + 外层 "survived"）、b5 死锁（exit 70 + "3 tasks parked"）、b6 调度交错真机观测（p1a→c1a/c1b→p1b 跨 park 序）、b7 timeout-ok（1000ms 预算任务速完 → "task\nok"）、b8 长跑（综合二进制 20/20 连续输出稳定）。
  **电池自身两处修正（非实现缺陷）**：b1 初版 select 双源取空 channel（缓冲已收干）→ 诚实死锁 exit 70——程序缺陷，select 前补 send 使源就绪（与 select-int 黄金形一致）；电池产物名解析改 find executable（build/ 内 .ll/.o 混在，basename 取错）。
  **真机确认**：调度 FIFO 序跨 park 成立（interleave 输出钉）；timeout 双向真时钟（30ms 挂起→取消→协作返回 vs 1000ms 预算内完成）；GC 跨 park 链（channel→buf→record）风暴下零崩溃零错序；死锁面诚实（含 main join 的 3 计数）。

- [x] T10 收尾全量验证：`go test ./...` 全绿 + validate `--all --strict` 过 + docs_sync 对数核对（纯实现零 docs 触碰预期——若触必为披露项）、D13 披露清单逐条核对成文
  来源：design D13
  验证：命令输出留存

  **完成记录（2026-09-07）**：三命令输出留存 /tmp/m9b-t10-gotest.log——`go test ./...` 9 包全绿零失败；`validate --all --strict` "OK: 1 change(s) valid; registry clean; mode=strict"；`docs_sync` "OK: 31 document pair(s) aligned"——纯实现零 docs 触碰，符合预期（无披露项）。**T5 记录失实修正成文**（T5 完成记录尾部已加 T10 修正段：「38 红均为目标红」实为 52 红 = 38 目标 + 14 链接回归，T8 链接序扩后归位）。
  **D13 六条逐条核对**：①四态胞单线程退化直访——已披露（T4 记录「退化面已按 D3 披露」，E1402 纯回调承载互斥，无等待队列）；②malloc 族有界泄漏——已披露（T4 记录等待链/scope 帧/任务结构栈/panic 报文/运行时位图五类不回收，无 finalizer 机制）；③select 取臂 = case 源序——已披露（T4 记录「快路径按 case 序取首就绪源」，unspecified-but-safe）；④死锁 abort——已披露（T3 记录 exit 70 面 + T8 黄金 "3 tasks" 校正实证）；⑤六枚黄金翻绿与 v2 词改写——实际「五枚翻绿 + 一枚维持」（T1 记录 D13-5 口径修正：build-bnd-conc-fn 维持 bndOtherFns 边界钉）+ T7 记录 v2 词全套改写面（m9a 十例/m8 七例/codegen 两例/边界黄金四枚）；⑥实现揭出缺陷——T2 四项测试面修正、T3 两项、T4 三项、T7 两项（trySend 三态 + codegen.go 快照笔误事件）、T8 七项 codegen + 两项运行塔 + 三类黄金源修正（逗号 8 枚/E1607 discharge 1 枚/死锁计数 1 枚），全部逐条披露于各完成记录，无静默绕过。

- [x] T11 审查与归档：welang-change-review 10 点审查（发现即修，披露）、归档 `openspec/changes/archive/2026-09-07-concurrency-run/`（status complete）、roadmap M9b 行翻 done（双语）+ 新发现 follow-up 登记
  来源：流程
  验证：validate `--all --strict` 过；归档同步检查

  **完成记录（2026-09-07）**：10 点 change-review 已于 active 门完成（proposal「审查记录」段，F1 = we_wait_link 补 task_next 账本链字段，当场处置）。本任务落 code-review 7 条（welang-code-review）：通过，记录落 proposal 第三段——七点证据（规范抽查/验证诚实/测试先行/诊断协议/单一权威/红线/dry-run）+ F1 处置。**F1（发现即修）**：T2 完成记录预告的 ABI 定形回填未做——design 补「ABI（T2 定形，T11 回填）」五处：D3 TaskHandle（`__we_handle_await` tag 0/1 + payload 单出参槽双职责——Ok 载荷与 panic 报文指针）、D3 Channel（`__we_chan_send/recv` 0/1 与 1/0 返回形）、D4（`__we_scope_enter(deadline_ms, collect_all)` 两参形、leave 返回 TimedOut 位）、D5（select 五 API：new/add_recv/add_await/park/value，运行时另载 add_send/add_sig 发射面外）、D6（`__we_task_fail(msg)` 任务体 panic 统一入口）；T2 预告落位「D2」为笔误——按机制归属落 D3/D4/D5/D6（D2 通用等待协议无发射面 ABI）。
  **tasks.md 自身修正（披露）**：T8 已勾条目后残留两段重复的未勾选 T8 行（T8 完成记录与 T9 之间的编辑残留）——删除，任务本体与完成记录不受影响。
  **审查期修正**：gofmt -l 揪出 codegen.go 未格式化（T8 编辑后遗留）→ gofmt -w + build 复验 ok；go vet 清；refr/ 零命中；diff 触界与 proposal「涉触码盘点」逐项对齐（modified 15 + 删 5 + 新增 m9b_test.go/33 黄金/sched.h/sched.c/conc.c/两 harness）。
  **归档**：change.yaml active → complete → archived；目录移 `openspec/changes/archive/2026-09-07-concurrency-run/`（零 spec 增量故无规范提升步；无新 ADR——审计第 4 点已裁 ucontext 为实现载体非机制承诺，GC 演进门由 ADR-0003 承载）。
  **roadmap（双语）**：M9b 行 State 翻 `done 2026-09-07`（EN + .zh.md 两文件）；follow-up #11 登记（双语）——运行时结构无回收（malloc 族有界泄漏，ch18 未定析构；design D7、D13 披露 2 的 roadmap 化）。
  **验证**：validate `--all --strict` 复跑通过（归档后）。

- [ ] T12 记忆与提交：project-overview.md 增 M9b 条目 + MEMORY.md 索引行更新、单提交（实现+归档+roadmap，英文消息、无署名 trailer、无 refr/）——等待用户明示「提交」后执行
  来源：M9a 先例 493465b
  验证：提交后面三查（英文/无 trailer/无 refr）
