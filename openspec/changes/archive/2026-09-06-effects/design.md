# design — effects

对应 proposal 目标 1–9；四项表面裁决（Q1 全量 / Q2 codegen 冻结 / Q3 织入一致机器 / Q4 闭包段 E0105）逐条落到 D 条。

## D1 AST 与 parser：effect 项、三声明段位、闭包段位（Q1/Q4）

`ast.EffectDecl{Name string; Pub bool; Line, Col int}` 顶层项（parser.go:404 停点替换）：`pub` 可选、`effect` 关键字、单标识符名、无体。三个段位替换停点 499/1276/1438：fn 声明、接口方法、impl 方法的参数表与 `-> T`（或体）之间接受 `effect tag1 tag2 …`——一至多个标识符（空格分隔），存 `EffectTags []string` + `EffectLine/EffectCol`（段首标签位，诊断锚用）；接口方法与 impl 方法 AST 各增字段（`ast.FnDecl`/`ast.MethodDecl` 两形）。闭包段位（停点 650）按 Q4 **不再解析**：`fn(…) effect …` 直接 `p.failTok` E0105（锚 `effect` token，报文既有「where a fn body block opens」形镜像——裸标签 `fn(…) io {` 同判 E0105，锚首标签 token）。fn 型段既有核对：`fn(T) effect io -> T`（类型位带 keyword）与 `fn(T) io -> T`（声明位裸标签）两串位形今日已正确 E0105，锁定枚钉住不改。`bndEffect` 常量删（五停点全消）。被拒：闭包段解析后忽略——半套语义（解析了不检查）正是拆片病；E0105 是章文裁定（闭包集是推断的，无段槽）。

## D2 效果名纪律：E0012 / E0404 / E1403（parser 声明面）

三项纯名字判定全在 parser（M2 先例：名字表 E0404 行 233、camelCase E0012 行 214）：(a) `effect Db` → E0012（camelCase，同绑定纪律）；(b) `effect db` 与同模块 fn/record/… 撞名 → E0404（入既有名字表，effect 是模块单一名字空间一员）；(c) `effect io`（及 net/time）→ E1403（内建三词硬表——内建为语言级非模块项，纯文本可判）。次序沿 M2 既定「collision outranks naming」并延伸：**E0404（模块内撞名）→ E1403（内建撞名）→ E0012（命名律）**。被拒：E1403 挪 typecheck 与 E1304 同层——内建判定不涉任何名字解析，声明面纪律一面留 parser。

## D3 效果集表示与解析：canonical 键、E1304、跨模块（ch16 R1/R3 + ch15）

`fnType.tags` 从「不解析透明字符串」翻为 **canonical 键串**：内建 → 裸键（`"io"`）；自定义 → `<模块键>.<名>`（模块 b 内裸写 `db` 与模块 A 内合格写 `b.db` 同键 `"b.db"`；根模块键 `main`）。集内去重保序（声明序）。两个解析入口：(a) **声明段/方法段标签**（pass 1 声明走查时逐标签 resolve）；(b) **fn 型段标签**（resolveTypeRef 的 fnType 构造点，行 2792 搬运前逐标签 resolve）。resolveEffectTag(tag) 判序：内建三词 → 裸键；裸名 → 本模块 `effect` 声明表；两段 qualified（`mod.tag`）→ import 符号表查 mod（E1304 未导入形）→ importSym 扩 effect kind（declares-no / non-pub 门沿用 E1304/E1303 既有报文）→ 得 canonical 键；全落空 → E1304（裸形报文 `unresolved name — no effect named "db" is declared here and no import introduces it` 之类，锚标签 token；合格形沿用既有 E1304 报文）。跨模块：装载层零改（EffectDecl 随模块 parse/typecheck 既有管线走，后序检查序既有）；typecheck 的 `symbol` 增 kind。被拒：tags 保持字符串、比较时双写归一——归一逻辑散布所有比较点，canonical 一次到位。

## D4 E1401 调用位：calleeSet 四路 + defer 归属 + panic 零集（ch16 R2/R4 + ch3 + ch14）

机器挂 fn/方法体走查（`walkItems` 既有路径）：每调用表达式取**被调效果集**，非 ⊆ 当前声明集（canonical 集比较）→ E1401（锚 callee 首 token，报文含差集效果名与 callee 名，注册表 title "undeclared effect at a call"）。被调集四路：**(1) 直接名** `save(s)`——syms 解析到 fn 符号 → 其声明段集；**(2) 方法调用** `x.save(s)`——memberOfType 定接收者：impl 已知 → impl 方法段集；接收者是接口类型（Dyn 箱/泛型 bound）→ 接口方法段集；**(3) fn 型值** `f(s)`——identType → fnType.tags（fn 型参数/let = 声明集；闭包值 = D6 推断集）；**(4) panic 族**——terminationFnType 硬 case 集 空（panic 族零效果，ch14 既有硬 case 天然旁路，验证不加机器）。**闭包体内不判 E1401**（调用并入推断集，D6）；**defer 体调用计入外围函数**（ch3「只偏移时机不偏移归属」——propCtx.deferBody 语境既有，外围声明集经新增 `fnTags []string` 语境字段携带，deferBody 位照查）。**接口默认方法体**：M6a 默认体为空标记（真体 M8），走查自然空——D13 披露。被拒：调用图全程序先收集后检查——声明序即检查序的既有 panic-stop 协议不动。

## D5 E1402 织入一致机器：agree 子集比较器（Q3）

新增 `fnAgree(value, slot Type)`：结构一致性按 sameType 递归（参数/返回/嵌套 fn 型 **tags 精确**——无方差），唯**顶层** fn 型 tags 判单向 ⊆（值集 ⊆ 槽集）；不满足且结构一致 → E1402（锚 = E0501 同锚：注解位/实参位/返回位/字段位），结构不一致 → 既有 E0501 原文不动。挂点四个（E0501 既有判位）：let 注解定型、调用实参、返回值、字段初始化与 update。集差报文含值侧多出的效果名（注册表 title "function value effect set does not match the expected type's"）。**既有假报翻绿披露**：纯闭包（空集）入 `fn(Int64) io -> Int64` 槽今日 sameType 精确比较假报 E0501——今日零黄金携带此形（已 grep `fn\([^)]*\) [a-z][a-zA-Z.]* ->` 于 testdata，零命中），M7 翻为合法，新黄金钉绿面。被拒：sameType 改子集——E0830/E0808/checkImplMethodSig/unify 恒等位全依赖精确（Q3 裁决记录）。

## D6 闭包推断：体调用并集、构造 ≠ 执行（ch16 R4）

`closureType` 三臂（全注解/短/裸参）在体走查处挂**收集器**：体内每个调用的被调集（D4 四路同源）并入闭包自身集；遇嵌套闭包字面量**跳过其体**（构造 ≠ 执行——外层构造内层闭包不执行它）；内层闭包自己推断自己的集。推断集写入产出的 fnType.tags（canonical），此后该值的一切面（E1402 一致位、E1401 经 fn 型值调用）与声明集同权。短闭包挂 M5 期望线程既有体走查位（同点收集）。**闭包构造表达式本身对外围函数零贡献**（外围 walkItems 遇闭包字面量同样跳过体——构造不污染）；闭包**执行**（经 D4 路 3）才计入外围。topLet 初始化器里的闭包构造同理豁免 E1405（D8）。被拒：闭包声明段（章文无此面，Q4）。

## D7 E1404 接口位：精确相等（ch16 R5 + ch10）

挂 `checkImplMethodSig`（M6b 增 clause 参后）：签名 sameType 判定之后，接口方法声明集与 impl 方法声明集**精确相等**（canonical 集双向比较；缺、多同拒，报文注明两侧差集标签，注册表 title "impl method effect set disagrees with the interface"，锚方法名 token——E0808 同位先例）。两侧缺省段 = 空集 = 纯（空 = 纯参与精确比较）。接口方法 override（M6a 默认方法机制）：impl 提供体走同一签名+集比较——默认体空标记自然过（真体 M8，D13）。被拒：单向 ⊆（接口语义面 impl 是**实现**不是子类型——规范明文精确相等）。

## D8 E1405：顶层初始化器纯（ch16 R6）

挂 pass 2b topLet 既有检查：初始化器表达式走查调用集（D4 收集器复用），非空 → E1405（锚首个效果调用首 token，报文含 callee 与效果名，注册表 title "effectful call in a top-level initializer"；remediation "Move the work into main"）。闭包构造豁免（构造 ≠ 执行，D6 同裁定）；panic 族调用零集合法（`let x = panic(…)` M6b 已绿，验证不回归）。被拒：初始化器禁一切调用——纯字面/纯函数调用合法（集空即过）。

## D9 多模块：合格标签与可见性（ch15 既有面复用）

装载层零改；跨模块面全在 D3 的 resolveEffectTag：合格 `b.db` 走 importSym 扩 kind（E1304 declares-no / E1303 non-pub 既有门）；后序检查序（被引先于引用者）既有——b 模块的 `effect db` 声明在 A 检查前已 ingest，裸/合格两写法 canonical 同键。顶层 let 跨模块初始化序（M6b 编译期计算）与 E1405 正交（序是运行面、纯是静态面）。

## D10 codegen 与运行时：零面（Q2，原则 10 三要素闭环）

类型检查 = 本变更全部（四族检查机器）；**代码生成** Emit 不触：效果段在声明/签名上（codegen 擦除声明面既有——FnDecl/方法签名不产 IR 前已忽略新增字段）；fn 型 tags 只出现在类型位（注解/参数定型期消费，IR 无类型注解）。`build` 对带效果程序行为与今日纯程序同构（M4 接收集维持——main 体单返回语句边界不变，带效果调用自然停 bndMainBody，诚实行）。**运行时** ch16 无面：效果是纯编译期纪律（无处理器、无运行钩子、无效果注解的任何运行时可观察行为——与资源纪律同形：M6b 运行面仅非目标披露），非目标第 3/6 条披露多态与处理器不在语言内。

## D11 黄金锁定枚与翻绿枚

- **锁定 2**（今日已对，钉住不改）：fn 声明位裸标签 `fn f() io {` → E0105；类型位 keyword `fn() effect io -> T` → E0105。
- **翻绿 1 类**（Q3 披露的今日假报）：纯闭包入效果注解槽 `let f: fn(Int64) io -> Int64 = \|n\| n + 1` 今日 E0501 → M7 合法——零既有黄金携带（grep 实证），新黄金钉绿面非改写。

## D12 黄金表（T1 落盘计划）

**~50 新增 + 0 改写**。分布预估：E1401 ×6（直接名/方法/接口值/fn 型值/defer 归属/多标签差集）；E1402 ×5（纯入 io 合法（翻绿钉）/效果值入纯槽拒/超集拒/结构不一致仍 E0501/嵌套 fn 型精确）；E1403 ×2（io/net）；E1404 ×4（impl 多/缺/两侧绿/接口缺省段对 impl 缺省段）；E1405 ×3（直接调用/经纯 fn 链/闭包构造豁免绿）；E1304 ×3（裸未声明/合格未导入/合格 non-pub）；E0012/E0404 ×3；E0105 ×5（闭包段 keyword/闭包段裸标签/声明位裸标签锁定/类型位 keyword 锁定/闭包段绿例不存在改钉合法全注解闭包）；声明段三面绿例 ×4（fn/接口/impl/多标签）；闭包推断 ×5（推断集入一致位绿/嵌套不并入/构造不污染/topLet 构造豁免/短闭包推断）；跨模块 ×3（合格标签绿/b.db 两写法同键绿/未导入 E1304）；panic 零效果 ×2（绿/顶层 panic topLet 绿）；defer ×2（体调用计入外围绿与拒）；综合绿 ×2（三族全绿程序/多模块效果程序）。0 改写：既有 356 枚无人声明效果（五停点实证），唯一行为差面（D11 翻绿类）零黄金携带。

## D13 不可达与边界披露（实现期逐条验证）

1. **ch18 task 段拼写**（`task effect net`）：`task` 块停 bndConc（chapter 18 (concurrency) forms），段 token 不可达——真机探针实证。
2. **ch20 test 免检查**：`test` 块停 bndTest（chapter 20 (testing) forms），体内效果调用不可达——同上。
3. **等待原语零效果**：ch16 规范句在（wait 等待不引入效果），ch18 原语本里程碑无实现——句面遵守（无机器可违），运行面 M9。
4. **接口默认方法体**：默认体是真实走查代码（check-e0815-default-body 既有证据——体内方法调用已检查），体内效果调用计入**该接口方法自身**的声明集；override 走 E1404 精确一致（含段位）。
5. **内建三词无库内容**：io/net/time 是合法标签与冲突面，无任何 std 函数携带其注解——M8。
6. **效果多态/处理器**：规范未定义——无面可落。
