# tasks — dependencies（M13）

- [x] T1 测试先行 A：runner env 面落地 + 黄金矩阵落盘（预计 21 新增 + 1 计划改写 check-deps-nonempty（design D8 定数 22 涉及；clean-leaves-lock 定性钉单列）），对当前构建必须先红；既有 674 枚零意外回归（负向对账：边界行删除后无既有黄金钉依赖面）
  来源：proposal 做什么 / design D7 D8
  验证：`go test ./internal/conformance/ -run TestGoldenCases` 红（新增失败清单 + 既有 674 零回归），红因分类记于完成记录；D8 各桶逐枚对账落点

  完成记录（2026-09-09）：
  - **runner env 面**：`runner.go` Case 增 `Env map[string]string`（JSON 键 `env`，字段序在 Setup 后——WriteCase 序不变兼容）+ `applyEnv`（字典序设入，LookupEnv 存旧值含缺席标记）/`restoreEnv`（缺席 Unsetenv）；Execute chdir 后 apply/defer restore。gofmt/vet 净。runner 改动 = 测试基建（D7），env 相对值（`registry`/`cache`）随 cwd=workdir 自然解析。
  - **22 枚落盘，D8 逐桶对账**：E2006 ×3（upper `MyLib`/underscore `my_lib`/std 保留）+ E2003 ×3（check-deps-nonempty **改写**（setup 逐字保全——脚本内断言钉死，仅 exit 70→1 与 stderr 边界→E2003 翻面）/ op `>1.2.3`/ leading-zero `^01.2.3`）+ E2001 ×1（left-pad 无源应答）+ E2002 ×2（ceiling = 章文场景逐字：root `b="=1.0.0"` + a@1.0.0 声 `b="^2.0.0"` → pick 2.0.0 违 `=1.0.0`、报文携抬升源；missing = floor 1.5.0 源只有 1.4.0）+ E2005 ×2（cache 篡改（锁胜路径，registry 未触）/ source 漂移（再生获取路径，缓存断言含漂移副本））+ 绿路径 ×3（check-deps-green（registry 双版本 1.2.0/1.3.0 在场证 MVS 不取最新 + we.lock+cache files 断言）/ build-deps-run（run 端到端 stdout `dep`）/ deps-mvs-transitive（章文场景逐字：a 声 `b=">=1.1.0"` + root `b="~1.2.0"` → 锁断言 b=1.2.0））+ 锁生命周期 ×3（lock-created（files 断言新锁+缓存）/ lock-satisfies-offline（WE_REGISTRY=registry-missing + 温缓存 → 零源触碰，离线性质黄金钉）/ lock-stale-rewritten（`^1.2.0`→`^2.0.0` 锁重写 + 缓存只有 2.0.0 断言））+ 优先级 ×1（deps-local-wins：本地 src/util/helper.we 与 cache util@1.0.0 并存 → "local"）+ clean ×1（clean-leaves-lock——**定性钉今日即绿**，单列）+ fmt ×1（fmt-deps-no-resolve：无 env 无 registry，依赖合法 fmt 照常 4→2 空格——证不解析）+ test ×1（test-deps-green：test 模块 import 依赖模块 + 断言 stdout 两行形）+ E1302 缓存形 ×1（check-deps-e1302-cache：期望路径 `cache/some/1.2.0/src/lib/nope.we`——第三腿负例契约）。
  - **先红实证**：全量 -v 跑 = **674 PASS + 21 FAIL = 695 枚**；21 失败 = 20 新增红 + check-deps-nonempty 改写红，**红因单类**——全部 `exit: want X, got 70` + 边界 stderr `we: pipeline commands with a non-empty dependency set (chapter 22) are not implemented in this reference build yet`（21/21 grep 计数核对），零 OTHER。
  - **负向对账**：失败清单 21 枚全为 deps 面（清单落 /tmp 比对）；既有 674 枚中唯一触碰 = 计划改写件 check-deps-nonempty——边界行删除后无其他既有黄金钉依赖面，坐实 proposal 影响范围预判。clean-leaves-lock 今日即绿（clean 不读 manifest，runClean 只删 build/——结构直证，D8 预判成立）。
  - **digest 即契约**：黄金锁内 digest 由 D5 序列化定形（sha256(Σ sorted: path+\0+content+\0) 渲染 `sha256-<hex>`，python 同规则预计算）——some@1.2.0=`sha256-c941aa36…13b3` 等 7 枚入黄金逐字，T5 实现对齐此值。we.lock 机器写形逐字节钉：首行注释 + 空行 + 字典序 `[[package]]` 块（name/version/digest）块间空行、尾单换行。
  - 生成方式：/tmp 脚本（m13_goldens.py）批量生成；机械断言内嵌——e1302 锚位 1:8（`import some.lib.nope` 列位计算）、改写件 setup 逐字保全、21 枚计数；夹具族 11 包版本（some×4/a×2/b×3/c/util）。

- [x] T2 测试先行 B：单测先红——internal/deps（version/四形 constraint/ReadDeps E2006+E2003/MVS 不动点+序独立/E2001/E2002 链渲染/锁满足判定+再生/digest 确定性）+ cli m13（prepareDeps 接线序/loadGraph 第三腿与优先级/fmt 豁免/test 面 exit 2 映射）
  来源：proposal 做什么 / design D1–D6
  验证：`go test ./internal/deps/ ./internal/cli/` 红（新用例引用未定义符号/断言不匹配），red 证据记于完成记录

  完成记录（2026-09-09）：
  - **两文件落盘**：`internal/deps/deps_test.go`（14 函数：version parse/compare、constraint 四形+floor/ceiling+Allows、ReadDeps 三面+sorted-first-fault、MVS max-floor-not-latest+transitive-raise+确定性 100 轮+E2001+E2002 ceiling/ceiling-chain/missing、DirRegistry、Digest、Acquire 幂等、锁 round-trip 逐字节+畸形拒读、LockStatus wins/transitive/stale/tampered）、`internal/cli/m13_test.go`（7 函数：prepareDeps 获取+写锁逐字节/离线锁胜/E2005 逐字/E2002 链渲染逐字、depModulePath 缓存腿、fmt 豁免 Run 级、test 面 exit 2 Run 级）。
  - **red 证据**：`go test ./internal/deps/ ./internal/cli/` 双红——deps 侧 `no non-test Go files` + `undefined: Version/ParseVersion/Compare/Table`（deps_test.go:20 起）；cli 侧测试二进制因 import internal/deps 同因不编译。`go build ./...` 净（红只在测试面，符合测试先行屋形）。
  - **API 契约随测试定形**（T3–T6 按此兑现）：`deps.Version/ParseVersion/Compare/String`；`Constraint{Op,V}/ParseConstraint/Floor/Ceiling/Allows/String`（ceiling 排他、无零特殊例）；`Table` + `ReadDeps(map[string]string) (Table, *DeclError)`（DeclError{Code,Key,Val}——sorted 首fault）；`Registry{Versions/Deps}` 接口 + `NewDirRegistry`；`Resolve(root, reg) (map[string]Version, *Conflict)`（Conflict{Code,Name,Pick,Con,Chain,Raised,Raiser,Demand,Dep}——Chain 形 `root -> a@1.0.0 -> b@2.0.0`）；`Digest/Acquire`；`LockEntry/ReadLock/WriteLock`（畸形 ok=false）；`LockStatus(root, entries, cacheRoot) (LockWins|LockStale|LockTampered, roots, *Integrity{Got,Want})`。cli 侧 `e.prepareDeps(dir, table) (map[string]string, int)` + `depModulePath(dirs, imp) (string, bool)`（undeclared/std 不取缓存腿）。
  - **双证绑定**：单测 digest/锁字节/E2005/E2002 文案与 T1 黄金同值同串（someDigest=c941aa…13b3、tampered=4336…d85f、E2002 全句逐字）——一套算法两处证人，T3–T6 两侧同时翻绿即对齐证明。
  - **披露 1（D1 签名修正）**：design D1 记 `ReadDeps(manifestKeys, bare)` 双参，测试定形为单参——D1 自身的语义裁定（约束值无 [vet] 式引号敏感性：裸 token `^1.0.0` 受理、裸 `1.2.3` 天然落 E2003 文法面）使 bare 集在依赖面零消费者，双参中第二参必为死参。按语义裁定修正签名，[vet] 三键的 bare 面在 cli 既有位不动。
  - **披露 2（优先级钉位说明）**：本地胜缓存的序（ch15 字面）住 loadGraph visit 内联序，单测层由纯函数 depModulePath 钉缓存腿映射、由黄金 deps-local-wins/check-deps-e1302-cache 钉优先级序（loadGraph 需完整 parse/typecheck env，Run 级黄金即其单测面——M9a modules 先例同形）。

- [x] T3 基础塔：deps 包 version + constraint（四形 floor/ceiling/Allows）+ ReadDeps（E2006/E2003）；loadManifest 边界行换真校验
  来源：proposal 做什么 1 / design D1
  验证：`go test ./internal/deps/` 绿；E2006/E2003 黄金翻绿对账；既有黄金零回归

  完成记录（2026-09-09）：
  - **实现**：`internal/deps/` 三文件——version.go（`Version{Major,Minor,Patch}` + ParseVersion（与 E2004 同谓词：三分量无前导零）+ Compare 全数值序 + String）、constraint.go（`Constraint{Op,V}` 恰四形；Floor=操作数；Ceiling 排他——`^`→(x+1).0.0、`~`→x.(y+1).0、`=`→pick 自身、`>=` 无；无零特殊例（^0.1.0 照 <0.2.0）；Allows=`=` 等值特判 + floor 含入/ceiling 排他）、manifest.go（`Table` + `ReadDeps(manifest)`：字典序遍历 dependencies.* 键，首fault 单报——E2006（charset/[a-z0-9-]/std 保留）与 E2003（四形文法）；`DeclError{Code,Key,Val}`）。
  - **cli 接线**：check.go loadManifest 边界块（`deps > 0` → exit 70）原位换 ReadDeps 校验——E2006 双形（charset 形/`std` 保留形）与 E2003 形渲染（`.At("we.toml",1,1)` + WithHelp 注册表 remediation 逐字）；parseManifest 撤 deps 计数返回（唯一消费者是边界行）；build.go `whatNonEmptyDeps` 常量删除。序保持：校验位 = 原边界位（vet 三键后、library 工件边界前）。
  - **翻绿对账**：conformance 21 红 → **14 红**，本塔翻绿 7 枚——E2006 ×3（upper/underscore/std）+ E2003 ×3（op/leading-zero/**nonempty 改写件**——边界 70 面消亡以改写件钉死）+ fmt-deps-no-resolve（fmt 直调 loadManifest 自然获形校验：合法声明通过、解析豁免即绿——D6 预判成立）。仍红 14 = T4 面 3（e2001/e2002×2）+ T5 面 5（e2005×2/lock×3）+ T6 面 6（green/mvs/run/local/e1302/test）。既有 674 零回归（失败清单仅 deps 面）。
  - **披露 3（单测绿点后移，非缺陷）**：T2 测试文件按包整体编译（一包一测试二进制），Resolve/Conflict/Registry/Lock/Digest 符号在 T4/T5 才存在——`go test ./internal/deps/` 的绿点因此在 **T5**（今日红因 = `undefined: Resolve/Conflict/...`，恰为塔序的先红证据）；cli 包同因（m13_test 的 prepareDeps/depModulePath = T6 符号）绿点在 T6。`go build ./...` 净、gofmt 净。

- [x] T4 解析塔：registry 读取（目录枚举）+ MVS 不动点（floors 只升、序独立）+ E2001/E2002（含依赖链渲染）
  来源：proposal 做什么 2 / design D2 D3
  验证：`go test ./internal/deps/` 绿；E2001/E2002 黄金翻绿；既有黄金零回归

  完成记录（2026-09-09）：
  - **实现**：`internal/deps/registry.go`——`Registry{Versions,Deps}` 接口 + `DirRegistry`（版本枚举 = 列目录名 parse 过滤、升序；包清单只读 [dependencies]（`parseDepsSection` 惰性小读——registry 内容是已获取物非用户面，name/version/type 不校验）；缺席根 = 空源宇宙非错）+ `resolve.go`——`Resolve(root, reg)`：edges 携 (constraint, 约束方, root 锚定链)；工作循环字典序——pick = max(floors)（严格抬升才重读 manifest）、包目录缺席 → E2001、pick 无源 → E2002 缺席形（Demand=最大 floor 边）、新边自 manifest 字典序加入（约束方 = pkg@ver、链 = 父链 + 自身）；终验字典序首违即报——**Allows 判定**（pick≥floor 恒真，唯 ceiling 可违；`=` 等值特判防自违）；Conflict{Code,Name,Pick,Con,Chain,Raised,Raiser,Demand,Dep}。
  - **披露 4（D13 面：T2 链测试场景笔误）**：TestResolveE2002CeilingChain 原场景把被违约束放在 root（链恒为 "root"，深层链渲染测不出）——实现期发现期望值与算法矛盾后重构场景：被违约束落在 b@1.0.0（链 `root -> a@1.0.0 -> b@1.0.0`）、d@1.0.0 为抬升方。断言面（Con/Chain/Raised/Raiser）不变，场景修正前从未绿过。
  - **验证**：`go test ./internal/deps/` 全绿（14 函数含解析塔 8 面——max-floor-not-latest（registry-state independence）/transitive-raise/100 轮确定性/E2001/E2002 ceiling/ceiling-chain/missing/DirRegistry）。E2001/E2002 **黄金**翻绿点在 T6（渲染住 cli prepareDeps）——conformance 失败集与 T3 后同为 14 枚零漂移（无回归）。

- [x] T5 锁与获取塔：we.lock 读写（机器写形/字典序/满足判定锁胜路径/畸形再生）+ 缓存获取（复制/digest/E2005 双触发）
  来源：proposal 做什么 3 / design D4 D5
  验证：`go test ./internal/deps/` 绿；E2005 与锁生命周期黄金翻绿；既有黄金零回归

  完成记录（2026-09-09）：
  - **实现**：`internal/deps/acquire.go`——`Digest(pkgDir)`（D5 序列化：常规文件收齐、相对 slash 路径字典序、逐个 `路径+0x00+内容+0x00` 折入 sha256、渲染 `sha256-`+小写 hex——与 T1 黄金/单测双证同值）；`Acquire(regRoot, cacheRoot, name, v)`（全树复制 0644/0755 幂等）；`CacheRoot()`（WE_CACHE → 缺省 os.UserCacheDir/we/packages）/`RegistryRoot()`/`PackageDir()`；`lock.go`——`WriteLock`（首行注释 + 空行 + 字典序 [[package]] 块、块间空行、尾单换行）/`ReadLock`（缺席/非机器形/缺键 → ok=false——静默再生零诊断）/`LockStatus(root, entries, cacheRoot)`：闭包 BFS 自 root——**root 自身约束**对锁版判定 + 各锁包缓存清单（readCachedDeps）读传递边、每边约束 Allows 锁版 + 缓存在场；全过才比 digest（字典序首失配 → LockTampered + Integrity{Got,Want}）；`LockWins|LockStale|LockTampered` 三态 + 可达 roots。
  - **披露 5（D13 面：design D1 括注与章文矛盾，章文权威）**：D1 写「`^0.1.0` 的 ceiling 照 `<0.2.0`」——R2 明文「There is no zero-major special case: `^` means the same major uniformly, whatever the major's value」→ `^0.1.0` ceiling <1.0.0。实现按章文（`(x+1).0.0` 一律），T2 测试期望 0.2.0 系随 D1 括注释写错，实现期真机对出后修正为 1.0.0。规范零触碰（章文本自正确）。另两处实现期对出：WriteLock 首块前空行（黄金逐字节形）、LockStatus 漏检 root 自身约束（漂移约束误判 LockWins——测试先行面当场拦下）。
  - **验证**：`go test ./internal/deps/` **全绿**（14 函数：锁 round-trip 逐字节+畸形拒读、wins/wins-transitive/stale/tampered（Integrity 双 digest 逐字）、Digest=黄金同值、Acquire 幂等）。E2005/锁生命周期**黄金**翻绿点在 T6（同 T4 披露）；conformance 失败集维持 14 枚零漂移。

- [x] T6 接线塔：prepareDeps（loadProject + test 装载位；fmt/clean 零调用）+ loadGraph 第三腿（本地胜 → 缓存答 → E1302）+ 边界行与常量删除 + check-deps-nonempty 改写翻绿
  来源：proposal 做什么 4 / design D6
  验证：`go test ./internal/cli/` 绿；绿路径/优先级/clean/fmt/test 黄金翻绿；conformance 全量绿（674 + 21 = 695，改写件原位不计增）

  完成记录（2026-09-09）：
  - **实现**：`internal/cli/deps.go` 新文件——`prepareDeps(dir, table)`：空表零调用直返；ReadLock 可读则 LockStatus（Wins 返 roots 离线胜 / Tampered 报 E2005 / Stale 携被顶替锁落再生）；再生 = Resolve → 字典序逐 pick Acquire → Digest → **与被顶替锁的同名同版条目比对 digest，失配即 E2005 于 WriteLock 之前**（e2005-source 黄金钉此序：缓存留漂移副本、we.lock 保持旧字节）→ WriteLock → roots。`reportConflict`（E2001 单形 / E2002 双形按 Con.Op 分流——ceiling 形携 Con+Chain+Raised+Raiser、missing 形携 Demand+Dep，helps 注册表逐字）；`depModulePath(dirs, imp)`（首段查已获取根、rest 点转路径 + `.we`；std/未声明名/裸包名 false）。E2005 双触发共用 `e2005` 渲染（锚 we.lock:1:1）。
  - **装载接线**：loadManifest 三返回值（keys + **table** + code——table 在 R1 校验位原位产出，ReadDeps 单调用点）；loadProject 六返回值（+depRoots 透出给 advisory 层——projectAdvisories 的 test 模块图走同一缓存腿），prepareDeps 位 = loadManifest 后、一切源工作前（R5）；loadGraph 第五参 depDirs + 第三腿——本地 src stat 先、miss 后缓存腿（ch15 优先级），E1302 消息名期望路径（双腿同句、路径随腿——e1302-cache 黄金钉 cache 路径形）；test.go 装载位同序（loadManifest → prepareDeps（exitDiagnostic→exitCompileFailure 映射）→ 探索计数 → 发现 → 各 test 模块 loadGraph 携 roots）；fmt 表丢弃零 resolve（fmt.go 注释钉）、clean 零 manifest。m11/m10c 测试两处签名跟随；advisory/build/vet/doc 四调用点透传 depRoots。
  - **翻绿对账**：14 红 → **0 红**——绿路径 ×5（check-deps-green/build-deps-run/deps-local-wins/lock-satisfies-offline/test-deps-green）+ E2001/E2002 ×3 + E2005 ×2 + 锁生命周期 ×3（lock-created/lock-stale-rewritten 含锁重写与缓存 2.0.0-only 断言/deps-mvs-transitive 锁 b=1.2.0）+ e1302-cache。**conformance 全量 695 全绿**（-v 计数 695 PASS / 0 FAIL）；`go test ./...` 全包过；build/vet/gofmt 净。cli 包 m13_test 7 函数绿（deps 包 14 函数绿于 T5 不变）。
  - **披露 6（D13 面：T1 黄金夹具限定调用形笔误，第 15 章 R2 权威）**：七枚夹具的 .we 源用全路径限定调用（`some.lib.util.answer()` / `util.helper.who()`）——R2 明文 qualified form `name.item` 的 `name` 是 **import 名**（路径末段或别名），`import some.lib.util` 的 import 名是 `util`。T6 接线后 5 枚绿路径件被既有 E1304 面正确拦下（`the qualifier "some" of "some.lib"`）——定谳夹具错非实现错（typecheck 形为 M7/M9a 既有面、674 黄金钉死，规范零触碰）。修正：五枚 setup+期望镜像改 import 名限定（`util.answer()` / `helper.who()`）；另两枚（e2005-cache/source）死于装载前不触类型阶段、行为无差，为夹具族一致性一并修正；/tmp 生成脚本同步。七枚均属本变更 M13 面、改写发生在从未绿过的红面，T1 先红证据不受影响（红因 = exit 70 边界，非 E1304）。

- [x] T7 黑盒电池：真二进制逐面探针——离线性质（温缓存断源 build 绿）、锁重写真机、默认缓存路径（不设 WE_CACHE 落 os.UserCacheDir）、clean/fmt 不解析真机、E2005 篡改真机、回归抽检
  来源：proposal 三塔 / design D9
  验证：电池探针全过，逐探针输出记于完成记录

  完成记录（2026-09-09）：
  - **真二进制**（`go build -o /tmp/we-bin ./cmd/we`）六组探针 **17/17 全过**（/tmp/m13_battery.sh）：
    - **P1 离线性质**：温缓存 + `WE_REGISTRY` 指不存在目录 → `we build` exit 0——锁胜路径真机断源成立（缓存预热后源全程未触）。
    - **P2 锁重写真机**：`^1.2.0` build 落锁 → registry 增 2.0.0、约束改 `^2.0.0` → `we check` 绿 + we.lock 重写为 `version = "2.0.0"`（1.2.0 缓存条目留存——逐出是缓存层自身的事，锁不删缓存）。
    - **P3 默认缓存路径**：不设 `WE_CACHE`、`XDG_CACHE_HOME=<tmp>` → 解析绿且包落 `$XDG_CACHE_HOME/we/packages/some/1.2.0/...`——os.UserCacheDir 缺省链真机成立。
    - **P4 clean/fmt 不解析真机**：依赖已声明 + `WE_REGISTRY`/`WE_CACHE` 指不存在路径 → `we fmt` exit 0 且 8→4 空格重排照常、`we clean` exit 0 且 build/ 删除——两豁免命令真机零解析。
    - **P5 E2005 篡改真机**：绿路径后改缓存内容（42→99）→ `we check` exit 1 + `error[E2005]: lockfile integrity mismatch` 全句。
    - **P6 回归抽检**（零 env）：无依赖项目 check/build/run/test 四绿；未声明 import 仍 E1302 exit 1 且消息名本地腿路径 `src/nope/mod.we`（第三腿未改变既有报文面）。

- [x] T8 全量终验：阶梯九步全绿 + 对账（refr/ 零触碰、既有黄金改写恰 1、conformance 总数对齐 D8 计数（695））
  来源：proposal 影响 / design D9
  验证：`go build ./...`、`go vet ./...`、`go test ./...`、conformance 全量、`validate.py --all --strict`、docs_sync、`gofmt -l`、`git diff --check`、git status 对账——全绿记录

  完成记录（2026-09-09）：
  - **九步全绿**：① `go build ./...` 净；② `go vet ./...` 净；③ `go test ./...` 全包 ok（cli/conformance/deps 含）；④ conformance 全量 **695 枚全绿**（`-v` 计数 695 PASS / 0 FAIL，对齐 D8 计数 674+21）；⑤ `validate.py --all --strict` → `OK: 1 change(s) valid; registry clean`；⑥ `docs_sync.py` → 31 对齐；⑦ `gofmt -l .` 净；⑧ `git diff --check` 净；⑨ git status 对账（下）。
  - **对账**：refr/ 零触碰（status 无命中）；**既有黄金改写恰 1**（`M check-deps-nonempty.json`，T1 计划内改写件）+ 本变更 21 新增黄金 untracked；改动面 = internal/cli 9 文件（deps.go/m13_test.go 新增）+ internal/deps/ 新包 6 文件 + runner.go（env 面，D7 测试基建）+ 22 黄金 + openspec/changes/dependencies/。`M openspec/changes/archive/2026-09-09-ffi/tasks.md` = M12 提交（b26f385）后的提交对账追加句，M10c/M11 先例形随本变更提交带走，非本变更工件。

- [ ] T9 实现审查：welang-code-review 7 条（规范符合性/验证诚实性/测试先行证据/诊断协议稳定/单一权威/红线复核/最小可信验证已跑），产出 = proposal.md「## 审查记录」+ status → complete
  来源：仓库节奏（M10a/b/c、M11、M12 先例）
  验证：7 条逐条过，发现当场处置或披露，审查记录落盘

- [x] T10 归档：welang-archive-sync 五步（本变更零规范提升零新 ADR——机制载体 = 变更记录 + roadmap）+ roadmap M13 行 pending → done（双语）
  来源：仓库节奏
  验证：validate --all --strict 复验绿；归档目录带日期前缀

  完成记录（2026-09-09）：
  - 五步：① 规范提升零（纯实现变更，docs/spec/ 全程零触碰）；② 决策提升零新 ADR（四裁决已记于变更工件，M12 同形）；③ roadmap M13 行 pending → done 双语（docs/roadmap/0000-reference-implementation.md/.zh.md:33）；④ `openspec/changes/dependencies` → `archive/2026-09-09-dependencies`（目录未提交过，git mv 不适用 untracked，普通 mv——变更工件随本变更首次提交，M12 同形）+ change.yaml status → archived；⑤ 复验 `validate.py --all --strict` → `OK: no changes found (nothing to validate); registry clean`（active 清空）+ docs_sync 31 对齐。

- [x] T11 记忆与提交：project-overview 增 M13 全量条 + MEMORY.md 索引行更新；提交信息草案呈报，**候用户明示**
  来源：仓库节奏（提交须明示）
  验证：记忆回写；提交草案就绪；未提交状态确认

  完成记录（2026-09-09）：
  - 记忆回写：project-overview.md 增 M13 全量条（一变更全量、四裁决、六塔实现、六披露总账、电池 17/17、conformance 674→695、M13 已归档）+ MEMORY.md 索引行更新（下一里程碑 M14）。提交信息草案呈报（`Land M13 dependencies: chapter 22 resolution, lockfile, and cache`），**候用户明示**。未提交状态确认：git status = 12 M + untracked（22 黄金 + deps 包 6 文件 + cli deps.go/m13_test.go + 本归档目录 + roadmap 双语）；refr/ 零触碰。
