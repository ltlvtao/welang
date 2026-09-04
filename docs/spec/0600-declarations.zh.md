# We 语言规范 —— 第 6 章：声明

### Requirement: 文件结构与模块同一性

源文件即模块。文件顶层是顶层项的序列：import 声明、fn 声明、依第 8 章的 record 与 newtype 声明、依第 9 章的 sum 类型声明、依第 10 章的接口声明与 impl 块、依第 19 章的 foreign 块、顶层 let 绑定。各项次序任意；排序约定是格式化器的事，不是文法的事。语句与表达式 MUST NOT 作为顶层项出现——它们只存在于块内——不适配任何项产生式的顶层 token 序列 MUST 以第 2 章意外 token 诊断（`E0105`）拒绝。模块名是按第 1 章命名约定指名它的点分小写路径；路径依第 15 章的映射解析到文件——源根下的目录嵌套、保留的 `std` 段、依赖缓存。

#### Scenario: 一个文件解析为一个模块

- **WHEN** 源文件持有 import、fn 声明、record、newtype 与 sum 类型声明、接口声明与 impl 块、顶层 let 绑定
- **THEN** 各解析为一个顶层项，文件是一个模块；顶层不接受其他内容

#### Scenario: 顶层语句被拒绝

- **WHEN** 表达式或赋值作为顶层项出现，例如文件顶层的 `count = count + 1` 或 `compute()`
- **THEN** 编译器以 `E0105:` unexpected token 拒绝，具名考虑过的项产生式

#### Scenario: import 只能出现在顶层

- **WHEN** `import std.io` 出现在块内
- **THEN** 编译器以 `E0105:` unexpected token 拒绝；import 只作为顶层项存在

#### Scenario: foreign 块是顶层项

- **WHEN** 源文件在其 import、fn 声明与顶层 let 绑定之间持有 `foreign "c" { ... }`
- **THEN** 该块依第 19 章解析为一个顶层项；任何块内的 foreign 块在那里被拒绝（`E1702`），其声明的条目加入模块的单一名字空间

### Requirement: import 声明

import 声明是 `import path` 或 `import path as name`。path MUST 是点分小写模块路径，别名（若有）MUST 是小写标识符——同受第 1 章模块命名约定（声明处 `E0013`）。一条 import 恰引入一个名字进模块名空间：路径末段，或别名（若有）。经该名字，导入模块依"pub 可见性"到达目标模块的公开项；路径依第 15 章的映射解析，跨模块可见性由第 15 章执法（`E1303`）。选择性导入（`import a.{b, c}`）、通缀导入、以多于一个名字导入均不存在。

#### Scenario: 无别名的 import 引入末段

- **WHEN** `import models.user` 作为顶层项出现
- **THEN** 名字 `user` 指代被导入模块；其公开项以 `user.item` 到达

#### Scenario: 带别名的 import

- **WHEN** `import std.io as io` 作为顶层项出现
- **THEN** 名字 `io` 指代被导入模块；路径末段不再自行引入名字

#### Scenario: import 路径或别名违反模块命名

- **WHEN** import 路径或别名非小写点分，例如 `import Models.User` 或 `import std.io as IO`
- **THEN** 编译器在声明处以 `E0013:` module names must be lowercase 拒绝

### Requirement: fn 声明

fn 声明是 `fn name(params) block` 或 `fn name(params) -> type block`，可选前缀 `pub`。名字是第 1 章命名约定下的标识符（`E0012`）。参数列表是零或多个以逗号分隔的 `name: type` 对；每个参数 MUST 携带类型注解——裸参数名不适配任何产生式，MUST 以第 2 章意外 token 诊断（`E0105`）拒绝，消息具名 `name: type` 产生式。`->` 后声明的返回类型表示函数产出值；其缺失表示不产出。填充注解槽的类型文法由类型章节批准。泛型参数由第 10 章批准：`fn name<T1, ..., Tk>(params)` 把子句携带于名字与参数列表之间，where 子句可尾随签名于体之前；唯一的裸参数例外是第 10 章的方法接收者——impl 块内第一个参数是 `self` 或 `mut self`，裸写，其类型由 impl 头固定。效果段由第 16 章批准：fn 声明可在参数列表与箭头或体之间携带 `effect tag1 tag2 ...`，该段参与的检查归该章。foreign 声明由第 19 章批准：foreign 块内的 fn 声明是无体的本签名形，且效果段在那里必写。`mut` 参数仍悬于其归属章节。

#### Scenario: 完整签名的函数

- **WHEN** `pub fn add(a: Int64, b: Int64) -> Int64 { a + b }` 作为顶层项出现
- **THEN** 解析为一个 fn 声明：两个带注解参数、声明的返回类型、第 2 章块体

#### Scenario: 无注解参数被拒绝

- **WHEN** 出现 `fn f(x) { }`
- **THEN** 编译器在参数处以 `E0105:` unexpected token 拒绝，具名 `name: type` 产生式；参数类型永不推断

#### Scenario: 无返回类型的函数不产出值

- **WHEN** 出现 `fn log(m: String) { emit(m) }`
- **THEN** 解析为不产出值的 fn 声明；返回类型省略即表明此意，无需 `->` 子句

#### Scenario: 泛型函数解析

- **WHEN** `fn identity<T>(x: T) -> T { x }` 作为顶层项出现
- **THEN** 它解析为按第 10 章携带单参数泛型子句的一个 fn 声明；子句位于名字与参数列表之间，其余循本章形式

#### Scenario: 带效果段的函数解析

- **WHEN** `fn write(msg: String) effect io { save(msg) }` 作为顶层项出现
- **THEN** 它解析为在参数列表与体之间携带效果段的一个 fn 声明，依第 16 章；省略时声明即纯函数

### Requirement: 函数体、return 与 defer

fn 的体块是函数语境。其内——包括嵌套于其中的块——第 3 章的 return 形式合法：声明了返回类型时，`return expr` 携该值提前退出，裸 `return` 在产出值路径上的合法性由类型章节批准；未声明时，裸 `return` 提前退出，`return expr` MUST 以 `E0402` 拒绝。任何函数体之外的 return——顶层，或 defer 体内（它在函数退出时执行，彼时无存活控制流可言）——MUST 以 `E0401` 拒绝。声明了返回类型时，体块值是函数的隐式返回值：末项表达式返回它，提前 return 携带它；每条路径是否产出值归类型章节。未声明返回类型时，体末项由第 8 章值丢弃规则治理：类型非 unit 的末项表达式 MUST 以 `let _ =` 显式丢弃，unit 类型无需仪式。fn 体的直接项正是第 3 章 defer 合法的位置，兑现该条 Requirement 的可拒绝侧。

#### Scenario: 末项表达式即隐式返回值

- **WHEN** 函数声明 `-> Int64` 且体末项是 `a + b`
- **THEN** 函数返回该值；在同一位置写 `return a + b` 等价

#### Scenario: 嵌套块内的提前 return

- **WHEN** 产出值函数的体内出现 `if done { return acc }`
- **THEN** return 合法、退出函数，且任何 defer 依第 3 章 Defer 逆序执行

#### Scenario: 无值函数中带值 return

- **WHEN** 未声明返回类型的函数含 `return 1`
- **THEN** 编译器以 `E0402:` return with a value in a function that declares none 拒绝

#### Scenario: 函数体之外的 return

- **WHEN** `return` 出现在顶层，或 defer 体内
- **THEN** 编译器以 `E0401:` return outside a function body 拒绝

#### Scenario: defer 的既定位置

- **WHEN** fn 体的直接项是 `defer { release() }`
- **THEN** 依第 3 章 Defer 合法并在函数退出时逆序执行；别处的同一 defer 继续触发 `E0204`

#### Scenario: 无值函数的非 unit 末项表达式被拒绝

- **WHEN** 未声明返回类型的函数以 `step()` 结尾，其返回 `Int64`
- **THEN** 编译器依第 8 章以 `E0605:` non-unit value dropped 拒绝；修复是 `let _ = step()` 或声明返回类型

### Requirement: 顶层绑定

顶层绑定是 `let name = expr` 或 `pub let name = expr`，各自可在 `=` 前带类型注解 `name: type`，依第 2 章绑定形式。`var` MUST NOT 出现在顶层：顶层 `var` 绑定 MUST 以 `E0403` 拒绝——模块级可变状态不存在，`var` 仍只在块内合法。一个模块顶层绑定的初始化子在模块初始化时按源序求值；跨模块初始化模型——每模块恰一次、import 图后序、先于 `main`——归第 15 章。

#### Scenario: 顶层 let 绑定

- **WHEN** `pub let maxRetries = 3` 作为顶层项出现
- **THEN** 解析为顶层绑定，可见性依"pub 可见性"

#### Scenario: 顶层 var 被拒绝

- **WHEN** `var count = 0` 作为顶层项出现
- **THEN** 编译器以 `E0403:` var at the top level 拒绝；块内同一形式不受影响

#### Scenario: 初始化子按源序求值

- **WHEN** 模块声明两个顶层绑定，初始化子先后为 `first()` 与 `second()`
- **THEN** 该模块内模块初始化时 `first()` 先于 `second()` 求值

### Requirement: pub 可见性

`pub` 是已批准声明类别的前缀——本章的 fn 声明与顶层 let 绑定、第 8 章的 record 与 newtype 声明、第 9 章的 sum 类型声明、第 10 章的接口声明与 impl 方法定义。pub 项在其模块之外可见；无 `pub` 的项模块内局部。他模块的项只有 pub 才可经 import 到达；使用他模块非 pub 项的诊断归第 15 章（`E1303`），main 函数约定归第 15 章。其他任何位置的 `pub`——import 前、块内、任何其他 token 序列之前——不适配产生式，MUST 以第 2 章意外 token 诊断（`E0105`）拒绝。

#### Scenario: pub 标记项跨模块可见

- **WHEN** 模块 A import 模块 B 且 B 声明 `pub fn f()`
- **THEN** `f` 经被导入模块名从 A 可达；同一 fn 无 `pub` 则模块内局部，其跨模块使用以第 15 章的 `E1303:` cross-module use of a module-local item 拒绝

#### Scenario: pub 在其已批准项之外

- **WHEN** `pub` 前缀 import 或出现在块内，例如 `pub import std.io` 或 `let x = { pub fn f() { } }`
- **THEN** 编译器以 `E0105:` unexpected token 拒绝

### Requirement: 模块名字空间

一个模块一个名字空间。fn 名、顶层 let 名、import 声明引入的名字共享之；同名第二个声明 MUST 以 `E0404` 拒绝。块内局部名不属于此空间；其与顶层名的遮蔽关系由类型章节批准。

#### Scenario: 一个名字两处声明

- **WHEN** 一个模块两次声明 `fn f`，或 `fn f` 与 `let f` 并存
- **THEN** 编译器以 `E0404:` duplicate name in one module 拒绝第二个

#### Scenario: import 名与声明冲突

- **WHEN** 一个模块同时含 `import std.io as io` 与 `fn io()`
- **THEN** 编译器以 `E0404:` duplicate name in one module 拒绝后者

### Requirement: 文档注释

连续 `///` 行构成一个文档单元，依第 1 章词法形式。文档单元附着下一个顶层项；单元与该项之间的空行与普通注释不断链。文件内再无后续顶层项的文档单元 MUST 以 `E0405` 拒绝。文档内容何意、工具链如何渲染归工具链章节；本章只批准附着。

#### Scenario: 文档单元附着下一项

- **WHEN** 两行 `///` 位于一个 fn 声明之前
- **THEN** 它们构成附着该 fn 的一个文档单元；其后的下一项不从中携带附着

#### Scenario: 孤儿文档注释

- **WHEN** 一段 `///` 在文件结束前再无后续顶层项
- **THEN** 编译器以 `E0405:` orphan documentation comment 拒绝

## 示例（非权威）

下面的示例只用第 1–6 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。类型层规则标注为待类型章节。

### 一个模块的声明

```we
import std.io as io
import models.user

/// Retry budget shared across this module's handlers.
pub let maxRetries = 3

pub fn handle(id: Int64) -> Bool {
    let u = user.find(id)
    retry(u, maxRetries)
}

fn retry(u: user.User, budget: Int64) -> Bool {   // module-local: no pub
    io.println("retrying")
    true
}
```

### return、defer 与体值

```we
pub fn loadAll(ids: List<Int64>) -> Int64 {   // final expression is the
    var total = 0                             // implicit return value
    for id in ids {
        total = total + process(id)
    }
    total
}

pub fn process(id: Int64) -> Int64 {
    defer { io.println("done") }   // legal: a direct item of the fn body
    if id < 0 {
        return 0                   // early return; defer runs at exit
    }
    transform(id)
}

fn emit(line: String) {            // no return type: produces no value
    io.println(line)               // final expression's value is
}                                  // discarded implicitly (types chapter)
```

### 被拒绝的形式

```we
var count = 0                      // E0403: var at the top level
return 1                           // E0401: return outside a function body
fn bad(x) { }                      // E0105: parameter annotation required
fn noValue() { return 1 }          // E0402: value return in a valueless fn
pub import std.io                  // E0105: pub prefixes fn and let only
import Models.User                 // E0013: module names must be lowercase
fn io() { }                        // E0404: duplicate name in one module
                                  // (io already introduced by import)

fn f() {
    import std.io                  // E0105: import is top-level only
    return
}
```

### 待后续章节

```we
// Module resolution, the cross-module visibility diagnostic, and
// the main convention landed with chapter 15; generic parameters
// landed with the interfaces chapter; effect segments landed with
// chapter 16; foreign declarations landed with chapter 19. mut
// parameters are their owning chapter's:
//
// fn read(path: String) effect io -> String
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| declaration | 声明 |
| top-level item | 顶层项 |
| module | 模块 |
| import declaration | import 声明 |
| alias | 别名 |
| fn declaration | fn 声明 |
| parameter | 参数 |
| return type | 返回类型 |
| function context | 函数语境 |
| top-level binding | 顶层绑定 |
| module-local | 模块内局部 |
| name space | 名字空间 |
| documentation unit | 文档单元 |
| orphan | 孤儿 |
