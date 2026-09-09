# design — benchmarks（M15）

四裁决已落定（AskUserQuestion 2026-09-09，均采推荐）：Q1 三指标全量 / Q2 种子集 ~18 / Q3 Go in-process 跑器 / Q4 合成批次自证。

## D1 文档形态：docs/ 顶层双语对

`docs/benchmarks.md`（英文权威）+ `docs/benchmarks.zh.md`（中文对照）——lsp.md 先例（M14）：非规范章的顶层能力文档，promote 规则走 dev-process §7「非语言行为能力 → proposal 声明的权威位置」，本 proposal 声明即此。docs/README 责任图双语言补行；docs_sync 32→33 对。

方法论文档六节：①三指标严格定义（D2 全文）②任务分类体系（D3）③陷阱清单格式（D3）④批次 schema 与反馈协议（D4）⑤跑器判定序（D4）⑥统计纪律（D8）。文档是方法论唯一权威——任务集与跑器是它的机器消费面。

被拒替代：写成 docs/spec/ 章（拒绝——评测是仓库工具面非语言行为，规范只承载语言与工具面承诺）；单英文文档（拒绝——仓库双语纪律，docs_sync 结构同步是 docs 变更的完成定义）。

## D2 三指标严格定义（逐桶钉死）

一切指标从 attempt 桶分类导出。每 attempt 两段判定（check → test），桶分类封闭五桶：

| 桶 | 判定 | FPCR | LBR |
|---|---|---|---|
| clean | check exit 0 且 test exit 0 | 分子分母皆计 | 分母 |
| latent | check exit 0 且 test exit 1 | 分子分母皆计 | 分子分母皆计 |
| test-malformed | check exit 0 且 test exit 2/70 | 分子分母皆计 | 分母（不计分子） |
| rejected | check exit 1（E 诊断事件） | 分母 | — |
| boundary | check exit 70/2（诚实边界 / usage 畸形） | 皆不计，单独披露 | — |

- **FPCR（First-Pass Compile Rate）** = 批次内全部任务的**首轮** attempt 中 (clean+latent+test-malformed) / (clean+latent+test-malformed+rejected)。advisory W 不影响判定（ch21/M11 已钉死：warning posture 缺省下报告但 exit 0——「过」= exit 0 的机器事实，无第二解释）。boundary 桶排除于分母：诚实边界（参考构建未实现的规范形）既非模型过错也非语言判决，计入任一侧都会污染指标——单独披露 boundary 率。
- **FLC（Fix-Loop Convergence）**：轮 k 的输入 = 原需求描述 + 轮 k−1 的 check JSON 事件流原文（反馈协议，D4）。收敛 = 某轮 check exit 0；收敛轮数 = 首个 check-pass 轮号。截尾 **N=5**：5 轮内未 check-pass 计 not-converged。报告收敛轮数分布（median / p90 / max 截尾值）+ not-converged 计数。
- **LBR（Latent Bug Rate）** = 全部 **check-pass attempt**（各轮，非仅末轮）中 latent / (clean+latent+test-malformed)。attempt 级是机械无歧义面；任务级（末轮 check-pass 的任务中 test 挂的比例）作衍生视图并列报告。被拒替代：仅末轮（拒绝——丢弃中途信息且「末轮」定义在 not-converged 序列上纠缠；attempt 级两语义并列报告更诚实）。

指标解读纪律（文档明写）：FPCR 单独不是「语言好坏」——rejected 是「语言抓住错误」的正面证据；判决面是 FPCR + FLC 联合（首过率与收敛速度的联合分布）+ LBR（漏网率）。种子集自证不产结论（非目标）。

## D3 任务集：根因轴 × L1 × 2 + 反向对照 4

- **根因轴 7 类**（错误根因非业务领域——分类轴测的是「语言设计减少了哪类错误」）：error-path（错误路径遗漏）/ race（并发数据竞争）/ resource-leak（资源生命周期泄漏）/ null-boundary（空值边界遗漏）/ implicit-conversion（类型隐式转换误用）/ dangling-reference（生命周期悬空引用）/ context-passing（上下文状态传递遗漏）。
- **复杂度**：仅 L1（单函数级 10–30 行）；L2/L3 留扩充变更（roadmap follow-up 登记）。
- **反向对照 4**：纯算法/字符串处理任务，不涉任何根因轴——区分「语言设计减少特定错误」与「LLM 对语法更顺手」的泛化因素。
- 每任务目录 `internal/benchmarks/tasks/<id>/`（id = `<轴缩写>-<两位序号>`，如 `race-01`；反向对照 `control-01`..`control-04`）四件：
  1. `prompt.md`——需求描述，**英文**（LLM 提示是机器面，英文是评测惯例基线；仓库工件语言纪律：机器面英文）；
  2. `traps.md`——陷阱清单（不给模型看，评分面）：每陷阱一条 + 对应检测路（check 码或参考测试断言）；
  3. `reference/`——参考解完整项目（we.toml + src/main.we [+ 按需模块]），要求：check 全绿零 W?（否——W 允许，诚实面）+ 参考测试全绿；
  4. `reference/tests/`——参考测试面（任务工件自带，非模型产出；对参考解 we test 全绿自证）。
- **attempt 组装模型**：跑器复制参考项目到临时目录，attempt 携带的 `files`（路径→源文本 map）覆盖 `src/` 下同路径——测试面固定（评分客观），模型自由度在 src/。
- **诚实披露（文档 + 本设计记录）**：7 轴中 dangling-reference 与 context-passing 在 We 面的**典型诱饵形被设计本身关死**——两项独立裁决：引用类型终身不引入（ch18 用户裁决，Ref<T> 不存在）与顶层 var 禁止（E0403，无模块级可变状态）——诱饵形态不可表达。该两格任务锚在**相邻可表达形态**（dangling → 转移后使用与别名纪律 E1104/E1105 面；context → 捕获纪律 E1602/E1603 面）并在 traps.md 披露映射。这本身是论点的一部分：模型写出此类形态时 check 直接拒（rejected 桶）——FPCR 受抑 + FLC 收敛正是设计生效的测量面。
- 任务语言面 = 当前构建能力面（check 全域 + test 运行面 M10b 集）：任务不得用未实现形（否则参考解自证先停边界）——任务集自证测试钉死此约束。

被拒替代：任务集外置仓库/独立仓库（拒绝——go:embed 单仓自证，任务集与跑器同版本演进）；中文提示面（拒绝——真实批次将跨模型对比，英文提示是跨评测可比基线）。

## D4 批次 schema 与跑器

**批次文件**（JSON，仓库工件、真实批次产物回流同 schema）：

```json
{
  "model": "string（自由文本标识）",
  "task": "任务 id（必须存在于任务集）",
  "attempts": [
    {"round": 1, "files": {"src/main.we": "…"}},
    {"round": 2, "files": {"src/main.we": "…"}}
  ]
}
```

校验面（LoadBatch 报错即拒）：model 非空；task 在任务集内；round 从 1 连续递增无跳号；files 非空且路径必须落在 `src/` 下（新增模块文件合法——模型可重构 src/ 布局；若重构破坏参考测试的导入，test 段 exit 2 归 test-malformed 桶，诚实分类），不得触 `tests/` 或 `we.toml`（评分面不可篡改）。

**反馈协议**（文档定义；跑器不生产反馈——一类事实一个权威位置：判定归跑器，批次生产归生产流程）：轮 k>1 attempt 的生成输入 = 原需求 prompt 原文 + 轮 k−1 的 `we check --json` 事件流原文（逐行，含 code/message/file/line/column/help）。合成批次手写每轮文本即模拟此协议的产出。

**跑器判定序**（每 attempt）：
1. 组装：参考项目复制到临时目录（`os.MkdirTemp`，用后删——M10b 单文件面先例）+ attempt files 覆盖；
2. check 段：`cli.Run(["check", dir, "--json"], out, errb)`（in-process 注入流，conformance runner 同构）→ 依 exit 码与事件流分桶（exit 0 → 进入 test 段；exit 1 → rejected；exit 70/2 → boundary）；
3. test 段（仅 check-pass）：`cli.Run(["test", dir, "--json"], …)` → exit 0 clean / exit 1 latent / exit 2 或 70 test-malformed；
4. chdir 纪律同 conformance Execute（进程工作目录进出现场恢复）；确定性：零时钟、零网络、虚拟钟测试域内建（ch20/ch21 测试模式）。

**Report**（JSON 机器面）：per-attempt 桶 + per-task 视图 + 三指标（首轮 FPCR、FLC 分布、attempt 级与任务级 LBR）+ boundary/畸形披露计数 + 样本量（任务数 × attempt 数）。

被拒替代：CLI 子命令面（拒绝——ch21 R1 封闭，跑器是仓库工具）；子进程真二进制（拒绝——in-process 注入流是 conformance 六百黄金验证过的同构面，且零构建开销；真二进制探针留给黑盒电池抽查）。

## D5 合成批次自证（五形）

合成批次全部基于任务集真实任务、手写 attempt 文本，确定性断言 Report 精确值：

| 形 | 构造 | 断言 |
|---|---|---|
| first-pass | 参考解原文作 round 1 | FPCR=1、clean 桶、LBR=0 |
| converge | round 1 注入可修 E 码形（如 E0501）→ round 2 修正 | FPCR=0、rejected→clean、收敛轮数 2 |
| not-converged | 5 轮全注入不同 E 码形 | not-converged 计 1、截尾披露 |
| latent | check 全绿但逻辑错（改参考解一处语义，测试抓住） | latent 桶、LBR=1 |
| boundary | 注入未实现规范形（如模块级解构 `let (a,b) = (1,2)` 顶层形） | boundary 桶、FPCR 分母排除 |

## D6 测试策略（全部 Go 测试面，测试先行）

- **schema 测试**：LoadBatch 正例 + 畸形负例（未知任务 / 跳号 / 空 files / 越界路径触 tests/ 与 we.toml）——先红后绿。
- **任务集自证测试**：18 任务参考解逐个 `check` + `test`（in-process）全绿——任务集本身是受检工件；同时钉死「任务不用未实现形」约束。
- **陷阱校准测试**：每任务按 traps.md 注入典型错误（每陷阱至少一枚负例源）→ check 或 test 至少一路抓住——「这段代码有没有踩坑」从主观判断变成机器判定；未被抓到的陷阱 = 任务设计缺陷（修任务或修陷阱清单，D13 披露纪律）。
- **跑器五形测试**：D5 表逐形断言。
- 黑盒电池（真二进制抽查）：合成批次经真 `we check`/`we test` 子进程复刻关键桶判定（in-process 与子进程零行为差——M6b 先例的对拍面）。

## D7 披露面

- 跑器只判定不生产（反馈构造、模型调用是批次生产面——真实批次为后续动作）。
- dangling-reference/context-passing 两轴的可表达诱饵形映射（D3 披露）。
- 任务集 18 = 方法论种子，统计功效不足，不产结论（任何方向的宣称都不从种子集得出）。
- `we bench` 不存在也不可能（ch21 R1 封闭面复述——防蔓延门）。
- 任务语言面锚定当前构建能力面（B1 codegen-full 落地后任务集可扩运行面更深的任务——扩充变更登记）。

## D8 统计与报告纪律

- 一切比例附分母（样本量）——报告无裸百分比。
- 截尾显式（FLC N=5；截尾样本计入 not-converged，不折算轮数）。
- boundary/test-malformed 桶永不静默——任何 Report 必带两桶计数（诚实边界的披露纪律同 ch21 exit 70 面）。
- 同一批次重跑逐字节同输出（确定性重放：无时钟依赖、MkdirTemp 路径不入报告——报告只含相对任务 id 与桶/指标）。
- 真实批次回流：批次文件即仓库工件（schema 同 D4），报告不入仓（可再生产物）——除非用户明示。
