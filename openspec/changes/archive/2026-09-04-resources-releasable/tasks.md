# Tasks: resources-releasable

- [x] 1. 编写规范增量（specs/resources/spec.md 7 条 ADDED / 30 Scenarios + examples.md；ch1 Keywords 1 条 MODIFIED / 6 场景（5 逐字继承 + 1 新增）；ch2 Statements 1 条 MODIFIED / 6 场景全数逐字继承；ch3 Defer 1 条 MODIFIED / 3 场景全数逐字继承（正文指针句替换）；ch8 Resource records 1 条 MODIFIED / 2 场景（1 THEN 尾句替换 + 1 逐字继承））
来源：proposal What Changes 第 1–5 条
验证：python3 openspec/tools/validate.py resources-releasable --strict 通过；每条 Requirement ≥ 1 个 Scenario（逐条清点：ADDED 30 + MODIFIED 17 = 47）；E1101–E1106 消息串与拟注册标题逐字对齐（单码单消息：消息前置、理由后置）；MODIFIED 场景集与宿主原场景集机器比对（ch1 恰一新增、ch2/ch3 零场景变更、ch8 恰一尾句替换）；宿主 MODIFIED 与 What Changes 清单一致

- [x] 2. 注册表维护（段位 E1100–E1199 认领 owner 1300-resources + E1101–E1106 共 6 条目 + 未认领段拆分收窄为 E1200–E9999 + E0204 remediation 实名维护）
来源：proposal What Changes 第 6 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E1199: → 校验报错（记录消息原文），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean（宿主落地前的过渡期 FAIL 属已知瞬态：owner 文件 1300-resources.md 尚不存在、requirement 标题待落——与 fn-types/iterables 片先例一致，终绿为准）；每条目 requirement 字段解析到 1300-resources.md 对应 Requirement 标题；条目计数 85→91；E0204 仅动 remediation 文本、title/severity/编号不动（9900:33 禁改码义，此为消歧维护，E0401 先例）

- [x] 3. 归档与提升（1300-resources.md + .zh.md 双语 + 示例节逐字并入 + ch1/ch2/ch3/ch8 宿主四条 MODIFIED 落地×2 + ch8 示例 D14 刷新（EN+zh，披露）+ docs_sync）
来源：proposal 目标全文；dev-process §7；design D6
验证：逐字提升 diff（ADDED 7/7、MODIFIED 4/4 零差异；splice 边界正则 `^(## |### )`——`^##+ ` 会吞 `#### Scenario:`，fn-types 教训）；zh 孪生计数镜像（Requirements 7 / Scenarios 30 / 代码块字节一致）+ 全角冒号扫描 `E\d{4}：` 零命中 + 术语对照表；ch1 场景计数 5→6、ch2 6/6、ch3 3/3、ch8 2/2 机器比对（差异恰为 D6 披露项）；docs_sync --check 19→20 对齐；validate --all --strict + registry clean 终绿
