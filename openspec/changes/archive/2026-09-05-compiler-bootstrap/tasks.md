# tasks.md — compiler-bootstrap

- [x] T1 validate.py 内部变更豁免修正 + openspec/README 状态表注记
  来源：proposal 目标 6 / design D10
  验证：`python3 openspec/tools/validate.py compiler-bootstrap --strict` 在 ready 态无 specs/ 通过；负向——临时目录构造无标记的假 ready 变更，validate 仍报 "requires at least one specs"，随后删除临时目录
- [x] T2 go.mod + .gitignore + internal/version（含测试）
  来源：proposal 目标 1 / design D1 D2 D11
  验证：`go build ./...` 通过；`go test ./internal/version/` 通过（三个常量非空、CompilerVersion 合语义版本形状）
- [x] T3 测试先行：internal/diag 协议快照测试（先红后绿，red 证据记于完成记录）
  来源：proposal 目标 3 / design D7 D12
  验证：`go test ./internal/diag/` 通过；快照逐字节断言含 help 与不含 help 两形态
- [x] T4 internal/diag 实现（Diagnostic + JSON Lines 编码 + 人类可读渲染）
  来源：design D7
  验证：`go test ./internal/diag/` 通过
- [x] T5 测试先行：internal/conformance 黄金用例集（先红，red 证据记于完成记录）
  来源：proposal 目标 2/4 / design D9
  验证：`go test ./internal/conformance/` 通过（用例集覆盖 D9 列举的全部八类场景）
- [x] T6 internal/cli 实现（子命令表、全局选项、version/new、E1904/E1907、usage 与 70 边界、退出码）
  来源：proposal 目标 2 / design D3–D8
  验证：`go test ./internal/conformance/` 全用例绿；`go vet ./...` 通过
- [x] T7 cmd/we 入口装配
  来源：design D1
  验证：`go build ./...` 通过；`go run ./cmd/we version` 输出两行固定形状
- [x] T8 docs/roadmap/0000-reference-implementation.md 双语
  来源：proposal 目标 5 / 薄垂直优先裁决（2026-09-04）
  验证：`python3 openspec/tools/docs_sync.py` 通过；里程碑表含 M0–M15 与 follow-ups 三条
- [x] T9 全量验证组合（验证阶梯）
  来源：design D12
  验证：`go build ./... && go test ./... && go vet ./... && python3 openspec/tools/validate.py --all --strict && python3 openspec/tools/docs_sync.py && git diff --check` 全通过
- [x] T10 七点实现审查 + 归档提升 + 记忆更新 + 提交
  来源：变更工作流 active → complete → archived
  验证：proposal 审查记录七点结论；`git log -1 --format='%(trailers)'` 无署名；`git show --stat HEAD` 无 refr/ 路径

## 完成记录

- 2026-09-05 T1：豁免正负双探针通过（负向：无标记假 ready 变更双 FAIL——layers 豁免理由 + specs 缺失；正向：带「不改变语言行为」标记通过）；同时修复潜伏缺陷——旧标记「非目标」是必填标题子串、strict 检查恒真，标记集收窄为「不改变语言行为」/「无规范增量」。
- 2026-09-05 T2 绿：`go test ./internal/version/ ok`。
- 2026-09-05 T3 red 证据：实现前 `go test ./internal/diag/` 失败——`undefined: Diagnostic`、`undefined: Error`（build failed，exit 1）；T4 后转绿。
- 2026-09-05 T5 red 证据：实现前 `go test ./internal/conformance/` 失败——`no required module provides package github.com/ltlvtao/welang/internal/cli`（setup failed，exit 1）；T6 后 18/20 绿，余 2 例为黄金键漏 `demo/` 前缀的作者笔误（跑器抓出、修黄金不改码），随后 20/20 绿。
- 2026-09-05 T6 附：`gofmt -w internal/cli/cli.go` 一次；`go vet ./...` 通过。
- 2026-09-05 T7 附：`go run` 在模块目录外不可用（go.mod 定位限制），冒烟改用 `go build -o` 产物执行——E1904/E1907/未知子命令/70 边界/骨架三件全部符合预期；tasks 内验证命令均在仓库根执行，不受影响。
- 2026-09-05 T8：docs_sync 29→30 对。
- 2026-09-05 T9 全绿；`git status` 恰为影响范围 8 路径。
- 2026-09-05 T10：七点实现审查通过（记录见 proposal 审查记录）；状态 → complete → archived，移入 archive/2026-09-05-compiler-bootstrap/；`validate --all --strict` 归档态复跑绿；提交验证于提交后复核。
