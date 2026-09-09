# We 语言服务器绑定文档

本文是第 21 章（工具链，"What the toolchain does not fix" 需求）指名的 LSP 绑定文档：语言服务器协议住在这里、住在语言规范之外，而规范的那一条承诺作为本文第一条规则复述——编辑器服务的诊断就是 `we check` 的。同一管线、同一码、同一判决，无论哪个面呈现。语言服务器不引入自己的任何诊断码；它推送的每一条诊断都是检查管线自身的，按下表逐字段映射。

## 协议基线

服务端说 LSP base protocol 的 3.17 修订版所定形的那一面：LSP 信封内的 JSON-RPC 2.0 消息。两种消息形状过线——请求（带 `id`，等待响应）与通知（无 `id`、无响应）——服务端既对它受理的请求发响应，也自发自己的通知（诊断推送）。

## 传输

服务端经 stdio 通信，每个编辑器会话一个进程，每条消息以 base protocol 头成帧：

```
Content-Length: <byte count>\r\n
\r\n
<JSON-RPC message, exactly <byte count> bytes>
```

头携带 UTF-8 JSON 主体的字节长度；读侧扫描头行直至空行，再精确读取该字节数。socket 传输不属于本绑定。

## 生命周期

会话跑一条固定序列：

1. 客户端发 `initialize` 请求。服务端以自身能力应答——文本同步 kind `1`（full：每次变更携带整份文档文本）——并自报身份 `we`。
2. 客户端发 `initialized` 通知。在它到来之前（以及 `initialize` 之前），服务端不受理其他任何消息：先于 `initialize` 到达的请求以错误 `-32002`（server not initialized）作答，唯 `exit` 例外——它永远落地。
3. 文档开、变、关（下节）；诊断随每次变更流动。
4. 客户端发 `shutdown` 请求；服务端以 null 结果作答并不再受理后续请求。
5. 客户端发 `exit` 通知；进程退出——`shutdown` 先至则退出码 0，否则非零。

错误面，皆循 base protocol：帧内 JSON 畸形以 `-32700`（parse error）作答；未知且非 `$/` 前缀方法的请求以 `-32601`（method not found）作答；`$/` 前缀通知静默忽略。

## 文档同步

文档归客户端所有；服务端经三条通知见到它们，采用 full 同步（每次变更携带整份文本——无增量位置编辑）：

- `textDocument/didOpen`——`{textDocument: {uri, languageId, version, text}}`。服务端以该文本为文档当前状态。
- `textDocument/didChange`——`{textDocument: {uri, version}, contentChanges: [{text}]}`。单元素的 `text` 即整份新文档。
- `textDocument/didClose`——`{textDocument: {uri}}`。

每次 `didOpen` 与每次 `didChange` 之后，服务端把文档文本送入单文件检查管线——正是 `we check <file.we>` 跑的那条——并把结果作为该 URI 的 `textDocument/publishDiagnostics` 通知推送：文本检查干净则空数组，否则推送映射后的诊断。`didClose` 之后，服务端为该 URI 推送最后一条空数组并忘掉该文档。

该管线选择的两个已披露面：

- **单文件作用域。** 每个 open 文档作为自己的单文件编译受检，恰如 `we check` 检查一个指名文件：`std.` import 自内建模块解析，其余 import 是管线自己的 `E1302` 判决。语言服务器不（尚）在 open 文档背后走项目图或解析依赖——同一文本从命令行与编辑器拿到同一判决，这正是承诺；项目感知诊断是未来绑定工作。
- **边界静默。** 当检查管线停在实现边界（本参考实现尚未实现的形式——命令行打边界行、退出 70），没有可报的诊断事件。服务端对该修订不推送；文档保持其上一次诊断。边界是实现暂态，不是诊断。

advisory 发现（`W` 码，缺省 warning 严重度）乘同一推送——单文件管线在 warning 缺省下跑它们，因为没有可提升它们的 manifest。

## 诊断映射

检查管线 JSON 面的每条诊断映射为一条 LSP `Diagnostic`：

| We 诊断（JSON Lines 面） | LSP `Diagnostic` | 规则 |
| --- | --- | --- |
| `severity`：`error` / `warning` / `note` / `help` | `severity`：`1` / `2` / `3` / `4` | Error、Warning、Information、Hint——四级一一对应 |
| `code`：`"E0501"` | `code`：`"E0501"` | 注册表码，原样；服务端永不铸码 |
| `message` | `message` | 同一字符串——注册表标题与细节 |
| `file`、`line`、`column`（1-based） | `range` | `start` = `{line-1, column-1}`、`end` = `{line-1, column}`——LSP 位置 0-based，1-based 点因此成为同行上的单字符跨度 |
| `help` | — | 无 LSP 槽位；remediation 留在命令行面 |
| — | `source`：`"we"` | 常量 |

通知的信封携带文档 `uri` 与受检文本的 `version`，编辑器因此绝不会把旧修订的诊断画在新文本上。

## 能力面与边界

本绑定所启：生命周期集（`initialize`、`initialized`、`shutdown`、`exit`）、full 文本同步（`didOpen`、`didChange`、`didClose`）、诊断推送（`publishDiagnostics`）。此外无它：hover、definition、completion、formatting、semantic tokens、code actions 及一切其他编辑器能力都不提供——`initialize` 响应的能力对象如实说如此，客户端不请求服务端未声明之物。增量同步、socket 传输、多根工作区同样在本绑定之外。每一项增补都以对本文档的变更到来——公开地、连同它的映射规则一起。
