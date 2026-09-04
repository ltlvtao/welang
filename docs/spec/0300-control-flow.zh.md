# We 语言规范 —— 第 3 章：控制流

### Requirement: If 与 else 表达式

if 表达式是 `if cond block` 或 `if cond block else else-arm`，其中 `cond` 是表达式，`else-arm` 是块或另一个 if 表达式（链终止于块或无 else 的 if）。无 else 的 if 无值，MUST NOT 出现在要求值的位置（`E0202`——由缺失的 else 静态可判定）。有 else 的 if 是表达式形式：每次求值其值为所取分支臂的块值，按第 2 章"块与块值"。有 else 的 if 的各臂 MUST 依第 8 章类型一致：无值臂的类型是 unit 类型，不一致以第 7 章 `E0501` 拒绝；never 类型对一致的排除归 sum 类型章。本章只定形与臂作用域。每个分支臂引入自己的作用域：臂内绑定 MUST NOT 泄漏到 if 之后。条件的类型化——须为布尔类型——由类型章节批准。

#### Scenario: 有 else 的 if 用作值

- **WHEN** 解析 `let x = if c { 1 } else { 2 }` 且 `c` 为真
- **THEN** `x` 取 then 臂的块值，此处为 `1`；臂块值遵循第 2 章不变

#### Scenario: 无 else 的 if 用于取值位置

- **WHEN** 出现 `let x = if c { 1 }`——无 else 的 if 用于要求值的位置
- **THEN** 编译器以 `E0202:` valueless form in value position 拒绝，具名无值的形式

#### Scenario: 臂绑定不泄漏

- **WHEN** 某臂绑定名字，例如 `if c { let a = 1 a } else { 0 }`
- **THEN** `a` 在 if 表达式之后不可见；每个臂是自己的作用域

#### Scenario: else-if 链

- **WHEN** 解析 `if c1 { a } else if c2 { b } else { c }`
- **THEN** else 臂持有一个嵌套 if 表达式；链是两个 if 表达式，各自遵循本条

#### Scenario: 一臂有值一臂无值

- **WHEN** `if c { 1 } else { io.println("no") }` 出现在表达式位置
- **THEN** 编译器依第 8 章以 `E0501` 拒绝：第一臂是 `Int64`，第二臂是 unit 类型；各臂必须一致

### Requirement: While 与 loop

while 语句是 `while cond block`；loop 语句是 `loop block`。二者都是语句：不产出值，MUST NOT 出现在要求值的位置（`E0202`）。其体遵循第 2 章块；体内绑定不泄漏过语句。条件的类型化由类型章节批准。

#### Scenario: while 作为语句项

- **WHEN** `while more() { step() }` 作为块项出现
- **THEN** 解析为 while 语句；体是第 2 章块

#### Scenario: loop 用于取值位置

- **WHEN** 出现 `let x = loop { step() }`——loop 用于要求值的位置
- **THEN** 编译器以 `E0202:` valueless form in value position 拒绝

### Requirement: Break 与 continue

`break` 与 `continue` 是裸语句形式：不携值、不接受标签。它们只在归属章节批准的循环体（本章 while 与 loop；迭代章节批准 for）内合法。在这种体外，编译器以 `E0201` 拒绝。两种形式都不产出值。

#### Scenario: 循环内的 break

- **WHEN** 解析 `loop { if done() { break } step() }`
- **THEN** break 合法：退出最内层包围循环

#### Scenario: 任何循环之外的 continue

- **WHEN** `continue` 出现在不在循环体内的块中
- **THEN** 编译器以 `E0201:` break or continue outside a loop 拒绝

#### Scenario: 带标签的 break 未批准

- **WHEN** 出现 `break 'outer` 或任何带标签的 break/continue 形式
- **THEN** 编译器按未批准产生式拒绝（第 2 章意外 token 诊断）；标签只能经 spec change 进入

### Requirement: Return

`return` 是两种形式的语句：裸 `return` 与 `return expr`。本章只批准形式：return 在何处合法（函数体）、其表达式类型与外围函数的关系由声明章节批准，never 类型交互由类型章节批准。

#### Scenario: 带表达式的 return

- **WHEN** `return a + b` 作为项出现
- **THEN** 解析为携带一个表达式操作数的 return 语句

#### Scenario: 裸 return

- **WHEN** `return` 单独作为项出现
- **THEN** 解析为裸 return；其合法性与类型化遵循声明章节

### Requirement: Defer

defer 语句是 `defer block`——体 MUST 是块；任何其他操作数以 `E0203` 拒绝。Defer 只作为函数体块的直接项才合法；任何其他位置以 `E0204` 拒绝（函数体由声明章节批准；在那之前可触发的是拒绝侧）。同一函数体中的多条 defer 在函数退出时逆序执行，包括经提前 return 或 break 的退出。Defer 只偏移执行时机；它不是资源安全保证——那是第 13 章的 scope resource，其块出口释放先于外围函数自己的 defer 运行，内块先出。defer 体内的错误传播是第 14 章的规则：`?` 运算符在那里被拒绝（`E1202`）——defer 体运行于出口、没有可传播的目标返回。defer 体内的效果规则归第 16 章：defer 体的调用计入外围函数的声明效果（`E1401`），因 defer 只偏移执行时机、从不偏移效果归属——defer 不引入对二者的任何例外。

#### Scenario: Defer 体必须是块

- **WHEN** 出现 `defer cleanup()`——defer 的操作数是表达式而非块
- **THEN** 编译器以 `E0203:` defer body must be a block 拒绝

#### Scenario: 嵌套块内的 defer

- **WHEN** 出现 `let x = { defer { log() } compute() }`——defer 在嵌套块表达式内而非函数体直接项
- **THEN** 编译器以 `E0204:` defer placement 拒绝，具名不是函数体的包围块

#### Scenario: defer 体内的传播

- **WHEN** 出现 `defer { readFile(path)? }`——传播后缀在 defer 体内
- **THEN** 编译器以 `E1202:` ? outside a function returning Result 拒绝；defer 体运行于出口，没有可传播的目标返回

#### Scenario: 退出时逆序

- **WHEN** 函数体含 `defer { a() }`，其后又有 `defer { b() }`，函数退出
- **THEN** `b()` 先于 `a()` 运行；defer 按语句逆序执行

### Requirement: 控制流诊断段位

控制流章节拥有注册表 `docs/spec/diagnostics.toml` `[segments]` 声明的段位 `E0200`–`E0299`。首批分配：`E0201` 循环外 break/continue、`E0202` 取值位置的无值形式、`E0203` defer 体非块、`E0204` defer 位置。`E0200` 与 `E0205`–`E0299` 预留。触发语义在本章 Requirements；条目在注册表。进一步分配控制流码在同一变更内扩展注册表。

#### Scenario: 某控制流码被发出

- **WHEN** 工具链发出任何 `E02xx` 诊断
- **THEN** 其完整条目可从 `docs/spec/diagnostics.toml` 取得，owner 为 `0300-control-flow`

#### Scenario: 后续变更需要控制流诊断

- **WHEN** 后续章节批准需要新诊断的控制流产生式
- **THEN** 其变更在同一变更内扩展注册表，在 `E0200`–`E0299` 内分配号码

## 示例（非权威）

下面的示例只用第 1–3 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。函数体表面随声明章节到来；defer 的正向用例相应标注。

### if 与 else

```we
let x = if c { 1 } else { 2 }     // 值：所取分支臂的块值
let y = if c { 1 }                 // E0202: valueless form in value position

let z = if c {
    let a = 1
    a                              // then 臂的值
} else {
    0
}
// a 在此处不可见：每个臂是自己的作用域

let grade = if n >= 90 { "high" } else if n >= 50 { "mid" } else { "low" }
// else-if 是持有一个嵌套 if 的 else 臂
```

### while、loop、break、continue

```we
while more() { step() }            // 语句项
let x = loop { step() }            // E0202: 循环不产出值

var found: Option<Item> = None     // 累加器是取值通道
loop {
    let it = next()
    if it.isEnd() { break }        // 裸 break：退出最内层循环
    if !it.matches() { continue }
    found = Some(it)
    break
}

continue                           // E0201: break or continue outside a loop
```

### return

```we
return
return a + b
```

### defer

```we
defer cleanup()                    // E0203: defer body must be a block
let x = {
    defer { log() }                // E0204: defer placement，此块
    compute()                      // 不是函数体
}

// 正向形式（函数体表面随声明章节到来）：
//
// fn work() {
//     defer { close(f) }          // 函数体直接项，OK
//     if bad() { return }         // 提前退出仍运行 defer
//     use(f)
// }                               // 退出时：defer 逆序执行
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| if expression | if 表达式 |
| arm | 分支臂 |
| value position | 取值位置 |
| bare form | 裸形式 |
| reverse order | 逆序 |
| function body | 函数体 |
| statement family | 语句族 |
