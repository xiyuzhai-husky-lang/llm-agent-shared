# Criteria

- [One package, nothing outside the movement loop](evidence/user-rulings.md#L5-L6)
  > Only `pathfinding` is new; the movement loop is the one caller changed.

- [The bot never steps on lava](evidence/user-rulings.md#L9-L10)
  > Lava cells cost infinity; the search never returns a path through one.

- [A blocked path is replanned, not pushed through](evidence/user-rulings.md#L13-L14)
  > A monster on the next cell for two ticks triggers a new search from the current cell.
