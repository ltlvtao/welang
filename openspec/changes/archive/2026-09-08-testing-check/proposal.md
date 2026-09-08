# 提案 — testing-check（M10a）

## Why

roadmap M10 承诺 ch20（测试块、mock、虚拟钟、确定性调度、test 边界）与 ch21 的 runner 面（`we test`、探索）。三裁决（2026-09-07，均采纳推荐）把 M10 切成三塔：**M10a 检查塔（本变更）**落 ch20 全部静态面，**M10b 运行塔**落 codegen 多函数发射、runner、虚拟钟与 test 边界的运行语义，**M10c 探索塔**落 `--explore` 的 POR 与双守卫。ch20 已批全量（8 Requirement / 30 Scenario），ch16/ch21 的相关修订同在册，E1801–E1806 与 E1901/E1902 已在 `docs/spec/diagnostics.toml`——本变更与 M9a 同为**纯实现**，规范增量零。

代码里四组预留边界自 M2/M6b/M9a 起等待归位，全部可黑盒验证：

1. **`bndTest`**（parser，两处 bnd 位 + 语句表一行）：`test` 顶层位（parser.go:413）与 `mock` 语句位（:1657）今日停「chapter 20 (testing) forms」；`test` 另在语句拒绝清单（:1661）报 E0105。
2. **`bndTaskTime` 的 advanceTime 半边**（typecheck，两处）：名值位（typecheck.go:6148）与调用位（:6648）今日停「task-scope and time-control functions (chapters 18 and 20)」——M9a Q3 裁决明确维持给 M10 归位（`currentCancelSignal` 半边 M9a 已收）。
3. **std.test 装载**：`import std.test` 今日 E1302 std 形（未知模块）——`StdModule` 注册表（M8）现挂 std.io / std.concurrent，待扩第三枚。
4. **advanceTime 的 prelude 位**：nameResolvable（typecheck.go:6443）已含其名——定型与位置规则（E1806）待落。

地基齐备无需新建：`time` 已是三大内建标签之一（typecheck.go:2586，E1403 拒声明冲突）；`assert` 已定型 `(Bool, String) -> ()`（terminationFnType，typecheck.go:1051）；E1401/E1402 效果机器（M7）、E1405 顶层纯净、fn 体语境机器（fnCtx）、导入解析（E1302/E1301/E1304/E1303）、泛型 bound 机器（M6a）全部既有。

## 裁决记录（candidate 阶段三项裁决，2026-09-07，均采纳推荐）

- **Q1 M10 体量切分 = M10a 检查塔 + M10b 运行塔 + M10c 探索塔**（M9a/M9b 先例同构）：静态面（test 块/mock 声明/E180x 检查/advanceTime 定型与位置/std.test 装载）与运行面（codegen 多函数发射 + `we test` runner + 虚拟钟 + test 边界 + mock 拦截）与探索面（POR + E1901/E1902）各自独立走全生命周期门。被拒面：两拆（探索并入 M10b——多函数发射已是全管线拓宽，五件事一锅审查面过宽）；单变更全量（M9 拆分前论证同型，不复述）。roadmap M10 行随本变更拆为三行（M6/M9 拆分先例）。
- **Q2 mock 拦截载体 = 运行时按名函数表**（M10b 落地，本变更记路由）：ch20 示例定死拦截必须覆盖被测函数体内的调用点（mock `store.read` 后 `load` 内部调用也被拦）——静态改写在 lexical 块内做不到；运行时名字槽间接精确匹配「按名拦截，非代理」语义。检查塔本变更只落声明检查，拦截运行时归 M10b。
- **Q3 test 边界载体 = 每测试一任务**（M10b 落地，本变更记路由）：生成的 `__we_main` 即 harness，逐 test 块按源序 spawn 任务 + await——Ok 即 pass、TaskPanic 即 fail，复用 M9b 任务边界全套机制（GC 窗口/panic 报文/完成链）。检查塔本变更只落 test 体的检查语境，运行边界归 M10b。

## 现状与差距

- **parser**：`test`/`mock` 两处 bnd（:413/:1657）+ `test` 语句表行（:1661）；关键字 `test`/`mock` 已在 ch1 保留闭集——零 lexer 改动。`Parse(name, src)`（:128）已接收文件名——`_test.we` 身份判定零管线改动。
- **typecheck**：advanceTime 两处 bndTaskTime（:6148/:6648）；fn 体语境机器（fnCtx/walkPlain/bodyTags）可承载 test 体的 valueless 语境与 E1401 抑制；导入符号表可承载 mock 目标解析（裸名 syms / 合格名跨模块 pub 面）。
- **装载**：`StdModule(key)` 注册表现两枚——std.test 待注册第三枚（assertTrue/assertFalse/assertEqual 特型面，M8 std.io 先例）。
- **conformance**：黄金 schema 的 `files` 映射自带文件命名权——`_test.we` 身份黄金（E1801 负例与绿面）零 runner 改动，`args: ["check", "tests/x_test.we"]` 单文件面直达。
- **跨模块 mock 目标的通路限制（披露）**：ch15 导入→文件映射为 `import x` → `src/x.we`，而 `isModuleSeg` 字符集 `[a-z][a-z0-9]*`（parser.go:200）拼不出下划线段——test 模块（`*_test.we` 词干）无法经导入图进入项目编译。ch21 场景称 `src/helpers_test.we`「importable」与该段集存在规范张力（**follow-up #12 登记**，design D10）；本变更跨模块 mock 目标（`mod.fn` 形）的覆盖走 typecheck 多模块单测，项目面编译通路归 M10b `we test`。
- **`test` 关键字与 std.test 段的碰撞（披露，实现准备期揭出）**：`test` 在 ch1 保留闭集（KindKeyword），moduleSeg 今只收 KindIdent——`import std.test` 不可拼写；ch20 示例的 `std.test.assertEqual(...)` 全点形非 ch15:64 定型的 `name.item`。决议（design D8）：路径段接受过字符集检查的关键字 token；调用面 = 别名形 `import std.test as st`；裸导入合法但惰性绑定；全点形不实现——**follow-up #13 登记**（design D10）。
- **codegen**：test 块/mock 落地后一切含其形的 build 停新边界行（design D9 钉死逐形），诚实边界由 M10b 收口。

## 目标与非目标

**目标**：

1. parser 产生式全落：`test "description" { body }` 顶层项（描述 = 恰一字符串字面量，逐字携带无语义）、`mock name(params) [-> type] [effect tags] { body }` test 体直接项——含 E0105 家族错误位、`pub test` 经既有 pub 尾分派自然 E0105、语句位 `mock` 从 E0105 换 E1802、顶层 `mock` E1802。
2. 模块身份：`_test.we` 后缀判定（文件名事实），E1801 test 块出现在非 test 模块（parse 期锚 `test` 关键字）。
3. test 体检查语境：valueless 函数体语境（裸 `return` 合法、携值 E0402 复用）、E1401 抑制的精确落位（ch16 修订：test 体内不触发）、defer 合法、E1405 顶层纯净维持。
4. mock 检查全链：E1802（位置）、目标解析（E1304/E1303 复用）、E1804（目标须模块级单调 fn——泛型 fn/构造器/非 fn 名拒）、E1803（签名逐字：参数名+型+序、返回在场+型、效果段在场+tag 序）、E1805（一块一目标一 mock）、mock 体按目标段检查（E1401 于其内照查）。
5. advanceTime 归位：定型 `(Int64) -> ()`、名值位与调用位同判、E1806 位置规则（合法位 = test 体及其内 task/scope 体；闭包体/mock 体/一切函数语境拒——保守字面读，披露）。
6. std.test 装载：`StdModule("std.test")` 第三枚注册——`assertTrue(Bool)`/`assertFalse(Bool)`/`assertEqual` 特型面（同型对，域 = 整型族/Bool/String；域外诚实边界 What）、成员闭集 E0816、零效果段。
7. conformance 黄金新增（预计 ~30）+ 单测先行全周期（红→绿证据、黑盒电池、`--json` 协议实测）。
8. roadmap 手术：M10 行拆 M10a/M10b/M10c 三行（双语）+ follow-up #12 登记。

**非目标**：

- `we test` runner（tests/ 递归发现、源序、`--filter`、退出码 0/1/2、报告与 `--json` test 事件）、test 模块的项目面编译通路（**M10b**）。
- 虚拟钟、advanceTime 的运行语义、确定性调度的运行时承诺、test 边界的运行捕获（**M10b**；本变更 advanceTime 合法位只在 test 体内、而 test 体停 codegen 边界——运行时不可达，design D9 论证）。
- mock 的运行时拦截（按名函数表——Q2 裁决路由 M10b）。
- `--explore`、POR、E1901/E1902、manifest `[test].explore-iterations`（**M10c**）。
- `assertEqual` 的 Eq 泛型面与内建 Eq 实例集（标准库拓宽变更的事——本变更落特型面 + 域外边界，design D8）。
- 规范增量与诊断注册表改动（零——六码全部在册纯落实现）。

## What Changes

无规范增量：ch20/ch16/ch21 已批、E1801–E1806 已在注册表，本变更全部行为都是已批面的落实现。变更交付：parser 产生式与 AST 节点（TestDecl/MockDecl）、模块身份位与 E1801、test 体检查语境与 E1401 抑制、mock 检查全链（E1802–E1805 + 判序）、advanceTime 定型与 E1806、std.test StdModule 注册与断言面、conformance 黄金与单测、roadmap 三拆与 follow-up #12。被替换的边界：bndTest 全删（parser 两处 + 常量）、bndTaskTime 的 advanceTime 半边（两处，常量随 M9a 后仅剩的用途消失而删）。

## 影响层

| 层 | 触及 |
| --- | --- |
| compiler | parser 产生式、typecheck 装载/语境/纪律面、conformance 黄金 |
| stdlib | std.test 内建模块注册（M8 StdModule 机制扩容第三枚） |

（纯实现——「无规范增量」豁免路径，规范层零触及。）

## 影响范围

| 层 | 文件 | 动作 |
| --- | --- | --- |
| compiler | `internal/ast/ast.go` | TestDecl/MockDecl 节点（+ File 的 test 身份位） |
| compiler | `internal/parser/parser.go` | 两处 bnd 位删 + 两产生式 + E1801/E1802 + E0105 家族报文位 |
| compiler | `internal/typecheck/typecheck.go` | advanceTime 两处归位、test 体语境与 E1401 抑制、mock 检查全链、E180x 接线 |
| stdlib | `internal/typecheck`（StdModule 注册表处） | std.test 合成声明 + 断言面特型 |
| compiler | `internal/codegen/codegen.go` | TestDecl 显式停点 case + bndTestModule 边界词（design D9；proposal 非目标「运行时不可达」论证的落地面） |
| compiler | `internal/cli/build.go` | 单文件 build 对 test 模块直达 codegen 停点（design D9 两形黄金的实现面） |
| tests | `internal/parser`/`internal/typecheck` 单测、conformance 黄金（预计 ~30 新增） | 测试先行 |
| docs | `docs/roadmap/0000-reference-implementation.md`（+ .zh.md） | M10 行拆三行 + follow-up #12 |

## 涉触码盘点

- parser.go：:413 顶层 `test` bnd、:1657 语句 `mock` bnd、:1661 语句表 `test` 行——bndTest 常量（:37）删；顶层 `mock` 入 E1802、语句位 `mock` 入 E1802（语境标志：仅 test 体直接项解析为 MockDecl）；`test` 语句位维持 E0105。
- typecheck.go：:6148/:6648 advanceTime 两处 bndTaskTime 归位（bndTaskTime 常量 :51 删——M9a 后仅剩这两处用途）；test 体语境（fnCtx valueless 形 + bodyTags 抑制哨兵）；mock 检查（目标解析走既有 syms/导入面）；E180x 五码 tHelps 补条。
- 既有测试更新面（随批更新，逐处披露，实现期核实）：parser dispatch 表若有 `test`/`mock` 行为锁、typecheck_test.go 的 bndTaskTime advanceTime 钉（:313 族，M9a T5 记录在案的「Q3 回归钉」——本变更翻真行为）；conformance 既有 508 枚预期零触碰（`advancetime-bnd` 锁定绿将翻——M9a 落的回归钉随 M10a 归位翻绿，随批披露）。
- 新增：conformance 黄金（六码 × 正负例 + 绿面族）、单测（parser 产生式/typecheck 语境与 mock 面）、真机电池。

## 已知风险与开放问题

- **E1401 抑制的边界精确性**：ch16 修订只说「calls inside one [test block]」不触发——抑制须穿透 if/while/match 臂的嵌套块，但不入 task 体（自段照查）、mock 体（对目标段查）、闭包体（本无 E1401 面）。抑制哨兵的进出时机是设计工作（design D3）。
- **advanceTime 的保守字面读**：「task and scope bodies included」明列两形——闭包体/mock 体未列，按函数体语境拒（E1806）。这是保守读而非规范明文，design D7 披露为静默处决议。
- **E1803 报文形**：registry 描述点名三失配类目（参数/返回/段）——报文渲染双签名还是首失配类目，design D5 钉死。
- **std.test 断言域**：assertEqual 的 Eq 泛型面被 ch10 两事实锁死（基类型头 impl E0811 拒、内建实例集规范未定）——特型面 + 域外边界是唯一不越权形状，design D8。
- **开放问题（design 收敛）**：test 模块在 codegen 停哪一行（helper fn 先行的源序使 bndOtherFns 可能先停）——design D9 钉死逐形；`advancetime-bnd` 既有锁定绿的翻绿处置——design D11。

## 审计记录

**2026-09-07：通过（7/7），status → ready。**

1. 问题真实性通过——Why 的四组预留边界（parser :413/:1657、typecheck :6148/:6648、StdModule 注册表、nameResolvable :6443）皆为可黑盒验证的工程缺口，规范引用 ch20（2000-testing.md 8 R/30 S）、ch16:36、ch21:95-97/124-150、diagnostics.toml E1801–E1806 全部在册。
2. 影响层通过——change.yaml `layers: [compiler, stdlib]` 与影响层表逐行一致；纯实现路径（M9a 先例）：行为全部由已批章节固定，非目标末条显式声明规范增量与注册表零触及。
3. 规范增量范围通过——增量零；六码 E1801–E1806 已在册（allocated 2026-09-04，owner 2000-testing/2100-toolchain），无冲突、无与 active 变更重复（无 active 变更）。
4. 原则一致性通过——无隐式转换；E1803 逐字匹配、assertEqual 域外诚实边界（exit 70）、test-ness 单一权威（文件名后缀）、保守字面读（E1806 闭包/mock 体、E1803 tag 序）全部显式披露非静默发明。
5. 参考基线通过——引用全部为已批 docs/spec 文件并锚行号（2026-09-07 工作树）；ch21「importable」张力登记 follow-up #12，未当既成规范消费。
6. 验收边界通过——目标 1–8 机械可判（黄金 ~30 对账、单测函数、roadmap 三行 + follow-up #12 在册）；非目标 6 条栅出 M10b（runner/虚拟钟/test 边界/mock 拦截）、M10c（POR/E19xx）、Eq 泛型面与项目面编译通路。
7. 粒度通过——检查塔单垂直单元（parser 产生式 → 检查语境 → 装载 → 黄金），M9a 同构先例；运行塔/探索塔各自独立走门（裁决 Q1）。

`openspec/tools/validate.py testing-check --strict`：OK。

## 审查记录

**2026-09-07：通过（10/10，发现 1 项已处置），status → active。**（`validate.py testing-check --strict` 先行通过。）

1. proposal 职责通过——黑盒问题与目标为主；现状与差距的码行锚是 M9a 既定格式，实现决策全部下放 design（proposal 仅以「design D 编号」指路）。
2. spec 增量通过——纯实现豁免路径（零增量），非目标末条显式声明；无越界的伪 spec 段。
3. design 通过——逐 D 单一路径；D1/D2 载被拒替代；引用精确到 file:line 与章节条目；与零 spec 增量无矛盾（无可矛盾面）。
4. tasks 通过——T1–T12 全部有来源/验证；无 deferred/未决方案（T12 的「等用户明示提交」是既有流程红线非技术悬置）。
5. 场景覆盖通过——D11 矩阵负例 ~19（六码全族 + E0105/E0402/E1304 复用面）/绿面 ~11/边界 ~3，failure-degradation 面（域外 assertEqual、E1806 保守读）在册。
6. 无空章节通过——四工件无凑格式段。
7. 测试先行通过——T1 黄金先红 + T2 单测先红，红因分类与锁定绿（advancetime-bnd）预期在验证栏。
8. 负向断言通过——E1801/E1802×3/E1803×5/E1804×3/E1805/E1806×4 皆构造违规样例断言拒绝，非文字声明。
9. 完成度闭环通过——三要素齐：类型检查（D3–D8 全量）、代码生成（D9 诚实停点逐形钉死 + MockDecl/advanceTime 不可达论证）、运行时（检查塔下合法 advanceTime ⊆ test extent ⊆ 停点，不可达；运行语义经裁决 Q2/Q3 路由 M10b）——M9a 检查塔同构。
10. 未决问题阻塞通过——proposal 开放问题四项全部在 design 收敛（D5/D9/D11），无阻塞 task。

**发现 F1（已处置）**：design D1 的 Desc 表示法初稿以修辞问句载决议、双形规则（普通串解码字节/插值串原始文本）散落难判——审查中改写为明确的双形陈述，M10b 报告渲染归后续。已改。


## 审查记录（2026-09-08，welang-code-review 7 条，通过；status → complete）

1. **规范符合性 ✓**：抽查对齐已批 ch20 原文——test 块八 Scenario（身份=文件名事实/E1801 于非 test 模块/pub 前缀 E0105/普通模块纪律全持——E1405/E1402 单测钉/体 = fn 体语境 E0402）、mock 需求逐句（产生式段序 `-> Ret` 先于 effect 段=ch20:34 字面、目标解析「chapter 15's resolution throughout, E1304 … E1303 as anywhere」=T5 既有面复用、签名逐字「every parameter with its type, in order and by name, the declared return or its absence, and the effect segment or its absence」=D5 三类目、体=「function body context checked against the declared segment … the segment being the target's own verbatim」=T5 体走查）、advanceTime 四条件与位置规则（「task and scope bodies included, for they are the test's own extent」=testExtent 继承读；闭包/mock 体保守读为 design D7 已披露读）、assertions 面对账（assert = ch14 prelude 零改；assertEqual「resolves as any imported pub fn」的可实现读 = D8 已裁定的 importCall 特型面——同型对面非单调 fn 声明可拼写，channelCall 先例，candidate 审查已过）、诊断段 E1801–E1806 分配在册。无规范外接受/拒绝行为。
2. **验证诚实性 ✓**：T1–T9 逐条复核——本审查期实跑：`go clean -testcache && go test ./...` 九包 ok、conformance 543/543（`ls testdata/cases | wc -l`）、`validate --all --strict` OK、`docs_sync` 31 对齐、`gofmt -l`/`go vet` 清、`git diff --check` 干净；红名单对账链 34→24→23→8→2→0 各任务快照与本会话逐级实跑一致；T8 十二面黑盒探针（含 --json E1806 事件逐字段）本审查期抽复现三面（E1801/E1805/域外 exit 70）。无「勾了没跑」项。
3. **测试先行证据 ✓**：T1 35 枚黄金先红（红因分类在案：bndTest exit 70 ×25、E0105 顶层 mock ×1、E0105 import 形 ×6、bndTaskTime ×2、锁定绿 ×1）；T2 单测编译期红（`undefined: ast.TestDecl` 五位 + `f.IsTestModule undefined`）先于实现；T3–T7 以红名单逐级翻绿为验收；自有黄金/单测实现前修正四处（e1804 锚位 ×3、E1303 钉行）逐处 D13 披露。
4. **诊断协议稳定 ✓**：零新诊断码（六码 T0 前在册）；--json 字段零触碰（internal/diag 零修改）；diagnostics.toml 零触碰（`git status` 证）；六码 helps 与 registry remediation 程序化逐字比对全 MATCH（parser E1801/E1802 + typecheck E1803–E1806）；新边界词 bndAssertEqDomain/bndTestModule 为 D8/D9 预定词汇非协议面。
5. **单一权威 ✓**：长期事实归位路径明确——roadmap 三拆/follow-up #12/#13 随 T11 归档入册；moduleSeg 收关键字的读法为 ch15 张力面已挂 follow-up #13（非静默滞留）；代码注释全英文（新增面逐文件过目）；tasks/design/proposal 中文合规。
6. **红线复核 ✓**：refr/ 零命中；提交待用户明示（T12）；diff 触界——modified 6（ast.go/parser.go/parser_test.go/typecheck.go/typecheck_test.go/advancetime-bnd 随批）+ 新增（m10a_test.go ×2/m10a codegen 单测/35 黄金/变更目录）。**发现 F1（当场处置）**：proposal 影响范围表漏列 `internal/codegen/codegen.go` 与 `internal/cli/build.go`（两者皆 design D9 定夺面——proposal 非目标的「运行时不可达」论证正依赖 D9 停点；表成文于 design 细化前）。处置：表已补两行，本记录披露。
7. **最小可信验证已跑 ✓**：T9 全量（九包 + validate + docs_sync + gofmt/vet + diff --check）+ 本审查期复验（黄金全量、三面黑盒抽复现、helps 逐字比对）。

**结论**：通过，status → complete。
