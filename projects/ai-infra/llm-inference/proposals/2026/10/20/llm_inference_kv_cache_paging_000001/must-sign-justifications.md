# MUST SIGN files

The project rule `attention/**` covers one changed file.

## attention/kernel.go

The contiguous read of keys and values becomes a gather through the block table; the arithmetic of the kernel is unchanged, as the equality test shows.
