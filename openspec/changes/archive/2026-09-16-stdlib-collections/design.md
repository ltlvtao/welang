# design — stdlib-collections（B2b）

> 权威：proposal.md（范围与勘查）。本文 D0–D8 为实现前设计；实现期逐任务补记（「实现期补记一」起）追加于文末。

## D0 范围、切片与规范立场

### D0-1 范围与五裁决落点

范围一句话：集合面的真数据结构——List 载体句柄化（别名可见性）、Map/Set 新载体、mapOf/setOf 构造面、13 员成员面（检查 + 发射）、mock 面钉边界。

| 裁决 | 设计落点 |
| --- | --- |
| ① List 载体 = 稳定句柄间接层 | D1（runtime/c/list.c 内部改造，五入口签名不变，零 IR 漂移） |
| ② 构造 = mapOf/setOf 自由函数、无 Builder、虚构体 + keyed C | D3（装载）、D5-2/D6（keyed 与 ABI） |
| ③ 13 员成员清单与返回形 | D4-1（检查表）、D5-2（发射 ABI 表） |
| ④ 键域 = Eq 域、域外发射侧诚实停 | D2-2（hash 族）、D3-4/D5-5（域标签与停点） |
| ⑤ mapOf 双列表 zip 形、长度不匹配 = 运行期陷阱 | D2-6（task_fail）、D3-1（源形）、D8（陷阱黄金） |

### D0-2 规范立场

- ch17 R104：锚定访问族之外的方法清单是**标准库表面**，不进规范。13 员、mapOf/setOf 签名、载体布局、hash 族全部是 stdlib 层事实；其权威载体 = 本设计 + `stdlib/src/collections.we` + `runtime/c/{list,coll}.c`。
- 别名可见性不是新语义——ch17 已批「gc 别名是同一对象的两句柄」；今日 List 的移动存储契约（push 返回新身份）是**实现与规范语义的背离**，B2b 修正实现向规范对齐。
- 快照语义零触碰：`iterator()` 的 `__we_list_snap` 照旧复制活前缀（复制的是 data 块，句柄化后仍是新句柄裹新数据块）。
- 双豁免标记：「不改变语言行为」「无规范增量」——零 Requirement 增删、零诊断码、零 ADR。

### D0-3 不打开的面（披露义务）

- **Map/Set 迭代发射**维持今日态（检查过、发射停，P11/P12）——正典习惯形 = `keys()` + `get()`；ch17「快照自身固定一序」由 keys 的表序兑现（同一次 keys() 调用内表序确定；跨 put/remove 的序不保证）。
- List 迭代/组合子语义零触碰；Builder 拒（裁决 ②）；元组值表达式簇域外（B1 轨遗留）。
- Map/Set 的**值域** = List 元素域（单字）。和式（Option/Result/用户 sum）值与元组值今日即不在 List 元素域（多字），B2b 同域外——发射侧诚实停，D8 负锚钉住。

## D1 List 载体 v2（稳定句柄）

### D1-1 今日态

`runtime/c/list.c`（132 行）：单块载体——冻结头 `{map@0, size@8}` + 三载荷字（`L_LEN`@16 / `L_CAP`@24 / `L_TRACED`@32）+ 元素字自 @40（槽 k 在字 k+2）。`__we_list_push` 满载时按 `len?len*2:4` 倍增、雕新块复制、**返回新身份**；头注释 :12-16 自述移动存储契约。别名可见性因此不成立（`let ys = xs` 后的增长对 ys 不可见）。

### D1-2 布局与描述符

v2 = **32B 句柄 + data 块**两层：

- **句柄**：`{map@0, size@8=32, data@16, 保留字@24（恒 0）}`。`data@16` 指向数据块。句柄描记 = **静态常量位图 `[1 x i64] [i64 1]`**（bit 0 标记偏移 16 的 data 指针）——gc.c 头注释 :4-10 的约定是 **bit i 标记载荷字 16+8i**，data 在载荷字 0 ⇒ bit 0 ⇒ 值 1。（T5 的 `@.dynmap [i64 2]` bit 1 = 偏移 24 同一约定互证。）静态常量放 list.c 内 `static` 数组，描述符本就从不回收，与 install() 的 malloc 位图同寿命语义。
- **数据块**：**今日布局原样**——冻结头 + `L_LEN/L_CAP/L_TRACED` + 元素自 @40；alloc 尺寸 `16 + 8*(L_ELEM+cap)` 不变；`install()`（运行期按容量建位图；标量域 map=NULL）原样复用。描记语义全在数据块自身的 map 字上，句柄层零参与。

入口行为：

- `__we_list_new(cap, traced)`：雕 32B 句柄 → 写静态描述符 → **根桥**（见 D1-3）→ 雕数据块 → 初始化三载荷字 + `install()` → 句柄 @16 存 data 指针 → 弹根 → 返回句柄。
- `__we_list_push(l, v)`：经句柄取 data；满载则倍增雕新数据块、复制、`install(nd, traced)`、**就地交换句柄 @16**、旧块交清扫；元素写入；**返回同一句柄**。
- `__we_list_get(l, i)` / `__we_list_len(l)`：经句柄一跳；越界陷阱与报文不变。
- `__we_list_snap(l)`：`__we_list_new(len, traced)` 造新句柄、复制活前缀、返回新句柄——快照语义不变。

### D1-3 内部根桥

`__we_alloc` 入口即可能触发收集（`GC_THRESHOLD = 1<<20`，收集先于雕块）。v2 的两段分配（句柄、数据块）之间开了收集窗口：句柄在此刻**只从 C 局部变量可达**——影子栈看不见 C 局部。窗口闭合：

- `__we_list_new`：雕句柄后 `__we_root_push(h)`，雕完数据块并接线后 `__we_root_pop()`。
- push 增长路径：接收者**由调用方**跨此次分配入根（既有契约，list.c :96-101 注释在案；根着的句柄经其描述符标记旧 data 块与元素目标，复制期间旧块存活）。新数据块雕出后零化、复制、交换——**无需额外根桥**（新块在交换前无别名，旧块经句柄可达）。
- 数据块未接线时句柄 `data@16 = 0`（分配器零化）：描记位指向 NULL 槽——数据块未用容量槽同形（零 + 被描记），收集器已容忍。

### D1-4 零 IR 漂移的结构性论证

发射器对 List 载体的**全部**接触面（grep 实证，HEAD `13e9fe9`）：

1. **5 个 declare**（`codegen.go:207-211`）：new/push/get/len/snap——签名零变（push 仍 `ptr @…(ptr, i64)`，只是返回同一句柄）。
2. **调用点五簇**：`:6290-6316`（list 走查：snap/len/get）、`:6548-6566`（emitListLit：new + push 重赋值）、`:6919-6947`（第二走查同形）、`:7327-7377`（emitCollect：new + push + 根替换对）、`:12229-12242`（ListIter 对象 next 体的内联文本：len/get）。
3. **零 gep 进载体内部**——一切访问经入口函数；`:12229` 簇的 `+16` gep 是迭代器**对象**槽位，非载体。

同句柄下：emitListLit 精确容量（零增长）+ 重赋值成恒等；emitCollect 的「pop 旧根 / push 新身份 / 存 r」根替换对退化为同指针 pop/push/store（文本不变）；`fs.c:300-305` 按精确容量构造并**忽略 push 返回值**——三处调用点全部逐字成立。⇒ **list.c v2 是运行时内部改造，IR 文本不变**；既有语料行为逐字节绿是验收线（hello IR 对拍 + 全量语料）。

### D1-5 C 侧消费者（全清单，零改动）

- `fs.c:300,305`：`__we_list_new(count,1)` + `__we_list_push`（忽略返回）；句柄最终作 Ok 载荷字返回。
- `process.c:47-48,121,128`：extern 声明 + `__we_list_len` / `__we_list_get` 读 argv 载体。
- 全部经入口函数，无直访载体内部者——句柄化对二者透明，**零改动**。

### D1-6 语义面：别名可见性与快照

- **别名可见性**（roadmap 点名项）：`let ys = xs; xs.add(4); ys.size() == 4`——两个绑定各持一个 32B 句柄，指向同一数据块；增长交换只改句柄内 data 指针，句柄本身不动。源级 run 黄金在 T4 落（add 面就绪才可观测）；T1 以 C harness 钉（同句柄两次取 data 见增长、跨句柄可见）。
- **快照**：`iterator()` 的 snap 复制照旧——`for x in xs { xs.add(...) }` 迭代器不受后续增长影响（ch17 快照语义）。既有语料的迭代行为是回归面。

### D1-7 注释改写（英文）

list.c 头注释 :1-16（移动存储契约自述）与 list.h 的 push 契约句（"must take result as new identity"）改写为句柄契约：push 返回同一句柄、增长就地交换、调用方仍须跨增长入根接收者。`codegen.go:7327` 附近的家族注释若含 "storage moves" 措辞一并订正（英文）。

### D1-8 拒绝的替代

- **IR 层重赋值真做**（保持移动存储，靠发射器每次 push 后重绑）：改 5 处调用簇的 IR 形、且绑定别名（`let ys = xs`）需要引入间接槽——规范语义（两句柄同一对象）兑现成本高于运行时间接层，且违反「零 IR 漂移」的既有语料稳定线。拒。
- **双字句柄 `{ptr,len}` 值形**：破坏 List 的 gc 单字 ABI（`fitAbi` 现判 abiGc），全线翻动。拒。
- **句柄内嵌 len/cap**（数据块只放元素）：get/len 每次经句柄快一步，但快照/增长要双写两处状态；不采——32B 句柄裁决 ① 已定形。

## D2 Map/Set 载体（runtime/c/coll.c）

### D2-1 数据块布局

与 List 同构的两层：**32B 句柄**（形同 D1-2，`{map@0, size@8=32, data@16, 保留@24}`，静态描记 `[i64 1]`）+ **数据块**：

```
Map 数据块（payload 自 @16）：
  @16   M_LEN     活条目数
  @24   M_CAP     容量（2 的幂，≥ 8）
  @32   M_TOMB    墓碑数
  @40   M_KDOM    0 = 恒等键域（8 整型 + Bool），1 = String（盒句柄）
  @48   M_VTRACE  值字是否 gc 引用
  @56   occ[cap]  占用字节（0 空 / 1 活 / 2 墓碑），补齐到 8 的倍数
  @56+P keys[cap] 键字
  @56+P+8*cap vals[cap] 值字
Set 数据块：M_LEN/M_CAP/M_TOMB/M_KDOM 四头 + occ + slots[cap]（无 vals）。
```

alloc 尺寸：Map `16 + 8*5 + P + 8*cap*2`；Set `16 + 8*4 + P + 8*cap`。M_KDOM/M_VTRACE 落块内使 put/get/remove/keys 自描述（无需实参携带域标签）。

**共享核**：hash/相等/查找/插入/rehash 以静态助手实现于一份小视图（len/cap/tomb/kdom/occ/keys/vals-or-NULL）上，Map/Set 两个公开面族共用——「一类事实一个位置」。

### D2-2 键域与 hash 族（裁决 ④）

- **KDOM_ID（0）**：8 整型（窄整型经符号/零扩展后字与值双射）+ Bool（0/1）。hash = 恒等（字本身），相等 = 字比较。永不描记。
- **KDOM_STR（1）**：String 键 = T11-② 32B 盒句柄（`{map@0=null, size@8, strptr@16, strlen@24}`）。hash = **FNV-1a 64** over `(strptr, strlen)` 字节；相等 = 字节串内容比较（We 字符串不驻留——两同容串是不同盒，必须按内容比）。**必须描记**（盒是 gc 块）。
- 域外（Float64、Rune、记录、和式、元组……）：检查器照常泛型代入（K 任意合法类型），**域门在发射侧**（D3-4/D5-5 的域标签映射失败 → `e.bnd()`）——与 Float 载荷 reduce 等既有面同层位。Eq 域与 T10 assertEqual 的叶子域**逐字一致**（8 整型 + Bool + String），一处两用。

### D2-3 探测、墓碑与 rehash

- 开放寻址 + **线性探测**：`idx = h & (cap-1)`，步进 +1 回绕。
- **查找**：跳过墓碑（occ=2），遇空（occ=0）即未命中，活且键等即命中。
- **插入**：记住首个墓碑位（复用），否则首个空位写 occ=1；写键字/值字。
- **删除**：命中位 occ 置 2（墓碑），M_LEN−1、M_TOMB+1——键字不清（描记位仍指向它；墓碑字不再是活键，但标记一个陈旧句柄只保守多活一轮，语义安全）。
- **rehash 触发**：`(M_LEN + M_TOMB) > cap*3/4` → 容量倍增、重插活条目（新表零墓碑）、重建描记、**就地交换句柄 @16**、旧块交清扫（与 List 增长同一姿态）。
- mapOf/setOf 初容量：`cap = 8; while (len > cap*3/4) cap <<= 1;`（构造即不触发 rehash）。

### D2-4 描记两区

数据块描记 = 运行期 install 式位图（构造/rehash 时按 cap 建，永不回收）：

- **键区**描记 iff `M_KDOM == 1`（String 键是盒句柄，必须标；盒自身 map=null——内容字节在 gc 域外，标到盒即止）。
- **值区**描记 iff `M_VTRACE == 1`——判定谓词 = **T11 的 `traces()`**（`face.gc || face.kind == skStr`）原样复用：gc 句柄与 String 盒描、标量（含 Float64 位型）不描。
- Set 的槽区描记 iff `S_KDOM == 1`。
- **订正入册**：设计早期速写「Eq 域键永不描记」对 String 为误——String 键是盒句柄、必须描记；本节为权威表述。
- 位图位号：键槽 j 的位 = `5 + P/8 + j`（P = occ 补齐字节数）；值槽 j 的位 = `5 + P/8 + cap + j`（Map）。

### D2-5 C 侧自根纪律（fs.c 先例）

C 代码对**自己跨分配持有的对象**入根，出口弹尽、配对平衡：

- `__we_coll_map_of/set_of`：入口 `root_push(keys 载体)`（+ `root_push(vals 载体)`）→ 雕句柄（内部根桥）→ 雕数据块 → 灌条目 → 弹尽 → 返回句柄。
- `put/remove`（可能 rehash）：入口 `root_push(接收者句柄)`；trace 实参字（kdom==STR 的键盒、vtrace 的值句柄）在 rehash 分支内入根/弹根——实参尚未入表时其存活是调用方纪律（构造根/绑定根），C 侧根为纵深（fs.c 先例：入口 receiver + traced 实参入根）。
- `keys`：`__we_list_new(len, kdom==1)` 精确容量 + 单次 `root_push`（fs.c 构造协议先例）逐键 push 后弹根；键字读自被根着的表——经描记可达。

### D2-6 陷阱（裁决 ⑤）

`mapOf` 两表长度不匹配 → `__we_task_fail("mapOf length mismatch")`（ch14 族；list.c 越界与窄整型溢出同入口）。黄金钉退出码与报文（形随既有 task_fail 黄金；红先态 = 今日 E1302，绿态手写自真机）。

## D3 构造面与 std.collections 装载

### D3-1 源形（stdlib/src/collections.we）

```we
pub fn mapOf<K, V>(keys: List<K>, values: List<V>) -> Map<K, V> {
    panic("fiction")
}

pub fn setOf<T>(items: List<T>) -> Set<T> {
    panic("fiction")
}
```

- `panic` 预导入名（ch15 R2）；**Never 满足任意返回位**（ch9 豁免）→ 检查干净；panic 族零效果段（ch16）→ 无 E1401 面。
- `Map/Set/List` 预导入名（ch17）→ std 源内无需 import。
- 泛型 fn：`K/V` 经 M6a 统一从两表实参的元素类型推断；返回 `Map<K,V>` 实例化。
- 嵌入 glob 自动纳新（`//go:embed src/*.we`）；门两测随批扩键集（`stdlib_test.go` 键集 + b2a 行——B2a T1 先例）。

### D3-2 装载接线（keyed 先例 = fs/process，非 string）

- **import 走查**：`codegen.go:1769` 邻域加 `case "collections"` 落 stdQuals——`collections.` 限定符可解，keyed 拦截才能按键命中。模块无 SumDecl/RecordDecl，注册腿自然空转。
- **programModules 零触碰**：collections 是 keyed 模块（虚构体、零 define），不乘程序面——fs/process 先例（string 需要白名单只因它有真体 define）。
- `internal/cli/build.go` 唯一涉触 = 运行时对象集 rt-coll 链位（rt-fs/rt-process 先例）。
- T5 探针守门：若 `collections.mapOf(...)` 限定调用解析出现非预期停点，按 string 先例补 curImports 腿并披露（预期不需要——keyed 拦截先于成员解析）。

### D3-3 泛型限定调用的推断缝（红先探针定形）

泛型 std fn 的限定调用今日语料零先例（`std.test.assertEqual` 走 importCall 前置拦截，非成员解析路）。`collections.mapOf(ks, vs)` 应走：resolveQual("collections") → importSym pub 门取泛型条目 → M6a 泛型推断 K/V → Site 登记。装载层与用户模块同机（StdModule 经同一 ingest），**预期无缝**；T5 红先探针实证，缝若在装载层按 B2a T4 先例参数化修复并披露。

### D3-4 域标签（从 Site 读）

`__we_coll_map_of` 的 `kdom`/`vtrace` 与 `__we_coll_set_of` 的 `kdom` 是发射侧从 **Site 的 K/V shape** 读出的 i64 即时量：

- K shape ∈ {8 整型, Bool} → kdom 0；String → kdom 1；其余 → `e.bnd()`（裁决 ④ 域外停）。
- vtrace = `traces(V)`（D2-4 谓词）；Set 的槽域 = 元素域同判。

## D4 检查器成员面

### D4-1 13 员表（collectionMembers 扩容）

`typecheck.go:1349-1372` 三 case 扩为（接收者 namedType 实参代入 K/V/T，机制同既有 get/has）：

| 族 | 成员 | fnType |
| --- | --- | --- |
| List\<T\> | `add` | `fn(T) -> ()` |
| | `removeAt` | `fn(Int64) -> Option<T>` |
| | `get`（既有） | `fn(Int64) -> Option<T>` |
| | `size` | `fn() -> Int64` |
| | `iterator`（既有） | 既有 |
| Map\<K,V\> | `put` | `fn(K, V) -> ()` |
| | `remove` | `fn(K) -> Option<V>` |
| | `get`（既有） | `fn(Int64) -> Option<V>`（索引型随既有 get——Int64） |
| | `keys` | `fn() -> List<K>` |
| | `size` | `fn() -> Int64` |
| | `iterator`（既有） | 既有 |
| Set\<T\> | `add` | `fn(T) -> ()` |
| | `remove` | `fn(T) -> Bool` |
| | `has`（既有） | `fn(T) -> Bool` |
| | `size` | `fn() -> Int64` |
| | `iterator`（既有） | 既有 |

- void 返回用 `unitType{}`（M3 在册）；`keys` 的 `List<K>` namedType 构造自接收者实参。
- 零效果段（纯内存操作——与 iterator/get/has 今日态一致）。
- E0816 落空臂零改：表外员（如 `push`、`values`、`clear`）自然 E0816。

### D4-2 E0816 收缩边界

今日 13 员无一在面（P1/P7/P8：add/removeAt/put → E0816）。扩容后 E0816 面**只收缩**：负锚黄金钉 `xs.push(4)` / `m.values()` 仍 E0816（防翻转过宽）。既有语料零枚携带 13 员形（grep 记档），零爆炸半径。

### D4-3 域外键的层位

检查器**不做**键域检查（`Map<Float64, Int64>` 检查过）——域门在发射侧（D3-4/D5-5），与 Float 载荷 reduce 同层位（分类塔诚实停，非检查器新码）。`check-*` 黄金因此只锚成员面本身；域外停以 `build-bnd-*` 黄金锚。

## D5 成员与构造发射

### D5-1 接收者字与实参字

- **接收者字**：List/Map/Set 绑定是 gc 句柄（单字 ptr）——绑定槽 / 形参槽 / 再扎根后的调用结果，一律 `loadPtr` 取句柄。链式接收者（`mapOf(...).put(...)`）T5 探针定形，停则披露（T8-3 先例：绑定后即通）。
- **实参字**：复用 T11 元素字管道——标量（含 Bool zext、窄整型扩展）原位 i64；Float64 位型 bitcast double；**String 装盒**（T11-② 32B 盒，字面量/绑定两路既有）；gc 句柄直过。元素/键/值同管道（`emitListOperand`/`listElemWord` 族）。

### D5-2 ABI 表（13 员 + 2 构造）

| We 面 | C 入口 | 签名 | 返回处理 |
| --- | --- | --- | --- |
| `xs.add(e)` | `__we_list_push`（既有） | `ptr(ptr, i64)` | 弃返回（void 面） |
| `xs.removeAt(i)` | `__we_coll_list_remove_at` | `void(ptr, i64, ptr out)` | out 三字组 → Option\<T\> |
| `xs.get(i)` | `__we_coll_list_get`（新，非 `__we_list_get`） | `void(ptr, i64, ptr out)` | out 三字组（OOB → None） |
| `xs.size()` | `__we_list_len`（既有） | `i64(ptr)` | 直入 |
| `m.put(k, v)` | `__we_coll_map_put` | `void(ptr, i64, i64)` | — |
| `m.get(k)` | `__we_coll_map_get` | `void(ptr, i64, ptr out)` | out 三字组 |
| `m.remove(k)` | `__we_coll_map_remove` | `void(ptr, i64, ptr out)` | out 三字组 → Option\<V\> |
| `m.keys()` | `__we_coll_map_keys` | `ptr(ptr)` | **再扎根**（T12 先例） |
| `m.size()` | `__we_coll_map_size` | `i64(ptr)` | 直入 |
| `s.add(t)` | `__we_coll_set_add` | `void(ptr, i64)` | — |
| `s.remove(t)` | `__we_coll_set_remove` | `i64(ptr, i64)` | Bool |
| `s.has(t)` | `__we_coll_set_has` | `i64(ptr, i64)` | Bool |
| `s.size()` | `__we_coll_set_size` | `i64(ptr)` | 直入 |
| `collections.mapOf(ks, vs)` | `__we_coll_map_of` | `ptr(ptr, ptr, i64, i64)` | **再扎根** |
| `collections.setOf(xs)` | `__we_coll_set_of` | `ptr(ptr, i64)` | **再扎根** |

**`xs.get` 走新入口的理由**：载体 `__we_list_get` 越界**陷阱**（"List element read out of range"），ch17 表面语义是 OOB → None——语义分叉，两入口并存（载体入口服务走查/迭代器内联，永不越界；表面入口服务用户代码）。

### D5-3 out 三字组（fs 先例全盘镜像）

Option 返回族（removeAt/get/map get/map remove）= 三字和 `{i64,i64,i64}` 24B 超 SysV 16B MEMORY 界 → `void f(ptr recv, i64 key, ptr out)`，C 侧写三字（tag/pay0/pay1；载荷单字入 pay0，pay1 恒 0）；发射臂 = fs `emitFsEntryCall` 镜像（alloca 三字、call、按熔合表 [Some/None] 物化 sum 值）。tag 索引从**被调方签名携带的和式表**读（`slotVariantIndex`——检查器 [Some,None] 与 codegen [None,Some] 相反的 T7-A 教训）。

### D5-4 返回句柄再扎根

`keys`/`mapOf`/`setOf` 返回 gc 句柄：调用点后**立即 `__we_root_push`**（T12 同步型返回位先例——C 侧构造根在自身出口弹尽，调用方补根直至语句/绑定出口）。

### D5-5 域外停

K/V 域标签映射失败（D3-4）或实参字管道不支（sum/元组值）→ `e.bnd()` 诚实停（exit 70，边界词随语境）。`build-bnd-*` 负锚钉：`Map<Float64, Int64>` 构造、`Map<String, Option<Int64>>` 值域。

## D6 runtime ABI 符号表

新增 C 入口全表（`runtime/c/coll.c`；declare 表随 keyed/成员臂入 `codegen.go:207` 邻域）：

```
void  __we_coll_list_get(ptr, i64, ptr out)      # OOB → None（表面语义）
void  __we_coll_list_remove_at(ptr, i64, ptr out) # 移位删除 → Option<T>
void  __we_coll_map_put(ptr, i64, i64)
void  __we_coll_map_get(ptr, i64, ptr out)
void  __we_coll_map_remove(ptr, i64, ptr out)
ptr   __we_coll_map_keys(ptr)
i64   __we_coll_map_size(ptr)
void  __we_coll_set_add(ptr, i64)
i64   __we_coll_set_remove(ptr, i64)
i64   __we_coll_set_has(ptr, i64)
i64   __we_coll_set_size(ptr)
ptr   __we_coll_map_of(ptr keys, ptr vals, i64 kdom, i64 vtrace)
ptr   __we_coll_set_of(ptr items, i64 kdom)
```

拒绝的替代：C 结构体返回（24B 隐式 sret——fs D7-2 已拒）；键值对变参入口（需元组值表达式簇，域外）；Map/Set 迭代器 C 入口（迭代发射不开，D0-3）。

## D7 mock 面

- **mapOf/setOf**：keyed std fn 骑槽（fsEntries 同形——键 `collections.mapOf`/`collections.setOf`，restore 指向 C 符号）。**但声明是泛型 fn**——ch20 R1 mock 目标 = 模块级**单态** fn，泛型类目 E1804。⇒ as-built：`mock collections.mapOf` → **E1804**（泛型类目，零码改），黄金钉住该诚实边界；槽基础设施按 fs 形 uniform 落位（不为 E1804 另开无槽 keyed 变体——一类事实一个形状；未来开放泛型 mock 即免费）。
- **13 成员面**：内建类型成员调用，非模块 fn——不在 mock 目标解析面（`xs.add` 解析为成员调用而非限定名），E1804 的类目论证落此：mock 的单位是模块 fn，内建成员无模块键。非目标，不设码。
- process.run 先例延续：成员/构造面在本变更零 mock 开放承诺。

## D8 黄金矩阵、突变电池与红先纪律

### D8-1 黄金矩阵（新增，全部手写、实现前取红态）

| 黄金 | 面 | 红态（今日 HEAD） |
| --- | --- | --- |
| `run-list-alias-visibility` | `let ys = xs; xs.add(4); ys.size()` → 4（roadmap 点名） | build 70（add E0816→T3 后 check 过仍 build 70） |
| `run-list-members` | add/removeAt/get（含 OOB→None）/size 往返 + println | 同上 |
| `run-map-members` | mapOf 构造 + put/get/remove/keys/size 往返（Int 键 + String 键各一段） | E1302（mapOf）|
| `run-set-members` | setOf + add/has/remove/size 往返 | E1302 |
| `run-map-string-keys` | String 键碰撞/覆盖/删除后重插（FNV + 墓碑路径） | E1302 |
| `run-map-gc-crossing` | gc 记录值 + String 键，churn 越 1 MiB 阈值后读回（描记两区压测） | E1302 |
| `run-mapof-length-mismatch` | `mapOf([1,2],[3])` → task_fail（退出码 + 报文） | E1302 |
| `run-map-iteration-idiom` | `keys()` + `get()` 正典习惯形（Map 迭代的 sanctioned 路径） | E1302 |
| `build-bnd-map-iteration` | `for (k,v) in m` 维持发射停（负锚） | build 70（今日同态——防 T5 误开） |
| `build-bnd-mapof-key-domain` | `Map<Float64, Int64>` 构造域外停（裁决 ④） | E1302（红态不同因，绿态锚 70） |
| `build-bnd-map-value-domain` | `Option` 值域外停 | 同上 |
| `check-e0816-list-push` | `xs.push(4)` 仍 E0816（防收缩过宽） | 今日即绿（E0816）——**钉住不翻** |
| `check-e0816-map-values` | `m.values()` 仍 E0816 | 同上 |
| `check-members-*`（数枚） | 13 员签名面（参数型/返回型 E0501 锚：`xs.add("x")` on List\<Int64\>） | E0816→翻检查过 |
| `test-mock-mapof-e1804` | `mock collections.mapOf` → E1804 泛型类目 | E1302（模块未在）→翻 E1804 |
| `run-fs-listdir-regression` | fs.c 消费者回归（listDir 经句柄化载体） | 今日绿——回归钉 |

计数基线：conformance 893 起步，逐任务增量记档（预期 +14~16，以 as-built 为准）。

### D8-2 突变电池（每枚：锚点 count==1 断言、单测层 + 黄金层双判决、逐枚还原后全绿再下一枚）

1. **M-a 句柄描记位错**（`[i64 1]`→`[i64 2]`）：data 指针不被标 → gc 压测黄金死（数据块被扫）。
2. **M-b 键区描记摘除**（kdom==1 不设位）：String 键压测黄金死（键盒被扫后 keys 读回陈旧/崩溃）。
3. **M-c 值区描记摘除**（vtrace 不设位）：`run-map-gc-crossing` 死。
4. **M-d 墓碑复用摘除**（首墓碑不记、只写空位）：功能不死但 `M_TOMB` 只增不减 → rehash 提前；**判决预期 = 功能静默、以单测钉 `M_TOMB` 收缩语义**（黄金层结构性静默的诚实预期，T11 M2 先例）。
5. **M-e push 不返同一句柄**（增长返回新句柄）：`run-list-alias-visibility` 死 + emitCollect 语料回归死（根替换对假化）。
6. **M-f 长度不匹配检查摘除**：`run-mapof-length-mismatch` 死（越界读）。
7. **M-g rehash 触发界摘除**（`>`→`>=` 无效化）：满表死循环/覆盖——功能黄金死。
8. **M-h out 三字组 tag 翻转**：成员往返黄金死（fs (a) 先例同形）。

### D8-3 红先纪律

每枚黄金落盘前对 HEAD 二进制取红态（E0816 / E1302 / build 70 三形之一，记档退出码与输出）；实现后翻绿逐字节比对。门两测（`stdlib_test.go` 键集）先行——源未落盘时 embed pattern 红。

### D8-4 验证阶梯（每任务收口）

`go build` / `go vet` / `gofmt -l`（0 文件）/ `go test -count=1 ./...`（16 包）/ `validate.py --all --strict` / `docs_sync.py`（33 对）/ `git diff --check` / staged 无 `refr/`；`WE_UPDATE_GOLDEN=1` 不用（黄金手写）。零 IR 漂移专项（T1）：hello IR 对 HEAD worktree 双二进制 `cmp` 逐字节 + 全量语料零改写。

---

（实现期补记按任务追加于下。）

## 实现期补记一（T1，2026-09-16）

- **push 不走公开入口**：今日实现里 push 增长调 `__we_list_new(ncap, traced)` 复用构造；句柄化后公开入口会雕出**第二个句柄**，与「返回同一句柄」矛盾。as-built：抽出静态助手 `carve_data(cap, traced)`（雕 data 块 + 三载荷字初始化 + `install()`），`__we_list_new` 与 push 增长共用——D1-2 的「雕新数据块复制后就地交换」按此落形，公开五入口对外形不变。
- **harness 复用段重构为双块**：旧「精确复用」测试单块自洽；句柄化后一条 list 占两块（32B + 56B），as-built 按 `__we_free(data)` 先、`__we_free(handle)` 后的顺序入 free 表（新者在前、零合并、first-fit），新句柄整取 32B 块、`carve_data` 整取 56B 块——`reuse == donor && DATA(reuse) == ddata` 双断言。可达段清扫计数随之 3 → 4（句柄 + data + 两引用块）。
- **emitCollect 注释的诚实改写**：根替换对（pop 旧根 / push 返回值 / store 回槽）在稳定句柄下退化为**同指针三连**——语义空转。as-built 措辞：保留理由 = 全 push 链形状统一、零成本、深度记账不变；真正承重的是 walk 开始前推入的那枚常驻根（跨整个增长分配护住接收者）。IR 文本零改动（零漂移缝 ② 的对拍即含此簇）。
- **`run-fs-listdir-regression` 形**：`run-fs-listdir-order` 的计数加厚版——4 文件 listDir + for 打印 + `var n` 累计计数；改前对未动树（HEAD `13e9fe9` 等价）先取绿态（特征钉），改后全量轮再绿。fs.c `:294-299`「精确容量 ⇒ identity never moves」注释未动：v2 下因更强理由（恒同句柄）恒真。
- **hello 对拍项目**：`/tmp/b2b-t1/hello`——字面量两处（标量 + String）、for 走查两处、`iterator().collect()` 两处，五入口 29 个调用点俱在 IR；双二进制（HEAD 等价树 vs v2 树）各跑后 `cmp build/hello.ll` 逐字节零差。探针教训两条入册：main 体内 `iterator().map(...).collect()` 链停在 bndMainBody（黄金用 `.collect()` 直取形）；`Result<(), String>` 裸 String 错误参 E1204（须命名和式）。

## 实现期补记二（T2，2026-09-16）

- **`find_slot` 的 `*home` 出参（D2-3 插入规则的真缺陷）**：设计速写「插入：记住首个墓碑位（复用），否则首个空位写 occ=1」隐含空位可从查找返值得出——实现的 `find_slot` 未命中统一返 -1，丢失了探测停靠槽。首版 `tbl_insert` 因此在无墓碑可复用时 `i` 保持 **-1**，occ/keys/vals 全写目标槽**前一字**（`keys[-1]` 恰是 occ 字）：键盒指针字节覆进占用区、值落键区，任何首插入即坏表。harness 段错误暴露（ASan → get 未命中仍解引用 out[1]=0），22 载荷字 hex 转储 + `tbl_of` 绑定打印互证定位（写入点 = keys[-1]）。as-built：`find_slot` 增 `*home` 出参（探测停靠的首空槽——加载界保证其存在，正是 D2-3 的「miss-stop」），`tbl_insert` 无墓碑臂取之；四枚读路径调用点传哑元（注释声明 a read/removal never inserts）。
- **恒等 hash 的槽算术可观测性（自反勘误两枚）**：harness 的确定性推导全部按 home = k&7——键 9 复用键 1 的墓（home 同 1）、键 11 复用键 3 的墓（home 同 3）、7 条目贴 3/4 界 8 条翻倍。最初两处期望算错：Set 复用键写成 13（home 5，探测不经过槽 3 的墓——**复用要求探测经过墓碑**，这是墓碑复用的真实语义边界）；List 段删尾后期望值。两枚皆 harness 侧订正、载体无辜——可观测性设计（identity hash 使槽位可手算）恰恰让它们当场暴露而非静默。
- **`map_keys` 视图稳定性承重于精确容量**：keys() 的 `tbl` 视图绑在 m 的现行数据块上；`__we_list_new(len, kdom==1)` 精确容量保证 push 链零增长 ⇒ 表块不换、视图恒有效。`coll.c` 的 `exact capacity: no growth` 注释承重此事实，落册为不变式（未来若 keys 改为增长式构造，须改为重取视图）。
- **风暴探针的假阳性防御**：值区标记取 `0x5AA00000+k`（churn 垃圾块只写循环计数 0..39999）——被误扫的重用块必读错，杜绝小数值巧合假绿；键区探针以 churn 后新鲜等容盒读回（盒被扫则键字悬垂、内容比较失败）+ 删除解析双验。
- **描记位图逐字对表（D2-4 的 as-built 数字）**：cap 8 Map nslots 22（头 5 + occ 1 + 键 8 + 值 8）——键槽位 6..13、值槽位 14..21；cap 8 Set nslots 13（头 4 + occ 1 + 槽 8）——位 5..12。harness 以位循环逐位断言三形（键区独占/值区独占/双区）+ scalar→NULL + 句柄 `[1]`；与 D2-4 的公式（键槽 j 位 = 5 + P/8 + j）在 cap 8 处互证（5+1=6）。

## 实现期补记三（T3，2026-09-16）

- **D4-1 表 Map.get 行勘误**：表内 `get`（既有）行的括注「索引型随既有 get——Int64」是 List 行的复制粘贴错误——既有 get 臂的参数类型是 `t.args[0]`（**键型 K**），返回 `Option<V>`，从未是 Int64。as-built 该臂零触碰（键型参数为权威行为）；单测 `map get stays key-typed`（`Map<String, Int64>` 上 `m.get(1)` → E0501「the argument is Int64, the parameter is String」）把这条既有事实钉进测试。
- **E0816 负锚的列位**：D8-1 表未写列位；as-built E0816 锚**成员名起点**——`xs.push(4)` → 2:8、`m.values()` → 2:7（与 B2a T5 的成员名锚规则一致，非整条访问表达式锚）。
- **13 员与 iterator 的计数口径**：`collectionMembers` 每族返回 5–6 键（iterator 在表但**不计入 13**，D4-1 的「既有」行口径）。表外员经未改动的落空臂 E0816——D4-2 的「只收缩」由两枚负锚黄金 + 三族单测（push/values/clear）钉住。
- **语料爆炸半径（扩容前 grep 记档）**：testdata 全量对九个成员名 grep——`.put(`/`.size(` 的命中皆非集合接收者（用户接口 `self.put`、泛型参数 `x.size()` 走 paramRef 授集路径，E0817/E0814 域不受影响）；`.add(`/`.removeAt(`/`.keys(`/`.remove(` 集合接收者零命中。唯一携带 = `m6a_test.go` 的 B2a 期单测锚（`xs.add(1)` → E0816），重锚披露见 tasks.md T3 落地记。
- **E0501 锚位差异（既有行为，非本变更引入）**：参数位锚**实参**（`xs.add("x")` → 2:12），返回位锚**绑定名**（`let y: Int64 = …` → 2:9）——黄金逐位照机器字节写，不做统一。

## 实现期补记四（T4，2026-09-16）

- **D5-1 分类塔的第二处臂（设计未列名的必要码改）**：`callStrKind` 无 List 成员臂 ⇒ `"${xs.size()}"` 直接洞内调用停——valueKind 的 Call 臂求助 callStrKind，String 成员由 `strMemberKind` 答而 List 成员无人答，落 skNone 后洞渲染即停。as-built 补臂镜像 strMemberKind 位形：size → skI64（byteLength 先例）、add → skNone（void 无值可渲染）、get/removeAt → skNone（和式非单字域，match 是其读者）。这枚臂是 D5-1「分类纯发射不纯」的延伸：成员面的结果族必须两塔同答，否则检查过的程序在洞位无声落界。教训记档：探针先疑成员臂本体，`s.byteLength()` 双二进制同过之后才把面缩到分类塔——对照组探针比逐行读码先定位。
- **D5-5 层的 as-built 边界：String 载荷的成员 get/removeAt 停**：`payloadFace` 拒 skStr（String 值对两字、和式载荷槽一字）⇒ `List<String>` 的 get/removeAt 停 bndMainBody，而 add+size 经 T11-② 装盒管道全通。与 T9 的 reduce/find 载荷面**同一行诚实边界**（该行 docs 已登），本任务零新开面、以单测 `TestListMemberStringPayloadStops` 钉住。黄金族避开 String 元素的 get/removeAt 面。
- **D5-1 接收者两形的 as-built 判据**：`listFaceOf` 收绑定 / 顶层 List / 字面量三形——字面量接收者 `[10,20].get(1)` 通（`lit:20`）；**链式调用接收者**（`xs.iterator().collect().get(0)`）停，Call 接收者不在 listFaceOf 的覆盖内。D5-1 已把链式接收者划为 T5 探针域（mapOf(...).put(...) 同族），绑定后即通为正典形——非本面引入的缺口，T5 一并定形。
- **裸表达式 match 臂为既有 M9b 面（黄金形避让，非缺口扩大）**：`Some(v) => io.println(...)` 无块形臂停 bndMainBody——M9b 语句集边界，与本变更无关（双二进制同停实证）。黄金按 fs 房式用块形臂；`let o = xs.get(0)` 绑定后 match 同形即过，洞径不涉。
- **和式表读的来源与 key（D5-2 的 as-built 细节）**：tag 索引**不硬编码**——`payloadFace(face)` + `optionShapes` 同 T9 一条管道，None 0 / Some 1 与 coll.h C 侧写出的 tag 序**互证对齐**（头注释在案）；`sumSlot.key = "Option"` 沿 fs 的 `"Result"` 先例（fs `emitFsEntryCall` 同位同由）。root 纪律零新增：out 三字组三枚皆 i64 标量、成员臂不雕 gc 值；真机 gc 记录载荷往返（removeAt 后 `v.hi` 读出）实证被删记录经**构造根**存活——根只在体出口弹，构造根贯穿全 body，与 fs 臂的 Ok 载荷再扎根相对（那边载荷自被调方裸渡、这边载荷自始有主）。
- **新旧 get 入口的分野纪律（D6 的可观测化）**：同一程序同持 for 走查与成员 get 时，走查保载体 `__we_list_get`（越界陷阱——走查长度由构造保证在界）、成员走 `__we_coll_list_get`（越界 = None——chapter 17 表面语义）。单测以 `call void @__we_coll_list_get(` 恰 1 + `call i64 @__we_list_get(` 恰 1 双计数钉分野，防后续重构把两入口误合。

## 实现期补记五（T5，2026-09-16）

- **D3-4 订正：域标签从操作数读，不从 Site 读**。`__we_coll_map_of` 的 kdom/vtrace 是发射侧从 `emitListOperand(keys/vals)` 回的元素面拼出的 i64 即时量——发射器本位单一权威（构造、走查、成员调用读同一条键值字管道），免 Shape→listElem 转换器。Site 登记本身钉进单测（`TestCollCtorRegistersItsSite`：Args [Int64@0, String@1]、Ret Map 应用），证 D3-3 推断缝无缝，但发射不消费它——设计速写「域标签从 Site（K/V shape → kdom/vtrace）」订正于此。
- **D5-2 的第三处臂：classType 的 Map/Set 形参位**。`Map<K,V>`/`Set<T>` 经 elemFaceOfType 双面 + keyDom 门 → abiGc 空键（List 臂同形镜像）；`fn f(m: Map<Int64, Int64>)` 与 Map 返回位不再停 fitAbi。faces 经 `fnParamAbi.coll`/`fnAbi.retColl` 双向过界（bindDefineParams 建 collEnv、emitCallCore 实参位走 emitCollOperand、bindResult 从 gcReg 补 reg），单测 `TestCollParamAndReturnCarryTheFaces` 钉 define/call/成员三面。黄金不增（D8-1 表定死）。
- **D5-1 接收者判据的 keyed 面（补记四的预告在此定形）**：`collFaceOf` 只收 collEnv 绑定（镜像 listFaceOf 无 Call 臂）⇒ 链式 `mapOf(...).put(...)` 停 build 70、绑定后即通——T8-3 先例，绑定先行为正典。`for k in m.keys()` 链式同停（for 源是 Call，listFaceOf 同不收）⇒ 正典形 = `let ks = m.keys(); for k in ks`（黄金 `run-map-iteration-idiom` 即此形）。emitCollOperand 的 Call 臂只在**实参位**（`f(mapOf(...))` 直过）——与 emitListOperand 对称。
- **无槽 keyed（D7 的前置兑现）**：import 走查 `case "collections"` 只落 stdQuals——无 registerStdModule（模块无 sums/records）、无 curImports 腿（非 string 真体形）；keyed 拦截臂直接 C 调用、无 slotFor，与 fs/process 的可 mock 槽形成对照。mock 的单位由此显形为**单态模块 fn**：`mock collections.mapOf` 命中泛型类目 E1804（检查面拒 ⇒ 运行面不可达），T6 黄金钉该边界。
- **D0-3 值域行的成员面延伸：String 值的 get/remove 停**：`payloadFace` 拒 skStr ⇒ `Map<Int64, String>` 的 get/remove 停 bndMainBody（真机 n8 探针记档），put/size/keys/串键全通——与 `List<String>` 的 get/removeAt 同一行边界（补记四已钉 List 侧，本面为其 Map 镜像）。Set 无此面（remove/has 答 Bool）。黄金族避开该面；域外三负锚 = Float64 键构造 / Float64 集构造 / Option 值域两形（签名停 bndFnBody、空标注表构造停 bndMainBody；`[Some(1)]` 字面形连检查都过不去 E0827——既有检查面边界，泛型调用不在元素位定参，故值域负锚用空表 + 签名两形）。
- **D2-6 as-built 报文形**：`__we_task_fail("mapOf length mismatch")` 真机输出 `error: Panicked: mapOf length mismatch` + exit 1——task_fail 与 panic 族共用报文前缀与退出码（窄整型溢出同形），黄金 `run-mapof-length-mismatch` 三面钉。
- **ABI 线程与 collEnv 面（D5-1 的运载细节）**：collBinding 的 faces 是 `listElem`——Map/Set 的键值恰占 List 元素的那一个字，Set 以 `key` 为槽面、`val` 零值；`keyDom()` 是 Eq 域门（skI64/skU64/skBool → 恒等域 0、skStr → 盒句柄域 1、其余拒——Float64 非双射、Rune 非整型序）。collEnv 与 listEnv 同构六处体上下文随行；callResult 增 `coll *collBinding`，Ident 别名与 bindResult ckGc 路各补一路。

## 实现期补记六（T6，2026-09-16）

- **D7 的 mock 面兑现与无槽反证**：`test-mock-mapof-e1804` 落盘——`mock collections.mapOf` 命中 E1804 泛型类目（消息头与 diagnostics.toml title 逐字），红态 E1302 于真 HEAD。该黄金同时反证 T5 的无槽 keyed as-built：检查面先拒 ⇒ codegen 的 mockTarget/槽基础设施不可达——**有无槽在该面不可观测**，D7 的「槽基础设施 uniform 落位」速写与 T5 的无槽实现在此汇合为同一可观测行为。
- **D8-2 判决表的四处 as-built 订正**：①**M-b/M-c「压测黄金死」被推翻**——交付黄金的形状（同体字面量源 + 60000 churn）落在两区描记的杀伤半径外，双机理：源列表经自身元素描记把键盒/值盒全程保活（p4：300000 churn 仍读回 7）；一次扫除后自由表 LIFO 降序重建使低地址小盒躲过其后 ~16000 次复用预算，且 24B churn 因分配器 rem=8 分支永不吞 32B 键盒。**We 层载重由探针补证**（p3 助手源 300000 → `k0:262104` 垃圾值；p5 助手源 + 32B Junk churn → `k0:none`；健康树两形皆 `k0:7`），C harness 承载主证；黄金不改（List-M2 结构性静默先例）。②**M-g「满表死循环/覆盖」**——`>`→`>=` 是界前移一步非摘除；满表须经真摘除才可达，且真摘除亦无黄金可达（最大黄金表 4 条，界在 7 条）；终止保证由 C 层 harness 承载。③**M-e「emitCollect 语料回归死」窄化**——恰 `run-acute-collect-user-iterable` 一枚死，内建急性四枚的 collect 循环逐次改绑定取 push 返回值故存活。④M-d 结构性静默与设计预期一致（预期的兑现）。
- **电池方法论记档**：conformance 走进程内 `cli.Run` 且 rt-coll.c 每次 build 从 go:embed 重写重编——黄金层突变判决 = 改 `runtime/c/*.c` 后定向 `-run 'TestGoldenCases/<名>'`，无需独立二进制；每枚纪律 = 锚点 grep count==1 → 双层判决 → 还原 → `git diff --quiet runtime/c/` 净 → 双层复绿 → 下一枚。M-f 的死形额外记档：摘除的长度检查其爆炸落在 **List 载体自身的界陷阱**上（报文变 `List element read out of range`）——干净 panic 非静默垃圾，载体诚实性的意外红利。
