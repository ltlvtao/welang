# 提案：collections（第 17 章）

## Why

类型六片与效果章落地后，规范里悬着的 collections 侧承诺全部指向本章：

- 第 2 章表达式骨架（`docs/spec/0200-grammar.md:101`）末句「Index syntax `expr[expr]` is deferred to the collections chapter; `[` and `]` remain lexical tokens」——下标延后句待落；
- 第 2 章文法诊断段位 E0105 场景（`docs/spec/0200-grammar.md:185`）以 `list[0]` 为「未批准章节语法」示例，注册表 E0105 描述同例——本章落地后「later chapter 可能批准」的暗示须换成永久立场；
- 第 2 章示例（`docs/spec/0200-grammar.md:281`–`282`）拒绝例注「the collections chapter adds it」——待刷；
- 第 11 章组合子预承诺（`docs/spec/1100-iterables.md:24`）「Combinators … land as default methods of this interface with their behavior fixed by the spec」，其 E0816 场景（`:44`）「until the combinator change ratifies them」——组合子表待兑现；
- 第 11 章 for 协议（`:77`）「the standard library's collection types arrive with the collections chapter」、实现者义务（`:144`）「The gc collections' snapshot-or-cursor obligations belong to the collections chapter」——两个指针句待落；
- 第 11 章段位预留（`:158`）与场景（`:167`）以 collections 为预期修订者；示例导语（`:172`）与 Pending 块（`:238`–`251`）标注组合子与 List/Map/Set 待批；
- 第 7 章（`docs/spec/0700-types.md:243`）、第 8 章（`docs/spec/0800-composites.md:367`）、第 5 章（`docs/spec/0500-iteration.md:114`）、第 12 章（`docs/spec/1200-fn-types.md:283`）示例 Pending 块同指本章；第 12 章段位场景（`:194`）以「the combinator change」为预期取码者——本章落地后须按实际裁决刷新；
- 第 15 章预导入（`docs/spec/1500-modules.md:35`）封闭集不含集合类型名——List 字面量若 import-gated 将违 P2；
- 第 10 章泛型参数声明（`docs/spec/1000-interfaces.md:295`）「Generic methods … are not ratified; they arrive, if wanted, through this requirement's amendment」——组合子签名（`map`/`fold` 的 `U`）是该预埋进入点的第一持有者。

v0.8 基线：§21.2.2 组合子表（12 组合子、惰性/急性规则、E0504 纯度、U 推断、编译器封闭 Iterator）、§51 std.collections（List/Map/Set gc 实现 Iterable+Iterator）、§9.4 String 字节/码点双层、§12 无运算符重载与无下标立场、§31.4 List mut 方法。

## What Changes

1. 新增第 17 章 `docs/spec/1700-collections.md`（+zh）：6 个 Requirements——集合类型（List/Map/Set、gc 类别、泛型元数 1/2/1、预导入成员、Iterable 实现：List/Set 元素、Map `(K, V)` 条目）、List 字面量（`[e1, ..., en]` 初等表达式、元素一致 E0501、空字面量期望类型、E1501、下标永不批准）、以命名方法索引（List/Map `.get -> Option`、Set `.has`、String `.byteLength/.byteSlice/.runeCount/.charAt`、越界字节/码点操作 panic 族）、快照迭代（兑付 ch11:144）、集合变异显式且别名可见（mut self 方法唯一变异面）、collections 诊断段位 E1500–E1599（唯一新码 E1501）；共 21 个 Scenarios。
2. 第 11 章修订：ADDED「Iterator combinators」（11 组合子——**forEach 弃置**；惰性 map/filter/take/skip 返回 `Dyn<Iterator<...>>`、急性 collect/fold/reduce/count/any/all/find；接收者保持单一一次性句柄、层叠无中间集合；f 纯函数类型 → E1402 复用；U 单向定出 E0827）+ MODIFIED 迭代器接口（组合子预承诺句兑现）、for 协议（集合类型到达句落定）、实现者义务（快照句落定）；+1 场景、改 1 场景。
3. 第 10 章修订 ×2 requirement：方法泛型子句批准（接口方法签名 / impl 方法定义 / 固有方法，名字后、接收者参数表前；impl 须与接口精确同子句 E0808；E0826 遮蔽规则活用于方法嵌套；无显式类型实参形，不定即 E0827）入「Generic parameter declarations」+「Interface declarations」方法签名形；+2 场景、改 1 场景。
4. 第 2 章修订 ×2 requirement：表达式骨架（初等枚举 + List 字面量；下标延后句改为永久不批准——索引唯命名方法，改政策须修本章）+1 场景、文法诊断段位 E0105 场景示例换永久措辞。
5. 第 12 章修订：段位场景「the combinator change among them」按实际裁决刷新（取码零新增、纯度乘 E1402）。
6. 第 15 章修订：预导入封闭集 + `List`、`Map`、`Set`（Option 先例：类型语言可见、方法清单归 stdlib 表面）；场景 1 WHEN 扩。
7. 注册表 `docs/spec/diagnostics.toml`：段位 `E1500`–`E1599` owner `1700-collections`，unclaimed 收窄为 `E1600`–`E9999`；分配 E1501（105→106 条、17 段）；描述维护 3 条（E0105 下标例、E0808 +泛型子句、E0826 方法子句活用）+ 段内注释 2 条（E0900/E1000「combinators, collections」措辞）。
8. D14 示例刷新 ×2 语言：第 2 章（`list[0]` 拒绝例换 `.get(0)` 合法例 + 永久拒绝例）、第 5 章（组合子 Pending 块移除）、第 7 章（List 注记落定）、第 8 章（集合方法 Pending 行）、第 11 章（导语 + Pending 块转真实组合子示例）、第 12 章（Pending 块刷新：组合子与方法子句落地、显式实例化形仍无）。

## 影响层

spec。

## 影响范围

`docs/spec/` 第 2、10、11、12、15、17 章 + `diagnostics.toml` + 六个 zh 孪生（2/10/11/12/15 新增 17）；`docs_sync` 对数 23→24；不触碰 chapters 之外任何工件。

## 裁决记录

1. **切片范围 = 一章全量**（用户裁决，采推荐）：List/Map/Set + List 字面量 + 索引政策 + gc 迭代语义落第 17 章，组合子以第 11 章修订在同一变更落地——互引关系（collect() 返回 `List<T>`、组合子示例需 List 字面量）拆两变则会互相悬空。
2. **索引政策 = v0.8 立场：无下标**（用户裁决，采推荐）：`[]` 下标永不批准，`expr[expr]` 永久无产生式（第 2 章延后句就此关死，重开须修第 17 章政策本体）；索引用命名方法：`.get(i) -> Option<T>`（越界 None）、String 字节/码点分名（`.byteLength/.byteSlice/.runeCount/.charAt`，越界 panic）。
3. **纯度执法 = 复用 E1402**（用户裁决，采推荐）：组合子 f 参数写纯函数类型（`fn(T) -> U` 无效果段），效果闭包在实参一致位自然 E1402——零新纯度码，v0.8 E0504 不继承。
4. **迭代语义 = 快照**（用户裁决，采推荐）：`iterator()` 调用时刻固定元素序列，迭代中经别名的变异不被该迭代器所见，新调用见新状态——读循环即知迭代集（P1），兑付 ch11 的 snapshot-or-cursor 债。

## 目标与非目标

目标：
- 落定第 2/11 章悬着的全部 collections 侧指针（下标延后句、组合子预承诺、快照债、类型到达句）；
- List/Map/Set 三类型入预导入、入 Iterable 协议（Map 迭代 `(K, V)` 条目），gc 别名语义与变异面全场景钉住；
- 11 组合子作为 Iterator 默认方法批准：惰性/急性、层叠无中间集合、接收者一次性、U 单向定出、override 全同（E0808）；
- 方法泛型子句经第 10 章修订批准（E0826 预埋规则活用，E0808 扩及子句一致）；
- E1500–E1599 段位与 E1501 一码入注册表；E0105/E0808/E0826 描述维护同步。

非目标：
- 完整方法清单（`.add/.removeAt/.put/.keys/.size/...`）——stdlib 表面，spec 只锚语义承载位（字面量、访问、迭代、变异面）；
- Map/Set 字面量——v0.8 无此表面，且 `{}` 与记录构造冲突，需要时另立变更；
- forEach——v0.8 自相矛盾（规则 7 令副作用归它、其 f 却为纯型），效果化需效果多态（任何章均未批）；序列副作用归 for 语句；
- 效果多态 / 泛型效果参数——不在任何已批准方向上；
- String 四锚点之外的方法（`toUpper` 等）——stdlib 表面；
- 下标糖或任何 `expr[expr]` 产生式——裁决 2 永久关死；
- 方法显式类型实参形（turbofish）——方法子句单向定出，不定即 E0827 改写调用，不新增文法。

## 审计记录

**2026-09-04：通过。** 7 点逐项：(1) Why 引 16 处已落地规范锚点（ch2:101/185/281、ch11:24/44/77/144/158/167/172/238、ch5:68/114、ch7:243、ch8:367、ch12:194/283、ch15:35、ch10:295）+ v0.8 §21.2.2/§51/§9.4/§12/§31.4/§21.2.1，逐条实读核对，黑盒可验证；(2) layers `[spec]` 与影响层一致（validate 结构检查同步通过）；(3) ch17 6R/20S + ch11 组合子 1R/7S 与 E1501 机器查重零冲突（注册表 105 条无 E1501、无其他活跃变更；delta 所提未注册码 E0100/E0106/E0199/E1000/E1005/E1099/E1500/E1502/E1599 均为段内预留边界提及，ch10 E0800/E0832/E0899 先例）；复用码 E0501/E0105/E1402/E0808/E0826/E0827/E0816/E0904 均非新增；MODIFIED 逐字前缀机器比对通过（差异仅既定改动 + 三处块尾空行拼接伪差）；(4) 无原则突破——E1501 是「不推断」的执行（P1）、快照消灭 v0.8 的「迭代中修改未指定」（P4）、字节/码点分名（P4）、元素一致 E0501 无隐式转换；第 10 章方法子句批准是预埋口（E0826「binds from the moment one arrives」）的兑现非原则例外；(5) 基线固定 refr/spec-0.8.md 六节；(6) 目标（指针落定 grep 可验、组合子表完备、段位/码数可数）与非目标七项排除（方法清单、Map/Set 字面量、forEach、效果多态、String 四锚外方法、下标糖、方法显式类型实参形）均可机械判定；(7) 单层垂直最小单元——ch10 方法子句修订由组合子签名强制（map/fold 的 U），同一垂直故事非无关层。

## 审查记录

**2026-09-04：通过（修复 4 处发现后）。** 10 点审查发现并已在激活前修复：F1（真缺陷）`byteSlice(start, end)` 的端语义未钉——端含/端斥两种读法都成立而示例无法裁决；修复：R3 明文「end exclusive, the span `start..end` as chapter 5's ranges」，示例注释改 `"hel": bytes, end exclusive`。F2（真缺陷）快照只钉了 List 的序——Map/Set 无序类型的迭代序未钉，「未指定」正是本规范拒收的（P4），且 v0.8 自己就带着这个洞；修复：R4 增「an order the snapshot itself fixes at the call for `Map` and `Set` — 无序类型不保证跨迭代器之序，只保证每个迭代器自己的序列是其调用时刻的快照」。F3 非空字面量对分歧注解（`let xs: List<String> = [1, 2]`）无场景——元素定 `List<Int64>`、注解要 `List<String>`，E0501 一致位在该处的触发须钉住；R2 +1 场景（20→21）。F4 两处措辞泛化：R1 元数场景「chapter 10's generic-arity diagnostic」实名 `E0828:` type argument arity mismatch；R3 越界定义实名「below zero or at or above the length」（负索引归 None 路径）。职责边界（proposal 黑盒、delta 无实现泄漏——组合子默认方法体归 std 不入文、design 唯一路径含被拒方案、tasks 来源验证齐且无未决选择）与内容质量（normal/boundary/failure 三路齐——E1501/E0501/E1402/E0816/E0105 拒绝侧、快照别名边界、急性空迭代器边界；无空章节；负向断言真实注入任务在列；纯 spec 层无三要素义务——实现未启动，原则 10 的三要素义务属编译器实现变更）其余各点通过。

## 实现审查记录

**2026-09-04：通过。** 7 点逐项：(1) 规范符合性机器抽检——landed==delta 断言 10/10（九 MODIFIED + ch11 ADDED）、ch17 组装 6R/21S 与 delta 逐字、注册表 E1501 requirement 字段解析到章文件、段位 E1500–E1599 owner `1700-collections`（tomllib 对照 106 条/17 段）；(2) 六任务验证命令全部真实运行且结果与完成记录一致（validate --all --strict、docs_sync --check 24 对、注入 E1599 于 `--all --strict` FAIL 后还原、宿主计数 10/10、一码一消息 12/12 精确）；(3) 负例注入于宿主落地后、定案收口前执行（spec 层测试先行 = 断言先于条目定案）；(4) 纯 spec 层无 `--json` 触碰，E1501 全局唯一且同批登记，E0105/E0808/E0826 仅描述维护、标题未动；(5) 长期事实全部落位 docs/spec（ch17、九处宿主、注册表），变更目录只存增量，docs/spec 英文无中文混入（zh 孪生为既定双语机制，代码注释英文）；(6) refr/ 未触碰、无署名 trailer、diff 范围 = docs/spec + 本变更目录，无非目标越界（D14 六章刷新均在 What Changes 8 申报范围内）；(7) 验证阶梯按实际 diff 选取的命令全绿。披露三则：其一，per-change 模式（`validate.py collections --strict`）不复跑 docs/spec usage 扫描，注入类负例必须 `--all --strict`——本次再次实证；其二，MODIFIED 机器比对在审计期发现的三处块尾空行拼接伪差于实现期以规范化消除（拼接函数统一 rstrip + 双换行收尾），EN/zh 邻接检查零违例；其三，注册表维护中主动捕捉 E0800 段内注释陈旧（「no nesting surface yet」在方法子句批准后失效）并同批修正——超出 What Changes 7 申报范围的这一处属描述性注释与事实一致性，随段位维护一并落地。另：zh ch2 示例注释既有中文风格为历史遗留（旧章如此），本变更加入的 zh 代码块一律英文注释（语言约定），不回改遗留块。

