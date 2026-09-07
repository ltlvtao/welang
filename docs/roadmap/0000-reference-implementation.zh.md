# welang 参考实现路线图

本文档是参考工具链执行状态的唯一权威：策略、里程碑次序与实现期发现的待办。它不承载语言行为（在 `docs/spec/`）、不承载长期取舍（在 `docs/decisions/`，宿主与目标策略见 ADR-0002）、不承载流程规则（在 `docs/process/development-process.md`）。

## 策略

- **薄垂直优先**（用户裁决，2026-09-04）：尽早打通最小端到端 `we new` → `we check` → `we build` → `we run`——最小前端子集、LLVM IR 发射、最小 C 运行时——再按章加宽每个管线阶段。这以执行次序兑现 ADR-0002 的「从第一天起产 LLVM IR」与第 0 章完成度闭环，并换来最早的诚实端到端信号。
- **规范先行**：每个切片实现已批准章节；无规范依据的编译器行为不存在（研发流程不变量）。
- **一里程碑一变更**：每个里程碑是一个小而可垂直验证的 openspec 变更；其状态行由归档它的变更翻写。

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
| M9b | `concurrency-run` | 第 18 章运行塔：单线程协作调度器、共享状态与 channel 原语运行时、真实时钟 timeout（虚拟钟随 M10 落地） | pending |
| M10 | `testing` | 第 20 章测试运行、mock、探索 | pending |
| M11 | `fmt-vet-doc` | 第 21 章格式化器、vet、文档生成 | pending |
| M12 | `ffi` | 第 19 章 foreign 块与构建时链接（E1906） | pending |
| M13 | `dependencies` | 第 22 章 MVS 解析、`we.lock`、依赖缓存、本地 registry 夹具 | pending |
| M14 | `lsp` | LSP 文档与 `we lsp` | pending |
| M15 | `benchmarks` | 评测套件（First-Pass Compile Rate 等） | pending |

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
