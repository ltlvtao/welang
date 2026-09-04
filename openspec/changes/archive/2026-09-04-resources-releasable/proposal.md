# Proposal: resources-releasable

## Why

类型六片队列（foundations → composites+ownership → sum+match → interfaces+generics → iterable-protocols → fn-types+closures）已全部落定（HEAD affe237）。资源纪律是规范正文当前的最后一处成章悬置——已落定文本为本片留位的引用逐一兑现：

- ch8:114「A resource record MUST be released through its Releasable implementation; the release operations, their placement, and the enforcement diagnostic are the resource chapter's」——释放操作、放置点、执法诊断三债按章指名本片
- ch8:62 byres 记录类别自批准日起悬置其释放语义（E0606 更新拒绝已生效，释放通道从未存在）
- ch3:81「it is not the resource-safety guarantee, which the resource chapter ratifies separately」——defer 与资源安全的边界按句指名本片
- ch1:58 keyword 增补先例链（composite 五词、`type`、interface 四词）——`scope resource` 循此进入，破坏性变更照录
- ch2:65 语句族枚举（binding/assignment/expression/control-flow/for）——scope resource 语句循「A statement family grows」场景进入
- ch6:5 模块顶层 let 存在——资源绑定在顶层无任何通道，本片以 E1104 关死
- v0.8 §35.1–35.2（Releasable 接口、scope resource 规则 1–4：三种易手方式、跨函数路径分析）；代码映射 E0726→E1101、E0727→E1102、E0728→E1103、E0729→E1104；W0724 类警告未获批准
- E0204 remediation 现文「or release the resource through the mechanism the resource chapter ratifies instead of a nested defer」——机制获名后本片同变更维护为实名 `scope resource`

边界：ch8:114 只定类别与更新排除，本片不动 E0606；E1002（闭包捕获）/E0903（Iterable 实现）已由 ch12/ch11 落定，本片在组合位禁令中引用而不变更；v0.8 §35.3 request-context 非本片范围。

## What Changes

1. **新增第 13 章 `1300-resources.md`「Resources and Releasable」**（段位 E1100–E1199）：7 条 ADDED Requirements / 30 个 Scenarios——Releasable 接口（std 声明、`fn release(mut self)`、无关联类型；byres 必实现 E1101 声明完备性在声明模块查、ch10 孤儿规则钉住 impl 位置；仅 byres 可实现 E1102；签名偏差走 ch10 E0808）、scope resource 语句（`scope resource(name = expr, ...)` k≥1、无注解位、头左到右求值、头类型须实现 Releasable E1103、块尾逆向逐绑定恰一次 release、return/break/continue 穿透即块出口、体内 defer 走 ch3 E0204、块出口 release 先于外围函数 defer、非表达式）、释放纪律（函数局线性：参数/let/scope 头绑定自绑定点活、每条控制路径恰达三通道之一——头转移（源绑定死于转移点）/ return 上交 / 实参传递给参数声明该资源类型的 fn；未释放落空或转移后使用 → E1104；模块顶层资源绑定 → E1104；分析仅函数体流图、义务由签名携带、局部可判定（chapter 0, Principle 1））、单一释放触发点（用户码任何表达式位直调 `release` 且接收者资源类型 → E1104；早释放=早出口；关死双重释放缝）、无别名（`let g = f`/`var g = f`/赋值任一侧资源类型 → E1105；`var` 声明资源类型 → E1105；唯一合法移动是转移）、资源只栖绑定位（tuple 元素、gc 字段（byval 字段系 E0601 自有）、sum payload、newtype 底型 → 单码 E1106；泛型实参一律拒绝（`Box<FileHandle>`、`id<FileHandle>` 同拒——泛型体内 maybe-resource 义务非局部可判定，绑定位精化留本章段内修订）；`Dyn<Releasable>` 类型位与装箱构造同拒（ch10:227 盒为 gc 共享句柄，装箱绕穿单句柄与单触发点）；引 E1002/E0903 同理已禁）、诊断段位（E1101–E1106 六码、E1100 与 E1107–E1199 留段内修订）。
2. **ch1 Keywords MODIFIED**：枚举列表 += `scope resource` 行；增补段 += 资源章句（两词自本列表修正起为关键字——`scope` 将引导更多复合形式，其预留先于使用如 `mut`）；+1 场景「The resources keywords are reserved」，其余 5 场景逐字继承；破坏性变更照先例记录。
3. **ch2 Statements MODIFIED**：语句族枚举「the iteration chapter ratifies the for statement」后 +=「; the resources chapter ratifies the scope resource statement」；`scope` 以关键字身份落入 ch2 既有 statement-start 类（类含 keywords——无需 Line-joining 修订）；6 场景逐字继承。
4. **ch3 Defer MODIFIED**：资源安全指针句「which the resource chapter ratifies separately」→ 落定引用（第 13 章 scope resource、其块出口 release 先于外围函数 defer、内块先出）；3 场景继承（仅正文句变更）。
5. **ch8 Resource records MODIFIED**：悬置句「the release operations, their placement, and the enforcement diagnostic are the resource chapter's — this chapter fixes only the category...」→ 落定句（ch13 的 Releasable 契约、scope resource 语句、线性释放纪律；本章定类别与更新排除）；场景 1 THEN 尾句同步；2 场景继承。
6. **注册表**：段位 E1100–E9999 未认领尾部拆分为 E1100–E1199（owner 1300-resources）+ E1200–E9999（未认领）；新增 E1101–E1106 六条目（85→91）；E0204 remediation 同变更维护（「the mechanism the resource chapter ratifies」→「`scope resource`」实名）。
7. **示例 D14 刷新（EN+zh，披露）**：ch8 示例 pending 注释（release operations 与 Releasable enforcement 系 resource 章之债 → 已落定）。

## 裁决记录（用户 2026-09-04 五项，均采纳推荐）

1. **函数局线性纪律**：资源绑定的每条控制路径恰达三通道之一（scope 头转移 / return 上交 / 实参传递给参数声明该资源类型的 fn）；未释放落空或转移后使用 → E1104。分析仅覆盖函数体流图，跨函数义务由签名携带——v0.8「跨函数路径分析」由此落地为局部可判定（chapter 0, Principle 1）。模块顶层不绑资源（顶层 let 资源类型 → E1104）。备选「跨函数全局流分析」（v0.8 字面）被否——非局部可判定，违 P1；备选「region/lifetime 标注系统」被否——本规范无任何标注文法，语言面过重。
2. **仅三通道、禁重绑**：资源仅经 scope 头、实参、return 三通道易手；`let g = f` 别名 → E1105；`var` 声明资源类型 → E1105（资源绑定一律 let 形）。备选「借用检查/多所有者」被否——与 ch8「reference-passed, with explicit lifetime management」的单句柄立场冲突，且借用属未批准的标注系统。
3. **禁入一切组合位**：tuple 元素、gc 记录字段、sum payload、newtype 底型、泛型实参一律（`Box<FileHandle>`、`id<FileHandle>` 同拒——「一切」的字面；泛型体内 maybe-resource 义务非局部可判定，绑定位精化留本章段内修订）、`Dyn<Releasable>` 类型位与装箱构造，统一单码 E1106。E1002（闭包捕获）、E0903（Iterable 实现）已禁同理，本片引用不变更。备选「按位分码」被否——同一拒绝理由（多路径句柄越过单一确定性释放点），单码单义更利于 AI 原生检索。审查补强（F2）：草案原允许参数仅绑定位的泛型实例化，审查发现泛型体流分析不可局部判定，收紧为一律禁——较草案更贴裁决字面。
4. **多绑定 + 已批准流边界**：`scope resource(name = expr, ...)` k≥1 绑定，块尾按声明逆序逐绑定释放；return/break/continue 穿透即块出口同样释放；体内 defer 走 ch3 既有 E0204（defer 仅直接函数体项）；panic 展开不落本片——error 章批准时经本条修订绑定到同一保证，边界披露。Releasable 为 std 接口（ch11「The standard library declares」先例）；W0724 类警告不批准。备选「单绑定形式」被否——多资源同域是常态，单绑定迫使嵌套；备选「本片定 panic 展开语义」被否——展开规则属错误机制章。
5. **禁直调、单触发点**：`release` 仅由块出口机制调用；用户码直调（任何表达式位、资源类型接收者）→ E1104；早释放的唯一拼写是早出口（return/break/continue）。Rust `Drop::drop` 禁手动调用先例。被调方关闭参数走头转移 `scope resource(x = f)`（转移杀死源绑定）。备选「允许显式 `f.release()` + 幂等义务」被否——双重释放缝（手动 + 块尾）由实现方承担，违最小信任面；备选「关闭后置空/标记」被否——引入运行时协议且与禁重绑冗余。

## 目标与非目标

目标：

- 落地 ch8/ch3 指名本片的全部悬置引用，byres 记录获得完整生命周期语义
- v0.8 §35.1–35.2 的 P1 健全落地（函数局线性纪律 + 签名携带义务）
- `scope resource` 语句进入 ch1 关键字表与 ch2 语句族（破坏性变更照录）
- 释放纪律、单一触发点、无别名、组合位禁令四壁合围，双重释放缝关闭
- 注册表段位 E1100–E1199 开段 + 六码落位 + E0204 remediation 实名维护

非目标：

- panic 展开的资源保证——error 机制章批准时经本章 R2 修订绑定（边界已披露）
- W0724 类「资源未沿全部路径释放」警告——不批准；纪律以错误执法（E1104）
- foreign 所有权资源（v0.8 E0741/E0742）——foreign 章
- Semaphore/Shared 并发资源——并发章
- 运行时 finalizer / GC 兜底回收——本规范无此运行时协议
- 池化惯用法（pool-owns：资源记录持资源句柄数组）——需组合位解禁，未来修订
- request-context 资源（v0.8 §35.3）——依赖未批准的并发/服务端语义

## 影响层

`spec`：新增 1300-resources.md（+zh）；修改 0100-lexical.md（+zh，1 条 MODIFIED）、0200-grammar.md（+zh，1 条 MODIFIED）、0300-control-flow.md（+zh，1 条 MODIFIED）、0800-composites.md（+zh，1 条 MODIFIED）；diagnostics.toml（段位拆分 + 6 条目 + E0204 remediation 维护）；ch8 示例刷新（EN+zh）。

## 影响范围

- docs/spec/1300-resources.md（新增）、1300-resources.zh.md（新增）
- docs/spec/0100-lexical.md / .zh.md（Keywords 1 条 MODIFIED）
- docs/spec/0200-grammar.md / .zh.md（Statements 1 条 MODIFIED）
- docs/spec/0300-control-flow.md / .zh.md（Defer 1 条 MODIFIED）
- docs/spec/0800-composites.md / .zh.md（Resource records 1 条 MODIFIED + 示例刷新）
- docs/spec/diagnostics.toml（[segments] 拆分 + [diagnostic.E1101..E1106] + E0204 remediation）
- docs sync 对 19 → 20 对（1300-resources.md/.zh.md）

## 审计记录

**2026-09-04 · candidate → ready · 通过（7/7）**

1. **问题真实性 ✓**：Why 全部锚点本会话逐一核实——ch8:114（资源章三债句逐字）、ch8:62（E0606 延期句）、ch3:81（资源安全指针句逐字）、ch1:58（keyword 先例段）、ch2:65（Statements 要求）、ch6:5（顶层 let）、refr/spec-0.8.md §35.1–35.2 + 代码表。附注：ch8:62 尾部「now landed」系 interfaces 片先行写入的预期表述，本片落地后该句变为完全为真，无需另行修订。
2. **影响层声明 ✓**：change.yaml `layers: [spec]` 与 proposal 影响层一致；纯规范层，无 compiler/stdlib/tooling 声明。
3. **规范增量范围 ✓**：7 ADDED（29 Scenarios）+ 4 MODIFIED（ch1 6 场景=5 继承+1 新增；ch2 6 全继承；ch3 3 全继承；ch8 2=1 尾句替换+1 继承）；E1101–E1106 对照 docs/spec 与全部变更 grep 零冲突；段位 E1100–E1199 今属未认领尾部（E1100–E9999），拆分合法；单码单消息前缀边界检查 PASS（消息前置、`;`/`:`/`,` 续理由，ch12 落地体例）。
4. **原则一致性 ✓**：P1 显式落文（「locally decidable (chapter 0, Principle 1)」）；keyword 破坏性变更照 ch1 先例链记录（含 `scope` 预留先于使用的先例论证，design D5）；无隐式转换；单触发点/禁别名与 ch8 单句柄立场一致；panic 边界诚实披露为 error 章修订位。
5. **参考基线固定 ✓**：refr/spec-0.8.md §35.1–35.2 与代码表（lines 2052–2099）固定；D7 映射表 E0726–E0729→E1101–E1104；W0724 显式不批准。
6. **验收边界 ✓**：目标可机械判定（悬置引用清零、6 码落位、段位拆分、E0204 remediation 实名、docs_sync 19→20）；非目标七项（panic/W0724/foreign/并发/finalizer/池化/request-context）各附理由，防蔓延充分。
7. **粒度 ✓**：单垂直单元——第 13 章 + 四宿主修订 + 注册表段位，同服一个验收面（资源纪律）；无跨层。

结论：通过。`change.yaml` status → ready。

## 审查记录

**2026-09-04 · ready → active · 通过（发现 F1/F2 两项真实缺陷，均已修复后复验）**

1. **proposal ✓**：黑盒问题与目标；裁决记录五项含被否备选（house 体例）；无实现决策混入。
2. **spec 增量 ✓**：触发/结果/失败路径齐备；BCP14 用法合规（R1 删去无默认值的 MAY——装箱句改写为禁令陈述）；R3「flow graph alone」句系 P1 边界契约（保护接受性：编译器不得据他函数体拒绝），非实现越界。
3. **design ✓**：D4 补 F1/F2 论证（ch10:227 引条目级、P1 论证），与增量无矛盾。
4. **tasks ✓**：三任务各有来源/验证；无 deferred/未决项。
5. **场景覆盖 ✓**：30 场景 normal/boundary/failure 齐（新增 Dyn 盒拒绝、泛型一律拒两负向）。
6. **无空章节 ✓**。
7. **测试先行 ✓**（spec 层适配：注册表任务哨兵负例注入先行——fn-types/iterables 先例）。
8. **负向断言 ✓**：E1199 哨兵 + 宿主场景集机器比对，非文字代替。
9. **完成度闭环 ✓**（spec 层披露：类型检查=E1101–E1106 触发语义；运行时=R2 恰一次/逆序/先于 defer 的可观察保证；代码生成层不存在——spec-first 仓库，与 fn-types 片同口径）。
10. **未决问题阻塞 ✓**：panic 展开系披露的非目标修订路径（error 章），非本片未决阻塞。

**F1（真实缺陷，已修复）**：ch10:227 Dyn 盒为 gc 共享值且构造仅查实现关系——`Dyn<Releasable>(f)` 可合法构造，盒句柄可别名（共享盒）、`d.release()` 接收者非资源类型绕过单触发点禁令，单句柄纪律整体被绕穿。修复：R1 装箱句改写为禁令陈述；R6 增 `Dyn<Releasable>` 类型位与构造位同码 E1106（E1102 完备性下每个 Releasable 盒必装资源）；+1 场景「A Dyn box of Releasable is rejected」。
**F2（收紧，已修复）**：草案允许参数仅绑定位的泛型实例化（`id<FileHandle>`），但被调体内 T maybe-resource，流分析非局部可判定（违 P1）；收紧为泛型实参一律 E1106，绑定位精化留段内修订——更贴裁决 3「禁入一切组合位」字面。场景 THEN 同步。
**F3（轻微，已修复）**：R3 补显句——break/continue 结束内层块作用域，let 型资源未先转移即未释放落空（E1104）。

处置后复验：validate --strict 绿；7R/30S；单码单消息前缀检查 PASS；proposal What Changes/裁决 3、design D4、tasks 计数同步。结论：通过，status → active。

## 实现审查记录

**2026-09-04 · active → complete · 通过（7/7）**

1. **规范符合性 ✓**：1300-resources.md 由 delta 机器拼装（逐字包含验证 True）；四宿主 EN splice 零外溢（pre/post 与 git HEAD 机器比对全 identical、spliced 区块字节等于 delta）；zh 孪生 7R/30S、代码块 4/4 逐字节一致、术语表 24 行；ch2/ch3 zh 为分句级最小修改。
2. **验证诚实性 ✓**：三任务逐项核对——任务 1：validate --strict 绿、场景机器清点 30+17=47、E11xx 单码单消息 PASS；任务 2：哨兵先行（`E1199:` FAIL 消息原文「diagnostic usage 'E1199:' has no registry entry in docs/spec/diagnostics.toml」已录、还原后 clean）、过渡期 6 项 owner-FAIL 如预期自愈、requirement 字段解析 PASS、条目 85→91；任务 3：docs_sync 19→20、全角冒号扫描零命中、宿主场景计数 5→6/6/6/3/3/2/2 机器比对。
3. **测试先行证据 ✓**：哨兵负例注入先于注册表扩展运行（本会话时序）；spec-first 仓库无 conformance 语料（fn-types/iterables 先例口径）。
4. **诊断协议稳定 ✓**：E0204 仅 remediation 字段变更（tomllib 对照 HEAD：diff 集合恰 {'remediation'}，title/severity/编号不动）；E1101–E1106 全局唯一、同变更登记。
5. **单一权威 ✓**：章与宿主已入 docs/spec；变更目录只存增量与过程记录；诊断消息检查的 5 处命中均为 HEAD 既有的已知检查器误报类（`fires` 短语、zh「拒绝」后缀——fn-types 片已披露），非本片引入。
6. **红线复核 ✓**：refr/ 未触及（git status 与影响范围清单恰一致：9 改 + 2 新 + 变更目录）；无越界改动；commit 信息将不带署名。
7. **最小可信验证已跑 ✓**：validate --all --strict 绿、docs_sync --check 20 对、注册表 13 段/91 条、E1100–E1199 owner=1300-resources、E1200–E9999 未认领。

结论：通过。`change.yaml` status → complete，进入归档。

## 归档记录

**2026-09-04 · complete → archived**

- 规范提升：docs/spec/1300-resources.md（+zh）已落（7R/30S、示例、术语表 24 行）；ch1/ch2/ch3/ch8 宿主四条 MODIFIED 双语落地（机器零外溢验证）；diagnostics.toml 段位 E1100–E1199 + E1101–E1106 六条目（85→91、13 段）+ E0204 remediation 实名；docs_sync 19→20。
- 决策提升：五项裁决无原则例外（P1 为落实而非突破），循 types 六片先例不另立 ADR；裁决与被否备选以本 proposal 为权威留存。
- 状态提升：无 roadmap 目录，留档于本归档。
- 移动：openspec/changes/resources-releasable → openspec/changes/archive/2026-09-04-resources-releasable；status → archived。
- 复验：validate --all --strict 待归档后复跑（见提交）。
