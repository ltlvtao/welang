# Tasks: iterable-protocols

- [x] 1. 编写规范增量（specs/iterables/spec.md 8 条 ADDED / 27 Scenarios + examples.md；specs/iteration/spec.md 2 条 MODIFIED / 11 Scenarios——For statement 6 场景：5 逐字继承 + 1 替换（「destructuring in the loop head is not ratified」→「a tuple pattern in the loop head」）；Range expression 5 场景逐字继承）
来源：proposal What Changes 第 1–3、5 条
验证：python3 openspec/tools/validate.py iterable-protocols --strict 通过；每条 Requirement ≥ 1 个 Scenario（逐条清点：ADDED 27 + MODIFIED 11 = 38）；E0901–E0904/E0501/E0404系/E0105/E0812/E0816/E0817/E0819/E0811 交叉引用与 Scenarios 消息串逐一对齐；MODIFIED 场景集与宿主原场景集机器比对（ch5 For statement 恰一处标题变更 + 文本更新，ch5 Range expression 场景全量逐字继承）；宿主 MODIFIED 与 What Changes 清单一致

- [x] 2. 注册表维护（段位 E0900–E0999 认领 owner 1100-iterables + E0901–E0904 共 4 条目 + 未认领段拆分收窄为 E1000–E9999）
来源：proposal What Changes 第 6 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E0901: → 校验报错（记录消息原文），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean（宿主落地前的过渡期 FAIL 属已知瞬态：owner 文件 1100-iterables.md 尚不存在、requirement 标题待落——与 interfaces 片先例一致，最终绿为准）；每条目 requirement 字段解析到 1100-iterables.md 对应 Requirement 标题

- [x] 3. 归档与提升（1100-iterables.md + .zh.md 双语 + 示例节逐字并入 + ch5 宿主两条 MODIFIED 落地×2 + ch5/ch7 示例 pending 注释刷新（EN+zh，披露）+ docs_sync）
来源：proposal 目标全文；dev-process §7；design D1/D2/D6/D10/D11
验证：逐字提升 diff（ADDED 8/8、MODIFIED 2/2 零差异）；zh 孪生计数镜像（Requirements/Scenarios/代码块 8/8）+ 代码块字节一致 + 全角冒号扫描 `E\d{4}：` 零命中；ch5 场景继承机器比对（差异恰为 D11 披露的一处）；docs_sync --check 17→18 对齐；validate --all --strict + registry clean 终绿
