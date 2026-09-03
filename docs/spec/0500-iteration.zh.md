# We 语言规范 —— 第 5 章：迭代

### Requirement: For 语句

for 语句是 `for name in expr block`：`for` 与 `in` 是第 1 章关键字，`name` 是标识符或 `_`（绑定空），`expr` 是表达式，体是第 2 章块。for 是语句：不产出值，MUST NOT 出现在要求值的位置（`E0202`）。可迭代表达式在迭代开始前恰求值一次。语句按序迭代可迭代值的元素，每个恰一次；每个元素使体执行一次且 `name` 绑定到该元素，每次迭代的绑定相互独立、作用域限于该次执行——MUST NOT 泄漏过循环。体内 `break` 与 `continue` 依第 3 章 Break and continue 合法：for 对该条而言是循环。可迭代表达式的类型能否被迭代——可迭代与迭代器协议——由类型章节批准。

#### Scenario: for 作为语句项

- **WHEN** `for i in 0..3 { sum = sum + i }` 作为块项出现
- **THEN** 解析为 for 语句；体每元素执行一次，`i` 依次绑定为 `0`、`1`、`2`

#### Scenario: for 用于取值位置

- **WHEN** 出现 `let x = for i in 0..3 { i }`——for 用于要求值的位置
- **THEN** 编译器以 `E0202:` valueless form in value position 拒绝

#### Scenario: 循环绑定不泄漏

- **WHEN** 某 for 语句绑定 `i`，例如 `for i in items { use(i) }`
- **THEN** `i` 在语句之后不可见；每次迭代的绑定是自己的作用域

#### Scenario: for 内的 break 与 continue

- **WHEN** 解析 `for i in items { if bad(i) { break } step(i) }`
- **THEN** break 依第 3 章 Break and continue 合法并退出 for；continue 同理进入下一元素

#### Scenario: 通配名绑定空

- **WHEN** 出现 `for _ in 0..3 { step() }`
- **THEN** 体每元素执行一次且无绑定；`_` 永远不是绑定名

#### Scenario: 循环头解构未批准

- **WHEN** 出现 `for (a, b) in pairs`
- **THEN** 编译器按第 2 章意外 token 诊断拒绝，直至元组模式经其成对修订批准

### Requirement: 区间表达式

区间运算符 `..` 是二元运算符，第 2 章优先级表最松的一档，本档非结合：`a..b..c` 以 `E0104` 拒绝。区间右排他：`start..end` 不含 `end`。数值操作数下，迭代按单位步进从 `start`（含）到 `end`（不含）依次产出每个值——`0..3` 迭代 `0`、`1`、`2`——start 不低于 end 的区间零元素。区间是表达式：表达式合法处皆可用，不限于 for 头；任一操作数上的算术与后缀形式结合更紧（`0..n-1` 即 `0..(n-1)`）。Range 类型、其类型化、非数值操作数迭代、其 Iterable 实现由类型章节批准。

#### Scenario: 区间作为表达式

- **WHEN** 出现 `let r = 0..n`
- **THEN** 解析为第 2 章最松二元档上的区间表达式；`0..n-1` 分组为 `0..(n-1)`

#### Scenario: 右排他

- **WHEN** 运行 `for i in 0..3 { collect(i) }`
- **THEN** `i` 取 `0`、`1`、`2`——永不为 `3`

#### Scenario: 链式区间被拒绝

- **WHEN** 解析 `a..b..c`
- **THEN** 编译器以 `E0104:` chained non-associative operator 拒绝；每个区间必须是单一显式组

#### Scenario: 开放端区间未批准

- **WHEN** 出现 `..5` 或 `5..`
- **THEN** 编译器按第 2 章意外 token 诊断拒绝；区间两端必须齐备

#### Scenario: 空区间零迭代

- **WHEN** 出现 `for i in 3..0 { step() }`
- **THEN** 体执行零次；语句不迭代即完成

## 示例（非权威）

下面的示例只用第 1–5 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。迭代器协议与组合子标注为待类型章节成对修订。

### 区间上的 for

```we
var sum = 0
for i in 0..10 {            // 右排他：i 取 0..9，永不为 10
    sum = sum + i
}
// i 在此处不可见：每次迭代的绑定是自己的作用域

for _ in 0..3 {             // 通配名：体跑 3 次，
    retry()                 // 不绑定
}

for i in 3..0 {             // start 不低于 end：零迭代
    never()
}
```

### 可迭代表达式上的 for

```we
for item in loadItems() {   // 表达式恰求值一次，
    handle(item)            // 其后元素按序迭代
    if skip(item) { continue }
    if done(item) { break } // break/continue 合法：for 依第 3 章
}                           // Break and continue 是循环

let x = for i in 0..3 { i } // E0202: valueless form in value position
for (a, b) in pairs {       // E0105: 头解构随元组模式
    use(a, b)               // （成对修订）进入
}
```

### 区间作为表达式

```we
let r = 0..n                // 区间是表达式，不限于 for
let page = offset..offset + size
// 分组为 offset..(offset + size)：算术结合紧于 `..`

let bad = a..b..c           // E0104: chained non-associative operator
let open = ..5              // E0105: both range bounds are required
```

### 待类型章节

```we
// 可迭代/迭代器协议及 iterator() 等方法由类型章节成对修订批准；
// 组合子另需闭包、Option、List：
//
// let out = items.iterator()
//     .filter(|x| x > 2)
//     .map(|x| x * 10)
//     .collect()
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| for statement | for 语句 |
| iterable expression | 可迭代表达式 |
| loop variable | 循环变量 |
| iteration | 迭代 |
| element | 元素 |
| range | 区间 |
| right-exclusive | 右排他 |
| unit step | 单位步长 |
