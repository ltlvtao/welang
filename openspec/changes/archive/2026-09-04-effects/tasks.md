# Tasks: effects

- [x] 1. 新增第 16 章 spec 文件与孪生
  来源：specs/effects/spec.md、specs/effects/examples.md
  验证：ADDED 7 Requirements / 31 Scenarios 逐字升格；examples H3 子节逐字升格；zh 孪生场景计数 7/31 镜像、代码块字节一致、全角冒号扫描 0
- [x] 2. 注册表扩展
  来源：specs/effects/spec.md R7
  验证：diagnostics.toml 增段位 E1400–E1499 owner 1600-effects、unclaimed 收窄 E1500–E9999、E1401–E1405 五条目（severity/title/description/remediation/owner/requirement/allocated），条目数 100→105、段 16；validate --all --strict 注册表检查通过
- [x] 3. 宿主修订 ×2 语言
  来源：specs/{lexical,control-flow,declarations,types,interfaces,fn-types,errors,modules}/spec.md
  验证：9 处 requirement 体逐字替换零外溢（ch1 +effect 词与场景、ch3 指针句、ch6 效果半句+解析场景、ch7 枚举句+场景 6 重写、ch10 方法签名段位+解析场景与 impl 匹配 E1404、ch12 段位句改批准文+1 场景、ch14 括注改写、ch15 main 段不参与 E1305 形状+1 场景）；宿主场景计数机器比对（ch1 7、ch3 4、ch6 5、ch7 6、ch10 5/7、ch12 6、ch14 2、ch15 8）；zh 孪生同步
- [x] 4. D14 示例刷新 ×2 语言
  来源：design.md D8
  验证：ch6 Pending 块效果行移除、ch10 导语与 Pending 块、ch12 拒绝例转合法例；docs_sync 22→23 对
- [x] 5. 验证套件
  来源：development-process.md 验证阶梯
  验证：validate --all --strict 通过；负例注入（未分配码注入已落地章 → FAIL 后还原）；一码一消息扫描（E14xx 新码报文 = 注册表标题）；docs_sync --check 通过

## 完成记录

2026-09-04 实现完毕。任务 1：EN 章文件逐字升格（7R/31S，机器断言 landed==delta）、examples H3 五子节逐字、zh 孪生 7/31 镜像、代码块 5/5 字节一致、全角冒号 0。任务 2：段位 E1400–E1499 + E1401–E1405（105 条、16 段），owner/requirement 字段解析到章文件（机器断言）。任务 3：9 处宿主拼接零外溢（git diff 逐行复核），场景计数机器比对 9/9 OK，zh 镜像定点编辑保留既有译文。任务 4：D14 四处 ×2 语言（ch6 Pending、ch10 导语+Pending、ch12 拒绝例转合法例+导语章界 1–12→1–12 与 16），docs_sync 22→23。任务 5：validate --all --strict 通过；负例注入 E1499 于已落地第 16 章 FAIL 后还原（先注入后还原的顺序执行）；一码一消息扫描 E14xx 全部命中注册表标题（反引号码形）；EN/ZH 计数全对齐。
