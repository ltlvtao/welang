# 提案 — fmt-vet-doc（M11）

第 21 章工具链三面的实现里程碑：R3「The formatter」（`we fmt`）、R4「Advisory diagnostics and we vet」（三 W 码 + `[vet]` 表 + E1903）、R9「Documentation generation」（`we doc`）。**纯实现变更，不改变语言行为，无规范增量**——章文与三 W 码注册表在册（W1910/W1911/W1912 owner 2100-toolchain / requirement "Advisory diagnostics and we vet" / allocated 2026-09-04，diagnostics.toml:1496/:1505/:1514），本变更只实现。

## Why

ch21 已批面与 M10c 落地后的差距，逐条可验证：

1. **三子命令是诚实边界**：`fmt`/`vet`/`doc` 在子命令表注册（cli.go:50-52）但 `implemented` 恒假——路径解析后一行 stderr + exit 70（cli.go:117-119）。roadmap M0 承诺的「未实现边界随里程碑收缩至零」剩最后一批。
2. **三告警是死文字**：W1910「unmocked custom effect in a test」（:1496）、W1911「possible indirect nested access to one shared value」（:1505）、W1912「function may block on a wait」（:1514）注册条目完整（触发语义钉在 ch21:73），无任何发射点。
3. **`[vet]` 表无读取面**：三值 `"warning"`/`"error"`/`"ignore"` 与非法值 E1903（ch21:73）——loadManifest 今日只读 name/version/type + `[test].explore-iterations`（check.go:178-190，注释自认「the [vet] posture」为待落）。
4. **提升语义悬空**：「an advisory promoted to error by the manifest's `[vet]` table stops it exactly as an error does」（ch21:30）与场景「`we build` stops as at an error and produces no artifact」（ch21:83）无载体。
5. **格式化器不存在**：ch21:54 钉死规则集（两空格缩进/LF/尾空白剥除/二元运算符间距/import 分组排序/从不折行从不对齐）与定点性（`fmt(fmt(x)) == fmt(x)`）；ch2 落地时的派生不变量「fmt 必须保持行结构」（分号推断使换行承义）等着兑现。
6. **文档生成不存在**：`we doc` 的 pub-only 面、`--check` 建议性发现（工具自有、不入册）、「`///` 单元渲染为 API 文档」（ch21:197）无实现。

工程缺口（非实现偏好）：generate–check–fix 回路的「fix」半环今日只有编译器诊断可用；fmt 给出确定性改写面、vet 给出启发式建议面、doc 给出文档完整性面——三者都是 AI 原生目标（第 0 章）的回路部件。

## 裁决记录（candidate 阶段，2026-09-09，四项均采纳推荐）

**Q1 fmt 遇不可解析文件 = 逐文件独立**。可解析的文件照常格式化写回；不可解析的文件原样不动、报其解析诊断；整体 exit 1。*被拒*：全有全无（一个坏测试文件殃及全项目的格式化，无规范依据）。

**Q2 fmt 项目面文件集 = 项目下全部 `.we`，排除 `build/`**。WalkDir 遍历项目目录全部 `*.we`（src/、tests/ 及任何位置的散置文件），仅排除 build/。fmt 是排版器不是导入图走查器——覆盖面最宽且边界规则只有一条。*被拒*：仅 src/ 与 tests/（引入「哪些目录算项目源」的未规范判断）。

**Q3 doc 渲染格式 = 每模块一个 Markdown 文件**。默认输出到项目 `docs/`，文件镜像模块键（`docs/main.md`、`docs/util.math.md` 平坦点分形）；pub 声明签名 + `///` 单元内容。机器可读、可 diff、AI 原生（P7）。*被拒*：单文件 api.md（大项目膨胀、模块间链接弱）；HTML 站点（不可 diff、与 AI 原生目标不合）。

**Q4 advisory 层运行面 = 全管线命令统一跑**。check/build/run/test/vet 都跑 advisory 分析：warning 态如实报 W 不停管线（R2「a W-severity diagnostic never stops the pipeline by itself」+ R7 check 场景「one error and one warning」的直证读法）；`[vet]=error` 提升为停；`ignore` 抑制。we vet 的特有面 = 退出语义（有未忽略发现即 exit 1）。一台分析、一处权威。*被拒*：仅 we vet 跑（R7 的「check 报一个 error 和一个 warning」场景失去最自然读法，且提升语义需管线命令特判跑不跑）。

**Q4a 的必然后果（如实载明）**：绿径既有黄金中直调五阻塞操作（`.send(`/`.receive(`/`.await(`/`.acquire(`/`.wait(`）的将新增 W1912 行——量面已数：**48 枚**（run-conc-* 27 / check-*green 11 / test-* 10，`grep -l` 于 testdata/cases）。本变更随批更新（T6 专任务、updater 重生成 + diff 逐枚过目披露），不是静默重写：行为变化的授权源 = 本裁决 + 章文 R2/R4。

## 现状与差距

| 面 | 现状（M10c 后） | 差距 |
|---|---|---|
| 子命令表 | fmt/vet/doc 注册 takesPath、implemented 假 → exit 70 | 三命令实装，边界表收缩 |
| 词法 | `lex.Scan` 丢弃 `//` 注释（仅 `///` 经 DocUnit 旁路） | keep-comments 面（fmt 注释保真的单一词法权威） |
| 格式化 | 无 | 行保结构 token 级重排器（缩进/间距/空行/import 分组/定点性） |
| 诊断 | diag 只有 Error 构造器（SeverityWarning 渲染位在） | Warning 构造器 + W 人类行/JSON 双面 |
| advisory | 无 | 三触发收集机（typecheck 侧、共享类型/效果信息） |
| manifest | `[test].explore-iterations` E1903 门 | `[vet]` 三键三值 E1903 门（报文形对齐 M10c 先例） |
| 管线接线 | check/build/run/test 走到 typecheck 止 | advisory 后置段全命令接线（报 W 不停/提升停/抑制） |
| doc | 无 | pub-only 渲染 + `--output`/`--check` |
| conformance | 606 枚 | fmt-*/vet-*/doc-* 新矩阵 + 48 枚随批更新 |

## 目标与非目标

**目标（做完可机械判定）**：

1. `we fmt`：项目面（Q2 文件集 + E1905 一致路径约定）与单文件面；规则集全量落地（两空格缩进、tab 替换、LF、尾空白剥除、顶层项间至多一空行、文件恰一换行收尾、二元运算符两侧单空格、逗号后单空格、冒号后单空格、`->` 两侧、花括号 `{ `/` }`（空块 `{}`）、import 分组排序 std.* 先 + 一空行隔组）；从不折行从不对齐；定点性（输出再 fmt 零改）；Q1 逐文件独立（解析失败 = 该文件不动 + 诊断 + exit 1）。
2. `we vet`：check 管线 + advisory 层；退出 0（无未忽略发现且无 E）/ 1（E 停或有未忽略发现）；无工件产出。
3. 三触发全量：W1910（test 体内调用自定义效果 fn 且同块无该目标 mock——mock 装填于块始、块内任意 mock 覆盖全块调用）；W1911（同步绑定回调体内、以该绑定为实参的调用——一次调用穿透的保守面）；W1912（五种阻塞操作的类型导向直调——接收者类型恰为对应原语）。
4. `[vet]` 表：三键三值；非法值 E1903（报文 `invalid toolchain configuration value — vet.{code} must be "warning", "error", or "ignore", got {v}`，check/build/run/test/vet 五面共享 loadManifest 直证）；提升 = severity 渲染 error + 停管线（build 无工件、test 编译败 exit 2）。
5. advisory 全管线接线（Q4）：check/build/run/test 报 W 不停；既有 48 枚绿径黄金随批更新（T6 逐枚披露）。
6. `we doc`：pub-only 面；默认 `docs/`、`--output DIR` 重定向、`--check` 只验不生成（未文档化 pub 声明 = 工具自有建议行，stderr 平文、不入册、不受 --json 影响——披露）；每模块一 Markdown（Q3）；check 管线先行（E 停无页面）。
7. 既有 606 枚 conformance 黄金除 Q4a 载明的 48 枚随批更新外**零回归**。
8. conformance 新黄金矩阵（35 枚，T1 定数）+ lex/fmt/advisory/cli 单测先行。

**非目标（显式栅出）**：

- fmt 的行宽/风格预设/`--check`/`--diff` 旗标——R3 零配置承诺 + 不折行不对齐；无任何 fmt 选项。
- fmt 移动 token 跨行（import 整声明重排是规范钦定的唯一例外）；fmt 对不可解析文件的「尽力修复」——Q1 已裁该文件不动。
- advisory 的路径敏感分析（mock 覆盖按块全量判定——mock 装填于块始的运行事实）；W1911 的传递闭包（恰一次调用穿透——章文自钉「looks one call through」）。
- 工具自有发现入册（fmt 发现、doc --check 缺口——章文明言「this chapter registers none of them」）。
- doc 的交叉引用解析与渲染美学（「the tool's rendering business」）；doc 对 std 内建模块的文档页（编译单元的自有模块面）。
- LSP（M14）、依赖解析（M13）、性能承诺（R11 一无所有）。

## What Changes

- **internal/lex**：keep-comments 面（注释为 trivia token 的第二入口——单一词法权威，parser 面零改）。
- **internal/fmt（新包）**：行保结构 token 级重排器（深度缩进、邻接间距表、空行策略、import 分组整声明重排）；定点性由构造保证、黄金钉死。
- **internal/typecheck**：advisory 收集机（三触发、共享既有类型/效果/绑定信息）；`///` Docs 已在 AST（ast.go:22）。
- **internal/diag**：Warning 构造器（SeverityWarning 渲染位已有，diag.go:22）。
- **internal/cli**：三子命令实装（runFmt/runVet/runDoc）；`[vet]` 表读取与 E1903；doc 的 `--output`/`--check` 旗标；advisory 后置段在 check/build/run/test 的接线。
- **internal/conformance**：fmt-*/vet-*/doc-* 黄金矩阵 + 48 枚随批更新。
- 单测：lex keep 模式 / fmt 重排 / advisory 触发 / cli 面。

## 影响层

`change.yaml` layers = `[compiler]`。全部 Go 侧（cli/lex/typecheck/diag/conformance）；不触 runtime/c（advisory 是编译期分析、fmt/doc 是工具面）。**不改变语言行为，无规范增量**——本变更不触 docs/spec/ 任何文件；三 W 码注册表在册，E1903 条文自点名 `[vet]` 表（description「configurable in the manifest's `[vet]` table」）。

## 影响范围

涉触码盘点（proposal 层面，design 细化）：

- `internal/cli/cli.go`——子命令表三行 implemented 翻真；选项解析（doc 的 --output/--check）。
- `internal/cli/check.go`——loadManifest 增 `[vet]` 三键校验（E1903）；parseManifest 复用 M10c section 前缀机制。
- `internal/cli/fmt.go`（新）、`vet.go`（新）、`doc.go`（新）——三命令体。
- `internal/cli/check.go`/`build.go`/`test.go`——advisory 后置段接线（报 W 不停/提升停）。
- `internal/lex/lex.go`——keep-comments 入口。
- `internal/fmt/`（新包）——重排器。
- `internal/typecheck/`——advisory 收集（新文件）。
- `internal/diag/diag.go`——Warning 构造器。
- `internal/conformance/testdata/cases/`——新黄金 + 48 枚更新。
- 单测新文件：lex/fmt/typecheck/cli 各一。

风险与缓解：

1. **48 枚黄金 churn 是本变更最大表面**——T6 专任务、updater 重生成后逐枚 diff 过目、每枚的 W 行增改与触发源对账；红因先行（T1 新黄金红 + T4 落地时 48 枚转红的预期红分类记录）。
2. **fmt 邻接间距表的完备性**——规范只钉载重规则，未点名的邻接（`=>`、闭包 `|`、属性 `#[`、后缀链）由间距表机制补全；表本身进 design D 逐对枚举，黄金矩阵按对覆盖。
3. **advisory 与 typecheck 的信息共享**——不做二次推断（两处权威之忌）；收集机挂在既有 checker 体内走查点位。

## 审计记录（2026-09-09，welang-spec-impact-audit 7 条，通过；status → ready）

1. 问题真实性 ✓——六条差距全部行锚可验证：ch21:52-69（R3 规则集在册无实现）、ch21:71-93（R4 三触发在册无发射点）、ch21:195-207（R9 在册无实现）、diagnostics.toml:1496/:1505/:1514（三 W 码注册条完整）、cli.go:50-52 + :117-119（三子命令诚实边界）、check.go:178-190（[vet] posture 注释自认待落）。
2. 影响层 ✓——change.yaml layers [compiler] 与本文一致；纯内部变更边界已写明（「不改变语言行为，无规范增量」），validate 豁免路径过。
3. 规范增量范围 ✓——零新增零修改零删除 Requirement；零诊断码分配（三 W 码 2026-09-04 已 allocated，E1903 在册复用）；与既有规范零冲突。
4. 原则一致性 ✓——零冲突；P7（零配置）恰由 R3 的落地兑现而非突破；无新隐式转换、无新语法面。
5. 参考基线 ✓——零 refr/ 引用（纯 ch21/ch99 已批文实现）。
6. 验收边界 ✓——八个目标皆可机械判定（退出码/byte-exact/定点性双跑断言/计数 48 对账）；非目标九条栅出（fmt 旗标、路径敏感 advisory、传递闭包、入册、交叉引用、std 页、LSP、依赖、性能）。
7. 粒度 ✓——M11 是 roadmap 钉死的单行里程碑（一里程碑一变更）；三面共享 [vet]/advisory 基建（vet 与 advisory 不可拆），fmt/doc 虽可独立但属同一工具链里程碑承诺（M10 拆三先例用于塔级巨变更，本变更三面合计量级不到）。

validate.py fmt-vet-doc --strict：OK。

## 审查记录（2026-09-09，welang-change-review 10 条，通过；status → active）

职责边界四条：
1. proposal 黑盒性 ✓——Why 六差距全为可观察行为（子命令 exit 70 / W 死文字 / [vet] 无读取 / 提升悬空 / fmt·doc 不存在）；裁决记录承载表面选择（M10a/b/c 先例）；What Changes 仅为涉触码盘点。
2. 规范增量零豁免路径 ✓——本变更零增量（validate 过）；三 W 码与 E1903 皆在册复用，零新分配。
3. design 唯一最小路径与被拒方案 ✓——fmt 行保结构（AST 全量重渲染被 ch2 派生不变量排除、纯 token 面被注释丢弃排除——D1 载明）、advisory 挂既有走查点位（独立遍历被两处权威之忌排除——D4 载明）；Q1–Q4 被拒选项在裁决记录逐条在案。
4. tasks 全项有来源/验证 ✓——T1–T11 皆双行（来源 = proposal 目标/design D；验证 = 可跑命令 + 对账面）；无 deferred/未决项混入。

内容质量六条：
5. 场景覆盖 ✓——normal（fmt 脏→净/已净、vet 净项目、doc 生成）/ boundary（空文件、单文件面、E1905、空块、已对齐间距）/ failure-degradation（fmt 解析失败逐文件独立、[vet]=error 提升停、E1903、doc E 停无页面、usage 2）三类皆有黄金钉。
6. 无空章节 ✓——四工件全实；D6 修辞残留（自问自答句）与 D3 空文件边界（零字节零注换行）当场修正。
7. 测试先行 ✓——T1 黄金先红（红因 = exit 70 not implemented）+ T2 单测先红（符号未定义），D9 红绿账定序。
8. 负向断言真实 ✓（审查揪出两缺口当场补）——「无任何 fmt 选项」缺负向钉：既有 606 枚零 unknown-option 黄金（仅 m10c 单测钉 check 面）→ D7 补 fmt 坏旗标 usage 2 枚；「从不折行从不对齐」缺证明源 → fmt-pass 综合脏源补明含超长行+手工对齐字段。ignore 抑制/E1903/doc --check 缺口等负例原已矩阵内。
9. 完成度闭环 ✓——工具链变更（非语言特性），闭合面 = 三 CLI 命令 + 黄金矩阵 + 单测；原则 10 三要素（类型/代码生成/运行时）本变更零新语言面，无未闭合项。
10. 无行为阻塞开放问题 ✓——四裁决全定；审查三修正（坏旗标枚/不折行证明源/D6 措辞与空文件边界）已落 design/tasks，计数 ~33 → ~35（T1 定数披露机制在）。

validate.py fmt-vet-doc --strict 复验：OK。

## 实现审查记录（2026-09-09，welang-code-review 7 条，通过；status → complete）

1. **规范符合性** ✓——本变更零规范增量（`openspec/changes/fmt-vet-doc/specs/` 无目录，validate 亦证）；W1910/W1911/W1912/E1903 皆在册复用（diagnostics.toml :1496/:1505/:1518/:1898 区段既有条目，本变更零新分配、`git status docs/` 零改动）；三命令行为面（fmt 逐文件独立/vet advisory 出口/doc pub-only 页）皆有黄金逐字钉，无规范外行为。
2. **验证诚实性** ✓——T1–T8 逐项核对：T1–T4 完成记录在案（含 D13 两笔误披露、46 枚预期红机械校验），T5–T8 本审查前 freshly 复跑（cli 单测绿、doc-\* 5 枚绿、conformance 全量 641 绿、九步阶梯全绿）；无「勾了没跑」项。
3. **测试先行证据** ✓——T1 黄金先红（红因三类记录在案）+ T2 单测先红（四包符号未定义编译红），tasks.md 载红因与笔误当场修正披露；黄金输出符合 §52（`file:line:col: warning[W1912]: message` 人类形）与 §62（--json 事件七字段 schema，vet-json-w-event + test-explore-json 双钉）。
4. **诊断协议稳定** ✓——diag.go diff 纯增量（Warning/AsError 两构造器，JSON/Human 渲染面零字段删改；warning 渲染形由既有 severity 分支自然承载）；提升面 AsError 保 W 码渲染 error（一码一严重度在注册侧不动，严重度是渲染层——注释载明）。
5. **单一权威** ✓——长期事实零滞留：三命令行为由既有第 21 章规范钉面承载（M11 是实现 slice 无新机制）；代码注释全英文（em-dash/省略号为仓库标点惯例，parser.go 392 处先例；十一个新/改代码文件零中文字符扫描过）。
6. **红线复核** ✓——refr/ 零触碰（`git status | grep refr/` 零命中）；未提交（提交候用户明示，T11）；diff 无越界（modified 54 + 未跟踪与 proposal 涉触码盘点逐项对齐；唯一盘点外项 = M10c 归档 tasks.md 遗留一笔，T4 已披露随行）。
7. **最小可信验证已跑** ✓——T8 D8 九步固定序全绿（build/vet/test/runtime harness -count=1/conformance fresh -count=1/validate --strict/docs_sync 31 对/gofmt -l 零/git 对账）。

发现与处置：无打回项。两处记录期勘误已在案——T4 桶分计数混入失败总数（T6 机械清点定数 46 = 29+8+8+1 纠正）；proposal Q4a 预估 48 vs 实 46 偏差定数披露（T6）。
