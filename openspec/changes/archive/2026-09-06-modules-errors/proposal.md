# proposal — modules-errors

## Why

M6 拆分裁决（M6a 归档记录）：泛型塔（`generics-iterables-collections`，已归档 2026-09-06）先行铺地基，本变更 **M6b 纪律塔** 收尾原 M6 的余下三章——第 13 章 resource（`docs/spec/1300-resources.md`）、第 14 章全量（`docs/spec/1400-errors.md`）、第 15 章模块解析全量（`docs/spec/1500-modules.md`）。三章共享一个主题：**跨边界的义务纪律**——资源义务跨函数边界（签名携带）、错误值跨函数边界（Result 在签名）、名字跨模块边界（pub 面）。它们互为地基：`?` 的早返回走 ch13 返回通道（资源义务随行）；scope resource 的 panic 展开走 ch14 unwinding；多模块把 E1304 名字解析从单模块扩到合格名。`we check` 的前端覆盖面在 M6b 后到达「单进程语言全部已批章」——余下 ch16/18/19/20 均为后端或大表面章。

规范先行（22 章全批），本变更**无规范增量、不改变语言行为**（章文与注册表早已批准冻结；纯实现，接受/拒绝行为全部由既有章文与注册表钉死，零新增/修改诊断码定义）。

## 裁决记录（candidate 阶段四项表面裁决，2026-09-06，均采纳推荐）

1. **Q1 ch13 线性纪律分析深度 = 结构化控制流活跃性**：语句序贯 kill/gen、if/else 双臂并集、while 回边保守近似（并集、只保证「至少一转移 + 至多一次使用」双向义务）、break/continue 穿透内层块出口。被拒：直线保守近似（违反 ch13:101「Conditional paths each transfer」场景）、全数据流位集（无 goto 下同效、重三倍——M5 Maranget 树上分析同构先例）。
2. **Q2 ch14 panic 族 = 预导入真签名**：`panic(msg: String) -> Never`/`todo(msg: String) -> Never`/`assert(cond: Bool, msg: String)` 三签名入 syms 层，调用走普通检查（实参型 E0501 既有；实参个数 = bndArityGap follow-up #5 维持），Never 满足任意返回位（ch9 豁免既有）；`bndTermination` 行删除。运行时中止/展开归 codegen 非目标（检查/运行时分离，M3 惯例）。被拒：维持边界至 M8（检查期全程可静态判定，半量推迟无技术依据）。
3. **Q3 ch15 多模块 = 全量**：路径映射装载 + import 图 DFS 环检（E1301）+ E1302 项目形（报文含期望路径）+ 跨模块 pub 面 E1303 三位 + 合格名 E1304 扩 + 后序检查次序 + 初始化序编译期确定性计算；`bndMultiModule` 行删除。被拒：最小双模块样例（E1301/E1303 两首发码落空、bndMultiModule 只能收窄不能删、M6b 收不齐 ch15「全量」承诺）。
4. **Q4 M6a 遗留三项精化 = 纳入本变更**（T8）：跨子句同 idx 撞名统一化显式检查、泛型 impl 覆写组合子（map/fold 携 U 子句）的指标空间核对、impl 自身 where 的应用点满足（E0830 新位）——三项全复用既有码，各落黄金钉死。被拒：登记 M7+（已知不精化面积压，每轮里程碑多一轮披露负担）。

## 现状与差距

- **已在发**（4 枚）：E1204（M3 落，Result 错误位命名 sum）、E1302 单文件形（无 source root）、E1304 单模块形（裸名/限定符非 import 名）、E1305（main 形状）。
- **首发**（11 枚）：E1101–E1106（资源六码）、E1201/E1202/E1203（`?` 三码）、E1301/E1303（循环依赖、跨模块局部项）。约为 M6a（34 枚）的三分之一——但机制单枚更重（见下）。
- **边界行待删/收窄**：typecheck 侧 `bndTermination`（panic 族调用）、`bndMultiModule`（多模块）；parser 侧 `bndErr`（`?` 后缀——ch14 全落）、`bndScope` **收窄**（「chapter 13 and 18 (scope) forms」合盖两章：`scope resource(...)` 形落地，bare/timeout/collectAll 的 ch18 形独立成行——What 文本拆分）。
- **维持不动**：`bndStdModules`（`import std.*` 与非锚定 stdlib 成员——std 段永不文件系统解析，内容是 M8 标准库的；3 枚黄金 check-bnd-std-import/check-bnd-std-member/check-parse-clean 钉住原文本，零改写）、`bndShareable`/`bndTaskTime`（ch18/20）、`bndEffect`/`bndMutPar`（ch16——M7）。
- **既有黄金触碰预估**（D12）：`check-bnd-multi-module`（多模块落地 → 改写为多模块真行为黄金）、`check-bnd-panic`（panic 族真签名落地 → 改写为真调用行为黄金）——约 2 枚，逐枚披露；其余 296 枚零改写（硬门）。

## 目标与非目标

**目标**（8 项）：

1. **ch13 声明层**：内建接口 `Releasable`（`fn release(mut self)`）登记；完备性义务双向判定——`byres record` 未实现 Releasable → **E1101**；非 resource 类别实现 Releasable → **E1102**；`byres record` 获得资源类别（catOf 已有，绑定纪律的判定地基）。
2. **ch13 scope resource 语句**：`scope resource(name = expr, ...) block` 产生式（≥1 绑定、无注解槽、`scope`+`resource` 双关键字头）；头表达式型须实现 Releasable → **E1103**；绑定块作用域；语句位值律 E0202 既有。
3. **ch13 线性纪律**：结构化控制流活跃性分析（每个资源绑定在每条控制路径上恰达一次三通道转移——scope 头转移/return/实参传递）——落空、转移后使用、顶层绑定、`release` 直调 → **E1104**（单码多触发位，注册表 description 分项枚举）。
4. **ch13 别名与组合位**：`let g = f`/`var f`/赋值两侧 → **E1105**；元组元素/gc 字段/sum 载荷/newtype 底层/泛型实参/Dyn<Releasable> 两形 → **E1106**（byval 字段位归 E0601 既有，披露）。
5. **ch14 `?` 传播**：操作数须 Result → **E1201**（Option 不传播）；语境枚举（非 Result 返回 fn/模块顶层/defer 体）→ **E1202**；E_src ≡ E_dst 同命名类型 → **E1203**；Ok 解包载荷型、Err 早返回（ch13 返回通道）、短闭包 `?` 定型值型 `Result<U, E_src>`、postfix 链级 1。
6. **ch14 panic 族**：预导入真签名——`panic(msg: String) -> Never`、`todo(msg: String) -> Never`、`assert(cond: Bool, msg: String)`；调用走普通检查、Never 满足任意返回位（ch9 豁免既有）、`bndTermination` 删。
7. **ch15 多模块**：路径映射（段→目录、末段 `.we`、相对 `src/`）+ import 图 DFS 环检 → **E1301**；项目形未解析 → **E1302**（报文含期望路径；依赖缓存路径未落 = not found 同码）；跨模块 pub 面 → **E1303**（值位/类型位/变体构造器位，接口值到达合法）；合格名解析 → **E1304** 扩（import 名限定 + pub 项）；初始化序 = import 图后序的编译期确定性计算（无诊断面；运行时执行序归 codegen 非目标，披露）。
8. **M6a 遗留三项精化纳入**（其完成记录披露的 M6b 候选）：(a) 跨子句空间同 idx 撞名的统一化兜底；(b) 精确 impl 覆写带子句默认方法的指标空间核对；(c) impl 自身 where 的应用点满足判定——各落黄金钉住。

## What Changes

见「目标与非目标」八项——本变更零规范增量，无 ADDED/MODIFIED Requirements 面；实现层变更 = parser 产生式（scope resource/`?`）+ typecheck 五层（声明/语句/纪律/传播/多模块）+ cli 装载层（多模块路径映射）。

## 影响层

- **layers**: [compiler]（`we check` 前端；`internal/parser` + `internal/typecheck` + `internal/cli` 的 check 装载层）
- **非目标**：codegen/运行时（scope 释放调用、`?` 的 Err 早返回代码、panic 中止、多模块初始化执行——`we build` 维持 M4 接收集，`bndMainBody`/`bndOtherFns` 兜底，M5/M6a 同构披露）；`std.*` 模块内容（M8）；依赖缓存/获取（ch22 工具层）；ch16 效果段（M7）；ch18/20（scope 并发形、task、test 块——bndScope 拆行后 ch18 形独立停）。
- **规范增量**：无（22 章已批；本变更不改章文与注册表——15 码全部在册）。

## 影响范围

**涉及层**：compiler（前端三包）；**涉及章**：ch13/ch14/ch15（全部已批，零修订）；**M6a 遗留**：三项精化（D10）。**非目标边界**见影响层节。

## 涉触码盘点

段内 15 枚（E1101–E1106、E1201–E1204、E1301–E1305）中 11 枚首发、4 枚已在发（新发位扩 E1302 项目形/E1304 合格名）。复用码新位：E0105（catch/recover/try/expr!/`try expr` 无产生式——章文明文走 E0105，黄金锁定）、E0202（scope resource 语句值位）、E0204（scope 块内 defer）、E0601（byval 字段资源位）、E1002（闭包捕获资源——M5 已落，黄金已有）、E0903（资源实现 Iterable——M6a 已落）、E0808（release 签名错——ch10 既有）。全部在册。

## 已知风险与开放问题

- **E1104 报文的路径渲染**：注册表「naming the binding and the path」——控制路径的描述形是消息设计义务（D4 定形，如「on a path reaching the end of its scope」）；黄金逐字钉死后冻结。
- **结构化流分析的边界**：We 无 goto（ch3 封闭控制集），语法树即流图；`if/else` 双臂汇聚、`while` 回边、`break/continue` 穿透内层块各有场景钉住。循环内绑定（每迭代新绑定）与循环外绑定的交互由场景定形。
- **多模块装载架构**：checker 现为单 `*ast.File`；D9 定多模块图装载（每模块独立 parse + 跨模块符号表 + 根模块 main 判定）。conformance runner `setup.files` 已支持多文件黄金（M1 落）。
- **短闭包 `?` 定型**：`|s: String| parse(s)?` 的值型固定为 `fn(String) -> Result<尾型, E_src>`（ch14 场景钉死）——与 M5 闭包期望线程的交互在 D7 细化。

## 审计记录（2026-09-06，welang-spec-impact-audit 7 条，通过）

1. **问题真实性：通过**。缺口 = roadmap M6b 行承诺（`docs/roadmap/0000-reference-implementation.md`——M6 拆分裁决落地行）的三章前端：当前 `scope resource(...)`/`?`/panic 族调用/多模块 import 全部停边界 exit 70，E1101–E1106/E1201–E1203/E1301/E1303 十一枚在册码零发射（grep 实证含补核对的 E1104）。章文引用固定（1300-resources/1400-errors/1500-modules）。
2. **影响层声明：通过**。change.yaml [compiler] 与影响层节一致；纯内部变更豁免标记在文（「无规范增量、不改变语言行为」）；validate.py --strict 结构合法。
3. **规范增量范围：通过**。零新增/修改诊断码——15 枚段内码全部在册（diagnostics.toml 对照：E1101–E1106/E1201–E1204/E1301–E1305）；复用码新位（E0105/E0202/E0204/E0601/E0808）全在册；无 active 变更冲突（当前唯一 active 即本变更）。
4. **原则一致性：通过**。三章是 P1（局部可判定）的直接运营化——线性义务跨函数走签名、错误值走签名、名字走 pub 面；无原则突破面。
5. **参考基线固定：通过**。全部引用 docs/spec/ 已批权威文本与注册表；无 refr/ 草案引用。
6. **验收边界：通过**。8 项目标机械可判（11 首发码可发射 + 黄金全绿 + 四边界行删除/收窄）；非目标显式排除 codegen/std 内容/依赖层/ch16/ch18-20 面。
7. **粒度：通过**。单层（compiler）垂直可验（`we check`）；三章一组是 M6 拆分裁决（M6a 归档记录）+ Why 节互为地基论证（`?` 早返回走 ch13 通道、scope 展开走 ch14、E1304 合格名扩到多模块）——同 M5/M6a 一变更多章先例。

发现与处置：无。E1104 补核对（审计中）——实现零触及，首发盘点 11 枚准确。

结论：**通过**，status candidate → ready。

## 审查记录（ready → active 关卡，welang-change-review 10 条，2026-09-06）

**职责边界**：

1. **proposal 只讲黑盒问题与目标：通过**。八项目标全部为行为面（码可发射 + 黄金钉死 + 边界行删除）；现状差距是黑盒盘点（grep 实证在发/首发/边界行）；无实现路径泄漏（无 AST 节点名、无 pass 编号——「pass 2a/2b」仅出现于 design）。裁决记录四项是表面裁决（选型与被拒理由，M6a 同位先例）。
2. **规范增量与章文一致：通过（豁免形）**。零规范增量（无 specs/ 目录），proposal 豁免标记在文（「无规范增量、不改变语言行为」）；15 码全部在册对照过（审计第 3 条），本章文引用（R1–R6/场景名）与已批文本一致。
3. **design 唯一最小路径 + 被拒替代 + 引用精确：通过（修正后）**。D1–D10 每条单一选型 + 被拒替代及理由（D11–D14 为接收集/改写/黄金/披露表，无需替代）；引用精确到行号并逐处实锚复核（见发现与处置）。审查中修正 7 处行号漂移（漂移不改变论点——所引场景/条文本身存在且支撑论点）。
4. **tasks 只执行前三者、来源与验证齐备：通过**。T1–T11 每条带来源（proposal 目标号 + design D 条）与验证命令；无超出 design 的新决策；T10 归档目录日期占位（2026-09-XX）沿 M6a 先例（执行日记前缀，T10 完成记录披露实值）。

**内容质量**：

5. **场景覆盖 normal/boundary/failure：通过**。D13 黄金计划三类齐备——正例 ~20（三转移各形绿例、短闭包定型、多模块 pub 面、接口值到达合法、初始化静默）、负例 24+8（11 首发码 ×形 + 复用码新位）、边界（E1302→E1301 依赖序负例对、build 双停 bndMainBody/bndOtherFns、E1301 环渲染）；D4 四形/D5 三形/D6 六位/D7 三码逐形枚举无漏。
6. **无空章节、无占位：通过**。What Changes 节引用目标节而非复制（零规范增量下的诚实形）；四工件无 TBD/待定；「已知风险与开放问题」四项全部指向 design 已定形条目（D4/D7/D9）非开放占位。
7. **测试先行：通过**。T1 黄金先红（对当前构建，red 证据入完成记录）+ T2 单测先红（parser/typecheck 引用未定义节点失败），先于 T3+ 实现；M3–M6a 同构。
8. **负向断言：通过**。每首发码至少一负例黄金（报文逐字钉死）；E1104 四触发位分形、E1303 三位、E1106 六位逐位负例；T8 三项精化各有「期望 E0501/E0830 而非静默」的反沉默断言；D14 不可达清单逐条给出「为何测不到」的机制理由（非跳过）。
9. **完成度闭环（类型检查/代码生成/运行时三要素）：通过**。类型检查 = design 主体（D2–D9 五层全量，`we check` 是验收面）；代码生成 = D11 维持 M4 接收集（What 表四行原样 + bndMainBody/bndOtherFns 兜底，真机验证 T9）；运行时 = 非目标显式（scope 释放执行/`?` 早返回代码/panic 中止/初始化执行归 codegen 与 M8——静态检查里程碑立场，M5/M6a 同构自洽）；三要素各有归宿无悬空。
10. **未决问题不阻塞：通过**。无 SHOULD/MAY 掩盖决策——四项已知风险全部由 D 条定形（E1104 报文 = D4 定形后黄金冻结；流分析边界 = D4 保守性披露；多模块架构 = D9 定形；短闭包定型 = D7 定形）；E1104 报文渲染的唯一真开放项（消息文案本身）由黄金逐字钉死机制收口，不阻塞实现。

发现与处置：

- **发现 1（章文行号漂移，7 处）**：design/proposal 引用行号与现行章文不符——ch13:66→101（Conditional paths 场景）、ch13:106→113（报文逐字）、ch13:120→122（三形并列）、ch13:139→141×4（组合位/b instantiation site）、ch15:9→6（E1302 报文义务）、ch15:84→64/89（接口值到达）、ch15:19→6（std 不文件系统解析）。**处置：全部修正，脚本逐处实锚复核 7/7 命中**（另核对无漂移 3 处 ch13:10/26/74 亦实锚）。定性：表面精度问题，所引条文与场景均存在且支撑论点，无语义影响。
- 其余九条无发现。

结论：**通过**，status ready → active。

## 审查记录（active → complete 关卡，welang-code-review 7 条，2026-09-06）

1. **规范符合性：通过**。三章判定逐面有章文依据：ch13 E1104 直调形报文 = ch13:113 逐字（黄金钉死）、release 签名错走 ch10 既有 E0808（D14 披露、黄金锁定）；ch14 `?` 判序 = D7 (a)→(c) 与 ch14 语境枚举（defer 体覆盖外围 fn——黄金 e1202-defer 实证）；ch15 E1302 报文含期望路径（ch15:6 义务）、E1303 只管直接名到达（ch15:64——接口值到达黄金正例锁定）、std 段不文件系统解析（ch15:6）。11 首发码报文 title 与注册表（docs/spec/diagnostics.toml [diagnostic.Exxxx] 条目）逐字一致（E1101/E1104/E1202/E1301/E1303 抽查 + 356 黄金字节级对等）；helpE1301/helpE1302 = 注册表 remediation 原文。实现无规范外行为；D14 八条不可达面真机逐条实证（T9 记录）。
2. **验证诚实性：通过**。T1–T9 逐项核对：T1 red 日志在卷（/tmp/t1-red.log 保留，56 红对账 T1 记录）；T3–T8 各轮 conformance 双向对账（54→42→26→13→2→0）与披露义务随记随落；T9 今日全量重跑——`go test ./...` 全绿（T8 清缓存绿在卷）+ 真二进制黄金对等 356/356 + D14 十探针全符（/tmp/m6b-t9/ 脚本两份在卷）。既有 298 枚零改写以 `git diff --stat internal/conformance/testdata/` 复核（恰 2 枚披露改写）。
3. **测试先行证据：通过**。T1 黄金对当期构建先红（54 预期红 + 2 改写，T1 记录逐枚审计）、T2 单测先红（parser 编译错 `undefined: ast.ScopeRes/ast.Prop` 恰为 D1 新节点、typecheck 七函数红因正确——M2/M5/M6a 同构）；58 新黄金 stderr 全量核对 §52 人类可读格式零违规（仓库既有非诊断行均属 M0 期工具面黄金——usage/run 输出/无位工具链诊断，非本变更面）。
4. **诊断协议稳定：通过**。`git diff HEAD -- internal/diag/` 零文件——诊断包未触及，`--json` 字段集不动；零新增/修改诊断码（11 首发码 + 4 已发位扩均为注册表既有条目，纯实现变更）；E1302/E1304 新报文自动落入既有协议渲染。
5. **单一权威：通过**。零规范增量（三章规范先行已批，docs/spec/1300/1400/1500 在位）——无 specs/ 提升面；代码注释纯英文（internal/ 非测试 .go 中文 grep 零命中）；变更工件（proposal/design/tasks）中文合规；长期取舍随 design.md 归档留存（M6a 先例，无 ADR 级政策变更）。
6. **红线复核：通过**。refr/ 在 .gitignore（`git check-ignore` 命中）且 porcelain 零出现；提交将按英文无署名 trailer 执行（T11）；diff 范围 = 11 修改（ast/parser/typecheck/codegen/cli 及其测试）+ 5 新文件（resource.go + 三单测文件 + modules_test.go）+ 58 新黄金 + 2 披露改写 + openspec 变更目录——与变更目标一致无越界。
7. **最小可信验证：通过**。gofmt -l 空；go build ./... / go vet ./... 过；go test ./... 全绿（conformance 356/356、typecheck/parser/cli/codegen 套件）；`python3 openspec/tools/validate.py --all --strict` OK（1 change valid, registry clean）；`python3 openspec/tools/docs_sync.py` 30 对齐（对数不变）；`git diff --check` 干净；真二进制（/tmp/we-bin）黄金对等 356/356 + D14 探针 10/10。

结论：**通过**，status active → complete，进入归档（welang-archive-sync：roadmap M6b → done 双语 → git mv 归档 → status archived → 复验）。
