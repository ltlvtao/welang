# welang 参考实现路线图

本文档是参考工具链执行状态的唯一权威：策略、里程碑次序与实现期发现的待办。它不承载语言行为（在 `docs/spec/`）、不承载长期取舍（在 `docs/decisions/`，宿主与目标策略见 ADR-0002）、不承载流程规则（在 `docs/process/development-process.md`）。

## 策略

- **薄垂直优先**（用户裁决，2026-09-04）：尽早打通最小端到端 `we new` → `we check` → `we build` → `we run`——最小前端子集、LLVM IR 发射、最小 C 运行时——再按章加宽每个管线阶段。这以执行次序兑现 ADR-0002 的「从第一天起产 LLVM IR」与第 0 章完成度闭环，并换来最早的诚实端到端信号。
- **规范先行**：每个切片实现已批准章节；无规范依据的编译器行为不存在（研发流程不变量）。
- **一里程碑一变更**：每个里程碑是一个小而可垂直验证的 openspec 变更；其状态行由归档它的变更翻写。
- **自举闭环**（用户裁决，2026-09-08）：bootstrap 轨（B1–B3）跟随参考里程碑之后，也可与未落的 M 行交错——B1 是关键路径，由 M10b 的多函数发射拓宽生长而来；B2 是 B1 的调试基建（fs 与 process 面先行）；LLVM 访问沿用 .ll 文本 + 子进程 clang 的既有架构，路径上没有 LLVM C API 绑定。决策载体为 ADR-0002。

## 里程碑

| ID | 变更 | 实现内容 | 状态 |
| --- | --- | --- | --- |
| M0 | `compiler-bootstrap` | 第 21 章命令面子集：封闭子命令表、全局选项、`we version` / `we new`（E1904）、E1907、JSON Lines 诊断事件、一致性测试地基 | done 2026-09-05 |
| M1 | `lexical` | 第 1 章 token 模型、字面量、注释、关键字；`we check <file>` 跑词法阶段（E0001–E0009） | done 2026-09-05 |
| M2 | `parser-core` | 第 2 章文法骨架与第 6 章声明；`we check` 跑词法+解析（E0101、E0102、声明诊断） | done 2026-09-05 |
| M3 | `types-and-main` | 第 7 章类型子集、第 15 章根模块与 main 形状、第 14 章 `Result`；`we check .` 对 `we new` 骨架跑绿 | done 2026-09-05 |
| M4 | `native-vertical` | 最小 LLVM IR 发射、最小 C 运行时（启动、分配器）、骨架程序端到端 `we build` 与 `we run` | done 2026-09-05 |
| M5 | `control-and-composites` | 第 3 章控制流、第 4 章 match、第 8 章复合类型与所有权、第 9 章和类型、第 12 章函数类型与闭包 | done 2026-09-05 |
| M6a | `generics-iterables-collections` | 第 10 章接口与泛型、第 11 章可迭代、第 5 章 for 语句、第 17 章集合 | done 2026-09-06 |
| M6b | `modules-errors` | 第 13 章 resource、第 14 章全量、第 15 章模块解析全量 | done 2026-09-06 |
| M7 | `effects` | 第 16 章效果声明与效果检查 | done 2026-09-06 |
| M8 | `stdlib-and-gc` | `std.io` 核心；运行时精确 GC 设计落地 | done 2026-09-07 |
| M9a | `concurrency-check` | 第 18 章检查塔：std.concurrent 装载、内建类型面、Shareable、task 块与捕获纪律、scope/句柄纪律、select 定型 | done 2026-09-07 |
| M9b | `concurrency-run` | 第 18 章运行塔：单线程协作调度器、共享状态与 channel 原语运行时、真实时钟 timeout（虚拟钟随 M10 落地） | done 2026-09-07 |
| M10a | `testing-check` | 第 20 章检查塔：test 模块与 test 块、mock 声明、advanceTime 定型与位置、std.test 装载 | done 2026-09-08 |
| M10b | `testing-run` | 第 20/21 章运行塔：codegen 多函数发射拓宽、we test runner、虚拟钟、确定性测试调度、test 边界、mock 拦截 | done 2026-09-08 |
| M10c | `testing-explore` | 第 21 章探索：交错探针、E1901/E1902 守卫、偏序归约 | done 2026-09-09 |
| M11 | `fmt-vet-doc` | 第 21 章格式化器（行保结构重排、项目面 = 项目下全部 .we 排除 build/）、advisory 层（W1910–W1912、[vet] 姿态全管线）、we vet、we doc（pub-only 页面、--check） | done 2026-09-09 |
| M12 | `ffi` | 第 19 章 foreign 块端到端：parser 产生式（E1701–E1704）、跨界集检查器（E1705–E1707；byres/opaque/效果走既有机器）、codegen declare + 位置 C ABI（String 为 (ptr, len) 双标量展开、Never 为 void + unreachable）、native/ 构建时链接 + llvm-nm 符号查证（E1906、工具链 gate 扩查 llvm-nm） | done 2026-09-09 |
| M13 | `dependencies` | 第 22 章 MVS 解析、`we.lock`、依赖缓存、本地 registry 夹具 | done |
| M14 | `lsp` | LSP 文档与 `we lsp` | done |
| M15 | `benchmarks` | 评测套件（First-Pass Compile Rate 等）：方法论文档、18 任务种子集（参考解自证）、in-process 批次跑器（五桶判定机）、校准与黑盒测试面 | done 2026-09-09 |
| B1 | `codegen-full` | 全量代码生成：所有已批准的表达式与声明形式皆可发射——任意位置的函数（M10b 起的拓宽）、泛型单态化、闭包捕获、Dyn 派发、record 运行时布局、非字面量 String、集合内建面；codegen 未实现集合收缩至零 | pending |
| B2 | `stdlib-real` | 编译器所需的标准库实库面：文件与目录 I/O、子进程 spawn（钉版 clang）、字符串构建、集合面背后的真数据结构 | pending |
| B3 | `bootstrap` | 自举：编译器从 Go 移植到 We，以三段式闭环验收——Go 宿主构建编译 We 编译器，其产物再编译编译器一次，两段产物逐字节一致；conformance 套件在 We 宿主编译器下全绿 | pending |

## 执行状态规则

- 里程碑的状态只在归档它的变更中翻写；执行状态只住在这里。
- M4 之后的次序可能随加宽暴露的依赖而调整；任何调整都由受影响里程碑的变更携带、改写本文件。
- 未实现边界：本参考构建尚未运行的子命令报一行 stderr 并以 70 退出（实现的过渡态、非规范表面）；集合随里程碑落地收缩至零。

## 已登记待办

实现期发现、规范未固定的事项；每项只能经自己的变更离开本清单。

1. **项目名精确规则。** 第 21 章 R1/R8 与第 22 章 R1 引用「第 1 章命名约定」指称 TOML 层名字，但第 1 章 Naming conventions 只固定标识符按绑定形态的大小写规则；具体字符集只住在第 22 章正文与 E1904 的 remediation（"lowercase letters, digits, and hyphens"）。未来的词法或工具链修订应在第 1 章固定精确规则（前导/尾随连字符、长度）。在此之前实现取：非空、字符集恰 `[a-z0-9-]`、不发明任何额外约束。
2. **注册表嵌入。** 编译器暂在调用点硬编码命令层注册表标题与补救文本；`diagnostics.toml` 的嵌入（`go:embed`）随第一个真实管线阶段落地——届时必须回答安装态二进制读哪个注册表的问题。
3. **规范版本标签。** `SpecVersion` "0.9.0"（`internal/version`）标记 v0.8 私草之后第一个正典基线；若裁决采用别的标记法，改动是一个常量加本行。
4. **运算符类型域表。** 已批准文本点名第 2 章接受的运算符的数值与 Bool 操作数，但没有任何 Requirement 固定逐运算符的完整域表（每个运算符在每个操作数位接受哪些基类型）。修订批准前，超出已批准数值/Bool/比较域的同型操作数对停在诚实边界（`arithmetic and comparisons beyond the ratified numeric and Bool domains (spec gap; roadmap follow-up)`）；两侧不一致仍是 E0501。M3（`types-and-main` design D4）发现。
5. **调用实参个数与非函数被调的诊断码。** 没有已批准 Requirement 固定「调用实参个数与被调声明不符」或「被调者不是函数」时的诊断。修订认领前，两者均停在诚实边界（`calls with an argument count the callee does not declare (spec gap; roadmap follow-up)`、`calls on values that are not functions (spec gap; roadmap follow-up)`）。M3（`types-and-main` design D10）发现。
6. **单文件构建工件命名。** 第 21 章 R2 说构建工件由 manifest 命名，但单文件编译（R1 固定的 `[path]` 文件形）没有 manifest 可取名。工具链修订固定单文件工件的名字与位置前，`we build <file>.we` / `we run <file>.we` 先跑完整管线（诊断照常报告），干净后停在诚实边界（`single-file builds (spec gap; roadmap follow-up)`）。M4（`native-vertical` 裁决 Q3）发现。
7. **赋值目标可变性。** 没有已批准 Requirement 固定 `let` 绑定名被重赋值（`let x = 1; x = 2`）时的规则与诊断；第 8 章所有权文本固定的是捕获类别，不是非 `var` 绑定的重赋值。修订认领前，实现维持 M3 行为（赋值对照绑定的类型检查通过）。M5（`control-and-composites` design D14）发现。
8. **注册表 E0604 陈旧子句。** E0604 的注册表描述含「a member access names an undeclared field / Fires under: Field access」，与第 8 章 R5 的路由（记录上未知名走 E0816，「一个规则一个码」）冲突——章文是触发语义权威，M5 在该处发 E0816。陈旧子句的清除归属下一个触 `diagnostics.toml` 的变更（或专门维护变更）。M5（`control-and-composites` design D10）发现。
9. **`we run` 自项目目录外的路径解析。** `we run <path>` 把子进程工作目录设为项目路径、却按相对路径解析工件，从项目目录外调用时报 `fork/exec … no such file or directory`；从目录内调用（`we run .`，conformance runner 同款）正常。工具链修复应在切换子进程 cwd 前按调用目录解析工件绝对路径。M5（`control-and-composites` T7 观察；M4 遗留，`internal/cli` 未被 M5 触碰）发现。
10. **newtype 与 Shareable 集。** 第 18 章闭集点名基类型、unit、全字段 Shareable 的 value 记录、全载荷 Shareable 的 value sum、tuple、十一类同步 gc 型——未点名「Shareable 底类型之上的 byval newtype」，第 8 章的品类派生规则也不延伸到 marker。修订批准入集前，实现维持字面读：newtype 不是 Shareable，无论其品类。M9a（`concurrency-check` design D3、T5 披露）发现。
11. **运行时结构无回收。** 第 18 章未定运行时自有结构——调度器任务与栈、scope 帧、等待链节点（含留待源操作懒回收的 select 落选者）、panic 报文、运行时生成位图——的析构或 finalizer 语义，M9b 运行塔以 malloc 族分配且不回收：泄漏有界于任务数与登记数，存活于进程生命周期。未来运行时变更（回收通路或 finalizer 机制——并过 ADR-0003 并行门）应认领。M9b（`concurrency-run` design D7、D13 披露 2）发现。
12. **test 模块可导入性与模块路径字符集。** 第 21 章场景称 `src/helpers_test.we`「随项目编译、可导入」——但第 15 章的导入→文件映射以 `[a-z][a-z0-9]` 形段拼写模块路径（实现为 E0013），下划线词干不可拼写；项目编译自 src/main.we 走导入图，图外的 test 模块在 `we check .` 下永不编译。或字符集收下划线词干、或场景的可导入性需要自己的映射——规范修订应定夺。M10a（`testing-check` design D10、Q1 切分的装载边界披露）发现。
13. **test 关键字与 std.test 模块段。** 第 1 章保留字闭集使 `test` 成为关键字（它领 test 块产生式），该段无法按标识符取词；第 15 章的限定触及恰为「经导入名的 `name.item` 形」，而第 20 章示例调 `std.test.assertEqual(...)`——第 15 章未定型的两级点形。M10a 使已批面可拼写（模块路径段收过字符集检查的关键字 token）、调用走别名形（`import std.test as st; st.assertEqual(...)`）；裸导入绑定关键字名且惰性。规范修订应定夺真实调用面：两级 std 限定符、仅别名示例、或非关键字模块名。M10a（`testing-check` design D8、实现准备期披露）发现。
14. **并发构造面为 mock 目标。** 第 20 章「不可 mock 目标」Requirement 以 `E1804:` 拒绝泛型 fn、impl 方法、「构造器如 `User`」——未点名并发构造器。M10a 的字面读在检查面接受 `mock conc.channel`（名字解析为模块级 fn 条目、无特判触发，`m10a_test.go:178`），但运行面是原始构造面：channel 构造的期望类型派生自调用实参、非声明签名可复述，唯一可复述的签名（无参、unit 返回）在运行时无从拦截。M10b 维持检查面零改、channel 不入槽表——检查过的 `mock conc.channel` 运行时跑真构造器。规范修订应裁决重分类：或 E1804 集点名原始构造器（检查面拒绝运行面无法兑现的目标）、或章文为其固定拦截面。M10a（`testing-check` 审查 F1）发现，M10b（`testing-run` design D3）登记。
15. **第 19 章第二个效果段示例拼写。** 第 19 章 foreign 声明 Requirement 给出的签名形为 `fn name(params) effect tag1 tag2 ...` 或 `fn name(params) -> type effect tag1 tag2 ...`；第二个字面示例把段画在返回类型之后，与同句自己的散文（「between the parameter list and the arrow or the declaration's end」）及两处权威——第 6 章（「between the parameter list and the arrow or body」）、第 16 章（「between the parameter list and the arrow (or the body, when no return type is written)」）——矛盾。示例级笔误、非语义冲突：M12 实现及其黄金矩阵一律按第 6/16 章拼写 foreign 声明（`fn name(params) effect tags -> type`）。规范变更应修正第 19 章示例。M12（`ffi` T1 黄金源探针、披露 1）发现。
16. **codegen 支配性缺陷：scope 块之前的带赋值 match。** 同一函数内，臂内赋值的 match 语句先于 `scope`（含 `scope timeout`）块执行时，发出的 IR 违反支配性——clang 以 `Instruction does not dominate all uses` 拒绝。参考构建的绕法（把首个 match 延迟到 scope 之后）塑造了 M15 errpath-02 参考解并在其 traps.md 披露。修复归属 codegen 线（B1 拓宽或专门变更）。M15（`benchmarks`，实现期披露二）发现。
17. **评测套件：真实批次面与 L2/L3 扩充。** M15 落了方法论、18 任务 L1 种子集、经合成批次自证的 in-process 跑器。尚欠：真实 LLM 批次生产（生产面，与判定面设计分离）、不终止 attempt 的墙钟守卫（in-process 跑器遇无等待源忙循环会挂起——死锁检测唯一看不见的形）、L2（模块级）/ L3（系统级）任务扩充。M15（`benchmarks`，非目标与跑器披露）发现。
18. **第 7 章未固定 `/` 与 `%` 的舍入与符号约定。** 该章的算术语义 Requirement 只覆盖 checked 的 `+ - *` 族；对余下两个整数算子，无论商的舍入方向还是负操作数下的余数符号，皆未固定。参考实现取截断读法（LLVM 的 `sdiv`/`srem`：商向零截断，余数随被除数符号）——即 Rust 与 C 固定的约定——并在描述发射处（codegen-mono 变更，design D2）披露。类型章修订应批准该约定或另定。B1a codegen-mono（T3，2026-09-10 裁定）发现。
19. **除零在全规范无任何条款。** 第 7 章的检查算术 Requirement 只点名溢出，且常量折叠器对未定义求值静默让路而非报告（`foldApply` 的 `/` 与 `%` 两分支返回 not-ok），故字面量 `10 / 0` check 干净、直达运行期。参考实现在那里设陷阱——经第 14 章机制的 `panic`，消息 `division by zero`，这也是此类程序唯一的捕获面——正是第 7 章「checked；绝不静默回绕」立场蕴含而未经文字确立的读法。修订应固定：零除是运行期陷阱、常量可折叠时为编译期错，还是第三种。B1a codegen-mono（T3，2026-09-10 裁定）发现。
20. **越界与溢出的移位语义。** 第 7 章未固定移位量达到或超过操作数位宽（或为负）时的行为，而 LLVM 的移位指令在那里是 poison，发射器必须自选。参考实现取陷阱（`Int64 shift overflow`），且 `<<` 在有一位将移出顶端时也陷阱——该章「绝不静默回绕」规则蕴含的 checked 读法，因为 LLVM 的 `shl` 会一声不吭丢掉的正是那些位。修订应固定越界规则，以及左移丢失的位是否算溢出。B1a codegen-mono（T3，2026-09-10 裁定）发现。
