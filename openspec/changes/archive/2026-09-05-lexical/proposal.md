# lexical — 第 1 章词法阶段与 `we check <file>`

## Why

M0（compiler-bootstrap，归档 2026-09-05）交付了命令面骨架：封闭子命令表、全局选项、`we version` / `we new`、E1904/E1907、JSON Lines 事件与一致性测试地基，但整条 check 管线仍是 exit 70 的诚实边界。按 roadmap 薄垂直优先策略，第一个真实管线阶段是词法分析（第 21 章 R2 固定的管线次序：lexical → parsing → …），其规范依据已全部批准：

- 第 1 章（`docs/spec/0100-lexical.md`）全部 Requirement：Source files / Token model / Identifiers / Keywords / Numeric literals / String and rune literals / Operators and punctuation / Comments / Attribute lexical unit；
- 注册表 `docs/spec/diagnostics.toml` E0001–E0009（词法 token 级错误，逐字引用标题与补救文本）；
- 第 21 章 R1（单文件编译：文件路径即整个编译）与 R2（管线次序可观察、E 诊断停止管线）。

可验证的黑盒缺口：`we check demo/main.we` 今天对任何输入都退出 70 并输出一行与文件内容无关的边界消息；没有任何输入能产生 E0001–E0009 中的任何一个。

## 目标与非目标

### 目标

1. `internal/lex` 包实现第 1 章词法：封闭 token 清单、最大吞噬、标识符（ASCII-only）、40 个完全保留关键字、数值/字符串/rune 字面量、三种注释、BOM/CRLF 机械归一化、属性单元（一个词法结构）。
2. 词法阶段产生 E0001–E0009（消息以注册表标题开头，help 取自注册表 remediation 的压缩形式；本变更不新增、不修改任何诊断码）。
3. `we check <file>.we` 单文件模式跑词法阶段：首个词法错误 → 报告 + exit 1；词法干净 → 解析边界一行 + exit 70（诚实边界从「子命令级」收缩到「阶段级」）。
4. `we check <dir|.>` 目录模式 → 项目编译边界一行 + exit 70（manifest 解析属 M3）。
5. internal/diag 人类可读渲染为带位置诊断增加 `file:line:column: ` 前缀（JSON Lines 协议字段与次序不变）。
6. conformance runner 的 Setup 支持 Files（用例可携带来源文件）；新增 E0001–E0009 与干净文件、BOM+CRLF、非 .we 后缀、--json 变体的黄金用例；改写 M0 的两个 check 边界用例（边界表面变化，随本变更记录）。

### 非目标

- **E0011–E0013（命名约定）**：判定需要「标识符 + 绑定形态」，绑定形态在解析阶段才存在 → M2+，不在本变更。
- **解析与后续阶段**（E0101/E0102、模块解析、类型）→ M2/M3。
- **项目模式**（manifest 校验 E1905）→ M3。
- **注册表 go:embed**：维持 M0 的调用点硬编码注册表标题模式（roadmap 已登记待办 2）。
- **多错误批量报告**：规范未固定；本变更取「首个错误停止词法阶段」（design D2），批量留待有证据的后续变更。
- **`///` 文档注释 token 化**：附着规则属第 6 章（M2）；本变更按普通行注释扫描跳过。

## What Changes

- 新增 `internal/lex/`（词法器 + 按章逐条的单测）。
- `internal/cli`：`check` 子命令从通用 70 边界改为分派——E1907（既有，dispatch 前不变）→ 目录模式边界 / `.we` 后缀 usage 错误 / 读文件 / 词法 / 首错误报告或解析边界。
- `internal/diag`：`Human()` 带位置时前置 `file:line:column: `。
- `internal/conformance`：`Setup.Files`；新增与改写黄金用例。
- docs：roadmap M1 状态行由归档本变更的动作翻写为 done。

## 影响层

`compiler`（词法阶段——已批准章节的实现，无规范增量）、`tooling`（`we check` 可观察表面：分派、边界消息、退出码——第 21 章既有行为的部分实现，无规范增量）。**本变更不改变语言行为、无规范增量**：一切接受/拒绝判定以已批准的 `docs/spec/0100-lexical.md` 与注册表为依据；机制自由度（列位置单位、错误报告粒度、消息措辞、属性参数校验次序）在 design.md 固定并作为事实上的稳定表面记录。

## 影响范围

- 新文件：`internal/lex/lex.go`、`internal/lex/lex_test.go`、`internal/cli/check.go`、约 14 个 `internal/conformance/testdata/cases/check-*.json`。
- 修改：`internal/diag/diag.go`（Human 前缀）、`internal/diag/diag_test.go`、`internal/cli/cli.go`（check 标记为 implemented 并分派）、`internal/conformance/runner.go`（Setup.Files）、`check-boundary.json`、`check-boundary-json.json`（改写）、`docs/roadmap/0000-reference-implementation.md` 与 `.zh.md`（M1 行）。
- 不动：`docs/spec/`、`diagnostics.toml`、go.mod、M0 其余 18 个黄金用例。

## 审计记录

**2026-09-04，candidate → ready，welang-spec-impact-audit 七条，通过。**

1. 问题真实性：✅ 黑盒缺口可验证——`we check <file>.we` 对一切输入 exit 70，E0001–E0009 均不可产生；锚点 `docs/spec/0100-lexical.md`、`docs/spec/diagnostics.toml`（E0001–E0009 条目）、`docs/spec/2100-toolchain.md` R1/R2 已在 Why 引用。
2. 影响层声明：✅ change.yaml `layers: [compiler, tooling]` 与 proposal「影响层」一致；proposal 含「不改变语言行为、无规范增量」边界声明（validate 内部变更豁免路径）。
3. 规范增量范围：✅ 零增量、零新码、零改码；E0001–E0009 按注册表逐字引用；无 active 变更冲突（changes/ 下仅本变更）。
4. 原则一致性：✅ 全部接受/拒绝判定有 ch1 条款依据；机制自由度（列单位、首错误停止、E0009/E0008 次序、位置前缀）记录于 design，不触十条原则。
5. 参考基线固定：✅ 引用均为 docs/spec 明确路径；无 refr/ 草案依赖。
6. 验收边界：✅ 目标可机械判定（黄金用例红→绿、退出码、validate/docs_sync）；非目标显式排除 E0011–E0013、解析、项目模式、embed、批量报告、文档注释 token。
7. 粒度：✅ 单一垂直单元——词法阶段经 `we check <file>` 端到端可观察；无跨三层的耦合改动。

## 审查记录

**2026-09-04，ready → active，welang-change-review 十条，通过（发现 F1、F2，均非阻塞）。**

1. proposal：✅ 黑盒缺口与目标；internal 包名属 internals-only 变更的最小工程语境（M0 审查先例一致）。
2. spec 增量：无（纯内部变更豁免路径），N/A。
3. design：✅ 唯一最小路径；D12 六条被拒方案含理由；引用精确到 Requirement 与注册条目。
4. tasks：✅ 每勾选项有来源与验证；无 deferred / non-goal 混入。
5. 场景覆盖：✅ normal（check-clean / check-bom-crlf）、boundary（`0..9` 与 `1.5..2.5` 吞噬、非 `.we` 后缀、插值配平）、failure（E0001–E0009 每码至少一黄金用例）。
6. 无空章节：✅。
7. 测试先行：✅ T2 黄金用例与 T3 位置渲染快照先红，T4–T6 后实现。
8. 负向断言：✅ 每码构造违规样例断言拒绝（exit 1 + 消息），check-clean 作阳性对照。
9. 完成度闭环：✅ 管线阶段实现而非语言特性变更；ch1 自身固定「本章只定义形状」（D3 引用），类型与代码生成闭环由 roadmap M3/M4 承载（M0 同构先例）。
10. 未决问题：✅ 无。

**F1（非阻塞）**：D9.3 读文件失败路径无黄金用例——进程内不可移植模拟权限错误，路径复用 M0 fsError 先例，归实现审查逐行核验。
**F2（非阻塞）**：`1.` 吞点与 `0b12` 整段提交取自注册表 description 的解读，design D3/D12 已论证并 grep 佐证无冲突用例；属事实上的机制表面，未来规范修订可钉死（届时走 spec 层变更）。

## 实现审查记录

**2026-09-05，active → complete，welang-code-review 七条，通过（F1 闭环核验；无新增阻塞项）。**

1. **规范符合性**：✅ ch1 全 Requirement 有实现轨迹且逐场景可观察——token 清单/最大吞噬（单测 0..9、1.5..2.5、0x1F、1_000）、标识符 ASCII-only（E0001 homoglyph 黄金）、41 关键字全保留零软关键字（单测枚举对照 ch1 清单原文块）、数值全条款（E0006 八条款逐一断言）、字符串/rune 转义闭集与插值配平（E0002/E0003/E0005/E0007，含规范原例 `"${f(}"`）、注释三形不嵌套（E0004）、编码 BOM/CRLF/裸CR/非法UTF-8（单测）、属性单元（E0008/E0009 与次序）、退出码 0/1/2/70。**F1 闭环**：读失败路径真实触发核验——chmod 000 的 .we 经真实二进制得 `we: open bad.we: permission denied`、exit 1（fsError 先例路径）；D9 六分支（目录 70 / 非 .we usage 2 / 读失败 1 / 词法首错 1 / 干净 70 / E1907 前置）全部以真实二进制逐分支实测。
2. **验证诚实性**：✅ T1–T7 逐项复跑——`go test ./internal/...` 五包 ok（conformance 35/35、lex 10 函数、diag 含 M0 三快照逐字节不变）；T2/T3 red 证据与完成记录一致（17 例红、位置快照红，日期 2026-09-04）。无「勾了没跑」。
3. **测试先行证据**：✅ 变更级红先行由 T2/T3 承载（实现前红、记录在案）；T5 按章单测为补充覆盖且实效已证——抓出四处实现缺陷当场修复（`=` 漏清单、后缀判定序、插值洞括号保持、消息引号一致性），其中洞内括号保持是规范示例 `"${f(}"` 命中 E0007 的必要机制，已补记 design D4；黄金 check-e0009 属性名撞关键字由实现正确拒绝（修金样不改码，完成记录披露）。黄金双面格式符合 ch21（人类行 `file:line:column: error[code]: title — detail`；JSON 固定字段序）。
4. **诊断协议稳定**：✅ M0 JSON 快照逐字节不变；零新增诊断码（E0001–E0009 皆注册表既有）；helps 表为注册表 remediation 的压缩复述，代码内已标注「registry embed 落地前过渡」（roadmap follow-up 2）。
5. **单一权威**：✅ 关键字表权威在 ch1 清单原文，代码 map 由单测逐词对照钉死；机制裁决留 design（D4 补记）；无长期事实滞留变更目录。新代码注释全英文（CJK 探针：仅测试数据字面量命中）。
6. **红线复核**：✅ refr/ 不在 git status（ignore 生效）；提交将不带署名 trailer；diff 范围 = internal/{lex,cli,diag,conformance} + 本变更目录，与影响层一致，无越界改动。
7. **最小可信验证**：✅ 按实际 diff 选取全阶梯重跑通过——gofmt -l 空、go build、go vet、go test ./internal/...、validate --all --strict（OK: 1 change(s) valid; registry clean）、docs_sync（OK: 30 pairs）、git diff --check 干净。
