# Tasks: composites-ownership

- [x] 1. 编写规范增量（specs/composites/spec.md 14 条 ADDED / 39 Scenarios；七宿主能力文件 10 条 MODIFIED / 43 Scenarios；specs/composites/examples.md）
来源：proposal What Changes 第 1–2、4 条
验证：python3 openspec/tools/validate.py composites-ownership --strict 通过；每条 Requirement ≥ 1 个 Scenario（逐条清点：ADDED 39 + MODIFIED 43 = 82）；E0601–E0606/E0501/E0404/E0105 与 Scenarios 消息串逐一对齐；宿主 MODIFIED 与 What Changes 第 3 条清单一致（ch1/ch2/ch3/ch4/ch6/ch7 共 10 条）；MODIFIED 场景集与宿主原场景集比对——原有场景全量继承（ch2 Statements 事故后立此核查）

- [x] 2. 注册表维护（段位 E0600–E0699 认领 + E0601–E0606 六条目 + E0501/E0404 description 扩展 + 未认领段收窄为 E0700–E9999）
来源：proposal What Changes 第 3 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E0700: → 校验报错（记录消息原文），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean；每条目 requirement 字段解析到本章对应 Requirement 标题；E0501/E0404 扩展后仍覆盖既有用法（回归：既有章节消息串零改动）

- [x] 3. 归档与提升（0800-composites.md + .zh.md 双语 + 示例节并入 + 七宿主章节 MODIFIED 落地 + docs_sync）
来源：proposal 目标 1、3；dev-process §7；用户偏好 2026-09-03（示例节通例）
验证：第 8 章 14 条 ADDED Requirements 与增量逐字一致（机器 diff 14/14，场景集分布匹配）；10 条 MODIFIED 与宿主章节逐字落地（ch1 Keywords 五词入表 + 新场景；ch2 Line-joining/Statements/Expression skeleton；ch3 If；ch4 Pattern set；ch6 File structure/Function bodies；ch7 Type references/Block value typing）；`## Examples (non-authoritative)` 逐字取自 specs/composites/examples.md；zh 孪生镜像（结构一致，诊断消息串保留英文原文，ASCII 冒号扫描 0 命中）；docs_sync 全对齐（14→15 对）；validate.py 全仓通过；git status 范围核对（七宿主章节 + 新章双件 + 注册表 + 变更目录）
