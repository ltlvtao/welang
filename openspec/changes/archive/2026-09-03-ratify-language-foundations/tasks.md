# Tasks: ratify-language-foundations

- [x] 1. 修订 `AGENTS.md` 语言研发不变量一节：十条原则的内联清单改为指向 `docs/spec/0000-principles.md` 的指针（"一类事实只有一个权威位置"）；不变量"编译器实现语言为 Go（与代码生成目标一致）"改为解耦表述（实现语言 Go；编译目标 LLVM → 原生二进制，详见规范第 0 章宿主策略）。
  来源：proposal `## What Changes` 第 2、3 条；`## 影响层` process 项。
  验证：`grep -n "0000-principles" AGENTS.md` 输出 ≥1 行；`grep -n "与代码生成目标一致" AGENTS.md` 无输出（旧表述已移除）；`grep -n "LLVM" AGENTS.md` 输出 ≥1 行。
- [x] 2. 补充"规范增量用英文"约定：`AGENTS.md` 硬规则 2 与 `docs/process/development-process.md` §3.3 各加一句——`specs/*/spec.md` 以英文书写，作为 `docs/spec/` 英文权威文本的字面片段，归档提升时逐字合并。两文件同步补中文对照（development-process.zh.md）。
  来源：proposal `## What Changes` 语言约定澄清项；design 决策 8。
  验证：`grep -n "英文" AGENTS.md | grep -c "spec"` ≥1；`grep -n "literal fragment\|逐字" docs/process/development-process.md docs/process/development-process.zh.md` 两文件各有输出。
- [x] 3. `docs/README.md` 新增"Document Naming Conventions"一节（+ `.zh.md` 同步）：spec 章节文件 `<nnnn>-<slug>.md` 四位零填充步长 100（0000 保留给第 0 章）；ADR 为 `ADR-<nnnn>-<slug>.md`；归档变更目录 `YYYY-MM-DD-<name>`；active 变更与 capability 用 kebab 语义名；固定结构名（README/AGENTS/SKILL/change.yaml/四件套）豁免。同步 ADR 命名：`docs/process/development-process.md` §7 补 `ADR-<nnnn>-<slug>` 格式引用；`.agents/skills/welang-archive-sync/SKILL.md` 的 `ADR-<nn>-<slug>` 改为 `ADR-<nnnn>-<slug>`（起草时定位到旧格式实际在 SKILL.md 而非 §7）。
  来源：proposal `## What Changes` 命名约定三条；`## 影响层` docs 项。
  验证：`grep -n "Naming Conventions\|文档命名约定" docs/README.md docs/README.zh.md` 两文件各有输出；`grep -rn "ADR-<nn>-" .agents/skills/welang-archive-sync/SKILL.md` 无输出且 `grep -n "ADR-<nnnn>" .agents/skills/welang-archive-sync/SKILL.md` 有输出。
- [x] 4. 归档日期前缀规则落三处：`docs/process/development-process.md` §7（+zh）、`openspec/README.md` 归档小节、`.agents/skills/welang-archive-sync/SKILL.md`——移入 `archive/` 时目录名加 `YYYY-MM-DD-` 前缀。SKILL.md 规范提升步骤中过时的单文件示例（`docs/spec/we-lang-spec-v0.9.md`）同步改为章节文件命名（`<nnnn>-<slug>.md`），与本变更批准的命名约定一致。
  来源：proposal `## 影响层` process 项。
  验证：`grep -rn "YYYY-MM-DD-" docs/process/development-process.md openspec/README.md .agents/skills/welang-archive-sync/SKILL.md` 三文件各有输出（zh 配对文件另计）；`grep -n "we-lang-spec" .agents/skills/welang-archive-sync/SKILL.md` 无输出。
- [x] 5. 创建 `docs/decisions/ADR-0002-llvm-first-commercial-runtime.md`（+ `.zh.md`，README 索引两文件各加一行）：商用交付目标、LLVM IR 从第一天、自建运行时含完整精确 GC 为产品范围、编译器实现语言 Go 与目标解耦、`foreign "c"` 受控边界、规范机制中性纪律；被拒替代：Go 后端（v0.8 路线）、C 源码中间层（封顶精确 GC/尾调用/优化上限）。**范围增补（用户指示 2026-09-03，归档前）**：增加 Dependencies 一节——三方件枚举与版本钉住（LLVM 21.1.8 初始钉住，版本号真实性已核实；Go 经 go.mod 钉住；libc/C ABI 按目标平台），并规定版本号永不进入规范正文。
  来源：proposal `## What Changes` 宿主策略条目；`## 影响范围`；design 决策 5、6；用户 2026-09-03 依赖钉住指示。
  验证：`ls docs/decisions/ADR-0002*` 列出 2 个文件；`grep -n "ADR-0002" docs/decisions/README.md docs/decisions/README.zh.md` 两文件各 1 行；`grep -c "21.1.8" docs/decisions/ADR-0002-llvm-first-commercial-runtime.md` ≥1（钉住值在依赖表中登记）。
- [x] 6. （归档提升时）创建 `docs/spec/0000-principles.md`：将 `specs/language-foundations/spec.md` 的 ADDED Requirements 逐字合并（去 delta 头），创建配对翻译 `0000-principles.zh.md`，附关键术语英中对照表；人工术语一致性审查（local reasonability / unique semantics / escape hatch 等跨文件统一）。
  来源：proposal `## 目标与非目标` 目标 3；design 决策 8。
  验证：`grep -c "^### Requirement:" docs/spec/0000-principles.md` 输出 14；`python3 openspec/tools/docs_sync.py --root docs` 退出码 0。
- [x] 7. 全量校验并归档：`python3 openspec/tools/validate.py --all --strict` 与 `python3 openspec/tools/docs_sync.py --root docs` 全部通过；变更目录移入 `openspec/changes/archive/2026-09-03-ratify-language-foundations/`（目录名加日期前缀，本变更是该规则的首个适用对象）；归档记录回写 proposal.md。
  来源：proposal `## 影响范围`；任务 4 的规则。
  验证：`python3 openspec/tools/validate.py --all --strict` 退出码 0；`ls openspec/changes/archive/ | grep ratify-language-foundations` 输出带日期前缀的目录名。
