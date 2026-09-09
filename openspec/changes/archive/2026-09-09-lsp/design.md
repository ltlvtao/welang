# design — lsp（M14）

## 表面裁决（AskUserQuestion，2026-09-09，四项均采推荐）

| # | 裁决点 | 裁决 | 理由 |
|---|--------|------|------|
| Q1 | LSP 文档落点 | docs/ 顶层双语 `docs/lsp.md` + `lsp.zh.md` | ch21 R12 字面「lives in a separate document of its own … outside the specification」——非规范章；docs/README 责任图补行；docs_sync 31→32 |
| Q2 | `we lsp` 实现深度 | 诊断核心面：生命周期 + 文档同步（full）+ publishDiagnostics | ch21 唯一规范承诺的完整兑现即此面；hover/definition/completion 等编辑能力后续增量 |
| Q3 | 协议载体 | 自实现最小 wire 层（纯标准库） | go.mod 零外部依赖纪律（M0 起）延续；JSON-RPC 2.0 + Content-Length 帧面窄量小（数百行） |
| Q4 | 测试面 | Go 测试真子进程会话（conformance 黄金零增量） | runner 是单发命令模型，LSP 是交互 stdio 会话——形状不合；语言面黄金与本变更正交（695 全绿回归即证） |

## D1 — 文档形态（docs/lsp.md，英文权威 + zh 对照）

结构（七节）：定位（ch21 R12 的 separate document 载体；唯一规范承诺复述——编辑器诊断 = `we check` 的）→ 协议基线（LSP 3.x base protocol、JSON-RPC 2.0）→ 传输（stdio、Content-Length 头帧）→ 生命周期方法集（initialize/initialized/shutdown/exit + 能力协商面）→ 文档同步（didOpen/didChange full/didClose + 推送时机）→ **诊断映射规则**（D4 的逐字段表，文档的主体）→ 能力面清单与边界（本里程碑所启/所不启，如实列出）。

文档性质：工具层绑定文档——不进 docs/spec/、无 E 码、无 Scenario 体；zh 对照遵循双语同步支持惯例（代码块字节一致、诊断消息串保留英文原文）。

## D2 — 诊断管线面 = 单文件面（一致性承诺的字面兑现）

每次 didOpen/didChange 以**文档全文**跑单文件 check 管线（parse → typecheck SingleFile）——与 `we check file.we` 同一函数、同一判决：std.* 内建解析、其余 import E1302、E1907/E 路径序、advisory 层同跑（单文件面无 manifest 恒 warning 缺省——M11 Q4 面）。CLI 与编辑器对同一文本拿到同一 `[]diag.Diagnostic`——零分歧由同函数保证，测试逐字段对拍钉死。

已知限制（文档如实披露）：项目面感知（模块图 + 依赖解析下的 didOpen）不在本里程碑——单文件面下项目内 import 报 E1302 是该面的既有判决形（CLI `we check src/main.we` 同形）。

## D3 — 包结构：internal/lsp（wire + server）+ cli/lsp.go 接线注入

- `internal/lsp/wire.go`：Content-Length 头帧读写（bufio + strconv）+ JSON-RPC 2.0 消息形（request/response/notification 的编解码，encoding/json）。
- `internal/lsp/server.go`：服务会话机——单读循环分发：initialize（capabilities 协商响应）→ initialized 后 didOpen/didChange/didClose 处理 + publishDiagnostics 推送 → shutdown 响应 → exit 退出；写侧串行化（写锁）。**依赖注入**：Server 以 `Check func(path, src string) []diag.Diagnostic` 构造参数拿管线——lsp 包不 import cli（依赖单向：cli → lsp）。
- `internal/cli/lsp.go`：接线——注入 `loadFile` 同源的单文件管线函数；`cli.go` 分发表 lsp 行加 `implemented: true` + `--json`/全局选项面（LSP 面恒走 wire，`--json` 不改 LSP 行为——协议自身即 JSON）。
- rootUri 语义：initialize 参数 `rootUri` 按惯例接收（本里程碑单文件面不消费根路径）；`[path]` 可选参数同收——两者皆不改变诊断面（单文件面由文档 URI 定 path）。

## D4 — 诊断映射规则（文档钉死的逐字段表）

| We 诊断（JSON Lines 面） | LSP Diagnostic | 规则 |
|---|---|---|
| severity: error/warning/note/help | severity: 1/2/3/4 | Error/Warning/Information/Hint 四级一一对应 |
| code: "E0501" | code: "E0501" | 注册表码字符串原样；LSP 面零新码（一致性承诺字面） |
| message | message | title — detail 原文（JSON Lines 的 message 字段同串） |
| file/line/column（1-based） | range | start = {line−1, column−1}、end = {line−1, column}（0-based，单字符宽下划线）；file 与文档 URI 对应（单文件面诊断恒属当前文档） |
| help | （不映射） | LSP Diagnostic 无对应槽；remediation 属 CLI 面 |
| — | source: "we" | 常量 |

边界形（not-implemented 70）的处理：单文件管线返回边界 What 时**无诊断事件**（CLI 面也是 stderr 平文非 diag）——LSP 面不推送该文档（保持上次诊断不更新）；文档如实披露此形。

## D5 — wire 层细节

- 帧：`Content-Length: N\r\n\r\n` + N 字节 JSON body（LSP base protocol）；读侧 bufio.Reader 先扫头行再定长读体；写侧构造头 + 体一次写。
- JSON-RPC 2.0：`jsonrpc: "2.0"`、request（id + method + params）、response（id + result/error）、notification（method + params 无 id）；服务端对未知方法回 `-32601 MethodNotFound`（LSP 惯例 $/ 前缀通知静默忽略）。
- initialize 前收到非 initialize/exit 消息 → error `-32002 ServerNotInitialized`（LSP 惯例）；exit 未 shutdown → 进程退出非零（LSP 基线）。

## D6 — 测试策略

- `internal/lsp` 单测：帧编解码往返（in-process bytes 对拍）、JSON-RPC 三消息形、server 会话机（管道两端——io.Pipe 驱动完整会话序）。
- `internal/cli/lsp_test.go` 真子进程：TestMain 一次性 `go build` 真二进制到共享临时目录（sync.Once），各测试起 `we lsp` 子进程写请求帧读响应/推送帧——钉死：initialize→initialized→didOpen→诊断推送→didChange→更新推送→didClose→清空推送→shutdown→exit 全序；**诊断一致性对拍**：同一源文件，`we check --json`（子进程同二进制）的事件流与 LSP 推送按 D4 映射逐字段相等。
- 黑盒电池（shell 探针，T6）：真二进制 + python 客户端模拟（独立于 Go 测试的第三方视角）——干净文件零推送、E 码负例推送、W1912 advisory 推送、malformed JSON 帧 -32700、exit 码面。

## D7 — 边界与已知限制（总披露位）

1. 单文件面无项目感知（D2 已述）。
2. advisory 推送带（单文件面恒 warning 缺省 posture——无 manifest 可提升）。
3. 边界形文档不更新诊断（D4 已述）。
4. LSP `--json` 全局选项在 lsp 面无行为（协议自身即 JSON——cli.go 分发不误接）。
5. 多根（workspaceFolders）、incremental 同步、socket 传输：不启（非目标）。
