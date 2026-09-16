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

- [ ] **declare 表 + 七键控发射臂**：declare 七行；Option 三枚 out 三字组（载荷字 Int64/UInt64 原位、Float64 位型字）、直返四枚寄存器形。
  来源：proposal 目标 4；design D3、D4
  验证：codegen 单测（七臂 IR 形状、out 布局、declare 存在性）先红后绿；`go test ./internal/codegen/ -count=1` 绿
- [ ] **混合分派**：string 别名限定调用先查键控集、未中落程序面；join/repeat 路径逐字节不变。
  来源：design D4
  验证：单测钉 join/repeat 既有调用 IR 快照不变；hello IR 仍零差；fs/process/collections 黄金不回归
- [ ] **正面黄金**：`run-stdlib-parse-int`、`run-stdlib-parse-float`、`run-stdlib-rune-code`、`run-stdlib-rune-roundtrip`、`run-stdlib-float-bits`（期望字节手写；Float64 渲染字节以真机先跑对锚；红态 = T2 后的 build 70 记档）。
  来源：design D7
  验证：新黄金 `-count=1` 由红转绿；conformance 总数 as-built 记账

## T4 负面/溢出/边界黄金 + 突变电池

- [ ] **负面与边界黄金**：`run-stdlib-parse-empty`、`run-stdlib-parse-garbage`（`"12a"`、`" 12"`）、`run-stdlib-parse-int-overflow`、`run-stdlib-parse-uint-overflow`、`run-stdlib-parse-int-max`、`run-stdlib-parse-uint-max`、`run-stdlib-parse-float-range`、`run-stdlib-float-bits-roundtrip`。
  来源：design D7
  验证：黄金 `-count=1` 全绿；期望字节手写（`WE_UPDATE_GOLDEN` 禁用）
- [ ] **突变电池判决表**：M-a endptr 全串核对删去、M-b ERANGE 映射删去、M-c 键控集泄漏到 join、M-d runeCode 恒等改 +1——逐枚单点突变、锚点先断言 count==1、单测层/黄金层各记捕获性；黄金层结构性静默者以 runtime 层探针补证（B2b M-b/M-c 先例）。
  来源：design D7
  验证：判决表入落地记；突变逐枚还原后全套复绿

## T5 mock 面

- [ ] **七枚入 mock 槽表**：Option 三枚 outTrio 双 define（B2a 先例）、直返四枚期望值直答。
  来源：proposal 目标 5；design D6
  验证：`go test ./internal/codegen/ -count=1` 绿；既有 34 mock 黄金不回归
- [ ] **mock 黄金**：`test-mock-string-parse`（拦截命中 + 非拦截路径照真入口）。
  来源：design D6
  验证：黄金 `-count=1` 绿

## T6 docs 与 roadmap

- [ ] **benchmarks 运行面句（双语）**：`docs/benchmarks.md` + `.zh.md` 补「stdlib-conversions 线」句——七面可跑、失败面 Option、全函数四枚；无新停面（如实）。
  来源：proposal 目标 7
  验证：`python3 openspec/tools/docs_sync.py`（33 对）；`git diff --check`
- [ ] **roadmap 策略段 B3 裁决句（双语）**：`docs/roadmap/0000-reference-implementation.md` + `.zh.md` 策略节补 2026-09-16 五裁决（六段切片、std.string+Option、wec 顶层、lsp/deps 非目标、逐字节对账）。
  来源：proposal 目标 8；轨道裁决表
  验证：同上 docs_sync；策略句与 proposal 裁决表逐字对照无出入

## T7 审查与归档

- [ ] **welang-code-review 七条**：①规范符合性 ②验证诚实性 ③测试先行 ④诊断协议 ⑤单一权威 ⑥红线 ⑦最小可信验证——逐条过、证据入 proposal「审查记录」。
  来源：AGENTS.md 变更工作流
  验证：七条结论表 + 发现处置记录落 proposal.md
- [ ] **follow-up 登记**：`\u` 逃逸/rune 字面量码点域无校验（design D5 披露——登记为待裁 spec 缺口，非本变更改动）、deps 非目标（轨道裁决 4——We 宿主编译器不含依赖解析，`[deps]` 表保留、有依赖时诚实停）。
  来源：design D5；轨道裁决 4；B2b #26 登记先例
  验证：roadmap follow-up 清单双语言行落册（编号顺延）；条目措辞与 D5 披露、裁决 4 逐字对照无出入
- [ ] **归档**：mv 至 `archive/2026-XX-XX-stdlib-conversions/`、change.yaml archived、roadmap B3 行拆 B3a done + B3b–B3f pending（双语）、follow-up 登记、记忆回写、归档后 validate 期望态 + 全阶梯、归档提交（不推送）。
  来源：AGENTS.md 变更工作流；B1/B2 归档先例
  验证：`python3 openspec/tools/validate.py --all --strict`（归档后「OK: no changes found (nothing to validate); registry clean」期望态）；`go build ./...` + `go test ./... -count=1` 16 包 0 FAIL；conformance 总数 as-built 记账
