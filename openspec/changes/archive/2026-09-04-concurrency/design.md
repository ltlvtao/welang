# 设计记录：并发（第 18 章）

## 用户裁决（2026-09-04，四项均采纳推荐）

1. **一章全量**：共享状态 + 结构化并发 + Channel/select 同章（collections 先例：互引用拆两变互相悬空）。预计 ~16R。
2. **拆为独立类型**：`Mutex<T>`/`RwLock<T>`/`Atomic<T>`/`AtomicRef<T>`——构造模式入类型名，签名即全部真相（P1 完整）；性能意图（读多写少/无锁）显式于类型；`Cond<T>` 仅配 `Mutex<T>`（rwlock+条件变量以易错著称，偏离 v0.8 披露）。否决 v0.8 单名 `Shared<T>` + 构造溯源（跨函数后 E0753 类检查不可局部判定，P1 破口）与统一方法集（丢 rwlock 读并行与无锁档位）。
3. **Ref<T> 终身不引入**：单任务可变 = ch8 `var` + ch10 `mut self`，一个事实一个权威位置（P5）；E0701 随之消亡。
4. **等待零效果标签**：等待不是环境读写；ch16 增补 carve-out；W0755/W0756 属 vet 层延后工具链章。

## 关键设计决定（超出裁决的实现面取舍）

- **D1 关键字**：+`task select case timeout collectAll`（`scope` 已预留，ch1 原文已预言「其预留先于后续复合形式使用」）。34→39 词，破坏性照录 ch1 预授权机制。零软关键字裁决下，产生式固定位的字面 token 必须是关键字。`collectAll` 保留 v0.8 驼峰拼写（关键字非标识符，不受 E0013 命名约束）。
- **D2 task 块 = 必写效果段的表达式形**：`task effect tag1 tag2 ... block`——第 16 章声明侧拼写（`effect` 关键字 + 裸标签）原样进表达式位。闭包的效果集靠期望类型推断（ch16 R4），task 块无期望语境——按 ch16「声明必写段」的立场，段必写（E1601），体内调用 ⊆ 段（E1401 复用）。偏离 v0.8 `task { effect net; ... }` 首行拼写（块体首行不是段位）。
- **D3 await 返回 Result**：`fn await(mut self) -> Result<T, TaskPanic>`——ch14:118 预承诺「任务边界成为捕获边界、unwinding panic 转 Err」的直接推论。`pub type TaskPanic = Panicked(String)`、`pub type TimeoutError = TimedOut`（命名 sum，ch14 E1204 合规；载荷位置式，ch9 裁决）。`scope timeout(m)` 返回 `Result<T, TimeoutError>`。
- **D4 句柄纪律 = 单触发点分析**：每句柄每路径恰一次 await 或 cancel（E1607 单码分项枚举：正常完成路径未处理 / 二次使用）——ch13 E1104 路径分析先例。`?` 早退出路径由 scope 出口语机接管：默认 scope 自动 cancel 余下、collectAll 等待余下完成再传播——两种语义的机械分界（存在 fail-fast 依赖→默认；需完整结果集→collectAll）。
- **D5 select = 表达式**：镜像 match（臂一致 E1610、语句位丢弃走 E0605、臂换行分隔）。case 模式仅 `name` 或 `_`（E1611）——变体模式需「不匹配怎么办」的政策，未定义行为不留口；≥2 case（E1612）。就绪分支选择 unspecified-but-safe（v0.8 §36.5 继承）。
- **D6 方向视图显式转换**：`Channel<T>.toSendOnly()/toReceiveOnly()`——非 v0.8 实参位隐式收窄。本规范 E0501 无隐式转换是绝对立场（Never 是唯一豁免；v0.8 依赖的另一处隐式转换 ?-自动提升已在错误章被否决），显式方法零损失。逆向转换不存在→E0816 复用；视图手写 impl→E0811 复用（内建头）。
- **D7 channel 构造走期望类型**：`channel(n)` 无调用位类型实参（无 turbofish，ch10 裁决），T 自注解/形参/返回位期望取得，无期望 = E1617（E1501 空列表同型）。`channel(0)` 无缓冲。
- **D8 位置实参**：v0.8 命名实参调用（`Semaphore(maxCount: 10)`、`channel(bufferSize: 4)`）不合 ch2/ch6 已批语法——全部位置化，`Semaphore(10)`、`channel(4)`。
- **D9 Semaphore 正数检查**：常量可求值→编译错（E1616，E0502 先例：编译期能定的编译期报）；非常量→运行时 checked panic（ch14 族）。release 超额 = 运行时 panic（v0.8 立场：程序逻辑错误尽早暴露）。
- **D10 嵌套访问**：同一共享值自身 update/read 回调内的直接嵌套调用拒绝（E1613，按绑定直接可达路径，局部可判定）；跨函数间接路径不追踪（W0755 → vet 层，规范如实记界）。
- **D11 Shareable 封闭集**：基类型；value 组合（字段递归 Shareable）；value sum（载荷递归）；同步类型（四锁/Cond/Semaphore/Channel/两视图）；unit 与 Shareable 的元组。**不含**：gc 非同步类型（含 List/Map/Set——裸捕即 E1602，共享必经同步类型，与 v0.8 E0702 对 gc 的一般立场一致）、闭包/fn 值（闭包可持活 gc 状态，无静态切分）、resource（ch13 F2 已禁入组合位，泛型位遇不到）。编译器自动附加，手写 impl = E1606；非 derives 目标（ch10 悬句预设修正，改写披露）。**审查修正**：E1602 消息由「gc binding not of a synchronized type」放宽为「captures unsynchronized gc state」——value 形状（含 List 字段的记录、含 List 元素的元组）携非 Shareable gc 跨任务边界同样是竞态，原码盖不住（10 点审查 F4）。
- **D12 currentCancelSignal = 语言级预导入名**：与 panic 族同级（语法邻接：仅在 task 块内有意义），非 std.concurrent 模块项（「仅在块内合法」的检查不适配导入函数）。块外调用 E1608（§35.3.1 条件 3：缺失场景静态可判定）。§35.3.1 四条件准则全文入章——隐式获取的封闭边界，未来候选逐条对照。
- **D13 panic × 取消**：await 边界捕获；被 cancel（未 await 结果）的任务内 panic 同样在边界捕获后弃置（scope 已选择放弃其结果）——无第二通道（P9）。main/init 内 panic 仍进程中止（ch14 不变）。
- **D14 task 块合法位**：必须词法处于某 scope 块体内（E1618）——句柄无处 join 的 task 块不合产生式意图；task 体内可再开 scope（嵌套并发）。顶层初始化器遇 E1405 纯函数约束在前，纯 task 亦无 scope 可 join，同码拒绝。
- **D15 v0.8 码映射**：E0701→消亡（Ref 拒）；E0702→E1602；E0703→E1603；E0733→E1604；E0704→E1605；E0705→E1606；E0752→E1614；E0753→消亡（拆类型）；E0754→E1613；E0755→消亡（Cond 单配）；E0756→E1616（静态支）；E0757→E1615；E0758–E0761→延后（transaction 族）；E0801→E1607；E0810→E1610；E0811→E1609；E0812→E1608；E0813→E0816 复用；E0814→E0816 复用；E0815→E0811 复用。E0731/E0732（region/WeakRef 族）延后。终局 18 码 E1601–E1618（E1600 + E1619–E1699 段内预留）。
- **D16 延后清单及理由**：WeakRef（GC 生命周期规范未立）；region（内存区域分配故事未立）；Shared.transaction 多锁原子复合（v0.8 设计依赖单名 Shared 家族的异构列表 `[a, b]`，拆类型后同 T 可用 `Mutex<T>` 列表但异构组合需新表面设计——登记缺口，转账场景以「按约定锁序逐个 update」过渡）；W0755/W0756（vet 分析，工具链章）；同作用域锁序静态检查（E0761 同类，与 transaction 一起回来）。
- **D17 task 体 = 函数体语境**（审查 F1 修正）：defer 仅 fn-body 直属项合法（ch3 E0204）、而 Semaphore 场景与 panic 展开都依赖 task 体内的 defer/return——照 ch12 闭包先例把 task 体定为第三个函数体语境（`return` 携任务值早退、defer 于体出口、E0401 语境），枚举权威仍一处：ch12 R6 的语境清单经本变更修订纳入，ch18 R5 引用之。scope 体不是函数体语境（嵌套块同 ch3 拒绝）。
- **D18 名字到达**（审查 F2/F3 修正）：import 语义 = 引入模块名单一名字（ch6 R2），故 `import std.concurrent` 后成员一律合格到达（`concurrent.Mutex`、`concurrent.channel(4)`、`concurrent.TaskPanic`）；示例用 `as conc` 别名保持行宽；章内 Requirement/Scenario 文字以裸名书写并R1 声明阅读约定。`Shareable` 与 `currentCancelSignal` 为语言级名入预导入（ch15 修订）：前者是编译器附加的约束标记、写不出即无约束可言，后者语法邻接（块外 E1608 的检查不适配导入函数）。其余类型名不入预导入（闭式预导入纪律，非目标照录）；`channel`/视图非「builtin」措辞——就是 std.concurrent 模块项。

## P1 复核

- 拆类型后方法集随类型名走：签名即真相，跨函数零损失。
- 句柄纪律/单触发：路径定值分析（ch13 先例），过程内可判定。
- 捕获四规则：类型检查（gc 非同步/var/泛型约束）+ 方法签名扫描（resource mut-self），全静态局部。
- select 等待源：封闭类型集上的成员签名匹配。
- Cond 配对：构造参数类型即约束，错配 E0501（无需新码）。
