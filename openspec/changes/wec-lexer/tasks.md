# tasks — wec-lexer（B3b）

> 权威：proposal.md（范围）+ design.md（D1–D7）。每任务实现前待用户点名；红先纪律 = 对账塔/子集在对应面落成前取红态（构建失败 / 输出字节不匹配两形，退出码与输出记档）。

## T1 wec 骨架 + 对账塔红先

- [ ] **对账塔先行**：`internal/wec/wec_test.go`——`wec/` 拷入 `t.TempDir`、`cli.Run` 构建、exec 工件、参考侧 `lex.Scan` 活算序列化、`bytes.Equal` 比对；语料提取器（stdlib 源直读 + conformance case JSON setup 提取，只读）+ 阶段门槛（参考侧 kind 集围栏）；Go 侧序列化器（design D4 规范）。塔落定时 wec/ 尚缺席——构建失败红态记档。
  来源：proposal 目标 4；design D4、D5、D7
  验证：红态记档（wec 缺席 → `we build` 失败）；`go vet ./internal/wec/`；塔自身单测（提取器条目数 > 0、序列化器自一致）
- [ ] **wec 骨架绿一枚**：`wec/we.toml`（name=wec、type=executable）+ `src/main.we` 最小驱动（读 `wec-input.we` → `tokens` 头 + `T … eof` 占位序列化）+ `src/lex.we`/`src/lexnum.we` 空壳模块（占位 fn，保证模块切分与导入链通）；语料首枚（`let x = 1` 形）塔绿。
  来源：proposal 目标 1、3；design D4、D6
  验证：首枚语料 `bytes.Equal` 绿；`go test ./internal/wec/ -count=1` 该子集绿；`we check wec/` 干净


## T2 扫描核：ident/keyword/运算符/空白注释跳过

- [ ] **checkEncoding 三面 + rune 走查地基**：`widthOf`、有效 UTF-8 判定（Σ 预测宽 == byteLength）、首病字节定位 + 字节值探针查定（243 值；不中即任务失败）、BOM 剥离/中置报错、裸 CR 报错——三面次序照 Go；`posAt` 前缀切片走查。
  来源：proposal 今日事实 4/5、目标 2；design D2
  验证：塔语料⑥（名可达五形 + BOM 中置 + 裸 CR）+ 语料③的编码子项先红后绿；13 值角剔除披露落测试注释；`"\xff"` 剔除记档
- [ ] **ident/keyword/运算符/跳过逻辑**：`scanAll` 主循环（空白/行注释/块注释跳过——块注释不嵌套、E0004 未闭）、ident 扫描（`_` 与字母数字 ASCII 域）、关键字表（String 列表字面 + 线性扫）、运算符逐字符分发（kind = 自身文本）、eof 词；行列维护（rune 列）；错误闩 + fail。
  来源：proposal 目标 2；design D1、D2、D6
  验证：塔语料子集（参考侧 kind ⊆ {ident, keyword, 运算符, eof} 的条目——含 stdlib 源/wec 自源/conformance 源的大部）先红后绿；E0004 未闭块注释语料绿


## T3 数值与 rune 字面量

- [ ] **numericSpan 移植**：E0006 七条款（前导零、`_` 位间、点侧数字、指数形）、后缀集（i8–u64/f32/f64）、`1e5` 切形（无点指数的 int/float 分界）；int/float kind 判定。
  来源：proposal 目标 2；design D2（lexnum.we 模块）
  验证：E0006 全码矩阵语料（每条款一正一负）先红后绿；conformance 源子集（数值密集）翻绿
- [ ] **rune 字面量**：`'x'` 单字符/单逃逸形、E0003 两形（未闭/多字符）、逃逸入口共享 T4 的 consumeEscape（本任务先直通字符形）。
  来源：proposal 目标 2
  验证：rune 正负矩阵语料绿；`'\u{41}'` 形留 T4（逃逸校验落成后补绿）


## T4 字符串、逃逸、attr、插值洞

- [ ] **字符串与逃逸**：`consumeString`（同行闭引 E0002 两形、`$` 非洞普通字符）、`consumeEscape`（`\n \t \r \0 \\ \" \' \u{1-6 hex}` 闭集 E0005、`\u` 域校验——0x10FFFF 上限 + 代理排除，与 Go 词法器同层）。
  来源：proposal 目标 2；design D2
  验证：逃逸正负矩阵（含 `\u{D800}`/`\u{110000}` E0005、尾随 `\` E0005）先红后绿；非 ASCII 字面语料⑦绿
- [ ] **attr 与洞**：attr 序 form（E0001）→ literal-only（E0009）→ whitelist（E0008 空集）次序；`consumeHole` 深度/括号平衡（E0007 未闭、E0002 跨行）；AttrName/AttrArgs 原样文本（byteSlice）。
  来源：proposal 目标 2；design D2
  验证：attr 三码矩阵（`#[test]` E0008、`#[x = f()]` E0009、`#[` E0001）+ 洞平衡语料先红后绿；语料⑤全量门槛复验（此刻参考侧 kind 集应全覆盖）


## T5 全量对账 + 突变电池 + 零漂移

- [ ] **全量逐字节对账**：语料①–⑦全清单（stdlib 7 文件 + wec 自源 3 + lex 样例 + E0001–E0009 矩阵 + 933 案例源 + 编码补充形 + 非 ASCII 项）塔全绿——阶段门槛摘除。
  来源：proposal 目标 4；design D7
  验证：`go test ./internal/wec/ -count=1` 全量绿；语料条目数 as-built 记账；逐字节（成功/失败两栏）
- [ ] **突变电池 ≥6 枚**：design D7 的 M1–M6 逐枚施加于 wec We 源、塔击杀判决表、逐枚还原。
  来源：design D7
  验证：每枚「施加 → 至少一枚语料红 → 还原 → 全绿」记档；击杀层（塔/单测）逐枚判定
- [ ] **零漂移全量复验**：conformance 933 `-count=1` 零改写全绿；`go build ./...`、`go vet ./...`、`gofmt -l` 0 文件、16+1 包全绿；`git diff --check` 净。
  来源：proposal 目标 6
  验证：各命令退出码 0；testdata `git status` 零改动


## T6 审查与归档

- [ ] **完成审查**：welang-code-review 七条（span 对账、验证诚实性、红先在册、诊断协议、单一权威、红线、最小可信验证）；发现随审处置。
  来源：AGENTS.md 流程
  验证：审查表入 proposal「完成审查」；七条全过
- [ ] **roadmap + follow-up + 归档**：B3b 行翻写 done（as-built 收口）；follow-up 登记 #29（argv/stdin 面）与 #30（字节暴露面）；`change.yaml` archived；目录 `mv` 至 `archive/2026-…-wec-lexer/`；归档提交。
  来源：proposal 目标 7；B2a/B2b/B3a 归档先例
  验证：`validate.py --all --strict` 归档后期望态；归档复验（17 包 0 FAIL、conformance 933、docs_sync 33 对）
