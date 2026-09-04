# Proposal: 工具链（第 21 章）

## Why

宿主规范早已为本章预留了全部接口点，逐条可 grep 验证：

1. **第 0 章原则 7 的两件待兑付**（`docs/spec/0000-principles.md`）："The toolchain MUST provide a mandatory formatter with no configuration options"——零配置格式化器是宪法级承诺，至今无章兑付；同段 "every compiler diagnostic MUST be provided ... with a structured, machine-operable remediation description"——`--json` 协议是它的机器面。本章兑付两件。
2. **第 0 章机制中立的分界**："the toolchain pins LLVM versions as a discipline"——版本号码永不入规范正文，工具行为归工具链章；本章全章遵守（`we version` 定形不定值）。
3. **第 0 章生成–检查–修复循环**：循环的"检查"腿需要 `we check` 的固定契约（管线止于何处、退出码、流式输出）——AI 原生语言的语言面落点是第 20 章，工具面落点是本章。
4. **第 15 章的两次让渡**（The source root and the dependency cache）："the manifest's format is the toolchain's business, not this chapter's" 与 "acquisition, version constraints, and lockfiles are the tooling layer's business"——manifest 骨架本章落定，获取故事本章显式留白。
5. **第 18 章的 vet 层让渡**（E1613 所在 Requirement）："the vet-tier heuristic survey is the tooling chapter's, not this layer's" 与章尾示例 "the heuristic survey and the may-block note await the tooling chapter's vet layer"——W1911/W1912 本章落定。
6. **第 19 章的链接让渡**（What this boundary does not fix）："Linkage — how a declared name binds to a native symbol, name mangling, library search, calling-convention variants beyond the platform C ABI — is the toolchain chapter's business"——本章以 E1906 + 机制不固定的方式承接。
7. **第 20 章的一整段让渡**（What testing does not fix）："The execution order of test blocks ..., parallelism between them, filtering, reporting formats, exit codes, the `tests/` directory layout ... all the toolchain chapter's" 与 "Exploration — v0.8's `--explore` ... is toolchain-layer business, as are its guard diagnostics (v0.8's W0601, E1001, E1003)"——we test 语义、探索模式与三守护码本章落定。
8. **第 6 章的文档渲染让渡**（Doc comments）："What documentation content means and how the toolchain renders it are the tooling chapters'"——`we doc` 本章落定。
9. **第 99 章的两条规则**（9900-diagnostics-registry）："`E` and `W` share one number space: a given four-digit number exists with at most one severity at a time"——本章 W 码取段内与 E 码不重叠的号码；"localization of compiler output is a toolchain concern outside the registry"——本地化归工具链，本章如实指名。

v0.8 基线（`refr/spec-0.8.md` §56–§64、§41.4–41.5、§65）给了完整参照系：CLI 总览、编译管线、fmt 规则、vet 配置、we test 语义、doc、JSON Lines 协议、模块解析（已归第 15 章）、LSP。本变更按四项已裁决取舍将其映射到 We 的已批表面。

## What Changes

1. **新章：第 21 章 Toolchain**（`docs/spec/2100-toolchain.md` + `.zh.md`），十二条 Requirements：
   - The `we` command surface —— 子命令集、目录/单文件 `[path]` 语义（`E1907`）、`we new` 名字检查（`E1904`）、`we version` 定形不定值；
   - The compile pipeline's observable contract —— 阶段序可观察、E 止损 W 放行（提升的 W 同 E）、check 不产不链、同输入同输出、增量是实现细节、目标选择词汇不固定；
   - The formatter —— 确定性、零配置（P7 兑付）、最小规则集（缩进/换行/空格/import 序），不折行不对齐；
   - Advisory diagnostics and `we vet` —— 三点名警告 `W1910`/`W1911`/`W1912` 触发语义、`[vet]` 三级配置与提升语义（`E1903`）、实现自有发现不入册；
   - `we test` —— tests/ 递归发现、文件内源序、跨文件不规定、pass/fail 两态（skip 与 error 随 v0.8 语法与双机制消亡）、退出码 0/1/2；
   - Exploration —— `--explore`/`--iterations`/偏序缩减、诚实有界（采样非穷尽）、守护码 `E1901`/`E1902`；
   - The JSON Lines protocol —— 逐行对象、诊断/测试事件字段集、字段稳定承诺（可增不可删改名）、`--json` 不改退出码；
   - The project manifest —— `we.toml` 骨架（name/version/type + `[vet]`/`[test]`，`E1905`）、依赖获取显式留白；
   - Documentation generation —— `///` 渲染、仅 pub、`--check` 不生成；
   - Linkage of foreign declarations —— foreign 名绑定原生符号、build 期链接、`E1906`、机制不固定；
   - What the toolchain does not fix —— 依赖获取、LSP（独立文档+一致性承诺）、本地化、增量/并行、性能预算；
   - Toolchain diagnostics segment —— 段位 `E1900`–`E1999` owner `2100-toolchain`，错误 `E1901`–`E1907` 与警告 `W1910`–`W1912` 同段共号 space。
2. **注册表**：段位拆分 `E1900`–`E1999` owner `2100-toolchain`（`E2000`–`E9999` 仍 unclaimed）；新码十条：`E1901`–`E1907` 七错误 + `W1910`/`W1911`/`W1912` 三警告（E/W 共号 space，号码不重叠）；条目 137→147、段 20→21。
3. **宿主修订：零处**。全部七处让渡句（ch0/ch6/ch15/ch18/ch19/ch20/ch99）在章落地后依然为真——让渡句写的是归宿不是内容，本章是被指向方而非语义接触方；无新语言表面（零新关键字、零新文法），故无宿主 Requirement 需要改写。
4. **v0.8 映射**（详见 design.md D13）：§56–§58、§60–§62 → 本章；§63 → 已归第 15 章；§64 → 独立文档 + 管线一致承诺；E0150→`E1904`、E0154→`E1903`、E1001→`E1901`、E1003→`E1902`、W0601→`W1910`、W0755→`W1911`、W0756→`W1912`（note→warning 档披露）；skip/error 双态消亡（随 test_each 与双机制）；W0140/W0141/W0257/W0303/W0401/W0402/W0901/W0902 不入册（实现自有发现，多数绑定 v0.8 死语法）；`--target` 词汇不固定（v0.8 的 GOOS/GOARCH 随 Go 后端消亡，ADR-0002）。
5. **双语文档**：`docs_sync` 27→28 对。

## 影响层

- **spec 层**：新章 21 + 注册表扩展，经 openspec 变更流程（本变更）；零宿主修订。
- **编译器/运行时层**（不在本变更）：管线实现、格式化器实现、vet 启发式、测试运行时与探索调度器、链接器集成——本章只固定可观察契约。
- **生态层**（不在本变更）：依赖获取、版本约束、锁文件、install/publish——显式留白，待专门变更。

## 影响范围

| 文件 | 动作 |
| --- | --- |
| `docs/spec/2100-toolchain.md` | 新建（第 21 章正文） |
| `docs/spec/2100-toolchain.zh.md` | 新建（中文镜像） |
| `docs/spec/diagnostics.toml` | 段位拆分 + 10 新码 |

## 裁决记录

用户裁决四项（2026-09-04，均采推荐）：

1. **全量一章** —— CLI+管线+fmt+vet+test（含 --explore 与守护码）+doc+JSON 协议+manifest 骨架+链接承接，一章收口；LSP 指向独立文档；依赖获取显式留白。
2. **点名 W 码入册** —— 只注册规范文本已点名承诺的三个码（v0.8 W0601/W0755/W0756 重编号入段，E/W 共号 space 内取不重叠号码）；fmt/vet 其余检查项作为工具行为入章、不入册。
3. **manifest 骨架+显式留白** —— `we.toml` 只批 name/version/type + `[vet]`/`[test]`（vet/test 语义需要它们可依）；依赖获取/版本约束/锁文件/install/publish 显式列入"本章不固定什么"。
4. **字段稳定性入章** —— JSON Lines 逐行对象、四级 severity、字段可增不可删改名、退出码不受 `--json` 影响，全部作为可观察契约入章（P7 机器可操作性的章级落点）。

## 目标与非目标

### 目标

- 工具链的可观察契约一站式固定：命令面、管线止损、格式化输出、advisory 层、测试运行形状、机器协议、manifest 骨架、链接失败。
- 七处宿主让渡句全部有承接（章内逐条对应），无一处悬空。
- 诚实边界：探索是采样不是穷尽、依赖获取是留白不是设计、性能零承诺、版本号码不入文。
- 注册表扩展遵守 E/W 共号 space 与一码一消息纪律。

### 非目标

- 依赖获取、版本约束语法、锁文件格式、install/publish —— 显式留白，待专门变更。
- LSP 协议细节 —— 独立文档；本章只承诺"编辑器诊断 = we check 诊断"。
- 格式化器/vet 的实现自有检查项入册 —— 它们是工具表面，不是规范承诺。
- 任何性能预算（编译时长、延迟、足迹）—— 评测层的事，规范不承诺。
- 编译器/工具链实现 —— layers: [spec]。

## 审计记录

2026-09-04，七点审计（welang-spec-impact-audit），结论：**通过**。

1. **问题真实性 ✓**：Why 的锚点 14 处逐一 grep 实证——ch0 P7 两承诺句（formatter/结构化修复描述）、ch0 机制中立句（LLVM 钉版）、ch6 文档渲染让渡、ch15 两让渡（manifest 格式/获取）、ch18 两让渡（vet 层调查/may-block note）、ch19 链接让渡、ch20 两让渡（tests/ 布局/探索与守护码）、ch99 两规则（E/W 共号 space/本地化），全部 1 hit 实证。基线固定为 `refr/spec-0.8.md` §56–§64、§41.4–41.5、§65 及 `refr/spec-0.8-review.md` 的 W0755/W0756 定义。
2. **影响层 ✓**：change.yaml `layers: [spec]` 与 影响层 一致；编译器/运行时实现、生态（依赖获取）边界在 影响层 与 非目标 双处写明。
3. **规范增量范围 ✓**：新增 12 Requirement（第 21 章）+ 修改 0 Requirement；新码 E1901–E1907 与 W1910–W1912 十条，grep 证实 `E19xx`/`W19xx` 在 docs/spec 正文与注册表零占用（唯一命中为待拆段头 `E1900-E9999`）；段位拆分 E1900–E1999（owner 2100-toolchain）+ E2000–E9999（unclaimed）遵守 ch99 共号 space 规则；E1406 段注随落段再刷新（沿 slice 5 先例）。无与既有规范冲突、无与 active 变更重复（当前唯一 active 即本变更）。
4. **原则一致性 ✓**：P7 双兑付——formatter 零配置 + `--json` 字段稳定性；P4——fmt 刻意不折行不对齐（无唯一解的决策不做）；P8——探索采样非穷尽、依赖获取 named gap、性能零承诺，三处如实直说；机制中立——版本号码永不入文（`we version` 定形不定值）；P5——零新语言表面，十码全工具层判定。**零宿主修订为唯一非常规形态**，论证在 design.md D2：七处让渡句是前瞻指针（写归宿不写内容），本章落地后逐句依然为真，审计逐句复核通过；无新语言表面故无宿主枚举需要扩。
5. **参考基线固定 ✓**：v0.8 引用固定为 §56–§64、§41.4–41.5、§65；W0755/W0756 触发语义对照评审文档原文核对；映射账 D13 逐条可追。
6. **验收边界 ✓**：目标四条可机械判定（12R 逐字提升、十条注册表条目 tomllib 机检、零宿主修订以 git diff 证、docs_sync 28 对）；非目标排除依赖获取/LSP 细节/实现项入册/性能承诺，蔓延面已封。
7. **粒度 ✓**：单一 spec 层变更，一章 + 注册表，垂直可验收，与 effects/concurrency/ffi/testing 变更同形。

## 审查记录

2026-09-04，10 点语义审查（welang-change-review），结论：**通过（3 发现 + 2 小疵，均已修复回灌增量）**。

1. **proposal ✓**：黑盒问题+目标；实现决策在 design（D1–D14），任务在 tasks，无混杂；零宿主修订作为设计立场显式论证（非遗漏）。
2. **spec 增量 ✓**：全部为可观察行为（命令面/止损/输出形状/退出码/失败码）；MUST/MAY 合 BCP 14；无实现方式泄漏。修复 F3（小疵）：R1 `--color` 值集（auto/always/never）原文未固定——补齐使其成为真契约；同处 "`lockless dependency caches`" 措辞含混（We 无锁文件）——改为 "leaves sources and the manifest untouched"。
3. **design ✓**：唯一最小路径；被拒替代方案有录（D4 不入册判据、D6 三取舍、D7 两处刻意不做）；引用精确到 Requirement 与码。修复 F2：W1912 触发面原仅列无缓冲 channel 收发——v0.8 清单自身不完备（满缓冲 send、空缓冲 receive 同样阻塞），扩为 ch18 五个阻塞操作并在 D8/D13 双处披露偏离。
4. **tasks ✓**：勾选项均有来源与验证；哨兵 E1999 注入为负向验证；无 deferred/未决方案选择。
5. **场景覆盖 ✓**：normal/boundary/failure 三路齐——R1 空过滤器与编译失败、R2 阶段序与止损、R6 分歧捕获与诚实边界、R8 值域与缺键、R10 未绑定符号。修复 F1：R12 段位保留清单原漏 E1913–E1999 且以 E/W 双前缀双重计号（违 ch99 共号 space 语义）——改为无字母号码清单（1900、1908–1909、1913–1999），并注明 1910–1912 为 W 码所占。
6. **无空章节 ✓**：12 Requirements 均有行为增量；R11（不固定什么）与 R12（段位）均载实质承诺（named gap 的指名义务、保留号规则）。
7. **测试先行 ✓**：spec 层变更的等价物 = 哨兵注入负例（E1999 → `--all --strict` FAIL → 还原），任务 4.1 已列；一码一消息扫描任务在列。
8. **负向断言 ✓**：E1901–E1907 十码中七个错误码各有拒绝/失败场景（E1901 分歧、E1902 未 mock 效果、E1903 值域×2、E1904 名字、E1905 缺件、E1906 未绑定、E1907 路径）；三 W 码各有触发场景；示例节补 E1905 拒绝形后十码样例齐全。
9. **完成度闭环 ✓**：D14 三要素账在位——类型检查零参与（十码全工具层）、代码生成零新面、运行时四件事（fmt/vet/test 运行器/探索调度器）全为契约下实现自由。
10. **未决问题阻塞 ✓**：无行为级未决——依赖获取是显式 named gap（R11 指名 + 非目标排除），LSP 协议显式独立文档，二者均非未决而是已决的留白。

## 实现审查记录

2026-09-04，七点实现审查（welang-code-review），结论：**通过（1 发现，实施中已修复）**。

1. **规范符合性 ✓**：第 21 章为 delta 逐字提升（12R/42S，行级包含扫描 0 缺失；标题邻接扫描排除围栏后 0 违例）；注册表十条目与 R12 逐字对账——十码 title 在 R12 文本中以 `` `CODE` title`` 形式全部在场，severity E 七条 error、W 三条 warning，owner/allocated 齐整；段位声明与 R12 一致（`E1900`–`E1999` owner `2100-toolchain`、`E2000`–`E9999` unclaimed）；requirement 字段全部指向章内真实 Requirement 标题。E1406 段注已按先例刷新（加 toolchain 落段一句）。
2. **验证诚实性 ✓**：tasks 4 节五项验证命令逐个实跑——哨兵负例（E1999 注入 1500-modules.md → `--all --strict` FAIL exit=1 → 还原 → 绿）、注册表 tomllib 计数 147/21、包含/邻接扫描、zh 三查（R/S 镜像 12/42、16 围栏字节同一、全角冒号 0）、一码一消息扫描（首跑 9 失配→修复→0，见发现 1）、docs_sync 28 对。tasks 1–3 节的候选/审计/审查工作在激活前已完成且记录在册，勾选属实。
3. **测试先行证据 ✓**：spec 层等价物 = 哨兵负例，在注册表扩展落地**之前**注入并实测 FAIL（负例先行的时序真实）；JSON Lines 示例块字段集与 R7 逐字段一致（diagnostic 八字段含可选 help、test-result 四字段、test-summary 四字段、无 errors 字段）。
4. **诊断协议稳定 ✓**：十新码全局唯一（`--all --strict` 注册表扫描绿即机检证明）；E/W 共号 space 守恒——1910–1912 为 W 码所占、保留清单无字母书写；JSON Lines 协议系本章首次固定，无既有字段可删改；`--json` 不改退出码入 R7。
5. **单一权威 ✓**：章正文与触发语义唯一权威在 `docs/spec/2100-toolchain.md`（.zh.md 镜像）；注册表只持条目、R12 明文分工（"Trigger semantics live in this chapter's requirements; the entries live in the registry"）；变更目录只留过程工件随归档；章/注册表英文、变更工件中文，各循其律。
6. **红线复核 ✓**：`git status` 实证——修改仅 `docs/spec/diagnostics.toml`，新增仅两章文件与变更目录，**零宿主章节被动**（零宿主修订从设计立场变为可验证事实）；refr/ 无变更；提交信息英文、无署名尾注。
7. **最小可信验证已跑 ✓**：`validate toolchain --strict`、`validate --all --strict`、`docs_sync --check`（28 对）、注册表 tomllib 机检、七让渡锚点落地后 grep 复验（八文件十锚点全部在场）——全部绿。

**发现与披露**：

- **D-1（实施中修复）**：一码一消息扫描首跑抓出示例节"Advisory findings"块三行 `// W19xx:` 注释以散文而非注册表标题起始（W1910/W1911/W1912 × delta/EN/zh 三份 9 失配）——已改为标题起始 + 续行散文的_house 形态，三份同步修复后复扫 0 失配。与 slice 5 的 E1805 先例同型。
- **D-2（工具披露）**：标题邻接扫描首版正则误报围栏内 `#` 注释行为标题——扫描修正为剥围栏后重跑（纯检查工具修正，零内容影响）。
