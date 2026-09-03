---
name: welang-change-review
description: Semantic review of a welang change's four artifacts (proposal / spec deltas / design / tasks) before activation. Strict structural validation does NOT replace this review. Use when a change is about to move from ready to active.
---

# welang 变更工件语义审查（ready → active 关卡）

先跑 `python3 openspec/tools/validate.py <name> --strict`，结构失败直接打回；结构通过后做以下**语义**审查。validate 只查形状，本技能查内容。

## 职责边界审查

1. **proposal**：只讲黑盒问题与目标；不混入实现决策、任务清单。
2. **spec 增量**：只写黑盒行为（触发/输入/结果/失败路径/可观察状态）。出现实现方式、owner、代码路径、迁移过程即为越界。MUST/MUST NOT/SHOULD/MAY 使用符合 BCP 14；SHOULD 写明偏离条件，MAY 写明默认值。
3. **design**：给出唯一、最小、可实施路径；记录被拒绝的替代方案及理由；引用规范章节/诊断码/原则处精确到条目。与 spec 增量无矛盾。
4. **tasks**：只执行前三者已定义的工作。每个勾选项有来源与验证；禁止出现 deferred、non-goal、未决方案选择。

## 内容质量审查

5. **场景覆盖**：change 整体覆盖实际存在的 normal / boundary / failure-degradation 路径。只有 happy path 的 spec 增量打回。
6. **无空章节**：没有为凑格式而生的空章节、N/A 表格、无行为增量的包装层。
7. **测试先行**：行为变更的第一个 task 必须是"建立会失败的目标测试"；重构必须是"先建立通过的 characterization"。
8. **负向断言**：变更声明禁止某模式时，tasks 中必须有真实的负向验证（构造违规样例并断言其被拒绝），不能用文字代替。
9. **完成度闭环**：语言特性类变更的 design 同时给出类型检查、代码生成、运行时三要素的方案；缺一即打回（原则 10）。
10. **未决问题阻塞**：影响行为/契约/验收的开放问题必须使相关 task 保持未勾选并显式标注阻塞，不得用 SHOULD/MAY/TODO 掩盖。

## 产出

- 审查结论追加到 proposal.md「## 审计记录」（或「## 审查记录」），注明发现的问题与处置。
- 通过后将 `status` 更新为 `active`，方可开始实现。
