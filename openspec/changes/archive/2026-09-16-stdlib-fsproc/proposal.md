# proposal — stdlib-fsproc（B2a）

> 状态：candidate 草案（勘查与四项裁决已完成，待 spec-impact audit）

## Why

B1 轨已整体收口（B1a `codegen-mono` + B1b `codegen-full` 皆 done，conformance 881、codegen 单测 445）。roadmap B2 行（`docs/roadmap/0000-reference-implementation.md:38`）给本轨道定的实现内容是「文件与目录 I/O、子进程 spawn（钉版 clang）、字符串构建、集合面背后的真数据结构」，策略句定的是「B2 是 B1 的调试基建（fs 与 process 面先行）」——编译器写出的程序要能读写文件、spawn 子进程，调试与自举才有地基。

**四项用户裁决（2026-09-15，AskUserQuestion，逐字入册）：**

| # | 裁决 | 内容 |
| --- | --- | --- |
| 1 | **切片** | 拆 **B2a + B2b**：B2a = 调试基建（fs + process + 字符串构建 + 装载机制 + E0816 收口）；B2b = 集合真数据结构（Map/Set 载体、构造面、变异成员面、List.add 别名可见性、Builder）。循 M6a/M6b、M9a/M9b、B1a/B1b 先例。 |
| 2 | **装载机制** | **go:embed 真 `.we` 源装载**——标准库源码成为嵌进二进制的真 We 文件，经真 parser 与同一检查器装载（用户推翻了「继续 Go 合成 StdModule AST 注册表」的推荐；自举预演即从此开始）。 |
| 3 | **fs/process 面形状** | **黑盒整文件 + 捕获式 spawn**——文件操作全是整文件粒度、无句柄类型；`process.run` 捕获 stdout/stderr。 |
| 4 | **未知 std 成员终判** | **E0816 收口、退役 `bndStdModules`**——基类型/集合上的未知成员从 exit-70 边界翻为 E0816 exit 1；两枚黄金重锚。 |

### 今日事实（勘查 + 真机探针，HEAD `10827df`）

**标准库的供给面**是编译器合成的：`internal/typecheck/typecheck.go:2469` 的 `StdModule(key)` 注册表恰好合成四枚模块——`std.io`（M8，`println`/`print`，`effect io`）、`std.concurrent`（M9a，`stdConcurrentFile`：`TimeoutError`/`TaskPanic`/`SendResult{Sent,Full,Closed}`/`ReceiveResult<T>{Received,Empty,Closed}` + channel 标记 FnDecl）、`std.test`（M10a，`assertTrue`/`assertFalse`；`assertEqual` 是调用面——`importCall` 前置分派，**非条目**）、`std.time`（M10b，`now` 虚构体 `0` + `sleep`）。合成 AST 走同一 ingest 机器——「供给是编译器建的；检查不享有特权」。**但源不是 We 写的**，B3 自举没有可迁移的库源。

**fs 今日缺席**（真机探针）：`import std.fs` → `E1302: module not found — no standard-library module "std.fs" exists in this build; the std segment is compiler-provided, so the name is misspelled or the module is not implemented yet`，exit 1。装载层在 `stdImports`（`typecheck.go:2568`）之外的段名上发 std 形 E1302。

**未知 std 成员的今日边界**：`internal/typecheck/typecheck.go:51` 的 `bndStdModules` 词（"standard-library modules (chapter 15) are not implemented in this reference build yet"）恰有两个产出点——`:6968`（集合和式：锚定族 `get`/`has`/`iterator` 经 `collectionMembers`（`:1349-1372`）解析，其余成员停）与 `:7022`（基类型含 String：`stringMembers`（`:1337-1340`）锚定四员 `byteLength`/`byteSlice`/`runeCount`/`charAt` 解析，**任何**未知成员停）。真机探针：裸 `xs.count()`（`xs: List<Int64>`）今日 exit 70 + 边界行、语料零锚定。**语料爆炸半径 = 恰好 2 枚黄金**携带该边界行（`check-bnd-std-member`：`let n = s.length`；`check-stdio-shadow-let`：遮蔽 `let io = 1; io.println("x")`）。

**翻转是规范正确的**：E0816 已在册（`docs/spec/diagnostics.toml:749-755`，owner `1000-interfaces`，requirement "Member name resolution"，title "no such member on the receiver's type"），ch18 并发类型面已用它报未知成员。`s.length`——ch17 只批了 `byteLength`/`byteSlice`/`runeCount`/`charAt` 四个命名方法，`length` 不在任何已批清单；`xs.count()`——组合子是 `Iterator` 的方法（ch11），`List` 的锚定族只有 `get`/`has`/`iterator`（ch17）。两者翻 E0816 都是「成员不在接收者类型上」的诚实答案。**ch17 R104 立场**："The method inventory beyond the anchored access family is the standard library's surface"——标准库表面**未规范批准**，故本变更是纯实现、零规范增量。

**发射面已有全部先例**：

- **keyed 发射**：std fn 声明携 We 可写的虚构体（fiction body）走正常检查，codegen 按 `(module, name)` 键发射替换体——`println` 先例（`codegen.go:92` declare 表 `{"__we_println", "declare void @__we_println(ptr, i64)"}` 形、`:270` keyed map、`:2375` emitIoCall）。
- **mock 槽**：M10b 把 std io/test/time 条目一律槽化——keyed std fn 天然骑槽机器 ⇒ **fs/process 成为 mock 目标免费获得**（ch20 虚拟化路径）。
- **运行时 C 面**：`str.h` `struct we_str {const char *p; long long n}`（malloc 域、永不回收，follow-up #21 姿态）；`list.h` `__we_list_new(cap, traced)`/`__we_list_push`（增长返回新身份）/`__we_list_get`/`__we_list_len`/`__we_list_snap`。
- **C 侧构造 Result/记录**：sum 三字 `{tag, pay0, pay1}`（T9-1）；Err 融合 tag 空间（tag 0=Ok、tag 1+k=E 第 k 变体，T9-2）；C 侧构造 List<String> 的先例 = T11-② 32B 无描述符盒 `map=null`（String 字节在 gc 域外，零描记引用）；T12 先例 = C 返回的 gc 记录由调用方立即再扎根；T9 `payRoundTrip` 处理 gc→inttoptr 的 sum 载荷字。
- **join/repeat 的真体可行面**（B1b 拓宽后探针全绿）：带注解 `List<String>` 的 for-in ✓（T11-② 后）、循环拼接 var 赋值 ✓、无标注 `var acc = ""` → `bindStringSlot` ✓（T10 簇 8）；**两处已知停面都被避开**——`List<String>` 上的回调（`listElemWord` skStr 守卫）、`Option<String>` 载荷臂读——join/repeat 的体两个都不用。

**std.concurrent 迁移的两枚硬阻塞**（披露例外）：变体名 `Closed` 同时住在 `SendResult` 与 `ReceiveResult` —— 真 `.we` 源会撞 E0404（模块单一名字空间的同名变体）；channel 标记 FnDecl 的定型是期望驱动的（E1617）——签名依赖调用侧期望类型，源码声明无法自足。**std.concurrent 在 B2a 保持合成**，装载机制对其是例外（design D1 记例外口）。

## 目标与非目标

目标（一条线：B2 的调试基建）：

1. **go:embed 装载**：新根包 `stdlib/`（`stdlib.go` + `//go:embed src/*.we`，镜像 `runtime/runtime.go` 对 `c/*.c` 的嵌法）；`std.io`/`std.test`/`std.time` 三枚迁移为真源（`stdlib/src/{io,test,time}.we`）；`StdModule(key)` 翻为「注册表键 → 嵌入源或合成构造器」，解析走真 parser、检查走同一 ingest——**狗粮从标准库吃起**。零漂移是硬验收：既有语料逐字节绿、IR 字节对拍、mock 槽物化同一。
2. **std.fs**：`pub type FsError = FsFailed(String)`；`readFile`/`writeFile`/`appendFile`/`removeFile`/`makeDir`/`removeDir`/`listDir` 七枚整文件操作，全部 `effect io -> Result<_, FsError>`（`listDir -> Result<List<String>, FsError>`）；keyed 发射 + 运行时 `fs.c`。
3. **std.process**：`pub type ProcessError = ProcessFailed(String)`；`pub record ProcessResult { exitCode: Int64, stdout: String, stderr: String }`；`pub fn run(cmd: String, args: List<String>) effect io -> Result<ProcessResult, ProcessError>`——fork/execvp PATH 搜索、双管道捕获、waitpid；keyed 发射 + 运行时 `process.c`。
4. **std.string**：`pub fn join(parts: List<String>, sep: String) -> String` + `pub fn repeat(s: String, n: Int64) -> String`，纯函数，**真体发射**（首批经 M10b 管线发射的标准库 define）；发射管线是否无改动可用是**开放验证项**（T4 阻塞前置；回退 = keyed C 助手 + 披露）。
5. **E0816 收口**：`:6968`/`:7022` 两处翻 E0816、退役 `bndStdModules` 词；两枚黄金重锚（保名保源，exit 70→1）；消息以注册表标题起头、detail 点名成员与接收者类型。
6. **fs/process 骑 mock 槽**：keyed std fn 槽化 ⇒ `mock fs.readFile` / `mock process.run` 成为检查器可过的目标（ch20 面，黄金钉）。

非目标：

- **B2b 集合真数据结构**：Map/Set 载体、构造面、变异成员面（`push`/`add` 等写入成员）、List.add 别名可见性（载体不搬家改造——增长时 push 返回新身份 vs ch17 别名可见性的冲突）、Builder——全部归 B2b `stdlib-collections`。
- **std.concurrent 真源迁移**：E0404 同名变体 + E1617 期望驱动两枚硬阻塞——保持合成，例外口记 design D1。
- **句柄形文件操作**（`File`/`Handle` 类型、流式读写）：裁决 3 定黑盒整文件，句柄形非本变更。
- **process 面的作业控制**（stdin 喂入、信号、超时、环境变量定制）：stdin 继承为文档化行为，其余不批。
- **spec 触碰**：零 Requirement 增删、零诊断码、零 ADR——ch17 R104 明说标准库表面归标准库，本变更「不改变语言行为」「无规范增量」。
- **性能**：join/repeat 真体按正确性先写（O(n²) 拼接可接受）；调优归后续。
- **CLI/工具面、LSP、formatter**：零触碰。

## What Changes

- **`stdlib/`（新根包）**：`stdlib.go`（package stdlib，`//go:embed src/*.we`，源文件表 `Source(key)`）+ `src/io.we`/`src/test.we`/`src/time.we`（迁移三枚，虚构体照今日合成 AST 的体逐字写）+ `src/fs.we`/`src/process.we`/`src/string.we`（新增三枚）。Go 侧门 = 嵌入源逐一解析干净的测试。
- **`internal/typecheck`**：`StdModule` 翻转（键 → 嵌入源 parse / 例外键 → 合成构造器；进程内 parse 缓存——AST 纯数据、检查器不改树，循 M6b 跨模块 AST 共享先例）；`assertEqual` 调用面缝（canonical `(std.test, assertEqual)` 身份保持，真源按裁决声明条目或维持拦截先行——零漂移语料定夺）；`:6968`/`:7022` 翻 E0816 + `:51` 词退役。
- **`internal/codegen`**：keyed declare 表扩 fs/process 十一枚符号（out 参形 `declare void @__we_fs_read_file(ptr, i64, ptr)`；发射 = alloca 三字 + call + load 三字构 sum 槽）；std.string 真体 define 发射（槽 `@std.string.join`/`@std.string.repeat`）；Ok 载荷 gc 字的再扎根与 `payRoundTrip` 复用。
- **`runtime/c`**：新 `fs.c` + `process.c`（join `runtime/runtime.go` embed 集 + link 参数 + C harness 编译集，M10c TestIOHarness 先例）；消息形 `"{op} {path}: {strerror}"`；`listDir` 排序 strcmp（确定性，P1）；NUL 入径 → `FsFailed`；`makeDir` 单层 mkdir 0755；`removeDir` = rmdir 仅空目录；`WIFSIGNALED` → `exitCode=128+sig`；stdin 继承（文档化）。
- **conformance**：黄金矩阵（design D8）——`check-fs-effect`（E1401）、fs 往返族 `run-fs-*`（write→read→append→list→remove；read 缺失 → Err stderr + exit 1）、`run-proc-run`（`/bin/echo`）、`run-string-join`/`-repeat`、两枚 E0816 重锚、mock 面、突变电池。
- **docs**：roadmap B2 行拆 B2a done / B2b pending（双语，归档期）。

## 影响层

- **spec：零**。ch17 R104："The method inventory beyond the anchored access family is the standard library's surface"——标准库表面不在规范里，本变更是实库兑现。「不改变语言行为」「无规范增量」；E0816 收口用的是**已批检查语义**（ch10 成员解析 + ch17 锚定族），不是新语言面。
- **compiler**：`internal/typecheck`（装载翻转 + E0816 两点）、`internal/codegen`（keyed 扩容 + std define 发射）、`runtime/c`（fs.c/process.c 新文件）。既有 E 码行为零改写（唯一例外 = 两枚黄金按裁决 4 重锚，逐枚披露）。
- **stdlib**：新根包 `stdlib/`——本轨道首个 stdlib 层内容（真 `.we` 库源 + embed 载体）。
- **docs**：roadmap 双语（归档期）。

## 影响范围

- **涉触码**：`stdlib/`（新）、`internal/typecheck/typecheck.go`（`StdModule` + 两处翻转 + 词退役）、`internal/codegen/codegen.go`（declare 表 + keyed map + 发射臂 + std define）、`runtime/c/fs.c`、`runtime/c/process.c`、`runtime/runtime.go`（embed 集）、`internal/conformance/testdata/cases/`（新增 + 2 重锚）、`internal/cli/build.go`（编译集如有独立清单）。
- **既有黄金**：除裁决 4 点名的 2 枚（`check-bnd-std-member`、`check-stdio-shadow-let`——保名保源重锚，exit 70→1）外**零触碰**；T1 迁移的零漂移以全语料 881 枚逐字节绿为证。
- **对外零影响**：CLI 面、诊断码注册表（零新码）、LSP、formatter、`we doc` 零触碰；`std.concurrent` 装载行为逐字节不变。
- **`refr/` 禁区**：本变更的任何提交不得纳入 `refr/`。

## 勘查依据

勘查 + 真机探针（HEAD `10827df`，探针二进制 `/tmp/b2-we`，探针项目 `/tmp/b2-probe`）：

1. **装载面清单**：`StdModule` 四枚注册表的合成 AST 面、`stdImports` 段名集、E1302 std 形报文、M10b 槽机器对 std 条目的键法、`assertEqual` 的 `importCall` 前置分派位（19 枚 `test-asserteq-*` + 5 枚 `check-assertequal-*` 黄金的身份所系）。
2. **成员解析面清单**：`bndStdModules` 两产出点的到达路、`stringMembers`/`collectionMembers` 的锚定集、E0816 注册表条目与 ch18 先例、语料 grep（边界行恰 2 枚）。
3. **发射与运行时面清单**：keyed 发射三件套、sum 三字 ABI 与 Err 融合、T11-② C 侧 List<String> 构造、T12 再扎根、T9 `payRoundTrip`、`we_str`/`we_list` C API、M12 String 双标量展开先例。
4. **真机探针（三枚）**：`import std.fs` → E1302 std 形 exit 1；裸 `xs.count()` → exit 70 + 边界行（未锚定）；`s.length`/`io.println` 遮蔽形 = 语料中恰 2 枚边界行携带者。join/repeat 体所用面（for-in `List<String>`、循环拼接、无标注 var）的绿态引 B1b 落地记录。

## 审计记录

**2026-09-15，candidate → ready，`welang-spec-impact-audit` 七条，通过（一枚开放项如实入册）。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | 问题真实性 | ✅ 黑盒可验证：fs 缺席 = `import std.fs` → E1302 std 形 exit 1（真机探针，HEAD `10827df`）；未知 std 成员 = exit 70 边界行（裸 `xs.count()` 探针；语料恰 2 枚携带者）；库源非 We 书写 = 工程缺口（B3 自举三段式闭环需要可迁移库源——roadmap B3 行验收面点名 conformance 全绿，库源是前提）。规范锚点：ch17 R104（标准库表面）、ch10 E0816 注册表条目 + ch18 先例、ch11 组合子归 Iterator、ch15 import/预导入、ch20 mock 面。工单 = roadmap B2 行（`docs/roadmap/0000-reference-implementation.md:38`）。 |
| 2 | 影响层声明 | ✅ `change.yaml` `layers: [compiler, stdlib, docs]` 与 `## 影响层` 一致（spec 条显式「零」并给理由）。触及 compiler+stdlib 但**不改变语言行为**，proposal 双标记（「不改变语言行为」「无规范增量」）开机械豁免（validate.py:102-107）；stdlib 层为 M0 词表 (`VALID_LAYERS`) 在册层的**首次使用**（benchmark 层首用于 M15 同形先例）。 |
| 3 | 规范增量范围 | ✅ **零 Requirement 增删、零诊断码、零 ADR**。E0816 收口用的是在册码与在册触发语义（ch10 成员名解析 + ch17 锚定族 + ch18 先例）——纯实现兑现。不触碰 `docs/spec/diagnostics.toml`。 |
| 4 | 原则一致性 | ✅ 无冲突。P1（可判定）：`listDir` strcmp 定序、E0816 结构化诊断替代 exit-70 边界行（generate–check–fix 回路收益）；P2（单一语义）：真源装载**消除**一处潜在双权威（Go 合成 AST 注册表 vs 库源——embed 后库源是唯一事实）；P9（单一错误机制）：Result + 注册表码，零新错误通道。无原则突破 ⇒ 无需 ADR。 |
| 5 | 参考基线固定 | ✅ 全部引用为仓内路径 + 行号（`internal/typecheck/typecheck.go`、`internal/codegen/codegen.go`、`runtime/c/*`、`stdlib/`（新）、`docs/spec/*`、`docs/roadmap/*`），随 HEAD `10827df` 固定。**未引用 `refr/`**（禁区）；无外部项目引用。 |
| 6 | 验收边界 | ✅ 机械可判：零漂移三缝（881 枚逐字节绿 / IR 字节对拍 / mock 槽同一）；fs/process/string 黄金先红后绿；E0816 收口 = 翻转后语料边界行 grep 归零 + 2 枚重锚绿 + `bndStdModules` grep 归零；T4 开放项三选一结论写回。非目标显式排除：B2b 集合、concurrent 迁移（两枚硬阻塞点名）、句柄形文件操作、process 作业控制、性能、CLI 面——足以防蔓延。 |
| 7 | 粒度 | ✅ 一个垂直面：装载机制是新三枚模块的**载体**（fs/process/string 作为真源无法先于装载存在）、E0816 收口是装载落地后 std 词汇的终局——四件咬合，拆开则装载机制无第二个消费者可验证。验收（黄金 + 零漂移）落在 compiler+stdlib 一层；docs 是归档期随行（B1a/B1b 同形）。B2 整体拆 B2a/B2b 已是用户裁决 1（B2b 集合载体独立成塔）。体量：预估 7 任务组 ≈ B1b 的一半强，同量级。 |

### 开放项（入册，非阻塞）

- **NUL 入径黄金的可拼性**（design D8 已注）：We 源能否拼出含 NUL 的 String 路径（`\u{0}` 在闭集逃逸表内，但 rune→String 拼接面待实证）——T2 实测定形：可拼则黄金、不可拼则 C 侧单测独守 + 披露。不影响验收边界（NUL 拒绝行为本身由 C 单测机械验证）。
- **T4 define 发射开放验证项**（design D5-3）：std 模块真体 define 无先例；三选一出口（直行/参数化修复/回退 keyed）已在任务书固化为阻塞前置 checkbox。

## 审查记录

**2026-09-15，ready → active，`welang-change-review` 十条，通过（审查前置已修一处基线引用漂移）。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | proposal 职责边界 | ✅ 黑盒问题与目标为主；目标句中的机制词（go:embed、keyed、E0816 收口）皆**逐字溯源自用户裁决 2/3/4**（裁决须逐字入册），非混入的实现决策；「今日事实」段是勘查证据（真机探针 + 行号引用），决策本体在 design。无任务清单混入。 |
| 2 | spec 增量 | ✅ N/A 成立——零 specs/ 目录、双豁免标记（「不改变语言行为」「无规范增量」）在案，validate strict 机械放行；E0816 收口用在册码与在册触发语义，非新语言面。 |
| 3 | design 唯一最小路径 | ✅ 单一路径：装载 = embed+parse+缓存（concurrent 例外是策略表一行非一层）；被拒替代在册（D7-2 C 结构返回 vs out 参，MEMORY-class/隐式 sret 理由）；引用精确——审查前对 HEAD `10827df` 逐条复核（见 F1 处置），`typecheck.go:51/:2469/:2539/:6968/:7022`、`codegen.go:92/:270/:2375`、`diagnostics.toml:749-755` 全中。 |
| 4 | tasks 职责边界 | ✅ 19 复选框全部有 来源：/验证： 行（strict 机械确认），来源皆指向 design D 节或裁决；无 deferred/non-goal 伪装成任务。三处「二选一/三选一」（T1-3 assertEqual 选 A/选 B、T2 NUL 形、T4 三出口）皆**带机械仲裁者的已定决策程序**（语料逐字节 / 可拼性探针 / 三面探针），非未决方案选择——判据在任务书内自足。 |
| 5 | 场景覆盖 | ✅ normal（fs 往返、proc-run、join/repeat、`?` 链）/ boundary（listDir 定序、NUL 拒、repeat 0 次、join 空表、exitCode 捕获）/ failure（read-missing → Err+exit 1、spawn 失败 → Err、E1401、E0816 重锚）三族齐；另有突变电池 (a)–(f) 对抗性验证。 |
| 6 | 无空章节 | ✅ 四件无凑格式章节；change.yaml 三行为最小形。 |
| 7 | 测试先行 | ✅ 任务头红先纪律明文；T1 首框「先红——源未落盘时测试无法成立」；T2/T3/T4 黄金逐枚先红（fs 族红于 E1302）；T5 重锚循 B1a/B1b 先例（新期望先取红、翻转落地位后绿）。 |
| 8 | 负向断言 | ✅ 构造违规样例真实在案：E0816 两枚重锚黄金（exit 1 + 消息逐字比对 title）、相邻面守界三探针（std.bogus 仍 E1302 / 链形仍绿 / 接口·Dyn·记录面照旧——防翻转过宽）、`check-fs-effect`（纯 fn 调 io → E1401）、突变 (d) NUL 检查摘除必死。无文字代替。 |
| 9 | 完成度闭环 | ✅ 三要素齐：typecheck（D1 装载翻转 + D6 两点翻转）、codegen（D2 keyed 扩容 + out 参发射 + D5-3 define）、runtime（D7 fs.c/process.c + ABI + 缓冲纪律）。本变更为实现层（非语言特性类），三要素仍全给。 |
| 10 | 未决问题阻塞 | ✅ 两枚开放项处置正确：T4 首框【阻塞前置】「未定不得勾其后各项」显式阻塞；NUL 可拼性是带双出口验证的机械分叉（黄金或 C 单测+披露，两枝皆有验收），不阻塞任务本体。选 A/选 B 同理（语料仲裁）。 |

### 发现与处置

- **F1（审查前置发现，已修）——基线引用漂移两处**：proposal 的 `collectionMembers（:1358-1368）` 为错引（该区间只盖 mapSum/setSum 两 case、漏 listSum 臂，函数实在 `:1349-1372`）；proposal+design 的 `stdImports（:2572）` 为体内行漂移（函数头 `:2568`）。两处已按 HEAD `10827df` 实测改平。其余引用逐条复核全中；`stringMembers（:1337-1340）` 经逐行核对恰为四锚定员字面行（`iterator` 在 1336），引用按书面成立。

**结论：十条全过，F1 已处置。status → active，可开始实现（T1 待点名）。**

## 审查记录（实现审查）

**2026-09-16，`welang-code-review` 七条，通过（F1 措辞级发现已处置）。**

| # | 条目 | 结论 |
| --- | --- | --- |
| 1 | 规范符合性 | ✅ 零规范增量声明兑现——`docs/spec/`（含 `diagnostics.toml`）在 `10827df..9a14c90` 六提交上 diff 为零；全部接受/拒绝判定走已批面：fs/process/string 的 We 源语法（声明、effect io、Result、泛型零参与）、E0816 收口用在册码与在册触发语义（ch10 成员解析 + ch17 锚定族 + ch18 先例）、E1302/E1401 在册、mock 面走 ch20 既有机器；ch17 R104（"the standard library's surface"）覆盖三枚新模块的 API 面不进规范这一立场。 |
| 2 | 验证诚实性 | ✅ tasks 19 框逐核，每框有补记一~六的落地记与验证输出；无「勾了但没跑」。本轮抽查复验（非引旧档）：①hello IR 对拍——父树 `10827df` 与本树双二进制对 /tmp 项目 `we build`，`build/demo.ll` 逐字节 `cmp` 零差、`@slot.io.println`/`@slot.time.now` 两槽俱在（T1 缝②复绿）；②`bndStdModules` 全仓 grep 归零（词退役）；③corpus 边界行 grep 归零（E0816 翻转）；④昨日 T6 阶梯八面全量绿（16 包 0 FAIL、conformance 893 枚 `-count=1`）。 |
| 3 | 测试先行证据 | ✅ 六任务红先记录齐：T1 门先红（`pattern src/*.we: no matching files found`）、T2/T3/T4 黄金实现前红于 E1302（装载未含）、T5 翻转前 exit 70 记档（corpus 恰 2 枚 + 裸形探针）、T6 严格红证据（同黄金源 × 父树 `4a061e4` worktree 二进制 `we test .` exit 70 + mockTarget 边界行）；突变电池六枚逐枚「先锚点 count==1、后突变、还原复绿」。黄金 JSON 形与语料统一，期望字节皆真机输出。 |
| 4 | 诊断协议稳定 | ✅ `internal/diag/` 零 diff（`--json` 字段集不变）；零新增诊断码（全局唯一性 N/A）；E0816 消息循一码一消息纪律——title 恒为注册表原文（T5 重锚黄金逐字比对）。 |
| 5 | 单一权威 | ✅ API 面的唯一事实 = 库源 `stdlib/src/*.we`（装载机制按裁决 2 消除 Go 合成注册表的双权威）；运行时行为契约（fs 七入口、NUL 拒、`"{op} {path}"` 消息形、listDir strcmp 序、makeDir 单层 0755、process 捕获形/PATH/128+sig/poll、string 纯函数真体、E0816 翻转）由 `docs/benchmarks.md`/`.zh.md` 双语运行面段完整承载（docs_sync 33 对含此二文件）；无滞留变更目录的长期事实、无待提升 spec/ADR 项（R104 立场：标准库表面不进规范）；新增码面中文混入机械 grep 归零。 |
| 6 | 红线复核 | ✅ 六提交 `refr/` 零文件；commit 信息零署名 trailer；逐提交文件清单对任务书零越界——T4 的 `build.go`+import 走查改动由【阻塞前置】授权（结论「修复」已写回任务行）、T2 的 m6b 门表重锚与 T5 的三枚单测重锚皆补记二/五披露。 |
| 7 | 最小可信验证已跑 | ✅ T6 收口阶梯八面（build/vet/gofmt 0 文件/`go test -count=1 ./...` 14 ok + 2 no-test/validate strict/docs_sync 33 对/diff-check 净/无 refr）+ 本轮 IR 对拍与双 grep 复验。 |

### 发现与处置

- **F1（措辞级，已处置）——tasks.md T6 第三框括注「15 包」为起草旧数**：as-built 16 包（T1 落地记「包数 15→16」，T6 落地记亦写 16），任务行括注沿抄了任务书起草时的 15。处置：括注订正为 as-built 16 并留原数注记（沿 B1b T15 F1 先例——措辞级不改行为，落地记为准）。
- **as-built 预告（非发现）——归档目录名**：任务书写 `2026-09-15-stdlib-fsproc`（起草日预测），实际归档日 2026-09-16；按 B1a/B1b 归档日惯例取 `2026-09-16-stdlib-fsproc`，落地记注明。

**结论：七条全过，F1 已处置。status → complete，进入归档（welang-archive-sync）。**
