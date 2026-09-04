# 任务清单：测试系统（第 20 章）

## 1. 候选与四件套

- [x] 建立 `openspec/changes/testing/`（change.yaml：candidate、layers [spec]、created）
  来源：用户裁决四项；本变更提案 Why。
  验证：`python3 openspec/tools/validate.py testing --strict` 通过（候选期容许未注册码引用）。
- [x] proposal.md（Why=九处宿主锚点+v0.8 §38–41/§60 基线；What Changes=新章 9 块 + 7 宿主修订 + 注册表 + v0.8 映射）
  来源：v0.8 §38–41/§60、四项用户裁决。
  验证：人工对照 ch0/ch1/ch6/ch12/ch14/ch15/ch16/ch18/注册表段注释原文，逐一可在文件中 grep 到。
- [x] design.md（四裁决 + D1–D13 设计决定 + v0.8 码映射账 + 四条件对号）
  来源：裁决记录与本章设计推演。
  验证：每条偏离（test_each/property 消亡、mock 收窄单态、E1002 消亡、效果豁免立场）均有先例或原则锚。
- [x] specs/ 增量（英文，docs/spec 字面片段）：testing 新章 + lexical/declarations/fn-types/errors/modules/effects/concurrency 宿主
  来源：What Changes 逐条。
  验证：`python3 openspec/tools/validate.py testing --strict` 通过。

## 2. 就绪门（7 点 welang-spec-impact-audit）

- [x] 七点审计全过并出 ready 报告（逐规则代码示例记录）
  来源：.agents/skills/welang-spec-impact-audit/SKILL.md。
  验证：报告落变更目录，七点逐项有结论。

## 3. 激活门（10 点 welang-change-review）

- [x] status→active 前通过 10 点审查，发现项修复回灌增量
  来源：.agents/skills/welang-change-review/SKILL.md。
  验证：审查记录落 proposal.md 审查记录段。

## 4. 实施（实施负例先行）

- [x] 注册表先行：diagnostics.toml 拆段 E1800–E1899 owner 2000-testing + E1801–E1806 六码条目 + E1406–E1499 段注释刷新（注入哨兵 E1899 于宿主章实测 FAIL 后还原——负例必须 `--all --strict` 跑）
  来源：注册表扩展义务（AGENTS.md 规则 2）。
  验证：`python3 openspec/tools/validate.py --all --strict` 注册表检查绿。
- [x] 新章 `docs/spec/2000-testing.md`：9R + 示例节（H3 子节）+ 术语对照
  来源：specs/testing delta 逐字提升。
  验证：逐字 diff 零差异；标题邻接扫描 0 违例。
- [x] zh 孪生 `2000-testing.zh.md`：H1 第 20 章、术语对照、代码块与 EN 字节一致（注释英文）、全角冒号 E1xxx 扫描 0
  来源：双语同步义务。
  验证：docs_sync 结构配对 + R/S 计数镜像机检。
- [x] 宿主章 EN+zh 拼接（归一化 rstrip+双换行，8 处 Requirement）：ch1 Keywords 词表+兑付句；ch6 File structure 枚举+场景；ch12 语境清单句；ch14 第三捕获边界句；ch15 prelude 闭集+场景；ch16 效果测试句；ch18 Scheduling carve-out+场景；ch18 task 边界指针句
  来源：specs/ 七 delta。
  验证：提升文==delta 逐字 diff（8/8）；宿主场景计数机器比对 git HEAD（无意外丢失）。
- [x] 一码一消息扫描：变更新增行 + 新文件全量，反引号码形与注释码形 `EXXXX:` message 与注册表 title 逐一相符
  来源：welang-code-review 习惯项。
  验证：扫描脚本 0 失配。
- [x] docs_sync 26→27 对 + `--check` 绿
  来源：双语同步义务。
  验证：`python3 openspec/tools/docs_sync.py --check`。

## 5. 完成门（7 点 welang-code-review）与归档

- [x] 7 点代码审查（规范符合性/验证诚实性/测试先行证据/诊断协议稳定/单一权威/红线复核/最小可信验证）+ 实现审查记录入 proposal.md
  来源：.agents/skills/welang-code-review/SKILL.md。
  验证：七点逐项结论 + 披露段。
- [x] 归档 `git mv` → archive/2026-09-04-testing，status archived，tasks 全 [x] + 完成记录
  来源：.agents/skills/welang-archive-sync/SKILL.md。
  验证：validate --all --strict 绿（归档态）。
- [x] 记忆更新 + 提交（英文提交信息、无署名尾注、refr/ 不入库）
  来源：常设授权「按列表顺序依次执行」。
  验证：git log + git show --stat 复核。

## 完成记录

2026-09-04 全周期完成：candidate（四件套 + 9R/34S 增量 + examples；四项用户裁决锁定设计空间）→ ready（7 点审计，Why 九锚点逐一 grep 实证；效果豁免为唯一刻意留白、D4 论证）→ active（10 点审查 3 发现修复回灌：scope timeout 括号拼写、mock 力量止于其块边界场景、D14 三要素账；33→34 场景）→ 实施（哨兵 E1899 注入 ch18 实测 `--all --strict` FAIL 先行；注册表 131→137/20 段；20 章 EN+zh 提升，逐字 9/9、代码块 6/6 字节一致；七宿主 ×2 语言 8 块拼接逐字、场景计数 HEAD 机比各恰 +1、Requirement 计数不变；一码一消息新内容 60 对 0 失配；docs_sync 27 对；实现期修复 E1805 示例折行、沿留 ch12 先在三连换行——见实现审查记录披露）→ complete（7 点代码审查零阻断、2 项披露）→ archived。设计取舍随归档 design.md 留档，无新 ADR（spec 权威在章，沿 effects/collections/concurrency/ffi 先例）。提交另记。
