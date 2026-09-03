---
name: welang-code-review
description: Review a welang change implementation before it may reach "complete". Checks spec compliance, test-first evidence, honest task verification, diagnostics protocol conformance, and single-authority discipline. Use when implementation tasks are done and the change is about to move from active to complete.
---

# welang 实现审查（active → complete 关卡）

## 审查清单

1. **规范符合性**：实现行为与 spec 增量的每个 Requirement/Scenario 一致；编译器接受/拒绝的判定在规范中有依据，无规范外行为。
2. **验证诚实性**：逐个核对 `tasks.md` 已勾选项——验证命令真实运行过、结果与预期一致。任何"勾了但没跑"的 task 立即打回并取消勾选。
3. **测试先行证据**：目标测试先于实现存在（提交历史或测试内容可佐证）；conformance 黄金用例的期望输出符合规范 §52 人类可读格式与 §62 JSON Lines 协议。
4. **诊断协议稳定**：未删除/重命名 `--json` 既有字段；新增诊断码全局唯一且已在 spec 增量中登记。
5. **单一权威**：长期事实没有滞留在变更目录——应进 `docs/spec/`、ADR 的内容标记为待提升（归档时处理）；代码注释/docs 用英文，无中文混入。
6. **红线复核**：`refr/` 未被提交；commit 信息无署名 trailer；diff 中无与变更非目标冲突的越界改动。
7. **最小可信验证已跑**：按验证阶梯对实际 diff 选取的命令全部通过（与 welang-pre-push-checks 的选取标准一致）。

## 产出

- 审查结论与发现的问题记录到变更目录（追加到 proposal.md「## 审查记录」）。
- 通过后将 `status` 更新为 `complete`，进入归档流程（welang-archive-sync）。
