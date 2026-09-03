# Proposal: diagnostics-registry

## Why

1. **第 0 章承诺了诊断协议是循环主接口，但注册表面是散文。** "第一作者与核心循环"条款：诊断协议（人类可读与机器可读形态）是生成—检查—修复循环的主接口、是稳定性承诺；原则 7：每个诊断必须附结构化、机器可操作的修复描述。现状：码的含义散在第 1 章 Requirement 正文与分段表的散文枚举里——给定一个码，没有一个单一位置可取到完整条目（严重级、稳定短消息、完整说明、修复建议、归属），LLM 与未来编译器无法把它当稳定接口消费。（用户提出 2026-09-03：错误码/警告码应独立列表管理，包含编译器完整说明，便于 LLM 处理。）
2. **散置将随章节增长而加深。** 0200-grammar 即将认领 E0100–E0199；若无注册表，每章的码继续进散文，机械核对与编译器消费的成本与章节数同步增长。注册表必须在大批码位分配之前落地。
3. **机器工件的命名与校验语言尚未定义。** 文档命名约定没有"注册表类结构化文件"条款（无中文配对、仅英文）；validate.py 没有注册表↔章节的一致性检查。

## 目标与非目标

目标：

1. 创建 `docs/spec/diagnostics.toml`：诊断码体系的**唯一注册表**——`[segments]` 段位表 + 第 1 章已分配 12 码的 `[diagnostic.EXXXX]` 条目；每条目含 severity / title / description / remediation / owner / requirement / allocated 七字段。
2. 瘦身第 1 章 Diagnostic code scheme（MODIFIED）：逐码枚举移入注册表，章节保留码格式规则、段内结构与指针；"后续章节认领段位"义务改为"同一变更内扩展注册表文件"。
3. 创建第 99 章 `docs/spec/9900-diagnostics-registry.md`（+zh）：注册表的角色分工（章节 = 触发权威，注册表 = 条目权威）、条目模式、扩展与稳定性义务的规范 Requirements。
4. 机制落地：validate.py 新增注册表一致性检查（解析/模式、`EXXXX:` 用法 ⊆ 条目、owner/requirement 可解析、条目落在归属段位、E/W 同号冲突）；AGENTS.md 与 development-process 增补扩展义务；docs/README.md 命名约定增补豁免结构文件 `diagnostics.toml`。

非目标：

- **诊断线缆协议**（`--json` 字段模式、JSON Lines、消息模板占位符）——未来的诊断协议/工具链章节。
- **编译器输出的语言与本地化**——英文条目不等于对编译器输出语言的裁决。
- **任何既有码的重编号或含义变更**——显式禁止（本变更不移动任何码位）。
- **警告码的分配**——仅重申 W 遵循同一方案，本章不分配任何 W 码。

## What Changes

- 新增能力 `diagnostics`（4 条 ADDED Requirements，归档提升为 `docs/spec/9900-diagnostics-registry.md`）。
- 能力 `lexical` MODIFIED 1 条 Requirement（Diagnostic code scheme 瘦身为规则 + 指针，2 个 Scenario 同步改写）。
- 新增机器工件 `docs/spec/diagnostics.toml`（段位表 + 12 条目），归档时创建。
- 流程三处：validate.py 检查、AGENTS.md 规则、development-process §3.3 义务表述、docs/README.md 命名约定（+zh）。

## 影响层

- **spec**：第 1 章 MODIFIED（1 条 Requirement）；第 99 章 ADDED；注册表文件创建。
- **process**：validate.py 新增检查；AGENTS.md / development-process 增补"分配码位 = 同一变更内扩展注册表"义务。
- **docs**：docs/README.md 命名约定条款（+zh）。
- 不触及 compiler/stdlib/tooling/benchmark（编译器尚不存在；注册表是未来编译器直接消费的接口契约，本身不是编译器改动）。

## 影响范围

- **所有后续分配诊断码的章节**（自 0200-grammar 始）：义务从"扩展分段表"变为"扩展注册表文件（段位认领 + 逐码条目）"。
- **验证阶梯**：validate.py --all 从此包含注册表一致性检查；破坏注册表一致性的任何 docs/spec 改动将被机械拦截。
- **第 1 章文本变化（MODIFIED）**：12 个已分配码的号码、含义、段位归属均不变——变的是条目内容的位置与形式；另有一处**语义澄清**：E 与 W 共享号码空间（同号并存自始禁止）——原文本"码全局唯一"未裁决同号异级并存，本变更为注册表唯一性将其显式关闭。
- **风险与不可逆性**：注册表文件成为稳定性承诺表面；条目模式**加字段**向后兼容，**删字段/改名**禁止。

## 审计记录

**2026-09-03 · welang-spec-impact-audit · 通过**

1. **问题真实性** ✅ 三条 Why 均可验证：第 0 章"第一作者与核心循环"Requirement 原文即"诊断协议……是循环的主接口，是本规范的稳定性承诺"，原则 7 要求"结构化、机器可操作的诊断修复描述"；现状可机械核实——`grep -ri remediation docs/spec/` 零命中（无任何完整条目），码义仅存于第 1 章散文；docs/README.md 命名约定无机器工件条款；validate.py 无注册表检查。用户 2026-09-03 直接提出本需求（独立列表管理 + 编译器完整说明 + 便于 LLM 处理）。
2. **影响层声明** ✅ `layers: [spec, process, docs]` 与影响层逐项对应：spec（第 1 章 MODIFIED、第 99 章 ADDED、注册表文件）、process（validate.py/AGENTS.md/development-process）、docs（README 命名约定）。validate.py 归 process 层与 d53ff57 先例一致。不触及 compiler/stdlib/tooling。
3. **规范增量范围** ✅ 明确列出：`diagnostics` ADDED 4 条、`lexical` MODIFIED 1 条（Diagnostic code scheme 全新文本）。**不新增、不移动任何码位**——12 个已分配码号码/含义/段位归属不变，变的只是条目位置与形式；增量文本中 E0500/W0500 为场景示例（不带冒号调用形），不构成分配。
4. **原则一致性** ✅ 本变更是原则 7 与第一作者条款的直接兑现（机器可操作修复描述有了唯一落点）；P2（单一注册表=单一查询路径）。无原则冲突，无例外需 ADR。
5. **参考基线固定** ✅ 未引用 refr/ 材料；引用的 docs/spec/0000-principles.md 与 0100-lexical.md 均为已批准章节。v0.8 的 §52/§52 诊断格式、§62 JSON Lines **不作为依据**——线缆协议列为非目标，留给未来章节从零裁决。
6. **验收边界** ✅ 目标可机械判定（tomllib 断言 12 条目、grep Requirement 计数、validate 退出码、负向测试 FAIL→PASS）；非目标四项（线缆协议/本地化/重编号/警告码分配）显式排除。
7. **粒度** ✅ 单一关注点"诊断码注册表成为独立管理的承诺表面"。spec/process/docs 三层的改动都服务同一垂直验收（注册表存在且被机械执法）；只建注册表不建检查会留下第二个不受执法的表面——反而违反本变更目的。

## 审查记录

**2026-09-03 · welang-change-review · 通过（2 项发现，已处置）**

结构校验先行：`validate.py --all --strict` 通过（ready 态）。十点结论：1 职责边界 ✅（proposal 黑盒，增量只述可观察行为）；2 spec 增量黑盒与 BCP 14 ✅（MUST 仅用于变更义务与工件形态约束，无 SHOULD/MAY）；3 design ✅（唯一路径、被拒替代、逐条引用，与增量无矛盾）；4 tasks ✅ 修复后通过；5 场景覆盖 ✅（normal：消费者取条目/引用可解析；boundary：E/W 同号、条目落段外、预留位未认领即用；failure：散文复制条目、缺修复建议）；6 无空章节 ✅；7 测试先行 ✅ 修复后通过（任务 2 改为三阶段：实施前注入实验=失败态目标测试、实施后 FAIL、还原 PASS）；8 负向断言 ✅（两个真实注入负向测试：用法违例 + 同号冲突）；9 完成度闭环 ✅（裁定：本变更是规范基础设施而非语言特性，类型检查/代码生成/运行时三要素不适用；等价闭环 = 条目可解析 + validate.py 执法 + 负向测试，三者均在本变更内交付）；10 无未决阻塞 ✅（用户三项裁决：TOML 单文件/仅英文/先提 lexical 再做注册表；其余为 design 主张）。

发现与处置：

1. **任务 2 未按测试先行排序**：原验证只要求"实施后注入 FAIL→还原 PASS"。已改为三阶段——实施**前**注入实验证明现状不拦（失败态目标测试先行存在），实施后同注入 FAIL，还原 PASS；三阶段输出均回写审查记录。
2. **影响范围未披露语义澄清**：E/W 共享号码空间是对第 1 章"码全局唯一"未裁决面的显式关闭（同号异级并存自始禁止），超出原表述"仅位置与形式变化"。已在影响范围补记。

**2026-09-03 · welang-code-review · 通过（7 点，1 项实现发现已修复）**

1. 规范符合性 ✅：9900 章与 diagnostics 增量逐字一致；0100 的 Diagnostic code scheme 节与 lexical MODIFIED 增量逐字一致（两处 diff 均仅 `## Terminology` 前空行）；中文版结构对齐（docs_sync 8 对）。
2. 验证诚实性 ✅：任务 1 三断言实际运行（12 条目/3 段位、grep 12、requirement 全解析）；任务 3 计数（0100=11/23 不变、9900=4/8）、docs_sync、validate 全过。
3. 测试先行证据 ✅（三阶段实验，输出已存于会话记录）：阶段一（实施前）双违例注入（0100 加 `E9999: bogus` 用法 + 注册表加 `W0006` 同号条目）validate 仍 PASS——失败态目标测试先行固定；阶段三（实施后）同注入 validate FAIL，两条精确命中（"entries 'E0006' and 'W0006' share number 0006"、"diagnostic usage 'E9999:' has no registry entry"）；还原后 PASS。**实现发现**：检查器初版把注册表路径写成 `docs/diagnostics.toml`（正确为 `docs/spec/diagnostics.toml`），阶段三首跑"registry clean"系空转假象——失败态测试拒绝通过暴露了 bug，修正路径后三阶段全部符合预期。此发现是测试先行方法有效性的直接证据。
4. 诊断协议稳定 ✅：零码重编号、零含义变更；注册表为新建表面；E/W 号码空间规则落地并由检查 (e) 执法。
5. 单一权威 ✅：注册表 = 条目权威 + 段位权威，章节 = 触发权威；AGENTS.md 规则 2、development-process §3.3（+zh）、docs/README 命名约定与目录职责（+zh）均已增补；不设新 ADR（注册表机制本身即规范第 99 章，无机制外的工程选型需要 ADR 承载）。
6. 红线复核 ✅：`git status` 范围 = 声明层 [spec, process, docs] 逐文件吻合；`refr/` 未触及；无署名 trailer（本变更尚无提交）。
7. 最小可信验证 ✅：validate --all --strict（含注册表五项执法，"registry clean"）、docs_sync 8 对、逐字 diff、注入负向测试两例。

**任务 2 负向测试三阶段补记（2026-09-03）**：阶段一 PASS（检查未实现，违例不拦）→ 实现中发现路径 bug 并修复 → 阶段三 FAIL（双违例精确拦截）→ 还原 PASS。全记录见上方代码审查第 3 点。

## 归档记录

**2026-09-03 · welang-archive-sync · 完成**

- 规范提升：第 99 章 `docs/spec/9900-diagnostics-registry.md`（+zh，4 Requirements/8 Scenarios，术语 6 条与既有章零重叠）创建；第 1 章 Diagnostic code scheme 按 MODIFIED 瘦身（分段表与逐码枚举让位注册表，11/23 计数不变）；机器工件 `docs/spec/diagnostics.toml` 创建（3 段位 + 12 条目）。
- 决策提升（裁定，见审查记录第 5 点）：注册表机制即规范第 99 章本身，不另设 ADR；design 全文随本目录归档保留。
- 状态提升：尚无 `docs/roadmap/`，留档于本记录。
- 流程提升：AGENTS.md 规则 2、development-process §3.3（+zh）、docs/README 命名约定与目录职责（+zh）增补"分配码位 = 同一变更内扩展注册表"义务与机器工件豁免条款；validate.py 五项注册表检查上线（三阶段测试先行验证，含路径 bug 发现与修复）。
- 目录移动：`openspec/changes/diagnostics-registry` → `openspec/changes/archive/2026-09-03-diagnostics-registry`；status: archived。
- 复验：validate --all --strict 通过（registry clean）；docs_sync 8 对通过。
