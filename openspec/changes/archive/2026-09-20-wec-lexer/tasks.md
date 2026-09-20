# tasks — wec-lexer（B3b）

> 权威：proposal.md（范围）+ design.md（D1–D7）。每任务实现前待用户点名；红先纪律 = 对账塔/子集在对应面落成前取红态（构建失败 / 输出字节不匹配两形，退出码与输出记档）。

## T1 wec 骨架 + 对账塔红先

- [x] **对账塔先行**：`internal/wec/wec_test.go`——`wec/` 拷入 `t.TempDir`、`cli.Run` 构建、exec 工件、参考侧 `lex.Scan` 活算序列化、`bytes.Equal` 比对；语料提取器（stdlib 源直读 + conformance case JSON setup 提取，只读）+ 阶段门槛（参考侧 kind 集围栏）；Go 侧序列化器（design D4 规范）。塔落定时 wec/ 尚缺席——构建失败红态记档。
  来源：proposal 目标 4；design D4、D5、D7
  验证：红态记档（wec 缺席 → `we build` 失败）；`go vet ./internal/wec/`；塔自身单测（提取器条目数 > 0、序列化器自一致）
  落地（2026-09-17）：红态 = `copy wec: lstat ../../wec: no such file or directory`；门槛 as-built 扩错误码集第二臂 + `ran==0` 响亮失败（design 补记一-2）
- [x] **wec 骨架绿一枚**：`wec/we.toml`（name=wec、type=executable）+ `src/main.we` 最小驱动（读 `wec-input.we` → `tokens` 头 + `T … eof` 占位序列化）+ `src/lex.we`/`src/lexnum.we` 空壳模块（占位 fn，保证模块切分与导入链通）；语料首枚（~~`let x = 1` 形~~→空源形——占位序列化器只答空源；`let x = 1` 已入语料、围栏滤除待 T3 翻绿，design 补记一-1）塔绿。
  来源：proposal 目标 1、3；design D4、D6
  验证：首枚语料 `bytes.Equal` 绿；`go test ./internal/wec/ -count=1` 该子集绿；`we check wec/` 干净
  落地（2026-09-17）：`reconciled 2 corpus entries`（literal:empty + case:fmt-empty:src/empty.we）；三测全绿 + `we check wec/` exit 0


## T2 扫描核：ident/keyword/运算符/空白注释跳过

- [x] **checkEncoding 三面 + rune 走查地基**：`widthOf`、有效 UTF-8 判定（~~Σ 预测宽 == byteLength~~→FFFD 三字节探针单趟走查——Σ 形被包含，design 补记二-5）、首病字节定位 + 字节值探针查定（243 值；不中即任务失败）、BOM 剥离/中置报错、裸 CR 报错——三面次序照 Go；`posAt` 前缀切片走查。
  来源：proposal 今日事实 4/5、目标 2；design D2
  验证：塔语料⑥（名可达五形 + BOM 中置 + 裸 CR）+ 语料③的编码子项先红后绿；13 值角剔除披露落测试注释；`"\xff"` 剔除记档
  落地（2026-09-18）：三面次序照 Go；BOM offset = stripped 内偏移 + adjust、裸 CR quirk 保真（消息 offset 用 stripped 索引、posAt 走原串）；语料⑥五形 + BOM 对 + 裸 CR 两形 + crlf 全绿；`"\xff"` 剔除披露在塔日志（design 补记二）
- [x] **ident/keyword/运算符/跳过逻辑**：`scanAll` 主循环（空白/行注释/块注释跳过——块注释不嵌套、E0004 未闭）、ident 扫描（`_` 与字母数字 ASCII 域）、关键字表（~~String 列表字面 + 线性扫~~→41 词 `||` 等值链——List 载体被停面排除，design 补记二-4）、运算符逐字符分发（kind = 自身文本）、eof 词；行列维护（rune 列）；错误闩 + fail。
  来源：proposal 目标 2；design D1、D2、D6
  验证：塔语料子集（参考侧 kind ⊆ {ident, keyword, 运算符, eof} 的条目——含 stdlib 源/wec 自源/conformance 源的大部）先红后绿；E0004 未闭块注释语料绿
  落地（2026-09-18）：先红（占位态 261 放行 / 259 红记档）后绿 `reconciled 252 corpus entries`（−7 attr 面 E0001 围栏特判待 T4、−2 wec 自源携数值/字符串字面待 T3/T5 回归）；E0004 + docs 两形（run/broken）绿；17 包全绿、`we check wec/` exit 0


## T3 数值与 rune 字面量

- [x] **numericSpan 移植**：E0006 七条款（前导零、`_` 位间、点侧数字、指数形）、后缀集（i8–u64/f32/f64）、`1e5` 切形（无点指数的 int/float 分界）；int/float kind 判定。
  来源：proposal 目标 2；design D2（lexnum.we 模块）
  验证：E0006 全码矩阵语料（每条款一正一负）先红后绿；conformance 源子集（数值密集）翻绿
  落地（2026-09-18）：先红（533 放行/281 发散——WEC-PENDING number 形）后绿 `reconciled 531`；七条款 + based/float 验证器逐条款、八后缀序表、`1e5` 切形与 `..` 不入 span 两边界语料钉；数值密集 conformance 源随围栏翻绿、`literal:let-one` 兑现 T1 预言；%q 多 rune 面 quoteText（design 补记三）
- [x] **rune 字面量**：`'x'` 单字符/单逃逸形、E0003 两形（未闭/多字符）、逃逸入口共享 T4 的 consumeEscape（本任务先直通字符形）。
  来源：proposal 目标 2
  验证：rune 正负矩阵语料绿；`'\u{41}'` 形留 T4（逃逸校验落成后补绿）
  落地（2026-09-18）：单字符形（含非 ASCII `'中'`）+ E0003 两形（EOF/换行未闭、空引/多字符形状）语料绿；逃逸半面 = pendingFail("rune escape")，围栏三判据保守排除（成功路径文本含反斜杠、E0003 源含反斜杠、源含 `#[`——T4 骑码两形，design 补记三-1）；consumeRuneLit 每处 fail 早退保 panic 语义


## T4 字符串、逃逸、attr、插值洞

- [x] **字符串与逃逸**：`consumeString`（同行闭引 E0002 两形、`$` 非洞普通字符）、`consumeEscape`（`\n \t \r \0 \\ \" \' \u{1-6 hex}` 闭集 E0005、`\u` 域校验——0x10FFFF 上限 + 代理排除，与 Go 词法器同层）。
  来源：proposal 目标 2；design D2
  验证：逃逸正负矩阵（含 `\u{D800}`/`\u{110000}` E0005、尾随 `\` E0005）先红后绿；非 ASCII 字面语料⑦绿
  落地（2026-09-18）：先红（1104 放行/573 发散——538 string/30 attr/5 rune escape 三族 WEC-PENDING）后绿 `reconciled 1104`；闭集七码 + `\u` 域检（空/七位/无 `{`/代理/超域五负 + 10FFFF 正界）+ 尾随 `\` + E0002 两形（EOF/换行）+ 洞四形（平衡/嵌套串/`${f(}` 未平衡/未闭）语料钉；非 ASCII 字面⑦（`"héllo — 中"`）绿；rune 逃逸两枚（`'\n'`/`'\u{41}'`）随 consumeEscape 兑现 T3 预留；wec 自源 −2 于本任务兑现回归（string kind 开闸）
- [x] **attr 与洞**：attr 序 form（E0001）→ literal-only（E0009）→ whitelist（E0008 空集）次序；`consumeHole` 深度/括号平衡（E0007 未闭、E0002 跨行）；AttrName/AttrArgs 原样文本（byteSlice）。
  来源：proposal 目标 2；design D2
  验证：attr 三码矩阵（~~`#[x = f()]` E0009~~→实死于 E0001 form，E0009 真形 `#[x(y)]`/`#[limit(5000 + n)]`——任务书例形订正，design 补记四-2）+ 洞平衡语料先红后绿；语料⑤全量门槛复验（此刻参考侧 kind 集应全覆盖）
  落地（2026-09-18）：attr 三码序逐位钉（form 名前后两形/`#` 非跟 `[`、E0009 于实参起点、E0008 于 unit 起点）+ 实参字面量/列表/交错括号/实参内 E0006 五形；**follow-up #31 We 编译器 let-别名错编译被发现**（`let x = 裸局部 var` 绑存储非快照，探针族五证；wec 绕行 = argStart 用 var 快照——design 补记四-1，随 T6 登记）围栏终态 = 全 named kind 除 comment + E0001–E0009 全码集（唯余 13 值角 #30）；17 包全绿、conformance 933 不动、`we check wec/` exit 0


## T5 全量对账 + 突变电池 + 零漂移

- [x] **全量逐字节对账**：语料①–⑦全清单（stdlib 7 文件 + wec 自源 3 + lex 样例 + E0001–E0009 矩阵 + 933 案例源 + 编码补充形 + 非 ASCII 项）塔全绿——阶段门槛摘除。
  来源：proposal 目标 4；design D7
  验证：`go test ./internal/wec/ -count=1` 全量绿；语料条目数 as-built 记账；逐字节（成功/失败两栏）
  落地（2026-09-20）：stage/fence/admits/namedKinds 四件删除、13 值角剔除内联单一判定点；`reconciled 1104`（corpus 1105，唯一剔除 lextest `"\xff"` 披露）；行为保持（T4 围栏已放行全词表）——摘除的结构意义 = 不再有阶段门能藏未来漂移；三测全绿
- [x] **突变电池 ≥6 枚**：design D7 的 M1–M6 逐枚施加于 wec We 源、塔击杀判决表、逐枚还原。
  来源：design D7
  验证：每枚「施加 → 至少一枚语料红 → 还原 → 全绿」记档；击杀层（塔/单测）逐枚判定
  落地（2026-09-20）：6/6 击杀、击杀层全为塔层（发散 2/870/1031/13/2/205——M1 宽表/M2 关键词/M3 列推进/M4 `..` 条款/M5 \u 上限/M6 转义下界），逐枚 exit 码与判决表入 design 补记五；M5 定点性自证（代理臂未动 ⇒ surrogate 条目存活）；附带发现 = 解析器按文本收词、kind 面唯一观测点是塔；逐枚还原复跑绿 ×6
- [x] **零漂移全量复验**：conformance 933 `-count=1` 零改写全绿；`go build ./...`、`go vet ./...`、`gofmt -l` 0 文件、16+1 包全绿；`git diff --check` 净。
  来源：proposal 目标 6
  验证：各命令退出码 0；testdata `git status` 零改动
  落地（2026-09-20）：build/vet/gofmt（refr/ 外 0 文件）/git diff --check 全净；17 包 -count=1 全绿（conformance 200.8s 零改写、testdata 0 行漂移）；零 Go 生产码改动（本任务唯一改动 = wec_test.go 围栏摘除）


## T6 审查与归档

- [x] **完成审查**：welang-code-review 七条（span 对账、验证诚实性、红先在册、诊断协议、单一权威、红线、最小可信验证）；发现随审处置。
  来源：AGENTS.md 流程
  验证：审查表入 proposal「完成审查」；七条全过
  落地（2026-09-20）：span `1a7e81a..2c1dc7c` 六提交、9 文件清点与影响层一一对应；零码级发现、一处记录性注记（proposal 状态行 candidate 字样沿 B3a 归档先例不改写）；七条全过
- [x] **roadmap + follow-up + 归档**：B3b 行翻写 done（as-built 收口）；follow-up 登记 #29（argv/stdin 面）与 #30（字节暴露面）；`change.yaml` archived；目录 `mv` 至 `archive/2026-…-wec-lexer/`；归档提交。
  来源：proposal 目标 7；B2a/B2b/B3a 归档先例
  验证：`validate.py --all --strict` 归档后期望态；归档复验（17 包 0 FAIL、conformance 933、docs_sync 33 对）
  落地（2026-09-20）：#31（let-别名错编译）随 T4 发现一并登记（三-entry 双语）；归档至 `archive/2026-09-20-wec-lexer/`；复验数字见归档提交信息
