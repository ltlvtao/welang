# 变更提案：FFI——`foreign "c"`（第 19 章）

## Why

规范对 FFI 的承诺已四处悬置，本章兑现：

- ch0（0000-principles.md）原则 8：跨语言边界的机制 MUST 与普通代码受同程度静态检查——「它是原生代码」MUST NOT 豁免类型/效果/所有权检查。
- ch0 宿主与编译目标策略：**外部函数接口为 `foreign "c"`：C ABI 上受控的互操作边界，受完整类型、效果、所有权检查（原则 8）**——接口拼写已钉、机制未批。
- ch1 关键字清单：`foreign` 自初始 21 词即预留，「其语法被或必然被某章批准」——至今无章批准。
- ch6（0600-declarations.md）fn 声明末句：`mut` 参数、foreign 声明由其归属章节批准——foreign 侧悬句在候。
- 注册表 E1400–E1499 段注释：E1406–E1499 预留注明「FFI 效果声明」——效果章预铺的 FFI 码位。
- ADR-0002 决策 4：FFI 为 `foreign "c"`，接受完整检查；Go 生态互操作经 C 桥另作决策——方向已锁、载体未落。

服务端语言（ch0 目标域）绕不开 libc 与既有 C 生态（系统调用、TLS、数据库驱动内核）；没有受控边界，生态只能靠运行时私设通道——违 P8。本片补齐机制本体。

**v0.8 基线**：§47 `foreign "go"` 声明块 + §47.1 回调闭包边界（E0740–E0744）。方向性参考不逐字继承：其 `ownership gc|value|resource` 前缀行与本规范 ch8 类型级类别声明根本不同构；其 Go 后端已被 ADR-0002 明确否决（`runtime.We*` 映射表降级为反例）。

## What Changes

**新增第 19 章 `docs/spec/1900-ffi.md`（+zh）**：

1. **foreign 块**：`foreign "c" { items }` 顶层项；字符串恰 `"c"`（他串 E1701，v0.8 后端枚举不继承——ADR-0002 唯一载体）；块外顶层位 E1702；块内条目（函数声明+不透明类型）入模块单一名字空间、可 `pub`。
2. **foreign 函数声明**：ch6 签名形无体——`fn name(params) -> type effect tags`；效果段 REQUIRED（E1703，E0740 对位）：无体可查、声明即唯一证据；裸 `effect`（零标签）= 显式纯声明——仅此处合法的段拼写（ch16 主体「一或多标签」不变）；泛型子句拒绝（E1704）——边界单态；名之值位不批（E0105，方法值先例）——回调缺口之 We 侧同面，包装闭包是路线。
3. **不透明类型**：块内零字段类别化 record——`record Socket` / `byval record Socket` / `byres record File`（ch8 零字段 record 已合法，类别语法完全同构，零新产生式）；值仅经跨界进入（构造/更新表达式拒绝 E1707——零字段的「全字段恰一次」空真必须关死）；byres 者配 impl Releasable（ch13 E1101 声明完备性既有义务自然继承，impl 体内可调 foreign 函数、普通受检代码）；字段访问 E0604 复用。
4. **跨界类型封闭集**：8 整型/Float32/Float64/Bool/Rune + String/Bytes（**仅参数位**，E1706 禁返回位——无谁拥有原生分配的返回约定）+ 不透明类型 + Never（仅返回位，「不返回」之声明、本章明示的唯一信任点）；其余一律 E1705——record/sum/元组/List/Map/Set/Option/Result/闭包与 fn 型/Dyn/泛型参数皆不跨界。
5. **String/Bytes 约定**：调用期借用（String 只读借用、Bytes (ptr,len) 缓冲对）；原生侧记忆体行为是原生侧契约——规范如实陈述其不可查（P8 落地：We 侧检查完整、原生侧陈述边界）。
6. **边界效果**：声明段即调用效果全集——调用位 E1401 既有裁决原样适用；自定义标签合法；裸 `effect` 纯声明调用方零欠。
7. **边界资源**：所有权以类型跨界、零新规则——byres 参数 = ch13 实参通道；byres 返回 = 义务在绑定处附着、scope resource 接管；三通道/单触发/组合位禁令不变。
8. **不检查之余**：回调延后（fn 型不在跨界集内、E1705 已盖签名位；fn 名转 C 函数指针的效果归属无已批答案——缺口登记，独立变更再入）；链接/符号解析/名修饰/调用约定变体归工具链章（机制中立纪律）；Go 互操作经 C 桥（ADR-0002 既有决策）。
9. **诊断段位** E1700–E1799（E1701–E1707，7 码）。

**宿主修订（2 章 ×2 语言，3 处 Requirement + 1 处 D14）**：ch1 Keywords +`foreign` 语法兑付句（预留先于使用，`mut` 句式先例）；ch6 File structure 顶层项枚举 + foreign 块（+1 场景）；ch6 Function declarations 末句刷新（foreign 落定、`mut` 参数仍悬）；ch6 示例注释 D14 刷新。

**注册表**：拆段 E1700–E1799 owner 1900-ffi + E1800–E9999 unclaimed；E1701–E1707 条目；E1406–E1499 段注释刷新（FFI 码落 E17xx，注记同步）。

## 影响层

spec。

## 影响范围

`docs/spec/` 第 1、6、19 章 + `diagnostics.toml` + 对应 zh 孪生（新增 19；1/6 修订）；`docs_sync` 对数 25→26；不触碰 chapters 之外任何工件。

## 裁决记录

1. **声明形式 = 复用签名+效果段必写**（用户裁决，采推荐）：foreign 块内 `fn open(p: String) -> Handle effect io`——ch6 签名形+ch16 段拼写，无体；段 REQUIRED（E1703）。否决 v0.8 前缀行（本规范效果段已有既定位置，第二种拼写违 P5）；否决段可选（无体之下「省略=纯」是无证据的静默信任，边界处显式优于隐式）。
2. **跨界类型封闭集 = 标量+String/Bytes+不透明句柄**（用户裁决，采推荐）：String/Bytes 仅参数位。否决宽映射（C ABI 组合布局/别名规则与精确 GC 相容故事未立）；否决极小集（无字符串的 FFI 实用性折损大于其诚实收益，借用约定已如实陈述）。
3. **不透明类型 = 无字段类别化 record**（用户裁决，采推荐）：`record Socket`/`byval record Socket`/`byres record File`——ch8 零字段合法+类别前缀同构，零新产生式。否决 v0.8 `type Name: category`（与 ch9 sum 语法冲突、类别三值与本规范四类别不同构）；否决新关键字（无必要破坏性支出）。
4. **回调 v1 不批准**（用户裁决，采推荐）：fn 型不在跨界集（E1705 盖签名）；fn 名转指针的效果归属（回调体稍后自原生帧运行、效果计入谁）无已批答案——缺口登记，待完整设计经独立变更进入。

## 目标与非目标

目标：
- 兑现全部悬句：ch0 P8+策略句、ch1 `foreign` 预留、ch6:43 悬句、E1406 段注、ADR-0002 决策 4 载体；
- 块形/声明形/不透明形全场景钉住（拼写、位置、名字空间、pub）；
- 跨界封闭集与 String/Bytes 借用约定如实固定（可判定、可核对）；
- 效果段必写+E1401 调用位复用、资源三通道复用——零重复规则；
- E1700–E1799 段位与 E1701–E1707 七码入注册表。

非目标：
- 回调（fn 名→C 函数指针）——裁决 4 缺口登记，独立变更；
- 链接、符号解析、名修饰、库搜索、调用约定变体——工具链章（机制中立）；
- Go 生态直连——经 C 桥，ADR-0002 另议；
- 宽类型映射（record→struct、sum→tagged union）——永久按裁决 2 关闭为「修订本章政策是唯一通路」；
- `mut` 参数——ch6 悬句另一半，非本片地界；
- 内联原生代码（asm 块之类）——无提案、无承诺。

## 审计记录

**2026-09-04：通过。** 7 点逐项：(1) 问题真实性——Why 六处宿主锚点实读核对（ch0:119 P8 跨界机制同受静态检查句、ch0:160 ``foreign "c"`` 策略句、ch0:123/170 违例场景、ch1:52 初始清单 `foreign`、ch6:43 fn 声明末句悬句、注册表 E1406–E1499 段注「FFI effect declarations」、ADR-0002 决策 4），全部 grep 可验；目标域缺口（服务端语言绕不开 libc 与 C 生态，无受控边界则生态靠运行时私设通道——违 P8）黑盒可验证。(2) 影响层 `spec` 与 change.yaml `layers: [spec]` 一致，纯规范变更无编译器/工具面。(3) 增量范围——ADDED 第 19 章 9R/34S + MODIFIED 三宿主 Requirement（ch1 Keywords 仅兑付句零新场景 8S 继承、ch6 File structure 枚举+1 场景 4S、ch6 Function declarations 末句刷新 5S 继承）；新码 E1701–E1707 对照注册表 124 条全空闲（E17xx 唯一出现 = 段头 `E1700-E9999` unclaimed 行与并发归档记录，无活跃变更占用）；段内 E1700/E1708–E1799 预留边界提及（E1600 先例）；乘码 E0404/E0105/E1401 冒号码形与注册表 title 机器比对零失配，E0501/E0604/E0811/E1101/E1106/E1304 括号码形语义一致。(4) 原则一致——P1（跨界封闭集=签名位置检查、段必写=产生式固定拼写、不透明语义=声明位置事实+封闭集上的检查，全部局部可判定，design P1 复核逐条）；P5（不透明形复用 ch8 零字段产生式、资源义务复用 ch13、调用位复用 ch16，ch8/ch13/ch16 零修订）；P8（We 侧检查完整、原生侧记忆体行为如实陈述为原生契约，Never 信任点正文披露）；无原则例外、无需 ADR。裸 `effect` 零标签拼写不是 ch16 修订——foreign 函数声明是本章自有产生式，ch16「一或多标签」主体不动。(5) 基线固定 refr/spec-0.8.md §47/§47.1 + ADR-0002（双语），方向性参考不逐字继承（前缀行/Go 载体/E0744 回调方案偏离均在 design D11 披露）。(6) 验收边界——目标可 grep（ch6:237 悬句归零、9R/34S、注册表 131 条/19 段、docs_sync 26 对），非目标六项排除明确（回调/链接/Go 直连/宽映射永久关闭/mut 参数/asm）。(7) 粒度——不透明类型离开块无 declares 位置、E1707 离开不透明类型无对象、跨界集离开声明无载体，同一垂直故事不可拆（collections 先例）。

## 审查记录

**2026-09-04：通过（修复 4 处发现后）。** 10 点审查发现并已在激活前修复：**F1（真缺陷）跨界集漏进口 opaque 类型的到达语义**——原文「the opaque foreign types」未说 import 模块的不透明类型可否跨界，条目级 `pub` 将在跨模块处失效；修复：R4 正文改「the opaque foreign types, the module's own or another module's reached under chapter 15's pub rules」+ 新场景「An imported opaque type crosses」（场景拼写并修正为条目级 pub——`pub foreign` 块本身非已批形式）。**F2（真缺陷）foreign 名值位行为未定义**——ch12:40 值域明文「A monomorphic declared fn — a top-level fn of chapter 6」，foreign 名文本上不在其中，`let f: fn(Int64) -> Int64 = abs` 无已批答案；修复：R2 增句「call form 是 foreign 名唯一已批用法，值位 E0105（方法值先例）——名为码指针即回调问题之 We 侧同面」+ 场景「A foreign name is not a value」+ R8 延后改两面表述（签名位 E1705 盖、值位 E0105 盖）+ examples 拒绝例 + design D9 扩写（包装闭包 `|x| abs(x)` 覆盖全部 We 侧用途，零实用性损失）。**F3（钉边界）foreign 块条目集的拒绝路径无场景**——「nothing else」仅陈述未钉；修复：+场景「A foreign block holds only declarations」（`let x = 1`、`effect gpio` 入块内 → E0105，绑定与效果声明各守顶层形）。**F4（措辞精度）R4 载重义务缺 MUST + Never 参数位含混**——修复：「MUST hold exactly the crossing set」插入；「Any type outside the set — `Never` in a parameter position included —」钉死位敏感集成员。场景 34→37。职责边界（proposal 黑盒、delta 无实现泄漏——E1702 专用码即 D3 之论证、design 唯一路径含 D9 两面记录、tasks 来源验证齐无 deferred）与内容质量（三路齐——7 码各至少一触发场景、空块合法、错串/块外/无段/泛型/组合/回调/String 返回/构造/更新/值位/异项入块各负例在列、无空章节、注入哨兵 E1799 在列、纯 spec 层无三要素义务披露——实现未启动，原则 10 义务属编译器实现变更、无未决问题阻塞——回调缺口是已披露非目标、注册三问题即其门）其余各点通过。

## 实现审查记录

**2026-09-04：通过。** 7 点逐项：

1. **规范符合性**：提升文与 delta 机器逐字验证——新章 9R/37S 全部要求块 `in chapter` 逐字（9/9 True，含审查后新增三场景）；宿主 3 处 Requirement（ch1 Keywords、ch6 File structure and module identity、ch6 Function declarations）拼接后 delta 块逐字回查 3/3 True；宿主场景计数对 git HEAD 机器比对——ch1 两语言 8→8（零新场景、纯兑付句）、ch6 File structure 两语言 3→4（恰为既定 +1），零意外丢失；ch19 R9 所述段位 E1700–E1799 与注册表 `[segments]` 一致，7 条目 requirement 字段全部解析到章内标题（validate owner-file 检查绿即证）。
2. **验证诚实性**：tasks.md §4 七项勾选与本窗口实跑一一对应——哨兵注入（E1799 注入 ch9 → `--all --strict` FAIL「no registry entry」→ 还原 → 绿）先于条目落地；注册表拆段+7 条目后 tomllib 131 条/19 段；逐字 diff、场景计数、docs_sync 26 对、一码一消息 800 行 0 失配（反引号码形+注释码形、消息完整在首行）、全角冒号扫描 0、代码块 EN↔zh 字节一致 6/6、三连换言断言、CJK-EN 界外 0——全部实跑、输出在案。
3. **测试先行证据**：spec 层变更沿先例（断言先于章文件）——注入哨兵先于条目落地实测 FAIL 即负例先行证据；纯 spec 层无编译器三要素义务。
4. **诊断协议稳定**：注册表 diff 的唯一删除行是段头 `[segments."E1700-E9999"]`（拆分为 E1700–E1799 owner 1900-ffi + E1800–E9999 unclaimed），既有 124 条目零删除零改名零字段变更；E1406–E1499 段注释按注册表描述维护先例刷新（FFI 码落 E17xx）；新增 7 码全局唯一（131 条确认）。
5. **单一权威**：章内容与段位已全量落 `docs/spec/`（19 新 + 1/6 修订 ×2 语言）；变更目录仅存提案/设计/任务/增量（归档即变更记录）；代码块注释英文（zh 孪生译文与术语表为既定义务）。
6. **红线复核**：`refr/` 零跟踪（`git ls-files refr/` = 0）；diff 范围 = docs/spec/ 第 1、6、19 章 ×2 语言 + diagnostics.toml + 变更目录，与「影响范围」行逐项相符、无越界改动；提交信息无署名（提交时复核）。
7. **最小可信验证**：`validate.py --all --strict` 绿（registry clean）、`docs_sync.py --check` 26 对绿——验证阶梯对本 diff 的全部适用命令。

无发现。披露两处实现期修正（均非规范缺陷）：E1701 注册表 title 初以 TOML 字面串写 `\'` 不合法（字面串不处理转义），改基本串 `"...is not \"c\""`；zh ch6 拼接一次产生三连空行被断言拦截、当场归一。
