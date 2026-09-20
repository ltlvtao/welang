# proposal — wec-parser（B3c）

> 状态：candidate（勘查已完成，待 spec-impact audit）

## Why

B3b 收口（归档提交 `8b86568`，conformance 933、17 包全绿、语料 1105 条 reconciled 1104、突变电池 M1–M6 全塔层击杀），wec 已持词法段：`wec/` 顶层项目 + `internal/wec` 对账塔 + T/D/L/E 行序列化规范。本变更是 B3 六段切片（轨道裁决 1）的第三段：**Go parser（`internal/parser/parser.go`，3865 行、~130 函数）移植为 We 源**——自举轨第一段真正消费词法输出的编译器代码。

本段自己的差分杠是**解析层逐字节对账**（区别于 B3e 的 .ll 逐字节杠）：wec 二进制对语料的输出在既有 tokens/docs 段之后追加解析段，整输出与「Go 词法器 + Go parser 活算」的同规范序列化逐字节相同。参考侧仍由 Go 测试塔活算——零手写期望。

B3b 对本段的预留在此兑现：词法侧 `ScanAt`/`Holes` 两面（B3b proposal 非目标明记「插值洞区域再基是 parser 的地基」）与序列化规范的扩栏（B3b design D4 预告「后续段（B3c+）沿袭扩栏（Parser 行类）」）。

## 今日事实（勘查 + 语言面探针，HEAD `8b86568`）

**Go parser 形状**（移植面，逐条有锚）：

1. 入口 `Parse(name string, src []byte) (file *ast.File, d *diag.Diagnostic, ni *NotImplemented)`（`parser.go:143`）：`lex.Scan` 后 `parseFile`；两形解栈——`stop{d}`（诊断）与 `bstop{what}`（NotImplemented 第三返回列），`Parse` 的 recover 双收。**NotImplemented 唯一边界词 `bndMutPar = "mut parameters"`**（`:657`，`annotatedParams` 遇参数位 `mut` 词）。
2. 状态载体 `parser` 结构（`:117` 邻域）：`toks []lex.Token` + pos 游标 + `last` Token（行续判 `brokeLine = cur().Line > last.Line`）、`depth`（括号区域内行不敏感）、`fns []fnCtx` 栈（name/hasRet/deferBody 三员）、`loopDepth`、`direct`/`noBrace`/`shortBody`/`fnBrace`、`valSite`、`names map[string]int`（模块名单一名字空间；值 = 声明行，E0404 消息引用）、`docs []lex.DocUnit` + `itemStarts []int`（`attachDocs :2022` 的孤儿 docs 判 E0405）、`testModule`（名尾 `_test.we`）与 `inTestBody` 0/1/2（mock 唯一合法深度 2）。
3. 诊断 31 码（E0105 ×284 主导；命名三检 E0011 Pascal/E0012 camel/E0013 模块段；E0404 撞名——同词上撞名压倒命名规约；顶层 E0401 return/E0403 var/E0405 孤儿 docs；E1802 mock 顶层）。`fail` 经 `panic(stop{d})`（`:186`）；**helps 表（~30 条）挂 `WithHelp` 独立字段——`Message()` 只回 message（`diag.go:80`），序列化不含 help（B3b `wec_test.go:57` 注释先例）⇒ E 行面无需复刻 helps 表**。
4. 分发与代表机制：`parseItem :317`（pub 前缀序 pub→byval→声明；byval type/record；byres record；var 顶层 E0403；return 顶层 E0401；mock 顶层 E1802）；`parseTypeRef :3665`（Pascal 名 + 小写 qual 链 + 泛型实参 `anglePos`、元组 arity E0602）；`parseBinary :3119` 十二级优先级爬升（非结合级 9/12 链式 E0104；`depth == 0 && brokeLine` 行续判；`=` 非表达式 E0103）；`attachDocs :2022`（`itemStarts` 首个大于单元 EndLine 的行定附着一一否则 E0405）。
5. 插值洞机制：`parseHoles :946` → `lex.Holes(name, lit.Text, lit.Line, lit.Col)` 报 `[]HoleRegion{Start,End,Line,Col}`（洞文本 `Text[1:len-1]`、Col = 洞列 +2、偏移移一）→ `parseHole :972` → `lex.ScanAt(name, region, r.Line, r.Col)` 再基词法 → 子解析器共享 name/valSite/depth/fns、names 新建；空区域 E0105、区域尾随词 E0105。lex 侧：`Holes :549`（重走字面文本，复用 consumeEscape/consumeHole 平衡）、`ScanAt :154`（`runAt(name, src, line, col, false)`——行/列再基）。
6. AST（`internal/ast/ast.go`，770 行）：50+ 节点型——Item 8 形（Import/FnDecl/TopLet/SumDecl/RecordDecl/NewtypeDecl/InterfaceDecl/ImplDecl/EffectDecl/TestDecl/MockDecl/ForeignBlock）、Stmt 13 形、Expr 20+ 形、Pattern 6 形、TypeRef 8 形、File/DocAttach——序列化规范（design D5）的对象面。
7. **T 行逆转可逆性**：esc 机械（`\` 双写、0x21–0x7E 直通、余者 `\` + 两位大写十六进制——`wec_test.go:35`）逐字节可逆；字段以未转义空格分界；词语文本恒良构 UTF-8（词法器已验），逆转结果 String 可载。
8. **解析消息无字节值角**：消息 `%q` 只作用于词文本（恒良构），无 E0001 类 `byte 0x%02X` 角——B3b 的 #30 十三值角不涉本段。

**We 语言面地图**（探针 20 枚，`/tmp/b3cprobe`，逐形记退出码与诊断，不向仓）：

绿面（载体可用，全形今日实证）：

1. **record 嵌 record 字段**：构造 + 链读（`h.t.k`）+ impl `mut self` 整字段写（`self.cur = Tok{…}`）——Token 入 Parser 状态载体的形（P1/P1b）。
2. **let List 局部全链**：`let xs: List<Int64> = [1,2]` + `xs.add(9)` 突变 + `size()/get()`——main 与 fn 体皆绿（P4c/P2d）；List 形参（P5a）、模块 fn 返回 List（P5b）、for-in 源（P5c）、`List<record>` gc 元素构造 + 形参（P5d）、impl 方法 List 形参（P15）。**成员名勘定：`add`（`push` = E0816）**。
3. **Option 消费**：`xs.get(0)` 的 `Option<Int64>` 变体模式 match（P6d）；`Option<String>` 块臂早返回 match + `Some("x")` 实参位构造（P7b）。
4. **用户 sum 载荷全链**：声明 `type Shape = Circle(Int64) | Tag(String)` + 构造 + 变体模式 match + 载荷绑定 + `_` 载荷臂（P8；或模式/守卫拼写见 conformance 黄金先例）。
5. **跨模块 record 面全绿**：fn 返回 record 后字段读（main/fn/impl 三位，P9/P9b/P9c）、限定构造 `lib.P { … }`（P9d）、record 作跨模块实参（P9f）、跨模块构造后调方法（P9g）、String 字段跨界（P9e）。
6. B3b 已证面沿用：charAt 走查 + runeCode 预测宽、String 拼接累积、递归、`\u{}` 逃逸、`fs.readFile` 的 `Result` match（main 体）、B3a `std.string` 转换（`parseUInt` 等——行/列数读写的来源）。

停面（exit 70 诚实边界，逐形钉死）：

- **A** `var` List 绑定（P2e/P2c）——let 全绿，突变走 add 句柄语义（B2b 稳定句柄）。
- **B** record 的 List 字段——一切形、main/fn 两位皆停（P3 族）。
- **C** impl 方法返回 List——未调用也停，形状本身停（P14）。
- **D** record 的 Option 字段（`Option<Node>` 递归树）（P6 族）。
- **E** Int64 被匹配对象的 match——字面臂/通配臂、值形/语句形皆停（P12 族）⇒ 标量分派走 if-else 链（补尾 else E0202 纪律，B3b 先例）。
- **F** String 臂的 `return match` 值形（skStr 值形——B1b 既有面，P12 首形）。
- **G** main 体构造 List/Option/sum 字段的 record（P3b/P6a/P8b）⇒ 端口纪律：main 极小（readFile → run → print → Ok），一切构造入 fn/impl。
- **H** `self.f.g = x` 双层字段赋值——**解析层 E0105**（We 与 Go 同规：单字段形 `self.field` 才是批准形，P17）——端口照抄同律。

与 B3b 停面地图的差异披露（三处，皆今日逐形重测为准；其间零 Go 生产码改动，地图原条目的形与此处诸形不同形）：地图②「var List 局部停」真身是 **var 绑定**（let List 全绿含 add 突变）；地图③「Option\<String\> 载荷 match 停」的停面在**值形/String 臂族**（块臂早返回形全绿）；地图④「跨模块 record 字段停」今日**全探形皆绿**。B3c 以今日地图为权威。

## 目标与非目标

目标（一条线：parser 移植 + 词法两面扩建 + 全输出逐字节对账塔）：

1. **parser 移植**：`Parse` 面完整移植为 We 源（`wec/src/parse.we`，单模块——design D2）；31 诊断码逐码保真（消息逐字，含 `%q` 拼写）；首错停 `panic(stop{d})` 改错误闩（B3b 先例）；`bstop` 改 NotImplemented 闩（`bndMutPar` 唯一边界）。
2. **词法侧两面扩建**：`lex.we` 增 `holes`/`scanAt` 两 pub fn（B3b 预留的非目标，机械移植 `lex.go:549`/`:154` 家族）；`run(src)` 公开面字节不动。
3. **序列化规范扩栏**（design D5）：`ast` 段（`P` 行族前序一行一节点）+ `N <what>` 行（NotImplemented）+ E 行沿用；**输出四态**：词法错 = E 单行；解析错 = tokens + docs + E；bnd = tokens + docs + N；成功 = tokens + docs + ast。权威实现 = `internal/wec` 的 Go 序列化器（走 `internal/ast`）。
4. **对账塔 v2**：整输出逐字节对账（参考侧 = 词法序列化 + parser 活算序列化拼接）；语料沿用 1105 条；B3b 的词法对账测试被全输出对账**蕴含与承接**（测试面收敛为单一全输出对账）。
5. **突变电池 ≥6 枚**：优先级表/行续判/命名检查/撞名检查/前缀序/洞再基列锚等单点突变，塔逐枚击杀判决表（B2b/B3b 先例）。
6. **零漂移**：conformance 933 零改写全绿（零 Go 生产码改动，漂移结构上不可达，仍全量复验）。
7. **docs**：roadmap B3c 行归档时翻写；勘查未发现新语言面缺口——无预期新 follow-up（解析消息无字节值角，今日事实 8）。

非目标：

- **checker/codegen/CLI/test/fmt/vet/doc**（B3d–B3f）。
- **Keep 面**（B3f 格式化器的注释保形脸）。
- **语言面增量**：wec 是语言消费者不是拓宽者（规范先行纪律）；#31 let-别名错编译的绕行（`var` 快照）在新移植代码中按需沿用。
- **helps 表复刻**：help 是注册表静态属性，`Message()` 不含，序列化面无它。
- **诊断文件名**：两侧共用固定名，序列化省略（B3b 先例）。
- **Go 生产码改动**：零——`internal/wec` 是纯测试塔。
- **性能调优**：String 栈编码的线性扫描与拼接在语料尺度可接受（B3b 拼接先例）。
- **13 字节值角**：不涉解析层（今日事实 8）；词法段既有披露（#30）不动。

## What Changes

- `wec/src/parse.we`（新）：parser 主体（状态载体 + T 行游标 + 文件/项/声明/语句/模式/表达式/类型引用/洞）。
- `wec/src/lex.we`（改）：增 `holes`/`scanAt` 两 pub fn（复用已移植扫描核）。
- `wec/src/main.we`（改）：驱动接解析段（lex E 行则止；否则 tokens+docs 后追加解析段）。
- `internal/wec/`（改）：AST 序列化器（Go 权威实现）+ 塔对账扩为全输出逐比对 + 突变电池扩编。
- `docs/roadmap/0000-reference-implementation.md` + `.zh.md`：B3c 行归档时翻写。
- `openspec/changes/wec-parser/`：本四工件（随 T1 入册）。

## 影响层

`layers: [compiler, docs]`——**internals-only**：compiler 层的触达是编译器仓的测试面（`internal/wec`，不进任何生产管线；Go 生产码零 diff），wec/ 是 We 语言消费者程序（自举轨的编译器种子），二者皆不改变语言行为；docs 层是 roadmap 执行状态（归档时翻写）。**本变更「不改变语言行为」「无规范增量」**：零 Requirement 增删、零诊断码、零 ADR；不触 `docs/spec/`、`diagnostics.toml`、stdlib、运行时。

## 影响范围

- Go 工具链：零生产面改动——所有既有命令、管线、黄金零漂移（目标 6 全量复验）。
- 语料涉及面：stdlib 源、conformance testdata、lex/parser 测试样例皆**只读**消费。
- 后续段：B3d（checker）在 wec/src 上续建，消费本段钉死的 ast 段序列化；本段塔结构与 esc/T 行规范沿袭至 B3f。
- 包账预期：17 不变（`internal/wec` 已立）；conformance 语料账 933 不动。

## 审计记录

**welang-spec-impact-audit（2026-09-20，HEAD `8b86568`）：通过，七条全过。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | 问题真实性 | 工程缺口（B3 轨道第三段）：轨道权威 = roadmap B3c 行 + 策略节 B3 五裁决 1（六段切分）；B3b 预留在案兑现（B3b proposal 非目标明记 ScanAt/Holes 留 B3c、其 design D4 扩栏预告）——代码锚 `parser.go:143`（入口/双 recover）、`:317`（分发）、`:3119`（优先级）、`:2022`（attachDocs）、`:946`（洞）、`:657`（bndMutPar）、`lex.go:549/:154`（Holes/ScanAt）、`diag.go:80`（Message 不含 help） |
| 2 | 影响层声明 | `change.yaml [compiler, docs]` 与 proposal 影响层合一；internals-only 论证在案（Go 生产码零 diff、`internal/wec` 纯测试塔、wec/ 是语言消费者）；「不改变语言行为」「无规范增量」标记在影响层 |
| 3 | 规范增量范围 | 零 Requirement 增删、零诊断码、零 ADR（proposal 显式）；无全局唯一性冲突面 |
| 4 | 原则一致性 | 无冲突；规范先行纪律正面兑现——停面 A–G 不发明面（载体按今日地图裁决、标量分派走 if-else 链）、#31 绕行沿用、List 成员名 `add` 依 E0816 事实勘定（非 `push`） |
| 5 | 参考基线固定 | 无 refr/ 引用；规范锚 = 已批 `docs/spec/`（ch2 文法/ch6 声明——parser 行为的规范面、ch1 词法——两面扩建的地基）；实现锚 = HEAD `8b86568` 逐行号；探针地图 = `/tmp/b3cprobe` 2026-09-20 二十枚逐形（HEAD 工具链复现，不向仓） |
| 6 | 验收边界 | 目标皆命令化可判（塔全输出 `bytes.Equal` 逐语料、`-count=1` 全量、突变判决表 ≥6、零漂移全阶梯、validate 期望态）；非目标显式排除 B3d+/Keep/语言面增量/helps/诊断文件名/Go 生产码/性能/13 值角 |
| 7 | 粒度 | 单一垂直单元：一个移植面（Parse 家族）+ 两面扩建（holes/scanAt）+ 塔扩栏 v2；roadmap B3c 行无拆分注记（裁决 1 只给 B3d/B3e 授权现场拆分）；T3–T5 任务内子提交序自由度 = B1/B2「按实现落地」先例（design D8 声明，不拆 roadmap 行） |

## 审查记录

**welang-change-review（2026-09-20，ready → active 关卡）：通过，十条全过，一处随审订正。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | proposal 职责 | 黑盒缺口（轨道第三段的工程缺口）+ 裁决引用 + 目标/非目标 + 影响面；勘查与探针事实逐条带锚；无任务清单混入（阶梯在 tasks.md） |
| 2 | spec 增量 | 豁免（「不改变语言行为」「无规范增量」标记在影响层；internals-only 论证——validate --strict 已过） |
| 3 | design 职责 | 唯一最小路径 D0–D8；被拒方案四条带探针依据（D0 表）；锚精确到行（`parser.go:186/:657/:3119/:3665/:2022/:946`、`lex.go:154/:549`、`diag.go:80`、`wec_test.go:35`）；P 行全表委落 T1 序列化器为权威——总则（每节点每字段有位）先钉、代表性枚举在册，机械誊写面非未决选择 |
| 4 | tasks 职责 | 只执行 proposal/design 已定义工作；逐框来源/验证；**随审订正一处**——T2「编码栈declare」排版缺空格，已补；T6「实跑取 ≥6（候选池超额）」是声明的选择规则非未决 |
| 5 | 场景覆盖 | normal（成功态：stdlib/wec/conformance 正面源至 ast 段尾）/ boundary（行续判、非结合链式、pub 前缀序、mock 深度 0/2、洞空区域/尾随词、元组 arity E0602）/ failure（E 单行、tokens+docs+E、tokens+docs+N、首错停闩、读文件防御行沿用）三路齐 |
| 6 | 无空章节 | design「补记（实现期追加）」是 B3b 房式累积点非凑格空章；订正后无 |
| 7 | 测试先行 | T1 塔 v2 先落且解析段缺席红态记档（全量发散形）+ 骨架绿一枚；T2–T6 语料子集先红（输出字节不匹配形）后绿；红先两形在 tasks 头注声明 |
| 8 | 负向断言 | 负面语料是实构违规样例（命名矩阵、前缀序正负、E0103/E0104 矩阵、mock 深度负形、洞负形、bndMutPar 形）；突变电池 ≥6 枚含负向捕获性判决 |
| 9 | 完成度闭环 | 非语言特性变更，闭环三要素对位为：移植体（parse.we + lex 两面，D1–D3）、验证塔（`internal/wec` v2，D5/D7）、序列化契约（D4——B3d/B3e 的输入契约）——三者皆有方案且互相咬合 |
| 10 | 未决问题 | 无阻塞未决——B3b 停面地图三处差异（②var 真身/③块臂绿/④跨模块绿）以今日逐形地图为权威显式披露（proposal 今日事实末段）；#29/#30/#31 沿用不阻塞；B3d/B3e 现场拆分属轨道裁决 1 授权 |
