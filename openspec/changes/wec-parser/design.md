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
