# 设计 — stdlib-and-gc（M8）

表面裁决四问已定（2026-09-07，均采推荐）：Q1 纯实现 / Q2 编译器内建模块 / Q3 main 体垂直扩 / Q4 shadow-stack + STW 标记清除。各问的推荐理由与被拒面留档如下：

- **Q1 std.io core 的规范定位**：推荐 A 纯实现。ch15 R2 已把预导入之外的 std 内容定位为「ordinary module under `std.`」——库内容是模块的代码，不是语言表面；机制（`std.` 保留段、import 到达、pub 面、E1303/E1304、ch16 效果检查）全部已批，println 只是通过这些机制到达的一个函数。库 API 清单入规范将使每次库演进都成规范变更（违 P2 的糖不开章同族立场）；v0.8 §51 自己也说「具体绑定代码清单……不在本规范中逐一列出」。被拒面 B（小规范增量钉 std.io 最小面）：获得规范级契约，但开「库表面入规范」先例，后续每枚库函数都要规范变更。println 的契约由 conformance 黄金钉（检查面报文逐字 + 运行面 stdout 逐字）。
- **Q2 stdlib 的提供机制**：推荐 A 编译器内建模块。stdlib 模块在装载图里表现为一个编译器构造的模块对象（Go 登记符号表：名、fn 型、效果集、pub 位），与 M6a 的内建类型登记（collectionMembers/stringMembers）同族——「内建面」先例。被拒面 B（embed 真 .we 源文件走同一 parser/checker）：更贴近「标准库不享有特权语法通道」的长期方向，但 std.io 的底层体无法用已批表面书写（We 无 syscall 面；foreign 是 M12），M8 落它必然造出特权通道（体内直连 runtime 符号）——自举源码路线推迟到 M12 后（ch19 已留钩子），design D9 记路由。
- **Q3 codegen 扩面深度**：推荐 A main 体垂直扩（Hello World + GC 端到端）。理由：M0 裁决「薄垂直优先」的第三次应用；没有分配面，GC 设计只能空转（被拒面 B 维持 M4 接收集——ADR-0002 的 product-scope runtime 交付被推迟到不确定的未来）；被拒面 C 全函数生成——单变更不可验证（黄金与真机电池面爆炸），且 ch21 未承诺任何中间深度。M8 扩面精确钉为：main 体 = 语句序列（`let` 绑定 / 表达式语句 / 单 return Ok 或 Err），表达式子集 = String 字面量 / gc 记录构造 `T { f: expr }` / 字段读链 / `io.println`·`io.print` 调用 / `Ok(())` / `Err(V("literal"))`；一切超集（算术、控制流、闭包、其余调用）仍停 bndMainBody（What 词表扩）。
- **Q4 GC 初版范围**：推荐 A shadow-stack 根协议 + stop-the-world 精确标记清除 + ADR-0003。被拒面 B 仅设计文档无实现——「设计落地」无实证，分配 ABI 无消费者，M9 并发设计悬空；被拒面 C 分代/并发/写屏障——单线程接收集下无观测面，过早优化违 P4。ADR-0003 记录：根协议选型（shadow stack 先行，LLVM stack map 为演进路由）、算法（mark-sweep，非移动——移动式需读屏障或根重写，M9 前无收益）、ABI 冻结面（`__we_alloc`/`__we_free`/根对）。

裁决位以下按推荐线预写，裁决翻转时同步修订。

## D1 std 模块装载（loadGraph 分支）

ch15 R1：`std.` 首段永远命名编译器提供的模块、永不文件系统解析。M6b loadGraph 目前在 `import std.*` 先停 bndStdModules。M8 落地：

- `import std.io` → 装载图插入内建模块 `std.io`（D2 的符号表），随依赖后序 ingest（先于导入它的模块——初始化序 ch15 R5 的编译期计算对 std 模块同构成立）。
- 未知 `std.*`（如 `import std.json`）→ E1302 报文 std 形：`module not found — no standard-library module "std.json" exists in this build` + 期望路径句省略（无文件系统映射）；WithHelp 沿注册表 remediation。
- `import std`（单段）→ 维持现状（E1302 形或既有判——单段 std 不命名模块，报 std 形）。
- 内建模块无 We 源 AST——checker ingest 走「内建模块」路径：符号直接登记进模块桶（fn 型 + 效果集 + pub 位），不产生 AST 键（fnTags 等以符号自带效果集参与 M7 机器——`io.println` 调用位 E1401 的被调集来自符号表）。

## D2 std.io core 面

```
pub fn println(s: String) effect io -> ()
pub fn print(s: String)   effect io -> ()
```

- 两函数返回 `()`（ch8 严格弃置的豁免类型——表达式语句位合法）；效果集 = 内建裸键 `io`（M7 canonical 键机器的真用户）。
- 合格到达：`import std.io` 引入名 `io`，经 `io.println(...)` 调用（ch15 R3/R4 既有机器）；本地遮蔽合法（预导入外层作用域 P5——`fn io()` 遮蔽后 `io.println` 走 E1304 自然语义，零新规则）。
- 运行面（D7）：IR 直接 `call @__we_println(ptr, i64)`——编译器内建符号到 runtime 符号的映射（与 `__we_fail` 同族先例），不经 We 层调用序列化。

## D3 组合子真实体

- 11 枚空标记换真实体，Go 构造 AST（M6a 内建面同族）；体语言只用品已批面。例：`map` = `match self.next() { Some(v) => Some(f(v)), None => None }`（match/闭包参数调用/Option 构造——ch4/ch9/ch11/ch12 全已批已实现）。
- M6a 起默认体走查机器存在（M7 checkIfaceBodies：默认体调用计入接口方法自身声明集）——空标记体走查零内容、真实体走查真代码。效果面自证：体内 `f` 调用被调集 = 参数类型 tags（纯槽恒空集）→ 恒绿；`self.next()` 走 ifaceType 视图（next 无段）→ 恒绿。stdlib 的体与用户体同机受检，无特权。
- `fold`/`reduce`/`any`/`all`/`find` 体同构（match + 初始值线程）；`take`/`skip` 需计数状态（let 可变绑定？——ch6 `var` 局部绑定 + 赋值已批已实现，体用 `var n = n0` + `n = n - 1` 合法）。
- 体的黄金钉：`xs.iterator().map(f).collect()` 检查面全绿（签名/效果/继承机器全走）+ 每枚组合子的负例（E1402 效果闭包入纯槽——M6a 已有，此番真体走查不改变签名面）。
- **勘误（T5 实现落定，2026-09-07）**：上文例体 `map = match self.next() { Some(v) => Some(f(v)), None => None }` 从未过类型检查——该体产出 `Option<U>`，与 map 签名返回 `Dyn<Iterator<U>>` 不符（D3 草图的内部矛盾，实现时揭出）。终形：**11 枚中 6 枚急性组合子（fold/reduce/count/any/all/find）挂签名一致真体**（递归定义——`self.fold(f(init, x), f)` 等，全部用品已批面）；**5 枚（map/filter/take/skip/collect）保留 `body: &ast.Block{}` 编译器内建标记**——Dyn 装箱与 List 构造面在 M8 已批子集不存在（D10 领域），伪体即语义谎言。规范依据：refr/spec-0.8.md:909 规则 9「组合子实现通过编译器内建支持（不依赖用户 impl），所有组合子行为由规范固定」——标记为诚实形。上文「take/skip 需计数状态」句随此失效（二者属保留标记组）。
- **体的惯用法成因（既有已批面，非新裁决）**：checkMatch 臂体无期望线程（`typeOf(arm.Body, nil)`）且 builtinCtorCall 无期望必 E0827——`Some(x)`/`None` 裸用不行——reduce/find 用 `let x: Option<T> = Some(...)` 注解惯用法；walkItems 对块尾 if/match 非单元臂的 E0605 是 HEAD 既有行为——find 的臂块尾 if 改 `let out = if ...; out` 形。体在符号位 `1:1 (std combinators)`，走查经 checkBuiltinCombinators 于每次 Check 入口运行（空 syms——用户遮蔽永不触及 stdlib 体；体坏则每次检查响亮报错）。

## D4 codegen 扩面（main 体语句序列）

- **接受形**（超集停 bndMainBody，What 扩为 `main bodies beyond let bindings, io calls, and a single Ok or Err return statement`）：
  - 语句：`let name = expr`（模式绑定限单名——元组模式停边界）、`io.println(expr)` / `io.print(expr)` 表达式语句、末尾单 `return Ok(())` / `return Err(V("literal"))`。
  - 表达式：String 字面量（含插值？——**不含**：插值需运行时拼接构造，停边界）、Int/Bool 字面量（仅作构造实参位）、gc 记录构造 `Name { f: expr, ... }`（字段值递归限于本子集）、字段读链 `expr.f.g`（供 println 实参）、`Ok(())`/`Err(V("literal"))`。
  - `let _ = expr` 弃置绑定合法（ch8）。
- **IR 面**：
  - gc 记录 → `%struct.Name = type { ... }` 类型行 + `call ptr @__we_alloc(i64 size)` + 逐字段 `store`；记录值 = 指针。
  - byval 记录构造停边界（无堆分配语义，M11+）；newtype/sum 构造停边界。
  - String 值 = `{ ptr, i64 }` 双字（D5）；字面量 = 全局常量对（`@.sN = constant [n x i8] c"..."` + 常量 struct 或两操作数直传）。
  - `io.println(e)` → 求值 e 到 (ptr,len) → `call void @__we_println(ptr, i64)`。
  - **根登记**：每个持有 gc 记录的 `let` 生存期 → `call void @__we_root_push(ptr)` 入口 / `__we_root_pop()` 出口（shadow stack；main 体直线控制流下 push/pop 严格嵌套）。
  - 函数体 IR 保持直线（无控制流生成——接受形本就无控制流）。
- **确定性**：Emit 纯函数不变（无路径无时钟）——语句序 → 指令序，字面量编号按出现序。
- **黄金与既有测试的碰撞面**（T2 对账落定，2026-09-07）：What 扩词表更新 4 枚既有 build 黄金的 stderr（源程序均仍在接受集外，exit 70 不变）；D1 装载使 check-bnd-std-import 翻绿（exit 0）、使 cli 的 TestLoadGraphStdBoundary 改钉装载绿面（T3 时更新）、使 check-parse-clean 双枚翻为诊断集（期望于首绿时逐行审计重导，义务记 proposal 涉触码盘点）；codegen_test.go 三行 bndMainBody 边界行随新词表更新（T2 已落）。全部为已批行为面的契约先行，落盘即红。
- **构造协议细则**（T2 快照钉死，补 D4 的 IR 面）：每次 gc 构造 = `__we_alloc(总字节数)` → `store map` → **立即** `__we_root_push`（嵌套构造的子分配不得运行在未根化的父对象存活期间——外层先根化，再求值字段值）→ 逐字段 store（字段值按源序求值，嵌套构造递归同协议）；`__we_alloc` 返回**清零**块（分配器写 size@8，map 位留零——半填充对象标记安全）；全部 pop 于唯一 ret 前按 push 逆序发射；Err 路径 `__we_fail` noreturn，其前不发射 pop。字段偏移从头 16 起（头 {map@0, size@8}）；`@.map.Name` = 引用槽位图常量（每 64 槽一 i64 字，槽 i = 偏移 16+8i；M8 字段型里仅嵌套 gc 记录指针置位，String 双字不置位——D5）。

## D5 String 运行时表示

- We 值语义 String = 不可变字节序列 → IR 值 `{ ptr, i64 }`（数据指针 + 字节长度）；按值传递（双字拷贝，ch8 基类型 byval）。
- 字面量：全局常量（M4 `@.err` 先例一般化）；无运行时 String 构造面（charAt/byteSlice 等 bnd 维持）——插值字面量停边界即为该立场的边界表达。
- gc 记录字段持 String = 存双字（String 自身的字节缓冲在 M8 全来自常量，非 GC 对象；运行时构造的 String 归后续里程碑，届时字节缓冲入 GC 堆——ADR-0003 记）。

## D6 GC 设计（ADR-0003 载体）

- **根协议**：shadow stack——编译器在 gc 引用的生存期边界生成 `__we_root_push(ptr)`/`__we_root_pop()`；runtime 维护根栈（线程局部，M8 单线程即全局）。precise 的依据：根集由编译器生成（每个根的类型确定——push 的是 GC 对象指针，无非根混入）；非精确面（保守栈扫描）不存在。
- **算法**：stop-the-world 精确标记清除。对象头 `{ map, size }`（map = 指向该类型布局描述符的指针——描述符逐字段标注引用槽；size = 含头总字节数）。标记：从根栈出发沿 map 描述的引用槽深度优先标灰；清扫：惰性 free-list（分配走 bump，清扫回收挂 free-list，`__we_alloc` 先查 free-list）。
- **描述符传递终形（审查 F1 收敛，不留两形）**：`__we_alloc(i64) ptr` 签名不变（M4「换 body 不换 ABI」的字面），返回已含对象头空间的裸块——分配器写头的 size 位（free-list 复用块按整块或分裂后的终值写）；编译器生成的构造代码在分配后 `store` 该类型的 map 指针进对象头 map 位。`__we_free(ptr)` 读头挂 free-list。描述符本身是编译器为每个 gc 记录类型生成的 IR 侧常量（字段引用位图——M8 的 gc 记录字段类型限于 String 双字与嵌套 gc 记录指针，位图即够）。
- **触发**：分配字节数超阈值（编译期常量起步，如 1 MiB）时在 `__we_alloc` 内同步触发；另提供 `__we_gc_collect()` 显式符号（测试与探针用，非 We 表面）。
- **ABI 冻结面**（M4 承诺兑现）：`__we_alloc(i64) ptr` / `__we_free(ptr)` 签名不变；新增符号（根对、collect、布局描述符约定）记入 ADR。
- **演进路由**（记 ADR-0003，不在 M8 落地）：M9 并发 → 分代 + 写屏障或并发标记的决策门；LLVM stack map（`gc "..."` 属性）替代 shadow stack 的根开销优化路由；移动式 GC 的读屏障代价评估。

## D7 runtime 面（runtime/c/）

- `gc.c`（新）：根栈、堆（bump + free-list）、对象头、标记清扫、`__we_alloc`/`__we_free` body 替换（malloc 直通退役）、`__we_gc_collect`、（可选）`__we_gc_stats` 供探针。
- `io.c`（新）：`__we_println(ptr, i64)` / `__we_print(ptr, i64)`——fwrite(stdout) + 换行差异；无缓冲语义钉死（每次调用即写——flush 时机可观测性优先，M9 前无并发）。
- `startup.c`：`__we_gc_boot()` 挂入启动序（main 前，ch15 R5 init 序的运行时半边）。
- `alloc.c`：符号迁往 gc.c 或保留薄壳（ABI 位不动，实现位归并——落实现时定）。
- C 测试 harness：`runtime/c/` 侧测试经 Go 测试编译执行（`go test ./runtime/`——clang 编 gc.c + 测试 main.c 断言分配/标记/清扫/存活），进验证阶梯。

## D8 效果擦除维持（M7 Q2 延伸）

`io.println` 调用生成零效果面——效果是检查期纪律（M7 D10），IR 与 runtime 符号无效果概念。用户程序 build 行为与效果段有无零相关（M7 电池 4 结论延伸到「带真实库调用」的程序）。

## D9 stdlib 自举路由（披露）

「标准库不享有特权语法通道」（v0.8 §51 立场，方向性采纳）：长期 stdlib 应是真 We 源码经 foreign 自举（ch19:207 场景）。M8 的内建模块是**过渡形态**——println 经编译器符号映射直连 runtime（特权位），披露于此并记 ADR-0003 的姊妹段（或 roadmap follow-up）：M12 FFI 落地后 std.io 迁真源码 + foreign 底层，届时内建模块退役。此路由不改变 M8 的任何检查面契约（黄金钉的是到达与效果，不是提供机制）。

## D10 不可达清单（诚实边界枚举）

1. `foreign` 顶层项停 bndFFI（M12）——std.io 文件函数的 foreign 声明形在 M8 不可达。
2. String 运行时构造方法（charAt/byteSlice/…）停 bndStdModules 收窄后的边界（`String` 的 spec-anchored 家族维持，其余维持停）。
3. 插值字面量入 build：Emit 对含 `${` 的字面量停 bndMainBody（检查面合法——插值是 ch1 已批语法；运行面边界）。
4. 组合子调用入 build：接受表达式子集不含方法调用——`xs.map(f)` 在 build 停 bndMainBody（检查面绿）。
5. `std.concurrent`/`std.test`/其余 std 模块：装载分支对未知 std.* 报 E1302——这些模块名在 M8 的 build 里同样不存在。
6. GC 并发/分代/移动：无面可落（单线程直线 main）；ADR-0003 路由。
7. `we test`/`vet`/`fmt`：M10/M11 子命令边界维持。

## D11 今日行为基线（翻绿面预测）

- `import std.io` → 今日 bndStdModules（exit 70）→ M8 绿（装载 + 符号 + E1401 机器全走）。
- `io.println("x")` 于纯 fn → 今日 bndStdModules → M8 E1401 @ 调用位（M7 机器首个真实库函数用户）。
- `xs.map(|x| x).collect()` 检查 → 今日绿（空体标记走查零内容）→ M8 绿（真体走查——行为不变、覆盖变真）。
- hello 程序 `build` → 今日 bndMainBody → M8 产工件；`run` → stdout 文本 + exit 0。
- `we new` 骨架程序（单 Ok return）build/run 行为零改写（既有黄金不动为证）。

## D12 黄金表预估（~30 枚）

- std.io 装载绿 3：`import std.io` + println 于 effect io fn 绿；print 同；多模块经 std.io 中转。
- E1401 面 4：纯 fn 调 println；裸名（无 import）调 io.println → E1304；跨模块 pub 面；效果差集报文（io 单标签）。
- E1302 std 形 2：`import std.json`；`import std`。
- 遮蔽 2：`fn io()` 遮蔽后调用；本地 `effect io` 声明（E1403 维持）。
- 组合子真体面 6：全链 map/filter/collect 绿；take/skip 计数体绿；fold/reduce/any/all/find 绿；闭包实参 E1402 维持（既有面回归钉）；效果闭包入纯槽（既有）；嵌套组合子绿。
- build 面（真机电池 + codegen 单测双钉）：hello 单 println；gc 记录 + 字段链 println；let 序列 + Ok；Err 路径回归（既有）；插值停边界；多 let 根登记。
- 边界回归 4：bndMainBody 新词表；bndOtherFns 维持；byval 构造停边界；方法调用停边界。

## D13 披露义务

- Q1 裁决立场（库表面=实现层）写入 proposal 与本设计——若未来要规范级库契约，走独立规范变更。
- 内建模块的特权位（println 直连 runtime）是过渡形态（D9）——不是长期立场。
- GC 阈值/清扫策略是实现参数（非规范承诺——ADR-0002 立场），黄金不得钉具体阈值行为（探针只钉存活正确性与输出，不钉「何时回收」）。
- 组合子真体若走查揭出检查器缺陷：修复披露（M5「测试先行抓真缺陷」先例），不得静默改体绕过。
