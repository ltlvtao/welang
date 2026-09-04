# Design: 工具链（第 21 章）

## 裁决依据

四项用户裁决（2026-09-04，均采推荐）锁定设计空间：

1. **全量一章**：CLI/管线/fmt/vet/test（含探索与守护码）/doc/JSON 协议/manifest 骨架/链接承接，一章收口；LSP 指独立文档；依赖留白。
2. **点名 W 码入册**：只注册 W0601/W0755/W0756 三个点名承诺码（重编号）；其余检查项入章不入册。
3. **manifest 骨架+显式留白**：name/version/type + `[vet]`/`[test]`；获取故事显式留白。
4. **字段稳定性入章**：JSON Lines 协议作为可观察契约。

## 设计决策

### D1 章的立场：可观察契约，不是实现规范

全章一条纪律：**只固定可观察的行为契约**（命令面、止损规则、输出形状、退出码、失败码），**不固定机制**（增量策略、缓存、调度算法、mangling、搜索路径、目标词汇）。这是 ch0 机制中立纪律的工具链面：规范定"观察到什么"，实现定"怎么做到"。性能零承诺——同输入同输出是唯一承诺，时长/足迹归评测层（FPCR 方法论在仓外评审文档，规范不引）。

### D2 零宿主修订的论证

七处让渡句（ch0 P7/机制中立、ch6 文档渲染、ch15 manifest/获取、ch18 vet 层、ch19 链接、ch20 运行形状/探索、ch99 共号 space/本地化）全部是**前瞻指针**——写的是"归工具链层"这一归宿，不是具体内容。本章落地后每句依然为真：归宿说对了，内容现在有了。与 effects/concurrency/testing 切片的宿主修订不同：那些章引入新语言表面（关键字、语境、边界），宿主的枚举/清单必须扩；本章**零新语言表面**（无关键字、无文法、无类型），故零宿主 Requirement 改写是诚实的最小增量，不是遗漏。审计第 3 点将逐句 grep 验证"落地后依然为真"。

### D3 E/W 共号 space 与段位设计

ch99 规定 "`E` and `W` share one number space: a given four-digit number exists with at most one severity at a time"。推论：W 段位不能独立成段复用已占号码（W0100–W0199 会与语法章 E0100–E0199 冲突）。设计：**单段 `E1900`–`E1999` owner `2100-toolchain` 同时容纳 E 码与 W 码**——错误取 `E1901`–`E1907`，警告取 `W1910`–`W1912`，号码不相交，一段一主。`E2000`–`E9999` 续 unclaimed。W 在段内不是前缀装饰而是 severity 标记：`[diagnostic.W1910]` 的号码 1910 落在段 1900–1999 内，满足"每条目必落在 owner 的段内"。

### D4 十码的取舍账

入册十条，每条有规范文本的触发承诺：

| 码 | title | 触发归 |
| --- | --- | --- |
| `E1901` | exploration detected nondeterminism | R6 探索 |
| `E1902` | unmocked effect executed during exploration | R6 探索 |
| `E1903` | invalid toolchain configuration value | R8 manifest（`[vet]`/`[test]`/`type` 值域） |
| `E1904` | invalid project name | R1 `we new` + R8 `name` |
| `E1905` | project manifest missing or incomplete | R8 manifest 骨架 |
| `E1906` | unresolved native symbol | R10 链接（ch19 承接） |
| `E1907` | command path not found | R1 `[path]` |
| `W1910` | unmocked custom effect in a test | R4 vet（ch20 W0601 承接） |
| `W1911` | possible indirect nested access to one shared value | R4 vet（ch18 W0755 承接） |
| `W1912` | function may block on a wait | R4 vet（ch18 W0756 承接） |

**不入册**（有录）：v0.8 W0140/W0141（tab/行宽——fmt 发现项，规则在 R3、发现属工具）、W0257（单 impl 接口——绑定 v0.8 interface 语义未在 We 批）、W0303（`_exh_ignore` 比例——绑定 v0.8 死语法）、W0401/W0402（未用 import/type alias 绕过——前者是合理工具发现但非规范承诺、后者绑定 v0.8 语义）、W0901/W0902（doc 完整性/交叉引用——项目策略，R9 直说"the completeness policy is the project's"）。判据一句话：**规范承诺了触发的才入册，工具自有的不入**。

### D5 CLI 面与单文件编译

`[path]` 两态：目录=项目（ch15 项目根事实复用）、文件=单文件编译。单文件模式的语义推演：无 manifest → 无源根 → 本地 import 无从映射 → `E1302` 且报文直说"no source root exists"；`std.*` 照常（编译器内建）。这不是新规则而是 ch15 映射规则的诚实推论——设计上写进 R1 而非留实现。`we new` 的 E1904 与 manifest `name` 的 E1904 同码同因（一条规则两个入口，一码一义不破）。`we version` 定形不定值：`we <compiler> (spec <spec>)` 形状入册、版本值永不入规范文本（ch0 纪律的直接应用）。

### D6 管线契约的三个关键取舍

1. **阶段序可观察**：一文件同时有语法错与类型错 → 只报语法错。这是"检查"腿的可预测性（P1 局部可判定在工具面的镜像）：模型作者按序修错，一次一个真相。
2. **E 止损、W 放行、提升的 W = E**：v0.8 规则全盘继承；提升语义让 advisory 层可被项目纪律化（`[vet]` 是策略出口）而不破坏"默认不阻断"。
3. **check 不产不链**：`we check` 是 generate–check–fix 循环的最小闭环——前端全查、零副作用；链接归 build（R10 依赖此句：E1906 在 check 下不可能发射）。
   增量/缓存/并行：实现细节，唯一承诺"同输入同输出"。`--target`：v0.8 固定 GOOS/GOARCH 随 Go 后端消亡（ADR-0002：LLVM 从第一天）；本章只留"build-only 的机制词汇，不固定"。

### D7 格式化器：规则全批，判断力零批

P7 的"零配置"兑付为 R3 的三个不可协商：确定性（同输入同输出）、不动点（fmt(fmt(x))=fmt(x)）、规则集封闭。规则集逐条来自 v0.8 §58 并保持其哲学：**只做有唯一正确答案的决策**。两处"刻意不做"是设计核心而非省略：不折行（断行位置无唯一解，P4 消歧义）、不对齐（对齐依赖名长，重命名即噪声——与 LLM 可预测性冲突）。fmt 发现项（tab、行宽）不入册（D4）：规则是承诺、发现是提示，一码一义不混。

### D8 vet 层与三点名警告的触发语义

- **W1910**（ch20 W0601 承接）：test 路径调用未 mock 的自定义效果函数。ch20 R8 已定"是真调用、spec 层无警告"——W1910 是工具链层的 advisory，补上"是否提醒"这一层，与 ch20 的"spec 层不警告"不矛盾（一个说规范不承诺，一个说工具可以提醒）。
- **W1911**（ch18 W0755 承接）：共享值回调体内调用的函数可能再触同一绑定。E1613 只看直接调用（ch18 直说"it does not follow calls into other functions"）——W1911 是"看一跳"的保守启发式，允许误报、明言非验证。范围收窄到一跳：跨函数启发式的完备性不存在，承诺范围即诚实范围。
- **W1912**（ch18 W0756 承接）：函数体直接调用五个其等待会阻塞的操作之一（`Semaphore.acquire`/`Cond.wait`/`Channel.send`/`Channel.receive`/`TaskHandle.await`——拼写与阻塞语义全部循 ch18 权威文本；v0.8 只列无缓冲 receive，其清单不完备，扩面在 D13 披露）。ch16 的 carve-out（等待不带效果标签）是设计决定：阻塞信息不进效果系统；W1912 把这个真实工程需求放在它该在的层——vet note。v0.8 的 note 档在注册表两档（error/warning）下沉为 warning，信息性写进 description（映射账 D13 披露）。
- **提升语义**：`"error"` 使该码如 E 止损止产——策略出口；`"ignore"` 抑制。值域三级封闭，越值 `E1903`。
- **实现自有发现**：MAY 携带、不入册、无承诺——第二实现不必复现。这句是防蔓延闸门：注册表是规范承诺的清单，不是工具功能的清单。

### D9 we test：运行形状与两态结局

- 发现：`tests/` 递归全量 `*_test.we`；他处测试模块（如 `src/`）是普通模块（ch20 已定其可编译可导入）不在默认集——两章的事实拼起来无新规则。
- 文件内源序执行（v0.8 承诺继承：可复现性）；跨文件不规定（实现自由）。
- 结局两态：pass/fail。v0.8 四态中 skip 随 test_each 死语法消亡；error/pass 之分随其双失败机制消亡——ch20 R6 已定 assert 失败即 panic 即同一边界，一刀两断。报文可名原因（assertion/panic），结局词汇封闭。
- 退出码 0/1/2（0 全过、1 有败、2 编译败且零测试运行）；`--filter` 空匹配=空跑退出 0（零测试零失败，与退出码语义自洽）。

### D10 探索：采样承诺与双守护码

探索 = 在 ch20 确定性承诺**之上**的探测层：确定性调度器自己选一个交错，探索强制走别的。三承诺三诚实：
- 承诺：迭代预算（`--iterations`，默认读 `[test].explore-iterations`）、偏序缩减默认开（等价运行折叠）、退出码 0/1/2。
- 诚实一：采样非穷尽——交错空间指数级，穷尽在原理上不可能，文档任何"穷尽覆盖"表述与本Requirement冲突（场景直说）。
- 诚实二：`E1901`（同测试探索运行分歧=确定性之外有真东西在动）与 `E1902`（探索执行未 mock 效果=破坏探针前提）。E1902 与 W1910 同因不同层：普通跑里是 advisory（提醒 mock），探索跑里是 error（前提已破）——v0.8 的 W0601/E1003 分层继承，理由在章内说透。

### D11 JSON Lines：字段稳定性作为规范承诺

字段集逐事件固定（diagnostic：type/severity/code/message/file/line/column/help-可选；test-result：file/name/status/duration_ms；test-summary：total/passed/failed/duration_ms）。稳定性承诺=注册表条目 schema 同款纪律（ch99："Adding fields ... backward-compatible; removing or renaming fields is not"）——两处承诺措辞对齐，一条纪律两个面。四级 severity（error/warning/note/help）是渲染层；注册表两档映射到前两档，advisory 可渲染为 note/help（W1912 信息性即典型 note 渲染）。v0.8 summary 的 `errors` 字段随 error 结局消亡（映射账披露）。`--json` 不改退出码——CI 的判定逻辑与渲染解耦。

### D12 manifest 骨架与留白的边界

骨架五件：`name`（ch1 命名约定，E1904）、`version`（形状归生态——不批 semver 就不写 semver）、`type`（executable|library 二值，越值 E1903）、`[vet]`（D8）、`[test].explore-iterations`（正整数，D10）。缺 manifest/缺必填键 E1905。**留白的写法**：不是沉默而是指名——"the acquisition story is this chapter's named gap, and the manifest's `[dependencies]` table belongs to the change that designs it"（R11）+ 场景"落了 `[dependencies]` 本规范对它既不拒也不定"。显式留白优于默示空白：读者知道这是设计决定不是遗漏。

### D13 v0.8 映射账

| v0.8 | 去向 |
| --- | --- |
| §56 子命令模型/全局选项 | 本章 R1 |
| §56.1 `we new`/E0150 | 本章 R1+R8；E0150→`E1904` |
| §56.2 `we clean` | 本章 R1（"artifacts not sources"一句承载） |
| §56.3 `we version` | 本章 R1（定形不定值——版本号永不入 spec，v0.8 示例值不承继） |
| §57 管线/E 止损/增量=实现细节 | 本章 R2 |
| §57 `--target` GOOS/GOARCH | **消亡**——随 Go 后端（ADR-0002）；目标词汇归工具机制不固定 |
| §58 fmt 规则 | 本章 R3 全批；W0140/W0141 **不入册**（工具发现项） |
| §59 vet/[vet] 配置/E0154 | 本章 R4；E0154→`E1903`；W0257/W0303/W0401/W0402 **不入册**（D4 判据） |
| §60 `we test`/退出码 0-1-2/--filter | 本章 R5；skip 随 test_each **消亡**；fail/error 之分随双机制**消亡**（ch20 R6 单路线） |
| §60 `--explore`/E1001/E1003 | 本章 R6；E1001→`E1901`、E1003→`E1902` |
| §61 doc/W0901/W0902 | 本章 R9；两 W **不入册**（项目策略非规范承诺） |
| §62 JSON Lines/字段稳定 | 本章 R7；summary 的 `errors` 字段**消亡**（无 error 结局） |
| §63 模块解析 | **已归第 15 章**（E0110→E1301、E0112→E1302 前切片已落）；CLI 路径不存在→本章 `E1907` |
| §64 LSP | **独立文档**（沿 v0.8 自己的措辞）；本章只承诺"编辑器诊断=we check 诊断"（R11） |
| §41.5 W0601 | →`W1910`（R4） |
| 评审补丁 W0755/W0756 | →`W1911`/`W1912`（R4；W0756 的 note 档→注册表 warning 档，信息性入 description；触发面从 v0.8 的"无缓冲 receive"**扩为 ch18 五个阻塞操作**——send 满缓冲亦阻塞、receive 空缓冲亦阻塞，v0.8 清单自身不完备，扩面循 ch18 权威语义并在此披露） |
| §65 留白（依赖获取等） | 本章 R11 显式指名承接为 named gap |

### D14 三要素账（原则 10）

- **类型检查**：零参与——本章无新语言表面；十个码全部是工具层判定（路径存在性、配置值域、符号绑定、探索分歧），无一个进入编译器的语言类型/效果/所有权裁决。`E1302` 在单文件模式的复用是 ch15 既有码在新入口的既有语义。
- **代码生成**：零新面——`we check` 止于文档检查（R2 固定的正是"不生成"）；`we build` 的产物命名由 manifest `name` 承载，无新文法。
- **运行时**：四件事全在工具运行时——fmt（确定性输出）、vet（启发式扫描）、test 运行器（发现/序/两态/退出码）、探索调度器（迭代预算/偏序缩减/双守护）。均为可观察契约下的实现自由；语言运行时（GC/调度/netpoll，ADR-0002）不在本章任何 Requirement 中出现——章界干净。
