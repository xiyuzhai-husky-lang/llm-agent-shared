# Diff · advanced

## 用途

`diff/advanced` 在 `diff/basic` 的基础上加了 MUST SIGN：用户在页面里逐 hunk 签署关键改动。它用来把一段需要用户逐处确认的改动交给用户审阅。

## 示例结构

example 位于 `llm-agent/correspondence/examples/diff/advanced/<descriptive-name>/`，目录中有：

- `diff.json`：`title`、`repo`（目标 worktree 的绝对路径）、`merge_base`，以及 `candidate`。`candidate` 是 `{"kind": "commit", "commit": "<ref or oid>"}` 或 `{"kind": "working-tree"}`。
- `must-sign-additions.json`：`{"additions": [{"path": ..., "lines": [{"start": ..., "end": ...}], "reason": ...}]}`。
- `signatures/`：用户签署的记录，由 viewer 写入，随 example 一起提交。

## MUST SIGN

- `must-sign-additions.json` 必须创建，且 `additions` 不得为空：每个 advanced diff 都至少列出一处要用户签署的改动。
- 每一项的 `path` 是 range 中一个改动过的文件，`lines` 是这处改动在 candidate 一侧的行范围（从 1 起、两端都含，至少一个），`reason` 说明这处为什么是关键改动、用户签它是在确认什么。
- 只有与 `lines` 相交的 hunk 要签；同一文件里跟着机械变化的 hunk 不在范围内。被删除的文件没有 candidate 一侧的行，不能列入。
- 只列关键改动：新类型、新语义、核心定义的变化；跟着关键改动机械变化的地方不列，但在 `reason` 里说明它们只是跟着改。
- 列不出任何需要签署的改动，说明这段改动不需要 advanced diff，改用 `diff/basic`。

## 启动与发送

viewer 是 Go 服务 `go-src/llm-agent/correspondence/diff-advanced`，web 端在 `webui/apps/diff-advanced/`；在仓库根目录运行 `bash webui/apps/diff-advanced/build.sh <example 的 diff.json 的绝对路径> <port>` 构建并启动。orchestrator 用 example 的 `diff.json` 启动 viewer、打开 quick tunnel，在浏览器里确认页面正常、MUST SIGN 一栏列出了所列的项，再通过 Telegram 发送 URL。range 的写法同 `diff/basic`：`<branch>: <base short oid>..<candidate short oid>`，附一句说明。
