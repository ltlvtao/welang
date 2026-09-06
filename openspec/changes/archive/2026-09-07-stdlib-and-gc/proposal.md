# 提案 — stdlib-and-gc（M8）

## Why

roadmap M8 承诺两件事：`std.io` core 与运行时精确 GC 设计落地。代码里三个标记等在 M8 归位，全部可黑盒验证：

1. **`bndStdModules` 边界**（`internal/typecheck` 三处）：`import std.*` 与内建类型上未锚定的 stdlib 成员自 M6b 起停在诚实边界（今日 `we check` 对 `import std.io` 程序 exit 70）——ch15 R1 的 `std.` 保留段（「compiler-provided，永不文件系统解析」）至今没有提供的模块。
2. **组合子空体标记**（M6a design）：Iterator 的 11 个组合子默认体是 `body: &ast.Block{}` 空标记——签名与语义 ch17 已批，真实体注释明写「真实体归 M8 stdlib」。
3. **alloc.c 的 malloc 直通**（M4）：注释明写「precise-GC 设计替换 body，不换 ABI（ADR-0002 的运行时范围）」。GC 自 M4 起没有用户——codegen 接收集（main 单 Ok/Err）不产生任何堆分配，设计无处验证。

codegen 自 M4 起经 M5/M6a/M6b/M7 四个里程碑三次裁决「维持接收集」——扩面条件（GC 设计决定分配 ABI 与根协议）恰是本变更的交付物。

## 裁决记录（candidate 阶段四项表面裁决，2026-09-07，均采纳推荐）

- **Q1 std.io core 的规范定位 = 纯实现**：ch15 R2 已把预导入之外的 std 内容定位为「ordinary module under `std.`」——库内容是模块的代码，不是语言表面；机制（`std.` 保留段、import 到达、pub 面、E1303/E1304、ch16 效果检查）全部已批。println 契约由 conformance 黄金钉（检查面报文逐字 + 运行面 stdout 逐字）。被拒面（小规范增量钉 std.io 最小面）：开「库表面入规范」先例，库演进每次都要规范变更，违 P2 的糖不开章同族立场；v0.8 §51 自己也说「具体绑定代码清单……不在本规范中逐一列出」。
- **Q2 stdlib 的提供机制 = 编译器内建模块**：Go 登记符号表（fn 型 + 效果集 + pub 位）进 M6b 装载图，与 M6a 内建类型登记（collectionMembers/stringMembers）同族。被拒面（embed 真 .we 源文件走同一 parser/checker）：更贴「标准库不享有特权语法通道」的长期方向，但 std.io 的底层体无法用已批表面书写（We 无 syscall 面；foreign 是 M12）——M8 落它必然造出特权通道。自举源码路由推迟到 M12 后（ch19:207 已留钩子），design D9 记路由。
- **Q3 codegen 扩面深度 = main 体垂直扩**：main 体 = 语句序列（`let` 绑定 / `io.println`·`io.print` 表达式语句 / 单 return Ok 或 Err），表达式子集 = String 字面量 / gc 记录构造 / 字段读链 / io 调用；gc 记录经 `__we_alloc` 堆分配 + shadow-stack 根登记——GC 第一次有真用户，hello world 端到端。M0「薄垂直优先」裁决的第三次应用。被拒面：维持 M4 接收集（GC 设计空转，M9 并发设计悬空）；全函数生成（单变更不可验证，ch21 未承诺中间深度）。
- **Q4 GC 初版范围 = shadow-stack + STW 标记清除**：ADR-0003 记战略（根协议选型/算法/ABI 冻结/演进路由），runtime/c 落实现。被拒面：仅设计文档（「设计落地」无实证）；分代/并发/写屏障（单线程直线 main 接收集下无观测面，过早优化违 P4；并发 GC 是 M9 的门）。

## 现状与差距

- 装载：`import std.*` 于 loadGraph 先停 bndStdModules；`io.println` 在纯 fn 里今日不可能出现（装载即停）——E1401 的首个真实库函数用户缺席，ch16 内建标签 io 至今无携带它的库函数（M7 D13 第 5 条归位）。
- 检查：String 成员的 spec-anchored 家族已实现，其余 stdlib 成员停 bndStdModules；组合子签名/语义/继承全实现，体是空标记（走查零内容）。
- 运行：codegen 接收集 = main 单 `return Ok(())` / `Err(V("literal"))`（295 行 Emit）；runtime/c 共 35 行（startup + malloc 直通）；`we run` 的唯一可观测行为是 Err 报文行。
- GC：无堆、无分配、无回收——ADR-0002 的 product-scope runtime 交付尚未开始。

## 目标与非目标

**目标**：

1. **std 模块装载机制**：`import std.io` 真装载——std.io 作为编译器内建模块进 M6b 装载图（不触文件系统，ch15 R1 兑现）；未知 `std.*` 路径报 E1302 的 std 形报文。装载后 ch15 R3/R4 全走既有机器（pub 面、合格名到达、E1303/E1304）。
2. **std.io core 检查面**：`println(s: String) effect io` 与 `print(s: String) effect io`（返回 `()`）——纯函数调用 `io.println` 报 E1401、`effect io` 函数内绿。
3. **std.io core 运行面**：`io.println` / `io.print` 落到 runtime 的 stdout 写出——`we build` / `we run` 端到端可观测输出。
4. **组合子真实体**：11 枚空标记换真实 We 体（Go 构造 AST，语言只用品已批面）；M6a 起存在的默认体走查机器自此走真代码——stdlib 的体与用户体同机检查。
5. **精确 GC 设计落地**：ADR-0003 记录战略；runtime/c 落实现——shadow-stack 根协议（`__we_root_push`/`__we_root_pop`）+ stop-the-world 精确标记清除；`__we_alloc`/`__we_free` ABI 不变（M4 承诺兑现：换 body 不换 ABI）。
6. **codegen 扩面（垂直最小）**：main 体从单 return 扩到语句序列；gc 记录堆分配 + 根登记；其余边界行（bndOtherFns/bndTopLets/bndMainBody 超集）不动。
7. **端到端闭环**：`we new` 项目改写为 hello 程序（gc 记录 + println）`build` + `run`，stdout 输出真实文本——参考工具链第一个非 Err 路径的可观测行为。

**非目标**：

- std.io 文件读写（readFile/writeFile）——ch19:207 的 foreign 场景，M12 后 stdlib 才能经 foreign 自举底层。
- 其余 std 模块装载（std.concurrent→M9、std.test→M10；List/Map/Set 已是编译器内建类型不经 import 到达）。
- 并发/分代/增量 GC——M9 并发之前的单线程 stop-the-world；演进路由记 ADR-0003。
- String 运行时构造面（charAt/byteSlice 等维持停界；插值字面量入 build 停界）。
- 全函数 codegen（bndOtherFns/bndTopLets 维持；算术/控制流生成不扩）。
- we test / vet / fmt（M10/M11）。

## What Changes

无规范增量（Q1 裁决）：`import std.io` 与 `io.println` 走的全是已批机制（ch15 到达、ch16 效果检查、ch8 弃置豁免），效果与诊断码零新增零改义；E1302 对未知 `std.*` 的 std 形报文是既有码在 ch15 R1 保留段语义下的报文形（无文件系统映射的路径），注册表 remediation 不动。变更交付：装载分支（typecheck/cli）、std.io 内建符号、组合子真体、GC 运行时（gc.c/io.c/startup/alloc 归并）、codegen Emit 扩面、ADR-0003（双语）、roadmap M8 → done（双语）。

## 影响层

- `compiler`（typecheck/cli/codegen）——主层。
- `stdlib`（std.io 内建面——本变更的主体交付之一）。
- `docs`（ADR-0003 与 roadmap 行）。
- 不含 `spec`：纯实现，无规范增量，不改变语言行为（豁免声明见 What Changes）。

## 影响范围

| 层 | 文件 | 动作 |
| --- | --- | --- |
| compiler | `internal/typecheck/typecheck.go` | bndStdModules 边界删/收窄；std 内建模块符号；组合子真实体 |
| compiler | `internal/cli/check.go` | loadGraph 的 std 分支（内建模块进图，后序 ingest） |
| compiler | `internal/codegen/codegen.go` | Emit 扩面：语句/表达式子集、struct 类型行、分配与根登记、println 调用 |
| runtime | `runtime/c/gc.c`（新）、`runtime/c/io.c`（新）、`runtime/c/alloc.c`、`runtime/c/startup.c` | GC 实现、stdout 写出、boot 挂接、alloc 归并 |
| runtime | `runtime/runtime.go` | embed 清单扩 |
| runtime | `runtime/`（Go 测试） | C 测试 harness（clang 编 gc.c + 断言 main） |
| docs | `docs/decisions/ADR-0003-gc-strategy.md`（新，双语） | GC 战略决策记录 |
| docs | `docs/roadmap/0000-reference-implementation.md`（双语） | M8 → done |
| tests | conformance 黄金 ~30 枚、parser/typecheck/codegen 单测、runtime harness | 测试先行 |

## 涉触码盘点

- `bndStdModules`（typecheck.go 三处 + 两处测试引用）：std.io 分支删除/收窄，其余 std 成员停界维持（String 未锚定成员等——design D10-2）。
- `internal/codegen` 四行边界：bndMainBody What 词表扩；bndErrPayload/bndOtherFns/bndTopLets 词表不动。
- `runtime/c/alloc.c`：`__we_alloc`/`__we_free` 的 malloc 直通 body 替换为 GC 实现（ABI 签名不变——M4 注释承诺兑现）。
- M6a 组合子 11 枚 `body: &ast.Block{}` → 真体（typecheck.go 内建接口构造处）。
- 既有 conformance 404 枚中 **399 枚零触碰；5 枚随已批行为面更新**（T2 落盘即红，2026-09-07 记录）：4 枚 build 边界黄金（build-bnd-body / build-bnd-m6b-main-body / build-ch10-method-call-boundary / build-bnd-new-forms）的 stderr 随 D4 新 What 词表更新——源程序全部仍在接受集外（Int64 字面量绑定 / `?` 传播 / 方法调用 / if 控制流），exit 70 不变；check-bnd-std-import 随 D1 装载翻绿（exit 70 → 0，源仅 import 无 io 调用）。另有 2 枚（check-parse-clean / check-parse-clean-json）将在 T3 装载落地时从 exit 70 翻为检查诊断集——期望报文于首绿时从实跑输出逐行审计重导（每行须与既有已批诊断面一致），随完成记录披露（「边界表面变化随变更记录」先例：lexical / parser-core）。`we new` 骨架程序 build/run 行为零改写。

## 已知风险与开放问题

- **组合子真体走查揭出检查器缺陷的风险**：真实体进入默认体走查（M7 checkIfaceBodies 机器），若体触发既有机器的边界（如闭包推断深度）将以 stdlib 位置报错——处置：修复检查器并披露（M5「测试先行抓真缺陷」先例），不得静默改体绕过（design D13）。
- **根登记与直线控制流**：M8 main 体无控制流，push/pop 严格嵌套成立；控制流生成（后续里程碑）引入分叉/合并时根协议需扩展（记 ADR-0003 演进段，非本变更面）。
- **效果段零行为差**：println 调用生成零效果面（M7 D10 延伸）——A/B 真机电池证之。
- 开放问题：无（审查 F1 已把 GC 描述符传递形收敛为终形——design D6）。

## 审计记录（2026-09-07，welang-spec-impact-audit 7 条，通过）

1. **问题真实性 ✓**：三个可黑盒验证的工程缺口（`import std.io` 今日 exit 70；组合子空体；GC 无用户），引用 ch15 R1/R2、ch16 内建标签、ch17 组合子语义、ch19:207、M4/M6a/M6b 代码内标记与 roadmap M8 行；refr 基线固定（spec-0.8.md §51、§65-13）。
2. **影响层声明 ✓**：change.yaml `layers: [compiler, stdlib, docs]` 与本文件「## 影响层」一致；行为层（compiler/stdlib）不带 spec 的豁免走「无规范增量 + 不改变语言行为」标记（proposal 正文含两标记，validate.py strict 豁免路径）。
3. **规范增量范围 ✓**：零新增零修改零删除 Requirement 与诊断码；E1302 std 形报文是既有码的报文形（触发语义 ch15 R1 已批——保留段命名的模块不存在即 module not found），注册表不动。
4. **原则一致性 ✓**：无原则突破。P1（效果声明携带、检查局部）、P2（库表面不入规范）、P4（GC STW 初版，无过早优化）、P7（Emit 纯函数维持）各得其所；无新隐式转换。
5. **参考基线固定 ✓**：refr/spec-0.8.md §51（stdlib 表）、§65-13（GC 留白）以文件+节号固定；v0.8 立场（「标准库不享有特权语法通道」「清单不入规范」）作方向参考引用，不当既成规范。
6. **验收边界 ✓**：目标 1–7 逐条可机械判定（import 绿/E1401 报文/hello stdout/collect 探针存活）；非目标六条显式排除（文件读写/其余 std 模块/并发 GC/String 构造面/全函数 codegen/test-vet-fmt）。
7. **粒度 ✓**：三面（装载+GC+codegen 扩）互为依赖构成一个垂直验证单元（hello 端到端缺一不可——分配 ABI、根协议、装载、符号映射各自独立则互相悬空）；M0「薄垂直」先例第三次应用。

**结论**：通过，status → ready。

## 审查记录（ready → active 关卡，welang-change-review 10 条，2026-09-07）

结构validate --strict 先行通过。逐条：

1. **proposal 职责 ✓**：黑盒问题（三标记 + 今日 exit 70 可验证）与目标/非目标为主；「涉触码盘点」为影响面陈述（M7 先例同形），实现决策归 design。
2. **spec 增量 ✓**：无（纯实现豁免路径），无越界可查。
3. **design 唯一路径 ✓（修复后）**：F1——D6 原把描述符传递形留两形「实现期定」，违唯一路径与第 10 条（ABI 契约开放问题不得悬置）；已收敛终形：`__we_alloc(i64) ptr` 签名不变（M4 ABI 承诺字面），分配器写 size 位、编译器 store map 位、描述符为 IR 侧位图常量。被拒替代方案在四裁决与各 D 记录（提供机制/扩面深度/GC 范围各有被拒面）。
4. **tasks 职责 ✓**：T1–T11 均有来源（proposal 目标/design D）与验证（命令级）；无 deferred/non-goal 项。
5. **场景覆盖 ✓**：D10 不可达清单七条 + D11 翻绿面 + D12 黄金分布（绿/负例/边界三类，failure 路径含 E1401 纯函数调用、E1302 未知 std、遮蔽）。
6. **无空章节 ✓**：D1–D13 均载实质；无 N/A 表格。
7. **测试先行 ✓**：T1 黄金先红（对当前构建红、404 既有零回归为证）+ T2 单测/harness 先红（引用未定义符号/快照不匹配）。
8. **负向断言 ✓**：黄金表负例族真实构造违规源并断言报文（E1401×4、E1302×2、E1403 遮蔽、边界 What 新词表）；T1 验证要求红名单清单。
9. **完成度闭环 ✓（原则 10 三要素）**：检查（D2/D3/D4 检查面）、代码生成（D4/D5 Emit 扩面）、运行时（D6/D7 GC + io）三要素齐备且互相咬合（分配 ABI ↔ 根登记 ↔ 描述符）。
10. **未决问题 ✓（修复后）**：F1 收敛后无悬置开放问题；「已知风险」三项均为风险披露非未决方案（处置路径已写死）。

**结论**：F1 修复后通过，status → active。

**T2 修正记录（2026-09-07，激活后、实现前的契约对账）**：审查通过文本中「既有 conformance 404 枚零改写」经 T2 全量对账不可达——D4 的 What 词表扩与 D1 的装载翻绿各有既有黄金逐字钉住旧行为。处置见「涉触码盘点」更新行（399 零触碰 + 5 随批更新 + 2 重导义务）：更新随已批裁决（D4/D1）先行落盘并即时转红，实现追赶契约，非实现漂移拟合。

**实现审查记录（active → complete 关卡，welang-code-review 7 条，2026-09-07）**：

1. **规范符合性 ✓**：本变更为纯实现（裁决记录 Q1，无 spec 增量），行为锚点全部在既有已批规范——组合子内建标记的依据 ch21 规则 9（D3 勘误段引注）、io 调用效果检查 ch15/ch16 既有 E1401/E1402 机器、GC 对象协议 ch16（ADR-0003 承载）。无规范外接受/拒绝判定：check 面新增绿全部为规范已允许形（装载使既有检查可达），build 面扩集全部停显式 What 边界。
2. **验证诚实性 ✓**：T1–T9 逐条核对，勾选与记录的验证命令真实可复跑——本审查窗口强制新鲜复证：conformance `-count=1` 全绿（431 枚）；真机电池五案重跑（hello `We`/exit 0；rec `core`/`wrap`/exit 0；err stdout `hi`、stderr `error: Failed: boom`、exit 1；new 骨架静默 exit 0；效果段 A/B build IR diff 空且双方运行一致）。T9 记录的 GC 计数缺陷披露与探针修正描述和 gc.c 实码一致（size 位 0 自由标记、清扫只计 `!blk_free` 新死块）。
3. **测试先行证据 ✓**：T1（27 黄金先红，红 20 = 全部新枚，comm 对账既有 404 零回归）与 T2（typecheck/codegen/runtime 三面引用未实现符号先红）的红证据在完成记录；黄金期望格式为 §52/§62 既有管线产物（重导枚从实跑捕获）。
4. **诊断协议稳定 ✓**：`--json` 字段零改动（diff 无 diag 序列化面）；本变更零新增诊断码——全部停既有码（E1302 std 形为既有码的新报文形，注册表未触碰，`git status` 证 diagnostics.toml 与 openspec/specs/ 干净）。
5. **单一权威 ✓**：长期事实提升路径明确——GC 设计取舍 → ADR-0003（本次归档落盘）、里程碑状态 → roadmap；代码注释/docs 英文复核（本变更新增 .go/.c 无 CJK；仓内三处既有 CJK 为合法测试数据与术语对照表，非本变更引入）；变更工件中文、规范差异无（纯实现）。
6. **红线复核 ✓**：refr/ 未入提交面（`git ls-files --others` 过滤后零 refr 路径）；提交在 T11（英文、无署名 trailer，pre-commit/commit-msg 双钩把守）；diff 面（17 修改 + 19 新增 + 1 删除）逐一核对落在影响范围表所列区域内——其中两个载体文件未在表中逐名：`cli/build.go`（三运行时源物化 + 三对象编译 + 链接，GC 运行时行的管线载体）与 `cli/modules_test.go`（loadGraph std 边界测试改钉装载绿面，check.go 行的测试载体），两者均已随 T3/T6 完成记录披露；无越界改动。
7. **最小可信验证已跑 ✓**（本窗口新鲜）：gofmt -l 空、go build ./... 过、go vet ./... 过、validate --all --strict 过（1 change valid; registry clean）、git diff --check 干净、docs_sync 30 对齐（ADR-0003 落盘后复核 31）。

**结论**：通过，status → complete，进入归档流程（welang-archive-sync）。
