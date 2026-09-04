# Tasks：collections

- [x] 1. 落地第 17 章规范文件（EN + zh 双语）
  来源：specs/collections/spec.md（6 ADDED Requirements）与 specs/collections/examples.md
  验证：docs/spec/1700-collections.md 与 delta 逐字一致（机器断言 landed==delta，Examples 节同）；1700-collections.zh.md requirement/scenario 计数与 EN 一致、代码块字节一致、全角冒号 `E\d{4}：` 扫描为 0
- [x] 2. 宿主修订九处 requirement 体 ×2 语言
  来源：specs/grammar/spec.md（2 MODIFIED）、specs/interfaces/spec.md（2 MODIFIED）、specs/iterables/spec.md（1 ADDED + 3 MODIFIED）、specs/fn-types/spec.md（1 MODIFIED）、specs/modules/spec.md（1 MODIFIED）
  验证：各 MODIFIED 与宿主 landed 文本逐字一致（拼接边界 `^(## |### )` 正则、标题邻接空行检查）；宿主场景计数机器比对（ch2 骨架 6→7、诊断段 3、ch10 接口声明 5→6、泛型参数 5、ch11 迭代器 4、for 协议 3、实现者义务 2、ch12 段位 2、ch15 预导入 3）
- [x] 3. 注册表扩展与描述维护
  来源：提案 What Changes 7
  验证：`python3 openspec/tools/validate.py collections --strict` 注册表干净；条目 105→106、段 16→17；E1501 条目字段齐全（severity/title/description/remediation/owner/requirement/allocated）；E0105/E0808/E0826 描述与 E0900/E1000/E0800 段内注释维护到位（tomllib 对照）
- [x] 4. 负例注入（测试先行）
  来源：研发流程验证阶梯（spec 层测试先行 = 断言先于条目定案）
  验证：向已落地宿主章注入未分配码 E1599 的使用 → validate FAIL → 还原后复绿；E1501 于章文件与注册表同批落地避免 owner-file 过渡 FAIL
- [x] 5. D14 示例刷新 ×2 语言（ch2/ch5/ch7/ch8/ch11/ch12）
  来源：提案 What Changes 8
  验证：全部刷新点 grep 复核无「pending the combinator change」残留；诊断注释消息完整于首物理行；一码一消息扫描（反引号码形 `EXXXX:` message 解析）PASS
- [x] 6. 全量验证与门禁
  来源：研发流程 §7
  验证：`python3 openspec/tools/validate.py --all --strict` 全绿；`python3 openspec/tools/docs_sync.py --check` 23→24 对；zh 孪生八项结构断言全绿

## 完成记录

2026-09-04 实现完毕。任务 1：EN 章文件组装（6R/21S，机器断言 landed==delta + Examples 逐字 + Terminology 13 行），zh 孪生 6/21 镜像、代码块 4/4 字节一致、全角冒号 0；段位条目标题按既有命名式定名「集合诊断段位」。任务 2：十处拼接（九 MODIFIED 替换 + ch11 ADDED「Iterator 组合子」插于「Iterator 接口」之后）×2 语言，landed==delta 机器断言 10/10，宿主场景计数 10/10，EN/zh 标题邻接空行检查零违例（ch17 组装时一处 Terminology 前缺空行即修）。任务 3：注册表 106 条/17 段（tomllib 计数），E1501 七字段齐全且 requirement 解析到章文件，E0105/E0808/E0826 描述与 E0800/E0900/E1000 段内注释维护到位。任务 4：E1599 注入第 11 章 → `--all --strict` FAIL（「diagnostic usage 'E1599:' has no registry entry」）→ 还原复绿；per-change 模式不复跑 usage 扫描的教训再次确认。任务 5：六处刷新点 ×2 语言（ch2 `list[0]` 拒绝例转 `.get(0)` 合法例 + 永久拒绝注、ch5 Pending 块转区间组合子实例试、ch7 待批块转已落地块、ch8 分层已落地/待并发、ch11 导语 + Pending 块转真实组合子示例、ch12 Pending 块改组合子已落地/方法子句已落地/显式形式与方法值仍待），残留 grep 为零；一码一消息扫描（含新章全量 + 本变更加入行）12 处报文全部精确等于注册表标题（E0501 用既定调用报文）。任务 6：validate --all --strict 全绿；docs_sync --check 24 对全对齐；zh 镜像八文件 R/S 计数 + ch17 代码块 + 冒号扫描 + 邻接全绿。
