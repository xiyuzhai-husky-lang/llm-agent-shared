## Path and name consistency

Judge every name and path in two directions. Inner consistency: within a close group (one directory, one enum, one
module's functions, one family of test files) every name follows one scheme: the same axis (all by contents, or all by
the source unit they mirror), the same number, the same suffix convention, the same granularity; one file named by
intent next to one named by carrier and one by operation is a finding even if each name alone reads fine. Outer
consistency: the name lines up with its counterparts across languages and roles: the Rust source path, the Lean
module, the test-data path and the docs entry for the same thing correspond; a test tree that mirrors the source tree
in one place and invents its own grouping in another is a finding. The user's reason: good consistency reduces the
burden on the brain, and organizing the same thing in different ways causes trouble when searching.
Say which group or counterpart the name breaks with and what the consistent name is.
