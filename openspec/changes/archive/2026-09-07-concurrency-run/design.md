# 设计 — concurrency-run（M9b）

三项表面裁决（proposal 裁决记录）框定本设计的解空间：ucontext 切换、双层直线集发射、登记-挂起-唤醒真 park。本文把三裁决展开到可实现的白盒：数据结构、协议、ABI、边界与测试面。所有运行时符号沿用 M8 的 `__we_` 前缀族。

## D1 调度器核心（runtime/c/sched.c）

**任务结构**（单一定义，sched.c 持有，conc.c 经头文件共享）：

```c
typedef struct we_task {
    ucontext_t        ctx;         // swapcontext 载体
    void             *stack;       // malloc 的独立栈（task 0 复用 C main 栈之外另配）
    we_task_state     state;       // READY / RUNNING / PARKED / DONE
    int               exit_code;   // DONE 时 main（task 0）专用
    // —— 结果与 panic 槽（D6）——
    int               has_result;  // 0 = panicked or cancelled
    we_value          result;      // 任务体返回值（双槽形，D8）
    char             *panic_msg;   // malloc；任务内 panic 的报文
    // —— 取消 ——
    int               cancel_flag; // 协作取消标记（不中断）
    // —— GC root 栈（D7）——
    void            **roots;       // 与 M8 roots[] 同协议，容量 WE_ROOT_MAX
    int               root_depth;
    // —— 等待机器挂点（D2）——
    struct we_wait_link *links;    // 该任务当前挂着的全部等待链节点
    // —— scope 帧（D4）——
    struct we_scope  *scope;       // 创建时的帧
    // —— 就绪队列侵入式链 ——
    struct we_task   *qnext;
} we_task;
```

- **task 0 = main 体**：`startup.c` 的 C `main` 不再直调 `__we_main`，改调 `__we_sched_boot(__we_main)`：为 task 0 建结构（栈另配，与 C main 栈无共享——swapcontext 全对称，main 体从此可阻塞 join）、调度器持自己的 `sched_ctx`、进调度循环。task 0 返回即进程 `exit(exit_code)`（Ok(()) → 0、Err → 既有 stderr 报告 + 1，维持 M8 行为）。main 内 panic 维持 `__we_fail`（ch14：进程边界）。
- **就绪队列**：单 FIFO（侵入式链）。调度循环：`swapcontext(sched_ctx, next->ctx)`；无就绪者走 D2 idle 路径。
- **栈尺寸**：每任务 256 KiB malloc（够直线集 + 深递归回调受限面）；`getcontext` + `makecontext` 指向 trampoline。makecontext 的 int 传参限制：任务函数指针与任务指针经全局交接槽（`__we_cur_boot`）传递，不依赖变长 int 拼装——glibc 弃用警告面用 `-Wno-deprecated-declarations` 消（真机验证）。
- **协作点封闭集**：任务结束、全部阻塞原语（D2/D3）、`__we_yield`（调度承诺「eventual」的显式让出——M9b 不暴露给语言层，仅为运行时内部与 harness 用）。

## D2 等待机器（登记-挂起-唤醒）

**通用节点**：每源一条侵入式 FIFO 等待链；节点由等待方 park 前分配（malloc，不入 GC 堆——见 D7 披露）：

```c
typedef struct we_wait_link {
    we_task           *owner;
    struct we_wait_link *next;     // 同队列链（挂入某源队列）
    struct we_wait_link *task_next; // owner->links 账本链（任务自持全部 link）
    // 源私有尾随数据（Channel 接收节点带目标槽指针等）
} we_wait_link;
```

**协议**（单线程无竞争，每步顺序推理）：

1. **快路径先查**：每源一个 probe（缓冲非空 / 计数 > 0 / 任务已完成 / 已标记取消 / 信号已标记）——命中则不 park 直接走。
2. **登记**：节点挂入源队列；`task->links` 记账（select 多源时逐源各挂一枚——见 D5）。
3. **挂起**：`swapcontext(task->ctx, sched_ctx)`。
4. **唤醒**：唤醒方（跑着的任务或调度器到期路径）把节点摘链、置 `owner->wake_reason`（单值——第二个唤醒者发现已摘链即无事）、任务回 READY。
5. **取值**：被唤醒任务恢复后按 wake_reason 从源取数据（值经源私有槽交接，见各原语）。

**deadline 与 idle**：scope 帧（D4）携带绝对到期时刻（CLOCK_MONOTONIC 毫秒）。就绪队列空且存在 PARKED 任务时，调度器计算最近 deadline：有 → `clock_nanosleep` 睡到该刻再查；无 → **死锁**（无未来事件可唤醒任何任务）——stderr 一行 `we: deadlock: N tasks parked with no wake source` + exit 70（诚实边界行为，非规范表面；ch18「eventual」承诺只对可前进的等待成立）。这是 M9b 的诚实披露面之一（D13）。

**取消交互**：标记 cancel（D4/D3 的 handle.cancel / scope 超时 / fail-fast）时，若目标任务正 park 在可取消源（awaitCancelled 自身、await、Channel 收发），唤醒之并置 wake_reason = CANCELLED——语言层形态由检查层已定（协作返回）。

## D3 六族原语运行时（runtime/c/conc.c）

**对象分配协议**：全部原语对象走 `__we_alloc`（GC 堆，构造协议=安全点不变）。布局沿用冻结头 `{map@0, size@8}`、载荷自 offset 16。

- **Mutex<T> / RwLock<T> / Atomic<T> / AtomicRef<T>**：载荷 = 单个 T 值槽（offset 16）。`update`/`get`/`set`/`read` 直接读写值槽、回调以函数指针 + 环境指针调起（D8 thunk）。**单线程协作推论下的退化披露**：E1402 已保证回调是纯函数（体内无阻塞形），临界区内不可能 park，而非重入调度使「任意时刻恰一任务在跑」——互斥由调度非重入性承载，这些原语不发等待队列、永不阻塞。真锁（多线程化）是 ADR-0003 演进门的既定后续，M9b 不预支。位图：T 为 gc 引用型时 offset 16 置位（编译器按构造位的 T 定，下同）。
- **Cond<T>**：载荷 = 配对 Mutex 指针（引用位）+ 等待链头（链节点 malloc，不计入位图）。`wait(f)`：循环 { 从配对 Mutex 取值 → 调 f → 真则返回；假则 park 到本链 }——**spurious wakeup 对用户不可见**由该循环承载（spec 的谓词循环即运行时职责）。`signal` 唤醒链首、`broadcast` 全唤醒（唤醒后各自重查谓词）。
- **Semaphore**：载荷 = 初始 count（i64）+ 当前 count + 等待链。`acquire`：count > 0 → 减一返回；否则 park。`tryAcquire` 同 probe。`release`：有等待者 → 唤醒一枚（计数不净增）；无 → count++，**超过初始 count → 任务 panic**（D6 机器，spec：release past count panics）。构造实参运行时值非正 → 任务 panic（E1616 的运行时陷阱半边——检查层只盖常量表达式，M9a 已落）。
- **Channel<T>**：载荷 = 容量 k（i64）+ 环形索引 head/count + 缓冲指针 + sendq/recq 两链 + closed 标志。**缓冲是独立 gc 分配**（大小运行时 = 16 + k·slot，k 来自构造实参值；k = 0 时无缓冲分配）——Channel 对象位图对缓冲指针置位。缓冲对象自身位图：T 为 gc 引用型 → 运行时生成全 1 位图（malloc，长度按 size；D7 披露泄漏面）；否则共享零位图常量。**ABI（T2 定形，T11 回填）**：`__we_chan_send(ch, v)` 返回 0/1——0 = 交付完成（rendezvous 直交接或入缓冲），1 = 取消未交付；closed 路径不返回值（任务 panic）。`__we_chan_recv(ch, *out)` 返回 0/1——1 = Some（值经 `*out` 交接），0 = None（close 排空后永久）。
  - `send(v)`：closed → 任务 panic（send-after-close）；k = 0 → 若 recq 有等待者，直接交接值并唤醒之（rendezvous 快路径），否则把 v 先 push 本任务 root 栈再 park 到 sendq（唤醒后完成交接，D7）；k > 0 → 缓冲满则同法 park，否则入缓冲；入缓冲后若 recq 有等待者，直接交接弹出一个。
  - `receive()`：缓冲非空 → 弹出返回 `Some(v)`；closed → `None`；否则 park 到 recq（被 send 方或 close 方唤醒；close 唤醒的收空者得 `None`）。
  - `close()`：置 closed、唤醒 sendq 全部（各自然 panic）、recq 全部（缓冲已先排空——close 语义：先 drain 后永久 None）。
  - `trySend`/`tryReceive`：probe 形返回三态 sum（M9a 已定型的内建 sum；D8 双槽构造）。
- **TaskHandle<T>**：载荷 = 任务指针 + done 标志镜像。`await`：probe 任务 DONE → 取结果槽（has_result = 0 时构造 `Err(Panicked(msg))`，D6）；否则 park 到任务的完成链。`cancel`：置 cancel_flag；若目标正 park 在可取消源，走 D2 唤醒。一次性由检查层 E1607 保证，运行时不复查。**ABI（T2 定形，T11 回填）**：`__we_handle_await(h, *payload)` 返回 tag、`*payload` 单出参槽双职责——0 = Ok（槽写任务结果字）、1 = Err TaskPanic（槽写 panic 报文 C 串指针；发射侧 errPanic 形 inttoptr 后入 `?` 的任务失败尾）。
- **CancelSignal**：载荷 = 标志 + 等待链。`isCancelled` 直读；`awaitCancelled` probe 后 park。标记方（scope 超时 / fail-fast / handle.cancel 的传播目标）经 D2 唤醒。

## D4 timeout 真实时钟与 scope 帧

**scope 帧**（挂调度器，`__we_scope_enter` 建于当前任务）：

```c
typedef struct we_scope {
    struct we_scope *parent;
    we_task_list     children;    // 本 scope 创建的全部任务
    int64_t          deadline;    // absolute CLOCK_MONOTONIC ms; 0 = unbounded
    int              timed_out;   // 到期标记
    int              collect_all; // 形别：plain/timeout = fail-fast；collectAll = join
} we_scope;
```

- **到期路径**：调度器每次从就绪队列取任务前查最早 deadline：已过 → 对该帧标记 `timed_out`、给全部未完子任务标记 cancel 信号（唤醒其可取消 park）、**不中断**——等各任务协作返回（检查点：全部阻塞原语的 probe 链自然携带；无阻塞长计算的「never checks」任务按 spec 跑完，M9b 照实）。此后 join 等待者被唤醒，`__we_scope_leave` 按 `timed_out` 构造 `Err(TimedOut)`（TimeoutError 为 M9a 已登记的内建 sum）或 `Ok(结果)`。
- **fail-fast vs join**：plain/timeout 形的早退（break/return 穿透，M9a 已定检查面）在发射层等价「leave with cancel+join」；collectAll 形 leave 只 join 不标记。粒度（join 是逐任务 await 还是批量）归运行时——实现为逐任务 await 序列（直线发射）。
- **timeout 表达式求值**：`scope timeout(e)` 的 e 在 enter 前求值为 i64 毫秒（e 属直线集标量面，D8）；M10 虚拟钟落此同一读取点。
- **ABI（T2 定形，T11 回填）**：`__we_scope_enter(deadline_ms, collect_all)` 两参形——四形之别由 ABI 承载（plain/timeout 传 collect_all=0、collectAll 传 1；无 timeout 形 deadline_ms=-1），`__we_scope_leave(sc)` 返回 TimedOut 位（0/1）供值形构造 `Result` 双槽。

## D5 select 运行时

- **登记**：`select` 的每 case 源先各自 probe（D2 快路径）；全部未命中 → 逐源登记节点（每源一枚独立 link，同 owner）→ park。任一源唤醒时，唤醒方只摘自己的链节点；被唤醒任务恢复后**先撤销自己其余 link**（遍历 `task->links` 摘链——单线程无竞争）再按 wake_reason 取臂。
- **取臂**：多源同时就绪（probe 命中多枚）取 case 源序首个——「unspecified-but-safe」的实现选择，非规范承诺（D13 披露）。臂内模式仅绑定/通配（E1611 检查层已钉），解构走 D8 双槽。
- **源序保证**：await 作源即触发发（M9a 检查层已钉）——运行时同 await probe。
- **ABI（T2 定形，T11 回填）**：五 API——`__we_select_new()` 建 select 块、`__we_select_add_recv(sel, ch)` 与 `__we_select_add_await(sel, h)` 逐源登记（M9b 发射面 receive/await 两源；运行时另载 add_send/add_sig，发射面外）、`__we_select_park(sel)` park 并返回取中的 case 序号（源序索引）、`__we_select_value(sel)` 读中源交付值。

## D6 TaskPanic 边界

- **任务体 panic**（E0502 溢出陷阱、Semaphore 超额、send-after-close、assert/panic 家族、Channel 运行时陷阱半边）：不 `__we_fail`。路径：任务 defer 栈倒排执行（发射层已按序生成，见 D8）→ `panic_msg` 存槽 → `has_result = 0` → DONE → 唤醒完成链。await 者构造 `Err(TaskPanic::Panicked(msg))`（TaskPanic 为 M9a 已登记内建 sum；msg 为 String 双字）。**ABI（T2 定形，T11 回填）**：`__we_task_fail(msg)` 为任务体 panic 统一入口（noreturn——报文存槽、has_result=0、DONE、唤醒完成链；main 即 task 0 走同一入口，调度器渲染报文并定退出码）。
- **取消任务的 panic 丢弃**：被 cancel 后仍跑到 panic 的任务同样走上述路径；若无人 await（fail-fast 场景典型），结果槽无人读——丢弃即 spec 语义。
- **main 内 panic 维持 `__we_fail`**（进程 abort，ch14 边界）；**回调内 panic**（update 谓词除零等）：回调跑在调用任务栈上——上抛为该任务的 TaskPanic（同栈自然传播）。
- **Panic 报文内存**：malloc；任务槽持有至 await 取走或任务结构回收（D7 披露：任务结构与栈 malloc 后不复用不复.free——进程生命周期一次性分配，泄漏有界于任务总数）。

## D7 GC 交互

- **root 栈任务化**：`gc.c` 的全局 `roots[]/root_depth` 迁入 `we_task`（task 0 含内）；`__we_root_push/pop` 经 `__we_cur_task` 定位。M8 既有调用协议（构造协议 root push 序列）零变化——IR 侧发射的同名调用自然落到当前任务。
- **collect 遍历**：`__we_gc_collect` 扫全部任务结构的 root 栈（含 PARKED——阻塞任务的根全程可达；任务等待期间持有的值（如 send 的交接值，D3）恰由 root 栈承载）。既有 4 MiB 存活风暴探针必须零损重证（T5 回归门）。
- **原语对象与捕获环境块**：原语对象（D3）与 task/回调捕获环境块（D8）均走 `__we_alloc`。位图三源：编译器生成（环境块引用布局、原语 T 引用位）、运行时零位图常量（无引用载荷）、运行时生成全 1 位图（Channel 缓冲，malloc）。等待链节点、scope 帧、任务结构/栈、panic 报文、运行时生成位图——**malloc 族，不回收**（无 finalizer 机制，ch18 未定析构；有界泄漏披露于 D13）。
- **ADR-0003 演进门**：单线程协作下不打开（alloc 触发的收集天然 STW）；真并行时先过该门。M9b 不做分代/屏障。

## D8 codegen 双层直线集

**值表示**（发射器内单一约定，两体共享）：

- 标量：i64 寄存器值（Int64/Bool；Int8–32 经符号扩展入 i64 域运算、E0502 陷阱按窄域判定——`sadd.with.overflow` 族 + 域重检）。Float64 走 double。**M9b 标量算术面诚实范围**：Int64/Float64 字面量与 let/var 绑定、Int64 算术比较、Bool 逻辑；其余基类型可 let 绑定不可运算（bndMainBody 边界，D9）。
- gc 引用：指针（M8 既有）。
- String：双字 {ptr, len}（M8 既有）。
- **sum 双槽**：`{i64 tag, [value_slot × payload_width]}`——alloca 两槽（payload 按载荷型宽度：标量 8 字节 / 引用 8 / String 16 / 嵌套 sum 按展开宽）。tag 为编译器局部序（每 sum 类型在单次发射内一致即可；跨函数交接经 thunk 参数约定）。变体构造写双槽；双臂 match 读 tag icmp 分派、载荷进臂绑定。

**发射器结构**（codegen.go 重写，机器保留：常量池、构造协议、IR 重逃逸、纯函数形）：

- `emitBody`（共享直线集，main 体与 task 体同入口）：语句序列逐条发射——
  1. let/var 标量绑定与算术比较（含 E0502 陷阱发射：overflow intrinsic + 域检查 + 失败转 D6 panic 块）；
  2. String 字面量 / gc 记录构造（M8 机器原样）；
  3. **原语构造**（`conc.Mutex(0)` → `__we_prim_new_*` 族调用 + root push）；
  4. **原语方法调用**（`emitPrimCall` 分派表 → ABI 族；接收者与实参按值表示装填；返回值按调用点期望装槽）；
  5. io 调用（M8 机器）；
  6. `?` 传播（解包双槽，Err 支路 → main 体转 Err 尾返回 / task 体转任务 Err 结束——同一 `emitQuestion` 两语境出口）；
  7. Option/Result 双臂 match（tag icmp + 载荷解构）；
  8. while/if 最小控制流（条件求值 + br 循环/分派块；体内复用 emitBody 递归）；
  9. defer 记录与倒排发射（块出口按记录序倒序内联——直线集内块出口静态可知；while 体内 defer 每迭代出口执行）；
  10. task / scope / select 三形（下条）。
- **task 块**：捕获环境块构造（编译器位图，同 M8 构造协议）→ task 体发射为私有 thunk 函数 `__we_task_thunk_N(env*) -> i64`（直线集第二层——同 emitBody 全集）→ `__we_task_new(thunk, env)` 得句柄。环境块在 thunk 首指令自行 root push（创建者与被创建者之间的 park 间隙由 D7 全任务遍历兜底——创建任务 park 时块仍在其 root 栈；启动后由任务自身持有。交接协议：`__we_task_new` 内创建者保持 root、thunk 入口重 root、成功后创建者 pop——两段覆盖无窗口）。
- **scope 形**：`__we_scope_enter(deadline_ms)`（timeout 形先求值 e；plain/collectAll 形 0/标志位）→ 体（任务创建自然挂当前帧）→ `__we_scope_leave(kind)`（运行时完成 cancel+join 序列并返回双槽 sum 值：`Ok(v)` / `Err(TimedOut)`）。
- **select 形**：逐 case 源求值句柄 → `__we_select_register` 族逐源登记 → `__we_select_park` → 返回臂序号 → switch 分派臂体（臂模式解构双槽）。
- **回调闭包**（update/read/wait 谓词）：体发射为私有 thunk `__we_cb_thunk_N(env*, arg) -> i1`（**回调直线面**：let 绑定 + 纯表达式，控制流/`?`/阻塞形外——谓词 fn(T)->Bool 的类型已天然挡 `?`，控制流显式入边界 D9）→ ABI 调用携 (fn, env)。环境块构造同 task 机器；调用期间 root 于当前任务。

**ABI 函数族**（runtime 侧权威名，declareLines 扩）：调度 `__we_sched_boot/__we_task_new/__we_yield`；scope `__we_scope_enter/__we_scope_leave`；原语 `__we_prim_new_{mutex,rwlock,atomic,atomicref,cond,sem,chan}` + `__we_prim_{update,get,set,read}` + `__we_cond_{wait,signal,broadcast}` + `__we_sem_{acquire,try_acquire,release}` + `__we_chan_{send,recv,close,try_send,try_recv}` + `__we_handle_{await,cancel}` + `__we_sig_{check,wait}`；select `__we_select_{register,park}`；panic `__we_task_panic`；sum 构造 `__we_sum_make`（薄助手，双槽写半可由发射器直发 store——按实现简者取）。

## D9 边界词汇（诚实停点 What 串）

- **bndMainBody v2**（既有词改写，随批披露）：main 体超出「标量 let/var 绑定与算术比较、String 与 gc 记录构造、并发原语构造与方法调用、io 调用、task/scope/select 形、`?` 传播、Option/Result 双臂 match、while/if 最小控制流、defer 倒排、单 Ok|Err 尾返回」之集的语句（发射器 default 兜底，What 串点名具体形——嵌套闭包传值、泛型构造器实参、元组算术、String 运算、for 语句等逐形点名）。
- **bndTaskBody**（新词）：task 体超出同一发射集之形（与 bndMainBody v2 同集不同语境——What 串标明位置；独立成词因用户可据停点定位「task 体内」）。
- **bndCallbackBody**（新词）：回调谓词体内的控制流 / 阻塞形 / 嵌套捕获外之形（回调直线面，D8）。
- **bndOtherFns 维持**：普通 fn 声明整体不发射（Q2 裁决；含 fn 内一切形）。
- **bndTaskTime 维持**：advanceTime/虚拟钟（M10）。

## D10 本变更后仍不可达面（诚实清单，供 review 核对）

1. 虚拟钟、`advanceTime`、确定性调度精化（M10——ch20 carve-out）。
2. 普通函数体发射与一等闭包值传参（fn 声明仍整体 bndOtherFns；闭包仅回调/task 捕获两内联机器）。
3. for 语句、元组算术、String 运算、集合构造的字面发射（无规范障碍，非 ch18 最小充分面——bndMainBody v2 点名）。
4. 真并行与真锁、并发/分代 GC（ADR-0003 演进门）。
5. vet 层 W1911/W1912 跨函数分析（M11）。
6. 资源记录跨任务的运行时义务巡检（ch13 检查层已全落，运行时无新增面）。

## D11 测试策略（先行红）

- **C harness**（runtime/sched_test.go + conc_test.go，仿 gc_test.go：clang 编 C + 跑断言）：调度交错（多任务 FIFO 序）、park/唤醒往返、yield 交接、timeout 真时钟（20–50 ms 级断言带容差）、死锁 exit 70、GC 跨 park 存活（任务持 gc 值 park 后 collect 再唤醒取值）、Semaphore 超额 panic 槽、Channel rendezvous 与 close 排空、select 多源取臂与撤销。
- **codegen 单测**（internal/codegen/m9b_test.go）：发射面逐形 IR 钉（标量算术含溢出陷阱形、原语构造调用、sum 双槽构造与 match 解构、task thunk 签名与 env root 协议、select register/park/switch、`?` 两语境、defer 倒排序）+ 三边界词 What 串钉（bndMainBody v2/bndTaskBody/bndCallbackBody）。
- **conformance run 黄金**：build+run 双断言（stdout + exit code，M4 hello/err 先例扩展到 run 族）——预计 ~30 枚：update/get/set/read 四态、Cond 谓词循环与 signal/broadcast、Semaphore acquire/tryAcquire/超额 panic（await 得 Err(Panicked)）、Channel 定向收发/缓冲满阻塞/close 后 drain/rendezvous/trySend 三态、select 双源就绪取一臂、scope plain join / timeout Ok / timeout Err(TimedOut) / collectAll、TaskPanic await 边界、cancel 协作返回、取消任务 panic 丢弃、GC 存活跨 park、交锁死锁 exit 70。
- **黄金翻绿（随批更新，逐枚披露）**：M9a 六枚 build-bnd-conc-*（check 净 + build 停 bndMainBody）改为全链 run 断言。其余 469 枚零触碰为回归门。

## D12 黄金账目

475（M9a 后）→ 预计 +~30 新增 + 6 枚改写翻绿 ≈ 505（以实现实计为准，tasks 记录实际数）。既有 469 枚零改写是硬回归线。

## D13 披露义务（实现与审查必核）

1. **Mutex/RwLock/Atomic/AtomicRef 单线程退化直访**：互斥由协作调度非重入性承载（E1402 纯回调 ⇒ 临界区无 park），四态胞无等待队列。真锁是 ADR-0003 演进门后续，M9b 明示不预支。
2. **malloc 族有界泄漏**：等待链节点、scope 帧、任务结构与栈、panic 报文、运行时生成位图——无 finalizer（ch18 未定析构），进程生命周期不回收。
3. **select 取臂 = case 源序**：unspecified-but-safe 的实现选择。
4. **死锁 abort**：就绪空 + 无 deadline + 有 parked 任务 → 一行 stderr + exit 70（诚实边界，非规范表面）。
5. **六枚黄金翻绿与 bndMainBody 词汇改写**：随批更新逐枚披露；M9a 十形停点单测同步改写披露。
6. **实现若揭出缺陷**：修复并披露，不得静默绕过（M9a finiteCtors 先例）。
