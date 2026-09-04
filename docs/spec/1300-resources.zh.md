# We 语言规范 —— 第 13 章：资源与 Releasable


### Requirement: Releasable 接口

标准库按第 10 章的声明形式声明接口 `Releasable`：唯一方法 `fn release(mut self)`，`mut self` 接收者，无返回类型——产生 unit 类型——无泛型参数、无关联类型。Releasable 不声明关联类型，故第 10 章的装箱规则本可接纳 `Dyn<Releasable>`；本章的组合位禁令拒绝它（资源只栖绑定位）——资源绝不装箱，盒的共享句柄语义与单句柄纪律相抵。Releasable 是资源类别的释放契约，而资源类别是其唯一实现者：实现 Releasable 的 impl 若其头类型不是 `byres record`——gc 记录、值记录、sum、newtype、基类型、泛型参数——以 `E1102:` Releasable implemented by a non-resource type 拒绝。每个 `byres record` MUST 实现 Releasable；该义务是声明完备性，在记录声明处检查，第 10 章的孤儿规则将 impl 钉在记录自己的模块：声明 `byres record` 的模块若不持有其 Releasable 的 impl，以 `E1101:` resource record must implement Releasable 拒绝。方法的签名是接口的；偏离的 impl——`self` 接收者、声明返回类型——是第 10 章的 `E0808`。

#### Scenario: byres 记录实现 Releasable

- **WHEN** 声明 `byres record FileHandle { fd: Int64 }` 且其模块持有 `impl Releasable for FileHandle { fn release(mut self) { close(self.fd) } }`
- **THEN** 声明完备；FileHandle 携带释放契约，可用于 scope resource 语句

#### Scenario: 非资源实现者被拒绝

- **WHEN** 出现 `impl Releasable for User`，`User` 是 gc 记录
- **THEN** 编译器以 `E1102:` Releasable implemented by a non-resource type 拒绝

#### Scenario: 无 impl 的 byres 记录被拒绝

- **WHEN** 声明 `byres record LogFile { fd: Int64 }` 且其模块不持有 LogFile 的 Releasable impl
- **THEN** 编译器以 `E1101:` resource record must implement Releasable 拒绝；修法是补 impl，不是换记录类别

#### Scenario: 偏离的 release 签名归第 10 章

- **WHEN** Releasable 的 impl 将方法写作 `fn release(mut self) -> Bool`
- **THEN** 编译器以第 10 章的 `E0808` 拒绝：impl 方法签名与接口方法不一致

### Requirement: scope resource 语句

scope resource 语句是 `scope resource(name = expr, ..., name = expr) block`，至少一个绑定。`scope` 与 `resource` 两词经本章修正进入第 1 章的关键字列表；语句以它们开头，故第 2 章的语句首位类——它接纳关键字——接纳它而无需进一步修正，第 2 章的语句族枚举记入本章名下。每个绑定是 `name = expr`，没有注解位：绑定的类型即头表达式的类型，且头表达式的类型 MUST 实现 Releasable——类型不实现的头以 `E1103:` scope resource head does not implement Releasable 拒绝；在上述完备性义务下实现者恰是 `byres record`，故该检查拒绝一切非资源头。头表达式从左到右求值；每个名字为该块绑定。块出口处编译器保证每绑定恰一次 `release` 调用，按声明逆序；被 `return`、`break` 或 `continue` 穿透的出口是块出口，同样释放。scope 块内的 `defer` 是第 3 章的 `E0204`——defer 仅是函数体直接项——且块出口释放先于外围函数自己的 defer 运行，内块先出。该语句不产生值：它不是表达式。

#### Scenario: 新建构造在出口释放

- **WHEN** 函数体中出现 `scope resource(f = openFile("data.txt")) { let n = f.read() }` 且块完成
- **THEN** 块出口处 `release` 为 `f` 恰运行一次，按 FileHandle 的 Releasable impl 所提供的实现

#### Scenario: 出口按声明逆序

- **WHEN** 一条 scope resource 语句先绑 `a` 后绑 `b`，块退出
- **THEN** `b` 的释放先于 `a` 的运行

#### Scenario: return 穿透 scope

- **WHEN** scope 块内含一个提前 `return` 某值
- **THEN** 释放在该出口运行，先于函数的值完成；外围函数自己的 defer 在其后运行，内块先出

#### Scenario: break 穿透 scope

- **WHEN** scope resource 语句出现在循环体内且块内含 `break`
- **THEN** 释放在该出口运行，先于循环被离开

#### Scenario: 非资源头被拒绝

- **WHEN** 出现 `scope resource(u = makeUser())`，`makeUser` 返回 gc 记录
- **THEN** 编译器以 `E1103:` scope resource head does not implement Releasable 拒绝

#### Scenario: scope 块内的 defer

- **WHEN** 出现 `scope resource(f = openFile("a")) { defer { log() } }`——defer 作为 scope 块的项
- **THEN** 编译器以第 3 章的 `E0204` 拒绝：defer 位置；defer 保持为函数体直接项

#### Scenario: 至少一个绑定

- **WHEN** 出现 `scope resource() { }`——空绑定表
- **THEN** 语句不匹配任何产生式，第 2 章的意外 token 诊断（`E0105`）在不提供绑定的 `)` 处报告

### Requirement: 释放纪律

资源绑定的生命周期是线性的且函数局。在每个函数体内，资源类型的绑定——参数、`let` 绑定、scope 头绑定——自绑定点起活，且在每条控制路径上 MUST 恰达三个转移之一：移入 scope 头 `scope resource(x = f)`，头绑定接手而源绑定死于转移点；`return` 该绑定，义务移给调用方；或在调用处以实参传给参数声明该资源类型的调用，义务移给被调方——签名携带它。活资源绑定在其作用域末端否则到达的控制路径以 `E1104:` resource binding outside its release discipline 拒绝；转移点之后使用绑定是同一拒绝。`break` 与 `continue` 同样结束其内层块的作用域：未先转移的 let 型资源即未释放落空。模块顶层不绑资源：资源类型的顶层 `let` 不匹配任何通道，同样以 `E1104:` resource binding outside its release discipline 拒绝。分析只覆盖函数体的流图——跨函数边界的每个义务都写在签名里——故纪律局部可判定（chapter 0, Principle 1）。

#### Scenario: 参数经头转移解除义务

- **WHEN** fn 声明资源类型参数 `f` 且其体为 `scope resource(x = f) { work(x) }`
- **THEN** 参数义务被解除：头绑定接手，`f` 在转移后死亡

#### Scenario: 未释放落空被拒绝

- **WHEN** 函数体含 `let f = openFile("a")` 且一条控制路径未转移 `f` 即到达函数末端
- **THEN** 编译器以 `E1104:` resource binding outside its release discipline 拒绝，指名绑定与路径

#### Scenario: 转移后使用被拒绝

- **WHEN** `f` 被作为实参传给参数声明 `f` 资源类型的 fn，随后一条语句读取 `f`
- **THEN** 编译器以 `E1104:` resource binding outside its release discipline 拒绝该读取；绑定死于实参传递

#### Scenario: return 携义务上行

- **WHEN** 函数体末语句是 `return f`，`f` 是活的资源绑定
- **THEN** 路径合法；义务移给调用方，其自身路径以同法检查

#### Scenario: 模块顶层不绑资源

- **WHEN** 出现模块顶层 `let f = openFile("a")`——资源类型的顶层绑定
- **THEN** 编译器以 `E1104:` resource binding outside its release discipline 拒绝；函数体之外不存在通道

#### Scenario: 条件路径各自转移

- **WHEN** 函数体绑定 `f` 且 `if`/`else` 一臂 `return f`、另一臂传给参数声明该类型的 fn
- **THEN** 每条路径恰一次转移 `f`；函数被接受

### Requirement: 单一释放触发点

释放只有一个触发点：块出口机制。用户码 MUST NOT 直调 `release`——用户码任何表达式位上指名 `release` 且接收者资源类型的方法调用，以 `E1104:` resource binding outside its release discipline 拒绝。在 Releasable 的 impl 内，方法正被定义，其体是普通代码；在那里对资源类型接收者指名 `release` 的调用是同一拒绝。早释放以退出 scope 表达：`return`、`break` 或 `continue` 在边界处释放，且那是唯一拼写。这关死了双重释放缝：scope 头下的绑定不可能被释放两次，因为机制的那次调用是唯一的调用。

#### Scenario: 直调 release 被拒绝

- **WHEN** 函数体出现 `f.release()`，`f` 是活的资源绑定
- **THEN** 编译器以 `E1104:` resource binding outside its release discipline 拒绝；早释放的拼写是早出 scope

#### Scenario: 早出口即早释放

- **WHEN** scope 块打开 `f` 且一个守卫在块末端前 `return`
- **THEN** 释放恰在该出口运行一次；无需也不允许手动调用

### Requirement: 资源绑定无别名

资源绑定是资源的唯一句柄。到同一资源的第二绑定——`f` 资源类型时的 `let g = f` 或 `var g = f`——以 `E1105:` resource binding rebound 拒绝；任一侧资源类型的赋值 `g = f` 是同一拒绝。资源绑定是 let 形：`var f = openFile("a")`，可重绑的资源绑定，同样以 `E1105:` resource binding rebound 拒绝。唯一受准的移动是转移（释放纪律）：它不别名——它交出并杀死源绑定。

#### Scenario: let 别名被拒绝

- **WHEN** 出现 `let g = f`，`f` 是活的资源绑定
- **THEN** 编译器以 `E1105:` resource binding rebound 拒绝

#### Scenario: 资源类型的 var 声明被拒绝

- **WHEN** 出现 `var f = openFile("a")`——资源类型的可重绑绑定
- **THEN** 编译器以 `E1105:` resource binding rebound 拒绝；资源绑定一律 let 形

#### Scenario: 赋值进出资源绑定被拒绝

- **WHEN** 出现 `g = f` 且 `f` 资源类型，或资源类型的 `g` 作为任何赋值的目标
- **THEN** 编译器以 `E1105:` resource binding rebound 拒绝

### Requirement: 资源只栖绑定位

资源类型只出现在绑定位：参数、`let` 或 scope 头绑定、函数的返回类型。资源类型出现在组合位——tuple 元素、gc 记录字段（byval 字段是 E0601 自有的拒绝）、sum payload、newtype 底型——以 `E1106:` resource type in a composite position 拒绝。资源类型作为泛型实参是实例化点处的同一拒绝，无论泛型对参数作何使用——`Box<FileHandle>` 与 `id<FileHandle>` 同拒；泛型体内无法局部判定 maybe-resource 义务，故绑定位精化留作本章修订。装箱路线随之关死：类型位的 `Dyn<Releasable>` 与资源类型实参的构造 `Dyn<Releasable>(r)` 是同一拒绝——在完备性义务下每个 Releasable 盒都装资源，而盒是共享句柄的组合位。已生效的同理裁定持同一到达论证：闭包 MUST NOT 捕获资源绑定（`E1002`，第 12 章），资源类型 MUST NOT 实现 `Iterable`（`E0903`，第 11 章）——到达路径多于一条的句柄越出纪律所依赖的单一确定性释放点。

#### Scenario: gc 记录字段被拒绝

- **WHEN** 出现 `record Conn { file: FileHandle }`——带资源类型字段的 gc 记录
- **THEN** 编译器以 `E1106:` resource type in a composite position 拒绝

#### Scenario: sum payload 被拒绝

- **WHEN** `type` 声明的变体携带 `FileHandle` payload
- **THEN** 编译器以 `E1106:` resource type in a composite position 拒绝

#### Scenario: tuple 元素被拒绝

- **WHEN** 函数声明 `(FileHandle, Int64)` 类型的参数
- **THEN** 编译器以 `E1106:` resource type in a composite position 拒绝

#### Scenario: newtype 底型被拒绝

- **WHEN** 出现 `newtype WrappedFile of FileHandle with Releasable`
- **THEN** 编译器以 `E1106:` resource type in a composite position 拒绝；newtype 包值，不包资源

#### Scenario: 泛型实例化被拒绝

- **WHEN** `Box<FileHandle>` 或 `id<FileHandle>` 作为类型表达式出现，后者 `id` 的参数仅在绑定位
- **THEN** 编译器以 `E1106:` resource type in a composite position 在实例化处拒绝；绑定位精化是预留修订

#### Scenario: Releasable 的 Dyn 盒被拒绝

- **WHEN** `f` 资源类型时出现 `Dyn<Releasable>(f)`，或 `Dyn<Releasable>` 作为注解的类型
- **THEN** 编译器以 `E1106:` resource type in a composite position 拒绝；盒共享句柄，纪律不共享

### Requirement: 资源诊断段位

资源章拥有注册表段位 `E1100`–`E1199`，在 `docs/spec/diagnostics.toml` `[segments]` 声明，owner 为 `1300-resources`。分配：`E1101` resource record must implement Releasable、`E1102` Releasable implemented by a non-resource type、`E1103` scope resource head does not implement Releasable、`E1104` resource binding outside its release discipline、`E1105` resource binding rebound、`E1106` resource type in a composite position。`E1100` 与 `E1107`–`E1199` 留作本章修订——需要段位码的后来章节认领自己的段位。

#### Scenario: 段位可检索

- **WHEN** 诊断消费者在注册表查任一 `E11xx` 码
- **THEN** 段位条目指名 owner `1300-resources`，每个已分配码的条目携带 severity、title、description、remediation、owner、requirement 与 allocated 日期

#### Scenario: 后续变更在段内扩展

- **WHEN** 本章后来的 spec 层变更需要新码
- **THEN** 它在自己的变更内从 `E1100`–`E1199` 分配；资源之外的章节认领别的段位

## 示例（非权威）

下面的示例只用第 1–13 章已批准的表面形式阐释上述 Requirements。它们是说明性的、非权威的：任何冲突以 Requirements 与 Scenarios 为准。标注诊断码的行是被拒绝的形式，展示编译器发出的码。进行中 scope 的 panic 展开标注为待错误机制章。

### Releasable 与其义务

```we
byres record FileHandle { fd: Int64 }    // a byres record: the category
                                         // carries the impl obligation
impl Releasable for FileHandle {
    fn release(mut self) { close(self.fd) }  // mut self, no return type
}

byres record LogFile { fd: Int64 }       // E1101: resource record must
                                         // implement Releasable — no impl
impl Releasable for User { ... }         // E1102: Releasable implemented
                                         // by a non-resource type
impl Releasable for FileHandle {
    fn release(mut self) -> Bool { ... } // E0808: signature mismatch
}
```

### scope resource 语句

```we
fn countLines(name: String) -> Int64 {
    scope resource(f = openFile(name)) { // head implements Releasable
        return f.read()                  // return pierces: releases run
    }                                    // once, before the defers of the
}                                        // enclosing function body run

fn two() {
    scope resource(a = openFile("a"), b = openFile("b")) {
        work(a)
        work(b)
    }                                    // at exit: b released, then a
}
```

### 释放纪律：三通道，函数局

```we
fn copy(src: FileHandle) -> FileHandle { // the parameter is a channel: the
    scope resource(s = src) {            // signature carries the obligation
        return makeOut()                 // return: caller takes the value
    }
}

fn useAndPass(f: FileHandle) {           // parameter: obligation starts here
    log(f.read())                        // reading is ordinary use
    sink(f)                              // argument pass: sink's parameter
}                                        // declares the type; f dies here

fn leaky() {
    let f = openFile("a")                // E1104: resource binding outside
    log(f.read())                        // its release discipline — the
}                                        // path ends with no transfer

let f = openFile("a")                    // E1104 at the top level: the
                                         // module top level binds no
                                         // resources
```

### 单触发点、无别名、只栖绑定位

```we
fn guarded(name: String) -> Int64 {
    scope resource(f = openFile(name)) {
        if empty(name) { return 0 }      // early exit is the early release
        return f.read()                  // no f.release() anywhere: a
    }                                    // direct call is E1104
}

fn aliases(f: FileHandle) {
    let g = f                            // E1105: resource binding rebound
    var h = openFile("a")                // E1105: resource bindings are
}                                        // let-shaped

record Conn { file: FileHandle }         // E1106: resource type in a
type Wrapped = Open(FileHandle)          // composite position — the field,
newtype WrappedFile of FileHandle with Releasable  // the payload, the under-
let boxed: Box<FileHandle> = pack(f)     // lying, the generic argument,
let d = Dyn<Releasable>(f)               // the Dyn box: never boxed; the
                                         // box shares handles
```

## 术语对照

本章关键术语，英中对译，以保翻译一致：

| English | 中文 |
| --- | --- |
| resource | 资源 |
| release | 释放 |
| release contract | 释放契约 |
| declaration completeness | 声明完备性 |
| scope resource statement | scope resource 语句 |
| head expression | 头表达式 |
| head binding | 头绑定 |
| block exit | 块出口 |
| pierce | 穿透 |
| scope-exit machinery | 块出口机制 |
| release discipline | 释放纪律 |
| linear | 线性 |
| function-local | 函数局 |
| transfer | 转移 |
| channel | 通道 |
| obligation | 义务 |
| use after transfer | 转移后使用 |
| release trigger | 释放触发点 |
| early release | 早释放 |
| double release | 双重释放 |
| aliasing | 别名 |
| binding position | 绑定位 |
| composite position | 组合位 |
| boxing | 装箱 |
