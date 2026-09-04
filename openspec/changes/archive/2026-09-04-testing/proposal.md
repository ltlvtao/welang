# Proposal: 测试系统（第 20 章）

## Why

宿主规范早已为本章预留了全部接口点，逐条可 grep 验证：

1. **第 1 章关键字段的预告**（`docs/spec/0100-lexical.md` Keywords）："feature-specific words (effects, types, concurrency, **testing**) join the list together with the chapter that ratifies them" —— 测试词自基础章起就在入列承诺里，本章兑付它。
2. **第 18 章四条件准则**（`docs/spec/1800-concurrency.md`，The implicit-acquisition criterion）：`currentCancelSignal()` 是本语言唯一隐式获取内建，后续章节批准另一个"only by a spec-layer change that shows the candidate against all four conditions"。`advanceTime` 是第二个候选，必须对号入座。
3. **第 18 章调度承诺的反面**（Scheduling promises）："the order of task execution, the interleaving of concurrent tasks ... are all unspecified — the runtime's own, **no chapter promising a bound**"。测试模式要求确定性调度，须以显式 carve-out 修订此句，而非默示缩小。
4. **第 14 章捕获边界的计数**（Unwinding）："the abort is the capture boundary ... the task boundary is the **second** capture boundary this specification defines — no expression of the language observes a panic"。测试失败需要第三个捕获边界（panic 在 test 边界转为用例失败、进程不中止），两个先行章的措辞都要如实修订。
5. **第 12 章函数体语境清单**（Closure bodies are function bodies）："chapter 6's named fns, this chapter's closures, and — by the concurrency chapter's amendment — that chapter's task bodies; the module top level and defer bodies remain returnless" —— test 体与 mock 体加入此清单。
6. **第 15 章预导入闭集 + currentCancelSignal 先例**（The prelude）：闭集加名是本章程修订；预导入携名、归属章定其合法位置（`E1608` 先例）—— `advanceTime` 完全镜像此模式。
7. **第 16 章调用位检查的语境锚**（The effect segment）："**Within a function body** ... every call's effect set MUST be a subset of the declared set"。test 块无段可写，规则需要测试句来划界。
8. **注册表的预告**（`docs/spec/diagnostics.toml` E1406 注释）："E1406-E1499 reserved for amendments (**test-virtualization-era codes**; ...)" —— 段注一直在等测试码落段。
9. **第 0 章生成–检查–修复循环**：测试是循环中"检查"一腿的语言面落点；mock 拦截按名字解析（P1 局部可判定）、不虚拟的边界如实陈述（P8 诚实边界）。

v0.8 基线（`refr/spec-0.8.md` §38–41、§60）给了完整参照系：test 块、test_each、property、mock、时间虚拟化与 `advanceTime`、确定性调度、`--explore` 与 W0601/E1001–E1003、`we test` CLI。本变更按四项已裁决取舍将其映射到 We 的已批表面。

## What Changes

1. **新章：第 20 章 Testing**（`docs/spec/2000-testing.md` + `.zh.md`），九条 Requirements：
   - 测试模块与 test 块 —— `*_test.we` 命名事实、`test "desc" { }` 顶层项、仅测试模块合法（`E1801`）；
   - mock 声明 —— 仅 test 块直接项（`E1802`）、签名+效果段与目标逐字一致（`E1803`）、目标须为模块级单态 fn（`E1804`）、一 test 块一目标一 mock（`E1805`）、按名字拦截不递归代理；
   - 虚拟钟 —— test 块内 `time` 标签调用全走虚拟钟，`io`/`net` 不虚拟，scope `timeout` 读虚拟钟；
   - `advanceTime` —— 预导入名、仅 test 体合法（`E1806`）、对第 18 章四条件逐条作答；
   - 测试模式确定性调度 —— 同源同钟同一交错，可观察确定性，策略归运行时（ch18 carve-out）；
   - test 边界的 panic —— 第三捕获边界，assert 失败即 panic 即用例失败；
   - 断言族与 stdlib 测试表面 —— `assert` 已在预导入；`assertEqual` 族 = `std.test` 普通库表面；参数化/性质测试 = 组合子，零新语法；
   - 测试不固定什么 —— 跨块执行序、并行、报告、退出码、`tests/` 布局、未 mock 自定义效果的真实性，均如实指名归工具链；
   - 测试诊断段位 —— `E1800`–`E1899` owner `2000-testing`。
2. **两个新关键字 `test`、`mock`**（第 1 章破坏性入列记录，沿 `task` 等添加式先例 +1 保留场景）。
3. **七处宿主修订**（×2 语言，每处修订配证据场景，合计 +7 场景）：
   - 第 1 章 Keywords（词表 + 破坏性句 + 1 新场景）；
   - 第 6 章 File structure and module identity（顶层项枚举 + test 块句 + 1 新场景）；
   - 第 12 章 Closure bodies are function bodies（语境清单句 + 1 新场景）；
   - 第 14 章 Unwinding（第三捕获边界指针句 + 1 新场景）；
   - 第 15 章 The prelude（闭集 + `advanceTime` + 1 新场景，镜像 `currentCancelSignal`/`E1608` 先例）；
   - 第 16 章 The effect segment（test 体无段不查 `E1401`、mock 体按段查的测试句 + 1 新场景）；
   - 第 18 章 ×2：Scheduling promises（测试模式 carve-out 句 + 1 新场景）、Panics at the task boundary（第三边界指针句，0 新场景——证据由 ch14 镜像承载）。
4. **注册表**：段位 `E1800`–`E1899` owner `2000-testing`（`E1900`–`E9999` 仍 unclaimed）；新码 `E1801`–`E1806` 六条；E1406 段注再刷新（测试码已落 E18xx）；条目 131→137、段 19→20。
5. **v0.8 映射**（详见 design.md D12）：§38 test→本章；§39 test_each→for 循环消亡；§40 property→stdlib 组合子消亡；§41.1 mock→收窄为模块级单态 fn；§41.2 虚拟钟→本章；§41.3 确定性调度→本章；§41.4 `--explore` 及迭代控制→工具链章；§41.5 W0601→工具链层；§41.6 E1001/E1003→工具链层、E1002 消亡（第 16 章调用位检查已治其形）；§60 `we test` CLI、`tests/` 目录、退出码→工具链章，`*_test.we` 命名→本章（模块同一性事实）。
6. **双语文档**：`docs_sync` 26→27 对。

## 影响层

- **spec 层**：新章 20 + 七宿主章修订 + 注册表，全部经 openspec 变更流程（本变更）。
- **工具链层**（不在本变更）：测试 CLI、`tests/` 目录布局与源根映射、执行序、并行、报告格式、退出码、`--explore` 及其诊断（v0.8 W0601/E1001/E1003 的宿居）。
- **stdlib 层**（不在本变更）：`std.test` 断言族与参数化/性质组合子 —— 本章只批准"库表面、非文法"这一立场，函数清单是 stdlib 发布的事。

## 影响范围

| 文件 | 动作 |
| --- | --- |
| `docs/spec/2000-testing.md` | 新建（第 20 章正文） |
| `docs/spec/2000-testing.zh.md` | 新建（中文镜像） |
| `docs/spec/diagnostics.toml` | 段位拆分 + 6 新码 + E1406 注刷新 |
| `docs/spec/0100-lexical.md` / `.zh.md` | MODIFIED：Keywords |
| `docs/spec/0600-declarations.md` / `.zh.md` | MODIFIED：File structure and module identity |
| `docs/spec/1200-fn-types.md` / `.zh.md` | MODIFIED：Closure bodies are function bodies |
| `docs/spec/1400-errors.md` / `.zh.md` | MODIFIED：Unwinding |
| `docs/spec/1500-modules.md` / `.zh.md` | MODIFIED：The prelude |
| `docs/spec/1600-effects.md` / `.zh.md` | MODIFIED：The effect segment |
| `docs/spec/1800-concurrency.md` / `.zh.md` | MODIFIED：Scheduling promises、Panics at the task boundary |

## 裁决记录

用户裁决四项（2026-09-04，均采推荐）：

1. **test + mock 两关键字** —— 顶层 `test "desc" { }` 块与 `mock name(sig) { }` 声明是仅有的新语法；参数化 = for 循环/辅助函数，性质测试 = stdlib 组合子闭包（P2 最小语法）。
2. **mock = 模块级单态 fn** —— 本模块裸名 + 导入 `pub` 合格名（`mod.fn`）；mock 声明携原签名含效果段，mock 体检查 ⊆ 该段；泛型函数、方法（impl/interface）、构造器不可 mock（专用拒绝码）。
3. **虚拟钟+确定性调度全套** —— test 块内 time 标签调用全走虚拟钟；`advanceTime(d)` 预导入名仅 test 块内合法（对 ch18 R9 四条件作答）；调度器测试模式确定性经 ch18 调度承诺 carve-out 句；scope timeout 纳入虚拟钟。
4. **探索模式归工具链章** —— `--explore`/偏序缩减/迭代控制/W0601/E1001/E1003 全部工具链（slice 6）；spec 只固定可观察确定性语义；未 mock 的自定义效果在测试中是真实调用——规范如实陈述。

## 目标与非目标

### 目标

- 测试的语法面最小：两个关键字，零新表达式、零新类型、零新文法类别。
- 每一处与宿主章的接触点都以显式修订句落定，不留默示语义。
- 确定性是可观察承诺（同源同钟同交错），调度策略保持运行时自由。
- 不诚实的东西不写：io/net 不虚拟、未 mock 效果是真调用、跨块执行序不固定。

### 非目标

- `--explore`、`--iterations`、`--reduce` 及 W0601/E1001/E1003 —— 工具链章（slice 6）。
- 测试 CLI（`we test`）、`tests/` 目录布局、源根映射、并行执行、报告格式、退出码 —— 工具链章。
- `std.test` 具体函数清单与性质测试组合子实现 —— stdlib 发布。
- mock 泛型函数/方法/构造器 —— 本变更以 `E1804` 关死，未来修订另行设计。
- 基准测试（benchmark）—— v0.8 范围列表之外，不引入。

## 审计记录

2026-09-04，七点审计（welang-spec-impact-audit），结论：**通过**。

1. **问题真实性 ✓**：Why 的九处锚点逐一 grep 实证——ch1 "(effects, types, concurrency, testing)" 预告句（1 处）、ch18 四条件准则 "answers all four conditions against this requirement's text"（1 处）、ch18 调度承诺 "no chapter promising a bound"（2 处：正文+场景）、ch14 "the second capture boundary"、ch18 "the one this specification ratifies besides the process abort"、ch12 函数体语境清单句、ch15 预导入闭集句、ch16 "Within a function body ... subset of the declared set"、注册表 E1406 注 "test-virtualization-era codes"（1 处）。基线固定为 `refr/spec-0.8.md` §38–41、§60。
2. **影响层 ✓**：change.yaml `layers: [spec]` 与 影响层 一致；工具链/stdlib 的边界在 影响层 与 非目标 双处写明（探索模式、CLI、std.test 清单）。
3. **规范增量范围 ✓**：新增 9 Requirement（第 20 章）+ 修改 8 Requirement 块（ch1/ch6/ch12/ch14/ch15/ch16/ch18×2）；新码 E1801–E1806 六条，grep 证实 `E18xx` 在 docs/spec 正文零占用、段位 `E1800-E9999` 现为 unclaimed，无冲突无重复；宿主 +7 场景、新章 33 场景。
4. **原则一致性 ✓**：P1——mock 拦截按名、六个 E 码全静态可判；P2——test_each/property 语法消亡（糖不开章，D10/D12 论证）；P5——advanceTime 依 ch18 准则四条件逐条作答（D7）；P8——io/net 不虚拟、未 mock 自定义效果是真调用、探索的有界性如实声明（R3/R8）；P9——断言失败=panic=test 边界失败一条路（R6）。**效果豁免（test 体不查 E1401）为唯一刻意留白**，论证在 design.md D4：豁免的是"上界检查"（不存在可查的上界声明），类型/资源/所有权检查完整，task/scope 内纪律不变，mock 体照查——非静默、非无限。
5. **参考基线固定 ✓**：v0.8 引用固定为 `refr/spec-0.8.md` §38–41、§60；映射账在 design.md D12 逐条可追。
6. **验收边界 ✓**：目标四条可机械判定（两关键字、七处显式修订句、可观察确定性承诺、三处诚实陈述）；非目标排除探索/CLI/stdlib 清单/benchmark/mock 扩类，蔓延面已封。
7. **粒度 ✓**：单一 spec 层变更，一章 + 宿主句，垂直可验收，与 effects/concurrency/ffi 变更同形。

## 审查记录

2026-09-04，10 点语义审查（welang-change-review），结论：**通过（3 发现，均已修复回灌增量）**。

1. **proposal ✓**：黑盒问题+目标；实现决策在 design（D1–D14），任务在 tasks，无混杂。
2. **spec 增量 ✓**：全部为可观察行为（触发/拒绝/语义承诺）；MUST/MAY 合 BCP 14；无实现方式泄漏。修复 F1：ch20 R3 场景与示例的 scope 拼写误写 `timeout 100`，ch18 权威拼写是 `scope timeout(expr)`——两处改为 `timeout(100)`。
3. **design ✓**：唯一最小路径；被拒替代方案有录（D5.2 mock 参数改名、D10 test_each/property 竖语法、D12 E1002 第二码）；引用精确到 Requirement 与码。修复 F3：补 D14 三要素账（类型检查六码全静态/代码生成零新面/运行时钟+调度+边界三事）——原则 10 闭环。
4. **tasks ✓**：勾选项均有来源与验证；哨兵 E1899 注入为负向验证；无 deferred/未决方案。
5. **场景覆盖 ✓**：normal/boundary/failure 三路齐——修复 F2：R2 原缺「mock 力量止于其块」边界场景（第二个 test 块调用目标时命中真函数），已补（33→34 场景）。
6. **无空章节 ✓**：九 Requirements 均有行为增量；宿主七处均为实质修订句。
7. **测试先行 ✓**：spec 层变更的等价物 = 哨兵注入负例（E1899 → `--all --strict` FAIL → 还原）+ 逐阶段 validate，任务 4.1 已列。
8. **负向断言 ✓**：E1801–E1806 六码各有拒绝场景；六码拒绝样例均在 delta 与示例中；注册表哨兵任务在列。
9. **完成度闭环 ✓**：D14 三要素账补齐（原缺，即 F3）。
10. **未决问题阻塞 ✓**：无行为级未决——探索/CLI/stdlib 清单均为显式非目标且有所归（工具链章/stdlib 发布），非未决。

## 实现审查记录

2026-09-04，七点代码审查（welang-code-review），结论：**通过（0 阻断；2 项披露）**。

1. **规范符合性 ✓**：delta 九个 Requirement 块逐字包含于 `docs/spec/2000-testing.md`（机检 9/9）；宿主八块拼接逐字校验通过；七宿主章场景计数较 HEAD 恰 +1（EN/zh 各一）且 Requirement 计数不变（机检 7 章 ×2 语言）；zh 孪生 R/S = 9/34 与 EN 镜像、全角冒号诊断形扫描 0、代码块 6/6 字节一致（delta 示例 == EN 章 == zh 章）。
2. **验证诚实性 ✓**：tasks 第 1–4 节 12 项逐一有真实运行证据——`validate testing --strict`、`validate --all --strict`、`docs_sync --check`（27 对）、注册表 tomllib 机检（137 条目/20 段、E1801–E1806 owner `2000-testing`）、场景计数 HEAD 比对、一码一消息扫描（新内容 60 对 0 失配；宿主非题注释行与 HEAD 字节一致，系既有内容非本变更引入）。
3. **测试先行证据 ✓**：spec 层等价物已先行——哨兵 E1899 注入 ch18 实测 `--all --strict` FAIL（"diagnostic usage ... has no registry entry"）后还原复绿；§52/§62 黄金用例对纯 spec 变更不适用（仓内无编译器产物，与 effects/concurrency/ffi 切片同判）。
4. **诊断协议稳定 ✓**：`--json` 协议未触碰（工具零改动）；六码全局唯一、段位 E1800–E1899 owner `2000-testing` 独占；E18xx 出现面机检 = 第 20 章双胞胎 + ch6/ch15 宿主场景的指名引用（沿 E1608 先例）。
5. **单一权威 ✓**：长期事实已全部落 `docs/spec/`（章正文 + 宿主句 + 注册表）；变更目录只余增量与流程记录；EN 规范文件 Terminology 节外零 CJK（术语表双语系房屋先例，20 章一致）。
6. **红线复核 ✓**：`refr/` 工作树零改动；提交信息英文、无署名 trailer；diff 面与影响范围表一致（17 文件），无越界改动。
7. **最小可信验证已跑 ✓**：`validate testing --strict`、`validate --all --strict`、`docs_sync --check`、上述全部机检脚本，最终态全绿。

**披露**：

- **D-1（沿留）**：`docs/spec/1200-fn-types.md` 在 HEAD 即存在 H1 后三连换行（先在缺陷，拼接断言发现、写入前拦截）；超出本变更范围，未触碰，留待独立清理。
- **D-2（实现期修复）**：示例中 E1805 注释消息原折行两行（"duplicate mock of one target" / "in a test block"），违一码一消息首行完整纪律；已在 delta 示例 + EN/zh 章三处同步改为单行完整消息，代码块字节一致性复验通过。
