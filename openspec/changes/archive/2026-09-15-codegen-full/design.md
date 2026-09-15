# design — codegen-full（B1b）

> 状态：candidate 草案（勘查完成：代码面三轮 + 语料一轮 + 运行时两轮；待 tasks.md 与 change-review）

## 定位

B1b 是 B1 的另一半：把 codegen 的未实现集**归零**。验收句是黑盒的、机械可判的——**语料中不再有任何一枚黄金以 codegen 边界行停住**（今日 14 枚：`bndMainBody` × 12、`bndFnBody` × 1、`bndGenericFns` × 1）。

B1a 的 `design.md` 里对本变更的预测有三条，本设计逐条对账：

| B1a 的预测 | 本设计的处理 |
| --- | --- |
| D0「B1a 完成时停点集合恰为 `{bndGenericFns}`」 | **已被 B1a 实现推翻**（实测 5 词、14 枚锚定）。T13 裁定 1 把退役改挂本变更，本设计的 D0 给出终态。 |
| D6「5 枚惰性组合子（map/filter/take/skip/collect）→ B1b」 | **家族划分写错**：规范钉死惰性**四**枚，`collect` 是急性第 7 枚（§D8）。 |
| D4:77「Dyn 接收者与泛型方法 = B1b（vtable/单态化）」、D12:177「`bndGenericFns` 括注改 B1b codegen-poly」 | 兑现。**变更名以 roadmap 为准：`codegen-full`**（B1a 工件里的 `codegen-poly` 是旧名，已在 roadmap 上被取代）。 |

---

## D0 停点词表的终态与两塔持有

**决策**：本变更不把「词表为空」当验收句——词表是**内部里程碑词汇**，它的产出点数（`e.bnd()` 226 处 + `bndGeneric()` 32 处）与语料锚定数是两件事。验收句取后者：**14 枚锚定黄金全部翻绿**。词表随后逐面清退：某词的全部产出点随对应能力落地而消失时，该词与 `codegen.go:28-57` 的历史注释一并退休。

**两塔持有（B1a 归档期的归因缺陷，本变更更正）**：`bndGenericFns` 这个**词面**被两座塔持有——

- **codegen 塔**：`codegen.go:60` 的常量，4 处真拒绝（`collectImpl:589`/`:599`、模块收集 `:1117`/`:1142`）+ 27 处 assertEqual Eq 面残行复用。黄金锚：`test-fn-generic-bnd`。
- **typecheck 塔**：`typecheck.go:57` 的常量，`:2776` 的 `assertEqualCall` 在 `!inEqDomain(got)` 时产出。黄金锚：`check-assertequal-residual-bnd`。`internal/cli/check.go` **不 import codegen**，故这枚不是 codegen 停点。

B1a 的 T13 只 grep 了 codegen 侧产出点，把后者记成了 codegen 停点。本变更的零集按塔划分：**codegen 14 枚**，typecheck 那枚归其本塔（Eq 域闭包的裁定，与本变更无关）。

**边界**：本变更**不改** `check-assertequal-residual-bnd` 的期望字节，也不动 typecheck 侧的词常量。两塔共享一个词面的现状在 `codegen.go` 的历史注释里记明（一行），免得下一次对账再错一次。

---

## D1 检查器注记面（Q2）——本变更的地基

**问题**：codegen 要把 `fn id<T>(x: T) -> T` 在 `id(5)` 处实例化，必须知道**检查器推出来的** `T = Int64`。检查器的推断是逐调用点的（`inferValueArgs:7477` 从实参单向上行、`unifyInto:7382`、方法调用还有 `substMap` 的逐参线程 `methodCall:7643`），而 `Check()` 返回即弃。codegen 若自行复刻这套推断，就是同一事实的第二处权威——违反「一类事实只有一个权威位置」。

**决策**：`Check()`/`CheckTestRoot()`/`CheckProject()` 的出口扩形为交出一个**只读的实例化登记**，键是 AST 节点指针（两侧走的是同一棵 AST），值是**类型形（Shape）**——一个**封闭的小结构**，不是渲染串：

```
Shape = Param(pos)            // 未定 / 声明侧的类型参数位
      | Base(name)            // Int64、Bool、String…
      | Nominal(decl, []Shape) // 记录/和式/新类型/接口/List/Map/Set/Option/Result…
      | Dyn(ifaceDecl, []Shape)
      | Fn([]Shape, []Shape)
      | Tuple([]Shape)
      | Unit | Never
```

登记面按站点分：`fn` 调用的类型实参绑定、`Construct` 的类型实参绑定、方法调用的类型实参绑定、`Dyn<I>(v)` 构造点的**被装箱具体型**、泛型 `where` 的实例化见证（若 codegen 需要）。站点以 `ast.Node` 为键。

另有**声明面条目**（D6 决策 2 的发现）：每个接口声明的**非默认方法槽序**（名字 + 参数/返回形）。`Iterator<T>` 的方法集不在任何源文件里（std 面由检查器内建注册，`typecheck.go:1226-1306`），且「哪些方法有默认体」是检查器独有的知识——vtable 的槽位与槽序只能由这里得到。

**实现期补记（T1 开工时露出，D1 的第一个真问题）——名义构造子的「声明标识」怎么给**：

*事实*：`Type` 的具体节点**只带名字、不带声明节点**——`sumInfo:388-395`、`recordInfo:410-418`、`newtypeInfo`、`ifaceInfo:460-465` 全都没有 AST 指针；符号表的 `symbol:1674-1686` 同样只带 `*sumInfo`/`*recordInfo`/`*newtypeInfo`/`*ifaceInfo`（类型声明不给节点，只有 `fn *ast.FnDecl` 留了节点）。而**内建面是包级条目、根本没有源文件**：`resultSum`/`optionSum`/`listSum`/`mapSum`/`setSum`/`rangeSum`（`:1017-1053`）与 `iteratorIface`/`iterableIface`（`:1088-1091`）。

*决策*：`Shape` 的名义构造子（`Nominal`/`Dyn`）携带**声明标识**，按来源二分——

1. **用户声明 → AST 声明节点**（`ast.Node`）。检查器生成登记面时**手里就有它**（`:2049`/`:2054`/`:2061` 三处的 `x` 即声明节点），登记面是一张**旁表**：节点不进类型、不进 AST。**这不是 D1 所拒的「AST 注记」**——注记指把类型写回 AST 节点（`ast.go:8-10` 的 pure-data 性质），旁表的两头都在检查器与登记面内，AST 逐字节不动（T1 的负断言仍成立）。
2. **内建声明 → 内建标记**（名字 + 类别）。内建面是**全局单例**，跨模块同名碰撞在结构上不可能，故此处用名字是安全的。**D1 的渲染串禁令针对的是用户声明的非限定名**（两个模块各有一个 `Wrap<T>` 会在渲染串上相撞），内建面不在其列——这条区分是本补记的核心，别把它读成禁令放宽。

*顺带钉死*：`*sumInfo` 等指针本身也是碰撞自由的（每个模块的同名声明各有自己的指针），但它们是**未导出型**，不能直接出口；登记面把「指针 → 标识」的映射一次做掉，codegen 只认标识。

**实现期补记二（T1 落地时露出，三处对 D1 记法的订正）**：

1. **`Fn([]Shape, []Shape)` → `Fn(Params []Shape, Ret *Shape)`**。两处原因：Go 禁止直接递归的结构字段（`Ret Shape` 编译不过，须为指针）；更实质的是**「一个 fn 恰有一个返回」是语言的规则**（第 6 章），而二元 `[]Shape` 的记法暗示多返回，且**分不出「无返回值」与「返回 unit」**——两者对 vtable 槽与调用约定是不同的东西。`Ret == nil` 即「声明无返回」，与槽条的同一读法一致（`Slot.Ret` 同）。
2. **八臂之外补第九臂 `Assoc`**。八臂枚举出自设计期；实现时 `Iterable<T>` 的方法面立刻用上了它表达不了的东西：`fn iterator(self) -> Iter`，返回的是**关联类型位**（`assocRef:249`）。故 `Shape` 是**九臂**：`Param | Base | Nominal | Dyn | Fn | Tuple | Unit | Never | Assoc`。`Assoc` 只出现在声明面槽条里（未解析即未解析，诚实投影），带接口标识与关联类型名。**这不是面的扩大**：它是 D1 自己那条「声明面槽序」要求的直接后果。
3. **方法调用站点的 `Args` 只列「本次调用自己定的」位置，`Params`/`Ret` 带已代入接收者的视图**。方法视图经 `substFn(m.fn, t.args, nil)` 到达 `methodCall` 时，接收者持有的位置**已经代入**——它们不是这次调用的绑定。设计期把站点记法写成 `Fn` 形的二元组，默认了「站点 = 一次完整应用」；实际三类站点（fn 调用 / 构造 / 方法调用）形不同，故 `Site` 记成 `Args []Arg`（`Arg{Pos, Shape}`，位置是**被调方自己的子句**）+ `Params []Shape` + `Ret Shape`，仅盒站点带 `Boxed *Shape`。`Args` 为空的站点 = 应用了泛型视图但没定任何位置。

**顺带钉死的两件**：

- **声明标识「恰好一个」**：用户声明**只带节点、不带名字**（`ShapeDecl.Name` 为空串），内建声明**只带名字、不带节点**。名字不出口是渲染串禁令的直接落地——两个模块的 `Wrap<T>` 节点不同、名字相同，若名字也在场，调用方仍可能误用；不在场则误用在结构上不可能。
- **登记表在 `checkBuiltinCombinators` 期间摘下**：那趟走的是 stdlib 自己的默认体（字面量构造的合成节点，不属于任何调用方持有的树），其站点无人能查到；留着只会污染 T2 的实例化表。这是对**新面**划界，不动检查器既有行为（`git diff docs/spec/diagnostics.toml` 为空、conformance 810 枚零漂移即证据）。

**未决（本决策末）已定**：登记表**按 AST 指针查**（`Shapes.At(node)`），与模块遍历次序天然解耦——不依赖 codegen 的模块收集与检查器同序。

**未登记的站点（明记）**：内建构造子 `Ok`/`Err`/`Some`/`None`（`builtinCtorCall`）不登记——它们不定用户声明的任何位置，其载荷型由期望型决定，codegen 从自己走的树即可读到；泛型 `where` 的见证不登记——D4 把 `where` 整体排除在 codegen 之外。

**为什么不导出渲染串**：`typecheck.Type.String()` 已经会渲染成 `Wrap<Point>`（`typecheck.go:193-198`），看上去可以拿来名字改编。但 `decl.name` 是**非限定名**——两个模块各有一个 `Wrap<T>` 时渲染串相撞，符号表就崩。名字既然是 codegen 的权威（`fnDef.sym():737`、方法表 `:601`），改编也必须留在 codegen：把 `Shape` 交给 codegen，由它用自己的 `headKey`/`recvKey` 机制映射成限定键。**导出串 = 把命名权挪到 typecheck**，是分层污染，拒绝。

**为什么不导出 `Type` 本身**：`Type` 是 `interface{ String() string }`（`:135`），具体节点（`namedType` 的 `decl`/`args`）全私有。要么把内里全公开（大面积出口），要么给一个封闭的 `Shape` 视图（本决策）。后者出口最小、意图最明确，且**新增面可以独立演化**——`Shape` 的存在不约束检查器内部怎么改。

**边界**：本变更**不重构 checker**，不加 AST 注记（`ast.go:8-10` 的 pure-data 性质保留），不新增诊断码。登记表是 `Check*()` 的一个**附加返回值**，既有调用方（测试、CLI、LSP）按需忽略。

**未决（实现期定）**：登记是「一次 `Check()` 后按站点查」还是「按模块增量取」——取决于 codegen 的模块遍历是否与检查器同序（B1a 的模块收集在 `codegen.go:1108-1127`）。若不同序，登记表须与检查器的模块遍历解耦（按 AST 指针查即可自然解耦）。

---

## D2 实例化表与名字改编

**决策**：新增 `{声明键, 类型实参 Shape[]} → 符号` 的实例化表。符号在既有形上**追加一段**：

```
fn 定义：      <module>.<name>              →  <module>.<name>$<arg>.<arg>…
方法定义：      <recvKey>.<name>             →  <recvKey>.<name>$<arg>.<arg>…
记录/和式/新类型：<module>.<Name>            →  <module>.<Name>$<arg>.<arg>…
```

**参数编码**（只用 LLVM 裸标识符允许的字符——既有符号是**不加引号**直接拼进 `define … @%s(...)` 的，见 `codegen.go:4120`/`:4191`）：`Int64`→`Int64`、`String`→`String`、`Bool`→`Bool`、`Float64`→`Float64`、`List<Int64>`→`List$Int64`、`Wrap<Point>`→`Wrap$Point`、嵌套 → 递归 `$` 拼接、元组 → `T$<n>$<elems>`、fn 型 → `F$<params>_<rets>`、`Unit`→`U`、`Never`→`N`。所有段以 `$` 分隔——**该字符在 We 标识符里不出现，故不与任何声明名相撞**。

**为什么不用渲染串/hash**：渲染串相撞（D1 已述）；hash 让 IR 不可读，而本仓的调试面全靠读 IR（`we build --emit-llvm` 的黄金面）。可读的改编串让「这是哪个实例化」在 IR 里一眼可见。

**边界**：B1a 的既有符号零改动——无类型实参时后缀为空，符号与今天逐字节相同，既有黄金零触碰。

**实现期补记三（T2 落地时露出，四处对 D2 记法的订正与补全）**：

1. **参数里的用户声明编成模块限定键，不是裸名**。D2 的编码表把 `Wrap<Point>` 写作 `Wrap$Point`。裸名正是 D1 拒绝渲染串的那条理由原样搬到参数位上：模块 `a` 与 `b` 各有一个 `Point`，同一份声明 `Wrap<T>` 应用在两者上会编出同一个参数串，符号表把两个类型并成一个——静默误编。故实作取 codegen 自己的限定键：`a.Wrap$a.Point`。**内建仍用裸名**（`List$Int64`）——它们是包级单例，裸名在此无碰撞，正是 D1 补记一刻下的那条线。**这是本补记唯一与任务书字面例子不符之处**；钉它的测试是 T2 第三条负断言：同一个 `Wrap` 应用在两个模块各自的 `Point` 上，改编结果必须不同。
2. **`.` 是元记法，不是字面分隔符**。D2 第 95–97 行的 `<arg>.<arg>…` 若读作字面分隔，会与限定键内部的 `.` 撞车；同一段末句「所有段以 `$` 分隔」才是规范句。实作：参数列表一律 `$` 连接（`main.id$Int64$a.Point`），`.` 只出现在限定键内部。
3. **D2 表未列的三臂**。① `Dyn<I>` → `Dyn$<faceKey>[$<arg>…]`：盒是自成一体的类型（第 10 章），`Dyn<I>` 与 `I` 不同型，面必须标记。② 无返回的 fn 型 → `F$<params>_V`：`U` 是 unit 的编码，而「无返回」与「返回 unit」在登记面上是两个不同的形（D1 补记二第 1 条的同一理由），不能编成同一个串——**注意这条的语料面**：第 7 章把 fn 型的返回位写作必写、无值位写 `()`，故 `_V` 只在检查器把**无返回的声明**投影成 fn 型时才到达（`fnType.ret == nil` 那一支）；改编器对两臂都保持全函数，且绝不把两者并成一个串。③ `ShapeParam`/`ShapeAssoc` **不编码，panic**：站点交给改编器的实参必须已解，带未解位置的应用根本不构成一次实例化——登记它是 D3 驱动层的错，不是可编的形。这条 panic 是契约边界，同时被「以真检查器输出喂改编器」的单测（`TestMangleCheckerShapes`）压在既有的站点集上：跑得通即证这批量里没有未解位置漏到参数位。
4. **表的键就是符号本身**。`Shape` 带切片，不能做 map 键；而符号即「声明键 + 实参串」的双射，故表以符号为键——D2 的 `{声明键, 实参} → 符号` 落到实作上就是这一个串。表在重复登记时另以 `Shape.Equal` 复核实参：符号相同而实参不同的那一刻是改编器自身的缺陷，当场 panic，而不是把两个类型发成一个 `define`。

**顺带钉死的两件**：

- **声明索引建在模块表之上，早于 pass one 的走查**（`declIndex`，mangle.go）：pass one 走到泛型边界即停，而实例化要命名的**正是**泛型声明，故索引不能跟着走查建。索引也走 `foreign` 块内的不透明记录——它不发自己的符号，但可以出现在类型实参里。
- **`fnDef.suffix` 是符号的唯一出口**：`sym()` 从「键 + 名」变为「键 + 名 + 后缀」，空后缀即今日字节。B1a 的既有符号因此不是「没走新路」，而是**走了新路且退化态逐字节相同**——`sym()` 是每条 `define`/`call` 的必经之路（14 处调用点），整个既有语料就是这条零漂移的见证（单点突变复验：给 `sym()` 恒定追加 `$`，`internal/codegen` 当场 42 处失败）。

**T2 的诚实边界**：本任务只落命名层，**不改任何发射路径**——没有站点被驱动，故没有实例化符号进 IR。任务书第 2 条要的「`we build` 端到端 exit 0」在 T2 阶段的可达形是两件：① 既有 clang 端到端面（`internal/cli` 的 build 测试 + conformance 810 枚）全绿；② 新增一枚把**改编器的真实输出**拼进最小 IR、过 clang 21.1.8 编译链接并执行取数的单测（`TestMangledSymbolsLinkThroughClang`，无 clang 时 skip）——它验的是字符集合法的**直接**证据（无引号符号可汇编、可链接、可运行），而不是「端到端跑过一次实例化」。真正的实例化端到端随 T3/T4 的发射一起到。

---

## D3 逐实例化发射的驱动与次序

**事实（须在设计里记住）**：main 体在 `codegen.go:1266-1289` 发射，**早于** fn 定义段 `:1291-1295`。今天 12 枚黄金报 `bndMainBody` 而 `build-bnd-prim-return` 报 `bndFnBody`，就是这条次序的直接后果。

**决策**：实例化采用**惰性工作表**——发射中遇到一个未见过的 `{声明, 实参}` 就登记并生成定义，已见的复用符号。定义文本进 `fnsDone`（`codegen.go:490`），与今天的 fn 定义段同列，顺序取**登记次序**（确定性的，B1a 的 `methodsOrd` 同法）。**递归实例化**（`fn f<T>(x: Box<T>)` 里再调 `g<T>`）由工作表自然处理：只要还有未发射的登记项就继续，直到不动点。

**边界**：不动点循环必须有**上界保护**——类型实参是有限的闭集，但推断可能产出无限深的应用（`f<Box<T>>` 调 `f<T>`…）。守卫：实例化表的键比较用结构等价；若某次登记的类型实参**深度超过声明的类型参数深度 + 常数**，报实现内部错误而非无限循环。**这条守卫是硬要求**，实现期必须落一枚负例黄金或单测。

**实现期补记四（T3 落地时露出，五处对 D3 的订正、补全与披露）**：

1. **驱动的来源面被切了一刀：本任务只驱动「写出来的」类型实参**（`ast.Construct.TypeArgs` / `ast.Call.TypeArgs`），**不驱动检查器推断出来的**那一半。理由不是省事，是次序：D1 的 `Shapes` 还没有接进 codegen（那是 T4 的管道活），而**发射器不重新推断**是 D1/D4 的同一条纪律（一类事实只有一个权威）。后果是**可预期的且必须记住**：一个**推断**出来的应用（`g(x)` 里 x 是 `Box<Int64>`）从**使用点**停在 `bndGeneric()`，故 `test-fn-generic-bnd`（`bndGenericFns` 的唯一黄金锚）**在本任务逐字节不变**——这正是 T4 要翻的那一枚。**本任务的端到端可达形是「显式写类型实参」的程序**：`record Box<T>` + `Box<Int64>{v:7}` + `"${b.v}"` 过 `we build` exit 0、打印 `7`、IR 里出 `%struct.main.Box$Int64 = type { i64 }`（真机取证）。
2. **合成，不是就地改写**。实作**从不改写源树**——源节点被同一份声明的所有实例化共享，就地替换 `T`→`Int64` 会把另一个实例化的读法一起改掉。取而代之：每个实例化**合成一份具体声明**（`Box$Int64` 作为一枚 `ast.RecordDecl` 进 `e.records`/`e.order`/`e.declKeys`，sum 走克隆、newtype 走代换后的底层），于是 `layout`/`emitConstruct`/`fieldSlotOf`/`render` 全都把它**当作写出来的声明读**——下游一行没改。代换本身是**惰性**的：`e.instEnv` 在读到一个类型引用解析时才被读，产物引用记在 `e.subst`/`e.substShape` 里（**键随节点走**）；实例化之外 `e.instEnv` 为 nil，代换是恒等，单态路径读的仍是它一直读的那几张表。
3. **不动点是两处，不是一处**。① fn 定义段：`for i := 0; i < len(e.fns); i++` 边走边登记，新实例化的 fn 落到表尾即进下一轮，顺序天然是**登记次序**（D3 要的确定性）。② `render` 的 `e.order` 走查：`layout` 会解析它走到的字段类型，而解析一个应用**当场合成并登记**那个实例化——一枚记录在走查途中入册，故这条走查同样要排到不动点，理由与 ① 一字不差（字段里够得到的实例化就是被用到的，欠它一条类型行）。**这条是实作才露出的**：只做 ① 时，经字段间接到达的实例化缺类型行。
4. **键的身份纪律：`namedKey`**。一个产物引用的**键必须随节点走**（`map[*ast.NamedType]string`），不能到时候重算——重算就是把 D1 拒绝渲染串的那条理由换个位置再犯一次。取键的唯一出口是 `namedKey(modKey, n)`：记着就用记着的，没记着才退化成 `modKey+"."+n.Name`。**踩过的坑记在这儿**：`shapeKey` 起初把**声明键**（`main.Box`）交给 `instDecl`，而 `instRecord` 登记的是**改编后的应用键**（`main.Box$Int64`）——同一个声明两个键，`layout` 里查表落空当场空指针。**一个应用只有一个键**，声明键只是它的退化态。
5. **守卫落在两处，而且第二处不是冗余**。`instDecl` 与 `instFn` 都调 `instDepthCheck(params int, key string, args []Shape)`，界是 `参数个数 + instDepthLimit(32)`。`instFn` 那一处是实作逼出来的：**一个在加深自己应用却不经过任何声明登记位的实参**（元组被代换逐层裹粗）**每一轮都登记一个新符号**，下游永远遇不到同一个名字第二次——`instDecl` 的守卫结构上够不到它。**两条负例各钉一处**：`fn f<T>() { f<Box<T>>() }`（过声明位，任务书点名的形）与 `fn f<T>() { f<(T, Int64)>() }`（元组形，只经 `instFn`）。**fixture 的形状是有理由的**：两例都写成**无参、返回 unit、体是单条 ExprStmt 调用**——先前写成 `f<Box<T>>(x)` 时停在 `bndFn`，原因是实参对不上被调方的 ABI（**一个类型不正确的 fixture 被正确拒绝**，不是守卫没生效）。**突变复验**：把 `instDepthCheck` 体挖空（单点突变、突变前后各断言锚点 `count==1`），自我加深那枚单测**挂死**（`panic: test timed out after 20s`，栈底 `emitCall→emitFnDefine→EmitProgram`），而同包其余 339 枚仍绿（`ok 0.206s`）——**守卫是承重的，且它的测试就是那几枚敏感的**。
6. **顺带修的一处**：`fitAbi` 的返回位原先记**声明上的名字**（实例化体内那是一个类型参数名 `T`），调用方按它绑结果就没有域；`let n = g<Int64>(5)` 因此在 `"${n}"` 处停在 `bndMainBody`（栈底 `emitHole → valueKind(ident n) == skNone`）。改为记**解析后代换过的名字**（`baseTypeName(e.derefNewtype(e.resolveRef(t)))`）——实例化体内声明的拼法是参数，调用方绑的是实参。

**T3 的诚实边界（三条，逐条记入完成记录）**：

- **检查器不解析显式类型实参位上的类型参数**：`fn f<T>(b: Box<T>) -> T { return g<T>(b) }` 在 `g<T>` 的类型实参位报 `E1304: unresolved name — "T" is held by no scope`（`resolveArgs`/`resolveTypeRef` 看不到类型参数）。故任务书点名的递归形 `fn f<T>(x: Box<T>)` 内调 `g<T>` **只在 AST 层单测**（`TestRecursiveInstantiationRegistersInOrder`：注册次序 + 定义次序 + 槽符号断言），端到端见证换成**检查器合法的传递形**（单态 `f` 调 `g<Int64>`，真机打印 `7`）。**这是 T4 的面，本任务不修**——修它就是替检查器做推断，正是 D1 不许的。
- **`io.println(<字段读>)` 之类的标量实参停在既有的 `argIsScalar`**（只探 Literal/Ident/Binary）：与 T3 无关，单测 fixture 用插值洞（`interp(memberOf(…))`）绕开，**未修、也不冒充 T3 的改动**。
- **零黄金改动，本任务不新增黄金**：14 枚边界黄金（含 `test-fn-generic-bnd`）逐字节不变是**要求**而非巧合（推断面没被驱动）；拓宽面的黄金是 T4 的交付物。任务书 T3 两条验证的可达形因此是「单测层满足、黄金层留待 T4」——逐条已在 `tasks.md` 的勾选里写明。

**确定性的两层证据**：单测层 8 次重复发射的 IR **逐字节相同**（`TestInstantiationEmissionIsDeterministic`）；CLI 层两次独立 `we build` 产物 SHA256 相同（`6c92d0e3…1400ac`）。

**实现期补记五（T4 落地时露出，八处对 D3/D0/D9/D10/D11 的订正、补全与披露）**：

1. **驱动的第二半：检查器的登记面（D1 的 `Sites`）接进 codegen**——补记四第 1 条预告的那一半。T3 只驱动**写出来**的类型实参；T4 把登记面顺着 `EmitProgram` 的形参接到 `instCallee`/`instMethod`/`instFn`：先读写出的实参，缺失时读该站点定下的形（`siteArgs`）。于是**推断**出来的应用（`id(5)` 里那个 `Int64`）也被驱动，13 簇里凡是卡在「应用没被驱动」的形一并翻绿。**锚**：`test-fn-generic-bnd`（`bndGenericFns` 的唯一黄金锚）HEAD `0030d60` 的 `we test .` → exit 70 + 边界行；本树 → exit 0 且 stdout 逐字节为 `pass  tests/m_test.we: generic (0ms)\ntotal 1, passed 1, failed 0 (0ms)`。**站点面不覆盖方法调用**（检查器不为「泛型接收者上的方法调用」记站点），故泛型方法一律**由接收者驱动**——那是 T3 就有的路，本任务让它与登记面并存而不是取而代之。

2. **「4 处泛型真拒绝」的归属订正**：任务书写「`codegen.go` 的 4 处泛型真拒绝（`collectImpl:589`/`:599`、模块收集 `:1117`/`:1142`）放行后……」。按 `0030d60` 逐行核对：模块收集那两处（`FnDecl` 与 `NewtypeDecl` 的 `len(d.TypeParams) != 0 → bndGeneric()`）**在 T3 就放行了**——不放行则没有任何泛型声明进得了表，T3 的「写实参」端到端也无从谈起。本任务放行的是 `collectImpl` 的两处：泛型 impl 子句（`impl<T> Box<T>` 进 `implTmpl`）与泛型方法（进 `genericMethods`）。本任务的拒绝面因此是「**2 新放 + 2 已放**」，订正不静默。

3. **拒绝面是收窄，不是删除**：`collectImpl` 的头部判据原为 `!ok || head.Qual != "" || len(head.Args) != 0` 三者共用 `e.bnd()`。拆成两条：非名义头仍是 `e.bnd()`；实参列表非法改走新的 `paramsInOrder` → `bndGeneric()`。理由是**词要报对**——`impl Box<Int64>` 拒绝的是**泛型声明的形**，报成 main 体的停点只是「收集跑在哪个上下文」的偶然，不是判断。**负断言三面**：① 非名义头（限定名／元组／unit 三形，`TestNonNominalImplHeadStops`，逐形断言 `bndMainBody`；检查器侧先由 E0811 拒，已有 `check-e0811-tuple-head`/`-base-head` 两枚黄金锚）；② 重复 impl；③ `impl` 头带实参的非法形（`TestImplHeadArgumentsMustBeTheParameters` 三子例，逐例断言 `bndGenericFns`）。

4. **`E0807` 是一处误引**（任务书与码内注释同误）：重复 impl 的诊断码是 **E0809**（`typecheck.go:3500`/`:3504`「duplicate impl of one interface for one type」），成员重名是 **E0814**（`addMethod:896`），E0807 是接口**完整性**（缺方法）。三者都已有黄金锚（`check-e0809-duplicate`/`-overlap`、四枚 `check-e0814-*`、`check-e0807-missing-method`）。本任务把 `collectImpl` 里那句「a duplicate is the check stage's (E0807)」改成两个真码——码内不留下误引，任务书那一句的订正留在完成记录。

5. **分类器上的两个读：一枚留（有红锚），一枚删（零可达）**——两者都在 `callStrKind`，属 D9 的「分类器补面」家族；本任务只保留了**先红后绿有锚**的那一枚。① **ident 分支**（泛型 fn 在**值串位**被调用时，向 `instCallee` 问一次它的实例化返回面）：**保留**，锚 = 新增黄金 `run-ch10-generic-application-in-interp`（`"${identity<Int64>(3)}"`：HEAD 70 → 本树 0）。**突变复验**：把该判据单点中性化（`is && fd != nil` → `false && …`；突变前锚点 `count==1`）→ 该黄金转红（exit 70 + `bndMainBody`），而 `check-ch10-*`/`run-ch10-*`/`test-fn-generic-bnd` **全组其余逐枚仍绿**——承重、敏感，且只对这一面承重。② **method 分支**（接收者驱动的同一个读，泛型方法在值串位）：**删除**——真机探针面（`/tmp/we-b1b/probes{,2,3,4}` 31 形）、810 枚黄金、codegen 单测三层下零可达，删前删后三层逐字节一致。**留下的理由只有一条**：它有一枚红先行的锚。

6. **两处新缺口（披露，本任务不修）**：
   - **插值洞的内容不被类型化**（B1a 的 N3／follow-up #23 的根因，本任务以 `grep -rn "Holes" internal/typecheck/*.go` **零命中**机械证实）：洞内一个**推断**的泛型应用**既没有站点、也没有写出的实参**，`instCallee` 无从判定 → `valueKind` 答 `skNone` → `emitHole` 停。**写实参的拼法**由 5① 关掉；**推断的拼法修不动**——它要检查器先把洞内容类型化，那是另一个变更的裁定面。**对照组**：泛型**方法**在已实例化接收者上的洞内调用跑得通（接收者驱动不依赖洞的站点）。
   - **泛型 sum 的构造子面**：`Some2<Int64>(2)` 这种**在构造子上写实参**的拼法停，`let a = Some2(2)` 的推断拼法同样停。根因：泛型 sum **不在 `e.sums`/`e.sumsOrd`**（它作为模板进 `e.generics`），而 `variantSite` 只查 prelude 与 `localVariantShapes`（`e.sumsOrd` 的扫描）——**构造子自己写的实参从未被读**，一个泛型 sum 的变体只有被**别处**（一个类型标注）实例化过才可达。故孪生用带标注的拼法（`let a: Opt2<Int64> = Some2(2)`，先红后绿）。构造子拼法登记为**候选 follow-up**（它要的是「构造子位的实参驱动」，与 T10 的分类器补面同族）。
   - **旁记一条易混的形**：语句位的 match 臂体**必须是块**（`Some2(v) => io.println(…)` 停在 `emitMatchBody`，加花括号即跑通）。这是既有的臂体规则、与泛型无关，孪生写作时踩到过——记在这儿，免得下一个人当成泛型缺口去查。

7. **孪生普查（D10-3 的对账表）**：`check-ch10-*` 17 枚 + `check-m6a-*` 4 枚 = **21 枚**。**补孪生 9 枚**、**另加 1 枚洞面锚**（`run-ch10-generic-application-in-interp`，不是任何 check 夹具的孪生，属 5① 的锚）：10 枚新黄金**逐枚先红后绿**，红态一律 exit 70，**词面有两形**——7 枚 `bndGenericFns` / 3 枚 `bndMainBody`（后者的差是「声明已进表、使用点仍等登记」）。**未补 12 枚，逐条列因**（任务书 ③ 要求的披露清单）：
   - `check-ch10-associated-type-green`：impl 体停在 `bndFnBody`（`self.value` 的返回面是关联型名 `Item`）——关联型解析不在本任务面。
   - `check-ch10-dyn-green`：`Dyn<I>` 盒面，T5/T6 的活。
   - `check-ch10-derives-green`：`derives Eq, Hash, Show` 生成的方法没有运行期路径（`equals`/`hash`/`toDebugString` 停在 `bndMainBody`）——属 derive 发射面，与泛型无关。
   - `check-ch10-if-comparison-green`、`-default-method-green`、`-interface-impl-green`、`-pub-interface-green`、`-inherent-mut-green`：**这 5 枚在 HEAD `0030d60` 就是 build 0 / run 0**（B1a 的单态面），被**红先行规则自证排除**——新黄金在 HEAD 必须红，而这 5 枚的 run 期望在 HEAD 即绿。其中 default-method／interface-impl／inherent-mut 三面已由 B1a 的 `run-method-dispatch` 逐形覆盖（`bump(mut self)` 的固有方法、`greetLoud` 的默认体、`impl Greeter for User`），`pub-interface` 面由 `run-ch10-where` 的 `impl Describable for User` 覆盖。
   - 四枚 `check-m6a-*`：三枚是**纯诊断面**（check exit 1：`cross-clause-collision` E0501、`generic-map-override-mismatch` E0808、`impl-where-instantiation` E0830），按「纯诊断面不补」不补；第四枚 `precise-map-override` check 绿但**没有 `main`**（覆写体是 `todo(…)`），不可运行。

8. **文档面：泛型函数这一面从「What remains closed」清单里关闭**（`docs/benchmarks.md` + `.zh.md` 双语各一条，措辞互为对照）。D11 把清单的**清空**留给 T14，但「哪一簇关掉哪一面、该面随之从清单里下线」是 T9 那一行的既定做法（「`docs/benchmarks.md` 的 N1 面随之关闭」），故本任务一并关掉泛型函数这一面；清单其余五面（`scope` 值形、main 体的 `?`、元组值绑定读 `String` 位、fn 值的值串/操作数位、内置 `Option` 载荷读 `String` 位）**逐字不动**。

---

## D4 `where` 子句在 codegen 的位置

**事实**：codegen 全文件对 `.Where`/`WhereBound`/`TypeEq` **零命中**——`where` 从未进入发射面。

**决策**：**保持零命中**。`where` 的职责在检查器：`checkWhereDecl`（`typecheck.go:3901`）逐声明校验、`checkWhereSatisfies`（`:4010`）逐应用校验（`E0830`）、有界体内的成员集由 `fnBounds`（`:6993-7000`）恢复。发射面只关心**实参绑定**——那个由 D1 的登记面给出。`where` 的等式（`TypeEq`）同理：`rebaseClause:703` 已在检查器侧把关联型解到位，`Shape` 交出来的就是解好的形。

**理由**：这是「一类事实只有一个权威」again——`where` 的判定已经有一个权威（检查器），codegen 再判一次只会制造两套可能分歧的规则。**若实现期发现某个形确实需要 codegen 侧读 `where`（例如为某实例化挑选不同的 ABI），那是一个新发现，须在此处补记并说明为什么 `Shape` 不足以表达。**

---

## D5 Dyn 盒的布局与 gc 集成

**事实（运行时对象模型，`runtime/c/gc.c` 为准）**：对象头冻结为 `{map@0, size@8}`、载荷自 16 起（`gc.c:5-16`）；map 字指向编译期布局描述符——每 64 个载荷槽一个位图字、位 *i* 覆盖偏移 `16+8*i`，字 0 的**位 0 是标记位**（描述符至少 8 对齐，`blk_map` 只掩位 0，`gc.c:27`）。描记循环 `gc.c:171-187`：全零/空 map 字 = **无描记内容，整个块跳过**；空子字跳过。`__we_alloc` `gc.c:252-310` 只写 size 字并清零，**map 字由调用方写**；根协议是影子栈（无保守扫描）。编译器能用的分配入点只有两条：直接 `call ptr @__we_alloc(i64 N)` + 自写 map 字，或一个运行期构造器——`malloc` 从不暴露给生成码，凡 `malloc` 的字节都在 gc 域外、**永不入影子栈**。

**决策 1——盒形**：`{map@0, size@8, vtbl@16, 载荷@24…}`，`size = 24 + 8*n`，经 `__we_alloc` 分配。vtbl 字**不描**——与 fn 载体把代码指针放在 `16` 且 `@.fnmap` 只置位 1（`[1 x i64] [i64 2]`，`fnMapName:4246-4252`）是同一姿势：头外一字「代码/表指针」，其后是 gc 载荷。

**决策 2——盒内没有类型标签**，只有 vtbl 字。We 无下转型、无反射、无子类型（`docs/spec/1000-interfaces.md` §No variance, no subtyping），盒的静态类型在**每个**站点都是已知的（D1），标签没有消费者。为不存在的语言特性留一个字是纯开销。vtbl 字是唯一的「类型证据」，且只用于找方法代码。

**决策 3——载荷按携带面入盒，不深拷贝**：载荷字由具体型的携带面（D1 的 `Shape` 映射到 codegen 既有面词汇：`strKind` + `rec`/`gc`）逐字排布。gc 句柄**按句柄存**（规范：盒是 gc 值、共享不复制）；scalar 一字直存；String 两字直存且**不置描记位**（缓冲在 gc 域外，与 `layout():9583` 的 `fkStr` 同法）；元组按元素面展开。

**决策 4——描述符恒为编译期常量**：装箱点的具体型静态已知（规范：`Dyn<I>(expr)` 是唯一构造形、装箱点零推断、不实现者 E0818），故盒的描述符像记录一样由发射器直接产出 `@.dynmap<n>`（`fnMapName:4246` 同法按面去重），**不需要 `list.c:34-77` 的运行期合成**。载荷无 gc 引用时 map 字写 **NULL**（`gc.c:174-176` 跳过内容），同 `list.c:63-65` 的标量姿态。载荷恰一字且为 gc 引用时描述符与 `@.fnmap` **同形**（`[i64 2]`）——不是巧合，两者都是「头外一字表指针 + 一字 gc 载荷」。

**决策 5——入盒协议与 `allocRecord:9680` 逐条同法**：alloc → `store ptr @.dynmap<n>, ptr %box` → `__we_root_push(%box)` → 存 vtbl 字 → 存载荷字；`e.pushes++` 与 `dischargeRoots:4578` 同账。alloc 与 push 之间无分配，故载荷寄存器不因收集而失效（`allocRecord` 的既有理由）。

**决策 6——盒的 ABI 是 `abiGc`（一个字）**：盒就是句柄，绑定/传参/返回全走既有 gc 面，**`fitAbi` 零新臂**。这是本设计刻意选盒为 gc 值而非多字聚合的理由：多字会新开一条 ABI 面（簇 9 的 `abiPrim` 拒绝正说明新 ABI 面的代价），而盒的载荷面只在**发射点与 thunk 点**出现，不外溢到签名。

**边界**：不做运行期描述符合成；不做盒的复用/缓存/CSE；不引入类型标签或下转型。**未决（实现期查证）**：`dynFace:7095` 只查 E0819/E0820，**不查所有权类别**——resource 类别的头进盒的释放纪律（规范 §Iterable implementer obligations 只对 `Iterable` 说了 E0903）须在实现期查证并在此补记；本设计不预设结论。

**实现期补记六（T5 落地时露出，五处对 D5 的收口、补全、订正与披露）**：

1. **① 阻塞前置的收口（上面那条未决）：resource 到不了盒——四条直路被 E1106 逐条关死；但有一条泛型位旁路，它属实是语言面缺口，本任务登记 follow-up、不动检查器。**

   四条直路（真机探针，check 阶段逐条 exit 1，措辞逐字）：① 直构 `Dyn<Describe>(r)` → `E1106 … the construction Dyn<Describe>(r) with a resource-typed argument is the box route; a resource is never boxed`（`typecheck.go:7222`，既有黄金锚 `check-e1106-dyn-construct`）；② 经 gc 记录字段 `record Holder { h: FileHandle }` → `E1106 … "FileHandle" appears as a gc record field`；③ 经 newtype 底层 `newtype Handle(FileHandle)` → `E1106 … "FileHandle" appears as a newtype's underlying type`；④ 经 sum 载荷 `type Slot = Full(FileHandle) | Empty` → `E1106 … "FileHandle" appears as a sum payload`（②③④ 为探针 `q-fieldres`/`q-newres`/`q-sumres`）。类型位的那条是 `check-e1106-dyn-type`（`typecheck.go:4348`）。

   **旁路**：`fn boxed<T>(v: T) -> Dyn<Describe> where T: Describe { return Dyn<Describe>(v) }` 加 `let d = boxed(f)`（`f: FileHandle`）→ **check exit 0 / build exit 0**，盒照发：`@.dynmap0 = [1 x i64] [i64 2]`、资源句柄落在偏移 24。

   **根因在检查器侧**（`resource.go:431-440` 的 Call 臂）：该臂对**任何**调用都把活的 resource 实参置 `resKilled`——注释自陈理由是 "the argument hands the handle to the callee's signature"——但**从不核对被调签名的形参是否声明了该资源类型**，而 E1104 的措辞里那句 "pass it to a fn whose parameter declares the type"（`resource.go:96`）正是一条**未被兑现的判据**。形参是开位置 `T` 时两头同时失守：**被调侧没有释放义务 ⇒ 释放被整条省掉**；**构造点的实参类型是位置而非资源 ⇒ `dynFace` 看不见 E1106**。

   **与 T5 的边界（为什么本任务不修）**：该缺口**独立于盒**——`fn width<T>(v: T) -> Int64` 加 `width(f)`（探针 `q-genres2`，无任何 `Dyn`）在**父树** `we-parent` 上就是 check 0 / build 0 且 release 被省；`q-genres3`（无约束的 `fn width<T>(v: T)`）同。T5 只是让**装箱这一支**变得可达（`q-genres`：父树 build 70、本树 build 0）。而**具体形参的纪律是硬的**：`fn width(v: FileHandle) -> Int64` 是 E1104——规矩本身存在，缺的是**开位置上的兑现**。故按任务书「属语言面则登记 follow-up 且本任务不预设结论」：登记 roadmap **#24**，本变更不碰检查器。**本任务不预设结论的部分**：那条规矩该落在哪一层（检查器的形参类型核对、实例化后的复核、还是 `Dyn` 构造点对位置实参的单独判据）留待该 follow-up 裁定。

2. **载荷面 D5 未枚举的一支：sum 恒三字，逐位置取「全变体同意」判据。** D5 决策 3 只枚举了 gc 句柄／scalar／String／元组四支。sum 按 `classType` 的既有分类落到 `abiSum`——**恒三字**（tag + 两载荷字），与 sum 的 ABI 同形；**tag 字不描**（它是运行期值，不是句柄）。每个载荷位置的描记位取「**所有到达该位置的变体都同意是 gc 句柄**」，两变体不同意即边界（`e.bnd()`）。**理由**：描述符是**一个编译期常量**，而 tag 是**运行期值**——没有一个位图能同时正确描述「这一字在 A 变体下是句柄、在 B 变体下是标量」。实测：`type Shape = Circle(Inner) | Dot` → `@.dynmap0 = private unnamed_addr constant [1 x i64] [i64 4]`（位 2 = 偏移 32 = pay0），`alloc(i64 48)`；`type Shape = Circle(Int64) | Boxed(Inner)` → exit 70 + `bndMainBody`（探针 `r-sumamb`）。全标量 sum（`Circle(Int64) | Dot`）map 字为 `null`。

3. **盒形与描述符逐面实测（实现锚）**：`internal/codegen/dyn_test.go` 十枚，每面钉**整条指令序列**而非单点（alloc 尺寸、map 字、root_push、vtbl 字、每个载荷字的偏移与**它从哪个寄存器读**）。实测面：scalar（`newtype Celsius(Int64)`）`alloc 32` / map `null` / 载荷常量 `i64 1` 于 24；gc 记录 `alloc 32` / `@.dynmap0` / 载荷经所有权读得 `ptr %r` 于 24；String 两字 `alloc 40` / map `null` / `ptr @.s0` 于 24、`i64 1` 于 32；sum 三字 `alloc 48` / map `null`；`where` 约束位（`fn boxed<T>(v: T) -> Dyn<Describe> where T: Describe`）解到实参后 `@.dynmap0` + 实参面载荷。**描述符恒为编译期常量**（D5 决策 4 兑现，`@.dynmap<n>` 按载荷面去重：四枚构造两枚面 ⇒ 恰一个全局），**`fitAbi` 零改动**，**偏移 16 恒是表指针且无 tag 字**（`store ptr null`，T6 的接缝）。

4. **③ 的两条验证项在 T5 内不可满足——订正，不静默**：
   - **「gc 压测黄金……盒载荷为 gc 句柄且循环分配越过 `GC_THRESHOLD` 后仍可读」**：T5 **没有读通路**——`Dyn<I>` 值的唯一读取是 T6 的方法派发（`Dyn<I>` 的方法集就是 `I` 的方法，`dyd.describe()` 形式；今天停在没有 thunk 上）。故 T5 的 run 黄金只能观测**流**：构造 → 传参 → 返回 → 丢。**取证**：`we-gccount`（临时给 `gc.c` 的 `__we_gc_collect` 加一行 stderr 计数、跑完即 `git checkout` 还原，`git status` 复核干净）实测交付黄金 `run-dyn-box-gc-crossing` 的源：120000 轮循环（`Cell` 24B/轮 ≈ 2.9MB）触发**两次收集**（swept=43687、43691），程序退出 0、stdout 逐字节为 `7199940000`——**盒在两次跨阈值收集之间存活且不破坏运行**，这就是 T5 能断言的全部；「仍可读」留给 T6 的方法派发黄金。
   - **「突变敏感——撤盒描述符的描记位即红（黄金层捕获，非仅单测层）」**：**前半不可达、后半说了反话**。两枚单点突变逐枚实测（各自还原、锚点先断言 `count==1`）：**M-t5-a**（`dynMapName` 的早退改成「永远有描述符」+ 位图循环改成「每个字都描记」）**黄金层与单测层双双捕获**——`run-dyn-box-gc-crossing` 在突变体下 `we: build/demo terminated by signal`（描记位落在标量载荷上，收集器把 `5` 当块指针读，SIGSEGV）；**M-t5-b**（`dynMapName` 恒返回 `""`，即**撤掉全部描记位**）**单测层四枚红、黄金层静默**——`run-dyn-box-gc-crossing` 在突变体下 exit 0、stdout 逐字节不变。**为什么静默**：载荷在 T5 无消费者，且每个存活的 gc 载荷同时被**它自己的构造根**持有（D5 决策 5 的 `e.pushes` 姿态），收集器扫不到它的描述符，也就无从看见那个位。故 T5 的黄金层能捕**假阳性描记**（错位指针立即炸），捕不了**假阴性**（该描的没描）——**黄金层的假阴性捕获随 T6 的读通路落地**，本任务的短语以删除线订正。

5. **一处真缺陷（不在本任务面内，由 T5 的探针照亮）：泛型体内推断应用的实参是开位置。** 语料 `fn id<T>(x: T) -> T { return x }` + `fn twice<T>(x: T) -> T { let a = id(x) return id(a) }`（探针 `q-nestgen`，**无任何 `Dyn`**）：检查器只走一遍泛型体，`noteApply` 在 `id(x)` 上记下的是**开位置** `ShapeParam`，而发射面上没有任何 shape 替换——`siteArgsFrom` 把开位置直接交给 `instFn` → `mangleSuffix` → `mangleShape`。**红证据（父树 `we-parent`）**：`panic: codegen: mangling an unresolved type position`（`mangle.go:117` ← `mangleList` ← `mangleSuffix`）——不是边界，是**崩溃**。**修法**：新增 `instShape`（`inst.go`，把 `ShapeParam` 在 `instEnv` 上解到实参，解不动则原样返回），并在 `siteArgsFrom` 里对每个记录下来的形参位置应用一次；锚 = `TestInferredApplicationInsideAGenericBodyResolves`（本树 `define i64 @main.twice$Int64(` 与 `define i64 @main.id$Int64(` 齐出）。**边界一条**：在泛型体内**自己写出**类型实参（`id<T>(v)`）仍是 **E1304**（`"T" is held by no scope`）——那是既有的类型实参位规则，与本崩溃是两回事，不随本修复放开。

---

## D6 vtable 与动态派发

**事实**：codegen 今天零 `Dyn` 下行（全文件唯一命中是 `:5531` 的注释）。但配对表的**半边已经存在**：`e.methods`（键 `headKey + "." + 方法名`，`collectImpl:589`）、`e.ifaceSeen[ifaceKey]` / `e.ifaceHeads[ifaceKey]`（接口 → 实现头的配对，`:609-618`）、`collectDefaults:627-646`（按头实例化默认方法体），且已有「`m.Body == nil` = 该头自己的义务、不是默认体」的判据（`:631`）。**缺口**：`collectImpl:609` 的 `len(iface.Args) == 0` 把**带类型实参的接口 impl**（`impl Iterator<Int64> for Cursor`）整个排除在配对表之外——而 `Iterator` 正是带参的那一个。B1b 须把配对表扩成 `{ifaceKey, ifaceArgs} → heads`（键法同 D2）。

**决策 1——vtable 只承载「无默认体」的方法槽**，槽序 = 接口声明的非默认方法**声明序**。默认方法由接口侧的**单份实现**承担（`collectDefaults` 已按头实例化一份），其体内对 `self` 的调用走同一张 vtable。对 `Iterator<T>` 这就是**恰一槽 `next`**（`typecheck.go:1227-1231`；其余 11 枚全有体，`:1232-1306`）。

**决策 2——D1 的补充（本勘查发现）**：登记面须含**每个接口声明的非默认方法槽序（名字 + 参数/返回形）**。理由是 `Iterator<T>` 的方法集**不在任何源文件里**——std 面由检查器内建注册（`typecheck.go:1226-1306`，`ifaceInfo` 无声明节点），codegen 无从另取；而「哪些是默认方法」是检查器独有的知识。这是 D1 那个 `Shape` 出口的**声明面条目**，不是新的出口。

**决策 3——vtable 全局**：`@.vt.<接口键>$<实参改编>.<具体型改编>`（改编法见 D2），`private unnamed_addr constant [n x ptr]`，槽 *i* 是第 *i* 个非默认方法的实现符号（`fnDef.sym():737`）。vtable **永远静态**——与 `list.c` 必须运行期合成描述符恰成对照：容量是运行期值，而具体型是编译期事实。

**决策 4——thunk 是薄适配层，不新造调用惯例**：每个 (vtable, 槽) 一枚 `define internal <retTyp> <thunk>(ptr %box, <参数…>)`，体 = 按载荷面从盒取接收者（gc 面直传句柄、scalar 面直传值）→ **静态调用** `@<impl 符号>`。这与 `emitFnRef` 的 adapter thunk（`codegen.go:4200-4202`：env 丢弃后转发到 `fd.sym()`）是**同一形**。

**决策 5——派发点用既有间接调用习语**：`gepLoadPtr(box, 16)` → GEP 到槽 *i* → `load ptr` → `emitCallCore(abi, "%reg", ["ptr "+box, …])`。勘查已证 vtable 槽载入**正好落在 `parts.fnptr` 的位置**（`closure_test.go:197` 钉的 IR 形逐字可复用），接收者经 `preOps` 以 `"ptr " + reg` 传入。返回 `abiSum`（`{i64,i64,i64}`，`fitAbi:10464-10475`）的槽**原样穿过**——`next` 返回 `Option<T>` 不引入新面。

**边界**：不做多接口合并、接口继承、trait 对象间的转换；不做类型标签/下转型；不改 `collectImpl` 的两条泛型拒绝（那是 D2/D3 的活）。**IR 钉**（D12）：vtable 全局的形一枚 + 一个槽载入的形一枚。

**实现期补记七（T6 落地时露出，九处对 D6 的收口、订正与披露）**：

1. **决策 4 的「scalar 面直传值」被实现推翻——接收者恒是 `ptr %self`。** D6 决策 4 写 thunk「按载荷面从盒取接收者（gc 面直传句柄、**scalar 面直传值**）」。实测方法 define 的接收者与头是什么**无关**：`define { ptr, i64 } @main.Celsius.describe(ptr %self)`、`define i64 @main.Point.describe(ptr %self)`。scalar/String/sum 载荷的「接收者作为一个值」在本 build **没有确立的读法**（newtype 接收者本身也不产派发）。故 **T6-2 的表只在 gc 载荷面（载荷恰一个 gc 句柄）发射**，其余三面保留 T5 的 `store ptr null` @16，**逐字节零漂移**。这是**发射能力之界，不是分类器之界**——标量背后挂接口合法且已被语料钉住（盒是跨边界传的值，派发与否都在），若为守住一条无人走的路而拒盒，反而把已落地的面收窄。表字买的是**可派发性**。

2. **决策 2 的兑现：登记面按「是不是本程序声明的接口」判，不按「带不带参」判。** `collectImpl` 的 `len(iface.Args) == 0` 早退整个撤掉，改由 `ifacePairKey` 统一判：`{ifaceKey, ifaceArgs} → heads`，键 = `mangleApply(declKey(iface.Decl), args)`（`Iterator$Int64` / `main.Seq$Int64` / 裸 `Show`）。模块声明的接口键前缀 `main.`，检查器内建的面（`Iterator`/`Iterable`/`Show`）无源文件故裸名；walked 模块不持有的接口（跨模块 `other.Seq`）不产行。行上带 `args`（`[]typecheck.Shape`），供默认体实例化与 thunk 替换用。

3. **决策 1 的兑现：`Iterator<T>` 实测恰一槽。** `TestIteratorFaceCarriesOneSlot` 钉死——`next` 是唯一 `body: nil`，map/filter/take/skip/collect/fold/reduce/count/any/all/find 皆内在默认体（`typecheck.go:1227-1306`）；槽序取 `Face.Slots`，即检查器注册的**非默认方法声明序**。

4. **决策 3 的兑现：表恒静态，且按需在盒构造点发射（不走配对表）。** 发射条件 = 载荷面为**一个 gc 句柄**（`abiGc`）**且**接口槽数 ≥ 1。`TestVtableIsStaticAndNeverAllocated` 先钉常量形（`@.vt.Iterator$Int64.main.CountIter = private unnamed_addr constant [1 x ptr] [...]`），再逐行扫 `@.vt.` 断言无一行含 `__we_alloc`，另以 `dispatchAllocRe`（`= call [^\n]*@\.vt\.`）断言**没有任何 call 的产物是一个表名**。实现面佐证：`grep -c '__we_alloc' internal/codegen/codegen.go` = 9，逐条为 declare、闭包环境、fn 载体、fn 环境——vtable 章节（`boxTable` → `ifaceSlots` → `slotAbi` → `emitVtableThunk` → `dynRecvOf` → `emitDynCall`）命中 **0**。**决策 5 的 IR 形零漂移**：`closure_test.go:197` 钉的间接调用形在父树与本树**逐字节相同**；19 枚探针横向比对——11 枚 IR 逐字节相同、7 枚两侧停在同一句、**唯一行为移动的是派发点自身**。

5. **决策 3/4 的前置修复：`refOfShape` 的拼写形（一次 widening）。** `Iterator<Int64>.next` 返回 `Option<Int64>`，而该应用**只经替换到达分类器**，改编名 `Option$Int64` 永不命中 `classType`——修前 `slotAbi` 无从分类，`@.vt.Iterator$Int64.main.CountIter` 根本出不来。原先在替换位停住的程序现在能 emit（`fn first<T>(x:T)->Int64` 喂 `Option<Int64>`：父树 exit 70 → 本树 exit 0，`TestSubstitutedPreludeSumReachesTheClassifier` 先红后绿）。**语料零漂移**（822 不变），但**面外无覆盖**——该 widening 只由两枚新单测守门，无黄金。

6. **派发点的可达面比 `Dyn<I>` 类型窄——`dyn` 随值走，不随类型走。** 这是 D6 唯一的**设计级**补记：`gcBinding.dyn` 只在 `boxTable` 真发射过表时置位，表只在本体构造点发射，故**可达面 = 在当前体内构造的盒**；一枚盒经参数、返回、字段抵达时静态上不知其载荷面，也就不知偏移 16 是表还是 T5 留下的 null——**停而非跳**。这不是权宜之计：`E0812`（`mut self` receiver on a value-category type）独立证明 value-category newtype 连 `Iterator<T>` 都实现不了，且 newtype 方法体内读 `self.value` 也停在既有边界——非记录接收者在本 build 本就无确立读法（同本补记第 1 条）。**后果**：`Dyn<I>` 作参数/返回值在签名面合法（T5 已钉「可携带」），只是拿到手里不能派发。**实证**：`fn width(d: Dyn<Shape>) -> Int64 { return d.area() }` + `width(d)` → exit 70 + `bndFnBody`。

7. **决策 1 的推论（未钉）：默认方法在盒上没有槽，因此不派发。** `ifaceSlots` 只收 `body: nil` 的方法，故 `Dyn<Shape>` 上 `d.name()`（`name` 有默认体）落回下层各面并停；`Dyn<Shape>` 上 `d.sides()` 才走槽 1。这不是遗漏——默认体已按头实例化（`@main.Square.name`），而**盒不记住头**，无从选到该实例。**风险未钉**：无黄金、无单测钉这条否定；若误把默认体也算进槽，`name` 会落到槽 1 即 `sides` 的 thunk 上，是一枚**不会被现有语料发现**的错渲染。另一条未钉：`slotAbi` 分类失败时本面认领该名（返 `is=true` + `e.bnd()`），不让下层以别的理由作答——名字既已确定为该脸的一个槽，正确的停止理由就是「这个槽的 ABI 分类不了」，诊断指错地方比诊断不出更坏。

8. **补记六第 4 条第二项的预测被推翻：T5 的假阴性方向在本 build 不可观测，不是覆盖缺口。** 补记六结句写「黄金层的假阴性捕获随 T6 的读通路落地」——**T6 落地后仍不成立**。撤销盒描述符的全部描记位（M-t5-b）后黄金层**仍然静默**，且理由与「写了哪些程序」无关：**可派发的盒与载荷被独立扎根的盒是同一个集合**。三步推理——①派发要求 `dyn != nil`，而 `dyn` 只在表发射时置位、表只在本体构造点发射，故可派发 ⇒ 盒在本体内构造；②载荷也在同一体内构造（`emitBox` 的实参），而 `allocRecord` 对载荷**自己 `__we_root_push`**（实测 IR：`%v2 = call ptr @__we_alloc(i64 24)` … **`call void @__we_root_push(ptr %v2)`** ← 载荷自己的根），该根与盒的根同在本体出口 `dischargeRoots` 才弹；③故描记位**从不是**某枚可派发载荷的唯一存续理由。**推论**：本 build 能发射的任何黄金都看不出这个差别——不是「还没写」，是**写不出**；该方向据此从 T15 的突变电池清单撤下。

9. **T6-4 另修一处 T6-3 自身的洞：派发结果的分类（拓宽，非订正）。** `callStrKind` 读成员调用的返回族时经 `recvKeyOf(fn.Recv)`，而**盒没有 record 键**（盒是唯一没有记录垫底的 gc 值），故 `d.area() + 1` 得 `skNone` 而停——值**可绑可打印**（那些消费者会在值落到的域重查），**唯独不能作操作数**（binop 在发射任一侧前先问两侧 kind）。修法是把发射端已有的答案换个读法：槽的 ABI 在 `instShape` 后的实参上分类，与 thunk 同源，故其返回族即调用的值。`emitDynCall` 据此重写到 `dynSlotIndex`/`dynSlotAbi` 两个助手上，**臂与派发共用同一次查表**——调用点与分类器不可能对「这个名字落在哪个方法」得出不同结论。**界**：槽的返回族不在数值域时（sum 如 `Option<T>`、String）仍 `skNone` 而停。

**T6 的突变判决**（单点、逐枚还原、锚点先断言 `count==1`）：**M1**（恒置 `dispatchable`）**黄金层红**（补黄金 `build-bnd-dyn-value-payload-dispatch` → exit 70）；**M2**（槽下标恒 0）**黄金层红**（补黄金 `run-dyn-vtable-two-slots` → 多槽脸 `4\n5\n`）；**M3**（接收者传表不传盒）**单测三红 + 黄金层红**（`run-dyn-vtable-dispatch` → `terminated by signal`）；**M4**（按头合并表）**单测红 + 黄金层红**（`run-dyn-vtable-two-faces` → `1\n1\n` 对 `1\n2\n`，**静默错值**正是该守卫要防的形，不是崩溃）；**M-t5-b** 见第 8 条（改判「不可观测」）。**三条补黄金清单**（M1/M2 的黄金层盲区与 T5 的一条）由此逐条兑现：`build-bnd-dyn-value-payload-dispatch`、`run-dyn-vtable-two-slots`、`run-dyn-vtable-two-faces` 三枚新黄金 + `run-dyn-box-gc-crossing` 扩形（补 `let r = held.next()` + `match`，读回载荷自己的值 `7`——**即 T5 想写而写不出的那句断言**）。

---

## D7 迭代器协议与用户 Iterable

**事实**：规范三段钉死——`Iterator<T>` 只有一个非默认方法 `next(mut self) -> Option<T>`，且**接口开放**（模块可为自己名义头实现，静态派发）、可装箱（`Dyn<Iterator<T>>`）；`Iterable<T>` 有一个关联型 `Iter`（故 **E0819 不可装箱**），其绑定须实现同元素型的 `Iterator<T>`（E0904）；`for` 协议单一形（求值一次 / `iterator` 一次 / 反复 `next` / 首个 `None` 结束），且**终止由 `None` 判定**而非长度或下标。检查器侧全绿（`forElem:8085`、`iterElemOf:8106`、E0901/E0903/E0904 在 `checkImplDecl:3519-3541`；`m6a_test.go:167` 是绿锚）。codegen 侧：`emitFor:4995` 只认三形，`listFaceOf:5243-5266` 对 `*ast.Call` 源（即 `x.iterator()`）直接 **false → `e.bnd()`**。

**决策 1——协议的权威在检查器，发射形可特化**：既有三形（Range 计数循环无堆 `:5023`、String `__we_str_runecount`/`__we_str_charat` `:5093`、List 快照 `__we_list_snap`/`__we_list_len`/`__we_list_get` `:5153`）与协议**逐条同义**——各是「求值一次 / iterator 一次 / 首个 None 结束」在某具体型上的特化。**三形字节不变，零黄金触碰**；协议形只为**其余**源（用户 Iterable、将来的 Set/Map）新增。理由：协议的规范力在语义，不在强制发射形；把三形改写成协议形是给已在跑的黄金添回归风险、换不来任何语言面的东西。

**决策 2——用户 Iterable 走协议形**：`for x in r` → `%it = call @<ImplIterable.iterator>(ptr %r)`（具体型已知，**静态调用**）→ 循环 `%o = call {i64,i64,i64} @<Iter.next>(ptr %it)` → 按 tag 分派：`None` 出循环、`Some` 按载荷面（D9 簇 1）绑元素执行体。`for` 源的 `*ast.Call` 形须进 `emitFor` 的分派（今天落 `listFaceOf` 的 default）。

**决策 3——内建源进盒需要具体迭代器对象**：惰性四枚的结果型是 `Dyn<Iterator<U>>`，故 List/String/Range 的迭代器要有**对象形**：`ListIter` = {list 句柄（描）, index（不描）}、`StrIter` = {String 两字（不描）, index}、`RangeIter` = {cur, hi}。载荷面决定 `@.dynmap`（D5 决策 4）。与决策 1 不矛盾：内联 for 形**不造**这个对象（零开销路径保留），只有进盒或交到组合子手上时才造。**未决（实现期第一件要查的事）**：这些类型是「std 侧声明 + 内建 impl」还是「codegen 内建合成」——`Iterator`/`Iterable` 本身是检查器内建 `ifaceInfo`、**无源文件**，故本案取决于 std 面今天如何承载内建 impl；形此处不定死。

**决策 4——`next` 的循环用现成的三槽语义**：`sumSlot` 就是 `{tag, pay0, pay1}` 三 i64 槽（`:853-880`），`variants` 表把运行期值映射到源级名（`None=0`/`Some=1`），`armTagTest:6545`/`slotVariantIndex:6566` 已有按 tag 分派的现成机制——协议循环复用它们，不新造判定。

**边界**：不改三内联形；不新增诊断码（`E0901`/`E0903`/`E0904` 全在检查器）；`iterator` 每次进 `for` **只调一次**（快照语义，与 `__we_list_snap` 同法）。**红证据**：~~`docs/benchmarks.md` 的「a user impl of `Iterable`」面~~（**该引文失准，见补记九订正一**）；黄金锚 `-list-iterable`、`-acute-user-iterable`（设计期皆 exit 70 + `bndMainBody`；T7-A 后 `-list-iterable` 已翻绿，`-acute-user-iterable` 仍红）。

**实现期补记八（T7-1 阻塞前置的查证结论：内建迭代器类型的载体 = codegen 内建合成）**：

**结论**：`ListIter`/`StrIter`/`RangeIter` **不落 std、不进检查器**——类型、gc 描述符、构造与 `next` 的发射四件事全在 `internal/codegen`。据此**定一处发射点**（决策 3 的未决项收口）。

1. **「std 侧声明 + 内建 impl」这条路今天不存在，走它要先造一个新权威。** `find . -name '*.we'` 不含任何 std 树；`std.io`/`std.concurrent`/`std.test`/`std.time` 是检查器里的**桶**（`typecheck.go:2467-2503` 的 import 臂），不是源文件。造 std 源树要连带一条模块解析通路与一套「std 声明如何进 codegen」的纪律——那是把「句柄型」的权威搬到一个今天不存在的地方。

2. **检查器侧刻意没有具体句柄型，故 codegen 造这一个不是第二处权威、是补上唯一一处。** `String.iterator` 与 `List<T>.iterator` 的声明返回是**裸接口型** `Iterator<Rune>`/`Iterator<T>`（`typecheck.go:1336`/`:1353`），注释自陈动机：「a String iterates runes, so iterator hands the Iterator<Rune> face, **the handle type itself the standard library's**」——检查器给的是**面**，不是**型**，该位置**不携带任何具体型信息**。**但它是发射面读不懂的型**：本树实测 `let it = "abc".iterator()` 与 `let it = xs.iterator()` 皆 **check 0 / build 70**，`Dyn<Iterator<Int64> >(xs.iterator())` 同（即 T6-2 披露 4 的 `boxFace` 那一条，今日在 List 上同样成立）。**本变更不修检查器**，故该占位与 codegen 的内建对象在**今天**不通信——这是**已登记的既有事实**（T6-2 披露 4 已记），不是本变更新造的洞；将来 std 面若落地，两处须合并。写在这里，不静默。

3. **既有先例已经这么走。** `<list>.iterator().<急性六枚>` 由 `emitAcute:6011` **合成识别**——不造对象、不进 std、`runtime/c/` 零改动；`emitForList`/`emitForString`/`emitForRange` 同理走 `__we_*` 运行期直发。内建迭代器对象是同一条姿势的延续。

**实测边界图**（本树 `we-cur`，真机逐形取数；`check` 列全为 0）：

| for 源形 | build | 归属 |
| --- | --- | --- |
| `0..3`（内联 Range） | **0** | `emitForRange` |
| `s`（String 绑定） | **0** | `emitForString` |
| `[1, 2, 3]`（List 字面） | **0** | `emitForList` |
| `xs`（List 绑定） | **0** | `listFaceOf` 的 Ident 臂 |
| `r`（**命名的 Range**） | 70 | **无臂**——T7-2 一并收 |
| `mk()`（Call 返回 List） | 70 | **无臂**——T7-2 的正身 |
| `h.xs`（局部记录的 List 字段） | 70 | `listFaceOf` 的 Member 臂只认 `topMember` |
| `r`（用户 Iterable 记录） | 70 | **无臂**——T7-2 的验收形 |

**七枚缺口全在发射面，检查器一个都不缺**——这条同时是 T7 的边界声明：**本变更零检查器改动**。

**决定性取证：用户 Iterable 的协议不需要任何新机制。** 手工写出的协议循环**今天已经跑通**（真机 `0\n1\n2\n` exit 0）——`let it = r.iterator()` 是普通方法调用（`emitMethodCall` → `@main.Range2.iterator`）、`let o = it.next()` 是普通方法调用（`@main.CountIter.next`）、`match o` 走**既有 Option 三槽语义**（`armTagTest`/`slotVariantIndex`）。故 **T7-2/T7-3 是一个发射面**（`emitFor` 的协议臂），不是新机制：三件**都已成立**，缺的只是 `emitFor` 认识这个源并把它串成循环。

**句柄型的权威是 impl 的 `type Iter = …` 行，不是方法签名的拼写。** 实测：impl 把签名写成 `fn iterator(self) -> Iter`（用关联型名而非具体型）**check 也是 0**——故发射器**不能**靠签名拼写取头键。`ast.ImplDecl.Assocs []*AssocBinding`（`ast.go:265`）携带 `type Iter = CountIter`，而 codegen 本来就 walk 这棵树（`collectImpl` 正在其上），**故权威在发射器已经持有的树里，无须新出口**。**由此暴露一条既有边界（如实登记，非本变更的洞）**：签名写成 `-> Iter` 时，`let it = r.iterator()` 的**普通绑定**停在 `bndFnBody`（`classify` 读声明返回型时解不动关联型名）。协议臂从 `Assocs` 取头键，故 `for c in r` 在**两种拼写下都能走**；~~普通绑定那一面维持原边界、不在 T7 面内。~~**订正：该面在 T7-A 一并关闭（同一处解析的连带结果），见补记九订正二。**

**D7-附 的例外口据此关闭**：查证**没有**落到「std 侧声明 + 内建 impl」，故 D7-附 预留的「补记一个运行期构造器符号」**不触发**——`ListIter`/`StrIter`/`RangeIter` 的构造是编译期形状（`__we_alloc` + 自写 map 字，与盒同法），`runtime/c/` 维持**零改动**。

---

**实现期补记九（T7-2/T7-3 落地时露出：两处订正、四处实现期裁定、一处超出任务书的拓宽）**：

**提交**：`7df9d4b`（码 + 单测 + 黄金，7 files / +603 −26）与 `45db419`（docs 双语，2 files / +2 −2）。T7 的四枚复选框**只关前两枚**（T7-1 阻塞前置已于补记八收口、T7-2/T7-3 本文），T7-4（内建迭代器对象）与急性面仍未开工。

**订正一：D7「红证据」的引文失准。** 原文引 `docs/benchmarks.md` 的「a user impl of `Iterable`」面。**实测该文档自 M15 落地以来从未出现 `Iterable` 字样**——`git log -S "Iterable" -- docs/benchmarks.md docs/benchmarks.zh.md` 为空，其「仍关死」清单五面也从来不含此面；`internal/benchmarks/` 任务集同样零命中。故**该引文是设计期的凭记忆引用**，真正的红证据是黄金锚本身（`build-bnd-list-iterable` 在 `2fbfc59` 树上 exit 70 + `bndMainBody`）与补记八的真机探针表。**处置**：D7 该句以删除线订正；`docs/benchmarks.md` 的该面**由本次补上**——但补的是**已拓宽**句（`45db419`），不是红证据句，因为 T7-A 后它已可跑。

**订正二：补记八末段登记的边界被同一枚改动关闭。** 补记八把「`-> Iter` 拼写下的普通绑定 `let it = r.iterator()` 停在 `bndFnBody`」登记为「既有边界、不在 T7 面内」。实作时发现**解不动该名的正是发射器自己**（`emitFnDefine` → `classify(fd)` 读 `fd.decl.Ret`），不解决它 `iterator` 的**定义本身**就发不出，协议臂无从谈起。修法是在 `collectImpl` 建表时用 `assocRet` 把 `decl.Ret` 换成关联型绑定命名的 `*ast.NamedType` **副本**（不改原树——树属检查阶段，其他 pass 也要走）。**连带结果**：`let it = r.iterator()` 在 `-> Iter` 拼写下**一并可用**。**这是超出任务书的拓宽**，如实登记：它不是 T7 面内的副产物，而是同一处解析的自然延伸。

**裁定一：协议臂的源形不限 `*ast.Call`。** 任务书写的是「增 `*ast.Call` 源的协议形分派」，实作按「`recvKeyOf` 能命名的记录」分派，故绑定（`r`）、字段链、构造皆可走。**但 `*ast.Call` 源恰好仍停**：`recvKeyOf` 对一个调用结果不给键，故 `for c in make()` **今日仍 exit 70**——任务书那一句的**字面验收形反而不通过**，而其验收黄金 `build-bnd-list-iterable`（源为绑定 `r`）通过。已由 `TestForProtocolStopsOnACallSource` 登记为**仍红的事实**（检查器两侧皆 0，界纯在发射面）。

**裁定二：`Some` 的索引必须从被调方签名携带的表读，不得硬编码。** 检查器 `optionSum`（`typecheck.go:1035-1042`）声明序为 **[Some, None]**，而 codegen **自己的** `variantShapes`（`:11347-11352`）返回 `[None, Some]`（None=0、Some=1）——**两处顺序相反**。故 head 的 tag 判据用 `slotVariantIndex(o.sum, "Some")` 从 `next` 返回的槽自己的表取。**这是 D7 决策 4「复用现成三槽语义」的正确性关键**：任一处硬编码都会让用户 `Iterator` 的 `None`/`Some` 反读。

**裁定三：终止判据是 `payloadBindable`，不是「能解出 Some」。** 载荷面不在 `{i64, double, str, gc, void}` 即 `iteratorOf` 返 false、源停。理由：三槽只能装一个值面，绑不出的载荷宁可停，不绑一个读作零的名字。

**裁定四：head pattern 须在调用 `bindArmWord` 前显式挡。** `bindArmWord`（`:7154`）对**非** `*ast.PatBinding` 的 pattern **静默无操作**——直接用会让元组头/变体头静默无绑定。`bindForIterElem` 只放行 `*ast.PatWildcard` 与 `*ast.PatBinding`，其余 `e.bnd()`。

**边界图更新（真机 `we-t7a`，逐形取数）**：补记八七行中**只有一行翻转**——`r`（用户 Iterable）70 → **0**。其余三行经实测仍 70：`r`（命名的 Range）、`mk()`（Call 返回 List）、`make()`（Call 返回用户 Iterable）、`h.xs`（局部记录的 List 字段）；四枚内置形（Range 字面 / String 绑定 / List 字面 / List 绑定）仍 0。**「`r` 命名的 Range」一行是补记八表内的既有缺口，其括注写的「T7-2 一并收」未被兑现**——实作只收了用户 Iterable 那一行，如实登记为**补记八的预测未兑现**（与 T12 记的 D12 第 10 枚同型：设计期对拓宽面的预测偏乐观）。

**陈旧单测的重锚（三枚，均在 `7df9d4b`）**：① `list_test.go` 的 `TestListWalkOverAUserIterableStops` —— 它构建的合成模块**根本没有任何 impl**，注释却自称测「a user impl of Iterable stops at the body word」；T7-A 后该断言对 `for` 面已不成立。**改为正向派发钉** `TestListWalkYieldsToTheProtocolFace`（合成 `Iterator`/`Iterable` 两枚 impl + 手工 `Assocs`，断言发射干净、IR 含 `@main.Range2.iterator` 与 `@main.CountIter.next`、**不含任何 `__we_list_` 谓词**——把记录读成 List 会是静默错走）。② `acute_test.go` 的 `TestAcuteOverAUserIterableStops` —— 断言仍真（急性面未拓宽），注释末句「as it does for a for statement over the same source」已成假，**订正为点明两面分属**。③ `m10b_test.go` 的 forBody 模块注释 —— 原称「the source the build still refuses … a user impl of Iterable」，T7-A 后窄化为「源是不命名任何绑定的 Ident」（该模块的 impl 也确实没绑 `type Iter`，已实测：绑上源后仍停在同一处）。

**实现期补记十（T7-B 落地时露出：一处预登记 sketch 的简化偏差、三处披露、一处重锚）**：

**提交**：`9045441`（码 + 单测 + 一枚翻绿重锚 + 一枚新 run 黄金，4 files / +465 −40）与 `1c77757`（docs 双语「仍关死」清单补第六面，2 files / +2 −2）。T7 第 3 枚至此收口；**余第 4 枚**（内建迭代器对象）。

**简化偏差：`listWalk` 无需 `proto` 字段，`closeListWalk` 零改动即两形共用。** T7-A 收尾时预登记的做法是「`listWalk` 增 `proto *protoWalk{it, slot, some, pay}`；`closeListWalk` 两形共用」——实作时发现**前半不必要**。理由：两形的 step 块本就同形——协议 walk 的 pass 计数器放在 step 而非 head（head 载 `cur` → body → step 自增 → 回 head），与载体形的 step 逐字节同构，故 `closeListWalk` 一行不改就共用。真正的差异**只在 head**（载体：快照 + 长度 + `icmp slt`；协议：`next` 调用 + tag 判），因此分叉点收敛为一个 `walkSrc{src string; proto *iterProtocol}` 与一个 `openWalk` 分派；`listWalk` **一个字段没加**，只改写了 doc（点明 `snap`/`n` 是 List 载体专有、协议形留空）。**这是对预登记 sketch 的简化，如实登记**——若照原 sketch 加字段，会引入一个两形中只有一形读的 `proto` 副本，与「一类事实一个权威」相悖。

**披露一：`emitStep` 的 pass 计数器在协议形无上界证明——缺口点名而非证掉。** 载体形的 `cur < n` 有证明（`n` 是快照长度，`cur` 从 `i` 起递增），故 `add` 不检查是安全的（既有 doc 自陈）。协议形**没有这条证明**：没有任何东西限制一个句柄能答多少次 `Some`。处置是**保留 unchecked `add` 并在 `emitStep` 自己的 doc 里点名该缺口**，理由三条：①wrap 需要 2^63 pass，物理上不可达；②计数器不是源可见值（无第 7 章的溢出面可报）；③它唯一的读者是 reduce 的首元素判据，而 wrap 只会把该判据**重新置位**（把一次 later 误读为 first），不产生越界访问。**登记为已知缺口，非已解决。**

**披露二：`String` 载荷停——这与 List 载体面一致，不是新引入的不对称。** `protoElemFace` 对 `abiStr` 返 false ⇒ 该源的**六枚组合子全停**，`count` 也不例外（它不读元素面，本可作答）。这条看似比 List 面严，**真机实测并非**（`we-final`，逐形取数）：`List<String>` 上 `xs.iterator().count()` → exit 70、`for x in xs { io.println("tick") }`（元素**不被使用**）→ **亦** exit 70，而同形的 `List<Int64>` 两枚皆 exit 0。**故停点是「源的元素面」，与循环体读不读元素无关，急性面与 for 面在 List 载体上同样按此分流。** 结论：**两源接受同一集合**——这是「元素面属于源、在选择组合子之前就已固定」的直接后果，也正是本任务要的：用户序列不比 List **少**接受，也不比它**多**接受。（注：这与 `docs/benchmarks.md:50` 清单里「builtin 所产 `Option` 载荷读进 `String` 位」**不是同一面**——那条是读入位，本条是源的元素面。）

**披露二的连带：一份既有披露缺口被探针抓出并补登。** 该面此前**不在** `docs/benchmarks.md` 的「仍关死」清单里——清单把「`List` 迭代」列在**已拓宽**句且未加限定，而 `List<String>` 的迭代本 build 发不出（上段实测）。运行面锚定段是 L1 任务作者读的唯一权威，「写了可跑、实跑 exit 70」正是该段最该避免的失准。**处置**：`1c77757` 在双语清单补第六面「元素为 `String` 的源（`List<String>`，或 `Iterable` 关联型产出 `String` 的记录）作 `for` 或组合子的源（无论元素是否被读取）」。**这是登记既有边界，不是关闭它**——关它是另一件工作，不在本变更面内。

**披露三：一处陈旧单测的重锚。** `acute_test.go` 的 `TestAcuteOverAUserIterableStops` 在 T7-B 后**断言仍真但名与 doc 皆失准**——它的夹具只 `recDecl("Range2", …)` 而**没有任何 impl**，故 `iteratorOf` 返 false、`emitAcute` 落到下面的面并被停住，与「用户 Iterable」毫无关系（补记九的重锚第 ② 条只订正了 doc 末句，未动名与夹具）。**更名为 `TestAcuteOverARecordThatIsNoIterableStops`**，doc 改写为「成员资格是关联的，不是接收者的形状」——它现在钉的是**门**：一枚没绑 `Iterable` 的记录不是这个面。

**边界图更新（真机，逐形取数）**：六行中**只有一行翻转**——`acute over a user Iterable (count)` 70 → **0**。其余五行实测 `same`：`for over a user Iterable`（T7-A 行）0/0、`acute over a List`（基线）0/0、`acute over a call-returned Iterable` 70/70、`acute over a String source` 70/70、`acute over a record with no Iterable` 1/1（此行为检查器 `E0901` 拒绝，非发射边界）。**故 T7-B 未引入任何新缺口**——六枚组合子在协议形上的 IR 逐枚复核：`iterator` 恰 1 处调用点（在 head 之前、紧随 `__we_root_push`）、head 内 `next` + 三 `extractvalue` + `icmp eq i64 <tag>, <Some 的索引>` + `br i1 … cbody/cexit`、`cbody` 首条即载荷 `load`、`cstep` 为「载计数器 + `add 1` + store + 回边」、全 IR 无 `__we_list_` 动词。

---

**实现期补记十一（T7-4 落地时露出：方案由既有约定唯一确定、两枚描述符位不可观测、一处判据冗余）**：

**提交**：`26817db`（`codegen.go` + 新 `iterobj_test.go` + 五枚黄金 + docs 双语，9 files / +545 −21）。T7 第 4 枚至此收口——**T7 全节五枚复选框全绿**。

**一、对象形不是设计品味，是被 `emitVtableThunk` 的既有约定唯一确定的。** 决策 3 定了「内建源进盒需要具体迭代器对象」，**没定对象长什么样**：原则上可把盒载荷**直接铺成** {句柄, index} 两字。唯一排除它的是 T6 已落的 thunk 约定——thunk 体恒为 `%recv = load ptr, ptr (box+24)` 并把 `%recv` 当 `self` 传给实现，故 `next(self)` 只能收到**一个**指针；扁平载荷下 `next` 无从读到 index。⇒ 句柄与位置必须同行于**同一次 `load`** ⇒ **对象是机制要求的形状**。**推论（本任务最省力的一点）**：`vtableFor`/`boxTable`/`emitVtableThunk`/`classify` **四者零改动**——合称一枚 `fnDef{abiOK: true}`（`ret: {i64,i64,i64}`、`ret: abiSum`）经 `e.methods` 表进现成机制即可（`abiOK` 短路 `classify`，故无须 AST、无须 impl 声明）。

**二、挂点在 `emitBox` 而非 `classType`，因为头键不在 site 里。** `Site.Boxed` 对 `Dyn<Iterator<T>>(<内建源>.iterator())` 是**接口 `Iterator<T>` 本身**——`dynCall` 取的是**实参的静态型**——**不携带具体头键**。故合成头键只能从**实参表达式**解析（`iterSourceOf` 读 `m.Recv` 的元素面，合成 `ListIter$<元素域>`）。这是本任务唯一的架构性判断，也是它为什么不能进 `classType`（那里拿到的是型，不是表达式）。

**三、`dynMapName` 的泛化：`lead` 参数。** 原签名按面取（`dynMapName(f boxFace)`）且**硬编码「slot 0 是表指针、永不描记」**——对盒为真，对迭代器对象**为假**：对象的偏移 16 是 **list 句柄**，是 gc 指针、**必须描记**。故拆为 `dynMapName(words []boxWord, lead bool)`：`words` 是**从偏移 24 起**的各字，`lead` 说偏移 16 是否描记；去重键以 `'g'`/`'x'` 起头（两枚 lead 值各一），后接每字 `'g'`/`'s'`。**故盒与对象拿到两枚不同 global**——真机 IR 逐字：`@.dynmap0 = private unnamed_addr constant [1 x i64] [i64 1]`（对象：bit 0 = 偏移 16 的句柄）与 `@.dynmap1 = private unnamed_addr constant [1 x i64] [i64 2]`（盒：bit 1 = 偏移 24 的对象）。两枚**恰好差一位**，正说明去重键必须含 lead，否则二者会串用同一张表。

**四、两枚描述符位在本 build 不可观测——与补记七第 8 条同因，不是覆盖缺口。** 真机突变两枚（逐枚还原）：**A** 对象 `lead` true→false（句柄位不描）；**B** 盒载荷 `traced` true→false（盒→对象指针位不描，map 写 null）。**两枚压测皆仍绿**（`7199940000 / 100 / 200 / 300 / none`，exit 0）。理由与补记七第 8 条**逐字同构**：`allocObj` 对**每个**分配都 `__we_root_push`，该根在作用域出口 `dischargeRoots` 才弹，故 box → object → list **三者互相独立扎根**，描记位从不是任一枚的唯一存续理由。**这是必须如实披露的事实，不得声称为「黄金层捕获」**——任务书的验收句只要求「`gc` 压测绿」，未要求突变可捕获；本任务按原句兑现，并把不可观测性登记在此，不冒充更强的东西。

**五、探针抓出一处判据冗余（新发现，非本任务引入的缺陷）。** `iterSourceOf` 有两道守卫：`elem.gc` 与 `strKindName(elem.kind) == ""`。真机突变证明**二者互为冗余**：单摘任一枚，另一枚接住——`List<Cell>` 的元素面 kind 是 `skNone` ⇒ 第二道接住（实测：单摘 `elem.gc`，`List<Cell>` 仍 exit 70）；而 `List<String>` **根本不进这里**（`elemFaceOfType` 对两字元素返 false ⇒ `listFaceOf` 在绑定层即答 false），正是 T7-B 登记的「源的元素面」那条边界，T7-4 **未引入新的 String 面**。**两枚同时摘掉**才放行，此时 `List<Cell>` exit 70 → **0**（一枚 `next` 会把 `__we_list_get` 答的 i64 当 gc 引用用）。**判决**：两道守卫**联合**定义边界，单独任一为冗余防御；负例黄金 `build-bnd-builtin-iterator-gc-elem` 盖住的是**联合行为**（实测捕获：单摘不响、同摘即红），**单枚突变不可观测**——如实登记，不冒充「逐行可突变」。

**六、可达面与不可达面（须一并披露）**：① **`RangeIter` 今日不可达**——`(0..3).iterator()` 停在检查器 `E0816`（`collectionMembers` 无 rangeSum 臂），而 T7 面明写「零检查器改动」；② **`StrIter` 今日不可达**——`Rune` 无 ABI 面（`classType` 无 Rune 臂）⇒ `Option<Rune>` 不成立；③ 可达元素面 = **单字域**（i64 族 / `UInt64` / `Bool` / `Float64`），两字与 gc 引用元素为边界（见五），已入 docs 双语「仍关死」清单；④ 故**本任务兑现的是 `ListIter` 一枚**——「三枚内建源的对象形」中另两枚是**随其上游前提不可达**，不是未写。**这是决策 3 与今日实际可达面之间的差额，登记在案**：另两枚的对象形一旦上游开口即可按同一处发射点补上（`iterSourceOf` 是唯一分派点）。

**七、零开销路径未被触碰（决策 1 的负断言，实测）。** `for x in xs` 与 `xs.iterator().count()` 同处一个 body 时，全 IR **零 `ListIter`、零 `@.dynmap`**；for 仍走 `__we_list_snap`、急性仍走 `__we_list_len`。有单测钉（`TestListIteratorObjectIsNotBuiltOnTheZeroCostPaths`）。**注意二者的判据不同**：前者挡在 `emitFor`（根本不进 `emitBox`），后者挡在 `emitAcute`——两条零开销路径**各自独立**绕开本处，不是「同一条捷径的两个出口」。

**八、docs 双语运行面随之订正。** ① 「已拓宽」句由**两次**改**三次**，第三项即本任务（`Dyn<Iterator<Int64> >(xs.iterator())` 造对象并经表派发）；② 「仍关死」清单**补一面**：「元素面为单字载体所不能容的内置源（两字 `String` / gc 引用）**进盒为迭代器**」——这一面与已有的「`String`-元素源作 `for` 或组合子的源」**不是同一条**（那条是循环源，本条是进盒），故分立。**顺带登记一处既有措辞的窄口径（未改）**：拓宽句写的是「a `for` source may be a record…」，而 T7-B 同时放开了急性面，字面只点了 `for`；该句末「so a user-defined sequence is a runnable source beside the builtin ones」可读作已覆盖急性面，本任务**不改它**（改则超出 T7-4 面，且需重新措辞 T7-B 的记录），只登记为**措辞歧义**。

---

**实现期补记十二（T8-1 落地时露出：发射形即决策 1 的直译、逐元素根四代突变不可观测、两处边界维持）**：

**提交**：`c1c8444`（`codegen.go` + `acute_test.go` + 三枚黄金 + docs 双语，7 files / +275 −24）。

**一、`collect` 的发射形是决策 1 的直译，六处协同而非一处新增。** `emitCollect`（`codegen.go:6689`）：`__we_list_new(i64 0, traced)` 起空载体 → 常驻根 push → 槽存指针 → `openWalk`（与六枚共用）→ 每轮 `__we_list_push(loadPtr(lst), w.word)` → 增长后根替换 → `closeListWalk`。结果**不是数值**，故 `callResult` 加 `list *listElem` 字段、`bindResult` 的 ckGc 臂把它注册进 `listEnv`——**与字面量绑定同一环境**，后续 `for`/组合子直读（`TestAcuteCollectResultWalksAgain` 钉的就是「cexit 的槽读寄存器正是第二走查的 snap 操作数」）。元素面走 `elemFaceOfType` 词汇表 ⇒ `List<String>` 源**上游即停**（真机负形确认），无新边界引入。

**二、增长带来两件字面量没有的事，纪律双双落死。** ① **载体移动**：push 翻倍增长返回**新身份**，而影子栈存指针值不存地址（gc.c 无保守栈扫）——故「pop 旧根 + push 新身份」成对发射、中间零分配，循环体出口的深度账不变；② **逐元素根**（仅 traced 面）：增长 push 内部的 `__we_alloc` 可触发收集，而本轮元素句柄只活在寄存器（`next` 尾声已弹构造根）——`wordPtr(w.word)` → `root_push` → push → `root_pop`，恰覆盖那一次调用。字面量路径**永免**此事（精确容量、永不增长，list.c 注释自陈留给后续任务），collect 是第一条增长 push 路径。

**三、逐元素根在本 build 不可观测——四代突变全部静默，机理已钉死。** 突变序：`s4big`（压力形）→ `align`（**对齐算术** `24M+96 < 2^20 ≤ 24M+120` ⇒ M=43686，插桩实证火点 sz=72 / acc=1048584）→ `align2`/`align3`（二段 pad 44000/95000）。全部输出逐字节正确。深挖三步：① 运行时 go:embed 烧入须重建组合二进制（we-mut 曾用旧 gc.c，假静默）；② 插桩清扫计数：两构建全收集清扫总数**恰差 1 块**（=被突变的 A1）；③ 火点活动根账 **4 对 3**（正确：Gen+CellIter+init list+元素根）。**机理** = 存储即时性（陈旧句柄在同一调用内、其间零分配，落进**有根有描记**的载体；此后每轮收集经描记遍历**复活**它）+ 芝诺竞速（空闲表排水速率 = 每弹 24B acc = 再触发周期，每次收集重建空闲表把 A1 洗回深部）。**判决**：与补记七第 8 条、补记十一第 4 条**同构**——保留纪律（gc.c 头部自陈的每 gc 引用生存期协议、零成本），**不冒充黄金层捕获**，Not-tested 里写明。

**四、两处边界维持 + 一处措辞留待。** ① 带参 `collect(1)` → `bndMainBody`（`want` 表 collect:0），负形单测钉；② `List<String>` 源上游停（负形真机确认）；③ `acute_test.go` 头部注释**六改七**随本提交，但 `TestAcuteOutsideTheSixStops` 注释里的「five lazy」**留给 T8-4 统一订正**——那是专项任务（grep 归零的验收面），本提交不沾染它的 diff。

---

**实现期补记十三（T8-2 落地时露出：内联链急性随臂可用而绑定形仍停、入口块回边之禁、守卫的分层与 Emit 辅助的管道局限）**：

**提交**：`795bc93`（`codegen.go` + `lazy_test.go` 新建 + `acute_test.go` 重锚 + 两枚黄金 + docs 双语，7 files / +889 −30）。

**一、惰性四枚的发射形是决策 2 的直译，链的归纳形落在盒上。** `emitLazy:11087`：`lazySource` 解接收者（`Dyn<Iterator<T>>` 盒）→ `elemOfShape(instShape(srcFace.Args[0]))` 定元素域 → 四臂分派（map 走 `e.siteOf(call)` 的检查期 site 定 U 并**重造结果面** `Iterator<U>`、头 `MapIter$T$U`、`emitFnArg` 造 fn 载体；filter 复用 `acuteCallback` 面、头 `FilterIter$T`；take/skip 走 `emitNumExpr`、头 `TakeIter$T`/`SkipIter$T`、参数字 `i64`）→ `mkLazyNext`（`head+".next"` 幂等入 `e.methods`，ABI 与协议 next 同为三字和）→ `ifaceSlots`/`vtableFor` 造表与 thunk → `emitLazyObject` 造适配器（源盒@16、参数@24）→ 结果盒（表@16、适配器@24）。**链 = 盒持盒持对象**，无任何快路径捷径——take 的适配器@16 持的是 map 的**盒**而非 list 对象（`TestLazyChainThreadsBoxToBox` 钉）。急性消费走 `emitAcute:6310` 的内联链臂：链臂结果直接进 `emitAcuteLoop` 的 `walkSrc{box: …}`，**走查机器零改动复用**——这是「内联链急性随之可用」的结构性理由，非测试巧合。

**二、filter 与 skip 各需一枚前置空跳块——LLVM 禁止分支指向入口块。** filter 的谓词失手回边进 dispatch、skip 的 `dec` 回边进 `check`，而 dispatch/check 是函数体首块即入口块；clang 实证「Entry block to function must have predecessors」（换块名无用——首块即入口，与名无关）。修法 = `entry: br label %loop` / `entry: br label %check` 空跳前置块。map/take 不需要：其 phi 引 entry 的边是**前向边**。教训入 `emitLazyNext` 模板注释。

**三、守卫分层：发射侧纵深防御不可达，真管道负形落在元素域。** 真机实证：`take()`/`take(1,2)` 停于**检查器** arity 词（"calls with an argument count the callee does not declare (spec gap; roadmap follow-up)"）、`take(1.5)` 停于 **E0501**——`emitLazy` 的 `len(Args)!=1 → bnd` 与 isF 守卫在检查管道下**不可达**，`lazy_test.go` 尾注说明不钉（钉不住：`checkShapes` 对检查失败自身 Fatalf）。负形真身 = String 源与 gc 元素源（`elemFace` 词汇表边界，与 T7-B 的第六面同一条），走真管道钉 bndMainBody。**裸 List 接收者**（`xs.filter(..)` 无 `.iterator()`）也停于检查器——但词是 **bndStdModules**（List 方法词汇属第 15 章 std 域），`xs.count()` 同停，故 `TestAcuteOutsideTheSixStops` 的裸 `Emit` 版是**环境性绿 + 主题错层**，随重锚整测退役；「five lazy」grep 因此归零，补记十二第四条预告的 T8-4 专项蒸发，D8 措辞义务关闭。

**四、`Emit` 测试辅助的管道局限披露。** 辅助不携带 Shapes ⇒ `ifaceSlots` 恒 false ⇒ 迭代器面（thunk 表）环境性停——与程序真假无关。故 `lazy_test.go` 全部 9 枚钉走 `checkShapes` + `EmitProgram(… Shapes: sh)` 真管道（文件头 doc 自述该理由）；既有 `Emit` 辅助的测试不受影响（它们的主题不过迭代器面）。

**五、docs 双语五次拓宽 + 关死清单补「绑定后过急性」。** 「four times」→「five times」+ 第五项（惰性四枚、盒持盒、元素工作在急性走查最外层盒处、Float64 以自身域穿行）；关死清单补一面：急性组合子过 `let` 绑定的惰性链停（`let it = xs.iterator().map(f)` 后 `it.count()`），而链自身绑定与内联急性调用可跑——该面即 T8-3（tasks.md 新增行），String 源/gc 元素源两关死面既登无需重写（「under a `for` or a combinator」与「reaching a boxed iterator」的口径已盖惰性面）。

---

**实现期补记十四（T8-3 落地时收口：dyn 写读点封闭集的安全论证、face (c) 的独立缺口与归属、负形的判别力）**：

**提交**：`e04c785`（`codegen.go` + `lazy_test.go` + 六枚黄金 + docs 双语，10 files / +214 −7）。

**一、dyn 写读点的封闭集是本臂安全论证的全部。** 新臂以 `dynRecvOf(fn.Recv)` 认绑定，而 `dyn` 只在**真发射过表**的构造上置位——写点封闭集恰三处：`emitBox:10851`（仅 dispatchable——表发射了才有面）、`iterBoxCore:10974`（T7-4 对象盒）、`emitLazy:11181`（T8-2 链盒）。**关键排除**：`emitMethodCall:14305` 经 `emitCallCore` 返回、**不设 dyn**——所以协议句柄绑定（`let it = r.iterator()`，经方法表的普通方法调用）不带 dyn 面，新臂结构上不会把句柄当盒走查。这条排除同时解释了 face (b)/(c) 的停点归属（见第三条）。

**二、名字判据有结构保证，不是字符串巧合。** `ShapeDecl{Kind, Node, Name}`：内建面 Node 空、Name 有值（"Iterator"）；用户面 Node 有值、Name 空。故 `face.Decl.Name == "Iterator"` 结构上只命内建 Iterator 面——用户自有面（哪怕方法叫 `count`）照走 T6 通用派发。`TestAcuteOverAUserFacesOwnCountDispatches` 是其最锐负形：用户 `Box` 面 + `Held.count` 槽经自有表派发调 impl 方法，IR 零走查；真机打印 `7`。

**三、face (c) 是独立缺口，非本臂遗漏。** 裸 `let it = xs.iterator()`（无惰性链、无显式盒）**绑定自身**停 bndMainBody（真机 exit 70）：检查器把裸 `xs.iterator()` 定型为**裸接口 `Iterator<Int64>`** 而非 Dyn 盒（无链盒、无 `Dyn<…>` 显式构造 ⇒ 无 dyn 面 ⇒ `dynRecvOf` 返回 false ⇒ 绑定自身落 bndMainBody）。emitAcute/emitLazy 也不认裸 `iterator` 名（那是 Iterable 协议方法，非组合子），故该停点在**检查器定型面**，不在发射器判据面——修它要么检查器把协议调用定型为盒（语言面取舍），要么发射器给裸接口绑定新发射臂（越出 D8 决策 2 的字面）。**不在 T8-3 任务行内，披露不修**；docs 双语关死句已改写为该形（T8-2 补登的「绑定后过急性」句随本任务关闭）。与协议形 `r.iterator()` 可绑定形成的不对称属同一事实的两面（后者同样不带 dyn 面、经 emitMethodCall 走方法表后绑定自身停）。

**四、负形的判别力教训：一次性契约下的「miss」两义。** any/all/find 黄金初版把命中与 miss 两次急性写在**同一绑定**上——第二次走查落在已耗尽的迭代器上（ch11 一次性契约：耗尽永久），miss-ok 会是**耗尽**的证据而非**谓词不命中**的证据，黄金判别力缺陷。落盘前自查改为独立绑定（`it`/`jt` 各持一盒）；单测层钉「链不被重复求值」（表恰一次）与该教训互补——前者钉绑定的**求值**语义，后者钉走查的**消耗**语义。

**实现期补记十五（T9 落地时收口：载荷面单管道闭合两簇、String 负例的双门冗余、next 面的判决与 docs 措辞、真机渲染字节）**：

**一、as-built：一条管道闭合簇 1 与簇 11。** 码面六件：`payloadFace(face)`（`codegen.go:6663`——排除 `skStr` 与未定面 `!face.gc && face.kind == skNone`，再经 `abiWordTypes(p) != 1` 一字校验）、`optionShapes(p)`（`:6678`——`[{None},{Some, pay:[p]}]`，序 = tag 算术 [None=0, Some=1]，与 codegen 自有 `variantShapes` 一致而与检查器 [Some,None] 相反——T7-A 第 ④ 条的既有事实，本管道沿用 tag 算术故自查无歧义）、`payRoundTrip`（`:6687`——三分读回：gc→inttoptr 出/ptrtoint 回、skF64→bitcast 双向、其余 i64 直过）、emitReduce rlater 按面发射（`:6927-6969`：载荷读出→double 域回调→位型存回）、emitFind 只换守卫（`:7016`——fhit 直存元素字，本来就把句柄/位型当字存，换守卫即通）、两处 sumSlot 挂 `variants: []string{"None","Some"}, shapes: optionShapes(p)`（`:6968`/`:7054`）。**消费端零新码**：臂绑定走既有 `bindArmPayload:7724` 表驱动路径——`shapeIndexOf` 定槽位、`bindArmWord` 按字面分派（abiI64 臂落 `scalarSlot{kind: baseStrKind(p.typ), num: narrowName(p.typ)}`，abiDouble 臂 bitcast + `isFloat`，abiGc 臂 inttoptr + gcEnv）；String 位消费同臂（簇 11）：`faceParam:6641` 的 typ（abiI64→`strKindName(face.kind)`）即 `emitHole` 渲染动词的来源。**D9 把两簇分开写、实现合在一条管道上**——「不得修两次」的字面兑现。

**二、String 负例是双门冗余，非单守卫功劳。** `List<String>` 的 reduce 停点有**两道门**给出同一个 bndMainBody：上游源元素面（T7-B 第六面，`elemFace` 词汇表）与 `payloadFace` 自身的 `skStr` 排除。单摘任何一道，另一道仍停——与 T7-4 的 `elem.gc`/`strKindName` 互为冗余先例同构。负例黄金 `build-bnd-acute-string-payload` 钉的是**净效果**（exit 70），不区分停在哪道门；这是「不越界放宽」的验收形，门的位置属实现细节。

**三、next 面的判决：docs 的 N2 句只关 reduce/find 两半。** 真机 p6–p10（新旧二进制**同停** ⇒ T9 前既有边界、非回归）：内建对象 `next`（直接 match scrutinee / 绑定后 match / 仅绑定）与用户 Iterator `next`（绑定后 match / 仅绑定）一律 exit 70——**停在绑定自身**，早于 docs 旧句描述的「读入 String 位」消费点。故 docs 双语关死句改写为「`next` 所产的 `Option` 停在绑定自身（`let o = it.next()` 即停），早于任何臂或读取够到它」——措辞按实测停点收缩，面不虚关（reduce/find 两半确实关了）。

**四、两枚真机字节事实。** ① `__we_str_of_f64` 把 4.0 渲染为 `4`（无尾 `.0`）——`run-acute-float-payload-face` 的 stdout `sum 4\n` 按真机字节落盘，非手推；② `we build` 不运行程序，翻绿 build 黄金的 stdout 恒空（沿 `build-err-payload-eager` 先例：exit 0 + stdout "" + stderr ""）。

**五、夹具层的引号陷阱（测试侧教训）。** `strLit(text)` 助手产 `Text: text` **无定界符**，而 `decodeStringLiteral`（`codegen.go:9429`）要求带引号文本——match None 臂的 `strLit("none")` 使三枚新单测 FAIL 于 bndMainBody（表面像 match 缺陷，实为字面量解码失败）。定位手法：`e.bnd()` 内 `debug.Stack()` 插桩 → 栈直指 `emitStringExpr` 的 Literal 臂。修法 = 三处改 `` strLit(`"none"`) ``（反引号内含双引号）；插桩全清（grep 验证零残留）+ `runtime/debug` import 移除。附带：Go builtin `println` 写 stderr，调试须 `2>&1` 捕获。

**实现期补记十六（T10 落地时收口：簇 2+7 的部分合并判决、负例的层次勘误、操作数位连带、blockKind 帧协议、突变电池两层判决、typed=false 不钉的理由）**：

**一、簇 2 与簇 7 的合并判决：部分合并。** D9 的勘查提示「汇槽若按携带面重做则两簇一并关」——as-built 是**两塔各一刀**：分类塔一刀合并（`joinKind` 新规 `if a == b { return a }; return skNone`——skStr 对放开、混合仍 skNone，簇 2 的帧分类与簇 7 的 join 判据在这一个函数上汇合）；发射塔簇 7 自有（`put` 的 ckStr 臂：首 String 臂预约 `strSlot/lenSlot` 双 store、`vf.slot != ""` 时再遇 ckStr 即 bnd；`ckI64` 首约时反向核对 `vf.strSlot != ""`）。汇槽**未**按「携带面」整体重做：`{slot, isFloat, unit}` 数值面与 `{strSlot, lenSlot}` String 面在同一个 valueForm 里互斥共存（两 face 混合的程序在检查层已被 E0501 拒绝，发射层的互斥核对是纵深防御）。**判决**：合并发生在判据层（joinKind），不发生在存储层（valueForm 结构未动）——存储层的重做无对应验收面，不做。

**二、负例的层次勘误：混合臂停检查器，非 codegen 边界。** 任务书的负例「分支不一致（数值 vs String）仍停」在真程序里停**检查器 E0501**（exit 1，`mixed types — the if arms are String and Int64; no coercion is ever inserted`）——不是 exit 70。语料此前**无任何 E0501 臂不一致黄金**（全语料机器扫描证实），补 `check-e0501-if-arm-disagreement`（exit 1 + 该报文）。发射层自身的 join 门由 AST 直构绕过检查器钉（`TestValueFormMixedArmsStillStop` → bndMainBody），`joinKind` 直调钉（`TestJoinKindCarriesAStringPair` 四断言）——三层各钉各的，落地记录按层次如实写。

**三、簇 10 的连带判决：操作数位一并通，docs 关整个 fn 值面。** `callStrKind` 的 fnEnv 臂不只开值串位——`arithKind` 经同一 `callStrKind` 分类，故 `f(1) + 1` 同通（真机 p7：exit 0 打 `3`）。docs 英文版的关死清单原列「a function value called in a value-string or operand position (`"${f(1)}"`, `f(1) + 1`; its statement and tail positions run)」**整项摘除**（两面皆通），不是只摘半项。

**四、blockKind 的帧协议。** `pushEnv()` → 逐顶层 `*ast.Binding`（`Pat != nil` → `patBoundNames` 逐名 `poisonName`——`scalarSlot{kind: skNone}` + `delete(strEnv)`；否则 `installBlockFace(b)`）→ `valueKind(尾表达式)` → `popEnv()`。**毒化的语义**：名字在块的帧里既无数值面也无 String 面 ⇒ 块尾读它时 `valueKind` 答 skNone ⇒ bnd——防的是「外层同名绑定泄漏进来被按外层面渲染」的静默错值（B1a T13 紧急缺陷的泛化收口）。String 面 = `strEnv[name] = strBinding{}` + `delete(scalars)`。

**五、fnEnv `typed=false` 是纵深防御，不钉。** 五个人口点（`:4836`/`:13947`/`:4444`/`:2016`/`:2256`）全 typed:true 或传播——`typed=false` 在本 build 的模块组合下**不可达**。强行钉须构造可达形，而构造它等于改人径（不是本变更的面）。循 B1a T6「双门冗余」先例：单测直调钉语义（`TestCallStrKindAnswersAFnValueByName`：seen 答 skI64、unseen 答 skNone），黄金层不冒充。

**六、突变电池 8 变体，两层判决表**（单点、逐枚还原、锚点 `count==1`；净树备份 `/tmp/we-t10/codegen.go.bak`）：M1（摘 pushEnv/popEnv）初轮**两层存活**——原遮蔽测试仍答对（installBlockFace 直接污染外层 env），真机上「块后再读外层名」的形产非法 IR（clang `expected value token` 拒收）——**真覆盖缺口**，修 = 黄金 `run-value-form-shadowed-block` 扩两 println + 新单测 `TestBlockKindLeavesTheOuterFrameAlone`，复跑两层齐红；M2（摘毒化）、M5（摘 put 的 ckStr 首约核对）两层齐死；M3（joinKind 恒返 a）两层齐死（黄金层是检查器 E0501 门、发射层由 AST 直构钉独守——分层如实）；M4（摘 var 初值面 switch）黄金层死、单测层存活 → 补 `TestVarWithAnUnannotatedInitializerBinds`；M6a/b/c（emitHole 元组 Ident 臂 / renderTuple 分隔符 intern / 转换器路由）各黄金层死、单测层存活 → 补 `TestTupleValueRendersIntoAStringPosition` 一枚覆盖三变体。**全部补钉至两层齐死才收**——B1a T8-2「两层齐活」纪律的兑现；M1 与 M4/M6* 的存活各暴露一枚真覆盖缺口，这正是电池的目的。

**实现期补记十七（T11 落地收口：①② 独立性双半实测、② 装箱定案与描记谓词、gc 压测黄金升级为构造函数形、M2 黄金层结构性静默的差分实测、突变电池两层终表、三处既有边界披露）**：

**一、①② 独立性双半实测——D9-4 边界栏的预测逐字兑现。** 勘查期断言「任一件都不单独关掉另一枚」，实现期两半都做了真机对账：① 半 = ①-only 树（`8be1f64`）上 `-list-elem` 探针仍 exit 70；② 半 = `fd2e263` 干净树 + 仅② 的六处功能编辑（`traces()` 谓词、装箱臂、`elemFaceOfType` 的 `abiStr` 臂、读回臂、`listElemWord` 守卫合并、`classType` 注释订正）构成②-only 树，其 `build-bnd-list-abi` 探针 exit 70 而 `build-bnd-list-elem` 探针 exit 0——与① 半恰成互补，两件工作的独立性不是推演而是两次实测。

**二、② 的表示形定案：32 字节无描述符盒。** 单字元素槽装不下 String 的两字对，取舍落在「宽槽」（改 `list.c` 的元素步长协议）与「装箱」（元素以句柄入槽）之间，**取装箱**，论据四条：(a) 载体协议一字不动——`__we_list_push(carrier, i64 word)` 处处照旧，`runtime/c/` 维持零改动（D7-附的零运行时姿势整条保住）；(b) 元素句柄的存活走的是 gc 记录元素的同一条路（描记载体 + `allocObj` 协议自带根账），无新 ABI 面；(c) 盒形 `{map@0=null, size@8=32, strptr@16, strlen@24}`——**map 恒 null 不是妥协而是精确**：String 的字节在 gc 域之外（str.c 的常量或 malloc 缓冲），盒内没有任何一个字是 gc 引用，而 null 正是「全零位图」的规范拼写（mark 阶段对 `blk_map==NULL` 直接过），全标量盒（T5）的先例同构；静态描述符方案（发一个 calloc 的全零位图全局）与 null 行为逐位等价却多一个 ABI 面，故弃；(d) `allocObj("null", 32)` 复用既有协议（`e.use`×2、`pushes++`），根协议白得。**载体描记谓词** `traces() = face.gc || face.kind == skStr` 一处定义、字面开口与 collect 两处同问——载体的字现在是句柄，故创建即描记（`list_new(i64 n, i64 1)`）。**读回协议**：走查的 `__we_list_get` 一字 → `inttoptr` 回盒 → gep16/gep24 两读；赋值名经 `bindStringSlot` 双槽（T10 的 String 绑定协议原样复用），未赋值名持 `strBinding{dataOp, lenOp}` 操作数对。**回调五枚仍停**：`listElemWord` 的入口守卫（`skStr || skNone → bnd`）——回调的参数得是两字 String 本身，盒句柄不是 String；count/collect 两枚不触元素，与载体一起开。

**三、gc 压测黄金升级为构造函数形（gc1 → gc4）。** 首版压测（gc1：100 元素 + 400 次丢弃 collect + 读回）净树绿、sweeps=1，但**盒的构造根是混淆变量**——字面量构造的每枚盒经 `allocObj` 协议扎根、`popRoots` 在**函数出口**才弹（`codegen.go` popRoots 自陈），main 体内构造的盒在整个压测期都是影子栈直接根，描记载体从不是其唯一存续理由。gc4 把构造挪进 `fn mk() -> List<String>`（① 的返回位渡口顺带实测：String 元素面随载体过签名）：`mk` 出口根弹尽，`ys = ms.iterator().collect()` 之后**描记载体是盒的唯一存活路径**——这正是 traced 位存在的理由。净输出与 gc1 逐字节同（`40000` + e00..e99），黄金期望字段不动，只换源。

**四、M2 黄金层结构性静默——差分实测把机理钉死。** M2（`traces()` 撤 skStr）四代压测（gc1 计数形 / gc2 读回形 / gc3 异串垃圾形 / gc4 构造函数形）输出全部逐字节净绿。插桩差分（同一 gc4、同一插桩 gc.c、净树 vs M2 树）：净树 `swept32=21300`，M2 树 `swept32=21400`——**恰差 100 枚 32B 块，即 ys 的百枚盒**。突变确实到达运行时：无描记载体下它们真的被回收了；输出仍不差一字是因为**回收块在回读前永不被重用**——首适合 + 分裂（≥16 余量即裂）让每次 32B 请求先被表头的大死块喂饱，清扫按「块龄chunk序 × 地址降序」重建空闲表把程序最老的分配（恰是 `mk` 的盒）排进扫描优先级的末段，而 1MB 阈值在前沿抵达之前就再次触发清扫重建链表；且回收只改写 map@0（空闲表穿线），读回只读 16/24——字节完好。**判决**：与补记七第 8 条、补记十一第 4 条、补记十二第 3 条同构——单测层双钉击杀（`list_new(i64 2, i64 1)` 与 `list_new(i64 0, i64 1)` 两枚 traced 钉），黄金层如实登记结构性静默，不冒充、不造刁钻夹具。

**五、突变电池终表（六黄金：② 自有五枚 + 新守门负例）**（单点、逐枚还原、锚点 `count==1`）：M1（撤 `listElemWord` skStr 守卫）单测杀 1 / 黄金**杀 1/6**——由新守门黄金 `build-bnd-list-elem-callback`（exit 70 + bndMainBody 报文）补杀，这同时是 D10「每拓宽面至少一正一负」的补课：② 的负例面（回调五枚停）此前零黄金锚定；M2 单测杀 2 / 黄金 0/6 结构性静默（见四）；M3（绕 `allocObj` 内联 alloc）单测杀 1 / 黄金杀 5/6（callback 面在到达 clang 前即停 70，不受影响）——**击杀机理如实标注**：撤 `e.use` 致 `@__we_alloc` 无 declare、clang 拒收 exit 1，是 declare 级击杀非 gc 语义级；M4（32→24）单测杀 1 / 黄金杀 4/6（run 面；build 面不回读故不察）；M5（16↔24 交换）单测杀 1 / 黄金杀 4/6。五枚突变至少单层击杀，无两层双活。

**六、三处既有边界披露（真机二分坐实与② 无关，如实入册不修）**：(a) **洞内急性**——急性组合子调用直接写在插值洞里（`"${xs.iterator().count()}"`）停 bndMainBody，Int64 列表同停（HEAD 二进制实测），绑定形（`let n = …; "${n}"`）则通——② 的全部夹具用绑定形；(b) **未绑定链式接收者**——`mk().iterator().collect()` 链式过签名返回值停 bndMainBody（gc4 夹具二分：链 70 / 先绑定 0 / 裸返回绑定 0），与 T8-3 face (c) 的披露同族；(c) **e7 装盒面**——`Dyn<Iterator<String>>` 构造开、`let o = d.next()` 绑定开、载荷读停（T9 的 payloadFace 排 skStr）、盒上急性停（elemOfShape 拒 String）——`iterSourceOf` 的 doc「a `List<String>` element is two words」句已随② 订正（元素字今是盒句柄，一字如常；下游面不随之开）。

**七、账目。** conformance 862→**867**（862 = ① 落地后的基数：T10 的 858 + ① 的四枚 `run-list-abi-*`；② +5 新文件——四枚 run 新黄金 `run-list-elem-walk`/`-param`/`-collect`/`-gc-crossing` 与守门负例 `build-bnd-list-elem-callback`；**两枚保名翻绿重锚**——`build-bnd-list-elem`（② 的正面）与 `build-bnd-builtin-iterator-string-elem`（T7-4 钉的「String 元素进盒停」，② 开其构造面，全套扫描捕获、真机复测 exit 0 后翻绿））；codegen 单测 427→**433**（427 = ① 的 `listabi_test.go` 七枚之后；② 的 `listelem_test.go` 六枚新钉；`listabi_test.go` 的 String 分类器钉重锚更名，枚数不动）；`run-list-elem-gc-crossing` 落为 gc4 形（红先在 HEAD worktree 复验：六枚于 `8be1f64` 全 FAIL）。

---

**实现期补记十八（T12 阻塞前置的查证结论：报告行是运行期错误文本，且运行期渲染机制已存在——`?` 复用 errReport 纪律，运行时零改动）**：

**一、查证结论：`errMsg`/报告行是运行期错误文本（用户可见的 stderr 输出），不是诊断位置。** D9 簇 6 登记的未决「`errMsg`/报告行的确切用途（诊断位置 vs 运行期错误文本）」收口为**后者**，证据链四条：

1. **`__we_fail` 的运行时行为**（`runtime/c/startup.c:42-47`）：`__we_fail(const char *line, long long len)` 的全部实现是 `if (len > 0) { fwrite(line, 1, (size_t)len, stderr); } exit(1);`——写恰 len 字节到 stderr 后退出。运行时里**零换行逻辑**：报告行末尾的 `\n` 是编译期拼进字符串的字节，不是运行时补的。
2. **startup.c 的头注释自陈入口契约**："the Err path never returns — the generated code calls `__we_fail` with the report line, which is written verbatim to stderr before exiting 1"——「逐字写 stderr」是规范陈述，不是实现巧合。
3. **errPanic 形已是运行期载荷先例**：`emitQuestion` 的 Err 块在 `slot.errPanic` 时发射 `inttoptr(loadNum(pay))` + `__we_task_fail(ptr)`——同一块里早有一条「错误载荷在运行期交给运行时」的路径。
4. **docs/spec 全域零命中** "report line" 与 `__we_fail`——报告行没有诊断语义的规范地位；它是 M6b 以来入口协议（`__we_main` 的 Result 面）的运行期表达。

**二、`errMsg` 的全部置位点（三点，grep 复核）**：字段声明 `codegen.go:1253`（`sumSlot.errMsg`，注释自陈 "a static Err report line (the `?` main tail writes it)"）；唯一读点 `:8798`（`emitQuestion` 的静态臂）；**唯一写点 `:9377`**（timeout scope 构造处：`sumSlot{variants: []string{"Ok","Err"}, errMsg: "error: TimedOut"}`）。任务书写「今日唯一置位点 `:8078`」是行号漂移，实测 `:9377`。

**三、重大发现：`__we_fail` 的发射点不止 emitQuestion 的静态路径——`emitTail` 的 Err 臂已有完整的运行期报告行机制，`?` 直接复用其纪律即可，运行时零改动。** `__we_fail` 在 `codegen.go` 的发射点共三处：

- `:8802-8804`（emitQuestion 静态臂）：`e.cstr("q", line, "\\0A")` 拼全局 + `__we_fail(ptr %cn, i64 len(line)+1)`——timeout 黄金 `run-conc-timeout-question` 钉住的字节面。
- `:12720-12721`（emitTail Err 臂·静态常量路径）：`constErrPayload` 判得载荷全字面量时，`line = head + payload + "\n"` 整体入 `errConsts` 全局，`__we_fail(ptr @name, i64 len(line))`——len 含 `\n`，与静态臂字节等价（`cstr` 的 `\\0A` 后缀就是那个 `\n`）。
- `:12732-12733`（emitTail Err 臂·**运行期渲染路径**）：`e.errReport(head, args)` 拼出 `(ptr, i64)` 操作数对后 `__we_fail(ptr %s, i64 %s)`——码内注释自陈 "renders at run time through the same value-to-String face interpolation uses"。

`errReport`（`:12810-12833`）的换行纪律：parts = [interned head] + 逐载荷 `emitHole` 的 strBind 对（`", "` 分隔）+ `[intern("\n"), "1"]`，`concatStr` 折叠成 `(ptr, len)` 对——**`"\n"` 是连接件（一个字面量字符段），不是运行时行为**。这就是 ② 的 main 侧要走的形。**由此推翻勘查期的一个备选方案**（「改 startup.c 加 fputc 换行」）：不需要，D7-附的零运行时姿势整条保住。

**四、② 的门与发射形（定案）**：

- **门函数两个，纯读 shapes、在 icmp/br 发射前调用**：`questionOkFace(shapes)`——`isResultShapes` + Ok 载荷各位 abiVoid 或恰一 abiI64（≤1 字；排除 String/gc/Float64/元组 Ok 面，静默错绑防护，诚实停）；`questionErrLine(shapes)`——`isResultShapes` + `len(shapes) == 2`（恰一 Err 变体）+ 载荷空（纯静态名，走 `errConsts` 同形的静态路径）或恰一 `[abiStr]`（运行期 String）→ `(name, strPayload, ok)`。多变体 E 或非 String 载荷在 **main 侧**停（报告行无处取名）。
- **main 侧 err 块第三路**（插在 errPanic 与 errMsg 静态两臂之后）：`head = "error: " + name`（有 String 载荷加 `": "`）；parts = `[intern(head)]` +（若有 String 载荷）`[wordPtr(loadNum(pay)), loadNum(pay1)]` + `[intern("\n"), "1"]`；经提取的 `errFold` 助手（`concatStr` 折叠，`errReport` 改用之——行的组装纪律单点化）得 `(ptr, i64)`；`__we_fail(ptr, i64)` + `unreachable`。**errPanic 与 errMsg 两臂字节不动**——timeout 黄金的 stderr 逐字节保住（实现前取基线、实现后复跑确认）。
- **fn 侧**：门 = `e.exit != nil && e.exit.kind == exitFn && e.exit.abi.ret == abiSum && questionOkFace(slot.shapes)`。**位点的排除面须按三类分说**（勘查尾段核验证伪了「`e.exit` 门结构性排除闭包」的初稿表述）：task 位点（`:9238` 置 `exitTask`）与 main（`:7632` 置 `exitMain`）被本门**结构性排除**——`kind != exitFn` 即不进；**闭包位点不被本门排除**（`:4681` 的闭包发射段同样置 `exitSite{kind: exitFn, abi: abi}`，四处 exitFn 赋值点 = 闭包 + 三处 define），闭包内 `?` 的不可达由**检查器**保证——E1202 只许返回 Result 的 fn 用 `?`，而闭包无法声明 Result 返回（带注解 E0105 / 带值 return E0402，P10/P10b 真机实证），故发射面上这枚门在闭包位点**逻辑上不可达**、非结构性不可达，如实分说。err 块 = tag/pay/pay1 三字 `loadNum` → `insertvalue` ×3 成 `{i64,i64,i64}` → `unwind(0)` → `drainDefers()` → `popRoots()` → `ret { i64, i64, i64 } <cur>`——**不设 `diverged`**（`?` 是 mid-expression 出口，ok 路径继续发射）。安全性两笔账：`popRoots`（`:5511-5516`）**只发射 pop 指令、不重置 `e.pushes`**，err 块弹根不污染 ok 路径的编译期根账；E 一致性信任检查器（fn 声明返回 `Result<T,E>` 且 `?` 作用于 `Result<T,E'>` 时 `E'` 不同形属检查器 E1202 的裁定面，codegen 不比对 `slot.shapes` 与 `retShapes`）。
- **fn/main 不对称（披露）**：fn 侧三字整体传播，**不需要 Err 行门**——多变体 E、非 String 载荷的 `Result` 在 fn 侧照样可用；main 侧才受报告行词汇表约束。
- **调用点核查**：`emitQuestion` 仅两处调用（`:2121` 绑定初值、`:2356` 语句位），`?` 不经 emitNumExpr/值位路由——`return f()?` 形今日诚实停在他处，不属本任务面。闭包 `?` 语法不可表达（带注解 E0105、带值 return E0402，P10/P10b 真机复跑），fn 侧放开纯是 codegen 的事。

**五、③ 的门与发射形（定案）**：`fitAbi` 的 `case abiPrim` 拒臂（`:13300-13309`，注释自陈 "retTyp would go unset... the two guards agree, and the T9-3 battery records that either alone holds"）改为设 `abi.retTyp = "ptr"` 放行 + 注释改写；`fnRetOperand` 补 `abiPrim` 臂（`Ident ∈ e.prims` → `"ptr " + p`；内联 `Call` → `emitCall` → `res.kind == ckPrim` → `"ptr " + res.i64`；`!hasValue`/其余 → `bndFn()`）。**调用侧零改动**：`emitCallCore` 参数臂 ckPrim（`:14961-14965`，`"ptr "+res.i64`）、绑定 `:2292`（`e.prims[s.Name] = res.i64`）、参数绑定 `:14138`（`e.prims[p.Name] = "%" + p.Name`）三处已活。**负例**：prim 的值面（p7/p8 形：prim 值直接进算术/插值）仍停——本簇只关返回位。

**实现期补记十九（T13 落地收口：timeout 结果型的语言面裁定、簇 12 的 timeout 二分发射、无死绑定消除的机制订正、SHADOW 黄金形、突变电池四判决）**：

**一、阻塞前置的裁定——语言面，本变更零改动。** `docs/spec/1800-concurrency.md:231` 明写：普通与 `collectAll` 形的值是体的块值（第 2 章），`timeout` 形的值是 `Ok(b)`/`Err(TimedOut)`——**按规范即 `Result`**，与普通形的结果类型**真的不同**。D13 的未决项由此收口：`if r == 7` 上的 `E0501 mixed types`（`operands of "==" are Result<Int64, TimeoutError> and Int64; no coercion is ever inserted`）是检查器判对，黄金 `check-e0501-scope-timeout-value` 钉住该面。q2 探针找到的**真残缺口**是另一处：对该 `Result` 作 match（`match r { Ok(v) => … }`）过检查（exit 0）却在构建停 `bndMainBody`（exit 70）——消费面未开，登记 roadmap follow-up **#25**，不属本变更。

**二、簇 12 的发射 = `emitScope` 尾求值块按 timeout 二分。** 普通与 `collectAll` 形：尾在**体语句之后、体环境内**求值（D10-2 的「出口值非入口值」由求值位置保证——尾在 pushEnv/popEnv 之间、在 `for` 体循环之后）；数值尾作 `ownFace`（`ckI64` + `typeName: strKindName(valueKind(尾))` + `num`），`__we_scope_leave` 后**早退 `ownFace`**——经 `bindResult` 既有值塔，算术/比较/插值/fn 体绑定面**免费**获得；String 尾 `emitStringExpr` 绑对子；记录尾落 `default → e.bnd()` 诚实停（黄金 `build-bnd-scope-value-gc-tail` 守卫）。`timeout` 形：保持三槽 `Ok/Err` 和式（`bodyVal` 载荷字 + tag + `pay1=0`），`errMsg` 面不变——T12 的 `?` 静态臂消费的正是它。

**三、机制订正：无任何死绑定消除（tasks.md 验证行的断言被直读 IR 推翻）。** 未被读取的 `let r = scope { 7 }` 照发 `__we_scope_enter`/`__we_scope_leave`，数值尾喂进伪和式的载荷字故「结果为 0」；String 尾不问读取与否在发射处即停。「锚必须读取」的黄金设计结论仍成立，但理由是**不读取则无可观测输出**，非「绑定被消除」。

**四、SHADOW 黄金形的设计理由。** `run-scope-value-faces` 的体形是外层 `let n = 10` + 体内 `let n = 3` + 尾 `n + 4`：若回归把尾挪回体之前（D10-2 复辟），该形产出**合法 IR 的错值 `v 14`**（读到外层 n）而非停——`n + 4` 里的 `n` 在体先行的世界里未绑定、但影子形让它有界可错。这是 M4 突变黄金层打出的正是错值的事实证明。

**五、突变电池四判决（单测层 = `scopeval_test.go` 六钉，黄金层 = 四枚 T13 黄金）。** M1（摘早退恒建和式）：两层齐杀——两枚正面黄金 exit 70；M2（摘 skStr 臂）：String 钉 + String 黄金杀，数值面存活——恰沿该臂的分界，是设计预期非覆盖缺口；M3（摘 typeName/num 携带）：**两层齐杀**（钉①自带 `"v "` 插值消费，故比预案「单测层存活、黄金层杀」更强一层）；M4（尾先于体）：两层齐杀，黄金打出 `v 14` 对 `v 7` 的错值。

**六、docs 双语第十次→第十一次拓宽。** 「仍关死」清单摘「普通 `scope { … }` 值形」、补登「读 `timeout` Result 的臂」面（`match r { Ok(v) => … }` 过检查停在构建——本窗口真机复证 check 0 / build 70）。

**实现期补记二十（T14 收口：四条字面预测的最终对账、词表清退的真实终态、两塔共享词面、docs 分写、10 枚非 codegen 黄金零字节改动）**：

**一、四条字面预测全部被实现推翻或收窄，逐条处置（删除线 + 理由，不以静默修正）。** ① proposal 验收边界句「语料中不再有任何一枚黄金以 **codegen 边界行**停住」——被本设计自家的 D10-2 一正一负政策否决：每簇的负例黄金正是停在 codegen 边界行上的钉子（B1b 终态 11 枚负例守卫在册，见下条 grep 对），验收句以 D0 的「14 枚锚定黄金全部翻绿」为准；② tasks ③「预期 **< 5 词**」——终态 **5 词**（`bndMainBody`/`bndFnBody`/`bndTaskBody`/`bndGenericFns`/`bndTopLets`），每词仍有活的产出点（见二）；③ tasks ①「grep 归零」——基线 `0030d60` 为 13 文件、HEAD 为 11 文件，11 文件皆 D10-2 负例守卫；④ tasks ④「清空」——只在**原义**上成立：B1b 起跑时清单载着的六面（泛型、普通 scope 值形、main 体 `?`、元组值读入 String 位、N1、N2）**全部离场**，与今清单内容零重叠；今清单载着的是拓宽**自家揭出**的诚实余面。

**二、词表清退的真实终态（逐词产出点清点，`grep -o` 计数须减去定义行 1）。** `bndMain()` 15 处生产点（main 体余面：`?` 两门、同步对象读入 String 位、`String` 元素源上的回调五枚、gc 源进盒、盒载荷读、盒上急性、timeout Result 臂读、next Option 臂读——即 11 枚负例守卫的面）；`bndFn()` 45 处（fn 体内同样的值面族 + 各自的守卫层）；`bndTask()` 2 处（`:1180`/`:5565`，task thunk 捕获超出标量集）；`bndGeneric()` 28 处——其中 `:793` 是 `collectImpl` 的**真拒绝**（impl 头实参不是自身参数的顺序排列，如给实例化点名）而非 Eq 域残余，余 27 处为 `:8321`–`:8722` 的 Eq 域残余；`bndTopLets` 1 处直产（`:7340`，顶层绑定注解 kind==skNone 且 valueKind(Init)==skNone 的不可定面）。**`bndGenericFns` 是两塔共享的词面**：检查塔在 `internal/typecheck/typecheck.go:57` 持有同串常量、于 `:2798` 的 `assertEqualCall`（`!inEqDomain` 时经 `c.bnd`）产出——本塔的该行与那一行**同退同留**，D0 的「typecheck 常量不动」即由此兑现。词表历史注释（`codegen.go` 头部）已按此补 T14 终审段。

**三、N1/N2 的归属句（docs 双语已落）。** N1（fn 值在值串位/操作数位的调用）由**值塔的 fnEnv 臂**关掉（T10 `fd2e263`）——语句位、尾位与 `"${f(1)}"`、`f(1) + 1` 今四位并列；N2（内置所产 `Option` 载荷读入 String 位）由**载荷面簇**关掉（T9 `c0b3851`）——`reduce`/`find` 按元素自身的面作答；`next` 的绑定自身其后亦开（T11），停点移到**读该 `Option` 的臂**（`match o { Some(v) => … }`）。

**四、docs 双语的分写结构。** 「仍关死的面」改为**两种分开写**：codegen 停面八枚（原披露内容逐字保留）→ 工具面停面一枚（裸句柄绑定——成因明写**在检查塔**：检查器把调用定型为裸 `Iterator` 接口而非盒的擦除面，是 codegen 拓宽不拥有的语言面取舍，D11 的「分开写」义务兑现）→ 运行期规则一条（**不是边界**，只是未变）。六面离场句 + N1/N2 归属句随行；收尾句「子集本身随 codegen-full 线落地收缩」被改写为「那条线已落地；仍关死的是拓宽自家的发现」——原句在收口后即为陈朽。

**五、10 枚非 codegen 停面黄金零字节改动（D11 的机械复验）。** `build-single-file`/`run-single-file`/`build-bnd-test-module`/`build-bnd-test-fns-first`（CLI `whatSingleFileBuild`）、`build-library`（`whatLibraryArtifacts`）、`check-bnd-std-member`/`check-stdio-shadow-let`（`bndStdModules`）、`check-bnd-method-arity`（`bndArityGap`）、`check-assertequal-residual-bnd`（typecheck `bndGenericFns`——本窗口对 `0030d60` 复验 diff 为零行）、`run-conc-deadlock`（运行期诚实死锁）：全部维持原期望字节。

---


## D7-附 运行时姿势：三要素的第三支

D5/D6/D7 合起来是一条**零运行时新增**的路：盒是 `__we_alloc` + 自写 map 字（冻结的 M4 ABI，`gc.c:243-258`）；vtable 与 thunk 是发射出的 IR，vtable 是静态常量、连分配都不经；迭代器对象是同一种 gc 块；协议循环只用既有 `sumSlot` 三槽与 `armTagTest`。**故 `runtime/c/` 在本变更是零改动**——这是刻意选的姿势，理由有三：

1. **冻结 ABI 的复用面已经够了**：`__we_alloc` + 影子栈 + 编译期描述符这套（ADR-0002/0003 冻结）本来就是为「编译器自造对象」设计的，`list.c` 之所以要运行期合成描述符，只因**容量是运行期值**；盒与迭代器对象的形全是编译期事实，不需要那条路。
2. **运行期不做编译期能做的事**：派发表是具体型到实现符号的映射，两边都是编译期知识；搬进运行期就要在 C 里重建一张符号表，并给它一套装载/初始化纪律（`__we_gc_boot` 一类）。这是把权威从 codegen 挪到 runtime，与本变更「一类事实只有一个权威」的整条线相反。
3. **不新增符号 = 不新增 ABI 面**：M4 的冻结面每加一个符号就要走一次 ABI 冻结记录；本变更的验收不需要付这个代价。

**这不是「运行时无需设计」**——恰恰相反，本节的结论是设计出来的：它把三支里最容易被忽略的一支（运行时）**显式地判为零改动**，并给出为什么零是对的。**唯一的例外口**是 D7 决策 3 的前置查证（内建迭代器类型的载体）：若查证落到「std 侧声明 + 内建 impl」而内建 impl 的 `next` 需要一个运行期构造器（例如持有 String 游走的 `StrIter`），那就在此补记该符号并说明发射器直发为何不足（同 `list.c` 的姿态：把一个只有运行期才知道的量交给运行期）。

---

## D8 组合子：从 6 枚到 11 枚

**事实**：`docs/spec/1100-iterables.md` §Iterator combinators 把家族钉死为**惰性四枚**（`map`/`filter`/`take`/`skip`，结果型 `Dyn<Iterator<…>>`，「performs no element work at the call」）与**急性七枚**（`collect` → `List<T>`，加 `fold`/`reduce`/`count`/`any`/`all`/`find`，「advances the receiver to exhaustion at the call」）。检查器分得对：`typecheck.go:1234-1263` 里四枚走 `dynIter`、`collect` 走 `listSum`。codegen 的注释写错了（`codegen.go:5529-5532` 把 `collect` 划进「the five lazy ones」并给它一个它没有的 `Dyn` 结果型），B1a 的归档工件沿用同一说法。

**决策**：分两件工作，**不混**——

1. **`collect` = 第 7 枚急性**（与 Dyn 无关）：结果型 `List<T>`，发射形同既有六枚的**变体**——不是「累加进一个 i64 结果槽」，而是「边遍历边 `__we_list_push`」。元素面走既有 `elemFaceOfType` 的词汇表；元素面装不进单字槽的情形（String/和式/元组）与 `build-bnd-list-elem` 同一条边界，须一并处理（§D9 簇 4）。
2. **惰性四枚**：结果型是 `Dyn<Iterator<U>>`，**硬依赖 D5/D6 的盒与 vtable**，且需要 `Iterator<T>` 的方法表（`next` 是唯一的非默认方法，`typecheck.go:1104-1323`）。`filter` 的谓词、`map` 的函数参数都是纯 fn 值（无 effect 段，`E1402` 守门），走既有 `acuteCallback:5709` 的 fn 值面。

**并须订正**：`codegen.go:5529-5532` 的家族注释（`collect` 归急性、惰性族四枚）、`docs/` 与 roadmap 里沿用的「五枚惰性」措辞。

---

## D9 单态余面十三簇

逐簇给发射面与边界。拒绝行均以 HEAD `0030d60` 为准。

| 簇 | 决策 | 边界 |
| --- | --- | --- |
| **1 急性载荷面**（`-acute-float-payload`、`-acute-gc-payload`） | 放开 `codegen.go:5841`/`:5926` 的 `scalarWordFace:5666` 守卫：`Option<E>` 的载荷字按元素的**真面**携带（Float64 按 double 位型、gc 载荷按句柄），match 臂绑定按同一面读取 | 既有的「载荷跟着元素面」规则不放宽到多字（String/元组仍是边界，与簇 4 同源） |
| **2 值位块帧**（`-value-form-emission-binding`、`-value-form-shadowed-binding`） | `blockKind:9167` 的分类器改为读**块自己的帧**而非 match 臂已还原的外层帧（`emitArmBlock:4490-4521` 已把臂帧弹出） | 遮蔽绑定必须仍以**块内**的声明定面——`str_test.go:430-450` 钉的就是「拿外层帧分类会把内层 `5` 走 bool 转换器」这个错，不得复发 |
| **3 用户 Iterable 作源**（`-list-iterable`、`-acute-user-iterable`） | 见 D7 | — |
| **4 List 载体边界契约**（`-list-abi`、`-list-elem`） | **两件独立工作**：① ABI 敷设——`classType:10215` 放行 `List<T>`，`bindTupleParams:10534` 让载体（ptr）与元素面进参数槽，`fnRetKind:9265`/`fnRetOperand` 同步；② 元素宽度——单字元素槽装不下 String 对，需**宽元素表示**（两字槽或装箱），`emitListElemValue:5406`/`emitNumOperand:2093` 同步 | ① 不做②则 `-list-elem` 仍停；② 不做①则 `-list-abi` 仍停——**任一件都不单独关掉另一枚**（勘查已证，①② 双半独立实测见补记十七）。宽元素的表示形是本簇的核心取舍，**已落：见补记十七** |
| **5 惰性组合子**（`-acute-lazy`） | 见 D8 | 依赖 D5/D6 |
| **6 `?` 的用户 Result**（`-m6b-main-body`） | `codegen.go:7500` 的 `slot.errMsg == ""` 是「非字面 `Err` 无法给出报告行」（`errMsg` 只在 `:8078` 被置——行号已漂移，实测 `:9377`）。决策：让 `?` 的早退不依赖**字面**消息——错误值本身随 Result 传播，报告行由其载荷在运行期取得 | fn 体侧 `:7475` 的直拒同步放开。**实现期须先查证 `errMsg`/报告行的确切用途**（是诊断位置还是运行期错误文本），再定放开的形——本行是方向，不是定案。**已查证，见补记十八**：报告行 = 运行期错误文本；运行期渲染机制（`errReport`）已存在，`?` 复用其纪律、运行时零改动 |
| **7 值位 String 臂**（`-value-form-string-arm`） | `valueForm` 的汇槽从「数值集专属」（`put:4476`、`joinKind:9181` 排除 `skStr`）扩到能承载**两字的 String 对**，并让两个 String 臂可 join | 与簇 2 同属 `valueForm` 家族：**若汇槽按「携带面」重做，簇 2 与簇 7 可一并关掉**（勘查提示，实现期评估是否合并） |
| **8 无标注 `var`**（`-var-unannotated`） | `codegen.go:2720-2722` 要求 `*ast.NamedType`；改为**从初值定面**（String 两字 vs 标量一字必须先定，再发初值）——即把 `valueKind` 家族（T13 补过臂）接到绑定面 | 无标注且初值面不可定的情形仍停（诚实边界） |
| **9 同步型返回位**（`-prim-return`） | `fitAbi:10477` 的 `case abiPrim: return abi, false` 放开：ch18 同步型需**返回拼写 + 调用侧结果面**（`fnRetOperand` 补 `abiPrim` 臂） | 同步型的**值面**（不是返回位）在本变更之外——本簇只关返回与消费 |

**另四簇：黄金账上没有，但 `docs/benchmarks.md` 的「What remains closed」列着**（该段末尾原文即「the set itself shrinks as the **codegen-full** line lands」——本变更被文档点了名，故这四簇是硬范围）：

| 簇 | 形 | 说明 |
| --- | --- | --- |
| **10 fn 值的值位调用**（N1，B1a T14 发现，**零黄金**） | `"${f(1)}"`、`f(1) + 1` 停；语句位与尾位跑得通 | 界在**调用所在的位置**，不在捕获——`valueKind` 家族对 fn 值调用返回 `skNone`，`emitHole:9287` 等消费者据此停。修法与簇 8 同族（分类器补面） |
| **11 内置 `Option` 载荷读入 String 位**（N2，B1a T14 发现，**零黄金**） | `reduce`/`find`/`next` 产出的 `Option` 载荷，数值消费跑得通、读进 `"${…}"` 停 | 与簇 1 **同源**（载荷面），差在消费端是 String 位而非 match 臂——两簇应一并设计，否则会把同一条面修两次 |
| **12 `scope` 值形**（**零黄金**） | `let r = scope { … }` 一旦被读取（数值比较/算术/String 位）即 exit 70；~~未被读取时整个绑定被消除故 exit 0（三轮探针实证）~~ **订正（T13 直读 IR）**：无任何消除——未被读取照样发 enter/leave，数值尾喂进伪和式载荷字故结果为 0；String 尾不问读取在发射处即停 | **措辞已核实准确**（B1a 归档期的失准假设被推翻，见 proposal 的 F3）。`scope timeout(N)` 形的结果型另有一处 `E0501` 存疑——见 D13（**已收口：语言面，补记十九第一节**） |
| **13 元组值绑定读入 String 位**（**零黄金**） | 元组绑定后读进 `"${…}"` 停 | 与 `emitTupleElemValue:8561` 的元素面相关；B1a 的 T13 补臂时已把该调用点列入「以 `skNone` 为界、会放宽但发射端重查」一类 |

**这四簇的共同点：语料零锚定。** 本变更须为它们**新增**锚定黄金（先红后绿），否则「归零」就没有机械证据——B1a 的 T14 正是靠探针发现的它们，而探针不留档即无法复验。

**红证据（真机探针，HEAD `0030d60`，`/tmp/we-b1b/we`）**——四簇逐条实测，且**文档措辞逐字准确**：

| 探针 | exit | stdout | 说明 |
| --- | --- | --- | --- |
| `let f: fn(Int64)->Int64 = \|v\| v+1` 后 `io.println("${f(1)}")` | 70 | — | N1 值串位停 |
| 同上后 `let r = f(1) + 1` | 70 | — | N1 操作数位停 |
| 同上后 `let r = f(1)` + `"${r}"` | **0** | `2` | **语句位跑得通**——与文档「its statement and tail positions run」逐字相符 |
| `xs.iterator().reduce(\|p,q\| p+q)` 数值消费 | **0** | `ten` | N2 数值面跑得通 |
| 同一结果读进 `"${v}"` | 70 | — | N2 值串位停 |
| `let r = scope { 7 }` 后 `if r == 7` | 70 | — | 簇 12 被读取即停 |
| 同上但 `r` 从不被读 | **0** | `done` | **整条绑定被消除**——B1a 归档期以为文档失准，就是踩了这个 |
| `let t = (1, "a")` 后 `"${t}"` | 70 | — | 簇 13 停 |

（探针另记一枚无关噪声：元组元素**不是**`.0` 拼写，`t.0` 报 `E0105`——与本题无关，只是别把那条当簇 13 的证据。）

---

## D10 conformance 与黄金策略

**决策**：

1. **14 枚锚定黄金逐枚翻绿**，每枚留**先红后绿**的真机证据（探针项目 + 边界行），记入完成记录。
2. **拓宽面一正一负**：每一簇至少一枚正面黄金（跑通并逐字节定 stdout）+ 一枚**负例**证明边界没被放得过宽（B1a 的 T13-B 同法）。
3. **泛型面与 Dyn 面的孪生**：检查面已有 17 枚 `check-ch10-*`/`check-m6a-*` 与 14 枚 `check-*-dyn*`，其中多数**零 build/run 孪生**。本变更为每个可运行形补孪生（不是全补——纯诊断面不补）。
4. **`WE_UPDATE_GOLDEN=1` 不得使用**（黄金全部手写）。

**边界**：非停点黄金**零触碰**；被翻绿的 14 枚逐枚披露（对照表入完成记录）。

---

## D11 披露与 not-implemented 边界

**决策**：10 枚非 codegen 停面（proposal 表）**逐枚维持原期望字节**。其中 `build-single-file`/`run-single-file`/`build-bnd-test-module`/`build-bnd-test-fns-first` 的黄金名里带 `bnd`，但它们的边界行是 CLI 侧的（`whatSingleFileBuild`），**不是** codegen 词——本变更不动，且在 `docs/benchmarks.md` 的措辞里把「codegen 停面」与「工具面停面」分开写。

`docs/benchmarks.md` + `.zh.md` 的「What remains closed」清单：B1a 刚把它从 4 面扩到 6 面（含 N1/N2），本变更把它**清空**，并在此处记明 N1（fn 值调用在值串位停）与 N2（内置 Option 载荷读入 String 位停）分别由哪一簇关掉。

---

## D12 验证阶梯

AGENTS.md:66 / process §4 的完整阶梯：`go build ./...` + `go vet ./...` + `gofmt -l` + `go test -count=1 ./...` + 受影响黄金 `-count=1` 取数 + `validate.py --all --strict` + `docs_sync.py` + `git diff --check` + staged 无 `refr/`。

**另加**（本变更特有）：

- **边界行 grep**：语料全域扫 codegen 边界行，翻绿前后各一次，作为验收句的机械证据。
- **突变电池**：按 B1a T13/T14 的同法，对每簇的守卫落单点突变（逐枚还原、锚点先断言 `count==1`），单测层 + 黄金层各记判决。
- **IR 面**：泛型实例化的符号可读性（D2）与 vtable 全局的形（D6）各留 IR 钉。
- **回归面**：B1a 的 226 处 `e.bnd()` 产出点中，凡本变更触碰到的，逐处核对「该停的仍停」（负例黄金兜底）。

---

## D13 风险、未决与已知缺陷

- **Q2 是本变更最大的未知**：登记面的形（D1）若在实现期被证明不足（例如某形要求检查器给出比 `Shape` 更多的知识），须停下补记，**不得**让 codegen 就地复刻推断。
- **`scope timeout(N)` 的值面**（探针发现）：`let r = scope timeout(1000) { 7 }` 后 `r == 7` 报 `E0501 mixed types`。`scope` 值形本身是 `bndMainBody` 停点（探针：未被读取时 exit 0、被读取即 70），但 `timeout` 形的**结果类型**与普通 `scope` 不同这点须在实现期查证——若真不同，它是设计问题（语言面）而非实现缺口。**登记为未决项**。**已收口（T13，2026-09-15）：语言面**——规范 `1800-concurrency.md:231` 明定 `timeout` 形的值为 `Ok(b)`/`Err(TimedOut)`，`E0501` 是检查器判对；残缺口是对该 `Result` 的 match 消费面（check 0 / build 70），登记 roadmap follow-up #25。详见补记十九。
- **`check-assertequal-residual-bnd` 的两塔词面共享**（D0）：本变更只记账不改动。Eq 域闭包是 typecheck 塔的裁定，若用户裁定放宽域门，那是另一个变更。
- **D3 的不动点上界守卫**：不落守卫就可能无限循环——列为硬要求。
- **`refr/` 禁区**：本变更的任何提交不得纳入。
