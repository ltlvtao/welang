# tasks — stdlib-conversions（B3a）

> 权威：proposal.md（范围）+ design.md（D1–D7）。每任务实现前待用户点名；红先纪律 = 黄金与测试在实现前取红态（E0816 / build 70 / 测试 FAIL 三形，退出码与输出记档）。

## T1 C 载体七入口

- [x] **str.c/str.h 七入口**：`__we_string_parse_int/parse_uint/parse_float`（ptr, i64, ptr out——out 三字组，Some 写 tag+载荷、None 写 tag+零载荷）+ `__we_string_rune_code/rune_from`（i64 恒等）+ `__we_string_float_bits/float_from_bits`（union 位型重解释）；NUL/长度纪律（拷贝 len+1 加 NUL、endptr 全串核对）、ERANGE→None 映射按 design D3。
  来源：proposal 目标 3；design D3
  验证：`runtime/str_test.go` 新测试（先红后红态记档）；`go test ./runtime/ -run TestStr -count=1` 绿；既有 runtime 全测试不回归
- [x] **边界覆盖**：空串/中途 NUL（`"12\0x"`）/尾随垃圾 → None；`9223372036854775807`/`18446744073709551615` 正上界 → 有值、越一位 → None；`parseFloat` 上溢（`1.0e999`）/下溢（`1.0e-999`）→ None；位型回环含 denormal 代表值。
  来源：design D3、D7
  验证：同上测试内断言段逐形；`1e999`（无点）不在测试正面集——核心形不含无点指数形（design D2）

### T1 落地记（2026-09-17）

- **载体**（`runtime/c/str.c` 七入口 + `str.h` 声明与契约注释，include 仅新增 `errno.h`）：`parse_none`/`parse_some` 静态助手（coll.c 姿态——tag 1/0、载荷字、第三字恒零，三字组不带脏栈字节）；接受文法前置检查 `all_digits`（整数核：非空纯数字串——符号/空白/中途 NUL 在任何库调用前即拒）与 `float_core`（浮点核状态机：`数字 '.' 数字 (eE [+-]? 数字)?` 全串——`1e5`/`.5`/`5.`/`1.5e`/`1.5e+` 皆形外）；`zcopy` 拷贝 len+1 加 NUL（str_alloc 姿态、调用方 free——scratch 非 String 血统）；三 parse 各自直写 strtoll/strtoull/strtod（errno 清零、endptr 全串核对）；四桥（rune_code/rune_from 恒等、float_bits/float_from_bits union 位型重解释）零分配零 gc 交互。
- **红态（2026-09-16 记档）**：str.h 声明未落时 `go test ./runtime/ -run TestStrParseHarness -count=1` → 20 枚 clang 错误 `call to undeclared function '__we_string_*'`——先红后绿纪律兑现。
- **实现期真缺陷一枚（harness 首轮暴露）**：初版 `PARSE_FULL` 宏的失败路径只 `free+return`、**不写 out 三字组**——None 路径泄漏上一轮 Some 残值（FAIL 31/33/58/88/90 皆为前轮 tag=1 残留）。修法：宏拆掉、三入口直写，纪律句入注释——**每条路径（含失败）必写 out**。
- **ERANGE 判据精化（design 实现期补记一）**：glibc 对 subnormal 也置 ERANGE，`errno == 0` 一刀切把 `2.5e-320`（FAIL 83）错杀为 None。判据改以答案形状为准：ERANGE 且（inf——位型指数字段全一，`bits_is_finite` 读位字不引 math.h——或答案为零）才 None；subnormal 存活为 Some。
- **验证**：`go test ./runtime/ -run TestStrParseHarness -count=1` 绿（60 断言：两正上界精确 Some、越一位 None、denormal `2.5e-320` Some、`1.0e±999` 双向 None、`-0.0` 位型 `0x8000000000000000`、`5e-324` 位型回环、rune 域外值 `-1`/`0x110000`/`0xd800` 恒等不变）；`go test ./runtime/ -run TestStr -count=1` 绿；`go test ./runtime/ -count=1` 全包不回归；`go build ./...` + conformance 全量轮 `-count=1`（165s，exit 0）——str.c 进每条 build 链，919 枚既有黄金零改写零漂移。
- **阶梯**：`gofmt -l`（0 文件）、`go vet ./runtime/`、`git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用。

## T2 面声明与装载零漂移

- [x] **string.we 七虚构体**：签名照 proposal 目标 1 表；panic 体（Never 满足任意返回位，mapOf 先例）；无 effect 段（join/repeat 纯性先例）。
  来源：proposal 目标 1/2；design D1、D2
  验证：探针项目 `import std.string` 调 `string.parseInt("12")` + match 两臂 → `we check` exit 0；`string.parseInt(12)` → E0501；`string.runeFrom("a")` → E0501（检查器零码改——typecheck 包零 diff）
- [x] **零漂移三缝**：既有黄金零改写 + hello IR 逐字节 + mock 黄金绿。
  来源：design D4（零漂移论证）
  验证：`go test ./internal/conformance/ -count=1` 919 枚零改写全绿；hello IR 对 HEAD worktree `cmp` 逐字节零差；既有 mock 黄金绿

### T2 落地记（2026-09-17）

- **面声明**（`stdlib/src/string.we` +7 虚构体 + 头注释段）：签名照 proposal 目标 1 表；panic 体沿 mapOf 先例形；无 effect 段（join/repeat 纯性先例）。
- **红态（记档）**：声明未落时探针 `we check` → `error[E1304]: unresolved name — the module "std.string" declares no "parseInt"` exit 1。
- **探针**：正面 `string.parseInt("12")` + match 两臂 → check exit 0；七面合探针（parseUInt/parseFloat 两 Option + 四桥，`+1` 算术钉 Int64 返回域）→ exit 0；负面 `string.parseInt(12)` / `string.runeFrom("a")` → E0501 逐字（实参 Int64/String 对参数位 String/Int64）。检查器**生产码零 diff**（typecheck 包改动仅 b2a_test.go 一枚重锚，见下）。
- **实现期发现与机制修正（design 补记二）**：D1「panic 体，mapOf 先例」被实现推翻——mapOf 先例的前提是 collections 不走程序面（发射器从不走其体）；std.string 是唯一走程序面的 std 模块，虚构体 define 会被走，而 (a) 非 Never 返回 + panic 尾今日不可发射（用户侧同形探针 `fn f() -> Int64 { panic("boom") }` → build 70；虚构体落地后 join-only 程序同炸），(b) 即使可发射，死 define 也漂移每个 import 者的 IR。修正 = 键控名集 `stringKeyedFns`（七名表，declare 表的键半）随 T2 落：define 收集臂对 `m.Key == "std.string"` 且键控名者跳过 fnTable/fns——join/repeat 照走程序面，programModules 零触碰。发射臂与分派重排仍归 T3。修正后七面 check 0、虚构体调用 build 停在调用点 instCallee 查无（70，「main bodies…」词）——T3 黄金红态由调用点诚实给出。
- **单测重锚一枚**：typecheck `TestStdModuleLoadsParsedSources` 的 std.string 行 `[join repeat]` → 九名（B2a 钉的是装载面条目清单，面真拓宽故重锚；测试文件注释同步）。
- **零漂移三缝**：① conformance 全量 `-count=1` 164s exit 0，919 枚零改写（testdata `git status` 零改动）；② IR 对拍——HEAD worktree（`b982d0f`）与工作树双二进制各跑 string 密集项目（join+repeat）与经典 hello（io-only），`build/demo.ll` `cmp` **逐字节零差**（两形皆过）；③ mock 黄金随套件全绿。
- **阶梯**：`go build` / `go vet ./internal/codegen/` / `gofmt -l`（0 文件）/ `go test -count=1` typecheck+codegen+cli+conformance 全绿；`git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用。

## T3 键控拦截与混合分派

- [x] **declare 表 + 七键控发射臂**：declare 七行；Option 三枚 out 三字组（载荷字 Int64/UInt64 原位、Float64 位型字）、直返四枚寄存器形。
  来源：proposal 目标 4；design D3、D4
  验证：codegen 单测（七臂 IR 形状、out 布局、declare 存在性）先红后绿；`go test ./internal/codegen/ -count=1` 绿
- [x] **混合分派**：string 别名限定调用先查键控集、未中落程序面；join/repeat 路径逐字节不变。
  来源：design D4
  验证：单测钉 join/repeat 既有调用 IR 快照不变；hello IR 仍零差；fs/process/collections 黄金不回归
- [x] **正面黄金**：`run-stdlib-parse-int`、`run-stdlib-parse-float`、`run-stdlib-rune-code`、`run-stdlib-rune-roundtrip`、`run-stdlib-float-bits`（期望字节手写；Float64 渲染字节以真机先跑对锚；红态 = T2 后的 build 70 记档）。
  来源：design D7
  验证：新黄金 `-count=1` 由红转绿；conformance 总数 as-built 记账

### T3 落地记（2026-09-17）

- **三处码**（`internal/codegen/codegen.go`）：① declare 表七行（`__we_string_parse_int/parse_uint/parse_float` 的 `(ptr, i64, ptr)`、四桥的 `i64`/`double` 寄存器形）——declUsed 机器照旧只在 `use` 后渲染（join-only 程序的 IR grep `__we_string_` 零命中，键控未调用即零痕迹）；② emitCall 限定调用臂的程序面腿前置键控门——`resolveQual(recv.Name) == "std.string" && stringKeyedFns[fn.Name]` 即短路进键控臂，未中者原样落程序面（其余 std 模块零触碰）；③ `emitStringKeyedCall` + `stringParseEntries` 三析取表——三 parse 走 fs 槽形（`@slot.string.<name>` 默认指 C 入口、调用点 `load` 后过 `%fp`）+ coll-get 的 out 三字组（`[3 x i64]` alloca、gep 0/8/16、三 i64 槽存储、`sumSlot{variants: [None Some], shapes: optionShapes(pay), key: "Option"}`）；四桥走寄存器形（runeCode/runeFrom `call i64 %fp(i64 %)`，floatBits `call i64 %fp(double %)`，floatFromBits `call double %fp(i64 %)`），答案的 `typeName` 携带绑定渲染域（Int64/UInt64 i64 词、Rune 过 `__we_str_of_rune`、Float64 `isFloat` 过 `__we_str_of_f64`）。
- **Float64 载荷字免费**：`stringParseEntries` 的 `pay` 面 `{abiDouble, "Float64"}` 直接进 `optionShapes`——Some 臂的位型回读（`bitcast i64 … to double`）与浮点域渲染由 `bindArmWord` 的 abiDouble 臂既有管道承担（T9 载荷面管道），键控臂零自绘读回码。
- **红态（T2 后记档）**：六枚单测 FAIL `boundary "main bodies beyond the M9b statement set…"`；五枚黄金 `we run` exit 70 同词——皆调用点 instCallee 查无的诚实停（补记二的预告兑现：T3 黄金红态由调用点给出）。
- **绿期一枚钉校正（非码改）**：`TestStringFloatBridgesCrossDomains` 初钉 `call i64 %(double %` 假设实参是寄存器——实发 `call i64 %v0(double 0x4004000000000000)`（字面量 2.5 折叠为十六进制位型立即数）；改钉 `(double 0x4004000000000000)` 反而更强（连折叠位型一并锚定）。
- **机器对锚两枚**：parse-float stdout `got 1.5
trim 1.5
exp 1.5e+02
d-none
`（`"1.500"` 尾零经渲染归一、`"1.5e2"` 渲染 `1.5e+02`、`"1.0e999"` None）；float-bits stdout `b 4607182418800017408
n 4612811918334230528
g 2.5
rt-ok
`（1.0 = 0x3FF0000000000000、2.5 = 0x4004000000000000 的十进制读数 + 回环）。
- **验证**：六单测绿、`go test ./internal/codegen/ -count=1` 全包绿（474 → 480）、typecheck 全包绿；五黄金绿；conformance 全量 `-count=1` 161s exit 0——**919 → 924（as-built）**、既有黄金零改写（testdata 除五新枚外零 diff）；fs/process/collections 黄金随套件全绿。
- **零漂移三缝**：① join+repeat 密集项目与经典 hello 两形，HEAD worktree（`a4270d1`）与工作树双二进制各跑 `build/demo.ll` `cmp` **逐字节零差**；② mock 黄金随套件全绿；③ join-only IR 键控痕迹零命中（declare 与槽俱不渲染）。
- **阶梯**：`go build ./...` / `go vet ./internal/codegen/` / `gofmt -l`（0 文件）/ `git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用。

## T4 负面/溢出/边界黄金 + 突变电池

- [x] **负面与边界黄金**：`run-stdlib-parse-empty`、`run-stdlib-parse-garbage`（`"12a"`、`" 12"`）、`run-stdlib-parse-int-overflow`、`run-stdlib-parse-uint-overflow`、`run-stdlib-parse-int-max`、`run-stdlib-parse-uint-max`、`run-stdlib-parse-float-range`、`run-stdlib-float-bits-roundtrip`。
  来源：design D7
  验证：黄金 `-count=1` 全绿；期望字节手写（`WE_UPDATE_GOLDEN` 禁用）
- [x] **突变电池判决表**：M-a endptr 全串核对删去、M-b ERANGE 映射删去、M-c 键控集泄漏到 join、M-d runeCode 恒等改 +1——逐枚单点突变、锚点先断言 count==1、单测层/黄金层各记捕获性；黄金层结构性静默者以 runtime 层探针补证（B2b M-b/M-c 先例）。
  来源：design D7
  验证：判决表入落地记；突变逐枚还原后全套复绿

### T4 落地记（2026-09-17）

- **八枚黄金**（正面字节全部真机先跑对锚后手写）：`parse-empty`（三 parse 空串各 None）；`parse-garbage`（`"12a"` 尾随垃圾、`" 12"` 前导空白、`"abc"`、`"1.5e"` 指数形残——float_core 状态机四拒）；`parse-int-overflow`（max+1 `9223372036854775808` 与 20 位 `99999999999999999999` 双 None）；`parse-uint-overflow`（2^64 `18446744073709551616` None + **域分裂钉**：u64 max 串 `18446744073709551615` 对 parseInt 亦 None——同一串两域两答）；`parse-int-max`（`9223372036854775807` 双 parse 皆 Some，i64/u64 渲染各自十进制）；`parse-uint-max`（`18446744073709551615` Some，of_u64 全无符号渲染）；`parse-float-range`（`1.0e999` 上溢 None、`1.0e-999` 下溢 None、`2.5e-320` subnormal **存活** Some 渲染 `sub 2.5e-320`——补记一的黄金层钉、`1e999` 无点形 None——文法门与范围门分立钉，四行三门）；`float-bits-roundtrip`（2.5/0.25 双回环 + parseFloat 入的 subnormal 经 bits 往返 `==` 精确成立——位型保真的最强钉）。
- **红态纪律的诚实注**：本批黄金钉 T3 已落的面，绿到是设计使然（T3 落地记已把红态记在案）；T4 的红先义务由突变电池承担——每枚突变的双层红即红态。
- **突变电池判决表**（逐枚单点、锚点 `git diff --numstat` 1/1 或 1/0 先行断言、逐枚还原复绿）：

| 突变 | 点 | runtime harness | codegen 单测 | 黄金层 | 判决 |
| --- | --- | --- | --- | --- | --- |
| M-a endptr 删去（parse_int） | str.c ok 行去 `end == z + n` 合取 | 绿 | n/a（IR 不变） | garbage/empty/overflow 三金绿 | **全层结构性静默**——前置 `all_digits` 吞并 endptr 域：过门者必纯数字串、strtoll 必全消耗；endptr 合取是**防御纵深非活门**（文法将来放宽时才活）。设计 D7「捕获性待测」有答：不可捕获，发现归位 str.c 头注释（本次提交内） |
| M-b-int ERANGE 删去（parse_int） | 同行去 `errno == 0` 合取 | **FAIL 31/33**（溢出 None 断言） | 绿（IR 不变，定义性） | **int-overflow 差**：`edge-bad 9223372036854775807`（strtoll 钳位值泄出） | 双层捕获 |
| M-b-float ERANGE 守卫删去（parse_float） | ok 行去整 ERANGE 括号 | **FAIL 88/90** | 绿 | **float-range 差**：`up-bad inf`/`down-bad 0`，`sub`/`nodot` 两行不动（恰证三门分立） | 双层捕获 |
| M-c 键控集泄漏 join | stringKeyedFns +`"join": true` | n/a | **三枚 join 单测停边界**（拦截无臂 arity 2 → bnd；define 亦被收集臂跳过） | **run-string-join exit 70** | 双层捕获 |
| M-d runeCode +1 | str.c `return c + 1` | **FAIL 124–126/132**（恒等断言族） | 绿（IR 不变，定义性） | **rune-code 差**：`A 66`/`zh 20014`，`r B` 不动（runeFrom 未染恰证桥分立） | 双层捕获 |

  - M-b 按两 ok 行形分两点施突（int/float）；parse_uint 的 ok 行与 parse_int 逐字同形，对称论证不另施（B2b 判决表先例）。
  - codegen 单测层对 runtime 侧突变（M-a/M-b/M-d）的定义性静默不记缺口——该层钉 IR 形状不钉运行语义，黄金层与 runtime harness 是语义面的双层守门。
- **验证**：八黄金 harness 绿（`-count=1`）；突变逐枚还原后 `go build` + runtime 全包（13.2s）+ codegen 全包 + 八黄金复绿；conformance 全量 `-count=1` exit 0——**924 → 932（as-built）**、既有黄金零改写。
- **阶梯**：`gofmt -l`（0 文件）/ `git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用。

## T5 mock 面

- [x] **七枚入 mock 槽表**：Option 三枚 outTrio 双 define（B2a 先例）、直返四枚期望值直答。
  来源：proposal 目标 5；design D6
  验证：`go test ./internal/codegen/ -count=1` 绿；既有 34 mock 黄金不回归
- [x] **mock 黄金**：`test-mock-string-parse`（拦截命中 + 非拦截路径照真入口）。
  来源：design D6
  验证：黄金 `-count=1` 绿

### T5 落地记（2026-09-17）

- **四处码**（`internal/codegen/codegen.go` 三处 + `internal/cli/test.go` 一处）：① `stringBridgeEntries` 四桥单一权威表（sym/argF/retF/tn），`emitStringKeyedCall` 的桥臂 switch 改读表——调用点与 mock 槽臂同表，槽名永不失配（行为零变，纯收拢）；② `mockTarget` 的 resolveQual 臂内、fnTable 查找**之前**的键控支线——`k == "std.string" && stringKeyedFns[md.Target]` 命中即以 `fnAbiOf(md.Params/md.Ret)` 分类、槽 `"string."+name`（调用点键控槽的同名）、恢复 `"@"+C 入口`（槽默认）、outTrio = 三 parse 在场（四桥 false）——位置在支线而非 std 臂的原因：std.string 在 modKeys（程序面成员）故 resolveQual 已答，fnTable 必 miss（T2 定义收集跳过），不像 fs 走 resolveStd 臂；③ test.go 的 std 过滤补 std.string 例外（对齐 build 面 `programModules` 的 D5 例外）——测试塔的 modKeys 由此含 std.string，测试体内的键控分派门与 mockTarget 的 resolveQual 才活；④ `classType` 的 abiI64 名单加 Rune。
- **实现期发现（④的来历，design 补记三）**：Rune 此前从未到达签名面——检查器接受 `fn code(c: Rune) -> Int64`（探针实证），发射面 `classType` 拒绝 → exit 70 潜伏缺口（先于 B3a 存在，无黄金锚定过）。B3a 的 runeCode 参数/runeFrom 返回首次把 Rune 拼进 mock 复述签名，`fnAbiOf` 必须分类。臂开后用户 fn 的 Rune 参数/返回随开：探针 `fn code(c: Rune) -> Int64 { return 65 }` + `let v = code('A')` 从 70 到打印 `v 65`——**行为拓宽如实披露**，mock 单测的 runeCode（参数侧）/runeFrom（返回侧）两 pin 即其锚。
- **次生面（③的免费后果，如实披露）**：测试块内的 `string.join`/`string.repeat` 此前 exit 70（探针记档：std.string 被测试面 std 过滤滤掉，resolveQual 空），现照程序面 define 走——`we test .` 打印 `a-b`；单文件 `we test file.we` 的 string 面仍停（single-file builds 是已登记 spec gap，先例不重开）。
- **红态（记档）**：两枚单测 FAIL boundary `main bodies beyond the M9b statement set…`（单测夹具自组 ProgModule 含 std.string，红在 mockTarget 的 fnTable-miss bnd）；黄金项目 `we test .` exit 70 同词（cli std 过滤——std.string 不上测试程序面，两条路皆断）。
- **单测两枚**：`TestStringParseMockRidesTheOutTrio`（槽默认 C 入口/install/restore/body define 保 Option 和 ABI/void wrapper 取入口调用形/转发/`extractvalue …, 1` ↔ `getelementptr …, i64 8` 载荷字对位/调用点 `load` 键控槽）；`TestStringBridgeMocksAnswerInRegisters`（三桥——runeCode 的 `(i64 %c)` 直 define、floatFromBits 的 `double` 答案、runeFrom 的 Rune 返回侧；`.mock.N.body` 零命中钉四桥无 wrapper）。
- **黄金 `test-mock-string-parse`**：两块——拦截命中（`"junk"` 被 mock 答成 `Some(12)`，真入口必 None——拦截的最强反证）+ 恢复后真入口（`"34"` → 34 过 C 入口）；stdout 照 test-fs-mock 驱动报格式真机对锚。
- **验证**：`go test ./internal/codegen/ ./internal/typecheck/ ./internal/cli/ -count=1` 全绿；conformance 全量 `-count=1` 170s exit 0——**932 → 933（as-built）**、既有黄金零改写（testdata 除一新枚外零 diff）；mock 承载黄金（fs/process/collections/std.string 各族 + check/vet 面）随套件全绿。
- **零漂移三缝**：hello（io-only）与 join+repeat 密集两 build 形 + fs-mock 测试塔形，HEAD（`7b75731`）worktree 与工作树双二进制各跑 `build/demo.ll`/`build/demo.test.ll` `cmp` **逐字节零差**。
- **阶梯**：`go build ./...` / `go vet`（codegen+cli）/ `gofmt -l`（0 文件）/ `git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用。

## T6 docs 与 roadmap

- [x] **benchmarks 运行面句（双语）**：`docs/benchmarks.md` + `.zh.md` 补「stdlib-conversions 线」句——七面可跑、失败面 Option、全函数四枚；无新停面（如实）。
  来源：proposal 目标 7
  验证：`python3 openspec/tools/docs_sync.py`（33 对）；`git diff --check`
- [x] **roadmap 策略段 B3 裁决句（双语）**：`docs/roadmap/0000-reference-implementation.md` + `.zh.md` 策略节补 2026-09-16 五裁决（六段切片、std.string+Option、wec 顶层、lsp/deps 非目标、逐字节对账）。
  来源：proposal 目标 8；轨道裁决表
  验证：同上 docs_sync；策略句与 proposal 裁决表逐字对照无出入

### T6 落地记（2026-09-17）

- **benchmarks 运行面句**（双语，插 stdlib-collections 句之后、停面清单之前）：stdlib-conversions 线句——三 parse 收十进制核心（整数非空纯数字串、浮点 `digits.digits` 可选带符号指数）以 Option 答（畸形/内嵌 NUL/越界 None、`2.5e-320` subnormal 存活、`1.0e999` 不存活）、`Some(v)` 臂按载荷自身面读回（Int64/UInt64 字、Float64 位型字读回 double——`sub 2.5e-320` 黄金锚）、四桥全函数（rune 对 i64 恒等、float 对位型重解释、双回环成立）、接受文法不带符号/前缀/下划线/空白（归一是调用方组合层）、七面三上下文可跑（main 体/内耗答案的纯助手体/测试块）+ mock 面骑槽；**T5 两枚拓宽随句披露**（string 模块乘测试塔程序面——测试块内 join 可跑；Rune 过 fn 签名）；**「本线未打开任何停面」如实句**。
- **句子级证据核验（每声明一面一探针/黄金）**：subnormal 存活与上溢 None = parse-float-range 黄金；Some 臂读回 = parse-float 黄金 `got 1.5`；双回环 = rune-roundtrip + float-bits-roundtrip 黄金；纯助手体 = 探针 `fn code(s: String) -> Int64`（内 match parseInt）打印 `r 66`；测试块 = test-mock-string-parse 黄金；join 于测试块 = 探针 `a-b`；Rune 过签名 = 探针 `v 65`。
- **枚如实注**：初稿曾写「七面于任意上下文可跑」——纯助手探针第一形（`fn pick(s: String) -> Option<Int64> { return string.parseInt(s) }`）停 fn 尾返 sum 调用面，**该面为既有停**（同形 `return xs.get(0)` 自 B2b 即停，与键控无关、非本线打开），改写为「内耗答案的纯助手体」精确形。
- **roadmap 策略段**（双语，策略节第五枚 bullet，逐字对照 proposal 裁决表五格——英文侧脚本核验 17+5+6+7+6 关键词全在场、中文侧逐字裁决句与五格实质全在场）：六段切片（B3a..B3f + B3d/e 现场拆分授权）、挂 std.string 函数面失败以 Option（非成员面非 panic）、wec 顶层（we.toml type=executable + src/ 多模块、Go 链不动、Go 侧测试编译并驱）、lsp/deps 非目标（lsp 诚实 exit 70、deps 登记 follow-up、[deps] 表保留有依赖诚实停）、B3e 逐字节对账（We .ll 与 Go .ll 逐字节同、非验收必需、「919 绿」从赌注变推论——数字标注裁决时点语料数）。中文侧载用户逐字裁决「1. 六段。2. 挂string，以Option答。3. 同意。4. 非目标。5. 逐字节对账」。
- **验证**：`python3 openspec/tools/docs_sync.py` → `OK: 33 document pair(s) aligned`；`git diff --check` 净；零码改零黄金改（四文件皆 .md）。

## T7 审查与归档

- [x] **welang-code-review 七条**：①规范符合性 ②验证诚实性 ③测试先行 ④诊断协议 ⑤单一权威 ⑥红线 ⑦最小可信验证——逐条过、证据入 proposal「审查记录」。
  来源：AGENTS.md 变更工作流
  验证：七条结论表 + 发现处置记录落 proposal.md
- [x] **follow-up 登记**：`\u` 逃逸/rune 字面量码点域无校验（design D5 披露——登记为待裁 spec 缺口，非本变更改动）、deps 非目标（轨道裁决 4——We 宿主编译器不含依赖解析，`[deps]` 表保留、有依赖时诚实停）。
  来源：design D5；轨道裁决 4；B2b #26 登记先例
  验证：roadmap follow-up 清单双语言行落册（编号顺延）；条目措辞与 D5 披露、裁决 4 逐字对照无出入
- [x] **归档**：mv 至 `archive/2026-09-17-stdlib-conversions/`、change.yaml archived、roadmap B3 行拆 B3a done + B3b–B3f pending（双语）、follow-up 登记、记忆回写、归档后 validate 期望态 + 全阶梯、归档提交（不推送）。
  来源：AGENTS.md 变更工作流；B1/B2 归档先例
  验证：`python3 openspec/tools/validate.py --all --strict`（归档后「OK: no changes found (nothing to validate); registry clean」期望态）；`go build ./...` + `go test ./... -count=1` 16 包 0 FAIL；conformance 总数 as-built 记账

### T7 落地记（2026-09-17）

- **审查七条**（span `ce66102..8bd3415` 六提交，结论表 + 发现处置 F1/F2 落 proposal「完成审查」节）：①规范符合性——`docs/spec/`、`diagnostics.toml`、`internal/diag/` span 零 diff（「无规范增量」标记兑现），七面锚定族外 stdlib 表面、ch16 纯性与 ch20 mock 面对黄金逐核；②验证诚实性——黄金账 919 → +5（T3）→ +8（T4）→ +1（T5）= 933 as-built 逐提交、codegen 单测 474 → 482、runtime 46、typecheck 76；③测试先行——六落地记红态全在册（T1 20 枚 clang undeclared、T2 E1304、T3 六单测+五黄金 70、T4 突变电池双层红、T5 两单测+黄金 70、T6 句子级证据核验）；④诊断协议——零新码零字段改、span 的四枚 E 码 diff 命中皆 openspec 文档散文；⑤单一权威——改动码文件 CJK 注释 0、长期事实归位 design 补记一/二/三 + str.c 注释 + 双语文档；⑥红线——22 改动文件皆在申报层 [compiler, stdlib, docs]、`refr/` 0、署名 trailer 0；⑦最小可信验证——全阶梯绿。**审查期零码改**（F1 = T5 两枚先决缺口的归账条目、F2 措辞级订正已随 T6 落，皆非新改）。
- **补记三补写**：tasks.md T5 落地记引用的「design 补记三」在 T5 提交时漏写（design.md 止于补记二）——归档前按单一权威纪律补写（测试塔程序面例外探针证据 + 单文件 spec gap #6 维持 + Rune 入名单的 `v 65` 拓宽披露 + as-built 机制落点），随归档提交入册。
- **follow-up #27/#28**（双语，编号顺延 #26）：#27 `\u` 逃逸/rune 字面量码点域无校验（D5 披露逐字对齐——`\u{1..6 hex}` 累加无 0x10FFFF 上限无代理区排除、runeFrom 忠实恒等、ch1/类型章修订应裁）；#28 We 宿主编译器依赖解析（裁决 4 对齐——`[deps]` 表解析保留、有依赖诚实停、Go 宿主 M13 面不动）。
- **B3 行拆分**（双语）：B3a done 2026-09-17 as-built（七枚原语、919→933、474→482、mock 面、两枚先决缺口随线闭合、运行面披露、#27/#28）+ B3b（wec 项目+词法器）/B3c（parser）/B3d（checker）/B3e（codegen+逐字节对账杠）/B3f（cli/test/fmt/vet/doc + 三段式闭环 + conformance 第二面）五 pending 行——三段闭环与 conformance 第二面的验收描述随 B3f 行保真。
- **归档**：`git mv` 至 `archive/2026-09-17-stdlib-conversions/`、change.yaml `status: archived`；归档后 validate 期望态与全阶梯复验见下。
- **验证**：`python3 openspec/tools/validate.py --all --strict` → 「OK: no changes found (nothing to validate); registry clean」期望态；`go build ./...` + `go test ./... -count=1` 16 包 0 FAIL；conformance 全量 `-count=1` **933（as-built）**全绿零改写；`docs_sync.py` → OK: 33 document pair(s) aligned；`gofmt -l` 0 文件；`git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用、`git stash` 未用。归档提交不推送（push 待用户明示）。
