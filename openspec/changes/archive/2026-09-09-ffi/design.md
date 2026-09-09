# design — ffi（M12）

章文权威：docs/spec/1900-ffi.md（9R/37S）+ docs/spec/2100-toolchain.md R12 linkage。注册表 E1701–E1707（owner 1900-ffi）+ E1906（owner 2100-toolchain）在册，title/remediation 逐字转录进 helps。

## D1 AST 与 parser 产生式

- `ast.ForeignBlock{ABI string; Line, Col int; Items []Item}`——条目复用既有 Item 形，两个标记位：
  - `FnDecl.Foreign bool`（codegen 直呼判定 + mock 面排除 + 无体事实——foreign 条目的 Body 恒 nil）；
  - `RecordDecl.Opaque bool`（E1707 拦截 + codegen 单指针布局 + 跨界集判定）。
  标记位优于「回溯父节点」判定：遍历器（doc/fmt/typecheck 走查）无需携带上下文。
- `parseTopItem` 的 `"foreign"` case 从 `p.bnd(bndFFI)` 改 `parseForeignDecl`：
  - `foreign` + 字符串字面量：非字面量或 ≠ `"c"` → E1701（`foreign block's ABI string is not "c"`）；
  - 块 = ch2 花括号块（行接续照常——括号内换行不敏感）；
  - 块内条目集封闭：`[pub] fn`（无体、段必写、禁泛型）+ `[pub] record Socket { }` / `byval record` / `byres record`（零字段强制）。其余一切（let/effect/type/interface/impl/import/语句）→ 既有 E0105（块 holds foreign function declarations and opaque type declarations, nothing else）；带字段 record → E0105（opaque 声明是 fieldless record 复用，有字段非此产生式）；fn 带体 → E0105。
- E1703：fn 条目解析到无段（`->` 后直达块尾或直接块尾）→ E1703。段解析复用 ch6 序机器加语境参数 `inForeign bool`：**允许零标签**（裸 `effect` 后直接 `->`/块尾）——仅此语境；顶层 fn 裸段维持既有拒绝。
- E1704：`fn name<T>` 于 foreign 块内 → E1704（泛型子句探测先于参数表）。
- E1702：块/语句位派发表对 `foreign` 关键字 → E1702（今日块内 foreign 落 E0105 泛形——M12 换专码；parser.go:1898 关键字集挂接）。
- E0404：parseTopItem 名字表挂接——foreign 块条目名（fn 名 + record 名）与顶层名同表；跨块重名/与顶层重名 → 既有 E0404。
- 41 关键字派发表（parser_test dispatchTable）：foreign 行从「bndFFI 边界」改「parseForeignDecl」，表测试同步。

## D2 typecheck：ingest 与跨界集

- ingest（pass1 走查）：`ForeignBlock` → 展开条目入模块符号表——foreign fn（Body nil 不入体走查队列；效果集 = 声明段逐 tag 走既有 resolveEffectTag：内建裸键/自有 tag/合格导入 tag；pub 按条目位）、opaque record（类别照 catOf：gc/value/resource）。
- **跨界集检查器**（声明位逐条目、参数与返回分位）：
  - 参数位合法集：Int8/16/32/64、UInt8/16/32/64、Float32、Float64、Bool、Rune、String、Bytes、opaque 命名类型（NamedType 解析到 Opaque RecordDecl——本模块直接/跨模块经 ch15 pub 门 importSym 到达）。
  - `Never` 参数位 → E1705（章文明文 included）。
  - 返回位合法集 = 参数集 − {String, Bytes} + {Never}；String/Bytes 返回 → **E1706 专码先于 E1705**（专形先报）；其余非法（带字段 record/sum/tuple/List/Map/Set/Option/Result/fn 型/Dyn/泛型参数/未知名）→ E1705。
  - 锚位：签名内非法类型 token 位（复用 E0501 同族锚法——类型注解锚名 token）。
- **E1707**：构造位 constructType 与更新位（with &base）——目标 NamedType 解析到 Opaque → E1707，先于 E0604/E0603 既有判（专形先报）。字段访问 opaque → 既有 E0604（零字段无名可查，天然命中）。
- byres opaque 全纪律 = ch13 既有机器零新规则：ingest 后真模块尾 E1101 配对查照发；E1105 别名/var/赋值；E1106 七组合位（catOf 判资源照常）；三通道转移（scope 头/return/实参位）照常；impl Releasable 体 = 普通受检代码（体内调 foreign close 是普通调用）。
- mock 面：mock 目标解析到 Foreign fn → E1804 排除（E1804 的不可 mock 类目面 +1 披露——ch20 七类目之「模块级单态 fn」集合 M12 收窄；remediation 面实现期对表注册表措辞）。
- 顶层 init（E1405）：foreign 调用带 tag 照发；裸段纯调用零账（空集早退既有机制）。

## D3 效果与调用（零新机制直证）

foreign 调用 = 普通调用：fnCall 位 callee 符号 = foreign 声明，效果集 = 声明集。E1401 三臂判序照常（空集早退 → 裸段纯调用天然放行；inClosure 收集；差集报）。自定义 tag（`effect gpio`）段的声明与调用两端全走既有机器。Never 返回调用 = 发散：既有 Never 类型事实（满足任意返回位/臂一致/臂掉出）零改动。

## D4 codegen：declare 与 ABI

- `EmitProgram` 遇 `ForeignBlock`：fn 条目 → `declare <ret> @<name>(<params>)`（模块键不限定——C 符号唯一，跨模块同名声明合并同 declare）；opaque record → 无类型发射（值形 = 单指针槽）。
- 调用位：callee 是 Foreign fn → `call <ret> @<name>(<args>)` 直呼（**无槽**——M10b 槽面只管 We 侧 fn 与 std 条目；mock 非目标直证）。
- **位置 ABI 映射**（Q4 裁决载形）：Int8/16/32/64 与 UInt 同宽 → i8/i16/i32/i64；Float32/64 → float/double；Bool → i8（C `_Bool` 同宽）；Rune → i32；String/Bytes → **(ptr, i64) 双标量参数展开**——一个 We 参数展开为两个 IR 参数（C 原型 `f(const char* p, long n)`）；opaque → ptr；返回 Never → void + 调用后 `unreachable`；无返回/Unit → void。
- opaque 值布局：单指针槽（i64 装载，String 同形先例）——native 侧铸造，**不入 GC root 位图**（无 We 冻结头；遗忘 = 指针槽死，指向的 native 对象生命周期是原生侧契约——章文 unchecked remainder 披露面）。
- 词汇面：foreign 调用是普通调用表达式——检查面全过后落任意已批表达式/语句位，零新边界词（bndMainBody v2 词汇不变）；native/ 工件与 IR 同落 build/（检视面）。

## D5 链接塔：native/ 与 E1906

- 发现：`buildProject` 在 IR 发射后 `os.Stat(dir + "/native")`；存在 → 收 `.c` 文件字典序（无 .c = 空集，非错）。
- 编译：逐 `clang -c native/<f>.c -o build/native-<f>.o`（钉版 clang，复用 runClang）。
- **查证**：程序 foreign 名全集（AST 面随 EmitProgram 收集——模块键 + 声明位）与 `llvm-nm --defined-only` 列出的全部 native .o 符号求差；缺者逐条 E1906：`unresolved native symbol — {name} (declared at {file}:{line})`，human 面 stderr / --json 面事件，全部渲染后 exit 1、**无 build/ 工件**（E1906 查证时点 = .o 编译后、最终链接前——链接永不带 unresolved 跑）。
- 链接：native .o 追加进最终 clang driver 调用实参列。
- toolchainGate：build/run 面扩查 `llvm-nm --version`（钉版同 pin；输出形实现期定形，gate 报文沿 clang 先例）。
- check/test/vet/doc 面：零链接面（ch21 直证黄金——同源项目 check 绿 + build E1906）。
- run 面走 buildProject 共用；单文件面维持既有 exit 70 边界（foreign 块单文件 = 走 check 单文件面照查）。

## D6 黄金矩阵（T1 落盘定数，预计 32）

| 桶 | 枚数 | 内容 |
| --- | --- | --- |
| check ffi | 14 | e1701 / e1702 / e1703 / e1704 / e1705-composite（List 参数）/ e1705-fntype（回调参数）/ e1705-never-param / e1706-string / e1706-bytes / e1707-construct / e1707-update / e0404-dup（跨块重名）/ green（全类别声明 + 纯调用净）/ cross-module（pub opaque 跨界） |
| 资源 | 4 | byres-green（open/read/close + impl Releasable + scope resource 全周期）/ e1101-missing / e1106-composite（List<File>）/ transfer（实参通道转移后禁用） |
| 效果 | 3 | e1401-call（纯 fn 调 io foreign）/ custom-tag（gpio 声明者绿/他者 E1401）/ bare-pure（裸段零账净） |
| never | 1 | exit Never 调用发散 check 绿 |
| build | 2 | e1906-missing（缺符号 exit 1 无 build/）/ e1906-partial（两声明恰报缺者） |
| run | 4 | scalar（native add 端到端 stdout）/ string（ptr+len C 侧打印）/ bytes（缓冲写回可见）/ opaque-handle（Socket 往返 + byres close 全周期 stdout） |
| fmt | 2 | foreign 块脏→净 / 净保持 |
| doc | 1 | foreign 条目不入页 |
| vet | 1 | foreign 调用面零告警净 |

run 桶 setup.files 携 native/*.c（runner files map 全树精确比对，M9b setup.dirs 先例）；fmt 桶钉 ForeignBlock 行保分类；doc 桶钉 pubDocItems 六类外排除（披露面）。既有 641 枚零改写（T1 负向对账：bndFFI 行删除后无既有黄金钉 foreign 面）。

## D7 验证阶梯（T8）

M11 D8 九步沿用：① `go build ./...` ② `go vet ./...` ③ `go test ./...` ④ runtime C harness（`go test ./runtime/ -count=1`）⑤ conformance 全量 fresh（641 + 新枚）⑥ `validate.py --all --strict` ⑦ `docs_sync.py` ⑧ `gofmt -l` 零 ⑨ git status 对账（refr/ 零触碰、既有黄金零改写、总数对齐 D6）。

## D8 归档与 roadmap

零规范提升零新 ADR（机制载体 = 本变更记录）；roadmap 双语 M12 行 pending → done；validate 复验 + docs_sync 对数 31 不变。

## D9 披露面（无黄金钉的机械形，tasks 记录在案）

1. opaque 单指针槽不入 GC root 位图（native 侧生命周期契约）。
2. String/Bytes 双参展开的 C 原型约定（native/ 作者侧须知——黄金夹具即文档）。
3. 跨模块同名 foreign 声明签名一致性不查（C 侧同名不同签名 = 原生侧契约面；E0404 只管模块内）。
4. fmt 对 ForeignBlock 行保分类（块内条目行保、块体缩进 2 空格）。
5. doc 排除 foreign 条目（pub-only 六类外——页脸不载 native 边界）。
6. mock foreign 目标 → E1804 类目面 +1（ch20 面收窄披露）。
