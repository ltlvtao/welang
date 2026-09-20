# design — wec-lexer（B3b）

> 权威：proposal.md（黑盒范围）。锚：Go 侧 `internal/lex/lex.go`（HEAD `1a7e81a`）；We 侧语言面 = 已批规范 + B2a/B2a' 译文 stdlib。本设计给唯一最小路径；每条裁决给被拒方案。

## D1 状态载体与首错停改形

Go 的 `(l *lexer)` 方法集（`lex.go:123-133`：name/src/off/line/col + docs/docOpen/docLast/keep）对位 We record + 固有 impl：

```we
record Lexer { src: String, n: Int64, off: Int64, line: Int64, col: Int64,
               toks: List<Token>, docs: List<DocUnit>, docOpen: Int64, ... }
```

- **字段突变通道**：E0105 只认方法接收者的 `self.field`（`check-e0105-field-assign` 黄金钉），故词法器操作全部是 `impl Lexer` 内 `pub fn xxx(mut self, ...) -> ...` 方法——`mut self` 突变跨调用持久（`run-for-user-iterable` 等 10 黄金实证）。`scanAll` 的循环体即「取 rune → 分类 → 调方法」，与 Go 逐方法一一同构。
- **首错停**：Go `panic(stop{d})`（`:197-203`）在 We 无对位（无 recover）。改形为**错误闩**：record 增 `failed: Bool` + `errLine/errCol/errCode/errMsg` 四字段；`fail` 方法置闩；`scanAll` 循环顶与每个可能失败的子扫描返回点查闩，闩置即逐层退出、token 流弃置（对位 Go 的 `toks = nil`）。语义等价：ch1 的首错停保证不产任何 token——闩在**产生下一个 token 前**阻断即可，不需要非局部跳转。
- **Token/DocUnit**：`record Token { kind: String, text: String, line: Int64, col: Int64, attrName: String, attrArgs: String }`（attr 外的词 attrName/attrArgs 置空串）；`record DocUnit { startLine: Int64, endLine: Int64, lines: List<String> }`（`List<String>` 载体 B1b 已批）。累积用 `List<Token>`/`List<DocUnit>` 的 push。
- **keep 开关不移植**：Scan 面注释分支只跳过（`keep=false` 路）；Keep 是 B3f 的事——wec 的 impl 里没有 keep 字段。

## D2 rune 走查 + 预测宽的字节层映射（勘查事实 2/3/4 的兑现）

| Go 操作 | We 对位 | 依据 |
| --- | --- | --- |
| `l.src[l.off]`（分类读） | `self.src.charAt(ri)` 得 Rune，ASCII 直接比较（Rune 有 `==`） | col 是 rune 列（`lex.go:211-253`），扫描核分支全 ASCII（勘查事实 5） |
| `advance()`（rune 推进 + 行列维护） | `let r = self.src.charAt(ri); off = off + widthOf(r); ri = ri + 1; <行列维护同 Go>` | `widthOf(r)`：码点预测宽（<0x80→1、<0x800→2、<0x10000→3、否则 4）；checkEncoding 已过后预测宽 = 实际宽 |
| `l.src[start:l.off]`（词文本） | `self.src.byteSlice(start, end)` | 原样字节视图（`str.c:129-137` 内部指针），含非 ASCII 词文本逐字节保真 |
| `posAt(off)`（字节偏移→行列，错误路径） | 对 `byteSlice(0, off)` 按 rune 走查计行列 | 前缀切片走查 ≡ Go `range`（病形 FFFD 宽 1 语义两侧一致——勘查事实 2），**含病形前缀亦精确** |
| `len(l.src)` | `self.src.byteLength()` | |
| `peek2()` | `ri+1 < runeCount` 时 `charAt(ri+1)`，否则哨兵 | 全 ASCII 使用点（`/` `/` 等），哨兵取 0 |

- **为什么走 rune 不走字节**：字节值读取面只有 `byteSlice`（构造切片）无取值面（勘查事实 4）；而 Go 的列本就是 rune 列、分类分支本就全 ASCII——rune 走查不是妥协，是语义对位。字节偏移 off 由预测宽维护，供 `byteSlice` 取文本与 offset 进消息。
- **`checkEncoding` 三面**：
  1. 有效 UTF-8 = **Σ 预测宽 == byteLength**（等价性：每病形字节实际推进 1 而其 FFFD 预测宽 3，两侧只在全良构时相等——勘查事实 3；含截尾/过长/代理/超域全部病形类，`utf8_step` 的 else/短尾/域检三分支一一对应）。
  2. 首病字节定位：走查中首个 `charAt(ri) == 0xFFFD` 且探针 `t = byteSlice(off, min(off+3, n))` 非（`t.runeCount() == 1 && t.byteLength() == 3`）者——通过探针的 FFFD 必是良构 EF BF BD（否则矛盾于 charAt 的实际解码）。行列为 `posAt(off)`。
  3. 字节值进消息（`byte 0x%02X`）：**等值探针查定**——可名字节 243 值由三段探针字面量覆盖（`"\u{80}…\u{7FF}"` 给 C2–DF 与 80–BF；`"\u{800}\u{1000}…\u{F000}"` 给 E0–EF；`"\u{10000}…\u{100000}"` 给 F0–F4；ASCII 直书），对 `byteSlice(off, off+1)` 逐候选位等值扫描，命中偏移算术反推值（codepoint 索引 K：偶位 = 0xC0 + cp/64 型、奇位 = 0x80|(cp mod 64) 型）。**C0、C1、F5–FF 13 值不可名**（`ValidRune` 拒绝构造，勘查事实 4）——查定不中即**响亮失败**（任务失败而非错消息），对账塔语料门槛保证不达（proposal 非目标）。
  4. BOM 面：前导 FEFF 跳过（ri/off 各推进其宽）；走查中再见 FEFF（ri>0 或剥离后）即报 `byte-order mark at offset %d`——offset = 当前预测 off（此时已过 UTF-8 面，预测 = 实际）。
  5. 裸 CR 面：走查中 `charAt(ri) == '\r'` 且（ri 是尾或 `charAt(ri+1) != '\n'`）即报 `bare carriage return at offset %d`。三面次序照 Go（UTF-8 → BOM → CR，`:259-296`）。
- **剥离 BOM 的落法**：Go 改 `l.src` 为剥离后串；We 侧 String 不可变——记 `base: Int64`（剥离字节数）或直接把 ri/off 起点 +3、`byteSlice` 一律加 base？**取后者之简**：checkEncoding 成功后构造 `self.src = self.src.byteSlice(bomWidth, n)`（一次切片，后续无 base 账）。

## D3 消息与转义纪律

- **verbatim 模板**：E0001–E0009 全部消息串从 `lex.go` 逐字转写（含 em-dash 与引号字形）；`fmt.Sprintf` 的参数点（offset、byte 0x%02X）以 We 串拼接 + `int64ToString`（手写十进制，stdlib 无 `toString` 面——B1a 值塔已有 `__we_str_of_i64` 但那是运行时内部；We 侧手写十余行）。
- **hex %02X**：nibble 表 `"0123456789ABCDEF"` 手写（B3a 勘查同结论：不进 std）。
- **quote() 有界偏差（披露）**：Go 词法器错误消息对词文本的引用（如 attr 名）用 `strconv.Quote`——非 ASCII 不可打印字符转 `\uXXXX`。We 侧 quote() 对 ≥0x80 字节原样保留。**语料不可达**（语料中错误消息引用的文本皆 ASCII），达到即逐字节对账响亮失败——非静默偏差。根治面是 #30（字节暴露）同源。
- **序列化转义**（D4 规范的一部分）：`esc(bytes)`——0x5C（`\`）→ `\\`；0x21–0x7E 原样；其余（含空格 0x20、控制、≥0x80）→ `\` + 两位大写 hex。机械可逆，两侧同实现。

## D4 驱动契约与序列化规范（无 argv 面的诚实解）

- **固定路径契约**：wec 二进制读其 **cwd** 下 `wec-input.we`（`fs.readFile`），产 stdout。无 argv/stdin 面（勘查事实 7）是语言面事实，契约不发明面。B3f 的 CLI 面落 argv 时（follow-up #29），驱动改 argv 形、契约退役——塔的比对逻辑不动。
- **读文件失败**：`Err(FsError)` → stdout 单行 `E 1 1 E0001 cannot read wec-input.we`（Go 参考侧塔以同输入复现同失败——语料不含此形，此分支为防御）。
- **序列化规范**（两栏：成功/失败）：
  ```
  tokens                        ← 头行
  T <line> <col> <kind> <esc(text)>[ <esc(attrName)> <esc(attrArgs)>]
  docs                          ← docs 段头（有单元才发）
  D <start> <end>
  L <esc(line)>
  ```
  失败：**单行** `E <line> <col> <code> <esc(msg)>`，无 tokens 头（对位首错停清流）。退出码恒 0。诊断的 name 字段不序列化（proposal 非目标）；help 不序列化（注册表静态属性）。
- **序列化器双实现**：We 侧在 `main.we`；Go 侧在 `internal/wec`（对 `internal/lex` 的产物活算）——同一规范文本（本节）的两栏实现，规范本身住本 design，后续段（B3c+）沿袭扩栏（Parser 行类）。

## D5 Go 侧对账塔（`internal/wec`）

- **构建一次**：`t.TempDir` 拷 `wec/` → `cli.Run([]string{"build", tmp}, …)`（`cli.go:88` in-process）→ `build/wec` 工件。conformance-runner 式 `exec.Command(dir)`。
- **逐语料**：条目源写 `tmp/wec-input.we` → `exec`（`Dir=tmp`）收 stdout → 参考侧 `lex.Scan("wec-input.we", src)` + 序列化器 → `bytes.Equal`。语料清单由提取器生成（stdlib 源直读；conformance 案例从 case JSON 的 `setup.files` 提取每个 .we——**只读**；lex 测试样例与错误矩阵为塔内字面量）。
- **阶段门槛**：T2/T3/T4 跑语料子集——子集由**参考侧活算的 kind 集**围栏（如 T2 = 参考侧 token kind ⊆ {ident, keyword, 各运算符, eof} 的条目），非手写清单；T5 全量。
- **包数 16 → 17**；测试命名沿仓例（`wec_test.go`）。

## D6 wec 模块切分与关键字表

- `src/main.we`：驱动（读 `wec-input.we` → 建 Lexer → 扫 → 序列化到 stdout）+ 序列化器 + quote/hex/int64ToString/esc 工具。
- `src/lex.we`：Lexer record + impl（checkEncoding、scanAll、空白/注释、ident/keyword、运算符、字符串/逃逸、rune、attr、洞记录、docs 侧通道、fail/闩）。
- `src/lexnum.we`：数值文法（numericSpan 七条款、后缀集、`1e5` 切形）+ `widthOf`/字节值查定探针（皆纯函数，独立可测）。
- 模块路径 `main`/`lex`/`lexnum` 合规 `[a-z][a-z0-9]`（ch15）；`import lex` 本地导入。
- **关键字表 = String 列表字面 + 线性扫**（~45 词，每词一次 `==`）：顶层 `let` 绑定构造 `Map` 走泛型构造器（B2b E1804 类目风险），列表字面零机制风险；词频下线性扫成本可忽略（正确性先写）。

## D7 验证策略

- **词法逐字节对账**（本段差分杠）：全语料 `bytes.Equal`（成功/失败两栏皆比）。
- **语料构成**（proposal 目标 4 全列）：① stdlib/src 全部 .we（7 文件）；② wec/src 三模块自源（自举一致性最早信号）；③ Go lex 测试语料逐字面提取（含 CRLF/BOM/CR/非法 UTF-8 正负样例；13 值角 `"\xff"` 一枚剔除披露）；④ E0001–E0009 全码矩阵（每码至少一形）；⑤ 933 conformance 案例 setup .we 逐文件；⑥ 名可达非法 UTF-8 五形（杂散 C3 28/截尾 E2 82/代理 ED A0 80/超域 F4 90 80 80/过长 E0 80 80）+ BOM 中置 + 裸 CR；⑦ 非 ASCII 字面项（注释/字符串内的名可达字节，使宽表突变可杀）。
- **突变电池**（T5）：wec We 源单点突变 ≥6 枚——M1 宽表界（widthOf 改一档）、M2 关键字表删一词、M3 列推进差一、M4 E0006 条款界（numericSpan 一条款翻转）、M5 逃逸域检删（\u 上限/代理排除去一）、M6 序列化转义支路改。逐枚：单测/塔层击杀判决表（B2b 形制）。
- **零漂移**：conformance 933 `-count=1` 零改写全绿（零 Go 生产码改动的结构性论证 + 全量复验）。
- **红先纪律**：T1 塔先落（wec 缺席 → 构建失败红态记档）→ 骨架绿一枚简单语料；T2–T4 各段的语料子集在对应面落成前红（token kind/文本不匹配）记档后翻绿。

## 被拒方案

| 方案 | 拒因 |
| --- |
| argv/stdin 驱动面 | 语言面不存在（ch15/ch21 无）；规范先行纪律禁发明——固定路径契约 + follow-up #29 |
| 字节暴露语言面（`byteAt(i)->Int64` 之类） | 同上；13 值角以语料门槛 + 披露处理，根治归 #30 |
| 探针辅助文件（塔为 wec 置一字节表文件供其读取） | 驱动契约污染——为绕语言面缺口让编译器依赖辅助数据文件，B3f 三段闭环须随身携带；诚实边界先例（lsp exit 70、deps 诚实停）优于机制包袱 |
| 顶层 `Map` 关键字表 | 泛型构造器 E1804 类目风险（B2b 教训）+ 机制风险零收益——列表字面 + 线性扫 |
| panic/recover 对位（We 加 recover） | 语言面增量，本变更零语言面纪律直接拒 |
| 单函数巨体扫描（无 record 方法，全局部变量） | E0105 禁普通字段赋值不是禁 record——方法集与 Go 同构、可测性/可读性皆优；巨体函数无先例 |
| 手写期望字节（语料的期望输出人工写） | 参考侧活算是对账杠的意义——手写期望把「移植正确」降为「转写一致」；零手写期望为硬纪律 |
| ScanAt/Holes/Keep 一并移植 | 消费者锚（proposal 今日事实 2）：ScanAt/Holes 是 B3c 地基、Keep 是 B3f 脸——未被消费的移植是死代码，仓纪律拒 |
| quote() 全保真复刻 strconv.Quote | 根治需字节暴露面（#30 同源）；语料不可达的偏差以响亮失败披露优于为不可达角扩语言面 |

## 实现期补记

（随任务落地追加。）

## 实现期补记一（T1，2026-09-17）

**两复选框全勾。红先记录 = 塔先落而 wec/ 缺席——`TestLexReconciliation` 红于 `copy wec: lstat ../../wec: no such file or directory`（构建失败形，任务书预期态）；骨架初版再红一次于 E0501（`mixed types — the final expression is (), the declared return is Result<(), AppError>`——match 尾表达式是 unit），按黄金形（`run-map-gc-crossing` 等：match 语句 + 尾 `return Ok(())`）重排后绿；终态 `reconciled 2 corpus entries`。**

1. **首枚绿语料 as-built 订正（任务书写「`let x = 1` 形」被订正为空源形）**：占位序列化器只答空源（`tokens` 头 + `T 1 1 eof ` 两行——恰为格式锚的形状），`let x = 1` 需 int kind（T3）。首绿条目实际两条：`literal:empty` 与 `case:fmt-empty:src/empty.we`（conformance 的 fmt-empty 案例带一枚空 .we 源——围栏恰好放进）。`let x = 1` 以 `literal:let-one` **即刻入语料**、被 T1 围栏滤除，T3 int kind 落地时自动翻绿——不是从语料缺席。
2. **阶段门槛 as-built 扩形（D5 只写「kind 集围栏」）**：错误条目首错停清流、无 kind 集可言——围栏落成双臂：kind 集（+`operators` 旗：运算符 kind = 自身文本非闭合集，以 `namedKinds` 判据整体放行）与**错误码集** `stage.errs`（T1 空集 = 全部错误条目暂缓；T2 起按码放行）。另加 `ran == 0` 响亮失败断言——围栏空转（集合配错）是任务失败非静默通过。
3. **语料②（wec 自源）随 T1 即入**：D7 列语料七项，T1 提取器落四路——字面量（empty/let-one 两枚）+ stdlib 全源 + wec 自源 + conformance setup .we；③lex 样例④错误矩阵⑥⑦编码与非 ASCII 项随 T2–T4 的面落。as-built 计数 = **990 条**（字面 2 + stdlib 7 + wec 3 + conformance 978——933 案例中多源案例与 tests/ 目录像各计其 .we 文件）。
4. **`we check wec/` 的证据形态**：塔的构建本身即全管线（干净 `build` 含检查与代码生成），另按任务书验证行以 CLI 字面复跑（`we check wec/` exit 0）——两侧皆零诊断。
5. **骨架形**：`we.toml` = `name = "wec"` / `version = "0.1.0"` / `type = "executable"`（benchmarks 参照形逐字段）；`src/main.we` 驱动 = `fs.readFile("wec-input.we")` 的 `Ok(_)`/`Err(_)` 双臂（通配黄金面）+ 占位两 println + 尾 `return Ok(())`——Err 臂 `E 1 1 E0001 cannot read wec-input.we` 是防御行（语料不含读失败形，D4）；`src/lex.we`/`src/lexnum.we` 各一枚占位 pub fn（模块链与导入图在场，真体落地时删）。本地导入 `import lex`/`import lexnum` 裸形（`build-bnd-multi-module` 黄金形）。
6. **塔结构细节**：拷贝以 `filepath.WalkDir` 全树（含 build/ 目录被拷入亦无害——目标目录全新）；比对按任务书字面 `bytes.Equal`；`io.println` 自加换行与序列化行尾 `\n` 逐字节对齐（空源输出 = `tokens\nT 1 1 eof \n` 10 字节）。
7. **计数**：包数 16→17（`internal/wec`，15 ok + 2 no-test 先例）；conformance 933 不动；`go build`/`go vet`/`gofmt -l`（refr/ 外零文件）全绿；终树全量 `go test ./... -count=1` 17 包 0 FAIL 复跑在案。

## 实现期补记二（T2，2026-09-18）

**两复选框全勾。红先记录在案：占位骨架时点 `reconciled 261 corpus entries` + 259 枚「output diverges」（唯二绿 = 两条空源）。真源落地后 `reconciled 252 corpus entries` 全绿——261→252 分解 = −7 attr 面 E0001（第 7 条）−2 wec 自源（重写后携带数值/字符串字面，属 T3/T4 面，T5 全量门槛摘除时回归；占位 lex/lexnum 原为纯 keyword/ident 形故红态时被放行）。**

**编写期先落一轮语言面探针（约二十枚微程序，/tmp 不入册）——停面地图推翻 D1/D6 的三处载体设计，订正如下：**

1. **停面地图（探针实证，全部绕行、零扩语言面）**：① record 构造的 List 型字段停（fn 体/main 体、字面/变量/调用实参全形）；② var 绑定 List 局部停（`let` 绑定含空表字面反而绿）；③ `Option<String>` 载荷 match 停（B1b docs「String 载荷」停面同族——`Option` 亦无 `default` 成员 E0816）；④ 跨模块 record 的字段访问停（参数位/同模块全绿）；⑤ main 体 `for` 停（M9b 语句集无 for）；⑥ `else if` 链缺尾 `else` 报 E0202（脱糖 `else { if … }` 块尾为无值 if——scanWord 拆两个独立 if）。绿面另证：全标量 record（19 字段实证）构造与 impl 读写、方法返回 record/tuple、方法调方法、跨模块纯函数（fn 体/方法体皆可）、41 项等值 `||` 链、嵌套 while、`while true`+break、void 方法内裸 `return`、Bool 字段裸条件、String 字段拼接累积。
2. **D1 订正——去 List 化**：`toks: List<Token>` 与 `docs: List<DocUnit>` 均不可构造（停面①）。token 流改**边扫边拼**：scanAll 每产一词即 `emit` 拼一行 `T …` 进 `outText: String` 字段；docs 侧通道改 `pendBody/docsText: String` 累积（flushDoc 拼 `D s e\n` + 已 esc 的 `L` 行）+ `docsCount: Int64` 计数替代 `len(docs)>0`。`Token` record 保留为扫描方法 → emit 的返回载体（6 标量字段，可构造）。可观测契约不变：闩置时 `outText` 整体弃置（对位 Go 清流）。
3. **D6 订正——序列化器住 lex.we**：跨模块 record 字段访问停（停面④）使 main 不能读 Token/Lexer——`render(l: Lexer) -> String` 落 `lex.we` 同模块（参数位读字段 = G2 绿面），`pub fn run(src: String) -> String` 出整份序列化；main 退回骨架形（`io.print(lex.run(src))`，`import lexnum` 随之从 main 退役）。lexnum 现驻纯工具：`widthOf`/`int64ToString`/`hex2`（−1→`"??"` 响亮）/`hex2Lower`/`esc`/`quote`/`probeByteValue`（D6 原文「序列化器 + 工具住 main.we」订正）。
4. **D6 订正——三张表皆等值链**：`List<String>` 关键字表被停面①③排除——41 词关键字、11 词 twoCharOps（maximal munch 先试）、24 码 oneCharOps 全落单表达式 `||` 等值链（零机制风险，词频下成本可忽略）。
5. **checkEncoding as-built**：三面次序照 Go（UTF-8 → BOM → CR，strip 收尾）。有效 UTF-8 判定 = **FFFD 三字节探针单趟走查**（`charAt(ri)` 得 FFFD 时，`byteSlice(off, off+3).runeCount() == 1` ⟺ 良构 EF BF BD 宽 3，否则 off 即首病字节）——任务书「Σ 预测宽 == byteLength」形被此单趟包含（每病形 FFFD 宽 1 语义两侧一致，勘查事实 2），不再单独落。BOM 面 offset = stripped 内字节偏移 + adjust；CR 面 **quirk 保真**：消息 offset 用 stripped 索引、`posAt` 走原串（Go 原样，`cr-after-bom` 语料钉）。字节值进消息 = `probeByteValue` 查定（243 可名值四探针串，ASCII 直读；不中 → `hex2(-1)` = `"??"` 响亮失败——围栏保证不可达）。
6. **quote() 有界偏差细化（D3）**：语料可达两形保真——ASCII 缺席字符的 `%q` 形（标准逃逸 + 可打印原样 + 控制字符 `\xNN` 小写十六进制，Go strconv.Quote 同形）与非 ASCII 字母原样（Go `%q` 对可打印 rune 亦原样）。`unicode.IsLetter` 不可表达 → ≥0x80 全走字母形消息（语料可达形恰为同形；非字母非 ASCII 第三形不可达，diverge 即响亮）。
7. **围栏第三特判（as-built）**：attr 面的形违规报 **E0001**（`"#"`-not-followed 与 unit-shape 两形，消息皆含 "attribute unit"）——码集判据放不出去，围栏加消息判据（参考侧活算，与 UTF-8 不可名判据同款）；T4 落地时随 attr 面拓宽移除，7 枚条目回归。`"\xff"` 13 值角剔除披露与 `reconciled` 计数日志不变。
8. **计数**：`reconciled 252`（stdlib 7 + conformance 242 + 字面量编码/docs/E0004 形 + lextest 子集）；corpus 1039 不变（字面 15）；17 包全绿（`go test ./... -count=1` 0 FAIL）；conformance 933 不动（零 Go 生产码改动——wec_test.go 测试码 + wec/ We 源）；`we check wec/` exit 0。

## 实现期补记三（T3，2026-09-18）

**两复选框全勾。红先记录在案：围栏拓宽（kinds +int/float/rune、errs +E0006/E0003）+ 语料 34 枚新字面量后 `reconciled 533 corpus entries` + 281 枚「output diverges」（全为 WEC-PENDING number/rune 形；252 旧绿零回归）。真源落地后 `reconciled 531 corpus entries` 全绿——533→531 = 两条 T4 骑码围栏收紧（第 1 条）而非实现修正。**

1. **围栏 as-built 第四/五特判**：T4 面可骑 T3 的码进场——①lextest `'\n x'` 的参考侧首错是 **E0003**（逃逸后多字符触发形状款）而非 E0005，成功路径的 rune 文本判据管不到错误条目 ⇒ 错误臂加「E0003 且源含反斜杠」排除；②lextest `#[cfg(1__0)]` 的参考侧首错是 **E0006**（attr 实参内的坏数字先于 attr 校验死亡）⇒ 错误臂加「源含 `#[`」排除（保守：死于 `#[` 之前的合法 E0006 条目会被一并暂缓，语料中无此形）。两判据与既有三判据同款——参考侧/源活算、随 T5 围栏整体退役。
2. **语料 as-built**：数值面 28 枚（七条款每条款一正一负 + based 三形正例 + 前导零 `0`/`00` 边 + 后缀闭集四负 + `1e5` 切形 + `1..5`/`0x1..2` 的 `..` 不入 span 边界）；rune 面 6 枚（`'a'`/`'中'` 正 + 空引/多字符/EOF 未闭/换行未闭四负）。corpus 1039→**1073**。T1 预言兑现：`literal:let-one` 随 int kind 放行自动翻绿。
3. **语言面两枚新教训（补记二停面地图的延伸）**：①E0202 复发于 lexnum 两处「bad」链——else-if 链缺尾 else 即报，补尾 `else { bad = false }` 成固定写法；②`charAt` 回 **Rune** 非 String（E0501 argument/parameter）——quoteText 改 off/ri 双走查以 `byteSlice(off, off+w)` 取单 rune 体，quote 拆 `quoteBody` 供两者复用。
4. **scanNumber as-built**：numericSpan + 三验证器逐条款移植（checkDigitRun/validateBased/validateFloat，八后缀序表 `intSuffixAt`）；span 全 ASCII ⇒ `probeByteValue` 的 `byteAt` 直读；尾缀三分类（骑行/闭集违规/切形 `consume = i`）；跳进维护 off/ri/col 三者（ri += consume——Go 无此量，端口锁步）。`%q` 多 rune 面落 `quoteText`（span 全可打印 ASCII ⇒ 原样；逐 rune 走 quoteBody，diverge 即响亮）。
5. **rune 面 as-built**：consumeRuneLit 每处 fail 后**早退**（Go fail = panic，后续语句不执行——端口保真）；逃逸半面 = `pendingFail("rune escape")`，`'\u{41}'` 等留 T4（任务书原案）。
6. **wec 自源回归时点订正**：T2 记「−2 wec 自源待 T3/T5 回归」——T3 后仍栏外：lex/lexnum/main 三源均含**字符串**字面（kind 面 T4 才开），数值字面已非障碍。订正为「待 T4/T5」。
7. **计数**：`reconciled 531`（红态 533 − 2 围栏收紧）；corpus 1073（stdlib 7/wec 3/lextest 36/conformance 978 + 字面 49）；17 包全绿（conformance 933 不动，零 Go 生产码改动）；`we check wec/` exit 0。

## 实现期补记四（T4，2026-09-18）

**两复选框全勾。红先记录在案：语料 +32 字面量（逃逸/洞/attr/非 ASCII 矩阵 + rune 逃逸两枚）+ 围栏拓宽（kinds +string/attr、errs +E0002/E0005/E0007/E0008/E0009）+ 四判据摘除后 `reconciled 1104 corpus entries` + 573 枚「output diverges」（538 string literal / 30 attribute / 5 rune escape 三族 WEC-PENDING；531 旧绿零回归）。真源落地后 `reconciled 1104 corpus entries` 全绿——corpus 1105 唯一栏外 = lextest `"\xff"` 13 值角披露。**

1. **【缺陷发现·follow-up #31，T6 随 #29/#30 登记】We 编译器 let-别名错编译**：`let x = <局部 var>` 绑定的是 var 的**存储**而非快照值——let 读处取 var 当前值。探针族（/tmp，t4probe 项目）：`let snap = i; i = i+1; return snap` 回当前值（**snapLet BUG**）；同形 `var snap: Int64 = i` 拷贝正确（**snapVar OK**）；循环外同中（**snapLetNoLoop BUG**）；快照源与被变 var 不同仍中（**snapLetTwoVars BUG**）；String 同中（**snapStr BUG**）。record 字段读取（`let ln = self.line`）与调用结果（`let c = self.codeHere()`）不受累——lex.we 全库其余 let 皆此两类，故 corpus 1104 绿未受染。wec 绕行 = `validateAttrArgs` 的 `argStart` 改 `var` 快照（唯一 let-from-local-var 点位，注释指明 #31）。修复属编译器变更、须带黄金锚（check 层可钉），不入 B3b。
2. **任务书 E0009 例形 as-built 订正**：`#[x = f()]` 实死于 **E0001 form**（名后 `=` 非 `(` 非 `]`）——真 E0009 形是 `#[x(y)]`（裸引用）与 `#[limit(5000 + n)]`（conformance check-e0009 案例源：数字实参解析成功后随空白与 `+` 非逗号 → E0009 于实参起点 `"5000 + n"`）。语料矩阵按真实形落；此例也是 #31 的唯一在塔可观测形（argStart 位差 → 列差 5）。
3. **围栏终态（除 13 值角外全量）**：kinds = 全 named kind 除 comment（塔走 `lex.Scan` 永不产 comment 词）；errs = E0001–E0009 全码集。T2 的 attr-unit 消息判据、T3 的 src-含-`#[` 与 E0003-src-反斜杠与 rune-文本-反斜杠四判据随面落地摘除——`#[cfg(1__0)]`（实参内 E0006）与 lextest `'\n x'`（E0003）两 T3 骑码形随围栏拓宽自动放行翻绿。
4. **wec 自源回归兑现（订正 T2/T3 的「待 T4/T5」）**：string kind 开闸即回——lex/lexnum/main 三源自 T2 红态起栏外，本窗随 538 族一并翻绿；自举一致性信号（语料②）全量在场。
5. **E0007 消息的自陷规避**：消息文本含字面 `${`——We 字符串字面内 `${` 恒开洞，直接书写会让 wec 编译自己的 lex.we 时死于自家 E0007；拆串拼接（`"…the $" + "{ hole…"`）绕行（拼接在运行时、词法扫描逐段看）。
6. **consumeAttrArgs 的括号栈 = String**：`stack = ")"` 起步、push 拼接、pop 切尾、top 取尾字节比对（List 载体停面之外的零机制选择；`([)]` 交错形保真）。
7. **次序与位置保真点**：attr 序 form（E0001，名前后两形 + `#` 非跟 `[`）→ literal-only（E0009，failAt 于实参原始偏移 base+argStart）→ whitelist（E0008，空集 ⇒ 凡合法 unit 必 E0008 于 unit 起点；emit 的 attr 三字段臂因此在本 build 不可达，按 D4 规范落形）；实参内坏数字 E0006 于实参自身位（failAt(base+i)）；soft 引号语义（洞内/attr 实参内的嵌套串 E0002 位置在该串开口处）。
8. **trimSpace 的 ASCII 面**：语料可达 = space/tab（\n\r 先死于平衡走查/checkEncoding；\v\f 入集）；Go strings.TrimSpace 会裁的 unicode 空白（U+00A0 等）为不可达角——diverge 即响亮失败（D3 quote 同款纪律）。
9. **语言面小记（t4probe 探针，/tmp 不入册）**：自递归/模块级互递归（skipArgString↔skipArgHole）/while+`continue` 全绿；E0202 复发于 consumeEscape 的 var 赋值 else-if 链（补尾 else 成固定写法）；`while true` + 值返回须补不可达尾 `return`（检查器不做无穷循环终止分析）；非 unit 值调用须显式弃置（`let _ = expr`，E0605）。
10. **计数**：`reconciled 1104`（红 573 → 0）；corpus 1105（stdlib 7/wec 3/lextest 36/conformance 978 + 字面 81）；17 包全绿（`go test ./... -count=1` 0 FAIL）；conformance 933 不动（零 Go 生产码改动——wec_test.go 测试码 + wec/ We 源）；`we check wec/` exit 0；`gofmt -l` 净、`git diff --check` 净。
