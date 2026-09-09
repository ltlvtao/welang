# proposal — ffi（M12）

## Why

roadmap M12 行：第 19 章 foreign 块与构建时链接（E1906）。章文 2026-09-04 已落地全权威（1900-ffi.md，9R/37S，变更归档 2026-09-04-ffi），E1701–E1707 与 E1906 注册表在册零实现——parser 至今停 `bndFFI` 边界行（`chapter 19 (ffi) forms`，parser.go:423）。M0–M11 后语言 22 章的工具链面全绿（conformance 641），foreign 是唯一整章停在边界的语言面。本变更不改变语言行为——纯实现，规范零增量，`docs/spec/` 零触碰（internals-only 豁免路径，M0 起先例）。

## 目标与非目标

目标（三塔）：

1. **检查塔**（ch19 声明与检查全量）：parser `foreign "c" { items }` 产生式（E1701/E1702/E1703/E1704 + E0105 位 + E0404 + 裸 `effect` 零标签段仅块内合法）；typecheck 跨界集（E1705/E1706 分位）+ opaque 纪律（E1707 构造/更新拦截）+ byres opaque 走 ch13 既有机器零新规则 + 跨模块 pub opaque 经 ch15 既有门；效果零新机制（调用 = 普通调用 E1401 既有判序，裸段 = 空集早退）。
2. **codegen 塔**：foreign fn → IR declare（位置 ABI，String/Bytes 双参展开）+ 调用位直呼原生符号 + Never 发散 + opaque 单指针槽。
3. **链接塔**（ch21 linkage 可观察契约）：build/run 发现项目 `native/` 目录 `.c` → 钉版 clang 编译 `.o` 链入；llvm-nm 列 defined 符号对程序 foreign 名全集查证，缺者 E1906 逐声明报、exit 1、无工件；check/test/vet/doc 永不链接。

非目标：

- 回调（ch19 注册缺口：效果归属/运行时上下文/调用约定三问题无批准答案——E1705 签名位 + E0105 值位双面已关）。
- std.io 重写为 foreign 声明（B2 stdlib-real 面——ch19「标准库无特权语法通道」的实现态维持合成模块）。
- foreign fn 的 mock 面（ch20 E1804 面——M12 排除并披露，roadmap follow-up 登记）。
- 系统库链接、库名声明/搜索路径、mangling 方案（机制面 ch21 明留 toolchain own——本实现取 native/ 目录唯一源 + 原样名，见裁决）。
- String/Bytes 之外的编组生成、编组子（marshalling 是 We 侧显式代码——章文立场）。

## What Changes

- 检查塔：AST `ForeignBlock` + `FnDecl.Foreign`/`RecordDecl.Opaque` 标记；parser 顶层派发 foreign 行从 bndFFI 边界改真解析、块/语句位 foreign → E1702 专码、段解析加 `inForeign` 语境参数（裸段仅此合法）、名字表挂接（跨块/与顶层重名 E0404）；typecheck ingest（无体 fn 不查体、效果集 = 声明段走既有 resolveEffectTag）、跨界集检查器（参数位 8 整型/Float32/64/Bool/Rune/String/Bytes/opaque；Never 参数位含内 E1705；返回位减 String/Bytes 加 Never、专码 E1706 先报）、E1707 于构造/更新位先于 E0604/E0603、mock 目标解析到 foreign fn → E1804 排除。
- codegen 塔：`declare <ret> @<name>(<params>)`（跨模块同名声明合并同 declare）；调用位直呼（无槽——mock 非目标直证）；ABI 映射（Bool→i8、Rune→i32、String/Bytes→(ptr,i64) 双参展开、opaque→ptr、Never 返回→void+unreachable）；opaque 值单指针槽不入 GC root 位图（披露面）。
- 链接塔：native/ 发现（.c 字典序）→ 逐 `clang -c` 成 build/native-*.o → llvm-nm defined 符号集 vs AST 面 foreign 名全集差集 → 缺者逐条 E1906（`unresolved native symbol — {name} (declared at {file}:{line})`）全渲染后 exit 1 无 build/ 工件；native .o 追加最终链接实参；toolchainGate 扩查 llvm-nm（build/run 面）。

## 影响层

- spec：零（章文已权威——2026-09-04-ffi 归档；本变更零规范增量零新码零新 ADR）。
- compiler：parser/typecheck/codegen/cli 四层（涉触码清单见影响范围）。
- tooling：fmt 对 ForeignBlock 行保分类；doc 对 foreign 条目排除（pub-only 六类外——披露面）；vet/check 零改（advisory 面不涉 foreign）。
- process/docs：roadmap 双语 M12 行（归档期）。

## 影响范围

- 涉触码：`internal/ast/ast.go`（ForeignBlock 节点 + 两标记位）、`internal/parser/parser.go`（产生式 + 派发表 + 块内 E1702 + 段语境参数）、`internal/typecheck/typecheck.go`（ingest + 跨界集 + E1707 + mock 排除）、`internal/codegen/codegen.go`（declare/call/ABI + foreign 名收集）、`internal/cli/build.go`（native/ + llvm-nm + E1906 + gate 扩查）、`internal/fmt/fmt.go`（ForeignBlock 分类）、`internal/cli/doc.go`（零改——六类 switch 的 default 天然排除，测试钉死）。
- 新测试件：parser/typecheck/codegen/cli 四包 ffi/m12 测试文件。
- conformance 黄金新增预计 32（design D6 定数：check ffi 14 / 资源 4 / 效果 3 / never 1 / build 2 / run 4 / fmt 2 / doc 1 / vet 1）。
- 既有黄金零改写（bndFFI 行删除后 `we check` 对 foreign 块从 exit 70 变真诊断——无既有黄金钉 foreign 面，T1 负向对账）。

## 裁决面（candidate 阶段 AskUserQuestion）

- Q1 变更范围：一变更全量（检查塔 + codegen + 链接端到端）vs 拆两片。
- Q2 native 符号源：项目 `native/` 目录 `.c` 编译链入 vs 其他机制。
- Q3 符号名映射：声明名原样直通 C 符号（无 mangling）vs 前缀修饰。
- Q4 String/Bytes 参数 ABI：(ptr, len) 双标量参数展开 vs 单指针形。

## 裁决记录（candidate 阶段，2026-09-09，四项均采纳推荐）

1. **Q1 一变更全量**：foreign 声明无链接则无端到端验证——检查塔单独落地时 build 面仍停边界，黄金矩阵割裂；量级与 M6a/M11 相当（既有先例全量过）。拆两片的稳定性收益不抵端到端断链。
2. **Q2 项目 native/ 目录 .c**：零配置、全检视（.o 落 build/）、与 runtime 同钉版 clang 工具链；we.toml [link] 段被拒——ch21 R8 已封 [vet]/[test] 两段，机制变规范故事会外溢 E1905/E1903 面量级；.o 直链被拒——钉版纪律旁落。
3. **Q3 原样名直通**：ch21「one native symbol per foreign name」最直读；native/ 作者侧所见即所得；标准 C 名（abs/log）可直写。前缀修饰被拒——mangling 方案自造（ch21 明留不定的面反而被定死）+ 模块键泄漏进 C 侧契约。
4. **Q4 (ptr, len) 双参数展开**：We String 可含 NUL，单指针 + NUL 终止约定会静默截断（P1 违规面）；Bytes 的章文 (pointer,length) pair 形同构；长度显式、零隐藏编组。

## 审计记录（2026-09-09，welang-spec-impact-audit 7 条，通过；status → ready）

① 问题真实性：工程缺口可验证——`we check` 对 foreign 块今日 exit 70（bndFFI 边界行 parser.go:423），章文 docs/spec/1900-ffi.md 全权威 + diagnostics.toml E1701–E1707/E1906 在册，roadmap M12 行（docs/roadmap/0000-reference-implementation.md:32）指名本切片。② 影响层：change.yaml layers [compiler] 与影响层节一致（fmt/doc 触面皆 internal/ 编译器包，M11 同形先例）；「不改变语言行为」边界在 Why 末句写明。③ 规范增量范围：零新增/修改/删除 Requirement、零新码（八码在册落实现）——对照 docs/spec/ 与 active 变更零冲突。④ 原则一致性：纯实现；Q4 双参展开即「零隐藏编组」的机制兑现；native 侧契约披露面 = P8 边界诚实；无原则突破。⑤ 参考基线：仅引仓库内正典（1900-ffi.md / 2100-toolchain.md R12 / diagnostics.toml / roadmap），refr/ 零引用。⑥ 验收边界：三塔目标各可机械判定（E 码黄金 + E1906 + run 端到端 stdout）；非目标五条防蔓延。⑦ 粒度：check→build→run 一条垂直线单层 compiler，Q1 裁决在案。通过。

## 审查记录（2026-09-09，welang-change-review 10 条，通过；status → active）

先跑 `validate.py ffi --strict` 结构过。① proposal 黑盒性：Why/目标非目标为黑盒面；三塔描述为 internals-only 实现变更的交付面（M11 同形先例）。② spec 增量：无 specs/ 目录——豁免路径（proposal「不改变语言行为」标记，validate 已认）。③ design 唯一最小路径：D1–D9 唯一；被拒替代方案在裁决记录（[link] 段/前缀修饰/单指针形）；引用精确到行号与注册表条目。④ tasks 各项有来源与验证，无 deferred/non-goal 项。⑤ 场景覆盖：D6 矩阵 32 枚含正/负/边界/端到端四路。⑥ 无空章节。⑦ 测试先行：T1 黄金先红 + T2 单测先红。⑧ 负向断言：八 E 码各有负例黄金 + T1 红因分类 + 既有 641 负向对账。⑨ 完成度闭环：类型检查（D2）/代码生成（D4）/运行时与链接（D5 + native 夹具）三要素齐。⑩ 未决问题：无——四裁决已定，mock 非目标已披露（D9-6）。通过。

## 审查记录（2026-09-09，welang-code-review 7 条，通过；status → complete）

① 规范符合性：行为面全数对照 1900-ffi.md/2100-toolchain.md——八 E 码（E1701–E1707/E1906）跨界集/构造/mock 排除/链接查证各有黄金钉、四裁决（Q1 全量/Q2 native//Q3 直通/Q4 双参展开）均可溯章文（审计记录⑤已核）；黑盒电池七探针 + run 四形真跑（five/first-h/exit 7/unboxed）端到端直证；无规范外行为（we.toml [link] 段被拒未引入、mangling 未引入）。② 验证诚实性：T1–T8 逐项复核——本审查会话亲自重跑 T7 电池（七探针）与 T8 阶梯（九步全绿）；T1–T6 记录带红因分类/翻绿计数/三枚实现期 bug 真机拦截修正（nativeObjects 误传依赖集、append 污染 -c 步、ForeignNames 锚位），计数链 641→673→674 一致；无「勾了没跑」项。③ 测试先行证据：T1 32/32 先红（31 parser 边界 + 1 码形差）、T2 四包红（undefined 符号编译红 + 行为红 6/6）在案；黄金人类可读面合 §52 形（`file:line:col: error[E]: msg`，探针逐字复核）；M12 零 --json 新面，§62 协议由既有 JSON 黄金承载（diag 包零改动）。④ 诊断协议稳定：internal/diag 零触碰、--json 字段零删改；八码系 c5efd3a（22 章规范落地）预注册——M12 零码认领、helps 逐字引注册表（「分配或认领」义务无涉，validate strict registry clean 直证）。⑤ 单一权威：零规范提升立场一致（纯实现变更，spec 先在）；ch19:39 示例拼写笔误 → roadmap follow-up #15 已登记（双语，docs_sync 31 对齐含）；机制载体 = 变更记录 + roadmap（M11 先例）；新增代码注释全英文。⑥ 红线复核：refr/ 零触碰（git status 直证）；尚无提交（trailer 面待 T11 草案呈报时复核）；diff 范围——fmt.go/fmt_test.go = 披露 5（M11 fmt 缺陷 D13 修复）、parser_test.go = T2 测试、archive tasks.md = M11 提交对账遗留句（已知随行）、roadmap = M12 行 + follow-up #15，无未披露越界改动。⑦ 最小可信验证：T8 阶梯九步（build/vet/test 全包/runtime C harness/conformance 674/validate strict/docs_sync/gofmt/whitespace）全绿，与 welang-pre-push-checks 的 compiler 层选取一致。发现与处置：披露 13（run 面相对路径双重解析，M9b 潜伏缺陷）已修复 + 回归钉 `run-ffi-scalar-subdir` 先红后绿——本变更期唯一新增缺陷面，D13 纪律闭环。通过。
