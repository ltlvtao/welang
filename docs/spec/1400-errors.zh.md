# We 语言规范 —— 第 14 章：错误机制


### Requirement: Result 类型

标准库依第 9 章的声明形式将规范结果类型声明为普通泛型 sum：`pub type Result<T, E> = Ok(T) | Err(E)`，`Ok` 与 `Err` 是标准库作用域内的普通 PascalCase 变体名，如同 `Some` 与 `None`。构造、模式与穷尽性是第 9 章与第 4 章的既有机制；本章不新增任何匹配规则。错误参数 `E` MUST 是命名 sum 类型——标准库或用户代码中以名字声明的 sum 类型。错误位上的基类型（含 `String`）、记录、接口、`Dyn` 盒或泛型参数以 `E1204:` Result error type is not a named sum type 拒绝。该约束是穷尽性的：命名 sum 的变体集封闭，故对 `Result` 的 match 覆盖 `Ok` 与 `Err` 且 `E` 的全部变体可见；泛型参数位不提供这种封闭，且本规范不设 is-a-sum 约束。丢弃 `Result` 值遵循第 8 章的丢弃规则：非 unit 表达式的值不得静默丢弃，拼写是 `let _ = expr`（`E0605`），不存在 `.ignore()` 方法。绑定后未用不被强制：对 `Result` 或任何类型都没有未用绑定检查。

#### Scenario: Result 按既有机制穷尽匹配

- **WHEN** `r: Result<Int64, AppError>` 以 `match r { Ok(n) => n, Err(e) => match e { ... } }` 匹配，内层 match 覆盖每个 `AppError` 变体
- **THEN** 该 match 依第 9 章泛型 sum 规则穷尽；本章没有新匹配规则被援引

#### Scenario: 错误位基类型被拒绝

- **WHEN** `Result<Int64, String>` 作为类型表达式出现
- **THEN** 编译器以 `E1204:` Result error type is not a named sum type 拒绝；`String` 不指名任何 sum

#### Scenario: 错误位泛型参数被拒绝

- **WHEN** 函数声明 `fn run[E](r: Result<Int64, E>)`——错误位是泛型参数
- **THEN** 编译器以 `E1204:` Result error type is not a named sum type 拒绝；对错误类型做泛型抽象的助手不可表达

#### Scenario: 丢弃在丢弃点显式

- **WHEN** `readFile(path)` 作为表达式语句出现，或其值被绑定而未用
- **THEN** 表达式语句以第 8 章的 `E0605` 拒绝；绑定未用形式被接受——强制只发生在丢弃点，拼写是 `let _ = readFile(path)`

### Requirement: 传播运算符

传播运算符是后缀 `expr?`——第 1 章的 `?` token，经第 2 章的后缀修订获得文法。其操作数 MUST 是 `Result<T, E_src>` 类型；任何其他类型的操作数（`Option` 在内）以 `E1201:` operand of ? is not a Result type 拒绝。它只在活的传播上下文中合法：最内层包围的函数或完整闭包 MUST 声明形如 `Result<U, E_dst>` 的返回类型，短闭包的 `?` 则相应把其推断值类型定为 `Result<U, E_dst>`——`?` 从该最内层体自身返回，绝不从闭包所嵌于或被传入的函数返回。defer 体运行于出口、没有可传播的目标返回，模块顶层没有包围函数：那里的 `?`，以及任何最内层函数返回非 Result 类型的体中的 `?`，以 `E1202:` ? outside a function returning Result 拒绝。`E_src` MUST 是 `E_dst`——同一命名类型——否则编译器以 `E1203:` ? error type disagrees with the declared error type 拒绝；不存在隐式包装或提升（单一错误机制）：错误转换是显式的——`match`，或未来变更批准后的标准库组合子。`Ok(t)` 时表达式的值是 `t`，类型 `T`；`Err(e)` 时包围函数或闭包在该点返回 `Err(e)`——第 13 章返回通道意义下的提前返回，其资源义务如同任何 return 一样随行。该后缀与成员访问、调用成链（`f()?.name`、`g(x)?.next()`），依第 2 章档 1 与任何后缀同等紧密地结合。

#### Scenario: Ok 解包为载荷

- **WHEN** 求值 `let n = readFile(path)?` 且 `readFile` 产出了载荷类型 `Int64` 的 `Ok(42)`
- **THEN** `n` 是 `Int64` 类型的 `42`；执行在下一条语句继续

#### Scenario: Err 从上下文函数早返回

- **WHEN** 同一语句被求值且 `readFile` 产出了 `Err(e)`
- **THEN** 包围函数在该点返回 `Err(e)`；传播之后的语句不运行

#### Scenario: 非 Result 操作数被拒绝

- **WHEN** 出现 `let n = find(id)?`，`find` 返回 `Option<Int64>`，或 `let m = 5?`
- **THEN** 编译器以 `E1201:` operand of ? is not a Result type 拒绝；`Option` 不传播

#### Scenario: 非 Result 函数上下文被拒绝

- **WHEN** 声明 `-> Int64` 的函数含 `readFile(path)?`
- **THEN** 编译器以 `E1202:` ? outside a function returning Result 拒绝，具名包围函数

#### Scenario: 模块顶层的传播被拒绝

- **WHEN** `readFile(path)?` 出现在模块顶层，不在任何函数体内
- **THEN** 编译器以 `E1202:` ? outside a function returning Result 拒绝；没有包围函数

#### Scenario: defer 体内的传播被拒绝

- **WHEN** 出现 `defer { readFile(path)? }`——传播后缀在 defer 体内
- **THEN** 编译器以 `E1202:` ? outside a function returning Result 拒绝；defer 体运行于出口，没有可传播的目标返回

#### Scenario: 闭包从自身传播

- **WHEN** 返回 `Int64` 的函数内出现 `items.map(fn(x: String) -> Result<Int64, AppError> { parse(x)? })`
- **THEN** `?` 合法：其上下文是声明了 `Result` 返回的闭包；闭包从自身返回，而非从包围函数

#### Scenario: 短闭包的 ? 定其值类型

- **WHEN** 出现 `let f = |s: String| parse(s)?`，`parse` 类型为 `fn(String) -> Result<Int64, ParseError>`
- **THEN** 闭包的值类型是 `fn(String) -> Result<Int64, ParseError>`：`?` 从闭包自身返回，顶层绑定合法，因为 `?` 的最内层包围函数是该闭包

#### Scenario: 错误类型不一致被拒绝

- **WHEN** 声明 `-> Result<Int64, AppError>` 的函数含 `parse(s)?`，`parse` 返回 `Result<Int64, ParseError>`，`AppError` 与 `ParseError` 是不同的命名 sum
- **THEN** 编译器以 `E1203:` ? error type disagrees with the declared error type 拒绝；转换是显式写出的 `match`

#### Scenario: 后缀紧密成链

- **WHEN** 解析 `f()?.name`
- **THEN** 依第 2 章档 1 分组为 `(f()?).name`——传播后缀与调用、成员访问同等紧密地结合，成员访问落在载荷上

### Requirement: panic 家族

标准库以普通函数声明终止形式：`pub fn panic(msg: String) -> Never`、`pub fn todo(msg: String) -> Never` 与 `pub fn assert(cond: Bool, msg: String)`。不新增关键字、不新增文法产生式：它们像任何函数一样被调用，调用的实参表按任何调用检查（第 6、7、10 章）；消息实参由声明要求——无解释的终止不可审计，缺消息不引入专码。`panic` 与 `todo` 产出 `Never`（第 9 章的底类型）：调用满足任何声明返回类型，控制绝不越过它继续。`todo` 是消息里带意图的 `panic`——未写成的代码自我标记且保持可审计。`assert` 先求值其条件：真时产出 unit 值、执行继续；假时它是携带消息的 `panic`。任何形式都不可捕获 panic：文法中不存在 `catch`、`recover`、`try` 形式，且不经本 Requirement 的修订它们不得进入——错误只经 `Result` 传播（第 0 章 Principle 9）；panic 是终止，不是错误通道。运行时整数溢出（第 7 章）以指名操作的 `panic` 陷阱——`Int64 add overflow` 及其同族——即第 7 章延后至本章的检查陷阱构造。

#### Scenario: panic 产出 Never

- **WHEN** 声明 `fn find(id: Int64) -> Int64 { if id == 0 { panic("no such id") } else { id } }`
- **THEN** `panic` 调用依第 9 章底类型规则满足 `Int64` 返回类型；控制绝不越过调用继续

#### Scenario: todo 标记未写代码

- **WHEN** `todo("parsing not written yet")` 出现在声明 `-> Int64` 的函数的返回位置
- **THEN** 代码通过检查——调用是 `Never` 类型——运行时它携带消息终止

#### Scenario: assert 真值通过

- **WHEN** 以 `n == 5` 求值 `assert(n > 0, "n must be positive")`
- **THEN** 调用产出 unit 值，执行在下一条语句继续

#### Scenario: assert 假值终止

- **WHEN** 同一调用以 `n == 0` 求值
- **THEN** 它是携带消息 `"n must be positive"` 的 `panic`；控制绝不越过调用继续

#### Scenario: 运行时溢出以指名 panic 陷阱

- **WHEN** `Int64` 加法在运行时溢出
- **THEN** 操作以指名操作的 `panic` 陷阱，绝不产出回绕值——第 7 章的检查陷阱构造，由本章指名

#### Scenario: 不存在捕获形式

- **WHEN** `catch`、`recover` 或 `try` 作为形式出现
- **THEN** 编译器以第 2 章的意外 token 诊断（`E0105`）拒绝；不存在此类产生式，且不经本 Requirement 的修订不得进入

### Requirement: 展开

panic 在飞展开调用栈：每个进行中的块按块出口退出。scope-resource 块按第 13 章的保证释放——每绑定恰一次 `release` 调用、按声明逆序——函数的 defer 语句在内块释放之后按语句逆序运行，内块先出；顺序即第 13 章为正常出口定下的顺序，经本 Requirement 扩展至展开出口。展开期间资源不转移：释放只属于块出口机制（第 13 章的单一释放触发点在每种出口上都成立）。栈尽时进程以 panic 的消息中止；该中止是本规范定义的捕获边界——语言的任何表达式都观察不到 panic。并发章节若批准，经本 Requirement 的修订将任务内 panic 绑定到同一机制（第 0 章 Principle 9）：任务边界成为把展开中 panic 转为 `Err` 的捕获边界；无第二通道进入。

#### Scenario: 展开释放资源

- **WHEN** panic 在先绑 `a` 后绑 `b` 的 scope resource 语句进行中触发
- **THEN** `b` 的释放先运行、然后 `a` 的——声明逆序——先于包围函数自己的 defer 运行

#### Scenario: defer 在出口途中运行

- **WHEN** 函数体含 `defer { a() }` 与其后的 `defer { b() }`，panic 在内部触发
- **THEN** 内块释放之后，`b()` 先运行、然后 `a()`——语句逆序，与正常出口同一顺序

#### Scenario: 中止即捕获边界

- **WHEN** 展开完成且无剩余帧
- **THEN** 进程以 panic 的消息中止；不存在处理器，不经 panic 家族 Requirement 的修订也写不出处理器

### Requirement: 单一错误机制

错误只经一种机制流动：声明在签名中的 `Result` 值、经 `?` 传播、经 `match` 消费——第 0 章 Principle 9 的操作化。本章固定其后果。会失败的函数在其返回类型中说明：不存在 throws 子句、不存在 effect 段逃逸（第 12 章未批准任何 effect 段）、不存在未检查通道。`?` 是唯一的传播运算符：没有其他后缀或前缀形式传播——`expr!` 与 `try expr` 不匹配第 2 章文法的任何产生式。失败跨函数边界即签名中的值，故每条失败路径在调用点可见，纪律局部可判定（第 0 章 Principle 1）。panic 是终止，不是错误通道：它不携带载荷类型、不匹配任何东西、不被任何东西捕获。

#### Scenario: 不存在其他传播拼写

- **WHEN** `expr!` 或 `try f()` 出现在表达式位置
- **THEN** 编译器以第 2 章的意外 token 诊断（`E0105`）拒绝；传播后缀只有 `?`

#### Scenario: 失败在签名中可见

- **WHEN** 调用方看到 `fn parse(s: String) -> Result<Config, ParseError>`
- **THEN** 每条失败路径都声明在签名里；没有东西隐式传播，调用方的处理只在调用点即可检查

### Requirement: 错误诊断段位

错误章拥有注册表段位 `E1200`–`E1299`，在 `docs/spec/diagnostics.toml` `[segments]` 声明，owner 为 `1400-errors`。分配：`E1201` operand of ? is not a Result type、`E1202` ? outside a function returning Result、`E1203` ? error type disagrees with the declared error type、`E1204` Result error type is not a named sum type。`E1200` 与 `E1205`–`E1299` 留作本章修订——需要段位码的后来章节认领自己的段位。

#### Scenario: 段位可检索

- **WHEN** 诊断消费者在注册表查任一 `E12xx` 码
- **THEN** 段位条目指名 owner `1400-errors`，每个已分配码的条目携带 severity、title、description、remediation、owner、requirement 与 allocated 日期

#### Scenario: 后续变更在段内扩展

- **WHEN** 本章后来的 spec 层变更需要新码
- **THEN** 它在自己的变更内从 `E1200`–`E1299` 分配；错误之外的章节认领别的段位

## 示例（非权威）

下面的示例只用第 1–13 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。

### Result 与其错误位

```we
type AppError = NotFound(String) | Denied(String)   // a named sum: a legal
                                                    // error parameter
type Lookup = Result<Int64, AppError>               // Ok(Int64) | Err(AppError)

let bad: Result<Int64, String> = make()
// E1204: Result error type is not a named sum type — String names no sum

fn run[E](r: Result<Int64, E>) { }
// E1204: Result error type is not a named sum type — the generic parameter
// admits no closure either
```

### 用 ? 传播

```we
fn readConfig(path: String) -> Result<String, AppError> {
    let text = readFile(path)?             // Err(AppError) returns here on
    return Ok(text)                        // error; Ok unwraps to String
}

fn lenOf(path: String) -> Result<Int64, ParseError> {
    let text = readFile(path)?
    // E1203: ? error type disagrees with the declared error type — convert
    // by match, written explicitly
    return Ok(text.len())
}
```

### 上下文规则

```we
fn firstLine(s: String) -> Int64 {
    let text = parse(s)?
    // E1202: ? outside a function returning Result — the context function
    // returns Int64
    return text.len()
}

fn guard() {
    defer { readFile("a")? }
    // E1202: ? outside a function returning Result — a defer body runs at
    // exit; no return to propagate to
}

fn deep(id: Int64) -> Int64 {
    let n = find(id)?
    // E1201: operand of ? is not a Result type — Option does not propagate
    return n
}
```

### panic 家族与展开

```we
fn find(id: Int64) -> Int64 {
    if id == 0 { panic("no such id") }     // Never: satisfies Int64
    return lookup(id)
}

fn half(m: Int64) -> Int64 {
    todo("half not written yet")           // panic with intent in the
}                                          // message; checks as Never

fn port(n: Int64) -> Int64 {
    assert(n > 0, "n must be positive")    // true: unit, continue;
    return n / 2                           // false: panic with the message
}

fn counted() {
    scope resource(a = openFile("a"), b = openFile("b")) {
        panic("boom")                      // unwinds: b released, then a,
    }                                      // then this function's defers,
    defer { log() }                        // then the abort — no handler
}
```

### 单一机制

```we
let v = try parse("1")
// E0105: unexpected token — no try form; ? is the only propagation operator

let w = parse("1")!
// E0105: unexpected token — no ! postfix; the propagation postfix is ? alone
```

## 术语对照

本章关键术语，英中对译，以保翻译一致：

| English | 中文 |
| --- | --- |
| error | 错误 |
| error mechanism | 错误机制 |
| error type | 错误类型 |
| result type | 结果类型 |
| propagation | 传播 |
| propagation operator | 传播运算符 |
| propagation context | 传播上下文 |
| payload | 载荷 |
| early return | 早返回 |
| termination | 终止 |
| panic family | panic 家族 |
| unwinding | 展开 |
| block exit | 块出口 |
| capture boundary | 捕获边界 |
| abort | 中止 |
| assertion | 断言 |
| named sum type | 命名 sum 类型 |
| exhaustiveness | 穷尽性 |
| lifting | 提升 |
| discard | 丢弃 |
| single error mechanism | 单一错误机制 |
