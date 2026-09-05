# native-vertical — 最小 LLVM IR 发射 + 最小 C 运行时：`we build` 与 `we run` 对骨架端到端

## Why

M3（types-and-main，归档 2026-09-05）交付了类型阶段：`we check .` 对 `we new` 骨架绿（静默 exit 0）。但管道到此为止——`we build` / `we run` / `we clean` 至今是 M0 的三行诚实边界（stderr 一行 + exit 70），没有任何输入能产出一个工件或执行一个 We 程序。按 roadmap M4 行（薄垂直优先策略的兑现点），下一阶段是把已批准的管道契约接到真实后端：

- 第 21 章 R2（`docs/spec/2100-toolchain.md`，The compile pipeline observable contract）：build 与 check 共用同一次序的七级管线，**check 在文档检查后停止、不产工件；build 继续穿代码生成并产出 manifest 命名的工件；run 先构建后执行**；同输入同输出是唯一性能承诺（时间不计）；链接属 build（E1906 只在 build 触发——本切片 foreign 块仍停解析边界，E1906 不可达，design D9 披露）；
- 第 21 章 R1：`we clean` 移除构建产物、不动源与 manifest；`[path]` 目录 = 项目、文件 = 单文件编译；E1907 分发前触发；
- 第 15 章 R6（`docs/spec/1500-modules.md`，The main convention）：main 先于一切用户代码执行（ch15 R5 初始化次序——本切片无顶层绑定接受面，design D8 披露）；`Ok(())` → exit 0；`Err(e)` → 在 stderr 报告失败消息后非零退出——**消息格式是标准库的自由度**，本切片的最小格式在 design D5 钉死为事实上的稳定表面；main 无参数；
- 第 22 章 R5（`docs/spec/2200-dependencies.md`）：acquisition 在管线前运行——空依赖集平凡满足；非空依赖集的获取未实现，诚实边界（本切片新增面，M3 从未定义）；
- ADR-0002（`docs/decisions/ADR-0002-llvm-first-commercial-runtime.md`）：LLVM 引脚 21.1.8；编译器 v1 用 Go **发射文本 LLVM IR 并驱动 LLVM 工具链**；运行时 C 源用钉版 clang 编译；长期自举。

可验证的黑盒缺口：`we new demo && cd demo && we check .` 绿之后，`we build .` 打印 `we: build is not implemented in this reference build yet`（exit 70），`we run .`、`we clean .` 同；磁盘上永远没有 `build/`、没有二进制、没有任何一次用户程序的执行。本里程碑交付 roadmap 承诺的最小端到端：`we new` → `we check` → `we build`（产 `build/<name>`）→ `we run`（Ok 路径 exit 0 / Err 路径 stderr 一行 + exit 1）。

## 裁决记录（candidate 阶段四项表面裁决，2026-09-05）

- **Q1 代码生成深度 → 骨架 + Err 路径**：代码生成覆盖和式声明擦除（零 IR）与 main 形 `Result<(), E>` 的两个返回形——`return Ok(())` 编译为 exit 0 路径；`return Err(V("msg"))`（V 为单 String 载荷变体、载荷为无插值孔的纯字符串字面量）编译为 stderr 一行 `error: V: msg` + exit 1；其余一切类型检查干净的形式（绑定、运算符、调用、其他函数、顶层 let、尾表达式形返回、Err 载荷以外的字符串构造）→ 诚实边界 exit 70（What 表封闭，design D3）。
- **Q2 产物布局与 `we clean` → `build/` 目录 + clean 落地**：工件在项目根 `build/<name>`（名取 manifest）；`we clean` 删除整个 `build/`（幂等：目录缺席时静默 exit 0；`--verbose` 逐行列出被删除的路径）；`we build --verbose` 打印一行 `we: built build/<name>`；成功三面与 check 同形（静默 0 / `--json` 零事件 / `--verbose` 一行）。
- **Q3 单文件 `[path]` 的 build/run → 边界行 + follow-up 登记**：`we build tool.we` 先跑完管线（诊断正常报告，exit 1），干净后在工件命名处停边界 exit 70——单文件编译没有 manifest，工件无名可取，是规范缺口（ch21 R2 说工件由 manifest 命名）；登记 roadmap follow-up（#6），离开该表只能经规范层变更。
- **Q4 manifest `type = "library"` 的 build → executable-only，library 边界**：manifest 三键校验全过后（E1905/E1904/E2004/E1903 照常），在工件步停边界 exit 70（`library artifacts (chapter 21)`）；`we check .` 对 library 项目的行为维持 M3（check 无工件面，不改写）。

## 目标与非目标

### 目标

1. `we build`（项目、executable）：与 check 共用项目加载（manifest 校验 → 源根 → 根模块解析 → 类型检查，次序与报文零改写——101 枚既有黄金不动为证），然后穿代码生成：发射文本 LLVM IR 到 `build/<name>.ll`，用钉版 clang 编译内嵌 C 运行时为对象、驱动链接，产 `build/<name>`；成功三面（静默 0 / `--json` 零事件 / `--verbose` 一行 `we: built build/<name>`）。
2. `we run`（项目）：先构建（同布局、同三面），后执行 `build/<name>`（子进程 stdio 直通、cwd = 项目目录），传播子进程退出码；Err 路径的 exit 1 即 ch15 R6 的非零。
3. `we clean`（项目目录）：删除 `build/` 整目录；幂等；`--verbose` 确定序列出被删路径；对 `.we` 文件路径 = usage 错误（机制，design D1）。
4. `internal/codegen`（新包）：接受集判定 + 边界 What 封闭表（design D3）+ 文本 IR 发射（符号约定 `__we_main` / `__we_fail`、字节级字符串常量双向逃逸、模块名 = manifest 名、零优化旗标，design D4）。
5. `runtime/`（新目录）：C 启动源（main → `__we_main`、`__we_fail` 报告函数）与分配器 ABI 种子（`__we_alloc` / `__we_free`，M4 生成码不调用、ABI 先钉——roadmap M4 行点名 allocator），Go 侧 `go:embed` 内嵌、构建时落盘按需编译（design D6）。
6. LLVM 驱动与钉版门：PATH 上的 `clang`，版本必须等于 `internal/version.LLVMPin`（21.1.8）；缺失/不符 → stderr 一行（非诊断、非边界）exit 1（design D7）；clang 作为编译与链接 driver（opt/llc/lld 直驱延后，design D6 披露）。
7. 管线前依赖面（ch22 R5）：manifest 含非空 `[dependencies]` → check/build/run 一致边界行（What = `pipeline commands with a non-empty dependency set (chapter 22)`——边界行语法要求复数名词短语，「管线命令」恰是该义务覆盖的命令组）；空集/缺席 = 平凡满足（M3 行为不变）。
8. 确定性（ch21 R2 唯一性能承诺）：同输入同输出——.ll 文本确定（单测逐字节）；同一项目两次构建的二进制字节一致（同钉版工具链，单测）。
9. conformance：黄金新增 22 枚（build/run/clean 三面 + Err 端到端 + 边界行代表 + 单文件两序 + library + 依赖非空边界 + 共享管线 E1905 + clean usage + run 单文件分派），全部先红；既有黄金零改写（含 M0 两枚 E1907 前置路径黄金——分发前触发不受影响）。

### 非目标

- **代码生成的其余形式**：运算符、绑定、调用、控制流、复合值 → M5 随各章落地；字符串插值的运行时构造 → 后续（本切片插值载荷停 Err 载荷边界，design D3 折叠披露）。
- **优化、target 选择、debug 信息**：零旗标、宿主缺省 triple、无调试信息（ch21 R2 明言不承诺性能；mechanism）。
- **E1906 链接诊断**：foreign 块维持 M2 解析边界（bndFFI），E1906 在本切片不可达——design D9 声明式披露。
- **依赖解析本体**（MVS、we.lock、缓存）→ M13；本切片只加「非空依赖集 → 边界」的诚实面。
- **库工件的形状**：ch21 只说「工件由 manifest 命名」，library 工件无已批形状 → 边界行（Q4），不发明。
- **多模块编译、`?` 传播、panic 家族、List/Map/Set 等 M3 边界全维持**；`we test/fmt/vet/doc/lsp` 维持 M0 边界。
- **运行时分发形态**（预制对象/位码随工具链分发）→ 后续打包故事；本切片源内嵌 + 按需编译（design D6 披露与 ADR-0002 的关系）。

## What Changes

- 新增 `internal/codegen/`（接受集 + What 表 + IR 发射 + 按章单测）。
- 新增 `runtime/`：`startup.c`、`alloc.c`、Go 内嵌文件（`go:embed`）。
- 新增 `internal/cli/build.go`：项目加载抽取（check 共用）、runBuild / runRun / runClean、依赖非空边界。
- 修改 `internal/cli/cli.go`（build/run/clean 三行翻 implemented）、`internal/cli/check.go`（项目加载函数抽取，行为零改写）。
- 新增 conformance 黄金（22 枚，零改写既有）。
- `docs/roadmap/0000-reference-implementation.md` 与 `.zh.md`：M4 行翻 done；follow-up #6（单文件构建工件命名）登记（归档动作）。

## 影响层

`compiler`（代码生成与运行时的最小切片——已批准章节与 ADR-0002 机制决策的实现，无规范增量）、`tooling`（`we build` / `we run` / `we clean` 可观察表面：第 21 章既有行为的部分实现，无规范增量）。**本变更不改变语言行为、无规范增量**：一切接受/拒绝判定以已批准的 `docs/spec/2100-toolchain.md`、`1500-modules.md`、`2200-dependencies.md` 及注册表为依据（零新增/修改诊断码）；Err 报告行的格式与退出码取值是 ch15 R6 明文留给标准库/机制的自由度，在 design D5 钉死并作为事实上的稳定表面记录；单文件工件命名这一规范缺口以 roadmap follow-up 登记，不以本变更私定行为。

## 影响范围

- 新文件：`internal/codegen/codegen.go`、`internal/codegen/codegen_test.go`、`internal/cli/build.go`、`internal/cli/build_test.go`、`runtime/c/startup.c`、`runtime/c/alloc.c`、`runtime/runtime.go`、22 个 `internal/conformance/testdata/cases/*.json`。
- 修改：`internal/cli/cli.go`（子命令表三行）、`internal/cli/check.go`（loadProject 抽取）。
- 不动：`docs/spec/`、`diagnostics.toml`、`go.mod`、`internal/lex`、`internal/diag`、`internal/version`（LLVMPin 既有）、`internal/ast`、`internal/parser`、`internal/typecheck`、全部既有黄金用例。

## 审计记录

**2026-09-05，candidate → ready，welang-spec-impact-audit 七条，通过（一处码位勘误已当场修正）。**

1. **问题真实性 ✓**：Why 的黑盒缺口本日真机复验——`we run .` / `we build .` / `we clean .` 对骨架项目均为 `we: <sub> is not implemented in this reference build yet` exit 70；磁盘无 `build/`、无二进制、零次程序执行。每条依据引用固定：ch21 R1/R2/R11（`docs/spec/2100-toolchain.md`）、ch15 R5/R6（`1500-modules.md`）、ch22 R5（`2200-dependencies.md`）、ADR-0002（`docs/decisions/ADR-0002-llvm-first-commercial-runtime.md`，LLVM 21.1.8 引脚与「文本 IR + 驱动工具链」机制）。
2. **影响层声明 ✓**：change.yaml `layers: [compiler, tooling]` 与 proposal 影响层一致；纯内部实现 + 既有行为的部分实现，影响层一节携带「本变更不改变语言行为、无规范增量」边界句，无 spec 层义务（M0–M3 internals-only 豁免路径第四个消费者）。
3. **规范增量范围 ✓**：零新增/修改/删除 Requirement 与诊断码（注册表不动）。触达的码全部既有且经共享管线原样复用（E1905/E1904/E2004/E1903/E1305 + 解析/类型段各码 + E1907 分发前）；E1906 声明式披露不可达（foreign 停解析边界）。Err 报告行格式与 Err 退出码取值是 ch15 R6 明文留给机制/标准库的自由度（design D5 钉死并披露），不构成诊断面。单文件工件命名缺口以 roadmap follow-up #6 登记（Q3 裁决义务），非私定行为。
4. **原则一致性 ✓**：无隐式转换、无新语法面；D2「管线先于边界」是 M2/M3 既立「形式到达才出边界」原则在新维度的沿用；代码生成接受集的形状识别在类型检查之后（语义判定不旁路）；非空依赖集边界把 ch22 R5 对 check/build/run 的既有义务接到诚实面上（此前未定义），属补全非突破。
5. **参考基线固定 ✓**：本变更零 refr/ 引用；章节引用全部固定到 `docs/spec/` 文件与 Requirement 名；ADR 引用固定文件名与钉版号（21.1.8，与 `internal/version.LLVMPin` 及本机 Ubuntu clang 21.1.8 (6ubuntu1) 一致）。
6. **验收边界 ✓**：目标按可观察面机械判定（exit 码、stdout/stderr 逐行、`build/<name>` 存在性、双构建字节一致、22 枚黄金）；非目标显式排除优化/target/debug、其余代码生成形式（M5+）、依赖解析本体（M13）、库工件形状、运行时分发形态、`we test/fmt/vet/doc/lsp`，蔓延风险受控。
7. **粒度 ✓**：一个里程碑一个变更（roadmap 执行状态规则）；单一切片可垂直验证（`we new` → `we check` → `we build` → `we run` 两路径真机端到端）；多章汇聚由管道契约的跨章强制力解释（ch21 定次序、ch15 定入口语义、ch22 定前置、ADR-0002 定后端机制），无需拆分。

（勘误：design D12 行 10 原写 E0007，本日真机验证未闭字符串报 E0002——已更正。）

## 审查记录

**2026-09-05，ready → active，welang-change-review 十条，通过（两处发现已当场修正）。**

1. **proposal 职责 ✓**：Why 为黑盒缺口 + 章节引用（本日真机复验边界行）；机制细节下放 design（proposal 仅按 M0–M3 先例点名包名、目录与可观察面）。
2. **spec 增量 ✓（无增量变更的空判定）**：本变更零 spec delta；「本变更不改变语言行为、无规范增量」边界句在 影响层 一节，无 BCP 14 义务可违。
3. **design ✓**：路径唯一最小（D1–D12）、被拒方案十二款有理由（D11）、引用精确到 Requirement 名与 ADR 文件。发现 2 已修正：依赖边界与 library 边界同在场时的先后未钉——D2 补「依赖边界先触发」并给可观察差异论证（两行皆 exit 70，差异只在 What 文本，钉死求确定）。
4. **tasks ✓**：T1–T10 每项有 来源/验证；无 deferred、无未决方案选择（T8 归档目录名日期为实现日回填，非方案占位）。
5. **场景覆盖 ✓（发现 1 已修正）**：normal（骨架两路径三面端到端）/ boundary（单文件两序、library、What 表四行、依赖非空、clean 幂等与 usage）/ failure（共享管线诊断、词法坏文件、工具链缺失单测、clang 失败报告）齐备。发现：三处路径无黄金实证——build 下的共享管线报文（E1905）、clean 对 .we 文件的 usage 2、run 的单文件分派——D12 表补钉为 #20/#21/#22（19→22 枚，proposal/tasks 计数同步）。
6. **无空章节 ✓**：审查记录为实现期门占位（M3 先例），其余无凑格式内容。
7. **测试先行 ✓**：T1（22 枚黄金对现构建红——build/run/clean 现为 70 边界、check-deps 对 M3 亦红）与 T2（codegen 包未实现编译红 + 门/确定性用例红）先于一切实现任务。
8. **负向断言 ✓**：每个拒绝声明（What 表四行 + 单文件 + library + 依赖非空 + clean usage）在 D12 有实样例黄金（#9–#16、#21、#22），非文字断言；逃逸与确定性有单测负例。
9. **完成度闭环 ✓（读法记录）**：原则 10 三要素（类型检查、代码生成、运行时）中，类型检查由 M3 交付且本变更显式零改写（既有黄金不动作证）；本变更是代码生成 + 运行时两要素的落地切片，工具链驱动（D6/D7）为其宿主机制。与 M3（检查阶段切片）过同一门的先例一致。
10. **未决问题阻塞 ✓**：四项表面裁决已答并烘焙；单文件工件命名是已裁决的 follow-up #6 登记义务（临时行为 = 边界 70，已定决策）；Err 行格式与退出码取值是 ch15 R6 明文机制自由度（D5 钉死），非未决。

## 实现审查记录

**2026-09-05，active → complete，welang-code-review 七条，通过（一处发现已当场修正；全部结论以真二进制证据为准）。**

1. **规范符合性 ✓**：本变更零规范增量，一切接受/拒绝判定对照已批章节——build/run/clean 的可观察面（ch21 R1/R2：`we clean` 删 build/ 幂等、build 穿代码生成产 manifest 命名工件、run 先构建后执行传播退出码）、Err 路径（ch15 R6：`error: Failed: boom` stderr + exit 1，格式取 D5 钉死的最小面）、依赖面前置（ch22 R5：check/build/run 一致）。真机证据：`we new`→`check`（静默 0）→`build`（静默 0、build/ 落 demo+demo.ll+rt-*）→`run`（Ok exit 0 / Err `error: Failed: boom` exit 1，直跑 `./build/demo` 同输出同码——工件自含）；边界行六类逐行复验（fn / library / deps 对 build 与 check / 单文件两序：E0002 诊断先出、干净后 70 / E1905 共享管线 / E1907 分发前）全部与 design 钉死文本逐字节一致。
2. **验证诚实性 ✓**：tasks.md 已勾选项逐条复核——T1 红证据（22 枚失败清单在完成记录）、T2 编译红、T3 真机 clang -c、T4/T5 测试侧修正按协议披露（逃逸语法与长度手算两类）、T6 `go test ./...` 全绿 + 真机端到端、T7 六命令阶梯本日当场复跑全过（gofmt 空、build/vet 过、validate --strict 过、docs_sync 30 对齐、git diff --check 干净）。无「勾了没跑」项。
3. **测试先行证据 ✓**：22 枚黄金与两组单测先于实现存在且红（失败清单在完成记录）；黄金期望输出沿用 §52 人类行与 §62 JSON Lines 既有协议（harness 零改动，123/123 全绿含 101 既有零改写——loadProject/loadFile 抽取行为零改写的实证）。
4. **诊断协议稳定 ✓**：`--json` 事件字段零改动；零新增/修改诊断码（E1905/E1904/E2004/E1903/E1305/E0002/E1304/E1907 全部既有原样复用，真机输出核对）。
5. **单一权威 ✓**：代码注释与 docs 英文（新文件逐个核对：codegen.go、build.go、check.go、cli.go、runtime.go、两个 .c、两个 _test.go）；变更目录不滞留长期事实——Err 行格式与 IR 符号 ABI 是 ch15 R6 明文留给机制的自由度，钉在 design D4/D5 并披露「后续 stdlib 落地时经其自身变更定形」，不属待提升的规范内容（归档动作核对：docs/spec/ 与 diagnostics.toml 本变更零触碰）。
6. **红线复核 ✓**：`git status` 无 refr/ 路径（未提交任何内容，提交在 T10 且钩子双保险）；diff 范围与影响范围一致——发现 1 已修正：`internal/cli/build_test.go` 实为新文件但未列入影响范围清单，已补（纯文档遗漏，无越界改动）。
7. **最小可信验证已跑 ✓**：T7 阶梯全过（输出在 tasks 完成记录）；另按 diff 形状加跑真机黑盒端到端（上列第 1 条证据）。
