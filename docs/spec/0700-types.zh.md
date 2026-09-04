# We 语言规范 —— 第 7 章：类型

### Requirement: 基础类型清单

基础类型是十三个成员的封闭集：检查整数 `Int8` `Int16` `Int32` `Int64` `UInt8` `UInt16` `UInt32` `UInt64`，IEEE 754 浮点 `Float32` `Float64`，`Bool`，`String`（不可变 UTF-8 字节序列；逻辑迭代产出 `Rune`，其协议由可迭代协议章节批准），`Bytes`（字节序列），以及 `Rune`（单个 Unicode 码点，U+0000..U+10FFFF）。`Never` 与 `()` 不是本清单的成员：底类型由 sum 类型章节（第 9 章）批准，unit 类型由复合类型章节（第 8 章）批准；二者各自成类，不是基础类型。新增基础类型是修订本清单的规范层变更。

#### Scenario: 基础类型名指名其类型

- **WHEN** 类型引用指名十三个基础类型之一，例如 `Int64` 或 `String`
- **THEN** 它指名该基础类型；这些名字按第 1 章命名约定为 PascalCase

#### Scenario: 清单是封闭的

- **WHEN** 后续章节需要新的基础类型
- **THEN** 它只能通过修订本 Requirement 清单的规范层变更进入；别无他路

### Requirement: 字面量类型化

无后缀整数字面量是 `Int64`；无后缀浮点字面量是 `Float64`。带后缀数值字面量指名其后缀指名的类型：`i8`..`i64` 映射到 `Int8`..`Int64`，`u8`..`u64` 到 `UInt8`..`UInt64`，`f32`/`f64` 到 `Float32`/`Float64`，按第 1 章封闭后缀集。关键字字面量 `true` 与 `false` 是 `Bool`；字符串字面量是 `String`；rune 字面量是 `Rune`。

#### Scenario: 无后缀字面量取默认类型

- **WHEN** 字面量 `42` 与 `3.14` 出现
- **THEN** `42` 是 `Int64`，`3.14` 是 `Float64`

#### Scenario: 后缀字面量指名其类型

- **WHEN** 字面量 `42u8`、`1000i32` 与 `3.14f32` 出现
- **THEN** 它们分别是 `UInt8`、`Int32` 与 `Float32`

#### Scenario: 关键字与定界字面量自定类型

- **WHEN** `true`、`"text"` 与 `'x'` 出现
- **THEN** 它们分别是 `Bool`、`String` 与 `Rune`

### Requirement: 无隐式转换

We 没有隐式转换。不同类型的操作数 MUST NOT 组合：混合整数位宽、有符号与无符号、两种浮点位宽，以及 `String`、`Bytes`、`Rune` 中任两者互混，都以 `E0501` 拒绝。该义务覆盖值类型必须与槽位一致的每个位置：绑定的表达式 MUST 匹配其注解，调用实参 MUST 匹配其参数，返回表达式 MUST 匹配声明的返回类型——每处不一致都以 `E0501` 拒绝，绝不插入强制转换。每次跨类型过渡都是显式方法（`toInt64()`、`toBytes()` 及其同族；方法清单归标准库）。本义务同等约束后续类型章节：未来某类型向另一类型的隐式转换是需要独立变更的规范层例外。

#### Scenario: 混合整数位宽被拒绝

- **WHEN** `Int64` 与 `Int32` 操作数组合，例如 `a + b`，`a: Int64` 且 `b: Int32`
- **THEN** 编译器以 `E0501:` operands of different types 拒绝；修复是显式转换方法

#### Scenario: 有符号与无符号永不隐式混合

- **WHEN** `Int64` 与 `UInt64` 操作数组合
- **THEN** 编译器以 `E0501:` operands of different types 拒绝；符号转换是显式的

#### Scenario: String、Bytes 与 Rune 保持分离

- **WHEN** `String`、`Bytes`、`Rune` 中两个的 操作数组合，例如比较 `String` 与 `Rune`
- **THEN** 编译器以 `E0501:` operands of different types 拒绝；`toBytes()`、`toString()` 及其同族的过渡是显式的

#### Scenario: 不同类型的注解被拒绝

- **WHEN** 绑定注解 `Int32` 而其表达式是无后缀字面量 `42`（即 `Int64`）——`let n: Int32 = 42`
- **THEN** 编译器以 `E0501` 拒绝，无强制转换；修复是后缀（`42i32`）或显式转换方法

#### Scenario: 实参与返回值匹配其声明类型

- **WHEN** 调用实参的类型不同于其参数注解，或返回表达式的类型不同于声明的返回类型
- **THEN** 编译器在该位置以 `E0501` 拒绝不一致；不插入强制转换

### Requirement: 整数溢出语义

整数算术是检查的；它 MUST NOT 静默回绕。编译期可判溢出——字面量表达式常量求值可见的溢出——是编译错误（`E0502`）。运行时溢出是检查陷阱：操作绝不产出回绕值，陷阱构造名与捕获边界由错误机制章节批准。回绕语义只能经显式方法获得（`wrappingAdd()` 及其同族；清单归标准库）。浮点算术遵循 IEEE 754，不设溢出检查。

#### Scenario: 常量溢出是编译错误

- **WHEN** 字面量表达式在编译期溢出其类型，例如作为 `Int64` 的 `9223372036854775807 + 1`
- **THEN** 编译器以 `E0502:` integer overflow 在该表达式处拒绝

#### Scenario: 运行时溢出陷阱而非回绕

- **WHEN** 整数操作在运行时溢出，例如相加两个和超过最大值的 `Int64`
- **THEN** 操作不产出回绕值；它以检查失败陷阱，其构造由错误机制章节指名

#### Scenario: 回绕是显式的

- **WHEN** 需要回绕语义，例如哈希计算中
- **THEN** 作者显式调用回绕方法；普通运算符保持检查语义

### Requirement: 类型引用

类型引用——填充第 2 章与第 6 章批准的每个类型注解槽的形式——是命名类型或结构复合：命名类型是 PascalCase 标识符，可选模块限定为 `module.Name`，按第 6 章触达已导入模块的公开类型，并可选按第 10 章携带泛型应用 `Name<T1, ..., Tk>`——实参为类型引用、元数依声明自身的子句；元组类型是 `(T1, T2, ..., Tn)`，n 从 2 到 8，每个元素自身是类型引用；unit 类型是 `()`；`Dyn<Interface>` 按第 10 章指名类型擦除的接口箱；函数类型 `fn(T1, ..., Tn) -> T` 按 fn-types 章是类型引用形式——零或多个逗号分隔的参数类型（每个是类型引用）、箭头、必写的返回类型（自身是类型引用，无值位为 unit 类型 `()`）；其文法与一致性归该章。其他一切类型文法尚不存在；随其归属章节经本章修订到达。类型槽持有命名引用、泛型应用、`Dyn<Interface>` 形式、元组类型、unit 类型或函数类型以外的任何东西 MUST 以第 2 章意外 token 诊断（`E0105`）拒绝。良构名字是否解析到已批准类型，随模块解析由模块系统章节决定。

#### Scenario: 模块限定的类型引用

- **WHEN** 注解持有 `user.User`，`user` 指名已导入模块
- **THEN** 它是指向该模块公开 `User` 类型的类型引用

#### Scenario: 注解槽中的元组类型

- **WHEN** 注解持有 `(Int64, String)`
- **THEN** 它是二元元组的类型引用，元数界限依第 8 章

#### Scenario: 注解槽中的泛型应用

- **WHEN** 注解持有 `Box<Int64>`，已声明 `record Box<T>`
- **THEN** 它是泛型应用的类型引用，按第 10 章取 `T` 为 `Int64`

#### Scenario: 注解槽中的 Dyn 形式

- **WHEN** 注解持有 `Dyn<Describable>`，`Describable` 是已声明的接口
- **THEN** 它是按第 10 章的类型擦除箱的类型引用

#### Scenario: 注解槽中的函数类型

- **WHEN** 注解持有 `fn(Int64) -> Int64`
- **THEN** 它是按 fn-types 章的函数类型形式；槽位持有该签名的函数

#### Scenario: 未批准的类型文法被拒绝

- **WHEN** 类型槽持有带效应段的函数类型，例如 `fn(Int64) io -> Int64`，或任何其他未批准的类型文法
- **THEN** 编译器以 `E0105:` unexpected token 在槽处拒绝；该形式若会到来，随其归属章节到达

### Requirement: 条件位置

条件位置的操作数是 `Bool`。`if` 条件、`while` 条件与 match 守卫 MUST 是 `Bool`；其他一律以 `E0503` 拒绝。这落地第 3 章条件类型化与第 4 章守卫类型化。

#### Scenario: 非 Bool 条件被拒绝

- **WHEN** `if` 或 `while` 条件不是 `Bool`，例如 `if count`，`count: Int64`
- **THEN** 编译器以 `E0503:` condition is not Bool 拒绝

#### Scenario: 非 Bool 守卫被拒绝

- **WHEN** match 守卫不是 `Bool`，例如 `n if n`，`n: Int64`
- **THEN** 编译器以 `E0503:` condition is not Bool 拒绝

### Requirement: 块值类型化

有值块的类型是其末项表达式条目的类型；无值块——末项为语句或空块——具有 unit 类型 `()`，由第 8 章批准并免除丢弃义务。这落地第 2 章延后的块值类型化。

#### Scenario: 块的类型是其末项表达式的类型

- **WHEN** 块以 `a + b` 结尾，`a`、`b`: `Int64`
- **THEN** 按第 2 章 Blocks and block value，块的类型是 `Int64`

#### Scenario: 无值块具有 unit 类型

- **WHEN** 块的末项是语句，或块为空
- **THEN** 块没有值且其类型是 `()`；依第 8 章，它可站立在要求 unit 类型之处

## 示例（非权威）

下面的示例只用第 1–7 章及其所依托的可迭代协议已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。函数类型与转换/回绕方法清单标注为待其各自章节；字符串迭代归第 11 章。

### 字面量与默认

```we
let n = 42            // Int64: unsuffixed integer default
let x = 3.14          // Float64: unsuffixed float default
let small = 100i32    // Int32: suffix names the type
let mask = 0xFFu8     // UInt8: base prefix and suffix combine
let flag = true       // Bool
let name = "Ada"      // String
let first = 'A'       // Rune

let typed: Float32 = 2.5f32   // annotation and literal agree
```

### 无隐式转换

```we
let a: Int64 = 1
let b: Int32 = 2
let sum = a + b               // E0501: operands of different types

let count: Int64 = 5
let size: UInt64 = 5
let bad = count + size        // E0501: signed and unsigned never mix

let s = "abc"
let c = 'a'
let cmp = s == c              // E0501: String and Rune stay separate

let n: Int32 = 42             // E0501: the literal is Int64, the
                              // annotation is Int32 — no coercion;
                              // write 42i32 or convert explicitly

// every transition is an explicit method (stdlib names, shown
// for shape only):
let wide = small.toInt64()
let bytes = s.toBytes()
```

### 溢出永不静默回绕

```we
let max = 9223372036854775807
let boom = max + 1            // E0502: integer overflow (constant-folded)

let hi: Int64 = 4611686018427387904
let lo: Int64 = 4611686018427387904
let pair = hi + lo            // runtime checked trap, never a wrapped
                              // value; the trap's construct arrives with
                              // the error-mechanism chapter

let hashed = hash()
let mixed = hashed.wrappingAdd(1)   // wrapping is always explicit
```

### 类型引用填充注解槽

```we
fn parse(text: String) -> Int64 {   // named base types
    transform(text)
}

let u: user.User = user.find(1)     // module-qualified reference per
                                    // chapter 6's import names
```

### 条件是 Bool

```we
let count = 3
if count { step() }          // E0503: condition is not Bool
while ready() { poll() }     // legal: the call is Bool

match next() {
    n if n { }               // E0503: guard must be Bool
    _ { }
}
```

### String 按码点迭代

```we
for c in name { step(c) }      // chapter 11: builtin Iterable<Rune>;
                                // c binds each code point in order
```

### 待后续章节

```we
// Fn types landed with the function-types chapter:
//
// let f: fn(Int64) -> Int64 = square
//
// The collection types (List among them) arrive with the collections
// chapter — the generic application form they use is chapter 10's:
//
// let ids: List<UserId> = build()
// fn forEach(items: List<Int64>) { }
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| base type | 基础类型 |
| literal typing | 字面量类型化 |
| suffix | 后缀 |
| implicit conversion | 隐式转换 |
| explicit conversion | 显式转换 |
| type-agreement position | 类型一致位置 |
| checked arithmetic | 检查算术 |
| overflow | 溢出 |
| wrapping | 回绕 |
| type reference | 类型引用 |
| module-qualified name | 模块限定名 |
| annotation slot | 注解槽 |
| condition position | 条件位置 |
| block value | 块值 |
| valueless block | 无值块 |
