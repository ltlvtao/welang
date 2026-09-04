# Design Notes：collections

## D1：一章全量还是拆两变

拆分方案（collections 章 / combinators 章各一变更）被否：两者互引到拆不开——`collect()` 的签名返回 `List<T>`（组合子需要集合类型先存在），组合子示例链 `.iterator().filter(...).collect()` 需要 List 字面量提供输入（集合章的示例需要组合子已批）。裁决 1 采一章全量：第 17 章持类型/字面量/索引/迭代语义，第 11 章修订持组合子表，同一变更内互引闭合。缺点（单变更大）由 delta 分文件（collections/iterables/interfaces/grammar/fn-types/modules 六处）抵消。

## D2：List/Map/Set 入预导入

字面量 `[1, 2, 3]` 若需 `import std.collections` 才能定型，则最常见表达式带着 import 噪声（P2）；v0.8 全文裸用 `List`。预导入是第 15 章封闭集的扩展，本变更同批修订。边界沿用 Option 先例（ch11:5）：类型名语言可见，方法清单是 stdlib 表面——spec 只锚语义承载位：`.get/.has` 与 String 四方法（访问语义）、`iterator` 快照（迭代语义）、mut self 方法（变异面）；`.add/.removeAt/.put/.keys/.size/...` 不入 spec。Map/Set 字面量不批：v0.8 无此表面，`{}` 与记录构造表达式冲突。

## D3：无下标，永久

v0.8 §12 立场（无运算符重载、自定义索引用 `.at(index)/.get(index)`）继承为规范本体：`expr[expr]` 永无产生式。承载位：第 17 章 R2/R3（政策本体）+ 第 2 章骨架修订（延后句 →「not a form of this language … a revision of that policy there is the sole route」）。重开此政策的唯一路径是修第 17 章政策本体，非仅修第 2 章。String 双层分名（`.byteLength/.byteSlice` 字节层、`.runeCount/.charAt` 码点层，名字即层，无裸 `.slice`）：字节/码点是两个不同的数（P4），越界字节切片与越界 charAt 走第 14 章 panic 族——命名不存在的存储，范围检查归调用方，panic 是边界；此为 v0.8 沉默处的补全（byteSlice 越界 panic 在 v0.8 §9.4 明文，charAt 越界沉默，同判齐平）。

## D4：E1501，唯一新码

全部其余拒绝位复用既有码：元素不一致 E0501（一致位族已涵盖）、下标尝试 E0105（意外 token）、组合子纯度 E1402（裁决 3）、override 不全 E0808、impl 绑定 E0904。唯一无码位是空字面量 `[]` 无期望类型——语言不推断（P1），注解是元素类型唯一来源。分配 E1501（段位 E1500–E1599，E1500 与 E1502–E1599 留段内修订）。

## D5：E1402 复用与 forEach 弃置

组合子 f 参数类型写纯函数形 `fn(T) -> U`（第 16 章类型位裸标签省略 = 纯）：效果闭包的推断集非空、期望集为空，子集检查在实参一致位失败 → E1402。第 16 章的一致位族设计（值 ⊆ 期望，单向）使组合子纯度零新码零新规则——v0.8 E0504（专码）不继承，效果章落地时已预埋此路。forEach 弃置（v0.8 12→11 组合子）：v0.8 规则 7 要求副作用经 forEach 表达而其签名为 `fn(T) -> ()` 纯型——自相矛盾；效果化 forEach 需要按调用方效果集参数化组合子（效果多态），任何章均未批准且不在方向上；序列上的副作用是 for 语句的（其体是命令式语句序列，携带外围声明效果——v0.8 自己也这么对比）。`.forEach(...)` 调用按 E0816 拒绝（ch11 场景 + 本章示例钉住）。

## D6：快照，不是游标

裁决 4：`iterator()` 调用时刻固定元素序列。游标方案（迭代器持原集合引用、变异即反映——Go slice 语义）被否：读循环不能确知迭代集（P1），v0.8 §21.2.1 自己也承认「迭代中修改集合的行为是未指定的」——未指定行为正是本规范拒收的。快照写成语义义务非实现命令（复制 / 不可变共享 / COW 皆可），只要可观察序列是调用时刻的。兑付 ch11:144 的 snapshot-or-cursor 债，落定句进 ch11 修订。

## D7：方法泛型子句，经第 10 章预埋口批准

`map`/`fold` 的 `U` 使方法级泛型子句不可回避。第 10 章「Generic parameter declarations」预留句（「Generic methods … are not ratified; they arrive, if wanted, through this requirement's amendment」）即为此口；E0826 描述更预写「the rule binds from the moment one arrives」。批准面：接口方法签名 / impl 方法定义 / 固有方法，子句位于方法名后、接收者参数表前；impl 须与接口子句精确一致（名字+元数，E0808 描述同步维护）；E0826 遮蔽活用于「方法子句嵌套于声明子句」（场景由「nesting arrives」改为活例）。显式类型实参形不批：`obj.m<T>(x)` 与比较表达式歧义（`<` 是运算符），turbofish 需 `::` 而 ch1 标点清单无之——方法子句单向定出（实参文本），不定即 E0827、修复是改写调用（如先注解绑定 f）；v0.8 规则 8 本就是纯推断。E0827 描述「generic call or construction」天然涵盖方法调用，不需维护。

## D8：惰性返回 `Dyn<Iterator<...>>`，接收者是活的

v0.8 签名 `-> Iterator<U>` 在本规范不可表达（E0821：接口名非值类型）。解：`Dyn<Iterator<U>>`——Iterator 无关联类型、可装箱，ch11:48 预留句「may be boxed, `Dyn<Iterator<T>>`, when an erased iterator handle is wanted」正是为此预写的。接收者语义不新造规则：迭代器是一次性对象（恰一次、按序、耗尽永续——ch11 契约按对象不按绑定），惰性组合子返回的迭代器与原绑定是同一对象的两只句柄（ch8 gc 别名）——谁调 `next` 谁取下一元素。v0.8「急性组合子后原迭代器被消耗」从中自然推出（collect 把对象推进永续耗尽）；「层叠无中间集合」（v0.8 规则 6）同样按需单遍流过。v0.8 规则 9（编译器封闭 Iterator、E0705 拒绝用户 impl）不继承——ch11 已裁开放 Iterator，组合子是 std 提供体的默认方法体，用户 impl 自动继承、override 走 E0808 全同；此偏离承 ch11 裁决并在此披露。

## D9：宿主指针清理全景

落定：ch2:101（延后→永久）、ch2:185（E0105 场景示例换永久措辞）、ch11:24/:44（组合子预承诺兑现 + E0816 场景反转）、ch11:77（类型到达句）、ch11:144（快照债）、ch15:35（预导入枚举）。刷新：ch12:194 场景「the combinator change among them」按事实改写（该变更零取码、纯度乘 E1402、走「claims its own segment」分支——原文的反例预期不成立）。保留：ch11:167 场景「the collections chapter among them」——其 THEN 的「or claims its own segment」分支恰是本章事实，原文自洽。注册表同步：E0105 描述（下标例换永久措辞）、E0808（+泛型子句）、E0826（方法子句活用）、E0900/E1000 段内注释（「combinators, collections」措辞兑现后过时）。D14 示例刷新 ×2 语言：ch2（`list[0]` 例）、ch5（组合子 Pending 块移除——组合子示例归 ch11/ch17）、ch7（List 注记落定）、ch8（集合方法 Pending 行）、ch11（导语 + Pending 块转真实示例）、ch12（Pending 块：组合子与方法子句已落地、显式实例化形与方法值仍未）。

## D10：v0.8 映射

§51 三类型（List/Map/Set、gc、Iterable+Iterator）继承并锚定（Map 迭代元素定为 `(K, V)` 条目——v0.8 沉默处补全）；§21.2.2 组合子 12→11（forEach 弃置，D5）；E0504→E1402 复用（D5）；规则 8 U 推断→E0827 单向定出（D7）；规则 9 编译器封闭→开放默认方法（D8 披露）；§12 无下标→政策本体（D3）；§9.4 String 四方法→锚定访问位（D3）；§31.4 mut 方法（`q.add(job)/q.removeAt(0)/q.size()`）→stdlib 表面、变异面由 R5 钉住；§21.2.1 gc 迭代「未指定」→快照裁决（D6）。
