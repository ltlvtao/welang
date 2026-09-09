# design — dependencies（M13）

四裁决（2026-09-09，均采纳推荐）：Q1 一变更全量 / Q2 WE_REGISTRY 目录树 / Q3 WE_CACHE + 复制 + sha256 / Q4 Case schema 增 env 面。零规范增量（ch22 已权威，diagnostics.toml E2001–E2006 在册——title/description/remediation 逐条在案，本设计引其原文要素）。

## D1 新 `internal/deps` 包：版本与约束

包结构（一目录五文件，皆纯函数无 CLI 依赖——诊断由调用侧渲染）：

- `version.go`：`Version{Major, Minor, Patch int}` + `ParseVersion(s) (Version, bool)`（三分量非负整数无前导零——与既有 `cli.validVersion` 同谓词，实现归一于此、cli 侧保留薄壳或改引）+ `Compare(a, b) int`（全数值序——prerelease/build 故意不载，ch22 R2）+ `String()`（渲染回 `x.y.z`）。
- `constraint.go`：`Constraint{Op string, V Version}`，Op ∈ {`^`, `~`, `>=`, `=`} 恰四形；`ParseConstraint(s) (Constraint, bool)`——首段运算符后随合法版本串，其余一切（裸版本、`>1.2.3`、`v1.2.3`、`^1.2`、`^01.2.3`、空）拒；`Floor()`/`Ceiling() (Version, bool)`——`^x.y.z` floor x.y.z ceiling 排除 `(x+1).0.0` 及以上、`~x.y.z` ceiling 排除 `x.(y+1).0` 及以上、`>=` 无 ceiling（false）、`=` floor=ceiling；无零特殊例（cargo `^0` 不存在——`^0.1.0` 的 ceiling 照 `<0.2.0`）；`Allows(v)` = floor ≤ v 且（无 ceiling 或 v < ceiling）。
- `manifest.go`：`Table(map[string]Constraint)` + `ReadDeps(manifestKeys map[string]string, bare map[string]bool) (Table, *DeclError)`——自 `dependencies.*` 前缀键集（parseManifest 既有捕获面）取条目；键校验 = E2006 谓词（非空、字符集恰 `[a-z0-9-]`——与 validProjectName 同集、`std` 保留拒）；值校验 = E2003 谓词（四形 ParseConstraint；裸未引用值天然落 E2003——`foo = 1.2.3` 的值串无运算符前缀，无需 [vet] 式引号敏感性）。E2004 不涉（own version 校验留 cli 既有位）。DeclError 携码与键名，调用侧渲染。

## D2 registry 夹具形（Q2 裁决）

`WE_REGISTRY` 环境变量指本地目录；未设或目录不存在 = 空源宇宙。

- 包内容：`<registry>/<name>/<version>/{we.toml, src/…}`——版本枚举 = 列 `<registry>/<name>/` 下目录名（parse 为 Version，非法目录名跳过）；包缺席 = E2001（名无源应答）；版本缺席进解析（D3 的 E2002 面）。
- 包清单只读 `[dependencies]` 表（同一 parseManifest 谓词）；其 name/version/type 不再校验——registry 内容是已获取物，非用户面（ch22 R3 只说「read at its resolved version」）。
- 每次解析按需读、无索引缓存——目录即 registry，「获取协议 = 机制」的参考取值（真机可用、conformance 可复刻、零服务器）。

## D3 MVS 解析：不动点与 E2001/E2002

算法（ch22 R3 逐句兑现）：

1. 边集 `edges`：每约束记录 (target 包名, Constraint, 约束方 = root 或 `pkg@ver`)；`picks` 起始空。
2. 工作循环：对每包取「已见 floors 的最大版本」为 pick；pick 变更时读 `<registry>/<name>/<pick>/we.toml` 的 [dependencies]（包目录缺席 → E2001；版本目录缺席 → E2002「demanded version no source holds」），新约束入 edges、其 floors 抬升对应 picks；floors 只升、图只宽、源宇宙有限 → 必然不动点（章文原句）。
3. 终验（包名字典序遍历，首个违规即报——panic-stop 屋形）：pick 违任一 ceiling → E2002。不动点 = floors 最大值的函数 → 序独立、注册表态独立（新版本入源不改 picks，直至某 floor 要求——章文「picks are a function of the manifests alone」）。
4. `std` 在 ReadDeps 键校验已拒（E2006），永不入解析。

诊断形（house 形 `we.toml:1:1: error[E….]: title — detail`，help = 注册表 remediation 逐字）：

- E2001：`unknown dependency — %q answers to no package in any source`（名包）。
- E2002（ceiling）：`unsatisfiable dependency constraint — package %q resolves to %s, violating %q required along %s`，链 = root 锚定的 `root -> a@1.0.0 -> …` 路径（约束方至 root 的记录边回溯）；同句缀抬升源 `; the pick was raised by %q from %s`（注册表「the violated constraint with its dependent chain」三要素齐）。
- E2002（版本缺席）：`… — package %q resolves to %s, which no source holds (demanded by %q from %s)`。

黄金定形即契约（T1 落盘、T3/T4 实现对齐；实现期以真机输出更正黄金笔误 = D13 披露屋形）。

## D4 we.lock：满足判定、再生与 E2005

- 形：项目根 `we.lock`，机器写 TOML——首行注释 `# We lockfile — machine-written by resolution; commit it, do not edit.` + 每包一 `[[package]]` 块（name/version/digest 三键），包名字典序（确定性）。闭包空 → 不写不删（无可锁物；遗留锁满足性空真、无害搭乘）。
- **锁胜路径（离线性质，ch22 R4 场景「A satisfying lock wins, offline」逐字）**：锁可解析时，自 root 清单 + 各锁包@锁版清单（读**缓存副本**，零 registry 触碰）走可达闭包；全部可达包有锁项 + 全部约束（floor/ceiling）被锁版满足 + 各包缓存在场且 digest 合 → 锁即构建真值，解析不跑、registry 不碰。「锁版在源存在」子句的离线读法：digest 合的缓存条目就是该版本存在于源的物证（获取时刻的源应答被锁钉死）——E2002 的存在性检查只在真解析跑时触源。
- **再生路径**：锁不可解析 TOML、或缺可达包条目、或约束不满足、或缓存缺席 → 当锁不存在，跑 D3 全量解析 + D5 获取 + 重写 we.lock（畸形零诊断——机器写面无手编辑契约，章文「regenerated, not diagnosed」）。多余条目（图不再达）不阻碍满足——锁胜路径搭乘；下次真解析时重写自然收敛。
- **E2005 双触发**（注册表「Content acquired into the dependency cache does not hash to its we.lock entry's digest」）：① 锁胜路径上缓存内容 digest ≠ 锁条目（篡改缓存）——`we.lock:1:1: error[E2005]: lockfile integrity mismatch — cached %q %s hashes to %s, we.lock records %s`（名包，注册表要素）；② 再生获取路径上锁满足约束但源内容 digest ≠ 锁条目（源漂移）——同码同形，锚同 we.lock:1:1。两形皆拒构建。

## D5 获取、缓存与 digest（Q3 裁决）

- 缓存根 = `WE_CACHE` env；未设 = `os.UserCacheDir()/we/packages`（跨项目共享层——ch22 R6「we clean 不触缓存」的语义预设缓存住项目外）。布局 = registry 镜像：`<cache>/<name>/<version>/{we.toml, src/…}`。
- 获取 = registry→cache 目录复制（`<registry>/<name>/<version>/` 全树逐文件 `0o644`/目录 `0o755`）；已在场不重取（幂等）。锁胜路径缓存全在场 = 零 registry 触碰 = 零网络（本地目录参照下「网络」即源——CI 性质的参考兑现）。
- digest = sha256 确定性序列化：walk 包目录收全部常规文件，相对路径（slash 归一）字典序排序，逐个折入 `路径字节 + 0x00 + 内容字节 + 0x00`；渲染 `sha256-` + 小写 hex。夹具内容固定 → 黄金可断言逐字 digest。算法 = 机制（E2005 注册表明文「The digest algorithm itself is the toolchain's mechanism」），此处定参考取值。
- 解析后获取序：逐 pick 包——缓存缺席 → 复制 → 算 digest；缓存在场 → 算 digest 对锁（有锁且该包有锁项）→ 不合 E2005；无锁（首解析）→ 以新算 digest 写锁。

## D6 CLI 接线：六命令、fmt 例外与 loader 第三腿

- **manifest 校验面（全读者）**：`loadManifest` 内 deps>0 边界行（check.go:236）与 `build.go:23` 常量删除，原位换 ReadDeps 校验（E2006/E2003——声明形对一切读清单者成立，含 fmt；fmt 的 loadManifest 直调因此自然获得形校验而不触解析）。
- **解析获取面（管线六命令）**：新 `prepareDeps(dir, table) (map[string]string, int)`——返回 包名 → 缓存包根路径（`<cacheRoot>/<name>/<version>`）与退出码；内部序 = D4 锁胜判定 →（不满足）D3 解析 → D5 获取 → we.lock 重写。接线位：`loadProject`（check/build/run/vet/doc 共享入口，loadManifest 之后 loadGraph 之前——R5「before the pipeline's module-resolution stage」）+ `test.go` 装载位（同序；退出码经 exitCompileFailure=2 映射——M10b 既有门同形，E19xx 诊断面 1、test 面 2）。`fmt`/`clean` 零调用（R5「neither runs the pipeline」——clean 结构即合规，fmt 由调用位豁免）。序保持：prepareDeps 先于 library 工件边界（现 deps 边界同位——行为序零改写）。
- **loadGraph 第三腿**：import `a.b.c` 解析序 = ① `std` 首段 → StdModule（既有）；② `src/a/b/c.we` 本地（既有，ch15 优先级「local sources win over the cache」——本地胜字面）；③ 缺席且 `a` ∈ depDirs → `<cache>/<a>/<ver>/src/b/c.we`（缓存模块走同一 parse/typecheck 机器入图，无特权路径——初始化序/环检测/E1301 既有三色 DFS 自然覆盖）；④ 仍缺 → E1302 既有报文（期望路径 = 第三腿缓存路径或本地路径如实，help「add the dependency to the cache」本就为此写）。非声明名维持现状（本地查找 E1302）。
- 源自缓存模块的 E13xx/E0xxx 诊断锚 = 缓存内真实路径（渲染走既有 normalize 面）。

## D7 conformance env 面（Q4 裁决）

- Case schema 增可选 `Env map[string]string`（JSON 键 `env`）；`Execute` 进入 workdir 前逐键存旧值（含缺席）、`os.Setenv`、defer 恢复（缺席恢复 `os.Unsetenv`）——黄金以相对值 `"WE_REGISTRY": "registry"`、`"WE_CACHE": "cache"` 指 case setup 夹具目录（CLI 的 cwd = workdir，相对 env 自然解析；输出 normalize 后呈 `<dir>/cache/…` 确定性形）。runner 改动 = 测试基建非编译器行为（测试先行不涉）。
- 夹具落 setup：`registry/<name>/<version>/{we.toml, src/…}` 与断言 `files`（we.lock 逐字节断言含 digest 行）。

## D8 黄金矩阵定数（21 新增 + 1 计划改写 = 22 枚涉及；总数 674 → 695）

- **E2006 ×3**：check-deps-e2006-upper（`MyLib`）/ -underscore（`my_lib`）/ -std（`std` 保留）。
- **E2003 ×3**：check-deps-nonempty **计划改写**（裸 `1.2.3` → E2003 面，文件名沿用——M5 bnd-ch8/M6a check-bnd-list 改写先例）/ check-deps-e2003-op（`>1.2.3` 非四形新增）/ -leading-zero（`^01.2.3` 新增）。
- **E2001 ×1**：check-deps-e2001-unknown（名不在册）。
- **E2002 ×2**：-ceiling（章文场景逐字：root `b = "=1.0.0"` + `a = "^1.0.0"`、a@1.0.0 声明 `b = "^2.0.0"` → pick 2.0.0 违 `=1.0.0`）/ -missing（floor 版本源无）。
- **E2005 ×2**：-cache（锁胜路径篡改缓存）/ -source（锁满足但源内容漂移，获取路径）。
- **绿路径 ×3**：check-deps-green（一依赖、缓存答 import、check 静默 0 + we.lock 落盘断言）/ build-deps-run（build+run 端到端——main import 依赖模块、二进制 stdout）/ deps-mvs-transitive（章文场景逐字：a@1.0.0 声明 `b = ">=1.1.0"`、root `b = "~1.2.0"` → we.lock 断言 b=1.2.0）。
- **锁生命周期 ×3**：lock-created（首跑写锁——files 断言）/ lock-satisfies-offline（缓存温、WE_REGISTRY 指不存在路径、build 绿——离线性质黄金钉）/ lock-stale-rewritten（`^1.0.0` → `^2.0.0` 后重跑、锁重写断言）。
- **优先级 ×1**：deps-local-wins（import 同时命中本地 src/ 与依赖包——本地答，章文场景逐字）。
- **clean ×1**：clean-leaves-lock（we clean 后 we.lock 与缓存俱在——files 断言）。
- **fmt ×1**：fmt-deps-no-resolve（依赖声明合法、registry 未设——fmt 照常格式化绿，证明不解析）。
- **test ×1**：test-deps-green（test 编译含依赖模块，exit 0）。
- **E1302 缓存形 ×1**：check-deps-e1302-cache（声明包内 import 未命中——第三腿负例契约，期望路径 = 缓存路径；E1302 既有码新报文形）。
- 分桶合计：E2006 3 + E2003 3（含 1 改写）+ E2001 1 + E2002 2 + E2005 2 + 绿路径 3 + 锁 3 + 优先级 1 + clean 1 + fmt 1 + test 1 + E1302 缓存形 1 = 22 枚涉及 = 21 新增 + 1 改写；clean-leaves-lock 为定性钉（clean 现即合规——今日即绿、防回归性质，T1 红因分类单列）。预注册零认领：六码 helps 逐字引注册表（c5efd3a 规范落地期预注册，M12 同形）。

## D9 验证阶梯

黑盒电池（T7）面：真二进制 + 真磁盘 registry/cache——离线性质探针（温缓存 + 断源 build 绿）、锁重写真机、默认缓存路径探针（不设 WE_CACHE 落 `~/.cache/we/packages`）、clean/fmt 不解析真机、E2005 篡改真机。终验（T8）九步：`go build ./...` / `go vet ./...` / `go test ./...` / conformance 全量（674 + 21 新 = 695，改写件原位不计增）/ `validate.py --all --strict` / docs_sync / gofmt -l / `git diff --check` / git status 对账（refr/ 零触碰、既有黄金改写恰 1）。
