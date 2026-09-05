# tasks.md — types-and-main

- [x] T1 internal/ast 增量：和式声明节点（类型名/变体表/载荷复用 TypeRef/pub/byval 位）与单元值表达式节点
  来源：proposal 目标 3 / design D7 D8
  验证：`go build ./...` 通过；节点无行为方法（纯数据）
- [x] T2 测试先行：D16 黄金表落盘（新增 + 改写，含项目模式 fixture），对当前构建运行必须先红
  来源：proposal 目标 8 / design D16
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红，red 证据（失败用例名清单）记于完成记录
- [x] T3 测试先行：parser 修订测试（派发表 type 行、TestTypeRefs F3 翻转、和式声明与行接续）与 typecheck 单测骨架两批先写先红
  来源：proposal 目标 3 7 / design D7 D15 D18
  验证：`go test ./internal/parser/`（F3 翻转红——现行接受未分隔形）与 `go test ./internal/typecheck/`（包未实现红），red 证据记于完成记录
- [x] T4 internal/parser 修订实现：和式声明解析（前缀序/变体载荷/行接续）、单元值 `()`、`type` 派发行落地、F3 closeAngle 分隔收口
  来源：proposal 目标 3 7 / design D7 D14 D15
  验证：`go test ./internal/parser/` 全绿（含翻转后的 F3 与派发表穷举）
- [x] T5 internal/typecheck 实现一：类型表示与结构相等（D1）、作用域链三层（D2）、预导入 30 名登记与逐名处置（D3）、字面量 context-free 定型、运算符域表（D4）、E0501 位点锚点表、E0502 常量折叠（D5）
  来源：proposal 目标 1 2 / design D1–D5
  验证：`go test ./internal/typecheck/` 对应测试绿
- [x] T6 internal/typecheck 实现二：块值与体尾四分 + E0605 位（D6）、期望类型线程（D8）、构造机制（E0704/裸单元变体/E0703 位点表/E1204 归属读法）、内建 Result/Option（E0827/E0828/E1204，D9）、E1304 三形与 checker 级边界 What 表（D10 D14）
  来源：proposal 目标 2 3 4 5 / design D6 D8–D10 D14
  验证：`go test ./internal/typecheck/` 全绿
- [x] T7 internal/cli：runCheck 项目模式（极简 TOML 读取 + E1905/E1904/E2004/E1903 触发表 + 源根与根模块 + 缺文件 E1305 读法，D11）、E1305 判据链（D12）、单文件模式 import 处置（D10）、成功三面（静默 0 / --json 零事件 / --verbose 一行，D13）、「type checking 未实现」尾边界删除
  来源：proposal 目标 5 6 / design D10–D13
  验证：`go test ./internal/conformance/` 全绿（含骨架端到端绿用例）
- [x] T8 验证阶梯：gofmt -l（空）、go build ./…、go vet ./…、validate --strict、docs_sync、git diff --check
  来源：研发流程验证阶梯
  验证：全部命令通过，输出记于完成记录
- [x] T9 实现审查（welang-code-review 7 条，真二进制证据）结论写回 proposal；通过后 status → complete 并归档（2026-09-05-types-and-main）
  来源：研发流程
  验证：openspec/changes/archive/2026-09-05-types-and-main/ 存在且 validate --all --strict 通过
- [x] T10 roadmap：M3 行翻写 done（双语对同步）+ Registered follow-ups 登记 #4（运算符类型域表）与 #5（调用实参个数与非函数被调诊断码）两条（双语对同步）
  来源：roadmap 执行状态规则 + design D4 D10 的发现登记义务
  验证：`python3 openspec/tools/docs_sync.py` 通过；表格行含 done 与两条新编号项

## 完成记录

**T1（2026-09-05）**：`internal/ast` 增 `SumDecl`/`Variant`/`Unit` 三节点（纯数据、1-based 位点）；`go build ./...` 与 gofmt 通过。

**T2 红证据（2026-09-05）**：`go test ./internal/conformance/ -run TestGoldenCases` → **52 枚失败**，全部为本次新增/改写用例，零意外失败：

- 新增 45 枚 + 改写 2 枚（check-clean / check-clean-json——D16 的真干净改写）；
- M2 时代需随 M3 翻转、一并改写为 M3 期望并落入红集的 5 枚：check-bom-crlf（`()` 转合法 + AppError 声明补齐 → exit 0）、check-parse-clean / check-parse-clean-json（尾部 type-checking 边界删除，其 `import std.io` 在 M3 先触 std 边界行——顺带钉住 ch21 R2 模块解析先于类型检查的次序）、check-boundary / check-boundary-json（项目边界删除，空目录 → E1905）；
- 保持绿（按设计不翻转）：check-boundary-ch8 / check-boundary-ch8-json——记录构造花括号与元组表达式的 ch8 边界保留至 M5；其余 M0/M1/M2 黄金全绿。

**T3 红证据（2026-09-05）**：

- `go test ./internal/parser/` → 4 套件红，其余绿：TestKeywordDispatchExhaustive（`type` 行翻为 clean）、TestExpressions（`()` 单元值）、TestSumDecls（新增，D7 全量：前缀/续行/行首 `|` E0102/`type T = Int64` 遮蔽边例/E0011/E0404/E0701/泛型与 derives 边界/前缀序）、TestTypeRefs（F3 翻转：未分隔 `>>`/`>=` → E0105，分隔形 clean）；
- `go test ./internal/typecheck/` → 编译失败红：`undefined: Mode / Check / NotImplemented`（测试即 API 契约，覆盖 D1 结构相等、D3 预导入逐名、D4 字面量与运算符域与 E0501 锚点、D5 折叠、D6 体尾四分与 E0605、D7/D8 和式与构造与 E0703 位点、D9 内建和式三码、D10 名字解析与 import 处置、D11/D12 项目模式与 main 形状链、D14 边界 What 十二行）。
  （勘误：本条与上条之间原缺 T4 条目头——T3 的第二对来源/验证实为 T4 所有，已补 T4 行。）

**T4 完成（2026-09-05）**：`go test ./internal/parser/` 全绿（10 套件：TestKeywordDispatchExhaustive / TestLineJoining / TestPrecedence / TestStatements / TestExpressions / TestDeclarations / TestSumDecls / TestTypeRefs / TestBoundaryForms / TestDiagnosticShape）。T3 红四套件全部转绿；F3 翻转后未分隔 `>>`/`>=` E0105、分隔形 clean。

**T5/T6 完成（2026-09-05）**：`internal/typecheck/typecheck.go` 落地（约 900 行），`go test ./internal/typecheck/` 全绿（9 套件：D4 字面量与运算符 / D5 E0502 折叠 / D6 块值体尾 / D7/D8 和式与构造 / D9 内建和式 / D2/D3/D10 名字解析 / D11/D12 项目模式与 main 形状 / D1 结构相等）。实现披露（D17 之外的机制读法与裁量）：

- **前向 topLet 引用读 E1304 裸形**：模块绑定按源序定型（pass 2b 先 topLet 后 fn 体），前向引用处 letType 尚空 → 按未解析名报——这是「模块绑定只引用更早者」规则的机制读法（ch15 一模块一名字空间，定型次序是源序），披露不另设码。
- **对模块 let 赋值只查型合**：赋值位不查可变性位（ch8 所有权级 M5 才抵达）——permissive 角落，披露。
- **`~` 不入折叠集**：D5 折叠集 = 整数字面量、一元 `-`、`+ - * / % << >> & ^ |`；`~127i8` 走通用二元/一元定型路径，不做越界折叠。
- **测试侧更正（先红后绿过程中的对表勘误，均有理由）**：单测 5 处列号算错（`&&` 14、fn 型 Never 12、`Option<` 14、参数 Never 9、`Circle(` 16——按 E1304/golden 同位逐字符复核）；`~true` 行由域表边行改 E0501（D4 锚表：一元域违例是 E0501，二元同型域外才是边行）；`1f32` 非法字面量（E0006）改 `1.5f32`；`Result<(), E2>>` 未分隔触发 F3 E0105 改 `> >` 分隔；check-e0605-stmt 黄金源与 stmt 形消息不相容（原源=体尾单表达式）补尾行 `let u = ()` 使其真为语句位；check-e0105-unseparated 黄金消息按实现串更正（分隔补救句式）。
- **`pub type` 解析缺位（M2 符合性缺口，M3 黄金发现）**：ch6 批准 `[pub] [byval] type`，M2 parseItem 的 `pub` 分支漏裸 `type` 行——6 枚黄金（check-clean / check-clean-json / check-bom-crlf / check-project-skeleton / check-e1302-project / check-bnd-multi-module）先红为证，补行后全绿。
- **E1304 消息形统一为两形（裸/限定符），与位置无关**：T2 两枚类型位黄金互不相容（check-e1304-type 要「type positions resolve by the same rules」形，check-f3-separated 要裸形，同为类型位）——注册表枚举三种触发且「type positions resolve by the same rules」是澄清句非第四触发；同规则即同消息，check-e1304-type 更正为裸形（实现删除 typeUnresolved 变体）。

**T7 完成（2026-09-05）**：`internal/cli/check.go` 重写（单文件/项目两模式、manifest 四码触发表、E1305 缺根读法、成功三面、尾边界删除）；`go test ./internal/conformance/` 全绿——**101 枚黄金全过**（T2 红 52 枚全转绿，含 check-project-skeleton = roadmap 承诺的 `we check .` 于 `we new` 骨架静默 exit 0）。

**T8 完成（2026-09-05）**：验证阶梯全过——`gofmt -l .` 空；`go build ./...` 过；`go vet ./...` 过；`go test ./...` 全绿（conformance 101 / diag / lex / parser 10 套件 / typecheck 9 套件 / version）；`python3 openspec/tools/validate.py types-and-main --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`；`python3 openspec/tools/docs_sync.py` → `OK: 30 document pair(s) aligned`；`git diff --check` 过。

**T9 完成（2026-09-05）**：welang-code-review 七条全过（proposal「## 实现审查记录」）——真二进制证据约 40 次调用（骨架三面、17 码双面、四条边界行、F3 两形、E1305 五判据、manifest 四码）。三项发现当场处置：① 测试侧勘误六款（先红后修，3 条披露于 T5/T6 记录）；② E1304 消息形统一为裸/限定两形与位置无关（check-e1304-type 黄金更正）；③ `pub type` M2 符合性缺口（6 枚黄金先红为证修复）。status → complete（validate --strict OK）后归档 2026-09-05-types-and-main（plain mv），validate --all --strict OK（无活跃变更，注册表净）。

**T10 完成（2026-09-05）**：roadmap M3 行翻 done（双语对同步：0000-reference-implementation.md/.zh.md）；follow-up 登记 #4（运算符类型域表）与 #5（调用实参个数与非函数被调诊断码）双语对同步；`docs_sync` → OK: 30 document pair(s) aligned。
