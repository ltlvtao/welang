# We 评测套件

这是 We 语言的评测方法论文档——语言编译回路主张如何被测量的唯一权威。它定义三指标、任务分类体系、陷阱清单格式、批次 schema、反馈协议、跑器判定序与统计纪律。两个机器面消费它：内嵌任务集（`internal/benchmarks/tasks/`）与批次跑器（`internal/benchmarks`，Go）。机器面与本文不一致时，以本文为准，机器面承缺陷。

前提是第 0 章的一等作者面。generate–check–fix 循环里模型写、检查器拒或过、模型再写——编译器是回路中唯一不学习的组件。它的四项承诺（可观察、可重复、可操控、可归因）在被一套套件对着模型写出的代码测量之前，都是实验主张；本套件就是那个测量。每个判定都归结为 `we check` 与 `we test` 的退出码与事件流——正是第 21 章为回路本身稳定的机器面，整套件因此可复现。

三条范围门框住下文一切。套件只判定不生产：模型调用与反馈构造是批次生产面的工作，跑器的唯一输入是批次文件——本 schema 下的仓库工件。不新增 CLI 面：`we bench` 不存在也不可能——第 21 章命令集是封闭面；跑器是仓库工具，经 Go 测试 in-process 驱动。种子任务集证明的是方法论而非语言——任何关于 We 的比较性结论都不出自种子集。

## 三指标

一切指标从 attempt 桶分类导出。每个 attempt 面对两段判定——先 `we check --json`，check 过后才 `we test --json`——落入且仅落入五桶之一：

| 桶 | 判定 | FPCR | LBR |
| --- | --- | --- | --- |
| `clean` | check exit 0，test exit 0 | 分子分母皆计 | 分母 |
| `latent` | check exit 0，test exit 1 | 分子分母皆计 | 分子分母皆计 |
| `test-malformed` | check exit 0，test exit 2 或 70 | 分子分母皆计 | 只计分母，不计分子 |
| `rejected` | check exit 1（E 级诊断事件） | 分母 | 不入 |
| `boundary` | check exit 70 或 2（诚实未实现形，或工具用法畸形） | 两侧皆排除，单独披露 | 不入 |

退出码是第 21 章的稳定契约：0 通过、1 发现 E 级诊断、2 用法畸形、70 本参考构建未实现的已批准形。advisory `W` 事件永不动判定——warning 缺省下工具报告它但照样 exit 0，「过」就是 exit 0 的机器事实，没有第二种读法。

**FPCR（First-Pass Compile Rate，首过编译率）**在全部任务的 round-1 attempt 上测量：(clean + latent + test-malformed) / (clean + latent + test-malformed + rejected)。首过计的是「编译通过」——测试随后说什么不在此问；测试结局是 LBR 的问题。boundary attempt 两侧皆排除：诚实边界既非模型过错也非对语言的判决，计入任一侧都会污染数字。boundary 占比随指标一同披露。

**FLC（Fix-Loop Convergence，修复回路收敛）**测的是首过被拒后回路的形状。反馈协议（下文）钉死轮 k 的输入：原需求 prompt 原文 + 轮 k−1 的 `we check --json` 事件流原文。序列在其首个 check exit 0 的轮收敛；收敛轮数即该轮轮号。序列在 N=5 截尾：五个 attempt 内无 check-pass 轮的序列计 not-converged——截尾显式，不折算轮数。报告携带收敛轮数分布（median、p90、截尾值处的 max）与 not-converged 计数。

**LBR（Latent Bug Rate，潜伏缺陷率）**在全部轮次（非仅末轮）的全部 check-pass attempt 上测量：latent / (clean + latent + test-malformed)。attempt 级是机械无歧义面。任务级衍生视图并列报告——末轮 check-pass 的任务中 test 挂的比例，直观读法——两者命名分立，永不混同。

解读纪律：FPCR 单独不是语言质量数字——rejected attempt 是正面证据，语言抓住了模型犯的错。判决面是联合读法：FPCR 配 FLC（是否首过就编译通过；不过时回路收敛多快）再配 LBR（什么从检查器漏到测试）。一门全拒的语言 FPCR 为 0；一门全收的语言 LBR 为 1；设计的主张活在中段，只有联合读法测得到它。

## 任务分类体系

任务按错误根因分类而非业务领域——分类轴测的是语言设计本身消除了哪些错误类。七轴，每轴两个 L1 任务：**error-path**（可失败调用的失败分支未处理）、**race**（并发任务在同步纪律之外改共享状态）、**resource-leak**（句柄逃逸或在 scope-resource 纪律之外释放）、**null-boundary**（`Option` 分支集缺一臂）、**implicit-conversion**（显式转换设计拒绝的混宽算术）、**dangling-reference**（转移后使用与别名纪律——见下文可表达性披露）、**context-passing**（隐式上下文捕获与传递纪律——同前）。

复杂度只到 L1——单函数级 10–30 行。模块级（L2）与系统级（L3）任务是后续扩充，已登记 roadmap follow-up；种子集先证明方法论。

四枚**反向对照**任务（`control-01`..`control-04`）是纯算法与字符串工作，不触任何轴。它们把轴面位移可能携带的两种解释分开——「语言设计消除了该错误类」与「模型单纯觉得这套语法顺手」——对照任务只随第二种因素移动。

每任务目录 `internal/benchmarks/tasks/<轴>-<nn>/`（对照为 `control-<nn>`）四件：

1. `prompt.md`——需求描述，英文。prompt 是真实批次的机器面；LLM 提示是评测基线，跨模型比较需要单一提示语言。
2. `traps.md`——陷阱清单（下节）；评分面，永不给模型看。
3. `reference/`——参考解完整项目（`we.toml` + `src/`），构造上 check 全绿 test 全绿——自证测试机器强制两绿。
4. `reference/tests/`——参考测试面：测试随任务而来，非 attempt 产出；对参考解 `we test` 全绿。

**attempt 组装。**跑器复制参考项目到临时目录，把 attempt 的 `files`（路径 → 源文本 map）覆盖到 `src/` 上。测试面固定——评分客观——模型的自由度是整个 `src/`：重构布局、新增模块，皆合法。若重构破坏参考测试的导入，test 段 exit 2，attempt 落 `test-malformed`——诚实分类，非跑器错误。

**可表达性披露。**两轴值得单列一段。悬空引用与隐式上下文的经典诱饵形被设计本身关死，两项独立裁决：引用类型终身不引入 We（第 18 章裁决——`Ref<T>` 不存在）；模块级可变状态不存在（`E0403`：无顶层 `var`）。不可表达的形做不成任务。context-passing 轴因此锚在相邻可表达形上——捕获与 effect 纪律（`E1602`/`E1603`）；dangling-reference 轴锚 task 句柄生命周期——句柄到 scope 退出未 await（`E1607`）与任何 scope 之外 spawn task（`E1618`）——因为本参考构建的 byres 运行面未实现（byres 资源在任何函数体内的使用都被运行面拒斥），`E1104`/`E1105` 留作 check 锚定的诱饵形、由 traps.md 在 prompt 邀到处注记。每枚此类任务的 `traps.md` 披露映射。这本身就是测量的一部分：模型写出被关死的形时 `we check` 直接拒——FPCR 受抑、回路收敛，正是设计生效的测量面。

**运行面锚定。**参考构建的运行塔实现已批准语言的刻意子集，L1 任务锚在其内。该子集随 codegen-mono 线拓宽：sum 型可构造、可返回、可 match——用户函数可收可返 `Result`/`Option`/自有 sum，sum 因此不再被限制在产出它的那几枚原语调用里；sum 型与同步型参数入可跑槽；数值返回可为任意 checked 整型宽度而非仅 `Int64`（`fn sum(a: Int16, b: Int16) -> Int16` 可跑，且其算术按 `Int16` 检查，溢出陷阱点名该宽度）；`String` 相等与拼接可跑，记录方法、元组、新类型、`List` 迭代、闭包与 fn 值、顶层绑定亦然；`%` 与 `/` 发射检查算术；函数体可从任意深度 return，不限尾位；由值位 `if`/`match`/块绑定而来的 `let`（各分支同为标量时）亦可读入 `String` 位。codegen-full 线随后十一次拓宽该子集：泛型声明的应用按检查阶段定下的形单态化，`fn id<T>(x: T) -> T` 应用于 `Int64` 即可端到端跑通；`for` 的源亦可是 impl 绑定了 `Iterable` 关联型的记录，按协议迭代——`iterator` 在循环头之前跑一次，每轮 `next` 一次，首个非 `Some` 的 tag 结束循环——故用户自定义序列与内置的 `Range`/`String`/`List` 并列成为可跑的源；内置源的迭代器亦可进盒，`Dyn<Iterator<Int64> >(xs.iterator())` 造出一枚持有载体与位置的迭代器对象，`next` 经盒的表派发——故进盒面不再限于自身声明了 impl 的记录；`collect()` 亦加入急性组合子成为第七枚——在内置 `List` 或协议源上，它以字面量建载体的方式作答，遍历过的每个元素推一枚，故收集得的 `List` 本身即可作后续 `for` 或组合子的可跑源，其元素面即遍历自身的元素面；`map`/`filter`/`take`/`skip` 亦作为惰性四枚加入——在内置标量元素源上，每次调用以一枚持有自身适配器对象的盒 `Dyn<Iterator<U>>` 作答，链是盒持盒地穿起来，元素工作发生在急性组合子走查最外层盒之处，故 `xs.iterator().map(f).take(2).collect()` 端到端可跑，`map` 的 `U` 可以是检查阶段记下的任意标量（`Float64` 以自身域穿行）；急性七枚亦可读以名字指盒的接收者——`let it = xs.iterator().map(f)` 之后 `it.count()` 走查的正是链作答的那枚盒，每轮经其表一次 `next`，链在其自身调用处只建一次，显式进盒的内置迭代器（`let b = Dyn<Iterator<Int64> >(xs.iterator())`）绑定后亦以同法走查；`reduce` 与 `find` 作答的 `Option` 对子按元素自身的面携带载荷——`Float64` 的字即其 double 位型、gc 记录的字即其句柄——绑定 `Some(v)` 的臂与渲染它的 `String` 位都从同一面把值读回（`got 3`、`sum 4`、`found 7` 端到端可跑），而多字载荷——`String`、元组——仍停：一趟只有一个字可给；而值塔的五簇拓宽回应的是这些形周围的位置本身：值位块按其自身的帧分类，块在尾表达式之前重绑名字的，答案出自重绑——`{ let t = a + 1; t * 2 }` 渲染 `8`，块内 `let b: Int64 = 5` 遮蔽外层布尔 `b` 时渲染的是内层 `5`，绝非外层面；各臂皆答 `String` 的控制形在其自身的 String 对子上 join，故 `if c { "a" } else { "b" }` 以自身绑定与渲染，全程无转换器；注解未点名的 `var` 按其初值自身的分类定面，故 `var s = "a"` 可绑定、可赋值、可打印；fn 值在值串位或操作数位被调用时可跑——`"${f(1)}"` 与 `f(1) + 1`——与早已可跑的语句位、尾位并列；元组值读入 `String` 位时按其整体形状渲染，`(1, 1.5, true, a)`——每个元素按自身偏移载入、按自身面渲染，一字一转换器——而规则域未点名的元素面（gc 记录或 sum）作为诚实边界停下；而 `String` 元素的源今已是可跑的载体：字面量把每个元素装进一枚无描述符的 gc 对象——对子的指针在 16、长度在 24、map 字恒 null（`String` 的字节在收集器域之外）——载体描记这些句柄一如描记记录的句柄，`for` 从盒里把对子读回，`count` 与 `collect`——从不把元素递给回调的两枚——在走查旁一并开（收集得的载体本身又是 `String` 元素源），载体携其元素面过签名（`fn tally(xs: List<String>)` 可跑）；用户 `Result` 上的 `?` 今在入口界线的两侧皆可跑——返回 Result 的 fn 体内三字整体经 return 的出口协议传播，故 fn 把内层调用产出的错误原样交给调用方，入口侧的报告行由载荷在运行期渲染（`error: Failed: boom`——头是编译期常量、载荷对经运行时自身的拼接连接、消息是程序自己的数据而非折叠常量），绑定的 Ok 载荷按其声明面读取（`v 7`；`String` 载荷以同法绑自身对子——`let t = fs.readFile("f")?` 渲染的即文件自身的字节）；门以面不以字面拼写：Ok 载荷非「一字或一对 String」、或错误侧非「恰一变体且载荷为 String 或空」者诚实停而非错绑（fn 体内只把 Ok 面——这样的错误面自由传播）；同步型的返回位今以自身指针过界——`fn make() -> conc.Mutex<Int64>` 返回 `conc.Mutex(41)` 可跑，调用方在调用处为新鲜答案补根，三种调用形（main 内绑定、fn 内绑定、内联构造作实参）打印同一个 `42`；普通与 `collectAll` 的 `scope` 块，其值今即体的块值自身——数值尾绑定的正是「对尾本身作 `let`」会绑定的那枚 SSA 字，故 `let r = scope { let n = 3  n + 4 }` 作答 `7`，绑定、算术、比较、插位读的都是同一枚字，String 尾以同法绑定其对子；而 `timeout` 各形保持规范所定的 `Result`——`Ok(b)`/`Err(TimedOut)` 居于和式所持的三字——故拿该 `Result` 与其载荷相比（`scope timeout(1000) { 7 }` 之后 `if r == 7`）是检查器自家的 `E0501`，属语言面而非构建边界。stdlib-fsproc 线随后再拓宽该子集：fs 七入口以槽载调用可跑，经 out 参三字组答一枚熔合 `Result`——`fs.readFile` 的整读、write/remove/mkdir 诸位的单位作答、`fs.listDir` 的有序句柄——错误侧由 C 族自身的消息渲染（`read no-such-file: No such file or directory`），`listDir` 的 Ok 句柄在调用处补根、以 `String` 元素源走查，而持 NUL 字节的路径在任何 syscall 之前即遭拒绝；process 入口以同一形加入——`process.run` 将一条命令跑到完（无斜杠者按 PATH 搜索），两根输出管经同一 poll 循环排至 EOF，经 out 三字组作答、Ok 侧是 C 侧雕刻的记录——`exitCode`、捕获的 `stdout`/`stderr` 对子，句柄在调用处补根——正常退出报其退出码、信号致死报 128 加信号号，错误侧由 spawn 自身的 errno 渲染（`exec /no/such/cmd: No such file or directory`），命令或参数内含 NUL 字节在任何 spawn 之前即遭拒绝，空参数表以签名命名的 `List<String>` 面过界。string 模块以本线第一份真体加入：`join` 与 `repeat` 不声明 effect 段——第 16 章的显式纯性——故该模块乘程序面而非键控表，管线以其发射程序模块 fn 的方式发射二者的 define，调用点走的槽形与每枚键控入口所走的相同、槽指向 define 自身；二者于任意上下文可跑，纯助手的体与 main 的体一样——`string.join` 作用于空、单元素、多元素的 `List<String>`，`string.repeat` 于零次或 n 次——调用的答案与程序 fn 的一样，可嵌作实参或插值洞。stdlib-collections 线随后再一次拓宽该子集：`List` 载体今为稳定句柄，别名因此看得见增长——`let ys = xs; xs.add(4); ys.size()` 打印 `4`（两句柄、同一对象），而 `iterator()` 仍快照自身前缀——十三员成员面在真数据结构上可跑：`List` 的 `add`/`removeAt`/`get`/`size`（`get` 作答 `Option<T>`、越界即 `None`——表面语义的缺席，绝非载体的陷阱）、`Map` 的 `put`/`get`/`remove`/`keys`/`size`、`Set` 的 `add`/`remove`/`has`/`size`；`collections.mapOf(keys, values)` 与 `collections.setOf(items)` 构造它们——开放寻址表、线性探测、倍增 rehash——`String` 键按内容作 FNV-1a 哈希、按内容比较，两表长度不一的 `mapOf` 以任务自身的失败作答（`error: Panicked: mapOf length mismatch`）；`Map`/`Set` 以一个指针字过签名（`fn fifth(m: Map<Int64, Int64>) -> Int64` 可跑），收集器描记两区，故 `String` 键盒与 gc 引用值在越过阈值的 churn 后存活；正典迭代习惯形端到端可跑——`let ks = m.keys(); for k in ks` 而后 `m.get(k)`。今仍关死的面分两种，分开写——先 codegen 停面（皆拓宽在打开邻面时自身揭出的诚实 exit 70，皆在此披露）：Ok 载荷非「一字或一对 String」的 `?`——元组、`Float64` 的字位、被读作数字的 list 或 record 句柄——以及——仅在入口——错误侧非「恰一变体且载荷为 String 或空」的 `?`、同步对象读入 `String` 位、把元素递给回调的五枚组合子作用在 `String` 元素源上——`List<String>` 上的 `fold`/`reduce`/`any`/`all`/`find`，或 `Iterable` 关联型产出 `String` 的记录上的同五枚（回调的参数得是两字的 `String` 本身，而载体的字是其句柄）、gc 元素源进盒为迭代器（`List<Cell>` 之于 `Dyn<Iterator<…>>`）、`String` 元素迭代器进盒后的载荷读（`Dyn<Iterator<String> >` 可构造、`let o = d.next()` 可绑定；绑定 `Some(s)` 并读取的臂停）、该盒上的急性组合子（`d.count()` 停）、读 `timeout` scope 所答 `Result` 的臂——`scope timeout(…) { … }` 之上的 `match r { Ok(v) => … }` 过检查却在构建停——以及读任何 `next` 所产 `Option` 的臂——内置迭代器对象自身的或用户 `Iterator` 的（`match b.next() { Some(v) => … }` 停；裸绑定 `let o = b.next()` 可跑）、集合自身的域门——`Map`/`Set` 的键域即 Eq 域（八种整型宽度、`Bool`、`String`），`Float64` 键的构造停在域标签处，值域即 List 元素域的那一个字，sum 或元组值因此停；成员面作答的 `String` 载荷 `Option`——`List<String>` 的 `get`/`removeAt`、`Map<K, String>` 的 `get`/`remove`——停，载荷槽一字而 `String` 对两字，与载荷面簇所披露的同一行；直接的 `Map`/`Set` 迭代（`for (k, v) in m`）按申报的非目标维持停，受认可的习惯形即上面的 keys 后 get；链式集合接收者（同一表达式内 `mapOf(...).put(...)`）停在调用处，绑定形可跑——急性链早已遵守的先绑定规则——皆诚实边界（exit 70），皆在此披露。工具面停面是裸句柄绑定：`let it = xs.iterator()`——无惰性链、无显式进盒——停在绑定自身，成因在检查塔而非此处：检查器把该调用定型为裸 `Iterator` 接口而非盒的擦除面，这是 codegen 拓宽不拥有的语言面取舍，其上的急性调用随之同停；链绑定形与显式进盒拼写可跑。另有一条运行期规则根本不是边界、只是未变：等待属于 task 体——main 纤程停在自己的 timeout scope 内 `receive` 在虚拟钟下死锁。codegen-full 线起跑时本清单载着的六面皆已离场，各随关掉它的簇：泛型函数（单态化簇）、读作值的普通 `scope { … }`（scope 值簇）、main 体的 `?`（用户 Result 簇）、元组值绑定读入 `String` 位（值塔的元组渲染臂）、N1——fn 值在值串位或操作数位的调用（`"${f(1)}"`、`f(1) + 1`），由值塔的 fnEnv 臂关掉，故早已可跑的语句位、尾位今与这两位并列——以及 N2——内置所产 `Option` 载荷（`reduce`/`find`/`next`）读入 `String` 位，由载荷面簇关掉：`reduce` 与 `find` 今按元素自身的面作答，`next` 的绑定自身其后亦开，停点移到读该 `Option` 的臂。stdlib-fsproc 线其后又送走一面，非靠拓宽而靠诚实分类：被局部绑定压过的模块限定名（`let string: Int64 = 7` 遮蔽 string 导入、其后 `string.join(...)`）不再停于边界词——检查器答 E0816「no such member on the receiver's type」，`Int64` 无成员、锚定族之外的标准库表面未获规范批准；同一翻转把基类型/集合接收者上的未知 std 成员（`s.length`、`xs.push(1)`）也变成同一诊断，exit 70 之位换 exit 1。子集不再随 codegen-full 线收缩——那条线已落地；仍关死的是拓宽自家的发现：上文的 codegen 面与那一枚工具面，各已披露、各等各自的后续变更。这些约束塑造任务编写，不削减模型自由度：越出子集的 attempt 被分类（boundary 或 test-malformed），绝不静默通过。

## 陷阱清单

每任务的 `traps.md` 承载需求所邀请的陷阱。每陷阱一条：一行点名错误形状，一行给出检测路——注册表码（`we check` 抓住）或参考测试断言（`we test` 抓住）。清单是评分面；永不进入 prompt。

陷阱清单是受检工件，不是散文。校准测试把每陷阱的典型错误形注入参考解，断言 check 或 test——至少一路——抓住它。没有东西抓住的陷阱是任务设计缺陷：修任务或修清单并披露错过，绝不静默留下。

## 批次 schema 与反馈协议

批次文件是一个任务的 attempt 序列——仓库工件：

```json
{
  "model": "string（自由文本标识）",
  "task": "任务 id（必须存在于任务集）",
  "attempts": [
    {"round": 1, "files": {"src/main.we": "…"}},
    {"round": 2, "files": {"src/main.we": "…"}}
  ]
}
```

装载期校验直接拒批于：`model` 为空；`task` 未知；round 不从 1 连续（无跳号无重复）；`files` 为空；任何路径越出 `src/`——触 `tests/` 或 `we.toml` 是篡改评分面，拒绝而非分类。`src/` 下新增文件依上节组装模型合法。

**反馈协议**定义轮 k（k>1）attempt 的生成输入是什么：原 `prompt.md` 原文 + 轮 k−1 的 `we check --json` 事件流原文——逐行，code、message、file、line、column、help。这个字面形状就是 generate–check–fix 回路：模型读机器自己的话。跑器永不构造反馈——判定与生产分立（一类事实一个权威位置：判定归跑器，批次生产归生产流程）。合成批次按此协议形状手写每轮文本；真实批次的文本由其流程自行产出，跑器分不出两者——by design。

## 跑器判定序

每 attempt 四段有序：

1. **组装。**复制参考项目到全新临时目录；attempt 的 `files` 覆盖 `src/`。attempt 结束后目录删除。
2. **check。**经 in-process CLI 入口跑 `we check <dir> --json`——conformance runner 同款注入流面：无子进程、无新入口。exit 0 进入下段；exit 1 落 `rejected`；exit 70 或 2 落 `boundary`。
3. **test**（仅 check-pass）。同路跑 `we test <dir> --json`。exit 0 → `clean`；exit 1 → `latent`；exit 2 或 70 → `test-malformed`。
4. **分类记录。**attempt 的桶、退出码与诊断码入报告。

确定性是套件的属性而非愿景：零时钟、零网络、零路径泄漏。工作目录纪律同 conformance runner（进、跑、恢复），任何机器可变量不入报告——同一批次文件每次运行产出逐字节相同的报告。

一项已披露限制：评测是 in-process 的，编译产物永不终止的 attempt——无等待源的忙循环，死锁检测器唯一看不见的形——会挂起评测。死锁形错误 attempt 在虚拟钟下确定性中止、正常分类；真实批次生产面在喂入不受信 attempt 前必须加墙钟守卫。

**报告**是 JSON 机器面：逐 attempt 桶、逐任务视图、三指标（round-1 FPCR；FLC 分布与 not-converged 计数；attempt 级 LBR 附任务级衍生视图）、boundary 与畸形计数（永不缺席——见下节）、每个比例背后的样本量。

## 统计与报告纪律

- 一切比例附分母。报告中不存在裸百分比。
- 截尾显式。FLC 于 N=5 截尾；截尾序列计 not-converged，不为其折算轮数。
- boundary 与 test-malformed 桶永不静默。每份报告必带两桶计数——exit 70 诚实边界披露纪律延伸到评测。
- 重放逐字节相同（上文确定性）；临时路径永不入报告——只有任务 id、桶与指标。
- 真实批次文件是本 schema 下的仓库工件；报告是可再生产物，非明示不入仓。
- 种子集不承载统计结论。十八枚任务证明方法论可跑通；不支撑任何关于语言的比较性主张——那是真实批次的职责。
