---
name: welang-archive-sync
description: Promote a completed welang change's long-term facts to their authoritative locations and archive the change directory. Use when a change has reached "complete" and is ready for baseline promotion to "archived".
---

# welang 归档基线提升（complete → archived）

## 前置确认

- 变更 `status` 为 `complete`，`tasks.md` 全部勾选且验证可复核。
- `python3 openspec/tools/validate.py <name> --strict` 通过。
- code-review 结论已记录在 proposal.md「## 审查记录」。

## 提升步骤（顺序执行）

1. **规范提升**：将 spec 增量（ADDED/MODIFIED/REMOVED/RENAMED）合并进 `docs/spec/` 下的权威规范。目录或版本文件不存在时本次创建（如 `docs/spec/we-lang-spec-v0.9.md`），并保证：
   - 诊断码全局唯一，无与既有规范的冲突定义；
   - § 交叉引用在合并后仍然有效；
   - 术语与既有章节一致（同一概念不引入第二种称呼）。
2. **决策提升**：design 中具有长期价值的取舍（被拒方案、原则例外、信任边界）提炼为 `docs/decisions/ADR-<nn>-<slug>.md`（首次创建目录与索引），包含：状态、背景、决策、后果、被拒替代方案。
3. **状态提升**：执行状态（若已有 `docs/roadmap/`）同步更新；尚无 roadmap 时在 `openspec/changes/` 层面留档即可，不创建空目录。
4. **移动归档**：`git mv openspec/changes/<name> openspec/changes/archive/<name>`；`change.yaml` 的 `status` 改为 `archived`。
5. **复验**：重跑 `python3 openspec/tools/validate.py --all --strict`，必须通过。

## 提交

- 归档提交与规范提升可合并为一个提交，信息说明"为什么"（英文、无署名 trailer）。
- 提交前过 welang-pre-push-checks（diff 含 docs/spec/ 时含规范一致性复核）。
