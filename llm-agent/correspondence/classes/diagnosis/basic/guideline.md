# Diagnosis basic

页面只包含以下六节，顺序固定，不添加标题、摘要或其他章节。

1. **要点**。有结构，不写成一段紧凑的文字。固定为四块，每块用 `\textbf{…。}` 起头：
   - `一句话。` 一句话说清楚出了什么错。
   - `输入发生了什么。` 一个 `itemize`，按发生顺序列出这个输入上实际发生的事：哪条语句、打开了什么变量、代入或 normalize 之后表达式变成什么样、关键的顺序或缺失是什么。每个 item 一到两句。
   - `<阶段>里发生了什么。` 一个 `itemize`，列出从上一块到出错点的步骤：谁被 assume、谁被触发、哪条 rule、哪个前提、缺哪个条件、为什么证不出来。每个 item 一到两句。
   - `为什么 panic 而不是失败。` 一个 `itemize`，只在错误是 panic 或断言时出现，说明哪一步把本应 graceful 的失败变成了 panic。
   先说输入和现象，再说机制；机制名（函数、rule）只在它真正参与的那一条 item 里出现。converter 只支持 `itemize`，不支持 `enumerate`。
2. **panic**。用一个 `kyQuote` 原样引用 panic 行。随后放一个引用源 item 的 `kyCodespan`，只用一个 span 标记触发语句。
3. **essential call stack**。每条栈使用一个 `kyQuote`，内部使用 `verbatim`，按外层到内层排列，每行一个 frame。每条栈只配一句话，说明它是哪条栈。
4. **相关代码**。每个代码位置使用一个 `kyCodespan`。每个 label 至多两句。
5. **修法**。恰好三句，只提出一种修法。说明修改位置、具体做法和结果。
6. **验证**。说明运行过的 harness 和结果。修法尚未获批时必须明说。区分历史探针结果与本次运行，不把未运行的验证写成已通过。

正文用中文，保留 English terms。句子简短完整，每句表达一个要点。不用箭头或斜杠代替句子。

每个数字和 handle 都在首次出现的句子里解释。例如，`\verb|#17|` 是该 item 的 MIR 表达式 arena 中索引为 `\verb|17|` 的表达式，索引从零开始。不能把 arena 索引写成源码行号。

只使用真实 identifiers。方法必须带 owner type，例如 `\verb|QyBsqElabVariable::transcribe|`。自由函数带实际模块路径，不杜撰 owner type。variant 必须带类型，例如 `\verb|QyMirExprElaboration::ArchOnly|`。

正文和 label 中的 identifiers，以及任何含 `_`、`(`、`→`、`=`、`#` 的内容，都放在 `\verb|...|` 中。panic 和调用栈放在 `verbatim` 中，保留原文，不在其中嵌套 `\verb`。

正文汉字总数必须少于 400，不计 codespan labels。源码摘录、panic 原文和 stack frames 是引用材料，不属于正文。整个页面至多使用六个 `kyCodespan`。

在 preamble 中用 `\kySourceDefaultOid{oid}` 固定一个源码提交。oid 是该提交的 Git object ID。页内引用不得混用其他 oid。所有 span 都必须按 `git show <oid>:<path>` 验证。

语法以现有 [kysource.sty](../../../../gadgets/kathy-page/kysource.sty) 为准。

```tex
\begin{kyCodespan}{path}{L1-L2}[oid]
\kySpan{line}{c1}{c2}{label}[severity]
\end{kyCodespan}
```

`path` 是仓库相对路径。`L1` 和 `L2` 表示摘录的起止源码行号，行号前的 `L` 可省略。`line` 是 span 的源码行号。`c1` 和 `c2` 是从一开始计数的 Unicode 字符列，包含两端。`label` 是注释正文。`severity` 可取 `note`、`warning`、`error`，省略时为 `note`。`[oid]` 是可选参数，省略时使用页面的默认 oid。环境内部只允许 `\kySpan`。

没有注释的源码引用使用 `\kySource{path}{L1-L2}[oid]`。converter 也接受旧式 `\kySource{path}{L1-L2}{oid}`。本 form 要求标注的位置仍使用 `kyCodespan`。

```tex
\begin{kyQuote}[title]
\begin{verbatim}
literal text
\end{verbatim}
\end{kyQuote}
```

`title` 是可省略的引用标题。`literal text` 表示原样保留的引用内容。`kyQuote` 支持正文和嵌套 `verbatim`；panic 与调用栈使用上面的形式。
