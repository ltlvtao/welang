# We 语言规范 —— 第 15 章：模块


### Requirement: 模块路径与文件解析

模块路径是第 1 章模块命名约定下的点分小写模块路径。本地模块路径相对项目源根解析：末段之外每段指名一个目录，末段指名一个 `.we` 文件，均相对项目根的 `src/` 目录——`import models.user` 解析到 `src/models/user.we`。项目根是持有项目清单的目录；清单的格式是工具链的事，不是本章的事。路径首段 `std` 为保留：以 `std.` 起头的模块路径总是指名编译器内建的标准库模块，绝不相对文件系统解析，且本地文件或依赖模块不能占据 `std` 段。本地路径优先于缓存：在项目 `src/` 下解析得到的点分路径指名该本地模块；其余路径自依赖缓存按同一目录与文件映射解析——获取、版本约束与锁文件是工具层的业务，不是本章的。目标无法解析的 import MUST 以 `E1302:` module not found 拒绝，报文按上述映射指名期望的文件路径。import 构成有向图：循环依赖——直接或传递——MUST 以 `E1301:` circular module dependency 拒绝——没有前向声明，修复是把共享代码抽进两者都 import 的第三模块。

#### Scenario: 本地路径映射到文件

- **WHEN** 出现 `import models.user` 且 `src/models/user.we` 存在
- **THEN** 路径按映射解析到该文件；模块名是 `models.user`

#### Scenario: std 路径绝不触碰文件系统

- **WHEN** 出现 `import std.io`，无论 `src/std/` 下有何文件
- **THEN** 路径指名编译器内建标准库模块 `std.io`；不咨询任何文件，且 `std` 段不能被本地占据

#### Scenario: 找不到模块以期望路径拒绝

- **WHEN** 出现 `import models.missing` 且 `src/models/missing.we` 不存在，也无依赖缓存条目应答该路径
- **THEN** 编译器以 `E1302:` module not found 拒绝，报文指名期望路径 `src/models/missing.we`

#### Scenario: 依赖路径自缓存解析

- **WHEN** 出现 `import external.package.name` 且依赖缓存持有该模块
- **THEN** 路径自缓存按同一目录与文件映射解析；同时能在项目 `src/` 下解析的路径确定地指名本地模块

#### Scenario: 循环依赖被拒绝

- **WHEN** 模块 `a` import `b` 且模块 `b` import `a`，直接或经更多模块
- **THEN** 编译器以 `E1301:` circular module dependency 拒绝；没有前向声明，修复是两者都 import 的第三模块

### Requirement: 预导入
预导入是每个模块免 import 可见的固定标准作用域名字集：第 7 章的基础类型名、第 9 章的 `Never`、第 10 章的 `Dyn`、结果类型名 `Result`、`Ok` 与 `Err`、选项类型名 `Option`、`Some` 与 `None`、集合章节的集合类型名 `List`、`Map` 与 `Set`、第 14 章的终止函数名 `panic`、`todo` 与 `assert`、并发章的编译器附加标记 `Shareable`，以及并发章的取消访问器 `currentCancelSignal`。该集封闭：向其增名是对本章的 spec 变更，不是标准库发版。模块自身的声明遮蔽预导入名——预导入是外层作用域，显式声明胜出，即第 0 章 Principle 5；遮蔽预导入名合法且不触发诊断，因为 `E0404` 只管一个模块自有名字之间的重复。标准库其余的一切是 `std.` 下的普通模块，只能经 import 到达。

#### Scenario: 预导入名无需 import

- **WHEN** 一个无 import 的模块使用 `String`、`List`、`Result`、`Ok`、`Err` 与 `panic`，且其中 task 块体调用 `currentCancelSignal()`
- **THEN** 每个名字解析到其标准作用域含义；不要求任何 import

#### Scenario: 本地声明遮蔽预导入名

- **WHEN** 一个模块自行声明 `fn panic(msg: String) -> Never`
- **THEN** 该模块的 `panic` 在其内遮蔽预导入者，无碰撞诊断；`E0404` 仍只管模块自有名之间的重复

#### Scenario: 取消访问器的合法性归并发章

- **WHEN** `currentCancelSignal()` 在 task 块体内被调用，又在任何 task 块之外的普通函数体内
- **THEN** 前者解析到预导入的访问器；后者是并发章的 `E1608`——预导入携带该名字，该章固定它在何处合法

#### Scenario: 标准库其余部分须 import

- **WHEN** 一个模块不经 `import std.io` 使用 `std.io` 的名字
- **THEN** 该使用以 `E1304` 未解析名拒绝；预导入之外的标准库模块只能经 import 到达

### Requirement: 跨模块可见性

带 `pub` 的项构成模块的公共面：第 6 章的 fn 声明与顶层 let 绑定、第 8 章的 record 与 newtype 声明、第 9 章的 sum 类型声明、第 10 章的接口声明、第 10 章的 impl 方法定义。模块持有的其余一切皆模块局部。跨模块到达恰是经 import 名的合格形式 `name.item`；使用他模块的模块局部项——值位、类型位、或作变体构造器——MUST 以 `E1303:` cross-module use of a module-local item 拒绝。接口成员依第 10 章不带 `pub`：其可达性是接口的，故经接口值——`Dyn<Interface>` 箱或泛型约束——的方法调用合法，即使 impl 方法定义模块局部；`E1303` 只管直接名字到达。

#### Scenario: pub fn 跨模块可达

- **WHEN** 模块 B 声明 `pub fn create()` 且模块 A import B
- **THEN** `b.create()` 解析；经 import 名的合格形式即到达

#### Scenario: 模块局部 fn 被跨模块拒绝

- **WHEN** 模块 B 声明 `fn helper()` 无 `pub` 且模块 A 调用 `b.helper()`
- **THEN** 编译器以 `E1303:` cross-module use of a module-local item 拒绝

#### Scenario: 模块局部类型在类型位被拒绝

- **WHEN** 模块 B 声明 `record Config` 无 `pub` 且模块 A 写注解 `b.Config`
- **THEN** 编译器以 `E1303:` cross-module use of a module-local item 拒绝；类型位执法同一边界

#### Scenario: 模块局部变体构造器被拒绝

- **WHEN** 模块 B 声明 `type AppError = NotFound(String)` 无 `pub` 且模块 A 写 `b.NotFound("x")`
- **THEN** 编译器以 `E1303:` cross-module use of a module-local item 拒绝；构造器位与值位、类型位执法同一边界

#### Scenario: 模块局部 impl 方法经接口到达

- **WHEN** 模块 B 声明 pub 接口、方法定义模块局部的 impl、产出实现值的 pub 构造器，模块 A 把该值装箱为 `Dyn<Describable>` 并调用方法
- **THEN** 调用合法：接口的可达性作准，`E1303` 只管直接名字到达

### Requirement: 名字解析

非限定名自内向外经作用域解析：第 8 章作用域规则下的块局部绑定与参数，然后第 6 章模块自身的名字空间——其声明与 import 名同域——然后预导入。持有该名字的最内层作用域胜出。无任何作用域持有的名字 MUST 以 `E1304:` unresolved name 拒绝。合格形式 `name.item` 在 `name` 是 import 名且 `item` 是被导入模块的 pub 项时解析；非 import 名的限定符与被导入模块未声明的项以 `E1304:` unresolved name 拒绝——已声明而非 pub 者归 `E1303`，不归此码。同一规则填充类型位：良构类型名——裸名、模块限定、或预导入名——是否解析在此决定，落定第 7 章悬句；本地声明对预导入类型名的遮蔽、`Dyn` 在内，与对任何预导入名的遮蔽一样。

#### Scenario: 未知裸名被拒绝

- **WHEN** 出现 `compute()` 且任何作用域中的声明、import 或预导入名都不持有 `compute`
- **THEN** 编译器以 `E1304:` unresolved name 拒绝

#### Scenario: 缺失限定符或项被拒绝

- **WHEN** `notauser.item` 出现而 `notauser` 不是 import 名，或 `user.missing()` 出现而 `user` 是 import 名且该模块未声明 `missing`
- **THEN** 编译器各以 `E1304:` unresolved name 拒绝——限定符不指名模块，或模块未声明该项

#### Scenario: 未解析类型名被拒绝

- **WHEN** 注解持有 `Config` 且无声明、被导入 pub 项或预导入名提供类型 `Config`
- **THEN** 编译器以 `E1304:` unresolved name 拒绝；类型名解析在此决定，落定第 7 章悬句

#### Scenario: 被遮蔽的预导入类型名依本地含义

- **WHEN** 一个模块自行声明 `type Result = Win | Lose` 并注解 `let r: Result`
- **THEN** 注解指名模块自身的 sum；预导入的 `Result` 在该模块内不可经裸名到达，且无诊断触发——遮蔽预导入类型名合法

### Requirement: 模块初始化

每个模块的顶层 let 初始化子恰运行一次，于进程启动、`main` 运行之前——急切初始化，无惰性模块加载。次序是 import 图的后序：一个模块只在其 import 的每个模块全部初始化之后初始化，后序遍历自根模块的 import 按源序展开；一个模块内，初始化子依第 6 章按源序运行。循环依赖到不了初始化——`E1301` 在编译期拒绝它们——故次序是全序且确定的，即第 0 章 Principle 1。被两个模块导入的模块初始化一次，不是两次。初始化子可调用其 import 模块的函数：它们先于它初始化。初始化期间的 panic 依第 14 章展开后中止进程——`main` 之前无捕获边界。

#### Scenario: 被导入模块先于导入者初始化

- **WHEN** 根模块 import `a`、`a` import `b`，每个模块的顶层初始化子记录自己的模块名
- **THEN** 记录次序是 `b`、然后 `a`、然后根模块自身——import 图后序

#### Scenario: 双重导入的模块初始化一次

- **WHEN** 根模块同时 import `a` 与 `b`，两者都 import 带顶层初始化子的 `c`
- **THEN** `c` 的初始化子恰运行一次，先于 `a` 与 `b` 两者

#### Scenario: 初始化期间的 panic 中止

- **WHEN** 一个顶层初始化子调用 `panic`
- **THEN** 进程依第 14 章展开后携带 panic 消息中止，先于 `main` 运行——`main` 之前不存在捕获边界

### Requirement: main 约定

程序的入口模块是根模块：源根处的 `main` 模块，即文件 `src/main.we`。根模块 MUST 声明恰一个 `pub fn main() -> Result<(), E>`，`E` 为第 14 章约束下的命名 sum 类型——v0.8 草案的 `Error` 是 `E` 的一个合法拼写，不是必拼者。未声明 `main`、声明不带 `pub` 的 `main`、或签名非该形状的根模块 MUST 以 `E1305:` main function signature violation 拒绝。`main` MAY 按第 16 章携带效果段——`pub fn main() effect io -> Result<(), AppError>` 合规：进程启动工作恰是 `main` 的用途，段随声明走，`E1305` 检查的形状是无参参数表、`Result<(), E>` 返回与 `pub`——效果段不参与该形状。非根模块中的 `main` 是不承载约定的普通 fn 名。`main` 在每个模块初始化之后运行。返回 `Ok(())` 以退出码 0 退出；返回 `Err(e)` 在 stderr 报告失败消息后以非零退出——消息格式是标准库的事，不是本章的。`main` 无参数：可执行文件的命令行可达性归标准库，不归语言。

#### Scenario: 合规的根模块

- **WHEN** `src/main.we` 声明 `pub fn main() -> Result<(), AppError>` 并返回 `Ok(())`
- **THEN** 程序通过编译，每个模块初始化，`main` 运行，进程以退出码 0 退出

#### Scenario: main 无 pub 被拒绝

- **WHEN** `src/main.we` 声明 `fn main() -> Result<(), AppError>` 而无 `pub`
- **THEN** 编译器以 `E1305:` main function signature violation 拒绝

#### Scenario: 带效果段的 main 合规

- **WHEN** `src/main.we` 声明 `pub fn main() effect io -> Result<(), AppError>`，其体按第 16 章执行 io
- **THEN** 程序如约定般编译运行；效果段随声明走，不参与 `E1305` 检查的形状

#### Scenario: 缺失 main 被拒绝

- **WHEN** `src/main.we` 根本不声明 fn `main`
- **THEN** 编译器以 `E1305:` main function signature violation 拒绝

#### Scenario: 非 Result 的 main 形状被拒绝

- **WHEN** `src/main.we` 声明 `pub fn main() -> Int64`
- **THEN** 编译器以 `E1305:` main function signature violation 拒绝；形状是 `Result<(), E>`，`E` 为命名 sum 类型

#### Scenario: main 的非 sum 错误位归第 14 章的码

- **WHEN** `src/main.we` 声明 `pub fn main() -> Result<(), String>`
- **THEN** 编译器以 `E1204:` Result error type is not a named sum type 拒绝——main 形状自身合规，错误位的命名 sum 约束归第 14 章，`E1305` 不触发

#### Scenario: Err 退出码非零

- **WHEN** `main` 返回 `Err(e)`
- **THEN** 进程在 stderr 报告失败消息并以非零退出；唯 `Ok(())` 退出 0

#### Scenario: 非根模块中的 main 是普通 fn

- **WHEN** 一个被导入模块声明 `fn main() -> Int64`
- **THEN** 它是不承载约定的普通 fn 名；唯根模块的 `main` 是入口

### Requirement: 模块诊断段位

模块章拥有注册表段位 `E1300`–`E1399`，在 `docs/spec/diagnostics.toml` `[segments]` 声明，owner 为 `1500-modules`。分配：`E1301` circular module dependency、`E1302` module not found、`E1303` cross-module use of a module-local item、`E1304` unresolved name、`E1305` main function signature violation。`E1300` 与 `E1306`–`E1399` 留作本章修订；需要段位码的后来章节认领自己的段位。

#### Scenario: 段位可检索

- **WHEN** 诊断消费者在注册表查任一 `E13xx` 码
- **THEN** 段位条目指名 owner `1500-modules`，每个已分配码的条目携带 severity、title、description、remediation、owner、requirement 与 allocated 日期

#### Scenario: 后续变更在段内扩展

- **WHEN** 本章后来的 spec 层变更需要新码
- **THEN** 它在自己的变更内从 `E1300`–`E1399` 分配；模块之外的章节认领别的段位

## 示例（非权威）

下面的示例只用第 1–15 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。

### 路径与解析

```we
import models.user                    // resolves to src/models/user.we
import std.io                         // compiler-provided; src/std/ is never
                                      // consulted, the std segment reserved
import external.package.name          // dependency cache, same mapping

import models.missing
// E1302: module not found — expected src/models/missing.we

// a.we: import b    b.we: import a
// E1301: circular module dependency — extract shared code into a third module
```

### 预导入与遮蔽

```we
fn run(s: String) -> Result<Int64, AppError> {
    let n = parse(s)?                // String, Result, Ok: prelude, no import
    if n < 0 { panic("negative") }   // panic: prelude, chapter 14's
    return Ok(n)
}

fn assert(cond: Bool) { }            // legal: the module's own assert
                                     // shadows the prelude's within it

fn write(s: String) {
    io.print(s)
    // E1304: unresolved name — io is not an import name; import std.io
}
```

### 跨模块可见性

```we
// b.we
pub fn create() -> User { ... }
fn helper() { ... }
record Config { ... }

// a.we
import b

let u = b.create()                   // pub: reached through the import name
let h = b.helper()
// E1303: cross-module use of a module-local item — helper is not pub

let cfg: b.Config = make()
// E1303: cross-module use of a module-local item — type positions too
```

### 名字解析

```we
let x = compute()
// E1304: unresolved name — no declaration, import, or prelude name holds it

import models.user
let y = models.user.create()
// E1304: unresolved name — the qualifier is the import name, user

fn size(c: Config) -> Int64 { ... }
// E1304: unresolved name — no type Config in any scope
```

### 初始化与 main

```we
// src/models/config.we
pub let retries = { log("config init"); 3 }   // runs once, before main

// src/main.we
import models.config

pub fn main() -> Result<(), AppError> {
    return Ok(())                   // config initialized first; exit 0
}

pub fn main() -> Int64 { 0 }
// E1305: main function signature violation — the shape is Result<(), E>
```

## 术语对照

本章关键术语，英中对译，以保翻译一致：

| English | 中文 |
| --- | --- |
| module path | 模块路径 |
| file resolution | 文件解析 |
| project root | 项目根 |
| source root | 源根 |
| dependency cache | 依赖缓存 |
| circular dependency | 循环依赖 |
| prelude | 预导入 |
| shadowing | 遮蔽 |
| public surface | 公共面 |
| module-local | 模块局部 |
| name resolution | 名字解析 |
| qualified form | 合格形式 |
| module initialization | 模块初始化 |
| eager initialization | 急切初始化 |
| post-order | 后序 |
| root module | 根模块 |
| entry module | 入口模块 |
| exit code | 退出码 |
