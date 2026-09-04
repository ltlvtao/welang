# compiler-bootstrap：参考实现切片 1 —— CLI 骨架 + 一致性地基

## Why

规范侧 22 章 + 诊断注册表（153 码 / 22 段）已全部落地（最后的规范侧指名留白 registry/publish 属生态基础设施，刻意留白）；实现侧是当前唯一的大留白：仓库里没有一行编译器代码。ADR-0002 已裁决实现路线（Go 实现、从第一天起产 LLVM IR、自建运行时含精确 GC），环境就绪（go1.26.6、LLVM/clang 21.1.8），但该路线还没有第一个可垂直验证的落点。

工具链章（`docs/spec/2100-toolchain.md` R1 命令面、R7 JSON Lines 协议）固定了 `we` 命令面、全局选项与 E1904/E1907 等命令层诊断——这些契约不依赖词法/解析/代码生成，是编译管线一切阶段的载具。ADR-0002 Consequences 明言评测方法论重心在前端诊断循环（`we check` 不依赖代码生成），而诊断循环的可观察面正是本切片的地基：没有命令脊柱与一致性测试地基，后续每个前端切片都无可验证的落点。

同时，本切片是仓库第一个「实现已批准规范、不改变语言行为」的变更，暴露出 validate.py 与流程文档的一处矛盾：`openspec/README.md` 允许纯内部变更不带 `specs/`，但 validate.py 对 ready+ 状态机械要求至少一个规范增量。该矛盾在本切片必须解决（否则被迫伪造规范增量），修正随本切片落地（用户裁决 2026-09-04：切片 1 取 CLI 骨架 + 一致性地基；里程碑策略取薄垂直优先）。

## 目标与非目标

### 目标

1. Go 模块与版本清单落地：`go.mod`（module `github.com/ltlvtao/welang`、`toolchain go1.26.6` 指令——Go 引脚唯一权威，循 ADR-0002）与 `internal/version`（编译器版本、规范基线标签、LLVM 引脚 21.1.8）。
2. `we` 命令脊柱（第 21 章 R1 的可实现子集）：封闭子命令表（11 个子命令全部注册）；全局选项 `--json` / `--color=auto|always|never` / `--verbose`；`we version`（固定形状，数值在机制层）；`we new <name>`（骨架三件套 + E1904 校验）；路径型子命令的 E1907 检查。
3. JSON Lines 诊断协议（第 21 章 R7）：diagnostic 事件发射器 + 字段稳定性快照测试（既有字段永不删改名、只增）。
4. 一致性黄金用例地基：`internal/conformance` 跑器 + 本切片全量用例（version / new / E1904 / E1907 / 未知子命令 / 未实现边界 / 全局选项）。
5. 里程碑文档 `docs/roadmap/0000-reference-implementation.md`（双语）：薄垂直优先策略、M0–M15 里程碑表、执行状态规则、实现期发现的规范缺口登记——执行状态的唯一权威位置（`docs/README.md` 职责表）。
6. validate.py 内部变更豁免修正 + `openspec/README.md` 状态表同步，使机械执法与流程文档一致。

### 非目标（本变更不改变语言行为，无规范增量、无新诊断码）

- 不实现词法、解析、模块解析、类型/效果/所有权/文档检查、代码生成、运行时——第 1 章起的前端切片按 roadmap 里程碑推进。
- 不实现 E1905（清单缺失）：其触发面是目录路径型管线命令发现清单缺失，本切片无管线命令实现；E1907 先行是因为它独立于管线、在分发前即可判定。
- 不实现 `we build`/`we check`/`we run`/`we test`/`we fmt`/`we vet`/`we doc`/`we clean`/`we lsp` 的本体：它们以诚实的「本参考构建未实现」边界收口（stderr 一行 + 独立退出码，非规范表面），随切片推进逐个替换。
- 不嵌入诊断注册表（`go:embed` 推迟到真实管线阶段一并设计，登记于 roadmap follow-ups）；本切片 E1904/E1907 的标题与补救文本按注册表条目逐字硬编码于调用点。
- 不做 CI 配置、安装脚本、发布物打包；不动 `.githooks/` 与 `.agents/skills/`。
- 不修「项目名精确规则」规范缺口（前导/尾随连字符等无规范依据）：登记为 roadmap follow-up，留待专门修订切片。

## What Changes

- 新增 Go 模块与参考工具链首批包：`cmd/we`（入口）、`internal/cli`（参数解析、子命令分发、退出码）、`internal/diag`（诊断构造与 JSON Lines 编码）、`internal/version`（版本清单）、`internal/conformance`（黄金用例跑器）+ `internal/conformance/testdata/cases/*.json`。
- 新增 `docs/roadmap/0000-reference-implementation.md`（+ 中文对照），创建 `docs/roadmap/` 位置（首次需要，循 `docs/README.md`「不建空目录」规则）。
- 修正 `openspec/tools/validate.py`：提案携带「不改变语言行为」/「无规范增量」标记的行为层变更豁免 ready+ 的 specs/ 机械要求；`openspec/README.md` 状态表同步该豁免。
- `.gitignore` 增补 Go 构建产物。

## 影响层

| 层 | 变更 | 黑盒摘要 |
| --- | --- | --- |
| compiler | 新增 | `we` 命令脊柱实现第 21 章已批准契约的命令层子集（R1 命令面与全局选项、R7 JSON Lines 诊断事件、E1904/E1907）；不改变语言行为 |
| tooling | 新增 | 一致性黄金用例跑器与协议快照测试——验证阶梯「编译器/工具链代码」与「诊断协议」两行的落地载体 |
| process | 修改 | validate.py 内部变更豁免 + openspec/README 状态表注记：消除机械执法与流程文档的矛盾 |
| docs | 新增 | roadmap 双语文档（执行状态唯一权威位置，含本切片裁决记录） |

纯内部/落地类变更理由：本变更实现的是第 21 章已批准的可观察契约，不新增、不修改任何语言行为或诊断码，故无规范增量（specs/ 为空）；validate.py 修正是使机械关卡与 `openspec/README.md` 既有条款（「纯内部重构可不带 specs/，但必须在 proposal 影响层中说明理由」）一致的缺口修复，两层耦合于同一目的（让本类变更有合法路径），不构成无关层拼盘。

## 影响范围

- 受影响：`openspec/tools/validate.py` 的执法行为（所有未来无规范增量的内部变更从此有合法路径）；`openspec/README.md` 状态表一行；`.gitignore`；新建 `docs/roadmap/`。
- 不受影响：`docs/spec/` 全部 23 个双语章节与 `diagnostics.toml`（零改动）；`.agents/skills/`；`.githooks/`；既有归档变更。
- 实现期发现、登记于 roadmap follow-ups（本变更不修）：
  1. 项目名精确规则缺口——第 21 章 R1/R8 与第 22 章 R1 引用「第 1 章命名约定」，但第 1 章 Naming conventions 只固定标识符按绑定形态的大小写规则；TOML 层名字的具体拼写仅散见于第 22 章正文与 E1904 条目 remediation（"lowercase letters, digits, and hyphens"）。实现取「非空且字符集恰 `[a-z0-9-]`」，不发明前导/尾随连字符约束。
  2. 诊断注册表嵌入（`go:embed`）推迟。
  3. 规范未固定的命令层行为——`we new` 目标目录已存在、`we version` 携多余参数、`we` 无子命令调用：实现取 usage 错误（退出 2、stderr、无诊断事件），均为机制层选择。
- 时序说明：validate.py 修正属本变更 process 层范围，但须先于本变更自身的 ready 状态切换落盘（validate.py 以现状会机械拒绝无 specs/ 的 ready 变更）；时序与理由记录于 design.md D10，提交时与变更同库。

## 审计记录

**2026-09-05，welang-spec-impact-audit，七点结论：通过。**

1. 问题真实性 ✓：Why 引用 `docs/spec/2100-toolchain.md` R1/R7/R8、ADR-0002 Consequences（评测重心在前端诊断循环）与 validate.py×openspec/README.md 的现存矛盾（可机械复现：README 允许内部变更免 specs/，validate.py ready+ 强制 specs/）；「仓库无编译器代码」是可验证工程缺口。
2. 影响层声明 ✓：change.yaml `[compiler, tooling, process, docs]` 与 proposal 影响层四行一一对应；行为层无 spec 的边界以「不改变语言行为」明写（非目标标题行与影响层理由段）。
3. 规范增量范围 ✓：无规范增量、无新增/修改/删除的 Requirement 与诊断码（非目标显式声明）；E1904/E1907 是实现既有条目，非新码。
4. 原则一致性 ✓：落地第 0 章原则 7（工具链一致性与机器可操作性——JSON Lines 协议是其实现载体）；机制中立纪律保持（版本数值只入 internal/version 与 roadmap，不入规范文本）。
5. 参考基线固定 ✓：引用均锚定 `docs/spec/` 章节、ADR-0002、diagnostics.toml 条目；refr/ v0.8 仅作 SpecVersion 标签的历史出典，不具权威性。
6. 验收边界 ✓：目标 1–6 皆可机械判定（go build/test、黄金用例、docs_sync、validate 正负探针）；非目标六条显式排除词法/解析/管线/运行时/E1905/CI/规范缺口修复，足以防蔓延。
7. 粒度 ✓：单一目的（实现侧第一个可垂直验证落点 + 其必需的流程关卡修正）；四层耦合于同一目的，影响层已论证，非无关层拼盘。

审计中发现并已处置：validate.py 原实现的豁免标记「非目标」是必填标题「## 目标与非目标」的子串，strict 层检查因此恒真（潜伏空检查）——本次修正收窄标记集为「不改变语言行为」/「无规范增量」，负向探针证实无标记变更现在被正确拒绝（D10 验证记录）。

## 审查记录

**2026-09-05，welang-change-review，十点结论：通过（发现 F1 已处置）。**

1. 职责边界 ✓：proposal 只讲黑盒问题与目标；spec 增量不存在（无规范增量变更）；design 十二条决策唯一、最小、可实施，被拒替代四条带理由；tasks 只执行前三者已定义的工作。
2. spec 增量越界检查 N/A（specs/ 为空，豁免路径已由 T1 正负探针机械验证）。
3. design 引用精确 ✓：第 21 章 R1/R7/R8、第 9/14/15 章、ADR-0002、diagnostics.toml E1904/E1907 条目逐条对应。
4. tasks 格式 ✓：十项各带来源与验证，无 deferred/未决选择。
5. 场景覆盖 ✓：normal（version/new 骨架）、boundary（目录已存在、多余参数、--color 非法值、无子命令、--json×--verbose 组合）、failure-degradation（E1904/E1907/未知子命令/未实现边界 70）三类齐备。
6. 无空章节 ✓。
7. 测试先行 ✓：T3（diag 协议快照）与 T5（黄金用例集）声明先红后绿，red 证据记于完成记录。
8. 负向断言 ✓：T1 无标记假变更必须仍被拒（已跑，双 FAIL）；黄金用例断言未知子命令在 --json 下无事件、E1904 创建零文件。
9. 完成度闭环 N/A（非语言特性变更）：实现类切片的对应义务是验证阶梯全行覆盖，D12 已列。
10. 未决问题 ✓：无阻塞项。SpecVersion "0.9.0" 标签与「项目名精确规则」缺口均为已决策/已登记事项，不影响本变更验收。

F1（审查发现，已处置）：design D7 原文未显式说 JSON Lines 写 stdout、人类诊断写 stderr 的流分工——已补「JSON Lines 写 stdout；人类可读诊断写 stderr；--json 时诊断只以事件形式写 stdout、不重复人类渲染」，黄金用例据此锁定。

## 实现审查记录

**2026-09-05，welang-code-review，七点结论：通过（0 行为发现；披露 D-1–D-4）。**

1. 规范符合性 ✓：R1 封闭子命令表 11 个全注册、集外子命令为 shell 之错（退出 2、stderr、无诊断事件，--json 下亦无事件——黄金 unknown-subcommand-json 钉死）；`[path]` 缺省工作目录；E1907 在分发前触发（`we build nosuchdir` 对边界子命令成立，黄金钉死）；`we new` 校验先于任何写（new-e1904 黄金断言空树）；`we version` 固定形状（四形态黄金）；全局选项每个子命令接受（解析先于分发与边界）。R7 diagnostic 事件字段集与序 = spec 逐字（快照测试逐字节），help 缺省省略，`--json` 不改退出码（E1904 两形态黄金同为退出 1）。usage 类（退出 2）是规范未固定的机制层选择，D6 已声明并归入「shell 之错」同族。
2. 验证诚实性 ✓：十项验证逐条在本会话真实执行（validate 双探针、go build/test/vet、docs_sync、黄金 20 例、git diff --check）；无「勾了没跑」。
3. 测试先行证据 ✓：T3 red（`undefined: Diagnostic`，build failed）、T5 red（internal/cli 缺包，setup failed）输出均在实现前捕获、记于完成记录；黄金期望手写自规范，首绿轮跑器抓出 2 处黄金作者笔误（漏 demo/ 前缀）——修黄金不修码，证明期望不是从实现再生的。
4. 诊断协议稳定 ✓：本切片是协议基线；字段序 type/severity/code/message/file/line/column/help 与 spec 一致；无字段删改；E1904/E1907 为既有码、无新码入册。
5. 单一权威 ✓：版本数值唯一住 internal/version（go 引脚唯一住 go.mod toolchain 指令）；执行状态唯一住 docs/roadmap/0000；规范文本零改动；代码注释、黄金内容全英文。
6. 红线复核 ✓：refr/ 未触碰；提交信息将无署名 trailer（提交后以 git log 复核）；git status 恰为影响范围 8 路径，无越界文件。
7. 最小可信验证 ✓：验证阶梯三行全跑（编译器/工具链：build+test+黄金；诊断协议：快照；工件：validate --all --strict + docs_sync + git diff --check）。

披露（非行为发现）：
- D-1 黄金笔误修正：new-skeleton/new-verbose 两例文件键漏 `demo/` 前缀，由跑器抓出后修正黄金；实现代码零改动。
- D-2 `go run` 模块目录限制：冒烟以 `go build -o` 产物执行；tasks 验证命令均在仓库根，不受影响。
- D-3 时序：validate.py 修正先于本变更 ready 翻转落盘（D10 声明）；同 diff 顺带修复「非目标」子串恒真缺陷（审计记录已记）。
- D-4 边界次序：未实现路径型子命令先 E1907 后 70 边界——规范场景要求路径检查独立于管线，次序选择记录于 D5。
