# Tasks: module-system

- [x] 1. 编写规范增量（specs/modules/spec.md 7 条 ADDED / 29 Scenarios + examples.md；ch6 File structure and module identity 1 条 MODIFIED / 3 场景全数逐字继承（末句映射指针落地）、Import declarations 1 条 MODIFIED / 3 场景全数逐字继承（指针句落地）、Top-level bindings 1 条 MODIFIED / 3 场景全数逐字继承（指针句落地）、Visibility with pub 1 条 MODIFIED / 2 场景（pub 枚举句刷新 + 指针句落地 + 场景 1 THEN 占位换 `E1303:` 全码，场景 2 标题「two ratified items」→「ratified items」）；ch7 Type references 1 条 MODIFIED / 6 场景全数逐字继承（末句落地）；ch10 Dyn values 1 条 MODIFIED / 5 场景全数逐字继承（冲突句落地为预导入遮蔽））
来源：proposal What Changes 第 1–4、8 条
验证：python3 openspec/tools/validate.py module-system --strict 通过；每条 Requirement ≥ 1 个 Scenario（逐条清点：ADDED 29 + MODIFIED 22 = 51）；E1301–E1305 消息串与拟注册标题逐字对齐（单码单消息：消息前置、理由后置、首物理行完整）；MODIFIED 场景集与宿主原场景集机器比对（ch6×4 零场景变更除披露的 THEN 替换与场景标题、ch7 零场景变更、ch10 零场景变更）；宿主 MODIFIED 与 What Changes 清单一致

- [x] 2. 注册表维护（段位 E1300–E1399 认领 owner 1500-modules + E1301–E1305 共 5 条目 + 未认领段拆分收窄为 E1400–E9999）
来源：proposal What Changes 第 7 条；AGENTS.md 规则 2
验证：负例注入先行：临时在已落地宿主章使用未注册码 E1399: → 校验报错（记录消息原文），还原 clean（用量扫描只覆盖 docs/spec，delta 不扫——错误章教训）；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean（宿主落地前的过渡期 FAIL 属已知瞬态：owner 文件 1500-modules.md 尚不存在、requirement 标题待落——与 fn-types/iterables/resources/errors 片先例一致，终绿为准）；每条目 requirement 字段解析到 1500-modules.md 对应 Requirement 标题；条目计数 95→100

- [x] 3. 归档与提升（1500-modules.md + .zh.md 双文 + 示例节逐字并入 + ch6×4/ch7/ch10 宿主六条 MODIFIED 落地×2 + ch6「Pending later chapters」示例块 D14 刷新（EN+zh，披露）+ docs_sync）
来源：proposal 目标全文；dev-process §7；design D6
验证：逐字提升 diff（ADDED 7/7、MODIFIED 6/6 零差异；splice 边界正则 `^(## |### )`——`^##+ ` 会吞 `#### Scenario:`，fn-types 教训）；zh 孪生计数镜像（Requirements 7 / Scenarios 29 / 代码块字节一致）+ 全角冒号扫描 `E\d{4}：` 零命中 + 术语对照表；ch6 场景计数 3/3/3/2、ch7 6/6、ch10 5/5 机器比对（差异恰为披露项）；docs_sync --check 21→22 对齐；validate --all --strict + registry clean 终绿

## 完成记录

- 任务 1：specs/modules 7R/29S（审查补强 F1–F3 后由 26 增至 29）+ 六宿主 delta；机器比对全过（ADDED 29 + MODIFIED 22 = 51 场景清点；宿主场景集差异恰为披露项：Visibility with pub 三处）；单码单消息 E1301–E1305/E1204 对齐注册标题（delta 与落地章各扫一遍，含 zh）。
- 任务 2：负例注入两次实测（审计期 + 执行期，目标 docs/spec/0600-declarations.md 与 ch15 前）、均 FAIL `diagnostic usage 'E1399:' has no registry entry`、还原 clean；段位 E1300–E1399 + 五条目落地（95→100）；章文件同步落地故 owner-file 过渡 FAIL 未出现，registry 一次终绿。
- 任务 3：1500-modules.md EN 逐字提升（landed==delta 机器断言 True）；六宿主 EN 拼接（边界正则 `^(## |### )`；修一处拼接吞并块尾空行——三宿主各一处，diff 复核零外溢）；zh 孪生 7R/29S 镜像、5 代码块字节一致、全角冒号零命中、术语对照 18 行；zh 宿主镜像编辑 9 处（ch6×7 + ch7 + ch10，含场景标题「两个已批准项之外」→「已批准项之外」）+ zh Pending 块 D14；宿主场景计数 EN=ZH（3/3/3/2、6/6、5/5）；docs_sync 21→22；validate --all --strict + registry clean 终绿。
