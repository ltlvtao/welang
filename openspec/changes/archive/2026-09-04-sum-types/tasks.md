# Tasks: sum-types

- [x] 1. 编写规范增量（specs/sum/spec.md 4 条 ADDED / 16 Scenarios；specs/match/spec.md 2 条 ADDED / 10 Scenarios + 4 条 MODIFIED；specs/lexical、specs/declarations、specs/types 各 1 条 MODIFIED——共 6 ADDED / 26 Scenarios + 7 MODIFIED / 25 Scenarios）
来源：proposal What Changes 第 1–5 条
验证：python3 openspec/tools/validate.py sum-types --strict 通过；每条 Requirement ≥ 1 个 Scenario（逐条清点：ADDED 26 + MODIFIED 25 = 51）；E0303–E0307/E0701–E0704/E0501/E0404/E0105/E0102 与 Scenarios 消息串逐一对齐；MODIFIED 场景集与宿主原场景集比对（机器：ch4 Pattern set 弃「变体模式未批准」1 条系变更本体、其余全量继承；ch1/ch6/ch7 逐字核对）；宿主 MODIFIED 与 What Changes 清单一致

- [x] 2. 注册表维护（段位 E0700–E0799 认领 owner 0900-sum-types + E0303–E0307 五条目 + E0701–E0704 四条目 + E0404/E0011 description 扩展 + 未认领段收窄为 E0800–E9999）
来源：proposal What Changes 第 6 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E0800: → 校验报错（记录消息原文），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean；每条目 requirement 字段解析到 owner 章节对应 Requirement 标题；E0404/E0011 扩展后仍覆盖既有用法（回归：既有章节消息串零改动）

- [x] 3. 归档与提升（0900-sum-types.md + .zh.md 双语 + 示例节并入 + ch1/ch4/ch6/ch7 宿主 MODIFIED 落地×2 + ch4/ch7 示例节刷新（披露）+ docs_sync）
来源：proposal 目标 1–3；dev-process §7；design D14
验证：第 9 章 4 条 ADDED Requirements 与增量逐字一致（机器 diff 4/4）；7 条 MODIFIED 与宿主章节逐字落地（ch1 Keywords +type 入表 + 新场景；ch4 Pattern set/Or-pattern/Guards/Match 诊断段位；ch6 File structure；ch7 Base type inventory）；ch4 两新增 Exhaustiveness/Match arm agreement 落宿主（按章内既有序位）；`## Examples (non-authoritative)` 逐字取自 specs/sum/examples.md；zh 孪生镜像（结构一致，诊断消息串保留英文原文，ASCII 冒号扫描 0 命中）；ch4/ch7 示例节 pending 注释刷新（EN+zh，实现审查记录披露）；docs_sync 全对齐（15→16 对）；validate.py 全仓通过；git status 范围核对（ch1/ch4/ch6/ch7 ×2 + 0900 双件 + 注册表 + 变更目录）
