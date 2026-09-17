# proposal — wec-lexer（B3b）

> 状态：candidate（勘查已完成，待 spec-impact audit）

## Why

B3a 收口（归档提交 `1a7e81a`，conformance 933、codegen 单测 482、16 包全绿），转换原语已落齐——B3 轨道勘查钉死的「We 语言面写不出编译器」结论中，语言面缺口一侧已由 B3a 兑现。本变更是六段切片的第二段（轨道裁决 1）与裁决 3 的兑现：**wec 项目落地仓库顶层，Go 词法器（`internal/lex/lex.go`，1273 行）移植为 We 源**——自举轨的第一块真编译器代码。

本段自己的差分杠是**词法层逐字节对账**（区别于 B3e 的 .ll 逐字节杠）：wec 二进制对语料的序列化输出与 Go 词法器在同一序列化规范下的输出逐字节相同。参考侧由 Go 测试塔从 `internal/lex` 活算——零手写期望，移植漂移无处藏身。

**轨道裁决引用（2026-09-16，用户逐字「1. 六段。2. 挂string，以Option答。3. 同意。4. 非目标。5. 逐字节对账」）**：裁决 1 定本段为 B3b（wec 项目 + 词法器移植）；裁决 3 定 wec 的家——仓库顶层 `wec/`（`we.toml` type=executable + `src/` 多模块），Go 构建链不动，Go 侧测试编译并驱它；裁决 4 定 lsp/deps 非目标（沿 roadmap 策略段，已随 B3a 落双语句）。

## 今日事实（勘查，HEAD `1a7e81a`）

**Go 词法器形状**（移植面，逐条有锚）：

1. 五入口：`File`（`lex.go:135`）与 `Scan`（`:148`，附 docs 侧通道）开整文件；`ScanAt`（`:157`，插值洞区域再基）与 `Keep`（`:165`，注释保形——格式化器面）是另外两张脸。首错停（ch1）：`fail` 经 `panic(stop{d})` 解栈（`:197-203`），`runAt` 的 recover 收为首诊断、令牌流清空（`:182-190`）。
2. 消费者：`parser.go:144`（Scan）、`:947`（Holes）、`:973`（ScanAt）、`fmt.go:31`（Keep）——本段只移植 Scan 家族真正消费的面（File/Scan + docs）；ScanAt/Holes 是 B3c 插值洞的地基，Keep 是 B3f 格式化器的脸。
3. `Token{Kind,Text,Line,Col,AttrName,AttrArgs}`（`:82-94`）：kind 集 = ident/keyword/int/float/string/rune/attr/comment/eof，运算符 kind 即自身文本；AttrName/AttrArgs 只在 attr 词上置位。`DocUnit{StartLine,EndLine,Lines}`（`:101`）。
4. `checkEncoding`（`:259-296`）先于一切扫描：全文件有效 UTF-8（否则报**首病字节值** `byte 0x%02X at offset %d is not valid UTF-8`）、BOM 只许单枚前导（他处出现报 offset）、裸 CR 拒绝（CRLF 计一次断行）；成功则剥离前导 BOM。
5. 扫描核全 ASCII 分类：`peek/peek2` 读字节，但分支只比 ASCII；`advance`（`:244-253`）按 **rune** 推进并计列——**col 是 1-based rune 列**（`posAt :211-226` 同基，从起点走查）。数值文法 `numericSpan`（E0006 七条款 + 后缀集 + `1e5` 切形）、字符串/逃逸（E0002/E0005/E0007；`\u{1-6 hex}` 带上限与代理排除——词法器校验，与 #27 的 codegen 缺口不同层）、attr 序 form(E0001)→literal-only(E0009)→whitelist(E0008，空集)、`consumeHole` 深度/括号平衡。

**We 面已齐**（B3a 结论兑现，本段纯消费、零语言面增量）：

1. String 双层成员（ch17）：`byteLength()->Int64`、`byteSlice(Int64,Int64)->String`、`runeCount()->Int64`、`charAt(Int64)->Rune`（`typecheck.go:1343-1350`；发射臂 `codegen.go:11939-11947`/`:12233-12257`）；`string.runeCode/runeFrom`（B3a 键控面）。
2. **关键勘定——rune 层与 Go range 语义逐字节一致**：运行时 `utf8_step`（`runtime/c/str.c:37-74`）把病形序列（杂散连续字节/截尾/过长/代理/超 U+10FFFF）读为 U+FFFD 并**推进一字节**——恰是 Go `range string` 的语义。故「按 rune 走查 + 按码点预测宽推进字节偏移 off + `byteSlice(start,end)` 取词文本」的映射成立：分类用 rune 比较（`charAt`），文本用字节切片（原样字节视图），`posAt` 用前缀切片走查精确复刻。
3. **有效 UTF-8 判定可精确表达**：Σ 预测宽 == `byteLength` ⟺ 全串良构（每病形字节实际推进 1 而预测 3，二者只在良构时相等）；首病字节可定位（走查中首个 FFFD 且其三字节探针 `byteSlice(off,off+3)` 非 `runeCount()==1 && byteLength()==3`——即非良构 EF BF BD）。
4. **字节值暴露面的边界**：String 字面量经 `decodeStringLiteral`（`codegen.go:16847-16894`）发射——`utf8.ValidRune` 拒绝代理与超域值，故**字面量只能构造良构 UTF-8 字节**；可名字节 = 良构编码中出现过的字节 = 0x00–0xBF、C2–DF、E0–EF、F0–F4 共 243/256（等值探针查定，命中偏移算术反推值）。**C0、C1、F5–FF 共 13 值不可名**——无字节算术面，等值/runeCount/charAt/byteLength 四观测对该 13 值不变（形式上不可区分）。E0001 的 UTF-8 消息含 `byte 0x%02X`：243 值角可逐字节复现，13 值角在现有语言面下**不可复现**——语料门槛 + follow-up（见非目标）。
5. `fs.readFile(path) effect io -> Result<String, FsError>` 原样字节（`fs.c:151-170`：`rb` + `read_all`，无校验）——非法字节可达 We String，checkEncoding 的三面在 wec 内部可达。
6. 词法器状态模式：record + 固有 impl + `mut self` 方法内 `self.field` 赋值（E0105：普通字段赋值被拒、`self.field` 属方法接收者——`check-e0105-field-assign` 黄金钉；`run-for-user-iterable` 等 10 枚黄金实证 `mut self` 突变跨调用持久）——直接对位 Go 的 `(l *lexer)` 方法集。We 无 recover：`panic(stop)` 改形为**错误闩**（failed 标志 + 诊断五字段），每方法入口查闩。
7. 无 argv/stdin 面（ch15/ch21 均无；main 无参形）——规范先行纪律禁止发明：驱动契约取**固定路径**（wec 二进制读其 cwd 下 `wec-input.we`）。B3f 的 CLI 面需要 argv——follow-up #29（归档时登记）。
8. Go 侧塔模式：`cli.Run(args, stdout, stderr) int` 进程内（`internal/cli/cli.go:88`）、`we build <dir>` 产 `build/<name>`（`build.go:25`）、conformance-runner 式 `os.Chdir` + 子进程执行（`runner.go`）。新 `internal/wec` 测试包，包数 16→17。

## 目标与非目标

目标（一条线：wec 骨架 + 词法器移植 + 逐字节对账塔）：

1. **wec 项目**：仓库顶层 `wec/`——`we.toml`（`name = "wec"`、`type = "executable"`）+ `src/` 三模块（`main.we` 驱动与序列化、`lex.we` 扫描核、`lexnum.we` 数值文法；模块路径字符集 `[a-z][a-z0-9]` 合规）。裁决 3 的形。
2. **词法器移植**：`File`/`Scan` 面 + docs 侧通道的完整移植——扫描核（空白/注释、ident/keyword、运算符、数值、rune、字符串与逃逸、attr、插值洞记录、eof）+ `checkEncoding` 三面 + 首错停改形（错误闩）。Token/DocUnit 对位 We record；关键字 ~45 词收 String 列表字面。
3. **序列化规范**（design D4 定案）：stdout 文本——成功 = `tokens` 头 + 逐词 `T <line> <col> <kind> <text>`（attr 词附名与实参）+ docs 段（`D <start> <end>` + `L <text>`）；词法错 = 单行 `E <line> <col> <code> <msg>`。转义机械（反斜杠/界外字节）。退出码恒 0（诊断在输出里，不在进程位上）。
4. **对账塔**（`internal/wec`）：进程内构建 wec 一次，逐语料条目置 `wec-input.we` 于临时目录、exec 工件；参考侧从 `internal/lex` 活算同规范序列化；逐字节比对。语料构成（design D7）：stdlib/src 全部 .we、wec/src 自身、Go lex 测试语料逐字面提取、E0001–E0009 全码矩阵、933 conformance 案例的 setup .we 源、名可达非法 UTF-8 补充形（杂散 C3/截尾 E2 82/代理 ED A0 80/超域 F4 90 80 80/过长 E0 80 80）、非 ASCII 字面项。
5. **突变电池**：对 wec 的 We 源做单点突变（分类界/宽表/关键字表/列推进/E0006 条款/逃逸校验），塔逐枚击杀判决表（B2b 先例）。
6. **零漂移**：conformance 933 零改写全绿（本变更零 Go 生产码改动，漂移在结构上不可达，仍以全量轮复验）。
7. **docs**：roadmap B3b 行归档时翻写；follow-up 登记 #29（argv/stdin 面——B3f 的 CLI 需要它）与 #30（字节暴露面——E0001 消息 13 值角的根治面）。

非目标：

- **ScanAt/Holes 移植**：B3c（插值洞区域再基是 parser 的地基）。
- **Keep 移植**：B3f（格式化器的注释保形脸）。
- **argv/stdin/字节暴露等任何语言面增量**：规范先行纪律拒——wec 是语言消费者不是语言拓宽者；缺口以 follow-up 登记（#29/#30），与本变更的固定路径驱动契约不冲突。
- **13 字节值角（C0 C1 F5–FF）的对账**：现有语言面下形式不可达（勘查事实 4 的不变性论证）；语料门槛剔除，达到即响亮失败。lex 测试语料中恰有一枚 `"\xff"` 用例属此角——剔除并披露，同面以五枚名可达形覆盖。
- **消息 help 文本的对账**：help 是注册表静态属性（`lex.go:64-72`），非词法器行为；序列化不含。
- **文件名进诊断序列化**：`At(name, ...)` 的 name 是调用方给的；两侧共用固定名，序列化省略 name 字段。
- **lsp/deps**：轨道裁决 4 已定非目标（roadmap 策略段已载）。
- **Go 生产码改动**：零——`internal/wec` 是纯测试塔。
- **性能调优**：正确性先写（probe 查定是错误路径上的一次 ~256 次等值扫描，可接受）。
- **B3c 及以后**（parser/checker/codegen/CLI）：不在本段。

## What Changes

- `wec/`（新目录）：`we.toml` + `src/main.we` + `src/lex.we` + `src/lexnum.we`——We 宿主编译器的词法段种子。
- `internal/wec/`（新测试包）：对账塔（构建一次 + 逐语料 exec + 参考侧活算 + 逐字节比对）+ 突变电池 + 语料提取器（读 conformance testdata，只读）。
- `docs/roadmap/0000-reference-implementation.md` + `.zh.md`：B3b 行归档时翻写；follow-up #29/#30 登记。
- `openspec/changes/wec-lexer/`：本四工件（随 T1 入册）。

## 影响层

`layers: [compiler, docs]`——**internals-only**：compiler 层的触达是编译器仓的测试面（`internal/wec`，不进任何生产管线；Go 生产码零 diff），wec/ 是 We 语言消费者程序（未来编译器的种子），二者皆不改变语言行为；docs 层是 roadmap 执行状态（归档时翻写）。**本变更「不改变语言行为」「无规范增量」**：零 Requirement 增删、零诊断码、零 ADR；不触 `docs/spec/`、`diagnostics.toml`、stdlib、运行时。

## 影响范围

- Go 工具链：零生产面改动——所有既有命令、管线、黄金零漂移（目标 6 全量复验）。
- 语料涉及面：stdlib 源、conformance testdata、lex 测试样例皆**只读**消费。
- 后续段：B3c 起在 wec/src 上续建（parser 对接 Scan 面）；本段固定的序列化规范与塔是 B3c–B3f 的对账地基（diff 杠随段换，塔结构沿袭）。
- 包账预期：16 → 17（`internal/wec`）；conformance 语料账 933 不动。

## 审计记录

**welang-spec-impact-audit（2026-09-17，HEAD `1a7e81a`）：通过，七条全过。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | 问题真实性 | 工程缺口（B3 轨道第二段）：轨道权威 = roadmap B3b 行 + 策略节裁决 1/3（六段切分与 wec 的家）；词法器移植是自举路径的第一块真编译器代码——代码锚 `internal/lex/lex.go`（五入口/首错停/Token 形）、`typecheck.go:1343-1350`（双层 String 成员）、`runtime/c/str.c:37-74`（rune 层语义）、`fs.c:151-170`（原样字节） |
| 2 | 影响层声明 | `change.yaml [compiler, docs]` 与 proposal 影响层合一；internals-only 论证在案（Go 生产码零 diff、`internal/wec` 纯测试塔、wec/ 是语言消费者）；「不改变语言行为」「无规范增量」标记在目标与非目标 |
| 3 | 规范增量范围 | 零 Requirement 增删、零诊断码、零 ADR（proposal 显式）；无全局唯一性冲突面 |
| 4 | 原则一致性 | 无冲突；规范先行纪律正面兑现——argv/stdin 与字节暴露两缺口不发明面、以固定路径契约 + follow-up 登记（#29/#30）处理，被拒方案表载明 |
| 5 | 参考基线固定 | 无 refr/ 引用；规范锚 = 已批 `docs/spec/0100-lexical.md`（词法面）、ch15（模块路径字符集）、ch17（String 双层成员）；实现锚 = HEAD `1a7e81a` 逐行号 |
| 6 | 验收边界 | 目标皆命令化可判（塔 `bytes.Equal` 逐语料、`-count=1` 全量、validate 期望态、突变电池判决表）；非目标显式排除 ScanAt/Holes/Keep、语言面增量、13 字节角、help/name 序列化、lsp/deps、Go 生产码、性能、B3c+ |
| 7 | 粒度 | 单一垂直单元：一个项目骨架 + 一个移植面（Scan 家族）+ 一座对账塔；B3c 的 parser 在本段结束处起建，无跨段耦合 |

## 审查记录

**welang-change-review（2026-09-17，ready → active 关卡）：通过，十条全过，一处随审订正。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | proposal 职责 | 黑盒问题（轨道第二段的工程缺口）+ 裁决引用 + 目标/非目标 + 影响面；勘查事实逐条带锚；无任务清单混入 |
| 2 | spec 增量 | 豁免（「不改变语言行为」「无规范增量」标记在目标与非目标；internals-only 论证在影响层——validate --strict 已过） |
| 3 | design 职责 | 唯一最小路径 D1–D7；被拒方案九条带理由；锚精确到行（`lex.go:211-253`、`str.c:37-74`、`fs.c:151-170`、`codegen.go:16847-16894`、黄金 `check-e0105-field-assign`/`run-for-user-iterable`）；与豁免的 spec 面无矛盾 |
| 4 | tasks 职责 | 只执行 proposal/design 已定义工作；逐框来源/验证；**随审订正一处**——初稿预置六枚「（待落地）」占位落地记节（落地记按 B2a/B2b/B3a 先例是落地时追加，非创建时预置），已删 |
| 5 | 场景覆盖 | normal（stdlib/wec/conformance 正面源）/ boundary（BOM 前导与中置、CRLF/裸 CR、名可达非法 UTF-8 五形、E0006 逐条款、非 ASCII 字面项）/ failure（E0001–E0009 全码矩阵、首错停两栏、读文件失败防御行）三路齐 |
| 6 | 无空章节 | 订正后无（见第 4 条） |
| 7 | 测试先行 | T1 塔先落且 wec 缺席红态记档（构建失败形）；T2–T4 语料子集先红（输出字节不匹配形）后绿；红先两形在 tasks 头注声明 |
| 8 | 负向断言 | 负面语料是实构违规样例（全码矩阵逐枚）；13 字节值角的不可达以「查定不中即任务失败」的响亮失败断言（非静默）；突变电池 ≥6 枚含负向捕获性判决 |
| 9 | 完成度闭环 | 非语言特性变更，闭环三要素对位为：移植体（wec We 源，D1/D2/D6）、验证塔（`internal/wec`，D5/D7）、驱动与序列化契约（D4）——三者皆有方案且互相咬合（塔的参考侧活算依赖契约、契约依赖固定路径决策） |
| 10 | 未决问题 | 无阻塞未决——argv/stdin 与字节暴露两缺口已以固定路径契约 + 语料门槛 + 响亮失败处理并登记 #29/#30（登记项非设计选择，B2b #26 先例）；B3d/B3e 现场拆分属轨道裁决 1 授权的执行自由度 |
