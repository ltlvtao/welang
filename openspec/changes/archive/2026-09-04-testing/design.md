# Design: 测试系统（第 20 章）

## 裁决依据

四项用户裁决（2026-09-04，均采推荐）锁定设计空间：

1. **test + mock 两关键字**：`test "desc" { }` 顶层块 + `mock name(sig) { }` 块内声明是全部新语法；参数化与性质测试不新增语法（for 循环/组合子）。
2. **mock = 模块级单态 fn**：目标限本模块裸名与导入 `pub` 合格名；声明携原签名含效果段；泛型函数、方法、构造器不可 mock。
3. **虚拟钟+确定性调度全套**：time 全虚拟、`advanceTime` 仅 test 体、ch18 调度 carve-out、scope timeout 入虚拟钟。
4. **探索模式归工具链章**：spec 只锁可观察确定性；探索模式及其诊断是工具链（slice 6）的事。

## 设计决策

### D1 两关键字与破坏性记录

`test`、`mock` 入第 1 章词表，沿既定模式：词表加一行 `test mock`，破坏性句照录（"two words were identifiers before and are keywords from this list's amendment on"）。第 1 章Keywords 段早有预告句 "feature-specific words (effects, types, concurrency, **testing**) join the list together with the chapter that ratifies them" —— 本章正是那句承诺的兑付。零软关键字纪律不变：两词全保留、任何位置不得作标识符。+1 场景沿并发五词场景的先例（"The testing keywords are reserved"）。

### D2 测试模块：命名事实，布局归工具链

测试模块 = 文件名以 `_test.we` 结尾的模块。这是**模块同一性事实**（一个文件是不是测试模块由名字判定，编译器静态可知），故归 spec；`tests/` 目录布局、测试文件放哪里、其 import 如何映射到源根（`src/` 之外）是**布局与构建事实**，归工具链章（ch15 已言 manifest 是工具链的）。测试模块在其他一切方面是第 15 章普通模块：import、名字空间、跨模块初始化、pub 纪律照常。测试模块可被导入（无人会这么做，但无规则禁止）——不为此发明规则。

`test` 块出现在非测试模块 → `E1801`。这个检查以文件名为据，与 ch6 "一个文件解析为一个模块" 的同一性判定同源。

### D3 test 块的形状与语境

`test "description" { body }`：`test` 关键字 + 恰一个字符串字面量（描述，无语义角色——报告用，工具链读它）+ 第 2 章块。顶层项（ch6 枚举加入），仅测试模块。不携 `pub`（test 块不在 ch6 pub 批准类别里，那里照 `E0105` 拒绝——不新造码）。不携效果段——见 D4。体是**函数体语境**（ch12 清单第四、五成员连同 mock 体）：裸 `return` 提前结束用例合法、`return expr` 照 `E0402` 拒绝（test 块不产出值）、defer 照 ch3 合法。一个测试模块任意多个 test 块；**跨块执行序本规范不固定**（工具链的事），但每块运行到自身终点或 panic 边界。

### D4 效果豁免的立场（ch16 修订句的根据）

ch16 的调用位检查锚在 "Within a function body ... subset of the declared set"——test 块没有声明集，无段可写。设计立场：**test 体是驱动代码，效果真相站在被测声明自己的段里**；每个用例写 `effect io net time ...` 是零信息的噪声，mock 掉的调用本来就不真正执行。故 ch16 加测试句：test 块不携段、`E1401` 在 test 体内不裁决；mock 声明携原段、mock 体按该段检查（`E1401` 照常）。边界内的纪律不变：test 体里创建的 **task 块仍必写自己的段**（ch18 规则原样，task 对自己的段负责）、scope 块照常。豁免不隐藏任何东西：io/net 未 mock 时是真调用（D6 如实陈述），只是驱动侧不重述被驱动侧的效果上界。类型检查、资源检查、所有权检查在测试代码中**完整**——豁免的仅是效果上界这一个检查，因为不存在可供检查的上界声明。

### D5 mock 声明：单态、逐字、按名拦截

`mock name(params) -> ret effect seg { body }`，仅 test 块体的直接项（别处 `E1802`）。规则五条：

1. **目标解析**：名字是本模块顶层 fn 裸名，或导入模块的 `pub` fn 合格名 `mod.fn`（ch15 解析规则；未解析 `E1304`、非 pub `E1303` 照常）。目标须为**模块级单态 fn**：泛型 fn、impl/interface 方法、构造器、record/newtype/sum 名一律 `E1804`（mock 目标不是可 mock 函数）——泛型的逐实例化故事、方法的接收者通道、构造器的字段纪律都是未设计面，本变更不批。
2. **签名逐字一致**：参数表（名可不同？——**名也须一致**：签名是目标声明的事实复写，改名是自由度但没有需求，逐字一致把 `E1803` 的检查变成机械字符串级比较）与返回类型、**效果段**须与目标声明一致；不一致 `E1803`。段必写：即使目标无段（纯函数），mock 也写裸 `effect`？——不：目标无段则 mock 无段（逐字一致原则覆盖两端；ch19 的"段必写"是 foreign 无体的证据问题，这里有体可查）。
3. **按名拦截、不递归**：test 块内对该名字的调用位解析到 mock 体；目标函数自己的体内调用不受影响（mock 不是代理重写，目标或许根本不被执行）。拦截范围 = 当前 test 块（含其内嵌套块与 task/scope 体）。同块同名二次 mock `E1805`。
4. **mock 体是函数体语境**：按声明段检查（`E1401`）、return 规则照签名、资源纪律照常。mock 体里可再调用被 mock 的其他函数（非自目标）——那就是普通调用。
5. **效果归属**：调用位的效果集 = mock 声明的段 = 原声明段。因为 test 体不查 `E1401`，归属实际只对 mock 体内部与其他 mock 目标解析有意义——一致原则让一切免于特判。

### D6 虚拟钟：time 虚拟、io/net 不虚拟

test 块（含其内 task/scope 体）中一切携 `time` 标签的调用在虚拟钟上运行：不流逝墙上时间；钟仅在 `advanceTime` 与注册其上的等待处推进。**`io`、`net` 不虚拟**：未 mock 的 io/net 调用是真调用——诚实边界，正文直说。未 mock 的自定义效果同样真执行（W0601 式的警告是工具链层，本规范只陈述事实）。scope `timeout` 子句在测试模式读虚拟钟：超时预算随 `advanceTime` 流逝，用例可确定性触发超时路径。虚拟钟的粒度/单位 = stdlib 钟函数所报的单位（stdlib 表面的事）；本章只锁"虚拟、单调、仅由 advanceTime 与已注册等待推进"。

### D7 advanceTime 与四条件（ch18 R9 的对号入座）

`advanceTime(d: Int64)` 入预导入（ch15 闭集修订），仅 test 块体内合法（`E1806`），推进虚拟钟 d 单位并释放在被跨越时刻注册的等待。对 ch18 四条件逐条作答（正文章内展示，沿场景 222 "its spec amendment answers all four conditions against this requirement's text" 的要求）：

1. **非业务数据**：虚拟钟是测试运行时的控制令牌，不携用户语义 ✓
2. **生命周期被迫**：绑定 test 块；不存在携带钟越出块的手柄 ✓
3. **缺失静态可判**：test 体之外调用是编译错误 `E1806`，绝非运行时默认 ✓
4. **无竞争路径**：不存在测试钟的构造器或参数形——钟不是值 ✓

预导入机制完全镜像 `currentCancelSignal` 先例（ch15："the prelude carries the name, that chapter fixes where it is legal"）。

### D8 确定性调度：可观察承诺，策略归运行时

测试模式下（test 块及其内并发结构），调度确定性：**同一测试源 + 同一 advanceTime 序列 → 同一可观察交错**，逐次运行一致。虚拟钟上同刻就绪的多个等待按确定性次序解决（何序是运行时实现细节）。ch18:376 说 "no chapter promising a bound"——本章以显式 carve-out 句修订：调度承诺的未规定性是测试模式之外的一切执行的规则；测试模式是唯一精化。只锁可观察确定性，不锁策略（轮转/ FIFO/任何）——工具链与运行时保有实现自由，规范只承诺重现性。v0.8 §41.4 的 `--explore`（探索非确定交错、偏序缩减、迭代控制）是这一承诺之上的**工具链探测层**，归 slice 6。

### D9 第三捕获边界：panic 即用例失败

panic/todo 在 test 块内起飞 → 按 ch14 展开规则逐块退出（scope 释放、defer 逆序——ch14 的次序原文适用）→ 在 **test 边界**转为该用例失败（进程不中止，运行继续下一用例）。这是本规范定义的**第三捕获边界**，ch14（"second"）与 ch18（"the one ... besides the process abort"）各加一句指针修订，规则本体在本章。`assert` 失败是 panic（ch14 既定），故断言失败即用例失败，无第二机制（P9 单机制）。test 边界不是表达式：语言内仍无任何表达式观察 panic（ch14 纪律原样）；观察者是测试运行时，与任务边界的 `Err(TaskPanic)` 同一standing——但 test 边界**不产出值**（用例结果不是语言值，是工具链报告的事实）。

### D10 断言族：assert 在预导入，族在 std.test

`assert(cond, msg)` 已在第 14 章预导入族里。`assertEqual`/`assertTrue`/… 一律 `std.test` 普通库表面（`import std.test`），零文法。参数化：for 循环遍历表 + 辅助函数（既有表面，v0.8 §39 的 `test_each ... with [...] as (a,b,expected)` 消亡——它只是 for 的语法糖，P2 不为糖立项）。性质测试：stdlib 组合子（生成器闭包 + 收集器），v0.8 §40 的 `property "..." for_all (...)` 语法消亡。失败断言 = panic = 用例失败（D9 一条路）。

### D11 诊断段位与注册表

段位 `E1800`–`E1899` owner `2000-testing`；`E1900`–`E9999` 继续 unclaimed。六码：

| 码 | title |
| --- | --- |
| `E1801` | test block outside a test module |
| `E1802` | mock declaration outside a test block |
| `E1803` | mock signature does not match its target |
| `E1804` | mock target is not a mockable function |
| `E1805` | duplicate mock of one target in a test block |
| `E1806` | advanceTime called outside a test block |

`E1800` 边界保留、`E1807`–`E1899` 留修订。E1406 段注再刷新（"the testing codes landed in chapter 20's E18xx segment"）。条目 131→137、段 19→20。一码一消息纪律照旧（title 即消息首行）。

### D12 v0.8 映射账

| v0.8 | 去向 |
| --- | --- |
| §38 `test "desc" { assertEqual(...) }` | 本章 R1（assertEqual 改 std.test 表面） |
| §39 `test_each ... with [...] as (a,b,expected)` | **消亡**——for 循环即其全部语义，糖不立项 |
| §40 `property "..." for_all (a: Int64, b: Int64)` | **消亡（语法）**——stdlib 组合子承载 |
| §41.1 `mock fetchUser(_: Int64) -> Result<...>`（仅 test 块内） | 本章 R2，**收窄**：目标限模块级单态 fn（泛型/方法/构造器 `E1804`）；签名逐字一致含效果段 |
| §41.2 时间效果自动虚拟化 + `advanceTime(duration)` + scope timeout 虚拟化 | 本章 R3 + R4（裁决 3） |
| §41.3 确定性调度（task.await/channel.receive/signal） | 本章 R5 + ch18 carve-out（裁决 3） |
| §41.4 `--explore` `--iterations` `--reduce` 偏序缩减 | **工具链章**（裁决 4）；能力诚实有界（概率性非穷尽）的表述归那一章 |
| §41.5 W0601 未 mock 自定义效果警告 | **工具链层**（裁决 4）；spec 只如实陈述"是真调用" |
| §41.6 E1001 探索不确定性 / E1002 无 time 段的真钟调用 / E1003 探索中未 mock 效果 | E1001/E1003 → **工具链层**；E1002 → **消亡**——其形（调用携 time 的函数而未声明）已是 ch16 `E1401` 所治，无第二码 |
| §60 `we test`：tests/ 目录、`_test.we` 文件、文件内顺序/跨文件未定、pass/fail/skip/error、退出码 0/1/2 | `*_test.we` 命名 → 本章 R1（模块同一性）；文件内顺序"顺序执行"、跨文件未定 → 本章如实转述为"跨块执行序不固定"；其余（CLI、目录、并行、报告、退出码、skip 语义）→ **工具链章** |

### D13 宿主句的措辞纪律

七处宿主修订全部是**最小增量句**（不改写既有句），沿并发章修订 ch16、FFI 章修订 ch1/ch6 的先例：枚举加一短语、清单加一从句、计数句后补指针句。每处修订在 delta 里以完整 Requirement 形给出（MODIFIED 需全段重写），落地时按既定 splice 流程逐字验证。场景配给沿「每处修订配证据场景」的既定先例：ch1 +1（testing 关键字保留，沿并发五词场景）、ch6 +1（test 块顶层项）、ch12 +1（test/mock 体语境，沿 task 体场景）、ch14 +1（test panic 展开后失败，沿 task panic 场景）、ch15 +1（advanceTime 合法性，沿 currentCancelSignal/E1608 场景）、ch16 +1（test 体不查效果，沿 task 段拼写场景）、ch18 Scheduling promises +1（测试模式唯一精化）；ch18 Panics at the task boundary 0——其镜像证据由 ch14 的新场景承载。宿主合计 +7 场景。

### D14 三要素账（原则 10）

- **类型检查**：全部六个新码均为静态裁决——`E1801`（文件名同一性）、`E1802`（mock 位置）、`E1803`（签名逐字比对：参数名+类型+次序、返回、段的在/缺）、`E1804`（目标类别）、`E1805`（块内重复）、`E1806`（advanceTime 位置）；mock 体按段查 `E1401`（ch16 既有）；test 体类型/资源/所有权检查完整（D4）。
- **代码生成**：零新面——test 块编译为无参无名值函数体（描述串进 harness 可见的元数据）；mock 拦截是编译期名字解析（调用位解析到 mock 体而非原声明），不生成任何代理/跳板代码。
- **运行时**：三件事——虚拟钟（test 模式下 time 标签调用的钟源替换，advanceTime 推进并按确定次序释放注册等待，scope timeout 预算读钟）、确定性调度（同源同钟同交错的运行时策略，策略本身自由）、test 边界（panic 飞行到块边界转用例失败、进程继续）。三者都是测试运行时的可观察语义，策略细节（钟的数据结构、调度算法）实现自由。
