# Proposal: interfaces-generics

## Why

类型队列六片计划的第 4 片：接口与泛型——类型系统的抽象层。前三片（基础 331cf44、复合+所有权 b96331f、sum+match dc67237）落定的规范到处为本片留位，本次逐一兑现：

- ch2:101「成员是字段还是方法由类型章解决」——Expression skeleton 的成员解析悬置
- ch2 赋值族「更丰富的赋值目标由其所属章批准」——接收者字段赋值一目
- ch8:57 资源 record「其字段只经资源章与接口章的机制变化」
- ch8:76 字段访问「方法及其他访问形式随接口章到来」「全语言无字段赋值」句的唯一例外
- ch8:123 newtype「derives 随接口章到来」
- ch8:142 「元组不实现接口；该义务的机制是接口章的」
- ch1 运算符表「与嵌套泛型闭括号的交互（需分隔）由泛型章规定」
- ch7 类型引用「泛型应用与其他类型句法尚不存在」、示例 pending 块的 `List<T>` 行
- ch8/ch9 示例 pending 注释（derives、Dyn）

## What Changes

1. **新增第 10 章 `1000-interfaces.md`「Interfaces and generics」**（段位 E0800–E0899）：16 条 ADDED Requirements / 72 个 Scenarios——接口声明、关联类型、impl 声明（含泛型 impl、孤儿规则、头类型名义性）、方法接收者（self/mut self）、接收者字段赋值（唯一字段赋值形式）、固有 impl、成员名字解析（落地 ch2:101 与 ch8:76）、默认方法、Dyn 值（仅显式构造）、derives（Eq/Hash/Show）、泛型参数声明（≤8、不透明性）、泛型类型引用与推断（实参统一 + 显式形式）、where 子句、无型变无子类型（长期）、无运算符重载（长期）、诊断段位。
2. **ch1 Keywords MODIFIED**：+`interface impl where derives`（25→29 词，破坏性变更依例记录；`mut` 预留位由 `mut self` 兑付，仅补说明句）+1 场景。
3. **ch2 MODIFIED ×2**：Statements 赋值族开接收者字段一目（`self.field = expr`，场景说明更新）；Expression skeleton 成员解析指针落至第 10 章。
4. **ch6 MODIFIED ×2**：File structure 顶层项 +接口声明与 impl 块；Function declarations 泛型子句与接收者豁免（裸首参数）指针 +1 场景。
5. **ch7 Type references MODIFIED**：命名类型 + 泛型应用 `Name<T...>` 与 `Dyn<Interface>`；「未批准句法」场景收窄至函数类型 +2 场景。
6. **ch8 MODIFIED ×4**：Record declarations（泛型子句 + derives 子句 + 实例化检查句 +1 场景）；Update expressions（资源字段变化机制指针落地）；Field access（未知成员迁 E0816、无字段赋值句开例外）；Newtype declarations（derives 落地 +1 场景）。
7. **ch9 Sum declarations MODIFIED**：泛型子句 + derives 子句 + 实例化检查句 +1 场景。
8. **注册表**：段位 `E0800–E0899` 认领（owner `1000-interfaces`）；新条目 E0801–E0831 共 31 条；未认领段收窄为 `E0900–E9999`。
9. **示例**：第 10 章 `## Examples (non-authoritative)`（归档时逐字并入双语章）；ch7/ch8/ch9 示例 pending 注释随本片刷新（EN+zh，实现审查记录披露）。

## 裁决记录（用户 2026-09-04 四项，均采纳推荐）

1. **变异通道：`self.field = expr` 仅限 mut self 方法体内**——gc 原地变更、byres 允许、byval 禁 mut self（E0812）；ch8「全语言无字段赋值」句修订为指向本例外，方法体外仍 E0105。补落推论：mut self 方法经不可变绑定可调用（绑定不可变管重绑定、不管对象状态）。
2. **关联类型本片纳入**——上限 4、声明无上界、impl 绑定先于方法、禁 Dyn 装箱、where 等式约束 RHS 具体（v0.8 §12.1 全套规则）。
3. **derives 目标 Eq/Hash/Show**——`==` 保持仅基础类型，组合类型相等一律生成方法 `.equals()`；Encodable/Decodable 延后（JSON+容器未批准）、Shareable 归并发章。
4. **Dyn 构造仅显式 `Dyn<Interface>(expr)`**——单一形式零推断；v0.8 的 `Dyn(expr)` 上下文推断与 E0722 不继承。

既有长期立场随本章成文：无运算符重载（D12）、无型变无子类型（D10）、扩展方法否决（D11）、`self` 非关键字（D2）、`Dyn` 非关键字（D4）。

## 目标与非目标

目标：

- 落地接口/impl/接收者/成员解析/默认方法/Dyn/derives 的完整表面语法与类型规则，兑现上列全部前向引用
- 落地泛型参数（fn/record/sum/interface/impl 五处声明位）、where 约束、调用点推断（实参统一 + 显式形式）
- 注册表新段位 E0800–E0899 与 31 条新码，第 10 章双语成对入库

非目标：

- Encodable/Decodable——随 JSON 变更（容器与 JSON 表示未批准）
- Shareable——归并发章
- 泛型方法（含方法显式类型实参形式）——无 v0.8 依据，E0826 规则先批、触发面随其后变更
- 接口级 where 子句、泛型 newtype——同上延后
- typealias——无归属章，另行变更
- 函数类型/一等变体构造器（E0704 悬置的延续）——第 6 片
- effect 集合与签名 effect 检查（v0.8 E0272）——effects 章不存在，不继承
- 扩展方法（否决，非延后）、运算符重载（否决，长期）
- 编译器/工具链代码——纯规范层变更

## 影响层

- spec（`layers: [spec]`）——仅规范与注册表，无编译器/工具链代码

## 影响范围

- `docs/spec/1000-interfaces.md` + `.zh.md`（新增一对，docs_sync 16→17 对）
- `docs/spec/0100-lexical.md`、`0200-grammar.md`、`0600-declarations.md`、`0700-types.md`、`0800-composites.md`、`0900-sum-types.md` 各含 `.zh.md`（MODIFIED 落地 ×2）
- `docs/spec/diagnostics.toml`（46→77 条）
- `openspec/changes/interfaces-generics/`（本变更工件）

## 审计记录（welang-spec-impact-audit，2026-09-04）

1. **问题真实性：通过**——Why 逐条引用 live 规范前向引用位（ch2:101 成员解析、ch2 赋值族、ch8:57/76/123/142、ch1 运算符表嵌套闭括号指针、ch7 类型引用与示例 pending、ch8/ch9 示例 pending），全部可对 `docs/spec/` 机械定位；对应 refr/spec-0.8.md §12–§20。
2. **影响层声明：通过**——change.yaml `layers: [spec]` 与 proposal 影响层一致；无编译器/工具链代码，边界明示。
3. **规范增量范围：通过**——ADDED 16（第 10 章）/ MODIFIED 11（ch1×1、ch2×2、ch6×2、ch7×1、ch8×4、ch9×1）逐一列于 What Changes；新码 E0801–E0831 共 31 条，段位 E0800–E0899 为注册表未认领区，全仓（含全部 active 变更——仅本变更）无冲突；validate --strict 绿；增量内 E 码引用与分配清单双向闭合（机器核对：31/31 用尽、无游离码，E0800/E0832/E0899 仅作段位边界）。
4. **原则一致性：通过**——P1 局部可判定：推断单向（实参统一 + 显式形式，无期望类型推断）、Dyn 单形式零推断，均为原则正向设计而非突破；P4 无歧义：`+` 约束连接符处于与表达式位不相交的文法位、嵌套闭括号 `> >` 分隔（兑现 ch1 指针）；零软关键字保持：`self`/`Dyn` 非关键字、`derives` 为完整关键字（合法类别）；无新隐式转换（实现接口不产生可代性，E0501 保持）；D15（固有 vs 默认同名改拒绝）系对 v0.8 的刻意收紧，已在 design 与 Requirement 文本披露，非原则突破、无需 ADR。完成度闭环按既有片先例：本片定表面语法、类型规则与派生方法的签名与要求，生成体运行时行为归运行时（编译器未开工，与前四片同形）。
5. **参考基线固定：通过**——refr/spec-0.8.md §12、§12.1–§12.6、§14、§16、§17–§20，v0.8 版本固定；未确认草案未当既成规范（v0.8 码号一律不继承，D 拒绝清单末条）。
6. **验收边界：通过**——目标可机械判定（validate --strict、ADDED/MODIFIED 与宿主逐字 diff、场景继承机器比对、docs_sync 16→17、注册表 46→77）；非目标逐项指明去向（JSON 变更/并发章/第 6 片/后续修订），蔓延面已封。
7. **粒度：通过**——spec 单层、单一垂直单元：接口-泛型产生式网格不可再拆（拆则同变更跨章互改，D1 论证）；16 条 Requirement 为该网格的最小完备表面。

结论：**通过，进入 ready。**

## 审查记录（welang-change-review，2026-09-04）

1. **proposal 职责：通过**——只讲黑盒缺口（前向引用清单）与目标；What Changes 为增量范围清单（规范增量范围审计项的载体），无实现细节、无任务清单混入。
2. **spec 增量职责：通过**——全部为触发/输入/结果/失败路径（WHEN/THEN）；诊断段位条目命名注册表与 owner 系全章既有惯例（ch4 Match 同形）；MUST/MUST NOT 用于强制项，MAY（derives 子句、默认方法、impl 省略默认方法）均带默认语义（缺省即不用/继承默认）。
3. **design 职责：通过**——D1–D22 唯一最小路径，拒绝方案清单 12 条各带理由；引用精确到章/条/码（ch1 运算符表指针、ch8:57/76/123/142、E0601/E0816 复用关系）；与增量无矛盾（D15 收紧、D18 实例化检查、D22 默认体位置均有对应 Requirement 条文）。
4. **tasks 职责：通过**——三任务全部来源于 proposal/design；各带来源与验证（命令+预期）；无 deferred/non-goal 勾选项。
5. **场景覆盖：通过**——normal（解析/调用/推断/派生）、boundary（上限 8/4、嵌套闭括号、空推断）、failure-degradation（31 码各有触发场景；变异三类别分野；孤儿/重叠/冲突三向）覆盖齐备，无仅 happy path 的 Requirement。
6. **无空章节：通过**——16 条 Requirement 均有行为增量；「无型变」「无运算符重载」为带拒绝场景的长期立场条目（v0.8 立场成文），非包装层。
7. **测试先行：不适用（按前例）**——纯 spec 层（layers: [spec]），编译器未开工，无目标测试可先行；验证阶梯为 validate/docs_sync/逐字 diff（与基础、复合、sum 三片同形）。
8. **负向断言：通过**——task 2 哨兵注入（未注册码 E0801 → 校验必败，消息原文记录）为真实负向验证；task 1 场景继承机器比对、task 3 逐字 diff 均为机械负向检查。
9. **完成度闭环：通过（按前例）**——类型检查三要素中类型规则完整（成员解析/推断/约束/冲突均有条文与码）；代码生成与运行时在编译器未开工阶段按四片先例处理：派生方法签名与要求入规范、生成体运行时行为显式归运行时（Requirement 原文「the generated bodies' runtime behavior is the runtime's」）。
10. **未决问题阻塞：通过**——行为面无开放问题（四项裁决已定、拒绝项成文）；延后项全部为带去向的 non-goal，非阻塞。

结论：**通过，进入 active。**

## 实现审查记录（welang-code-review，2026-09-04）

1. **规范符合性：通过**——第 10 章 16 条 Requirements/72 Scenarios 与增量逐字一致（机器 diff 16/16）；11 条 MODIFIED 与宿主逐字落地（11/11，按 Requirement 标题分块替换后字节比对）；注册表 31 条 title 与场景 THEN 消息子句逐码核对一致（31/31，均在场景中被行使）；validate --all --strict 对全部条目 owner/requirement 解析通过。
2. **验证诚实性：通过**——三任务验证命令本轮全部真实运行：task 1（validate、120 场景清点 72+48=120、E 码交叉引用机器核对、场景继承机器比对）；task 2（哨兵负例注入先行，消息原文 `FAIL: .../0100-lexical.md: diagnostic usage 'E0801:' has no registry entry in docs/spec/diagnostics.toml`，还原 clean 后再扩展；过渡期 31 条 owner-FAIL 如预期出现，宿主落地后自愈全绿）；task 3（逐字 diff、zh 孪生结构核对、全角冒号扫描 0 命中、docs_sync 16→17、git status 范围核对：ch1/ch2/ch6/ch7/ch8/ch9 ×2 + 1000 双件 + 注册表 + 变更目录，无越界文件）。
3. **测试先行：不适用（按前例）**——纯 spec 层（layers: [spec]），编译器未开工；验证阶梯为 validate/docs_sync/逐字 diff/负例注入（与前三片同形）。
4. **诊断协议稳定：通过**——validate.py/docs_sync.py 未改动，`--json` 字段无涉；新码 E0801–E0831 全局唯一（注册表 77 条无重复），同一变更内登记并入诊断段位 Requirement。
5. **单一权威：通过**——长期事实全部在 docs/spec/（第 10 章双件 + 注册表 + 六章宿主修订）；变更目录仅存变更工件；EN 文件无中文混入，zh 孪生为既定双语文档约定。
6. **红线复核：通过**——refr/ 不在 git status；提交未发生（commit 信息待用户指示后另出）；diff 与影响范围逐项吻合、无非目标改动。
7. **最小可信验证已跑：通过**——validate --all --strict OK；docs_sync 17 对 OK；逐字 diff 16/16+11/11；场景继承比对仅预期差异（+7 新场景、ch8 2 场景改题、ch7 1 场景原地收窄）；E 码冒号扫描 0。

**披露（示例刷新，Why 第 9 条既有预告）**：ch7/ch8/ch9 示例节 pending 注释与引言随本片刷新（EN+zh）——ch7 引言去「泛型」、pending 块改注「泛型应用形式已随第 10 章、List 等集合类型待集合章」；ch8 引言收窄为「引用模型待其章节」、pending 块移除已落地四项（方法/接口/impl/derives）并连带清除 ch9 落地后遗留的过时行（variant patterns/Never），`impl Eq for UserId` 注释行删除（该形现为 E0822 拒绝形而非 pending）；ch9 引言去「接口、derives」、pending 块标注 derives/Dyn 已落地，`derives Eq for Shape { ... }` 注释行删除（同 E0822 理）。审查后无增量文本修订——落地文本与 ready/active 两轮审查所通过者逐字一致。

结论：**通过，进入 complete。**

## 归档记录（welang-archive-sync，2026-09-04）

1. **规范提升**：实现期已完成——`docs/spec/1000-interfaces.md` + `.zh.md` 新建（16 Requirements/72 Scenarios + 示例节 + 术语表，docs_sync 16→17 对）；ch1/ch2/ch6/ch7/ch8/ch9 宿主 MODIFIED ×11 双语落地；注册表 46→77 条、段位 E0800–E0899 认领、未认领段收窄 E0900–E9999。诊断码全局唯一（validate registry clean）、§ 交叉引用合并后有效（逐字落地保证）、术语与既有章节一致（术语表 + 既有称呼沿用）。
2. **决策提升**：无新 ADR——全部取舍为规范内部决策，权威文本即规范自身（无型变/无子类型/无运算符重载等长期立场已成文为 Requirement）与本章 design.md（随目录归档，可追溯）；docs/decisions/ 现有两条 ADR 不受影响。
3. **状态提升**：无 docs/roadmap/，按约定在 openspec 层留档，不创建空目录。
4. **移动归档**：git mv 至 `openspec/changes/archive/2026-09-04-interfaces-generics/`，status 改 archived，tasks 全勾。
5. **复验**：validate --all --strict 通过。
