# Tasks: declarations

- [x] 1. 编写规范增量（specs/declarations/spec.md 8 条 ADDED；specs/declarations/examples.md）
来源：proposal What Changes 第 1–2 条
验证：python3 openspec/tools/validate.py declarations --strict 通过；每条 Requirement ≥ 1 个 Scenario（8 条逐条清点，共 23 个）；E0105/E0013/E0012/E0204/E0401–E0405 与 Scenarios 消息串逐一对齐；零宿主 MODIFIED（What Changes 第 4 条核对）

- [x] 2. 注册表维护（段位 E0400–E0499 认领 + E0401–E0405 五条目 + 未认领段收窄为 E0500–E9999）
来源：proposal What Changes 第 3 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E0501: → "diagnostic usage 'E0501:' has no registry entry in docs/spec/diagnostics.toml"（exit=1），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean（条目先于章文件的 5 条瞬态 FAIL 在第 6 章创建后消解，match 先例）；每条目 requirement 字段解析到本章对应 Requirement 标题

- [x] 3. 归档与提升（0600-declarations.md + .zh.md 双语 + 示例节并入 + docs_sync）
来源：proposal 目标 1、3；dev-process §7；用户偏好 2026-09-03（示例节通例）
验证：第 6 章 8 条 Requirements 与增量逐字一致（机器 diff 8/8 OK，场景集 3+3+3+5+3+2+2+2 匹配）；`## Examples (non-authoritative)` 逐字取自 specs/declarations/examples.md（2643 字符 diff 为零）；zh 孪生镜像（H3 req 8=8、H4 scen 23=23、示例小节 12=12、fence 8=8，诊断消息串保留英文原文，ASCII 冒号扫描 0 命中）；宿主章节零改动复核（git status：docs/spec/ 下仅注册表改动 + 两个新章文件）；docs_sync 13 对全对齐；validate.py 全仓通过
