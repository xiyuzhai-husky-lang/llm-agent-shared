# Criteria

- [One package, the kernel is the one caller changed](evidence/user-rulings.md#L5-L6)
  > Only `kvcache` is new; the attention kernel reads through the block table.

- [Blocks are sixteen tokens](evidence/user-rulings.md#L9-L10)
  > The block size is a constant, sixteen, not a configuration knob.

- [A finished sequence frees its blocks at the end of the step](evidence/user-rulings.md#L13-L14)
  > Eviction runs once per step, after the last token of the step is sampled.
