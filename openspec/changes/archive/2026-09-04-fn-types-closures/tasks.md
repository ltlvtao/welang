# Tasks: fn-types-closures

- [x] 1. 编写规范增量（specs/fn-types/spec.md 7 条 ADDED / 32 Scenarios + examples.md；ch2 Expression skeleton 1 条 MODIFIED / 5 场景（4 逐字继承 + 1 新增）；ch7 Type references 1 条 MODIFIED / 6 场景（4 逐字继承 + 1 新增 + 1 示例替换）；ch9 Variant constructors 1 条 MODIFIED / 4 场景（3 逐字继承 + 1 THEN 尾句替换））
来源：proposal What Changes 第 1–4 条
验证：python3 openspec/tools/validate.py fn-types-closures --strict 通过；每条 Requirement ≥ 1 个 Scenario（逐条清点：ADDED 32 + MODIFIED 15 = 47）；E1001–E1004/E0404/E0102/E0104/E0105/E0501/E0605/E0704/E0816 系消息串与注册表逐字对齐；MODIFIED 场景集与宿主原场景集机器比对（ch2 恰一新增、ch7 一新增一示例替换、ch9 恰一尾句替换）；宿主 MODIFIED 与 What Changes 清单一致

- [x] 2. 注册表维护（段位 E1000–E1099 认领 owner 1200-fn-types + E1001–E1004 共 4 条目 + 未认领段拆分收窄为 E1100–E9999 + E0401 description 扩展）
来源：proposal What Changes 第 5 条；AGENTS.md 规则 2
验证：负例注入先行：临时使用未注册码 E1001: → 校验报错（记录消息原文），还原 clean；扩展后 python3 openspec/tools/validate.py --all --strict 输出 registry clean（宿主落地前的过渡期 FAIL 属已知瞬态：owner 文件 1200-fn-types.md 尚不存在、requirement 标题待落——与 interfaces/iterables 片先例一致，最终绿为准）；每条目 requirement 字段解析到 1200-fn-types.md 对应 Requirement 标题；E0401 扩展仅动 description、title/编号不动

- [x] 3. 归档与提升（1200-fn-types.md + .zh.md 双语 + 示例节逐字并入 + ch2/ch7/ch9 宿主三条 MODIFIED 落地×2 + ch5/ch7/ch9/ch11 示例 D14 刷新（EN+zh，披露）+ docs_sync）
来源：proposal 目标全文；dev-process §7；design D5/D8
验证：逐字提升 diff（ADDED 7/7、MODIFIED 3/3 零差异）；zh 孪生计数镜像（Requirements 7 / Scenarios 32 / 代码块字节一致）+ 全角冒号扫描 `E\d{4}：` 零命中；ch2/ch7/ch9 场景继承机器比对（差异恰为 D8 披露项）；docs_sync --check 18→19 对齐；validate --all --strict + registry clean 终绿
