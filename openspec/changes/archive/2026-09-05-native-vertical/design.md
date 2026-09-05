# design.md — native-vertical

机制自由度的固定记录。每条决策给出规范依据（章 R 号 / ADR / 注册表）；无依据处显式标注为机制读法并给出论证。IR 与 C 的具体拼写在此钉死为事实上的稳定表面（符号名、行格式、路径名都是后续里程碑要依赖的 ABI 面）。

## D1 产物布局、三面与 `we clean` / `we run` 契约（裁决 Q2）

**产物与中间产物**：`we build`（项目、executable）在项目根创建 `build/`，落四个文件：

```
build/<name>.ll        # 代码生成发射的文本 IR（模块名 = manifest name）
build/rt-startup.c     # runtime/ 内嵌 C 源按需落盘（design D6）
build/rt-alloc.c
build/<name>           # 链接产物：clang driver 链 .ll 与两个 .o
```

`<name>` 取 manifest `name` 键（已过 E1904 校验，字符集 `[a-z0-9-]` 对文件名安全）。中间产物（.ll / rt-*.c / .o）留在 `build/` 不删——机制选择：可检视、可调试、clean 全删（ch21 无中间产物承诺，也无保留承诺）。

**成功三面**（与 check 同形，ch21 R2「同输入同输出」含输出面）：静默 exit 0；`--json` 零事件；`--verbose` 恰一行 `we: built build/<name>`（相对项目根的路径）。

**`we run`**：先跑完整 build（含三面），后执行 `build/<name>`：子进程 stdio 直通（不捕获、不改写——子进程的输出就是程序的输出），cwd = 解析出的项目目录，环境继承。退出码传播：子进程码 ≥ 0 → 原样返回；信号终止（Go `ExitCode() < 0`）→ stderr 一行 `we: <artifact path> terminated by signal`、exit 1（机制：M4 合法程序只会正常退出，此分支只为崩溃留一个诚实报告位）。

**`we clean`**：删除项目根 `build/` 整目录（`os.RemoveAll`）；幂等——目录缺席时静默 exit 0（三面皆零输出，ch21 R1「移除构建产物」：移除空集是成功）；`--verbose` 时先按字典序列出 `build/` 下将被删除的每个文件路径（`we: removed build/<rel>`），末行 `we: removed build/`（目录缺席时不打印任何行）。对 `.we` 文件路径：usage 错误 exit 2（`clean wants a project directory, got %q`）——机制读法：clean 的对象是构建产物，单文件编译在本切片无工件面（Q3 边界），把文件路径留给 usage 错误比留边界行更诚实（这不是「未实现的形式」，是「无意义的命令形」）。

**`we clean` 不触依赖面**：ch22 R5 明言 fmt/clean 不做 acquisition——`we clean` 不读 manifest 内容（只删目录），对无 we.toml 的目录也成功（`we clean` 一个空目录 = 删不存在的 build/ = 幂等成功）。披露：这与 check/build/run 的 E1905 前置不同，是 ch22 R5 的直接后果。

## D2 管线共享与两条边界序（裁决 Q3 / Q4）

**项目加载抽取**：`internal/cli/check.go` 的 runCheckProject 主体抽为 `(e *env) loadProject(dir) (*ast.File, string, int)`——manifest 读取与三键校验（E1905/E1904/E2004/E1903 原序原报文）、src/main.we 读取（E1305）、解析（诊断/边界）、类型检查（Project 模式）。返回根模块 AST 与 manifest name；失败时已报告并返回退出码。`we check .` 调它后走 checkPassed；`we build` / `we run` 调它后穿代码生成。**行为零改写**：101 枚既有黄金不动、全绿为证（T6 验证）。单文件侧同理抽 `loadFile(path)`（parse + SingleFile 类型检查）。

**依赖面前置**（ch22 R5：acquisition 在 module resolution 之前）：loadProject 在 manifest 校验后、src/main.we 读取前检测 `[dependencies]` 段——缺席或段内零键 = 空集平凡满足（M3 行为不变）；段内至少一键 → 边界行 `we: pipeline commands with a non-empty dependency set (chapter 22) are not implemented in this reference build yet`，exit 70。检测是对 manifest 原文的机制读法（极简读取器增一段感知：`[dependencies]` 行后到下一 `[section]` 前的 `key = "value"` 行计数）。check/build/run 一致适用（ch22 R5 对六命令一致）；fmt/clean 不适用。

**单文件 build/run 的边界序**（Q3）：`we build tool.we` / `we run tool.we` 先跑完单文件管线（词法/语法/类型诊断正常报告 exit 1——「形式到达才出边界」的既立原则），类型干净后在工件命名处停边界：What = `single-file builds (spec gap; roadmap follow-up)`，exit 70。规范缺口：ch21 R2 说 build 产「manifest 命名的工件」，单文件编译无 manifest——工件无名无位。follow-up #6 登记（归档动作），离开 follow-up 表只能经规范层变更。

**library 的边界序**（Q4）：loadProject 校验三键全过后（library 是 `type` 的合法值——E1903 只拒非法值），build/run 在读 src/main.we 之前停边界：What = `library artifacts (chapter 21)`，exit 70。序的论证：工件种类由 manifest 决定，先于一切源码工作；这也避免「library 项目无 main → E1305」的伪诊断（M3 的 check 对 library 项目同样要求 main——那是 check 已交付的行为，本变更不改写；build 在更早处停边界，两个子命令各自诚实）。`we check .` 对 library 项目维持 M3 行为（check 无工件面）。**两边界同在场时依赖边界先触发**（library 且非空依赖 → 依赖边界行）：acquisition 按 ch22 R5 先于一切管线工作，工件种类判定其后——两行都是 exit 70，先后的可观察差异只在 What 文本，钉死以求确定。

## D3 代码生成接受集与边界 What 表（裁决 Q1）

类型检查已保证类型干净；代码生成在 AST 上做**形状识别**——本切片只接受两个已裁决的 main 体形状，其余类型干净形式停边界。这不是用语法替代语义（类型阶段已跑完），而是已裁量子集的成员判定。

**模块级接受集**（零或多项和式声明 + 恰一个 main 函数——main 恰一个是 Project 模式类型检查 E1305 已保证的）：

- `SumDecl`（任意前缀 pub/byval、任意变体表）：**擦除**——零 IR。和式值在 M4 的两个 main 体形状里不出现为运行时值（Ok 载荷是 `()`、Err 载荷直接进报告行常量）。
- `FnDecl` 且 `Name == "main"`：进 D4 的形状识别。
- 其余任何顶层项（`FnDecl` 非 main、`TopLet`）→ 边界行。import 不可能到达（M3 在类型阶段已对 std/多模块停边界）。

**main 体接受集**（体 = 恰一条语句，`return` 形）：

| 形状 | 判据（AST 模式） | 编译为 |
| --- | --- | --- |
| Ok 路径 | 体恰一项 `Return{HasValue, Value = Call{Fn: Ident"Ok", Args: [Unit]}}` | `ret i32 0` |
| Err 路径 | 体恰一项 `Return{Value = Call{Fn: Ident"Err", Args: [Call{Fn: Ident V, Args: [Literal string 无插值孔]}]}}`，V 是 main 返回 `Result<(), E>` 的 E 和式的变体、恰一载荷 String | 调 `__we_fail` 报告常量行后 `unreachable` |

形状判据是语法模式匹配，其**语义正确性由类型检查背书**（`Ok(())` 已对 `Result<(), E>` 判合；`Err(V(...))` 的载荷已对 V 的 String 载荷判合；变体名 V 已解析到 E 的变体）。变体归属的确认：代码生成需要 E 的声明——从 main 返回注解 `Result<(), E>` 的 `E` 名回查模块符号表（机制：codegen 自建一遍顶层和式名 → 变体表 → 载荷形索引；不依赖 typecheck 内部结构）。

**边界 What 封闭表**（四行，语法 `we: %s are not implemented in this reference build yet`）：

| What | 触发 |
| --- | --- |
| `main bodies beyond a single Ok or Err return statement` | 体空（类型检查已拒，防御位）/ 多语句 / 绑定或赋值 / 尾表达式形返回 / 裸 return / 其他 return 值形 |
| `Err payloads beyond one plain string-literal variant argument` | Err 实参非（单 String 载荷变体 ∘ 纯字符串字面量）复合形：单位变体作 Err 实参、多载荷变体、非字面量载荷、含插值孔字符串（折叠于此行——插值的运行时构造是后续里程碑的同一形式的加宽，不单列） |
| `functions other than main in code generation` | 模块含非 main 的 FnDecl |
| `top-level value bindings in code generation` | 模块含 TopLet |

## D4 IR 形状与符号约定（ADR-0002：文本 IR）

**符号 ABI**（钉死，后续里程碑的稳定面）：用户 main 编译为外部可见 `define i32 @__we_main()`；运行时报告函数在 IR 侧 `declare void @__we_fail(ptr, i64) noreturn`。`__we_` 前缀保留给运行时 ABI。Ok 路径完整 IR（模块名以 demo 为例）：

```llvm
; ModuleID = 'demo'

define i32 @__we_main() {
entry:
  ret i32 0
}
```

Err 路径（`return Err(Failed("boom"))`）：

```llvm
; ModuleID = 'demo'

@.err = private unnamed_addr constant [20 x i8] c"error: Failed: boom\0A"

declare void @__we_fail(ptr, i64) noreturn

define i32 @__we_main() {
entry:
  call void @__we_fail(ptr @.err, i64 20)
  unreachable
}
```

**布局要点**：不发射 target triple 与 datalayout（宿主缺省——ch21 R2 明言 target 词汇未定，机制不预设）；零优化旗标（无 -O；性能零承诺）；`ptr` 不透明指针（LLVM 21 文法）；常量按字节定长、无 NUL 终止（`__we_fail` 收显式长度）。

**字符串双向解码**：We 字面量侧先按 ch1 封闭逃逸集解码为字节（`\n \t \r \0 \\ \" \'` 与 `\u{...}` 按 UTF-8 编码展开——词法已验证，解码不会失败），IR 侧再按实现期真机验证钉死的规则重逃逸：可打印 ASCII（0x20–0x7E，`"` 与 `\` 除外）原样，其余字节（控制字节、`"`、`\`、≥0x80）一律 `\XX` 大写十六进制。实现期勘误：原稿写「`\"` 与 `\\` 双写转义」，真机探针证明钉版 LLVM 文法不接受 c"..." 内的 `\"`（字符串提前终结，报 expected top-level entity），而 `\22`/`\5C`/`\E4` 十六进制形经编译、链接、`__we_fail` 运行时逐字节回放全链验证——规则以十六进制封闭，双写形弃用。报告行是运行时直写的字节序列，程序打印的就是源里写的。

**确定性**：发射是纯函数（AST + 模块名 → 字符串），无计数器、无路径、无时间——单测逐字节断言两形快照。

## D5 Err 报告行与退出码（ch15 R6 的机制读法）

ch15 R6：`Err(e)` 在 stderr 报告失败的消息后非零退出；**消息格式是标准库的自由度**。M4 无标准库，最小格式在此钉死并作为事实上的稳定表面（后续 stdlib 落地时经其自身变更定形——在此之前这是编译器与运行时的约定 ABI）：

- 行 = `error: ` + 变体名 + `: ` + 载荷解码字节 + `\n`（例：`error: Failed: boom`）；
- 退出码：Ok → 0；Err → **1**（ch15 只说非零，取 1 与 we 自身的诊断退出码无冲突——子进程码原样传播，`we run` 不重映射）；
- 报告发生在 `__we_fail`（运行时，D7）——行内容是编译期常量这一事实是切片的直白推论（唯一 Err 形的载荷就是字面量），非编译期求值的引入；载荷可计算时（M5+）表示自然加宽，此处不预支。

## D6 运行时：源、内嵌、按需编译与分配器种子

**runtime/ 目录**布局（实现期一处机制修正：Go 工具链拒绝无 cgo 包目录中的 `.c` 文件，C 源移入无 Go 文件的 `runtime/c/` 子目录——`go build ./...` 不视其为包、`go:embed c/startup.c` 照常内嵌）：

`runtime/c/startup.c`——C 入口与报告函数（完整内容，钉死）：

```c
#include <stdio.h>
#include <stdlib.h>

int __we_main(void);

int main(void) {
    return __we_main();
}

void __we_fail(const char *line, long long len) {
    if (len > 0) {
        fwrite(line, 1, (size_t)len, stderr);
    }
    exit(1);
}
```

`runtime/c/alloc.c`——分配器 ABI 种子（完整内容）：

```c
#include <stdlib.h>

void *__we_alloc(long long n) {
    return malloc((size_t)n);
}

void __we_free(void *p) {
    free(p);
}
```

`runtime/runtime.go`——`package weruntime`，`//go:embed c/startup.c` 与 `//go:embed c/alloc.c` 导出两个源字符串。**内嵌理由（机制）**：编译出的 `we` 二进制自含运行时源，`we build` 不依赖仓库布局或安装位置（conformance 跑器 chdir 进工作目录，相对路径必错）；`go:embed` 是唯一把源与二进制绑定的 Go 机制。

**按需编译 vs 预制对象**：`we build` 每次把内嵌源落盘 `build/rt-*.c`、`clang -c` 编译为 `build/rt-*.o`。ADR-0002 的分发故事是「运行时以预制对象/位码随工具链分发，We 用户无需 C 工具链」——本切片是**开发态参考构建**：钉版 clang 就在工具链内（版本门 D7 保证），每次编译 ~几十毫秒、零缓存复杂度；预制对象随包分发的形态属后续打包故事（非目标披露）。分配器种子 M4 生成码不调用：roadmap M4 行点名 allocator，此 ABI 先钉（`__we_alloc`/`__we_free` 符号与签名），M5 起的堆分配形式以此为目标——死码由链接器自然裁剪，不为「未用」另设机制。

**clang driver 的使用**（ADR-0002「驱动 LLVM 工具链」的 v1 读法）：

```sh
clang -c build/rt-startup.c -o build/rt-startup.o
clang -c build/rt-alloc.c    -o build/rt-alloc.o
clang -Wno-override-module build/<name>.ll build/rt-startup.o build/rt-alloc.o -o build/<name>
# -Wno-override-module：无 triple 的 IR（D4）让 clang 打一条 stderr 警告，
# 而成功三面钉死静默——真机验证后加旗标消警，警告也是输出。
```

clang 按 .ll 扩展名识别文本 IR、内部走 LLVM 后端产对象、再以 driver 身份链接（crt/libc 自动带上——C 运行时模型与 ADR-0002 的 C 启动源一致）。直驱 opt/llc/lld（分级缓存、LTO 调优）延后到需要 driver 未暴露的阶段时再引入——被拒方案见 D11。

**编译/链接失败的报告**（机制）：任一 clang 调用非零退出 → stderr 一行 `we: %s failed: %s`（命令短名 + CombinedOutput 首行），exit 1。非诊断（无码）、非边界——工具链自身的失败类，与 D7 同族。

## D7 钉版门与工具链缺失报告（ADR-0002：LLVM 21.1.8）

`we build` / `we run` 在项目加载前做门：`exec.LookPath("clang")` 失败、或 `clang --version` 输出解析不出主版本 `21.1.8`（`internal/version.LLVMPin`——版本号永不入 spec 文本，ch0 机制中立）→ stderr 一行 `we: LLVM toolchain not available (want clang 21.1.8): <原因>`，exit 1。解析取输出中 `clang version <x.y.z>` 模式的 x.y.z 精确比对（Ubuntu 后缀 `(6ubuntu1)` 不参与）。PATH 查找、无环境变量覆盖——机制从简；可注入性留给单测（门函数收 clang 路径参数，测试用临时脚本伪造版本输出验证拒绝路径）。`we check` / `we clean` 不做门（不触工具链——与 ch22 R5 的 fmt/clean 同理，check 止于文档检查级）。

## D8 初始化与 main 前语义（ch15 R5 披露）

ch15 R5：模块初始化急切、恰一次、先于 main。M4 接受集不含顶层绑定（TopLet → 边界）、不含多模块——**无初始化面**：不发射 init 函数、startup 直调 `__we_main`。当 TopLet 与多模块的代码生成落地时，init 的发射与次序随彼时变更定形；此处零预支。

## D9 E1906 与既有边界的延续披露

- **E1906 不可达**：foreign 块维持 M2 解析边界（bndFFI），M4 管线永不到达链接检查的 foreign 面——E1906 只在 build 触发的条款在本切片是空真（ch21 R11 明言 check 不链接所以 check 下同样不可达）。M12（ffi）兑现。
- **M3 的 12 行 typecheck 边界 What 全维持**（List/Map/Set、panic 家族、Dyn、多模块、spec-gap 三行……）：代码生成在其后，这些形式永不到达 codegen。
- **E1907 维持分发前**：`we build nosuchdir` 照旧 exit 1（M0 两枚黄金不动）。
- **`we test/fmt/vet/doc/lsp` 维持 M0 边界**。

## D10 确定性验证（ch21 R2 的唯一性能承诺）

1. `.ll` 逐字节：单测断言两形完整快照（D4 的两块即黄金）。
2. **二进制字节一致**：单测在两个临时项目目录对同一源各跑一次完整 build（真实 clang 子进程），断言 `build/<name>` 字节相等。论证：IR 无路径；rt-*.c 内容固定；clang 无优化旗标、同输入；链接 build-id 是内容的哈希（非时间戳）。若实现期发现不稳定因素（如嵌入路径），当场披露并处置——测试就是为此写的。
3. conformance 黄金不快照二进制内容（工件存在性由 run-* 黄金的执行结果隐证；`files` 断言只用于 clean 用例的纯文本树）。

## D11 被拒方案

| 方案 | 拒因 |
| --- | --- |
| `@__we_main` 返回 `{i1, ptr, i64}` 结构体、运行时格式化 | 跨 C/IR 的聚合体 ABI（sret/寄存器分类）依赖目标分类规则；`i32 + noreturn 调用`零 ABI 假设、行常量直写 |
| 运行时把报告行拆 variant/payload 两段运行时拼接 | M4 唯一 Err 形的载荷是字面量——拆段是为不存在的可计算性付格式化机制 |
| 直驱 opt/llc/lld 三段管线 | v1 无 driver 未暴露的需求；三进程替代一进程无收益，缓存/LTO 故事到需要时再引入（ADR-0002 的「驱动工具链」不限定具体驱动面） |
| 预制 runtime .o 入库分发 | 提交二进制工件进 git；参考构建期钉版 clang 在场，按需编译更可检视（分发形态 = 后续打包故事） |
| 运行时用 C++/C 混合或自建入口绕 crt | M4 只需 main + fwrite + exit；crt 由 clang driver 自动带，自建入口是无收益的复杂度 |
| 编译期解释执行两形 main（不产二进制） | 快捷假象：`we run` 必须执行真实工件（roadmap M4 行的端到端承诺），非规范面不可伪装 |
| build 缓存（按源哈希跳过重编） | ch21 R2 唯一承诺是同入同出；缓存是性能机制，无承诺支撑，延后 |
| `--release`/优化旗标面 | 同上；零旗标 |
| WE_CLANG 环境变量覆盖工具链路径 | 机制从简：PATH + 版本门已足；覆盖面留给真实需要（交叉编译）时经变更引入 |
| 单文件 build 产出 `build/tool`（取文件名） | 私定规范缺口的行为（ch21：工件由 manifest 命名）；Q3 裁决 = 边界 + follow-up |
| library build 产 `.o` 留作链接输入 | ch21 未批 library 工件形状；发明形状违「规范先行」 |
| Err 行格式带码位/颜色 | ch15 R6 留白的机制面从最小钉起；颜色是 ch21 未批的 `--color` 语义延伸 |

## D12 测试策略与黄金枚举（测试先行协议）

**单测**（internal/codegen）：两形 IR 逐字节快照；逃逸表（`\n` `\"` `\\` `\u{...}`、控制字节 `\XX`）；What 表四行逐行触发；同输入两次发射字节一致。（internal/cli 或独立）确定性双构建测试（D10.2，真实 clang）；版本门拒绝路径（伪造版本脚本）；clean 幂等与 verbose 列出。

**conformance 黄金新增**（22 枚，全部对当前构建先红——`we build/run/clean` 现为 M0 边界行 exit 70；check-deps-nonempty 对 M3 check 亦红）：

| # | 用例 | 断言要点 |
| --- | --- | --- |
| 1 | run-skeleton | `we run .`（Ok 骨架）→ exit 0、stdout/stderr 空 |
| 2 | run-skeleton-verbose | stdout `we: built build/demo`、exit 0 |
| 3 | run-skeleton-json | `--json` 零事件、exit 0 |
| 4 | run-err | Err(Failed("boom")) → stderr `error: Failed: boom`、exit 1 |
| 5 | run-err-verbose | built 行 + error 行、exit 1 |
| 6 | build-skeleton | 静默 0 |
| 7 | build-verbose | 一行 built、exit 0 |
| 8 | build-json | 零事件、exit 0 |
| 9 | build-single-file | 干净单文件 → 边界 70（What = single-file builds...） |
| 10 | build-single-file-diag | 词法坏单文件（未闭字符串）→ E0002 exit 1（证管线先于边界） |
| 11 | build-library | type=library → 边界 70（library artifacts） |
| 12 | build-bnd-body | main 体含绑定 → 边界 70 |
| 13 | build-bnd-fn | 非 main 函数 → 边界 70 |
| 14 | build-bnd-toplet | 顶层 let → 边界 70 |
| 15 | build-bnd-err-payload | Err(Two(1i64, 2i64)) 多载荷 → 边界 70 |
| 16 | check-deps-nonempty | manifest 带 [dependencies] 一键 → check 也边界 70（ch22 R5 对 check 生效） |
| 17 | clean-artifacts | 预置 build/ 桩 → 静默 0 + files 断言树 = 仅源 |
| 18 | clean-verbose | 同上 + 逐行 removed |
| 19 | clean-idempotent | 无 build/ → 静默 0 |
| 20 | build-manifest-missing | `we build .` 于无 we.toml 目录 → E1905 exit 1（证 build 共享 check 的项目加载与报文，零改写） |
| 21 | clean-file-usage | `we clean x.we` → usage exit 2（D1 机制） |
| 22 | run-single-file | `we run` 单文件 → 与 build 同 What 边界 70（证 run 的单文件分派） |

**红证据协议**：T1 黄金落盘对当前构建跑必须红（exit/输出不匹配即红，19 枚全红）；T2 单测对未实现 API 编译失败红。红证据（失败清单）记 tasks 完成记录。**既有黄金零改写**（含 M0 两枚 E1907——分发前触发不受子命令实现影响；M3 101 枚——loadProject 抽取行为零改写）。
