# generics-iterables-collections — 第 10/11/5/17 章前端（解析 + 类型检查）：接口、泛型、迭代协议与集合

## Why

M5（control-and-composites，归档 2026-09-05）把表达层（ch3/4/8/9/12 前端）接通，conformance 201 例全绿。类型系统余下的前端面全部压在逐章边界行上，且互相成锁：方法解析等接口、迭代协议等泛型、集合等迭代。按 roadmap M6 行（经本变更拆分，见裁决 Q1），第一刀把这些章的前端接上：

- 第 10 章（`docs/spec/1000-interfaces.md`，全规范最大章 16R/75S）：interface 声明（关联类型、方法签名、默认方法）、impl 声明（for 形与 inherent 形、泛型子句、where 子句）、方法接收者（`self`/`mut self`）、接收者字段赋值（`self.field = expr` 全语言唯一字段写形式）、成员名解析（字段+内禀+接口+派生统一成员集）、Dyn 值、derives 子句、泛型参数与单向定出、where 子句；
- 第 11 章（`docs/spec/1100-iterables.md`，9R/34S 含 collections 变更增补的组合子需求）：Iterator<T>（`next` + 十一枚组合子默认方法）、Iterable<T>（`Iter` 关联类型与句柄契约）、for 循环协议（单形 E0901）、String 内建 `Iterable<Rune>`、Range<T> 内建、实现者义务（资源类别禁实现）；
- 第 5 章（`docs/spec/0500-iteration.md`，2R/11S）：for 语句（头绑定/元组模式、E0202 值位）与 Range 表达式的类型面——**roadmap M6 行原未列 ch5，实现勘察发现 for 语句依赖 ch11 的 Iterable 判定（E0901），属 M6 行的遗漏依赖**，本变更随拆分补入 M6a 行；
- 第 17 章（`docs/spec/1700-collections.md`，6R/21S）：List/Map/Set 三个内建 gc 泛型集合类型（预导入）、列表字面量（元素一致 E0501、空字面量期望型 E1501）、命名方法索引（`.get/.has` 返回 Option、String 字节/码点双层四法）、快照迭代的静态面。

可验证的黑盒缺口（本日真机复验，`/tmp/we` 为当前构建）：

- `interface Describable { ... }` / `impl ... for ...` 顶层项、fn/声明上任何泛型子句 → `we: chapter 10 (interfaces and generics) forms are not implemented in this reference build yet`，exit 70；`pub interface ...` 更在 pub 派发处先行 E0105（`pub interface` 是章文批准形，当前排列不可解析）；`derives` 子句现状 E0105（"derives" after a top-level item——行尾追踪未接，解析层先行拒绝）；
- `for i in 0..3 { ... }` → `we: chapter 5 (iteration) forms ...`，exit 70；
- `[1, 2, 3]` 列表字面量 → `we: chapter 17 (collections) forms ...`，exit 70；
- 类型侧同停：`List<Int64>` / `Map<String, Int64>` 注解槽 → `we: std collection types (chapter 17) ...`，exit 70；`let d: Dyn<I> = ...` 注解槽 → `we: Dyn boxes (chapter 10) ...`，exit 70（构造形 `Dyn<I>(u)` 现状更早在解析层 E0104——`<` 归比较运算，泛型实参前瞻正是本变更要落的机制）；`0..n` → `we: range expressions (chapter 11) ...`——for 与集合程序无从写起。

涉触码盘点：M6a 范围 36 枚段内码（E0801–E0831、E0901–E0904、E1501）中 **33 枚从未发射**（E0816/E0827/E0828 已在发——M5 成员解析、M3 内建泛型定出与元数），另 E1004（ch12 泛型 fn 名值位）随泛型 fn 声明可解析而首次可达——**共 34 枚首次发射**，约为 M5（24 枚）的 1.4 倍。全部在注册表就位、触发语义由章文钉死。本里程碑交付：上述形式的程序在 `we check .` 下按规范全量接受或拒绝。

## 裁决记录（candidate 阶段四项表面裁决，2026-09-05，均采纳推荐）

- **Q1 切片范围 → 拆两变更**：M6 原范围 46 枚新码 + 66 需求约为 M5 两倍，且 ch10 是 ch11（Iterator 接口+方法集）、ch13（impl Releasable）、ch17（方法泛型子句 + `Dyn<Iterator<U>>`）、ch5（for 协议）的共同地基。第一刀本变更「泛型塔」= ch10 + ch11 + ch5 + ch17（33 段内新码 + E1004，全是类型检查器扩展：方法集、泛型单向定出、协议检查、内建类型面）；第二刀 M6b「纪律塔」= ch13 资源 + ch14 全量 + ch15 模块解析全量（13 新码，全是管线/数据流扩展：资源流分析、`?` 上下文、多模块加载器）。roadmap M6 行由本变更拆为 M6a/M6b 两行（其余 ID 不动；执行状态规则明文允许「any reorder is a change to this file carried by the affected milestone's change」）；M6b 依赖 M6a（impl 机器、方法集、泛型定出先在）；M6b 自有的表面裁决（多模块深度、资源纪律深度等）在 M6b 候选阶段另问。
- **Q2 Dyn 深度 → 全落检查面**：型位三查（E0820 参数非接口、E0821 裸接口名占值类型槽）+ 构造位两查（E0818 未实现类型、E0819 关联类型接口）+ 经 Dyn 值的成员调用解析接口方法集（集外 E0815）。方法集机制本片反正要落地，Dyn 增量适中且 E0818–E0821 全部可达；运行时盒实现属 M8 GC（章文「gc-category value carrying some implementing type's value」的表示未定）。
- **Q3 derives 深度 → 全落**：子句解析 + E0824 未知/重复目标 + E0822 手工 impl 拒绝 + 生成方法（`.equals`/`.hash`/`.toDebugString`）入成员集（可调用、可在成员解析下碰撞 E0814）+ E0823 能力传播检查（字段/底层/payload 逐位携带能力、泛型声明实例化时检查）。生成体的运行时行为是运行时的（章文明文「the generated bodies' runtime behavior is the runtime's」），静态面全落。
- **Q4 代码生成深度 → 维持 M4 接受集 + 声明擦除**：interface/impl 声明零 IR（record/newtype/sum 擦除同机制）；含方法调用/泛型调用/Dyn/for/集合方法的 main 体停 bndMainBody 边界 exit 70。方法派发与 Dyn 盒的诚实 IR 依赖 M8 的 GC 设计（接收者布局、盒表示未定）；FPCR 评价门以 `we check` 为核心（ch0/ch21 立场）。

## 目标与非目标

### 目标

1. **ch10 声明解析与检查**：interface 声明（`pub` 前缀、泛型子句、关联类型 `type Name` 至多四、方法签名 = 接收者先行裸参 + 章六形参参、默认方法块体、字段项 E0105、无接收者 E0801、接收者名 E0802、E0404 一名字空间）；impl 声明（`impl<T...> Name for Head where ... { ... }` 与 inherent `impl Head { ... }`；关联类型绑定 `type Name = TypeRef` 先于方法定义 E0806、缺绑定 E0805；完整性 E0807、签名一致 E0808（名字/接收者可变性/参数型列/返回型；方法子句精确同 E0808）、唯一性 E0809（含泛型 impl 与具体 impl 重叠）、孤儿规则 E0810（本变更单模块下 = 接口或头类型须本模块声明）、头非名目类型 E0811（元组/基类型/裸泛型参数头））；接收者纪律（E0812 `mut self` 于值类别头、E0813 mut-self 体外字段写、写目标恰 `self.field` 其余 E0105、未知字段 E0816、值型 E0501）。
2. **成员解析与派生**（裁决 Q2/Q3 定深度）：成员集 = 类型字段 + 内禀方法 + 各接口方法 + 派生生成方法（ch10 Member name resolution 统一四源）；一类型同名双成员 E0814（字段对方法、内禀对接口、两接口之间、内禀对继承默认）；接口静态位（Dyn 值或默认方法体内 `self`）候选集 = 该接口方法集、集外 E0815、字段永不在集；未知成员 E0816；无约束泛型参数无成员调用 E0817；derives 全落（封闭集 Eq/Hash/Show：E0824 未知/重复目标与 `Shareable` 拒绝、E0822 手工 impl、E0823 能力传播、生成方法入集参与碰撞与调用）；Dyn 全落（E0818 构造位未实现、E0819 关联类型接口、E0820 非接口参数、E0821 裸接口名值类型槽、经 Dyn 调用走接口方法集）；方法值不可驳（`let f = u.describe` 经成员解析后无值产生式 → E0105，ch12 场景）；泛型 fn 名值位 E1004。
3. **泛型与 where**：泛型子句 `<T1..Tk>` k≤8（E0825）于 fn/record/newtype/sum/interface/impl + 方法子句（名字后接收者参数表前；impl 方法子句须与接口方法精确一致 E0808；E0826 子句遮蔽外层参数——方法嵌套处活用）；定出单向（实参统一化或显式形 `name<T1, ..., Tk>(args)`/`Name<T1, ..., Tk>(args)`/`Name<T1, ..., Tk> { ... }`——fn 调用、newtype/变体构造、record 构造与更新头三形；不定即 E0827、元数不合 E0828；嵌套收口分隔 `Box<Box<Int64> >` 维持既有 E0105）；where 子句（fn 签名后/impl 头后；bound 授方法集；E0829 非接口 bound、E0830 调用/实例化位不满足、E0831 等式右侧非具体；等式在声明内钉住关联类型）；实例化类别诚实（`byval record Box<T>` 实例化 `Box<User>` → E0601、值和式镜像 E0702，按实例化检查）。
4. **ch11 迭代协议**：Iterator<T> 内建接口（`fn next(mut self) -> Option<T>` + 十一枚组合子默认方法——惰性 map/filter/take/skip 返 `Dyn<Iterator<...>>`、急性 collect/fold/reduce/count/any/all/find；方法泛型参数 U 由调用自身文本定出，不定 E0827）；Iterable<T> 内建接口（`type Iter; fn iterator(self) -> Iter`；用户 impl 绑定 Iter + E0904 句柄契约 = Iter 须实现同元素型的 Iterator；bound 反射——经 bound 已知 Iterable 处 `Iter` 携 Iterator 方法集）；for 协议单形（可迭代式须实现 Iterable，否则 E0901；裸 Iterator 非 for 可迭代）；String 内建 `Iterable<Rune>`（手工 impl → E0811 基类型头）；Range<T> 内建（操作数八种整型 E0902、混界 E0501、值类别 = 界对拷贝）；实现者义务（资源类别禁实现 E0903、值类别按拷贝迭代、gc 快照语义的静态面）。
5. **ch17 集合**：List<T>/Map<K,V>/Set<T> 内建 gc 泛型（元数 1/2/1，预导入经 ch15 既批修订；E0828 元数）；列表字面量 `[e1, ..., en]`（元素一致 E0501 逐位建立、空字面量取期望型否则 E1501、方括号即括号区可跨行）；命名方法索引锚定族落地（`List.get(i) -> Option<T>`、`Map.get(k) -> Option<V>`、`Set.has(t) -> Bool`、String `.byteLength()/.byteSlice(start,end)`/`.runeCount()/.charAt(i)`——字节/码点双层不同名）；`expr[expr]` 下标 E0105 维持（永久拒绝，ch17 政策）；组合子经 Iterator 默认方法解析（`.forEach(...)` → E0816，章文点名）；内建接收者其余成员停诚实边界（stdlib 表面 M8——`add/put/keys` 族与 String 非锚定方法章文明文「标准库自己的表面，非本章批准」）。
6. **codegen 最小增量**（裁决 Q4 定深度）：interface/impl 声明擦除（零 IR，record/newtype/sum 擦除同机制）；M4 What 表其余原样——新形式程序穿检查后在 bndMainBody 停边界 exit 70（真机披露）。
7. **边界表收敛**：解析器删 bndGener/bndIter/bndColl 三行；类型检查删 bndCollections/bndDyn/bndRange 三行；bndStdModules 语义收窄（String 锚定方法族与迭代落地后，所守 = stdlib 余面）；钉这些行的既有黄金改写并逐枚披露（D12）。
8. **conformance**：黄金新增约 100 枚（每新触码至少一负例 + 分组正例 + 边界收敛改写 + codegen 边界代表），全部先红；既有 201 枚除计划内边界钉死改写外零改写（回归护栏）。

### 非目标

- **ch13/ch14/ch15（M6b 纪律塔）**：scope resource（bndScope）、`?` 传播与 panic 家族（bndErr）、多模块解析/跨模块可见性（bndMultiModule）原样；E1101–E1106、E1201–E1203、E1301–E1303/E1305 维持不可达。
- **效果系统（M7）**：接口方法签名效果段（`effect io ->`）与 fn 型效果段维持 bndEffect；**E1404（impl 方法效果集与接口声明精确一致）与 E1402（组合子纯度）在 M6a 不可达**——效果声明不可解析，一切闭包/fn 的效果集为空、精确一致空对空恒真，design 披露；E1402 的发射属 M7。
- **stdlib 余面（M8）**：基类型方法（Int64 包装算术族）、String 非锚定方法（`.size()` 等）、集合变异方法族（`add/removeAt/put/keys` 及其同族）、Option 方法（`.unwrap` 族）——各章文明文「标准库自己的表面」，停诚实边界行；M6a 不对未批准清单私发 E0816（无法区分「未实现」与「不存在」，诚实边界是唯一通路）。
- **代码生成 IR 拓宽**：方法调用、虚派发、泛型单态化、Dyn 盒、for/组合子的 IR——M8 GC 设计后（裁决 Q4 理由）；`we build`/`we run` 对新形式程序停 M4 What 表。
- **运行时语义**：组合子求值次序、快照迭代、Dyn 盒共享、`self.field` 原地写——静态检查里程碑，可观察行为随后续代码生成里程碑。
- **下标语法**：`expr[expr]` 永非目标（ch17「no chapter may ratify without amending this policy here」）。
- **E0809 跨模块重叠判定**：单模块项目下唯一性 = 本模块内两 impl 及泛型/具体重叠；跨模块 impl 面属 M6b 多模块加载器。

## What Changes

- `internal/ast`：新节点——InterfaceDecl（关联类型/方法签名/默认方法体）、ImplDecl（for 形/inherent 形、关联类型绑定、方法定义）、derives 子句于 RecordDecl/NewtypeDecl/SumDecl、泛型子句与 where 子句于各声明、MethodSig/WhereBound 结构、ForStmt（头模式）、ListLit 表达式、显式泛型实参于调用/构造。
- `internal/parser`：ch10/ch11/ch5/ch17 产生式落地（interface/impl 项、接收者参数、方法子句、where、derives、for 语句、列表字面量、显式泛型形）；What 表删 3 行；pub 派发表补 interface/impl 行。
- `internal/typecheck`：接口/impl 符号层与验证（E0803–E0811、E0822–E0824）、成员集解析（E0814–E0817）、Dyn 面（E0818–E0821）、泛型定出与 where（E0825–E0831）、E1004、for 协议（E0901）、Range（E0902）、义务（E0903/E0904）、集合与字面量（E1501、锚定方法族）、内建声明层（Iterator/Iterable/String/Range/集合的内建 impl 与默认方法与检查器同机器）；What 表删 3 行、bndStdModules 收窄。
- `internal/codegen`：声明擦除扩展（interface/impl 零 IR）。
- conformance：约 100 枚新黄金 + 边界钉死改写若干（D12 逐枚披露）。
- `docs/roadmap/0000-reference-implementation.md` 与 `.zh.md`：M6 行拆为 M6a（本变更，归档时翻 done）与 M6b 两行（裁决 Q1，双语同步）。

## 影响层

`compiler`（已批准章节前端的部分实现，无规范增量）。**本变更不改变语言行为、无规范增量**：一切接受/拒绝判定以已批准的 `docs/spec/1000-interfaces.md`、`1100-iterables.md`、`0500-iteration.md`、`1700-collections.md` 及经其修订的宿主章（ch1/ch2/ch6/ch7/ch8/ch9/ch12/ch15 相应分句）与注册表现有 36 枚段内涉触码（另 E1004 与 E0105/E0404/E0501/E0601/E0605/E0702/E0812 等按章复用码）为依据（零新增/修改诊断码；消息按一码一消息纪律以注册表 title 起头组合）；单向定出、成员集四源、E0904 句柄契约均为章文 Requirement 的判定器实现（机制选型记录于 design，非规范私定）；内建声明层是「stdlib declares」类章文的实现载体（Iterator/Iterable/Option/List/Map/Set/Range/String 的内建面以与用户声明同形的检查器内建登记实现，design 披露其清单边界）；stdlib 余面维持诚实边界不以私发码冒充规范判定。

## 影响范围

- 修改：`internal/ast/ast.go`（新节点）、`internal/parser/parser.go`（产生式 + What 表 + pub 派发）、`internal/typecheck/typecheck.go`（检查器扩展 + What 表）、`internal/codegen/codegen.go`（声明擦除）、既有边界钉死黄金（钉 bndGener/bndIter/bndColl/bndCollections/bndDyn/bndRange 行的金样随形式落地改写——T1 逐枚清点披露）、`docs/roadmap/0000-reference-implementation.md` + `.zh.md`（M6 行拆分，归档动作）。
- 新增：`internal/parser`/`internal/typecheck` 的按章单测扩展、约 100 个 `internal/conformance/testdata/cases/*.json`。
- 不动：`docs/spec/`、`diagnostics.toml`、`go.mod`、`internal/lex`、`internal/diag`、`internal/version`、`internal/cli`（三命令行为面不变）、`runtime/`、其余既有黄金。

## 审计记录（2026-09-05，welang-spec-impact-audit 7 条，通过）

1. **问题真实性：通过**。Why 的黑盒缺口今日真机复验（`go build` 新鲜构建 `/tmp/we`，/tmp/m6a-audit 逐探针）：interface/impl 顶层项与泛型子句 → `chapter 10 (interfaces and generics) forms ...` exit 70；`pub interface` → pub 派发 E0105（7:5，报文实录）；`derives` 行尾 → E0105（"derives" after a top-level item）；`for i in 0..3` → `chapter 5 (iteration) forms ...` 70；`[1, 2, 3]` → `chapter 17 (collections) forms ...` 70；`List<Int64>` 注解槽 → `std collection types (chapter 17) ...` 70；`Dyn<I>` 注解槽 → `Dyn boxes (chapter 10) ...` 70；`0..n` 值位 → `range expressions (chapter 11) ...` 70。**审计过程自身抓出 Why 两处断言不精确并已修正**：构造形 `Dyn<I>(u)` 现状先在解析层 E0104（`<` 归比较——泛型实参前瞻正是本变更机制）、`derives` 现状是 E0105 非 exit 70。章文依据齐全（1000-interfaces/1100-iterables/0500-iteration/1700-collections 及宿主修订，proposal 逐条引用）。
2. **影响层声明：通过**。change.yaml `layers: [compiler]` 与「影响层」节一致；纯内部实现变更，proposal 明写「**本变更不改变语言行为、无规范增量**」边界（validate.py internals-only 豁免路径，本变更为该路径消费者，`--all --strict` 过）。
3. **规范增量范围：通过**。零新增/修改诊断码；34 枚首发码（36 段内涉触 − 3 已在发 + E1004）全部对照注册表现有条目（E0801–E0831/E0901–E0904/E1501/E1004，diagnostics.toml 自规范阶段变更落齐）；消息纪律 = 一码一消息以注册表 title 起头组合；无 active 变更冲突（当前唯一 active 即本变更）。
4. **原则一致性：通过**。无原则例外：单向定出是章文明文（否期望型回灌，P1）；E0816 不私发于 stdlib 未批准清单（诚实边界，不冒充规范判定）；iterHandle marker 型、内建 derives {Eq,Hash,Show} 登记、成员集单一机器均为已批 Requirement 的判定器实现（design D4/D7/D8 披露），非规范私定。四项表面裁决（Q1–Q4）记录在案。
5. **参考基线固定：通过**。全部引用已批准的 `docs/spec/` 章文与 diagnostics.toml 注册表为据；零 refr/ 草案引用（refr/ 为方向性参考的本仓库立场，M3 起实现变更一律以正典章文为据）。
6. **验收边界：通过**。目标 1–8 机械可判（黄金 ~100 枚先红、边界 What 行删除可 grep、exit 70/1/0 真机断言、既有 201 枚零改写硬门）；非目标 7 项划清 M6b（ch13/14/15）、效果系统（含 E1402/E1404 不可达披露）、stdlib 余面、IR 拓宽、运行时语义、下标永久拒绝、跨模块 E0809。
7. **粒度：通过**。单层（compiler）垂直可验（`we check` 为 FPCR 评价核心）；粒度本身经裁决 Q1 定夺（46 码拆 34+13，依赖论证 ch10 为共同地基）；M6b 自带候选阶段裁决。

结论：**通过**，状态 candidate → ready。

## 审查记录（welang-change-review 10 条，2026-09-05，ready → active，通过）

**职责边界**：

1. **proposal：通过**。Why = 黑盒缺口（真机复验逐条）；裁决/目标/非目标/What Changes 分节清楚，无任务清单混入；实现机制全部下放 design（D 编号引用）。
2. **spec 增量：不适用（豁免核实）**。零 specs/ 目录；「本变更不改变语言行为、无规范增量」标记在位，validate.py internals-only 豁免路径双探针验证过（M0 落地）；BCP-14 义务随之不适用。
3. **design：通过**。14 条 D 各含单一选型与被拒替代（D1 独立 MethodSig、D2 尾随逗号与大小写启发、D3 惰性成员集、D4 Dyn flag 与等式强制 iterHandle、D6 双向合一与独立实现检查器、D7 组合子特判）；章文/注册表引用精确（ch10:295 方法子句、ch11 组合子签名、注册表 title 纪律）；与 proposal 无矛盾（34 首发码、D12 改写 6–10 枚与目标 7/8 一致）。
4. **tasks：通过**。T1–T11 均有 来源/验证 两行；无 deferred/未决选项——T4 的「postfix `[` 现状核对」是判定义务（D14 ⑦ 定处置：属实则随 bndColl 收敛披露），两种结果路径均已定义，非开放问题。

**内容质量**：

5. **场景覆盖：通过**。normal（接口+impl+方法调用、泛型 fn/record、where 授集、组合子链、for 三源、derives 三目标、Dyn、Map/Set 参数面绿程序组）/ boundary（D12 边界钉死改写、bndMainBody exit 70、Map/Set 无构造面、Dyn 遮蔽）/ failure（33 段内新码逐枚负例 + E1004 + 披露锁定例）三类齐备（D13）。
6. **无空章节：通过**。无 specs/ 目录故无空 delta；proposal/design/tasks 各节皆承载内容；「影响层」节的规范依据枚举非包装层。
7. **测试先行：通过**。T1 黄金 ~100 枚先红（对当前构建）、T2 单测先红（引用未定义节点），red 证据为完成记录必填项；边界改写枚亦先红（翻转断言）。
8. **负向断言：通过**。每首发码至少一负例黄金 + T9 真机逐码电池；禁止模式有真实违规样例（E0809 泛型/具体重叠、E0814 四源碰撞、E0824 重复/Shareable、E0903 资源实现、方法值 E0105、`obj.m<T>(x)` 锁定例）。
9. **完成度闭环：通过**。类型检查 = design 主体（D3–D10）；代码生成 = D11（声明擦除 + bndMainBody 边界，裁决 Q4）；运行时 = 非目标显式排除（M8 GC 依赖，影响范围声明 runtime/ 不动、组合子默认体与派生生成体归 stdlib/运行时侧）——三要素各有归宿，静态检查里程碑立场自洽（M5 同构先例）。
10. **未决问题阻塞：通过**。无影响行为/契约/验收的开放问题；D10 不可达清单（E1402/E1404/E0810）是已论证事实 + T9 实证义务，非阻塞项；follow-up #5/#7/#8 维持登记不新增。

发现与处置：F1（审计阶段自纠，已处置）——Why 两处黑盒断言不精确（Dyn 构造形实为 E0104 先拦、derives 实为 E0105 非 exit 70），修正于审计前并记入审计记录第 1 条。无新增发现。

结论：**通过**，status ready → active，进入实现（T1–T11）。

## 审查记录（active → complete 关卡，welang-code-review 7 条，2026-09-06）

1. **规范符合性：通过**。ch11:48 组合子需求十一签名逐字对照 checker init() 表（map/filter/take/skip lazy 返回 Dyn<Iterator<…>>、collect/fold/reduce/count/any/all/find eager、map/fold 携 U 子句、全 recvMut——注释即章文签名）；ch10 E0807 报文以注册表 title `impl misses a non-defaulted interface method` 起头组合（黄金 check-e0807-missing-method 实证）；ch5 for 头 irrefutable 三形、ch17 锚定成员族由黄金族钉死（真机扫 298/298 逐字节）。实现无规范外行为；D10 不可达面（E1402/E1404/E0810/E0809 跨模块）均有章文或边界先停实证（T9 记录）。
2. **验证诚实性：通过**。T1–T9 逐项复核：T1 red 日志（98/298 失败清单零计划外）、T3–T8 各轮 conformance 双向 diff 零回归、T9 全量清缓存重跑 + 真机扫/绿程序/边界/擦除端到端/D10 八条——日志与脚本在 /tmp（t1-red.log、t2-*.log、t9-sweep.py）；本审查再跑最小集全绿（下第 7 条）。
3. **测试先行证据：通过**。T1 黄金先红（对当前构建 98 红）、T2 单测先红（引用未定义 ast 节点——M2/M5 同构）；黄金 stderr 全量正则核对 §52 人类可读格式（`file:line:col: error[CODE]: title — message`），97 枚 M6a 黄金零违规（仓库唯一非诊断行属既有 M0 usage 黄金，非本变更面）。
4. **诊断协议稳定：通过**。`git diff HEAD -- internal/diag/` 零文件——诊断包未触及，`--json` 字段集不动；零新增/修改诊断码（36 段内码 + E1004 规范阶段已落注册表，本变更为纯实现）；E0810 判定实现但按 D10 单模块不发射。
5. **单一权威：通过**。代码注释纯英文（grep 中文字符于四核心文件零命中）；零规范增量故无 specs/ 提升面——长期行为事实在 docs/spec/（规范先行）；变更工件（proposal/design/tasks）中文合规。
6. **红线复核：通过**。refr/ 在 .gitignore 首行且未入 staged（git status 核对）；提交将按无署名 trailer 执行（T11）；diff 范围 = 7 修改文件（ast/parser/typecheck/codegen + 双测试 + 1 改写黄金）+ 97 新黄金 + 2 单测文件 + openspec 变更目录，与变更目标一致无越界。
7. **最小可信验证：通过**。gofmt -l 空；go build ./... / go vet ./... 过；go test ./... 全绿（conformance 298/298、typecheck 30 套件、parser/codegen/cli）；`python3 openspec/tools/validate.py --all --strict` OK（1 change valid, registry clean）；`python3 openspec/tools/docs_sync.py` 30 对齐（对数不变）；`git diff --check` 干净。

结论：**通过**，status active → complete，进入归档（welang-archive-sync：roadmap M6 拆分同步 → git mv 归档 → status archived → 复验）。
