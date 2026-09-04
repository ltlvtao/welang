# We 语言规范 —— 第 10 章：接口与泛型

### Requirement: 接口声明

接口声明是顶层项 `interface Name { items }`，可选以 `pub` 为前缀，并可选按本章"泛型参数声明"在名字后携带泛型参数子句。`Name` 为 PascalCase（`E0011`），加入第 6 章的模块唯一名字空间，不得与任何其他名字冲突（`E0404`）。花括号组对第 2 章行接续是块状的：其各项各占一行，以推断边界分隔，无分隔 token。项是方法签名与关联类型声明（"关联类型"）；接口声明的是能力、不是数据——`name: type` 字段项不合任何产生式，按第 2 章意外 token 诊断（`E0105`）拒绝。方法签名是 `fn name(receiver, p1: T1, ..., pn: Tn) -> R`——接收者按"方法接收者"首置且裸写，其余参数与可选的返回标注依第 6 章形式；无接收者的签名以 `E0801` 拒绝。方法 MAY 携带块体——按"默认方法"的默认方法。方法名为 camelCase（`E0012`）。接口成员不带 `pub`：其触达即接口自身的。

#### Scenario: 接口解析为顶层项

- **WHEN** `interface Describable { fn describe(self) -> String }` 出现于顶层
- **THEN** 它声明带唯一方法 `describe` 的接口 `Describable`，按接口自身的可达性可达

#### Scenario: 无接收者的方法被拒绝

- **WHEN** 接口方法写作 `fn describe(d: Describable) -> String`——带标注的首参数、无裸接收者
- **THEN** 编译器以 `E0801:` method declares no receiver 拒绝；第一个参数是接收者、裸写

#### Scenario: 接口名在模块名空间冲突

- **WHEN** 模块声明 `interface Describable { ... }` 且同时声明 `record Describable { ... }`
- **THEN** 编译器以 `E0404:` duplicate name in one module 拒绝第二个声明

#### Scenario: 带关联类型的泛型接口解析成立

- **WHEN** 接口写为一行 `interface Container<T> {`、项 `type Element` 与 `fn first(mut self) -> Element` 各自成行、末行 `}`
- **THEN** 它按本章"泛型参数声明"与"关联类型"声明带关联类型 `Element` 与方法 `first` 的单参数接口

### Requirement: 关联类型

关联类型在接口体内以独立项 `type Name` 声明：`Name` 为 PascalCase（`E0011`），一个接口至多声明四个关联类型（否则 `E0803`），且声明不带上界——`type Name: Bound` 以 `E0804` 拒绝。关联类型命名一个由各实现固定的类型空洞：在接口的方法签名内该名字可用作类型引用；所属接口的关联类型名在那里、在其 impl 的签名与体内均为合法类型引用。接口的每个 impl MUST 以项 `type Name = TypeRef` 绑定每个已声明的关联类型——右侧为第 7 章修订下的类型引用，允许 impl 自身子句的裸泛型参数——且绑定 MUST 先于该 impl 的全部方法定义（否则 `E0806`）；缺失绑定以 `E0805` 拒绝。声明关联类型的接口 MUST NOT 以 `Dyn` 装箱（`E0819`，"Dyn 值"）：擦除值无法履行绑定。

#### Scenario: 关联类型被声明并使用

- **WHEN** 接口各自成行声明 `type Element` 与 `fn next(mut self) -> Element`，且某 impl 在其方法之前绑定 `type Element = Int64`
- **THEN** 该接口的 `next` 在该 impl 处返回绑定类型 `Int64`

#### Scenario: 五个关联类型被拒绝

- **WHEN** 接口声明五个关联类型
- **THEN** 编译器以 `E0803:` associated type count above four 拒绝

#### Scenario: 声明上的上界被拒绝

- **WHEN** `type Element: Eq` 出现于接口体内
- **THEN** 编译器以 `E0804:` associated type declares an upper bound 拒绝；约束属于使用处的 where 子句

#### Scenario: 缺失绑定被拒绝

- **WHEN** `Stream` 的某 impl 定义了其方法但未绑定 `Element`
- **THEN** 编译器以 `E0805:` impl misses an associated type binding 拒绝

#### Scenario: 方法定义之后的绑定被拒绝

- **WHEN** 某 impl 的 `type Element = Int64` 项出现在其首个方法定义之下
- **THEN** 编译器以 `E0806:` associated type binding after a method definition 拒绝；绑定在前

### Requirement: impl 声明

impl 声明是顶层项 `impl Name for Head { items }`——可选在 `impl` 后带泛型子句、在头类型后带 where 子句——为头类型实现接口 `Name`。接口 MUST 是本模块或已导入模块已声明的接口；头 MUST 是名义类型——record、newtype 或 sum 名，或其泛型应用——其余一切不合产生式：元组头、基础类型头或裸泛型参数头以 `E0811` 拒绝。项是关联类型绑定而后方法定义，行接续块状如接口。局部性——孤儿规则：仅当接口或头类型声明于本模块时，impl 才在该模块合法（否则 `E0810`）；impl 块本身不带 `pub`——其方法的触达是"固有 impl"的方法级 `pub` 与接口自身的触达。完备性：impl MUST 定义接口每个无默认的方法（否则 `E0807`），MAY 定义有默认者；被定义的方法 MUST 与其接口方法签名一致——同名、同接收者可变性、同参数个数与依序类型、同返回类型，参数名可异（否则 `E0808`）。唯一性：一代换下接口对一个头类型至多实现一次——第二个 impl、或与具体 impl 重叠的泛型 impl，以 `E0809` 拒绝。带关联类型接口的 impl 先按"关联类型"绑定之。

#### Scenario: impl 解析、绑定并定义

- **WHEN** `impl Describable for User { fn describe(self) -> String { self.name } }` 出现，`Describable` 声明 `fn describe(self) -> String`
- **THEN** 它为 `User` 实现 `Describable`；按 impl 头方法的接收者是 `self: User`，签名与接口一致

#### Scenario: 缺失无默认方法被拒绝

- **WHEN** 接口声明 `describe` 与 `render`、均无默认，而某 impl 只定义 `describe`
- **THEN** 编译器以 `E0807:` impl misses a non-defaulted interface method 拒绝，指名 `render`

#### Scenario: 签名不一致被拒绝

- **WHEN** 接口声明 `fn set(mut self, key: String)` 而 impl 写 `fn set(self, key: String)`
- **THEN** 编译器以 `E0808:` impl method signature mismatches the interface method 拒绝——接收者可变性不同

#### Scenario: 重复 impl 被拒绝

- **WHEN** 模块两度持有 `impl Describable for User { ... }`
- **THEN** 编译器以 `E0809:` duplicate impl of one interface for one type 拒绝第二个

#### Scenario: 孤儿 impl 被拒绝

- **WHEN** 模块导入 `Describable` 与 `User` 两者并写 `impl Describable for User { ... }`——两者均非本地声明
- **THEN** 编译器以 `E0810:` orphan impl 拒绝；接口或头类型必须声明于本模块

#### Scenario: 元组 impl 头被拒绝

- **WHEN** `impl Describable for (Int64, String) { ... }` 出现
- **THEN** 编译器以 `E0811:` impl head is not a nominal type 拒绝；元组不实现接口——第 8 章的义务在此兑现

#### Scenario: 带 where 子句的泛型 impl 解析成立

- **WHEN** `impl<T> Describable for Box<T> where T: Describable { fn describe(self) -> String { self.value.describe() } }` 在已声明 `record Box<T> { value: T }` 时出现
- **THEN** 它按"where 子句"为每个 `T` 实现了 `Describable` 的 `Box<T>` 实现 `Describable`

### Requirement: 方法接收者

每个方法——接口签名或 impl 块中的定义——以接收者开始：第一个参数，裸写不带类型标注，两种形式之一 `self` 或 `mut self`；其类型是 impl 头的类型，由 impl 固定、从不写出。无接收者的方法以 `E0801` 拒绝——固有 impl 亦然，否则方法无所附丽。接收者 MUST 命名为 `self`；拼写按第 1 章强制其大小写约定的同一模型强制，任何其他名字以 `E0802` 拒绝。`self` 不是关键字——它是普通 camelCase 绑定，体内如任何绑定般可用，`mut` 是第 1 章关键字；接收者形式是独立产生式：impl 块方法中的裸首参数。`self` 不可变绑定接收者——字段读取与方法调用自由；`mut self` 可变绑定之，并额外允许"接收者字段赋值"的接收者字段赋值。value 类别类型——`byval` record 或 sum——上的 `mut self` 接收者以 `E0812` 拒绝：值语义没有诚实的原地变更；gc 与 resource（`byres`）类型允许之。mut self 方法经任何接收者表达式可调用，不可变绑定在内：绑定的不可变管重绑定、不管对象的状态——写入原地落在共享值上，这正是所裁决原地通道的意义所在。

#### Scenario: self 接收者读取

- **WHEN** impl 方法 `fn upper(self) -> String { self.name }` 在 `self: User` 声明 `name: String` 时运行
- **THEN** 主体经不可变绑定读取接收者的字段

#### Scenario: mut self 接收者声明变更

- **WHEN** gc record 上的 impl 方法声明为 `fn rename(mut self, to: String)` 且其体写 `self.name = to`
- **THEN** 接收者可变绑定，该写入即本章的接收者字段赋值

#### Scenario: 另命接收者被拒绝

- **WHEN** impl 方法声明为 `fn upper(me) -> String`
- **THEN** 编译器以 `E0802:` method receiver must be named self 拒绝；该拼写使每个方法体全语言统一

#### Scenario: value 类别类型上的 mut self 接收者被拒绝

- **WHEN** `byval` record 上的 impl 声明 `fn bump(mut self)`
- **THEN** 编译器以 `E0812:` mut self receiver on a value-category type 拒绝；值拷贝没有可原地变更之物

#### Scenario: mut self 方法经不可变绑定运行

- **WHEN** 在 `let c = Counter { count: 0 }` 上调用 `c.bump()`，`bump` 是写 `self.count` 的 mut self 方法
- **THEN** 调用成立且写入原地落在共享值上——`c.count` 读到新值；`c` 自身绝不被重绑定

### Requirement: 接收者字段赋值

在 `mut self` 方法体内，`self.field = expr` 是全语言唯一的字段赋值形式，开通第 8 章推迟给接口章的通道：gc 接收者原地变更——随后经同一绑定的读取看到该写入——resource（`byres`）接收者同样变更。目标恰为 `self.field`：一层，接收者拼作 `self`（`E0802`），名字是接收者类型的字段（否则 `E0816`）；`self.f.g = e`、`u.name = e` 与其余一切字段目标不合任何产生式，按第 2 章意外 token 诊断（`E0105`）拒绝。出现在 mut self 体外——`self` 方法内、普通 fn 内或顶层——的 `self.field = expr` 以 `E0813` 拒绝。表达式 MUST 与字段声明类型一致（`E0501`）。赋值是第 2 章下的语句：不产出值。

#### Scenario: gc 接收者原地变更

- **WHEN** `u.rename("bob")` 在 gc `User` 上运行 `fn rename(mut self, to: String) { self.name = to }`
- **THEN** 接收者自身的字段被写入；随后经同一绑定的 `u.name` 读到 `"bob"`——未发生拷贝

#### Scenario: resource 接收者原地变更

- **WHEN** `byres` record 的方法 `fn setFd(mut self, fd: Int64) { self.fd = fd }` 运行
- **THEN** resource 的字段被原地写入；resource 的同一性不变

#### Scenario: 普通 self 方法内的写入被拒绝

- **WHEN** 声明为 `fn reset(self)` 的方法写 `self.count = 0`
- **THEN** 编译器以 `E0813:` field write outside a mut self method body 拒绝；修复是接收者处 `mut self`

#### Scenario: 未知字段的写入被拒绝

- **WHEN** 某 mut self 体写 `self.email = x` 而接收者的类型未声明 `email`
- **THEN** 编译器以 `E0816:` no such member on the receiver's type 拒绝

### Requirement: 固有 impl

固有 impl 是顶层项 `impl Head { items }`、无 `for`：它把方法直接附于名义类型——头形式与名义规则是"impl 声明"的。局部性是孤儿规则的：固有 impl 仅在声明头类型的模块内合法（否则 `E0810`）——以外在固有方法扩展外来类型不存在；外来类型的能力路由是本地接口。项是按"方法接收者"带接收者的方法定义；定义 MAY 冠以 `pub`、使之自导入模块可达，无 `pub` 则模块局部。for-form impl 中同一 `pub` 只管直名触达——经接口时由接口的可达性管之，无论方法自身如何。固有方法不与该类型的任何其他成员同名（`E0814`，"成员名字解析"）。

#### Scenario: 固有 impl 解析且其方法被调用

- **WHEN** `impl User { fn displayName(self) -> String { self.name } }` 出现且 `u.displayName()` 被调用
- **THEN** 固有方法按"成员名字解析"解析并返回该字段的值

#### Scenario: 外来类型的固有 impl 被拒绝

- **WHEN** 模块导入 `User` 并写 `impl User { ... }` 而不声明之
- **THEN** 编译器以 `E0810:` orphan impl 拒绝；固有方法只附于本地声明的类型

#### Scenario: pub 方法跨模块可达

- **WHEN** 固有 impl 声明 `pub fn emit(self)` 与 `fn log(self)`，而某导入模块持有该类型的值
- **THEN** `emit` 在那里可调用而 `log` 不可——它模块局部；使用处的修复是自身的一个 pub 成员

### Requirement: 成员名字解析

后缀成员访问按接收者的类型解析：候选名字是该类型的字段（第 8 章）、其固有方法（"固有 impl"）、其实现的每个接口的方法（"impl 声明"）与生成的派生方法（"Derives"）。这为第 2 章悬置的字段或方法解析与第 8 章的 record 侧落地。既非该类型字段亦非其方法的名字以 `E0816` 拒绝。一体一名：一个类型 MUST NOT 携带两个同名成员——字段对方法、固有方法对接口方法、或两接口的方法——在责任声明处以 `E0814` 拒绝。接收者静态位置为接口处——`Dyn<I>` 值（"Dyn 值"）或默认方法体内的 `self`（"默认方法"）——候选集仅该接口的方法集：集外名字以 `E0815` 拒绝，无论某擦除类型是否可能携带它；字段永不在集内。无约束泛型参数是不透明的——其上没有方法调用，以 `E0817` 拒绝——where 约束恢复受约束接口的方法集（"where 子句"），调用按其检查。方法调用的实参与结果处处依第 7 章：实参一致 `E0501`、结果类型是方法声明的返回。

#### Scenario: 方法经已实现接口解析

- **WHEN** 调用 `u.describe()`，`User` 以 `fn describe(self) -> String` 实现 `Describable`
- **THEN** 该成员解析到接口的方法；调用的类型是 `String`

#### Scenario: 未知成员被拒绝

- **WHEN** `u.email` 出现而接收者的类型未声明名为 `email` 的字段或方法
- **THEN** 编译器以 `E0816:` no such member on the receiver's type 拒绝

#### Scenario: 同名字段与方法被拒绝

- **WHEN** record 声明字段 `name` 而固有 impl 声明 `fn name(self) -> String`
- **THEN** 编译器以 `E0814:` member name collision on one type 拒绝该冲突

#### Scenario: 两接口的同名方法被拒绝

- **WHEN** 一个类型实现两个都声明 `render` 的接口
- **THEN** 编译器以 `E0814:` member name collision on one type 拒绝第二个 impl

#### Scenario: Dyn 方法集之外的调用被拒绝

- **WHEN** `d.inherentHelper()` 在 `d: Dyn<Describable>` 且 `inherentHelper` 不是 `Describable` 的方法时出现
- **THEN** 编译器以 `E0815:` method call outside the receiver's method set 拒绝；擦除类型自身的成员在那里不可达

#### Scenario: 无约束参数上的调用被拒绝

- **WHEN** `fn show<T>(x: T)` 调用 `x.describe()` 而对 `T` 无约束
- **THEN** 编译器以 `E0817:` method call on an unconstrained generic parameter 拒绝；修复是 `where T: Describable`

### Requirement: 默认方法

接口方法 MAY 声明体——默认方法。impl MAY 省略有默认的方法：默认体为该类型运行；impl MAY 自行定义该方法，签名与每个 impl 方法一致（`E0808`）。默认体内 `self` 的静态位置是接口：其成员仅是接口的方法集（集外 `E0815`），字段永不在集内——那里不知任何具体类型。默认体可经 `self` 调用接口的其他方法；此类调用晚绑定——存在 impl 体处运行 impl 的，否则运行默认的。类型 MUST NOT 继承其固有同名声明之名上的默认——两体将应答一个选择器——以 `E0814` 拒绝；修复是在 for-form impl 中定义该方法，此即覆写。这把 v0.8 的固有优先规则自静默选择收紧为拒绝，记录于本变更的 design。

#### Scenario: 被省略的方法运行默认

- **WHEN** 接口声明 `fn greet(self) -> String { "hello" }` 而 `User` 的 impl 省略 `greet`
- **THEN** `u.greet()` 运行默认体并产出 `"hello"`

#### Scenario: 被定义的方法覆写默认

- **WHEN** 同一接口对 `Admin` 的 impl 定义 `fn greet(self) -> String { "welcome" }`
- **THEN** `a.greet()` 运行 impl 的体——该定义覆写默认

#### Scenario: 默认经 self 调用同胞

- **WHEN** 默认 `greet` 体调用 `self.name()` 而 impl 自行定义 `name`
- **THEN** 该调用运行 impl 的 `name`——经接收者晚绑定

#### Scenario: 固有方法对继承默认被拒绝

- **WHEN** 带固有 `fn greet(self)` 的类型实现该接口而省略 `greet`、继承其默认
- **THEN** 编译器以 `E0814:` member name collision on one type 拒绝；修复是在 impl 中定义该方法

### Requirement: Dyn 值

`Dyn<Interface>` 是第 7 章修订下的类型引用，指名类型擦除箱：携带某实现类型之值的 gc 类别值，仅经接口的方法集触达。`Dyn` 是标准作用域的普通 PascalCase 类型名——非关键字——与基础类型名一样；其冲突与它们的一样归模块系统章。构造恰一形式，显式者：`Dyn<Interface>(expr)`，其中表达式的类型 MUST 实现该接口（否则 `E0818`）；上下文 `Dyn(expr)` 形式不存在、亦无待批者——所裁决的单形式规则，装箱处零推断。接口 MUST 是接口——`Dyn<Int64>`、`Dyn<User>`、`Dyn<T>` 以 `E0820` 拒绝——且 MUST NOT 声明关联类型（`E0819`）。裸接口名不是值类型：`let d: Describable` 或参数 `x: Describable` 以 `E0821` 拒绝——接口仅以 `Dyn<Interface>` 占类型槽。Dyn 箱上的成员调用只见接口的方法集（`E0815`）。Dyn 箱是 gc 值：赋值、绑定、传参与返回共享箱——被装箱值不发生拷贝。

#### Scenario: Dyn 箱被构造并调用

- **WHEN** `let d = Dyn<Describable>(u)` 在 `User` 实现 `Describable` 时出现，随后调用 `d.describe()`
- **THEN** `d` 的类型是 `Dyn<Describable>` 且该调用分发到 `User` 的实现

#### Scenario: 非实现类型的构造被拒绝

- **WHEN** `Dyn<Describable>(42)` 出现而 `Int64` 未实现 `Describable`
- **THEN** 编译器以 `E0818:` Dyn construction from a non-implementing type 拒绝

#### Scenario: 关联类型接口被拒绝

- **WHEN** `Dyn<Stream>(s)` 出现而 `Stream` 声明关联类型 `Element`
- **THEN** 编译器以 `E0819:` Dyn of an interface with associated types 拒绝；绑定无法被履行

#### Scenario: 非接口实参被拒绝

- **WHEN** `Dyn<Int64>(5)` 或 `Dyn<User>(u)` 出现
- **THEN** 编译器以 `E0820:` Dyn argument is not an interface type 拒绝

#### Scenario: 裸接口类型槽被拒绝

- **WHEN** `fn f(x: Describable)` 或 `let d: Describable = ...` 出现
- **THEN** 编译器以 `E0821:` interface name used as a value type 拒绝；箱形式是 `Dyn<Describable>`

### Requirement: Derives

record、newtype 或 sum 声明 MAY 携带 derives 子句：关键字 `derives`（第 1 章修订）尾随封闭集 `Eq`、`Hash`、`Show` 的逗号分隔列表，写于声明的最后一行——`record Point { x: Float64, y: Float64 } derives Eq, Hash`、`newtype UserId(Int64) derives Eq`、`type Shape = Circle(Float64) | Rectangle(Float64, Float64) derives Eq`；完整声明之后以 `derives` 起头的行不合任何产生式（第 2 章 `E0105`）。未知或重复的目标以 `E0824` 拒绝。该子句生成的方法与实现的方法完全一样加入该类型的成员，按"成员名字解析"可调用、可冲突：`Eq` 生成 `.equals(other: Self) -> Bool`，`Hash` 生成 `.hash() -> Int64`，`Show` 生成 `.toDebugString() -> String`；生成体的运行时行为归运行时——本章固定表面与要求。`Eq` 与 `Hash` 要求每个字段类型、底层类型或载荷类型具备该能力——基础类型天然具备、组合类型经其自身 derives 具备——否则 `E0823`；`Show` 无要求。泛型声明上该要求在每次实例化处检查（该处 `E0823`）。目标是内建的：手写 `impl Eq for ...` 以 `E0822` 拒绝——组合相等是生成的 `.equals()`、永非运算符：`==` 按第 7 章仅比较基础类型，此即组合相等的全部故事，如所裁决。`Encodable` 与 `Decodable` 延后至 JSON 变更、`Shareable` 归并发章；它们将经此条的修订扩展该集合。

#### Scenario: 三个 derives 在其声明上解析成立

- **WHEN** `record Point { x: Float64, y: Float64 } derives Eq, Hash`、`newtype UserId(Int64) derives Eq` 与 `type Shape = Circle(Float64) | Rectangle(Float64, Float64) derives Eq` 出现
- **THEN** 各自带其尾随子句解析；`Point` 获得 `.equals` 与 `.hash`、`UserId` 获得 `.equals`、`Shape` 获得 `.equals`——`Show` 同样会生成 `.toDebugString`

#### Scenario: 组合相等是 .equals()

- **WHEN** `p1.equals(p2)` 出现在两个派生 `Eq` 的 `Point` 值上
- **THEN** 该调用解析到生成的方法并产出按字段比较的 `Bool`

#### Scenario: 运算符保持仅基础类型

- **WHEN** `p1 == p2` 出现在 `Point` 值上
- **THEN** 编译器按第 7 章 `E0501` 拒绝；修复是 `p1.equals(p2)`——`==` 仅比较基础类型

#### Scenario: 派生目标的手写 impl 被拒绝

- **WHEN** `impl Eq for User { ... }` 出现
- **THEN** 编译器以 `E0822:` manual impl of a builtin derive target 拒绝；子句是 `record User { ... } derives Eq`

#### Scenario: 未满足的字段要求被拒绝

- **WHEN** `record Wrap { inner: Shape } derives Eq` 出现而 `Shape` 未派生 `Eq`
- **THEN** 编译器以 `E0823:` derive field requirement unmet 拒绝，指名 `Shape`；修复是 `Shape` 也 `derives Eq`

#### Scenario: 未知或重复目标被拒绝

- **WHEN** `derives Ord` 或 `derives Eq, Eq` 出现
- **THEN** 编译器以 `E0824:` unknown or duplicate derive target 拒绝；封闭集是 `Eq`、`Hash`、`Show`

### Requirement: 泛型参数声明

泛型参数子句是 `<T1, ..., Tk>`，k 从 1 到 8——k 超过八以 `E0825` 拒绝，复合诸章之八的元数镜像——每个参数是作用于其声明的 PascalCase 名（`E0011`）。子句可携带于：顶层 fn 声明（名字与参数列表之间）、record 声明（名字后、花括号前）、sum 类型声明（名字后、`=` 前）、接口声明（名字后、花括号前）与 impl 声明（`impl` 后、接口名前）。泛型方法——impl 块方法上的子句——未批准；若需要，经此条的修订到来。子句 MUST NOT 重声明外围子句的名字（`E0826`）；本章未批准任何嵌套语境，该规则自其到来之时起约束——泛型方法或随函数类型章的嵌套声明。声明内参数名在每个类型引用位置可用；无约束参数是不透明的——无方法调用（`E0817`）、运算符依第 7 章规则——直到 where 约束指名之。泛型字段上的类别诚实在每次实例化处检查：`byval record Box<T> { value: T }` 实例化为 gc `User` 的 `Box<User>` 按第 8 章 `E0601` 在实例化处拒绝，值 sum 镜像同样按 `E0702`。

#### Scenario: 泛型 fn 子句解析成立

- **WHEN** `fn identity<T>(x: T) -> T { x }` 作为顶层项出现
- **THEN** 它按本章解析为带单参数子句的一个 fn 声明；其余归第 6 章形式

#### Scenario: record、sum 与接口子句解析成立

- **WHEN** `record Box<T> { value: T }`、`type Result<T> = Ok(T) | Err(String)` 与 `interface Sink<T> { fn send(mut self, item: T) }` 出现
- **THEN** 各自带其子句解析；`T` 分别可用于字段、载荷与签名

#### Scenario: 九个参数被拒绝

- **WHEN** 声明携带九参数子句
- **THEN** 编译器以 `E0825:` generic parameter count above eight 拒绝

#### Scenario: 嵌套到来时遮蔽子句被拒绝

- **WHEN** 后续章节批准某嵌套语境而内层子句重声明外围子句的名字
- **THEN** 编译器以 `E0826:` generic parameter shadows an enclosing parameter 拒绝；规则现在批准、彼时生效

#### Scenario: gc 实参的值实例化被拒绝

- **WHEN** `byval record Box<T> { value: T }` 实例化为 `Box<User>` 而 `User` 是 gc record
- **THEN** 编译器按第 8 章 `E0601` 拒绝该实例化——类别诚实逐实例化检查

### Requirement: 泛型类型引用与推断

类型引用可以是泛型应用 `Name<T1, ..., Tk>`——第 7 章修订下的命名类型规则——其中 `Name` 指名携带 k 参数子句的声明、每个实参是类型引用；元数不匹配以 `E0828` 拒绝。嵌套闭括号必须分隔：最大吞噬把 `>>` 取作移位 token（第 1 章），故嵌套应用写作 `Box<Box<Int64> >`，未分隔的 `Box<Box<Int64>>` 按第 2 章 `E0105` 拒绝并附该修复——这兑现第 1 章运算符清单留给泛型章的指针。泛型 fn 的调用或泛型类型的构造恰以两途之一确定其类型实参：统一——每个实参表达式的类型与其标注参数类型统一，或构造处每个字段或载荷表达式的类型与其声明类型统一——或显式形式：fn 的 `name<T1, ..., Tk>(args)`、newtype 与变体构造的 `Name<T1, ..., Tk>(args)`、record 构造与更新头的 `Name<T1, ..., Tk> { ... }`。期望类型推断不存在：实参不能确定的类型实参以 `E0827` 拒绝——修复是显式形式——所裁决的单向规则使每个调用可由其自身文本判定。显式形式的元数不匹配是 `E0828`。约束在每次调用与实例化处检查（`E0830`，"where 子句"）。

#### Scenario: 推断调用自实参统一

- **WHEN** 在 `fn identity<T>(x: T) -> T` 上调用 `identity(3)`
- **THEN** `T` 与实参的类型统一；调用的类型即该类型——无需显式形式

#### Scenario: 显式调用写出其实参

- **WHEN** `identity<Float64>(1.0)` 或 `Pair<Int64, String> { first: 1, second: "a" }` 出现，`Pair` 声明两个参数
- **THEN** 类型实参如所写，检查元数（`E0828`）与约束

#### Scenario: 未确定的调用被拒绝

- **WHEN** 在 `fn empty<T>() -> Box<T>` 上调用 `empty()`
- **THEN** 编译器以 `E0827:` generic call does not determine its type arguments 拒绝；修复是 `empty<Int64>()`

#### Scenario: 元数不匹配被拒绝

- **WHEN** 单参数的 `identity` 上出现 `identity<Int64, String>(1)`
- **THEN** 编译器以 `E0828:` type argument arity mismatch 拒绝

#### Scenario: 未分隔的嵌套闭括号被拒绝

- **WHEN** `Box<Box<Int64>>` 出现于类型槽
- **THEN** 词法最大吞噬已把 `>>` 取作移位 token，编译器按第 2 章 `E0105` 拒绝该槽，修复是分隔形式 `Box<Box<Int64> >`

#### Scenario: 构造自字段推断

- **WHEN** `Pair { first: 1, second: "a" }` 在 `record Pair<A, B> { first: A, second: B }` 时出现
- **THEN** `A` 与 `B` 和字段表达式的类型统一；构造的类型是 `Pair<Int64, String>`

### Requirement: where 子句

where 子句尾随泛型 fn 声明的签名（体之前）或 impl 头（花括号之前）：`where c1, c2, ...`，约束逗号分隔，每个是约束 `T: Bound`、多约束 `T: A + B`——此处 `+` 是该文法位置的约束连接符，按第 2 章原则与表达式位置不相交——或关联类型等式 `T.Assoc == Concrete`。约束 MUST 指名已声明的接口（否则 `E0829`）：record、sum、基础类型或泛型参数名不合任何约束。约束授予方法集：声明内，受约束参数携带其约束的方法集（"成员名字解析"），其外对其一无所知。每次调用或实例化的类型实参 MUST 满足约束——不实现某约束的实参以 `E0830` 拒绝。等式的右侧 MUST 是具体类型——基础类型、名义类型或其泛型应用；`T.Element == U` 与 `T.Element == T` 以 `E0831` 拒绝——声明内被指名的关联类型即可用作该具体类型。接口声明上的 where 子句本章未批准；那里的约束若需要，经此条的修订到来。

#### Scenario: 约束授予其方法集

- **WHEN** `fn show<T>(x: T) -> String where T: Describable { x.describe() }` 出现
- **THEN** 受约束调用按"成员名字解析"解析；无约束形式将是 `E0817`

#### Scenario: 带加号连接符的多约束解析成立

- **WHEN** where 子句中出现 `where T: Eq + Hash`
- **THEN** 它把 `T` 约束到两者；两个方法集在内部都可用

#### Scenario: 非接口约束被拒绝

- **WHEN** `User` 为 record 时出现 `where T: User`
- **THEN** 编译器以 `E0829:` where bound is not a declared interface 拒绝

#### Scenario: 调用处未满足的约束被拒绝

- **WHEN** 在上述带约束的 `show` 上调用 `show(42)` 而 `Int64` 未实现 `Describable`
- **THEN** 编译器以 `E0830:` type argument does not satisfy a where bound 拒绝

#### Scenario: 非具体等式右侧被拒绝

- **WHEN** `U` 为泛型参数时出现 `where T: Stream, T.Element == U`
- **THEN** 编译器以 `E0831:` associated-type equality right side is not concrete 拒绝

#### Scenario: 等式在声明内固定关联类型

- **WHEN** where 子句携带 `T.Element == Int64` 而主体把 `self.next()` 累加为 `Int64`
- **THEN** 声明内 `T.Element` 即 `Int64`——累加无需转换

### Requirement: 无型变、无子类型

不存在型变标注：没有声明位置标记泛型参数为协变或逆变，没有型变关键字被批准。不存在子类型化：实现接口是能力、不是继承——`impl I for T` 不使 `T` 在期望 `I`、期望 `Dyn<I>` 或期望异实例化泛型处可接受；每个此类位置按第 7 章 `E0501` 拒绝。擦除路由恰是 `Dyn<I>`（"Dyn 值"）、参数化路由恰是带界参数（"where 子句"）：实现 `Describable` 的 `User` 抵达 `fn f(d: Dyn<Describable>)` 仅作为 `Dyn<Describable>(u)`、抵达 `fn g<T>(t: T) where T: Describable` 直接作为 `g(u)`。record 与 sum 是名义且封闭的：语言任何地方不存在被声明的子类型关系。

#### Scenario: 实现类型不是其接口

- **WHEN** `u: User` 实现 `Describable` 而 `f` 期望 `Dyn<Describable>` 时出现 `f(u)`
- **THEN** 编译器按 `E0501` 拒绝；修复是 `f(Dyn<Describable>(u))`——能力不是子类型

#### Scenario: 参数化路由直接接纳之

- **WHEN** `g` 声明为 `fn g<T>(t: T) where T: Describable` 时出现 `g(u)`
- **THEN** 调用成立：`T` 与 `User` 统一，其能力满足约束

### Requirement: 无运算符重载

运算符清单（第 1 章）与运算符语义（第 7 章）是封闭的：没有产生式把运算符绑定到方法——名为 `+`、`==` 或 `[]` 的 impl 或方法不合任何产生式，按第 2 章 `E0105` 拒绝，运算符 token 永非标识符。组合相等是生成的 `.equals()`（"Derives"）；组合上的算术与索引（若存在）是其所属章的具名方法——`.add()`、`.at()`——永非运算符；`==` 仅比较基础类型（第 7 章）。这是长期立场，与 derives 目标集一同裁决：重载会使每个运算符的类型成为谎言。

#### Scenario: 运算符命名的方法被拒绝

- **WHEN** impl 声明 `fn +(self, other: Point) -> Point`
- **THEN** 编译器按第 2 章 `E0105` 拒绝：`+` 是运算符 token、永非方法名；具名方法是 `.add()`

#### Scenario: 组合上的运算符被拒绝

- **WHEN** `Point` 值上出现 `p1 == p2` 或 `p1 + p2`
- **THEN** 编译器以 `E0501` 拒绝两者；修复是 `p1.equals(p2)` 与 `p1.add(p2)`

### Requirement: 接口与泛型诊断段位

接口与泛型章拥有注册表段位 `E0800`–`E0899`，声明于 `docs/spec/diagnostics.toml` 的 `[segments]`。分配：`E0801` method declares no receiver、`E0802` method receiver must be named self、`E0803` associated type count above four、`E0804` associated type declares an upper bound、`E0805` impl misses an associated type binding、`E0806` associated type binding after a method definition、`E0807` impl misses a non-defaulted interface method、`E0808` impl method signature mismatches the interface method、`E0809` duplicate impl of one interface for one type、`E0810` orphan impl、`E0811` impl head is not a nominal type、`E0812` mut self receiver on a value-category type、`E0813` field write outside a mut self method body、`E0814` member name collision on one type、`E0815` method call outside the receiver's method set、`E0816` no such member on the receiver's type、`E0817` method call on an unconstrained generic parameter、`E0818` Dyn construction from a non-implementing type、`E0819` Dyn of an interface with associated types、`E0820` Dyn argument is not an interface type、`E0821` interface name used as a value type、`E0822` manual impl of a builtin derive target、`E0823` derive field requirement unmet、`E0824` unknown or duplicate derive target、`E0825` generic parameter count above eight、`E0826` generic parameter shadows an enclosing parameter、`E0827` generic call does not determine its type arguments、`E0828` type argument arity mismatch、`E0829` where bound is not a declared interface、`E0830` type argument does not satisfy a where bound、`E0831` associated-type equality right side is not concrete。`E0826` 的规则批准时尚无嵌套触发面；一个被批准时它即触发。`E0800` 与 `E0832`–`E0899` 保留给本章修订。触发语义在本章 Requirements；条目在注册表。

#### Scenario: 接口码被发出

- **WHEN** 工具链发出任何 `E08xx` 诊断
- **THEN** 其完整条目可自 `docs/spec/diagnostics.toml` 检索，owner 为 `1000-interfaces`

#### Scenario: 后续变更需要本段位的码

- **WHEN** 后续章节批准需要新接口或泛型诊断的规则——并发章的 `Shareable`、JSON 变更的编解码器
- **THEN** 其变更在同一变更内于 `E0800`–`E0899` 内扩展注册表，或认领自己的段位

## 示例（非权威）

下面的示例只用第 1–10 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。泛型方法、Encodable/Decodable、Shareable 与 effect 集标注为待批或延后。

### 接口、impl 与固有方法

```we
interface Describable {
    fn describe(self) -> String
}

record User {
    id: UserId,
    name: String
} derives Eq, Show

impl Describable for User {
    fn describe(self) -> String {
        "user ${self.name}"
    }
}

impl User {
    pub fn displayName(self) -> String {
        self.name
    }
}

let u = User { id: UserId(1), name: "Ada" }
let text = u.describe()          // through the interface
let name = u.displayName()       // inherent, pub-reachable
```

### 关联类型与默认方法

```we
interface Stream {
    type Element
    fn next(mut self) -> Element
}

interface Greeter {
    fn greet(self) -> String {
        "hello"                  // a default method: bodies in interfaces
    }
    fn name(self) -> String
}

record IntStream {
    data: Int64
}

impl Stream for IntStream {
    type Element = Int64         // bindings come before the methods
    fn next(mut self) -> Int64 {
        self.data = self.data + 1
        self.data
    }
}

impl Greeter for User {
    fn name(self) -> String {
        self.name
    }
}

let g = u.greet()                // default body runs: greet is omitted
```

### 经 mut self 的变更

```we
record Counter {
    count: Int64
}

impl Counter {
    fn bump(mut self) {
        self.count = self.count + 1   // the one field-assignment form
    }
    fn read(self) -> Int64 {
        self.count
    }
}

let c = Counter { count: 0 }
c.bump()
let n = c.read()                 // 1: the gc receiver mutated in place

impl Counter {
    fn reset(self) {
        self.count = 0           // E0813: field write outside a mut self
    }                            // method body
}

byval record Pair {
    left: Int64,
    right: Int64
}

impl Pair {
    fn swap(mut self) {          // E0812: mut self receiver on a
    }                            // value-category type
}
```

### Dyn 值

```we
let d = Dyn<Describable>(u)      // the one construction form
let text = d.describe()          // the interface's method set alone

let bad = Dyn<Describable>(42)   // E0818: construction from a
                                 // non-implementing type
let s = Dyn<Stream>(stream)      // E0819: Stream has associated types
let n = Dyn<Int64>(5)            // E0820: argument is not an interface

fn label(x: Describable) -> String {
    x.describe()
}                                // E0821: interface name used as a
                                 // value type; write Dyn<Describable>
```

### Derives

```we
record Point {
    x: Float64,
    y: Float64
} derives Eq, Hash, Show

newtype UserId(Int64) derives Eq, Show

type Shape = Circle(Float64) | Rectangle(Float64, Float64) derives Eq

let same = p1.equals(p2)         // composite equality is the generated
let text = p1.toDebugString()    // method; Show asks nothing
let h = p1.hash()

let eq = p1 == p2                // E0501: == compares base types only
impl Eq for Point {             // E0822: manual impl of a builtin
}                                // derive target
record Wrap { inner: Shape } derives Ord
                                 // E0824: unknown or duplicate derive
                                 // target; the set is Eq, Hash, Show
```

### 泛型与 where 子句

```we
fn identity<T>(x: T) -> T {
    x
}

record Box<T> {
    value: T
}

impl<T> Describable for Box<T> where T: Describable {
    fn describe(self) -> String {
        "box of ${self.value.describe()}"
    }
}

fn show<T>(x: T) -> String where T: Describable {
    x.describe()
}

fn sum<T>(s: T) -> Int64 where T: Stream, T.Element == Int64 {
    s.next() + 1                 // T.Element is Int64 inside
}

let a = identity(3)              // T unifies with the argument: Int64
let b = identity<Float64>(1.0)   // the explicit form
let nested: Box<Box<Int64> > = Box { value: Box { value: 1 } }
                                 // nested closers separated: >> is the
                                 // shift token under maximal munch

fn empty<T>() -> Box<T> {
    ...                          // a Never expression satisfies any return;
}                                // producers are the error-mechanism
                                 // chapter's

let bad1 = empty()               // E0827: undetermined; write empty<Int64>()
let bad2 = identity<Int64, String>(1)
                                 // E0828: type argument arity mismatch
let bad3: Box<Box<Int64>>        // E0105: unseparated nested closers
```

### 被拒绝的形式

```we
impl Describable for Foreign {  // E0810: orphan impl — neither side
}                                // is declared in this module
impl Describable for (Int64, String) {
}                                // E0811: impl head is not a nominal type
impl Describable for User {
    fn describe(me) -> String {
        ""
    }
}                                // E0802: receiver must be named self
interface Broken {
    fn helper(x: Int64)
}                                // E0801: method declares no receiver
impl User {
    fn name(self) -> String {
        ""
    }
}                                // E0814: member name collision — the
                                 // field name already answers
fn render<T>(x: T) -> String {
    x.describe()
}                                // E0817: unconstrained parameter is
                                 // opaque; add where T: Describable
fn bound<T>(x: T) where T: Point {
}                                // E0829: where bound is not a declared
                                 // interface
```

### 待后续章节

```we
// Generic methods, Encodable/Decodable (the JSON change), Shareable
// (the concurrency chapter), and interface-level where clauses are
// not ratified at this chapter; effect-set checks belong to the
// effects chapter and are deliberately absent here:
//
// impl Box<T> { fn pair<U>(self, other: U) -> ... }
// record Config derives Encodable
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| interface | 接口 |
| impl block | impl 块 |
| method | 方法 |
| receiver | 接收者 |
| associated type | 关联类型 |
| default method | 默认方法 |
| inherent impl | 固有 impl |
| orphan rule | 孤儿规则 |
| derives clause | derives 子句 |
| derive target | 派生目标 |
| type-erased box | 类型擦除箱 |
| Dyn value | Dyn 值 |
| generic parameter | 泛型参数 |
| type argument | 类型实参 |
| where clause | where 子句 |
| bound | 约束 |
| method set | 方法集 |
| member name resolution | 成员名字解析 |
| capability | 能力 |
| subtyping | 子类型化 |
| variance | 型变 |
| late-bound | 晚绑定 |
