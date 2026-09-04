# Proposal: 依赖获取（第 22 章）

## Why

宿主规范为本章预留的承接点全部可 grep 实证，缺口在上一片被显式指名：

1. **第 15 章的让渡句**（Module paths and file resolution）："acquisition, version constraints, and lockfiles are the tooling layer's business, not this chapter's"——本章就是那个 tooling layer；句中已固定的缓存优先级（本地 `src/` 胜、`std.` 保留、其余走缓存同映射）原样承继，零修订。
2. **第 21 章的唯一 registered gap**（What the toolchain does not fix）："Dependency acquisition — the manifest's `[dependencies]` table, version-constraint syntax, lockfile format, and any `install` or `publish` command — is not designed here … honestly named as this chapter's one registered gap"——缺口由本变更填上，该 Requirement 必须修订。
3. **第 21 章 R8 的骨架留白**（The project manifest）："the manifest's `[dependencies]` table belongs to the change that designs it" 与场景 "this specification fixes nothing about it"；同处 "`version`, a string whose shape is the ecosystem's to constrain" ——本章既是生态层，version 形状在此兑付。
4. **v0.8 §65 优先级高第 3 条**（`refr/spec-0.8.md`）："包仓库与版本解析：`we.toml` 的 `[dependencies]` 版本约束语法与解析算法、`we.lock` 锁文件格式、`we install`/`we publish` 子命令语义的完整设计"——指名四件套。
5. **v0.8 §56 规则 2**：`[dependencies]` 键=包名（小写+数字+连字符）、值=版本约束串，四写法 `^`/`~`/`>=`/`=`，semver 生态借用的唯一性例外论证在案。
6. **v0.8 错误表**：`E0111` unknown external package（随获取机制延后至今）待承接；`E0151` project version must be valid semver 被 ch21 有意搁置（推给生态层）——两条在本章重编号落册。
7. **注册表段位**：`E2000`–`E9999` unclaimed——本章认领 `E2000`–`E2099`，未认领区间收窄为 `E2100`–`E9999`。

## What Changes

1. **新章：第 22 章 Dependencies**（`docs/spec/2200-dependencies.md` + `.zh.md`），七条 Requirements：
   - Dependency declarations —— `[dependencies]` 表：可选、缺省=空集；键循第 1 章命名约定且 `std` 不可声明（`E2006`）；own-`version` 兑付为 semantic version（`E2004`）；
   - Semantic versions and constraint meanings —— 版本=三段无前导零整数（无 prerelease/build，刻意不载）；约束恰四形（`^` 同 major、`~` 同 minor、`>=` 无上界、`=` 恰等），每形一义一拼法（唯一性例外承继）；越形 `E2003`；
   - Resolution by the maximum of floors —— MVS：每包取全图下界最大者为唯一版本；序无关、registry 状态无关（新版本出现不改任何解析直到约束下界要它）；违反上界或所求版本不存在 `E2002`；无名应答 `E2001`；`std` 永不入解；
   - The lockfile —— `we.lock` 机器写、项目提交；传递闭包每包精确版本+内容摘要；满足即胜（离线可复现）；失满足重解重写；损坏静默重生成；摘要失配 `E2005`；
   - Acquisition and the cache —— 凡跑检查管线的子命令先解析后获取；`we fmt`/`we clean` 不解析；缓存位置/布局/驱逐/网络协议=机制不固定；缓存满足则零网络（CI 可复现性）；**零新子命令**（ch21 命令面原封不动）；
   - What acquisition does not fix —— registry 与 publish 故事=本章唯一 registered gap；镜像/代理/缓存布局=机制；vendoring 不固定；`we clean` 不动缓存与锁文件；获取性能零承诺；
   - The dependencies diagnostics segment —— `E2000`–`E2099` owner `2200-dependencies`，六错误码 `E2001`–`E2006`，保留号无字母书写。
2. **注册表**：拆段 `E2000`–`E2099` owner `2200-dependencies`（`E2100`–`E9999` 续 unclaimed）；新码六条（全 error）；条目 147→153、段 21→22；E1406 段注随落段刷新。
3. **宿主修订：恰两处**（均在第 21 章）——R8 The project manifest（version 形状句兑付 + 尾句改指第 22 章 + "named gap"场景重写为 "Dependencies are chapter 22's"）、R11 What the toolchain does not fix（获取句整句移除 + 对应场景移除）；另两处示例块注释随真值刷新（manifest 块与 pending-later-changes 块）。第 15 章零修订（让渡句为真前瞻指针）、第 21 章 R1 零修订（零新子命令）。
4. **v0.8 映射**（详见 design.md D10）：约束语法→R2 四形；解析→R3 MVS；`we.lock`→R4；`we install` **消亡**（裁决三：隐式获取）；registry/`we publish`→显式 named gap；`E0111`→`E2001`、`E0151`→`E2004`（从 own-version 扩为任何 version 值）；`E0153` we-version **续死**（ch21 已拒）、`E0154` 闭合 schema **续死**（软姿态承继）；prerelease/build 语法**不载**（刻意收窄，design 披露）。
5. **双语文档**：`docs_sync` 28→29 对。

## 影响层

- **spec 层**：新章 22 + 注册表扩展 + 第 21 章两处 Requirement 修订，经 openspec 变更流程（本变更）。
- **编译器/运行时层**（不在本变更）：解析器实现、缓存管理、锁文件读写、摘要校验——本章只固定可观察契约。
- **生态层**（不在本变更）：registry 服务协议、publish 流程、账号与命名治理——显式 named gap。

## 影响范围

| 文件 | 动作 |
| --- | --- |
| `docs/spec/2200-dependencies.md` | 新建（第 22 章正文） |
| `docs/spec/2200-dependencies.zh.md` | 新建（中文镜像） |
| `docs/spec/2100-toolchain.md` | 修改（R8/R11 两 Requirement + 两示例块注释） |
| `docs/spec/2100-toolchain.zh.md` | 修改（同两处镜像） |
| `docs/spec/diagnostics.toml` | 拆段 + 6 新码 + E1406 注刷新 |

## 裁决记录

用户裁决四项（2026-09-04，均采推荐）：

1. **约束+解析+锁+获取** —— 四件套中三件入章；registry 服务器协议与 `we publish` 显式留白（生态基础设施需运营现实，沿 LSP 先例）。
2. **四写法 + semver 定形** —— `^`/`~`/`>=`/`=` 四形（v0.8 唯一性例外论证承继）；版本形状定为 semantic version，own-`version` 字段同步兑付（v0.8 `E0151` 复活为 `E2004` 并扩义）。
3. **零新子命令** —— 项目命令隐式解析+获取+写锁；升级=手改约束再跑命令；ch21 封闭子命令集零修订。
4. **MVS 最大下界** —— Go 式：每包全图恰一版=下界最大者；按构造无冲突；违反上界/版本不存在才 `E2002`；序无关且 registry 状态无关。

## 目标与非目标

### 目标

- ch21 唯一 registered gap 填上：声明语法、约束语义、解析算法、锁文件、获取行为一站式固定为可观察契约。
- 确定性两连：解析序无关 + registry 状态无关；锁满足即胜 + 摘要完整性。
- 宿主修订最小且必要：只改语义已变假的句子（ch21 R8/R11），其余指针句逐句复核为真。
- 注册表扩展守 E/W 共号 space 与一码一义（`E0111`/`E0151` 重编号入册）。

### 非目标

- registry 服务协议、publish、账号、命名治理 —— 显式 named gap，待专门变更。
- 镜像/代理配置、缓存布局与驱逐、网络协议 —— 机制，不固定。
- vendoring —— 不承诺不禁止不定义。
- 任何获取性能（延迟/带宽/存储）预算 —— 评测层的事。
- 编译器/工具链实现 —— layers: [spec]。

## 审查记录

（10 点审查后填写）

## 审计记录

2026-09-04，七点审计（welang-spec-impact-audit），结论：**通过**。

1. **问题真实性 ✓**：Why 九处锚点逐一 grep 实证——ch15 R1 让渡句（acquisition/version constraints/lockfiles 归 tooling layer）、ch21 R11 唯一 registered gap 句、ch21 R8 两留白句（`[dependencies]` 归"设计它的那个变更"+ version 形状归生态层）、v0.8 §65 高优 3（四件套指名）、§56 规则 2（键/值/四写法）、错误表 E0111/E0151 两行、注册表待拆段头 `E2000-E9999`，全部 1 hit。缺口是 ch21 显式注册的规范留白，黑盒可判定。
2. **影响层 ✓**：change.yaml `layers: [spec]` 与 影响层 一致；编译器/运行时实现与生态（registry/publish）边界在 影响层 与 非目标 双处写明。
3. **规范增量范围 ✓**：新增 7 Requirement（第 22 章）+ 修改 2 Requirement（ch21 R8/R11，均语义变假的必改句）；新码 E2001–E2006 六条，grep 证实 `E2001`–`E2006` 在 docs/spec 正文与注册表零占用（唯一命中为待拆段头）；段位拆分 E2000–E2099（owner 2200-dependencies）+ E2100–E9999（unclaimed）遵守 ch99 共号 space 规则（本段无 W 码，保留号无字母书写）；E1406 段注随落段再刷新（沿 slice 5/6 先例）。无与既有规范冲突（ch15 缓存三优先级原样承继、ch21 管线"模块解析阶段"先于获取的时序一致、ch20/ch21 探索确定性在缓存定型后开始计时不冲突）、无与 active 变更重复（当前唯一 active 即本变更）。
4. **原则一致性 ✓**：P4——MVS 单规则无特例（cargo `^0` 特例不载）、四形四义各一拼法；P7——锁机器写零配置、获取失败循 ch21 R7 JSON Lines 字段集零新字段；P8——registry/publish 指名留白、prerelease/build 不载明言边界、获取性能零承诺、"缓存满足则零网络"是行为承诺非性能承诺；机制中立——摘要算法/缓存布局/网络协议不固定；P5/三要素——design D11 账齐（类型检查零参与、代码生成零新面、运行时三件事全工具层）。无原则突破项，无需 ADR。
5. **参考基线固定 ✓**：v0.8 引用固定为 §65（高优 3）、§56（规则 1/2/6）、§3.1（唯一性例外）、错误表 E0111/E0151 行；对照原文逐条核对；映射账 D10 逐条可追。
6. **验收边界 ✓**：目标四条可机械判定（7R 逐字提升、六条注册表条目 tomllib 机检、两宿主 Requirement 与 delta 逐字一致+其余与 HEAD 逐字一致、docs_sync 29 对）；非目标排除 registry/publish/镜像代理/vendoring/性能/实现，蔓延面已封。
7. **粒度 ✓**：单一 spec 层变更，一章 + 注册表 + 两处宿主修订，垂直可验收，与 testing/toolchain 变更同形。
## 审查记录

2026-09-04，10 点语义审查（welang-change-review），结论：**通过（3 发现，均已修复回灌增量）**。

1. **proposal ✓**：黑盒缺口+目标；四裁决与映射账分层清晰；实现决策全在 design（D1–D12）。
2. **spec 增量 ✓**：全部为可观察契约（检查判定、解析结果函数、锁字段与胜出规则、失败码）；MUST/MAY 合 BCP 14；MVS 以"结果函数+不动点良基"陈述可观察结果、不固定遍历序。修复 F1：R4 锁"满足图"原为未定义短语——补精确判定（图达每包有锁版、锁版皆在源、图内全部约束——根的与各锁版自身 manifest 的——被锁版满足），"失满足"分支随之改写。
3. **design ✓**：唯一最小路径；被拒替代方案有录（D4 cargo 统一求解、D2 prerelease、D3 cargo `^0` 特例、D5 we install 子命令）；引用精确到 Requirement 与码。
4. **tasks ✓**：勾选项均有来源与验证；哨兵 E2099 注入为负向验证；无 deferred/未决。
5. **场景覆盖 ✓**：normal/boundary/failure 三路齐——R1 空表与双拒绝、R2 越形与坏操作数、R3 上界违例与无名应答、R4 摘要失配与损坏锁、R5 零网络与命令面拒绝。
6. **无空章节 ✓**：7 Requirements 均有行为增量；R6/R7 载实质承诺（named gap 指名义务、保留号规则）。修复 F2：R6 clean 场景原以解释性口吻引述第 21 章句子（"carries the lockfile with it"）——改为本章直说（缓存属机制、锁是项目记录非产物），不留跨章解释。
7. **测试先行 ✓**：spec 层等价物 = 哨兵负例（E2099 → `--all --strict` FAIL → 还原），任务 4.1 已列；一码一消息扫描任务在列。
8. **负向断言 ✓**：六码各有拒绝/失败场景与示例形（E2006 键越约与 std、E2003 越形与坏操作数、E2004 坏版本值、E2002 上界违例、E2001 无名、E2005 摘要失配）；"版本分裂不存在"与"零新子命令"两条负向禁令各有场景承载。
9. **完成度闭环 ✓**：D11 三要素账在位——类型检查零参与（六码全工具层）、代码生成零新面、运行时三件事（解析器/缓存管理/锁读写与摘要校验）全为契约下实现自由。
10. **未决问题阻塞 ✓**：无行为级未决——registry/publish 是显式 named gap（R6 指名+非目标排除）、镜像/代理/vendoring 同为已决留白。修复 F3：宿主修订 R8 的 version 句残留元叙述（"the shape chapter 21 left to the ecosystem layer, now fixed there"）——删元从句，规范文本只留现状。

## 实现审查记录

2026-09-04，七点实现审查（welang-code-review），结论：**通过（0 行为发现；2 检查工具披露）**。

1. **规范符合性 ✓**：第 22 章为 delta 逐字提升（7R/24S，行级包含 0 缺失；标题邻接剥围栏 0 违例）；注册表六条目与 R7 逐字对账（六码 title 在 R7 文本以 `` `CODE` title`` 形式在场、severity 全 error、owner/allocated 齐整、requirement 字段全部指向章内真实 Requirement 标题）；段位声明一致（`E2000`–`E2099` owner `2200-dependencies`、`E2100`–`E9999` unclaimed）；E1406 段注第四次刷新。宿主两 Requirement 修订块逐字在场（EN），zh 镜像句对句替换；ch15 让渡句落地后原样在场（前瞻指针依然为真）。
2. **验证诚实性 ✓**：tasks 4 节六项验证命令逐个实跑——哨兵负例（E2099 注入 1500-modules.md → `--all --strict` FAIL exit=1 → 还原 → 绿，先于注册表落地）、tomllib 计数 153/22、包含/邻接扫描、宿主修订块逐字核 + `git diff` 全量人工复核（五处改动恰为预期：R8 两句+场景、R11 一句+场景、两示例块）、zh 三查（7/24 镜像、9 围栏字节同一、全角冒号 0）、一码一消息扫描（七目标含 ch21 宿主双语 0 失配）、docs_sync 29 对。
3. **测试先行证据 ✓**：spec 层等价物 = 哨兵负例，时序在注册表扩展之前（FAIL 实测）；六码示例形齐（E2001–E2006 各有拒绝/失败样例）；锁文件示例块与 R4 字段集逐字段一致。
4. **诊断协议稳定 ✓**：六新码全局唯一（`--all --strict` 绿即机检）；E0111/E0151 重编号入册、一码一义两清（约束操作数归 E2003、版本值归 E2004）；获取失败循 ch21 R7 JSON Lines 字段集、零新字段零删除。
5. **单一权威 ✓**：章正文与触发语义唯一权威在 `docs/spec/2200-dependencies.md`（.zh.md 镜像）；注册表只持条目、R7 明文分工；`[dependencies]` 语义从 ch21 的"既不拒也不定"单点移交 ch22——一处事实一处权威；变更目录只留过程工件随归档。
6. **红线复核 ✓**：`git status` 实证——修改恰三文件（2100 双语 + diagnostics.toml）、新增两章 + 变更目录，与 影响范围 表逐一对应，零越界（ch21 diff 五处全为预期）；refr/ 无变更；提交信息英文、无署名尾注。
7. **最小可信验证已跑 ✓**：`validate dependencies --strict`、`validate --all --strict`、`docs_sync --check`（29 对）、注册表 tomllib 机检、宿主零越界 diff 复核、ch21 陈旧句清除断言（EN+zh 六句 grep 为零）、ch15 指针在场断言——全部绿。

**披露**：

- **D-1（检查工具）**：E1406 注刷新断言首跑假失败——短语跨注释行换行（`the\n# dependency`），断言修正为单行短语匹配；文件内容零改动。
- **D-2（检查工具）**：zh 宿主修订不能走 EN 的程序化块替换（标题语言不同），改为句对句定点替换后以"修订块逐字在场"（EN）+"陈旧句清除"（zh）双面验证——zh 镜像的等价性由六句正反断言与 docs_sync 结构配对共同背书。
