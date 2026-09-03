# Tasks: dual-language-docs

- [x] 1. 实现 `openspec/tools/docs_sync.py`（纯标准库；`--root` 参数默认仓库 `docs/`；检测缺配对、孤儿翻译、标题层级计数不一致、围栏块计数不一致）
  来源：design.md「决策 3」；Requirement: Structural synchronization check
  验证：在 `/tmp` 构造四个负向夹具（缺配对、标题层级数不一致、围栏块数不一致、孤儿翻译）与一个干净夹具（其中含一对文档：围栏内各有一行 `#` 注释且两版注释文字不同），分别运行 `python3 openspec/tools/docs_sync.py --root <夹具路径>`——负向夹具各自输出对应 FAIL 行且退出码 1，干净夹具（证明围栏内 `#` 行不计入标题）输出 OK 且退出码 0

- [x] 2. `docs/README.md` 新增 "Documentation Language" 章节（配对规则、英文权威、完成定义的权威英文表述）
  来源：proposal「What Changes-约定」；Requirement: Paired translation files / English authority / Bilingual sync is part of change completion
  验证：`grep -c "^## Documentation Language" docs/README.md` 输出 1；逐条对照 `specs/docs-language/spec.md` 四条 Requirement 确认表述一致

- [x] 3. `docs/process/development-process.md` 三处修订：§4 验证阶梯 docs 行加 `docs_sync.py`；新增文档语言规则小节；§7 增补非语言能力增量的推广目标一句
  来源：proposal「What Changes-流程文本修订」；design.md「决策 4」
  验证：`grep -c "docs_sync" docs/process/development-process.md` 输出 2；`grep -c "English is authoritative" docs/process/development-process.md` 输出 1；§7 一句同时出现 `docs/spec/` 与 `authoritative location`

- [x] 4. `AGENTS.md` 硬规则 2 改为双语同步支持表述；验证阶梯表 docs 行加 `docs_sync.py`
  来源：proposal「What Changes-流程文本修订」
  验证：`grep -n "docs_sync" AGENTS.md` 命中验证阶梯行（L56）；硬规则 2 含 `.zh.md` 与"英文为权威文本"表述

- [x] 5. 翻译 `docs/README.md` → `docs/README.zh.md`（按任务 2 修订后的英文终稿翻译）
  来源：proposal「What Changes-翻译落地产物」；Requirement: Paired translation files
  验证：`python3 openspec/tools/docs_sync.py` 输出中无涉及 README 文件对的 FAIL 行（最终运行输出 OK: 2 document pair(s) aligned）

- [x] 6. 翻译 `docs/process/development-process.md` → `docs/process/development-process.zh.md`
  来源：proposal「What Changes-翻译落地产物」；Requirement: Paired translation files
  验证：`python3 openspec/tools/docs_sync.py` 退出 0（输出 OK: 2 document pair(s) aligned，全仓库文档对齐）

- [x] 7. 全量关卡：双校验器 + 提交前检查
  来源：AGENTS.md「验证阶梯」仅文档/流程/openspec 工件行
  验证：`python3 openspec/tools/validate.py --all --strict` 退出 0；`python3 openspec/tools/docs_sync.py` 退出 0；`git diff --check` 无输出

- [x] 8. 修复 `openspec/tools/validate.py` 严格模式验证内容提取缺陷（双重 partition → 前缀剥离）
  来源：proposal「What Changes-校验器缺陷修复」（实施中发现的范围增补，见审查记录）
  验证：修复后 `python3 openspec/tools/validate.py --all --strict` 通过（真实变更含无 ASCII 冒号的已勾选验证行，无空验证误报）；/tmp 合成变更（已勾选任务 + 空验证行）经 `validate_change(strict=True)` 报 "checked task has empty verification"（负向断言）
