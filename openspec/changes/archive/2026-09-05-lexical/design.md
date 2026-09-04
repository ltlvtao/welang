# lexical — 设计

规范依据：`docs/spec/0100-lexical.md`（下称 ch1）全部 Requirement；`docs/spec/2100-toolchain.md` R1/R2（下称 ch21）；注册表 E0001–E0009 条目逐字引用。以下每条决策标注规范条款与机制自由度的边界。

## D1 Token 模型与位置

`Token{Kind, Text string; Line, Col int; AttrName, AttrArgs string}`，`Kind` 为字符串：`ident` / `keyword` / `int` / `float` / `string` / `rune` / `attr`，运算符与标点的 Kind 就是其自身文本（`"+"`、`"=="`、`".."`、`"#["`、`"_"` …，ch1 Operators and punctuation 的封闭清单）。词法器输出 token 流并以一个 `eof` token 结尾（M2 解析器消费）。`AttrName`/`AttrArgs` 仅在 `attr` token 上填（属性是「一个词法结构」，不拆子 token；被拒方案见 D12）。

- 位置：行 1 起、列 1 起、**列按 rune 计**（ch1 未固定单位；rune 计列对非 ASCII 字符的 E0001 定位最自然，记为机制决策）。
- CRLF 记一次换行、行号在 LF 处递增；单个前导 U+FEFF 剥离（ch1 Source files「机械归一化」场景：两者都不产生诊断）。
- 裸 `_` 是标点 token（ch1 清单将 `_` 列入标点），`_x` 按标识符规则吞噬为 `ident`——最大吞噬下两者不冲突。

## D2 报告粒度：首错误停止

词法器对每文件至多产生 **一个** 诊断：首个错误处停止扫描、不产出 token 流（ch1 Source files 场景「reports E0001 … and does not produce a token stream」；ch21 R2「E 诊断停止管线」）。多错误批量是规范未固定的机制自由度，本变更不发明（被拒方案 D12）。

## D3 数值字面量：贪婪吞噬 + 整段校验

从数字起始处贪婪吸收 **数值形状段**：`[0-9a-zA-Z_]` 全吸收；`.` 仅当 **不** 开启 `..` 时吸收（ch1 明文「数值字面量的最大吞噬绝不吸收开启 `..` 的点」：`0..9` → `0` `..` `9`、`1.5..2.5` → `1.5` `..` `2.5`）；`+`/`-` 仅在小数上下文 `e`/`E` 紧后吸收（`1.5e+3` 合法形状）。吸收完成后对整段校验，任一条不满足 → **E0006 于段首**，消息按被违反的条款命名（ch1 Numeric literals 三个场景 + 注册表 description 的五个子句）：

1. `0x`/`0o`/`0b` 前缀后至少一个该进制数字（`0x`、`0b2` → E0006；`0b12` 整段已由前缀提交 → E0006，而非 `0b1` + `2` 两 token——前缀场景明文提交，段内进制外数字是形状错误）；
2. `_` 仅夹在两个数字之间（`1__0`、`100_`、`0x_FF`、`100_i64` → E0006；`_100` 以 `_` 起始走标识符路径，永不进入数值）；
3. 十进制整数字面量无前导零（`0` 自身与 `0` + 后缀允许；`01` → E0006）；
4. float 的点两侧各有至少一位数字（`1.`、`1.x` → E0006；`.5` 以点起始是标点 + 整数，永不进入数值）；
5. 指数 `e`/`E`（仅 decimal、仅含点的形状）后可选符号、至少一位数字（`1.5e` → E0006；无点的 `1e5` 不构成 float 尝试 → `1` `e5` 两 token）；
6. 后缀紧跟末位数字且为闭集：整数 `i8 i16 i32 i64 u8 u16 u32 u64`、float `f32 f64`（`42i8`、`3.14f32` 合法；`42int`、`42f32` → E0006）。

**Token 不携带值**：Text 存原始段，值解码（进位、后缀类型化）属类型阶段（ch1「unsuffixed literal 的类型由类型章定义；本章只定义形状」）。

## D4 字符串 / rune 字面量与插值

单行；转义闭集 `\n \t \r \0 \\ \" \' \u{h..}`（1–6 位十六进制、值 ≤ U+10FFFF、禁代理项），其余字符跟在 `\` 后 → **E0005 于反斜杠处**，消息点名该字符；`\u` 形状违规（位数、值域、代理）同为 E0005。

- 行末（LF，含 CRLF 的 LF）前未闭合：字符串 → **E0002 于开引号**，rune → **E0003 于开引号**（ch1 场景「at the opening quote」）。
- rune 内容须恰为一个字符或一个转义：空 `''`、多字符 `'ab'`、转义后带字符 `'\n x'` → **E0003**，detail 点名内容违规（注册表 E0003 description 本身把「恰一个字符或一个转义」折入该码；无独立码，不发明）。
- 插值：字符串内 `$` 紧跟 `{` 开洞，洞内花括号须在 **字面量内** 配平（ch1 场景：`"${f(}"` → **E0007 于 `${` 的 `$` 处**）；洞是「balanced-brace 表达式区域」，表达式可含嵌套字符串/rune 字面量——扫描洞时遇 `"`/`'` 递归按字面量规则跳过（其内花括号不计数、转义照常校验）；`$` 不跟 `{` 是普通字符（永不触发）；洞外裸 `}` 是普通字符。**配平的机制**：洞内未闭的 `(`/`[` 保持洞开启——`}` 仅在无未闭括号时才关洞。规范示例 `"${f(}"` 在纯花括号计数下其实是平衡的（`}` 关洞、`"` 关串）；正是括号保持让 `"` 落入仍开启的洞、作为无法闭合嵌套串的引号 → E0007，场景因此命中。
- 洞内引号的歧义规则：洞开启期间遇 `"`，先按嵌套字符串尝试（软消费，行末即失败）；失败则该引号只能是外层字面量自身的闭引号，而洞未闭 → **E0007 于 `${` 处**。
- 字面量内合法非 ASCII 自由使用（ch1 Source files / Identifiers：非 ASCII 仅注释与字面量内允许）。

## D5 注释

`//` 与 `///` 至行末（`///` 的附着规则属第 6 章，本变更按行注释扫描跳过、不产 token——非目标）；`/* */` 不嵌套，首个 `*/` 关闭整个注释（内层 `/*` 是普通文本）；EOF 前未闭合 → **E0004 于 `/*` 处**。注释如空白分隔 token，内容可为任意 Unicode 文本。

## D6 编码、BOM、裸 CR

读入后先整文件 `utf8.Valid`：非法 → **E0001**，消息含违规字节与字节偏移（ch1 场景「with the offset of the offending byte」——偏移进消息，行/列仍按规范协议字段给出）。U+FEFF 仅允许出现在偏移 0（剥离），其余任何出现 → E0001。裸 CR 不跟 LF → E0001（任何位置，含字面量与注释内——Source files 条款是文件级）。

## D7 属性单元（E0009 先于 E0008）

`#[` 是一个 token：后跟名字（标识符形状）、可选 `(...)` 参数区、闭合 `]`；组件间允许水平空白。扫描顺序与校验次序：

1. **形态**：行结束或非预期字符出现在 `]` 之前 → **E0001 于该处**（detail：属性单元期望 name、可选 (args)、`]`）。
2. **参数区字面量校验** → **E0009 于首个违规参数起始处**，detail 点名参数文本：参数文法 `arg := string | number | true | false | '[' arg-list ']'`（逗号分隔、允许尾逗号、空参数区合法）；任何标识符引用、运算符、函数调用（如 `5000 + n`）违规。
3. **名字白名单** → **E0008 于 `#[` 处**，detail 点名属性名。

**次序论证**：白名单「由已批准章节拼装」（ch1 Attribute lexical unit）；全量 grep `docs/spec/` 证实 **没有任何章节批准过任何属性**——白名单今天是空集，`#[inline]` 即注册表场景原例。若先查名字，E0009 将不可达（任何名字都先死于 E0008），与注册表场景 `#[timeout(5000 + n)]` 的可测试性矛盾；「参数只能是编译期字面量」是独立于名字合法性的形态规则，先于名字校验。白名单实现为显式空集常量（附 grep 证据注释），后续章节批准属性时随其变更扩表。

## D8 人类可读渲染的位置前缀

带位置的诊断（`At` 已设）人类一行制为 `file:line:column: error[code]: title — detail`（行业惯用的前缀式；ch21 只固定 JSON Lines 协议与 `error[Exxxx]:` 形状，位置渲染是机制自由度，本决策即事实上的稳定表面）。无位置诊断（E1904/E1907 等命令层）保持 M0 形状不变。JSON Lines 字段集与次序零改动。

## D9 `we check` 分派

E1907 在 dispatch 前既有路径不变（M0 黄金用例 build-e1907 / vet-e1907 不动）。`check` 自 `subcommands` 表的通用 70 边界中摘出（`implemented: true`），进入专属分派：

1. stat 为目录（含默认 `.`）→ `we: project compilation is not implemented in this reference build yet` + exit 70（manifest 解析属 M3）。
2. 文件路径不以 `.we` 结尾 → usage 错误（`we: check wants a .we file, got %q`）+ exit 2（ch21「文件路径命名一个 `.we` 文件」；非 `.we` 是调用方误用，与未知选项同类——规范未固定，记为机制决策）。
3. 读文件失败 → `we: <err>` + exit 1（M0 fsError 先例）。
4. 词法：`lex.File(path, src)` → 首个诊断 report（--json 走 stdout 事件，否则 stderr 人类一行）+ exit 1；干净 → `we: parsing is not implemented in this reference build yet` + exit 70。

诚实边界从「子命令级」收缩为「阶段级」，集合收缩记录于本变更；`build`/`run`/`test`/… 维持子命令级边界。

## D10 关键字表

ch1 Keywords 封闭枚举 40 词（8+10+3+5+1+4+2+1+5+2）：`fn let var pub import as mut` / `if else return match for in while loop break continue defer` / `true false foreign` / `record byval byres newtype with` / `type` / `interface impl where derives` / `scope resource` / `effect` / `task select case timeout collectAll` / `test mock`。标识符形状段吞噬后查表：命中 → `keyword`，未命中 → `ident`（`records`、`types` 是标识符；`collectAll` 是关键字——大小写全敏感，ch1 Identifiers「Case sensitivity is total」）。「关键字出现在标识符位置」的拒绝属解析阶段（ch1 Keywords 场景明文「the parser reports」），词法层不报。

## D11 conformance

`Setup` 增 `Files map[string]string`（dirs 先建、文件后写）。新增黄金用例（期望全部手钉自规范场景，非实现回填）：

| 用例 | 输入要点 | 期望锚点 |
| --- | --- | --- |
| check-e0001-inventory | `let price = $` | E0001 于 `$`，消息点名字符 |
| check-e0001-homoglyph | `let 名 = 3` | E0001，homoglyph 理由（ch1 Identifiers 场景） |
| check-e0002 | `let s = "abc`（行末） | E0002 于开引号 |
| check-e0003 | `let c = 'a` | E0003 于开引号 |
| check-e0004 | `/*` 未闭合 | E0004 于 `/*` |
| check-e0005 | `"\e"` | E0005 于反斜杠，点名 `\e` |
| check-e0006 | `1__0` | E0006，分隔符条款 |
| check-e0007 | `"${f(}"` | E0007 于 `${`（ch1 原例） |
| check-e0008 | `#[inline]` | E0008（ch1 原例，空集白名单） |
| check-e0009 | `#[timeout(5000 + n)]` | E0009 于参数（ch1 原例，D7 次序） |
| check-e0006-json | 同 e0006 + `--json` | JSON 事件全字段（file/line/column/help） |
| check-clean | 干净 `.we` 文件 | exit 70，parsing 边界一行 |
| check-clean-json | 同上 + `--json` | exit 70、stdout 空（边界不是事件） |
| check-bom-crlf | BOM + CRLF 干净文件 | 无诊断（机械归一化场景）→ 70 |
| check-nonwe | `main.txt` | usage exit 2（D9.2） |

改写 M0 两例：`check-boundary`（空目录 → project compilation 边界）、`check-boundary-json`（同上 + `--json`，stdout 空）。其余 18 例不动。

## D12 被拒替代方案

- **属性参数拆子 token 流**：违背「一个词法结构」的章文；解析器 M2 需要时从 `AttrArgs` 原文再切。
- **多错误批量报告**：规范未固定；首错误停止是最小确定读取，且与「不产出 token 流」的章文一致。批量需先有规范裁决。
- **E0008 先于 E0009**：空集白名单下 E0009 不可达，注册表场景失去可测试性（D7 论证）。
- **现在引入 pipeline/compiler 组织层**：解析尚不存在，词法器独立成包即薄垂直；M2 引入解析时再立管线编排骨架，避免空抽象。
- **`1.` 吞点 vs 不吞**：不吞则注册表「float 点两侧须有数字」条款永不触发（`.5` 走标点路径、`1.` 变两 token）；吞点使 `1.x` 报 E0006——全规范无 `1.5.sqrt()` 式用例（已 grep），无冲突。
