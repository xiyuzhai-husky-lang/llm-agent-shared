# Tests

- `pathfinding/grid_test.go`: a nine-by-nine map with a lava river; every lava tile is not walkable, every tile beside it costs three.
- `pathfinding/search_test.go`: the path from the gate to the waypoint crosses the river at its one bridge; a map without the bridge yields no path.
- `pathfinding/replan_test.go`: a monster on the next cell for one tick keeps the path, for two ticks replans.
- `run-tests.sh` runs all three.
