# tasks.md — parser-core

- [x] T1 internal/ast：D1 节点模型（File/Item/Stmt/Expr/TypeRef + 位置）
  来源：proposal 目标 1 / design D1
  验证：`go build ./...` 通过；节点无行为方法（纯数据）
- [x] T2 测试先行：新增 21 个 check-* 黄金用例并改写 check-clean / check-clean-json（期望手钉自规范场景与 D14 表），对当前构建运行必须先红
  来源：proposal 目标 5 / design D14
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红（新用例与改写用例失败），red 证据记于完成记录
- [x] T3 测试先行：internal/parser 单测骨架先红——41 关键字派发表穷举（D6 三位置映射）与行接续表（D3 两判定点）两测试先写
  来源：proposal 目标 1 / design D3 D6
  验证：`go test ./internal/parser/` 红（包尚不存在/未实现），red 证据记于完成记录
- [x] T4 internal/lex：`Scan` 旁路文档单元（D10：连续 /// 一单元、空行/普通注释断开、不产 token），`File` 薄壳化 + 文档单元单测
  来源：proposal What Changes / design D10
  验证：`go test ./internal/lex/` 全绿（M1 十测试函数不动）
- [x] T5 internal/parser 实现一——ch2 骨架：行接续（D3）、块与块值、语句族（D4 含 `=` 三分规则）、表达式骨架与 12 级优先表（D5，E0104 两级）、E0102/E0103/E0105（D12 消息表）
  来源：proposal 目标 1 / design D3–D5 D12
  验证：`go test ./internal/parser/` 对应测试绿
- [x] T6 internal/parser 实现二——ch6 声明：顶层项与关键字派发表（D6）、fn 签名与参数（D7）、return 与 E0401/E0402（D4）、import 与 E0013/E0012（D8）、E0403/E0404、文档附着与 E0405（D10）、类型槽文法（Q3 全量，D7/D12）
  来源：proposal 目标 2 / design D6–D10
  验证：`go test ./internal/parser/` 全绿（含关键字穷举测试）
- [x] T7 internal/cli：runCheck 接 parser.Parse——诊断双面报告 1 / NotImplemented 边界行 70 / 干净解析 type checking 边界 70（D11）
  来源：proposal 目标 4 / design D11
  验证：`go test ./internal/conformance/` 全绿
- [x] T8 验证阶梯：gofmt -l（空）、go build ./…、go vet ./…、validate --all --strict、docs_sync、git diff --check
  来源：研发流程验证阶梯
  验证：全部命令通过，输出记于完成记录
- [x] T9 实现审查（welang-code-review 7 条）结论写回 proposal；通过后 status → complete 并归档（2026-09-05-parser-core）
  来源：研发流程
  验证：openspec/changes/archive/2026-09-05-parser-core/ 存在且 validate --all --strict 通过
- [x] T10 roadmap M2 行翻写 done 2026-09-05（双语对同步）
  来源：roadmap 执行状态规则（状态只在归档变更中翻写）
  验证：`python3 openspec/tools/docs_sync.py` 通过；表格行含 done 2026-09-05

## 完成记录

- **T1 完成**：`internal/ast/ast.go`（File/Item/Stmt/Expr/TypeRef 纯数据节点 + 1 起行列位置；File 携 Docs 附着记录），`go build ./...` 通过。
- **T2 红证据（2026-09-05）**：新增 21 个黄金用例 + 改写 check-clean / check-clean-json 落盘后，`go test ./internal/conformance/ -run TestGoldenCases` 红——23 例失败（21 新增 + 2 改写），实际输出均为旧边界行 `we: parsing is not implemented in this reference build yet` + exit 70（want：各诊断或新边界行 + 1/70）；`grep -c 'FAIL: TestGoldenCases/'` = 23，与新增/改写用例名一一对应；M1 其余 33 例保持绿。
- **T3 红证据（2026-09-05）**：`internal/parser/`（存根 Parse + 九测试函数：41 关键字派发表穷举 / 行接续两判定点 / 12 级优先表 / 语句族 / 表达式骨架 / 声明 / 类型引用 / 边界形式组 / 诊断形状）对存根运行 9/9 FAIL（`got E0105 (unexpected token — "parser" is a stub ...)`）。
- **T4 完成**：`internal/lex` 增 `Scan`（token 流 + `DocUnit` 旁路：连续 /// 一单元，空行/普通注释断开；`File` 薄壳化）；`go test ./internal/lex/` 全绿（M1 十测试函数不动 + 新 TestDocUnits）。缺陷披露：TestDocUnits 初版对「块注释同行断开」的期望写反（期望同单元），实现按 D10 语义正确断开——修期望而非实现。
- **T5 完成（2026-09-05）**：ch2 骨架全量落地——行接续（深度零两判定点：二元循环头与后缀循环头；续行集与语句起始类按 D3 封闭表）、块即表达式、语句族（绑定/赋值/return/表达式语句；`=` 三分规则经 exprCtx（valueCtx/stmtCtx）线程化）、12 级优先表（9/12 级非结合 → E0104 于第二运算符）、E0102/E0103/E0105 按 D12 消息表（help 取注册表 remediation 压缩）。缺陷披露（1 个真实现缺陷）：parseTopLet 初版在 parseBinding 前多调一次 `p.next()`——parseBinding 自己消费 let 关键字，导致绑定把名字当关键字、在 `=`/`:` 处误报 binding-head E0105；对照单测定位后删除多余消费修复。测试侧修正（期望错而非实现错）：`Dyn<io>`（小写泛型实参）改为 `Dyn<I>`——D7 定型名 PascalCase，小写实参本身就会触发类型槽 E0105；`a..b..c` E0104 定位于 17 列（非 19）、`a < b == c` 于 19 列（非 20）、`f() {` 构造头于 9 列、`b {` 于 15 列——均为列算术错；import eof 于 (2,1)（非 (1,18)，文件尾换行后 eof token 落次行首）；fn 类型 `=` 残留于 18 列。
- **T6 完成（2026-09-05）**：ch6 声明全量——41 关键字三位置派发（D6 表由 parser_test.go dispatchTable 41 行穷举钉死）、fn 签名（裸参数 E0105 于参数名、冒号检查先于 checkCamel、`<` → ch10 边界、签名内 `effect` → ch16 边界、`mut` 参数 → mut 参数边界）、return 形式（E0401 先于 E0402 判定；hasValue = 非空非 `}` 且深度零不跨行）、import（E0013 段级校验、引入名 = alias 或末段、as 后非 ident → E0105）、E0012（fn/参数/绑定名 camelCase；`_` 豁免）、顶层 var → E0403、E0404（名字表 map[name]首行，块内名不入表）、文档附着后置遍（E0405 于单元首行 1 列）、类型槽文法 Q3 全量（Name/限定路径/泛型应用/元组≥2/单元/fn 类型；泛型闭包 `>>`/`>=` 的 maximal-munch 合并由 closeAngle 拆分——原位改写当前 token 为 `>` 于 Col+1，`Vec<Vec<Int64>>` 与 `Map<K,V>= …` 均解析；更正 2026-09-05，实现审查 F2：`>=` 分支同用 `>` 改写，把本应保留的 `=` 重写成了 `>`——单闭口 `Map<K,V>= x` 误报 binding-head E0105，而既有单测输入 `>>=` 实为 `>>`+`=` 两 token、从未触达该分支；已按测试先行补红例 `Map<String, Int64>= never` 后修为按第二字符拆分）。`go test ./internal/parser/` 全绿（9/9 测试函数）。
- **T7 完成（2026-09-05）**：runCheck 接 `parser.Parse`——诊断双面报告 + exit 1、NotImplemented 边界行 `we: %s are not implemented in this reference build yet` + exit 70、干净解析 `we: type checking is not implemented in this reference build yet` + exit 70；M1「parsing is not implemented」行删除。缺陷披露（黄金期望错，4 例空行计数错）：check-e0404（3:4→4:4）、check-e0405（3:1→4:1）、check-e0105-top-stmt（4:1→5:1）、check-e0401（4:1→5:1）——生成器对输入空行计漏，逐一按实际行号修正并以脚本对全部含 `\n\n` 输入的黄金做系统审计（无进一步实例）。第三例同源改写 **check-bom-crlf**：旧期望钉住 M1「parsing 未实现」边界行，管线推进后同输入（骨架含 `Ok(())`）落 ch8 边界行——期望随改（exit 仍 70），D14 已补记。`go test ./internal/conformance/` 全绿（56 例）。
- **T8 完成（2026-09-05）**：验证阶梯全过——`gofmt -l .` 空（初跑 3 文件违例：internal/ast/ast.go、internal/lex/lex.go、internal/parser/parser_test.go，`gofmt -w` 后复跑空）；`go build ./...` 过；`go vet ./...` 过；`go test ./...` 全绿（conformance 56 / diag / lex / parser / version）；`python3 openspec/tools/validate.py parser-core --strict` → `OK: 1 change(s) valid; registry clean; mode=strict`；`python3 openspec/tools/docs_sync.py` → `OK: 30 document pair(s) aligned`；`git diff --check` 过。F1 复跑证据（真实二进制）：`/tmp/f1.we:2:22: error[E0104]: chained non-associative operator — ">" chains a non-associative level; write the split form, for example (a < b) && (b < c)`，exit=1——design D5 与 proposal F1 已同步更正（预判 E0105 → 实际 E0104）。

- **T9 完成（2026-09-05）**：welang-code-review 七条全过（proposal「## 实现审查记录」）；发现 F2——closeAngle 对 `>=` 共用 `>` 改写分支、单闭口连写 `Map<String, Int64>= never` 误报 E0105（既有 `>>=` 单测实为两 token、从未触达该分支）——按测试先行补红例后修为按第二字符拆分，全阶梯复跑过；F1 复验与更正后披露一致（E0104 于 `>`）。status → complete（validate --strict OK）后归档 2026-09-05-parser-core（plain mv），validate --all --strict OK（无活跃变更，注册表净）。
- **T10 完成（2026-09-05）**：roadmap M2 行翻写 `done 2026-09-05`（双语对同步），docs_sync `OK: 30 document pair(s) aligned`。
