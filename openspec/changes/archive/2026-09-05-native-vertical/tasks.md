# tasks.md — native-vertical

- [x] T1 测试先行 A：D12 黄金表 22 枚落盘（build/run/clean 三面、Err 端到端、边界行代表、单文件两序、library、依赖非空），对当前构建运行必须先红
  来源：proposal 目标 1 2 3 7 9 / design D1 D2 D12
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红（22 枚新用例失败、既有全绿），red 证据记于完成记录
- [x] T2 测试先行 B：internal/codegen 单测（两形 IR 逐字节快照、逃逸表、What 表四行、发射确定性）与 cli 侧单测（版本门拒绝、clean 幂等/verbose、双构建字节一致）先写先红
  来源：proposal 目标 4 6 8 / design D4 D7 D10 D12
  验证：`go test ./internal/codegen/`（包未实现红）与 cli 确定性/门用例红，red 证据记于完成记录
- [x] T3 runtime/ 落地：startup.c 与 alloc.c 按 D6 逐字、runtime/runtime.go（go:embed，package weruntime）
  来源：proposal 目标 5 / design D6
  验证：`go build ./...` 通过；`clang -c` 两源零警告零旗标编译通过（真机）
- [x] T4 internal/codegen 实现：模块级接受集（和式擦除 + main）、两形 main 体识别（变体归属回查）、文本 IR 发射（符号 ABI、双向逃逸、零旗标零 triple）、What 表四行
  来源：proposal 目标 4 / design D3 D4 D5
  验证：`go test ./internal/codegen/` 全绿
- [x] T5 internal/cli 接线：loadProject/loadFile 抽取（行为零改写）、build.go（runBuild/runRun/runClean + 钉版门 + clang driver 序 + 依赖非空边界）、cli.go 子命令表 build/run/clean 三行翻 implemented
  来源：proposal 目标 1 2 3 6 7 / design D1 D2 D6 D7
  验证：`go test ./internal/conformance/` 全绿（22 新 + 既有零改写全绿）；`go test ./...` 全绿
- [x] T6 确定性与回归复验：双构建字节一致测试绿；既有 101+2 黄金零改写全绿复跑
  来源：proposal 目标 8 9 / design D10 D12
  验证：`go test ./...` 全绿；输出记于完成记录
- [x] T7 验证阶梯：gofmt -l（空）、go build ./…、go vet ./…、validate --strict、docs_sync、git diff --check
  来源：研发流程验证阶梯
  验证：全部命令通过，输出记于完成记录
- [x] T8 实现审查（welang-code-review 7 条，真二进制证据：真机 `we new`→`we check`→`we build`→`we run` 两路径 + clean 幂等 + 边界行代表）结论写回 proposal；通过后 status → complete 并归档（归档目录 2026-09-05-native-vertical）
  来源：研发流程
  验证：openspec/changes/archive/ 内本变更目录存在且 validate --all --strict 通过
- [x] T9 roadmap：M4 行翻写 done（双语对同步）+ Registered follow-ups 登记 #6（单文件构建工件命名——ch21 R2 工件由 manifest 命名 vs 单文件无 manifest）（双语对同步）
  来源：roadmap 执行状态规则 + 裁决 Q3 的登记义务
  验证：`python3 openspec/tools/docs_sync.py` 通过；表格行含 done 与 #6 新编号项
- [ ] T10 记忆更新与提交：project-overview 增 M4 条目、MEMORY.md 索引行更新；英文提交信息（无署名 trailer；refr/ 不入提交）
  来源：研发流程 + 用户红线
  验证：`git log -1` 消息为英文且无 Co-Authored-By；`git show --stat` 无 refr/ 路径

## 完成记录

**T1（2026-09-05）**：22 枚黄金落盘后对当时构建（build/run/clean = M0 边界、check 无依赖面）跑 `go test ./internal/conformance/ -run TestGoldenCases`，红 22 枚——build-bnd-body build-bnd-err-payload build-bnd-fn build-bnd-toplet build-json build-library build-manifest-missing build-single-file build-single-file-diag build-skeleton build-verbose check-deps-nonempty clean-artifacts clean-file-usage clean-idempotent clean-verbose run-err run-err-verbose run-single-file run-skeleton run-skeleton-json run-skeleton-verbose——既有 101 枚全绿（零改写为证）。

**T2（2026-09-05）**：`go test ./internal/codegen/` 编译红 ×5（`undefined: Emit`）；cli 侧 `go vet` 报 `undefined: checkClangVersion`（×3 处引用）——API 契约先钉，红证据即此。

**T3（2026-09-05）**：`runtime/c/startup.c`、`runtime/c/alloc.c`、`runtime/runtime.go` 落盘。机制修正（已同步 design D6 与 proposal 影响范围）：Go 工具链拒绝无 cgo 包目录中的 `.c` 文件，C 源移入无 Go 文件的 `runtime/c/` 子目录。验证：`go build ./...` 过；钉版 clang `clang -c` 两源零旗标零警告（真机）。

**T4（2026-09-05）**：`internal/codegen/codegen.go`（Emit + NotImplemented + 四行 What 表 + 双向逃逸）。实现期按 M3 协议（红 → 真机验证 → 修正测试 → 披露）处理了两类测试侧错误：

- **逃逸语法**：T2 手写的 IR 常量用了 `\xe4\xb8\xad` 与 `\"` 双写形——都不是钉版 LLVM 文法。真机探针：c"..." 内 `\"` 使字符串提前终结（报 expected top-level entity）；`\22`/`\5C`/`\E4`/`\00`/`\09` 十六进制形经 clang 编译、链接、`__we_fail` 运行时逐字节回放全链验证（`error: Failed: tab	here"\` + LF 逐字节正确）。规则钉死为：可打印 ASCII（除 `"` `\`）原样，其余一律 `\XX` 大写十六进制；design D4 双向解码段已同步勘误。
- **长度手算**：errIR 快照 19 → 20（「error: Failed: boom\n」= 15 前缀 + 4 载荷 + 1 换行）；逃逸表四行重算（载荷自身的尾部 `\n` 与格式行 `\n` 叠加）：tab 25→26、unicode 4→20、backslash 19→20、nul 19 不变。

修正后 `go test ./internal/codegen/` 全绿（快照 + 逃逸表 + What 表九触发 + 多载荷 + 确定性）。

**T5（2026-09-05）**：`internal/cli/check.go` 抽出 loadProject（含依赖非空边界与 library 边界，序按 D2：依赖先）与 loadFile；`internal/cli/build.go`（runBuild/runRun/runClean、checkClangVersion 钉版门、buildProject 的 clang driver 序、runClang 失败报告）；cli.go 三行翻 implemented。实现期发现（已同步 design D6）：无 triple 的 .ll 让 clang 打 `overriding the module target triple` stderr 警告，而成功三面钉死静默——链接调用加 `-Wno-override-module` 消警（真机验证：消警后编译链接全静默）。另两处测试侧修正：build_test.go 的 `_, err :=` 双值形改单值（checkClangVersion 只返回 error）；clean 的 verbose 末行 `we: removed build/`（尾斜杠——filepath.Rel 会清掉，实现显式补回）。黑盒复验修出一处契约偏差：verbose + build/ 缺契时实现多打了末行——D1 钉死「缺席时不打印任何行」，已修并补 `verbose silent when absent` 子测试锁定。验证：`go test ./internal/conformance/` 全绿（123/123：22 新 + 101 既有零改写）。

**T6（2026-09-05）**：`go test ./...` 全绿——cli（版本门六例、clean 三面、双构建字节一致 TestBuildDeterministic 真机 clang 通过）、codegen、conformance 123/123、lex/parser/typecheck/diag/version 全部 cached 绿。真机黑盒端到端复验（/tmp/demo）：`we new`→`we check .`（静默 0）→`we build .`（静默 0，build/ 内 demo/demo.ll/rt-startup.c/rt-alloc.c/两 .o）→`we run .`（Ok：exit 0 静默；--verbose：`we: built build/demo`）；Err 路径 `error: Failed: boom` stderr、exit 1（直跑 `./build/demo` 同输出同码——工件自含）；clean 三面（静默幂等 / verbose 列行 + `we: removed build/` / 缺席 verbose 零行）；`clean src/main.we` usage 2。

**T7（2026-09-05）**：验证阶梯全过——`gofmt -l .` 空；`go build ./...` 过；`go vet ./...` 过；`python3 openspec/tools/validate.py native-vertical --strict` 过；`python3 openspec/tools/docs_sync.py` 过；`git diff --check` 干净。输出见本记录（命令逐条当场执行）。

**T8（2026-09-05）**：welang-code-review 七条全过，结论与真二进制证据（端到端两路径、clean 三面、六类边界行、共享管线 E1905、E1907 分发前）记 proposal「## 实现审查记录」；一处发现（build_test.go 未列入影响范围）当场修正。status → complete 后归档至 2026-09-05-native-vertical。零规范增量与零 ADR 提升均按 M0–M3 先例核对（Err 行格式与 IR 符号 ABI 为 ch15 R6 机制自由度，钉于 design D4/D5，stdlib 里程碑经自身变更定形）。

**T9（2026-09-05）**：roadmap 双语对同步——M4 行翻 `done 2026-09-05`；Registered follow-ups 登记 #6（单文件构建工件命名，M4 裁决 Q3 的登记义务）；docs_sync 30 对齐通过。
