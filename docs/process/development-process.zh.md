# welang 研发流程

本文档是 welang 仓库中工作方式的唯一权威。它改编自 openTalon 项目使用的 AI 辅助研发流程（规范驱动变更、技能关卡评审、机械校验），并按规范先行的语言项目重塑。

## 1. 目的与原则

welang 研发一门 AI 原生编程语言。产品链是：**规范正典 → 参考编译器 → 标准库 → 工具链 → 评测基准**。质量依赖以下按序排列的原则：

1. **规范先行。** `docs/spec/` 下的正典规范是语言行为的唯一权威。没有对应 active 变更规范增量的编译器或工具链代码，不得改变语言行为。
2. **一类事实只有一个权威位置。** 语言行为只存在于规范中；取舍只存在于 ADR 中；执行状态只存在于 roadmap 中；流程规则只存在于本文档中。变更是过程增量，永远不是长期权威。
3. **验证后再勾选。** 变更中的每个任务必须携带验证（精确命令 + 预期结果）。验证未实际运行且通过，任务不得标记完成。
4. **编译器行为测试先行。** 改变行为的编译器工作从会失败的目标测试（含黄金诊断用例）开始，然后才是实现。
5. **小而可垂直验证的变更。** 禁止创建"实现整个编译器"或"合并整个评审"式的变更。每个变更必须可独立验收。
6. **能机械强制处尽量机械强制。** 可由脚本或钩子检查的规则就由脚本或钩子检查；模型纪律是补充，永不替代机械关卡。

## 2. 变更生命周期

```
candidate ──(规范影响审计)──▶ ready ──(四件套 + 变更评审)──▶ active
    ──(实现 + 验证 + 代码评审)──▶ complete ──(归档同步)──▶ archived
```

| 状态 | 含义 | 进入条件 | 退出条件 |
| --- | --- | --- | --- |
| `candidate` | 有问题陈述的想法，尚未承诺 | `proposal.md` 草稿 + `change.yaml` 存在 | 通过规范影响审计（技能：`welang-spec-impact-audit`） |
| `ready` | 范围、影响层与验收边界已固定 | 审计通过 | 四件套齐全且通过语义评审（技能：`welang-change-review`） |
| `active` | 实施中 | 变更评审通过 | 全部任务验证完成，代码评审通过（技能：`welang-code-review`） |
| `complete` | 已实现并验证，等待基线提升 | 代码评审通过 | 基线提升完成（技能：`welang-archive-sync`） |
| `archived` | 完成；长期事实已提升；目录移入 `openspec/changes/archive/` | 提升完成 | — |

状态变迁记录在变更的 `change.yaml`（`status` 字段）。校验器按状态强制工件完备性。

## 3. 变更工件

每个变更位于 `openspec/changes/<kebab-name>/`，由以下工件组成：

| 工件 | 职责 | 不包含 |
| --- | --- | --- |
| `change.yaml` | 名称、状态、影响层 | 其他任何内容 |
| `proposal.md` | 为什么需要该变更；目标与非目标；改什么；影响层与影响范围 | 需求、设计决策、任务清单 |
| `specs/<capability>/spec.md` | 黑盒规范增量：带场景（WHEN/THEN）的 Requirement | 实现、owner、代码路径、迁移历史 |
| `design.md` | 白盒决策：选定方案、被拒替代、受影响不变量、验证策略 | 需求、任务清单 |
| `tasks.md` | 可验证的工作单元；每个勾选项携带来源与验证 | 未定义的工作、延后项、未决选择 |

### 3.1 `change.yaml`

```yaml
name: <kebab-name>        # 必须与目录名一致
status: candidate         # candidate | ready | active | complete | archived
layers: [spec]            # 取值子集：spec | compiler | stdlib | tooling | benchmark | process | docs
```

### 3.2 `proposal.md`（必需标题）

- `## Why` —— 黑盒方式描述的问题
- `## 目标与非目标` —— 目标与显式非目标
- `## What Changes` —— 增量摘要
- `## 影响层` —— 影响层及各层黑盒变更摘要
- `## 影响范围` —— 其他受影响或显式不受影响的内容

### 3.3 规范增量

增量使用章节标题 `## ADDED Requirements` / `## MODIFIED Requirements` / `## REMOVED Requirements` / `## RENAMED Requirements`。每个 `### Requirement:` 必须至少有一个含 `**WHEN**` 与 `**THEN**` 的 `#### Scenario:`。强制义务用 MUST / MUST NOT（BCP 14）；允许偏离用 SHOULD 并写明条件；可选项用 MAY 并写明默认值。

改变语言行为的变更 MUST 携带规范增量。只影响内部实现的变更（重构、测试基础设施）MAY 省略 `specs/`，但 MUST 在 `## 影响层` 中以 `compiler`（或类似）层声明并说明理由。

### 3.4 `tasks.md`

每个勾选项（`- [ ]` / `- [x]`）后必须紧跟两行：

```
来源：<proposal / requirement+scenario / design 章节>
验证：<精确命令 + 预期结果>
```

规则：

- 已勾选（`- [x]`）但验证行为空，是校验失败。
- 验证未实际运行不得勾选——这是智能体纪律，由 `welang-code-review` 把关。
- 禁止路径（变更不得做的事）需要真实的负向断言，不能用文字描述代替。
- 影响行为/契约/验收的未决问题 MUST 阻塞相关任务；不得用 SHOULD/MAY/TODO 掩盖。

## 4. 验证阶梯

提交或推送前，按实际 diff 选取最小可信集合（技能：`welang-pre-push-checks`）：

| diff 范围 | 必跑验证 |
| --- | --- |
| 仅文档 / 流程 / openspec 工件 | `python3 openspec/tools/validate.py --all --strict`；`python3 openspec/tools/docs_sync.py`；`git diff --check`；确认 staged 无 `refr/` |
| 语言规范（`docs/spec/`） | 规范一致性复核：诊断码全局唯一、§ 交叉引用有效、术语与既有章节一致；外加上一行全部 |
| 编译器 / 工具链代码 | `go build ./...`；`go test ./...`；受影响的 conformance 黄金用例 |
| 诊断协议 | 协议快照测试（JSON Lines 字段稳定性） |

验证命令自第一天起固定，只能经 process 层变更修改。

### 4.1 文档语言与双语同步

`docs/` 下的文档提供双语同步支持：**英文为权威文本**，每个文档在同目录携带配对中文翻译 `<basename>.zh.md`。创建或修改 `docs/` 下文档的变更在同一变更内更新其翻译；`openspec/tools/docs_sync.py`（配对、标题层级计数、围栏块计数）必须在变更进入 `complete` 前通过。该约定的权威表述位于 `docs/README.md` 的 "Documentation Language" 一节。

## 5. 提交协议

- 提交信息用英文，解释**为什么**，不罗列文件。
- 禁止署名 trailer（`Co-Authored-By`、`Generated with ...` 等）——由 `commit-msg` git 钩子强制。
- 决策上下文 trailer 允许且鼓励：`Constraint:` / `Rejected:` / `Tested:` / `Not-tested:`。
- 一个变更对应一个或一组可垂直验证的提交。

## 6. 强制模型

四层，从强到弱：

1. **Git 钩子（机械、工具无关）。** 版本化钩子位于 `.githooks/`，每个克隆一次性激活：`git config core.hooksPath .githooks`。`pre-commit` 拦截任何 staged 含 `refr/` 的提交；`commit-msg` 拒绝含 `Co-Authored-By` 或 `Generated with` 的信息。无论哪个工具（或人）运行 git 都会触发。禁止用 `--no-verify` 绕过。
2. **校验器（机械）。** `openspec/tools/validate.py --all --strict` 按状态强制工件完备性、必需标题、场景结构、任务验证格式；`openspec/tools/docs_sync.py` 强制文档双语配对与结构对齐。
3. **技能（语义关卡）。** `welang-spec-impact-audit`、`welang-change-review`、`welang-code-review`、`welang-pre-push-checks`、`welang-archive-sync`——在对应生命周期变迁时调用。strict 校验通过不替代语义评审。
4. **AGENTS.md（入口纪律）。** 每个会话必读的硬规则与不变量，包括最高优先级的 `refr/` 规则。

## 7. 基线提升（归档）

变更到达 `complete` 后，`welang-archive-sync` 技能执行提升：

- 规范增量 → 提升进权威位置：语言行为能力进 `docs/spec/` 下的正典规范（首次需要时创建目录/版本）；其他项目能力进变更 `proposal.md` 声明的权威位置。
- 长期取舍 → `docs/decisions/` 下的 ADR。
- 执行状态 → roadmap（或 roadmap 出现前的 `openspec/changes/` 列表）。
- 变更目录 → `openspec/changes/archive/<name>/`。
- 提升后重跑校验器，必须通过。

## 8. 已知范围（当前限制）

- 校验器查结构不查语义；语义评审由技能承载。
- 诊断码全局唯一性作为规范评审的一部分检查，直到出现规范注册表脚本（未来的 `spec` 层变更可补充）。
- `docs/spec/`、`docs/decisions/`、`docs/roadmap/` 尚不存在；由首个需要它们的归档创建。
