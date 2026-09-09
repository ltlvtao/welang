# tasks — lsp（M14）

- [x] T1 LSP 绑定文档双语落盘：`docs/lsp.md`（英文权威）+ `docs/lsp.zh.md`（中文对照）——七节结构（design D1：定位/协议基线/传输/生命周期/文档同步/诊断映射逐字段表 D4/能力面清单与边界；代码块字节一致、诊断消息串保留英文）+ `docs/README.md`/`README.zh.md` 责任图补行
  来源：裁决 Q1 / design D1 D4
  验证：`python3 openspec/tools/docs_sync.py` 31→32 对齐；双语结构检查过
  完成（2026-09-09）：七节结构落盘（协议基线/传输/生命周期/文档同步/诊断映射表/能力面边界 + 开篇定位复述 R12 承诺）；README 双语责任图插行；docs_sync 31→32 过

- [x] T2 测试先行 wire：`internal/lsp/wire_test.go` 先红（帧往返 bytes 级对拍、JSON-RPC 2.0 三消息形编解码、malformed 面），再 `wire.go` 实现（Content-Length 头帧读写 + 三消息形，纯标准库）
  来源：裁决 Q3 / design D5
  验证：`go test ./internal/lsp/ -run TestWire` 红 → 实现后绿；`go vet` 过；red 证据记于完成记录
  完成（2026-09-09）：7 测试（往返/帧字节级精确/通知省略 id/ErrParse/坏头非 parse 错/错误响应形/字符串 id 直通）。red 证据：实现前 `undefined: Write/Read/Message/ErrParse`。披露二：① TestReadMalformedJSON 初版头写 `Content-Length: 5` 而体 4 字节——差一字落 unexpected EOF（transport 错）非 ErrParse，测试笔误修正为 4（D13 记档）；② Message.Result 初为 `any`——json.Unmarshal 到 any 得 map[string]any 破坏逐字断言，实现前推演发现改 json.RawMessage

- [x] T3 测试先行会话机：`internal/lsp/server_test.go` 先红（io.Pipe 驱动完整会话序：initialize→initialized→didOpen→didChange→didClose→shutdown→exit，含 -32002/-32601 面），再 `server.go` 实现（读循环分发 + publishDiagnostics 推送 + 写侧串行化 + `Check` 函数注入——lsp 不 import cli）
  来源：裁决 Q2 / design D3 D5
  验证：`go test ./internal/lsp/` 绿；red 证据记于完成记录
  完成（2026-09-09）：9 会话测试（全生命周期含 checks==2 与 exit 码 0/映射逐字段/-32002/-32601/$-静默/-32700 会话存活/边界静默/裸 exit 非零）。red 证据：实现前 `undefined: CheckResult/NewServer`。server.go：单读循环同步分发（写天然串行），shutdown 旗标 + exit 码面（shutdown 先至 0，transport 断同规），didClose 推空数组后删文档，file:// URI → path。披露三：① 影响范围预判修正——`internal/diag/diag.go` 增四只读访问器 Severity()/File()/Line()/Column()（Message/Code 先例形，映射读位置用），不在 proposal 清单内，T8 审查披露；② harness 双管道接线初版交叉（服务端在 server→client 管道自读自写）——vet 报 unused 顺藤摸出，重接后过；③ done channel 测试体与 cleanup 双收死锁（buffered 1 只发一次）——改 exitCode() reap 一次模式。全绿 + vet + gofmt 干净

- [x] T4 cli 接线：`internal/cli/lsp.go`（注入 loadFile 同源的单文件 check 管线）+ `cli.go` 分发表 lsp 行 `implemented: true`（`--json`/全局选项面不误接）；`we lsp` 真 stdio 服务跑通、exit 码面正确
  来源：裁决 Q2 Q3 / design D3 D4
  验证：`go build ./...` 过 + 手工会话冒烟（initialize/exit 帧）
  完成（2026-09-09）：internal/cli/lsp.go runLsp 注入；loadFile 共享核提取为 checkSrc（盘读之外的 parse+typecheck 阶段，双面同函数同判决——D3 同源注入的落实形）；cli.go lsp 行 implemented: true + 分发表 case。--json 等全局选项照常解析但服务端不用（stdout 是协议流）。全测试 + conformance 零回归过；真二进制冒烟：initialize 能力/脏 didOpen E0501 推送/净 didChange 空推送/shutdown null/exit 0 全对

- [x] T5 真子进程会话测试：`internal/cli/lsp_test.go`——TestMain 一次性 `go build` 真二进制复用；全生命周期序钉死；诊断一致性对拍（同一源文件 `we check --json` 事件流 vs LSP 推送按 D4 映射逐字段相等，含 W advisory 面与边界形不推送面）
  来源：裁决 Q4 / design D6
  验证：`go test ./internal/cli/ -run TestLsp` 绿
  完成（2026-09-09）：internal/cli/lsp_test.go——TestMain 一次性 `go build` 真二进制（包级 lspBin，全部 cli 测试复用；in-process Run 无法承载协议占用 stdio 的会话，注入流双胎测的不是编辑器跑的东西）。三测试：TestLspLifecycle（真进程全序：握手能力/脏开推送/净变空推/close 空推/shutdown null/exit 0）、TestLspExitWithoutShutdown（裸 exit 非零）、TestLspMatchesCheck（对拍语料三源：E0501 型、W1912 型、边界型 `let (a, b) = (1, 2)`——check exit 70 零事件 ↔ LSP 零推送；E/W 逐字段：code/message/severity 映射/1-based 点→0-based 单字符跨度/source="we"；help 无 LSP 槽不比对）。披露：recv 带 30s 期限（不应答必须挂测试不得挂死测试套）

- [x] T6 黑盒电池：真二进制 + python 客户端模拟六组探针（干净文件零推送 / E 码负例推送 / W1912 advisory 推送 / malformed 帧 -32700 / exit 码面 / CLI-LSP 判决对拍）
  来源：design D6
  验证：探针全过，输出留档
  完成（2026-09-09）：/tmp/m14_battery.py 真二进制（/tmp/we-m14）python 客户端九探针 **9/9 全过**：① 干净文件空数组推送（版本信封）② E0501 推送（severity 1、source we、help 不入 LSP 槽）③ W1912 advisory 推送（severity 2）③b shutdown+exit 退出码 0 ④ malformed 帧 -32700 id null 会话存活 ⑤ exit 码面双腿（shutdown 先至 0 / 裸 exit 非零）⑥ 判决对拍三源逐字段（e/w 逐字段相等；bd 边界 exit 70 零事件零推送）

- [x] T7 验证阶梯：`validate.py --all --strict` OK + registry clean；`docs_sync` 32 对；`gofmt -l` 空；`go vet ./...`；全测试；conformance 695 全绿零回归
  来源：仓库验证惯例（M13 T8 形）
  验证：全绿，输出留档
  完成（2026-09-09）：`validate.py --all --strict` OK + registry clean；`docs_sync.py` 32 对齐；`gofmt -l`（除 refr/）空；`go vet ./...` 过；`go test ./...` 全包绿；conformance **695 全过零回归**（黄金零增量：无任何 case 调 `we lsp`，usage 行文本与实现无关）

- [x] T8 code-review：welang-code-review 7 条（规范符合性/验证诚实性/测试先行证据/诊断协议稳定/单一权威/红线复核/最小可信验证已跑）；审查记录入 proposal.md
  来源：仓库节奏
  验证：7 条通过，记录在案
  完成（2026-09-09）：7 条全过，记录入 proposal.md「## 实现审查记录」；两处披露——diag 四访问器影响范围预判修正（合法最小增量）、ffi_test.go:40 既有注释混排（范围外不修）；status → complete

- [x] T9 归档：roadmap 双语 M14 行 pending → done；change.yaml status → archived（目录名日期前缀）；记忆回写；提交草案呈报候用户明示
  来源：仓库节奏（M13 T10/T11 形）
  验证：validate 复验过；git status 对账
  完成（2026-09-09）：规范提升零（零 spec 增量——LSP 事实本就落 docs/lsp.md）；决策提升零（design 取舍面随归档件留存，无 ADR 级长期决策）；roadmap 双语 M14 → done；目录 mv 2026-09-09-lsp + status → archived；记忆回写 project-overview/MEMORY.md（含 M13 条提交号 394ff98 修正）
