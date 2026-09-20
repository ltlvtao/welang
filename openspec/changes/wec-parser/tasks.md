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

- [ ] **parseFile/checkStart/parseItem 分发 + import/effect/fn 基形/参数/顶层 let**：checkStart 列锚；parseItem 关键字分发（含 var 顶层 E0403、return 顶层 E0401、mock 顶层 E1802）；fnDecl（名字/参数表 annotatedParams——**bnd 闩于参数位 `mut` 词**、返回位、效果段）；顶层 let；命名三检 E0011/E0012/E0013（isPascal/isCamel/isModuleSeg 字符集逐字对齐）+ E0404 撞名（namesText 编码栈 declare/查名——撞名压倒命名的次序照 Go）；行续判 brokeLine 于 if-else 与优先级循环的两处用点。
  来源：proposal 目标 1、今日事实 1–4；design D1、D2、D6
  验证：命名矩阵 + 撞名 + 顶层错码语料先红后绿；stdlib 源子集（import/fn 密集）翻绿；`we check wec/` exit 0
- [ ] **属性与 pub 前缀族**：pub 前缀序（pub→byval→声明；byval type/record；byres record）与 pub 尾随形错码；import 路径 isModuleSeg；effect 声明（E1403 内建标签冲突 + 命名次序）。
  来源：今日事实 4；design D6
  验证：前缀序正负矩阵语料先红后绿

## T3 声明与类型引用

- [ ] **复合声明族**：record（gc/value/resource 三形、字段表 FieldDecl、derives）、sum（variant 载荷形 + 无载荷形）、newtype、interface（MethodSig/AssocDecl）、impl（AssocBinding/WhereBound/TypeEq/TypeParam）、foreign 块的声明面（E1701–E1704 产生式）。
  来源：proposal 目标 1；design D4（各节点 P 行）
  验证：conformance 源声明密集子集先红后绿；E17xx 矩阵（parser 产生式侧）
- [ ] **typeref 族**：parseTypeRef（Pascal 名/qual 链/泛型实参 anglePos）、元组型（E0602 arity）、unit 型、fn 型、Dyn<I>；NamedType 的 ArgLine/ArgCol 位。
  来源：今日事实 4；design D4
  验证：类型标注密集语料先红后绿

## T4 块、语句与模式

- [ ] **block 与语句族**：block（direct/inTestBody 深度律——mock 唯一合法深度 2）、binding/assign（赋值目标规则——单字段形 self.field，We 与 Go 同律）、return（fnsText 栈的 hasRet 检查）、if/else-if（补尾 else E0202）、while/loop/break/continue（loopDepth）、defer（E0204 一层）、for、scope/scope resource/timeout、ExprStmt 与尾表达式律。
  来源：proposal 目标 1；design D1（fnsText 栈）、D6
  验证：控制流矩阵语料先红后绿；深度律正负形（mock 深度 0/2）
- [ ] **match 语句与模式族**：PatLiteral/PatWildcard/PatBinding/PatOr/PatTuple/PatVariant（守卫形）；test 块（testModule 名尾律）与 mock 声明的解析面。
  来源：proposal 目标 1；design D4
  验证：模式矩阵语料先红后绿

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
