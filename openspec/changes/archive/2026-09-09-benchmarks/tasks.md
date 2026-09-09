# tasks — benchmarks（M15）

## T0 实现期 D13 修复：parser 控制流头悬挂缺陷（已落地）

> 实现期插入的任务集探针揭出（非计划任务，D13 修复披露纪律）；T1–T9 编号保持稳定——proposal 审查记录的任务引用不变。

- [x] `internal/parser/parser.go` 两处修复：①后缀 `{` 悬挂条件 `noBrace > 0 && depth == 0` 无条件化（头的事实非区域的事实——depth>0 区域内控制流头的体花括号被误读为构造头 E0105）+ tryTypeArgs 镜像位同步；②parseScopeRes 绑定列表 depth 于 `)` 显式闭合（原 defer 泄入体内块）+ 绑定值 `headExpr()` → `parseExpr(valueCtx)`（值位事实）
  来源：M15 实现期探针揭出（design 无此内容——变更内发现；proposal 影响层 compiler 条已同步披露）
  验证：`go test ./internal/parser/` 全绿；gofmt 干净

- [x] `internal/parser/parser_test.go` TestControlFlowInDepthRegions：7 损伤形 wantClean（impl 方法体 if / 接口默认方法体 if / if 作调用实参 / if 在括号组 / scope resource 体 if / impl 方法 match scrutinee 等）+ 2 构造仍开守卫（scope 绑定值构造 / 调用实参构造）——先红（7 形 E0105）后绿
  来源：同上（修复的行为钉）
  验证：先红（7 形 E0105）后绿证据在案；`go test ./internal/parser/` 全绿

- [x] conformance 六枚新黄金钉损伤形：check-if-impl-green / check-if-default-green / check-if-arg-green / check-if-paren-green / check-if-scope-res-green / check-scope-res-construct-green；conformance 695→701，既有黄金零触碰零回归
  来源：同上（损伤形的黄金回归钉）
  验证：`go test ./internal/conformance/`（701）全绿；既有黄金零触碰

## T1 方法论文档落盘

- [x] docs/benchmarks.md + docs/benchmarks.zh.md（三指标严格定义 / 任务分类体系 / 陷阱清单格式 / 批次 schema 与反馈协议 / 跑器判定序 / 统计纪律）+ docs/README 双语责任图补行
  来源：design D1/D2/D8
  验证：docs_sync 33 对全绿；双语结构检查（标题级数/围栏计数）

## T2 批次 schema 测试先行

- [x] internal/benchmarks 批次 schema（LoadBatch 解析 + 校验：model/task/round 连续性/files 路径域）+ 畸形负例测试（未知任务 / 跳号 / 空 files / 越界路径触 tests/ 与 we.toml）——先红后绿
  来源：design D4
  验证：go test ./internal/benchmarks/ schema 套件绿；红证据捕获于完成记录

## T3 任务集种子 18 落盘

- [x] internal/benchmarks/tasks/ 18 任务（7 根因轴 × L1 × 2 + 反向对照 4：errpath/race/resleak/nullbnd/implconv/dangling/ctxpass × 2 + control × 4），每任务 prompt.md（英文）+ traps.md（英文，含检测路）+ reference/ 完整项目（we.toml + src/main.we + tests/）+ registry.go go:embed 注册表（Tasks/Has）
  来源：design D3
  验证：任务集自证测试——18 参考解 check + test in-process 全绿（TestReferenceSolutionsGreen，先红 `undefined: Tasks` 后绿）；TestTaskSetShape 钉死 18 id 集与四件完整性；「任务不用未实现形」约束经运行塔探针测绘后钉死（探针结论与两处实现期披露见 proposal 审计记录第 4 点）

## T4 跑器判定机与指标计算测试先行

- [x] 判定机（组装临时项目 → cli.Run 注入流 check --json → test --json → 五桶分类）+ 指标计算（FPCR 首轮 / FLC 分布与截尾 / LBR attempt 级 + 任务级衍生视图）+ Report JSON 渲染——五形合成批次断言先红后绿
  来源：design D2/D4/D5
  验证：五形测试绿（FPCR/收敛轮数/LBR/边界排除逐值断言）；先红证据在案（`undefined: Evaluate`/`undefined: Bucket*`）；attempt 叠加层 src/ 防御守卫（越界面 panic）

## T5 陷阱校准

- [x] 每任务按 traps.md 注入典型错误负例 → check 或 test 至少一路抓住；未抓到的陷阱按 D13 披露并修任务/清单
  来源：design D3/D6
  验证：TestTrapCalibration 64 枚负例全绿（每任务 ≥1，落桶逐枚断言 rejected/latent/test-malformed/boundary）；全部 mutation 先经真二进制子进程探针验证后定形；traps.md 点名诊断码全部实证（E0501/E1105/E1202/E1602/E1603/E1607/E1618/E0204/E0305/E0605/E1202）。D13 披露两枚初版落 clean 的负例并修面：nullbnd-02 conflate 打在不产生 Closed 的第一轮（行为死面）→ 改第三轮锚定；control-02 swap 单独是交换律正确 → 校准面改 swap+reset 复合（陷阱文本本就如此主张）；不可校准面（busy-loop 挂死/设计注记/同路径变体/codegen 缺陷注记）以注释与 traps.md 措辞披露

## T6 黑盒电池抽查

- [x] 真二进制子进程复刻关键桶判定（clean/latent/rejected/boundary 至少各一形经真 `we check`/`we test` 对拍 in-process 结果）
  来源：design D6
  验证：TestBlackboxBucketParity 五桶形（clean/latent/rejected/boundary/test-malformed）全绿——go build cmd/we 真二进制子进程退出码推导桶 == Evaluate in-process 桶，零漂移；另 T5 的 64 枚 mutation 全部先经真二进制探针定形再入 Go 断言（第二重对拍面）

## T7 验证阶梯

- [x] gofmt -l 干净 + go vet ./... + go test ./... 全绿 + openspec/tools/validate.py --all --strict + docs_sync.py（33 对）
  来源：仓库验证纪律
  验证：六面全绿输出在案（gofmt 零输出 / vet 零输出 / 13 包全 ok——benchmarks 27.9s 含 schema+任务集自证+五形+64 校准+黑盒；validate strict 修复 T0 三枚 checkbox 逐枚补 来源/验证 后 OK；docs_sync 33 对齐；conformance TestGoldenCases 701 PASS 零回归）

## T8 code-review

- [x] welang-code-review 7 条（读 .agents/skills/welang-code-review/SKILL.md 手动执行）；审查记录入 proposal.md
  来源：仓库节奏
  验证：7 条逐条通过（记录落 proposal.md「实现审查记录」节）；发现 F2（T0 checkbox 结构）已处置——validate strict 复验 OK

## T9 归档

- [x] roadmap 双语 M15 → done + change.yaml status archived + 目录日期前缀 2026-09-09-benchmarks + 记忆回写 + L2/L3 扩充 follow-up 登记
  来源：仓库节奏
  验证：validate --all --strict 复验 OK；git status 对账（新增工件清单）；基线提升完成——docs/benchmarks.md 双语补运行面锚定段与 in-process 挂起披露（dangling 轴锚定改 E1607/E1618 的正文化）、roadmap follow-up 16（codegen 支配性缺陷）+ 17（真实批次面与 L2/L3 扩充、墙钟守卫）登记
