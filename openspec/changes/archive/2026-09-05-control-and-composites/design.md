# design.md — control-and-composites

机制自由度的固定记录。每条决策给出规范依据（章 R 名 / 注册表码）；无依据处显式标注为机制读法并给出论证。M5 是纯前端拓宽（裁决 Q2：代码生成接受集不扩，见 D11），一切决策服务于 `we check` 的接受/拒绝面。

## D1 AST 扩展与位置策略

新节点（`internal/ast`，全部携带首 token 的 1 起行列，沿 M2 惯例）：

- **语句**（`stmt()`）：`While{Cond, Body}`、`Loop{Body}`、`Break`、`Continue`、`Defer{Block}`。if/match 是表达式（ch3/ch4 明言），经 ExprStmt 作语句项。
- **表达式**（`expr()`）：`If{Cond, Then Block, Else *Block | *If}`——else-if 链按 ch3 = 嵌套 if 表示（Else 指向内层 If 节点；`else { if ... }` 与 `else if` 同形）；`Match{Scrutinee, Arms []MatchArm}`；`Construct{Head NamedType 形, Fields []FieldInit{Name, Expr}, Base Expr?}`——Base 非 nil 即更新表达式（同一节点两用法，ch8 R3/R4 同头同字段语法）；`Tuple{Elems []Expr}`；`Closure{Params []Param, Ret TypeRef?, Body Block, Short bool}`——短形式体为单表达式时归一为 `Block{Items: [ExprStmt]}`（解析期归一，检查器只见块）。
- **模式**（新接口 `Pattern`，独立于 Expr/Stmt）：`PatLiteral{Kind, Text}`、`PatWildcard`、`PatBinding{Name}`、`PatOr{Branches []Pattern}`（平坦 n 叉）、`PatTuple{Elems []Pattern}`、`PatVariant{Name, Args []Pattern, Qualified bool}`。
- **声明**（`item()`）：`RecordDecl{Pub, Cat("gc"|"value"|"resource"), Name, Fields []FieldDecl{Name, Typ}}`、`NewtypeDecl{Pub, Name, Underlying}`。泛型/derives 子句不解析（bndGener 维持，非目标）。
- `Binding.Name` 名位扩展：新字段 `Pat Pattern`——非 nil 时为元组模式（Name 为空）；单名绑定维持原字段（零改写既有路径）。

模式不实现 `expr()`：模式位与表达式位是不相交的文法位（ch4 明言），AST 上也分离——`let Circle(r) = s` 在解析层即拒（E0105），不可能构造出混用节点。

## D2 ch3 解析（裁决无关，章文直接落地）

**语句分发**（parseStmt 关键字列扩展）：`while`/`loop`/`break`/`continue`/`defer` 解析为对应语句；`if`/`match`/`fn`/`true`/`false` 落入表达式语句路径（ch2 关键字引导的表达式形）。checkStart 语句起始类相应扩充（这些关键字成为合法语句首）。

**E0202 值位判定**：`while`/`loop`/`break`/`continue`/`defer` 在 parsePrimary（任何值位——初始化、实参、返回值）出现 → E0202（注册表描述族： ratified valueless statement forms in expression position）；`if` 无 else 在值位 → E0202（ch3 R1）。有 else 的 if 在值位合法（臂块值）。语句位 if 无 else 合法（无非值产生）。

**E0201 循环深度**：parser 增 `loopDepth` 计数（while/loop 体 +1），`break`/`continue` 时为 0 → E0201。**闭包体与嵌套函数体重置**：闭包体是函数体语境（ch12），不是循环体——进入 Closure 体时保存清零、退出恢复（机制：沿 `p.fns` 的既有栈式保存恢复模式）。E0201 属解析层（与 E0401 return-outside-fn 同层先例：纯位置知识，无需类型）。

**E0203/E0204 defer 放置**：`defer` 后非 `{` → E0203；defer 不是函数体顶层项（在 if 臂/while 体/loop 体/match 臂块/嵌套块内）→ E0204。机制：parseBlock 增 `direct` 标记——函数体顶层块 direct=true，其余（控制流臂、嵌套块、闭包体内再嵌套……闭包体自身是函数体顶层，direct=true）传递判定。defer 体本身 parseBlock(nil)（非函数语境）：体内 `return` → E0401 既有（ch6「defer 体内无 return」由既有 p.fns 空栈路径命中，ch3 明言）。

**if 臂作用域**：臂是块（parseBlock），块即作用域（M3 walkItems 既有压栈）；arm scope isolation 无需新机制。

**行拼接**：if/match/while/loop 的头表达式按 ch2 既有规则（深度 0 续行集）；块花括号不增深度（既有）。

## D3 match 解析（ch4 逐条）

**臂组不是 ch2 块**：新 `parseMatchArms`——`match expr {` 后，每臂起于自身行（行界分隔 = ch2 boundary inference 同款判定：前一臂体完成后遇行界即新臂）；臂 = `pattern [if cond] => body`，body 是一个表达式（块经 `{` 进入 BlockExpr——ch4「一条规则」）。**臂间逗号 → E0105**（`unexpected token — "," ...`，指明臂以换行分隔、无分隔符 token——报文按 E0105 既有模板组合）。**零臂 → E0301**。臂体内完成后遇 `}` 收组。守卫 cond 按值位表达式解析（续行规则适用）。scrutinee 按值位解析。

**模式文法**（`parsePattern`，递归下降）：

- 字面量：int/float/string/rune/bool token → PatLiteral。**负数值不是 ch1 字面量**（`-1` 是一元负 + 字面量）→ `-` 起头不属模式 → E0105（机制读法：ch4 字面量模式 = "a chapter-1 literal"，ch1 字面量无符号；披露于 D14）。
- `_` → PatWildcard（`_` 永非绑定名）。
- camelCase 标识符 → PatBinding（E0012 检查沿 checkCamel）。
- PascalCase 标识符：裸 → PatVariant{Args: []}（单元变体模式）；`(` 后 → PatVariant{Args: sub-patterns}（载荷子模式递归）。限定形 `mod.Name`（Receiver 位）→ PatVariant{Qualified}（M5 单模块下解析接受、类型侧 E1303 系不可达——多模块是 bndMultiModule 既有边界；披露）。
- `(` → 元组模式：≥2 子模式（`(p)` 单元素与 `()` 空形皆非已批模式 → E0105——元组模式是 ch8 的 2–8 元；`()` 不是模式（ch4 模式集无 unit 模式）；披露于 D14）；>8 元模式**不设 E0602**（注册表描述只覆盖 type 与 expression 两处）——模式照解析，类型侧对 scrutinee 必 E0501（不存在 9 元元组类型可匹配，注册表 E0602 描述之字面读法）。
- `|` → PatOr 平坦收集（`a | b | c` 一节点三支）。守卫 `if` 作用于整个或模式之后（先收尽 `|` 链再看 `if`）。

**句法名字集检查**（解析层，ch4 明言 "The name-set check is syntactic"）：单模式内同名二绑 → E0404（含元组嵌套递归收集）；或模式各支名字集不等 → E0302。类型侧不再重复名字集，只查**同名同型**（E0501，D8）。

**let 名位仅不可驳模式**：Binding 名位 `( ` 进元组模式（子模式递归仅接受 binding/wildcard/元组——PatVariant/PatLiteral/PatOr 出现 → E0105，ch4「refutable patterns are match-only」）；PascalCase 名位（`let Circle(r) = s`）→ E0105（现路径已拒——`(` 检查前 default 分支，维持并改进报文指向）。

## D4 ch8 解析

**顶层声明**：`record Name { fields }`、`byval record`、`byres record`、`newtype Name(Underlying)`、各可 `pub` 前缀。字段 `name: type` 逗号分隔（尾逗号一律允许，沿实参表先例）；零字段合法；字段名 E0012、类型名 E0011、E0404 入模块名字表（既有 declare）。花括号 = 括号（行拼接不敏感——构造同款，ch8 R3 明言构造括号是 brackets；声明侧同机制读法）。泛型子句 `<`、derives 子句 → bndGener 维持（ch10）。

**构造/更新表达式**：parsePostfix 的 `{` 分支（现 bndComp 点）落地——头为 PascalCase 裸名/成员链（既有 chainHead 判定，`module.User` 合法头形）；括号内 `field: expr` 逗号分隔；`with &` 尾随 → 更新（Base = `&` 后**一个后缀表达式**——成员/调用链；二元运算须括号。机制读法：ch8 未钉 `&` 后优先级，最简消歧 = 后缀链，`with &(a + b)` 分组可达；披露于 D14）。括号深度 +1（brackets）。字段名重复 → E0604（解析层即可判，归类型侧统一报——见 D6 报文锚点）。`with` 非 `&` 跟随 → E0105。

**元组表达式**：parsePrimary 的 `(` 分支——现 `,` 即 bndComp 点落地为 Tuple 收集（≥2、逗号分隔、尾逗号允许）；>8 → E0602。`(e)` 维持分组（折叠，既有）。`()` 维持 Unit（既有）。**E0602 两处统一**（注册表描述字面：type 与 expression）：parseParenType（类型槽，现无上限检查——补）与元组表达式；元组模式超元走 E0501（D3）。

**绑定头元组模式**：parseBinding 的 `(` 分支（现 bndComp 点）→ D3 模式文法（不可驳限定）。

## D5 ch12 解析

**完整闭包**：parsePrimary 的 `fn` 分支（现 bndClosur 点）——`fn` 后 `(` = 闭包（表达式位；顶层 `fn` 后跟名字 = 声明，ch6 既有——**一 token 前瞻**，ch12 明言）。参数表 = ch6 全注解形（`name: type` 逗号分隔；`mut` 参数 → bndMutPar 既有）；返回型 `-> type` 可省（省 = 无值）；体 = 块（函数体语境：p.fns 压栈携带 hasRet——体内 `return expr` 于无返回型闭包 → E0402 既有路径；defer 合法 = 函数体顶层；loopDepth 重置见 D2）。语句位 `fn`：parseStmt 表达式语句路径命中（闭包值是表达式；其丢弃受 E0605——fn 型非 unit）。

**短闭包**：parsePrimary 的 `|` 分支（现 bndClosur 点）——`|p1, ..., pk| body`，k≥1（**`||` 是 ch1 逻辑或单 token**——maximal munch 先于模式拆分，`||` 后非表达式位 → E0105「零参短形式不存在」，ch12 场景明言；`| |` 空参间隔形同拒 E0105——参数表以名字开头）。参数 `name` 裸或 `name: type`。**体最大化**：body = 一个表达式或 `{ ... }` 块；解析上 body 从一个 primary 起按**完整表达式**解析（后缀与二元全部归体）——故闭包自身无后缀（`(|x| x + 1)(2)` 须括号，ch12 明言）且**作二元/一元操作数须括号**：实现 = parsePrimary 产短闭包仅在 primary 位可达，而二元循环的操作数调 parseUnary→parsePrimary——操作数位的 `|` 自然不可达（`1 + |x| x` 的 `|` 在二元循环中被当下一 token → E0105 unexpected token，ch12 场景钉死的就是这个报文）。**语句首 `|`** → E0102（既有 statement-start 类，M2 已钉——黄金补一枚正例锁定）。

**方法值**（`u.describe` 无调用）：解析不拒（Member 节点照常）；类型侧按 Q3 裁决（D10）——M5 无方法可存在，记录上非字段名 → E0816 已涵盖；ch12 的 E0105 方法值面在 M5 不可达（无方法名可解析），披露于 D14。

**fn 型注解**：parseFnType 既有（含效果段裸标签）；M5 不改。

## D6 类型侧：声明登记与复合值

**符号表扩展**：`symRecord{cat, fields []{name, Type}}`、`symNewtype{underlying}` 入 c.syms（E0404 由解析层 declare 保证不撞）。resolveNamed 增两kind：记录 → `recordType{decl}`（新 Type 形，String = 名）；newtype → `newtypeType{decl}`（String = 名）。**类别函数** `catOf(t Type)`：base/unit → value 系；recordType → 声明类别；newtypeType → **底层派生**（newtype 布局擦除 = 传递语义随底层——机制读法，ch8「zero-cost wrapper whose layout is erased」；`byval record Bad { id: UserId(Int64) }` 依派生读作 value 合法，字面读作「newtype ≠ value 类别」会荒谬地拒绝零成本包装；披露于 D14）；sum（namedType 指向 sumInfo）→ 声明类别（gc 默认 / byval = value）；tupleType → **逐元派生**（全 base/value → value；含 gc → gc；含 resource → resource——机制读法，见 D9 捕获）；fnType → gc（闭包 gc 类别，ch12）。

**E0601/E0702**：byval record 字段型 catOf 必须 value 系（base/unit/value/newtype-over-value/tuple-of-value）否则 E0601；byval sum 载荷同规 E0702（M3 缺口补齐）。gc/resource 无限制（引用传递）。

**构造/更新检查**：构造——头解析为记录（非记录头：sum/newtype-by-call 之外的 NamedType → E0603「construction head ...」；**newtype 构造走调用形** `UserId(42)`，ch8 明言 call form，在 callType 分派）；字段集三违（未声明/遗漏/重复）→ E0604（锚点：违例字段名位置，无违例字段时锚构造头）；字段表达式对声明型 E0501（expected 线程）。更新——Base 型 ≠ 头记录型 → E0603；命名字段未声明/重复 → E0604（更新不要求全字段——只查声明性与唯一性）；命名字段表达式 E0501；头 cat = resource → E0606（先于字段检查——类别是声明级事实）。

**newtype**：callType 中 callee 名解析为 symNewtype → 构造：恰一实参（非一 → bndArityGap 既有缺口边界）、实参型对 underlying E0501；产 newtypeType。`.value` 成员（D10）。

**元组**：tupleType 既有（M3 注解侧）；Tuple 表达式 → 逐元 typeOf，产 tupleType；注解/实参/返回位的逐元 E0501 经既有 agree（sameType 对 tupleType 逐元——M3 已有）。元组模式（match/let）→ D8。

**局部遮蔽**：M3 机制（同名后绑覆盖同作用域 map 项）即规范语义（后绑胜出、旧绑失名）——M5 补黄金钉死（`let x = 1; let x = normalize(x)` 型变 Int64→String 后续使用按 String；参数遮蔽；嵌套就近）。

## D7 类型侧：控制流

**if**：Cond typeOf 须 Bool 否则 E0503（`conditions must be Bool` 族——if/while/guard 同码，ch7 R6）。有 else：Then 块型 vs Else 块型（无值臂 = unit，ch8 R If arm agreement；经 agree——Never 臂豁免既有）不合 → E0501 锚 if 首 token；值位 if 的型 = 一致型。语句位：一致型非 unit → E0605（ch8 丢弃点「control form's body」+「statement-position」——报文锚 if）。**控制流体块的新 walk 模式**：现 walkItems 的 plainBlock 尾表达式 = 块值（不丢弃）；控制流体块（if 臂、while/loop 体、defer 体）的尾非 unit 表达式 = 被丢 → E0605（ch8 R Value discard 明列「the final item of a control form's body block」）。机制：walkItems 增 mode 参数（plain / controlForm / fnBody 既有二元展开为三元；fnBody 尾规则既有不动）。无 else 语句位 if：Then 按 controlForm 模式走（臂值被丢）；无类型面。else-if 链 = 嵌套 If（D1），递归自然。

**while/loop**：语句（解析层已保证不在值位）；Cond（while）E0503；体 controlForm 模式；类型 = 无值（语句）。**break/continue**：解析层 E0201 已保证位置合法；类型侧无事。

**defer**：体 = controlForm 模式（尾非 unit → E0605；`defer { step() }` step()->Int64 拒——机制读法：ch8 丢弃点枚举「control form's body block」覆盖 defer 体；披露于 D14）。效果计数（E1401）与 `?`（E1202）不可达（非目标披露）。

**块值交互**：BlockExpr（plain 块）维持现行为（尾表达式 = 值）。

## D8 类型侧：match 与穷尽性（裁决 Q4 定验证深度）

**模式类型化** `checkPattern(p Pattern, scrut Type, binds map[string]Type)`：

- PatLiteral：字面量型（M3 literalType 复用）对 scrut 经 agree 不合 → E0501（机制读法：ch8 对元组模式明文 E0501「against a scrutinee that is not a tuple of the same arity」；字面量模式同理——「matching a scrutinee equal to the literal」蕴含 scrut 型可等于该字面量型；bool 字面量对 Bool scrut 合、对 Int64 scrut 拒；披露于 D14）。
- PatWildcard：任意 scrut，无绑定。
- PatBinding：任意 scrut，binds[name] = scrut。
- PatTuple：scrut 须 tupleType 同元（否则 E0501——ch8 明文），逐子递归。
- PatVariant：scrut 须 namedType 指向 sumInfo（否则：字面量读法下 E0501？——**非 sum scrut 上的变体模式**：PascalCase 模式对 base/record scrut → E0303（「names no variant of the scrutinee's type」字面成立——scrut 型无变体集；机制读法：E0303 比 E0501 更精准，注册表描述「The pattern's Name is not a declared variant of the scrutinee's sum type」涵盖 scrut 非和式之形；披露于 D14））；Name 非该 sum 变体 → E0303；Args 数 ≠ 声明载荷数 → E0304；逐载荷子模式递归（子模式型 = 载荷型）。
- PatOr：逐支独立 checkPattern（各支产 binds 副本），**同绑定名同型**经 agree 不合 → E0501（ch4 R3「their bound types MUST agree」）；名字集已由解析层 E0302 保证。合流 binds = 任一支（同集同型）。

**守卫**：Cond 在「外围作用域 + 该臂 binds」内 typeOf，须 Bool 否则 E0503；守卫懒惰性是运行时命题，M5 无面。

**臂体**：在「外围 + binds」作用域（新压一层）typeOf(arm.Body)。**值位 match**：各臂体型经 agree 链不合 → E0501 锚 match 首 token；型 = 一致型（全 Never 臂 = Never，agree 既有）。**语句位**：一致型非 unit → E0605 锚 match（ch4 R Match arm agreement 明文「the unbound match-arm body in statement position」）。

**穷尽性 = Maranget 有用性算法**（机制实现，ch4 Exhaustiveness Requirement 的判定器）：

- 构造集：namedType-sum = 变体（各载 k 元）；tupleType = 唯一元组构造子（k 元）；base/unit/fnType/recordType/newtypeType = **无穷域**（字面量永不覆盖——ch4 明文「literal patterns never cover Int64, String, or Bool」推广到一切基类型与未枚举型；Bool 不设特例）。
- `useful(patterns []Pattern, q Pattern) bool`：标准递归——取首构造子位分类（wildcard 类 / 具体构造子类），对 q 的每候选构造子递归特化（Maranget 2007 usefulness；无守卫、无绑定-构造区分——binding 等价 wildcard）。
- **E0305**：`useful(unguardedArms 的模式列, wildcard)` 为真（有漏）且**全 match 无守卫臂** → E0305，报文点名首个漏构造成（sum → 变体名；无穷域 → 型名）。**E0306**：同样有漏且**存在至少一守卫臂** → E0306（ch4「a match containing a guarded arm MUST retain an unguarded exhaustive fallback」字面——守卫臂的存在使补全义务成立，漏在何处不改判；场景「Circle 守卫 + Rectangle 无守卫、Circle 无兜底」命中）。
- **E0307**：对第 i 臂，`useful(前 i-1 臂中**无守卫**臂的模式列, 第 i 臂模式)` 为假 → E0307（「its every value already covered by earlier arms」；守卫前臂不计——守卫可能假，后臂仍可达；通配后臂、变体二列、字面量重复皆命中）。
- 验证深度（裁决 Q4）：章内全部穷尽性场景黄金 + 对抗矩阵单测——含假穷尽反例 `(Some(a), None) | (None, Some(b))` 对 `(Option, Option)` 必须报非穷尽（逐位并集会误判穷尽——积空间推理的必要性证明）、三元组组合、或模式展开、嵌套变体。

## D9 类型侧：fn 值与闭包

**fn 名值位**：identType 的 symFn 分支（现 bndFnValues 点）→ `fnType{params, ret}`（M3 fnParams/fnRets 表复用；无效果段——声明携带段的解析维持 M2 现状，见 D14 披露）。签名精确一致：fnType 进 sameType/agree（params 逐位 + ret；效果标签串相等——M5 标签只能来自注解侧，声明侧恒空，含标签注解无值可绑 → E0501 必然，披露）。**经 fn 型值调用**：callType 的 callee 型为 fnType → 实参逐位 E0501（checkArgs 复用）、返回 ret；实参数不合 → bndArityGap 既有边界（follow-up #5）。

**闭包类型化** `closureType(x *ast.Closure, expected Type)`：

- Params 全注解 → 自足：param 型逐个 resolveTypeRef；ret = Ret 声明或体块值（体走函数体语境 walk：fnValued = Ret 非 nil）；产 fnType。
- 裸参（任一）→ 依赖期望：expected 非 nil 且为 fnType → 裸参取期望对应位型；**标注参与期望对应位不合 → E0501**；ret 取体块值（期望 ret 不回灌——ch12「the return type comes from the body」）。expected nil 或非 fnType → **E1001**（锚闭包首 token）。
- 期望线程：binding 注解（既有）、实参（checkArgs 既有 expected 线程）、返回位（checkReturn 的 c.fnRet）、let-元组模式位（逐元）。

**捕获账本**：checker 增 `closures []map[name]captureInfo` 栈（每活跃闭包一层）。闭包体检查期间：名字解析命中**闭包边界之下**的作用域层（locals 栈深 < 闭包进入时深度）或模块 syms（symLet）→ 记入**每层**其边界在被命中层之上的账本（嵌套闭包传递：内层用外层之捕获 = 外层亦须捕获——ch12「its captures keeping the reach they were created with」的静态面）。账本条目按 catOf(绑定型) + 绑定 var/let：

- gc（含 fnType、含 gc 元组的 tuple、底层 gc 的 newtype、gc sum）→ **活引用**：体内 Assign 目标为该名 → 合法。
- value 系（base/unit/value record/value sum/值元组/底层 value 的 newtype）→ **拷贝快照**：体内 Assign 目标为该名 → **E1003**（锚赋值目标）。
- resource（byres record / 含 resource 的 tuple / 底层 resource 的 newtype）→ **E1002**（锚捕获使用点——首个命中名处）。
- 顶层 let 捕获同规（按型类别；模块绑定无 var——E0403 既有保证）。
- 赋值目标可变性（let 重赋值）无已批规则——非捕获位维持 M3 现行为（通过），follow-up #7（D14）。

**闭包体 = 函数体语境**：return 携闭包值（E0402 值形/无值形按 Ret）、defer 合法（E0204 顶层）、E0401 语境枚举含闭包（ch12 修订句）——解析层 p.fns 压栈已承载（D5）。闭包值本身：fnType = gc 类别——可绑定/传参/返回/再捕获（catOf(fnType)=gc 使其作为外层绑定被内层闭包捕获时走活引用——一致）。

## D10 成员解析（裁决 Q3 定深度）

memberType 重写（现 bndStdModules 全停点）：

- 接收者型 recordType → 名为声明字段 → 字段型；非字段 → **E0816**（ch10 R Member name resolution 的记录侧在 M5 的完整实例：无方法可存在——impl/derives 不解析——成员集 = 字段集恰成立；码与触发语义均已批，实现次序不改变其真值；黄金锁定）。**注册表 E0604 描述含「member access names an undeclared field / Fires under: Field access」陈旧子句**——与 ch8 R5（经 ch10 修订）的 E0816 路由冲突（「unknown names are one rule, one code」）；章文是触发语义权威（ch99 权威分立：chapter=trigger / registry=entry），M5 发 E0816，E0604 描述子句登记 roadmap follow-up（#8，注册表维护——由下一个触注册表的规范层变更或专门维护变更清除；本变更为 internals-only 不触 diagnostics.toml）。
- 接收者型 newtypeType → `.value` → underlying；其余名 → E0816（`.value` 是唯一已批 newtype 成员）。
- 接收者型 base/tuple/unit/fnType → 成员面属 stdlib/后续（String 方法是 stdlib 表面；元组无 `.0` 位——ch8 明言只有模式解构通道，`.0` 在解析层已拒 E0105「member names are identifiers」）→ 维持 **bndStdModules** 边界（What 文本不变：`standard-library modules (chapter 15)`——机制自由度的措辞：基类型成员 = stdlib 方法面 = stdlib 模块面未实现；既有 What 复用零新行）。
- 接收者是未解析裸名 → E1304 限定符报文（既有路径维持）。

## D11 codegen 最小增量（裁决 Q2 推荐）

Emit 的模块走查增 RecordDecl/NewtypeDecl 分支 → **擦除**（与 SumDecl 同机制：声明位零 IR；使用位全部在 main 体内 → bndMainBody 既有边界兜住）。What 表**零新增零改名**；bndOtherFns/bndTopLets/bndMainBody/bndErrPayload 四行原样。新形式程序（if/match/record/闭包体）穿类型检查后在 main 体形状处停边界 exit 70——真机披露（黄金一枚代表：控制流程序 build → 70）。runtime/ 零改动。

**不扩 IR 的论证**：闭包与 gc 记录的诚实 IR 依赖 M8 的 GC 设计（追踪单元格布局未定）；控制流 IR 的可观察收益在 M5 程序集上为零（程序仍只能以 `Ok(())` / `Err(字面量)` 收尾——Err 载荷边界 M4 已裁）；评价门以 `we check` 为核心（FPCR 不需要代码生成）。IR 拓宽随 GC 落地后的里程碑。

## D12 边界表收敛

- 解析器 What 表 15 → 11：删 `chapter 3 (control flow) forms`、`chapter 4 (match) forms`、`chapter 8 (composites) forms`、`chapter 12 (fn types and closures) forms`。保留：bndIter/bndGener/bndScope/bndErr/bndEffect/bndColl/bndConc/bndFFI/bndTest/bndMutPar。
- 类型检查 What 表 12 → 11：删 `function values (chapter 12)`。保留其余 11（bndStdModules 语义收窄为基类型成员面——D10，What 文本不变）。
- 钉住被删行的黄金 2 枚（check-boundary-ch8 / check-boundary-ch8-json）随形式落地**改写**为真实 ch8 行为（记录声明检查绿）——行为变更即里程碑本义，披露于完成记录；其余 121 枚零改写。

## D13 conformance 黄金表（T1 落盘契约，约 60 枚）

**负例（每码至少一枚，消息逐字节）**：E0201 break / E0201 continue / E0202 if-无-else-值位 / E0202 while-值位 / E0203 defer-非块 / E0204 defer-嵌套 / E0301 零臂 / E0302 或模式名字集 / E0303 未知变体 / E0303 非-sum-scrut 变体模式 / E0304 载荷数 / E0305 缺变体 / E0305 基类型无通配 / E0306 守卫无兜底 / E0307 通配后臂 / E0307 变体二列 / E0307 字面量重复 / E0503 if-条件 / E0503 while-条件 / E0503 守卫条件 / E0601 值记录-gc-字段 / E0702 值和式-gc-载荷 / E0602 元组型-9 / E0602 元组式-9 / E0603 构造头-非记录 / E0603 更新基型 / E0604 未声明字段 / E0604 遗漏字段 / E0604 字段重复 / E0606 资源更新 / E0816 记录未知成员 / E0816-newtype-非-value 成员 / E1001 裸闭包无期望 / E1002 资源捕获 / E1003 捕获值赋值 / E0501 或模式同名异型 / E0501 臂不一致 / E0501 构造字段型 / E0501 签名不合 / E0605 语句位-match-非-unit / E0605 语句位-if-非-unit / E0605 控制流体尾 / E0105 臂间逗号 / E0105-let-变体模式 / E0105-零参短形 / E0105 短闭包操作数位 / E0102 语句首竖线（正例锁定既有）/ E0105 负字面量模式 / E0105 单元模式。（~49）

**正例（分组）**：if 值位+else-if 链 / if 语句位-unit 臂 / while+break+continue / loop+break / defer 顶层绿 / match 值位全变体穷尽 / match 守卫+兜底 / match 元组逐元穷尽（含积组合枚举）/ match 绑定+或模式+嵌套 / match-Int64-通配 / 记录构造+访问+更新 / byval-记录拷贝（静态）/ newtype 构造+解包+等价拒（E0501）/ 元组式+let-解构+match-解构 / unit-值+空块 / 遮蔽三枚（同域后绑/参数/嵌套就近）/ fn-名绑定+经值调用+作实参 / 完整闭包（有值+无值）/ 短闭包（标注/裸+注解/裸+实参）/ 闭包 return-仅出闭包 / gc-var-活捕获绿 / 值捕获快照绿 / 嵌套闭包传递捕获 / 构造头 module 形解析位（单模块下 E1303 不可达——改钉 std 边界代表）。（~22）

**边界收敛**：check-boundary-ch8 两枚改写（D12）；build 侧代表 1 枚（控制流程序 build → main 体 What 70）。合计 ≈ 60 新增 + 2 改写。全部先红（T1 证据：对当前构建跑失败清单）。

## D14 已知缺口与披露（非目标展开）

1. **赋值目标可变性无已批规则**：ch2 定赋值语句形、ch8 定遮蔽（新绑非改写）、ch12 定捕获位赋值（E1003），但「赋值到 let 绑定」的拒绝规则与诊断码不存在于任何已批章。M5 维持 M3 现行为（类型检查通过），**登记 roadmap follow-up #7**（赋值目标可变性规则——建议未来 ch2/ch8 修订分配码）。非捕获位与捕获位一致遵循（捕获 gc-活绑定赋值放行 = 现行为延伸）。
2. **E1004 不可达**：泛型 fn 声明不解析（bndGener，M6），无泛型名可入值位——码与检查器位留 M6。
3. **E0105 方法值面不可达**：M5 无方法可解析（ch10 是 M6）；记录非字段成员已由 E0816 覆盖（D10）。
4. **E1202/E1401 defer 侧不可达**：`?` 停 bndErr（M6）、效果系统是 M7。
5. **负数值与 `()` 非已批模式**：`-1 =>` / `() =>` → E0105（ch1 字面量无符号；ch4 模式集无 unit 模式）。补齐通道 = 模式集的规范层修订。
6. **fn 声明携带效果段**：维持 M2 现状（解析层拒绝/边界），ch16 是 M7；fn 型**注解**侧效果标签惰性可解析（M3 既有），无值可绑（无声明携带段）→ 含标签注解绑定必 E0501。
7. **`with &` 后优先级**：Base 取一个后缀链（二元须括号）——ch8 未钉，最简消歧机制读法。
8. **限定变体模式**（`mod.Name(...)`）：解析接受、类型侧多模块边界（bndMultiModule）先行——E1303 面随 M6 模块解析。
9. **元组/newtype 捕获类别派生**：ch12 枚举 gc/value/base/resource 四答，元组与 newtype 未逐字列——逐元/底层派生是「captures answer chapter 8's ownership categories」的机制读法（D6 catOf）。
10. **E0303 对非-sum scrut**：变体模式对 base/record scrut 报 E0303 而非 E0501（注册表描述涵盖「scrutinee 无变体集」之形；比混合类型更精准）。
11. **字面量模式对异型 scrut**：E0501（D8 机制读法）。
12. **defer 体尾丢弃**：E0605（ch8「control form's body block」枚举的机制读法覆盖 defer 体）。
