# We 语言规范 —— 第 18 章：并发


### Requirement: 共享状态类型

标准库在其 `std.concurrent` 模块中声明四个共享状态类型——`Mutex<T>`、`RwLock<T>`、`Atomic<T>`、`AtomicRef<T>`——每个都是第 10 章形式下带一参数的泛型，每个都是第 8 章别名规则下的 gc 类别值：跨任务按引用共享，绝不复制。每个类型恰以一种方式构造——以自己的名字应用于一个初值——`Mutex(0)`、`RwLock(text)`、`Atomic(true)`、`AtomicRef(defaultConfig)`——名字即全部契约：值携带哪个同步纪律从其类型读出，绝不从一个名字背后的构造模式读出（裁决的拆分；v0.8 单名 `Shared<T>` 携方法集随构造变化不继承，其检查跨函数边界非局部可判定）。四者共享一个方法三件套——`fn update(mut self, f: fn(T) -> T) -> T`，持有值之纪律的读-改-写，返回新值；`fn get(self) -> T`，一次读；`fn set(mut self, v: T)`，一次盲写——`RwLock<T>` 另加 `fn read<U>(self, f: fn(T) -> U) -> U`，与其他并发读者共享之锁下的读，其 `U` 依第 10 章单向规则自调用自身文本定出（无定出时 `E0827`）。回调参数是纯函数类型——`fn(T) -> T` 与 `fn(T) -> U`，无效果段——故执行效果的闭包不合，在实参一致位以第 16 章的 `E1402` 拒绝；不存在共享状态特有的纯度码，组合子先例统辖于此。`Atomic<T>` 的参数 MUST 是原子基类型——第 7 章八个整数类型、`Bool`、`Float32`、`Float64` 之一——任何其他实参以 `E1614:` Atomic type argument is not an atomic base type 拒绝；`AtomicRef<T>` 的参数 MUST 是 gc 类别 record 类型，任何其他以 `E1615:` AtomicRef type argument is not a gc record type 拒绝。这些类型的等待不是效果：在这些原语上等待不携带效果标签，即第 16 章经本变更增补的 carve-out。本模块的名字经第 15 章 import 到达——`import std.concurrent` 引入名字 `concurrent`，经它 `concurrent.Mutex` 及同类合格到达；本章的 Requirement 与 Scenario 以裸名书写供阅读，合格拼写才是可达拼写。

#### Scenario: 四个名字各携其纪律

- **WHEN** `import std.concurrent` 在作用域内，注解持有 `concurrent.Mutex<Int64>`、`concurrent.RwLock<Map<String, User>>`、`concurrent.Atomic<Bool>`、`concurrent.AtomicRef<Config>`（`Config` 为 gc record）
- **THEN** 每个都经 import 的名字解析到声明的类型，值上的每个方法调用只对照该类型自身的方法集检查

#### Scenario: update 为整个读-改-写持有纪律

- **WHEN** 两个任务在一个 `Mutex<Int64>` 上调用 `counter.update(|v| v + 1)` 且两个调用都完成
- **THEN** 计数器恰好多二：每个 `f` 都在自己的获锁与释放之间运行，回调内部无交错

#### Scenario: RwLock 读者共享、写者排他

- **WHEN** 多个任务在一个 `RwLock<Map<String, User>>` 上并发持有 `cache.read(|m| m.get("key"))`
- **THEN** 读可并行进行——读锁是共享的——而 `set` 或 `update` 在其期间排除每个读者

#### Scenario: 非纯回调不合

- **WHEN** 出现 `counter.update(|v| save(v))` 且 `save` 声明 `effect io`
- **THEN** 编译器以 `E1402:` function value effect set does not match the expected type's 拒绝——`f` 的参数类型是纯函数类型；在回调之外、以返回值执行效果

#### Scenario: Atomic 拒绝非原子参数

- **WHEN** 出现 `Atomic("name")` 或 `Atomic(users)`，`users: List<User>`
- **THEN** 编译器以 `E1614:` Atomic type argument is not an atomic base type 拒绝；把组合值包进 `Mutex` 或 `RwLock`

#### Scenario: AtomicRef 拒绝非 record 参数

- **WHEN** 出现 `AtomicRef(3)`
- **THEN** 编译器以 `E1615:` AtomicRef type argument is not a gc record type 拒绝；原子基值的类型是 `Atomic`

### Requirement: 对同一共享值的嵌套访问被拒绝

在一个共享值的纪律下运行的回调——该值上的 `update` 或 `read` 的 `f`——MUST NOT 再次直接访问该值：在其自身回调内对同一绑定的 `update`、`get`、`set`、`read` 调用以 `E1613:` nested access to one shared value within its own callback 拒绝。检查是直接的、按绑定的：它看见写在回调体内同一绑定上的调用，不追随调用进入其他函数——经辅助函数的间接嵌套访问此处不被抓到，本规范陈述这一边界而非隐藏它；vet 层启发式调查属工具链章，非本层。一个回调 MAY 访问其他共享值——跨不同绑定的嵌套合法，锁序是程序员的纪律。

#### Scenario: 直接嵌套访问被拒绝

- **WHEN** 在 `Mutex<Int64>` 绑定 `m` 上出现 `m.update(|v| m.get() + v)`
- **THEN** 编译器以 `E1613:` nested access to one shared value within its own callback 拒绝；回调已持有该值——`v` 即是它

#### Scenario: 不同绑定的嵌套合法

- **WHEN** 出现 `a.update(|x| x + b.get())`，`a` 与 `b` 为不同共享值
- **THEN** 访问合法；跨值的次序是程序员的纪律，非本规则的地界

### Requirement: Cond 类型

`std.concurrent` 模块声明 `Cond<T>`，构造时绑定一个 `Mutex<T>` 的条件变量——`Cond(m)`（`m: Mutex<T>`）是唯一构造器，非 `Mutex` 实参在实参位依第 7 章 `E0501` 拒绝：v0.8 允许的与 `RwLock` 配对不继承，读锁上的条件变量以易错著称，单一配对是裁决的表面。`Cond<T>` 是与互斥量共享生命周期的 gc 类别值，跨任务传递安全。其方法：`fn wait(mut self, f: fn(T) -> Bool)`——释放所绑互斥量的纪律、阻塞调用任务、每次唤醒后重新获锁并复查 `f`，在重新获锁后的值上 `f` 成立时返回；释放-等待-重获-复查循环是该操作固定的语义，虚假唤醒对调用者不可见。`fn signal(mut self)` 唤醒一个等待任务（若有）；`fn broadcast(mut self)` 唤醒全部。唤醒是提示不是保证：被唤醒的任务在从自己的 `wait` 返回前复查谓词。谓词是纯函数类型 `fn(T) -> Bool`；效果闭包在实参位是 `E1402`，共享状态纯度规则原样统辖于此。等待不携带效果标签（第 16 章的 carve-out）。

#### Scenario: wait 阻塞至谓词成立

- **WHEN** 消费任务持有 `notEmpty.wait(|q| q.size() > 0)`（一个绑定到互斥量持有队列的 `Cond<List<Job>>`，队列为空），生产者随后添加一个作业并调用 `notEmpty.signal()`
- **THEN** 消费任务的 `wait` 在其重新获锁后的检查看见队列非空后返回；谓词不成立时它不可能返回

#### Scenario: 构造按类型绑定互斥量

- **WHEN** 出现 `Cond(cache)`，`cache: RwLock<Map<String, User>>`
- **THEN** 编译器在实参位以 `E0501` 拒绝——`Cond` 的参数类型是 `Mutex<T>`，rwlock 配对不是本语言的表面

#### Scenario: 非纯谓词不合

- **WHEN** 出现 `notEmpty.wait(|q| log(q) && q.size() > 0)` 且 `log` 声明 `effect io`
- **THEN** 编译器以 `E1402:` function value effect set does not match the expected type's 拒绝；在 `wait` 返回后检查效果

#### Scenario: 被唤醒的任务复查

- **WHEN** 一个任务从 `wait` 唤醒，另一任务已 meanwhile 取走最后一个元素
- **THEN** 重新获锁后的检查看见谓词为假、任务再次等待；唤醒是提示、循环是 `wait` 自己的

### Requirement: Semaphore 类型

`std.concurrent` 模块声明 `Semaphore`，限制并发访问一个资源的计数信号量：`Semaphore(n)`（`n: Int64`）构造一个持有 `n` 个许可的信号量。`n` MUST 是正整数：常量可求值且非正的计数是 `E1616:` Semaphore count is not a positive integer 之下的编译错误（第 7 章溢出先例——编译器能定的，编译器定）；运行时为零或负的非常量计数是第 14 章族的受检陷阱。其方法：`fn acquire(mut self)`——取一个许可，无空闲时阻塞；`fn tryAcquire(mut self) -> Bool`——不阻塞取一个许可，无空闲时 `false`；`fn release(mut self)`——归还一个许可；归还超出构造计数是第 14 章族的运行时 panic，逻辑错误早暴露而不被静默吸收；`fn currentCount(self) -> Int64`——瞬时计数，仅供检视，返回之刻即过时。`Semaphore` 是 gc 类别值，跨任务安全。等待不携带效果标签（第 16 章的 carve-out）。超时路线是 scope 的、不是信号量的：带期限的 `acquire` 是围住它的 `scope timeout(...)`，每个原语一条等待路线（v0.8 立场继承）。

#### Scenario: acquire 限制并发

- **WHEN** 十一个任务各在查询外围持有 `dbLimiter.acquire()` 然后 `defer { dbLimiter.release() }`，作用于一个 `Semaphore(10)`
- **THEN** 至多十个查询同时运行；第十一个 `acquire` 阻塞直到某个 `release` 归还许可

#### Scenario: tryAcquire 不阻塞

- **WHEN** 所有许可被取走时调用 `tryAcquire()`
- **THEN** 立即返回 `false`；调用者保持自己的行动路线

#### Scenario: 常量非正计数是编译错误

- **WHEN** 出现 `Semaphore(0)` 或 `Semaphore(-3)`
- **THEN** 编译器以 `E1616:` Semaphore count is not a positive integer 拒绝

#### Scenario: 归还超额 panic

- **WHEN** 在一个 `Semaphore(2)`（两个许可已空闲）上运行 `release()`
- **THEN** 调用运行时依第 14 章族 panic——归还多于获取是逻辑错误，暴露而不吸收

### Requirement: task 块

task 块是 `task effect tag1 tag2 ... block`——`task` 关键字（修订后的第 1 章）、第 16 章拼写的效果段（`effect` 后一个或多个标签）、一个第 2 章块。它是第 2 章骨架修订下的关键字引导表达式形。段是 REQUIRED 的：task 块运行在自己的任务里、不在外围函数的 extent 内，故其效果没有外围声明可搭乘——块声明自己的，省略以 `E1601:` task block without an effect segment 拒绝。块体内每个调用的效果集 MUST 是声明集的子集，第 16 章的 `E1401` 在任务自身 extent 内裁决；创建任务——求值块表达式本身——不执行任何调用，只有 task 捕获纪律的捕获复制，故外围函数对创建不欠任何效果。task 块的值是一个 `TaskHandle<T>`——来自 `std.concurrent` 的 gc 类别句柄——其中 `T` 是体的块值类型（第 2 章），无值体即 unit 类型；`T` MUST NOT 是资源类型：体的值进入一个句柄，第 13 章 `E1106` 之下的组合位。一个 task 块 MUST 词法上处于某个 scope 块的体内——无处 join 的句柄不合任何产生式的意图，以 `E1618:` task block outside any scope block 拒绝；task 体内可自持 scope 块，任务内嵌套并发。task 体是像闭包一样的函数体语境：`return` 携任务值早退、体内的 `defer` 依第 3 章在其出口运行、`E0401` 的函数体语境经第 12 章修订后的枚举延伸到它。句柄的方法：`fn await(mut self) -> Result<T, TaskPanic>`——等待任务完成并产出其值，任务内 panic 依「任务边界处的 panic」浮出为 `Err(TaskPanic)`；`fn cancel(mut self)`——依「CancelSignal 与 currentCancelSignal」请求取消该任务，不等待。各为一次性：await 或 cancel 消耗句柄，其余由「scope 块」的句柄纪律规则固定。两者只执行等待或协调——无效果标签（第 16 章的 carve-out）。

#### Scenario: 任务声明自己的效果

- **WHEN** scope 块内出现 `let t = task effect net { fetchA() }` 且 `fetchA` 声明 `effect net`
- **THEN** task 块解析、句柄绑定、外围函数自身的段不受触碰——体的效果对任务的声明负责，不对任何外围声明负责

#### Scenario: 无段的 task 块被拒绝

- **WHEN** 出现 `task { fetchA() }`
- **THEN** 编译器以 `E1601:` task block without an effect segment 拒绝；任务运行于所有外围 extent 之外、声明自己的效果

#### Scenario: 段外效果在任务内被拒绝

- **WHEN** 出现 `task effect net { save("x") }` 且 `save` 声明 `effect io`
- **THEN** 编译器以 `E1401:` undeclared effect at a call 拒绝——io 不在任务的声明集内

#### Scenario: 任何 scope 之外的 task 块被拒绝

- **WHEN** task 块出现在无外围 scope 块的函数体内
- **THEN** 编译器以 `E1618:` task block outside any scope block 拒绝；句柄必须 join 一个 scope，而作用域内没有

#### Scenario: 资源类型的任务体被拒绝

- **WHEN** task 块体的最终表达式是一个 `byres` record 值
- **THEN** 编译器依第 13 章 `E1106` 拒绝——值将进入 `TaskHandle<T>`，组合位；先物化内容

#### Scenario: await 产出体的值

- **WHEN** `let t = task effect net { fetchA() }` 运行完成且调用 `t.await()`
- **THEN** 调用产出 `Ok(a)`（`a` 即体的值）；`t` 已消耗，其上第二次 `await` 或 `cancel` 皆不合法

### Requirement: task 捕获纪律

task 块的捕获依第 8 章所有权类别回答自有一套纪律：任务与其创建者并发运行，捕获是可能发生竞态的通道，规则是该事实的裁决闭环。闭包本尊——第 12 章的形式——留在第 12 章的同步纪律下；本要求只统辖 task 块，第 12 章在本变更中的修订以一句话说明此事。基类型、unit 类型、value 类别类型（value record、value sum、元组）的绑定在任务创建时按复制捕获：形状的不可变快照，构造上安全，其内的 gc 元素是自身的同步纪律随行的引用。`var` 绑定 MUST NOT 被捕获，无论类型——以 `E1603:` task block captures a var binding 拒绝——可变性与并发并举按定义即竞态；值放进共享状态类型或先复制到不可变绑定。gc 类别类型的绑定被捕获时 MUST 是同步类型——`Mutex`、`RwLock`、`Atomic`、`AtomicRef`、`Cond`、`Semaphore`、`Channel`、`SendOnly`、`ReceiveOnly`、`TaskHandle`、`CancelSignal` 之一——任何其他 gc 类型、collections 在内，以 `E1602:` task block captures unsynchronized gc state 拒绝，同一码拒绝部件持有 Shareable 集排除之 gc 状态的 value 类别绑定——带 `List` 字段的 record、带 `List` 元素的元组：非同步 gc 状态穿越任务边界即数据竞态，无论何种形状携它穿越，同步类型是方法使共享安全的那些。资源类别绑定 MAY 被捕获，恰当其类型除 `release` 外不声明任何 `mut self` 方法时——任务能竞态的每个方法都是读——检查是对类型声明方法签名的扫描，局部可判定；声明任何其他 `mut self` 方法的类型以 `E1604:` task block captures a resource whose type declares mut self methods 拒绝。捕获不转移资源的释放义务：`release` 仍是第 13 章 scope 出口机制的唯一触发，任务的句柄是引用不是所有权；join 任务的 scope 块位于资源自身的 scope 之内，否则绑定根本不在作用域内。类型提及外围声明泛型参数的绑定 MAY 被捕获，仅当该参数携带 `Shareable` 约束——否则以 `E1605:` task block captures a generic-parameter binding without a Shareable bound 拒绝；资源依第 13 章 `E1106` 永不抵达泛型位，故该约束永不面对资源。

#### Scenario: 基类型捕获是复制

- **WHEN** task 块捕获 `let limit: Int64 = 10` 且外围绑定在任务创建后被重绑
- **THEN** 任务整个生命读 `10`；捕获是创建时刻的复制

#### Scenario: var 捕获被拒绝

- **WHEN** `var total = 0` 在作用域内且 task 块读取 `total`
- **THEN** 编译器以 `E1603:` task block captures a var binding 拒绝；把状态放进 `Mutex` 经 `get` 读

#### Scenario: 裸 gc 捕获被拒绝

- **WHEN** task 块捕获 `let xs = [1, 2, 3]`——一个 `List<Int64>` 绑定
- **THEN** 编译器以 `E1602:` task block captures unsynchronized gc state 拒绝；把列表存进 `Mutex` 或经 `Channel` 发送

#### Scenario: value 形状内的非同步 gc 状态同样被拒绝

- **WHEN** task 块捕获 `let pair = (1, [1, 2])`——第二个元素是裸 `List<Int64>` 的元组
- **THEN** 编译器以 `E1602:` task block captures unsynchronized gc state 拒绝；形状是 value 类别不改变什么，它携带的列表跨任务是活的

#### Scenario: 同步 gc 捕获合法

- **WHEN** task 块捕获 `let m = Mutex(0)` 且其体调用 `m.update(|v| v + 1)`
- **THEN** 捕获合法——`Mutex` 是同步类型且其方法携纪律随行

#### Scenario: 只读资源捕获合法

- **WHEN** `byres` record 类型 `Config` 除 `release` 外只声明 `self` 方法，task 块捕获其 `scope resource` 绑定
- **THEN** 捕获合法：不存在两个任务可竞态的方法，`release` 仍独属 scope 出口机制

#### Scenario: 可变资源捕获被拒绝

- **WHEN** 资源类型声明 `fn bump(mut self)` 且 task 块捕获其绑定
- **THEN** 编译器以 `E1604:` task block captures a resource whose type declares mut self methods 拒绝；把类型限制为 `self` 方法或不要任务

#### Scenario: 泛型捕获需要约束

- **WHEN** 泛型 fn 的 task 块捕获类型 `T` 的绑定且 `T` 不携带约束
- **THEN** 编译器以 `E1605:` task block captures a generic-parameter binding without a Shareable bound 拒绝；声明约束或保持捕获具体

### Requirement: Shareable 标记

`Shareable` 是编译器附加的标记接口：一个类型是 Shareable 恰当它属于封闭集——第 7 章基类型；unit 类型；每个字段类型皆 Shareable 的 value 类别 record；每个载荷类型皆 Shareable 的 value 类别 sum；Shareable 类型的元组；同步 gc 类型（`Mutex`、`RwLock`、`Atomic`、`AtomicRef`、`Cond`、`Semaphore`、`Channel`、`SendOnly`、`ReceiveOnly`、`TaskHandle`、`CancelSignal`）。集合封闭且从声明机械计算；编译器附加标记，别无他者，手写 `impl Shareable for ...` 以 `E1606:` Shareable cannot be manually implemented 拒绝。同步集之外的 gc 类型不是 Shareable——闭包或函数值也不是，闭包能持有静态规则无法从纯一中切分的活 gc 状态；跨任务传逻辑经 `Channel` 或共享状态格，不经捕获的闭包。`Shareable` 不是 derives 目标——第 10 章 derives 集不受触碰，其前指句经本变更修订——且不占任何值表面：它是约束，在 where 子句或声明自身的子句中写作 `T: Shareable`，绝不是参数或箱。名字经第 15 章修订预导入可见——编译器附加的标记是语言级名字、非模块项，泛型声明写不出的约束不成其为约束。

#### Scenario: 约束接纳封闭集

- **WHEN** 泛型 fn 声明 `Shareable` 约束参数并以 `Int64`、Shareable 字段的 value record、`Mutex<Int64>` 应用
- **THEN** 每个应用合法——每个类型的 Shareable 成员资格单凭封闭集成立

#### Scenario: 手写 impl 被拒绝

- **WHEN** 写下 `impl Shareable for Conn {}`
- **THEN** 编译器以 `E1606:` Shareable cannot be manually implemented 拒绝；标记从声明计算，`Conn` 是 Shareable 恰当其自身形状如此

#### Scenario: 闭包不是 Shareable

- **WHEN** 泛型 fn 的 `Shareable` 约束参数以闭包的类型应用
- **THEN** 应用被拒绝——函数值在封闭集之外；改经 channel 发送工作项

### Requirement: CancelSignal 与 currentCancelSignal

`CancelSignal` 是 `std.concurrent` 的 gc 类别类型，任务系统的协作式取消令牌。任务的信号随任务创建、被其每个句柄共享：`TaskHandle` 上的 `fn cancel(mut self)` 标记该任务的信号取消；到期的 `scope timeout` 标记其未完任务的信号；别无他者。信号的方法：`fn isCancelled(self) -> Bool`——非阻塞查询；`fn awaitCancelled(mut self)`——阻塞至信号被取消，select 四等待源的第四个。取消是协作式的、没有任何东西强行中断运行中的任务：被取消的任务持续执行直到它检查自己的信号并返回、或完成；从不检查的任务运行到完成，其取消被信号化但未被应答——边界被陈述而非隐藏，强行中断在本语言中无处存在，它与第 13 章单一释放点和共享状态纪律同样不相容。读信号不执行任何效果（第 16 章的 carve-out）。task 体经 `currentCancelSignal()` 到达自己的信号——依第 15 章在本变更中的修订，预导入的语言级内建名，仅词法上 task 块体内可调用；任何其他位置以 `E1608:` currentCancelSignal called outside a task block 拒绝，静态错误，绝非运行时默认值或空信号。

#### Scenario: 任务轮询自己的信号

- **WHEN** task 体内的循环每轮检查 `currentCancelSignal().isCancelled()` 且句柄的 `cancel()` 于运行中触发
- **THEN** 下一次检查见 `true` 且任务自行返回；检查之前，它在运行

#### Scenario: 从不检查的任务运行到完成

- **WHEN** 任务的体从不触碰自己的信号且其句柄被取消
- **THEN** 任务仍运行到完成；join 它的 scope 等待它——取消是请求，不是中断

#### Scenario: task 之外的 currentCancelSignal 被拒绝

- **WHEN** `currentCancelSignal()` 出现在任何 task 块之外的普通 fn 体内
- **THEN** 编译器以 `E1608:` currentCancelSignal called outside a task block 拒绝；调用方想取消时改取 `CancelSignal` 参数

### Requirement: 隐式获取准则

`currentCancelSignal()` 是本语言唯一从调用语境而非参数获取值的内建，该例外的边界在此机械固定：后续章节 MAY 批准另一个隐式获取，仅经 spec 层变更把候选对照本准则全部四条件展示。一——非业务数据：值是运行时或并发系统自身的控制令牌，不携任何用户定义语义。二——生命周期被迫：值的生命周期绑定于获取它的语法结构；没有办法把它持到该结构之外，也没有办法在结构之内时让它缺失。三——缺失静态可判：结构之外的使用是编译错误，绝非运行时默认或变装的不存在值。四——无竞争路径：不存在到达同一值的显式路线，故没有调用者需要在两种拼写之间选择。`currentCancelSignal()` 通过全部四条——纯控制令牌、绑定 task 块、块外 `E1608`、不存在任务自身信号的构造器或参数形。请求语境按定义不过第一条——它是业务数据——显式参数是它的路线，第 0 章原则不需要第二个机制。

#### Scenario: 准则约束后续候选

- **WHEN** 后续变更提议新的语境获取内建
- **THEN** 其 spec 增补对照本要求文本回答全部四条件，任何一条失败即仅凭本准则构成拒绝理由

#### Scenario: 请求语境不是其中之一

- **WHEN** 提案主张隐式的当前请求访问器
- **THEN** 它过不了条件一——trace 标识与租户是业务数据——显式参数路线是本规范给出的答案

### Requirement: scope 块

scope 块是 `scope block`、`scope timeout(expr) block`、`scope collectAll block`、或 `scope timeout(expr) collectAll block`——`scope` 关键字（自资源变更起第 1 章预留，其复合使用在此批准）、可选的 `timeout` 子句带一个 `Int64` 表达式、可选的 `collectAll` 标记、一个第 2 章块。它是第 2 章骨架修订下的关键字引导表达式形：plain 与 `collectAll` 形的值是体的块值（第 2 章）；`timeout` 形的值在体于预算内完成时是 `Ok(b)`（`b` 即体的块值），预算耗尽时是 `Err(TimedOut)`——标准库命名 sum `pub type TimeoutError = TimedOut` 的唯一构造器——错误由第 14 章普通手段处理，`?`、`match`、或 `let _ =`；丢弃 Result 值的 scope 表达式是第 8 章的 `E0605`。scope 自身的纪律：体内创建的每个 `TaskHandle` 绑定 MUST 在从其创建到体末的每条控制流路径上恰被 await 或 cancel 一次——一条路径上到达体正常完成而未 await 的句柄、或一个句柄上的第二次 `await` 或 `cancel`，以 `E1607:` task handle reaches scope exit un-awaited, or await and cancel both fire 拒绝，分析即第 13 章应用于句柄的单触发点路径分析。体内经 `?` 早退——或经 `return`、`break`、`continue` 穿透它——解除剩余句柄的义务且不构成违例，两种形式说明如何解除：plain 与 `timeout` 形取消它们——fail-fast，第一个错误使整个 scope 撤退——`collectAll` 形让它们运行到完成并在退出完成前 join 它们；形式之间的选择是机械的，第一个错误使余者无谓时 fail-fast、结果全都要时 collectAll，一个准则对一个拼写（第 0 章原则 2）。scope 出口 join 其任务：scope 块只在体内创建的每个任务都完成后才返回，被取消与否皆然——没有任务活过其 scope，嵌套向内同规则——内层 scope 在外层继续前 join 自己的任务，到期外层超时的取消经其信号抵达内层 scope 的任务。超时到期时未完任务的信号被标记、scope 等待其协作返回、值是 `Err`；预算的粒度是运行时的，没有一章承诺界限。scope 边界的等待不携带效果标签（第 16 章的 carve-out）；创建任务也不曾执行任何效果，故 scope 块的整个 extent 不对任何外围声明负责——任务的效果对它们自己块内的段负责。

#### Scenario: plain scope join 两个任务

- **WHEN** scope 体创建两个 fetch 任务、以 `?` await 两者、两者皆成功
- **THEN** scope 的值是两个结果；块只在两个任务都完成后返回

#### Scenario: fail-fast 取消余者

- **WHEN** plain scope 内第一个 `await` 的 `?` 产出 `Err` 且第二个句柄未被 await
- **THEN** `?` 经其信号取消第二个任务后退出 scope；句柄义务由出口解除，任务运行直到它检查自己的信号或完成——穿透体的 `return` 或 `break` 同样解除

#### Scenario: collectAll 等待每个结果

- **WHEN** 同形状运行于 `scope collectAll` 之下且第一个 `await` 的 `?` 产出 `Err`
- **THEN** scope 在错误离开块之前等待第二个任务的完成；完整结果集是该形式的要点

#### Scenario: 到达完成未 await 的句柄被拒绝

- **WHEN** scope 体创建一个任务句柄且在任何路径上未经任何 `await` 或 `cancel` 而完成
- **THEN** 编译器以 `E1607:` task handle reaches scope exit un-awaited, or await and cancel both fire 拒绝；每条路径上 join 它或取消它

#### Scenario: 双重使用被拒绝

- **WHEN** scope 体在同一个句柄绑定上持有 `t.await()` 然后 `t.cancel()`
- **THEN** 编译器以 `E1607:` task handle reaches scope exit un-awaited, or await and cancel both fire 拒绝——句柄一次性，每路径上二者之一

#### Scenario: timeout 包裹体的值

- **WHEN** `scope timeout(500) { fetchRemote() }` 在预算内完成且 `fetchRemote()` 产出一个 `Response`
- **THEN** scope 表达式的值是 `Result<Response, TimeoutError>` 的 `Ok(response)`，由第 14 章普通手段处理

#### Scenario: timeout 到期转 Err

- **WHEN** 同一 scope 的体活过 500 毫秒
- **THEN** 其未完任务的信号被标记、scope 等待其协作返回、表达式的值是 `Err(TimedOut)`

#### Scenario: timeout 子句的表达式是 Int64

- **WHEN** 出现 `scope timeout("500") { work() }`
- **THEN** 编译器以 `E0501` 拒绝——子句的表达式 MUST 是 `Int64`

### Requirement: 任务边界处的 panic

任务内展开的 panic 在任务内遵循第 14 章规则：块退出、scope-resource 绑定各恰一次逆序释放、defer 逆语句序运行、语言的任何表达式不观察这次飞行。任务边界是捕获边界——本规范批准的、除进程中止之外的唯一一个：以 panic 结束的任务不留下进程中止、不留下第二通道；其句柄的 `await` 产出 `Err(TaskPanic)`——标准库命名 sum `pub type TaskPanic = Panicked(String)` 携带 panic 的消息——故 join 的 scope 经第 14 章普通 `Result` 手段遇见该失败，原则 9 的单一机制成立。被取消而无 awaited 结果的任务——其句柄被取消且从未被 await——在同一边界捕获自己的 panic 并弃置之：scope 选择放弃该结果，没有任何路径重新打开它。`main` 或模块初始化器中的 panic 恰如第 14 章所定中止进程；并发不改变那里任何东西。测试章之修订在其自己的位置加入第三个捕获边界：test 块内展开的 panic 止于 test 边界、成为该测试的失败，无进程中止——第 20 章的 Requirement 携带该规则。

#### Scenario: panic 任务在 await 浮出

- **WHEN** 任务体调用 `panic("boom")` 且其句柄在 scope 内被 await
- **THEN** `await` 产出 `Err(Panicked("boom"))`；任务自身的展开先释放了其资源并运行了其 defer，进程没有中止

#### Scenario: 被取消任务的 panic 被弃置

- **WHEN** 一个任务在其句柄被取消且从未 await 的运行中 panic
- **THEN** panic 在任务边界被捕获并弃置——scope 放弃了该结果；没有通道携带它

#### Scenario: main 内的 panic 仍中止

- **WHEN** `panic("boom")` 在任何任务之外的 `main` 内触发
- **THEN** 进程带消息中止，第 14 章规则不变

### Requirement: Channel 类型

`std.concurrent` 模块声明 `Channel<T>`，结构化并发的一等通道：`T` 值的 fifo 队列、有界缓冲。通道由标准库的 `channel(n)` 构造——`n: Int64` 为缓冲容量，`channel(0)` 为无缓冲同步通道——元素类型取自位置的期望类型，注解、参数、或声明的返回，空列表先例；无期望类型位置的构造以 `E1617:` channel construction without an expected type 拒绝，本语言不推断任何东西、也不存在显式类型实参调用形（第 10 章单向规则）。`Channel<T>` 是 gc 类别值，跨任务安全。其方法：`fn send(mut self, v: T)`——入队一个值，缓冲满时阻塞；向已关闭通道发送是第 14 章族的运行时 panic，写后关闭是早暴露的逻辑错误。`fn receive(mut self) -> Option<T>`——出队一个值 `Some(v)`，缓冲空且开放时阻塞；通道关闭且已排空后永久 `None`，接收方的流末。`fn close(mut self)`——关闭通道；接收排空缓冲后永久产出 `None`。`fn trySend(mut self, v: T) -> SendResult` 与 `fn tryReceive(mut self) -> ReceiveResult<T>`——非阻塞形，其三态答案由标准库命名 sum `pub type SendResult = Sent | Full | Closed` 与 `pub type ReceiveResult<T> = Received(T) | Empty | Closed` 携带：成功、现在不、已关闭，不折叠进 `Option` 或 `Result`——那会丢失是两种失败中的哪一种——关闭的 sum 依第 9 章规则像任何其他一样穷尽匹配。发送与接收不携带效果标签（第 16 章的 carve-out）。

#### Scenario: 通道自注解取得元素类型

- **WHEN** 出现 `let ch: Channel<Int64> = channel(4)`
- **THEN** `ch` 是带四格缓冲的 `Channel<Int64>`；期望类型固定了元素类型，不存在其他形式

#### Scenario: 无期望的构造被拒绝

- **WHEN** 出现 `let ch = channel(4)` 而无注解
- **THEN** 编译器以 `E1617:` channel construction without an expected type 拒绝；注解绑定或把通道传到有类型固定它的位置

#### Scenario: receive 产出 Some 直到关闭排空

- **WHEN** 生产者发送 1 和 2 然后关闭，消费者接收三次
- **THEN** 接收产出 `Some(1)`、`Some(2)`、然后 `None`——缓冲已排空、通道已关闭、`None` 永久

#### Scenario: trySend 不阻塞作答

- **WHEN** `trySend(v)` 先在满缓冲上、后在已关闭通道上运行
- **THEN** 答案是 `Full` 与 `Closed`——值两种情况下都未发送，调用者凭变体知道是哪种情况，不凭探测

#### Scenario: 关闭后发送 panic

- **WHEN** `send(v)` 在已关闭通道上运行
- **THEN** 调用运行时依第 14 章族 panic；close 意为发送者已完，越过它写被暴露而不被吸收

### Requirement: 方向通道视图

`SendOnly<T>` 与 `ReceiveOnly<T>` 是 `std.concurrent` 的 `Channel<T>` 视图类型：`SendOnly<T>` 携 `send`、`trySend`、`close`；`ReceiveOnly<T>` 携 `receive`、`tryReceive`。视图经显式转换制成——通道上的 `fn toSendOnly(self) -> SendOnly<T>` 与 `fn toReceiveOnly(self) -> ReceiveOnly<T>`——且绝不经其他：v0.8 的实参位隐式收窄不继承，本规范的无隐式转换规则不破（第 7 章 `E0501`；`Never` 豁免是仅有的那个），显式方法花一个 token，隐式形把转换藏在参数注解里。转换是单向的：没有任何成员把视图变回 `Channel<T>`，对不存在成员的调用依第 10 章 `E0816` 解析，被丢能力之再引入正是视图存在的目的。视图是 gc 类别值，跨任务安全，等待不携带效果标签，对视图或本章任何其他类型的手写 `impl` 依第 10 章 `E0811` 拒绝——内建类型携内建表面，没有手写 impl 产生式拟合它们。

#### Scenario: 通道为生产者显式收窄

- **WHEN** task 块把 `ch.toSendOnly()` 传给声明为 `fn produce(out: SendOnly<Int64>)` 的 fn
- **THEN** 转换是使函数只写意图在签名中可读的那一个 token；fn 之内不存在可调用的 receive

#### Scenario: 错误方向没有成员

- **WHEN** 出现 `out.receive()`（`out: SendOnly<Int64>`）
- **THEN** 编译器以 `E0816:` no such member on the receiver's type 拒绝——视图只携其方向的成员

#### Scenario: 没有回全通道的路

- **WHEN** `SendOnly<T>` 绑定用于期待 `Channel<T>` 之处
- **THEN** 编译器以 `E0501` 拒绝——没有成员、没有转换归还全通道；在源头从通道重新制视图

#### Scenario: 第 18 章类型上的手写 impl 被拒绝

- **WHEN** 写下 `impl Iterable<Int64> for Channel<Int64>`
- **THEN** 编译器以 `E0811:` impl head is not a nominal type 拒绝，第 10 章规则统辖内建头；内建类型携内建表面

### Requirement: select 表达式

select 表达式是 `select { case name = source => body ... }`——两个或更多 case、换行分隔、每个 `case`（修订后的第 1 章）、绑定名或通配符 `_`、等号、等待源、箭头、体表达式；绑定与通配符之外的第 4 章模式臂不拟合任何产生式——变体模式需要一个无规则给予的不匹配政策，且 `case` 绑定源产出的一切、全部。select 表达式是第 2 章骨架修订下的关键字引导表达式形：其值是被取 case 的体值，臂 MUST 依第 8 章一致规则类型一致——`E1610:` select arms disagree in type——语句位的 select 遵循第 8 章丢弃规则（`E0605`），unit 类型体是常见形状，`process(v)` 不丢弃任何东西。等待源恰是四个方法调用之一——`Channel<T>.receive()`、`ReceiveOnly<T>.receive()`、`TaskHandle<T>.await()`、`CancelSignal.awaitCancelled()`——封闭集，别无他者；源位置的任何其他表达式以 `E1609:` select source is not a wait source 拒绝。case 位置超出名字或 `_` 的模式以 `E1611:` select case pattern is not a binding or the wildcard 拒绝；少于两个 case 以 `E1612:` select holds fewer than two cases 拒绝——单 case 的 select 是穿着语法的普通调用。语义：表达式在其源处等待、取一个就绪的 case——数个就绪时取哪一个 unspecified-but-safe：任何选择都是合法执行、没有一个竞态或损坏——把该源的产出绑定到名字、并以该 case 的体求值为值。等待不携带效果标签（第 16 章的 carve-out）；源调用是原语自己的。

#### Scenario: select 让 receive 对完成竞速

- **WHEN** `stop` 已取消且 `ch` 持有值时运行 `select { case v = ch.receive() => process(v) case _ = stop.awaitCancelled() => abandon() }`
- **THEN** 取一个就绪 case——任何一个，两者皆合法——其体以源的产出绑定求值；另一个源的状态未被触碰，臂在 unit 类型上一致

#### Scenario: 非源被拒绝

- **WHEN** select case 持 `xs.get(0)` 为其源
- **THEN** 编译器以 `E1609:` select source is not a wait source 拒绝；等待源是那四个调用，`get` 不在其中

#### Scenario: 臂必须一致

- **WHEN** 一个 case 的体类型为 `Int64` 而另一个的体类型为 `String`
- **THEN** 编译器以 `E1610:` select arms disagree in type 拒绝；给臂一个类型，unit 类型在内

#### Scenario: case 中的变体模式被拒绝

- **WHEN** 出现 `case Some(v) = ch.receive() => v`
- **THEN** 编译器以 `E1611:` select case pattern is not a binding or the wildcard 拒绝；绑定整个 `Option` 在体内 match

#### Scenario: 单一 case 被拒绝

- **WHEN** select 表达式持有一个 case
- **THEN** 编译器以 `E1612:` select holds fewer than two cases 拒绝；直接调用源

### Requirement: 调度承诺

任务系统承诺最终执行、且不承诺更多：scope 体内创建的每个任务最终被调度并运行到自己的终点，没有任务因调度器缺陷永久饥饿；任务执行的次序、并发任务的交错、时间片分配的公平性、信号之后唤醒的及时性全部 unspecified——运行时自己的，没有一章承诺界限。测试章之修订是唯一精化：test 块内调度器是确定性的——一个测试源配一个 `advanceTime` 序列产出一个可观察交错、每次运行相同，该章的 Requirement 固定所承诺的确定性是什么——测试模式之外的每个执行仍在本段的 unspecified 之下。unspecified 是安全的：无论发生何种交错，每个任务自身的执行遵循本规范的单任务规则——第 8 章别名、第 13 章释放、第 12 章闭包——同步类型的纪律是任务之间唯一的交叉，故没有交错破坏本规范陈述的任何规则。

#### Scenario: 每个任务最终运行

- **WHEN** scope 体创建任务
- **THEN** 每个最终被调度并运行；没有一个无轮次地永久等待

#### Scenario: 交错 unspecified 但绝不破坏规则

- **WHEN** 两个任务并发运行于一个 `Mutex` 持有的值上、其 `update` 调用之间夹着无关工作
- **THEN** 交错是可能者中的任何一个；无论发生哪个，每个 `update` 的回调都不可分地运行了——纪律在每个调度下成立

#### Scenario: 测试模式是唯一精化

- **WHEN** 同一个测试——创建任务并以一个固定序列推进虚拟钟——运行两次
- **THEN** 两次运行的可观察交错完全相同；测试模式之外，上述 unspecified 一如既往地主宰

### Requirement: 并发诊断段位

并发章拥有注册表段位 `E1600`–`E1699`，声明于 `docs/spec/diagnostics.toml` 的 `[segments]`。分配：`E1601` task block without an effect segment、`E1602` task block captures unsynchronized gc state、`E1603` task block captures a var binding、`E1604` task block captures a resource whose type declares mut self methods、`E1605` task block captures a generic-parameter binding without a Shareable bound、`E1606` Shareable cannot be manually implemented、`E1607` task handle reaches scope exit un-awaited, or await and cancel both fire、`E1608` currentCancelSignal called outside a task block、`E1609` select source is not a wait source、`E1610` select arms disagree in type、`E1611` select case pattern is not a binding or the wildcard、`E1612` select holds fewer than two cases、`E1613` nested access to one shared value within its own callback、`E1614` Atomic type argument is not an atomic base type、`E1615` AtomicRef type argument is not a gc record type、`E1616` Semaphore count is not a positive integer、`E1617` channel construction without an expected type、`E1618` task block outside any scope block。`E1600` 与 `E1619`–`E1699` 为本章修订保留。触发语义在本章 Requirement 内；条目在注册表。

#### Scenario: 一个并发码被发射

- **WHEN** 工具链发射任何 `E16xx` 诊断
- **THEN** 其完整条目可从 `docs/spec/diagnostics.toml` 以 owner `1800-concurrency` 检索

#### Scenario: 后续变更需要本段位的码

- **WHEN** 后续章节批准需要新并发诊断的规则——transaction 族在其中
- **THEN** 其变更在同一变更内于 `E1600`–`E1699` 内扩展注册表，或声明自己的段位

## 示例（非权威）

下列示例只用第 1–18 章已批准的表面形式。它们是说明性的、非权威的：任何冲突处以 Requirement 与 Scenario 为准。带诊断码注解的行是被拒绝的形式，展示编译器发射的码。并发类型住 `std.concurrent`、经 import 到达——下面用 `import std.concurrent as conc`，别名保持行短；仅 `Shareable` 与 `currentCancelSignal` 预导入可见。

### 共享状态

```we
import std.concurrent as conc

let counter = conc.Mutex(0)                  // the name carries the discipline
let cache = conc.RwLock(text)                // readers share, writers exclude
let ready = conc.Atomic(true)                // atomic base type only
let cfg = conc.AtomicRef(Config { keep: 1 }) // gc record only

let next = counter.update(|v| v + 1)         // indivisible read-modify-write
let seen = cache.read(|s| s.size())          // shared read lock, U from text
ready.set(false)

let a = conc.Mutex(0)
let b = conc.Mutex(0)
a.update(|x| x + b.get())                    // distinct bindings: legal
// a.update(|x| a.get() + x)                 // E1613: nested access to one shared value within its own callback
// let bad = conc.Atomic("name")             // E1614: Atomic type argument is not an atomic base type
// let r = conc.AtomicRef(3)                 // E1615: AtomicRef type argument is not a gc record type
```

### 条件变量与信号量

```we
import std.concurrent as conc

let queue = conc.Mutex(List<Int64>())
let notEmpty = conc.Cond(queue)              // Cond binds a Mutex, by type

let dbLimiter = conc.Semaphore(10)           // positive count, by construction
// let none = conc.Semaphore(0)              // E1616: Semaphore count is not a positive integer

scope {
    let t = task effect net {
        dbLimiter.acquire()                  // at most ten run at once
        pull()
        dbLimiter.release()
    }
    let _ = t.await()
}
```

### task 块与 scope

```we
import std.concurrent as conc

fn fanOut(a: Url, b: Url) effect net -> Result<(Page, Page), conc.TaskPanic> {
    scope {
        let ta = task effect net { fetch(a) }
        let tb = task effect net { fetch(b) }
        let pa = ta.await()?                 // first Err cancels tb's task
        let pb = tb.await()?
        Ok((pa, pb))
    }
}

let both = scope collectAll {                // every result wanted
    let t1 = task effect net { fetch(x) }
    let t2 = task effect net { fetch(y) }
    collectPair(t1.await()?, t2.await())     // waits for t2 even on Err
}

let page = scope timeout(500) {              // Result<Page, conc.TimeoutError>
    fetchRemote()
}
```

### 取消

```we
scope timeout(3000) {
    let t = task effect net {
        defer { markStopped() }              // a task body is a function body
        while !currentCancelSignal().isCancelled() {
            pullChunk()                      // cooperative: it checks, or runs
        }
        done()                               // the task's value
    }
    let _ = t.await()                        // join on every path
}
```

### 通道与 select

```we
import std.concurrent as conc

let ch: conc.Channel<Int64> = conc.channel(4)  // the annotation fixes T
ch.send(1)
let first = ch.receive()                       // Some(1) while values remain
ch.close()
// ch.send(2)                                  // panics at runtime: closed

let out = ch.toSendOnly()                      // explicit, one-way
fn produce(out: conc.SendOnly<Int64>) {
    out.send(7)                                // no receive exists to call
}

select {
    case v = ch.receive() => process(v)        // v binds the Option<Int64>
    case _ = stop.awaitCancelled() => abandon()
}
```

### 被拒绝的形式

```we
// task { fetchA() }                           // E1601: task block without an effect segment
// let xs = [1, 2, 3]
// task effect net { put(xs) }                 // E1602: task block captures unsynchronized gc state
// var total = 0
// task effect net { put(total) }              // E1603: task block captures a var binding
// impl Shareable for Conn {}                  // E1606: Shareable cannot be manually implemented
// scope {
//     task effect net { fetchA() }            // E1607: task handle reaches scope exit un-awaited, or await and cancel both fire
// }
// fn plain() { currentCancelSignal() }        // E1608: currentCancelSignal called outside a task block
// select {
//     case n = xs.get(0) => n                 // E1609: select source is not a wait source
//     case _ = ch.receive() => 0
// }
// select {
//     case a = ch.receive() => a              // arms Option<Int64> and Option<String>
//     case s = names.receive() => s           // E1610: select arms disagree in type
// }
// select {
//     case Some(v) = ch.receive() => v        // E1611: select case pattern is not a binding or the wildcard
//     case _ = t.await() => 0
// }
// select {
//     case v = ch.receive() => v              // E1612: select holds fewer than two cases
// }
// let bare = conc.channel(4)                  // E1617: channel construction without an expected type
// fn top() {
//     task effect net { fetchA() }            // E1618: task block outside any scope block
// }
```

### 待后续变更

```we
// The multi-lock atomic composite (v0.8's Shared.transaction) is a
// registered gap: transfer-shaped work goes through per-value update
// calls under a conventional lock ordering for now. WeakRef and memory
// regions await their own chapters, and the cross-function nested-access
// survey and the may-block note await the tooling chapter's vet layer:
//
// transaction([a, b], |(x, y)| x - 1; y + 1)   // not ratified
```

## 术语对照

本章关键术语，英中对照，供翻译一致：

| English | 中文 |
| --- | --- |
| shared state | 共享状态 |
| shared-state types | 共享状态类型 |
| synchronized type | 同步类型 |
| task block | task 块 |
| task capture discipline | task 捕获纪律 |
| scope block | scope 块 |
| handle discipline | 句柄纪律 |
| wait source | 等待源 |
| cooperative cancellation | 协作式取消 |
| cancellation signal | 取消信号 |
| condition variable | 条件变量 |
| counting semaphore | 计数信号量 |
| channel | 通道 |
| directional view | 方向视图 |
| select expression | select 表达式 |
| select case | select 分支 |
| marker interface | 标记接口 |
| implicit acquisition | 隐式获取 |
| scheduling promise | 调度承诺 |
| task boundary | 任务边界 |
| task panic | 任务 panic |
| function-body context | 函数体语境 |
