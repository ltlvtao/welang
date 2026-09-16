# design — stdlib-fsproc（B2a）

> 载体：proposal.md（问题与裁决）；本文：机制设计。D0 切片与规范立场 → D8 黄金矩阵。

## D0 范围、切片与规范立场

**切片**（裁决 1）：B2 拆 B2a（本变更：装载机制 + fs + process + 字符串构建 + E0816 收口）与 B2b（`stdlib-collections`：Map/Set 真数据结构、构造面、变异成员面、List.add 别名可见性、Builder）。拆法循 M6a/M6b、M9a/M9b、B1a/B1b 先例——B2a 的四件互相咬合（装载机制是 fs/process/string 三枚新模块的载体；E0816 收口是装载落地后 std 域词汇的终局），B2b 的集合载体是另一条塔（ch17 变异语义 + 载体改造，独立验收面）。

**四项裁决表**（逐字，出处 proposal「Why」）：

| # | 裁决 | 设计落点 |
| --- | --- | --- |
| 1 | 拆 B2a + B2b | 本变更 = B2a；roadmap B2 行拆两行（归档期） |
| 2 | go:embed 真 `.we` 源装载 | D1（用户推翻 Go 合成注册表续用） |
| 3 | 黑盒整文件 + 捕获式 spawn | D3/D4 |
| 4 | E0816 收口、退役 `bndStdModules` | D6 |

**规范立场**：ch17 R104 "The method inventory beyond the anchored access family is the standard library's surface"——标准库表面未规范批准。本变更零 Requirement、零诊断码、零 ADR；E0816 收口应用的是已批语义（ch10 成员名解析 + ch17 锚定族 + ch18 E0816 先例），不是新语言面。「不改变语言行为」。

**语言面事实（设计依赖，均为已批章文）**：

- `effect io` 段位：`pub fn readFile(path: String) effect io -> Result<String, FsError>`——段在参数表与 `->` 之间（ch6/ch16 权威；ch19 示例笔误的教训 follow-up #15 在册）。
- sum 声明与构造：`pub type FsError = FsFailed(String)`（ch9）；record 声明块形 `pub record ProcessResult { exitCode: Int64, stdout: String, stderr: String }`（ch8；构造块形，调用形 E0105）。
- 预导入集不含 fs/process/string 模块名——`import std.fs` 引入 `fs`，合格到达 `fs.readFile(...)`（ch15 R2/R4；D18 名字到达：import 只引入模块名）。
- `?` 传播用户 Result：fn 内 propagate、main 内 report（ch14 + B1b T12 落地）——fs 黄金的错误路径直接可用。

## D1 go:embed 装载

### D1-1 载体形

新根包 `stdlib/`，与 `runtime/` 平级：

```
stdlib/
  stdlib.go        # package stdlib; //go:embed src/*.we; Sources() map[string]string
  stdlib_test.go   # 门：每枚嵌入源 parser.Parse 干净
  src/
    io.we          # println/print（迁移，虚构体照 M8 合成 AST 逐字）
    test.we        # assertTrue/assertFalse(+assertEqual 条目问题见 D1-4)
    time.we        # now/sleep（now 虚构体 0）
    fs.we          # 新增（D3）
    process.we     # 新增（D4）
    string.we      # 新增（D5）
```

镜像先例：`runtime/runtime.go` `//go:embed c/*.c`。Go 工具链约束（M4 学到的）：embed 文件须在包目录树内 ⇒ 源住 `stdlib/src/`。无 import 环：`stdlib` 包不 import typecheck；`internal/typecheck` import `stdlib` 取源。

### D1-2 StdModule 翻转

`typecheck.go:2469` `StdModule(key)`：注册表从「键 → 合成构造器」翻为「键 → 装载策略」：

```go
// key → embedded source (parse + cache) | synthetic builder (concurrent only)
```

- 六键走嵌入源：`io`/`test`/`time`/`fs`/`process`/`string`。解析用 `internal/parser` 正常入口，产 `*ast.File` 后走**同一 ingest 机器**——零特权路径（M8 立场从「合成 AST 无特权检查」升为「真源真检查」）。
- 一键例外：`concurrent` 保持 `stdConcurrentFile` 合成构造器（阻塞与例外口见 D1-5）。
- **进程内 parse 缓存**：`sync.Once`/lazy map，键 → `*ast.File`。AST 纯数据（`ast.go:8-10`）、检查器不改树；跨 Check 调用共享安全——M6b 跨模块 AST 指针共享先例（`CheckProject` 早已多模块共享 AST）。
- `stdImports`（`typecheck.go:2568`）零改动——import 解析仍把 std 段名引到 `StdModule`；未知段名照发 E1302 std 形（std.bogus 仍 E1302，std.fs 自本变更起存在）。

### D1-3 虚构体（fiction body）纪律

std fn 声明携 We 可写虚构体，走正常检查机器（今日合成 AST 已如此——`now` 体 `0`）；codegen 按 `(module, name)` 键发射替换体（D2）。迁移三枚的虚构体**逐字照抄今日合成 AST 的体**，零语义差：

| 模块 | 条目 | 虚构体 |
| --- | --- | --- |
| io | `println(s: String) effect io -> ()` / `print` | `{ }`（空块 = unit，今日同形） |
| test | `assertTrue(b: Bool) -> ()` / `assertFalse` | `{ }` |
| time | `now() -> Int64` | `0`；`sleep(ms: Int64) effect time -> ()` 体 `{ }` |

新模块的虚构体（D3/D4/D5 各表）：fs/process 七+一枚 = `Err(FsFailed("fiction"))` / `Err(ProcessFailed("fiction"))` 形——构造 Err 变体，类型自足（Error sum 同模块声明），检查器面干净；join/repeat **无虚构体**——真体即检查体（D5）。

### D1-4 三条零漂移缝（T1 的验证面）

1. **corpus 逐字节**：881 枚既有黄金全绿、零改写——装载翻转对外不可见。std 面 24 枚直接锚定者：19 枚 `test-asserteq-*` + 5 枚 `check-assertequal-*`（assertEqual 身份）+ std.io/time 的 io/test 黄金族。
2. **IR 字节对拍**：`import std.io` 的 hello 程序 build 产物 IR 前后逐字节（M9a `m8HelloIR` 先例——import 擦除与 declare 面的对拍锚）。
3. **mock 槽物化同一**：M10b 槽机器按 `(module, name)` canonical 键物化槽——模块键不变（`std.io` 等字符串常量）⇒ 槽名逐字节同；既有 mock 黄金即证。

**assertEqual 条目缝**（三缝中最细）：今日 `assertEqual` 非条目、`importCall` 前置分派拦截。真源两选一：

- **选 A（默认）**：`src/test.we` **声明** `assertEqual<T>` 条目（虚构体 `{ }` 或返回 unit 的最小体），拦截仍前置——canonical 身份 `(std.test, assertEqual)` 不变，24 枚黄金逐字节绿即证。
- **选 B（回退）**：若声明条目扰动任何黄金（如值位解析 `st.assertEqual` 作 fn 值的语义差），则真源不声明、拦截独占——`src/test.we` 头注释记「assertEqual 由调用面合成（M10a D8）」。
- T1 实现时先 A 后跑全语料，任何一枚漂 → B + 披露。**分派序不可反**：拦截必须先于成员解析（首参 T 定型与 Eq 域门是调用面专属机器）。

### D1-5 concurrent 例外口（披露）

`std.concurrent` 保持合成。两枚硬阻塞（勘查实证）：

1. **E0404 同名变体**：`SendResult.Closed` 与 `ReceiveResult.Closed` 同名——真 `.we` 源在模块单一名字空间（ch6 R7）下撞 E0404。合成 AST 绕过声明层名字表（直接注记成员），真源无此自由。出路 = 重命名变体或拆模块——**语言面取舍，归 B3 前的标准库表面裁定，非本变更**。
2. **E1617 期望驱动**：channel 标记 FnDecl 的定型依赖调用侧期望类型（`Channel<T>` 由期望定 T）——真源声明无法自足。出路 = 显式类型参数拼写，同为表面裁定。

例外口的性质：装载机制对六键生效、对 concurrent 不生效；`import std.concurrent` 行为逐字节不变（语料锚定）。D1-2 的策略表让例外是一行，不是一层。

## D2 三类 std 函数

| 类 | 成员 | 检查面 | 发射面 |
| --- | --- | --- | --- |
| **keyed 虚构体** | io×2、test×2(+assertEqual 若选 A)、time×2、fs×7、process×1 | 虚构体走正常检查 | `(module, name)` 键 → declare 表 + 替换调用；**骑 mock 槽**（M10b 槽机器先例） |
| **真体** | string×2（join/repeat） | 真体即检查体 | M10b 管线发射 define（`@std.string.join`）；槽物化同 keyed（调用位间接 load）——**开放验证项**（D5-3） |
| **合成** | concurrent 全部 | 合成 AST 走 ingest | 照旧（M9a/M9b/M10b 机器） |

**keyed 发射机制**（println 三件套扩容）：`codegen.go:92` declare 表加 11 行；`:270` keyed map 加 11 键；发射臂走 out 参形（D7-2）。fs 键集：`__we_fs_read_file`/`__we_fs_write_file`/`__we_fs_append_file`/`__we_fs_remove_file`/`__we_fs_make_dir`/`__we_fs_remove_dir`/`__we_fs_list_dir`；process：`__we_proc_run`。

**Result 返回的发射形**（out 参，D7-2 定形）：

```
%out = alloca [3 x i64]
call void @__we_fs_read_file(ptr %p, i64 %n, ptr %out)
%t = load i64, ptr %out        ; + 两个载荷字 load → sum 槽三字
```

**gc 载荷字的纪律**：`listDir` Ok 载荷 = List 句柄（gc 域）——load 出的 i64 字经 `inttoptr` 后**立即再扎根**（T12 先例：C 返回的 gc 记录由调用方补根；T9 `payRoundTrip` 的 gc→inttoptr sum 载荷先例）。`process.run` Ok 载荷 = ProcessResult 记录句柄，同理。**验证项**：Ok 载荷是 sum 内 gc 记录的 match 臂读（T9 管道存在——`bindArmPayload` 表驱动消费 pay 字）。

**mock 面**：keyed std fn 已骑槽 ⇒ `mock fs.readFile` 检查器可过（E1804 七类目不含模块 fn）、运行时拦截生效——黄金钉一枚（`test-fs-mock`：mock 后调用返回桩值）。这是裁决 3 之外免费获得的 ch20 面。

## D3 std.fs

**源**（`stdlib/src/fs.we`）：

```we
pub type FsError = FsFailed(String)

pub fn readFile(path: String) effect io -> Result<String, FsError> { Err(FsFailed("fiction")) }
pub fn writeFile(path: String, data: String) effect io -> Result<(), FsError> { Err(FsFailed("fiction")) }
pub fn appendFile(path: String, data: String) effect io -> Result<(), FsError> { Err(FsFailed("fiction")) }
pub fn removeFile(path: String) effect io -> Result<(), FsError> { Err(FsFailed("fiction")) }
pub fn makeDir(path: String) effect io -> Result<(), FsError> { Err(FsFailed("fiction")) }
pub fn removeDir(path: String) effect io -> Result<(), FsError> { Err(FsFailed("fiction")) }
pub fn listDir(path: String) effect io -> Result<List<String>, FsError> { Err(FsFailed("fiction")) }
```

（虚构体在真源里就是这些字面 Err 构造——检查机器消费它们，发射机器按键替换。）

**面形状**（裁决 3：黑盒整文件）：

- 七枚全是整文件粒度；无句柄类型、无流式读写、无权限位参数。
- `makeDir` = 单层 `mkdir(path, 0755)`（不递归——`mkdir -p` 语义不批）；`removeDir` = `rmdir`（仅空目录，ENOTEMPTY → FsFailed）。
- `listDir -> Result<List<String>, FsError>`：条目名（非路径拼接——调用方自己拼）；**排序 strcmp 升序**（P1 确定性：同一目录两次列举同序；不承诺自然序）。
- **NUL 入径**：We String 可含 NUL（M12 的 (ptr,len) 双标量先例正因此）；C 侧 NUL 终止会把路径静默截断 ⇒ C 入口先 `memchr(p, 0, n)`，命中即 `FsFailed("path contains a NUL byte")`——**拒绝而非截断**。
- **错误消息形**：`"{op} {path}: {strerror}"`——op ∈ {read, write, append, remove, mkdir, rmdir, list}，strerror 用 `strerror(errno)`。Ok 路径零消息。
- readFile/writeFile/appendFile 走整读整写（`fopen`/`fread`/`fwrite`/`fclose`，尺寸先 `stat` 或循环读至 EOF）。

**C ABI**（D7-2 out 参形统一）：String 参数 = (ptr, len) 双标量展开（M12 先例）；`Result<String,_>` 与 `Result<(),_>` 皆三字 out；`Result<List<String>,_>` 的 Ok pay0 = list 句柄。C 侧构 List：`__we_list_new(0, 1)` + 逐条目 `__we_list_push`（String 元素按 T11-② 的 32B 无描述符盒装箱——C 侧直接 `we_str` 对 + 盒，map=null）。

## D4 std.process

**源**（`stdlib/src/process.we`）：

```we
pub type ProcessError = ProcessFailed(String)

pub record ProcessResult { exitCode: Int64, stdout: String, stderr: String }

pub fn run(cmd: String, args: List<String>) effect io -> Result<ProcessResult, ProcessError> { Err(ProcessFailed("fiction")) }
```

**面形状**（裁决 3：捕获式 spawn）：

- `run`：fork + `execvp`（PATH 搜索——`cmd` 不含 `/` 时按 PATH 找，含则直用），子进程 stdout/stderr 双管道捕获，父进程 `poll(2)` 轮读双管至 EOF（**避免顺序读的死锁面**：子进程写满一管时另一管的阻塞读会饿死），`waitpid` 收割。
- `exitCode`：正常退出 = wait status 的退出码；`WIFSIGNALED` → `128 + sig`（shell 惯例编码）；spawn 失败（execvp 报错）→ `Err(ProcessFailed("exec {cmd}: {strerror}"))`。
- **stdin 继承**（文档化行为）：子进程 stdin = 父进程 stdin。喂入面不批（非目标）。
- `args` 为 `List<String>`：C 侧 `__we_list_len` + 逐元素读盒解 `we_str` → `argv[]`（NUL 终止 char* 数组，末位 NULL）。argv[0] = cmd 本身（execvp 惯例）。
- **ProcessResult 的 C 侧构造**：`allocRecord` 协议形——`__we_alloc` 32+n、写冻结头、字段按声明序（exitCode@16、stdout.ptr@24、stdout.len@32、stderr.ptr@40、stderr.len@48——具体偏移随 layout 实测钉，design 不写死）。**map=null**：字段集是 Int64 + String×2，String 字节在 gc 域外（malloc 域，follow-up #21）⇒ 零描记引用——T11-② 同判据。调用方 load 句柄后立即再扎根（D2 纪律）。
- **阻塞与调度器**：`run` 是真实 io 阻塞（非协程 yield）——单线程协作调度器下，并发程序里 `process.run` 会阻塞整个调度器。诚实立场：文档化「`process.run` 阻塞至子进程退出」；与 runtime 的集成（netpoll 化）归后续。测试域同理（io/net 不虚拟——ch20 立场，mock 是隔离之路）。

## D5 std.string

**源**（`stdlib/src/string.we`，真体）：

```we
pub fn join(parts: List<String>, sep: String) -> String {
    var acc = ""
    var first = true
    for p in parts {
        if first { first = false } else { acc = acc + sep }
        acc = acc + p
    }
    return acc
}

pub fn repeat(s: String, n: Int64) -> String {
    var acc = ""
    var i: Int64 = 0
    while i < n {
        acc = acc + s
        i = i + 1
    }
    return acc
}
```

- **纯函数**：零效果段（无 `effect` 声明 = 显式纯，ch16）——从任何上下文可调。
- **体只用已拓宽绿面**（B1b 落地记录）：带注解 `List<String>` 的 for-in（T11-② 后绿）、`+` 拼接与 var 重赋值（B1a T4 全链）、无标注 `var acc = ""` → `bindStringSlot`（T10 簇 8）、`while` + `Int64` 计数（M9b 起绿）。**避开两处已知停面**：`List<String>` 上的回调（`listElemWord` skStr 守卫——join 不用组合子，for 直接拿元素）、`Option<String>` 载荷臂读（不涉及）。
- `if first { first = false } else { acc = acc + sep }`：臂一致（Unit vs Unit——赋值语句无值，块尾无表达式 = `()`，两臂皆 unit）✓。
- 性能非目标：O(n²) 拼接可接受（每轮 `__we_str` 新缓冲）；Builder 归 B2b。

### D5-3 开放验证项（T4 阻塞前置）

**std 模块的 define 发射是否无改动可用**：M10b 管线（`EmitProgram` 两遍 + per-module import 面 + 槽 `@<key>.<name>`）从未发射过 std 模块的**真体** define——今日 std fn 全 keyed（body 被替换）。join/repeat 是首批。三个子问：

1. 槽物化：`@std.string.join` 槽形是否与既有槽一致（keyed fn 的槽 vs 真 define 的槽——M10b 对两者应同形，但无先例证）。
2. 泛型零涉：join/repeat 非泛型 ⇒ 单态化机器零涉及（真体不含类型参数）。
3. import 面擦除：`import std.string` 的擦除是否与 std.io 同路（M8 擦除仅 std 段——`stdImports` 零改动即证）。

**判定法**（T4 首项）：真机探针——最小项目 `import std.string` + `io.println(string.join(...))` 走 check/build/run 三面。**过** → 直接真体；**停** → 定位停点，若是 std 模块 define 的管线缺口且修复面 ≤ 既有管线参数化（非新机器），修复 + 披露；**否则回退** = join/repeat 转 keyed C 助手（`__we_str_join`/`__we_str_repeat`，C 侧循环拼接），真体改虚构体，**B2b 或 B3 前再开**——回退是披露不是失败（装载机制不受影响，D1 的六键不动）。

## D6 E0816 收口

**翻转两点**（`typecheck.go`）：

- `:6968`（集合和式）：锚定族经 `collectionMembers` 解析如旧；其余成员 → **E0816**（不再 `bndStdModules`）。
- `:7022`（基类型含 String）：`stringMembers` 四员解析如旧；其余 → **E0816**。

**词退役**：`:51` `bndStdModules` 常量删除（两产出点已翻，词无宿主）。

**消息形**（一码一消息纪律：以注册表 title 起头）：

```
E0816: no such member on the receiver's type — `length` is not a member of `String`
```

detail 点名成员名与接收者类型渲染（`String`/`List<T>` 渲染循既有 displayType 面）。helps 引注册表 remediation。

**规范正确性**（勘查钉死）：

- `s.length`：ch17 只批 `byteLength`/`byteSlice`/`runeCount`/`charAt`；`length` 无处批准。
- `xs.count()`：ch11 组合子在 `Iterator`；`List` 锚定族 = `get`/`has`/`iterator`（ch17）。裸 `xs.count()` 今日 exit 70 未锚定——用户拼写是 `xs.iterator().count()`（链形，`iterator` 锚定解析 + Iterator 接口成员解析照旧，**不受翻转影响**）。
- `io.println("x")` 的遮蔽形：`io` 是 `Int64` 局部——基类型无成员方法（ch7/ch10），E0816 是唯一正确答案。

**黄金重锚 2 枚**（裁决 4 点名，保名保源，exit 70 + 边界行 → exit 1 + E0816 行）：`check-bnd-std-member`、`check-stdio-shadow-let`。

**爆炸半径验证**（T5 硬验证项）：翻转前 corpus grep 边界行恰 2 枚；翻转后 0 枚 + 全语料绿。语料外裸形（`xs.count()` 等）从 70 翻 1——**这是收口的意图**（诚实诊断替代边界行），以探针记入完成记录。

**不翻的相邻面**（守边界）：`stdImports` 外的未知 std 段名照旧 E1302（模块不存在 ≠ 成员不存在——两码两事）；接口/Dyn/记录面的成员解析照旧（E0814–E0817 既有机器）；`assertEqual` 域门照旧（编译器面非 E 码，M10a D8）。

## D7 运行时 C

### D7-1 新文件

`runtime/c/fs.c` + `runtime/c/process.c`。接线三处：`runtime/runtime.go` embed 集（`c/*.c` 通配即含新文件——若为显式清单则补行）；link 参数（clang 三步序的输入集随 embed 集走）；C harness 编译集（M10c `TestIOHarness` 先例——测试侧若有显式文件集 `{gc,sched,test,io,main}`，扩 `{...,fs,process}`，按实际清单核对）。

### D7-2 ABI 定形：out 参形为主选

**主选（采纳）**：sum 返回以 out 参交付——

```c
void __we_fs_read_file(const char *p, long long n, long long out[3]);
void __we_fs_list_dir(const char *p, long long n, long long out[3]);
void __we_proc_run(const char *cmd, long long cmdn, void *args, long long out[3]);
```

IR：`declare void @__we_fs_read_file(ptr, i64, ptr)`；发射 = alloca `[3 x i64]` + call + 三 load → sum 槽。**理由**：(a) 与 `__we_handle_await(h,*payload)` 的单出参槽先例同族（M9b ABI 定形）；(b) 调用方显式分配/装载，发射器无需理解 C 结构布局；(c) 出参数组即 sum 三字 ABI（T9-1）的直接投影。

**被拒替代（记录）**：C 结构返回 `struct {i64,i64,i64}`——24B > 16B 触 SysV MEMORY class ⇒ 隐式 sret 首参（C 侧看不见、IR 侧多一个隐藏指针参数）——能工作但把 ABI 耦合进 C 编译器的分类规则；out 参把同样的事显式化。**被拒理由**：ABI 耦合 + declare 形随平台分类漂移的风险，对照 println 的显式 declare 传统。

**String 参数** = (ptr, len) 双标量（M12 跨界先例，We String 可含 NUL 故永不单指针）。`args: List<String>` = 单指针（list 句柄，C 侧 `__we_list_len`/元素盒读）。**Never/Bytes 不涉及**（fs/process 面无）。

### D7-3 输出缓冲纪律

所有 String 输出（读到的文件内容、捕获的 stdout/stderr、错误消息、listDir 条目）走 `we_str` malloc 域分配——**永不回收**（follow-up #21 姿态：泄漏有界于程序自身的 io 量、存活于进程生命周期）。不为 fs/process 开回收先例；回收归 ADR-0003 门后的分配器工作。

## D8 黄金矩阵

| 族 | 枚数 | 形 |
| --- | --- | --- |
| E1401 效果门 | 1 | `check-fs-effect`：纯 fn 内调 `fs.readFile` → E1401（io ⊄ ∅） |
| fs 往返 | 4–5 | `run-fs-roundtrip`（write→read→append→读回校验→listDir 序校验→remove→removeDir 全链 exit 0）；`run-fs-read-missing`（读不存在 → `Err` → main report `error: FsFailed: read …: No such file…` stderr + exit 1）；`run-fs-listdir-order`（多文件 strcmp 序）；`check-fs-nul-path` 若可行（NUL 入径 We 源不可拼写——rune `\0` 进插值？`\u{0}` 是闭集逃逸——可拼！rune→String 需拼接面，T2 实测定形，不可拼则 C 侧单测钉 + 披露） |
| process | 2 | `run-proc-run`（`/bin/echo hi` → exitCode 0、stdout "hi\n"）；`run-proc-fail`（`/bin/false` → exitCode 1 捕获；或不存在命令 → Err） |
| string | 2 | `run-string-join`（含空表/单元素/多元素）；`run-string-repeat`（0 次 → ""、n 次） |
| E0816 重锚 | 2 | `check-bnd-std-member`、`check-stdio-shadow-let`（保名保源，70→1） |
| mock | 1 | `test-fs-mock`：`mock fs.readFile` 后调用返回桩值（ch20 面） |
| `?` 集成 | 1 | `run-fs-question`：fn 内 `let c = fs.readFile(p)?` 传播链（B1b T12 propagate 臂 + fs 真运行时） |

**突变电池**（T6）：(a) fs.c 的 Err tag 翻转（1↔0）——fs 黄金族死；(b) out 参三字乱序——往返黄金死；(c) listDir 排序摘除——序校验黄金死；(d) NUL 检查摘除——NUL 黄金/单测死（若黄金不可拼则单测层独守 + 披露）；(e) process exitCode 不收 WIFSIGNALED——proc 黄金单测双钉；(f) join 真体 sep 摘除——join 黄金死。每枚锚点 `count==1` 断言 + 逐枚还原。

**红先纪律**：fs/process/string/E0816 全部新增黄金在实现前先跑红（fs 族红于 E1302/E0816/边界行按任务序自然成立——T2 落装载、T3/T4 落面、T5 落翻转，各任务的黄金在本任务实现前取红态）。

## 实现期补记一（T1，2026-09-15）

1. **D1-3 表的 `-> ()` 拼写订正（渲染级失准，非语义差）**：今日合成 AST 的 io/test/sleep 条目 **Ret 与 Body 皆 nil**——fn 型渲染时 nil Ret 呈 `-> ()`，但源码不可如此拼写（写 `-> ()` 解析出的 Ret 非 nil，与「逐字照抄」相悖）。三源采用**缺省返回形**（如 `pub fn println(s: String) effect io { }`——无 `->` 段），解析后 Ret/Body 同为 nil，与今日 AST 零差。表中 `-> ()` 列按此读。
2. **键约定 as-built**：`StdModule` 键是**点分全名**（`"std.io"`……，`m10b_test.go:63` `StdModule("std.time")` 与 `check.go` visit 分支双侧实证）；`stdlib.Sources()` 映射键为**裸段名**（D1-1 原样）。翻转以 `strings.TrimPrefix(key, "std.")` 桥接：`std.io.extra` → 段名 `io.extra` → 查表不中 → `(nil, false)` → E1302 std 形（与今日 switch 不中同路）。parse 缓存以 `sync.Map` 落地（D1-2 写「sync.Once/lazy map」——同族双检形，非偏差）。
3. **T1-② 的「六键」按终态读（数据驱动）**：策略表实际形态**数据驱动**——仅 `stdlib.Sources()` 在场的键翻转。T1 落三键（io/test/time）；fs/process/string 随 T2/T3/T4 源落盘**自动翻转、零码改**。`std.fs` 今日 → `(nil, false)`，E1302 std 形保持为 T2 的红先锚（`TestStdFsIsNotYetAStandardModule` 钉）。
4. **选 A 判决成立：24 枚仲裁黄金逐字节绿；两个次生面语料不可观测（披露）**：全语料 assertEqual 面清点 = **49 行、全部纯限定调用** `st.assertEqual(...)`——零 `mock st.assertEqual`、零值位引用（勘误：case 源在 `setup.files` 下，首次扫描按顶层 `files` 读漏了 43 行）。故选 A 的次生面（值位解析 E1304→E1004 泛名、mock 目标 E1304→E1804 泛 fn 类）**无任何黄金可观测**——按「已披露、无黄金覆盖」记档，不产生漂移。19 枚 `test-asserteq-*` + 5 枚 `check-assertequal-*` 定向 `-count=1` 全绿（分派序未动：拦截仍前置于 importSym 成员门）。
5. **concurrent 例外零动**：`stdConcurrentFile` 包级单例与其构造代码逐字节未动；单测以**同指针**断言（同一对象 ⇒「逐字段同旧」平凡成立），另钉 4 sum + 1 fn 形状计数。
6. **包数 15→16**：新根包 `stdlib` 入梯；`go test -count=1 ./...` 16 包全绿（14 ok + `cmd/we`/`internal/ast` 两枚 no-test 先例不变）。
7. **三缝记档**：① 全语料 881 枚 `-count=1` 绿零改写（270s）；② hello IR 对拍——HEAD `10827df` worktree 与本树双二进制 `we build`，`build/demo.ll` 30 行 `cmp` 逐字节零差，`@slot.time.now` 与 `@slot.io.println` 两槽俱在（io+time 双面覆盖；test 面由 24 枚黄金端到端覆盖）；③ mock 槽——34 枚 mock 面黄金（含 `test-mock-println`、`test-clock-now-stable`/`test-clock-now-tick` 虚钟两枚）随全量绿 ⇒ 槽物化同一。
8. **红先记录**：stdlib 门先红（`stdlib.go:18:12 pattern src/*.we: no matching files found` → FAIL setup）→ 三源落盘 → 双测试绿。

## 实现期补记二（T2，2026-09-15）

**两条任务书复选框全勾；红先记录 = fs 族黄金实现前红于 E1302（T1 的 `TestStdFsIsNotYetAStandardModule` 红锚随装载翻转成 `TestStdFsIsAStandardModule`），check-fs-effect 红于装载未含 fs。真机二进制复核五枚 run 行为 stdout/stderr/exit 三面（roundtrip 八标记 exit 0 + rt/ 清净、read-missing exit 1 精确 stderr、question config-ok、nul 拒、listdir M.txt/a.txt/z.txt 序）。**

1. **限定 sum 的导入位注册（D2/D3/D7 未名的缝，D8 黄金所需）**：`Result<_, fs.FsError>` 在调用方签名里要求 `fs.FsError` 可解——std 模块非程序模块，其 SumDecl 此前到不了任何 pass-one 走查。落 `registerStdSums`：import 走查的 `case "fs"` 把 `StdModule("std.fs")` 的 SumDecl 注册进 `e.sums/e.sumsOrd/e.sumDecls`（键 `std.fs.FsError`；幂等标记 `e.sumsOrd[modKey]=nil`）；`namedSumShapes` 的 Qual 分支经 `resolveStd` + `isLocalName` 守卫解析。concurrent 不受影响（concAlias 不在 stdQuals）。
2. **payloadShape 的 List 面摆渡**：List 载荷位把 `carrierElemFace` 的元素面装进 `fnParamAbi.list`；bindArmWord 的 abiGc 臂随之在 `p.list != nil` 时登记 `listEnv`——`Ok(xs)` 臂体 `for n in xs` 直接走查，零自身注解。D3/D7 只写了「listDir 答句柄」，未名此摆渡。
3. **emitMatch 直接调用 scrutinee 拓宽**：scrutinee 改走 `emitSumSource`（Ident 查 sums2、Call 发射）——`match fs.readFile(p) { … }` 直书可用（此前仅 Ident 绑定形）。 Bisect 阶梯 H（绑定+match 绿）对 I（直调 match 红）实证此为独立缝。
4. **questionOkFace 放开 abiStr + emitQuestion Ok 支答 ckStr**：`?` 的 Ok 侧今绑定恰一 abiI64 **或** abiStr 位（String 对子经 `wordPtr(inttoptr)` + len 读出）；abiGc（list/记录句柄）仍拒——**诚实停**，`TestQuestionOnListOkStillStops` 钉，黄金留待 T3（process.run 返回记录句柄）随其自身拓宽落。三处重锚随之：m6b 门表 `a String pair` 臂 false→true；黄金 `build-bnd-question-wide-ok-face` 翻绿重锚为 `run-question-string-ok-face`（保源改期望：run、exit 0、`v s`——用户方法路由的 String-Ok 绑定，与 run-fs-question 的 std 路由互补）；docs 双语关闭清单措辞订正（「Ok 载荷宽于一字」→「非一字亦非一对 String」，枚举元组/Float64 字位/被读作数字的句柄）+ 运行面段落补 stdlib-fsproc 拓宽句。
5. **D8 NUL 形判决：可拼**——`\u{0}` 闭集逃逸直接入 String 字面量（`"a\u{0}b"`），无需求 rune→String 拼接面；`run-fs-nul` 黄金落地（exit 0、`nul-rejected`）。数据内 NUL 经 (ptr,len) 对往返（C harness `read-nul-data=0[a\x00b]` 钉），路径内 NUL 前置拒——两面分开锚。
6. **C 侧 List<String> 生根协议 as-built**：精确容量 `__we_list_new(count, 1)` + 单次 `__we_root_push` + 盒装推入（容量精确 ⇒ 零增长 ⇒ 零身份变更）+ `__we_root_pop`；调用点再扎根（inttoptr + `__we_root_push`）镜像 emitCallCore 的 abiGc 臂。D3 的「走 T11-② 盒」是形状判据，生根时序是本补记的落地面。
7. **run-fs-question setup 字节修**：配置文件初带尾 `\n` 与 println 自加换行叠成双——setup 字节改 `config-ok`（无尾换行）。
8. **runtime harness 两条**：链集需 str.c（`__we_str_of_bool` 未定义先红）；路径长度按字节兑现（8 字节路径误传 7 时 C 侧逐字截取——该「教训」本身即 ABI 长度参数工作的证据，harness 已全数订正）。
9. **emitQuestion Ok-String 路径双载清理（IR 级）**：pay 槽原载两次，今单载；黄金面不可观测（语义零差），fs_test 以 lastDefinedReg 钉 Ok 支 inttoptr（体内两枚——Err 折叠在前、Ok 绑定在后）。
10. **计数**：conformance 881→887（六新增 + 一改名净零）；codegen 单测 as-built 451（fs 族 6 枚新增）；16 包全绿（14 ok + 2 no-test）；validate strict、docs_sync 33 对、`git diff --check` 净、staged 无 refr/。

## 实现期补记三（T3，2026-09-15）

**两条任务书复选框全勾；红先记录 = run-proc-run/run-proc-fail 两枚实现前红于 E1302（装载未含 process——T2 红先记录的同形延续）。真机二进制复核：echo 捕获 stdout 逐字节含尾换行（`hi\ncode 0\n`，`od -c` 复核）、false 空 stdout/stderr + code 1、PATH 搜索（`echo` 无斜杠）、stderr 捕获（`1>&2`）、信号形 137（`kill -9 $$` → 128+9）、缺命令 Err 臂命中（`/no/such/cmd`）。**

1. **空参数字面量 `[]` 停点与拓宽松（设计未名的缝）**：`emitListOperand` 对 `*ast.ListLit` 走 `emitListLit(v, nil)`——typ 为 nil 时 `listElemFace` 的两条取名路径皆断（`listArgOf(nil)` 不中、零元素无首元素可读），空表诚实停 bndMainBody（run-proc-fail 首跑实证）。拓宽：`emitProcRunCall` 的 ListLit 实参路由类型化路径——签名命名 `List<String>` 作 typ（非空字面量经 `elemFaceOfType(String)` 取得与首元素推导同一 skStr 面，行为不变）；复用既有已测的标注绑定路径（`let xs: List<Int64> = []` 同路），零新发射码。`TestEmptyArgsLiteralRidesTheTypedFace` 钉 `list_new(i64 0, i64 1)`。
2. **NUL 拒句泛化（设计未名 cmd/args 拒句）**：命令 → `"command contains a NUL byte"`、参数 → `"argument contains a NUL byte"`——fs 路径形（"path contains a NUL byte"）的泛化，皆拒而不截。
3. **失败消息的 op 词集**：`fail(out, op, cmd)` 形 `"{op} {cmd}: {strerror}"`，op ∈ {exec, fork, pipe, poll, read, wait}——设计只点了 exec 形；exec 是经 CLOEXEC 失败管传递的 errno（子进程写 errno + `_exit(127)`，成功 exec 由管道自证关闭——比 exit-code-126 魔数诚实：真程序可自发 126）。
4. **C 侧记录雕刻免根论证（D2 纪律的时序面）**：捕获缓冲先 malloc、后 carve `__we_alloc(56)`、carve 后零分配 ⇒ 入口全程无需自根；调用点 re-root（inttoptr + `__we_root_push`，load 与 push 之间零指令）承担持有。map 恒 null 按 T11-② 判据：字段集 Int64 + String×2 零描记字，分配器清零即终态。
5. **registerStdSums → registerStdModule 泛化（记录入注册，设计未名）**：`process.ProcessResult` 作 Ok 载荷要求限定 record 可解——import 走查的注册从 SumDecl 扩到 RecordDecl（`e.order` 追加 + `e.records[key]`，幂等标记同构）。`e.order` 追加惰性：std 记录永不 We 侧构造 ⇒ 无 usedRecs 标记 ⇒ 无 map 描述符物化。
6. **classType 限定 record 臂（新增）**：`n.Qual != ""` 经 `!isLocalName` + `resolveStd` 守卫解析 `"std."+sk+"."+n.Name` 入 `e.records` → `(abiGc, key, true)`——镜像 namedSumShapes 的限定解析；`payloadShape("Ok", [process.ProcessResult])` 依赖此臂。
7. **Err 臂绑定读入 String 洞仍停（诚实边界，不属本任务）**：真机二分实证 `Err(e)` + `"${e}"` 停 bndMainBody（信号形/PATH 形单独皆绿——停点在 Err 绑定的洞读，与 `?` 的 Err 侧同族）。C harness 直读 out 三元组覆盖 Err 消息形；We 侧黄金以 `Err(_)` 锚臂命中（真机另证）。
8. **println/print 黄金教训（与补记二 run-fs-question 同族）**：`io.println(pr.stdout)` 对已带尾换行的捕获文本叠出双换行——改 `io.print`，黄金期望 `hi\ncode 0\n` 逐字节。
9. **C harness 十一面**：echo 捕获 / PATH 搜索 / false 空+code 1 / exit 3 / stderr 捕获 / 双管同捕 / 信号 137 / **死锁面**（`head -c 70000 /dev/zero 1>&2; echo tail`——stderr 越管道容量先于 stdout 首字节，顺序读者死锁、poll 轮读者通过；长度打印保 want 可读）/ 缺命令 exec 报 / NUL 命令拒 / NUL 参数拒（手工盒——C 字符串不能携终止符后字节）。`prec` 读记录 16/24/32/40/48 钉 C 侧雕刻对发射器布局。
10. **mkargs 载体构造纪律**：精确容量 `__we_list_new(n, 1)` + 首 root + 盒装推入（ptr@16/len@24）——镜像 emitListLit 的字面量发射；容量精确 ⇒ 零增长 ⇒ 身份恒定。
11. **codegen 单测六枚**：四操作数调用（slot+declare+out 三元组+gep 0/8/16+操作数计数）、Ok 句柄再扎根（首个 inttoptr+push）、记录字段布局读（臂绑定 gep 24/32/16）、限定 ProcessError 熔合（helper define 三字返回 + `icmp sge`）、空表类型化面、`?` 仍停（bndMainBody——docs `?` 关死清单措辞随之订正：list 句柄 → list 或 record 句柄）。
12. **计数**：conformance 887→889（两枚新增）；codegen 单测 451→457（六枚新增）；runtime 单测 +1（TestProcHarness）；门两测扩（TestEmbeddedSourceSet +process、TestStdModuleLoadsParsedSources +std.process 行）；16 包全绿；validate strict、docs_sync 33 对、`git diff --check` 净、staged 无 refr/；docs 双语 stdlib-fsproc 句加 process 从句。

## 实现期补记四（T4，2026-09-15）

**三条复选框全勾。D5-3 结论 = 修复（写回任务行）：两处参数化放行，emitCall 零改动。红先记录 = run-string-join/run-string-repeat 两枚实现前红于 E1302（装载未含 string——T2/T3 红先的同形延续）。真机复核六面皆 exit 0：join 空/单/多元素（``/`a`/`a-b-c`）、repeat 0/3 次（``/`ababab`）、嵌套实参（`string.join(many, string.repeat(", ", 2))` → `<a, , b, , c>`）、插值洞（`"r=${string.repeat("x", 5)}"` → `r=xxxxx`，嵌套引号可解析）、纯上下文（无 effect 段的 helper 内调用 → `[a,b]`）。**

1. **探针记录（D5-3 的修复判决依据）**：首跑 E1302 为陈旧二进制（probe 二进制建于 string.we 嵌入之前）——重建后过 E1305（main 裸 `pub fn main() effect io` 被拒，须 `Result<(), E>` 形），订正探针后 **check exit 0 / build exit 70**。二分定位：join、repeat 单独导入皆停 ⇒ 停点在 import 走查的 `default` 分支（codegen.go:1778 一族）而非首判的 emitCall 回退。修复两处：`internal/cli/build.go:378` programModules 放行 `std.string` 乘程序面；`internal/codegen/codegen.go:1771` import 走查 `case "string"` 落 stdQuals + curImports（:1783）。修后 build 0 / run 0。
2. **emitCall 零改动的结构理由**：调用路由全走既有机器——resolveQual（curImports 命中）→ instCallee（fnTable 取 `std.string.join`）→ emitFnCall（`slotFor(fd.sym(), fnSlotTarget(fd))`）。修复落在「谁进 curImports / 谁乘程序面」两个集合上，不在调用发射本身。
3. **D5-3 三子问的答案**：① 真体 define 的槽与键控槽同形——`@slot.std.string.join = global ptr @std.string.join`，槽指向 define 自身，调用点 `load ptr, ptr @slot.std.string.join` + 间接调用（TestStringJoinEmitsDefineAndSlot 钉）；② 泛型零参与——join/repeat 皆非泛型，instCallee 直取 fnTable，无 instShape 交叠；③ 擦除同路——修复前 check 即 exit 0（import 消除与键控 std 同路，stdImports 不变），停点全在 build 段，故装载侧零改动。
4. **白名单安全论证（为何只放 std.string）**：resolveQual 的 modKeys 守卫使 curImports 条目只对乘程序面的模块生效——键控 std 模块的键不在 modKeys，其槽仍指 C 侧条目（io 若入 curImports/modKeys 将破 `@slot.io.println` 路由）。放行是枚举白名单（`m.Key != "std.string"`），非前缀匹配；每多放一枚即多一份「真体承诺」，由 D5 的名单而非启发式决定。
5. **string.we as-built（D5 逐字）**：join 的 first 标志形（for + `if first {} else {}` + 拼接）、repeat 的 while + Int64 计数器。`List<String>` 参数经 abiGc 载体臂过界（载体指针一字），String 返回 `{ ptr, i64 }` 对；体内 list_snap/root_push/list_len/list_get + 盒读 gep 16/24 + `__we_str_concat` 全为既有绿面——真体的全部新材料是源本身。空表经标注绑定过界（`let none: List<String> = []`，既有路径；T3 的类型化字面量路由是 proc.run 内联实参专用）。
6. **黄金两枚 + 两教训**：E1305 迫使黄金源取 AppError 形（`pub type AppError = Failed(String)` + `-> Result<(), AppError>` + `return Ok(())`）；repeat 0 次的 `""` 经 println 打出空行——期望 `"\nababab\n"` 首行为空（初写漏，真机复核捉）。
7. **单测三枚 + 两教训**：define 钉必须带形参名——`(ptr, ptr, i64)` 与 `(ptr %parts, ptr, i64)` 皆 miss（实际 `%sep0` 跟在裸 `ptr` 后），全形 `(ptr %parts, ptr %sep0, i64 %sep1)` / `(ptr %s0, i64 %s1, i64 %n)` 过；第三枚初稿宣称测遮蔽而体未遮蔽——真机遮蔽探针（`let string: Int64 = 7` + `string.join(...)`）实证停 **bndStdModules**（既有界，恰是 T5 要退役成 E0816 的词）后，改写为纯上下文钉（TestStringJoinCallsFromAPureContext，真机先证 `[a,b]` exit 0）。
8. **遮蔽停面披露（既有，非本任务引入）**：局部绑定压过模块限定名 → exit 70 bndStdModules；docs 双语 closed 清单已入列（T5 落地时随词退役订正）。
9. **门两测扩**：stdlib_test want 集 +string（{fs,io,process,string,test,time}）；b2a_test 行 `{"std.string", [join, repeat]}`——装载器只管名，body 真伪是管线的事（行注释自陈）。
10. **计数**：conformance 889→891（两枚新增）；codegen 单测 457→460（三枚新增）；16 包全绿；validate strict、docs_sync 33 对、`git diff --check` 净、staged 无 refr/；docs 双语 stdlib-fsproc 句加 string 从句 + closed 清单加遮蔽停面。

## 实现期补记五（T5，2026-09-15）

**三条复选框全勾。翻转前记档：corpus 边界行恰 2 枚（check-bnd-std-member、check-stdio-shadow-let）；裸 `xs.count()` 探针 exit 70 + 边界行。翻转后：0 枚 + 891 全绿；裸形翻为 exit 1 + E0816。**

1. **两产出点翻转 as-built**：设计行号 :6968/:7022/:51 漂移至 :6956/:7010/:54（后续提交先行插入所致）。消息形 = 注册表 title 起头 + detail `%q is not a member of %s` + 与 namedType 落空臂同一尾句（"member access names a field or a method of the receiver's type"）；接收者型渲染循 `t.String()`——namedType 出 `List<Int64>`（含实参）、baseType 出 `String`/`Int64`，即设计的 displayType 面。设计速写的反引号形（`` `length` is not a member of `String` ``）按一码一消息纪律折衷为 %q/%s 渲染：本仓实态是 title 恒为注册表原文、detail 逐站点定制、既有站点皆用 %q/%s 无反引号——纪律高于速写。
2. **helps 零新码**：typecheck.go:95 的 help 表已有 E0816 → 注册表 remediation 逐字（"Fix the name, or declare the member; unknown-field access on records lands here too."），c.fail 自动挂载（m8 重锚的 JSON 断言里可见）。
3. **爆炸半径实为 2+3**：corpus 边界行 2 枚如记档；**三枚**单测引用该词——m5（`s.length`）、m6a（`xs.add(1)`）、m8 TestStdIoShadow（遮蔽 `io`）——设计勘查只数了 corpus，第三处是翻转后编译器 undefined 揪出的。三枚全部重锚 wantDiag（2:15 / 2:17 / 5:8；列号手算错一次 18→17、行号错一次 4→5，钉住即改）。
4. **黄金重锚 2 枚**：保名保源，exit 70 → 1 + E0816 行**逐字节取自真机输出**（WE_UPDATE_GOLDEN 未用）。
5. **语料外裸形翻转（收口的意图，探针记档）**：裸 `xs.count()` 翻转前 70 → 翻转后 `error[E0816]: … "count" is not a member of List<Int64>` exit 1。
6. **不翻的相邻面真机记档**：std.bogus → E1302 逐字同前（模块不存在 ≠ 成员不存在，两码两事）；`xs.iterator().count()` 链形 run 0 打 `3`（锚定族 + Iterator 接口解析照旧）；记录未知成员 → E0816 record 形（`"Cell" has no member "missing"`）、接口方法缺失 → E0816、Dyn 未知方法 → E0815——E0814–E0817 既有机器零改动。
7. **遮蔽面从停面变诊断的 docs 订正（补记四预告的兑现）**：T4 刚入 closed 清单的遮蔽条款撤下；「已离场」叙述补一面——非靠拓宽而靠诚实分类（`Int64` 无成员 ⇒ E0816 是唯一正确答案；exit 70 之位换 exit 1）。
8. **注册表零增量**：T5 全程用已批语义（ch10 成员解析 + E0816 在册条目），proposal 的「零规范增量、不改语言行为」兑现——翻转把「检查器知道答案却停给边界词」改为「检查器说出答案」。
9. **计数**：conformance 891 不变（两枚重锚非新增）；typecheck 单测计数不变（三枚重锚）；16 包全绿；validate strict、docs_sync 33 对、`git diff --check` 净、staged 无 refr/。

## 实现期补记六（T6，2026-09-15）

**三条复选框全勾。mock 面红先记录 = 严格红证据（同一黄金源 × 父树 `4a061e4` worktree 二进制）`we test .` exit 70 + mockTarget 的 `e.bnd()` 边界行（fs 表缺席）；实现后真机双绿——拦截（stub 值 assertEqual 过）与恢复（真实 C 入口 ENOENT → Err 臂）皆成立。突变电池六枚逐枚施、逐枚还原（还原后双绿再下一枚），电池毕后 `go test -count=1 ./...` 16 包全绿。**

1. **mock 面 as-built（mockTarget 的 outTrio 旗标 + 双 define）**：fs 族走自己的解析支——`fsEntries[md.Target]` 命中即返 `(abi, "fs."+target, "@"+ent.sym, true)`（槽键 `fs.readFile`、恢复默认 = C 入口 sym）；`process.run` 维持诚实停（Ok 半是记录句柄——「no mock body has yet」的载荷面）。outTrio 目标的 mock 骑两个 define：体在 `<key>.mock.<n>.body`（声明拼写的 sum ABI `{ i64, i64, i64 }` 原样返回），槽装的 `mockOutWrapper` 手装 void wrapper（调用形 = 调用者字 + 尾 `ptr %__out`，转发体 define 后 `extractvalue %m, N` ↔ `gep 8*N` 三字落 out 块——寄存器序 = C 侧读者所期）。wrapper 为手装文本不入 `e.inst`，体的 SSA 计数器不受扰、两 define 独立。mock 的 ABI 判据仍是声明拼写的 sum（`fnAbiOf(&ast.FnDecl{Params, Ret})`），不因 outTrio 变形——「目标自己的调用形」是安装的透明规则，非体的 ABI。
2. **单测钉一枚（TestFsMockRidesTheOutTrio）**：经真测试模块管线——`typecheck.CheckTestRoot`（std.fs 作 dep 摄入、FsError 注册经此可解；程序面滤除 std 模块，镜像 `cli/test.go` 的图规则）。八钉：槽默认 = C 入口、install/restore 两 store、体 define 名 + sum ABI、wrapper define 的 out-trio 调用形、转发调用、第三字 extractvalue、`%__out` +16 的 gep。
3. **突变电池判决表（D8 六枚，每枚锚点 `count==1` 断言先立）**：

   | 枚 | 突变 | 单测层 | 黄金层 | 存活面 | 机理 |
   | --- | --- | --- | --- | --- | --- |
   | (a) | fs.c Err tag 翻转（`fail`/`nul_fail` 的 `out[0]=1`→`0`） | **死**（TestFsHarness 5 面 Err 翻 0） | **死**：run-fs-read-missing + test-fs-mock（恢复面 Ok 臂 `assert(false)`——mock 黄金跨层免费击杀） | roundtrip/question/listdir-order/nul（happy path 与 nul_fail 面不消费 Err） | Err 消息对被当 Ok 载荷消费 |
   | (b) | `ok_str` 三字旋转乱序 | **死** | **死**：run-fs-roundtrip + run-fs-question（皆消费 String 载荷） | listdir-order（自有 out 写）/nul（nul_fail）/read-missing（fail）/test-fs-mock（mock+fail 面） | tag 槽落长度字（非零 ⇒ 误入 Err 臂）+ 载荷两字对调 |
   | (c) | listDir 排序摘除（插入排序双循环整段） | **死** | **死**：run-fs-listdir-order | — | readdir 序与 strcmp 序真实分歧（首项 `sub`）——确定性击杀，P1 的可观测面 |
   | (d) | `path_ok` 的 memchr 摘除 | **死**两面：`nul-read=1`（截断路径 ENOENT 误报错）+ **`nul-mkdir=0` 静默错行为**（截断名 `a` 真被 mkdir 成功） | **死**：run-fs-nul（强化件立功，突变树打 `nul-opened`） | — | 「拒绝而非截断」的立意实证：截断名是另一个文件，不是错误 |
   | (e) | WIFSIGNALED 臂断路 | **死**（TestProcHarness `sig` 面翻 `-1`） | **死**：run-proc-signal（`sig-bad -1` 对 `sig-ok`） | run-proc-run/run-proc-fail（WIFEXITED 面） | 信号死退 `code=-1`（else 支），128+sig 外壳约定丢失 |
   | (f) | join 真体 sep 摘除（`acc + sep` 恰 1 处） | **存活**（string 三枚皆 IR 形状钉——define/槽/concat 形在场即过，不问 sep 之值） | **死**：run-string-join（`a-b-c`→`abc`） | run-string-repeat（repeat 体独立） | sep 并置摘除后多元素并集失真；黄金层独杀的诚实记录 |

4. **电池伴生的黄金强化与新增**：run-fs-nul 强化（setup 落 `"a": "X"`——无此文件时截断路径 ENOENT 会伪装成拒绝，强化后 (d) 的突变才可观测）；run-proc-signal 新增（`kill -9 $$` → 137 面——D8 process 族写 2 枚，信号编码面因 (e) 需可死黄金而补第三枚；`kill $$` 的 `$$` 在 `/bin/sh` 下是子 shell 自身 PID）。
5. **验证阶梯八面**：`go build`/`go vet`/`gofmt -l`（0 文件）/`go test -count=1 ./...`（14 ok + 2 no-test 零 FAIL）/validate strict（1 change valid, registry clean）/docs_sync 33 对/`git diff --check` 净/staged 无 refr/；`WE_UPDATE_GOLDEN=1` 未用（黄金全手写）。
6. **计数**：conformance 891→893（test-fs-mock + run-proc-signal）；codegen 单测 460→461（TestFsMockRidesTheOutTrio）；16 包全绿。
