# welang 变更工作流（openspec/）

本目录是 welang 的变更管理区。流程的**唯一权威**是 [docs/process/development-process.md](../docs/process/development-process.md)（英文）；本文件是其操作侧的中文速查，两者冲突时以权威文档为准。

## 目录布局

```
openspec/
  README.md                 # 本文件
  tools/validate.py         # 工件校验脚本（strict 模式）
  changes/
    <kebab-name>/           # 一个变更一个目录
      change.yaml           # name / status / layers
      proposal.md           # Why / 目标与非目标 / What Changes / 影响层 / 影响范围
      specs/<capability>/spec.md   # 规范增量（黑盒 Requirement + Scenario）
      design.md             # 白盒决策、被拒方案、验证策略
      tasks.md              # 可验证任务清单
    archive/                # 已归档变更（状态推进到 archived 后移入）
```

## 状态机与关卡

```
candidate →（welang-spec-impact-audit）→ ready →（四件套 + welang-change-review）→ active
    →（实现 + 验证 + welang-code-review）→ complete →（welang-archive-sync）→ archived
```

| 状态 | 工件要求（validate.py 强制） |
| --- | --- |
| `candidate` | `change.yaml` + `proposal.md`（五个必需标题齐全） |
| `ready` / `active` / `complete` | 四件套齐全：`proposal.md`、`design.md`、`tasks.md`、至少一个 `specs/*/spec.md` |
| `archived` | 目录必须已移入 `changes/archive/` |

## `change.yaml` 字段

```yaml
name: <kebab-name>        # 必须与目录名一致
status: candidate         # candidate | ready | active | complete | archived
layers: [spec]            # 取值：spec | compiler | stdlib | tooling | benchmark | process | docs
```

`layers` 声明本变更触及的层。**涉及 `compiler`/`stdlib`/`tooling` 且改变语言行为的变更，`layers` 必须同时包含 `spec` 并提供规范增量**；纯内部重构可不带 `specs/`，但必须在 proposal「影响层」中说明理由。

## 规范增量格式

- 章节标题：`## ADDED Requirements` / `## MODIFIED Requirements` / `## REMOVED Requirements` / `## RENAMED Requirements`（按需使用）。
- 每个 `### Requirement:` 至少一个 `#### Scenario:`，场景必须含 `**WHEN**` 与 `**THEN**`。
- 强制义务用 MUST / MUST NOT（BCP 14）；允许偏离用 SHOULD 并写明条件；可选项用 MAY 并写明默认。
- spec 只写黑盒行为（触发、输入、结果、失败路径、可观察状态），不写实现、owner、代码路径、迁移过程。

## tasks.md 格式

每个勾选项后必须紧跟两行：

```
来源：<proposal / Requirement+Scenario / design 章节>
验证：<精确命令 + 预期结果>
```

- 已勾选（`- [x]`）但验证行为空 → 校验失败。
- **验证未实际执行且通过，不得勾选**（由 welang-code-review 语义把关）。
- 禁止路径必须有真实的负向断言，不能用文字描述代替。
- 影响行为/契约/验收的未决问题必须阻塞相关 task，不得用 SHOULD/MAY/TODO 掩盖。

## 校验命令

```sh
python3 openspec/tools/validate.py --all --strict
```

退出码 0 通过、1 失败。strict 模式额外要求：已勾选 task 的验证行非空。

## 归档

变更到 `complete` 并完成基线提升后（技能 `welang-archive-sync`）：

- 规范增量合并进 `docs/spec/`（首次需要时创建）；
- 长期取舍提升为 `docs/decisions/` 下的 ADR；
- 变更目录移入 `changes/archive/`，移入时目录名加 `YYYY-MM-DD-` 日期前缀（归档命名约定，见 `docs/README.md`）；
- 重新跑校验，必须通过。
