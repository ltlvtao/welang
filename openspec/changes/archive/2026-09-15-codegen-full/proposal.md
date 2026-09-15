# proposal — codegen-full（B1b）

> 状态：candidate 草案（勘查已完成，待 spec-impact audit）

## Why

B1a（`codegen-mono`）把单态值塔立起来了：conformance 701 → 810，语句集、表达式值形、字符串全链、记录/元组/newtype、闭包与 fn 值、List 载体与六枚急性组合子、顶层绑定、sum 三槽 ABI 全部落地，另修掉 follow-up #16 的支配缺陷。但它没有把 codegen 的未实现集归零——那是 B1 的另一半，本变更。

**今天仍然存在的停点，语料侧是 14 枚黄金**（真机复测，HEAD `0030d60`）：

| 停点词 | 枚数 | 黄金 |
| --- | --- | --- |
| `bndMainBody` | 12 | `build-bnd-acute-float-payload`、`-acute-gc-payload`、`-acute-lazy`、`-acute-user-iterable`、`-list-abi`、`-list-elem`、`-list-iterable`、`-m6b-main-body`、`-value-form-emission-binding`、`-value-form-shadowed-binding`、`-value-form-string-arm`、`-var-unannotated` |
| `bndFnBody` | 1 | `build-bnd-prim-return` |
| `bndGenericFns` | 1 | `test-fn-generic-bnd`（模块收集期的 `codegen.go:591`/`:600`：泛型 fn 与泛型 impl 方法） |

`bndTopLets` 与 `bndTaskBody` 零黄金锚定（产出点仍在源码里，`bndTopLets` 另有 `:6033` 一处）。**语料之外**，`e.bnd()` 仍有 226 处产出点、`bndGeneric()` 32 处（1 定义 + 4 真泛型 + 27 处 assertEqual Eq 面残行的复用）。

**一枚勘误（勘查期抓到，B1a 归档期的读法有误）**：`check-assertequal-residual-bnd` **不是 codegen 停点**。虽然它印出的是 `bndGenericFns` 的措辞，但产出点是**检查器自己**——`internal/typecheck/typecheck.go:2776` 的 `assertEqualCall` 在 `!inEqDomain(got)` 时调 `c.bnd(bndGenericFns)`（词常量声明在 `typecheck.go:57`），而 `internal/cli/check.go` 根本不 import codegen。即：**一个词面同时被两座塔持有**，B1a 的 T13 只 grep 了 codegen 侧的产出点，把这一枚记成了 codegen 停点。本变更的零集按**塔**划分，故它归 typecheck 塔（见「不在零集内」）。

**主/函数体的报词次序**（design 阶段会依赖）：main 体在 `codegen.go:1266-1289` 发射，**早于** fn 定义段 `:1291-1295`——这就是为什么列表/组合子/m6b 那批报的是 **main** 词，而 `build-bnd-prim-return` 报 **fn** 词。

这 14 枚不是同一种东西。按**依赖方向**分三类：

1. **单态余面**（10 枚，`bndMainBody` × 9 + `bndFnBody` × 1）：List 的 String 元素面、`List<Int64>` 与 `conc.Mutex<Int64>` 这类**具体**泛型应用进参数槽与返回槽、无标注 `var`、用户方法 `Result` 上的 `?`、值位控制形三形状（块内绑定 / 遮蔽绑定 / String 分支）、急性组合子的 Float 载荷与记录载荷。**这些都不需要新机器**——是把既有发射面按同一套规则再放宽一格。
2. **泛型单态化**（1 枚黄金 + 17 枚 `check-ch10-*`/`check-m6a-*` 只有检查面、零 build/run 孪生）：`fn id<T>`、泛型 impl/method、泛型 newtype 在模块收集期直报 `bndGenericFns`（`codegen.go:1117`、`:1142`、`collectImpl:589`、`:599`）；泛型 record/sum **被收集**却在每一处使用点静默拒绝（`layout:9549`/`:9570`、`emitConstruct:9640`、`namedSumShapes:10286`、`recordKeyOf:5502`、`recordKeyOfType:6130`、`isSumType:10421`、`opaqueKeyOf:10714`、`emitFieldChainString:8894`、`emitVarBinding:2721`、`chanElemRecord:2864`、`listElemFace:5430`，共约 20 处 `Args != 0` 判据）。**`where` 子句在 codegen 里零触碰**（全文件无 `.Where`/`WhereBound`/`TypeEq` 命中）。codegen 无实例化表、无名字改编、无逐实例化发射——`fnDef.sym()`（`:737`）里没有类型实参位。
3. **Dyn 派发与迭代器协议**（3 枚 + 14 枚 `check-*-dyn*` 只有检查面）：检查器**已完整实现** `Dyn<...>`（`dynType:261`、`dynFace:7095`、`dynCall:7173`、`implementsFace:7713`、`memberOfType:6873`），**codegen 侧零支持**——`classType` 没有 `"Dyn"` 臂，全文件无 `Dyn` 盒、无 vtable、无标签；`emitCall` 的 ident 分支（`:3009`）没有 `Dyn<Ifc>(v)` 构造面，直落 `e.bnd()`。运行时**没有任何 vtable/方法表/派发机制**，唯一描述符先例是 gc 冻结头的 map 字（`gc.c:5-9`、`:173-186`）与 `list.c:64-77` 的运行时合成。用户 `Iterable<T>` 的 `for` 停在 `emitFor:5015`；5 枚惰性组合子被 `acuteCombinator:5533` 显式排除（注释 `:5529-5531`），`map/filter/take/skip/collect` 的检查器签名是 `Dyn<Iterator<T|U>>`——**惰性组合子依赖 Dyn 盒**，这是本变更内部的硬序。

按**能力簇**（哪一件工作关掉哪几枚）分九组，这是 tasks.md 的直接骨架：

| 簇 | 黄金 | 拒绝点（`internal/codegen/codegen.go`） |
| --- | --- | --- |
| 1 急性载荷面 | `-acute-float-payload`、`-acute-gc-payload` | 同一对守卫 `:5841`/`:5926` + 同一谓词 `scalarWordFace:5666`——`Option<E>` 载荷字要按元素真面携带，而非重解释成 i64 |
| 2 值位块帧 | `-value-form-emission-binding`、`-value-form-shadowed-binding` | 同一对 `blockKind:9168` → `emitHole:9369`——分类器读的是 match 臂已经还原掉的帧（`emitArmBlock:4490-4521`），而它该读块自己的帧 |
| 3 用户 Iterable 作源 | `-list-iterable`、`-acute-user-iterable` | 同一能力，两个首拒点：`emitFor:5015` 与 `emitAcute:5559`→`emitCall:3105` |
| 4 List 载体边界契约 | `-list-abi`、`-list-elem` | **同一子系统、两件独立工作**：ABI 敷设（`classType:10215`→`bindTupleParams:10534`）与元素宽度（单字槽装不下 String 对，`:5374`→`emitListElemValue:5406`→`emitNumOperand:2093`）。任一件都不单独关掉另一枚 |
| 5 惰性组合子 | `-acute-lazy` | `acuteCombinator:5533` 排除 → `emitAcute:5559` → `emitCall:3105`；需 Dyn 面 |
| 6 `?` 的用户 Result | `-m6b-main-body` | `:7500`（`slot.errMsg == ""`；`errMsg` 只在 `:8078` 被置）——非字面 `Err` 的 `?` 需要报告行/错误面；fn 体侧另有 `:7475` 直拒 |
| 7 值位 String 臂 | `-value-form-string-arm` | `joinKind:9181`（排除 `skStr`）+ `valueForm.put:4476`（汇槽数值专属） |
| 8 无标注 `var` | `-var-unannotated` | `:2720-2722`——存储面（String 两字 vs 标量一字）必须先定，再发初值 |
| 9 同步型返回位 | `-prim-return` | `fitAbi:10477`（`case abiPrim: return abi, false`）→ `emitFnDefine:11031` → `bndFn`——ch18 同步型需返回拼写 + 调用侧结果面（`fnRetOperand` 无 `abiPrim` 臂） |

簇 2 与簇 7 同属 `valueForm` 家族（一个把脸板做宽即可同时关掉），但作为守卫是两条不同的线；簇 4 是唯一一个「两枚同源却须两件工作」的。**泛型簇**（`test-fn-generic-bnd` + 17 枚无孪生的检查面）与 **Dyn 簇**（3 枚 + 14 枚无孪生）在上表之外，各自是一整条塔。

**规范锚点**（本变更零 spec 增量，因为要兑现的面全部已在册）：泛型的声明/引用/推断/`where` 四件在 `docs/spec/1000-interfaces.md`（§Generic parameter declarations、§Generic type references and inference、§Where clauses、§No variance, no subtyping）；`Dyn` 盒的构造形与派发语义在 §Dyn values（含 "The interface MUST be an interface"、"Construction is exactly one form, the explicit one"、"Dyn boxes are gc values … no copy of the boxed value occurs" 三条硬约束）；迭代器协议与组合子在 `docs/spec/1100-iterables.md` §The Iterator interface / §Iterator combinators / §The Iterable interface / §The for-loop protocol；`List<T>` 载体在 `docs/spec/1700-collections.md` §The collection types / §Snapshot iteration；同步型在 `docs/spec/1800-concurrency.md` §The shared-state types。**检查器逐条已实现，缺的只是发射面。**

**一枚规范对读发现的措辞缺陷（本变更须一并订正）**：`1100-iterables.md` §Iterator combinators 把家族钉死为**惰性四枚**（`map`/`filter`/`take`/`skip` → `Dyn<Iterator<…>>`）与**急性七枚**（`collect` → `List<T>`，加 `fold`/`reduce`/`count`/`any`/`all`/`find`）——即 **`collect` 是急性的、返回 `List<T>`、不需要 Dyn**。检查器分得对（`typecheck.go:1234-1263`：四枚走 `dynIter`，`collect` 走 `listSum`）。但 **codegen 的注释写错了**：`codegen.go:5529-5532` 称「the five lazy ones (map/filter/take/skip/collect) whose Dyn<Iterator<U>> results belong to B1b's vtable face」——把 `collect` 划进惰性族并给了它一个它没有的 `Dyn` 结果型。B1a 的归档工件沿用同一说法（其「非目标」也写五枚）。**订正落点**：codegen 注释、本变更的 design、以及 B1a 归档工件的对外引用（归档件本身不改，按惯例以本变更的记账为准）。对工作量的影响：`collect` 是**第 7 枚急性组合子**（直发循环构 `List`），与 Dyn 无关。

**架构主轴（B1a 的 Q2 发现，本变更必须处理）**：今日检查器知识全部瞬时——`Check()`（`typecheck.go:1867`）返回即弃、checker struct 全私有、AST 零注记（`ast.go:8-10` 自陈 pure data）、codegen 全部自行重推。而泛型实例化的类型实参是检查器**逐调用点推出来的**（`inferValueArgs:7477`、`unifyInto:7382`、`methodCall:7643` 的逐参 `substMap` 线程）。codegen 要么复刻这套推理（直接违反单一权威原则），要么由检查器把实例化登记交出来。**`Check()` 出口扩形是本变更的地基，不是可选项。**

**验收边界（黑盒）**：~~语料中不再有任何一枚黄金以 **codegen 边界行**停住~~（订正（T14 对账）：该句被 design 自家的 D10-2 一正一负政策否决——每簇的**负例黄金**正是停在 codegen 边界行上的钉子，B1b 终态 11 枚负例守卫在册；验收句以 D0 为准），即上表 14 枚全部翻绿；`e.bnd()` 的 226 处产出点与 `bndGeneric()` 的 32 处随簇逐面清退——**清退是随行，不是验收句**（产出点与语料锚定数不等价，见 design D0；T14 终态：5 词全存活、逐词理由见 tasks.md T14 落地记）。

**验收面不止这 14 枚。** `docs/benchmarks.md` 的 run-face「What remains closed」段列着**六面**，并以「the set itself shrinks as the **codegen-full** line lands」**点了本变更的名**——其中**四面在语料里零锚定**（B1a 的 T14 靠探针发现、未留夹具）：`scope` 值形、元组值绑定读入 String 位、fn 值的值位调用（N1）、内置 `Option` 载荷读入 String 位（N2）。**这四面的归零是本变更的硬范围**，且必须先**新增**锚定黄金（先红后绿），否则归零没有机械证据。见 design 的簇 10–13。

**不在零集内的 10 枚 exit 70**（B1a 已裁定、本变更维持；按塔列出，附各自的持有者）：

| 枚 | 持有者 | 阻塞工作 |
| --- | --- | --- |
| `build-single-file`、`run-single-file`、`build-bnd-test-module`、`build-bnd-test-fns-first`（4） | `internal/cli/build.go:21` `whatSingleFileBuild` | 单文件编译无 manifest ⇒ 工件无名（follow-up #6 规范缺口） |
| `build-library`（1） | `internal/cli/build.go:22` `whatLibraryArtifacts` | ch21 库工件 |
| `check-bnd-std-member`、`check-stdio-shadow-let`（2） | `internal/typecheck/typecheck.go:51` `bndStdModules` | ch15 标准库模块装载（归 B2 `stdlib-real`） |
| `check-bnd-method-arity`（1） | `internal/typecheck/typecheck.go:59` `bndArityGap` | 调用实参数目的接受/强制面（follow-up #5 规范缺口） |
| `check-assertequal-residual-bnd`（1） | `internal/typecheck/typecheck.go:57` `bndGenericFns`，`:2776` 产出 | 复合型的 Eq 域闭包（`record Point` 未声明 `derives Eq`）——**typecheck 塔**的词，非 codegen 停点 |
| `run-conc-deadlock`（1） | `runtime/c/sched.c:682-684` | **不是边界**：构建并运行后由运行时诚实死锁中止（`we: deadlock: 3 tasks parked with no wake source`）；exit 70 是与未实现停点共用的 `EX_SOFTWARE`（`internal/cli/cli.go:30`），stderr 无「not implemented」行 |

## 目标与非目标

目标（一条线：codegen 的未实现集归零）：

1. **单态余面退役**（10 枚锚定 + 4 面无锚定）：List 的 String 元素面、具体泛型应用（`List<T>`/`conc.Mutex<T>` 等合格具体型）进参数槽/返回槽/字段槽、无标注 `var` 的分类、用户方法 `Result` 上的 `?`、值位控制形三形状、急性组合子的 Float 与记录载荷、ch18 同步型的返回位；**外加 benchmarks 文档列明而语料零锚定的四面**——`scope` 值形、元组值绑定读入 String 位、fn 值的值位调用（N1）、内置 `Option` 载荷读入 String 位（N2）。**目标是不引入新机器**——同一套发射规则再放宽一格（唯一的例外是 `scope` 值形，其存储面可能要新形，见 design 簇 12）。
2. **检查器注记面**（Q2，地基）：`Check()`/`CheckTestRoot()`/`CheckProject()` 出口交出 codegen 需要的实例化知识（每调用点/构造点的类型实参绑定、关联型绑定、接口的非默认方法槽序）；codegen 停止自行重推。**形由 design D1 定**：出口是一个封闭的只读类型视图，**不加 AST 注记**——注记是 proposal 期的候选，design 期被拒（`ast.go:8-10` 的 pure-data 性质保留）。
3. **泛型单态化**：`{声明, 类型实参} → 符号` 实例化表 + 名字改编；fn/record/sum/newtype/impl/impl-method 六类声明的逐实例化发射；列表元素面、layout/shape/class/键面、参数槽、返回槽、`chanElemRecord` 的约 20 处 `Args != 0` 判据按实例化后的具体形作答。**`where` 的判定仍归检查器**（design D4：codegen 侧保持零命中），codegen 只消费实例化后的具体形。
4. **Dyn 盒与 vtable**：盒的 gc 布局、`Dyn<Ifc>(v)` 构造面、经 vtable 的动态派发。**两项 proposal 期的写法被 design 否决**：盒内**无类型标签**（`Dyn` 无下转型面，标签没有消费者，D5 决策 2）、描述符**恒为编译期常量**（`list.c:64-77` 的运行期合成是因为容量是运行期值，而装箱点的具体型是编译期事实，D5 决策 4）。
5. **迭代器协议与用户 `Iterable`**：`for` 源放宽到任意 `Iterable<T>`（先做接收者静态已知的形状），用户 `Iterator<T>`/`Iterable<T>` 的 impl 面。
6. **组合子补全到 11 枚**：惰性的**四**枚 `map/filter/take/skip` 返回 `Dyn<Iterator<…>>`（**硬依赖 4**）；急性的 `collect` 返回 `List<T>`（第 7 枚急性，直发循环，**与 Dyn 无关**）——见「规范锚点」末段的措辞订正。
7. **停点词表退役**：五词的产出点随各面实现逐面清退。**验收句不是「词表为空」**——词表产出点与语料锚定数是两件事，见 design D0：验收句是 14 枚锚定黄金全部翻绿，词表随产出点消失而**逐词**退休（某词的最后产出点消失即该词退休）。D0 那句设计期预测（「停点集合恰为 `{bndGenericFns}`」）已被 B1a 的实现推翻并挂到本变更（T13 裁定 1）。

非目标：

- **10 枚非 codegen 停面**（上表）：不属于 codegen 的未实现集，本变更零触碰。其中 `check-assertequal-residual-bnd` 是**检查器自己的词**（`typecheck.go:2776`），把它归零是 Eq 域闭包的裁定（typecheck 塔 + 可能的 spec 增量），与本变更无关；但**词面共享**这件事本变更要记账——词表清空时两座塔各自的持有者是谁，见 design。
- **Map/Set 真数据结构**（B2 `stdlib-real`）：ch17 未批构造方法面，维持检查面即诚实。
- **检查器的主体重构**：Q2 是**出口扩形**（新增返回面 + 必要的节点注记），不是把 checker 拆开或把类型系统搬到 codegen。
- **优化**：一切发射以正确性为先（物化数组、拷贝快照、盒不做逃逸分析）；性能归 B2 调优。
- **CLI/工具面、LSP、formatter**：零触碰。
- **spec 触碰**：零 Requirement 增删、零诊断码、零 ADR——本变更是**已批语言形**在参考实现里的兑现（泛型与 `Dyn` 都是 22 章已落的面，检查器早已实现），不是语言行为变更。「不改变语言行为」。

## What Changes

- `internal/typecheck`：`Check()`/`CheckTestRoot()`/`CheckProject()` 出口扩形（实例化登记表；**零 AST 注记**，形见 design D1）——**本变更的地基，第一步落地**。
- `internal/codegen`：实例化表与名字改编；六类声明的逐实例化发射；约 20 处 `Args != 0` 判据按实例化形放宽（**`where` 仍不进 codegen**，判定归检查器）；`Dyn` 盒构造 + vtable 派发；`emitFor` 的 Iterable 源；`acuteCombinator` 扩到 11 枚（惰性四枚经 Dyn 迭代器协议、`collect` 作第 7 枚急性循环直发）；值位分类器与发射的余面（String 元素面、具体泛型应用、`?`、无标注 `var`、值形三形状、急性载荷、同步型返回位）；`codegen.go:5529-5532` 的组合子家族注释订正。
- `runtime/c`：**零新增**——盒、vtable、thunk、迭代器对象全走冻结的 M4 ABI（`__we_alloc` + 影子栈 + 编译期常量描述符），不新增运行期符号（design D7-附 的运行时姿势）。
- conformance：14 枚停点锚定黄金逐枚翻绿（含**先红后绿**记录）+ 泛型面与 Dyn 面的新增黄金（检查面已有 17 + 14 枚，缺 build/run 孪生）。
- benchmark：M15 运行面的「What remains closed」清单重锚（`docs/benchmarks.md` + `.zh.md`）——B1a 刚把它从 4 面扩到 6 面，本变更把它清空。
- docs：roadmap B1b 行收口、follow-up 划账（归档期）。

## 影响层

- **spec：零**。本变更是纯实现：泛型单态化与 `Dyn` 派发都是 22 章已批的语言形、检查器早已实现，缺的只是参考实现的发射面。「不改变语言行为」。（B1a 归档期曾疑 `check-assertequal-residual-bnd` 会牵出 spec——勘查证明它是 typecheck 塔的词，不在本变更范围内。）
- **compiler**：`internal/typecheck`（`Check()` 出口扩形——本变更的架构主轴）+ `internal/codegen`（B1b 主面）+ `runtime/c`（**预期零改动**，见 What Changes）。既有 E 码行为零改写。
- **benchmark**：M15 运行面「仍关死」清单的清空与重锚。
- **docs**：`docs/benchmarks.md` 双语、roadmap 双语（归档期）。

## 影响范围

- **涉触码**：`internal/codegen/codegen.go`（主面）、`internal/typecheck/typecheck.go`（`Check` 出口与实例化登记的出口面）、`runtime/c/*.c/.h`（**预期零改动**；若 D7 的内建迭代器载体落到「运行期构造器」一形则在此补记并说明发射器直发为何不足）、`internal/conformance/testdata/`（14 枚重锚 + 新增黄金）、`docs/benchmarks.md` + `.zh.md`、`docs/roadmap/0000-reference-implementation.md`(+`.zh.md`)。
- **既有黄金零静默改写**：14 枚翻绿逐枚披露（对照表入完成记录）；非停点黄金零触碰。
- **对外零影响**：CLI 面、诊断码面、LSP、formatter、`we doc` 零触碰。
- **`refr/` 禁区**：本变更的任何提交不得纳入 `refr/`。

## 勘查依据

三份并行勘查 + 真机探针（HEAD `0030d60`，探针二进制 `/tmp/we-b1b/we`）：

1. **泛型端到端清单**：检查器侧（`Check` 出口、`subst`/`substMap`、`inferValueArgs`、`openRefs`/E0827、`where` 的两段式）、AST 侧（零注记）、codegen 侧（4 处真拒绝 + 27 处词复用 + 约 20 处静默 `Args != 0`）、无单态化机器、std 合成 AST 面、黄金与单测面。
2. **Dyn/接口派发清单**：检查器完整实现 vs codegen 零支持、运行时无派发机制、最近邻是 fn 值载体 `{fnptr@16, env@24}`（`makeFnValue:4259`、`emitFnValueCall:4232`）与方法表的静态直派（`emitMethodCall:11937`，doc `:11931-11936`）。
3. **残停面归因**：24 枚 exit 70 的逐枚落点、拒绝行、能力簇（上表九簇 + 文档列明而语料零锚定的四簇 = 十三簇），并钉死两处归档期误读——`check-assertequal-residual-bnd` 属 typecheck 塔、`run-conc-deadlock` 不是边界。
4. **真机探针**（三轮）：`scope` 值形——未被读取时 exit 0，一旦被读取即 exit 70（`bndMainBody`），即 `docs/benchmarks.md` 的「a plain `scope { … }` read as a value」措辞**准确**（B1a 归档期怀疑其失准的假设被推翻）；`scope timeout(N)` 取整数字面（`1s` 是 E0006 拼写错），其值参与 `==` 时 E0501——归 design 阶段细究。

## 审计记录

**2026-09-12，candidate → ready，`welang-spec-impact-audit` 七条，通过（两枚发现已当场修正）。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | 问题真实性 | ✅ 黑盒可验证：HEAD `0030d60` 真机复测，语料 810 枚中 24 枚 exit 70，其中 **14 枚**以 codegen 边界行停住（逐枚落到具体拒绝行，见上文九簇表 + design 的簇 10–13）。规范锚点逐章引用（ch10 §Dyn values / §Generic …、ch11 §Iterator combinators、ch17 §The collection types、ch18 §The shared-state types）——**且这些面全部已在册**，本变更兑现而不修改。 |
| 2 | 影响层声明 | ✅ `change.yaml` `layers: [compiler, benchmark, docs]` 与 `## 影响层` 逐条一致。触及 compiler 但**不改变语言行为**，proposal 已写明该边界（「## 影响层」spec 条 + 「非目标」末条）。 |
| 3 | 规范增量范围 | ✅ **零 Requirement 增删、零诊断码、零 ADR**。不新增 E/W 码：泛型与 `Dyn` 的码段 `E0800`–`E0899` 已在册且检查器已实现，本变更不开新码位，故不触碰 `docs/spec/diagnostics.toml`。 |
| 4 | 原则一致性 | ✅ 无冲突。相关条：P2 唯一语义（发射面兑现已批语义，不改语义）、P9 单一错误机制（诊断面零增删）、P10 完成度闭环（本变更**就是**闭环动作）、以及「一类事实只有一个权威位置」——Q2 注记面恰恰是**消除**一处潜在违反（codegen 若自行复刻检查器的实参推断，即第二处权威）。无原则突破 ⇒ 无需 ADR。 |
| 5 | 参考基线固定 | ✅ 全部引用为仓内路径 + 行号（`internal/codegen/codegen.go`、`internal/typecheck/typecheck.go`、`runtime/c/*.c`、`docs/spec/*`），随 HEAD `0030d60` 固定。**未引用 `refr/`**（禁区）；无外部项目引用。 |
| 6 | 验收边界 | ✅ 机械可判：判据是「语料中是否仍有黄金以 codegen 边界行停住」（对 `internal/conformance/testdata/cases/*.json` 的 `stderr` 做边界行匹配，14 枚全部翻绿即零集达成）。非目标显式排除 10 枚 exit 70 并**逐枚给出持有者**（表见上文），足以防止蔓延。 |
| 7 | 粒度 | ✅ 通过，**但这是七条里唯一有争议空间的一条，理由记录如下**。审计条文的判据是「是否需要**同时**改动三个**不相关层**才能验收」：本变更的验收（14 枚翻绿）只需要 `compiler` 一层——`benchmark`/`docs` 是同一变更的披露随行（B1a 同形），不是验收前提；三个能力簇（单态余面 / 泛型 / Dyn）同属 compiler 一层且构成一条垂直面（检查器注记面 → codegen 实例化与派发 → 运行时盒/vtable ABI），不是三个不相关层。体量上：B1a 实测约 30 个任务单元、新增 109 枚黄金；本变更预估 25–30 个任务单元、14 枚翻绿 + 约 30 枚新增，同量级。**反方意见如实记录**：泛型单态化（实例化表 + 名字改编 + 六类声明逐实例化发射）与 Dyn 盒/vtable 是两块可以各自独立落地、独立验收的机器，按「最小可垂直验证单元」的严格读法亦可拆。本变更**选择不拆**的理由是验收判据本身是单一的跨簇性质（「未实现集归零」），拆开后每一块的 done 都不是目标；且仓内已有先例（T7/T8 的子任务在拓宽中被追加，未拆变更）。**若实现期某簇暴露出与其余簇的锁步要求（例如注记面的形被 Dyn 反向推翻），届时按 B1 → B1a/B1b 的同一条路拆，并由承载的变更改 roadmap 行。** |

### 发现与修正（审计期当场落地）

- **F1（措辞缺陷，已修正）**：`codegen.go:5529-5532` 与 B1a 归档工件的「非目标」均把组合子家族写成「五枚惰性（含 `collect`）」。规范 `1100-iterables.md` §Iterator combinators 钉死为**惰性四枚**（`map/filter/take/skip`）+ **急性七枚**（`collect` 加六枚），`collect` 返回 `List<T>`、**不需要 Dyn**；检查器分得对（`typecheck.go:1234-1263`）。已在本 proposal 的「规范锚点」段与目标 6 修正，并列入 `## What Changes` 的 codegen 注释订正项。
- **F2（归因缺陷，已修正）**：B1a 归档期把 `check-assertequal-residual-bnd` 记为 codegen 停点（T13 只 grep 了 codegen 侧产出点）。实际产出点是**检查器自己**——`typecheck.go:2776` 的 `assertEqualCall`，而 `internal/cli/check.go` 不 import codegen。已改归 typecheck 塔，codegen 零集由 15 枚修正为 14 枚，非零集由 8 枚修正为 10 枚。**该枚仍是一笔债**（一个词面被两座塔持有），本变更在 design 里记清两塔各自的持有者。
- **F3（B1a 归档期假设被推翻，如实记录）**：归档期怀疑 `docs/benchmarks.md` 的「a plain `scope { … }` read as a value」措辞失准（探针一里 `scope` 值未被消费时 exit 0）。三轮探针证明**措辞准确**：`let r = scope { 7 }` 只要被读取（数值比较 / 算术 / 读进 String 位）即 exit 70，未被读取时整个绑定被消除故 exit 0。docs 该行**无需改动**。

## 审查记录

**2026-09-12，ready → active，`welang-change-review` 十条，通过（四枚发现，三枚当场修正、一枚补章节）。**

结构面：`validate.py codegen-full --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`（改动前与改动后各一次）。

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | proposal 职责边界 | ✅ **但有一处实质问题（R1）**：proposal 期写的若干**具体机制**被 design 期否决后未回改。逐处订正见 R1。 |
| 2 | spec 增量边界 | ✅ 本变更**无 spec 增量**（`## 影响层` 首条已声明「不改变语言行为」并给出理由：泛型与 `Dyn` 都是 22 章已批形、检查器早已实现）。无增量即无越界面。 |
| 3 | design 的路径唯一性/最小性/替代方案 | ✅ 唯一路径；被拒替代方案逐条在册：渲染串与 `Type` 导出（D1）、hash 改编（D2）、把三内联 for 形改写为协议形（D7 决策 1）、盒内类型标签（D5 决策 2）、运行期描述符合成（D5 决策 4）。引用精确到文件:行与规范节。**与 spec 增量无矛盾**（无增量）。 |
| 4 | tasks 只执行前三者已定义的工作 | ✅ 15 组逐条挂在 design 节或 proposal 目标上；**无 deferred / non-goal / 未决方案选择**——四枚未决一律以**阻塞前置 checkbox** 形式出现（T5/T7/T12/T13 各自首项），符合第 10 条而不掩盖。 |
| 5 | 场景覆盖 | ✅ 每簇一正一负；失败/退化面另在册：D3 的不动点上界（无限实例化）、簇 6 的 `?` 错误路径、T15 的突变电池。纯 happy path 的面不存在。 |
| 6 | 无空章节 | ✅ 逐节有内容；`D7-附` 为 R3 新增（非凑格式，见下）。 |
| 7 | 测试先行 | ✅ **补强一枚（R4）**：T1–T3 是基础设施（characterization = 既有 76 例 + 810 枚零漂移，已在各验证行写明）；行为面自 T4 起，T4 首项现为「红先行」——把 14 枚的红态固化为目标测试基线。四枚零锚定簇（D9 簇 10–13）各自以「先新增锚定黄金取红态」开项。 |
| 8 | 负向断言 | ✅ 逐簇有真实负例（不是文字）：簇 1 的「多字载荷仍停」、簇 7 的「分支不一致仍停」、簇 8 的「面不可定仍停」、簇 9 的「值面仍在外」、T13 的「锚必须读取绑定」（未读时绑定被消除，锚会假绿）。另有全局负断言四条（T15 末项）：`WE_UPDATE_GOLDEN=1` 未用 / staged 无 `refr/` / 无新诊断码 / `ast.go` 与 typecheck 塔零改动。 |
| 9 | 完成度闭环（检查器 + 代码生成 + 运行时三支） | ✅ **补一节（R3）**：运行时一支此前散落在 D5 的事实段里。现补 `design D7-附`，**显式判定运行时零改动**并给三条理由（冻结 ABI 复用面已够、运行期不做编译期能做的事、不新增符号即不新增 ABI 面）与唯一例外口（D7 决策 3 的前置查证）。 |
| 10 | 未决问题阻塞 | ✅ 四枚未决各自是所在任务组的**首个 checkbox**，明写「未完成不得勾选其后各项」；标注处为 T5（resource 进盒纪律）、T7（内建迭代器类型的载体）、T12（`errMsg`/报告行语义）、T13（`scope timeout(N)` 的结果型）。 |

### 发现与处置

- **R1（proposal/design 矛盾，已修正——本审查最实质的一枚）**：proposal 里有五处**具体机制**是 proposal 期的预测，design 期被否决后未回改，形成同一变更内两份互相打脸的文本：①「必要时给 AST 节点加注记」（D1 明确**零 AST 注记**）；② 盒布局「标签 + 载荷 + 方法表指针」（D5 决策 2 明确**无标签**）；③ 方法表描述符「合成（先例 `list.c:64-77`）」（D5 决策 4 明确**编译期常量**，`list.c` 那条路的成因是容量为运行期值）；④「`where` 子句首次进入 codegen」（D4 明确**保持零命中**）；⑤ `runtime/c`「新符号族」（D7-附 判定**零改动**）。**处置**：五处逐处订正，并在原处注明「被 design 否决、去向见某节」——不静默改写。**成因反思**：proposal 是在勘查完成、design 未开写时落笔的，其中若干句是从 B1a 归档件的预测里继承下来的；**这正是 B1a 的 D0/D12 两次预测落空的同一失效模式**（计划期的机制预测被实现期推翻）。故本次订正保留原文痕迹，供下一次对账。
- **R2（验收句冲突，已修正）**：proposal 两处写「词表在 B1b 完成时为空」/「退役的终态是词表为空」，与 design D0 的「验收句取 14 枚锚定黄金、不取词表为空」直接冲突。**处置**：两处均订正为「清退是随行，不是验收句」，并点明产出点与语料锚定数不等价（今日 `bndTopLets`/`bndTaskBody` 即零锚定而产出点尚存）。
- **R3（三要素闭环，已补章节）**：审查条 9 要求语言特性类变更同时给出检查器/代码生成/运行时三支方案。检查器（D1）与代码生成（D2–D9）在册，运行时一支此前只在 D5 的事实段里出现（`__we_alloc`/影子栈/描述符协议）。**处置**：新增 `design D7-附 运行时姿势：三要素的第三支`，把「运行时零改动」从**事实**升为**决策**，给出三条理由与唯一例外口。
- **R4（测试先行，已补强）**：T4 原来的首项直接是「翻绿重锚」，红态靠黄金的既有失败隐含。**处置**：T4 增首项「红先行」，把 14 枚的红态显式固化为目标测试基线（exit code + stderr 行 + `-count=1` 红态计数）。

### 审查记录（2026-09-15，T15 实现完成后复审）

**`welang-change-review` 十条，通过（一枚措辞级发现，按先例处置；审查期零 tracked-file 改动）。**

结构面：`validate.py codegen-full --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`（实现完成态复跑；`--all --strict` 同绿）。status 已为 `active`（2026-09-12 ready→active 审查所置），技能产出的「通过后将 `status` 更新为 `active`」一步 N/A——本轮是**实现完成后**的复审。实现期全程：29 枚提交（`c2032c6`..`378dd0d`，基线 `0030d60`）、conformance **810 → 881**、codegen 顶层单测 **321 → 445**、typecheck 新增 `shape.go`/`shape_test.go`、`runtime/` 零 diff。

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | proposal 职责边界 | ✅ 黑盒验收句（14 枚锚定黄金全部翻绿）已兑现；T14 对账把「语料不再有任何黄金以 codegen 边界行停住」的过强原文删除线订正为「终态 11 枚负例守卫在册」（D10-2 一正一负政策），:51 订正在案；R1 订正的五处机制预测无新增未回改。 |
| 2 | spec 增量边界 | ✅ 零增量机械实证：`git diff 0030d60 --stat -- docs/spec/diagnostics.toml internal/ast/` 为空——诊断码与 AST 双零触碰（T15-③ 命令取证）。 |
| 3 | design 路径唯一性/最小性/替代方案 | ✅ D0–D13 + D7-附 14 节全在；被拒替代逐条在册（D1 渲染串与 `Type` 导出、D2 hash 改编、D5 决策 2 无标签/决策 4 编译期常量、D7 决策 1 三内联形不动）；实现期补记一至二十把被推翻的设计期预测逐条记档（T5 M-t5-b 判决撤下、D6 决策 4「scalar 直传值」被推翻等），无静默修正。 |
| 4 | tasks 只执行前三者已定义的工作 | ✅ 49 项勾选逐条挂 design 节/proposal 目标，各带来源/验证行；四枚阻塞前置（T5/T7/T12/T13）各自先勾并带结论；与任务书字面不符处均删除线+订正落档（T5 突变判决、T14 四条预测等），无虚勾、无 deferred/non-goal/未决方案选择。 |
| 5 | 场景覆盖 normal/boundary/failure | ✅ 每簇一正一负兑现：终态语料 881 枚含 11 枚 `build-bnd-*` 负例守卫 + 2 枚 E0501 语言面钉（`check-e0501-if-arm-disagreement`、`check-e0501-scope-timeout-value`）；纯 happy path 的面不存在。 |
| 6 | 无空章节 | ✅ design 14 节、proposal、tasks 各节均有实内容。 |
| 7 | 测试先行 | ✅ T4 红基线固化（14 枚红态）；各拓宽任务红先证据（父树 worktree 探针/黄金先红）逐条在落地记；T10 M1 突变暴露真覆盖缺口后补黄金+单测两层齐红——红先纪律在实现期持续承重。 |
| 8 | 负向断言真实 | ✅ 构造违规样例并断言被拒（11 枚负例黄金，非文字代替）；突变电池单测/黄金两层判决表入 T15-② 汇总，结构性静默项（T11 M2、T8-1、T7-4 A/B）带机理不冒充捕获。 |
| 9 | 完成度闭环（检查器/代码生成/运行时） | ✅ 三要素在册：检查器 D1（`Shapes`/`Site` 出口）、codegen D2–D9、运行时 D7-附（零改动决策 + 三条理由 + 例外口）；例外口经 T5/T6 查证未触发——`git diff 0030d60 --stat -- runtime/` 为空。 |
| 10 | 未决问题阻塞 | ✅ 四枚前置全部解毕并带落点（resource 进盒 → roadmap #24 语言面登记；内建迭代器载体 → 补记十一 codegen 内建合成；`errMsg` 语义 → report 臂运行期渲染；`timeout` 值 → 规范即 Result，残缺口 → roadmap #25）；无 SHOULD/MAY/TODO 掩盖未决。 |

#### 发现与处置（2026-09-15）

- **F1（措辞级，按 T14-③ 先例处置）**：T15 复选框 ③ 的「typecheck 塔零改动」按字面为假——T1 的章程就是 typecheck 的出口扩形（`shape.go` +385、`shape_test.go` +470、`typecheck.go` 83 行 diff 全为 `Shapes`/`Site`/`noteApply` 登记面）。成立的是**窄断言**：词面与判据零触碰——83 行 diff 中 `^\+` 行对七个词常量 grep 取空（唯一视觉命中 `c.bnd(bndArityGap)` 是**上下文行**，因相邻 `newtypeCall` 尾部加 `noteApply` 而入 diff）。「fitAbi 零新臂」同理按「臂集恒等」验证：基线与 HEAD 的 `fitAbi` `case` 行集 9↔9、diff 为空（T12 改既有 `abiPrim` 臂体、T5 走 `classType` 的 Dyn 分支不入 fitAbi）。落地记沿用 T14 的精确措辞，复选框原文不动。
- **复审无码级发现**：审查期零 tracked-file 改动——B1a 先例 `0aac57b` 的「审查期改动以独立提交落」一条本轮不适用；本记录与 T15 落地记落在未跟踪的变更目录，随归档入册。29 枚 B1b 提交的提交信息 `grep -ci co-authored` = 0（`.githooks/commit-msg` 机械拦截之外的双证）。

#### 归档复验（2026-09-15）

归档动作（`mv` 至本目录——变更目录此前从未入册故用文件系统 mv；`change.yaml` → `archived`）之后全阶梯重跑：`gofmt -l` 空、`go build`/`go vet` 0、`go test -count=1 ./...` **15 包（13 ok + 2 无测试文件）0 FAIL**（conformance 124.748s / 881 枚、benchmarks 28.234s、runtime 16.462s）；`validate.py --all --strict` → `OK: no changes found (nothing to validate); registry clean`（归档后活动变更集为空属期望态，`validate.py` 按设计排除 `archive/`）；`docs_sync.py` 33 对；`git diff --check` 空。roadmap 双语 B1b 行 → **done 2026-09-15**，as-built 收口行如实写「未实现集未达字面归零——五个边界词存活、守卫着十一枚负例黄金，残停面披露于 `docs/benchmarks.md`」，follow-up #24/#25 在册（B1b 无其他新 follow-up；#16 已随 B1a 闭环）。
