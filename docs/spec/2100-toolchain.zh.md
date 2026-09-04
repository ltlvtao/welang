# We 语言规范 —— 第 21 章：工具链

### Requirement: `we` 命令面

`we` 是唯一的命令行入口，子命令模型：每个工具链操作是 `we <subcommand> [path] [options]`。子命令集为 `we new`、`we build`、`we check`、`we run`、`we test`、`we fmt`、`we vet`、`we doc`、`we clean`、`we version`、`we lsp`——本章固定该集合；集合之外的子命令是 shell 的错误，不是诊断。`[path]` 参数可选、默认工作目录：目录路径名一个项目——持有项目清单的目录，循第 15 章——文件路径名一个 `.we` 文件、作为它自己的单文件编译。名不见经传的路径 MUST 以 `E1907:` command path not found 报告。单文件编译中，被命名的文件就是整个编译：`std.` import 从编译器内建模块解析，任何其他 import 以 `E1302` 拒绝——没有源根可资映射，报文直说。`we new <name>` 创建骨架——一份清单、带 `pub fn main` 的 `src/main.we`、`tests/` 下一个空测试模块——超出第 1 章命名约定的 `<name>` MUST 以 `E1904:` invalid project name 拒绝。`we clean` 移除构建产物，不动源与清单。`we version` 以固定形状打印编译器与规范的版本，数值本身在本规范正文之外——第 0 章机制中立纪律：版本值永不入规范，只入形状。全局选项每个子命令都接受：`--json` 选本章 JSON Lines 协议要求所定的协议、不改退出码；`--color` 取 `auto`、`always`、`never`，`auto` 为默认；`--verbose` 加宽人类可读输出。

#### Scenario: 目录路径名一个项目

- **WHEN** `we check .` 在项目清单存在的目录中运行
- **THEN** 编译循第 15 章把该目录当作项目根，源根是其 `src/`

#### Scenario: 单文件自成编译

- **WHEN** `we check tool.we` 在只 import `std.io` 的文件上运行
- **THEN** 编译从内建模块解析该 import、无需清单；同文件中的本地 import 是 `E1302`，报文名说单文件模式下没有源根

#### Scenario: 名不见经传的路径被拒绝

- **WHEN** `we build nosuchdir` 运行而没有文件或目录应答该路径
- **THEN** 命令报 `E1907:` command path not found 并非零退出

#### Scenario: we new 检查项目名

- **WHEN** `we new My_Project` 运行
- **THEN** 命令报 `E1904:` invalid project name 且什么都不创建——名字循第 1 章命名约定

### Requirement: 编译管线的可观察契约

`we build`、`we check`、`we run` 共享一条管线：词法分析、解析、模块解析、类型检查、效果检查、所有权检查、文档检查，依次而行——次序是可观察的，因为同时持有语法错与类型错的文件报的是语法错。任一阶段的 `E` 级诊断止住管线：后续阶段不运行、不产产物；`W` 级诊断自身永不止住管线——被清单 `[vet]` 表提升为错误的建议性诊断如错误一样止住它。`we check` 止于文档检查、不产产物——第 0 章生成–检查–修复循环的检查腿不需要代码生成；`we build` 继续过代码生成、产清单所名的产物；`we run` 构建后执行所建产物。唯一的性能承诺是输出的确定性、从不是时间：同输入产同输出，增量编译、缓存与并行策略是实现细节、不载任何规范承诺。链接归 `we build`：`we check` 不链接，foreign 声明的链接发生在构建时、循本章其要求。目标选择——为宿主之外的平台构建——是 `we build` 独有的事务，其目标词汇是工具链的机制、不在此固定。

#### Scenario: 阶段次序可观察

- **WHEN** 一个文件同时持有语法错与类型错
- **THEN** 被报告的诊断是语法错——管线止于第一个失败的阶段，类型错不在同一轮中被报告

#### Scenario: 错误止住管线，警告不

- **WHEN** 一个项目带一警告零错误编译，再加一个错误编译一次
- **THEN** 第一轮完成、产产物、报警告；第二轮止于错误所在阶段、报错、什么都不产

#### Scenario: we check 不产产物

- **WHEN** `we check` 在干净项目上运行
- **THEN** 全管线到文档检查为止会报的每个诊断都被报告，不写任何构建产物

#### Scenario: 只承诺输出

- **WHEN** 一个项目在缓存策略不同的两个工具链版本下无源变更地构建两次
- **THEN** 产物相同；这里没有任何东西承诺第二次构建更快、更慢、或根本是增量的

### Requirement: 格式化器

`we fmt` 确定性格式化 `.we` 源：同输入总产同输出，格式化器自身的输出是不动点——再跑一次第二次什么也不改。格式化器没有配置选项——第 0 章原则 7 的承诺在此兑付——其规则集是表面样式的唯一来源。规则：缩进两空格，tab 字符被它们替换；行尾 LF、剥除尾随空白；顶层项之间至多一空行、文件恰以一换行结束；每个二元运算符两侧各一空格、每个逗号后一空格、每个冒号后一空格、每个 `->` 两侧各一空格；花括号 `{` 后 `}` 前各一空格，空块除外、它是 `{}`；import 声明按字母序，`std.*` 组在前、第三方模块在后、组间一空行。两个决定刻意不做：格式化器永不折行——在哪断行是作者的判断、无唯一正确答案；它永不对齐字段或注释——对齐依赖名长、把重命名变成噪声。格式化发现项（如 tab 字符、长行）是工具链自有的建议性发现，不是注册表诊断；本章一条也不注册。

#### Scenario: 格式化是确定性的

- **WHEN** 同一文件从同一输入被格式化两次
- **THEN** 两个输出字节相同，且格式化格式化器自身的输出什么也不改

#### Scenario: 规则集可观察

- **WHEN** 一个用 tab、CRLF 结尾、`fn(x,y)`、无序 import 的文件被格式化
- **THEN** 输出用两空格缩进、LF 结尾、`fn(x, y)`、import 组按其固定次序

#### Scenario: 没有配置可加

- **WHEN** 一项提案建议为行宽或样式预设加格式化器选项
- **THEN** 它在第 0 章原则 7 的场景下被拒——格式化器在此的契约是零选项

### Requirement: 建议性诊断与 we vet

`we vet` 运行检查管线、再运行建议层：以 `W` 级诊断报告的启发式发现、单独的检查管线不产它们。建议层的注册表面恰是本章注册的三个警告——`W1910:` unmocked custom effect in a test、`W1911:` possible indirect nested access to one shared value、`W1912:` function may block on a wait——各自的触发在此固定；工具链 MAY 自携更多启发式发现，它们是实现表面、非本规范的：它们不占注册表号码、无规范承诺它们。点名触发：`W1910` 在测试的代码路径调用一个无 mock 的自定义效果标签的函数时发射——调用是真的，第 20 章如是说，发现项建议 mock；`W1911` 在共享值回调体内调用的函数可能再触同一绑定时发射——第 18 章 `E1613` 只看直接调用，此启发式多看一跳，保守、容许误报，是调查、从来不是验证；`W1912` 在函数体直接调用等待会阻塞的操作之一时发射——`Semaphore.acquire`、`Cond.wait`、`Channel.send`、`Channel.receive`、`TaskHandle.await`，第 18 章自己语义里的五个——第 16 章刻意留在效果系统之外的信息，在此以 note 相供。每条建议在清单 `[vet]` 表可配，值为 `"warning"`（默认）、`"error"`（提升：如错误一样止住管线与构建）、`"ignore"`（抑制）；任何其他值 MUST 以 `E1903:` invalid toolchain configuration value 拒绝。`we vet` 不产产物；无未忽略发现项余留时退出 0、否则 1。

#### Scenario: 点名警告各按触发发射

- **WHEN** 一个测试调用未 mock 的自定义效果函数、一个共享值的回调体调用可能触同一绑定的辅助函数、一个函数体调用 `Semaphore.acquire`
- **THEN** `W1910`、`W1911`、`W1912` 三个发现项被报告——各携其注册表条目的标题，各为建议性，无一止住检查管线

#### Scenario: 提升使建议阻断

- **WHEN** 清单写 `W1910 = "error"` 且一个测试的路径调用未 mock 的自定义效果函数
- **THEN** `we build` 如遇错误一样止住、不产产物；`we vet` 退出 1

#### Scenario: 非法配置值被拒绝

- **WHEN** 清单的 `[vet]` 表写 `W1912 = "strict"`
- **THEN** 工具链报 `E1903:` invalid toolchain configuration value，名说其键与合法值

#### Scenario: 实现自有发现不是规范表面

- **WHEN** 一个工具链版本加了三个点名警告之外的启发式发现
- **THEN** 它在该工具链自己的命名下报告，无任何规范文本或注册表条目承诺它——第二个工具链不必产它

### Requirement: we test

`we test` 运行项目的测试。默认集是项目 `tests/` 目录下每个 `*_test.we` 文件、递归；项目他处的测试模块是第 15 章与第 20 章下的普通模块——随项目编译、可导入、不在默认集。一个文件内，test 块按源序依次运行；跨文件，次序与并行性是实现的、不在此规定——第 20 章固定一个测试观察到什么，本章固定整轮运行的形状。测试的结局是 `pass`（运行到自身终点）与 `fail`（止于第 20 章的 test 边界）——一条失败路线，断言与 panic 抵达同一边界；无第三结局，v0.8 的 skip 与其 error/pass 之分随携带它们的语法与第二机制一起消亡。`--filter <pattern>` 把运行限于描述匹配模式的测试；匹配无测试的过滤器跑一轮空运行、退出 0。`we test` 全过退出 0、有败退出 1、编译自身失败退出 2——编译失败从不运行任何测试。

#### Scenario: 默认集是 tests/ 递归

- **WHEN** 一个项目持有 `tests/a_test.we`、`tests/unit/b_test.we` 与 `src/helpers_test.we`，`we test` 运行
- **THEN** 前两个文件的 test 块是整轮运行；第三个作为普通模块编译、什么都不运行

#### Scenario: 文件内次序是源序

- **WHEN** 一个测试模块持有三个 test 块、一轮运行报告其完成
- **THEN** 完成次序匹配源序——文件内序列是一个承诺，为重现而作

#### Scenario: 一条失败路线、两种拼法

- **WHEN** 一个测试的断言为假、另一个直接 panic
- **THEN** 两个结局都是 `fail`——第 20 章的边界是唯一路线，报告可名原因，但结局词汇是 pass 与 fail

#### Scenario: 退出码跟随运行

- **WHEN** 一个项目的测试全过，再令其一败，再向一个测试模块引入语法错
- **THEN** 三轮分别退出 0、1、2——第三轮中没有任何测试运行

#### Scenario: 匹配无物的过滤器是空运行

- **WHEN** `we test --filter "nosuch.*"` 不匹配任何描述
- **THEN** 运行为空、摘要报零、退出码为 0

### Requirement: 探索

`we test --explore` 在第 20 章确定性承诺之上探测：在工具链调度器控制下重跑一个测试，选确定性调度器不会走的交错，`--iterations N` 界定轮数——默认读清单的 `[test].explore-iterations`——偏序缩减默认开、`--no-reduce` 关它。覆盖是采样、本章如是说：没有探索是穷尽的、没有探索自称穷尽，非平凡测试的交错数使穷尽在原理上不可能。两条守护诊断载这份诚实。`E1901:` exploration detected nondeterminism 在同一测试的探索运行产出分歧的可观察交错时发射——虚拟化确定性之外有真东西在动，探针找到了它。`E1902:` unmocked effect executed during exploration 在探索运行执行未 mock 的自定义效果时发射——真调用的行为破坏探针的可重现前提，恰是 `W1910` 在普通运行中所建议的条件，在探索下是错误。探索在每轮探索运行确定且通过时退出 0、任何失败或守护诊断时退出 1、编译失败时退出 2。

#### Scenario: 探索探查另类交错

- **WHEN** 一个创建同刻任务与 channel 的测试在 `--explore --iterations 200` 下运行
- **THEN** 工具链驱使调度器跨确定性调度器不会选的交错，在迭代预算内、以偏序缩减折叠等价运行

#### Scenario: 分歧被抓住

- **WHEN** 同一测试的探索运行产出不同的可观察交错
- **THEN** 报 `E1901:` exploration detected nondeterminism——虚拟钟之外有真东西动了，运行退出 1

#### Scenario: 探索下的未 mock 效果是错误

- **WHEN** 一个被探索测试的路径调用未 mock 的自定义效果函数
- **THEN** 报 `E1902:` unmocked effect executed during exploration——`W1910` 在普通运行中所建议的同一条件在此是错误，因为探针的前提是重现

#### Scenario: 界限是诚实的

- **WHEN** 任何关于探索的文档声称穷尽交错覆盖
- **THEN** 它与本 Requirement 冲突——探索采样，确定性承诺重现，两者都不自称枚举

### Requirement: JSON Lines 协议

每个子命令接受 `--json`、然后每行写一个 JSON 对象——JSON Lines，可流式，第 0 章原则 7 的机器可操作面。diagnostic 事件的字段是 `type`、`severity`、`code`、`message`、`file`、`line`、`column`、以及存在时的 `help`；severity 是 `error`、`warning`、`note`、`help` 之一——四个渲染级；注册表的两级映射到前两级，建议层可渲染于 `note` 或 `help`。测试事件是 `test-result`（携 `file`、`name`、`status`、`duration_ms`）与 `test-summary`（携 `total`、`passed`、`failed`、`duration_ms`）——摘要不载错误计数，因为除了 pass 与 fail 没有别的结局。字段集是一份稳定性承诺：既有字段永不删除、永不改名，新字段可以加，消费者可向前依赖它——与注册表条目 schema 同款的纪律。`--json` 不改退出码：码是人类格式的码，读它们的管线行为不变。

#### Scenario: 诊断逐行流式一对象

- **WHEN** `we check --json` 在带一错一警的项目上运行
- **THEN** 每个诊断是独立一行上的一个 JSON 对象、字段集固定，随运行流逐行可解析

#### Scenario: 测试事件载整轮运行

- **WHEN** `we test --json` 在两个测试通过一个失败的项目上运行
- **THEN** 发出三个 `test-result` 对象与一个 `test-summary`，`total: 3, passed: 2, failed: 1`

#### Scenario: 字段稳定

- **WHEN** 后来的工具链版本向 diagnostic 事件加一个字段
- **THEN** 既有字段保其名与义——按此处字段集构建的消费者继续工作，删除或改名字段被禁止

#### Scenario: 退出码不理会格式

- **WHEN** 同一失败的检查带与不带 `--json` 运行
- **THEN** 两轮以相同码退出——协议改渲染、从不改判定

### Requirement: 项目清单

项目清单是项目根处名为 `we.toml` 的 TOML 文件——持有它的目录是项目根，第 15 章事实的复述。本章固定的骨架：`name`，第 1 章命名约定下的字符串——非法值 MUST 以 `E1904:` invalid project name 拒绝；`version`，第 22 章形状下的语义版本——非法值 MUST 以 `E2004:` invalid version value 拒绝；`type`，`executable` 或 `library` 之一——任何其他值是 `E1903:` invalid toolchain configuration value；建议层要求所定的 `[vet]` 表；以及 `[test]` 表，其键 `explore-iterations` 是正整数——非正或非整数值是 `E1903`。项目调用——目录路径命令——无清单、或清单缺 `name`、`version`、`type`，MUST 以 `E1905:` project manifest missing or incomplete 拒绝。清单的 `[dependencies]` 表与锁文件是第 22 章的：该表可选——缺省即空依赖集——其键与值在那章受检，`we.lock` 由其解析机器书写。

#### Scenario: 骨架被检查

- **WHEN** `we build` 在 `we.toml` 缺 `type` 的目录中运行
- **THEN** 工具链报 `E1905:` project manifest missing or incomplete，名说所缺的键

#### Scenario: 非法值被点名

- **WHEN** 一份清单写 `type = "bin"` 或 `explore-iterations = 0`
- **THEN** 两者都以 `E1903:` invalid toolchain configuration value 拒绝，报文名说其键、合法值与所见值

#### Scenario: we new 写骨架

- **WHEN** `we new demo` 运行
- **THEN** 所建的 `we.toml` 携 `name = "demo"`、一个 `version`、`type = "executable"`、无工具链未定义的表；`src/main.we` 持一个 `pub fn main`、`tests/` 持一个空测试模块

#### Scenario: 依赖是第 22 章的

- **WHEN** 一份清单携带 `[dependencies]` 表
- **THEN** 其键与值循第 22 章受检——非法名或保留的 `std` 是 `E2006`、坏约束是 `E2003`——且在该章要求下、命令所跑的任何管线之前先解析与获取

### Requirement: 文档生成

`we doc` 把第 6 章附件规则的 `///` 文档单元渲染为 API 文档。被文档化的表面是 `pub` 声明、再无其他——非 pub 项不出现在任何页面，第 15 章的可见性纪律被带进渲染输出。默认输出目录是项目的 `docs/` 目录，`--output` 重定向它，`--check` 验证而不生成：它报告未携文档单元的 pub 声明，作为工具链自有的建议性发现——完备性策略是项目的、非规范的，本章不为此注册任何码。文档内容中的交叉引用是工具的渲染事务；一条不可解析的是工具的建议性发现、同样不入册。

#### Scenario: 表面仅 pub

- **WHEN** 一个模块持有带文档的 pub 与非 pub 声明、`we doc` 运行
- **THEN** pub 声明的页面存在、非 pub 声明在输出中无处出现

#### Scenario: check 验证而不生成

- **WHEN** `we doc --check` 先在每个 pub 声明都携 `///` 单元的项目上运行、再在有一个无文档 pub fn 的项目上运行
- **THEN** 第一次什么都不报、什么都不写；第二次把缺口作为工具链自有的建议性发现报告、仍什么都不写

### Requirement: foreign 声明的链接

foreign 块中声明的名绑定平台 C ABI 的恰一个原生符号——第 19 章定义声明与边界的检查；本章固定绑定的可观察契约。绑定发生在构建时：`we check` 不链接，链接问题在其中不发生。构建时，绑定不到任何可发现符号的 foreign 名 MUST 以 `E1906:` unresolved native symbol 报告，报文名说该声明与所寻符号。声明与符号之间的一切是工具链的机制、在此无处固定：名字映射或 mangling 方案、库搜索路径、库格式、平台 C ABI 之外的调用约定变体——都是构建工具自己的——只通过其失败可观察，失败携上面这一个码。

#### Scenario: 未绑定符号败掉构建

- **WHEN** 一个 foreign 块声明 `fn abs(x: Int64) -> Int64 effect` 而构建发现无原生符号应答该绑定
- **THEN** 构建报 `E1906:` unresolved native symbol 名说该声明，不产产物

#### Scenario: check 从不链接

- **WHEN** 同一项目在 `we check` 下运行
- **THEN** 第 19 章该声明的检查照己运行、照己过败；无链接问题发生、`E1906` 不可能发射

#### Scenario: 机制是工具链的

- **WHEN** 两个工具链以不同的 mangling 方案与搜索路径绑定同一声明
- **THEN** 两者都合规——规范固定声明的检查与绑定的失败码、不固定两者之间的任何东西

### Requirement: 工具链不固定什么

本章指名它所留白的东西。语言服务器协议住在它自己的独立文档里，如 v0.8 本已意向的那样——这里作的唯一承诺是一致性：编辑器服务的诊断是 `we check` 的，同一管线报同样的码，任何编辑器表面不得偏离命令行的判定。诊断输出的本地化是工具链的——注册表条目是英文、翻译层在规范之外。增量编译、缓存与构建并行性是实现细节、藏在同输入同输出这唯一承诺后面。跨文件测试并行性与报告布局同样是实现的。任何性能预算——构建时长、延迟、足迹——在本章任何地方都不被承诺。

#### Scenario: 编辑器诊断是 check 的诊断

- **WHEN** 一个编辑器服务为一个文件报一条诊断
- **THEN** 码、消息与判定是 `we check` 对同一源所报的——一条管线、一个真相，无论它显示哪张脸

#### Scenario: 没有性能承诺存在

- **WHEN** 一个工具链被拿本章衡量构建速度或延迟预算
- **THEN** 没有东西应答——本章承诺输出确定性与可观察契约，性能是评测层要测的、不是规范层要承诺的

### Requirement: 工具链诊断段位

工具链章拥有注册表段位 `E1900`–`E1999`，声明于 `docs/spec/diagnostics.toml` 的 `[segments]` 下，未认领区间收窄为 `E2000`–`E9999`。`E` 与 `W` 共享号码空间——第 99 章规则、一码一严重级——故段内持有错误 `E1901` exploration detected nondeterminism、`E1902` unmocked effect executed during exploration、`E1903` invalid toolchain configuration value、`E1904` invalid project name、`E1905` project manifest missing or incomplete、`E1906` unresolved native symbol、`E1907` command path not found，与警告 `W1910` unmocked custom effect in a test、`W1911` possible indirect nested access to one shared value、`W1912` function may block on a wait——本章承诺的三个建议项，v0.8 的 `W0601`、`W0755`、`W0756` 重编号入共享空间。段内未分配号码——`1900`、`1908`–`1909`、`1913`–`1999`——为本章修订保留，循第 99 章共号空间规则无字母书写，因号码 `1910`–`1912` 是警告们自己的。触发语义在本章各 Requirement；条目在注册表。

#### Scenario: 一个工具链码被发射

- **WHEN** 工具链发射任何 `E19xx` 或 `W19xx` 诊断
- **THEN** 其完整条目可在 `docs/spec/diagnostics.toml` owner `2100-toolchain` 下检索

#### Scenario: 后来的变更需要本段位码

- **WHEN** 本章未来的修订需要一个新诊断
- **THEN** 它在同一变更内扩展 `E1900`–`E1999` 内的注册表、或认领它自己的段位

## 示例（非权威）

下列示例只用第 1–21 章已批准的表面形式。它们是说明性的、非权威的：任何冲突处以 Requirement 与 Scenario 为准。带诊断码注解的行是被拒绝的形式，展示工具链发射的码。

### 一个项目与它的清单

```toml
# file: we.toml — the skeleton chapter 21 fixes; [dependencies] is
# chapter 22's table — optional, absent means an empty dependency set
name = "demo"
version = "0.1.0"
type = "executable"

[vet]
W1910 = "warning"        # the default, spelled out; "error" promotes, "ignore" suppresses

[test]
explore-iterations = 200
```

```toml
# type = "bin"                           // E1903: invalid toolchain configuration value
# explore-iterations = 0                 // E1903: invalid toolchain configuration value
# (we build run in a directory with no we.toml)
#                                        // E1905: project manifest missing or incomplete
```

### 命令面

```sh
we new demo                # skeleton: we.toml, src/main.we, tests/main_test.we
we check .                 # full check pipeline, no artifact, no link
we check tool.we           # single-file compilation: std.* only, no manifest needed
we build                   # artifact named by the manifest; links foreign declarations
we test                    # every *_test.we under tests/, recursively
we test --filter "relay.*" # descriptions matching the pattern; empty match exits 0
we test --explore --iterations 200
we version                 # shape fixed, numbers never in the spec's text
```

```sh
# we build nosuchdir                      # E1907: command path not found
# we new My_Project                       # E1904: invalid project name
```

### 管线的可观察次序

```sh
# a file holding both a parse error and a type error reports the parse error —
# the stages run in order and the first failing stage is the run's verdict:
#
# we check src/one.we
# error[E0102]: ... parse error reported, type error not reported in this run
```

### 格式化器的规则

```we
// before we fmt (tabs, CRLF stripped here, unordered imports, tight commas):
// import zeta
// import std.io
// import alpha
// fn add(x: Int64,y: Int64)->Int64 { return x + y }
```

```we
// after we fmt:
import std.io

import alpha
import zeta

fn add(x: Int64, y: Int64) -> Int64 { x + y }
```

### we test 的运行形状

```we
// file: tests/order_test.we — three blocks run in source order
test "first" { assert(true, "first") }
test "second" { std.test.assertEqual(1 + 1, 2) }
test "third" { panic("arrived at the boundary") }   // this test fails, the run continues
```

```sh
# we test
# ... three test-result events in source order; exit 1 — one failure, no error outcome
```

### 探索与它的守护

```we
// file: tests/probe_test.we
test "probe tied wakes" {
    let ch = channel(0)
    scope {
        task effect time { let _ = sleep(10); ch.send(1) }
        task effect time { let _ = sleep(10); ch.send(2) }
    }
    advanceTime(10)
}
```

```sh
we test --explore --iterations 200
# E1901 fires when explored runs diverge; E1902 when an explored run executes an
# unmocked custom effect — the condition W1910 advises on in an ordinary run
```

### JSON Lines 协议

```json
{"type":"diagnostic","severity":"error","code":"E1907","message":"command path not found","file":"","line":0,"column":0}
{"type":"test-result","file":"tests/order_test.we","name":"third","status":"fail","duration_ms":3}
{"type":"test-summary","total":3,"passed":2,"failed":1,"duration_ms":11}
```

### 链接

```we
// file: src/math.we — declaration per chapter 19; binding per this chapter
foreign "c" {
    fn abs(x: Int64) -> Int64 effect
}
```

```sh
# we check    — no link, E1906 cannot fire
# we build    — E1906: unresolved native symbol, if no symbol answers the binding
```

### 建议性发现

```we
// W1910: unmocked custom effect in a test — a test path calls an unmocked
//        custom-effect function; real call, advised mock
// W1911: possible indirect nested access to one shared value — a helper called
//        inside a shared value's callback may touch the same binding; the
//        survey E1613 cannot make, conservative by design
// W1912: function may block on a wait — a function body calls Semaphore.acquire;
//        a may-block note, informational
```

### 待后续变更

```toml
# Dependencies resolve under chapter 22; the registry and publish story is
# that chapter's named gap. The LSP protocol lives in a separate document;
# its one spec promise is here: editor diagnostics are we check's.
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| subcommand | 子命令 |
| project root | 项目根 |
| single-file compilation | 单文件编译 |
| check pipeline | 检查管线 |
| advisory finding | 建议性发现 |
| promotion | 提升 |
| exploration | 探索 |
| iteration budget | 迭代预算 |
| partial-order reduction | 偏序缩减 |
| guard diagnostic | 守护诊断 |
| JSON Lines | JSON Lines |
| field stability | 字段稳定性 |
| manifest | 项目清单 |
| named gap | 指名留白 |
| linkage | 链接 |
| blocking wait | 阻塞等待 |
| deterministic scheduling | 确定性调度 |
| virtual clock | 虚拟钟 |
