# 变更提案：并发（第 18 章）

## Why

规范至今对并发的全部承诺都是悬句，本章兑现它们：

- ch0（0000-principles.md）P9：并发相关失败（超时、取消、任务内 panic）走与其他失败相同的机制，不得存在「只为并发」的第二通道；runtime 含调度器与并发原语是产品范围。
- ch8（0800-composites.md）示例悬句：`Ref/Shared still arrive with the concurrency chapters`——所有权四类别落地后，跨任务共享状态一直没有权威承载。
- ch10（1000-interfaces.md）Derives：`Shareable to the concurrency chapter`——标记接口悬置至今。
- ch14（1400-errors.md）Unwinding：`A concurrency chapter, if ratified, binds task-internal panics to this same mechanism`——任务边界捕获 panic→Err 的预承诺。
- ch16（1600-effects.md）：内建标签 io/net/time 已定，但并发原语的阻塞等待在效果系统中的地位未钉（v0.8 §31.3/§37 规则 3 的裁决未继承）。

服务端语言（ch0 目标域：HTTP/RPC 服务、后台作业）没有结构化并发与共享状态原语即不可用；本片补齐规范的最大缺口。

**v0.8 基线**：§29–§37 + 评审补丁（E0733 resource 捕获 iff 无 mut self 方法；§35.3.1 隐式上下文获取四条件准则）。v0.8 是方向性参考，不逐字继承。

## What Changes

**新增第 18 章 `docs/spec/1800-concurrency.md`（+zh）**，一章全量承载（裁决 1）：

1. **共享状态类型**（裁决 2：拆独立类型，非 v0.8 单名 Shared）：`Mutex<T>` / `RwLock<T>`（+read——v0.8 的 write 冗余于 update，弃置披露）/ `Atomic<T>`（T 限原子基类型，E1614）/ `AtomicRef<T>`（T 限 gc record，E1615）；update/get/set 统一三法，锁级别随类型名显式；嵌套访问自身回调内拒绝（E1613）。
2. **Cond 与 Semaphore**：`Cond<T>` 仅配 `Mutex<T>` 构造（rwlock 配对被裁掉，偏离披露）；`Semaphore(n)` 计数信号量，release 超额 = 运行时 panic。
3. **task 块与捕获纪律**：`task effect 标签... block` 表达式形（效果段必写，第 16 章声明侧拼写原样进表达式位；块外无 scope 拒绝 E1618；task 体 = 第三个函数体语境——return/defer/E0401，经 ch12 语境枚举修订）；捕获四规则——gc 裸捕拒绝（E1602，含经 value 形状携带的非 Shareable gc）、var 禁捕（E1603）、resource 无 mut-self 方法可捕（E1604，E0733 补丁）、泛型参数需 Shareable 约束（E1605）。
4. **Shareable 标记接口**：编译器自动附加、禁手写 impl（E1606）；非 derives 目标（修正 ch10 悬句的预设）。
5. **scope 块族**：`scope { }` / `scope timeout(m) { }` / `scope collectAll { }` / 复合——表达式形；句柄纪律（每路径 await 或 cancel，E1607）；fail-fast 与 collectAll 的机械分界；超时返回 `Result<T, TimeoutError>`。
6. **协作式取消**：`CancelSignal` / `currentCancelSignal()`（task 块外 E1608）；§35.3.1 四条件准则入规范——隐式获取的封闭边界。
7. **Channel 与 select**：`channel(n)` 构造（期望类型定 T，E1617）；send/receive/trySend/tryReceive + SendResult/ReceiveResult 三态 sum；方向视图 `SendOnly<T>`/`ReceiveOnly<T>` 经显式方法转换（非 v0.8 隐式收窄——本规范无隐式转换豁免传统）；select 表达式、四等待源封闭集（E1609）、臂一致（E1610）、case 模式仅绑定/通配（E1611）、至少两 case（E1612）。
8. **任务边界 panic 捕获**：unwinding 在任务内走完（资源/defer 全执行），边界转为 `Err(TaskPanic)`——兑现 ch14 预承诺。
9. **诊断段位** E1600–E1699（E1601–E1618，18 码）。

**裁决 3**：`Ref<T>` 终身不引入——单任务可变已由 ch8 `var` + ch10 `mut self` 权威承载，再立即违 P5（一个事实一个权威位置）。

**裁决 4**：并发原语的等待操作零效果标签（ch16 增补 carve-out 句 + 场景）；W0755/W0756 类分析属 vet 层，延后工具链章。

**宿主修订（7 章 ×2 语言，8 处 Requirement）**：ch1 关键字 +`task select case timeout collectAll`（34→39 词，破坏性照录）；ch2 表达式骨架 +scope/task/select 关键字引导形；ch10 Derives 悬句改写（Shareable 非派生目标）；ch12 捕获纪律指针句（闭包=同步捕获，task 块=第 18 章并发捕获）+ E0401 语境枚举扩展（task 体入列）；ch14 Unwinding 悬句落定；ch15 预导入 +`Shareable` +`currentCancelSignal`；ch16 carve-out（task 段必写 + 等待零效果）。

**延后并披露**：`WeakRef<T>`（GC 生命周期故事未批）；`region`（内存区域故事未批）；`Shared.transaction` 多锁原子复合（其 v0.8 设计依赖单名 Shared 家族，拆类型后需重新设计，登记为缺口）；W0755 跨函数嵌套访问启发式、W0756 may-block note（vet 层，工具链章）；锁序静态检查（E0761 同类）。

## 影响层

spec。

## 影响范围

`docs/spec/` 第 1、2、10、12、14、15、16、18 章 + `diagnostics.toml` + 对应 zh 孪生（新增 18；1/2/10/12/14/15/16 修订；8 之 D14 示例刷新）；`docs_sync` 对数 24→25；不触碰 chapters 之外任何工件。

## 裁决记录

1. **切片范围 = 一章全量**（用户裁决，采推荐）：共享状态（Mutex/RwLock/Atomic/AtomicRef/Cond/Semaphore）+ 结构化并发（task/scope/取消）+ Channel/select 落第 18 章——捕获规则要求 gc 包同步类型、select 等待源含 TaskHandle.await 与 Channel.receive，拆两章互相悬空（collections 先例）。
2. **Shared 家族 = 拆独立类型**（用户裁决，采推荐）：`Mutex<T>`/`RwLock<T>`/`Atomic<T>`/`AtomicRef<T>` 各自类型各自方法集，构造模式入类型名——v0.8 单名 `Shared<T>` + 构造溯源跨函数后不可局部判定（fn f(s: Shared<Int64>) 体内 s.read() 合法与否取决于调用方构造方式，P1 破口）；统一方法集案丢失 rwlock 读并行与无锁档位（性能是产品目标）。Cond 仅配 Mutex（rwlock+条件变量易错，偏离披露）。
3. **Ref<T> 终身不引入**（用户裁决，采推荐）：单任务可变已由 ch8 `var` + ch10 `mut self` 权威承载，再立即违 P5 一个事实一个权威位置；E0701 随之消亡。
4. **等待零效果标签**（用户裁决，采推荐）：并发原语的阻塞等待不是环境读写，不计效果（v0.8 §31.3/§37 规则 3 继承）；ch16 增补 carve-out 句；W0755/W0756 类分析属 vet 层，延后工具链章。

## 目标与非目标

目标：
- 兑现已落地规范对并发的全部悬句承诺：ch0 P9（并发失败同机制）、ch8 示例（Ref/Shared 悬句）、ch10 Derives（Shareable）、ch14 Unwinding（任务边界捕获 panic→Err）、ch16（原语等待的效果地位）；
- 四共享类型 + Cond + Semaphore 的构造/方法集/嵌套访问纪律全场景钉住（锁级别随类型名显式）；
- task 块（必写效果段）+ 捕获四规则（gc 裸捕 E1602 / var E1603 / resource 无 mut-self 可捕 E1604 / 泛型 Shareable E1605）；
- scope 块族（bare/timeout/collectAll/复合）+ 句柄纪律（路径分析）+ fail-fast 与 collectAll 机械分界；
- 协作式取消（CancelSignal/currentCancelSignal + §35.3.1 四条件准则入规范）；
- Channel/select：四等待源封闭集、三态 try 结果 sum、方向视图显式转换、select 表达式臂一致；
- E1600–E1699 段位与 E1601–E1618 十八码入注册表；ch1 关键词 34→39。

非目标：
- `Ref<T>`——裁决 3 终身关死；
- `WeakRef<T>`——GC 生命周期故事未批，另立变更；
- `region`——内存区域分配故事未批，另立变更；
- `Shared.transaction` 多锁原子复合——v0.8 设计依赖单名 Shared 的异构列表，拆类型后需重新设计，登记缺口（转账场景以约定锁序逐个 update 过渡）；
- W0755 跨函数嵌套访问启发式、W0756 may-block note——vet 层分析，工具链章；
- 同作用域锁序静态检查（v0.8 E0761 同类）——随 transaction 一起回来；
- 强制中断/抢占终止——协作式取消是唯一与资源/共享状态完整性兼容的模型（v0.8 §36.3 立场继承）；
- 并发原语在预导入中的类型名——住 std.concurrent 模块（import 到达）；仅 `Shareable`（编译器附加标记，语言级名）与 `currentCancelSignal`（语法邻接）入预导入；
- 公平性/优先级调度承诺——仅承诺「最终执行」底线（v0.8 §36.5 继承）。

## 审计记录

**2026-09-04：通过。** 7 点逐项：(1) 问题真实性——Why 五处宿主锚点实读核对（ch0:126 P9 Requirement + ch0:158 runtime 产品范围句、ch8:372 `Ref/Shared still arrive with the concurrency chapters`、ch10 Derives 末句 `` `Shareable` to the concurrency chapter``、ch14:118 条件句 `A concurrency chapter, if ratified...`、ch16 内建标签已定而等待效果地位未钉），全部 grep 可验；目标域缺口（服务端语言无结构化并发不可用）是黑盒可验证的规范缺口。(2) 影响层 `spec` 与 change.yaml `layers: [spec]` 一致。(3) 增量范围——ADDED 第 18 章 16R/66S + MODIFIED 七宿主（ch1/ch2/ch10/ch12/ch14/ch15/ch16，机器 diff 差异恰为既定改动、零意外丢失）；新码 E1601–E1618 对照注册表 106 条全空闲、无其他活跃变更；段内 E1600/E1619–E1699 为预留边界提及（E0800/E0900/E1000 先例）；乘码 E0501/E1401/E1402/E0816/E0811/E0605/E1106/E0827 注册表 title 逐一比对语义一致。(4) 原则一致——P1（拆类型后签名即真相、隐式获取四条件准则封闭边界、捕获规则全静态局部）、P5（Ref 终身不引入、write 冗余于 update 弃置）、P9（TaskPanic 走 Result 无第二通道）、P4（调度 unspecified-but-safe 明文记界）；关键字 +5 走 ch1 预授权破坏性照录机制；等待零效果标签镜像 ch14 panic 族 carve-out 先例，非原则例外。(5) 基线固定 refr/spec-0.8.md §29–§37 + refr/spec-0.8-review.md E0733/§35.3.1，方向性参考不逐字继承（偏离七项均在 design 披露）。(6) 验收边界——目标可 grep（悬句归零、16R/66S、注册表 124 条/18 段、关键字 39 词），非目标九项排除明确（Ref/WeakRef/region/transaction/W0755-W0756/锁序检查/强制中断/预导入类型名/公平性）。(7) 粒度——捕获纪律要求同步类型集、select 等待源要求 Channel+TaskHandle、scope 句柄纪律要求 task 块，同一垂直故事不可拆（collections 先例）。

## 审查记录

**2026-09-04：通过（修复 5 处发现后）。** 10 点审查发现并已在激活前修复：**F1（真缺陷）task 体内的 defer/return 无根基**——ch3 E0204 限定 defer 为函数体直属项、而 Semaphore 场景与任务内 panic 展开都依赖 task 体携带 defer；修复：照 ch12 闭包先例把 task 体定为第三个函数体语境（return 携任务值早退、defer 于体出口、E0401 语境），枚举权威保持一处——ch12 R6 语境清单经本变更修订纳入、ch18 R5 引用（ch12 delta 增至 2 条 Requirement）。**F2（真缺陷）名字到达与 import 语义冲突**——ch6 R2 import 只引入模块名单一名字，示例的裸 `Mutex(0)` 不可达；修复：成员一律合格到达（`concurrent.Mutex`、`concurrent.channel(4)`、`concurrent.TaskPanic`），示例用 `as conc` 别名，章内 R1 声明「正文裸名为阅读约定、合格拼写才是可达拼写」；`channel`/两视图/`CancelSignal` 的「builtin」措辞改为 std.concurrent 模块项（仅 Shareable/currentCancelSignal 是语言级）。**F3（真缺陷）`T: Shareable` 写不出**——约束名需在作用域内而 Shareable 未入预导入；修复：ch15 预导入 +`Shareable`（编译器附加标记属语言级名，与 currentCancelSignal 同批），R7 补「名字预导入可见」句。**F4（真缺陷）E1602 原消息盖不住 value 形状**——含 List 字段的记录、含 List 元素的元组按类别是 value，不触「gc binding」措辞，gc 状态照样穿越任务边界；修复：消息放宽为「task block captures unsynchronized gc state」，R6 增递归闭包句 + 新场景「Unsynchronized gc state inside a value shape is rejected alike」（66→67 场景）。**F5（真缺陷）scope 体的 return/break/continue 穿透未定**——ch13 已定穿透即出口，并发 scope 未说穿透时句柄义务与 join 语义；修复：R10 早退句由「`?`」扩为「`?` 或 return/break/continue 穿透」，fail-fast 场景 THEN 补同名半句。另两处措辞修正随行：R1 构造例 `RwLock(Map.empty())`（非已批表面）换 `RwLock(text)`；timeout 场景标题「The timeout millis answer chapter 7」更名「The timeout clause's expression is Int64」。职责边界（proposal 黑盒、delta 无实现泄漏、design 唯一路径含 D17/D18 被拒方案与理由、tasks 来源验证齐）与内容质量（normal/boundary/failure 三路齐——18 码各至少一触发场景、trySend 三态、release 超额、cancelled-never-checks、无空章节、注入哨兵任务在列、纯 spec 层无三要素义务披露——实现未启动，原则 10 义务属编译器实现变更、无未决问题阻塞）其余各点通过。

## 实现审查记录

**2026-09-04：通过（1 处披露已当场修复）。** 7 点逐项：

1. **规范符合性**：提升文与 delta 机器逐字 diff——新章 16R/67S 全部要求块逐字（regex 块抽取 + rstrip+双换行归一化，16/16 True）；宿主 8 处 Requirement（ch1 Keywords/ch2 Expression skeleton/ch10 Derives/ch12 Capture discipline + Closure bodies are function bodies/ch14 Unwinding/ch15 The prelude/ch16 The effect segment）逐字 8/8 True；宿主场景计数对 git HEAD 机器比对，差异恰为既定新增（7 处各 +1、ch16 +2、zh 镜像同步 8/8 OK），零意外丢失。ch18 R16 所述段位 E1600–E1699 与注册表 `[segments]` 一致；18 条目 requirement 字段全部解析到章内标题（validate owner-file 检查转绿即证）。
2. **验证诚实性**：tasks.md §1–4 逐项勾选与实跑命令一一对应——负例注入（E1699 注入 ch2 → `--all --strict` FAIL「no registry entry」→ 还原 → 绿）先于条目落地；注册表拆段 + 18 条目后 validate 绿；逐字 diff、场景计数、docs_sync 25 对、一码一消息 0 失配、全角冒号扫描 0、CJK-EN 界外 0、代码块 EN↔zh 字节一致 7/7——全部本窗口实跑，输出在案。
3. **测试先行证据**：spec 层变更沿 lexical-foundations 先例（断言先于章文件）——注入哨兵先于条目落地实测 FAIL 即负例先行证据；无编译器三要素义务（纯 spec 层）。
4. **诊断协议稳定**：注册表 diff 的唯一删除行是段头 `[segments."E1600-E9999"]`（拆分为 E1600–E1699 + E1700–E9999），既有 106 条目零删除零改名零字段变更；新增 18 码全局唯一（124 条、tomllib 解析 18 段确认）。
5. **单一权威**：章内容与段位已全量落 `docs/spec/`；变更目录仅存提案/设计/任务/增量（归档即变更记录）；注释与 docs 英文（zh 孪生除外，其术语表与译文为既定义务）。
6. **红线复核**：`refr/` 零跟踪文件；diff 范围 = docs/spec/ 9 章 ×2 语言 + diagnostics.toml + 变更目录，无越界改动；提交信息无署名（待提交时复核）。
7. **最小可信验证**：`validate.py --all --strict` 绿（registry clean）、`docs_sync.py --check` 25 对绿——即验证阶梯对本 diff 的全部适用命令。

**发现并当场修复**：提案「影响范围」行漏列 ch8（D14 示例刷新的实际触点，tasks.md §4 本有该任务）——已补「8 之 D14 示例刷新」入范围行。无其他发现。
