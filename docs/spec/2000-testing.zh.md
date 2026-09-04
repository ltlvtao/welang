# We 语言规范 —— 第 20 章：测试

### Requirement: 测试模块与 test 块

测试模块是文件名以 `_test.we` 结尾的源文件——模块同一性事实，编译器由文件名判定，正如它由文件名判定模块路径。测试模块内，test 块是 `test "description" { body }`：`test` 关键字、恰一个字符串字面量——测试的描述，为报告而携、自身不载任何语义——以及一个第 2 章块。test 块是第 6 章修订后枚举中的顶层项，仅测试模块合法：在任何其他文件中同一形式 MUST 以 `E1801:` test block outside a test module 拒绝。test 块不携 `pub`——它不在第 6 章已批准的 pub 类别里，该前缀在那里如处处一样被拒（`E0105`，第 6 章）。test 块不声明效果段；第 16 章的修订句固定这对体内调用意味着什么。体是第 12 章修订后枚举中的函数体语境——裸 `return` 提前结束测试，带值 `return` 如任何无值函数一样被拒（`E0402`），`defer` 依第 3 章在体出口运行。测试模块在其他一切方面是第 15 章普通模块：import、单一名字空间、跨模块初始化、pub 纪律——不变；测试文件在项目根下放哪里、其 import 如何映射到它，是工具链的事、非本规范的。每个 test 块运行到自身终点或本章的 panic 边界；test 块在文件内、跨文件的执行次序与它们之间的任何并行性不在此固定——工具链章的事。

#### Scenario: test 块解析为顶层项

- **WHEN** 文件 `math_test.we` 在其 import 与辅助函数之间持有 `test "absolute value" { let _ = abs(-3) }`
- **THEN** 该块依本章解析为测试模块的一个顶层项；描述串与体循本 Requirement 的形式

#### Scenario: 非测试模块中的 test 块被拒绝

- **WHEN** `test "x" { }` 出现在文件名不以 `_test.we` 结尾的文件中
- **THEN** 编译器以 `E1801:` test block outside a test module 拒绝——块仍是它所是的那项，只是它所在的模块不是测试模块；文件名说了算

#### Scenario: test 块上的 pub 被拒绝

- **WHEN** `pub test "x" { }` 出现在测试模块中
- **THEN** 编译器依第 6 章以 `E0105:` unexpected token 拒绝——test 块不是已批准的 pub 类别

#### Scenario: 测试模块是普通模块

- **WHEN** 测试模块 import `std.test` 与被测模块、声明辅助函数与顶层绑定、其测试调用两者
- **THEN** 第 15 章与第 6 章的每条规则如对任何模块一样成立；`_test.we` 名字不改变解析、初始化或可见性

#### Scenario: 单个测试的运行形状被固定，整轮运行的不被

- **WHEN** 一个测试模块持有两个 test 块
- **THEN** 各块运行到自身终点或其 panic 边界；谁先运行、它们是否共享进程，是工具链的——本章固定一个测试观察到什么，不固定测试的次序

### Requirement: mock 声明

在 test 块体内——作为其直接项——mock 声明是 `mock name(params) effect tag1 tag2 ... { body }`、`mock name(params) -> type effect tag1 tag2 ... { body }`、或无段的同形：`mock` 关键字、目标名、参数表、可选的声明返回、可选的效果段、以及体。目标名是被 mock 的函数：解析到本模块自己顶层 fn 的裸名，或解析到导入模块 pub fn 的合格名 `mod.fn`——全程第 15 章解析，名字无处解析时 `E1304`、解析到另一模块非 pub 条目时 `E1303`，如处处一样。mock 声明出现在 test 块体直接项之外的任何位置 MUST 以 `E1802:` mock declaration outside a test block 拒绝。声明的签名——每个参数及其类型、按次序按名、声明返回或在缺、效果段或在缺——MUST 与目标声明逐字一致；不一致 MUST 以 `E1803:` mock signature does not match its target 拒绝——mock 复述目标的契约，不重新设计它。目标 MUST 是模块级单态 fn：泛型 fn、impl 或 interface 方法、构造器、或任何非普通 fn 声明的名字 MUST 以 `E1804:` mock target is not a mockable function 拒绝——mock 的单一形状故事是普通调用，而泛型、接收者与字段初始化没有已批的 mock 语义。一个 test 块对一个目标至多 mock 一次：同块对同一目标的第二个 mock MUST 以 `E1805:` duplicate mock of one target in a test block 拒绝。mock 生效即调用按名解析：该 test 块内目标名的每个调用位——嵌套块、task 与 scope 体在内——解析到 mock 的体而非目标的。拦截按名、不按代理：目标自己的体不受影响、可能从不运行——mock 了 `f` 不等于 mock 了 `f` 所调用的一切。mock 的体是函数体语境、按声明段检查：第 16 章 `E1401` 在那里如在任何函数中一样适用，段即目标自己的逐字段。

#### Scenario: 本模块函数的 mock 拦截其调用

- **WHEN** 测试模块的 test 块持有 `mock readCount() -> Int64 effect { 7 }` 且被测代码调用 `readCount()`
- **THEN** 调用解析到 mock 的体；测试观察到 7，而目标的体——foreign 调用、文件读、任何东西——从不运行

#### Scenario: 导入 pub 函数的 mock

- **WHEN** 目标是导入模块的 `pub fn find(id: Int64) -> Option<User> effect io` 且 test 块持有 `mock user.find(id: Int64) -> Option<User> effect io { Some(User { id: id }) }`
- **THEN** 合格目标依第 15 章解析，拦截覆盖该块内的 `user.find(...)` 调用位；pub 规则是唯一的进入之路，如同任何跨模块到达

#### Scenario: test 块之外的 mock 被拒绝

- **WHEN** `mock f() { }` 出现在模块顶层或普通函数体内
- **THEN** 编译器以 `E1802:` mock declaration outside a test block 拒绝——mock 只作为 test 体的直接项存在

#### Scenario: 签名不一致被拒绝

- **WHEN** 目标声明 `fn find(id: Int64) -> Option<User> effect io` 而 mock 写 `mock find(name: String) -> Option<User> effect io { ... }`
- **THEN** 编译器以 `E1803:` mock signature does not match its target 拒绝——参数、返回与段是复述，不是重新设计

#### Scenario: 不可 mock 的目标被拒绝

- **WHEN** mock 指名泛型 fn（`fn id<T>(x: T) -> T`）、impl 方法、或构造器如 `User`
- **THEN** 编译器以 `E1804:` mock target is not a mockable function 拒绝——只有模块级单态 fn 可 mock

#### Scenario: 一个目标的第二个 mock 被拒绝

- **WHEN** 一个 test 块持有两个 `readCount` 的 mock
- **THEN** 编译器以 `E1805:` duplicate mock of one target in a test block 拒绝第二个——一块、一目标、一 mock

#### Scenario: 拦截按名、不按代理

- **WHEN** `f` 内部调用 `g` 而测试只 mock `f`
- **THEN** 对 `g` 的调用不受影响，无论出现在哪里——mock 了 `f` 是 mock 了名字 `f`，不是它之下的调用图

#### Scenario: mock 的力量止于其块

- **WHEN** 一个 test 块 mock `readCount`，同模块的第二个 test 块调用 `readCount()` 而不设自己的 mock
- **THEN** 第二个块的调用到达真 `readCount`——mock 的力量属于它自己的块，每块为自己而 mock

### Requirement: 虚拟钟

在 test 块内——块自己的体与其内每个 task 与 scope 体——效果集携 `time` 标签的每个调用在虚拟钟上运行：这样的调用等待时不流逝墙上时间，标准库时间函数所报的钟仅在 `advanceTime` 与注册于钟上的等待处推进。虚拟化对 `time` 是完全的、对其余是诚实的：`io` 与 `net` 不虚拟——测试中未 mock 的 io 或 net 调用是真调用，读真文件、开真 socket——未 mock 的自定义效果同样真执行；隔离之路是 mock，本规范如实直说而非默默默认。test 块内 scope 块的 `timeout` 子句读虚拟钟：预算由 `advanceTime` 跨过它而到期，绝不因等待而到期，于是超时路径被确定性地行使。

#### Scenario: time 标签调用观察虚拟钟

- **WHEN** 测试两次调用标准库的钟而其间无 `advanceTime`
- **THEN** 两次读数报告同一时刻——钟只在本章所说之处移动

#### Scenario: 未 mock 的 io 调用是真调用

- **WHEN** 测试经未 mock 的 io 标签函数读文件
- **THEN** 读触到真实文件系统；隔离来自 mock 那个函数，此处没有任何东西假装不是

#### Scenario: scope 超时在 advanceTime 下触发

- **WHEN** test 块的 scope 携 `timeout(100)` 且测试将虚拟钟推进 100 而预算其余未花
- **THEN** 超时路径运行——预算读自虚拟钟，且路径在每次运行中重现

#### Scenario: 钟恰在两处推进

- **WHEN** 测试持有 time 标签调用与 `advanceTime` 调用
- **THEN** 报告的时刻只在 `advanceTime` 之间与注册于钟的等待之间变化——没有别的构造移动它

### Requirement: advanceTime

`advanceTime(d: Int64)` 将虚拟钟推进 `d`——标准库钟函数所报的单位——按确定性次序释放在被跨越时刻或之前注册的每个等待。名字依第 15 章修订入预导入，且仅在 test 块体内合法——task 与 scope 体在内，因为它们是测试自己的范围；别处 MUST 以 `E1806:` advanceTime called outside a test block 拒绝。该名字是本语言第二个隐式获取，第 18 章准则约束它：本 Requirement 对该准则文本的全部四个条件作答。一——非业务数据：虚拟钟是测试运行时自己的控制令牌，不携用户定义语义。二——生命周期被迫：钟绑定 test 块；不存在携带钟越过块的手柄，也不存在块内缺失它的方式。三——缺失静态可判：test 体之外的使用是 `E1806`、编译错误，绝非运行时默认或变装的缺席值。四——无竞争路径：不存在测试钟的构造器或参数形——钟不是值，没有调用方在它的两种拼写之间做选择。

#### Scenario: advanceTime 推进并释放等待

- **WHEN** 测试中的一个 task 等待注册在虚拟钟上的时长，而测试调用 `advanceTime` 越过它
- **THEN** 唤醒发生在跨越处，与绑在同一时刻的等待按确定性次序

#### Scenario: test 块之外的 advanceTime 被拒绝

- **WHEN** 测试模块的普通函数——任何 test 块之外——调用 `advanceTime(5)`
- **THEN** 编译器以 `E1806:` advanceTime called outside a test block 拒绝；预导入携名，本章定其合法之处

#### Scenario: advanceTime 答四条件

- **WHEN** 本 Requirement 对照第 18 章隐式获取准则量度
- **THEN** 四条件全部成立——控制令牌、绑定 test 块、块外静态拒绝、无竞争拼写——准则为后续候选设的场景在此兑现

### Requirement: 测试模式确定性调度

测试模式内——一个 test 块与其持有的每个并发结构——交错是确定性的：同一测试源配同一 `advanceTime` 序列，每次运行产出同一可观察交错。哪一个是那一个交错不在此固定：同刻唤醒的确定性次序与产生它的调度策略是运行时自己的——本规范承诺的是重现，不是任何特定次序。这是第 18 章调度承诺的唯一精化、经该章修订：测试模式之外，任务执行次序与并发任务交错保持该 Requirement 所固定的未规定。

#### Scenario: 同一测试两次交错相同

- **WHEN** 一个创建 task 与 channel 的测试在同一工具链下运行两次
- **THEN** 可观察交错——发送、接收与完成的次序——两次运行之间完全相同

#### Scenario: 同刻唤醒确定性地解决

- **WHEN** 两个等待注册在同一虚拟时刻而钟跨过它
- **THEN** 它们唤醒的次序对该测试源固定，无论运行时策略定义什么次序

#### Scenario: 承诺是重现、不是次序

- **WHEN** 两个运行时运行同一测试
- **THEN** 各自产出自己的一个固定交错、在自己的运行之间相同——本章不承诺两者一致，只承诺各自自洽

### Requirement: test 边界的 panic

panic 或 `todo` 在 test 块内起飞依第 14 章规则在块内展开：块退出、scope-resource 绑定各恰一次逆序释放、defer 逆语句序运行、语言内没有表达式观察这次飞行。test 边界是捕获边界——本规范定义的第三个，与进程中止和任务边界并列：在边界处飞行转为该测试的失败；进程不中止，运行越过失败的测试继续。测试的结果不是语言值——没有表达式读它；观察者是测试运行时，如同 joining scope 是任务边界的观察者。`assert` 的失败依第 14 章是 panic，失败的断言经这唯一路线使其测试失败——没有第二失败机制（第 0 章，原则 9）。

#### Scenario: panic 使测试失败、不使进程失败

- **WHEN** test 块的体 panic
- **THEN** 测试在边界处以失败结束、运行继续下一个测试；没有中止发生

#### Scenario: 资源在边界之前释放

- **WHEN** panic 的测试持有 scope-resource 绑定与 defer
- **THEN** 释放与 defer 依第 14 章展开次序在块内运行，先于飞行在 test 边界结束

#### Scenario: 失败的 assert 是失败的测试

- **WHEN** 测试的 `assert(cond, msg)` 发现 `cond` 为假
- **THEN** 它依第 14 章 panic 的那个 panic 不越过任何表达式、在 test 边界结束、测试失败——一条路，无第二机制

### Requirement: 断言与标准库测试表面

`assert(cond, msg)` 是预导入自己的——第 14 章终止族，在测试中如处处可调。更宽的断言族——`assertEqual` 与其同类——是标准库表面、非文法：函数住在 `std.test`、仅经 import 到达，本章不为它们添加任何语法。参数化测试是表上的 for 循环配辅助函数——第 3 章与第 6 章早批的表面；性质测试是生成器上的标准库组合子闭包——第 12 章表面。v0.8 的 `test_each ... with [...] as (a, b, expected)` 与 `property "..." for_all (a: Int64, b: Int64)` 形未被批准：二者都是已批表面上的糖，糖不开章（第 0 章，原则 2）。

#### Scenario: assert 在测试中可调

- **WHEN** 测试体调用 `assert(n == 7, "n was not 7")`
- **THEN** 调用是预导入的第 14 章函数；其失败是 panic、在边界处使测试失败

#### Scenario: assertEqual 族是 std.test 表面

- **WHEN** 测试想要 `assertEqual(got, want)` 而先写 `import std.test`
- **THEN** 调用如任何导入 pub fn 一样解析；本章没有文法参与，缺失 import 时如处处一样是 `E1304`

#### Scenario: 参数化与性质测试用已批形式

- **WHEN** 一张用例表与一个 for 循环驱动一个体，或一个生成器闭包喂一个性质检查
- **THEN** 涉及的每个形式都是第 3、6、12 章表面；本章的语法是 `test` 与 `mock`、再无其他

### Requirement: 测试不固定什么

本章指名它留下什么不固定。test 块在文件内与跨文件的执行次序、它们之间的并行性、过滤、报告格式、退出码、`tests/` 目录布局、以及测试模块 import 到项目源根的映射——全是工具链章的；本章固定一个测试观察到什么，不固定一轮运行的形状。探索——v0.8 的 `--explore` 及其迭代控制与偏序缩减，探测确定性调度器不会选择的交错——是工具链层事务，其守护诊断（v0.8 的 W0601、E1001、E1003）也是；本规范固定这类工具所探测的可观察确定性，诚实有界：确定性重现是承诺，交错的穷尽覆盖不是、也无处声称为。测试中未 mock 的自定义效果是真调用——如实直说；工具链是否警告它是工具链的，spec 层不存在任何警告。

#### Scenario: 一轮运行的形状是工具链的

- **WHEN** 一个项目的测试在工具链下运行
- **THEN** 次序、并行、过滤、报告与退出码随那一章；此处除每个测试自己的确定性外不约束它们

#### Scenario: 探索是工具链的

- **WHEN** 工具探索一个测试的替代交错
- **THEN** 它在本章确定性承诺之上探测；spec 层固定重现、从不固定穷尽覆盖，v0.8 探索模式的守护诊断是工具链层的

#### Scenario: 未 mock 的自定义效果真运行

- **WHEN** 测试调用一个自定义效果标签的函数而其路径上什么也不 mock
- **THEN** 调用真实执行；隔离是 mock 给的，本章直说而不默默默认

### Requirement: 测试诊断段位

测试章拥有注册表段位 `E1800`–`E1899`，声明于 `docs/spec/diagnostics.toml` 的 `[segments]`。分配：`E1801` test block outside a test module、`E1802` mock declaration outside a test block、`E1803` mock signature does not match its target、`E1804` mock target is not a mockable function、`E1805` duplicate mock of one target in a test block、`E1806` advanceTime called outside a test block。`E1800` 与 `E1807`–`E1899` 为本章修订保留。触发语义在本章 Requirement 内；条目在注册表。

#### Scenario: 一个测试码被发射

- **WHEN** 工具链发射任何 `E18xx` 诊断
- **THEN** 其完整条目可从 `docs/spec/diagnostics.toml` 以 owner `2000-testing` 检索

#### Scenario: 后续变更需要本段位的码

- **WHEN** 本章的未来修订需要新诊断
- **THEN** 它在同一变更内于 `E1800`–`E1899` 内扩展注册表，或声明自己的段位

## 示例（非权威）

下列示例只用第 1–20 章已批准的表面形式。它们是说明性的、非权威的：任何冲突处以 Requirement 与 Scenario 为准。带诊断码注解的行是被拒绝的形式，展示编译器发射的码。

### 一个测试模块

```we
// file: math_test.we — the _test.we name makes the module a test module
import std.test

fn abs(n: Int64) -> Int64 {                 // helper under test, same module
    if n < 0 { -n } else { n }
}

test "absolute value" {
    std.test.assertEqual(abs(-3), 3)        // the assert family is std.test surface
}

test "zero" {
    assert(abs(0) == 0, "abs of zero")      // assert itself is prelude, chapter 14
}
```

### mock 按名拦截

```we
// file: cache_test.we
import std.test
import store

fn load(key: String) -> String effect io { store.read(key) }   // under test

test "served from the mock, not the store" {
    mock store.read(key: String) -> String effect io { "cached" }
    std.test.assertEqual(load("k"), "cached")   // the call site resolves to the
}                                               // mock's body; the store never runs

test "a second mock of one target is a duplicate" {
    mock store.read(key: String) -> String effect io { "cached" }
    // mock store.read(key: String) -> String effect io { "other" }
    //                                     // E1805: duplicate mock of one target in a test block
}
```

### 虚拟钟

```we
// file: timer_test.we
import std.test

test "a scope timeout fires under advanceTime" {
    fn pollPeriod() -> Int64 effect time { 10 }    // time runs on the virtual clock

    scope timeout(100) {
        task effect time {
            let _ = sleep(50)                   // registered on the virtual clock
        }
    }
    advanceTime(100)                            // the budget expires here,
    std.test.assertTrue(true)                   // deterministically, every run
}
```

### 确定性调度

```we
// file: relay_test.we
import std.test

test "relay order reproduces on every run" {
    let ch = channel(0)                          // unbuffered, std.concurrent
    scope {
        task effect time { let _ = sleep(10); ch.send(1) }
        task effect time { let _ = sleep(10); ch.send(2) }
    }
    advanceTime(10)                              // both waits tied at one instant:
    // the wake order is fixed for this source — same test, same advanceTime
    // sequence, same order, on every run
}
```

### 被拒绝的形式

```we
// test "x" { }                               // E1801: test block outside a test module
//                                            //        (in a file not ending _test.we)
// fn helper() { mock f() { } }               // E1802: mock declaration outside a test block
// mock find(name: String) -> User { }        // E1803: mock signature does not match its target
//                                            //        (target: fn find(id: Int64) -> User effect io)
// mock id(x: Int64) -> Int64 { x }           // E1804: mock target is not a mockable function
//                                            //        (target: fn id<T>(x: T) -> T)
// pub test "x" { }                           // E0105: unexpected token — pub on a test block
// fn drive() { advanceTime(5) }              // E1806: advanceTime called outside a test block
```

### 待后续变更

```we
// The test runner — we test, the tests/ directory layout, parallelism,
// reporting, exit codes — is the toolchain chapter's business; exploration
// (--explore, iteration controls, partial-order reduction) and its guard
// diagnostics (W0601, E1001, E1003) are that layer's too. The std.test
// surface — the assertEqual family, property combinators — is a standard
// library release's:
//
// we test --explore --iterations 200        // toolchain, not this chapter
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| test module | 测试模块 |
| test block | test 块 |
| description | 描述 |
| mock declaration | mock 声明 |
| target | 目标 |
| interception | 拦截 |
| by name, not by proxy | 按名拦截，非代理 |
| virtual clock | 虚拟钟 |
| wall time | 墙上时间 |
| test mode | 测试模式 |
| deterministic scheduling | 确定性调度 |
| tied wake | 同刻唤醒 |
| test boundary | test 边界 |
| assertion family | 断言族 |
| parameterized testing | 参数化测试 |
| property testing | 性质测试 |
| exploration | 探索 |
