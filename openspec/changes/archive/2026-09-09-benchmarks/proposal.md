# proposal — benchmarks（M15）

## Why

roadmap M15 行：The evaluation suite (First-Pass Compile Rate and its siblings)。AGENTS.md 编译器定位节把这一步钉在论点最高处：generate–check–fix 回路里编译器是唯一不学习组件，四能力（可观察/可重复/可操控/可归因）的实证判决落在 M15——「整个论点的实验判决」。loop-contract（第 0 章规范级收录编译器定位）被用户裁决推迟到 M15 之后，正是为了让证据先说话；本变更就是证据的载体：评测方法论文档 + 机器可跑的任务集与批次跑器。

规范面早已铺好且只欠系统性消费：ch0 把 generate–check–fix 循环定为一等作者面；ch21 定形 JSON Lines 诊断协议（字段稳定、`--json` 不改退出码）与 we test 双结局 + 退出码 0/1/2；diagnostics.toml 153 码全量 help（每码一线索）。评测不需要任何新语言面——它是对既有面的第一次系统性消费。本变更不改变语言行为——纯 benchmark + docs 工件，规范零增量，`docs/spec/` 零触碰（internals-only 豁免路径，M0 起先例）。

## 目标与非目标

目标（四件，一条垂直线：方法论 → 任务集 → 跑器 → 自证）：

1. **评测方法论文档（docs/benchmarks.md + .zh.md，顶层双语文档）**：三指标严格定义（分子分母逐桶钉死）、任务分类体系（错误根因轴 × 复杂度层级）、反向对照任务 (~20%)、陷阱清单格式、批次 schema、反馈协议（fix 轮输入 = 原需求 + 前轮 check 的 JSON 事件流原文——generate–check–fix 的字面形状）、统计纪律（样本量披露、截尾、诚实边界桶单独披露不计入指标分母）。
2. **任务集种子 18（internal/benchmarks/tasks/，go:embed 机器工件）**：根因轴全 7 类 × L1（单函数级 10–30 行）各 2 + 反向对照 4；每任务四件：需求描述（英文提示面——LLM 提示是机器面）+ 陷阱清单（评分面，不给模型看）+ 参考解（完整项目：we.toml + src/）+ 参考测试面（对参考解 we test 全绿自证）。
3. **批次跑器（internal/benchmarks 包，Go in-process）**：批次 JSON 输入（model/task/attempts 每轮源文本）→ 组装临时项目（参考项目骨架 + attempt 的 src/ 覆盖）→ cli.Run 注入流跑 `check --json` 与 `test --json`（conformance runner 同构、零新入口、确定性重放）→ 逐 attempt 桶分类 + FPCR/FLC/LBR 计算 + Report（JSON 机器面）。
4. **合成批次自证**：五形合成批次（首过 / 多轮收敛 / 不收敛 / 潜伏缺陷 / 诚实边界）确定性钉死跑器全链与三指标计算——真实 LLM 批次是独立后续动作（产物回流仓库），不在本变更内。

非目标：

- 真实模型批次的跑动（调 LLM 生成 attempt——批次生产面是后续动作；本跑器的输入是批次文件，合成先行自证）。
- 对照组语言（Rust/Go/TypeScript 正面比较）与消融实验（关掉 effect 检查等单特性拆测）——评测阶段三，前置是本变更的方法论落地。
- L2/L3 任务（模块级 50–150 行 / 系统级 150+ 行）——扩充走独立变更登记 roadmap follow-up。
- `we bench` 子命令（ch21 R1 封闭面——评测跑器是仓库工具/Go 测试面，不是 CLI 面；子命令表不增一行）。
- 任务集统计结论（种子 18 是方法论自证集，不是数据结论集；任何「We 更好/更差」的宣称都不从本变更得出——数据要等真实批次）。

## What Changes

- 新 `docs/benchmarks.md` + `docs/benchmarks.zh.md`（英文权威 + 中文对照，评测方法论文档）+ `docs/README.md`/`README.zh.md` 责任图补行。
- 新 `internal/benchmarks` 包：
  - `tasks/`（go:embed 任务集：18 任务目录，每目录 prompt + traps + reference 完整项目）；
  - 批次 schema（LoadBatch：解析 + 校验，畸形批次的报错面）；
  - 跑器（判定机：组装 → check --json → test --json → 桶分类；指标计算 FPCR/FLC/LBR；Report 渲染）。
- Go 测试面：任务集自证（18 参考解 check + test 全绿）、陷阱校准（每任务注入典型错误 → check/test 至少一路抓住）、合成批次五形断言、schema 畸形负例。
- 既有面零触碰：cli/conformance/parser/typecheck/codegen 全不动；conformance 695 黄金零增量零改写。

## 影响层

- spec：零（评测消费既有规范面——ch0 一等作者面、ch21 JSON 协议与 we test、diagnostics.toml help；零规范增量零新码零新 ADR）。
- compiler：**变更内 D13 修复**——任务集探针揭出 parser 真缺陷：控制流头（if/while/match 条件位）的体花括号悬挂带 `depth == 0` 附加条件，depth>0 区域内（impl 方法体、接口默认方法体、调用实参、括号组、scope resource 体）合法控制形被误读为构造头 E0105；另 parseScopeRes 绑定列表的 depth 以 defer 泄入体内块。修复 = 悬挂无条件化（头的事实非区域的事实）+ 绑定值走值位 parseExpr + depth 于 `)` 处闭合。conformance 695→701（+6 钉损伤形回归）。
- benchmark：新 internal/benchmarks 包（任务集 + schema + 跑器 + 测试面）——benchmark 层首个消费者。
- docs：docs/benchmarks.md + .zh.md 新对（32→33）+ README 责任图双语言补行。
- process：roadmap 双语 M15 行（归档期）。

## 影响范围

- 新增：`internal/benchmarks/`（包源 + tasks/ 工件目录 + 单测）、`docs/benchmarks.md` + `docs/benchmarks.zh.md`、docs/README 双语补行、conformance 六枚新黄金。
- 涉触码：`internal/parser/parser.go`（悬挂条件两处 + parseScopeRes depth 纪律）+ `internal/parser/parser_test.go`（TestControlFlowInDepthRegions 先红后绿）。
- 既有测试/黄金零改写（负向对账：695 既有黄金零触碰零回归；修复面此前无黄金钉损伤形）。

## 审计记录

2026-09-09，welang-spec-impact-audit 7 点，**通过**：

1. 问题真实性 ✅——评测面是可验证黑盒缺口：仓库无任务集/无批次跑器/无方法论文档（docs/ 无 benchmarks 行、internal/ 无 benchmarks 包）；引用锚点全数核实——roadmap M15 行（0000-reference-implementation.md:35「The evaluation suite (First-Pass Compile Rate and its siblings)」）、AGENTS.md 定位节（:17「M15（FPCR/FLC）= 整个论点的实验判决」）、ch0 一等作者面（0000-principles.md:19 generate–check–fix loop）、ch21 R7 JSON Lines 协议（2100-toolchain.md:148 字段稳定承诺 + --json 不改退出码——反馈协议的机器面依据）。
2. 影响层声明 ✅——change.yaml 初始 [benchmark, docs] 与 proposal 影响层一致；「不改变语言行为——纯 benchmark + docs 工件，规范零增量，docs/spec/ 零触碰」边界明示（internals-only 豁免，M0 起先例；本变更是 benchmark 层设立以来首个消费者）。**实现期修订（披露）**：任务集探针揭出 parser 悬挂缺陷（见影响层 compiler 条），layers 扩为 [benchmark, compiler, docs]——修复不改变规范行为面（只让既有批准形正确解析），D13 纪律在案。
3. 规范增量范围 ✅——零 Requirement 增删、零诊断码、零 ADR；评测只消费既有面（ch0/ch21/diagnostics.toml 153 码 help）。
4. 原则一致性 ✅——无原则突破；评测是对 P7（结构化诊断面）与一等作者面承诺的系统性消费；跑器只判定不生产（一类事实一个权威位置）。
5. 参考基线固定 ✅——工件零处引用 refr/（方法论轴系是本变更四裁决自建权威；v0.8-review 是方向性输入未入引用面）；引用面全为仓库本体（roadmap/AGENTS.md/docs/spec/）。
6. 验收边界 ✅——四目标机械可判（docs 双语对 + docs_sync 33 / 任务集 18 自证 check+test 全绿 / 跑器五形逐值断言 / 黑盒电池）；非目标六项排除防蔓延（真实批次、对照组、消融、L2/L3、we bench、统计结论）。
7. 粒度 ✅——一条垂直线（方法论 → 任务集 → 跑器 → 自证）；无跨不相关层；涉触码为零的最纯形。

**实现期修订二（披露，T3 任务集探针）**：任务集落盘前的运行塔能力测绘（黄金对照 + 二进制探针 + codegen 源码三重验证）揭出两类事实，均按 D13 披露、不改规范面：

- **运行面白名单收窄任务锚定**（design D3「任务不用未实现形」条款的执行面）：① sum 型（Option/Result/自有 sum）仅能经原语调用物化（channel.receive / scope timeout / task await 的 Result）——用户函数不能 return Some/None（bnd）；② sum 型参数与同步类型参数（Mutex/Channel）不入可跑槽（bnd）——任务契约全部走局部绑定；③ 数值返回仅 Int64（Int32 返回 bnd）；④ String 相等仅在 test 断言面可用（src 内 `if s == "x"` 运行面 bnd），字符串拼接 `+` 与 `%`/`/` 算子为诚实边界；⑤ main 纤程直停 receive 于 timeout scope 内在虚拟钟下死锁——等待须在 task 体内（黄金 run-conc-scope-timeout-err 同构）。据此：**dangling 轴锚定从 design 原记的 E1104/E1105 相邻面移至 task 句柄生命周期（E1607 未 await 逃逸 / E1618 scope 外 spawn）**——byres 资源的任何函数体内使用（含构造返回）运行面 bnd，诱饵形按 D3 披露条款留 traps.md 注记（errpath-02/dangling-01 在案）；error-path 轴锚定 scope timeout + Ok/Err match；implicit-conversion 轴以 E0501 check 面为主测量面（无任何转换算子——混合宽度不可表达正是轴的论点）。
- **新发现 codegen 缺陷（不在本变更内修）**：同函数内「带臂赋值的 match 语句先于 scope（含 timeout）块」发出非支配 IR（`Instruction does not dominate all uses`，clang 拒绝）——errpath-02 参考解以延迟 match 形绕开并在 traps.md 披露；修复属 codegen 线（follow-up 于归档时登记 roadmap），不阻断 M15（任务集已绕开该形）。

## 审查记录

2026-09-09，welang-change-review 10 点，**通过**（发现 1 处已处置）：

职责边界四条 ✅——proposal 黑盒问题+目标（What Changes 载结构属 M13/M14 先例形）；spec 增量零（internals-only 豁免，技术面由 design 承载）；design 唯一路径 + D1/D3/D4 各记被拒替代与理由、引用精确到 ch21 R1/R7、E0403/E1104/E1105/E1602/E1603、M10b/M11 先例；tasks 全项来源/验证、无 deferred/未决。

内容质量六条 ✅——场景覆盖三路（normal 首过形 / boundary 诚实边界形 / degradation 不收敛形 + schema 畸形负例四枚）；无空章节；测试先行（T2 schema/T3 判定机首动作即红——行为面首 task 是失败测试，T1 文档先行非行为变更）；负向断言真实（路径越界负例构造于 T2、「任务不用未实现形」由 T4 自证钉死）；完成度闭环第 9 条适用性判断——工具/评测面非语言特性（判定复用 cli.Run 既有 check/test 管线，零新编译器语义面），M14 判例同形；无未决问题（四裁决已落定）。

F1（已处置）：D3 括号压缩把两项独立裁决（引用类型终身不引入 ch18 裁决 / 顶层 var 禁止 E0403）并置成一句，易误读为因果——修正为分号分立、各标出处。

## 实现审查记录（active → complete 关卡，2026-09-09）

welang-code-review 7 条手动执行，**通过**（发现 1 处已处置）：

1. **规范符合性** ✅——本变更规范零增量（internals-only 豁免）；实现只消费既有面：跑器桶判定锚定 ch21 退出码语义（check 0/1/70|2、test 0/1/2|70），方法论文档 docs/benchmarks.md 为判定序权威（runner.go 文件头声明消费关系）；任务集校准所涉全部诊断码（E0501/E1105/E1202/E1602/E1603/E1607/E1618/E0204/E0305/E0605）皆既有注册码，零规范外行为。
2. **验证诚实性** ✅——T0–T7 逐项核对：全部验证命令本窗口真实运行且输出在案（T7 六面全绿：gofmt 零输出 / vet 零输出 / 13 包 ok（benchmarks 27.9s 含 schema+任务集自证+五形+64 校准+黑盒五桶）/ validate strict OK / docs_sync 33 对 / conformance TestGoldenCases 701 PASS）；T4/T5 先红证据（`undefined: Evaluate/Bucket*`）在完成记录；无「勾了没跑」项。
3. **测试先行证据** ✅——T2 schema（先红 `undefined: LoadBatch`）、T3 registry（先红 `undefined: Tasks/Has`）、T4 跑器（先红 `undefined: Evaluate`）、T0 parser（TestControlFlowInDepthRegions 先红 7 形 E0105）均先于实现；六枚新黄金走既有 cases/*.json 格式（TestGoldenCases 全量解析通过即证）；T5 校准属审计型测试面——其「红」moment 是 D13 披露的两枚落 clean 负例（nullbnd-02 conflate 死面 / control-02 swap 交换律正确），均已修面并在 tasks.md 披露。
4. **诊断协议稳定** ✅——零 `--json` 字段改动；零新增诊断码（diagnostics.toml 不在 diff 面）；benchmarks 包只读退出码不产诊断。
5. **单一权威** ✅——长期事实全部落权威位：方法论文档在 docs/benchmarks.md（变更目录不滞留）；任务集是机器工件（go:embed，非规范面）；代码注释/docs 英文（全量 grep 验证：Go 文件与 benchmarks.md 零中文字符，中文仅在 .zh.md 与变更工件）；归档期待提升项已标记：roadmap M15 → done、codegen 支配性缺陷 follow-up、L2/L3 扩充 follow-up（T9 处理）。
6. **红线复核** ✅——`refr/` 在 .gitignore（check-ignore 验证）；尚无 commit（提交候用户明示）；diff 面对账：parser 24 行核心 + 68 行测试（T0 披露范围内）、docs/README 双语各 1 行、六枚黄金、benchmarks 包与 docs 对、变更目录——无非目标越界（真实批次/对照组/消融/L2/L3/we bench 子命令均未触碰）。
7. **最小可信验证已跑** ✅——实际 diff = parser 源改 + 新 Go 包 + 文档对 + 黄金：按选取标准跑全量 `go test ./...` + validate --strict + docs_sync + conformance 计数复核（701）。

F2（已处置）：tasks.md T0 三枚 checkbox 共用一组 来源/验证，validate strict 打回（checkbox 逐枚要求）——拆为三枚独立条目各带来源/验证后复验 OK。
