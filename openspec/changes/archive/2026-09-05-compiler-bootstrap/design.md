# design.md — compiler-bootstrap

白盒决策。规范依据：第 21 章 R1（命令面）、R7（JSON Lines 协议）、R8（清单骨架与「we new 写骨架」场景）；第 9 章 Sum declarations、第 14 章 Result 约束、第 15 章 main 形状（骨架内容依据）；ADR-0002（实现语言与引脚纪律）；`docs/spec/diagnostics.toml` 的 E1904/E1907 条目（标题与 remediation 的逐字来源）。

## D1 Go 模块与目录布局

module `github.com/ltlvtao/welang`（与 git remote 一致；未来自举/迁移时的重命名是机械操作）。布局：`cmd/we/main.go`（薄入口，只调 `internal/cli`）；`internal/cli`（解析与分发）；`internal/diag`（诊断与协议编码）；`internal/version`（版本清单）；`internal/conformance`（黄金用例跑器）。`internal/` 防外部依赖未定型 API——参考实现的公共面只有 `we` 二进制的 CLI 与其 JSON Lines 输出。

## D2 版本清单（ADR-0002 Dependencies 表的落地）

`internal/version/version.go` 唯一持有三个值：CompilerVersion = "0.1.0"（语义版本，第 22 章形状）；SpecVersion = "0.9.0"——v0.8 私草（refr/）之后第一个正典基线的标签，机制中立纪律下值只住在代码与 roadmap、永不入规范正文；LLVMPin = "21.1.8"。Go 引脚的唯一权威是 `go.mod` 的 `toolchain go1.26.6` 指令（ADR-0002 明文），`we version --verbose` 的 go 行读 `runtime.Version()`。LLVM 引脚本切片仅展示（无 LLVM 调用）；opt/llc/lld 的驱动在里程碑 M4 落地。

## D3 we version 的固定形状

人类格式（stdout）两行：`we 0.1.0`、`spec 0.9.0`；`--verbose` 追加 `go <runtime.Version()>` 与 `llvm 21.1.8` 两行。`--json`：单行 `{"type":"version","compiler":"0.1.0","spec":"0.9.0"}`，`--verbose` 时追加 `"go"` 与 `"llvm"` 字段（R7 允许新增字段）。version 事件不在 R7 固定的三事件集（diagnostic/test-result/test-summary）内——其字段是工具链机制层承诺，一旦发布遵循同一稳定性纪律（不删不改名、只增）。数值从 `internal/version` 读；格式串在 `internal/cli` 唯一持有。

## D4 未实现子命令的诚实边界

11 个子命令全部注册（R1 封闭集的可观察事实：`we install` 是 shell 之错，不是「未实现」）。本切片实现 version 与 new；其余 9 个（build/check/run/test/fmt/vet/doc/clean/lsp）分发到统一边界：stderr 一行 `we: <subcommand> is not implemented in this reference build yet`，退出码 70（sysexits EX_SOFTWARE）。非规范表面：无诊断码、无 JSON 事件（`--json` 下行为相同）；随切片逐个替换为真实现，终态归零。70 不占用、也不进入任何退出码规范承诺（第 21 章只固定 `we test` 的 0/1/2）。

## D5 E1904 与 E1907 的触发位

E1904（`we new <name>`）：名字校验 = 非空且字符集恰 `[a-z0-9-]`（第 22 章 R1 正文与 E1904 条目 remediation 的逐字拼写；前导/尾随连字符无规范依据、不发明——缺口登记 roadmap follow-ups）。校验先于任何文件系统写（R1 场景「creates nothing」）。目标目录已存在 → usage 错误（退出 2、stderr、无诊断事件；规范未固定，机制层选择）。
E1907（路径型子命令 build/check/run/test/fmt/vet/doc/clean/lsp）：`[path]` 参数在分发到子命令本体之前统一检查——stat 失败即 E1907（诊断事件 + 退出 1）；由此 R1 场景「we build nosuchdir」在边界子命令上也成立（路径检查先于未实现边界）。`version` 不取 path（多余参数 → usage 错误）；`new` 取名字而非路径。

## D6 退出码协议（本切片范围内）

0 成功；1 任何 E 级诊断（E1904/E1907）；2 usage 错误——未知子命令、未知选项、`--color` 非法值、`we` 无子命令、`new`/`version` 的多余参数、`new` 目标目录已存在：usage 类与「集合之外的子命令是 shell 的错误」同族（R1），故无诊断事件、无 JSON 输出、stderr 人类可读一行；70 未实现边界（D4）。

## D7 诊断构造与 JSON Lines 编码（R7 落地）

`internal/diag`：Diagnostic 携带 Type("diagnostic")、Severity、Code、Message、File、Line、Column、可选 Help。JSON 序列化字段序固定 `type/severity/code/message/file/line/column/help`，help 缺省时整个字段省略（R7「when one exists」）；手工序列化保字段序，快照测试逐字节锁定（含 help 与不含 help 两形态）。JSON Lines 写 stdout；人类可读诊断写 stderr，形状 `error[E1904]: invalid project name — <detail>`（与第 21 章示例 `error[E0102]:` 一致）；`--json` 时诊断只以事件形式写 stdout、不重复人类渲染。消息文本以注册表标题开头——把「一码一消息」纪律带进代码。E1904/E1907 的 title 与 remediation（作 help 字段）逐字取自 `diagnostics.toml` 条目、硬编码于调用点；注册表 `go:embed` 推迟（roadmap follow-up 2），理由：安装态二进制读哪个注册表文件需要随真实管线阶段一并设计。

## D8 we new 骨架内容（循已批准章节，保证未来 check 绿）

创建目录 `<name>/`：
- `we.toml`：`name` / `version = "0.1.0"` / `type = "executable"` 三键、无其他表（R8 场景「不写工具链未定义的表」）；文件注释镜像第 21 章示例块。
- `src/main.we`：

```we
pub type AppError = Failed(String)

pub fn main() -> Result<(), AppError> {
    return Ok(())
}
```

  依据：第 9 章允许单变体 sum；第 14 章 E 必须是命名 sum 类型；第 15 章根模块恰一个 `pub fn main() -> Result<(), E>`、`return Ok(())` 退出 0。
- `tests/main_test.we`：注释-only 空测试模块（R1 场景「one empty test module」；默认测试集是 tests/ 递归的 `*_test.we`，空模块零 test 块）。

## D9 黄金用例跑器

`internal/conformance`：用例 JSON `{name, args[], exit, stdout, stderr, files?{relpath: content}}`；跑器 in-process 调 `cli.Run(args, stdout, stderr)`（in-process 保确定性、免 exec 开销；`new` 类用例在 `t.TempDir()` 执行、`files` 记相对路径与内容）。`WE_UPDATE_GOLDEN=1` 再生黄金。输出中的临时目录路径以 `<dir>` 占位归一。本切片用例：version 四形状（人类 / --json / --verbose / --verbose --json）、new 骨架全文件树、`new My_Project`（E1904 人类 + --json 两形态）、`build nosuchdir`（E1907 人类 + --json）、`install`（未知子命令；--json 下仍无事件）、`check`（未实现边界 70，含 --json）、`--color strict`（非法值 2）、`new` 目录已存在（2）。协议快照测试独立于用例集（`internal/diag` 内逐字节断言）。

## D10 validate.py 豁免修正（时序：先于本变更 ready 落盘）

现状矛盾：`openspec/README.md` 允许纯内部变更免 specs/；validate.py 对 ready+ 机械要求至少一个 `specs/*/spec.md`。修正：提案文本含「不改变语言行为」或「无规范增量」标记 → 豁免 ready+ 的 specs/ 要求（design/tasks 仍必需）；同一标记集并入 strict 层检查的豁免标记（现状认「非目标」/「不改变语言行为」，增「无规范增量」）。`openspec/README.md` 状态表该行加注豁免条件。修正属本变更 process 层；因 validate.py 以现状会拒绝本变更的 ready 态，须先行落盘、随变更同库提交——时序在此记录、提交范围在 proposal 影响范围声明。豁免仅对带标记者生效：负向断言——无标记的假变更必须仍报 specs/ 缺失（防豁免过宽）。

## D11 .gitignore 增补

`/we`（根目录构建产物）、`*.test`（go test 二进制）。

## D12 验证策略（验证阶梯两行 + 工件行）

编译器/工具链代码行：`go build ./...` + `go test ./...` + 黄金用例（跑器即测试）；诊断协议行：`internal/diag` 快照测试；工件行：`python3 openspec/tools/validate.py --all --strict`、`python3 openspec/tools/docs_sync.py`（roadmap 双语对）、`git diff --check`、staged 无 `refr/`。测试先行：T3/T5 的目标测试先于 `internal/cli`/`internal/diag` 实现写入并确认失败，red 证据记于完成记录。

## 被拒绝的替代方案

- **切片 1 并入第 1 章词法分析器**（用户裁决 2026-09-04 拒绝）：CLI 脊架与一致性地基先行，词法独立成片，保持「小而可垂直验证」。
- **伪造一个最小规范增量以绕过 validate.py 现状**：违反「变更工件只是过程增量」与单一权威纪律——无行为变更却产生规范文本，是污染；修正执法脚本才是对的那一刀。
- **未实现子命令完全不注册（表现为未知子命令）**：混淆「规范封闭集内的子命令」与「集外子命令」两个可观察事实，且每个后续切片都要改子命令表；诚实边界一行更便宜。
- **module 取裸名 `welang`**：无远程语义；与 git remote 一致的路径免去未来重命名，成本为零。
