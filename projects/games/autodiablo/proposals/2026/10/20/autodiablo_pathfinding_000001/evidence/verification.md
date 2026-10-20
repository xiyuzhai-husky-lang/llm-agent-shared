# Verification

Working directory is the repository root; the commands run in the implementation step.

```sh
go test ./pathfinding/...
./run-tests.sh
```

Expected: exit code 0; the three test files pass; the movement loop compiles with the path call.
