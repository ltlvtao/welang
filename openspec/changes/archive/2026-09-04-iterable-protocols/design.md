# Design: iterable-protocols

## D1 迭代器句柄形态：静态分发（裁决 5 的完整推理）

起草时发现 v0.8 §21.1.1 的协议声明 `fn iterator(self) -> Iterator<T>` 与本仓 ch10 刚批准的 E0821（「an interface occupies type slots only as `Dyn<Interface>`」）直接冲突：接口名作返回标注即隐式存在类型，正是上一片裁定要显式化的东西。两案：

- **Dyn 装箱**：`fn iterator(self) -> Dyn<Iterator<T>>`。零 ch10 修改，v0.8 的 Iterator 自动满足 Iterable 内建 impl 可表达；代价是每次迭代一个分配 + `next` 逐元素虚分发，组合子链永久带装箱地板。
- **静态分发（裁决采纳）**：`type Iter` 关联类型 + `fn iterator(self) -> Iter`，每 impl 绑定具体迭代器类型。零成本；代价有二且均已消化：`Dyn<Iterable<T>>` 因含关联类型被 E0819 既有规则拒绝（罕见需求，`Dyn<Iterator<T>>` 仍合法）；v0.8 的自动满足内建 impl 无法表达（Iter 的绑定值不能是接口名——E0821），故 for 收窄为单一形式（D2）。

## D2 for 单一形式，无自动满足

v0.8 靠「每个 `Iterator<T>` 内建满足 `Iterable<T>`」统一 for 的判定。静态分发下该内建 impl 写不出，于是 for 只认 `Iterable<T>`（E0901 单码）。用户类型同时实现 `Iterator<T>` 与 `Iterable<T>`（`Iter` 绑定到自身、`iterator()` 返回 `self`）完全合法且无歧义——for 恒走 `Iterable` 一侧。裸迭代器经显式 `next` 消费；v0.8「for 半消耗迭代器从当前位置继续」语义不再存在，提案已披露为有意偏离。

## D3 E0904 句柄契约及其约束反映，不动 ch10

ch10 的 where 约束主语只认泛型参数，`C.Iter: Iterator<T>` 不可写。静态分发的落地是接口契约：每条 `Iterable<T>` impl 必须把 `Iter` 绑定到实现 `Iterator<T>`（同元素类型）的类型——违者 E0904；凡已知某类型实现 `Iterable<T>` 处（有界泛型参数等），其 `Iter` 携带 `Iterator<T>` 方法集。这是对「`C.Iter` 已知什么」的声明，不改「对 `C` 已知什么」（ch10「and nothing else about it is known」的对象是参数自身），故无需 ch10 MODIFIED。需精确调用 `Iter` 时用 ch10 既有等式约束 `C.Iter == Concrete` 收窄。

## D4 E0901 对应 v0.8 E0252，码位重排

v0.8 E0252「type does not implement Iterable, cannot use in for-in」语义原样继承，按本仓段位规则落 E0901。for 头元素类型非元组的错配复用 E0501（ch8 Tuple patterns 既定：「a tuple pattern against a scrutinee that is not a tuple of the same arity is rejected under chapter 7's `E0501` at the match or binding position」——for 头即绑定位置），不开新码。

## D5 Range 整数限定与 E0902/E0501 分工

裁决 3 限整数：浮点单位步长的元素个数依赖位表示，与 P1 相悖。码分工：两个操作数类型不同（含同族整数混合宽度）→ ch7 E0501（类型一致位置）；操作数类型根本不在八整数之列（Float、Rune……）→ E0902。`Range<T>` 仅经 `..` 构造（无字面、无 record 形式），值即边界对，携带无状态。

## D6 内建 impl 复用 derives 先例

String（基类型头）与 `Range<T>`（内建类型头）的 `Iterable` impl 及其 `Iter` 绑定按 derives 同义的内建 impl 处理：提供而非书写——手写尝试落在 ch10 E0811（头不合 impl 产生式）；「同头唯一」由 E0809 语义保证。标准库迭代器类型（rune 迭代器、range 迭代器）的具体命名留给标准库表面，本章只固定义务。

## D7 resource 禁止的义务框架（E0903）

继承 v0.8 §21.2.1 规则 3 的 resource 分支并升格为 MUST NOT + 诊断：迭代器是对集合的活视图、跨循环体执行存活，其触达超出 resource 确定性释放所依赖的单一释放点。替代路径显式化：先物化（一次读入集合）再迭代。value 类别按 ch8 值语义拷贝迭代（`iterator` 收到接收者副本），原绑定永不消耗。gc 快照/游标义务归集合章。

## D8 Option 进驻本片

`next` 的返回类型需要 element-or-absence：`Option<T>` 是最小必要进驻。定位为标准库 canonical 选项 sum（构造/匹配全按 ch9 既有规则，零新句法），后续章节的选项形结果绑定于它而非另立兄弟——与「一类事实一个权威位置」一致。方法清单留标准库表面。

## D9 组合子延后的落点

v0.8 §21.2.2 惰性/急性组合子族需函数值（fn 类型 + 闭包，队列下一片）与 List（collect 产出）——随其所属变更落地为 Iterator 接口默认方法（ch10 机制），行为由规范固定。本章 Iterator requirement 与示例 pending 块均已注明，避免「不存在」误读为「永不」。

## D10 ch7 零修改

ch7 Base type inventory 的 String 句在接口片刷新时已按名指向「iterable-protocols chapter」，事实随本章落地即自动闭合——前向引用纪律的又一次兑现（declarations 片先例之外的第二例：零 delta、仅示例 D14）。

## D11 ch5 场景替换的诚实记录

「destructuring in the loop head is not ratified」场景以批准场景「a tuple pattern in the loop head」替换（标题变更），理由文本过期问题随之消解；该替换在 tasks 验证中以机器比对披露（继承差异 = 恰此一处标题变更，其余 5 场景逐字继承）。
