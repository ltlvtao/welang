# design — stdlib-conversions（B3a）

## D1 面形与语义（裁决 2 的兑现）

七枚皆挂 `std.string`、皆纯（无 effect 段——join/repeat 先例，ch16 显式纯性）、虚构体（panic 体）+ 键控拦截：

| 面 | 签名 | 失败面 |
| --- | --- | --- |
| `parseInt` | `(String) -> Option<Int64>` | 畸形/溢出 → `None` |
| `parseUInt` | `(String) -> Option<UInt64>` | 畸形/溢出 → `None` |
| `parseFloat` | `(String) -> Option<Float64>` | 畸形/范围错 → `None` |
| `runeCode` | `(Rune) -> Int64` | 全函数（恒等过桥） |
| `runeFrom` | `(Int64) -> Rune` | 全函数（恒等过桥） |
| `floatBits` | `(Float64) -> Int64` | 全函数（位型重解释） |
| `floatFromBits` | `(Int64) -> Float64` | 全函数（位型重解释） |

失败/全函数的分界即裁决 2 的「以 Option 答」的边界：**能失败的面答 Option，数学上是双射的面不答**——给恒等/位型四枚包 Option 是把确定性藏进 Maybe 的谎形，拒。

`runeCode`/`runeFrom` 在 IR 层是 i64 恒等（Rune 本就以 i64 立即数过寄存器，`codegen.go:2755` 注释自述）；`floatBits`/`floatFromBits` 是 double↔i64 位型重解释。四枚经 C 入口过桥（`__we_string_rune_code(i64)` 直接返回实参；float 两枚 union/memcpy）——**不做内联位型发射**（新发射形状的复杂度换不来一枚 call 的开销，统一键控形胜出）。

## D2 接受文法：十进制核心，组合归调用方

**`parseInt`/`parseUInt`**：非空、纯 `[0-9]`、全串；无符号（ch1 字面量本身无符号，负号是一元算子，wec 折叠 `-5` 时自行取反）、无 base 前缀、无下划线、无空白。值域检查：`[−2^63, 2^63−1]` / `[0, 2^64−1]`，越界 `None`。

**`parseFloat`**：ch1 浮点**核心形**——`数字串 '.' 数字串 (eE [+-]? 数字串)?`，全串、无下划线（ch1 的下划线规则是整数字面量段落的规则；浮点段落未含下划线）。范围错（上溢到 inf、下溢到零）一律 `None`——与 Go 折叠路对齐（`ParseFloat` 的 ErrRange 使 codegen 折叠让路，`codegen.go:2778-2781`）。

**与 Go 内部的偏差（如实记录，B3e 对账时兑现）**：Go 编译器折叠走 `strconv.ParseInt(text, 0, 64)`——base 0 连带前缀与下划线语义一并交给 strconv；We 侧分层：**std 收核心形，ch1 全形（0x/0o/0b、下划线、后缀剥离）的归一是 wec 词法器自己的代码**。两路对同一 ch1 合法字面量必答同值（B3e 的 .ll 逐字节对账（裁决 5）将机械police此点）；Go 借 strconv 下划线规则、wec 借自身归一，路径不同、折叠值同源。

**拒 strtod 全脸**：inf/nan/十六进制浮点/前后空白皆非 ch1 形，规范先行纪律下面不能比语言自己宽。

## D3 C 载体

落 `runtime/c/str.c`（既有 println/print 家族在此，命名 `__we_string_*`）：

```c
void __we_string_parse_int   (ptr s, i64 len, ptr out);   // out: Option<Int64> 三字
void __we_string_parse_uint  (ptr s, i64 len, ptr out);   // out: Option<UInt64> 三字
void __we_string_parse_float (ptr s, i64 len, ptr out);   // out: Option<Float64> 三字
i64  __we_string_rune_code   (i64 c);                      // return c
i64  __we_string_rune_from   (i64 n);                      // return n
i64  __we_string_float_bits  (double f);                   // union { double d; i64 b; }
double __we_string_float_from_bits(i64 n);                // 同上反向
```

- **Option 族走 out 三字组**（B2b `__we_coll_*_get` 先例：sum 三字 ABI 24B 超 SysV 16B 寄存器界，fs D7-2 已拒 C 结构返回）。三字布局 = 键控发射臂既有形：tag 字 + 载荷字（Int64/UInt64 原位 i64；Float64 的字即其 double 位）+ 第三字。C 侧 `Some` 写 tag 与载荷、`None` 写 tag 与零载荷。
- **NUL/长度纪律**（fs 先例）：String 可含 NUL 字节；strtoll/strtoull/strtod 皆 NUL 终止驱动。实现拷贝 len+1 到栈/堆缓冲加 NUL，解析后以 **endptr == 缓冲+len** 核对全串消耗（否则 `None`——中途 NUL、尾随垃圾同归）。
- **溢出映射**：strtoll/strtoull 的 `errno == ERANGE`、strtod 的 ERANGE（上溢 inf 与下溢零同查）→ `None`。空串、非数字打头 → `None`。
- **正确舍入**：strtod（glibc）按当前舍入模式（默认 nearest-even）正确舍入——这是 parseFloat 不可手写的理由（手写十进制→double 的正确舍入需要任意精度中间量，We 无 BigInt）。
- 四枚全函数是纯过桥，无失败路径、无分配、无 gc 交互（描记面零新增——不碰 gc）。

## D4 混合模块分派（本变更唯一的机制新面）

`std.string` 今日是唯一真体 std 模块：`import std.string` 的装载腿给 `curImports[alias] = "std.string"`（`codegen.go:1864-1874`），join/repeat 调用经 instCallee + 槽走程序面。本变更后同一别名下两类调用并存：

**分派规则**：限定调用 `string.X(...)` 先查**键控集**（七枚新名）；未中者照旧走程序面。落点在发射臂的限定调用分派处（今日 `stdQuals` 命中即键控的判据处）——string 别名得「先查键控成员表」的前置一步，其余 std 模块不受影响（它们的键控集是全集）。**检查器零码改**：七虚构体声明在 `stdlib/src/string.we`，B2a 装载机制（真 parser + 同一检查器）自然定型——monomorphic 具体类型，连 collections 的泛型合一步骤都不需要（T2 探针复核）。

**`registerStdModule("std.string")` 不需要**：该登记的和式/记录表服务限定类型拼写（`fs.FsError` 的熔合表）——七原语无和式/记录声明，`Option`/标量皆内建。装载腿除键控集外零触碰。

**真体白名单不动**：join/repeat 的 define 发射白名单（programModules 侧）不收七原语——虚构体 define 是死 IR（`codegen.go:1192-1195` 注释自述同一理由）。

**零漂移论证**：分派是「先查后落」——join/repeat 不在键控集，路径逐字节不变；fs/process/collections 的键控表与共享分派臂零触碰。T2 三缝复核（黄金零改写 + hello IR 逐字节 + mock 黄金绿）钉住。

## D5 Rune 域的如实记录（披露，非改动）

今日 Rune = i64 码点、无范围校验：`\u{1..6 hex}` 逃逸任意累加（`codegen.go:16718-16742`，无 0x10FFFF 上限、无代理区排除），rune 字面量收任意单字符。`runeFrom` 恒等过桥**忠实于**该事实——不新增校验也不假装有校验。范围域的缺席是既有语言面事实：归档时提议登记 follow-up（ch1/类型章应否固定 `\u` 逃逸与 rune 字面量的码点域）。**本变更不动它**：动了就是无规范依据的语言面变更（规范先行纪律）。

## D6 mock 面

七枚是单态模块 fn ⇒ ch20 的合法 mock 目标（B2b 的 E1804 泛型类目不适用——`mock collections.mapOf` 因泛型落 E1804，`mock string.parseInt` 无此形）。落位沿 B2a T6 的 outTrio 先例：

- Option 三枚：mock 拦截以双 define（真入口 + mock 槽）形接 out 三字组——B2a fs mock 的同形。
- 四枚直返：mock 拦截直接答期望值（i64/double 寄存器形）。
- 黄金：`mock string.parseInt` 拦截命中 + 非拦截路径照真入口。

## D7 验证策略

- **黄金矩阵**（先红后绿、手写期望字节、`WE_UPDATE_GOLDEN` 禁用）：正面（`parseInt("12")` → Some 臂打印、`parseFloat("1.5")`、`runeCode('A')` → 65、`floatBits(1.5)` 位型十进制值打印、`runeFrom(65) == 'A'`、`floatFromBits(floatBits(2.5)) == 2.5` 回环）；负面（`parseInt("")`/`parseInt("12a")`/`parseInt(" 12")` → None 臂、`parseUInt` 超 2^64−1、`parseFloat("1e999"→无点先 None`——注意 `1e999` 无点不合核心形、`parseFloat("abc")`）；溢出（`parseInt("99999999999999999999")` → None）；`parseInt` 正上界（`9223372036854775807` → Some）与 `parseUInt` 上界（`18446744073709551615` → Some）边界黄金。println 的 Float64 渲染字节以真机先跑对锚后手写。
- **突变电池**（B2b 判决表先例，逐枚单点突变、锚点先断言 count==1、单测层/黄金层各记判决）：候选 M-a endptr 全串核对删去（`"12\0x"` 型应 None 变 Some——黄金层捕获性待测）；M-b ERANGE 映射删去（溢出黄金）；M-c 键控集泄漏到 join（join 程序面应不变——IR 钉）；M-d runeCode 恒等改 `+1`（回环黄金）。
- **零漂移三缝**：既有 919 黄金 `-count=1` 零改写全绿；hello IR 对 HEAD worktree `cmp` 逐字节零差；34 mock 黄金绿。
- **runtime 侧**：`runtime/str_test.go` 直测 C 入口（NUL 中断串、endptr、ERANGE、位型回环）。

## 被拒方案

| 方案 | 拒因 |
| --- | --- |
| 挂成员面（`r.toInt64()`/`f.bits()`） | 裁决 2 拒；E0501 remediation 预告的清单归未来变更 |
| panic 作失败面 | 裁决 2 拒；`None` 是可 match 的诚实答案，panic 把可恢复畸形当任务失败 |
| parseInt/parseUInt 收全 ch1 形（前缀/下划线/base 判定进 std） | 面宽失度——归一是词法器业务，std 收核心形；分层后 wec 与 Go 折叠同值由 B3e 对账police |
| parseFloat 真体手写（We 源内十进制→double） | 正确舍入不可手写（无任意精度中间量）；这是七枚里唯一不可手写的，也是本变更存在的最小理由 |
| 四枚全函数做内联位型/恒等发射（免 C 入口） | 新发射形状换一枚 call 开销，统一键控形胜出 |
| Rune 域校验（`runeFrom` 拒代理区/超界） | 无规范依据的语言面变更；今日域即无校验（D5），规范先行纪律禁止顺手改 |

## 实现期补记

**补记一（T1，2026-09-17）：ERANGE 判据的精化——glibc 对 subnormal 也置位。** D3 溢出映射句「strtod 的 ERANGE（上溢 inf 与下溢零同查）」在实现时暴露一枚载体陷阱：glibc 的 strtod 对 **subnormal（可表示的 denormal）结果同样置 ERANGE**，故「ERANGE 一刀切 ⇒ None」会把 `2.5e-320` 这类可表示值错杀。落地判据以**答案自身的形状**为准（与 D2 的措辞「上溢到 inf、下溢到零」逐字一致）：ERANGE 仅当舍入答案离开有限非零清单——位型指数字段全一（inf；核心形不产 nan）或答案为零——才 `None`；subnormal 存活为 `Some`（denormal 行因此进 harness 断言与 D7 的位型回环）。inf 判据读位字（`bits_is_finite`）而非引 math.h——str.c 的 include 集合保持原样（仅新增 errno.h）。
