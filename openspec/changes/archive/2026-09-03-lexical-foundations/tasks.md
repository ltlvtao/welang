# Tasks: lexical-foundations

- [x] 1. （归档提升时）创建 `docs/spec/0100-lexical.md`：将 `specs/lexical/spec.md` 的 ADDED Requirements 逐字合并（去 delta 头，加第 1 章章标题），创建配对翻译 `0100-lexical.zh.md`；两文件术语对照表在 0000 章基础上增补本章新术语。
  - 来源：proposal `## 目标与非目标` 目标 1；design 决策 10。
  - 验证：`grep -c "^### Requirement:" docs/spec/0100-lexical.md` 输出 11；`grep -c "^#### Scenario:" docs/spec/0100-lexical.md` 输出 23；`python3 openspec/tools/docs_sync.py --root docs` 退出码 0。
- [x] 2. 规范一致性复核：本章 12 个已分配诊断码（E0001–E0009、E0011–E0013）与既有规范（0000 章）无冲突；一码一义（同一码在场景处与注册表处语义一致）；六类禁止路径（软关键字、非 ASCII 标识符、原始/多行字符串、自定义运算符、未知属性、关键字作名字）各有 THEN-拒绝场景；新术语与 0000 章术语表无冲突。
  - 来源：proposal `## 目标与非目标` 目标 4；design 决策 8；验证阶梯"语言规范"行。
  - 验证：`grep -oE "E[0-9]{4}" openspec/changes/lexical-foundations/specs/lexical/spec.md | sort -u | wc -l` 输出 19（E0001–E0014、E0019、E0020、E0099、E0100、E0199——12 分配 + 段界与预留引用）；一码一义与六类禁止场景逐项人工核对，结论回写 proposal 审查记录。
- [x] 3. 全量校验并归档：`python3 openspec/tools/validate.py --all --strict` 与 `python3 openspec/tools/docs_sync.py --root docs` 通过；变更目录移入 `openspec/changes/archive/`，目录名加日期前缀；归档记录回写 proposal.md。
  - 来源：proposal `## 影响范围`；仓库归档命名约定（`docs/README.md`）。
  - 验证：`python3 openspec/tools/validate.py --all --strict` 退出码 0；`ls openspec/changes/archive/ | grep lexical-foundations` 输出带日期前缀的目录名。
