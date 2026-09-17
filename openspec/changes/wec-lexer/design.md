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
