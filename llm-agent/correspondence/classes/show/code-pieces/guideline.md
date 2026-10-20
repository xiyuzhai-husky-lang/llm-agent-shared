# Show / code-pieces

针对「给我看相关代码」「show me code locs」一类请求，把真实源码片段按阅读顺序整理成可在手机阅读的单文件 HTML。此 correspondence 承接旧 `skills/general/code-pieces/SKILL.md`（提交 `427cfadc5dd`）的用途，复用 [code-pieces gadget](../../../../gadgets/code-pieces/)，不创建 skill。

## 示例与内容

示例位于 `examples/show/code-pieces/<descriptive-name>/`，包含 `pieces.json` 与构建出的 `index.html`。保留旧 schema：

```json
{"topic": "...", "summary": "optional", "pieces": [
  {"order": 1, "title": "...", "path": "<repo-relative>",
   "start_line": 10, "end_line": 42, "lang": "rust",
   "note": "一句话说明该片段在流程中做什么",
   "code": "<verbatim text>"}
]}
```

`start_line`、`end_line` 是源文件中从 1 开始、两端包含的真实行号；`code` 必须逐字摘录对应连续区间，不用省略号替换中间代码。每个片段有一行 caption（`note`），按数据流或适合读者理解的顺序排列，`order` 从 1 递增。用可选的 `repo` 字段记录源码仓库根目录，`path` 使用相对该根的真实路径；可选 `generated_at` 记录实际摘录时间。路径、行号和代码一律从指定工作树实际读取并核对，不用 HEAD 替代；涉及未提交变更时在 summary 中说明来源。源码展示不是运行或证明结果。

## Build → publish → send

1. 从真实源码摘录并核对 `pieces.json`。
2. 从仓库根目录运行 `bash llm-agent/correspondence/classes/show/code-pieces/ui/build.sh <example-dir>`。入口调用 `python3 llm-agent/gadgets/code-pieces/build.py <example-dir>/pieces.json -o <example-dir>`，仅依赖 Python 标准库，生成示例内的 `index.html`，失败立即退出。页面内嵌样式、脚本和源码，不需要 Kathy converter 或 Git oid。
3. 需要公开链接时运行 `bash llm-agent/gadgets/code-pieces/publish.sh <example-dir> [port]`，最后一行是 `URL: https://….trycloudflare.com/`。公开页面不得含秘密或私人数据。发送前用真实 390 CSS px 的设备模拟检查手机排版，确认无横向溢出；普通 headless Chrome 的窗口宽度可能被限制为 500 px，不能代替设备模拟。
4. 发送 URL，并用一小段文字按片段顺序说明内容。只要求本地构建时交付示例及 HTML 路径即可，不启动公开隧道。

发布脚本在示例目录记录 `.serve.pid`、`.tunnel.pid`；两个进程存活期间链接有效。重新构建到同一目录会原地更新页面，URL 不变。运行 `bash llm-agent/gadgets/code-pieces/publish.sh --stop <example-dir>` 停止服务与隧道。
