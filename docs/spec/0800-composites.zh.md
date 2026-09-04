# We 语言规范 —— 第 8 章：复合类型与所有权

### Requirement: 所有权类别

每个复合类型恰属四个所有权类别之一，类别决定该类型的传递、共享与生命周期规则：`gc`（引用传递，生命周期由回收器管理）、`value`（赋值与传递时复制）、`resource`（引用传递，显式生命周期管理）、`newtype`（零开销编译期包装，运行时布局被擦除）。record 以其前缀声明类别：`record` 默认为 `gc`，`byval record` 为 `value`，`byres record` 为 `resource`；`newtype` 声明即第四类别。不存在其他类别，且复合类型在声明之后 MUST NOT 更改类别。

#### Scenario: 前缀指名类别

- **WHEN** 声明 `record User { ... }`、`byval record Point { ... }`、`byres record FileHandle { ... }` 与 `newtype UserId(Int64)` 出现
- **THEN** 其类别分别是 gc、value、resource 与 newtype

#### Scenario: 类别集合封闭

- **WHEN** 后续章节需要第五种传递纪律
- **THEN** 它只能通过修订本 Requirement 的规范层变更进入；别无他路

### Requirement: record 声明

record 声明是顶层项 `record Name { fields }`，可选地带 `byval` 或 `byres` 前缀与 `pub` 前缀，并可选按第 10 章在名字后携带泛型参数子句、在花括号组后携带 derives 子句；字段为零个或多个以逗号分隔的 `name: type` 对，每个类型是第 7 章之下（经本章与第 10 章修订）的类型引用；字段名遵循第 1 章变量规则——camelCase（`E0012`）；record 名为 PascalCase（`E0011`）。零字段的 record 合法。泛型字段的类别诚实与派生要求按第 10 章在每次实例化处检查。record 与 newtype 名加入第 6 章模块的唯一名字空间，MUST NOT 与任何其他名字冲突（`E0404`）。

#### Scenario: record 作为顶层项解析

- **WHEN** `record User { id: UserId, name: String }` 出现在顶层
- **THEN** 它声明 gc record `User`，具字段 `id: UserId` 与 `name: String`

#### Scenario: 空 record 合法

- **WHEN** `byval record Empty { }` 出现
- **THEN** 它声明无字段的 value 类别 record；value 类别的空 record 不携带数据

#### Scenario: record 名在模块名字空间冲突

- **WHEN** 模块声明 `record User { ... }` 与 `fn User() { ... }`
- **THEN** 编译器以 `E0404:` duplicate name in one module 拒绝第二个声明

#### Scenario: 带 derives 子句的泛型 record 解析

- **WHEN** `record Box<T> { value: T } derives Eq` 出现在顶层
- **THEN** 它按第 10 章声明单参数 gc record，其 `.equals` 对 `T` 的要求在每次实例化处检查

### Requirement: record 构造表达式

record 构造表达式是 `TypeRef { field: expr, ... }`：头 `TypeRef` 是命名类型引用——所在模块的 PascalCase 标识符，或按第 6 章抵达已导入模块公开 record 的 `module.Name`——花括号内为逗号分隔的 `field: expr` 对。构造恰好指名 record 的每个字段一次——record 未声明的字段、被遗漏的已声明字段、或被指名两次的字段都以 `E0604` 拒绝。每个字段的表达式 MUST 具有该字段声明的类型；不一致以第 7 章无隐式转换诊断（`E0501`）拒绝。不指名 record 类型的头以 `E0603` 拒绝。在构造的花括号内换行无意义：花括号对第 2 章续行规则是括号，字段以逗号分隔。

#### Scenario: 完整构造

- **WHEN** `User { id: UserId(1), name: "Ada" }` 出现在要求值的位置，`User` 声明 `id: UserId` 与 `name: String`
- **THEN** 它是产出带这些字段的新 `User` 值的表达式

#### Scenario: 字段类型不一致被拒绝

- **WHEN** `User { id: UserId(1), name: 42 }` 出现且 `name: String`
- **THEN** 编译器对 `42` 以 `E0501` 拒绝；修复是显式转换方法

#### Scenario: 字段集错误的构造被拒绝

- **WHEN** 构造指名 `User` 未声明的 `email`，或遗漏 `name`，或指名 `id` 两次
- **THEN** 编译器以 `E0604:` construction names a field set that does not match the record 拒绝

### Requirement: 更新表达式

更新表达式是 `TypeRef { field: expr, ..., with &old }`，头形式与构造相同：它产出头 record 类型的一个新值，其中被指名字段取给定表达式，每个未指名字段从 `old` 复制。基 `old` MUST 是恰为头 record 类型的表达式（否则 `E0603`）；被指名字段集 MUST 只指名已声明字段（`E0604`），每个给定表达式 MUST 匹配其字段类型（`E0501`）。基值不受影响：更新绝不改写它。更新表达式只存在于 gc 与 value record：资源记录有身份，MUST NOT 以此方式更新（`E0606`）；其字段只经 resource 章的释放机制与第 10 章的接收者字段赋值改变——本章推迟给它们的机制，现已落地。

#### Scenario: 更新复制未指名的字段

- **WHEN** `User { name: "bob" with &u1 }` 出现，`User` 声明 `id` 与 `name`，`u1: User`
- **THEN** 结果是新 `User`，其 `name` 为 `"bob"`、其 `id` 为 `u1` 的；`u1` 不变

#### Scenario: 非 record 基被拒绝

- **WHEN** 更新表达式的基类型不同于所构造 record，例如 `Point { x: 1.0 with &origin}` 且 `origin: User`
- **THEN** 编译器以 `E0603:` update base does not have the constructed record type 拒绝

#### Scenario: 资源记录不能被更新

- **WHEN** `FileHandle { fd: 3 with &h }` 出现，`FileHandle` 是 byres record
- **THEN** 编译器以 `E0606:` update expression on a resource record 拒绝

### Requirement: 字段访问

对 record 类型值的后缀成员访问 `receiver.name` 指名该 record 的字段，为第 2 章悬置的字段或方法判定落定 record 一侧：接收者类型指名 record，被访问名是其字段之一，表达式的类型是该字段声明的类型。完整候选集——字段连同固有方法、接口方法与生成的派生方法——是第 10 章的成员名字解析：既非接收者类型的字段亦非其方法的名字在那里被拒绝（`E0816`），方法调用归第 10 章。字段访问只读：mut self 方法体外，语言中任何地方都不存在对字段的赋值——`obj.field = value` 不合任何产生式，MUST 以第 2 章意外 token 诊断（`E0105`）拒绝，无论可见性、类别或模块；体内，第 10 章恰批准 `self.field = expr` 为接收者字段赋值——本章推迟给接口章的通道。

#### Scenario: 字段访问读取字段

- **WHEN** `u.name` 出现且 `u: User` 声明 `name: String`
- **THEN** 它是类型为 `String`、指名该字段值的表达式

#### Scenario: 方法之外不存在字段赋值

- **WHEN** `u.name = "bob"` 出现在 mut self 方法体之外
- **THEN** 编译器以 `E0105:` unexpected token 拒绝；唯一的字段赋值形式是第 10 章的 mut self 体内 `self.field`——`u` 不是接收者——方法之外的修改是产出新值的更新表达式

#### Scenario: 未知成员访问被拒绝

- **WHEN** `u.email` 出现且 `User` 未声明名为 `email` 的字段或方法
- **THEN** 编译器按第 10 章成员名字解析以 `E0816:` no such member on the receiver's type 拒绝

### Requirement: 值记录

`byval record` 具有值语义：赋值、绑定、传参与返回都复制整个值；被复制值的两个绑定互不共享。值 record 的每个字段 MUST 自身属基础类型或 value 类别（否则 `E0601`）——仅当其中一切都可按值复制，复制才是诚实的。

#### Scenario: 传参复制

- **WHEN** `byval record Point` 的值 `p` 被传给函数，被调方从其参数构造另一个 `Point`
- **THEN** 调用方的 `p` 不受影响；参数是独立副本

#### Scenario: 非 value 字段被拒绝

- **WHEN** `byval record Bad { u: User }` 出现且 `User` 是 gc record
- **THEN** 编译器以 `E0601:` value record field is not of the value category or a base type 拒绝

### Requirement: 资源记录

`byres record` 声明一个资源：引用传递，显式生命周期管理。资源记录 MUST 经其 Releasable 实现释放；释放操作、其位置与执行诊断归 resource 章——本章只固定类别与其对更新表达式的排除。复制纪律：资源值的身份即资源；本章没有任何形式复制它（更新为 `E0606`；构造制造新资源，不是副本）。

#### Scenario: 资源记录声明其类别

- **WHEN** `byres record FileHandle { fd: Int64 }` 出现
- **THEN** 它声明 resource 类别的 record；其释放机制归 resource 章

#### Scenario: 资源不得静默丢弃

- **WHEN** 资源记录类型的表达式作为被丢弃的语句站立
- **THEN** 第 8 章值丢弃规则适用且无豁免：非 Unit 值 MUST 被显式丢弃

### Requirement: newtype 声明

newtype 声明是顶层项 `newtype Name(Underlying)`，可选地带 `pub` 前缀：`Name` 是新类型，属 newtype 所有权类别，其运行时布局是其底层类型的并被擦除——零开销、双向无隐式转换。构造是调用形式 `Name(expr)`，`expr` 属底层类型；被调用者指名 newtype 的调用是构造（第 6 章唯一名字空间保证一个名字绝不同时是函数与 newtype，故调用形式绝无歧义）。解包是字段访问 `.value`，其类型是底层类型。newtype 与其底层类型——或与任何其他类型——混合以第 7 章 `E0501` 拒绝。newtype MAY 按第 10 章携带 derives 子句——`newtype UserId(Int64) derives Eq` 生成 `.equals`；泛型 newtype 未批准。本章记录的 derives 推迟至此落地。

#### Scenario: 构造与解包

- **WHEN** `let id = UserId(42)` 而后 `let n = id.value` 出现，声明了 `newtype UserId(Int64)`
- **THEN** `id` 类型为 `UserId`，`n` 类型为 `Int64` 且值为 `42`

#### Scenario: 双向皆无隐式转换

- **WHEN** `UserId(1) + 1` 或 `let uid: UserId = 42` 出现
- **THEN** 编译器对两者皆以 `E0501` 拒绝；过渡是 `UserId(42)` 与 `.value`，皆为显式

#### Scenario: 调用形式即构造

- **WHEN** `UserId(42)` 出现且模块同时声明 `fn UserId(x: Int64)`——并不存在，因为 `E0404` 禁止该冲突
- **THEN** 唯一名字空间使构造读法成为唯一读法；无需歧义规则

#### Scenario: 带 derives 子句的 newtype 解析

- **WHEN** `newtype UserId(Int64) derives Eq, Show` 出现在顶层
- **THEN** 它按第 10 章声明带生成 `.equals` 与 `.toDebugString` 方法的 newtype

### Requirement: 元组类型与表达式

元组类型是 `(T1, T2, ..., Tn)`，n 从 2 到 8，每个元素是类型引用；元组表达式是 `(e1, e2, ..., en)`，元数相同；元组表达式的类型是其元素类型的元组，且在任何类型一致位置元组类型 MUST 逐元素一致（`E0501`）。元数超过八以 `E0602` 拒绝——修复是改用 record，它给字段命名。元组不实现接口；该义务的机制归接口章。unit 类型不是元组：`()` 是自己的形式，括号化单表达式 `( e )` 是分组，不是元组。

#### Scenario: 元组值

- **WHEN** `let pair: (Int64, String) = (1, "a")` 出现
- **THEN** `pair` 具该元组类型；元素类型与注解逐元素一致

#### Scenario: 元素不一致被拒绝

- **WHEN** `let pair: (Int64, String) = ("a", 1)` 出现
- **THEN** 编译器在每个不一致元素处以 `E0501` 拒绝

#### Scenario: 元数超过八被拒绝

- **WHEN** 九个元素的元组类型或表达式出现
- **THEN** 编译器以 `E0602:` tuple arity above eight 拒绝；修复是改用 record

#### Scenario: 单元素是分组不是元组

- **WHEN** `( e )` 出现在任何表达式位置
- **THEN** 它按第 2 章分组 `e`；不存在一元元组类型

### Requirement: unit 类型

unit 类型是 `()`；它恰有一个值，写作 `()`，不携带数据。无值块——末项为语句，或为空——具有 unit 类型，修订第 7 章块值类型化。unit 类型的表达式可自由丢弃；其他所有类型落入值丢弃规则。unit 类型可出现在注解槽与元组类型的元素位置。

#### Scenario: unit 值

- **WHEN** `()` 出现在表达式位置
- **THEN** 它是 unit 类型的唯一值

#### Scenario: 空块具有 unit 类型

- **WHEN** 块为空或其末项是语句
- **THEN** 块的类型是 `()`，可站立在要求 unit 类型之处

### Requirement: 元组模式与解构

元组模式是 `(p1, p2, ..., pn)`：每个子模式是任何已批准模式，递归嵌套；模式匹配同元数的元组值，把每个子模式的绑定自对应元素绑定。元组模式对非同元数元组的匹配对象（scrutinee）在 match 或绑定位置以第 7 章 `E0501` 拒绝。一个模式 MUST NOT 绑定同一名字两次（`E0404`）。绑定语句的名字位接受元组模式——`let (a, b) = pair` 逐元素解构，规则与 match 相同。第 4 章或模式一致性对元组模式原样适用：各备选 MUST 绑定相同名字集。穷尽性：元组类型恰在其元素类型可穷尽的范围内可穷尽——由不可穷尽元素组成的元组需要在某处放一个通配符。

#### Scenario: 以 match 解构

- **WHEN** `match pair { (a, b) => a + b }` 出现且 `pair: (Int64, Int64)`
- **THEN** 该臂命中，把 `a` 与 `b` 绑定到各元素

#### Scenario: 以 let 解构

- **WHEN** `let (a, b) = pair` 出现且 `pair: (Int64, String)`
- **THEN** `a` 是 `Int64`、`b` 是 `String`；该语句是第 2 章绑定语句在名字位放元组模式

#### Scenario: 嵌套模式

- **WHEN** `match trio { (0, (a, _)) => a, _ => 0 }` 出现且 `trio: (Int64, (Int64, Int64))`
- **THEN** 嵌套元组模式匹配嵌套元组；通配符子模式不绑定任何东西

#### Scenario: 单模式内重复绑定被拒绝

- **WHEN** `let (x, x) = pair` 出现
- **THEN** 编译器以 `E0404:` one name bound twice in the same pattern 拒绝

### Requirement: 值丢弃

非 Unit 值 MUST NOT 被静默丢弃。凡表达式的值未被消费之处——表达式语句、无声明返回类型的函数体的末项、控制形式体块的末项、语句位置的未绑定 match 臂体——该表达式 MUST 属 unit 类型或经绑定到 `_` 显式丢弃（`let _ = expr`），否则编译器以 `E0605` 拒绝。unit 类型无需仪式：unit 值表达式自由站立。本条为第 6 章悬置的值级丢弃检查落定。

#### Scenario: 丢弃非 Unit 值被拒绝

- **WHEN** `step()` 作为语句出现而其返回 `Int64`
- **THEN** 编译器以 `E0605:` non-unit value dropped 拒绝；修复是 `let _ = step()` 或消费该值

#### Scenario: 显式丢弃满足规则

- **WHEN** `let _ = step()` 出现在同一位置
- **THEN** 丢弃是显式的，该项合法

#### Scenario: Unit 无需仪式

- **WHEN** `io.println("hi")` 作为语句出现而其返回 unit 类型
- **THEN** 该语句自由站立；不要求任何丢弃形式

### Requirement: if 臂一致

有 else 的 if 的各臂 MUST 类型一致：每个臂的块类型须为同一类型，无值臂的类型是 unit 类型 `()`。不一致在 if 表达式处以第 7 章 `E0501` 拒绝。本条为第 3 章悬置的臂一致规则落定；never 类型对一致的排除归 sum 类型章。语句位置中一致类型非 unit 类型的有 else 的 if 落入值丢弃规则。

#### Scenario: 各臂经 unit 类型一致

- **WHEN** `if flag { io.println("a") } else { io.println("b") }` 出现——两臂皆为 unit 类型
- **THEN** if 表达式具有 unit 类型，作为语句自由站立

#### Scenario: 一臂有值一臂无值

- **WHEN** `if flag { 1 } else { io.println("no") }` 出现在表达式位置
- **THEN** 编译器以 `E0501` 拒绝：第一臂是 `Int64`，第二臂是 `()`；须一臂显式转换或两臂一致

### Requirement: 局部遮蔽

函数体内，靠后的 `let` 或 `var` MAY 重新绑定同作用域中已绑定的名字：新绑定自其语句起胜出，旧绑定失名——它未被改写，只是经该名字不可再达。参数是绑定，同样 MAY 被遮蔽。嵌套作用域按同一规则遮蔽外层绑定。名字解析取最近外围绑定：一个名字的使用指名使用处可见的该名字最内层绑定。模块层归第 6 章：一个名字一个绑定，无遮蔽（`E0404`）。

#### Scenario: 后绑定胜出

- **WHEN** `let x = 1` 而后 `let x = normalize(x)` 出现在同一作用域
- **THEN** 第二个 `x` 是新绑定；其后使用见到规范化值，第一绑定经名字不可再达

#### Scenario: 参数可被遮蔽

- **WHEN** `fn f(a: Int64) { let a = trimmed(a) ... }` 出现
- **THEN** 体内的 `a` 是新绑定；参数仍是其自身绑定右侧 `trimmed(a)` 的实参

#### Scenario: 解析取就近绑定

- **WHEN** 外层作用域绑定 `n: Int64`，内层臂绑定 `n: String`，且臂体使用 `n`
- **THEN** 该使用指名臂的 `String` 绑定——使用处最近外围的绑定

## 示例（非权威）

下面的示例只用第 1–8 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。引用模型标注为待其章节。

### record、构造、更新

```we
record User {
    id: UserId,
    name: String,
}

byval record Point {
    x: Float64,
    y: Float64,
}

newtype UserId(Int64)

let u1 = User { id: UserId(1), name: "Ada" }
let u2 = User { name: "bob" with &u1 }   // new value; u1 unchanged
let id = u1.id.value                     // field access, then unwrap

User { id: UserId(1), name: 42 }         // E0501: name is String
User { id: UserId(1), email: "a" }       // E0604: no such field
User { name: "bob" with &p }             // E0603: p is not a User
u1.name = "bob"                          // E0105: no field assignment
```

### 所有权类别

```we
byval record Bad { u: User }             // E0601: User is gc, not
                                        // value or a base type

byres record FileHandle {
    fd: Int64,
}
// release operations and Releasable enforcement are the
// resource chapter's; this chapter fixes the category:

FileHandle { fd: 3 with &h }             // E0606: resources are
                                        // never updated this way

fn scale(p: Point, k: Float64) -> Point {
    Point { x: p.x * k, y: p.y * k }    // byval: the caller's p is
}                                       // an independent copy
```

### 元组与 unit 类型

```we
let pair: (Int64, String) = (1, "a")
let (n, s) = pair                        // destructuring let

match pair {
    (0, tag) => tag,
    (count, _) => "many",
}

let nothing = ()                         // the unique unit value
let alsoNothing = { io.println("hi") }   // block typed ()

(1, "a", 3, 4, 5, 6, 7, 8, 9)            // E0602: arity above eight
let one: (Int64) = (1)                   // grouping, not a tuple
```

### 值丢弃

```we
fn step() -> Int64 { ... }

step()                                   // E0605: non-unit value dropped
let _ = step()                           // explicit discard: legal
io.println("hi")                         // unit: no ceremony needed

fn quietly() {
    step()                               // E0605: valueless function,
    let _ = step()                       // same rule at the body tail
}
```

### if 臂一致与遮蔽

```we
let flag = true

if flag { 1 } else { io.println("no") }  // E0501: Int64 vs ()

let x = 1
let x = normalize(x)                     // later binding wins; the
                                        // first x is out of reach

fn trim(a: String) -> String {
    let a = a.trim()                     // shadowing a parameter is
    a                                     // legal: nearest binding
}
```

### 待后续章节

```we
// Ref/Shared with the concurrency chapters; List, indexing, and the
// other collection types with the collections chapter — methods,
// interfaces, impl, and derives landed with chapter 10:
//
// let xs: List<Int64> = build()
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| ownership category | 所有权类别 |
| gc category | gc 类别 |
| value category | value 类别 |
| resource category | resource 类别 |
| newtype category | newtype 类别 |
| record | record（记录） |
| field | 字段 |
| construction expression | 构造表达式 |
| update expression | 更新表达式 |
| value record | 值记录 |
| resource record | 资源记录 |
| newtype | newtype（新类型包装） |
| tuple | 元组 |
| tuple pattern | 元组模式 |
| unit type | unit 类型 |
| value discard | 值丢弃 |
| explicit discard | 显式丢弃 |
| arm agreement | 臂一致 |
| local shadowing | 局部遮蔽 |
| nearest binding | 就近绑定 |
