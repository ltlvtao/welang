# Tasks: control-flow

- [x] 1. 编写规范增量（specs/control-flow/spec.md 6 条 ADDED；specs/grammar/spec.md 1 条 MODIFIED）
来源：proposal What Changes 第 1、2 条
验证：python3 openspec/tools/validate.py control-flow --strict 通过；每条 Requirement ≥ 1 个 Scenario（6 条共 16 个：4+2+3+2+3+2）；E0201–E0204 码与 Scenarios 消息串逐一对齐；grammar MODIFIED 与 docs/spec/0200-grammar.md 现行 Statements 逐字可比（diff 核对无漂移）。delta 内 Requirement 标题大小写与章节/TOML 对齐（sed 修正 5 处，见实现审查记录 F1）

- [x] 2. 扩展注册表（[segments] 新增 E0200-E0299 + E0300-E9999 收窄 + E0201–E0204 四条目）
来源：proposal What Changes 第 3 条；AGENTS.md 规则 2
验证：python3 openspec/tools/validate.py --all --strict 输出 "1 change(s) valid; registry clean"；负例注入：临时使用未注册码 E0205: → 校验报 "diagnostic usage 'E0205:' has no registry entry"（exit=1），移除后 clean（两步法：FAIL→还原 clean，本轮归档前复跑确认消息原文）

- [x] 3. 归档与提升（0300-control-flow.md + .zh.md 双语 + 示例节并入 + 第 2 章 Statements 逐字合并 + docs_sync）
来源：proposal 目标 1；dev-process §7；用户偏好 2026-09-03（示例节通例，design D9）
验证：第 3 章 Requirements 与增量逐字一致（diff 核对 6/6，第 6 条尾部差异为章节装置已知误报，requirement 本体 diff 一致）；第 2 章 Statements 与 MODIFIED 增量逐字一致（diff 核对，4 个 Scenario）；`## Examples (non-authoritative)` 逐字取自 specs/control-flow/examples.md（diff 核对一致）；zh 孪生镜像（docs_sync 结构校验通过；诊断消息串保留英文原文，全角冒号扫描为零）；docs_sync 10 对全对齐；validate.py 全仓通过
