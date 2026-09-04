# 任务清单：工具链（第 21 章）

## 1. 候选与四件套

- [x] 建立 `openspec/changes/toolchain/`（change.yaml：candidate、layers [spec]、created）
  来源：用户裁决四项；本变更提案 Why。
  验证：`python3 openspec/tools/validate.py toolchain --strict` 通过（候选期容许未注册码引用）。
- [x] proposal.md（Why=九处宿主锚点+v0.8 §56–§64 基线；What Changes=新章 12 块 + 注册表 + 零宿主修订论证 + v0.8 映射）
  来源：v0.8 §56–§64、四项用户裁决。
  验证：人工对照 ch0/ch6/ch15/ch18/ch19/ch20/ch99 让渡句原文，逐一可在文件中 grep 到。
- [x] design.md（四裁决 + D1–D14 设计决定 + 十码取舍账 + v0.8 映射账 + 三要素账）
  来源：裁决记录与本章设计推演。
  验证：每条不入册决定（W0140 等）与消亡决定（skip/error 双态、--target、errors 字段）均有先例或原则锚。
- [x] specs/ 增量（英文，docs/spec 字面片段）：toolchain 新章 12R/42S + examples
  来源：What Changes 逐条。
  验证：`python3 openspec/tools/validate.py toolchain --strict` 通过。

## 2. 就绪门（7 点 welang-spec-impact-audit）

- [x] 七点审计全过并出 ready 报告（含七让渡句"落地后依然为真"逐句 grep 复核）
  来源：.agents/skills/welang-spec-impact-audit/SKILL.md。
  验证：报告落变更目录，七点逐项有结论。

## 3. 激活门（10 点 welang-change-review）

- [x] status→active 前通过 10 点审查，发现项修复回灌增量
  来源：.agents/skills/welang-change-review/SKILL.md。
  验证：审查记录落 proposal.md 审查记录段。

## 4. 实施（实施负例先行）

- [x] 注册表先行：diagnostics.toml 拆段 E1900–E1999 owner 2100-toolchain + E2000–E9999 unclaimed + E1901–E1907/W1910–W1912 十码条目（注入哨兵 E1999 于宿主章实测 FAIL 后还原——负例必须 `--all --strict` 跑）
  来源：注册表扩展义务（AGENTS.md 规则 2）。
  验证：`python3 openspec/tools/validate.py --all --strict` 注册表检查绿。
- [x] 新章 `docs/spec/2100-toolchain.md`：12R + 示例节（H3 子节）+ 术语对照
  来源：specs/toolchain delta 逐字提升。
  验证：逐字 diff 零差异；标题邻接扫描 0 违例。
- [x] zh 孪生 `2100-toolchain.zh.md`：H1 第 21 章、术语对照、代码块与 EN 字节一致（注释英文）、全角冒号码形扫描 0
  来源：双语同步义务。
  验证：docs_sync 结构配对 + R/S 计数镜像机检。
- [x] 一码一消息扫描：新章与 delta 全量（反引号码形与注释码形 `EXXXX:`/`WXXXX:` message 首行完整、以注册表 title 开头）
  来源：welang-code-review 习惯项。
  验证：扫描脚本 0 失配。
- [x] docs_sync 27→28 对 + `--check` 绿
  来源：双语同步义务。
  验证：`python3 openspec/tools/docs_sync.py --check`。

## 5. 完成门（7 点 welang-code-review）与归档

- [x] 7 点代码审查（规范符合性/验证诚实性/测试先行证据/诊断协议稳定/单一权威/红线复核/最小可信验证）+ 实现审查记录入 proposal.md
  来源：.agents/skills/welang-code-review/SKILL.md。
  验证：七点逐项结论 + 披露段。
- [x] 归档 `git mv` → archive/2026-09-04-toolchain，status archived，tasks 全 [x] + 完成记录
  来源：.agents/skills/welang-archive-sync/SKILL.md。
  验证：validate --all --strict 绿（归档态）。
- [x] 记忆更新 + 提交（英文提交信息、无署名尾注、refr/ 不入库）
  来源：常设授权「按列表顺序依次执行」。
  验证：git log + git show --stat 复核。

## 完成记录

2026-09-04 完成门通过：七点实现审查（记录在 proposal.md 实现审查记录，1 发现 D-1 实施中修复、D-2 工具披露）；注册表 137→147 条、20→21 段；第 21 章双语落地，docs_sync 27→28 对；零宿主修订（git status 实证仅三路径被动）；`validate --all --strict` 归档态复跑绿。状态 → archived，移入 archive/2026-09-04-toolchain/。
