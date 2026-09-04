# We 语言规范 —— 第 17 章：集合


### Requirement: 集合类型

标准库声明三个集合类型：`List<T>`——同类型零或多个元素的有序序列；`Map<K, V>`——从一键类型到一值类型的无序有限映射；`Set<T>`——同类型元素的无序有限集。三者皆泛型——元数一、二、一——实参为第 10 章下的类型引用；三者皆为第 8 章下的 gc 类别类型：按引用共享、变异经别名可见、绝不按值语义复制。三个名字经第 15 章在本变更中的修订入预导入：List 字面量定类型无需 import，其余由 Option 先例统辖——类型语言可见，完整方法清单是标准库的表面。三者皆实现第 11 章的 `Iterable`——`List<T>` 迭代其元素，`Set<T>` 迭代其元素，`Map<K, V>` 迭代其 `(K, V)` 二元组条目——迭代语义由下文「快照迭代」固定。标准库进一步的便利方法（`add`、`removeAt`、`put`、`keys` 及其同类）是其自身表面，本章不予批准；本章固定的是承载语义的锚点：类型的类别与可达、字面量、访问、迭代与变异面。

#### Scenario: 三个名字经预导入可见

- **WHEN** 无任何 import 的模块注解 `List<Int64>`、`Map<String, User>` 或 `Set<Rune>`
- **THEN** 每个名字经预导入解析到标准库声明的类型；无需 `import std.collections`

#### Scenario: 集合是 gc 类别

- **WHEN** 两个绑定别名同一 `List<Int64>` 值，且经其中一个别名的 mut 方法调用添加了一个元素
- **THEN** 该元素经另一个别名可见——第 8 章的 gc 语义；集合绝不静默复制

#### Scenario: 泛型元数由声明固定

- **WHEN** 注解持有 `List<Int64, String>` 或 `Map<String>`
- **THEN** 编译器以 `E0828:` type argument arity mismatch 拒绝；`List` 与 `Set` 取一实参，`Map` 取二

### Requirement: List 字面量

列表字面量是 `[e1, e2, ..., en]`——方括号间零或多个逗号分隔的表达式——是第 2 章骨架的初等表达式。每个元素表达式 MUST 依第 7 章无隐式转换规则（`E0501`）确立一个共同类型：字面量的元素类型即该类型，字面量的类型是它的 `List`。空字面量 `[]` 的元素类型取其位置的期望类型——注解、参数、声明的返回——无期望类型位置的空字面量 MUST 以 `E1501:` empty list literal has no expected type 拒绝：本语言不推断任何东西（第 0 章原则 1）。方括号对第 2 章行接续规则即是括号；字面量 MAY 跨行。同一对括号不服务任何其他表达式形式：`expr[expr]` 下标语法不拟合本规范的任何产生式，现在不、将来仅修第 2 章也不——索引唯命名方法（「以命名方法索引」）。

#### Scenario: 元素一致的字面量

- **WHEN** 出现 `let xs = [1, 2, 3]`
- **THEN** 依第 1 章字面量默认，公共类型为 `Int64`，字面量的类型为 `List<Int64>`

#### Scenario: 元素分歧被拒绝

- **WHEN** 出现 `let xs = [1, "two"]`
- **THEN** 编译器以 `E0501:` operands of different types 拒绝——一个元素类型、无隐式转换；显式转换使元素一致

#### Scenario: 空字面量取期望类型

- **WHEN** 出现 `let xs: List<String> = []`，或 `[]` 被传入期待 `List<Int64>` 之处
- **THEN** 空字面量类型化为期望类型的实例；注解是元素类型的唯一来源

#### Scenario: 裸空字面量被拒绝

- **WHEN** 出现 `let xs = []`，绑定处无注解也无其他期望
- **THEN** 编译器以 `E1501:` empty list literal has no expected type 拒绝——写注解，或以元素起头

#### Scenario: 字面量对分歧注解

- **WHEN** 出现 `let xs: List<String> = [1, 2]`——元素定一个元素类型，注解定另一个
- **THEN** 编译器在绑定一致位以 `E0501:` operands of different types 拒绝；无转换可调和二者

#### Scenario: 下标语法无产生式可拟合

- **WHEN** 出现 `xs[0]`——表达式之后的括号运算对象
- **THEN** 编译器按第 2 章意外 token 诊断（`E0105`）拒绝；索引依命名方法，且不修订本章政策任何章都不得批准 `expr[expr]`

#### Scenario: 字面量在括号内跨行

- **WHEN** 字面量写作一行 `[`、元素每行一个、末行 `]`
- **THEN** 换行无语义——方括号在第 2 章下即括号

### Requirement: 以命名方法索引

索引形式 `expr[expr]` MUST NOT 在语言的任何位置解析：它无产生式可拟合，而括号访问所要表达的意思由一个调用点可见契约的命名方法承担。规范锚定的访问族：`List<T>.get(i: Int64) -> Option<T>`——位置 i 的元素，i 越界时为 `None`——小于零或大于等于长度——位置从零起、无 panic、无部分读；`Map<K, V>.get(k: K) -> Option<V>`——键 k 处的值，缺席时 `None`；`Set<T>.has(t: T) -> Bool`。越界访问是 `None`，不是陷阱也不是错误通道：范围推理留在对返回 Option 的局部 `match` 里，局部可判定（第 0 章原则 1）。`String` 依同政策携其双层，落在第 7 章 Rune 迭代事实上：字节层 `.byteLength() -> Int64` 与 `.byteSlice(start: Int64, end: Int64) -> String`——端斥，跨度 `start..end` 如第 5 章的范围；范围越界的字节操作是 panic——第 14 章家族——因为字节范围指名不存在的存储；码点层 `.runeCount() -> Int64` 与 `.charAt(i: Int64) -> Rune`，越界索引同样 panic。两层绝不共名：字节偏移与码点位置是不同的数字，方法名告诉读者手里握的是哪一个（第 0 章原则 4）。

#### Scenario: List 访问返回 Option

- **WHEN** 三元素列表上出现 `xs.get(2)`，同样出现 `xs.get(3)`
- **THEN** 前者是 `Some(element)`，后者是 `None`；越界是一个值，调用者对之 match

#### Scenario: Map 与 Set 访问

- **WHEN** 无该键的映射上出现 `m.get("id")`，集合上出现 `s.has(x)`
- **THEN** 映射访问为 `None`——缺席是一个值——集合成员测试为 `Bool`

#### Scenario: 不可能范围的字节操作 panic

- **WHEN** 运行时出现 `"hello".byteSlice(0, 99)` 或 `"hello".charAt(7)`
- **THEN** 进程经第 14 章 panic 家族终止——范围指名不存在的存储；范围检查归调用者，panic 是边界

#### Scenario: 字节层与码点层绝不共名

- **WHEN** 读者在 String 上看到 `.byteSlice` 或 `.charAt`
- **THEN** 名字本身言明所在层——字节或码点；不存在可供猜测的无索引 `.slice`

### Requirement: 快照迭代

在 gc 集合上调用 `iterator` 即刻固定迭代的元素序列：返回的迭代器 MUST 恰产出调用 `iterator` 时刻集合持有的元素——`List` 按集合之序，`Map` 与 `Set` 按快照自身在调用时刻固定的一序——无序类型不保证跨迭代器之序，只保证每个迭代器自己的序列是其调用的快照——调用之后添加或移除的元素——经任何别名——MUST NOT 被该迭代器所见。新的 `iterator` 调用返回新快照。此规则使读循环即知迭代集（第 0 章原则 1），兑付第 11 章悬置的 gc 义务为快照，并与第 13 章资源先物化纪律复合：集合迭代的任何部分都不依赖循环文本未展示的执行。快照是语义义务而非实现命令：实现可以复制、不可变共享或写时复制，只要可观察序列是调用时刻的。

#### Scenario: 迭代中的变异不被所见

- **WHEN** 从列表取得迭代器，经别名变异列表，然后完全消费该迭代器
- **THEN** 产出元素恰为调用时刻的；变异对该迭代器不可见，对下一次 `iterator` 调用可见

#### Scenario: 新迭代器，新快照

- **WHEN** 一次变异前后各调用一次 `iterator`
- **THEN** 第一个返回的迭代器产出变异前序列，第二个产出变异后序列——皆与调用时刻快照一致

#### Scenario: Set 与 Map 迭代同为快照

- **WHEN** 从 `Set<T>` 取得迭代器，或从元素为 `(K, V)` 条目的 `Map<K, V>` 取得
- **THEN** 调用时刻规则同样成立；每个集合类型的迭代器都是其调用时刻持有之物的快照

### Requirement: 集合变异显式且经别名可见

集合类型携带 mut 方法——第 10 章接收者规则下的 `mut self` 方法——且 MUST NOT 携带任何其他变异面：无字段赋值（集合不声明字段）、无更新表达式（第 8 章的 `with &` 属记录）、无可变异的运算符。经一个别名调用的 mut 方法对每个别名可见——第 8 章的 gc 语义——而另一别名迭代期间的变异是安全的，因为迭代是快照：迭代器的序列在此之前已固定。锚定访问族之外的方法清单是标准库的表面；本章固定的是每一变异都是读者可见绑定上的命名方法调用。

#### Scenario: mut 方法经别名可见

- **WHEN** 在被第二绑定别名的列表上调用 `shared.add(x)`，然后调用第二绑定的 `size` 族方法
- **THEN** 添加的元素得到反映——gc 别名，第 8 章；调用处无复制发生

#### Scenario: 命名方法之外无变异形式

- **WHEN** 集合值是赋值 `xs = ...` 的目标——重绑定而非变异——或任何运算符的目标
- **THEN** 重绑定遵循第 2 章赋值规则且不触及其他别名；集合变异唯 `mut self` 方法调用

### Requirement: 集合诊断段位

集合章拥有注册表段位 `E1500`–`E1599`，以 owner `1700-collections` 声明于 `docs/spec/diagnostics.toml` 的 `[segments]`。分配：`E1501` empty list literal has no expected type。`E1500` 与 `E1502`–`E1599` 为本章修订预留；需要更多段位码的章自领段位。下标尝试、元素分歧、组合子纯度与迭代器句柄违例携带其所属章的码——`E0105`、`E0501`、`E1402`、`E0904`——本段位无一码与之重复。

#### Scenario: 段位可检索

- **WHEN** 诊断消费者在注册表中查任何 `E15xx` 码
- **THEN** 段条目指名 owner `1700-collections`，且每个已分配码的条目携带 severity、title、description、remediation、owner、requirement 与 allocated 日期

#### Scenario: 后续变更在段内扩展

- **WHEN** 本章的后续 spec 层变更需要新码
- **THEN** 它在其自身变更内从 `E1500`–`E1599` 分配；将 collections 诊断改号到其他章的事实之上不存在

## 示例（非权威）

下面的示例只用第 1–17 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码；标注 panic 的行指名第 14 章的家族。

### 列表字面量与访问

```we
let xs = [1, 2, 3]                     // List<Int64>: the elements agree
let ys: List<String> = []              // the annotation is the only source
                                       // of the empty literal's element type

let bare = []
// E1501: empty list literal has no expected type — write the annotation

let mixed = [1, "two"]
// E0501: operands of different types — one element type; convert
// explicitly so the elements agree

match xs.get(3) {                      // out of range is a value, not a trap
    Some(n) => put(n)
    None => put("absent")
}

let bad = xs[0]
// E0105: unexpected token — a bracketed index fits no production;
// indexing is by the named access methods
```

### Map、Set 与 String 双层

```we
fn total(m: Map<String, Int64>) -> Int64 {
    match m.get("count") {             // absence is a value
        Some(n) => n
        None => 0
    }
}

fn isAdmin(s: Set<Int64>, id: Int64) -> Bool {
    s.has(id)                          // membership is a Bool
}

let runes = "hì".runeCount()           // 2: code points
let bytes = "hì".byteLength()          // 3: UTF-8 bytes — a different number

let slice = "hello".byteSlice(0, 3)    // "hel": bytes, end exclusive
let beyond = "hello".charAt(7)
// panics: the index names storage that does not exist; range checks
// are the caller's, the panic is the boundary
```

### 快照迭代

```we
let it = jobs.iterator()               // the sequence is fixed here
jobs.add(newJob)                       // a mut method through the binding —
                                       // not seen by it; gc aliasing, and
let fresh = jobs.iterator()            // a fresh call sees the fresh state

for (name, count) in m {               // Map iterates (K, V) entries
    put(name)
}
```

### 组合子

```we
let out = [1, 2, 3, 4, 5]
    .iterator()
    .filter(|x| x > 2)
    .map(|x| x * 10)
    .collect()                         // List<Int64>: 30, 40, 50 — layers,
                                       // no intermediate collection

let total = [1, 2, 3]
    .iterator()
    .fold(0, |acc, x| acc + x)         // 6: eager, left to right

let biggest = [1, 2, 3]
    .iterator()
    .reduce(|a, b| if a > b { a } else { b })   // Some(3): empty gives None

let eff = names.iterator().map(|s| load(s))
// E1402: function value effect set does not match the expected type's
// — combinators take pure functions; effects are the for statement's

let each = names.iterator().forEach(|s| put(s))
// E0816: no such member on the receiver's type — forEach does not exist
```

## 术语对照

本章关键术语，英中对译，以保翻译一致：

| English | 中文 |
| --- | --- |
| collection types | 集合类型 |
| list literal | 列表字面量 |
| element agreement | 元素一致 |
| expected type | 期望类型 |
| named-method indexing | 命名方法索引 |
| byte layer | 字节层 |
| code-point layer | 码点层 |
| out of range | 越界 |
| snapshot | 快照 |
| alias | 别名 |
| mutation surface | 变异面 |
| lazy combinator | 惰性组合子 |
| eager combinator | 急性组合子 |
