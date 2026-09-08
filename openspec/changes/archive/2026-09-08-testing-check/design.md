# design — testing-check（M10a）

规范基线：`docs/spec/2000-testing.md`（已批 8 R / 30 S）、ch16:36（test 体/mock 体的效果修订）、ch21:95-97/124-150（runner 与协议——本变更不落其行为，仅引边界）、`docs/spec/diagnostics.toml` `[diagnostic.E1801]`–`[diagnostic.E1806]`。引用行号以 2026-09-07 工作树为准。

## D1 语法与 AST：TestDecl/MockDecl 两产生式

**AST**（ast.go）：

- `TestDecl`：`{ Desc string; DescLine, DescCol int; Body ast.Block; Line, Col int }`——顶层项（`File.Items` 一员）。Desc 表示法**双形**：普通字面量存解码后字节（`decodeStringLiteral` 语义），插值串（`${}`——ch20「exactly one string literal」下的合法字面量）无法整体解码、逐字携带原始文本（含插值源文）。M10a 检查塔对描述串零语义使用（仅携带）；两形的报告渲染归 M10b。
- `MockDecl`：`{ Target string; TargetQual string; Params []ast.Param; Ret ast.TypeRef; HasRet bool; EffectTags []string; EffectLine, EffectCol int; Body ast.Block; Line, Col int; TargetLine, TargetCol int }`——TargetQual 空 = 裸名（本模块），非空 = 合格名首段（导入别名/模块名）。Params 复用 `ast.Param`（fn 参数节点，含名+型+位）。

**产生式**（parser.go）：

- 顶层 `case "test"`（换 :413 的 bnd）：`test` → 恰一**字符串字面量** token（KindStr；非之 → E0105 `unexpected token — %q where a test block names its description string`；EOF 形同）→ 块（parseBlock 惯例：`{` 换行、语句序列、`}`）。`pub test` 不达此处——pub 分派尾 E0105 自然命中（:356-359 既有报文「pub prefixes fn, let, type, chapter 8 declaration, or interface」，ch20 场景即此码，**零新报文**）。
- test 体直接项 `case "mock"`（换 :1657 的 bnd）：语境标志 `p.inTestBody int`——parseBlock 进入 test 体时置 1、离开复位；**仅深度 1**（嵌套块/控制流体内不置）——语句分派见 `p.inTestBody == 1` 才解析 MockDecl，否则 E1802。`mock` → 目标（Ident 裸名，或 Ident `.` Ident 合格形；非之 E0105 `unexpected token — %q where a mock declaration names its target function`）→ 参数表（复用 fn 参数表解析 parseParams）→ 可选 `-> TypeRef` → 可选效果段（复用 parseEffectSegment，段首位记录）→ 块。顶层位 `mock`：顶层分派新增 `case "mock"` → E1802（锚 mock 关键字）。
- **段序对照（实现准备期钉死，T2 揭出）**：ch20:34 的 mock 产生式定死「参数表 → 可选声明返回 → 可选效果段」（`mock name(params) -> type effect tag1 tag2 ...`）；ch6:53 的 fn 声明定死「参数表 → 可选效果段 → 可选声明返回」（`fn name(params) effect tag1 tag2 ... -> type`）——两序真实相异，mock 不是 fn 声明的语序复刻。ch20 场景散文（:43/:53）把目标 fn 拼成 `-> Option<User> effect io`（ch6 下不可拼的形）——示例笔误：fn 语法权威在 ch6，无已批面冲突，不需规范裁决（与 follow-up #12/#13 的两可张力不同类，仅披露）。E1803 报文的双签名渲染以 mock 序为规范形（D5），目标侧源序规范化入渲染。
- 语句位 `test` 维持 :1661 的 E0105（test 块不是语句）；语句位 `mock` 从该 E0105 清单语义中**分离**——改报 E1802（非 test 体直接项的一切 mock）。E0105 意外 token 清单里 `mock` 移除、`test` 留存。
- lexer 零改动（两关键字已在保留闭集）。

**被拒替代**：TestDecl 复用 FnDecl（描述串非名字、无参数无段——语义字段全不匹配，假复用）；MockDecl 复用 FnDecl + Target 字段（可行但「直接项」语境与 E1802 位置面需要独立节点承载位——独立节点防静默漏过，M9a D1 先例）。

## D2 模块身份与 E1801

test-ness = **文件名后缀 `_test.we`**（ch20:5「a module-identity fact the compiler decides from the file's name」）。落位：`parser.Parse(name, src)`（:128）在入口判 `strings.HasSuffix(name, "_test.we")` → `File.IsTestModule bool`。模块路径/导入身份照旧（ch15 面零触碰）。

E1801（parse 期）：TestDecl 解析完成后、`!IsTestModule` → E1801 锚 `test` 关键字 token。报文（registry 语义，家族形）：`test block outside a test module — this file's name does not end _test.we; the suffix is the module-identity fact that makes test blocks legal` + help（registry remediation）。**parse 期而非 check 期**的论证：身份是文件名事实、判定零依赖符号表；parse 期锚点最精确（`test` token），且 E0105/E1801 同层不混期。

**被拒替代**：check 期判（File.IsTestModule 已入 AST，check 期同样可判——但 parse 期让单文件 `we check plain.we` 在 parse 阶即报，与 E0105 同窗；两期皆合法，取 parse 期为锚点精确性）。

## D3 test 体检查语境：valueless fn 体 + E1401 抑制

- **语境**：`fnCtx{name: "(test)", hasRet: false}`——裸 `return` 合法（既有 valueless 机器）、`return e` E0402（ch20:5 明文「as in any valueless function」）、环深归零（fn 体惯例）、defer 合法（ch3）。
- **E1401 抑制**（ch16:36「E1401 does not fire at calls inside one: a test is driver code」）：test 体语句树直呼位不查效果子集。实现：走查 test 体时 bodyTags 置**抑制哨兵**（`nil` 语义化——checkCallEffect 见哨兵即返），**进出边界**：入 test 体置、嵌套 if/while/match/BlockExpr/for 维持、入 **task 体复位为任务自段**（taskType 既有换装）、入 **mock 体换目标段**（D4）、入闭包体不复位（闭包体本无 E1401 面——推断集在 agreement 位查，E1402 照常）、**离开 test 体时恢复外层**（外层恒为模块层——无嵌套 test）。scope 体既有「answers no enclosing declaration」（ch16:36）零改动。
- **E1405**（顶层 let 纯净）：test 模块顶层照查（test 模块是普通模块，ch20:5）。
- **E1402**（闭包纯度 agreement）：test 体内闭包照查（修订只免 E1401）。
- **顶层要求**：单文件模式无 main 要求（既有）；test 模块在项目模式仍只能经导入图编译（`_test.we` 词干不可拼写为导入段——D10 张力），E1305 面零改动。

## D4 mock 检查全链：判序与目标解析

**判序**（每 mock 首中即停，D5–D6 依序）：

1. **位置 E1802**：parse 期已拦（D1 语境标志）——check 期不可达（AST 只可能在 test 体直接项位）。
2. **目标解析**：裸名 → 本模块 `syms`（顶层符号）；合格名 → 导入面（`mod.fn` 解析既有机器）。解析失败 → **E1304**（ch20:34 明文「E1304 when the name resolves nowhere」）；跨模块非 pub → **E1303**（同引「as anywhere」）。锚目标名 token。
3. **E1804**（目标非模块级单调 fn）：解析到符号但非 FnDecl、或 FnDecl 带 TypeParams、或为构造器（record/sum 名）、或顶层 let/接口/impl/effect 名 → E1804 锚目标名 token，报文含目标类别渲染（「a generic fn / an impl or interface method / a constructor / a top-level binding」按实形）。impl 方法名不可达为模块符号（方法非模块项）——但 record 名/泛型 fn 名可达，两形足证。**synthetic std 模块 fn（如 std.concurrent 的 channel）按字面读可为目标**——合成的模块级单调 Pub FnDecl 满足 ch20 全部文字，无特判（披露：M10a 不为其落黄金，单测一枚钉行为）。
4. **E1803**（签名逐字）——D5。
5. **E1805**（重复）——D6。
6. **体检查**：fn 体语境（fnCtx name "(mock target 名)"、hasRet 按目标返回在场）+ **bodyTags = 目标段**（ch16:36「its body is checked against exactly that set」）+ task/scope 深度照常 + E1401 于其内**照查**（D3 边界）。

## D5 E1803 逐字匹配的机械定义

**三类目逐一**（任一失配 → E1803 锚 mock 关键字——块内唯一稳定锚，目标签名渲染入报文）：

- **参数**：个数相等；逐参**名字相同**（字符串等）+ **类型 sameType**（既有 sameType 机器，含命名/泛型实参/tuple/fn 型全域）+ 顺序一致。
- **返回**：在场一致（HasRet == 目标 Ret 非空）；在场时类型 sameType。
- **效果段**：在场一致；在场时 **tag 序列逐字相同**（`effect io net` ≠ `effect net io`——ch20:34「verbatim」按序读，披露为静默处决议）。

**报文形**：`mock signature does not match its target — the %s differs: mock %s, target %s`（%s₁ = 首失配类目 parameter list / declared return / effect segment；%s₂/%s₃ = 双方签名渲染 `name(params) [-> ret] [effect tags]`——mock 自身源序；目标侧的 ch6 源序[段先于返回]规范化入此渲染序）。锚 mock 关键字 token。tHelps 补 registry remediation。

## D6 E1805 目标同一性

同一性 = **解析后的 (模块键, fn 名)**——裸名 `(本模块, name)`、合格名 `(导入模块键, name)`；别名不产生第二身份（`import a.b as c` 下 `mock c.f` 与同模块直名同身——一模块一键）。判重域 = **单 test 块**（ch20:34「One test block mocks one target at most once」）：checkTestDecl 内局部 map，块间独立（ch20:71 场景「each block mocks for itself」）。锚**第二个** mock 的 mock 关键字，报文含目标渲染名。

## D7 advanceTime 定型与 E1806 位置

- **定型**：`fnType{params: [Int64], ret: unitType{}}`——名值位（:6148 归位）与调用位（:6648 归位）同型（fn 值可绑可传——但见位置规则，值逃不出 test extent）。调用面：checkArgs E0501（`advanceTime(true)` 报 E0501）；效果集空（钟是运行时控制令牌，ch20:102 四条件之三「the clock is not a value」的精神——无段可声明）。bndTaskTime 常量（:51）随两处归位而删（M9a 后仅剩用途）。
- **位置（E1806）**：合法位 = test 体 + **其内 task 体与 scope 体**（ch20:102「task and scope bodies included, for they are the test's own extent」）。实现：checker 增 `testExtent int` 深度——test 体走查置 1、task/scope 体维持（不增不减——它们是 test 的自有延伸）、**闭包体入深复位 0**、**mock 体入深复位 0**、离开复原。`testExtent == 0` 位的名值/调用 → E1806 锚 advanceTime 名 token。
- **保守字面读披露**：闭包体/mock 体内 advanceTime → E1806——ch20 未列两形为合法位，闭包体是函数体语境（ch12）、mock 体按目标契约语境（ch20:34），皆「outside a test block's body」的字面读。非规范明文，design 静默处决议，tasks 披露复记。

## D8 std.test 装载与断言面（特型，Eq 泛型面不启）

`StdModule("std.test")` 第三枚注册（M8 std.io 先例——合成声明 + 特型调用面）。

**先决：`test` 关键字与 `std.test` 模块段的碰撞（实现准备期揭出，静默处决议）**。`test` 自 M2 起在 ch1 保留闭集（lex.go:49，KindKeyword），而 moduleSeg 只收 KindIdent（parser.go:528）——`import std.test` 今不可拼写。决议三件：

- **路径段接受关键字**：moduleSeg 收 KindKeyword 且文本过 isModuleSeg 字符集的 token（E0013 字符集检查照常）——ch15:6 明文 `std.` 后段是模块路径（已批面），保留字闭集管的是标识符/名位，路径段是另一词法类；一般规则不特判 `test`（`import fn` 由此改走 E1302 而非 E0105——未批面上的诊断重路由，规范未固定该诊断，披露）。
- **调用面 = 别名形**：`import std.test as st` + `st.assertTrue(...)`——ch15:64 定死跨模块触及「exactly the qualified form `name.item` through an import's name」，裸 `import std.test` 绑定名 `test` 是关键字、不可作限定符（合法但惰性绑定，黄金钉绿）；ch20 示例的 `std.test.assertEqual(...)` 全点形是 `name.item.item`——**非 ch15 定型的形**，示例自声明非权威，不实现。
- **follow-up #13 登记**（design D10）：关键字 `test` 与 std 段/限定符的可拼性——规范修订须定夺全点限定形或改示例为别名形。

**装载与断言面**：

- **合成声明**：`assertTrue(cond: Bool)`、`assertFalse(cond: Bool)` 两 Pub FnDecl（无体，M8 println 先例）——经普通导入调用面定型，`-> ()`，零效果段。
- **assertEqual 特型面**（importCall 位特判，channelCall 先例）：同型对 `(got, want)`——**首参定型 T**、次参 sameType（失配 E0501 锚次参，报文 `mixed types — the argument is X, the parameter is Y; no coercion is ever inserted`）；**域 = 整型族（Int64/Int32/Int16/Int8/UInt64/UInt32/UInt16/UInt8）/Bool/String**；域外（record/sum/tuple/泛型参数等）→ 边界 What `assertEqual beyond the scalar, Bool, and String domains (the Eq-generic face is the standard library's own widening)`（exit 70 诚实停）。返回 unit，零效果段。
- **成员闭集**：未知成员经 importSym → **E1304**（模块级条目无此名——`unresolved name — the module %q declares no %q`，与一切模块的未知条目同码同形；D8 初稿写 E0816 是记录成员码，修正于此）；`std.test` 装载门于模块键——用户模块同名不受扰（M9a 同款 gate）。
- **Eq 泛型面不启的论证**：ch10:68 定死**基类型头 impl 拒（E0811）**、内建 Eq 只经 derives（:266「manual impl … E0822」「composite equality is the generated .equals()」）——`assertEqual<T where T: Eq>` 接受 Int64 需要内建实例面，而 implementsFace（:7061）只走 c.impls、规范未定内建实例集；引入它即扩 bound 满足域（`fn f<T where T: Eq>` 全局行为变）——标准库拓宽变更的事，本变更不越权。「base types carry it」（:266）立于 derives 要求文本，不自动延及 bound 位。

## D9 codegen 零发射与停点

- `TestDecl` 于 `Emit` 的 item 走查（codegen.go:275 族）：新增 case → 停 `NotImplemented{What: bndTestModule}`，`bndTestModule = "test modules in code generation (the M10b run tower: test harness, mock interception, virtual clock)"`。**显式 case，never vanish**（M9a D9 先例）。
- **源序先行形**：test 模块先 `fn` 后 `test`（惯例序）——item 走查先遇非 main FnDecl → **bndOtherFns 先停**（既有行为，helper fn 发射归 M10b 多函数拓宽）；仅含 test 块（无 fn）的模块 → bndTestModule。两形皆诚实，黄金各钉一枚。
- **MockDecl 不可达论证**：mock 只在 test 体内（D1 语境），test 体整体停 bndTestModule/bndOtherFns——mock 声明永不单独入 codegen 走查。
- **advanceTime 不可达论证**：E1806 使合法位 ⊆ test extent；test extent ⊆ TestDecl；TestDecl 停点先于体内任何发射——advanceTime 调用永不达 codegen（M10b 归位）。
- **st.conc/st.test 导入擦除**：既有 std 擦除路径零改（M9a TestM9aStdConcurrentImportErases 先例续用）。

## D10 roadmap 手术与 follow-up #12

- **三拆**（双语成对，M6/M9 先例）：M10 行 → `M10a | testing-check | Chapter 20 check face: test modules and test blocks, mock declarations, advanceTime typing and position, std.test loading | 状态由本变更归档翻行`、`M10b | testing-run | Chapter 20/21 run towers: multi-function codegen widening, we test runner, virtual clock, deterministic test scheduling, test boundary, mock interception | pending`、`M10c | testing-explore | Chapter 21 exploration: interleaving probes, E1901/E1902 guards, partial-order reduction | pending`。
- **follow-up #12（新登记）**：`Test-module importability versus the module-path charset.` Chapter 21's scenario calls `src/helpers_test.we` "compiled with the project, importable" — but the import→file mapping (chapter 15) spells module paths from `[a-z][a-z0-9]`-shaped segments (E0013 as implemented), and an underscore stem cannot be spelled; the project compilation walks the import graph from src/main.we, so a test module outside it never compiles under `we check .`. Either the charset admits the underscore stem or the scenario's importability needs its own mapping — a spec amendment should fix which. Found by M10a (`testing-check` design D10; Q1 split's loader-boundary disclosure).
- **follow-up #13（新登记）**：`The test keyword versus the std.test module segment.` Chapter 1's reserved-word closure makes `test` a keyword (it heads the test-block production), so the segment cannot lex as an identifier; chapter 15's qualified reach is exactly `name.item` through an import's name, and the chapter 20 examples call `std.test.assertEqual(...)` — a two-level dotted form chapter 15 does not fix. M10a makes the ratified surface spellable (module-path segments accept keyword tokens passing the charset check) and serves calls through the alias form (`import std.test as st; st.assertEqual(...)`); the bare import binds the keyword name and is inert. A spec amendment should fix which call surface is real: the two-level std qualifier, alias-only examples, or a non-keyword module name. Found by M10a (`testing-check` design D8, implementation-prep disclosure).

## D11 黄金矩阵与单测面

**conformance 黄金（预计 ~35，files 映射控制文件名，零 runner 改动；调用面一律别名形 `import std.test as st`——D8 决议）**：

- 负例族（E 码）：E1801×1（`src/plain.we` 单文件含 test 块）；E1802×3（顶层 / 普通 fn 体内 / test 体内嵌套块）；E1803×5（参名 / 参型 / 返回在场 / 返回型 / 段在场或异序）；E1804×3（泛型 fn / 构造器 / 顶层 let 名）；E1805×1；E1806×3（test 模块普通 fn / 非 test 模块 main / test 体内闭包体[保守读钉]）+ mock 体内×1；E0105 pub-test×1；E0402 test 体携值 return×1；E1304 mock 目标无解析×1；E1401 mock 体内照查×1（目标段 io、体调 net 段 fn）—— **~21 负例**。
- 绿面族：test 模块单文件全绿（helper fn + 双 test 块 + assert + std.test 三面）；own-module mock 绿（含段逐字重述）；advanceTime 体内绿 / task+scope 体内绿；E1401 抑制绿（体内调 io 段 fn 无段不报）；mock 体对目标段 E1401 绿（目标 effect io、体调 io fn）；assertEqual 三型绿（Int64/Bool/String）+ 异型 E0501×1；裸 `import std.test` 惰性绑定装载绿×1 —— **~12 绿/码例**。
- 边界族：build-bnd-test-module（仅 test 块模块 → bndTestModule exit 70）×1；build-bnd-test-fns-first（fn 先行 → bndOtherFns）×1；assertEqual 域外（record 对）→ 边界 What×1 —— **~3**。
- **跨模块 mock 目标（mod.fn 形）走单测不走黄金**（D10 张力：项目装载面归 M10b）——typecheck 多模块单测（wantDiagProject 先例）覆盖：pub 目标绿、非 pub E1303、别名同身 E1805。
- **既有黄金触碰预期**：`advancetime-bnd` 锁定绿（M9a Q3 钉）→ 翻绿面真行为（advanceTime 体内合法）——随批披露；其余 507 枚零触碰预期。

**单测**：`internal/parser/m10a_test.go`（TestTestBlockProduction/TestMockProduction/TestTestModuleIdentity——AST 字段、E0105 家族、E1802 三位、E1801 锚）；`internal/typecheck/m10a_test.go`（TestTestBodyContext/TestMockChecking/TestAdvanceTimePosition/TestStdTestLoading/TestMockCrossModule——E0402/E1401 抑制两形/E1402 维持/判序 E1304→E1804→E1803→E1805/E1806 四位/E1303 跨模块/assertEqual 域与 E0501/synthetic 目标单测；跨模块三用例经 CheckProject 三模块辅助——util 先入、test 模块随行、极小 root main 满足项目约）。

## D12 验证阶梯

三阶（M9a 惯例）：①黄金/单测先红（红因分类记档：parse 边界 exit 70 / E1302 std 形 / 既有 E0105——与 M9a T1 红因分类同型）；②实现翻绿逐任务对账（conformance 总数 508 → 508+N 对账、双向 comm 校验）；③真机黑盒电池（真二进制：`we check tests/x_test.we` 绿、E180x stderr 人类面 + `--json` 协议字段实测、`we build` 停点逐形、`we check .` 项目面零回归）。收尾全量：`go clean -testcache && go test ./...`、gofmt/vet、`validate --strict`、docs_sync 对数不变（纯实现零规范对）。

## D13 披露义务

- design 静默处决议（本 D 编已列：E1803 tag 序逐字、闭包/mock 体内 E1806 保守读、synthetic std 目标可 mock、advanceTime 零效果段）——tasks 完成记录复记。
- 自有新增黄金实现前修正：自由（T1 落盘红跑揭出即修，记档）。
- 既有黄金随批更新：`advancetime-bnd` 翻绿披露；其余零改写预期——任何意外触碰即停查因。
- 实现若揭既有面缺陷：修复披露，不得静默绕过（M9a finiteCtors 先例）。
