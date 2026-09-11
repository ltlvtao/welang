# proposal — codegen-mono（B1a）

## Why

roadmap B1 权威行：Full code generation: every ratified expression and declaration form emits — the codegen not-implemented set reaches zero（0000-reference-implementation.md:36）。执行状态规则同文件钉死纪律：「the set shrinks to zero as milestones land」——诚实边界（exit 70）是过渡实现态，不是规范面；B 轨 B1 是自举关键路径（用户裁决 2026-09-08 登记），且用户已指名开始。M15 在 docs/benchmarks.md:50 预披露「the set itself shrinks as the codegen-full line lands」——本变更就是那条线的第一塔。

今日的 codegen 停点表（internal/codegen/codegen.go:42-54）共 8 词：`bndMainBody`/`bndTaskBody`/`bndFnBody`（M9b 语句集三锚）、`bndGenericFns`、`bndTopLets`、`bndErrPayload`、`bndCallbackBody`、`bndAssertEqDomain`。检查器自 M5–M7 起就接受全部已批语言面（22 章规范先行、检查塔全绿）；codegen 是唯一的落后者——每个停点都是「check 绿、build 停 70」的诚实边界。roadmap follow-up #16（同文件 :65）登记的支配性缺陷（match 臂赋值先于 scope 块 → `Instruction does not dominate all uses`）同属 codegen 线，本变更承载其根治。

**四项用户裁决（2026-09-09/10，均采推荐）**：

- **Q1 两塔切分**：B1 = B1a 单态值塔（本变更）+ B1b 多态派发塔（`codegen-poly` 后行）。B1a 把检查器已接受的单态值面全部发射（语句集、表达式值形、字符串、记录/元组/newtype、闭包捕获、List 载体、顶层 let、sum 物化、assertEqual Eq 域）；B1b 承载泛型单态化 + 检查器注记面 + Dyn vtable + 接口派发 + 5 枚惰性组合子。
- **Q2 检查器知识出口**：B1b 拓宽 Check() 出口为注记面（今日检查器知识全部瞬时——checker struct 返回即弃、泛型实例化逐调用点无登记、AST 零注记、codegen 全部自行重推）。本裁决在本变更内为**预裁记录**（B1b 生效），B1a 不动检查器出口。
- **Q3 集合运行时载体**：最小真实 C 载体现在落（B1a）——`List` 的可增长载体 + 快照迭代；Map/Set 无字面量、构造方法是 stdlib 自有面未批（ch17:5「its own surface, not ratified here」），检查器今日无生产者路径——Map/Set 真数据结构归 B2 `stdlib-real`。
- **Q4 assertEqual Eq 域**：B1a 落 derive 结构比较（`derives Eq` 的记录/和式/元组按结构逐字段比较发射），`bndAssertEqDomain` 退役。

本变更不改变语言行为——纯实现变更，无规范增量（22 章早已权威；本变更只让 codegen 跟上检查器，internals-only 豁免路径，M0 起先例、M5「规范先行的首个全载里程碑」同形）。

## 目标与非目标

目标（八件，一条线：让单态值面的 check 绿 = build 绿 = run 正确）：

1. **语句集完备**（D1）：loop/break/continue、for-in、scope resource、元组模式绑定、裸块、任意深度 return、`self.field =` 赋值、赋值面放宽（let 槽同 var 槽可存）——三锚（main/task/fn 体）词表同步退役。**env 作用域化**：发射环境按块作用域化（match 臂绑定/嵌套块 let 的 alloca 与 SSA 名出块即弃），根治 follow-up #16 支配缺陷。
2. **表达式值形完备（单态）**（D2）：if/match/块值表达式（结果 alloca 模式）、一元 `!`/`-`/`~`、`/` 与 `%`（零除 panic）、`&&`/`||` 短路分支形、rune 走 i64 域。
3. **字符串全链**（D3）：插值（基类型族值转串 + 具体复合结构渲染——规范对洞值无类型锚、检查器零限制，渲染形为实现定义并披露）、拼接 `+`、相等 `==`/`!=`、`byteLength`/`byteSlice`/`runeCount`/`charAt` 方法族——运行时 `__we_str_*` 符号族。
4. **记录/元组/newtype/unit 全值面**（D4）：标量字段读、`with &` 拷贝构造、mut self 方法与 inherent 方法经 (型, 名) 方法表静态派发、value record 同 ptr 表示整值拷贝、newtype 零擦除恒等。
5. **闭包全捕获与 fn 值**（D5）：gc 载体 `{fnptr, env}`、value 域字拷贝入 env、gc/String/record 以可 trace 指针入 env、fn 值调用面、prim 回调 ABI 兼容（(fn, env) 对形）。
6. **List 字面量 + 最小 C 载体 + 组合子 + for-in**（D6）：`__we_list_new` 可增长载体（新分配 + 拷贝——mark-sweep 无紧缩下安全）、快照迭代（拷贝）、Range/String for-in 物化数组（B2 调优）、6 枚急性组合子内建面（循环直发；We 体仍是检查面）。
7. **顶层 let + 模块 init**（D7）：`@<key>.init` 装载后序执行、gc 顶层绑定的全局根登记（运行时新面）。
8. **sum/Err 物化泛化 + 槽放宽**（D8）：任意载荷 sum 的 {tag, payload} 物化（payload i64 域，gc 载荷 ptrtoint 先例）、用户函数返回 sum/记录/同步类型、同型参数槽——`bndErrPayload` 与 fn 尾返回停点退役。

（D9 assertEqual Eq 域 = Q4；D10 既有缺陷修复组；D11–D12 基准套件重锚与对账；D13 验证阶梯——见 design。）

非目标：

- **泛型单态化与 Dyn 派发**（B1b `codegen-poly`）：`bndGenericFns` 保留为 B1a 后唯一停点词；接口值动态派发（vtable）、`Dyn<Iterator>` 惰性组合子（map/filter/take/skip/collect）不在本变更。
- **检查器注记面**（Q2，B1b）：Check() 出口、实例化登记表、AST 注记零触碰（B1a 的 typecheck 触面仅 assertEqual 域门放宽一处）。
- **Map/Set 真数据结构**（B2 `stdlib-real`）：ch17 未批构造方法面；检查器无生产者路径，维持检查面即诚实。
- **CLI 项目面 What**：`whatSingleFileBuild`（follow-up #6 规范缺口）与 `whatLibraryArtifacts`（ch21）不是 codegen 停点，不在零集目标内。
- **优化**：一切发射以正确性为先（物化数组、拷贝快照）；性能归 B2 调优与后续。
- **spec 触碰**：零 Requirement 增删、零诊断码、零 ADR（E0502 运行时陷阱语义已在 ch7 落定，窄整型逐宽检查是实现面兑现）。

## What Changes

- `internal/codegen`：D1–D10 全量拓宽（本仓库最大单变更 codegen 面）——env 作用域化、语句/表达式完备、字符串/记录/闭包/List/顶层 let/sum 物化、方法表静态派发、组合子内建面、模块 init、assertEqual 结构比较、四缺陷修复。
- `runtime/c`：新符号族——`__we_str_*`（拼接/相等/码点计数/切片/插值值转串）、`__we_list_*`（可增长载体/快照迭代）、全局根登记面。
- `internal/typecheck`：assertEqual 域门放宽一处（D9；8 整型+Bool+String → +derive Eq 的记录/和式/元组）。
- conformance：9 枚停点锚定黄金翻绿/重锚（5 枚 build-bnd-* 翻绿、test-fn-body-bnd、test-asserteq-domain-bnd、check-assertequal-domain-boundary 重锚）+ 拓宽面的新增黄金。
- benchmark（M15 重锚，D11）：traps_test.go 4 枚校准例重锚（2 枚陷阱消亡删除）、errpath-02/ctxpass-02 traps.md 支配缺陷括注更新、docs/benchmarks.md 双语运行面锚定段重写。
- docs：roadmap B1 行拆 B1a（本变更）+ B1b（双语，归档期）+ follow-up #16 划账。

## 影响层

- spec：零（纯实现；「不改变语言行为」——22 章权威面只被兑现不被修改）。
- compiler：internal/codegen 主拓宽 + runtime/c 新符号族 + internal/typecheck assertEqual 域门一处。既有 E 码行为零改写（E0502 窄整型为兑现 ch7 逐宽检查语义，非新行为）。
- benchmark：M15 套件重锚（校准例/traps.md/运行面锚定段）——「the set itself shrinks」承诺的兑现面。
- docs：docs/benchmarks.md + .zh.md 重锚段、roadmap 双语 B1 拆行与 follow-up #16 划账（归档期）。
- process：roadmap 双语（归档期）。

## 影响范围

- 涉触码：`internal/codegen/codegen.go`（全量）、`internal/typecheck/typecheck.go`（assertEqual 域门）、`runtime/c/*.c/.h`（新符号族）、`internal/conformance/testdata/`（9 枚重锚 + 新增）、`internal/benchmarks/traps_test.go` + `tasks/*/traps.md`（重锚）、`docs/benchmarks.md` + `.zh.md`、`docs/roadmap/0000-reference-implementation.md`(+zh)。
- 既有黄金零静默改写：全部重锚逐枚披露（D12 对账表）；非停点黄金零触碰。

## 审计记录

2026-09-10，welang-spec-impact-audit 7 点，**通过**：

1. 问题真实性 ✅——可验证黑盒缺口：检查器接受、codegen 停 70 的诚实边界集今日 8 词（internal/codegen/codegen.go:42-54 逐一在案：bndMainBody/bndTaskBody/bndFnBody/bndGenericFns/bndTopLets/bndErrPayload/bndCallbackBody/bndAssertEqDomain）；引用锚点全数核实——roadmap B1 权威行（0000-reference-implementation.md:36）、执行状态规则「the set shrinks to zero as milestones land」（:44）、follow-up #16 支配缺陷（:65）、M15 预披露「the set itself shrinks as the codegen-full line lands」（docs/benchmarks.md:50）。黑盒可证：任一停点形 `we check .` exit 0、`we build .` exit 70。
2. 影响层声明 ✅——change.yaml [compiler, benchmark, docs] 与 proposal 影响层一致（spec 零行 + process 归档期行同 M15 先例形）；「不改变语言行为——纯实现变更，无规范增量」边界明示（internals-only 豁免，M0 起先例；validate strict 通过即证豁免路径生效）。
3. 规范增量范围 ✅——零 Requirement 增删、零诊断码、零 ADR。D10-4 窄整型逐宽检查是 ch7 E0502 已批逐宽 checked 语义的实现面兑现（今日陷阱只查 i64 域 = 兑现缺口，非新行为）；D2 短路是 ch2 已批逻辑运算符语义的正确发射。
4. 原则一致性 ✅——无原则突破。本变更是诚实边界纪律（ch21 R2 exit 70 过渡态）的收敛兑现；D1 作用域化拒绝 alloca 全提升押后端优化（决定论发射 = P7 可重复性的实现面）；D6 组合子内建直发语义权威在章文（一类事实一个权威位置——We 体是检查面，直发是发射面，两者同锚 ch11/ch17）。
5. 参考基线固定 ✅——零处引用 refr/；引用面全为仓库本体且逐处带行号（roadmap:36/:44/:65、codegen.go:42-54/:846/:894、ch17:5、ch17:63、docs/benchmarks.md:50——本审计逐条复核在案）。
6. 验收边界 ✅——目标机械可判：停点集合恰为 {bndGenericFns}（grep 词表）、conformance 9 枚锚定黄金重锚绿、M15 校准套件重锚绿、四缺陷红测试转绿、支配形真机编译零拒绝；非目标六项排除防蔓延（B1b 全部、Map/Set、CLI What 两项、优化、spec）。
7. 粒度 ✅——一条垂直线（单态值面 check 绿 = build 绿 = run 正确）；两塔切分（Q1 用户裁决）正是粒度机制本身；M5 五章一变更先例承载过同等量级。

## 审查记录

2026-09-10，welang-change-review 10 点，**通过**（发现 2 处已处置）：

职责边界四条 ✅——proposal 黑盒问题+目标（八件带 D 编号同 M15 先例形；What Changes 载结构属 M13/M14 先例形）；spec 增量零（internals-only 豁免——四裁决记录 + 纯实现边界明示；技术面由 design 承载）；design 唯一路径 + D1/D2/D4/D6/D8 各记被拒替代与理由、引用精确到 roadmap:36/:44/:65、codegen.go:42-54/:846/:894、ch17:5/:63、docs/benchmarks.md:50、ch1:130（词法锚）；tasks 全项来源/验证、无 deferred/非目标/未决。

内容质量六条 ✅——场景覆盖三路（normal 各拓宽面正程序 / boundary 域外形——泛型 fn 体、非 Eq derive 泛型断言 / degradation——溢出陷阱、零除、OOB、短路 RHS 不求值、窄整型逐宽检查）；无空章节；测试先行（T2–T11 首 checkbox 对应形今日全部 bnd 停点、首测自然红——顶部注句统一钉死；T1/T3/T4/T6/T11 缺陷钉单列红行）；负向断言真实（每拓宽面至少一正一负 T13、域外停点形 + panic 面真机负例）；完成度闭环第 9 条适用性判断——本变更是实现追赶而非新语言特性（零规范增量），检查器三要素面早已闭合（M5–M7 检查塔全绿、规范 22 章先行），codegen/runtime 是唯一落后层，design 唯一检查器触面（assertEqual 域门）明示在 D9；无未决问题（四裁决已落、D3 实证定谳）。

F1（已处置）：D3 原稿把「记录/和式插值走 bndGenericFns 同停点」写成设计定稿——词意错位（bndGenericFns 词文是 generic functions / monomorphization，装具体复合值插值是冒充）；且实证（2026-09-10 探针）显示检查器接受 record 插值（check exit 0）、规范对洞值无类型锚（ch1:130 只定词法）。处置 = 定谳 B1a 落**具体复合结构渲染**（record/元组/sum；与 derive Show 解耦——渲染形是规范沉默位的实现定义并披露；递归、无环不可变构造下终止；泛型洞值随函数体整体停 bndGenericFns，词意吻合）。design D3/proposal 目标 3 已同步修正。

F2（已处置）：tasks.md T5/T7/T8/T9/T10 首 checkbox 缺显式「红」标注（T2 有单列红行）——测试先行纪律的标注不齐。处置 = 顶部注句统一声明（T2–T11 首 checkbox 形今日全停 bnd、首测自然红；缺陷钉单列），不改逐枚行。

## 实现记录

### T1 env 作用域化 + follow-up #16 根治（2026-09-10）

**红证据**（修复前真机——同一 we 源码三形，`/tmp/we-bin-old` 为修复前工作树编译）：

1. **支配形（errpath-02 触发序）**——match 臂绑定先于 `scope timeout` 块、scope 内 task 捕获外层同名绑定：`we check .` exit 0 全绿，`we run .` exit 1：
   `we: clang failed: error: invalid LLVM IR input: Instruction does not dominate all uses!`
2. **支配形（最小）**——同遮蔽 + join 后纯读，无 scope/task：同诊断 exit 1。
3. **静默错值形**——臂内 `let n: Int64 = 9`（常数）遮蔽外层 `n: Int64 = 5`：常数无支配问题，clang 接受、exit 0，但 join 后 `if n == 5` 输出 **`notfive`**（泄漏的常数 9 遮蔽外层 5）。

IR 机制铁证（修复前 demo.ll，支配形 2）：

```
marm0_0:
  %v9 = load i64, ptr %v5        ← 臂内载荷绑定 n 的 load（SSA 寄存器定义于臂块）
...
mjoin0:
  %v11 = icmp eq i64 %v9, 5      ← join 块读取 %v9——marm0_0 不支配 mjoin0
```

根因：emitMatchArm 把载荷绑定名无条件写入平铺 e.scalars、永不回滚（臂体 let 同）；join 后任何读外层同名绑定的发射点拿到臂块 SSA 名 → 非支配 use；同族泄漏另见 select case yield 名注册与 scope 体绑定（后者同块发射，无 clang 拒绝面——静默错值）。

**绿证据**（修复后，同三源程序）：三形 `we run .` 全部 exit 0，输出依次 `ok`/`five`/`five`（值正确——join 读外层 5）。join 区 IR：

```
mjoin0:
  %v11 = icmp eq i64 5, 5        ← 外层常数操作数，臂内寄存器零泄漏
```

作用域化后同源程序编译链接运行零拒绝。

**实现**：emitter 增 `frames []envFrame`；pushEnv/popEnv 对五张 env map（scalars/strEnv/gcEnv/sums2/prims）clone-on-enter/restore-on-exit（maps.Clone）。帧点——`emitBlockStmts` 统一收口一切块体（if 两分支/while 体/match 臂体/defer 尾排），match 每臂一帧（emitMatchArm 覆盖载荷绑定注册区）、emitScope 体循环一帧、emitSelect 每 case 一帧（yield 名同族封口）；capture 解析走活动帧视图（collectCaptures/emitTask 于创建点读当前帧 map——词法可见性正确）。emitTask/emitFnDefine/mock/test 上下文 save/restore 闭包纳入 frames。臂体经 emitBlockStmts 时双重帧（arm 帧 + 体帧）——嵌套无害。

**测试**：`internal/codegen/envframe_test.go` 5 例（match 臂遮蔽读 / errpath-02 形 task 捕获 env store / if 分支遮蔽 / scope 体遮蔽 / while 体遮蔽——全部先红后绿）；conformance 新黄金 3 枚（run-frame-arm-payload-shadow / run-frame-arm-let-shadow / run-frame-scope-task-capture，701→704）逐枚先经真机红→绿定形；`go test ./...` 全仓 14 包全绿，既有黄金零回归。

### T2 语句集：循环跳转/任意深度 return（2026-09-10）

**红证据**（HEAD 树 `git archive HEAD` → `/tmp/we-head`，B1a 树对照；M9b 集内形 HEAD 已能发射，故逐形界定缺口）：

| 形 | HEAD check | HEAD build | B1a |
| --- | --- | --- | --- |
| main 体内 `loop { break }` | 0 | 70 `bndMainBody` | 0 |
| main 体内 `while … { i = i + 1; continue }` | 0 | 70 `bndMainBody` | 0 |
| main 体内 `while … { i = i + 1; break }` | 0 | 70 `bndMainBody` | 0 |
| main 体内 if 臂 `return Ok(())` | 0 | 70 `bndMainBody` | 0 |
| fn 体内 if 臂 return | 0 | 70 `bndFnBody` | 0 |
| task 体内 `break` | 0 | 70 `bndTaskBody` | 0 |
| （对照）main 体内纯 `while … { io.println }` | 0 | **0**（M9b 集内） | 0 |

三锚各有一枚真源在案——缺口精确落在 M9b 集外：`loop`、`break`/`continue`、任意深度 return。

**实现**：`emitBlockStmts` 统一收口块体（T1 的帧点）+ `emitWhile`/`emitLoop` 循环帧（break → 出口标签、continue → 头标签）+ `diverged` 守卫（终止符后不得再发指令——join 分支丢弃而标签照开）+ 退出协议 `exitSite`/`drainDefers`/`drainScopes`（return 求值 → 作用域自内向外 discharge（fail-fast cancel + join；collectAll 只 join）→ defer 逆排 → 根栈 pop → ret）。

**测试**：`loop_test.go` 5 例（break/continue 目标、loop 无限环、break 穿 scope、continue 穿 collectAll、scope 内 break 不外穿）+ `return_test.go` 9 例（if 臂 return、defer 双路排空、穿 scope、嵌套 scope 由内及外、根栈 pop 序、task thunk 不 pop、循环内 return、main 深 return 的 Result 面、test 深 return 的 void 面）；conformance：停点黄金 `build-bnd-new-forms`（深 return 形，70 边界）删除，按 D12 重锚为真机面 `run-main-early-return`（同形程序加 io 面，Ok 早返回 exit 0，走查后的语句不发）+ `run-main-early-return-err`（Err 早返回 exit 1，stderr `error: Failed: early`）——一正一反，逐枚对账在 T13。

### T2-a 裸块语句 + 赋值面放宽（2026-09-10）

**红证据**（HEAD 树 `/tmp/we-head` 与 B1a 树同源程序对照）：

| 形 | HEAD check | HEAD build | B1a build+run |
| --- | --- | --- | --- |
| 裸块（内层 `let s` 遮蔽 + 块内写外层名） | 0 | 70 `bndMainBody` | 0 —— 输出 `inner`·`outer`·`deep-ok` |
| let 重赋值（含别名拷贝形） | 0 | 70 `bndMainBody` | 0 —— 输出 `let-ok`·`alias-ok` |
| 参数重赋值（fn 体内循环改参数） | 0 | 70 `bndFnBody` | 0 —— 输出 `bump-ok`·`loop-ok` |
| 非标量赋值（`s = "b"`） | 0 | 70 | 70（留作负例黄金，T4 字符串面重锚） |

单测红证据：`internal/codegen/assign_test.go` 7 例初跑 5 例停在 `bndMainBody`/`bndFnBody`，报文含 design D1 点名的 M9b statement set 措辞。

**实现**：① **裸块语句**——`emitExprStmt` 增 `*ast.BlockExpr` 臂 → `emitBlockStmts`（一帧作用域）。块尾值弃置由检查器拥有（实证：main 体内 `{ 1 + 2 }` check exit 1 报 `error[E0605]: non-unit value dropped`），codegen 不到达该形。② **赋值面放宽**——`emitStmt` 的 `Assign` 分支删 `!slot.isVar → bnd`，改判 `slot.alloca == ""`；`scalarSlot.isVar` 退役（`alloca != ""` 即「可寻址」，字段收为 `{operand, alloca, isFloat}`）；每 body 一次 `collectAssigned` 预扫描（**发射之前**——绑定点必须知道该名会不会被写；跳过嵌套 TaskExpr/Closure，它们自带扫描且写不到外层名），被赋值名经 `bindScalarSlot` 取 alloca 槽、纯读名保持 SSA operand；7 个绑定点接入（emitLetBinding 的 Literal/Binary/Ident-alias 三分支、bindResult ckI64、match 臂载荷、select case 名、define 参数）。别名形要点：被赋值的别名必须**拷值**（读源槽/operand 后存入自己的槽），否则写别名会穿透到源名。D10-1 边界：`let x: Float64 = a + b` 的 SSA 面继续丢域，仅**被赋值名**的槽路径把 `isFloat` 传下去（槽带宽度是硬要求），注释在码中披露，缺陷本体留 T11。

**缺陷发现与修复（D13 披露——既有缺陷第五枚，实现期发现）**：slot 面首次落地即暴露 **alloca 逐迭代**缺陷——clang 以无 `-O` 旗标调用（`internal/cli/build.go` 的驱动序列），mem2reg/SROA 不运行，非 entry 块的 `alloca` 每经过该块重新分配，函数返回前不归还。**单源真机红→绿**（同一 `while i < 30000000 { var j: Int64 = 1; i = i + j }`，`/tmp/p5`）：

```
HEAD 树 /tmp/we-head：check 0 · build 0 · run → Segmentation fault（exit 139）
B1a 树 /tmp/we-now ：check 0 · build 0 · run → done（exit 0）
```

**该缺陷在 HEAD 上可达，非潜伏**——`var` 在 while 体内属 M9b 语句集，HEAD 真机即可编译（IR：`whbody0:` 内 `%v5 = alloca i64`），即 M15 参考实现里「循环体内声明 var」的 3000 万次迭代必崩。机制铁证（同 IR 仅该行位置之差，`/tmp/al`）：

```
build/al.ll         entry:  里 %v5 = alloca i64  → 30M 迭代 run exit 0，输出 done
build/al-broken.ll  whbody0: 里 %v5 = alloca i64 → Segmentation fault，exit 139
```

（复现：`we build` 出的 `.ll` 手工把该行移入循环体后，以同一 clang 序列重链。）性质界定：这是 design D10 四枚**之外**的第五枚既有缺陷（设计期的四枚清单不动），由 T2-a 的槽面重构连带根治——披露不绕过（D13）。

修法：`e.slot(typ)` 把 alloca 文本缓冲入 `e.allocas`（**值编号仍在取号点取**——编号即发射序，位置不动），`e.bodyText()` 在 body 关闭时把缓冲插到所有 define 格式串写入的 `entry:` 之后；13 处发射点改 `e.slot(...)`，6 处 body 的保存/恢复纳入 `allocas`/`assigned`。**零 golden 漂移**：6 处 byte-exact IR 快照本就不含 alloca，conformance 比运行输出不比 IR。回归钉 = `TestSlotHoistedToEntry`（两枚 alloca 在 `entry:`，`whbody0` 段内不含 `alloca`；判空前先验标签存在，防 vacuous pass）。

design D1 的「被拒替代：alloca 全提升 + mem2reg 依赖 clang 优化」经重读**不构成对本修复的否决**：(a) 拒绝把正确性押在后端优化 pass 上；(b) 全提升治不了 capture 按名巧合命中的**语义**错误（拿错值）。两条都由「逐 body 提升到 entry + T1 的自持 env-frame 作用域」满足——提升是自持的（不依赖任何 pass，clang -O0 下正确性由发射形本身保证），语义错误由作用域栈承担。

**测试**：`internal/codegen/assign_test.go` 7 例（裸块作用域遮蔽 / 裸块写外层名 / let 取槽 / 参数取槽 / 纯读 let 保持 SSA / 别名自有槽 / 槽提升钉）全绿；conformance 新黄金 4 枚——`run-bare-block-scope`（遮蔽 + 深度赋值读写正确）、`run-let-reassign`（重赋值 + 别名拷贝语义）、`run-param-reassign`（参数重赋值：直改与循环内改各一）、`build-bnd-string-assign`（负例：非标量赋值仍 70），709 = 705+4，逐枚真机先行定形；`go test ./...` 14 包全绿；gofmt/vet 干净。

**D11 陷阱消亡处置**：`control-04/mutate-let-param-decline` 校准例与 `tasks/control-04/traps.md` 同名条目删除——let 重赋值发射后该突变落在 bucket "clean"（check 0 + test 0），是行为正确的等价形而非陷阱；按陷阱纪律「无捕获面 = 任务设计缺陷」，以校准例删除 + 本披露处置，不放宽断言。

**发现（implement 期，处置已入 tasks.md）**：

1. **值位调用缺口**——`emitNumExpr` 无 `*ast.Call` 臂，用户函数调用落在算术/比较/逻辑算子的**操作数位**（`if bump(5) == 15`、`let x = bump(5) + 1`）时 check 0 / build 70 bnd；HEAD 与 B1a 树同报（`/tmp/we-head` 对照），**非回归**，属 design D2「表达式值形完备（单态）」面。当面调用（`if f()`）与实参位（`f(g())`）今日均已发射——缺口精确界定为 operand 位置。处置：T3 新增同名 checkbox 承载（D13：披露不绕过）。
2. **三锚词退役次序**——`e.bnd()` 按体上下文选词（ctxMain/ctxTask/ctxFn），表达式缺口与语句缺口同走三锚词；语句集完成即删锚，会让值位调用这类缺口被 `bndGenericFns` 冒名（词文是泛型函数体，装上具体调用是冒充）。design D0 表本就把退役绑在 **D1+D2** 双面，T2 第三枚（三锚词表退役）据此移入 T13 词表终态对账，T2 收窄为 for-in / scope resource / 元组模式。

### T2-b for-in：Range 源直发计数循环（2026-09-10）

**裁决（实现期，design D6 的物化措辞据此重划）**：Range 源发射为**计数循环**，不物化数组。两者对 Range **观测等价**——边界按源序各求值一次、单位步迭代至 end、无任何受审面能观测那次分配（iterator 值不外逃、无用户可见分配点）——而计数循环既不需要 T7 的 `__we_list_*` 载体，也不需要堆。B2「改惰性」对 Range 由此天然满足。**源归属重划**（原 D6 的「三源同 T7」拆开）：Range → 本任务；String 源（需 `__we_str_runecount`/`charat` 游走）→ T4；List 源（需载体快照）→ T7。tasks.md 三处（T4 新增 checkbox、T7 措辞、T13 重锚对账）已同步。

**红证据**（HEAD 树 `/tmp/we-head` 与 B1a 树 `/tmp/we-now`，同源程序逐枚真机对照）：

| 形（真机程序） | HEAD | B1a |
| --- | --- | --- |
| main 体内 `for i in 0..3 { acc = acc + i }` | 70 `bndMainBody` | 0 —— 输出 `count-ok` |
| `for … { if i == 2 { continue } … }` | 70 `bndMainBody` | 0 —— `cont-ok`（4 次体） |
| `for … { if i == 2 { break } … }` | 70 `bndMainBody` | 0 —— `break-ok`（2 次体） |
| 循环变量重赋值 `i = i + 5` 后累加 | 70 `bndMainBody` | 0 —— `fresh-ok`（5+6+7=18，序列不坏） |
| 体内改边界名 `hi = 1` | 70 `bndMainBody` | 0 —— `bound-ok`（仍 3 次，边界只读一次） |
| `3..3` 与 `5..2`（start ≥ end） | 70 `bndMainBody` | 0 —— `empty-ok`（零元素） |
| 嵌套 `for` / 通配头 `for _ in` | 70 `bndMainBody` | 0 —— `nest-ok` / `wild`×2 |
| fn 体内 `for … { if i > limit { return i } }` | 70 `bndFnBody` | 0 —— `fn-ret-ok`（深 return 自循环体穿出） |
| task 体内 `for i in 0..2 { io.println }` | 70 `bndTaskBody` | 0 —— `task-tick`×2 |
| String 源 `for c in "abc"`（负例） | 70 `bndMainBody` | 70 `bndMainBody`（T4 重锚，非回归） |

三锚各有一枚真源在案；缺口精确落在「for-in 语句」本身，与源域无关的其余语义（continue/break/重赋值/边界求值序）全部随该形一并成立。

**实现**：`emitStmt` 增 `*ast.ForStmt` 臂 → `emitFor`。源为 `*ast.Binary{Op:".."}` 时：两边界经 `emitNumExpr` 在 head 块**之前**按源序求值，落寄存器（体内改边界名因此改不动迭代数——边界名是槽，若在 head 内重读会现第二次 `load`，`TestForRangeEvaluatesBoundsOnce` 钉的就是这一点）；计数器取 `e.slot("i64")` 一枚（槽，源码不可命名，体内任何写都碰不到序列）；四块 `forhead<n>` / `forbody<n>` / `forcont<n>` / `forexit<n>`，head 重读计数器并以 `icmp slt i64` 比边界寄存器，body 开一帧（`pushEnv`/`defer popEnv`）后 `bindForPattern` 绑定本趟元素，`loopFrame{brk: exit, cont: step}` 使 continue 落在 **step 块**（落在 head 会原地打转）；step 重读计数器、`add i64 …, 1`、写回、回到 head。`bindForPattern`：通配头不绑定；具名头按既有标量绑定规则——体内写过该名（`e.assigned`）则 `bindScalarSlot` 取自有槽（**每趟重新绑定**，故 `i = i + 5` 不破坏序列），否则留寄存器 operand；其余头形（元组头归 T2-c）走 `e.bnd()`。

**一处计划修正（D13 披露）**：初稿打算把 `emitOverflowArith` 的溢出尾提为 `checkedTail`，与计数器步进共用。实现时两处推翻：① **步进溢出不可达**——step 块仅由 body 落入或 continue 转入，而 body 只由 head 的 `counter < hi` 真边进入，`hi` 是 i64 ⇒ 步进点 counter ≤ MaxInt64-1，和必不溢出；计数器是循环自身状态、非源码可见值，也不欠第 7 章任何面。② 该形态会在 step 块内多开一个块并给每个循环挂一段死 panic 面与一枚死常量。故步进用无检查 `add`（证明写在 `emitStep` 注释里），`emitOverflowArith` 保持原位，`checkedTail` 不提——**零漂移**（用户可见 `+ - *` 的 IR 逐字未动）。

**测试**：`internal/codegen/for_test.go` 7 例（计数序与两处 `br label %forhead`、边界只读一次（head 恰一次 `load`）、每趟重绑（恰两枚 alloca、step 段读计数器）、continue 落 step 且无终止符后落指令、break 落 exit、通配头不绑定、非 Range 源停 bnd）——初跑 6 红 1 绿（红报文为 `expected clean emission, got boundary "main bodies beyond the M9b statement set (…)"`）。真机电池上表全绿（`go build -o /tmp/we-now ./cmd/we` 后逐枚 build→clang→run）。conformance 新黄金 4 枚，逐枚真机先行定形：`run-for-range`（计数/continue/break/重绑/边界一次）、`run-for-nested`（嵌套 + start≥end 零元素 + 通配头）、`run-fn-for-range`（fn 体内循环 + 循环体内深 return）、`build-bnd-for-string`（负例：String 源仍 70）——713 = 709+4。

**停点黄金重锚 1 枚（T13 对账表首行）**：`test-fn-body-bnd` 的 for 源改 String 形（`for i in 0..3` → `for c in "abc"`）——原形所钉的 for-over-Range 已发射，若删则丢一枚 fn 体边界活钉；改源后仍在 `bndFnBody` 停且该形正是 T4 待接的面，翻绿时一并重锚。`TestM10bBndStops` 的 fn 体断言同步改锚（同理由，注释在码中披露）。

**发现（implement 期，处置已入 tasks.md）**：无新增——本任务除上述裁决与计划修正外未揭出既有缺陷。

### T2-c 前置核验：两面停在 T5，并入 T5 执行（2026-09-10）

T2-c 计划的两面（元组模式绑定、scope resource）在动工前逐面核验可行域，结论是**两面都由 T5 的机器承载**，T2 无可落之行——按 D13 披露，并并入 T5 执行（tasks.md T5 新增一枚 checkbox 承载 scope resource，元组面由 T5 既有的「元组 value 聚合 {f0,…}（模式解构/传参展开）」承载）。

| 面 | 形态 | 今日 check | 今日 build | 卡在哪 |
| --- | --- | --- | --- | --- |
| 元组模式 | `fn pair() -> (Int64, Int64) { return (1, 2) }` + `let (a, b) = t` | 0 | 70 `bndFnBody` | 元组**值**：聚合布局 `{f0,…}` / 多 retval 返回属 T5-D4/D8 |
| 元组模式 | `let (a, b) = (1, 2)`（字面量直解） | 0 | 70 `bndMainBody` | 同上 |
| scope resource | `foreign "c" { byres record File {}; fn weOpen() -> File; fn weClose(f: File) }` + `impl Releasable for File { fn release }` + `scope resource(f = weOpen())` | 0 | 70 `bndMainBody` | 释放面：见下 |

scope resource 的两处硬前置：① 资源值只能是 **byres record**——检查器把资源锁在 byres record 上（`impl Releasable for User`（普通 record）被 E1101 拒；m6b_test.go:31/33 在案），而 byres record 的构造/返回属 T5 记录值面与 D8 记录返回。② **释放调用无符号可呼**——`impl Releasable for T { fn release(mut self) }` 的 fn 体今日根本不发射：codegen 对 `*ast.ImplDecl` 是显式 erase（`case *ast.InterfaceDecl, *ast.ImplDecl: continue`，注释「a declaration-level fact only … neither emits IR」）。要让 `release` 存在并可派发，正是 T5 第一枚 checkbox「方法表静态派发：(型名, 方法名) → fn 表（inherent + interface 具体实现 + 默认方法体）」。

不落「半面」的理由：为 resource 先造一个只认 `Releasable` 的特例派发，等于把 T5 方法表的一角提前实现，且 foreign opaque 资源先落也仍要记录值面的构造/返回；与其造后即弃的窄机器，不如按 T5 的一次做齐（同 T2-a 对「发现」的处置口径：披露 + 重排，不静默绕）。T2 至此收口——语句集内可落的面已全落（T2 / T2-a / T2-b），余下的两面在 T5 归档。

### T3 表达式值形完备（单态）（2026-09-10）

**落地面**（tasks.md 四枚 checkbox）：if/match/块值的值位结果（结果 alloca 模式）、一元 `!`/`-`/`~`、`/` 与 `%`（整数检查算术 + 浮点域）、rune 值形（i64 域）、值位调用（`emitNumExpr` 的 `*ast.Call` 臂）。

**红证据（双基线）**。

单测面——pre-T3 树 `/tmp/we-base-src`（`git archive 2eae49c` 解出）+ 本窗 `internal/codegen/expr_test.go`（22 例）：**22 红**，21 枚边界红 + 1 枚缺陷红：

| 类 | 枚 | 报文 |
| --- | --- | --- |
| 边界红（形今日未发射，停在语句集 bnd） | 21 | `expr_test.go:253: expected clean emission, got boundary "main bodies beyond the M9b statement set (scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return)"` |
| 缺陷红（发射但序错） | 1 | `TestScopeValueBodyFirst`：`expr_test.go:538: slot %v0 is read at line 17, before its write at line 26 — the tail runs before the body` |

真机面（基线 `/tmp/we-t2base` vs B1a 树 `/tmp/we-t3`，逐枚 `we build .` 后跑二进制；本记录落笔前用当前树重跑）：

| 探针 | 形 | 基线 | B1a |
| --- | --- | --- | --- |
| a | main 体全形（值位 if/else-if 链/块值、`/` `%` 含负操作数、一元三形、rune、值位调用、短路） | 70 `bndMainBody` | build 0 / run 0 —— 16 行值全对 |
| b | fn 体同形 | 70 `bndFnBody` | build 0 / run 0 —— 11 行 |
| c | task 体同形（值位调用 + 短路 + `div`/`mod`） | 70 `bndTaskBody` | build 0 / run 0 —— `task-body` / `task-awaited` |
| d | `10 / d`，`d = 0` | 70 | build 0 / **run 1** —— `error: Panicked: division by zero` |
| d2 | `10 % d`，`d = 0` | 70 | build 0 / **run 1** —— `error: Panicked: division by zero` |
| d3 | `min / (0 - 1)`，`min = -2^63` | 70 | build 0 / **run 1** —— `error: Panicked: Int64 div overflow`（裁定 3 落地后为指名形；落地前为通用 `integer overflow`） |
| e1 | timeout scope 值形 + match 值形 | 70 | build 0 / run 0 —— `scope-tail-ok` |
| e4 | timeout scope 内值位 if-let | 70 | build 0 / run 0 —— `tmo-if-let-ok` |

负例三形（check 面即拒，故**无**发射面黄金、emitter 侧同类防御分支不可达、留作护栏）：`!1.5` → E0501；混值 if 臂 → E0501；无 else 值位 if → E0202。

**实现**：

1. **值形结果 alloca**（`emitValueForm` / `valueForm` / `emitArmBlock` / `storeValue`）——值位形先开一枚类型化槽，每臂帧内算值、`storeValue` 存入、`br join`；join 块 `load` 得值。unit 形不开槽（`TestValueUnitIfNoAlloca` 钉「无 alloca、无 join load」）。块值 `{ stmts; tail }` 的**体语句先发、尾表达式后发**——顺序即语义（缺陷②）。
2. **一元**（`emitUnary`）——`!` = `icmp eq i64 0` + `zext`；`-` 浮点 `fneg`、整数走 `llvm.ssub.with.overflow.i64` 复用既有溢出尾（`emitCheckedIntr` 本次从 `emitOverflowArith` 抽出，两处共用，用户可见 `+ - *` 的 IR 逐字未动）；`~` = `xor i64 x, -1`。浮点 `!`/`~` 停 bnd（无域）。
3. **`/` 与 `%`**（`emitDivMod`）——浮点 `fdiv`/`frem`（无陷阱面，ch7 的 IEEE 条）；整数默认两道守卫：零除 `divz<n>` → `@.dvz<n>` c"division by zero" + `__we_task_fail`，`/` 另加 `dovf<n>`/`dofk<n>`（`icmp MIN` & `icmp -1` → 复用 `@.ovf<n>` c"integer overflow"）——LLVM 把这两对都列为 UB，而 ch7 的整数算术是逐值 checked 的；`%` 无溢出患：该对余数恰为 0，`%s = select i1 %m1, i64 1, i64 %c` 一指令消解（`a srem 1 == a srem -1 == 0`）。源码拼写的常量除数且 ∉ {0, -1} 时旁路守卫直接发 sdiv/srem（热点形 `n % 2` 单指令）。
4. **rune 值形**——`numImmediate` 增 rune 分支 + `decodeRuneLiteral`；rune 在 i64 域，与整数算术同机（`'a'`/`'\n'`/`'\u{48}'`/`'\''` 四形钉在 `TestRuneLiteralValue` 与黄金 run-rune-binding）。
5. **值位调用**——`emitNumExpr` 增 `*ast.Call` 臂；承 T2-a「发现」① 的缺口（operand 位的用户函数调用），当面调用与实参位早已发射。
6. **phi 前驱块（缺陷①修复）**——见下。

**缺陷与计划修正（D13 披露）**：

① **phi 前驱块不匹配（实现期揭出，真机 clang 拒整模块）**——`emitLogic`（`&&`/`||` 短路）初版把 join 的 phi 表项硬编码为 `%lgcNrhs`，即操作数**开块时的标签**。RHS 自身含开块控制流（除法守卫、嵌套值形、会陷的调用）时，join 的实际直接前驱是 RHS 发射**结束时**所在的块（如 `dofk29`），clang 报 `error: invalid LLVM IR input: PHI node entries do not match predecessors!`。修：emitter 增 `curBlock`——`label()` 写入、`beginBody()` 为新体播种 `"entry"`（define 格式写出的那个块是唯一不经 `label()` 的块，故单列一个 helper，6 处换体全部改走它）、五处嵌套体（task/fn/test/省略体/driver）保存-恢复；`emitLogic` 取发射结束时的实际块作表项。**突变复验**：把取值改回 `rhsL`，`TestLogicShortCircuitPhiNamesRealPreds` 即转红并给出 clang 级诊断（该测试不做子串断言——它解析每枚 phi 表项、定位该前驱块、断言该块确有到 phi 所在块的边，因为「标签在 phi 里出现」与「标签是正确的前驱」是两回事）。**另一修法披露**：RHS 结果走槽 + join `load` 也能过，但会把全部短路形的 IR 改形（值不再走 phi），故不取。
   附带事实：既有短路测试在 pre-T3 树是**边界红**（RHS 的除法先 bnd），故缺陷①的真红面是**修后钉**（结构断言 + 黄金 run-logic-short-circuit——RHS 若求值必 panic 的短路序钉），与 design D10-3「潜伏错误」的判定一致。
② **design D10-1 措辞与实况不符**——D10-1 预期「float let 绑定丢域 ⇒ 下游按 i64 消费 double = 无效 IR」。实测 pre-T3 树三形（f/g/h 探针）**均停在混合域检查处 bnd**（`arithmetic and comparisons beyond the ratified numeric and Bool domains`），不发无效 IR。修复面与 design 一致（统一数值绑定路径 `bindNumericValue` 保留 `isFloat`，浮点 let 的下游消费走 double），已加 `TestFloatLetBindingKeepsDomain` 钉死（pre-T3 树红报文与上表边界红同形）。design D10 已加注。
③ **design D2 三处实现期补正**（已回写 design）——`/` 的 MIN/-1 守卫、`%` 的 select 消解、float 明确 `fdiv`/`frem`、`&&`/`||` 补「join 的 phi 表项必须指实际前驱块」。
④ **design D2 ② 的论证被实证推翻（本次订正）**——初稿写「字面量 0 已被 ch7 常量折叠 E0502 拒绝」。实证：`let q = 10 / 0` **check 干净**（rc 0、无诊断），build 0、run 1 —— ch7 的 E0502 只覆盖**溢出**（「a compile-time-detectable **overflow**」），折叠器对零除返回未定义并静默让路（`foldApply` 的 `/` 与 `%` 两个分支都 `ok=false`）。故零除的唯一捕获面就是运行期陷阱；design D2 该句已改写为实证陈述。**这一条与下方裁定项 2 同源。**
⑤ **普通 `scope { … }` 值形（预存边界，非 T3 回归；T5 移交）**——`let s = scope { … }`（**无 timeout**）在 B1a 树仍停 70 bnd（e2/e3 探针；`/tmp/we-t2base` 同源同停）。以临时 `WE_DEBUG_BND` 栈转储定位到 `emitNumExpr` 的 `*ast.Ident` 分支：`scopeType` 对无 timeout 的 scope 返回 `val` **本身**（typecheck.go:5720；带 timeout 才是 `Result<T, TimeoutError>`），绑定路径把该值 sum 化、随后按标量读 ⇒ bnd。`git show 2eae49c` 同路同停，确证预存。插桩与随之引入的 `os`/`runtime/debug` 导入已移除（gofmt/vet 复检干净）。T5 移交。

**D10 划账（tasks.md T11 已加括注）**：① float let 丢域（`bindNumericValue`，钉 `TestFloatLetBindingKeepsDomain` + 黄金 run-float-values）、② `&&`/`||` 短路（钉 `TestLogicShortCircuit*` + 黄金 run-logic-short-circuit）、③ 块值序（钉 `TestScopeValueBodyFirst` + 黄金 run-scope-value-order）**均已随 T3 落地**；④ 窄整型逐宽溢出检查留在 T11。

**测试与黄金账**：`internal/codegen/` 105 例全绿（本窗新增 `expr_test.go` 26 例：值形 5、一元 3、`/`/`%` 4、浮点 3、rune 1、值位调用 1、短路 3、块值序 1、D10-1 钉 1、位运算族 3、指名消息 1）。conformance **713 → 728**（15 枚，逐枚真机先行定形、手钉期望、首跑即绿、未用 `WE_UPDATE_GOLDEN`）：run-div-mod-values（商/余含负操作数 + 常量除数旁路）、run-div-zero-panic、run-mod-zero-panic、run-div-overflow-panic、run-call-in-operand（`bump(5)+1`、`bump(5)==6`、`!(bump(0)==1)`、嵌套 `bump(bump(1)+1)+bump(2)`）、run-value-forms（值位 if/else-if 链/块值/unit-if/scope timeout 值形 + match 值形）、run-scope-value-order（体先于尾）、run-unary-ops、run-rune-binding、run-logic-short-circuit、run-float-values（`fadd`/`fdiv`/`frem`/`fneg`）、run-bitwise-ops（六值：`and`/`or`/`xor`/`shl`/`shr`/算术 `ashr`）、run-shift-amount-panic、run-shift-value-panic、run-add-overflow-panic。`go test -count=1 ./...` 14 包全绿；gofmt/vet 干净。本任务无停点黄金重锚（T13 对账表不受影响）。

**M15 面连带修正（T3 的直接后果，已划账 T12）**：`%` 一发射，两处 M15 工件即失真——① `traps_test.go` 的 `control-04/modulo-decline` 消亡（突变后的参考解成了干净程序、不再落 test-malformed；陷阱纪律：无捕获面 = 修清单），`control-04/traps.md` 条目同步改写为「已退役」；② `blackbox_test.go` 的 test-malformed 载体重锚 `control-04`（模）→ `control-03`（字符串相等 decline，语料现存唯一 check-clean/test-70 形）——该载体**亦有寿命**，字符串相等落地时须再寻载体。校准例计数 64 → 62 达成（63 于 T2-a、62 于 T3）。③ `docs/benchmarks.md` 双语 run-face anchoring 段的算子句做**最小修正**（`%`、`/` 已非边界、字符串拼接 `+` 仍是）；该段的全量重写（槽放宽/字符串/记录方法/let 重赋值面）仍归 T12 第二枚 checkbox。三项均在 T3 之后由 `go test ./...` 直接逼出（红→绿），非提前扩面。

**发现（implement 期）**：phi 前驱块缺陷（①，已修 + 突变复验 + 结构钉）；D10-1 措辞不符（②）；D2 零除论证被推翻（④）；预存普通 scope 值形边界（⑤，T5 移交）。四者均已披露，无静默绕过。

**四项提请裁定与回执（2026-09-10，均属既有缺口，非本任务引入）**：

| # | 事项 | 裁定 | 落地 |
| --- | --- | --- | --- |
| 1 | `/`、`%` 的舍入与符号约定（ch7 对两算子全无条款） | 维持截断读法（商向零、余数随被除数，Rust/C 同） | 登记 roadmap **follow-up #18**，双语在册 |
| 2 | 除零语义（规范全文无「division by zero」字样） | 维持运行期检查陷阱（零除唯一捕获面） | 登记 roadmap **follow-up #19**（含「常量零除是否升为编译期诊断」一并交规范裁） |
| 3 | 溢出陷阱消息文本（ch14 要求指名操作；构建自 M5 起 emit 通用文案） | **本变更内改指名形** | 本记录下方「裁定落地」第 2 条 |
| 4 | 位运算二元族 `& \| ^ << >>`（check 通过、build 停 bnd、无 checkbox；越界移位 ch7 未裁且 LLVM 为 poison） | **并入本变更**，越界取陷阱 | 本记录下方「裁定落地」第 1 条 + 登记 roadmap **follow-up #20** |

**裁定落地（T3 追加面）**：

1. **位运算二元族**（tasks.md T3 第五枚 checkbox，`emitBitwise`）——`& | ^` 全值域直发（`and`/`or`/`xor i64`，无陷阱面：任何位形都是可表示结果）。`<<`/`>>` 两道守卫：① 移位量域——LLVM 的移位指令对量 ≥ 位宽（或负）是 poison，`icmp ult i64 b, 64` 守之（无符号比较一并覆盖负量），越界 → `Int64 shift overflow`；② `<<` 的丢位——LLVM 的 `shl` 静默丢弃移出顶端的位，正是 ch7「绝不静默回绕」禁止的形（`1 << 62 << 2` 若静默成 0，而 `(1<<62) * 4` 陷阱，两者不可并存），故移回校验 `ashr v, b == a` 不还原即同上陷阱；`>>` 无此面（右移丢的只是低位，那正是右移的语义），取**算术移位** `ashr`（有符号操作数的保号读法）。**红证据**：pre-位运算树（`/tmp/we-t3`）五枚算子逐枚真机 check 0 / build 70 `bndMainBody`；**突变复验**：撤 `emitNumExpr` 的 dispatch 臂，三枚新测试转红且报文与真机同形（`expected clean emission, got boundary "main bodies beyond the M9b statement set (…)"`）。真机绿：`and/or/xor/shl/shr/ashr` 六值全对；量越界（`var k = 64; 1 << k`）与丢位（`1 << 62` 再 `<< 2`）两形 run exit 1、stderr 精确 `error: Panicked: Int64 shift overflow`。
2. **陷阱消息指名操作**（tasks.md T3 第六枚 checkbox）——`emitCheckedIntr` 增消息实参，新增 `msgConst`/`trap` 两个小 helper（后者把四处重复的 panic 尾收成一处，IR 逐字未变）；`Int64 add/sub/mul/div/neg/shift overflow` 各带己文，通用 `integer overflow` 退役。真机三形验证：`max + k` → `Int64 add overflow`、`min / -1` → `Int64 div overflow`、`-min` → `Int64 neg overflow`（`0 - min` 走二元减法，报 `Int64 sub overflow`——两形正确区分）。窄整型的逐宽命名（`Int8 add overflow` 形）留 T11-D10④。
3. **三枚 follow-up 登记**（`docs/roadmap/0000-reference-implementation.md` + `.zh.md` 双语，docs_sync 33 对复验）：#18 除法/取模的舍入与符号约定、#19 除零语义、#20 越界与溢出的移位语义。

**停点**：T3 六枚 checkbox 全绿、全部验证行可复核；四项裁定均已回执并落地。T4（字符串全链）为下一任务。

### T4 字符串全链（2026-09-10）

**落地面**（tasks.md 三枚 checkbox）：① `runtime/c/str.c` 新符号族（拼接/相等/ch17 两访问层/值转串族 + OOB panic 面）；② 发射面（String 绑定从字面量对扩为运行时值 {ptr operand, len operand}；拼接/相等/方法族/插值）；③ for-in String 源（自 T2 移入）。

**红证据（双基线）**。

单测面——T4 新测试面（`internal/codegen/str_test.go`、`internal/parser/str_test.go`、`runtime/str_test.go`）对 pre-T4 树（`git archive HEAD` 解出的 `/tmp/we-t3src`）是**载体红**：新面骑的 AST 场与运行时导出 pre-T4 不存在。

| 测试面 | 首红（pre-T4 树 + 本窗测试） |
| --- | --- |
| `internal/codegen/str_test.go` | `unknown field Segs in struct literal of type ast.Literal` |
| `internal/parser/str_test.go` | `lit.Segs undefined (type *ast.Literal has no field or method Segs)` |
| `runtime/str_test.go` | `undefined: StrHeader` |

这三枚不是边界红而是载体红——设计 D3 把插值洞归 parser 面（裁定③），洞的 AST 场与运行时导出是本任务自己引入的，测试与实现同批落地时其首红只能是「载体不存在」。**T4-3 的四枚则是常规边界红**：`TestForStringSourceWalksRunes / BindsRunes / StepsOnce / BreakLeavesTheWalk` 对 pre-T4 树逐枚 `expected clean emission, got boundary "main bodies beyond the M9b statement set (…)"`（与下方真机面同形）。

真机面（基线 `/tmp/we-t3`（T3 期二进制，无 String 面）vs 本树 `/tmp/we-t4now`；本记录落笔前重跑，逐枚 `we run .`）：

| 探针 | 形 | 基线 | B1a |
| --- | --- | --- | --- |
| a | 拼接 `io.println(a + b)`，`a="ab"`、`b="cd"` | **check 70** `arithmetic and comparisons beyond the ratified numeric and Bool domains (spec gap; roadmap follow-up)` | run 0 —— `abcd` |
| b | 相等分支 `if a == b { … }` | check 干净、**build/run 70** `bndMainBody` | run 0 —— 走正确分支 |
| c | 插值 `io.println("n=${n}")`，`n = 7` | 70 `bndMainBody` | run 0 —— `n=7` |
| d | for-in String `for c in "abc" { … }` | 70 `bndMainBody`（退役黄金 `build-bnd-for-string.json` 即此形本体） | run 0 —— 逐码点 |

探针 a 与 b/c/d 的**停面不同**，这一差别决定了本次 typecheck 面必须动一处：字符串拼接在 pre-T4 连 check 都过不去（二元域门只认数值/Bool 同型对，String `+` 落 `bndDomainGap`），故 `binaryType` 增 String 门（见实现 5）；相等/插值/for-in 则是 check 干净、发射面停 `bndMainBody`——前者印证 ch10「`==` compares base types only」早已把 String 纳入，后者是发射集未含该形。

**真机三程序首跑全红（缺陷面）**：三枚黄金（run-string-concat / run-string-equality / run-string-interp）**首次真机运行三枚全红**——不是黄金笔误，是三个实现缺陷（下 ①②③），其中 ①② 是**静默错误**（IR 干净、clang 不报、exit 0、输出错值）。这正是「先实现后写测试」的风险实证：三枚黄金与 18 枚内部测试是同一批写就的，真正把缺陷逼出来的是**真机首跑**而非测试。故本任务把 13 枚突变复验当作收口必要条件（下）。

**实现**：

1. **运行时符号族**（`runtime/c/str.c` 238 行 + `str.h` 39 行）——`struct we_str { const char *p; long long n; }` 双八字节全 INTEGER class，clang 双寄存器返回，正是发射面 IR 拼的 `{ ptr, i64 }`。十枚导出：`concat`（新分配、逐段 memcpy）、`eq`（先比长再 memcmp，返 i64 真值）、`runecount`/`charat`（共用 `utf8_step` 解码器故互洽）、`byteslice`（**内部指针**——源长生不灭，共享不可观察）、`of_i64`/`of_u64`/`of_f64`/`of_bool`/`of_rune`。两枚 OOB 面走 `__we_task_fail`（ch14 族），消息指名操作（`String charAt out of range` / `String byteSlice out of range`）。分配器失败即 `abort()`（gc.c 分块同姿势：无回退的运行时停）。
2. **发射面**（`internal/codegen/codegen.go` +799/−61）——`emitStringExpr` 五臂（Literal 含洞 / Binary 拼接 / Call / Ident / Member 字段链）、`concatStr`（调 `__we_str_concat` 取回 struct 再 `extractvalue` 成双 operand）、`strCompare`（`__we_str_eq` → `icmp ne i64 …, 0`）、`emitStrMember`（ch17 四成员）、`emitInterp`（洞逐段求值 → 值转串 → 折叠为左结合 concat 链）、`valueKind` 分类域（skNone/skStr/skI64/skU64/skF64/skBool/skRune）。
3. **插值管道（跨三包）**——`lex.Holes`（洞区间的第二遍扫描，报表 `HoleRegion{Start, End, Line, Col}`）+ `lex.ScanAt`（区域重定位扫描，`runAt` 为两面共用体）/ `ast.Literal.Segs/Holes`（裁定③：parser 建洞 AST，`len(Segs) == len(Holes)+1`）/ parser 逐洞解析表达式（洞内坐标重定位到文件坐标；洞内字面量可再含洞——嵌套形有测试钉）/ typecheck **不类型化洞**（裁定③）。
4. **for-in String 源**（`emitForString`）——`runecount` 定数一次（头前）、每趟 `charat` 取一枚码点（跑马灯式，无字节游标）；计数槽 + 独立 step 块（continue 落点）；元素绑 Rune 域（`bindForPattern(..., skRune)`）。
5. **typecheck 二元域门一处放宽**——`+` 在 String×String 上放行（拼接是 ch10 已含 String 的 `==` 配对的 `+` 读法），`- * / %` 与序关系仍停 `bndDomainGap`。
6. **两枚小型修复**（缺陷②③的落地载体）——`splitIntSuffix` + `literalStrKind`（u64 后缀的分类与超 i64 幅值的位形重写）。

**缺陷与计划修正（D13 披露）**：

① **`argIsScalar` 把一切 Binary 当标量**（真机 red，**静默错值**）——`io.println(a + b)` 的实参分类只看节点形状不看域，String 拼接被送进 i64 渲染器 ⇒ IR 干净、exit 0、输出是拼接结果的位形读数。修：`*ast.Binary` 臂改问 `e.valueKind(v) != skStr`，把判定交回分类器。钉 `TestConcatAsIoArgumentIsNotScalar`（突变 M7 守门）。
② **相等谓词取反**（真机 red，**静默错值**）——`strCompare` 初版对 `==` 发 `icmp eq %eq, 0`、对 `!=` 发 `icmp ne`，语义整体反转（实测输出 `ne\nsame\nprefix-ne`）。修：`pred` 取 `ne`、`!=` 改 `eq`——「运行时答 1/0，`==` 即该答案非零」，结果按 Bool-as-i64 寄存位形加宽。钉 `TestStringEqualityUsesTheRuntime` / `TestStringInequalityIsNe`（突变 M2）。**与 T3 缺陷① 同族**：都是「真机才暴露、单测同批写就查不出」的形。
③ **`u64` 后缀字面量不可达**（真机 red）——`let m: UInt64 = 18446744073709551615` 揭出两处缺口：`scalarImmediate` 对 `ParseInt` 失败的 u64 幅值直接 bnd（超 i64 的字面量没有寄存器形），且 `litStrKind("int")` 恒 skI64（裸 u64 字面量分类错）。修：`splitIntSuffix` 剥后缀，失败且后缀以 `u` 开头时走 `ParseUint` → 按位形重写（同一批位，按其有符号读法写出）；`literalStrKind` 令 `u64` 后缀 → skU64，接线于 `emitLetBinding` 与 `valueKind`。钉 `TestUInt64LiteralCarriesItsSuffix` / `TestUInt64LiteralPastI64KeepsItsBits`（突变 M8/M9 守门）。
④ **design D3 分配协议被实证推翻（实现期订正）**——设计稿写「gc 域外裸分配 **+ 立即 root push 构造协议**（M8 先例）」。实证否定后半句：裸 malloc 的字节缓冲**不是** gc 对象（无头部、无描述符），把非 gc 指针推上 shadow root 栈会被收集器读成块头。协议订正为「malloc 裸分配、**从不推根、永不回收**」（裁定①）。连带一条正面结果：字面量常量与家族缓冲都长生不灭 ⇒ `byteslice` 可返回**内部指针**，设计稿的 `out` 出参形因此也无必要（见⑤）。回收登记 roadmap **follow-up #21**（`docs/roadmap/0000-reference-implementation.md` + `.zh.md` 双语，docs_sync 33 对复验；回收归 ADR-0003 门后的分配器工作，B2 收编）。
⑤ **design D3 符号表三处实现期偏差**——`concat` 去 `out_len` 出参、改 `struct we_str` 返回（省一次间接写）；`eq` 返 i64 而非设计稿的 `i1`（发射面按 Bool-as-i64 读真值，见②）；`bytelen` **不实现**——`byteLength` 折叠为长度操作数本身（零指令），设计稿的独立符号无消费方。
⑥ **两处实现定义（规范沉默位，随本记录披露）**——a. 无效 UTF-8 的读数：`utf8_step` 对任何畸形序列读 U+FFFD 且推进一字节，使 `runecount`/`charat` 互洽且对任意字节串全定义（ch17 未裁）；b. `of_f64` 的十进制形：最短往返（裁定②，`%.*g` 逐精度试到 `strtod` 还原，至多 17 位），非有限值走 `%g` 自己的词（`inf`/`-inf`/`nan`），负零保号。两者属「规范未定、实现定义、随完成记录披露」的**渲染文本**自由，不登记 follow-up——与 #18/#19/#20 不同，那三枚是待裁定的**语义**缺口。
⑦ **emitter 的 `emitInterp` 段数守卫（保留）**——`len(Segs) == len(Holes)+1` 是 parser 的契约；emitter 收到不等长的对（测试手工构造的 AST）即 bnd 而非越界。本任务写测试时正撞上这一形（⑧③）。
⑧ **测试侧笔误三枚（本任务内揭出并订正；实现正确、测试写错）**——a. `TestInterpolationDiscardStillRuns` 断言「单洞不发 concat」，但段非空时 concat 本就该发 ⇒ 改用双侧空段形；b. `TestCharAtRendersAsRune` 期望写成 `%struct.we_str` 返回（`charat` 实返 i64）；c. `TestRuneCountRendersAsInt` 段数少一（触⑦守卫）⇒ 补段。三枚按 D13 披露订正**测试**而非改实现。
⑨ **run-string-concat 黄金钉子自身笔误**——`io.println(c + a)` 期望写 `cdab`，真机（正确）输出 `abcdab`。程序对、钉子错 ⇒ 改钉子。
⑩ **第三枚 checkbox 的黄金处置改判（计划修正）**——checkbox 原文写「`build-bnd-for-string` 黄金翻绿重锚」；实现期改判为**退役 + 新锚 `run-for-string`**：边界黄金翻绿后只剩「build 退出 0」这一弱断言，而 run 面黄金断言逐码点输出（含 UTF-8 多字节与空串零元素），严格更强。退役件留档 `/tmp/build-bnd-for-string.json.retired`。
⑪ **harness 落点与 checkbox 措辞的偏差（措辞级）**——checkbox 写「C harness 单测（`runtime_test.go` 扩容）」；仓库实际惯例是 per-family 文件（`conc_test.go`/`gc_test.go`/`sched_test.go`），故落 `runtime/str_test.go`。语义无差，循仓库惯例。

**突变复验（13 枚，`/tmp/mutate.py`；2026-09-10 收口时复跑取证）**：对 `codegen.go` 逐枚施加单点突变 → 断言目标测试转红 → 还原断言转绿。**13/13 全捕获、13/13 还原转绿、汇总 `all mutations caught`**（exit 0）。逐枚：M1 拼接丢弃右操作数、M2 相等谓词取反、M3 String 成员门摘除（五枚成员钉齐红）、M4 插值丢洞（三枚插值钉；**捕获形是 panic 而非断言**——`_ = res` 令 concat 结果丢失、随后解引用空 pair，故报文为栈迹）、M5 `valueKind` 恒 skNone（八族钉齐红）、M6 `byteLength` 走码点、M7 `println` 把 String join 当标量（即缺陷①的守门）、M8 u64 后缀失效、M9 u64 超 i64 不可达、M10 walk 产出字节索引、M11 String 源回落边界、M12 元素丢 Rune 域、M13 walk 数字节。M7/M8/M9 三枚是**为已修缺陷配的守门钉**：缺陷若回归，测试即转红。

**测试与黄金账**：`internal/codegen/` **105 → 132** 例全绿（+27：`str_test.go` 23 例、`for_test.go` T4-3 四枚）；新增 `internal/parser/str_test.go` 8 例（洞切分/段转义解码/洞表达式/洞内坐标/`$` 非 `{`/洞内错误/嵌套洞/原文本保真）、`runtime/str_test.go` 2 例（C harness 双形：happy path 逐条 `CHECK`；panic 面经调度器夹具断言 `tag=1 msg=…` 两行）。conformance **728 → 731**（+4 新枚、−1 退役）：**run-string-concat**（拼接链含空串与多字节片段）、**run-string-equality**（相等/不等两分支 + 前缀形不等）、**run-string-interp**（基类型覆盖 + String 恒等 + 相邻洞 + 成员链洞）、**run-for-string**（`"abc"` 逐码点 + `"héllo"` 6 字节 5 码点 + `""` 零元素）；四枚逐枚真机先行定形、手钉期望、**首跑即绿**（未用 `WE_UPDATE_GOLDEN`）。退役 `build-bnd-for-string.json`（见⑩）。`go test -count=1 ./...` **13 个有测试的包全绿**（另 2 包无测试文件）；gofmt / `go vet ./...` 干净。停点黄金重锚一枚：`test-fn-body-bnd`（见下 M15 面⑦，T13 对账表同步）。

**M15 面连带修正（T4 的直接后果，已划账 T12）**：字符串面一发射，control-03 的两枚校准例即失真——① `concatenation-boundary` → **`concatenation-latent`**（拼接发射运行、错值直达参考测试：latent / test exit 1）；② `string-equality-decline` → **`string-equality-latent`**（同上；突变形改用 `let` 形以留在 check 干净面）；③ **新增 `string-mutable-binding-decline`（TestMalformed）** 顶替 test-malformed 桶的空位——`var r: String = s` 形（String 可变绑定）是 T4 **不覆盖**的发射集边界（见下「提请裁定」），恰好 check 干净 + test exit 70，正是该桶要的形。④ `blackbox_test.go` 的 test-malformed 载体注释改写（第三枚载体：control-04 模 → control-03 字符串相等 → control-03 可变 String 绑定）；⑤ `control-03/traps.md` 四条重写（两条注明「ratified by T4 … latent」，一条注明「outside the emission set … 该钉在可变绑定落地时转 latent」）；⑥ `m8_test.go` 的 M8 边界例重锚——原「arithmetic in io argument」在拼接发射后不再是边界 ⇒ 改「record value in io argument」（greeter 记录形）；⑦ `m10b_test.go` 的 fn 体钉与黄金 `test-fn-body-bnd` 双双重锚到 List 源（`for x in [1, 2, 3]`）——两枚原都拿 for-in String 当 `bndFnBody` 的载体，T4-3 一发射该载体即失效。校准例计数不变（62——latent 化只换桶不改枚数）。

**发现（implement 期）**：三个真机缺陷（①②③，其中两枚静默错值）；design D3 分配协议被实证推翻（④）；符号表三处实现期偏差（⑤）；两处实现定义披露（⑥）；黄金处置改判（⑩）；harness 落点偏差（⑪）。全部披露，无静默绕过。

**提请裁定与回执（2026-09-10）**：

| # | 事项 | 裁定 | 落地 |
| --- | --- | --- | --- |
| 1 | **String 可变绑定**（`var r: String = s` 及 String 名重赋值）——T4 章程（checkbox 2「String 绑定 {ptr operand, len operand} 运行时值形」）只覆盖值与 `let`，未含**两字槽存储面**；今日该形 check 干净、test 面 exit 70 | **并入 T9「sum 物化泛化 + 槽放宽」**（该任务 checkbox 已含「String 参数双标量展开」，同族；本变更内不实现，边界保持诚实） | 下方「裁定落地」 |

**裁定落地（T4 提请面）**：design D3「发射集边界」段改为记录裁定结论；design D8「参数槽放宽」条加注**本面扩形**（String 两字槽存储面归 T9）；tasks.md T9 第二枚 checkbox 加括注（含落地时连带：`control-03/string-mutable-binding-decline` 校准转 latent，须再寻 test-malformed 载体——T3 先例）。该缺口的机械钉在本裁定前已就位（三处括注同步：`traps_test.go` 注释、`blackbox_test.go` 注释、`traps.md` 条目），故口径自始未漂移，裁定只是把它从「待定」变为「有主」。

**停点**：T4 三枚 checkbox 功能面全绿、红证据与验证行可复核；13 枚突变复验全捕获；一枚提请裁定已回执并落地（并入 T9）。T5（记录/元组/newtype 值面 + 方法表）为下一任务。

### T5 记录/元组/newtype 值面 + 方法表（2026-09-10）

**落地面**（tasks.md 四枚 checkbox）：① 记录标量字段读（getelementptr+load 统一成员读）+ `with &` 拷贝构造 + value record 整值拷贝；② 方法表静态派发（inherent + interface 具体实现 + 默认方法体）+ `mut self` 字段写 + `self.field=` 语句；③ newtype 零擦除恒等 + 元组 value 聚合（模式解构/传参展开）；④ scope resource 语句（自 T2 移入）——多绑定逆序释放 + 早出口穿透，release 经方法表派发到 `impl Releasable` 的 fn 体。

**红证据（双基线）**。四枚 checkbox 各有一枚真机探针（`/tmp/t5probe/p1rec`·`p2meth`·`p3newtuple`·`p4scope`），对 pre-T5 树（`/tmp/we-t5base`）逐枚 **check 干净 / build 70** `main bodies beyond the M9b statement set (…)`：

| 探针 | 形 | 基线 | B1a |
| --- | --- | --- | --- |
| p1rec | `Point{x:1,y:2}` / `Point{x: p.x + 1 with &p}` / `User{name:"a",age:3}` / `u.age` | check 0 / **build 70** `bndMainBody` | build 0，run 0 —— `2 2 3` |
| p2meth | `impl Counter { fn bump(mut self) -> Int64 { self.n = self.n + 1; self.n } }` 两跳 | check 0 / **build 70** | build 0，run 0 —— `1 2` |
| p3newtuple | `newtype UserId(Int64)` + `fn raw(id: UserId) -> Int64 { return id.value }` + `fn pair() -> (Int64, Int64)` + `let (a, b) = pair()` | check 0 / **build 70** | build 0，run 0 —— `42 3` |
| p4scope | `scope resource(a = Res{tag:2}, b = Res{tag:3})` + `impl Releasable for Res` | check 0 / **build 70** | build 0，出块打印 `body`；run 1 —— 该探针的 release 体是 `panic("rel")`，`error: Panicked: rel`（panic 无展开清理，见「边界」） |

四枚的 check 均干净、停点全在发射面，故本任务不动 typecheck（与 T4 的「探针 a 连 check 都过不去」形成对照）。表内 B1a 列为收口时对 `/tmp/we-t5final`（当轮源码构建）重跑所得。

**实现**：

1. **记录值面**（T5-1）——成员读统一为 `chainOf` + `walkHops` + `fieldSlotOf` + 四臂分类（`fkStr` 双词 / `fkScalar` i64 / `fkF64` double / `fkRef`·`fkVal` 指针），`p.x` 不再只认 String 字段尾；`with &` 更新式走「新分配 + 未命名字段自基值拷贝 + 命名字段覆写」。值记录的整值拷贝落 `emitRecCopy`（见披露①）。
2. **方法表**（T5-2）——`collectImpl` 把 inherent 方法、interface 具体实现与默认方法体一并收进 `methods[modKey + "." + Head + "." + name]`，符号 `@<module>.<Head>.<name>`（头型段使方法与同名裸 fn 不撞）；接收者走 define 首参 `ptr %self`，`gcEnv["self"]` 绑到接收者的记录键；调用点直呼符号、**不走槽**（派发是静态的，槽是 mock 的面）；interface 自身零符号。`mut self` 字段写与 `self.field=` 语句经 `emitFieldStore` 落 store。
3. **newtype 与元组**（T5-3）——newtype 的零擦除落在 `classType` 顶部的 `derefNewtype`：参数位、返回位、元组元素位走同一个分类器，构造即底层值的恒等；`.value` 解包靠 `ntEnv`（名字的静态类型是新包体这件事，值里没有任何东西说得出——那正是擦除）。元组值 = 栈聚合句柄（`tupEnv`）：构造逐元素存偏移、解构逐位 load、跨边界逐字展开、对岸 load/store 重建。
4. **scope resource**（T5-4）——`nest` 统一嵌套序数（每开一个 scope expr 或 scope resource 取下一号），`loopFrame`/exit 记 `depth`；`unwind(depth)` 把 `scopeLive`/`resFrames` 中 `depth ≥ 参数`者合成 opening 列表、按 depth **降序**发射，故内层先出、块出口释放先于外围函数的 defer；`releaseRes(live)` 倒序取 `e.methods[key+".release"]` 经 `classify` + `emitCallCore` 直呼其符号。被 `return`/`break`/`continue` 穿透的出口与正常出块走同一条 `unwind`。

**缺陷与计划修正（D13 披露）**：

① **记录拷贝的运行时面落成站点内联（T5-1，计划修正）**——design D4 点名 `__we_rec_copy` 运行时新面，但根推送按 body 记账（`e.pushes` 在每个 body 出口弹），callee 的 push 无法由 caller 平衡。故拷贝在站点内联展开为 `emitRecCopy`，其 push 记在发起的 body 上。面是真、拼写是发射器的；理由写在 `record_test.go` 文件头。
② **章 6 的隐式尾返回随本任务落地，且首版过宽（T5-2 落地、T5-4 收窄，真机探针揭出）**——ch10 三枚黄金的方法体全用「体末项表达式即返回值」的形，故尾项识别在 `emitFnDefine` 增 `*ast.ExprStmt` 臂。但章 6 只在**声明了返回类型**时把体块值当隐式返回（`fnAbiOf`：`d.Ret == nil` 即未声明），未声明者归第 8 章值丢弃规则治理（「类型非 unit 的末项表达式 MUST 以 `let _ =` 显式丢弃，unit 类型无需仪式」）。该臂对 void fn 一并提升，而章 19 的释放习语恰是「裸尾调用」⇒ `impl Releasable` 的 release 体被按值返回分类、停 bndFn。收窄为 `if fd.decl.Ret == nil { break }`。单测的释放体一律写 `&ast.Return{}`，故该过宽只在真机上现形（钉 `TestVoidBodyTrailingExpressionIsAStatement`，突变 M9 守门）。**main 体的同形仍停**，归 T9「fn 尾返回停点退役」。
③ **`callStrKind` 不认方法表（T5-2）**——String 返回的方法调用无法进 String 域（`self.greet() + "!"` 停），补方法表臂。
④ **计划里的 `p.0` 位置成员读在本语言并不存在（T5-3，凭空造面已删）**——parser 对 `p.0` 报 E0105「member names are identifiers」，章 8「元组模式与解构」把**模式**定为唯一的元素读者（design D4 亦只写「模式解构/传参展开」）。首版三枚单测与探针按 `p.0` 写成；已删除该分支（`emitMemberValue`/`memberKind` 的元组成员臂）并按真实源面重锚三枚单测（构造的可观测量改走解构）。
⑤ **`fnRetOperand` 的 abiStr 臂只认 Ident/Literal/Binary/Call（T5-3）**——`return n.value`（String 底层 newtype 的解包返回）停在 bndFn，真机电池揭出，补 `*ast.Member` 臂。
⑥ **`emitForeignCall` 的不透明实参只查 `prims`（T5-4）**——而方法接收者 `self` 绑在 `gcEnv`；不透明头没有字段可传，`self` 是它唯一的把手（章 19 的释放习语把 `self` 直传原生 `close`）。补 `gcEnv` 臂并按 `e.opaques[g.rec]` 判别（钉 `TestForeignOpaqueReceiverPassesToNativeClose`）。
⑦ **`emitRecordValue` 的 Call 臂只认 `ckGc`（T5-4）**——不透明返回是 `ckPrim`（裸指针）⇒ scope 头拒不透明值。析出 `opaqueKeyOf`（自 `isOpaqueRef`）+ `calleeDecl`，新增 `emitResHead` 供 scope 头走。
⑧ **三处单测盲区（测试侧，均经突变复验揭出并补钉）**——a. `TestTupleConstructionAggregates` 只钉两处 store 的存在、不钉偏移（两元素重叠时单测仍绿、黄金红）；b. `TestRecordScalarFieldRead` 只钉 16/24 两偏移都在用、不钉「哪个字段读哪个字」（全部读右移一字时该断言照样成立）；c. `TestRecordUpdateCopiesTheBase` 的「有 load」断言被随后那次字段读满足，拷源改成常量仍绿。三枚按 T5-3 先例补钉（b 新增 `readsAt` 辅助——gep/load 成对计数，构造的 store 自带 gep 故只数 gep 分不清读与写），补后突变 M1/M2 在单测层亦转红。
⑨ **跨栈放电的形（T5-4，测试侧）**——`scope { loop { scope resource { break } } }` 的 break 只跨 loop 的 depth（scope 的 depth 更低），`unwind` 正确地**不**放电 scope；该形因此不是「跨两条栈」的载体，改以 `return` 穿两条栈（`TestScopeResourceNestedInScopeExprDischargesInnermostFirst`）。

**边界（披露，均安全停在 bnd 而非误发射；已逐条真机取证）**：无尾返回的非 void fn 停在 bnd（check 已按 E0501 拒之，属正确防御面而非缺陷）；panic 路径无展开清理（`emitPanic` 直发 `__we_task_fail` + unreachable，与既有 defer 行为同规，非本任务引入——p4scope 探针的 release 体即 `panic("rel")`，实测输出 `body` 后 task fail）；元组插值直渲染、嵌套元组（构造与解构两侧）、newtype 作 record 字段、newtype 包 record 的方法派发、泛型 newtype（T5-3 逐条真机取证 check 0 / build 70，见该 checkbox 落点）。

**突变复验（9 枚）**：对 `codegen.go` 逐枚施加单点突变 → 断言目标测试转红 → 还原转绿；每枚在**单测层**与**黄金层**（`TestGoldenCases/<对应 run 黄金>`）各记一次。9/9 全捕获、9/9 还原转绿。逐枚：M1 字段读偏移 `off → off+8`（单测·黄金）、M2 拷贝的源操作数改常量（单测·黄金）、M3 方法接收者不传（`ptr %self` 摘除；4 枚方法钉齐红，单测·黄金）、M4 `mut self` 字段写丢弃（单测·黄金）、M5 newtype 停止擦除（单测·黄金）、M6 元组偏移步长归零（单测·黄金）、M7 release 正序（单测·黄金，三枚转红）、M8 `unwind` 排序反向（**单测红、黄金绿**——既有三枚 run 黄金的程序都不含「scope expr 套 res 块」的嵌套形，该面由单元测试在 IR 指令序上钉住）、M9 章 6 收窄回退（缺陷②的守门钉转红）。M1/M2 两枚即披露⑧ b/c：**首轮跑出「单测绿·黄金红」**，补钉后两层齐红。

**测试与黄金账**：`internal/codegen/` **132 → 164** 例全绿（+32：`record_test.go` 8、`method_test.go` 7、`tuple_test.go` 7、`scope_res_test.go` 8、`ffi_test.go` +1、`return_test.go` +1）。conformance **731 → 738**（+7 新枚、0 退役）：**run-record-read-update**（`1 2 3 2 p` / `13 2 p`——更新改写 x 而基 p 不动）、**run-method-dispatch**（`1 2` 计数器两跳 / `hi!` 默认体派发回头型自身方法）、**run-newtype-erasure**（`42 id 41`）、**run-tuple-value**（`7 seven` / `7 30` / `11`）、**run-ch13-scope-res-order**（`in` / `21`——`n = n*10 + id` 下 `21` 即逆序且恰一次）、**run-ch13-scope-res-early-exit**（`7` / `39` / `0` / `3909`——早 return 穿出与正常出块各一，`9` 是 defer 标记，`39` 证释放先于 defer）、**run-ch13-scope-res-opaque**（`in` / `21` / `7` / `213`——foreign 块的不透明 `byres record CFile` 作头，两出口都经原生 `fclose`，程序带 `native/handle.c`）。四枚 scope-res 与三枚值面黄金均**真机先行定形、手钉期望、首跑即绿**（未用 `WE_UPDATE_GOLDEN`）。重锚一枚：**build-ch10-method-call-boundary**（exit 70→0、stderr 清空、顶层 `files` 键移除——与 `build-bnd-body`/`build-bnd-fn` 先例同规：**边界黄金翻绿只改期望、不改名**）。全量黄金三轮全绿（`ok … 73.809s` / `ok … 79.293s` / `ok … 70.881s`），`check-ch13-scope-green` 零回归满足；收口时 `go test -count=1 ./...` 全绿（13 个有测试的包，conformance `81.586s`；另 2 包无测试文件），`go vet ./...` / gofmt / `validate.py --all --strict` / `docs_sync.py`（33 对）/ `git diff --check` 干净。

**发现（implement 期）**：七个实现缺陷（②③⑤⑥⑦为真机探针揭出，其中②是**过宽发射**、⑥⑦是**该走而不走**；①④属计划与规范的偏离，已订正并披露）；三处单测盲区（⑧）；一处测试侧笔误（⑨）。全部披露，无静默绕过。

**停点**：T5 四枚 checkbox 功能面全绿、红证据与验证行可复核（四枚探针 check 0 → build 0 → run 值钉）；9 枚突变复验全捕获（含两处盲区补钉）；黄金账 738 枚、check 面零回归。T6（闭包全捕获 + fn 值）为下一任务。

### T6 闭包全捕获 + fn 值（2026-09-10）

**落地面**（tasks.md 两枚 checkbox）：① 红证据：捕获外层绑定（标量 / gc record / String 各一）的闭包今日 bndCallbackBody 停；② gc 载体 `{fnptr, env}` + value 字拷贝 + gc 指针 trace 入 env + fn 值调用 + prim 回调 `(fn, env)` ABI 兼容。

**红证据（双基线）**。六枚真机探针（`/tmp/t6i`·`t6g`·`t6h`·`t6fn`·`t6nest`·`t6ho`）对 HEAD 树（`/tmp/we-head`——由 `/tmp/head-welang` 临时副本内 `git show HEAD:…` 的源码构建，**不触碰工作树**）：

| 探针 | 形 | HEAD | B1a |
| --- | --- | --- | --- |
| t6i | 标量捕获 `let k = 3` / `n = m.update(\|v\| v + k)` | check 0 / **build 70** `bndCallbackBody` | run 0 —— `3` |
| t6g | gc record 捕获 `let c = Cell{n:5}` / `\|v\| v + c.n` | check 0 / **build 70** `bndCallbackBody` | run 0 —— `5` |
| t6h | **计算形** String 捕获 `let s = "a" + "b"` / `\|v\| v + s.byteLength()` | **clang 拒** `use of undefined value '%v2'`（披露①） | run 0 —— `2` |
| t6fn | fn 裸名作值 / fn 值作参 / 裸参闭包作参 | check 0 / **build 70** `bndMainBody` | run 0 —— `4 9 10` |
| t6nest | record 捕获 + 绑定 fn 值经载体调用 + 闭包体内调用另一闭包 | check 0 / **build 70** `bndFnBody` | run 0 —— `42 15 7` |
| t6ho | 高阶 fn 作值（`let g = apply`）+ 经载体传裸参闭包 | check 0 / **build 70** `bndMainBody` | run 0 —— `4 11` |

标量与 gc record 两形 HEAD 确实停在 `bndCallbackBody`（红行对这两形成立）；String 形**不停**——见披露①。

**实现**：

1. **闭包 = 内联 thunk define + 创建点环境块**（T6-1）——`emitClosure` 让闭包体走 `emitBodyCore`（与 fn 体**同一条路径**：尾表达式即隐式返回、`return` 是 thunk 的、`defer` 在 thunk 出口），define 形 `(ptr %env, 声明参数…)`；**捕获在创建点先物化**（`emitEnvBlock`：位图常量 `@.emapN` + `__we_alloc(16 + 8*words)` + `store` 头 + `__we_root_push` + 逐字 `gepStore`），thunk 入口 `materializeCaptures` 把 env 逐槽 load 回创建点名字的绑定环境（标量→`scalars`、prim→`prims`、String→`strEnv` 双词、gc→`gcEnv` 带记录键、fn→`fnEnv` 带签名）——闭包体内因此**零特殊读法**。捕获分类落 `captureOf` 五臂，布局序号 `collectClosureCaptures` 一处给定。
2. **fn 值 = 单 gc 载体**（T6-2）——`makeFnValue`：`__we_alloc(32)`，描述符 `@.fnmapN = [1 x i64] [i64 2]`（**只 trace 第二字**：第一字是代码指针，标了会把收集器引向正文），16 存 fnptr、24 存 env，载体入本 body 的根窗口。调用 `emitFnValueCall` 从载体取回两字 → `call <ret> %fnptr(ptr %env, args…)`。
3. **统一调用约定与适配 thunk**（T6-2）——所有 fn 值一律 `<fnptr>(env, args…)`；声明式 fn 的符号没有 env 参数 ⇒ `emitFnRef` 为值位的裸 fn 生成 `fnAdapter`（`(ptr %env, a0…an)` → `call @sym(a0…an)`）并配 null env。一条约定让调用点无需知道到手的是哪一类 fn 值（披露②）。
4. **fn 型参数 = 一个载体指针**（`abiFn` + `fnParamAbi.sig`）——签名随载体**静态传递**：绑定、参数、捕获、实参四处同一个来源。`emitFnArg` 按参数声明的签名发射实参（闭包字面量按之定形、名字按其自身签名核对）——**裸参闭包在 fn 型实参位因此有形可依**（章 12）。
5. **prim 回调 `(fn, env)` ABI 兼容**——`update`/`read`/`wait` 的闭包实参直发 `call i64 @__we_prim_update(ptr %cell, ptr %thunk, ptr %env)`；零捕获闭包 env = `null`——这正是此前黄金既有的形。
6. **`bndCallbackBody` 词删**——常量、`ctxCallback` 枚举值、`bndCallback()` 三者随旧 callback 机器（`emitCallback`）一并退役；`bnd()` 只余 ctxTask/ctxFn/default 三臂。`m9b_test.go` 两枚边界例重锚 `bndFnBody`（`TestM9bBoundaryWhats` 的「谓词返回 io 调用」与「谓词返回用户 fn 调用」两枚，期望词由 `bndCallbackBody` 改 `bndFnBody`）。

**缺陷与计划修正（D13 披露）**：

① **String 捕获在 HEAD 不是「停」而是「静默泄漏」，其中计算形发射无效 IR**（T6-1 红证据；本任务根治）——旧 `emitCallback` 只换 `e.scalars`，`e.strEnv`/`e.gcEnv`/`e.prims` 一概不动，于是回调体解析外层 String 名时走进**创建点**的 `strEnv`、把创建点的 SSA 操作数原样发进 thunk。静态字面量形（`let s = "abc"`）的操作数是模块常量 `@.s0` ⇒ 泄漏恰好产出**合法且正确**的 IR（HEAD 实测输出 `3`，与 t6h 同形不同源）；计算形（`let s = "a" + "b"`）的操作数是创建点的 SSA 值 ⇒ clang 拒 `use of undefined value '%v2'`。checkbox 红行原文「捕获外层绑定（标量/gc record/String 各一）的闭包今日 bndCallbackBody 停」对 String 面**不成立**——已在 tasks.md 订正为逐形分层。
② **值位的裸 fn 必须经适配 thunk**（T6-2，实机揭出）——首版 `emitFnRef` 直发 `{@main.square, null}`（design D5 原文），而调用点一律发 `(ptr env, args…)`：`square(n: Int64)` 于是把 null env 当成 `n`，`apply(f, 2)`/`apply(square, 3)` 实测打印 `0` 而非 `4`/`9`（同程序里 `apply(|x| x*2, 5)` 正确输出 `10`——闭包 thunk 本就有 env 首参，两形对照即定位）。修：`fnAdapter`。**D5 载体常量对订正为 `{适配 thunk, null}`**，design.md 已同步。
③ **String 捕获槽不置 trace 位**（T6-1，设计细化）——`runtime/c/str.c` 的 `str_alloc` 是 malloc 且永不回收、字面量指向模块只读常量池，而 `runtime/c/gc.c` 的标记阶段**对指针写 MARK 位**：置位就是把标记写进 rodata（段错误）或写进字符串自己的字节（数据损坏）。与任务捕获面（只 trace prim 句柄）同规。单测钉位图 `[i64 0]`（M2 守门）。
④ **`abiWordTypes` 的 abiStr/abiSum 字型原先写错**（T6-2，编译期揭出）——两族被合写成同一行；String 应 `{ptr, i64}`、sum 应 `{i64, i64}`（M10b 的 ABI 扩展披露面）。随本任务订正。
⑤ **无注解裸参闭包在值位仍是 check 面 E1001 边界**（T6-2，非缺陷）——`let g = |x| x * 3` 被检查器拒（章 12：裸参须有期望 fn 型或注解），加注解（`let g: fn(Int64) -> Int64 = …`）即通。t6nest 首版正因此未过，改注解形后 run 0——codegen 面按签名发射，对无签名可依的形正确拒绝。

**边界（披露，均安全停在 bnd 而非误发射）**：float 捕获停（块的 i64 存储与 double 域两面须一致，`captureWords` 的 `isF` 分支归 T11）；`walkBody` 遇未知节点形返回 false ⇒ 整个闭包停（宁可停，也不发射可能缺名的捕获集）；fn 型参数带 effect 段或泛型应用 ⇒ `bindTupleParams` 返回 false ⇒ 停；未声明返回类型的闭包按体尾域推签名，体尾非表达式或域不可名 ⇒ 停。

**突变复验（8 枚）**：对 `codegen.go` 逐枚单点突变 → 断言目标测试转红 → 还原转绿；每枚在**单测层**与**黄金层**（`TestGoldenCases/run-closure*`·`run-fn-value*`）各记一次。8/8 全捕获、8/8 还原转绿。逐枚：M1 捕获基偏移 `16 → 24`（单测·黄金）、M2 String 捕获置 trace 位（**单测红、黄金绿**——错位图只在收集发生时才可见，本批黄金程序不跨 GC 阈值）、M3 env 块尺寸丢头部（单测·黄金）、M4 载体两字互换（单测·黄金）、M5 声明式 fn 值跳过适配 thunk（单测·黄金）、M6 fn 型参数按两字过界（**单测红、黄金绿**，见下）、M7 env 置于 fn 值调用末位（单测·黄金）、M8 载体描述符 trace 代码指针（**单测红、黄金绿**，同 M2 的收集面）。**M6 补钉记录**：首轮该枚**两层均存活**（高阶 fn 作值当时无黄金覆盖），补 `run-fn-value-higher-order` + `TestHigherOrderFnInValuePositionTakesAnAdapter` 后单测转红；黄金层仍绿——实测原因是本机 clang 21.1.8 对「多传实参的直接调用」既不报错也不走样（干净重建后 build 0 且 `4 11` 照常输出），故该分支只有 IR 钉守得住。M2/M6/M8 三枚的黄金层空缺已在 tasks.md 的 T13「新增黄金总量」项下留痕。

**测试与黄金账**：`internal/codegen/` **164 → 175** 例全绿（+11，全在新增 `closure_test.go`：三类捕获各自的位图与槽位钉、零捕获 null env 钉、`(fn, env)` 回调 ABI 钉、载体描述符与两字钉、载体调用取回钉、适配 thunk 钉、fn 型参数单指针钉、裸参闭包取位类型钉、高阶 fn 作值适配钉）。conformance **738 → 744**（+6 新枚、**0 退役 0 重锚**）：**run-closure-scalar-capture**（`3`）、**run-closure-record-capture**（`5`）、**run-closure-string-capture**（`2`——计算形，即 HEAD 上发射无效 IR 的那枚）、**run-fn-value-call**（`4 9 10`）、**run-closure-nested**（`42 15 7`）、**run-fn-value-higher-order**（`4 11`）。六枚均真机先行定形、手钉期望、首跑即绿（未用 `WE_UPDATE_GOLDEN`）。T6 未翻任何既有边界黄金——该翻的面 T5 已翻、其余归 T13 的词表退役。全量黄金 `ok … 88.134s`（744 枚）；收口时 `go test -count=1 ./...` 全绿（13 个有测试的包；另 2 包无测试文件），gofmt / `go vet ./...` 干净。

**发现（implement 期）**：两处实现缺陷（② 实机揭出、④ 编译期揭出）；一处**基线缺陷**（①：HEAD 的回调机器静默泄漏 String 捕获、计算形产无效 IR——本任务根治）；两处设计细化（③⑤）；一处突变覆盖缺口（M6 首轮两层存活，补钉后单测捕获）。全部披露，无静默绕过。

**停点**：T6 两枚 checkbox 功能面全绿、红证据与验证行可复核（六枚探针 HEAD 逐形红 → B1a run 值钉）；8 枚突变复验（含一枚补钉）；`bndCallbackBody` 词表 grep 归零（活代码内零命中；所余三处均为记载性引用：`m9b_test.go:14` 的退役注、design D0 计划的退役表行、archive 历史）；黄金账 744 枚、零重锚零退役。T7（List 载体 + 组合子 + for-in）为下一任务。

### T7-0 闭包体尾部值位置（检查面缺陷，T7 侦察期发现，2026-09-10）

**缺陷**：短形闭包体尾部为 `if`/`match` 时，检查器以 E0605「non-unit value dropped」拒绝——而章 12 明定闭包体**是函数体**、其尾部表达式即返回值（章 2 块值）。缺陷面不止闭包：**任何**以 walkPlain 走查的普通块，其作为块值的最后一条表达式都被当作「被丢弃的值」。

**红证据（真机）**。四形探针（`/tmp/t7qa`…`qd`：短形 `if` 尾 / 块形 `if` 尾 / 带 fn 型注解 / 高阶实参位）全部 `check 1` + E0605 锚在闭包体尾；**同刻对照**：命名 fn 的 if 尾（`fn sign(n: Int64) -> Int64 { if n > 0 { 1 } else { 2 } }`）check 0、`|acc, x| acc + x` check 0 —— 一处对照即定位「闭包体块走的是块路径、不是 fn 体路径」。新增黄金 `check-ch12-closure-tail-value` 的程序在**修复前检查器**（`/tmp/we-old`，由临时回退条件重建的 CLI，不落工作树）上红：`demo/main.we:6:37: error[E0605]: non-unit value dropped — the statement's if arms are Int64, not ()`。

**根因**：`walkItems` 的丢弃判定 `dropped := mode != walkFn || !last || c.fnRet == nil`——`mode != walkFn` 使块内**每条**表达式语句按丢弃处理，尾部那条也在内。

**修复**：`dropped := !last || mode == walkControl || (mode == walkFn && c.fnRet == nil)`。仍丢三种位置：非尾语句、控制流臂体、**无声明返回**的 fn 体尾部（该形由下方 4883 行指名报「the final expression is …」）。不再丢：普通块的尾部（块值）、有声明返回的 fn 体尾部。

**边界核验（无新放行）**：`fn f() { { 1 + 2 } }`（裸块语句丢弃非 unit 值）与 `fn f() { 1 + 2 }`（unit fn 尾非 unit）**修复前后均 E0605**——裸块**语句位**的值丢弃由 T2-a 的块路径独立判，未被本次放宽覆盖。

**发射面停点（披露，非缺陷；本任务不扩张）**：修复只动检查面。闭包体的**块形体**（`|v| { v + 1 }`）与 `if`/`match` 尾（裸尾或块尾）在 codegen 仍停 `bndFnBody`（exit 70）；插值内调用 fn 值（`${c(1)}`）停 `bndMainBody`。今日可跑形为**裸表达式尾**（`|v| v + 1`——T6 六枚黄金即此形）。逐形探针在案，ch17 的 `reduce(|a, b| if …)` 例因此仍是**安全停点**而非误发射。此缺口的扩张（闭包体走与 fn 体相同的值位置）登记为后续任务候选，不在 T7 三项 checkbox 内。

**验证**：`internal/typecheck/m5_test.go` 四枚 wantOK（短形 if 尾 / 块形 if 尾 / match 尾 / 高阶实参位）——修复前红、修复后绿；突变复验一枚（条件回退为新式→四枚单测与黄金同时转红，还原转绿）；conformance **744 → 745**（+`check-ch12-closure-tail-value`，真机先行 check 0），全量黄金绿、零退役零重锚。

### T7-1 List 载体（runtime/c/list.c，2026-09-10）

**实现**：`{len@16, cap@24, traced@32}` 头 + 自槽 3 起的元素区，**一元素一字**；`__we_list_new(cap, traced)` / `__we_list_push(l, v) -> 新标识`（倍增增长、新块拷贝、旧块就地弃——本收集器不压缩）/ `__we_list_get(l, i) -> i64`（越界走章 14 族停任务，**非**章 17 `List.get` 的 Option 语义：后者越界是值、不是陷阱）/ `__we_list_len` / `__we_list_snap`（快照拷贝）。traced 载体的布局描述符由族自建（容量是运行期值，编译期静态描述符覆盖不了），掩码按块的 size 字定尺寸并在新建的零块上整容量置位——标记阶段跳过空子指针，且 push 不碰描述符。标量载体不发描述符。

**测试与黄金账**：`runtime/list_test.go` 两枚 harness（`TestListHarness` 追加与读取、倍增增长连元素拷贝与域、掩码逐槽对收集器自算槽数、自由链复用、快照独立性、元素槽可达性；`TestListPanicHarness` 越界读）。五枚突变复验（掩码位、增长拷贝、快照长度、get 守卫、增长域）逐枚转红后还原。黄金账零变动（载体是运行期面，T7-2 才接发射）。完整论证见提交 `56ed689` 正文。

### T7-2 List 字面量发射 + for-in List 源（2026-09-10）

**红证据（真机，HEAD = `552e2ea` 临时 worktree）**：`list_test.go` 十二枚对 HEAD 发射器跑——**七枚转红**（`expected clean emission, got boundary "main bodies beyond the M9b statement set …"`），四枚边界例（String/sum 元素、跨 fn 边界、非绑定位、用户 Iterable 源）在 HEAD 本就成立故不红。同刻真机：`/tmp/t7/g1` 在 HEAD 二进制上 `exit 70` + 同句边界词，在 B1a 二进制上 `exit 0`。黄金层红证据即 HEAD 已提交的 `test-fn-body-bnd`（`for x in [1, 2, 3]` 于 fn 体 → exit 70）——该枚正被本任务翻绿（见下）。

**实现**：

1. **字面量 = 载体开启 + 逐元素单字推入**（design D6）——`emitListLit`：`__we_list_new(i64 元素数, i64 traced)` 开局，容量**就是元素数**（本拓宽期无增长路径：字面量按精确元素数开块）；随后一条 `__we_list_push` 链，每个元素以 `emitListElemValue` 求值为**一个字**。创建点推一次根，链尾再推一次根——推链中途每次 push 都可能换块（增长），故最终标识必须自己入根窗口（**不是**靠「预开容量使标识不变」这条算术巧合兜底，注释在案）。
2. **元素面 = 一个字**（`listElem`{kind, rec, gc} / `listElemFace` / `elemFaceOfType` / `elemFaceOfExpr`）——标量域（字**就是**值）、Float64（**位型**，推入 `bitcast double→i64`、头部 `i64→double`）、gc 记录（字是句柄，载体 `traced=1`）。面来源两处：注解 `List<E>` 优先（**空字面量唯一的来源**，章 17 E1501），否则取首元素表达式之形。值类（byval）记录**照旧以句柄入载体**（一个堆表示），头部绑定处再按章 8 拷贝。
3. **for-in List 源 = 快照 + 计数循环**（`listFaceOf` 静态判定 → `emitForList`）——源为 List 绑定（`listEnv` 面）或字面量（就地物化）时：先 `__we_list_snap` 取快照、**快照入根**、读 `__we_list_len`，再开计数循环（`forhead/forbody/forcont/forexit` 复用具名 fn 循环的同一套），体首 `__we_list_get(ptr 快照, i64 计数器)`，头模式绑定那一个字。**源序**：字面量源的元素求值先于快照（`listSource` 注释在案），所以迭代序就是源表达式所指的序列。
4. **绑定面共享**（章 17）——`let ys = xs` 把 xs 的 `listBinding` 原样复制给 ys（**零发射**）：List 是 gc 类，绑定即共享同一载体，与别名的引用同形。
5. **停点面**（均安全停在 bnd，非误发射）：String/sum/元组/嵌套集合元素（单字装不下——String 两字、sum 两字、元组多字，**不写半截**）；List 出现在 fn ABI 面（参数/实参/返回，`abiWordTypes` 分类器拒 `List<T>`）；用户 Iterable 源（协议走接口机器，`listFaceOf` 为假 ⇒ 落到 `bnd()`）；非绑定非 for 源位置的字面量（拓宽面**只有这两处**，验收集即测试所钉）；gc 头模式在体内被赋值（gc 记录无槽可写，章 8 的 gc 槽面归 B 轨）。

**突变复验（6 枚，单测层 + 黄金层各记一次）**：M1 循环读**活载体**而非快照 → **单测红**（补钉后；见下）、**黄金绿**（快照语义在 B1a 的 check 面无受审面：`push`/`add` 不在检查集，任何程序都观测不到那次拷贝）；M2 载体 trace 位恒 0 → **单测红**、**黄金绿**（字面量元素的**构造根**在本 body 生命周期内一直钉着它们，而返回记录的函数调用不在元素验收集内——`[mk(11), mk(22)]` 实测 exit 70，故 B1a 无从观测缺位）；M3 浮点元素不转位型 → **单测红 · 黄金红**（clang 拒 `floating point constant invalid for type`）；M4 值类记录头部不拷贝 → **单测红**、**黄金绿**（章 8 禁止值类记录 `mut self`——`E0812` 实测——故拷贝无物可变，从 We 源不可观测）；M5 载体按容量 0 开 → **单测红**、**黄金绿**（push 自增长，预开容量是算术而非语义）；M6 字面量忽略注解 → **单测红 · 黄金红**（空字面量失去面，`run-list-domains` 的 `empty 0` 转红）。6/6 还原转绿。四枚黄金层空缺均为上列**结构性不可观测**，非覆盖疏漏——证据全部落在单测层与运行期层（载体自身的槽描述符钉在 T7-1 的 `TestListHarness`）。

**补钉记录（M1 首轮暴露）**：`TestListWalkSnapshotsBeforeTheHead` 原版只正则钉 `call i64 @__we_list_get(ptr %v\d+, …)`——**把快照读换成活载体读仍匹配**，M1 首轮因此单测层存活。补钉：从 `__we_list_snap` 行捕获快照寄存器名，断言 `get` 的第一实参**就是它**、且 `__we_root_push` 推的也是它。这才是 design D6 快照语义的可钉形。

**测试与黄金账**：`internal/codegen/` **175 → 187** 例全绿（+12，全在新增 `list_test.go`：字面量开启与推链、gc 元素 trace 位、空字面量取注解、快照先于头部且读快照、gc 头部按引用、值类头部拷贝、浮点位型往返、String/sum 元素停、跨 fn 边界停、非绑定位停、用户 Iterable 源停）。conformance **745 → 751**（**+7 新枚、1 退役、0 重锚**）：新增 **run-for-list**（和/重走/break/continue/字面量直源/`_` 头/嵌套七形，`sum-ok again-ok break-ok cont-ok direct-ok blank-ok nested-ok`）、**run-list-records**（gc 元素求和与**经方法共享**——`for c in cs { c.bump() }` 后重走见 23，章 17 按引用共享的行为钉）、**run-list-domains**（空字面量经注解、Bool、Float64 位型往返 `3.75`、Rune）、**test-fn-body-for-list**（**翻绿重锚**，见下）、**build-bnd-list-elem**、**build-bnd-list-abi**、**build-bnd-list-iterable**（三枚负例，exit 70 + 边界词）。七枚均真机先行定形、手钉期望、首跑即绿（未用 `WE_UPDATE_GOLDEN`）。

**翻绿重锚（D12 纪律）**：`test-fn-body-bnd`（HEAD 上 exit 70，钉「fn 体里的 for-in List 字面量尚未实现」）**退役删除**，按新事实面改锚为 `test-fn-body-for-list`（同名程序，exit 0 + `tick\ntick\ntick\npass  tests/m_test.we: looped (0ms)\ntotal 1, passed 1, failed 0 (0ms)\n`）。`bnd-` 名随旧事实退役，新名述新事实。

**m10b 单测重锚**：`TestM10bBndStops` 的 fn 体钉原以 List 字面量源（`for x in [1]`）为锚——那正是本任务的拓宽面，锚已失效。改用**用户 Iterable 源**（`impl Iterator<Int64> for CountIter` + `impl Iterable<Int64> for Range2` + `for c in r`）：真机复核仍在 `bndFnBody` 停（exit 70），钉改后绿。

**发现（implement 期，D13 披露）**：① 快照语义与 trace 位在 B1a 的 check 面**均无受审面**（M1/M2/M4/M5 四枚的黄金层空缺）——同类事实的既有措辞是「证据落在单测层与运行期层」，本次逐枚给出结构性理由而非泛泛带过。② 检查器不让**非空**字面量采纳注解的元素类型：`let i32s: List<Int32> = [1, 2, 3]` 报 `E0501 mixed types — the expression is List<Int64>, the annotation is List<Int32>`（`List<UInt64>` 同）。故注解面实际只服务空字面量；窄宽整型元素因此无 We 源入口，发射面的 `elemFaceOfType` 分支对非空字面量当前不可达（**非缺陷**——属检查器既有形，登记为观察项，不在 T7 三项 checkbox 内）。③ 元素位放**返回记录的函数调用**（`[mk(11)]`）停在 70：`emitRecordValue` 只收构造形与名字，调用结果不在元素验收集——与「一个字装不下」同属**安全停点**，登记为后续扩张候选。

**停点**：T7-2 checkbox 功能面全绿、红证据与验证行可复核（HEAD worktree 七红四不红 + 真机 70→0 + 已提交黄金翻绿）；6 枚突变复验（含一枚补钉）；黄金账 751 枚、1 退役 1 重锚；`bnd` 词表本任务零增删（`bndCallbackBody` 已于 T6 退役）。T7-3（6 急性组合子内建面）为下一任务。

### T7-3 六枚急性组合子内建面（2026-09-10）

**红证据（真机，HEAD = `1d389fc` 临时 worktree）**：`acute_test.go` 十八枚对 HEAD 发射器跑——**十二枚转红**（全为 `got boundary "main bodies beyond the M9b statement set …"`），六枚边界例（reduce 浮点元素 / find gc 元素 / 非 List 源 / 非 `iterator` 接收者 / 用户 Iterable 源 / 六枚之外）在 HEAD 本就成立故不红（HEAD 处处不停才是不正常）。黄金层同刻：四枚 `run-acute-*` 在 HEAD 上 `exit: want 0, got 70`、stdout 空（`run-acute-combinators` 实测 `want "fold-ok\n…find-miss-ok\n"、got ""`）；四枚 `build-bnd-acute-*` 在 HEAD 上**也通过**（它们钉的是「某形停在边界」，HEAD 对任何形都停，故**非判别性**——该四枚的可判别性由突变 M3/M10/M11 承担）。真机探针：加了 `push` 的 `fold` 程序在 HEAD 与 B1a 上同停 `标准库模块` 边界（见披露②）。

**实现**：

1. **识别形与三关**（`emitAcute`，挂在 `emitCall` 的 Member 分支、`fn, ok := call.Fn.(*ast.Member)` 之后）——名在六枚内（`acuteCombinator`）、接收者是**零实参的 `iterator` 调用**（`it.Fn` 为 Member 且 `im.Name == "iterator"`）、`listFaceOf(im.Recv)` 判其为 List 源。三关的设计意图是**面归属**而非防御：任一不合即「非本面」返回 `false`，调用继续落给下方各面（String 迭代器、用户 Iterable、五枚惰性名），停点与 HEAD 逐字相同；越过三关之后的失败才是本面的边界（返回 `true` + 停），故两种失败在调用点被区分开。
2. **一条走查，六枚共用**（`openListWalk` / `closeListWalk`）——快照 `__we_list_snap` → **快照入根 `__we_root_push` + `e.pushes++`**（走查活得比回调里那次分配久）→ 读 `__we_list_len` → 计数器槽 → 计数头 → 每轮 `__we_list_get(ptr 快照, i64 计数器)` 取本轮元素字。与 for 走查同一形、同一快照序，章 17 的定序规则因此**同因同码**而非另立一份。
3. **回调先于头开**——`emitFnArg(arg, new(acuteCallback(...)))` 在 `openListWalk` **之前**调用，故 thunk 定义与创建点环境块都落在循环之首：回调是 T6 的 fn 值，<fnptr>(env, …)，每次调用从载体取回两字。签名由 `acuteCallback` 按章 11 的声明读出：fold 的累加子面**由 init 之形定**（`emitNumExpr` 定 i64 或 double）、reduce 与谓词取**元素面**；谓词返回 Bool，走 i64 域，与声明式 fn 的 Bool 参数同形（`retName: "Bool"`）。`fnAbi` 手搭时必须**同时**设 `ret` 与 `retTyp`（前者是排版用的种类、后者是 `define` 的结果类型拼写）——漏设 `retTyp` 会emit 出 `define internal  @.cb0(…`（双空格、无类型），是 T7-3 首轮的真机红。
4. **单字出口**（`listElemWord` / `scalarWordFace` / `faceAbiKind` / `faceParam`）——载体一元素一字，回调却按元素自身的类型收参，故每轮把那个字还原：gc 句柄 `wordPtr` 回指针（**值类记录在头部按章 8 拷贝** `emitRecCopy`）、Float64 位型 `bitcast` 回 double、标量域字**就是**值。`scalarWordFace` 只认 `skI64/skU64/skBool/skRune`——即「元素字**是**其类型在 i64 域的值」；gc 句柄与浮点位型同为字而皆非其类型的值，reduce/find 的载荷因此**停在 bnd**：`Option<E>` 的载荷会被 match 臂绑进标量域，发出去就是把地址或位型重新解释成一个整数（安全停点，非缺陷）。
5. **六枚各自的形**——count：计数器槽自增、结果读槽，**不取元素面**（答案是载体长度，走查本来就要读）；fold：累加子在槽（跨回边，同循环携带绑定），`(acc, elem)` 调用后写回槽，结果按 init 的域给 `ckI64` + `isFloat`；any/all：`res` 槽起始 0/1、命中值 1/0（按名翻转），`icmp eq %p, hitWhen` 命中即写 `1-start` 并**直跳 `w.exit`**（章 11 的体在余项上递归，短路是同答案而少做功）；reduce：计数器自 0 起，`icmp eq cur, 0` 分 `rfirst`（写 tag=1、pay=元素字）/`rlater`（`(pay, elem)` 折回 pay）；find：`icmp ne %p, 0` 命中即写 Some(元素字)、直跳出口。reduce 与 find 的结果是 `ckSum{tag, pay, variants: ["None","Some"]}`——运行期 Option 的 ABI（None 0 / Some 1），与 `receive` 的表同一份。

**突变复验（11 枚，单测层 + 黄金层各记一次；每枚锚点唯一、逐枚还原转绿）**：M1 六枚里抹掉 `find` → **单测红·黄金红**；M2 all 的极性拉平为 any（删去起始/命中的按名翻转）→ **单测红·黄金红**；M3 reduce 删去载荷守卫（`scalarWordFace`）→ **单测红·黄金红**；M4 走查读**活载体**而非快照（`get` 第一实参换成源）→ **单测红**、**黄金绿**；M5 fold 忽略 init（槽开在 0）→ 首轮**单测绿**（补钉后红）·**黄金红**；M6 find 载荷写常量 0 → **单测红·黄金红**；M7 `faceParam` 抹掉 gc 记录键 → **单测红·黄金红**；M8 `listElemWord` 抹掉值类记录拷贝 → **单测红**、**黄金绿**；M9 抹掉 `iterator` 名判 → 首轮**单测绿·黄金绿**（补钉后单测红）·黄金绿；M10 `scalarWordFace` 恒真 → **单测红·黄金红**；M11 `scalarWordFace` 抹掉 Bool·Rune → **单测红·黄金红**。11/11 还原转绿。

**三枚黄金层空缺（**结构性不可观测**，非覆盖疏漏；逐枚已第一手取证）**：

- **M4（快照 vs 活载体）**：要观测这次拷贝，需要**回调在走查途中改动被走查的载体**——而章 17 的 List 方法（`push`/`len`/`get`）住在 std，本 build 对 std 模块调用一律停边界。真机：`xs.push(4)` 单独一枚（与 `fold` 体无关）`exit 70` + `we: standard-library modules (chapter 15) are not implemented in this reference build yet`；`fold(0, |acc, x| { xs.push(x); acc + x })` 同停。字面量源的载体更是本 body 的新鲜临时，无第二引用。故 B1a 无任何可构建程序能区分两者——与 T7-2 的 M1 同一事实，本次给出第一手探针而非沿用措辞。
- **M8（值类记录拷贝 vs 共享）**：章 8 禁 `mut self` 于值类记录（`E0812`，T7-2 已实测），值类记录没有可变的物，拷贝与共享**从 We 源不可区分**。
- **M9（`iterator` 名判）**：本面之外，`<List>.<非 iterator>()` 的接收者调用**只能是 std 的 List 方法**，本 build 在发射前就停在 std 边界。真机：`xs.len().count()`（唯一能命中该路径的形）`exit 70` + 同句 std 边界词。故没有可构建程序走到该守卫覆盖的路径；守卫以单测层补钉钉住（`TestAcuteOverANonIteratorReceiverStops`），理由是**发射器不得静默误发**——少了这一关，`xs.twice()` 里的调用会被丢掉、走查改读 `xs`，这是**错的答案**而不是缺的功能。

**补钉记录（M5·M9 首轮暴露，各一枚）**：① `TestAcuteFoldThreadsTheAccumulator` 原以 `fold(0, …)` 为锚——init 恰是域的单位元，删掉「用 init 开槽」改写成常量 0 后**钉纹丝不动**。补钉：init 改 `5`、槽的开局 store 正则改 `store i64 5`，并把这层意思写进注释（「init 刻意不取域的单位元」）。② 新增 `TestAcuteOverANonIteratorReceiverStops`：`xs.twice().count()` 必须是**非本面**（`ni != nil`），钉住第一关的名判。两枚补钉后 M5·M9 的单测层均转红。

**测试与黄金账**：`internal/codegen/` **187 → 205** 例全绿（+18，全在新增 `acute_test.go`：快照先于头开且读快照 / 累加子线程化 / count 数圈 / any 短路 / all 短路 / reduce 首元素即累加子 / find 首中答 Some / 浮点累加子 / 回调取元素面 / 回调建在循环外 / 谓词拷值类记录元素 / Bool·Rune 载荷作答 / reduce 浮点元素停 / find gc 元素停 / 非 List 源停 / 非 iterator 接收者停 / 用户 Iterable 源停 / 六枚之外停）。conformance **751 → 759**（**+8 新枚、0 退役、0 重锚**）：新增 **run-acute-combinators**（六枚正形 + any/all/find 的**未命中**三形，`fold-ok count-ok any-ok any-miss-ok all-ok all-miss-ok reduce-ok find-ok find-miss-ok`）、**run-acute-empty**（空载体的六枚：`count 0`、`any` 假、`all` 真、`fold` 答 init、`reduce`/`find` 答 None——空载体的**边界语义**）、**run-acute-callbacks**（浮点累加子 `8.0` / gc 记录元素折叠 `6` / gc 谓词 / **闭包捕获外层标量** `60` / **声明式 fn 作值**（走 `fnAdapter`）`6` / **块体闭包** / **for 体内嵌套组合子** `66`）、**run-acute-domains**（Bool 的 reduce 与 find、Rune 的 find 与 count——i64 域载荷四形），及四枚负例 **build-bnd-acute-lazy**（`filter`——五枚惰性名归 B1b）、**build-bnd-acute-float-payload**（reduce 的浮点载荷）、**build-bnd-acute-gc-payload**（find 的 gc 载荷）、**build-bnd-acute-user-iterable**（用户 `Iterable` 源），四枚皆 `exit 70` + 同句边界词。四枚 run 黄金均真机先行定形、手钉期望、首跑即绿（未用 `WE_UPDATE_GOLDEN`）。

**发现（implement 期，D13 披露）**：① **M10 暴露一条死判定**——reduce/find 的载荷守卫原写作 `face.gc || face.kind == skF64 || face.kind == skNone`，而 `skNone` 恰是 `strKind` 的**零值**、又恰是 `listElemFace` 给 gc 元素留下的 kind，故 `face.gc ||` 是一段**死析取**：M10（恒真）首轮两层存活、逐层加 `debug.PrintStack` 才定位。改为按 kind 集判定的 `scalarWordFace`（`skI64/skU64/skBool/skRune`），**行为不变**（gc 的 kind 是 `skNone`、浮点是 `skF64`，两者仍在集外）而每一部分都成为载荷——这正是 D13 突变复验要暴露的那类缺陷：不是发错了，而是**判据里有不参与判定的部分**。同时补 `TestAcuteBoolAndRunePayloadsAnswer` 与 `run-acute-domains` 黄金，把 `skBool`/`skRune` 两个成员钉住（M11）。② 组合子的**源序**与 for 的源序同因：字面量源的元素求值先于快照，故 `[1,2,3].iterator().fold(…)` 迭代的是源表达式所指的序列。③ 惰性五枚（map/filter/take/skip/collect）在 List 接收者上落 `bnd`（`build-bnd-acute-lazy` 钉住），归 B1b 的 vtable 面。

**停点**：T7-3 checkbox 功能面全绿、红证据与验证行可复核（HEAD worktree 十二红六绿 + 四枚 run 黄金 HEAD 70 / B1a 0 + 四枚 build 负例）；11 枚突变复验（含两枚补钉）；黄金账 759 枚、0 退役 0 重锚；`bnd` 词表本任务零增删。T7 三项 checkbox（T7-1/T7-2/T7-3）至此全绿，T8（顶层 let + 模块 init + 全局根）为下一任务。

### T8-1 模块 init 装载后序 + 标量顶层绑定全局直存（2026-09-10）

**红证据（真机，HEAD = `799fff9` 临时 worktree）**：`toplet_test.go` 十五枚对 HEAD 发射器跑——**十三枚转红**（十二枚为 `got boundary "top-level value bindings in code generation"`；`TestBoundaryWhats/scalar_top-level binding` 一行同因），三枚边界负例（String 停 / List 停 / 无绑定不发 init）在 HEAD 本就成立故不红。黄金层同刻：**七红一绿**——`build-bnd-toplet` 重锚后 `exit: want 0, got 70` + 边界词满行，六枚 `run-toplet-*` 全部 `want 0, got 70`、stdout 空（`run-toplet-cross-module` 实测 `want "x=2\ny=2\n"`、got `""`）；唯一的绿是 `build-bnd-toplet-carrier`（它钉的正是「String 顶层绑定停在边界」，HEAD 对任何顶层绑定都停，故**非判别性**——其可判别性由 T8-2 承担）。真机探针先行定形四组：跨模块链 `x=2 / y=2`（依赖模块的值被 main 经限定名与经局部绑定两条路读到）、装载序链 `142 10`（错序会得 `101 10`）、域谱 `2 20 23 2.5 true A 9000000000`、init 期陷阱 `error: Panicked: division by zero` + exit 1 且 `boot` **未打印**；补探针两组：菱形 `41 42`、名字遮蔽 `7`。

**实现**：

1. **一遍收集，域先于发射定死**（`collectTopLet`，挂在 pass one 的 `*ast.TopLet` 分支，该分支原为直接停边界）——顶层绑定的**全局 LLVM 类型必须对每个读它的 body 已知**，而读它的 body 可能先于它的模块发射，故分类只能发生在 pass one。分类是发射器既有的静态判据（注解优先、否则 `valueKind` 初值），与 `let` 语句同一份（`baseStrKind` + `valueKind`）。
2. **标量字的停面**（`if kind == skNone || kind == skStr { return &NotImplemented{What: bndTopLets} }`）——`skStr` 是 String 的 kind，`skNone` 覆盖记录/列表及任何静态不可判的形。二者都是**载体**：String 是 (ptr,len) 对、记录与列表是 gc 句柄，落全局即**收集器找不到的指针**。T8-1 不发它们不是省事，是不发一个**已知潜在 use-after-free** 的全局——这正是 D7 把「全局根登记」单列为 T8-2 的理由，也是本任务边界词**一字未改**的理由（D13：不静默降低要求）。
3. **全局直存**（`@<key>.<name> = internal global i64 0` / `double 0.0`；`emitTopLetInit` 尾 `store`）——符号经 `topScalar` 表由**限定名**拼出，与读面的键同一份，故 `@main.base` 既是存储也是读面。初值走模块 init（`@<key>.init`，源序），**不经槽**：`emitLetBinding` 照常求值并绑定，随后把该名字从 `e.scalars` **删除**并把值 store 进全局——删除是关键，留着 SSA 操作数会让同模块后续读走寄存器而非全局，则「读面 = 存储面」不成立（M12 钉住）。
4. **读面三处入钩**（`topName` 裸名 / `topMember` 限定名 / `topRead` 一次 load）——`emitNumExpr` 的 Ident、`emitMemberValue` 头、`emitLetBinding` 的 Ident 分支各接一处，`valueKind`/`memberKind` 各接一处（**域必须随读传递**，否则下游按 `skNone` 分类、插值空洞停边界——真机缺陷，见披露①）；`argIsScalar` 亦认顶层名。`topMember` 先过 `e.isLocalName` **再** `resolveQual`：局部遮蔽模块名时（章 6）限定名是局部的，不是模块的（M9 钉住）。
5. **装载后序的调序面**（`emitInitCalls` + `emitInitTopInits`）——`e.topLets` 按 pass one 走模块的次序追加，故它本身就是**装载后序 + 模块内源序**；调用按 key 去重后逐个落在 `__we_main` 的**体首**。每模块一枚 `@<key>.init`（`emitInitDefine`：照抄 `emitFnDefine` 的体协议快照 + 自己的 `e.ctx = ctxMain` 与 `exitMain` 出口位），**零共模绑定即零 define 零调用**。
6. **init 期 panic**（`exitSite{kind: exitMain}` + `e.trap`）——初值里的陷阱（除零、溢出）走 `__we_task_fail` + `unreachable`，进程中止且 `main` 体不执行（ch15 R5：main 之前没有捕获边界；真机 `boot` 未打印即此）。

**裁定（两处，D13 披露）**：

- **调序面落在 `__we_main` 体首，而非 D7 字面的「startup.c 调序面」**。理由：`runtime/c/startup.c` 是**固定的 C 文件**（ADR-0002 pinned sources），无法命名编译期才知道的模块集；`__we_main` 由 `__we_sched_boot` 作 task 0 体调用（startup.c:38），故落在体首的调用**正是**「进程启动时、`main` 体之前」。ch15 可观察形（每模块恰一次、装载后序、模块内源序、双导入一次、main 前）逐条被真机探针钉住（后三条见黄金）。**副产品**：init 既然在 task 0 的体内，D7 要求的「boot 期 init 前后根扫描屏障」的**后侧**已被 task 0 的 gc 窗口覆盖——T8-2 落地时按此复核，勿重复屏障。
- **`build-bnd-toplet` 重锚取「只改期望、不改名」**（proposal.md:412 的 T5 规则），**未**按 D12 对账表改名 `build-toplet-init`——T5 规则晚于 D12 成文且 T5 已按此落地 `build-ch10-method-call-boundary`，故 D12 该行的改名读法被 T5 取代。**但**该枚的源码形**同时**改了（String 顶层绑定 → 标量顶层绑定）：这是「只改期望」在此处的**不可行**——String 绑定是 T8-2 的面、HEAD 与本 build 都停，改期望改不出绿。被移走的 String 形落在**同源新枚** `build-bnd-toplet-carrier`（钉 T8-2 的面），先例同 T2-b/T4 对边界黄金源码形的重锚。

**突变复验（15 枚，单测层 + 黄金层各记一次；每枚锚点唯一、逐枚还原转绿）**：M1 调用序逆走（`e.topLets` 倒序）→ **单测红·黄金红**；M2 删去 `e.emitInitCalls()` → **单测红·黄金红**；M3 槽形回读分支摘除（`slot.alloca` 恒空）→ 两层存活（见下）；M4 `collectTopLet` 放行 `skStr` → **单测红·黄金红**；M5 `topRead` 丢掉 `typeName`（域不随读传递）→ **单测红·黄金红**；M6 去掉 key 去重 → 首轮**单测绿·黄金绿**（钉是空钉，补钉后单测红）·黄金绿；M7 `topName` 按裸名查表 → **单测红·黄金红**；M8 分类只看注解、去掉 `valueKind` 兜底 → **单测红·黄金红**；M9 `isLocalName` 守卫摘除 → 首轮**单测绿·黄金绿**（补钉后两层齐红）；M10 `topRead` 的 load 恒按 i64 → 首轮**单测绿**·**黄金红**（补钉后单测红）；M11 浮点错配守卫摘除（`slot.isFloat != ts.isFloat`）→ 两层存活；M12 绑定后不删 `e.scalars` 项 → **单测红**·**黄金绿**；M13 init define 落全局组而非 fn 组 → 两层存活；M14 `emitInitDefine` 不设 `exitMain` 出口位 → 两层存活；M15 全局初值改非零 → **单测红**·**黄金绿**。15/15 还原转绿。

**补钉记录（M6·M9·M10 首轮暴露，各一枚）**：① **M6 首轮的单测绿是空钉**——菱形钉当时让 u3 只有**一枚**绑定，而 `initEmitted` 是按**绑定**去重的，一模块一绑定恰是它无事可做的形：钉纹丝不动。补钉：u3 给**两枚**绑定（并补「根模块两绑定只发一次调用」的同层断言），断言从「菱形的 u3 发一次」变成「一模块 N 绑定发**一次**」——这才是 `initEmitted` 的载荷。② **M9 首轮两层齐绿是真守卫缺席**——补钉前无人钉 `e.isLocalName`，而真机取证该守卫**可达且可观测**：局部 `let u1 = Box { base: 7 }` 遮蔽同名模块时，`${u1.base}` 今日答 `7`（局部的字段）；守卫摘除则答模块全局的 `3`——**错的答案**，不是缺的功能。补钉 `TestTopLetShadowedQualifierIsNoModule`（发射层，断全局无 load）+ 黄金 `run-toplet-shadowed-qualifier`（真机先行定形，`7`）。③ **M10 的单测绿是覆盖面缺口**——原浮点钉只钉全局**声明**的 `double` 与**存储**的 `store double`，`topRead` 的 **load 类型**无人钉；补 `load double, ptr @main.f` 与 `wantNoIR(…"load i64, ptr @main.f")` 后单测转红。

**四枚两层存活（逐枚给出结构性理由，非覆盖疏漏）**：

- **M3（槽形回读 `if slot.alloca != ""`）**：`scalarSlot.alloca` 只由 `bindScalarSlot`（`e.assigned` 为真时）与 var 语句写——而 `emitInitDefine` 给 init 体开的是**空 `e.assigned`**，且顶层初值是单表达式、语法上装不下 var/循环。故该分支**今日不可达**。**不删**：它是**正确的通用回读**（`slot.alloca` 里就是当前值），删它是为了让一枚突变转红而改正确代码——T7-3 的死**析取**是冗余，此处不是。
- **M11（浮点错配守卫）**：`slot.isFloat != ts.isFloat` 要求「静态分类说的域」与「`emitLetBinding` 实际留下的域」一致；今日两者同源（同一份 `baseStrKind`/`literalStrKind` 判据），故恒不触发。同 M3：是安全网而非冗余判定，保留。
- **M13（define 落组）**：init define 落 fn 组还是全局组只影响**排版**——LLVM 模块内全局引用允许前向，两处皆合法 IR。分组是形状惯例，无断言需要它。
- **M14（`exitMain` 出口位）**：`e.exit` 只被 `emitReturn` 读，而顶层初值**语法上不含 return**，故该行的效果今日不可观测。**保留的理由与 M3/M11 不同**：不设它，init 体的出口位就**取决于哪个 body 最后发射**——现在无害纯属巧合，而下一个放宽（初值取块形/含 `?`）会立刻把「继承来的出口位」变成错答案。这是体协议**不得依赖发射次序**的前瞻要求，故留着并在此披露。

**测试与黄金账**：`internal/codegen/` **205 → 220** 例全绿（+15，全在新增 `toplet_test.go`：全局直存且无槽 / init 先于体 / 装载后序 / 双导入与多绑定各一次 / 遮蔽名非模块 / 模块内源序且读经全局 / 跨模块限定名读 / 同模块裸名读 / 域随读传递 / 浮点全局与浮点 load / 丢弃仍求值不绑定 / String 停 / List 停 / init 陷阱走 task-fail / 无绑定不发 init），另 `codegen_test.go` 的 `TestBoundaryWhats` 一行**重锚**（原「top-level binding」行的合成形是**无初值**的 `TopLet`——解析器产不出、词只覆盖载体后更不可达——改为真实载体形 `String`，并**新增**翻绿行 `scalar top-level binding`，同 M10b D8 对 other-fns 行的重锚先例）。conformance **759 → 766**（**+7 新枚、0 退役、1 重锚**）：新增 **build-bnd-toplet-carrier**（移来的 String 形，`exit 70` + 同句边界词）、**run-toplet-cross-module**（`x=2\ny=2`）、**run-toplet-load-order**（`142 10`——错序得 `101 10`，是一枚真正钉序的断言）、**run-toplet-domains**（`2 20 23 2.5 true A 9000000000`——Int64/UInt64/Float64/Bool/Rune 五域各一）、**run-toplet-init-panic**（stdout 空 + `error: Panicked: division by zero` + exit 1——`boot` 未打印即「main 之前无捕获边界」）、**run-toplet-diamond**（`41 42`——双导入 + 两绑定模块）、**run-toplet-shadowed-qualifier**（`7`）。七枚 run 黄金均**真机先行定形、手钉期望、首跑即绿**（未用 `WE_UPDATE_GOLDEN`）。重锚一枚：**build-bnd-toplet**（源码形 String→标量、`exit 70→0`、stderr 清空，**名未改**）。

**发现（implement 期，D13 披露）**：① **M5 的真机缺陷**：`topRead` 初版回填 `typeName: ts.typeName`（顶层的注解），而**无注解**的顶层绑定 `typeName` 为空 → `bindResult` 算 `skNone` → 下游插值空洞无法分类而停 `bndMainBody`。真机探针 `/tmp/t8/q1`、`p1` 首跑即停 70（`let x = u2.deeper` 后再 `${x}`）。修：`topRead` 回填 `strKindName(ts.kind)`——**域由全局的类型定，而非由一条绑定不必有的注解定**。补钉 `TestTopLetReadKeepsItsDomain`。② **`emitInitCalls` 里一条自认不可达的守卫**（`if ref.decl.Binding.Name == ""`）——注释即「unreachable」，是 M10b 那类「判据里有不参与判定的部分」的同形，实现期自查时删。③ **D7 的「全局根登记」在 T8-1 的边界上留了一个洞**：init 体里 `emitLetBinding` 的**中间** gc 值（如 `let s: String = mk()` 里的 `mk()` 结果）在 T8-2 的根屏障落地前不受保护——T8-1 不引入这个洞（`skStr` 已停），但 T8-2 落地时须确认屏障覆盖的是 **init 体全程**而不只是「init 前后各一次」。

**停点**：T8-1 checkbox 功能面全绿、红证据与验证行可复核（HEAD worktree 十三红三不红 + 七枚黄金 70→0 + 一枚重锚 + 一枚 T8-2 面负例）；15 枚突变复验（含三枚补钉）；黄金账 766 枚、0 退役 1 重锚；`bnd` 词表本任务零增删（`bndTopLets` 在位，服务 T8-2 的载体面）。T8-2（gc 顶层绑定全局根登记）为下一任务。

### T8-2A String 顶层绑定的一对全局（2026-09-10）

T8-2 拆两枚可垂直验证的提交：**A = String 对形**（本记录，纯发射器，零 runtime 改动），**B = gc 句柄全局 + `__we_gc_root_global` 根表**（改 runtime）。拆分理由见裁定一。

**红证据（真机，HEAD = `2a259b1` 临时 worktree）**：`toplet_carrier_test.go` 九枚对 HEAD 发射器跑——**八枚转红**（七枚为 `got boundary "top-level value bindings in code generation"`），`TestTopLetStringDiscardBindsNoPair` 在 HEAD 本就成立故不红（**照 T8-1 的 `build-bnd-toplet-carrier` 先例记明：非判别性**——丢弃面在 HEAD 与 B1a 都是「不发符号」，其可判别性由同层其余八枚承担）。黄金层同刻：**三红**——`run-toplet-string`（want `"hi\nhi!\n"`、got `""`）、`run-toplet-string-cross-module`（want `"ok\nok-u1\nok\n"`、got `""`）、`run-toplet-string-concat`（want `"ok!\nok?\n"`、got `""`），三枚皆 `exit: want 0, got 70` + 边界词满行；`build-bnd-toplet-carrier` 在本提交中**首次翻绿**（`exit 70 → 0`，源码形与名皆不动——它钉的一直是这个面）。真机探针先行定形四组：单模块同模块两段串（`hi` / `hi!`——后续初值读前一枚）、跨模块限定名读（`ok` / `ok-u1`——依赖模块内后枚读前枚）、限定名经局部绑定转写（`ok`）、限定名进拼接（`ok!` / `ok?`）。

**实现**：

1. **`topScalarSlot` → `topSlot`，载体面进同一张表**——`topSlot` 增 `str bool`：一个模块级绑定自有的存储**形**（一个标量全局，或一对全局），符号（`sym`）与读面键不变。改名而非加表：两类绑定在同一张表里，读面才只有一处（`topRead`），「读面 = 存储面」才是一句话而不是两处约定。
2. **一对全局**（`collectTopLet` 的 `skStr` 分支）——`@<sym>.p = internal global ptr null` 与 `@<sym>.len = internal global i64 0`。不取聚合体（`{ptr,i64}`）的理由：聚合体要求每个读面 `extractvalue` 拆包，而标量面的读是**一次 load**；一对全局让两个域各自是一次 load，读面形状与标量面同构，`topRead` 因此仍是一处。`internal` 链接足够——跨模块读发生在同一份 `.ll` 里（整程序一个 LLVM 模块）。
3. **存储面**（`emitTopLetInit` 的 `ts.str` 分支）——初值照常走 `emitLetBinding`（**所有** String 来源自动可用：字面量、插值、拼接、函数返回、限定名读），随后从 `e.strEnv` **取出并删除**该名字，把 `(dataOp, lenOp)` 存进两个全局；字面量的字节此时才 intern（`bind.dataOp == ""` 时 `e.intern(bind.data)`——M8 的「未用不发」纪律在顶层绑定的存储面上同样成立，因为存储**就是**一次使用）。删除与 T8-1 的 `delete(e.scalars, …)` 同因：留着 SSA 操作数会让同模块后续读走寄存器，读面与存储面分家。
4. **读面三处入钩**——`emitStringExpr` 的 Ident 分支（`e.strEnv` 未命中后查 `topName`）与 `emitNumExpr` 的 Ident 分支（**拒绝** String 槽：一对全局不是数字操作数）、`argIsScalar`（`ok && !ts.str`——println 的域探针据此把 String 绑定路由到字节对渲染器）。限定名一路**不需新钩**：`emitStringExpr` 的 Member 分支委派给 `emitFieldChainString` → `emitMemberValue`，而 T8-1 的 `topMember` 钩在那里；`emitStringExpr` 里我曾加的同一钩经突变 S8 证为**冗余**（摘除后两层皆绿），已删——T7-3 对「死析取」的处置同规。`valueKind`/`memberKind` 的 T8-1 钩**零改动**即覆盖 String：它们回填的是 `ts.kind`，而 `collectTopLet` 把 `skStr` 写进了 `kind`——域随读传递是 T8-1 就做对的事。

**裁定（两处，D13 披露）**：

- **String 半不需要根表，因此不需要 runtime 改动——D7 的「String 头入全局根表」在 D3 下是错的**。`runtime/c/str.c` 的头注释把 String 的字节定在 malloc 缓冲里（或发射器的私有常量），**两者都在 gc 域之外**：裸字节缓冲没有块头、没有描述符，而该注释自己写明「非 gc 指针推上 shadow root 栈，会被收集器读作块头」。故把 String 全局登记为根**不是漏做而是做错**——收集器会去解引用一块 `malloc` 缓冲的第一字。反过来说，D7 要处理的**只有 gc record/list 的句柄**（那是可收集块的指针），这正是 T8-2B 的面。本提交据此把 `build-bnd-toplet-carrier` 翻绿而不动 runtime。
- **拆分点落在「是否改 runtime」**——A 是纯发射器提交，B 要动 `runtime/c/gc.c`/`gc.h`（新 API + 根表）。两者可各自独立回滚、独立验证，符合「一个变更一个可垂直验证的提交」；且 A 的红证据与 B 的红证据互不依赖（A 的真机探针全绿时 B 的面仍停在边界）。

**突变复验（14 枚，单测层 + 黄金层各记一次；每枚锚点唯一、逐枚还原转绿）**：S1 长度存进 `.p` → **单测红·黄金红**；S2 数据字 store 删去 → **单测红·黄金红**；S3 对的角色对调（`dataOp`/`lenOp` 互换）→ 首轮**单测绿·黄金红**（补钉后单测红）；S4 `argIsScalar` 不顾 `ts.str` → **单测红·黄金红**；S5 `emitNumExpr` 收下 String 槽 → 两层存活（见下）；S6 `.len` 全局删去 → **单测红·黄金红**；S7 `emitStringExpr` 的 Ident 钩摘除 → **单测红·黄金红**；S8 `emitStringExpr` 的 Member 钩摘除 → 两层存活（**冗余，已删**）；S9 init 后不删 `e.strEnv` → 两层存活（等价，见下）；S10 两个 store 目标对调 → **单测红·黄金红**；S11 init 的 String 面守卫摘除 → 两层存活（见下）；S12 `.p` 全局声明为 `i64` → **单测红·黄金绿**（等价，见下）；S13 长度从 `.p` 载入 → **单测红·黄金红**；S14 `emitMemberValue` 的 `topMember` 钩摘除 → **单测红·黄金红**（T8-1 的面，本提交复验其同时覆盖 String 读）。14/14 还原转绿。

**补钉记录（S3 首轮暴露，一枚）**：原 `TestTopLetStringReadLoadsBothWords` 只钉两条 load 指令的**存在**，不钉**角色**——S3 把 `dataOp`/`lenOp` 对调后两条 load 一字未变，单测照绿。补钉：连寄存器一起钉（`%v0 = load ptr, ptr @main.greeting.p` / `%v1 = load i64, ptr @main.greeting.len` 的连续块 + `call void %v2(ptr %v0, i64 %v1)`），即「数据字进指针位、长度进 i64 位」。同类先例 `m8_test.go:82` 的连续指令块钉法。

**四枚两层存活（逐枚给出结构性理由，非覆盖疏漏）**：

- **S5（`emitNumExpr` 的 `ts.str` 守卫）**：真机探针三组穷举可达路径——`let n: Int64 = greeting` 被前端 `E0501`（mixed types，no coercion is ever inserted）拒；`${greeting + 1}` 在更早的面停边界（`bndMainBody`，把守卫体临时改成 `panic` 后真机跑该程序**未触发**，证明它根本没走到这里）；`greeting > 1` 同 `E0501`。**全 769 枚黄金 + 全部单测在 S5 下全绿**（已跑满整套 conformance 确认）。故是**安全网**：前端把「模块级绑定的跨域读」全数拒掉，而前端留给发射器的域判定（println 的实参、插值空洞）都先过 `valueKind`/`argIsScalar`，二者都认 `ts.str`。保留理由同 T8-1 的 M3/M11：**发射器的静态分类与前端不得假定是同一份函数**。
- **S11（init 的 String 面守卫 `if !ok { return e.bnd() }`）**：初值未留下 String 面要求「分类说 String、`emitLetBinding` 留下的却是别的域」——同 S5 被 `E0501` 排除。是安全网，保留；删了它，域不一致时就变成「把零值 `strBinding` 的 `e.intern("")` 空串存进去」，那是**静默错答案**而不是边界。
- **S9（init 后不删 `e.strEnv`）**：留在 `strEnv` 的是刚被存进全局的那个 `(dataOp, lenOp)`；SSA 寄存器不可变，故同模块后续初值走寄存器与走全局**恒等值**。这是**等价突变**（T8-1 的 M12 同类），非覆盖缺口。删除仍保留：读面与存储面必须是同一个面，这一点靠结构保证而非靠「恰巧同值」。
- **S12（`.p` 全局声明为 `i64`）**：不透明指针下 `store ptr %v, ptr @g` 对 `@g: i64` 是**合法 IR**——verifier 只比 `ptr` 与 `ptr`，不去看全局的声明类型（已用 `clang -c` 直接喂错型 `.ll` 确认：exit 0）。`ptr` 与 `i64` 同为八字，机器码因此正确。故是**等价突变**；单测红是因为形状钉（`@main.greeting.p = internal global ptr null`）钉的正是声明形。**披露**：这一枚同时说明「全局声明类型」在 golden 层不可判别，其正确性由单测的形状钉承担。

**测试与黄金账**：`internal/codegen/` **220 → 228** 例全绿（+8 全在新增 `toplet_carrier_test.go`：一对全局 / 两个 store / 两条 load 及其角色 / 跨模块限定名读 / 拼接里的限定名读 / 读不进标量域 / 无根登记 / 函数返回的 String；**退役 1** 枚 `TestTopLetStringStops`——它钉的正是本提交翻绿的面，且其注释援引的「D7 的全局根表」在 D3 下不成立，见裁定一），另 `codegen_test.go` 的 `TestBoundaryWhats` 两行**重锚**：原「carrier top-level binding」（String 源）行改名为「String top-level binding」并翻绿（`""`），**新增**「gc carrier top-level binding」（List 源）行承接仍停的载体面——同 T8-1 对 `build-bnd-toplet` 的处置，源码形随面走、行名随形走。conformance **766 → 769**（**+3 新枚、0 退役、1 重锚**）：**run-toplet-string**（`hi\nhi!\n`）、**run-toplet-string-cross-module**（`ok\nok-u1\nok\n`）、**run-toplet-string-concat**（`ok!\nok?\n`），三枚均**真机先行定形、手钉期望、首跑即绿**（未用 `WE_UPDATE_GOLDEN`）；重锚一枚 **build-bnd-toplet-carrier**（`exit 70 → 0`、stderr 清空，源码形与**名皆不动**）。

**发现（implement 期，D13 披露）**：① **D7 对 String 的根登记要求与 D3 冲突**（见裁定一）——本提交按 D3 落地，因此 T8-2B 的根表**只收 gc 句柄**，`run-toplet-string*` 三枚里**没有**任何 `__we_gc_root_global`，`TestTopLetStringIsNoGcRoot` 正向钉住这一点。② **`build-bnd-toplet-carrier` 从 T8-1 起就是一枚「转移中的负例」**——T8-1 移交时它钉「String 停在边界」，本提交把它翻绿；两枚提交里它都不是判别性的（HEAD 对任何顶层绑定都停），其价值在于**同一源码形跨两枚提交的期望追踪**。③ **`emitStringExpr` 的 Member 钩是 T8-1 的 `topMember` 钩的下游重复**——`emitFieldChainString` 委派 `emitMemberValue`，后者已解限定名；S8 存活即其证据，已删（见实现 4）。

**停点**：T8-2A 功能面全绿、红证据与验证行可复核（HEAD worktree 八红一不红 + 三枚新黄金 70→0 + 一枚翻绿）；14 枚突变复验（含一枚补钉、四枚分类存活）；单测 228、黄金 769（本提交 +8/-1 与 +3/1 重锚）；`bnd` 词表本任务零增删。T8-2B（gc 句柄全局 + `__we_gc_root_global` + init 期根屏障复核）为下一提交。

### T8-2B-0 根记账按 body 放电（块出口 + break/continue 边）（2026-09-11）

**编号**：本面在探 T8-2B 的负控制时暴露，是 2B 的**前置步**（没有它，`__we_gc_root_global` 的负控制不可观测——见发现②），故排在 2B 之 **0**。但它的主题**比 2B 宽**：修的是**发射器的根记账不变量**本身（任何在循环里分配的 We 程序都踩得到，与有无顶层绑定无关），不是 gc 句柄全局那一面。这也是它单独成提交的理由——可独立回滚、独立验证。

**红证据（真机，HEAD = `db14b9d` 临时 worktree）**：`rootdischarge_test.go` 十二枚对 HEAD 发射器跑——**十一红一绿**。红的十一枚报的都是 `the line carrying … owes N root pops, the emission carries 0`（HEAD 只在函数出口放电，任何 body 出口都是 0）。绿的一枚 `TestBodiesWithoutRootsEmitNoDischarge` 在 HEAD 本就成立故不红（**零漂移守卫，按构造非判别性**——它守的是「本修复不碰不推根的 body」，其可判别性由同层其余十一枚承担；同类先例 T8-2A 的 `TestTopLetStringDiscardBindsNoPair`）。黄金层同刻：**一红**——`run-gc-root-discharge`，`exit` 两侧皆 0、`stderr` 两侧皆空，**harness 只报 stdout 一行**（want `"49 800080000 2016\n"`、got `"1 800080000 2016\n"`）。**（测量纪律）**：该工作树一度被插桩过，插桩期的 harness 输出把诊断行也报成 stderr 差异；**插桩已 `git checkout` 还原并复跑**，上句是本工作树 runtime 干净时的实测。

**真机探针（先行定形，本提交的皇冠证据）**：100k 趟纯 churn 探针（每趟 `let xs = [i, i+1, i+2, i+3]` + `for x in xs`），带诊断的 runtime 上：

| | HEAD | 本提交 |
|---|---|---|
| 收集时的 `roots` | 218460 → 240306 → 262152 → **283998**（单调爬） | **0**（每次都是 0） |
| 每次 `swept` | **0**（三次都是） | **14564** |
| `freelist` | 0（从未回收过一块） | — |
| `used` | 10.5 MiB → 13.6 MiB（还在爬） | 稳定在一个 chunk 内 |

即 HEAD 上循环体的推根**一趟一趟堆在任务的根窗口里**，收集器一字节都收不回；修复后窗口每趟归零，收集器每次收回 14564 块。**程序输出两侧相同**——这正是关键：HEAD 的缺陷是**纯泄漏、不改变可观察值**，所以判别性黄金必须落在**截断面**上（见下）。

**判别性黄金的定形（`run-gc-root-discharge`，纯黑盒：不用内存统计、不用 runtime 内部钩子）**：它把**过弹截断**摆在一个可读的位置。`a`、`b` 两枚 Box 先绑定（窗口恰好是 `[a, b]`），紧接着 `maybe(false)`——它的 `if c` 臂**静态算两个推根、动态路径一个不走**，而共享的 `ifjoin` 出口照静态数弹两个，于是把 `a`、`b` 从窗口里弹掉。此后 `boxes(64)` 分配并复用那两块，`a.value + b.value` 读到的是新 Box 的值：**HEAD 打 `1`，修复后打 `49`**。**探针顺序是关键**——早先把 `maybe(false)` 放在 churn **之后**，泄漏出来的几万枚根压在窗口**顶上**，过弹弹掉的是顶上的而不是受害者，探针照打 49（这个失败尝试本身是发现①的证据）。`n`（800080000）与 `m`（2016）两侧相同：黄金只差一个数，判别面窄而准。

**实现（四处）**：

1. **`emitBlockStmts` 按块记账**——入口 `base := e.pushes`，出块时若未发散则 `dischargeRoots(base)`，随后**无条件** `e.pushes = base`。无条件回写是第二半：SSA 文本里 join 之后的语句仍会被发射（只是不可达），计数留着就会让后续出口**多弹**。
2. **`dischargeRoots(base)`**——弹 `e.pushes - base` 次，后推先弹（运行期 shadow stack 要求的次序）。`e.pushes == base` 时**一条不发**：整个既有语料的字节不变。
3. **`emitArmBlock`（值形臂）**同规——臂有 sink 而不是落空，但推根欠在同一处：尾值算完、去 join 的边之前。
4. **`break`/`continue` 各自带放电**——`emitBreak`/`emitContinue` 在 `unwind(fr.depth)` 与 `br` 之间插 `dischargeRoots(fr.roots)`。`loopFrame` 增 `roots int`，由新助手 `emitLoopBody` 在压帧时记 `e.pushes`（五处循环发射点全部改走这个助手，帧的构造只剩一处）。**弹到循环基深而不是函数 0**：break/continue 的对面仍在函数里，基深以下（外层 body 自己的根、for 的快照根）在那边**还活着**——弹穿了就是给收集器送活对象。

**裁定与披露（D13）**：

- **`return` 面零改动**——这是「按 body 记账」的推论而非疏漏。函数出口欠的是「路径上的全部活根」，而 `e.pushes` 现在**就是**这个数：穿过一个 body 但没走到它出口（return 跳过去了）的推根仍在计数里，而那正是**还活着**的那些。故四处函数出口（`emitReturn` 的 exitTest/exitFn 两 case 走 `popRoots()`；`we_main` 的 `Ok(())` 分派尾与 `emitFnBody` 的落空尾各内联一段）**一行未改**；本提交**新增一枚钉** `TestReturnDischargesTheWholeLiveSet` 把这个事实钉住（臂内 return 欠 **4** 而不是 2——外层 body 的两枚一并带走）。
- **`@<key>.init` 体的放电随之移交 T8-2B**——本提交面里**没有**这一项。理由：此刻里程碑上没有任何顶层绑定是**可收集句柄**（标量是一个字、String 的字节在 gc 域外，见 T8-2A 裁定一），故 init 体**推不出根**，钉它会是空钉（`rootdischarge_test.go` 头注释记明）。2B 的 gc 顶层绑定才是把根放进 init 体的那一面——那时钉才有内容。
- **~~`return` 在循环体内这一面在 B1a 不可达~~（T9-3 结清：这条记录写下时就是错的）**——原文称 M9b 语句集只收「每 body 一个尾 return」、循环体里出现 return 是边界。**事实相反**：`for`/`while` 体从 T2 起就吃 return（`TestReturnInsideLoop` 早已钉住，main 里与 fn 里都是），且同样的可达性在披露被写下的那个提交上也成立（T9-3 在 `f303bb4` 与 `69dbf78` 两处真机复核）。故本提交按 D13 记的这处「覆盖面缺口」是**一处误记**而非一处缺口：`emitReturn` 在任意深度放电整个活集（循环帧是 break/continue 停的地方，而 return 离开函数），**发射器一处未改**。`TestReturnDischargesTheWholeLiveSet` 的 body 仍取 if 臂（无害），**T9-3 补上循环体臂并把它关掉**——补上的是缺的那枚测试，不是缺的那段代码。
- **回补 design.md:82 的**意图**，非协议变更**——D7 原文「根推送按 body 记账（`e.pushes` 在每个 body 出口弹）」本来就是**按 body**；HEAD 的实现只做了函数体一个 body，其余 body 的出口既没弹也没归零。本提交把它落成设计说的样子。

**突变复验（12 枚，单测层 + 黄金层各记一次；每枚锚点唯一、逐枚还原转绿）**：`no-block-discharge` → **单测红**（6 枚钉）·黄金绿；`no-counter-reset` → **单测红**（2 枚·含 `BareBlock`）·黄金绿；`discharge-when-diverged` → 单测绿·**黄金红**（见下）；`arm-no-discharge` → **单测红**·黄金绿；`arm-no-counter-reset` → **单测红**·黄金绿；`loop-frame-plus-one` → **单测红**（3 枚）·黄金绿；`break-no-discharge` → **单测红**（2 枚）·黄金绿；`continue-no-discharge` → **单测红**·黄金绿；`break-to-zero` → **单测红**·黄金绿；`return-one-short`（`popRoots` 少弹一枚）→ **单测红**（`ReturnDischargesTheWholeLiveSet` + 既有 `ReturnPopsRoots`）·黄金绿；`one-pop-short`（`dischargeRoots` 少弹一枚）→ **单测红**（10 枚钉）·黄金绿。**12/12 还原转绿，零存活**。

**补钉记录（三枚，全部由电池首轮暴露）**：

- **`popsBefore` 的「恰好」语义**：首轮 `no-counter-reset` 与 `break-discharge-to-zero` **存活**，原因是原正则 `(?:call void @__we_root_pop\(\)\n  ){n}br` 在**更长**的 pop 串里也能匹配到 n 条（`{n}` 是「至少 n 条相邻」）。改为从目标行行首**反向逐条**数相邻 pop、比**恰好** n（`popsBeforeBr` → 统一为 needle 寻址的 `popsBefore`，顺带把两套 helper 收成一套）。两枚随转红。
- **`noPopsBefore`（新增）**：`arm-no-counter-reset` **存活**说明**主 body 尾部的 pop 数无人钉**——计数不回写不会少弹，只会让后续出口多弹。补两处（`TestValueArmDischargesItsRoots`、`TestNestedBodyDischargesOnlyItsOwnRoots`）钉 `ret i32 0` 前**没有**多余放电。
- **`TestReturnDischargesTheWholeLiveSet`（新增）**：见裁定一。
- **两枚首轮空过的钉**：`TestBreak/ContinueDischargesThePassesRoots` 的字面量原本在 `if` **之外**，臂的推根数是 0，正则 `{0}` 匹配了空串——把字面量移进臂内，并给 `popsBefore` 加 `n < 1` 直接判「这是空钉」。`TestBareBlockDischargesItsRoots` 首轮以整模块 push/pop 计数为钉（太松），改钉到**块尾**：下一个语句开自己的载体之前**恰好**两条。

**两层存活（一枚，给出结构性理由）**：`discharge-when-diverged`（摘掉 `if !e.diverged` 守卫，在终结指令之后仍发 pop）**单测绿·黄金红**。理由：**单测层只发射 IR、不喂 clang**，而该突变产出的 IR 非法（终结指令之后还有指令）；黄金层要 build+run，`we build` 走 clang，故被抓住。这是**层间分工**而不是覆盖缺口——但它同时说明「单测层 IR 的合法性由黄金层兜底」这一分工在本面上是**唯一的**兜底，记此备查。

**测试与黄金账**：`internal/codegen/` **228 → 240** 例全绿（**+12**，全在新增 `rootdischarge_test.go`）。conformance **769 → 770**（**+1 新枚、0 退役、0 重锚**）：**run-gc-root-discharge** 真机先行定形、手钉期望、首跑即绿（未用 `WE_UPDATE_GOLDEN`）；未跑 `-count=1` 时该包会命中缓存，**本行数字取自 `-count=1` 的 116.2s 全绿一轮**。`bnd` 词表本提交**零增删**。

**发现（implement 期，D13 披露）**：① **泄漏面不改变可观察值，故只能在截断面判别**——首个探针顺序（churn 在前）打 49，是因为几万枚泄漏根压在窗口顶上替受害者挡了弹。**这条对后续任何「根窗口过弹/欠弹」类缺陷都成立**：探针必须让受害者处在窗口**顶**（或其上无更晚的垃圾）。② **本面是 2B 负控制的前提**：HEAD 上 `__we_gc_root_global` 的效果被循环体的无界泄漏淹没（窗口永远在长，单看「某对象活没活」分辨不出根表生没生效）；修完之后 2B 的负控制才咬得住——这正是本子项先于 2B 落地的理由。③ **`e.pushes` 的语义在本提交改变**：从「静态见过的推送数」变成「**路径上的活根数**」。四处函数出口的正确性因此从「在这些路径上碰巧对」变成「由不变量保证」，而 body 出口与 break/continue 边则是**新**欠的债——这是本提交真正的语义变化。④ **for 的快照根位置被确认**：快照的推根发生在 head **之前**（body 之外），跨每条回边存活，由外层 body 的出口一并弹——`TestForListBodyDischargesEachPassesRoots` 钉的就是「body 只弹自己的」，不是「弹到 0」。

**停点**：T8-2B-0 功能面全绿、红证据与验证行可复核（HEAD worktree 十一红一绿 + 黄金一红仅 stdout + 真机 283998/swept=0 → 0/14564）；12 枚突变复验（含 3 枚补钉、1 枚层间分工存活）；单测 240、黄金 770（本提交 +12 与 +1）；`bnd` 词表零增删。**T8-2B（gc 句柄全局 + `__we_gc_root_global` + init 期根屏障复核 + init 体放电及其钉）** 为下一提交，其负控制现在咬得住了。

### T8-2B gc 顶层绑定的全局根表（`__we_gc_root_global` + init 体放电）（2026-09-11）

**主题**：T8-2 的后半——顶层绑定其值是一枚**可收集句柄**（一个 gc 记录、一个 List）。至此顶层绑定有三种存储形，本提交补上第三种：**标量一个字**（T8-1 的全局直存）、**String 的一对 `(ptr,len)`**（T8-2A）、**一个持有句柄的全局**（本提交）。T8-2B-0 移交来的 init 体放电随本提交落地——不是搭车：**没有它，本提交的根表在程序上不可观测**（见发现②）。

**红证据（真机，HEAD = `f303bb4` 工作树）**：单测层九枚新钉对 HEAD 发射器跑——**七红二绿**。七红报的**全部**是 `expected clean emission, got boundary "top-level value bindings in code generation"`：HEAD 上这一面根本是**边界停**，钉的形状断言一行都没跑到。二绿是**零漂移守卫**，按构造非判别性——`TestTopLetStringIsNotRegistered` 守 D3 的「String 不登记」、`TestTopLetGcValueRecordStops` 守「值记录仍停」，两条在 HEAD 上本就成立，它们守的是**本修复不越界**，判别性由同层其余七枚承担（同类先例 T8-2A 的 `TestTopLetStringDiscardBindsNoPair`）。黄金层同刻：**三红**，三枚全是 `exit: want 0, got 70`、`stdout` 空、`stderr` 带上面那个边界词。

**（红证据的诚实定性）** 本面的 HEAD 红是「**这一面编不出来**」，不是 T8-2B-0 那种「**数字错**」。能判别「根表在不在被读」的是**负控制**——发射完整、只有根表被动过的那些构建。故本提交的判别性证据主体在突变电池（运行期三枚），而不是 HEAD 红证据。这是本子项特有的形态，记此以免后人误读上段为「黄金有判别力」。

**真机探针（先行定形）**：三枚探针 + 一枚专门判别「泄漏是否掩盖根表」的探针。ON = 本提交构建，OFF = 负控制构建（发射完整、只摘登记）。

| 探针 | 形状 | ON | OFF |
|---|---|---|---|
| p4 记录 | `record Big{a,b,c,d}`（48B 块）+ 8 万趟空表 churn + `let probe = Big{a:9,…}` | `1 9 80000` | `9 9 80000` |
| p6 列表 | 顶层 `List<Int64> = [10,20,30,40]` + 同 churn + `let probe: List<Int64> = [7,7,7,7]` | `100 28 80000` | `0 28 80000` |
| p5 跨模块 | u1 模块的 `shared: Leaf` 与 `ns: List<Int64>`，root 读两者 | `77 60 80000` | `77 0 80000` |

p4 的机制是**精确**的：受害者那块 48B 被扫掉后进空闲表，`probe` 的 `Big` 同样要 48B——churn 的 16B 空表块**太小、满足不了这次请求**，故只有受害者那块能应它，于是 `probe` 与 `g` **同块**：OFF 下 `g.a` 读到 `probe` 写进去的 9，两个数**必然相等**；ON 下两块分开，`1` 与 `9` 各是各的。**尺寸类的选择是负控制的成败所在**（见发现①）。

p5 的 `77` **不是**「记录活着」的证据：那块被扫掉后进空闲表，而 churn 的 16B 块与它的尺寸类不同、**没人覆写它的字节**，读到的 `77` 是**陈旧字节**——deterministic，但不是 liveness。该探针的判别数是列表那一项（`60` vs `0`）。

**泄漏掩盖根表的判别探针（本提交必须同时做放电与登记的理由）**：发射完整但**登记与 init 体放电都摘掉**的构建打 `1 9 80000`——与修复后**逐字相同**；只摘登记的打 `9 9 80000`。即：init 体泄漏的那一枚根**独自**把句柄钉活着，根表在不在被读**看不出来**。这是 T8-2B-0 把 init 体放电移交至此的理由的**实测兑现**——若只做登记不做放电，本提交的全部黄金都会绿，而根表可能一行都没生效。

**判别性黄金的定形（三枚，纯黑盒）**：`run-toplet-gc-record-root`（`1 9 80000`）、`run-toplet-gc-list-root`（`100 28 80000`）、`run-toplet-gc-cross-module`（`77 60 80000`）。三枚**真机先行定形、手钉期望、首跑即绿**（未用 `WE_UPDATE_GOLDEN`）。

**实现（五处）**：

1. **`topSlot` 增三字段** `gc bool / rec string / elem listElem`。`topGcShape` 是**无发射**的 pass-one 分类（记录键、或列表的元素面），槽位带着它在模块间走——emit-free，与其余 pass-one 分类同规，理由是「另一个模块里、在拥有者的 init 之前发射的 body」必须读到 pass one 定下的面。
2. **第三种存储形**：`@<key>.<name> = internal global ptr null`，读面是**一条 load**。不是 String 那种一对——句柄是一个字。
3. **`emitTopRootRegistrations`**：在**入口最前**、且**在所有 init 调用之前**发 `call void @__we_gc_root_global(ptr @<sym>)`，每枚 gc 槽一条。次序是硬要求：收集器必须在任何初始化器能把句柄放进槽**之前**认识这个槽。
4. **运行期根表**（`runtime/c/gc.c`）：`groot_slots/n/cap` 三个静态，`__we_gc_root_global(slot)` 登记**槽地址**，`__we_gc_collect` 的 mark 阶段逐个**解引用读当前值**。
5. **init 体出口放电**（T8-2B-0 移交至此）。

**裁定与披露（D13）**：

- **裁定一：D7 的「初始化器期的根屏障」是被**回答**而不是被**实现**的**。D7 原文要求 init 期有一个 root barrier。本实现让这个需求**消失**：屏障的语义对象是**值**（把某个值排进某次扫描——先 store 后 collect，或先 collect 后 store），而根表里存的是**槽地址**、扫描时**自己解引用**——槽是零初始化的静态，首次 store 之前的收集读到 NULL 直接跳过，之后的收集读到的就是 store 留下的值。**没有值，就没有屏障的语义对象**。故本提交不实现屏障，按 D13 记明这是对 D7 的**回答**。附带的好处是写屏障也不需要：不存在「旧值未记录」的窗口。
- **裁定二：String 不登记**（承 T8-2A 裁定一 / D3 的存储裁定）。String 的字节是私有常量或 malloc 缓冲，mark 阶段会把它的首字当块头读。`TestTopLetStringIsNotRegistered` 钉住；该钉在 HEAD 上**本就绿**，方向是负控制。
- **裁定三：值记录仍停**。第 8 章给每个值记录的绑定**自己的对象**，故顶层那枚是初始化器做的一份**拷贝**而不是指向可收集块的句柄——另一套存储故事，不在本面。`TestTopLetGcValueRecordStops` 钉住；同样在 HEAD 绿。
- **裁定四：`List<String>` 在 B1a 整层之外，不是顶层特有的缺口**。实现期发现顶层 `let ns: List<String>` 停，遂复核函数级 `let xs = ["a","b"]`——**同样停**（同一个 `bndMainBody` 词）。故顶层这一停与函数级一致，是 B1a 值面的一致边界（`elemFaceOfType` 只认 abiI64/abiDouble/abiGc 三类元素），**不记为本面的缺口**。备查：将来外扩 String 元素列表时，顶层面要同步跟上。

**突变复验（9 枚；每枚锚点唯一、逐枚还原转绿）**：

- **发射层 6 枚**：`no-registration`（根表不被认识）→ **单测红**（3 枚）·**黄金红**（3 枚）；`registration-after-init`（登记落在 init 之后）→ **单测红**（`RegistersTheSlotAheadOfTheInit`）·黄金绿；`no-init-discharge` → **单测红**（`InitBodyDischargesItsPushes`）·**黄金绿**（反向一枚，见下）；`list-read-drops-elem` → **单测红**（`ListCopiedToALocalKeepsTheElementFace`）·黄金绿；`gc-read-loses-record-key` → **单测红**（`GcCopiedToALocalKeepsTheRecordFace`）·黄金绿；`every-gc-binding-takes-list-face` → **单测红**（同枚）·黄金绿。
- **运行期 3 枚**（`runtime/c/gc.c`）：`groot-marks-nothing`（登记了但扫描不标记）、`groot-off-by-one`（丢最后一槽）、`groot-registration-inert`（登记变空操作）→ 三枚皆 **单测绿·黄金红**（各 3 枚）。

**9/9 还原转绿，零存活**（补钉之后）。运行期三枚的 OFF 数字经**黄金 harness 自身**实测复现，与探针一致：`9 9 80000` / `0 28 80000` / `77 0 80000`，且**只差 stdout 一行**——`exit` 两侧皆 0、`stderr` 两侧皆空。即这是「程序照常编出、照常跑完、安静地打错数」，判别面窄而准。

**补钉记录（两枚，由电池首轮的三枚存活暴露）**：三枚存活——`list-read-drops-elem`、`gc-read-loses-record-key`、`every-gc-binding-takes-list-face`——**指向同一处**：`bindTopRead` **只有一个调用点**（`let` 语句的顶层名那一臂），而既有全部钉与黄金都走**就地读**那条路（`for x in ns`、`g.value`），**拷贝那一臂没有任何程序走过**。两枚补钉：

- **`TestTopLetGcCopiedToALocalKeepsTheRecordFace`**（`let k = g` 后 `k.value`）——杀掉 2 枚。该臂把 `recKey` 交给 `gcEnv`，丢键的拷贝拿着句柄却读不出字段，实测转为**边界停**，故一枚「干净发射」的钉就够。
- **`TestTopLetListCopiedToALocalKeepsTheElementFace`**（`let m = ns` 后 `for y in m`）——杀掉 1 枚。**这一枚首轮是空钉**：元素面取 `List<Int64>` 时，标量元素的遍历**在用处自行重新推导** kind，摘掉面产出的 IR **逐字相同**；改用 **gc 记录元素** `List<Node>` 才判别——gc 元素没有这条后路，只有绑定的面能说走出来的句柄是哪个记录。

**两层存活（三枚，给出结构性理由）**：三枚运行期突变**单测绿·黄金红**。理由：`internal/codegen` 包**不 embed 运行期**，单测层只发射 IR、既不喂 clang 也不跑 C；而 `runtime/c/gc.c` 经 `go:embed` 进 `runtime/runtime.go`，只有走到 `we build`/`we run` 的黄金层会把它编进去。**故运行期的正确性在本面上只有黄金层兜底**——与 T8-2B-0 的 `discharge-when-diverged`（单测发不出非法 IR、靠黄金层）方向相反而互补：两层各有一段**独占的**兜底面。

另记**反向一枚**：`no-init-discharge` **单测红·黄金绿**——泄漏的那枚根把句柄钉活着，黄金看到的数**是对的**，故只有单测层抓得住。这与上面三枚合起来给出本面的层间分工全貌：**发射形状由单测层守、运行期行为由黄金层守、而「泄漏掩盖缺席」这一类只有单测层守得住**。

**测试与黄金账**：`internal/codegen/` **240 → 250** 例全绿（**+11 新钉、−1 退役**）。退役的是 `TestTopLetListStops`——它命名的「列表顶层绑定停」这个事实**已不成立**，按 T8-2A 处理 String 停的同规**退役而非改锚**（改锚会留下一个名字与内容不符的钉）。conformance **770 → 773**（**+3 新枚、0 退役、0 重锚**），三枚首跑即绿；未跑 `-count=1` 时该包会命中缓存，**本行数字取自 `-count=1` 的全绿一轮**。`bnd` 词表**零增删**（diff 里新增的 `e.bnd()` 是**调用点**不是词条）：`TestBoundaryWhats` 的 `"gc carrier top-level binding"` 行**重锚**为 `"gc handle top-level binding"`、期望由边界词改为空串——改的是**期望**，不是词表。

**发现（implement 期，D13 披露）**：① **尺寸类是负控制的成败所在**：churn 与受害者**同尺寸类**时，更大的空闲块能靠**切分**满足更小的请求，first-fit 就够不到受害者——负控制四次不咬正是这条。改成「让 churn 分配**小于**句柄的块」（受害者 48B、churn 16B）后**一次咬住**。**这条对后续任何「块是否被复用」类探针都成立**。② **init 体泄漏会独自把句柄钉活，从而掩盖根表的缺席**——实测与修复后**逐字相同**。这是 T8-2B-0 移交放电的理由的兑现，也是本提交把「登记」与「放电」放在同一提交里的原因：分开做的话，只做登记的那一版**全绿而无判别力**。③ **陈旧字节不是活性证据**：被扫掉的块若无人覆写，读回去仍是旧值（p5 的 `77`），deterministic 成立但**不能**当「活」读；探针的判别数必须落在**会被覆写**的那个对象上。④ **`bindTopRead` 是覆盖面缺口所在的一臂**：它只有一个调用点，而既有钉与黄金全部走就地读；本提交以两枚补钉补上，但**每加一种存储形就要给这一臂补一钉**——记此备查。⑤ **「元素面」有两类，判别性不同**：标量元素的面可以在用处重新推导，gc 元素的面**只能**来自绑定——故钉列表类行为时，**元素取 gc 记录**才有判别力。

**停点**：T8-2B 功能面全绿；红证据两层可复核（单测 7 红 2 绿、黄金 3 红，且已记明本面的 HEAD 红是「编不出来」而非「数字错」）；三枚判别性黄金真机先行定形、手钉期望、首跑即绿；9 枚突变复验零存活（含 2 枚补钉、3 枚运行期层间分工、1 枚反向单测独守）；单测 250、黄金 773（本提交 +11/−1 与 +3）；`bnd` 词表零增删。验证阶梯：`go build ./...` + `go test ./...`（13 包全绿）+ 受影响黄金 `-count=1` + `validate.py --all --strict` + `docs_sync.py` + `git diff --check` 全部通过。**T9**（深返回与循环体内 return、String 元素列表等 B1a 外扩面）为后续。

### T9-1 sum `{tag, pay0, pay1}` 三槽 ABI 扩形（2026-09-11）

**主题**：design D8 把 sum 的布局定为三槽——String 载荷是一对 `(ptr, len)`，折不进一个字，而第二个载荷位必须**先存在**，任何构造路径才有可能往里放值。本枚只做形，值是 T9-2 的。

**可垂直验证的形态**：本枚没有任何一个点能产出非零 `pay1`，这正是它能单独落且可独立验证的原因——773 枚黄金**无一枚断言 IR**（全部只断 exit/stdout/stderr/产物树），而每个把 sum 交回来的运行期面都是**出参式**（`__we_chan_recv(ptr, ptr) -> i64`），故 **C 侧零改动**。证据 = 形钉 + 零漂移，不是行为变更；design D8 的「黄金对拍过渡期披露」指的正是这段缺口。

**扇出面（19 处）**：family 拼写（`sumSlot` / `fnAbiKind` 注释 / `abiTypOf` / `abiWordTypes` / `fitAbi`）；聚合布局（`tupleShapeOf` / `emitTupleElemValue` / `storeTupleElem`）；参数（`bindDefineParams` sum 臂 2 alloca → 3、`emitFxGate` 补 `n+"2"`）；返回（`fnRetOperand` abiSum 的常量对）；运行期产的 sum 一律补零 `pay1`（`emitSumCall3` / `emitSumCall` / `emitReduce` / `emitFind` / `emitScope` 值形）；载荷消费（`emitMatchArm` / `emitQuestion` / `emitCallCore` 结果面）。

**裁定**：**`fnRetOperand` 的常量聚合装不下 SSA 值**——与 `abiStr`、`abiTuple` 两处同陷阱，值形须走 `insertvalue` 链（照 `strPair`）。本枚尚未暴露，但它是 T9-2 构造返回面的先决条件。

**突变复验（15 枚，逐枚还原）**：**8 枚被杀**（承载 arity 本身的那些——参数三槽绑定、实参三元组、聚合取回链），**7 枚存活**，且**每一枚存活都回到同一个事实**：语料里还没有任何东西能产出或消费非零 `pay1`。五枚 `pay1` 零存（try 三连的共享形、receive/await、timeout scope、`emitReduce`、`emitFind`）因此逃逸——丢掉那个 store，槽位里本来就还是它原有的值；第六枚是 `bindDefineParams` 里第二个参数位的绑定 store，同样逃逸，因为没有逻辑读一个 sum 参数的第三个字。第七枚 `abiTypOf` 的 sum 臂是唯一与 `pay1` 无关的：它**今日无可达输入**（`faceAbiKind` 从不产 `abiSum`，折叠累加器只可能是 i64 或 double），留着是为 family 一致性——**惰性而非未测**。被杀的八枚里包括实参展开的载荷 load：它**按槽名钉**，故两个载荷字都还是零时，读错槽仍然红。

**测试与黄金账**：`internal/codegen/` **250 → 250**（M10b 矩阵的**行**扩容，不新增钉）。conformance **773 → 773**，零跳过、零黄金改动。

**披露**：本枚**没有自己的语义钉**——语料里没有程序能到达 `pay1`，语义验证在 T9-2 才成为可能（`Not-tested` 行记此）。

### T9-2 sum 构造与返回物化（2026-09-11）

**主题**：T9-1 把 sum 拓成三槽却没有任何东西能往第二个字里放值；本枚是填它的那一枚，同时也是 **fn 返回面**余下停点的退役——sum 可以在表达式位置被构造、经聚合返回、并被一个**按变体声明**（而不是按 M9b 那个无类型的字）绑定各载荷位的模式消费。

**融合 tag 空间**（裁定 3，详见 design D8 与下「裁定」节）：`Result<T, E>` **一个 tag 空间花在两边**——Ok 在 0、E 的变体在它之上——故 `Err(p)` 不指名任何自己的 tag：它解析为紧随其后的那个，而它的载荷字**就是外层槽的**，因为融合不为 E 另花一对。于是 `Err(e)` 不是第二种编码而是**改写**：E 型 sum 的 tag 是融合 tag 越过 Ok 的那一段，它**就地别名**外层载荷字。检查器接受的两种拼写都发射（`Err(Failed(a, b))`，以及带内层 match 的 `Err(e)`）。检查器同样接受的 `Err(_)` 什么都不消费——**它的缺席是融合面上的一个洞，不是一条决定**。

**报告行的泛化**：载荷不再是常量时折不进行里，故走**插值已有的逐类值转串面**在运行期渲染；常量路径**逐字节未动**——这就是 M4 程序的 IR 仍是它一直以来的 IR 的原因。同一 entry 里的两个报告点现在各取一枚全局：否则它们共用一个 `@.err`，其中一个会打出另一个的字节。

**红证据（真机，HEAD = `54b859f` 工作树）**：新钉的形状（`fn f() -> Option<Int64> { return Some(3) }`、带载荷用户 sum、`let x = Some(1)` + match、String 载荷 sum）**全部停在 `bndFnBody`**；黄金 `build-bnd-err-payload` 同刻 `exit 70`。

**语料与本面的落差（本枚最重要的发现）**：六枚单点突变**全部死在单测层**，但在本枚的黄金加入**之前**只有**两枚**死在黄金层。逃逸的四枚**指名了没有黄金到达的路径**：从 fn 里把有类型 sum 的 error 返回来、`Err(_)` 落在带变体表的槽上、变体写在表达式位置、一个 entry 里两个报告点。既有每一枚带 match 的 run 黄金 match 的都是**运行期面**（一个不带表的槽），故**带表的路径从来没有被构建过或运行过，只被检查过**。测后补的两枚 run 黄金把前三条路径端到端走通；第四条留在单测层独守。

**判别性黄金的定形（四枚）**：`build-bnd-err-payload` → **`build-err-payload-eager`**（源不变，`args` 由 build 改回 build，`exit 0`）、`run-err-payload-two-words`（`Err(Failed(1, 2))`，`exit 1`、`error: Failed: 1, 2\n`）、`run-fused-err-return`、`run-sum-ctor-expression`。四枚皆真机先行定形、手钉期望、首跑即绿（未用 `WE_UPDATE_GOLDEN`）。

**实现**：`Option<T>` 进 `classType`（对照 `Result` 分支）；变体形表 + 预算谓词（**无独立绿色，折入本枚**——它打开的程序仍停在同一个边界词）；构造子进 `emitCall` 的 Ident 分派（行位在 record 构造子之后、局部 fn 值之前），背后一个产三枚 operand 词的共享 helper 加两个适配器（返回面要 `insertvalue` 链、表达式面要 alloca+store）；`fnRetOperand` 的 abiSum 臂泛化；`sumSlot` 携带变体形表供 `emitMatchArm` 按声明种类绑定载荷；`emitCallCore` 两个实参入口都加 `abiSum` case（快路径的 `default:` 会**静默**停掉内联构造实参）；main 的 Err 报告面保留 `__we_fail`、载荷渲染泛化到值转串面并加常量快路径。

**裁定与披露**：见上「融合 tag 空间」与「披露边界」两节。

**突变复验（6 枚，逐枚还原）**：**6/6 死在单测层**，**5/6 死在黄金层**——融合构造游走里的 Err 递归、报告分隔符、报告点的全局、融合谓词、通配臂、构造 tag。第五枚（两个报告点共用一枚全局）**只有单测层守得住**，因为**没有一枚黄金在一个 entry 里有两个报告点**。

**测试与黄金账**：`internal/codegen/` **250 → 263**（+13）。conformance **773 → 776**（`build-bnd-err-payload` 改名不计数，`run-err-payload-two-words` / `run-fused-err-return` / `run-sum-ctor-expression` 三枚新增；**无其他黄金移动**——这正是本枚的面就是它自称的那一面的检验）。

### T9-3 参数/实参槽放宽 + 同步型 prim 面 + 循环体 return 结清（2026-09-11）

**主题**：T9-2 给了 sum 三个字和填它们的路径；仍然关着的是 sum 横穿**调用边界**的两个地方。参数不能是**被调方 match 的** sum——它的 match 没有变体表可测；实参只能是名字、不能是**构造**，而构造恰恰是唯一离开参数自己的声明就解析不了的拼写。第 18 章的共享态类型是同一面上新来的：它们**根本没有 ABI family**。

**sum 参数把变体表从它的声明类型里带出来**，而不是从模块的 sum 表里。前奏的 sum 没有声明可供一个键去找，而一个对参数的 match 按声明绑定它的载荷位，**与对构造出来的 sum 的 match 完全同形**；`classType` 为了预算这个参数已经走过同样的形，故两者不可能不一致。`fnParamAbi` 为此带上声明本身——**它就是 `variantSite` 一向列在绑定标注旁边的那个权威**。

**实参位随后把那份声明当自己的期望读**。写在实参处的构造是一次调用，故它走**调用臂**而不是名字臂：`f(Some(1))` 与 `f(None)` 现在都在那里解析，后者在从前没有一个括号可供抵达。其余各 family 都不读那份期望——**没有一族接受变体**——故只在参数是 sum 的地方传。

**第 18 章的 family 是一个指针**。类型实参命名载荷的**域**、从不命名值的布局，故它不被读；参数名骑 `prims` 环境，于是对它调方法与对帧内自建对象调方法**解析完全相同**。对象在调用期间保持有根**而无需被调方做事**：调用方在造出对象时就推了根，而那个帧在被调方运行期间一直活着。该 family 由**限定名抵达时经过的 import** 识别，而不是仅由名字识别——一个把别的东西别名成 `conc` 并导出名为 `Mutex` 的类型的模块，是用户自己的类型穿着同一个名字。

**chapter 18 对象回来**是本枚**故意不开**的面：它需要一个结果面还没有的被调方 operand 与调用方结果槽，且调用方会把值当一个标量读。两道守卫拒它——`fitAbi` 在分类处（故调用点按同一个答案拒，而不是按一个它从没见过的 body 拒），以及 `fnRetOperand` 的 switch（没有该 family 的臂）。**突变电池记录其中任何一道单独就成立**，故第一道上的注释现在说的是实话，而不是宣称结果面会误读。

**循环体 return 的披露在此结清，且它结清的形式是「这条记录写下时就是错的」，而不是「一个缺口被填上了」**。`for`/`while` 体从 T2 起就吃 return——`TestReturnInsideLoop` 早已钉住——main 里与 fn 里都是；`emitReturn` 在任意深度放电整个活集：循环帧是 break/continue 停的地方，而 return 离开函数。同样的可达性在披露被写下的那个提交上也成立。**发射器一处未改；补上的是缺的那枚测试。**

**红证据（真机，HEAD = `69dbf78` 工作树）**：四枚新钉里三枚停在边界词——sum 参数的表与实参位构造停在 `bndMainBody`，同步型参数停在 `bndFnBody`。第四枚（prim 返回）在那个树上**也是绿的**：它是**本枚故意留关的那一面的负钉**而非翻转，而电池显示**这正是放宽的拒绝所依靠的东西**。

**判别性黄金的定形（两枚）**：`run-sum-param-match`、`run-prim-param`——两枚在 `69dbf78` 上都 `exit 70` 带 body 词；`build-bnd-prim-return` 钉住返回面（负钉）。

**突变复验（14 枚，逐枚还原、锚点先断言唯一）**：**12 枚被杀**——**7 枚两层都死**（family 分类、参数绑定、实参指针、表的来源、携带的声明、期望的传递、构造的 tag）；**5 枚只死在单测层**，因为**没有黄金到达它们所动的面**（import 别名守卫、适配器的字表、门的参数表、裸名变体回退、绑定名实参的载荷字）。**两枚存活有结构性理由**：摘掉 `fitAbi` 对 prim 返回的拒绝**什么也不改**，因为 `fnRetOperand` 自己拒绝同一个面；把期望传给每一族**也什么也不改**，因为非 sum 的声明不为正在写的实参命名任何变体。两枚首轮未编译（一个被遮蔽的声明，故它的第一次 FAIL 是构建失败而不是判决）／锚点不唯一，皆已重定目标并重跑，记录的判决是重跑的那一次。

**测试与黄金账**：`internal/codegen/` **263 → 271**（+8）。conformance **776 → 779**，无其他黄金移动。

**披露**：chapter 18 对象作为返回值；`List` 形参；同步型对象方法调用的**值**用在 main 体里会停、同样的调用在 fn 体内发射（main-body 边界，`69dbf78` 与本枚一致）。T9-4 的 red evidence 也记录在案：本枚**未触碰** String 存储面。

### T9-4 String 两字槽存储面 + M15 重锚（2026-09-11）

**主题**：一个**被 body 写入**的标量名会把它的 SSA operand 让给一个栈槽，而槽正是「写入对之后每一次读取可见」的原因（T2-a）。String 名从前没有这样的面：它的值是一对 `(ptr, len)`，故其 operand 形是**两个 SSA 字**，而两个字无法被改写——`let s = "a"; s = "b"` 没有地方放 `"b"`，停在 body 词。**模块层早就有答案**：一个顶层 String 绑定是一对每个读都 load 的全局（T8-2A）。本枚是那一对**下移一层帧**，也是 M9b 语句集等的最后一枚。

**红证据（真机，HEAD = `55fa2b7` 工作树）**：`var s: String = "a"` 与 `let s = "a"; s = "b"` 都停在 `bndMainBody`；fn 里对 String 形参赋值停在 `bndFnBody`；黄金 `build-bnd-string-assign` 同刻 `exit 70`。

**实现（11 处）**：

1. **`strBinding` 增两字段** `slot` / `lenSlot`——一个被写名拥有的两个字。三个面互斥且**优先级是存储 > operand 对 > 字节**。
2. **三个 helper**（紧随 `bindScalarSlot`）：`bindStringSlot`（保留两个字、依次 store、登记进 `strEnv`——标量 `bindScalarSlot` 的 String 对偶，且**两个字都在任一 store 之前保留**，故一个名的存储与初始化器发过什么无关）、`strSlotLoad`（**每次使用都发新鲜 load**——这正是存储面的全部意义，也是赋值对之后每一次读可见的原因）、`emitStrStore`（**先算值、后两 store**）。
3. **三处绑定面**沿用标量面自 T2 起就在问的那个问题「这个 body 写不写这个名字」：`emitVarBinding` 的 String 分支；`emitLetBinding` 的字面量臂与 Ident 别名臂；`bindStringValue` 与 `bindResult` 的 `ckStr` 臂。
4. **参数是同一个切分、不同的价格**：`bindDefineParams` 的 abiStr 臂——它的两个字是**只读的 IR 字**，故写它的 body 先把它们拷进自己的字再让写入落地，只读的 body 一分钱不花。
5. **三处读面各自 normalize**：表达式面 `emitStringExpr` 的 Ident 臂（在 `dataOp` 判据**之前**）、fn 返回面 `fnRetOperand` 的 abiStr Ident 臂、`.value` 解包面 `bindingFace`。`*ast.Assign` 的 String 臂放在**标量查找之前**（String 名住在 `strEnv` 而不是 `scalars`）。
6. **D13 的循环提升纪律按 `e.slot` 自动继承**：两个字都是 `e.slot` 出来的 alloca，由 `bodyText()` 提升到 entry——一个字都不会在循环体里被重新分配。

**真机探针发现的漏改（本枚最重要的一处）**：`fnRetOperand` 的 abiStr Ident 臂**直接读 `strEnv` 并要求 `dataOp != ""`**——槽绑定的 String 没有 `dataOp`，故它必须先 `strSlotLoad`。最初漏掉这一处时，参数赋值形**编得过、然后返回名字被绑定时的那份值**。判据因此落在「拼接实际消费的是哪两个寄存器」上，而不是落在形状上：正/反两种 store 次序产出的指令**形状相同**，只有寄存器判据抓得住。

**裁定**：

- **赋值先算值、后写两个字**。`s = s + "b"` 经那次计算读名字自己的对；先落的 store 会把半写的名字喂给拼接。
- **var String 必须标注类型**，与 var 标量完全一样。无标注的 `var s = "a"` 停在 body 词，且它在**本枚之前**的树上同样停——`var n = 0` 与它同停，故**标注要求属于 var 面而不是 String 的缺口**。新黄金把它钉成一条诚实边界，而不是让这次拓宽读起来像「已全」。
- **`.value` 解包面与 fn 返回面同样要 normalize**：读面有三处，不是一处。
- **本枚不触碰无标注 var**：它是两个族共有的预存在边界，钉住而不修（`build-bnd-var-unannotated`）。

**M15 重锚（三处）**：`control-03` 的 mutable String 绑定原是该任务的 test-malformed 载体，traps.md 记它是「mutable binding 落地后转 latent 的那个校准」——它现在**跑起来并落在 latent**（check 0 / test 1），突变电池的子进程面给出同一判决。该桶仍需要一枚 check 干净、test 停的载体，而本 build 拥有的**最后一个停词是泛型声明**：第 10 章认可它，代码生成拒绝它直到 B1b 单态化（`bndGenericFns`）。载体因此取 `control-03` 自己的 `echo` 改写成泛型形——桶锚在**工具链的边界**上，而不是锚在一枚语义突变上。三枚备选被弃：`List<String>` 形参（预存在的 `elemFaceOfType` 边界，非本任务的面）、三载荷 sum（撞预算边界，只在它是 D8 的披露边界这一点上正当）、`var` 一个 sum/记录（`emitVarBinding` 的 `default` 是 D8 未命名的残留——把校准锚在一个未设计的洞上，三枚里最弱）。

**判别性黄金的定形（四枚，真机先行定形、手钉期望、首跑即绿）**：`run-string-reassign`（`let s = "a"; s = "b"; io.println(s)` → `b\n`）、`run-string-mutable-binding`（`fn pick(s: String) -> String { var r: String = s; if s == "mirror" { r = "mirror-x" }; r = r + "!"; return r }`，main 里 `pick("mirror")` / `pick("plain")` 加一段 `acc = acc + "b"` / `acc = acc + "c"` → `mirror-x!\nplain!\nabc\n`；**参数拷贝、写后读、自引用赋值、长度变化的写**四种形一次走通）、`run-string-binding-faces`（**由电池的三枚存活补定形**：一个被写的名字分别从**插值字面量**、**拼接表达式**、**调用结果**三个生产面绑定，各自写后读；再加一个被写的 `Name`（newtype over String）形参的 `.value` 读——四个生产面一次走通 → `A\nB\nC\nt\n`）、`build-bnd-var-unannotated`（负钉：`var s = "a"` 无标注 → `exit 70` + `bndMainBody` 原词）。

**突变复验（14 枚，逐枚还原、锚点先断言唯一；单测层 + 黄金层各记一次）**：**9 枚两层皆死**——V1 `emitVarBinding` 不收 String（三枚钉）、V2 var String 共用初始化器面（同三枚钉）、V3 `bindStringSlot` 占字不填（三枚钉）、L1 被写的字面量不占自己的字（`TestStringReassignStoresThroughTheSameWords` · `TestStringSelfAssignReadsBeforeItWrites` · `run-string-reassign`）、A1 赋值够不着 String 名（五枚钉 · 两枚黄金）、S1 `strSlotLoad` 从数据槽读长度（五枚钉 · 两枚黄金）、S2 被写名读绑定时的面（四枚钉 · 两枚黄金）、R1 fn 返回不读被写名的字（`TestStringFnReturnReadsTheWritableName` · `TestStringParamTakesSlotWhenAssigned` · `run-string-mutable-binding`）、**A2 store 丢长度字**（两层都在 `go build` 阶段死，报文 `declared and not used: l`——**这一枚的判决来自编译器而非断言**，记此以免读成测试捕获）；**2 枚单测红·黄金绿**——L2 被写的别名不占自己的字（`TestStringAliasOwnWords` 钉住 IR，本批黄金没有一枚在别名上写）、B3 被写的 String 形参不拷入字（`TestStringParamTakesSlotWhenAssigned`，黄金不跨那条路径）；**3 枚首轮两层齐活**——B1 `bindStringValue` 丢掉被写名的存储、B2 `bindResult` 的 ckStr 臂同形、R2 `bindingFace` 交回槽而非载入的对。

**补钉记录（四枚钉 + 一枚黄金，由三枚存活暴露）**：三枚存活**同源**——`e.assigned` 那条判据在 String 面上一共**五个入口**（`let` 字面量臂、`let` 别名臂、`bindStringValue`、`bindResult` 的 ckStr 臂、`bindDefineParams` 的 abiStr 臂；`var` 绑定不在其中，它按定义就是被写的名字，故无条件占字），八枚实现钉把两个 `let` 臂与参数走了个遍，**另外两个（值表达式与调用结果）一枚都没走到**；三处 normalize 的读面同理，表达式面（S2）与 fn 返回面（R1）都有钉，`.value` 解包面（R2）一枚没有。补的不是断言而是**入口**：`TestStringConcatBindingOwnsItsWords`（拼接表达式）、`TestStringInterpolatedBindingOwnsItsWords`（插值字面量——`bindStringValue` 的两个调用点各一枚）、`TestStringCallResultOwnsItsWords`（调用结果）、`TestNewtypeValueReadsTheWritableName`（被写的 newtype-over-String 名的 `.value` 读），另加一枚黄金 `run-string-binding-faces` 把四个生产面一次走通。**三枚重跑，两层齐红**：B1 单测红在拼接与插值两枚、B2 单测红在调用结果、R2 单测红在 `.value`，三枚的黄金层同红。

**R2 的可达性判定（真机探针作证）**：这一面要求一个名字**同时在 `ntEnv` 与「有字的 `strEnv`」里**，故首轮存活的第一问是它到底可不可达。两条入口都真：newtype-over-String 的**形参**（`bindDefineParams` 在 `newtypeOf` 命中时记 `ntEnv`，随后 abiStr 臂按 `e.assigned` 给字）与它的 **`let` 别名**（`emitLetBinding` 的 Ident 臂记 `ntEnv`，紧随的 `strEnv` 臂按 `e.assigned` 给字）。真机上 `fn label(n: Name) -> String { n = n; return n.value }` 在未突变树上 `check 0 / build 0 / run` 出 `t`，在突变树上 clang 报 `build/demo.ll:40:46: error: expected value token`——**不是惰性面，是一个真的缺口**（把槽的地址当 `{ptr, i64}` 交给聚合返回）。这一点与 T9-1 的 `abiTypOf` sum 臂不同：那一枚确实无可达输入、按惰性记账。

**测试与黄金账**：`internal/codegen/` **271 → 283**（+8 实现钉、+4 电池补钉）。conformance **779 → 782**（**+4 新、−1 退役**），`build-bnd-string-assign` 退役、`run-string-reassign` 接手运行面。翻绿集**恰好为一**（`build-bnd-string-assign`），与计划要求的「任何额外翻绿都是 bug」相符。M15 全包绿（`go test -count=1 ./internal/benchmarks/`）。

**披露（本枚记入完成记录）**：prim 返回停在边界（T9-3 的负钉）；main 里 prim 方法调用的**值**会停、fn 里正常（main-body 边界，T9-3 已记）；`List` 形参停在边界（预存在）；`match` 的内联 scrutinee；match 臂里的嵌套变体模式；`?` 在 fn 内（裁定 2）；用户自有 sum 带 sum 载荷；**无标注的 `var`（两个族共有，本枚未触碰，由 `build-bnd-var-unannotated` 钉住）**；`docs/benchmarks.md:50` 那句本身（「sum-typed and synchronized-typed parameters do not enter runnable slots」「string concatenation `+` is an honest boundary」「a function body ends in exactly one tail return」三条在本枚之后皆已陈朽——**归 T12**，本枚只把原句与新事实一并写进完成记录）。

**停点**：T9-4 功能面全绿——真机探针语料逐枚复核：var 绑定、重赋值、自引用赋值、别名、形参赋值、形参只读、fn 返回、闭包捕获、调用初始化、循环体内 var 十形都编出并跑出预期值；无标注 `var`（String 与标量两形）停在 `bndMainBody`（负例）；`var` 的 task 捕获按 `E1603` 被**检查器**拒斥（不在本面）。红证据真机可复核；四枚判别性黄金真机先行定形、手钉期望、首跑即绿；14 枚突变复验（含三枚补钉）见上；单测 283、黄金 782；M15 三处重锚后 benchmarks 全包绿。

### T9 收口：工件对账（2026-09-11）

四枚提交 `54b859f` / `69dbf78` / `55fa2b7` / `7c8ebce`，单测 **250 → 283**、黄金 **773 → 782**。收口改动五处：

1. **design D8 改写**：布局裁决（三槽，含被拒替代）、**Err 融合 tag 空间**的语义与三行真值样表、**预算规则单点强制于 `classType`** 及其残余边界、fn 返回面（同步类型判为不开的两道守卫）、参数槽放宽（含 T9-3 的声明携带与调用臂）、T4 提请的 String 两字槽存储面（T9-4 已兑现）。
2. **design D11 表增一行**：`control-03 string-mutable-binding-decline` → **latent**（T4 裁定期识别、本表写成时尚未列入），并新增「T9-4 的载体迁移」一条：桶的活载体改为**泛型声明边界**，三枚备选及其被弃理由在案。
3. **design D12 增「落地对账（as-built）」**：T9 的实际黄金落点十枚逐一列名、`build-bnd-string-assign` 退役在案；`test-fn-body-bnd` **已不存在**（T7-2 `1d389fc` 取代为 `test-fn-body-for-list.json`）——本表的该项是对账项而非待办。
4. **proposal.md:638 的陈朽披露改写**：T8-2B-0 记的「`return` 在循环体内这一面在 B1a 不可达」**是误记**——`for`/`while` 体从 T2 起就吃 return（`TestReturnInsideLoop` 早已钉住），且同样的可达性在披露写下的那个提交上就成立。T9-3 补上循环体臂（`TestReturnFromALoopBodyDischargesTheWholeLiveSet`）并把披露关掉；**补的是测试，不是代码**（`emitReturn` 一处未改）。原句加删除线保留，改写为「这条记录写下时就是错的」。
5. **tasks.md 两枚 checkbox 勾选**：T9-1（含形钉与零漂移的验证行）；T9-2/T9-3/T9-4 合并项（含三枚逐条收口、五条裁定、两条实现记录）。

### T10-1 检查器域门放宽（2026-09-11）

**主题**：域门从前是标量集，而第 10 章正是理由——复合相等是生成的 `.equals()`、不是算子，故没有 Eq 实例的比较子没有相等可断言；而 `fn f<T where T: Eq>` 需要规范未定的内建实例集。同一条裁决却留下了**确实生成了一份**的复合：声明 Eq 的记录/和式/新类型有语言钉死的相等，其组分按子句本就被检查的那条规则各有自己的相等，故域恰好闭在 ch10 认可处，并且只在**能力本身**停的地方停。

**判据为什么不复用 `carriesType`（三处语义不合）**：(a) 它无条件接受每个基类型——子句的要求 Float64/Rune/Bytes 都满足——而比较子域只认叶集；(b) 它**不认元组**（没有 `tupleType` 臂），而元组无声明可载子句、按裁定 5 随其元素；(c) 它是 **E0823 的判据**，改它等于改规范面，而 `codegen-mono` 是纯实现变更（无 `specs/` 增量）。故**平行新增 `inEqDomain`**，`carries` 一字未动。

**递归的底是叶集、不是子句**：什么都没声明的组分已是 E0823 该拒的，故域门在那里与能力判据一致、从不见到那种情形；第二层的用处是**声明了 Eq 却触到断言不钉的叶子**的组分——这正是裁决认下的**诚实不对称**：`record P { x: Float64 } derives Eq` 合法、`.equals()` 可用、断言仍然停。

**词退役**：`bndAssertEqDomain` 删除，残形并进代码生成已有的残余词（`bndGenericFns`），两阶段报同一套词汇。代价是裁定过的：一个只是**忘了子句**的记录，现在会被告知单态化。

**测试与黄金账**：`internal/typecheck/` **75 → 76**（`TestAssertEqualEqDomain`，镜像表的检查面半边）；conformance **782 → 783**——`check-assertequal-domain-boundary` 改名 `check-assertequal-domain-eq`（同源，转 check 干净），新钉 `check-assertequal-residual-bnd`（残余停点在新词下）。`internal/codegen/` 未动（仍 283）。

**红证据（真机，HEAD = `7c8ebce` 工作树）**：derives-Eq 记录行在退役前停在旧词 `bndAssertEqDomain`；`test-asserteq-domain-bnd` 同刻 `exit 70`。

### T10-2a 叶族 + 记录结构比较（2026-09-11）

**主题**：T10-1 把域闭到了声明 Eq 的复合上，故到达发射面的记录与从前的叶子一样多。本枚是那一步发射：记录的比较按**声明序**是它各字段的比较，每个叶子报出它被抵达的位置。

**比较是直线的**：`__we_task_fail` 从不返回，故首个不等的叶子就是最后一个跑到的调用——walk 不需要基本块、也不需要自己的短路，**发射序就是语义的全部**。测试因此断言**调用的次序**而不是指令形状：一个把叶子发出声明序之外的 walk 会报出一个程序从未抵达的位置。

**报文契约（裁定 4/6 展开，逐字节）**：

- 顶层 Int64/String 走**旧符号**（`__we_assert_eq_i64` / `_str`），故既有字节钉死的黄金模块是**结构性**不变的；顶层路径为 `""` 时整条 `at` 子句不出现。
- 带的路径是**位置索引段**：`at Point.x`、`at Point.inner.x`（嵌套记录只贡献字段名、不重复型名）、`at Shape.Rect.1`、`at 1`（元组无名根 = 裸索引）。
- 顶层 Bool/UInt64 **一并对齐**到同一模具（裁定 7）：这是 M10b 冻结面上一处**行为变更**（今日 `assertEqual(true, false)` 发 `__we_assert_eq_i64(i64 1, i64 0)`）、一枚无黄金钉住的行为；`_bool_at` 经 `__we_str_of_bool` 出 true/false，不第二次拼写这两个词。

**报文精确尺寸**：两趟 `vsnprintf` + malloc——路径深度是这一族唯一由**程序**定尺寸的输入，定长缓冲等于把静默截断放在新输入上。深度界 **32 段落在下降处**而不是比较处：比较只看得到根的键，花在比较处的守卫对它存在的形状永不触发（突变证据：守卫搬回比较处即接受 33 层链）。

**测试与黄金账**：`internal/codegen/` **283 → 291**（+8）；conformance **783 → 787**（+4：`test-asserteq-record-int-fail` / `-record-str-fail` / `-bool-fail` / `-u64-fail`）；`internal/typecheck/` 不变（76）。

**红证据（真机，HEAD = `0e18bbd` 工作树）**：记录的 walk 停在新词 `bndGenericFns`；两枚对齐行发出它们的前 T10 报文（`assertEqual(true, false)` → `__we_assert_eq_i64(i64 1, i64 0)`、`assertEqual(18446744073709551615u64, 2u64)` → `i64 -1, i64 2`）。

**突变复验（8 枚，逐枚还原、锚点先断言唯一）**：字段序、UInt64 行、Eq 重查、路径分隔符死单测层，后两者**兼死黄金层**；顶层 String 行**只死单测层，设计如此**（走位置行与走旧行的字节相同——这正是「既有字节不变是结构性事实」的检验）；深度界**跟随常量的测试存活、写死字面量后死**；守卫位置搬到比较处死。

**披露**：`layout` 无 Rune 行，故 Rune 字段比域停**早一步**（记录的构造即 body 的停点）；无子句记录的停点语见裁定 3。

### T10-2c 元组 + 新类型透明（2026-09-11）

**主题**：元组**没有地方可声明 Eq**（没有声明），故裁决给它其元素有的域、别的不给。本枚是那条 walk。新类型面**零新增**：包装在别处已经擦除，walk 继承那份擦除而不是重复它。

**位置 = 裸索引**：记录根报在型名下（记录有一个），元组没有，故索引是抵达元素的全部；本身就是记录的元素在其后贡献字段名（`0.x`），而记录的记录字段报 `Outer.i.x`——两者读法相同：抵达复合的那一段已经说明它是哪一个，只有根需要名字。

**元素形取自元组自己的分类**（`tupEnv` 携带 `emitTupleAgg` 交回的同一对），故一个已绑定到元组的名按绑定记下的形比较、字面量按聚合记下的形比较——元组的形是它的值不携带的静态事实。

**元素集是叶集**：double 元素与 Rune 元素像对应记录字段一样停；**和式元素本枚也停**（T10-3 未把它打开，边界理由见 T10-3 的订正 2）；嵌套元组比域**早一阶段**停——检查器认它（叶子在域里），聚合布局没有「元组内含元组」的行。

**新类型透明是被继承、然后被钉住的**：`emitNewtypeCtor` 早已交回底层的族与底层的型名、`bindResult` 早已只把包装记进 `ntEnv`，故叶子上的新类型以叶子的身份抵达并在报文中不带任何位置，记录上的新类型以记录的身份抵达、在记录名下报位。两端都钉——因为这条性质**对报文是有承载力的**：一个某天开始贡献自己名字的包装会在不改任何值的情况下改报文。

**测试与黄金账**：`internal/codegen/` **291 → 296**（+5）；conformance **787 → 790**（+3：`test-asserteq-tuple-pass` / `-tuple-fail` / `-newtype-fail`）。

**红证据（真机，HEAD = `83dc2c0` 工作树）**：三枚入域元组行在镜像表与三枚 walk 测试里同样停在新词；**每一枚新类型行在那棵树上已经是绿的**——这正是「继承」被看见而不是被假设的原因。

**突变复验（2 枚，逐枚还原）**：索引 off-by-one 两层皆死；元素用元组自身位置（而非元素自己的位置）抵达**只死单测层**。

### T10-3 和式 tag 分派 + 载荷递归（2026-09-11）

**主题**：T10-2c 留下的和式是唯一被拒的复合，且它被拒的理由别族没有：记录与元组的叶子坐在**声明钉死的偏移**上，故那两条 walk 是声明序里的直线调用；和式的叶子取决于**运行值最终是哪一个变体**。本枚是那套分派，以及它唯一还缺的东西——槽自己从不携带的**型名**。

**判别式先读，它就是「两个比较子有没有共同载荷」的全部**。两个不同变体是两个不同声明，它们的载荷字不是同一字段读两次，故**不等边报两名后从不返回**：它调的助手**无条件**报告（调用者已经判定了 tag 不等），而发射出的调用声明 `noreturn`——这才使那条边的 `unreachable` 成为**构造的事实**，而不是对「程序能造出哪些 tag」的推理。相等边按 tag 分派：每变体一块，各自走该变体声明的各位置、各自 `br` 到同一个 join。**最后一个变体是链的落点**，那是拼法而不是情形——表**就是** tag 空间。

**载荷读面逐字镜像 `bindArmWord`**：同一批字、同一个次序、同一种读法，因为 match 臂的绑定与这次比较是**同一布局的两个读者**，一个槽用一种方式绑定载荷、用另一种方式比较它，就是对同一个问题给出两个答案。把这条镜像钉住的测试比较的是**操作数从哪些指针 load 来**、而不是操作数本身：一个把同一个字读两遍的 walk 仍会发出两个位置、仍会给它们命名，错的只有值。

**槽获得它所持和式的名字**。它的载荷表说每个变体装什么；它说不出**声明有没有派生 Eq**，而第 10 章让子句成为复合相等的全部——故 walk 在**声明处重查**子句，这意味着表必须带着它的来源名一同抵达。给这个名字的路径是：变体处、构造子、参数的分类、调用的返回。**融合 Result 的 Err 别名故意带空键**：融合抹掉了尾巴来自哪个和式，空键被读作它本来的意思——没有声明可抵达的和式——而那正是那个面本来会得到的答案。

**报文**：`at Shape: got Circle, want Rect`（变体名是编译器写的名字，**不引号**）、`at Shape.Rect.1: got 5, want 6`。

**测试与黄金账**：`internal/codegen/` **296 → 304**（+8）；conformance **790 → 796**（+6）。

**红证据（真机，`26a7104` 提交的 worktree，携带本枚的测试与黄金）**：每一行和式都停在残余词「generic functions in code generation」——tag 分派、载荷位置、参数的 face、调用的返回同样；六枚和式黄金全部失败。

**突变复验（6 枚，逐枚还原、锚点先断言唯一、逐枚以 sha256 复核还原）**：载荷字规则（`eqSumWord`）**两层皆死**；构造的键**两层皆死**（四枚单测行 + 除参数外全部和式黄金）；参数的键**两层皆死**；名链尾、分派的落点、String 载荷的长度字**黄金层死**（这三处是形状而非值，行为面是它们的正确判据）。

**电池的产物（一枚黄金）**：第三枚突变（参数键）**只死单测层、一枚黄金都不死**——没有一个黄金把一个和式**当参数**传进被调方，而那恰恰是用户会写的程序。补钉 `test-asserteq-sum-param-fail` 后该枚两层齐死。这与 T9-4 的 `run-string-binding-faces` 同类：**电池的存活枚指名了没有黄金抵达的路径**。

**超出计划的三枚黄金及其理由**：计划为和式列了三枚（pass / 标签不等 / 载荷不等）。`-str-fail` 钉**两字载荷的读面**（String 载荷在前三枚里一枚都没有，而它是 `bindArmWord` 镜像里唯一两字的位置）；`-three-variants-fail` 钉**分派的中段**——两变体的链**够不到**中间那一步（一步链的测试与它的落点之间什么都没有），`eqst` 块族在两变体上**结构性不可达**。

**订正（三处，逐条记）**：

1. **计划文档的元组报文示例是笔误**：报文契约表把 `(1, 2) vs (1, 3)` 写成 `at 0: got 1, want 3`，而首个不等位是**索引 1**、两侧值是 **2 与 3**——正确形 `at 1: got 2, want 3`。黄金 `test-asserteq-tuple-fail` 钉的是正确形（`at 1: got "ab", want "cd"`）。示例要说的「无名根 = 裸索引」成立，数字写错了。
2. **和式内嵌元组的边界理由订正**：计划的披露清单把它与嵌套元组并列写成「`tupleShapeOf` 拒」。实际 `tupleShapeOf` **有 `abiSum` 臂、接受和式元素**并把它的三个字存进聚合；真因在**下一层**——`tupleElem` 只带 kind/key/typ/off/words，**不带变体表**，而变体表正是「tag 的载荷是什么」的答案。两者相邻但不同源：嵌套元组确实被 `tupleShapeOf` 拒（没有 `abiTuple` 臂）。判决不变（codegen 停 = 诚实边界），理由订正。
3. **裸变体名与载荷预算停在 body 词而非域词**：和式裸变体名（`Dot`/`None`）在值位置不被支持——这是 **body** 停点；`Both(Int64, String)`（3 词超预算）与 Rune 载荷同理，ABI 分类先跑，故它们**到不了 walk**。测试因此只写可构造的拼法（`Circle(1)` vs `Circle(2)`），并把 Rune 行从镜像表里撤掉——留在表里会声称一个域判决，而实际发生的是 body 停点。

**退路未采用**：计划为 tag 分派链预留的退路（「两侧 tag 均可静态判定才走载荷递归，否则 bnd」）**未采用**，落地的是完整动态分派。理由是它**是降级而非等价**：`pick(1)` 对 `pick(2)` 是普通程序，而它省下的分派每变体只花一个块。

### T10 收口：工件对账（2026-09-11）

四枚提交 `0e18bbd` / `83dc2c0` / `26a7104` / `a1beee9`，单测 **283 → 304**、typecheck **75 → 76**、黄金 **782 → 796**。收口改动三处：

1. **design D9 增「落地对账（T10 收口，as-built）」**：七项裁定（复合自带 `derives Eq` / 叶子限原标量集 / 残形不设新词 / 报文 = 路径 + 叶子值 / 元组仅结构判 / 路径 = 位置索引段 / 顶层 Bool·UInt64 一并对齐）与逐字节报文契约入 D9；D9 原文「字段/载荷/元素同样入域」在此精确化为「组分须落在叶集或自带子句（或对元组而言，随其元素）」——原文那句读起来像「Float64 字段自动入域」，而 as-built 是**不入**。
2. **D9 的残余停点裁决 as-built 兑现**：D9 原文已倾向「不设新词、残形并 `bndGenericFns`」（当年写成带问号的猜测），as-built 即此；`bndAssertEqDomain` 删除、代价（忘了子句的记录拿到泛型词）在案。
3. **tasks.md 的 T10 checkbox 勾选**：含对账（两枚黄金改名、`check-assertequal-residual-bnd` 新钉、两族枚数 6→19 与 4→5）、四枚逐条收口、七条裁定、三条披露与电池账。

### T11-3 窄整型逐宽溢出检查（2026-09-11）

**实现**（`internal/codegen/codegen.go`，diff 443 插入 / 114 删除）：ch7 的检查算术按**声明宽度**检查，而 i64 寄存器对八枚整型中的六枚**不是**那个宽度。三个面：

1. **宽度经发射流出，不另造分类塔**——`emitNumOperand` 增第 4 个返回值（镜像 `isFloat` 的第 2 个），`emitNumExpr` 作为丢弃宽度的 shim 让既有调用点一字不动。宽度随六面流转：`scalarSlot` / `capSlot` / `topSlot` / `callResult` / `listElem` / `captureSet`（后者用平行的 `nums []string`）。助手族 `narrowName` / `narrowBounds` / `narrowMask` / `narrowSuffix` / `litNarrow` / `annNarrow` / `narrowDomain` 与守卫 `narrowGuard`（`icmp slt`/`icmp sgt` 对界 → `or i1` → `task_fail`）新增在 `litStrKind` 之后、`trap` 之前。
2. **窄域是替换而非叠加**——`+ - *` 与一元 `-` 的窄路径发平算术 + 宽度界守卫，**不走** i64 内建。`UInt32` 乘法的真积可达 2^64，`smul.with.overflow.i64` 的**有符号**位会在一批合法乘积上误报——那是替窄域做错判决的判据。`<<` 保留全部三枚守卫（量 < 64、ashr 回环、宽度界；回环在窄域下仍活：`2^31u32 << 33u32` 的真积 2^64 在 i64 回绕成 0，宽度界放过 0，回环逮住）。`/` 的宽度界只加在商上（两处返回路径各一），`%` 不需（`|r| < |除数|`），`& | ^ >>` 皆不需。两条陷阱消息共用新 `ovMsg`（`<型名> shift overflow`）。
3. **`~` 的宽度掩码修正与本任务同落**——i64 补码只在操作数**符号扩展**时才是该宽度的补码，`~200u8` 曾得 -201。新增 `narrowMask`（UInt8→255、UInt16→65535、UInt32→4294967295；有符号窄型与两个全宽仍 -1），`valueKind` 的 `*ast.Unary` 增 `~` 臂。`annNarrow`（注解优先、回退字面量后缀）在 `bindNumericValue` 的 `res.num == ""` 处兜底，`emitNewtypeCtor`、`emitForRange`、`collectCaptures` 各接一线。

**红证据（真机，`a1beee9` 的 worktree，携带本枚的 15 枚单测与 8 枚黄金）**：单测层 **13/15 红**——两枚绿的是断言「无新行为」的 `TestFullWidthArithmeticKeepsTheIntrinsic` 与 `TestNarrowLogicIsTotal`（正确的红绿划分）。黄金层 **8/8 红**，且红形逐枚即缺陷本身：`add`/`uint32-mul`/`neg`/`div`/`capture`/`range` 六枚 **exit 0**（`127+1` 静默回绕成 -128、`max*2` 成 4294967294、`-(-128)` 成 -128、`-128/-1` 成 -128、捕获副本丢宽度后回绕、循环变量丢宽度后打印 200…252）；`shift` 一枚 exit 1 但消息报**寄存器**宽度（`Int64 shift overflow`）；`values` 一枚 exit 70（`~u` 与 fn 参数面在 pre-变更树尚是 M9b 边界）。

**电池（8 枚单点突变，逐枚还原、锚点先断言 `count==1`、两层各记判决）**：

| 突变 | 单测层 | 黄金层 |
| --- | --- | --- |
| `narrowGuard` 上界 `icmp sgt`→`icmp slt` | 死（`…ChecksItsOwnWidth`） | 死 4/8（`neg` 一枚**因更早的同名陷阱存活**——初值 `-128i8` 自身先撞坏守卫，见下） |
| `narrowBounds` UInt32 上界→2147483647 | 死（`…/UInt32`） | 死（`values`） |
| `narrowMask` UInt8 掩码→65535 | 死（`…/UInt8`） | 死（`values` 的 `~200u8`） |
| `annNarrow` 注解分支禁用 | 死（`…TakesTheAnnotation`） | **存活**——唯一的注解面是值形 join，源码不可达（见下） |
| `capNum` 返回 `""` | **存活** | **存活** → 补黄金后两层齐死 |
| `emitForRange` 丢弃宽度 | **存活** | **存活** → 补黄金后两层齐死 |
| `ovMsg` 恒 `Int64` | 死 ×2 | **存活** → 补黄金后两层死 |
| `narrowDomain` 左优先→右优先 | 死（新钉 `…PrefersTheSideThatHasAWidth`） | 存活（ch7 无隐式转换 ⇒ 良类型源两侧同宽，**该突变等价**，符合预期） |

**电池的产物（三枚黄金 + 一枚单测）**：三枚**两层齐活**的突变各自指名一条没有黄金抵达的路径——捕获副本的宽度（`capNum`）、range 循环变量的宽度（`emitForRange`）、窄型移位消息的命名（`ovMsg`）。补钉 `run-narrow-capture-panic` / `run-narrow-range-panic` / `run-narrow-shift-overflow-panic`，逐枚复核在新钉上**该突变两层齐死**。`narrowDomain` 锚另落 `TestNarrowDomainPrefersTheSideThatHasAWidth`（两侧名都带宽度，则任一侧被丢弃即失守；两侧序都写，故单侧偏好被钉住）。

**阴性结果（记在案而非藏起来）**：`run-narrow-neg-overflow-panic` 不区分 `narrowGuard` 的上界——上界被替换成下界后它**仍以同一消息失败**，因为初值 `-128i8` 的发射本身就是一次一元负号，坏守卫在它身上先炸。该黄金钉的是**退出码与消息**，上界由 `TestNarrowArithmeticChecksItsOwnWidth` 与 `TestNarrowNegationChecksTheWidthMinimum` 独守。

**测试与黄金账**（提交 `ca268e7`）：`internal/codegen/` **304 → 319**（+15，新 `narrow_test.go`）；`internal/typecheck/` **76**（本枚不动检查面）；conformance **796 → 804**（+8；`run-narrow-*` 族 0 → 8）。`go build ./...` + `go vet ./...` + `gofmt -l` 干净；`go test -count=1 ./...` 13 包全绿（conformance 取 `-count=1` 一轮的数）；`validate.py --all --strict` + `docs_sync.py` 全绿；黄金全部手写，`WE_UPDATE_GOLDEN` 未用。

**订正（两处，逐条记）**：

1. **design D10-4 的报文字面是错的**：原文写 `__we_task_fail("integer overflow")`，而 as-built 是 `<型名> <算子> overflow`。这不是新裁决——D2 的 2026-09-10 裁定已让通用 `integer overflow` 退役并要求陷阱消息指名操作，D10-4 那句与它自己的前一节抵触。D10-4 已加落地对账块（详见 design）。
2. **D10-4 的「修：逐宽界检查 icmp」只说了一件事，as-built 是三件**：替换式守卫（而非叠加）、三个守卫面的分层（`<<` 三枚全留 / `/` 只查商 / `% & | ^ >>` 零枚）、`~` 的掩码修正。逐条入 design D10-4 对账块。

**披露（三条）**：

1. **`UInt64` 全宽算术仍有缺陷**：`max + 1` 静默回绕成 0，合法精确和 2^63 被误报 `Int64 add overflow`。全宽无值域检查可作判据（和越过 2^64-1 会回绕成界内的值，内建的符号位不表示进位）——不在本任务修，已登记 roadmap follow-up #22（英文/中文两份）。
2. ~~**源码不可达面**（与前序任务同界，非本枚引入）：值位置 if/match/块表达式绑定到 `let`、元组值绑定、`match` 表达式绑定、`?` 解包、任务体内 `let`——两种二进制上均停在 M9b 边界。`TestNarrowValueFormTakesTheAnnotation` 与 `TestNarrowDomainPrefersTheSideThatHasAWidth` 因此钉的是**发射器契约**而非语料能拼出的程序，注释在码中写明。~~ **订正（T12 收口探针，2026-09-11 用户裁定，原文保留）**：五面中**四面不成立**。「值位置 if/match/块表达式绑定到 `let`」「元组值绑定」「`match` 表达式绑定」三面的真实边界是**读入 `String` 位**而非绑定本身——绑定可跑，数值消费（比较、实参、窄算术）可跑，`"${k}"` / `"${k+1}"` / `"${box}"` 才停；「任务体内 `let`」**完全可跑**。仅「`?` 解包」如原述（T9-4 裁定 2 的 fn 体面；本枚列的是它之外的第二种上下文）。**探针**：`narrowform`（`let n: Int8 = if a>0 {2i8} else {1i8}` 两枚 + `let y: Int8 = n + m` + `if y == 5i8`）**check 0 / run 0 / 打印 `five`**——正是 `TestNarrowValueFormTakesTheAnnotation` 镜像的形状，故该测试钉的是**可达**契约而非发射器契约（其注释与 `internal/codegen/narrow_test.go` 已同批订正）；`narrowformovf`（同形 + `127i8`）run 1 抛 `Int8 add overflow`，即该测试断言的那条陷阱；`iflet`/`matchbind`/`tupwhole` → 70（边界面本身）、`ifcmp` run 0、`tasklet` run 0。**根因**：`valueKind`（`codegen.go:9068-9118`）无 `*ast.If`/`*ast.Match`/`*ast.BlockExpr` 臂 → `:9117 return skNone` → `emitHole`（:9300）的 `skNone` 判据在 `String` 位停。**缺口本身已并进 T13 边界词退役对账表**（design D12 的 T13 对账项）。**不可改的一处**：`ca268e7` 已推送的 `Not-tested:` trailer 携带同款旧措辞——提交信息不能改写，此订正即其披露。
3. **既有陷阱消息的可见变更**：`<<`/`>>` 在窄型上的溢出报文由 `Int64 shift overflow` 变为 `UInt32 shift overflow` 形。既有黄金无一钉窄型移位（`run-shift-amount-panic` / `run-shift-value-panic` 都是 i64 域，逐字不变），故零回归；新黄金 `run-narrow-shift-overflow-panic` 把变更本身钉住。

### T12 M15 基准套件重锚（2026-09-11）

**主题**：M15（`7b7992c`）按本变更落地前的运行面写下陷阱校准表与运行面预披露；B1a 的 T1–T11 把那张表上的四个停词逐一拆掉，本枚把套件的两面——机器面（`traps_test.go` 的桶断言 + 黑盒电池的 test-malformed 载体）与文献面（两枚 traps.md 的括注、`docs/benchmarks.md` 双语的 run-face anchoring 段）——重锚到**今日真二进制**的事实上。

**① 四校准例对账（逐枚真机定形，收口时无剩余枚——与任务书注记相符）**

| 校准例 | 去向 | 落地点 | 对账证据 |
| --- | --- | --- | --- |
| `control-03 concatenation-boundary` | → `concatenation-latent`，桶 boundary → latent | T4 `b55d208` | traps_test.go:95 `BucketLatent`，突变形 `return s + "x"`——编译运行、错值直达参考测试 |
| `control-03 string-equality-decline` | → `string-equality-latent`，桶 test-malformed → latent | T4 `b55d208` | traps_test.go:98，突变形 `if s == "mirror"` |
| `control-04 modulo-decline` | **陷阱消亡**（`%` 发射 ⇒ 突变后的参考解是干净程序，无捕获面） | T3 `05820e0` | traps_test.go:123 退役注记 + traps.md 条目同删 |
| `control-04 mutate-let-param-decline` | **陷阱消亡**（let/参数重赋值发射行为正确的等价形） | T2-a `2eae49c` | traps_test.go:132 退役注记 + traps.md 条目同删 |

**计数订正（任务书 → as-built）**：任务书 ① 的验证行写「64→62」并注「T4 只换桶不改枚数」。**后半句是错的**——四枚 git 快照逐枚数出：`7b7992c` = 64 → T2-a `2eae49c` = 63（删 mutate-let-param）→ T3 `05820e0` = 62（删 modulo-decline）→ T4 `b55d208` = **63**。T4 的 diff（`git diff 05820e0 b55d208 -- internal/benchmarks/traps_test.go`）是三行而非两行：两枚换桶**之外**新增 `control-03 string-mutable-binding-decline`（`BucketTestMalformed`）——它顶上 `string-equality-decline` 转 latent 后空出的 test-malformed 位，正是 D11 表第五行「T4 裁定期识别、本表写成时尚未列入」的那一枚。T9-4 把该枚改名 `string-mutable-binding-latent` 并转 `BucketLatent`（`git diff 7c8ebce~1 7c8ebce` 同样是一行改名），**不改枚数**。as-built 因此是 **63**（Latent 26 + Rejected 37，无 Boundary、无 TestMalformed 表项），覆盖 18 枚任务全数（尾部「每枚种子任务至少携带一校准陷阱」断言）。

**支配缺陷的披露兑现（errpath-02 traps.md 括注的对象）**：T1 记录的红证据（非支配 IR → `Instruction does not dominate all uses`，以及常数遮蔽的静默错值形）与三枚回归黄金（`run-frame-arm-let-shadow` / `run-frame-arm-payload-shadow` / `run-frame-scope-task-capture`）已钉住根因；本枚补的是**今日构建上的三枚复核探针**，与 traps.md 新注记逐句对应（`/tmp/dom`，真二进制 `/tmp/we-probe`）：`probe1` = match 臂赋值先于 `scope timeout`（errpath-02 触发序）、`probe2` = 同一 var 的臂赋值**跨 scope 两侧**（环境须双向穿过块边界）、`probe3` = ctxpass-02 的披露形（receive-with-match 放在 scope **之前**）。三枚 check 0 / build 0 / run 0，输出 `dom1 1007`（7 + Err 的 1000）、`dom2 1025`（20 + 5 + 1000）、`dom3 1041`（41 + 1000），值逐一正确。errpath-02 参考解本身仍把首个 match 放在 scope 之后——**该次序现已非必需**，新注记把这一点写明。

**test-malformed 载体链（对账项，非待办）**：`control-04` modulo（亡于 T3）→ `control-03` 字符串相等（亡于 T4）→ `control-03` 可变 String 绑定（亡于 T9-4）→ **今日载体 = `control-03` 的 `echo` 泛型形**（`blackbox_test.go:35-51`，`pub fn echo<T>(s: T) -> T`）。该桶因此锚在**工具链的最后一道停词**上而非语义突变上；注释块把整条链在案。

**② 四文件落地（面兑现）**

1. **`errpath-02/traps.md`** 末条 implementation note 重写：由「绕行注记」改为「缺陷已修注记」（缺陷在案、误判形状在案、`roadmap follow-up #16` 由 `codegen-mono` T1 修、参考解次序非必需）。
2. **`ctxpass-02/traps.md`** 第三条尾注同面更新（receive-with-match 先于 scope 的形曾被支配缺陷误判为编译器过 fault，现已修，并指回 errpath-02 的注记）。
3. **`docs/benchmarks.md:50`「Run-face anchoring」段重写**（英文权威面）：原段的三条陈朽句（sum 仅经原语物化、sum/同步型参数不入可跑槽、字符串拼接是诚实边界、函数体恰好一个尾 return）换成 as-built 事实，并把**仍关死的五面**列成显式清单。`docs/benchmarks.zh.md:50` 逐句镜像。

**逐条真机探针（新段每一主张皆有探针，`/tmp/probe/*`，真二进制 `/tmp/we-probe`）**：

| 主张 | 探针 | 判定 |
| --- | --- | --- |
| sum 可构造/返回/match | `positive`：`match` 自有 sum → `sumc 9` | check 0 / run 0 |
| sum 型参数入可跑槽 | `sumparam`：`fn amount(a: Amount) -> Int64` → `sumparam 42 0` | check 0 / run 0 |
| 同步型参数入可跑槽 | `syncparam`：`fn drain(ch: conc.Channel<Int64>)` → `syncparam 9` | check 0 / run 0 |
| 数值返回任意 checked 宽度、陷阱点名宽度 | `i16ovf`：`sum(32767i16, 1i16)` → `error: Panicked: Int16 add overflow` | check 0 / run 1 |
| `String` 相等与拼接 | `positive`：`streq true concat ab!` | check 0 / run 0 |
| 记录方法/元组/新类型/List 迭代/闭包与 fn 值/顶层绑定 | `positive`：`method 42 field 21` / `tuple 7` / `newtype big small` / `list 6` / `fnval 42` / `%`·`/` → `mod 1 div 3` | check 0 / run 0 |
| 任意深度 return，不限尾位 | `depth`：`for` 体内 `return x` → `depth 6` | check 0 / run 0 |
| 关死：泛型函数 | `generic` | check 0 / run 70（`bndGenericFns` 原词） |
| 关死：普通 `scope { … }` 的值形 | `scopeval` | check 0 / run 70 |
| 关死：main 体内的 `?` 解包 | `trymain` | check 0 / run 70 |
| 关死：整只元组值绑定读入 `String` 位 | `tupwhole`（`let box = (3, 4)` + `"${box}"`；同一值的**解构**读入 `String` 位可跑——`tupdestr` → `7 seven`） | check 0 / run 70 |
| 关死：值位 `if`/`match` 绑定读入 `String` 位 | `iflet` / `matchbind` | check 0 / run 70 |
| 运行期规则未变：main 纤程在自家 timeout scope 内 `receive` | `mainrecv` | check 0 / run 70（`we: deadlock: 1 tasks parked with no wake source`） |

**验证**：`docs_sync.py` → `OK: 33 document pair(s) aligned`；`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`；`go test -count=1 ./internal/benchmarks/` → ok（`TestTrapCalibration` 全绿，63 枚逐枚过真二进制判定）；`gofmt -l` 空、`go vet ./...` 干净。文档面无 Go 侧消费者（`grep` 确认无测试读 `docs/benchmarks*.md`），故套件自身的红绿状态不与文献面耦合。

**披露（两条）**：

1. **`T11-3` 记录第 2 条「源码不可达面」措辞过宽**（钉在案，本枚不改历史记录）：该条把五面并列为「停在 M9b 边界」，但其中三面（值位 if/match/块表达式绑定、整只元组值绑定、`match` 表达式绑定）的真实边界是**读入 `String` 位**，不是绑定本身。本枚探针作证：`narrowform`（`let n: Int8 = if a>0 {2i8} else {1i8}` 两枚 + `let y: Int8 = n + m` + `if y == 5i8`）**check 0 / run 0 / 打印 `five`**——正是 `TestNarrowValueFormTakesTheAnnotation` 所镜像的形状，故该测试（及其注释「pins the emitter's contract rather than a program the corpus can spell」）钉的是**可达**契约；`narrowformovf`（同形 + `127i8`）check 0 / run 1 抛 `Int8 add overflow`，即该测试断言的那条陷阱；`iflet` / `matchbind` / `tupwhole` 则给出边界面本身（`"${k}"`、`"${m}"`、`"${box}"` → 70），而 `ifcmp`（`if k == 10`）run 0 → `n 1`。另：该条列的「任务体内 `let`」在本 build 上**可跑**（`tasklet` → `a 1`）。根因是 `valueKind`（`codegen.go:9068-9118`）无 `*ast.If`/`*ast.Match`/`*ast.BlockExpr` 臂，落到 `return skNone`，故绑定面可用而 `emitHole` 的 `skNone` 判据在 `String` 位停。**订正（用户裁定 2026-09-11）**：三处同批改——`proposal.md` T11-3 披露②、`design.md` D10 同句、`narrow_test.go:325` 的码内注释；三处历史原文一律加删除线保留后附订正，不改写。`ca268e7` 已推送的 `Not-tested:` trailer 携带同款旧措辞，提交信息不可改写，此订正即其披露。**缺口本身并进 T13 边界词退役对账表**（design D12 的 T13 对账项 + tasks.md T13 对账行），连同「把可达形状落成黄金」的要求——`narrow_test.go` 的注释今日只能引探针，无夹具可引。
2. **任务书 ① 的计数注记订正为 63**（见上「计数订正」），D11 与 D13 同步加落地对账块。

### T13 边界词退役对账 + conformance（2026-09-11）

**主题**：B1a 的收口任务。任务书的 ① 以 design D0 的验收句为纲（「停点集合恰为 `bndGenericFns`」），② 要求把新增黄金按拓宽面逐面落盘。**两项预测被实现推翻**（D0 的验收句、D12 对 `build-bnd-m6b-main-body` 的翻绿预测），用户裁定 1 把三锚保留、退役改挂 B1b；用户裁定 2 让本任务带一处真码改——`valueKind` 的值位控制形补臂。三提交：`5f8ac25`（T13-1，**修正自 `7834e29`**，见披露 6）/ `5ae9208`（T13-2）/ `c0ca3bf`（T13-3）。

**① 词表终态 grep（按裁定 1 订正为 5 词，非任务书写的 1 词）**

`grep -c` 逐词（`internal/` 全域 .go + .json）：`bndMainBody` 33 / `bndGenericFns` 19 / `bndFnBody` 12 / `bndTaskBody` 7 / `bndTopLets` 2。**5 词全在，含定义、辅助函数与全部生产点**；`bndErrPayload`/`bndCallbackBody`/`bndAssertEqDomain` 三词已随各自任务删净。生产点普查（非测试文件、排除定义行）：`e.bnd(` **226** + `bndFn()` 39 + `bndGeneric()` 31 + `bndMain()` 15 + `bndTask()` 2 + `bndTopLets` 直构 1（`codegen.go:6041`）= **314 处**。词表字节与这批生产点本任务**零改动**。

**①-对账表：23 枚 exit-70 黄金逐枚归词（本任务实测，非计划值）**

| 停点词 | 枚 | 黄金 |
| --- | --- | --- |
| `bndMainBody` | **12** | `build-bnd-acute-float-payload` / `-acute-gc-payload` / `-acute-lazy` / `-acute-user-iterable` / `-list-abi` / `-list-elem` / `-list-iterable` / `-m6b-main-body` / `-value-form-string-arm` / `-value-form-emission-binding` / `-value-form-shadowed-binding` / `-var-unannotated` |
| `bndGenericFns` | 2 | `test-fn-generic-bnd` / `check-assertequal-residual-bnd` |
| `bndFnBody` | 1 | `build-bnd-prim-return` |
| `bndTaskBody` | **0** | —（生产点 2 处，语料无一枚到达） |
| `bndTopLets` | **0** | —（生产点 1 处 `:6041`，语料无一枚到达） |
| 非 codegen 词 | 8 | single-file ×4（`build-single-file`/`run-single-file`/`build-bnd-test-module`/`build-bnd-test-fns-first`，follow-up #6）、std ×2（`check-bnd-std-member`/`check-stdio-shadow-let`，ch15）、ch21 ×1（`build-library`）、arity ×1（`check-bnd-method-arity`，follow-up #5）——**D12 已列为非停点零触碰** |

**这是一处新发现**：D0 表列 `bndTaskBody` 与 `bndTopLets` 为 B1a 退役项，但两词今日**无任何黄金锚定**——它们由生产点守着，不是由冻结黄金守着。故「三枚体锚」的说法精确化应为：**锚在黄金上的是 `bndMainBody` 与 `bndFnBody`，`bndTaskBody` 与 `bndTopLets` 只有生产点**。保留五词的裁定因此对 `bndMainBody`/`bndFnBody` 有黄金级证据、对另两词只有代码级证据。

**①-D12 九枚对账表（逐枚实测，兑现 8 / 未兑现 1）**

| D12 拟项 | as-built | 落地 |
| --- | --- | --- |
| `build-bnd-new-forms`（翻绿改锚） | **名系已终**：经 `build-bnd-string-assign` 落到 **`build-bnd-var-unannotated`**（今日 exit 70——无标注 `var s = "a"`），正向面拆出 **`run-main-early-return`**（0）/ **`run-main-early-return-err`**（1） | T2 / T4 / T9-4 |
| `build-bnd-m6b-main-body` | **未兑现**——今日仍 exit 70 + `bndMainBody`（源 `let v = f.fetch()?` 落在仍关死的 main 体 `?` 面；T12 探针 `trymain` 实证） | 无（见 D0 订正） |
| `build-ch10-method-call-boundary` | 保名，期望改 0 | T5 |
| `build-bnd-err-payload` | **`build-err-payload-eager`**，源不变，期望改 0 | T9-2 |
| `build-bnd-toplet` | 保名，期望改 0（`build-bnd-toplet-carrier` 同） | T8-1 / T8-2A |
| `test-fn-body-bnd` | **不存在**——T7-2 删并取代为 **`test-fn-body-for-list`**（0） | T7-2 |
| `test-asserteq-domain-bnd` | **`test-asserteq-domain-eq`**（0） | T10-2a |
| `check-assertequal-domain-boundary` | **`check-assertequal-domain-eq`**（0）；残余停点新钉 **`check-assertequal-residual-bnd`**（70，`bndGenericFns`） | T10-1 |
| `build-bnd-for-string`（D12 未列，T4 期退役） | **不存在**——`for` over String 已发射，取代为 **`run-for-string`**（0） | T4 |

**② 新增黄金总量落盘：逐拓宽面清点表（T1–T13 as-built）**

计数：语料 **804 → 810 枚**（T13 增 6：`5f8ac25` 一枚 + `5ae9208` 五枚）；`run-*` **128 → 131**；`build-*` 37 / `build-bnd-*` 21（其中 **15 枚 70、6 枚 0**）/ `check-*` 499 / `test-*` 77。`internal/codegen` 单测 **319 → 321**；`internal/typecheck` 76 不动。

| 拓宽面 | 任务 | 正面黄金 | 负/边界面 |
| --- | --- | --- | --- |
| env 作用域化（D1） | T1 | `run-frame-arm-payload-shadow` / `-arm-let-shadow` / `-scope-task-capture` | 无 exit-70 钉——红面是 **clang 拒绝**（`does not dominate all uses`）与静默错值，钉在单测（`envframe_test.go` 5 例）+ 探针；最近黄金面是 `run-bare-block-scope` |
| 语句集完备（D2） | T2 / T2-b | `run-for-range` / `run-for-nested` / `run-fn-for-range` / `run-main-early-return` / `-early-return-err` / `run-bare-block-scope` | 原两枚负例（`build-bnd-new-forms`/`build-bnd-for-string`）**皆已退役**，名系转正 |
| 赋值面放宽 | T2-a / T11-5 | `run-let-reassign` / `run-param-reassign` / `run-bare-block-scope` | `build-bnd-var-unannotated`（名系继承）；第五缺陷（逐迭代 alloca）**IR-only**：`TestSlotHoistedToEntry` |
| for-in 源归属 | T2-b / T4 / T7-2 | `run-for-range` / `run-for-string` / `run-for-list` | `build-bnd-list-elem` / `-list-abi` / `-list-iterable` |
| 表达式值形（D2/D3 交） | T3 / T13-1 | `run-value-forms` / **`run-value-form-string-face`** / `run-unary-ops` / `run-div-mod-values` / `run-rune-binding` / `run-call-in-operand` / `run-bitwise-ops` / `run-logic-short-circuit` / `run-float-values` / `run-scope-value-order` | panic 面 6 枚（exit 1）+ **T13 新钉 3 枚**（`build-bnd-value-form-string-arm` / `-emission-binding` / `-shadowed-binding`） |
| 字符串全链（D3） | T4 / T9-4 | `run-string-concat` / `-equality` / `-interp` / `run-for-string` / `run-string-reassign` / `-mutable-binding` / `-binding-faces` | `build-bnd-var-unannotated`；`build-bnd-string-assign` 退役。**T4 自身收口时无活着的 exit-70 负例**——活的字符串面负例是 T9-4 才到的 |
| 记录/元组/newtype/unit + 方法表（D4） | T5-1 / T5-2 | `run-record-read-update` / `run-method-dispatch` / `run-newtype-erasure` / `run-tuple-value`；build 面翻绿 `build-ch10-method-call-boundary` / `build-record-erasure` / `build-ch10-erasure` | `build-bnd-m6b-main-body`（`f.fetch()?`）；`run-method-dispatch` 另钉 ch6 隐式尾 return |
| scope resource（D4） | T5-4 | `run-ch13-scope-res-order` / `-early-exit` / `-opaque` | 无（check 面回归钉 `check-ch13-scope-green`）；负面只在单测（`scope_res_test.go` 8 枚） |
| 闭包全捕获 + fn 值（D5） | T6 | `run-closure-scalar-capture` / `-record-capture` / `-string-capture` / `-nested` / `run-fn-value-call` / `-higher-order` / **`run-closure-gc-capture-crossing`** | 无。`bndCallbackBody` 词已删；**M6（fn 型参数按两字过界）维持 IR 钉**——clang 21.1.8 对多传实参的直接调用既不报错也不走样 |
| List 载体 + 字面量（D6） | T7-1 / T7-2 | `run-for-list` / `run-list-domains` / `run-list-records` / `test-fn-body-for-list` | `build-bnd-list-elem` / `-list-abi` / `-list-iterable`。**T7-1（`runtime/c/list.c`）无黄金**——只有 `runtime/list_test.go` 载体 |
| 6 急性组合子内建面（D6） | T7-3 | `run-acute-combinators` / `-callbacks` / `-domains` / `-empty` | `build-bnd-acute-float-payload` / `-gc-payload` / `-lazy` / `-user-iterable`。`-lazy` 是 **B1b vtable** 那五枚惰性组合子的面 |
| 顶层 let + 模块 init + 全局根（D7） | T8-1 / T8-2A / T8-2B-0 / T8-2B | `run-toplet-cross-module` / `-load-order` / `-domains` / `-diamond` / `-shadowed-qualifier` / `-string` / `-string-cross-module` / `-string-concat` / `-gc-record-root` / `-gc-list-root` / `-gc-cross-module` / `run-gc-root-discharge`；panic 面 `run-toplet-init-panic` | 无活着的——`build-bnd-toplet` 与 `-toplet-carrier` **皆翻 exit 0**；D12 拟的两枚改名（`build-toplet-init`/`build-toplet-string`）仍待，故 T8 无 exit-70 钉 |
| sum 物化泛化 + Err 载荷（D8） | T9-1 / T9-2 | `run-sum-ctor-expression` / `run-fused-err-return` / `run-err-payload-two-words` / `build-err-payload-eager` / `run-err` / `run-main-early-return-err` | `build-bnd-prim-return`（属 T9-3）。**T9-1（三槽布局）构造上无语义黄金**——任务记为形钉 + 零漂移，语义证明推给 T9-2；由 `TestM10bFnAbiMatrix` 钉 |
| 参数槽放宽 + String 两字槽（D8） | T9-3 / T9-4 | `run-sum-param-match` / `run-prim-param` / `run-string-reassign` / `-mutable-binding` / `-binding-faces` | `build-bnd-prim-return` / `build-bnd-var-unannotated`。`run-string-binding-faces` 是**电池产物**而非计划项 |
| assertEqual Eq 域（D9） | T10-1 / T10-2a / T10-2c / T10-3 | test 面 6 枚 0 + check 面 3 枚 0 | test 面 13 枚 exit 1（告失败报文钉）+ `check-assertequal-residual-bnd`（70）+ `check-assertequal-mixed-e0501`（1）。族 6→19 `test-asserteq-*`、4→5 `check-assertequal-*` |
| 窄整型逐宽溢出（D10/D11） | T11-3 | `run-narrow-values` / **`run-narrow-value-form`** | panic 面 7 枚 exit 1 |
| 陷阱消息指名形 | T3（09-10 裁定）/ T11-3 | —（报文钉在 panic 黄金 stderr + `expr_test.go`） | `run-add-overflow-panic` / `run-div-overflow-panic` / 全 `run-narrow-*-panic` |
| M15 基准套件重锚 | T12 | — | —（**无黄金**：证据是 `traps_test.go` 桶断言 + `docs/benchmarks.md` 探针） |
| **边界词退役 + 值位控制形** | **T13-1 / T13-2** | `run-value-form-string-face` / `run-narrow-value-form` / `run-closure-gc-capture-crossing` | **3 枚新边界面**（`build-bnd-value-form-string-arm` / `-emission-binding` / `-shadowed-binding`） |

**② 的负例设计订正（对账，非待办）**：任务书原拟的两枚负例之一是「`let u = if c {1}`（无 else）后 `"${u}"`」。**该形到不了 codegen**——无 else 的 `if` 在检查期即 E0202。等价的保守面因此改钉 `build-bnd-value-form-emission-binding`（块局部绑定）与 `-shadowed-binding`（外层同名遮蔽），两枚都是发射器真能到达、分类器有意拒绝的形。

**T13 的码改（裁定 2）：`valueKind` 补三臂 + 一处守卫**

`valueKind`（`codegen.go`）新增 `*ast.If` / `*ast.Match` / `*ast.BlockExpr` 三臂与 `blockKind` / `joinKind` 两辅助。界有三条：join 要求各分支 kind **一致且非 `skStr`**（`valueForm.put` 只接受 `ckI64`——值位控制形发射不出 String，故排除 skStr 让所有以 skStr 为界的消费者逐字节不变）；match 无臂、`If` 无 else 一律 skNone；**块在尾项之前若有任何 `*ast.Binding` 项则拒绝**（`blockKind`——见下）。

**连带行为变更（披露 3）**：顶层 `let k = if …`（`:6038` 亦调 `valueKind`）与列表元素面（`elemFaceOfExpr`）随之可用。两者是同一分类器的自然延伸，**登记为行为变更而非附带修复**；`bndTopLets` 的生产点（`:6041`）在补臂后收窄但未死，故不删。

**突变电池（6 枚，单测层 + 黄金层各记判决）**

| 突变 | 内容 | 单测层 | 黄金层 |
| --- | --- | --- | --- |
| A | `*ast.BlockExpr` 臂 `return e.blockKind(...)` → `return skI64` | 红 | **红**（2 枚新的 `build-bnd-*` 翻 0）——且探针显示发射端本可正确处理块局部绑定（打印正确 `8`），即该界是**保守**的 |
| B | `joinKind` 去掉 `skStr` 排除 | 绿 | 绿——**两层存活**：String 分支由 `valueForm.put` 自己的 `ckI64` 规则独立停住，分类器的排除对该形**冗余**（在案，不作为缺陷） |
| C | M2：String 捕获置 trace 位 | 红 | **红**（`run-closure-gc-capture-crossing` 崩溃 exit 1） |
| D | M8：载体描述符 trace 代码指针 | 红 | **红**（同上） |
| E | `blockKind` 的绑定守卫 | 红 | **红**（`build-bnd-value-form-shadowed-binding`） |
| F | —（锚点与替换串相同，**空操作**，不作为结果） | — | — |

**T6 移交的验证兑现**：T6 记「M2/M6/M8 单测层捕获、黄金层空缺」。本枚的 `run-closure-gc-capture-crossing` **把 M2 与 M8 的黄金层空缺补齐**（突变 C/D 在黄金层红）。M6 维持原状——clang 21.1.8 对多传实参的直接调用既不报错也不走样，无 clang 侧守门，故维持 IR 钉（`narrow_test.go` 与 `m10b` 面的既有钉），在对账表注明。

**② 的跨收集黄金（T6 移交要求的落盘）**：`run-closure-gc-capture-crossing` = 捕获 String 的闭包（`|v| v + s.byteLength()`）经 `conc.Mutex.update` 跨界调用，程序先分配 **60000 个 `Cell` 记录**把 `allocated_since` 推过 `GC_THRESHOLD`（`gc.c:250`，1 << 20）触发收集，再读捕获物。stdout 逐字节定 `payload:7:1799970000`，exit 0。**两处订正**：(a) 最初的循环设计是 `buf = buf + buf` 增长串，**推不动 GC**——`str_alloc` 走 malloc（`str.c:78`），`allocated_since` 只统计 `__we_alloc` 的切块（`gc.c:308`），故改记录分配才越阈；(b) 捕获字面量须避开首字节低位为 1 的值——`blk_marked(p)` 读 `*(u64*)p & 1`（`gc.c:30`），`"captured"` 的 `'c'` = 0x63 低位为 1 会让收集器误判已标记而跳过写入、突变静默存活，故改用 `"payload"`（`'p'` = 0x70 低位 0），突变 C/D 随即在黄金层红。

**验证**：`go build ./...` 干净；`go vet ./...` 干净；`gofmt -l .` 空；`go test -count=1 ./...` 全绿（codegen 321 / conformance 111.5s / benchmarks 29.0s / 其余 10 包 ok，无 skip、无 fail）；`docs_sync.py` → `OK: 33 document pair(s) aligned`；`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`；`git diff --check` 空。黄金账数字取自 `-count=1` 一轮。`WE_UPDATE_GOLDEN=1` 全程未使用（黄金全部手写）。

**披露（六条）**：

1. **两处设计期预测被实现推翻**：D0 的验收句（「停点集合恰为 `bndGenericFns`」——实为 **5 词**）与 D12 对 `build-bnd-m6b-main-body` 的翻绿预测（今日仍 70）。两处均以**删除线保留原文 + 订正**在案（`design.md` D0 / D12），不静默修正。
2. **10 枚新增边界面**（D12 未列、T7-3/T9-3/T9-4 拓宽中新钉）逐枚入本记录的 ①-对账表；另 **`bndTaskBody` 与 `bndTopLets` 零黄金锚定**（只有生产点），这是对「三锚有黄金级证据」的精确化订正。
3. **词面括注夸大**：`bndMainBody` 的词面括注自称覆盖「scalars, strings, records, primitives, io, task/scope/select, ?, match, while/if, defer, one tail return」，而其中 **10 处实际仍停**（`?` 对用户方法返回、`primitives` 的返回位、`strings` 的 List 元素面等）。本任务**选择不改词面字节**（改则动 15 枚黄金 stderr + `codegen_test.go` 字面量），只登记为披露：这些词对用户本就是内部里程碑词汇，操作性消息（"not implemented in this reference build yet"）是真的，失准在细节。
4. **排除 skStr 的界**：`if c {"a"} else {"b"}` 读入 String 位仍停（`build-bnd-value-form-string-arm`）。这是**发射能力之界**而非分类器之界——`valueForm.put` 的结果槽是数值集专属。突变 B 证明分类器的排除对该形冗余。
5. **`blockKind` 的绑定守卫是被一次错误编译逼出来的**：见披露 6。
6. **T13-1 的初版有紧急缺陷，已修正（`7834e29` → `5f8ac25`）**。初版 `blockKind` 按**外层**帧分类块尾——`emitArmBlock` 在任何人提问之前已 pop 掉块的帧，故分类器看到的是外部环境。后果：外部 `let b = true` 下 `let k = { let b: Int64 = 5  b }` 走 bool 转换器渲染 `5` → **静默打印 `true`，exit 0**（真机复现，非推理）。修正 = 尾项之前有 `*ast.Binding` 即拒绝 + `valueKind` doc 重写 + 修正提交的 message 披露该稿错渲染此形、且初版 doc 里「猜错只付一枚边界」的自陈是错的。守卫由 `TestValueFormDeclinesABlockThatBinds` 与 `build-bnd-value-form-shadowed-binding` 两层钉住（突变 E 两层齐红）。

### T14 验证阶梯（2026-09-11）

六面逐面取证，全部在 HEAD 工作树上、以 `-count=1` 一轮取数。**本任务是纯验证枚：零码改、零黄金改、零文档改**（发现的处置见下）。

**六面输出**

| # | 面 | 命令 | 输出 |
| --- | --- | --- | --- |
| 1 | 格式 | `gofmt -l .` | 空（无文件待格式化） |
| 2 | 静态 | `go vet ./...` | exit 0，无输出 |
| 3 | 测试 | `go test -count=1 ./...` | **13 包 ok、2 包无测试文件、0 fail、0 skip**；codegen 0.157s / conformance 121.994s / benchmarks 35.067s / runtime 19.293s / cli 3.210s / typecheck 0.144s / 其余 6 包 sub-0.05s |
| 4 | 变更工件 | `validate.py --all --strict` | `OK: 1 change(s) valid; registry clean; mode=strict` |
| 5 | 双语对齐 | `docs_sync.py` | `OK: 33 document pair(s) aligned` |
| 6 | 真机电池 | `/tmp/we-t14/battery.py`（15 探针） | 见下表，15/15 符合预期 |

**包数订正（对账）**：任务书写「全绿（14 包）」，**as-built 是 15 包**——`go list ./...` 出 15 个包，其中 13 个有测试文件（`[no test files]` 的两枚是 `cmd/we` 与 `internal/ast`）。「14」在 T1–T13 任一阶段的实测里都对不上，属任务书的计数笔误，不改 task 文本字节，在此订正。

**D13-4（traps_test 重锚）**：`TestTrapCalibration` **63 个子测试全 PASS**。**溯源订正**：该测试在 **`internal/benchmarks`**（`traps_test.go:55`）而不在 `internal/conformance`——`-run TestTrapCalibration` 在 conformance 包上回 `[no tests to run]`（此路探过一次，作废）。v 面 PASS 行 64 = 1 父测试 + 63 子测试，与 D13 记的 63 一致。

**D13-5（支配缺陷回归钉）**：`b9-dominance` 探针即 errpath-02 形（match 臂给外层 `total` 赋值先于 `scope` 头，再 match 同一变量）——真机 `we run` exit 0 打印 `dom 1007`；`we build . --verbose` → `we: built build/demo`；对产物 IR 直接跑 `clang-21 -S -emit-llvm` → **exit 0、无 error、IR 文本无 "does not dominate"**。**订正**：上一轮工作笔记写的「零诊断」不确——clang 出 1 条 **warning**（`-Woverride-module`: "overriding the module target triple with x86_64-pc-linux-gnu"），是驱动器对 target triple 的常规告警，与 IR 无关；as-built 准确表述是「exit 0、1 warning、0 error」。

**真机电池（15 探针，build→clang→run 走真二进制，非 conformance 的进程内入口）**

| 面 | 探针 | 判决 |
| --- | --- | --- |
| 字符串 | `b1-string` | run 0 / `concat abcd` `eq-ok` `reassign xy` `len 4` `rune h/e/y` `valueform 10` `block 8` |
| 记录方法 | `b2-method` | run 0 / `bump 1 2` `hi ann!` `newtype 42` `tuple 3 seven` `field 6` |
| 闭包捕获 | `b3-closure` | run 0 / `higher 11 6`、`gc payload:7:1799970000`（含跨 GC 读捕获物） |
| List 迭代 | `b4-list` | run 0 / `for 10` `records 3` `comb 10 4 true true` `reduce-ok` `find-ok` |
| 顶层 let | `b5-toplet` | run 0 / `x=2` `s=top-level` `y=2` |
| sum 载荷 | `b6-sum` | run 0 / `some 3` `none-ok` `circle 12` `rect 12` `err-ok` |
| Eq 断言 | `b7-test` | test 0 / 4 passed 0 failed |
| Eq 断言负例 | `b7-test-fail` | **test 1** / `assertion failed: at Point.y: got 2, want 3` |
| panic | `b8-panic-div` / `-add` / `-narrow` / `-shift` | **run 1** ×4 / `division by zero` · `Int64 add overflow` · `Int8 add overflow` · `Int64 shift overflow` |
| 支配回归钉 | `b9-dominance` | run 0 / `dom 1007` |
| 合成 | `b10-composed` | run 0 / `run sum=50 extra=5 verdict=0 face=small`、`classify err` |

十三个正面探针全绿，两枚负例按设计红（Eq 断言 1 枚、panic 4 枚）。电池源码写在**冻结语料已证过的形状**上，故 15/15 无一是新面——新面全部落在下面的发现里。

**三枚发现（T14 产物）**

**N1 — fn 值调用在操作数/洞位停（诚实边界，无黄金覆盖）**。`let f: fn(Int64) -> Int64 = |v| v + 1` 之后：`let r = f(1)` → 0（`2`）、`var r: Int64 = f(1)` → 0、`if f(1) > 0` → 0、`return f(1)` → 0、`double(f(1))`（`double` 为已定义 fn）→ 0（`4`）；但 `"${f(1)}"` → 70、`let r = f(1) + 1` → 70、`let r = { f(1) }` → 70、match 臂尾位 → 70。**与捕获无关**（无捕获闭包同样停）——界在**位置**（值串位/操作数位），不在捕获。全部 70 都是 `bndMainBody` 词。**黄金覆盖：无**（机器扫描全语料：凡 `let <name>: fn(` 绑定且 `<name>(` 出现在 `${…}` 洞内者，零枚）。故 `docs/benchmarks.md` 的 run-face 段「闭包和函数值……会运行」这句在位置维度上过宽，而语料里没有一枚案例把这条界钉住。

**N2 — 内置函数所产 `Option` 的载荷读入 String 位停（诚实边界，无黄金覆盖）**。`xs.iterator().reduce(…)` / `.find(…)` / `.next()` 之后取载荷：数值消费（`if v == 10`）→ 0，`"${…v…}"` → 70。用户 fn 的 `Option` 返回（`fn pick(n) -> Option<Int64>`）→ 0（`some 3`），字面量 `Some(42i64)` → 0（`lit 42`）。**注解不救**：`let r: Option<Int64> = xs.iterator().reduce(…)` 仍 70，把载荷重绑到带注解的 let 仍 70。**黄金覆盖：无**（同上机器扫描）。这两枚与 N1 同属「词对、面不对」——`bndMainBody` 的消息为真，但它不指名是哪一面。

**N3 — 洞（`${…}`）内容不被类型化，其最强后果未被披露（缺陷类，需裁定）**。裁定③（T4 期）记「parser 建洞 AST、typecheck **不类型化洞**」，是**已记录的故意裁定**；本枚电池揭出的是它的**后果**，而该后果在任何工件里都没有被写过：

- 洞内 `one(true)`（`one` 的参数是 `Int64`）→ **check 0 / build 0 / run 0，打印 `1`**。同一程序把该调用挪出洞外即 **E0501**「mixed types — the argument is Bool, the parameter is Int64; **no coercion is ever inserted**」exit 1。IR 作证：`%v1 = call i64 %v0(i64 1)`——Bool 以 i64 常量 `1` 直接落进实参位，发射器**无签名可比**（它信任检查期已保证类型，而洞内检查期从未看过）。
- **根因（结构级，可复验）**：`grep -rn '\.Holes' --include=*.go .` 的全部命中只在 `internal/parser`（建）与 `internal/codegen`（发），**`internal/typecheck/*.go` 一个都不读**。故洞内不受任何类型规则约束：`"${zzzz}"`、`"${1 + zzzz}"`、`"${zzzz.field}"`、`"${one(1, 2)}"`（元数）、`"${n.nosuch()}"`、`"${1 + true}"` 全部 check 0。
- **范围（不夸大）**：多数洞内错误**恰好**落到 codegen 边界（exit 70）——未定义名、String→Int64、Float→Int64 在洞内都是 70。**滑过去的只有 Bool→Int64 这一形**：`one(1.5)` → 70，`one(s)`（String）→ 70，`one(true)` → 0 且输出 `1`。原因是 Bool-as-i64 本就是这个参考实现的常规寄存器表示，发射器照发，无处可停。
- **归类依据**：按本项目自己在 T4 立下的判据——「IR 干净、clang 不报、exit 0、**输出错值**」即**静默错误**，属缺陷类而非诚实边界类（T4 披露①②正是按此判据说自己是缺陷）。N3 满足该判据的全部四项。
- **处置**：**本枚不动码**。裁定③是已记录的**设计裁定**，推翻它（把洞纳入类型化）或替代它（发射面复核洞内调用的实参域）都是超出验证阶梯的码改，且是用户级取舍。登记为**待裁定项**：三条路——(a) 类型化洞；(b) 洞内调用点按 callee 签名复核实参域（窄修，够用）；(c) 声明「洞内不检查」为接受的参考实现缺口并写进 `docs/`（须显式，不能只留在裁定③的四个字里）。

**披露**

1. **「14 包」是任务书笔误**，as-built 15（13 有测试 + 2 无）。已订正，不改 task 文本字节。
2. **D13-5 的 clang 面无「零诊断」**：exit 0 + 1 warning（`-Woverride-module`）+ 0 error。上一轮笔记的「零诊断」是错的，本记录为准。
3. **D13-4 的测试在 `internal/benchmarks` 而非 `internal/conformance`**；63 子测试数与 D13 一致。
4. **N1/N2 是「词对、面不对」的两枚**：`bndMainBody` 的消息为真，但 `docs/benchmarks.md` 的 run-face 段对「函数值/闭包」的表述在位置维度上过宽，而两形**零黄金覆盖**。是否补两枚边界面黄金，留给 T15 归档时的对账（本枚按「纯验证枚」不动语料）。
5. **N3 是本阶梯唯一的缺陷类发现**，且回溯到一条已记录的裁定（③）。它的可复验入口是一行 grep（typecheck 不读 `.Holes`）+ 一枚三行程序（`"${one(true)}"`）。

## 审查记录（2026-09-11）

**范围**：`7b7992c..0aac57b`（B1a 的 27 枚提交 + 审查期订正 1 枚）。按 `.agents/skills/welang-code-review/SKILL.md` 七条逐条执行。机械面（CJK 扫描、JSON 协议、诊断码、署名 trailer、`refr/`、变更路径清点）由独立子代理出**原始证据**（file:line 与逐字串），判定与复验由审查会话自己做。

### 1 规范符合性 —— 通过（附一枚未决缺陷，见发现表 N3）

- **零规范增量成立**：`git diff 7b7992c..HEAD -- docs/spec/` 为空；`docs/spec/diagnostics.toml` 未触（条目数两端皆 153）；变更目录下无 `specs/`。故「与 spec 增量每条 Requirement/Scenario 一致」在本变更为空真（internals-only 豁免，M0 先例，`validate --strict` 通过即豁免路径生效）。
- **行为面对既有章文的兑现**：D10-4 的窄整型逐宽检查 = ch7 E0502 已批的逐宽 checked 语义；D2 短路 = ch2 已批的逻辑运算符语义；D5 的 `{thunk, env}` 载体 = ch12 闭包章文的发射面选择。三者都不是新行为。
- **新增接受面逐面核过**：T4 的两处新 E0105 发点码已登记（见 check 4）；T13 的 `valueKind` 三臂不返回 `skStr`，而 `valueForm.put`（`codegen.go:4448`）只接受 `ckI64` ⇒ 以 `skStr` 为界的既有路径**结构上不可能改变**（T13 记录已给 20 个调用点的三分类）。
- **N3（未决缺陷）**：规范依据是 ch1 的 "No implicit conversion" 与 ch7 的 "Arguments and returns match their declared types"（`docs/spec/0700-types.md:59-63`），且 22 章无任何章节为洞开豁免。该形在 T4 之前**不可达**（洞内一律停在边界 exit 70），T4 使之可达 ⇒ 属本变更引入的可达面。**处置：登记待裁定（三路），不阻塞归档**——根因是 T4 期已记录的设计裁定③（typecheck 不类型化洞），三条出路（类型化洞 / 发射点按被调签名复核实参域 / 显式声明为可接受缺口）都是用户级取舍，且每条都改检查器或发射器的行为，超出本变更「实现追赶」的边界。

### 2 验证诚实性 —— 通过（一枚工件缺陷已修）

- **提交消息里的计数全对**：对 range 内 27 枚提交逐枚跑 `git ls-tree -r <c> -- internal/conformance/testdata/cases/ | grep -c '\.json$'`，与消息里的「conformance N [→ M] goldens」逐枚相符——744 / 751 / 759 / 766 / 773 / 776 / 776→779 / 779→782 / 782→783 / 783→787 / 787→790 / 790→796 / 796→804，**零偏差**。
- **proposal 的计数链全对**：任务记录里的 20 处「conformance N → M」逐枚相符（701 基线 → 713 → 728 → 731 → 738 → 744 → 745 → 751 → 759 → 766 → 769 → 770 → 773 → 776 → 779 → 782 → 783 → 787 → 790 → 796 → 804 → 810）。两处乍看「对不上」的点经查是**任务作用域基线**：T5/T6 共用一个提交 `b3e2735`（真实树 731 → 744），T5 记「731 → 738」、T6 记「738 → 744」，与 731 + T5 的 7 枚 + T6 的 6 枚 = 744 精确一致。`internal/codegen` 的单测计数链（105 → … → 319 → 321）与 HEAD 实测 `go test -list` 的 321 一致。
- **「未用 `WE_UPDATE_GOLDEN`」可验**：range 内该串的全部出现 = 1 处提交消息（说的是**未**用它）+ `internal/conformance/{conformance_test.go,runner.go}` 里该环境变量自身的实现；零调用点。
- **突变声明独立复现（抽验）**：取 T13 记录表 C 行「M2：String 捕获置 trace 位 / 单测层红 / 黄金层红（`run-closure-gc-capture-crossing` 崩溃 exit 1）」，对 `codegen.go:3718` 施加单点突变 `capSlot{name: name, n: 2}` → `capSlot{name: name, n: 2, trace: [2]bool{true}}`：单测层 `TestClosureStringCaptureCopiesThePairUntraced` **红**；黄金层 exit 1 + `we: build/demo terminated by signal`——**与记录逐字相符**。还原后 `git diff` 空、两层复绿。
- **工件缺陷（已修）**：`tasks.md` 曾有两份「新增黄金总量落盘」复选框——一份已勾且带对账（含 `valueKind` 缺口项），一份是 T13 之前的**陈旧未勾副本**。归档前置条件要求 tasks 全勾，故该副本是**阻塞性工件缺陷**；逐行 diff 确认陈旧版无独有内容后删除。

### 3 测试先行证据 —— 通过（形态是构造性红 + 探针 + 突变电池，而非 red-first 提交）

- **逐提交核过**：27 枚提交**没有一枚**是「先只加测试」的 red 提交——每枚都把实现、单测与黄金同批落地。故「目标测试先于实现存在」**不能从提交历史读出**，只能从别处取证据。
- **替代证据三路**：(a) **构造性**——各任务首 checkbox 的目标形在开工时必然停在边界（exit 70 + bnd 词），tasks.md 顶部的注句统一钉死此点，测试是「照已测得的红行为写期望」。(b) **探针先行**——每个拓宽面在写黄金期望之前逐枚过真机（T14 电池 15/15 复验；本审查独立复现 M2）。(c) **突变电池**——12（T6）+ 15（T8）+ 14（T10）+ 6（T13）枚单点突变，逐枚断言目标测试转红、还原转绿，留痕在各任务记录。
- **黄金格式**：本变更新增黄金全是 run/build/check/test 面的程序输出期望，人类可读面即 §52 的 `we: …` 形（T14 已实机逐枚核过）；**无一枚新黄金载 JSON Lines 期望**（机器扫描：新 golden 无 `json` / `"severity"` / `"type":"diagnostic"` 键）⇒ §62 协议面在本变更为空真。

### 4 诊断协议稳定 —— 通过（一枚消息纪律偏离，已在 `0aac57b` 修正）

- **JSON 字段零删改**：`git diff --stat 7b7992c..HEAD -- internal/diag` 为空；`internal/cli` 只 `build.go` +8 行（C 源/目标文件清单，无 JSON）；`internal/cli/cli.go` 的 `{"type":"version",…}` 发射点未动；全 range 的增删行对 `json:` / `"severity"` / `"column"` / `"help"` / `"diagnostic"` 零命中。
- **新增诊断码：零枚**。`diagnostics.toml` 未触（153 条两端相同）；range 新增行出现的 18 个码在 base 均已登记；两处新引用的 E0105 已有条目（title `unexpected token`，owner `0200-grammar`，`diagnostics.toml:344-350`）。
- **偏离与修正**：`internal/parser/parser.go:979` 的新消息原为 `"invalid interpolation hole — …"`，未以登记 title 起头，违反 `internal/diag/diag.go:40` 的 MUST。`0aac57b` 改为 `"unexpected token — the ${} region is empty; a hole holds one expression"`（与该文件既有的 house 变体 `unexpected <what> — …` 同形）。同文件另有一枚同类偏离（`a newtype wraps exactly one underlying type — …`）是**既有**的（base 即在），按 check 6「不越界」不动。只钉消息子串的 `internal/parser/str_test.go:121` 无需改。

### 5 单一权威 —— 通过（一枚语言纪律偏离，已在 `0aac57b` 修正）

- **长期事实无滞留**：本变更零 Requirement、零 ADR、零诊断码 ⇒ 变更目录里没有该进 `docs/spec/` 或 ADR 的内容；技术面（D1–D13）本就承载在 design.md。可提升项为零。
- **偏离与修正**：本变更在 `.go` 注释里引入 **25 行中文**（`internal/codegen/codegen.go` 8 行 + 7 个新 `_test.go` 17 行），形态一律是「英文句 + 括号内中文引文」，引的是（中文的）变更工件与规范中译。语言约定（AGENTS.md:39）要求代码注释用英文。**base 证据**：base 全仓 `.go` 只有 3 个文件含中文，其中 2 个是**测试输入数据**（`diag_test.go:74` 的诊断消息样本、`lex_test.go:239/267/283` 的词法夹具），1 个是既有笔误词（`ffi_test.go:40`，本变更已顺手改成英文）——**生产文件注释零中文** ⇒ 该形态是本变更新立的，不是既有风格。`0aac57b` 把 25 行改为英文释义并保留 D-编号/章号指针（可追性不丢）。修正后全仓 `.go` 中文只剩上述两处既有测试数据。
- **流程缺口**：无机械门查注释语言（`validate.py` 查注册表与 Requirement/Scenario 结构，不查注释），故 13 个任务无人拦下——登记为观察 O1。

### 6 红线复核 —— 通过

- **`refr/` 零触碰**：`git diff --name-only 7b7992c..HEAD | grep -c '^refr/'` = 0；每次提交前查 staged，`.githooks/pre-commit` 未触发。
- **无署名 trailer**：27 条消息大小写不敏感扫 `co-authored` / `generated with` / `signed-off` / `anthropic` / `claude` / `noreply` / `🤖` —— **全 0**；尾部只有决策 trailer（`Tested:` / `Not-tested:`）。审查期新增的 `0aac57b` 同规。
- **变更路径清点**：range 内 181 条路径逐条分类。170 条在 `internal/codegen`(28) / `internal/conformance/testdata/cases`(118) / `runtime`(16) / `docs`(4) / `internal/typecheck`(4)。另 11 条在 codegen 之外但属实质范围：`internal/{ast,lex,parser}`（T4 洞的词法/AST/解析支持——洞正是 T4 的面）、`internal/cli/build.go`（C 源/目标清单登记）、`internal/benchmarks`（T12 的 M15 重锚）。`change.yaml` 声明的层 `[compiler, benchmark, docs]` 覆盖全部 181 条 ⇒ **无层外改动**。3 枚改名 + 2 枚删除全在 `testdata/cases/`，逐枚有任务留痕。

### 7 最小可信验证已跑 —— 通过

- **阶梯**（AGENTS.md:66，对照实际 diff）：`go build ./...` 0 / `go vet ./...` 0 / `gofmt -l` 空 / `go test -count=1 ./...`（13 个有测试的包 ok + 2 个无测试文件的包，0 失败）/ 全量黄金 810 枚 `ok 169.176s`（`0aac57b` 后重跑）/ `validate.py --all --strict` OK / `docs_sync.py` 33 对 OK。T14 收口轮同阶梯全绿，计数取自 `-count=1` 单轮。
- **归档复验**：见本文末「归档复验」段。

### 发现汇总

| # | 类 | 内容 | 处置 |
| --- | --- | --- | --- |
| N3 | **缺陷类·待裁定** | 洞 `${…}` 内容不被类型化：`"${one(true)}"`（Bool 实参进 `Int64` 参）check 0 / build 0 / run 0 打印 `1`，洞外同形即 E0501 | 登记 roadmap follow-up **#23**；三条出路待用户裁定；不阻塞归档 |
| N1 | 诚实边界 + 披露过宽 | fn 值调用在值串位/操作数位停（exit 70，`bndMainBody`），`docs/benchmarks.md` 的 run-face 段「闭包与 fn 值」未限位置；**零黄金覆盖** | 词面收窄（本提交，双语）；补黄金归 B1b 行 |
| N2 | 诚实边界 + 零覆盖 | 内置所产 `Option` 的载荷读入 `String` 位停，数值消费可跑；**零黄金覆盖** | 同上 |
| D1 | 工件缺陷 | `tasks.md` 双份「新增黄金总量落盘」（一份陈旧未勾） | 已删（本提交） |
| D2 | 语言纪律偏离 | 25 行中文注释 | 已修（`0aac57b`） |
| D3 | 消息纪律偏离 | 1 行 E0105 消息未以 title 起头 | 已修（`0aac57b`） |
| O1 | 流程观察 | 注释语言无机械门；本变更无 red-first 提交（测试先行证据在探针与突变电池） | 登记，不改流程 |

### 与 D12 的差（as-built）

D12 写「follow-up #16 划账（修复随 D1 落地，**条目移除**）」。as-built **不移除该条，改在原位记闭环**，理由是引用完整性：本变更自己的记录已按当时编号写死 #17–#22（T3 的 #18/#19/#20、T4 的 #21、T11 的 #22、D0 订正引 #22），而归档工件同样引 #4–#15，follow-up 编号在本仓是**只增不改**的引用锚。移除 #16 会把 #17–#22 全部前移，使已归档工件与本变更记录里的引用**当场失真**——那是结构化的谎言而非对账。故 #16 保留编号、行内记「已由 B1a 修复」并留证据指针；D12 那句「条目移除」的意图（不再作为开放项挂着）由此实现而不失真。**同表新增 #23**（N3，见发现汇总）。

### 归档复验（2026-09-11）

归档动作完成（`mv openspec/changes/codegen-mono → openspec/changes/archive/2026-09-11-codegen-mono`；`change.yaml` 由 `complete` 置 `archived`）后，在归档树上重跑整条阶梯，逐项与 §7 一致：

- **编译面**：`gofmt -l .` 空；`go build ./...` exit 0；`go vet ./...` exit 0。
- **测试面**：`go test -count=1 ./...` exit 0 —— **13 个有测试的包 ok + 2 个无测试文件的包（`cmd/we`、`internal/ast`），0 FAIL，即 15 包**（与 §7 的包数订正一致）。其中 `internal/conformance` 全量黄金 **810 枚** `ok 104.201s`、`runtime`（C 运行时测试）`ok 19.034s`。**该轮 104.201s 与 §7 记的 169.176s 是同一命令的两次真跑**，差值来自机器负载，非语料变化——两轮的落盘计数都是 810。
- **工件面**：`validate.py --all --strict` → `OK: no changes found (nothing to validate); registry clean`（exit 0）。**该输出是归档后的期望态而非跳过校验**：`validate.py:286` 按设计排除 `archive` 目录，故「no changes found」= 活动变更集为空；registry 检查未随之短路（零活动变更路径的早退漏洞已在 `grammar-skeleton` 归档后修掉，registry 检查移到早退之前），所以 `registry clean` 是本轮实跑结论。归档前同一命令在本变更上为 OK。
- **文档面**：`docs_sync.py` → `OK: 33 document pair(s) aligned`（exit 0）。
- **红线面**：`git diff --check` 空（无空白错误）；`git status --porcelain | grep -c 'refr/'` = 0、`git diff --cached --name-only | grep -c '^refr/'` = 0；`.githooks/pre-commit` 全程未触发。
- **落盘复核（磁盘独立计数，非引用旧记录）**：`internal/conformance/testdata/cases/*.json` 共 **810** 枚 = check 499 + run 131 + test 77 + build 37 + 工具面 66（fmt 19 / vet 16 / new 6 / doc 6 / version 5 / clean 5 / lock 3 / unknown 2 / deps 2 / no 1 / color 1）——与 §2 的 701→810 全链终值相符。
- **词表终态复核**：五个边界词全部在场（`bndMainBody`/`bndTopLets`/`bndTaskBody`/`bndGenericFns`/`bndFnBody`，`codegen.go:55-61`），与 T13 裁定 1「保留三锚、退役改挂 B1b」一致；`build-bnd-*` 黄金期望未动。
- **正副本核对**：归档目录内恰四件（`change.yaml` + `proposal.md` + `design.md` + `tasks.md`），无 `specs/`——与本变更「零规范增量」的 internals-only 豁免一致（proposal 载 `无规范增量` 标记）；目录名 `2026-09-11-codegen-mono` 合规（`docs/README.md` 的 `YYYY-MM-DD-<name>`）。

**未跑项（如实记录）**：无。归档复验只跑阶梯与对账，未新增码改、黄金改或文档改——本段落地后工作树相对 `0aac57b` 的差异仅为本变更目录（新入册）+ `docs/roadmap/0000-reference-implementation.{md,zh.md}` + `docs/benchmarks.{md,zh.md}`。
