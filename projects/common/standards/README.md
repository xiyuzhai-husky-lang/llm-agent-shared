# Standards

所有项目的 Orchestrator、Worker、Reviewer 和 nitpicker 共用这些标准，一条规则一个文件。Orchestrator 在命名、组织和提出方案时应用；Worker 在实现前读并应用；Reviewer 与 nitpicker 对照实际对象和候选检查。角色文件链接标准，不各自维护一份概念定义。用户原话用于保存来源，正文写成可理解、可应用的准则。

涉及命名、组织或相关性判断时，先按 `naming-consistency.md` 确定对象与参照，再应用具体的目录和命名规则。审查严重性顺序仍以隐藏失败为第一。

Order of severity for review: `failures-are-exposed.md` first, then the rest.

- [failures-are-exposed.md](failures-are-exposed.md) — a failure stays where it is and fails; moving, relabelling or isolating it to pass the Outcome is cheating.
- [test-statuses-are-the-users.md](test-statuses-are-the-users.md) — `deferred`, `regressed`, `pending`, `avoided` are set by the user only; an agent that adds one commits treason.
- [naming-consistency.md](naming-consistency.md) — 从对象定义和组成确定分类，保持组内及跨语言、跨职责的对应。
- [test-data-mirrors-source.md](test-data-mirrors-source.md) — 测试跟随被测对象的结构；先确定 source unit，再应用文件对应规则。
- [file-names.md](file-names.md) — a path says what it holds; dots only for the suffix; no "successful", "proofs", "divergence".
