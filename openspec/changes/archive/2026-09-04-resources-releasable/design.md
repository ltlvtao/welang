# Design: resources-releasable

## D1 函数局线性纪律（裁决 1）

资源绑定生命周期线性且函数局：参数、let、scope 头绑定自绑定点起活，函数体内每条控制路径必须恰达三通道之一——scope 头转移（`scope resource(x = f)`，源绑定死于转移点）、return 上交（义务移给调用方）、实参传递给参数声明该资源类型的调用（义务移给被调方，签名携带）。

- **P1 健全性**：v0.8 §35.2 规则 4 字面要求「跨函数路径分析」。全局流分析非局部可判定，违 chapter 0 Principle 1。本设计把跨函数义务全部压进签名（参数类型、返回类型即义务声明），函数体内部的分析只需自身流图——义务能否出境由签名读出，义务在哪终结由本体裁决。这与 ch6「parameter types are never inferred」同一立场：不推断、只声明。
- **顶层关死**：ch6:5 模块顶层 let 存在，但三通道都是函数体内部的通道；顶层资源绑定无任何合法终局，E1104 直接拒绝。副作用初始化需求由 `fn init()` + 调用点承担（后端服务章若批准）。
- **路径判定**：if/else 各臂、循环体、match 臂逐路径检查「恰一次」；提前 return 的路径在 return 处终结。转移后使用（use-after-transfer）同一码 E1104——绑定已死，读取死绑定就是脱离纪律。

## D2 单一释放触发点（裁决 5）

`release` 只有一个调用者：块出口机制。

- **双重释放缝**：若允许直调 `f.release()`，实现方必须幂等（手动 + 块尾各一次），信任面落在每个 byres 实现上。禁直调后，绑定在 scope 头下恰好释放一次是结构性保证，非实现方义务。Rust `Drop::drop` 禁手动调用同一先例（`std::mem::drop` 是取走值而非调 drop）。
- **impl 体内部**：impl Releasable 的方法体是定义处，体是普通代码；但体内再对资源类型接收者调 `release`（递归/交叉）同样 E1104——触发点唯一无例外。
- **早释放 = 早出口**：`return`/`break`/`continue` 是块出口，释放照走。守卫早退即守卫早释放，无需新拼写。
- **被调方关闭参数**：`sink(f)` 把义务交给被调方后，被调方以 `scope resource(x = f)` 头转移接手——转移杀死源绑定，两函数各自局部可判定。

## D3 禁别名与禁 var（裁决 2）

资源绑定是资源的唯一句柄。`let g = f`（E1105）制造第二句柄；`var f = ...`（E1105）制造可重绑句柄——重绑后旧值去留不明，违单句柄。赋值任一侧资源类型同码（E1105）。唯一合法移动是 D1 的转移：交出并杀死源绑定，非别名。ch8:62「reference-passed」不构成别名通道：引用传递说的是实参传递语义，绑定本身仍唯一。

## D4 组合位禁令（裁决 3）

资源类型只出现在绑定位：参数、let/scope 头绑定、返回类型。组合位（tuple 元素、gc 记录字段、sum payload、newtype 底型、泛型实参一律、`Dyn<Releasable>` 类型位与装箱构造）统一单码 E1106。

- **同一拒绝理由**：组合位里的句柄有多条到达路径（字段读写、模式解构、泛型展开、盒共享），越过纪律依赖的单一确定性释放点。E1002（闭包捕获，ch12）/E0903（Iterable 实现，ch11）已禁同理，本片引用不变更——「同理已禁」保持一类事实一个权威位置。
- **Dyn 盒关死（审查 F1）**：ch10:227 盒为 gc 共享值（赋值/传参/返回共享盒）且构造仅查实现关系（E0818）——`Dyn<Releasable>(f)` 合法构造即可绕穿单句柄（盒别名）与单触发点（`d.release()` 接收者非资源类型）。在 E1102 完备性下每个 Releasable 盒必装资源，故类型位与构造位一并 E1106 关死；ch10 拥有一般可装箱规则，本章持有资源例外——一类事实一个权威位置。
- **泛型实参一律禁（审查 F2）**：草案原拟「到达组合位才拒」（参数仅绑定位的 `id<FileHandle>` 放行），但被调体内 T 可能是资源，流分析非局部可判定，违 P1；收紧为一律禁，绑定位精化留段内修订（E1100–E1199 预留）——亦更贴裁决 3「禁入一切组合位」字面。
- **byval 字段例外是 E0601 自有**：byval 记录整体拷贝语义下资源字段本就被 ch8 拒绝，无需本片重述，仅括注指路。
- **池化惯用法（pool-owns）被否为非目标**：池需要 `record Pool { handles: Box<FileHandle> }` 之类的组合位持有，与禁令冲突；解禁需本条修订 + 组合位语义设计，留未来变更。披露于非目标。

## D5 文法落位

- **ch1 关键字增补**：`scope resource` 两词循 composite/`type`/interface 先例进入枚举列表——破坏性变更照录（两词此前是合法标识符）。`scope` 单独预留超出本片最小需要，但 ch1 先例（`mut` 预留先于使用）支持：并发章的 scope 复合形式将以 `scope` 引导，一次破坏性好过两次。此为裁决 4 附属决定，先例链披露。
- **ch2 statement-start 类无需修订**：ch2:101 语句首位类「identifiers, literals, keywords, `(`, `{`, and the prefix operators」已含 keywords——`scope` 作为关键字自动获得语句首位资格，无需 Line-joining 或 statement-start 修订。语句族枚举加一句「the resources chapter ratifies the scope resource statement」。
- **绑定无注解位**：`name = expr` 不带 `name: type`——头表达式类型即绑定类型，且该类型必须实现 Releasable（E1103 检查），注解无信息增量；与 ch2 let 的可选注解不同属（let 无实现性约束）。
- **k≥1**：空绑定表 `scope resource() { }` 无意义（无绑定则无释放），按无产生式处理走 E0105——与短闭包零参裁决（fn-types D5 第 4 点）同一「无意义形不设产生式」立场。

## D6 宿主账目与注册表

- **四宿主 MODIFIED**：ch1 Keywords（列表 +1 行、增补段 +1 句、+1 场景）、ch2 Statements（枚举 +1 分句）、ch3 Defer（指针句落定）、ch8 Resource records（悬置句落定 + 场景 1 THEN 尾句）。splice 边界正则必须 `^(## |### )`——`^##+ ` 会误吞 `#### Scenario:`（fn-types 教训，零 diff + 场景计数 + git HEAD 对照三验）。
- **E0204 remediation 同变更维护**：现文「or release the resource through the mechanism the resource chapter ratifies instead of a nested defer」中机制获名，改为「`scope resource`」实名。remediation 文本变更不是改码义（9900:33 禁的是改 meaning），是消歧维护；E0401 先例（fn-types 同变更扩展 description）。
- **段位**：E1100–E1199（owner 1300-resources），未认领尾部收窄为 E1200–E9999。六码 allocation 日期 2026-09-04。
- **D14 刷新**：ch8 示例 pending 注释（EN+zh）——「release operations and Releasable enforcement are the resource chapter's」→ 已落定表述。ch3 示例无资源引用（已核）。

## D7 代码分配映射

| v0.8 | 本规范 | 落位 |
|---|---|---|
| E0726 byres must implement Releasable | E1101 | R1（声明完备性，声明模块查） |
| E0727 non-byres implements Releasable | E1102 | R1 |
| E0728 scope resource head not Releasable | E1103 | R2 |
| E0729 resource not released on all paths | E1104 | R3（纪律违例总码：落空、转移后使用、顶层、直调） |
| — | E1105 | R5（别名/重绑，v0.8 未细分） |
| — | E1106 | R6（组合位，v0.8 未细分） |
| W0724 warnings | 不批准 | 纪律以错误执法，非警告 |

E1104 单码多触发是裁决点：落空、use-after-transfer、顶层、直调共享「脱离释放纪律」单一语义，描述在注册表条目内分项枚举；分码会使「纪律」碎片化为四条可分别检索的规则，AI 原生检索反而失焦。
