---
name: welang-pre-push-checks
description: Select and run the smallest credible welang validation set before commit, push, marking a change ready/complete, or claiming work is done — based on the actual diff and affected layers. Do not rerun unrelated full suites merely because a push follows. Use before any commit/push.
---

# welang 提交前验证

## 确定范围

先确认：当前分支、目标基线、staged/unstaged/untracked 文件及其依赖关系。基线合并或重定目标后重新评估。

## 按实际 diff 选取证据

- **仅文档 / 流程 / openspec 工件**：`python3 openspec/tools/validate.py --all --strict`、`git diff --check`、Markdown 链接抽查、staged 无 `refr/`。
- **语言规范（docs/spec/）**：上述全部 + 规范一致性复核（诊断码全局唯一、§ 交叉引用有效、术语一致）。
- **编译器 / 工具链代码**：`go build ./...`、`go test ./...`、受影响的 conformance 黄金用例；触及诊断输出时加协议快照测试。
- **变更状态推进（ready/complete 声明）**：对应关卡技能（spec-impact-audit / code-review）已执行且结论记录在案。

只追加与 diff 实际触及的共享边界相称的更大范围，不为"保险"跑全量。禁止用以下方式掩盖缺失验证：pass-with-no-tests、降低阈值、重写快照迁就漂移、绕过出厂路径的 mock。

## 失败与推送

- 任何相关失败必须先修复或在报告中明确列出，再进行普通推送。
- 环境相关结论必须附精确命令与平台证据。
- 不得在无用户明确授权时绕过钩子（包括 `git commit --no-verify`）。

## 推送后

fetch 远端 ref 并核对一致。报告：运行了哪些命令、结果如何、跳过了哪些检查及为何不适用。
