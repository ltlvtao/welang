# tasks — wec-parser（B3c）

> 权威：proposal.md（范围）+ design.md（D0–D8）。每任务实现前待用户点名；红先纪律 = 对应塔语料子集在面落成前取红态（构建失败 / 输出字节发散两形，退出码与输出记档）。

## T1 塔先行 v2 + AST 序列化器 + parse 骨架

- [x] **塔 v2 红先**：`internal/wec` 对账扩为**单一全输出对账**（design D5）——参考侧 B3b serialize + `parser.Parse` 三返回分派（E 行/N 行/ast 段三形拼接）；`TestLexReconciliation` 退役由 `TestOutputReconciliation` 承接（塔日志记演进一行）；Go 侧 AST 序列化器（design D4：P 行族总则 + 全 50+ 节点型对照 `internal/ast` 落定，空源形 ~~`P file 1 1`~~→`P file 0 0 0` 为首锚——`ast.File` 无 Line/Col 字段，D4 无位者 0 律 + isTestModule 第三栏，design 补记一-1）。红态 = wec 输出尚无解析段 → 全量语料发散记档。
  来源：proposal 目标 3、4；design D4、D5
  验证：红态记档（发散条目数与首例）；序列化器自一致单测（空源/单语句两形）；`go vet ./internal/wec/`
  落地（2026-09-20）：序列化器 `internal/wec/astser_test.go`（~60 P 行形全表 + parseSection 三返回分派；钉两形 `TestParseSectionPinsTheFormat`，钉逮手推错一枚——Binding 的 NameLine 5→1）；红态 = **95 绿 / 1009 发散、首例 `literal:empty` 字节 18**（恰在 ast 段应起处；95 = 词法错条目 E 单行态不含解析段）；门槛棘轮 97 = 95 + 空源 2（D5「门槛=无门槛」句的 as-built 修正，补记一-2：发散记日志不算失败、失败面 = 门槛 + ran==0，T6 升全量）；发散报告改首差字节 + ±48 窗口；语料 as-built **1106**（parse.we 自入 +1；放行 1105、剔 1 = #30 lextest `"\xff"`）；`go vet` 净
- [x] **parse 骨架绿一枚**：`wec/src/parse.we` 立 Parser record（D1 字段表全量）+ Token record + T 行游标（next/peek/cur/brokeLine + unesc，D2）+ `run(toksText: String, docsText: String) -> String` 入口（parseFile 空源形：tokens/docs 原样透传 + `ast` 头 + ~~`P file 1 1`~~→`P file 0 0 0`）；`main.we` 接线（lex E 行则原样止，否则两段文本入 parse.run、输出追加）。首枚语料（空源）塔绿。
  来源：proposal 目标 1；design D1、D2
  验证：空源语料 `bytes.Equal` 绿；`go test ./internal/wec/ -count=1` 该子集绿；`we check wec/` 干净
  落地（2026-09-20）：Token 重建 = 模块级 `tokAt(toksText, pos)`（初值在构造字面量内给——**局部 record 字段赋值 `p.cur = x` 是解析层 E0105**，与停面 H 同律：字段形唯 impl `self.field`）；lexnum 增 `byteChar`（probeByteValue 之逆：34 值控制表 + 四探针表的泄漏位）；**语言面地图新停面 I：`var` 绑定构造 record = M9b 语句集停**（探针隔离钉死：同形 `let` 绿——lex.we `let l = Lexer{…}` + impl 变异即既有绿习语，照抄；补记一-3）；分段逻辑住 parse.we 的 `drive` 适配器（main 保持极小三行纪律）；空源冒烟字节 = 钉原样（`tokens\nT 1 1 eof \nast\nP file 0 0 0\n`）；`we check wec/` exit 0、塔 `reconciled 97 of 1105` 全量绿

## T2 T 游标消耗 + 文件结构与顶层项 I

- [x] **parseFile/checkStart/parseItem 分发 + import/effect/fn 基形/参数/顶层 let**：checkStart 列锚；parseItem 关键字分发（含 var 顶层 E0403、return 顶层 E0401、mock 顶层 E1802）；fnDecl（名字/参数表 annotatedParams——**bnd 闩于参数位 `mut` 词**、返回位、效果段）；顶层 let；命名三检 E0011/E0012/E0013（isPascal/isCamel/isModuleSeg 字符集逐字对齐）+ E0404 撞名（namesText 编码栈 declare/查名——撞名压倒命名的次序照 Go）；行续判 brokeLine 于 if-else 与优先级循环的两处用点。
  来源：proposal 目标 1、今日事实 1–4；design D1、D2、D6
  验证：命名矩阵 + 撞名 + 顶层错码语料先红后绿；stdlib 源子集（import/fn 密集）翻绿；`we check wec/` exit 0
  落地（2026-09-20）：parse.we ~1350 行（33 个 impl 方法 + run/drive）；t2: 语料 56/56 全绿（红先态见 T1 记录）；棘轮 97 → **206**；`we check wec/` 与 `we build wec/` 皆 exit 0；stdlib 翻绿 1 枚（io.we——import/fn 密集面）；发散余量 = case 945 + stdlib 6 + wec 4（literal/lextest 全绿）；三枚停点构造与两枚消息面修正入 design 补记二（var 绑 record 停的推广、调用结果字段读停、#31 let-别名复发被塔逮住、lexnum.quote 单 rune 面须换 quoteText）
- [x] **属性与 pub 前缀族**：pub 前缀序（pub→byval→声明；byval type/record；byres record）与 pub 尾随形错码；import 路径 isModuleSeg；effect 声明（E1403 内建标签冲突 + 命名次序）。
  来源：今日事实 4；design D6
  验证：前缀序正负矩阵语料先红后绿
  落地（2026-09-20）：pub/byval/byres 三族分发 + 尾随形 E0105 逐形（含 byval 后 pub 的前缀序消息、byres 的 record 限定、pub byres 的 eof 形以 pub 锚位报错照 Go）；isModuleSeg 双源词（ident/keyword——std.test 须拼出，chapter 1 保留闭包只辖名位）；E1403 与 E0404/E0012 的优先级矩阵（撞名 > 内建冲突 > 命名规约）逐形对齐；P fn 行的 recv 空位（名后两空格 = 名域界 + 空 recv 域）补记于补记二-4

## T3 声明与类型引用

- [x] **复合声明族**：record（gc/value/resource 三形、字段表 FieldDecl、derives）、sum（variant 载荷形 + 无载荷形）、newtype、interface（MethodSig/AssocDecl）、impl（AssocBinding/WhereBound/TypeEq/TypeParam）、foreign 块的声明面（E1701–E1704 产生式）。
  来源：proposal 目标 1；design D4（各节点 P 行）
  验证：conformance 源声明密集子集先红后绿；E17xx 矩阵（parser 产生式侧）
  落地（2026-10-09）：parseSumDecl/parseRecordDecl/parseNewtypeDecl/parseInterfaceDecl/parseImplDecl/parseForeignDecl 六族 + typeParamsText/whereText/derivesSlot；载体判决 = interface 两缓冲分组（assocs 先于 methods，与源序解耦）vs foreign 逐项追加（单一切片源序承载）入补记四-2；E1701 的 ABI 闭集校验经 descText 拆码（\u{} 域检 + runeText 算术渲染，补记四-4）；收口期塔逮真缺陷两枚（sum 变体漏入册——t3 语料与 conformance `check-e0404-variant-fn` 双面捕获；runeText 表基差一）订正入补记四-3；T3 期自举内存面修复见补记三；E17xx 矩阵全绿（E1702 语句位归 T4）
- [x] **typeref 族**：parseTypeRef（Pascal 名/qual 链/泛型实参 anglePos）、元组型（E0602 arity）、unit 型、fn 型、Dyn<I>；NamedType 的 ArgLine/ArgCol 位。
  来源：今日事实 4；design D4
  验证：类型标注密集语料先红后绿
  落地（2026-10-09）：parseTypeRef（Pascal 名/qual 链/泛型实参 anglePos——where-bound 的 iface 名参考不填角锚，泛型应用形下仍拼 0 0，补记四-5）、元组 E0602、unit、fn 型（effect 标签）、Dyn<I>（钉拼写——具名应用面 T2 已开）；t3: 语料 144/144 绿、棘轮 206 → **381**（conformance case 翻绿 30 + stdlib 1——纯声明形源；发散余量 = case 915 + stdlib 5 + wec 4）；红先态补取（原始红态未随开发期记档——OOM 期跨段）：parse.we 回退 HEAD=T2 全量复跑（2026-10-09），t3: 143/144 发散（`t3:ty-dyn` 在 T2 解析器下即绿——钉拼写枚的本义）、reconciled 恰 207 = 206 基线 + 该枚

## T4 块、语句与模式

- [x] **block 与语句族**：block（direct/inTestBody 深度律——mock 唯一合法深度 2）、binding/assign（赋值目标规则——单字段形 self.field，We 与 Go 同律）、return（fnsText 栈的 hasRet 检查）、if/else-if（补尾 else E0202）、while/loop/break/continue（loopDepth）、defer（E0204 一层）、for、scope/scope resource/timeout、ExprStmt 与尾表达式律。
  来源：proposal 目标 1；design D1（fnsText 栈）、D6
  验证：控制流矩阵语料先红后绿；深度律正负形（mock 深度 0/2）
  落地（2026-10-09，上半刀——T4 两复选框分两刀，match/模式族/test/mock 归下半）：blockLoopText（fctxLine 承 fn 体/defer 体两类上下文；inTestBody 1→2 else→0 深度律；双 `}` 闭口 + 分隔 E0105）、stmtText 关键字分发（22 词闭集逐词对账 parser.go:2158-2161；self.field 与裸名赋值双探头 = tokAhead 只读行走；mock 深度 0 报 E1802、深度 2 闩 T4-B）、returnText（E0401 双形 or 起 fnsText 空、E0402 fnsTopName）、if/else-if/else 块（else-折叠 P 行手术 = cnt==1 && 剥 `P block` 行 && `P exprstmt ` 前缀 && `P if ` 前缀，Go len(Items)==1 && ExprStmt && If 的字面等价；BlockExpr 锚点在闭括号后 cur）、while/loop/break/continue（loopDepth 存取）、deferText（direct 律 E0204/E0203、体上下文空名行 `;0;1`）、forText/forHeadText（`(` 元组形闩 pattern——T4-B）、scopeResText（head 深度只包绑定表、E0404 头措辞、`_` 拒绝）；timeout/ch18 复合 scope 形走 exprstmt → T5 闩（Go 同形）；消息拼写逐条对 parser.go 复核逮 em dash 误拼一枚（E0103/assignAfterExpr-E0105 的 `—` 与引号形——语料未命中处塔不设防，人工对账面）；语料 t4: 39/39 绿（红先态记档 /tmp/t4-red.log：reconciled 382/1343、t4 37/38 红——`t4:err-break-top` 红态即绿，顶层 fallthrough）；棘轮 381 → **523**（翻绿 141 = t4 38 + stdlib io/test/time 3 + conformance case 100；发散余量 821 = case 815 + stdlib 2 + wec 4：expr 689/test 49/pattern 2/docs 1 + 窗口未露 N 行同类 80）；塔 330.7s（T3 收口 247.7s——增量主项 wec:parse.we 自源 51.6KB → 151.3KB 在补记三-5 charAt O(R²) 面上的放大，前缀计时 600/1200/2400 行 = 4.5s/23.4s/>60s 复钉该面仍在；非 T4-A 回归——闩点在词法完成后极早处，慢全在词法扫描）；补记五
- [x] **match 语句与模式族**：PatLiteral/PatWildcard/PatBinding/PatOr/PatTuple/PatVariant（守卫形）；test 块（testModule 名尾律）与 mock 声明的解析面。
  来源：proposal 目标 1；design D4
  验证：模式矩阵语料先红后绿
  落地（2026-10-09，下半刀）：parsePatternText（or 锚首 `\|`、两遍检查序 = 逐分支查重后 or 名集律——Go checkDupBinds/checkOrNames 次序）、patternAtomText 六原子、patVariantText/patTupleText（单元素无逗号 E0105@open、尾逗号闭口免 <2 检查两 quirk 照抄）、matchExprText/matchArmText（守卫 `if` 走 headExprText、`=>` 检查、exprValue 场域存取）、letTupleText/irrefutableText、parseTestDecl（inTestBody 1→体块、E1801 体后）、mockDeclText（`->` 返回先于效果段——与 fn 声明相反；单段 qual；fnBlock(display, hasRet)）；载体 = binds 标记方案（or 祖先行 `!` 前缀：checkDupBindsOf 跳过/namesAllOf 剥一重，segs `;;\n` 分段；markBinds 幂等——嵌套 or 角语料钉）；判据修复两枚 = ① namesLineOf 哨兵 0→−1（顶层元组 let 空名登记行 0，Go 按 map 存在性命中 ⇒ 第二枚 E0404@0:0 引行 0；三调用者 `!= 0`→`>= 0`，另两处行恒 ≥1 等价）② parseBinding `(` 臂平铺病灶（default 兜底误吞成功臂——首绿跑 594 逮出、修后 600）；语料 t4b: 44/44 绿（42 原始 + 2 补钉；红先态 /tmp/t4b-red.log reconciled 523/1386、42/42 红；收口期改拼 2 + 补钉 2 的 latch 态论证披露于补记六）；棘轮 523 → **600**（翻绿 77 = t4b 44 + conformance case 33；发散余量 788 = case 780 + stdlib 4 + wec 4，种类 = expr 787 T5 面 + docs 1 T6 面）；补记五-1 分类账订正（821 = case 813 + stdlib 4 + wec 4，原记 815/2）；塔侧 E1801 披露（固定输入名恒非 _test.we 名尾 ⇒ P test/mock 行塔内不可观测，拼写按 astser 权威表人工钉）；塔 490.9s/495.6s 两跑（T4-A 330.7s——parse.we 151.3KB→179.6KB 在 charAt O(R²) 面的放大，补记三-5 同案）；补记六

## T5 表达式与洞

- [ ] **表达式核**：parseExpr 十二级优先级爬升（非结合级 9/12 的 E0104 链式判 + depth==0 行续判 + `=` 非表达式 E0103）、unary（!/−/~）、postfix 链（call/member/index）、primary（ident/literal 前缀 tuple/construct/闭包/list 字面/blockExpr/if-expr/match-expr/task/scope/select）。
  来源：今日事实 4；design D6
  验证：表达式密集语料（conformance 源大头）先红后绿；E0103/E0104 矩阵
- [ ] **词法两面 + 插值洞**：lex.we 增 `holes`/`scanAt`（design D3——复用扫描核，`run` 面字节不动）；parseHoles（H 行族消耗、区域内偏移移一、Col = 洞列 +2）+ parseHole 子解析器（共享 valSite/depth/fnsText、namesText 新建；空区域 E0105、尾随词 E0105）。
  来源：proposal 目标 2、今日事实 5；design D3
  验证：洞语料（平衡/嵌套/多洞/洞内调用）先红后绿；lex 对账面零漂（`run` 字节不动由塔全量钉住）

## T6 docs 附着 + bnd 面 + 全量对账 + 突变电池 + 零漂移

- [ ] **attachDocs + NotImplemented 面**：itemStartsText 消耗（首个大于 EndLine 的行定附着，E0405 孤儿 docs）；DocAttach 的 P 行族；bnd 闩全链（参数位 `mut` → tokens/docs 透传 + N 行）。
  来源：今日事实 1、2；design D4、D6
  验证：docs 语料（stdlib 源为主）+ bnd 语料先红后绿
- [ ] **全量逐字节对账**：语料 1105 条四态全绿（词法错 E 单行 / 解析错 tokens+docs+E / bnd tokens+docs+N / 成功至 ast 段尾）；条目四态分布记账。
  来源：proposal 目标 4；design D5
  验证：`go test ./internal/wec/ -count=1` 全量绿
- [ ] **突变电池 ≥6 枚 + 零漂移**：D7 候选池实跑取 ≥6，逐枚「施加 → 击杀 → 还原 → 复跑绿」判决表入补记；conformance 933 `-count=1` 零改写、`go build ./...`、`go vet ./...`、`gofmt -l`（refr/ 外 0 文件）、`git diff --check` 净。
  来源：proposal 目标 5、6；design D7
  验证：判决表 + 各命令退出码 0 + testdata 零改动

## T7 审查与归档

- [ ] **完成审查**：welang-code-review 七条（span 对账、验证诚实性、红先在册、诊断协议、单一权威、红线、最小可信验证）；发现随审处置。
  来源：AGENTS.md 流程
  验证：审查表入 proposal「完成审查」；七条全过
- [ ] **roadmap + follow-up + 归档**：B3c 行翻写 done（as-built 收口）；follow-up 无预期新账（有则登记）；`change.yaml` archived；目录 `mv` 至 `archive/2026-…-wec-parser/`；归档提交。
  来源：proposal 目标 7；B3a/B3b 归档先例
  验证：`validate.py --all --strict` 归档后期望态；归档复验（17 包 0 FAIL、conformance 933、docs_sync 对数）
