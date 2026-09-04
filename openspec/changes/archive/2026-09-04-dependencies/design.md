# Design: 依赖获取（第 22 章）

## 裁决依据

四项用户裁决（2026-09-04，均采推荐）锁定设计空间：

1. **约束+解析+锁+获取**：manifest 语法、确定性解析、`we.lock`、获取行为入章；registry/publish 显式留白。
2. **四写法 + semver 定形**：`^`/`~`/`>=`/`=` 四形；版本=semantic version；own-`version` 同步兑付。
3. **零新子命令**：隐式获取，ch21 命令面零修订。
4. **MVS 最大下界**：每包全图恰一版=下界最大者。

## 设计决策

### D1 章的立场：仍是可观察契约

沿第 21 章纪律：固定观察到什么（声明检查的判定、解析的结果函数、锁的字段与胜出规则、零网络可观察性、失败码），不固定怎么做到（遍历算法的实现、缓存的布局、摘要的具体算法、网络协议）。解析算法是本章最接近"机制"的东西，但 MVS 的**结果函数**是可观察契约（同一 manifest+同一 sources → 同一 picks），固定结果不固定遍历——规范写"到不动点的迭代良基"，不写迭代序。

### D2 版本形状的最小全序

版本=三段无前导零非负整数。**刻意不载** prerelease/build metadata：semver 的 prerelease 排序规则（标识符分字典序/数字序、连字符语义）是全序上的疤痕——载入即付出比较复杂度与语法面，换来的发布通道管理属生态运营问题，归 registry 那个 named gap 的变更考虑。三段整数的数值比较**total 且 trivial**，MVS 的数学（max、floor、ceiling）在其上无死角。这是收窄不是遗漏：R2 场景明说"their absence is this chapter's own boundary"。

### D3 四形语法的 floor/ceiling 表

| 形 | floor | ceiling | 语义 |
| --- | --- | --- | --- |
| `^x.y.z` | x.y.z | < (x+1).0.0 | 同 major |
| `~x.y.z` | x.y.z | < x.(y+1).0 | 同 minor |
| `>=x.y.z` | x.y.z | 无 | 只设下界 |
| `=x.y.z` | x.y.z | == x.y.z | 恰等 |

两条纪律：**无零_major 特例**（cargo 的 `^0.y.z` 特例是生态历史包袱，v0.8 只批了四写法没批特例；一条规则uniform适用whatever the major's value——P4 消歧义）；**无组合子**（逗号合取、`||` 析取、通配符全部在外——唯一性原则：四形四义各一拼法，v0.8 §3.1 例外的原文论证）。

### D4 MVS：为什么最大下界而非最高满足版

备选是 cargo 式"每包取满足全部入边的最高版"。选 MVS 的账：

- **按构造无冲突**：floor 集合的 max 永远存在（整数全序），唯一失败模式是 max 违反某 ceiling 或所求版本不存在——一个错误码（E2002）覆盖全部失败面；cargo 式还要处理"多约束交集为空"的判定与报文组合学。
- **registry 状态无关**：picks 是 manifest 的函数，不是 registry 快照的函数。新版本发布不改任何既有解析——这对 AI 原生工作流是硬性质：模型读 manifest 即知解析结果，无需查询 registry 状态。
- **升级语义诚实**：你要新版，就把下界提到你要的版本（`^2.0.0` 就是"我要 2.0.0 起"）——所见即所锁，没有"静默跳到最新"的惊喜。与零子命令裁决（升级=手改约束）互为表里。
- 代价：minor 修复不会自动跟进——用户显式改约束。这是裁决四接受的代价，design 记账。

### D5 零子命令与获取触发面

哪些命令解析？判据=**是否跑检查管线**：管线有模块解析阶段，缓存必须先答。build/check/run/test/vet/doc 六个跑管线；fmt（纯文本变换）与 clean（删产物）不跑。这张表是可观察契约不是实现细节（用户能问"为什么 we fmt 不报 E2001"——答案是它不解析）。`we install` 消亡的论证：获取是管线的前置义务而非独立动作；AI 原生故事里模型直接写 TOML 行（第 21 章 D5 已立"manifest 是模型可写表面"）；ch21 R1 封闭集的修订成本（开修订=破坏"this chapter fixes the set"的已批语义）远大于一个便利子命令的价值。

### D6 锁文件：机器写、项目提交、满足即胜

- **满足即胜**：锁满足当前图 → 用锁的精确版本，不跑解析。这是可复现性的机制本体：两台 checkout 同一锁即同版本集，无论各自约束新鲜解析会得到什么。
- **失满足重解**：约束变/图变 → MVS 重跑 → 锁重写。锁是缓存而非宪法。
- **损坏静默重生成**：机器写表面无手改契约，malformed 锁不配诊断码（与 E1905 的 manifest 不同——manifest 是人写表面，缺键要报）。判据一句话：**人写表面报错，机器写表面自愈**。
- **摘要完整性**：锁记每包内容摘要；缓存内容 hash 不符 → E2005。摘要算法=机制（sha256 出现在示例但不入 Requirement 正文——规范不钉算法，P7 机制中立的既有纪律）。

### D7 六码账

| 码 | title | 触发归 | v0.8 去向 |
| --- | --- | --- | --- |
| `E2001` | unknown dependency | R3 解析：无名应答 | `E0111` 重编号 |
| `E2002` | unsatisfiable dependency constraint | R3 解析：上界违例/版本不存在 | 新（MVS 唯一失败模式） |
| `E2003` | invalid version constraint | R2 语法：越形/坏操作数 | 新（v0.8 无此码，§56 规则 2 隐含） |
| `E2004` | invalid version value | R1：own-version 非三段 | `E0151` 重编号+扩义（任何 version 值） |
| `E2005` | lockfile integrity mismatch | R4：摘要失配 | 新 |
| `E2006` | invalid dependency name | R1：键越约/`std` | 新（v0.8 键规则"小写+数字+连字符"的兑付） |

无 W 码：六个发现全部止损（获取失败=管线不跑），无建议性发现承诺。段内保留 `2000`、`2007`–`2099` 无字母书写（共号 space，本段无 W 码但保留号书写纪律与 E19xx 段一致）。

### D8 宿主修订的必要性账（恰两处）

- **ch21 R8 必修**：三句话变假——"version … the ecosystem's to constrain"（生态层到了，形状已定）；"What the manifest does not yet carry: dependency declarations…"（携带了）；场景 "this specification fixes nothing about it"（固定了）。同场景重写为指向第 22 章的检查语义。
- **ch21 R11 必修**：首句整句是获取留白声明，留白填上即假；其场景 "The acquisition gap is named, not hidden" 的 WHEN（"a change proposal claims this chapter designed dependency acquisition"）现在为真命题——第 22 章确实设计了它。场景随句移除（R11 场景 3→2），LSP/本地化/性能句原样。
- **ch15 R1 零修订**："acquisition, version constraints, and lockfiles are the tooling layer's business, not this chapter's"——真前瞻指针（写归宿不写内容），落地后依然为真（slice 6 D2 同型论证）。
- **ch21 R1 零修订**：零新子命令裁决的直接红利——"this chapter fixes the set" 不动。
- **ch21 示例两块注释刷新**：manifest 块 "[dependencies] table is not designed yet"（假）与 pending 块 "[dependencies] is the named gap"（假）——示例非权威但不可说谎，随宿主修订同步刷新（EN+zh 字节同步）。
- **ch21 R8 场景 "we new writes the skeleton"**："no tables the toolchain does not define"——`[dependencies]` 现在是规范定义的表，但 we new 仍不写它（可选表、缺省空集），句子读作"创建的骨架不含工具链不定义的表"依然为真，不动（已逐词复核）。

### D9 获取与缓存的边界

ch15 的三优先级（本地 `src/` 胜、`std.` 内建、其余走缓存同映射）原样承继——本章不改解析映射只填缓存。缓存的位置/布局/驱逐/网络协议=机制。**零网络可观察性**："缓存满足则命令离线可跑"是 CI 依赖的性质，值得一句可观察承诺（不是性能承诺——是行为承诺：needs nothing → touches no network）。

### D10 v0.8 映射账

| v0.8 | 去向 |
| --- | --- |
| §65 高优 3 四件套 | 约束→R2；解析→R3；`we.lock`→R4；install→**消亡**（裁决三）；publish/registry→R6 named gap |
| §56 规则 2（表/键/值/四写法） | R1+R2 全承 |
| §56 规则 1（`[project]` 表、we-version、E0153） | **续死**——ch21 已批扁平骨架拒斥 `[project]` 表；we-version 随之 |
| §56 规则 6（E0154 闭合 schema） | **续死**——ch21 软姿态（未知表不拒不定义）承继；本章 `[dependencies]` 从"不定义"变"定义"，其余未知表姿态不变 |
| §3.1 唯一性例外（四写法借用论证） | R2 原文承继 |
| E0111 | →`E2001` |
| E0151 | →`E2004`（own-version 扩为任何 version 值：约束操作数归 E2003、版本值归 E2004，一码一义两清） |
| semver prerelease/build | **不载**（D2 收窄） |
| cargo `^0` 特例 | **不载**（D3 uniform） |

### D11 三要素账（原则 10）

- **类型检查**：零参与——六码全是工具层判定（manifest 值域、语法形、图求解、摘要比对），无新语言表面（零关键字、零文法、零类型）；`import` 路径解析是 ch15 既有语义，本章只保证缓存能答。
- **代码生成**：零新面——解析与获取全在管线之前；产物的依赖链接故事属构建机制。
- **运行时**：三件事全在工具运行时——解析器（MVS 不动点）、缓存管理、锁文件读写与摘要校验；语言运行时（GC/调度/netpoll）零接触。章界干净。

### D12 与第 21 章的接缝

- JSON Lines：获取/解析失败是 diagnostic 事件，循 R7 字段集与稳定性承诺——本章零新字段。
- `we new` 骨架：不含 `[dependencies]`（空集合法），ch21 R8 场景已覆。
- `we clean`：ch21 定义为"删产物、不动源与清单"；本章补一句不动缓存（项目外）与 `we.lock`（非产物而是项目记录）——R6 场景承载，ch21 R1 该句零改动（"sources and the manifest untouched"未提锁，本章补义不冲突）。
- 探索/测试/vet：零交互——依赖图在管线之前定型，探索的确定性承诺（ch20/ch21 R6）在缓存定型之后才开始计时。
