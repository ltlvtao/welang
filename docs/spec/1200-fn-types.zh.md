# We 语言规范 —— 第 12 章：函数类型与闭包

### Requirement: fn 类型

fn 类型是 `fn(T1, ..., Tn) [effect-segment] -> T`：`fn` 关键字、圆括号内逗号分隔的零或多个参数类型（每个本身是第 7 章下的类型引用）、参数类型与箭头之间可选的一个或多个裸空格分隔标签的效果段——第 16 章的批准，类型位无 `effect` 关键字——箭头、以及返回类型（本身是类型引用且必写 REQUIRED——不产出值的函数类型为 `fn(...) -> ()`，即第 8 章的 unit 类型）。参数表不设数量上限，与第 6 章声明面一致。fn 类型是单态的：其参数位与返回位只容纳类型引用、永不容纳泛型参数——泛型函数命名一个族而非一个函数，没有单一 fn 类型（"函数值"）。省略段即声明纯函数类型；标签的声明与解析、段参与的一致性检查，归第 16 章。类型槽位依第 7 章的修订接纳 fn 类型为类型引用形式。

#### Scenario: 注解槽位中的 fn 类型

#### Scenario: fn 类型中的效果段

- **WHEN** 出现 `let f: fn(Int64) io -> Int64`——参数类型与箭头之间的裸空格分隔标签
- **THEN** 注解是可执行 io 的 Int64 到 Int64 函数的 fn 类型；段的拼写是裸标签，`effect` 关键字在类型位不合任何产生式

- **WHEN** `let f: fn(Int64) -> Int64` 与右侧匹配的函数值一同出现
- **THEN** 注解即恰该签名的 fn 类型；`f` 绑定不了任何其他签名的函数值

#### Scenario: 零参数 fn 类型

- **WHEN** `fn() -> Bool` 出现在类型槽位
- **THEN** 它是返回 `Bool` 的零参数函数的类型

#### Scenario: 无值签名经 unit 类型类型化

- **WHEN** `fn(String) -> ()` 出现在类型槽位
- **THEN** 它是不产出值的函数的类型；返回位是 unit 类型、永不省略

#### Scenario: 畸形 fn 类型被拒绝

- **WHEN** 类型槽位持有无箭头的 `fn(Int64)` 或任何其他畸形的 fn 类型片段
- **THEN** 编译器在槽位依第 2 章的 unexpected-token 诊断（`E0105`）拒绝；箭头与返回类型是产生式的一部分

#### Scenario: fn 类型嵌于组合位

- **WHEN** 元组类型 `(fn(Int64) -> Int64, Int64)` 或泛型应用 `Box<fn(Int64) -> Int64>` 出现在类型槽位
- **THEN** fn 类型与其他类型引用形式一样；第 8 章的元组与第 10 章的应用规则原样适用

### Requirement: 函数值

单态声明 fn——不带泛型子句的第 6 章顶层 fn——是值：在表达式位置以裸名使用时，其类型即其签名 `fn(ParamTypes) -> ReturnType`，无值声明类型化为 `fn(...) -> ()`。经 fn 类型值发起的调用是第 2 章的后缀调用、与其他无异。函数值 MUST 只在完全一致的签名处与 fn 类型协调：第 7 章的每个一致性位置都适用，错配签名为 `E0501`——无 variance、无参数量或返回放宽，第 10 章的 no-subtyping 立场延伸至函数类型。泛型 fn 的名字不是值：泛型子句使该名成为族，将其用于值位置是 `E1004`；调用形式仍是其唯一用法。方法值不在批准之列：在接收者上解析到方法名而不带调用时依第 10 章解析、但不合任何已批准的值产生式——编译器依第 2 章的 unexpected-token 诊断（`E0105`）拒绝——携带接收者的闭包才是可传递的形式。

#### Scenario: fn 名可绑定可调用

- **WHEN** `square` 声明为 `fn square(x: Int64) -> Int64` 且出现 `let f = square` 与 `f(3)`
- **THEN** `f` 的类型为 `fn(Int64) -> Int64`；调用是第 2 章的后缀调用、产出 `9`

#### Scenario: 签名错配被拒绝

- **WHEN** 出现 `let f: fn(Int64, Int64) -> Int64 = square`，而 `square` 只有一个参数
- **THEN** 编译器以 `E0501` 在绑定与注解的位置拒绝；签名必须完全一致

#### Scenario: 泛型 fn 名用于值位置被拒绝

- **WHEN** 出现 `let g = pairUp`，而声明了 `fn pairUp<T>(a: T, b: T) -> Tuple2<T>`
- **THEN** 编译器以 `E1004:` generic function name in value position 拒绝；调用形式 `pairUp(1, 2)` 不受影响

#### Scenario: 函数值作实参

- **WHEN** 出现 `apply(square, 3)`，而声明了 `fn apply(f: fn(Int64) -> Int64, x: Int64) -> Int64`
- **THEN** 实参与形参的 fn 类型完全一致、调用合法

#### Scenario: 方法值不在批准之列

- **WHEN** 出现 `let f = user.describe`，`describe` 是 `user` 类型的方法
- **THEN** 访问依第 10 章成员名解析但不合任何已批准的值产生式；编译器依第 2 章的 unexpected-token 诊断（`E0105`）拒绝；`|u| u.describe()` 携带接收者

### Requirement: 完整闭包形式

完整闭包形式是 `fn(params) block` 或 `fn(params) -> type block`——匿名的、处于表达式位置。参数表完全是第 6 章的：零或多个 `name: type` 对，每个参数都标注、无接收者例外。返回类型可省，其省略声明闭包不产出值、类型化为 `fn(...) -> ()`。函数体是第 2 章的 block。该形式自含类型：参数与返回类型 MUST 写出、绝不推断，且在任何表达式位置合法。它与第 6 章的声明只隔一字前瞻：表达式位置上 `fn` 后随 `(` 是闭包；顶层 `fn` 后随名字是声明——不同位置的不同产生式。

#### Scenario: 完整闭包连同类型绑定

- **WHEN** 出现 `let add = fn(x: Int64, y: Int64) -> Int64 { x + y }`
- **THEN** `add` 的类型为 `fn(Int64, Int64) -> Int64`；block 值即返回值

#### Scenario: 无值完整闭包

- **WHEN** 出现 `fn(s: String) { put(s) }`
- **THEN** 它是 `fn(String) -> ()` 类型的闭包，其返回类型的省略声明无值

#### Scenario: 完整闭包内联传递

- **WHEN** 出现 `apply(fn(x: Int64) -> Int64 { x * 2 }, 5)`，`apply` 如上
- **THEN** 闭包与形参的 fn 类型一致、调用合法

#### Scenario: 闭包不是声明

- **WHEN** `fn(x: Int64) -> Int64 { x }` 作为表达式项出现，而 `fn triple(x: Int64) -> Int64 { x * 3 }` 出现在顶层
- **THEN** 前者是闭包——`fn` 遇到 `(`；后者是第 6 章的声明——`fn` 遇到名字；一个 token 区分两个产生式

### Requirement: 短闭包形式

短闭包形式是 `|p1, ..., pk| body`，k 从 1 起：竖线定界的逗号分隔参数表，随后是一个表达式或一个 `{ ... }` block 的体；体的值是闭包的返回值，其中的 `return` 与 `defer` 遵循"闭包体即函数体"。零参数闭包只有完整形式——`||` 是第 1 章的逻辑或 token，maximal munch 下为单 token，故不存在可写的零参短形式。参数是裸 `name` 或标注 `name: type`。全标注形式自含类型、在任何表达式位置合法。裸参数——任何一个——使闭包依赖其位置上的显式 fn 类型期望：绑定注解、形参声明的 fn 类型、或返回位的已声明 fn 类型。在那里，每个裸参数自期望取得对应类型，返回类型来自体，每个标注参数 MUST 与期望一致（否则 `E0501`）。不存在期望 fn 类型之处，裸参闭包以 `E1001` 拒绝。短形式与完整形式一同坐落于 primary/postfix/unary 骨架之外，其体最大化：体首表达式之后的后缀与二元运算符属于体，故闭包自身不受后缀——调用须加括号 `(|x| x + 1)(2)`——且作为二元或一元运算符的直接操作数时 MUST 加括号：操作数位未加括号的 `|` 不合任何产生式、依第 2 章的 unexpected-token 诊断（`E0105`）拒绝。语句首位的 `|` 是第 2 章的 `E0102`——语句起始类不容纳它——且丢弃闭包值在任何情况下都是 `E0605`。

#### Scenario: 标注短闭包独立成立

- **WHEN** 出现 `let inc = |x: Int64| x + 1`
- **THEN** 闭包自含类型、类型为 `fn(Int64) -> Int64`；无需任何期望

#### Scenario: 裸短闭包从注解取得类型

- **WHEN** 出现 `let f: fn(Int64) -> Int64 = |x| x + 1`
- **THEN** `x` 自注解取得 `Int64`，返回来自体，绑定一致

#### Scenario: 裸短闭包作实参

- **WHEN** 出现 `apply(|x| x + 1, 3)`，`apply` 的形参为 fn 类型
- **THEN** 闭包的参数取得形参声明的 fn 类型；调用合法

#### Scenario: 无期望的裸短闭包被拒绝

- **WHEN** 出现无注解的 `let f = |x| x + 1`
- **THEN** 编译器以 `E1001:` bare-parameter closure without an expected function type 拒绝；标注参数或绑定

#### Scenario: 操作数位的短闭包须括号

- **WHEN** 出现 `1 + |x| x`
- **THEN** 编译器在操作数依第 2 章的 unexpected-token 诊断（`E0105`）拒绝；`1 + (|x| x)` 是合法分组

#### Scenario: 语句首位短闭包被拒绝

- **WHEN** 语句以 `|x| x + 1` 开始
- **THEN** 编译器以 `E0102:` statement begins with a continuation token 拒绝；`|` 不开启语句

#### Scenario: 零参短形式不存在

- **WHEN** 出现 `let z = || ready()`——或空格变体 `| |`——而要的是零参数闭包
- **THEN** 编译器依第 2 章的 unexpected-token 诊断（`E0105`）拒绝：`||` 是逻辑或 token、参数表以名字开列，零参闭包只有完整形式 `fn() -> ...`

#### Scenario: 调用短闭包须括号

- **WHEN** 出现 `(|x| x + 1)(2)`
- **THEN** 括号固定接收者；值为 `3`，无括号则调用后缀属于体

### Requirement: 闭包体即函数体

闭包的体 block 是函数体：其末表达式依第 2 章的 block 值规则是其返回值；`return` 从闭包返回、永不从外围具名函数返回——控制流目标总是局部可判定的、绝不咨询外围调用链；体内嵌套 block 的提前 `return` 依第 6 章模型合法；闭包体内的 `defer` 于闭包体退出时依第 3 章执行。`E0401` 的函数体语境是第 6 章的具名 fn 与本章的闭包；模块顶层与 defer 体仍不可 return。闭包自身是与其他无异的值——可绑定、可传递、可从其外围函数返回、可随后调用，其捕获保持创建时的触达（"捕获纪律"）。

#### Scenario: return 只退出闭包

- **WHEN** 某 fn 持有 `let get = fn(m: Option<Int64>) -> Int64 { return match m { Some(n) => n None => 0 } }` 并调用 `get(Some(3))`
- **THEN** `return` 携带闭包的值；外围 fn 的控制流未被触及，其自身的 `return` 仍须携带其自身的值

#### Scenario: defer 于闭包体退出时执行

- **WHEN** 闭包体在末表达式前持有 `defer { done() }`
- **THEN** `done()` 在闭包体退出时——每次调用——依第 3 章的 LIFO 规则执行

#### Scenario: 闭包逃出外围 fn

- **WHEN** 某 fn 返回一个捕获 gc 类别绑定的闭包，调用方在该 fn 返回后调用该闭包
- **THEN** 闭包可调用、依"捕获纪律"看到其捕获；gc 意味着追踪单元格活得比栈帧久

### Requirement: 捕获纪律

闭包一律 gc 类别——一个类别、引用语义：可存储、可传递、可返回、可逃逸；value 类别闭包将需要本规范未批准的逃逸分析。捕获对接第 8 章的所有权类别。gc 类别类型的绑定按引用捕获：体内的读与赋值看到并突变活绑定，被捕获的单元格受追踪、逃逸健全。value 类别类型或基类型的绑定在闭包创建时按拷贝捕获：体读到的是创建时刻的值，外围绑定其后的突变不可见，体内对这类被捕获绑定的赋值以 `E1003` 拒绝——快照是冻结的；中间值由局部绑定携带。resource 类别绑定 MUST NOT 被捕获——`E1002:` resource binding captured by a closure：闭包按构造可越出创建点，句柄外逸破坏资源纪律所依赖的单一确定性释放点；资源的内容先物化、闭包捕获物化后的值。

#### Scenario: gc 绑定被活捕获

- **WHEN** 一个 gc record 类型的 `var` 绑定被捕获且闭包体内对其赋值，随后闭包被调用
- **THEN** 赋值突变活绑定；变化在外部可见、调用后持续

#### Scenario: value 绑定被拷贝捕获

- **WHEN** 一个 `byval` record 绑定被捕获、外围绑定在闭包创建后被重绑，随后闭包被调用
- **THEN** 闭包读到创建时刻的值；重绑对它不可见

#### Scenario: resource 绑定不被捕获

- **WHEN** 闭包体读取外围作用域的 `byres` record 绑定
- **THEN** 编译器以 `E1002:` resource binding captured by a closure 拒绝；先物化内容、捕获物化后的值

#### Scenario: 对被捕获 value 绑定赋值被拒绝

- **WHEN** 一个 `var n: Int64` 绑定被捕获且闭包体内持有 `n = n + 1`
- **THEN** 编译器以 `E1003:` assignment to a captured value-category binding 拒绝；以局部绑定或 gc 单元格携带中间值

#### Scenario: gc var 计数器经捕获工作

- **WHEN** `var c = Counter { n: 0 }`（gc `Counter`）且闭包体跨调用重赋 `c`
- **THEN** 重赋作用于活绑定；每次调用见到上一次调用的结果

### Requirement: 函数类型诊断段位

fn-types 章拥有 `docs/spec/diagnostics.toml` `[segments]` 声明的注册表段位 `E1000`–`E1099`。分配：`E1001` 裸参闭包无期望 fn 类型、`E1002` resource 绑定被闭包捕获、`E1003` 对被捕获 value 类别绑定赋值、`E1004` 泛型函数名用于值位置。`E1000` 与 `E1005`–`E1099` 为本章修订保留。触发语义在本章 Requirements；条目在注册表。

#### Scenario: fn-types 码被发出

- **WHEN** 工具链发出任何 `E10xx` 诊断
- **THEN** 其完整条目可自 `docs/spec/diagnostics.toml` 以 owner `1200-fn-types` 检索

#### Scenario: 后续变更需要本段位的码

- **WHEN** 后续章节批准需要新函数值诊断的规则——组合子变更亦在其列
- **THEN** 其变更于同一变更内在 `E1000`–`E1099` 中扩展注册表，或认领自己的段位

## 示例（非权威）

下面的示例只用第 1–12 章与第 16 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。迭代器组合子、泛型函数实例化与方法值标注为待其各自变更。

### 函数类型与函数值

```we
let f: fn(Int64) -> Int64 = square       // a monomorphic fn name is a
let nine = f(3)                          // value; the call is postfix
let eff: fn(Int64) io -> Int64 = fetch   // effect segment: bare tags,
                                         // chapter 16's ratification

fn apply(f: fn(Int64) -> Int64, x: Int64) -> Int64 {
    f(x)                                 // the parameter's call
}

let make = fn() -> Bool { ready() }      // zero-parameter, valueless
let side = fn(s: String) { put(s) }      // no return type: fn(String) -> ()
```

### 闭包

```we
let inc = |x: Int64| x + 1               // annotated: self-contained
let f: fn(Int64) -> Int64 = |x| x * 2    // bare: types from the annotation
let out = apply(|x| x + 1, 3)            // bare: types from the parameter

let get = fn(m: Option<Int64>) -> Int64 {
    return match m {                     // return exits the closure
        Some(n) => n
        None => 0
    }
}

var c = Counter { n: 0 }                 // gc var: captured live
let bump = fn() { c = Counter { n: c.n + 1 } }  // zero-parameter: the
bump()                                   // full form; there is no || form
```

### 捕获对接所有权类别

```we
let text = "fixed"                       // base type: copy at creation
let show = fn() { put(text) }            // zero-parameter: full form only

var p = Point { x: 1, y: 2 }             // byval: snapshot at creation
let read = fn() -> Int64 { p.x }
p = Point { x: 9, y: 9 }
// read() still sees 1: the capture is the creation-time copy
```

### 拒绝形式

```we
let g = |x| x + 1                        // E1001: bare-parameter closure
                                        // without an expected function type
let h = pairUp                           // E1004: generic function name
                                        // in value position
let z = || ready()                       // E0105: unexpected token; ||
                                        // is logical-or, a zero-parameter
                                        // closure is fn() -> ...
let m = user.describe                    // E0105: unexpected token; a
                                        // method value is not ratified
let bad = fn(Int64)                      // E0105: unexpected token; the
                                        // arrow and return type are
                                        // required
let eff: fn(Int64) io -> Int64 = f       // E0105: unexpected token; no
                                        // effect segment is ratified
let one = 1 + |x| x                      // E0105: unexpected token; write
                                        // 1 + (|x| x)
|x| x + 1                                // E0102: statement begins with a
                                        // continuation token

fn over(log: LogFile) -> fn() -> Int64 { // E1002: resource binding
    fn() -> Int64 { log.lines() }        // captured by a closure
}

var n = 0
let step = fn() { n = n + 1 }            // E1003: assignment to a captured
                                        // value-category binding
let g2 = Circle                          // E0704: payloaded variant
                                        // constructor used without
                                        // arguments
```

### 待后续变更

```we
// Iterator combinators arrive with their owning change as default
// methods of Iterator; generic instantiation `map<Int64>` and generic
// methods with chapter 10's own amendment; method values, if ever,
// with this chapter's amendment:
//
// let out = names.iterator()
//     .filter(|n| n.size() > 2)
//     .map(|n| n.toUpper())
//     .collect()
```

## 术语对照

本章关键术语，英中对译，以保翻译一致：

| English | 中文 |
| --- | --- |
| function type | 函数类型 |
| function value | 函数值 |
| closure | 闭包 |
| full closure form | 完整闭包形式 |
| short closure form | 短闭包形式 |
| bare parameter | 裸参数 |
| annotated parameter | 标注参数 |
| expected function type | 期望函数类型 |
| capture | 捕获 |
| capture discipline | 捕获纪律 |
| copy snapshot | 拷贝快照 |
| live binding | 活绑定 |
| traced cell | 追踪单元格 |
| escape | 逃逸 |
| monomorphic | 单态 |
| generic clause | 泛型子句 |
| method value | 方法值 |
| effect segment | 效应段 |
| block value | 块值 |
