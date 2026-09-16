# proposal — stdlib-collections（B2b）

> 状态：candidate 草案（勘查与五项裁决已完成，待 spec-impact audit）

## Why

B2a `stdlib-fsproc` 已归档（HEAD `13e9fe9`，conformance 893、codegen 单测 461、16 包全绿）。roadmap B2b 行给本变更定的内容是「Map/Set 载体、构造面、变异成员面、List.add 别名可见性、Builder」——B2 策略句「B2 是 B1 的调试基建」的集合半场：编译器写出的程序要能建出并使用 Map/Set，集合面背后的真数据结构才存在，B3 自举的语料面才够宽。

**五项用户裁决（2026-09-15/16，AskUserQuestion，逐字入册）：**

| # | 裁决 | 内容 |
| --- | --- | --- |
| 1 | **List 载体** | **稳定句柄间接层**——32B 句柄（`{map@0, size@8=32, data@16}`），增长时就地交换 data 指针；别名可见性由此成立（`let ys = xs; xs.add(4)` 后 `ys.size()` 为 4——roadmap 点名的「List.add 别名可见性」项）。 |
| 2 | **构造面** | **mapOf/setOf 自由函数、不做 Builder**——参数定域（arg-determined）、keyed C 入口、`std.collections` 的 `.we` 虚构体（B2a 装载机制的第二批消费者）。 |
| 3 | **成员清单** | **全三族 13 员**：List{add, removeAt, get, size} + Map{put, remove, get, keys, size} + Set{add, remove, has, size}。返回形：put 答 `()`、add 答 `()`、removeAt 答 `Option<T>`、Map remove 答 `Option<V>`、Set remove 答 `Bool`、get 答 `Option`、keys 答 `List<K>`、size 答 `Int64`。 |
| 4 | **键域** | **Eq 域**——8 整型 + Bool + String；String 键 FNV-1a 64（字节串上）、整型恒等；域外（Float64、记录、和式……）诚实停点。 |
| 5 | **mapOf 签名** | **双列表 zip 形**——`mapOf(keys: List<K>, values: List<V>)`（另 `setOf(items: List<T>)`）；长度不匹配 = 运行期陷阱（ch14 task_fail 族）。勘查漏算的约束：`List<(K,V)>` 今日不可构造——元组元素面与元组值表达式整簇是停面（B1 轨遗留、B2b 域外）。 |

### 今日事实（勘查 + 真机探针，HEAD `13e9fe9`）

**成员面今日态**（`internal/typecheck/typecheck.go:1356` `collectionMembers`，咨询点 `:6959`）：List{iterator, get} / Map{iterator, get} / Set{iterator, has}——恰是 ch17 锚定族。真机探针（二进制 `/tmp/b2b-bin/we`，项目 `/tmp/b2b-probe`）：

- `xs.add(4)`（`List<Int64>`）→ **E0816** exit 1（`"add" is not a member of List<Int64>`）；`xs.removeAt(0)` → E0816；`m.put("a", 1)`（`Map<String,Int64>` 参数位）→ E0816——**13 员里今日无一在面**。
- `xs.get(1)` + match → check 0 / **build 70**（`bndMainBody`）——get 检查过、发射停；`s.has("a")`（Set 参数位）同形 check 0 / build 70（`bndOtherFns`）。
- `for (k, v) in m` / `for e in s` → check 0 / build 70——Map/Set 迭代检查过、发射停（**本变更不打开**，见非目标）。

**构造面今日态**：裸名 `mapOf`/`setOf` → **E1304**（held by no scope）；`import std.collections` → **E1302** std 形（`no standard-library module "std.collections" exists in this build`）。**Map 的值今日完全不可构造**——字面量、构造函数、std 面皆无。

**List 载体今日态**（`runtime/c/list.c`）：单块——冻结头 `{map@0, size@8}` + 三载荷字（len/cap/traced）+ 元素字；`__we_list_push` 满载时另雕大块复制、**返回新身份**（`list.c:12-16` 注释自述 "the storage moves, which is why push answers with the list's new identity"）。别名可见性因此今日不成立——`let ys = xs` 后的增长对 ys 不可见，与 ch17「gc 别名是同一对象的两句柄」的语义冲突；roadmap B2b 行点名此项。

**发射端零 IR 漂移的结构性事实**（裁决 1 落地的关键）：发射器对 List 载体的全部接触面 = 5 个 C 入口 declare（`codegen.go:207-211`：new/push/get/len/snap），**零 gep 进载体内部**；`fs.c:300-305` 的 C 侧 List<String> 构造按精确容量雕块并**忽略 push 返回值**；`emitCollect`（`:7346`）的重赋值形在「push 返回同一句柄」下逐字成立。句柄化是 `runtime/c/list.c` 内部改造，IR 文本不变。

**勘误记录（as-built 诚实）**：裁决 5 的问句选项文本曾断言「add 增量构造全通」——实测为误（`xs.add(4)` 今日 E0816）。正解：**两列表 zip 形的增量构造骑的是 B2b 自身落地的 add 面**（本变更成员清单裁决 3），组合自洽；元组条目形则另需元组值表达式簇（B1 轨遗留），故弃。裁决本身不受影响。

**发射先例全部在册**：keyed 三件套（`codegen.go:92` declare 表 + keyed map + 发射臂，fs 七枚先例）；**Option 返回族走 out 三字组**（`__we_coll_*_get(ptr, …, ptr out)`——sum 三字 ABI 24B 超 SysV 16B 寄存器界，fs D7-2 已拒 C 结构返回，本变更全盘沿用）；C 侧构造 List<String> 的 T11-② 32B 盒（String 字/键的装箱形）；C 返回 gc 句柄的调用点再扎根（T12 先例）；运行期陷阱走 `__we_task_fail`（ch14 族，list.c 越界先例）。

## 目标与非目标

目标（一条线：集合面的真数据结构）：

1. **List 载体 v2（稳定句柄）**：`__we_list_new/push/get/len/snap` 五入口签名不变、语义翻为句柄间接——增长就地交换 data 指针、push 返回**同一句柄**；发射端零改动（结构性论证见上）、既有语料行为逐字节绿；别名可见性黄金钉死。
2. **Map/Set 载体**：`runtime/c` 新载体——开放寻址 + 线性探测、容量 2 的幂、墓碑删除、rehash 倍增；键 Eq 域（8 整型恒等 / Bool 0·1 / String 盒上 FNV-1a 64）；值域 = List 元素域（单字：标量原位、String/gc 句柄装箱）；描记两区（键区 iff String、值区 iff traced——键里 8 整型与 Bool 永不描记）。
3. **std.collections 装载**：`stdlib/src/collections.we`（mapOf/setOf 虚构体——panic 体，Never 满足任意返回位；keyed 拦截替换调用点）+ import 走查 `case "collections"` 落 stdQuals（fs keyed 先例）；**programModules 零触碰**——collections 是键控模块（虚构体、零 define），不乘程序面，fs/process 先例（string 需白名单只因真体 define；T5 探针复核）。
4. **检查器成员面**：`collectionMembers` 扩 13 员（裁决 3 表）；E0816 自然收缩（未知员仍 E0816——边界黄金钉住不翻转过宽）。
5. **成员发射**：13 员 C 入口臂——Option 族 out 三字组、keys 答 List 句柄（调用点再扎根）、size 答 i64、put/add 答 void；域外键（Float64/记录……）发射侧域标签映射失败即诚实停（裁决 4）。
6. **mock 面**：mapOf/setOf 骑槽（keyed 先例 uniform 落位），但声明是**泛型 fn** ⇒ `mock collections.mapOf` 走 ch20 R1 既有 E1804 泛型类目（零码改，黄金钉住诚实边界）；成员面非模块 fn、非 mock 目标（E1804 类目论证入 design D7）。

非目标：

- **Map/Set 迭代发射**：`for (k,v) in m` / `for e in s` 维持今日态（检查过、发射停）——正典习惯形 = `keys()` + `get()`（ch17「快照自身固定一序」由 keys 的表序兑现）；发射打开是独立后续。
- **Builder**：裁决 2 明拒——mapOf/setOf 自由函数即全部构造面。
- **元组值表达式簇**：`List<(K,V)>` 不可构造是 B1 轨遗留，域外。
- **List 迭代/组合子语义改动**：`iterator()` 快照（`__we_list_snap` 复制）与十一组合子零触碰——句柄化后快照照旧复制数据块，语义不变。
- **std.concurrent 迁移、fs/process/string 面扩展**：B2a 域。
- **spec 触碰**：零 Requirement 增删、零诊断码、零 ADR——ch17 R104（"The method inventory beyond the anchored access family is the standard library's surface"）标准库表面不进规范；本变更「不改变语言行为」「无规范增量」（别名可见性是 ch17 已批语义的**兑现**，不是新语义）。
- **性能调优**：hash/rehash 按正确性先写；FPCR 基准归 M15 既有套件。
- **CLI/工具面、LSP、formatter**：零触碰。

## What Changes

- **`runtime/c/list.c` + `list.h`**：载体 v2——32B 句柄 + data 块分离；句柄描记 = 静态常量位图 `[1 x i64] [i64 1]`（bit 0 = 偏移 16——gc.c 约定 bit i 标记载荷字 16+8i，data 在载荷字 0）；`__we_list_new` 内部根桥（雕句柄后 `__we_root_push`、雕 data 前防收集窗口）；push 增长 = 雕新 data 块复制后**就地交换**句柄 @16、返回同一句柄；get/len 经句柄一跳；snap 答句柄裹复制；头注释与 push 契约句改写。
- **`runtime/c/coll.c`（新）**：Map/Set 载体（布局/探测/墓碑/rehash/描记两区/hash 族）+ 全部 `__we_coll_*` 面（map_of/set_of/put/get/remove/keys/size/add/has/list_get/list_remove_at）；C 侧自根纪律（入口处 receiver + traced 实参入根、出口弹尽——fs.c 先例）；mapOf 长度不匹配 `__we_task_fail`。
- **`stdlib/src/collections.we`（新）**：`pub fn mapOf<K, V>(keys: List<K>, values: List<V>) -> Map<K, V>` + `pub fn setOf<T>(items: List<T>) -> Set<T>` 虚构体（panic 体）；`stdlib.go` 的 embed glob 自动纳新（门两测随批扩键集）。
- **`internal/typecheck`**：`collectionMembers`（`:1356`）扩 13 员——List{iterator,get,add,removeAt,size} / Map{iterator,get,put,remove,keys,size} / Set{iterator,has,add,remove,size}；unit 返回形用 `unitType`；E0816 落空臂零改（表外员自然 E0816）。
- **`internal/codegen`**：成员调用臂（List/Map/Set 内建和式接收者 + 13 员名 → C 入口）；Option 族 out 三字组（fs fsEntries 发射臂全盘镜像）；`keys`/`mapOf`/`setOf` 返回句柄的调用点再扎根；keyed 表加 `collections.mapOf`/`collections.setOf`（域标签从 Site 的 K/V shape 读，映射失败 `e.bnd()` 诚实停）；import 走查 `case "collections"`（`:1769` 邻域）。
- **`internal/cli/build.go`**：运行时对象集加 coll.o（rt-fs/rt-process 链位先例）；`programModules` 零触碰（键控模块先例，T5 探针复核）。
- **conformance**：黄金矩阵（design D8）——别名可见性、三族成员往返、mapOf/setOf 构造、String 键 FNV、gc 值描记压测、长度不匹配陷阱、迭代停面负锚、E0816 边界负锚、mock 面、突变电池。
- **docs**：`docs/benchmarks.md`/`.zh.md` 运行面段（集合面在册 + 迭代停面披露）；roadmap B2b 行收口（双语，归档期）。

## 影响层

- **spec：零**。13 员是 ch17 R104 立场下的标准库表面；别名可见性是 ch17 已批 gc 语义（两句柄同一对象）的兑现；快照语义零触碰。「不改变语言行为」「无规范增量」。
- **compiler**：typecheck（成员表扩容）、codegen（成员臂 + keyed 扩容 + out 组 + 再扎根）、runtime/c（list.c v2 + coll.c 新）。零新诊断码（E0816/E1302/E1304/E0501 皆在册既有）。
- **stdlib**：`stdlib/src/collections.we` 第二批嵌入源（装载机制的第二个多模块消费者——io/test/time/fs/process/string 之后第七枚）。
- **docs**：benchmarks 双语运行面 + roadmap（归档期）。

## 影响范围

- **涉触码**：`runtime/c/list.c`、`runtime/c/list.h`、`runtime/c/coll.c`（新）、`runtime/runtime.go`（embed 集）、`stdlib/src/collections.we`（新）、`internal/typecheck/typecheck.go`（`:1356` 表 + unit 形）、`internal/codegen/codegen.go`（成员臂 + keyed + import 走查）、`internal/cli/build.go`（对象集 rt-coll；programModules 零触碰）、`internal/conformance/testdata/cases/`（新增）、runtime/codegen/stdlib 测试面。
- **既有黄金**：**零改写预期**——list.c 句柄化是 IR 文本不变的运行时改造（结构性论证：5 入口 + 零 gep + fs.c 忽略返回值 + collect 重赋值形成立）；任何意外漂移逐枚披露并须证明为行为等价。E0816 面只**收缩**（add/put 等从 E0816 翻检查过——今日语料零枚携带该形成员调用，grep 记档为证）。
- **对外零影响**：CLI 面、诊断注册表、LSP、formatter、`we doc` 零触碰；fs/process/string 行为零触碰。
- **`refr/` 禁区**：本变更的任何提交不得纳入 `refr/`。

## 勘查依据

勘查 + 真机探针（HEAD `13e9fe9`，二进制 `/tmp/b2b-bin/we`，项目 `/tmp/b2b-probe`）：

1. **成员面清单**：`collectionMembers`（`:1349-1372`）三 case 的锚定集、咨询点 `:6959` 的到达路、`stringMembers`（`:1342`）对照、E0816 注册表条目与 B2a T5 翻转先例（消息 title 起头 + `%q is not a member of %s`）。
2. **载体面清单**：`list.c` 全文（布局/install() 描记/push 增长契约/snap 复制/越界 task_fail）、`list.h` API 契约句、发射端 5 declare（`:208-211`）与 `emitCollect`（`:7346`）/ `emitListLit`（`:6566`）的重赋值形、`fs.c:300-305` 的 C 侧构造、T11-② 盒形、T12 再扎根先例、fs out 三字组发射臂。
3. **构造面清单**：`StdModule` 策略表（`:2481`）与 embed 键桥、import 走查 stdQuals case 集（`:1769-1777`）、`programModules`（`build.go:367`）白名单、fsEntries keyed 表形、M10b 槽机器对 keyed std fn 的键法、Sites/Shapes 的 K/V 读取面（B1b T4 先例）。
4. **真机探针（12 枚）**：P1 `xs.add(4)` → E0816；P2 `xs.get(1)?` → E1201（Option 不传播，get 在面的旁证）；P4/P5 裸 mapOf/setOf → E1304；P6 `import std.collections` → E1302 std 形；P7 `xs.removeAt(0)` → E0816；P3c `xs.get(1)`+match → check 0/build 70；P8 `m.put` → E0816；P9 `s.has` → check 0/build 70；P11/P12 Map/Set 迭代 → check 0/build 70。
5. **语料 grep**：今日语料零枚携带 13 员形成员调用（E0816 收缩的零爆炸半径证据，落地时记档）。

## 审计记录

### spec-impact audit（welang-spec-impact-audit 七条，2026-09-16）

**结论：通过**（7/7）。逐条：

1. **问题真实性：过**。三类可验证黑盒问题：① List 移动存储契约与 ch17 已批 gc 别名语义（「gc 别名是同一对象的两句柄」）**背离**——`let ys = xs; xs.add(4); ys.size()` 黑盒可判（今日语义错，规范语义对）；roadmap B2b 行点名（`docs/roadmap/0000-reference-implementation.md` B2 拆行，2026-09-16 B2a 归档提交 `13e9fe9`）。② 13 员成员面今日无一在面（P1/P7/P8 探针：E0816）——ch17 锚定族之外的方法清单按 R104 是标准库表面，缺口是工程缺口，可判。③ Map 值今日完全不可构造（P4–P6：字面量/裸名/std 皆无路径）——黑盒可判。规范引用在册：ch17（R104、别名语义、快照序、`.get` OOB→None）、ch14（task_fail 族）、ch15（预导入名、E1302/E1304）、ch16（效果段）、ch9（Never 豁免）、ch12（fn 值）。
2. **影响层声明：过**。`change.yaml` layers `[compiler, stdlib, docs]` 与 proposal 影响层逐项一致（spec 零 + compiler + stdlib + docs）。纯实现变更，双豁免标记「不改变语言行为」「无规范增量」在目标与非目标、影响层两处明写；validate.py `--all --strict` 已按豁免路径放行（specs/ 不要求，与 B2a 先例同路径）。
3. **规范增量范围：过**。零 Requirement 增删、零诊断码、零 ADR——ch17 R104 立场（方法清单是 stdlib 表面）明写于 design D0-2；别名可见性登记为**已批语义的兑现**非新语义。本变更涉及的全部诊断码（E0816/E1302/E1304/E0501/E1804/E1201）皆注册表在册既有码，对照 `docs/spec/diagnostics.toml` 无冲突、无新分配。唯一新名字（`__we_coll_*` C 符号族、`Map/Set` 数据块布局）是运行时/载体层事实，不进规范。
4. **原则一致性：过**。无隐式转换新增（mapOf 双列表 zip 形实参定域，`List<(K,V)>` 域外明写）；P1 局部可判定（键域门在发射侧、与 Float 载荷 reduce 同层位——检查器照常泛型代入，域外诚实停，非新分析路径）；P4 语义确定（开放寻址 + 表序快照，`keys()` 同调用内定序）；P2 表面最小（Builder 拒、13 员封闭清单、迭代发射不开）。无需 ADR 的原则突破：零。
5. **参考基线固定：过**。全部基线钉死：HEAD `13e9fe9`（B2a 归档提交，conformance 893 / codegen 单测 461 / 16 包）；探针二进制 `/tmp/b2b-bin/we`（HEAD 新建，非旧 `/tmp/b2b-we`）+ 项目 `/tmp/b2b-probe`；行号引 `codegen.go:207-211`（5 declare）、`typecheck.go:1349-1372`（collectionMembers）、`gc.c:4-10`（描记位约定）、`list.c:12-16`（移动存储契约自述）、`fs.c:300-305`/`process.c:47-48,121,128`（C 消费者全清单）。refr/ 草案零引用（本变更不触 v0.8 映射）；引用的规范章节全部是已批准 docs/spec/ 章（ch9/12/14/15/16/17）。
6. **验收边界：过**。目标六条皆机械可判：① 零 IR 漂移 = hello IR 双二进制 `cmp` 逐字节零差 + 893 枚零改写；② 别名可见性 = run 黄金 `ys.size()` → 4；③ 13 员 = check/run 黄金族逐枚退出码与 stdout；④ E0816 收缩边界 = 两枚负锚钉住不翻；⑤ mock = E1804 黄金；⑥ 域外停 = exit 70 负锚。非目标七条排除面足够（迭代发射/Builder/元组簇/spec/性能/CLI/LSP/std.concurrent）。
7. **粒度：过**。单一能力（集合面的真数据结构），compiler+runtime+stdlib 三层是**同一能力的三面**非三个不相关层——B2a 先例（fs+process+string+装载机制同变更）同构；T1–T7 每任务垂直可验（T1 纯运行时可独立验收、T3 检查器可独立黄金锚、T4/T5 按可观测性切两刀）。

**开放项（进任务书已带守门，此处登记）**：

- **O-1 虚构体可检查性（含泛型 `Map<K,V>` 返回解析）**：嵌入源里「泛型 fn + 泛型容器返回位」今日零先例（io/test/time 无返回、fs/process 非泛型、string 非容器返回）——`collections.we` 落盘首跑 `Check` 是否干净（`Map<K,V>` namedType 实参代入、panic 体 Never 满足返回位）由 T5 门两测红先实证；不净则按 B2a T4 先例三出口处置并披露。
- **O-2 fs.c push 契约透明性**：`fs.c:300-305` 忽略 push 返回值是代码读得的事实，但「句柄化后零改动即过」是 T1 的验收断言——C harness 别名面 + `run-fs-listdir-regression` 黄金双钉，未验前不算数。
- **O-3 programModules 腿必要性**：collections 键控模块零触碰 programModules 的立场（fs/process 先例）是结构性论证，T5 探针复核（若限定调用解析非预期停，按 string 先例补腿 + 披露）。
- **O-4 链式接收者**：`mapOf(...).put(...)` 零先例，T5 探针定形（停则披露、绑定后通为正典形）。
- **O-5 M-d 结构性静默预期**：突变电池 M-d（墓碑复用摘除）预判黄金层结构性静默、以单测钉 `M_TOMB` 收缩语义——诚实预期随判决表记档，不冒冒充黄金层捕获（T11 M2 先例）。

### welang-change-review 十条（ready → active 关卡，2026-09-16）

**结论：通过**（10/10，零打回项）。逐条：

1. **proposal 职责：过**。Why = 黑盒问题三类（别名语义背离可黑盒判 / 13 员不在面 E0816 / Map 值不可构造），探针证据在册；What Changes/影响范围仅列涉触面与验收线（B2a 先例同构）；实现权威让渡 design（proposal 开头声明「权威：proposal（范围）+ design（D0–D8）」见 design 头）。今日事实段的行号级勘查是证据非决策。
2. **spec 增量：过（N/A 正当声明）**。零规范增量走双豁免标记（B2a 同路径）；无 specs/ 目录、无 BCP14 义务面待检——豁免本身在目标与非目标、影响层两处明写，不属「无行为增量的包装层」。
3. **design 唯一最小路径：过**。被拒替代三处在册（D1-8：IR 层重赋值/双字句柄/句柄内嵌 len；D6：C 结构体返回/变参入口/迭代器入口；D0-2：Builder 拒于裁决 ②）——每处带拒绝理由；规范引用精确（ch17 R104、ch9 Never 豁免、ch14 task_fail、gc.c :4-10 位约定）；与零 spec 增量立场无矛盾（D0-2 专门论证别名可见性 = 已批语义兑现）。
4. **tasks 职责：过**。每勾选项自带 来源：/验证： 两行（strict 已验证）；无 deferred/non-goal 词形；T5 阻塞前置项的三出口是**带守门的开放问题**（见第 10 条）非未决方案走私。
5. **场景覆盖：过**。normal（三族成员往返）/ boundary（OOB→None、长度不匹配陷阱、rehash 触发界、E0816 收缩边界）/ failure-degradation（域外键 exit 70、迭代误开负锚、gc 风暴压测）三路皆有黄金；无 only-happy-path 面。
6. **无空章节：过**。D0–D8 各节皆载重；无 N/A 表格。
7. **测试先行：过**。红先纪律头部行 + 逐枚红态三形（E0816/E1302/build 70）记档于 D8-1 表；T1 作为纯运行时重构其 characterization = hello IR 双二进制 cmp + 893 枚零改写（重构先行建立的通过性特征）。
8. **负向断言：过**。真实负例四族：`check-e0816-list-push`/`check-e0816-map-values`（防收缩过宽）、`build-bnd-map-iteration`（防 T5 误开迭代）、`build-bnd-mapof-{key,value}-domain`（域外停）、mock 面 E1302→E1804 翻转钉——全部构造违规样例断言拒绝，非文字代替。
9. **完成度闭环：过**。类型检查（D4 13 员表）/ 代码生成（D5 发射 + D6 ABI）/ 运行时（D1 句柄 + D2 载体）三要素齐备，原则 10 兑现；装载（D3）与 mock（D7）作为 stdlib 层第四要素在册。
10. **未决问题阻塞：过**。审计开放项 O-1–O-5 全部映射到任务守门：O-1/O-3/O-4 → T5 阻塞前置项「未定不得勾其后各项」；O-2 → T1 的 C harness + fs 回归黄金两项未勾；O-5 → T6 判决表记档义务。无 SHOULD/MAY/TODO 掩盖形。

**发现与处置**：F1（措辞级，随审订正）：audit 记录 O-5 原文「不冒冒充」笔误 → 已订正为「不冒充」。此外零内容级发现。

## 审查记录

### welang-code-review 七条（2026-09-16）

**结论：通过**（7/7；发现三项 F1/F2/F3 均已随审处置——F1/F2 为审查期 docs 独立提交，F3 为记录原位订正）。逐条：

| # | 条目 | 结论 | 证据 |
| --- | --- | --- | --- |
| 1 | 规范符合性 | 过 | `docs/spec/` 与 `diagnostics.toml` 对 `13e9fe9..HEAD` 零 diff（双豁免标记兑现）；ch17 锚逐枚核验——R1 gc 别名（`run-list-alias-visibility`：两句柄一对象）、R3 `.get` 越界→None（`run-list-members`）、`Map.get→Option<V>`/`Set.has→Bool`（13 员黄金族）、R5 变更方法别名可见性；ch20 R1 mock 目标类目（`test-mock-mapof-e1804`：泛型 fn → E1804，消息头与注册表 title 逐字）；Map/Set 迭代发射维持申报非目标（`build-bnd-map-iteration` exit 70）——ch17 R1 的 Iterable 面按 D0-3 留作 follow-up #26 |
| 2 | 验证诚实性 | 过（含 F3 订正） | 26 枚新金逐提交重数（893→919，`git diff --name-only` 六提交并集）；16 枚 B2b 定向金 `-count=1` 复跑绿（5.394s）；`go test ./runtime`（14.767s）、`./stdlib`、`./internal/typecheck` 绿；codegen 包逐提交对账 `grep -h '^func Test'` = 461（`13e9fe9`）+ 4（T4）+ 9（T5）= 474，T6 零增量——T5 落地记原载 466→475 双端各差一，已订正（见 F3）；全阶梯见第 7 条 |
| 3 | 测试先行 | 过 | 六份落地记皆载红先记录（T5 三态：真 HEAD E1302 → 源落盘未接线 build 70 → 接线后绿；T6 E1302→E1804；T1 双向特征钉 = 改前绿态先取 + 改后再绿）；26 枚黄金 JSON 键形合 §62 协议、文本合 §52 人类可读形 |
| 4 | 诊断协议 | 过 | `internal/diag/` 零 diff、零新码（涉及码 E0816/E1302/E1304/E0501/E1804/E1201 全部注册表在册既有码）；E0816/E1804 消息以注册表 title 起头（`check-e0816-list-push` / `test-mock-mapof-e1804` 黄金字节即证）；`--json` 字段零触碰 |
| 5 | 单一权威 | 过（含 F1/F2 处置） | B2b 全部改动文件 CJK 注释 0（`coll.c/coll.h/list.c/list.h`、b2b 两测、`collections.we`、diff 增行全扫）；13 员清单/载体布局/域标签的长期权威 = design 实现期补记 + `stdlib/src/collections.we` + `runtime/c/{list,coll}.c` 头注释（ch17 R104 立场：方法清单是 stdlib 表面，不进 spec）；docs 欠账与陈旧例句见 F1/F2 |
| 6 | 红线 | 过 | 六提交 diff 42 文件全在 proposal 影响层内（compiler/runtime/stdlib/conformance）；`refr/` 零文件；提交信息零 trailer；无与非目标冲突的越界改动 |
| 7 | 最小可信验证 | 过 | 全阶梯复跑：`go build`/`go vet` 绿、`gofmt -l` 0 文件、`go test -count=1 ./...` 16 包全绿（conformance 919 枚零改写）、`validate.py --all --strict` 过、`docs_sync.py` 33 对齐、`git diff --check` 净、staged 无 `refr/`；`WE_UPDATE_GOLDEN` 全程未用 |

**发现与处置**（三项；F1/F2 落审查期 docs 提交，F3 落地记原位订正）：

- **F1（docs 欠账，已补齐）**：proposal「What Changes」承诺的 docs 义务——「`docs/benchmarks.md`/`.zh.md` 运行面段（集合面在册 + 迭代停面披露）」——T1–T6 未落：tasks.md 无 docs 项，各落地记的「零 docs 触碰，T_n 无文档面」逐任务为真而变更级义务失落。处置：审查期双语补齐——运行面段新增 stdlib-collections 拓宽句（稳定句柄别名可见性、13 员成员面、mapOf/setOf 构造与 FNV 串键、长度不匹配陷阱、签名单字过界与双区描记、keys+get 正典习惯形）+ 停面披露四条（Eq 键域门与值域单字、String 载荷成员 Option 停、直接迭代按申报非目标停、链式接收者先绑定规则）。
- **F2（陈旧例句，已订正）**：运行面段 E0816 翻转句的例 `xs.add(1)` 自 T3 起失真——add 已是成员、check 0 非诊断。换 `xs.push(1)`（负锚黄金 `check-e0816-list-push` 恰钉此形），双语同改。
- **F3（计数记录订正，零码改）**：T5 落地记「codegen 包 466→475」双端各差一（delta +9 本身正确）；基线实为 461（`13e9fe9` 逐文件数）+ T4 四测 = 465 + T5 九测 = 474，今 `go test -v` 顶层 `--- PASS:` 计 474、零 SKIP/零 FAIL 互证。落地记行已原位订正（订正句内注明对账链）。
