# tasks — stdlib-and-gc

- [x] T1 测试先行 A：D12 黄金表落盘（~30 新增 + 0 改写），对当前构建运行必须先红
  来源：proposal 目标 1–7 / design D1 D2 D3 D4 D11 D12
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红（新增失败清单，其余 404 枚既有全绿零回归），red 证据与锁定/翻绿分类记于完成记录

  **完成记录（2026-09-07）**：27 案例落盘（生成器 `/tmp/m8-goldens/gen.py`，/tmp 惯例；锚点 marker 子串程序化定位——E1401/E1405 锚 `io.println` 起=接收者首 token，与 exprPos(Member)→Recv 既有规则吻合）。conformance 对账：总数 431（404 既有 + 27 新），**红 20 = 全部新枚**（comm 交叉核对红 ∖ 新 = 空——404 既有零回归），绿 411 = 404 既有 + 7 锁定枚。分布：装载绿 5（io-green/print-green/xm-green 三模块链/alias/unit 弃置）、E1401 面 3（纯 fn/差集 net/跨模块 b.say 传导）、E1304 面 3（bare 无 import/块内 let 遮蔽——锁定装载族）、E1302 std 形 3（std.json 未知/std 单段/单文件模式）、E1405 面 1（topLet 调 println）、E0501 面 1（println(42) 实参错型）、fn 值面 1（println 作 `fn(String) io -> ()` 实参绿）、hello 族 3（纯字面量/gc 记录构造+字段链/插值——check 面绿）、组合子锁定 6（map-collect/nested/take-skip/any-all-find/filter-count-reduce/effect-e1402）+ fold 1。
  **红因分类**：17 红为 exit 70 bndStdModules（一切含 `import std.*` 的源今日停装载边界）；**2 红为既有实现缺口的正确类型期红**——check-std-comb-fold：`fold(0, |acc, x| acc + x)` 裸闭包今日报 `operands of "+" are U and Int64`（M6a 方法泛型期望线程未把初始值位推断的 U 传入闭包参数定型）；check-std-comb-effect-e1402：组合子实参位今日报 E0501 而非 E1402（M7 织入位五处未覆盖内建方法的实参定型路——普通 fn 参数位 E1402 工作，组合子位绕过）。两缺口归 T5 修复（组合子真体任务的相邻面），黄金保持自然写法为契约。
  **黄金措辞修正（实现前，自有新增自由修正，披露）**：check-stdio-shadow-fn 原拟 `fn io()` 遮蔽后 io.println 报 E1304——落盘即红揭示设计错误：`import std.io` 引入名 `io` 与 `fn io()` 在模块单一名字空间本就 E0404 撞名（今日已正确报 `3:4 … "io" is already declared at line 1`）——改为锁定今日 E0404 行为（import 名占名字空间的钉）；块内 `let io = 1` 遮蔽（shadow-let）才是 E1304 的合法形，保持装载族红。
  **D12 对账**：预估 ~30 → 实落 27（shadow 双枚合一面、reduce 并入 filter-count 枚）。**0 改写**：`git status` 既有 testdata 零触碰。

- [x] T2 测试先行 B：typecheck 单测（std 装载/E1401 库函数面/E1302 std 形/遮蔽/组合子真体走查）与 codegen 单测（语句序列 IR 快照：hello/构造/根登记/边界 What 新词表）与 runtime 测试 harness（gc.c 分配-标记-清扫-存活断言）先写先红
  来源：proposal 目标 1–7 / design D2 D3 D4 D6 D7
  验证：`go test ./internal/typecheck/ ./internal/codegen/ ./runtime/` 红（新用例引用未定义符号/快照不匹配/harness 未实现），red 证据记于完成记录

  **完成记录（2026-09-07）**：三文件落盘——`internal/typecheck/m8_test.go`（7 函数：装载绿族 4 例 / E1302 std 形 4 例 / E1401·E1405·E0501 4 例 / 遮蔽 3 例[E0404 直钉 parser 层——名字空间查重在解析层先于装载] / 组合子真体 2 例 / 绿面锁定 5 例）；`internal/codegen/m8_test.go`（快照 5：hello / print+弃置 / 记录+字段链 / 嵌套构造[双 struct、位图位 1、外层先根化] / 记录+Err 路径[noreturn 前零 pop]；边界 7 例钉新 What 词表；import 擦除 2 例[别名同形 + 本地 import 维持 bndOtherFns 锁]）；`runtime/gc_test.go`（C harness：分配互异/头契约[size 位已写、map 位清零]/无根清扫计数/根化存活+经引用槽精确可达[父根化、叶仅经父可达、载荷完整]/解根双清/清扫后同尺寸再分配；io.c stdout 逐字节 `hi\nho`；钉版门同 cli checkClangVersion 族）。构造协议细则与快照终形随本记录写入 design D4（补记）。
  **红证据**：typecheck 5 FAIL（装载族 3 停 bndStdModules 边界 + shadow-let 停边界 + fold 裸闭包 E0501 U 未线程——T5 缺口；effect-e1402 例在其后未达）；codegen 8 FAIL（5 快照 + TestBoundaryWhats 三行既有行[词表更新] + TestM8BodyBoundaryWhats[首红因=import 未擦除停 bndOtherFns，T7 擦除后钉位生效]；TestM8LocalImportBoundary 绿=今日行为锁）；runtime 编译红（GCSource/IOSource 未定义）。conformance 25 红 = T1 的 20 新枚 + 本番 5 枚契约更新（4 What 换 + check-bnd-std-import 翻绿期望），`comm` 对账无意外枚；parse-clean 双枚今日仍绿（T3 装载落地时翻红，重导义务已记 proposal）。其余包全绿（parser/cli/diag 缓存绿，go build 全过，gofmt 干净）。
  **既有黄金契约更新披露（随 D4/D1 已批裁决，落盘即红）**：build-bnd-body / build-bnd-m6b-main-body / build-ch10-method-call-boundary / build-bnd-new-forms 的 stderr 换 D4 新 What 词表（源全在接受集外，exit 70 不变）；check-bnd-std-import 翻绿（exit 0，空 stderr）；codegen_test.go TestBoundaryWhats 三行 bndMainBody 行同步新词表。**E1302 std-bare 锚修正**（T1 黄金自有新增）：`import std` 例锚 1:1 → 1:8，对齐既有 E1302 锚法（`imp.PathLine/PathCol`——路径首 token，typecheck.go:1814 既有实现）；gen.py 同步。T3 需更新 cli TestLoadGraphStdBoundary（改钉装载绿面）并于首绿时重导 check-parse-clean 双枚（逐行审计披露）。

- [x] T3 装载层：D1 loadGraph std 分支（std.io 内建模块进图、后序 ingest、未知 std.* E1302 std 形、import std 单段形）、checker 内建模块符号路径
  来源：proposal 目标 1 / design D1
  验证：`go test ./internal/typecheck/ ./internal/cli/` 装载套件绿；既有套件零回归

  **完成记录（2026-09-07）**：`typecheck.StdModule(key)` 注册表为单权威（合成 `*ast.File`——真声明走同一 ingest/checkModule 全机器，无特权检查路径；Q2 裁决的落形），cli loadGraph 与 checker 两路同询。loadGraph std 分支：已知模块 `visit` 进图（后序、provided-before-importing），未知报 E1302 std 形（`StdModuleNotFound` 单函数供两侧同文）；`Check` 入口对自含文件预装载 std 模块（**不限模式**——单测 helper 的 `Check(Project)` 无装载器，装载图属 CLI 管线）。checkImport 的 std 位从 bndStdModules 改真分派；5298/5337 两处（基类型/collection 未锚定 stdlib 成员）维持边界——check-bnd-std-member 黄金绿。合成声明 `Ret` 留空（「无 -> 子句即无值」正形；显式 `UnitType` 触发空体 sameType 误报）。
  **黄金修正披露（T1 自有新增设计错误，实跑揭示，全部实现前修正）**：① check-stdio-shadow-let——E1304 设计错误：`io` 为已声明局部（Int64）时走成员路径（baseType 未锚定成员 → bnd，D10 不私拒），E1304 qualifier 形仅属接收者未声明形；改锁今日 bnd 行为（M8 后仍边界——Int64 的 stdlib 面在 M12+）。② check-e1401-println-xm——源漏 main 约定（E1305 先于一切），补 main 后锚移 10:5。③ check-e1405-println-toplet——尾部 remediation 自造文本，改注册表逐字（`initialize from a pure computation over constants`）。④ hello 族×3（check-stdio-hello/record/interp-green）——main 无效果段调 println 必 E1401；ch15:136 已批「main MAY carry an effect segment」，源改 `pub fn main() effect io ->`。gen.py 同步全部修正。design D1/D2 的遮蔽段随勘误（E1304 断言撤回，见 T2 记录的成员路径裁决）。
  **parse-clean 双枚重导（T2 义务兑现）**：装载落地后首错即停于单条 `demo/main.we:16:13 E1304 qualifier "svc"`（svc 未声明；码/文/锚/help 全为既有已批面；无 E0xxx 解析码——「parse-clean」原意图保留；io.println 在其后未达）。双枚（human/--json）逐字节从实跑捕获重导，exit 70→1。
  **既有测试更新披露**：typecheck_test TestNameResolution 的 `import std` 行 bnd→E1302 std 形（随 D1 单段裁决）；cli TestLoadGraphStdBoundary → TestLoadGraphStdLoading（装载绿面）+ TestLoadGraphStdUnknown（E1302 std 形 CLI 面）。

- [x] T4 std.io core 检查面：D2 符号表（println/print fn 型 + effect io + pub 位）、bndStdModules 收窄（三处边界改真装载或维持收窄形）、E1401 被调集接线（符号效果集）
  来源：proposal 目标 2 / design D2
  验证：`go test ./internal/typecheck/` std.io 套件绿

  **完成记录（2026-09-07）**：与 T3 同体落成——注册表即符号表（println/print：`fn(String) effect io -> ()`、pub 位经 `Pub: true`），合成 FnDecl 的 `EffectTags ["io"]` 经既有 ingest 流入 fnTags，E1401/E1405/E0501/fn 值位全走既有机器零新码：TestStdIoLoading/UnknownModule/CallEffect/Shadow 全绿，conformance 装载绿 5 + E1401×3 + E1405 + E0501 + fn-value + unit 全绿。bndStdModules 收窄于 T3 记录（1810 位真装载，5298/5337 维持）。

- [x] T5 组合子真实体：D3 十一枚空标记换真体（Go AST 构造、体只用品已批面）、默认体走查全绿核对（含效果面自证）
  来源：proposal 目标 4 / design D3
  验证：`go test ./internal/typecheck/` 组合子套件绿；conformance 组合子黄金翻绿零回归

  **完成记录（2026-09-07）**：**D3 内部矛盾揭出与终形**——D3 例体 `match self.next() { Some(v) => Some(f(v)), None => None }` 产出 `Option<U>`，与 map 签名返回 `Dyn<Iterator<U>>` 不符（草图从未过类型检查）。终形：6 枚急性组合子（fold/reduce/count/any/all/find）挂**签名一致真体**（递归定义：`self.fold(f(init, x), f)` / `self.fold(first, |acc, x| f(acc, x))` / `self.fold(0, |acc, _| acc + 1)` / `if f(x) { true } else { self.any(f) }` 族 / find 的 let-out 形），全部用品已批面（match/闭包/二元运算/let 注解/if 表达式/递归调用）；5 枚（map/filter/take/skip/collect）保留 `body: &ast.Block{}` **编译器内建标记**——Dyn 装箱与 List 构造面不在 M8 已批子集（D10 领域），规范 ch21 规则 9（组合子实现由编译器内建支持）使标记为诚实形。design.md D3 勘误段已补记。走查接线：`checkBuiltinCombinators()` 于每次 Check/CheckProject 入口运行（先于一切 ingest，空 syms——用户遮蔽永不触及 stdlib 体；开销微秒级；体坏则每次检查响亮报错），复用 checkMethodBody 全机器（ifaceScope + paramScopeOffset + self = 接口自身符号视图）——「同机受检，无特权」；9 枚带参方法设 paramNames（builtin 面无声明节点，注册表自述参数名）。
  **D13 真缺陷修复（走查揭出，披露）**：默认体走查揭出 methodCall 的 openRefs 要求「接收者自持的子句位」由实参定——错误（ch10 两个名字空间：方法自己的子句位才由实参欠定；接收者结构中的位已由接收者定型确定）。修复 = methodCall 加 recv 参数 + `recvHeld(recv)` 收集接收者类型结构中的 paramRef，openRefs 的 missing 过滤 held 位。递归 `self.fold(f(init,x), f)` 靠 unifyInto 未绑分支的 identity binding（m[pr.idx]=arg）自然通过——recvHeld 只需救 `self.next()`（无实参位）。既有黄金 check-e0827-method-undetermined（非泛型接口+具体接收者，held=∅）核对不受影响。
  **methodCall 两缺口修复（T1 记录的预期红兑现）**：(1) U 线程——`argTypes[i] = c.typeOf(a, substMap(ft.params[i], bindings))`（先前实参的 determination 先代入期望，fold 的 init 钉 U 后闭包参数读到具体型——check-std-comb-fold 翻绿的机制）；(2) E1402 织入——agreement 循环 disagree 时先 checkFnSlot 再 E0501（组合子实参位的纯槽闭包报 E1402 而非 E0501——check-std-comb-effect-e1402 翻绿的机制）。
  **E1402 黄金尾部修正（T1 自有新增，实现前修正，披露）**：check-std-comb-effect-e1402 的 stderr 尾部原为自造 "or perform less in the value"，重导为注册表权威文本（checkFnSlot——"...to cover the value's set, or supply a value that performs less"）；gen.py:158 已同步，重生成与 testdata 逐字节一致（post-regen conformance 复跑同红集）。
  **体惯用法成因（既有已批面，未改判）**：checkMatch 臂体 `typeOf(arm.Body, nil)` 无期望线程 + builtinCtorCall 无期望必 E0827（`Some(x)`/`None` 裸用不行）→ reduce/find 用 `let x: Option<T> = Some(...)` 注解惯用法；walkItems 对块尾 if/match 非单元臂的 E0605 是 HEAD 既有行为 → find 臂块尾 if 改 `let out = if ...; out` 形。
  **memberTypeRecv 重构（支撑性，无行为差）**：methodCall 需接收者类型，但 memberType 的前置（import qualifier 分派/qualifier 形 E1304/forEach 卫 E0816）不可绕过——memberType 改薄壳，`memberTypeRecv(x, asCall) (Type, Type)` 返回视图+接收者类型（接收者只定型一次，避免副作用双走）；callType member 分支改用之。
  **验证**：typecheck 全绿（TestCombinatorRealBodies 走查 6 真体 + TestCombinatorGreenFaces 绿面锁；parser 绿）；conformance 红 = 仅 4 枚 T7 build-bnd 词表金（本任务预期外集，T7 翻绿），组合子 7 枚全绿（fold/effect-e1402 本番翻绿，其余 5 枚装载期已绿、真体挂接零回归）；gofmt -l 空、go vet 过。

- [x] T6 GC 运行时：D6/D7 gc.c（对象头/根栈/标记清除/free-list/阈值触发/`__we_gc_collect`）、io.c（`__we_println`/`__we_print`）、startup 挂接、alloc.c 归并、runtime.go embed 扩、C 测试 harness 过
  来源：proposal 目标 5 / design D6 D7
  验证：`go test ./runtime/` 绿；clang 钉版门全链编译过

  **完成记录（2026-09-07）**：`runtime/c/gc.c` 落成——冻结头 `{map@0, size@8}`（载荷自 16），标记位 = map 字 bit 0（描述符 ≥8 对齐故该位空闲；两次收集之间不存在标记位——标记置位、清扫清位）；shadow-stack 根栈（realloc 增长数组）；精确标记 = 显式工作表深度优先（描述符位图：每 64 槽一 u64 字，bit i = 偏移 16+8i 是 gc 引用；desc 为 NULL 的块即叶/构造中块——零化载荷槽读作 NULL 安全跳过）；**清扫 = 重建式**（逐 chunk 走 used 前缀——每字节恰属一个块头，故步进可走；幸存者清标记位，其余全部进重建的 free list——「自由」只活在重建后的表里，无陈旧位）；free-list 经 map 槽单链（first-fit，余量 ≥16 分裂——余块补头）；bump 于 1 MiB chunk（malloc 载体，块 8 对齐）；阈值 1 MiB 于 `__we_alloc` **入口**触发（D4 构造协议保证入口是安全点——活集全在根栈，在飞块尚未刻出）；`__we_alloc` 返回零化块 + 写 size 位（构造代码后存 map 指针）；`__we_free` 读头直挂 free-list（ABI 冻结面，M8 生成码无 drop 路径不调它）。`io.c`：fwrite + 每调用 fflush（flush 时机可观测的钉死实现——println 空 payload 仍带换行）。startup.c 挂 `__we_gc_boot()`（幂等——零值全局即 boot 态）。**alloc.c 退役删除**（D7「迁往 gc.c」裁决——符号迁移，ABI 不动；M4 的 malloc 直通体不复活）。runtime.go embed：StartupSource/GCSource/IOSource；build.go 管线写 rt-startup/rt-gc/rt-io 三源 + 三对象编译 + 链接。
  **harness 路径修正（T2 自有新增，首个真跑揭出，披露）**：gc_test.go 的 compileAndRun 以相对名传 inputs 而 clang 在包目录跑——`no such file or directory: 'gc.c'`；改绝对路径。T2 先红时 undefined GCSource 挡在编译步，此 bug 未曾暴露。
  **验证**：`go test ./runtime/` 双 harness 绿（GC：分配互异/头契约[零化+size 位]/无根清扫计数=2/根化存活+经引用槽精确可达+载荷完整/解根双清/清扫后同尺寸再分配；io：stdout 逐字节 `hi\nho`）；cli 绿 + conformance build 黄金 5 枚 exit-0 全 PASS（钉版 clang 编 startup+gc+io 三对象并链接成功——含 startup 对 `__we_gc_boot` 的新链接需求）；全仓红集 = codegen 8（T7 预期）+ conformance 4（T7 build-bnd 词表金），runtime/typecheck/parser/cli/diag 全绿；gofmt/vet 清。

- [x] T7 codegen 扩面：D4/D5 main 体语句序列生成（let/表达式语句/单 return）、表达式子集（字面量/gc 构造/字段链/io 调用）、String 双字、struct 类型行、`__we_alloc` 分配 + 描述符、根 push/pop、bndMainBody What 新词表与超集停点、其余边界行维持
  来源：proposal 目标 3 6 / design D4 D5
  验证：`go test ./internal/codegen/` IR 快照全绿；既有 M4 快照零改写

  **完成记录（2026-09-07）**：codegen.go 重写为发射器结构（emitter 状态机：两张环境表 + 常量池 + 指令流 + fresh 计数）。**语句序列**：`let name = <string 字面量|gc 构造>`（env 记名，绑定自身零 IR——String 到使用时才 intern，未用的 let 零输出）、`io.println/print(e)` 表达式语句与 `let _ =` 弃置位、末尾单 return（Ok：全 pops 逆 push 序 + ret 0；Err：`__we_fail` + unreachable，**noreturn 路径零 pop**）。return 出现在尾位之外即停。**构造协议**（D4 逐条）：`__we_alloc(总字节)` → map store → **立即** root push（外层先根化，再求值字段值——嵌套分配绝不运行在未根化父对象存活期）→ 字段 store 按源序（String 双字两对 gep+store；嵌套构造先整体递归再存引用；标量 i64 store）。字段偏移从 16（头 {map@0,size@8}）；总字节 = 16 + 载荷。**描述符**：`@.map.Name = [n x i64]`——每 64 槽一字，bit i = 偏移 16+8i 是 gc 引用（String 双字不置位——D5）；零字段记录 `[0 x i64] zeroinitializer`。**struct 行**：`%struct.Name = type { 载荷型 }`（String→ptr,i64；引用→ptr；标量→i64；零字段 `{}`）；struct 行与 map 行按**声明序**渲染（仅被构造触及的记录——「使用集」），常量池按首现序（内容去重）。**declare 区固定序**：alloc→root_push→root_pop→println→print→fail（按使用过滤；快照三形全对齐）。**字段链**：根须 env 中 gc 绑定，中间跳 = 记录引用字段 gep+load，末跳 = String 双字 gep+load×2（嵌套快照 v7–v12 形）。**子集裁决细化（快照未钉处，D4 封闭清单的忠实读法）**：构造字段值 = String/Int/Bool 字面量 | 嵌套构造（**封闭**——ident/链不入字段位；链注记「供 println 实参」）；let 位 = String 字面量 | 构造（int/bool 字面量钉在构造实参位——TestBoundaryWhats「binding in body」行）；io 实参 = 字面量 | ident(String) | 链(String)；**io 限定词 = 任意名**（类型段已裁决合法性；erase 测试钉无 import 也发），但限定词为体内 let 名时是成员调用非 io——env 成员资格即判别（fn 字段撞名 println/print 的错发由此闭合）；标量字段限 Int64/UInt64/Bool（8 字节 i64 store），Int32 等窄型停边界；插值字面量任位停边界。**bndMainBody 扩词表**：`main bodies beyond let bindings, io calls, and a single Ok or Err return statement`；bndErrPayload/bndOtherFns/bndTopLets 三行原词汇维持；import 擦除 = 仅 Path[0]=="std" 段（本地 import 停 other-fns——TestM8LocalImportBoundary 锁）。
  **m8_test.go 三处 patch bug 修正（T2 自有新增，实现前修正，披露）**：control_flow/method_call/non-io 三用例以 `body := f.Items[N].(*ast.FnDecl).Body; body.Items = ...` 把 ast.Block **值拷贝**到局部再改——补丁为空操作（三例实跑的是干净 hello/greeter 模块，「got none」即由此）；改穿透指针直赋（其余四用例本就直赋故真改）。发射器本体无此缺陷。
  **验证**：`go test ./internal/codegen/` 全绿（M4 四测试 OkIR/ErrIR/EscapeDecoding/Deterministic 零改写通过 = M4 快照零改写达成；M8 五快照 + 边界 7 例 + import 擦除 3 例全绿）；conformance 4 枚 build-bnd 黄金随新词表翻绿——**431 枚全绿**（红名单 25 枚逐枚清零）。

- [x] T8 收尾：conformance 全绿（431 枚：399 既有零触碰 + 5 枚随批更新 + 27 新增）、`go test ./...` 清缓存全绿、gofmt/vet 清
  来源：proposal 目标 1–7
  验证：`go clean -testcache && go test ./...` 全绿；T1/T2 红名单（25 枚）逐枚翻绿零回归

  **完成记录（2026-09-07）**：`gofmt -l`（refr/ 外）空；`go vet ./...` 过；`go clean -testcache && go test ./...` 九包全绿（cli/codegen/conformance/diag/lex/parser/typecheck/version/runtime）。conformance 431 = 399 既有（含 5 枚随批更新：4 What 词表 + check-bnd-std-import 翻绿）+ 27 新增，零跳过零遮蔽。T1 红名单 20 枚 + T2 新增 5 枚契约更新全部翻绿；对账记录见各任务完成记录。

- [x] T9 全量验证与真机电池：既有 399 枚零触碰核对 + 5 枚更新枚按 T2 记录核对新期望；真机黑盒——hello 程序 build+run stdout 逐字、gc 记录 + 字段链输出、Err 路径回归、`we new` 骨架零改写、GC 存活探针（显式 collect 后存活对象字段值正确）、效果段零行为差 A/B（带 effect io 段与不带同 build 输出）；D10 不可达清单逐条实证记录
  来源：proposal 目标 7 / design D8 D10
  验证：输出与不可达实证记于完成记录

  **完成记录（2026-09-07）**：真机二进制 `go build ./cmd/we` 全电池过——① hello：`We`/exit 0（stdout 逐字）；② gc 记录 + 字段链：嵌套 Wrapper→Point 构造 + `w.point.name`/`w.label` 双链输出 `core`/`wrap`/exit 0；③ Err 回归：语句序列后 `return Err(Failed("boom"))` → stdout `hi`、stderr `error: Failed: boom`、exit 1（noreturn 路径零 pop 的运行面核证）；④ `we new demo` 骨架 M4 原文零改写 build+run exit 0 静默；⑤ **GC 存活探针**（C harness 直链 runtime/c/gc.c——harness 之外的新探针，覆盖 harness 未覆盖的**阈值触发路径**）：parent 根化 + leaf 经引用槽链接后 65536×64B≈4 MiB 未根化垃圾风暴（阈值触发 3+ 次中途回收）→ leaf 指针与载荷 0xABCD 完好、显式 collect 回收风暴尾期垃圾（≥1）、解根后 collect 恰 2；⑥ 效果段 A/B：同骨架带/不带 `effect io` 段 → build IR 逐字节相同（diff 空）+ 运行输出相同（段擦除的实证）；⑦ D10 七条逐条实证（见下）。
  **D10 实证**：1. `foreign "c" fn` → exit 70 `chapter 19 (ffi) forms are not implemented...`（bndFFI 面）；2. `"abc".len()` → exit 70 bndStdModules（String spec-anchored 族外成员维持收窄）；3. 插值字面量：check exit 0（ch1 已批语法）+ build exit 70 bndMainBody 新词表；4. 方法调用入 main 体（fn 字段记录 `h.run("x")`——检查面绿的最近可达形）：check exit 0 + build exit 70 bndMainBody 新词表（emit 层同钉于 TestM8BodyBoundaryWhats/method_call_on_a_record）；5. `import std.concurrent` → E1302 exit 1 std 形报文；6. GC 并发/分代/移动：无运行面可落——单线程直线 main 无并发踪迹，路由在 ADR-0003（不存在可实证的表面，此即实证）；7. `we test`/`vet`/`fmt` → 各 exit 70 子命令边界报文。
  **GC 计数缺陷修复（探针揭出，D13 披露）**：`__we_gc_collect` 的清扫计数把**已在 free-list 上的块再次计入**——重建式清扫把既有自由块重复计数（harness 未暴露：其自由块恰被同尺寸复用耗尽）。修复 = size 位 0 作自由块标记（size 8 对齐低 3 位空闲；push_free 置位、分裂余块置位、carve 后 memset+写 size 自然清位），清扫只计新死块。`blk_size` 读时掩 ~7；头部契约（size@8 == 参数）不受影响（carve 写净 size）。探针风暴后的 `collect()==2` 精确性由此成立；harness 与全仓测试复跑绿。
  **399+5 对账**：T8 清缓存全量 431 绿已含——399 既有零触碰（绿 = 零改写核证）+ 5 枚更新枚按 T2 记录新期望（4 What 词表 + check-bnd-std-import 翻绿）全过；本番 gc.c 修复后全仓再复跑仍全绿。

- [x] T10 验证阶梯 + 实现审查（welang-code-review 7 条，真二进制证据）+ ADR-0003 落盘（双语，docs_sync 配对）+ 归档（status → archived，归档目录带日期前缀）+ roadmap M8 → done 双语同步
  来源：研发流程 + design D6 D13 披露义务
  验证：gofmt -l 空、go build/vet 过、validate --all --strict 过、docs_sync 对数 +1（ADR 对）、git diff --check 干净；归档目录存在

  **完成记录（2026-09-07）**：验证阶梯全绿（gofmt -l 空、go build/vet 过、validate --all --strict 过、git diff --check 干净）。**实现审查 7 条通过**（记录全文在 proposal.md「实现审查记录」）——审查窗口强制新鲜复证：conformance `-count=1` 全绿 431 枚 + 真机电池五案重跑（hello `We`/exit 0、rec `core`/`wrap`/exit 0、err stdout `hi`+stderr `error: Failed: boom`+exit 1、new 骨架静默 exit 0、效果段 A/B build IR diff 空且运行一致）；红线复核确认 refr/ 零路径、诊断注册表零触碰（本变更零新增码）、新增 .go/.c 零 CJK；审查中修正一处自审断言（第 6 条「逐文件对齐」改为如实表述：cli/build.go 与 cli/modules_test.go 为影响范围表两行的实现载体、未逐名、已随 T3/T6 披露）。**ADR-0003 落盘**：`docs/decisions/ADR-0003-gc-strategy.md` + `.zh.md`（shadow-stack 根登记 + STW 精确标记清除 + 冻结 ABI + 三条演进路由：M9 并发决策门、LLVM stack map、移动式读屏障评估）；decisions README 索引双语各加一行。**roadmap M8 → done 2026-09-07**（双语；zh 行保持既有 `done <date>` 未翻译惯例——首次误写「2026-09-07 完成」即改）。**归档**：`openspec/changes/archive/2026-09-07-stdlib-and-gc/`（目录整体未跟踪故 plain mv 非 git mv；status → archived）；归档后 validate --all --strict 复验过（no active changes; registry clean）。docs_sync **30 → 31 对**（+1 恰为 ADR 对）。规范提升步骤为无操作——Q1 纯实现裁决，本变更无 spec 增量（openspec/specs/ 与 diagnostics.toml 全程零触碰）。

- [x] T11 记忆更新与提交：project-overview 增 M8 条目（下一里程碑指向）、MEMORY.md 索引行更新；英文提交信息（无署名 trailer；refr/ 不入提交）
  来源：研发流程 + 用户红线
  验证：`git log -1` 消息为英文且无 Co-Authored-By；`git show --stat` 无 refr/ 路径

  **完成记录（2026-09-07）**：记忆更新——project-overview 追加 M8 段（四裁决、五机制面、conformance 404→431、GC 缺陷披露指引、下一里程碑 M9 concurrency[GC 演进路由第一门]）+ MEMORY.md 索引行 M0–M7 → M0–M8。提交 `1395bb3`「Land M8 stdlib-and-gc: std.io core, combinator bodies, precise GC runtime」——59 文件 +3000/−167，英文全文、零署名 trailer（commit-msg 钩子通过）、零 refr/ 路径（pre-commit 钩子 + `git show --stat` grep 双证）。工作树提交后干净。
