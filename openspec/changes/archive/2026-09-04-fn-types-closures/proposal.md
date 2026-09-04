# Proposal: fn-types-closures

## Why

类型队列六片计划的第 6 片（末片）：函数类型与闭包。已落定的规范为本片留位的引用逐一兑现：

- ch9:44「Whether a payloaded variant's bare name is a first-class value is the function-types chapter's; until then … rejected with `E0704`」——一等构造器之问按章债必须在本片回答（批准或永久否决）
- ch7:86「Function types and every other type syntax do not exist yet; each arrives with its owning chapter through this chapter's amendment」+ ch7:108 场景（`fn(Int64) -> Int64` 以 E0105 拒绝、「the form arrives with its owning chapter」）——fn 类型文法按名指向本片
- ch6:43 声明面已备（`name: type` 参数、返回可省=无值、无参数上限），但 fn 名在值位置的类型化从未批准——本片定义
- ch2:101 keyword-led 增长句（「a keyword-led expression form enters only through a spec-layer change in its owning chapter」）——`fn` 引导的闭包完整形式循此进入；`|params| body` 短形式需本句扩展
- ch5:119 / ch7:234 / ch9:212 示例 pending 注释（组合子需闭包、Fn types 待其章、一等构造器待其章）——D14 刷新义务
- v0.8 §13.2–13.5（fn 型文法 + effect 段 + lambda 双语法）、§22 注（lambda 体 return 不穿透外层 fn）、§23

边界：ch11:24 组合子「need function values, arrive with their owning change」——本片提供函数值，组合子本身仍是其所属变更（非目标），其 E0816 场景文本不变更。

## What Changes

1. **新增第 12 章 `1200-fn-types.md`「Function types and closures」**（段位 E1000–E1099）：7 条 ADDED Requirements / 32 个 Scenarios——fn 类型（`fn(T1, ..., Tn) -> T`，返回必写、无值=`-> ()`、参数无上限随 ch6、单态、无 effect 段）、函数值（单态声明 fn 名是值、类型即签名、一致性 E0501、泛型 fn 名非值 E1004、方法值不批准）、完整闭包形式（`fn(params) -> type block`、参数随 ch6、返回可省、自含类型、与声明一字前瞻区分）、短闭包形式（`|params| body`、k≥1——零参唯一完整形式，`||` 系 ch1 逻辑或单 token；标注参自含处处合法、裸参需显式 fn 类型期望否则 E1001、体最大化、二元/一元操作数位须括号 E0105、语句首位 `|` 依 E0102 拒绝）、闭包体即函数体（block 值、return 从闭包返回不穿透、defer 于闭包体退出执行、E0401 语境含闭包体）、捕获纪律（闭包一律 gc 类别；gc 型绑定按引用活捕获；value 类别与基类型按拷贝快照、体内赋值以 E1003 拒；byres 禁捕获 E1002）、诊断段位。
2. **ch2 Expression skeleton MODIFIED**：keyword-led 句扩展——fn-types 章批准两类闭包形式（`fn` 引导完整形式为 keyword-led；`|params| body` 短形式同样坐落骨架之外、体最大化、自身不受后缀）；+1 场景「A closure form is used」，其余场景逐字继承。
3. **ch7 Type references MODIFIED**：类型引用枚举加入 fn 类型形式（文法归第 12 章）；「Unratified type syntax is rejected」场景标题保留、示例换为 effect 段形式 `fn(Int64) io -> Int64`（真实未批准文法）；+1 场景「A function type in an annotation slot」，其余场景逐字继承。
4. **ch9 Variant constructors MODIFIED**：一等构造器之问落定为永久否决——「not a first-class value — the fn-types chapter answered the question negatively」，构造只经调用形式、需要构造器函数时写闭包 `|r| Circle(r)`；场景 THEN 尾句同步；E0704 永久生效（注册表删码本就禁止，无残值问题）。
5. **注册表**：段位 E0900–E9999 的未认领尾部拆分为 E1000–E1099（owner 1200-fn-types）+ E1100–E9999（未认领）；新增 E1001–E1004 四条目（81→85）；E0401 description 扩展（闭包体是 return 合法的函数体语境）。
6. **示例 D14 刷新（EN+zh，披露）**：ch9 pending 块（一等构造器已答否——`let g = Circle` 转为 E0704 拒绝形式）+ ch9 示例引言；ch7 pending 块（Fn types 行兑现）；ch5 pending 块（「组合子需闭包」表述刷新）；ch11 pending 块「need function values」表述微调（若审定需要）。

## 裁决记录（用户 2026-09-04 四项，均采纳推荐）

1. **一等构造器不批准**：构造永远用调用形式 `Circle(1.0)`；需要构造器函数时写 `|r| Circle(r)`。E0704 永久生效；表面更小（P1/P5）；无构造器值代码生成。备选「批准」（裸名即 `fn(P1,...,Pk) -> SumName` 值）被否——E0704 触发面消失而条目不可删（9900:33 删码禁令），且 `.map(Circle)` 的便利由闭包等价覆盖。
2. **单一 gc 闭包**：闭包一律 gc 类别（引用语义、可存储可返回可逃逸）。捕获按 ch8 类别：gc 型绑定按引用（体内读写皆见活绑定，逃逸经追踪单元格健全）；value 类别与基类型按拷贝快照（创建时定格，体内赋值以 E1003 拒——局部绑定携带中间值）；byres 绑定禁止捕获（E1002——闭包按构造可越出创建点，句柄外逸破坏资源纪律的单一确定性释放点；内容先物化）。备选「双类别闭包」（非逃逸 byval 零成本 + 逃逸 gc）被否——逃逸分析非局部可判定，违 P1。
3. **短闭包严格语境推断**：裸参短闭包仅当所在位置有显式 fn 类型期望（绑定注解、形参声明类型、已声明返回位）才合法——参数自期望定型、返回自体定型；无期望以 E1001 拒。带标注参 `|x: Int64| x + 1` 自含类型、处处合法。备选 v0.8 宽松推断（从任意使用处）被否——推断非局部、错误远程报，与 ch6「parameter types are never inferred」立场冲突。
4. **值域=单态 fn 名 + 闭包**：无泛型子句的声明 fn 名是值，类型即其签名（不一致处走既有 E0501 一致性）；泛型 fn 名不是值（E1004，本片不批准实例化语法 `map<Int64>`）；方法值（`obj.method` 裸取）不批准——receiver 纪律留待后续修订，非目标。fn 型语法随此裁决成文：`fn(T1, ..., Tn) -> T`、返回必写（无值=`-> ()`）、参数无上限（随 ch6「zero or more」）、无 effect 段（v0.8 偏离：本规范未批准任何效应系统；若未来批准经第 12 章该条修订进入）。

## 目标与非目标

目标：

- 落地 ch7/ch9 指向本片的全部悬置引用，类型六片闭环
- fn 类型进入类型引用文法；函数值（单态 fn 名 + 闭包）成为一等值
- ch9 一等构造器之问的诚实永久回答（E0704 终身语义）
- 捕获纪律与 ch8 所有权类别的对接（gc 引用/value 拷贝/byres 禁止）

非目标：

- 迭代器组合子（map/filter 及急性族）——其所属变更，落地为 Iterator 默认方法
- 泛型 fn 实例化语法与泛型方法——ch10 自身修订（若需要）；ch10 zh:290「或随函数类型章」仅系揣测，不构成本片义务
- 方法值（`obj.method` 裸取不带调用）——receiver 纪律，后续修订
- 嵌套/局部 fn 声明——闭包覆盖局部函数需求；若需要经 ch6 修订
- 类型别名（v0.8 §13.6）——从未被承诺，独立变更
- effect 段与效应检查（v0.8 §13.2–13.4/E0503 效应义）——无已批准效应语义可挂靠
- 并发捕获规则（task 闭包的 Shared 包裹等，v0.8 §34）——并发章节
- 柯里化与部分应用；fn 型变体（no-variance 立场由 ch10 延伸覆盖）

## 影响层

`spec`：新增 1200-fn-types.md（+zh）；修改 0200-grammar.md（+zh，1 条 MODIFIED）、0700-types.md（+zh，1 条 MODIFIED）、0900-sum-types.md（+zh，1 条 MODIFIED）；diagnostics.toml（段位拆分 + 4 条目 + E0401 描述扩展）；ch5/ch7/ch9/ch11 示例刷新（EN+zh）。

## 影响范围

- docs/spec/1200-fn-types.md（新增）、1200-fn-types.zh.md（新增）
- docs/spec/0200-grammar.md / .zh.md（Expression skeleton 1 条 MODIFIED）
- docs/spec/0700-types.md / .zh.md（Type references 1 条 MODIFIED + 示例刷新）
- docs/spec/0900-sum-types.md / .zh.md（Variant constructors 1 条 MODIFIED + 示例刷新）
- docs/spec/0500-iteration.md / .zh.md（仅示例 pending 注释刷新）
- docs/spec/1100-iterables.md / .zh.md（仅示例 pending 注释微调，若审定需要）
- docs/spec/diagnostics.toml（段位拆分 + E1001–E1004 + E0401 描述扩展）
- docs_sync 文档对 18→19

## 审计记录（welang-spec-impact-audit，2026-09-04）

1. **问题真实性：通过**。Why 各条均锚定活性文本：ch9:44（一等构造器悬置句）、ch7:86（fn 型文法悬置句）、ch7:108（E0105 fn 型场景）、ch6:43（声明面已备）、ch2:101（keyword-led 增长句）、ch5:119/ch7:234/ch9:212（示例 pending 注释）、ch11:24/44（组合子边界——本片供给函数值、组合子仍归其变更）；v0.8 参照固定为 refr/spec-0.8.md §13.2–13.5（fn 型 + effect 段 + lambda 双语法）、§22 注（lambda 体 return 不穿透，原文行 1256）、§23、§34（并发捕获——非目标）。
2. **影响层声明：通过**。change.yaml `layers: [spec]` 与 proposal 影响层一致，纯规范层。
3. **规范增量范围：通过**。ADDED 7（新章）+ MODIFIED 3（Expression skeleton/Type references/Variant constructors——标题与宿主 0200-grammar.md:99、0700-types.md:85、0900-sum-types.md:43 逐一相符）；新码 E1001–E1004 全局唯一（docs/spec 全文除未认领段位行外零 E10xx 诊断用法）；引用既有码 E0102/E0105/E0501/E0605/E0704/E0404 的消息串与注册表一致（E0102/E0704 逐字；E0105/E0501 沿用括号形，先例既立）；一码一消息（E1001 两处、E1002–E1004 各处消息串相同）。
4. **原则一致性：通过**。严格语境推断与单一 gc 闭包服务 P1（无跨句推断、无逃逸分析）；D5 论证 `|` 三位置不存在静默误析路径（P4）；返回类型必写（P5）；E0501 承接签名一致性、no-variance 延伸（ch10 立场）。对 v0.8 的偏离（effect 段不继承、一等构造器否决、宽松推断否决）均在裁决记录中论证；无机制/依赖级决策，不触发 ADR。
5. **参考基线固定：通过**。refr/spec-0.8.md 固定文件与节号（§13.2–13.5、§22 注、§23、§34）。
6. **验收边界：通过**。目标可机械判定（文件清单、计数 7/31 与 3/15、落地项、docs_sync 19）；非目标排除组合子/泛型实例化与泛型方法/方法值/嵌套 fn/类型别名/effect 段/并发捕获/柯里化。
7. **粒度：通过**。单一规范层垂直切片：一章 + 三宿主块 + 注册表，无跨层依赖。

**结论：通过，进入 ready。**

## 审查记录（welang-change-review，2026-09-04）

1. **proposal 职责边界：通过**。Why/What Changes 为黑盒问题与规范变更，无任务清单；裁决记录系决策记录（先例既立）；影响范围与 What Changes 逐项对账一致。
2. **spec 增量黑盒 + BCP-14：通过（F2 修正后）**。原稿 R2「agrees only at the exactly matching signature」与 R3「are written, never inferred」两处承重义务缺 RFC 2119 关键字——已补 MUST；R1 REQUIRED、R4 MUST×2（标注参与期望一致、操作数位须括号）、R6 MUST NOT 齐备；全部增量描述外部可观察行为。
3. **design 唯一最小路径与被拒替代：通过（F1 修正后）**。D1–D9 均给出唯一最小路径；被拒替代（一等构造器批准、双类别闭包、宽松推断、effect 段继承）均记录于裁决记录。**F1（真缺陷）**：原稿零参短闭包写作 `||`——`||` 是 ch1 逻辑或算子的单 token（maximal munch），`|| body` 永远吞为逻辑或，零参短闭包不可表达。修正：短形式限 k≥1（参数表以 name 开列，`| |` 空格变体同样于参数首位遇 `|` 即 E0105），零参闭包唯一合法形为完整形式 `fn() -> ...`；R4 增场景「No zero-parameter short form exists」（32 场景），examples.md 四处 `||` 改完整形式 + 拒绝块新增 `let z = || ready()` → E0105 例，design D5 增第 4 点记录论证。与 sum-types 片 arm-逗号缺陷同类（词法层碰撞在审查期拦截，先例）。
4. **tasks 来源验证：通过**。三项任务均可溯源至 What Changes/目标/dev-process/design；验证步骤机器可判定。
5. **场景覆盖：通过**。normal（绑定/调用/传参/逃逸）、boundary（零参、无值、拷贝 vs 活捕获、括号调用、零参短形不存在）、failure（E1001×2 语境、E1002、E1003、E1004、E0105×4、E0102、E0501×2、E0704）三轴齐备。
6. **无空章节：通过**。7 条 Requirement 全部承重（无凑数节）。
7. **测试先行：通过**。任务 2 负例注入先行（未注册码 E1001 → 校验报错 → 还原），与 interfaces/iterables 片先例一致。
8. **负向断言：通过**。每条新码至少一个拒绝场景；拒绝形式示例带注册表消息串逐字引导。
9. **完成度闭环：通过**。spec 层变更，类型检查/代码生成/运行时三要素以语义规则文法化承载（10 片先例）；无工具链断言悬置。
10. **未决问题：无阻塞**。四项裁决全部落地成文；ch11 组合子边界（E0816 场景文本不变更）维持。

**结论：通过（F1/F2 已修正并复验 validate --strict 绿），进入 active。**

## 实现审查记录（welang-code-review，2026-09-04）

1. **规范符合性：通过**。提升逐字（ADDED 7/7、MODIFIED 3/3 机器零差异）；zh 孪生镜像（7 Requirements / 32 Scenarios / 5 代码块字节一致）；宿主差异 vs git HEAD 恰为 D8 披露项（ch2 +1 句 +1 场景；ch7 段内 fn 型句 + 拒绝句扩展 + 1 新场景 + 未批准场景示例换文；ch9 悬置句→永久句 + E0704 尾句替换）；ch11/ch5 需求文本零改动（仅示例注释刷新，scope 纪律）。
2. **验证诚实性：通过**。任务 1–3 全部验证本会话真实运行：validate --strict / --all --strict 绿；机器计数 47（32+15）；docs_sync 18→19；冒号扫描 0；直角引号扫描 0；注册表 85 码 12 段、E1001–E1004 requirement 字段解析到 1200-fn-types.md 标题、E0401 仅动 description。审查后勾选 tasks.md。
3. **测试先行证据：通过**。注入负例先于注册表扩展：0200-grammar.md 临时使用 `E1001:` → `FAIL: ... diagnostic usage 'E1001:' has no registry entry in docs/spec/diagnostics.toml`（消息原文），git checkout 还原后复绿；随后才扩展注册表（interfaces/iterables 片先例）。
4. **诊断协议稳定：通过**。E1001–E1004 全局唯一（段位 E1000–E1099 owner 1200-fn-types、未认领尾收窄 E1100–E9999）；一码一消息语义核验 PASS（实现期发现两处偏差并修正：R6 散文 `E1002`： 后随理由非注册表消息→改为先消息后理由；示例折行 "fn type"→"function type" 与注册表逐字）；E1002 示例跨代码行折行为 ch11 房屋惯例同款。
5. **单一权威：通过**。第 12 章条文、示例、术语已落 docs/spec/（EN+zh）；变更目录仅存提案/设计/任务/增量（归档时随目录入 archive）；EN 工件 CJK 仅术语表中文列（ch11 先例）。
6. **红线复核：通过**。git status 恰为 12 文件 + 变更目录，无 refr/、无越界路径；提交信息将无署名 trailer。
7. **最小可信验证：通过**。validate --all --strict、docs_sync --check（19 对）、逐字/镜像/继承/注册表/一码一消息机器套件全绿。

实现期过程缺陷披露（均当场拦截并修复）：首次宿主拼接块界正则 `^##+ ` 误吞 `#### Scenario:` 致仅段落替换——场景计数打印 0 立即暴露，修正正则（`^(## |### )`）重拼并以 git HEAD 机器比对确认；与 sum-types 片拼接缺陷同类教训，建议后续片拼接脚本固化正确块界。

**结论：通过，进入 complete。**

## 归档记录（welang-archive-sync，2026-09-04）

- 规范提升：docs/spec/1200-fn-types.md + .zh.md（新章，7R/32S + 示例 + 术语）；0200-grammar / 0700-types / 0900-sum-types 三宿主 MODIFIED ×2 语言落地（逐字零差异）；0500/0700/0900/1100 示例 D14 刷新 ×2 语言；diagnostics.toml 段位 E1000–E1099（owner 1200-fn-types）+ E1100–E9999（unclaimed）+ E1001–E1004（81→85）+ E0401 描述扩展；docs_sync 18→19。
- 决策提升：不触发 ADR（审计记录第 4 点：无机制/依赖级决策；四项裁决与被拒替代完整留存于本文件裁决记录，v0.8 偏离清单在 design D4）。
- 状态提升：docs/roadmap 不存在，按技能约定不创建，本记录留档。
- 归档移动：openspec/changes/fn-types-closures → openspec/changes/archive/2026-09-04-fn-types-closures，status → archived。
- 复验：validate --all --strict 绿 + docs_sync 19 对（归档后终验见提交前检查）。
