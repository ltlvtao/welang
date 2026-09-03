# Tasks: match-patterns

- [x] 1. 编写规范增量（specs/match/spec.md 6 条 ADDED；specs/grammar/spec.md 1 条 MODIFIED）
来源：proposal What Changes 第 1、2 条
验证：python3 openspec/tools/validate.py match-patterns --strict 通过；每条 Requirement ≥ 1 个 Scenario（6 条共 18 个：4+3+5+2+2+2，delta 与章节双侧 grep 一致）；E0301/E0302 码与 Scenarios 消息串逐一对齐；grammar MODIFIED 与 docs/spec/0200-grammar.md 现行 Expression skeleton diff 核对——差异恰为插入句，旧文零漂移

- [x] 2. 扩展注册表（[segments] 新增 E0300-E0399 + E0300-E9999 收窄为 E0400-E9999 + E02xx 注释去 match arms + E0301/E0302 两目录）
来源：proposal What Changes 第 3 条；AGENTS.md 规则 2
验证：负例注入先行：向 docs/spec/0200-grammar.md 临时追加未注册码 E0303: → 校验报 "diagnostic usage 'E0303:' has no registry entry"（exit=1），git checkout 还原 clean；随后写入段位与条目，python3 openspec/tools/validate.py --all --strict 输出 "1 change(s) valid; registry clean"（条目写入后、章节文件落地前有预期瞬态 FAIL "owner file does not exist" ×2，章节创建后消除）

- [x] 3. 归档与提升（0400-match.md + .zh.md 双语 + 示例节并入 + 第 2 章表达式骨架逐字合并 + docs_sync）
来源：proposal 目标 1；dev-process §7；用户偏好 2026-09-03（示例节通例）
验证：第 4 章 Requirements 与增量逐字一致（diff 核对 6/6，第 6 条尾部差异为章节装置已知误报，本体 common-lines 全等）；第 2 章表达式骨架与 MODIFIED 增量逐字一致（diff 核对 True，3 个 Scenario）；`## Examples (non-authoritative)` 逐字取自 specs/match/examples.md（diff 核对一致）；zh 孪生镜像（docs_sync 结构校验通过，11 对全对齐；诊断消息串保留英文原文，全角冒号扫描为零）；validate.py 全仓通过
