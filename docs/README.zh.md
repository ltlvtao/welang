# welang 文档索引

## 文档原则

welang 文档遵循"一类事实只有一个权威位置"。语言规范定义语言行为，ADR 记录长期取舍，roadmap 管理执行状态，流程文档治理工作方式。这些位置之间不重复任何内容。

`docs/` 下的文档提供双语同步支持：英文文本配对中文翻译（`<basename>.zh.md`）——见[文档语言](#文档语言)一节。`docs/` 之外的智能体操作文件（`AGENTS.md`、`openspec/`、`.agents/skills/`）使用中文；交互语言为中文。

## 文档语言

`docs/` 下的文档提供双语同步支持：

- 每个 `*.md` 文档在同目录下有配对的中文翻译，命名为 `<basename>.zh.md`。翻译文件继承源文件的全部命名元素，包括数字前缀与 ADR 编号。
- 英文为权威文本。同一文档的英文与中文渲染不一致时，以英文为准；翻译在引入分歧的同一变更内修正。
- 双语同步是变更完成定义的一部分：创建或修改 `docs/` 下文档的变更，在其翻译反映同一变更且 `openspec/tools/docs_sync.py` 通过之前，不得进入 `complete`。
- `docs_sync.py` 只做结构检查（配对存在性、标题层级计数、围栏代码块计数）。翻译的语义等价由变更评审承担，不由脚本承担。

`docs/` 之外的文件按设计为单语：`AGENTS.md`、`openspec/`、`.agents/` 为中文；代码注释与提交信息为英文。

## 文档命名约定

数字前缀在顺序要紧处编码阅读顺序；语义名在跨文件引用要紧处承载身份：

- **设计语料用位置编号。** `docs/spec/` 下的正典规范章节文件为 `<nnnn>-<slug>.md`——四位零填充、步长 100（`0000`、`0100`、`0200`……），新章节可在相邻编号间插入而无需重排。`0000` 保留给规范第 0 章（基础与原则）；横切注册表类章节从尾部计数（`9900` 为诊断码注册表章），内容章节向上生长、注册表章节锚定尾部，互不相撞。ADR 使用同样步长：`ADR-<nnnn>-<slug>.md`。翻译文件继承全部命名元素（见"文档语言"）。
- **只追加账本用日期前缀。** 变更目录移入 `openspec/changes/archive/` 时改名为 `YYYY-MM-DD-<name>`——日期即全序，无需并发协调，且 `validate.py` 不校验 `archive/`。
- **语义身份用语义名。** active 变更目录与 `specs/<capability>/` 能力名保持 kebab 语义名——变更名是跨工件引用的连接键，能力到章节文件的映射在归档提升时决定，不提前焊死。
- **固定结构名豁免。** `README.md`、`AGENTS.md`、`SKILL.md`、`change.yaml` 与四件套工件文件名永不带前缀——工具与跨工具约定依赖这些名字。机器工件同样豁免：`docs/spec/diagnostics.toml`（诊断码注册表）为固定结构名——仅英文、无配对翻译，由编译器、工具与 `validate.py` 直接解析。

## 目录职责

| 目录或文件 | 权威职责 | 不承载 |
| --- | --- | --- |
| [`AGENTS.md`](../AGENTS.md) | 智能体入口（跨工具标准）：硬规则、语言不变量、工作流摘要、验证阶梯、提交协议、环境设置 | 语言需求、roadmap 状态、流程细节 |
| [`docs/process/`](process/development-process.md) | 研发流程（工作方式的唯一权威） | 语言行为、变更级决策 |
| `docs/spec/` | 规范正典，带版本；含机器可读诊断码注册表 `diagnostics.toml`（每个已分配码的唯一条目权威） | 实现决策、迁移历史、执行状态 |
| [`docs/lsp.md`](lsp.zh.md) | LSP 绑定文档（第 21 章 R12 的 separate document）：协议基线、传输、生命周期、诊断映射 | 诊断之外的编辑器能力、任何新诊断码 |
| `docs/decisions/` | 长期 ADR：状态、依赖、被拒选项（首个 ADR 需要时创建） | 需求、任务清单 |
| `docs/roadmap/` | 战略、里程碑、变更目录、执行状态（roadmap 超出 `openspec/changes/` 所能承载时创建） | 需求、字段级设计 |
| [`openspec/`](../openspec/README.md) | 变更管理：active/archived 变更及其 proposal/spec-delta/design/tasks 工件；仓库工件校验器（`validate.py` 查变更结构，`docs_sync.py` 查双语配对） | 任何事实的长期权威（变更是过程增量） |
| [`refr/`](../.gitignore) | 私有参考材料（spec v0.8 草案与评审）。永不提交——最高优先级规则，钩子强制 | 任何权威；`refr/` 中没有内容定义语言行为 |
| [`.agents/`](../.agents/) | 项目研发技能（生命周期评审关卡），工具无关 | 流程权威（在 `docs/process/`）、产品运行时资产 |
| [`.githooks/`](../.githooks/) | 版本化 git 钩子（机械强制：`refr/` 禁令、署名禁令），经 `core.hooksPath` 激活 | 任何无法在 git 层强制的策略 |

## 阅读路径

### 每个变更

1. 阅读 [`AGENTS.md`](../AGENTS.md)。
2. 阅读[研发流程](process/development-process.zh.md)。
3. 按上表加载与变更目标相关的最小文档集。

### 语言行为变更

1. [研发流程](process/development-process.zh.md)
2. `docs/spec/` 下的当前规范正典（最新版本）
3. 相关的 `openspec/changes/<name>/` 工件

### 工具链 / 验证

1. [研发流程](process/development-process.zh.md)验证阶梯一节
2. 受影响的包或模块文档

## 更新规则

- 流程变更走触及 `process` 层的 openspec 变更，然后提升进 `docs/process/`。
- 语言行为变更走带规范增量的 openspec 变更；归档时增量提升进 `docs/spec/`。
- 长期取舍在归档时提升为 `docs/decisions/` 下的 ADR。
- 不创建空目录或占位文档；在真实内容首次需要时才创建位置。
