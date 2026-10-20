# Tests

- `kvcache/pool_test.go`: a pool of four blocks hands out four, refuses the fifth, and hands out a freed one again.
- `kvcache/table_test.go`: the seventeenth token lands in the second block; lookup of token index twenty returns block one, offset four.
- `kvcache/evict_test.go`: two finished sequences out of three free exactly their blocks.
- `attention/kernel_test.go`: the output over a paged cache equals the output over a contiguous one.
