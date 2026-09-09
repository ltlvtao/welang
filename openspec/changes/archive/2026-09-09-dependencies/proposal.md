# proposal — dependencies（M13）

## Why

roadmap M13 行：Chapter 22 MVS resolution, `we.lock`, the dependency cache, a local registry fixture。章文 2026-09-04 已落地全权威（2200-dependencies.md，7R/24S，归档 2026-09-04-dependencies，提交 c5efd3a），E2001–E2006 注册表在册；其中 E2004（own version 语义版本形）已随 M4 落实现（check.go validVersion + 黄金 check-e2004-version），其余五码零实现——非空依赖集至今停边界行（check.go:236 `whatNonEmptyDeps`，黄金 check-deps-nonempty 钉 exit 70）。conformance 674 全绿下，依赖是最后一章「声明面已读、语义面全停」的语言面。本变更不改变语言行为——纯实现，规范零增量，`docs/spec/` 零触碰（internals-only 豁免路径，M0 起先例）。

## 目标与非目标

目标（四件，一条垂直线）：

1. **manifest 面**：`[dependencies]` 条目真读——键 = 包名（ch1 约定小写字母数字连字符 + `std` 保留不可声明，违者 E2006）；值 = 四形约束（`^ ~ >= =` 各一拼写、操作数合法语义版本，违者 E2003）。
2. **解析（最大下界 MVS）**：图 = 根清单 + 各已选包在选版处的清单；每约束贡献下界、每包取 max、新包清单可加新下界——迭代至不动点；序独立 + 注册表态独立；天花板违反或选版不存在 → E2002（报文携包名/选版/违反约束与依赖链）；无源应答的名字 → E2001；一包一版本（无版本分裂）；`std` 不入解析。
3. **锁 `we.lock`**：机器写 TOML（`[[package]]` name/version/digest，传递闭包）；满足图即真——锁名版本即构建用版本（离线复现机制）；不满足（约束变/图越闭包）→ 重解析 + 重写；畸形（非 TOML/缺条目）→ 静默再生零诊断；缓存内容摘要与锁条目失配 → E2005。
4. **获取与缓存 + loader 第三腿**：六个管线命令（build/check/run/test/vet/doc）先解析获取后模块解析（fmt/clean 不解析——现状即合规）；缓存答非本地 import（ch15 优先级字面维持：`src/` 下解析到的本地模块胜、`std.` 内建）；`we clean` 不触缓存与 we.lock（现状即合规——结构直证）。

非目标：

- registry/publish 服务面（ch22 R6 指名留白：服务器协议/搜索/账号认证/命名蹲位政策/`we publish` 皆不在）；镜像与代理配置；缓存布局之外的逐出策略；vendoring。
- 网络获取——参考实现的获取 = 本地目录复制（「acquisition 的网络协议是机制」的参考取值，见裁决 Q2/Q3）。
- 性能承诺（ch22 R6 明文：延迟/带宽/存储零承诺）。

## What Changes

- 新 `internal/deps` 包（包名定夺见 design）：version（三分量数值比较）、constraint 四形（floor/ceiling 语义表）、manifest 依赖表读取校验（E2006/E2003）、MVS 解析器（不动点 + E2001/E2002 依赖链渲染）、`we.lock` 读写（满足图判定 + 畸形再生 + digest E2005）、缓存获取（目录复制 + sha256 确定性序列化）。
- `internal/cli`：`loadManifest` 尾部非空依赖集边界删（`whatNonEmptyDeps` 死行），原位接入「解析 → 获取 → 摘要验证」序（六管线命令经共享 loader 面一次接线）；`loadGraph` 第三腿——import 首段命中声明依赖包名时经缓存路径解析（`<cache>/<pkg>/<version>/src/<rest>.we`），`src/` 本地优先与 `std.` 内建两腿字面不动、非本地非声明维持 E1302 既有报文（报文本就写「add the dependency to the cache」）。
- `internal/conformance`：Case schema 增 env 面（Execute 运行期设置/恢复）——黄金可控 WE_REGISTRY/WE_CACHE 指向 case 内夹具目录（裁决 Q4）。
- 黄金：check-deps-nonempty **计划改写 1 枚**（边界 70 → 真面；其源约束 `"1.2.3"` 无操作符实为 E2003 面——M5 bnd-ch8 改写先例）；新增预计 ~22（六码负例 + 绿路径端到端 + 锁生命周期四态 + 本地胜缓存 + clean 不触锁）。

## 影响层

- spec：零（章文已权威；零规范增量零新码零新 ADR）。
- compiler：cli（loader 序 + loadGraph 第三腿）+ 新 internal/deps 包；parser/typecheck/codegen 零触碰——依赖模块经既有 parse/typecheck 机器入图，无特权路径。
- tooling：conformance runner env 面（Case schema + Execute）。
- process/docs：roadmap 双语 M13 行（归档期）。

## 影响范围

- 涉触码：`internal/cli/check.go`（loadManifest 接线 + loadGraph 第三腿 + 边界删）、`internal/cli/build.go`（whatNonEmptyDeps 常量删）、`internal/cli/test.go`/`doc.go`（共享面零改或极小）、新 `internal/deps/` 包、`internal/conformance/runner.go`（env 面）。
- 新测试件：internal/deps 包单测（version/constraint/resolve/lock/acquire 五面）+ cli m13 单测 + conformance 黄金 ~22。
- 既有黄金改写恰 1（check-deps-nonempty）；其余 673 枚零触碰（负向对账：依赖面无其他黄金钉边界形）。

## 裁决面（candidate 阶段 AskUserQuestion）

- Q1 变更范围：一变更全量（manifest + 解析 + 锁 + 获取 + loader 第三腿）vs 拆两片（解析+锁 / 获取+loader）。
- Q2 registry 夹具机制：`WE_REGISTRY` 环境变量指本地目录树（`<name>/<version>/{we.toml, src/}`、版本枚举 = 列目录、未设 = 空源宇宙）vs 单文件索引 vs 其他。
- Q3 缓存与获取：`WE_CACHE`（默认 `~/.cache/we/packages`）+ 获取 = registry→cache 目录复制 + digest = sha256 确定性序列化（排序路径+内容）渲染 `sha256-<hex>`；锁满足且缓存摘要合 → 零源触碰（真离线）。
- Q4 conformance 夹具面：Case schema 增 env 面（黄金可控源/缓存目录）vs 黄金骑默认全局路径。

## 裁决记录（candidate 阶段，2026-09-09，四项均采纳推荐）

1. **Q1 一变更全量**：manifest 捕获 → MVS 解析 → we.lock → 获取/缓存 → loader 第三腿是一条垂直线——拆两片则片 1（解析+锁）无获取喂不了 loader、片 2（获取+第三腿）无锁无版本可读，互相悬空且片 1 无法端到端验证。量级与 M6a/M11/M12 相当，既有先例全量过。
2. **Q2 WE_REGISTRY 目录树**：`WE_REGISTRY` 指本地目录，`<name>/<version>/{we.toml, src/…}`；版本枚举 = 列目录名；未设 = 空源宇宙（一切依赖名 E2001）。零服务器、conformance 可用 case setup 复刻夹具、真机即真实可用——「获取协议 = 机制」下目录即 registry 是最诚实参考取值。单文件索引被拒（索引与内容可失同步多一层间接）；内嵌测试专册被拒（真机无源可用、E2001 恒真）。
3. **Q3 WE_CACHE + 复制 + sha256**：缓存 = `WE_CACHE` env，缺省 `os.UserCacheDir()/we/packages`（跨项目共享层——ch22「we clean 不触缓存」的语义预设）；获取 = registry→cache 目录复制；digest = sha256 对确定性序列化（相对路径排序 + 内容逐个折入）渲染 `sha256-<hex>`；锁满足且缓存摘要核对通过 = 零 registry 触碰（真离线，CI 性质兑现）；缓存摘要 ≠ 锁条目 → E2005。缓存住项目内被拒（违背跨项目共享语义、conformance 每例重获取）。
4. **Q4 Case schema 增 env 面**：Case 增可选 `env` map（Execute 设入/恢复），黄金把 WE_REGISTRY/WE_CACHE 指向 case setup 内夹具目录——六码全套黄金可控、封闭、不触真机全局缓存。骑默认全局路径被拒（测试不封闭、E2005 篡改面要动真缓存）。

## 审计记录（2026-09-09，welang-spec-impact-audit 7 条，通过；status → ready）

① 问题真实性：工程缺口可验证——六个管线命令对非空 [dependencies] 今日 exit 70（check.go:236 `whatNonEmptyDeps` 边界行，黄金 check-deps-nonempty 钉边界形），章文 docs/spec/2200-dependencies.md（7R/24S，归档 2026-09-04-dependencies，提交 c5efd3a）全权威 + diagnostics.toml E2001–E2006 在册（E2004 已随 M4 落 own version 面，其余五码零实现），roadmap M13 行（docs/roadmap/0000-reference-implementation.md:33）指名本切片。② 影响层：change.yaml layers [compiler] 与影响层节一致（conformance runner env 面是 internal/ 测试基建，M12 同形先例——layers 只列 compiler）；「不改变语言行为」边界在 Why 末句写明。③ 规范增量范围：零新增/修改/删除 Requirement、零新码（五码在册落实现、E2004 既有面维持）——对照 docs/spec/ 与 active 变更零冲突。④ 原则一致性：纯实现；MVS「picks = manifests 的函数」即 P1 局部可判定的解析面兑现、锁离线性质即 P8 可重复构建；无原则突破。⑤ 参考基线：仅引仓库内正典（2200-dependencies.md / 2100-toolchain.md R8 R12 / 1500-modules.md R1 / diagnostics.toml / roadmap），refr/ 零引用。⑥ 验收边界：四件目标各可机械判定（E 码黄金矩阵 21+1、we.lock files 断言、绿路径端到端、离线黄金钉）；非目标五条防蔓延（registry/publish 服务面、网络获取、vendoring、逐出策略、性能承诺）。⑦ 粒度：manifest 捕获 → 解析 → 锁 → 获取 → loader 第三腿一条垂直线单层 compiler，Q1 裁决在案（拆两片互相悬空）。通过。

## 审查记录（2026-09-09，welang-change-review 10 条，通过；status → active）

先跑 `validate.py dependencies --strict` 结构过。① proposal 黑盒性：Why/目标非目标为黑盒面；What Changes 的 internals 描述为 internals-only 实现变更的交付面（M11/M12 同形先例）。② spec 增量：无 specs/ 目录——豁免路径（proposal「不改变语言行为」标记，validate 已认）。③ design 唯一最小路径：D1–D9 唯一；被拒替代方案在裁决记录（拆两片/索引文件/内嵌专册/项目内缓存/全局路径黄金）；引用精确到行号（check.go:236、build.go:23）、注册表条目与章文场景逐字（E2002 链三要素、离线场景、local-wins 场景）。④ tasks 各项有来源与验证，无 deferred/non-goal 项。⑤ 场景覆盖：D8 矩阵 21 枚含正（绿路径/锁生命周期/传递 MVS）/负（六码）/边界（local-wins、fmt/clean 不解析、离线）三路。⑥ 无空章节。⑦ 测试先行：T1 黄金先红（env 面 = 测试基建先行）+ T2 单测先红。⑧ 负向断言：六码各有负例黄金 + T1 红因分类 + 既有 674 负向对账 + 边界黄金 check-deps-nonempty 计划改写（边界 70 面的消亡以改写件钉死）。⑨ 完成度闭环：工具链管线类变更——解析（D3）/锁（D4）/获取（D5）/接线（D6）四要素齐，codegen/runtime 零触碰（依赖模块经既有机器入图）。⑩ 未决问题：无——四裁决已定，registry/publish 非目标已披露（ch22 R6 指名留白）。审查期修正一处：D8/T1/T6/T8 计数算术（e2003-bare 即改写件 → 20 新增 + 1 改写 = 21 涉及、总数 694），已同步。通过。

## 实现审查记录（2026-09-09，welang-code-review 7 条，通过；status → complete）

① 规范符合性：ch22 六 R 逐面对上——R1 声明面（E2006 双形/E2003 四形/E2004 既有面，loadManifest 校验位）、R2 四形天花板无零特殊例（^0.1.0 ceiling <1.0.0，披露 5 定谳）、R3 MVS（max-floors/E2001/E2002 双形/一包一版本/std 不入解析）、R4 锁与缓存（机器写形/畸形静默再生/本地胜缓存/ch15 优先级/离线性质）、R5 子命令面（六管线命令解析、fmt/clean 豁免）、R6 clean 姿势（缓存缺省外置项目外 UserCacheDir/we/packages）；E2001/E2003/E2006/E2005 消息首短语与注册表 title 逐字对齐（抽查）。② 验证诚实性：T1–T8 勾选与实跑对上——conformance 695 计数（-v 逐枚）、deps 14 + cli 7 测试函数、电池 17/17、九步全绿皆本日复跑输出。③ 测试先行：T1 黄金先红（674 PASS + 21 FAIL、红因单类 exit 70）+ T2 单测先红（undefined 符号）；绿点随塔序推进（披露 3）；黄金 §52 单行人类面抽查无 help 泄漏；runner env 面为测试基建（D7）。④ 诊断协议：Case 增 env 纯增量（omitempty、既有 674 件零破坏）；零新码零重命名——五码在册落实现。⑤ 单一权威：digest 序列化/锁机器形/MVS 算法/e2005 双触发序注释住代码（英文）；helps 注册表逐字；docs/spec/ 零触碰。**发现处置（D13）**：change-review 期修正句计数算术错（「20 新增 + 1 改写 = 21 涉及、总数 694」——e2003-bare 实为新增件、改写件 check-deps-nonempty 原位不计增；权威计数 = 21 新增 + 1 改写 = 22 涉及、总数 695，与 D8 定数及实测一致），design D9 终验行的同错残留已修（694 → 695），该修正句作废、以本记录为准。⑥ 红线：refr/ 零触碰；未提交（候用户明示）；m10c/m11 测试签名跟随与 advisory/build/vet/doc 四点透传为接线必要面；ffi 归档追加句为 M12 提交对账先例形。⑦ 最小可信验证：T8 九步与 welang-pre-push-checks 选取标准一致全跑过。实现期 T6 另出黄金夹具限定形笔误（披露 6，七件修正）——ch15 R2 权威、规范零触碰。通过。
