# design — fmt-vet-doc（M11）

规范依据：ch21 R3（:52-69 格式化器）、R4（:71-93 advisory 与 vet）、R9（:195-207 文档生成）、R2（:28-50 管线可观察契约）、R7（:148-170 JSON Lines）、R8（:172-194 manifest）；注册条 W1910/W1911/W1912（diagnostics.toml:1496/:1505/:1514）、E1903（:1451）。四裁决见 proposal。本设计零规范增量。

## D1 fmt 架构 = 行保结构的 token 级重排器

ch2 落地裁决的派生不变量（换行承义、分号推断）使 fmt **永不移动 token 跨行**——唯一的规范钦定例外是 import 声明重排（R3 明文「import declarations are ordered alphabetically」——排序必含移动，整声明为单位移动）。

管线（每文件）：读原文 → `lex.Keep`（token + 注释 trivia，见 D2）→ `parser.Parse`（失败检测门，Q1）→ 重排（D3/D4）→ 差异才写回。

**注释保真**：`internal/lex` 增第二入口 `Keep(name, src) ([]Token, *diag.Diagnostic)`——与 `Scan` 同一扫描器、注释不跳过而作 trivia token 产出（Kind = `comment`，Text 原文含 `//`）。单一词法权威；parser 面零改（仍走 `Scan`）。`///` 文档单元在 Keep 面同为 comment token（fmt 原样保真；附着语义归 parser 面）。

**歧义邻接的 AST 分类**：token 邻接间距表（D3）对两处歧义无局部解——`|`（or-模式的二元管 vs 闭包参数定界管）与一元/二元 `-`（后者 token 局部可判：前邻 ident/字面量/`)`/`]` 即二元）。`|` 的判据 = AST：parse 已是必跑步骤，走查 AST 收集闭包参数位 `|` 的 token 坐标集（ast.Closure 节点参数表区间），命集中 = 定界管（紧邻），否则二元（双侧空格）。`=>` 恒双侧空格（match 臂箭头，机制补全——规范只点名 `->`，臂形 `Circle(r) => r` 的间距自明）。

## D2 fmt 的运行载体与门

- **路径约定一致**：目录 = 项目（须 manifest，E1905 与 check/build/run/test 同门同报文）；`.we` 文件 = 单文件面（无需 manifest）。fmt 不跑 toolchainGate（不产原生工件）、不跑模块解析/类型检查（R3 的规则面是纯排版；类型坏不影响排版，解析坏才挡）。
- **文件集**（Q2）：项目面 WalkDir 全部 `*.we`，排除 `build/` 子树；单文件面恰该文件。
- **失败面**（Q1）：lex 或 parse 失败 = 该文件不动、诊断双面报告（E0xxx/E01xx 原码），整体 exit 1；其余文件照常。
- **写回**：输出与输入逐字节同则不写（mtime 静默）；不同则写回。成功静默（check 先例）；`--verbose` 列被改写的文件——stdout 行 `we: formatted {path}`（build 的 `we: built` 先例形）。
- **退出码**：0（全格式化或本已净）/ 1（任一文件失败或 fs 错）/ 2（usage——fmt 无专有旗标，仅全局）。诊断走协议双面（--json 下 E 事件照常 stdout）。

## D3 fmt 规则落点（R3 规则集的机制化）

**行内与行尾**：
1. CRLF→LF；行尾空白剥除（剥空后的空行 = 空白行）；BOM 剥除（M1 规范化先例）。
2. 缩进 = 行首起 2 × 深度；行首 tab 先按 2 空格展开（「a tab character is replaced by them」）再按深度重排（展开是为规则自洽的中间态，最终缩进恒由深度定）。
3. **深度模型**：未闭 `{`/`(` 的累计计数（token 流计数——字符串/注释内的括号不进账，Keep 面天然正确）；`}`/`)` 开头的行先闭后计（闭行自身缩进按闭合后深度）。`#[...]` 属性单元单行自闭合、净零。

**邻接间距表**（前邻类 × 本 token 类的封闭判定；规范点名的规则加粗。判定优先级：括号零规则 > 管一空格规则 > 其余）：
- **二元运算符**（算术/比较/逻辑/位移/位/`=` 赋值/`..` 区间/or 管）：前侧一空格、后侧一空格。
- 一元 `!`/`-`/`~`：与前邻一空格（若有 token）、后侧零。
- **逗号**：前零后一。**冒号**（`name: type` 位）：前零后一。**`->`**：双侧一空格；`=>`：双侧一空格（机制补全）。
- **花括号**：`{` 后侧一空格（同行有内容时；行尾则零——尾空白剥除兜底）、`}` 前侧一空格（同行有前邻 token 时——`{ x }` 形；行首独占由缩进定）、空块 `{}` 零空格。
- **圆括号/方括号通则**：开括号 `(`/`[` 前侧恒零附加（前 token 自身的尾随空格已足——ident 后调用形、`(` 嵌套、逗号/运算符后由彼等自身规则定间隙）；闭括号 `)`/`]` 前侧恒零（聚簇紧贴），后侧由后 token 规则定。
- `.` 成员访问：双侧零。`?` 传播后缀：前零后零（后缀链内 `)?.name`）；链外由后 token 规则定。
- 闭包定界 `|`（D1 AST 判据）：内侧零、外侧一空格（`|acc, r| acc + r`）；参数表内逗号照逗号规则。括号邻接处括号零规则优先（`update(|v| v + 1)`）。
- 词 token（关键字/ident/字面量）之间恒一空格；注释 token：前侧一空格（若同行有 token）、原文保真至行尾。
- 字符串/符文字面量内容逐字保真（不重排不逃逸改写）。
- （`#[` 属性在当前语言零批准面——「no attribute target is ratified yet」，不设行；若语言后补属性面再扩表。）

**空行策略**：空行串（≥2 连续）坍缩为一；文件首无空行、文件尾恰一换行；import 头之后一空行接首个非 import 项（无 import 则无此空行）。块内空行同规坍缩为一（规范只钉顶层项间距，块内取同形——机制补全，定点性自洽）。空文件（零字节）保持空——「文件尾恰一换行」作用于有内容的文件，零内容不注换行（边界定形，黄金钉死）。

**import 重排**：全部 import 声明收集提升至文件头，两组三段：std.* 组（按模块路径字典序）→ 一空行 → 其余组（字典序）→ 一空行 → 其余项。**注释随行**：紧邻某 import 声明上方、无空行相隔的注释行块随该声明移动（附着直觉与 `///` 的 next-item 规则一致）；有空行相隔的注释留在原位。别名字形（`import x as y`）按路径参与排序。

## D4 advisory 收集机（三触发）

载体：`internal/typecheck` 新文件 advisory.go——挂在既有 checker 的体走查点位（绑定类型、调用目标效果集、mock 目标身份皆已算好的地方），不做二次推断（两处权威之忌）。收集恒发生（廉价），返回面由调用方定。

**W1910 unmocked custom effect in a test**：test 体子树（含嵌套闭包/task 体/mock 体——mock 体内调用在被调时同样真实）内调用某 fn、其声明效果集含至少一个非内建标签（内建 = io/net/time 裸键，与 M10c fxgate 的 isCustomEffect 同判据——一处判据两处消费），且同 TestDecl 内无该目标的 MockDecl（身份 = E1805 的 canonical 模块键+名机器）。块内任意位置的 mock 覆盖全块调用（M10b 装填于块始的运行事实，「no mock on that path」的块粒度读法——设计定形披露）。锚 = 调用点被调名 token。消息 `unmocked custom effect in a test — {fn} carries custom effect {tag}, no mock in this test`。

**W1911 possible indirect nested access to one shared value**：对每个同步 gc 型绑定（Mutex/RwLock/Atomic/AtomicRef——E1613 的绑定面），其回调实参闭包子树内（含嵌套闭包穿透）出现以**该绑定自身**为实参（含接收者位）的调用 → 锚 = 该调用名 token。恰一次调用穿透（章文自钉「looks one call through」）；直访（`m.update` 于 `m` 回调内）归 E1613 已治，不重复发。消息 `possible indirect nested access to one shared value — {binding} is passed to {callee} inside its own callback`。

**W1912 function may block on a wait**：函数形体（fn 体/方法体/闭包体/task 体/test 体/mock 体——ch12 语境清单全量）内**直接**调用五操作之一，类型导向：接收者命名类型恰为对应原语——`Semaphore.acquire`、`Cond.wait`、`Channel.send`、`Channel.receive`、`TaskHandle.await`（trySend/tryReceive 非阻塞、不入面）。锚 = 方法名 token。消息 `function may block on a wait — {Type}.{method} may block`。

三码 helps = 注册表 remediation 逐字（三条 remediation 本已是祈使短句，逐字取用零转写漂移——单一权威；M1 模式的实义）。

## D5 [vet] 表、提升与接线

**manifest 面**：parseManifest 复用 M10c section 前缀（键形 `vet.W1910`）；loadManifest 于 `[test]` 门后增 `[vet]` 门——三键 W1910/W1911/W1912 各验值 ∈ {"warning","error","ignore"}，非法值 E1903：`invalid toolchain configuration value — vet.{code} must be "warning", "error", or "ignore", got {v}`（裸值无引号、At we.toml 1:1、WithHelp 同 M10c 形）。表内其余键不校验（与 `[test]` 同姿势——M10c posture 注释的兑现与退场）。五面（check/build/run/test/vet）共享 loadManifest 直证。

**提升语义**：advisory 发现为 severity warning 的 diag.Diagnostic；CLI 按 `[vet]` 映射——`warning`：如实报（stderr 人类行 `warning[W1910]: …` / --json stdout 事件 severity:"warning"）、不停；`error`：severity 渲染为 error（码保持 W19xx——ch99 一号一严重度是注册表侧恒真，severity 是渲染四层、提升即改渲染层并停管线）；`ignore`：弃置不报。默认（键缺省/表缺省）= warning。

**管线接线**（Q4）：check/build/run/test/vet 在 typecheck 干净后跑 advisory 段。提升停的行为 = E 停的同款：check exit 1、build 无工件 exit 1、run 不执行 exit 1、test 映射编译败 exit 2（「exactly as an error does」）。advisory 段在文档检查语义位之后、codegen 之前（R2 七段之后的工具层——vet 场景「runs the check pipeline and then the advisory layer」的序）。

**we vet 面**：路径/manifest/管线与 check 同形（无 toolchainGate、无工件）；输出 = E 诊断（若停）否则未忽略 W 发现集；退出 0（净）/ 1（E 停或有未忽略发现）——E 停归 1 是机制定形（spec 只钉 0/1 的发现轴；披露）。

## D6 we doc 面

- **管线先行**：check 管线全量（含 advisory 段——doc 也是管线命令，Q4 一致）——E 停则无页面 exit 1；净则渲染。
- **文档集**：项目面 = 根图模块（src/main.we 导入图，滤 std.*——与 build 程序面同滤）；单文件面 = 该文件。pub 声明全集：`pub fn` / `pub let` / `pub record` / `pub newtype` / `pub type`（sum）/ `pub interface`。impl 块非声明、不入面（披露——方法经类型页签名之外无独立文档位，机制从简）。
- **渲染**（Q3）：每模块 `docs/{module-key}.md`（平坦点分——镜像模块键、零目录创建、零碰撞）。页形：H1 `Module {key}`；每 pub 项源序：H3 项名 + 围栏代码块内的规范签名（fn 形 `fn name(params) effect tags -> Ret`——ch6 序，effect 段在返回类型前）+ `///` 单元内容原文（围栏外）。`--output DIR` 重定向根；缺省根 = 项目面 `docs/`、单文件面 `{file} 所在目录下的 docs/`（披露）。
- **--check**：零生成；未文档化 pub 声明报工具自有建议行——stderr 平文 `we: undocumented pub declaration {name} at {file}:{line}`，**不入册不受 --json 影响**（注册外发现的规范立场；R7 事件词表不扩——披露）；退出 0（全文档化）/ 1（有缺口）。E 停先于 --check 面。
- **旗标**：`--output`（取值形，同 `--iterations` 先例：`--output DIR` 与 `--output=DIR`，缺值 usage 2）、`--check`（裸 bool）；皆 doc 专属（check 面未知选项照旧 unknown option）。成功生成静默；`--verbose` 列页面路径——stdout 行 `we: wrote {path}`（同 build/fmt 先例形）。

## D7 conformance 黄金矩阵（T1 落盘定数，预计 ~35）

| 组 | 枚 | 面 |
|---|---|---|
| fmt 基础 | 4 | pass 综合脏→净（脏源含超长行 + 手工对齐字段——净输出逐字节钉**不折行不对齐**的负向证明）/ tabs+CRLF / 间距（逗号冒号箭头花括号）/ already-clean 零改写 |
| fmt 结构 | 5 | 空行坍缩 / 空块 `{}` / import 分组排序 / import 交错提升+注释随行 / 闭包管与 or 管的歧义间距 |
| fmt 边界 | 6 | 解析失败文件不动+exit 1 / 单文件面 / 项目面 E1905 / --verbose 列改写 / **坏旗标 usage 2**（`--width` → unknown option——零选项非目标的负向钉）/ 空文件保持空（零字节零注换行） |
| fmt 定点 | 1 | fmt(fmt(x)) == fmt(x)（对偶传递证明：pass 枚钉 fmt(dirty)=clean，本枚 setup=clean 断言零改写，传递即定点；零 runner 改动。真双跑归 T7 真机探针） |
| vet 触发 | 6 | w1910 发 / w1910 mocked 静默 / w1911 绑定穿参 / w1912 五操作（1 黄金多调用位逐行）/ 触发位置与消息逐字 / helps 面 |
| vet 配置 | 5 | [vet]=error 提升停 build / 提升下 test exit 2 / ignore 抑制 / E1903 非法值 / check 面共享 E1903 |
| vet 运行 | 3 | 净项目 exit 0 / W 在 check 输出不停管线 / --json W 事件 schema |
| doc | 5 | 生成页形（签名序+文档单元）/ pub-only / --check 缺口行+exit 1 / --check 净 / --output 重定向 |
| **计** | **35** | 另：48 枚既有随批更新（Q4a，T6 专任务） |

## D8 验证阶梯（T8 固定序）

① `go build ./...` ② `go vet ./...` ③ `go test ./...` ④ runtime C harness（零改——确认性重跑）⑤ conformance 全量 fresh（606 + 35 新 = 641；48 枚在位更新不增减总数，对齐 proposal 目标 8）⑥ `validate.py --all --strict` ⑦ `docs_sync.py` ⑧ `gofmt -l`（M10c T10 教训——纳入固定阶梯）⑨ git status 对账（refr/ 零触碰、既有黄金改动恰 48 枚清单）。

## D9 实现顺序与红绿账

T1 黄金先落先红（红因 = 三子命令 exit 70「not implemented」；**唯一绿哨兵 = fmt 坏旗标枚**——选项解析先于子命令分发，usage 2 今日即达且 T3 后恒绿，负向钉本职）→ T2 单测先红（lex.Keep/fmt 重排/advisory 收集/parseOptions 符号未定义）→ T3 fmt 塔（lex.Keep + internal/fmt + runFmt）→ fmt 组翻绿 → T4 advisory 塔（diag.Warning + advisory.go + [vet] 门 + 全管线接线 + runVet）→ vet 组翻绿 + **48 枚既有转红（预期红，T6 处置）** → T5 doc 塔（runDoc + 渲染）→ doc 组翻绿 → T6 随批更新（WE_UPDATE_GOLDEN 重生成 + 逐枚 diff 过目 + W 行与触发源对账披露）→ T7 黑盒电池 → T8 终验 → T9 审查 → T10 归档 → T11 记忆与提交。
