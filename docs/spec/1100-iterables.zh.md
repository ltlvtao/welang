# We 语言规范 —— 第 11 章：可迭代协议

### Requirement: Option 类型

标准库声明泛型 sum 类型 `pub type Option<T> = Some(T) | None`——第 9 章声明形式下的 gc sum，携带第 10 章的一个泛型参数，命名与触达依第 6 章模块规则。`Some(T)` 携带元素类型 `T` 的值；`None` 是其缺席。构造与匹配完全遵循第 9 章：`Some(expr)` 以调用形式构造载荷变体，`None` 以裸名作为无载荷变体，变体模式解构二者。`Option` 是本语言唯一的 canonical 选项类型：接口需要 element-or-absence 结果时——本章的 `next` 是第一个——MUST 返回 `Option`，后续各章 MUST 将其选项形结果绑定于它而非另立兄弟。选项的其余方法清单（`unwrap` 等）是标准库自身表面，不在本章批准。

#### Scenario: Some 与 None 的构造

- **WHEN** `Some(1)` 与 `None` 出现于需要 `Option<Int64>` 之处
- **THEN** 二者均为该已声明标准库 sum 类型的值，依第 9 章构造器

#### Scenario: match 取回携带值

- **WHEN** 对 `Option<Int64>` 的 match 携带臂 `Some(n) => n + 1` 与 `None => 0`
- **THEN** 变体模式按第 9 章绑定，诸臂一致于 `Int64`

#### Scenario: 泛型实例化按使用处检查

- **WHEN** `Option<String>` 出现于类型槽
- **THEN** 它是第 10 章下的泛型应用，其构造器在该实例化处定型

### Requirement: Iterator 接口
标准库声明泛型接口 `pub interface Iterator<T> { fn next(mut self) -> Option<T> }`——第 10 章声明形式下的单方法接口。`next` 将迭代器推进一个元素并返回 `Option<T>`：尚有元素时为 `Some(element)`——迭代器 MUST 恰好每元素返回一次、按序——耗尽后为 `None`。耗尽是永久的：首个 `None` 之后的每次调用都返回 `None`，且迭代器 MUST NOT 被回卷、重置或重放。迭代器天然有状态——`mut self` 接收者即推进本身——故值类别头无法诚实实现本接口：`byval` 头上的 `mut self` 方法是第 10 章的 `E0812`。本接口开放：模块可依第 10 章 impl 规则为自己的名义头实现 `Iterator<T>`，与任何接口相同——自定义集合产出自定义迭代器，静态分发。组合子——`map`、`filter` 及其同族——是本接口自己的默认方法，随集合章节的修订批准：其签名、惰性、纯度与定出规则由本章"Iterator 组合子"条目固定。

#### Scenario: next 按序产出元素后耗尽

- **WHEN** 在 `0`、`1`、`2` 之上的迭代器跨耗尽调用 `next`
- **THEN** 它返回 `Some(0)`、`Some(1)`、`Some(2)`，然后 `None`，此后每次调用仍为 `None`

#### Scenario: 用户迭代器解析并实现

- **WHEN** 模块声明 gc record `Cursor` 并书写 `impl Iterator<Int64> for Cursor`，其 `next(mut self) -> Option<Int64>` 推进位置字段
- **THEN** 该 impl 依第 10 章合法——孤儿规则、签名匹配、唯一性——其调用静态分发

#### Scenario: 值类别迭代器头被拒绝

- **WHEN** 为 `byval` record 书写 `impl Iterator<Int64>`，带 `fn next(mut self) -> Option<Int64>`
- **THEN** 编译器以 `E0812:` mut self receiver on a value-category type 拒绝，依第 10 章

#### Scenario: 组合子调用解析到默认方法

- **WHEN** 在实现 `Iterator<T>` 的类型的值上调用 `.map(...)` 或 `.filter(...)`
- **THEN** 调用按第 10 章成员解析解析到接口的默认方法——无 `E0816`——override 它的 impl 精确重复接口签名

### Requirement: Iterator 组合子
`Iterator` 接口声明十一个默认方法——组合子——由本条目固定、由标准库提供默认体：impl 按第 10 章默认方法继承它们，override MUST 精确重复接口签名、含泛型子句（`E0808`）。惰性族在调用处不做任何元素工作并返回派生迭代器：`fn map<U>(mut self, f: fn(T) -> U) -> Dyn<Iterator<U>>`、`fn filter(mut self, f: fn(T) -> Bool) -> Dyn<Iterator<T>>`、`fn take(mut self, n: Int64) -> Dyn<Iterator<T>>`、`fn skip(mut self, n: Int64) -> Dyn<Iterator<T>>`。急性族在调用处把接收者推进至耗尽并返回其结果：`fn collect(mut self) -> List<T>`、`fn fold<U>(mut self, init: U, f: fn(U, T) -> U) -> U`、`fn reduce(mut self, f: fn(T, T) -> T) -> Option<T>`、`fn count(mut self) -> Int64`、`fn any(mut self, f: fn(T) -> Bool) -> Bool`、`fn all(mut self, f: fn(T) -> Bool) -> Bool`、`fn find(mut self, f: fn(T) -> Bool) -> Option<T>`。

惰性组合子返回的迭代器按需提取接收者的剩余元素——`map` 产出每个经 `f` 变换者，`filter` 产出满足 `f` 者，`take` 至多前 `n` 个，`skip` 前 `n` 个之后的一切——而惰性组合子链在同一个原始序列上叠层，每个元素一次流过每一层，无中间集合。接收者绑定保持对同一一次性对象的活句柄——第 8 章的 gc 别名：哪个句柄调用 `next` 就由哪个提取下一元素，本章的恰一次按序与永久耗尽契约约束的是对象、不是每个绑定。急性组合子返回时留下耗尽的接收者：`collect` 按序构建剩余元素的 `List<T>`——集合章节的类型、经本变更的预导入修订预导入可见——`fold` 自 `init` 从左到右应用 `f`，`reduce` 以首元素为种子并在空接收者上产出 `None`，`count` 计数剩余，`any` 与 `all` 测试谓词并在首个定夺元素处停止，`find` 以 `Option` 产出首个满足者、无满足者时为 `None`。

函数参数是纯函数类型——无效果段的 `fn(T) -> U`——故执行效果的闭包不合：值的推断集不是空期望集的子集，拒绝是第 16 章的 `E1402`、落于实参一致位。没有组合子专属的纯度码，也没有 `forEach`：v0.8 一边把它的函数类型定为纯、一边把副作用路由给它，而效果化的它需要没有任何章批准的效果多态——序列上的副作用归 for 语句，其体携带外围声明的效果。`.forEach(...)` 调用是第 10 章的 `E0816`。方法泛型参数——`map` 与 `fold` 的 `U`——依第 10 章单向规则自调用自身文本定出，即函数实参的返回类型；什么都不定的实参是 `E0827`。

#### Scenario: 惰性链按序变换

- **WHEN** `[1, 2, 3].iterator().map(|x| x * 10).collect()` 运行
- **THEN** 结果是持有 `10`、`20`、`30` 的 `List<Int64>`——每个元素一次流过 map 层、按需提取、无中间集合

#### Scenario: 效果闭包不合

- **WHEN** 出现 `names.iterator().map(|s| save(s))` 而 `save` 声明 `effect io`
- **THEN** 编译器以 `E1402:` function value effect set does not match the expected type's 拒绝——`f` 的参数类型是纯函数类型；副作用归 for 语句

#### Scenario: 空接收者上 reduce 产出 None

- **WHEN** 在空或已耗尽的迭代器上调用 `reduce(f)`
- **THEN** 结果是 `None`——种子本应是首元素，而没有首元素

#### Scenario: any 在首个定夺元素处停止

- **WHEN** 调用 `any(f)` 且某元素满足 `f`
- **THEN** 调用返回 `true`、不再提取更多元素；`all` 在首个失败元素处镜像之

#### Scenario: 接收者保持单一活句柄

- **WHEN** `let derived = it.map(f)` 之后对绑定 `it` 自身调用 `next`
- **THEN** 该调用提取该对象的下一元素——一个序列，无论哪个句柄推进它；`derived` 再看不到被提取的元素

#### Scenario: forEach 不存在

- **WHEN** 出现 `xs.iterator().forEach(|x| put(x))`
- **THEN** 编译器以 `E0816:` no such member on the receiver's type 拒绝；序列上的副作用归 for 语句

#### Scenario: U 来自调用自身文本

- **WHEN** 出现 `names.iterator().map(|n| n.size()).count()`
- **THEN** `U` 是 `Int64`，由函数实参的返回类型依第 10 章单向规则定出；无期望类型推断参与

### Requirement: Iterable 接口

标准库声明泛型接口 `pub interface Iterable<T> { type Iter; fn iterator(self) -> Iter }`——第 10 章声明形式下的一个关联类型与一个方法。元素类型 `T` 以泛型参数行进——`List<Int64>` 迭代 `Int64`——而 `Iter`，迭代器句柄类型，是实现方的关联绑定：每条 impl 为其头固定一个具体迭代器类型。`Iterable<T>` 的每条 impl MUST 将 `Iter` 绑定到为同一元素类型实现 `Iterator<T>` 的类型——句柄契约——绑定其他任何东西的 impl 以 `E0904` 拒绝。契约反映进约束：凡已知某类型实现 `Iterable<T>` 之处——具体地，或仅经约束——其 `Iter` 携带 `Iterator<T>` 的方法集，因为 `E0904` 义务是每条 impl 都遵守的接口的一部分。可迭代值可重复迭代：每次 `iterator` 调用返回一个遍历相同元素的新独立迭代器，可迭代值自身绝不被消耗。`Iterable` 声明关联类型，故不可装箱——`Dyn<Iterable<T>>` 是第 10 章的 `E0819`——而 `Iterator<T>` 不声明关联类型，可以装箱，`Dyn<Iterator<T>>`，当需要擦除的迭代器句柄时。

#### Scenario: 每次 iterator 调用产生新迭代器

- **WHEN** 在同一集合绑定上调用 `iterator` 两次，两个返回的迭代器都被完全消耗
- **THEN** 二者按序产出相同元素；集合从其头部重新迭代

#### Scenario: 手写 impl 绑定其迭代器类型

- **WHEN** 模块书写 `impl Iterable<Int64> for IntSet`，项为 `type Iter = IntSetIter` 与 `fn iterator(self) -> IntSetIter`
- **THEN** 该 impl 依第 10 章合法，`IntSetIter` 按句柄契约实现 `Iterator<Int64>`

#### Scenario: Iter 绑定到非迭代器被拒绝

- **WHEN** `Iterable<T>` 的一条 impl 绑定 `type Iter = Int64`——一个未实现任何 `Iterator` 的类型
- **THEN** 编译器以 `E0904:` impl binds Iter to a non-iterator type 拒绝

#### Scenario: 约束的 Iter 携带方法集

- **WHEN** 泛型 fn 声明 `where C: Iterable<Int64>` 且其函数体调用 `c.iterator().next()`
- **THEN** `next` 调用解析到 `Iterator<Int64>` 的方法集；无该约束时调用将是第 10 章的 `E0817`

#### Scenario: 等式约束收窄迭代器类型

- **WHEN** where 子句书写 `where C: Iterable<Int64>, C.Iter == CountingIter`
- **THEN** 声明内部 `C.Iter` 按第 10 章等式规则可用作 `CountingIter`

### Requirement: for 循环协议
第 5 章 for 语句的可迭代表达式 MUST 具有"为某元素类型 `T` 实现 `Iterable<T>`"的类型；其他一切类型以 `E0901` 拒绝。规则是单一形式：仅 `Iterator<T>` 不是 for 可迭代值——裸迭代器由显式 `next` 调用消耗，或在标准库提供适配器后包装之——且除 `Iterable` 外没有接口使类型可迭代。执行：可迭代表达式恰求值一次；`iterator` 恰获取一次；`next` 被反复调用；每个 `Some(element)` 依第 5 章 name 规则绑定该元素并执行函数体一次；首个 `None` 结束循环。元素类型 `T` 即循环绑定的类型。目前已批准的类型携带内建实现——`String` 于 `Rune` 与 `Range<T>` 于 `T`（"String 迭代"、"Range 类型"）——而集合章节的 `List`、`Map` 与 `Set` 是本协议上迭代其元素与条目的可迭代值。

#### Scenario: 非可迭代表达式被拒绝

- **WHEN** `for x in 5 { ... }` 出现——`Int64` 未实现任何 `Iterable`
- **THEN** 编译器以 `E0901:` for-in expression does not implement Iterable 拒绝

#### Scenario: 裸迭代器不是 for 可迭代值

- **WHEN** `for x in iter { ... }` 出现，`iter` 的类型仅实现 `Iterator<Int64>`
- **THEN** 编译器以 `E0901:` for-in expression does not implement Iterable 拒绝；裸迭代器由显式 `next` 调用消耗

#### Scenario: 迭代器恰获取一次

- **WHEN** 同一集合绑定支撑两个 for 语句
- **THEN** 每个语句获取自己的迭代器且都看到全部元素；可迭代值绝不为迭代所消耗

### Requirement: String 迭代

`String` 携带 `Iterable<Rune>` 的内建实现：元素类型是 `Rune`，`Iter` 绑定是标准库上 `Rune` 的迭代器类型——其名属标准库，不属本章——迭代按序产出该字符串的码点，每元素一个。这落地了第 7 章既录事实：字符串的逻辑迭代产出 `Rune`。字节级遍历不是迭代：它是显式 `toBytes()` 转换的，依第 7 章无隐式转换义务，且 `Bytes` 根本不携带 `Iterable` 实现。该实现内建且唯一——手写尝试不合任何 impl 产生式，基类型头即第 10 章的 `E0811`——且一个头之上不存在第二个实现。

#### Scenario: for 遍历字符串按序产出 rune

- **WHEN** `for c in "hì" { put(c) }` 运行
- **THEN** `c` 按序将每个码点绑定为 `Rune`

#### Scenario: 为 String 手写 impl 被拒绝

- **WHEN** 书写 `impl Iterable<Rune> for String`
- **THEN** 编译器以 `E0811:` impl head is not a nominal type 拒绝，依第 10 章；内建实现即唯一实现

#### Scenario: Bytes 不可迭代

- **WHEN** `for b in bytes { ... }` 出现，`bytes: Bytes`
- **THEN** 编译器以 `E0901:` for-in expression does not implement Iterable 拒绝；字节视图是显式转换的

### Requirement: Range 类型

`Range<T>` 是带一个参数 `T` 的内建泛型类型，`T` MUST 是第 7 章的八种整数类型之一：任何其他类型的区间操作数——浮点在内——以 `E0902` 拒绝。区间值仅由第 5 章的区间运算符构造，两个操作数为同一整数类型——混合操作数是第 7 章的 `E0501`——依第 7 章字面规则定型：`0..n` 是 `Range<Int64>`。区间值即其两个边界，别无其他：它不携带状态，绑定、传参或返回都拷贝该边界对。`Range<T>` 携带 `Iterable<T>` 的内建实现——`Iter` 绑定为 `T` 上的标准库迭代器类型——迭代按单位步长产出从 `start` 含至 `end` 排他的每个值：第 5 章的迭代规则，现已类型落地，start 不低于 end 时产出零元素。该实现内建且唯一，与 `String` 的相同（手写尝试 `E0811`）。

#### Scenario: 区间以其类型绑定

- **WHEN** `let r = 0..n` 出现，`n: Int64`
- **THEN** `r` 是 `Range<Int64>`，第 7 章与第 10 章类型引用语法中的内建泛型应用

#### Scenario: 浮点操作数被拒绝

- **WHEN** `1.5..2.5` 出现
- **THEN** 编译器以 `E0902:` range operand is not an integer type 拒绝；浮点跨度用显式循环

#### Scenario: 非数值操作数同样被拒绝

- **WHEN** `'a'..'z'` 出现
- **THEN** 编译器以 `E0902:` range operand is not an integer type 拒绝

#### Scenario: 混合整数操作数被拒绝

- **WHEN** `0i32..9i64` 出现
- **THEN** 编译器依第 7 章以 `E0501` 拒绝；两个边界必须是同一种整数类型

#### Scenario: 迭代按单位步长产出

- **WHEN** `for i in 2..5 { step(i) }` 运行
- **THEN** `i` 取 `2`、`3`、`4`——start 含、end 排他、步进为一

### Requirement: Iterable 实现方义务
`Iterable` 的实现回应第 8 章的所有权类别。resource 类别类型 MUST NOT 实现 `Iterable`——拒绝码为 `E0903`：迭代器在循环的多次执行间持有对其集合的活视图，而触达超出 resource 纪律所依赖的单一确定性释放点的句柄并不诚实；遍历 resource 的内容须先物化——一次显式读入集合，再迭代该集合。值类别可迭代值按拷贝诚实迭代：第 8 章值语义适用——`iterator` 接收接收者自己的副本，迭代绝不消耗或改动原绑定。gc 集合的迭代语义由集合章节固定——`iterator` 调用处的一次快照——而本章固定了它们所依赖的协议。

#### Scenario: resource impl 被拒绝

- **WHEN** 为 `byres` record 书写 `impl Iterable<Int64>`
- **THEN** 编译器以 `E0903:` impl of Iterable for a resource-category type 拒绝；先物化再遍历

#### Scenario: 值可迭代值永不被消耗

- **WHEN** 实现 `Iterable<T>` 的 `byval` record 支撑两个 for 语句
- **THEN** 二者都看到全部元素；绑定不受任一循环影响

### Requirement: 可迭代协议诊断段位

可迭代协议章拥有 `docs/spec/diagnostics.toml` `[segments]` 中声明的注册表段位 `E0900`–`E0999`。分配：`E0901` for-in expression does not implement Iterable、`E0902` range operand is not an integer type、`E0903` impl of Iterable for a resource-category type、`E0904` impl binds Iter to a non-iterator type。`E0900` 与 `E0905`–`E0999` 为本章修订保留。触发语义在本章 Requirements；条目在注册表。

#### Scenario: 可迭代码被发出

- **WHEN** 工具链发出任何 `E09xx` 诊断
- **THEN** 其完整条目可从 `docs/spec/diagnostics.toml` 以 owner `1100-iterables` 取回

#### Scenario: 后续变更需要本段位码

- **WHEN** 后续某章批准需要新可迭代诊断的规则——集合章在其中
- **THEN** 其变更在同一变更中于 `E0900`–`E0999` 内扩展注册表，或认领自己的段位

## 示例（非权威）

下面的示例只用第 1–11 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。下方组合子示例触及第 12 章的闭包与第 17 章的 `List`。

### Option 与迭代

```we
let maybe = Some(3)                    // Some carries its element
let none: Option<Int64> = None         // None stands bare

let score = match maybe {              // arms agree on Int64
    Some(n) => n
    None => 0
}
```

### 迭代器与可迭代值

```we
let it = names.iterator()              // fresh per call; the collection
let first = match it.next() {          // itself is never consumed
    Some(name) => name
    None => "anonymous"
}

impl Iterable<Int64> for IntSet {      // a manual impl binds its own
    type Iter = IntSetIter             // iterator type; the contract:
    fn iterator(self) -> IntSetIter {  // Iter implements Iterator<Int64>
        IntSetIter { set: self, pos: 0 }
    }
}
```

### for 遍历字符串与区间

```we
for c in "hì" {                        // String: builtin Iterable<Rune>
    put(c)                             // c is Rune, code points in order
}

for i in 2..5 {                        // Range<Int64>: 2, 3, 4
    step(i)
}
```

### 被拒绝的形式

```we
for x in 5 { }                         // E0901: for-in expression does
                                       // not implement Iterable
for b in bytes { }                     // E0901: Bytes carries no
                                       // implementation; convert first
let r = 1.5..2.5                       // E0902: range operand is not an
                                       // integer type
let m = 0i32..9i64                     // E0501: the bounds must be one
                                       // integer type
impl Iterable<Int64> for LogFile { }   // E0903: LogFile is of the
                                       // resource category
impl Iterable<Rune> for String { }     // E0811: a base-type head fits no
                                       // impl production
impl Iterable<Int64> for IntSet {
    type Iter = Int64                  // E0904: impl binds Iter to a
    fn iterator(self) -> Int64 { 0 }   // non-iterator type
}
for (a, b) in names { }                // E0501: the element type String
                                       // is not a tuple
```

### 组合子

```we
let out = names.iterator()
    .filter(|n| n.size() > 2)  // lazy: a layer over the receiver
    .map(|n| n.toUpper())      // lazy: U comes from this call's text
    .collect()                 // List<String>: eager — the chain ends

let total = [1, 2, 3].iterator().fold(0, |acc, x| acc + x)   // 6
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| option type | 选项类型 |
| element type | 元素类型 |
| iterator | 迭代器 |
| iterable | 可迭代值 |
| iterator handle | 迭代器句柄 |
| handle contract | 句柄契约 |
| exhaustion | 耗尽 |
| one-shot | 一次性消耗 |
| builtin implementation | 内建实现 |
| code point | 码点 |
| bounds | 边界 |
| materialize | 物化 |
| combinator | 组合子 |
