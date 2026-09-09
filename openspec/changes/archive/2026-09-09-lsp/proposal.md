# proposal — lsp（M14）

## Why

roadmap M14 行：The LSP document and `we lsp`。`we lsp` 自 M0 起是 11 子命令表中最后一个诚实边界（cli.go `takesPath: true` 注册 + 边界 70 行）；ch21 R12（What the toolchain does not fix）把 LSP 指名留在规范外——「The Language Server Protocol lives in a separate document of its own」——而规范面唯一承诺已钉死：场景「Editor diagnostics are check's diagnostics」——编辑器服务的诊断就是 `we check` 的，同一管线同一码，编辑器面不得偏离命令行判决。本里程碑兑现两件：LSP 绑定文档落盘（separate document 从「指名留白」变「实际存在」）+ `we lsp` 从边界行变真实服务。本变更不改变语言行为——纯实现 + docs 文档，规范零增量，`docs/spec/` 零触碰（internals-only 豁免路径，M0 起先例）。

## 目标与非目标

目标（三件，一条垂直线）：

1. **LSP 绑定文档（docs/lsp.md + lsp.zh.md，顶层双语文档）**：ch21 R12 separate document 的实际载体——协议基线（LSP 3.x base protocol + JSON-RPC 2.0）、stdio 传输（Content-Length 头帧）、生命周期方法集、能力面（textDocumentSync full + publishDiagnostics）、**诊断映射规则**（We 诊断 → LSP Diagnostic 的逐字段规则，severity 四级映射 error/warning/note/help → Error/Warning/Information/Hint、code=注册表码、source="we"、message=title — detail 形）、以及一致性承诺的复述（LSP 不引入任何新码——一切诊断是 check 管线的既有码）。docs/README 责任图补行；docs_sync 31→32 对。
2. **`we lsp` 服务实现**：自实现最小 wire 层（纯标准库——JSON-RPC 2.0 编解码 + Content-Length 帧，零第三方依赖纪律延续）+ 服务会话机：initialize/initialized/shutdown/exit 生命周期、didOpen/didChange（full 同步）/didClose 文档同步、每次文档变更推送 publishDiagnostics——**诊断管线复用单文件 check 面**（parse + typecheck SingleFile：`we check file.we` 的逐字同面，std.* 内建解析、其余 import E1302——与 CLI 判决零分歧的字面兑现）。
3. **测试面**：Go 测试真子进程会话（internal/cli/lsp_test.go 起真 `we lsp` 二进制，客户端模拟写请求帧读诊断帧）钉死：生命周期协议序、诊断一致性（同一源文件，CLI `we check --json` 的事件流与 LSP 推送的 diagnostic 逐字段对拍）。

非目标：

- hover / definition / completion / formatting / semantic tokens / code actions 等编辑能力（后续里程碑的增量面；文档的能力面如实只列本里程碑所启）。
- 项目面诊断（模块图 + 依赖解析感知的 didOpen——本里程碑每个 open 文档按单文件面 check，与 `we check file.we` 同形同判；项目面感知是后续裁决点）。
- incremental 文档同步（full 同步起——简单且确定性）；socket 传输（stdio 起）；多工作区（rootUri 单根）。
- conformance 黄金增量（runner 是单发命令模型，LSP 是交互会话——形状不合，裁决 Q4）。

## What Changes

- 新 `docs/lsp.md` + `docs/lsp.zh.md`（英文权威 + 中文对照，LSP 绑定文档）+ `docs/README.md`/`README.zh.md` 责任图补行。
- 新 `internal/lsp` 包（wire 层 + 服务会话机）：帧编解码（Content-Length 头 + JSON body）、JSON-RPC 2.0 三消息形（request/response/notification）、服务循环（读→分发→响应/推送）、诊断管线注入面（一段源文本 → `[]diag.Diagnostic` 的函数依赖——cli 接线时注入单文件 check 面）。
- `internal/cli`：`we lsp` 从边界 70 行接真服务（cli.go 分发表 + lsp.go 接线——注入 loadFile 同源的单文件管线）；`takesPath` 语义定形（LSP 以 initialize 的 rootUri 为根惯例，[path] 参数为可选初值）。
- 新 `internal/cli/lsp_test.go`：真子进程会话测试（TestMain 一次性 `go build` 真二进制复用）。
- conformance 黄金零增量；既有 695 枚全绿回归即证（we lsp 不触任何编译器语言面）。

## 影响层

- spec：零（ch21 R12 早已封口——LSP 在规范外；零规范增量零新码零新 ADR）。
- compiler：cli（lsp.go 接线 + 分发行改）+ 新 internal/lsp 包；parser/typecheck/codegen/deps 零触碰。
- docs：docs/lsp.md + lsp.zh.md 新对（31→32）+ README 责任图双语言补行。
- process：roadmap 双语 M14 行（归档期）。

## 影响范围

- 涉触码：`internal/cli/cli.go`（分发表 lsp 行接真面）、新 `internal/cli/lsp.go`、新 `internal/lsp/` 包（wire.go + server.go + 单测）。
- 新测试件：internal/lsp 包单测（帧编解码 + JSON-RPC 消息形）+ cli lsp_test.go（真子进程会话——生命周期序 + 诊断一致性对拍）。
- 既有黄金零触碰（负向对账：无任何黄金钉 we lsp 边界形——cli.go 分发表无 lsp 专属黄金）。

## 审计记录

2026-09-09，welang-spec-impact-audit 7 点，**通过**：

1. 问题真实性 ✅——`we lsp` 是 11 子命令最后一个诚实边界（cli.go:55 分发表无 implemented 位）+ ch21 R12 separate document 指名留白未兑现（docs/ 无 lsp 文档）；规范引用 ch21 R12（2100-toolchain.md R12 + 场景「Editor diagnostics are check's diagnostics」）。
2. 影响层声明 ✅——change.yaml [compiler, docs] 与 proposal 影响层一致；「不改变语言行为——纯实现 + docs 文档，规范零增量」边界明示（internals-only 豁免，M0 先例）。
3. 规范增量范围 ✅——零 Requirement 增删、零诊断码（LSP 面零新码，design D4 钉死一致性承诺字面）。
4. 原则一致性 ✅——诊断单一权威 = check 管线（一类事实一个权威位置）；零第三方依赖延续；无原则突破。
5. 参考基线固定 ✅——工件只引规范本体（ch21），未引 refr 草稿。
6. 验收边界 ✅——三目标机械可判（docs/lsp.md+zh 存在且 docs_sync 32 对 / we lsp 真服务可会话 / 测试绿）；非目标九项排除防蔓延。
7. 粒度 ✅——单垂直线（文档 + wire + 会话 + 接线 + 测试），无跨不相关层。

## 审查记录

2026-09-09，welang-change-review 10 点，**通过**（无发现需处置）：

职责边界四条 ✅——proposal 黑盒问题+目标（What Changes 载结构属 M13 先例形）；spec 增量零（internals-only 豁免）；design 唯一路径 + 四裁决各记被拒替代与理由、引用精确到 ch21 R12 场景名；tasks 全项来源/验证、无 deferred/未决。
内容质量六条 ✅——场景覆盖三路（正常生命周期/边界 malformed+边界形不推送/失败降级 -32700、-32002、-32601）；无空章节；测试先行（T2 wire_test、T3 server_test 首动作即红）；负向断言真实（malformed 帧、未初始化请求、未知方法、边界形不推送均有构造样例）；完成度闭环第 9 条适用性判断——本变更是工具面非语言特性（诊断复用既有 check 管线，无新 typecheck/codegen 语义面），三要素以「管线复用方案」（design D2/D3）闭合；无未决问题阻塞。

## 实现审查记录

2026-09-09，welang-code-review 7 条（active → complete 关卡），**通过**：

1. **规范符合性** ✅——零 spec 增量（ch21 R12 纯实现）。R12 唯一承诺「Editor diagnostics are check's diagnostics」以 checkSrc 同函数落实（loadFile 共享核提取，check 与 LSP 同 stages 同判决）；对拍语料三源（E0501/W1912/边界 exit 70）九探针逐字段证实。LSP 零新码（推送码全部原样来自注册表）；映射按 docs/lsp.md D4 表逐字段（severity 1–4 / code 原样 / message 同串 / 1-based 点 → 0-based 单字符跨度 / source="we" 常量 / help 不映射）。
2. **验证诚实性** ✅——T1–T7 逐项核对：docs_sync 32 对、go test ./internal/lsp/ 绿（16 测试）、TestLsp 三测试绿、电池 9/9（/tmp/m14_battery.py 输出在案）、验证阶梯全绿（validate --all --strict、gofmt 空、vet、全测试、conformance 695）。无「勾了没跑」项。
3. **测试先行证据** ✅——T2 red：`undefined: Write/Read/Message/ErrParse`（wire.go 前）；T3 red：`undefined: CheckResult/NewServer`（server.go 前），均记于完成记录。conformance 黄金零增量（无新 case），黄金格式条款不适用。
4. **诊断协议稳定** ✅——diag.go JSON() 手拼面零触碰（diff 仅四只读访问器）；registry 零改动（validate registry clean）；--json 既有字段无删改。
5. **单一权威** ✅——LSP 长期事实落 docs/lsp.md（R12 指名的 separate document，正确权威位置）；变更目录无滞留长期事实。代码注释/docs 英文：本变更 diff 全英文（既有测试文件中文 = 测试数据合法：同形字混淆、字符串/注释面规范行为测试）。范围外既有瑕疵披露：internal/codegen/ffi_test.go:40 注释一处中英混排（M12 遗留，非本变更引入，不越界修，留待下次触碰该文件时顺手清理）。
6. **红线复核** ✅——refr/ 零触碰（git status 干净）；未提交（候用户明示）；diff 无越界：影响范围预判修正一处已披露——internal/diag/diag.go 四访问器（Severity/File/Line/Column，Message/Code 先例形，映射读位置用），不在 proposal 原清单内，属实现必需最小增量；check.go checkSrc 提取为 loadFile 重构（双面同源承诺的落实，行为等价——conformance 695 零回归证）。
7. **最小可信验证已跑** ✅——T7 阶梯与实际 diff 匹配（docs+code+tests 全量）。
