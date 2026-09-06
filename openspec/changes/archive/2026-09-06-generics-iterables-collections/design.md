# design.md — generics-iterables-collections

机制决策。每条单一选型；被拒替代记于条内。一切触发语义以章文为据（ch10/ch11/ch5/ch17 及宿主修订），本设计只定「判定器怎么写」。

## D1 AST 节点（ch10/ch11/ch5/ch17）

- `InterfaceDecl{Name, TypeParams []*TypeParam, Assocs []*AssocDecl, Methods []*MethodSig, Pub bool}`；`MethodSig{Name, TypeParams, Recv RecvKind(self|mutSelf|none), Params, Ret, HasRet, Body *Block(nil = 无默认), Line/Col}`——接口方法签名与默认体同节点，`Body != nil` 即默认方法。
- `ImplDecl{TypeParams, Iface *TypeRef(nil = inherent), Head *TypeRef, Where []WhereBound, Assocs []*AssocBinding, Methods []*FnDecl, Line/Col}`；impl 方法复用 `FnDecl`，增 `Recv RecvKind` 与 `Pub bool` 字段（内禀/for 形方法共用产生式）。
- `TypeParam{Name, Line/Col}`（本切片无 bound 位——bound 在 where 子句）；`WhereBound{Subject string, Ifaces []*TypeRef, Eq []*TypeEq, ...}` 支撑 `T: A + B` 与 `T.Assoc == Concrete` 两形。
- `DerivesClause{Targets []string, Line/Col}` 挂 `RecordDecl`/`NewtypeDecl`/`SumDecl`（nil = 无子句）。
- `ForStmt{Pat *Pattern(绑定/通配/元组), Iter Expr, Body *Block}`。
- `ListLit{Elems []Expr, Line/Col}`。
- 显式泛型实参：`CallExpr`/`Construct`/变体构造调用增 `TypeArgs []TypeRef(nil = 推定)`——解析侧一次定形，检查侧不再猜。
- `SumDecl`/`RecordDecl`/`NewtypeDecl`/`FnDecl` 增 `TypeParams`；`FnDecl` 增 `Where`。

被拒替代：方法签名复用 `FnDecl`（省一节点）——接收者裸参/默认体/无体签名三差异会使 FnDecl 分支布尔泛滥；独立 `MethodSig` 更直白。

## D2 解析器产生式

- **顶层派发**：`interface`/`impl` 行加真实产生式（现状 bndGener）；pub 派发表补两行（`pub interface` 为章文批准形，现状 E0105 是排列未派发，非规范拒绝——修复披露）。
- **interface 体**：行界项，无分隔符（块式花括号区）。项三形：`type Name`（关联类型；`type Name: Bound` → E0804 锚冒号或 bound 首 token；至多四，E0803 锚第五个 `type` 关键字）；`fn name<T...>(recv, params) [-> Ret]`（签名）；同形带块体（默认方法）。字段形 `name: type` 项 → E0105（章文）。方法名 E0012。
- **接收者参数**：参数表首位裸参（无注解）——`self` 或 `mut self`（`mut` 须直接前缀 `self`，其余裸参名 → E0802 锚该名；首位带注解 → E0801 锚参数名；零参数 → E0801 锚 `(`）。接收者只合法于接口签名与 impl 方法（普通 fn 首参裸名维持 E0105 既有行为——章六参数须全注解）。接口方法签名效果段（`effect io`）→ 既有 bndEffect（M7 边界）。
- **impl 头**：`impl [T...,(]) Name<args> for Head<args> [where ...] {` 与 inherent `impl Head<args> [where ...] {`（无 `for`）。头解析为 TypeRef；元组/基类型/裸参数头**先解析后由检查器拒 E0811**（产生式上 TypeRef 覆盖这些形，判定属类型层——E0811 注册表描述「impl head is not a nominal type」是语义判定）。关联绑定项 `type Name = TypeRef` 行于方法前（E0806 锚迟到的 `type` 项）。
- **where 子句**：`where` 后逗号分隔约束；约束 = `T: Bound[ + Bound2 ...]` 或 `T.Name == TypeRef`。挂 fn 签名尾（体前）与 impl 头后（花括号前）。接口声明上 where → E0105（「not ratified at this chapter」）。
- **derives 子句**：声明行尾 `derives Target[, Target]*`（E0824 未知/重复目标解析即可判——目标集封闭 {Eq,Hash,Show}；锚违例目标名）。
- **for 语句**：`for pat in expr block`，pat = 绑定名 | `_` | 元组模式（复用模式文法）；值位 for → 既有 E0202 判定（walkItems 语句位族）；`for` 头模式可驳（变体形）→ E0105（仅 match/let 不可驳位同律）。
- **列表字面量**：primary 位 `[` → `ListLit`（元素逗号分隔、尾随逗号容许——方括号即括号区行折叠）；postfix 位 `[`（下标）→ E0105 既有/落地（现状 bndColl 在 primary 位；postfix `[` 现状即 E0105？——T4 核对，若为边界行则一并收敛）。
- **显式泛型实参**：postfix 循环遇 `<` 时**角括号前瞻**（有界 token 扫描，不回溯重解析）：
  1. 从 `<` 起扫描，允许的 token = **PascalCase 形标识符**（首字符 A–Z）、`,`、嵌套 `<`、裸 `>`（闭一层）；深度须恰在某个裸 `>` 归零，且其后紧跟 `(`（调用形）或 `{`（record 构造头形）；其余任何 token（含 `(`、`>>`、`>=`、`.`、小写词、字面量）即扫描失败。
  2. 头名字约束：`{` 形要求 `<` 前的头是**裸 PascalCase 标识符**（构造头本就 PascalCase）；`(` 形要求裸标识符（fn 名 camelCase 合法）。
  3. 成功则消费为 TypeArgs（随后 `(` = 携实参调用、`{` = 携实参构造）；任何失败则 `<` 归还比较运算符，行为与 M5 完全一致。
  消歧论证：`if a < b {` 类常见形被双保险拦下（头 camelCase + 内容 camelCase 均失败）；现行文法里「头 < PascalCase 内容 > (」形在比较读法下**本就恒为 E0104 链**（9 级非结合），前瞻只把「必错程序」的错码从 E0104 换成构造/调用检查的错码。**残余歧义披露**：`if Wrap < Seal { ... }`（双 PascalCase 单位变体比较 + 块）会被前瞻吃成 `Wrap<Seal>{...}` 构造——该程序在两种读法下均为错码程序（变体不可 `<` 序），仅诊断从 E0501 换为 E0603 族；病态形接受，黄金不设例。`Dyn<I>(expr)` 构造经同一前瞻天然解析（PascalCase 头 + `(` 形）。类型位 `<` 无歧义（注解槽只有类型文法）。`mod.fn<T>(x)` 限定头暂归比较（单模块面；M6b 多模块时随加载器扩展规则并披露）。
- **E0809 等检查器侧码的锚点**在 D5。

被拒替代：显式形用尾随逗号强制区分比较（`f<T,>(x)`）——章文形无尾逗号（`name<T1, ..., Tk>(args)`），前瞻扫描已足；「仅大写头才试泛型」启发——小写泛型 fn 名合法（fn 名 camelCase 也可带子句），启发会漏。

## D3 检查器符号层（接口/impl/方法集）

- `ifaceInfo{Name, TypeParams, Assocs []assocDecl, Methods map[string]*methodSig, MethodOrder []string}`；`assocDecl{Name}`。
- `implInfo{Iface *Type(application), Head *Type, Assocs map[string]Type, Methods map[string]*fnDecl, Generic bool}`；按（接口名, 头基名）索引。
- **成员集**（解析的统一面）：`membersOf(t)` 返回有序候选——record/newtype 字段（既有）→ 内禀 impl 方法 → 各接口方法（代入 impl 的关联绑定与泛型实参后的签名）→ derives 生成方法（D8）。同名第二源登记时即 E0814（声明期判定，锚后到声明的名字 token——「rejected at the responsible declarations」）。接口方法互撞 = 第二个 impl 声明处报（E0809 已拒重复 impl；两**不同**接口同名方法 → 第二个 impl 头处 E0814）。
- **内禀/接口方法体检查**：inherent/for 形方法体 = 函数体语境（return/defer/E0402/E0605 既有），`self` 以头类型入局部作用域（mut self 不改变检查面——可变性只在 E0813 域判定用）；**默认方法体** = 接口位语境：`self` 的类型是「该接口」——成员解析走 D4 接口位规则（E0815），字段永不可达；体内泛型参数 = 接口子句 + 方法子句（E0826 遮蔽检查在声明期）。
- **E0807–E0811 判定序**（impl 登记）：头名目性（E0811：头 TypeRef 解析后须为 record/newtype/sum 名或其泛型应用；基类型/元组/裸参数 → E0811 锚头首 token）→ 孤儿（E0810：单模块下接口与头均本模块声明恒真——**M6a 内不可达**，多模块归 M6b，披露）→ 唯一性（E0809：同接口同头基名且实参相等或一侧含泛型参数 → 重叠）→ 关联绑定齐全（E0805 锚 impl 头）→ 完整性（E0807 锚 impl 头，报文点名缺失方法名）→ 逐方法签名一致（E0808 锚方法名：名字/接收者可变性/参数个数与型列/返回型逐位；方法子句精确同名同元）→ 方法体检查。
- E0803/E0804/E0806 声明期于接口/impl 体走查时判（锚点见 D2）；E0801/E0802 解析期（D2）。

被拒替代：成员集惰性每次现算——E0814 碰撞要求声明期定位「负责的声明」，登记期算一次并存是对的。

## D4 成员解析与 Dyn（裁决 Q2 全落）

- `Member` 表达式检查分流（替换 M5 的字段-only 面）：
  1. 接收者类型为 **Dyn 位**（DynType 或默认体内 self）→ 候选 = 接口方法集；集外 E0815 锚成员名；字段永不在集（`d.name` 即使接口有同名关联类型也是 E0815——关联类型非成员）。
  2. 接收者为**泛型参数**：无 bound → E0817 锚成员名；有 bound → 各 bound 接口方法集之并（`+` 多 bound 并集；关联类型经 bound 内解析为……本切片 bound 接口的关联类型仅经 `T.Assoc == Concrete` 等式钉住后可用——无等式时 `T.Assoc` 型位成员调用走 E0815 面？**否**：bound 接口的方法集直接可用（章文「its Iter carries Iterator<T>'s method set … concretely or through a bound」——E0904 契约的 bound 反射：经 bound 已知 Iterable 处 `.iterator()` 的结果类型携带 Iterator 方法集，`.next()` 可链）。实现：`iterator()` 返回型 = 该 bound 的 `Iter`，若 Iter 无等式钉住则其类型为「Iterator<T> 的知集类型」——以 marker 类型 `iterHandle{T}` 表示，其成员集 = Iterator<T> 方法集（链式 `.iterator().next()` 由此绿）。
  3. **具体名目类型** → D3 成员集：命中字段（既有）/方法/派生方法；未命中 → 基类型与内建接收者走 4；record/newtype/sum 未命中 → E0816（既有锚点成员名）。
  4. **内建接收者**（String/Bytes/Range/List/Map/Set/Option/Result 与基类型）：锚定族表命中 → 签名检查（实参 E0501、个数缺口 bndArityGap 既有 follow-up #5、返回型代入）；`.forEach` 于 Iterator/iterHandle 接收者 → **E0816**（ch11 点名「no such member … does not exist」——规范明文不存在者发 E0816，锚成员名）；其余未知名 → **bndStdModules 收窄边界**（stdlib 表面 M8——`add/.size()/.unwrap` 族章文归标准库，不能私发 E0816 假称不存在）。
- **方法值**：成员解析成功且命中方法、但表达式处于值位（非调用头）→ E0105 锚成员名（ch12 场景「resolves per member name resolution but fits no ratified value production」——机制：`typeOf(Member)` 命中方法时仅当父为 Call 才成值，否则 E0105 消息指明方法值未批准；字段命中不受影响）。
- **方法调用**：`Call{Callee: Member}` 于成员集命中方法 → 实参逐位 E0501 / bndArityGap / 返回型代入（泛型方法经 D6 定出）；`Call{Callee: Member}` 命中**字段且字段为 fn 型** → M5 既有经值调用路径不变。
- **Dyn 构造**：`Dyn<I>(expr)` = Call 于名 `Dyn`（预导入名）——检查器特识：I 须接口（E0820 锚 `Dyn` 或实参型首 token）、无关联类型（E0819 同锚）、expr 型须实现 I（E0818 锚 expr 首 token）；产 `DynType{I}`（gc 类别）。`Dyn` 本地遮蔽后按本地含义（ch15 既有遮蔽律）。**裸接口名值类型槽**：注解/参数/返回位解析到 interfaceInfo → E0821 锚该名（「the box form is Dyn<Describable>」）；DynType 本身是合法槽型。
- **E0818 判定「须实现」**：头类型有该接口的 impl（代入后匹配——泛型 impl 按 D6 统一化判其覆盖此头）。

被拒替代：Dyn 经 namedType 加 flag——Dyn<I> 的成员解析/类别/构造三面全异，独立类型形更直白；iterHandle 用关联类型等式强制——无等式时 `.iterator().next()` 须仍绿（章文 bound 反射场景），marker 型是对的。

## D5 锚点表（新增码统一登记于 tasks 黄金表，此处定则）

报文以注册表 title 起头（一码一消息）；锚点规则：

- 声明期码锚**违例声明自身的首 token 或违例项名**：E0803 第五个 `type`、E0804 bound 首 token、E0805/E0807/E0809/E0811 impl 头首 token（`impl`）、E0812 违例方法名、E0814 后到声明名字 token、E0822 手工 impl 头、E0823 声明头名、E0824 违例目标名、E0825 子句 `<`、E0826 遮蔽参数名。
- 使用期码锚**使用点**：E0808（impl 方法名——虽属声明期，锚违例方法更指位）、E0813 赋值目标 `self`、E0815/E0816/E0817 成员名 token、E0818 expr 首 token、E0819/E0820/E0821 型/名 token、E0827 调用/构造头名、E0828 子句 `>`（实参元数）或头名（声明侧无码——E0828 只发使用位）、E0829/E0831 约束首 token、E0830 调用头名、E0901 for 的 iterable 式首 token、E0902 违界操作数、E0903/E0904 impl 头、E1501 字面量 `[`、E1004 泛型 fn 名 token。

M3 锚点惯例（名词分流、报文含具体名字）沿用；黄金表逐枚钉死。

## D6 泛型定出与 where

- **代入**：`subst(t, map)` 结构递归（paramRef → 映射命中则替换；namedType 代入实参递归；fn 型/元组递归）。
- **统一化**（单向，章文「args/fields determine」）：`unify(declType, argType, map)`——declType 为 paramRef 且未绑 → 绑定（已绑则须 sameType 否则 E0501 既有位）；两侧结构同形则递归（named 同头同元、元组同长、fn 型逐位）；不同形 → 该实参位的 E0501（既有协议位报文）。调用/构造后 map 有未绑参数 → **E0827**；显式形 TypeArgs 元数 ≠ 子句元数 → **E0828**（锚闭口 `>`）。既有 E0827（Option/None 期位）/E0828（内建泛型注解位）报文与锚点**不改**（201 例护栏零改写）；新发位按 D5。
- **方法泛型**（组合子）：`map<U>/fold<U>` 等在调用时由实参文本定出——fn 型实参的返回型绑 U；绑不出 → E0827（锚调用头名）；**方法调用无显式实参形**（章文「a method call carries no explicit type-argument form」——`obj.m<T>(x)` 不可写，解析侧 postfix 泛型前瞻只认名目头，`obj.m<Int64>(x)` 的 `<` 归比较 → E0104/E0105 按既有非结合/未批准路径，披露）。
- **实例化诚实**：泛型 record/newtype/sum 的构造/注解实例化时，`subst` 后字段/payload 类别重查（E0601/E0702 既有报文，锚类型 token——「checked at each instantiation」）；泛型 impl 头代入后照常走 E0812（`mut self` 于值类别实例化头）。
- **where 检查**：声明期——bound 名须解析到 interfaceInfo（否则 **E0829** 锚 bound 首 token；基类型/record/sum/参数名均拒）；bound 主体须为本声明泛型参数（他名 → E0105 既有未批准形？章文「A bound MUST name a declared interface」约束的是右侧；左侧主体章文限定「where-bound subjects are generic params only」——违者 E0105，披露）；等式左侧 `T.Assoc` 须为 bound 接口的关联类型（否则 E0105——章文未批他形）、右侧须具体（基类型/名目/其泛型应用；泛型参数/另一关联名 → **E0831** 锚右侧首 token）。使用期——调用/构造/实例化的每个实参对每个 bound 走 E0818 同判定，不满足 → **E0830** 锚调用头名；等式在声明内生效：`T.Assoc` 位解析为右侧具体型。
- **bound 授方法集**：声明体内参数成员解析走 D4-2（bound 并集）；无 bound 参数 E0817。

被拒替代：双向合一（期望型回灌）——章文明拒（「Expected-type inference does not exist」）；E0830 用独立「实现检查器」——与 E0818 同判定同机制，一处实现两码只差锚点与报文。

## D7 内建声明层（ch11/ch17 的「stdlib declares」实现载体）

检查器启动时登记**与用户声明同形**的内建面（单一机器，两类来源）：

- 内建接口：`Iterator<T>`（方法 `next(mut self) -> Option<T>` + 十一组合子**签名**——组合子默认体是「标准库的默认体」（章文明文），M6a 只登记签名面：可调用、可在用户 impl 内 override（E0808 精确一致含方法子句）、参与 E0816/E0815 判定；**体不存在故无需检查**，M8 随 stdlib 落）；`Iterable<T>`（`type Iter; fn iterator(self) -> Iter`）。
- 内建 impl：`String: Iterable<Rune>`、`Range<T>: Iterable<T>`、`List<T>: Iterable<T>`、`Map<K,V>: Iterable<(K,V)>`、`Set<T>: Iterable<T>`——Iter 绑定 = 内建迭代器句柄 marker 型（名字是「标准库的，非本章的」——不占用户可见名，仅内部标记 `stdIterHandle<Elem>`）。E0811 于内建头的手工 impl 重写：用户写 `impl Iterable<Rune> for String` → 头解析到基类型名 → E0811（既有判定路径天然覆盖）。
- 内建类型：`List<T>/Map<K,V>/Set<T>` 为 gc 名目型（元数 1/2/1，E0828 既有码扩发）；`Range<T>` 为值类别名目型（catOf → value——「copies the pair」）；`Option<T>/Result<T,E>` 沿 M3 既有内建和式不动。
- 内建方法族表（锚定族）：String `.byteLength()->Int64`/`.byteSlice(Int64,Int64)->String`/`.runeCount()->Int64`/`.charAt(Int64)->Rune`；List `.get(Int64)->Option<T>`；Map `.get(K)->Option<V>`；Set `.has(T)->Bool`；一切内建 Iterable 接收者 `.iterator()`（经内建 impl 方法集解析，非族表特判——impl 机器统一走）；Bytes 无任何面（E0901 路径「Bytes not iterable」）。
- **Map/Set 无构造面**（章文只批列表字面量；Map/Set 构造是 stdlib 面）——M6a 程序中 Map/Set 值只能经参数到达；披露（黄金正例即此形）。
- Option 方法（`.unwrap` 族）不入集——stdlib 面，bndStdModules 边界。

被拒替代：组合子做成检查器特判路径——会绕开 E0808/E0816/E0827 的统一判定，用户 override 组合子的负例无从构造；内建与用户同机器是「stdlib declares」章文的直译。

## D8 derives（裁决 Q3 全落）

- 解析：D2。检查：目标 ∈ {Eq,Hash,Show} 封闭（E0824 解析期已拒未知/重复/`Shareable`）。
- **手工 impl 拒绝**：`impl Eq for X` → 头解析 `Eq/Hash/Show` 为内建标记接口名（登记于 D7 层，无方法体）→ **E0822** 锚 impl 头（「builtin derive target; the clause is …」）。
- **能力传播**：`capability(t, target)`——基类型恒 true；String/Bytes/Rune 恒 true；名目类型 = 其 derives 子句含该目标（Option/Result/List/Map/Set/Range 内建声明登记 derives {Eq,Hash,Show}——stdlib 声明面，章文「base types carry it, composites carry it through their own derives」的内建侧推论，披露）；元组 = 逐元；泛型参数 = false（实例化期再判）。record 每字段 / newtype 底层 / sum 每 payload 不满足 → **E0823** 锚违例字段/底层/payload 的类型 token，报文点名该类型名与所缺目标。
- **泛型声明**：声明期不查（参数能力未知）——实例化期（D6）以实参重查，违者 E0823 锚实例化位类型 token（章文「checked at each instantiation (E0823 there)」）。
- **生成方法**：`.equals(other: Self) -> Bool`（Eq）、`.hash() -> Int64`（Hash）、`.toDebugString() -> String`（Show）入 D3 成员集（声明期登记；`Self` = 该声明类型）——可调用（实参 E0501/bndArityGap/返回型照常）、可碰撞 E0814（字段 `equals` 或方法 `hash` 同名即拒）。**生成体不存在故不检查**（运行时的，章文明文）——`p1 == p2` 于组合值仍 E0501（ch7 既有：`==` 只比较基类型；D7 家族表不为 `==` 开口）。
- `.equals` 的 `Self` 实参型：调用位实参须同型（E0501 既有协议位）——无子类型无变体（ch10 立场），零新规则。

## D9 for / Range / 列表字面量 / 集合定型

- **for**：iterable 式定型 → 须实现 Iterable（D7 内建 impl 集 ∪ 用户 impl；泛型参数经 bound）否则 **E0901** 锚 iterable 式首 token（报文含两形注记——非 Iterable 与裸 Iterator 同码同消息，注册表一字面）；元素型 T = 接口第一实参代入；头模式检查复用 ch8 绑定规则（元组模式 vs 非元组元素型 → E0501 锚模式首 token，ch5 场景）；体内绑定作用域 = 每迭代独立（既有块作用域机制；循环深度 +1 使 break/continue 合法——ch3 既有）。语句位值律 E0202 既有。
- **Range**：`a..b` 操作数定型——两侧均八种整型之一且同型 → `Range<T>`（value 类别）；一侧非整型（float/rune/String/名目）→ **E0902** 锚违界操作数；两侧整型异型 → E0501 既有（锚 `..`，M3 惯例）；`0..n` 字面量定形经 ch7 既有规则。for/值位两用（ch5 既有）。bndRange 行删除。
- **列表字面量**：元素链式 agree（首元定准，后续 E0501 锚违例元素——「operands of different types」既有消息形，锚违例元素首 token）；空字面量：期望型（注解/参数/返回位）为 `List<X>` → 取 X；非 List 期望或无期望 → **E1501** 锚 `[`；非空字面量对期望 `List<X>` 元素不合 → E0501 既有绑定协议位。bndColl/bndCollections 行删除。
- **集合型引用**：`List/Map/Set` 预导入名（ch15 既有修订——M3 已登记预导入名集，核对含三者则零改动，否则补登记）；元数 E0828（扩发，锚同既有内建泛型位）。

## D10 不可达与边界披露（实现期须逐条验证）

- **E1404/E1402 不可达**（M7 前效果声明不可解析）：impl 方法效果集与接口声明精确一致 = 空对空恒真；组合子 fn 型实参纯度 = 空 ⊆ 空 恒真。黄金不设此二码负例（设了必红）；design 记录在案。
- **E0810 孤儿规则 M6a 不可达**：单模块下接口或头类型必本模块声明（无 import 面）——判定实现但不发；多模块归 M6b。
- **E0809 范围**：本模块内同接口同头两 impl、泛型 impl 与具体 impl 重叠（头实参含泛型参数视为覆盖）。跨模块面 M6b。
- **E0821 与 Dyn 遮蔽**：本地声明名 `Dyn` 遮蔽预导入（ch15 既有律）——`Dyn<I>(x)` 按本地含义走（本地 `Dyn` 非 interface 构造路径 → 既有 E1304/E0603 面），黄金正例锁定。
- **方法调用显式实参形不可写**：`obj.m<T>(x)` 解析为比较链 → E0104/E0105 既有路径（章文「a method call carries no explicit type-argument form」；非诊断新面，黄金一枚锁定）。
- **赋值目标可变性**（follow-up #7）、**E0604 注册表陈旧子句**（follow-up #8）维持登记不动。
- **bndStdModules 收窄**：What 文本不变（M5 D10 先例），可达面缩至「非锚定 stdlib 成员」；既有 check-bnd-std-member 黄金若锚定成员属锚定族则改写为族内正例（T1 清点披露），否则零改写。
- **既有 E0827/E0828 报文与锚点冻结**：M3 已发位（Option/None、内建泛型注解）不动，新发位按 D5——同码多触发位是注册表 description 既载形态（E1104 先例）。

## D11 codegen（裁决 Q4）

`Emit` 首 pass switch 增 `case *ast.InterfaceDecl, *ast.ImplDecl: continue`（与 record/newtype/sum 擦除同机制同注释列）；方法调用/泛型调用/Dyn/for/字面量表达式**不入 M4 受纳的 main 体形**——bndMainBody 既有边界兜住（真机验证：含方法调用的 main → 边界行 exit 70）。What 表四行原样。

## D12 边界钉死黄金改写（T1 逐枚清点，此处预估）

钉 bndGener/bndIter/bndColl（parser 侧）与 bndCollections/bndDyn/bndRange（typecheck 侧）的既有黄金：预估 6–10 枚（M2/M3/M4 落的 check-bnd-* 族；`check-bnd-std-member` 若属锚定族照 D10 处理）。每枚改写翻转为真实行为（负例转真码或正例转绿），金样名保留「boundary」字样（M5 D12 先例），完成记录逐枚披露。既有 201 枚其余零改写（硬门）。

## D13 黄金表（T1 落盘计划）

约 100 枚新增：33 段内新码各至少一负例（多数 2+：声明期/使用期、接口位/构造位分流）+ E1004 一枚 + 分组正例（接口+impl+方法调用绿程序、泛型 fn/record 绿程序、where bound 授集绿程序、组合子链绿程序、for over String/Range/List 绿程序、derives 三目标绿程序、Dyn 构造+调用绿程序、Map/Set 参数面绿程序）+ 边界改写（D12）+ codegen 边界代表（interface/impl 声明擦除 build+run、方法调用 main 停界）+ 披露锁定例（E0104 方法显式实参形、Dyn 遮蔽、E0821 关联类型接口、`.forEach` E0816）。生成器沿用 M5 模式（源串 + 子串定位锚点，杜绝行列漂移）。

## D14 实现期披露义务汇总

① pub+interface/pub+impl 排列修复披露（E0105 → 真实产生式，属边界行删除的必然结果）；② 组合子默认体 = stdlib 面（签名面全落，体 M8）；③ Map/Set 无构造面（参数到达形）；④ E1402/E1404/E0810 不可达记录；⑤ 内建 derives {Eq,Hash,Show} 登记披露（stdlib 声明面的内建侧推论）；⑥ 方法值 E0105 机制（解析成功+值位拒绝）；⑦ postfix `[` 现状核对（若属边界行则随 bndColl 收敛披露）。
