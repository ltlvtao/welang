# Tasks: error-mechanism

- [x] 1. 编写规范增量（specs/errors/spec.md 6 条 ADDED / 27 Scenarios + examples.md；ch2 Expression skeleton 1 条 MODIFIED / 6 场景（5 逐字继承 + 1 新增传播后缀链）、Operator precedence 1 条 MODIFIED / 5 场景全数逐字继承（level 1 行 + 表后句增补）；ch3 Defer 1 条 MODIFIED / 4 场景（3 逐字继承 + 1 新增 defer 体内 `?`）；ch7 Integer overflow semantics 1 条 MODIFIED / 3 场景（正文指针句落定 + 场景 2 THEN 替换）；ch9 The bottom type 1 条 MODIFIED / 4 场景全数逐字继承（产生形式句替换）；ch13 The scope resource statement 1 条 MODIFIED / 8 场景（7 逐字继承 + 1 新增 panic 穿透；R2 增补展开句））
来源：proposal What Changes 第 1–6 条
验证：python3 openspec/tools/validate.py error-mechanism --strict 通过；每条 Requirement ≥ 1 个 Scenario（逐条清点：ADDED 27 + MODIFIED 30 = 57）；E1201–E1204 消息串与拟注册标题逐字对齐（单码单消息：消息前置、理由后置）；MODIFIED 场景集与宿主原场景集机器比对（ch2 骨架恰一新增、ch2 优先表零场景变更、ch3 恰一新增、ch7 恰一 THEN 替换、ch9 零场景变更、ch13 恰一新增）；宿主 MODIFIED 与 What Changes 清单一致

- [x] 2. 注册表维护（段位 E1200–E1299 认领 owner 1400-errors + E1201–E1204 共 4 条目 + 未认领段拆分收窄为 E1300–E9999）
来源：proposal What Changes 第 7 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E1299: → 校验报错（记录消息原文），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean（宿主落地前的过渡期 FAIL 属已知瞬态：owner 文件 1400-errors.md 尚不存在、requirement 标题待落——与 fn-types/iterables/resources 片先例一致，终绿为准）；每条目 requirement 字段解析到 1400-errors.md 对应 Requirement 标题；条目计数 91→95

- [x] 3. 归档与提升（1400-errors.md + .zh.md 双文 + 示例节逐字并入 + ch2×2/ch3/ch7/ch9/ch13 宿主六条 MODIFIED 落地×2 + ch9×2/ch7/ch10/ch13 示例 D14 刷新（EN+zh，披露）+ docs_sync）
来源：proposal 目标全文；dev-process §7；design D6
验证：逐字提升 diff（ADDED 6/6、MODIFIED 6/6 零差异；splice 边界正则 `^(## |### )`——`^##+ ` 会吞 `#### Scenario:`，fn-types 教训）；zh 孪生计数镜像（Requirements 6 / Scenarios 27 / 代码块字节一致）+ 全角冒号扫描 `E\d{4}：` 零命中 + 术语对照表；ch2 骨架场景计数 5→6、ch2 优先表 5/5、ch3 3→4、ch7 3/3（THEN 替换恰一处）、ch9 4/4、ch13 7→8 机器比对（差异恰为 D6 披露项）；docs_sync --check 20→21 对齐；validate --all --strict + registry clean 终绿

## 完成记录

- 任务 1：specs/errors 6R/27S（审查补强 F1/F2 后由 25 增至 27）+ 五宿主 delta；机器比对全过（ADDED 27 + MODIFIED 30 = 55 场景清点；宿主场景集差异恰为披露项）；一码一讯 E1201–E1204/E0105 对齐注册标题（examples 折行注释已修）。
- 任务 2：负例注入宿主章实测 FAIL（`diagnostic usage 'E1299:' has no registry entry`），还原 clean；段位 E1200–E1299 + 四条目落地（91→95）；发现：用量扫描只覆盖 docs/spec，delta 不扫——负例注入须落宿主章。过渡期 4 处 owner-file FAIL 在 ch14 EN 落地后自愈，终绿。
- 任务 3：ch14 EN 逐字提升（landed==delta 机器断言 True）；六宿主 EN 拼接（边界正则 `^(## |### )`，diff 零外溢）；zh 孪生 6R/27S 镜像、5 代码块字节一致、全角冒号扫描零命中、术语对照 21 行；宿主场景计数 EN=ZH（6/5/4/3/4/8）；D14 刷新五处×2 语言（ch9×2、ch7、ch10、ch13 导语）；docs_sync 20→21 对齐（途中修 examples 子节 H2→H3，ch13 先例）；validate --all --strict + registry clean 终绿。
