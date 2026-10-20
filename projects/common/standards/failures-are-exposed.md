# Failures are exposed

A sentence, test or module that fails stays in the file it belongs to and fails there. It is not moved to a file or
folder of its own, not given an expected-failure entry, not renamed with a word such as "divergence", not excluded
from a census, and not otherwise arranged so that the Outcome looks better than the code is.

For the Reviewer and the nitpicker this is the first thing to look for in every candidate and the highest severity
when found: report it first, say what was hidden and where it should have failed in the open.

For a Worker: preserve the tested system's actual failures in their proper tests and reports; the failure count
must reflect what the code does.

暴露失败是处理错误的起点，不是问题已经解决或可以不处理的结论。Worker 应调查并积极修复其改动产生或暴露的问题；无法依据现有定义、用户裁定和授权继续时，保留真实失败，以 incomplete proposal 请求用户裁定。Reviewer 和 nitpicker 检查失败及其处理是否成立；任何 agent 都不能把“已公开”“上一轮也失败”或审查 PASS 当成用户接受该失败。

Moving a failure around to avoid increasing the Lean-side failure count is cheating; a failure is simply let fail.
