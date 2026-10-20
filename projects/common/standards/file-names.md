# File and directory names

A name says what the file or directory holds. Not the author's intent or the moment of writing (`proofs.qcy.tex` for
a set of assertions), not the outcome (`successful_assertions`, `accepted`), not a euphemism for a failure
(`lean_divergence`), not a catch-all (`misc`, `extra`, `new`, `tmp`), not an abbreviation.

目录、namespace 和 module 的类别先由 [naming-consistency.md](naming-consistency.md) 确定。不能把本次一起修改的主题拼成一个类别，也不能仅因拆开了主题就认为分类成立：先确定它们在被处理对象中的层级，再沿对象结构命名。

每个新名字都要指出其对应的既有概念及包含关系。共享代码按它实际承担的共同职责命名；仅仅“没有写 operation 名”还不足以说明放置合理。Reviewer 和 nitpicker 检查名字与对象结构的对应，不能只检查名字是否读得懂。

Dots in a file name are for the suffix only (`.qcy.tex`, `.test.toml`). Categories are not stacked with dots
(`sum.reductions.lean_divergence.qcy.tex`); a further level is a folder (`sum/reductions.qcy.tex`).

Reviewers and nitpickers check path slop closely. A case they missed: `PowerModulo`, a newly coined grab-bag category
rather than an established historical name. No `XXdivergence` names.
