# We 评测套件

这是 We 语言的评测方法论文档——语言编译回路主张如何被测量的唯一权威。它定义三指标、任务分类体系、陷阱清单格式、批次 schema、反馈协议、跑器判定序与统计纪律。两个机器面消费它：内嵌任务集（`internal/benchmarks/tasks/`）与批次跑器（`internal/benchmarks`，Go）。机器面与本文不一致时，以本文为准，机器面承缺陷。

前提是第 0 章的一等作者面。generate–check–fix 循环里模型写、检查器拒或过、模型再写——编译器是回路中唯一不学习的组件。它的四项承诺（可观察、可重复、可操控、可归因）在被一套套件对着模型写出的代码测量之前，都是实验主张；本套件就是那个测量。每个判定都归结为 `we check` 与 `we test` 的退出码与事件流——正是第 21 章为回路本身稳定的机器面，整套件因此可复现。

三条范围门框住下文一切。套件只判定不生产：模型调用与反馈构造是批次生产面的工作，跑器的唯一输入是批次文件——本 schema 下的仓库工件。不新增 CLI 面：`we bench` 不存在也不可能——第 21 章命令集是封闭面；跑器是仓库工具，经 Go 测试 in-process 驱动。种子任务集证明的是方法论而非语言——任何关于 We 的比较性结论都不出自种子集。

## 三指标

一切指标从 attempt 桶分类导出。每个 attempt 面对两段判定——先 `we check --json`，check 过后才 `we test --json`——落入且仅落入五桶之一：

| 桶 | 判定 | FPCR | LBR |
| --- | --- | --- | --- |
| `clean` | check exit 0，test exit 0 | 分子分母皆计 | 分母 |
| `latent` | check exit 0，test exit 1 | 分子分母皆计 | 分子分母皆计 |
| `test-malformed` | check exit 0，test exit 2 或 70 | 分子分母皆计 | 只计分母，不计分子 |
| `rejected` | check exit 1（E 级诊断事件） | 分母 | 不入 |
| `boundary` | check exit 70 或 2（诚实未实现形，或工具用法畸形） | 两侧皆排除，单独披露 | 不入 |

退出码是第 21 章的稳定契约：0 通过、1 发现 E 级诊断、2 用法畸形、70 本参考构建未实现的已批准形。advisory `W` 事件永不动判定——warning 缺省下工具报告它但照样 exit 0，「过」就是 exit 0 的机器事实，没有第二种读法。

**FPCR（First-Pass Compile Rate，首过编译率）**在全部任务的 round-1 attempt 上测量：(clean + latent + test-malformed) / (clean + latent + test-malformed + rejected)。首过计的是「编译通过」——测试随后说什么不在此问；测试结局是 LBR 的问题。boundary attempt 两侧皆排除：诚实边界既非模型过错也非对语言的判决，计入任一侧都会污染数字。boundary 占比随指标一同披露。

**FLC（Fix-Loop Convergence，修复回路收敛）**测的是首过被拒后回路的形状。反馈协议（下文）钉死轮 k 的输入：原需求 prompt 原文 + 轮 k−1 的 `we check --json` 事件流原文。序列在其首个 check exit 0 的轮收敛；收敛轮数即该轮轮号。序列在 N=5 截尾：五个 attempt 内无 check-pass 轮的序列计 not-converged——截尾显式，不折算轮数。报告携带收敛轮数分布（median、p90、截尾值处的 max）与 not-converged 计数。

**LBR（Latent Bug Rate，潜伏缺陷率）**在全部轮次（非仅末轮）的全部 check-pass attempt 上测量：latent / (clean + latent + test-malformed)。attempt 级是机械无歧义面。任务级衍生视图并列报告——末轮 check-pass 的任务中 test 挂的比例，直观读法——两者命名分立，永不混同。

解读纪律：FPCR 单独不是语言质量数字——rejected attempt 是正面证据，语言抓住了模型犯的错。判决面是联合读法：FPCR 配 FLC（是否首过就编译通过；不过时回路收敛多快）再配 LBR（什么从检查器漏到测试）。一门全拒的语言 FPCR 为 0；一门全收的语言 LBR 为 1；设计的主张活在中段，只有联合读法测得到它。

## 任务分类体系

任务按错误根因分类而非业务领域——分类轴测的是语言设计本身消除了哪些错误类。七轴，每轴两个 L1 任务：**error-path**（可失败调用的失败分支未处理）、**race**（并发任务在同步纪律之外改共享状态）、**resource-leak**（句柄逃逸或在 scope-resource 纪律之外释放）、**null-boundary**（`Option` 分支集缺一臂）、**implicit-conversion**（显式转换设计拒绝的混宽算术）、**dangling-reference**（转移后使用与别名纪律——见下文可表达性披露）、**context-passing**（隐式上下文捕获与传递纪律——同前）。

复杂度只到 L1——单函数级 10–30 行。模块级（L2）与系统级（L3）任务是后续扩充，已登记 roadmap follow-up；种子集先证明方法论。

四枚**反向对照**任务（`control-01`..`control-04`）是纯算法与字符串工作，不触任何轴。它们把轴面位移可能携带的两种解释分开——「语言设计消除了该错误类」与「模型单纯觉得这套语法顺手」——对照任务只随第二种因素移动。

每任务目录 `internal/benchmarks/tasks/<轴>-<nn>/`（对照为 `control-<nn>`）四件：

1. `prompt.md`——需求描述，英文。prompt 是真实批次的机器面；LLM 提示是评测基线，跨模型比较需要单一提示语言。
2. `traps.md`——陷阱清单（下节）；评分面，永不给模型看。
3. `reference/`——参考解完整项目（`we.toml` + `src/`），构造上 check 全绿 test 全绿——自证测试机器强制两绿。
4. `reference/tests/`——参考测试面：测试随任务而来，非 attempt 产出；对参考解 `we test` 全绿。

**attempt 组装。**跑器复制参考项目到临时目录，把 attempt 的 `files`（路径 → 源文本 map）覆盖到 `src/` 上。测试面固定——评分客观——模型的自由度是整个 `src/`：重构布局、新增模块，皆合法。若重构破坏参考测试的导入，test 段 exit 2，attempt 落 `test-malformed`——诚实分类，非跑器错误。

**可表达性披露。**两轴值得单列一段。悬空引用与隐式上下文的经典诱饵形被设计本身关死，两项独立裁决：引用类型终身不引入 We（第 18 章裁决——`Ref<T>` 不存在）；模块级可变状态不存在（`E0403`：无顶层 `var`）。不可表达的形做不成任务。context-passing 轴因此锚在相邻可表达形上——捕获与 effect 纪律（`E1602`/`E1603`）；dangling-reference 轴锚 task 句柄生命周期——句柄到 scope 退出未 await（`E1607`）与任何 scope 之外 spawn task（`E1618`）——因为本参考构建的 byres 运行面未实现（byres 资源在任何函数体内的使用都被运行面拒斥），`E1104`/`E1105` 留作 check 锚定的诱饵形、由 traps.md 在 prompt 邀到处注记。每枚此类任务的 `traps.md` 披露映射。这本身就是测量的一部分：模型写出被关死的形时 `we check` 直接拒——FPCR 受抑、回路收敛，正是设计生效的测量面。

**运行面锚定。**参考构建的运行塔实现已批准语言的刻意子集，L1 任务锚在其内。该子集随 codegen-mono 线拓宽：sum 型可构造、可返回、可 match——用户函数可收可返 `Result`/`Option`/自有 sum，sum 因此不再被限制在产出它的那几枚原语调用里；sum 型与同步型参数入可跑槽；数值返回可为任意 checked 整型宽度而非仅 `Int64`（`fn sum(a: Int16, b: Int16) -> Int16` 可跑，且其算术按 `Int16` 检查，溢出陷阱点名该宽度）；`String` 相等与拼接可跑，记录方法、元组、新类型、`List` 迭代、闭包与 fn 值、顶层绑定亦然；`%` 与 `/` 发射检查算术；函数体可从任意深度 return，不限尾位；由值位 `if`/`match`/块绑定而来的 `let`（各分支同为标量时）亦可读入 `String` 位。仍关死的面：泛型函数（`bndGenericFns` 边界）、普通 `scope { … }` 的值形、main 体内的 `?` 解包、元组值绑定读入 `String` 位、fn 值在值串位或操作数位的调用（`"${f(1)}"`、`f(1) + 1`；其语句位与尾位可跑）、内置所产 `Option` 载荷（`reduce`/`find`/`next`）读入 `String` 位——皆诚实边界（exit 70），皆在此披露。一条运行期规则未变：等待属于 task 体——main 纤程停在自己的 timeout scope 内 `receive` 在虚拟钟下死锁。这些约束塑造任务编写，不削减模型自由度：越出子集的 attempt 被分类（boundary 或 test-malformed），绝不静默通过；子集本身随 codegen-full 线落地收缩。

## 陷阱清单

每任务的 `traps.md` 承载需求所邀请的陷阱。每陷阱一条：一行点名错误形状，一行给出检测路——注册表码（`we check` 抓住）或参考测试断言（`we test` 抓住）。清单是评分面；永不进入 prompt。

陷阱清单是受检工件，不是散文。校准测试把每陷阱的典型错误形注入参考解，断言 check 或 test——至少一路——抓住它。没有东西抓住的陷阱是任务设计缺陷：修任务或修清单并披露错过，绝不静默留下。

## 批次 schema 与反馈协议

批次文件是一个任务的 attempt 序列——仓库工件：

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

装载期校验直接拒批于：`model` 为空；`task` 未知；round 不从 1 连续（无跳号无重复）；`files` 为空；任何路径越出 `src/`——触 `tests/` 或 `we.toml` 是篡改评分面，拒绝而非分类。`src/` 下新增文件依上节组装模型合法。

**反馈协议**定义轮 k（k>1）attempt 的生成输入是什么：原 `prompt.md` 原文 + 轮 k−1 的 `we check --json` 事件流原文——逐行，code、message、file、line、column、help。这个字面形状就是 generate–check–fix 回路：模型读机器自己的话。跑器永不构造反馈——判定与生产分立（一类事实一个权威位置：判定归跑器，批次生产归生产流程）。合成批次按此协议形状手写每轮文本；真实批次的文本由其流程自行产出，跑器分不出两者——by design。

## 跑器判定序

每 attempt 四段有序：

1. **组装。**复制参考项目到全新临时目录；attempt 的 `files` 覆盖 `src/`。attempt 结束后目录删除。
2. **check。**经 in-process CLI 入口跑 `we check <dir> --json`——conformance runner 同款注入流面：无子进程、无新入口。exit 0 进入下段；exit 1 落 `rejected`；exit 70 或 2 落 `boundary`。
3. **test**（仅 check-pass）。同路跑 `we test <dir> --json`。exit 0 → `clean`；exit 1 → `latent`；exit 2 或 70 → `test-malformed`。
4. **分类记录。**attempt 的桶、退出码与诊断码入报告。

确定性是套件的属性而非愿景：零时钟、零网络、零路径泄漏。工作目录纪律同 conformance runner（进、跑、恢复），任何机器可变量不入报告——同一批次文件每次运行产出逐字节相同的报告。

一项已披露限制：评测是 in-process 的，编译产物永不终止的 attempt——无等待源的忙循环，死锁检测器唯一看不见的形——会挂起评测。死锁形错误 attempt 在虚拟钟下确定性中止、正常分类；真实批次生产面在喂入不受信 attempt 前必须加墙钟守卫。

**报告**是 JSON 机器面：逐 attempt 桶、逐任务视图、三指标（round-1 FPCR；FLC 分布与 not-converged 计数；attempt 级 LBR 附任务级衍生视图）、boundary 与畸形计数（永不缺席——见下节）、每个比例背后的样本量。

## 统计与报告纪律

- 一切比例附分母。报告中不存在裸百分比。
- 截尾显式。FLC 于 N=5 截尾；截尾序列计 not-converged，不为其折算轮数。
- boundary 与 test-malformed 桶永不静默。每份报告必带两桶计数——exit 70 诚实边界披露纪律延伸到评测。
- 重放逐字节相同（上文确定性）；临时路径永不入报告——只有任务 id、桶与指标。
- 真实批次文件是本 schema 下的仓库工件；报告是可再生产物，非明示不入仓。
- 种子集不承载统计结论。十八枚任务证明方法论可跑通；不支撑任何关于语言的比较性主张——那是真实批次的职责。
