# Tasks: grammar-skeleton

- [x] 1. 编写规范增量 specs/grammar/spec.md（7 条 ADDED Requirement，含触发 Scenarios）
来源：proposal What Changes 第 1 条
验证：python3 openspec/tools/validate.py grammar-skeleton --strict 通过；每条 Requirement ≥ 1 个 Scenario；E0101–E0105 码与 Scenarios 中的消息串逐一对齐

- [x] 2. 扩展注册表 docs/spec/diagnostics.toml（段位 owner 转正 + 段内结构注释 + E0101–E0105 五条目）
来源：proposal What Changes 第 2 条；AGENTS.md 规则 2（同一变更内扩展注册表）
验证：python3 openspec/tools/validate.py --all --strict 输出 registry clean；负例注入（实施前执行，先于章节提升与注册表条目写入）：在 9900-diagnostics-registry.md 临时追加 `E0106: injected...` → validate FAIL（"diagnostic usage 'E0106:' has no registry entry"，exit=1）→ 还原 → OK registry clean（exit=0）。注：本变更为注册表既有检查器的使用者，无需重跑三阶段方法论，负例点测足矣（审查时修正原"三阶段"措辞）

- [x] 3. 归档与提升（0200-grammar.md + .zh.md 双语 + 示例节并入 + docs_sync + 审查记录补记）
来源：proposal 目标 1；docs/process/development-process.md §7 提升规则；用户裁决 2026-09-03（示例以非权威节并入正文）
验证：提升后 docs/spec/0200-grammar.md 的 Requirements 与增量逐字一致（diff 核对）；`## Examples (non-authoritative)` 节逐字取自 specs/grammar/examples.md 置于末条 Requirement 与 Terminology 之间；zh 孪生含镜像示例节（注释译文，fenced 块数一致）；docs_sync.py 报告双语结构同步；validate.py 全仓通过
