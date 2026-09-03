# Tasks: types-foundations

- [x] 1. 编写规范增量（specs/types/spec.md 7 条 ADDED；specs/types/examples.md）
来源：proposal What Changes 第 1–2 条
验证：python3 openspec/tools/validate.py types-foundations --strict 通过；每条 Requirement ≥ 1 个 Scenario（7 条逐条清点，共 19 个）；E0501–E0503/E0105 与 Scenarios 消息串逐一对齐；零宿主 MODIFIED（What Changes 第 4 条核对）

- [x] 2. 注册表维护（段位 E0500–E0599 认领 + E0501–E0503 三条目 + 未认领段收窄为 E0600–E9999）
来源：proposal What Changes 第 3 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E0601: → 校验报错（记录消息原文），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean；每条目 requirement 字段解析到本章对应 Requirement 标题

- [x] 3. 归档与提升（0700-types.md + .zh.md 双语 + 示例节并入 + docs_sync）
来源：proposal 目标 1、3；dev-process §7；用户偏好 2026-09-03（示例节通例）
验证：第 7 章 7 条 Requirements 与增量逐字一致（机器 diff 7/7，场景集 2+3+5+3+2+2+2 匹配）；`## Examples (non-authoritative)` 逐字取自 specs/types/examples.md；zh 孪生镜像（结构一致，诊断消息串保留英文原文，ASCII 冒号扫描 0 命中）；宿主章节零改动复核（git status 仅新章 + 注册表 + 变更目录）；docs_sync 全对齐；validate.py 全仓通过
