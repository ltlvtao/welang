# We 语言规范 —— 第 9 章：Sum types

### Requirement: sum 类型声明

sum 类型声明是顶层项 `type Name = V1 | V2 | ... | Vn`，n 至少为一，可选以 `pub` 与 `byval` 为前缀，并可选按第 10 章在名字后携带泛型参数子句、在末变体后携带 derives 子句：`Name` 是 sum 类型的名字，每个 `Vi` 是一个变体——单元变体写作 `Unit` 单独出现，或 `Unit(T1, T2, ..., Tk)`，k 从 1 到 8，每个 `Ti` 是第 7 章修订下的类型引用。首变体之前没有 `|`。载荷数超过八以 `E0701` 拒绝——修复是 record 载荷，其为字段命名。类型名与每个变体名均为 PascalCase（`E0011`），并加入第 6 章的模块唯一名字空间，不得与任何其他名字冲突（`E0404`）。排版遵循第 2 章：变体间的 `|` 是运算符 token，故跨行声明在每行行尾携带 `=` 或 `|`——完整变体之后以 `|` 起行的行按第 2 章续行诊断（`E0102`）拒绝。sum 类型是第 8 章下的复合类型：`type` 为 `gc` 类别、`byval type` 为 `value` 类别；resource 与 newtype 类别不适用于 sum，且 sum 类型 MUST NOT 在声明后改变类别。泛型载荷的类别诚实与派生要求按第 10 章在每次实例化处检查。

#### Scenario: sum 声明解析为顶层项

- **WHEN** `type Shape = Circle(Float64) | Rectangle(Float64, Float64)` 出现于顶层
- **THEN** 它声明 gc sum 类型 `Shape`，携带变体 `Circle` 与 `Rectangle` 及其载荷类型

#### Scenario: 多行声明尾随分隔符

- **WHEN** 声明写为一行 `type Shape =`、次行 `Circle(Float64) |`、末行 `Rectangle(Float64, Float64)`
- **THEN** 各行按第 2 章续行集接续，声明解析为一个项

#### Scenario: 独立成行的前导分隔符被拒绝

- **WHEN** 声明写为一行 `type Shape = Circle(Float64)`、下一行以 `| Rectangle(Float64, Float64)` 起始
- **THEN** 首行已在边界结束而次行以 `|` 起始——该 token 不能作为语句的开头；编译器以 `E0102:` statement begins with a continuation token 拒绝

#### Scenario: 载荷数超过八被拒绝

- **WHEN** 出现带九个载荷类型的变体
- **THEN** 编译器以 `E0701:` variant payload arity above eight 拒绝；修复是 record 载荷

#### Scenario: 变体名在模块名空间冲突

- **WHEN** 模块声明 `type Shape = Circle(Float64)` 且同时声明 `fn Circle(r: Float64)`
- **THEN** 编译器以 `E0404:` duplicate name in one module 拒绝第二个声明

#### Scenario: byval 声明 value 类别

- **WHEN** 出现 `byval type Axis = X(Float64) | Y(Float64)`
- **THEN** 它在第 8 章所有权类别下声明一个 value 类别的 sum 类型

#### Scenario: 带 derives 子句的泛型 sum 解析

- **WHEN** `type Result<T> = Ok(T) | Err(String) derives Eq` 出现在顶层
- **THEN** 它按第 10 章声明单参数 gc sum，其 `.equals` 对 `T` 的要求在每次实例化处检查

### Requirement: 变体构造器

载荷变体以调用形式 `Name(e1, ..., ek)` 构造，k 与声明的载荷数一致，每个 `ei` 是其载荷类型在一致位置上的表达式（不一致则 `E0501`）；构造出的表达式类型是所属 sum 类型。单元变体以裸名构造——一个 PascalCase 标识符表达式，其类型即该 sum 类型。载荷变体的裸名不是一等值——fn-types 章对该问题作出了否定回答：构造只有调用形式，未附带载荷使用的载荷构造器以 `E0704` 拒绝；需要构造器函数时以闭包包装，`|r| Circle(r)`。已导入模块的公开变体按第 6 章以 `module.Name(args)` 触达——与其他所有模块级名字相同的限定形式。唯一名字空间保证一个名字绝不同时是函数与变体（`E0404`），故调用形式永无歧义。

#### Scenario: 带载荷构造

- **WHEN** `Circle(1.0)` 出现于需要 `Shape` 之处，已声明 `type Shape = Circle(Float64) | Rectangle(Float64, Float64)`
- **THEN** 它是携带载荷的 `Shape` 类型表达式

#### Scenario: 单元变体裸名成立

- **WHEN** `None` 出现于需要 `Option` 之处，`None` 是已声明的单元变体
- **THEN** 裸 PascalCase 标识符即构造出的值；不写调用形式

#### Scenario: 载荷类型不匹配被拒绝

- **WHEN** `Circle("big")` 在声明为 `Circle(Float64)` 时出现
- **THEN** 编译器以 `E0501` 拒绝该实参；修复是显式转换

#### Scenario: 缺实参的载荷构造器被拒绝

- **WHEN** `Circle` 作为值出现，而声明为 `Circle(Float64)`
- **THEN** 编译器以 `E0704:` payloaded variant constructor used without arguments 拒绝；以调用形式构造，或需要构造器函数时以闭包包装

### Requirement: 值 sum

`byval type` 在第 8 章 value 类别下具有值语义：构造、赋值、绑定、传参与返回都拷贝整个值。值 sum 的每个载荷类型 MUST 是基础类型或 value 类别（否则 `E0702`）——第 8 章值 record 的镜像：拷贝唯有在其内一切都可按值拷贝时才诚实。

#### Scenario: 传参拷贝

- **WHEN** 值 sum 的值被传给函数而被调方检视它
- **THEN** 被调方持有独立拷贝；调用方的值不受影响

#### Scenario: 非值载荷被拒绝

- **WHEN** `byval type Bad = Wrap(User)` 在 `User` 为 gc record 时出现
- **THEN** 编译器以 `E0702:` value sum payload is not of the value category or a base type 拒绝

### Requirement: 底类型

`Never` 是底类型：它没有值；`Never` 类型的表达式是从不产出值的计算。产出 `Never` 的形式归错误机制章；本章定该类型的位置与一致。`Never` 只可作为已声明的函数返回类型出现：变量绑定、字段、参数、变体载荷或元组元素标注为 `Never` 以 `E0703` 拒绝。`Never` 类型的表达式可出现于任何期望类型的类型一致位置——它从不产出可与之一致的值——这是第 7 章无隐式转换义务的唯一一处刻意豁免，如此记录。`Never` 自臂类型一致中脱出：对第 3 章与第 8 章下的 if、对第 4 章下的 match，`Never` 类型的臂被排除出一致检查，全 `Never` 的诸臂一致为 `Never`。

#### Scenario: 被禁止的标注被拒绝

- **WHEN** 出现 `let x: Never = ...` 或参数 `f(x: Never)`
- **THEN** 编译器以 `E0703:` Never in a non-return annotation position 拒绝

#### Scenario: Never 臂脱出一致检查

- **WHEN** if 或 match 某臂的类型为 `Never` 而其余臂一致为 `Int64`
- **THEN** 该构造的类型是 `Int64`；`Never` 臂不参与比较

#### Scenario: 全 Never 臂一致为 Never

- **WHEN** 带 else 的 if 或 match 的每臂类型均为 `Never`
- **THEN** 该构造的类型是 `Never`

#### Scenario: Never 表达式满足任何返回类型

- **WHEN** 函数声明 `-> Int64` 而某路径的末表达式类型为 `Never`
- **THEN** 该路径满足声明；该表达式从不产出值

## 示例（非权威）

下面的示例只用第 1–9 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。产出 `Never` 的错误形式标注为待其章节；一等构造器之问已答——构造只有调用形式。

### 声明与构造

```we
type Shape =
    Circle(Float64) |
    Rectangle(Float64, Float64)

byval type Axis = X(Float64) | Y(Float64)

pub type Event = Ping | Message(String)

let c = Circle(1.0)                       // payloaded: call form
let quiet = Ping                          // unit: bare name
let e = std_event.Message("hi")           // module-qualified variant

type Wide = Nine(Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64)
                                         // E0701: payload arity above eight
type Shape = Circle(Float64) | fn(Float64)
                                         // E0404: variant name collides
```

### 匹配与穷尽性

```we
fn area(s: Shape) -> Float64 {
    match s {
        Circle(r) => 3.14159 * r * r
        Rectangle(w, h) => w * h         // exhaustive: no wildcard needed
    }
}

match code {
    200 | 404 => handleFine()
    _ => handleOther()                   // base type: wildcard required
}

match s {
    Circle(r) => r
}                                        // E0305: Rectangle not covered
match s {
    Circle(r) if r > 1.0 => r
    Rectangle(w, h) => w
}                                        // E0306: guarded arm needs an
                                         // unguarded exhaustive fallback
match s {
    _ => 0.0
    Circle(r) => r                       // E0307: arm after a wildcard
}
match s {
    Circle(r) => r
    Circle(r) => 0.0                     // E0307: variant listed twice
    _ => 0.0
}
```

### 变体模式

```we
match s {
    Triangle(r) => r                     // E0303: no such variant
}
match s {
    Circle(a, b) => a                    // E0304: payload arity mismatch
}
let Circle(r) = c                        // E0105: refutable patterns
                                         // are match-only

match pair {
    (Circle(r), _) => r                  // nesting: variant in tuple
    _ => 0.0
}

match either {
    (a, _) | (_, a) => a                 // E0501: a binds Int64 here
    _ => 0                               // and String there
}
```

### 底类型

```we
fn bail() -> Never { ... }               // producers: error-mechanism
                                         // chapter's

let x: Never = bail()                    // E0703: only return-type
fn f(x: Never) { }                       // annotations allowed

fn find(id: Int64) -> Int64 {
    if id == 0 {
        bail()                           // Never satisfies Int64 here
    } else {
        id
    }
}
```

### 待后续章节

```we
// The forms that produce Never (panic, todo, trap names) arrive with
// the error-mechanism chapter; Eq/Show derives on sums and Dyn<...>
// open sets landed with the interfaces chapter. The first-class
// constructor question the function-types chapter carried is answered
// negatively — construction is the call form only:
//
// let g = Circle                        // E0704: payloaded variant
//                                        // constructor used without
//                                        // arguments; |r| Circle(r) wraps
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| sum type | sum 类型 |
| variant | 变体 |
| unit variant | 无载荷变体 |
| payloaded variant | 载荷变体 |
| payload | 载荷 |
| variant constructor | 变体构造器 |
| value sum | 值 sum |
| bottom type | 底类型 |
