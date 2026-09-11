# design — codegen-mono（B1a）

调研基线（两路 Explore + 自查，2026-09-10）：codegen.go 4598 行接受集全清点、检查器知识瞬时性、runtime 符号缺口、AST 形面、M10b ABI 基座、M15 重锚连带。本 design 按 D0–D13 组织；每节给唯一路径与被拒替代。

## D0 两塔切分与零集账目

B1 权威行（roadmap:36）的验收 = codegen not-implemented 集合归零。今日 8 词（codegen.go:42-54）；两塔清零账目：

| 停点词 | 塔归属 | 退役机制 |
| --- | --- | --- |
| `bndMainBody` / `bndTaskBody` / `bndFnBody` | **B1a**（D1+D2） | 语句集完备，三锚改词后删（B1b 收尾时 `bndGenericFns` 删后全表空） |
| `bndTopLets` | **B1a**（D7） | 模块 init 发射 |
| `bndErrPayload` | **B1a**（D8） | sum 物化泛化 |
| `bndCallbackBody` | **B1a**（D5） | 闭包全捕获 |
| `bndAssertEqDomain` | **B1a**（D9） | derive 结构比较 |
| `bndGenericFns` | B1b | 单态化（Q1 裁决；~~B1a 后唯一幸存词~~，词表括注改指 B1b——**订正见下**） |

B1a 验收面：**停点集合恰为 {bndGenericFns}**（机械可判：grep 词表 + conformance 9 枚锚定黄金重锚后全绿）。

**订正（T13 收口，2026-09-11 用户裁定 1，原文保留）**：上句是设计期预测，**被实现推翻**。B1a 落地时词表余 **5 词**（`bndMainBody`/`bndTopLets`/`bndTaskBody`/`bndGenericFns`/`bndFnBody`）而非 1 词；冻结的 23 枚 exit-70 黄金里 **15 枚**锚在三枚体锚上（main 12 / generic 2 / fn 1），另 8 枚锚在四个非 codegen 词上（single-file ×4、std ×2、ch21 ×1、arity ×1）。原因是拓宽后的塔仍够不着三类诚实停点：用户方法的 `?`（`build-bnd-m6b-main-body`）、List 的 String 元素面（`build-bnd-list-elem`）、同步型作返回型（`build-bnd-prim-return`）。**同一错还推翻了 D12 的第 10 枚预测**（`build-bnd-m6b-main-body` 的「翻绿改锚」——今日仍 exit 70 + `bndMainBody`）。三锚退役改挂 B1b 实际落地时；词表字节与生产点零改动。逐枚对账入实现记录 T13。

被拒替代：一塔全量（B1 整体）——泛型单态化需要检查器注记面（Q2）这个独立前置机器，与单态值塔正交；混载则单塔过大且注记面裁决会被实现细节倒逼（粒度规则打回形）。

## D1 env 作用域化 + 语句集完备

**根因先修（follow-up #16）**：今日发射环境是平坦映射（`e.scalars`/`e.strEnv` 等从不按块作用域化）——match 臂绑定与嵌套块 let 的 SSA operand/alloca 在 join 点后仍可读，capture 按名解析无作用域信息 → 同名巧合命中泄漏绑定 → 非支配 IR（clang 拒绝 `Instruction does not dominate all uses`）。修法：发射环境引入**块作用域栈**（进入块 push、退出 pop；match 每臂一帧）；capture 解析走作用域栈（深度语义与检查器 lookupLocalDepth 镜像）。这是 D1 其余项的机制前提（任意深度控制流嵌套下名唯一性靠它成立）。

被拒替代： alloca 全提升 + mem2reg 依赖 clang 优化——决定论发射纪律（同输入同 IR 文本）拒绝把正确性押在后端优化 pass 上；且 capture 按名巧合命中的**语义**错误（拿错值）alloca 提升治不了。

**语句集**（三锚 main/task/fn 同集，emitStmt 的 default 兜底面收窄）：

- `loop` / `break` / `continue`：无限循环 + 跳转（break 跳出块、continue 回环头）；defer 与 M9b 倒排机制组合（break/continue 出块 = 块出口，跑该块注册的 defer——ch3 LIFO 语义）。
- `for name in expr` / `for pat in expr`：D6 的迭代面（协议 = 求值一次 → iterator 一次 → next 至 None；List 快照/Range 计数循环/String 码点游走——Range 与 String 的形在实现期修正，见 D6）。
- `scope resource(...)` 块：绑定多资源、逆序释放（运行时 `__we_res_*` 或直接调用 Releasable 方法——见 D4 方法表；早出口穿透）。**实现期次序核验（T2-c，2026-09-10）**：本条与下一条的机器都在 D4——资源的释放要呼 `impl Releasable` 的 fn 体，而 impl fn 体今日整体 erase（不发射 IR），元组模式要解构元组值（D4 的聚合布局）；故两面并入 T5 执行，见 proposal.md 实现记录 T2-c。
- 元组模式绑定 `let (a, b) = ...`：解构到各槽（值面随 D4 的元组聚合落地）。
- 裸块语句：一帧作用域 + 块尾值弃置走 E0605 已检查面（check 拦，codegen 不到达）。
- 任意深度 `return`：从嵌套控制流直返（defer 倒排链在 return 处全部跑）；task 体 return 出 task。
- `self.field = expr`：D4 记录字段存（接收者指针 getelementptr）。
- 赋值面放宽：`Assign` 分支删 `!slot.isVar → bnd`（codegen.go:846）——let 槽同 var 槽可存（检查器对 let 重赋值是 M3 行为=接受，follow-up #7 在册；codegen 拒绝会造成 check/build 面静默分裂）。let 标量绑定从 SSA operand 改为 alloca 槽（可寻址）——仅对被赋值名；纯读名保持 SSA。

## D2 表达式值形完备（单态）

- **if/match/块值表达式**（值位）：结果 alloca 模式——`%r = alloca <ty>`，各臂算值后 store，join 后 load。块值 = 尾表达式求值（**体先于尾**：修复缺陷②，见 D10——合法 IR 但值序错误）。unit 结果免 alloca。
- **一元** `!`（Bool icmp eq false）/ `-`（sub 0, x 带溢出陷阱）/ `~`（xor -1）。
- **`/` 与 `%`**：sdiv/srem + **零除检查**（icmp eq 0 → `__we_task_fail("division by zero")` panic 面，ch14 族；`%` 语义 = srem 截断形——校准例 control-04 `n % 2` 非负输入下正确）。Float `/` = fdiv、`%` = frem（IEEE，无陷阱面 ch7）。**实现期补两处（T3，见实现记录）**：① LLVM 对 `INT_MIN / -1` 与零除同列 UB，而 ch7 的整数算术是逐值 checked 的，故 `/` 加第二道守卫（icmp MIN & icmp -1 → `__we_task_fail("Int64 div overflow")`）；`%` 无此患——该对的余数恰为 0，`select (b == -1) ? 1 : b` 一指令消解（`a srem 1 == a srem -1 == 0`）。② 源码拼写的常量除数且 ∉ {0, -1} 时省掉守卫直接发 sdiv/srem：非 0/-1 的常量除数无守卫可言（零与非 1 的 -1 都在旁路条件外），且 benchmark 热点形 `n % 2` 由此是单指令 srem。**实现期订正（T3，见实现记录）**：本条初稿写「字面量 0 已被 ch7 常量折叠 E0502 拒绝」——实证为假：`let q = 10 / 0` 今日 check 干净（ch7 的 E0502 只覆盖**溢出**；折叠器对零除返回未定义并静默让路，`foldApply` 两个分支都是 `ok=false`），故零除的唯一捕获面就是本条的运行期陷阱。
- **`&&`/`||` 短路**：分支形（条件 br → RHS 仅在需要时求值）。今日 i64 and/or 是潜伏错误（现集合无调用侧效应故安全；D2 拓宽后 RHS 可含调用/panic，必须分支形——修复归 D10 记账）。**join 的 phi 表项必须指实际前驱块**：RHS 自带控制流时（除法守卫、嵌套值形）其发射停在若干块之后，写 RHS 起始标签会让 clang 拒整模块（`PHI node entries do not match predecessors!`，T3 真机电池抓到）——emitter 记当前块（`curBlock`），表项取发射结束时的块。
- **位运算二元族 `& | ^ << >>`**（2026-09-10 裁定并入本变更）：`& | ^` 全值域直发（`and`/`or`/`xor i64`，任何位形都可表示、无陷阱面）。`<<`/`>>` 两道守卫：① **移位量域**——LLVM 的移位指令对量 ≥ 位宽（或负）是 poison，故 `icmp ult i64 b, 64` 守之（无符号比较一并覆盖负量），越界 → `__we_task_fail("Int64 shift overflow")`；② **`<<` 的丢位**——LLVM 的 `shl` 静默丢弃移出顶端的位，正是 ch7「绝不静默回绕」禁止的形，故移回校验（`ashr v, b == a`）不还原即同上陷阱；`>>` 无此面（右移丢的只是低位，那正是右移的语义）。`>>` 取**算术移位**（`ashr`，有符号操作数的保号读法）。越界/丢位语义登记 roadmap follow-up #20。
- **rune**：i64 域标量（charAt/runeCount 返回；无独立存储形）。
- 溢出陷阱沿用 llvm.s{add,sub,mul}.with.overflow.i64（i64 域计算）；**窄整型逐宽检查**见 D10④。
- **陷阱消息指名操作**（2026-09-10 裁定，兑现 ch14）：`Int64 add overflow` / `Int64 sub overflow` / `Int64 mul overflow` / `Int64 div overflow` / `Int64 neg overflow` / `Int64 shift overflow` 各带己文，通用 `integer overflow`（M5 起既有的共享文案）退役；窄整型的逐宽命名（`Int8 add overflow` 形）随 D10④ 一并落。除法/取模的舍入与符号约定（roadmap follow-up #18）与除零语义（#19）按裁定维持现状并登记。

被拒替代：一切表达式压平为 select 指令——select 两臂都求值，panic 面/副作用序错；分支形是唯一正确形。

## D3 字符串全链

运行时新族（runtime/c/str.c，命名循 `__we_` 前缀纪律）：

- `__we_str_concat(a_ptr, a_len, b_ptr, b_len) -> struct we_str`（新分配；gc 域外 **malloc 裸分配、从不推根、永不回收**——M8 先例的构造协议**不适用**，见下方「T4 实现订正」①；**被拒替代**：改为 gc 分配内联——分配器接口属 ADR-0003 门，本变更用裸分配侧门，B2 收编）。
- `__we_str_eq(a_ptr, a_len, b_ptr, b_len) -> i64`（memcmp 定长比较；真值按 Bool-as-i64 读法，见订正②）。
- `__we_str_runecount / __we_str_charat(p, n, idx) -> i64` / `__we_str_byteslice(p, n, lo, hi) -> struct we_str`——端斥 OOB panic（ch17:63 端斥句「end exclusive … an out-of-range range is a panic, the chapter 14 family」，panic 族 `__we_task_fail`；`bytelen` 无独立符号，见订正③）。
- **插值**：洞逐段求值 → 值转串面。**值域（实证定谳，2026-09-10）**：规范对插值洞值类型无锚（ch1:130 只定词法「balanced-brace expression region」；ch7/ch2 无值域句），检查器零值域限制（record 插值 check 实证 exit 0）。发射面 = 基类型族（8 整型/Float/Bool/Rune，`__we_str_of_*`）+ String 恒等 + unit 空串 + **具体复合值结构渲染**（record `User{n: 1}` 形/元组 `(f0, f1, …)`/sum 变体名 + 载荷——渲染机制随 D4 遍历骨架，递归、无环不可变构造下终止；**与 derive Show 解耦**：渲染形是规范沉默位的实现定义，披露于完成记录；derive Show 合成体自身无消费方，归 B2）。**泛型洞值**（泛型参数名入洞）随函数体整体停 `bndGenericFns`（词意吻合——词文就是泛型函数体）。

发射面：String 绑定从 strEnv 字面量对扩为运行时值 {ptr operand, len operand}；字面量为私有常量 + 双 operand。**发射集边界（T4 定形）**：值形与 `let` 绑定；`var`/重赋值的**两字槽存储面**不在本任务内——**裁定（2026-09-10）：并入 T9「sum 物化泛化 + 槽放宽」**（该任务 checkbox 已含「String 参数双标量展开」，同族；D8 已加注本面扩形）。今日该形 check 干净、test 面 exit 70，机械钉在 M15 的 `control-03/string-mutable-binding-decline` 上，T9 收口时该校准转 latent 并再寻 test-malformed 载体（T3 先例）。

**T4 实现订正（2026-09-10，实证）**：

① **分配协议推翻设计稿**——设计稿的「gc 域外裸分配 **+ 立即 root push 构造协议**」后半句不成立：裸 malloc 的字节缓冲不是 gc 对象（无头部、无描述符），非 gc 指针推上 shadow root 栈会被收集器读成块头。订正为「malloc 裸分配、从不推根、永不回收」（裁定①：字符串缓冲 malloc 裸分配、永不回收）。正面结果：字面量常量与家族缓冲皆长生不灭 ⇒ `byteslice` 可返**内部指针**（String 不可变，共享不可观察）；**登记 roadmap follow-up #21**（回收归分配器工作，ADR-0003 门，B2 收编）。
② **`eq` 返 i64 而非 `i1`**——发射面按 Bool-as-i64 寄存读真值（`icmp ne %eq, 0`），`i1` 形需额外 zext 且与既有 Bool 表示不一致。
③ **`bytelen` 不实现**——`byteLength` 折叠为长度操作数本身（零指令），独立符号无消费方。
④ **插值渲染的两处实现定义**（规范沉默位，随完成记录披露）：无效 UTF-8 读 U+FFFD 且推进一字节（`runecount`/`charat` 共用 `utf8_step`，故互洽且全定义）；`of_f64` 取最短往返十进制（裁定②）。

## D4 记录/元组/newtype/unit 全值面 + 方法表

- **布局**（M10b 已定）：gc record = `__we_alloc` + 16 字节冻结头 {map@0, size@8}，字段序源序偏移 16+8i（标量 i64 域/float double/String 双字/gc ptr/嵌套 record ptr）。value record = 同 ptr 表示，绑定/传参/赋值 = 整值拷贝（`__we_rec_copy` 新面：新分配 + 字段逐拷——value 域字段直拷、gc 域字段拷指针）。
- **标量字段读**：`getelementptr` + load（今日仅 String 字段链可读——删专面，统一成员读发射）。
- **`with &` 更新表达式**：拷贝构造（同 `__we_rec_copy`）+ 具名字段覆写。
- **方法表（静态派发）**：从 AST ImplDecl 建 (型名, 方法名) → fn 表（模块限定符号已 M10b 在位）；接收者 concrete 型已知时直呼（含 inherent impl 与 interface impl 的具体实现）；默认方法体 = 接口声明内真体同表。mut self 经 self 指针可写字段（D1 `self.field=`）。**Dyn 接收者与泛型方法 = B1b**（vtable/单态化）。
- **newtype**：零擦除恒等——构造 = 内值、`.value` = 恒等、传参/返回 = 内值 ABI（ch8 零成本承诺的 IR 兑现）。
- **unit**：void（已在位，钉死勿回归）。
- **元组**：value 聚合 {f0, f1, ...} alloca；模式解构逐位存；传参逐字段展开（value 语义拷贝）。

**T5 实现订正（2026-09-10，实证）**：

① **`__we_rec_copy` 落成站点内联 `emitRecCopy`**——设计稿点名的运行时新面不成立：根推送按 body 记账（`e.pushes` 在每个 body 出口弹），callee 的 push 无法由 caller 平衡。拷贝在站点内联展开，其 push 记在发起的 body 上；面是真、拼写是发射器的（理由见 `record_test.go` 文件头）。
② **章 6 的隐式尾返回在 T5-2 落地、T5-4 收窄**——ch10 方法与章 19 释放体都写「体末项表达式即返回值」的形，故 `emitFnDefine` 的尾项识别加 `*ast.ExprStmt` 臂；但章 6 只在**声明了返回类型**时如此（未声明返回类型者归第 8 章值丢弃规则），首版臂对 void fn 一并提升 ⇒ 释放体（章 19 的裸尾调用）被按值返回分类、停 bndFn。收窄为 `fd.decl.Ret == nil` 即不提升。main 体的同形仍停（归 T9「fn 尾返回停点退役」）。
③ **scope resource 的放电机器**：`nest` 统一嵌套序数（scope expr 与 scope resource 同序数空间），exit 记 `depth`；`unwind(depth)` 合并 `scopeLive`/`resFrames` 中 `depth ≥ 参数`者、按 depth **降序**发射（内层先出 ⇒ 块出口释放先于外围函数的 defer），`releaseRes` 倒序取 `methods[key+".release"]` 直呼其符号。不透明头（foreign 块）与 record 资源走同一条路——头键即 `opaques` 表的键。
④ **不透明资源的两处入口面**（章 19）：不透明头的方法把 `self` 直传原生 `close`，故 `emitForeignCall` 的不透明实参需认 `gcEnv`（方法接收者）与 `prims` 两处；scope 头接受不透明调用结果需 `emitResHead`（不透明返回是 `ckPrim` 裸指针，而 `emitRecordValue` 的 Call 臂只认 `ckGc`）。

被拒替代：方法表经检查器（Q2 注记面）——B1a 自建表（AST 直读，与 sum 变体表回查同 M4 先例）是注记面落地前的诚实桥；B1b 注记面到位后吸收。

## D5 闭包全捕获 + fn 值

- **载体**：gc 分配 `{fnptr, env}` 对（gc record 布局：头 + 两槽）；fn 值 = 该指针。
- **捕获**：value 域（基类型/元组/value record）= 字拷贝入 env 块；gc 域（gc record/String 头/sum gc 载荷）= 指针入 env（可 trace）；闭包调用 = load fnptr + env 传首参。
- **回调 ABI 兼容**：prim 回调（Mutex.update 等）今日 (fn, env) 对形 —— 闭包值直接满足（fnptr = thunk、env = 载体指针）；零捕获闭包 = env 空块。
- **fn 值调用**：fnValueCall 经 `call fnptr(env, args...)`；顶层 fn 名作值 = `{@fn, null}` 常量对。
- 捕获纪律镜像检查器（E1002 byres/E1003 值赋值已 check 期拦；codegen 不重复判）。
- **实现期订正（T6，2026-09-10）**：①「顶层 fn 名作值 = `{@fn, null}` 常量对」**被订正**——既然调用点一律发 `(ptr env, args…)`，而声明式 fn 的符号没有 env 参数，值位的裸 fn 必须经一枚**适配 thunk**（`emitFnAdapter`：`(ptr %env, a0…an)` → `call @sym(a0…an)`）方能满足统一约定；载体常量对为 `{适配 thunk, null}`。首版按原文直发 `{@main.square, null}`，`apply(f, 2)`/`apply(square, 3)` 实测输出 `0` 而非 `4`/`9`（null env 被 `square(n)` 当成了 `n`），真机揭出。②「零捕获闭包 = env 空块」**被订正**为空指针 `null`——空块也是分配也是根推送，而 null 是既有黄金与单测的形（零漂移）。③「gc 域（gc record/String 头/sum gc 载荷）= 指针入 env（可 trace）」**对 String 收窄**：String 的两字入块但**不置 trace 位**——`str_alloc` 是 malloc 且永不回收、字面量指向只读常量池，而 gc.c 的 mark 会**对指针写 MARK 位**（写 rodata 是段错误、写进字符串自己的字节是数据损坏）；与任务捕获面（只 trace prim 句柄）同规。④ fn 型参数跨边界 = **一个载体指针**（`abiFn`），签名随载体静态传递（`fnParamAbi.sig`）——绑定/参数/捕获/实参四处的签名来源就此统一。

## D6 List 载体 + 组合子内建面 + for-in

- **载体**（runtime/c/list.c）：`__we_list_new(cap, traced) -> ptr`（16 字节冻结头 + len/cap 双 i64 + 数据区；traced 位入 map 位图描述符——元素为 gc 指针时置位，mark 期按元素位 trace）；`__we_list_push(ptr, val)`（满则新分配双倍 + 拷贝——mark-sweep **无紧缩**下旧块立即死亡安全）；`__we_list_get(ptr, i) -> i64`（OOB panic）；`__we_list_len`。
- **List 字面量** `[e1, ..., en]`：new + 逐 push；空 `[]` 经期望型（E1501 已检查面）。
- **快照迭代**：`iterator()` = 拷贝载体（`__we_list_snap`）——ch17 快照语义（调用时刻固定序列）；next = {i++ 越界 None}。
- **for-in**：List = 快照循环；**Range** = 物化数组（start..end 循环 push——B2 改惰性）；**String** = 物化码点数组。
- **实现期修正（T2-b，2026-09-10）**：Range 一行**被替代**——Range 源直发**计数循环**，不物化数组（两形对 Range 观测等价：边界按源序各求值一次、单位步至 end、无可观测分配；计数循环不需要 T7 的载体也不需要堆，B2 的惰性要求对 Range 天然满足）。String 一行改述为 **rune 游走**（`__we_str_runecount`/`charat`，复用 D3 运行面，不物化）。源归属随之重划：Range → T2-b（已落）；String → T4；List → T7。论证与证据见 proposal.md 实现记录 T2-b。
- **组合子内建面**：6 急性枚（fold/reduce/count/any/all/find）在 codegen 识别为内建调用形（接收者 List + 限定名匹配）→ **循环直发**（不发射 std 模块 fn 体；We 体仍是检查面每 check 走查——M8 同机受检先例）。5 惰性枚（map/filter/take/skip/collect）返回 `Dyn<Iterator<U>>` → **B1b**（vtable 面），B1a 停 `bndGenericFns` 同词（调用面属泛型方法）。

被拒替代：发射组合子 We 真体——需要泛型单态化（B1b 机器），且 std 内建模块发射链是 B2 面；内建直发是最小正确路径（语义权威 = ch11/ch17 章文，循环直发按章文实现，校准面 conformance 黄金钉行为）。

## D7 顶层 let + 模块 init

- `@<key>.init` 每模块一枚：装载后序（loadGraph 已算好编译期序）main 前逐模块调用（startup.c 调序面）；模块内源序。
- **全局根登记**：gc 顶层绑定（String 头/gc record/list 载体）入全局根表——`__we_gc_root_global(ptr)` 新运行面（boot 期 init 前后各跑一次根扫描屏障：init 期分配的 gc 值在后续收集中必须可达）；标量顶层绑定 = 全局 `@<key>.<name>` 直存（M10b 槽形已在此，扩值域）。
- init 期 panic 依 ch15 R5（进程中止——`__we_task_fail` 面）。
- M10b 预披露兑现：「bndTopLets 在位时零观察行为——顶层绑定发射归 B 轨，届时 init 需真代码，本决议随之退役」（M10b design 模块 init 静态化决议原文）。

## D8 sum/Err 物化泛化 + 槽放宽

- **布局裁决：sum alloca = `{tag, pay0, pay1}` 三槽**（变体表驱动，M9b 已有 sum 表回查）。M9b 的 `{tag, pay}` 双槽装不下 String 载荷（一对 `(ptr, len)`）与双标量载荷，而第二个载荷位必须**先存在**，任何构造路径才有可能往里放值。三槽直存是最小形；聚合返回 = 三 i64 多 retval（M10b 六族 ABI 的 sum 行扩形，黄金对拍过渡期披露），聚合参数同形。**被拒替代**：保持 `{tag, pay}` 双槽（String 载荷无处可放）；载荷堆分配指针化（多一层间接且引入分配失败面）；四槽 `{tag, pay0..pay3}`（只把嵌套边界从一层推到两层而不关闭）。
- **Err 融合 tag 空间（T9-2 裁定）**：`Result<T, E>` **一个 tag 空间花在两边**——tag `0` = Ok，tag `1+k` = E 的第 k 个变体；`pay0`/`pay1` 两侧**别名复用**，融合不为 E 另花第二对。E1204 保证 E 必为具名 sum（`check-e1204-result` 钉 `Result<Int64, String>` 为 check 拒绝），故该编码对每个合法 `Result` 都是**全函数**，三槽恒定：

  | 值 | 三槽 |
  | --- | --- |
  | `Ok(7)` | `{0, 7, 0}` |
  | `Err(Nope)` | `{1, 0, 0}` |
  | `Err(Failed(1, 2))` | `{1, 1, 2}` |

  于是 `Err(p)` 不指名任何自己的 tag：它解析为紧随 Ok 之后的那个；`Err(e)` 则是对 E 型 sum 的**就地改写**（tag 越过 Ok 的那一段，载荷字别名外层槽）。代价：tag 映射表随变体表走，`emitMatchArm` 合成内 tag（`sub outer, 1`）并**直接复用外层 `pay0`/`pay1` 槽**（布局重合），`?`/报告面解码减 1。**被拒替代**：拒绝（`Err(<E>)` 保持边界——D8「Err 返回经三槽物化」整句落空，`bndErrPayload` 词删的理由与其含义脱钩）；四槽（同上）。
- **预算规则（单点强制）**：一个 sum 类型可入 ABI，当且仅当**每个变体的载荷 ≤ 2 字**且载荷**不得为 sum**（`abiSum`/`abiTuple` 作载荷直接拒）。规则在 `classType` 一处强制，于是 `fitAbi` / `tupleShapeOf` / `bindDefineParams` / `emitCallCore` 全都不需要预算检查。**残余边界**：用户自有 sum（非 Result）带 sum 载荷没有空闲 tag 位，仍超预算——披露。
- **fn 尾返回停点退役**：fnRetVal ~20 位 bnd 收敛——Err 返回与带载荷 sum 返回经三槽物化；记录返回经 ptr（M10b 行）。**同步类型返回判为不开（T9-3）**：它需要一个结果面还没有的被调方 operand 与调用方结果槽，且调用方会把值当一个标量读；两道守卫拒它——`fitAbi` 在分类处（调用点按同一答案拒，而非按一个它从没见过的 body 拒），以及 `fnRetOperand` 的 switch 没有该 family 的臂。**电池记录其中任何一道单独就成立**（`build-bnd-prim-return` 钉住）。
- **参数槽放宽**：sum 型/记录型/同步型参数入槽（今日 M15 运行面锚定「sum 型参数与同步类型参数不入可跑槽」——本条即该句重锚的兑现面，**文档同步归 T12**）；String 参数 (ptr, len) 双标量展开（M12 FFI 先例同形）。**T9-3 补**：sum 参数把**变体表从声明类型**里带出来（`fnParamAbi` 携带声明本身——它就是 `variantSite` 一向列在绑定标注旁边的那个权威），实参位把那份声明当期望读，写在实参处的构造因此走**调用臂**（`f(Some(1))` 与 `f(None)` 都在那里解析，后者从前没有一个括号可供抵达）；第 18 章的 family 是**一个指针**，由限定名抵达时经过的 import 识别，而不是仅由名字识别。
- **本面扩形（T4 裁定，2026-09-10；T9-4 落地）**：String 的**两字槽存储面**——`var r: String = s` 绑定与 String 名重赋值——亦归本任务（T4 只携带值形与 `let`，见 D3「发射集边界」）。该形今日 check 干净、test 面 exit 70，机械钉在 M15 的 `control-03/string-mutable-binding-decline` 上。**T9-4 已兑现**：该校准转 latent，test-malformed 载体迁到**泛型声明边界**（见实现记录 T9-4 与 D11 表）；落地面是 `strBinding` 的两枚槽字段与 `bindStringSlot` / `strSlotLoad` / `emitStrStore` 三个 helper，与标量面的 `bindScalarSlot` 同构，D13 的循环提升纪律按 `e.slot` 自动继承。**这一面的扇出比标量面宽**（`e.assigned` 的判据在 String 面共有**五个**入口——`emitLetBinding` 的字面量臂与 Ident 别名臂、`bindStringValue`、`bindResult` 的 ckStr 臂、`bindDefineParams` 的 abiStr 臂；`var` 面不在其中，它按定义就是被写的名字，故无条件占字——normalize 的读面有三处），突变电池首轮因此留下三枚两层齐活的缺口，补钉后闭合——记录在 T9-4 的补钉记录里，作为「每加一种存储形就要给每一臂补一钉」这条既有教训的第二次实证。

## D9 assertEqual Eq 域（Q4）

- 检查器域门放宽：8 整型 + Bool + String + **derive Eq 的记录/和式/元组**（递归域判：字段/载荷/元素同样入域）。
- 发射：结构比较——记录逐字段 icmp/递归、sum 先 tag 后载荷、元组逐位、String 走 `__we_str_eq`；不等时报失败（消息面 = 现断言失败渲染扩值形——首个不等位 + 两侧值渲染，基类型值转串复用 D3 `__we_str_of_*`；记录渲染 = 型名 + 首不等字段）。
- 非 Eq derive 型仍域外（E 码？**不**——域门是 bnd 边界非 E 码；B1a 后域外仅泛型面停 `bndGenericEqDomain`？**不设新词**——域外形并入 `bndGenericFns` 停点（Eq 泛型面是 stdlib 自有面同 M10b 判词理由），词表收编）。
- **落地对账（T10 收口，as-built）**：四枚 `0e18bbd` / `83dc2c0` / `26a7104` / `a1beee9`（实现记录 T10-1/2a/2c/3）。**七项裁定**（2026-09-11，用户）逐条落地：①**复合须自带 `derives Eq`**（记录/和式/新类型；无 clause 的 `record P { x: Int64 }` 仍是边界，拿到的是泛型词——裁定 3 的代价）；②**叶子限既有标量集**（8 整型 + Bool + String，与 `assertEqScalar` 逐字一致）——`derives Eq` 记录带 Float64/Rune 字段**规范合法而断言面边界**，绕行 `assertTrue(p.equals(q))`；③**残形并 `bndGenericFns` 不设新词**（上一条 bullet 的倾向在此兑现，`bndAssertEqDomain` 删除）；④**报文 = 路径 + 叶子值**；⑤**元组仅结构判**（无声明可载子句，随其元素；嵌套元组比域早一阶段停——`tupleShapeOf` 无 `abiTuple` 臂）；⑥**路径 = 位置索引段**（`Point.x` / `Point.inner.x` / `Shape.Rect.1` / 裸索引 `1`）；⑦**顶层 Bool/UInt64 一并对齐**（冻结面上一处**行为变更**，见下）。**逐字节报文契约**：顶层 Int64/String 走旧符号故既有字节钉死的模块**结构性**不变、`path == ""` 时整条 `at` 子句不出现、变体名**不引号**（`at Shape: got Circle, want Rect`）。**检查器面**：`inEqDomain` **平行新增**而非复用 `carriesType`（后者三处语义不合：无条件收基类型、不认元组、且是 E0823 的判据——改它等于改规范面）；递归的底是**叶集**，第二层专收「声明了 Eq 却触到断言不钉的叶子」的组分。**发射面**：`eqFace`/`eqWalk`（算子形，零 store、零槽预留）、记录逐字段、元组逐位、新类型**继承擦除**（零新增）、和式 tag 分派（唯一有控制流的一族：`eqsd` 报两名 `noreturn` + `unreachable`，`eqss` 按 tag 分派每变体一块，`eqsp` join；载荷读面**逐字镜像 `bindArmWord`**，`sumSlot.key` 是表的另一半）。**原文精确化**：本 D9 首条「字段/载荷/元素同样入域」读起来像「Float64 字段自动入域」，as-built 是**组分须落在叶集或自带子句**（元组随其元素），Float64/Rune **不入**。**披露**：元组内和式（检查器入域、聚合存三字，但 `tupleElem` 不带变体表——**聚合的边界而非域的边界**）；`Option`/`Result` 无声明可查（手写 impl 是 E0822）；载荷预算 2 词（`Both(Int64, String)` 3 词整个和被拒）与 Rune 载荷、裸变体名在值位置——三者都停在 **body 词**而非域词。**账**：conformance **782 → 796**、`internal/codegen/` 单测 **283 → 304**、`internal/typecheck/` **75 → 76**。

## D10 既有缺陷修复组（D13 披露纪律：全部红测试先行）

1. **float let 绑定丢域**（codegen.go:894 `case *ast.Binary` 丢弃 isFloat）：`let h = 1.5 + 0.5` 记 i64 域 → 下游按 i64 消费 double operand = 无效 IR。修：isFloat 透传入 scalarSlot。（**T3 已落地**：数值绑定统一走 `bindNumericValue`，见实现记录 T3「D10 划账」。）
2. **scope 值形尾值先于体求值**：`var n = 0; let r = scope { n = n + 1; n }` 得 Ok(0)——合法 IR 错值（tail 表达式在体语句前求值）。修：体先于尾（D2 块值同机制）。task 尾同形今日已拒绝（bnd），随 D1/D2 一并正确。（**T3 已落地**：`emitScope` 体先于尾 + `TestScopeValueBodyFirst` + 黄金 `run-scope-value-order`。）
3. **`&&`/`||` 非短路**：D2 分支形落地即修（单列红测试：RHS 含 panic 面/调用的求值序钉死）。（**T3 已落地**：`emitLogic` 分支形 + 黄金 `run-logic-short-circuit`。）
4. **窄整型溢出检查缺口**：陷阱只查 i64 域（i8 127+1 在 i64 域不溢出 → E0502 陷阱永不触发，ch7 逐宽 checked 语义未兑现）。修：窄整型算术后**逐宽界检查**（值域 icmp slt/sgt 对宽度界 → 越界 `__we_task_fail("integer overflow")`）。验证先行：红测试实证今日 i8 127+1 静默回绕。

**D10-4 落地对账（T11-3，as-built，2026-09-11）**——原文四处按实现订正，其余不动：

- **报文字面订正**：原文的 `"integer overflow"` **不是 as-built**。本轮 `<型名> <算子> overflow`（`Int8 add overflow` / `UInt32 mul overflow` / `Int8 neg overflow` / `Int8 div overflow` / `UInt32 shift overflow`，算子词表 `add|sub|mul|div|neg|shift`），命名源**宽度**而非寄存器宽度。这与上文 D2 的 2026-09-10 裁定一致——通用 `integer overflow` 在 T3 已退役——D10-4 当年那句是与自己前一节的抵触，以本节为准。
- **窄域是替换而非叠加**：`+ - *` 与一元 `-` 的窄路径**不走** i64 内建，改发平算术 + 宽度界守卫。三条理由：①窄值在 i64 域中精确，内建的判据在窄域**永不触发**（原文已说）；②更硬的一条——`UInt32` 乘法的真积可达 2^64，`smul.with.overflow.i64` 的**有符号**位会在一批合法乘积上误报，那是一个**替窄域做错判决**的判据；③叠加会留下「判据里有不参与判定的部分」。
- **三个守卫面分层**（原文只写了「逐宽界检查」一件事）：`<<` **保留全部三枚**（量 < 64、ashr 回环、宽度界——回环在窄域下仍活，`2^31u32 << 33u32` 的真积 2^64 在 i64 回绕成 0、宽度界放过 0、回环逮住）；`/` 的宽度界**只加在商上**（零除与寄存器 min/-1 是机器 UB 守卫，保留）；`%` 不需宽度界（`|r| < |除数|`）；`& | ^ >>` 皆不需（两位宽内值的按位结果仍在宽内，算术右移只丢低位）。`>>` 的**移位量**守卫保留且消息随宽度命名。
- **一并修正的两处**：`~` 在无符号窄型上原先给错值（`~200u8` 得 -201）——新增 `narrowMask`（UInt8→255、UInt16→65535、UInt32→4294967295；有符号窄型与两个全宽仍取 -1），因为 i64 补码只在操作数**符号扩展**时才是该宽度的补码。宽度**经发射流出**（`emitNumOperand` 的第 4 个返回值，镜像 `isFloat`），未另造平行分类塔；`scalarSlot`/`capSlot`/`topSlot`/`callResult`/`listElem`/`captureSet` 六面各带 `num`。
- **未兑现面（disclosure）**：`UInt64` 全宽算术仍有缺陷——`max + 1` 静默回绕成 0、合法精确和 2^63 被误报 `Int64 add overflow`。全宽无值域检查可作判据（和越过 2^64-1 会回绕成界内的值，内建的符号位不表示进位），故不在本任务修，已登记 roadmap follow-up #22。~~源码不可达面（值位置 if/match/块表达式绑定到 `let`、元组值绑定、`match` 表达式绑定、`?` 解包、任务体内 `let`）与前序任务同界，停在 M9b 边界。~~ **订正（T12 收口，2026-09-11 用户裁定）**：五面中四面不成立——前三面的边界是**读入 `String` 位**而非绑定，「任务体内 `let`」可跑，仅「`?` 解包」如原述；根因是 `valueKind` 无 `If`/`Match`/`BlockExpr` 臂，缺口已并进 D12 的 T13 对账项（详见实现记录 T11-3 披露②的订正与 T12）。

## D11 M15 基准套件重锚（docs/benchmarks.md:50 预披露兑现）

| 校准例 | 今日桶 | B1a 后 | 处置 |
| --- | --- | --- | --- |
| control-03 concatenation-boundary | boundary | **latent**（`s + "x"` 编译运行，值错测试抓） | 重锚桶断言 + traps.md 措辞 |
| control-03 string-equality-decline | test-malformed | **latent**（`if s == "mirror"` 编译运行，行为错测试抓） | 同上 |
| control-04 modulo-decline | test-malformed | **陷阱消亡**（`n % 2` 正确实现） | 删校准例 + traps.md 条目（按陷阱纪律：无捕获面的陷阱 = 任务设计缺陷，修清单披露） |
| control-04 mutate-let-param-decline | test-malformed | **陷阱消亡**（let 重赋值发射 = 行为正确的等价形） | 同上 |
| control-03 string-mutable-binding-decline（**T4 裁定期识别，本表写成时尚未列入**） | test-malformed | **latent**（`var r: String = s` 的两字槽发射，行为错测试抓） | 校准例改名 `string-mutable-binding-latent` 并转 `BucketLatent` + traps.md 第三条重写（**T9-4 已兑现**） |

- **T9-4 的载体迁移（test-malformed 桶）**：上表第三行转 latent 后，`control-03` **不再校准任何 test-malformed 形**，而黑盒电池的该桶需要一枚**check 干净 / test 停**的样本。本 build 拥有的**最后一个停词**是**泛型声明**：第 10 章认可它、check 干净，而 test 阶段的代码生成拒绝它，直到 B1b 单态化落地（`bndGenericFns`，B1b 的正式边界；黄金 `test-fn-generic-bnd` 即此形）。故载体改为 `control-03` 自己的 `echo` 声明改写成泛型形——桶因此锚在**工具链的边界**上，而不是锚在一枚语义突变上。候选面另有三枚（`List<String>` 形参、三载荷 sum、`var` 一个 sum/记录），皆因锚在未设计的洞或披露边界上而被弃。
- traps.md 括注更新：errpath-02（支配缺陷绕行注记 → 缺陷已修注记）、ctxpass-02 同面；run-face anchoring 段（docs/benchmarks.md 双语）按 D8 槽放宽与字符串/算子面兑现重写；LBR/FPCR 语义零触碰（方法论不动，只重锚运行面事实）。
- **落地对账（T12 收口，as-built）**：
  - **校准例计数订正为 63**（任务书与 D13 原写 64→62）。四枚 git 快照逐枚数：`7b7992c` = 64 → T2-a `2eae49c` = 63（删 `mutate-let-param`）→ T3 `05820e0` = 62（删 `modulo-decline`）→ T4 `b55d208` = **63**。T4 的 diff 是三行：两枚换桶**之外**新增 `control-03 string-mutable-binding-decline`（`BucketTestMalformed`）——它占据 `string-equality-decline` 转 latent 后空出的 test-malformed 位，即上表第五行「T4 裁定期识别、本表写成时尚未列入」的那一枚；「T4 只换桶不改枚数」是错的。T9-4 把该枚改名 `...-latent` 并转 `BucketLatent`，不改枚数。as-built 桶分布 Latent 26 + Rejected 37，无 Boundary 与 TestMalformed 表项。
  - **test-malformed 载体链**（黑盒电池该桶的活载体逐代迁移）：`control-04` modulo（亡于 T3）→ `control-03` 字符串相等（亡于 T4）→ `control-03` 可变 String 绑定（亡于 T9-4）→ **`control-03` 的 `echo` 泛型形**（`blackbox_test.go:35-51`）。
  - **支配缺陷的三枚复核探针**（T12 当日、今日构建）：`probe1` match 臂赋值先于 `scope timeout`、`probe2` 同一 var 的臂赋值跨 scope 两侧、`probe3` receive-with-match 先于 scope——check 0 / build 0 / run 0，值 `1007` / `1025` / `1041` 逐一正确。
  - **run-face anchoring 新段的每一主张皆有真二进制探针**（正向 6 枚 + 五面关死 5 枚 + 运行期规则 1 枚，逐条见实现记录 T12）。

## D12 边界词退役表 + conformance 对账 + roadmap 拆行

- **词表**：7 词删（D0 表）；`bndGenericFns` 括注改「B1b codegen-poly」。
- **T13 对账项：`valueKind` 的值位控制形缺口**（T12 收口登记，2026-09-11 用户裁定；**记账项，非待退役停点**）。`valueKind`（`codegen.go:9068-9118`）无 `*ast.If`/`*ast.Match`/`*ast.BlockExpr` 臂，落 `:9117 return skNone`，故值位控制形绑定到 `let` 后**数值面可用而 `String` 面停**（`emitHole` :9300 的 `skNone` 判据），且**污染下游**（`let j = k + 1; "${j}"` 亦停）。今日由 `bndMainBody` 代守。T13 的表里给它一行（今日词 = `bndMainBody`；订正后归属与是否补臂由 T13 定），并连带把**可达形状**落成黄金——`let n: Int8 = if a > 0 {2i8} else {1i8}` 两枚 + 窄和 + `if y == 5i8` 真机 run 0 打印 `five`（`narrow_test.go` 的注释今日只能引探针，无夹具可引）。**不是删词项**：这个词守的是一批真实停点，洞在分类塔的一处缺臂。
- **conformance 对账**（9 枚停点锚定黄金）：
  - 翻绿改锚：build-bnd-new-forms / build-bnd-m6b-main-body / build-ch10-method-call-boundary（→ 方法调用真发射，重锚 build 面）、build-bnd-err-payload、build-bnd-toplet——M9b「改名翻绿」先例：bnd- 名退役、按新事实面命名（如 build-loop-stmt / build-method-call / build-toplet-init / build-err-payload-eager）。
  - test-fn-body-bnd（fn 体新语句形 → test 绿重锚）、test-asserteq-domain-bnd + check-assertequal-domain-boundary（Eq 域 → 绿重锚）。
  - 非停点黄金零触碰（check-bnd-std-member/check-stdio-shadow-let = std 装载面、check-bnd-method-arity = follow-up #5、build-single-file/run-single-file/build-bnd-test-module/build-bnd-test-fns-first = follow-up #6、build-library = ch21）。
- **落地对账（T9 收口，as-built）**：T9 实际落点与上表的差：`build-bnd-err-payload` → **`build-err-payload-eager`**（T9-2，源不变、exit 0）；`run-err-payload-two-words` / `run-fused-err-return` / `run-sum-ctor-expression`（T9-2 新增）；`run-sum-param-match` / `run-prim-param` / `build-bnd-prim-return`（T9-3）；`run-string-reassign` / `run-string-mutable-binding` / `run-string-binding-faces` / `build-bnd-var-unannotated`（T9-4），其中 **`build-bnd-string-assign` 退役**（改名先例：`let s = "a"; s = "b"` 从 build 停点转为 run 正面，钉运行面而非构建面）。`run-string-binding-faces` 是**电池的产物**而非计划项：三枚突变在首轮两层齐活，暴露出值表达式 / 调用结果 / `.value` 解包三个生产面一枚钉都没有（详见实现记录 T9-4 的补钉记录）。另：**`test-fn-body-bnd` 已不存在**——T7-2（`1d389fc`）删除并取代为 `test-fn-body-for-list.json`（已绿），本表的「test-fn-body-bnd 重锚」是对账项而非待办。
- **新增黄金**：D1–D9 每拓宽面至少一正一负（负 = 域外停点词形 + panic 面）；总量级 ~40–60 枚（实现期定形，逐枚过真编译器）。
- **roadmap**：B1 行拆 B1a done + B1b pending（双语，归档期）；follow-up #16 划账（修复随 D1 落地，条目移除）。
- **落地对账（T13 收口，as-built）**：本表的三条预测各有差，逐条订正如下。
  - ① 词表不是「B1a 删 7 留 1」而是 **删 3 留 5**：删的是 `bndErrPayload`（T9）、`bndCallbackBody`（T6）、`bndAssertEqDomain`（T10），留的是 `bndMainBody`/`bndTopLets`/`bndTaskBody`/`bndGenericFns`/`bndFnBody` 五词（用户裁定 1，退役改挂 B1b）。3 + 5 = 8 与 D0 表的总数相符。
  - ② 9 枚停点锚定黄金**兑现 8 枚、未兑现 1 枚**——未兑现的是 `build-bnd-m6b-main-body`（源 `let v = f.fetch()?` 落在仍关死的 main 体 `?` 面，T12 探针 `trymain` 实证），见 D0 订正。
  - ③ 本表「翻绿改锚」列的名字只有 1 枚照写（`build-bnd-err-payload` → `build-err-payload-eager`）；其余按新事实面命名而非按本表拟名（`build-bnd-new-forms` 名系经 `build-bnd-string-assign` 落到 `build-bnd-var-unannotated`，`build-bnd-toplet` 保名只改期望）。
  - ④ 本表未列的**新增边界面**有 10 枚（T7-3/T9-3/T9-4 拓宽中新钉的诚实停点），逐枚入实现记录 T13 的对账表。
  - ⑤ 总量：804 枚 → **810 枚**（T13 新增 6 枚：1 枚 T13-1 + 5 枚 T13-2）；`run-*` 128 → **131**。

## D13 验证阶梯

1. 每拓宽面红→绿（单测/黄金先红后绿，红证据在完成记录）。
2. 真机电池：build+run 端到端逐面（字符串程序/记录方法程序/闭包捕获程序/List 迭代程序/顶层 let 程序/sum 载荷程序/Eq 断言程序 + panic 面负例）。
3. 全量：gofmt/vet/`go test ./...`（14 包）/validate --all --strict/docs_sync（33 对）/conformance 计数对账。
4. M15 重锚后全测试绿（traps_test 64→62 于 T3 + 重锚 2；**T12 收口订正为 64→63**——见 D11 落地对账块）。
5. 支配缺陷回归钉：errpath-02 形（match 臂赋值先于 scope）直发 IR 经 clang 编译零拒绝。
