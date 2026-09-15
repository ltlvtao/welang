# tasks — codegen-full（B1b）

> 红先行注（两件不同的事，别混）：14 枚边界黄金**今日全绿**——它们的期望里就写着「真机 exit 70 + 边界行」（12 枚 `bndMainBody`：`build-bnd-list-iterable` / `-acute-user-iterable` / `-acute-float-payload` / `-acute-gc-payload` / `-value-form-emission-binding` / `-value-form-shadowed-binding` / `-list-abi` / `-list-elem` / `-acute-lazy` / `-m6b-main-body` / `-value-form-string-arm` / `-var-unannotated`；1 枚 `bndFnBody`：`build-bnd-prim-return`；1 枚 `bndGenericFns`：`test-fn-generic-bnd`），所以它们是**今日行为的 characterization**，不是红。B1b 的红来自**新写的 widened 期望**——每簇先把目标程序的 run 期望写进黄金，它在实现前必红（实现仍停在边界行），实现后转绿。D9 簇 10–13 的**四面语料零锚定**——连 characterization 都没有，须先新增锚定黄金。
>
> 未决项阻塞制（process §3.4）：凡影响行为/契约/验收的未决问题，各自作为所在任务组的**首个 checkbox**——未完成不得勾选其后各项。本变更共四枚：resource 进盒的所有权纪律（T5）、内建迭代器类型的载体（T7）、`errMsg`/报告行的语义（T12）、`scope timeout(N)` 的结果型（T13）。

## T1 检查器注记面（Q2）——本变更的地基

- [x] 定形并导出 `Shape` 封闭视图（`Param | Base | Nominal | Dyn | Fn | Tuple | Unit | Never`）~+ 关联类型位~ + 站点键（AST 节点指针）+ **声明面槽序条目**（每个接口声明的非默认方法名与形）
  来源：design D1（含 D6 决策 2 的声明面补充；**订正见 D1 实现期补记二**——落地为九臂 `+ Assoc`、`Fn(Params, Ret *Shape)`、`Site{Args, Params, Ret, Boxed}` 三处对记法的订正）
  验证：`go test ./internal/typecheck/` 全绿（新增 `Shape` 单测：`Wrap<Point>` / `List<Int64>` / 元组 / fn 型 / `Dyn<Iterator<Int64>>` / 参数位逐形断言）；`Iterator<T>` 的槽序为**恰一槽 `next`**（`typecheck.go:1227-1231` 的判据：`body == nil` 即义务）
- [x] `Check()` / `CheckTestRoot()` / `CheckProject()` 出口扩形为**附加返回值**，既有调用方按需忽略
  来源：design D1
  验证：`go build ./...` + `go vet ./...` 全绿；~~`internal/cli` 与 `internal/lsp` **零改动**即编译通过（附加返回值兼容性的直接证据）~~ **订正（T1 落地）**：「零改动」在 Go 下**不可达**——三返回值必须逐调用点显式忽略。实况：`internal/lsp` 零改动；`internal/cli` 三处各插一个 `_`（`check.go:97`、`check.go:164`、`test.go:234`）。真正的兼容性证据 = **改动面恰为 `_` 插入**（`git diff internal/cli` 逐行即此）+ 两个命令全绿；`go vet` 是必要一环而非补充——`go build ./...` 不编 `_test.go`，`m10b_test.go:68` 那处只被 vet 抓到
- [x] 导出去重与命名：`Shape` 不含渲染串，限定键由 codegen 侧 `headKey`/`recvKey` 给出
  来源：design D1（「导出串 = 把命名权挪到 typecheck」的拒绝理由）
  验证：负断言——`Shape` 的导出面**不含**任何 `String()` 渲染（逐符号复核 + 反射单测 `TestShapesFaceCarriesNoRendering` 作机械半）；两模块同名 `Wrap<T>` 的单测（`TestShapeDeclIdentitySeparatesModules`：同形不同模块的两个 `Shape` 不相等，且**用户声明不出口名字**——`ShapeDecl` 恰好一个非零，用户带节点、内建带名字）
- [x] 负断言：零 AST 注记、零新诊断码、零 checker 重构
  来源：design D1 边界
  验证：`git diff --stat internal/ast/` 为空；`git diff docs/spec/diagnostics.toml` 为空；`go test -count=1 ./internal/conformance/` 810 枚全绿（既有诊断路径零改写的证据）

## T2 实例化表与名字改编

- [x] 改编器 + `{声明键, 实参 Shape[]} → 符号` 实例化表；**无类型实参时符号逐字节不变**
  来源：design D2
  验证：`go test ./internal/codegen/` 329 枚全绿（新增 8 枚）；`go test -count=1 ./internal/conformance/` 810 枚全绿；**零黄金改动**（`git status internal/conformance/testdata/` 干净）。**「零漂移」不是「新路没人走」**——`sym()` 改形为「键 + 名 + 后缀」后是每条 `define`/`call` 的必经之路（14 处调用点），空后缀的退化态即今日字节，故整个既有快照面就是见证；**突变复验**：给 `sym()` 恒定追加 `$` → `internal/codegen` 当场 42 处失败（面是敏感的，不是恰好没覆盖）。表以符号为键、重复登记以 `Shape.Equal` 复核实参，理由见 D2 实现期补记三第 4 条
- [x] 参数编码逐形落单测：`Int64`/`String`/`Bool`/`Float64` 直用其名、`List<Int64>`→`List$Int64`、`Wrap<Point>`→`Wrap$Point`、嵌套递归 `$`、元组 `T$<n>$<elems>`、fn 型 `F$<params>_<rets>`、`Unit`→`U`、`Never`→`N`
  来源：design D2 参数编码
  验证：`TestMangleShapeForms` 18 形逐形断言，每形另断字符集 `^[-a-zA-Z$._][-a-zA-Z$._0-9]*$`（LLVM 裸标识符的许可集；边界经真机取数：`$`/`-`/`.`/`_` 无引号可用，`#`/`%`/`@` 被 clang 拒——后者作为**对照组**写进了下一条的端到端单测）；**与任务书唯一不符处**：用户声明作参数编**限定键**而非裸名（`a.Wrap$a.Point`，不是 `Wrap$Point`）——理由是裸名即 D1 所拒的跨模块碰撞，**订正与测试见 D2 实现期补记三第 1 条**；另补 D2 未列的三臂（`Dyn$…`、无返回 fn 的 `_V`、未解位置 panic），见补记三第 3 条。**LLVM 裸标识符合法性**：`TestMangledSymbolsLinkThroughClang` 取改编器的真实输出（`main.id$main.Wrap$main.Point`、元组形、fn 形、`Dyn$Iterator$Int64`）拼进最小 IR，过 clang 21.1.8 编译、链接、执行取数，真机绿（无 clang 时 skip）——这是字符集合法的**直接**证据。**「`we build` 端到端 exit 0」在 T2 阶段的可达形**：既有 clang 端到端面（`internal/cli` build 测试 + conformance 810 枚）全绿；T2 不改发射路径、无站点被驱动，故**没有实例化符号进 IR**，实例化的端到端随 T3/T4 的发射一起到（见补记三末段）
- [x] 负断言：改编串不以 `String()` 渲染或 hash 为源
  来源：design D2（「为什么不用渲染串/hash」）
  验证：`TestMangleSeparatesSameNamedDeclarations`——手工构造两个模块各自的 `Point`，同一个 `Wrap` 应用于二者：`a.Wrap$a.Point` ≠ `a.Wrap$b.Point`（裸名读法下二者同串，正是 D1 拒绝渲染串的那次碰撞）；同测断言串里**拼着两个声明的名字**（人眼可辨是哪个实例化，非 hash）。**另有一枚机械半**：`TestMangleCheckerShapes` 拿真检查器在一个会实例化两次的程序上的登记表，逐个站点喂改编器——既证约定（无未解位置漏进参数位），也证真实来源（ctor 的绑定编成 `$main.Point`，即本模块自己的声明节点，不是同名串）

## T3 逐实例化发射的驱动与次序

- [x] 惰性工作表 + 不动点；定义文本进 `fnsDone`，顺序取**登记次序**
  来源：design D3
  验证：递归实例化单测（`fn f<T>(x: Box<T>)` 内调 `g<T>`）绿；同源两次构建 IR **逐字节相同**（确定性证据）
  **落地记**：单测面 `TestRecursiveInstantiationRegistersInOrder` 绿（注册次序 + 定义次序 + 槽符号三断言）；`TestInstantiationEmissionIsDeterministic` 8 次重复发射 IR 逐字节相同；CLI 面两次独立 `we build` 产物 SHA256 相同（`6c92d0e3…1400ac`）。**任务书的递归形只在 AST 层可达**——检查器不解析显式类型实参位上的类型参数（`g<T>` 报 `E1304`），端到端见证改用检查器合法的传递形（单态 `f` 调 `g<Int64>`，真机 exit 0 打印 `7`），**订正与披露见 D3 实现期补记四第 1、7 条**；不动点实为**两处**（fn 定义段 + `render` 的 order 走查），见补记四第 3 条；本任务零黄金改动（推断面未被驱动，`test-fn-generic-bnd` 逐字节不变是要求），拓宽面黄金归 T4
- [x] **上界守卫（硬要求）**：实参深度超过声明的类型参数深度 + 常数即报实现内部错误，不得无限循环
  来源：design D3 边界
  验证：负例——自我加深的实例化（`f<Box<T>>` 内调 `f<T>`）产出实现内部错误而非挂死；**突变复验**——撤销守卫后同例挂死（单点突变、锚点先断言 `count==1`）
  **落地记**：两条负例各钉一处守卫——`TestSelfDeepeningInstantiationIsAnInternalError`（过声明位，任务书点名的形）与 `TestDeepeningTupleInstantiationIsAnInternalError`（元组形，只经 `instFn`）；界 = 参数个数 + `instDepthLimit(32)`。**突变复验**：挖空 `instDepthCheck` 体（单点突变、锚点 `count==1`）→ 自我加深那枚**挂死**（`panic: test timed out after 20s`，栈底 `emitCall→emitFnDefine→EmitProgram`），同包其余 339 枚仍绿——守卫承重且其测试敏感；还原后全绿。**fixture 形**（无参、返回 unit、体单条 ExprStmt 调用）与 `instFn` 处守卫的必要性见 D3 实现期补记四第 5 条

## T4 泛型单态化端到端

- [x] **红先行（两件事，别混）**：① 固化 characterization——14 枚边界黄金**今日全绿**（它们的期望里就写着 exit 70 + 边界行，是今日行为的钉），动代码前先以 `-count=1` 跑一遍留档；② 每簇的目标测试是**新写的 widened 期望**（原形 + 目标的 exit/stdout），它在实现前必**红**——红态即「期望 widened 行为、实现仍停在边界行」的差
  来源：process §3.4（测试先行）；design D10-1
  验证：① `go test ./internal/conformance/ -count=1 -run 'TestGoldenCases/<14 枚>'` → 14/14 PASS（基线留档 `/tmp/we-b1b/red-baseline.txt`，2026-09-12 已取）；② 逐簇新期望先跑 → FAIL（stderr 报边界行 vs 期望 exit 0），实现后 → PASS
  **落地记**：① 动代码前取数，14 枚边界黄金在 HEAD `0030d60` 上 `-count=1` 一轮 **14/14 PASS**（留档 `/tmp/we-b1b/red-baseline.txt`，2026-09-12），今日行为的 characterization 已固化。② 新写的 widened 期望共 **10 枚**（9 枚 check 夹具孪生 + 1 枚洞面锚），逐枚在 HEAD `0030d60` 的真机二进制（`/tmp/we-b1b/we`）上**先红**：exit 一律 70，词面两形——7 枚 `bndGenericFns`（`generic-fn`/`-impl`/`-newtype`/`-where`/`-where-multi-bound`/`-where-equality`/洞面锚）与 3 枚 `bndMainBody`（`generic-record`/`generic-sum`/`nested-application`）；实现后逐枚转绿（exit 0，stdout 逐字节定）。红绿两侧由同一条脚本一轮产出（无手抄），逐枚的三元组留档 `/tmp/we-b1b/twins-evidence.json`（每项 = `[HEAD exit, stdout, stderr, 本树 exit, stdout, stderr]`）。**语料侧的对账**：翻绿后仍带边界行的黄金 14 枚（12 `bndMainBody` + 1 `bndFnBody` + 1 两塔共享的 `bndGenericFns`，后者见下条），即其余簇的诚实边界
- [x] `test-fn-generic-bnd`（`bndGenericFns` 的唯一黄金锚）翻绿重锚
  来源：design D0/D2；proposal 目标 3
  验证：先红后绿逐枚——HEAD `0030d60` 树 `we build` exit 70 + `bndGenericFns`；本树 run exit 0 且 stdout 逐字节定
  **落地记**：该黄金的源（`tests/m_test.we`：`fn id<T>(x: T) -> T` + `st.assertEqual(id(5), 5)`）在 HEAD `0030d60` 树上 `we test .` → exit 70 + `we: generic functions in code generation (monomorphization is the B-track codegen-full widening) are not implemented in this reference build yet`；本树 → exit 0 且 stdout 逐字节 = `pass  tests/m_test.we: generic (0ms)\ntotal 1, passed 1, failed 0 (0ms)`。**订正一处对记**：它并非语料里 `bndGenericFns` 的**唯一**锚——另一枚 `check-assertequal-residual-bnd`（`check` 面 exit 70、同一行字）是**两塔词面共享**（该词在 `typecheck.go:57` 与 `codegen.go:61` 各有一份定义，Eq 域闭包是 typecheck 塔的裁定），按 D0/D13 只记账不改动，本任务后仍是唯一残留；故本枚是 **codegen 侧**的唯一锚，翻绿后**零黄金以 codegen 的泛型词停住**
- [x] 泛型孪生黄金：`check-ch10-*` / `check-m6a-*` 中**可运行**的形逐面补 build/run 孪生
  来源：design D10-3
  验证：新增黄金逐枚先红后绿；**纯诊断面不补**——未补的逐条列名与理由入披露清单
  **落地记**：补孪生 **9 枚**（`run-ch10-{generic-fn,generic-impl,generic-newtype,generic-record,generic-sum,nested-application,where,where-multi-bound,where-equality}`，逐枚先红后绿，红态 exit 70 见上条）+ **1 枚洞面锚**（`run-ch10-generic-application-in-interp`，非任何 check 夹具的孪生，见 D3 实现期补记五第 5 条）。**未补 12 枚、逐条列名与理由入披露清单**（D3 补记五第 7 条）：3 枚仍停且不属本任务面（`associated-type` 停在 `bndFnBody` 的关联型返回面、`dyn` 属 T5/T6 的盒面、`derives` 无运行期路径）；5 枚（`if-comparison`/`default-method`/`interface-impl`/`pub-interface`/`inherent-mut`）在 HEAD 即 build 0 / run 0，**被红先行规则自证排除**（新黄金在 HEAD 必须红），其中三面已由 B1a 的 `run-method-dispatch` 覆盖；4 枚 `check-m6a-*` 中 3 枚纯诊断（E0501/E0808/E0830，check exit 1）、第 4 枚 `precise-map-override` 无 `main` 不可运行。**另**：泛型函数这一面随本任务从 `docs/benchmarks.md` + `.zh.md` 的「What remains closed」清单**下线**（按 T9 那一行的既定做法；清单的**清空**是 T14 的收口项）——见 D3 实现期补记五第 8 条
- [x] 负断言：`codegen.go` 的 4 处泛型真拒绝（`collectImpl:589`/`:599`、模块收集 `:1117`/`:1142`）放行后，**非泛型的非法形仍被拒**
  来源：design D0 边界
  验证：负例黄金/单测——非名义头（`collectImpl:594`）、重复 impl（~~E0807~~ **订正：E0809**，检查器侧）、`impl` 头带实参的非法形逐条仍拒
  **落地记**：三面逐面钉住——① 非名义头：新增 `TestNonNominalImplHeadStops`（限定名／元组／unit 三形，逐形断言 `bndMainBody`；检查器侧先由 E0811 拒，已有 `check-e0811-tuple-head`/`-base-head` 两枚黄金锚）；② 重复 impl：码是 **E0809**（`typecheck.go:3500`/`:3504`），已有 `check-e0809-duplicate`/`-overlap` 两枚黄金锚（成员重名另是 E0814，四枚 `check-e0814-*`）；③ `impl` 头带实参：`TestImplHeadArgumentsMustBeTheParameters` 三子例逐例仍拒（`bndGenericFns`）。**「4 处」的归属订正**：模块收集那两处在 **T3** 已放行（不放行则没有泛型声明进得了表），本任务放行的是 `collectImpl` 的两处——见 D3 实现期补记五第 2 条；拒绝面本身是**收窄**（非名义头仍 `e.bnd()`，实参列表非法改走 `bndGeneric()`，见补记五第 3 条）。码内那句误引的 `(E0807)` 同批订正为 E0809/E0814

## T5 Dyn 盒的布局与 gc 集成

- [x] **阻塞前置**：查证 resource 类别进盒的所有权纪律（`dynFace:7095` 只查 E0819/E0820，**不查类别**）并补记结论
  来源：design D5 未决
  验证：真机探针——resource 类别的头实现接口后 `Dyn<I>(v)` 的 check/build/run 实测；结论写入 design D5，若属语言面则登记 follow-up 且本任务不预设结论
  **落地记**：四条直路逐条实测关死（直构 `Dyn<Describe>(r)`、经 gc 记录字段、经 newtype 底层、经 sum 载荷，check 一律 exit 1 + E1106，措辞入 D5 补记六第 1 条），**但第五条路是活的**：`fn boxed<T>(v: T) -> Dyn<Describe> where T: Describe` 加 `boxed(f)`（`f` 为 byres 记录）→ check 0 / build 0，盒照发。根因在检查器：`resource.go:431-440` 的 Call 臂对**任何**调用都置 `resKilled`，从不核对被调形参是否声明了该资源类型（E1104 措辞里 "pass it to a fn whose parameter declares the type" 是未兑现的判据）。**属语言面 ⇒ 登记 roadmap #24，本任务不动检查器、不预设结论**。缺口独立于盒：`fn width<T>(v: T)` 加 `width(f)`（无 `Dyn`）在**父树**上就是 check 0 / build 0 且释放被省；T5 只是让装箱这一支可达。**具体形参的纪律是硬的**：`fn width(v: FileHandle)` 是 E1104
- [x] 盒形 `{map@0, size@8, vtbl@16, 载荷@24…}` + `@.dynmap<n>` 按载荷面去重 + 全标量载荷 map 字写 `null`
  来源：design D5 决策 1/3/4
  验证：IR 钉（`internal/codegen` 单测逐字节断言 alloc/store map/store vtbl/载荷序列）；标量载荷盒的 map 字为 `null`（`gc.c:174-176` 跳过内容的姿态）；载荷恰一字且为 gc 引用时描述符与 `@.fnmap` 同形（`[1 x i64] [i64 2]`）
  **落地记**：`internal/codegen/dyn_test.go` **十枚**，每面钉**整条指令序列**（`boxRun`/`wantRun`：alloc 尺寸 → map 字 → root_push → 偏移 16 的 vtbl 字 → 每个载荷字的偏移与**它从哪个寄存器读**，寄存器分 `%box`/`%r` 两类消解新生编号）。逐面实测：scalar `alloc 32` / map `null`；gc 记录 `alloc 32` / `@.dynmap0` / 载荷 `store ptr %r` 于 24；String `alloc 40` / map `null` / 两字于 24、32；sum `alloc 48` / map `null` / 三字；`where` 位解到实参后面同。**去重**：四枚构造（三 gc 记录 + 一 scalar）⇒ 恰一个 `@.dynmap0`，三个 gc 盒各命名它一次。**同形锚**：`@.dynmap0 = private unnamed_addr constant [1 x i64] [i64 2]`，与 `@.fnmap` 逐字节同形
- [x] 入盒协议同 `allocRecord`（alloc → store map → `__we_root_push` → vtbl → 载荷），`e.pushes++` 与 `dischargeRoots` 同账
  来源：design D5 决策 5
  验证：IR 钉断言指令顺序；根账单测（构造后 pop 平衡）；~~**gc 压测黄金**——盒载荷为 gc 句柄且循环分配越过 `GC_THRESHOLD`（`gc.c:250`）后仍可读~~ **订正：「仍可读」在 T5 内不可满足**——`Dyn<I>` 值的唯一读取是 T6 的方法派发（今天停在无 thunk）；T5 的 run 黄金只能观测流（构造 → 传参 → 返回 → 丢）。**黄金改为断言「盒在两次跨阈值收集之间存活且不破坏运行」**；~~**突变敏感**——撤盒描述符的描记位即红（黄金层捕获，非仅单测层）~~ **订正：撤描记位黄金层不捕获，假阳性方向才捕获**（实测见 D5 补记六第 4 条）
  **落地记**：协议逐条同 `allocRecord`——`allocRecord` 抽成薄壳，新 `allocObj(mapOp, total)` 承担 alloc → `store ptr <map>, ptr %box` → `__we_root_push(%box)`，`e.pushes++` 与 `dischargeRoots` 同账；盒在 `allocObj` 之后写 vtbl 字与载荷字，故实测 body 出口 `__we_root_pop` 数 == push 数（新黄金 `run-dyn-box-call-boundaries` 五种载荷面跨参数与返回面、`run-dyn-box-gc-crossing` 两枚面常驻）。**新增黄金两枚，逐枚父树先红**（`we-parent` = 父提交 `0498b09`（T4 落地后的 HEAD，2026-09-13 复测）：`run-dyn-box-call-boundaries` exit 70 + `bndFnBody`、`run-dyn-box-gc-crossing` exit 70 + `bndMainBody`；本树 exit 0）。**gc 压测取证**：临时给 `gc.c` 的 `__we_gc_collect` 加 stderr 计数（跑完 `git checkout` 还原、`git status` 复核干净）实测 `run-dyn-box-gc-crossing` 的源——120000 轮 × 24B ≈ 2.9MB 触发**两次收集**（swept=43687、43691），程序 exit 0、stdout 逐字节 `7199940000`。**突变两枚逐枚判决**（单点、各自还原、锚点先断言 `count==1`）：**M-t5-a**（永远有描述符 + 每个字都描记）**黄金层与单测层双双捕获**（黄金层：`we: build/demo terminated by signal`，收集器把标量载荷 `5` 当块指针 → SIGSEGV）；**M-t5-b**（恒返回 `""`，撤掉全部描记位）**单测层四枚红、黄金层静默**（exit 0、stdout 逐字节不变）——载荷的独立构造根使假阴性在 T5 无从观测，该方向的黄金层捕获随 T6 落地
- [x] 负断言：盒的 ABI 是 `abiGc`（一个字），`fitAbi` **零新臂**；盒内**无类型标签**
  来源：design D5 决策 2/6
  验证：逐行复核 `fitAbi` 无新增臂；盒作参数与返回的 run 黄金绿；盒形单测断言 offset 16 是表指针、**无 tag 字**（`Dyn` 无下转型面）
  **落地记**：`fitAbi` **零改动**（`git diff` 里该函数零命中）——`classType` 是唯一分类权威，Dyn 分支返回 `(abiGc, "", true)` 即满足本项；空 key 顺带让盒**共享不复制**（`recKey == ""` 时 `e.records[""]` 查不到，`emitOwnedRecord` 不被触发）。**无 tag 字**：每面实测偏移 16 恒为 `store ptr null`（指针型、T6 的接缝），标量面盒尺寸恰 32 = 24+8，连放 tag 的空间都没有；单测逐面断言该序列。`run-dyn-box-call-boundaries` 本树 exit 0、stdout `5`（盒跨 `makePoint`/`makeCelsius`/`makeTag` 三枚返回面与 `pick`/`width` 两枚参数面）

**另披露（不在本任务面内，由本任务的探针照亮并一并修掉的一处真缺陷）**：**泛型体内推断应用的实参是开位置**——语料 `fn twice<T>(x: T) -> T { let a = id(x) return id(a) }`（**无任何 `Dyn`**）在父提交 `0498b09` 上不是停在边界而是**崩溃**：`panic: codegen: mangling an unresolved type position`（`mangle.go:117` ← `mangleList` ← `mangleSuffix`）。检查器只走一遍泛型体，`noteApply` 记下的是开位置 `ShapeParam`，而发射面上没有 shape 替换，`siteArgsFrom` 把开位置直接喂给名字改编层。修法：新增 `instShape`（`inst.go`，按 `instEnv` 把位置解到实参，解不动原样返回）并在 `siteArgsFrom` 内逐位置应用；锚 = `TestInferredApplicationInsideAGenericBodyResolves`（`define i64 @main.twice$Int64(` 与 `define i64 @main.id$Int64(` 齐出；父树为上述 panic）。**同族的一条既有边界不随之放开**：泛型体内写出类型实参（`id<T>(v)`）仍是 E1304（`"T" is held by no scope`）。见 design D5 补记六第 5 条

**T5 提交三枚**（英文正文、无署名 trailer、不用 `--no-verify`）：`3023d68` 泛型体内推断应用的实参解位（`inst.go` + `inst_test.go`）；`2495d0d` Dyn 盒的发射（`codegen.go` + `dyn_test.go` + 两枚黄金）；`bab0abb` roadmap #24。`openspec/changes/codegen-full/` 按惯例不提交。**验证阶梯**（2026-09-13，三提交落地后的树）：`go build ./...` ok、`go vet ./...` ok、`gofmt -l` 空、`go test -count=1 ./...` **13 包全 ok**（conformance 119.362s、benchmarks 31.140s、runtime 17.621s、codegen 0.386s）、`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`、`docs_sync.py` → `33 pair(s) aligned`、`git diff --check` 空；语料 **820 → 822**、`internal/codegen` 顶层单测 **349 → 360**（+10 盒 + 1 泛型体内应用）。**首枚单独复验**：`git worktree` 于 `3023d68`（`go build ./...` + `go test -count=1 ./internal/codegen/` ok，worktree 已移除）；后两枚的树即阶梯所验之树。

## T6 vtable 与动态派发

- [x] 配对表扩形为 `{ifaceKey, ifaceArgs} → heads`（今日 `collectImpl:609` 的 `len(iface.Args) == 0` 把带参接口 impl 整体排除）
  来源：design D6 事实
  验证：单测——`impl Iterator<Int64> for Cursor` **进入**配对表（今日不入，红→绿）；无参接口的既有配对零回归
  **落地记**（提交 `8a08ef3`）：`collectImpl` 的 `len(iface.Args) == 0` 早退整个撤掉，改由 `ifacePairKey`（`codegen.go`）统一判：`{ifaceKey, ifaceArgs} → heads`，键 = `mangleApply(declKey(iface.Decl), args)`（`Iterator$Int64` / `main.Seq$Int64` / 裸 `Show`）。**判据不是「带不带参」而是「是不是本程序声明的接口」**——模块声明的接口键前缀 `main.`，检查器内建的面（`Iterator`/`Iterable`/`Show`）无源文件故裸名；walked 模块不持有的接口（跨模块 `other.Seq`）不产行。行上带 `args`（`[]typecheck.Shape`），供默认体实例化与 thunk 替换用。红先证据：`TestParameterizedImplEntersThePairingTable` 在父树红（表里无行）本树绿；无参接口既有配对零回归（`TestPairingKeyNamesTheInterfaceFace` 五例 + 全语料）。
- [x] vtable 全局 `@.vt.<接口键>$<实参改编>.<具体型改编>`（`private unnamed_addr constant [n x ptr]`，槽序 = 非默认方法声明序）+ thunk（`(ptr %box, 参数…)` → 静态调用 impl 符号）
  来源：design D6 决策 1/3/4
  验证：IR 钉两枚（vtable 全局的形、thunk 的形）；thunk 的转发目标与 `fnDef.sym():737` 逐符号一致；`Iterator<T>` 的槽数恰为 1
  **落地记**（提交 `708b148` + `76dd8c8`）：**按需在盒构造点发射**，不走配对表——发射条件 = 载荷面为**一个 gc 句柄**（`abiGc`）**且**接口槽数 ≥ 1；其余载荷面（scalar/String/sum）保持 T5 的 `store ptr null` @16，逐字节零漂移。槽序取 `Face.Slots`（检查器注册的**非默认方法**声明序）；`Iterator<T>` 实测恰 1 槽（`next` 是唯一 `body: nil`，map/filter/take/skip/collect/fold/reduce/count/any/all/find 皆内在默认体）——`TestIteratorFaceCarriesOneSlot` 钉死。thunk 形与 `fnAdapter:4623` 同法：`define internal <retTyp> <thunk>(ptr %box, <转发字…>)`，体 `%addr = getelementptr i8, ptr %box, i64 24` → `%recv = load ptr, ptr %addr` → `call <retTyp> @<fd.sym()>(ptr %recv, <转发字>)`；String 参数展开两字、sum 三字（经 `abiWordTypes`）。**`refOfShape` 拼写修复是本项的前置**：`Iterator<Int64>.next` 返回 `Option<Int64>`，而该应用只经替换到达分类器，改编名 `Option$Int64` 永不命中 `classType`——修前 `slotAbi` 无从分类，修后 `@.vt.Iterator$Int64.main.CountIter` 才出得来。**`sameAbi(slotAbi, classify(fd))` 是表整体或不出**（有洞的表 = 调地址零）。三枚 `bndFn()` 改 `e.bnd()`（停点在 body 里不在 fn 体里）。
- [x] 派发点：`gepLoadPtr(box, 16)` → GEP 到槽 → `load ptr` → `emitCallCore`，接收者经 `preOps` 以 `"ptr " + reg` 传入；`abiSum` 返回（`{i64,i64,i64}`）原样穿过
  来源：design D6 决策 5
  验证：`Dyn<Iterator<Int64>>` 上 `next` 调用的 run 黄金（先红后绿）；`closure_test.go:197` 钉的既有间接调用 IR 形**零漂移**
  **落地记**（提交 `1128276`）：派发点是 **`emitCall` 的一条前置臂**（在 `recvKeyOf` 之前）——`dynRecvOf(fn.Recv)` 只在接收者是 `*ast.Ident` **且**其 `gcBinding.dyn != nil` 时作答；命中则 `emitDynCall`：`slot` 由 `ifaceSlots(face)` 按**方法名线性查**（表序即 `Face.Slots` 的声明序），`abi` 由 `slotAbi(slots[slot], ifaceArgs)` 在 `instShape` 后的实参上分类，链为 `table = gepLoadPtr(box, 16)` → `thunk = gepLoadPtr(table, 8*slot)` → `emitCallCore(abi, thunk, []string{"ptr " + box}, args)`——正是 `closure_test.go:197` 的形，只是表字在 16、槽在 8*slot。**未命中即静默落回**（返回 `false`），交给下面各面按自己的说法停：默认方法在盒上**没有槽**，名字不是接口方法也一样。**载荷面的 gc 性随值走，不随类型走**是本项的核心判据：`Dyn<I>` 只说派发走哪张脸，从不说载荷是什么，故 `dyn *typecheck.Shape` 随 `gcBinding`/`callResult` 走，**只在 `boxTable` 真发射过表时置位**（`boxTable` 第二返回改 `dispatchable bool`）；经参数/返回/字段抵达的盒不可判定 ⇒ 停而非跳。**`slotAbi` 分类失败 ⇒ `e.bnd()` 且 `is=true`**（本面已认领该名，不许下层再答）。**红先证据**（真机、`76dd8c8` worktree）：`TestDispatchReadsTheSlotOutOfTheBox` 与 `TestDispatchOnAGcPayloadBoxIsEmitted` 在父树红（`bndMainBody`）、本树绿；黄金 `run-dyn-vtable-dispatch` 在父树 exit 70 + `bndMainBody` 对 want exit 0，本树绿（`--- PASS: TestGoldenCases/run-dyn-vtable-dispatch`）；纯负例 `TestDispatchIsEmittedOnlyWhereATableWas` 两侧皆「停」，其判别力由突变 M1 证明（见下）。**零漂移**：`closure_test.go` 的间接调用形在父树与本树**逐字节相同**（探针 `drift` 的 `build/drift.ll` diff 为空，形为 `gep+16` / `load` / `gep+24` / `load` / `call i64 %v(ptr %v, i64 2)`）；**19 枚探针（`p1`–`p19`）逐枚比对**：11 枚 IR 逐字节相同、7 枚两侧停在同一句、**唯一行为移动的是 `p1`**（`Dyn<Iterator<Int64>>` 上 `next`，父树 `bndMainBody` → 本树 exit 0）——即派发点自身。**突变三枚**（单点、逐枚还原、先断言 `count==1`）：M1「恒置 `dispatchable`」单测层红（`TestDispatch...OnlyWhereATableWas`）；M2「槽下标恒 0」单测层红（`TestDispatchTakesTheSlotAtItsOwnIndex`，本项为此新增的第四枚钉）；M3「接收者传表不传盒」单测层三枚红 **+ 黄金层红**（`run-dyn-vtable-dispatch` → `terminated by signal`，exit 1）。**黄金层盲区两枚**：M1 与 M2 黄金层均**静默**——理由分别是「语料无非 gc 载荷盒上的方法调用」与「唯一的派发黄金其脸恰 1 槽」，两条并入 T6-4 的补黄金清单（见 T6-4 行）。
- [x] 负断言：vtable 永远静态（不经 `__we_alloc`）、不做多接口合并/接口继承/下转型
  来源：design D6 边界
  验证：vtable 产出路径的 `__we_alloc` 命中数为 **0**（grep + 单测）；`grep -c "dynType\|Dyn" internal/codegen/` 只余注释与实现点，无运行期类型标签机制
  **本项继承的补黄金清单三条**（T5 一条 + T6-3 两条，均为**单测层捕获、黄金层静默**的假阴性方向，须逐条补黄金或显式维持披露）：①T5 的 M-t5-b（撤盒描述符的全部描记位，断言「盒在跨阈值收集后仍可读」的形）；②T6-3 的 M1（恒置 `dispatchable`：需要一枚**非 gc 载荷盒上的方法调用**黄金，形为 `Dyn<Describe>(Celsius(1))` + `d.describe()`，钉「exit 70 + 边界词」——本树停在上层，突变后 emit 出调地址零的 IR，故该黄金能杀它）；③T6-3 的 M2（槽下标恒 0：需要一枚**多槽脸**的派发 run 黄金，形为 `Shape`（2 槽，`area` 在第 0、`sides` 在第 1）上调 `d.sides()`——现有唯一派发黄金的脸恰 1 槽，故下标的对错在里面不可见）。
  **落地记**（提交 `3a23f01` + `2fbfc59`）：**负断言两条落地、第三条经查证不可言说**。①表恒静态：`TestVtableIsStaticAndNeverAllocated` 先钉表的常量形（`@.vt.Iterator$Int64.main.CountIter = private unnamed_addr constant [1 x ptr] [...]`），再逐行扫 `@.vt.` 断言无一行含 `__we_alloc`，另以 `dispatchAllocRe`（`= call [^\n]*@\.vt\.`）断言**没有任何 call 的产物是一个表名**。实现面佐证：`grep -c '__we_alloc' internal/codegen/codegen.go` = 9，逐条为 declare（:77）、闭包环境（:4270/4274）、fn 载体（:4735/4739）、fn 环境（:8329/8332）——**vtable 章节（`boxTable`→`ifaceSlots`→`vtableFor`→`slotAbi`→`emitVtableThunk`→`dynRecvOf`→`emitDynCall`）命中 0**。②不做多接口合并：`TestTwoFacesOverOneHeadKeepSeparateTables` 钉同一 record 在两个脸下各得一张表且各只含自己的槽；③**不做接口继承在本语言不可言说**——`ast.InterfaceDecl`（`ast.go:217`）字段为 `Pub/Name/TypeParams/Assocs/Methods/Line/Col/NameLine/NameCol`，**无超类型位**，语法上写不出继承；下转型同理（无语法），其背面即 T5 钉死的「盒无 tag 字」。**运行期类型标签**：`grep -c 'dynType'` 在 `internal/codegen/` = **0**（仅 `internal/typecheck/{shape,typecheck,shape_test}.go` 各有命中，那是检查器自己的分类）；`internal/codegen/` 非测试代码的 `Dyn` 拼写共 18 处（`inst.go` 4 处 `case typecheck.ShapeNominal, typecheck.ShapeDyn:`、`mangle.go` 3 处改编名、`codegen.go` 11 处 = 9 处注释 + `id.Name == "Dyn"` 名测 2 处 + `emitDynCall` 定义），**无一处是运行期判别**。
  **三枚补黄金逐条落地**：②→`build-bnd-dyn-value-payload-dispatch`（exit 70 + `bndMainBody`）；③→`run-dyn-vtable-two-slots`（stdout `4\n5\n`）；另加一枚 `run-dyn-vtable-two-faces`（`1\n2\n`，合并突变 M4 的黄金层捕获）。①→**改 `run-dyn-box-gc-crossing`**：T5 那版只能断言「盒在两次跨阈值收集间存活」（`Dyn<I>` 值在 T6-3 前无读取），现补 `let r = held.next()` + `match`，读回载荷自己的值 `7`（源另留 T5 的标量载荷盒 `cold` 不动），stdout `7199940000\n7\n`——**即 T5 想写而写不出的那句断言**。**突变判决四条**（单点、逐枚还原、先断言 `count==1`）：M1（恒 `dispatchable`）**黄金层红**（`build-bnd-dyn-value-payload-dispatch`）；M2（槽下标恒 0）**黄金层红**（`run-dyn-vtable-two-slots`）；M4（`name := "@.vt." + headKey`，按头合并表）**单测层红 + 黄金层红**（`run-dyn-vtable-two-faces` → `1\n1\n` 对 `1\n2\n`，**静默错值**正是该守卫要防的形，不是崩溃）；**M-t5-b（`dynMapName` 恒返 `""`）仍黄金层静默——且经查证是本 build 不可观测，不是覆盖缺口**，理由见下条披露 8。
  **T6-4 另带一处拓宽**（提交 `2fbfc59`，非负断言面，是本项过程中查出的 T6-3 自身的洞）：**派发结果的分类**。`callStrKind` 读成员调用的返回族时经 `recvKeyOf(fn.Recv)`，而盒没有 record 键，故 `d.area() + 1` 得 `skNone` 而停——**值可绑可打印（那些消费者会在值落到的域重查），唯独不能作操作数**（binop 在发射任一侧前先问两侧 kind）。修法是把发射端已有的答案换个读法：槽的 ABI 在 `instShape` 后的实参上分类，与 thunk 同源，故其返回族即调用的值。`emitDynCall` 重写到两个新助手 `dynSlotIndex`/`dynSlotAbi` 上，臂与派发**共用同一次查表**——调用点与分类器不可能对「这个名字落在哪个方法」得出不同结论。**红先证据**（真机、`3a23f01` worktree）：`TestDispatchResultJoinsTheArithmeticDomain` 红（`bndMainBody`）、黄金 `run-dyn-vtable-arithmetic` 红（exit 70 对 want 0，stdout `""` 对 `"5\n"`），本树双绿。**零漂移**：11 枚非 dyn 探针 IR 与 T6-3 树逐字节相同；语料 826 → 827（唯一新增即该黄金本身），其余 826 枚零改动。

**T6-1/T6-2 披露四条**（每条都已实证，逐条写明而不静默修正）

1. **design D6 决策 4 的「scalar 面直传值」被实现推翻**。D6 原文（`design.md:237`）写 thunk「按载荷面从盒取接收者（**gc 面直传句柄、scalar 面直传值**）」。实测方法 define 的接收者**恒是 `ptr %self`**，与头是什么无关：`define { ptr, i64 } @main.Celsius.describe(ptr %self)`、`define i64 @main.Point.describe(ptr %self)`。scalar/String/sum 载荷的「接收者作为一个值」在本 build **没有确立的读法**（newtype 接收者本身也不产派发，见 `recvKeyOf`）。故 T6-2 的表只在 gc 载荷面发射，三面保留 null 表字——**这是发射能力之界，不是分类器之界**：标量背后挂接口合法且已被语料钉住（盒是跨边界传的值，派发与否都在），若为守住一条无人走的路而拒盒，反而把已落地的面收窄。表字买的是**可派发性**，盒自己的 IR 是它停止免费的地方。
2. **非 gc 载荷盒的派发在本 build 未定义**。同上：盒保留 null 表字，语料只钉「**可携带**」不定「可派发」。T6-3 的派发点因此只在 gc 面成立，须在 T6-3 落地记里重申。
3. **`refOfShape` 拼写形是一次 widening**。原先在替换位停住的程序现在能 emit（`fn first<T>(x:T)->Int64` 喂 `Option<Int64>`：父树 exit 70 → 本树 exit 0，`TestSubstitutedPreludeSumReachesTheClassifier` 先红后绿）。**语料零漂移**（822 不变），但**面外无覆盖**——该 widening 只由两枚新单测守门，无黄金。
4. **内建 `Iterator<Rune>` 句柄作盒载荷仍是边界**。`Dyn<Iterator<Rune> >("abc".iterator())` 停在 `boxFace`（`classType` 无内建迭代器臂），非本任务面；且裸 `"abc".iterator()` 本身即 T7 边界。T6-2 的探针用**用户 record 头实现内建面**（`impl Iterator<Int64> for CountIter`）绕开它。

**T6-3/T6-4 披露五条**（同上，逐条实证）

5. **派发点的可达面比 `Dyn<I>` 类型窄**。这是 T6-2 披露 2 的正身，也是本项唯一的设计级判据：`dyn` 随**值**走而不随**类型**走，故可达的只有「**在当前体内构造的盒**」。一枚盒经参数、返回、字段抵达时，静态上不知其载荷面，也就不知偏移 16 是表还是 T5 留下的 null——**停而非跳**。这不是权宜之计而是一条自证的界：`E0812`（`mut self receiver on a value-category type`）独立证明 value-category newtype 连 `Iterator<T>` 都实现不了，且 newtype 方法体内读 `self.value` 也停在既有边界——非记录接收者在本 build 本就无确立读法（T6-2 披露 1）。**后果**：`Dyn<I>` 作参数/返回值在签名面是合法的（T5 已钉「可携带」），只是拿到手里不能派发。**实证**（探针 `/tmp/t63/p-def`，本树二进制）：`fn width(d: Dyn<Shape>) -> Int64 { return d.area() }` + `width(d)` → exit 70 + `bndFnBody`。**无黄金覆盖此面**（属 T6-4 清单 ②）。
6. **默认方法在盒上没有槽，因此不派发**。`ifaceSlots` 只收 `body: nil` 的方法（D6 决策 1），故 `Dyn<Shape>` 上 `d.name()`（`name` 有默认体）落回下层各面并停；`Dyn<Shape>` 上 `d.sides()` 才走槽 1。这不是遗漏：默认体已按头实例化（`@main.Square.name`），而盒不记住头，无从选到该实例。**实证**：同一探针把 `d.area()` 换成 `d.name()` → exit 70 + `bndMainBody`（可观测的只是「停」；「无槽」是代码路径，外部无法与「恰好撞上别的边界」区分）。**未钉**——无黄金、无单测钉这条否定；且若误把默认体也算进槽，`name` 会落到槽 1 即 `sides` 的 thunk 上，是一枚**不会被现有语料发现**的错渲染。
7. **`slotAbi` 分类失败时本面认领该名**（`emitDynCall` 返 `is=true` + `e.bnd()`），不让下层以别的理由作答。理由：名字既已确定为该脸的一个槽，正确的停止理由就是「这个槽的 ABI 分类不了」，而不是恰好先撞上的另一条边界——诊断指错地方比诊断不出更坏。**未钉**（无引发该路径的语料）。
8. **T5 的 M-t5-b 在本 build 不可观测，不是覆盖缺口**（T6-4 查证结论，取代 T5 记录的「该方向的黄金层捕获随 T6 落地」）。撤销盒描述符的全部描记位后，黄金层**仍然静默**，且理由与「写了哪些程序」无关：**可派发的盒与载荷被独立扎根的盒是同一个集合**。三步推理——①派发要求 `gcBinding.dyn != nil`，而 `dyn` 只在 `boxTable` 真发射过表时置位，表只在**本体内构造的盒**上发射，故可派发 ⇒ 盒在本体内构造；②盒的载荷也在同一体内构造（`emitBox` 的实参），而 `allocRecord` 对载荷**自己 `__we_root_push`**（实测 IR，探针 `/tmp/t63/p-b1b`：`%v0 = call ptr @__we_alloc(i64 32)` `store ptr @.dynmap0, ptr %v0` `call void @__we_root_push(ptr %v0)` … `%v2 = call ptr @__we_alloc(i64 24)` `store ptr @.map.main.Count, ptr %v2` **`call void @__we_root_push(ptr %v2)`** ← 载荷自己的根），该根与盒的根同在本体出口 `dischargeRoots` 才弹；③故描记位**从不是**某枚可派发载荷的唯一存续理由。**推论**：本 build 能发射的任何黄金都看不出这个差别——不是「还没写」，是**写不出**。故该方向从 T15 的突变电池清单里撤下（它已被 T15 行引用为「T5 的盒描述符位须被黄金层捕获」，该句据此订正）。
9. **派发结果的值不在算术域外时仍停**（T6-4 拓宽的界）。`callStrKind` 的新臂把槽的返回族交给 `baseStrKind`；若槽返回的是 sum（如 `Iterator<T>.next` 的 `Option<T>`）或 String，族仍是 `skNone`，故 `d.next() == 1` 这类消费仍停。这不是遗漏：sum 三字本就不在数值域，String 的判据一直以 `skStr` 为界分流。**未钉**（无一枚黄金走这条面）。

**T6-1 提交一枚 + T6-2 提交两枚**（英文正文、无署名 trailer、不用 `--no-verify`）：`8a08ef3` 配对表扩形（`codegen.go` + `vtable_test.go`）；`708b148` `refOfShape` 拼写修复（`inst.go` + `inst_test.go`）；`76dd8c8` vtable 全局 + thunk（`codegen.go` + `inst.go` 两枚 shape 助手 + `dyn_test.go` 一枚钉改 + `vtable_test.go` 三枚新测）。`openspec/changes/codegen-full/` 按惯例不提交。**验证阶梯**（2026-09-13，两枚提交落地后的树）：`go build ./...` ok、`go vet ./...` ok、`gofmt -l` 空、`go test -count=1 ./...` **15 包全 ok**（conformance 115.108s、benchmarks 33.155s、runtime 19.935s、codegen 0.365s）、`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`、`docs_sync.py` → `33 pair(s) aligned`、`git diff --check` 空、staged 无 `refr/`；语料 **822 不变**、`internal/codegen` 顶层单测 **360 → 367**（+2 投影/分类器 + 4 vtable + 1 配对表）。**首枚单独复验**：`git worktree` 于 `708b148`（`go build ./...` + `go vet ./...` + `go test -count=1 ./...` 15 包全 ok，worktree 已复用为 T6-2 的红面）。
**T6-2 红先证据**（真机、`8a08ef3` worktree）：三枚新/改钉在父树红——`TestBoxGcPayloadDescriptorMatchesTheFnCarrier`（want `store ptr null`，实测 `store ptr @.vt.main.Describe.main.Point`）、`TestVtableGlobalHoldsTheSlotsInDeclarationOrder`、`TestVtableThunkForwardsToTheImplSymbol`；两枚为两侧皆绿的**事实钉**（`TestIteratorFaceCarriesOneSlot`、`TestVtableIsEmittedWhereABoxIsBuilt`）。`refOfShape` 两枚在 `8a08ef3` 红（`the reference reads "Option$Int64" at 0 arguments` / `boundary: function bodies beyond the M9b statement set`）、在 `708b148` 绿。

**T6-3 提交一枚**（英文正文、无署名 trailer、不用 `--no-verify`）：`1128276` 派发点（`codegen.go` + `vtable_test.go` + 新黄金 `run-dyn-vtable-dispatch.json`，3 files / +355 −9）。`openspec/changes/codegen-full/` 按惯例不提交，红面 worktree `/tmp/we-t63-red`（`76dd8c8`）用后即清。
**T6-3 红先证据**（真机、`76dd8c8` worktree，把本树 `vtable_test.go` 与新黄金拷入）：`TestDispatchReadsTheSlotOutOfTheBox` 与 `TestDispatchOnAGcPayloadBoxIsEmitted` 在父树红（`vtable_test.go:454/530`，逐字 `boundary: main bodies beyond the M9b statement set (...)`）、本树绿；`TestGoldenCases/run-dyn-vtable-dispatch` 在父树 `exit: want 0, got 70` / `stderr: we: main bodies beyond the M9b statement set (...)` / `stdout: want "10\n11\nnone\n", got ""`，本树 `--- PASS`；`TestDispatchIsEmittedOnlyWhereATableWas` 是**纯负断言**（两侧皆「停」，本就不该先红），其判别力由突变 M1 单独证明——此为如实记录，不冒充红先。
**T6-4 提交两枚**（英文正文、无署名 trailer、不用 `--no-verify`）：`3a23f01` 负断言两枚 + 三枚补黄金 + 扩 `run-dyn-box-gc-crossing`（`vtable_test.go` + 4 枚黄金，5 files / +128 −2）；`2fbfc59` 派发结果分类（`codegen.go` + `vtable_test.go` + 黄金，3 files / +115 −8）。红面 worktree `/tmp/we-t64b-red`（`3a23f01`）用后即清。
**T6-4 验证阶梯**（2026-09-13，两枚落地后的树）：`go build ./...` ok、`go vet ./...` ok、`gofmt -l` 空、`go test -count=1 ./...` **15 包全 ok**（benchmarks 32.053s、cli 2.682s、codegen 0.310s、conformance 129.204s、runtime 18.076s）、`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`、`docs_sync.py` → `33 pair(s) aligned`、`git diff --check` 空、staged 无 `refr/`；语料 **823 → 827**（+4：`run-dyn-vtable-two-slots` / `run-dyn-vtable-two-faces` / `build-bnd-dyn-value-payload-dispatch` / `run-dyn-vtable-arithmetic`；`run-dyn-box-gc-crossing` 为改写不计入）、`internal/codegen` 顶层单测 **373 → 376**（+2 负断言 +1 分类器）。**T6 全节至此四项全绿。** **设计面的订正与披露落在 `design.md` 的「实现期补记七」**（九处：决策 4 被推翻 / 决策 2 的兑现 / 决策 1 的兑现 / 决策 3 的兑现与零漂移 / `refOfShape` 前置 widening / 派发可达面的界 / 默认方法无槽与 `slotAbi` 认领 / 补记六第 4 条第二项被推翻 / T6-4 的结果分类），四枚突变判决随之同处记录。
**T6-3 验证阶梯**（2026-09-13，本树）：`go build ./...` ok、`go vet ./...` ok、`gofmt -l` 空、`go test -count=1 ./...` **15 包全 ok**（benchmarks 36.095s、cli 3.537s、codegen 0.429s、conformance 121.544s、runtime 21.112s）、`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`、`docs_sync.py` → `33 pair(s) aligned`、`git diff --check` 空、staged 无 `refr/`；语料 **822 → 823**、`internal/codegen` 顶层单测 **369 → 373**（+4 派发钉；父树 369 系在本树同一测量法下重取，订正 T6-2 记录的 367——那是另一次 `-v` 输出的计数法，as-built 以本处为准）。**`closure_test.go` 零漂移**见 T6-3 落地记（逐字节 + 19 枚探针横向比对）。

## T7 迭代器协议与用户 Iterable

- [x] **阻塞前置**：查证内建迭代器类型（`ListIter`/`StrIter`/`RangeIter`）的载体——std 侧声明 + 内建 impl，还是 codegen 内建合成
  来源：design D7 决策 3 未决（`Iterator`/`Iterable` 本身是检查器内建 `ifaceInfo`、无源文件）
  验证：查证结论写入 design D7；据此**定一处**发射点（两处并存即违反「一类事实一个权威」）
- [x] `emitFor` 增 ~~`*ast.Call` 源~~ **用户 Iterable 源**的协议形分派（**订正见 T7-A 落地记·裁定一**：字面的 `*ast.Call` 源实测仍停）（今日经 `listFaceOf:5266` 的 `default` 落 `e.bnd()`）
  来源：design D7 决策 2
  验证：`build-bnd-list-iterable` 翻绿（先红后绿：HEAD 树 exit 70 + `bndMainBody`）
- [x] 协议循环复用现成三槽语义（`sumSlot` 的 `{tag,pay0,pay1}`、`armTagTest:6545`、`slotVariantIndex:6566`）；`iterator` 每次进 `for` **只调一次**（**收口见 T7-B 落地记**：前三项由 T7-A 在 `for` 面兑现，末项 `build-bnd-acute-user-iterable` 翻绿由 T7-B 在急性面兑现）
  来源：design D7 决策 4
  验证：`build-bnd-acute-user-iterable` 翻绿 + 新 run 黄金（元素序列逐字节、`break` 早退、空集零元素）；IR 中 `iterator` 调用点数为 **1**（快照语义钉）
- [x] 内建迭代器对象（决策 3 定形后）：载荷面决定 `@.dynmap`，供惰性四枚进盒
  来源：design D7 决策 3
  验证：IR 钉——`ListIter` 盒的载荷 {句柄（描）, index（不描）}；`gc` 压测绿
- [x] 负断言：三内联形（Range 计数 `:5023`、String 索引 `:5093`、List 快照 `:5153`）**字节不变**
  来源：design D7 决策 1
  验证：`git diff` 中 `emitForRange`/`emitForString`/`emitForList` 零改动；`for_test.go` 与既有 for 黄金零漂移

**T7-A 落地记（T7-1 阻塞前置 + 第 2 枚 + 第 5 枚；2026-09-14）**：T7 的五枚复选框**关三枚、留两枚**——第 1 枚（阻塞前置，补记八已收口）、第 2 枚、第 5 枚（负断言）已勾；第 3 枚（验证项含 `build-bnd-acute-user-iterable` 翻绿）与第 4 枚（内建迭代器对象）未勾，理由是验收形未全部兑现，不留虚勾。

**提交两枚**（英文正文、无署名 trailer、不用 `--no-verify`）：`7df9d4b` 协议臂（`codegen.go` + `iter_test.go` + 三枚陈旧单测重锚 + 两枚黄金，7 files / +603 −26）；`45db419` docs 双语运行面订正（2 files / +2 −2）。`openspec/changes/codegen-full/` 按惯例不提交。

**T7-A 红先证据**（真机、`2fbfc59` 树）：`build-bnd-list-iterable` 在父树 `exit: want 0, got 70` + `stderr: we: main bodies beyond the M9b statement set (…)`，本树 `exit 0` / `stdout ""` / `stderr ""`（**保名保源翻绿重锚**）；新改的派发钉 `TestListWalkYieldsToTheProtocolFace` 在摘掉 `emitFor` 协议臂后 `FAIL`（`boundary "main bodies beyond the M9b statement set (…)"`）、装上后 PASS——**单测层的先红后绿为真机实跑，非推断**。

**T7-A 验证阶梯**（2026-09-14，本树）：`go build ./...` ok、`go vet ./...` ok、`gofmt -l internal/ cmd/ runtime/` 空、`go test -count=1 ./...` **15 包全 ok**（benchmarks 33.622s、cli 3.621s、codegen 0.322s、conformance 113.037s、runtime 19.323s）、`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`、`docs_sync.py` → `33 pair(s) aligned`、`git diff --check` 空、staged 无 `refr/`；语料 **827 → 828**（+`run-for-user-iterable`；`build-bnd-list-iterable` 为翻绿重锚不计入）、`internal/codegen` 顶层单测 **376 → 384**（+8：七枚协议钉 + 一枚关联型解析钉，另有一枚废测删除、一枚负例改写）。

**T7-A 的验收面逐条**：① 「`iterator` 每次进 `for` 只调一次」——IR 中 `@main.Range2.iterator` 调用点 `countCall(ir,"ptr",…)==1`、`@main.CountIter.next` 恰 2 处（定义 + 一枚静态调用点）；② 「新 run 黄金（元素序列逐字节、`break` 早退、空集零元素）」——`run-for-user-iterable` 一枚覆盖三属性，真机 `0\n1\n2\nearly 0\ndone\n`，另以探针 `od -c` 复核字节为 `0 \n 1 \n 2 \n`；③ 「复用现成三槽语义」——head 用 `icmp eq i64 <tag>, slotVariantIndex(o.sum,"Some")`（**索引取自被调方签名携带的表**，见 design 补记九裁定二）、body 走 `bindArmWord`；④ 「三内联形字节不变」——`emitForRange`/`emitForString`/`emitForList` 函数体零改动，for 面既有黄金零漂移。

**T7-A 的披露（四条，全文见 design 补记九）**：① D7 的红证据引文指向 `docs/benchmarks.md` 一个**从不存在的面**（该文档零 `Iterable` 字样），已删除线订正，并以 `45db419` 把该面补进**已拓宽**句；② 补记八登记为「不在 T7 面内」的 `-> Iter` 普通绑定边界**被同一枚改动关闭**（`assocRet`），超出任务书的拓宽如实登记；③ 任务书第 2 枚的字面形（`*ast.Call` 源）**实测仍停**，验收由绑定源形的黄金兑现；④ 补记八边界表里「`r`（命名的 Range）——T7-2 一并收」**未兑现**，七行中只有用户 Iterable 一行翻转。

**T7 余下两枚的落点**：~~T7-B~~（急性面 `emitAcute:6011` 经 `:6023` 的 `listFaceOf(im.Recv)` 挡住用户 Iterable，`build-bnd-acute-user-iterable` 仍红——**已由 T7-B 关闭，见下**）与 T7-4（`ListIter`/`StrIter`/`RangeIter` 的 codegen 内建合成，补记八已定形）。

**T7-B 落地记（第 3 枚收口；2026-09-14）**：T7 的五枚复选框至此**关四枚**——第 4 枚（内建迭代器对象）仍留，理由同 T7-A（验收形未兑现，不留虚勾）。提交两枚（英文正文、无署名 trailer、不用 `--no-verify`）：`9045441`（`codegen.go` + `acute_test.go` + 一枚翻绿重锚 + 一枚新 run 黄金，4 files / +465 −40）与 `1c77757`（docs 双语运行面补登「`String` 元素源」面，2 files / +2 −2）。`openspec/changes/codegen-full/` 按惯例不提交。

**T7-B 红先证据**（真机、`45db419` 树的 `git worktree add --detach /tmp/t7b/pre-wt` 版编译器 `we-pre`）：同一份源在改动前 `we run` → `exit 70` + 全句 `bndMainBody`，改动后 → `exit 0` + 12 行 `-ok`；`build-bnd-acute-user-iterable` 在父树 `want 0, got 70`、本树 `exit 0` / `stdout ""` / `stderr ""`（**保名保源翻绿重锚**）。**单测层的先红后绿亦为真机实跑**：把新门合上（`emitAcute` 协议分支恒返 false）后，六枚正向协议钉中**五枚 FAIL**、两枚负例钉（String 载荷停、Call 源停）维持 PASS，还原后全绿。

**T7-B 验证阶梯**（2026-09-14，本树）：`go build ./...` ok、`go vet ./...` ok、`gofmt -l` 空、`go test -count=1 ./...` **15 包全 ok**（benchmarks 34.861s、cli 3.160s、codegen 0.415s、conformance 120.957s、runtime 20.010s）、`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`、`docs_sync.py` → `33 pair(s) aligned`、`git diff --check` 空、staged 无 `refr/`；语料 **828 → 829**（+`run-acute-user-iterable`；`build-bnd-acute-user-iterable` 为翻绿重锚不计入）、`internal/codegen` 顶层单测 **384 → 391**（+7 枚协议钉；原 `TestAcuteOverAUserIterableStops` 为保断言的更名，不入增减）。

**T7-B 的验收面逐条**：① 「`build-bnd-acute-user-iterable` 翻绿」——见上，真机先红后绿；② 「新 run 黄金（元素序列逐字节、空集零元素）」——`run-acute-user-iterable` 一枚覆盖六枚组合子 + 空句柄三枚，真机 `count=3 fold=3 any=false all=false emptycount=0` / `find=two` / `reduce=three` / `emptyreduce=none` / `emptyfind=none`，`stdout` 12 行逐字节入册；③ 「IR 中 `iterator` 调用点数为 **1**」——`countCall(ir,"ptr","main.Range2.iterator")==1`，且 `@main.CountIter.next` 恰 2 处（定义 + 一枚静态调用点）、全 IR 无 `__we_list_` 载体动词（钉在 `TestAcuteProtocolBuildsTheHandleOnce`）。

**T7-B 的披露（四条，全文见 design 补记十）**：① **预登记的 sketch 有一处简化偏差**——原计划给 `listWalk` 增 `proto` 字段，实作发现**不必要**：两形的 step 块本就同形（都是「载计数器 + 自增 + 回边」），故 `closeListWalk` 零改动即共用，`listWalk` 只改写 doc、字段一个没加。② `emitStep` 的无界计数器缺口**如实点名不掩盖**（协议 walk 的 pass 数无上界证明，keep unchecked add）。③ **`String` 载荷停且与 List 面一致**——真机逐形取数：`List<String>` 上 `xs.iterator().count()` 与 `for x in xs { io.println("tick") }`（元素**不被使用**）**皆** exit 70，同形的 `List<Int64>` 两枚皆 0，故停点是**源的元素面**；`protoElemFace` 对 `abiStr` 返 false 不是新不对称，而是与 List 载体面同一取舍。该面**此前未入 `docs/benchmarks.md` 的「仍关死」清单**（清单只写「`List` 迭代」可跑、未加限定），本窗口以 `1c77757` 补登为第六面——这是**探针发现的既有披露缺口**，非 T7-B 引入的边界。④ **一处陈旧单测重锚**：`TestAcuteOverAUserIterableStops` 更名为 `TestAcuteOverARecordThatIsNoIterableStops`（其夹具无 impl，断言仍真但名与 doc 失准）。

**T7-4 落地记（第 4 枚收口；2026-09-14）**：T7 的五枚复选框至此**五枚全绿**。提交一枚（英文正文、无署名 trailer、不用 `--no-verify`）：`26817db`（`codegen.go` + 新 `iterobj_test.go` + 五枚黄金 + docs 双语）。`openspec/changes/codegen-full/` 按惯例不提交。

**T7-4 红先证据（真机，双层）**：单测层——把本树 `codegen.go` 换成 `HEAD`（`1c77757`）的版本，五枚新测**四枚红**（`TestListIteratorObjectCarriesTheTracedHandle` / `HeadIsSynthetic` / `NextReadsThroughTheObject` / `ElementDomainPicksTheHead`，逐枚 `boundary "main bodies beyond the M9b statement set (…)"`），第五枚 `TestListIteratorObjectIsNotBuiltOnTheZeroCostPaths` 是**纯负断言**（两侧皆绿，本就不该先红）——其判别力由自身三条断言与突变 D 共同承担，不冒充红先。黄金层——同一 `HEAD` worktree（`/tmp/we-t74-red`，用后即清）拷入五枚黄金后：三枚正身 `exit: want 0, got 70` + 全句 `stderr`；两枚负例在父树**皆 70**，即**两侧同色**，处置分别如实登记——`-gc-elem` 的判别力由突变 C3 单独证明（单摘不响、同摘即红），`-string-elem` 则**判别力未经突变证明**：它的停点在**上游**的源元素面规则（`elemFaceOfType` 对两字元素返 false，`listFaceOf` 在绑定层即答 false），**不在本任务的码里**（突变 C3 后它仍 70 即为实证），故它是一枚**边界钉**——把 docs 新补的那一面（元素面为单字载体所不能容的内建源进盒为迭代器）钉进语料，而不是某一行码的守卫；证伪它需要动 `elemFaceOfType` 的 String 判据，波及 `for`/组合子全族，**本任务不做、如实记为未证伪的边界钉**。

**突变电池（真机五枚，逐枚还原）**：**A** 对象 `lead` true→false ⇒ **不可观测**（压测仍绿）；**B** 盒载荷 `traced` true→false ⇒ **不可观测**；**C** 单摘 `elem.gc` ⇒ **不可观测**（`List<Cell>` 的 kind 是 `skNone`，被第二道守卫接住）；**C3** 两道守卫**同摘** ⇒ **可观测**，`List<Cell>` 70 → 0，**被负例黄金捕获**（`exit: want 70, got 0`）；**D** 换 `next` 的 tag 极性（`1`↔`0`）⇒ **可观测且高声**，三枚正身黄金全红 + 单测 `TestListIteratorNextReadsThroughTheObject` 红。**A/B 的不可观测性不是覆盖缺口**——与补记七第 8 条同因（可派发的盒与载荷被独立扎根的盒是同一个集合），三步理由逐字见 design 补记十一第四条。

**T7-4 验证阶梯**（2026-09-14，本树）：`go build ./...` ok、`go vet ./...` ok、`gofmt -l` 空、`go test -count=1 ./...` **15 包全 ok**（benchmarks 32.515s、cli 3.067s、codegen 0.431s、conformance 115.108s、runtime 19.420s）、`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`、`docs_sync.py` → `33 pair(s) aligned`、`git diff --check` 空、staged 无 `refr/`；语料 **829 → 834**（+5：`run-builtin-iterator-box` / `run-builtin-iterator-box-gc-crossing` / `run-builtin-iterator-box-elements` / `build-bnd-builtin-iterator-string-elem` / `build-bnd-builtin-iterator-gc-elem`）、`internal/codegen` 顶层单测 **391 → 396**（+5）。

**T7-4 的验收面逐条**：① 「IR 钉——`ListIter` 盒的载荷 {句柄（描）, index（不描）}」——真机逐字 `@.dynmap0 = private unnamed_addr constant [1 x i64] [i64 1]`（对象：bit 0 = 偏移 16 的句柄）与 `@.dynmap1 = private unnamed_addr constant [1 x i64] [i64 2]`（盒：bit 1 = 偏移 24 的对象），构造序列「alloc 32 → 写 map → `__we_root_push` → 写 16 → 写 24」两遍；表 `@.vt.Iterator$Int64.ListIter$Int64 = private unnamed_addr constant [1 x ptr] [ptr @.vt.Iterator$Int64.ListIter$Int64.next]`；`next` 体走 `+16` 载句柄 / `+24` 载 index / `__we_list_len` / `icmp slt` / `__we_list_get` / 自增回写；thunk 走既有 `+24 load` 约定。五枚钉在 `iterobj_test.go`。② 「`gc` 压测绿」——跨 120000 次 `Cell` 分配（≈ 越过 `GC_THRESHOLD`）后仍逐字 `7199940000 / 100 / 200 / 300 / none`，exit 0，入黄金 `run-builtin-iterator-box-gc-crossing`。**两项皆按原句兑现，无删除线。**

**T7-4 边界图更新（真机，逐形取数）**：`Dyn<Iterator<Int64> >(xs.iterator())` **70 → 0**（含经表派发的 `next` 四次）；`Dyn<Iterator<Float64> >` / `Dyn<Iterator<Bool> >` 同形 70 → **0**（两枚合成头共存、共用同一对描述符——按面去重的实测）；`List<String>` 与 `List<Cell>` 进盒 **70/70 不变**；`let it = xs.iterator()` 裸绑定 70/70、`for x in xs` 0/0、`xs.iterator().count()` 0/0（零开销路径未动）。

**T7-4 的披露（八条，全文见 design 补记十一）**：① 对象形由 `emitVtableThunk` 的 `+24 load` 约定**唯一确定**（非偏好选择），故 `vtableFor`/`boxTable`/`emitVtableThunk`/`classify` **四者零改动**；② 挂点在 `emitBox` 而非 `classType`——`Site.Boxed` 对 `Dyn<Iterator<T>>(<内建源>.iterator())` 是**接口脸本身、不携带具体头键**，头键只能从实参表达式解析；③ `dynMapName` 增 `lead` 参数（对象偏移 16 是 list 句柄、**必须描记**），去重键以 `g`/`x` 起头，故盒与对象拿到两枚恰好差一位的 global；④ **两枚描述符位在本 build 不可观测**（突变 A/B 皆静默），与补记七第 8 条同因，**不冒充黄金层捕获**；⑤ 探针抓出一处**判据冗余**（`elem.gc` 与 `strKindName` 互为冗余，单摘不响、同摘即红），负例黄金盖的是**联合行为**；⑥ **`RangeIter`/`StrIter` 今日不可达**（前者检查器 `E0816`、后者 `Rune` 无 ABI 面 ⇒ `Option<Rune>` 不成立），本任务兑现的是 `ListIter` **一枚**——另两枚是**上游前提未开**，不是未写，且 `iterSourceOf` 是唯一分派点，前提一开即可补；⑦ 两条零开销路径**各自独立**绕开本处（`emitFor` 与 `emitAcute` 各挡一道，非同一捷径的两个出口）；⑧ docs 双语「已拓宽」两次→**三次** + 「仍关死」**补一面**（元素面为单字载体所不能容的内建源**进盒为迭代器**，与既有的「`String`-元素源作 `for`/组合子源」分列），并登记 T7-B 拓宽句的**措辞窄口径**（字面只点 `for`，未点急性面）为歧义、**本任务不改**。

## T8 组合子：从 6 枚到 11 枚

- [x] `collect` 归**急性第 7 枚**：边遍历边 `__we_list_push`，结果 `List<T>`，**与 Dyn 无关**
  来源：design D8 决策 1；proposal §规范锚点（`1100-iterables.md:46-88`）
  验证：run 黄金（List 元素逐个断言）；`acuteCombinator:5529-5532` 的家族注释订正（`collect` 移出惰性族）
  落地（`c1c8444`，2026-09-14）：码面 = `emitCollect`（`codegen.go:6689`——空载体 `__we_list_new(i64 0, traced)` 起手、常驻根、槽存指针、每轮 `__we_list_push(loadPtr(lst), w.word)`、增长后根替换对）+ `callResult.list *listElem` + `bindResult` ckGc 臂入 `listEnv`（后续 `for`/组合子与字面量绑定同环境直读）+ `loadPtr` 助手 + `acuteCombinator`/`emitAcuteLoop`/`want` 表三处归第七枚、家族注释改「seven eager … as against the four lazy」。
  验证实跑：①单测 4 枚（396→400）——`TestAcuteCollectBuildsTheCarrierWhileWalking`（钉空载体起手先于 snap、每轮从槽读载体、pop+push 根替换对、cexit 后槽读结果）、`TestAcuteCollectTracesTheGcElement`（钉 `list_new(i64 0, i64 1)`、`inttoptr`+`root_push` 先于 push、紧随双 pop）、`TestAcuteCollectResultWalksAgain`（钉第二走查 snap 的正是 cexit 的槽读寄存器）、`TestAcuteCollectTakesNoArgument`（带参负形 → bndMainBody）；②黄金 3 枚（834→837）——`run-acute-collect`（for 逐元素断言 + count/fold/再 collect 再 find + 空载体）、`run-acute-collect-user-iterable`（协议源 Gen{hi:100000} 增长穿越 GC 阈值：count 100000 / sum 4999950000）、`run-acute-collect-gc-crossing`（gc Cell 协议源收集后 120k 次分配再全量读回）；③红先：HEAD worktree 上 3 单测 + 3 黄金全红于 bndMainBody，负形两侧皆绿（设计如此）；④验证阶梯全绿（build/vet/gofmt/15 包/conformance/validate --strict/docs_sync 33 对/diff --check）。
  披露：①**逐元素根不可观测**——四代突变探针（s4big 压力 → align M=43686 精确对齐火点（插桩实证 sz=72 acc=1048584）→ align2/align3 二段 pad）全部输出逐字节正确；机理 = 存储即时性（陈旧句柄同调用内零分配落入有根有描记载体，其后每轮收集经描记遍历复活）+ 空闲表排水（每弹 24B）与再触发周期的芝诺竞速；插桩实证两构建清扫总数恰差 1 块（=A1）、火点活动根 4（正确）对 3（突变）。按 T7-4 先例**保留纪律**（gc.c 头部自陈协议、零成本），不冒充黄金层捕获；②`List<String>` 源仍停上游（`elemFaceOfType` 词汇表边界，负形真机确认）；③`acute_test.go` 头部注释六改七 + `TestAcuteOutsideTheSixStops` 注释里的「five lazy」**留给 T8-4** 统一订正（本提交不动，避免跨任务沾染）。
- [x] 惰性四枚（`map`/`filter`/`take`/`skip`）：接收者装箱 + 适配器对象 {源盒, 参数} + 装箱返回 `Dyn<Iterator<U>>`
  来源：design D8 决策 2
  验证：`build-bnd-acute-lazy` 翻绿（先红后绿）；链式黄金 `xs.iterator().map(f).take(2).collect()` 端到端 run 绿
  落地（`795bc93`，2026-09-14）：码面 = `lazyCombinator:6277` 四名表 + `emitAcute` 内联链臂（`:6310`——急性走查直接消费链盒，`emitAcuteLoop` 零改动复用）+ `emitLazy:11087`（源盒解析→四臂分派→`mkLazyNext`→`ifaceSlots`/`vtableFor`→`emitLazyObject:11190`）+ `mkLazyNext:11303`（`head+".next"` 幂等入 `e.methods`）+ `emitLazyNext:11348` 四模板（公共 dispatch/carrier/tail/elemCross 闭包；map 的 U 取 `e.siteOf(call)` 检查期记录并重造结果面；filter/test 回边、take 计数槽、skip drain/pass 双前缀）。适配器对象 32B：源盒@16、参数@24（map/filter 为 fn 载体、take/skip 为 i64 计数）；结果盒 `Dyn<Iterator<U>>` 表@16 指适配器 vtable、载荷@24 持适配器——链的归纳形是盒持盒持对象，每次派发都过表。
  验证实跑：①单测 9 枚（400→408，`lazy_test.go` 全走 `checkShapes` 真管道）——四枚模板各一（map 盒/适配器/载体/表槽、filter 回边结构、take 计数槽与三入边 phi、skip drain/pass）、Float64 U 双向位换、链持盒（take 适配器@16 持 map 盒非 list 对象）、急性走查经外层表且零 `list_snap`、String 源/gc 元素源两负形（bndMainBody）；②黄金 837→838——`build-bnd-acute-lazy` 翻绿重锚（保名保源，exit 0）+ `run-acute-lazy-chain` 新增（七段：map×10+take(2)+collect、skip(2)+collect、filter+count、map×2+skip(1)+take(2)+collect、Float64 map+collect、map+take(0)+collect 再 count=0，stdout 逐字节定）；③红先：HEAD worktree 上 7 枚正向单测红于 bndMainBody、两枚黄金 want 0/got 70（重锚枚的红态 = 新期望对旧实现），负形两侧皆绿（设计如此）；④验证阶梯全绿（build/vet/gofmt/15 包/validate --strict/docs_sync 33 对/diff --check）。
  披露：①**内联链急性随之可用**——`emitAcute` 的内联链臂让 `…map(f).count()` 等直接链式形今日即跑（任务书未单列，黄金七段已覆盖）；**绑定后过急性仍停**（`let it = …map(f); it.count()` → bndMainBody）——Dyn 接收者的急性路径登记为 T8-3（下方新增行），docs 关死清单已补该面；②**「五枚惰性」grep 归零随本提交兑现**——唯一残留在 `TestAcuteOutsideTheSixStops` 注释，而该测本身陈旧（裸 `Emit` 助手环境性绿 + 裸 List 主题实为检查器 std 模块词，真机实证 `xs.count()`/`xs.filter()` 皆停于 bndStdModules），随重锚整测退役，补记十二第四条预告的「T8-4 专项」因此蒸发，D8 的措辞义务（`codegen.go` 家族注释已在 T8-1 订正 + docs/roadmap 本无残留）就此关闭；③**发射侧 arity/Float64 守卫为纵深防御**——`take()`/`take(1,2)` 停于检查器 arity 词（spec gap）、`take(1.5)` 停于 E0501，皆真机实证不可达发射，`lazy_test.go` 尾注说明而不钉；④**`Emit` 测试辅助的管道局限**——无 Shapes 则 `ifaceSlots` 恒 false、迭代器面环境性停，惰性面单测必须走 `checkShapes` 真管道（文件头 doc 已述）；⑤filter/skip 的循环头/排水检查各自前置空跳入口块——LLVM 禁止分支指向入口块（首块即入口，与名无关；clang 错误直指 `label %loop` 实证），map/take 的 phi 引 entry 是前向边故合法。
- [x] 急性七枚的 **Dyn 接收者路径**（T8-2 披露①登记）：`let it = xs.iterator().map(f)` 后 `it.count()` 等仍停 bndMainBody——绑定名是 `Dyn<Iterator<U>>`，急性臂不认 Dyn 接收者（内联链形已可用，黄金 `run-acute-lazy-chain` 七段覆盖）
  来源：design D8 决策 2 的收尾面（11 枚词汇在绑定形下闭合）
  验证：run 黄金——map 之后 reduce/fold/count/any/all/find 各一枚（含跨 let 绑定形）；既有 List 快路径黄金零漂移
  落地（`e04c785`，2026-09-14）：码面 = `emitAcute` 在 acuteCombinator 检查后插 dynRecvOf 臂（分派序 `:3473` 先于 `:3503` 的通用盒派发，故本臂优先）——`face.Kind == ShapeDyn && face.Decl.Name == "Iterator" && len(face.Args) == 1` 时经 `instShape`/`elemOfShape` 定元素域，`emitAcuteLoop` 走 `walkSrc{box: &boxSrc{reg, face}}`，与内联链臂**同一走查**；非 Iterator 面不拦（落 T6 通用派发）。安全论证收口：`res.dyn` 写点封闭集 = `emitBox:10851`（仅 dispatchable）/`iterBoxCore:10974`/`emitLazy:11181`，**`emitMethodCall:14305` 经 `emitCallCore` 返回不设 dyn**——协议句柄绑定（`let it = r.iterator()` 经方法表）不带 dyn 面，新臂不会把句柄当盒走查。
  验证实跑：①单测 3 枚（408→411，`lazy_test.go` 尾部走 checkShapes 真管道）——`TestLazyAcuteWalksABoundChainsBox`（chead 派发 gep 基址 == 链结果盒、`MapIter` 表恰一次（链不被重复求值）、零 `__we_list_snap`）、`TestLazyAcuteWalksABoundBuiltinBox`（显式盒绑定形，派发过 ListIter 盒、对象 next 在表）、`TestAcuteOverAUserFacesOwnCountDispatches`（用户面自有 count 槽经自有表派发调 impl 方法、IR 零 `chead` 零 `@.vt.Iterator`——名字判据不误拦的最锐负形）；②黄金 838→844——`run-acute-bound-{reduce,fold,count,any,all,find}` 六枚（map 之后跨 let 各一急性；any/all/find 的 miss 臂用**独立绑定**——同一绑定的第二次急性观测到的是一次性耗尽契约而非谓词不命中）；③红先：HEAD `795bc93` worktree 上两枚正向单测 FAIL + 六枚黄金全 exit 70 无输出，用户面负形两侧皆绿（设计如此）；④既有 List 快路径零漂移 = 全量 844 枚全绿（含全部 `run-acute-*` 既有枚）。
  披露：①**face (c) 独立缺口**——裸 `let it = xs.iterator()`（无链、无显式盒）**绑定自身**停 bndMainBody：检查器把该调用定型为裸接口 `Iterator<Int64>` 非 Dyn 盒，emitAcute/emitLazy 均不认裸 `iterator` 名，不在 T8-3 任务行字面内；docs 双语关死句已改写为该形（原「绑定后过急性」句随本任务关闭）；②协议句柄绑定（`r.iterator()` 经方法表）不带 dyn 面——`emitMethodCall` 不设 dyn 的封闭集论证即其结构性理由，与 face (c) 同停；③黄金 any/all/find 初版曾把两次急性写在同一绑定上（判别力缺陷：miss-ok 会是耗尽的证据而非谓词不命中的证据），落盘前自查改独立绑定。

## T9 载荷面二簇（D9 簇 1 + 11）

- [x] 簇 1 **急性载荷面**：放开 ~~`scalarWordFace:5666` 守卫（`:5841`/`:5926`）~~（D9 勘察旧号，编辑按内容匹配不按行号），载荷按元素**真面**携带（Float64 按 double 位型、gc 按句柄），match 臂绑定按同一面读取
  来源：design D9 簇 1
  验证：`build-bnd-acute-float-payload`、`build-bnd-acute-gc-payload` 双双翻绿（先红后绿）；**负例**——多字载荷（String/元组）**仍停**（不越界放宽）
  **落地记**：码面 = `payloadFace`（`codegen.go:6663`——排除 `skStr` 与未定面 `!face.gc && face.kind == skNone`，再经 `abiWordTypes(p) != 1` 一字校验）+ `optionShapes`（`:6678`，序 = tag 算术 [None=0, Some=1]）+ `payRoundTrip`（`:6687`，三分读回：gc→inttoptr/ptrtoint、skF64→bitcast 双向、其余 i64 直过）+ emitReduce rlater 按面发射（`:6927-6969`）+ emitFind 只换守卫（`:7016`）+ 两处 sumSlot 挂 `variants/shapes`（`:6968`/`:7054`）——**簇 1 与簇 11 同管道闭合**（臂绑定走既有 `bindArmPayload:7724` 表驱动路径，abiI64 臂落 `scalarSlot`、abiDouble 臂 bitcast + `isFloat`、abiGc 臂 inttoptr + gcEnv）。验证四层：两枚翻绿黄金（红证 = HEAD 二进制 p1–p4 探针全 exit 70）+ `run-acute-float-payload-face`（真机 `sum 4`——`__we_str_of_f64` 把 4.0 渲染为 `4` 无尾 `.0`，黄金按真机字节落盘）+ `run-acute-gc-payload-face`（`found 7`）+ 负例 `build-bnd-acute-string-payload`（String 元素 reduce 维持 exit 70 全句 bndMainBody）。单测四枚（`acute_test.go:404-548`：float 载荷七步全序钉、gc 载荷句柄钉、int 载荷 String 位钉、String 负例钉）。
- [x] 簇 11 内置 `Option` 载荷读入 String 位（N2）：与簇 1 同一条载荷面，差在消费端是 String 位而非 match 臂
  来源：design D9 簇 11（与簇 1 一并设计，不得修两次）
  验证：**先新增锚定黄金**（今日零覆盖）取红态 → 修后 run 绿；`docs/benchmarks.md` 的 N2 面随之关闭
  **落地记**：与簇 1 同一提交（同管道：sumSlot 携 `shapes` → `bindArmWord` 的 abiI64 臂落 `e.scalars[b.Name] = scalarSlot{kind: baseStrKind(p.typ), num: narrowName(p.typ)}`，String 位经 `faceParam:6641` 的 typ 来源闭环）。新黄金 `run-acute-int-payload-string-position`（p1 源：`[1,2,3].iterator().find(|x| x > 2)` + match Some(v)→`"got ${v}"` → exit 0 stdout `got 3\n`；红证 = HEAD 二进制同源 exit 70）。**N2 面只关 reduce/find 两半**——`next` 的判决（p6–p10 真机，新旧二进制同停 ⇒ T9 前既有边界非回归）：内建对象 `next`（直接 match scrutinee / 绑定后 / 仅绑定）与用户 Iterator `next`（绑定后 / 仅绑定）一律 exit 70，**停在绑定本身**、早于 docs 旧句描述的「读入 String 位」⇒ docs 双语关死句已改写为该形（补记十五第 4 条）。

## T10 valueForm / valueKind 五簇（D9 簇 2 + 7 + 8 + 10 + 13）

- [x] 簇 2 **值位块帧**：`blockKind:9167` 读块自己的帧，而非 match 臂已还原的外层帧（`emitArmBlock:4490-4521` 已弹出臂帧）
  来源：design D9 簇 2
  验证：`build-bnd-value-form-emission-binding`、`build-bnd-value-form-shadowed-binding` 翻绿；**遮蔽回归钉**——`str_test.go:430-450` 钉的「拿外层帧分类会把内层 `5` 走 bool 转换器」不得复发
  **落地记**：码面 = `blockKind` 头部 `pushEnv()` → 逐顶层 `*ast.Binding`（`Pat != nil` 逐名 `poisonName`；否则 `installBlockFace`）→ `valueKind(尾表达式)` → `popEnv()`；毒化 = `scalarSlot{kind: skNone}` + `delete(strEnv)`。两枚黄金保名保源翻绿；遮蔽钉按计划重锚为 `TestValueFormClassifiesInTheBlocksOwnFrame`（要求 i64 转换器在场、bool 不在场）。**突变 M1 揭出真覆盖缺口**：摘掉 pushEnv/popEnv 后原遮蔽测试仍答对（installBlockFace 直接污染外层 env），块后再读外层名（`io.println("${b}")`）净树打 `5\ntrue\n` 而 M1 树产非法 IR（clang `expected value token` 拒收）——修 = 黄金 `run-value-form-shadowed-block` 扩两 println + 新单测 `TestBlockKindLeavesTheOuterFrameAlone`（要求 i64 与 bool 两转换器各恰一次），两层齐红。
- [x] 簇 7 **值位 String 臂**：`valueForm` 汇槽从数值集专属（`put:4476`、`joinKind:9181` 排除 `skStr`）扩到能承载两字 String 对，两个 String 臂可 join
  来源：design D9 簇 7
  验证：`build-bnd-value-form-string-arm` 翻绿；**负例**——分支不一致（数值 vs String）**仍停**；**评估与簇 2 合并**（簇 7 边界的勘查提示：汇槽若按「携带面」重做则两簇一并关），合并与否的判决入完成记录
  **落地记**：黄金翻绿 + run 孪生 `run-value-form-string-arm`（`a\n`）。**合并判决：部分合并**——分类塔一刀（`joinKind` 新规 `if a == b { return a }; return skNone`，skStr 对放开、混合仍 skNone），发射塔簇 7 自有（`put` 的 ckStr 臂：首 String 臂预约 `strSlot/lenSlot` 双 store、已约数值槽再遇 ckStr 即 bnd；`ckI64` 首约时反向核对）；汇槽**未**按「携带面」整体重做，`{slot, isFloat, unit}` 数值面与 `{strSlot, lenSlot}` String 面在同一个 valueForm 里互斥共存。**负例层次勘误**：混合分支在真程序里停**检查器 E0501**（exit 1，`the if arms are String and Int64`，无既有黄金锚——补 `check-e0501-if-arm-disagreement`）；发射层自身的 join 门由 `TestValueFormMixedArmsStillStop`（AST 直构绕过检查器）钉 `bndMainBody`，`joinKind` 直调四断言钉 `TestJoinKindCarriesAStringPair`——按层次如实记录，三层纵深。
- [x] 簇 8 **无标注 `var`**：`codegen.go:2720-2722` 的 `*ast.NamedType` 要求改为**从初值定面**（先定面再发初值）
  来源：design D9 簇 8
  验证：`build-bnd-var-unannotated` 翻绿；**负例**——初值面不可定仍停（诚实边界）
  **落地记**：`s.Typ == nil` → `switch k := e.valueKind(s.Init)`：skStr → `emitStringExpr` → `bindStringSlot`；skI64/skU64/skBool/skRune/skF64 → `emitNumericValue` → 校验 `res.kind == ckI64 && res.isFloat == (k == skF64)` → `res.num == ""` 时 `annNarrow(nil, s.Init)` 兜底 → `bindScalarSlot`；default → bnd。黄金翻绿 + run 孪生 `run-var-unannotated`（`b\n`，打印赋后值非初值）；负例 `build-bnd-var-face-indeterminable`（`var v = Some(1)` → 面 skNone 维持 70）。突变 M4（初值面分支全摘）单测层原无钉——补 `TestVarWithAnUnannotatedInitializerBinds`（绑定 + 赋值 + println）后两层齐红。
- [x] 簇 10（N1）**fn 值的值位调用** + 簇 13 **元组值读入 String 位**
  来源：design D9 簇 10/13
  验证：**先各新增锚定黄金一枚**（今日零覆盖）取红态 → 修后 run 绿；**负例**——语句位与尾位**仍须跑通**（B1a T14 的实测：界在位置不在捕获，修复不得反把语句位改停）；`docs/benchmarks.md` 的 N1 面随之关闭
  **落地记**：簇 10 码面 = `callStrKind` 新 fnEnv 臂（`fv.typed` → `baseStrKind(fv.abi.retName)`）——**值串位与操作数位一并打通**（arithKind 经同一 callStrKind）：`run-fn-value-call-in-string`（`2\n`）、`run-fn-value-call-operand`（`3\n`）皆取红后绿；语句位回归钉 `run-fn-value-statement-position`（`2\n`，未被改停）。**fnEnv `typed=false` 纵深防御不强行钉**：五个人口点（:4836/:13947/:4444/:2016/:2256）全 typed:true 或传播，模块不可达——循 B1a T6「双门冗余」先例，单测 `TestCallStrKindAnswersAFnValueByName` 直调钉语义（seen 答 skI64、unseen 答 skNone），黄金层无法构造可达形。簇 13 码面 = `emitHole` 头部两臂（`*ast.Ident` 命中 `e.tupEnv` → `renderTuple`；`*ast.Tuple` 字面量 → `emitTupleAgg` → `renderTuple`）+ `renderTuple`（intern'd `(` / `, ` / `)` 分隔 parts 链、concatStr 链接；元素按 `el.kind`：abiStr 恒等两 load、abiDouble `__we_str_of_f64`、abiI64 `strOfSyms[baseStrKind(el.typ)]` 查不到即 bnd；**abiGc/abiSum → bnd 诚实边界**）；`strOfSyms` 包级 map 供 emitHole 与 renderTuple 共用（原内联表改读它）。锚定黄金 `run-tuple-in-string`（`(1, 1.5, true, a)\n(2, b)\n`——绑定形与字面量形两 println）取红后绿；负例 `build-bnd-tuple-gc-element`（gc 记录元素 → 70）。突变 M6a/b/c（Ident 臂/分隔符/转换器路由）单测层原无钉——补 `TestTupleValueRendersIntoAStringPosition`（`countCall(of_i64)==2` + 无 bool 转换器 + IR 含 intern 的 `c", "`）后两层齐红。**docs N1 面关闭**：英文版「What remains closed」清单摘「元组值绑定读入 String 位」与「fn 值在值串位或操作数位的调用」两面、追加第八次拓宽从句、计数 seven→eight；中文版同步三处（`七次`→`八次`、摘两面、追加从句）。

**T10 突变电池（8 变体，单点、逐枚还原、锚点先断言 `count==1`，单测层 + 黄金层各记判决）**：净树备份 `/tmp/we-t10/codegen.go.bak`、电池脚本 `/tmp/we-t10/battery.py`（M1 用 `mutate.py`）。判决表——

| 变体 | 突变点 | 初轮判决 | 处置 |
| --- | --- | --- | --- |
| M1 | 摘 `blockKind` 的 pushEnv/popEnv | **两层存活**（原遮蔽测试仍答对——installBlockFace 直接污染外层 env；真机：块后再读外层名产非法 IR，clang 拒收） | **真覆盖缺口**：黄金 `run-value-form-shadowed-block` 扩两 println + 新单测 `TestBlockKindLeavesTheOuterFrameAlone` → 复跑两层齐红 |
| M2 | 摘 `blockKind` 的毒化（Pat → poisonName） | 两层齐死 | 无需处置 |
| M3 | 摘 `joinKind` 的混合拒绝（恒返回 a） | 两层齐死（`TestValueFormMixedArmsStillStop` + `check-e0501-if-arm-disagreement` 是检查层门、发射层由单测独守——如实分层记录） | 无需处置 |
| M4 | 摘 `emitVarBinding` 无标注路径的初值面 switch | 黄金层死、**单测层存活** | 补 `TestVarWithAnUnannotatedInitializerBinds` → 两层齐红 |
| M5 | 摘 `put` 的 ckStr 臂首约核对 | 两层齐死 | 无需处置 |
| M6a | 摘 `emitHole` 元组 Ident 臂 | 黄金层死、**单测层存活** | 并入 M6 补钉（下枚） | 
| M6b | renderTuple 分隔符改无 intern | 黄金层死、**单测层存活** | 并入 M6 补钉 |
| M6c | renderTuple 转换器路由改恒 i64 | 黄金层死、**单测层存活** | 补 `TestTupleValueRendersIntoAStringPosition` 一枚覆盖 M6a/b/c → 复跑三层齐红 |

**「两层齐活」纪律的兑现**：M1 与 M4/M6* 的初轮存活各暴露一枚真覆盖缺口（黄金有单测无，或反之），全部补钉至两层齐死才收——B1a T8-2「两层齐活」先例的延续。电池收尾：`codegen.go.bak` 还原 + `go test ./internal/codegen/` 全绿 + `gofmt -l` / `go vet` / `go build` 三面净复核。

**T10 收口账**：单提交 `fd2e263`（18 文件 +675/−80：`codegen.go` + `str_test.go`（2 重锚 + 7 新）+ docs 双语 + 4 翻绿黄金 + 10 新黄金）。conformance **848→858**、codegen 单测 **413→420**、15 包全绿（`-count=1` 一轮）；validate `--all --strict` OK、docs_sync 33 对、`git diff --check` 空、staged 零 `refr/`；`WE_UPDATE_GOLDEN` 未使用。**未推送**：本地 ahead 16。

## T11 List 载体边界契约（D9 簇 4）

- [x] ① **ABI 敷设**：`classType:10215` 放行 `List<T>`，`bindTupleParams:10534` 让载体与元素面进参数槽，`fnRetKind:9265`/`fnRetOperand` 同步
  来源：design D9 簇 4 ①
  验证：`build-bnd-list-abi` 翻绿（先红后绿）
  **落地记（2026-09-15）**：13 处改动——`listArgOf` 新纯函数（裸 `List` 引用 + 唯一实参的形状判）；嵌套拒上移到 `elemFaceOfType` 入口（`listArgOf` 命中即 false——今日该路径本就 false 故零既有漂移；classType List 臂靠同一读自动拒，签名位/元素读/字面量注解三处不可能不一致）；classType List 臂（`elemFaceOfType(实参)` 不 ok 即拒；`abiGc` + 空 key，空 key 即盒的先例、面随 ABI 行走）；`carrierElemFace` 新读法；`fnParamAbi.list`/`fnAbi.retList` 两字段；`fitAbi` abiGc 返回位带面；`bindTupleParams` 一般臂记面；`bindDefineParams` abiGc 臂按 `pa.list` 分流（listEnv vs gcEnv——盒参数保持 gcBinding 路径，其 dyn 面是 T6 的事）；`emitListOperand` 新（Ident/Member/ListLit/Call 四源）；`emitCallCore` 实参臂 List 直传载体 + 返回 `callResult.list`（T8-1 的字段）；`fnRetOperand` abiGc 臂前插载体发射。`fnRetKind`/`bindResult`/`sameAbi`/Call 实参臂零改动（`baseTypeName(List<Int64>)=""`⇒skNone 已核；Call 臂 `e.records[""]` 不中即直传）。**红先**：`build-bnd-list-abi` 父树（`fd2e263`）exit 70 → 本树 0 三空面；新单测 7 枚（`listabi_test.go`）父树 5 正向红（2 负向是两侧不变的拒绝钉）；新 run 黄金 4 枚（`run-list-abi-param/return/arg-call/gc-elem`，stdout `6/6/6/7`）父树全 70——`List<Cell>` 端到端 run 0 打印 7 是超出黄金覆盖的连接面实证。**单测层钉签名位**：`List<String>` 参数源端到端停 `bndMainBody`（main 体字面量先停），签名位拒由 `TestStringElementListRefusesAtTheClassifier` 钉（f 不被调用、`bndFnBody`）。**语言事实**：嵌套闭包须分离拼写 `List<List<Int64> >`（`>>` 是运算符，E0105）；效果段在参数表与 `->` 之间。
- [x] ② **元素宽度**：单字元素槽装不下 String 对，需宽元素表示（两字槽或装箱），`emitListElemValue:5406`/`emitNumOperand:2093` 同步
  来源：design D9 簇 4 ②
  验证：`build-bnd-list-elem` 翻绿；run 黄金（`List<String>` 的构造/遍历/索引逐字节）
  **落地记（2026-09-15）**：表示形取**装箱**——32B 无描述符盒 `{map@0=null, size@8=32, strptr@16, strlen@24}`，元素以句柄入槽、载体协议一字不动（`runtime/c/` 零改动，D7-附姿势保住）；取舍论据与静态描述符方案的否定见 design 补记十七第 2 条。码面六处功能编辑：`traces()` 单谓词（`face.gc || face.kind == skStr`，字面开口与 collect 两处同问）；`emitListElemValue` skStr 装箱臂（`emitStringExpr` → `allocObj("null", 32)` → gep16/gep24 两存 → `ptrWord`）；`elemFaceOfType` 尾 `case abiStr` 臂；`bindForListElem` skStr 读回臂（inttoptr 回盒 + 两读；赋值名 `bindStringSlot`、未赋值名 `strBinding` 操作数对）；`listElemWord` 入口守卫合并（`skStr || skNone → bnd`——回调五枚仍停：参数得是两字 String 本身而句柄不是）；`classType` List 臂注释订正。**红先三层**：单测层（① 树四正向钉红取屏）+ 黄金层（HEAD worktree 五枚全 FAIL）+ 真机层（前窗 e1–e7 探针全 70 取数）；本树单测全绿、真机 e1/e2/e3/e4b/e6/e7=0（e5=70 守卫钉住）、黄金六枚全绿（四 run 新 + `build-bnd-list-elem` 翻绿重锚 + 新守门负例 `build-bnd-list-elem-callback`——D10 一正一负的补课，② 的负例面此前零黄金）。**突变电池五枚两层终表**见 design 补记十七第 5 条（M2 黄金层结构性静默的差分实测在第 4 条；M3 的击杀是 declare 级，如实标注）。**测试账**：codegen 单测 427→433（427 = ① 七枚之后；② 的 `listelem_test.go` 六新钉；`listabi_test.go` String 分类器钉重锚更名不计）；conformance 862→867（862 = ① 四枚 `run-list-abi-*` 之后；② +5 新文件——四 run 新 + 守门负例 `build-bnd-list-elem-callback`；另**两枚保名翻绿重锚**——`build-bnd-list-elem` 与 `build-bnd-builtin-iterator-string-elem`（T7-4 钉的 String 元素进盒停，② 开其构造面，全套扫描捕获后真机复测翻绿））。**既有边界披露**（真机二分坐实与②无关）：洞内急性停（Int64 同停，夹具一律绑定形）、未绑定链式接收者 `mk().iterator().collect()` 停（先绑定则通）、e7 装盒面（构造/`next` 绑定开，载荷读停 T9、盒上急性停 elemOfShape）——`iterSourceOf` doc 的「two words」句已订正。**docs 双语**（`benchmarks.md`/`.zh.md` 运行面段）第九次拓宽句 + 关死清单改写（String 元素源开、回调五枚/gc 进盒/盒上载荷读与急性关）；**顺带订正一句过时关死**——「`next` 的 `Option` 停在绑定自身」今已不真（四探针：盒内建 Int64/String 与用户 `Iterator` 的裸绑定 `let o = …next()` 皆 exit 0，停点在**臂读** `match … Some(v) =>`），此开口非② 所为、系其后的值塔拓宽连带，docs 按实测改写不认领。
- [x] 两件的**独立性负断言**（勘查已证）：只做①则 `-list-elem` 仍停、只做②则 `-list-abi` 仍停——逐件实测记录
  来源：design D9 簇 4 边界
  验证：逐件单独落地时的红态实测入完成记录；两件合并后两枚同绿
  **落地记（①半，2026-09-15）**：① 树上 `build-bnd-list-elem` PASS @ 仍期望 exit 70——只做①不翻开 String 元素面（停点在 `emitListElemValue` 标量尾臂，① 不触）。**②半（2026-09-15）**：`fd2e263` 干净 worktree + 仅② 六处功能编辑 → `build-bnd-list-abi` 探针 exit 70、`build-bnd-list-elem` 探针 exit 0——与①半恰成互补，双半实测入册；合并树两枚同绿。
- [x] 宽元素**表示形取舍**在此补记（两字槽 vs 装箱），含 `__we_list_new` 的容量/描述符与 gc 描记影响
  来源：design D9 簇 4 边界（「本簇的核心取舍，实现期在 D9-4 下补记」）
  验证：取舍结论写入 design D9 簇 4；选定形的 gc 压测（越过 `GC_THRESHOLD`）绿
  **落地记（2026-09-15）**：取舍结论 = 装箱（32B 无描述符盒），论据四条与弃静态描述符的理由见 design 补记十七第 2 条；gc 压测黄金 `run-list-elem-gc-crossing` 落为**构造函数形**（盒的构造根在 `mk` 出口弹尽，描记载体成为唯一存活路径；净输出 `40000` + e00..e99 逐字节），插桩实测 sweeps=1、越过阈值；M2 突变（撤描记谓词）在该形下差分实测**恰多回收 100 枚 32B 盒**（= ys 的元素盒）而输出仍逐字节净绿——机理与「结构性静默」判决见补记十七第 3/4 条。

## T12 `?` 的用户 Result 与返回位（D9 簇 6 + 9）

- [x] **阻塞前置**：查证 `errMsg` / 报告行的确切用途（诊断位置 vs 运行期错误文本），据以定放开的形
  来源：design D9 簇 6 边界（「本行是方向，不是定案」）
  验证：查证结论写入 design D9 簇 6，含 `errMsg` 的全部置位点（~~今日唯一置位点 `:8078`~~ 行号漂移，实测 `:9377`）
  落地记（2026-09-15）：结论 = **运行期错误文本**（用户可见 stderr 输出），证据链四条与 errMsg 三点表（`:1253` 字段 / `:8798` 读 / `:9377` 唯一写）见 design 补记十八。**勘查期重大发现**：`emitTail` Err 臂已有运行期报告行机制 `errReport`（`:12728-12733` 发射、`:12810-12833` 定义）——② 复用其纪律（intern head + 载荷对 + `"\n"` 连接件 + `__we_fail(ptr, i64)`），**运行时零改动**，勘查期「改 startup.c 加 fputc」的备选作废（D7-附零运行时姿势保住）。②③ 的门与负例（questionOkFace / questionErrLine / e.exit exitFn+abiSum 门 / prim 值面仍停）同在补记十八定案。
- [x] 簇 6：`?` 的早退**不依赖字面消息**——错误值随 Result 传播，报告行由其载荷在运行期取得；`codegen.go:7500` 与 fn 体侧 `:7475` 同步放开
  来源：design D9 簇 6
  验证：`build-bnd-m6b-main-body` 翻绿（先红后绿）；run 黄金——`?` 的成功路径与错误路径各一枚（报告行逐字节）
  **落地记（2026-09-15）**：`emitQuestion` 重构为四臂 switch（errPanic / errMsg 静态两臂字节不动，timeout 黄金 stderr 保住）+ 两新臂——**propagate**（fn 侧：tag/pay/pay1 三字 `insertvalue`×3 → `inExit` 守卫（防 deferred 内重入）→ `unwind(0)` → `drainDefers()` → `popRoots()` → `ret {i64,i64,i64}`；**不设 diverged**，`?` 是 mid-expression 出口，ok 路径继续同体发射）与 **report**（main 侧：String 载荷走提取的 `errFold`（intern head `"error: <V>: "` + 载荷对 + `"\n"` 常量段——`errReport` 改用同一助手，报告行组装纪律单点化，运行时零改动）；裸变体走 `errConsts` 同形静态常量）。Ok 绑定带 `typeName`/`num`（shapes[0].pay[0] 的声明基名——插值域与算术宽度按载荷真面回答，`run-question-main-success` 打印 `v 7` 走 `__we_str_of_i64`）。两门在 icmp/br 前纯读 shapes：`questionOkFace`（Ok 面 ≤1 个 i64 字）+ `questionErrLine`（恰一 Err 变体、载荷空或恰一 String）。**fn/main 不对称**如补记十八：fn 侧不问 Err 行门，多变体/非 String 载荷照传（`run-question-fn-propagate` 全链传播后由 main 报告行收口 `error: inner-bang`）。**红先**：`build-bnd-m6b-main-body` 父树（`da85332`）exit 70 → 本树 0。**新黄金六枚**：三 run（success `v 7`/error `error: Failed: boom` exit 1 stderr 逐字节/propagate）+ 三负例（wide-ok-face、non-string-err、multi-err-variants——三道门各一钉）。**单测六钉**（`m6b_test.go`）：门表测 12 子测、fn 侧传播（qerr 臂恰 3 个 insertvalue + 无 `__we_fail` + ok 路径在 join 后）、运行期渲染（恰 2 次 concat、整行非编译期常量）、裸变体常量（`@.q0, i64 12`）、Ok 绑定载荷面（唯一 `__we_str_of_i64`）、（簇 9 的再扎根钉同文件）。**突变电池**：M2（Err 行门杀）黄金层三杀（两负例 70→0 翻红、error 路径行文本错）+ 单测层门表测杀；M1（Ok 面门杀）**单测层杀、黄金层被掩蔽**——掩蔽机理真机二分定名（M1 二进制探针 p2–p6）：`?` 本身放行（p3 exit 0），停点在 `${v}` 插值（String 位读分类器拒），算术面检查器 E0501 先停（p6 exit 1）——两道独立下游守门（T10/T11 值塔 + 检查器），黄金的 exit 70 双因成立，如实记「either alone holds」先例（同 T9-3）。
- [x] 簇 9 **同步型返回位**：`fitAbi:10477` 的 `case abiPrim: return abi, false` 放开，补返回拼写与调用侧结果面（`fnRetOperand` 的 `abiPrim` 臂）
  来源：design D9 簇 9
  验证：`build-bnd-prim-return` 翻绿（先红后绿，报 `bndFnBody`）；**负例**——同步型的**值面**（非返回位）仍在本变更之外，其负例仍停
  **落地记（2026-09-15）**：三处——`fitAbi` 的 `case abiPrim` 拒臂改 `abi.retTyp = "ptr"` 放行（原「两卫一致」注释改写为「T9-3 拓参数位、本臂拓返回位，拒绝在此吊销」）；`fnRetOperand` 补 `abiPrim` 臂（`Ident ∈ e.prims` → `"ptr "+p`；内联 `Call` → `emitCall` → `res.kind == ckPrim` → `"ptr "+res.i64`；`!hasValue`/其余值面 → `bndFn()` 诚实停）；`emitCallCore` 补 `abiPrim` 结果臂——`call ptr` 后**立即再扎根**（`use/pushes++/push`，gc-record 返回位同款；被调方构造根在其出口弹尽，ckPrim 契约在制造处扎根 ⇒ 调用方对新鲜答案补根）。**红先**：`build-bnd-prim-return` 父树 exit 70（`bndFnBody`）→ 本树 0；`m9a_test` 的 `TestM9aPrimReturnStaysAtTheBoundary` 重锚为 `TestM9aPrimReturnCrossesAsAPointer`（`define ptr` + 构造器答案 + `ret ptr`）。**新黄金四枚**：`run-prim-return`（main 直调/let 绑定后传参/内联实参三形皆 `42`）、`run-prim-return-gc-crossing`（churn 形 200k 次构造-越限-丢弃，`churn 41`）、`run-prim-return-gc-hold`（**持有形**，突变电池 M3 的直接产物——见下）、负例 `build-bnd-prim-string-position`（prim 值面读进 String 位仍停，T9-3 词汇表外）。**突变电池 M3**（撤调用点再扎根）：初测 churn 形**黄金层静默**（每迭代的持有窗太短，块未及复用）——真机持有探针（make 后越限分配再读 `m.get()`）实测打出 `174761`（应 41），**危害可观测、黄金形选错**；按 D10 纪律补 `run-prim-return-gc-hold`（`m 41` 逐字节）后 M3 黄金层击杀（`m 174761`）；单测层 `TestPrimReturnCallReRootsTheAnswer`（调用与 push 之间零指令 + main 体槽形间接调用拼写）本就钉住。

## T13 `scope` 值形与 `timeout` 未决（D9 簇 12 + D13）

- [x] **阻塞前置**：查证 `scope timeout(N)` 的结果型为何在 `==` 上报 `E0501 mixed types`
  来源：design D13；design D9 簇 12
  验证：真机探针逐形实测；**若为语言面**（结果类型与普通 `scope` 不同）则登记 follow-up 且本变更内不改，**若为实现缺口**则并入本任务——两种去向都须显式记录
  **落地记（2026-09-15）**：裁定为**语言面**，本变更零改动。规范 `docs/spec/1800-concurrency.md:231` 明写：普通与 `collectAll` 形的值是体的块值（第 2 章），`timeout` 形的值是 `Ok(b)`/`Err(TimedOut)`——**按规范即 Result**。q1 探针捕获 `if r == 7` 的逐字 stderr：`error[E0501]: mixed types — operands of "==" are Result<Int64, TimeoutError> and Int64; no coercion is ever inserted`——检查器判得对（`Result` 永不胁迫为载荷），黄金 `check-e0501-scope-timeout-value` 钉住该语言面。q2 探针找到真正的残缺口：**对该 Result 作 match**（`match r { Ok(v) => … }`）过检查（exit 0）却在构建停 `bndMainBody`（exit 70）——消费面未开，登记 roadmap follow-up **#25**（双语）。
- [x] 簇 12 `scope` 值形：被读取即停 → 值面发射（数值位 / String 位 / 比较位）
  来源：design D9 簇 12
  验证：**先新增锚定黄金**（今日零覆盖；锚**必须读取**该绑定——~~未被读取时整条绑定被消除故 exit 0，三轮探针实证~~ **订正（本窗口直读 IR）**：无任何死绑定消除——未被读取的 `let r = scope { 7 }` 照发 `__we_scope_enter`/`__we_scope_leave`，数值尾喂进伪和式的载荷字故结果为 0；String 尾不问读取在发射处即停。「锚必须读取」的黄金设计结论仍成立，但成立的原因是「不读取则无可观测输出」而非「绑定被消除」）取红态 → 修后 run 绿；负例——`timeout` 形按前置结论处理
  **落地记（2026-09-15）**：四枚黄金先落（`run-scope-value-faces` SHADOW 形——外层 `let n = 10` + 体内 `let n = 3` + 尾 `n + 4`，使「尾先于体」的回归产**错值 14 而非停**、`run-scope-value-string`、负例 `check-e0501-scope-timeout-value` 按前置裁定钉语言面、`build-bnd-scope-value-gc-tail` 守卫默认臂仍 70），两枚正面在父树 `c290b28` 二进制上先红（exit 70 + `bndMainBody`）。修 = `emitScope` 尾求值块按 timeout 二分：普通/collectAll 形的尾在**体语句之后、体环境内**求值，数值尾作 `ownFace`（ckI64 + `typeName: strKindName(valueKind(尾))` + `num`——经 `bindResult` 既有值塔免费获得算术/比较/插值面），String 尾 `emitStringExpr` 绑对子，`__we_scope_leave` 后**早退 `ownFace`** 不建和式；timeout 形保持三槽 `Ok/Err` 和式（载荷字 + tag + pay1=0）。修后四枚全绿；全量回归 conformance **877→881**、codegen 单测 **439→445**（新 `scopeval_test.go` 六钉：数值面 SHADOW + sadd 计数 + 零 store、String 面 concat 计数、timeout 三 store、collectAll 普通面、gc 尾仍停、fn 体内绑定 `ret i64 5`）、15 包全绿。突变电池四枚（M1 摘早退恒建和式：两层齐杀，两枚正面黄金 exit 70；M2 摘 skStr 臂：String 钉 + String 黄金杀、数值面如设计存活——恰沿该臂的分界；M3 摘 typeName 携带：**两层齐杀**（钉①自带插值消费故比预案「单测层存活」更强）+ `run-scope-value-faces` 杀；M4 尾先于体（D10-2 复辟）：两层齐杀，黄金打出 `v 14` 对 `v 7` 的**错值**——SHADOW 形的设计目的）。docs 双语第十次→**十一次**拓宽 + 「仍关死」摘「普通 `scope` 值形」+ 补登「读 `timeout` Result 的臂」面。

## T14 收口：零集机械归零 + 文档清空

- [x] **边界行 grep 归零**（验收句的机械证据）
  来源：design D0（验收句）、design D12（边界行 grep）
  验证：语料全域扫 codegen 边界行，翻绿前后各一次——`grep -rl "beyond the M9b\|beyond the" internal/conformance/testdata/cases/` ~~在 codegen 塔**归零**~~ **订正（T14 对账）**：被本设计自家的 D10-2 一正一负政策否决——每簇的负例黄金正是停在 codegen 边界行上的钉子；终态 11 枚负例守卫在册，验收句以 D0 的「14 枚锚定黄金全部翻绿」为准（全绿）；typecheck 塔的 `check-assertequal-residual-bnd` 按 D0 **维持原期望字节**（负断言）
  **落地记（2026-09-15）**：`grep -rl "beyond the M9b\|beyond the" internal/conformance/testdata/cases/` 基线（`0030d60`）= **13 文件**（即 14 枚锚定黄金中携带 "beyond the" 的 13 枚；`test-fn-generic-bnd` 的词面无该短语故不在 grep 集内）→ HEAD = **11 文件**，逐枚核对皆为 D10-2 的**负例守卫**（acute-string-payload、builtin-iterator-gc-elem、dyn-value-payload-dispatch、list-elem-callback、prim-string-position、question-multi-err-variants、question-non-string-err、question-wide-ok-face、scope-value-gc-tail、tuple-gc-element、var-face-indeterminable）——无一正面黄金停在 codegen 边界行，归零的原义（正面全通）兑现；字面归零被 D10-2 否决（见订正）。`check-assertequal-residual-bnd` 对 `0030d60` diff 零行（本窗口复验）。
- [x] 14 枚边界黄金**逐枚翻绿对账表**入完成记录
  来源：design D10-1
  验证：逐枚先红后绿证据；`go test -count=1 ./internal/conformance/` 全绿（黄金账数字取自 `-count=1` 一轮）
  **落地记（2026-09-15）**：14 枚逐枚对账（基线全 exit 70，全数在本变更内翻绿）——

  | 锚定黄金 | 基线词 | 关闭簇 / 提交 |
  | --- | --- | --- |
  | `build-bnd-acute-float-payload` | bndMainBody | 簇 1 载荷面（T9 `c0b3851`） |
  | `build-bnd-acute-gc-payload` | bndMainBody | 簇 1 载荷面（T9 `c0b3851`） |
  | `build-bnd-value-form-emission-binding` | bndMainBody | 簇 2 块帧（T10 `fd2e263`） |
  | `build-bnd-value-form-shadowed-binding` | bndMainBody | 簇 2 块帧（T10 `fd2e263`） |
  | `build-bnd-list-iterable` | bndMainBody | 簇 3 协议臂（T7-A `7df9d4b`） |
  | `build-bnd-acute-user-iterable` | bndMainBody | 簇 3 急性面（T7-B `9045441`） |
  | `build-bnd-list-abi` | bndMainBody | 簇 4① ABI 敷设（T11 `8be1f64`） |
  | `build-bnd-list-elem` | bndMainBody | 簇 4② 元素装箱（T11 `da85332`） |
  | `build-bnd-acute-lazy` | bndMainBody | 簇 5 惰性四枚（T8-2 `795bc93`） |
  | `build-bnd-m6b-main-body` | bndMainBody | 簇 6 `?` 用户 Result（T12 `c290b28`） |
  | `build-bnd-value-form-string-arm` | bndMainBody | 簇 7 String 臂（T10 `fd2e263`） |
  | `build-bnd-var-unannotated` | bndMainBody | 簇 8 无标注 var（T10 `fd2e263`） |
  | `build-bnd-prim-return` | bndMainBody | 簇 9 同步返回位（T12 `c290b28`） |
  | `test-fn-generic-bnd` | bndGenericFns | 泛型单态化（T4 `9cc47de`，翻绿重锚 pass 输出；词面无 "beyond the" 故不在 grep 集） |

  conformance 终态 **881**（B1b 起点 810）、`go test -count=1 ./internal/conformance/` 全绿（数字取自 `-count=1` 一轮，T13 提交时与 T14 本轮复跑双证）。
- [x] 词表清退：某词的**全部产出点**消失时该词与 `codegen.go:28-57` 的历史注释一并退休；两塔共享词面在历史注释里记一行
  来源：design D0
  验证：词表终态 grep 结果入完成记录（~~预期 **< 5 词**~~ **订正（T14 对账）**：终态 **5 词**，无词退休——「预期 <5」是被实现推翻的设计期预测，逐词说明存活理由如下）；typecheck 侧词常量**零改动**（负断言）
  **落地记（2026-09-15）**：终态 **5 词**，逐词产出点清点（`grep -o` 计数已减去定义行 1）——**`bndMainBody`** 15 处（main 体余面：`?` 两门、同步对象读入 String 位、`String` 元素源的回调五枚、gc 源进盒、盒载荷读、盒上急性、timeout Result 臂读、next Option 臂读——11 枚负例守卫的面）；**`bndFnBody`** 45 处（fn 体内同样的值面族 + 守卫层）；**`bndTaskBody`** 2 处（`:1180`/`:5565`，task thunk 捕获超出标量集）；**`bndGenericFns`** 28 处（`:793` 是 `collectImpl` 的**真拒绝**——impl 头实参非自身参数顺序排列；余 27 处为 `:8321`–`:8722` 的 Eq 域残余）；**`bndTopLets`** 1 处直产（`:7340`，顶层绑定注解与初值面皆不可定的形）。**两塔共享词面已记入 `codegen.go` 历史注释**（T14 终审段）：`bndGenericFns` 与检查塔 `typecheck.go:57` 常量（`:2798` `assertEqualCall` 于 `!inEqDomain` 时产出）同串——两行**同退同留**。typecheck 塔词常量**零触碰**（负断言：`git diff 0030d60 -- internal/typecheck/typecheck.go` 的 83 行改动皆为 T1/T4 的 `Shapes`/`Site` 登记面，`bndGenericFns` 词面行不在 diff 内——grep 对 diff 取空实证）。
- [x] `docs/benchmarks.md` + `docs/benchmarks.zh.md` 的「What remains closed」清单**清空**，记明 N1/N2 各由哪一簇关掉，并分开写「codegen 停面」与「工具面停面」
  来源：design D11
  验证：`docs_sync.py`（33 对）通过；**10 枚非 codegen 停面黄金逐枚字节不变**（负断言，含 4 枚名字带 `bnd` 的 CLI 侧停面）
  **落地记（2026-09-15）**：~~清空~~ **收窄（T14 对账）**：清空只在原义上成立——B1b 起跑时清单载着的**六面全部离场**（泛型、普通 scope 值形、main 体 `?`、元组值读入 String 位、N1、N2），与今清单零重叠；今清单载着拓宽**自身揭出**的诚实余面。双语段落重构为**两种分开写**：codegen 停面八枚（原披露内容逐字保留）→ 工具面停面一枚（裸句柄绑定，成因明写在检查塔的定型而非 codegen）→ 运行期规则一条（明写**不是边界**）。N1 归属 = **值塔 fnEnv 臂**（T10 `fd2e263`），N2 归属 = **载荷面簇**（T9 `c0b3851`）+ `next` 绑定其后亦开（T11）停点移到读 `Option` 的臂——归属句与六面离场句随行。收尾句「子集本身随 codegen-full 线落地收缩」改写为「那条线已落地；仍关死的是拓宽自家的发现」（原句收口后陈朽）。`docs_sync.py` **33 对**通过；10 枚非 codegen 黄金对 `0030d60` 逐枚 diff **零字节改动**（本窗口 `git diff --quiet` 逐枚核对：build-single-file、run-single-file、build-bnd-test-module、build-bnd-test-fns-first、build-library、check-bnd-std-member、check-stdio-shadow-let、check-bnd-method-arity、check-assertequal-residual-bnd、run-conc-deadlock）。
- [x] 披露清单逐条入完成记录（各任务的未决补记 + D11 + D13）
  来源：design D11/D13
  验证：完成记录含逐条披露；proposal 的实现记录段与之一致
  **落地记（2026-09-15）**：逐条汇总（各任务落地记的浓缩面，详证在各任务节与 design 补记三至二十）——**T3**：两处诚实边界（检查器不解析显式类型实参位上的类型参数→E1304，递归形单测层钉；`io.println(字段读)` 停 `argIsScalar`）；**T4**：N3 洞内推断应用仍停（洞内无站点无写出实参）+ 泛型 sum 构造子两种拼写皆停（`variantSite` 不读 `e.generics` 模板的实参）+ 分类器两读一留一删 + 21 枚 ch10 孪生普查（9 孪生 / 12 未补逐条列因）；**T5**：`#24` resource 进开类型位置（检查器 `resource.go:431-440` 从不核对被调形参类型，语言面登记不预设结论）+ 泛型体内推断应用 `instShape` 解位缺陷（panic→已修）+ sum 载荷面「全变体同意」判据；**T6**：T5 的「假阴性黄金层捕获随 T6 落地」被推翻（本 build 不可观测，可派发集 = 独立扎根集）+ 决策 4「scalar 直传值」被推翻（表只在 gc 载荷面发射）+ 默认方法无槽不派发 + `callStrKind` 无 record 键致派发结果不能作操作数（T6-4 修）；**T7-A**：字面 `*ast.Call` 源仍停（验收句的字面形不通过、由绑定源形兑现）+ `-> Iter` 普通绑定被同枚改动一并拓宽（超任务书披露）+ D7 红证据引文指向从不存在的面；**T7-B**：`listWalk` 无需 `proto` 字段（sketch 简化偏差）+ `String` 载荷停且成因是**源的元素面**（此前未入 docs 清单的披露缺口，补登第六面）+ `emitStep` 无界计数器点名不掩盖；**T7-4**：两枚描述符位本 build 不可观测（不冒充黄金层捕获）+ `RangeIter`/`StrIter` 上游前提未开（检查器 `E0816`/`Rune` 无 ABI）；**T8-1**：逐元素根纪律四代突变不可观测（保留纪律不冒充捕获）；**T8-2**：LLVM 入口块回边之禁（空跳前置块修）+ 守卫分层真机实证（arity/E0501/bndStdModules 皆检查器先停）+ `Emit` 测试辅助无 Shapes 环境性停（凡钉迭代器面必须走 `checkShapes` 真管道）；**T8-3**：face (c) 裸 `xs.iterator()` 绑定自停披露不修（检查器定型裸接口，语言面取舍）+ any/all/find 黄金须独立绑定（同一绑定第二次急性观测的是一次性耗尽）；**T9**：String 负例双门冗余维持（源元素面 + `payloadFace` 排除，黄金钉净效果）+ `next` 判决（绑定即停为 T9 前既有、T11 后绑定开停点移臂读）+ `strLit` 须带引号文本；**T10**：负例层次勘误（混合臂真程序停检查器 **E0501** 非 codegen 边界，补 `check-e0501-if-arm-disagreement`）+ M1 摘帧存活揭真覆盖缺口（补黄金两 println + `TestBlockKindLeavesTheOuterFrameAlone`）+ fnEnv typed=false 不钉（模块不可达，循 T6 双门先例）；**T11**：洞内急性停（Int64 同停）+ 未绑定链式接收者停（先绑定则通）+ e7 装盒面（构造/`next` 绑定开，载荷读停 T9、盒上急性停 elemOfShape）+ M2 黄金层结构性静默机理已测得（插桩差分净树 swept32=21300 vs M2 树 21400）；**T12**：fn/main 不对称（fn 侧不问 Err 行门）+ M1 黄金层双因掩蔽（「either alone holds」循 T9-3 先例）+ M3 churn 形静默而持有探针实打错值（黄金形选错、按 D10 补 gc-hold 后击杀）；**T13**：`#25` timeout Result 臂读（check 0 / build 70，语言面登记）+ 无死绑定消除（tasks 断言被直读 IR 推翻）+ M2 数值面存活 = 设计预期分界；**T14（本任务）**：四条字面预测的最终对账（proposal 验收句 / `<5 词` / grep 归零 / 清空，见上四枚订正）。**D11** 的「分开写」与 10 枚非 codegen 黄金负断言已兑现（上枚落地记）；**D13** 的 Q2 未决项已收口（T13 语言面裁定 + `#25` 登记）、`check-assertequal-residual-bnd` 两塔词面共享维持记账不动。proposal 的验收边界句已按同一对账加删除线订正（见 proposal 订正）。

## T15 验证阶梯与 change-review

- [x] 完整阶梯：`go build ./...` + `go vet ./...` + `gofmt -l` + `go test -count=1 ./...` + 受影响黄金 `-count=1` + `validate.py --all --strict` + `docs_sync.py` + `git diff --check`
  来源：design D12；AGENTS.md:66 / process §4
  验证：逐条输出入完成记录；包数与黄金账数字取自 `-count=1` 一轮
  **落地记（2026-09-15）**：八面全绿，逐条——`gofmt -l .` **空**；`go build ./...` **exit 0**；`go vet ./...` **exit 0**；`go test -count=1 ./...` **15 包（13 ok + 2 [no test files]）0 FAIL**，逐包计时：benchmarks 60.809s、cli 6.975s、codegen 0.888s、conformance **162.818s**、deps 0.032s、diag 0.012s、fmt 0.018s、lex 0.006s、lsp 0.015s、parser 0.085s、typecheck 0.320s、version 0.028s、runtime 48.000s（`cmd/we`、`internal/ast` 无测试文件）；语料账 **881 枚**（本轮盘存 `internal/conformance/testdata/cases/*.json` 计数，conformance 包 `-count=1` 全绿即受影响黄金面；B1b 起点 810）；`validate.py --all --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`；`docs_sync.py` → `OK: 33 document pair(s) aligned`；`git diff --check` **空**。
- [x] **突变电池**：每簇的守卫落单点突变（逐枚还原、锚点先断言 `count==1`），**单测层 + 黄金层各记判决**
  来源：design D12
  验证：突变逐枚记录（存活 / 被杀 + 被哪一层杀）；~~T5 的盒描述符位~~ **订正（T6-4 查证）：「本 build 不可观测」而非「须被捕获」**——可派发的盒与载荷被独立扎根的盒是同一集合，任何黄金都看不出描记位的差别，故该项从本电池撤下（论证见 T6 披露 8）；**T10 的遮蔽回归钉仍须被黄金层捕获**（B1a T6 的教训：单测层捕获、黄金层空缺须显式披露）
  **落地记（2026-09-15）**：汇总表（单点突变、逐枚还原、锚点先断言唯一命中；判决来源 = 各任务落地记与 design 补记）——

  | 任务 | 突变 | 单测层 | 黄金层 | 判决注记 |
  | --- | --- | --- | --- | --- |
  | T2 改编 | `sym()` 恒追加 `$` | 42 处失败 | —（既有快照面即见证） | 面敏感的承重实证 |
  | T3 驱动 | `instDepthCheck` 挖空 | **杀**（自我加深挂死、20s 超时；同包 339 枚仍绿） | — | 守卫承重 |
  | T4 分类器 | 泛型被调读中和 | — | **杀**（恰翻转 `run-ch10-generic-application-in-interp`，其余全绿） | 保留读的单点证据（两读一留一删） |
  | T5 盒 | M-t5-a 永远有描述符+每字描记 | **杀** | **杀**（SIGSEGV `terminated by signal`） | 两层齐杀 |
  | T5 盒 | ~~M-t5-b 撤全部描记位~~ | 单测 4 红 | 静默 | **撤下**（T6-4：可派发盒与独立扎根盒同集，本 build 不可观测——验证行原文的订正在案） |
  | T6 vtable | M1 恒置 dispatchable | **杀** | 补 `build-bnd-dyn-value-payload-dispatch` 后**杀** | |
  | T6 vtable | M2 槽下标恒 0 | **杀** | 补 `run-dyn-vtable-two-slots` 后**杀** | |
  | T6 vtable | M3 接收者传表不传盒 | **杀**（3 红） | **杀** | 两层齐杀 |
  | T6 vtable | M4 按头合并表 | **杀** | **杀**（`1\n1\n` 静默错值） | 两层齐杀 |
  | T7-4 对象 | A/B 两枚描述符位 | 静默 | 静默 | **不可观测不冒充**（与 T6 披露 8 同因：每分配独立扎根） |
  | T7-4 对象 | C 单摘 `elem.gc` 守卫 | 静默（与 `strKindName` 互为冗余） | C3 同摘两道 → 负例黄金**杀** | 冗余守卫以「同摘」钉 |
  | T7-4 对象 | D `next` tag 极性翻转 | **杀**（三正身） | **杀**（高声） | 两层齐杀 |
  | T8-1 collect | 逐元素根纪律四代突变 | 静默 | 静默 | 插桩实证两构建清扫差恰 1 块（火点根账 4 对 3）——纪律承重但不可观测，**保留不冒充** |
  | T10 值塔 | M1 摘 `pushEnv`/`popEnv` 块帧 | 初轮**存活** → 补 `TestBlockKindLeavesTheOuterFrameAlone` 后**杀** | 初轮**存活** → 黄金扩两 println 后**杀**（M1 树产非法 IR、clang 拒收） | **真覆盖缺口补钉**；**遮蔽回归钉的黄金层捕获要求就此兑现**（验证行点名的那条，B1a T6 教训闭合） |
  | T10 值塔 | M2 / M3 / M5 | 齐杀 | 齐杀 | 两层齐杀 |
  | T10 值塔 | M4 / M6a/b/c | 存活 → 各补钉后齐杀 | 齐杀 | 黄金先杀、单测补钉 |
  | T11 载体 | M1 描记谓词撤 gc 面 | **杀** 1 | **杀** 1 | |
  | T11 载体 | M2 `traces()` 撤 skStr | **杀** 2 | **0——结构性静默**（插桩差分：净树 swept32=21300 vs M2 树 21400，恰 = ys 百枚盒真被回收；机理 = first-fit+分裂+清扫 LIFO 重建+1MiB 阈值使回收块回读前永不被复用，且回收只写 map@0、回读只读 16/24） | 机理已测得、单测层双钉在案 |
  | T11 载体 | M3 盒形字段撤 | **杀** 1 | **杀** 5 | declare 级击杀如实标注 |
  | T11 载体 | M4 / M5 | 各**杀** 1 | 各**杀** 4 | |
  | T12 `?` | M1 Ok 面门 | **杀** | **双因掩蔽**（M1 二进制探针二分定名：`?` 放行、停点在 `${v}` String 位分类器） | 「either alone holds」循 T9-3 先例 |
  | T12 `?` | M2 Err 行门 | 门表测**杀** | **杀**（三枚） | |
  | T12 `?` | M3 撤调用侧再扎根 | — | churn 形静默；持有探针实打 `174761`（应 41）错值 → 补 `run-prim-return-gc-hold` 后**杀** | 危害真、黄金形初选错，按 D10 补形击杀 |
  | T13 scope 值 | M1 摘早退恒建和式 | **杀** | **杀** | 两层齐杀 |
  | T13 scope 值 | M2 摘 skStr 臂 | String 钉**杀** | String 黄金**杀**；数值面存活 | **设计预期分界**非覆盖缺口 |
  | T13 scope 值 | M3 摘 typeName/num 携带 | **杀** | **杀**（钉①自带 `"v "` 插值消费，强于预案） | 两层齐杀 |
  | T13 scope 值 | M4 尾先于体求值 | **杀** | **杀**——错值 `v 14` 对 `v 7` | SHADOW 黄金形的设计目的兑现（D10-2 复辟陷阱） |

  汇总裁决：**存活项三类各带机理在案**——本 build 不可观测（T5 M-t5-b、T7-4 A/B、T8-1）、结构性静默（T11 M2）、双因掩蔽（T12 M1），无一冒充捕获；补形后击杀两处（T12 M3、T10 M1）皆按 D10 一正一负政策落形。
- [x] 负断言汇总：`WE_UPDATE_GOLDEN=1` **未使用**；staged 无 `refr/`；无新诊断码；`ast.go` 零改动；typecheck 塔零改动；`fitAbi` 零新臂
  来源：design D11/D12；AGENTS.md
  验证：`git diff --cached --stat` 逐条核对；提交信息无署名 trailer（`.githooks/commit-msg` 拦截）；逐项以命令输出为准，不以叙述代断言
  **落地记（2026-09-15）**：逐项命令取证——
  - **staged 无 `refr/`**：`git status --porcelain` 仅 `?? openspec/changes/codegen-full/`；`git diff --cached --stat` **空**（staged 零文件，`refr/` 无从混入；`.githooks/pre-commit` 另有机械拦截）。
  - **`WE_UPDATE_GOLDEN=1` 未使用**：全程未导出；佐证 = 语料 881 枚全部经 29 枚提交逐枚披露落地——14 枚翻绿重锚逐枚手工改期望（机械再生成会把期望整文件覆写，与在档的逐枚红先证据矛盾），新增黄金全部手写。
  - **无新诊断码 + `ast.go` 零改动**（同一命令双证）：`git diff 0030d60 --stat -- docs/spec/diagnostics.toml internal/ast/` **空**。
  - **typecheck 塔零改动 → 沿 T14-③ 精确措辞**：按字面为假（T1 章程即出口扩形），成立的是**词面与判据零触碰**——`git diff 0030d60 -- internal/typecheck/typecheck.go` 共 83 行，全为 `Shapes`/`Site`/`noteApply` 登记面；diff 的 `^\+` 行对七个词常量（`bndGenericFns`/`bndMainBody`/`bndFnBody`/`bndTaskBody`/`bndTopLets`/`bndStdModules`/`bndArityGap`）grep **取空**（exit 1）；唯一视觉命中 `c.bnd(bndArityGap)` 是**上下文行**（相邻 `newtypeCall` 尾部加 `noteApply` 而入 diff，非本变更改动）。`shape.go` +385 / `shape_test.go` +470 是 T1 章程自身的导出面。
  - **`fitAbi` 零新臂（按「臂集恒等」验证）**：基线 `0030d60` 与 HEAD 的 `fitAbi` `case` 行集 **9 ↔ 9**、`diff` **空**——T12 改的是既有 `abiPrim` 臂体（`retTyp="ptr"` 放行），T5 走 `classType` 的 Dyn 分支不入 `fitAbi`，无新臂。
  - **提交信息无署名 trailer**：29 枚 B1b 提交（`0030d60..HEAD`）`git log --format=%B | grep -ci co-authored` = **0**（`.githooks/commit-msg` 机械拦截之外的双证）。
  - 附带双证：`git diff 0030d60 --stat -- runtime/` **空**（D7-附 运行时零改动决策的机械兑现，例外口未触发）。
- [x] `welang-change-review` 过审（读 `.agents/skills/welang-change-review/SKILL.md` 手动执行）
  来源：process §3.5
  验证：审查记录入完成记录；审查期改动以**独立提交**落（B1a 的先例 `0aac57b`）
  **落地记（2026-09-15）**：**十条全过**。结构面先行：`validate.py codegen-full --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`（实现完成态复跑，`--all --strict` 同绿）。审查记录 = proposal.md「## 审查记录」之下新增 **2026-09-15 复审块**（十条结论表 + 发现与处置）：职责边界 ✅（:51 删除线订正在案）、spec 增量 ✅（零增量双命令实证）、design 路径与被拒方案 ✅（14 节 + 补记一至二十）、tasks 纪律 ✅（49 项勾选全挂来源、无虚勾）、场景覆盖 ✅（11 负例守卫 + 2 枚 E0501 语言面钉）、无空章节 ✅、测试先行 ✅（T4 红基线 + 各任务红先证据在档）、负向断言真实 ✅、三要素闭环 ✅（含 `runtime/` 零 diff 机械兑现）、未决阻塞 ✅（四枚前置解毕带落点 #24/#25 等）。**一枚措辞级发现 F1**：本复选框 ③ 的「typecheck 塔零改动」按字面为假（T1 章程即出口扩形）——按 T14-③ 先例以 ③ 落地记的精确措辞（词面与判据零触碰 + 83 行 diff 全为登记面 + `^\+` grep 取空）订正，复选框原文不动。**审查期零 tracked-file 改动 → 独立提交一条不适用**（B1a 先例 `0aac57b` 是为真码级发现而设；本轮唯一产出是本四枚落地记与 proposal 审查记录，均落未跟踪的变更目录，随归档入册）。status 已为 `active`，技能产出的「通过后置 active」一步 N/A。
