# We 语言规范 —— 第 99 章：诊断码注册表

### Requirement: 注册表权威与角色分工

`docs/spec/diagnostics.toml` 是诊断码体系的唯一注册表。章节定义码**何时发出**——触发权威位于 Requirements 与 Scenarios；注册表定义每个已分配码的**条目**——条目权威：身份、严重级、标题、说明、修复建议、归属章节与 Requirement。一码恰有一义、一条目、一归属。章节散文可为可读性引用码与段位区间，但不得重定义条目或其字段。

#### Scenario: 章节散文复制条目

- **WHEN** 某章节增量以散文描述某码的修复建议或条目字段，而不是扩展注册表
- **THEN** 变更评审拒绝它；注册表是定义条目的唯一位置

#### Scenario: 消费者查询某码

- **WHEN** 任何消费者——编译器、工具、模型或人类——持有工具链发出的某诊断码
- **THEN** 完整条目仅从 `docs/spec/diagnostics.toml` 即可取得：严重级、标题、完整说明、修复建议、以及承载触发语义的归属章节 Requirement

### Requirement: 条目模式

每个已分配码恰有一条 `[diagnostic.<CODE>]` 条目，提供：`severity`（`error` 或 `warning`）、`title`（稳定短消息，英文）、`description`（被拒内容与原因的完整陈述，触发指向归属 Requirement）、`remediation`（机器可操作的修复指引，第 0 章原则 7）、`owner`（归属章节文件 slug）、`requirement`（归属章节中的 Requirement 标题）、`allocated`（批准该码变更的 ISO 日期）。条目以英文书写；编译器输出的本地化是注册表之外的工具链议题。注册表格式为 TOML，且必须保持可被标准 TOML 读取器解析。

#### Scenario: 条目缺失修复建议

- **WHEN** 某注册表条目省略 `remediation`，或其修复建议不是可操作的指引
- **THEN** 校验拒绝该条目（原则 7：机器可操作的修复建议是强制义务）

#### Scenario: 条目引用可解析

- **WHEN** 注册表被校验
- **THEN** 每条目的 `owner` 指向 `docs/spec/` 下已存在的章节文件，且其 `requirement` 指向该文件中已存在的 Requirement 标题

### Requirement: 扩展与稳定性

新诊断码或新段位只能经 spec 层变更进入——同一变更内扩展 `docs/spec/diagnostics.toml`。`E` 与 `W` 共享同一号码空间：一个四位数同一时刻至多存在一个严重级。对已分配码重编号、删除或改义被禁止（第 0 章：诊断协议是稳定性承诺）。条目模式**加字段**向后兼容；删字段或改名不兼容。

#### Scenario: 某章节认领码位

- **WHEN** 某内容章节批准新诊断
- **THEN** 其变更在注册表段位表认领未占用区段，并为每个已分配码添加条目；对既有码重编号在评审中失败

#### Scenario: 出现 E/W 同号冲突

- **WHEN** 某变更要在 `E0500` 已存在时分配 `W0500`
- **THEN** 校验拒绝该分配；号码空间为两种严重级共享

### Requirement: 段位

段位声明于注册表的 `[segments]` 表，每段含领域与归属。初始段位为：`E0001`–`E0099` 词法与命名（第 1 章）、`E0100`–`E0199` 语法与解析（语法章节）、`E0200` 以上由后续章节经修订认领。每条注册表条目必须落在其 `owner` 章节所拥有的已声明段位内；预留范围以段位数据声明，不枚举为条目。

#### Scenario: 条目落在其归属段位之外

- **WHEN** 注册表包含某条目，其号码不在其 `owner` 章节拥有的任何段位内
- **THEN** 校验拒绝该条目

#### Scenario: 预留码未经认领即被使用

- **WHEN** 某变更使用已声明预留范围内的号码，却未通过注册表认领
- **THEN** 校验与评审拒绝该变更；预留号码不会隐式可用

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| registry | 注册表 |
| diagnostic entry | 诊断条目 |
| remediation | 修复建议 |
| number space | 号码空间 |
| trigger authority | 触发权威 |
| entry authority | 条目权威 |
