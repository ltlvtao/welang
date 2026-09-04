# Tasks: interfaces-generics

- [x] 1. 编写规范增量（specs/interfaces/spec.md 16 条 ADDED / 72 Scenarios + examples.md；specs/lexical、specs/grammar（×2）、specs/declarations（×2）、specs/types、specs/composites（×4）、specs/sum 共 11 条 MODIFIED / 48 Scenarios——含 ch8 Field access 未知成员迁 E0816、无字段赋值句开例外）
来源：proposal What Changes 第 1–7、9 条
验证：python3 openspec/tools/validate.py interfaces-generics --strict 通过；每条 Requirement ≥ 1 个 Scenario（逐条清点：ADDED 72 + MODIFIED 48 = 120）；E0801–E0831/E0501/E0404/E0401系/E0105/E0102/E0601/E0816 交叉引用与 Scenarios 消息串逐一对齐；MODIFIED 场景集与宿主原场景集机器比对（ch1 Keywords +1 新场景、ch2 Statements 1 场景说明更新、ch2 Expression skeleton 原样、ch6 File structure 1 场景 WHEN 更新、ch6 Function declarations +1 新场景、ch7 Type references +2 新场景 +1 场景内容收窄、ch8 Record/Newtype 各 +1 新场景、ch8 Update/Field access 原场景文本更新、ch9 Sum +1 新场景，其余逐字继承）；宿主 MODIFIED 与 What Changes 清单一致

- [x] 2. 注册表维护（段位 E0800–E0899 认领 owner 1000-interfaces + E0801–E0831 共 31 条目 + 未认领段收窄为 E0900–E9999）
来源：proposal What Changes 第 8 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E0801: → 校验报错（记录消息原文），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean（宿主落地前的过渡期 FAIL 属已知瞬态：owner 文件 1000-interfaces.md 尚不存在、requirement 标题待落——与 sum-types 片先例一致，最终绿为准）；每条目 requirement 字段解析到 1000-interfaces.md 对应 Requirement 标题

- [x] 3. 归档与提升（1000-interfaces.md + .zh.md 双语 + 示例节逐字并入 + ch1/ch2/ch6/ch7/ch8/ch9 宿主 MODIFIED 落地×2 + ch7/ch8/ch9 示例节 pending 注释刷新（EN+zh，披露）+ docs_sync）
来源：proposal 目标 1–9；dev-process §7；design D1/D17/D21
验证：第 10 章 16 条 ADDED Requirements 与增量逐字一致（机器 diff 16/16）；11 条 MODIFIED 与宿主章节逐字落地（ch1 Keywords +4 关键字入表 + 新场景；ch2 Statements/Expression skeleton；ch6 File structure/Function declarations；ch7 Type references；ch8 Record/Update/Field access/Newtype；ch9 Sum declarations）；`## Examples (non-authoritative)` 逐字取自 specs/interfaces/examples.md；zh 孪生镜像（结构一致，诊断消息串保留英文原文，MUST/MUST NOT/WHEN/THEN 保留英文，ASCII 冒号扫描 0 命中）；ch7/ch8/ch9 示例节 pending 注释刷新（EN+zh，实现审查记录披露）；docs_sync 全对齐（16→17 对）；validate.py 全仓通过；git status 范围核对（ch1/ch2/ch6/ch7/ch8/ch9 ×2 + 1000 双件 + 注册表 + 变更目录）
