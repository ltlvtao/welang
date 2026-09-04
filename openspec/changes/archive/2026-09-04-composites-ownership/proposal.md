# Proposal: composites-ownership

## Why

1. 类型体系欠账第二批到期：ch3「if-else 臂一致规则由类型章节批准」、ch6「值级丢弃检查延后至类型层」、ch7 R5「tuple 类型文法随复合章节修订进入」、ch7 R7「无值块行为随 unit 类型由复合章节批准」、ch4「tuple 模式随配对修订进入」、ch2「成员访问字段/方法判定由类型章节解析」——六处前向引用等待本片。
2. v0.8 素材固定：§8 四类所有权总览（gc 默认 / byval / byres / newtype）、§10 record 规则（byval 字段类别约束、更新表达式 `with &`、字段不可赋值）、§13.1 元组（上限 8）、§15 newtype（零开销、双向无隐式转换）、§9.3 Unit（`()` 唯一值、空块类型 Unit）。
3. 六片序列第二片（裁决 1）：复合+所有权。后端语言的核心数据形态（业务对象/DTO/资源/强类型 ID）全部落位于此片；sum、接口、泛型、可迭代、函数类型五片都依赖本片的概念地基。

## 目标与非目标

目标：

1. 建第 8 章 `docs/spec/0800-composites.md`（+zh 孪生）：四类所有权、record 声明与构造/更新表达式、字段访问、值记录、资源记录、newtype、元组类型与表达式、Unit、元组模式与解构、值丢弃检查、if 臂一致、局部遮蔽。
2. 注册表：认领段位 E0600–E0699（owner 0800-composites），分配 E0601–E0606 六码；扩展 E0501 与 E0404 的 description 枚举；未认领段收窄为 E0700–E9999。
3. 七处宿主修订：ch1 Keywords（+`record byval byres newtype with` 五词，破坏性变更如实记录）、ch2 Line-joining（构造表达式花括号按括号处理）、ch2 Statements（绑定语句名字位接受元组模式；字段赋值显式关死）、ch2 Expression skeleton（primary += 构造表达式；圆括号产生式 += 元组与 unit）、ch3 If（臂一致规则落地指向第 8 章）、ch4 Pattern set（+= 元组模式，变体模式仍留 sum 片）、ch6 File structure（顶层项 += record/newtype）、ch6 Function bodies（隐式丢弃改为严格丢弃规则）、ch7 Type references（+= 元组类型与 unit 类型）、ch7 Block value typing（无值块类型 = `()`）——共 10 条 MODIFIED。

非目标：

1. sum 类型、变体模式、Never 底类型、穷尽性机制 → sum+match 修订片（ch4 配对修订的另一半）。
2. 接口、impl、方法（`mut self`、inherent method）、derives（Eq/Hash）、tuple 实现接口的义务执行 → 接口+泛型片；本片只陈述「元组不实现接口」的形状义务。
3. byres 的 Releasable 实现机制、释放操作、`scope resource` → resource 章节；本片只立「资源记录必须实现 Releasable」的义务与类别语义。
4. `Ref`/`Shared`/`WeakRef`/`region` 三层引用模型与闭包捕获检查 → 并发章节；本片的「所有权」是四类别，不是那套引用体系。
5. List 等集合类型与下标语法 `expr[expr]` → collections 章节。
6. 函数类型、lambda、闭包 → 函数类型片。
7. 泛型应用 `List<T>` → 接口+泛型片。
8. record/newtype 的内存布局与 GC 实现细节 → 实现层，不入规范。
9. 模块限定名（`module.Name`）是否解析到已声明类型 → 模块系统章节。

## What Changes

1. `specs/composites/spec.md`：14 条 ADDED Requirements / 39 Scenarios——Ownership categories / Record declarations / Record construction expressions / Update expressions / Field access / Value records / Resource records / Newtype declarations / Tuple types and expressions / The unit type / Tuple patterns and destructuring / Value discard / If arm agreement / Local shadowing。
2. 宿主修订（7 个能力文件，10 条 MODIFIED / 43 Scenarios）：`specs/lexical/spec.md`（1）、`specs/grammar/spec.md`（3）、`specs/control-flow/spec.md`（1）、`specs/match/spec.md`（1）、`specs/declarations/spec.md`（2）、`specs/types/spec.md`（2）。
3. `docs/spec/diagnostics.toml`：段位 E0600–E0699 认领；E0601–E0606 六条目；E0501 description 枚举扩展（字段值、if 臂、元组元素）；E0404 description 扩展（同一模式内重复绑定）；E0012 description 扩展（record 字段名 camelCase）。
4. `specs/composites/examples.md`：示例节（非权威，提升时逐字并入第 8 章）。

## 影响层

- spec：仅规范层（一个新章增量 + 六处宿主修订 + 注册表段位与条目）。

## 影响范围

- 新章 `docs/spec/0800-composites.md`（+zh）；`docs/spec/0100-lexical.md`、`0200-grammar.md`、`0300-control-flow.md`、`0400-match.md`、`0600-declarations.md`、`0700-types.md`（各含 MODIFIED 落地）；`docs/spec/diagnostics.toml`。

## 裁决记录

2026-09-04 用户裁决七项（1–5 于预告问答，6–7 于选项确认，均含推荐采纳）：

1. **四类所有权全继承**——gc（默认，无修饰符）/ byval（按值复制）/ byres（资源，显式生命周期）/ newtype（零开销包装）；类别决定传递、共享、生命周期规则。
2. **更新表达式 `with &` 入本片**——record 的唯一「修改」语法随本片批准（否决我「延后至引用语义」的建议）；配套设计约束：byres record 不适用更新表达式（资源有身份，复制字段即复制句柄）。
3. **丢弃检查取严格**——非 Unit 值不得静默丢弃：作语句、作无返回值函数体末项、作控制形式体的非 Unit 末项，一律报错，必须 `let _ = expr` 显式丢弃；Unit 自由丢弃。
4. **局部遮蔽允许**——后绑定胜出，旧绑定失名（不是改写）；参数可被遮蔽；嵌套遮蔽随作用域模型成立；名字解析 = 就近作用域。
5. **元组上限 8 继承**——超出报错并建议改 record。
6. （裁决 3 之追问确认）严格度选项三选一取「严格：必须显式丢弃」。
7. （裁决 4 之追问确认）遮蔽选项二选一取「允许：后绑定胜出」。

## 审计记录

2026-09-04，状态 candidate → ready 前置审计（welang-spec-impact-audit 七点）：

1. **问题真实性：确认。** 本片偿付的悬置引用逐一存在：ch3:5「if-else 臂一致由类型章节批准」、ch6:61「值级丢弃检查是 types chapter 的」、ch7 R5「tuple 类型文法随归属章节修订进入」、ch7 R7「无值块行为随 unit 由复合章节批准」、ch4:46「tuple 模式随配对修订进入」、ch2:86「成员访问字段/方法判定由类型章节解析」——六处全部落地。遗留引用仍有主：ch3:62 Never 与 ch4 变体模式/穷尽性 → sum 片；ch5 迭代协议/Range → 可迭代协议片；ch2:86 方法侧 → 接口片。无无主引用。
2. **影响层准确性：确认。** layers `[spec]` 与实际文件集一致（七宿主章节修订 + 新章 + 注册表，全规范层）。
3. **增量范围与完整性：确认。** ADDED 14/39 + MODIFIED 10/43 = 82 场景，每 Requirement ≥ 1；E0601–E0606 六码全部有消息串场景；E0501/E0404/E0105 复用措辞与宿主一致；E0501 的臂一致/字段/元组元素扩展与其「一码管所有一致位置」裁决立场一致。关键词核验推翻起草假设（五词不在 ch1 表内）→ 以 ch1 MODIFIED 落地，走其预授权机制并如实记录破坏性。MODIFIED 场景全量继承已核（ch2 Statements 起草丢失 2 原场景，审查前发现并修复，已列入 task 1 验证项）。
4. **原则一致性：确认。** 类别即声明时策略选择（局部可判定，无推导）；模式为元组解构唯一通道（单一教义）；newtype 双向无隐式转换直接由 E0501 覆盖；严格丢弃 = 「绝不静默」教义对值落地处的延伸；遮蔽就近解析保 P1。无 ADR 触发，权衡入 design D1–D13。
5. **参考基线固定：确认。** v0.8 §8/§9.3/§10/§13.1/§15/§21.4 素材已钉。偏差披露：v0.8 码位（E0602/E0260/E0261→E0501/E0262/E0251/E0233/E0242）不继承，新分配 E06xx；**严格丢弃检查超出 v0.8 基线**（v0.8 无值级丢弃检查——裁决 3 为有意加严，非继承）；`.0` 位置访问不引入（v0.8 亦无）；`with &` 的 byres 排除为 v0.8 未涵盖的补全。
6. **验收标准可机械判定：确认。** tasks.md 三项全机检：validate --strict、计数清点、注入负例 + 还原、逐字 diff（ADDED 14/14 + MODIFIED 10/10 对宿主落地）、MODIFIED 场景集与宿主原场景集比对、zh 镜像结构计数、ASCII 冒号扫描、git status 范围核对。
7. **粒度与耦合：确认。** 六片之二；依赖仅 ch1–ch7（已批准），向后续片单向输出概念（sum 片的变体模式嵌套于本片元组模式规则之上、接口片的方法修改互补于本片字段不可变、resource 章兑现 byres 义务）。E0600–E0699 段位与既有段零交叠。

## 审查记录

2026-09-04，状态 ready → active 前置审查（welang-change-review 十点），发现 2 项、修复 2 项，另 2 项考量后放行：

- **F1（文法精度，已修）**：构造头原文「Name MUST name a record type of the enclosing module or an imported module」写法含混——裸标识符无法跨模块构造 pub record。已修：头 = 命名类型引用（PascalCase 标识符或 `module.Name`），R3/R4/ch2 Expression skeleton 三处同步。
- **F2（命名规则错挂，已修）**：R2 原称字段名「lowercase per ch1」——ch1 变量规则是 camelCase（E0012）且未列举字段绑定种类。已修：字段名挂 E0012 变量规则，注册表 E0012 description 扩展（与 E0501/E0404 同例），What Changes 第 3 条补记。
- **考量放行 1**：ch7 R3「every position」枚举（绑定/实参/返回三处）不随本片扩展——ch8 各 Requirement 在自己权威位置陈述各自的 E0501 义务，ch7 三例皆为真命题、无一变假；注册表 description 扩展承担操作面统一。避免第三条 ch7 MODIFIED 的冗余。
- **考量放行 2**：示例中 `{ ... }`/`= ...` 占位符——与 ch7 已批准场景的既有先例一致（非权威示例节自免责），不引入新文法声明。

十点结论：

1. **六 artifact 完整**：change.yaml / proposal / 8 个 spec 增量文件 / examples / design / tasks 齐备，validate --strict 通过。
2. **需求可测试性**：82 场景（ADDED 39 + MODIFIED 43）均为可判定 WHEN/THEN；六新码 + 三扩展码消息串钉死。
3. **BCP-14 在场**：ADDED 12 个 MUST 行；MODIFIED 义务句保留宿主原有 MUST（ch2×3、ch3×1、ch1×1、ch6×2、ch7×1——match Pattern set 为陈述式定义章句，与宿主原文一致零 MUST，非倒退）。
4. **一致性**：E0501 复用与 ch7 措辞一致；E0404 扩展经场景消息串对齐；ch1 命名（E0011 预授权 record/newtype、E0012 扩展字段）；`with` 零续行集交互（构造花括号内无意义换行）；五新词走 ch1 预授权机制。
5. **无范围蔓延**：10 条 MODIFIED 全部对应 What Changes 清单；场景继承机器核查——宿主原场景零意外丢失（唯一替换是 ch7「无值块无类型→有 unit 类型」，即本片变更本体）。
6. **非目标边界**：9 项延后各有归属（sum/接口/resource/并发/collections/函数类型/泛型/实现层/模块系统），无悬空。
7. **术语单一权威**：四类所有权、更新表达式、丢弃规则、遮蔽规则均唯一落位第 8 章；「类型一致位置」仍在 ch7 总纲、ch8 各条自持。
8. **前向引用消解**：审计点 1 六处偿付在本增量内兑现；遗留路由不变。
9. **注册表卫生**：E0600–E0699 认领 + 六码 + 三 description 扩展（E0501/E0404/E0012）方案与既有段位零交叠；注入哨兵 E0700 在未认领段。
10. **归档可追溯**：tasks.md 三项均带来源/验证行；「MODIFIED 场景集与宿主原场景集比对」已固化为标准验证项（ch2 Statements 起草事故的机制化回应）。

结论：通过，进入 active。

## 实现审查记录

2026-09-04，状态 active 实现完成 → 归档前置代码审查（welang-code-review 七点），实现内容：docs/spec/0800-composites.md（14R/39S + 示例节 + 术语表）+ .zh.md 孪生、六宿主章节 ×2 的 10 条 MODIFIED 落地、diagnostics.toml（段位 + 六条目 + 三 description 扩展，37 条目）：

1. **逐字提升核验：通过。** 机器 diff：第 8 章 14 条 ADDED Requirements 与 specs/composites/spec.md 逐字一致（14/14）；10 条 MODIFIED 与六宿主 EN 章节逐字一致（10/10）；`## Examples (non-authoritative)` 与 specs/composites/examples.md 逐字一致。
2. **场景继承核验：通过。** 对照 git HEAD 宿主原场景集：10 条 MODIFIED 零意外丢失（唯一替换 = ch7「无值块没有类型→无值块具有 unit 类型」，即变更本体）；非修改 Requirements 逐条字节比对未变；新增 9 场景（34+9=43 ✓）。
3. **注册表核验：通过。** E0600–E0699 认领、E0601–E0606 六条目 requirement 字段全部解析到本章 Requirement 标题；E0501/E0404/E0012 description 扩展后既有章节用法零改动（回归通过）；注入哨兵先例复用（E0700 FAIL 消息已记录于 task 2 验证）；validate --all --strict 输出 registry clean。
4. **zh 孪生核验：通过。** 九个章节对全部 EN/zh 结构计数一致（含新 0800 对 14R/39S）；诊断消息串保留英文原文，全角冒号扫描 0 命中；代码块与 EN 逐字相同；docs_sync 14→15 对全对齐。
5. **范围核验：通过。** git status 恰为计划文件集（六宿主 ×2 + 0800 双件 + 注册表 + 变更目录），无计划外改动。
6. **一致性抽查：通过。** zh 修订措辞与既有宿主译文连续（匹配对象（scrutinee）、或模式、E0404/E0105/E0012 消息串沿用既有译法）；ch8 zh 对 EN 的五处「grounding/deferred」偿付句翻译无遗漏。
7. **可追溯核验：通过。** tasks.md 三项全勾，每项带来源/验证行；验证行与实际执行的机器核验一一对应，无凭空声称。

结论：通过，进入归档。

## 归档记录

2026-09-04 归档。实现审查七点通过后执行 welang-archive-sync：

- 变更目录整体移入 openspec/changes/archive/2026-09-04-composites-ownership/。
- change.yaml status: active → archived。
- 提升物已就位于 docs/spec/：0800-composites.md（+zh）、六宿主章节（+zh）10 条 MODIFIED、diagnostics.toml（37 条目）。
- 遗留路由（非本片范围，各有归属）：ch3 Never 与 ch4 变体模式/穷尽性 → sum+match 修订片；ch5 迭代协议/Range → 可迭代协议片；ch2 方法侧判定、元组接口义务、derives → 接口+泛型片；byres Releasable 机制 → resource 章；闭包捕获 → 函数类型片/并发章。
