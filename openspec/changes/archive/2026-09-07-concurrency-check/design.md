# design — concurrency-check（M9a）

白盒决策。规范引注全部指向 `docs/spec/1800-concurrency.md`（已批权威）；行为面零发明——每个接受/拒绝判定必须能指回 ch18 Requirement 或既有章节。

## D1 parser 面：三产生式与三 AST 节点

- **节点**：`TaskExpr{Line, Col, Tags []string, Body Block}`、`ScopeExpr{Line, Col, Timeout Expr, CollectAll bool, Body Block}`、`SelectExpr{Line, Col, Cases []SelectCase}`、`SelectCase{Line, Col, Wildcard bool, Name string, Source Expr, Body Expr}`——全部 `ast.Expr`（ch2 关键字先导表达式形，If/Match 先例；`If`/`Match` 的既有结构照抄风格）。
- **task 产生式**（语句位 :1651 与表达式位 :2927 的 bndConc 删）：`task` → 若下一 token 非 `effect` → **E1601 于 task 关键字**（parse 期—— omission 是纯语法事实，M6b `?` 先例）；`effect` 后 ≥1 个标签（ch16 段拼写既有 parseEffectTags 复用）→ block。task 是表达式：`let t = task effect net { ... }`；语句位 = 表达式语句。
- **scope 复合产生式**（:1985 bndConcScope 删，`scope resource` 分派保留在前）：`scope` → 可选 `timeout(` expr `)`（顺序固定：timeout 在 collectAll 前，ch18:231 四形枚举）→ 可选 `collectAll` → block。timeout 子句表达式的 Int64 检查归检查层（E0501 既有机器）。
- **select 产生式**：`select` `{` 换行分隔的 case 臂（Match 臂先例）→ `case` 后解析模式：**仅绑定名/`_` 合法，其余模式 → E1611 于 case 首 token**（parse 期——「a variant pattern would need a no-match policy no rule gives」）；`=` expr `=>` 表达式体；`}`。**<2 臂 → E1612 于 select 关键字**（parse 期计数）。
- E0105 意外 token 清单（:1655）**不变**：`case`/`timeout`/`collectAll` 仍无独立语句产生式——它们是 scope/select 产生式内部的位置关键字（先导 `scope`/`select` 消费后进入），语句位裸写 `case ...` 依旧 E0105。
- **锚点裁决**：E1601/E1611/E1612 parse 期发射（语法事实）；E1618 检查期（词法包含需 AST 祖先遍历）；其余 ch18 码全部检查期。与注册表 description 逐字对齐由黄金钉。

## D2 std.concurrent 装载与内建类型（List/Map 路线）

- **模块文件**：`StdModule("std.concurrent")` 合成 `*ast.File` 载**四命名 sum 真声明**（`TimeoutError = TimedOut`、`TaskPanic = Panicked(String)`、`SendResult = Sent|Full|Closed`、`ReceiveResult<T> = Received(T)|Empty|Closed`——全为可 We 书写的普通 sum，走同一 ingest/checkModule 机器，M8「同机受检」先例）+ **`channel` 的标记 FnDecl**（`pub fn channel(n: Int64) -> ?`——体空标记；其定型不走该声明而走 D2 末尾的期望驱动路径，声明只为名字可达与 pub 面存在）。注册表单例（包级 var）保证跨次调用 AST 指针同一——sameType 指针键机器（M6b）的前提，M8 std.io 同形。
- **11 内建类型走 checker 侧注册**（List/Map/Set 的 sumInfo + collectionMembers 先例，非合成声明）：`concurrentTypes` 名义类型注册（泛型参数表 + gc 品类 + 封闭成员集）。合成文件**不载**这 11 类——它们不可 We 书写（成员面是内建表面，ch18:323「the builtin types carry builtin surfaces」），`impl ... for Channel<Int64>` 落既有 E0811（impl 头非名义类型——List/Map 今日同路）。
- **限定解析**：`import std.concurrent as conc` 引入名 `conc`；`conc.Mutex<Int64>` 的 NamedType 头经 importSym 到模块后查 `concurrentTypes`（stdlib-first 成员走查先例 :5584）。裸 `Mutex` 无 import 时 E1304（bareUnresolved）——规范「the qualified spellings being the reachable ones」。`conc.channel(4)` 的调用头解析到模块标记项后转内建构造路径。
- **成员集**（签名纯函数型——E1402 织入走既有 checkFnSlot，零新机器）：Mutex/RwLock/Atomic/AtomicRef 四型共享 `update(mut self, f: fn(T)->T)->T`、`get(self)->T`、`set(mut self, v: T)`；RwLock 加 `read<U>(self, f: fn(T)->U)->U`（U 调用文本定，E0827 既有）；Cond：`wait(mut self, f: fn(T)->Bool)`、`signal(mut self)`、`broadcast(mut self)`；Semaphore：`acquire(mut self)`、`tryAcquire(mut self)->Bool`、`release(mut self)`、`currentCount(self)->Int64`；Channel：`send(mut self, v: T)`、`receive(mut self)->Option<T>`、`close(mut self)`、`trySend(mut self, v: T)->SendResult`、`tryReceive(mut self)->ReceiveResult<T>`、`toSendOnly(self)->SendOnly<T>`、`toReceiveOnly(self)->ReceiveOnly<T>`；SendOnly：`send/trySend/close`；ReceiveOnly：`receive/tryReceive`；TaskHandle：`await(mut self)->Result<T, TaskPanic>`、`cancel(mut self)`；CancelSignal：`isCancelled(self)->Bool`、`awaitCancelled(mut self)`。等待零效果标签 = 签名纯——ch16 carve-out 的落形。`SendResult`/`ReceiveResult`/`TaskPanic` 的返回引用经模块内合成 sum 解析（指针同一）。
- **构造面**：`conc.Mutex(0)`/`RwLock(e)`/`Atomic(e)`/`AtomicRef(e)`/`Cond(m)`/`Semaphore(n)`——限定名构造走内建构造路径（builtinCtorCall 族扩展），T 从实参定型（Atomic/AtomicRef 另有 D8 实参码）；`conc.channel(n)` **期望驱动**（空列表先例）：期望型的 Channel<T> 实参定 T，无期望 → **E1617**。
- 被拒替代：合成 record+impl 真声明（体不可书写——运行时原语 M9b 才有，必然造特权体或空体走查两难）；规范改动（零增量已裁决）。

## D3 Shareable 标记：闭集机械计算 + bound 分派

- **成员判定 `shareableOf(Type) bool`**（自 :3164 名单的 Shareable 行改真分派）：ch7 八整数型 + Float32/Float64/Bool/String/Rune/Bytes 基础型；unit；value 记录（全字段 Shareable）；value sum（全载荷 Shareable）；tuple（全元 Shareable）；**同步 gc 十一类**（`Mutex/RwLock/Atomic/AtomicRef/Cond/Semaphore/Channel/SendOnly/ReceiveOnly/TaskHandle/CancelSignal`——型自身在集，实参不再下钻：Atomic 实参本就限原子基型）。闭包/fn 型不在集（ch18:179「a closure is not Shareable」）；非同步 gc（List/Map/gc 记录）不在集；resource 型不在集（捕获走 E1604 自己的门）。
- **bound 位**：where 子句/声明子句的 `T: Shareable` 今日 E0829 → 真分派走 `shareableOf`（checkWhereSatisfies 的 implements 判断加 Shareable 特支——它无方法集，是纯成员判定）。prelude 可见 = bound 解析不查 import（panic 族先例）。泛型体内以 `T`（带 Shareable bound）为型的绑定，成员判定 = bound 携带（M6a bound 授集机器）。
- **E1606**：`impl Shareable for Conn {}` —— impl 头解析后，头名是 Shareable → E1606（检查期，锚 impl 头）；其余 ch18 型作 impl 头维持 E0811。
- **E1605**：捕获纪律的泛型门（D4），非 bound 位自身。

## D4 捕获纪律：task 体自由变量 × 品类

- **分析对象**：task 体 AST 的自由标识符，解析到**task 块外**的绑定（fn 参数/外围体 let/var）——模块级名（pub fn/类型/常量）不是捕获（调用与引用非状态携带）；解析到 task 体内绑定的不算。收集走新 `taskCaptures(TaskExpr) []binding`（自研轻量 walk，落 resource.go——ch13/ch18 纪律层单文件；不复用 ch12 闭包捕获机器：ch18:135 明文分治「this requirement governs task blocks alone」，闭包按引用+同步纪律，task 按品类纪律，合并会让两纪律互相污染）。
- **判序（每捕获，规范序）**：① `var` 绑定 → **E1603**（无型视——「whatever its type」）；② resource 品类 → 类型声明方法签名扫描：`release` 外任何 `mut self` → **E1604**；③ 型提及无 Shareable bound 的外围声明泛型参数 → **E1605**；④ `shareableOf` 不成立 → **E1602**（覆盖裸 gc 与 value 形状内部非同步 gc——「a record with a List field, a tuple with a List element」；同步 gc 捕获合法）。
- **resource 释放义务不转移**（ch18:135 尾句）：捕获纪律只查方法可竞态面，release 单触发仍在 scope resource 机器——E1104 既有面零触碰。
- 锚点：捕获名首次使用位（task 体内）；报文用注册表逐字。

## D5 句柄纪律：单触发路径分析（M6b walk 改造）

- **对象**：scope 体内**一切 TaskExpr 出现位**——let 初始化位（绑定名入路径分析）与裸表达式语句位（规范示例「scope { task effect net { fetchA() } } → E1607」即裸形：句柄被弃置 = 任何路径都未消费的极端形，直接 E1607 于 task 关键字；panic-stop 使其先于 E0605 弃置报文浮现——与规范示例的码选一致）。别名绑定（`let u = t`）不入分析（规范字面「every TaskHandle binding created in the body」指创建位，边界照字面并在 D10 披露）。
- **路径状态机**（resource.go 结构化活跃性 walk 同骨架，kill 语义改造）：消费 = 该绑定上的 `await()`/`cancel()` 方法调用（Member 调用头 Recv 解析到该绑定）；**正常路径到体尾未消费 → E1607**；**同路径双消费 → E1607**（一码双触发，规范原句「or await and cancel both fire」）。早出口（`?`/`return`/`break`/`continue` 穿透）discharge 剩余句柄——检查层只验证「每路径 ≤1 次且非早出口路径恰 1 次」，取消 vs join 的运行时差异（plain/timeout 取消、collectAll join）是 M9b 语义，检查面无差别。
- **环保守性**（M6b 先例同形）：while/for 体内创建+同迭代消费合法；跨迭代持有（环内创建、环后消费）——零迭代路径并入环后 join 后判 E1607；黄金只锁直线索族，半形以单测补证披露。
- 嵌套 scope：内层 scope 的句柄归内层分析（walk 按最近 enclosing scope 分帧）；task 体内的 scope 同理（task 体是函数体语境）。
- 锚点：task 块的 `task` 关键字（创建位）。

## D6 task 块检查：extent、语境、体值

- **型**：`TaskExpr` 型 = `TaskHandle<T>`，`T` = 体块值型（ch2 块值——BlockExpr 既有定型路）；无值体 T=unit。**T 为 resource 品类 → E1106**（既有 catOf 位，复合位置禁入——ch18:101 明文引）。
- **E1618**：fn 体/顶层 walk 维护「enclosing scope 块体」深度（ScopeExpr 入 +1 出 −1，闭包体穿透计入——词法包含）；task 于深度 0 → E1618（锚 task 关键字）。
- **效果 extent**：体 walk 期间 `bodyTags` 换 task 声明集（M7 语境协议五字段——闭包推断分支不走：task **声明**不推断）；体内调用 E1401 对 task 集；**TaskExpr 自身对外围 extent 零贡献**（「creating the task performs no calls, only the capture copies」——外围 bodyTags 不含体调用）。
- **函数体语境**：task 体并入 E0401 语境枚举（return 携值 = 体值早出口；defer 于任务出口运行——检查面 defer 语境既有机器）。`currentCancelSignal()` 于 task 体内 → `CancelSignal` 型（prelude 名，panic 族先例）；体外（含闭包内？**否**——「callable lexically inside a task block's body only」，闭包嵌在 task 体内算 lexically inside，深度计数穿透闭包）→ **E1608**（锚调用名）。
- **E1401 在 task 集内的锚** = 既有 calleeSite 规则零改。

## D7 scope 块定型与 select 定型

- **scope 值**：plain/collectAll 形型 = 体块值型；timeout 形（含 timeout+collectAll）型 = `Result<T, TimeoutError>`（T = 体块值型；TimeoutError 经模块合成 sum——`Result` 既有泛型实参机器）。timeout 子句表达式 `Int64` 非 → **E0501 既有**（锚子句表达式）。scope 块体是新作用域（let 入体帧）；体是函数体语境（return/break/continue 穿透语义归 D5 discharge）。**丢弃纪律**：scope 表达式非 unit 值的语句位弃置 → 既有 E0605。
- **select 定型**：先定型 source（Member 调用既有路），再判**四等待源闭集**（接收者型 × 方法名）：`Channel<T>.receive()`/`ReceiveOnly<T>.receive()` → 绑定 `Option<T>`；`TaskHandle<T>.await()` → `Result<T, TaskPanic>`；`CancelSignal.awaitCancelled()` → unit。非闭集 → **E1609**（锚 source 表达式）。臂体定型无期望（块尾 if/match 同位先例），**臂型不一致 → E1610**（锚不一致臂首）；一致者 = select 值型。绑定名入 case 体作用域（`_` 弃置）。语句位非 unit → E0605 既有。
- **E1613 嵌套访问**：`update`/`read` 实参闭包体内，Recv 为 Ident 且解析到**同绑定**的 `update/get/set/read` 成员调用 → E1613（锚内层调用头）。「直接、逐绑定、不追进其他函数」照字面（ch18:39 自述边界）。

## D8 杂项码接线

- **E1614/E1615**：`conc.Atomic(e)`/`conc.AtomicRef(e)` 构造位——T 从实参定型后判：非原子基型（ch7 八整型 + Bool/Float32/Float64）→ E1614；非 gc 记录 → E1615（锚构造名）。
- **E1616**：`conc.Semaphore(n)` 实参**常量可求值**且非正 → E1616（锚实参）。常量面 M9a 收窄为：整字面量与一元负整字面量（算术常量折叠不引入——「what the compiler can decide, it decides」的最小诚实读法，更广的常量面待真需要时随披露扩）；非常量位的运行时陷阱归 M9b（D10 记）。
- **E1617**：D2 末（期望驱动路径的无期望位）。

## D9 codegen 边界：零新发射，逐形核验停点

- M9a 后 `we build` 对并发形的停点**全部既有行**：fn 含并发形 → bndOtherFns（未触）；main 体语句位 ScopeExpr/TaskExpr → emitStmt ExprStmt 要求 Call → **bndMainBody**；let init TaskExpr → Binding init 三型外 → **bndMainBody**；`conc.Mutex(0)` 构造 → emitConstruct records 查无 → **bndMainBody**；io 实参位任并发表达式 → emitStringExpr default → **bndMainBody**。`import std.concurrent` 段 → M8 std 擦除路径（Path[0]=="std"）零改。新增 Expr 节点在 codegen 各 switch 的 default 停点由单测逐形钉（防「新节点静默漏过」——M4「显式 case，never vanish」纪律）。

## D10 不可达清单（诚实边界枚举，真机电池逐条实证）

1. 调度与交错（运行序、让出点、唤醒及时性）——无运行面（M9b）。
2. 六族原语运行时行为（锁互斥、Cond 唤醒循环、Semaphore 陷阱、Channel 缓冲/关闭语义、send-after-close panic）。
3. timeout 的真实时钟到期与 `Err(TimedOut)` 运行值；E1616 非正计数的运行时陷阱半边。
4. TaskPanic 边界捕获（await 的 `Err(Panicked(msg))` 运行值；取消任务的 panic 丢弃）。
5. select 的运行时取臂（「unspecified-but-safe」）。
6. `advanceTime`/虚拟钟/测试模式确定性（M10——bndTaskTime 维持）。
7. transaction/WeakRef/内存区域（无已批面）。
8. vet 层跨函数嵌套访问巡检（tooling 章）。

## D11 今日行为基线（翻绿面与测试更新预测）

- 消失的边界：bndConc（parser 双位）、bndConcScope、bndShareable（三处）、bndTaskTime 的 currentCancelSignal 半边；bound 位 Shareable 的 E0829。
- 既有测试更新面（随批，逐处披露）：parser_test.go :201-203 常量与 :711 族（bndConc/bndConcScope 锁行改钉真行为）、m6b_test.go :76-81（scope 复合形锁行）、typecheck_test.go :311-313（Shareable/bndTaskTime 锁行——:313 advanceTime 行**维持**）。
- conformance 既有 431 枚零触碰（无黄金锁这些边界——已核：grep 零命中）。
- E0105 报文面零变化：`case/timeout/collectAll` 仍在意外 token 清单（D1 修正——原稿误判为移除）；T1 已核无黄金钉该清单文本。

## D12 黄金表预估（~58 枚）

负例族 ~28（E1601–E1618 各 1–2：E1602 双形[裸 gc/value 形状内]、E1607 双形[未消费/双消费]、E1614/E1615/E1616 各实参形）+ 绿面族 ~24（装载/别名/注解/成员调用全型/Shareable bound 应用闭集/task 绿/scope 四形绿/select 绿/方向视图绿/命名 sum match）+ 边界 What ~6（D9 逐形）。生成器 /tmp 惯例（M8 gen.py 模式），锚点 marker 程序化定位。

## D13 披露义务

- 黄金自有新增在实现前修正 → 披露（M8 先例四处）。
- 实现揭出的规范面缺陷或既有机器缺陷 → 修复披露，不静默绕过。
- 既有测试/黄金任何触碰 → 逐处披露。
- D5 别名边界、D8 常量面收窄、D6 闭包穿透计数——照字面最小实现，扩面另批。
