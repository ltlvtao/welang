# tasks.md — lexical

- [x] T1 conformance runner 支持 Setup.Files（dirs 先建、文件后写、按路径排序），既有 20 用例不受影响
  来源：proposal 目标 6 / design D11
  验证：`go test ./internal/conformance/` 在仅扩展 Setup 后全绿（20/20）
- [x] T2 测试先行：新增 15 个 check-* 黄金用例并改写 check-boundary / check-boundary-json（期望手钉自规范场景），对当前构建运行必须先红
  来源：proposal 目标 2/3/6 / design D11
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红（新用例与改写用例失败），red 证据记于完成记录
- [x] T3 测试先行：internal/diag 带位置人类渲染快照（`file:line:column: error[code]: …`）先红
  来源：proposal 目标 5 / design D8
  验证：`go test ./internal/diag/` 红（位置快照用例失败），red 证据记于完成记录；无位置快照保持 M0 形状
- [x] T4 internal/diag：Human() 带位置前缀（JSON Lines 零改动）
  来源：design D8
  验证：`go test ./internal/diag/` 全绿
- [x] T5 internal/lex 实现：D1 token 模型、D3 数值、D4 字符串/rune/插值、D5 注释、D6 编码/BOM/CR、D7 属性、D10 关键字表 + 按章单测（token 清单表、最大吞噬 0..9/1.5..2.5/0x1F/1_000、41 关键字全枚举、数值校验全条款、转义/插值配平/注释不嵌套/BOM+CRLF）
  来源：proposal 目标 1/2 / design D1–D7 D10
  验证：`go test ./internal/lex/` 全绿
- [x] T6 internal/cli：check 分派（目录边界 / .we usage / 读文件 / 词法首错误 / parsing 边界），check 自通用 70 边界摘出
  来源：proposal 目标 3/4 / design D9
  验证：`go test ./internal/conformance/` 全绿（35/35）
- [x] T7 验证阶梯：gofmt -l（空）、go build ./…、go vet ./…、validate --all --strict、docs_sync、git diff --check
  来源：研发流程验证阶梯
  验证：全部命令通过，输出记于完成记录
- [ ] T8 实现审查（welang-code-review 7 条）结论写回 proposal；通过后 status → complete 并归档（2026-09-05-lexical）
  来源：研发流程
  验证：openspec/changes/archive/2026-09-05-lexical/ 存在且 validate --all --strict 通过
- [ ] T9 roadmap M1 行翻写 done 2026-09-05（双语对同步）
  来源：roadmap 执行状态规则（状态只在归档变更中翻写）
  验证：`python3 openspec/tools/docs_sync.py` 通过；表格行含 done 2026-09-05

## 完成记录

- 2026-09-04 T1 绿：`go test ./internal/conformance/ ok`（20/20）。
- 2026-09-04 T2 red 证据：实现前 `go test ./internal/conformance/ -run TestGoldenCases` 失败——17 例红（15 新增 check-* 全部 mismatch：实际输出为通用 `we: check is not implemented in this reference build yet` 边界行 + 2 改写 check-boundary(-json) 期望项目边界 70 而实际为旧通用边界文本），18 例旧黄金绿。
- 2026-09-04 T3 red 证据：实现前 `go test ./internal/diag/` 失败——TestHumanPositionSnapshot 期望 `demo/main.we:1:9: error[E0006]: …` 实得无前缀 `error[E0006]: …`；T4 实现后全绿，既有三条 M0 快照（JSON×2 + 无位置 Human）逐字节不变。
- 2026-09-05 T5 绿：`go test ./internal/lex/ ok`（10 个测试函数：token 清单 / 41 关键字枚举+近邻 ident / 数值合法表 / 数值全条款非法表 / 0..9、1.5..2.5、1e5 切分 / 字符串转义-插值-三失败码 / rune / 注释不嵌套 / 编码 BOM-CRLF-裸CR-非法UTF-8 / 属性形态-E0009先于E0008-白名单注入token形状 / 首错即停）。过程披露：按章单测抓出四处实现缺陷并当场修复——(a) 单字符 `=` 漏出运算符清单（对照 ch1 清单原文补入）；(b) 整数后缀判定序——`i8`/`f32` 含数字，须先查后缀闭集再判纯字母尾（`42i8` 曾被错误切回为 `42`+ident，`42f32` 曾漏报）；(c) 插值洞须跟踪 `(`/`[`——未闭括号内 `}` 不关洞，这是规范示例 `"${f(}"` 判 E0007 的必要机制（纯花括号计数下它平衡），已补记 design D4；(d) E0005/E0001 消息引号统一双引号（`"\e"`、`"$"`，与其余消息一致）。
- 2026-09-05 T5 附：黄金用例 check-e0009 的属性名 `timeout` 撞 41 关键字表（并发章节保留字），实现按 D7 正确报 E0001 形态错——修金样（改 `#[limit(...)]`、期望位 1:9）不改码。
- 2026-09-05 T6 绿：`go test ./internal/conformance/ ok`（35/35）；internal/cli/check.go 新增 runCheck（D9 全序：E1907 先于分派不动 → 目录→项目边界 70 → 非 .we→usage 2 → 读失败→fsError 1 → 词法首错→协议双面 1 → 干净→parsing 边界 70），cli.go 捕获 os.Stat 的 FileInfo 分派、check 标记 implemented。
- 2026-09-05 T7 通过：`gofmt -l internal/` 空；`go build ./...` 通过；`go vet ./...` 通过；`go test ./internal/...` 五包全 ok；`python3 openspec/tools/validate.py --all --strict` → OK: 1 change(s) valid; registry clean; mode=strict；`python3 openspec/tools/docs_sync.py` → OK: 30 document pair(s) aligned；`git diff --check` 干净。
