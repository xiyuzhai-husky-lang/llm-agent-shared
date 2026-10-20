# Test data mirrors the source layout

先按 [naming-consistency.md](naming-consistency.md) 确定被测对象，再确定应当对应的 source。“source”指拥有被测概念、结构或行为的定义及其组织，不能默认成测试直接 import 的文件、执行时最后经过的 helper，或候选自己新拆出的实现文件。

同一 source unit 确需多个测试文件时，额外文件放同名目录；先确定同一个 source unit，再使用这条文件对应规则。

分类不取决于测试当前通过还是失败。失败留在按对象应属的位置并真实失败；“mirrors source”不授权建立失败隔离目录或改变测试状态。见 [failures-are-exposed.md](failures-are-exposed.md) 和 [test-statuses-are-the-users.md](test-statuses-are-the-users.md)。

来源：用户要求 custodes 跟随 term crate 的既有布局；要求 Codec 从 `QyExpr ↔ Lean.Expr` 的定义及其对应对象确定组织方式。
