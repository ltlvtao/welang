# proposal — stdlib-conversions（B3a）

> 状态：candidate（勘查与五项轨道裁决 + 面形裁决已完成，待 spec-impact audit）

## Why

B2 轨整体收口（B2a+B2b 皆 done，HEAD `ce66102`，conformance 919、codegen 单测 474、16 包全绿）。roadmap B3 行给轨道定的验收是三段闭环（Go 宿主构建编译 We 编译器，其产物再编译编译器一次，两段产物逐字节一致）加 conformance 套件在 We 宿主编译器下全绿。B3 勘查（2026-09-16，详见下节）钉死了两个事实：其一，63.7k 行 Go 编译器必须移植到 We；其二，**We 语言面今天写不出这门编译器**——不是结构面（AST/环境表/字符串建造皆有对位），而是几个转换原语整个缺席。本变更是六段切片的第一段：先把这些原语落进标准库，wec 移植（B3b 起）才有可写的地基。

**B3 轨道五项裁决（2026-09-16，用户逐字回复「1. 六段。2. 挂string，以Option答。3. 同意。4. 非目标。5. 逐字节对账」）：**

| # | 问句 | 裁决 |
| --- | --- | --- |
| 1 | 切片粒度（α 六段 / β 四段 / 自定） | **六段**：B3a stdlib 原语 → B3b wec 项目 + lex 移植 → B3c parser → B3d checker → B3e codegen → B3f cli/test/fmt/vet/doc + 三段闭环 + conformance 第二面。B3d/B3e 实测过大时按 B1/B2「按实现落地」先例现场拆分。 |
| 2 | 原语挂哪、失败面怎么答 | **挂 `std.string` 函数面，失败以 `Option` 作答**（非成员面、非 panic）。 |
| 3 | wec 的家 | **同意**：仓库顶层 `wec/`（`we.toml` type=executable + `src/` 多模块），Go 链不动，Go 侧测试编译并驱它。 |
| 4 | lsp 与 deps 的归宿 | **非目标**：lsp 零黄金——We 宿主二进制里诚实 exit 70（实现态披露）；deps 零黄金且自举不需要——申报非目标、登记 follow-up（`[deps]` 表解析保留，有依赖时诚实停）。 |
| 5 | B3e 差分杠 | **逐字节对账**：中间杠 = We 发射的 .ll 与 Go 发射的 .ll 逐字节相同（非验收必需，但把「919 绿」从赌注变推论）。 |

轨道裁决的长期载体是 roadmap 策略段（本变更 T6 落双语句）；里程碑行拆分（B3 → B3a..B3f）在 B3a 归档时按 B1/B2 先例执行。

## 今日事实（勘查，HEAD `ce66102`）

**语言面缺口——写编译器必需、今天没有的**（B3 勘查结论，逐条有代码锚）：

1. **Rune 无序比较**：`<` 域 = 纯数值（`internal/typecheck/typecheck.go:6527-6531` `isNumeric`），Rune 只有 `==`/`!=`（Eq 域含 Rune，`typecheck.go:1640`）。词法分类 `c >= 'a' && c <= 'z'` 不可写。
2. **无 rune↔int 转换**：`runeMembers` 表不存在（全 typecheck 无此表）；无从把 Rune 变 Int64 做算术，也无从从 Int64 造 Rune。
3. **无 string→number 解析**：Go 编译器六处 strconv（`internal/codegen/codegen.go:2778` ParseFloat——字面量折叠；`:2917` ParseInt(10)；`:13697`/`:13860` ParseInt(0)；`:13705`/`:13853` ParseUint(0)——后五处字面量与插值折叠）。We 无任何对应面——连逐位求数值都因缺 rune↔int 写不出。
4. **无 Float64 位型**：`.ll` 浮点字面量按 `0x%016X` 位型发射（`codegen.go:2782`，`math.Float64bits`）——位型重解释（bitcast）在 We 里不可表达。
5. **hex 格式化与排序不需要 std 面**：得了 rune↔int 后 hex 逐 nibble 手写（`"0123456789ABCDEF"` 查表）约十五行；mergeSort 手写。二者留在 wec 源内，**不进 std**（最小面纪律）。`unicode.IsLetter`×1 实为 ASCII 判定（`internal/lex/lex.go:419` 自证 a-zA-Z），同样手写。

**结构面对位良好**（勘查结论，B3a 无需触碰）：AST/Type 树 = Go interface+struct → We sum+record 天然对位；环境表 = `Map<String, rec>`（载荷一字句柄 ✓）；字符串建造 = `List<String>` + `std.string.join` ✓；`defer`/位运算/while/loop 已批；fs/process 足够（readFile/writeFile/listDir/process.run）；成员面 Option 载荷一字的（`Map<String,rec>.get`）可跑。

**std.string 今日态**：`stdlib/src/string.we` 真体二枚（join/repeat，纯、乘程序面——`codegen.go:1864-1874` 唯一得 `curImports` 腿的 std 模块）；键控拦截机制在册（`registerStdModule:1198`、declare 表 `codegen.go:88`、fs 七枚/process/collections 先例）；Option 返回族 out 三字组先例（B2b `__we_coll_*_get`）。**混合模块是本变更唯一的机制新面**：string 别名下既有真体调用（join/repeat 走程序面）又将出现键控调用（七原语走拦截）——分派须先查键控集再落程序面。

**Rune 域的既有事实**（如实记录，本变更不改变它）：`\u{...}` 逃逸解析不设范围校验（`codegen.go:16718-16742` `parseUnicodeEscape`——1-6 个 hex 位任意累加，无 0x10FFFF 上限、无代理区排除）；rune 字面量收任意单字符。今日 Rune 就是「i64 码点、不校验」——`runeFrom` 作恒等过桥正是对此事实的忠实，而非新开的口子。范围校验的缺席是既有语言面事实，本变更披露并提议归档时登记 follow-up。

## 目标与非目标

目标（一条线：七个转换原语落到 std.string，键控+虚构体+真载体）：

1. **面**（裁决 2 的形）：`stdlib/src/string.we` 增七枚虚构体声明——
   - `pub fn parseInt(s: String) -> Option<Int64>`
   - `pub fn parseUInt(s: String) -> Option<UInt64>`
   - `pub fn parseFloat(s: String) -> Option<Float64>`
   - `pub fn runeCode(c: Rune) -> Int64`
   - `pub fn runeFrom(n: Int64) -> Rune`
   - `pub fn floatBits(f: Float64) -> Int64`
   - `pub fn floatFromBits(n: Int64) -> Float64`
   七枚皆纯（无 effect 段，join/repeat 先例）；失败面（畸形/溢出）以 `None` 作答；`runeCode`/`runeFrom`/`floatBits`/`floatFromBits` 是全函数（恒等与位型重解释，无失败面）。
2. **接受文法 = 十进制核心**（design D2 定案）：`parseInt`/`parseUInt` 收非空纯十进制数字串（无符号、无前缀、无下划线、无空白；溢出 `None`）；`parseFloat` 收 ch1 浮点核心形（`数字.数字[eE[±]数字]`，全串、无下划线；范围错 `None`）。前缀/下划线/符号的归一是调用方的组合层——wec 的词法器自己按 ch1 规则归一后调核心面。
3. **C 载体**：`runtime/c/str.c` 七入口（`__we_string_parse_int/parse_uint/parse_float`（ptr, i64, ptr out）out 三字组 + `__we_string_rune_code/rune_from/float_bits/float_from_bits` 四枚恒等/位型过桥），NUL/长度纪律沿 fs 先例（拷贝加 NUL 终止、endptr 全串核对）。
4. **键控拦截 + 混合模块分派**：declare 表七行；string 别名下分派先查键控集、未中再落程序面（join/repeat 零改动）；`registerStdModule("std.string")` 不需要（无和式/记录要登记）——装载面零触碰的论证入 design D4。
5. **mock 面**：七枚单态模块 fn 皆 mock 目标（B2a outTrio 先例、B2b E1804 泛型类目的对面），mock 拦截黄金钉住。
6. **黄金**：正/负/溢出/回环矩阵（`floatFromBits(floatBits(x)) == x`、`runeFrom(runeCode(c)) == c`）逐枚先红后绿、手写期望字节。
7. **docs**：`docs/benchmarks.md` + `.zh.md` 运行面句双语补「stdlib-conversions 线拓宽」。
8. **roadmap 策略段**：B3 轨五裁决双语句落 `docs/roadmap/0000-reference-implementation.md`（±zh）策略节。

非目标：

- **成员面转换**（`r.toInt64()` 形）：裁决 2 拒——挂 std.string 函数面。E0501 remediation 预告的「conversion method 清单」是未来另一变更的事。
- **Rune/String 序比较**（`<` 域扩）：不开——wec 用 runeCode 换 Int64 后比较，语言面零增量。
- **hex 格式化、排序、IsLetter、前缀/下划线归一进 std**：皆 wec 源内手写（B3b 起），std 面保持最小。
- **strtod 全脸**（inf/nan/十六进制浮点/前后空白）：拒——接受文法锚 ch1 核心形，规范先行纪律不破。
- **`\u` 逃逸范围校验**：既有语言面事实，本变更只披露；是否登记 follow-up 归档时裁定（T7）。
- **spec 触碰**：零 Requirement 增删、零诊断码、零 ADR——七面是 stdlib 自有表面（join/repeat、fs 七枚、collections 构造面同 Doctrine：锚定族之外的标准库表面不进规范）；本变更「不改变语言行为」「无规范增量」。
- **性能调优**：parse 按正确性先写。
- **CLI/工具面、LSP、formatter、deps**：零触碰。

## What Changes

- `stdlib/src/string.we`：+7 虚构体声明（panic 体，Never 满足任意返回位——mapOf 先例）。
- `runtime/c/str.c`（+`str.h`）：+7 C 入口；`runtime/str_test.go` 新测试。
- `internal/codegen/codegen.go`：declare 表 +7 行；键控拦截臂 +7（Option 族 out 三字组、四枚直过）；string 别名混合分派（先键控后程序面）。
- `internal/conformance/testdata/cases/`：新黄金（正面/负面/溢出/回环/mock）。
- `docs/benchmarks.md` + `.zh.md`：运行面句。
- `docs/roadmap/0000-reference-implementation.md` + `.zh.md`：策略段 B3 五裁决句。

## 影响层

`layers: [compiler, stdlib, docs]`——codegen（键控臂与分派）、stdlib（string.we 七面）、docs（benchmarks 运行面 + roadmap 策略句）。typecheck 零码改（虚构体声明驱动定型，B2b mapOf 同形；T2 探针复核）。

## 影响范围

- 调用方：任何 `import std.string` 的程序多七枚可用面；既有 join/repeat 调用零漂移（T2 三缝复核：黄金零改写 + hello IR 逐字节 + mock 黄金绿）。
- 键控机制：fs/process/collections 既有拦截零触碰（分派臂是 string 别名下的先行查询，不改共享表）。
- mock：std.string 键控面入槽表（monomorphic，无 E1804 类目）。
- 黄金账预期：919 → ~933（正负矩阵；as-built 数字以 `-count=1` 实测为准）。

## 审计记录

**welang-spec-impact-audit（2026-09-16，HEAD `ce66102`）：通过，七条全过。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | 问题真实性 | 工程缺口（B3 轨道前置）：We 无 rune↔int/解析/位型原语，写不出编译器——代码锚 `typecheck.go:6527`（Rune 无序比较）、`codegen.go:2778/2782`（折叠与位型）、`lex.go:419`；轨道权威 = roadmap B3 行 + 策略节自举闭环（ADR-0002 载体） |
| 2 | 影响层声明 | `change.yaml [compiler, stdlib, docs]` 与 proposal 影响层合一；「不改变语言行为」「无规范增量」标记在目标与非目标（join/repeat、fs 七枚、collections 构造面同 Doctrine 先例） |
| 3 | 规范增量范围 | 零 Requirement 增删、零诊断码（proposal 显式）；无全局唯一性冲突面 |
| 4 | 原则一致性 | 无冲突；显式转换函数面兑现 ch1「无隐式转换」的显式路（E0501 remediation 预告的标准库清单方向） |
| 5 | 参考基线固定 | 无 refr/ 引用；规范锚 `docs/spec/0100-lexical.md:111`（字面量文法）、ch16（纯性）、ch20（mock）；实现锚 HEAD `ce66102` |
| 6 | 验收边界 | 目标皆命令化可判（黄金 `-count=1`、IR `cmp` 零差、docs_sync）；非目标显式排除成员面/序比较/hex/排序/strtod 全脸/`\u` 域/spec 触碰 |
| 7 | 粒度 | 单一垂直单元：七面共一个机制新面（混合模块分派），C 载体/装载/发射/黄金皆服务该面 |

## 审查记录

**welang-change-review（2026-09-16，ready → active 关卡）：通过，十条全过，一处随审订正。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | proposal 职责 | 黑盒问题（语言面缺口）+ 目标/非目标 + 影响面；勘查与裁决表沿 B2a/B2b 形制；无任务清单混入 |
| 2 | spec 增量 | 豁免（「不改变语言行为」「无规范增量」标记在目标与非目标；锚定族外 stdlib 表面 Doctrine 同 B2a/B2b） |
| 3 | design 职责 | 唯一最小路径 D1–D7；被拒方案七条带理由；规范/代码锚精确到行（`0100-lexical.md:111`、`codegen.go:2778/2782`、`typecheck.go:6527`）；与豁免的 spec 面无矛盾 |
| 4 | tasks 职责 | 只执行 proposal/design 已定义工作；逐框来源/验证；**随审订正一处**——T7 follow-up 项原引入「用户裁定后落」的不必要阻塞，改为直接登记（B2b #26 先例：登记是如实记账，裁定属未来打开它的变更） |
| 5 | 场景覆盖 | normal（正面上界/回环）/ boundary（`9223372036854775807`、`18446744073709551615`、denormal）/ failure（空/垃圾/NUL 中断/溢出上下溢 → None）三路齐 |
| 6 | 无空章节 | 无 |
| 7 | 测试先行 | T1 runtime 测试先红；T3 黄金红态（T2 后 build 70）记档后实现翻绿；头注红先纪律声明 |
| 8 | 负向断言 | 负面黄金实构违规样例断言 None（T4）；E0501 两形探针（T2）；突变电池四枚含负向捕获性判决（T4） |
| 9 | 完成度闭环 | 三要素齐：检查器（零码改+探针证）、代码生成（七键控臂+混合分派）、运行时（str.c 七入口） |
| 10 | 未决问题 | 无阻塞未决——`\u` 域与 deps 是登记项非设计选择（已订正为直接登记）；B3d/B3e 现场拆分属轨道裁决 1 授权的执行自由度 |
