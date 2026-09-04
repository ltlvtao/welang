# We 语言规范 —— 第 16 章：效果


### Requirement: 效果声明与内建标签

效果声明是顶层项 `effect name`，可选带 `pub` 前缀。名字按第 1 章命名约定（`E0012`）为 camelCase，并按第 6 章入模块单一名字空间，与任何其他名字碰撞即 `E0404`。内建标签是 `io`、`net` 与 `time`——语言级名字，非模块项；自定义效果 MUST NOT 取内建之名，MUST 以 `E1403:` effect name conflicts with a built-in effect 拒绝。效果名与一切其他名字一样按第 15 章名字解析：声明模块内裸名，pub 时经 import 以 `mod.tag` 合格到达。效果标签参与的检查从不取决于哪个模块声明了它、或它是否内建——所有标签一套规则。

#### Scenario: 自定义效果声明并入名字空间

- **WHEN** 顶层出现 `effect db`
- **THEN** 它声明效果 `db`，camelCase，入模块单一名字空间——同模块再一个 `effect db` 或一个 `fn db` 皆 `E0404:` duplicate name in one module

#### Scenario: 内建名被拒作自定义效果

- **WHEN** 出现 `effect io`、`effect net` 或 `effect time`
- **THEN** 编译器以 `E1403:` effect name conflicts with a built-in effect 拒绝

#### Scenario: 内建标签不是模块名字

- **WHEN** 模块声明 `fn time(n: Int64) -> Int64` 或绑定 `let net = 3`
- **THEN** 二者皆合法：内建标签是语言级的，非模块项，故不占模块名字空间中的任何名字——`E1403` 只管以内建名声明效果，从不管把该词作普通名字使用

#### Scenario: pub 效果跨模块合格到达

- **WHEN** 模块 B 声明 `pub effect db` 且模块 A import B
- **THEN** A 的签名以 `b.db` 合格该标签；检查与治理其本模块裸标签者是同一套

#### Scenario: 未解析标签被拒绝

- **WHEN** 效果段持有无任何内建、声明或合格 import 提供的标签
- **THEN** 编译器按第 15 章解析以 `E1304:` unresolved name 拒绝

### Requirement: 效果段

fn 声明与接口方法签名 MAY 在参数表与箭头（或无返回类型时的体）之间携带效果段：`effect tag1 tag2 ...`——`effect` 关键字引出一个或多个空格分隔的标签。省略声明纯函数；有段声明该函数可执行恰这些效果。函数体内——defer 体在内，它运行于出口但属于外围函数自身的范围——每个调用的效果集 MUST 是声明集的子集；要求段外效果的调用 MUST 以 `E1401:` undeclared effect at a call 拒绝，报文指名效果与被调用者。Defer 只偏移执行时机，从不偏移效果归属：defer 体的调用计入外围函数的声明效果（第 3 章指针句，落定）。第 14 章的 panic 家族不带效果：终止不是副作用，`panic`、`todo` 与 `assert` 在任何函数中皆可调用。并发章的 task 块在拼写 `task effect tag1 tag2 ... block` 之下携带本条的效果段——在那里 REQUIRED 且绝不可选：任务运行于所有外围 extent 之外，其体的调用对自身声明集负责、绝不对外围声明负责。并发章的等待同样不带效果：阻塞于锁、条件变量、信号量许可、通道、任务完成或取消信号，不读环境也不写环境——等待不是副作用，原语的等待方法在任何函数中皆可调用，且 scope 块的整个 extent 不对任何外围声明负责。测试章的 test 块不携段——它没有可携的段——`E1401` 不在其内的调用处触发：测试是驱动代码，它所驱动之物的效果真相站在那段代码自己的声明里。test 块内的 mock 声明逐字携被 mock 声明的段，其体恰对该集受检——第 20 章的 Requirement 携带这两条规则。

#### Scenario: 段声明并受检

- **WHEN** 出现 `fn write(msg: String) effect io { save(msg) }` 且 `save` 声明 `effect io`
- **THEN** 段解析通过，`save` 的集在声明集内，声明合法

#### Scenario: 段携带多个标签

- **WHEN** 出现 `fn sync(rows: List<Int64>) effect io net { send(persist(rows)) }`，`send` 声明 `effect net`、`persist` 声明 `effect io`
- **THEN** 双标签段解析通过，两个调用都在声明集内；声明多于体所执行者合法，执行超出段者不合法

#### Scenario: 调用处的未声明效果被拒绝

- **WHEN** 出现 `fn greet() { save("hi") }` 且 `save` 声明 `effect io`
- **THEN** 编译器以 `E1401:` undeclared effect at a call 拒绝——io 不在声明集内；修复是声明该段或改调纯函数

#### Scenario: 省略声明纯度

- **WHEN** fn 声明不携带效果段
- **THEN** 其体只可调用纯函数；体内任何效果调用皆 `E1401:` undeclared effect at a call

#### Scenario: 无返回类型的段

- **WHEN** 出现 `fn log(msg: String) effect io { save(msg) }`——段在、无 `->` 类型
- **THEN** 段位于参数表与体之间并解析通过；返回类型的缺席与段相互独立

#### Scenario: defer 体的效果归属外围函数

- **WHEN** 不声明效果的函数持有 `defer { save("x") }` 且 `save` 声明 `effect io`
- **THEN** 编译器以 `E1401:` undeclared effect at a call 拒绝；defer 只偏移时机、从不偏移效果归属——外围函数要么声明 io、要么 defer 体改调纯代码

#### Scenario: panic 家族不带效果

- **WHEN** 纯函数体调用 `panic("unreachable")` 或 `assert(n > 0, "positive")`
- **THEN** 不触发任何效果诊断；终止不是副作用

#### Scenario: task 块的效果段是本条的拼写

- **WHEN** 出现 `task effect net { fetchA() }` 且 `fetchA` 声明 `effect net`
- **THEN** 段即本条的——`effect` 加裸标签——体的调用依并发章的规则对照任务自身的声明集检查

#### Scenario: 等待不带效果

- **WHEN** 不声明效果段的函数持有 `slot.acquire()` 或 `ch.send(v)`
- **THEN** 不触发任何效果诊断；在并发原语上等待不是副作用

#### Scenario: test 体不受效果检查

- **WHEN** test 块的体调用一个声明 `effect io` 的函数而 test 块什么都不声明
- **THEN** 不触发任何效果诊断——test 块没有可对照检查的段，测试章之修订；被调用声明自己的段是效果真相所在

### Requirement: 函数类型中的效果段

函数类型的效果段位于参数类型与箭头之间，为裸空格分隔标签——类型位无 `effect` 关键字：`fn(T1, ..., Tn) tag1 tag2 -> T`。省略声明纯函数类型。分裂拼写是批准者：声明以 `effect` 引段，类型携带裸标签——两个表面各持自己的引导词，任一拼写出现在对方位置都不被接受（`E0105`）。在持有函数值的每个类型一致位——绑定对注解、实参对参数、返回对声明类型——值的效应集 MUST 是期望类型集的子集；要求期望类型未声明之效果的值 MUST 以 `E1402:` function value effect set does not match the expected type's 拒绝。子集方向是刻意的单向：更纯的值满足期待效果的槽——它只会做得更少——而效果值入纯槽是 `E1402` 的。

#### Scenario: 类型段裸拼写解析

- **WHEN** 注解持有 `fn(Int64) io -> Int64`
- **THEN** 它是可执行 io 的 Int64 到 Int64 函数的类型，依修订后的第 12 章

#### Scenario: 纯槽中的效果值被拒绝

- **WHEN** 体调用 io 声明代码的闭包、或 io 声明的 `save` 本身作 fn 名，绑定为 `let f: fn(Int64) -> Int64 = ...`
- **THEN** 编译器以 `E1402:` function value effect set does not match the expected type's 拒绝——io 不在期望集内；一致位规则不问值以何为名

#### Scenario: 纯值满足期待效果的槽

- **WHEN** 出现 `let f: fn(Int64) io -> Int64 = |n| n + 1`——闭包推断集为空
- **THEN** 绑定合法；空集是 io 的子集，做得更少总是安全

#### Scenario: 调用带效果类型的参数计入声明

- **WHEN** 出现 `fn apply(f: fn(Int64) io -> Int64) { f(3) }`——参数的类型声明 io，而 `apply` 无声明
- **THEN** 编译器以 `E1401:` undeclared effect at a call 拒绝——经 `f` 的调用执行 `f` 的类型所声明者；写作 `fn apply(f: fn(Int64) io -> Int64) effect io -> Int64 { f(3) }` 则合法，段与调用相抵

#### Scenario: 两套拼写互不越位

- **WHEN** fn 声明写裸标签（`fn f() io { }`）或函数类型写关键字（`fn(Int64) effect io -> Int64`）
- **THEN** 编译器按第 2 章意外 token 诊断（`E0105`）拒绝；两个表面各持自己的引导词

### Requirement: 闭包效果集推断

两种闭包形式——完整 `fn(params) -> type block` 与短式 `|p| expr`——都无可写的效果段；其效果集自体调用推断，并在每个一致位按子集规则受检（`E1402`）。闭包的推断集是其体调用集的并，恰如其值类型由其体按第 12 章定——两种形式一条推断规则，不存在也不新增注解位。

#### Scenario: 完整闭包的集自体推断

- **WHEN** 出现 `let f = fn(n: Int64) -> Int64 { write(log, n); n + 1 }` 且 `write` 声明 `effect io`
- **THEN** `f` 的类型携带推断集 {io}：依修订后的第 12 章为 `fn(Int64) io -> Int64`

#### Scenario: 短闭包同法推断

- **WHEN** 出现 `items.map(|s| parse(s))` 且 `parse` 纯——短式与完整式恰同一受检
- **THEN** 闭包的集为空；若 `parse` 声明效果，同一并集规则将携带之

#### Scenario: 推断在一致位受检

- **WHEN** 上携 io 的 `f` 被传入期待 `fn(Int64) -> Int64` 之处
- **THEN** 编译器以 `E1402:` function value effect set does not match the expected type's 拒绝

#### Scenario: 构造效果闭包不是执行它

- **WHEN** 纯函数体持有 `let f = |s: String| { save(s) }`——构造 io 推断闭包而不调用——并返回 `f`
- **THEN** 纯声明合法：推断集随值走，不污染构造函数；唯调用——直接 `save`、体内 `f("x")`、或 `f` 逸入带效果类型的槽——才参与任何检查

### Requirement: 接口与 impl 中的效果

接口方法签名 MAY 携带效果段——声明拼写、`effect` 关键字——impl 方法的效果集 MUST 与接口声明精确相等，缺标签与多标签同以 `E1404:` impl method effect set disagrees with the interface 拒绝。默认方法的段（第 10 章）治理其体与每个 override，同一精确一致规则约束 override 与其替换的签名。经接口值的调用——`Dyn<Interface>` 箱或泛型约束——按接口声明集受检：静态分发没有效果盲区，无论哪个实现运行。

#### Scenario: 接口方法声明段

- **WHEN** 出现 `interface Store { fn get(mut self, k: String) effect io -> String }`
- **THEN** 签名携带其段解析通过；实现被绑到恰 {io}

#### Scenario: impl 集必须精确一致

- **WHEN** `Store` 对 `FileStore` 的 impl 声明 `fn get(mut self, k: String) effect net -> String`
- **THEN** 编译器以 `E1404:` impl method effect set disagrees with the interface 拒绝——net 不是 io，缺标签与多标签同为分歧

#### Scenario: 默认方法 override 同样匹配段

- **WHEN** 接口声明带默认体的 `fn reload(mut self) effect io`，impl 无段 override 之，其体调用 io 声明代码
- **THEN** override 被拒绝：弃接口之 io 出签名是 `E1404`，impl 签名为纯而体有效果调用是 `E1401`——override 在同一精确一致规则下替换默认

#### Scenario: 无段接口的无段 impl 是纯的

- **WHEN** 接口方法不声明段而 impl 体调用 io 声明代码
- **THEN** 该调用是 `E1401:` undeclared effect at a call；impl 签名是纯的，其体必须也是

#### Scenario: 经接口的调用按接口集受检

- **WHEN** `Dyn<Store>` 值的 `get` 在声明 `effect io` 的函数内被调用
- **THEN** 调用合法——接口声明的集是调用方所见，无论哪个实现运行

### Requirement: 模块初始化期的效果检查

顶层 let 初始化子是唯一在任何函数签名之外求值的批准位置，其规则固定于此：初始化子 MUST 纯。调用效果函数的初始化子 MUST 以 `E1405:` effectful call in a top-level initializer 拒绝——进程启动工作归 `main`，使第 15 章的急切初始化在内容上与次序上同样确定（第 0 章 Principle 1）。

#### Scenario: 纯初始化子合法

- **WHEN** 顶层出现 `pub let retries = 3` 或 `pub let name = "svc"`
- **THEN** 初始化子合法；字面量与纯调用总是合法

#### Scenario: 效果初始化子被拒绝

- **WHEN** 出现 `pub let host = loadHost()` 且 `loadHost` 声明 `effect io`
- **THEN** 编译器以 `E1405:` effectful call in a top-level initializer 拒绝——把工作移入 main

#### Scenario: 初始化内容保持确定

- **WHEN** 每个模块的初始化子都纯
- **THEN** 进程启动不做任何环境读写；模块初始化是常量上的确定计算，无论哪个平台运行它

### Requirement: 效果诊断段位

效果章拥有注册表段位 `E1400`–`E1499`，在 `docs/spec/diagnostics.toml` `[segments]` 声明，owner 为 `1600-effects`。分配：`E1401` undeclared effect at a call、`E1402` function value effect set does not match the expected type's、`E1403` effect name conflicts with a built-in effect、`E1404` impl method effect set disagrees with the interface、`E1405` effectful call in a top-level initializer。`E1400` 与 `E1406`–`E1499` 留作本章修订；需要段位码的后来章节认领自己的段位。

#### Scenario: 段位可检索

- **WHEN** 诊断消费者在注册表查任一 `E14xx` 码
- **THEN** 段位条目指名 owner `1600-effects`，每个已分配码的条目携带 severity、title、description、remediation、owner、requirement 与 allocated 日期

#### Scenario: 后续变更在段内扩展

- **WHEN** 本章后来的 spec 层变更需要新码
- **THEN** 它在自己的变更内从 `E1400`–`E1499` 分配；效果之外的章节认领别的段位

## 示例（非权威）

下面的示例只用第 1–16 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。

### 效果声明

```we
effect db                              // camelCase, joins the module's one
                                       // name space (E0404 on collision)
effect io
// E1403: effect name conflicts with a built-in effect — io, net, time are the
// language's own tags

pub effect audit                       // cross-module: qualified b.audit

fn write(msg: String) effect io {      // declared set {io}
    save(msg)                          // save declares effect io: within
}

fn greet() {
    save("hi")
    // E1401: undeclared effect at a call — io is not in the declared set
}
```

### 段与 defer

```we
fn log(msg: String) effect io {        // segment without a return type
    save(msg)
}

fn pure(n: Int64) -> Int64 {
    assert(n > 0, "positive")          // panic family carries no effect
    panic("unreachable")               // callable from any function
}

fn bad() {
    defer { save("bye") }
    // E1401: undeclared effect at a call — defer shifts timing, never effect
    // attribution; the enclosing function must declare io
}
```

### 函数类型与闭包

```we
let f: fn(Int64) io -> Int64 = fetch   // bare tags in type position
let g: fn(Int64) -> Int64 = |n| n + 1  // pure value in effect-expecting slot
                                       // is legal: empty set ⊆ {io}

let h: fn(Int64) -> Int64 = fetch
// E1402: function value effect set does not match the expected type's — io
// is not in the expected set

fn k() io { }                          // E0105: bare tags belong to type
                                       // position; declarations use `effect`

fn calc(rows: List<Int64>) -> Int64 {
    rows.map(|r| score(r))             // short closure's set inferred from
}                                      // body: score is pure, set is empty
```

### 接口与 impl

```we
interface Store {
    fn get(mut self, k: String) effect io -> String
}

impl Store for FileStore {
    fn get(mut self, k: String) effect net -> String { ... }
    // E1404: impl method effect set disagrees with the interface — net is
    // not io
}

fn use(store: Dyn<Store>) effect io -> String {
    return store.get("k")              // checked against the interface's
}                                      // set, whichever implementation runs
```

### 初始化子保持纯

```we
pub let retries = 3                    // literals and pure calls are fine

pub let host = loadHost()
// E1405: effectful call in a top-level initializer — move the work into main
```

## 术语对照

本章关键术语，英中对译，以保翻译一致：

| English | 中文 |
| --- | --- |
| effect | 效果 |
| effect tag | 效果标签 |
| effect declaration | 效果声明 |
| built-in tag | 内建标签 |
| effect segment | 效果段 |
| declared effect set | 声明效果集 |
| pure function | 纯函数 |
| effect set | 效果集 |
| subset check | 子集检查 |
| exact match | 精确一致 |
| agreement position | 一致位 |
| attribution | 归属 |
| initializer | 初始化子 |
| diagnostics segment | 诊断段位 |
