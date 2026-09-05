# control-and-composites — 第 3/4/8/9/12 章前端（解析 + 类型检查）：`we check` 深度拓宽

## Why

M4（native-vertical，归档 2026-09-05）交付了端到端骨架：`we new` → `we check` → `we build` → `we run` 对「和式声明 + main 单返回」程序成立。但语言的表达层到此为止——已批准的二十二章里，控制流、模式匹配、复合类型、函数值与闭包全部停在 M2 解析器的逐章边界行上。按 roadmap M5 行，下一里程碑把这些章的前端接上：

- 第 3 章（`docs/spec/0300-control-flow.md`）：if/else 表达式（臂一致、无 else 值位 E0202）、while/loop 语句、裸 break/continue（E0201）、defer 块（E0203/E0204、LIFO）；
- 第 4 章（`docs/spec/0400-match.md`）：match 表达式（ scrutinee 恰一次、首臂胜）、封闭模式集（字面量/绑定/通配/或/守卫/元组/变体）、穷尽性（E0305/E0306/E0307）、臂一致（E0501/E0605）；
- 第 8 章（`docs/spec/0800-composites.md`）：record/byval/byres 声明、构造与更新表达式、字段访问、newtype、元组（2–8）、unit、元组模式解构、值丢弃（E0605 已在 M3）、if 臂一致、局部遮蔽；
- 第 9 章残余（`docs/spec/0900-sum-types.md`）：M3 已落 sum 声明、变体构造器、Never（E0701/E0703/E0704）；残余 = 值和式载荷域检查（E0702）；
- 第 12 章（`docs/spec/1200-fn-types.md`）：函数类型（M3 已解析注解槽）、单态 fn 名值位、完整/短闭包形式、期望类型推断（E1001）、捕获纪律（E1002/E1003）。

可验证的黑盒缺口（本日真机复验，`/tmp/we` 为当前构建）：

- `if` 语句体 → `we: chapter 3 (control flow) forms are not implemented in this reference build yet`，exit 70；
- match 体 → `we: chapter 4 (match) forms ...`，exit 70；
- `record User { ... }` 顶层项、元组表达式 → `we: chapter 8 (composites) forms ...`，exit 70；
- 闭包表达式 → `we: chapter 12 (fn types and closures) forms ...`，exit 70；
- 类型侧同停：fn 名出现在值位（`let f = square`）→ `we: function values (chapter 12) ...`，exit 70；成员访问（`u.name`）→ `we: standard-library modules (chapter 15) ...`，exit 70——记录字段无从访问。

二十一个 M5 段位的已批诊断码（E0201–E0204、E0301–E0307、E0601–E0606、E0702、E1001–E1003，另 E0501/E0503/E0816 按章复用）在注册表中全部就位、无一曾被发射。本里程碑交付 roadmap 承诺的前端深度：上述形式的程序在 `we check .` 下按规范全量接受或拒绝。

## 裁决记录（candidate 阶段四项表面裁决，2026-09-05，均采纳推荐）

- **Q1 切片范围 → 一个变更全载**：ch3 + ch4 + ch8 + ch9 残余（E0702）+ ch12 前端（解析 + 类型检查）在本变更一并落地——roadmap「一里程碑一变更」既定；验证面单一（`we check` 深度）；五章共享表达式层，拆开互相悬空（match 的模式要元组与变体、闭包的类型要 fn 型、if 的臂一致要 unit）。
- **Q2 代码生成深度 → 维持 M4 接受集**：Emit 只增 record/newtype 声明擦除（与和式擦除同机制、零 IR），M4 What 表四行原样；新形式程序穿类型检查后在 main 体形状处停边界 exit 70。理由（design D11）：闭包与 gc 记录的诚实 IR 依赖 M8 的 GC 设计（追踪单元格布局未定）；控制流 IR 在 M5 程序集上可观察收益为零（程序仍只能 `Ok(())` / `Err(字面量)` 收尾）；FPCR 评价门以 `we check` 为核心。IR 拓宽随 GC 落地后的里程碑。
- **Q3 成员解析深度 → 字段落地 + E0816**：记录字段访问与 newtype `.value` 真落地；记录上非字段名 → E0816（码与触发语义均已批——M5 无方法可存在（impl/derives 不解析），「成员集 = 字段集」恰成立，实现次序不改变已批规则的真值）；基类型接收者（String 方法 = stdlib 表面）维持既有 bndStdModules 边界（What 文本不变、语义收窄，design D10）。
- **Q4 穷尽性验证深度 → 章内场景 + 对抗矩阵**：ch4 全部穷尽性场景黄金之外，另加对抗矩阵单测锁定 Maranget 有用性算法——含假穷尽反例（`(Some(a), None) | (None, Some(b))` 对 `(Option, Option)` 必须报 E0305——逐位并集会误判穷尽，积空间推理的必要性证明）、三元组合、或模式展开、嵌套变体（design D8）。

## 目标与非目标

### 目标

1. **ch3 解析与检查**：if/else 表达式（值位 = 所取臂块值；语句位；else-if 链 = 嵌套；臂作用域隔离）、while/loop 语句、裸 break/continue（循环体内合法、闭包体不算循环体）、defer 块（E0203 非块、E0204 仅函数体顶层项、体内 return = E0401 既有）；E0201/E0202 按注册表描述族触发。
2. **ch4 解析与检查**：match 表达式（非 ch2 块的臂组、行界分隔、逗号 E0105、零臂 E0301）；封闭模式集递归文法（变体模式 PascalCase 词法分野、`let` 名位仅不可驳模式 E0105、`()` 与负字面量非已批模式 E0105、E0302/E0404 句法名字集）；类型侧 E0303/E0304/E0503/E0501（含或模式同名异型）；穷尽性 = Maranget 有用性算法（E0305 基类型不可穷尽、E0306 守卫臂不计数、E0307 前缀覆盖即不可达）。
3. **ch8 解析与检查**：record（gc/byval/byres）与 newtype 顶层声明（泛型/derives 子句维持 ch10 边界）；构造表达式（E0604 字段集三违、E0603 头非记录、E0501 字段型）；更新表达式（E0603 基型不合、E0604 未声明字段、E0606 资源）；字段访问只读（成员赋值 E0105 既有）；E0601 值记录字段域、E0702 值和式载荷域；元组 2–8（E0602 两处：类型/表达式——注册表描述字面；模式超元走 E0501，design D3 勘误）、元组模式（E0501 逐元、E0404 同名二绑）；if 臂一致经 unit；局部遮蔽（就近绑定，M3 机制补测试钉死）。
4. **ch12 解析与检查**：单态 fn 名 = 值（fnType 签名型，签名精确一致 E0501）；经 fn 型值的调用；完整闭包（`fn(params) [-> type] block`，函数体语境：return/defer/E0402）；短闭包（`|p| body` 体最大化、操作数位须括号 E0105、零参唯一完整形式 `||` = 逻辑或 E0105、裸参依赖期望类型否则 E1001、标注参与期望不合 E0501）；捕获纪律（gc 活引用、值/基类型拷贝快照 + 内部赋值 E1003、资源禁捕 E1002）。
5. **成员解析**（裁决 Q3 定深度）：记录字段访问落地、newtype `.value` 落地、记录非字段成员 E0816、基类型接收者维持 stdlib 边界。
6. **codegen 最小增量**（裁决 Q2 定深度）：record/newtype 声明擦除（与和式擦除同机制，零 IR）；接受集其余不动——新形式程序穿检查后在 M4 What 表停边界 exit 70（真机披露）。
7. **边界表收敛**：解析器 15 What 删 4（bndCtl/bndMatch/bndComp/bndClosur）、类型检查 12 What 删 1（bndFnValues）；钉住这些行的 2 枚既有黄金（check-boundary-ch8 两枚）随形式落地改写并披露。
8. **conformance**：黄金新增约 60 枚（每新触码至少一负例 + 分组正例 + 边界收敛改写 2 枚 + codegen 边界代表），全部先红；其余 121 枚既有黄金零改写。

### 非目标

- **代码生成的控制流/复合值/闭包 IR**（裁决 Q2 推荐：维持 M4 接受集）——`we build`/`we run` 对新形式程序停在 M4 What 表；IR 拓宽随后续里程碑（GC 设计落地后）。
- **ch10 方法、impl、derives、泛型**：record/newtype 的泛型子句与 derives 子句维持 bndGener 边界；方法值（`u.describe`）在 M5 无方法可存在，E0105 方法值面不可达——design 披露；E1004（泛型 fn 名值位）因泛型 fn 声明不可解析而不可达——design 披露。
- **效果系统**（ch16，M7）：fn 声明效果段、闭包效果推断、defer 效果计数（E1401）、fn 型注解效果段的检查——带效果段的 fn 型在 M5 惰性可解析不可绑定（无声明可携带段），design 披露。
- **`?` 传播与 panic 家族**（ch14，M6）：维持 bndErr 边界；defer 体内 `?` 的 E1202 因此不可达——design 披露。
- **for/迭代**（ch11，M6）：维持 bndIter；for 头元组模式随 M6。
- **多模块、stdlib 模块、Dyn、List/Map/Set、Range**：既有边界原样（bndMultiModule/bndStdModules/bndDyn/bndCollections/bndRange）。
- **赋值目标可变性**：`let x = 1; x = 2` 的拒绝规则与诊断码在已批章节中不存在（规范缺口）——维持 M3 现行为（类型检查通过），登记 roadmap follow-up（#7），design D14 披露。
- **shadow 后旧绑定可达性、运行时语义**（拷贝/活引用的可观察差异）：M5 是静态前端；运行时行为随代码生成里程碑。
- **注册表 E0604 描述陈旧子句**：E0604 描述含「a member access names an undeclared field / Fires under: Field access」子句，与 ch8 R5（经 ch10 修订）的 E0816 路由（「unknown names are one rule, one code」）冲突——章文是触发语义权威，M5 对记录未知成员发 E0816；本变更不触 diagnostics.toml，子句清除登记 roadmap follow-up #8（由下一个触注册表的规范层变更或专门维护变更执行）。

## What Changes

- `internal/ast`：新节点——语句（While/Loop/Break/Continue/Defer）、表达式（If/Match/Construct/Tuple/Closure）、模式族（PatLiteral/PatWildcard/PatBinding/PatOr/PatTuple/PatVariant）、声明（RecordDecl/NewtypeDecl）、match 臂；绑定语句名位接受元组模式。
- `internal/parser`：ch3/ch4/ch8/ch12 产生式落地（dispatch 表扩展、E0201 循环深度、E0202 值位判定、E0203/E0204 defer 放置、模式文法、构造/更新、闭包两形）；What 表删 4 行。
- `internal/typecheck`：声明登记（record/newtype + 类别表 + E0601/E0702）、控制流检查（臂一致/条件 E0503/块值交互）、match 全量（模式类型化 + 穷尽性算法 + 臂一致）、fn 值/闭包（期望线程 + 捕获账本 E1002/E1003）、成员解析（字段/.value/E0816）；What 表删 1 行。
- `internal/codegen`：声明擦除扩展（record/newtype 零 IR）。
- conformance：约 60 枚新黄金 + 2 枚改写。
- `docs/roadmap/0000-reference-implementation.md` 与 `.zh.md`：M5 行翻 done；follow-up #7（赋值目标可变性规则）、follow-up #8（注册表 E0604 描述陈旧子句维护——见下「影响层」）登记（归档动作）。

## 影响层

`compiler`（已批准章节前端的部分实现，无规范增量）。**本变更不改变语言行为、无规范增量**：一切接受/拒绝判定以已批准的 `docs/spec/0300-control-flow.md`、`0400-match.md`、`0800-composites.md`、`0900-sum-types.md`、`1200-fn-types.md` 及注册表现有 21+3 个涉触码为依据（零新增/修改诊断码；消息按一码一消息纪律以注册表 title 起头组合）；穷尽性算法是 ch4 Exhaustiveness Requirement 的机制实现（Maranget 有用性，design D8 论证其与逐场景的覆盖等价）；捕获纪律的元组/newtype 类别派生是 ch12「captures answer chapter 8's ownership categories」对未逐字枚举组合的机制读法（design D9 披露）；赋值目标可变性的规范缺口以 roadmap follow-up 登记，不以本变更私定行为。

## 影响范围

- 修改：`internal/ast/ast.go`（新节点）、`internal/parser/parser.go`（产生式 + What 表）、`internal/typecheck/typecheck.go`（检查器扩展 + What 表）、`internal/codegen/codegen.go`（声明擦除）、既有 2 枚黄金（check-boundary-ch8、check-boundary-ch8-json——钉住的边界行随形式落地消失）。
- 新增：`internal/parser`/`internal/typecheck` 的按章单测扩展、约 60 个 `internal/conformance/testdata/cases/*.json`。
- 不动：`docs/spec/`、`diagnostics.toml`、`go.mod`、`internal/lex`、`internal/diag`、`internal/version`、`internal/cli`（三命令行为面不变——check 三面、build/run 走共享管线）、`runtime/`、其余 121 枚既有黄金。

## 审计记录（welang-spec-impact-audit，2026-09-05）

1. **问题真实性**：成立。Why 所列黑盒缺口本日真机复验（`/tmp/we` 当前构建）：ch3/ch4/ch8/ch12 四条解析边界行 + 类型侧两条（fn 名值位 `let f = square`、成员访问 `u.name`）全部 exit 70 逐字命中；二十一个 M5 段位已批诊断码（E0201–E0204、E0301–E0307、E0601–E0606、E0702、E1001–E1003，另 E0501/E0503/E0816 按章复用）注册表在位、发射记录为零。缺口 = 已批准规范的前端未实现，非规范缺口。
2. **影响层声明**：`layers: [compiler]` 与内容一致（五个 `internal/` 包 + conformance 黄金）；「**本变更不改变语言行为、无规范增量**」边界句在位（影响层小节），满足 validate.py internals-only 豁免标记；零 `docs/spec/` delta、零注册表改动。
3. **规范增量范围**：零新增码、零修改码。22 个涉触码逐条对照注册表核验（title/description/remediation）：E0202 的「another ratified valueless statement form」族读法覆盖 break/continue/defer 值位；E0201/E0203/E0204/E0301–E0307/E0503/E0601–E0606/E0702/E0816/E1001–E1003 触发语义与 design 判定一致。**审计期两项勘误**：① E0602 注册表描述只覆盖「type 与 expression」——design 草稿曾把 >8 元模式位也计 E0602，已修正（模式照解析、类型侧 E0501，D3/D4）；② E0604 描述含「member access names an undeclared field」陈旧子句，与 ch8 R5（经 ch10 修订）的 E0816 路由冲突——章文为触发语义权威（ch99 权威分立），M5 发 E0816，子句清除登记 follow-up #8（D10 披露）。两项均为对既有注册表的字面复读纠偏，不构成规范增量。
4. **原则一致性**：通过。穷尽性 Maranget 算法是 ch4 Exhaustiveness Requirement 的判定器实现（D8 论证覆盖等价 + 假穷尽反例强制）；捕获账本的类别派生是 ch12「captures answer chapter 8's ownership categories」的机制读法（D9 披露）；catOf 的 newtype 底层派生有「zero-cost wrapper layout erased」支撑（D6）；一切拒绝以已批章文 + 注册表 title 为据，无私定行为——let 重赋值规范缺口维持现状并登记 follow-up #7，不以实现私立法。
5. **参考基线固定**：通过。proposal/design/tasks/裁决记录零 `refr/` 引用；全部依据为 `docs/spec/` 已批章文与 `diagnostics.toml`。
6. **验收边界**：可机械判定。每目标附验证命令（`go test ./internal/{parser,typecheck,codegen,conformance}/`、真机电池、gofmt/vet/validate/docs_sync）；红先绿后有证据义务（T1/T2 red 输出记完成记录）；行为变更面 = 2 枚钉边界黄金改写（D12 逐枚披露），其余 121 枚零改写为回归护栏。
7. **变更粒度**：恰当。裁决 Q1（一里程碑一变更、五章共享表达式层拆开互相悬空）；无规范增量故无 spec 层拆分义务；影响面单一（`we check` 接受/拒绝面）。

**结论：通过**（2026-09-05）。两项审计期勘误已回写 design（D3/D4/D10）与本 proposal（非目标 + What Changes）；follow-up #7/#8 待归档时登记 roadmap。

## 审查记录（welang-change-review，2026-09-05，ready → active）

1. **proposal 职责边界**：通过——Why 为真机黑盒缺口，目标为接受/拒绝面描述，机制细节以「design Dx」指位不展开；无任务清单混入。
2. **spec 增量边界**：无 spec 增量（internals-only，豁免标记 + validate --strict 过）；「不改变语言行为」句在位，一切判定以已批章文 + 注册表为据。
3. **design 质量**：通过——唯一可实施路径（每决策单选型）；被拒替代有录（D11 不扩 IR 的论证、D6 newtype 类别字面读法的归谬、Q2/Q4 裁决记录）；引用精确到章 R 与注册表码；与章文无矛盾（审计第 3 点逐码核验承接）。**发现 F1（真缺陷，已修复）：E0602 勘误（注册表描述只覆盖 type/expression）审计期只回写了 design，proposal 目标 3 与 tasks T2/T4 仍残留「三处」旧读法——审查期 grep 抓出，三处当场改为「两处 + 模式超元走 E0501」，与 D3/D4/D13 黄金表（两枚 E0602 负例）一致。**
4. **tasks 职责**：通过——T1–T10 全部来自 proposal 目标/design 决策，来源/验证行完整（无 `- ` 续行标记）；grep 无 deferred/非目标/未决字样；F1 修复同步。
5. **场景覆盖**：通过——normal（~22 正例分组）、boundary（边界黄金改写 2 枚 + build 停界代表）、failure-degradation（~49 逐码负例 + 4 项 falsifiability 披露的真机核对）三路齐备。
6. **无空章节**：通过——四工件各节均承载决策内容。
7. **测试先行**：通过——T1 黄金先红（对当前构建跑失败清单）、T2 单测先红（引用未定义节点失败），证据义务写入验证列。
8. **负向断言**：通过——每新触码至少一枚负例黄金（消息逐字节）；对抗矩阵强制假穷尽反例 `(Some(a), None) | (None, Some(b))` 必须报 E0305（T6 验证列点名）；或模式同名异型、捕获三类别、E0816 均有构造违规样例。
9. **完成度闭环**：通过（显式处置型）——类型检查 D6–D10 全量；代码生成 D11 最小增量 + 裁决 Q2 记录不扩 IR 的理由（GC 设计未定、可观察收益为零）；运行时零触达（静态前端里程碑，运行时语义列非目标并指明归属里程碑）。三要素无沉默缺失。
10. **未决问题阻塞**：通过——四项表面裁决（Q1–Q4）全部落定并回写裁决记录；let 重赋值与 E0604 描述子句两个规范/注册表缺口均有显式处置（维持现状 + follow-up #7/#8），不构成行为未决。

**结论：通过，激活**（2026-09-05）。F1 已闭环；进入实现（T1 起）。

## 审查记录（welang-code-review，2026-09-05，active → complete）

1. **规范符合性**：通过。实现行为对照已批章文与注册表：**201 枚黄金全绿**（T8 缓存清空重跑 8 包零失败）；真机电池 **37 枚逐码负例**码/锚位/报文全过（ch3/ch4 句法 14 + ch8/成员/E05/E06/ch12 23）＋绿静默程序一枚覆盖五章形式（check 无输出 exit 0）。机制读法两处（穷尽性 = Maranget 有用性、捕获类别 = catOf 派生）为审计第 4 点核准的判定器实现——假穷尽反例于真机命中 `the value shape "(Hit, Hit)" is not covered`，积空间推理在位。无规范外行为：赋值目标可变性维持 M3 现状（follow-up #7）、记录未知成员走 E0816（章文权威，follow-up #8 登记注册表子句维护）。
2. **验证诚实性**：通过。T1–T8 逐项核对，验证命令均真实运行且结果相符——抽查 /tmp/t1-red.log 实存且**恰 78 个子测试失败**（与完成记录「78/80」精确一致）；/tmp/t2-parser-red.log 实存（`undefined: ast.While/Loop` build 失败）；/tmp/t8-all.log 为 `go clean -testcache` 后重跑。披露义务全部兑现：T2 镜像钉位修正 12 处、金样按 D12 计划改写 2 枚、E0501-newtype 金样锚位修正 1 枚（违反既有惯例的 T1 生成器笔误）、T8 电池脚本自身 2 缺陷——均记于对应完成记录。无「勾了但没跑」项。
3. **测试先行**：通过。T1 黄金先红（78 失败对 M4 基线）、T2 单测先红（引用未定义节点 build 失败），日志在案；红→绿经 T3–T7 各步 conformance 增量（135→139→158→188→201）逐步佐证。黄金期望输出符合 §52 人类可读格式与 §62 JSON Lines 协议（--json 真机探针全字段核对，T5/T6/T7 记录在案）。
4. **诊断协议稳定**：通过。internal/diag、internal/cli、docs/spec/diagnostics.toml **零改动**（git status 核对）；--json 字段零删改；涉触码全部为已批注册表在位码，零新增/重命名（影响层「无规范增量」承诺兑现）；消息以注册表 title 起头——e0102 金样曾在 T1 当场抓出漏 title 首版，纪律有机械护栏。
5. **单一权威**：通过。变更目录无滞留长期事实：章文本就位于 docs/spec/（规范先行，零 spec 增量）；执行状态翻 roadmap（归档动作随行）；D8 Maranget 与 D9 捕获账本为实现机制记录于归档 design——**不构成 ADR 义务**（非规范留白处的原则取舍，而是已批 Requirement 的判定器选型；被拒替代「逐臂并集计数」以假穷尽反例钉死于测试）。代码注释全英文（`git diff` 与两个 m5_test.go 中文扫描零命中）；变更工件中文、规范引用英文，符合 ADR-0001 双语纪律。
6. **红线复核**：通过。git status 无 refr/ 路径（提交尚未发生，钩子在位）；diff 范围与影响范围承诺**逐文件吻合**——8 修改（ast/parser/typecheck/codegen + 2 既有测试 + 2 计划内改写黄金）+ 81 未跟踪（78 新黄金 + 2 m5_test + 变更目录），121 + 78 + 2 = 201 账目闭合；internal/cli、runtime/、internal/lex、internal/diag、docs/spec/ 零触碰；非目标无越界（build 维持 M4 接受集——真机 exit 70 复验、IR 零声明行复验）。
7. **最小可信验证已跑**：通过。按 welang-pre-push-checks 以实际 diff 选取：编译器代码 → `go build ./...` + `go vet ./...` + `go test ./... -count=1`（8 包全绿）＋逐码黄金；触 CLI 输出面 → 协议核对经黄金全量＋--json 真机探针；文档/工件 → `validate.py --all --strict`（OK: 1 change(s) valid; registry clean）+ `docs_sync.py --check`（30 对齐，对数不变）+ `git diff --check` 干净 + `gofmt -l` 空。无跳过项、无降阈值。

**结论：通过，complete**（2026-09-05）。过程发现（非阻塞，均已披露）：T8 电池脚本 rc 捕获缺陷与 8 处预期锚位手数错位——报文与实现锚位全部一次正确，错在脚本预期侧。归档随行：roadmap M5 行翻 done、follow-up #7（赋值目标可变性）/#8（E0604 注册表陈旧子句）/#9（we run 项目外调用路径解析）登记（双语对同步）。
