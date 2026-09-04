# 任务清单：FFI——`foreign "c"`（第 19 章）

## 1. 候选与四件套

- [x] 建立 `openspec/changes/ffi/`（change.yaml：candidate、layers [spec]、created）
  来源：用户裁决四项；本变更提案 Why。
  验证：`python3 openspec/tools/validate.py ffi --strict` 通过（候选期容许未注册码引用）。
- [x] proposal.md（Why=五处宿主悬句+ADR-0002 实锚；What Changes=新章 9 块 + 3 宿主修订 + 回调缺口）
  来源：v0.8 §47/§47.1、ADR-0002、四项用户裁决。
  验证：人工对照 ch0/ch1/ch6/注册表段注释原文，逐一可在文件中 grep 到。
- [x] design.md（四裁决 + D1–D12 设计决定 + v0.8 码映射 + P1 复核）
  来源：裁决记录与本章设计推演。
  验证：每条偏离（段必写+裸段、仅参数位、E1702 专用码、E1707 空真关死）均有先例或原则锚。
- [x] specs/ 增量（英文，docs/spec 字面片段）：ffi 新章 + lexical/declarations 宿主
  来源：What Changes 逐条。
  验证：`python3 openspec/tools/validate.py ffi --strict` 通过。

## 2. 就绪门（7 点 welang-spec-impact-audit）

- [x] 七点审计全过并出 ready 报告（逐规则代码示例记录）
  来源：.agents/skills/welang-spec-impact-audit/SKILL.md。
  验证：报告落变更目录，七点逐项有结论。

## 3. 激活门（10 点 welang-change-review）

- [x] status→active 前通过 10 点审查，发现项修复回灌增量
  来源：.agents/skills/welang-change-review/SKILL.md。
  验证：审查记录落 proposal.md 审查记录段。

## 4. 实施（实施负例先行）

- [x] 注册表先行：diagnostics.toml 拆段 E1700–E1799 owner 1900-ffi + E1701–E1707 七码条目 + E1406–E1499 段注释刷新（注入哨兵 E1799 于宿主章实测 FAIL 后还原——负例必须 `--all --strict` 跑）
  来源：注册表扩展义务（AGENTS.md 规则 2）。
  验证：`python3 openspec/tools/validate.py --all --strict` 注册表检查绿。
- [x] 新章 `docs/spec/1900-ffi.md`：9R/37S + 示例节（H3 子节）+ 术语对照
  来源：specs/ffi delta 逐字提升。
  验证：逐字 diff 零差异；标题邻接扫描 0 违例。
- [x] zh 孪生 `1900-ffi.zh.md`：H1 第 19 章、术语对照、代码块与 EN 字节一致（注释英文）、全角冒号 E1xxx 扫描 0
  来源：双语同步义务。
  验证：docs_sync 结构配对 + R/S 计数镜像机检。
- [x] 宿主章 EN+zh 拼接（归一化 rstrip+双换行，3 处 Requirement）：ch1 关键字 `foreign` 兑付句；ch6 File structure 枚举+场景；ch6 Function declarations 末句刷新
  来源：specs/ 两 delta。
  验证：提升文==delta 逐字 diff（3/3）；宿主场景计数机器比对 git HEAD（无意外丢失）。
- [x] D14 刷新：ch6 示例注释 foreign 悬句改写（EN+zh）
  来源：D14 纪律。
  验证：grep "mut parameters and foreign blocks" 归零。
- [x] 一码一消息扫描：变更新增行 + 新文件全量，反引号码形与注释码形 `EXXXX:` message 与注册表 title 逐一相符
  来源：welang-code-review 习惯项。
  验证：扫描脚本 0 失配。
- [x] docs_sync 25→26 对 + `--check` 绿
  来源：双语同步义务。
  验证：`python3 openspec/tools/docs_sync.py --check`。

## 5. 完成门（7 点 welang-code-review）与归档

- [x] 7 点代码审查（规范符合性/验证诚实性/测试先行证据/诊断协议稳定/单一权威/红线复核/最小可信验证）+ 实现审查记录入 proposal.md
  来源：.agents/skills/welang-code-review/SKILL.md。
  验证：七点逐项结论 + 披露段。
- [x] 归档 `git mv` → archive/2026-09-04-ffi，status archived，tasks 全 [x] + 完成记录
  来源：.agents/skills/welang-archive-sync/SKILL.md。
  验证：validate --all --strict 绿（归档态）。
- [x] 记忆更新 + 提交（英文提交信息、无署名尾注、refr/ 不入库）
  来源：常设授权「按列表顺序依次执行」。
  验证：git log + git show --stat 复核。

## 完成记录

2026-09-04 全周期完成：candidate（四件套+9R/37S 增量+examples）→ ready（7 点审计，锚点 6/6 实证）→ active（10 点审查 4 发现修复：导入不透明类型到达、foreign 名值位 E0105 钉死、块内条目集负例、R4 MUST+Never 参数位；34→37 场景）→ 实施（哨兵 E1799 注入 FAIL 先行；注册表 124→131/19 段；19 章 EN+zh 提升，逐字 9/9、代码块 6/6 字节一致；宿主 ch1/ch6 ×2 语言拼接逐字 3/3、场景计数 HEAD 机比恰为既定差；D14 归零；docs_sync 26 对）→ complete（7 点代码审查零发现）→ archived。ADR-0002 决策 4 即载体决策之权威，无新 ADR（spec 内容权威在章，先例）。提交另记。
