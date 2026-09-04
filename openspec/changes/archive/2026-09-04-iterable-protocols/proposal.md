# Proposal: iterable-protocols

## Why

类型队列六片计划的第 5 片：可迭代协议——for 循环的类型侧地基。已落定的规范为本片留位的引用逐一兑现：

- ch5:5 For statement「Whether the iterable expression's type may be iterated — the iterable and iterator protocols — is ratified by the types chapter」——可迭代合法性悬置
- ch5:39 Range expression「The Range type, its typing, iteration over non-numeric operands, and its iterable implementation are ratified by the types chapter」——Range 类型本体悬置
- ch7:5 String「logical iteration yields `Rune`, whose protocol is ratified by the iterable-protocols chapter」——按名指向本章
- ch5:32 场景「destructuring in the loop head is not ratified … until tuple patterns are ratified by their paired amendment」——第 8 章已批准元组模式（用于 let/var），该场景理由文本已过期，本片一并处置
- ch5/ch7 示例 pending 块的迭代协议注释（D14 刷新义务；ch5 示例中 `for (a, b) in pairs` 行现为 E0105 拒绝示例）
- v0.8 §21.1.1/§21.2.1 的 `Iterator<T>`/`Iterable<T>` 协议、§21.2.2 组合子延后（需闭包）

## What Changes

1. **新增第 11 章 `1100-iterables.md`「Iterable protocols」**（段位 E0900–E0999）：8 条 ADDED Requirements / 27 个 Scenarios——Option 类型（标准库唯一 canonical 选项 sum）、Iterator 接口（`fn next(mut self) -> Option<T>`，一次性消耗语义、开放实现、值类别头 E0812、组合子延后）、Iterable 接口（`type Iter` + `fn iterator(self) -> Iter`，静态分发、E0904 句柄契约及其约束反映、可重复迭代、E0819 不可装箱 / `Dyn<Iterator<T>>` 可装箱）、for 循环协议（单一形式 E0901、求值一次取一次迭代器、裸迭代器不可 for）、String 迭代（内建 `Iterable<Rune>`，落地 ch7 既录事实；Bytes 不可迭代）、Range 类型（内建泛型 `Range<T>`，T 限八整数类型 E0902、仅经 `..` 构造、边界对即值、内建 `Iterable<T>` 单位步长）、Iterable 实现方义务（resource 类别禁止实现 E0903、value 类别按拷贝迭代、gc 快照/游标义务归集合章）、诊断段位。
2. **ch5 For statement MODIFIED**：name 位接受 ch8 元组模式（元素类型须为元组，错配 E0501）；「协议归类型章」句替换为 E0901 合法性 + 指向第 11 章执行模型；场景「destructuring in the loop head is not ratified」由拒绝转为批准（标题改为「a tuple pattern in the loop head」，其余 5 场景逐字继承）。
3. **ch5 Range expression MODIFIED**：「Over numeric operands」收窄为「Over integer operands——八章八整数类型，其他类型含 Float 以 E0902 拒」；「Range 类型归类型章」句替换为指向第 11 章；5 场景全部逐字继承。
4. **ch7 零修改**：其 Base type inventory 已按名指向 iterable-protocols 章，事实落地即可；仅示例 pending 块 D14 刷新（String 迭代行兑现）。
5. **ch5 示例 D14 刷新（EN+zh，披露）**：pending 块协议部分兑现（组合子仍延后）；「for over an iterable expression」块中 `for (a, b) in pairs` 行由 E0105 拒绝示例转为合法形式。
6. **注册表**：段位 E0900–E9999 拆分为 E0900–E0999（owner 1100-iterables）+ E1000–E9999（未认领）；新增 E0901–E0904 四条目（77→81）。

## 裁决记录（用户 2026-09-04 五项，均采纳推荐）

1. **元素类型用泛型参数**（`Iterator<T>`/`Iterable<T>`，v0.8 §21.1.1 成文理由：元素类型由容器实例化决定，一个泛型 impl 覆盖家族；无关联类型可 Dyn 装箱；where 约束更短）——备选 Rust 式关联类型被否。
2. **Iterator 开放实现**（偏离 v0.8 E0705 编译器封闭：自定义集合需要自定义迭代器；组合子后续作为接口默认方法落地，行为由规范固定，兼顾开放与控制）。
3. **Range 操作数仅整数**（偏离 v0.8 数值：浮点单位步长有表示误差累积，元素个数依赖位表示，与 P1 局部可判定相悖；Float 操作数以新码 E0902 拒绝，浮点区间用显式循环）。
4. **for 头接受第 8 章元组模式**（`for (k, v) in pairs` 与 let/var 对称；元素类型非元组按 ch8 规则 E0501）。
5. **迭代器句柄静态分发**（`type Iter` 关联类型 + `fn iterator(self) -> Iter`，每 impl 绑定具体迭代器类型；Rust 路线零成本，符合商用语言极致性能定位）。代价披露：`Dyn<Iterable<T>>` 因含关联类型不再合法（E0819 既有规则自然生效）；v0.8「Iterator 自动满足 Iterable」内建 impl 在静态分发下无法表达（Iter 绑定值不能是接口名，E0821），故 for 只认 Iterable 单一形式，裸迭代器经显式 `next` 消费——v0.8 的「for 半消耗迭代器从当前位置继续」语义不再存在，属有意偏离。

## 目标与非目标

目标：

- 落地 ch5/ch7 全部指向本片的悬置引用，for 语句类型侧闭环
- `Option<T>` 作为标准库 canonical 选项类型进入规范（next() 的返回类型所需，也是后续章节的选项载体）
- 迭代协议零成本静态分发，为集合章与组合子变更铺轨
- ch5 过期场景理由的诚实处置（元组模式批准）

非目标：

- 迭代器组合子（map/filter/take/skip 及急性族）——需函数值，随其所属变更落地为 Iterator 默认方法
- 集合类型（List/Map/Set）及其快照/游标义务、下标访问——集合章
- 标准库迭代器类型命名（rune 迭代器、range 迭代器的具体名）——标准库自身表面
- Option 的方法清单（unwrap 等）——标准库自身表面
- `?` 传播、Result——错误机制章

## 影响层

`spec`：新增 1100-iterables.md（+zh）；修改 0500-iteration.md（+zh，2 条 MODIFIED）；diagnostics.toml（段位拆分 + 4 条目）；0500/0700 示例刷新（EN+zh）。

## 影响范围

- docs/spec/1100-iterables.md（新增）、1100-iterables.zh.md（新增）
- docs/spec/0500-iteration.md / .zh.md（For statement、Range expression 两条 MODIFIED + 示例刷新）
- docs/spec/0700-types.md / .zh.md（仅示例 pending 块刷新）
- docs/spec/diagnostics.toml（段位拆分 + E0901–E0904）
- docs_sync 文档对 17→18

## 审计记录（welang-spec-impact-audit，2026-09-04）

1. **问题真实性：通过**。Why 各条均锚定活性文本：ch5:5（For statement 协议悬置句）、ch5:39（Range 类型悬置句）、ch7:5（String 迭代按名指向本章）、ch5:32（过期场景「until tuple patterns are ratified」）、ch5/ch7 示例 pending 注释；v0.8 参照固定为 refr/spec-0.8.md §21.1.1/§21.2.1/§21.2.2。起草中发现并裁决的 E0821 冲突（v0.8 协议声明用裸接口作返回标注）记录于 design D1。
2. **影响层声明：通过**。change.yaml `layers: [spec]` 与 proposal 影响层一致，纯规范层。
3. **规范增量范围：通过**。ADDED 8（新章）+ MODIFIED 2（For statement、Range expression——标题与宿主 0500-iteration.md:3/37 逐一相符）；新码 E0901–E0904 全局唯一（docs/spec 全文除未认领段位行外零 E09xx）；引用既有码 E0501/E0811/E0812/E0816/E0817/E0819/E0821 的消息串与注册表标题逐字一致；一码一消息（E0901 三处、E0902 两处消息串相同）。
4. **原则一致性：通过**。Range 限整数服务于 P1（元素个数可判定）；for 单一形式与显式 Dyn 保持 P2；无新隐式转换（for 头元组模式错配走 ch8 既有 E0501 通道）。对 v0.8 的三处偏离（Iterator 开放、Range 限整数、无自动满足）均在裁决记录中论证；无机制/依赖级决策，不触发 ADR。
5. **参考基线固定：通过**。refr/spec-0.8.md 固定文件与节号。
6. **验收边界：通过**。目标可机械判定（文件清单、计数 8/27 与 2/11、落地项）；非目标排除组合子/集合/标准库命名/Option 方法/`?` 传播。
7. **粒度：通过**。单一规范层垂直切片：一章 + 两宿主块 + 注册表，无跨层依赖。

**结论：通过，进入 ready。**

## 审查记录（welang-change-review，2026-09-04）

结构 validate --strict 先行通过。语义十点：

1. **proposal：通过**。只述黑盒缺口与目标；裁决记录为决策留档，无任务清单混入。
2. **spec 增量：修正后通过（F1/F2）**。F1：R1 canonical 义务与 R2 一次性消耗/耗尽永久性两处载重义务缺 BCP-14 关键词（declarations 片 F1 同类）——已补 MUST/MUST NOT；R2 的「statically dispatched」经斟酌保留：它是名义分发与无子类型规则的可判别语义推论（调用解析到唯一具体 impl），非代码生成指令。F2：R3 约束反映句「known only as implementing」不精确——具体已知实现的类型同样携带方法集，已改为「known to implement — concretely, or only through a bound」。
3. **design：通过**。D1 记录被否的 Dyn 装箱替代及代价；D2/D3 与 delta 无矛盾；引用精确到条目（ch8 Tuple patterns 原句、ch10 where 约束主语规则）。
4. **tasks：通过**。三项均出自前三者；来源/验证齐备；无未决方案。
5. **场景覆盖：通过**。normal（构造/迭代/新鲜迭代器）、boundary（耗尽后行为、空区间、半消耗集合再迭代）、failure（E0901×3/E0902×2/E0903/E0904/E0811/E0812/E0816/E0501×2）三路俱备，非仅 happy path。
6. **无空章节：通过**。8 条 Requirement 均有行为增量。
7. **测试先行：通过**。task 2 负例注入先于注册表扩展（spec 层「会失败的目标测试」等价物，types-foundations 片先例）。
8. **负向断言：通过**。E0901 注入 sentinel + Rejected forms 示例段，非文字代替。
9. **完成度闭环：通过**。spec 层切片，三要素以机制中立语义文本落定（前 10 片同构先例）；静态分发（裁决 5）即代码生成侧承诺的规范表达。
10. **未决问题阻塞：通过**。五项裁决均已落定，无阻塞项。

**结论：通过（F1/F2 已修），进入 active。**

## 实现审查记录（welang-code-review，2026-09-04）

1. **规范符合性：通过**。机器提升零差异：ADDED 正文 8 Requirements / 27 Scenarios 逐字并入 1100-iterables.md（examples 节亦逐字）；MODIFIED 2 条与 0500-iteration.md 宿主块零差异（脚本切割比对 True×2）。zh 孪生 8/27 计数镜像、代码块 5/5 字节一致；ch5 zh 两块语义与 EN delta 一致（元组模式/`E0901`/`E0902` 表述对应）。示例只用已批准表面形式；D14 刷新四处（ch5/ch7 × EN/zh）与 What Changes 第 4、5 条逐一对应。
2. **验证诚实性：通过**。tasks 三项勾选均有实际运行记录：validate --strict（active 前后各一次）；负例注入先行且 FAIL 消息已录（task 2 验证行）；提升后全套机器验证（逐字 diff、zh 计数、全角冒号扫描零命中、ch5 场景继承比对——For statement 差异恰为正文段 + D11 披露的一处场景标题与内容，Range expression 仅正文段、5 场景逐字；比对基准为 git HEAD 原文）；docs_sync --check 18 对齐；validate --all --strict 终绿（"OK: 1 change(s) valid; registry clean; mode=strict"）。无「勾了没跑」项。
3. **测试先行证据：通过**。spec 层切片：未注册码注入 sentinel 先于注册表扩展失败（消息原文录于 task 2），与 types-foundations 片先例同构；本仓尚无 conformance 黄金用例（纯规范层）。
4. **诊断协议稳定：通过**。E0901–E0904 全局唯一且注册（81 条目，owner/requirement 字段解析核验：E0901→The for-loop protocol、E0902→The Range type、E0903→Iterable implementer obligations、E0904→The Iterable interface）；一码一消息全库扫描：新四码各自唯一消息串且与注册表 title 逐字一致（扫描中出现的多消息码 E0105/E0605/E0404/E0808/E0104/E0013/E0502 均为先前章节的旧文件尾缀变体，非本片触碰）；`--json` 协议不涉（无工具链产物）。
5. **单一权威：通过**。长期事实已入 docs/spec/（第 11 章双语、ch5 宿主、注册表）；变更目录仅存过程工件；EN 文件 CJK 行机器核验全部位于 Terminology 表中文列（14/9/16 行均在其表头之后）。
6. **红线复核：通过**。git status 恰为影响范围声明的 8 个路径（5 改 + 3 新），refr/ 不在其中；尚未提交；无越界改动。
7. **最小可信验证已跑：通过**。按 spec 层验证阶梯：validate --all --strict、registry clean、docs_sync --check（17→18）、全角冒号扫描、代码块字节比对、一码一消息扫描、CJK 滞留核验——全部通过。

**结论：通过，进入归档（complete）。**

## 归档记录（welang-archive-sync，2026-09-04）

- 变更目录移至 openspec/changes/archive/2026-09-04-iterable-protocols/；change.yaml status: complete → archived。
- 提升核验：docs/spec/1100-iterables.md（+zh）为本变更唯一权威；ch5 双宿主块已替换；diagnostics.toml 段位 E0900–E0999 认领 + E0901–E0904 四条目；docs_sync 文档对 17→18。
- 无待提升残留：变更目录内无长期事实滞留；无 ADR 触发（裁决记录随 proposal 归档）。
