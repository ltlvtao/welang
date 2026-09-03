# Proposal: dual-language-docs

## Why

本项目的交互与协作语言是中文，而 `docs/` 目前只有英文一种文本——中文读者尚不能以自己的语言直接阅读规范正典与流程权威文档。另一方面，英文有不可替代的价值：规范增量以英文片段书写、commit 英文、术语与诊断标识符是英文，且英文规范对 LLM 生态与外部世界更友好。单一语言无法同时覆盖两侧需求。

结论是对 `docs/` 提供**双语同步支持**：英文与中文并行维护、同步演进，两类读者各自以母语直接阅读全部文档。但**无纪律的双语会漂移**——两份文档渐行渐远，最终一方成为没人维护的化石。漂移必须从第一天就受控，且必须赶在第一个大语料（`docs/spec/`）诞生之前建立——事后给一个大规范补翻译、补纪律，成本高一个量级。

## 目标与非目标

目标：

1. **确立双语约定**：`docs/` 下每个 `*.md` 都有同目录的兄弟翻译 `<name>.zh.md`；**英文为权威文本，中文为同步翻译**；任何不一致以英文为准，直至翻译被修复。
2. **机械防漂移层**：新增 `openspec/tools/docs_sync.py`——校验配对存在性（无缺翻译、无孤儿翻译）、标题层级数量一致、围栏代码块数量一致；纳入验证阶梯。
3. **语义防漂移层**：翻译同步进入每个 docs 层变更的完成定义（change-review / code-review 把关）。
4. **翻译现有文档**：`docs/README.md`、`docs/process/development-process.md` 两个文件补齐 `.zh.md`。

非目标：

- 不改变 `AGENTS.md`、`openspec/`、`.agents/skills/` 的语言（中文单语，它们本来就是中文面）。
- 不翻译 commit 信息与代码注释（维持英文单语）。
- 不引入机器翻译流水线——翻译由变更内的智能体产出并经评审；工具只查结构。
- 不做 `docs/` 之外任何位置的双语。

## What Changes

- **约定**（记入 `docs/README.md` 新章节"Documentation language"）：
  - 配对规则：`<name>.md` ↔ `<name>.zh.md` 同目录；翻译文件继承主文件全部命名元素（四位数字前缀、ADR 编号），仅追加 `.zh.md` 后缀；
  - 权威规则：英文权威，冲突时以英文为准；
  - 完成定义：docs 变更的 done = 英文与中文同时更新且 `docs_sync.py` 通过。
- **新工具** `openspec/tools/docs_sync.py`：结构校验（配对存在、标题层级计数、围栏块计数），退出码 0/1；接入验证阶梯。
- **流程文本修订**：`AGENTS.md` 硬规则 2（语言约定）改为"docs/ 英文权威 + 中文同步翻译"；验证阶梯表 docs 行增加 `docs_sync.py`；`docs/process/development-process.md` 验证阶梯与文档语言规则同步；§7 增补一句推广目标区分——语言行为能力提升到 `docs/spec/`，其他能力提升到 proposal 声明的权威位置（本变更即 `docs/README.md`）。
- **校验器缺陷修复（实施中发现，已并入范围）**：`openspec/tools/validate.py` 严格模式对已勾选任务验证内容的提取存在双重 partition 缺陷——验证文本不含 ASCII 冒号时被误判为空验证，阻塞一切变更到达 `complete`。修复为按前缀剥离；正负两向断言见任务 8。
- **翻译落地产物**：`docs/README.zh.md`、`docs/process/development-process.zh.md`。

## 影响层

- **process**：`AGENTS.md` 硬规则 2 与验证阶梯表；`docs/process/development-process.md` 验证阶梯与新增文档语言规则。
- **docs**：`docs/README.md` 新增语言约定章节；现有两个文档的 `.zh.md` 翻译。

## 影响范围

- **所有后续 docs 层变更**：完成定义扩大为双语同步；结构性漂移由工具拦截，语义漂移由评审拦截。
- **`ratify-language-foundations`（依赖声明）**：本变更应在其进入 active 前完成，使其归档产出的 `docs/spec/0000-principles.md` 天然携带 `.zh.md` 兄弟文件。
- **不受影响**：`AGENTS.md` / `openspec/` / `.agents/`（中文单语不变）；commit 与代码注释（英文单语不变）；编译器/工具链代码（不存在）。
- **代价**：docs 写作与评审工作量约翻倍；结构校验无法覆盖语义漂移，剩余风险由评审纪律承担。

## 审计记录

**2026-09-03 · welang-spec-impact-audit · 通过（附带一项范围补充）**

1. 问题真实性：✅ 工程缺口可验证——`docs/` 现无任何 `.zh.md`，且无同步校验工具；引用基线为 `docs/README.md` 语言行与 `AGENTS.md` 硬规则 2（HEAD d53ff57）。
2. 影响层声明：✅ `change.yaml` layers [process, docs] 与 proposal 影响层一致；不触及 compiler/stdlib/tooling，不改变语言行为。
3. 规范增量范围：✅ 新增能力 `docs-language`，4 条 Requirement（配对翻译、英文权威、结构同步校验、完成定义），全部 ADDED；**不涉及诊断码**。
4. 原则一致性：✅ 不触及语言行为；机械校验扩展与"工具链一致性"取向同向。无原则例外，无需 ADR。
5. 参考基线固定：✅ 未引用 refr/ 材料；流程文本引用固定到文件与小节号。
6. 验收边界：✅ 每项目标可机械判定（工具存在且退出码正确、两个 `.zh.md` 存在且通过校验、约定成文且与增量一致）；非目标显式排除机器翻译流水线与 `docs/` 之外范围。
7. 粒度：✅ 单一关切（双语文档体系）；process+docs 两层因同一约定强耦合，可垂直验证。

范围补充（审计发现的流程缺口）：specs/ 增量的推广目标需区分语言行为能力与非语言能力——已在 What Changes 增补 development-process.md §7 一句，任务 3 落实。开放问题无遗留。

## 审查记录

**2026-09-03 · welang-change-review · 通过（修复 1 项设计缺陷后）**

1. proposal 职责：✅ 仅黑盒问题/目标/范围；`docs_sync.py` 之名与其检查项在 spec 增量中同为黑盒接口（可观察的检查行为与退出码），非实现细节；实现决策全部在 design。
2. spec 增量职责：✅ 全部为触发/可观察结果；无 owner、代码路径、迁移过程；BCP 14 用法仅 MUST/MUST NOT，无 SHOULD/MAY（无可选行为）。
3. design：✅ 五个决策唯一且可实施；被拒方案（平行树、单文件双语、并入 validate.py）均有理由；与增量无矛盾——**审查中发现并修复一处缺陷**：标题识别未排除围栏区，围栏内 `#` 注释行会被误计为标题，翻译示例内注释即触发误报；已改为两遍解析（先切围栏、围栏外数标题），并记录已知局限。
4. tasks：✅ 全部映射 proposal/design 已定义工作；无 deferred、无未决选择；任务 1 验证已随缺陷修复强化（增加"围栏内 `#` 行不视为标题"的正向对照夹具）。
5. 场景覆盖：✅ normal（新增文档、干净态）、boundary（孤儿翻译、缺配对）、failure（标题漂移、渲染分歧、完成被阻塞）均在增量中。
6. 空章节：✅ 无。
7. 测试先行：✅（非编译器行为变更）负向/正向断言内嵌于任务 1 的夹具验证，先于任何仓库内落地。
8. 负向断言：✅ 四个负向夹具为真实构造的违规样例并断言退出码 1，非文字描述。
9. 完成度闭环：N/A——非语言特性变更，不涉及类型检查/代码生成/运行时三要素；已确认无诊断码。
10. 未决问题：✅ 无遗留（审计阶段的 §7 推广目标问题已并入范围并落任务 3）。

**范围增补（实施中发现）**：任务 7 全量关卡暴露 `validate.py` 严格模式的验证内容提取缺陷（详见 What Changes 与任务 8），随本变更修复——它阻塞本变更自身的 `complete`，且属同一机械强制层，独立变更的流程开销与缺陷规模不成比例。同时更正任务 2 验证命令的大小写拼写（`^## Documentation Language`）。

**2026-09-03 · welang-code-review · 通过（补齐一处验证执行后）**

1. 规范符合性：✅ 实现与 `docs-language` 四条 Requirement 一致——配对/孤儿检测、标题层级与围栏计数、退出码语义均经夹具实测（任务 1 五例）；权威与完成定义表述已进入三处权威文本。
2. 验证诚实性：✅（审查中补齐）任务 2 的更正版验证命令此前未实际运行，本次补跑（输出 1）；任务 3 的 §7 句断言（`docs/spec/` ×4、`authoritative location` ×2）与任务 4 的硬规则 2 短语断言（`英文为权威文本`、`<basename>.zh.md`）一并补跑通过。其余任务验证均有本会话实际执行记录。
3. 测试先行：N/A（非编译器行为变更）；负向/正向夹具断言先于仓库内任何落地产物完成。
4. 诊断协议：N/A——未触及任何诊断码与 `--json` 字段。
5. 单一权威：✅ 约定权威表述已落 `docs/README.md`（"Documentation Language"）、`development-process.md` §4.1、`AGENTS.md` 硬规则 2；变更目录仅存过程工件。`docs/` 英文 + 授权的 `.zh.md` 翻译，无中文混入英文正文。命名三条规则的权威落点属 `ratify-language-foundations` 范围，不滞留本变更。
6. 红线复核：✅ `refr/` 未入库且未 staged；本变更尚无 commit；diff 无越界改动（`.gitignore` 增 `__pycache__/` 一行为披露的卫生项，防导入校验器时产生噪音）。
7. 最小可信验证：✅ 验证阶梯"仅文档/流程/openspec 工件"行三项全部通过（validate --strict / docs_sync / git diff --check），staged 无 `refr/`（尚无 staged 内容）。
