# Verification

Working directory is the repository root; the commands run in the implementation step.

```sh
go test ./kvcache/... ./attention/...
./run-tests.sh
```

Expected: exit code 0; the paged and contiguous kernels agree on the equality test.
