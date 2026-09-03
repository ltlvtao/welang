---
name: welang-spec-impact-audit
description: Audit a candidate welang change before it may enter "ready". Fixes impact layers, spec-delta scope, principle consistency, reference baselines, and the acceptance boundary. Use when a change is about to move from candidate to ready.
---

# welang 变更影响审计（candidate → ready 关卡）

对 `openspec/changes/<name>/` 执行以下审计。任何一项不通过，变更停留在 `candidate`，不得进入 `ready`。

## 审计清单

1. **问题真实性**：proposal 的 `## Why` 描述的是可验证的黑盒问题（语言行为缺陷/缺口/工程缺口），不是实现偏好。能引用规范章节（`docs/spec/` 或 refr/ 草案的对应条目）或具体工单/评审条目的，必须引用。
2. **影响层声明**：`change.yaml` 的 `layers` 与 proposal `## 影响层` 一致。触及 `compiler`/`stdlib`/`tooling` 且改变语言行为的，必须包含 `spec` 层并提供规范增量计划；纯内部变更必须在 proposal 中写明"不改变语言行为"的边界。
3. **规范增量范围**：明确列出将新增/修改/删除的 Requirement 与诊断码。诊断码不得与既有规范冲突；新增码需全局唯一（对照 `docs/spec/` 与所有 active 变更的 ADDED 段）。
4. **原则一致性**：变更与十条设计原则不冲突；若刻意突破某条原则（如引入新的隐式转换），proposal 必须显式论证，且后续需产出 ADR。
5. **参考基线固定**：引用 refr/ 材料时固定文件与版本（如 `refr/spec-0.8.md` §26.1）；引用外部项目时固定版本/commit。不得把未确认的草案当既成规范。
6. **验收边界**：proposal 的目标/非目标能机械判定"做完了没有"；非目标里显式排除的范围足以防止蔓延。
7. **粒度**：变更是可垂直验证的最小单元；若一个变更需要同时改动三个不相关层才能验收，拆分它。

## 产出

- 审计结论（通过/不通过 + 理由）写回变更目录的审计记录（追加到 proposal.md 末尾「## 审计记录」小节，注明日期与结论）。
- 通过后将 `change.yaml` 的 `status` 更新为 `ready`。
- 运行 `python3 openspec/tools/validate.py <name> --strict` 确认结构合法。
