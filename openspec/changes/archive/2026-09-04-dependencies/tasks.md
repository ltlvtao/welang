# 任务清单：依赖获取（第 22 章）

## 1. 候选与四件套

- [x] 建立 `openspec/changes/dependencies/`（change.yaml：candidate、layers [spec]、created）
  来源：用户裁决四项；本变更提案 Why。
  验证：`python3 openspec/tools/validate.py dependencies --strict` 通过（候选期容许未注册码引用）。
- [x] proposal.md（Why=ch15 让渡句+ch21 两处 gap+v0.8 §65/§56/E0111/E0151 锚点；What Changes=新章 7 块+注册表+两处宿主修订+v0.8 映射）
  来源：v0.8 §65、四项用户裁决。
  验证：人工对照 ch15 R1/ch21 R8/R11 原文，逐一可在文件中 grep 到。
- [x] design.md（四裁决 + D1–D12 设计决定 + 六码账 + MVS 取舍账 + v0.8 映射账 + 三要素账）
  来源：裁决记录与本章设计推演。
  验证：每个"不载/续死/消亡"决定（prerelease、cargo ^0 特例、we install、E0153/E0154）均有先例或原则锚。
- [x] specs/ 增量（英文，docs/spec 字面片段）：dependencies 新章 7R/24S + examples；toolchain R8/R11 两 MODIFIED
  来源：What Changes 逐条。
  验证：`python3 openspec/tools/validate.py dependencies --strict` 通过。

## 2. 就绪门（7 点 welang-spec-impact-audit）

- [x] 七点审计全过并出 ready 报告（含 ch15 让渡句与 ch21 两 gap 句"落地后依然为真/必假"逐句复核、E20xx 零占用 grep、MVS 与既有规范一致性）
  来源：.agents/skills/welang-spec-impact-audit/SKILL.md。
  验证：报告落变更目录，七点逐项有结论。

## 3. 激活门（10 点 welang-change-review）

- [x] status→active 前通过 10 点审查，发现项修复回灌增量
  来源：.agents/skills/welang-change-review/SKILL.md。
  验证：审查记录落 proposal.md 审查记录段。

## 4. 实施（实施负例先行）

- [x] 注册表先行：diagnostics.toml 拆段 E2000–E2099 owner 2200-dependencies + E2100–E9999 unclaimed + E2001–E2006 六码条目（注入哨兵 E2099 于宿主章实测 FAIL 后还原——负例必须 `--all --strict` 跑）
  来源：注册表扩展义务（AGENTS.md 规则 2）。
  验证：`python3 openspec/tools/validate.py --all --strict` 注册表检查绿；tomllib 机检 153 条/22 段。
- [x] 新章 `docs/spec/2200-dependencies.md`：7R + 示例节（H3 子节）+ 术语对照
  来源：specs/dependencies delta 逐字提升。
  验证：逐字包含扫描零缺失；标题邻接扫描（剥围栏）0 违例。
- [x] 宿主修订第 21 章（EN+zh）：R8/R11 两 Requirement 按 delta 替换 + 两示例块注释刷新（码块 EN/zh 字节同步）
  来源：specs/toolchain MODIFIED delta。
  验证：替换后与 delta 逐字一致；ch21 其余 Requirement 与 HEAD 逐字一致（零越界修订证明）。
- [x] zh 孪生 `2200-dependencies.zh.md`：H1 第 22 章、术语对照、代码块与 EN 字节一致（注释英文）、全角冒号码形扫描 0
  来源：双语同步义务。
  验证：docs_sync 结构配对 + R/S 计数镜像机检（7/24）。
- [x] 一码一消息扫描：新章与 delta 全量（反引号码形与注释码形 `EXXXX:` message 首行完整、以注册表 title 开头）
  来源：welang-code-review 习惯项。
  验证：扫描脚本 0 失配。
- [x] docs_sync 28→29 对 + `--check` 绿
  来源：双语同步义务。
  验证：`python3 openspec/tools/docs_sync.py --check`。

## 5. 完成门（7 点 welang-code-review）与归档

- [x] 7 点代码审查（规范符合性/验证诚实性/测试先行证据/诊断协议稳定/单一权威/红线复核/最小可信验证）+ 实现审查记录入 proposal.md
  来源：.agents/skills/welang-code-review/SKILL.md。
  验证：七点逐项结论 + 披露段。
- [x] 归档 `mv` → archive/2026-09-04-dependencies，status archived，tasks 全 [x] + 完成记录
  来源：.agents/skills/welang-archive-sync/SKILL.md。
  验证：validate --all --strict 绿（归档态）。
- [x] 记忆更新 + 提交（英文提交信息、无署名尾注、refr/ 不入库）
  来源：常设授权（用户指名本片）。
  验证：git log + git show --stat 复核。

## 完成记录

2026-09-04 完成门通过：七点实现审查（记录在 proposal.md 实现审查记录，0 行为发现、2 检查工具披露）；注册表 147→153 条、21→22 段；第 22 章双语落地 + 第 21 章两处宿主修订（diff 五处恰为预期），docs_sync 28→29 对；`validate --all --strict` 归档态复跑绿。状态 → archived，移入 archive/2026-09-04-dependencies/。
