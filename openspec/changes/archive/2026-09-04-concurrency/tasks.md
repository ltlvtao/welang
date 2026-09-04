# 任务清单：并发（第 18 章）

## 1. 候选与四件套

- [x] 建立 `openspec/changes/concurrency/`（change.yaml：candidate、layers [spec]、created）
  来源：用户裁决「一章全量」；本变更提案 Why。
  验证：`python3 openspec/tools/validate.py concurrency --strict` 通过（候选期容许未注册码引用）。
- [x] proposal.md（Why=七处宿主悬句实锚；What Changes=新章 9 块 + 7 宿主修订 + 延后清单）
  来源：v0.8 §29–§37、评审补丁 E0733/§35.3.1、四项用户裁决。
  验证：人工对照 ch0/ch8/ch10/ch14/ch16 悬句原文，逐一可在文件中 grep 到。
- [x] design.md（四裁决 + D1–D16 设计决定 + v0.8 码映射 + P1 复核）
  来源：裁决记录与本章设计推演。
  验证：每条偏离（Cond 单配、效果段拼写、await→Result、方向视图显式化、命名实参位置化、Shareable 非 derives）均有先例或原则锚。
- [x] specs/ 增量（英文，docs/spec 字面片段）：concurrency 新章 + lexical/grammar/interfaces/fn-types/errors/modules/effects 七宿主
  来源：What Changes 逐条。
  验证：`python3 openspec/tools/validate.py concurrency --strict` 通过。

## 2. 就绪门（7 点 welang-spec-impact-audit）

- [x] 七点审计全过并出 ready 报告（逐规则代码示例记录）
  来源：.agents/skills/welang-spec-impact-audit/SKILL.md。
  验证：报告落变更目录，七点逐项有结论。

## 3. 激活门（10 点 welang-change-review）

- [x] status→active 前通过 10 点审查，发现项修复回灌增量
  来源：.agents/skills/welang-change-review/SKILL.md。
  验证：审查记录落 proposal.md 审查记录段。

## 4. 实施（实施负例先行）

- [x] 注册表先行：diagnostics.toml 拆段 E1600–E1699 owner 1800-concurrency + E1601–E1618 十八码条目（注入哨兵 E1699 于宿主章实测 FAIL 后还原——负例必须 `--all --strict` 跑，per-change 模式不复跑用量扫描）
  来源：注册表扩展义务（AGENTS.md 规则 2）。
  验证：`python3 openspec/tools/validate.py --all --strict` 注册表检查绿。
- [x] 新章 `docs/spec/1800-concurrency.md`：16R/~60S + 示例节（H3 子节）+ 术语对照
  来源：specs/concurrency delta 逐字提升。
  验证：逐字 diff 零差异；标题邻接扫描 0 违例。
- [x] zh 孪生 `1800-concurrency.zh.md`：H1 第 18 章、术语对照、代码块与 EN 字节一致（注释英文）、全角冒号 E1xxx 扫描 0
  来源：双语同步义务。
  验证：docs_sync 结构配对 + R/S 计数镜像机检。
- [x] 七宿主章 EN+zh 拼接（归一化 rstrip+双换行，8 处 Requirement）：ch1 关键词 39 词+场景；ch2 表达式骨架+场景；ch10 Derives 句+场景；ch12 捕获指针句+场景、E0401 语境枚举+场景；ch14 Unwinding 落定+场景；ch15 预导入名（Shareable/currentCancelSignal）+场景扩展；ch16 carve-out+两场景
  来源：specs/ 七 delta。
  验证：提升文==delta 逐字 diff（8/8）；宿主场景计数机器比对 git HEAD（无意外丢失）。
- [x] D14 刷新：ch8 示例 Ref/Shared 悬句改写（EN+zh）
  来源：D14 纪律。
  验证：grep "still arrive with the concurrency" 归零。
- [x] 一码一消息扫描：变更新增行 + 新文件全量，反引号码形 `EXXXX:` message 与注册表 title 逐一相符
  来源：welang-code-review 习惯项。
  验证：扫描脚本 0 失配。
- [x] docs_sync 24→25 对 + `--check` 绿
  来源：双语同步义务。
  验证：`python3 openspec/tools/docs_sync.py --check`。

## 5. 完成门（7 点 welang-code-review）与归档

- [x] 7 点代码审查（规范符合性/验证诚实性/测试先行证据/诊断协议稳定/单一权威/红线复核/最小可信验证）+ 实现审查记录入 proposal.md
  来源：.agents/skills/welang-code-review/SKILL.md。
  验证：七点逐项结论 + 披露段。
- [x] 归档 `git mv` → archive/2026-09-04-concurrency，status archived，tasks 全 [x] + 完成记录
  来源：.agents/skills/welang-archive-sync/SKILL.md。
  验证：validate --all --strict 绿（归档态）。
- [x] 记忆更新 + 提交（英文提交信息、无署名尾注、refr/ 不入库）
  来源：常设授权「按列表顺序依次执行」。
  验证：git log + git show --stat 复核。

## 完成记录

2026-09-04 实施完成并归档。全部任务勾选；七点实现审查通过（见 proposal.md 实现审查记录）；验证阶梯全绿（validate --all --strict、docs_sync --check 25 对）。最终提交紧随归档。
