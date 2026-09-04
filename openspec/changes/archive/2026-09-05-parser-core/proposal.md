# parser-core — 第 2 章语法骨架与第 6 章声明：`we check` 跑到解析

## Why

M1（lexical，归档 2026-09-05）交付了词法阶段：`we check <file>.we` 对首个词法错误报 E0001–E0009 并 exit 1，词法干净则停在 `we: parsing is not implemented in this reference build yet` 边界（exit 70）。按 roadmap M2 行（薄垂直优先），下一管线阶段是解析（第 21 章 R2 次序 lexical → parsing → …），其规范依据已全部批准：

- 第 2 章（`docs/spec/0200-grammar.md`）全部 Requirement：Parse model and ambiguity rejection / Line-joining and semicolon inference / Blocks and block value / Statements / Expression skeleton / Operator precedence and associativity / Grammar diagnostics segment；
- 第 6 章（`docs/spec/0600-declarations.md`）全部 Requirement：File structure and module identity / Import declarations / Function declarations / Function bodies, return and defer / Top-level bindings / Visibility with pub / Module name space / Documentation comments；
- 第 7 章 R「Type references」——`name: type` 与 `-> type` 注解槽的文法属该章（范围按裁决 Q3 落定，见下）；
- 注册表 `docs/spec/diagnostics.toml`：E0101–E0105（语法段）、E0012/E0013（ch1 命名块，M1 显式缓期至解析期）、E0401–E0405（ch6 段，范围按裁决 Q2 落定）。

可验证的黑盒缺口：今天没有任何输入能产生 E0101–E0105 或任何声明类诊断——词法干净的文件一律 exit 70 一行边界消息，与文法合法性无关。

## 裁决记录（candidate 阶段三项表面裁决，2026-09-05）

- **Q1 已批准未实现形式的边界风格 → 逐章 70 边界行**：每个形式组一行 stderr + exit 70（如 `we: chapter 3 (control flow) forms are not implemented in this reference build yet`），映射表随后续里程碑收缩到零；E0105 语义保持「不合任何已批准产生式」。落 design D6/D11。
- **Q2 ch6 解析期诊断码范围 → 四组全落地**：return 形式 + E0401/E0402、命名 E0012/E0013、文档注释附着 + E0405、名字空间 E0404。落 design D4/D8/D9/D10。
- **Q3 类型注解槽文法范围 → 完整 ch7 类型引用**：命名/限定/泛型应用/元组/单元/Dyn/fn 类型全量（纯文法、零类型检查），槽内非法文法按 ch7 场景报 E0105。落 design D7/D12。

## 目标与非目标

### 目标

1. `internal/ast` + `internal/parser`：消费 ch1 token 流（含 attr token 的 AttrName/AttrArgs 消费面）产出 AST；第 2 章机制全量——行接续（括号内换行无意义；深度零续行集 = 二元运算符 + `=` + `.`；语句起始类固定；E0102）、块即表达式与块值、语句族（绑定/赋值/表达式语句；赋值不是表达式 E0103）、表达式骨架（初等/后缀链/一元前缀）、12 级封闭优先表（9 级比较与 12 级 `..` 非结合，E0104）。
2. 第 6 章声明：文件即模块；顶层项 import / fn / 顶层 let（含 pub 前缀）；fn 签名（`name: type` 参数、可选 `-> type`、注解槽按 Q3）；函数体与 return 形式；顶层绑定（顶层 var → E0403）；模块名字空间；`///` 文档单元附着。解析期诊断码范围按 Q2（E0103/E0105 之外至少含 E0102/E0104；E0105 消息按注册表义务点名「该位置考虑过的产生式」）。
3. E0101 无 M2 触发输入的论证记录于 design（确定性递归下降下每个接受序列恰一棵树；该码为第 0 章原则 4 的阀门，注册表条目照引）。
4. `we check` 单文件模式跑词法+解析：首个诊断（词法或语法）→ 报告 + exit 1；干净解析 → 类型检查边界一行 + exit 70（阶段级诚实边界从「解析」收缩为「类型检查」）；已批准未实现形式按 Q1。
5. conformance：新增 21 个黄金用例（每码至少一例、ch2/ch6 场景原例优先）并改写 check-clean / check-clean-json（骨架内容 `Ok(())` 命中第 8 章形式，期望随 Q1 落定）；internal/parser 按章单测（含 41 关键字派发表穷举测试）。

### 非目标

- **类型检查与 ch7 其余**（字面量类型化、无条件转换、条件位置、块值类型）→ M3。
- **后续章节形式的文法**（控制流/match/迭代/复合/和类型/接口泛型/闭包/资源/错误 `?`/集合/并发/ffi/测试）→ M5+；本变更只按 Q1 处理其边界，不实现其产生式。
- **项目模式**（目录 → 项目编译边界一行 70 不变；manifest 校验）→ M3。
- **E0011**（类型名声明处 PascalCase）→ M3 起有类型声明形式时；本变更解析类型引用但无类型声明。
- **名字解析**（E1303/E1304、std 保留段、跨模块可见性）→ M6（ch15）。
- **注册表 go:embed**：维持调用点硬编码标题/help 模式（roadmap 待办 2）。
- **多错误批量报告**：解析沿用首错误停止（M1 design D2 同构）。

## What Changes

- 新增 `internal/ast/`（AST 节点）、`internal/parser/`（解析器 + 按章单测）。
- `internal/lex`：`///` 文档单元收集的旁路输出（注释仍不产 token；附着规则属 ch6，M1 非目标在此兑现）——`Scan` 入口返回 token 流 + 文档单元。
- `internal/cli`：runCheck 在词法后接解析；「parsing 未实现」边界行删除，尾部边界换为「type checking 未实现」；新增按 Q1 的形式边界行分派。
- `internal/conformance`：新增与改写黄金用例。
- docs：roadmap M2 状态行由归档本变更的动作翻写为 done。

## 影响层

`compiler`（解析阶段——已批准章节的实现，无规范增量）、`tooling`（`we check` 可观察表面：边界行、退出码——第 21 章既有行为的部分实现，无规范增量）。**本变更不改变语言行为、无规范增量**：一切接受/拒绝判定以已批准的 `docs/spec/0200-grammar.md`、`docs/spec/0600-declarations.md`（及 Q3 范围内的 `0700-types.md` 文法条款）与注册表为依据；机制自由度（AST 形状、行接续实现方式、消息措辞与位置约定、关键字派发表）在 design.md 固定并作为事实上的稳定表面记录。

## 影响范围

- 新文件：`internal/ast/ast.go`、`internal/parser/parser.go`、`internal/parser/parser_test.go`、约 21 个 `internal/conformance/testdata/cases/check-*.json`。
- 修改：`internal/lex/lex.go`（`Scan` 文档单元旁路；`File` 保持既有签名语义）、`internal/lex/lex_test.go`（文档单元单测）、`internal/cli/check.go`（解析接线）、`internal/conformance/testdata/cases/check-clean.json`、`check-clean-json.json`（改写）、`docs/roadmap/0000-reference-implementation.md` 与 `.zh.md`（M2 行）。
- 不动：`docs/spec/`、`diagnostics.toml`、go.mod、M1 其余 33 个黄金用例。

## 审计记录

**2026-09-05，candidate → ready，welang-spec-impact-audit 七条，通过。**

1. 问题真实性：✅ 黑盒缺口可验证——词法干净文件一律 exit 70 一行边界消息，无任何输入能产生 E0101–E0105 或声明类诊断；锚点 `docs/spec/0200-grammar.md`（全部 Requirement）、`docs/spec/0600-declarations.md`（全部 Requirement）、`docs/spec/0700-types.md` R「Type references」、`docs/spec/diagnostics.toml`（E0101–E0105、E0012/E0013、E0401–E0405 条目）已在 Why 引用。
2. 影响层声明：✅ change.yaml `layers: [compiler, tooling]` 与 proposal「影响层」一致；proposal 含「不改变语言行为、无规范增量」边界声明（validate 内部变更豁免路径）。
3. 规范增量范围：✅ 零增量、零新码、零改码；E0101–E0105、E0012/E0013、E0401–E0405 均为注册表既有条目、按标题与 remediation 逐字引用；无 active 变更冲突（changes/ 下仅本变更）。
4. 原则一致性：✅ 全部接受/拒绝判定有 ch2/ch6/ch7 文法条款依据；机制自由度（AST 形状、行接续实现、消息措辞与位置、关键字派发表、E0105 产生式清单、尾随逗号统一、E0405 附着直读）记录于 design D1–D15，不触十条原则。
5. 参考基线固定：✅ 引用均为 docs/spec 明确路径；无 refr/ 草案依赖。
6. 验收边界：✅ 目标可机械判定（黄金用例红→绿、退出码、validate/docs_sync）；非目标显式排除类型检查、后续章形式实现、项目模式、E0011、名字解析、embed、批量报告。
7. 粒度：✅ 单一垂直单元——解析阶段经 `we check <file>` 端到端可观察；ch2+ch6 为一个不可拆的文法整体（ch6 项的体即 ch2 块），无跨三层耦合改动。

## 审查记录

**2026-09-05，ready → active，welang-change-review 十条，通过（发现 F1，非阻塞）。**

1. proposal：✅ 黑盒缺口与目标；实现语境仅限 internal 包名（M0/M1 审查先例一致）；裁决记录三问三答自含。
2. spec 增量：无（纯内部变更豁免路径），N/A。
3. design：✅ 唯一最小路径；D15 八条被拒方案含理由；引用精确到 Requirement、场景原例与注册条目；与零 spec 增量声明无矛盾。
4. tasks：✅ T1–T10 每项有来源（proposal 目标 / design D 条）与验证（具体测试命令）；无 deferred / non-goal 混入。
5. 场景覆盖：✅ normal（check-parse-clean 全形模块 + --json）、boundary（行接续两判定点、`=` 三分规则、单元/元组/构造/列表/`?` 五类后续章形式边界行、尾随逗号）、failure（E0102×1、E0103×2、E0104×2、E0105×6 含注册表原例与 --json、E0401–E0405 各一、E0012/E0013）。
6. 无空章节：✅（proposal 预留的「实现审查记录」为实现期占位，M1 同构）。
7. 测试先行：✅ T2 黄金用例先红、T3 单测骨架先红（关键字派发表穷举 + 行接续表），T4–T7 后实现。
8. 负向断言：✅ 每个落地的拒绝码均有违规样例黄金用例（D14 表 19 例中 13 例为负向）；「赋值非表达式」「链式非结合」「索引永不解析」均有 ch2/注册表原例断言。
9. 完成度闭环：✅ 管线阶段实现而非语言特性变更；ch2 parse model 自身固定「token 流 + 行注解即解析的全部输入」，类型/代码生成闭环由 roadmap M3/M4 承载（M0/M1 同构先例）。
10. 未决问题：✅ 无——三项裁决已落 design（D6/D11、D4/D8–D10、D7/D12）。

**F1（非阻塞）**：泛型构造头 `Box<Int64> { … }` 在 M2 按比较式解析落 E0105 而非 ch8 边界行（design D5 披露的已知缺口，`<` 与比较式同形、前瞻检测不值代价）——ch8 里程碑（M5）落地构造表达式时自然吸收；实现审查时以真实二进制复跑该形状确认实际行为与披露一致。（更正 2026-09-05，实现期复跑：实际行为为 **E0104 于第二个运算符 `>`**——`<` 与 `>` 同属 9 级非结合级，链式判定先于构造语境到达；「非 ch8 边界」的披露实质不变，具体诊断码以复跑结果为准，design D5 已同步更正。）

## 实现审查记录

**2026-09-05，active → complete，welang-code-review 七条，通过（发现 F2，审查期即修）。**

1. 规范符合性：✅ ch2/ch6/ch7（Q3 范围）全部规范原例以真实二进制复跑核对——E0102（`svc`⏎`.query()` 双位置）、E0103（`let x = y = 1`、`work(a = 1)`）、E0104（`a < b < c`、`a..b..c` 两消息形）、E0105（`list[0]` 索引点名命名方法、顶层 `compute()` 点名项产生式、`u.name = "bob"`、裸参数 `fn bad(x)`）、E0401–E0405、E0012/E0013 共 16 例逐一位置与消息符合注册表语义；行接续正例（运算符尾随续行、成员链 `.` 续行、括号内换行无意义、`f(1,`⏎`2,`⏎`3)`）全部解析至类型检查边界；构造/单元/闭包/ch10 `self.field` 落对应边界行。
2. 验证诚实性：✅ T1–T8 逐项复核——构建/测试/validate/docs_sync/git diff --check 于本审查会话真实复跑全过；T2/T3 红证据（23 例黄金红、9/9 单测红）与失败计数已记完成记录；T8 阶梯首跑 gofmt 三文件违例如实记录并修复后复跑空。
3. 测试先行证据：✅ 黄金先红（T2）、单测骨架先红（T3）有计数证据；conformance 期望符合 §52 人类可读形与 §62 JSON Lines 协议（--json 实测字段序 type/severity/code/message/file/line/column/help，help 全字段在）。
4. 诊断协议稳定：✅ 零 --json 字段增删改名（report 路径未动）；零新增诊断码（E0101–E0105/E0012/E0013/E0401–E0405 均为注册表既有条目，与零规范增量声明一致）。
5. 单一权威：✅ 机制自由度（AST 形状、行接续实现、E0105 消息表、helps 过渡态）记录于 design D1–D15 作事实稳定表面；helps 硬编码维持 M1 模式并显式指向 roadmap 待办 2（embed）；代码注释全英文（grep 复核，中文仅存在于 M1 既有测试的字面量输入数据）；docs/ 除 roadmap 状态行（T10 归档动作）外零改动。
6. 红线复核：✅ `git status` 全量对照影响范围清单——改动恰为声明文件集 + check-bom-crlf（D14 已披露第三例同源改写）；refr/ 零文件；commit 待 T10 后按规范落（无署名 trailer，钩子不绕过）。
7. 最小可信验证已跑：✅ F2 修复后全阶梯复跑——gofmt -l 空、go build、go vet、go test ./... 全绿、validate --strict `OK`、docs_sync `OK: 30 document pair(s) aligned`、git diff --check 过。

**F2（审查期发现即修）**：closeAngle 对 `>=` 与 `>>` 共用「剩余字节改写为 `>`」分支，把 `>=` 的 `=` 重写成 `>`——单闭口连写 `let n: Map<String, Int64>= never` 误报 binding-head E0105（真实二进制取证 `1:26` 即 `=` 列）。既有单测的 `>>=` 输入词法上切成 `>>`+`=` 两 token，从未触达该分支（注释声称测 `>=` junction 实未命中——未测分支藏缺陷）。修复按测试先行：先补红例 `Map<String, Int64>= never`（红证据：`E0105 unexpected token — ">" in a binding head` 于 1:26），closeAngle 改为按第二字符拆分（`>>` 余 `>`、`>=` 余 `=`），复跑全绿 + 二进制复验 `Map<Int64, Int64>= other` 达类型检查边界。完成记录 T6 已同步更正。

**F1 复核（审查记录遗留项）**：以真实二进制复验泛型构造头 `Box<Int64> { … }` → `2:22: error[E0104]: chained non-associative operator — ">" chains a non-associative level; write the split form, for example (a < b) && (b < c)`，exit 1——与更正后的披露（E0104 于第二个运算符 `>`）一致，design D5 / proposal F1 更正成立。
