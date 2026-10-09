# design — wec-parser（B3c）

> 权威：proposal.md（范围与今日事实）。本件定载体、序列化规范、对账塔与任务切分；实现期订正以补记段追加（B3b 先例）。

## D0 总则：载体裁决

**流式序列化载体沿用**（B3b「边扫边拼 outText」先例）：解析过程中不物化任何树——每个 AST 节点在其解析完成点即拼进行输出 `astText`；错误闩与 NotImplemented 闩按 B3b 错误闩形。裁决由今日语言面地图（proposal 今日事实）驱动：

| 候选载体 | 判决 | 依据（探针） |
| --- | --- | --- |
| 真 AST 树（record 字段挂 List/Option 子节点） | 拒 | 停面 B（record List 字段全形停，P3 族）、停面 D（Option 字段停，P6 族） |
| var List 局部栈 | 拒 | 停面 A（var List 绑定停，P2e）——let List 虽全绿（P2d），状态载体需 mut self 字段位，字段位即停面 B |
| List 作递归实参（函数式穿透） | 拒 | 形可行（P5a/P5d 绿）但状态（闩/names/depth）仍需载体；双层载体复杂度不敌流式，且 impl 方法返 List 停（P14） |
| 字面量 match 标量分派 | 拒 | 停面 E（P12 族）——if-else 链 + 补尾 else（E0202 纪律） |
| **流式序列化 + String 编码栈 + if-else 分派** | **取** | 全构成面皆绿：record 嵌 record（P1/P1b）、String 拼接（B3b）、charAt 走查（B3b）、sum 载荷闩可选（P8） |

B3d/B3e（checker/codegen）消费的即 `astText` 段——本段钉死的序列化规范是它们的输入契约（B3b design D4 扩栏预告的兑现）。

## D1 模块切分与状态载体

**单模块 `parse.we`**（Go parser 亦单文件 3865 行；We 侧预计 4–5k 行）：record 定义与其 impl 必须同模块（跨模块 impl 无规范依据），拆模块只会切走自由 fn 而留不下状态载体——单模块免此险。`lexnum.we` 复用不动；`main.we` 只换驱动接线。

**Parser record**（全字段 scalar/String/record 三类——停面 B/D 即被载体排除）：

```
record Parser {
    toksText: String,      // 词法段的 T 行文本（驱动传入——lex 输出原样）
    docsText: String,      // 词法段的 D/L 行文本（attachDocs 的输入）
    pos: Int64,            // T 行游标（当前 T 行在 toksText 中的字节起点）
    cur: Token, last: Token,   // 同模块 Token record（六员：kind/text/line/col/attrName/attrArgs）
    depth: Int64, loopDepth: Int64,
    direct: Int64, noBrace: Int64, shortBody: Int64, fnBrace: Int64,   // 0/1
    valSite: String,
    fnsText: String,       // fnCtx 栈编码
    namesText: String,     // 名→声明行编码
    itemStartsText: String,// 顶层项起始行编码
    testModule: Int64, inTestBody: Int64,   // 0/1（inTestBody 0/1/2）
    astText: String,       // 流式输出累积器
    failed: Int64, errLine: Int64, errCol: Int64, errCode: String, errMsg: String,   // 错误闩
    bnd: Int64, bndWhat: String,            // NotImplemented 闩
}
```

**栈编码**（三栈同律：行式记录、`\n` 分界、线性扫描；名与数字的字符集保证编码无歧义——标识符不含空格/换行/分号）：

- `fnsText` 每项 `name;hasRet;deferBody`（fnCtx 三员；入栈 = 追加行，出栈 = 截断末行，栈顶 = 末行）。
- `namesText` 每项 `name line`（declare 入栈追加；查名线性扫前缀 + 行值供 E0404 消息）。
- `itemStartsText` 空格分界 Int64 序列（parseItem 追加；attachDocs 线性扫首个大于 EndLine 者）。

B3b 先例：consumeAttrArgs 的括号栈即 String 编码（design 补记四）；#31 绕行纪律（let-from-裸-var 用 `var` 快照）沿用到新代码。

## D2 T 行游标与 Token 重建

**Token 不跨模块复用**（lex.we 的 Lexer record 是内部状态，非公开面）：parse.we 自持 `Token` record，从 `toksText` 的 T 行**重建**——`next()`/`peek()`/`cur()`/`brokeLine()` 五访问对位 Go 的游标族：

- T 行形 `T <line> <col> <kind> <text>`（attr 词附名与实参两字段）——字段以**未转义空格**分界：走查中空格字节且前置反斜杠数为偶即界。
- **esc 逆转机械**：`\\` → `\`；`\` + 两位大写十六进制 → 字节；其余直通。词文本恒良构 UTF-8（词法器已验），逆转结果 String 可载。逆转是纯函数 `unesc(s: String, from: Int64, to: Int64) -> String`（charAt 走查 + 拼接，B3b 习语）。
- `pos` 越过末行 → eof Token（T 行族的 `T <l> <c> eof eof`——B3b 序列化的 eof 词拼写）。
- `brokeLine = cur.line > last.line`（Go 同式；行续判在 if-else 与优先级循环两处用点照抄）。

## D3 词法侧两面扩建（lex.we）

B3b 预留兑现，两 pub fn 皆内部复用已移植扫描核，`run(src)` 公开面字节不动（塔 v2 全输出对账钉住）：

- `pub fn holes(name: String, text: String, line: Int64, col: Int64) -> String`——`lex.go:549` 的移植：重走字面文本（复用 consumeEscape/consumeHole 的平衡逻辑），报 `H <start> <end> <line> <col>` 行族（区域内偏移；Col = 洞列 +2；文本 `text[1:len-1]` 的偏移移一规则在调用侧）。无洞答空串。
- `pub fn scanAt(name: String, src: String, line: Int64, col: Int64) -> String`——`lex.go:154` 的移植：`runAt` 再基（行/列初值换入已移植的走查状态），答 T 行族文本（或区域词法错的 E 行——由 parseHole 的调用侧按 E 行形识别）。**非对账面**（区域再基的中间产物；其正确性经子解析输出间接对账），We 侧答 T 行文本即子解析器的 toksText——形自洽。

## D4 序列化规范扩栏（权威实现 = internal/wec 序列化器）

**输出四态**（proposal 目标 3）：

1. 词法错：`E <line> <col> <code> <esc(msg)>` 单行（B3b 不变）。
2. 解析错：tokens 段 + docs 段 + `E <line> <col> <code> <esc(msg)>` 单行（E 行形同 1；首错停——诊断即输出之尾）。
3. NotImplemented：tokens 段 + docs 段 + `N <what>` 单行（`what` 不转义——ASCII 短语）。
4. 成功：tokens 段 + docs 段 + `ast` 头 + P 行族。

**P 行族**：一节点一行，**前序遍历序**（父先于子；兄弟按源序）——行序即树，无缩进无闭包行。行形：

```
P <node> <line> <col> [<标量字段…>] [<子占位>]
```

- 总则：`internal/ast` 每节点型的**每个字段**皆有序列化位；Line/Col 恒列（无位者 0）；Bool 作 0/1；String 字段 esc；Option 无值作 `-`；切片字段以元素 P 行的行序承载（无显式计数——子行连续）；空切片零行。
- 代表性枚举（全表落 internal/wec 序列化器为权威——本件不抄全 50+ 型，移植期逐型对照 ast.go）：

```
P file <line> <col>                      // File：子行 = 顶层项
P import <line> <col> <esc(path)>        // Import
P fn <line> <col> <esc(name)> <pub>      // FnDecl：子行 = 参数、TypeRef、Block
P param <line> <col> <esc(name)>         // Param：子行 = TypeRef
P let <line> <col> <esc(name)> <pub>     // TopLet/Binding
P binary <line> <col> <esc(op)>          // Binary：子行 = 左、右
P call <line> <col>                      // Call：子行 = 被调、实参…
P member <line> <col> <esc(name)>        // Member：子行 = 接收者
P literal <line> <col> <kind> <esc(text)>// Literal
P ident <line> <col> <esc(name)>         // Ident
P match <line> <col>                     // Match：子行 = 被匹配者、臂…
P arm <line> <col>                       // MatchArm：子行 = 模式、守卫?、块
```

- **文档附着**（`DocAttach`）：成功态 docs 段后、ast 头前行族？——**否**：DocAttach 是 File 的子行（`P docattach <start> <end> <itemIndex>` + L 行子行），在顶层项行族之前（Go `File.Docs` 的序列化位）。
- **Escaped 行为锚**：序列化器的 Go 实现是规范权威；We 侧逐拼对照。P 行拼写表在 T1 随塔落定（红先锚 = 空源 `P file 1 1` 形）。

## D5 对账塔 v2

- **单一全输出对账**：`TestOutputReconciliation`（承接并蕴含 B3b 的 `TestLexReconciliation`——该测试退役，其语料与断言由 v2 承接；演进非删面，塔日志记一行）。参考侧：`lex.Scan` 序列化（B3b serialize 原样）+ `parser.Parse` 三返回分派——`(nil, d, nil)` 追加 E 行、`(nil, nil, ni)` 追加 N 行、`(file, nil, nil)` 追加 ast 段（Go 序列化器走 internal/ast）。`bytes.Equal` 全输出。
- 语料 1105 条沿用（提取器零改动）；条目四态分布随面落成翻绿（门槛 = 无门槛：T1 起即全量跑，红态记档——B3b T5 摘围栏的终态直接继承，不再立阶段围栏）。
- **构建与驱动不变**：copyTree + `cli.Run` 构建 + `wec-input.we` 固定路径契约（#29 前不动）。

## D6 诊断协议

- **31 码逐码保真**：消息逐字（含 `%q` 于词文本的拼写、E0105 各形的具体文案）；E0404 撞名在同词上压倒命名规约（declare 次序照 Go）；命名三检（E0011/E0012/E0013）与 isKw/isPascal/isCamel/isModuleSeg 的字符集判定逐字对齐。
- **首错停**：fail 闩（B3b 形——闩后各方法入口早退）；闩即输出态 2 之 E 行。
- **bnd 闩**：`bndMutPar` 唯一（`annotatedParams` 的参数位 `mut` 词）；闩即输出态 3 之 N 行。Go 侧 `bstop` 的 panic 值经 `NotImplemented{What}` 第三返回列——两闩独立（诊断闩不掩 bnd 闩：Go 语义 bnd 先于一切诊断 panic——`p.bnd` 即 panic，闩序照抄）。
- **helps 非面**：`Message()` 不含 help（今日事实 3）；wec 不移植 helps 表。
- **消息无字节值角**（今日事实 8）——语料无 #30 类剔除。

## D7 突变电池（≥6 枚，B3b M1–M6 先例）

候选池（施加于 wec We 源单点，塔逐枚击杀 + 逐枚还原复跑绿；击杀层逐枚判定）：

| # | 突变 | 预期击杀面 |
| --- | --- | --- |
| M1 | 优先级表换目（binLevels 等值链错位一级） | 塔层（嵌套表达式发散） |
| M2 | brokeLine 反号（`>` → `>=`） | 塔层（行续形语料发散） |
| M3 | isPascal/isCamel 判定互换 | 塔层（E0011/E0012 矩阵发散） |
| M4 | names 撞名检查摘除（declare 不查重） | 塔层（E0404 语料发散） |
| M5 | pub 前缀序换序（byval 先于 pub 收词） | 塔层（前缀序语料发散） |
| M6 | 洞再基列锚 +1（scanAt 列基错位） | 塔层（洞内词位置发散） |
| M7 | itemStarts 追加摘除（attachDocs 无附着点） | 塔层（E0405/附着行族发散） |
| M8 | 非结合级检查摘除（E0104 链式放行） | 塔层（链式比较语料发散） |

实跑取 ≥6（候选超额——B3b 先例：突变设计随面落成微调，判决表入补记）。

## D8 任务切分与红先阶梯

对应 tasks.md T1–T7。红先纪律：每任务对应塔语料子集在面落成前取红态记档（构建失败 / 输出发散两形，B3b 形）。切分沿 Go parser 的章节簇：

- T1 塔先行 v2 + 序列化器 + 骨架（空源绿一枚）。
- T2 T 游标 + 文件结构 + 顶层项 I（import/effect/fn/let/命名检查/行续判）。
- T3 声明与类型引用（record/sum/newtype/interface/impl/typeref 族）。
- T4 块、语句与模式（block/stmt/match 语句/模式族/test/mock/foreign）。
- T5 表达式与洞（优先级/unary/postfix/primary/list/tuple/closure + lex 两面 + 子解析器）。
- T6 docs 附着 + bnd 面 + 全量对账 + 突变电池 + 零漂移。
- T7 审查与归档。

**实测过大现场拆分**：B3c roadmap 行无拆分注记（B3d/B3e 有）——但 B1/B2「按实现落地」先例保留：若 T3–T5 任一实测过大，在其任务内拆子提交序（不拆 roadmap 行）。

## 补记

### 补记一（T1 落地，2026-09-20）
1. **空源锚订正**：D4 枚举表与「Escaped 行为锚」句的红先锚 ~~`P file 1 1`~~ → **`P file 0 0 0`**。`ast.File` 无 Line/Col 字段（`ast.go:14-23`）——位栏按总则「无位者 0」取 0 0，第三栏是 IsTestModule（Bool 作 0/1）。tasks.md T1 两处同句同订正。
2. **D5「门槛 = 无门槛」句 as-built 收窄**：全量恒跑、无阶段围栏照写（B3b T5 终态直继）；但断言面 = **单调棘轮常量 `reconciledFloor`**——T1 落地值 97（95 词法错条目 E 单行态 + 2 空源），发散条目记日志（首差字节 + ±48 窗口）不算失败，`ran==0` 兜底提取器坏死。棘轮只升不降，T6 升至全量。「无门槛」原意（无准入围栏、全量从 T1 起跑）保持不变；无棘轮则中程绿面无回归保护，与 D5 自身「条目四态分布随面落成翻绿」的记账义务相抵。
3. **语言面地图新停面 I（探针钉死）**：`var` 绑定构造 record 形（`var p = Parser { … }`）= M9b 语句集停——We 的 `var` 语句集不含 record 构造表达式。**同形 `let` 绿**：lex.we `let l = Lexer { … }` + impl 内变异即既有绿习语，parse.we 照抄（Parser 构造以 `let` 落绑定、impl 方法内 `self.field` 变异）。另钉同族形：**局部 record 字段赋值 `p.cur = x` 在解析层即 E0105**——字段赋值形唯 impl `self.field` 合法（停面 H 同律），故 Token 游标重建为模块级 `tokAt(toksText, pos)`、初值在构造字面量内一次给足。
4. **P 行总则 as-built 细则**（序列化器头注即权威，`astser_test.go:21-30`）：可选指针子节点独占 `-` 行（空槽保持位形）；变长 `[]string` 字段（effect 标签、derive 目标、import 路径、插值段）**内联**为父行 esc 字段空格 join——esc 不出裸空格、标识符非纯数字，join 无歧义；节点切片仍按子行承载。File 位栏见订正 1。
5. **语料 as-built 1106**：提取器逻辑零改动，`wec:parse.we` 随新源自入 +1；放行 1105（剔 1 = lextest `"\xff"`，#30 十三值角原样）。塔日志行随实数打印。
6. **drive 适配器住 parse.we**：lex 输出的分段（E 行判定 + tokens/docs 两段切分）实现为 `parse.drive(lexOut)`——分段逻辑属解析侧职责，main.we 保持极小三行纪律（读源 → lex → 追加 drive）。
7. **序列化器钉形逮手推错一枚**：`TestParseSectionPinsTheFormat` 钉空源与单语句两形；单语句形的手推期望在 Binding 的 NameLine 上推错（5→1，构造字面量位不在名行）——钉形测试在落钉当日即修正了权威实现的一份手推拷贝，此即 D4「Escaped 行为锚 = Go 实现权威」的理由。

### 补记二（T2 落地，2026-09-20）

1. **假线索自我更正（在案）**：构建失败的归因先误记为 typecheck 的 bndArityGap——根因是 bisect 载具只判退出码、消息归因靠记忆。实停 = **codegen 的 M9b 语句集**（bndFnBody，`we check` 过、`we build` 返回 NotImplemented）。定位法 = 给 typecheck `bnd()` 与 codegen `e.bnd()` 临时加 `WEC_DBG_BND` 栈打印（debug.PrintStack），一跑即见发射路径；插桩验证后零残留撤销（`git diff` 两 Go 文件为空）。教训入档：bisect 判红必须连同失败消息一起记录。
2. **停面 I 推广——var 绑 record 一切形停**：T1 探针钉的是 `var p = Parser { … }` 构造形；实为 `emitVarBinding` 只答 String/数值面（codegen 侧结构事实），**方法调用初始化、let 拷贝初始化的 record var 同停**（parseImport 的 `var last = self.moduleSeg()`、typeRefText 的 `var last = t`）。绕行 = 段 token 三字段拆各自标注 var（`var segText: String`/`var segLine: Int64`/`var segCol: Int64`），同一纪律两处。
3. **新停面 J——调用结果的字段读停**：`emitMemberValue` 的成员链根须为绑定（`chainOf`），`self.peek().kind` 形停；let 绑记录的字段读绿（parseItem 的 `t.kind`/`t.text` 即既有绿形）。绕行 = 先 `let pk = self.peek()` 再读（parsePubItem 先例；parseBinding 照改）。
4. **P fn 行 recv 空位**：astser 权威表 FnDecl 的 RECV 域在名与效果标签之间——普通 fn 的空 recv 拼写为名后**两空格**（一空格成名域界、一空格即空域）。首版漏此域，塔在 fn 语料上发散后对照 astser_test.go:185-207 补齐。
5. **#31 let-别名复发被塔在运行期逮住**：parseImport 的 `let pathCol = segCol`（裸局部 var）绑**存储**非快照——while 循环改 segCol 后 pathCol 跟随，输出 P import 的路径锚变成末段（甚而别名列），而 Go 锚**首段**。B3b 纪律（let-from-裸-var 用 var 快照）重申并绕行：锚位改从字段读直取（`let pathCol: Int64 = first.col`——字段读不别名）。此枚是 #31 在「值经三层调用后错位」形上的首例：构建绿、检查绿，仅对账塔的字节比较能逮。
6. **lexnum.quote 单 rune 面的误用**：B3b 的 `quote` 是 E0001 字节值消息的单 rune %q 面（首 rune 可打印时返回**整段**——故多 rune 可打印首字形碰巧全对）；解析器消息引整段词文本，首 rune 为 `"`/`\`（串、rune 逃逸词）时内容丢失。全量换 `quoteText`（B3b 已有的多 rune %q 面，31 处）；10 枚 literal 串/rune 逃逸语料由红转绿。
7. **记账**：棘轮 97 → **206**（t2: 56/56 + 10 枚消息面 + 词法面存量）；发散余量 955 = case 945 + stdlib 6 + wec 4（literal 与 lextest 族全绿）；stdlib 翻绿 1 枚（io.we——import/fn 密集即 T2 面）；`we check`/`we build` 皆 exit 0。

### 补记三（T3 期·自举内存面修复，2026-10-08）

T3 增量使塔中止于自举项 `wec:parse.we`（产物 OOM 死、runWec 首 Fatal——「一运行就退出」的直接机理）。根因不在 T3 新码，而在两处遗留的二次方串累积被源码增长推过内存上限：

1. **机理——bootstrap 运行时的字符串缓冲是永生的**（`wec/build/rt-str.c` 头注即权威：回收属 ADR-0003 门槛下的 B2 分配器工作）：`__we_str_concat` 每次新鲜 malloc、全程无 free；GC 域只覆盖 record/List 盒。任何 `out = out + 片段` 的逐单元累积因而是无界真泄漏，不可指望 GC 兜底。两处病灶：`lex.we` emit 的 per-token `outText` 追加（B3b T2 遗留）与 `parse.we` drive 的 per-line toks/docs 重拼（本 change T1 遗留，补记一第 6 条）。T3 使 parse.we 51,595B/6,902 词 → 119,947B/15,268 词（词元 2.3× ⇒ 二次方代价 ~5.4×），自举项由 T2 时的可存活（~GB 级泄漏）转为 OOM 死。
2. **偏差一——drive 以位置切分替代一切构建**：drive 逐行拼装的每个「行」本就是 `lexOut` 的逐字 byteSlice，且序列化行永非空（各行以其字母起头）⇒ tokens 节恰为〔tokens 头行换行后, docs 头行行首〕的字节区间、docs 节恰为其后区间。as-built 改为两次 `byteSlice` 直取（运行时 byteslice 返内指针共享源缓冲——零分配零拷贝），逐行扫描仅余「找 docs 头行」的线性走查。既定的「分块 + 归并」在此处失去存在理由：要归并的构件是原文的连续区间。
3. **偏差二——lex 侧以常量块两级折叠替代 List 平衡归并**：桶清单须住扫描器记录（跨方法调用的状态载体），而 **record 带 List 字段 = 停面 B**（D1 停面表 P3 族），方案原形在本构建不可表达。承重层是分块本身（它拆掉 S·T/2 的二次方），顶层归并形只差常数因子，故取最简可表达形：新字段 `outTail` 作开块，emit 行进 tail、过 8 KiB 折叠进 `outText`，render 末尾 `outText + outTail`；拼接顺序未动，输出字节恒等。docsText 侧不动：语料源 `///` 单元为零（实测 grep），该量级永不付一次折叠。
4. **验证（实跑）**：杀手项 `wec:parse.we` 修复后 exit 0、**峰值 RSS 150MB**（修复前 OOM 死）、输出形态正确（tokens 节 + `N WEC-PENDING stmt`——与参考的分歧属 T4+ 未落地面）；完整塔 PASS 245.9s、`reconciled 378/1305 ≥ 棘轮 206`（378 为 T3 面翻绿所致；本修复保字节恒等，不贡献对账数）；docs 节抽查（`literal:docs-run` 形）切分逐字节正确。此为塔自 T3 增量后首次完整跑通（此前 OOM 中止）。
5. **遗留披露——`charAt` 二次方（独立事项，建议登记 follow-up）**：`__we_str_charat` 从头走查 O(i)（rt-str.c），扫描器每 rune 经 codeHere/codeNext 多次走查 ⇒ 扫描 O(R²) 纯 CPU：parse.we 项单跑 3:20、塔 245.9s 中约 200s 耗于此。此缺先于本修复存在、与泄漏无关（该项 RSS 稳定 115MB、状态 R 可证）；`lexnum.byteAt` 为 O(1)（probeByteValue(byteSlice(i,i+1))），改预测宽推进即可线性化。次要累积点按「只修两处」约定不动（unesc 逐字节、astText 逐项、itemStartsText——量级皆有界，数十 MB 以下）。

### 补记四（T3 落地，2026-10-09）

1. **记账**：棘轮 206 → **381**；语料 as-built 1306（放行 1305，剔 1 仍为 #30）；t3: 语料 144/144 全绿；翻绿构成 = t3 144 + conformance case 30（纯声明形源 29 + 漏册同病 `check-e0404-variant-fn` 1）+ stdlib 1；发散余量 = case 915 + stdlib 5 + wec 4（四源自身皆含 fn 体语句 → `N WEC-PENDING stmt`，T4 面）；完整塔 PASS 236–248s（补记三-4 修复后首跑 245.9s，收口复跑 247.7s）。
2. **载体判决——interface 两缓冲分组 vs foreign 逐项追加**：Go 侧 `InterfaceDecl` 的关联型与方法签名是两个切片（astser 序列化恒先 assocs 后 methods，与源序解耦——`fn` 先于 `type` 的交错源在 P 行族里重排，`t3:iface-method-before-assoc` 钉住），流式载体取 assocsText/methodsText 两缓冲分组拼接；`ForeignBlock` 的 Items 是单一切片源序承载（serItem 循环）——逐项解析即逐项追加（append-as-parsed）。检错序两钉 = `err-iface-item-sep`（方法间行隔）与 `err-impl-assoc-after-method`（impl 内 assoc 须先于方法）。
3. **收口期塔逮真缺陷两枚**（红测试即语料本体，先修后绿）：① **sum 变体只 dupCheck 未入册**——`type S = A | A` 的第二枚查无不撞（参考 E0404@1:14）；Go 位点 = dupCheck/checkPascal 后即 `p.names[vt.Text] = vt.Line`（parser.go:1342，先于载荷解析），We 侧同位补 `registerRaw`。此枚同时拦着一枚 conformance 用例（`check-e0404-variant-fn`：变体 `Circle` 撞 fn `Circle`——fn 侧 dupCheck 找不到变体的册），修复后双面同绿。② **runeText 可打印 ASCII 表基差一**——表首字节 0x20（空格）而索引取 `r - 0x21`，`"\u{63}"` 渲染成 `"b"` ≠ `"c"` 误报 E1701；索引订正 `r - 0x20`（守卫 `r >= 0x21` 不动——0x20 自身走 byteChar）。
4. **闭集解码面**：E1701 的 ABI 字面量校验经 descText 拆码——`\u{}` 域检共享 rune/串两形（parseUEscape/hexDigitVal/validRune），码点经 runeText 算术渲染（可打印 ASCII 无探针表泄漏位——lexnum 只出探针表与控制表，故自带 0x20–0x7E run 表；表基事实见 3②）。
5. **where-bound 角锚**：`P named` 的 ArgLine/ArgCol 是泛型实参 `<` 位；where 约束的 iface 名参考不填（零值）——泛型应用形下仍拼 `0 0`。typeref 主面（参数位/字段位/返回位/载荷位）照填 anglePos。

### 补记五（T4 落地·上半刀，2026-10-09）

T4 的两枚复选框本就是两条垂直可验的验收线，按框拆两刀：上半刀收 block 与语句族（本补记），下半刀收 match/模式族与 test/mock。语句族独立成塔绿面已足（棘轮 +142）。

1. **记账**：棘轮 381 → **523**；语料 as-built 1344（放行 1344，剔 1 仍为 #30 角）；t4: 39/39 绿；翻绿 141 = t4 38 + stdlib 3（io/test/time——体内容即语句面）+ conformance case 100；发散余量 821 = case 815 + stdlib 2 + wec 4（类别：expr 689——T5 面、test 49——下半刀、pattern 2——下半刀、docs 1——T6 面，另 80 枚首差窗口未露 N 行，结构同类）；红先态记档 /tmp/t4-red.log（reconciled 382/1343、t4 37/38 红；`t4:err-break-top` 红态即绿——break 顶层走 parseItem 的 fallthrough，两态同形）。
2. **载体判决——else-折叠 P 行手术**：Go 的 else 块在 `len(Items)==1 && ExprStmt && If` 时折叠进 else-if 链。流式载体无树可查，改在组合后的 P 行上做同一判定：blockLoopText 增返项数，cnt==1 且剥首行 `P block` 后取前缀 `P exprstmt `、再剥后取前缀 `P if ` 即折叠——三段剥离与参考三重条件字面等价，无启发式。弃用的候选是括号平衡探针：if 条件含 `{`（构造字面量形）即破。BlockExpr 锚点取闭括号后的 cur（参考的结构体字面量求值序，非 else 关键字位）。
3. **afterPrimary 停点集**：参考的 parseBinary/parsePostfix 两循环头折为一门。停 = atEnd / depth==0 && brokeLine / `}`/`)`/`,` / 后缀 default 臂**含关键字**（故 `if c let x = 1` 由体块检查报 E0105 引 let，而非此处闩）。真面两枚：`[` 的 E0105 与 `=` 双语境（语句位 E0105 assignAfterExpr / 值位 E0103）——参考先于任何 T5 产生式触达，故归 T4 面；续面（21 操作符 + `.`/`(`/`?`/`<` 与构造 `{`）闩 T5。
4. **消息拼写对账纪律**：携带稿把 E0103 与 assignAfterExpr-E0105 拼作连字符，复读 parser.go 逮出——两消息皆 em dash `—` 加裸引号 `"="`。塔只钉语料命中的面；未命中拼写（for 头 eof/泛型、scope 头诸形、`[`、E0103）无塔防，靠对参考逐条人工复核，本条记录该纪律。
5. **exprValue 场域清单**（Go exprCtx 0/1 的 We 镜像；唯一读点 = ifExprText 的 E0202）：置 1 于赋值双头的值、return 值、scope 绑定值、binding 初值、headExprText、else-if 链递归；置 0 于 exprstmt。valSite 缺省位 "this value position"（parsePrimary 的 valueless 关键字臂用裸 valSite——T5/下半刀面，此处不碰）。
6. **fn 上下文栈复用**：fctxLine 参数承两类上下文——fn 体 `name;hasRet;0` 与 defer 体空名行 `;0;1`；fnsTopFlag/fnsTopName 供 return 双查。mock 深度律内嵌 blockLoopText（savedTest 1→2 else→0），深度 2 正形待下半刀的 test 面。
7. **22 词保留闭集**逐词对账 parser.go:2158-2161；tokAhead 只读探针不推游标——#31 纪律的读侧延伸（探针走 toksText 的 T 行，不触碰 last/cur 的行位事实）。
8. **自举性能面复钉（非回归，在案未决）**：塔 330.7s（T3 收口 247.7s）。增量主项 = wec:parse.we 自源条目：源码 T4 期间 51.6KB → 151.3KB，在补记三-5 的 charAt O(R²) 扫描面上自然放大；本窗口前缀计时（600/1200/2400 行 = 4.5s/23.4s/>60s 超时）确认超线性仍在。该条目闩点在词法完成后的极早处（`Token { … }` 构造面 → N 行），慢全在词法扫描，与 T4-A 码无涉。follow-up 裁定仍待用户（补记三-5 同案——charAt 线性化即消）。

### 补记六（T4 落地·下半刀，2026-10-09）

1. **记账**：棘轮 523 → **600**；语料 as-built 1389（放行 1388，剔 1 仍 #30）；t4b: 44/44 绿（42 原始 + 2 收口期补钉）；翻绿 77 = t4b 44 + conformance case 33；发散余量 788 = case 780 + stdlib 4 + wec 4，latch 种类恰两类 = expr 787（T5 面）+ docs 1（T6 面）；红先态记档 /tmp/t4b-red.log（reconciled 523/1386、t4b 42/42 红）；塔两跑 490.9s/495.6s（T4-A 330.7s——parse.we 151.3KB→179.6KB 在补记三-5 charAt O(R²) 面上的放大，同案待裁定）。
2. **补记五-1 分类账订正**：T4-A 收口发散余量 821 的构成实为 **case 813 + stdlib 4 + wec 4**（原记 815/2/4——总数对、case/stdlib 拆分错）。本轮以两轮日志逐枚核对 stdlib 名单与首差字节（collections/fs/process/string 四枚全同）证无回归后订正。
3. **载体判决——binds 标记方案**：模式函数返 `(text, binds, l, c)`，binds = 走查序 `name l c\n` 行族；**or 祖先下的行加 `!` 前缀**——checkDupBindsOf 跳过标记行（Go checkDupBinds 对 PatOr 整树跳过的镜像）、namesAllOf 剥一重取名录（Go patNames 递归进 or 分支的镜像）；or 各分支 binds 以 `;;\n` 分段（segs）承载**两遍名集律**：先逐分支查重、后 or 名集律（E0302 锚 or 首 `|` 位）——次序保 Go 的先错先报。**markBinds 幂等**：嵌套 or（分支原子经元组元素/变体载荷含 or）已带标记的行不得再标，否则 or 名集律剥一重后取到 `!n` 污染 E0302 消息——语料 `t4b:err-pat-or-nested-mark` 钉（无幂等侧拼 `("!n" and "q")`，Go 拼 `("n" and "q")`；内层 or 各自名集合法、外层违反，错误才达外层）。
4. **判据修复两枚（自查逮出，语料补钉）**：① **namesLineOf 缺席哨兵 0 → −1**（三调用者 `!= 0`→`>= 0`）：Go dupCheck 按 map 存在性判重，而顶层元组 let 以合成 token `{Text:"", Line:0, Col:0}` declare——空名登记在行 0，`!= 0` 判据把「在场于行 0」误判缺席，第二枚顶层元组 let 该发的 `E0404 @ 0:0`（消息引 "at line 0"）被漏；另两调用者（scope 头 seen、成员名册）条目行恒 ≥1，判据换对其等价。语料 `t4b:err-topletpat-dup` 钉。② **parseBinding `(` 臂平铺病灶**：Go switch 的 `case "("` 成功后直落 `:`/`=` 检查、default 只兜非 ident/_/`(` 词；We 侧嵌套 if 平铺把 eof/default 兜底留在 `(` 臂同层——元组 let 五枚正例全在 `(` 位误报 E0105（`we check`/`we build` 不逮：wec 自源无元组 let，塔是唯一差分面）。首绿跑（594）逮出，修后复跑（600）。
5. **语料收口期改拼两枚（披露）**：`t4b:match-variant-qual`（`std.db.Q(n)`→`std.Q(n)`）与 `t4b:test-mock-full`（`mock std.io greet`→`mock io greet`）——Go 限定形只取单段（qual 臂核对一个 `.` 后的 ident），原拼法死在中段 ident（仍绿但探针比意图浅）。改拼后未重取红跑；先红论证 = latch 在关键词位早于臂内容（match→`N WEC-PENDING expr`、test→`N WEC-PENDING test`，红态日志在案）不受拼法影响。补钉两枚（err-topletpat-dup / err-pat-or-nested-mark）同论证（元组→`WEC-PENDING pattern`）。
6. **塔侧 E1801 披露**：塔的固定输入名 wec-input.we 恒非 `_test.we` 名尾 ⇒ 绿 test 块两侧同报 E1801——**P test/mock 行族本体在塔内不可观测**（块内诊断先于模块检查闩定故仍可观测）；P 行拼写以 astser 权威表人工逐字段钉（serTest/serMock/serPat 族）。语料注释块同披露。
7. **教训（在案）**：读中途塔日志只看尾部会漏头部信号——t4b 段先于 stdlib 段跑，中途 40 行日志的头部已含 6 枝 t4b 发散而尾部只见 stdlib/case；中途判读必须头尾皆看。
