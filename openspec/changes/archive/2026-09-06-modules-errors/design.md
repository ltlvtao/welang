# design — modules-errors

先例：M6a design 的 D 编号与「单一选型 + 被拒替代」结构。章文/注册表引用精确到位；D14 为披露义务表。

## D1 AST 与 parser：scope resource 语句（Q1）

`ast.ScopeRes{Binds []ast.ScopeBind, Body *ast.Block, Line, Col}`，`ScopeBind{Name string, Val ast.Expr, Line, Col}`（无注解槽——绑定型即头表达式型）。产生式 `parseScopeRes`：`scope` 后**必须**逐字随 `resource`（否则按 ch18 形停新边界行 `bndConcScope`，What `chapter 18 (scope) forms`——bndScope 合盖行收窄拆分，既有 298 黄金零枚钉 bndScope 文本，grep 实证）；`(` 绑定列表 `name = expr` 逗号分隔（E0012 camelCase；重名 E0404 既有名空间——scope 头绑定入块作用域非模块名空间，重名走 ch8 模式内重名 E0602？**裁定：scope 头绑定是块内绑定**，重名沿用 let 元组重名路径——同块 let 同名走既有 E0404 块内名空间；无新码）；`)` 后 block（loopDepth 不变——break/continue 从内层循环穿透视 M3 既有）。

`?` 后缀：parser 既有 `case "?": p.bnd(bndErr)` 换真节点 `ast.Prop{X ast.Expr, Line, Col}`（锚 `?` token 自身）；postfix 链级 1 与 `(`/`.` 同列（ch2 修订既载）；值位/语句位同 postfix 循环。

**被拒替代**：ScopeRes 项复用 Assign 节点（形似而语义异——无注解槽、块作用域、线性纪律锚点名位，独立节点防止复用误锚）。

## D2 Releasable 声明层与完备性义务（ch13 R1）

内建接口 `releasableIface`：无关联类型、单方法 `fn release(mut self)`（recvMut: true、无参、返回 unit——ch13:10 场景 `fn release(mut self) { close(self.fd) }` 无返回标注 = unit）；deriveTarget 无；nameResolvable 入名。判定序（impl 声明期，M6a checkImplDecl 复用）：resolveIfaceRef 命中 Releasable → **头类别判定**：catOf(head)=="resource" 正常落方法集（签名错走 E0808 既有——ch13:26 明文）；非 resource（gc/value/newtype/内建/接口头）→ **E1102**（锚 impl 头，报文含头类型渲染与「byres record」补救指名）。**E1101 判定位是 byres 声明自身**：pass 2a 的 byres record 声明在**模块收尾遍**（所有 impl 收集后）查 Releasable impl 精确命中（sameType(head)）——缺失 → **E1101**（锚 record 头名）。判定序：E1101 于模块尾（impl 可能后置——「the module holds impl」语义是模块级的）；E1102 于 impl 处即时。

**被拒替代**：E1101 于声明处即时判（impl 尚未收集——同文件后置 impl 会误报；模块级义务模块级判定）。

## D3 scope resource 语句检查（ch13 R2）

`checkScopeRes`：头表达式逐个定型（nil expected、左到右）；型 T_i 须 `implementsFace(T_i, Releasable<>)`——四层判定复用（M6a implementsFace：精确 impl 命中 byres 头；sameType/泛型 impl/bound 层对资源位**不可达**——byres 头非接口非参数，披露）→ false 时 **E1103**（锚该头表达式首 token，报文含类型渲染与「byres record」指名）。绑定入独立块作用域层（类型 = 头表达式型）；体 walkControl；块值丢弃（语句位——E0202 由 parser 既有值位族钉）。**E0204 内嵌 defer**：scope 块内 defer 语句走 parser 既有 fn-body-top-level 判定（defer 语境栈——M5 落，scope 块非 fn 体直接层 → E0204 既有报文，核对锚点）。

**被拒替代**：头绑定共享函数 locals 层（破坏「each name binds for the block」块语义与线性纪律的 kill 边界）。

## D4 线性纪律：结构化活跃性分析（Q1）

资源绑定集合 R(fn) = 参数 ∪ let 绑定 ∪ scope 头绑定中 catOf(型)=="resource" 者。**活跃性按控制流树传播**：每节点 `live-in`/`live-out` 集，语句序列 `out(s_i) = in(s_{i+1})`；转移语句（scope 头转移 `scope resource(x = f)`、`return f`、实参传递 `g(f)`——被调参数声明该资源型）kill 其操作数绑定；使用语句（读 `f`）要求 `f ∈ in(s)` 否则 **E1104 转移后使用形**。控制节点：`if` = 双臂并集（臂内各自序贯）；`while/loop/for` = 保守回边近似（体内绑定每迭代新绑定——体内 let 资源绑定若在体内未转移即 **E1104 落空形**，锚绑定名；**循环外绑定入循环体 = 转移或使用**——使用合法、转移后循环外再用即 E1104）；`break/continue` = 内层块出口（穿透路径上未转移的 let 资源 = 落空——ch13:74 明文「`break` and `continue` end their inner blocks' scopes the same way」）；`match` 臂同 if 臂。**函数尾**：fn 末 live-out 非空 → **E1104 落空形**（报文 `the binding %q reaches the end of its scope on a path with no transfer`，锚绑定语句位）；**模块顶层** topLet 资源型 → **E1104 顶层形**（pass 2b 既有 topLet 位，报文「no channel exists outside function bodies」注册表分项）。**release 直调**：methodCall 名 `release` + 接收者资源型（含 impl 体内——判定位是调用处）→ **E1104 直调形**（报文「the spelling for early release is an early scope exit」，ch13:113 逐字）。

**保守性披露**：while 回边近似取并集（体内转移的绑定视作可能多次到达——**不报**多重转移，只保证「每路径至少一转移」单向义务；规范「exactly one of three transfers」的 exactly 语义按「至多一次使用后死亡 + 至少一次转移」双句落实：转移即 kill 保证至多、落空检查保证至少。双转移形（if 双臂各转移同一绑定后汇聚不再用）= 每路径恰一次，合法——并集分析天然正确）。

**被拒替代**：(a) 直线保守近似——拒绝「Conditional paths each transfer」场景（ch13:101 明文须接受），直接违反规范；(b) 位集全数据流——无 goto 下与树分析同效、实现重三倍。

## D5 别名禁令（ch13 R5）

三触发位，全检查期静态可判：`let/var g = f`（f 资源型）→ **E1105**（锚 `g` 名位，报文「a resource binding is the one handle」）；`var f = <资源型表达式>` → **E1105**（var 形——资源绑定 let-shaped，锚 `f`）；赋值 `g = f` 任一侧资源型 → **E1105**（锚 `=` 位——M3 assign 检查位既有）。实现位：let/var 绑定初始化定型后 catOf 判定（M5 绑定路径复用）；assign 值型与目标型双侧判定。

**被拒替代**：仅判 let 别名漏 var/赋值两形（ch13:122 三形并列，注册表 description 全列）。

## D6 组合位禁令（ch13 R6）

**E1106** 六触发位：(1) 元组元素（类型解析位——tupleType 构造处，M6a 参数/注解定型路径）；(2) gc record 字段（pass 2a 字段定型——byval 字段位**归 E0601** 既有（ch13:141 明文「a byval field is E0601's own rejection」），披露不改）；(3) sum 载荷（pass 2a 载荷定型）；(4) newtype 底层（pass 2a）；(5) 泛型实参（**实例化位**——resolveArgs/M6a 泛型应用路径，`Box<FileHandle>`/`id<FileHandle>` 同码，锚实参 token）；(6) `Dyn<Releasable>` 类型位（dynFace 解析接口实参遇 Releasable）与构造位（dynCall 接口实参 Releasable）。判定核心 `resourceInType(Type) bool`（递归：named decl byres / 元组逐元素 / newtype 底层 / Dyn 实参 Releasable；参数位 ref 不入——实例化位才判，ch13:141「at the instantiation site」）。锚：组合位置的首 token（字段名后类型 token/实参 token/Dyn< 等——黄金逐枚钉死）。

**被拒替代**：判定挂 typeOf 全局（误伤绑定位——参数位/返回位是合法位，ch13:141 明文绑定位合法；只在组合构造处判）。

## D7 `?` 传播算子（ch14 R2）

typeOf `*ast.Prop`：操作数定型（语境 expected 不入——`?` 自身定形）→ (a) 非 namedType Result（含 Option、基类型、元组、任何）→ **E1201**（锚 `?` token，报文含操作数型渲染与「Option does not propagate」注记——注册表 title 起头）；(b) 操作数 Result<T, E_src>：语境判定——`propCtx` 检查器字段（fn 体 = 声明返回型；闭包体 = 闭包自身返回位；defer 体/模块顶层/非 Result 返回 → **E1202** 锚 `?`，报文 naming the enclosing function——defer 形注记「a defer body has no return to propagate to」）；(c) E_src vs E_dst sameType → false **E1203**（锚 `?`，报文含两命名渲染与「the conversion is a match」补救）。通过 → 表达式型 = T（Ok 解包）。**短闭包定型**：闭包体含 Prop 节点（体遍预扫描）→ 闭包值型 = `fn(params) -> Result<尾型, E_src>`（E_src 取体内首个 `?` 的操作数——多个 `?` 依序判定同一 E_src，不一致 → E1203 链报）；全注解闭包声明返回非 Result 而体含 `?` → E1202（闭包语境 = 自身声明）。**postfix 链**：`f()?.name` 型 = 成员在 T 上解析（M6a memberType 复用）。

**被拒替代**：`?` 作为 fn 调用糖（无函数可指——纯 postfix 语义，节点独立）。

## D8 panic 族预导入签名（Q2）

prelude 三名换真签名（syms 层登记，M3 prelude 机制扩展）：`panic(msg: String) -> Never`、`todo(msg: String) -> Never`、`assert(cond: Bool, msg: String) -> ()`。调用走 callType 普通路径（实参型 E0501 既有；实参**个数** = bndArityGap follow-up #5 诚实边界维持，披露）；Never 返回满足任意返回位（agree 既有 Never 豁免）；`assert` unit 返回值位受 E0605 丢弃律既有。**bndTermination 行删除**（常量 + 使用点）。**顶层 `?` 的 E1202 语境**不依赖 panic（独立判定）；topLet 初始化器里 panic 调用合法（ch15 R5 场景「A panic during initialization aborts」——检查期 = 普通调用过）。**不可捕获无产生式**：catch/recover/try/`expr!` 无产生式 → E0105 既有（parser 关键字派发表已拒，黄金一枚锁定）。

**被拒替代**：维持 bndTermination 至 M8（检查器签名与运行时实现分离是 M3 以来惯例——`we check` 可全程静态判定，无理由半量）。

## D9 多模块装载与解析（Q3）

装载层 `internal/cli/check.go` 扩：项目模式发现 import（parse 每文件先收集）→ **路径映射**（`a.b.c` → `src/a/b/c.we`；首段 `std` → bndStdModules 维持，M8）→ 文件缺失 → **E1302 项目形**（报文含期望路径 `src/a/b/c.we`，ch15:6 报文义务）→ **环检**：模块图 DFS 三色标记，回边 → **E1301**（锚 import 语句，报文含环路径渲染 `a -> b -> a`）→ 每模块 parse + 类型检查（**跨模块符号表**：每模块符号登记含 pub 标记；合格名 `name.item` 解析——name 须 import 名（E1304 既有非 import 名形）、item 查被引模块：未声明 → E1304 既有形；声明非 pub → **E1303**（值位/类型位/变体构造器位三形同码，报文含模块名与项名））→ 接口值到达（Dyn 箱/泛型 bound 调用方法）合法（E1303 只管直接名到达——ch15:64 明文）。**根模块 main**：多模块下 main 判定仍在根（src/main.we），E1305 既有。**初始化序**：编译期计算拓扑后序（确定性——import 源序；无诊断面：环已被 E1301 拒、序本身合法），运行时执行归 codegen 非目标（披露）；**bndMultiModule 行删除**。

**模块级检查次序**：后序遍（被引先检）——被引模块的诊断先出（与初始化序一致，确定性披露）；panic-stop 首错即停全局序不变。

**被拒替代**：全局扁平符号表（模块名空间隔离是 E1303 的判定地基——必须按模块分桶）。

## D10 M6a 遗留三项精化（Q4）

(a) **跨子句空间同 idx 撞名**：unifyInto 的 paramRef idx 空间按声明局部——跨空间同 idx 混具体/位实参的漏报由 checkArgs/fnValueCall 兜底（M6a T8 记录论证）；精化 = 统一化前对撞名位显式检查（黄金：外围 fn<T> 调用头实参类型含 U 位 + 被调子句同名同 idx 位形——期望 E0501 而非静默）。(b) **精确 impl 覆写带子句默认**：覆写 map/fold（携 U 子句）的方法签名时子句指标空间错位——impl 方法的子句 idx 偏移（外层子句 + 方法子句，M6a T5 机制）核对泛型 impl 的 ifaceArgs 代换一致性（黄金：泛型 impl 覆写组合子签名错位形）。(c) **impl where 应用点满足**：`Wrap<User>` 构造时 impl<T> where T: Eq 的 User: Eq 判定（M6a T7 披露 (c)）——constructType 显式实参路径补 implementsFace 前置查（黄金：where 未满足的构造形 → E0830 既有码新位）。

三项各 1–2 枚黄金钉死；不扩消息面（全部复用既有码）。

## D11 codegen（维持 M4 接收集）

scope resource 语句/`?` 表达式/panic 族调用/多模块程序：main 体含任一新形式 → bndMainBody 既有（真机验证）；辅助模块全是 fn 声明 → bndOtherFns 既有；声明擦除面不变（byres record 无 IR 新面——RecordDecl 擦除既有）。What 表四行原样（M4 D3 封闭表）。

## D12 边界钉死黄金改写（T1 逐枚清点，此处预估）

`check-bnd-multi-module`（多模块落地 → 改写：双模块 pub fn 调用绿例，金样名保留 boundary 字样——M5/M6a D12 先例）+ `check-bnd-panic`（panic 族真签名 → 改写：panic 族 Never 返回位绿例或实参型负例）。预估 2 枚；`check-bnd-std-import`/`check-bnd-std-member`/`check-parse-clean`（bndStdModules 三钉）零改写。既有 298 枚其余零改写（硬门）。

## D13 黄金表（T1 落盘计划）

预估 ~55 枚新增：段内新码负例 11×2≈24（E1101/E1102/E1103/E1105×3 形/E1106×6 位归 22–26 枚）+ 复用码新位 8（E0105 catch/try/expr!、E0202 scope 值位、E0204 scope 内 defer、E0601 byval 字段资源、E1303 三位=E1303 归新码）+ 正例 ~20（资源三转移各形/条件双臂转移/scope 释放绿例——绿例检查静默、`?` 合法族含短闭包定型/postfix 链/panic 族三形/多模块 pub 面/接口值到达/初始化确定性（`we check` 静默）/E1302→E1301 负例对）+ build 边界 2（新形式 main 体 bndMainBody、多模块 build bndOtherFns）+ M6a 精化 3–4。多模块黄金全走 `setup.files` 多键（runner M1 既有）。生成器脚本 /tmp 惯例；全部先红（对当前构建）。

## D14 不可达与边界披露（实现期逐条验证）

- **E1106 byval 字段位**归 E0601 既有（ch13:141 明文）——黄金不设 E1106-byval 位。
- **E1202 语境与 M8 后效面**：defer 体顶层形已可达；task/test 体内的 `?`（ch18/20 语境）非本里程碑——task/test 块解析即停（bndConc/bndTest），不可达披露。
- **E1302 std 段**：`import std.*` 永停 bndStdModules（M8）——E1302 于 std 形不发（ch15:6「never resolved against the file system」）。
- **E1303 接口值到达位**合法（Dyn 箱/bound 调用）——黄金正例锁定（ch15:89 场景）。
- **E1608/E1806**（currentCancelSignal/advanceTime 位）：ch18/20 面，预导入名已登记（ch15 R2）但合法性判定归后章——bndTaskTime 维持。
- **while 回边近似**的保守性（D4 披露段）——「体内转移后循环外使用」形黄金锁定接受。
- **初始化序运行时执行**、**panic 中止运行时行为**：codegen 非目标（D11），检查期黄金只有静默绿例。
- **E0808 于 release 签名**：用户 impl Releasable 的签名错走 ch10 既有码（黄金一枚锁定复用形）。
