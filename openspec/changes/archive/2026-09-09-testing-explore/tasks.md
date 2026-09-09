# tasks — testing-explore（M10c）

- [x] T1 测试先行 A：D11 黄金矩阵落盘（预计 ~18，test-explore-* 全 byte-exact），对当前构建运行必须先红
  来源：proposal 目标 1–8 / design D11
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红（新增失败清单，既有 588 枚零回归），红因分类记于完成记录；D11 各组逐枚对账落点
  完成记录（2026-09-09）：18 枚 `test-explore-*` 落盘，18/18 全红，红因两类——
  ① **16 枚旗标未注册**（args 携 --explore/--iterations/--no-reduce）：exit 2 + `we: unknown option "--explore"` + usage 四行（parseOptions 未注册三旗标；explore-usage-iterations 的 got 也是此类——旗标注册后 `invalid --iterations value "abc"` 路径才可达）；
  ② **2 枚 manifest 侧**（explore-manifest-invalid / -check，args 无旗标）：exit 0 vs want 1——`[test]` 表今日无读取面，E1903 死文字的直证（test 面还多打了一行 pass——测试照常跑过）。
  **D11 逐组对账**：基础 4/4（pass/flag/manifest-iterations/default）、发现 1/1（order-found，RNG 依赖预测值 attempts=2 实现后校正披露）、守卫 3/3（e1902/e1902-mocked/e1901-negative）、io 抑制 1/1、POR 2/2（no-reduce/reduce-merges）、manifest 2/2、usage 1/1、协议 1/1（json）、既有面 4/4（compile-fail/filter/single-file/empty）——19 槽 18 枚（explore-pass 兼旗标面），与 D11 表一致。
  **计数 churn 披露**：设计期 ~17 → 18 → 19 槽 → 终定 18 枚（explore-pass 兼旗标面占双槽）；proposal 目标 9 / design D11 / tasks T1 三处已同步 18。
  **负面对账**：`git status internal/conformance/testdata/` 仅新增未跟踪文件、零既有文件改动——目标 8（588 零回归）T1 侧证据落盘。
  **实现前黄金修正披露（T2 取证期发现，改 2 处）**：① explore-manifest-invalid 退出码 1 → **2**——runTestProject 对 loadManifest 诊断走 M10b 既有映射 `exitCompileFailure`（cli.go test.go:181-184），check 面保持 1（runCheck 直传）；② explore-e1902 锚 `7:5` → **`7:1`**——TestDecl 位置在 `test` 关键字（parser.go:697 Line/Col at test），设计 D5/D6 本就裁「测试声明位置」。两处均实现前自有黄金修正，未触既有黄金。

- [x] T2 测试先行 B：单测先红——codegen m10c（drive-thunk 提取形、fxgate 槽默认与恢复常量、meta/drive 调用序、m10b HarnessSynthesis 随批更新面）+ cli m10c（三旗标解析/非法值 usage 2/manifest [test] 读取与 E1903/argv 透传）+ runtime C harness m10c（splitmix64 序列表、pick 钩子、迹规范化、E1901 对比器、POR 键、fxgate 检查、死锁转换）——C 侧与实现同任务先红（M9b/M10b 惯例）
  来源：proposal 目标 2–7 / design D1 D2 D4 D5 D6 D7 D8 D9
  验证：`go test ./internal/codegen/ ./internal/cli/` 红（新用例引用未定义符号/断言不匹配），red 证据记于完成记录；runtime C 套件红因记于 T3/T4 完成记录
  完成记录（2026-09-09）：两套件落盘、红证据分类——
  **codegen m10c**（`m10c_test.go`，5 用例）：4 红 1 绿——红 = TestM10cDriveThunkExtraction（`__we_main` 仍是内联序列，无 drive thunk/meta 调用对）、TestM10cFxGateCustomEffect / QualifiedTag / MockRestore（无 fxgate 发射、槽默认仍是真体）；绿 = TestM10cBuildFaceUnchanged（负向钉：build 面今日无 meta/drive/fxgate——本就是 T5 重构期间的回归哨兵，非先红面）。
  **cli m10c**（`m10c_test.go`，4 用例）：编译红（符号未定义）——e.explore/e.noReduce/e.itersFlag/e.itersSet/e.exploreIters、resolveExploreIters、testBinaryArgs 均待 T6 定义；测试同时钉死：非法值报文形 `we: invalid --iterations value %q (want a positive integer)`、E1903 报文 `explore-iterations must be a positive integer, got {v}`（0/字符串/浮点三态）、check 面旗标仍 unknown option、默认链 flag > manifest > 100、argv 序 `--json --explore --iterations N --no-reduce`。
  **m10b 回归**：`go test ./internal/codegen/ -run TestM10b` 全绿零改动——m10b HarnessSynthesis 钉的是指令面片段（begin/await/report/install 均在重构后的 thunk 内存活），随批更新需求消解为「结构归属变化由 m10c 钉、指令面不变由 m10b+conformance 钉」（原计划「随批更新」的更优处置，披露）。
  **runtime C harness m10c**：按惯例与实现同任务先红——PRNG 对表/pick 钩子红因记 T3，对比器/门/键集/死锁红因记 T4。

- [x] T3 runtime 探索机核心：D2 取队点钩子 + splitmix64 + 种子推导；D3 runner 环与全复位；D4 三事件迹（conc.c send/receive 挂钩 + sched.c task_done 挂钩）与规范化 id
  来源：proposal 目标 2 3 / design D2 D3 D4
  验证：runtime C harness m10c 核心套件绿（PRNG 对表/基线迭代 0 FIFO/pick 随机可复现/复位后世界干净/迹事件四元组与截断位）；既有 M9b/M10b runtime 套件零回归（常态路径原样）
  完成记录（2026-09-09）：
  **先红**：`runtime/m10c_test.go` 6 用例落盘，6/6 链接红——全部失败于未定义符号（`__we_rng_seed/__we_rng_next/__we_explore_seed`、`__we_explore_on/__we_explore_arm`、`__we_trace_select/reset/count/get/truncated`、`__we_ready_count`、`__we_test_meta/__we_test_drive`），红因一类：探索面尚未存在（M9b/M10b「与实现同任务先红」惯例）。
  **期望值独立预计算**：splitmix64 首 16 输出、4 组种子推导值、pick 序（seed 1 → `bca`，按取队机制逐事件模拟）全部由 Python 独立算出后钉进测试——实现后**首次执行即逐字节吻合**（黄金可 pin 的根基实证）。
  **实现落点**：sched.c = splitmix64（sm64_step/of，常数宏）+ rng 面 + 种子推导 + `ready_count` 维护（push/pop/unlink/随机摘除四路）+ `we_pick_ready()`（未武装或 count≤1 = 现行队头；武装 = 走链摘除第 idx 个）+ `__we_explore_arm` + task_done 完成事件挂钩；test.c = 探索全局（`__we_explore_on/iters=100/reduce`）+ meta 暂存 + 迹机制（双跑对双缓冲、创建计数器、4096 截断、四元组读取含派生 seq）+ `explore_reset_world`（就绪队空防御断言→abort + 解除武装）+ `__we_test_drive`（常态直调一次；探索态 N×A×双跑环，失败即停，attempts/vdur/max 记账）+ report 探索分支（记账不打印）；conc.c = `we_chan.trace_id`（80→88 字节，desc 位图不变）+ 全部值移动完成点挂钩（见 D4 回填）；sched.h = 跨文件声明。
  **环内 harness 自修一处（披露）**：pickSceneHarness 的 scene 漏调 `__we_trace_reset()`（trace_on 未武装→迹空）——补调用即绿；期望值零改动。
  **设计修正回填 design.md（两处）**：① D4 task-id 规范化——原文「task_id = 裸创建序号，对比不依赖跨迭代同 id」对 E1901 **同尝试两跑对比**不成立（任务表 slot 单调不复用，第二跑 slot 严格更大，裸 slot 必致每对分歧）；实现 = slot − extent 基准（begin 表计数）——「规范化 id」标题承诺的兑现，实质修正；② D4 双记录机制 = 双缓冲对 + 读取时派生 seq（同观察面单一数据源，替代「追加时同步写两处」），另补挂钩覆盖定形（全部值移动点，含 try/select；关闭通路 None 不记）。
  **T3/T4 边界定形（披露）**：runner 落 D3 全环形（N×A=4×双跑、失败即停、attempts 记账），T4 判定桩 = 接受首尝试即 break——POR 重采样/守卫/对比器入 T4 不重构环体；report 探索记账分支（D10 机制性部分）随 T3 落（无它环体无法洁净运行）。
  **验证**：m10c 6/6 绿；`go test ./runtime/` 全绿（M9b/M10b/gc/conc 套件零回归——常态路径 pick 未武装即现行队头、迹挂钩单分支空转、report 探索分支被 `__we_explore_on=0` 短路）；conformance 全量对账见 T3 附记（18 枚 explore 续红属预期，既有 588 零回归为证）。

- [x] T4 runtime 判定面：D5 E1901 对比器与双面渲染 + We 级负例、D6 `__we_explore_fx_check` 门、D7 POR 键集与重采样、D8 探索态死锁转换、D10 聚合报告（人类行 + JSON explore 对象 + duration max 语义）
  来源：proposal 目标 3–7 / design D5 D6 D7 D8 D10
  验证：runtime C harness m10c 判定套件绿（对比器分歧/一致两路、注入 E1901 正例、fxgate 常态直通 + 探索态触发 + 幂等、键集去重与 no-reduce、死锁 force-fail 转换[We 级不可构造则 C 承载披露]、聚合行渲染）；黄金 explore-e1902/e1901-negative 翻绿对账
  完成记录（2026-09-09）：
  **先红**：7 新用例（E1901CompareAndRender/FxGateCheck/PorKey/DriveVerdict/GuardThroughDrive/DeadlockConvert/IoSuppressed）+ T3 driveResetHarness 断言随批更新 = **8/8 红**，实测分类：**4 链接红**（clang compile exit 1，16 处 undefined reference——五个新面 `__we_e1901_first_div/__we_e1901_render/__we_explore_fx_check/__we_explore_guard_reason/__we_por_key` 的重定位点）+ **4 运行红**（T3 桩行为 vs T4 语义：DriveResetWorld spawn=4 无聚合行、DriveVerdict 桩接受首尝试输出不匹配、DeadlockConvert 无转换面走 M10b abort、IoSuppressed io 无抑制分支全打印）。
  **期望值独立预计算**：POR 键钉值 `50e11b4212785a7f`（Python 独立实现 FNV-1a 逐字节 + 排序串接 + 截断折入，实现首跑吻合）；E1901/E1902 双面渲染字节从 internal/diag/diag.go 与 diagnostics.toml（:1433/:1442）逐字复制核对。
  **order-found 黄金校正 2→3（T1 预留路径兑现，披露）**：T1 预测 seed(0,1,0) 首抽取 b（立即失败 attempts=2）；Python 实测（本记录期以 sched.c 实现复算钉值全部吻合）——i=0 基线 FIFO 选 a 通过收类（attempts=1）；i=1,a=0 seed=40a380c0196203f2 首抽 idx=0 选 a 通过→Mazurkiewicz 同类（tied 两任务每任务子序相同）→重采样；i=1,a=1 seed=8285e3c778730970 首抽 idx=1 选 b→`assertion failed: got 2, want 1`→attempts=3 停。实现前自有新黄金修正，未触既有 588。
  **判定序缺陷（红测试揭出，D13 披露）**：首版 verdict_attempt 未在 `ex_failed` 时先停——失败尝试继续走 POR（入类集致 classes=1 应 0、判已见类致重采样 attempts=5），违反 design D3「失败即停」。driveVerdict/json fail/deadlock 三测试同根因红。修复 = 守卫分支后加失败先停（失败尝试不入类集不重采样）。
  **测试侧修正两处（期望值零改）**：e1901Harness 首版传 (1,0)/(0,0) 而渲染契约为调用方传 1 基计数、meta flen 11 应 10（JSON 渲染出 ` `）；porKeyHarness 打印 `==` 而断言性质为「异键→1」，改 `!=`。
  **TestIOHarness 编译集扩容（披露）**：io.c 四出口探索抑制引用 test.c 的 `__we_explore_on`（单一权威），gc_test.go 编译集 {gc,io,main} → {gc,sched,test,io,main}，io 行为断言 `"hi\nho"` 零改。
  **实现落点**：test.c = 判定节（diag 双面渲染 [复制 diag.go 形，C 侧披露]、E1901 纯对比器 + ev_render + message 单源 ex_div_msg、fxgate 门 + 幂等旗标、FNV-1a 64 + `__we_por_key` [匿名可重集：每任务滚动→升序排序→串接→截断字节]、开方寻址键集 [容 max(2N,8)，no-reduce 跳查留插]、flatten_pair [O(n) seq 派生 + realloc 计数行]、settle 全局、verdict_attempt 判定序 [守卫→失败先停→E1901 两侧未截断才比→POR 收/采样]）+ runner 补全（keyset 开/关、守卫跳半 2、verdict 分派、失败 reason 一次合成 `explored schedule failed: {run reason}`）+ aggregate_report（人类行 iterations=配置值/attempts=实跑/classes=键集基数 + JSON explore 对象缀 duration_ms 后 + 失败因 JSON 走 stderr）；sched.c = explore_settle（旗标未消费即 abort 形状防御 + 消息 + 非 done 全弃置 + 唤 task0）+ handle_await 循环顶消费（tag 1 + payload=消息）+ idle 分岔置于 M10b 死锁面之前；sched.h = 判定面声明块；io.c = 四出口各一探索态分支。
  **design.md 回填三段**：D5 渲染细节（纯对比器形/紧凑渲染 `<end>`/1 基计数/无 help 字段/message 单源/reason 前缀语义）、D8 实现定形（We 级确不可构造 → C harness 承载 + settle 机制落点）、D10 实现定形（JSON 失败因 stderr 面 + io 四出口 + `__we_explore_on` 单权威 + 编译集扩容代价）。
  **验证**：m10c 13/13 绿（T3 六 + T4 七）；`go test ./runtime/` 全包绿（10.7s，M9b/M10b/gc/conc 零回归）+ `go vet ./runtime/` 净；conformance 全量（/tmp/m10c-conf-t4.log）——**18 失败全 test-explore-*（T5/T6 未落：`we: unknown option "--explore"`），既有 588 零回归**。黄金 e1902/e1901-negative 翻绿对账需 T5（codegen fxgate 发射）/T6（CLI 旗标）落地——本任务验证条的对账边界在此，全量绿对账归 T8。

- [x] T5 codegen：D1 drive-thunk 提取 + `__we_test_meta`/`__we_test_drive` 一形两模式；D6 fxgate 直通门发射（六族 ABI 克隆、槽初值与恢复常量换 fxgate、真体 define 名不变）；m10b HarnessSynthesis 随批更新（披露）
  来源：proposal 目标 2 5 / design D1 D6
  验证：`go test ./internal/codegen/` 绿（m10c 套件 + 更新后的 m10b 套件 + 既有全绿）；conformance 既有 588 枚零回归（不带 --explore 行为逐字节不变——fxgate 常态直通 + 驱动重构行为中性的实证）
  完成记录（2026-09-09）：
  **先红**（T2 落盘）：m10c 4 红（DriveThunkExtraction——`__we_main` 仍内联序列无 thunk/meta 对；FxGateCustomEffect/QualifiedTag/MockRestore——无门发射、槽默认仍是真体）+ 1 绿哨兵（BuildFaceUnchanged——负向钉 build 面无 meta/drive/fxgate）。
  **实现落点**：codegen.go——declare 表加 `__we_test_meta`/`__we_test_drive`/`__we_explore_fx_check` 三行；driveStep 扩 key/line/col；emitDriver 重写为提取形（每测试步骤序列原样入 `define internal void @<key>.drive.<i>`（i = 驱动全局序号，种子身份），`__we_main` 只携 meta/drive 对 + summary——指令流逐条不变，m10b HarnessSynthesis 片段在 thunk 内存活全绿零改动）；fxgate = `isCustomEffect`（内建 = io/net/time 三裸键，合格形与裸自定义皆 custom）+ `fnSlotTarget`（custom → `@<key>.<name>.fxgate`，否则真体）+ `emitFxGate`（六族 ABI 克隆：参数带类型前缀原样转发、check 入口、直调真体不回槽、void 族 `ret void`/值族 `%r` 固定名[bindDefineParams 碰撞接受先例同款披露]）；三处接线 = emitFnDefine 尾（门发射 + 槽默认换门）、emitFnCall 槽位、mockTarget 恢复常量（两分支）。
  **测试侧修正两处（T2 笔误，从未绿过，披露）**：① flen 钉 14 实为 `len("tests/m_test.we")`=15（m10b 无此钉可对照，纯转写误数）；② entry body want 序列漏 `e.inst` 两空格缩进。两处均为期望值转录修正，断言意图零改。
  **验证**：`go test ./internal/codegen/` 全绿（m10c 5/5 + m10b + 既有）；`go test ./runtime/` 绿；conformance 全量 = **18 失败全 test-explore-*（T6 旗标未注册），既有 588 零回归**——驱动重构行为中性（常态 drive 直调 thunk 一次）+ fxgate 常态直通（check 即返）的双实证。

- [x] T6 CLI：D9 三旗标解析 + manifest [test] 读取与 E1903 + 迭代默认序（显式 > manifest > 100）+ argv 透传 + startup.c 三旗标解析
  来源：proposal 目标 1 / design D9
  验证：`go test ./internal/cli/` 绿；黄金 explore-manifest-*/explore-usage-iterations/explore-default-iterations 翻绿对账；check/build/run 面 E1903 一致（共享 loadManifest 直证）
  完成记录（2026-09-09）：
  **先红**（T2 落盘）：cli m10c 4 用例编译红——env 五字段/resolveExploreIters/testBinaryArgs 符号未定义。
  **实现落点**：cli.go = env 加 explore/noReduce/itersFlag/itersSet/exploreIters 五字段 + parseOptions 三旗标 case（test 专属，`--iterations` 两形式、last-wins）+ setIterations（Atoi + n>0；非法/缺值 = usage 2，报文 `we: invalid --iterations value %q (want a positive integer)`，缺值形无 %q——黄金钉 `%q` 形 `"abc"` 吻合）；check.go = parseManifest 非空 section 键改携 `section.` 前缀出面（`test.explore-iterations` 键形；[dependencies] 计数不变、无既有读者——name/version/type 顶层键不受影响）+ loadManifest type 校验后加 E1903 门（键存在即须正整数，报文 `…got {v}` 裸值无引号；表内其余键不校验[与 {vet} 同姿势]；check/build/run/test 四面共享直证）；test.go = resolveExploreIters（flag > manifest > 100，manifest 值防御性复验——单文件面无 manifest 传空）+ testBinaryArgs（`--json` 先、explore 面成组：`--explore` + `--iterations` + 数值**两元素** + `--no-reduce`；普通跑零参）+ runTestProject loadManifest 后解析入 e.exploreIters、runTestSingle 无 manifest 解析、runTestBinary 接 argv；startup.c = main 四旗标扫描（--json 先例扩展）置 `__we_explore_on`/清 `__we_explore_reduce`/`--iterations` 取下词 atoll（CLI 恒发两词形；杂散值不解析保持内建 100）。
  **实现侧缺陷两处（黄金揭出，D13 披露）**：① 首版 testBinaryArgs 把 `--iterations 3` 拼成**单 argv 元素**（字符串内含空格）——子进程匹配不上任何旗标，explore 黄金全以 100 迭代跑（attempts 397 等）；T2 的 strings.Join 断言对此盲（join 后字形相同）——join 盲区。修复 = `"--iterations"` + `strconv.Itoa(n)` 两元素。② order-found 黄金源 T1 笔误：`st.assertEqual(first, 1)` 拿 `Option<Int64>` 直接比 Int64——命 Eq-generic 域边界 exit 70，**从未编译通过**。实现前自有黄金修正（未触既有 588）= match 解 Some 载荷再断言（None 臂防御 assert(false)），语义意图零改（首收载荷应等 1；交换调度首收 2 即败）。match 分支零调度事件——钉死的 `3 iterations, 3 attempts, 1 classes` 与原因行 `got 2, want 1` 修正后**首跑逐字节吻合**（RNG 推导与调度中性双实证）。
  **验证**：`go test ./internal/cli/` 绿（m10c 4/4 + 既有）；`go vet ./...` 净——T2 红符号已定义，**全仓 vet 自 T2 起首次全绿**；`go test ./runtime/` 绿（startup.c 不入 C harness 编译集——runtime 套件用自带 harness main.c，真二进制面由 conformance 承载）；conformance 全量 **606/606**（18 枚 explore 全翻绿 + 既有 588 零回归）；`go test ./...` 全包绿。翻绿对账：manifest-invalid（test 面 exit 2 = exitCompileFailure 映射）/manifest-invalid-check（check 面 exit 1，共享 loadManifest 直证）/usage-iterations/default-iterations（100 行面）+ 其余 14 枚全绿。

- [x] T7 黑盒电池：真二进制逐面探针——explore-order-found 场景真机复现（首个失败迭代号与黄金一致）；--json schema python 断言（explore 对象字段）；--iterations N 界实测（行内 N 与 attempts 关系）；--no-reduce classes == iterations 直证；mock 后 E1902 静默；既有 we test 无旗标回归抽检
  来源：proposal 目标 1 4 6 7 / design D9 D10 D11
  验证：电池探针全过，逐探针输出记于完成记录
  完成记录（2026-09-09）：真二进制 `go build ./cmd/we`（we 0.1.0 / spec 0.9.0），夹具复用黄金源（order-found 修正版/tied/gate-mocked），六探针全过——
  ① **order-found 真机复现**：`we test . --explore --iterations 3` → exit 1 + `fail  tests/order_test.we: order (explore 3 iterations, 3 attempts, 1 classes)` + 缩进原因行 `explored schedule failed: assertion failed: got 2, want 1`——与黄金逐字节一致；attempts=3 按计数公式唯一钉死**首败迭代 i=1（第 2 迭代）a=1**（基线 i=0 过收类 + i=1,a=0 同类重采样 + i=1,a=1 败），与 T4 Python 推导吻合。
  ② **--json schema**（python 逐字段断言）：test-result 键集恰为 `{type,file,name,status,duration_ms,explore}`；explore 对象键集恰为 `{iterations,attempts,classes}` 全 int，值 2/5/1；test-summary 键集 `{type,total,passed,failed,duration_ms}` 零改；stdout 恰两行。
  ③ **--iterations N 界实测**：N=1/5/12 → 行内 N 逐一同值；attempts 1/17/45 = **1+4(N−1)** 公式逐点吻合（tied 单类：基线后每迭代 4 次同类重采样 + 1 收）。
  ④ **--no-reduce**：N=3/6 → attempts == iterations == N（重采样关的直证）；classes=1——tied 的交换调度同属一个 Mazurkiewicz 类（D11 自钉 classes=1）。T7 任务行原文「classes == iterations」措辞宽泛：no-reduce 的直证面是 **attempts**，classes 恒为该源等价类数（披露）。
  ⑤ **mock 后 E1902 静默**：`mock save() effect db` 后探索跑 `pass … (explore 3 iterations, 9 attempts, 1 classes)`，E1902 零出现。
  ⑥ **既有 we test 无旗标回归抽检**：三夹具（tied/order/gate）普通跑全 pass、行形 `name (10ms)` 无 explore 子句、exit 0——M10b 面零改；order 普通跑过 = 基线 FIFO 交付 a 序（探索才是发现者，常态与探索态语义分界的真机直证）。

- [x] T8 全量终验：D12 阶梯固定序全绿 + 对账（refr/ 零触碰、既有黄金零触碰、conformance 总数对齐 proposal 计数）
  来源：proposal 目标 8 9 / design D12
  验证：`go build ./...`、`go vet ./...`、`go test ./...`、runtime C harness、conformance 全量、validate --all --strict、docs_sync（变更创建态基线）、git status 对账——全绿记录
  完成记录（2026-09-09）：D12 阶梯固定序**八步全绿**——
  ① `go build ./...` OK；② `go vet ./...` 净（全仓）；③ `go test ./...` 全包 ok（cli/codegen/conformance/diag/lex/parser/typecheck/version/runtime 九测试包）；④ runtime C harness 含于 runtime 包全绿（M9b/M10b/gc/conc/m10c 13 用例零回归）；⑤ conformance 全量 fresh run ok——**606/606**（588 既有 + 18 test-explore-*，与 proposal 目标 9 计数对齐）；⑥ `validate.py --all --strict` OK（1 change valid; registry clean）；⑦ `docs_sync.py` OK（31 document pairs aligned——变更创建态基线，M10b T8 判明口径）；⑧ git status 对账：**refr/ 零触碰**（grep 零匹配）；**既有黄金零触碰**（`git diff internal/conformance/` 空——18 枚全为新增未跟踪文件）；改动面 = 11 个跟踪文件（cli×3 / codegen / runtime c×6 / gc_test）+ 新增 3 测试文件 + 18 黄金 + 本变更目录，全为 M10c 材料，无预期外文件。

- [x] T9 实现审查：welang-code-review 7 条（规范符合性/验证诚实性/测试先行证据/诊断协议稳定/单一权威/红线复核/最小可信验证已跑），产出 = proposal.md「## 审查记录」+ status → complete
  来源：仓库节奏（M10a/M10b 先例）
  验证：7 条逐条过，发现当场处置或披露，审查记录落盘
  完成记录（2026-09-09）：7 条全过，审查记录落 proposal.md「## 审查记录（2026-09-09，welang-code-review 7 条，实现完成后 active → complete 关）」——①规范符合性（E1901/E1902 title 与 diagnostics.toml:1433/:1442 逐字比对、ch21 迭代 0 基线/POR/两守卫/退出码全由黄金钉死）②验证诚实性（T1–T8 逐项核对，T4 判定序缺陷红测试揭出当场修复 = 验证面起作用的自证）③测试先行证据（18 黄金先红 + 单测先红 + C 侧同任务先红）④诊断协议稳定（--json 只增 explore 可选对象，零码分配）⑤单一权威（`__we_explore_on` 单权威、diff 全量 CJK 扫描零命中）⑥红线复核（refr/ grep 零匹配、既有黄金 `git diff internal/conformance/` 空）⑦最小可信验证已跑（606 fresh + 真二进制六探针）。实现期发现处置汇总随附（T1 两处修正 + order-found 源笔误、T2 笔误两处、T3 D4 回填两处、T4 判定序缺陷 + 笔误两处 + 编译集扩容、T6 argv 缺陷；m10b 随批更新消解为零改动）。change.yaml `status: active` → `complete`。
  （勾选披露：本记录与复选框翻转在 T10 归档期补落——前窗口审查已完成、proposal.md 记录与 status 翻转在位，tasks.md 勾选遗漏。）

- [x] T10 归档：welang-archive-sync 五步（本变更零规范提升零新 ADR——机制载体 = 变更记录 + roadmap）+ roadmap M10c 行 pending → done（双语）
  来源：仓库节奏
  验证：validate --all --strict 复验绿；归档目录带日期前缀
  完成记录（2026-09-09）：archive-sync 五步——① 规范提升零（纯实现路径，无 specs/ 增量）；② 决策提升零新 ADR（M10c 四裁决为机制实现态，长期载体 = 本变更 design/proposal/审查记录 + roadmap M10c 行，未达 ADR 级架构承诺——M9b/M10b 先例同姿势）；③ roadmap M10c 行翻 `done 2026-09-09`（en/zh 两文件，第 30 行）；④ `mv openspec/changes/testing-explore openspec/changes/archive/2026-09-09-testing-explore`（目录未跟踪，mv 非 git mv）+ change.yaml `status: complete` → `archived`；⑤ 复验全套过。
  **归档期复验拦下一处（披露，D13）**：`gofmt -l` 揪出 `internal/cli/m10c_test.go` 未格式化——T2–T6 验证门（go test/go vet/conformance）不含 gofmt，结构体字段对齐空白漏网（TestM10cResolveExploreIters 用例表四字段）。`gofmt -w` 修复（零行为改动，diff 纯空白），`go test ./internal/cli/` 复验绿。T8 阶梯不含 gofmt 属验证清单缺口，如实记录；M10b T10 验证面（gofmt -l 空）本次补齐。
  **验证对账**：`validate.py --all --strict` → `OK: no changes found (nothing to validate); registry clean`（active 目录清空之谓，M10b 归档后同形）；docs_sync `31 document pair(s) aligned`（对数不变）；gofmt -l 空（修复后）；`go build ./...`/`go vet ./...` 过；`git diff --check` 干净；归档目录 `archive/2026-09-09-testing-explore/` 在位、roadmap M10c 行 done（双语）。

- [ ] T11 记忆与提交：project-overview 增 M10c 全量条 + MEMORY.md 索引行更新；提交信息草案呈报，**候用户明示**
  来源：仓库节奏（提交须明示）
  验证：记忆回写；提交草案就绪；未提交状态确认
