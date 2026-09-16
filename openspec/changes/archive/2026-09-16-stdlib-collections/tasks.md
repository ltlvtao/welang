# tasks — stdlib-collections（B2b）

> 权威：proposal.md（范围）+ design.md（D0–D8）。每任务实现前待用户点名；红先纪律 = 黄金在实现前取红态（E0816 / E1302 / build 70 三形，退出码与输出记档）。

## T1 List 载体 v2（稳定句柄，零 IR 漂移）

- [x] **list.c/list.h 句柄化改写**：32B 句柄（静态描记 `[1 x i64] [i64 1]`）+ data 块（今日布局原样、install() 复用）；`__we_list_new` 内部根桥；push 增长就地交换句柄 @16、**返回同一句柄**；get/len 一跳；snap 新句柄裹复制；头注释与 push 契约句英文改写（design D1-2/D1-3/D1-7）。
  来源：design D1-2、D1-3、D1-7
  验证：runtime C harness——别名面（同句柄增长跨 C 局部可见、两入口取 data 一致）、快照独立性、越界陷阱报文；`fs.c`/`process.c` 零改动即过既有 harness
- [x] **零 IR 漂移三缝**：全量语料 + IR 对拍 + C 消费者回归。
  来源：design D1-4、D1-5
  验证：`go test ./internal/conformance -count=1` 893 枚全绿零改写；hello IR 对 HEAD worktree 双二进制 `cmp` 逐字节零差；`run-fs-listdir-regression` 黄金绿（fs.c 消费者回归钉）
- [x] **注释与契约句收口**：`codegen.go` 家族注释的 "storage moves" 措辞订正（如有）。
  来源：design D1-7
  验证：grep 无移动存储旧措辞；gofmt/vet 绿

### T1 落地记（2026-09-16）

- **载体**：`runtime/c/list.c` v2——32B 句柄（静态描记 `handle_desc[1] = {1}`，bit 0 = 载荷字 0 的 data 指针；保留字 @24 恒零、无描记位）+ data 块（今日布局逐字原样，`install()` 原样复用、参数名 l→d）。`carve_data(cap, traced)` 静态助手抽出（new 开形与 push 增长重建共用——push 不再走公开 `__we_list_new`，避免雕出多余句柄）。`__we_list_new` 根桥：雕句柄 → `__we_root_push(h)` → `carve_data` → 接线 @16 → pop。push 增长：`carve_data(ncap)` → 复制 → `install` 在 carve_data 内 → **就地交换 @16** → 返回 `l` 本身。get/len/snap 经 `data_of` 一跳；snap = 新句柄裹复制（`sd = data_of(s)`）。
- **harness**（`runtime/list_test.go`）：宏加 `DATA(l)`（句柄一跳），LEN/CAP/TRACED/MAP/nslots/bit/elem_at 全部落 data 块；可达段清扫计数 3 → **4**（句柄 + data + 两引用块）；新**别名段**（头版面：C 局部复制的句柄跨增长可见——`av == alias`、`DATA(av) != before`、两入口取 data 一致、get 经两句柄一致）；增长段加 `gid` 钉 96 次 push 后同句柄；空表段扩为**双形检查**（句柄 `[1]` 常量位图 + data 三载荷字无描述符）；复用段重构为**双块精确复用**（`__we_free(ddata)` 先、`__we_free(donor)` 后——free list 新者在前，新句柄整取 32B、`carve_data` 整取 56B，`reuse == donor && DATA(reuse) == ddata`）；快照段加 `DATA(snap) != DATA(src)`。panic harness 零改动过（同句柄下重赋值形成立、报文不变）。
- **零 IR 漂移三缝**：① `go test ./internal/conformance -count=1` 164s 全绿——**893 枚既有零改写**（testdata 除新金外 `git status` 零改动）+ 新金 1 枚 = 894；② hello IR 对拍——`/tmp/b2b-t1/we-head`（HEAD `13e9fe9` 等价树先建）与 `we-v2` 双二进制各跑同一 list 密集项目（五入口 29 个调用点：字面量/走查/collect 三簇俱在），`build/hello.ll` `cmp` **逐字节零差**；③ `run-fs-listdir-regression`——改前对未动树先取绿态（特征钉），改后全量轮内再绿；**fs.c/process.c 零改动**（`git diff --stat` 四文件之外无涉触），`go test ./runtime` 全绿即过（O-2 的 C 侧半面落定）。
- **注释收口**：codegen.go 三处——声明表 `:202`（"(growth moves the block)" → 同句柄措辞 + B2b D1 引）、`emitListLit` doc（"the block never moves" → "no push allocates and the storage behind the handle never turns over"）、`emitCollect` 主段（"*new* identity / root replacement" 段整段改写：根替换对在稳定句柄下退化为同指针 pop/push/store，保留理由 = 全 push 链形状统一 + 零成本 + 深度记账不变）与追踪段（"may move" → "may grow"）。fs.c `:294-299` 注释未动——「精确容量 ⇒ 恒等性不动」在 v2 下因更强理由恒真。grep 五词形（new identity/storage moves/block never moves/moves the block/push that may have moved）全库零命中。
- **阶梯**：`go build`/`go vet`/`gofmt -l`（0 文件）绿；`go test -count=1 ./...` 16 包全绿（含上文 conformance 轮）；`validate.py --all --strict` 过；`docs_sync.py` 33 对（零 docs 触碰，T1 无文档面）；`git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用。

## T2 Map/Set 载体 C（runtime/c/coll.c）

- [x] **coll.c 载体与 13 入口落盘**：共享核（hash/相等/探测/墓碑/rehash）+ Map/Set 数据块布局 + `__we_coll_*` 全表（design D2-1–D2-6、D6）；描记两区（键区 iff String、值区 iff vtrace）；C 侧自根纪律（fs.c 先例）；mapOf 长度不匹配 `__we_task_fail`。
  来源：design D2-1、D2-2、D2-3、D2-4、D2-5、D2-6、D6
  验证：runtime C harness——put/get/remove 往返（Int 键 + String 键）、墓碑复用与 `M_TOMB` 收缩、rehash 触发界、set 同族、keys 构造、长度不匹配陷阱报文、gc 风暴跨 1 MiB 存活（描记两区各一探针）
- [x] **build.go rt-coll 链位**：运行时对象集加 coll.o（rt-fs 先例）。
  来源：design D3-2
  验证：`go build`/链接面真机跑通（T5 端到端复用）；programModules 零触碰 grep 记档

### T2 落地记（2026-09-16）

- **载体**：`runtime/c/coll.h`（13+2 入口契约注释 + 布局/探测/描记/Option 三字组四段头注释）+ `runtime/c/coll.c`（约 500 行）。共享核 = `tbl` 小视图（len/cap/tomb/kdom/vtrace/occ/keys/vals-or-NULL 指针族）上的一份 `fnv1a`/`key_hash`/`key_eq`/`find_slot`/`raw_place`/`tbl_insert`/`grow_table`/`tbl_put`——Map/Set 两族同核（D2-1「一类事实一个位置」）。两层载体：32B 句柄（静态 `handle_desc[1] = {1}`）+ 数据块（Map 五头 + occ + 键区 + 值区 / Set 四头 + occ + 槽区）。`install_desc` 两区位图（键区 iff kdom==1、值区 iff vals&&vtrace、全标量 → map=NULL）；构造/增长路径 `new_coll`（根桥）/`carve_table`/就地交换 @16（List 姿态）。`__we_coll_list_remove_at` 跨族读 list 数据块布局（`+2+3` 元素基、`[2]` LEN——两常量在注释里引 list.h 公开形状；经公开入口的行为由 harness 钉）。
- **真缺陷一枚（harness 段错误暴露、ASan + hex 转储互证定位）**：`tbl_insert` 未命中路径未取探测停靠的空槽——`find_slot` 未命中统一返 -1，无墓碑可复用时 `i` 保持 **-1**，occ/keys/vals 全写到目标槽**前一字**（`keys[-1]` 恰是 occ 字：键盒指针字节覆进占用区、值写进键区）。修法：`find_slot` 增 `*home` 出参（探测停靠的首空槽），`tbl_insert` 无墓碑臂取 `home`；四枚读路径（map_get/map_remove/set_remove/set_has）传哑元并以注释声明 a read/removal never inserts。定位链 = ASan 报 main.c:109（get 未命中仍解引用）→ 22 载荷字 hex 转储（键盒落在 payload[5]=occ 位、值落 payload[13]）→ `tbl_of` 绑定打印互证（keys=payload6、occ=payload5 ⇒ 写入点 = keys[-1]）。
- **harness 自身两处算术勘误（载体无辜）**：① Set 墓碑复用段键 13 → **11**——复用要求新键的探测**经过**墓碑（恒等 hash 下 home = k&7：键 3 之墓在槽 3，11 的 home 恰 3；13 的 home 是 5 不经过）；② List 段 `[20,30]` 删 index 1 后剩 `[20]`，期望值 30 → 20。恒等 hash 使槽算术可观测，这两枚勘误正是可观测性的自反。
- **harness 十段**（`runtime/coll_test.go`，panic harness 独立第二枚）：精确计数可达段（清扫 4 = 两构造 list 的四块——键盒+值块经两区描记存活以计数差证明；终局 5 = 句柄+表+盒+值+探针）、Int 键往返（覆盖/删后 None 零化垃圾预填三字组/墓碑复用键 9 home 1 = 键 1 之墓、TOMB 1→0）、rehash 两条触发路（插入驱动：6 条 cap 8 → 第 7 条仍 8 → 第 8 条先倍增 16，DATA 换块、句柄与别名同指；删除驱动：7 删 7 → tomb 7 → put 42 → cap 16 tomb 0）、String 键内容等价（六枚新鲜盒读/覆盖/新键/删除全按字节解析）、描记位图逐位断言（cap 8 Map nslots 22：键 6..13 值 14..21；键区独占形/值区独占形/双区形/scalar→NULL 四种；Set 字串槽 nslots 13：5..12；句柄 `[1]` + bit1==0）、keys()（域携带——String 键 map 出 traced list、按内容成员恰一次、删空后 keys 空）、Set 族（构造去重/重复 add no-op/has/remove 双答/墓碑复用/增长界 7 贴 8 翻）、List 表面族（get OOB→None、remove_at 移位语义、空表）、双区 gc 风暴（各 2.5MB churn 越阈值数轮：值区 8 枚标记块 0x5AA00000+k 读回全中——垃圾块只写循环计数，扫掉重用必读错，防小数值巧合假绿；键区 String 盒 churn 后新鲜等容盒读回 + 删除解析）。panic harness：`mapOf length mismatch` → tag=1 报文逐字（sched boot + report 形，list 越界先例）。
- **rt-coll 链位**：`build.go` 三处插入（write list `coll.h`+`rt-coll.c`、linkArgs `rt-coll.o`、`clang -c` 步骤）全部紧随 rt-process 之后（fs/process 先例）；`runtime.go` 增 `CollHeader`/`CollSource` 两 embed。**programModules 零触碰**（grep：`build.go:229` 调用点与 `:363`/`:367` 定义之外无涉触；本任务对 build.go 的全部改动在 `compileProgram` 内）。链接面真机验证 = conformance 全量轮（894 枚中每一枚 run/build 黄金都经 compileProgram 链 rt-coll.o）——强于任务书的最小要求，T5 端到端在此基础上复用。
- **阶梯**：`go build`/`go vet`/`gofmt -l`（0 文件）绿；`clang -Wall -Wextra -c coll.c` 零警告；`go test -count=1 ./...` **16 包全绿**（conformance 161s——**894 枚零改写**，本任务零新金，testdata `git status` 零改动；每一枚 run/build 黄金经 compileProgram 链 rt-coll.o，链接面随全量轮真机验证）；`validate.py --all --strict` 过；`docs_sync.py` 33 对（零 docs 触碰，T2 无文档面）；`git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用。

## T3 检查器 13 员表

- [x] **collectionMembers 扩容**：13 员 fnType（void 员 `unitType{}`、`keys` 构造 `List<K>` namedType、K/V/T 实参代入机制同既有 get/has）；E0816 落空臂零改。
  来源：design D4-1、D4-2
  验证：typecheck 单测（13 员签名 + 表外员 E0816）；真机 `xs.add(4)` 翻检查过（E0816 → exit 0 check）记档
- [x] **check 黄金族落盘**：`check-members-*`（签名面 E0501 锚：`xs.add("x")` on List\<Int64\> 等）+ E0816 边界负锚（`check-e0816-list-push` / `check-e0816-map-values`）。
  来源：design D8-1 表
  验证：逐枚先红（E0816）后绿；边界负锚今日即绿——**钉住不翻**；`-count=1` 定向跑

### T3 落地记（2026-09-16）

- **扩容**：`typecheck.go` `collectionMembers` 三 case 从 get/has/iterator 扩为 13 员全表——List{add,removeAt,get,size} + Map{put,remove,get,keys,size} + Set{add,remove,has,size}；iterator 三族保留、不计入 13（D4-1「既有」行）。void 员 `unitType{}`；`keys` 构造 `namedType{decl: listSum, args: [K]}`；Option 员 `namedType{decl: optionSum, args: [T/V]}`；K/V/T 全部自 `t.args` 代入（与既有 get/has 同一机制）。消费点（成员解析 namedType 臂）与 E0816 落空臂**零改**。
- **真机翻检查过记档**：HEAD 二进制（`9ceb1d3` 构建）上三族清洁面全 E0816（List `add` 2:17 / Map `put` 2:7 / Set `add` 2:7）；T3 二进制上 exit 0 静默——`xs.add(4)` 检查过兑现。unit 值绑定 `let ys = xs.add(1)` 合法（exit 0）。
- **单测**：新 `internal/typecheck/b2b_test.go` 两测——`TestCollectionMembersThirteenSurface`（5 清面 + 12 枚 E0501 锚：参数位 argument-vs-parameter、返回位 binding-vs-annotation；Map 面以 `Map<Int64, String>`/`Map<String, Int64>` 钉 K/V 代入出 `Option<String>`/`List<String>`，put 双位钉 K 先 V 后，record 元素面钉 T 线程入 removeAt 载荷经 match 读 `v.hi`）+ `TestCollectionMembersOffTableFallThrough`（push/values/clear 三族 E0816）。typecheck 包全绿。
- **m6a 重锚披露（唯一语料携带）**：`m6a_test.go` 的 B2a 期锚 `xs.add(1)` → E0816 随本任务翻转（add 今为成员）——重锚为表外员 `xs.push(1)` → `"push" is not a member of List<Int64>`（2:17 同列，`push` 与 `add` 同为四字母），注释注明 B2b T3 重锚。爆炸半径 grep 预检：黄金零携带（`.put(`/`.size(` 命中皆非集合接收者——用户接口 `self.put`、泛型参数 `x.size()` 走 paramRef 路径不受影响），单测携带恰此一枚。
- **黄金 13 枚**（conformance 894→907）：`check-members-{list,map,set}` 三清面（List 面含 unit 绑定形）+ 8 枚 E0501 签名锚（add 实参 / removeAt 实参 / removeAt 返回 `Option<Int64>` / put K 位 / keys 返回 `List<Int64>` / void 返回 `()` / set size 返回 Int64 / set remove 返回 Bool）+ 负锚 `check-e0816-list-push`（2:8）/`check-e0816-map-values`（2:7——E0816 锚**成员名起点**，非整条访问表达式）。逐枚红态（HEAD 二进制 E0816）真机记档；负锚 HEAD↔T3 输出逐字节一致——**钉住不翻**兑现。
- **阶梯**：`go build`/`go vet`/`gofmt -l` 绿；`go test -count=1 ./...` **16 包全绿**（conformance 169s / 907 枚，既有零改写）；`validate.py --all --strict` 过；`docs_sync.py` 33 对；`git diff --check` 净；`WE_UPDATE_GOLDEN` 未用（黄金全手写自真机捕获字节）。

## T4 List 成员发射

- [x] **成员臂 + `__we_coll_list_get`/`__we_coll_list_remove_at`**：接收者字（loadPtr 句柄）+ 实参字管道复用（T11 元素字）；out 三字组（fs 发射臂镜像、tag 索引自被调方签名携带的和式表读）；add 复用 `__we_list_push`（弃返回）、size 复用 `__we_list_len`；`xs.get` 走新入口（OOB → None，非载体陷阱）。
  来源：design D5-1、D5-2、D5-3
  验证：codegen 单测（out 组 IR 钉 + 新旧 get 入口分野）；真机 removeAt/get/size 往返
- [x] **别名可见性 run 黄金**：`run-list-alias-visibility`（`let ys = xs; xs.add(4); ys.size()` → 4——roadmap 点名项）+ `run-list-members`（含 OOB → None）。
  来源：design D8-1 表、D1-6
  验证：先红（T3 后 check 过仍 build 70）后绿；真机 stdout 逐字节复核

### T4 落地记（2026-09-16）

- **成员臂落位**：`emitCall` 分派在方法表面（`e.methods`）之后、String 成员面之前插 `emitListMember`——门 = `listMember`（add/removeAt/get/size 四名）∧ `listFaceOf`（纯分类不发射：listEnv 绑定 / 顶层 List / 字面量），门外的接收者（Map/Set 绑定、String、链式调用）原样落到下面各面。add 复用 `__we_list_push` **弃返回**（句柄稳定 ⇒ 返回值即绑定已持的同一指针——别名可见性的全部要点）；size 复用 `__we_list_len`；get/removeAt 走 coll 新入口的 out 三字组——fs `emitFsEntryCall` 逐形镜像（alloca `[3 x i64]`、gep 0/8/16 三读、三 i64 槽）。Option 表 = `payloadFace(face)` + `optionShapes`——**None 0 / Some 1 与 C 侧写出的 tag 序一致**（coll.h 头注释在案），`sumSlot.key = "Option"`（fs 的 `"Result"` 先例；sum 参数位绑定同键）。接收者字与实参字全走既有管道：`emitListOperand`（句柄）+ `emitListElemValue`（元素字——含 String 装盒、f64 位型、窄整型）/ `emitNumExpr`（索引）。
- **分类塔补臂（本任务的第二处码改）**：`callStrKind` 无 List 成员臂 ⇒ `"${xs.size()}"` 直接洞内调用停（valueKind → skNone）。补臂镜像 `strMemberKind` 位形：size → skI64（byteLength 先例）、add → skNone（void 无值）、get/removeAt → skNone（sum 非单字域，match 是其读者）。洞径随后全通（`__we_str_of_i64` 渲染）。
- **真机往返记档**：gc 记录载荷（`List<Cell>` removeAt → `v.hi` 读出 7、get 读 8——**被删记录经构造根纪律存活**：根只 在体出口弹，构造根贯穿全body）；f64 载荷（2.5/1.5 位型往返）；窄整型 Int8（11/7 宽度随载荷面携带）；`List<String>` add+size 通（T11-② 装盒管道）而 **get/removeAt 停**——`payloadFace` 的 skStr 拒绝（String 值对是两字、载荷槽是一字），与 reduce/find 载荷面同一行诚实边界，已在 codegen 单测 `TestListMemberStringPayloadStops` 钉住。字面量接收者 `[10,20].get(1)` 通；链式调用接收者 `collect().get(0)` 停——D5-1 的 T5 探针域，绑定后即通。
- **红先记档**：两枚黄金源在 HEAD 二进制（`961592c` 构建）皆 exit 70 `bndMainBody`；T4 二进制上 exit 0，stdout 逐字节手捕（od -c 复核）。
- **单测**（新 `internal/codegen/b2b_test.go`，四测）：out 三字组（两入口 declare 行 + 尾随 out 操作数 + gep 0/8/16）；**新旧 get 入口分野**（一程序同持走查与成员：`call void @__we_coll_list_get` 恰 1、`call i64 @__we_list_get` 恰 1——走查保载体入口）；add（push 恰 2 = 字面量自带 + 成员、len、`__we_str_of_i64`）；String 载荷停面。
- **披露（既有停面，非本面引入）**：裸表达式 match 臂（`Some(v) => io.println(...)` 无块）停 bndMainBody——M9b 语句集边界，黄金按 fs 房式用块形臂。
- **阶梯**：`go build`/`go vet`/`gofmt -l` 绿；`go test -count=1 ./...` **16 包全绿**（conformance 231.8s / **909** 枚，既有零改写）；`validate.py --all --strict` 过；`docs_sync.py` 33 对（零 docs 触碰，T4 无文档面）；`git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用（黄金手写自真机捕获字节）。

## T5 构造面 + Map/Set 成员发射

- [x] **【阻塞前置】泛型限定调用推断缝探针**：`collections.mapOf(ks, vs)` 三面真机——过则直行；停则按 B2a T4 先例参数化修复 + 披露（≤ 管线参数化，否则回退 + 披露）。同时复核 programModules 零触碰立场（键控先例；如需 curImports 腿按 string 先例补 + 披露）。未定不得勾其后各项。
  来源：design D3-3、D3-2
  验证：探针输出记档（三面退出码 + IR 样本）；结论写回本行（直行/修复/回退 三选一 + programModules 裁定）
  **结论（2026-09-16）：直行。** 三面（p1 无标注 / p2 标注形 / p3 仅 import 不调用）check 全 exit 0——M6a 泛型限定调用推断无缝，标注与无标注两形同过；Site 登记实证（`collections.mapOf([1,2],["a","b"])` → Args [Int64@0, String@1]、Ret Map 应用；setOf 同形）。build/run 停点隔离 = codegen import 走查 default 臂（p3 同停 ⇒ 停在装载非调用解析，D3-2 预测精确命中），`case "collections"` 一行即通。**programModules 裁定：零触碰确认**（`build.go:363-367` 三处之外无涉触）；curImports 腿不需要（keyed 模块非 string 真体形）。
- [x] **collections.we 落盘 + keyed 构造**：`stdlib/src/collections.we`（mapOf/setOf 虚构体，design D3-1 源形）；import 走查 `case "collections"`；keyed 表 + 域标签从 Site（K/V shape → kdom/vtrace，映射失败 `e.bnd()`）；门两测扩键集。
  来源：design D3-1、D3-2、D3-4
  验证：门两测红先（源未落盘 embed pattern 红）后绿；真机 `import std.collections` 过装载
- [x] **Map/Set 成员臂**：put/get/remove/keys/size/add/remove/has（out 组 / i64 直入 / keys 再扎根 T12 先例）；链式接收者探针（`mapOf(...).put(...)`——停则披露，绑定后即通为正典形）。
  来源：design D5-1、D5-2、D5-4
  验证：codegen 单测（再扎根 IR 钉 + 域标签 i64 即时量）；链式/绑定两形探针输出记档
- [x] **构造与成员 run 黄金族**：`run-map-members`、`run-set-members`、`run-map-string-keys`（碰撞/覆盖/删除重插）、`run-map-gc-crossing`（描记压测）、`run-mapof-length-mismatch`（task_fail 退出码 + 报文）、`run-map-iteration-idiom`（keys()+get() 正典形）+ 负锚 `build-bnd-map-iteration`（维持发射停）、`build-bnd-mapof-key-domain` / `build-bnd-map-value-domain`（域外停）。
  来源：design D8-1 表、D0-3、D2-6、D5-5
  验证：逐枚先红（E1302）后绿；域外负锚绿态锚 exit 70；真机 stdout/stderr/exit 三面复核

### T5 落地记（2026-09-16）

- **接线总形**（codegen.go +528 行）：import 走查 `case "collections"`（io/test/time 之后）；`emitCollCtorCall`（mapOf/setOf 两臂——长度先验、keyDom 门、域标签 i64 即时量随行、答案**紧邻下一行**再扎根——T12 返回纪律）；`emitCollMember` 分派臂（方法表面之后、String 成员面之前）；resolveStd keyed 拦截臂（直接 C 调用、**无 slotFor**）；classType Map/Set 形参臂；`fnParamAbi.coll`/`fnAbi.retColl` 全 ABI 线程；collEnv 六处 savedList 体上下文随行。声明表 +11 行。
- **面读来源订正（D3-4 as-built）**：kdom/vtrace 从**操作数**读（`emitListOperand` 回的元素面）不从 Site 读——发射器本位单一权威（构造、走查、成员调用读同一条键值字管道），免 Shape→listElem 转换器。Site 登记钉进单测 `TestCollCtorRegistersItsSite`——证 D3-3 推断缝无缝，但发射不消费它；设计速写「域标签从 Site」订正于补记五。
- **成员臂 ABI**：put/add → ckVoid；size → ckI64（洞径 `__we_str_of_i64`）；Set remove/has → ckI64 typeName "Bool"；Map get/remove → `[3 x i64]` out 三字组 + `payloadFace(b.val)` 门 + `optionShapes`（None 0/Some 1 与 C 侧写序互证——T4 同一管道）；keys → ckGc 且 `res.list = &b.key`（List<K> 绑定，for 走查即走）+ 紧邻再扎根。String 键过元素装盒管道（单测钉 alloc 32 恰 2 = 字面键 + put 键）。
- **无槽 keyed vs 可 mock 槽的对照（D7 前置兑现）**：fs/process 拦截臂走 slotFor（mock 可换）；collections 臂直接 C 调用——`mock collections.mapOf` 命中 E1804 泛型类目（检查面拒 ⇒ 运行面不可达 mock），T6 黄金钉该边界。
- **形参/返回 ABI 面（D5-1 点名项兑现）**：`fn make() -> Map<...>` / `fn fifth(m: Map<...>) -> Int64` 全通——define 一指针过界、调用经 slot-load 形、被调方成员经签名携带的 faces 分类（单测 `TestCollParamAndReturnCarryTheFaces` 钉 define/call/成员三面）。
- **链式接收者两形记档**：`mapOf(...).put(...)` build 70（collFaceOf 只收绑定——T8-3 先例，绑定先行为正典）；`for k in m.keys()` 链式同停（for 源是 Call）⇒ 黄金用 `let ks = m.keys(); for k in ks` 正典形。
- **值域停两形（D0-3）**：签名 `fn f(m: Map<Int64, Option<Int64> >)` 停 bndFnBody；构造 `let vs: List<Option<Int64> > = []` + mapOf 停 bndMainBody。`[Some(1)]` 字面形 E0827——既有检查面边界（泛型调用不在元素位定参），故值域负锚用空表 + 签名两形。另 String 值的 get/remove 停（payloadFace 拒 skStr——`List<String>` get 同一行边界的 Map 镜像，n8 探针真机记档）；put/size/keys/串键全通，黄金族避开该面。
- **陷阱面（D2-6 as-built）**：mapOf 长度不匹配 → stderr `error: Panicked: mapOf length mismatch` + exit 1（task_fail 走 panic 族报文形，窄整型溢出同形），黄金三面钉。
- **黄金 9 枚（909→918）**：`run-map-members` / `run-set-members` / `run-map-string-keys`（覆盖/删除/墓碑重插）/ `run-map-gc-crossing`（60000 Cell churn >1MiB 后读回）/ `run-mapof-length-mismatch` / `run-map-iteration-idiom`（keys()+get() 正典形）+ 负锚 `build-bnd-map-iteration` / `build-bnd-mapof-key-domain` / `build-bnd-mapof-value-domain`（D8-1 表名 `mapof-`；任务书行文 `map-` 为笔误，文件名从表）。红先三态：真 HEAD `cd6093e` 九枚全 `2:8 E1302 module not found` exit 1；源落盘未接线二进制（we-pre）六枚 run 金 build 70（负锚三枚同态）；本树全绿。
- **单测**（b2b_test.go 4→13 函数，codegen 包 465→474——T7 审查期计数订正：原记 466→475 双端各差一；基线 13e9fe9 实测 461 + T4 四测 = 465 + T5 九测 = 474，T6 零增量，`grep -h '^func Test'` 逐提交对账）：Site 登记 / 域标签 + 紧邻再扎根 / 成员面符号 + out 组 / keys 再扎根 / 串键装盒 / 形参返回往返 / 链式停 / 双参 for 维持停 / 域外三停（Float64 键、Float64 集、Option 值域签名）。
- **阶梯**：`go build`/`go vet` 绿；`gofmt -l` 初跑标 codegen.go → `-w` 后净；`go test -count=1 ./...` **16 包全绿**（14 ok + 2 无测试文件；conformance 374.7s / **918 枚**，既有零改写）；`validate.py --all --strict` 过；`docs_sync.py` 33 对（零 docs 触碰，T5 无文档面）；`git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用（九枚 stdout/stderr 逐字节自真机 od -c 核后落盘）。

## T6 mock 面 + 电池与验证阶梯

- [x] **mock 面黄金**：`test-mock-mapof-e1804`（`mock collections.mapOf` → E1804 泛型类目，零码改的诚实边界钉）。
  来源：design D7、D8-1 表
  验证：先红（E1302——模块未在）后绿（E1804）；消息与注册表 title 逐字比对
- [x] **突变电池**：design D8-2 八枚（M-a 句柄描记位 / M-b 键区描记 / M-c 值区描记 / M-d 墓碑复用 / M-e push 句柄 / M-f 长度检查 / M-g rehash 界 / M-h tag 翻转）——每枚锚点 `count==1` 断言、单测层 + 黄金层双判决、逐枚还原（还原后全绿再下一枚）。
  来源：design D8-2
  验证：八枚判决表入完成记录（死/存活 + 层 + 机理——结构性静默的诚实预期随 M-d 记档）；结束后 `go test -count=1 ./...` 全绿
- [x] **验证阶梯全量**：AGENTS.md 阶梯——`go build`/`go vet`/`gofmt -l`/`go test -count=1 ./...`（16 包，conformance 总数记档）/`validate.py --all --strict`/`docs_sync.py`（33 对）/`git diff --check`/staged 无 `refr/`；`WE_UPDATE_GOLDEN=1` 未用（黄金全手写）。
  来源：AGENTS.md 验证阶梯、B2a T6 先例
  验证：八面输出记档；conformance 新总数（893 + 逐任务增量）与 tasks 各任务对账

### T6 落地记（2026-09-16）

- **mock 面黄金**：`test-mock-mapof-e1804` 落盘（conformance 918→**919**）。红态真 HEAD `cd6093e` 二进制 = `1:8 E1302 module not found` exit 2；绿态 = `4:22 E1804 mock target is not a mockable function — mapOf is a generic fn; only a module-level monomorphic fn is mockable` exit 2——消息头与 diagnostics.toml `[diagnostic.E1804]` title **逐字一致**（比对记档）。位置 4:22 锚目标短名起点。`test-` 前缀走 `we test .` 管线（check 面先拒 ⇒ codegen 的 mockTarget/槽基础设施不可达——无槽 keyed as-built 由此黄金反证不可观测）。源形镜像 `check-e1804-generic-fn`（无返回段、`return` 体）。
- **电池跑法记档**：conformance 走进程内 `cli.Run`（runner.go:75）且 rt-coll.c 每次 build 从 go:embed 重写重编——突变改 `runtime/c/*.c` 后 `go test -count=1 -run 'TestGoldenCases/<名>'` 即生效，无需独立二进制；定向黄金集 = 九枚 T5 金 + 两枚 T4 List 金 + `run-fs-listdir-regression` + 本任务 mock 金（M-e 另扩 collect 族四枚 + `run-list-elem-gc-crossing`）。
- **八枚判决表**（每枚锚点先 grep `count==1`、逐枚还原后 `git diff --quiet runtime/c/` 净 + 双层复绿再下一枚）：

  | 突变 | 锚点 | 单测层 | 黄金层 | 机理 |
  | --- | --- | --- | --- | --- |
  | M-a 句柄描记位 `{1}`→`{2}` | coll.c 句柄描记行 | TestCollHarness 死 | **run-map-gc-crossing 死** | 描记指错载荷字 ⇒ 表块不被标 ⇒ 扫除后复用即腐 |
  | M-b 键区描记摘除 | install_desc `trace_keys=1` | TestCollHarness 死 | 全活（结构性静默） | 双因：同体源列表经自身元素描记保活键盒 + 24B churn 因 rem=8 分支不吞 32B 盒；**p5 探针**（助手源 + 32B Junk churn 300000）→ `k0:none` vs 健康 `k0:7`——We 层载重实证 |
  | M-c 值区描记摘除 | install_desc `trace_vals` | TestCollHarness 死 | 全活（结构性静默） | 同上；**p3 探针**（助手源 + 300000 churn）→ `k0:262104` 垃圾值 vs 健康 `k0:7`；同体字面量源 300000 仍 `k0:7`（p4）——源列表保活的直接证据 |
  | M-d 墓碑复用摘除 `if (0)` | tbl_insert | TestCollHarness 死 | 全活（设计自预期） | 功能不死、M_TOMB 只增 ⇒ rehash 提前；收缩语义由 harness「7 删 7 → put 42 → cap 16 tomb 0」段钉 |
  | M-e push 增长返新句柄 | list.c 就地交换行 | TestListHarness 死 | **三死**：run-list-alias-visibility / run-list-members / run-acute-collect-user-iterable | add 弃返回 ⇒ 绑定持旧块（别名可见性黄金即此钉）；collect 族恰**用户迭代器形**死——内建急性四枚的 collect 循环逐次改绑定取返回值故存活 |
  | M-f 长度检查摘除 | map_of task_fail 行 | TestCollPanicHarness 死 | **恰 run-mapof-length-mismatch 死** | 摘除后越界读值撞 **List 载体自身界陷阱**：报文变 `error: Panicked: List element read out of range`——载体的诚实陷阱兜住爆炸（干净 panic 非静默垃圾） |
  | M-g 界前移 `>`→`>=` | crowded | TestCollHarness 死 | 全活（结构性静默） | 倍增界前移一步；最大黄金表 4 条距 7 条界尚远——无黄金过界 |
  | M-h out 三字组 tag 翻转 | out_none/out_some | TestCollHarness 死 | **五死**：map-members / string-keys / gc-crossing / iteration-idiom / list-members | Some/None 臂互换（fs (a) 先例同形）；Set 族 Bool 直答与负锚/别名面不涉 |

- **设计预测 vs as-built 四处订正**（随 M 记档）：①M-b/M-c「压测黄金死」被推翻——交付黄金（60000 churn、同体源）落在杀伤半径外：一次扫除后自由表 LIFO 降序重建使低地址小盒躲过其后的复用预算（~16000 次），且同体源列表全程保活键值（p3/p4/p5 三探针互证）；**两区的 We 层可观测性由探针补证、C harness 承载主证**，黄金不改（T2 List-M2「结构性静默记档」先例）。②M-g「满表死循环/覆盖」——`>=` 是界前移非界摘除，满表须经真摘除才可达且亦无黄金可达（终止保证由 C 层承载）。③M-e「emitCollect 语料回归死」窄化为用户迭代器形一枚。④M-d 结构性静默与设计预期一致（唯一兑现 predicting 的静默行）。
- **阶梯**：`go build`/`go vet`/`gofmt -l`（0 文件）绿；`go test -count=1 ./...` **16 包全绿**（14 ok + 2 无测试文件；conformance 全量轮 **919 枚**零改写——893 基线 + T1 1 + T3 13 + T4 2 + T5 9 + T6 1 对账相符）；`validate.py --all --strict` 过；`docs_sync.py` 33 对（零 docs 触碰，T6 无文档面）；`git diff --check` 净；staged 无 `refr/`；`WE_UPDATE_GOLDEN` 未用（mock 黄金字节手写自真机捕获）。

## T7 实现审查 + 归档

- [x] **welang-code-review 七条**：读 `.agents/skills/welang-code-review/SKILL.md` 手动执行；审查记录入 proposal.md。
  来源：里程碑惯例（B2a T7 先例）
  验证：七条结论表 + 发现与处置段入档；审查期码改（如有）独立提交
- [x] **归档**：`mv` 至 `openspec/changes/archive/<日期>-stdlib-collections/`、`change.yaml` → archived、roadmap B2b 行收口（双语）、follow-up 划账、记忆回写。
  来源：里程碑惯例（B2a 归档先例）
  验证：归档后 `validate.py --all --strict` → 「no changes found + registry clean」期望态；全阶梯复跑绿；提交与推送待用户明示

### T7 落地记（2026-09-16）

- **七条全过**（记录入 proposal.md「## 审查记录」）：①规范符合性——`docs/spec/`+`diagnostics.toml` 对 `13e9fe9..HEAD` 零 diff、ch17 R1/R3/R5 锚与 ch20 E1804 类目逐枚对黄金核验；②验证诚实性——26 枚新金重数（893→919）、16 枚定向金复跑绿、runtime/stdlib/typecheck 绿、codegen 包逐提交对账（461→465→474）；③测试先行——六份落地记红先记录在册；④诊断协议——零新码、零 `--json` 字段触碰、消息 title 起头字节即证；⑤单一权威——改动文件 CJK 注释 0、长期事实归位 design 补记 + 源码头注释；⑥红线——42 文件全在影响层、`refr/` 零、零 trailer；⑦全阶梯复跑绿（16 包、conformance 919、validate strict、docs_sync 33 对、diff --check 净）。
- **发现三项，全部随审处置**：F1 docs 欠账（proposal 承诺的运行面段集合面在册 + 迭代停面披露未落）→ 审查期双语补齐（独立提交）；F2 陈旧例句（`xs.add(1)` 自 T3 失真）→ 换 `xs.push(1)`（负锚黄金恰钉此形）；F3 T5 落地记载 codegen 包 466→475 双端各差一 → 原位订正 465→474（逐提交 `grep -h '^func Test'` 对账链记档）。
- **归档**：`mv` 至 `openspec/changes/archive/2026-09-16-stdlib-collections/`；change.yaml → archived；roadmap 双语 B2b 行 as-built 收口（诚实注记：直接 Map/Set 迭代按申报非目标留 follow-up #26、String 载荷成员 Option 与域门停面已在 benchmarks 披露）；follow-up 登记 #26（ch17 R1 的 Map/Set Iterable 发射面）；记忆回写 project-overview。
