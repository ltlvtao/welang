# Tasks: diagnostics-registry

- [x] 1. （归档提升时）创建 `docs/spec/diagnostics.toml`：`[segments]` 初始段位表（E0001–E0099 词法与命名/0100-lexical、E0100–E0199 语法与解析/grammar、E0200+ 待认领）+ 第 1 章 12 个已分配码的条目（E0001–E0009、E0011–E0013，每条目 severity/title/description/remediation/owner/requirement/allocated 七字段）；预留范围以段位数据与注释表达，不枚举为条目。
  - 来源：proposal `## 目标与非目标` 目标 1；design 决策 1、2、5。
  - 验证：`python3 -c "import tomllib; r=tomllib.load(open('docs/spec/diagnostics.toml','rb')); assert len(r['diagnostic'])==12 and len(r['segments'])==3"` 退出码 0；`grep -c "^\[diagnostic\." docs/spec/diagnostics.toml` 输出 12；12 条目的 requirement 字段逐一在 0100-lexical.md 中有同名 Requirement 标题（脚本核对退出码 0）。
- [x] 2. 机制落地：validate.py 新增五项注册表检查（解析/模式、冒号用法 ⊆ 条目、owner/requirement 可解析、条目落段、E/W 同号冲突）；AGENTS.md 增补"分配码位 = 同一变更内扩展注册表"义务；development-process §3.3 同步；docs/README.md 命名约定增补豁免结构文件 `diagnostics.toml`（+zh）。
  - 来源：proposal `## 目标与非目标` 目标 4；design 决策 3、4、6。
  - 验证：测试先行——实施检查**前**先做注入实验证明现状不拦（临时向 docs/spec/0100-lexical.md 注入 `` `E9999: bogus` ``、向注册表追加 `[diagnostic.W0006]` 同号条目，validate 仍 PASS = 目标测试失败态）；实施后同两注入期望 validate FAIL；还原后 `python3 openspec/tools/validate.py --all --strict` 退出码 0；三个阶段的输出记录回写 proposal 审查记录。
- [x] 3. （归档提升时）章节提升与归档：将 lexical MODIFIED 合并进 `docs/spec/0100-lexical.md`（+.zh.md）；创建 `docs/spec/9900-diagnostics-registry.md`（+.zh.md，4 条 Requirements，术语对照新增 registry / diagnostic entry / remediation / number space / trigger 等且与既有章零冲突）；全量校验；变更目录加日期前缀移入 archive；归档记录回写。
  - 来源：proposal `## 目标与非目标` 目标 2、3；design 决策 6、7；仓库归档命名约定。
  - 验证：`grep -c "^### Requirement:" docs/spec/0100-lexical.md` 输出 11 且 `grep -c "^### Requirement:" docs/spec/9900-diagnostics-registry.md` 输出 4；`python3 openspec/tools/docs_sync.py --root docs` 退出码 0；`python3 openspec/tools/validate.py --all --strict` 退出码 0；`ls openspec/changes/archive/ | grep diagnostics-registry` 输出带日期前缀目录名。
