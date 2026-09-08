# tasks — testing-check（M10a）

- [x] T1 测试先行 A：D11 黄金矩阵落盘（~30 新增，files 映射控制 `_test.we` 命名），对当前构建运行必须先红
  来源：proposal 目标 1–7 / design D11 D12 D13
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红（新增失败清单，既有 508 枚零回归），红因分类（parse 边界 exit 70 / E1302 std 形 / 既有 E0105 / 锁定绿）记于完成记录；`advancetime-bnd` 锁定绿维持为证（M9a Q3 回归钉，T4 归位时随批翻绿并披露）

  **完成记录（2026-09-08）**：35 案例落盘（生成器 `/tmp/m10a-goldens/gen.py`，/tmp 惯例；锚点 marker 子串程序化定位）。conformance 对账：总数 543（508 既有 + 35 新），**红 34 = 新枚减 1 锁定绿**（双向 `comm` 对账：红 ∖ 新 = 空、新 ∖ 红 = 恰一枚），既有 508 枚零回归。分布：负例 22（E1801×1、E1802×3[顶层/普通 fn 体/嵌套块]、E1803×6[参名/参型/返回在场/返回型/段在场/段序]、E1804×3[泛型 fn/构造器/顶层 let]、E1805×1、E1806×4[test 模块普通 fn/非 test 模块 main/闭包体/mock 体——后两形保守读钉]、E0105 pub-test×1、E0402 test 体携值 return×1、E1304 mock 目标无解析×1、E1401 mock 体内照查×1[目标段 io、体调 net]）；绿面 10（test 模块全绿[helper+双 test 块+assert+std.test 三面]/own-module mock 绿/advanceTime 体内绿/task+scope 体内绿/E1401 抑制绿/mock 体对目标段 E1401 绿/assertEqual Bool·String 绿/裸 `import std.test` 惰性装载绿）；边界 3（build-bnd-test-module[纯 test 块 → bndTestModule]/build-bnd-test-fns-first[fn 先行 → bndOtherFns]/assertEqual 域外[record 对 → 边界 What]）。
  **红因分类**：parse 边界 exit 70 bndTest（26 枚——一切 parse 先触 `test`/`mock` 分派的文件）；E0105 import 形（6 枚——`import std.test` 的 `test` 段今不可拼，D8 碰撞的直接红证）；bndTaskTime exit 70（2 枚——e1806-plain-fn/e1806-main，无 test/mock token 直达类型期停点）。**锁定绿 1 枚**：check-e0105-pub-test——pub 尾分派今日已逐字命中（真机探针复核），「零新报文」设计主张的现实验证。
  **复用码报文探针**：E0501（`mixed types — the argument is Bool, the parameter is Int64; …`，锚实参位）、E1401（`undeclared effect at a call — …`，锚调用名）、E1304（bareUnresolved 原文）、E0105 语句位/pub 位/import 位三形——全部现行二进制捕获，黄金零手拼。
  **design 修正（D8/D10/D11，实现准备期揭出，已在 design 内披露）**：`test` 关键字与 `std.test` 段碰撞（moduleSeg 今拒 KindKeyword → `import std.test` 不可拼）——决议路径段接受过字符集的关键字 token、调用面别名形、**follow-up #13 登记**；D8 成员闭集码修正 E0816 → E1304（模块级条目形）；D11 矩阵 ~30 → ~35（段序 E1803 独立成枚、E1401-mock-body 负例、裸装载绿加入）。
  **0 改写**：`git status` 既有 testdata 零触碰（advancetime-bnd 维持锁定绿，T4 随批翻）。

  **修正（2026-09-08，T2 揭出）**：T1 黄金的目标 fn 源码误用 mock 段序（`-> Ret effect tags`）——ch6:53 定死 fn 声明段序在参数表与箭头之间（`effect tags -> Ret`），ch20:34 只为 mock 产生式定死返回先于段，两序真实相异（design D1 段序对照）。11 枚黄金源修正（e1803×6、e1805、e1806-mock-body、e1401-mock-body、mock-own-module-green、mock-body-segment-green——自有新增黄金实现前修正，D13 自由），报文零改（E1803 渲染本就以 mock 序为规范形）。**红因分类修正**（真二进制逐枚复核）：bndTest exit 70 = **25**（非 26）、E0105 顶层 mock = **1**（e1802-toplevel——顶层 `mock` 今日 E0105「fits no top-level item production」，非 bndTest）、E0105 import 形 6、bndTaskTime 2、锁定绿 1——原记录把 e1803 族（实为 fn 序 E0105 红）与顶层 mock 误归 bndTest 桶。总数对账不变：红 34 + 锁定绿 1 = 35 新枚，既有 508 零回归。

- [x] T2 测试先行 B：parser 单测（TestDecl/MockDecl 产生式、E1801 锚、E1802 三位、E0105 家族）与 typecheck 单测（test 体语境与 E1401 抑制、mock 检查全链与判序、advanceTime 定型与 E1806 四位、std.test 装载与 assertEqual 域、synthetic std 目标单测）先写先红
  来源：proposal 目标 1–6 / design D1 D3 D4 D5 D6 D7 D8 D11
  验证：`go test ./internal/parser/ ./internal/typecheck/` 红（新用例引用未定义符号/停点断言不匹配），red 证据记于完成记录；跨模块 mock 目标三用例（pub 绿 / 非 pub E1303 / 别名同身 E1805）走多模块单测不走黄金（design D11 理由）

  **完成记录（2026-09-08）**：`internal/parser/m10a_test.go` 三函数（TestTestBlockProduction/TestMockProduction/TestTestModuleIdentity）+ `internal/typecheck/m10a_test.go` 六函数（TestTestBodyContext/TestMockChecking/TestAdvanceTimePosition/TestStdTestLoading/TestMockCrossModule）。两包本地 As 族辅助（parseAs/wantCleanAs/wantDiagAs 与 runCheckAs/wantOKAs/wantDiagAs/wantBndAs——共享 helper 钉死 "test.we" 文件名，而模块身份是文件名事实，design D2）；跨模块走 CheckProject 三模块辅助（util 先入桶、test 模块 `src/t_test.we` 随行、极小 root main——装载图不可达 `_test.we` 词干，单测直入 checker 机器，design D11）。
  **覆盖面**：parser——TestDecl/MockDecl 全字段与位（Desc 双形：普通解码/插值逐字 `${n}`）、E0105 家族四形（非串描述/pub 尾/语句位 test/非名 mock 目标——后两形报文文本由本测试定约）、E1802 三位、E1801 锚、IsTestModule 七名表；typecheck——E0402 `(test)` 语境、E1401 抑制（直呼/穿透 if+while）与三处照查（task 体自段/mock 体目标段/普通 fn）、E1405/E1402 维持、判序四证（E1304→E1804→E1803→E1805 首中即停）、E1803 六类目（报文含双签名渲染逐字）、E1804 三类、E1805 第二 mock 锚、advanceTime 定型（名值绿/调用 E0501）与 E1806 四位 + 名值位、std.test 三面（assertTrue E0501/assertEqual 三域绿 + 异型 E0501 + 域外 bnd What/成员闭集 E1304）、裸导入惰性、E1302 兄弟模块、synthetic std 目标（conc.channel 逐字重述绿）、跨模块三用例（pub 绿/E1303 非 pub/别名同身 E1805）。
  **red 证据**：parser 包构建失败——`undefined: ast.TestDecl`（m10a_test.go:64/83/103/137/143 五位）+ `f.IsTestModule undefined`（:180/:181），M9a T2 先例形（MockDecl 在类型开关内随 TestDecl 报）；typecheck 包恰 5 新函数红——TestTestBodyContext/TestMockChecking/TestAdvanceTimePosition/TestMockCrossModule 各 `parse boundary: chapter 20 (testing) forms`（bndTest）、TestStdTestLoading `E0105 "test" in an import`（D8 碰撞直证），既有测试全绿零回归。
  **T2 揭出的规范事实（design D1 修正披露 + T1 黄金修正）**：fn/mock 段序相异（ch6 fn = 段先于返回；ch20 mock = 返回先于段；ch20 场景散文的 fn 拼法是示例笔误，ch6 为 fn 语法权威）——11 枚黄金源修正（T1 修正段详列）、单测目标 fn 全部 ch6 序、E1803 渲染以 mock 序为规范形。复用码报文探针新增四枚：E1405（`effectful call in a top-level initializer — …`）、E1401 task 形（`…which the task block does not declare…`）、E1402（`function value effect set does not match…`，锚绑定名）、E0105 语句位 test 全文。

- [x] T3 parser 面：D1 两产生式与 AST 节点（TestDecl/MockDecl + File.IsTestModule）、D2 模块身份与 E1801（parse 期）、E1802 三位（顶层 mock / 普通 fn 体内 / test 体内嵌套块——语境标志仅深度 1）、语句位 `test` 维持 E0105 与语句位 `mock` 换报、bndTest 两处 bnd 位与常量删、既有 parser 测试更新面披露
  来源：proposal 目标 1 2 / design D1 D2 D9
  验证：`go test ./internal/parser/` 绿；既有套件更新面逐处披露；conformance parse 期黄金翻绿对账

  **完成记录（2026-09-08）**：`internal/ast/ast.go` 三处——File 增 `IsTestModule`（文件名事实，parse 期定）、TestDecl/MockDecl 两节点（含 ch6/ch20 段序对照注释、MockDecl「直接项无他位」注释）、`item()`/`stmt()` 标记入分组块、包注释章列表加 20。`internal/parser/parser.go`——包注释（spec 文件列表加 2000-testing.md、prose 加 test 块与 mock 声明）；bndTest 常量与两 bnd 位删（顶层 `case "test"` → parseTestDecl；语句 `case "mock"` → 深度判）；struct 增 `testModule bool` + `inTestBody int`（1→2 握手：parseTestDecl 置 1，block() 入口 1→2 标记 test 体自身块、其余→0，三个 return 位恢复——镜像 p.direct 手法，仅深度 2 解析 MockDecl）；Parse 入口 `strings.HasSuffix(name, "_test.we")` 定身份、parseFile 落 f.IsTestModule；顶层 `case "mock"` → E1802（原走 E0105「fits no top-level item production」默认位——T1 红因分类的 1 枚）；E0402 由 parse 期 fnCtx 自然携带（test 体 `(test)` 语境压栈，携值 return 即报——e0402 黄金由此 parse 期翻绿）；helps 增 E1801/E1802（registry remediation 逐字）；parseTestDecl（串门 E0105 家族/块门/E1801 于块解析后锚 test 关键字）/parseMockDecl（目标 Ident 或 qual.name、参数表复用 parseParamList、ch20 序 `-> Ret` 先于 effect 段、parseFnBlock 语境名 = 目标显示名、名字空间零注册——mock 不绑模块名）/descText（双形：普通解码/插值逐字，含 `${` 返原文；`parseUEscape`/`hexDigit` 本地镜像 codegen 未导出对——转义表权威在 lexer 校验，两读者按构造一致）。
  **D8 落地**：moduleSeg 收 KindKeyword 且文本过 isModuleSeg——探针三形：`import fn` E0105→E1302（design 披露的重路由）、`import std.test` 今可拼（E1302 待 T6 注册）、`import collectAll` E0013 字符集照拦。既有面零触碰：无黄金/单测钉关键字段位（检索证）。
  **既有 parser 测试更新面（三处披露）**：dispatchTable `test` 行 top 位 bndTest→E1801（runParse 钉 "test.we" 名，非 _test 后缀）、`mock` 行 top/stmt 位→E1802（expr 位 E0105 不变）；TestBoundaryForms 删 test/mock 两 probes（真行为归 m10a_test.go 钉）+ 注释更新 M9a→M9a+M10a；testForms 常量删。
  **T2 测试修正（D13 披露）**：m10a_test.go EffectCol 36→43——T2 手数错列（36 是 effect 关键字；家族锚点是段首 tag，parseEffectSegment 复用即 43，FnDecl/MethodSig/TaskExpr/FnType 同锚）。测试缺陷非规范分歧。
  **conformance 对账**：红 34 → **24**，翻绿 10 = 负例 5（E1801×1、E1802×3、E0402×1）+ 绿面 5（mock-own-module/mock-body-segment/advancetime-test-body/advancetime-task-scope/e1401-suppressed——TestDecl 于 checkModule 走查暂静默跳过，T4/T5 落检查前的测试先红中间态）；锁定绿 check-e0105-pub-test 维持；既有 508 零回归（红名单 24 枚全在新增 35 集内）。余红 24 归位：check 期 exit-0 类 16（E1803×6/E1804×3/E1805×1/E1304/E1401/E1806 闭包+mock 体）+ bndTaskTime 2（E1806-main/plain-fn，T4）+ std.test E1302 类 6（T6）+ build 2（T7）。
  **验证**：`go test ./internal/parser/` 全绿（M10a 三函数 + 既有全量）；typecheck 仅 T2 五函数红（T4/T5/T6 先红维持）；codegen 绿；`go vet ./...`/`gofmt -l` 清。

- [x] T4 typecheck 面 A：D3 test 体语境（fnCtx valueless、E0402 复用、E1405 维持）与 E1401 抑制哨兵（进出边界：穿透 if/while/match 臂、task 体复位自段、mock 体换目标段、闭包体不复位）；D7 advanceTime 归位（(Int64)->() 定型、名值/调用位同判、E1806 位置规则 testExtent 深度、闭包/mock 体保守读）、bndTaskTime 常量删（M9a 后仅剩用途）
  来源：proposal 目标 3 5 / design D3 D7
  验证：`go test ./internal/typecheck/` test 体与 advanceTime 套件绿；`advancetime-bnd` 既有黄金随批翻绿披露（其余 507 枚零触碰对账）；typecheck_test.go Q3 回归钉行更新披露

  **完成记录（2026-09-08）**：checker struct 增两字段——`bodyTest`（D3 抑制哨兵；design 草案的 nil 语义化落为 bool：nil 与纯函数体空集歧义，bodyTask 同款握手更真）与 `testExtent int`（D7 钟深）。checkCallEffect 于 inClosure 分支后增 bodyTest 早退（闭包先读——其推断集与 E1402 agreement 照常）；taskType 换装增 bodyTest=false（task 体复位自段，E1401 于其内照查——task 报文形既有）；closure walk 增 testExtent 复位 0（D7 保守字面读：闭包体是函数体语境非 test 自有延伸）；scope 体零触碰（继承语境——ch16:36「answers no enclosing declaration」既有）。checkTestDecl：bodyTags/nil、"(test)"、inBody、bodyTest=true、testExtent=1、fnRet nil、prop "(test)"、locals 栈、walkItems(walkFn)（valueless 全闭包缺省形模板）、resCheck(items, nil, nil)（ch13 释放纪律对 test 体内声明绑定照查——设计未列，语句纪律一致性落码，披露）；mock 项今触 walkItems 缺省 bndCtlForms 诚实停（T5 机器翻）。checkModule 体走查 pass 增 `case *ast.TestDecl`。advanceTime 双位归位：名值位（fnType{Int64}->() + E1806 门）与调用位（fnValueCall 同型——E0501 于实参位照查、产 unit、零效果段）；bndTaskTime 常量删；tHelps 增 E1806（registry remediation 逐字）；`advanceTimeOutside` 报文常量（E1806 家族文，两同文）。
  **既有面更新披露（两处，皆 T4 预告或实现期揭出）**：typecheck_test.go :313 M9a Q3 钉行随特征翻面（wantBnd bndTaskTime → wantDiag E1806 2:5）；check-conc-advancetime-bnd 既有黄金随批更新（exit 70 边界钉 → exit 1 E1806 4:13 锚名 token）——`git status` 对账恰此 1 枚 M、其余 507 枚零触碰。m10a helper 修正（D13 披露）：runCheckAs 的 parse 诊断从 fatal 改浮出为流水线诊断——E0402 是 parse 期 "(test)" 语境面（T3 落），T2 helper 假设 check 期系写法缺陷。
  **conformance 对账**：红 24 → **23**——E1806 三枚翻绿（closure-in-test/main/plain-fn）；mock 两绿例（own-module/body-segment）从 T3 绿转 bndCtlForms 诚实停红：T3 时 TestDecl 整体被跳过故绿，今 test 体走查激活、mock 项候 T5——同因 e1806-mock-body 维持红（停点形）。既有 508 零回归（唯一触碰即上列 advancetime-bnd）。
  **验证**：TestTestBodyContext 全绿（E0402/抑制穿透 if+while/task 复位 8:13/E1405 5:9/E1402 6:9/普通 fn E1401 6:5）；TestAdvanceTimePosition 唯余 mock 体一子例（候 T5）；其余既有 typecheck 套件全绿；`gofmt -l`/`go vet` 清；全仓仅 conformance 23 红 + typecheck 四函数红（T5/T6/T7 面）。

- [x] T5 typecheck 面 B：D4 mock 检查全链（目标解析走既有 syms/导入面、E1304/E1303 复用、判序 E1304→E1303→E1804→E1803→E1805→体）；D5 E1803 机械定义（参数名+型+序/返回在场+型/段在场+tag 序、报文形）；D6 E1805 同一性（解析后 (模块,名)、逐块判重、别名同身）；mock 体按目标段检查（fnCtx + E1401 于其内照查）
  来源：proposal 目标 4 / design D4 D5 D6
  验证：`go test ./internal/typecheck/` mock 套件绿；跨模块三用例绿；conformance mock 黄金翻绿对账

  **完成记录（2026-09-08）**：checker struct 增 `mockSeen map[string]bool`（D6 活块身份集——checkTestDecl 每 walk 前换新图、walk 后恢复，块间独立 ch20:71）。walkItems 语句 switch 增 `case *ast.MockDecl`（Assign 后——parser E1802 已锁其余位，链自身无需位置门）。`checkMockDecl` 新函数（checkTestDecl 后）——判序首中即停后仍继续走面（诊断器单报文契约：每判一层 c.fail 后继续，唯一诊断由流水线取首）：①目标解析（裸名 `c.syms[target]` 无则 E1304 bareUnresolved 锚名 token；合格名 `c.importQualifier` 无则 E1304 同形 → `c.importSym`（E1304 模块形/E1303 非 pub，既有报文逐字复用，锚名 token））；②`mockableTarget`（E1804——symFn+TypeParams→"a generic fn"、symVariant/symType/symRecord/symNewtype→"a constructor"、symLet→"a top-level binding"、symIface→"an interface"、symEffect→"an effect declaration"、symImport→"an import"；报文 `mock target is not a mockable function — %s is %s; only a module-level monomorphic fn is mockable` 锚名 token）；③E1803 三类目（参数 len/名串等/sameType 逐位 → "the parameter list differs"；返回在场 `fn.Ret != nil`+sameType → "the declared return differs"；段 len+canonical 键序等 → "the effect segment differs"（序非集：`effect net io` ≠ `effect io net`，D5 披露读）——双签名渲染 `name(params) [-> ret] [effect tags]`（param `name: Type` 逗号连接、displayTag 空格连接、mock 序=ch20 产生式序），锚 mock 关键字）；④E1805（identity = 模块键+名——裸名 c.modKey、合格名 importQualifier 的 canonical 键，别名同身；报文用裸 fn 名，锚第二 mock 关键字）；⑤体走查（bodyTags=目标段、bodyName=fn.Name、inBody、bodyTest=false、testExtent=0（E1806 保守读）、fnRet/prop 按目标、locals 压 fn.Params×fnParams[fn]（逐字重述使两拼法一等）、valued 且非 tailProduces → E0501、resCheck(items, fn.Params, targetParams)、recvMut 保存恢复）。tHelps 增 E1803/E1804/E1805（registry remediation 逐字）。
  **黄金修正（D13 披露，自有新增黄金实现前修正）**：e1804 三枚（generic-fn/constructor/top-let）生成期锚位手滑钉 mock 关键字（5:5/3:5/3:5）——design D4-3 明文「E1804 锚目标名 token」，按 design 正为 5:10/3:10/3:10（报文零改）。**T2 单测修正（D13 披露）**：TestMockCrossModule E1303 钉行手数错 3:15——源码 `mock util.hidden()` 在第 4 行，正为 4:15（列与报文本对，EffectCol 同类缺陷）。
  **类目渲染披露**：E1804 七类目中 interface/effect declaration/import 三类超三黄金钉面（design 列「a generic fn / an impl or interface method / a constructor / a top-level binding」按实形）——impl 方法名不可达模块符号（方法非模块项），代以 interface/effect/import 三可达类按实形自由渲染，design 静默处决议记档。
  **conformance 对账**：红 23 → **8**——翻绿 15 = E1803×6、E1804×3（黄金锚位正后）、E1805×1、E1304-mock、E1401-mock-body、E1806-mock-body、mock-own-module-green、mock-body-segment-green；余红 8 全归 T6（std.test 族 6：test-module-green/assertequal×4/bare-import——E1302 `no standard-library module "std.test"`）+ T7（build×2）。既有 508 零回归（红 8 全在新增 35 集内；`git status` 既有黄金触碰面仍唯 advancetime-bnd 一枚=T4 披露）。
  **验证**：`go test ./internal/typecheck/ -run 'TestMockChecking|TestAdvanceTimePosition|TestMockCrossModule'` 全绿（mock 体 E1806 随体走查翻绿）；全仓 `go test ./...` 唯 TestStdTestLoading（T6）+ conformance 8 红（T6/T7）——预期中间态；`go vet`/`gofmt -l` 清。

- [x] T6 std.test 装载：D8 StdModule("std.test") 第三枚注册（assertTrue/assertFalse 合成声明经普通导入面、assertEqual 特型面[首参定型 T、次参 sameType E0501、域 = 整型族/Bool/String、域外边界 What]、成员闭集 E0816、装载门于模块键）、Eq 泛型面不启论证落码注释
  来源：proposal 目标 6 / design D8
  验证：`go test ./internal/typecheck/` 装载套件绿；conformance assert 族黄金翻绿对账；用户模块同名 std.test 不受扰单测

  **完成记录（2026-09-08）**：`StdModule` 增 `case "std.test"`——assertTrue/assertFalse 两 Pub FnDecl（param `cond: Bool`、无体无段，std.io println 先例；1:1 位随 stdConcurrentFile 惯例）；**assertEqual 非条目而是调用面**——importCall 于 pub 门前置分派（channelCall 先例）：`mod == "std.test" && name == "assertEqual"` → `assertEqualCall`，**装载门 = 模块键本身**（importQualifier 的 canonical 键；用户别名/自有同名皆不达）。`assertEqualCall`：arity≠2 → bndArityGap（族一致）；首参 `typeOf(arg0, nil)` 定型 T；域门 `baseType ∈ assertEqScalar`（Int64/Int32/Int16/Int8/UInt64/UInt32/UInt16/UInt8/Bool/String）否则 **bndAssertEqDomain**（新边界词，design D8 What 逐字）；次参 `typeOf(arg1, got)`（期望型穿线）+ sameType 失配 → E0501 锚次参（`mixed types — the argument is X, the parameter is Y; no coercion…` 既有报文形）；返 unit 零效果段。**Eq 泛型面不启论证落码注释**（assertEqualCall 注释：ch10 基类型头 impl 拒 E0811/内建 Eq 只经 derives/`fn f<T where T: Eq>` 全局拓宽——标准库自家变更）。成员闭集：assertLength 等经 importSym → **E1304**（`unresolved name — the module "std.test" declares no %q`，锚限定符——D8 修正码，非 E0816）；mock 于 assertEqual → E1304（面非模块级 fn，诚实读，注释记）。裸 `import std.test`：绑定名 `test` 是关键字、限定符位不可拼（E0105）——合法惰性，零新码；装载门 riding 既有 StdModule 注册（checkImport E1302 与项目装载器同源，零改）。**同名不受扰单测**：条目级探针（自有 `fn assertTrue(n: Int64)` 与 `st.assertTrue(true)` 同 test 体共存绿——门是模块键非 fn 名）；模块级同名结构性不可达（std 段永不触文件系统，E1302 面已钉）。
  **conformance 对账**：红 8 → **2**——std.test 族 6 枚翻绿（test-module-green/assertequal-bool/string-green/mixed-e0501/domain-boundary/bare-import）；余红恰 build×2（T7 面）。既有 508 零回归（`git status` 黄金触碰面不变）。
  **验证**：`go test ./internal/typecheck/` 全绿（含 TestStdTestLoading 全部探针）；`go vet`/`gofmt -l` 清。

- [x] T7 codegen 停点：D9 TestDecl 显式 case 停 bndTestModule（新边界词）、两形钉死（纯 test 块文件 vs helper fn 先停 bndOtherFns）、MockDecl/advanceTime 不可达论证落码注释、std.test 导入擦除经既有 std 路径、停点单测逐形钉（What 逐字断言）
  来源：proposal 目标 7 / design D9
  验证：`go test ./internal/codegen/` 停点套件绿；build 边界黄金两枚翻绿对账

  **完成记录（2026-09-08）**：`internal/codegen/codegen.go`——item 走查（今无 default，TestDecl 会**静默消失**——正是 D9「显式 case，never vanish」所防）增 `case *ast.TestDecl` → 停 `NotImplemented{What: bndTestModule}`（新边界词 = design D9 What 逐字 `test modules in code generation (the M10b run tower: test harness, mock interception, virtual clock)`）；**不可达论证落码注释**（case 内）：MockDecl 只在 test 体自有深度可解析（parser E1802 锁其余位）、advanceTime 合法位 ⊆ test extent（E1806 锁域外）——两者严格居于本停点所停的 TestDecl 之内，走查永不至，故皆无需 walk case。**CLI 单文件路由（D9 两形钉死的实现面）**：`internal/cli/build.go` runBuild 单文件位——`file.IsTestModule` 时越过 `whatSingleFileBuild` 命名边界直达 `codegen.Emit`（test 模块的 build 故事属测试塔：单文件 test 模块在 M10b runner 前无 artifact 面，codegen item 走查是诚实停点）；Emit 返 IR 的极端形（test 模块含 main 且无 test 块）回落命名边界——仍是单文件 build，两停皆诚实。**std.test 导入擦除**：既有 `d.Path[0] == "std"` 路径零改收第三枚（M9a TestM9aStdConcurrentImportErases 先例续用——concAlias 特判不触及 std.test，擦除面在 std 段总门）。停点单测 `internal/codegen/m10a_test.go` TestM10aTestModuleBoundary（两形 What 逐字断言：纯 test 块含 std.test 导入 → bndTestModule；helper fn 先行 → bndOtherFns——源序先决，体未读即停）。
  **conformance 对账**：红 2 → **0**——build-bnd-test-module/b build-bnd-test-fns-first 翻绿（exit 70 + CLI 面 `we: %s are not implemented…` 逐字）。**总数 543（508 既有 + 35 新）全绿**；既有 508 零回归（`git status` 黄金触碰面仍唯 advancetime-bnd 一枚=T4 披露）。
  **验证**：`go test ./internal/codegen/`/`./internal/cli/` 全绿；conformance 全绿；`go vet`/`gofmt -l` 清。

- [x] T8 黑盒电池与 --json 实测：真二进制逐面（`we check tests/x_test.we` 绿/E180x stderr 人类面、`we build` 停点两形、`we check .` 项目面零回归、E1801 单文件 `we check plain.we`、advanceTime 域外位）+ E 码 JSON 事件字段实测（diagnostics 协议携带新码零协议改动为证）
  来源：proposal 目标 7 / design D12 阶③
  验证：电池逐探结果记于完成记录（exit 码与 stderr/stdout 逐字）

  **完成记录（2026-09-08）**：真二进制 `/tmp/m10a-we`（repo 根 `go build ./cmd/we`）十二面逐探，exit/stderr 逐字：
  ① `we check tests/green_test.we`（helper fn + 双 test 块 + std.test 三面 + advanceTime）→ exit 0 静默；
  ② `we check plain.we`（test 块在非 test 模块）→ exit 1，`plain.we:1:1: error[E1801]: test block outside a test module — this file's name does not end _test.we; …`；
  ③ E1803 人类面 → exit 1，`…6:5: error[E1803]: mock signature does not match its target — the parameter list differs: mock read(k: String) -> String effect io, target read(n: String) -> String effect io`；
  ④ `we build tests/only_test.we`（纯 test 块）→ exit 70，`we: test modules in code generation (the M10b run tower: test harness, mock interception, virtual clock) are not implemented…`；
  ⑤ `we build tests/math_test.we`（helper fn 先行）→ exit 70，`we: functions other than main in code generation are not implemented…`；
  ⑥ E1806 单文件（plain fn 内 advanceTime）→ exit 1 `…2:5: error[E1806]: advanceTime called outside a test block…`；项目面 main 内 → exit 1 `4:5` 同文；
  ⑦ `--json`（E1806）→ exit 1，单事件对象逐字段：type/severity/code/message/file/line/column/help（remediation 逐字 `Advance the clock inside the test that observes it; …`）——**diagnostics 协议零改动携带新码为证**；
  ⑧ `we check .`（干净项目）→ exit 0；**项目面带随行 test 模块**（`src/green_test.we`）→ exit 0——装载图不可达 `_test.we` 词干（follow-up #12 面实测：不编译、不扰项目检查）；
  ⑨ E1802 顶层 mock → exit 1 `…1:1: error[E1802]: mock declaration outside a test block…`；
  ⑩ assertEqual 域外（record 对）→ exit 70 `we: assertEqual beyond the scalar, Bool, and String domains … not implemented…`；
  ⑪ E1805 → exit 1 `…9:5: error[E1805]: duplicate mock of one target in a test block — read is mocked twice in this block…`；
  ⑫ 裸 `import std.test` 装载绿 → exit 0。
  **结论**：CLI 面（人类报文/exit 码/JSON 协议）与检查塔全链一致，零协议改动。

- [x] T9 全量终验：`go clean -testcache && go test ./...` 全绿、gofmt/vet 清、`validate --strict` 过、docs_sync 对数不变（纯实现零规范对）、T1/T2 红名单逐枚翻绿零回归、conformance 总数对账（508 + N）
  来源：proposal 目标 7 / design D12
  验证：命令输出记于完成记录

  **完成记录（2026-09-08）**：`go clean -testcache && go test ./...` 九包全 ok（cli 1.465s / codegen 0.012s / conformance 20.488s / diag / lex / parser 0.039s / typecheck 0.279s / version / runtime 4.332s）；`gofmt -l .` 空；`go vet ./...` 清；`python3 openspec/tools/validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`；`docs_sync` → `OK: 31 document pair(s) aligned`（对数不变——纯实现零规范对，E1801–E1806 与 helps 皆 T0 前在册）；`git diff --check` 干净。**红名单对账链**：T1 落盘红 34 → T3 parse 面 24 → T4 面 A 23 → T5 面 B 8 → T6 std.test 2 → T7 codegen **0**——逐枚翻绿零滞留；既有 508 零回归（全程唯一触碰 = advancetime-bnd 随批翻绿，T4 披露）。**conformance 总数对账：543 = 508 既有 + 35 新增**（`ls testdata/cases | wc -l` = 543）。


- [x] T10 实现审查：welang-code-review 7 条（真二进制证据），发现项修复或披露
  来源：研发流程 + design D13
  验证：审查记录（7 条逐条 + 发现项处置）追加于 proposal.md

  **完成记录（2026-09-08）**：7 条全过，审查记录落 proposal.md「## 审查记录（2026-09-08，welang-code-review 7 条，通过）」——规范符合性（ch20 八 Requirement 抽查对齐：mock 需求逐句/advanceTime 四条件/assertions 特型面对账/诊断段在册）、验证诚实性（本审查期实跑全量 + 三面黑盒抽复现）、测试先行（T1/T2 红证据在案）、诊断协议（**六码 helps 与 registry 程序化逐字比对全 MATCH**、--json 零触碰、diagnostics.toml 零触碰）、单一权威（follow-up #12/#13 归档入册路径明确）、红线（refr/ 零命中、提交待明示）、最小验证（T9 全量 + 复验）。**发现 F1（当场处置）**：proposal 影响范围表漏列 codegen.go/cli build.go（design D9 定夺面）——表已补两行，审查记录披露。status → complete（`validate --all --strict` OK）。

- [x] T11 归档与 roadmap 手术：archive-sync 五步（status → archived、目录带日期前缀）；M10 行拆 M10a/M10b/M10c 三行（双语）+ M10a 翻 done + follow-up #12 登记（test 模块可导入性 vs E0013 段集张力）
  来源：proposal 目标 8 / design D10 + 研发流程
  验证：gofmt -l 空、go build/vet 过、validate --all --strict 过、docs_sync 对数不变、git diff --check 干净；归档目录存在、roadmap 三行与 follow-up #12 在册

  **完成记录（2026-09-08）**：**规范提升零步**（纯实现无增量，M9a/M9b 先例）；**决策提升零枚**（无新 ADR——三裁决 Q1 切分是过程面、Q2/Q3 路由 M10b、保守读皆任务内披露，长期张力由 follow-up 承载）。**roadmap 手术（双语）**：M10 行拆三行——M10a `testing-check`（ch20 检查塔描述）翻 **done 2026-09-08**、M10b `testing-run`（多函数发射/we test runner/虚拟钟/确定性调度/test 边界/mock 拦截）pending、M10c `testing-explore`（交错探针/E1901/E1902 守卫/POR）pending——行文 design D10 逐字；**follow-up #12/#13 登记**（双语，design D10 文本：test 模块可导入性 vs 模块路径字符集；test 关键字 vs std.test 段——含 M10a 已落的可拼面与别名形决议）。**归档**：change.yaml complete → archived；目录 → `openspec/changes/archive/2026-09-08-testing-check/`（未跟踪目录故 mv 非 git mv）。**复验**：`validate --all --strict` → OK（零 active 变更、registry clean）；`docs_sync` 31 对齐（对数不变——roadmap 双语对同步编辑）；gofmt -l 空、go build/vet 过、`git diff --check` 干净。

- [x] T12 记忆更新与提交：project-overview 增 M10a 条目（下一里程碑指向 M10b testing-run）、MEMORY.md 索引行更新；英文提交信息（无署名 trailer；refr/ 不入提交；等用户明示提交）
  来源：研发流程 + 用户红线
  验证：`git log -1` 消息为英文且无 Co-Authored-By；`git show --stat` 无 refr/ 路径

  **完成记录（2026-09-08）**：project-overview.md 增 testing-check (M10a) 条目（M10 拆三/五面机制/conformance 508→543/黄金修正披露/follow-up #12/#13/下一里程碑 M10b）+ MEMORY.md 索引行更新（M0–M10a、conformance 543、下一 M10b testing-run）。**提交前实况（`git status` 对账）**：modified 10（roadmap ×2、ast、cli/build、codegen、conformance advancetime-bnd、parser、parser_test、typecheck、typecheck_test）+ untracked 39（35 新黄金 + parser/typecheck/codegen m10a_test.go ×3 + 本归档目录）= 49 项，`git status --porcelain | grep refr/` 零命中——与 proposal 影响范围表逐项对齐。用户明示提交（2026-09-08），提交信息 `Land M10a testing-check: chapter 20 check tower`（英文、无署名 trailer）。
