# tasks.md — control-and-composites

- [x] T1 测试先行 A：D13 黄金表落盘（约 60 枚新增 + 2 枚改写），对当前构建运行必须先红
  来源：proposal 目标 1 2 3 4 5 7 8 / design D13
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红（新增用例失败清单 + 改写 2 枚失败，其余 121 枚既有全绿），red 证据记于完成记录

完成记录（T1，2026-09-05）：
- 实际落盘 **80 枚**（超出 D13 估算 ~60：逐码负例 50 + 分组正例 25 + 边界/代表 3 + 改写 2；估算为「约」，多出部分为 E0501 四个额外位面（或模式/臂一致/构造字段/fn 签名/newtype）与 build 正例）。生成器 /tmp/gen_goldens.py 以「源串 + 子串定位锚点」产出，杜绝源与行列漂移；锚点定位笔误 5 处（e0307-after-wildcard/e0603-update/e1002/e0501-fn-signature/e0501-newtype 行号）被生成器 IndexError 当场拦截修正。
- **red 证据**：对当前构建（M4 基线）跑 `go test ./internal/conformance/ -run TestGoldenCases` → **78/80 失败**（全部为 want 诊断 vs got 边界行 exit 70、或 want 绿 vs got 70），既有 121 枚全绿。日志 /tmp/t1-red.log。
- **两枚按设计绿**（既有行为锁定金样，非 red-first 违例——D5/D12 明示）：check-e0102-statement-pipe（语句首 `|`，真机逐字节核对）、check-bnd-std-member（基类型接收者维持 bndStdModules 边界，M5 不得改变）。首版 e0102 金样漏 title 前缀被红测当场抓出（生成器 msg 须完整含 title——一码一消息纪律的机械体现）。
- 改写 2 枚（check-boundary-ch8 / -json）：原钉边界行改钉真实 ch8 行为（记录声明 + 构造 + 字段访问 + 块值，check 绿 / --json 零事件），披露：金样名保留「boundary」字样（D12 计划内改写），语义已翻转为绿。
- [x] T2 测试先行 B：parser 单测扩展（dispatch 表新语句首、模式文法矩阵、闭包两形、构造/更新、E0602 两处 + 模式超元解析照常）与 typecheck 单测（模式类型化、穷尽性对抗矩阵含假穷尽反例、捕获账本矩阵、臂一致表）先写先红
  来源：proposal 目标 1 2 3 4 / design D2 D3 D5 D8 D9
  验证：`go test ./internal/parser/ ./internal/typecheck/` 红（新用例引用未定义节点/分支失败），red 证据记于完成记录

完成记录（T2，2026-09-05）：
- **parser 侧** internal/parser/m5_test.go（10 套件）：ch3 语句与树形（else-if 嵌套 If）/ E0202 值位五面 / E0201 循环深度含**闭包体重置**（短闭包体内 break 报错、闭包体内 while 合法）/ defer 放置 E0203、E0204、闭包体顶层 defer 合法、defer 体内 return → E0401 既有空栈路径（D2 钉死）/ match 臂组 E0301 + 臂间逗号 E0105 / 模式文法矩阵（字面、通配、绑定、**平铺或模式一节点三分支**、元组递归嵌套、限定变体头）/ 模式拒收（E0105 unit/负字面/单括号/尾随令牌、E0302 两形锚 `|`、E0404 同名）/ let 模式位（元组解构绿、可反驳仅 match → E0105）/ ch8 声明（gc 默认、byval、byres、空字段、pub、newtype 两形）/ 构造与更新（点号头 Qual、`with &` 单后缀基式、跨行括号折叠、缺 `&` E0105）/ 元组（`(e)` 折叠、unit 值、**E0602 两处**（型 9 元 + 式 9 元）、**>8 元模式解析照常绿**）/ 闭包两形树形 + 短体最大化 + `||` E0105 + 操作数位 E0105 + 语句首 `|` E0102 锁定 + 闭包函数体语境（E0402）。
- **typecheck 侧** internal/typecheck/m5_test.go（12 套件）：类别诚实 E0601/E0702 / 构造更新 E0603×2、E0604×3、E0606、字段型 E0501 / newtype 调用形、`.value`、拒转化 E0501、E0816 / 元组解构嵌套与注解错配 / 成员解析 E0816 + **基类型接收者 bndStdModules 边界锁定**（D10 不变量）/ 控制流类型化（值位 else-if 链、臂一致 E0501、E0503 三位、E0605 三面：语句位 if/match + 控制流体尾）/ 模式类型化（E0303×2、E0304、或模式同名异型 E0501、守卫 E0503）/ **穷尽性对抗矩阵**（E0305 变体缺、基型无通配、积组合全枚举绿、**假穷尽反例 `(Hit(a), Miss) | (Miss, Hit(b))` 于 `(Opt, Opt)` 必报 E0305 见证 `(Hit, Hit)`**、E0306 守卫无兜底 + 兜底绿、E0307 三形）/ fn 名值位 + 经值调用 + 签名错配 E0501（**fn 型源形式渲染钉入消息** `fn(String) -> Int64`）/ 闭包四绿 + E1001 / 捕获账本（值快照绿、gc 活捕获绿、嵌套传递捕获绿、E1002、E1003）/ 遮蔽三枚。
- **red 证据**：parser 包 **build failed**（`undefined: ast.While/Loop/Break/Continue/Defer/If/Pattern…`，引用未定义节点，日志 /tmp/t2-parser-red.log）；typecheck 包 **12 新套件全 FAIL**（`parse boundary: chapter 3/4/8/12 forms`，TestFnValues 命中既有 `function values (chapter 12)` 边界——该既有钉死行将于 T7 按 D12 收敛移除），既有套件全绿。日志 /tmp/t2-typecheck-red.log。
- 负例消息与行列**逐枚镜像 T1 黄金表**（实现即对表）；两枚非金样新例（if 臂一致、元组注解错配）按同构消息形锚定。
- [x] T3 AST + parser ch3/ch4：D1 节点、D2 控制流产生式（E0201/E0202/E0203/E0204）、D3 match 臂组与模式文法（E0301/E0302/E0404 句法侧、E0105 三形）
  来源：proposal 目标 1 2 / design D1 D2 D3
  验证：`go test ./internal/parser/` ch3/ch4 套件绿

完成记录（T3，2026-09-05）：
- **ch3/ch4 九套件全绿**（TestCh3Statements/IfTree/ValuePosition/LoopDepth、TestDeferPlacement、TestMatchArms、TestPatternGrammar/Rejections、TestLetPatternPosition），既有 10 套件全绿；唯一红为 5 个 T4 套件（ch8/ch12 边界，按设计）。日志 /tmp/t3-parser.log。
- 转绿过程中修正 **4 处 T2 测试笔误**：裸 break/continue 探针须裹 loop（fn 顶层正确报 E0201）；三分支或模式探针改 `_ | _ | _`（各分支名集须一致，否则正确报 E0302）；TestLetPatternPosition 两探针位置 4:9→2:9（源码无前置声明行）；TestClosureFunctionContext E0402 探针 `return`→`return 1`（无值闭包内裸 return 合法）且 part 改「declares no return type」。
- **两枚闭包语境探针移入 TestClosureFunctionContext**（循环深度重置、闭包顶层 defer）——闭包机制属 T4，移动后 T3 之门按套件精确可验。
- **范围披露**：let 头不可反驳元组（D4 binding-head 一片）随模式文法一并落地（parseBinding 与模式机制同源），两枚被取代的边界钉死（TestStatements 的 wantBnd、TestBoundaryForms 探针）于 T3 移除而非 T4。
- parseBinding Pascal 名按 peek `(` 分流：`let Circle(r)` → E0105 可反驳，`let BadName` → E0012（保持 M2 TestDeclarations）。既有表更新：dispatch 表 7 行（if/match/while/loop/break/continue/defer 由 bd 改真实结局）、TestBoundaryForms 删 3 探针、`let fn = 1` part 换新措辞（核对无金样钉旧措辞）。
- **checker 边界兜底**：parser 先行落地后，types 阶段对新树原会 Go-panic（违反 CLI 出码契约）。typecheck 增 4 行 What（复用 parser 词表）于 walkItems default / typeOf If·Match·Construct·Tuple·Closure / checkBinding Pat——ch3+ 树停边界 exit 70，绝不静默跳过、绝不崩溃；conformance 全程可跑。T5/T6/T7 各自删行。
- **conformance 135/201 绿**：12 枚 T1 解析层金样随 T3 转绿（E0201/E0202/E0203/E0204/E0301/E0302/E0404 模式侧、E0105 三形），余 66 枚红全为 T1 新增 M5 金样（T4–T7 材料），**既有 121 枚零回归**。日志 /tmp/t3-conformance.log。
- [x] T4 AST + parser ch8/ch12：D4 声明与构造/更新/元组（E0602 两处 + parseParenType 上限补检）、D5 闭包两形（`||` E0105、操作数位 E0105、语句首 `|` E0102 既有锁定）
  来源：proposal 目标 3 4 / design D4 D5
  验证：`go test ./internal/parser/` ch8/ch12 套件绿

完成记录（T4，2026-09-05）：
- **ch8/ch12 五套件转绿**（TestCh8Decls/ConstructUpdate/TupleExprs/Closures/ClosureFunctionContext），既有 19 套件全绿（24/24）。日志 /tmp/t4-parser.log；gofmt 空、build/vet 过。
- **D4 落地**：parseRecordDecl（record/byval record/byres record + pub 前缀；字段表逗号或换行分隔 + 尾随逗号——括号即括号区，行折叠；字段名 E0012、缺注解 E0105）、parseNewtypeDecl（恰一底层型，多逗号 E0105）；parseItem 分支重排（byval 补 record → "value"、byres/newtype 真产生式；`byval pub`/`pub var` 既有 E0105 钉死保留，尾句更新为 "type or record" / "the chapter 8 declarations"，part 锚不变）；构造/更新：parsePostfix `{` 经 chainPath（点号路径）判 Pascal 头 → parseConstruct——节点锚**头名 token**（p.last），字段初始化 `name: value`，`with &` 单后缀基式（`with u` 于 2:32 E0105 金样逐位核对）；元组：tupleTail + tupleArity 使 **E0602 两处**（式/型）同锚 `(`，parseParenType 改 break 收口后补检上限。
- **D5 落地**：全形闭包 parsePrimary `fn` peek `(` 前瞻（否则 E0105 "where a closure's parameter list opens"），参数表提取 parseParamList 与 fn 声明共享（一处消息一处产生式）；短闭包 `|`：参数 name 或 name: type、短体最大化归一一元块、loopDepth/direct 重置、**shortBody 使体内 break/continue 报 E0201 先于 E0202**（failLoopCtrl 与语句形共用）；**fnBrace 一次性捕获**（parsePrimary 头取后即清）使体首 `{` 得函数体读法（顶层 defer 合法），嵌套/操作数括号仍普通块——M2 钉死 `let x = { return 1 }` E0401 不动；操作数位 `|` 经 operandSlot（binLevels ∪ {!,~}，`-` 由表覆盖）拒绝（金样消息逐字）；`||` E0105 零参（金样消息逐字）；语句首 `|` 维持 E0102 既有锁定。
- **checker 注册边界**：checkModule 增 RecordDecl/NewtypeDecl → bnd(bndCompForms)——声明绝不经由 pass-2 静默跳过（与 T3 的 walkItems/typeOf 兜底同一纪律）。
- **既有表收敛**：dispatch 5 行翻真实结局（fn → dg×2；record/byval/byres/newtype → okRes，rest 改写为合法完成文本）；TestBoundaryForms 删 5 探针；TestExpressions 5 枚 wantBnd→wantClean；compForms8/closureForms 常量随之删除（词表在代码中收敛）。既有钉死逐一复核：`b { }`/`f() { }` E0105 PascalCase、`Box<Int64> {` E0104、`byval pub`/`pub var` E0105、闭包体内裸 return 合法。
- **conformance 139/201 绿**：本步新转绿 4 枚解析层金样（check-e0105-short-closure-operand / check-e0105-zero-param-short-closure / check-e0602-tuple-expr-9 / check-e0602-tuple-type-9，报文逐字节），余 62 枚红全为 T1 类型层金样（T5–T7 材料，逐一核对名单），**既有 121 枚零回归**。日志 /tmp/t4-conformance.log。typecheck 既有 9 套件全绿，M5 12 套件按设计红（边界断言）。
- [x] T5 typecheck 声明与复合：D6 符号表（recordType/newtypeType/catOf）、E0601/E0702、构造/更新（E0603/E0604/E0606）、newtype 调用形、元组、遮蔽钉死
  来源：proposal 目标 3 / design D6
  验证：`go test ./internal/typecheck/` 声明与复合套件绿

完成记录（T5，2026-09-05）：
- **五套件绿**（TestRecordDeclarations/ConstructionAndUpdate/Newtype/Tuples + TestMemberResolution）；既有 9 套件全绿，parser/codegen 包全绿。日志 /tmp/t5-typecheck.log；gofmt 空、build/vet 过。**TestMemberResolution（D10 面）随 T5 落地**——TestNewtype 的 `.value`/E0816 探针要求成员解析先行，故 D10 的记录/新类型两侧在本步实现（E0816 锚**成员名 token**，ast.Member 增 NameLine/NameCol）；基类型接收者维持 bndStdModules 边界不变量，D10 其余收敛留 T7。
- **D6 落地**：符号层 symRecord/symNewtype + recordInfo（cat+fields）/newtypeInfo（underlying）+ sumInfo 增 byval 位；recordType/newtypeType 型形 + sameType 两案；**catOf**（base/unit/never→value；record→声明类；newtype→底层；sum→声明类；tuple 逐元 any-resource→resource else any-gc→gc；fn→gc）与 catNoun（类别诚实消息的非值名词）。类别诚实：E0601 锚**字段类型 token**（pass-2a 解析即检）、E0702 锚 **payload 类型 token**（M3 缺口收口，sumInfo.byval）。构造/更新 constructType：限定头按限定符规则（import→multi-module 边界，否则 E1304）；未解析名→E1304、基类型名→E0603（"a base type"）、和/新类型/值名→E0603（名词分流，金样钉和类型形）；**E0606 类别先于字段检查**（声明事实优先，锚头名）；更新基与头不同型→E0603 锚**基式首 token**；E0604 三面（未声明字段/重复字段锚**违例字段名**、缺字段锚头）；字段值 E0501 锚**字段名**；更新只查声明+唯一，构造查全集恰一次。newtype 调用形 newtypeCall（恰一参数 bndArityGap、E0501 对底层、产 newtypeType）；Tuple 表达式逐元产 tupleType；块级 let 元组解构 checkLetPattern+destructure（不可反驳模式逐位绑定入当前作用域，**模块级解构仍停 bndCompForms 边界**——最后一枚保留行）；遮蔽= M3 覆盖机制即规范语义。
- **TestShadowing 范围披露**：前两例（laterBinding/paramShadow）绿；第三例 nearestBinding 的 if 块内遮蔽依赖 D7 控制流 walk（T6），本步红、T6 转绿——套件按设计留待 T6 全绿。
- **T2 镜像测试修正 9 处**（对照金样逐位列核对）：E0601 3:31→3:29、E0603-update 6:36→6:37、E0604-undeclared 4:30→4:31、E0604-duplicate 4:30→4:31、E0501-field 4:30→4:31、E0816-record 4:19→4:15、E0816-newtype 4:20→4:18、TestTuples E0501 2:20→2:9、newtype E0501 探针**补回金样既有的 `let id = UserId(42)` 绑定行**（镜像漏抄致 E1304 先发）→ 5:9/5:18。前八处为锚点列差或惯例违反，末处为源缺失。
- **金样改写 1 枚（披露）**：check-e0501-newtype-no-conversion 7:20→7:9——与既有 M3 金样 check-e0501-annotation（1:5 名字锚、绿、零改写门）冲突；绑定注解 E0501 的批准惯例是**锚名字 token**，T1 生成器的表达式锚为笔误，按「既有 121 枚零改写」判据修正新增金样而非改写既有惯例。
- **codegen D11 显式擦除提前一行**：build-record-erasure 金样随检查通过而转绿，其机制原是 codegen 首 pass switch 的**隐式落空**——违反「绝不静默跳过」，改为显式 `case *ast.RecordDecl, *ast.NewtypeDecl: continue`（声明零 IR；构造/更新/调用形表达式不进 M4 受纳的 main 体形）。D11 其余（构造表达式擦除）仍属 T7。
- **conformance 158/201 绿**：本步新转绿 19 枚（check-ch8-byval-record/newtype/record-construct-access-update/tuple-destructure、check-boundary-ch8×2、check-e0501-construction-field-type/newtype-no-conversion、check-e0601/e0603×2/e0604×3/e0606/e0702/e0816×2、build-record-erasure），余 43 枚红全为 T6 材料（ch3/ch4、E0303–E0307、E0503、E0605、臂/或模式 E0501、check-ch8-shadowing/unit-value 两枚含 if 程序）与 T7 材料（ch12、E1001–E1003、e0501-fn-signature、build-bnd-new-forms），逐一核对名单（/tmp/t5-clean.txt），**既有 121 枚零回归**。日志 /tmp/t5-conformance.log。真机抽验 E0601/E0702/E0603-update/E0606/E0604/E0816 报文逐字节、--json help 字段齐备。
- [x] T6 typecheck 控制流与 match：D7 三元 walk 模式（控制流体尾 E0605）、if 臂一致/E0503、D8 模式类型化 + Maranget 有用性（E0305/E0306/E0307）+ 臂一致
  来源：proposal 目标 1 2 / design D7 D8
  验证：`go test ./internal/typecheck/` 控制流与穷尽性对抗矩阵绿（假穷尽反例必红转绿）

完成记录（T6，2026-09-05）：
- **四套件绿**（TestControlFlowTyping/PatternTyping/ExhaustivenessMatrix/**TestShadowing**——第三例 if 块内遮蔽随 D7 控制流 walk 落地转绿，T5 披露按期兑现）；既有 9 套件绿，parser/codegen 包绿；仅剩三套件红（TestFnValues/Closures/CaptureLedger，ch12——按设计留 T7）。日志 /tmp/t6-typecheck.log；gofmt 空、build/vet 过。
- **D7 落地**：walkItems 收敛为三元模式（walkPlain/walkControl/walkFn，替代 fnBody+fnValued 双参）——plain 尾即块值、控制流体（while/loop/defer 体、无 else 语句位 if 的 then 块）尾被丢 → E0605「final item of a control form's body」、fn 体尾对声明返回（值位无漏斗以 E0605 fn 形）；语句位 if/match 的非 unit 一致型 → E0605 专形报文（「the statement's if arms are %s」/「the unbound match-arm bodies are %s」锚 **if/match 关键字**），仅在真正被丢处触发（值位 fn 尾不触发，值经 agree 对声明返回）；checkCond 一处三用（if/while/guard，「the if condition is %s」锚条件式首 token）；值位 if：Cond→Bool、Then vs Else 经 agree（else-if 链经 typeOf 递归）、Never 臂取 inhabited 侧、失配 E0501 锚 if；while/loop/defer 体走 controlForm；break/continue 无类型面（E0201 句法侧已钉）。
- **D8 落地**：checkPattern（字面 agree 否则 E0501、通配、绑定、元组同元否则 E0501、变体：非和 scrut → E0303「declares no variants」/未知名 → E0303「no nearest-match search」/子模式数 ≠ payload 数 → E0304，锚均**变体名 token**；限定变体头 → bndMultiModule）；或模式逐分支独立收集 binds+节点，同名异型 E0501 锚**后分支的绑定 token**（5:28 金样逐位），合并取首分支名集；守卫与臂体在 binds 层内类型化；臂一致 agree 链（`one match arm is %s, the others are %s` 锚 match 关键字）。**Maranget 有用性**：patRows/expandOr（或模式逐层展开为多行）、ctorKey（变体/元组构造子/字面量自身即 0 元构造子）、specialize/defaultRows/finiteCtors（sum→变体集、tuple→单 k 元构造子、base/unit/fn/record/newtype→无穷域，无 Bool 特例）——useful 判 E0307（**只对先前无守卫臂的行集**查询，锚臂模式 token），missing 带见证回溯判 E0305/E0306：变体见证 `the variant "Yellow" of "Light"`、无穷域见证 `%q does not enumerate its values`、积见证 `the value shape "(Hit, Hit)"`（**假穷尽反例 `(Hit(a), Miss) | (Miss, Hit(a))` 于 `(Opt, Opt)` 真机命中**）；有漏且存在守卫臂 → E0306（`the unguarded arms alone do not cover "Circle" of "Shape"`），全无守卫 → E0305；检查序：逐臂（模式 → E0307 → 守卫 → 体）→ 臂一致 → 穷尽性。
- **T2 镜像测试修正 1 处（披露）**：假穷尽反例探针源两分支绑异名（`a`/`b`）——违反 T2 自身钉死的 E0302 同名集规则、句法层即拦、永不到 checker；改为同名集 `(Hit(a), Miss) | (Miss, Hit(a))`，矩阵形状与见证不变，锚 4:12 不变。
- **边界表**：bndMatchForms 行删除（typeOf Match 真实现后零引用）；walkItems default 保留 bndCtlForms 作未类型化语句形的诚实兜底（当前全部 Stmt 形已有 case，行为上不可达，防未来节点静默）；bndCompForms 模块级解构行、bndClosurForms/bndFnValues 行按 D12 计划留 T7。
- **conformance 188/201 绿**：本步新转绿 30 枚——ch3×5（defer-top/if-stmt-unit/if-value-else-if/loop-break/while-break-continue）、ch4×5（guard-fallback/int-wildcard/or-bindings/tuple-exhaustive/value-exhaustive）、ch8×2（shadowing/unit-value）、E03xx×9（E0303×2/E0304/E0305×2/E0306/E0307×3）、E0501×2（臂一致/或模式）、E0503×3、E0605×3、build-bnd-new-forms（ch3/ch4 程序首次真正类型化通过、抵达 build 主体边界——正当转绿）；余 13 红全为 T7 材料（ch12×9、E1001–E1003、e0501-fn-signature），名单 /tmp/t6-clean-red.txt，**既有 121 枚零回归、金样零改写**。日志 /tmp/t6-conformance.log。真机抽验 E0305 三形（含 --json help 全字段）/E0306/E0307/E0503/E0605 两形/E0303/E0501 两形逐字节，绿程序（else-if 链+穷尽 match+遮蔽+while+defer）静默 exit 0。
- [x] T7 typecheck fn 值/闭包/成员 + codegen 擦除：D9 fn 名值位/经值调用/闭包两形/期望线程（E1001/E0501）/捕获账本（E1002/E1003）、D10 成员解析（E0816）、D11 codegen record/newtype 擦除；D12 边界表收敛（What 删 4+1）
  来源：proposal 目标 4 5 6 7 / design D9 D10 D11 D12
  验证：`go test ./internal/typecheck/ ./internal/codegen/` 绿；边界 What 删行后 parser/typecheck 单测表同步

完成记录（T7，2026-09-05）：
- **全绿**：`go test ./...` 8 包全过（typecheck 12 个 M5 套件 + 既有 9 套件全绿，parser 24 套件、codegen、conformance 201/201——**含既有 121 枚零改写**）；gofmt 空、build/vet 过。日志 /tmp/t7-all.log、/tmp/t7-conformance.log。
- **D9 落地**：fn 名值位——identType 的 symFn 分支产 `fnType{params, ret}`（M3 fnParams/fnRets 表复用；值less fn 的 ret = unit）；经值调用——callType 局部名命中 fnType 与非名 callee 两路收敛 fnValueCall（实参逐位 checkArgs E0501、计数不合 bndArityGap、返回 ret）；闭包两形 closureType——全注解参数自足（resolveTypeRef），裸参取期望 fnType 对应位（标注位不合 → E0501 锚**参数名 token**）、无期望或非 fnType → **E1001**（锚闭包首 token，金样逐字节）、期望 ret 不回灌；体走函数体语境（c.fnRet 栈存取）：完整形带 Ret → walkFn + tailProduces（体产 () → E0501）、值less 完整形 → walkFn（fnRet=nil，非 unit 尾 E0605 fn 形）、短形归一一元块 → walkPlain（体即值，ch12「return type comes from the body」）；闭包值 = fnType（gc 类别）。**捕获账本**：closures+closureBounds 双栈，闭包进入时记边界（locals 栈深）；lookupLocalDepth 报命中层，noteCapture/noteAssignCapture 对**每个**边界在被命中层之上的账本记录（嵌套传递捕获内建）；类别经 catOf：resource → **E1002**（锚使用点/赋值目标，金样逐字节）、value 系捕获位赋值 → **E1003**（锚赋值目标）、gc → 活引用放行（金样 gc-var-活捕获绿）；模块 let 同规（层 -1，恒在一切边界下）；非捕获位赋值维持 M3 行为（D14 #1，follow-up #7）。
- **D10 收敛**：记录/新类型两侧随 T5 已落地；基类型接收者维持 bndStdModules 边界（What 文本不变，语义收窄为基类型成员面）；E0604 注册表陈旧子句 follow-up #8 维持登记。
- **D11 验证**（声明擦除行随 T5 提前落地）：构造/更新/调用形/闭包/控制流表达式全部经 bndMainBody 既有边界兜住——真机：新形式程序（记录构造+短闭包+if）build → `we: main bodies beyond a single Ok or Err return statement are not implemented in this reference build yet` exit 70；记录+newtype 声明擦除程序 build+run 端到端——IR 中声明零行、`error: Failed: record declarations erased` 字节级、exit 1。runtime/ 零改动。
- **D12 边界表收敛**：parser What 表 15→11（bndCtl/bndMatch/bndComp/bndClosur 四行删除——T3/T4 dispatch 翻转后已成死常量）；typecheck 删 bndFnValues、bndClosurForms 两行（**bndCtlForms 保留为 walkItems default 诚实兜底**、**bndCompForms 保留为模块级解构边界**——均非 D12 删除清单内，机制披露于代码注释）；typecheck_test.go:237 的 `let g = id` bndFnValues 钉死行按 D12 移除，换绿钉 `let g: fn(Int64) -> Int64 = id`（fn 名值位成真）。
- **T2 镜像测试修正 2 处（披露）**：TestFnValues E0501 探针 9:18→10:18、TestCaptureLedger E1002 探针 4:24→5:24——T2 从金样换算行号时各滑一行（金样 12:18/7:24 的 `fn f` 包装版逐列核算属实）；报文均一次命中，仅钉位修正。
- **观察（非 M5 回归，登记 T9 follow-up 候选）**：`we run <path>` 自项目目录**外**调用时报 `fork/exec … no such file or directory`——runRun 以 `cmd.Dir = path` chdir 后按子进程 cwd 解析**相对** artifact 路径（`path/build/<name>` 再叠一层）；从项目目录内 `we run .`（conformance runner 同款调用）正常。internal/cli 本变更零触碰（git diff 空）、既有金样不受影响；属 M4 遗留路径解析问题，随 T9 登记 roadmap follow-up。
- [x] T8 全量验证与真机电池：`go test ./...` 全绿（含 conformance 123→~183、既有 121 枚零改写）；真机黑盒——控制流/match/记录/闭包程序 check 绿静默、逐码负例真机报文核对、新形式程序 build 停 M4 What 边界、记录擦除程序 build+run Ok 端到端
  来源：proposal 目标 8 / design D11 D12 D13
  验证：输出记于完成记录

完成记录（T8，2026-09-05）：
- **全量验证（缓存清空重跑）**：`go clean -testcache && go test ./... -count=1` → **8 包全过零失败**（含 conformance **201/201**、既有 121 枚零改写——T7 已以 git status 核对）；gofmt -l 空、`go build ./...`/`go vet ./...` 过。日志 /tmp/t8-all.log。真机二进制 `go build -o /tmp/we-t8 ./cmd/we` 全新构建。
- **绿静默**：综合程序 /tmp/t8probe/green.we 一枚覆盖 ch3（else-if 链、while+break+continue、loop 不入——语句族已由金样盖、defer）、ch4（或模式臂、守卫臂、穷尽 match）、ch8（record 声明/构造/更新、newtype 构造+.value、元组解构）、ch12（fn 名值位、短闭包裸参+注解参、完整闭包、经值调用、值/gc 捕获）→ `we check` **无输出 exit 0**。首版探针被真机当场抓住：defer 体尾为产 Int64 的调用 → E0605 控制流体尾（D7 规则的真机自证），改 `let _ =` 弃置后绿——非实现修正，探针修正。
- **逐码负例真机电池 37/37**（/tmp/t8probe/battery*.sh，日志 /tmp/t8-battery.log）：ch3/ch4 句法 14 枚（E0201/E0202/E0203/E0204/E0301/E0302/E0303×2/E0304/E0305×3（变体见证/无穷域/积见证 `(Hit, Hit)`）/E0306/E0307）+ ch8/成员/E05 族/E06 族/ch12 23 枚（E0601/E0602/E0603×2/E0604/E0606/E0702/E0816×2/E0501×5（或模式同名异型/臂一致/构造字段/fn 签名 `fn(String) -> Int64` 源形式/newtype 拒转化）/E0503×3（if/while/guard）/E0605×3（语句位 if、语句位 match、控制流体尾）/E1001/E1002/E1003）——**码、行：列锚位、报文全过**。电池脚本自身 2 处缺陷披露：`local rc=$?` 误捕 local 自身退出码（恒 0）改独立赋值；8 处预期锚位手数错位（E0301 锚臂组 `{`、E0302 锚 `|`、E0304/E0307 锚臂模式 token、E0602 锚 `(`、E0501-arms 锚 match 关键字、fn 签名与 E1002 在本电池源中的行号）——报文与实现锚位全部一次正确，错的全是脚本预期侧，逐一对照黄金表锚修正后 37/37。
- **新形式 build 边界**：/tmp/t8probe/bnd（记录构造+更新+短闭包+if 的 main）→ `we build` 输出 `we: main bodies beyond a single Ok or Err return statement are not implemented in this reference build yet`，**exit 70**。
- **记录擦除 build+run 端到端**：/tmp/t8probe/erase（record+newtype 声明、单 Err 返回）→ build **exit 0**，IR（build/*.ll）**零 record/newtype 声明行**（grep 唯一命中为 Err 消息字符串字面量 `record declarations erased`，非声明）；`we run .`（项目内调用）输出 `error: Failed: record declarations erased` 字节级、**exit 1**——与 T7 真机证据一致并复验。
- 过程修正披露：`cp -r` 首次落入已存在目录致 E1905 假报（we.toml 不在项目根），清理后重拷即绿——工具侧失误，非实现问题。
- [x] T9 验证阶梯 + 实现审查（welang-code-review 7 条，真二进制证据）+ 归档（status → complete，归档目录 2026-09-05-control-and-composites）+ roadmap M5 行翻 done 与 follow-up #7 登记（双语对同步）
  来源：研发流程 + design D14 披露义务
  验证：gofmt -l 空、go build/vet 过、validate --all --strict 过、docs_sync 对数不变（30）、git diff --check 干净；归档目录存在

完成记录（T9，2026-09-05）：
- **验证阶梯全绿**：`gofmt -l` 空；`go build ./...` + `go vet ./...` 过；`python3 openspec/tools/validate.py --all --strict` 过（registry clean）；`python3 openspec/tools/docs_sync.py --check` **30 对齐（对数不变）**；`git diff --check` 干净。
- **实现审查（welang-code-review 7 条）通过**，结论与逐条证据（真二进制）追加于 proposal.md「审查记录」：规范符合性（201 黄金 + 37 真机负例 + 绿静默 + Maranget 假穷尽真机命中）/ 验证诚实性（抽查 /tmp/t1-red.log **恰 78 个子测试失败**与完成记录「78/80」精确一致、t2 红 logs 实存）/ 测试先行（T1/T2 红→绿经 conformance 135→139→158→188→201 增量链）/ 诊断协议稳定（diag/cli/diagnostics.toml 零改动、--json 字段零删改、零新增码）/ 单一权威（零 spec 增量、无 ADR 义务论证、代码注释中文扫描零命中）/ 红线复核（无 refr/、diff 与影响范围**逐文件吻合** 121+78+2=201 账目闭合）/ 最小可信验证（按 pre-push-checks 实际 diff 选取，无跳过）。
- **归档**：status active → complete → archived；目录 `git mv` 语义移至 `openspec/changes/archive/2026-09-05-control-and-composites/`（目录未跟踪，plain mv 等效）——与 M0–M4 归档并列；归档后 `validate.py --all --strict` 复验过（no changes found; registry clean）。
- **roadmap 双语同步**：M5 行翻 `done 2026-09-05`（en/zh 两份）；follow-up **#7 赋值目标可变性**（D14）、**#8 E0604 注册表陈旧子句**（D10）、**#9 `we run` 项目外调用路径解析**（T7 观察，M4 遗留）登记——en/zh 逐条对照成对。
- [ ] T10 记忆更新与提交：project-overview 增 M5 条目、MEMORY.md 索引行更新；英文提交信息（无署名 trailer；refr/ 不入提交）
  来源：研发流程 + 用户红线
  验证：`git log -1` 消息为英文且无 Co-Authored-By；`git show --stat` 无 refr/ 路径
