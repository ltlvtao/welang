# Tasks: for-iteration

- [x] 1. 编写规范增量（specs/iteration/spec.md 2 条 ADDED；specs/lexical/spec.md 1 条 MODIFIED；specs/grammar/spec.md 3 条 MODIFIED）
来源：proposal What Changes 第 1–3 条
验证：python3 openspec/tools/validate.py for-iteration --strict 通过；每条 Requirement ≥ 1 个 Scenario（iteration 2 条逐条清点）；E0104/E0102/E0205 等码与 Scenarios 消息串逐一对齐；四处 MODIFIED 旧文与现行章节 diff 核对零漂移（差异恰为预定插入）

- [x] 2. 注册表维护（E0104 description/remediation 枚举扩展；无新码）
来源：proposal What Changes 第 4 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E0401: → "diagnostic usage 'E0401:' has no registry entry"（exit=1），还原 clean；E0104 扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean；E0104 requirement 字段仍解析到 "Operator precedence and associativity"

- [x] 3. 归档与提升（0500-iteration.md + .zh.md 双语 + 示例节并入 + 第 1/2 章四处 MODIFIED 逐字合并 + docs_sync）
来源：proposal 目标 1；dev-process §7；用户偏好 2026-09-03（示例节通例）
验证：第 5 章 Requirements 与增量逐字一致（机器 diff 2/2 OK，场景集 6+5 匹配）；第 1 章算符清单、第 2 章行接续/语句/优先级三处与 MODIFIED 逐字一致（机器 diff 4/4 OK，场景集 3+4+4+5 匹配）；`## Examples (non-authoritative)` 逐字取自 specs/iteration/examples.md（2028 字符 diff 为零）；zh 孪生镜像（6+5 场景、4 示例子节、8 术语对齐，诊断消息串保留英文原文，ASCII 冒号扫描 0 命中——修复 1 处 E0105：→E0105:）；docs_sync 12 对全对齐；validate.py 全仓通过
