# proposal — effects

## Why

第 16 章（效果声明与检查）已批准两年于规范、五码已注册（E1401–E1405，owner `1600-effects`），而参考实现对整章停在五个解析占位上——真机探针逐一钉死：

- `effect db`（顶层声明）→ exit 70 `chapter 16 (effects) forms`（parser.go:404）
- `fn f() effect io { … }`（fn 声明段）→ 70（parser.go:499）
- 接口方法段 / impl 方法段 → 70（parser.go:1276/1438）
- `let f = fn(s: String) effect io { … }`（闭包段）→ 70（parser.go:650）
- fn **类型**段是另一半空洞：`fn(Int64) io -> Int64` 解析器已支持（`ast.FnTypeRef.EffectTags` 全管线保留——subst/rebaseClause/closureType 均搬运），但语义为空——(a) 标签不解析：未声明的 `weird` 不报 E1304；(b) `sameType` 对 tags 做精确比较：纯闭包入 `fn(Int64) io -> Int64` 槽今日**假报 E0501**，而规范裁定这是合法（纯值满足效果槽）；(c) 调用位零检查：纯函数调用 io 注解参数不报 E1401。

typecheck 里零效果机器（grep "effect" 两处皆注释）。效果是语言的横切静态纪律——不落 ch16，「AI 可审计的效果面」这一语言卖点只剩文句。

## 裁决记录（candidate 阶段四项表面裁决，2026-09-06，均采纳推荐）

1. **Q1 范围 = ch16 全量一变更**。含全部跨章活表面：ch12 fn 文法段位、ch3 defer 只偏移时机不偏移归属、ch14 panic 族零效果、ch15 跨模块合格名 `mod.tag`、ch10 接口/impl 精确一致。理由：效果检查是一台横切机器（声明集 → 推断 → 三族检查），拆片会让中间里程碑携带半套集语义（如先落声明不落检查——每条调用都在未定义纪律下通过）。ch18 task 段拼写 / ch20 test 免检查两条表面本里程碑**不可达**（解析停 bndConc/bndTest，见非目标与 design D13）。
2. **Q2 代码生成 = 维持 M4 接收集**。效果是编译期擦除面：检查通过即效果集对代码生成零语义，IR 零改、codegen 不触（M5–M6b 冻结先例延续）。
3. **Q3 一致位架构 = 织入一致机器**。E1402 与 E0501 同锚同位：结构不一致仍报 E0501（既有），唯「效果超集」报 E1402，子集合法。副作用是一处**既有假报翻绿**：纯闭包入效果注解槽从今日 E0501 翻为合法（今日无黄金携带此形——已 grep 实证，零改写）。被拒：改 sameType 全局语义为子集——E0830/E0808/unify 恒等位全依赖精确比较。
4. **Q4 闭包段位 = 改为 E0105**。ch16 R4：闭包无声明段槽（效果集是推断的），`fn(…) effect io { … }` 是段位拼写串位——E0105（既有码，今日闭包位停 70 是 M2 占位边界应替换）。fn 声明位裸标签 `fn f() io { … }` 与类型位 `effect` 关键字 `fn() effect io -> T` 两形今日已正确发 E0105（锁定枚钉住）。

## 现状与差距

| 面 | 现状 | 差距 |
|---|---|---|
| 顶层 `effect name` 项 | bndEffect 停点（parser.go:404） | 产生式 + pub 位 + 名字纪律（E0012/E0404/E1403） |
| fn 声明段 / 接口方法段 / impl 方法段 | bndEffect 停点（499/1276/1438） | 三段位解析入 AST |
| 闭包段 | bndEffect 停点（650） | 换 E0105（Q4） |
| fn 类型段 | 解析器已支持，tags 为不解析透明字符串 | 标签解析（E1304）+ canonical 键 |
| sameType | tags 精确比较 | 保持精确；一致位新机器织入（Q3） |
| 调用位 / 一致位 / 接口位 / 初始化位 | 无机器 | E1401/E1402/E1404/E1405 四族 |
| 闭包效果集 | 无推断（恒空） | 体调用并集推断 |

今日全程序无人能声明效果（五停点），故一切闭包推断集为空、⊆ 任意声明集——**M7 落地对既有 356 黄金零回归**（唯一行为差是 Q3 披露的假报翻绿，而该形零黄金携带）。

## 目标与非目标

目标：

1. effect 声明与名字纪律：顶层 `effect name` 项（pub 位、camelCase E0012、模块单一名字空间 E0404、内建 io/net/time 冲突 E1403——内建为语言级非模块项）；段内标签解析（裸名本模块、`mod.tag` 跨模块经 import 扩展、未解析 E1304）。
2. 声明段三面：fn 声明、接口方法、impl 方法（`effect tag1 tag2` 位于参数表与 `->`/体之间，缺省 = 纯）；闭包段位 E0105（Q4）。
3. fn 类型段激活：`fn(T1,…,Tn) tag1 tag2 -> T`（裸标签，无 `effect` 关键字——两拼写互串 E0105）。
4. E1401 调用位：被调效果集 ⊆ 当前声明集；被调集四路来源（fn 符号声明集 / 方法集（impl 与接口两视角）/ fn 型值 tags（含闭包推断集）/ panic 族零集）；defer 体调用计入外围函数；报文含效果名与 callee。
5. E1402 一致位：单向 ⊆（值集 ⊆ 槽集），织入 E0501 同位（Q3）；嵌套 fn 型保持精确（无方差）。
6. 闭包推断：效果集 = 体调用并集；构造 ≠ 执行（构造不污染外围函数）；嵌套闭包体不并入外层。
7. E1404 接口位：impl 方法效果集与接口方法**精确相等**（缺、多同拒）。
8. E1405：顶层 let 初始化器必须纯（效果调用即拒；闭包构造豁免）。
9. bndEffect 常量删；conformance 全绿（既有 356 零回归 + 新增 ~50）。

非目标：

- 代码生成扩面（Q2 裁决；效果编译期擦除）。
- ch18 task 段拼写（`task effect net`）与 ch20 test 免检查——解析停在 bndConc/bndTest，体内 token 不可达，本里程碑披露不可达（design D13）；等待原语零效果句已入规范，运行面归 M9。
- 效果多态 / 高阶效果 / 效果处理器（规范未定义，无实现面）。
- std 库效果注解（M8——io/net/time 内建三词本里程碑仅作名字冲突面与合法标签，无库内容）。

## What Changes

见「目标与非目标」九项——本变更零规范增量，无 ADDED/MODIFIED Requirements 面；实现层变更 = parser 产生式（effect 项 + 三声明段位 + 闭包段位替换）+ typecheck 四层（名字与集表示 / 调用位 / 一致位与推断 / 接口位与初始化位）；cli 装载层与 codegen 零改（Q2）。机制选型（canonical 键形状、织入比较器挂点、收集器跳过边界）全部在 design.md 对应 D 条，此处不重复。

## 影响层

compiler（parser + typecheck）。conformance 黄金新增 ~50、改写 0。规范先行（22 章全批），本变更**无规范增量、不改变语言行为**（ch16 章文与 E1401–E1405 注册表早已批准冻结；纯实现，接受/拒绝行为全部由既有章文与注册表钉死，零新增/修改诊断码定义），无 specs/ 提升面。

## 影响范围

- `internal/parser/parser.go`（五停点替换 + 名字纪律）
- `internal/ast/ast.go`（EffectDecl + 三段位字段）
- `internal/typecheck/typecheck.go`（集表示/解析/四族检查/闭包推断）
- `internal/conformance/testdata/cases/`（新增黄金）

## 涉触码盘点

**首发五码**（注册表在位，owner `1600-effects`，本变更落实现）：

- E1401 undeclared effect at a call（调用位 ⊆）
- E1402 function value effect set does not match the expected type's（一致位单向 ⊆）
- E1403 effect name conflicts with a built-in effect（`effect io` 声明位）
- E1404 impl method effect set disagrees with the interface（精确相等）
- E1405 effectful call in a top-level initializer（初始化器纯）

**已在发码新位**：E1304（段内标签解析，裸/合格两形）；E0404（effect 名入模块名字空间）；E0012（effect 名 camelCase）；E0105（闭包段位——Q4 替换；fn 声明位裸标签与类型位 keyword 两形今日已对，锁定枚钉住）。

**复用既有机器**：sameType（嵌套 fn 型保持精确——Q3 织入不改全局）；subst/rebaseClause/closureType 的 tags 既有保留（canonical 化沿用管线）；importSym/跨模块七合格位（扩 effect kind）；propCtx.deferBody（defer 归属语境）；terminationFnType（panic 族天然零集）。

## 已知风险与开放问题

1. **sameType 精确性的依赖面**：E0830/E0808/checkImplMethodSig/unify 恒等比较全依赖 tags 精确——织入而非改全局（Q3 已裁）；单测钉「结构不一致仍 E0501」防分流错位。
2. **tags 表示翻转**：从「不解析透明字符串」到「canonical 已解析键」触 fnType 全生命周期（subst/rebase/闭包三处搬运既有，键替换需一致）——T4 单测矩阵钉。
3. **E1402/E0501 同锚分流次序**：结构比较先行、效果子集后判——黄金钉死两形（结构错报 E0501、唯超集报 E1402）。
4. **闭包推断与 M5 期望线程交互**：短闭包定型已有体走查位，推断收集器挂同点；嵌套闭包「构造≠执行」的跳过边界单测钉。
5. **报文成分**：E1401/E1405 注册表 title 未含完整模板——实现期以注册表 description 为骨、实发为准，黄金钉实发（M6b 惯例）；与注册表 title 不冲突。

## 审计记录（2026-09-06，welang-spec-impact-audit 7 条，通过）

1. **问题真实性 ✓**：Why 全部为真机探针可复现的黑盒缺口（五停点 exit 70 + fn 型段三空洞），基线固定为已批规范 `docs/spec/1600-effects.md`（7 Requirement / 31 Scenario）与 `docs/spec/diagnostics.toml` 行 1100–1145（E1401–E1405 五条目，owner `1600-effects`，grep 复核在位）。无 refr/ 草案被当规范引用。
2. **影响层声明 ✓**：change.yaml `layers: [compiler]` 与 proposal 影响层一致。改变语言行为但**零规范增量**——ch16 与五码注册表先行已批（规范先行纪律，M6b 同形先例：三层已批章节的 compiler 变更不含 spec 层），proposal What Changes 明文「docs：零规范增量」。
3. **规范增量范围 ✓**：新增/修改/删除 Requirement = 0；诊断码新增 = 0（五码均为注册表在位的**落实现**，非新定义）。无冲突：E1401–E1405 与既有码域无交（14xx 段归 effects 章）；复用码新位（E1304/E0404/E0012/E0105）均为各章已定义语义在新表面的到达，非语义扩容。
4. **原则一致性 ✓**：不突破十条。P1 局部可判定（调用位/一致位/接口位全是局部静态比较，无调用点依赖的合法性）；P2 唯一语义（Q4 消除闭包段的 M2 占位歧义）；P3 可判定（集比较可判定，无效果多态）；Q3 织入不改 sameType——无新隐式转换（子集合法是 ch16 明文语义非本变更引入）。
5. **参考基线固定 ✓**：规范引用全部指向 `docs/spec/1600-effects.md`、`docs/spec/diagnostics.toml`（仓库内已批文件）；探针行号（parser.go:404/499/650/1276/1438）为实现期现状快照，非规范依据。
6. **验收边界 ✓**：目标 1–9 逐条可机械判定（五码各形黄金 + conformance 全绿 + bndEffect 常量删 + 356 既有零回归）；非目标四面（codegen/ch18/ch20/效果多态）显式排除且 D13 逐条披露不可达机制，足以防蔓延。
7. **粒度 ✓**：ch16 全量一变更（Q1 裁决）是可垂直验证的最小单元——效果是「声明集 → 推断 → 三族检查」一台横切机器，五停点与四检查族互不可拆；无三层改动（cli/codegen 零改，docs 零改）。

**结论：通过。** status → ready。

## 审查记录（ready → active 关卡，welang-change-review 10 条，2026-09-06）

1. **proposal 职责边界：通过（自纠一处后）**。Why/现状与差距全为探针可复现黑盒缺口（五停点 + fn 型段三空洞）；裁决记录节是 Q1–Q4 表面裁决的记录非实现决策（M6b 先例同形）；What Changes 首版混入实现符号（fnAgree/checkImplMethodSig/canonical 键形状）——已自纠为层概览 + 指向 design D 条，机制细节归位 design。
2. **spec 增量：通过（豁免形）**。零规范增量（无 specs/ 目录），proposal 豁免标记「无规范增量、不改变语言行为」在文（validate.py --strict 结构合法）；接受/拒绝行为全部由已批 ch16 章文与注册表 E1401–E1405 钉死。
3. **design 唯一最小路径：通过**。D1–D13 每条给出唯一选型并记被拒替代（闭包段解析后忽略 / E1403 挪 typecheck / tags 双写归一 / 调用图全程序先收集 / sameType 改子集 / E1404 单向 ⊆ / 闭包声明段 / 初始化器禁一切调用）；章文与诊断码引用精确（R1–R7、E1401–E1405、E1304/E0404/E0012/E0105 各归位）；与零 spec 增量无矛盾。
4. **tasks 职责边界：通过**。T1–T11 每项有来源（proposal 目标号 + design D 条）与验证（命令/对账面）；无 deferred、无 non-goal 引用、无未决方案选择。
5. **场景覆盖：通过**。D12 三路齐：normal（三声明段绿例/推断入一致位/跨模块两写法同键）、boundary（defer 归属/嵌套闭包跳过/构造 ≠ 执行/缺省段 = 纯/panic 天然零集）、failure-degradation（五首发码各形 + E1304 裸/合格 + E0501 分流两形）；非 happy-path-only。
6. **无空章节：通过**。四工件无 TBD/占位/无行为增量包装层；「已知风险」五项全部指向 design 已定形条目（D5 分流/D6 跳过边界/D3 键翻转）或既有惯例（报文成分 M6b 形），非开放问题。
7. **测试先行：通过**。T1 黄金表先红（对当前构建运行必须先红 + 356 既有零回归对账）、T2 单测先红（引用未定义节点/分支）皆为首任务；后续任务逐枚翻绿对账（T8 收尾全绿）。
8. **负向断言：通过**。每个禁止模式有真实违规样例黄金：E1401 六形、E1402 超集/效果入纯槽、E1403 io/net、E1404 多/缺、E1405 三形、E1304 三形、E0105 闭包段两形——构造违规并断言被拒，非文字代替。
9. **完成度闭环（原则 10）：通过（自纠一处后）**。三要素明文：类型检查 = 变更全部；代码生成 = D10 零面（Q2 裁决 + 擦除论证）；运行时 = D10 首版未明文「ch16 无运行时面」——已自纠补一句（效果是纯编译期纪律，无处理器/运行钩子/运行时可观察行为，非目标披露多态与处理器）。
10. **未决问题阻塞：通过**。无影响行为/契约/验收的开放问题——「已知风险」五项均已有设计定形与钉住手段（单测/黄金），无一需要阻塞 task；四项表面裁决（Q1–Q4）已批复记录在案。

**结论：通过（自纠 2 处：What Changes 实现符号归位 design / D10 运行时要素明文）。** status → active，进入实现。

## 实现审查记录（active → complete 关卡，welang-code-review 7 条，2026-09-06，真二进制证据）

1. **规范符合性：通过**。对照 `docs/spec/1600-effects.md` 7 Requirement 逐条（真机二进制黑盒）：R1——`effect name` 项/pub 位/camelCase E0012/单一名字空间 E0404/内建冲突 E1403（io/net 两形真机逐字）；「内建词作普通名合法」场景真机绿（`fn time(n: Int64)` + `let net = 3` exit 0——名字表零内建预置，E1403 只辖 effect 声明位）；跨模块 pub 合格标签（xm 三模块链静默）；未解析 E1304 三形（裸/未导入/non-pub→E1303）。R2——段位于参数表与 `->`/体之间；⊆ 判 E1401 六形（直接/方法/bound/fn 型值/defer 8:13/多标签差集）；defer 归属外围；panic 族零集两绿；task 段拼写与 test 免检查两表面 ch18/20 停边界不可达（D13-1/2 真机实证 exit 70）。R3——fn 型段裸标签、keyword 互串 E0105（fntype-keyword 锁定枚）；一致位单向 ⊆ E1402 六形（含 Q3 翻绿钉静默、结构不一致 E0501、嵌套精确 E0501）。R4——两形闭包推断（全注解/短）、构造 ≠ 执行（construction-pure 绿）、推断集入一致位（closure-arg 绿）。R5——接口方法段；E1404 精确相等（extra/missing 逐字）；默认体调用计入接口方法自身集 + override 同判（D13-4 真机双针：默认体 E1401 @ 9:9 报 "put"、override 段不一致 E1404 @ 18:8）；Dyn/bound 调用查接口集（bound-call 绿）。R6——E1405 两形；纯初始化器绿；闭包构造豁免。R7——E1400–E1499 段位注册表在位（validate --strict registry clean）。零规范外行为。
2. **验证诚实性：通过**。tasks.md T1–T9 逐项核对——T8/T9 全部验证命令本窗口真实重跑（conformance 404/404、`go clean -testcache && go test ./...` 九包 ok、gofmt/vet、真机电池 48/48）；T1–T7 完成记录的红名单递减链（43→22→17→11→4→2→0）与终态零矛盾，实现期修正（黄金 13+7 处/锚对齐/m7_test 四处 off-by-one）全部披露在案。无「勾了没跑」项。
3. **测试先行证据：通过**。T1 黄金先红（43 红全部新枚，comm 交叉核对红 ∖ 新 = 空）+ T2 单测先红（`undefined: ast.EffectDecl`/`FnDecl.EffectTags undefined` 编译红——恰为 D1 新节点）皆先于 T3 实现；黄金期望输出符合 §52 Human face（真机 stderr 逐字 diff 48/48）与 §62 JSON Lines（`--json` 实证：type/severity/code/message/file/line/column/help 字段齐，help 位带 remediation）。
4. **诊断协议稳定：通过**。`internal/diag` 零 diff（`--json` 字段零删改）；新码零个——E1401–E1405 均为注册表既有落实现（行 1100–1145，owner `1600-effects`）；复用码新位（E1304/E0404/E0012/E0105）语义无扩容，validate --strict 过。
5. **单一权威：通过**。零规范增量（无 specs/ 目录，归档无提升面——docs_sync 30 对齐不变）；代码注释全英文（typecheck/parser/ast/codegen 增行中文行数 = 0）；变更工件中文符合惯例。
6. **红线复核：通过**。git status 无 refr/ 路径（48 untracked 全在 testdata/cases + openspec 变更目录）；提交未发生（T11 时 pre-commit/commit-msg 钩子 + core.hooksPath 双重强制再拦）；diff 改动面 = proposal 影响范围清单所列（ast/parser/typecheck/parser_test/黄金/tasks）+ codegen `*ast.EffectDecl` 擦除 case——后者是新 AST 节点在 Emit switch 的必要归处（D10 记录在案，IR 结构零改），非越界。
7. **最小可信验证：通过**。gofmt -l 空；go build/vet 过；`validate.py --all --strict` 过（1 change valid, registry clean）；docs_sync 30 对齐；git diff --check 干净；conformance 404/404；清缓存全仓 9 包 ok；真机电池 48/48（20 绿静默 + 28 负例逐字）。

**结论：通过。** status → complete，进入归档（welang-archive-sync）。
