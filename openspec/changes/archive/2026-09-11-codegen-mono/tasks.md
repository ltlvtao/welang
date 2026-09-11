# tasks — codegen-mono（B1a）

> 红先行注：T2–T11 各面首 checkbox 对应形今日全部停 bnd 边界（check 绿、build 70）——首测试即自然红，红证据在完成记录；T1/T3/T4/T6/T11 单列的红行是缺陷钉（今日静默错值/无效 IR 形，非 bnd 停点）。

## T1 env 作用域化 + follow-up #16 根治（测试先行）

- [x] 红：errpath-02 形真机钉——match 臂赋值先于 scope 块的程序 build 被 clang 拒（`Instruction does not dominate all uses`）记录在案；作用域化后同源程序编译链接运行零拒绝
  来源：design D1（follow-up #16 根治机制）
  验证：红输出与绿输出对照在完成记录（proposal.md 实现记录 T1——红：三形真机 exit 1 clang 诊断 / 静默错值 notfive；绿：同源三形 exit 0 输出 ok/five/five）；`go test ./internal/codegen/` 作用域套件绿（envframe_test.go 5 例红→绿）
- [x] 发射环境块作用域栈（入块 push/出块 pop；match 每臂一帧；capture 解析走作用域栈，深度语义镜像检查器 lookupLocalDepth）
  来源：design D1
  验证：`go test ./internal/codegen/` 全绿；既有 conformance 黄金零回归（go test ./... 14 包全绿；701 既有黄金零触碰）
- [x] conformance 新黄金：嵌套块同名绑定/match 臂绑定 join 后不可读形（值正确性钉）+ 支配形回归钉
  来源：design D12/D13
  验证：`go test ./internal/conformance/` 全绿（新增计数对账：701→704——run-frame-arm-payload-shadow / run-frame-arm-let-shadow / run-frame-scope-task-capture，逐枚先真机红→绿定形）

## T2 语句集完备（三锚同集）

- 元组模式绑定与 scope resource **移入 T5**：实现期前置核验证明两面都停在 T5 的机器上，不是本任务可落的面——元组模式必解构一个元组**值**（聚合布局/多 retval 返回属 T5-D4/D8；实证 `fn pair() -> (Int64, Int64)` check 0 / build 70 `bndFnBody`），scope resource 的释放必须调用 `impl Releasable for T { fn release }` 的 **fn 体**，而今日 `ImplDecl` 在 codegen 显式 erase（`case *ast.InterfaceDecl, *ast.ImplDecl: continue`，注释即「neither emits IR」）——release 符号不存在于发射产物里（实证 foreign 背负的 scope resource 程序 check 0 / build 70 `bndMainBody`）。T5 的既有 checkbox（方法表派发 / 元组 value 聚合）本已覆盖两面机器，故并入 T5 执行，T2 至此收口。证据与处置见 proposal.md 实现记录 T2-c。
  （同组其余面已全落：loop/break/continue 与任意深度 return、task 体 return 见实现记录 T2；裸块与赋值面见 T2-a；for-in 见上方 checkbox 与实现记录 T2-b）
- [x] for-in 语句：Range 源直发计数循环（非物化数组——语义等价论证见实现记录 T2-b）；String 源归 T4、List 源归 T7（各自 checkbox 已列）
  来源：design D1/D6（源归属在实现期重划，见实现记录 T2-b）
  验证：`go test ./internal/codegen/` for 套件 7 例绿（for_test.go）；真机电池 main/fn/task 三体上下文全绿（计数/continue/break/循环变量重赋值/边界只读一次/start≥end 零元素/嵌套/通配头，实现记录 T2-b 表）；黄金 4 枚入账（红→绿：HEAD 树 run 70 bndMainBody（fn 形为 bndFnBody）；B1a 树同源 run exit 0 输出 count-ok/cont-ok/break-ok/fresh-ok/bound-ok；负例 build-bnd-for-string 两树同 70）；`go test ./...` 14 包全绿（713 黄金）
- [x] 赋值面放宽：Assign 删 `!slot.isVar → bnd` 分支；let 标量被赋值名改 alloca 槽（纯读名保持 SSA）
  来源：design D1（check/build 面一致性）
  验证：let 重赋值程序 build+run 行为正确（红→绿：HEAD 树 check 0 / build 70 bndMainBody；B1a 树 run exit 0，输出 let-ok/alias-ok）；既有黄金零回归（`go test ./...` 14 包全绿，6 处 byte-exact IR 快照零漂移）；证据在 proposal.md 实现记录 T2-a
- 三锚词表退役（bndMainBody/bndTaskBody/bndFnBody 删除）**移入 T13**：实现期发现 `e.bnd()` 按体上下文选词（ctxMain/ctxTask/ctxFn），语句集之外的表达式缺口（如 `emitNumExpr` 无 Call 臂——值位调用）与语句缺口同走三锚词，语句集完成即删锚会让这些缺口被 bndGenericFns 冒名；design D0 表本就把退役绑在 D1+D2——T2 收窄为 for-in/scope resource/元组模式。处置披露见 proposal.md 实现记录 T2-a「发现」。

## T3 表达式值形完备（单态）

- [x] 红：if/match/块值表达式（值位）、一元 `!`/`-`/`~`、`/` 与 `%`、rune 值形的单测先红
  来源：design D2
  验证：红证据在完成记录；`go test ./internal/codegen/` 表达式套件绿
- [x] 结果 alloca 模式（各臂算值 store → join load；unit 免 alloca；块值体先于尾——缺陷②修复同机制）
  来源：design D2/D10-2
  验证：scope 值形红测试（Ok(0) 错值）转绿 Ok(1)
- [x] `/`/`%` 零除 panic 面 + 浮点 fdiv；rune i64 域
  来源：design D2
  验证：零除断言程序 run exit 非零 + panic 消息面；黄金钉
- [x] 值位调用（operand 位置的用户函数调用——算术/比较/逻辑算子的操作数及任意嵌套）：`emitNumExpr` 增 `*ast.Call` 臂
  来源：design D2（implement 期发现，见 proposal.md 实现记录 T2-a「发现」①）
  验证：红→绿——今日 `if bump(5) == 15` 与 `let x = bump(5) + 1` check 0 / build 70 bnd（HEAD 与 T2-a 树同报，非回归）；修后同形 run exit 0 值正确；黄金 run-call-in-operand 钉
- [x] 位运算二元族 `& | ^ << >>`（2026-09-10 裁定并入本变更）：`& | ^` 全值域直发无条件；`<<`/`>>` 守移位量域（`icmp ult i64 b, 64` ——无符号比较一并覆盖负量→`Int64 shift overflow` 陷阱），`<<` 另守丢位（`ashr` 移回不还原即陷阱：LLVM 的 `shl` 静默丢弃溢出位，正是 ch7 禁止的静默回绕）；`>>` 取算术移位（有符号操作数的保号读法）
  来源：design D2（2026-09-10 裁定；越界语义登记 follow-up #20）
  验证：红→绿——pre-位运算树（`/tmp/we-t3`）五枚算子逐枚真机 check 0 / build 70 `bndMainBody`；修后 run exit 0 六值全对（`and/or/xor/shl/shr/ashr`）+ 三形黄金（run-bitwise-ops / run-shift-amount-panic / run-shift-value-panic，两枚 panic run exit 1 消息面精确）；突变复验（撤 dispatch → 三测试转红，边界报文与真机同形）
- [x] 溢出陷阱消息改指名形（2026-09-10 裁定本变更内改，兑现 ch14「指名操作」）：`Int64 add/sub/mul/div/neg overflow` 各带己文，通用 `integer overflow` 退役；窄整型逐宽命名留 T11
  来源：design D2（2026-09-10 裁定；gap 系 M5 起既有）
  验证：旧断言与黄金同步改写（`expr_test.go` 文本断言、run-div-overflow-panic 的 stderr）；真机三形消息面精确——`max + k` → `Int64 add overflow`、`min / -1` → `Int64 div overflow`、`-min` → `Int64 neg overflow`；新黄金 run-add-overflow-panic 钉

## T4 字符串全链

- [x] runtime/c/str.c 新符号族：concat/eq/bytelen/runecount/charat/byteslice/of_* 值转串族（OOB panic 面）
  来源：design D3（**已落地**：十枚导出，`bytelen` 经 D13 披露改为不实现——`byteLength` 折叠为长度操作数；harness 落 `runtime/str_test.go` 循仓库 per-family 惯例，非 checkbox 字面的 `runtime_test.go` 扩容；见实现记录 T4 实现 1 与披露⑤⑪）
  验证：C harness 单测（`runtime/str_test.go` 双形：happy path 逐条 CHECK + panic 面 tag=1 指名消息）+ 真机端到端（实现记录 T4 真机面四探针）
- [x] 发射面：String 绑定 {ptr operand, len operand} 运行时值形；拼接/相等/方法族/插值（基类型+String 域）
  来源：design D3（**已落地**：`emitStringExpr` 五臂 + `concatStr`/`strCompare`/`emitStrMember`/`emitInterp` + `valueKind` 分类域；洞 AST 归 parser 面（裁定③）；真机首跑揭出三缺陷，见实现记录 T4 披露①②③）
  验证：字符串程序 build+run（拼接/相等分支/插值各一）+ 黄金钉（run-string-concat / run-string-equality / run-string-interp，三枚首跑即绿；**注意**：三枚的功能正确性由真机首跑与 13 枚突变复验共同背书，非由测试先行背书——见实现记录 T4）
- [x] for-in String 源（自 T2 移入）：跑马灯式 rune 游走（runecount/charat 复用），`build-bnd-for-string` 黄金翻绿重锚；`TestForNonRangeBnd` 随之改锚 List 源
  来源：design D3/D6（源归属在 T2-b 实现期重划，见实现记录 T2-b）
  验证：for-over-String 程序 build+run 值钉（元素序列 + 空串零元素）**+ 黄金退役改锚**（实现期改判：边界黄金翻绿只剩 build 0 弱断言，改退役 `build-bnd-for-string` 并新锚 run 面 `run-for-string`——逐码点输出，严格更强；见实现记录 T4 披露⑩）；`TestForNonRangeBnd` 已改锚 List 源，`TestForStringSource*` 四枚为 T4-3 边界红转绿

## T5 记录/元组/newtype 值面 + 方法表

- [x] 标量字段读（getelementptr+load 统一成员读）+ `with &` 拷贝构造（__we_rec_copy）+ value record 整值拷贝
  来源：design D4
  验证：记录读写/更新程序 build+run；`go test ./internal/codegen/` 绿
  落点：`record_test.go` 八枚（标量读/浮点读/更新拷基/值绑定拷/gc 绑定共享/传参拷/嵌套深拷/构造收表达式）；黄金 `run-record-read-update`（`1 2 3 2 p` / `13 2 p`——更新改写 x 而基 p 不动）。**偏离披露（D13）**：design D4 点名 `__we_rec_copy` 运行时新面，但根推送按 body 记账（`e.pushes` 在每个 body 出口弹），callee 的 push 无法由 caller 平衡——故拷贝在站点内联展开（`emitRecCopy`），其 push 记在发起的 body 上。面是真、拼写是发射器的；理由写在 `record_test.go` 文件头。
- [x] 方法表静态派发：(型名, 方法名) → fn 表（inherent + interface 具体实现 + 默认方法体）；mut self 字段写；`self.field=` 语句发射
  来源：design D4
  验证：方法调用程序 build+run（含 mut self 计数器形）；build-ch10-method-call-boundary 黄金翻绿重锚
  落点：`method_test.go` 七枚（inherent 派发 / self.field= 写 / gc 计数形原地 / 隐式尾返回 / 方法调用归类为 String / interface impl / 默认体按头实例化）；黄金 `build-ch10-method-call-boundary` 翻绿（exit 70→0）与 `run-method-dispatch`（`1 2` 计数器两跳、`hi!` 默认体派发回头型自身方法）。符号 = `@<module>.<Head>.<name>`（头型段使方法与同名裸 fn 不撞）；接收者走 define 首参 `ptr %self`，调用点直呼符号不走槽（派发静态，槽是 mock 的面）；interface 自身零符号。**偏离披露（D13，两处）**：①chapter 6「体块值即函数隐式返回」（尾表达式项）此前不在任何 checkbox 上——ch10 三枚黄金的方法体全用该形，故随本任务落地（`emitFnDefine` 的尾项识别加 `*ast.ExprStmt` 臂）；main 体的同形仍停（归 T9「fn 尾返回停点退役」）。②`callStrKind` 此前不认方法表，致 String 返回的方法调用无法进 String 域（`self.greet() + "!"` 停）——补方法表臂。
- [x] newtype 零擦除恒等 + 元组 value 聚合 {f0,...}（模式解构/传参展开）
  来源：design D4
  验证：newtype 构造-解包程序 + 元组解构程序 build+run；黄金钉
  落点：`tuple_test.go` 七枚（newtype 构造恒等 / 参数即底层 / String 底层恒等；元组构造聚合 / 模式解构逐位 load / 返回即多值 / 传参逐字段展开）；黄金 `run-newtype-erasure`（`42 id 41`——构造、别名、`.value`、参数与返回都不多一句 IR）与 `run-tuple-value`（`7 seven` / `7 30` / `11`——解构取回、传参展开、返回重组）。newtype 的零擦除落在 `classType` 顶部的 `derefNewtype`（参数位、返回位、元组元素位走同一个分类器）；`.value` 解包靠 `ntEnv`——名字的静态类型是新包体这件事，值里没有任何东西说得出（那正是擦除），只能由环境记住（参数绑定、别名、构造结果三处登记）。元组值 = 栈聚合句柄（`tupEnv`）：构造逐元素存偏移、解构逐位 load、跨边界逐字展开、对岸 load/store 重建。
  **偏离披露（D13，三处）**：①计划里的「`p.0` 位置成员读」在本语言并不存在——parser 对 `p.0` 报 E0105「member names are identifiers」，章 8「元组模式与解构」把**模式**定为唯一的元素读者（design D4 亦只写「模式解构/传参展开」）。首版三枚单测与探针按 `p.0` 写成，属凭空造面；已删除该分支（`emitMemberValue`/`memberKind` 的元组成员臂）并按真实源面重锚三枚单测（构造的可观测量改走解构）。②`fnRetOperand` 的 abiStr 臂只认 Ident/Literal/Binary/Call，`return n.value`（String 底层 newtype 的解包返回）停在 bndFn——真机电池揭出，补 `*ast.Member` 臂。③突变复验揭出单测盲区：`TestTupleConstructionAggregates` 只钉两处 store 的存在、不钉偏移，把元素偏移步长改成 0（两元素重叠）时单测仍绿而黄金红——已补 `gepsOf(0)`/`gepsOf(8)` 偏移钉，复验后单测亦红。
  **边界（披露，均安全停而非误发射；已逐条真机取证 check 0 / build 70）**：元组插值直渲染（D4 的 `(f0, f1, …)` 形）；嵌套元组（构造与解构两侧）；newtype 作 record 字段类型（`layout` 不脱壳——先于本任务即停）；newtype 包 record 的方法派发（`recvKeyOf` 见 `ntEnv` 即不解析）；泛型 newtype（`bndGenericFns`，与泛型 fn 声明同规）。
- [x] scope resource 语句（自 T2 移入）：多绑定逆序释放 + 早出口穿透；release 经方法表派发到 `impl Releasable` 的 fn 体（foreign opaque 资源先落，record 资源随记录值面）
  来源：design D1/D4（T2-c 前置核验后移入，见实现记录 T2-c）
  验证：scope resource 程序 build+run（正常出块 / 早 return 穿出各一，释放序钉）+ 黄金钉；既有 check-ch13-scope-green 检查面黄金零回归
  落点：`scope_res_test.go` 八枚（逆序释放 / 早 return 释放 / 每条出口恰一次 / 派发到 impl 体 / 释放先于外围 defer / break 穿出 / 不透明头经原生 close / 嵌 scope expr 内层先出）；`ffi_test.go` 新增 `TestForeignOpaqueReceiverPassesToNativeClose`（章 19 释放习语：不透明头的方法把 `self` 直传原生 `fclose`）；`return_test.go` 新增 `TestVoidBodyTrailingExpressionIsAStatement`（下①收窄的钉）。黄金 `run-ch13-scope-res-order`（`in` / `21`——`n = n*10 + id` 下 `21` 即逆序且恰一次）/ `run-ch13-scope-res-early-exit`（`7` / `39` / `0` / `3909`——早 return 穿出与正常出块各一；`9` 是 defer 标记，`39` 证释放先于 defer）/ `run-ch13-scope-res-opaque`（`in` / `21` / `7` / `213`——foreign 块的不透明 `byres record CFile` 作头，两出口都经原生 `fclose`）。全量黄金三轮全绿（`ok … 73.809s` / `ok … 79.293s` / `ok … 70.881s`），`check-ch13-scope-green` 零回归。
  **偏离披露（D13，三处实现缺陷，均为真机探针揭出、非静默绕过）**：①T5-2 给 `emitFnDefine` 加的尾项识别臂（`*ast.ExprStmt`）过宽——章 6 只在**声明了返回类型**时把体末项表达式当隐式返回（`fnAbiOf`：`d.Ret == nil` 即未声明），未声明者归第 8 章值丢弃规则。该臂对 void fn 一并提升，而章 19 的释放习语恰是「裸尾调用」，于是 release 体被按值返回分类、停 bndFn。收窄为 `if fd.decl.Ret == nil { break }`；单测的释放体一律写 `&ast.Return{}`，故该过宽只在真机上现形。②`emitForeignCall` 的不透明实参只查 `prims`，而方法接收者 `self` 绑在 `gcEnv`——不透明头没有字段可传，`self` 是它唯一的把手。补 `gcEnv` 臂（并据 `e.opaques[g.rec]` 判别是否不透明）。③`emitRecordValue` 的 Call 臂只认 `ckGc`，不透明返回是 `ckPrim`（裸指针）⇒ scope 头拒不透明值。析出 `opaqueKeyOf`（自 `isOpaqueRef`）+ `calleeDecl`，新增 `emitResHead` 供 scope 头走。
  **边界（披露，均安全停在 bnd 而非误发射；已逐条真机取证）**：无尾返回的非 void fn 停在 bnd（check 已按 E0501 拒之，属正确防御面而非缺陷）；panic 路径无展开清理（`emitPanic` 直发 `__we_task_fail` + unreachable，与既有 defer 行为同规，非本任务引入）。

## T6 闭包全捕获 + fn 值

- [x] 红：捕获外层绑定（标量/gc record/String 各一）的闭包今日 bndCallbackBody 停——红证据
  来源：design D5
  验证：红证据在完成记录
  落点：六枚真机探针（`/tmp/t6i`·`t6g`·`t6h`·`t6fn`·`t6nest`·`t6ho`）对 HEAD 树 `/tmp/we-head` 逐枚取证（临时副本内构建，不触碰工作树）：标量捕获与 gc record 捕获两形 check 0 → **build 70 停 `bndCallbackBody`**；fn 裸名作值 / fn 值作参 / 高阶 fn 作值三形停 `bndMainBody`；嵌套闭包形停 `bndFnBody`。**红行原文对 String 面不成立（D13 订正）**：旧 `emitCallback` 只换 `e.scalars`，`e.strEnv`/`e.gcEnv`/`e.prims` 一概不动，回调体解析外层 String 名时走进创建点 `strEnv`、把创建点操作数原样发进 thunk——静态字面量形（操作数是模块常量 `@.s0`）泄漏恰好产出合法且正确的 IR（HEAD 实测输出 `3`，**不停**）；计算形（操作数是创建点 SSA 值）被 clang 拒 `use of undefined value '%v2'`。即 HEAD 的 String 捕获不是「停」而是**静默泄漏 + 无效 IR**，本任务根治（t6h 计算形 B1a run 0 —— `2`）。
- [x] gc 载体 {fnptr, env} + value 字拷贝 + gc 指针 trace 入 env + fn 值调用 + prim 回调 (fn, env) ABI 兼容
  来源：design D5
  验证：闭包捕获程序 build+run（含 Mutex.update 回调闭包捕获形）；bndCallbackBody 词删；黄金钉
  落点：闭包 = 内联 thunk define（`(ptr %env, 参数…)`）+ **创建点环境块**（`@.emapN` 位图 + `__we_alloc(16 + 8*words)` + 头 store + 入根窗 + 逐字 gepStore），thunk 入口 `materializeCaptures` 把 env 逐槽 load 回创建点名字的绑定环境（标量→`scalars`、prim→`prims`、String→`strEnv` 双词不置 trace 位、gc→`gcEnv` 带记录键、fn→`fnEnv` 带签名），闭包体因此走 `emitBodyCore` 与 fn 体**同一条路径**；fn 值 = 单 gc 载体（`__we_alloc(32)`，描述符 `@.fnmapN = [1 x i64] [i64 2]` **只 trace 第二字**，16 存 fnptr / 24 存 env），调用 `emitFnValueCall` 取回两字后一律 `<fnptr>(ptr %env, args…)`；声明式 fn 走值位经 `fnAdapter` 适配 thunk（披露②）；fn 型参数 = 一个载体指针（`abiFn` + `fnParamAbi.sig`，签名随载体**静态传递**）；prim 回调 `(fn, env)` 直发，零捕获 env = `null`（旧黄金形不变）。`bndCallbackBody` 词删（常量 + `ctxCallback` + `bndCallback()` 随 `emitCallback` 整体退役；`m9b_test.go` 两枚边界例重锚 `bndFnBody`）。单测 `closure_test.go` 十一枚目录见实现记录；黄金 738 → **744**（+6：`run-closure-scalar-capture` `3` / `run-closure-record-capture` `5` / `run-closure-string-capture` `2` / `run-fn-value-call` `4 9 10` / `run-closure-nested` `42 15 7` / `run-fn-value-higher-order` `4 11`），**零重锚零退役**；8 枚突变复验全捕获（M6 首轮两层存活 → 补钉，M2/M6/M8 黄金层空缺已留痕 T13）。

## T7 List 载体 + 组合子 + for-in

- [x] runtime/c/list.c：__we_list_new/push/get/len/snap（可增长新分配拷贝；traced 位图；快照拷贝）
  来源：design D6
  验证：C harness 单测 + GC 存活探针扩容（list 元素跨收集可达）
  落点：`{len@16, cap@24, traced@32}` 头 + 自槽 3 起元素区、**一元素一字**；`push` 答**新标识**（倍增增长、新块拷贝、旧块就地弃）；`get` 越界走章 14 族**停任务**（**非**章 17 `List.get` 的 Option 语义）。traced 载体的布局描述符由族自建（容量是运行期值，静态描述符覆盖不了），掩码按块 size 字定尺、零块上整容量置位；标量载体不发描述符。`runtime/list_test.go` 两枚 harness（追加与读取 / 倍增增长连拷贝与域 / 掩码逐槽对收集器自算槽数 / 自由链复用 / 快照独立性 / 元素槽可达性；越界读）——五枚突变复验（掩码位、增长拷贝、快照长度、get 守卫、增长域）逐枚转红后还原。黄金账零变动。完整论证见提交 `56ed689` 正文。
- [x] List 字面量发射 + for-in List 源（快照语义：迭代前取 __we_list_snap）；Range 源已在 T2-b 直发计数循环（不物化），String 源归 T4
  来源：design D6（源归属在 T2-b 实现期重划，见实现记录 T2-b）
  验证：迭代程序 build+run；黄金钉
  落点：字面量 = `__we_list_new(i64 元素数, i64 traced)`（容量即元素数）+ 一条 `push` 链，**每元素求值为一个字**（标量域 / Float64 **位型** / gc 句柄），创建点与链尾各推一次根；元素面取注解 `List<E>`（空字面量唯一来源）否则首元素之形。for-in List 源 = `__we_list_snap` 快照 + 入根 + 计数循环，体首 `get(ptr 快照, i64 计数器)`，头模式绑定那一个字；字面量源的**元素求值先于快照**（源序）。`let ys = xs` 零发射共享载体（章 17 gc 类）。停点面：String/sum/元组/嵌套元素（单字装不下，不写半截）、List 入 fn ABI、用户 Iterable 源、非绑定非 for 源位、gc 头模式体内被赋值——均安全停 bnd。红证据：`list_test.go` 十二枚对 HEAD worktree（`552e2ea`）**七红四不红**（正形七枚 `got boundary "main bodies …"`；四枚边界例 HEAD 本就成立）+ 真机 `/tmp/t7/g1` HEAD 70 → B1a 0。黄金 745 → **751**（+7：`run-for-list` / `run-list-records` / `run-list-domains` / `test-fn-body-for-list` / `build-bnd-list-elem` / `build-bnd-list-abi` / `build-bnd-list-iterable`；**1 退役**：`test-fn-body-bnd` 按 D12 翻绿重锚为 `test-fn-body-for-list`）。m10b fn 体钉由 List 字面量源重锚为用户 Iterable 源（真机复核仍停 `bndFnBody`）。6 枚突变复验（M1 活载体 / M2 trace 位 / M3 浮点位型 / M4 值类拷贝 / M5 容量 / M6 注解；M1 首轮暴露单测钉缺口 → 补钉）；M3·M6 两层皆红，M1·M2·M4·M5 黄金层因**结构性不可观测**空缺（逐枚理由见实现记录）。
- [x] 6 急性组合子内建面（fold/reduce/count/any/all/find 循环直发；We 体仍检查面）
  来源：design D6（Q3 裁决）
  验证：组合子程序 build+run 断言值；check 面同机受检（既有 M8 机制零改）
  落点：识别形 = `<List 源>.iterator().<名>(实参…)`——三关（名在六枚内 / 接收者是零实参 `iterator` 调用 / `listFaceOf` 判其为 List 源）任一不合即**非本面**，静落下方各面（String 迭代器、用户 Iterable、五枚惰性名在 HEAD 的旧停点零改）。六枚共一条走查 `openListWalk`/`closeListWalk`（快照 → 快照入根 → 读长 → 计数头 → 每轮 `get(ptr 快照, i64 计数器)` 取本轮元素字）：与 for 走查同一形，故章 17 定序规则同因同码；**回调先于头开**（`emitFnArg` + `acuteCallback` 签名——fold 由 init 之形定累加子面、reduce 与谓词取元素面），回调即 T6 的 fn 值、调用 `<fnptr>(env, …)`。单字出口 `listElemWord` 把元素字还原为回调实参（gc 句柄→指针、**值类记录在头部按章 8 拷贝**、Float64 位型→double）；`scalarWordFace` 判元素字**是否其类型在 i64 域的值**（`skI64/skU64/skBool/skRune`）——gc 句柄与浮点位型皆非，reduce/find 的载荷因此停 bnd（match 臂把载荷绑进标量域，发出去就是把错的东西重新解释一遍）。fold 累加子在槽（跨回边，同循环携带绑定）；count 数自己的圈数、不取元素面；any/all 首次定局即置位并直跳出口（起始 0/1 与命中 1/0 按名翻转）；reduce 首元素即累加子（计数器自 0 起，`icmp eq cur, 0` 分 `rfirst`/`rlater`）、载荷对 None=0/Some=1；find 首次命中写 Some(元素字)。结果形 ckI64（Int64/Bool/Float64）与 ckSum{None,Some}。红证据：`acute_test.go` 十八枚对 HEAD worktree（`1d389fc`）**十二红六绿**（十二枚 `got boundary "main bodies …"`；六绿皆边界例——HEAD 本就无处不停，属非判别性）＋真机四枚 `run-acute-*` 黄金在 HEAD 上 `exit: want 0, got 70`、stdout 空。黄金 751 → **759**（+8：四枚 run 正形 + 四枚 build 负例），**零退役零重锚**；11 枚突变复验（M1/M2/M3/M6/M7/M10/M11 两层皆红，M5 首轮单测层存活→补钉后两层皆红，M4/M8/M9 单测红·黄金绿——三枚结构性理由逐条见实现记录）。

## T8 顶层 let + 模块 init + 全局根

- [x] `@<key>.init` 装载后序发射 + startup 调序面 + 标量顶层绑定全局直存
  来源：design D7
  验证：跨模块顶层 let 程序 build+run（依赖模块值被 main 用）；build-bnd-toplet 黄金翻绿重锚
  收口：调序面落 `__we_main` 体首（startup.c 是固定 C 文件，命名不了模块集——见实现记录 T8-1 裁定一）；重锚只改期望改名不动（源码形 String→标量，先例 T2-b/T4）；`run-toplet-{cross-module,load-order,domains,diamond,shadowed-qualifier}` 真机定形首跑绿；实现记录 T8-1（15 枚突变、4 枚分类存活、3 枚补钉）
- [x] String 顶层绑定的一对全局（`@<key>.<name>.{p,len}` + init 存读同面）
  来源：design D7（**本面按 D3 修正**：String 字节在 gc 域外，**不得**登记为根——见实现记录 T8-2A 裁定一）
  验证：跨模块 String 顶层绑定程序 build+run（限定名读、拼接、依赖内后枚读前枚）；build-bnd-toplet-carrier 黄金翻绿
  收口：`topScalarSlot` 改名 `topSlot` 并增 `str bool`（一张表一处读面）；`.p`/`.len` 两枚全局而非聚合体（读 = 两次 load，与标量面同构）；`emitStringExpr` 的 Member 钩经突变证为冗余已删（`emitFieldChainString` → `emitMemberValue` 的 T8-1 钩已覆盖）；`run-toplet-string{,-cross-module,-concat}` 真机定形首跑绿；实现记录 T8-2A（14 枚突变、1 枚补钉、4 枚分类存活）
- [x] 根记账按 body 放电（`emitBlockStmts` 块出口 + break/continue 边）
  来源：**回补 design D7 / design.md:82 的意图**（「根推送按 body 记账，`e.pushes` 在每个 body 出口弹」——HEAD 只做了函数体一个 body，非协议变更）
  验证：100k 趟循环内分配探针（`roots` 归零、`swept` 由 0 转 14564）；run-gc-root-discharge 黄金真机先行定形（HEAD 出 `1`、修复后出 `49`）
  收口：`e.pushes` 语义由「静态见过的推送数」改为「**路径上的活根数**」；break/continue 弹到**循环基深**而非函数 0（基深以下仍活着）；`return` 面**零改动**（四处函数出口欠的就是「全部活根」）；`@<key>.init` 体的放电**随 2B**（此刻无顶层绑定推得出根，钉会是空钉）；**披露缺口**：return 穿过循环体在 B1a 无源入口（M9b 只收每 body 一个尾 return），其钉落 T9 深返回；实现记录 T8-2B-0（12 枚突变零存活、3 枚补钉、1 枚层间分工存活）
- [x] gc 顶层绑定全局根登记（`__we_gc_root_global` + init 期根屏障复核 + init 体放电及其钉）
  来源：design D7
  验证：三枚真机先行定形的判别性黄金——`run-toplet-gc-record-root`（`1 9 80000`，负控制 `9 9 80000`）、`run-toplet-gc-list-root`（`100 28 80000` / `0 28 80000`）、`run-toplet-gc-cross-module`（`77 60 80000` / `77 0 80000`），OFF 数字经黄金 harness 实测复现且**只差 stdout 一行**；「泄漏掩盖根表」判别探针（登记与放电皆摘 → 与修复后逐字相同）
  收口：T8-2A 已证 String 半入根表是**错的**（D3），本子项只收 gc 句柄；注册的是**槽地址**而非值，故 D7 的「init 前后各跑一次根屏障」需按此复核（值语义才需要屏障——见实现记录 T8-1 裁定一与 T8-2A 裁定一）；**init 体放电及其钉由 T8-2B-0 移交至此**（2B 的 gc 顶层绑定是把根放进 init 体的那一面）
  裁定：D7 的根屏障被**回答**而非实现（槽是零初始化静态、扫描自解引用，**没有值就没有屏障的语义对象**，写屏障同样不需要）；String 不登记（D3）；值记录仍停（第 8 章的各自对象）；`List<String>` 在 B1a 整层之外（顶层与函数级同停，**非顶层缺口**）
  实现记录 T8-2B（9 枚突变零存活、2 枚补钉、3 枚运行期层间分工 + 1 枚反向单测独守）

## T9 sum 物化泛化 + 槽放宽

- [x] sum 三槽 {tag, pay0, pay1} 布局扩形（聚合返回/参数多 retval；M10b ABI sum 行扩形过渡披露）
  来源：design D8
  验证：`go test -count=1 ./internal/codegen/` 绿（`TestM10bFnAbiMatrix` 的 sum 行改三槽形即红）；conformance 773 枚**零漂移**（无一枚黄金断 IR，运行期面是出参式故 C 侧零改动）；真机 build+run 探针，与扩形前的行为逐字一致
  收口：`fnAbiKind` 注释 / `abiTypOf` / `abiWordTypes` / `fitAbi` / `sumSlot` / `tupleShapeOf` / `emitTupleElemValue` / `storeTupleElem` / `bindDefineParams` / `emitFxGate`（补 `n+"2"`）/ `fnRetOperand` abiSum / `emitSumCall3` / `emitSumCall` / `emitReduce` / `emitFind` / `emitScope` 值形 / `emitMatchArm` / `emitQuestion` / `emitCallCore` 结果面。每个生产点把 `store i64 0, ptr <pay1>` 发在 tag/pay0 之后（`acute_test.go` 的正则取前两个零存，次序错即红）
  裁定：`fnRetOperand` 的常量聚合**装不下 SSA 值**（与 `abiStr`/`abiTuple` 两处同陷阱）——值形走 `insertvalue` 链，常量形仍是一枚字面聚合
  披露：**本枚单独无法做语义钉**——此刻还没有东西能产出非零 `pay1`，验证只能是「形钉更新 + 零漂移」，语义验证在 T9-2 到达（design D8「黄金对拍过渡期披露」所指）
- [x] fn 尾返回停点退役（Err/带载荷 sum 返回物化）+ sum/记录/同步型参数入槽 + String 参数双标量展开
  来源：design D8（**本面扩形，T4 裁定 2026-09-10**：String 的**两字槽存储面**——`var r: String = s` 绑定与 String 名重赋值——一并归本任务；T4 只携带值形与 `let`，边界披露见 design D3「发射集边界」与实现记录 T4 裁定表第 1 条。落地时连带：M15 的 `control-03/string-mutable-binding-decline` 校准转 latent，须再寻 test-malformed 载体——见实现记录 T4「裁定落地（T4 提请面）」）
  验证：用户函数返回 Option/Result/自有 sum 程序 build+run；bndErrPayload 词删；~~test-fn-body-bnd 黄金重锚绿~~（**对账：该黄金已不存在**——T7-2 `1d389fc` 删除并取代为 `test-fn-body-for-list.json`，已绿）；M15 run-face anchoring 槽放宽句兑现（T12 同步）
  收口（三枚，T9-2/T9-3/T9-4）：**T9-2** sum 构造与返回物化——`Option<T>` 进 `classType`、变体形表 + 预算谓词、构造子进 `emitCall` 的 Ident 分派、`fnRetOperand` abiSum 泛化、`sumSlot` 携带变体形表、`emitCallCore` 两实参入口加 `abiSum`、main Err 报告面泛化（常量快路径逐字节不变）；黄金 `build-err-payload-eager`（改名）+ `run-err-payload-two-words` / `run-fused-err-return` / `run-sum-ctor-expression`。**T9-3** sum/记录/同步型参数入槽——`abiPrim`（第 18 章 family，一个指针）、`fnParamAbi` 携带声明、实参位读期望、构造走调用臂；黄金 `run-sum-param-match` / `run-prim-param` / `build-bnd-prim-return`（负钉）；连带结清 T8-2B-0 的循环体 return 披露（**误记，补的是测试不是代码**）。**T9-4** String 两字槽存储面——`strBinding` 增 `slot`/`lenSlot` + `bindStringSlot`/`strSlotLoad`/`emitStrStore` 三 helper，五处绑定判据接 `e.assigned`（`var` 面无条件占字），三处读面 normalize；黄金 `run-string-reassign` / `run-string-mutable-binding` / `run-string-binding-faces` / `build-bnd-var-unannotated`（负钉），`build-bnd-string-assign` 退役；电池首轮三枚两层齐活（B1/B2/R2），补四枚钉 + 一枚黄金后两层齐红。
  裁定：**Err 融合 tag 空间**（tag 0 = Ok、tag 1+k = E 第 k 变体、`pay0`/`pay1` 别名复用、三槽恒定——装入 `Err(<E>)` 的唯一全函数编码；被拒：保持边界 / 四槽）；**预算规则单点强制于 `classType`**（每变体载荷 ≤ 2 字且不得为 sum）；**同步类型返回判为不开**（两道守卫，电池证任一道单独成立）；**`?` 在 fn 内不进本任务**（D8 只指名 fnRetVal，非目标防蔓延，记披露）。
  实现记录 T9-2（6 枚突变，6 死单测 / 5 死黄金）、T9-3（14 枚突变，12 死、2 枚结构性存活）、T9-4（14 枚突变，9 枚两层皆死〔其中 1 枚由编译器捕获〕、2 枚单测红·黄金绿、3 枚首轮齐活 → 补钉后两层齐红）

## T10 assertEqual Eq 域

- [x] typecheck 域门放宽（+derive Eq 记录/和式/元组递归域）+ 结构比较发射 + 失败消息值渲染（of_* 复用）
  来源：design D9（Q4 裁决）
  验证：Eq 记录断言 pass/fail 程序（失败消息面钉）；bndAssertEqDomain 词删（域外并 bndGenericFns）；~~test-asserteq-domain-bnd + check-assertequal-domain-boundary 黄金重锚绿~~（**对账：两枚皆改名**——`check-assertequal-domain-boundary` → `check-assertequal-domain-eq`（T10-1，同源转 check 干净），`test-asserteq-domain-bnd` → `test-asserteq-domain-eq`（T10-2a，源不变转 pass）；另新钉 `check-assertequal-residual-bnd`（残余停点在新词下）；`test-asserteq-*` 族由 6 枚到 **19 枚**、`check-assertequal-*` 族由 4 枚到 **5 枚**）
  收口（四枚）：**T10-1** 检查器域门——`inEqDomain` 递归域谓词（平行于 `carries`，**不动 E0823**）+ `bndAssertEqDomain` 词删、残形并 `bndGenericFns`；**T10-2a** 叶族 + 记录结构比较——`eqFace`/`eqWalk` 算子形 walk、`eqPathJoin`/`eqPathArg` 位置路径、报文精确尺寸（两趟 vsnprintf + malloc）、32 段深度界在**下降**而非比较、顶层 Bool/UInt64 各归自家行（裁定 7）；**T10-2c** 元组 + 新类型透明——`emitEqTuple`（元素形取自元组自己的分类，`tupEnv` 与 `emitTupleAgg` 同一对）、newtype 零新增（包装的擦除被继承）；**T10-3** 和式 tag 分派——`emitEqSum` 三块族（`eqsd` 报两名 `noreturn` + `eqss` 分派 + `eqsp` join）、载荷读面逐字镜像 `bindArmWord`、`sumSlot.key`（表的另一半：**表说不出声明有没有派生 equals**）。
  裁定（T10 期，逐条已落 design D9 对账）：**复合须自带 `derives Eq`**（记录/和式/新类型；元组无声明可载，按元素递归，裁定 5）；**叶子限既有标量集**（8 整型 + Bool + String，与 `assertEqScalar` 逐字一致）；**残形不设新词**；**报文 = 路径 + 叶子值**（路径为位置索引段，顶层无路径则整条 `at` 子句不出现）；**顶层 Bool/UInt64 一并对齐**到同一模具（M10b 冻结面上一处**行为变更**：今日 `assertEqual(true,false)` 报 `got 1, want 0`，改后报 `got true, want false`）。
  披露：非泛型无 derives 记录拿到「generic functions…」指向（裁定 3 的代价，最常见的用户错误配错方向的提示）；`derives Eq` 记录带 Float64/Rune 字段是**规范合法而断言面边界**（`.equals()` 可用，绕行 `assertTrue(p.equals(q))`）；元组内和式、嵌套元组、跨模块嵌套记录 = 检查器入域而发射面拒（**诚实边界，非新边界**）；`Option`/`Result` 无声明可查（手写 impl 是 E0822）；载荷预算 2 词（`Both(Int64, String)` 3 词整个和被拒）与 Rune 载荷停在 **body 词**非域词；裸变体名在 `let` 值位置同停 body 词。
  实现记录 T10-1（`inEqDomain` 三处语义不合故平行新增）、T10-2a（8 枚突变：字段序 / UInt64 行 / Eq 重查 / 路径分隔符 / 顶层 String 行〔单测独守，设计如此〕/ 深度界〔跟随常量的测试存活，写死字面量后死〕/ 守卫位置）、T10-2c（2 枚突变：索引 off-by-one 两层皆死、元素用元组自身位置单测死）、T10-3（6 枚突变：载荷字规则与构造 key 两层皆死、参数 key 单测独守 → 补黄金后两层死、名链尾 / 分派 fall / String 长度字黄金层死）
  平账：conformance **782 → 796**（+14）、`internal/codegen/` 单测 **283 → 304**（+21）、`internal/typecheck/` **75 → 76**（+1）；四枚提交 `0e18bbd` / `83dc2c0` / `26a7104` / `a1beee9`

## T11 既有缺陷修复组（各红先行）

- [x] float let 绑定丢域（isFloat 透传 scalarSlot）——红：`let h = 1.5 + 0.5` 下游消费无效 IR
  来源：design D10-1（**本面已随 T3 落地**：统一数值绑定路径 `bindNumericValue` 保留 isFloat，见实现记录 T3「D10 划账」——本 checkbox 收口时对账，勿重做）
  验证：红→绿；浮点 let 程序 build+run 值正确
  对账（T11 收口，2026-09-11，**只对账不改码**）：`TestFloatLetBindingKeepsDomain`（`expr_test.go:257`）+ 黄金 `run-float-values` 均在，跑绿。另记 D10-1 的措辞与实况不符（pre-T3 三形实为**边界红**，不发无效 IR）——T3 实现期已订正，见 proposal.md 实现记录 T3 披露②。
- [x] `&&`/`||` 短路分支形——红：RHS 含 panic 面求值序（今日 i64 and/or 两臂恒求值）
  来源：design D2/D10-3（**本面已随 T3 落地**：`emitLogic` 分支形 + 黄金 run-logic-short-circuit，见实现记录 T3「D10 划账」——本 checkbox 收口时对账，勿重做）
  验证：短路序程序（RHS 不求值形）值钉；黄金钉
  对账（T11 收口，2026-09-11，**只对账不改码**）：`TestLogicShortCircuitSkipsRHS` + `TestLogicShortCircuitPhiNamesRealPreds`（`expr_test.go:419`/`:504`）+ 黄金 `run-logic-short-circuit` 均在，跑绿。附带事实在案：pre-T3 树的短路测试是**边界红**，缺陷①的真红面是修后钉（结构断言 + 黄金），见 proposal.md 实现记录 T3。
- [x] 窄整型逐宽溢出检查——红：i8 127+1 今日静默回绕实证；修：逐宽界检查 icmp + task_fail
  来源：design D10-4（ch7 逐宽 checked 语义兑现）
  验证：i8 溢出程序 run panic（E0502 陷阱面消息）；i64 既有黄金零回归
  落地（T11-3，2026-09-11，提交 `ca268e7`，见 proposal.md 实现记录 T11-3 与 design D10-4 对账块）：**探针实证红形与任务书逐字相符**——pre-变更二进制上 `var a: Int8 = 127i8; let b: Int8 = a + 1i8` **exit 0**、静默得 -128。修：窄域**替换**（而非叠加）i64 内建，改发平算术 + `narrowGuard`（逐宽界 `icmp slt`/`icmp sgt` → `or i1` → `task_fail`）；`<<` 保留量/回环/宽度三枚守卫、`/` 只查商、`% & | ^ >>` 零枚；同落 `~` 的无符号宽度掩码（`narrowMask`）。**红证据**：`a1beee9` worktree 上单测 **13/15 红**（两枚绿的是断言无新行为者）、黄金 **8/8 红**（六枚 exit 0 静默回绕、`shift` 报错名字存器宽度、`values` exit 70）。**电池 8 枚**：三枚两层齐活（`capNum`/`emitForRange`/`ovMsg`）→ 逐枚补黄金后两层齐死；一枚等价存活（`narrowDomain` 左优先，ch7 无隐式转换故不可观测）。**验证**：i8 溢出程序 run panic；i64 既有黄金零回归（`run-shift-*`/`run-add-overflow-panic` 等逐字不变）；conformance 796 → **804**、`internal/codegen/` 304 → **319**。
- 第五枚（非 entry 的 alloca 逐迭代——循环体内 `var` 在 HEAD 真机 30M 迭代 SIGSEGV）于 T2-a 实现期发现并随槽面重构根治：红（HEAD exit 139）/绿（B1a exit 0）/机制铁证（同 IR 移行对照）见 proposal.md 实现记录 T2-a；回归钉 `TestSlotHoistedToEntry`。design D10 的设计期四枚清单不动。

## T12 M15 基准套件重锚

- [x] traps_test.go 四校准例重锚：concatenation-boundary → latent、string-equality-decline → latent、modulo-decline 删（陷阱消亡）、mutate-let-param 删（陷阱消亡）——逐枚真二进制探针定形后落断言
  来源：design D11（**四枚已全部落地**：mutate-let-param 消亡于 T2-a、modulo-decline 消亡于 T3——`%` 已发射 ⇒ 突变后的参考解是干净程序、无捕获面；concatenation-boundary → `concatenation-latent`、string-equality-decline → `string-equality-latent` 于 T4 落地（字符串链一发射，两枚突变形均成干净程序、错值直达参考测试）。同批连带：`blackbox_test.go` 的 test-malformed 载体重锚两次（control-04 → control-03 字符串相等于 T3，二锚 → control-03 可变 String 绑定的 `string-mutable-binding-decline` 于 T4——T4 新增该校准例顶替空位），以及 `docs/benchmarks.md` 双语算子句最小修正（T3）。见实现记录 T3/T4「M15 面连带修正」。**本 checkbox 收口时无剩余枚**，勿重做）
  验证：TestTrapCalibration 全绿（~~64→62——63 于 T2-a、62 于 T3，计数已达标；T4 只换桶不改枚数~~）；消亡披露于完成记录（陷阱纪律：无捕获面 = 修清单）
  对账（T12 收口，2026-09-11）：**四枚处置与任务书注记相符，逐枚真机定形在案**——`concatenation-latent`（traps_test.go:95）/ `string-equality-latent`（:98）皆 `BucketLatent`；`modulo-decline`（:123 退役注记）与 `mutate-let-param`（:132）随陷阱消亡删，traps.md 条目同删。**计数订正：as-built 为 63，非 62**——四枚 git 快照逐枚数 `7b7992c` = 64 → T2-a `2eae49c` = 63 → T3 `05820e0` = 62 → T4 `b55d208` = **63**；T4 的 diff 是三行而非两行（新增 `control-03 string-mutable-binding-decline` 顶上 test-malformed 空位，即 D11 表第五行那枚），T9-4 改名 `...-latent` 不改枚数。桶分布 Latent 26 + Rejected 37，覆盖 18 枚任务全数。test-malformed 载体链：`control-04` modulo → `control-03` 字符串相等 → `control-03` 可变 String 绑定 → **`control-03` 的 `echo` 泛型形**（`blackbox_test.go:35-51`）。支配缺陷三枚复核探针（match 臂赋值先于 scope / 跨 scope 两侧 / receive-with-match 先于 scope）check 0·build 0·run 0，值 `1007`/`1025`/`1041` 逐一正确。详见 proposal.md 实现记录 T12。
- [x] errpath-02/ctxpass-02 traps.md 支配缺陷括注更新 + docs/benchmarks.md 双语 run-face anchoring 段重写（槽放宽/字符串/算子/let 重赋值面兑现）
  来源：design D11
  验证：docs_sync 33 对；基准套件全测试绿
  落地（T12-2，2026-09-11）：四文件——errpath-02/traps.md 末条 implementation note 由「绕行注记」改「缺陷已修注记」（`roadmap follow-up #16`，修于 T1，参考解次序非必需）；ctxpass-02/traps.md 第三条尾注同面更新并指回 errpath-02；`docs/benchmarks.md:50`「Run-face anchoring」段重写（三条陈朽句换成 as-built 事实 + **仍关死的五面**显式清单），`docs/benchmarks.zh.md:50` 逐句镜像。新段每一主张皆有真二进制探针：正向——sum 构造/返回/match、sum 型与同步型参数入可跑槽、`Int16` 返回且陷阱点名宽度（`error: Panicked: Int16 add overflow`）、`String` 相等与拼接、记录方法/元组/新类型/List 迭代/闭包与 fn 值、`%`·`/`、任意深度 return，全 check 0 / run 0；关死五面——泛型（`bndGenericFns`）、普通 `scope` 值形、main 体 `?`、整只元组值绑定读入 `String` 位、值位 `if`/`match` 绑定读入 `String` 位，全 check 0 / run 70；运行期规则——main 纤程在自家 timeout scope 内 `receive` 仍 `we: deadlock: 1 tasks parked with no wake source`（exit 70）。验证：`docs_sync.py` → `OK: 33 document pair(s) aligned`；`validate.py --all --strict` → `OK`；`go test -count=1 ./internal/benchmarks/` → ok；`gofmt -l` 空、`go vet ./...` 干净。披露：T11-3 记录第 2 条「源码不可达面」措辞过宽（三面的真实边界是**读入 `String` 位**而非绑定本身，`narrowform` 探针 run 0 即证）——**已按用户裁定（2026-09-11）订正**：三处同批改（proposal.md 该条 / design D10 同句 / `narrow_test.go:325` 码内注释，历史原文删除线保留），`ca268e7` 的 `Not-tested:` trailer 不可改写、此订正即其披露；缺口本身并进 T13 对账项（design D12 + 本文件 T13 行）。详见 proposal.md 实现记录 T12。

## T13 边界词退役对账 + conformance

- [x] 三锚词表退役 + 词表终态 grep：~~bndMainBody/bndTaskBody/bndFnBody 删除~~（**订正：按用户裁定 1 保留三锚，退役改挂 B1b 实际落地时**——见 design D0 订正）；~~终态恰余 bndGenericFns（括注改 B1b）~~（**订正：终态为 5 词**——`bndMainBody`/`bndTopLets`/`bndTaskBody`/`bndGenericFns`/`bndFnBody`）；停点锚定黄金全部重锚绿（5 枚改名翻绿 + 3 枚重锚 + test-fn-generic-bnd 维持 bndGenericFns 钉）——逐枚对账表入完成记录。T2-b 已先行重锚 1 枚：test-fn-body-bnd 的源改 String 形（for-over-Range 已发射），`TestM10bBndStops` 同步改锚；**T4 再锚 1 枚**：该黄金的源改 List 形（`for x in [1, 2, 3]`——for-in String 被 T4 发射后原载体失效，List 源在 fn 体仍停 `bndFnBody`），`TestM10bBndStops` 同步再锚；同批 `m8_test.go` 的 M8 边界例改「record value in io argument」（原「arithmetic in io argument」随拼接发射失效）——见实现记录 T4「M15 面连带修正」⑥⑦
  来源：design D0/D12
  验证：`go test ./internal/conformance/` 全绿（新计数对账 + 既有非停点黄金零触碰清单）；~~词表 grep 恰余 bndGenericFns~~ **订正：grep 恰余 5 词，生产点 314 处零改动**
  对账（T13 收口，2026-09-11）：**23 枚 exit-70 黄金逐枚归词**——`bndMainBody` **12** / `bndGenericFns` 2 / `bndFnBody` 1 / `bndTaskBody` **0** / `bndTopLets` **0** / 非 codegen 词 8（single-file ×4、std ×2、ch21 ×1、arity ×1，D12 已列零触碰）。**新发现**：`bndTaskBody` 与 `bndTopLets` 今日**无任何黄金锚定**（只有生产点：前者 2 处、后者 `codegen.go:6041` 1 处），故「三枚体锚」精确化为「锚在黄金上的是 `bndMainBody` 与 `bndFnBody`」。**D12 九枚对账：兑现 8 / 未兑现 1**——未兑现的是 `build-bnd-m6b-main-body`（今日仍 70 + `bndMainBody`，源 `let v = f.fetch()?` 落在关死的 main 体 `?` 面），见 design D0 订正；其余逐枚实测（`build-bnd-new-forms` 名系终到 `build-bnd-var-unannotated`、正向面拆 `run-main-early-return`/`-err`；`build-bnd-err-payload` → `build-err-payload-eager`；`test-fn-body-bnd` → `test-fn-body-for-list`；两枚 Eq 域改名对 + 新钉 `check-assertequal-residual-bnd`；`build-bnd-toplet`/`-carrier`/`build-ch10-method-call-boundary` 保名改期望 0）。完整表在 proposal.md 实现记录 T13。
- [x] 新增黄金总量落盘（每拓宽面至少一正一负；逐枚过真编译器）
  来源：design D12
  验证：同上计数对账
  落地（T13-2 `5ae9208` + T13-1 `5f8ac25`，2026-09-11）：**语料 804 → 810 枚**（6 枚新增：1 + 5），`run-*` 128 → **131**；`internal/codegen` 单测 319 → **321**，`internal/typecheck` 76 不动。新增 6 枚：`run-value-form-string-face`（0，T13-1）/ `run-narrow-value-form`（0）/ `run-closure-gc-capture-crossing`（0，stdout `payload:7:1799970000`）/ `build-bnd-value-form-string-arm`（70）/ `-emission-binding`（70）/ `-shadowed-binding`（70）。**逐拓宽面清点表**（T1–T13，含正面与负/边界面两列）在 proposal.md 实现记录 T13。**负例设计订正**：任务书原拟的「`let u = if c {1}` 无 else」到不了 codegen——检查期即 E0202——改钉 `-emission-binding`（块局部绑定）与 `-shadowed-binding`（外层同名遮蔽）两枚发射器真能到达、分类器有意拒绝的形。
  留痕（T6 移交）：~~三枚突变（M2 String 捕获置 trace 位 / M6 fn 型参数按两字过界 / M8 载体描述符 trace 代码指针）**单测层捕获、黄金层空缺**~~ **已兑现**——`run-closure-gc-capture-crossing` 把 M2/M8 的黄金层空缺补齐（突变 C/D 黄金层红）；M6 维持 IR 钉（clang 21.1.8 对多传实参的直接调用既不报错也不走样，无 clang 侧守门），在对账表注明。两处设计订正：增长串推不动 GC（`str_alloc` 走 malloc、`allocated_since` 只计 `__we_alloc` 切块），改记录分配越阈；捕获字面量须避开首字节低位为 1（`blk_marked` 读 `*(u64*)p & 1`，`"captured"` 的 `'c'` 低位为 1 会让突变静默存活），改 `"payload"`。
  对账项（T12 收口登记，2026-09-11 用户裁定，见 design D12）：**`valueKind` 的值位控制形缺口**——~~`valueKind` 无 `*ast.If`/`*ast.Match`/`*ast.BlockExpr` 臂，值位控制形绑定到 `let` 后数值面可用而 `String` 面停（`bndMainBody` 代守），且污染下游~~ **已修（T13-1 `5f8ac25`，用户裁定 2：补臂而非只记账）**：三臂 + `blockKind`/`joinKind`，界为「各分支一致且非 `skStr`」+「块尾之前有绑定则拒绝」。正面落 `run-value-form-string-face`（0）与 `run-narrow-value-form`（0）；反面落 3 枚新边界面。**连带行为变更**：顶层 `let k = if …`（`:6038`）与列表元素面（`elemFaceOfExpr`）随之可用（登记为变更而非附带修复）；`bndTopLets` 生产点收窄但未死，不删。**紧急缺陷与修正**：初版 `blockKind` 按外层帧分类（`emitArmBlock` 已 pop 块的帧），在外部 `let b = true` 下把 `let k = { let b: Int64 = 5  b }` 渲染成 `true`（真机复现、静默错值）——修正 = 绑定守卫 + doc 重写 + `7834e29` → `5f8ac25` 修正提交，mutations E 两层齐红。

## T14 验证阶梯

- [x] gofmt -l 干净 + go vet ./... + go test ./... 全绿（~~14 包~~ **15 包**，见对账②）+ validate.py --all --strict + docs_sync.py（33 对）+ 真机电池（D13-2 全面：字符串/记录方法/闭包捕获/List 迭代/顶层 let/sum 载荷/Eq 断言 + panic 负例 + 支配回归钉）
  来源：design D13
  验证：六面输出在完成记录
  **对账（六面，`-count=1` 一轮）**：① `gofmt -l .` 空；`go vet ./...` exit 0 无输出；`go test -count=1 ./...` **13 包 ok / 2 包无测试文件 / 0 fail / 0 skip**（codegen 0.157s、conformance 121.994s、benchmarks 35.067s、runtime 19.293s）；`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`；`docs_sync.py` → `OK: 33 document pair(s) aligned`。② **包数订正**：任务书写「14 包」，`go list ./...` 实测 **15**（13 有测试 + `cmd/we`、`internal/ast` 两枚无测试文件）——「14」与任一阶段实测都对不上，属计数笔误，task 文本字节不改。③ **D13-4 兑现**：`TestTrapCalibration` **63 子测试全 PASS**，但**在 `internal/benchmarks`**（`traps_test.go:55`）而非 conformance（在 conformance 上 `-run` 回 `[no tests to run]`）。④ **D13-5 兑现**：`b9-dominance`（errpath-02 形）真机 run 0 `dom 1007`；`we build . --verbose` → `we: built build/demo`；产物 IR 直跑 `clang-21` → exit 0、**0 error、1 warning**（`-Woverride-module` 的 target triple 告警，与 IR 无关——上一轮笔记记的「零诊断」不确，此处为准）、IR 文本无 "does not dominate"。⑤ **真机电池 15/15 符合预期**：13 正面探针 run/test 0（字符串/记录方法/闭包捕获含跨 GC/List 迭代/顶层 let/sum 载荷/Eq 断言/支配钉/十面合成），2 负面按设计红（`b7-test-fail` test 1 带断言报文；四枚 panic run 1 带逐宽溢出报文）。探针源码全写在冻结语料已证过的形状上，故 15/15 无新面。⑥ **本枚零码改、零黄金改、零文档改**（纯验证枚）。
  **三枚发现（详见 proposal.md 的 T14 记录）**：**N1** fn 值调用在**值串位/操作数位**停（`let r = f(1)` 可跑而 `"${f(1)}"` 停 70；与捕获无关，界在位置）——诚实边界，**零黄金覆盖**；**N2** 内置函数所产 `Option` 的载荷读入 String 位停（数值消费可跑，注解不救）——诚实边界，**零黄金覆盖**。两枚都属「`bndMainBody` 词对而面不对」，`docs/benchmarks.md` 的 run-face 段对函数值/闭包的表述在位置维度上过宽；是否补黄金留 T15 归档对账。**N3（缺陷类，待裁定）**：**洞（`${…}`）内容不被类型化**——裁定③（T4 期）的已记录故意裁定，其**后果**任何工件都未写过：洞内 `one(true)`（形参 `Int64`）**check 0 / build 0 / run 0 打印 `1`**，同一调用挪出洞外即 **E0501**「no coercion is ever inserted」；IR 作证 `call i64 %v0(i64 1)`，发射器无签名可比。根因结构级可复验：`grep -rn '\.Holes' --include=*.go` 的命中只在 parser（建）与 codegen（发），**typecheck 一个都不读**。范围不夸大：多数洞内错误恰好落 codegen 边界（未定义名/String→Int64/Float→Int64 全 70），**滑过去的只有 Bool→Int64 一形**（Bool-as-i64 是常规寄存器表示，无处可停）。按 T4 自立的判据（IR 干净 + clang 不报 + exit 0 + 输出错值 = 静默错误）属**缺陷类**。**本枚不动码**——裁定③是设计裁定，推翻或替代（类型化洞 / 洞内按 callee 签名复核实参域 / 显式声明为接受缺口）皆为用户级取舍，登记待裁定。

## T15 code-review + 归档

- [x] welang-code-review 7 条（读 SKILL.md 手动执行）；审查记录入 proposal.md
  来源：仓库节奏
  验证：7 条逐条通过记录在案
  审查（2026-09-11）：7 条逐条执行，记录为 proposal.md 的「## 审查记录（2026-09-11）」§1–§7 + 发现汇总表 + 「与 D12 的差（as-built）」。**结论：7 条全部通过**，其中 §4（诊断协议）与 §5（单一权威）各带一枚**已修**偏离（D3 = `parser.go:979` 的 E0105 消息未以登记 title 起头；D2 = 25 行中文注释违 AGENTS.md:39），修正提交 `0aac57b`。审查另抓到一枚**阻塞性工件缺陷** D1（tasks.md 的陈旧重复复选框，已删）并产出 3 枚发现：N1/N2 为诚实边界但**零黄金覆盖**（已收窄 `docs/benchmarks.md` 双语措辞）、**N3 为缺陷类待裁定**（插值洞不被类型化，登记 roadmap follow-up #23）。审查期独立复现了 M2 突变（单测层 + 黄金层双红，逐字还原后 `git diff` 空）。
- [x] roadmap B1 行拆 B1a done + B1b pending（双语）+ follow-up #16 划账 + change.yaml archived + 目录日期前缀 + 记忆回写
  来源：design D12（仓库节奏）
  验证：validate --all --strict 复验 OK；git status 对账
  归档（2026-09-11）：`mv` 至 `openspec/changes/archive/2026-09-11-codegen-mono`（变更目录此前未入册，故用 `mv` 而非 `git mv`），`change.yaml` `complete` → `archived`。**roadmap 双语**：B1 行拆为 **B1a `codegen-mono` done 2026-09-11**（一句话含 conformance 701→810 与 #16 已修）+ **B1b `codegen-full` pending**（点名仍未实现的面，含 B1a 对账遗留的六项停面），`docs/roadmap/0000-reference-implementation.{md,zh.md}` 同步；**follow-up #16 原位闭环**并登记 **#23**（N3），双语。**复验**：`validate.py --all --strict` → `OK: no changes found (nothing to validate); registry clean`（活动变更集为空；registry 检查未短路）+ `docs_sync.py` 33 对 + 完整阶梯全绿，逐项记入 proposal.md「归档复验」段。**D12 偏离（as-built）**：D12 写「#16 条目移除」，实作改为原位闭环保留编号——移除会使 #17–#22 前移、令已归档工件与变更记录里的引用当场失真，见 proposal.md「与 D12 的差（as-built）」。**记忆回写**已执行（`project-overview.md` 的 B1a 收口条目 + `MEMORY.md` 索引行）。
