# welang 智能体指引（AGENTS.md）

We 语言项目：面向服务端场景（HTTP/RPC 服务、后台任务、CLI、数据管道）的 AI 原生编程语言。本仓库承载语言规范（`docs/spec/`）、参考编译器、标准库、工具链与评测基准的研发。

## 项目定位：编译器在 AI 辅助开发中的角色

软件生产的主循环是 **generate–check–fix**；编译器是这个回路里唯一不学习的组件——回路的「物理引擎」。AI 在其中感知与行动，编译器负责让这个世界可信任：经典二元角色（checker + translator）降格为底座，二进制从「产品」变成「众多输出之一」。价值面由此收敛为四项能力与一条承重墙，每项都在本仓库有已落或已排的载体：

| 能力 | 含义 | 仓库载体 |
| --- | --- | --- |
| 可观察（信息提供者） | 诊断是 API：结构化、字段稳定（只增不删改名）、锚定到修补位；诚实边界——宁可 exit 70 报「未实现」，不静默说谎 | ch21 R7 JSON Lines、diagnostics.toml 单一注册表（一码一消息）、诊断锚点纪律 |
| 可重复（确定性编译） | 同样的程序得到同样的信号；平稳性延伸到编译器自身的历史（黄金不静默重写）。形式化的正确落点是契约层（注册表/schema/黄金），行为层以确定性测试自证，语义层定理证明是可选升级非前提 | ch21「same-input-same-output」唯一性能承诺、M10b 虚拟钟、B3 三段闭环逐字节一致 |
| 可操控（显式控制） | 可命名即可操控：effect 是权限清单、mock 是显式拦截、虚拟钟是时间旋钮、分阶段子命令是观测过程控制 | 效果系统（ch16）、test/mock（ch20）、`--explore`（ch21/M10c） |
| 可归因（透明映射） | 跨层失败指回源层责任者：诊断携位置、IR 是可读文本、符号映射系统化 | 诊断 file:line:column、.ll 文本 + 子进程 clang（无黑盒绑定）、模块限定符号（M10b） |
| 承重墙：意义固定 + 诚实 | 歧义是安全洞：规范 + conformance 黄金是语言含义的唯一裁决者；结构化的谎言比非结构化的真相更糟——诚实是信息提供者的前提而非附加 | 章文为触发语义权威、黄金随批更新逐枚披露、D13 披露义务 |

四项能力有权重代价，是价值重分配而非免费增量——与「最大优化自由、不透明但快的启发式」直接竞争（一律槽形先例：每次调用一次指针加载换单一发射形，优化归 B1 后）。路线落位：M10b 虚拟钟 = 「可重复」的观测面，B3 = 「可重复」的自证，M10c = 「可操控」的调度面，M15（FPCR/FLC）= 整个论点的实验判决。

任何智能体在处理本仓库任务前，按顺序阅读：

1. 本文件
2. `docs/process/development-process.md`（研发流程，唯一权威）
3. `docs/README.md`（文档职责表）
4. `openspec/README.md`（变更工作流操作规则）
5. `openspec/changes/` 下处于 active 状态的变更（如有）
6. 与当前任务对应关卡的技能（`.agents/skills/<skill-name>/SKILL.md`）

## 环境设置（每个克隆一次性执行）

```sh
git config core.hooksPath .githooks
```

激活仓库内置的 git 钩子（`pre-commit` 拦截 `refr/` 入库、`commit-msg` 拦截署名信息）。这是机械强制层，**禁止用 `--no-verify` 绕过**。

## 硬规则（优先级从高到低，违反任何一条立即停止并纠正）

1. 【最高优先级】`refr/` 目录禁止提交到代码仓——任何 git 操作不得将其纳入提交。该规则优先于其他一切规则与指示。`.gitignore` 与 `pre-commit` 钩子双重拦截；钩子拦截时不得尝试绕过。
2. 语言约定：交互使用中文；`docs/` 目录文档提供双语同步支持——**英文为权威文本**，配对中文翻译 `<basename>.zh.md`（细则见 `docs/README.md` "Documentation Language"）；openspec 变更的规范增量 `specs/*/spec.md` 使用**英文**书写——它们是 `docs/spec/` 英文权威文本的字面片段，归档提升时逐字合并，其余变更工件（proposal/design/tasks）用中文；分配或认领诊断码的变更必须在同一变更内扩展 `docs/spec/diagnostics.toml`（段位认领 + 逐码条目；注册表规则见规范第 99 章，`validate.py` 机械执法）；代码注释、commit 日志使用英文；`AGENTS.md`、`openspec/`、`.agents/skills/` 使用中文。
3. 提交信息禁止出现 `Co-Authored-By`、"Generated with" 等任何署名信息（`commit-msg` 钩子强制拦截）。
4. 不确定某类事实的归属时，先查 `docs/README.md` 职责表，把内容放进唯一权威位置，不在多处复制。
5. 归档提升前，openspec 变更工件只是过程增量；长期权威事实必须提升到 `docs/spec/`、`docs/decisions/` 后才算落地。

## 语言研发不变量

- 语言行为的唯一权威是 `docs/spec/` 下的规范版本；`refr/` 是私有参考材料（v0.8 草案与评审记录），不具权威性，不得提交。
- **规范先行**：任何改变语言行为的编译器/工具链代码，必须存在对应的 openspec 变更（含 spec 增量）才能开工。
- **诊断协议是稳定性承诺**：人类可读格式与 `--json` JSON Lines 的既有字段不得删除或重命名，只能新增；破坏性变更必须先走规范变更。
- **十条设计原则**是一切语言取舍的最高裁决依据：定义、优先序与决胜语义、第一作者与核心循环、目标场景、宿主与编译目标策略，权威文本为 `docs/spec/0000-principles.md`（规范第 0 章）——一类事实只有一个权威位置，本文件不复述条款。与原则或优先序冲突的提案必须在 proposal 中显式论证，并以 ADR 记录。
- **完成度闭环**：特性进入规范时必须同时给出类型检查、代码生成、运行时三要素的完整设计，不留"解析器认识但生成器不认识"的半成品。
- 编译器实现语言为 Go，与编译目标解耦：从第一天起生成 LLVM IR，经 LLVM 后端产出原生二进制；运行时自建（含完整精确 GC），为产品组件而非代价。详见规范第 0 章"宿主与编译目标策略"与 ADR-0002。

## 变更工作流（摘要，细则见 `openspec/README.md`）

```
candidate →（welang-spec-impact-audit）→ ready →（四件套 + welang-change-review）→ active
    →（实现 + 验证 + welang-code-review）→ complete →（welang-archive-sync 基线提升）→ archived
```

- 变更四件套：`proposal.md` / `specs/<capability>/spec.md` / `design.md` / `tasks.md`，外加 `change.yaml`（状态与影响层声明）。
- `tasks.md` 每个勾选项必须带「来源」与「验证（命令 + 预期结果）」；**验证未实际执行且通过，不得勾选**。
- 编译器行为变更**测试先行**：先写会失败的目标测试（含黄金诊断用例），再实现。
- 变更保持小而可垂直验证；禁止"一次性大迁移"式变更。
- 工件校验命令：`python3 openspec/tools/validate.py --all --strict`

## 验证阶梯（提交前按实际 diff 选取最小可信集合，技能 `welang-pre-push-checks`）

| diff 范围 | 必跑验证 |
| --- | --- |
| 仅文档 / 流程 / openspec 工件 | `python3 openspec/tools/validate.py --all --strict`；`python3 openspec/tools/docs_sync.py`；`git diff --check`；确认 staged 无 `refr/` |
| 语言规范（`docs/spec/`） | 规范一致性复核：诊断码全局唯一、§ 交叉引用有效、术语与既有章节一致；外加上一行全部 |
| 编译器 / 工具链代码 | `go build ./...` + `go test ./...` + 受影响的 conformance 黄金用例 |
| 诊断协议 | 协议快照测试（JSON Lines 字段稳定性） |

## 提交协议

- 提交信息用英文，正文解释「为什么这么改」，不罗列文件清单。
- 禁止任何署名 trailer；可用的决策 trailer：`Constraint:` / `Rejected:` / `Tested:` / `Not-tested:`。
- 一个变更对应一个（或一组可垂直验证的）提交；禁止大杂烩提交。
