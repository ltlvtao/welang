# Proposal: sum-types

## Why

1. 类型体系欠账第三批到期：ch4:48+78「变体模式随 sum types 配对修订进入」、ch4:96「穷尽性规则（含守卫臂须有无守卫兜底）由 types 章批准」、ch4:82「或模式绑定名类型一致由 types 章批准」（ch8 元组或模式已使其可触发）、ch4:110 段位预留「exhaustiveness, guard coverage, arm agreement」三项中两项待兑现、ch7 R1「Never 随 sum 类型章到达」、ch3:5 与 ch8 R13「never 类型对臂一致判定的排除归 sum 类型章」——七处悬置等待本片。
2. v0.8 素材固定：§11 联合类型（封闭变体、穷尽性、`_exh_ignore`——后者经 ch4 既有裁决不复活）、§11.1 开放集合走 `Dyn<Interface>` 不开开放 sum、§21.3 变体模式（嵌套解构）、§21.4 守卫不计入穷尽证明、§9.2 底类型 `Never`（仅返回类型标注、臂一致排除、全 Never 臂为 Never）。
3. 六片序列第三片（裁决于 types-foundations）：sum+match 修订。Option/Result、错误机制、状态机、AST/协议消息——后端语言一半的数据建模等这一片；穷尽性检查是 P6「绝不静默」在类型层的最大兑现。

## 目标与非目标

目标：

1. 建第 9 章 `docs/spec/0900-sum-types.md`（+zh 孪生）：sum 声明（`type`/`byval type`/`pub` 前缀、变体与位置载荷 0–8、名字空间 E0404、跨行尾随分隔符）、变体构造（调用形式/裸单元变体/跨模块限定）、值 sum（E0702 镜像 E0601）、底类型 Never（标注位置限制 E0703、一致豁免、臂一致排除）。
2. ch4 配对修订：新增 Exhaustiveness（E0305 非穷尽 / E0306 守卫须兜底 / E0307 不可达臂）与 Match arm agreement（E0501 + 语句位 E0605）两条 ADDED；Pattern set（变体模式入集 E0303/E0304、大小写词法切分、refutable 仅 match）、Or-pattern binding consistency（同名同型 E0501）、Guards（两处指针落定）、Match 诊断段位（分配 E0303–E0307）四条 MODIFIED。
3. 宿主修订：ch1 Keywords（+`type` 第 25 词，破坏性如实记录）、ch6 File structure（顶层项 += sum 类型声明）、ch7 Base type inventory（Never/() 非清单成员的指针句落定）——共 7 条 MODIFIED。
4. 注册表：认领段位 E0700–E0799（owner 0900-sum-types），分配 E0303–E0307 与 E0701–E0704 九码；扩展 E0404（sum 类型名与变体名）、E0011（变体名 PascalCase）description；未认领段收窄为 E0800–E9999。
5. ch4/ch7 示例节 pending 注释刷新（EN+zh，非权威节维护，实现审查记录披露）。

非目标：

1. panic/todo/trap 等 `Never` 产生式与捕获边界 → 错误机制章；本片只定 Never 的位置与一致语义。
2. 接口、`Dyn<Interface>` 开放集合（v0.8 §11.1 的开放侧）、derives（Eq/Show/Encodable on sums）→ 接口+泛型片。
3. 函数值化构造器（`let g = Circle` 是否为值）→ 函数类型片；本片以 E0704 关闭至彼时。
4. `if let` / `let else` → 否决记录（design D9），非延后承诺；refutable 绑定走 match 单通道。
5. 泛型 sum（`type Option<T> = ...`）→ 接口+泛型片；本片变体载荷只收已批准类型引用。
6. sum 的内存布局（标签表示、 niche 优化）→ 实现层，不入规范。
7. 变体在模块间的选择性导入/重导出细则 → 模块系统章节；本片只定 `module.Name(args)` 限定形式与 ch6 唯一名字空间归属。
8. 嵌套类型内声明 sum（函数体内 type）→ 不存在：类型声明是顶层项（ch6 修订后清单封闭）。

## What Changes

1. `specs/sum/spec.md`：4 条 ADDED Requirements / 16 Scenarios——Sum declarations / Variant constructors / Value sums / The bottom type。
2. `specs/match/spec.md`：2 条 ADDED（Exhaustiveness 7 Scenarios / Match arm agreement 3 Scenarios）+ 4 条 MODIFIED（Pattern set 10 Scenarios / Or-pattern binding consistency 2 / Guards 2 / Match diagnostics segment 2）。
3. 宿主修订（3 个能力文件，3 条 MODIFIED / 9 Scenarios）：`specs/lexical/spec.md`（Keywords，+1 场景）、`specs/declarations/spec.md`（File structure，场景 1 改写）、`specs/types/spec.md`（Base type inventory，指针句落定）。
4. `docs/spec/diagnostics.toml`：段位 E0700–E0799 认领；E0303–E0307、E0701–E0704 九条目；E0404/E0011 description 扩展。
5. `specs/sum/examples.md`：示例节（非权威，提升时逐字并入第 9 章）。

合计：6 ADDED / 26 Scenarios + 7 MODIFIED / 25 Scenarios = 13 Requirements / 51 Scenarios。

## 影响层

- spec：仅规范层（一个新章增量 + 四处宿主修订 + 注册表段位与条目）。

## 影响范围

- 新章 `docs/spec/0900-sum-types.md`（+zh）；`docs/spec/0100-lexical.md`、`0400-match.md`、`0600-declarations.md`、`0700-types.md`（各含 MODIFIED 落地与示例节刷新）；`docs/spec/diagnostics.toml`。

## 裁决记录

2026-09-04 用户裁决四项（选项问答，均含推荐采纳）：

1. **声明关键字取 `type`**——v0.8 原案；未来别名同关键字复用（一词两形）；破坏性走 ch1 预授权机制如实记录。
2. **变体载荷仅位置形式**——`Circle(Float64)`、`None`；命名字段需求装 record 载荷；v0.8 §11 具名载荷形式不继承（偏差披露）。
3. **穷尽性兜底仅 `_`**——维持 ch4 既有裁决（`_exh_ignore` 不存在），本片复核后确认不复活。
4. **不可达臂取错误级 E0307**——v0.8 无此检查，有意加严（偏差披露）；静态可判定的必然笔误按 P6 报错。

## 审计记录

2026-09-04，状态 candidate → ready 前置审计（welang-spec-impact-audit 七点）：

1. **问题真实性：确认。** 本片偿付的悬置引用逐一存在：ch4:48 与 :78「变体模式随 sum types 配对修订进入，此前 E0105 拒绝」、ch4:96「穷尽性规则（含守卫臂须有无守卫兜底）由 types 章批准」、ch4:82「或模式绑定名类型一致由 types 章批准」、ch4:110 段位预留三项中 exhaustiveness 与 arm agreement 两项待兑现、ch7:5「Never 随 sum 类型章到达」、ch3:5 与 ch8 R13「never 类型对臂一致判定的排除归 sum 类型章」——七处全部落地。另发现两处示例节失真（ch4:156 变体模式 pending 注释、ch7:220 Never/() pending 注释——后者自 ch8 起已失真）→ 以 D14 披露并列入实现期刷新。遗留引用仍有主：ch5 迭代协议 → 可迭代协议片；ch2:86 方法侧 → 接口片；Never 产生式 → 错误机制章；裸载荷构造器 → 函数类型片。无无主引用。
2. **影响层准确性：确认。** layers `[spec]` 与实际文件集一致（四处宿主修订 + 新章 + 注册表，全规范层）。
3. **增量范围与完整性：确认。** ADDED 6/26 + MODIFIED 7/25 = 13 Requirements / 51 Scenarios（机器清点一致），每条 Requirement ≥ 1；E0303–E0307/E0701–E0704 九码全部有消息串场景；E0501/E0404/E0105/E0102 复用措辞与宿主一致；MODIFIED 场景集与宿主机器比对——起草期一处承袭事故（or-pattern 原有「consistent or-pattern」场景丢失）被核查抓回修复，最终仅两处有意变更（弃「变体模式未批准」场景=变更本体；ch6 场景 1 改写入 sum 声明）。
4. **原则一致性：确认。** 穷尽性检查=P6 在类型层的最大兑现；大小写词法切分（变体 PascalCase/绑定 camelCase）使裸模式无类型导向歧义=P1；单通配符 `_`=P2；Never 一致豁免按 ch7 R3 预写的「规范层例外」通道显式设立；`type` 入词走 ch1 预授权机制。无 ADR 触发，权衡入 design D1–D15。
5. **参考基线固定：确认。** v0.8 §11/§11.1/§21.3/§21.4/§9.2 素材已钉。偏差披露：v0.8 码位（E0301/E0302/E0305/E0306/E0311）不继承（我方 E0301/E0302 已另有其义），新分配 E0303–E0307/E0701–E0704；具名载荷不继承（裁决 2）；`_exh_ignore` 不复活（ch4 既有裁决复核，裁决 3）；**E0307 不可达臂超出 v0.8 基线**（裁决 4 有意加严）；Never 不入基础类型清单（v0.8 §9 表偏差，D7）；Bool 不做穷尽特例（v0.8 总览表继承，D15）；开放集合走 `Dyn<Interface>` 立场原样继承（§11.1，机制归接口片）。
6. **验收标准可机械判定：确认。** tasks.md 三项全机检：validate --strict、计数清点（含机器场景继承比对，or-pattern 事故后已为标准步骤）、注入负例 + 还原、逐字 diff（ADDED 4/4 + MODIFIED 7/7 宿主落地）、zh 镜像结构计数、ASCII 冒号扫描、docs_sync（15→16 对）、git status 范围核对、ch4/ch7 示例刷新核对（EN+zh）。
7. **粒度与耦合：确认。** 六片之三；依赖仅 ch1–ch8（已批准），向后续片单向输出（接口片收 derives/Dyn、函数类型片收一等构造器、错误机制章收 Never 产生式、模块系统章收变体跨模块细则）。E0700–E0799 段位与既有段零交叠；E0303–E0307 兑现 ch4 段位预留的字面承诺。

## 审查记录

2026-09-04，状态 ready → active 前置审查（welang-change-review 十点）：

1. **职责边界——proposal：通过。** 全程黑盒（问题、目标、影响范围）；章节架构决策在 design D1，proposal 不含实现路径。
2. **职责边界——spec 增量：通过。** 全部为触发/结果/诊断码黑盒行为；未注册码防御以「负例注入任务」承载而非散文。MUST/MUST NOT 使用符合 BCP 14；无 SHOULD/MAY 悬空（Never 两处小写 `may` 为描述性散文，规范力由 E0703 与一致条款显式锚定——核查确认无第二读法）。
3. **职责边界——design：通过。** D1–D15 唯一最小路径；9 项被拒方案含理由；引用精确到条款（ch4:110 段位预留、ch7 R3 例外通道、ch2 续行集）。
4. **职责边界——tasks：通过。** 三项均出自 What Changes；各有来源/验证；无 deferred、无 non-goal 混入、无未决方案。
5. **场景覆盖：通过。** normal（解析/构造/匹配/拷贝）、boundary（载荷九元、跨行尾随分隔、守卫不计数、全 Never 臂）、failure-degradation（九码全部带消息串的拒绝场景）三路俱备，非纯 happy path。
6. **无空章节：通过。** 13 Requirements 均有实质行为增量；无包装层。
7. **测试先行：通过（spec 层等价形式）。** layers=[spec]、仓库尚无编译器；等价物=任务 1 在注册表/宿主动笔**之前**完成增量验证 + 机器场景继承比对（起草期 or-pattern 事故即被此步骤抓回），沿用 foundations/composites 先例。
8. **负向断言：通过。** 任务 2 含真实负例：未注册码 E0800 临时注入 → 校验失败 → 记录消息原文 → 还原 clean；非文字断言。
9. **完成度闭环：通过（spec 层裁定）。** 原则 10 三要素中，类型检查侧（穷尽/臂一致/载荷一致/E0702 拷贝性）与可观察行为侧（值拷贝、模式绑定）本片全覆盖；代码生成与运行时表示为声明的非目标 6（内存布局显式排除，实现层后置）——spec-first 流程下本片交付物即规范文本，先例同前两片。
10. **未决问题阻塞：通过。** 四项裁决全部闭合；遗留引用全部有主（接口片/函数类型片/错误机制章/模块系统章），无阻塞任务。

结论：通过，进入 active。

## 实现审查记录

2026-09-04，active → complete 实现审查（welang-code-review 七点）：

1. **规范符合性：通过。** 宿主落地逐字：ch9 ADDED 4/4、ch4 ADDED 2/2、ch4 MODIFIED 4/4、ch1/ch6/ch7 MODIFIED 各 1/1（机器 diff 全绿）；9 条注册表条目 requirement 字段全部解析到 owner 文件（validate --all --strict registry clean）。
2. **验证诚实性：通过。** 三项任务验证命令全部真实运行：任务 1（validate --strict + 计数 26+25=51 + 场景继承机器比对——实现期对改动后的增量复跑一次）；任务 2（哨兵注入先行：`E0800:` 注入 → FAIL 消息原文 `diagnostic usage 'E0800:' has no registry entry` 捕获 → 还原 clean；扩展后 --all --strict clean）；任务 3（docs_sync 15→16 对、逐字 diff、zh 镜像 R/S 计数全对齐、ASCII 冒号扫描 0 命中、git status 范围与预期逐一相符）。
3. **测试先行证据：通过（spec 层等价形式）。** 各验证在对应步骤当场执行并留有输出；编译器尚不存在，conformance 协议项 N/A（先例同前三片）。
4. **诊断协议稳定：通过。** E0303–E0307、E0701–E0704 全局唯一；零删除、零重命名、零重定义；E0404/E0011 仅扩 description，title 与各章消息串零改动（回归核对：宿主章节消息串未动）；未认领段收窄 E0800–E9999 后 E0800 即新哨兵起点。
5. **单一权威：通过，附披露。** 除增量外仅动两类非权威节：ch4/ch7 示例节刷新（D14 预先披露）与 ch4 术语表补 4 行（穷尽兜底/变体模式/不可达臂/臂类型一致——新批准概念入表，zh 孪生翻译所需）。具体刷新：ch4「模式」块原 `other =>` 后跟 `_ =>` 在 E0307 下已成错误形式，改写为绑定臂收尾并注明不可达语义；ch4 增「穷尽性与臂类型一致」示例块（新 Requirements 的章内示例覆盖，仅用第 1–4 章形式）；ch7 弃已失真的元组 pending 行（ch8 落地时漏刷，本次一并纠正）、pending 块收缩至仍待者。ch4 示例导语「变体与元组模式待成对修订」句改为指向各自章节。
6. **红线复核：通过。** refr/ 未入 diff；提交信息将无署名 trailer；git 范围恰为声明集合（ch1/ch4/ch6/ch7 ×2 + 0900 双件 + 注册表 + 变更目录），无越界改动。
7. **最小可信验证已跑：通过。** validate --all --strict、docs_sync --check（16 对）、逐字 diff/继承/镜像/冒号四项机检全绿。

**实现期修正披露（两处，均已在增量与宿主双侧同步修正）：**
- **臂间逗号缺陷**：examples.md 草稿与两处 WHEN 行以内联逗号书写多臂（`Circle(r) => r,`）——ch4 明确臂以换行分隔、逗号即 E0105，所示程序不可能解析到达类型检查。提升装配时自纠：示例块全部去逗号；Exhaustiveness 与 Match arm agreement 两场景 WHEN 行改写为「holds the arms ... and ...」措辞。审查记录之后、归档之前的增量修订，如实体 now。
- **WHEN 行修正随附**：上述两 WHEN 行修订涉及已过 ready 审查的增量文本；修订不改变行为语义（同一判定的合法表述），已同步 delta/ch4 EN/ch4 zh 三处。

结论：通过，进入 complete，随即归档。

## 归档记录

2026-09-04，complete → archived（welang-archive-sync）：

1. **规范提升**：第 9 章 `docs/spec/0900-sum-types.md`（+zh 孪生）新建并入基线——4 Requirements / 16 Scenarios 逐字取自增量，示例节逐字取自 specs/sum/examples.md（装配期修正披露见实现审查记录）；ch4 两 ADDED（Exhaustiveness 7S / Match arm agreement 3S）按章序落于 Guards 之后、Match 诊断段位之前；ch1/ch4/ch6/ch7 七处 MODIFIED 双语落地。
2. **决策提升**：无新 ADR——全部取舍为规范内部决策，权威文本即规范自身与本章 design.md（随目录归档，可追溯）；docs/decisions/ 现有两条 ADR 不受影响。
3. **状态提升**：docs/roadmap/ 不存在，按技能约定在变更目录留档（本记录 + design.md D1–D15）。
4. **移动归档**：`openspec/changes/sum-types` → `openspec/changes/archive/2026-09-04-sum-types`；change.yaml status → archived。
5. **复验**：validate --all --strict 通过（归档后复跑）。

诊断码全库对照：46 条目；段位 E0001–E0799 已认领，E0800–E9999 未认领；E0303–E0307 兑现 ch4 段位预留、E0701–E0704 为 0900-sum-types 段首批分配。
