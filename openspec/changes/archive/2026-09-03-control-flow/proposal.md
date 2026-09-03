# Proposal: control-flow

## Why

1. **骨架无控制流。** 第 2 章批准了块/语句/表达式骨架与优先级表，但语句形式只有绑定/赋值/表达式语句——`if`/`while`/`loop`/`break`/`continue`/`return`/`defer` 七个已保留关键字（第 1 章 21 词清单）没有任何产生式。控制流是插入骨架的第一组产生式，后续 match、迭代、函数各章都以它为前置。
2. **v0.8 基线有裁决点未分离。** §21.2 把 if/else 表达式形式、分支类型一致（Never 除外）与编译器内部结果变量命名混在一处；§24 的 defer 限定为"函数体顶层语句"（E0725）——该限制先于块表达式裁决，与第 2 章"块是表达式"的正交性未论证。从头裁决须分离形式与类型化，并对 defer 作用域作出明确取舍。
3. **E01xx 段位机械上不可用于本章。** 注册表 E0100–E0199 owner 是 `0200-grammar`，条目的 `requirement` 字段须在该文件内解析——本章触发 Requirement 位于第 3 章，故本章码位必须从 E0200+ 认领新段（注册表修订，第 99 章明文允许）。

## 目标与非目标

目标：

1. 创建规范第 3 章（`docs/spec/0300-control-flow.md`）：if/else 表达式与语句双形、while/loop、break/continue（裸形式）、return 形式、defer 语句；与块值/行接续的整合规则；控制流语义中可脱离类型系统陈述的部分（分支独立作用域、Never 参与规则的前向引用）。
2. **注册表段位修订**：认领 E0200–E0299（owner `0300-control-flow`），原 E0200–E9999 未认领行收窄为 E0300–E9999；分配首批 E02xx 码（4 个：E0201 循环控制词脱离循环、E0202 无值形式用于取值位置、E0203 defer 体非块、E0204 defer 位置违规；E0200 与 E0205–E0299 预留）。
3. 显式裁决 v0.8 的 defer 作用域限制是否继承（块作用域 vs 函数体顶层）与 break 是否携值。

非目标：

- **match 与模式匹配**（模式文法、或模式、守卫、穷尽性——v0.8 §21.3–§21.5）——独立章节；穷尽性是模式×类型系统的复合检查，体积自成一体。
- **for 与迭代协议**（`for x in`、Iterable/Iterator、组合子、`..` 范围——v0.8 §21.2/§21.2.1/§21.2.2）——独立章节；`..` 不在第 1 章词法闭集内，需词法+语法成对增补。
- **分支/条件类型化**（条件须为 Bool、分支类型一致、Never 细则）——类型系统章节；本章以前向引用声明义务，不定义类型。
- **return 的函数上下文规则**（返回类型一致性、fn 体外 return 拒绝的完整判定）——声明/函数章节定上下文；本章只批准形式。
- **panic/todo/assert**（v0.8 §25）——错误机制章节。

## What Changes

- 新增规范能力 `control-flow`，ADDED Requirements（6 条：if/else 双形与作用域、while/loop、break/continue、return 形式、defer、控制流诊断段位）。
- 修改第 2 章能力 `grammar` 的 Statements Requirement（MODIFIED）：语句族改为"按归属章节增量批准"的封闭结构（本条以增量定稿为准）。
- 修改注册表 `docs/spec/diagnostics.toml` `[segments]`：新增 `E0200-E0299`（domain 控制流，owner `0300-control-flow`）+ 段内结构注释；`E0200-E9999` 行改为 `E0300-E9999`（仍 unclaimed）；新增 E0201–E0204 四条目。

## 影响层

- **spec**：第 3 章 ADDED；注册表段位修订与条目扩展（同一变更内，履行 AGENTS.md 规则 2）。
- 不触及 compiler/stdlib/tooling/process/docs。

## 影响范围

- **第 2 章宿主**：控制流产生式插入骨架的语句类与表达式类；第 2 章"Statements"的语句清单由"三种"扩为封闭枚举的增补（以 MODIFIED 或引用关系定稿于增量）。若以 MODIFIED 触及第 2 章，需保持逐字合并纪律。
- **后续章节**：match/迭代/函数/类型各章以本章为前置；defer 作用域裁决影响 resource 章节（§35 scope resource 的对齐方式）。
- **注册表**：E0200–E0299 认领后，后续章节按段认领（E0300+）。
- **与 v0.8 的关系**：§21.2 if/else 形式与分支独立作用域继承为素材；编译器内部结果变量命名（`_ifResult0`）判定为实现细节**不进规范**；§24 defer 的"函数体顶层"限制**继承**（用户裁决，见裁决记录）；break/continue 裸形式**继承**（用户裁决）；v0.8 文中散见的 `defer f.Release()` 非块形式与其 §24 块形式规范不一致，从头裁决取**块形式唯一**（P2）。
- **风险与不可逆性**：defer 作用域与 break 携值是语言表面决定；E02xx 码位是稳定性承诺。

## 裁决记录

2026-09-03 用户裁决两项：

1. **defer 作用域：继承 v0.8**——仅函数体顶层语句可用，函数退出（含提前 return/break）时逆序执行；嵌套块内 defer 被拒（E0204）。未取推荐的块作用域方案（块作用域与块表达式正交、P1 更局部，但用户择简：单一退出点语义）。后果："函数体"概念由声明章节批准——本章以带前向引用的规则批准 defer 形式与位置约束，位置规则在声明章节落地前只有拒绝侧可触发。
2. **break 携值：裸 break，循环是语句**——loop/while 不产出值；break/continue 无携值、无标签（闭集，加标签 = spec change）。循环产出值走显式路径（累加器变量、if/match 表达式、后续迭代组合子）。

## 审计记录

2026-09-03 审计（candidate → ready 关卡，7 点）：

1. **问题真实性：通过。** Why 引用可核验事实：第 2 章无控制流产生式（`docs/spec/0200-grammar.md` Statements 枚举可查）；7 个关键字保留未用（第 1 章 21 词清单）；v0.8 素材固定（§21.2、§24）；E01xx owner 约束在注册表与 validate.py 检查 (c) 中机械可验。
2. **影响层声明：通过。** layers `[spec]`；两个规范增量 + 注册表修订均 spec 层；不改编译器/工具行为。
3. **规范增量范围：通过。** 6 条 ADDED（control-flow）+ 1 条 MODIFIED（grammar Statements）逐条列于 What Changes；E0201–E0204 对照注册表与 active 变更（当前无）无冲突；E/W 无同号冲突；段位修订走第 99 章"经修订认领"明文路径。
4. **原则一致性：通过。** P2：break 裸形式唯一、defer 块形式唯一（v0.8 双形式被否）；P5：循环产出值须显式状态，无隐性数据通道；P4：关键字起始无歧义、else-if 链=嵌套无独立产生式；无原则例外，无需 ADR。
5. **参考基线固定：通过。** 素材固定 `refr/spec-0.8.md` §21.2/§24；结果变量命名判为实现细节排除；v0.8 defer 双形式自不一致已披露并裁决（P2 取块）。
6. **验收边界：通过。** 目标机械可判：章节存在含 6 条 Requirement、注册表两行段位 + 4 条目、validate strict + registry clean、第 2 章 Statements 逐字合并。非目标五项排除 match/for/类型化/return 上下文/panic。
7. **粒度：通过。** 单能力 + 宿主章一条 MODIFIED；validate + 注册表检查垂直验证；不与 match/迭代耦合（显式非目标）。

**结论：通过，进入 ready。**

## 审查记录

2026-09-03 语义审查（ready → active 关卡，10 点）：

**职责边界**：(1)–(4) 全部通过——proposal 纯黑盒；增量全为可观察行为且 BCP 14 无 SHOULD/MAY；design 与增量一致；tasks 无未决选择。

**内容质量**：(5) 发现 **F1：形式/类型边界泄漏**——R1 原文 "the if produces no value for that evaluation" 把臂取值推向运行时表述；块值语句性是句法可判定，但混合值臂（一臂有值一臂无值）属类型层。已修正：E0202 严格限于"无 else 的 if 用于取值位置"（静态可判），有 else 的臂值一致性规则显式前向引用类型章节，本章只定形与臂作用域。(6) 无空章节 ✓；(7) task 2 注入先于条目写入（spec 层断言先例）✓；(8) E0205 负例真实 ✓；(9) 完成度闭环 N/A——spec 层、无实现，类型化以非目标+前向引用明示；(10) 无阻塞性未决 ✓。

**E0105/E0202 边界（记录，无工件改动）**：语句关键字出现在表达式位置时，可识别的无值形式（loop/while/defer 等）报 E0202（更有用的修复指引），不可识别为形式的 token 仍走第 2 章 E0105 兜底——design D3 记录该类定义，实现期以此为准。

**结论：通过（F1 修复后复验），进入 active。**

## 实现审查记录

2026-09-03 实现审查（active → complete 关卡，7 点）：

1. **规范符合性：通过。** 第 3 章 6 条 Requirement 与增量逐字一致（diff 核对 6/6；第 6 条尾部差异为章节示例/术语装置，requirement 本体另行 diff 一致——已知误报）；第 2 章 Statements 与 MODIFIED 逐字一致（4 个 Scenario）；注册表 4 条目 requirement 字段解析到章节实际标题（E0201→Break and continue、E0202→If and else expressions、E0203/E0204→Defer），validate strict + registry clean。
2. **验证诚实性：发现并修正 F2 后通过。** tasks.md 最初写"6 条共 17 个 Scenario"，实数 16（4+2+3+2+3+2，delta 与章节双侧 grep 一致）——修正为实际数。其余验证命令本轮真实复跑：validate（全仓 + 定向）、docs_sync（10 对）、逐字 diff ×3、E0205 负例注入（exit=1，消息 "diagnostic usage 'E0205:' has no registry entry"，还原后 clean）、全角冒号扫描（零）。
3. **测试先行证据：N/A。** spec 层变更（layers `[spec]`），无编译器实现与测试；验收即第 1、2 点的机械校验。
4. **诊断协议稳定：通过。** 新码 E0201–E0204 全局唯一、同变更登记；未触碰任何既有条目与 `--json` 字段；E0200/E0205–E0299 预留未用（触发扫描为零）。
5. **单一权威：通过。** 长期事实全部落位：段位与条目在 diagnostics.toml（唯一注册表）、触发语义在第 3 章 Requirements、示例节按裁决以非权威节并入章节正文、术语进第 3 章术语表；变更目录仅存增量与过程记录。
6. **红线复核：通过。** `refr/` 未入暂存（待提交清单仅本章产物）；commit 信息将无署名 trailer；diff 无越界（未动 validate.py 与任何工具）。
7. **最小可信验证已跑：通过。** spec 层验证阶梯全绿：validate.py --all --strict（"1 change(s) valid; registry clean"）、docs_sync.py（10 对对齐）、双语结构镜像 + 诊断消息串英文保留抽检。

**结论：通过（F2 修正后），进入 complete。**

## 归档记录

2026-09-03 归档（complete → archived）：

- **规范提升**：第 3 章 `docs/spec/0300-control-flow.md` + `.zh.md`（6 Requirement / 16 Scenario，非权威示例节 4 小节，术语 7 条）；第 2 章 Statements 按 MODIFIED 逐字更新（EN+zh，新增 "A statement family grows" Scenario）；注册表 E0200–E0299 段位 + E0201–E0204 条目。
- **决策提升**：无 ADR——两项裁决（defer 继承 v0.8、裸 break）均遵循既有原则（P2 单一形式、P5 显式状态），无原则例外；取舍理由保留于裁决记录与 design D1/D2，供后续章节引用。术语 "statement family/语句族" 入第 3 章术语表，第 2 章 zh 用同一名，无第二种称呼。
- **状态提升**：无 docs/roadmap/，按技能约定不创建空目录，留档于此。
- **移动归档**：`git mv` 至 `openspec/changes/archive/2026-09-03-control-flow`，status → archived。
- **复验**：validate --all --strict 通过（零 active 变更 + registry clean）。
