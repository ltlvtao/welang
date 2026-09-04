# We 语言规范 —— 第 22 章：依赖

### Requirement: 依赖声明

清单的 `[dependencies]` 表声明项目的直接依赖：每个键是一个包名、每个值是该包在本章语法下的版本约束。该表可选——不携带它的清单声明空依赖集，`we new` 不写它。键 MUST 是第 1 章命名约定下的包名——小写字母、数字与连字符——保留名 `std` 不可声明：标准库依第 15 章内建、从来不是依赖；越约的键、或 `std`，MUST 以 `E2006:` invalid dependency name 拒绝。清单自身的 `version` 字段是本章形状下的语义版本——第 21 章把形状推给生态层，本章就是那个层到来；非法值 MUST 以 `E2004:` invalid version value 拒绝。声明了而无源应答的依赖归解析去拒（本章其要求）；声明自身在此受检，不是良构约束串的值在语法要求下被拒。

#### Scenario: 缺席的表是空依赖集

- **WHEN** 一份清单不携带 `[dependencies]` 表且一个项目命令运行
- **THEN** 项目无直接依赖、解析得空图、运行照常进行——缺席是一个陈述、不是一个错误

#### Scenario: 非法依赖名被拒绝

- **WHEN** 一份清单在 `[dependencies]` 下写 `My_Lib = "^1.0.0"`
- **THEN** 工具链报 `E2006:` invalid dependency name——包名循第 1 章约定，小写字母、数字与连字符

#### Scenario: 保留的 std 不可声明

- **WHEN** 一份清单在 `[dependencies]` 下写 `std = "^1.0.0"`
- **THEN** 工具链报 `E2006:` invalid dependency name——`std` 依第 15 章名内建标准库，它从来不是依赖

#### Scenario: 项目自身的版本是语义版本

- **WHEN** 一份清单写 `version = "1.0"` 或 `version = "preview"`
- **THEN** 工具链报 `E2004:` invalid version value——字段的形状是本章固定的语义版本，第 21 章向生态层的让渡在此兑付

### Requirement: 语义版本与约束含义

版本是三段点隔的无前导零非负整数——`major.minor.patch`，按分量数值比较。pre-release 后缀与 build 元数据刻意不载：比较保持全序且纯数值，它们的不在场是本章自己的边界、不是等以后悄悄补的疏漏。约束语法恰四形，每形是版本上的一个下界：`^x.y.z`——下界 `x.y.z`、上界低于 `(x+1).0.0`，同 major 版本；`~x.y.z`——下界 `x.y.z`、上界低于 `x.(y+1).0`，同 minor 版本；`>=x.y.z`——下界 `x.y.z`、无上界；`=x.y.z`——下界与上界恰为 `x.y.z`。四形、四义、各一拼法——唯一性原则对借用 semver 生态操作符的例外，如 v0.8 已论证：这是四个不同语义各恰一拼法，不是一个语义多条路径。没有零 major 特例：`^` uniform 地指同 major、`~` uniform 地指同 minor，无论 major 的值是什么。四形之外的约束串、或操作数不是良构版本的约束串，MUST 以 `E2003:` invalid version constraint 拒绝。

#### Scenario: 四形各载其义

- **WHEN** 约束 `^1.2.3`、`~1.2.3`、`>=1.2.3`、`=1.2.3` 逐一被读
- **THEN** 它们的下界都是 `1.2.3`；上界分别是低于 `2.0.0`、低于 `1.3.0`、无、恰为 `1.2.3`

#### Scenario: 第五形被拒绝

- **WHEN** 一份清单写 `lib = "1.x"` 或 `lib = ">1.0.0"` 或 `lib = "^1, ^2"`
- **THEN** 工具链报 `E2003:` invalid version constraint——语法就是那四形，通配符、集外比较符、组合式全部在外

#### Scenario: 坏操作数被拒绝

- **WHEN** 一份清单写 `lib = "^1.2"` 或 `lib = "~01.2.3"`
- **THEN** 工具链报 `E2003:` invalid version constraint——操作数必须是良构版本，三段整数、无前导零

### Requirement: 最大下界解析

解析为每包选一个版本，规则是最大下界：图施加于一个包的每条约束贡献其下界，解析版本是这些下界的最大者。图是从根可达的每份 manifest——根的 `[dependencies]`、以及每个被解析包自身在被解析版本处读得的 manifest。规则跑到不动点：新入图的包的 manifest 可能对更远的包添下界，下界只抬升 picks，picks 只放宽图，有限的源宇宙使迭代良基。结果序无关——一个集合的最大值不依赖收集它的次序——且与 registry 状态无关：源中新版本的出现不改任何解析、直到某条约束的下界要它，因为 picks 只是 manifest 的函数。每个 pick 必须满足图中每条约束且必须存在于源中：违反任何上界的解析版本、或无源持有的被求版本，MUST 以 `E2002:` unsatisfiable dependency constraint 拒绝，报文名说包、pick、与连同依赖链的被违约束。完全无源应答的包名 MUST 报 `E2001:` unknown dependency。每包一版在全图成立——版本分裂不存在，图的两部分不能对同一个包的两个版本构建。`std` 永不入解：它内建、第 15 章的保留，不为它读任何约束。

#### Scenario: 传递下界收敛于其最大值

- **WHEN** 根声明 `a = "^1.0.0"` 与 `b = "~1.2.0"`，且 `a` 在 1.0.0 处声明 `b = ">=1.1.0"`
- **THEN** `a` 解析为 `1.0.0`——它身上唯一的下界就是其声明下界——`b` 解析为 `1.2.0`，下界 `1.1.0` 与 `1.2.0` 的最大者，它存在且满足两条约束；各一版，无论图以何种次序被走

#### Scenario: 上界违例不可满足

- **WHEN** 根声明 `a = "^1.0.0"` 与 `b = "=1.0.0"`，且 `a` 在其解析版本处声明 `b = "^2.0.0"`
- **THEN** `b` 身上的下界是 `1.0.0` 与 `2.0.0`，最大 pick `2.0.0` 违反 `=1.0.0` 上界，工具链报 `E2002:` unsatisfiable dependency constraint 名说其链

#### Scenario: 无名之名解析为无

- **WHEN** 根声明 `left-pad = "^1.0.0"` 而无源持有该名包
- **THEN** 工具链报 `E2001:` unknown dependency——声明良构，名字只是无物应答

#### Scenario: 解析序无关

- **WHEN** 两个工具链对同一 manifest 以同一可用版本集解析，或一份 manifest 的表行被重排
- **THEN** picks 相同——下界最大值是约束集的函数，不是任何遍历、搜索或排序的函数

### Requirement: 锁文件

锁文件是项目根处名为 `we.lock` 的 TOML 文件，由解析机器书写、由项目提交。它记录传递闭包：构建所需的每个包的精确解析版本与对其已获取内容的摘要。锁满足图、当图所达的每个包都有锁版、每个锁版都在源中存在、图中每条约束——根的与各锁版在自身锁版处读得的 manifest 的——被锁版满足。满足图期间锁是构建的真相——它名的精确版本就是构建用的版本，这是可复现性机制：一个项目一份锁文件的两处 checkout 对一个版本集构建，无论各自 manifest 的约束新鲜解析会得到什么。不再满足的锁——约束变了、或图长得越过锁的闭包——使解析重跑、锁以新 picks 重写。malformed 锁文件被重新生成、不被诊断：它是机器写的表面、无手改契约，坏掉的就重做。完整性检查是摘要：获取进缓存的内容 hash 不等于其锁条目的摘要 MUST 以 `E2005:` lockfile integrity mismatch 拒绝——锁记得被解析的是什么，构建拒绝不是所锁内容的内容。

#### Scenario: 满足的锁获胜、离线

- **WHEN** 项目的 `we.lock` 名的精确版本满足 manifest 的约束且每一个都在缓存中在场
- **THEN** 构建用锁定的版本、解析不跑、命令不触网络——锁是真相、缓存已足、构建可复现

#### Scenario: 失满足的锁被重解重写

- **WHEN** 一条约束从 `^1.0.0` 提到 `^2.0.0` 且一个项目命令运行
- **THEN** 锁不再满足图、解析循最大下界规则重跑、锁以新 picks 重写、构建照新 picks 进行

#### Scenario: 摘要失配败掉构建

- **WHEN** 一个包的缓存内容 hash 不等于其锁条目的摘要
- **THEN** 工具链报 `E2005:` lockfile integrity mismatch 名说该包——内容不是所锁的，构建拒绝它

#### Scenario: malformed 锁被重新生成

- **WHEN** 一份不可解析为 TOML 的 `we.lock`、或缺某被解析包条目的它，坐在项目根
- **THEN** 解析如无锁般重跑并重写文件——机器写的表面不发诊断

### Requirement: 获取与缓存

凡跑检查管线的子命令——`we build`、`we check`、`we run`、`we test`、`we vet`、`we doc`——在管线的模块解析阶段之前解析并获取：依赖缓存必须应答编译将问的每个非本地 import，否则管线在开始之前止住。`we fmt` 与 `we clean` 不解析：两者都不跑管线。获取填充依赖缓存；缓存的位置、布局、驱逐与网络协议是工具链的机制、在此无处固定——只通过什么解析了与什么失败了可观察，第 15 章的优先级原样有效：在项目 `src/` 下解析的点径名那个本地模块，缓存应答其余，`std.` 内建且对无物解析。无需新版本的获取不进行任何网络访问——满足的缓存意味着命令离线可跑，这是持续集成依赖的性质。不存在为此的任何新子命令：获取隐式内在于管线命令、第 21 章的命令面原封不动，升级一个依赖就是改它的约束再跑命令——确定性解析做剩下的。

#### Scenario: 管线命令在模块解析前获取

- **WHEN** `we check` 在 import 含 `[dependencies]` 下声明的 `some.lib.util` 的项目上运行
- **THEN** 解析与获取先完成、缓存循第 15 章映射应答该 import、管线随后恰如对一份总是满的缓存那样跑

#### Scenario: 满足的缓存不触网络

- **WHEN** 锁名的每个版本已在缓存中且机器无网络
- **THEN** `we build` 完成——获取无事可取、故无事需要网络

#### Scenario: 本地源胜过缓存

- **WHEN** `import util.helper` 既解析到 `src/util/helper.we` 又解析到依赖包 `util`
- **THEN** 本地模块确定性应答，循第 15 章优先级——本章不改它一分

#### Scenario: 命令面不变

- **WHEN** 一项提案为获取建议 `we install` 或 `we add` 子命令
- **THEN** 它必须先修订第 21 章的封闭子命令集——本章的设计是刻意的：获取隐式、模型自己写 manifest 行、命令面保持第 21 章所固定的

### Requirement: 获取不固定什么

本章指名它所留白的东西。包仓库与 publish 故事——服务器协议、包搜索、账号与认证、占名治理、任何 `we publish` 命令——不在此设计：它是需要运营现实的生态基础设施，被诚实指名为本章唯一的注册留白，设计它的变更扩展本章或认领它自己的章。镜像与代理配置是工具链的机制，缓存的布局与驱逐同样是。vendoring——把依赖复制进项目树——不固定：这里不承诺它、不禁止它、不定义它的布局。`we clean` 移除构建产物，既不动依赖缓存——它住在项目之外、其管理是机制的——也不动 `we.lock`——它不是构建产物、而是项目的记录。本章任何地方不承诺任何获取延迟、带宽或存储预算。

#### Scenario: registry 与 publish 故事是指名留白

- **WHEN** 一项变更提案声称本章设计了 registry 或 `we publish`
- **THEN** 它与本 Requirement 冲突——留白在此为一个专门变更注册，本章固定的获取止于填满的缓存

#### Scenario: clean 不动缓存与锁文件

- **WHEN** `we clean` 在缓存满、锁文件已提交的项目上运行
- **THEN** 构建产物被移除；项目外的缓存与根处的 `we.lock` 都原封不动——缓存归机制、锁文件是项目的记录、不是构建产物

#### Scenario: 无获取性能承诺

- **WHEN** 一个工具链被拿本章衡量取回延迟或缓存足迹
- **THEN** 没有东西应答——本章固定什么解析、什么失败，性能是评测层要测的、不是规范层要承诺的

### Requirement: 依赖诊断段位

依赖章拥有注册表段位 `E2000`–`E2099`，声明于 `docs/spec/diagnostics.toml` 的 `[segments]` 下，未认领区间收窄为 `E2100`–`E9999`。段内持有六条错误 `E2001` unknown dependency、`E2002` unsatisfiable dependency constraint、`E2003` invalid version constraint、`E2004` invalid version value、`E2005` lockfile integrity mismatch、`E2006` invalid dependency name——v0.8 的 `E0111` 重编号为 `E2001`、v0.8 的 `E0151` 重编号为 `E2004` 且从项目自身版本扩为任何版本值。段内未分配号码——`2000` 与 `2007`–`2099`——为本章修订保留，循第 99 章共号 space 规则无字母书写。此处无 `W` 码分配：六个发现全部止住运行、不承诺这层的任何建议性发现。触发语义在本章各 Requirement；条目在注册表。

#### Scenario: 一个依赖码被发射

- **WHEN** 工具链发射任何 `E20xx` 诊断
- **THEN** 其完整条目可在 `docs/spec/diagnostics.toml` owner `2200-dependencies` 下检索

#### Scenario: 后来的变更需要本段位码

- **WHEN** 本章未来的修订需要一个新诊断
- **THEN** 它在同一变更内扩展 `E2000`–`E2099` 内的注册表、或认领它自己的段位

## 示例（非权威）

下列示例只用第 1–22 章已批准的表面形式。它们是说明性的、非权威的：任何冲突处以 Requirement 与 Scenario 为准。带诊断码注解的行是被拒绝的形式，展示工具链发射的码。

### 声明依赖

```toml
# file: we.toml — the [dependencies] table chapter 22 fixes; absent it is
# an empty dependency set, and we new writes none
name = "web"
version = "0.2.1"
type = "executable"

[dependencies]
router = "^1.4.0"        # floor 1.4.0, ceiling below 2.0.0 — same major
codec = "~2.1.3"         # floor 2.1.3, ceiling below 2.2.0 — same minor
logging = ">=0.3.0"      # floor 0.3.0, no ceiling
testkit = "=1.0.0"       # floor and ceiling exactly 1.0.0
```

```toml
# My_Lib = "^1.0.0"                      // E2006: invalid dependency name
# std = "^1.0.0"                         // E2006: invalid dependency name
# lib = "1.x"                            // E2003: invalid version constraint
# lib = "^1.2"                           // E2003: invalid version constraint
# version = "1.0"                        // E2004: invalid version value
```

### 最大下界解析

```toml
# the root asks:
#   a = "^1.0.0"
#   b = "~1.2.0"
# and a at 1.0.0 asks:
#   b = ">=1.1.0"
#
# floors on a: 1.0.0            -> a resolves to 1.0.0
# floors on b: 1.1.0, 1.2.0     -> b resolves to 1.2.0 (exists, satisfies both)
```

```toml
# the root asks:
#   a = "^1.0.0"
#   b = "=1.0.0"
# and a at 1.0.0 asks:
#   b = "^2.0.0"
#
# floors on b: 1.0.0, 2.0.0     -> pick 2.0.0 violates the =1.0.0 ceiling
#                                      // E2002: unsatisfiable dependency constraint
# left-pad = "^1.0.0"           // E2001: unknown dependency (no source answers)
```

### 锁文件

```toml
# file: we.lock — machine-written, committed by the project; the exact
# versions and digests of the transitive closure
[[package]]
name = "a"
version = "1.0.0"
digest = "sha256-9f2c…"

[[package]]
name = "b"
version = "1.2.0"
digest = "sha256-4b8e…"
```

```sh
we check        # lock satisfies the graph + cache holds all -> locked versions, no network
# constraint raised ^1.0.0 -> ^2.0.0, we check again:
#                resolution runs, we.lock rewritten with the new picks, build proceeds
# cached content hashes against its lock entry:
#                                        // E2005: lockfile integrity mismatch
```

### 获取与缓存

```we
// file: src/main.we — the import that reaches the cache
import router.route

fn main() effect io -> Result<(), E> {
    return Ok(route(handle))             // router resolves from the cache under
}                                        // chapter 15's mapping; local src/ wins
                                         // over any same-named cache package
```

```sh
# we fmt          — no resolution, no pipeline
# we clean        — removes artifacts; the cache and we.lock stand untouched
# we install      — the shell's error: no such subcommand, chapter 21's set is
#                   closed and this chapter adds none — write the manifest row
```

### 待后续变更

```toml
# The registry and publish story is the named gap: server protocol, search,
# accounts and auth, name policy, and any we publish command — a dedicated
# change extends this chapter or claims its own. Mirrors, proxies, cache
# layout and eviction are the toolchain's mechanism, unfixed here.
```

## 术语对照

本章关键术语，英中对照，用于翻译一致性：

| English | 中文 |
| --- | --- |
| dependency | 依赖 |
| package name | 包名 |
| version constraint | 版本约束 |
| semantic version | 语义版本 |
| floor | 下界 |
| ceiling | 上界 |
| resolution | 解析 |
| maximum of floors | 最大下界 |
| fixed point | 不动点 |
| lockfile | 锁文件 |
| digest | 摘要 |
| integrity mismatch | 完整性失配 |
| dependency cache | 依赖缓存 |
| acquisition | 获取 |
| transitive closure | 传递闭包 |
| offline build | 离线构建 |
| registry | 包仓库 |
| named gap | 指名留白 |
| vendoring | 源内嵌仓 |
