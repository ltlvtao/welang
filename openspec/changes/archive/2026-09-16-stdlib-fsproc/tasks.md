# tasks — stdlib-fsproc（B2a）

> 权威：proposal.md（范围）+ design.md（D0–D8）。每任务实现前待用户点名；红先纪律 = 黄金在实现前取红态。

## T1 go:embed 装载与三枚迁移（零漂移）

- [x] **stdlib 根包与嵌入源落盘**：`stdlib/stdlib.go`（`//go:embed src/*.we` + `Sources()`）+ `src/io.we`/`src/test.we`/`src/time.we`（虚构体逐字照抄今日合成 AST 的体，design D1-3 表）；`stdlib_test.go` 门。
  来源：design D1-1、D1-3
  验证：`stdlib_test.go` 逐一断言嵌入源 `parser.Parse` 干净（先红——源未落盘时测试无法成立）；`go build ./...`/`go vet` 绿
- [x] **StdModule 翻转 + concurrent 例外**：`typecheck.go:2469` 策略表（六键嵌入源 parse+缓存 / concurrent 合成构造器）；进程内 parse 缓存（AST 纯数据 + M6b 共享先例）。
  来源：design D1-2、D1-5
  验证：typecheck 单测——六键的 `StdModule` 返回真解析 `*ast.File`、`concurrent` 返回合成且逐字段同旧；重复调用同指针（缓存生效）
- [x] **assertEqual 条目缝定夺**：选 A（`src/test.we` 声明条目 + 拦截仍前置）先行；任何黄金漂移 → 选 B + 披露。
  来源：design D1-4
  验证：19 枚 `test-asserteq-*` + 5 枚 `check-assertequal-*` 逐字节绿（`-count=1` 定向跑 + 全量跑双面）
- [x] **零漂移三缝验证**：全语料 + IR 对拍 + mock 槽。
  来源：design D1-4
  验证：`go test ./internal/conformance -count=1` 881 枚全绿零改写；hello IR 前后字节对拍（`cmp` 零差，M9a m8HelloIR 先例）；既有 mock 黄金绿即槽物化同一

## T2 std.fs

- [x] **fs 源与面落盘**：`stdlib/src/fs.we`（FsError + 七枚虚构体，design D3 源形）；`runtime/c/fs.c`（七 C 入口：NUL 前置拒、`"{op} {path}: {strerror}"` 消息、makeDir 单层 0755、removeDir rmdir、listDir strcmp 序、C 侧 List<String> 构造走 T11-② 盒）；codegen keyed 扩容（declare 表 7 行 + 键 + out 参发射臂 + gc 载荷再扎根）。
  来源：design D3、D2、D7-1、D7-2
  验证：runtime C harness 单测（fs 七入口的 Ok/Err 双路 + NUL 拒 + listDir 序）；codegen 单测（out 参发射形 IR 钉）；先红记录（实现前 fs 黄金族红于 E1302）
- [x] **fs 黄金族落盘**：`check-fs-effect`（E1401）、`run-fs-roundtrip`、`run-fs-read-missing`、`run-fs-listdir-order`、`run-fs-question`（`?` 传播链）、NUL 形（可拼则黄金，否则 C 单测 + 披露）。
  来源：design D8 表
  验证：逐枚先红（E1302/E1401 形）后绿；真机二进制复核 stdout/stderr/exit 三面

## T3 std.process

- [x] **process 源与面落盘**：`stdlib/src/process.we`（ProcessError + ProcessResult record + run 虚构体）；`runtime/c/process.c`（fork/execvp PATH、双管 poll 轮读、waitpid、WIFSIGNALED→128+sig、argv 构造、ProcessResult C 侧构造 map=null）；codegen keyed `__we_proc_run`。
  来源：design D4、D2、D7
  验证：C harness 单测（echo 捕获 / false 退出码 / 缺命令 Err / 信号形 128+sig）；codegen 单测（返回记录句柄的再扎根 IR 钉）
- [x] **process 黄金落盘**：`run-proc-run`（`/bin/echo hi`）、`run-proc-fail`。
  来源：design D8 表
  验证：先红后绿；真机复核（捕获文本含换行逐字节）

## T4 std.string 真体

- [x] **【阻塞前置】define 发射管线开放验证**：真机探针 `import std.string` + join 调用走 check/build/run 三面——过则直行；停则定位（≤ 既有管线参数化 → 修复 + 披露；否则回退 keyed C 助手 + 真体改虚构体 + 披露）。未定不得勾其后各项。
  来源：design D5-3
  验证：探针输出记档（三面退出码 + IR 样本）；结论写回本行（直行/修复/回退 三选一）
  **结论：修复**——check 0 / build 70 / run 未达；停点二分至 import 走查 default 分支（非 emitCall）；修复 = build.go programModules 放行 std.string + import 走查 `case "string"` 落 curImports（两处参数化，emitCall 零改动）；修后 build 0 / run 0（`a-b-c`）。记档：design 补记四
- [x] **string 源与发射落盘**：`stdlib/src/string.we`（join/repeat 真体，design D5 源形逐字）；发射按 D5-3 结论（define 槽 `@std.string.join`/`@std.string.repeat` 或 keyed C 助手）。
  来源：design D5
  验证：真机 join/repeat 三面绿；发射形单测（define + 槽 load，或 declare 形）
- [x] **string 黄金落盘**：`run-string-join`（空表/单元素/多元素三段）、`run-string-repeat`（0 次/n 次）。
  来源：design D8 表
  验证：先红（红于装载未含 string 或发射停点按 D5-3 结论）后绿

## T5 E0816 收口

- [x] **两产出点翻转 + 词退役**：`:6968`/`:7022` → E0816（消息以注册表 title 起头 + detail 点名成员/接收者型，helps 引 remediation）；`:51` `bndStdModules` 常量删除。
  来源：design D6
  验证：翻转前 corpus grep 边界行恰 2 枚记档；翻转后 0 枚 + 全语料 `-count=1` 绿；裸 `xs.count()` 探针 exit 1 + E0816（翻转前 70 记档）
  **已记档：翻转前 2 枚（check-bnd-std-member、check-stdio-shadow-let）+ 裸 `xs.count()` exit 70；翻转后 0 枚 + 891 全绿 + 裸形 `"count" is not a member of List<Int64>` exit 1；as-built 行号 :6956/:7010/:54；三枚单测（m5/m6a/m8）随翻重锚——设计勘查未数的第三处**
- [x] **2 枚黄金重锚**：`check-bnd-std-member`、`check-stdio-shadow-let`——保名保源，期望改 exit 1 + E0816 行。
  来源：design D6、裁决 4
  验证：重锚后 `-count=1` 绿；E0816 消息与注册表 title 逐字比对（一码一消息纪律）
- [x] **相邻面守界复核**：std.bogus 仍 E1302 std 形；接口/Dyn/记录成员解析照旧；`xs.iterator().count()` 链形照旧绿。
  来源：design D6「不翻的相邻面」
  验证：三探针真机记档（探针项目 /tmp/b2-probe 复用）

## T6 电池与验证阶梯

- [x] **mock 面黄金**：`test-fs-mock`（`mock fs.readFile` 桩值往返）。
  **已落地：mockTarget outTrio 旗标 + fsEntries 解析支 + 双 define（体 `.body` + mockOutWrapper）；严格红证据 = 同源 × 父树二进制 exit 70；单测钉 TestFsMockRidesTheOutTrio（八钉）**
  来源：design D2 mock 面、D8 表
  验证：先红后绿；mock 拦截与恢复的 stdout 逐字节
- [x] **突变电池**：D8 表 (a)–(f) 六枚——每枚锚点 `count==1` 断言、单测层 + 黄金层双判决、逐枚还原（还原后全绿再下一枚）。
  **已落地：六枚判决表入 design 补记六（(a)–(e) 双层皆死、(f) 黄金层独杀——单测 IR 形状钉不问 sep 值的诚实记录）；伴生 run-fs-nul 强化 + run-proc-signal 新增**
  来源：design D8 突变电池
  验证：六枚判决表入完成记录（死/存活 + 层 + 机理）；结束后 `go test -count=1 ./...` 全绿
- [x] **验证阶梯全量**：AGENTS.md 阶梯——`go build`/`go vet`/`gofmt -l`/`go test -count=1 ./...`（as-built 16 包——T1 落地记已订正，任务书起草数为 15；conformance 总数记档）/`validate.py --all --strict`/`docs_sync.py`（33 对）/`git diff --check`/staged 无 `refr/`；`WE_UPDATE_GOLDEN=1` 未用（黄金全手写）。
  来源：AGENTS.md 验证阶梯、B1b T15 先例
  验证：八面输出记档；conformance 新总数（881 + 新增 − 0 改写 + 2 重锚原位）与 tasks 各任务增量对账
  **已记档：build/vet/gofmt(0)/test(16 包 0 FAIL)/validate strict/docs_sync 33 对/diff-check 净/无 refr；conformance 893 = 881 + T2 六 + T3 两 + T4 两 − 0 改写；codegen 单测 461**

## T7 实现审查 + 归档

- [x] **welang-code-review 七条**：读 `.agents/skills/welang-code-review/SKILL.md` 手动执行；审查记录入 proposal.md。
  **已落地：七条全过（2026-09-16）；审查期零码改——F1 措辞级（T6 行「15 包」为起草旧数）随审订正；抽查复验 = hello IR 双二进制 cmp 逐字节零差（两槽俱在）+ bndStdModules/corpus 边界行双 grep 归零；发现与处置段入 proposal「## 审查记录（实现审查）」**
  来源：里程碑惯例（B1b T15 先例）
  验证：七条结论表 + 发现与处置段入档；审查期码改（如有）独立提交
- [x] **归档**：`mv` 至 `openspec/changes/archive/2026-09-16-stdlib-fsproc/`（目录未入册故文件系统 mv；**归档日 as-built 2026-09-16——任务书起草时预测 09-15，审查记录 as-built 预告段已注明**）、`change.yaml` → archived、roadmap B2 行拆 B2a done / B2b pending（双语）、follow-up 划账（无新账——B2a 零 follow-up 登记，registry 止于 #25 皆 B1/M 轨）、记忆回写。
  来源：里程碑惯例（B1b 归档先例）
  验证：归档后 `validate.py --all --strict` → 「no changes found + registry clean」期望态；全阶梯复跑绿；提交与推送待用户明示
